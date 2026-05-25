package main

import (
	"context"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/josh-wong/tegata/pkg/model"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// TOTPResult holds a TOTP code and seconds remaining until expiry.
type TOTPResult struct {
	Code      string `json:"code"`
	Remaining int    `json:"remaining"`
}

// VaultLocation describes a discovered vault file on a mounted drive.
type VaultLocation struct {
	Path      string `json:"path"`
	DriveName string `json:"driveName"`
}

// UpdateInfo describes an available update from GitHub Releases.
type UpdateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Notes   string `json:"notes"`
}

// App is the thin adapter struct that wraps internal packages for Wails binding.
// Every exported method becomes a JavaScript-callable function via the Wails
// runtime. The struct holds references to the vault manager, config, clipboard,
// and idle timer, delegating all business logic to the internal packages.
type App struct {
	ctx          context.Context
	vault        *vault.Manager
	config       config.Config
	clipboard    *clipboard.Manager
	vaultPath    string
	vaultModTime time.Time     // mtime of vault file at last read; used to detect external writes
	watcherStop  chan struct{} // closed to stop the vault file watcher goroutine
	idleTimer    *IdleTimer
	locked       bool
	builder      *audit.EventBuilder // nil when audit disabled or vault locked
}

// NewApp creates a new App instance with default configuration.
func NewApp() *App {
	return &App{
		config: config.DefaultConfig(),
	}
}

// startup is called by Wails when the application starts. It saves the runtime
// context and loads configuration.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called by Wails when the application is closing. It stops the
// Docker audit stack (if configured) and locks the vault to zero sensitive memory.
func (a *App) shutdown(_ context.Context) {
	a.deleteClientProperties()
	if a.config.Audit.DockerComposePath != "" {
		_ = audit.StopStack(a.config.Audit.DockerComposePath, a.config.Audit.DockerProjectName)
		// Lock the encrypted ledger volume after stopping the stack.
		_ = audit.LockLedgerVolume(a.config.Audit)
		// Zero the volume key from memory.
		zeroBytes(a.config.Audit.LedgerVolumeKey)
	}
	a.LockVault()
}

// ScanForVaults returns paths to vault.tegata files on mounted drives. It checks
// the TEGATA_VAULT environment variable first (matching CLI resolution order),
// then scans mounted drives for vault files.
func (a *App) ScanForVaults() ([]VaultLocation, error) {
	var results []VaultLocation

	// Check TEGATA_VAULT env var first, matching CLI resolution order.
	if envVal := os.Getenv("TEGATA_VAULT"); envVal != "" {
		path := resolveEnvVaultPath(envVal)
		if _, err := os.Stat(path); err == nil {
			results = append(results, VaultLocation{
				Path:      path,
				DriveName: "TEGATA_VAULT",
			})
		}
	}

	// Scan mounted drives for vault files.
	driveResults := scanMountedDrives()
	results = append(results, driveResults...)

	return results, nil
}

// ScanRemovableDrives returns mounted removable/USB drives for vault creation.
// Unlike ScanForVaults, this returns drives even if they don't contain vaults.
func (a *App) ScanRemovableDrives() ([]VaultLocation, error) {
	return scanRemovableDrives(), nil
}

// IsRemovablePath reports whether the given path resides on a removable drive
// (USB, microSD, etc.). Used by the GUI to warn users about local vault storage.
func (a *App) IsRemovablePath(path string) bool {
	return isRemovablePath(path)
}

// resolveEnvVaultPath resolves a TEGATA_VAULT environment variable value to a
// vault file path. If the path is a directory, it appends "vault.tegata".
func resolveEnvVaultPath(path string) string {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return filepath.Join(path, "vault.tegata")
	}
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return filepath.Join(path, "vault.tegata")
	}
	return path
}

// CreateVault initializes a new encrypted vault at the given path. The path
// must be a full file path ending in .tegata. Returns the hex-encoded recovery
// key display string. Returns an error if a file already exists at the path.
func (a *App) CreateVault(path, passphrase string) (string, error) {
	passBytes := []byte(passphrase)
	defer zeroBytes(passBytes)

	if len(passBytes) < 8 {
		return "", fmt.Errorf("passphrase must be at least 8 characters")
	}

	path = cleanVaultPath(path)

	// Check if file already exists.
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("a vault already exists at %s", path)
	}

	// Create parent directory if it doesn't exist.
	dir := vaultDir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	recoveryDisplay, err := vault.Create(path, passBytes, crypto.DefaultParams)
	if err != nil {
		return "", fmt.Errorf("creating vault: %w", err)
	}

	return recoveryDisplay, nil
}

// cleanVaultPath strips surrounding quotes and whitespace from a vault path.
// Windows "Copy as path" adds quotes that cause silent failures.
func cleanVaultPath(path string) string {
	path = strings.Trim(path, "\"'")
	path = strings.TrimSpace(path)
	return path
}

