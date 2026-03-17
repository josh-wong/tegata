// Package crypto provides cryptographic primitives for Tegata vault operations.
//
// Seal and Open implement AES-256-GCM encryption with internal nonce
// management. Callers pass a key, write counter, plaintext/ciphertext, and
// optional AAD. The nonce is derived deterministically from the counter,
// ensuring callers never construct or handle nonces directly.
//
// DeriveKey implements Argon2id key derivation, returning key material in a
// guarded SecretBuffer.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"

	"github.com/josh-wong/tegata/internal/crypto/guard"
)

// Seal encrypts plaintext with AES-256-GCM using the provided key and counter.
// The 12-byte nonce is derived internally from the counter as counter_be8 || zeros4.
// AAD (additional authenticated data) is authenticated but not encrypted.
// The key must be exactly 32 bytes (AES-256). The caller manages key lifecycle.
func Seal(key *guard.SecretBuffer, counter uint64, plaintext, aad []byte) ([]byte, error) {
	if key.Size() != 32 {
		return nil, fmt.Errorf("invalid key length: got %d, want 32", key.Size())
	}

	block, err := aes.NewCipher(key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := deriveNonce(counter)
	return gcm.Seal(nil, nonce[:], plaintext, aad), nil
}

// Open decrypts ciphertext with AES-256-GCM using the provided key and counter.
// The counter must match the value used during Seal. AAD must also match.
// Returns an error if the key is wrong, the ciphertext is tampered, or the AAD
// does not match.
func Open(key *guard.SecretBuffer, counter uint64, ciphertext, aad []byte) ([]byte, error) {
	if key.Size() != 32 {
		return nil, fmt.Errorf("invalid key length: got %d, want 32", key.Size())
	}

	block, err := aes.NewCipher(key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := deriveNonce(counter)
	plaintext, err := gcm.Open(nil, nonce[:], ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

// deriveNonce produces a 12-byte nonce from a monotonic write counter.
// The encoding is: big-endian uint64 in the first 8 bytes, last 4 bytes zero.
// This matches the design doc section 3.4: nonce = counter_be8 || zeros4.
func deriveNonce(counter uint64) [12]byte {
	var nonce [12]byte
	binary.BigEndian.PutUint64(nonce[:8], counter)
	// nonce[8:12] are already zero from the array initialization.
	return nonce
}
