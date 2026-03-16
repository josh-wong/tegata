package auth

import (
	"testing"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

func TestParseOTPAuthURI_TOTP(t *testing.T) {
	uri := "otpauth://totp/Example:alice@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Example&algorithm=SHA256&digits=8&period=60"
	cred, err := ParseOTPAuthURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Type != model.CredentialTOTP {
		t.Errorf("type = %q, want %q", cred.Type, model.CredentialTOTP)
	}
	if cred.Label != "alice@example.com" {
		t.Errorf("label = %q, want %q", cred.Label, "alice@example.com")
	}
	if cred.Issuer != "Example" {
		t.Errorf("issuer = %q, want %q", cred.Issuer, "Example")
	}
	if cred.Algorithm != "SHA256" {
		t.Errorf("algorithm = %q, want %q", cred.Algorithm, "SHA256")
	}
	if cred.Digits != 8 {
		t.Errorf("digits = %d, want 8", cred.Digits)
	}
	if cred.Period != 60 {
		t.Errorf("period = %d, want 60", cred.Period)
	}
	if cred.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("secret = %q, want %q", cred.Secret, "JBSWY3DPEHPK3PXP")
	}
	if cred.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestParseOTPAuthURI_HOTP(t *testing.T) {
	uri := "otpauth://hotp/MyService:bob?secret=JBSWY3DPEHPK3PXP&counter=42"
	cred, err := ParseOTPAuthURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Type != model.CredentialHOTP {
		t.Errorf("type = %q, want %q", cred.Type, model.CredentialHOTP)
	}
	if cred.Counter != 42 {
		t.Errorf("counter = %d, want 42", cred.Counter)
	}
	if cred.Label != "bob" {
		t.Errorf("label = %q, want %q", cred.Label, "bob")
	}
	if cred.Issuer != "MyService" {
		t.Errorf("issuer = %q, want %q", cred.Issuer, "MyService")
	}
}

func TestParseOTPAuthURI_Defaults(t *testing.T) {
	uri := "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"
	cred, err := ParseOTPAuthURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Algorithm != "SHA1" {
		t.Errorf("default algorithm = %q, want SHA1", cred.Algorithm)
	}
	if cred.Digits != 6 {
		t.Errorf("default digits = %d, want 6", cred.Digits)
	}
	if cred.Period != 30 {
		t.Errorf("default period = %d, want 30", cred.Period)
	}
}

func TestParseOTPAuthURI_HOTPDefaults(t *testing.T) {
	uri := "otpauth://hotp/Test?secret=JBSWY3DPEHPK3PXP"
	cred, err := ParseOTPAuthURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Counter != 0 {
		t.Errorf("default counter = %d, want 0", cred.Counter)
	}
}

func TestParseOTPAuthURI_IssuerQueryOverridesPath(t *testing.T) {
	uri := "otpauth://totp/PathIssuer:alice?secret=JBSWY3DPEHPK3PXP&issuer=QueryIssuer"
	cred, err := ParseOTPAuthURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Issuer != "QueryIssuer" {
		t.Errorf("issuer = %q, want QueryIssuer (query should win)", cred.Issuer)
	}
}

func TestParseOTPAuthURI_BadScheme(t *testing.T) {
	_, err := ParseOTPAuthURI("https://totp/Test?secret=JBSWY3DPEHPK3PXP")
	if err == nil {
		t.Fatal("expected error for non-otpauth scheme")
	}
	if !errors.Is(err, errors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestParseOTPAuthURI_MissingSecret(t *testing.T) {
	_, err := ParseOTPAuthURI("otpauth://totp/Test")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
	if !errors.Is(err, errors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestParseOTPAuthURI_UnsupportedType(t *testing.T) {
	_, err := ParseOTPAuthURI("otpauth://push/Test?secret=JBSWY3DPEHPK3PXP")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !errors.Is(err, errors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestParseOTPAuthURI_InvalidBase32(t *testing.T) {
	_, err := ParseOTPAuthURI("otpauth://totp/Test?secret=!!!invalid!!!")
	if err == nil {
		t.Fatal("expected error for invalid base32 secret")
	}
	if !errors.Is(err, errors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestParseOTPAuthURI_LabelWithoutIssuerPrefix(t *testing.T) {
	uri := "otpauth://totp/alice@example.com?secret=JBSWY3DPEHPK3PXP"
	cred, err := ParseOTPAuthURI(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.Label != "alice@example.com" {
		t.Errorf("label = %q, want %q", cred.Label, "alice@example.com")
	}
	if cred.Issuer != "" {
		t.Errorf("issuer = %q, want empty", cred.Issuer)
	}
}
