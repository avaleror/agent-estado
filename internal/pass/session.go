package pass

import (
	"crypto/sha256"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (r *runDir) loadStage() ([]entry, error) {
	var out []entry
	var total int
	for _, name := range allowNames {
		p := filepath.Join(r.dir, "stage", filepath.FromSlash(name))
		fi, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
			return nil, errStopped()
		}
		if fi.Size() > maxPlain {
			return nil, errTooBig(name)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		total += len(body)
		if total > maxTotal {
			return nil, exitCode(1, "the handoff is too big.")
		}
		out = append(out, entry{Name: name, Body: body, Size: uint32(len(body)), Sum: sha256.Sum256(body)})
	}
	if len(out) == 0 {
		return nil, errStopped()
	}
	return out, nil
}

func (r *runDir) saveStage(files []entry) error {
	for _, f := range files {
		sum := sha256.Sum256(f.Body)
		if !allowedName(f.Name) || len(f.Body) != int(f.Size) || sum != f.Sum {
			return errStopped()
		}
		if f.Name == ".cursor/rules/handoff.mdc" {
			base := filepath.Join(r.dir, "stage")
			if err := os.Mkdir(filepath.Join(base, ".cursor"), 0o700); err != nil && !os.IsExist(err) {
				return err
			}
			if err := os.Mkdir(filepath.Join(base, ".cursor", "rules"), 0o700); err != nil && !os.IsExist(err) {
				return err
			}
		} else if strings.Contains(f.Name, "/") {
			return errStopped()
		}
		dest := filepath.Join(r.dir, "stage", filepath.FromSlash(f.Name))
		if err := writePriv(dest, f.Body); err != nil {
			return err
		}
	}
	var b strings.Builder
	b.WriteString("set\n")
	for _, f := range files {
		b.WriteString(f.Name)
		b.WriteByte(' ')
		b.WriteString(itoa(int(f.Size)))
		b.WriteByte(' ')
		b.WriteString(hexSum(f.Sum[:]))
		b.WriteByte('\n')
	}
	if err := r.event("manifest", strings.TrimRight(b.String(), "\n")); err != nil {
		return err
	}
	return r.event("staged", "")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d [16]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	return string(d[i:])
}

func hexSum(b []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = h[c>>4]
		out[i*2+1] = h[c&0x0f]
	}
	return string(out)
}

type peerRec struct {
	typ byte
	pt  []byte
	err error
}

func runSession(conn net.Conn, role string, path *Path, label string, run *runDir, keys *Keys, files []entry) error {
	if err := run.event("path", label); err != nil {
		return err
	}
	if err := run.event("sas", keys.SAS); err != nil {
		return err
	}
	peerCh := make(chan peerRec, 1)
	go func() {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		typ, pt, err := readRec(conn, path)
		peerCh <- peerRec{typ, pt, err}
	}()
	ans, err := run.waitCmd("sas", 5*time.Minute)
	if err != nil {
		return err
	}
	if ans != "yes" {
		_ = writeRec(conn, path, recDecline, nil)
		return errDeclined()
	}
	if err := writeRec(conn, path, recAccept, nil); err != nil {
		return errStopped()
	}
	peer := <-peerCh
	if peer.err != nil {
		return errStopped()
	}
	switch peer.typ {
	case recDecline:
		return errDeclined()
	case recBusy:
		return exitCode(2, "busy")
	case recAccept:
	default:
		return errStopped()
	}
	if role == "to" {
		return sendFiles(conn, path, files)
	}
	return recvFiles(conn, path, run)
}

func sendFiles(conn net.Conn, path *Path, files []entry) error {
	body, err := encodeManifest(files)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := writeRec(conn, path, recManifest, body); err != nil {
		return errStopped()
	}
	for _, f := range files {
		if err := writeRec(conn, path, recFile, f.Body); err != nil {
			return errStopped()
		}
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	typ, _, err := readRec(conn, path)
	if err != nil {
		return errStopped()
	}
	switch typ {
	case recAccept:
		return nil
	case recDecline:
		return errDeclined()
	case recBusy:
		return exitCode(2, "busy")
	default:
		return errStopped()
	}
}

func recvFiles(conn net.Conn, path *Path, run *runDir) error {
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	typ, pt, err := readRec(conn, path)
	if err != nil || typ != recManifest {
		if typ == recDecline {
			return errDeclined()
		}
		return errStopped()
	}
	files, err := decodeManifest(pt)
	if err != nil {
		return err
	}
	for i := range files {
		typ, body, err := readRec(conn, path)
		if err != nil || typ != recFile {
			return errStopped()
		}
		if len(body) != int(files[i].Size) || sha256.Sum256(body) != files[i].Sum {
			return errStopped()
		}
		files[i].Body = body
	}
	if err := run.saveStage(files); err != nil {
		return err
	}
	ans, err := run.waitCmd("files", 5*time.Minute)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if ans != "yes" {
		_ = writeRec(conn, path, recDecline, nil)
		return errDeclined()
	}
	if err := writeRec(conn, path, recAccept, nil); err != nil {
		return errStopped()
	}
	return nil
}

func exchangeType1(conn net.Conn, path *Path, sender bool) error {
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{})
	if sender {
		if err := writeRec(conn, path, recConfirm, nil); err != nil {
			return err
		}
		typ, _, err := readRec(conn, path)
		if err != nil || typ != recConfirm {
			return errStopped()
		}
		return nil
	}
	typ, _, err := readRec(conn, path)
	if err != nil || typ != recConfirm {
		return errStopped()
	}
	if err := writeRec(conn, path, recConfirm, nil); err != nil {
		return err
	}
	return nil
}
