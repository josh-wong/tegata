package crypto

import (
	"crypto/rand"
	"fmt"

	"github.com/josh-wong/tegata/internal/crypto/guard"
	"golang.org/x/crypto/argon2"
)

// KDFParams holds the tunable parameters for Argon2id key derivation.
type KDFParams struct {
	Time    uint32 // Number of iterations (time cost).
	Memory  uint32 // Memory in KiB (memory cost).
	Threads uint8  // Degree of parallelism.
	KeyLen  uint32 // Output key length in bytes.
}

// DefaultParams provides the recommended Argon2id parameters from the design
// doc section 3.3: time=3, memory=64 MiB, threads=4, keyLen=32.
var DefaultParams = KDFParams{
	Time:    3,
	Memory:  64 * 1024, // 64 MiB in KiB.
	Threads: 4,
	KeyLen:  32,
}

// DeriveKey derives a cryptographic key from a passphrase and salt using
// Argon2id. The returned SecretBuffer holds the key in guarded memory. The
// caller is responsible for calling Destroy on the returned buffer when done.
func DeriveKey(passphrase, salt []byte, params KDFParams) *guard.SecretBuffer {
	raw := argon2.IDKey(passphrase, salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return guard.NewSecretBuffer(raw)
}

// GenerateSalt generates a 32-byte cryptographically random salt suitable for
// use with DeriveKey.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}
	return salt, nil
}
