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

func (m *mockClient) Validate(ctx context.Context, objectID string) (*audit.ValidationResult, error) {
	panic("unimplemented")
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

func (m *mockClient) Submit(ctx context.Context, entry audit.QueueEntry) error {
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
