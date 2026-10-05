package pass

import (
	"context"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"
)

var dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	return d.DialContext(ctx, network, addr)
}

// directBudget is how long a direct attempt may run before the relay.
var directBudget = 5 * time.Second

func skipDirect() bool { return os.Getenv("OVERTO_SKIP_DIRECT") == "1" }

type established struct {
	conn  net.Conn
	path  *Path
	label string
}

func fileRole(role string) byte {
	if role == "to" {
		return 0x01
	}
	return 0x02
}

func labelFor(class byte) string {
	switch class {
	case classLAN:
		return "lan"
	case classMap:
		return "mapped"
	case classRelay:
		return "relay"
	case classDirect:
		return "direct"
	default:
		return "trying"
	}
}

func owns(conn net.Conn, c cand) bool {
	la, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok || la.Port != c.Port {
		return false
	}
	ip, err := netip.ParseAddr(c.IP)
	if err != nil {
		return false
	}
	if ip.IsPrivate() {
		lip, ok := netip.AddrFromSlice(la.IP)
		if !ok {
			return false
		}
		lip = lip.Unmap()
		if lip.IsUnspecified() {
			return true
		}
		return lip == ip
	}
	return true
}

// establish tries TCP to the peer's candidates, then the relay.
// This version does not run the UDP punch stream.
func establish(ctx context.Context, role string, keys *Keys, plate []byte, ln net.Listener, local, remote []cand, relayHost string, relayPort int) (net.Conn, *Path, string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if skipDirect() || relayHost == "" {
		if relayHost == "" {
			return nil, nil, "", errUnreachable()
		}
		return dialRelay(ctx, role, keys, plate, relayHost, relayPort)
	}
	win := make(chan established, 1)
	var mu sync.Mutex
	won := false
	claim := func() bool {
		mu.Lock()
		defer mu.Unlock()
		if won {
			return false
		}
		won = true
		return true
	}
	if ln != nil {
		go acceptLoop(ctx, ln, role, keys, plate, local, win, claim)
	}
	for i, c := range remote {
		if !dialable(c) {
			continue
		}
		go dialOne(ctx, role, keys, plate, c, uint16(i), win, claim)
	}
	timer := time.NewTimer(directBudget)
	defer timer.Stop()
	select {
	case e := <-win:
		return e.conn, e.path, e.label, nil
	case <-timer.C:
	case <-ctx.Done():
		return nil, nil, "", errUnreachable()
	}
	type relayTry struct {
		e   established
		err error
	}
	relayCh := make(chan relayTry, 1)
	go func() {
		conn, path, err := relayPreamble(ctx, role, keys, plate, relayHost, relayPort)
		if err != nil {
			relayCh <- relayTry{err: err}
			return
		}
		relayCh <- relayTry{e: established{conn: conn, path: path, label: "relay"}}
	}()
	select {
	case e := <-win:
		return e.conn, e.path, e.label, nil
	case rr := <-relayCh:
		select {
		case e := <-win:
			if rr.e.conn != nil {
				rr.e.conn.Close()
			}
			return e.conn, e.path, e.label, nil
		default:
		}
		if rr.err != nil || !claim() {
			if rr.e.conn != nil {
				rr.e.conn.Close()
			}
			select {
			case e := <-win:
				return e.conn, e.path, e.label, nil
			case <-time.After(2 * time.Second):
				return nil, nil, "", errUnreachable()
			case <-ctx.Done():
				return nil, nil, "", errUnreachable()
			}
		}
		if err := exchangeType1(rr.e.conn, rr.e.path, role == "to"); err != nil {
			rr.e.conn.Close()
			return nil, nil, "", err
		}
		return rr.e.conn, rr.e.path, "relay", nil
	case <-ctx.Done():
		return nil, nil, "", errUnreachable()
	}
}

func acceptLoop(ctx context.Context, ln net.Listener, role string, keys *Keys, plate []byte, local []cand, win chan established, claim func() bool) {
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			e, err := finishAccept(conn, role, keys, plate, local, claim)
			if err != nil {
				conn.Close()
				return
			}
			select {
			case win <- e:
			default:
				conn.Close()
			}
		}(conn)
	}
}

