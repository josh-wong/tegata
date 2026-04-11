package audit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
)

// NewEventBuilderFromConfig constructs an EventBuilder from config and vault
// passphrase. Returns a disabled (no-op) builder when cfg.Enabled is false.
// On ledger connection failure, returns a disabled builder — audit errors
// must never block authentication commands (per D-11).
//
// Queue key derivation (AUDT-08): the queue is AES-256-GCM encrypted using a
// 32-byte key derived from the vault passphrase via Argon2id with a distinct
// salt. The salt is stored in the 32-byte queue file header.
func NewEventBuilderFromConfig(cfg config.AuditConfig, vaultDir string, passphrase []byte) (*EventBuilder, error) {
	if !cfg.Enabled {
		return NewEventBuilder(nil, "", nil, 0)
	}

	queuePath := filepath.Join(vaultDir, "queue.tegata")

	// Read the Argon2id salt from the existing queue file header, or
	// generate a new one when the file does not yet exist.
	var queueSalt []byte
	if data, err := os.ReadFile(queuePath); err == nil && len(data) >= 32 {
		queueSalt = make([]byte, 32)
		copy(queueSalt, data[:32])
	} else {
		var genErr error
		queueSalt, genErr = crypto.GenerateSalt()
		if genErr != nil {
			return nil, fmt.Errorf("generating queue salt: %w", genErr)
		}
	}

	// Derive the 32-byte queue encryption key using Argon2id.
	keyBuf := crypto.DeriveKey(passphrase, queueSalt, crypto.DefaultParams)
	defer keyBuf.Destroy()

	// Copy key bytes out of the SecretBuffer before it is destroyed.
	// Note: queueKey is NOT zeroed here — EventBuilder owns it for the
	// lifetime of the command and will use it for queue Save operations.
	queueKey := make([]byte, 32)
	copy(queueKey, keyBuf.Bytes())

	client, err := NewClientFromConfig(cfg)
	if err != nil {
		zeroSlice(queueKey)
		// A failed ledger connection is not fatal — the queue will hold events.
		fmt.Fprintf(os.Stderr, "tegata: audit ledger unavailable (%v); events will be queued\n", err)
		return NewEventBuilder(nil, "", nil, 0)
	}

	return NewEventBuilder(client, queuePath, queueKey, cfg.QueueMaxEvents)
}

// zeroSlice overwrites b with zeros.
func zeroSlice(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
