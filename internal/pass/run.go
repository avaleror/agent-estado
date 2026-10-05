package pass

import (
	"context"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"
)

// Config is one side of a transfer. The shell sets Role to "here" or "to".
type Config struct {
	Role   string
	RunDir string
	Direct string
	Intro  string
}

// Run speaks the run-directory contract and moves the share set.
func Run(ctx context.Context, cfg Config) error {
	run, err := openRun(cfg.RunDir)
	if err != nil {
		return err
	}
	err = run.transfer(ctx, cfg)
	return run.finish(err)
}

func (r *runDir) transfer(ctx context.Context, cfg Config) error {
	plate, password, display, err := r.readID()
	if err != nil {
		return err
	}
	_ = r.event("id", display)
	pw := []byte(password)
	defer func() {
		for i := range pw {
			pw[i] = 0
		}
	}()
	var files []entry
	if cfg.Role == "to" {
		files, err = r.loadStage()
		if err != nil {
			return err
		}
	} else if cfg.Role != "here" {
		return exitCode(1, "the transfer stopped. Nothing was written.")
	}
	if cfg.Direct != "" {
		return r.direct(ctx, cfg.Role, cfg.Direct, pw, []byte(plate), files)
	}
	if err := checkIntro(cfg.Intro); err != nil {
		return err
	}
	return r.viaIntro(ctx, cfg.Role, cfg.Intro, pw, []byte(plate), files)
}

func (r *runDir) direct(ctx context.Context, role, addr string, password, plate []byte, files []entry) error {
	var conn net.Conn
	if role == "here" {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return errUnreachable()
		}
		defer ln.Close()
		if err := r.event("bound", boundText(ln)); err != nil {
			return err
		}
		c, err := acceptOne(ctx, ln)
		if err != nil {
			return err
		}
		conn = c
	} else {
		c, err := dialContext(ctx, "tcp", addr)
		if err != nil {
			return errUnreachable()
		}
		conn = c
	}
	defer conn.Close()
	keys, err := handshakeDirect(conn, role, password, plate)
	if err != nil {
		return err
	}
	path, err := derivePath(keys.Send, keys.Recv, classDirect, 0, 0, role == "to")
	if err != nil {
		return err
	}
	if err := exchangeType1(conn, path, role == "to"); err != nil {
		return err
	}
	return runSession(conn, role, path, "direct", r, keys, files)
}

func acceptOne(ctx context.Context, ln net.Listener) (net.Conn, error) {
	ch := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		ch <- c
	}()
	select {
	case c := <-ch:
		return c, nil
	case <-errCh:
		return nil, errUnreachable()
	case <-ctx.Done():
		ln.Close()
		return nil, errUnreachable()
	}
}

func boundText(ln net.Listener) string {
	ta, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return ln.Addr().String()
	}
	port := ta.Port
	ip, _ := netip.AddrFromSlice(ta.IP)
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsUnspecified() {
		lines := []string{net.JoinHostPort(ta.IP.String(), itoa(port))}
		for _, c := range localCandidates(port) {
			lines = append(lines, net.JoinHostPort(c.IP, itoa(port)))
		}
		return strings.Join(lines, "\n")
	}
	return net.JoinHostPort(ip.String(), itoa(port))
}

func handshakeDirect(conn net.Conn, role string, password, plate []byte) (*Keys, error) {
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	defer conn.SetDeadline(time.Time{})
	w, err := HashToScalar(password, plate)
	if err != nil {
		return nil, errRejected()
	}
	defer func() {
		for i := range w {
			w[i] = 0
		}
	}()
	if role == "to" {
		pA, st, err := BeginA(idSender, idReceiver, w, nil)
		if err != nil {
			return nil, errStopped()
		}
		if err := writeOVD1(conn, pA); err != nil {
			return nil, errUnreachable()
		}
		pB, err := readExact(conn, 65)
		if err != nil {
			return nil, errUnreachable()
		}
		keys, err := st.finishOverto(pB, plate)
		if err != nil {
			return nil, errRejected()
		}
		if _, err := conn.Write(keys.CA); err != nil {
			return nil, errUnreachable()
		}
		cB, err := readExact(conn, 32)
		if err != nil || !macMatch(cB, keys.CB) {
			return nil, errRejected()
		}
		return keys, nil
	}
	pB, st, err := BeginB(idSender, idReceiver, w, nil)
	if err != nil {
		return nil, errStopped()
	}
	pA, err := readOVD1(conn)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(pB); err != nil {
		return nil, errUnreachable()
	}
	keys, err := st.finishOverto(pA, plate)
	if err != nil {
		return nil, errRejected()
	}
	cA, err := readExact(conn, 32)
	if err != nil || !macMatch(cA, keys.CA) {
		return nil, errRejected()
	}
	if _, err := conn.Write(keys.CB); err != nil {
		return nil, errUnreachable()
	}
	return keys, nil
}

