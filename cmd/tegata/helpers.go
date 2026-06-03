package main

import (
	"bufio"
	"bytes"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const vaultFilename = "vault.tegata"

// resolveVaultPath determines the vault file path using the resolution order:
// 1. --vault flag (directory or file path)
// 2. TEGATA_VAULT env var (directory or file path)
// 3. ./vault.tegata in the current working directory
func resolveVaultPath(cmd *cobra.Command) (string, error) {
	// Check --vault flag.
	if flagVal, _ := cmd.Flags().GetString("vault"); flagVal != "" {
		return resolvePathArg(flagVal)
	}

	// Check TEGATA_VAULT env var.
	if envVal := os.Getenv("TEGATA_VAULT"); envVal != "" {
		return resolvePathArg(envVal)
	}

	// Fall back to current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%s: %w",
			i18n.Tf("helpers.error.getWD", map[string]any{"Err": err}),
			err)
	}
	path := filepath.Join(cwd, vaultFilename)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s: %w",
			errors.UserMessage(
				i18n.T("helpers.error.noVault"),
				i18n.T("helpers.info.noVaultHint"),
			),
			errors.ErrNotFound)
	}
	return path, nil
}

// resolvePathArg resolves a user-provided path argument to an absolute vault
// file path. If the path is a directory, it appends the vault filename. If it
// is a file path, it is used as-is. Relative paths are made absolute using the
// current working directory.
func resolvePathArg(path string) (string, error) {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		path = filepath.Join(path, vaultFilename)
	} else if strings.HasSuffix(path, string(filepath.Separator)) {
		// Looks like a directory path (ends with separator) but Stat failed.
		path = filepath.Join(path, vaultFilename)
	}
	return filepath.Abs(path)
}

// promptPassphrase reads a passphrase using the precedence:
// 1. TEGATA_PASSPHRASE env var (with warning)
// 2. stdin pipe (if not a terminal)
// 3. Interactive prompt (echo disabled)
func promptPassphrase(prompt string) ([]byte, error) {
	// Check env var first.
	if envPass := os.Getenv("TEGATA_PASSPHRASE"); envPass != "" {
		fmt.Fprintln(os.Stderr, i18n.T("helpers.info.envPassphrase"))
		return []byte(envPass), nil
	}

	// Check if stdin is piped.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		reader := bufio.NewReader(os.Stdin)
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("%s", i18n.Tf("helpers.error.readStdin", map[string]any{"Err": err}))
		}
		// Trim trailing newline from piped input.
		return []byte(strings.TrimRight(string(data), "\r\n")), nil
	}

	// Interactive prompt.
	fmt.Fprint(os.Stderr, prompt)
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden input
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.Tf("helpers.error.readPassphrase", map[string]any{"Err": err}))
	}
	return pass, nil
}

// promptNewPassphrase handles passphrase creation for tegata init. It displays
// a tip, prompts for the passphrase with a strength meter, enforces a minimum
// length, and confirms the passphrase.
func promptNewPassphrase() ([]byte, error) {
	fmt.Fprintln(os.Stderr, i18n.T("helpers.tip.passphrase"))

	pass, err := promptPassphrase(i18n.T("cmd.add.prompt.secret"))
	if err != nil {
		return nil, err
	}

	if len(pass) < 8 {
		return nil, fmt.Errorf("%s", i18n.Tf("helpers.error.shortPassphrase", map[string]any{"Err": errors.ErrInvalidInput}))
	}

	// Display strength meter.
	displayStrengthMeter(pass)

	// Confirm passphrase (skip for non-interactive modes).
	if os.Getenv("TEGATA_PASSPHRASE") != "" || !term.IsTerminal(int(os.Stdin.Fd())) {
		return pass, nil
	}

	confirm, err := promptPassphrase(i18n.T("cmd.add.prompt.secret"))
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(pass, confirm) {
		// Zero both copies before returning the error.
		for i := range pass {
			pass[i] = 0
		}
		for i := range confirm {
			confirm[i] = 0
		}
		return nil, fmt.Errorf("%s", i18n.Tf("helpers.error.passMismatch", map[string]any{"Err": errors.ErrInvalidInput}))
	}

	// Zero the confirmation copy.
	for i := range confirm {
		confirm[i] = 0
	}

	return pass, nil
}

