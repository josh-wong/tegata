package audit_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
)

// sha256Hex is a test helper that computes hex(SHA-256(s)) for expected values.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestAuthEvent_HashFields(t *testing.T) {
	evt := audit.NewAuthEvent("totp", "GitHub", "acme", "hostname", true, "")

	wantLabel := sha256Hex("GitHub")
	wantService := sha256Hex("acme")
	wantHost := sha256Hex("hostname")

	if evt.LabelHash != wantLabel {
		t.Errorf("LabelHash = %q, want %q", evt.LabelHash, wantLabel)
	}
	if evt.ServiceHash != wantService {
		t.Errorf("ServiceHash = %q, want %q", evt.ServiceHash, wantService)
	}
	if evt.HostHash != wantHost {
		t.Errorf("HostHash = %q, want %q", evt.HostHash, wantHost)
	}
}

func TestAuthEvent_NoPlaintextInJSON(t *testing.T) {
	evt := audit.NewAuthEvent("totp", "GitHub", "acme", "hostname", true, "")

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)
	for _, plaintext := range []string{"GitHub", "acme", "hostname"} {
		if strings.Contains(jsonStr, plaintext) {
			t.Errorf("JSON contains plaintext %q, must not: %s", plaintext, jsonStr)
		}
	}
}

func TestAuthEvent_EventIDIsUUIDv4(t *testing.T) {
	evt := audit.NewAuthEvent("totp", "GitHub", "acme", "hostname", true, "")

	// EventID format: "tegata-" prefix + UUID v4 (8-4-4-4-12 hex chars)
	id := evt.EventID
	const prefix = "tegata-"
	if !strings.HasPrefix(id, prefix) {
		t.Fatalf("EventID missing prefix: %q", id)
	}
	uuidPart := id[len(prefix):]
	if len(uuidPart) != 36 {
		t.Fatalf("EventID UUID part length = %d, want 36: %q", len(uuidPart), id)
	}
	// Check dashes at positions 8, 13, 18, 23
	for _, pos := range []int{8, 13, 18, 23} {
		if uuidPart[pos] != '-' {
			t.Errorf("EventID UUID[%d] = %q, want '-': %q", pos, uuidPart[pos], id)
		}
	}
	// Version nibble at position 14 must be '4'
	if uuidPart[14] != '4' {
		t.Errorf("EventID version nibble = %q, want '4': %q", uuidPart[14], id)
	}
}

func TestAuthEvent_OperationTypePreserved(t *testing.T) {
	for _, opType := range []string{"totp", "hotp", "challenge-response", "static"} {
		evt := audit.NewAuthEvent(opType, "label", "service", "host", true, "")
		if evt.OperationType != opType {
			t.Errorf("OperationType = %q, want %q", evt.OperationType, opType)
		}
	}
}

func TestAuthEvent_SuccessFieldInJSON(t *testing.T) {
	trueEvt := audit.NewAuthEvent("totp", "label", "svc", "host", true, "")
	falseEvt := audit.NewAuthEvent("totp", "label", "svc", "host", false, "")

	trueData, _ := json.Marshal(trueEvt)
	falseData, _ := json.Marshal(falseEvt)

	if !strings.Contains(string(trueData), `"success":true`) {
		t.Errorf("success=true not found in JSON: %s", trueData)
	}
	if !strings.Contains(string(falseData), `"success":false`) {
		t.Errorf("success=false not found in JSON: %s", falseData)
	}
}

func TestAuthEvent_HashStringDeterministic(t *testing.T) {
	h1 := audit.HashString("test-value")
	h2 := audit.HashString("test-value")
	if h1 != h2 {
		t.Errorf("HashString not deterministic: %q != %q", h1, h2)
	}
	if h1 != sha256Hex("test-value") {
		t.Errorf("HashString(%q) = %q, want %q", "test-value", h1, sha256Hex("test-value"))
	}
}

func TestAuthEvent_PrevHashPassedThrough(t *testing.T) {
	prevHash := "abc123def456"
	evt := audit.NewAuthEvent("totp", "label", "svc", "host", true, prevHash)
	if evt.PrevHash != prevHash {
		t.Errorf("PrevHash = %q, want %q", evt.PrevHash, prevHash)
	}
}
