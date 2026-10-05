package pass

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
)

const (
	frameAAD    = "overto-frame-v1"
	maxPlain    = 262144
	maxCipher   = maxPlain + 16
	maxFiles    = 4
	maxTotal    = 524288
	classLAN    = 0x01
	classPunch  = 0x02
	classMap    = 0x03
	classRelay  = 0x04
	classDirect = 0x05

	recConfirm  byte = 1
	recManifest byte = 2
	recFile     byte = 3
	recAccept   byte = 4
	recDecline  byte = 5
	recBusy     byte = 6

	dirStoR byte = 0x01
	dirRtoS byte = 0x02
)

var (
	sealMu   sync.Mutex
	sealed   = map[string]struct{}{}
	errReuse = errors.New("nonce reused")
)

func noteSeal(key, nonce []byte) error {
	id := string(key) + "\x00" + string(nonce)
	sealMu.Lock()
	defer sealMu.Unlock()
	if _, ok := sealed[id]; ok {
		return errReuse
	}
	sealed[id] = struct{}{}
	return nil
}

func resetSeals() {
	sealMu.Lock()
	sealed = map[string]struct{}{}
	sealMu.Unlock()
}

// Path is one GCM direction pair. outDir is the direction this side seals.
type Path struct {
	class   byte
	attempt uint16
	dialer  byte
	outKey  []byte
	inKey   []byte
	outDir  byte
	inDir   byte
	outNext uint64
	inNext  uint64
}

func derivePath(sendKey, recvKey []byte, class byte, attempt uint16, dialer byte, sender bool) (*Path, error) {
	ps, err := pathKey(sendKey, class, attempt, dialer)
	if err != nil {
		return nil, err
	}
	pr, err := pathKey(recvKey, class, attempt, dialer)
	if err != nil {
		return nil, err
	}
	p := &Path{class: class, attempt: attempt, dialer: dialer, outNext: 1, inNext: 1}
	if sender {
		p.outKey, p.inKey = ps, pr
		p.outDir, p.inDir = dirStoR, dirRtoS
	} else {
		p.outKey, p.inKey = pr, ps
		p.outDir, p.inDir = dirRtoS, dirStoR
	}
	return p, nil
}

func pathKey(ikm []byte, class byte, attempt uint16, dialer byte) ([]byte, error) {
	info := make([]byte, 0, 20)
	info = append(info, "overto-path-v1"...)
	info = append(info, class)
	info = append(info, byte(attempt>>8), byte(attempt))
	info = append(info, dialer)
	return hkdf.Key(sha256.New, ikm, nil, string(info), 32)
}

func (p *Path) seal(typ byte, pt []byte) (ct []byte, counter uint64, err error) {
	if len(pt) > maxPlain {
		return nil, 0, errors.New("record too big")
	}
	counter = p.outNext
	ct, err = sealKey(p.outKey, p.class, p.attempt, p.dialer, p.outDir, typ, counter, pt)
	if err != nil {
		return nil, 0, err
	}
	p.outNext++
	return ct, counter, nil
}

func (p *Path) open(typ byte, counter uint64, ct []byte) ([]byte, error) {
	if counter != p.inNext {
		return nil, errors.New("bad counter")
	}
	pt, err := openKey(p.inKey, p.class, p.attempt, p.dialer, p.inDir, typ, counter, ct)
	if err != nil {
		return nil, err
	}
	p.inNext++
	return pt, nil
}

func sealKey(key []byte, class byte, attempt uint16, dialer, dir, typ byte, counter uint64, pt []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := nonceFor(counter)
	if err := noteSeal(key, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, pt, frameAADBytes(class, attempt, dialer, dir, typ, counter)), nil
}

func openKey(key []byte, class byte, attempt uint16, dialer, dir, typ byte, counter uint64, ct []byte) ([]byte, error) {
	if len(ct) > maxCipher {
		return nil, errors.New("ciphertext too big")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonceFor(counter), ct, frameAADBytes(class, attempt, dialer, dir, typ, counter))
}

func nonceFor(counter uint64) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], counter)
	return nonce
}

func frameAADBytes(class byte, attempt uint16, dialer, dir, typ byte, counter uint64) []byte {
	aad := make([]byte, 0, 15+1+2+1+1+1+8)
	aad = append(aad, frameAAD...)
	aad = append(aad, class)
	aad = append(aad, byte(attempt>>8), byte(attempt))
	aad = append(aad, dialer, dir, typ)
	var c [8]byte
	binary.BigEndian.PutUint64(c[:], counter)
	return append(aad, c[:]...)
}