func (r *runDir) viaIntro(ctx context.Context, role, intro string, password, plate []byte, files []entry) error {
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return errUnreachable()
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if err := r.event("bound", boundText(ln)); err != nil {
		return err
	}
	var mappedIP net.IP
	var mappedPort int
	if udp, uerr := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: port}); uerr == nil {
		defer udp.Close()
		if dest, derr := introUDP(intro); derr == nil {
			if ip, p, merr := mapOnce(ctx, udp, dest); merr == nil {
				mappedIP, mappedPort = ip, p
			}
		}
	}
	cands := mergeMapped(localCandidates(port), mappedIP, mappedPort)
	w, err := HashToScalar(password, plate)
	if err != nil {
		return errRejected()
	}
	defer func() {
		for i := range w {
			w[i] = 0
		}
	}()
	api := newIntro(intro)
	var keys *Keys
	var remote []cand
	var relayHost string
	var relayPort int
	if role == "here" {
		pB, st, err := BeginB(idSender, idReceiver, w, nil)
		if err != nil {
			return errStopped()
		}
		reg, err := api.register(ctx, string(plate), cands, pB)
		if err != nil {
			return err
		}
		defer api.delete(context.Background(), string(plate))
		go r.refreshLoop(ctx, api, string(plate), port)
		got, pA, cA, err := api.waitApproach(ctx, string(plate))
		if err != nil {
			return err
		}
		keys, err = st.finishOverto(pA, plate)
		if err != nil || !macMatch(cA, keys.CA) {
			api.release(context.Background(), string(plate))
			return errRejected()
		}
		if err := api.confirmB(ctx, string(plate), keys.CB); err != nil {
			return err
		}
		remote = got.Cands
		relayHost, relayPort = got.RelayHost, got.RelayPort
		_ = reg
	} else {
		pA, st, err := BeginA(idSender, idReceiver, w, nil)
		if err != nil {
			return errStopped()
		}
		got, pB, err := api.approach(ctx, string(plate), cands, pA)
		if err != nil {
			return err
		}
		keys, err = st.finishOverto(pB, plate)
		if err != nil {
			return errRejected()
		}
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		cB, err := api.confirm(cctx, string(plate), keys.CA)
		cancel()
		if err != nil {
			return err
		}
		if !macMatch(cB, keys.CB) {
			return errRejected()
		}
		remote = got.Cands
		relayHost, relayPort = got.RelayHost, got.RelayPort
	}
	var useLn net.Listener
	if !skipDirect() {
		useLn = ln
	}
	conn, path, label, err := establish(ctx, role, keys, plate, useLn, cands, remote, relayHost, relayPort)
	if err != nil {
		if role == "here" {
			api.release(context.Background(), string(plate))
		}
		return err
	}
	defer conn.Close()
	if os.Getenv("OVERTO_DEBUG") == "1" {
		os.Stderr.WriteString("overto-pass: path " + label + "\n")
	}
	return runSession(conn, role, path, label, r, keys, files)
}

func mergeMapped(cs []cand, ip net.IP, port int) []cand {
	if ip == nil || port == 0 {
		return cs
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return cs
	}
	addr = addr.Unmap()
	if !allowAdvertise(addr) {
		return cs
	}
	if len(cs) >= 8 {
		return cs
	}
	return append(cs, cand{Proto: "tcp", IP: addr.String(), Port: port})
}

func (r *runDir) refreshLoop(ctx context.Context, api *introAPI, plate string, port int) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = api.refresh(context.Background(), plate, localCandidates(port))
		}
	}
}
