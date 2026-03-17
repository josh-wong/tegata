package auth

import (
	"testing"
)

// RFC 4226 Appendix D test vectors. Secret is "12345678901234567890" (ASCII).
func TestGenerateHOTP_RFC4226(t *testing.T) {
	secret := []byte("12345678901234567890")
	expected := []string{
		"755224", // counter 0
		"287082", // counter 1
		"359152", // counter 2
		"969429", // counter 3
		"338314", // counter 4
		"254676", // counter 5
		"287922", // counter 6
		"162583", // counter 7
		"399871", // counter 8
		"520489", // counter 9
	}

	for counter, want := range expected {
		t.Run("", func(t *testing.T) {
			got := GenerateHOTP(secret, uint64(counter), 6, "SHA1")
			if got != want {
				t.Errorf("GenerateHOTP(counter=%d) = %q, want %q", counter, got, want)
			}
		})
	}
}

func TestGenerateHOTP_8Digits(t *testing.T) {
	secret := []byte("12345678901234567890")
	code := GenerateHOTP(secret, 0, 8, "SHA1")
	if len(code) != 8 {
		t.Errorf("expected 8-digit code, got %d digits: %q", len(code), code)
	}
}

func TestGenerateHOTP_DefaultAlgorithm(t *testing.T) {
	secret := []byte("12345678901234567890")
	codeEmpty := GenerateHOTP(secret, 0, 6, "")
	codeSHA1 := GenerateHOTP(secret, 0, 6, "SHA1")
	if codeEmpty != codeSHA1 {
		t.Errorf("empty algorithm %q != SHA1 %q", codeEmpty, codeSHA1)
	}
}

func TestResyncHOTP(t *testing.T) {
	secret := []byte("12345678901234567890")
	// Codes at counter 7 and 8 are "162583" and "399871"
	newCounter, err := ResyncHOTP(secret, "162583", "399871", 0, 6, "SHA1")
	if err != nil {
		t.Fatalf("ResyncHOTP: %v", err)
	}
	// Should return counter 9 (next unused after matching 7 and 8)
	if newCounter != 9 {
		t.Errorf("ResyncHOTP = %d, want 9", newCounter)
	}
}

func TestResyncHOTP_NotFound(t *testing.T) {
	secret := []byte("12345678901234567890")
	_, err := ResyncHOTP(secret, "000000", "000000", 0, 6, "SHA1")
	if err == nil {
		t.Fatal("expected error for non-matching codes")
	}
}

func TestDynamicTruncate(t *testing.T) {
	// RFC 4226 section 5.4 intermediate test values for counter=0
	// HMAC-SHA1("12345678901234567890", 0x0000000000000000) =
	// cc93cf18508d94934c64b65d8ba7667fb7cde4b0
	// offset = 0x0 & 0xf = 0x0... (actually last byte & 0xf)
	// We test indirectly through GenerateHOTP which uses dynamicTruncate.
	// This test just ensures the function exists and is callable.
	hmacResult := []byte{
		0xcc, 0x93, 0xcf, 0x18, 0x50, 0x8d, 0x94, 0x93,
		0x4c, 0x64, 0xb6, 0x5d, 0x8b, 0xa7, 0x66, 0x7f,
		0xb7, 0xcd, 0xe4, 0xb0,
	}
	result := dynamicTruncate(hmacResult)
	if result < 0 {
		t.Errorf("dynamicTruncate returned negative: %d", result)
	}
}
