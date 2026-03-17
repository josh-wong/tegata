package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/josh-wong/tegata/pkg/model"
)

func TestVaultPayload_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)

	original := model.VaultPayload{
		Version:         1,
		CreatedAt:       now,
		ModifiedAt:      now,
		Credentials:     []model.Credential{},
		RecoveryKeyHash: "abc123",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded model.VaultPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("Version: got %d, want %d", decoded.Version, original.Version)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch")
	}
	if decoded.RecoveryKeyHash != original.RecoveryKeyHash {
		t.Errorf("RecoveryKeyHash: got %q, want %q", decoded.RecoveryKeyHash, original.RecoveryKeyHash)
	}
}

func TestCredential_TOTP_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 5, 0, 0, time.UTC)

	original := model.Credential{
		ID:         "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Label:      "GitHub",
		Issuer:     "GitHub",
		Type:       model.CredentialTOTP,
		Algorithm:  "SHA1",
		Digits:     6,
		Period:     30,
		Secret:     "base32-encoded-secret",
		Tags:       []string{"dev", "work"},
		CreatedAt:  now,
		ModifiedAt: now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Verify JSON field names match design doc schema.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	expectedFields := []string{"id", "label", "issuer", "type", "algorithm", "digits", "period", "secret", "tags", "created_at", "modified_at"}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}

	var decoded model.Credential
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Label != original.Label {
		t.Errorf("Label: got %q, want %q", decoded.Label, original.Label)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Digits != original.Digits {
		t.Errorf("Digits: got %d, want %d", decoded.Digits, original.Digits)
	}
	if decoded.Period != original.Period {
		t.Errorf("Period: got %d, want %d", decoded.Period, original.Period)
	}
}

func TestCredential_HOTP_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 10, 0, 0, time.UTC)

	original := model.Credential{
		ID:         "b2c3d4e5-f6a7-8901-bcde-f12345678901",
		Label:      "AWS-prod",
		Issuer:     "Amazon",
		Type:       model.CredentialHOTP,
		Algorithm:  "SHA1",
		Digits:     6,
		Counter:    42,
		Secret:     "base32-encoded-secret",
		Tags:       []string{"cloud"},
		CreatedAt:  now,
		ModifiedAt: now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Verify counter field is present.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}
	if _, ok := raw["counter"]; !ok {
		t.Error("HOTP credential missing 'counter' field in JSON")
	}

	var decoded model.Credential
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Counter != 42 {
		t.Errorf("Counter: got %d, want 42", decoded.Counter)
	}
}

func TestCredential_ChallengeResponse_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 15, 0, 0, time.UTC)

	original := model.Credential{
		ID:         "c3d4e5f6-a7b8-9012-cdef-123456789012",
		Label:      "SSH-signing",
		Type:       model.CredentialChallengeResponse,
		Algorithm:  "SHA256",
		Secret:     "hex-encoded-key",
		Tags:       []string{},
		CreatedAt:  now,
		ModifiedAt: now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Verify the type value in JSON is "challenge-response".
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	var typeVal string
	if err := json.Unmarshal(raw["type"], &typeVal); err != nil {
		t.Fatalf("Unmarshal type: %v", err)
	}
	if typeVal != "challenge-response" {
		t.Errorf("type in JSON: got %q, want %q", typeVal, "challenge-response")
	}

	var decoded model.Credential
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != model.CredentialChallengeResponse {
		t.Errorf("Type: got %q, want %q", decoded.Type, model.CredentialChallengeResponse)
	}
}

func TestCredential_Static_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 20, 0, 0, time.UTC)

	original := model.Credential{
		ID:         "d4e5f6a7-b8c9-0123-defa-234567890123",
		Label:      "WiFi-office",
		Type:       model.CredentialStatic,
		Secret:     "plaintext-password-value",
		Tags:       []string{"network"},
		CreatedAt:  now,
		ModifiedAt: now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded model.Credential
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Label != "WiFi-office" {
		t.Errorf("Label: got %q, want %q", decoded.Label, "WiFi-office")
	}
	if decoded.Type != model.CredentialStatic {
		t.Errorf("Type: got %q, want %q", decoded.Type, model.CredentialStatic)
	}
}

func TestCredentialType_String(t *testing.T) {
	tests := []struct {
		ct   model.CredentialType
		want string
	}{
		{model.CredentialTOTP, "totp"},
		{model.CredentialHOTP, "hotp"},
		{model.CredentialChallengeResponse, "challenge-response"},
		{model.CredentialStatic, "static"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
