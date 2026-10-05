package pass

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

func writeFrame(w io.Writer, typ byte, counter uint64, ct []byte) error {
	if len(ct) > maxCipher {
		return errors.New("ciphertext too big")
	}
	var hdr [13]byte
	hdr[0] = typ
	binary.BigEndian.PutUint64(hdr[1:9], counter)
	binary.BigEndian.PutUint32(hdr[9:13], uint32(len(ct)))
	buf := make([]byte, 0, len(hdr)+len(ct))
	buf = append(buf, hdr[:]...)
	buf = append(buf, ct...)
	_, err := w.Write(buf)
	return err
}

func readFrame(r io.Reader) (typ byte, counter uint64, ct []byte, err error) {
	var hdr [13]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return 0, 0, nil, err
	}
	n := binary.BigEndian.Uint32(hdr[9:13])
	if n > maxCipher {
		return 0, 0, nil, errors.New("ciphertext too big")
	}
	ct = make([]byte, n)
	if _, err = io.ReadFull(r, ct); err != nil {
		return 0, 0, nil, err
	}
	return hdr[0], binary.BigEndian.Uint64(hdr[1:9]), ct, nil
}

func writeRec(conn net.Conn, p *Path, typ byte, pt []byte) error {
	ct, counter, err := p.seal(typ, pt)
	if err != nil {
		return err
	}
	return writeFrame(conn, typ, counter, ct)
}

func readRec(conn net.Conn, p *Path) (byte, []byte, error) {
	typ, counter, ct, err := readFrame(conn)
	if err != nil {
		return 0, nil, err
	}
	pt, err := p.open(typ, counter, ct)
	if err != nil {
		return 0, nil, err
	}
	return typ, pt, nil
}

func writeOVD1(w io.Writer, pA []byte) error {
	if len(pA) != 65 {
		return errors.New("bad point")
	}
	buf := make([]byte, 0, 70)
	buf = append(buf, 'O', 'V', 'D', '1', 1)
	buf = append(buf, pA...)
	_, err := w.Write(buf)
	return err
}

func readOVD1(r io.Reader) ([]byte, error) {
	buf := make([]byte, 70)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if string(buf[:4]) != "OVD1" {
		return nil, errVersion()
	}
	if buf[4] != 1 {
		return nil, errVersion()
	}
	return append([]byte(nil), buf[5:]...), nil
}