// UnlockVault opens and decrypts the vault at the given path with the
// passphrase. It loads configuration from the vault directory and starts
// the idle timer.
func (a *App) UnlockVault(path, passphrase string) error {
	passBytes := []byte(passphrase)

	path = cleanVaultPath(path)
	mgr, err := vault.Open(path)
	if err != nil {
		zeroBytes(passBytes)
		return fmt.Errorf("opening vault: %w", err)
	}

	if err := mgr.Unlock(passBytes); err != nil {
		mgr.Close()
		zeroBytes(passBytes)
		return fmt.Errorf("unlocking vault: %w", err)
	}

	a.vault = mgr
	a.vaultPath = path
	a.locked = false

	// Capture the vault file mtime so ListCredentials can detect external writes.
	if fi, err := os.Stat(path); err == nil {
		a.vaultModTime = fi.ModTime()
	}

	// Load config from vault directory.
	cfg, err := config.Load(vaultDir(path))
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.config = cfg

	// Attempt to load HMAC secret from vault (encrypted storage).
	secretFromVault := a.vault.GetSecret("audit.secret_key")

	// Migration: if the vault doesn't have the secret but tegata.toml does,
	// store it in the vault now. Only rewrite the TOML if vault migration succeeds
	// to maintain consistency: either both succeed or both fail.
	if secretFromVault == "" && a.config.Audit.SecretKey != "" {
		if vaultErr := a.vault.SetSecret("audit.secret_key", a.config.Audit.SecretKey); vaultErr != nil {
			a.config.Audit.SecretKey = ""
			return fmt.Errorf("migrating secret to vault: %w", vaultErr)
		}
		secretFromVault = a.config.Audit.SecretKey

		// Cleanup: rewrite tegata.toml to remove the plaintext secret_key field
		// now that it has been safely stored in the vault.
		if writeErr := config.WriteAuditSection(vaultDir(path), a.config.Audit); writeErr != nil {
			a.config.Audit.SecretKey = ""
			return fmt.Errorf("rewriting audit config to remove secret_key: %w", writeErr)
		}

		// Only zero the in-memory secret after both vault and TOML writes succeed.
		a.config.Audit.SecretKey = ""
	}

	// Use the secret (from vault or after migration).
	if secretFromVault != "" {
		a.config.Audit.SecretKey = secretFromVault
	}

	// Load the ledger volume key from vault (encrypted storage) so the
	// encrypted data directory can be decrypted and mounted for postgres.
	volumeKeyHex := a.vault.GetSecret("audit.ledger_volume_key")
	if volumeKeyHex != "" {
		if volumeKey, hexErr := hex.DecodeString(volumeKeyHex); hexErr == nil {
			a.config.Audit.LedgerVolumeKey = volumeKey
		}
	}

	// Ensure the audit stack is running before building the EventBuilder so
	// that the first credential operation after unlock records events
	// immediately. EnsureStack is a no-op when DockerComposePath is empty or
	// auto_start is false, and returns immediately when the ledger is already
	// reachable. Failure is non-fatal — vault unlock succeeds regardless.
	// Passes bundleFS so docker-compose.yml is synced on each unlock, keeping
	// the live stack config current after binary upgrades.
	auditProgress := func(msg string) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "audit:unlock-progress", msg)
		}
	}
	bundleFS, _ := fs.Sub(dockerBundle, "docker-bundle")
	if ensureErr := audit.EnsureStack(a.config.Audit, bundleFS, auditProgress); ensureErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit auto-start: %v\n", ensureErr)
	}

	// Build EventBuilder while passphrase is available (AUDT-02).
	// Use a.config (not cfg) so the SecretKey loaded from the vault is included.
	builder, builderErr := a.buildEventBuilder(a.config, path, passBytes)
	if builderErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit unavailable: %v\n", builderErr)
	}
	a.builder = builder

	// Wire OnHashStored so each submitted audit event's hash is persisted to
	// the vault for independent verification (D-15).
	if a.builder != nil {
		a.builder.OnHashStored = func(eventID, hashValue string) {
			if err := a.vault.SetAuditHash(eventID, hashValue); err != nil {
				slog.Error("failed to store audit hash in vault", "err", err)
			}
		}
	}

	// Emit vault-unlock after builder is wired up and passphrase is still available.
	if a.builder != nil {
		if logErr := a.builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit log failed: %v\n", logErr)
		}
	}

	// Zero passphrase AFTER builder construction.
	zeroBytes(passBytes)

	// Initialize clipboard manager.
	a.clipboard = clipboard.NewManager()

	// Start idle timer and vault file watcher.
	a.startIdleTimer()
	a.startVaultWatcher()

	return nil
}

// LockVault locks the vault by closing the manager and zeroing sensitive memory.
// It emits a "vault:locked" event to the frontend.
func (a *App) LockVault() {
	if a.builder != nil {
		if logErr := a.builder.LogEvent("vault-lock", "", "", audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit log failed: %v\n", logErr)
		}
		_ = a.builder.Close()
		a.builder = nil
	}
	if a.idleTimer != nil {
		a.idleTimer.Stop()
	}
	a.stopVaultWatcher()

	// Delete plaintext client.properties file when vault is locked.
	// This ensures the HMAC secret is not accessible on disk after the app exits.
	a.deleteClientProperties()

	// Zero the HMAC secret key from memory before closing the vault.
	a.config.Audit.SecretKey = ""

	if a.vault != nil {
		a.vault.Close()
		a.vault = nil
	}
	if a.clipboard != nil {
		a.clipboard.Close()
		a.clipboard = nil
	}
	a.locked = true

	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "vault:locked")
	}
}

