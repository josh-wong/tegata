package main

import (
	"context"
	"encoding/base32"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
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
	ctx       context.Context
	vault     *vault.Manager
	config    config.Config
	clipboard *clipboard.Manager
	vaultPath string
	idleTimer *IdleTimer
	locked    bool
	builder   *audit.EventBuilder // nil when audit disabled or vault locked
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

// shutdown is called by Wails when the application is closing. It locks the
// vault to zero sensitive memory.
func (a *App) shutdown(_ context.Context) {
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

	// Load config from vault directory.
	cfg, err := config.Load(vaultDir(path))
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.config = cfg

	// Ensure the audit stack is running before building the EventBuilder so
	// that the first credential operation after unlock records events
	// immediately. EnsureStack is a no-op when DockerComposePath is empty or
	// auto_start is false, and returns immediately when the ledger is already
	// reachable. Failure is non-fatal — vault unlock succeeds regardless.
	auditProgress := func(msg string) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "audit:unlock-progress", msg)
		}
	}
	if ensureErr := audit.EnsureStack(a.config.Audit, auditProgress); ensureErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit auto-start: %v\n", ensureErr)
	}

	// Build EventBuilder while passphrase is available (AUDT-02).
	builder, builderErr := a.buildEventBuilder(cfg, path, passBytes)
	if builderErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit unavailable: %v\n", builderErr)
	}
	a.builder = builder

	// Zero passphrase AFTER builder construction.
	zeroBytes(passBytes)

	// Initialize clipboard manager.
	a.clipboard = clipboard.NewManager()

	// Start idle timer.
	a.startIdleTimer()

	return nil
}