func readExact(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func ovt1MAC(punch, head []byte) []byte {
	m := hmac.New(sha256.New, punch)
	m.Write(head)
	m.Write([]byte("tcp"))
	return m.Sum(nil)[:16]
}

func writeOVT1(w io.Writer, punch, nameplate []byte, class byte, attempt uint16, role byte) error {
	if len(nameplate) != 8 || len(punch) != 32 {
		return errors.New("bad preamble")
	}
	head := make([]byte, 17)
	copy(head, "OVT1")
	head[4] = 1
	head[5] = class
	binary.BigEndian.PutUint16(head[6:8], attempt)
	head[8] = role
	copy(head[9:17], nameplate)
	buf := append(head, ovt1MAC(punch, head)...)
	_, err := w.Write(buf)
	return err
}

func readOVT1(r io.Reader, punch []byte) (class byte, attempt uint16, role byte, plate []byte, err error) {
	buf, err := readExact(r, 33)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	if string(buf[:4]) != "OVT1" || buf[4] != 1 {
		return 0, 0, 0, nil, errVersion()
	}
	sum := ovt1MAC(punch, buf[:17])
	if subtle.ConstantTimeCompare(sum, buf[17:]) != 1 {
		return 0, 0, 0, nil, errRejected()
	}
	return buf[5], binary.BigEndian.Uint16(buf[6:8]), buf[8], append([]byte(nil), buf[9:17]...), nil
}

func writeOVR1(w io.Writer, role byte, token []byte) error {
	if len(token) != 32 {
		return errors.New("bad relay token")
	}
	buf := make([]byte, 0, 38)
	buf = append(buf, 'O', 'V', 'R', '1', 1, role)
	buf = append(buf, token...)
	_, err := w.Write(buf)
	return err
}

func readOVR1(r io.Reader) (role byte, token []byte, err error) {
	buf, err := readExact(r, 38)
	if err != nil {
		return 0, nil, err
	}
	if string(buf[:4]) != "OVR1" || buf[4] != 1 {
		return 0, nil, errVersion()
	}
	return buf[5], append([]byte(nil), buf[6:]...), nil
}

func marshalMap(cookie []byte) []byte {
	b := make([]byte, 14)
	copy(b, "OVMP")
	b[4] = 1
	b[5] = 1
	copy(b[6:14], cookie)
	return b
}

func marshalMapped(cookie []byte, addr *net.UDPAddr) []byte {
	ip := addr.IP
	fam := byte(6)
	raw := ip.To16()
	if v4 := ip.To4(); v4 != nil {
		fam = 4
		raw = v4
	}
	b := make([]byte, 17+len(raw))
	copy(b, "OVMP")
	b[4] = 1
	b[5] = 2
	copy(b[6:14], cookie)
	b[14] = fam
	binary.BigEndian.PutUint16(b[15:17], uint16(addr.Port))
	copy(b[17:], raw)
	return b
}

func parseMapped(b, cookie []byte) (net.IP, int, error) {
	if len(b) < 17 || string(b[:4]) != "OVMP" || b[4] != 1 || b[5] != 2 {
		return nil, 0, errors.New("bad mapped")
	}
	if subtle.ConstantTimeCompare(b[6:14], cookie) != 1 {
		return nil, 0, errors.New("stale cookie")
	}
	port := int(binary.BigEndian.Uint16(b[15:17]))
	switch b[14] {
	case 4:
		if len(b) < 21 {
			return nil, 0, errors.New("short map")
		}
		return append(net.IP(nil), b[17:21]...), port, nil
	case 6:
		if len(b) < 33 {
			return nil, 0, errors.New("short map")
		}
		return append(net.IP(nil), b[17:33]...), port, nil
	default:
		return nil, 0, errors.New("bad family")
	}
}

var allowNames = []string{
	"HANDOFF.md",
	"AGENTS.md",
	"CLAUDE.md",
	".cursor/rules/handoff.mdc",
}

func allowedName(name string) bool {
	for _, n := range allowNames {
		if name == n {
			return true
		}
	}
	return false
}

type entry struct {
	Name string
	Body []byte
	Size uint32
	Sum  [32]byte
}

func encodeManifest(files []entry) ([]byte, error) {
	if len(files) < 1 || len(files) > maxFiles {
		return nil, errStopped()
	}
	b := []byte{1, byte(len(files))}
	var total int
	for _, f := range files {
		if !allowedName(f.Name) || len(f.Name) > 64 {
			return nil, errStopped()
		}
		if int(f.Size) > maxPlain || len(f.Body) != int(f.Size) {
			return nil, errTooBig(f.Name)
		}
		total += int(f.Size)
		b = append(b, byte(len(f.Name)))
		b = append(b, f.Name...)
		var sz [4]byte
		binary.BigEndian.PutUint32(sz[:], f.Size)
		b = append(b, sz[:]...)
		b = append(b, f.Sum[:]...)
	}
	if total > maxTotal {
		return nil, exitCode(1, "the handoff is too big.")
	}
	return b, nil
}

func decodeManifest(b []byte) ([]entry, error) {
	if len(b) < 2 || b[0] != 1 {
		return nil, errStopped()
	}
	n := int(b[1])
	if n < 1 || n > maxFiles {
		return nil, errStopped()
	}
	b = b[2:]
	out := make([]entry, 0, n)
	var total int
	for i := 0; i < n; i++ {
		if len(b) < 1 {
			return nil, errStopped()
		}
		nl := int(b[0])
		b = b[1:]
		if nl < 1 || nl > 64 || len(b) < nl+4+32 {
			return nil, errStopped()
		}
		name := string(b[:nl])
		b = b[nl:]
		if !allowedName(name) {
			return nil, errStopped()
		}
		sz := binary.BigEndian.Uint32(b[:4])
		b = b[4:]
		if sz > maxPlain {
			return nil, errTooBig(name)
		}
		var sum [32]byte
		copy(sum[:], b[:32])
		b = b[32:]
		total += int(sz)
		out = append(out, entry{Name: name, Size: sz, Sum: sum})
	}
	if len(b) != 0 || total > maxTotal {
		return nil, errStopped()
	}
	return out, nil
}
