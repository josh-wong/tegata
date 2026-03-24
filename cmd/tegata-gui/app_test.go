package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/josh-wong/tegata/pkg/model"
)

// testPassphrase is a fixed passphrase used across all adapter tests.
const testPassphrase = "test-passphrase-12345"

// setupTestVault creates a temporary directory with a new vault and returns
// the vault file path and a cleanup function. The vault is created with fast
// KDF params to keep tests quick.
func setupTestVault(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "vault.tegata")

	params := crypto.KDFParams{
		Time:    1,
		Memory:  8,
		Threads: 1,
		KeyLen:  32,
	}

	_, err := vault.Create(vaultPath, []byte(testPassphrase), params)
	if err != nil {
		t.Fatalf("creating test vault: %v", err)
	}

	return vaultPath
}

func TestAdapter_CreateAndUnlock(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "vault.tegata")

	app := NewApp()

	// Override DefaultParams for test speed: use minimal Argon2id params.
	// We call vault.Create directly with fast params instead of app.CreateVault
	// which uses crypto.DefaultParams (too slow for tests).
	params := crypto.KDFParams{Time: 1, Memory: 8, Threads: 1, KeyLen: 32}
	_, err := vault.Create(vaultPath, []byte(testPassphrase), params)
	if err != nil {
		t.Fatalf("creating vault: %v", err)
	}

	// Unlock via the adapter with fast params vault.
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath
	app.locked = false

	creds, err := app.ListCredentials()
	if err != nil {
		t.Fatalf("listing credentials: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("expected 0 credentials, got %d", len(creds))
	}

	app.LockVault()
}

func TestAdapter_AddAndListCredentials(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	// Unlock the vault.
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath

	// Add a TOTP credential.
	id, err := app.AddCredential("test-totp", "TestIssuer", "totp", "JBSWY3DPEHPK3PXP", "SHA1", 6, 30, []string{"test"})
	if err != nil {
		t.Fatalf("adding credential: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty credential ID")
	}

	creds, err := app.ListCredentials()
	if err != nil {
		t.Fatalf("listing credentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].Label != "test-totp" {
		t.Errorf("expected label 'test-totp', got %q", creds[0].Label)
	}

	app.LockVault()
}

func TestAdapter_RemoveCredential(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath

	// Add then remove a credential.
	id, err := app.AddCredential("to-remove", "Issuer", "totp", "JBSWY3DPEHPK3PXP", "SHA1", 6, 30, nil)
	if err != nil {
		t.Fatalf("adding credential: %v", err)
	}

	if err := app.RemoveCredential(id); err != nil {
		t.Fatalf("removing credential: %v", err)
	}

	creds, err := app.ListCredentials()
	if err != nil {
		t.Fatalf("listing credentials: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("expected 0 credentials after removal, got %d", len(creds))
	}

	app.LockVault()
}

func TestAdapter_GenerateTOTP(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath

	// Add a TOTP credential with a known secret.
	_, err = app.AddCredential("totp-test", "Issuer", "totp", "JBSWY3DPEHPK3PXP", "SHA1", 6, 30, nil)
	if err != nil {
		t.Fatalf("adding credential: %v", err)
	}

	result, err := app.GenerateTOTP("totp-test")
	if err != nil {
		t.Fatalf("generating TOTP: %v", err)
	}

	if len(result.Code) != 6 {
		t.Errorf("expected 6-digit code, got %q (length %d)", result.Code, len(result.Code))
	}
	if result.Remaining < 1 || result.Remaining > 30 {
		t.Errorf("expected remaining between 1-30, got %d", result.Remaining)
	}

	app.LockVault()
}

func TestAdapter_ScanForVaults_EnvVar(t *testing.T) {
	// Create a temp directory with a vault.tegata file.
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "vault.tegata")
	if err := os.WriteFile(vaultPath, []byte("test"), 0600); err != nil {
		t.Fatalf("creating test vault file: %v", err)
	}

	// Set TEGATA_VAULT to the directory.
	t.Setenv("TEGATA_VAULT", dir)

	app := NewApp()
	results, err := app.ScanForVaults()
	if err != nil {
		t.Fatalf("scanning for vaults: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 vault location from TEGATA_VAULT")
	}

	// The first result should come from the env var.
	if results[0].DriveName != "TEGATA_VAULT" {
		t.Errorf("expected first result DriveName='TEGATA_VAULT', got %q", results[0].DriveName)
	}
	if results[0].Path != vaultPath {
		t.Errorf("expected first result Path=%q, got %q", vaultPath, results[0].Path)
	}
}

func TestAdapter_LockVault(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath
	app.locked = false

	app.LockVault()

	if app.vault != nil {
		t.Error("expected vault to be nil after LockVault")
	}
	if !app.locked {
		t.Error("expected locked to be true after LockVault")
	}

	// ListCredentials should fail when locked.
	_, err = app.ListCredentials()
	if err == nil {
		t.Error("expected error when listing credentials on locked vault")
	}
}

func TestAdapter_ChangePassphrase(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath

	// Valid change should succeed.
	newPass := "new-passphrase-12345"
	if err := app.ChangePassphrase(testPassphrase, newPass); err != nil {
		t.Fatalf("changing passphrase: %v", err)
	}

	// Lock and verify new passphrase works.
	app.LockVault()

	mgr2, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("reopening vault: %v", err)
	}
	if err := mgr2.Unlock([]byte(newPass)); err != nil {
		mgr2.Close()
		t.Fatalf("unlocking with new passphrase: %v", err)
	}
	mgr2.Close()

	// Old passphrase should no longer work.
	mgr3, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("reopening vault: %v", err)
	}
	if err := mgr3.Unlock([]byte(testPassphrase)); err == nil {
		mgr3.Close()
		t.Fatal("expected old passphrase to fail after change")
	}
	mgr3.Close()
}

