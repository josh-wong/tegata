package auth

import "time"

// GenerateTOTP produces a time-based one-time password per RFC 6238. The
// counter is derived as t.Unix() / period. It returns the formatted code and
// the number of seconds remaining until the code expires.
func GenerateTOTP(secret []byte, t time.Time, period, digits int, algorithm string) (string, int) {
	counter := uint64(t.Unix()) / uint64(period)
	code := computeHOTP(secret, counter, digits, hashFuncFromAlgorithm(algorithm))

	// Seconds until the next period boundary.
	remaining := period - int(t.Unix()%int64(period))
	return code, remaining
}

// TimeRemaining returns the number of seconds until the current TOTP code
// expires for the given period.
func TimeRemaining(period int) int {
	return period - int(time.Now().Unix()%int64(period))
}
