package main

import (
	"bufio"
	"bytes"
	"encoding/base32"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
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
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	path := filepath.Join(cwd, vaultFilename)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s: %w",
			errors.UserMessage("No vault found",
				"Run tegata init to create one, or use --vault /path to specify a location"),
			errors.ErrNotFound)
	}
	return path, nil
}

// resolvePathArg resolves a user-provided path argument to a vault file path.
// If the path is a directory, it appends the vault filename. If it is a file
// path, it is used as-is.
func resolvePathArg(path string) (string, error) {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return filepath.Join(path, vaultFilename), nil
	}
	// If it's a file or doesn't exist yet (for init), return as-is.
	// If it looks like a directory path (ends with separator), append filename.
	if strings.HasSuffix(path, string(filepath.Separator)) {
		return filepath.Join(path, vaultFilename), nil
	}
	return path, nil
}

// promptPassphrase reads a passphrase using the precedence:
// 1. TEGATA_PASSPHRASE env var (with warning)
// 2. stdin pipe (if not a terminal)
// 3. Interactive prompt (echo disabled)
func promptPassphrase(prompt string) ([]byte, error) {
	// Check env var first.
	if envPass := os.Getenv("TEGATA_PASSPHRASE"); envPass != "" {
		fmt.Fprintln(os.Stderr, "! Using passphrase from TEGATA_PASSPHRASE")
		return []byte(envPass), nil
	}

	// Check if stdin is piped.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		reader := bufio.NewReader(os.Stdin)
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("reading passphrase from stdin: %w", err)
		}
		// Trim trailing newline from piped input.
		return []byte(strings.TrimRight(string(data), "\r\n")), nil
	}

	// Interactive prompt.
	fmt.Fprint(os.Stderr, prompt)
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden input
	if err != nil {
		return nil, fmt.Errorf("reading passphrase: %w", err)
	}
	return pass, nil
}

// promptNewPassphrase handles passphrase creation for tegata init. It displays
// a tip, prompts for the passphrase with a strength meter, enforces a minimum
// length, and confirms the passphrase.
func promptNewPassphrase() ([]byte, error) {
	fmt.Fprintln(os.Stderr, `! Tip: Use a memorable phrase (e.g., "correct horse battery staple")`)

	pass, err := promptPassphrase("Passphrase: ")
	if err != nil {
		return nil, err
	}

	if len(pass) < 8 {
		return nil, fmt.Errorf("passphrase must be at least 8 characters: %w", errors.ErrInvalidInput)
	}

	// Display strength meter.
	displayStrengthMeter(pass)

	// Confirm passphrase (skip for non-interactive modes).
	if os.Getenv("TEGATA_PASSPHRASE") != "" || !term.IsTerminal(int(os.Stdin.Fd())) {
		return pass, nil
	}

	confirm, err := promptPassphrase("Confirm passphrase: ")
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
		return nil, fmt.Errorf("passphrases do not match: %w", errors.ErrInvalidInput)
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
		return "[X____]", "Too short"
	}
	classes := charClasses(pass)
	if classes < 2 {
		return "[X____]", "Weak"
	}
	score := len(pass) + classes*3
	if score >= 22 {
		return "[XXXXX]", "Strong"
	}
	return "[XXX__]", "Fair"
}

// displayStrengthMeter prints a passphrase strength meter to stderr based on
// length and character diversity. The meter is informational only; all lengths
// >= 8 are accepted.
func displayStrengthMeter(pass []byte) {
	bar, label := strengthLevel(pass)
	fmt.Fprintf(os.Stderr, "  Strength: %s %s\n", bar, label)
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
		return "", fmt.Errorf("reading secret: %w", err)
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
		bundleFS, _ := fs.Sub(dockerBundle, "docker-bundle")
		if err := audit.EnsureStack(cfg.Audit, bundleFS, nil); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata: audit auto-start: %v\n", err)
		}
	}
	return mgr, nil
}

// printAuditNotEnabledHint writes the standard audit-not-enabled guidance to w.
// Called by any command that requires audit to be enabled.
func printAuditNotEnabledHint(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Audit logging is not enabled. To enable, choose one of:")
	_, _ = fmt.Fprintln(w, "  Quick setup (Docker): tegata ledger start --vault <path>")
	_, _ = fmt.Fprintln(w, "  Manual setup: add [audit] to tegata.toml and run: tegata ledger setup --vault <path>")
	_, _ = fmt.Fprintln(w, "Run 'tegata ledger setup --help' for the required tegata.toml fields.")
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
// (e.g. tegata add → vault-unlock + credential-add).
func setupAuditBuilder(w io.Writer, dir string, passphrase []byte, mgr *vault.Manager) *audit.EventBuilder {
	cfg, _ := config.Load(dir)
	builder, err := newEventBuilder(cfg, dir, passphrase)
	if err != nil {
		_, _ = fmt.Fprintf(w, "Warning: Audit unavailable: %v\n", err)
		return nil
	}
	if builder == nil {
		return nil
	}
	builder.OnHashStored = func(eventID, hashValue string) {
		if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
			_, _ = fmt.Fprintf(w, "Warning: Failed to store audit hash: %v\n", err)
		}
	}
	if logErr := builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
		_, _ = fmt.Fprintf(w, "Warning: Audit log failed: %v\n", logErr)
	}
	builder.LogVaultLockOnClose = true
	return builder
}

// humanizeError translates OS and filesystem errors into user-friendly messages.
// Falls through to the original error for unknown types.
func humanizeError(err error) string {
	if err == nil {
		return "Unknown error"
	}

	msg := err.Error()

	// File not found (check both os.IsNotExist and text patterns for wrapped errors)
	if os.IsNotExist(err) || strings.Contains(msg, "no such file or directory") || strings.Contains(msg, "file does not exist") {
		return "Vault file not found. Check the path and try again."
	}

	// Permission denied (check both os.IsPermission and text pattern for wrapped errors)
	if os.IsPermission(err) || strings.Contains(msg, "permission denied") {
		return "Permission denied. Check file permissions and try again."
	}

	// Read-only filesystem
	if strings.Contains(msg, "read-only file system") {
		return "Cannot write to this location—it appears to be read-only."
	}

	// Vault file is corrupt or invalid
	if strings.Contains(msg, "invalid header") || strings.Contains(msg, "corrupted") {
		return "The vault file appears to be corrupt. Restore from a backup if available."
	}

	// Fall back to original error if no pattern matches
	return msg
}

// truncateVaultPath returns a truncated vault path that fits within maxWidth.
// If the path fits entirely, it is returned as-is. If truncation is needed,
// the start and end of the path are shown with "..." in the middle.
// Example: /Volumes/External_Drive.../my-vault.tegata
func truncateVaultPath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	if maxWidth < 10 {
		return "vault" // minimal fallback
	}

	// Reserve space for "..." (3 chars) + some buffer
	ellipsis := "..."
	usableWidth := maxWidth - len(ellipsis)
	startWidth := usableWidth / 2
	endWidth := usableWidth - startWidth

	return path[:startWidth] + ellipsis + path[len(path)-endWidth:]
}

