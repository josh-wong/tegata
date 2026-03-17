package vault

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

// GenerateRecoveryKey generates a 32-byte random recovery key. It returns the
// raw bytes and a human-readable display string (base32-encoded without padding,
// chunked with dashes every 4 characters for readability).
func GenerateRecoveryKey() (raw []byte, display string, err error) {
	raw = make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generating recovery key: %w", err)
	}

	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	// Chunk into groups of 4 separated by dashes for readability.
	var chunks []string
	for i := 0; i < len(encoded); i += 4 {
		end := i + 4
		if end > len(encoded) {
			end = len(encoded)
		}
		chunks = append(chunks, encoded[i:end])
	}
	display = strings.Join(chunks, "-")

	return raw, display, nil
}
