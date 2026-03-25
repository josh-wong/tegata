// Package audit provides data types and utilities for tamper-evident audit
// logging. AuthEvent records authentication operations with all identifying
// fields hashed to protect user privacy even in the audit log.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// AuthEvent is an immutable audit record for a single authentication operation.
// All identifying strings (label, service, host) are stored as SHA-256 hashes
// so no plaintext credential names appear in audit logs or on-disk queues.
type AuthEvent struct {
	EventID       string    `json:"event_id"`
	Timestamp     time.Time `json:"timestamp"`
	OperationType string    `json:"operation_type"`
	LabelHash     string    `json:"label_hash"`
	ServiceHash   string    `json:"service_hash"`
	HostHash      string    `json:"host_hash"`
	Success       bool      `json:"success"`
	PrevHash      string    `json:"prev_hash"`
}

// HashString returns the lowercase hex-encoded SHA-256 digest of s. Used to
// hash labels, service names, and host names before including them in audit
// records.
func HashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// NewAuthEvent constructs an AuthEvent with a UUID v4 EventID, a UTC timestamp,
// and hashed label/service/host fields. The prevHash argument is the SHA-256
// of the previous queue entry's JSON (or "" for the first event) and is passed
// through directly — callers are responsible for computing the chain linkage.
func NewAuthEvent(opType, label, service, host string, success bool, prevHash string) AuthEvent {
	return AuthEvent{
		EventID:       "tegata-" + uuid.New().String(),
		Timestamp:     time.Now().UTC(),
		OperationType: opType,
		LabelHash:     HashString(label),
		ServiceHash:   HashString(service),
		HostHash:      HashString(host),
		Success:       success,
		PrevHash:      prevHash,
	}
}