// deleteClientProperties removes the plaintext client.properties file created
// by the audit setup. This ensures the HMAC secret key is not persisted to
// disk after the app closes or the vault is locked.
func (a *App) deleteClientProperties() {
	// Try to delete from the configured Docker compose location if available.
	// DockerComposePath is a file path (e.g., .../docker/docker-compose.yml),
	// so use its parent directory to locate the certs/ subdirectory.
	if a.config.Audit.DockerComposePath != "" {
		composeDir := filepath.Dir(a.config.Audit.DockerComposePath)
		clientPropsPath := filepath.Join(composeDir, "certs", "client.properties")
		if err := os.Remove(clientPropsPath); err != nil && !os.IsNotExist(err) {
			_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: warning: could not delete client.properties at %s: %v\n", clientPropsPath, err)
		}
		return
	}

	// Fallback: scan all per-vault compose directories for client.properties.
	// This handles shutdown before any vault was unlocked (DockerComposePath empty).
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	matches, _ := filepath.Glob(filepath.Join(homeDir, ".tegata", "docker", "*", "certs", "client.properties")) // Glob only errors on malformed patterns, not missing paths
	for _, p := range matches {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: warning: could not delete client.properties at %s: %v\n", p, err)
		}
	}
}

// ListCredentials returns all credentials in the vault.
func (a *App) ListCredentials() ([]model.Credential, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	return a.vault.ListCredentials(), nil
}

// GetCredential returns the credential matching the given label.
func (a *App) GetCredential(label string) (*model.Credential, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return nil, fmt.Errorf("getting credential: %w", err)
	}
	return cred, nil
}

