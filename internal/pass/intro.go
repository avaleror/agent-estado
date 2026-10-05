package pass

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	regTTL    = 90 * time.Second
	regCap    = 4096
	regPerIP  = 8
	waitSlice = 25 * time.Second
	relayCeil = 600 * 1024
	relayPair = 10 * time.Second
)

// Server is one in-memory introduction point. It keeps no files.
type Server struct {
	mu        sync.Mutex
	slots     map[string]*slot
	now       func() time.Time
	relayHost string
	relayPort int

	mux     *http.ServeMux
	httpLn  net.Listener
	httpSrv *http.Server
	udp     *net.UDPConn
	relay   net.Listener
	done    chan struct{}
}

type slot struct {
	cond      *sync.Cond
	tokenHash [32]byte
	ip        string
	expiry    time.Time
	state     string
	pB        []byte
	pA        []byte
	cA        []byte
	cB        []byte
	recvC     []cand
	sendC     []cand
	fail      string
}

func NewServer(relayHost string, relayPort int) *Server {
	s := &Server{
		slots:     map[string]*slot{},
		now:       time.Now,
		relayHost: relayHost,
		relayPort: relayPort,
		done:      make(chan struct{}),
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("POST /v1/register", s.register)
	s.mux.HandleFunc("POST /v1/refresh", s.refresh)
	s.mux.HandleFunc("DELETE /v1/register", s.drop)
	s.mux.HandleFunc("POST /v1/approach", s.approach)
	s.mux.HandleFunc("POST /v1/confirm", s.confirm)
	s.mux.HandleFunc("GET /v1/wait", s.wait)
	s.mux.HandleFunc("POST /v1/confirm-b", s.confirmB)
	s.mux.HandleFunc("POST /v1/release", s.release)
	s.mux.HandleFunc("GET /health", s.health)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenIntro binds HTTP, the UDP map, and the relay. An empty udpAddr uses
// the HTTP port. cert and key turn the HTTP listener into TLS.
func ListenIntro(httpAddr, udpAddr, relayAddr, cert, key string) (*Server, string, string, string, error) {
	ln, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return nil, "", "", "", err
	}
	tcp := ln.Addr().(*net.TCPAddr)
	if udpAddr == "" {
		udpAddr = net.JoinHostPort(tcp.IP.String(), strconv.Itoa(tcp.Port))
	}
	uaddr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		ln.Close()
		return nil, "", "", "", err
	}
	udp, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		ln.Close()
		return nil, "", "", "", err
	}
	relay, err := net.Listen("tcp", relayAddr)
	if err != nil {
		ln.Close()
		udp.Close()
		return nil, "", "", "", err
	}
	rp := relay.Addr().(*net.TCPAddr)
	host := tcp.IP.String()
	s := NewServer(host, rp.Port)
	s.httpLn = ln
	s.udp = udp
	s.relay = relay
	s.relayHost = rp.IP.String()
	s.httpSrv = &http.Server{Handler: s.mux}
	go func() {
		var err error
		if cert != "" {
			err = s.httpSrv.ServeTLS(ln, cert, key)
		} else {
			err = s.httpSrv.Serve(ln)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return
		}
	}()
	go s.serveMap()
	go s.serveRelay()
	httpShow := net.JoinHostPort(tcp.IP.String(), strconv.Itoa(tcp.Port))
	udpShow := udp.LocalAddr().String()
	relayShow := relay.Addr().String()
	return s, httpShow, udpShow, relayShow, nil
}

func (s *Server) Close() {
	if s.httpSrv != nil {
		s.httpSrv.Close()
	}
	if s.udp != nil {
		s.udp.Close()
	}
	if s.relay != nil {
		s.relay.Close()
	}
}

func (s *Server) alive(sl *slot) bool {
	return s.now().Before(sl.expiry)
}

func (s *Server) countIP(ip string) int {
	n := 0
	for _, sl := range s.slots {
		if sl.ip == ip && s.alive(sl) {
			n++
		}
	}
	return n
}

func (s *Server) live() int {
	n := 0
	for _, sl := range s.slots {
		if s.alive(sl) {
			n++
		}
	}
	return n
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func b64n(s string, n int) ([]byte, bool) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) != n {
		return nil, false
	}
	return b, true
}

func (s *Server) tokenOK(sl *slot, r *http.Request) bool {
	raw, err := hex.DecodeString(r.Header.Get("Overto-Token"))
	if err != nil || len(raw) != 16 {
		return false
	}
	sum := sha256.Sum256(raw)
	return subtle.ConstantTimeCompare(sum[:], sl.tokenHash[:]) == 1
}

