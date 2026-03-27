package audit

import (
	"bytes"
	"strings"
	"testing"
)

// TestFormatArgument_EmbedsNonce verifies that formatArgument includes the
// nonce in its output. HMACSigner.Sign uses the formatted argument (not the
// raw JSON) as part of the HMAC message, so this confirms the nonce is covered
// by the signature even though HMACSigner.Sign does not use the nonce
// parameter directly.
func TestFormatArgument_EmbedsNonce(t *testing.T) {
	nonce := "my-unique-nonce-abcdef"
	arg := `{"object_id":"test"}`

	formatted := formatArgument(arg, nonce)

	if !strings.Contains(formatted, nonce) {
		t.Errorf("formatted argument does not contain nonce %q: got %q", nonce, formatted)
	}
}

// TestHMACSigner_DifferentNoncesProduceDifferentSignatures verifies that two
// requests with different nonces produce different HMAC-SHA256 signatures.
// HMACSigner.Sign does not use the nonce parameter directly — the nonce is
// embedded in contractArgument via the V2 envelope produced by formatArgument.
// This test confirms that nonce replay attacks are prevented via that path.
func TestHMACSigner_DifferentNoncesProduceDifferentSignatures(t *testing.T) {
	signer := NewHMACSigner("test-secret-key")

	rawArg := `{"object_id":"tegata-test","hash_value":"abc123"}`
	formatted1 := formatArgument(rawArg, "nonce-aaa")
	formatted2 := formatArgument(rawArg, "nonce-bbb")

	sig1, err := signer.Sign("object.v1_0_0.Put", formatted1, "nonce-aaa", "entity-1", 1)
	if err != nil {
		t.Fatalf("Sign call 1: %v", err)
	}
	sig2, err := signer.Sign("object.v1_0_0.Put", formatted2, "nonce-bbb", "entity-1", 1)
	if err != nil {
		t.Fatalf("Sign call 2: %v", err)
	}

	if bytes.Equal(sig1, sig2) {
		t.Error("expected different signatures for different nonces, got identical HMAC output")
	}
}

// TestHMACSigner_Deterministic verifies that HMAC-SHA256 produces identical
// output for identical inputs.
func TestHMACSigner_Deterministic(t *testing.T) {
	signer := NewHMACSigner("test-secret-key")

	arg := formatArgument(`{"object_id":"test"}`, "fixed-nonce")
	sig1, err := signer.Sign("contract.id", arg, "fixed-nonce", "entity", 1)
	if err != nil {
		t.Fatalf("first Sign: %v", err)
	}
	sig2, err := signer.Sign("contract.id", arg, "fixed-nonce", "entity", 1)
	if err != nil {
		t.Fatalf("second Sign: %v", err)
	}

	if !bytes.Equal(sig1, sig2) {
		t.Error("HMAC should be deterministic for identical inputs")
	}
}

// TestHMACSigner_Zero verifies that Zero overwrites all key material.
func TestHMACSigner_Zero(t *testing.T) {
	signer := NewHMACSigner("test-key-material")
	signer.Zero()

	for i, b := range signer.secretKey {
		if b != 0 {
			t.Errorf("secret key byte %d not zeroed: got %d", i, b)
		}
	}
}
