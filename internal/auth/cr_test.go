package auth

import (
	"encoding/base32"
	"testing"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// testCRSecret decodes the well-known base32 secret JBSWY3DPEHPK3PXP used
// across challenge-response tests. It matches the secret used in TOTP tests.
func testCRSecret(t *testing.T) []byte {
	t.Helper()
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("base32 decode: %v", err)
	}
	return b
}

func makeCRCred(algo string) *model.Credential {
	return &model.Credential{
		Label:     "test-cr",
		Type:      model.CredentialChallengeResponse,
		Algorithm: algo,
	}
}

// TestSignChallenge_SHA1 verifies the known-answer for HMAC-SHA1.
// Vector: secret=JBSWY3DPEHPK3PXP, challenge="hello"
// Expected: 9329f782d404a67fb1d8f0e05d8a0e4a15e2a1b8 (40 hex chars, lowercase).
func TestSignChallenge_SHA1(t *testing.T) {
	secret := testCRSecret(t)
	cred := makeCRCred("SHA1")
	got, err := SignChallenge(cred, secret, []byte("hello"))
	if err != nil {
		t.Fatalf("SignChallenge SHA1: %v", err)
	}
	want := "9329f782d404a67fb1d8f0e05d8a0e4a15e2a1b8"
	if got != want {
		t.Errorf("SHA1 result:\ngot  %q\nwant %q", got, want)
	}
}

// TestSignChallenge_SHA256 verifies the known-answer for HMAC-SHA256.
// Vector: secret=JBSWY3DPEHPK3PXP, challenge="hello"
// Expected: ee772560953abd0fae6bbec28c2b3ae4a15380faf70d48c0ff4741a661c10a19 (64 hex chars, lowercase).
func TestSignChallenge_SHA256(t *testing.T) {
	secret := testCRSecret(t)
	cred := makeCRCred("SHA256")
	got, err := SignChallenge(cred, secret, []byte("hello"))
	if err != nil {
		t.Fatalf("SignChallenge SHA256: %v", err)
	}
	want := "ee772560953abd0fae6bbec28c2b3ae4a15380faf70d48c0ff4741a661c10a19"
	if got != want {
		t.Errorf("SHA256 result:\ngot  %q\nwant %q", got, want)
	}
}

// TestSignChallenge_WrongType verifies that passing a non-challenge-response
// credential type returns an error wrapping ErrInvalidInput.
func TestSignChallenge_WrongType(t *testing.T) {
	secret := testCRSecret(t)
	cred := &model.Credential{
		Label:     "totp-cred",
		Type:      model.CredentialTOTP,
		Algorithm: "SHA1",
	}
	_, err := SignChallenge(cred, secret, []byte("hello"))
	if err == nil {
		t.Fatal("expected error for wrong credential type, got nil")
	}
	if !errors.Is(err, errors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

// TestSignChallenge_EmptyAlgorithm verifies that an empty Algorithm field
// defaults to SHA1, matching the hashFuncFromAlgorithm convention.
func TestSignChallenge_EmptyAlgorithm(t *testing.T) {
	secret := testCRSecret(t)
	cred := makeCRCred("") // empty algorithm defaults to SHA1
	got, err := SignChallenge(cred, secret, []byte("hello"))
	if err != nil {
		t.Fatalf("SignChallenge empty algorithm: %v", err)
	}
	// Same expected as SHA1 known-answer.
	want := "9329f782d404a67fb1d8f0e05d8a0e4a15e2a1b8"
	if got != want {
		t.Errorf("empty algorithm result:\ngot  %q\nwant %q", got, want)
	}
}

// TestSignChallenge_SHA1OutputLen verifies the output is exactly 40 hex chars.
func TestSignChallenge_SHA1OutputLen(t *testing.T) {
	secret := testCRSecret(t)
	cred := makeCRCred("SHA1")
	got, err := SignChallenge(cred, secret, []byte("hello"))
	if err != nil {
		t.Fatalf("SignChallenge: %v", err)
	}
	if len(got) != 40 {
		t.Errorf("SHA1 output length: got %d, want 40", len(got))
	}
}

// TestSignChallenge_SHA256OutputLen verifies the output is exactly 64 hex chars.
func TestSignChallenge_SHA256OutputLen(t *testing.T) {
	secret := testCRSecret(t)
	cred := makeCRCred("SHA256")
	got, err := SignChallenge(cred, secret, []byte("hello"))
	if err != nil {
		t.Fatalf("SignChallenge: %v", err)
	}
	if len(got) != 64 {
		t.Errorf("SHA256 output length: got %d, want 64", len(got))
	}
}
