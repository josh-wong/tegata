package vault

import (
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/crypto/guard"
	"github.com/josh-wong/tegata/pkg/model"
)

// Manager provides access to an opened vault file. Call Unlock to decrypt the
// payload before performing credential operations. Always defer Close to zero
// sensitive memory.
type Manager struct {
	path             string
	header           *model.VaultHeader
	payload          *model.VaultPayload
	dek              *guard.KeyEnclave
	recoveryWrapped  []byte
	params           crypto.KDFParams
	unlocked         bool
}

// Create initializes a new encrypted vault at the given path using the provided
// passphrase. It returns a recovery key that can be used to recover the vault
// if the passphrase is forgotten.
func Create(path string, passphrase []byte, params crypto.KDFParams) (recoveryKey string, err error) {
	panic("not implemented")
}

// Open opens an existing vault file at the given path without decrypting it.
// Call Unlock with the passphrase to access credentials.
func Open(path string) (*Manager, error) {
	panic("not implemented")
}

// Unlock decrypts the vault payload using the provided passphrase.
func (m *Manager) Unlock(passphrase []byte) error {
	panic("not implemented")
}

// UnlockWithRecoveryKey decrypts the vault using a base32-encoded recovery key.
func (m *Manager) UnlockWithRecoveryKey(recoveryKeyBase32 string, params crypto.KDFParams) error {
	panic("not implemented")
}

// Save persists the vault to disk using a temp-file-rename strategy.
func (m *Manager) Save() error {
	panic("not implemented")
}

// Close zeroes all sensitive memory held by the manager.
func (m *Manager) Close() {
	panic("not implemented")
}

// AddCredential adds a credential to the vault and saves.
func (m *Manager) AddCredential(cred model.Credential) (string, error) {
	panic("not implemented")
}

// RemoveCredential removes a credential by ID and saves.
func (m *Manager) RemoveCredential(id string) error {
	panic("not implemented")
}

// GetCredential returns the credential matching the given label (case-insensitive).
func (m *Manager) GetCredential(label string) (*model.Credential, error) {
	panic("not implemented")
}

// ListCredentials returns a copy of all credentials in the vault.
func (m *Manager) ListCredentials() []model.Credential {
	panic("not implemented")
}