// AddCredential creates a new credential in the vault and returns its ID.
func (a *App) AddCredential(label, issuer, credType, secret, algorithm string, digits, period int, tags []string, category string) (string, error) {
	if a.vault == nil {
		return "", fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	ctype := model.CredentialType(credType)
	algorithm = strings.ToUpper(strings.TrimSpace(algorithm))
	if ctype == model.CredentialHOTP {
		if algorithm == "" {
			algorithm = "SHA1"
		}
		if algorithm != "SHA1" {
			return "", fmt.Errorf("HOTP algorithm must be SHA1 per RFC 4226")
		}
	}
	if ctype == model.CredentialTOTP && algorithm == "" {
		algorithm = "SHA1"
	}

	if ctype == model.CredentialTOTP && (period < 15 || period > 120) {
		return "", fmt.Errorf("period must be between 15 and 120 seconds")
	}

	cred := model.Credential{
		Label:     label,
		Issuer:    issuer,
		Type:      ctype,
		Secret:    secret,
		Algorithm: algorithm,
		Digits:    digits,
		Period:    period,
		Tags:      tags,
		Category:  strings.ToLower(strings.TrimSpace(category)),
	}

	id, err := a.vault.AddCredential(cred)
	if err != nil {
		return "", err
	}
	if a.builder != nil {
		if logErr := a.builder.LogEvent("credential-add", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit log failed: %v\n", logErr)
		}
	}
	return id, nil
}

// AddCredentialFromURI parses an otpauth:// URI and adds the credential to the
// vault. Returns the assigned credential ID.
func (a *App) AddCredentialFromURI(uri string) (string, error) {
	if a.vault == nil {
		return "", fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	cred, err := auth.ParseOTPAuthURI(uri)
	if err != nil {
		return "", fmt.Errorf("parsing URI: %w", err)
	}

	id, err := a.vault.AddCredential(*cred)
	if err != nil {
		return "", err
	}
	if a.builder != nil {
		if logErr := a.builder.LogEvent("credential-add", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit log failed: %v\n", logErr)
		}
	}
	return id, nil
}

// RemoveCredential removes a credential by ID from the vault.
func (a *App) RemoveCredential(id string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	// Capture label and issuer before removal so the audit event can reference them.
	var label, issuer string
	if a.builder != nil {
		for _, c := range a.vault.ListCredentials() {
			if c.ID == id {
				label = c.Label
				issuer = c.Issuer
				break
			}
		}
	}

	if err := a.vault.RemoveCredential(id); err != nil {
		return fmt.Errorf("removing credential: %w", err)
	}

	if a.builder != nil {
		if logErr := a.builder.LogEvent("credential-remove", label, issuer, audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit log failed: %v\n", logErr)
		}
	}
	return nil
}

// EditCredential updates a credential's label, issuer, category, and/or tags.
// Label is only updated if non-empty; issuer and category are always updated (can be cleared by passing empty strings);
// tags are only updated if non-nil.
func (a *App) EditCredential(id, label, issuer string, tags []string, category string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	// Find the credential by ID
	creds := a.vault.ListCredentials()
	var cred *model.Credential
	for i := range creds {
		if creds[i].ID == id {
			cred = &creds[i]
			break
		}
	}
	if cred == nil {
		return fmt.Errorf("credential not found")
	}

	// Track original values for per-field audit events.
	origLabel := cred.Label
	origIssuer := cred.Issuer
	origCategory := cred.Category
	origTags := slices.Clone(cred.Tags)

	// Apply updates
	if label != "" {
		label = strings.TrimSpace(label)
		if label == "" {
			return fmt.Errorf("label cannot be empty or whitespace-only")
		}
		// Check for duplicate label
		for _, c := range creds {
			if strings.EqualFold(c.Label, label) && c.ID != id {
				return fmt.Errorf("a credential with label %q already exists", label)
			}
		}
		cred.Label = label
	}

	cred.Issuer = issuer

	if tags != nil {
		// Normalize tags to lowercase and validate no duplicates
		var normalizedTags []string
		seen := make(map[string]struct{})
		for _, t := range tags {
			normalized := strings.ToLower(t)
			if _, exists := seen[normalized]; exists {
				return fmt.Errorf("duplicate tag: %q", t)
			}
			seen[normalized] = struct{}{}
			normalizedTags = append(normalizedTags, normalized)
		}
		cred.Tags = normalizedTags
	}

	// Always overwrite category (empty string clears it).
	cred.Category = strings.ToLower(strings.TrimSpace(category))

	if err := a.vault.UpdateCredential(cred); err != nil {
		return fmt.Errorf("updating credential: %w", err)
	}

	// Log one audit event per changed field.
	if a.builder != nil {
		type fieldEvent struct {
			changed bool
			opType  string
		}
		events := []fieldEvent{
			{cred.Label != origLabel, "credential-label-update"},
			{cred.Issuer != origIssuer, "credential-issuer-update"},
			{cred.Category != origCategory, "credential-category-update"},
			{!slices.Equal(origTags, cred.Tags), "credential-tag-update"},
		}
		for _, fe := range events {
			if fe.changed {
				if logErr := a.builder.LogEvent(fe.opType, cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit log failed: %v\n", logErr)
				}
			}
		}
	}

	return nil
}

// GenerateTOTP generates a TOTP code for the credential with the given label.
func (a *App) GenerateTOTP(label string) (*TOTPResult, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return nil, err
	}

	secret, err := decodeBase32Secret(cred.Secret)
	if err != nil {
		return nil, fmt.Errorf("decoding secret: %w", err)
	}
	defer zeroBytes(secret)

	period := cred.Period
	if period == 0 {
		period = 30
	}
	digits := cred.Digits
	if digits == 0 {
		digits = 6
	}

	code, remaining := auth.GenerateTOTP(secret, time.Now(), period, digits, cred.Algorithm)
	return &TOTPResult{Code: code, Remaining: remaining}, nil
}

// RecordTOTPUsed logs an audit event for a TOTP credential. It is called
// explicitly by the frontend when the user copies a TOTP code, not on every
// automatic code refresh.
func (a *App) RecordTOTPUsed(label string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	if a.builder == nil {
		return nil
	}
	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return err
	}
	return a.builder.LogEvent("totp", cred.Label, cred.Issuer, audit.Hostname(), true)
}

// RecordHOTPUsed logs an audit event when the user copies an HOTP code.
func (a *App) RecordHOTPUsed(label string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	if a.builder == nil {
		return nil
	}
	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return err
	}
	return a.builder.LogEvent("hotp-copy", cred.Label, cred.Issuer, audit.Hostname(), true)
}

// RecordChallengeResponseUsed logs an audit event when the user copies a
// challenge-response signature.
func (a *App) RecordChallengeResponseUsed(label string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	if a.builder == nil {
		return nil
	}
	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return err
	}
	return a.builder.LogEvent("challenge-response-copy", cred.Label, cred.Issuer, audit.Hostname(), true)
}

// GenerateHOTP generates an HOTP code for the credential with the given label.
// It increments the counter and saves the vault.
func (a *App) GenerateHOTP(label string) (string, error) {
	if a.vault == nil {
		return "", fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return "", err
	}

	secret, err := decodeBase32Secret(cred.Secret)
	if err != nil {
		return "", fmt.Errorf("decoding secret: %w", err)
	}
	defer zeroBytes(secret)

	digits := cred.Digits
	if digits == 0 {
		digits = 6
	}

	code := auth.GenerateHOTP(secret, cred.Counter, digits, cred.Algorithm)

	// Increment counter and save (counter-before-code pattern).
	cred.Counter++
	if err := a.vault.UpdateCredential(cred); err != nil {
		return "", fmt.Errorf("updating counter: %w", err)
	}

	if a.builder != nil {
		_ = a.builder.LogEvent("hotp", cred.Label, cred.Issuer, audit.Hostname(), true)
	}
	return code, nil
}

