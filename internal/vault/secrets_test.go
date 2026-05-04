package vault

import (
"testing"

"github.com/josh-wong/tegata/internal/crypto"
)

func TestManager_GetSetSecret(t *testing.T) {
tmpDir := t.TempDir()
path := tmpDir + "/test.vault"
passphrase := []byte("test-passphrase-1234567890")

// Create a new vault.
_, err := Create(path, passphrase, crypto.DefaultParams)
if err != nil {
t.Fatalf("Create failed: %v", err)
}

// Open and unlock the vault.
mgr, err := Open(path)
if err != nil {
t.Fatalf("Open failed: %v", err)
}
defer mgr.Close()

if err := mgr.Unlock(passphrase); err != nil {
t.Fatalf("Unlock failed: %v", err)
}

// Test SetSecret and GetSecret.
testSecret := "test-hmac-secret-key-1234567890"
if err := mgr.SetSecret("audit.secret_key", testSecret); err != nil {
t.Fatalf("SetSecret failed: %v", err)
}

retrieved := mgr.GetSecret("audit.secret_key")
if retrieved != testSecret {
t.Errorf("GetSecret returned %q, want %q", retrieved, testSecret)
}

// Close and reopen to verify persistence.
mgr.Close()

mgr2, err := Open(path)
if err != nil {
t.Fatalf("Open (second) failed: %v", err)
}
defer mgr2.Close()

if err := mgr2.Unlock(passphrase); err != nil {
t.Fatalf("Unlock (second) failed: %v", err)
}

retrieved2 := mgr2.GetSecret("audit.secret_key")
if retrieved2 != testSecret {
t.Errorf("GetSecret (after reopen) returned %q, want %q", retrieved2, testSecret)
}
}

func TestManager_GetSecret_Locked(t *testing.T) {
tmpDir := t.TempDir()
path := tmpDir + "/test.vault"
passphrase := []byte("test-passphrase-1234567890")

// Create and unlock a vault.
_, err := Create(path, passphrase, crypto.DefaultParams)
if err != nil {
t.Fatalf("Create failed: %v", err)
}

mgr, err := Open(path)
if err != nil {
t.Fatalf("Open failed: %v", err)
}
defer mgr.Close()

if err := mgr.Unlock(passphrase); err != nil {
t.Fatalf("Unlock failed: %v", err)
}

if err := mgr.SetSecret("audit.secret_key", "test-secret"); err != nil {
t.Fatalf("SetSecret failed: %v", err)
}

// Close the vault (locks it).
mgr.Close()

// Try to get secret from locked vault (should return empty string).
result := mgr.GetSecret("audit.secret_key")
if result != "" {
t.Errorf("GetSecret on locked vault returned %q, want empty string", result)
}
}

func TestManager_SetSecret_Locked(t *testing.T) {
tmpDir := t.TempDir()
path := tmpDir + "/test.vault"
passphrase := []byte("test-passphrase-1234567890")

// Create a vault but don't unlock it.
_, err := Create(path, passphrase, crypto.DefaultParams)
if err != nil {
t.Fatalf("Create failed: %v", err)
}

mgr, err := Open(path)
if err != nil {
t.Fatalf("Open failed: %v", err)
}
defer mgr.Close()

// Try to set secret on locked vault (should return error).
err = mgr.SetSecret("audit.secret_key", "test-secret")
if err == nil {
t.Error("SetSecret on locked vault should have returned an error")
}
}