// LockVault locks the vault by closing the manager and zeroing sensitive memory.
// It emits a "vault:locked" event to the frontend.
func (a *App) LockVault() {
	if a.builder != nil {
		_ = a.builder.Close()
		a.builder = nil
	}
	if a.idleTimer != nil {
		a.idleTimer.Stop()
	}
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
func (a *App) AddCredential(label, issuer, credType, secret, algorithm string, digits, period int, tags []string) (string, error) {
	if a.vault == nil {
		return "", fmt.Errorf("vault is locked")
	}
	a.resetIdle()

	if model.CredentialType(credType) == model.CredentialTOTP && (period < 15 || period > 120) {
		return "", fmt.Errorf("period must be between 15 and 120 seconds")
	}

	cred := model.Credential{
		Label:     label,
		Issuer:    issuer,
		Type:      model.CredentialType(credType),
		Secret:    secret,
		Algorithm: algorithm,
		Digits:    digits,
		Period:    period,
		Tags:      tags,
	}

	return a.vault.AddCredential(cred)
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

	return a.vault.AddCredential(*cred)
}

// RemoveCredential removes a credential by ID from the vault.
func (a *App) RemoveCredential(id string) error {
	if a.vault == nil {
		return fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	if err := a.vault.RemoveCredential(id); err != nil {
		return fmt.Errorf("removing credential: %w", err)
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
	if a.builder != nil {
		_ = a.builder.LogEvent("totp", cred.Label, cred.Issuer, audit.Hostname(), true)
	}
	return &TOTPResult{Code: code, Remaining: remaining}, nil
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

// SetIdleTimeoutSeconds updates the idle lock timeout and restarts the timer.
// A value of 0 disables auto-lock.
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
	return nil
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

// buildEventBuilder constructs an EventBuilder from config and the vault
// passphrase. Returns a disabled builder (no-op) when cfg.Audit.Enabled is
// false. Replicates the logic from cmd/tegata/helpers.go since this is a
// separate package.
func (a *App) buildEventBuilder(cfg config.Config, vaultPath string, passphrase []byte) (*audit.EventBuilder, error) {
	if !cfg.Audit.Enabled {
		return audit.NewEventBuilder(nil, "", nil, 0)
	}

	dir := vaultDir(vaultPath)
	queuePath := filepath.Join(dir, "queue.tegata")

	// Read Argon2id salt from existing queue file header, or generate new.
	var queueSalt []byte
	if data, err := os.ReadFile(queuePath); err == nil && len(data) >= 32 {
		queueSalt = make([]byte, 32)
		copy(queueSalt, data[:32])
	} else {
		var genErr error
		queueSalt, genErr = crypto.GenerateSalt()
		if genErr != nil {
			return nil, fmt.Errorf("generating queue salt: %w", genErr)
		}
	}

	// Derive 32-byte queue encryption key using Argon2id.
	keyBuf := crypto.DeriveKey(passphrase, queueSalt, crypto.DefaultParams)
	defer keyBuf.Destroy()

	queueKey := make([]byte, 32)
	copy(queueKey, keyBuf.Bytes())
	// Note: queueKey is NOT zeroed here — EventBuilder owns it for the
	// lifetime of the session and will use it for queue Save operations.

	client, err := audit.NewClientFromConfig(cfg.Audit)
	if err != nil {
		zeroBytes(queueKey)
		_, _ = fmt.Fprintf(os.Stderr, "tegata-gui: audit ledger unavailable (%v); events will be queued\n", err)
		return audit.NewEventBuilder(nil, "", nil, 0)
	}

	return audit.NewEventBuilder(client, queuePath, queueKey, cfg.Audit.QueueMaxEvents)
}

// AuditHistoryRecord is the JSON-serializable shape returned by GetAuditHistory.
type AuditHistoryRecord struct {
	ObjectID  string `json:"object_id"`
	Operation string `json:"operation"`
	LabelHash string `json:"label_hash"`
	Timestamp int64  `json:"timestamp"`
	HashValue string `json:"hash_value"`
}

// AuditVerifyResult is the JSON-serializable shape returned by VerifyAuditLog.
type AuditVerifyResult struct {
	Valid       bool   `json:"valid"`
	EventCount  int    `json:"event_count"`
	ErrorDetail string `json:"error_detail,omitempty"`
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

	records := make([]AuditHistoryRecord, len(result.Records))
	for i, r := range result.Records {
		records[i] = AuditHistoryRecord{
			ObjectID:  r.ObjectID,
			Operation: r.Operation,
			LabelHash: r.LabelHash,
			Timestamp: r.Timestamp,
			HashValue: r.HashValue,
		}
	}
	return records, nil
}

// VerifyAuditLog verifies the integrity of the audit log by validating each
// event individually via the per-entity collection.
func (a *App) VerifyAuditLog() (*AuditVerifyResult, error) {
	if a.vault == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	client, err := a.newAuditClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := audit.VerifyAll(ctx, client, a.config.Audit.EntityID)
	if err != nil {
		return nil, err
	}

	return &AuditVerifyResult{
		Valid:       result.Valid,
		EventCount:  result.EventCount,
		ErrorDetail: result.ErrorDetail,
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
	return audit.StartStack(a.config.Audit.DockerComposePath)
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
	composeDir := filepath.Join(u.HomeDir, ".tegata", "docker")

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
	onRegistered := func(auditCfg config.AuditConfig) error {
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

// StopAuditServer handles audit server operations.
// When wipe is true, truncates the ScalarDL ledger database (including stored
// entity credentials), restarts the ledger container, re-registers the entity
// secret, and resets the EventBuilder so audit logging resumes immediately.
// When wipe is false, runs docker compose stop (preserves named volume).
func (a *App) StopAuditServer(wipe bool) error {
	a.resetIdle()

	if a.config.Audit.DockerComposePath == "" {
		return fmt.Errorf("audit Docker setup not found. Run StartAuditServer first")
	}

	if wipe {
		if err := audit.WipeHistory(a.config.Audit.DockerComposePath); err != nil {
			return err
		}

		// Delete the offline queue file. WipeHistory clears the ledger but not
		// the local queue cache. Stale queue entries would fail to decrypt on
		// the next vault unlock, disabling audit (D-26). Deleting the queue file
		// forces a clean slate — future entries queue normally if the ledger is
		// not immediately ready.
		dir := vaultDir(a.vaultPath)
		queuePath := filepath.Join(dir, "queue.tegata")
		_ = os.Remove(queuePath)

		cfg := a.config.Audit

		// WipeHistory truncates the ledger database — including stored entity
		// credentials — and restarts the ScalarDL ledger container. Wait for
		// the ledger to become ready (up to 30s), then re-register the entity
		// secret so audit logging resumes immediately after the wipe.
		client, err := audit.NewClientFromConfig(cfg)
		if err != nil {
			return nil // audit unavailable; not fatal
		}

		// Wait for the privileged service to be ready, then re-register the
		// entity secret. RegisterSecret may return AlreadyExists immediately
		// (entity credentials are outside the truncated asset table), so this
		// loop mostly guards against the privileged port not yet accepting calls.
		var regErr error
		for i := 0; i < 15; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			regErr = client.RegisterSecret(ctx, cfg.EntityID, cfg.KeyVersion, cfg.SecretKey)
			cancel()
			if regErr == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if regErr != nil {
			_ = client.Close()
			return fmt.Errorf("re-registering entity after wipe: %w", regErr)
		}

		// RegisterSecret only confirms the privileged port is ready. The regular
		// ledger service (used for contract execution) may still be starting.
		// Retry Ping until it succeeds so that the first Submit after a wipe
		// does not silently time out and queue the event in the memory-only queue.
		var pingErr error
		for i := 0; i < 15; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			pingErr = client.Ping(ctx)
			cancel()
			if pingErr == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if pingErr != nil {
			_ = client.Close()
			return fmt.Errorf("ledger did not become ready after wipe: %w", pingErr)
		}

		// Verify contract execution is available, not just the gRPC transport.
		// The health check endpoint (Ping) responds before ScalarDL's execution
		// engine finishes initialising. A Put probe confirms contracts are
		// callable before the EventBuilder is created. Collection operations
		// (CollectionCreate/Add) are handled by Submit's own retry path.
		// 10 retries × 2 s = 20 s (contracts are already registered in the
		// database — only JVM startup time is needed after a restart).
		var contractErr error
		for i := 0; i < 10; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			contractErr = client.Put(ctx, audit.SetupTestObjectID, strings.Repeat("0", 64))
			cancel()
			if contractErr == nil {
				break
			}
			if i < 9 {
				time.Sleep(2 * time.Second)
			}
		}
		if contractErr != nil {
			_ = client.Close()
			return fmt.Errorf("ledger contracts not ready after wipe: %w", contractErr)
		}

		// Reset the EventBuilder so the hash chain restarts from a clean slate.
		if newBuilder, buildErr := audit.NewEventBuilderMemQueue(client); buildErr == nil {
			if a.builder != nil {
				_ = a.builder.Close()
			}
			a.builder = newBuilder
		} else {
			_ = client.Close()
		}
		return nil
	}

	return audit.StopStack(a.config.Audit.DockerComposePath, false)
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