// GetStaticPassword copies the static password for the given credential label
// to the clipboard with auto-clear.
func (a *App) GetStaticPassword(label string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return err
	}

	password, err := auth.GetStaticPassword(cred)
	if err != nil {
		return err
	}
	defer zeroBytes(password)

	if a.builder != nil {
		_ = a.builder.LogEvent("static", cred.Label, cred.Issuer, audit.Hostname(), true)
	}
	if a.clipboard != nil {
		return a.clipboard.CopyWithAutoClear(string(password), a.config.ClipboardTimeout)
	}
	return nil
}

// SignChallenge computes an HMAC over the challenge using the credential's
// secret. Returns the hex-encoded signature.
func (a *App) SignChallenge(label, challenge string) (string, error) {
	if a.vault == nil {
		return "", fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	cred, err := a.vault.GetCredential(label)
	if err != nil {
		return "", err
	}

	// Try base32 first (standard for OTP secrets), fall back to raw bytes
	// for challenge-response since users may enter plain text shared keys.
	secret, err := decodeBase32Secret(cred.Secret)
	if err != nil {
		secret = []byte(cred.Secret)
	}
	defer zeroBytes(secret)

	result, err := auth.SignChallenge(cred, secret, []byte(challenge))
	if err != nil {
		return "", err
	}
	if a.builder != nil {
		_ = a.builder.LogEvent("challenge-response", cred.Label, cred.Issuer, audit.Hostname(), true)
	}
	return result, nil
}

// ExportVault exports all credentials encrypted with the given passphrase.
func (a *App) ExportVault(exportPassphrase string) ([]byte, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	passBytes := []byte(exportPassphrase)
	defer zeroBytes(passBytes)

	return a.vault.ExportCredentials(passBytes)
}

// ExportVaultToFile opens a native save dialog and writes encrypted credentials
// to the selected file. Returns the file path on success.
func (a *App) ExportVaultToFile(exportPassphrase string) (string, error) {
	if a.vault == nil {
		return "", fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	passBytes := []byte(exportPassphrase)
	defer zeroBytes(passBytes)

	data, err := a.vault.ExportCredentials(passBytes)
	if err != nil {
		return "", fmt.Errorf("exporting: %w", err)
	}
	defer zeroBytes(data)

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export Credentials",
		DefaultFilename: "tegata-export.enc",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Encrypted Export (*.enc)", Pattern: "*.enc"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil // User canceled
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return path, nil
}

// ImportVault imports credentials from an encrypted backup. Returns the count
// of imported and skipped credentials.
func (a *App) ImportVault(data []byte, importPassphrase string) (int, int, error) {
	if a.vault == nil {
		return 0, 0, fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	passBytes := []byte(importPassphrase)
	defer zeroBytes(passBytes)

	return a.vault.ImportCredentials(data, passBytes)
}

// ImportResult holds the outcome of a credential import.
type ImportResult struct {
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
	Path     string `json:"path"`
}

// PickImportFile opens a native file dialog and returns the selected path.
func (a *App) PickImportFile() (string, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Import Credentials",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Encrypted Export (*.enc)", Pattern: "*.enc"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("open dialog: %w", err)
	}
	return path, nil
}

// maxImportSize is the maximum allowed size for an import file (10 MB).
const maxImportSize = 10 << 20

// ImportVaultFromFile reads the encrypted file at the given path and imports
// credentials into the vault.
func (a *App) ImportVaultFromFile(path, importPassphrase string) (*ImportResult, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	if info.Size() > maxImportSize {
		return nil, fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), maxImportSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	defer zeroBytes(data)

	passBytes := []byte(importPassphrase)
	defer zeroBytes(passBytes)

	imported, skipped, err := a.vault.ImportCredentials(data, passBytes)
	if err != nil {
		return nil, fmt.Errorf("importing: %w", err)
	}

	return &ImportResult{Imported: imported, Skipped: skipped, Path: path}, nil
}

// ChangePassphrase verifies the current passphrase and changes it to a new one.
func (a *App) ChangePassphrase(current, newPass string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	currentBytes := []byte(current)
	defer zeroBytes(currentBytes)
	newBytes := []byte(newPass)
	defer zeroBytes(newBytes)

	if len(newBytes) < 8 {
		return fmt.Errorf("new passphrase must be at least 8 characters")
	}

	// Verify current passphrase by opening a temporary vault instance.
	verifier, err := vault.Open(a.vaultPath)
	if err != nil {
		return fmt.Errorf("verifying current passphrase: %w", err)
	}
	if err := verifier.Unlock(currentBytes); err != nil {
		verifier.Close()
		return fmt.Errorf("current passphrase is incorrect")
	}
	verifier.Close()

	return a.vault.ChangePassphrase(newBytes)
}

// VerifyRecoveryKey checks whether the provided recovery key matches the vault.
func (a *App) VerifyRecoveryKey(key string) (bool, error) {
	if a.vault == nil {
		return false, fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	// Decode the display-format recovery key (base32 with dashes).
	cleaned := strings.ReplaceAll(key, "-", "")
	recoveryRaw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleaned)
	if err != nil {
		return false, fmt.Errorf("decoding recovery key: %w", err)
	}
	defer zeroBytes(recoveryRaw)

	return a.vault.VerifyRecoveryKey(recoveryRaw)
}

// GetConfig returns the current configuration.
func (a *App) GetConfig() config.Config {
	return a.config
}

// GetVersion returns the application version string set at build time.
func (a *App) GetVersion() string {
	return version
}

// GetIdleTimeoutSeconds returns the current idle lock timeout in seconds.
func (a *App) GetIdleTimeoutSeconds() int {
	return int(a.config.IdleTimeout.Seconds())
}

// SetIdleTimeoutSeconds updates the idle lock timeout, restarts the timer, and
// persists the new value to tegata.toml. A value of 0 disables auto-lock.
func (a *App) SetIdleTimeoutSeconds(seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}
	a.config.IdleTimeout = time.Duration(seconds) * time.Second
	if a.idleTimer != nil {
		a.idleTimer.Stop()
	}
	if seconds > 0 && a.vault != nil {
		a.startIdleTimer()
	}
	if a.vaultPath != "" {
		clipSec := int(a.config.ClipboardTimeout.Seconds())
		if err := config.WriteClipboardVaultSections(vaultDir(a.vaultPath), clipSec, seconds); err != nil {
			return fmt.Errorf("saving idle timeout: %w", err)
		}
	}
	return nil
}

// GetClipboardTimeoutSeconds returns the current clipboard auto-clear timeout
// in seconds. A value of 0 means auto-clear is disabled.
func (a *App) GetClipboardTimeoutSeconds() int {
	return int(a.config.ClipboardTimeout.Seconds())
}

// SetClipboardTimeoutSeconds updates the clipboard auto-clear timeout and
// persists the new value to tegata.toml. A value of 0 disables auto-clear.
func (a *App) SetClipboardTimeoutSeconds(seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}
	a.config.ClipboardTimeout = time.Duration(seconds) * time.Second
	if a.vaultPath != "" {
		idleSec := int(a.config.IdleTimeout.Seconds())
		if err := config.WriteClipboardVaultSections(vaultDir(a.vaultPath), seconds, idleSec); err != nil {
			return fmt.Errorf("saving clipboard timeout: %w", err)
		}
	}
	return nil
}

