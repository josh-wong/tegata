package vault

import (
	"time"

	"github.com/josh-wong/tegata/pkg/model"
)

// CheckRateLimit returns the remaining wait duration before an unlock attempt
// is allowed. Returns zero if no wait is needed.
func CheckRateLimit(_ *model.VaultHeader) (time.Duration, error) {
	panic("not implemented")
}

// RecordFailure increments the failed attempt counter and updates the
// timestamp. The counter saturates at 255 rather than wrapping.
func RecordFailure(_ *model.VaultHeader) {
	panic("not implemented")
}

// ResetAttempts clears the failed attempt counter to zero.
func ResetAttempts(_ *model.VaultHeader) {
	panic("not implemented")
}
