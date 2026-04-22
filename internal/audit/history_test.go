package audit_test

import (
	"context"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
)

// mockClient implements the Client interface for testing.
type mockClient struct {
	collectionGet map[string][]string
	get           map[string][]*audit.EventRecord
	validate      map[string]*audit.ValidationResult
}

func (m *mockClient) Put(ctx context.Context, objectID, hashValue string) error {
	panic("unimplemented")
}

func (m *mockClient) PutWithMetadata(ctx context.Context, objectID, hashValue string, metadata map[string]interface{}) error {
	panic("unimplemented")
}

func (m *mockClient) Get(ctx context.Context, objectID string) ([]*audit.EventRecord, error) {
	if records, ok := m.get[objectID]; ok {
		return records, nil
	}
	return nil, nil
}

func (m *mockClient) Validate(ctx context.Context, objectID, expectedHash string) (*audit.ValidationResult, error) {
	if result, ok := m.validate[objectID]; ok {
		return result, nil
	}
	// Default: valid
	return &audit.ValidationResult{Valid: true}, nil
}

func (m *mockClient) CollectionCreate(ctx context.Context, collectionID string, objectIDs []string) error {
	panic("unimplemented")
}

func (m *mockClient) CollectionAdd(ctx context.Context, collectionID string, objectIDs []string) error {
	panic("unimplemented")
}

func (m *mockClient) CollectionGet(ctx context.Context, collectionID string) ([]string, error) {
	if objectIDs, ok := m.collectionGet[collectionID]; ok {
		return objectIDs, nil
	}
	return nil, nil
}

func (m *mockClient) RegisterCert(ctx context.Context, entityID string, keyVersion uint32, certPEM string) error {
	panic("unimplemented")
}

func (m *mockClient) Ping(ctx context.Context) error {
	panic("unimplemented")
}

func (m *mockClient) Close() error {
	return nil
}

func (m *mockClient) Submit(ctx context.Context, entry audit.QueueEntry) (string, error) {
	panic("unimplemented")
}

func TestFetchHistory_SortsByTimestampDescending(t *testing.T) {
	// Create a mock client with events in oldest-first order from the ledger.
	// FetchHistory should return them in newest-first order.
	client := &mockClient{
		collectionGet: map[string][]string{
			audit.CollectionID("test-entity"): {"event-1", "event-2", "event-3"},
		},
		get: map[string][]*audit.EventRecord{
			"event-1": {
				{
					ObjectID:  "event-1",
					HashValue: "hash1",
					Version:   1,
					Metadata: map[string]interface{}{
						"operation":  "totp",
						"label_hash": "label1",
						"timestamp":  float64(1000), // oldest
					},
				},
			},
			"event-2": {
				{
					ObjectID:  "event-2",
					HashValue: "hash2",
					Version:   2,
					Metadata: map[string]interface{}{
						"operation":  "hotp",
						"label_hash": "label2",
						"timestamp":  float64(3000), // newest
					},
				},
			},
			"event-3": {
				{
					ObjectID:  "event-3",
					HashValue: "hash3",
					Version:   3,
					Metadata: map[string]interface{}{
						"operation":  "static",
						"label_hash": "label3",
						"timestamp":  float64(2000), // middle
					},
				},
			},
		},
	}

	result, err := audit.FetchHistory(context.Background(), client, "test-entity")
	if err != nil {
		t.Fatalf("FetchHistory failed: %v", err)
	}

	// Verify records are sorted by timestamp in descending order (newest first).
	if len(result.Records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(result.Records))
	}

	expectedOrder := []int64{3000, 2000, 1000}
	for i, expected := range expectedOrder {
		if result.Records[i].Timestamp != expected {
			t.Errorf("record %d: expected timestamp %d, got %d", i, expected, result.Records[i].Timestamp)
		}
	}
}

func TestFetchHistory_EmptyCollection(t *testing.T) {
	client := &mockClient{
		collectionGet: map[string][]string{
			audit.CollectionID("test-entity"): {},
		},
		get: map[string][]*audit.EventRecord{},
	}

	result, err := audit.FetchHistory(context.Background(), client, "test-entity")
	if err != nil {
		t.Fatalf("FetchHistory failed: %v", err)
	}

	if len(result.Records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(result.Records))
	}
}

