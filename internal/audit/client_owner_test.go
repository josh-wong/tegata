package audit_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
)

// putArgument decodes the JSON argument of a recorded object.Put request. The
// ScalarDL V2 envelope is "V2" \x01 nonce \x03 functionIDs \x03 json.
func putArgument(t *testing.T, formatted string) map[string]interface{} {
	t.Helper()
	i := strings.LastIndex(formatted, "\x03")
	if i < 0 {
		t.Fatalf("unexpected contract argument format: %q", formatted)
	}
	var arg map[string]interface{}
	if err := json.Unmarshal([]byte(formatted[i+1:]), &arg); err != nil {
		t.Fatalf("decoding Put argument: %v", err)
	}
	return arg
}

func TestClient_Submit_IncludesOwnerHashMetadata(t *testing.T) {
	ledgerSrv := &mockLedgerServerMulti{}
	conn := newBufconnServerMulti(t, ledgerSrv, &mockPrivilegedServer{})
	client := audit.NewLedgerClientFromConn(conn, nil, &mockSigner{sig: []byte("sig")}, "test-entity", 1)
	defer func() { _ = client.Close() }()

	evt := audit.NewAuthEvent("totp", "GitHub", "github.com", "host", true, "")
	evt.OwnerEmailHash = "emailhash"
	if _, err := client.Submit(context.Background(), audit.QueueEntry{Event: evt}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	meta, _ := putArgument(t, ledgerSrv.calls[0].ContractArgument)["metadata"].(map[string]interface{})
	if meta["owner_hash"] != "emailhash" {
		t.Errorf("owner_hash metadata = %v, want email hash as primary", meta["owner_hash"])
	}
	content, _ := meta["content"].(string)
	if !strings.Contains(content, `"owner_email_hash":"emailhash"`) {
		t.Errorf("owner hash must be part of the authenticated content: %s", content)
	}
}

func TestClient_Submit_OmitsOwnerHashWhenUnset(t *testing.T) {
	ledgerSrv := &mockLedgerServerMulti{}
	conn := newBufconnServerMulti(t, ledgerSrv, &mockPrivilegedServer{})
	client := audit.NewLedgerClientFromConn(conn, nil, &mockSigner{sig: []byte("sig")}, "test-entity", 1)
	defer func() { _ = client.Close() }()

	evt := audit.NewAuthEvent("totp", "GitHub", "github.com", "host", true, "")
	if _, err := client.Submit(context.Background(), audit.QueueEntry{Event: evt}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	meta, _ := putArgument(t, ledgerSrv.calls[0].ContractArgument)["metadata"].(map[string]interface{})
	if _, ok := meta["owner_hash"]; ok {
		t.Errorf("owner_hash should be absent for events without an owner: %v", meta)
	}
}

func ownerContent(t *testing.T) (string, string) {
	t.Helper()
	evt := audit.AuthEvent{
		EventID:             "tegata-test",
		Timestamp:           time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		OperationType:       "hotp",
		LabelHash:           "lhash",
		Success:             true,
		OwnerEmployeeIDHash: "idhash",
		OwnerHashScheme:     audit.OwnerHashSchemeSHA256,
	}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return string(b), hex.EncodeToString(sum[:])
}

func validateWithMetadata(t *testing.T, hashValue string, metadata map[string]interface{}) *audit.ValidationResult {
	t.Helper()
	ledgerSrv := &mockLedgerServerMulti{results: []string{buildGetResult(t, "tegata-test", hashValue, metadata)}}
	conn := newBufconnServerMulti(t, ledgerSrv, &mockPrivilegedServer{})
	client := audit.NewLedgerClientFromConn(conn, nil, &mockSigner{sig: []byte("sig")}, "test-entity", 1)
	defer func() { _ = client.Close() }()
	res, err := client.Validate(context.Background(), "tegata-test", hashValue)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return res
}

func TestClient_Validate_OwnerHashIntact(t *testing.T) {
	content, hashValue := ownerContent(t)
	res := validateWithMetadata(t, hashValue, map[string]interface{}{
		"content": content, "operation": "hotp", "label_hash": "lhash", "owner_hash": "idhash",
	})
	if !res.Valid {
		t.Errorf("expected valid, got %q", res.ErrorDetail)
	}
}

// TestClient_Validate_OwnerHashTampered verifies that reattributing an event
// to another person by editing metadata.owner_hash is detected even when
// hash_value and content are left intact.
func TestClient_Validate_OwnerHashTampered(t *testing.T) {
	content, hashValue := ownerContent(t)
	res := validateWithMetadata(t, hashValue, map[string]interface{}{
		"content": content, "operation": "hotp", "label_hash": "lhash", "owner_hash": "someone-else",
	})
	if res.Valid || !strings.Contains(res.ErrorDetail, "owner field has been altered") {
		t.Errorf("expected owner tampering to be detected, got valid=%v detail=%q", res.Valid, res.ErrorDetail)
	}
}

// TestClient_Validate_OwnerHashInjected verifies that adding an owner_hash to
// an event recorded without an owner is detected.
func TestClient_Validate_OwnerHashInjected(t *testing.T) {
	content := `{"event_id":"tegata-test","timestamp":"2024-01-01T00:00:00Z","operation_type":"hotp","label_hash":"lhash","service_hash":"shash","host_hash":"hhash","success":true,"prev_hash":""}`
	sum := sha256.Sum256([]byte(content))
	hashValue := hex.EncodeToString(sum[:])
	res := validateWithMetadata(t, hashValue, map[string]interface{}{
		"content": content, "operation": "hotp", "label_hash": "lhash", "owner_hash": "injected",
	})
	if res.Valid {
		t.Error("expected injected owner_hash to be detected")
	}
}

// TestClient_Validate_OwnerHashRemoved verifies that deleting owner_hash from
// an event whose authenticated content names an owner is detected, so the
// event cannot be hidden from owner_hash queries.
func TestClient_Validate_OwnerHashRemoved(t *testing.T) {
	content, hashValue := ownerContent(t)
	res := validateWithMetadata(t, hashValue, map[string]interface{}{
		"content": content, "operation": "hotp", "label_hash": "lhash",
	})
	if res.Valid || !strings.Contains(res.ErrorDetail, "owner field has been altered") {
		t.Errorf("expected removed owner_hash to be detected, got valid=%v detail=%q", res.Valid, res.ErrorDetail)
	}
}

// TestClient_Validate_OwnerHashWrongType verifies that replacing owner_hash
// with a non-string value is detected.
func TestClient_Validate_OwnerHashWrongType(t *testing.T) {
	content, hashValue := ownerContent(t)
	res := validateWithMetadata(t, hashValue, map[string]interface{}{
		"content": content, "operation": "hotp", "label_hash": "lhash", "owner_hash": 42,
	})
	if res.Valid {
		t.Error("expected non-string owner_hash to be detected")
	}
}
