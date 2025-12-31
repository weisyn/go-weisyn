package methods

import (
	"bytes"
	"testing"
)

func TestNonceManagerNextPerAddress(t *testing.T) {
	manager := NewNonceManager()

	addrA := bytes.Repeat([]byte{0xAA}, 20)
	addrB := bytes.Repeat([]byte{0xBB}, 20)

	nonceA1 := manager.Next(addrA)
	nonceA2 := manager.Next(addrA)
	nonceB1 := manager.Next(addrB)

	if bytes.Equal(nonceA1, nonceA2) {
		t.Fatalf("expected different nonces for successive calls, got identical values")
	}
	if bytes.Equal(nonceA1, nonceB1) {
		t.Fatalf("expected different nonces for different addresses")
	}
	if len(nonceA1) != 32 || len(nonceA2) != 32 || len(nonceB1) != 32 {
		t.Fatalf("nonce length should be 32 bytes")
	}
}

func TestDeriveInputNonce(t *testing.T) {
	manager := NewNonceManager()
	base := manager.Next(bytes.Repeat([]byte{0xCC}, 20))

	nonce0 := deriveInputNonce(base, 0)
	nonce1 := deriveInputNonce(base, 1)

	if !bytes.Equal(nonce0, base) {
		t.Fatalf("index 0 should reuse base nonce")
	}
	if bytes.Equal(nonce0, nonce1) {
		t.Fatalf("different indices should yield different nonces")
	}
	if len(nonce1) != 32 {
		t.Fatalf("derived nonce length should be 32 bytes")
	}
}
