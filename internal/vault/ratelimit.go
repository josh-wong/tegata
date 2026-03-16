package vault

import (
	"time"

	"github.com/josh-wong/tegata/pkg/model"
)

// maxBackoff is the maximum rate-limit wait time in seconds.
const maxBackoff = 300

// CheckRateLimit returns the remaining wait duration before an unlock attempt
// is allowed. Returns zero if no wait is needed (no prior failures or the
// backoff period has elapsed).
//
// The backoff formula is 2^(attempts-1) seconds, capped at 300 seconds.
func CheckRateLimit(h *model.VaultHeader) (time.Duration, error) {
	if h.FailedAttempts == 0 {
		return 0, nil
	}

	backoffSec := int64(1) << (h.FailedAttempts - 1)
	if backoffSec > maxBackoff {
		backoffSec = maxBackoff
	}

	elapsed := time.Since(time.Unix(h.LastAttemptTime, 0))
	remaining := time.Duration(backoffSec)*time.Second - elapsed
	if remaining <= 0 {
		return 0, nil
	}
	return remaining, nil
}

// RecordFailure increments the failed attempt counter and updates the
// timestamp. The counter saturates at 255 (uint8 max) rather than wrapping.
func RecordFailure(h *model.VaultHeader) {
	if h.FailedAttempts < 255 {
		h.FailedAttempts++
	}
	h.LastAttemptTime = time.Now().Unix()
}

// ResetAttempts clears the failed attempt counter to zero.
func ResetAttempts(h *model.VaultHeader) {
	h.FailedAttempts = 0
}
