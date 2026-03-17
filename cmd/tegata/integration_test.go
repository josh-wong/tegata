package main

import (
	"encoding/base32"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/josh-wong/tegata/pkg/model"
)

// testParams uses minimal Argon2id settings for fast tests.
var testParams = crypto.KDFParams{
	Time:    1,
	Memory:  64,
	Threads: 1,
	KeyLen:  32,
}

func createIntegrationVault(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.tegata")
	recoveryKey, err := vault.Create(path, []byte("integration-test-passphrase"), testParams)
	if err != nil {
		t.Fatalf("vault.Create: %v", err)
	}
	return path, recoveryKey
}

func TestIntegration_FullLifecycle(t *testing.T) {
	path, _ := createIntegrationVault(t)

	// Open and unlock.
	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer mgr.Close()

	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Add TOTP credential.
	totp := model.Credential{
		Label:     "GitHub",
		Issuer:    "GitHub",
		Type:      model.CredentialTOTP,
		Algorithm: "SHA1",
		Digits:    6,
		Period:    30,
		Secret:    "JBSWY3DPEHPK3PXP",
	}
	totpID, err := mgr.AddCredential(totp)
	if err != nil {
		t.Fatalf("AddCredential TOTP: %v", err)
	}

	// Add HOTP credential.
	hotp := model.Credential{
		Label:     "AWS",
		Issuer:    "Amazon",
		Type:      model.CredentialHOTP,
		Algorithm: "SHA1",
		Digits:    6,
		Counter:   0,
		Secret:    "GEZDGNBVGY3TQOJQ",
	}
	_, err = mgr.AddCredential(hotp)
	if err != nil {
		t.Fatalf("AddCredential HOTP: %v", err)
	}

	// Add static credential.
	static := model.Credential{
		Label:  "backup-key",
		Type:   model.CredentialStatic,
		Secret: "my-static-password-123",
	}
	_, err = mgr.AddCredential(static)
	if err != nil {
		t.Fatalf("AddCredential static: %v", err)
	}

	// List should have 3 entries.
	list := mgr.ListCredentials()
	if len(list) != 3 {
		t.Fatalf("ListCredentials: got %d, want 3", len(list))
	}

	// Generate TOTP code from stored credential.
	cred, err := mgr.GetCredential("GitHub")
	if err != nil {
		t.Fatalf("GetCredential GitHub: %v", err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(cred.Secret))
	if err != nil {
		t.Fatalf("decoding secret: %v", err)
	}
	code, remaining := auth.GenerateTOTP(secret, time.Now(), cred.Period, cred.Digits, cred.Algorithm)
	if len(code) != 6 {
		t.Errorf("TOTP code length: got %d, want 6", len(code))
	}
	if remaining <= 0 || remaining > 30 {
		t.Errorf("TOTP remaining: got %d, want 1-30", remaining)
	}

	// Get static password.
	staticCred, err := mgr.GetCredential("backup-key")
	if err != nil {
		t.Fatalf("GetCredential backup-key: %v", err)
	}
	password, err := auth.GetStaticPassword(staticCred)
	if err != nil {
		t.Fatalf("GetStaticPassword: %v", err)
	}
	if password != "my-static-password-123" {
		t.Errorf("static password: got %q, want %q", password, "my-static-password-123")
	}

	// Remove TOTP credential.
	if err := mgr.RemoveCredential(totpID); err != nil {
		t.Fatalf("RemoveCredential: %v", err)
	}
	list = mgr.ListCredentials()
	if len(list) != 2 {
		t.Fatalf("ListCredentials after remove: got %d, want 2", len(list))
	}
}

func TestIntegration_WrongPassphrase(t *testing.T) {
	path, _ := createIntegrationVault(t)

	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer mgr.Close()

	err = mgr.Unlock([]byte("wrong-passphrase"))
	if !errors.Is(err, errors.ErrAuthFailed) {
		t.Fatalf("expected ErrAuthFailed, got %v", err)
	}

	if mgr.Header().FailedAttempts != 1 {
		t.Errorf("FailedAttempts: got %d, want 1", mgr.Header().FailedAttempts)
	}
}

func TestIntegration_RateLimiting(t *testing.T) {
	path, _ := createIntegrationVault(t)

	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer mgr.Close()

	// Simulate 5 failed attempts by directly setting header state.
	for i := 0; i < 5; i++ {
		_ = mgr.Unlock([]byte("wrong"))
		// Move time back so we can try again quickly.
		mgr.Header().LastAttemptTime = mgr.Header().LastAttemptTime - 300
	}

	if mgr.Header().FailedAttempts != 5 {
		t.Errorf("FailedAttempts: got %d, want 5", mgr.Header().FailedAttempts)
	}

	// Reset time to now so rate limit is active.
	mgr.Header().LastAttemptTime = time.Now().Unix()

	// Next attempt should be rate-limited.
	err = mgr.Unlock([]byte("integration-test-passphrase"))
	if !errors.Is(err, errors.ErrAuthFailed) {
		t.Fatalf("expected ErrAuthFailed (rate-limited), got %v", err)
	}
}

func TestIntegration_RecoveryKey(t *testing.T) {
	path, recoveryKey := createIntegrationVault(t)

	// Add a credential, then close.
	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	_, err = mgr.AddCredential(model.Credential{
		Label:  "TestService",
		Type:   model.CredentialTOTP,
		Secret: "JBSWY3DPEHPK3PXP",
	})
	if err != nil {
		t.Fatalf("AddCredential: %v", err)
	}
	mgr.Close()

	// Reopen and unlock with recovery key.
	mgr2, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open for recovery: %v", err)
	}
	defer mgr2.Close()

	cleanKey := strings.ReplaceAll(recoveryKey, "-", "")
	rawKey, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanKey)
	if err != nil {
		t.Fatalf("decoding recovery key: %v", err)
	}

	if err := mgr2.UnlockWithRecoveryKey(rawKey); err != nil {
		t.Fatalf("UnlockWithRecoveryKey: %v", err)
	}

	list := mgr2.ListCredentials()
	if len(list) != 1 {
		t.Fatalf("ListCredentials after recovery: got %d, want 1", len(list))
	}
	if list[0].Label != "TestService" {
		t.Errorf("Label: got %q, want %q", list[0].Label, "TestService")
	}
}

