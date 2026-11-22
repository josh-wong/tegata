// Package model defines the shared data types used across Tegata. These types
// represent the vault file structure and credential records as specified in the
// design document sections 3.1 and 3.2.
package model

import "time"

// CredentialType identifies the kind of authentication credential stored in the
// vault. The string values match the JSON schema defined in the design doc.
type CredentialType string

const (
	// CredentialTOTP represents a time-based one-time password credential.
	CredentialTOTP CredentialType = "totp"

	// CredentialHOTP represents an HMAC-based one-time password credential.
	CredentialHOTP CredentialType = "hotp"

	// CredentialChallengeResponse represents an HMAC-SHA256 challenge-response
	// signing credential.
	CredentialChallengeResponse CredentialType = "challenge-response"

	// CredentialStatic represents a static password credential.
	CredentialStatic CredentialType = "static"
)

// String returns the string representation of the credential type.
func (ct CredentialType) String() string {
	return string(ct)
}

// Credential represents a single authentication entry in the vault. Fields vary
// by type: TOTP uses Algorithm/Digits/Period, HOTP uses Algorithm/Digits/Counter,
// challenge-response uses Algorithm, and static uses only the shared fields.
type Credential struct {
	ID         string         `json:"id"`
	Label      string         `json:"label"`
	Issuer     string         `json:"issuer,omitempty"`
	Type       CredentialType `json:"type"`
	Algorithm  string         `json:"algorithm,omitempty"`
	Digits     int            `json:"digits,omitempty"`
	Period     int            `json:"period,omitempty"`
	Counter    uint64         `json:"counter,omitempty"`
	Secret     string         `json:"secret"`
	Tags       []string       `json:"tags"`
	Category   string         `json:"category,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	ModifiedAt time.Time      `json:"modified_at"`
}

// VaultPayload represents the decrypted inner JSON document of a vault file.
// After AES-256-GCM decryption, the blob is deserialized into this structure.
type VaultPayload struct {
	Version         int          `json:"version"`
	CreatedAt       time.Time    `json:"created_at"`
	ModifiedAt      time.Time    `json:"modified_at"`
	Credentials     []Credential `json:"credentials"`
	RecoveryKeyHash string       `json:"recovery_key_hash"`
	VaultID         string            `json:"vault_id,omitempty"`
	AuditHashes     map[string]string `json:"audit_hashes,omitempty"`
	// DeletedLabels maps labelHash → label for credentials that have been removed.
	// Entries are never pruned; the map grows by one entry per removed credential
	// so that audit history can display "Label (deleted)" indefinitely.
	// TODO: Add a pruning strategy (e.g., remove entries whose hash appears in no
	// audit record fetched within the last N months) to bound map growth for
	// long-lived vaults with frequent credential rotation.
	DeletedLabels map[string]string `json:"deleted_labels,omitempty"`
	// Secrets stores auxiliary secrets such as the HMAC secret for ScalarDL Ledger.
	// These are stored encrypted in the vault and never persisted to tegata.toml.
	Secrets map[string]string `json:"secrets,omitempty"`
	// Owner records the hashed organizational identifiers (employee ID and
	// email) that attribute this vault's audit events to a person in managed
	// multi-user deployments. Only hashes are stored; raw identifiers are never
	// persisted. Nil when no identifier has been configured.
	Owner *OwnerIdentity `json:"owner,omitempty"`
}

// OwnerIdentity holds the hashed organizational identifiers for the vault
// owner. Values are hex-encoded digests produced by audit.HashOwnerIdentifier;
// HashScheme records how they were computed so administrators can reproduce
// them ("sha256" or "hmac-sha256").
type OwnerIdentity struct {
	EmployeeIDHash string    `json:"employee_id_hash,omitempty"`
	EmailHash      string    `json:"email_hash,omitempty"`
	HashScheme     string    `json:"hash_scheme,omitempty"`
	// SaltID is a short, non-reversible fingerprint of the organization salt
	// in effect when the hashes were computed (empty when unsalted). It lets
	// Tegata detect that the salt changed so a kept hash is never mixed with
	// a hash computed under a different salt.
	SaltID    string    `json:"salt_id,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsZero reports whether no identifier hash is set.
func (o *OwnerIdentity) IsZero() bool {
	return o == nil || (o.EmployeeIDHash == "" && o.EmailHash == "")
}

// PrimaryHash returns the identifier hash used for operational ownership
// mapping. The employee ID is the recommended primary identifier, so its hash
// takes precedence; the email hash is used only when no employee ID is set.
func (o *OwnerIdentity) PrimaryHash() string {
	if o == nil {
		return ""
	}
	if o.EmployeeIDHash != "" {
		return o.EmployeeIDHash
	}
	return o.EmailHash
}

// VaultHeader represents the plaintext header of a vault file as specified in
// design doc section 3.1. The header serializes to exactly 128 bytes in
// big-endian format at explicit byte offsets.
//
// Layout (128 bytes total):
//
//	magic(8) + version(2) + argonTime(4) + argonMemory(4) + argonParallelism(1)
//	+ salt(32) + recoverySalt(32) + writeCounter(8)
//	+ failedAttempts(1) + lastAttemptTime(8) + reserved(28) = 128
//
// The nonce is never stored on disk; it is derived deterministically from
// WriteCounter as counter_be8 || zeros4.
type VaultHeader struct {
	Magic            [8]byte
	Version          uint16
	ArgonTime        uint32
	ArgonMemory      uint32
	ArgonParallelism uint8
	Salt             [32]byte
	RecoveryKeySalt  [32]byte
	WriteCounter     uint64
	FailedAttempts   uint8
	LastAttemptTime  int64
	Reserved         [28]byte
}
