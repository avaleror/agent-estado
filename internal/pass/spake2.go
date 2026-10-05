package pass

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"filippo.io/nistec"
)

const (
	idSender   = "overto-sender-v1"
	idReceiver = "overto-receiver-v1"
)

// RFC 9382 section 6, compressed SEC1.
var (
	mCompressed = "02886e2f97ace46e55ba9dd7242579f2993b64e16ef3dcab95afd497333d8fa12f"
	nCompressed = "03d8bbd6c639c62937b04d997f38c3770719c629d7014d49a24b4f98baa1292b49"
)

func fixedPoint(h string) *nistec.P256Point {
	b, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	p, err := loadPoint(b)
	if err != nil {
		panic(err)
	}
	return p
}

func loadPoint(b []byte) (*nistec.P256Point, error) {
	p := nistec.NewP256Point()
	if _, err := p.SetBytes(b); err != nil {
		return nil, err
	}
	if p.IsInfinity() == 1 {
		return nil, errors.New("identity point")
	}
	return p, nil
}

func pointBytes(p *nistec.P256Point) ([]byte, error) {
	if p.IsInfinity() == 1 {
		return nil, errors.New("identity point")
	}
	raw := p.Bytes()
	if len(raw) != 65 || raw[0] != 0x04 {
		return nil, errors.New("bad point encoding")
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

func scalarMult(q *nistec.P256Point, k []byte) (*nistec.P256Point, error) {
	r := nistec.NewP256Point()
	if _, err := r.ScalarMult(q, k); err != nil {
		return nil, err
	}
	if r.IsInfinity() == 1 {
		return nil, errors.New("identity point")
	}
	return r, nil
}

func baseMult(k []byte) (*nistec.P256Point, error) {
	r := nistec.NewP256Point()
	if _, err := r.ScalarBaseMult(k); err != nil {
		return nil, err
	}
	if r.IsInfinity() == 1 {
		return nil, errors.New("identity point")
	}
	return r, nil
}

// HashToScalar is HKDF-SHA256(password, salt=nameplate, info="overto-pass-1 w", L=40),
// reduced modulo the P-256 order.
func HashToScalar(password, nameplate []byte) ([]byte, error) {
	wide, err := hkdf.Key(sha256.New, password, nameplate, "overto-pass-1 w", 40)
	if err != nil {
		return nil, err
	}
	w, err := reduceWide(wide)
	for i := range wide {
		wide[i] = 0
	}
	return w, err
}

type sideState struct {
	role byte // 'A' or 'B'
	a, b string
	w, k []byte // scalar and secret scalar (x or y)
	mine *nistec.P256Point
	wFix *nistec.P256Point // w*M for A, w*N for B
}

func start(role byte, idA, idB string, w, secret []byte) ([]byte, *sideState, error) {
	if len(w) != 32 || len(secret) != 32 {
		return nil, nil, errors.New("scalar length")
	}
	M := fixedPoint(mCompressed)
	N := fixedPoint(nCompressed)
	st := &sideState{role: role, a: idA, b: idB, w: append([]byte(nil), w...), k: append([]byte(nil), secret...)}
	// pA = w*M + x*P, and A later subtracts w*N. pB is the mirror.
	var share *nistec.P256Point
	var err error
	if role == 'A' {
		share, err = scalarMult(M, w)
		if err != nil {
			return nil, nil, err
		}
		st.wFix, err = scalarMult(N, w)
	} else {
		share, err = scalarMult(N, w)
		if err != nil {
			return nil, nil, err
		}
		st.wFix, err = scalarMult(M, w)
	}
	if err != nil {
		return nil, nil, err
	}
	base, err := baseMult(secret)
	if err != nil {
		return nil, nil, err
	}
	st.mine = nistec.NewP256Point().Add(share, base)
	pb, err := pointBytes(st.mine)
	if err != nil {
		return nil, nil, err
	}
	return pb, st, nil
}

// Keys are the session secrets. Send is the file-sender's encrypt key.
type Keys struct {
	Ke, Ka     []byte
	CA, CB     []byte
	Send, Recv []byte
	Relay      []byte
	SASKey     []byte
	Punch      []byte
	SAS        string
}

func (st *sideState) finish(peer []byte, aad, nameplate []byte) (*Keys, error) {
	defer func() {
		for i := range st.w {
			st.w[i] = 0
		}
		for i := range st.k {
			st.k[i] = 0
		}
	}()
	other, err := loadPoint(peer)
	if err != nil {
		return nil, err
	}
	neg := nistec.NewP256Point().Negate(st.wFix)
	diff := nistec.NewP256Point().Add(other, neg)
	// h = 1 on P-256. Still one multiplication, as the RFC writes it.
	K, err := scalarMult(diff, st.k)
	if err != nil {
		return nil, err
	}
	kb, err := pointBytes(K)
	if err != nil {
		return nil, err
	}
	mine, err := pointBytes(st.mine)
	if err != nil {
		return nil, err
	}
	var pA, pB []byte
	if st.role == 'A' {
		pA, pB = mine, append([]byte(nil), peer...)
	} else {
		pA, pB = append([]byte(nil), peer...), mine
	}
	tt := transcript(st.a, st.b, pA, pB, kb, st.w)
	sum := sha256.Sum256(tt)
	ke := append([]byte(nil), sum[:16]...)
	ka := append([]byte(nil), sum[16:]...)
	cA, cB, err := confirmation(ka, tt, aad)
	if err != nil {
		return nil, err
	}
	keys := &Keys{Ke: ke, Ka: ka, CA: cA, CB: cB}
	if nameplate != nil {
		if err := keys.derive(nameplate); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

func transcript(idA, idB string, pA, pB, K, w []byte) []byte {
	var tt []byte
	tt = appendLen(tt, []byte(idA))
	tt = appendLen(tt, []byte(idB))
	tt = appendLen(tt, pA)
	tt = appendLen(tt, pB)
	tt = appendLen(tt, K)
	tt = appendLen(tt, w)
	return tt
}

func appendLen(dst, b []byte) []byte {
	var l [8]byte
	binary.LittleEndian.PutUint64(l[:], uint64(len(b)))
	dst = append(dst, l[:]...)
	return append(dst, b...)
}

func confirmation(ka, tt, aad []byte) (cA, cB []byte, err error) {
	info := []byte("ConfirmationKeys")
	info = append(info, aad...)
	okm, err := hkdf.Key(sha256.New, ka, nil, string(info), 32)
	if err != nil {
		return nil, nil, err
	}
	kcA, kcB := okm[:16], okm[16:]
	cA = macTT(kcA, tt)
	cB = macTT(kcB, tt)
	return cA, cB, nil
}

func macTT(key, tt []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(tt)
	return m.Sum(nil)
}

func overtoAAD(nameplate []byte) []byte {
	aad := make([]byte, 0, len("overto-pass-1")+1+len(nameplate))
	aad = append(aad, "overto-pass-1"...)
	aad = append(aad, 0)
	aad = append(aad, nameplate...)
	return aad
}

func (k *Keys) derive(nameplate []byte) error {
	info := make([]byte, 0, 64)
	info = append(info, "overto-pass-1"...)
	info = append(info, 0)
	info = append(info, nameplate...)
	info = append(info, 0)
	info = append(info, idSender...)
	info = append(info, 0)
	info = append(info, idReceiver...)
	okm, err := hkdf.Key(sha256.New, k.Ke, nil, string(info), 128)
	if err != nil {
		return err
	}
	k.Send = okm[0:32]
	k.Recv = okm[32:64]
	k.Relay = okm[64:96]
	k.SASKey = okm[96:128]
	pinfo := append([]byte("overto-punch-v1"), nameplate...)
	punch, err := hkdf.Key(sha256.New, k.Ke, nil, string(pinfo), 32)
	if err != nil {
		return err
	}
	k.Punch = punch
	mac := hmac.New(sha256.New, k.SASKey)
	mac.Write([]byte("sas"))
	sum := mac.Sum(nil)
	n := binary.BigEndian.Uint32(sum[:4]) % 1000000
	k.SAS = pad6(n)
	return nil
}

func pad6(n uint32) string {
	var b [6]byte
	for i := 5; i >= 0; i-- {
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[:])
}

// BeginA starts a SPAKE2 side. secret nil draws x at random.
func BeginA(idA, idB string, w, secret []byte) ([]byte, *sideState, error) {
	var err error
	if secret == nil {
		secret, err = randomScalar()
		if err != nil {
			return nil, nil, err
		}
	}
	return start('A', idA, idB, w, secret)
}

// BeginB starts the receiver side. secret nil draws y at random.
func BeginB(idA, idB string, w, secret []byte) ([]byte, *sideState, error) {
	var err error
	if secret == nil {
		secret, err = randomScalar()
		if err != nil {
			return nil, nil, err
		}
	}
	return start('B', idA, idB, w, secret)
}

func (st *sideState) Finish(peer, aad, nameplate []byte) (*Keys, error) {
	return st.finish(peer, aad, nameplate)
}

// overtoBegin runs hash-to-scalar and starts one side of an overto session.
func overtoBegin(role byte, password, nameplate []byte) ([]byte, *sideState, error) {
	w, err := HashToScalar(password, nameplate)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		for i := range w {
			w[i] = 0
		}
	}()
	if role == 'A' {
		return BeginA(idSender, idReceiver, w, nil)
	}
	return BeginB(idSender, idReceiver, w, nil)
}

func (st *sideState) finishOverto(peer, nameplate []byte) (*Keys, error) {
	return st.finish(peer, overtoAAD(nameplate), nameplate)
}
