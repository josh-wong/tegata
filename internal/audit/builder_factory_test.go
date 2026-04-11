package audit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
)

func TestNewEventBuilderFromConfig_Disabled(t *testing.T) {
	cfg := config.AuditConfig{Enabled: false}
	builder, err := audit.NewEventBuilderFromConfig(cfg, t.TempDir(), []byte("passphrase"))
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if builder == nil {
		t.Fatal("expected non-nil builder for disabled config")
	}
}

func TestNewEventBuilderFromConfig_Enabled_NoQueue(t *testing.T) {
	cfg := config.AuditConfig{
		Enabled:          true,
		Server:           "localhost:50051",
		PrivilegedServer: "localhost:50052",
		EntityID:         "test-entity",
		KeyVersion:       1,
		SecretKey:        "test-secret",
		Insecure:         true,
		QueueMaxEvents:   100,
	}
	vaultDir := t.TempDir()

	// Client will fail to connect (no server running); per D-11, should
	// return a disabled (no-op) builder rather than an error.
	builder, err := audit.NewEventBuilderFromConfig(cfg, vaultDir, []byte("passphrase"))
	if err != nil {
		t.Fatalf("expected nil error on client connect failure, got: %v", err)
	}
	if builder == nil {
		t.Fatal("expected non-nil disabled builder when ledger is unreachable")
	}
}

func TestNewEventBuilderFromConfig_Enabled_ExistingQueue(t *testing.T) {
	cfg := config.AuditConfig{
		Enabled:          true,
		Server:           "localhost:50051",
		PrivilegedServer: "localhost:50052",
		EntityID:         "test-entity",
		KeyVersion:       1,
		SecretKey:        "test-secret",
		Insecure:         true,
		QueueMaxEvents:   100,
	}
	vaultDir := t.TempDir()

	// Write a valid queue file: 32-byte salt header + empty JSON array.
	salt := make([]byte, 32)
	for i := range salt {
		salt[i] = byte(i)
	}
	content := append(salt, []byte("[]")...)
	queuePath := filepath.Join(vaultDir, "queue.tegata")
	if err := os.WriteFile(queuePath, content, 0600); err != nil {
		t.Fatalf("writing fake queue file: %v", err)
	}

	// Client will fail to connect; per D-11, returns disabled builder.
	builder, err := audit.NewEventBuilderFromConfig(cfg, vaultDir, []byte("passphrase"))
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
}
