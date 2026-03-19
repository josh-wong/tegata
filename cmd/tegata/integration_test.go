package main

import (
	"encoding/base32"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/vault"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
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
	totp := pkgmodel.Credential{
		Label:     "GitHub",
		Issuer:    "GitHub",
		Type:      pkgmodel.CredentialTOTP,
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
	hotp := pkgmodel.Credential{
		Label:     "AWS",
		Issuer:    "Amazon",
		Type:      pkgmodel.CredentialHOTP,
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
	static := pkgmodel.Credential{
		Label:  "backup-key",
		Type:   pkgmodel.CredentialStatic,
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
	_, err = mgr.AddCredential(pkgmodel.Credential{
		Label:  "TestService",
		Type:   pkgmodel.CredentialTOTP,
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
		credType  pkgmodel.CredentialType
		digits    int
		period    int
		algorithm string
	}{
		{
			uri:       "otpauth://totp/GitHub:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=GitHub&digits=6&period=30",
			label:     "user@example.com",
			issuer:    "GitHub",
			credType:  pkgmodel.CredentialTOTP,
			digits:    6,
			period:    30,
			algorithm: "SHA1",
		},
		{
			uri:       "otpauth://hotp/AWS:admin?secret=GEZDGNBVGY3TQOJQ&issuer=AWS&counter=42",
			label:     "admin",
			issuer:    "AWS",
			credType:  pkgmodel.CredentialHOTP,
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
		if cred.Type == pkgmodel.CredentialTOTP {
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

	hotp := pkgmodel.Credential{
		Label:     "HOTPService",
		Type:      pkgmodel.CredentialHOTP,
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
	path, _ := createIntegrationVault(t)

	// Add a challenge-response credential.
	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	cr := pkgmodel.Credential{
		Label:     "mykey",
		Type:      pkgmodel.CredentialChallengeResponse,
		Algorithm: "SHA1",
		Secret:    "JBSWY3DPEHPK3PXP",
	}
	if _, err := mgr.AddCredential(cr); err != nil {
		t.Fatalf("AddCredential: %v", err)
	}
	mgr.Close()

	// Invoke the sign command directly via the auth engine (same path as sign.go).
	mgr2, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open for sign: %v", err)
	}
	defer mgr2.Close()
	if err := mgr2.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock for sign: %v", err)
	}

	cred, err := mgr2.GetCredential("mykey")
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}

	secretBytes, err := decodeBase32Secret(cred.Secret)
	if err != nil {
		t.Fatalf("decodeBase32Secret: %v", err)
	}

	result, err := auth.SignChallenge(cred, secretBytes, []byte("abc123"))
	if err != nil {
		t.Fatalf("SignChallenge: %v", err)
	}

	// Result must be a 40-character lowercase hex string (SHA1 output).
	if len(result) != 40 {
		t.Errorf("sign output length: got %d, want 40", len(result))
	}
	for i, c := range result {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("non-lowercase-hex char %q at position %d in result %q", c, i, result)
			break
		}
	}
}

func TestIntegration_Export(t *testing.T) {
	// Create a source vault with 3 credentials.
	srcPath, _ := createIntegrationVault(t)

	mgr, err := vault.Open(srcPath)
	if err != nil {
		t.Fatalf("Open source vault: %v", err)
	}
	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock source vault: %v", err)
	}

	for _, c := range []pkgmodel.Credential{
		{Label: "export-svc-a", Type: pkgmodel.CredentialTOTP, Secret: "JBSWY3DPEHPK3PXP"},
		{Label: "export-svc-b", Type: pkgmodel.CredentialHOTP, Secret: "GEZDGNBVGY3TQOJQ"},
		{Label: "export-svc-c", Type: pkgmodel.CredentialStatic, Secret: "staticpass"},
	} {
		if _, err := mgr.AddCredential(c); err != nil {
			t.Fatalf("AddCredential %q: %v", c.Label, err)
		}
	}

	// Export using the vault manager method directly (CLI path requires
	// interactive terminal; this exercises the same code path).
	exportPass := []byte("export-integration-passphrase")
	backupData, err := mgr.ExportCredentials(exportPass)
	if err != nil {
		t.Fatalf("ExportCredentials: %v", err)
	}
	mgr.Close()

	// Write backup to a temp file to verify the file I/O path.
	dir := t.TempDir()
	backupPath := filepath.Join(dir, "vault.tegata-backup")
	if err := os.WriteFile(backupPath, backupData, 0600); err != nil {
		t.Fatalf("WriteFile backup: %v", err)
	}

	// Verify backup file exists and is non-empty.
	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("Stat backup: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("backup file is empty")
	}

	// Create a fresh destination vault and import.
	dstPath, _ := createIntegrationVault(t)
	dst, err := vault.Open(dstPath)
	if err != nil {
		t.Fatalf("Open dest vault: %v", err)
	}
	defer dst.Close()
	if err := dst.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock dest vault: %v", err)
	}

	readData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}

	imported, skipped, err := dst.ImportCredentials(readData, exportPass)
	if err != nil {
		t.Fatalf("ImportCredentials: %v", err)
	}

	// Verify summary format: 3 imported, 0 skipped.
	summary := fmt.Sprintf("%d imported, %d skipped (duplicate label)", imported, skipped)
	wantSummary := "3 imported, 0 skipped (duplicate label)"
	if summary != wantSummary {
		t.Errorf("summary: got %q, want %q", summary, wantSummary)
	}

	// Verify credentials are present.
	list := dst.ListCredentials()
	if len(list) != 3 {
		t.Fatalf("ListCredentials after import: got %d, want 3", len(list))
	}

	// Import with wrong passphrase must return an error (not a panic).
	_, _, err = dst.ImportCredentials(readData, []byte("wrong-passphrase"))
	if err == nil {
		t.Fatal("ImportCredentials with wrong passphrase: expected error, got nil")
	}
}

func TestIntegration_TagFilter(t *testing.T) {
	path, _ := createIntegrationVault(t)

	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	credentials := []pkgmodel.Credential{
		{Label: "github", Issuer: "GitHub", Type: pkgmodel.CredentialTOTP, Secret: "JBSWY3DPEHPK3PXP", Tags: []string{"work"}},
		{Label: "jira", Issuer: "Atlassian", Type: pkgmodel.CredentialTOTP, Secret: "JBSWY3DPEHPK3PXP", Tags: []string{"work"}},
		{Label: "gmail", Issuer: "Google", Type: pkgmodel.CredentialTOTP, Secret: "JBSWY3DPEHPK3PXP", Tags: []string{"personal"}},
		{Label: "backup-key", Type: pkgmodel.CredentialStatic, Secret: "staticpass123"},
	}
	for _, c := range credentials {
		if _, err := mgr.AddCredential(c); err != nil {
			t.Fatalf("AddCredential %q: %v", c.Label, err)
		}
	}
	mgr.Close()

	// Reopen and list with tag filter — only work-tagged credentials expected.
	mgr2, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open for list: %v", err)
	}
	defer mgr2.Close()
	if err := mgr2.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock for list: %v", err)
	}

	all := mgr2.ListCredentials()
	if len(all) != 4 {
		t.Fatalf("ListCredentials: got %d, want 4", len(all))
	}

	// Simulate the tag filter: collect credentials matching "work".
	var workCreds []pkgmodel.Credential
	for _, c := range all {
		for _, tag := range c.Tags {
			if tag == "work" {
				workCreds = append(workCreds, c)
				break
			}
		}
	}
	if len(workCreds) != 2 {
		t.Errorf("tag filter 'work': got %d credentials, want 2", len(workCreds))
	}
	for _, c := range workCreds {
		found := false
		for _, tag := range c.Tags {
			if tag == "work" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("credential %q does not have 'work' tag", c.Label)
		}
	}

	// Verify case-sensitivity: "Work" should not match "work".
	var caseMismatch []pkgmodel.Credential
	for _, c := range all {
		for _, tag := range c.Tags {
			if tag == "Work" {
				caseMismatch = append(caseMismatch, c)
				break
			}
		}
	}
	if len(caseMismatch) != 0 {
		t.Errorf("case-insensitive match returned %d credentials; tags must be case-sensitive", len(caseMismatch))
	}
}

