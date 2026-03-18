package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
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
	displayStrengthMeter(len(pass))

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

// displayStrengthMeter prints a passphrase strength meter to stderr based on
// character count. The meter is informational only; all lengths >= 8 are
// accepted.
func displayStrengthMeter(length int) {
	var bar, label string
	switch {
	case length >= 20:
		bar = "[XXXXX]"
		label = "Strong"
	case length >= 12:
		bar = "[XXX__]"
		label = "Fair"
	default:
		bar = "[X____]"
		label = "Weak"
	}
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
func openAndUnlock(vaultPath string, passphrase []byte) (*vault.Manager, error) {
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		return nil, err
	}
	if err := mgr.Unlock(passphrase); err != nil {
		mgr.Close()
		return nil, err
	}
	return mgr, nil
}

// zeroBytes overwrites a byte slice with zeros for memory hygiene.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// decodeBase32Secret decodes a base32-encoded OTP secret, tolerating spaces,
// hyphens, lowercase, and missing padding.
func decodeBase32Secret(secret string) ([]byte, error) {
	// Strip spaces and hyphens that users or QR scanners may include.
	s := strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "=", "").Replace(secret))

	// Add padding if needed. Base32 blocks are 8 chars.
	if pad := len(s) % 8; pad != 0 {
		s += strings.Repeat("=", 8-pad)
	}

	return base32.StdEncoding.DecodeString(s)
}

// newEventBuilder constructs an EventBuilder from config and the vault passphrase.
// Returns a disabled builder (no-op) when cfg.Audit.Enabled is false.
//
// Queue key derivation (AUDT-08): the queue is AES-256-GCM encrypted using a
// 32-byte key derived from the vault passphrase via Argon2id with a distinct salt.
// The salt is stored in the 32-byte queue file header. Deriving here — while
// the passphrase is still in scope — avoids a second passphrase prompt and keeps
// latency to one Argon2id call per command.
func newEventBuilder(cfg config.Config, vaultDir string, passphrase []byte) (*audit.EventBuilder, error) {
	if !cfg.Audit.Enabled {
		return audit.NewEventBuilder(nil, "", nil, 0)
	}

	queuePath := filepath.Join(vaultDir, "queue.tegata")

	// Read the Argon2id salt from the existing queue file header, or generate a
	// new one when the file does not yet exist.
	var queueSalt []byte
	if data, err := os.ReadFile(queuePath); err == nil && len(data) >= 32 {
		// Existing queue file: first 32 bytes are the Argon2id salt.
		queueSalt = make([]byte, 32)
		copy(queueSalt, data[:32])
	} else {
		// New queue: generate a fresh salt. LoadQueue / NewQueue will write it
		// to the file header on the first Save call.
		var genErr error
		queueSalt, genErr = crypto.GenerateSalt()
		if genErr != nil {
			return nil, fmt.Errorf("generating queue salt: %w", genErr)
		}
	}

	// Derive the 32-byte queue encryption key using Argon2id.
	// The distinct salt ensures the queue key is independent from the vault DEK
	// even though both use the same passphrase.
	keyBuf := crypto.DeriveKey(passphrase, queueSalt, crypto.DefaultParams)
	defer keyBuf.Destroy()

	// Copy key bytes out of the SecretBuffer before it is destroyed.
	queueKey := make([]byte, 32)
	copy(queueKey, keyBuf.Bytes())

	client, err := buildLedgerClient(cfg.Audit)
	if err != nil {
		// A failed ledger connection is not fatal — the queue will hold events.
		_, _ = fmt.Fprintf(os.Stderr, "tegata: audit ledger unavailable (%v); events will be queued\n", err)
		// Return a disabled builder so auth commands are not blocked.
		zeroBytes(queueKey)
		return audit.NewEventBuilder(nil, "", nil, 0)
	}

	eb, err := audit.NewEventBuilder(client, queuePath, queueKey, cfg.Audit.QueueMaxEvents)
	zeroBytes(queueKey)
	return eb, err
}

// buildLedgerClient constructs a LedgerClient from AuditConfig cert paths.
func buildLedgerClient(cfg config.AuditConfig) (audit.Submitter, error) {
	signer, err := buildSigner(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("building ECDSA signer: %w", err)
	}

	if cfg.Insecure {
		return audit.NewLedgerClientInsecure(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, signer)
	}

	tlsCfg, err := buildTLSConfig(cfg.CertPath, cfg.KeyPath, cfg.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("building TLS config: %w", err)
	}

	return audit.NewLedgerClient(cfg.Server, cfg.PrivilegedServer, tlsCfg, cfg.EntityID, cfg.KeyVersion, signer)
}

// buildTLSConfig constructs a *tls.Config from cert, key, and CA PEM file paths.
func buildTLSConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("reading client certificate from %s: %w", certPath, err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading private key from %s: %w", keyPath, err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("loading TLS key pair: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	if caPath != "" {
		caPEM, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate from %s: %w", caPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parsing CA certificate from %s: no valid PEM block found", caPath)
		}
		tlsCfg.RootCAs = pool
	}

	return tlsCfg, nil
}

// buildSigner loads an ECDSA private key PEM file and returns a Signer.
func buildSigner(keyPath string) (audit.Signer, error) {
	if keyPath == "" {
		return &audit.NoOpSigner{}, nil
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading private key from %s: %w", keyPath, err)
	}
	return audit.NewECDSASigner(keyPEM)
}

// hostname returns the current machine hostname. On error returns an empty
// string so audit events can still be emitted without a valid host field.
func hostname() string {
	h, _ := os.Hostname()
	return h
}
