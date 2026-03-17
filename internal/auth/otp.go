// Package auth implements authentication engines for TOTP (RFC 6238), HOTP
// (RFC 4226), and static password retrieval. All engines are pure computation
// with no vault I/O dependency.
package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
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

// ParseOTPAuthURI parses an otpauth:// URI (as defined by the Google
// Authenticator key URI format) into a Credential. The URI must have the form:
//
//	otpauth://TYPE/LABEL?secret=SECRET&issuer=ISSUER&...
//
// The label from the URI path may contain an issuer prefix separated by a
// colon (e.g., "GitHub:user@example.com"). A query issuer parameter overrides
// the path prefix. The base32 secret is validated but stored as the original
// base32 string. A UUID is generated for the credential ID and timestamps are
// set to the current time.
func ParseOTPAuthURI(uri string) (*model.Credential, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parsing URI: %w", errors.ErrInvalidInput)
	}

	if u.Scheme != "otpauth" {
		return nil, fmt.Errorf("expected otpauth:// scheme, got %q: %w", u.Scheme, errors.ErrInvalidInput)
	}

	credType := model.CredentialType(u.Host)
	if credType != model.CredentialTOTP && credType != model.CredentialHOTP {
		return nil, fmt.Errorf("unsupported OTP type %q: %w", u.Host, errors.ErrInvalidInput)
	}

	// Parse label from path. The path may include an issuer prefix.
	label := strings.TrimPrefix(u.Path, "/")
	label, _ = url.PathUnescape(label)
	issuer := ""
	if idx := strings.Index(label, ":"); idx >= 0 {
		issuer = label[:idx]
		label = strings.TrimSpace(label[idx+1:])
	}

	q := u.Query()

	secret := q.Get("secret")
	if secret == "" {
		return nil, fmt.Errorf("missing secret parameter: %w", errors.ErrInvalidInput)
	}

	// Validate base32 encoding but store the original string.
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret); err != nil {
		return nil, fmt.Errorf("invalid base32 secret: %w", errors.ErrInvalidInput)
	}

	// Query issuer overrides path issuer.
	if qi := q.Get("issuer"); qi != "" {
		issuer = qi
	}

	algorithm := q.Get("algorithm")
	if algorithm == "" {
		algorithm = "SHA1"
	}

	digits := 6
	if d := q.Get("digits"); d != "" {
		digits, err = strconv.Atoi(d)
		if err != nil {
			return nil, fmt.Errorf("invalid digits %q: %w", d, errors.ErrInvalidInput)
		}
	}
	if digits < 6 || digits > 8 {
		return nil, fmt.Errorf("digits must be 6-8, got %d: %w", digits, errors.ErrInvalidInput)
	}

	period := 30
	if p := q.Get("period"); p != "" {
		period, err = strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid period %q: %w", p, errors.ErrInvalidInput)
		}
	}
	if period < 1 {
		return nil, fmt.Errorf("period must be positive, got %d: %w", period, errors.ErrInvalidInput)
	}

	var counter uint64
	if c := q.Get("counter"); c != "" {
		counter, err = strconv.ParseUint(c, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid counter %q: %w", c, errors.ErrInvalidInput)
		}
	}

	now := time.Now()
	cred := &model.Credential{
		ID:         uuid.New().String(),
		Label:      label,
		Issuer:     issuer,
		Type:       credType,
		Algorithm:  algorithm,
		Digits:     digits,
		Period:     period,
		Counter:    counter,
		Secret:     secret,
		CreatedAt:  now,
		ModifiedAt: now,
	}

	return cred, nil
}
