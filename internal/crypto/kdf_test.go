package crypto_test

import (
	"bytes"
	"testing"

	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/crypto/guard"
)

func TestDeriveKey_Deterministic(t *testing.T) {
	passphrase := []byte("test-passphrase")
	salt := bytes.Repeat([]byte{0xAA}, 32)
	params := crypto.DefaultParams

	key1 := crypto.DeriveKey(passphrase, salt, params)
	defer key1.Destroy()

	// Re-derive with same inputs (need fresh passphrase since DeriveKey may
	// not wipe it, but the value must be identical).
	passphrase2 := []byte("test-passphrase")
	key2 := crypto.DeriveKey(passphrase2, salt, params)
	defer key2.Destroy()

	if !bytes.Equal(key1.Bytes(), key2.Bytes()) {
		t.Error("same inputs produced different keys")
	}
}

func TestDeriveKey_DifferentSalt_DifferentKey(t *testing.T) {
	passphrase := []byte("test-passphrase")
	salt1 := bytes.Repeat([]byte{0xAA}, 32)
	salt2 := bytes.Repeat([]byte{0xBB}, 32)
	params := crypto.DefaultParams

	key1 := crypto.DeriveKey(passphrase, salt1, params)
	defer key1.Destroy()

	passphrase2 := []byte("test-passphrase")
	key2 := crypto.DeriveKey(passphrase2, salt2, params)
	defer key2.Destroy()

	if bytes.Equal(key1.Bytes(), key2.Bytes()) {
		t.Error("different salts produced the same key")
	}
}

func TestDeriveKey_OutputLength(t *testing.T) {
	passphrase := []byte("test-passphrase")
	salt := bytes.Repeat([]byte{0xCC}, 32)
	params := crypto.DefaultParams

	key := crypto.DeriveKey(passphrase, salt, params)
	defer key.Destroy()

	if key.Size() != 32 {
		t.Errorf("key length = %d, want 32", key.Size())
	}
}

func TestDeriveKey_ReturnsSecretBuffer(t *testing.T) {
	passphrase := []byte("test-passphrase")
	salt := bytes.Repeat([]byte{0xDD}, 32)
	params := crypto.DefaultParams

	key := crypto.DeriveKey(passphrase, salt, params)
	defer key.Destroy()

	// Type assertion: key must be *guard.SecretBuffer.
	var _ *guard.SecretBuffer = key
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := crypto.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt: %v", err)
	}
	if len(salt1) != 32 {
		t.Errorf("salt length = %d, want 32", len(salt1))
	}

	salt2, err := crypto.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt: %v", err)
	}

	// Two random salts should differ (astronomically unlikely to collide).
	if bytes.Equal(salt1, salt2) {
		t.Error("two GenerateSalt calls produced identical output")
	}
}

// BenchmarkDeriveKey benchmarks Argon2id with default parameters.
// The benchmark should complete in under 3 seconds per operation.
func BenchmarkDeriveKey(b *testing.B) {
	passphrase := []byte("benchmark-passphrase")
	salt := bytes.Repeat([]byte{0xEE}, 32)
	params := crypto.DefaultParams

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := crypto.DeriveKey(passphrase, salt, params)
		key.Destroy()
	}
}