// CopyToClipboard writes text to the clipboard with auto-clear. All credential
// copy actions in the GUI must route through this method so the configured
// clipboard timeout is respected regardless of credential type.
func (a *App) CopyToClipboard(text string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	if a.clipboard == nil {
		return nil
	}
	return a.clipboard.CopyWithAutoClear(text, a.config.ClipboardTimeout)
}

// ResetIdle resets the backend idle timer. The frontend calls this when user
// activity is detected (e.g., mouse or keyboard events) so the backend timer
// stays in sync with the frontend's idle tracking.
func (a *App) ResetIdle() {
	a.resetIdle()
}

// CheckForUpdate checks the GitHub Releases API for a newer version. Returns
// nil if the current version is up to date or if the check is disabled.
func (a *App) CheckForUpdate() (*UpdateInfo, error) {
	// Stub: update checking will be implemented in a later plan.
	return nil, nil
}

// startIdleTimer initializes and starts the idle timer using the configured
// timeout. The callback locks the vault when idle.
func (a *App) startIdleTimer() {
	if a.idleTimer != nil {
		a.idleTimer.Stop()
	}
	a.idleTimer = NewIdleTimer(a.config.IdleTimeout, func() {
		a.LockVault()
	})
}

// resetIdle resets the idle timer if it is running.
func (a *App) resetIdle() {
	if a.idleTimer != nil {
		a.idleTimer.Reset()
	}
}

// startVaultWatcher launches a background goroutine that polls the vault file
// mtime every 2 seconds. When a change is detected, it reloads the in-memory
// payload and emits "vault:updated" so the frontend can refresh its credential
// list without requiring user interaction. The goroutine exits when the vault
// is locked (stopVaultWatcher closes watcherStop).
func (a *App) startVaultWatcher() {
	a.stopVaultWatcher() // stop any previous watcher first
	stop := make(chan struct{})
	a.watcherStop = stop
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fi, err := os.Stat(a.vaultPath)
				if err != nil {
					continue
				}
				if fi.ModTime().After(a.vaultModTime) {
					if a.vault != nil {
						if reloadErr := a.vault.ReloadPayload(); reloadErr == nil {
							a.vaultModTime = fi.ModTime()
							if a.ctx != nil {
								wailsruntime.EventsEmit(a.ctx, "vault:updated")
							}
						} else {
							slog.Warn("vault watcher reload failed", "err", reloadErr)
						}
					}
				}
			}
		}
	}()
}

// stopVaultWatcher stops the vault file watcher goroutine if one is running.
func (a *App) stopVaultWatcher() {
	if a.watcherStop != nil {
		close(a.watcherStop)
		a.watcherStop = nil
	}
}

// buildEventBuilder constructs an EventBuilder from config and the vault passphrase.
// Returns a disabled builder (no-op) when cfg.Audit.Enabled is false.
func (a *App) buildEventBuilder(cfg config.Config, vaultPath string, passphrase []byte) (*audit.EventBuilder, error) {
	return audit.NewEventBuilderFromConfig(cfg.Audit, filepath.Dir(vaultPath), passphrase)
}

