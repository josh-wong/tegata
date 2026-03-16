package vault

import (
	"testing"
)

func TestGenerateRecoveryKey(t *testing.T) {
	raw, display, err := GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("GenerateRecoveryKey: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("raw key length: got %d, want 32", len(raw))
	}
	if display == "" {
		t.Error("display string is empty")
	}
	// Display should contain dashes for readability.
	found := false
	for _, c := range display {
		if c == '-' {
			found = true
			break
		}
	}
	if !found {
		t.Error("display string should contain dashes for readability")
	}
}

func TestGenerateRecoveryKeyUniqueness(t *testing.T) {
	_, d1, err := GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("GenerateRecoveryKey: %v", err)
	}
	_, d2, err := GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("GenerateRecoveryKey: %v", err)
	}
	if d1 == d2 {
		t.Error("two generated recovery keys should not be identical")
	}
}