// charClasses returns the number of distinct character classes in the
// passphrase (lowercase, uppercase, digits, symbols).
func charClasses(pass []byte) int {
	var lower, upper, digit, symbol bool
	for _, b := range pass {
		switch {
		case b >= 'a' && b <= 'z':
			lower = true
		case b >= 'A' && b <= 'Z':
			upper = true
		case b >= '0' && b <= '9':
			digit = true
		default:
			symbol = true
		}
	}
	n := 0
	if lower {
		n++
	}
	if upper {
		n++
	}
	if digit {
		n++
	}
	if symbol {
		n++
	}
	return n
}

// strengthLevel returns a bar and label representing passphrase strength based
// on length and character diversity. Shared by CLI, TUI, and wizard meters so
// the scoring algorithm stays in one place.
func strengthLevel(pass []byte) (bar, label string) {
	if len(pass) < 8 {
		return "[X____]", i18n.T("helpers.strength.tooShort")
	}
	classes := charClasses(pass)
	if classes < 2 {
		return "[X____]", i18n.T("helpers.strength.weak")
	}
	score := len(pass) + classes*3
	if score >= 22 {
		return "[XXXXX]", i18n.T("helpers.strength.strong")
	}
	return "[XXX__]", i18n.T("helpers.strength.fair")
}

// displayStrengthMeter prints a passphrase strength meter to stderr based on
// length and character diversity. The meter is informational only; all lengths
// >= 8 are accepted.
func displayStrengthMeter(pass []byte) {
	bar, label := strengthLevel(pass)
	fmt.Fprint(os.Stderr, i18n.Tf("helpers.strength.display", map[string]any{"Bar": bar, "Label": label}))
}

// vaultDir returns the directory containing the vault file at the given path.
func vaultDir(vaultPath string) string {
	return filepath.Dir(vaultPath)
}

// promptConfirmation prompts the user with a yes/no question and returns true
// only if the user types "y" or "yes" (case-insensitive). Defaults to no.
func promptConfirmation(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

// promptSecret reads a secret value with echo disabled. Used for interactive
// credential entry.
func promptSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	secret, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden input
	if err != nil {
		return "", fmt.Errorf("%s", i18n.Tf("helpers.error.readSecret", map[string]any{"Err": err}))
	}
	return string(secret), nil
}

// openAndUnlock opens a vault and unlocks it with the given passphrase.
// It also fires MaybeAutoStart so the Docker audit stack starts in the
// background on every CLI command, matching TUI and GUI behaviour.
func openAndUnlock(vaultPath string, passphrase []byte) (*vault.Manager, error) {
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		return nil, err
	}
	if err := mgr.Unlock(passphrase); err != nil {
		mgr.Close()
		return nil, err
	}
	// Auto-start Docker audit stack if configured (D-09, D-10).
	// Uses EnsureStack (synchronous) so the stack is ready before the
	// command runs — MaybeAutoStart's goroutine would be killed on CLI exit.
	// No-op when DockerComposePath is empty or AutoStart is false (D-11).
	// Passes bundleFS so docker-compose.yml is synced on each run, keeping
	// the live stack config current after binary upgrades.
	if cfg, err := config.Load(filepath.Dir(vaultPath)); err == nil {
		// Load HMAC secret from vault and inject into config so EnsureStack
		// can successfully probe the ledger.
		secretFromVault := mgr.GetSecret("audit.secret_key")
		if secretFromVault != "" {
			cfg.Audit.SecretKey = secretFromVault
		}
		defer func() { cfg.Audit.SecretKey = "" }()

		// Load the ledger volume key from vault so the encrypted data directory
		// can be decrypted and mounted for postgres to access.
		volumeKeyHex := mgr.GetSecret("audit.ledger_volume_key")
		if volumeKeyHex != "" {
			if volumeKey, err := hex.DecodeString(volumeKeyHex); err == nil {
				cfg.Audit.LedgerVolumeKey = volumeKey
				defer zeroBytes(volumeKey)
			}
		}

		bundleFS, _ := fs.Sub(dockerBundle, "docker-bundle")
		if err := audit.EnsureStack(cfg.Audit, bundleFS, nil); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("helpers.error.autoStart", map[string]any{"Err": err}))
		}
	}
	return mgr, nil
}