func TestAdapter_ChangePassphrase_WrongCurrent(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath

	if err := app.ChangePassphrase("wrong-passphrase", "new-passphrase-12345"); err == nil {
		t.Fatal("expected error when current passphrase is wrong")
	}

	app.LockVault()
}

func TestAdapter_ChangePassphrase_TooShort(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath

	if err := app.ChangePassphrase(testPassphrase, "short"); err == nil {
		t.Fatal("expected error for short new passphrase")
	}

	app.LockVault()
}

func TestAdapter_ChangePassphrase_RequiresUnlockedVault(t *testing.T) {
	app := NewApp()
	if err := app.ChangePassphrase("old", "new-passphrase-12345"); err == nil {
		t.Fatal("expected error when vault is locked")
	}
}

// Ensure VaultLocation and TOTPResult types are exported correctly for binding.
func TestAdapter_TypeExports(t *testing.T) {
	vl := VaultLocation{Path: "/test/vault.tegata", DriveName: "USB"}
	if vl.Path != "/test/vault.tegata" {
		t.Errorf("unexpected VaultLocation.Path: %q", vl.Path)
	}

	tr := TOTPResult{Code: "123456", Remaining: 15}
	if tr.Code != "123456" {
		t.Errorf("unexpected TOTPResult.Code: %q", tr.Code)
	}

	_ = model.CredentialTOTP // Ensure model package is accessible.
}

// TestApp_AuditBuilderNilSafe verifies that credential actions do not panic
// when builder is nil (audit not configured). The nil-guard must protect
// every LogEvent call site.
func TestApp_AuditBuilderNilSafe(t *testing.T) {
	vaultPath := setupTestVault(t)

	app := NewApp()

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatalf("opening vault: %v", err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatalf("unlocking vault: %v", err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath
	app.locked = false
	app.config = config.DefaultConfig()
	// builder is nil (audit not configured)

	// Add a TOTP credential.
	_, err = app.AddCredential("test-totp", "Test", "totp", "JBSWY3DPEHPK3PXP", "SHA1", 6, 30, nil)
	if err != nil {
		t.Fatalf("adding credential: %v", err)
	}

	// GenerateTOTP with nil builder should not panic.
	result, err := app.GenerateTOTP("test-totp")
	if err != nil {
		t.Fatalf("GenerateTOTP: %v", err)
	}
	if result.Code == "" {
		t.Error("expected non-empty TOTP code")
	}

	app.LockVault()
}

// TestApp_LockClearsBuilder verifies that LockVault closes the EventBuilder
// and sets the field to nil so resources are released.
func TestApp_LockClearsBuilder(t *testing.T) {
	app := NewApp()
	builder, err := audit.NewEventBuilder(nil, "", nil, 0)
	if err != nil {
		t.Fatalf("creating disabled builder: %v", err)
	}
	app.builder = builder

	app.LockVault()

	if app.builder != nil {
		t.Error("expected builder to be nil after LockVault")
	}
}