// AuditHistoryRecord is the JSON-serializable shape returned by GetAuditHistory.
type AuditHistoryRecord struct {
	ObjectID  string `json:"object_id"`
	Operation string `json:"operation"`
	Label     string `json:"label"`
	LabelHash string `json:"label_hash"`
	Timestamp int64  `json:"timestamp"`
	HashValue string `json:"hash_value"`
}

// AuditVerifyResult is the JSON-serializable shape returned by VerifyAuditLog.
type AuditVerifyResult struct {
	Valid      bool     `json:"valid"`
	EventCount int      `json:"event_count"`
	Skipped    int      `json:"skipped,omitempty"`
	Faults     []string `json:"faults,omitempty"`
}

// IsAuditEnabled returns whether audit logging is configured and enabled.
func (a *App) IsAuditEnabled() bool {
	return a.config.Audit.Enabled
}

// newAuditClient creates a new LedgerClient from the current config.
func (a *App) newAuditClient() (*audit.LedgerClient, error) {
	return audit.NewClientFromConfig(a.config.Audit)
}

// GetAuditHistory retrieves audit event records from the ScalarDL Ledger.
func (a *App) GetAuditHistory() ([]AuditHistoryRecord, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	if err := audit.CheckLedgerAvailability(a.config.Audit); err != nil {
		return nil, err
	}
	client, err := a.newAuditClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := audit.FetchHistory(ctx, client, a.config.Audit.EntityID)
	if err != nil {
		return nil, err
	}
	if result.Warning != "" {
		_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: %s\n", result.Warning)
	}

	// Build label maps for hash resolution — same approach as TUI/CLI history.
	creds := a.vault.ListCredentials()
	labelNames := make([]string, len(creds))
	for i, c := range creds {
		labelNames[i] = c.Label
	}
	labelMap := audit.BuildLabelMap(labelNames)
	deletedMap := a.vault.DeletedLabels()

	records := make([]AuditHistoryRecord, len(result.Records))
	for i, r := range result.Records {
		records[i] = AuditHistoryRecord{
			ObjectID:  r.ObjectID,
			Operation: audit.FormatOperation(r.Operation),
			Label:     audit.ResolveLabelWithDeleted(r.LabelHash, labelMap, deletedMap),
			LabelHash: r.LabelHash,
			Timestamp: r.Timestamp,
			HashValue: r.HashValue,
		}
	}
	return records, nil
}

// VerifyCredentialAuditLog verifies the integrity of audit events for a single
// credential identified by label. Only events whose label_hash matches the
// SHA-256 hash of label are validated, so a tamper in another credential's
// events will not affect the result for this credential.
func (a *App) VerifyCredentialAuditLog(label string) (*AuditVerifyResult, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	if err := audit.CheckLedgerAvailability(a.config.Audit); err != nil {
		return nil, err
	}
	client, err := a.newAuditClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hashes := a.vault.AuditHashes()
	defer vault.ZeroAuditHashes(hashes)

	labelHash := audit.HashString(label)
	result, err := audit.VerifyByLabelHash(ctx, client, a.config.Audit.EntityID, labelHash, hashes)
	if err != nil {
		return nil, err
	}

	return &AuditVerifyResult{
		Valid:      result.Valid,
		EventCount: result.EventCount,
		Skipped:    result.Skipped,
		Faults:     result.Faults,
	}, nil
}

// VerifyAuditLog verifies the integrity of the audit log by validating each
// event individually via the per-entity collection.
func (a *App) VerifyAuditLog() (*AuditVerifyResult, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	if err := audit.CheckLedgerAvailability(a.config.Audit); err != nil {
		return nil, err
	}
	client, err := a.newAuditClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hashes := a.vault.AuditHashes()
	defer vault.ZeroAuditHashes(hashes)

	result, err := audit.VerifyAll(ctx, client, a.config.Audit.EntityID, hashes)
	if err != nil {
		return nil, err
	}

	return &AuditVerifyResult{
		Valid:      result.Valid,
		EventCount: result.EventCount,
		Skipped:    result.Skipped,
		Faults:     result.Faults,
	}, nil
}

// GetAuditDockerPath returns the docker_compose_path from the audit config.
// Returns empty string when Docker audit setup has not been run.
// Called by AuditPanel on open to determine whether to show the full setup
// flow (empty) or the restart/stop/delete buttons (non-empty).
func (a *App) GetAuditDockerPath() string {
	return a.config.Audit.DockerComposePath
}

// RestartAuditServer starts the Docker containers without re-running the full
// setup sequence. Use this when the stack was previously stopped with
// StopAuditServer(false) and existing credentials should be reused.
func (a *App) RestartAuditServer() error {
	a.resetIdle()
	if a.config.Audit.DockerComposePath == "" {
		return fmt.Errorf("audit Docker setup not found. Run StartAuditServer first")
	}
	return audit.StartStack(a.config.Audit.DockerComposePath, a.config.Audit.DockerProjectName)
}