func finishAccept(conn net.Conn, role string, keys *Keys, plate []byte, local []cand, claim func() bool) (established, error) {
	_ = conn.SetDeadline(time.Now().Add(directBudget))
	class, attempt, peerRole, got, err := readOVT1(conn, keys.Punch)
	if err != nil {
		return established{}, err
	}
	if string(got) != string(plate) || int(attempt) >= len(local) {
		return established{}, errRejected()
	}
	if classOf(local[attempt]) != class || !owns(conn, local[attempt]) {
		return established{}, errRejected()
	}
	if peerRole == fileRole(role) {
		return established{}, errRejected()
	}
	path, err := derivePath(keys.Send, keys.Recv, class, attempt, peerRole, role == "to")
	if err != nil {
		return established{}, err
	}
	sender := role == "to"
	if sender {
		if err := exchangeType1(conn, path, true); err != nil {
			return established{}, err
		}
	} else {
		_ = conn.SetDeadline(time.Now().Add(directBudget))
		typ, _, err := readRec(conn, path)
		if err != nil || typ != recConfirm {
			return established{}, errStopped()
		}
		if !claim() {
			return established{}, errStopped()
		}
		if err := writeRec(conn, path, recConfirm, nil); err != nil {
			return established{}, err
		}
	}
	if sender && !claim() {
		return established{}, errStopped()
	}
	_ = conn.SetDeadline(time.Time{})
	return established{conn: conn, path: path, label: labelFor(class)}, nil
}

func dialOne(ctx context.Context, role string, keys *Keys, plate []byte, c cand, attempt uint16, win chan established, claim func() bool) {
	dctx, cancel := context.WithTimeout(ctx, directBudget)
	defer cancel()
	conn, err := dialContext(dctx, "tcp", net.JoinHostPort(c.IP, itoa(c.Port)))
	if err != nil {
		return
	}
	class := classOf(c)
	_ = conn.SetDeadline(time.Now().Add(directBudget))
	if err := writeOVT1(conn, keys.Punch, plate, class, attempt, fileRole(role)); err != nil {
		conn.Close()
		return
	}
	path, err := derivePath(keys.Send, keys.Recv, class, attempt, fileRole(role), role == "to")
	if err != nil {
		conn.Close()
		return
	}
	if role == "to" {
		if err := exchangeType1(conn, path, true); err != nil || !claim() {
			conn.Close()
			return
		}
	} else {
		typ, _, err := readRec(conn, path)
		if err != nil || typ != recConfirm || !claim() {
			conn.Close()
			return
		}
		if err := writeRec(conn, path, recConfirm, nil); err != nil {
			conn.Close()
			return
		}
	}
	_ = conn.SetDeadline(time.Time{})
	e := established{conn: conn, path: path, label: labelFor(class)}
	select {
	case win <- e:
	default:
		conn.Close()
	}
}

func relayPreamble(ctx context.Context, role string, keys *Keys, plate []byte, host string, port int) (net.Conn, *Path, error) {
	if host == "" || port == 0 {
		return nil, nil, errUnreachable()
	}
	dctx, cancel := context.WithTimeout(ctx, relayPair)
	defer cancel()
	conn, err := dialContext(dctx, "tcp", net.JoinHostPort(host, itoa(port)))
	if err != nil {
		return nil, nil, errUnreachable()
	}
	_ = conn.SetDeadline(time.Now().Add(relayPair))
	if err := writeOVR1(conn, fileRole(role), keys.Relay); err != nil {
		conn.Close()
		return nil, nil, errUnreachable()
	}
	if err := writeOVT1(conn, keys.Punch, plate, classRelay, 0, fileRole(role)); err != nil {
		conn.Close()
		return nil, nil, errUnreachable()
	}
	class, attempt, peerRole, got, err := readOVT1(conn, keys.Punch)
	if err != nil || class != classRelay || attempt != 0 || string(got) != string(plate) || peerRole == fileRole(role) {
		conn.Close()
		return nil, nil, errUnreachable()
	}
	path, err := derivePath(keys.Send, keys.Recv, classRelay, 0, 0, role == "to")
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, path, nil
}

func dialRelay(ctx context.Context, role string, keys *Keys, plate []byte, host string, port int) (net.Conn, *Path, string, error) {
	conn, path, err := relayPreamble(ctx, role, keys, plate, host, port)
	if err != nil {
		return nil, nil, "", err
	}
	if err := exchangeType1(conn, path, role == "to"); err != nil {
		conn.Close()
		return nil, nil, "", err
	}
	return conn, path, "relay", nil
}