// printAuditNotEnabledHint writes the standard audit-not-enabled guidance to w.
// Called by any command that requires audit to be enabled.
func printAuditNotEnabledHint(w io.Writer) {
	_, _ = fmt.Fprintln(w, i18n.T("helpers.audit.notEnabled"))
	_, _ = fmt.Fprintln(w, i18n.T("helpers.audit.quickSetup"))
	_, _ = fmt.Fprintln(w, i18n.T("helpers.audit.manualSetup"))
	_, _ = fmt.Fprintln(w, i18n.T("helpers.audit.helpHint"))
}

// zeroBytes overwrites a byte slice with zeros for memory hygiene.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// decodeBase32Secret decodes a base32-encoded OTP secret, tolerating spaces,
// hyphens, lowercase, missing padding, any input length, and common digit
// lookalikes (0→O, 1→L, 8→B).
func decodeBase32Secret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.NewReplacer(
		" ", "", "-", "", "=", "",
		"0", "O", "1", "L", "8", "B",
	).Replace(secret))

	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}

// newEventBuilder constructs an EventBuilder from config and the vault passphrase.
// Returns a disabled builder (no-op) when cfg.Audit.Enabled is false.
func newEventBuilder(cfg config.Config, vaultDir string, passphrase []byte) (*audit.EventBuilder, error) {
	return audit.NewEventBuilderFromConfig(cfg.Audit, vaultDir, passphrase)
}

// setupAuditBuilder loads config, initialises an EventBuilder, wires up the
// OnHashStored callback so hashes are persisted in the vault, and emits a
// vault-unlock event. Returns nil when audit is disabled or unavailable.
// The caller must defer builder.Close() when the return value is non-nil.
//
// Every CLI invocation that opens the vault logs a vault-unlock event followed
// by the operation-specific event. This is intentional: each invocation
// decrypts the vault from scratch, so vault-unlock accurately reflects that a
// session was started. Audit consumers will see one vault-unlock per command
// (e.g., tegata add → vault-unlock + credential-add).
func setupAuditBuilder(w io.Writer, dir string, passphrase []byte, mgr *vault.Manager) *audit.EventBuilder {
	cfg, _ := config.Load(dir)
	if secret := mgr.GetSecret("audit.secret_key"); secret != "" {
		cfg.Audit.SecretKey = secret
	}
	defer func() { cfg.Audit.SecretKey = "" }()
	builder, err := newEventBuilder(cfg, dir, passphrase)
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s", i18n.Tf("cmd.changePassphrase.warn.auditUnavailable", map[string]any{"Err": err}))
		return nil
	}
	if builder == nil {
		return nil
	}
	builder.OnHashStored = func(eventID, hashValue string) {
		if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
			_, _ = fmt.Fprintf(w, "%s", i18n.Tf("cmd.changePassphrase.warn.auditHash", map[string]any{"Err": err}))
		}
	}
	if logErr := builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
		_, _ = fmt.Fprintf(w, "%s", i18n.Tf("cmd.changePassphrase.warn.auditFailed", map[string]any{"Err": logErr}))
	}
	builder.LogVaultLockOnClose = true
	return builder
}

