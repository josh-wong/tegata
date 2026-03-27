package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
)

// Compile-time check: mockAuditClient must satisfy audit.Client.
var _ audit.Client = (*mockAuditClient)(nil)

// mockAuditClient implements audit.Client with configurable return values
// for testing verifyContracts without a live ScalarDL instance.
type mockAuditClient struct {
	putErr error
}

func (m *mockAuditClient) Put(_ context.Context, _, _ string) error {
	return m.putErr
}

func (m *mockAuditClient) Get(_ context.Context, _ string) ([]*audit.EventRecord, error) {
	return nil, nil
}

func (m *mockAuditClient) Validate(_ context.Context, _ string) (*audit.ValidationResult, error) {
	return nil, nil
}

func (m *mockAuditClient) RegisterCert(_ context.Context, _ string, _ uint32, _ string) error {
	return nil
}

func (m *mockAuditClient) Ping(_ context.Context) error {
	return nil
}

func (m *mockAuditClient) Close() error {
	return nil
}

func (m *mockAuditClient) Submit(_ context.Context, _ audit.QueueEntry) error {
	return nil
}

func (m *mockAuditClient) PutWithMetadata(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return m.putErr
}

func (m *mockAuditClient) CollectionCreate(_ context.Context, _ string, _ []string) error {
	return nil
}

func (m *mockAuditClient) CollectionAdd(_ context.Context, _ string, _ []string) error {
	return nil
}

func (m *mockAuditClient) CollectionGet(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// TestLedgerSetup_ContractVerification tests the verifyContracts function
// that checks whether generic contracts are registered on the ledger.
func TestLedgerSetup_ContractVerification(t *testing.T) {
	t.Run("succeeds when Put succeeds", func(t *testing.T) {
		mock := &mockAuditClient{putErr: nil}
		err := verifyContracts(context.Background(), mock)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("returns error when Put fails", func(t *testing.T) {
		mock := &mockAuditClient{putErr: fmt.Errorf("CONTRACT_NOT_FOUND")}
		err := verifyContracts(context.Background(), mock)
		if err == nil {
			t.Error("expected error when Put fails, got nil")
		}
	})
}