func TestVerifyByLabelHash_AllValid(t *testing.T) {
	// Two events share "hash-a"; one event has "hash-b". Only hash-a events are validated.
	client := &mockClient{
		collectionGet: map[string][]string{
			audit.CollectionID("entity"): {"evt-1", "evt-2", "evt-3"},
		},
		get: map[string][]*audit.EventRecord{
			"evt-1": {{ObjectID: "evt-1", HashValue: "h1", Metadata: map[string]interface{}{"label_hash": "hash-a", "timestamp": float64(1)}}},
			"evt-2": {{ObjectID: "evt-2", HashValue: "h2", Metadata: map[string]interface{}{"label_hash": "hash-a", "timestamp": float64(2)}}},
			"evt-3": {{ObjectID: "evt-3", HashValue: "h3", Metadata: map[string]interface{}{"label_hash": "hash-b", "timestamp": float64(3)}}},
		},
		validate: map[string]*audit.ValidationResult{
			"evt-1": {Valid: true},
			"evt-2": {Valid: true},
		},
	}
	vaultHashes := map[string]string{
		"evt-1": "h1",
		"evt-2": "h2",
		"evt-3": "h3",
	}

	result, err := audit.VerifyByLabelHash(context.Background(), client, "entity", "hash-a", vaultHashes)
	if err != nil {
		t.Fatalf("VerifyByLabelHash failed: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true, got false: %s", result.ErrorDetail)
	}
	if result.EventCount != 2 {
		t.Errorf("expected EventCount=2, got %d", result.EventCount)
	}
	if len(result.Faults) != 0 {
		t.Errorf("expected no faults, got %v", result.Faults)
	}
}

func TestVerifyByLabelHash_TamperedEvent(t *testing.T) {
	// One of two matching events fails validation.
	client := &mockClient{
		collectionGet: map[string][]string{
			audit.CollectionID("entity"): {"evt-1", "evt-2"},
		},
		get: map[string][]*audit.EventRecord{
			"evt-1": {{ObjectID: "evt-1", HashValue: "h1", Metadata: map[string]interface{}{"label_hash": "hash-a", "timestamp": float64(1)}}},
			"evt-2": {{ObjectID: "evt-2", HashValue: "h2", Metadata: map[string]interface{}{"label_hash": "hash-a", "timestamp": float64(2)}}},
		},
		validate: map[string]*audit.ValidationResult{
			"evt-1": {Valid: true},
			"evt-2": {Valid: false, ErrorDetail: "hash mismatch"},
		},
	}
	vaultHashes := map[string]string{
		"evt-1": "h1",
		"evt-2": "h2",
	}

	result, err := audit.VerifyByLabelHash(context.Background(), client, "entity", "hash-a", vaultHashes)
	if err != nil {
		t.Fatalf("VerifyByLabelHash failed: %v", err)
	}
	if result.Valid {
		t.Error("expected Valid=false for tampered event")
	}
	if result.EventCount != 2 {
		t.Errorf("expected EventCount=2, got %d", result.EventCount)
	}
	if len(result.Faults) != 1 {
		t.Errorf("expected 1 fault, got %d", len(result.Faults))
	}
}

func TestVerifyByLabelHash_NoMatchingEvents(t *testing.T) {
	// No events match the given label hash — should return valid with zero events.
	client := &mockClient{
		collectionGet: map[string][]string{
			audit.CollectionID("entity"): {"evt-1"},
		},
		get: map[string][]*audit.EventRecord{
			"evt-1": {{ObjectID: "evt-1", HashValue: "h1", Metadata: map[string]interface{}{"label_hash": "hash-b", "timestamp": float64(1)}}},
		},
	}
	vaultHashes := map[string]string{
		"evt-1": "h1",
	}

	result, err := audit.VerifyByLabelHash(context.Background(), client, "entity", "hash-a", vaultHashes)
	if err != nil {
		t.Fatalf("VerifyByLabelHash failed: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true when no matching events, got false")
	}
	if result.EventCount != 0 {
		t.Errorf("expected EventCount=0, got %d", result.EventCount)
	}
}