// humanizeError translates OS and filesystem errors into user-friendly messages.
// Falls through to the original error for unknown types.
func humanizeError(err error) string {
	if err == nil {
		return i18n.T("helpers.error.unknown")
	}

	msg := err.Error()

	// File not found (check both os.IsNotExist and text patterns for wrapped errors)
	if os.IsNotExist(err) || strings.Contains(msg, "no such file or directory") || strings.Contains(msg, "file does not exist") {
		return i18n.T("helpers.error.vaultNotFound")
	}

	// Permission denied (check both os.IsPermission and text pattern for wrapped errors)
	if os.IsPermission(err) || strings.Contains(msg, "permission denied") {
		return i18n.T("helpers.error.permissionDenied")
	}

	// Read-only filesystem
	if strings.Contains(msg, "read-only file system") {
		return i18n.T("helpers.error.readOnly")
	}

	// Vault file is corrupt or invalid
	if strings.Contains(msg, "invalid header") || strings.Contains(msg, "corrupted") {
		return i18n.T("helpers.error.corrupt")
	}

	// Fall back to original error if no pattern matches
	return msg
}

// truncateVaultPath returns a truncated vault path that fits within maxWidth.
// If the path fits entirely, it is returned as-is. If truncation is needed,
// the start and end of the path are shown with "..." in the middle.
// Example: /Volumes/External_Drive.../my-vault.tegata
func truncateVaultPath(path string, maxWidth int) string {
	runes := []rune(path)
	if len(runes) <= maxWidth {
		return path
	}
	if maxWidth < 10 {
		return "vault" // minimal fallback
	}

	// Reserve space for "..." (3 chars).
	ellipsis := "..."
	usableWidth := maxWidth - len(ellipsis)
	startWidth := usableWidth / 2
	endWidth := usableWidth - startWidth

	return string(runes[:startWidth]) + ellipsis + string(runes[len(runes)-endWidth:])
}

// formatVaultPathWithBoldFilename renders a vault path with the filename
// bolded for visual distinction. The prefix "Vault: " and directory part are
// rendered as plain text. Example output:
// Vault: /path/to/vault.tegata  (filename portion is bold)
func formatVaultPathWithBoldFilename(path string) string {
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	boldStyle := lipgloss.NewStyle().Bold(true)

	// If there's no directory (current dir or just filename), render just the
	// bold filename (no color), matching previous behavior.
	if dir == "." || dir == "" {
		return boldStyle.Render(filename)
	}

	// Translated "Vault: " prefix + directory, then concat bold filename.
	separator := string(filepath.Separator)
	return i18n.T("tui.vaultHeader") + dir + separator + boldStyle.Render(filename)
}

// unlockVaultForSecret opens a vault and returns the HMAC secret from encrypted storage.
// It uses interactive prompts for vault path and passphrase, with opt-in environment
// variables for automation (TEGATA_VAULT_PATH and TEGATA_VAULT_PASSPHRASE).
func unlockVaultForSecret(cmd *cobra.Command) (string, error) {
	// Prompt for vault path if not provided via flag or env var.
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return "", err
	}

	// Prompt for passphrase interactively, or use environment variable if set.
	// Always work with a byte slice so the passphrase can be zeroed after use.
	var passBytes []byte
	if envPass := os.Getenv("TEGATA_VAULT_PASSPHRASE"); envPass != "" {
		passBytes = []byte(envPass)
	} else {
		fmt.Fprint(os.Stderr, i18n.T("cmd.ledger.prompt.passphrase"))
		var err error
		passBytes, err = term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", fmt.Errorf("%s", i18n.Tf("helpers.error.readPassphrase", map[string]any{"Err": err}))
		}
		fmt.Fprintln(os.Stderr) // newline after password input
	}
	defer func() {
		for i := range passBytes {
			passBytes[i] = 0
		}
	}()

	// Open and unlock the vault.
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		return "", fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.openVault", map[string]any{"Err": err}))
	}
	defer mgr.Close()

	if err := mgr.Unlock(passBytes); err != nil {
		return "", fmt.Errorf("%s", i18n.Tf("cmd.ledger.error.unlockVault", map[string]any{"Err": err}))
	}

	// Retrieve the secret from the vault.
	secret := mgr.GetSecret("audit.secret_key")
	if secret == "" {
		return "", fmt.Errorf("HMAC secret not found in vault. Run 'tegata ledger setup' to register the secret")
	}

	return secret, nil
}
