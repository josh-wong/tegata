package auth

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/errors"
)

// GenerateHOTP produces an HMAC-based one-time password per RFC 4226 for the
// given counter value.
func GenerateHOTP(secret []byte, counter uint64, digits int, algorithm string) string {
	return computeHOTP(secret, counter, digits, hashFuncFromAlgorithm(algorithm))
}

// ResyncHOTP scans a look-ahead window of 100 counters starting from
// currentCounter to find where code1 matches at counter N and code2 matches at
// counter N+1. It returns N+2 (the next unused counter) on success. Returns
// ErrNotFound if no consecutive match is found within the window.
func ResyncHOTP(secret []byte, code1, code2 string, currentCounter uint64, digits int, algorithm string) (uint64, error) {
	hashFunc := hashFuncFromAlgorithm(algorithm)
	limit := currentCounter + 100
	for c := currentCounter; c < limit; c++ {
		if computeHOTP(secret, c, digits, hashFunc) == code1 {
			if computeHOTP(secret, c+1, digits, hashFunc) == code2 {
				return c + 2, nil
			}
		}
	}
	return 0, fmt.Errorf("resync failed within look-ahead window: %w", errors.ErrNotFound)
}