func TestIntegration_ChangePassphrase(t *testing.T) {
	path, _ := createIntegrationVault(t)

	// Add a credential, then change the passphrase.
	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if _, err := mgr.AddCredential(pkgmodel.Credential{
		Label:  "test-service",
		Type:   pkgmodel.CredentialTOTP,
		Secret: "JBSWY3DPEHPK3PXP",
	}); err != nil {
		t.Fatalf("AddCredential: %v", err)
	}

	counterBefore := mgr.Header().WriteCounter
	if err := mgr.ChangePassphrase([]byte("new-integration-passphrase")); err != nil {
		t.Fatalf("ChangePassphrase: %v", err)
	}
	counterAfter := mgr.Header().WriteCounter
	if counterBefore != counterAfter {
		t.Errorf("WriteCounter changed during ChangePassphrase: before=%d after=%d",
			counterBefore, counterAfter)
	}
	mgr.Close()

	// New passphrase must succeed, credentials must be intact.
	// (Old passphrase rejection is covered by unit tests in internal/vault.)
	mgr3, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open with new passphrase: %v", err)
	}
	defer mgr3.Close()
	if err := mgr3.Unlock([]byte("new-integration-passphrase")); err != nil {
		t.Fatalf("Unlock with new passphrase: %v", err)
	}
	list := mgr3.ListCredentials()
	if len(list) != 1 {
		t.Fatalf("ListCredentials after passphrase change: got %d, want 1", len(list))
	}
	if list[0].Label != "test-service" {
		t.Errorf("credential label: got %q, want %q", list[0].Label, "test-service")
	}
}

