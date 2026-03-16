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
}

// VaultHeader represents the plaintext header of a vault file as specified in
// design doc section 3.1. This is a binary format; binary serialization will be
// implemented in Phase 2 when the vault manager is built.
type VaultHeader struct {
	Magic            [8]byte
	Version          uint16
	ArgonTime        uint32
	ArgonMemory      uint32
	ArgonParallelism uint8
	Salt             [32]byte
	RecoveryKeySalt  [32]byte
	WriteCounter     uint64
	Nonce            [12]byte
}