func TestVerifyByLabelHash_MultipleCredentialsIndependent(t *testing.T) {
	// Scenario: Two credentials (hash-a and hash-b). hash-a has tampering, hash-b is clean.
	// Verify that tampering in hash-a does NOT affect hash-b's verification.
	client := &mockClient{
		collectionGet: map[string][]string{
			audit.CollectionID("entity"): {"evt-a1", "evt-a2", "evt-b1", "evt-b2"},
		},
		get: map[string][]*audit.EventRecord{
			"evt-a1": {{ObjectID: "evt-a1", HashValue: "ha1", Metadata: map[string]interface{}{"label_hash": "hash-a", "timestamp": float64(1)}}},
			"evt-a2": {{ObjectID: "evt-a2", HashValue: "ha2", Metadata: map[string]interface{}{"label_hash": "hash-a", "timestamp": float64(2)}}},
			"evt-b1": {{ObjectID: "evt-b1", HashValue: "hb1", Metadata: map[string]interface{}{"label_hash": "hash-b", "timestamp": float64(3)}}},
			"evt-b2": {{ObjectID: "evt-b2", HashValue: "hb2", Metadata: map[string]interface{}{"label_hash": "hash-b", "timestamp": float64(4)}}},
		},
		validate: map[string]*audit.ValidationResult{
			"evt-a1": {Valid: false, ErrorDetail: "hash mismatch"},    // hash-a is tampered
			"evt-a2": {Valid: true},
			"evt-b1": {Valid: true},                                    // hash-b is clean
			"evt-b2": {Valid: true},
		},
	}
	vaultHashes := map[string]string{
		"evt-a1": "ha1",
		"evt-a2": "ha2",
		"evt-b1": "hb1",
		"evt-b2": "hb2",
	}

	// Verify hash-a (should show tampered)
	resultA, err := audit.VerifyByLabelHash(context.Background(), client, "entity", "hash-a", vaultHashes)
	if err != nil {
		t.Fatalf("VerifyByLabelHash for hash-a failed: %v", err)
	}
	if resultA.Valid {
		t.Error("hash-a: expected Valid=false (tampered), got true")
	}
	if resultA.EventCount != 2 {
		t.Errorf("hash-a: expected EventCount=2, got %d", resultA.EventCount)
	}

	// Verify hash-b (should show clean, independent of hash-a's tampering)
	resultB, err := audit.VerifyByLabelHash(context.Background(), client, "entity", "hash-b", vaultHashes)
	if err != nil {
		t.Fatalf("VerifyByLabelHash for hash-b failed: %v", err)
	}
	if !resultB.Valid {
		t.Error("hash-b: expected Valid=true (clean), got false")
	}
	if resultB.EventCount != 2 {
		t.Errorf("hash-b: expected EventCount=2, got %d", resultB.EventCount)
	}

	// Key assertion: tampering in hash-a does NOT affect hash-b
	if resultA.Valid == resultB.Valid {
		t.Error("per-credential verification broken: hash-a and hash-b should have different results")
	}
}

func TestVerifyAll_SkipsPreExistingEvents(t *testing.T) {
	// 3 events in collection, only 2 in vaultHashes — 1 should be skipped.
	client := &mockClient{
		collectionGet: map[string][]string{
			audit.CollectionID("entity"): {"evt-1", "evt-2", "evt-3"},
		},
		validate: map[string]*audit.ValidationResult{
			"evt-1": {Valid: true},
			"evt-2": {Valid: true},
		},
	}
	vaultHashes := map[string]string{
		"evt-1": "hash1",
		"evt-2": "hash2",
		// evt-3 missing — pre-existing event
	}
	result, err := audit.VerifyAll(context.Background(), client, "entity", vaultHashes)
	if err != nil {
		t.Fatalf("VerifyAll failed: %v", err)
	}
	if result.EventCount != 2 {
		t.Errorf("expected EventCount=2, got %d", result.EventCount)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", result.Skipped)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
}