type regReq struct {
	Nameplate   string `json:"nameplate"`
	Version     int    `json:"version"`
	Candidates  []cand `json:"candidates"`
	ReceiverMsg string `json:"receiver_msg"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req regReq
	if !readJSON(w, r, &req) {
		return
	}
	if !validPlate(req.Nameplate) || req.Version != 1 || len(req.Candidates) > 8 {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	pB, ok := b64n(req.ReceiverMsg, 65)
	if !ok || !candsOK(req.Candidates) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	ip := clientIP(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.slots[req.Nameplate]; ok && s.alive(old) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "taken"})
		return
	}
	if s.live() >= regCap || s.countIP(ip) >= regPerIP {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "full"})
		return
	}
	var tok [16]byte
	if _, err := rand.Read(tok[:]); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusInternalServerError)
		return
	}
	sl := &slot{
		ip:     ip,
		expiry: s.now().Add(regTTL),
		state:  "waiting",
		pB:     pB,
		recvC:  append([]cand(nil), req.Candidates...),
	}
	sl.cond = sync.NewCond(&s.mu)
	sl.tokenHash = sha256.Sum256(tok[:])
	s.slots[req.Nameplate] = sl
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      hex.EncodeToString(tok[:]),
		"relay_host": s.relayHost,
		"relay_port": s.relayPort,
	})
}

type refreshReq struct {
	Nameplate  string `json:"nameplate"`
	Candidates []cand `json:"candidates"`
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.Candidates) > 8 || !candsOK(req.Candidates) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[req.Nameplate]
	if sl == nil || !s.alive(sl) || !s.tokenOK(sl, r) {
		http.NotFound(w, r)
		return
	}
	sl.recvC = append([]cand(nil), req.Candidates...)
	sl.expiry = s.now().Add(regTTL)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) drop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nameplate string `json:"nameplate"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[req.Nameplate]
	if sl == nil || !s.tokenOK(sl, r) {
		http.NotFound(w, r)
		return
	}
	delete(s.slots, req.Nameplate)
	sl.cond.Broadcast()
	w.WriteHeader(http.StatusNoContent)
}

type approachReq struct {
	Nameplate  string `json:"nameplate"`
	Version    int    `json:"version"`
	Candidates []cand `json:"candidates"`
	SenderMsg  string `json:"sender_msg"`
}

func (s *Server) approach(w http.ResponseWriter, r *http.Request) {
	var req approachReq
	if !readJSON(w, r, &req) {
		return
	}
	if !validPlate(req.Nameplate) || req.Version != 1 || len(req.Candidates) > 8 {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	pA, ok := b64n(req.SenderMsg, 65)
	if !ok || !candsOK(req.Candidates) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[req.Nameplate]
	if sl == nil || !s.alive(sl) {
		http.NotFound(w, r)
		return
	}
	if sl.state != "waiting" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "busy"})
		return
	}
	sl.pA = pA
	sl.sendC = append([]cand(nil), req.Candidates...)
	sl.cA = nil
	sl.cB = nil
	sl.fail = ""
	sl.state = "approached"
	sl.cond.Broadcast()
	writeJSON(w, http.StatusOK, map[string]any{
		"receiver_msg":        b64(sl.pB),
		"receiver_candidates": sl.recvC,
		"relay_host":          s.relayHost,
		"relay_port":          s.relayPort,
	})
}

type confirmReq struct {
	Nameplate string `json:"nameplate"`
	CA        string `json:"c_a"`
}

func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	var req confirmReq
	if !readJSON(w, r, &req) {
		return
	}
	cA, ok := b64n(req.CA, 32)
	if !ok {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[req.Nameplate]
	if sl == nil || !s.alive(sl) {
		http.NotFound(w, r)
		return
	}
	if sl.state == "waiting" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	if len(sl.cA) == 0 {
		sl.cA = cA
		sl.state = "confirming"
		sl.cond.Broadcast()
	} else if subtle.ConstantTimeCompare(sl.cA, cA) != 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "conflict"})
		return
	}
	deadline := time.Now().Add(20 * time.Second)
	if d, ok := r.Context().Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	for len(sl.cB) != 32 && sl.fail == "" && s.alive(sl) {
		remain := time.Until(deadline)
		if remain <= 0 {
			writeJSON(w, http.StatusGatewayTimeout, map[string]string{"error": "timeout"})
			return
		}
		timer := time.AfterFunc(remain, func() { sl.cond.Broadcast() })
		sl.cond.Wait()
		timer.Stop()
	}
	if sl.fail == "rejected" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "rejected"})
		return
	}
	if len(sl.cB) != 32 {
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{"error": "timeout"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"c_b": b64(sl.cB)})
}

