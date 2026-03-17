package vault

import (
	"encoding/base32"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// testParams uses minimal Argon2id settings for fast tests.
var testParams = crypto.KDFParams{
	Time:    1,
	Memory:  64,
	Threads: 1,
	KeyLen:  32,
}

func createTestVault(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vault")
	passphrase := []byte("test-passphrase")
	recoveryKey, err := Create(path, passphrase, testParams)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return path, recoveryKey
}

func TestCreateAndOpen(t *testing.T) {
	path, recoveryKey := createTestVault(t)
	if recoveryKey == "" {
		t.Error("recovery key is empty")
	}

	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	if m.header.Version != 1 {
		t.Errorf("Version: got %d, want 1", m.header.Version)
	}
	if m.header.Magic != [8]byte{'T', 'E', 'G', 'A', 'T', 'A', 0, 0} {
		t.Errorf("Magic mismatch: %v", m.header.Magic)
	}
}

func TestUnlockCorrectPassphrase(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	err = m.Unlock([]byte("test-passphrase"))
	if err != nil {
		t.Fatalf("Unlock with correct passphrase: %v", err)
	}
}

func TestUnlockWrongPassphrase(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	err = m.Unlock([]byte("wrong-passphrase"))
	if !errors.Is(err, errors.ErrAuthFailed) {
		t.Fatalf("expected ErrAuthFailed, got %v", err)
	}
}

func TestUnlockRecordsFailedAttempt(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	_ = m.Unlock([]byte("wrong"))
	if m.header.FailedAttempts != 1 {
		t.Errorf("FailedAttempts: got %d, want 1", m.header.FailedAttempts)
	}
}

func TestUnlockResetsAttemptsOnSuccess(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	// Fail once then succeed after backoff expires.
	_ = m.Unlock([]byte("wrong"))
	// Move the last attempt time into the past so rate limit doesn't block.
	m.header.LastAttemptTime = m.header.LastAttemptTime - 2
	_ = m.saveHeader()
	err = m.Unlock([]byte("test-passphrase"))
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if m.header.FailedAttempts != 0 {
		t.Errorf("FailedAttempts should be 0 after success, got %d", m.header.FailedAttempts)
	}
}

func TestAddAndListCredentials(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	cred := model.Credential{
		Label:  "GitHub",
		Issuer: "GitHub",
		Type:   model.CredentialTOTP,
		Secret: "JBSWY3DPEHPK3PXP",
	}
	id, err := m.AddCredential(cred)
	if err != nil {
		t.Fatalf("AddCredential: %v", err)
	}
	if id == "" {
		t.Error("AddCredential returned empty ID")
	}

	list := m.ListCredentials()
	if len(list) != 1 {
		t.Fatalf("ListCredentials: got %d, want 1", len(list))
	}
	if list[0].Label != "GitHub" {
		t.Errorf("Label: got %q, want %q", list[0].Label, "GitHub")
	}
}

func TestGetCredential(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	cred := model.Credential{
		Label:  "GitHub",
		Issuer: "GitHub",
		Type:   model.CredentialTOTP,
		Secret: "JBSWY3DPEHPK3PXP",
	}
	_, err = m.AddCredential(cred)
	if err != nil {
		t.Fatalf("AddCredential: %v", err)
	}

	got, err := m.GetCredential("github") // case-insensitive
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if got.Label != "GitHub" {
		t.Errorf("Label: got %q, want %q", got.Label, "GitHub")
	}
}

func TestGetCredentialNotFound(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	_, err = m.GetCredential("nonexistent")
	if !errors.Is(err, errors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRemoveCredential(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	cred := model.Credential{
		Label:  "GitHub",
		Issuer: "GitHub",
		Type:   model.CredentialTOTP,
		Secret: "JBSWY3DPEHPK3PXP",
	}
	id, _ := m.AddCredential(cred)

	err = m.RemoveCredential(id)
	if err != nil {
		t.Fatalf("RemoveCredential: %v", err)
	}

	list := m.ListCredentials()
	if len(list) != 0 {
		t.Errorf("ListCredentials after remove: got %d, want 0", len(list))
	}
}

func TestRemoveCredentialNotFound(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	err = m.RemoveCredential("nonexistent-id")
	if !errors.Is(err, errors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSaveIncrementsWriteCounter(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	initialCounter := m.header.WriteCounter
	if err := m.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if m.header.WriteCounter != initialCounter+1 {
		t.Errorf("WriteCounter: got %d, want %d", m.header.WriteCounter, initialCounter+1)
	}
}

func TestSavePersistsData(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	cred := model.Credential{
		Label:  "Persist",
		Issuer: "Test",
		Type:   model.CredentialTOTP,
		Secret: "JBSWY3DPEHPK3PXP",
	}
	_, err = m.AddCredential(cred)
	if err != nil {
		t.Fatalf("AddCredential: %v", err)
	}
	m.Close()

	// Reopen and verify data persisted.
	m2, err := Open(path)
	if err != nil {
		t.Fatalf("Open after save: %v", err)
	}
	defer m2.Close()

	if err := m2.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock after reopen: %v", err)
	}
	list := m2.ListCredentials()
	if len(list) != 1 {
		t.Fatalf("ListCredentials after reopen: got %d, want 1", len(list))
	}
	if list[0].Label != "Persist" {
		t.Errorf("Label: got %q, want %q", list[0].Label, "Persist")
	}
}

func TestRecoveryKeyUnlock(t *testing.T) {
	path, recoveryKey := createTestVault(t)

	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	// Remove dashes and base32-decode the recovery key.
	cleanKey := strings.ReplaceAll(recoveryKey, "-", "")
	rawKey, decErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanKey)
	if decErr != nil {
		t.Fatalf("decoding recovery key: %v", decErr)
	}

	err = m.UnlockWithRecoveryKey(rawKey)
	if err != nil {
		t.Fatalf("UnlockWithRecoveryKey: %v", err)
	}
}

func TestRecoveryKeyUnlockWrongKey(t *testing.T) {
	path, _ := createTestVault(t)

	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	err = m.UnlockWithRecoveryKey([]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"))
	if !errors.Is(err, errors.ErrAuthFailed) {
		t.Fatalf("expected ErrAuthFailed for wrong recovery key, got %v", err)
	}
}

func TestRecoveryKeyUnlockRecordsFailedAttempt(t *testing.T) {
	path, _ := createTestVault(t)

	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	_ = m.UnlockWithRecoveryKey([]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"))
	if m.header.FailedAttempts != 1 {
		t.Errorf("FailedAttempts: got %d, want 1", m.header.FailedAttempts)
	}
}

func TestRecoveryKeyUnlockRateLimited(t *testing.T) {
	path, _ := createTestVault(t)

	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	// Simulate a recent failed attempt so rate limiting kicks in.
	m.header.FailedAttempts = 3
	m.header.LastAttemptTime = time.Now().Unix()
	_ = m.saveHeader()

	err = m.UnlockWithRecoveryKey([]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"))
	if !errors.Is(err, errors.ErrAuthFailed) {
		t.Fatalf("expected ErrAuthFailed from rate limit, got %v", err)
	}
	// Attempt counter should not have increased because rate limiting
	// rejected the attempt before any crypto work.
	if m.header.FailedAttempts != 3 {
		t.Errorf("FailedAttempts should stay at 3 during rate limit, got %d", m.header.FailedAttempts)
	}
}

func TestOperationsOnLockedVault(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer m.Close()

	// Vault is opened but not unlocked -- operations should fail.
	_, err = m.AddCredential(model.Credential{Label: "test"})
	if !errors.Is(err, errors.ErrVaultLocked) {
		t.Errorf("AddCredential on locked vault: expected ErrVaultLocked, got %v", err)
	}
	err = m.RemoveCredential("id")
	if !errors.Is(err, errors.ErrVaultLocked) {
		t.Errorf("RemoveCredential on locked vault: expected ErrVaultLocked, got %v", err)
	}
	_, err = m.GetCredential("label")
	if !errors.Is(err, errors.ErrVaultLocked) {
		t.Errorf("GetCredential on locked vault: expected ErrVaultLocked, got %v", err)
	}
}

func TestFullLifecycle(t *testing.T) {
	path, recoveryKey := createTestVault(t)
	if recoveryKey == "" {
		t.Fatal("empty recovery key")
	}

	// Open and unlock.
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Add credentials.
	cred1 := model.Credential{Label: "Service1", Type: model.CredentialTOTP, Secret: "SECRET1"}
	id1, err := m.AddCredential(cred1)
	if err != nil {
		t.Fatalf("AddCredential 1: %v", err)
	}

	cred2 := model.Credential{Label: "Service2", Type: model.CredentialHOTP, Secret: "SECRET2"}
	_, err = m.AddCredential(cred2)
	if err != nil {
		t.Fatalf("AddCredential 2: %v", err)
	}

	// List.
	list := m.ListCredentials()
	if len(list) != 2 {
		t.Fatalf("ListCredentials: got %d, want 2", len(list))
	}

	// Get.
	got, err := m.GetCredential("service1")
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if got.Label != "Service1" {
		t.Errorf("Label: got %q, want %q", got.Label, "Service1")
	}

	// Remove.
	if err := m.RemoveCredential(id1); err != nil {
		t.Fatalf("RemoveCredential: %v", err)
	}

	list = m.ListCredentials()
	if len(list) != 1 {
		t.Fatalf("ListCredentials after remove: got %d, want 1", len(list))
	}

	// Close.
	m.Close()

	// Reopen and verify.
	m2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer m2.Close()

	if err := m2.Unlock([]byte("test-passphrase")); err != nil {
		t.Fatalf("Unlock after reopen: %v", err)
	}
	list = m2.ListCredentials()
	if len(list) != 1 {
		t.Fatalf("ListCredentials after reopen: got %d, want 1", len(list))
	}
	if list[0].Label != "Service2" {
		t.Errorf("remaining credential: got %q, want %q", list[0].Label, "Service2")
	}
}
