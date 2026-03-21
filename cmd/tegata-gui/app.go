package main

import (
	"context"
	"encoding/base32"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		return path + string(os.PathSeparator) + "vault.tegata"
	}
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path + "vault.tegata"
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
	defer zeroBytes(passBytes)

	path = cleanVaultPath(path)
	mgr, err := vault.Open(path)
	if err != nil {
		return fmt.Errorf("opening vault: %w", err)
	}

	if err := mgr.Unlock(passBytes); err != nil {
		mgr.Close()
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

	// Initialize clipboard manager.
	a.clipboard = clipboard.NewManager()

	// Start idle timer.
	a.startIdleTimer()

	return nil
}

// LockVault locks the vault by closing the manager and zeroing sensitive memory.
// It emits a "vault:locked" event to the frontend.
func (a *App) LockVault() {
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

	return auth.SignChallenge(cred, secret, []byte(challenge))
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
		return "", nil // User cancelled
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

	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}
