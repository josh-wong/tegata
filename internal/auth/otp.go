// Package auth implements authentication engines for TOTP (RFC 6238), HOTP
// (RFC 4226), and static password retrieval. All engines are pure computation
// with no vault I/O dependency.
package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"hash"
)

// dynamicTruncate implements RFC 4226 section 5.4. It extracts a 31-bit
// integer from an HMAC result using the offset defined by the last nibble.
func dynamicTruncate(hmacResult []byte) int32 {
	offset := hmacResult[len(hmacResult)-1] & 0x0f
	code := int32(hmacResult[offset]&0x7f)<<24 |
		int32(hmacResult[offset+1])<<16 |
		int32(hmacResult[offset+2])<<8 |
		int32(hmacResult[offset+3])
	return code
}

// computeHOTP implements the core HOTP computation shared by both TOTP and
// HOTP. It computes HMAC over the counter, applies dynamic truncation, and
// formats the result as a zero-padded string of the specified digit length.
func computeHOTP(secret []byte, counter uint64, digits int, hashFunc func() hash.Hash) string {
	// Encode counter as big-endian 8-byte value per RFC 4226 section 5.2.
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(hashFunc, secret)
	mac.Write(buf)
	sum := mac.Sum(nil)

	truncated := dynamicTruncate(sum)
	mod := int32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	otp := truncated % mod

	return fmt.Sprintf("%0*d", digits, otp)
}

// hashFuncFromAlgorithm maps an algorithm name to a hash constructor. An empty
// string defaults to SHA-1 per RFC 6238.
func hashFuncFromAlgorithm(algo string) func() hash.Hash {
	switch algo {
	case "SHA256":
		return sha256.New
	case "SHA512":
		return sha512.New
	default:
		return sha1.New
	}
}