func TestIntegration_OTPAuthParsing(t *testing.T) {
	uris := []struct {
		uri       string
		label     string
		issuer    string
		credType  model.CredentialType
		digits    int
		period    int
		algorithm string
	}{
		{
			uri:       "otpauth://totp/GitHub:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=GitHub&digits=6&period=30",
			label:     "user@example.com",
			issuer:    "GitHub",
			credType:  model.CredentialTOTP,
			digits:    6,
			period:    30,
			algorithm: "SHA1",
		},
		{
			uri:       "otpauth://hotp/AWS:admin?secret=GEZDGNBVGY3TQOJQ&issuer=AWS&counter=42",
			label:     "admin",
			issuer:    "AWS",
			credType:  model.CredentialHOTP,
			digits:    6,
			period:    0,
			algorithm: "SHA1",
		},
	}

	for _, tc := range uris {
		cred, err := auth.ParseOTPAuthURI(tc.uri)
		if err != nil {
			t.Errorf("ParseOTPAuthURI(%q): %v", tc.uri, err)
			continue
		}
		if cred.Label != tc.label {
			t.Errorf("Label: got %q, want %q", cred.Label, tc.label)
		}
		if cred.Issuer != tc.issuer {
			t.Errorf("Issuer: got %q, want %q", cred.Issuer, tc.issuer)
		}
		if cred.Type != tc.credType {
			t.Errorf("Type: got %q, want %q", cred.Type, tc.credType)
		}

		// Verify we can generate codes from parsed credentials.
		if cred.Type == model.CredentialTOTP {
			secret, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
				strings.ToUpper(cred.Secret))
			code, _ := auth.GenerateTOTP(secret, time.Now(), cred.Period, cred.Digits, cred.Algorithm)
			if len(code) != cred.Digits {
				t.Errorf("TOTP code from parsed URI: got %d digits, want %d", len(code), cred.Digits)
			}
		}
	}
}

func TestIntegration_HOTPCounterPersistence(t *testing.T) {
	path, _ := createIntegrationVault(t)

	// Add HOTP credential and generate a code.
	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	hotp := model.Credential{
		Label:     "HOTPService",
		Type:      model.CredentialHOTP,
		Algorithm: "SHA1",
		Digits:    6,
		Counter:   0,
		Secret:    "GEZDGNBVGY3TQOJQ",
	}
	_, err = mgr.AddCredential(hotp)
	if err != nil {
		t.Fatalf("AddCredential: %v", err)
	}

	// Get the credential and increment counter.
	cred, err := mgr.GetCredential("HOTPService")
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	secret, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(cred.Secret))
	_ = auth.GenerateHOTP(secret, cred.Counter, cred.Digits, cred.Algorithm)
	cred.Counter++
	if err := mgr.UpdateCredential(cred); err != nil {
		t.Fatalf("UpdateCredential: %v", err)
	}
	mgr.Close()

	// Reopen and verify counter persisted.
	mgr2, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open after counter update: %v", err)
	}
	defer mgr2.Close()

	if err := mgr2.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	cred2, err := mgr2.GetCredential("HOTPService")
	if err != nil {
		t.Fatalf("GetCredential after reopen: %v", err)
	}
	if cred2.Counter != 1 {
		t.Errorf("Counter: got %d, want 1", cred2.Counter)
	}
}

func TestIntegration_ConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.tegata")

	_, err := vault.Create(path, []byte("config-test"), testParams)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write config defaults.
	if err := config.WriteDefaults(dir); err != nil {
		t.Fatalf("WriteDefaults: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ClipboardTimeout != 45*time.Second {
		t.Errorf("ClipboardTimeout: got %v, want 45s", cfg.ClipboardTimeout)
	}
	if cfg.IdleTimeout != 300*time.Second {
		t.Errorf("IdleTimeout: got %v, want 300s", cfg.IdleTimeout)
	}
}

func TestIntegration_Sign(t *testing.T) {
	t.Skip("stub — implement after sign command is built in Task 2")
}

func TestIntegration_StaticBinaryBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "tegata")

	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", outPath, "./")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = filepath.Join(".") // cmd/tegata directory
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}

	// Verify size under 20MB.
	maxSize := int64(20 * 1024 * 1024)
	if info.Size() > maxSize {
		t.Errorf("binary size %d bytes exceeds %d bytes", info.Size(), maxSize)
	}

	// Verify it's a valid executable (check ELF magic bytes).
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open binary: %v", err)
	}
	defer func() { _ = f.Close() }()

	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		t.Fatalf("read magic bytes: %v", err)
	}

	// ELF magic: 0x7f 'E' 'L' 'F'
	isELF := magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F'
	// PE magic: 'M' 'Z'
	isPE := magic[0] == 'M' && magic[1] == 'Z'
	// Mach-O magic
	isMachO := (magic[0] == 0xfe && magic[1] == 0xed && magic[2] == 0xfa) ||
		(magic[0] == 0xcf && magic[1] == 0xfa && magic[2] == 0xed)

	if !isELF && !isPE && !isMachO {
		t.Errorf("binary does not have recognized executable magic bytes: %x", magic)
	}
}
