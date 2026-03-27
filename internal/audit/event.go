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

// BuildLabelMap creates a hash→label lookup table from a list of label strings.
// This allows client-side resolution of label hashes back to human-readable
// names without exposing plaintext labels in the audit ledger.
func BuildLabelMap(labels []string) map[string]string {
	m := make(map[string]string, len(labels))
	for _, label := range labels {
		m[HashString(label)] = label
	}
	return m
}

// ResolveLabel returns the human-readable label for a hash if found in the
// lookup map, otherwise returns "(deleted)" to indicate the credential no
// longer exists in the vault.
func ResolveLabel(labelHash string, labelMap map[string]string) string {
	if name, ok := labelMap[labelHash]; ok {
		return name
	}
	return "(deleted)"
}

// FormatOperation returns a human-readable display string for an operation
// type stored in audit records (e.g. "totp" → "TOTP", "challenge-response" →
// "Challenge-response").
func FormatOperation(op string) string {
	switch op {
	case "totp":
		return "TOTP"
	case "hotp":
		return "HOTP"
	case "static":
		return "Static password"
	case "challenge-response":
		return "Challenge-response"
	default:
		return op
	}
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
