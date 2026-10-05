package pass

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"filippo.io/bigmod"
)

// P-256 group order, and 2^256 mod n. A 40-byte HKDF output does not fit in
// SetOverflowingBytes, so w = hi*(2^256 mod n) + lo.
var (
	p256OrderHex  = "ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551"
	two256ModNHex = "00000000ffffffff00000000000000004319055258e8617b0c46353d039cdaaf"
)

func p256Modulus() (*bigmod.Modulus, error) {
	b, err := hex.DecodeString(p256OrderHex)
	if err != nil {
		return nil, err
	}
	return bigmod.NewModulus(b)
}

func two256ModN(m *bigmod.Modulus) (*bigmod.Nat, error) {
	b, err := hex.DecodeString(two256ModNHex)
	if err != nil {
		return nil, err
	}
	n := bigmod.NewNat()
	if _, err := n.SetBytes(b, m); err != nil {
		return nil, err
	}
	return n, nil
}

// reduceWide reduces a 40-byte big-endian integer modulo the P-256 order.
// It returns 32 bytes. A zero result is an error.
func reduceWide(wide []byte) ([]byte, error) {
	if len(wide) != 40 {
		return nil, errors.New("scalar width")
	}
	m, err := p256Modulus()
	if err != nil {
		return nil, err
	}
	r256, err := two256ModN(m)
	if err != nil {
		return nil, err
	}
	hiBytes := make([]byte, 32)
	copy(hiBytes[24:], wide[:8])
	hi := bigmod.NewNat()
	if _, err := hi.SetBytes(hiBytes, m); err != nil {
		return nil, err
	}
	hi.Mul(r256, m)

	lo := bigmod.NewNat()
	if _, err := lo.SetOverflowingBytes(wide[8:], m); err != nil {
		return nil, err
	}
	hi.Add(lo, m)
	if hi.IsZero() == 1 {
		return nil, errors.New("scalar is zero")
	}
	return hi.Bytes(m), nil
}

// randomScalar is uniform in [0, n). Values that are not strictly smaller
// than n are drawn again.
func randomScalar() ([]byte, error) {
	m, err := p256Modulus()
	if err != nil {
		return nil, err
	}
	for i := 0; i < 128; i++ {
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, err
		}
		n := bigmod.NewNat()
		if _, err := n.SetBytes(b[:], m); err != nil {
			continue
		}
		return n.Bytes(m), nil
	}
	return nil, errors.New("no scalar")
}
