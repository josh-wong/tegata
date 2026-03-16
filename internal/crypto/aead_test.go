package crypto_test

import (
	"bytes"
	"testing"

	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/crypto/guard"
)

func TestSealOpen_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hello")},
		{"1KB", bytes.Repeat([]byte("x"), 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := guard.NewSecretBufferFromSize(32)
			defer key.Destroy()

			// Fill key with deterministic data for testing.
			copy(key.Bytes(), bytes.Repeat([]byte{0xAB}, 32))

			counter := uint64(1)
			ciphertext, err := crypto.Seal(key, counter, tt.plaintext, nil)
			if err != nil {
				t.Fatalf("Seal: %v", err)
			}

			got, err := crypto.Open(key, counter, ciphertext, nil)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			if !bytes.Equal(got, tt.plaintext) {
				t.Errorf("round-trip mismatch: got %q, want %q", got, tt.plaintext)
			}
		})
	}
}

func TestSeal_DifferentCounter_DifferentCiphertext(t *testing.T) {
	key := guard.NewSecretBufferFromSize(32)
	defer key.Destroy()
	copy(key.Bytes(), bytes.Repeat([]byte{0xCD}, 32))

	plaintext := []byte("same plaintext")

	ct1, err := crypto.Seal(key, 1, plaintext, nil)
	if err != nil {
		t.Fatalf("Seal counter=1: %v", err)
	}

	ct2, err := crypto.Seal(key, 2, plaintext, nil)
	if err != nil {
		t.Fatalf("Seal counter=2: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Error("same key+plaintext with different counters produced identical ciphertext")
	}
}

func TestOpen_WrongKey_ReturnsError(t *testing.T) {
	key1 := guard.NewSecretBufferFromSize(32)
	defer key1.Destroy()
	copy(key1.Bytes(), bytes.Repeat([]byte{0x11}, 32))

	key2 := guard.NewSecretBufferFromSize(32)
	defer key2.Destroy()
	copy(key2.Bytes(), bytes.Repeat([]byte{0x22}, 32))

	ciphertext, err := crypto.Seal(key1, 1, []byte("secret data"), nil)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	_, err = crypto.Open(key2, 1, ciphertext, nil)
	if err == nil {
		t.Error("Open with wrong key should return error")
	}
}

func TestOpen_TamperedCiphertext_ReturnsError(t *testing.T) {
	key := guard.NewSecretBufferFromSize(32)
	defer key.Destroy()
	copy(key.Bytes(), bytes.Repeat([]byte{0x33}, 32))

	ciphertext, err := crypto.Seal(key, 1, []byte("important data"), nil)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Flip a byte in the middle of the ciphertext.
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)/2] ^= 0xFF

	_, err = crypto.Open(key, 1, tampered, nil)
	if err == nil {
		t.Error("Open with tampered ciphertext should return error")
	}
}

func TestSeal_WithAAD(t *testing.T) {
	key := guard.NewSecretBufferFromSize(32)
	defer key.Destroy()
	copy(key.Bytes(), bytes.Repeat([]byte{0x44}, 32))

	plaintext := []byte("authenticated data")
	aad := []byte("header-bytes")

	ciphertext, err := crypto.Seal(key, 1, plaintext, aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Open with correct AAD should succeed.
	got, err := crypto.Open(key, 1, ciphertext, aad)
	if err != nil {
		t.Fatalf("Open with correct AAD: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("mismatch: got %q, want %q", got, plaintext)
	}

	// Open with different AAD should fail.
	_, err = crypto.Open(key, 1, ciphertext, []byte("wrong-header"))
	if err == nil {
		t.Error("Open with wrong AAD should return error")
	}
}

func TestSeal_NilAAD(t *testing.T) {
	key := guard.NewSecretBufferFromSize(32)
	defer key.Destroy()
	copy(key.Bytes(), bytes.Repeat([]byte{0x55}, 32))

	plaintext := []byte("no aad needed")

	ciphertext, err := crypto.Seal(key, 1, plaintext, nil)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	got, err := crypto.Open(key, 1, ciphertext, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("mismatch: got %q, want %q", got, plaintext)
	}
}