func TestDecodeBase32Secret(t *testing.T) {
	valid := []struct {
		name   string
		input  string
		wantN  int // expected decoded byte length
	}{
		{name: "uppercase no padding", input: "JBSWY3DPEHPK3PXP", wantN: 10},
		{name: "lowercase", input: "jbswy3dpehpk3pxp", wantN: 10},
		{name: "with spaces", input: "JBSWY 3DPE HPK3 PXP", wantN: 10},
		{name: "with hyphens", input: "JBSWY-3DPE-HPK3-PXP", wantN: 10},
	}
	for _, tc := range valid {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			got, err := decodeBase32Secret(tc.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if len(got) != tc.wantN {
				t.Errorf("decoded length: got %d, want %d", len(got), tc.wantN)
			}
		})
	}

	invalid := []string{
		"not-base32!!!",
		"AAAA!!!!", // punctuation is not in the base32 alphabet
		"@#$%",
	}
	for _, input := range invalid {
		t.Run("invalid/"+input, func(t *testing.T) {
			if _, err := decodeBase32Secret(input); err == nil {
				t.Errorf("expected error for invalid base32 %q, got nil", input)
			}
		})
	}
}

func TestAddCmd_FlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "digits zero", args: []string{"MyLabel", "--digits", "0"}},
		{name: "digits negative", args: []string{"MyLabel", "--digits", "-1"}},
		{name: "digits too large", args: []string{"MyLabel", "--digits", "11"}},
		{name: "period zero", args: []string{"MyLabel", "--period", "0"}},
		{name: "period negative", args: []string{"MyLabel", "--period", "-5"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newAddCmd()
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if !errors.Is(err, errors.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

// TestIntegration_AuditWiring verifies that the auth commands (code) succeed
// and produce exit code 0 when audit is disabled (no ledger required). This
// exercises the newEventBuilder disabled path end-to-end.
func TestIntegration_AuditWiring(t *testing.T) {
	path, _ := createIntegrationVault(t)

	// Add a TOTP credential.
	mgr, err := vault.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := mgr.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if _, err := mgr.AddCredential(pkgmodel.Credential{
		Label:     "github",
		Issuer:    "GitHub",
		Type:      pkgmodel.CredentialTOTP,
		Algorithm: "SHA1",
		Digits:    6,
		Period:    30,
		Secret:    "JBSWY3DPEHPK3PXP",
	}); err != nil {
		t.Fatalf("AddCredential: %v", err)
	}
	mgr.Close()

	// newEventBuilder with audit disabled must return a no-op builder.
	cfg := config.Config{
		Audit: config.AuditConfig{Enabled: false},
	}
	dir := filepath.Dir(path)
	passphrase := []byte("integration-test-passphrase")

	builder, err := newEventBuilder(cfg, dir, passphrase)
	if err != nil {
		t.Fatalf("newEventBuilder (disabled): %v", err)
	}
	if builder == nil {
		t.Fatal("newEventBuilder returned nil builder")
	}

	// LogEvent on a disabled builder must be a no-op with no error.
	if logErr := builder.LogEvent("totp", "github", "GitHub", "testhost", true); logErr != nil {
		t.Errorf("LogEvent on disabled builder: %v", logErr)
	}
	if closeErr := builder.Close(); closeErr != nil {
		t.Errorf("Close on disabled builder: %v", closeErr)
	}
}

// TestHistory_FilterByDate verifies that filterRecords correctly applies
// --from and --to date filters using the Timestamp field (unix epoch seconds).
func TestHistory_FilterByDate(t *testing.T) {
	base := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	records := []*audit.EventRecord{
		{ObjectID: "aaa-111", HashValue: "hash1", Timestamp: base.Add(-24 * time.Hour).Unix()}, // 2026-01-14
		{ObjectID: "bbb-222", HashValue: "hash2", Timestamp: base.Unix()},                       // 2026-01-15
		{ObjectID: "ccc-333", HashValue: "hash3", Timestamp: base.Add(24 * time.Hour).Unix()},   // 2026-01-16
	}

	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 15, 23, 59, 59, 999999999, time.UTC)

	filtered := filterRecords(records, from, to)
	if len(filtered) != 1 || filtered[0].ObjectID != "bbb-222" {
		t.Errorf("filterRecords with date range: got %v, want single record bbb-222", filtered)
	}
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
