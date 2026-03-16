package auth

import (
	"testing"
	"time"
)

// RFC 6238 Appendix B test vectors. The secrets are ASCII strings padded/used
// as specified in the RFC for each hash algorithm.
func TestGenerateTOTP_RFC6238(t *testing.T) {
	// SHA-1 secret: "12345678901234567890" (20 bytes)
	sha1Secret := []byte("12345678901234567890")
	// SHA-256 secret: "12345678901234567890123456789012" (32 bytes)
	sha256Secret := []byte("12345678901234567890123456789012")
	// SHA-512 secret: "1234567890123456789012345678901234567890123456789012345678901234" (64 bytes)
	sha512Secret := []byte("1234567890123456789012345678901234567890123456789012345678901234")

	tests := []struct {
		name      string
		secret    []byte
		unixTime  int64
		algorithm string
		digits    int
		period    int
		wantCode  string
	}{
		// SHA-1 test vectors
		{"SHA1/t=59", sha1Secret, 59, "SHA1", 8, 30, "94287082"},
		{"SHA1/t=1111111109", sha1Secret, 1111111109, "SHA1", 8, 30, "07081804"},
		{"SHA1/t=1111111111", sha1Secret, 1111111111, "SHA1", 8, 30, "14050471"},
		{"SHA1/t=1234567890", sha1Secret, 1234567890, "SHA1", 8, 30, "89005924"},
		{"SHA1/t=2000000000", sha1Secret, 2000000000, "SHA1", 8, 30, "69279037"},
		{"SHA1/t=20000000000", sha1Secret, 20000000000, "SHA1", 8, 30, "65353130"},

		// SHA-256 test vectors
		{"SHA256/t=59", sha256Secret, 59, "SHA256", 8, 30, "46119246"},
		{"SHA256/t=1111111109", sha256Secret, 1111111109, "SHA256", 8, 30, "68084774"},
		{"SHA256/t=1111111111", sha256Secret, 1111111111, "SHA256", 8, 30, "67062674"},
		{"SHA256/t=1234567890", sha256Secret, 1234567890, "SHA256", 8, 30, "91819424"},
		{"SHA256/t=2000000000", sha256Secret, 2000000000, "SHA256", 8, 30, "90698825"},
		{"SHA256/t=20000000000", sha256Secret, 20000000000, "SHA256", 8, 30, "77737706"},

		// SHA-512 test vectors
		{"SHA512/t=59", sha512Secret, 59, "SHA512", 8, 30, "90693936"},
		{"SHA512/t=1111111109", sha512Secret, 1111111109, "SHA512", 8, 30, "25091201"},
		{"SHA512/t=1111111111", sha512Secret, 1111111111, "SHA512", 8, 30, "99943326"},
		{"SHA512/t=1234567890", sha512Secret, 1234567890, "SHA512", 8, 30, "93441116"},
		{"SHA512/t=2000000000", sha512Secret, 2000000000, "SHA512", 8, 30, "38618901"},
		{"SHA512/t=20000000000", sha512Secret, 20000000000, "SHA512", 8, 30, "47863826"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tm := time.Unix(tc.unixTime, 0).UTC()
			code, _ := GenerateTOTP(tc.secret, tm, tc.period, tc.digits, tc.algorithm)
			if code != tc.wantCode {
				t.Errorf("GenerateTOTP(%q, %d) = %q, want %q", tc.algorithm, tc.unixTime, code, tc.wantCode)
			}
		})
	}
}

func TestGenerateTOTP_6Digits(t *testing.T) {
	secret := []byte("12345678901234567890")
	tm := time.Unix(59, 0).UTC()
	code, _ := GenerateTOTP(secret, tm, 30, 6, "SHA1")
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %d digits: %q", len(code), code)
	}
	// Last 6 digits of "94287082" = "287082"
	if code != "287082" {
		t.Errorf("6-digit TOTP = %q, want %q", code, "287082")
	}
}

func TestGenerateTOTP_DefaultAlgorithm(t *testing.T) {
	secret := []byte("12345678901234567890")
	tm := time.Unix(59, 0).UTC()

	codeEmpty, _ := GenerateTOTP(secret, tm, 30, 8, "")
	codeSHA1, _ := GenerateTOTP(secret, tm, 30, 8, "SHA1")
	if codeEmpty != codeSHA1 {
		t.Errorf("empty algorithm %q != SHA1 %q", codeEmpty, codeSHA1)
	}
}

func TestTimeRemaining(t *testing.T) {
	remaining := TimeRemaining(30)
	if remaining < 1 || remaining > 30 {
		t.Errorf("TimeRemaining(30) = %d, want 1..30", remaining)
	}
}

func TestGenerateTOTP_Remaining(t *testing.T) {
	secret := []byte("12345678901234567890")
	tm := time.Unix(59, 0).UTC()
	_, remaining := GenerateTOTP(secret, tm, 30, 8, "SHA1")
	// t=59, period=30 => counter=1, next boundary=60, remaining=60-59=1
	if remaining != 1 {
		t.Errorf("remaining = %d, want 1", remaining)
	}
}
