package pass

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRFC9382AppendixB(t *testing.T) {
	w := mustHex(t, "2ee57912099d31560b3a44b1184b9b4866e904c49d12ac5042c97dca461b1a5f")
	x := mustHex(t, "43dd0fd7215bdcb482879fca3220c6a968e66d70b1356cac18bb26c84a78d729")
	y := mustHex(t, "dcb60106f276b02606d8ef0a328c02e4b629f84f89786af5befb0bc75b6e66be")
	wantPA := mustHex(t, "04a56fa807caaa53a4d28dbb9853b9815c61a411118a6fe516a8798434751470f9010153ac33d0d5f2047ffdb1a3e42c9b4e6be662766e1eeb4116988ede5f912c")
	wantPB := mustHex(t, "0406557e482bd03097ad0cbaa5df82115460d951e3451962f1eaf4367a420676d09857ccbc522686c83d1852abfa8ed6e4a1155cf8f1543ceca528afb591a1e0b7")
	wantKPrefix := mustHex(t, "0412af7e89717850671913e6b469ace67bd90a4df8ce45c2af19010175e37eed69f75897996d539356e2fa6a406d528501f907e04d97515fbe83db277b715d3325")
	if len(wantPA) != 65 || len(wantPB) != 65 || len(wantKPrefix) != 65 {
		t.Fatalf("vector lengths pA=%d pB=%d K=%d", len(wantPA), len(wantPB), len(wantKPrefix))
	}

	pA, stA, err := BeginA("server", "client", w, x)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pA, wantPA) {
		t.Fatalf("pA\n got %x\nwant %x", pA, wantPA)
	}
	pB, stB, err := BeginB("server", "client", w, y)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pB, wantPB) {
		t.Fatalf("pB\n got %x\nwant %x", pB, wantPB)
	}
	keysA, err := stA.Finish(pB, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	keysB, err := stB.Finish(pA, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantKe := mustHex(t, "0e0672dc86f8e45565d338b0540abe69")
	wantKa := mustHex(t, "15bdf72e2b35b5c9e5663168e960a91b")
	wantCA := mustHex(t, "58ad4aa88e0b60d5061eb6b5dd93e80d9c4f00d127c65b3b35b1b5281fee38f0")
	wantCB := mustHex(t, "d3e2e547f1ae04f2dbdbf0fc4b79f8ecff2dff314b5d32fe9fcef2fb26dc459b")
	if !bytes.Equal(keysA.Ke, wantKe) || !bytes.Equal(keysB.Ke, wantKe) {
		t.Fatalf("Ke A %x B %x", keysA.Ke, keysB.Ke)
	}
	if !bytes.Equal(keysA.Ka, wantKa) || !bytes.Equal(keysB.Ka, wantKa) {
		t.Fatalf("Ka A %x B %x", keysA.Ka, keysB.Ka)
	}
	if !bytes.Equal(keysA.CA, wantCA) || !bytes.Equal(keysB.CA, wantCA) {
		t.Fatalf("cA\n got %x\nwant %x", keysA.CA, wantCA)
	}
	if !bytes.Equal(keysA.CB, wantCB) || !bytes.Equal(keysB.CB, wantCB) {
		t.Fatalf("cB\n got %x\nwant %x", keysA.CB, wantCB)
	}
	_ = wantKPrefix
}

func TestHashToScalarAndNameplate(t *testing.T) {
	pass := []byte("4rv2np8cwxt3f6dtaj5meyb2qr")
	plate := []byte("7k9qm3hf")
	w1, err := HashToScalar(pass, plate)
	if err != nil {
		t.Fatal(err)
	}
	w2, err := HashToScalar(pass, plate)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w1, w2) || len(w1) != 32 {
		t.Fatalf("unstable scalar %x", w1)
	}
	w3, err := HashToScalar(pass, []byte("4rv2np8c"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(w1, w3) {
		t.Fatal("nameplate did not change w")
	}
	want := mustHex(t, "3a23eb4dde909e71e978154f26b5681609ef207d56fb48d62bcd3d0de3c18f40")
	if !bytes.Equal(w1, want) {
		t.Fatalf("w %x", w1)
	}

	ke := bytes.Repeat([]byte{0x42}, 16)
	k1 := &Keys{Ke: ke}
	k2 := &Keys{Ke: append([]byte(nil), ke...)}
	if err := k1.derive(plate); err != nil {
		t.Fatal(err)
	}
	if err := k2.derive([]byte("4rv2np8c")); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(k1.Send, k2.Send) || bytes.Equal(k1.Relay, k2.Relay) || k1.SAS == k2.SAS {
		t.Fatal("two nameplates agreed")
	}
	if len(k1.SAS) != 6 {
		t.Fatalf("sas %q", k1.SAS)
	}
}

func TestEmptyIdentityVector(t *testing.T) {
	// RFC 9382 appendix B, A empty, B "client". Confirms the length prefix of an empty identity.
	w := mustHex(t, "0548d8729f730589e579b0475a582c1608138ddf7054b73b5381c7e883e2efae")
	x := mustHex(t, "403abbe3b1b4b9ba17e3032849759d723939a27a27b9d921c500edde18ed654b")
	y := mustHex(t, "903023b6598908936ea7c929bd761af6039577a9c3f9581064187c3049d87065")
	pA, stA, err := BeginA("", "client", w, x)
	if err != nil {
		t.Fatal(err)
	}
	pB, stB, err := BeginB("", "client", w, y)
	if err != nil {
		t.Fatal(err)
	}
	keysA, err := stA.Finish(pB, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	keysB, err := stB.Finish(pA, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantKe := mustHex(t, "642f05c473c2cd79909f9a841e2f30a7")
	wantKa := mustHex(t, "0bf89b18180af97353ba198789c2b963")
	if !bytes.Equal(keysA.Ke, wantKe) || !bytes.Equal(keysB.Ke, wantKe) {
		t.Fatalf("Ke %x", keysA.Ke)
	}
	if !bytes.Equal(keysA.Ka, wantKa) {
		t.Fatalf("Ka %x", keysA.Ka)
	}
	if !bytes.Equal(keysA.CA, keysB.CA) || !bytes.Equal(keysA.CB, keysB.CB) {
		t.Fatal("sides disagree")
	}
}
