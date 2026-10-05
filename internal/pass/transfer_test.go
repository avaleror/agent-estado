package pass

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const goodID = "7k9qm3hf-4rv2-np8c-wxt3-f6dt-aj5m-eyb2qr"

func newRun(t *testing.T, id string) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"event", "cmd", "stage"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "id"), []byte(id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func putStage(t *testing.T, dir, name, body string) {
	t.Helper()
	dest := filepath.Join(dir, "stage", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitExists(t *testing.T, path string, d time.Duration) []byte {
	t.Helper()
	dead := time.Now().Add(d)
	for time.Now().Before(dead) {
		b, err := os.ReadFile(path)
		if err == nil {
			return b
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", path)
	return nil
}

func writeCmd(t *testing.T, dir, name, val string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "cmd", name), []byte(val+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resultOf(t *testing.T, dir string) string {
	t.Helper()
	b := waitExists(t, filepath.Join(dir, "event", "result"), 20*time.Second)
	return strings.TrimSpace(string(b))
}

func driveYes(t *testing.T, hereDir, toDir string) {
	t.Helper()
	hs := waitExists(t, filepath.Join(hereDir, "event", "sas"), 20*time.Second)
	ts := waitExists(t, filepath.Join(toDir, "event", "sas"), 20*time.Second)
	if strings.TrimSpace(string(hs)) != strings.TrimSpace(string(ts)) {
		t.Fatalf("sas mismatch %q %q", hs, ts)
	}
	if len(strings.TrimSpace(string(hs))) != 6 {
		t.Fatalf("sas %q", hs)
	}
	writeCmd(t, hereDir, "sas", "yes")
	writeCmd(t, toDir, "sas", "yes")
	waitExists(t, filepath.Join(hereDir, "event", "staged"), 20*time.Second)
	writeCmd(t, hereDir, "files", "yes")
}

func TestFrameReuseAndFlip(t *testing.T) {
	resetSeals()
	key := bytes.Repeat([]byte{0x11}, 32)
	ct, err := sealKey(key, classDirect, 0, 0, dirStoR, recConfirm, 1, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = sealKey(key, classDirect, 0, 0, dirStoR, recConfirm, 1, []byte("hi"))
	if !errors.Is(err, errReuse) {
		t.Fatalf("reuse: %v", err)
	}
	ct[len(ct)-1] ^= 0x01
	if _, err := openKey(key, classDirect, 0, 0, dirStoR, recConfirm, 1, ct); err == nil {
		t.Fatal("flipped ciphertext opened")
	}
}

func TestDialPolicy(t *testing.T) {
	loop, _ := netip.ParseAddr("127.0.0.1")
	ll, _ := netip.ParseAddr("169.254.1.1")
	doc, _ := netip.ParseAddr("203.0.113.5")
	if allowAdvertise(loop) || allowAdvertise(ll) || !allowAdvertise(doc) {
		t.Fatal("advertise policy")
	}
	if dialable(cand{Proto: "udp", IP: "203.0.113.5", Port: 9}) {
		t.Fatal("udp is not a tcp candidate")
	}
	if dialable(cand{Proto: "tcp", IP: "127.0.0.1", Port: 9}) {
		t.Fatal("loopback dial")
	}
	if !dialable(cand{Proto: "tcp", IP: "203.0.113.5", Port: 9}) {
		t.Fatal("global tcp")
	}
}

func TestDirectMovesFiles(t *testing.T) {
	hereDir := newRun(t, goodID)
	toDir := newRun(t, goodID)
	putStage(t, toDir, "HANDOFF.md", "goal line\n")
	putStage(t, toDir, "AGENTS.md", "agents\n")
	putStage(t, toDir, ".cursor/rules/handoff.mdc", "rule\n")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	hereErr := make(chan error, 1)
	go func() {
		hereErr <- Run(ctx, Config{Role: "here", RunDir: hereDir, Direct: "127.0.0.1:0"})
	}()
	bound := waitExists(t, filepath.Join(hereDir, "event", "bound"), 5*time.Second)
	addr := strings.TrimSpace(strings.Split(string(bound), "\n")[0])
	toErr := make(chan error, 1)
	go func() {
		toErr <- Run(ctx, Config{Role: "to", RunDir: toDir, Direct: addr})
	}()
	driveYes(t, hereDir, toDir)
	if err := <-hereErr; err != nil {
		t.Fatal(err)
	}
	if err := <-toErr; err != nil {
		t.Fatal(err)
	}
	if resultOf(t, hereDir) != "done" || resultOf(t, toDir) != "done" {
		t.Fatalf("results %q %q", resultOf(t, hereDir), resultOf(t, toDir))
	}
	for _, name := range []string{"HANDOFF.md", "AGENTS.md", ".cursor/rules/handoff.mdc"} {
		want, err := os.ReadFile(filepath.Join(toDir, "stage", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(hereDir, "stage", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s mismatch", name)
		}
	}
	if !strings.Contains(string(waitExists(t, filepath.Join(hereDir, "event", "path"), time.Second)), "direct") {
		t.Fatal("path label")
	}
}

func TestIntroRelayMovesFiles(t *testing.T) {
	t.Setenv("OVERTO_SKIP_DIRECT", "1")
	srv, httpShow, _, _, err := ListenIntro("127.0.0.1:0", "", "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	intro := "http://" + httpShow
	hereDir := newRun(t, goodID)
	toDir := newRun(t, goodID)
	putStage(t, toDir, "HANDOFF.md", "via relay\n")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	hereErr := make(chan error, 1)
	go func() {
		hereErr <- Run(ctx, Config{Role: "here", RunDir: hereDir, Intro: intro})
	}()
	waitHealth(t, intro, 1)
	toErr := make(chan error, 1)
	go func() {
		toErr <- Run(ctx, Config{Role: "to", RunDir: toDir, Intro: intro})
	}()
	driveYes(t, hereDir, toDir)
	if err := <-hereErr; err != nil {
		t.Fatal(err)
	}
	if err := <-toErr; err != nil {
		t.Fatal(err)
	}
	if resultOf(t, hereDir) != "done" || resultOf(t, toDir) != "done" {
		t.Fatalf("results %q %q", resultOf(t, hereDir), resultOf(t, toDir))
	}
	got, err := os.ReadFile(filepath.Join(hereDir, "stage", "HANDOFF.md"))
	if err != nil || string(got) != "via relay\n" {
		t.Fatalf("file %q %v", got, err)
	}
	if !strings.Contains(string(waitExists(t, filepath.Join(hereDir, "event", "path"), time.Second)), "relay") {
		t.Fatal("expected relay")
	}
}

func TestBadPasswordSkipsRelay(t *testing.T) {
	srv, httpShow, _, relayShow, err := ListenIntro("127.0.0.1:0", "", "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	_, relayPort, err := net.SplitHostPort(relayShow)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var dialed []string
	prev := dialContext
	dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, addr)
		mu.Unlock()
		return prev(ctx, network, addr)
	}
	t.Cleanup(func() { dialContext = prev })
	intro := "http://" + httpShow
	hereDir := newRun(t, goodID)
	badID := "7k9qm3hf-4rv2-np8c-wxt3-f6dt-aj5m-eyb2qs"
	toDir := newRun(t, badID)
	putStage(t, toDir, "HANDOFF.md", "nope\n")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	hereErr := make(chan error, 1)
	go func() {
		hereErr <- Run(ctx, Config{Role: "here", RunDir: hereDir, Intro: intro})
	}()
	waitHealth(t, intro, 1)
	toErr := make(chan error, 1)
	go func() {
		toErr <- Run(ctx, Config{Role: "to", RunDir: toDir, Intro: intro})
	}()
	if err := <-hereErr; err != nil {
		t.Fatal(err)
	}
	if err := <-toErr; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(hereDir, "event", "sas")); !os.IsNotExist(err) {
		t.Fatal("sas was written")
	}
	if _, err := os.Stat(filepath.Join(toDir, "event", "sas")); !os.IsNotExist(err) {
		t.Fatal("sender sas was written")
	}
	if resultOf(t, hereDir) != "error 2 that id was rejected." {
		t.Fatalf("here %s", resultOf(t, hereDir))
	}
	if resultOf(t, toDir) != "error 2 that id was rejected." {
		t.Fatalf("to %s", resultOf(t, toDir))
	}
	mu.Lock()
	defer mu.Unlock()
	for _, addr := range dialed {
		_, port, err := net.SplitHostPort(addr)
		if err == nil && port == relayPort {
			t.Fatalf("dialed relay %s", addr)
		}
	}
}

func waitHealth(t *testing.T, intro string, n int) {
	t.Helper()
	dead := time.Now().Add(8 * time.Second)
	for time.Now().Before(dead) {
		resp, err := http.Get(intro + "/health")
		if err == nil {
			var body struct {
				Count int `json:"count"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			if body.Count >= n {
				return
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("intro did not register")
}

func p65() string {
	b := bytes.Repeat([]byte{0x04}, 65)
	return base64.StdEncoding.EncodeToString(b)
}

func plateN(n int) string {
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = alphabet[n%32]
		n /= 32
	}
	return string(b[:])
}

func doJSON(s *Server, method, target, ip, token string, body any) (int, []byte) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, []byte(err.Error())
		}
		rdr = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, rdr)
	req.RemoteAddr = ip + ":9"
	if token != "" {
		req.Header.Set("Overto-Token", token)
	}
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	return rr.Code, rr.Body.Bytes()
}

func regBody(plate string, ip string) map[string]any {
	return map[string]any{
		"nameplate":    plate,
		"version":      1,
		"candidates":   []cand{{Proto: "tcp", IP: ip, Port: 9}},
		"receiver_msg": p65(),
	}
}

func TestIntroHTTP(t *testing.T) {
	s := NewServer("127.0.0.1", 9)
	code, body := doJSON(s, http.MethodPost, "/v1/register", "203.0.113.10", "", regBody("7k9qm3hf", "203.0.113.20"))
	if code != http.StatusOK {
		t.Fatalf("register %d %s", code, body)
	}
	var reg struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &reg); err != nil || len(reg.Token) != 32 {
		t.Fatalf("token %s %v", body, err)
	}
	code, health := doJSON(s, http.MethodGet, "/health", "203.0.113.10", "", nil)
	if code != 200 || bytes.Contains(health, []byte("7k9qm3hf")) || !bytes.Contains(health, []byte(`"ok":true`)) {
		t.Fatalf("health %d %s", code, health)
	}
	code, _ = doJSON(s, http.MethodPost, "/v1/refresh", "203.0.113.10", "00112233445566778899aabbccddeeff", map[string]any{
		"nameplate": "7k9qm3hf", "candidates": []cand{{Proto: "tcp", IP: "203.0.113.21", Port: 9}},
	})
	if code != http.StatusNotFound {
		t.Fatalf("bad token %d", code)
	}
	code, refreshed := doJSON(s, http.MethodPost, "/v1/refresh", "203.0.113.10", reg.Token, map[string]any{
		"nameplate": "7k9qm3hf", "candidates": []cand{{Proto: "tcp", IP: "203.0.113.21", Port: 9}},
	})
	if code != http.StatusNoContent || len(bytes.TrimSpace(refreshed)) != 0 {
		t.Fatalf("refresh %d %s", code, refreshed)
	}
	code, approached := doJSON(s, http.MethodPost, "/v1/approach", "198.51.100.8", "", map[string]any{
		"nameplate": "7k9qm3hf", "version": 1,
		"candidates": []cand{{Proto: "tcp", IP: "203.0.113.30", Port: 9}},
		"sender_msg": p65(),
	})
	if code != http.StatusOK || !bytes.Contains(approached, []byte("203.0.113.21")) || bytes.Contains(approached, []byte("sender_msg")) {
		t.Fatalf("approach %d %s", code, approached)
	}
	code, busy := doJSON(s, http.MethodPost, "/v1/approach", "198.51.100.9", "", map[string]any{
		"nameplate": "7k9qm3hf", "version": 1,
		"candidates": []cand{{Proto: "tcp", IP: "203.0.113.31", Port: 9}},
		"sender_msg": p65(),
	})
	if code != http.StatusConflict || !bytes.Contains(busy, []byte("busy")) {
		t.Fatalf("busy %d %s", code, busy)
	}

	cA := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xab}, 32))
	other := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xcd}, 32))
	type httpResult struct {
		code int
		body []byte
	}
	first := make(chan httpResult, 1)
	go func() {
		code, body := doJSON(s, http.MethodPost, "/v1/confirm", "198.51.100.8", "", map[string]string{
			"nameplate": "7k9qm3hf", "c_a": cA,
		})
		first <- httpResult{code, body}
	}()
	dead := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		sl := s.slots["7k9qm3hf"]
		ready := sl != nil && len(sl.cA) == 32
		s.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(dead) {
			t.Fatal("c_a was not stored")
		}
		time.Sleep(5 * time.Millisecond)
	}
	code, conflict := doJSON(s, http.MethodPost, "/v1/confirm", "198.51.100.8", "", map[string]string{
		"nameplate": "7k9qm3hf", "c_a": other,
	})
	if code != http.StatusConflict {
		t.Fatalf("conflict %d %s", code, conflict)
	}
	same := make(chan httpResult, 1)
	go func() {
		code, body := doJSON(s, http.MethodPost, "/v1/confirm", "198.51.100.8", "", map[string]string{
			"nameplate": "7k9qm3hf", "c_a": cA,
		})
		same <- httpResult{code, body}
	}()
	cB := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, 32))
	code, _ = doJSON(s, http.MethodPost, "/v1/confirm-b", "203.0.113.10", reg.Token, map[string]string{
		"nameplate": "7k9qm3hf", "c_b": cB,
	})
	if code != http.StatusNoContent {
		t.Fatalf("confirm-b %d", code)
	}
	select {
	case got := <-first:
		if got.code != http.StatusOK || !bytes.Contains(got.body, []byte(cB)) {
			t.Fatalf("first confirm %d %s", got.code, got.body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first confirm hung")
	}
	select {
	case got := <-same:
		if got.code != http.StatusOK || !bytes.Contains(got.body, []byte(cB)) {
			t.Fatalf("same c_a %d %s", got.code, got.body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("same c_a hung")
	}
}

func TestIntroCapsAndTTL(t *testing.T) {
	s := NewServer("127.0.0.1", 9)
	for i := 0; i < regPerIP; i++ {
		code, body := doJSON(s, http.MethodPost, "/v1/register", "203.0.113.50", "", regBody(plateN(i+1), "203.0.113.20"))
		if code != http.StatusOK {
			t.Fatalf("reg %d: %d %s", i, code, body)
		}
	}
	code, body := doJSON(s, http.MethodPost, "/v1/register", "203.0.113.50", "", regBody(plateN(regPerIP+1), "203.0.113.20"))
	if code != http.StatusTooManyRequests {
		t.Fatalf("ninth %d %s", code, body)
	}

	aged := NewServer("127.0.0.1", 9)
	clock := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	aged.now = func() time.Time { return clock }
	code, body = doJSON(aged, http.MethodPost, "/v1/register", "203.0.113.60", "", regBody("abcdefgh", "203.0.113.20"))
	if code != http.StatusOK {
		t.Fatalf("clock reg %d %s", code, body)
	}
	var reg struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &reg); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(91 * time.Second)
	code, health := doJSON(aged, http.MethodGet, "/health", "203.0.113.60", "", nil)
	if code != 200 || !bytes.Contains(health, []byte(`"count":0`)) {
		t.Fatalf("ttl health %d %s", code, health)
	}
	code, _ = doJSON(aged, http.MethodPost, "/v1/refresh", "203.0.113.60", reg.Token, map[string]any{
		"nameplate": "abcdefgh", "candidates": []cand{{Proto: "tcp", IP: "203.0.113.21", Port: 9}},
	})
	if code != http.StatusNotFound {
		t.Fatalf("expired refresh %d", code)
	}

	full := NewServer("127.0.0.1", 9)
	for i := 0; i < regCap; i++ {
		ip := hostGroup(i / regPerIP)
		code, body := doJSON(full, http.MethodPost, "/v1/register", ip, "", regBody(plateN(i+100), "203.0.113.20"))
		if code != http.StatusOK {
			t.Fatalf("full %d: %d %s", i, code, body)
		}
	}
	code, body = doJSON(full, http.MethodPost, "/v1/register", "192.0.2.9", "", regBody(plateN(regCap+100), "203.0.113.20"))
	if code != http.StatusTooManyRequests {
		t.Fatalf("global %d %s", code, body)
	}
}

func hostGroup(g int) string {
	return "10." + itoa((g>>16)&0xff) + "." + itoa((g>>8)&0xff) + "." + itoa(g&0xff)
}

func TestMapAndRelay(t *testing.T) {
	cookie := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	v6 := &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 9999}
	raw := marshalMapped(cookie, v6)
	ip, port, err := parseMapped(raw, cookie)
	if err != nil || port != 9999 || !ip.Equal(net.ParseIP("2001:db8::1")) {
		t.Fatalf("v6 %v %s %d", err, ip, port)
	}
	if _, _, err := parseMapped(raw, []byte{9, 9, 9, 9, 9, 9, 9, 9}); err == nil {
		t.Fatal("stale cookie was accepted")
	}

	srv, _, udpShow, relayShow, err := ListenIntro("127.0.0.1:0", "", "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	dest, err := net.ResolveUDPAddr("udp", udpShow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.WriteToUDP(marshalMap(cookie), dest); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	gotIP, gotPort, err := parseMapped(buf[:n], cookie)
	if err != nil {
		t.Fatal(err)
	}
	local := conn.LocalAddr().(*net.UDPAddr)
	if gotPort != local.Port || !gotIP.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("mapped %s %d, local %s", gotIP, gotPort, local)
	}

	a, err := net.Dial("tcp", relayShow)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := net.Dial("tcp", relayShow)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	token := bytes.Repeat([]byte{0x5a}, 32)
	if err := writeOVR1(a, 0x01, token); err != nil {
		t.Fatal(err)
	}
	if err := writeOVR1(b, 0x02, token); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0xab}, 700*1024)
	_ = a.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = b.SetReadDeadline(time.Now().Add(5 * time.Second))
	writeErr := make(chan error, 1)
	go func() {
		_, err := a.Write(payload)
		writeErr <- err
	}()
	var got []byte
	bufR := make([]byte, 32*1024)
	for {
		n, err := b.Read(bufR)
		got = append(got, bufR[:n]...)
		if err != nil || len(got) > relayCeil {
			break
		}
	}
	if len(got) == 0 || len(got) > relayCeil {
		t.Fatalf("relay moved %d bytes", len(got))
	}
	select {
	case <-writeErr:
	case <-time.After(5 * time.Second):
		t.Fatal("relay writer hung")
	}
}

func TestNoCompiledIntroURL(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte("https://")) || bytes.Contains(b, []byte("overto.example")) {
			t.Errorf("%s contains a compiled introduction URL", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
