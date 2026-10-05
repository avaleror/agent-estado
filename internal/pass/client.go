package pass

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

func checkIntro(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return exitCode(1, "the introduction point has to be https.")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		h := u.Hostname()
		if h == "127.0.0.1" || h == "::1" {
			return nil
		}
	}
	return exitCode(1, "the introduction point has to be https.")
}

func introUDP(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

type introAPI struct {
	base  string
	token string
	hc    *http.Client
}

func newIntro(base string) *introAPI {
	return &introAPI{
		base: base,
		hc: &http.Client{
			Transport: &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialContext(ctx, network, addr)
			}},
		},
	}
}

func (a *introAPI) call(ctx context.Context, method, path string, body, out any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if a.token != "" {
		req.Header.Set("Overto-Token", a.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if out != nil && len(slurp) > 0 {
		_ = json.Unmarshal(slurp, out)
	}
	return resp.StatusCode, slurp, nil
}

func statusErr(code int, slurp []byte) error {
	switch code {
	case http.StatusNotFound:
		return errGone()
	case http.StatusForbidden:
		return errRejected()
	case http.StatusConflict:
		if bytes.Contains(slurp, []byte("busy")) {
			return errBusy()
		}
		return exitCode(2, "that id was rejected.")
	case http.StatusTooManyRequests:
		return exitCode(2, "the introduction point is full.")
	default:
		if code >= 400 || code == 0 {
			return errUnreachable()
		}
	}
	return nil
}

type regResp struct {
	Token     string `json:"token"`
	RelayHost string `json:"relay_host"`
	RelayPort int    `json:"relay_port"`
}

func (a *introAPI) register(ctx context.Context, plate string, cands []cand, pB []byte) (regResp, error) {
	var out regResp
	code, slurp, err := a.call(ctx, http.MethodPost, "/v1/register", map[string]any{
		"nameplate": plate, "version": 1, "candidates": cands, "receiver_msg": base64.StdEncoding.EncodeToString(pB),
	}, &out)
	if err != nil {
		return out, errUnreachable()
	}
	if code != http.StatusOK {
		return out, statusErr(code, slurp)
	}
	a.token = out.Token
	return out, nil
}

func (a *introAPI) refresh(ctx context.Context, plate string, cands []cand) error {
	code, slurp, err := a.call(ctx, http.MethodPost, "/v1/refresh", map[string]any{
		"nameplate": plate, "candidates": cands,
	}, nil)
	if err != nil {
		return err
	}
	if code == http.StatusNoContent {
		return nil
	}
	return statusErr(code, slurp)
}

func (a *introAPI) delete(ctx context.Context, plate string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _, _ = a.call(ctx, http.MethodDelete, "/v1/register", map[string]string{"nameplate": plate}, nil)
}

func (a *introAPI) release(ctx context.Context, plate string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _, _ = a.call(ctx, http.MethodPost, "/v1/release", map[string]string{"nameplate": plate}, nil)
}

type approachResp struct {
	ReceiverMsg string `json:"receiver_msg"`
	Cands       []cand `json:"receiver_candidates"`
	RelayHost   string `json:"relay_host"`
	RelayPort   int    `json:"relay_port"`
}

func (a *introAPI) approach(ctx context.Context, plate string, cands []cand, pA []byte) (approachResp, []byte, error) {
	var out approachResp
	code, slurp, err := a.call(ctx, http.MethodPost, "/v1/approach", map[string]any{
		"nameplate": plate, "version": 1, "candidates": cands, "sender_msg": base64.StdEncoding.EncodeToString(pA),
	}, &out)
	if err != nil {
		return out, nil, errUnreachable()
	}
	if code != http.StatusOK {
		return out, nil, statusErr(code, slurp)
	}
	pB, err := base64.StdEncoding.DecodeString(out.ReceiverMsg)
	if err != nil || len(pB) != 65 {
		return out, nil, errRejected()
	}
	return out, pB, nil
}

func (a *introAPI) confirm(ctx context.Context, plate string, cA []byte) ([]byte, error) {
	var out struct {
		CB string `json:"c_b"`
	}
	code, slurp, err := a.call(ctx, http.MethodPost, "/v1/confirm", map[string]string{
		"nameplate": plate, "c_a": base64.StdEncoding.EncodeToString(cA),
	}, &out)
	if err != nil {
		return nil, errUnreachable()
	}
	if code != http.StatusOK {
		return nil, statusErr(code, slurp)
	}
	cB, err := base64.StdEncoding.DecodeString(out.CB)
	if err != nil || len(cB) != 32 {
		return nil, errRejected()
	}
	return cB, nil
}

type waitResp struct {
	Status    string `json:"status"`
	Sender    string `json:"sender_msg"`
	Cands     []cand `json:"sender_candidates"`
	CA        string `json:"c_a"`
	RelayHost string `json:"relay_host"`
	RelayPort int    `json:"relay_port"`
}

func (a *introAPI) waitApproach(ctx context.Context, plate string) (waitResp, []byte, []byte, error) {
	for {
		if ctx.Err() != nil {
			return waitResp{}, nil, nil, errUnreachable()
		}
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		var out waitResp
		code, slurp, err := a.call(reqCtx, http.MethodGet, "/v1/wait?nameplate="+url.QueryEscape(plate), nil, &out)
		cancel()
		if err != nil {
			return waitResp{}, nil, nil, errUnreachable()
		}
		if code != http.StatusOK {
			return waitResp{}, nil, nil, statusErr(code, slurp)
		}
		if out.Status != "approached" {
			continue
		}
		pA, err1 := base64.StdEncoding.DecodeString(out.Sender)
		cA, err2 := base64.StdEncoding.DecodeString(out.CA)
		if err1 != nil || err2 != nil || len(pA) != 65 || len(cA) != 32 {
			return out, nil, nil, errRejected()
		}
		return out, pA, cA, nil
	}
}

func (a *introAPI) confirmB(ctx context.Context, plate string, cB []byte) error {
	code, slurp, err := a.call(ctx, http.MethodPost, "/v1/confirm-b", map[string]string{
		"nameplate": plate, "c_b": base64.StdEncoding.EncodeToString(cB),
	}, nil)
	if err != nil {
		return errUnreachable()
	}
	if code == http.StatusNoContent {
		return nil
	}
	return statusErr(code, slurp)
}

func macMatch(a, b []byte) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1
}

func mapOnce(ctx context.Context, conn *net.UDPConn, dest string) (net.IP, int, error) {
	raddr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		return nil, 0, err
	}
	var cookie [8]byte
	if _, err := rand.Read(cookie[:]); err != nil {
		return nil, 0, err
	}
	pkt := marshalMap(cookie[:])
	dead := time.Now().Add(2 * time.Second)
	buf := make([]byte, 64)
	for time.Now().Before(dead) {
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		_, _ = conn.WriteToUDP(pkt, raddr)
		_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		ip, port, err := parseMapped(buf[:n], cookie[:])
		if err != nil {
			continue
		}
		return ip, port, nil
	}
	return nil, 0, fmt.Errorf("no map")
}