func (s *Server) wait(w http.ResponseWriter, r *http.Request) {
	plate := r.URL.Query().Get("nameplate")
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[plate]
	if sl == nil || !s.alive(sl) || !s.tokenOK(sl, r) {
		http.NotFound(w, r)
		return
	}
	deadline := time.Now().Add(waitSlice)
	if d, ok := r.Context().Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	for sl.state != "confirming" && s.alive(sl) {
		remain := time.Until(deadline)
		if remain <= 0 {
			break
		}
		timer := time.AfterFunc(remain, func() { sl.cond.Broadcast() })
		sl.cond.Wait()
		timer.Stop()
	}
	if sl.state == "confirming" && len(sl.cA) == 32 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "approached",
			"sender_msg":        b64(sl.pA),
			"sender_candidates": sl.sendC,
			"c_a":               b64(sl.cA),
			"relay_host":        s.relayHost,
			"relay_port":        s.relayPort,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "waiting"})
}

func (s *Server) confirmB(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nameplate string `json:"nameplate"`
		CB        string `json:"c_b"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	cB, ok := b64n(req.CB, 32)
	if !ok {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[req.Nameplate]
	if sl == nil || !s.alive(sl) || !s.tokenOK(sl, r) {
		http.NotFound(w, r)
		return
	}
	if sl.state != "confirming" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	sl.cB = cB
	sl.cond.Broadcast()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nameplate string `json:"nameplate"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[req.Nameplate]
	if sl == nil || !s.alive(sl) || !s.tokenOK(sl, r) {
		http.NotFound(w, r)
		return
	}
	sl.pA = nil
	sl.sendC = nil
	sl.cA = nil
	sl.cB = nil
	sl.fail = "rejected"
	sl.state = "waiting"
	sl.cond.Broadcast()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	n := s.live()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": n})
}

func candsOK(cs []cand) bool {
	for _, c := range cs {
		if !validCand(c) {
			return false
		}
	}
	return true
}

func (s *Server) serveMap() {
	buf := make([]byte, 64)
	for {
		_ = s.udp.SetReadDeadline(time.Now().Add(time.Second))
		n, addr, err := s.udp.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		if n != 14 || string(buf[:4]) != "OVMP" || buf[4] != 1 || buf[5] != 1 {
			continue
		}
		cookie := append([]byte(nil), buf[6:14]...)
		_, _ = s.udp.WriteToUDP(marshalMapped(cookie, addr), addr)
	}
}

type relayHalf struct {
	conn net.Conn
	role byte
}

func (s *Server) serveRelay() {
	pending := map[string]*relayHalf{}
	var mu sync.Mutex
	for {
		c, err := s.relay.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			_ = c.SetReadDeadline(time.Now().Add(relayPair))
			role, token, err := readOVR1(c)
			if err != nil || (role != 0x01 && role != 0x02) {
				c.Close()
				return
			}
			_ = c.SetReadDeadline(time.Time{})
			key := string(token)
			mu.Lock()
			other := pending[key]
			if other == nil {
				pending[key] = &relayHalf{conn: c, role: role}
				mu.Unlock()
				time.AfterFunc(relayPair, func() {
					mu.Lock()
					if pending[key] != nil && pending[key].conn == c {
						delete(pending, key)
						c.Close()
					}
					mu.Unlock()
				})
				return
			}
			if other.role == role {
				mu.Unlock()
				c.Close()
				return
			}
			delete(pending, key)
			peer := other.conn
			mu.Unlock()
			splice(c, peer)
		}(c)
	}
}

func splice(a, b net.Conn) {
	var once sync.Once
	closeBoth := func() { once.Do(func() { a.Close(); b.Close() }) }
	var wg sync.WaitGroup
	copyOne := func(dst, src net.Conn) {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		var n int
		for {
			nr, err := src.Read(buf)
			n += nr
			if n > relayCeil {
				closeBoth()
				return
			}
			if nr > 0 {
				if _, werr := dst.Write(buf[:nr]); werr != nil {
					closeBoth()
					return
				}
			}
			if err != nil {
				closeBoth()
				return
			}
		}
	}
	wg.Add(2)
	go copyOne(b, a)
	go copyOne(a, b)
	wg.Wait()
}