// StartAuditServer runs the full Docker audit setup sequence. It calls
// audit.SetupStack using the embedded docker bundle and writes the result
// to tegata.toml. Returns a map with "steps" ([]string of status lines)
// on success.
//
// Called from the GUI "Start ledger server" button in AuditPanel.
func (a *App) StartAuditServer() (map[string]interface{}, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	vaultID := a.vault.VaultID()
	dir := filepath.Dir(a.vaultPath)

	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	composeDir := audit.ComposeDirForVault(u.HomeDir, vaultID)

	// Strip the docker-bundle/ prefix from the embed.FS.
	bundleFS, err := fs.Sub(dockerBundle, "docker-bundle")
	if err != nil {
		return nil, fmt.Errorf("accessing embedded docker bundle: %w", err)
	}

	var steps []string
	progress := func(msg string) {
		steps = append(steps, msg)
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "audit:progress", msg)
		}
	}

	// onRegistered is called by SetupStack as soon as the entity secret is
	// registered and the ledger is confirmed reachable. Persisting tegata.toml
	// and initialising the EventBuilder here ensures audit is active even if
	// the contract-registration container is still running in the background.
	// The secret is stored FIRST to ensure transactional consistency: if vault
	// storage fails, the config file is not modified.
	onRegistered := func(auditCfg config.AuditConfig) error {
		// Store the HMAC secret in the encrypted vault instead of tegata.toml.
		// This ensures the secret is never persisted in plaintext on disk.
		// This must happen BEFORE writing the config file to maintain consistency.
		if auditCfg.SecretKey != "" {
			if vaultErr := a.vault.SetSecret("audit.secret_key", auditCfg.SecretKey); vaultErr != nil {
				return fmt.Errorf("storing secret in vault: %w", vaultErr)
			}
		}

		if writeErr := config.WriteAuditSection(dir, auditCfg); writeErr != nil {
			return fmt.Errorf("writing audit config: %w", writeErr)
		}

		// Update in-memory config so auto-start fires on next unlock.
		a.config.Audit = auditCfg

		// Initialise the EventBuilder for this session. The vault passphrase is
		// no longer available (zeroed after vault creation), so use an in-memory
		// queue. Events queue until contracts are ready, then flush on submission.
		client, clientErr := audit.NewClientFromConfig(auditCfg)
		if clientErr == nil {
			if newBuilder, buildErr := audit.NewEventBuilderMemQueue(client); buildErr == nil {
				if a.builder != nil {
					_ = a.builder.Close()
				}
				a.builder = newBuilder
			} else {
				_ = client.Close()
			}
		}
		return nil
	}

	_, err = audit.SetupStack(bundleFS, composeDir, vaultID, progress, onRegistered)
	if err != nil {
		return map[string]interface{}{"steps": steps}, err
	}

	return map[string]interface{}{"steps": steps}, nil
}

// StopAuditServer stops the ScalarDL Ledger Docker containers.
// Audit history is preserved (docker compose stop, named volume retained).
func (a *App) StopAuditServer() error {
	a.resetIdle()

	if a.config.Audit.DockerComposePath == "" {
		return fmt.Errorf("audit Docker setup not found. Run StartAuditServer first")
	}

	return audit.StopStack(a.config.Audit.DockerComposePath, a.config.Audit.DockerProjectName)
}

// IsAuditConfigured returns whether audit logging has been enabled by the user.
func (a *App) IsAuditConfigured() bool {
	return a.config.Audit.Enabled
}

// GetAuditAutoStart returns whether audit auto-start is enabled.
func (a *App) GetAuditAutoStart() bool {
	return a.config.Audit.AutoStart
}

// EnableAudit opts in to audit logging during vault creation.
// Sets both Enabled and AutoStart to true, unlike SetAuditAutoStart which
// only toggles AutoStart for the settings panel.
func (a *App) EnableAudit() error {
	a.config.Audit.Enabled = true
	a.config.Audit.AutoStart = true
	dir := vaultDir(a.vaultPath)
	return config.WriteAuditSection(dir, a.config.Audit)
}

// SetAuditAutoStart persists the audit auto-start setting without wiping other audit fields.
func (a *App) SetAuditAutoStart(enabled bool) error {
	a.config.Audit.AutoStart = enabled
	dir := vaultDir(a.vaultPath)
	return config.WriteAuditSection(dir, a.config.Audit)
}

// vaultDir returns the directory containing the vault file.
func vaultDir(vaultPath string) string {
	return filepath.Dir(vaultPath)
}

// zeroBytes overwrites a byte slice with zeros for memory hygiene.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// decodeBase32Secret decodes a base32-encoded OTP secret, tolerating spaces,
// hyphens, lowercase, missing padding, and common digit lookalikes.
func decodeBase32Secret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.NewReplacer(
		" ", "", "-", "", "=", "",
		"0", "O", "1", "L", "8", "B",
	).Replace(secret))

	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("the secret is not a valid base32 string — TOTP and HOTP secrets use characters A-Z and 2-7 only")
	}
	return b, nil
}
