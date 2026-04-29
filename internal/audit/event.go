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
// records. vault/manager.go calls this function directly when storing
// deleted-label hashes in the vault payload.
func HashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// emptyLabelHash is the SHA-256 of an empty string, used as a sentinel to
// identify events that have no associated credential (e.g. vault-unlock).
var emptyLabelHash = HashString("")

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
// lookup map. Returns "—" for events with no associated credential (e.g.
// vault-unlock), or "(deleted)" when the credential no longer exists.
func ResolveLabel(labelHash string, labelMap map[string]string) string {
	if labelHash == emptyLabelHash {
		return "—"
	}
	if name, ok := labelMap[labelHash]; ok {
		return name
	}
	return "(deleted)"
}

// ResolveLabelWithDeleted resolves a label hash using both the current label
// map and a deleted-labels map. Returns "—" for events with no associated
// credential, "Label (deleted)" for removed credentials, and "(deleted)" for
// hashes with no record (e.g. entries predating this feature).
func ResolveLabelWithDeleted(labelHash string, labelMap, deletedMap map[string]string) string {
	if labelHash == emptyLabelHash {
		return "—"
	}
	if name, ok := labelMap[labelHash]; ok {
		return name
	}
	if name, ok := deletedMap[labelHash]; ok {
		return name + " (deleted)"
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
	case "vault-unlock":
		return "Vault unlock"
	case "vault-lock":
		return "Vault lock"
	case "credential-add":
		return "Credential add"
	case "credential-remove":
		return "Credential remove"
	case "credential-update":
		return "Credential update"
	case "credential-tag-update":
		return "Credential tag update"
	case "hotp-resync":
		return "HOTP resync"
	case "credential-import":
		return "Credential import"
	case "credential-export":
		return "Credential export"
	case "vault-passphrase-change":
		return "Vault passphrase change"
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
