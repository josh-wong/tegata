package main

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/spf13/cobra"
)

// newLedgerCmd returns the 'tegata ledger' command with its subcommands.
func newLedgerCmd() *cobra.Command {
	ledgerCmd := &cobra.Command{
		Use:   "ledger",
		Short: "Manage ScalarDL Ledger connection",
		Long: `Commands for configuring and verifying the optional ScalarDL Ledger
audit integration. The audit layer records authentication events in a
tamper-evident ledger for post-hoc integrity verification.`,
	}

	ledgerCmd.AddCommand(newLedgerSetupCmd())
	ledgerCmd.AddCommand(newLedgerStartCmd())
	ledgerCmd.AddCommand(newLedgerStopCmd())
	return ledgerCmd
}

// newLedgerSetupCmd returns the 'tegata ledger setup' command.
func newLedgerSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Register HMAC secret and verify connectivity",
		Long: `Register the HMAC secret key with the ScalarDL LedgerPrivileged
service and verify that the ledger is reachable.

This command must be run once before audit logging is active. It reads the
[audit] section from tegata.toml (located in the vault directory) and uses
the configured secret key and server address.

See docs/scalardl-setup.md for configuration steps.`,
		Example: `  tegata ledger setup
  tegata ledger setup --vault /media/usb`,
		Args: cobra.NoArgs,
		RunE: runLedgerSetup,
	}
}

func runLedgerSetup(cmd *cobra.Command, _ []string) error {
	// Resolve vault path and directory.
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}
	dir := vaultDir(vaultPath)

	// Load configuration from tegata.toml.
	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if !cfg.Audit.Enabled {
		fmt.Fprintln(os.Stderr, "Audit not enabled. Add [audit] section to tegata.toml.")
		fmt.Fprintln(os.Stderr, "See docs/scalardl-setup.md for configuration instructions.")
		return nil
	}

	if cfg.Audit.SecretKey == "" {
		return fmt.Errorf("audit.secret_key is required in tegata.toml")
	}

	// Create HMAC signer from secret key.
	signer := audit.NewHMACSigner(cfg.Audit.SecretKey)
	defer signer.Zero()

	// Dial the ScalarDL Ledger.
	fmt.Fprintf(os.Stderr, "Connecting to ScalarDL Ledger at %s (privileged: %s)...\n",
		cfg.Audit.Server, cfg.Audit.PrivilegedServer)
	var client *audit.LedgerClient
	if cfg.Audit.Insecure {
		fmt.Fprintln(os.Stderr, "WARNING: Insecure mode enabled — TLS disabled. Do not use in production.")
		client, err = audit.NewLedgerClientInsecure(cfg.Audit.Server, cfg.Audit.PrivilegedServer, cfg.Audit.EntityID, cfg.Audit.KeyVersion, signer)
	} else {
		return fmt.Errorf("TLS mode not yet supported with HMAC auth — set insecure = true for local development")
	}
	if err != nil {
		return fmt.Errorf("%w: connecting to ledger: %s", tegerrors.ErrNetworkFailed, err)
	}
	defer func() { _ = client.Close() }()

	// Use a 30-second timeout covering both registration and verification.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register the HMAC secret with the LedgerPrivileged service.
	fmt.Fprintf(os.Stderr, "Registering secret for entity %q (key version %d)...\n",
		cfg.Audit.EntityID, cfg.Audit.KeyVersion)
	if err := client.RegisterSecret(ctx, cfg.Audit.EntityID, cfg.Audit.KeyVersion, cfg.Audit.SecretKey); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Secret registered successfully.")

	// Ping the Ledger service to confirm connectivity.
	fmt.Fprintln(os.Stderr, "Verifying ledger connectivity...")
	if err := client.Ping(ctx); err != nil {
		return err
	}

	// Verify that the generic contracts are registered by attempting a test Put.
	fmt.Fprintln(os.Stderr, "Verifying generic contracts are registered...")
	if err := verifyContracts(ctx, client); err != nil {
		fmt.Fprintln(os.Stderr, "Generic contracts are NOT registered on this ScalarDL instance.")
		fmt.Fprintln(os.Stderr, "Register them using: docker compose run --rm scalardl-contract-registration")
		fmt.Fprintln(os.Stderr, "See docs/scalardl-setup.md for instructions.")
		return fmt.Errorf("contract verification failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Generic contracts verified. Audit setup complete.")
	return nil
}

// verifyContracts attempts a test Put to confirm that the generic contracts
// are registered on the ScalarDL instance.
func verifyContracts(ctx context.Context, client audit.Client) error {
	return client.Put(ctx, audit.SetupTestObjectID, "0000000000000000000000000000000000000000000000000000000000000000")
}

// newLedgerStartCmd returns the 'tegata ledger start' command.
func newLedgerStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "One-click Docker audit setup",
		Long: `Start the ScalarDL Ledger Docker stack and configure audit logging.

This command:
  1. Checks Docker is installed
  2. Extracts the bundled docker-compose.yml to ~/.tegata/docker/
  3. Generates an entity ID and secret key from your vault
  4. Starts the Docker stack (docker compose up -d)
  5. Waits for the ledger to become ready (up to 30 seconds)
  6. Registers audit credentials with the ledger
  7. Writes the [audit] section to tegata.toml

After running this command, audit logging is active immediately. Subsequent
vault unlocks auto-start the Docker stack if it is not already running.`,
		Example: `  tegata ledger start
  tegata ledger start --vault /media/usb`,
		Args: cobra.NoArgs,
		RunE: runLedgerStart,
	}
}

func runLedgerStart(cmd *cobra.Command, _ []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}
	dir := filepath.Dir(vaultPath)

	// Get vault VaultID for entity ID derivation.
	// We must open and unlock the vault to read the VaultID from the payload.
	passphraseBytes, err := promptPassphrase("Vault passphrase: ")
	if err != nil {
		return fmt.Errorf("reading passphrase: %w", err)
	}
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		zeroBytes(passphraseBytes)
		return fmt.Errorf("opening vault: %w", err)
	}
	if err := mgr.Unlock(passphraseBytes); err != nil {
		zeroBytes(passphraseBytes)
		mgr.Close()
		return fmt.Errorf("unlocking vault: %w", err)
	}
	vaultID := mgr.VaultID()
	zeroBytes(passphraseBytes)
	mgr.Close()

	// Resolve compose directory: ~/.tegata/docker/
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	composeDir := filepath.Join(u.HomeDir, ".tegata", "docker")

	// Strip the docker-bundle/ prefix from the embed.FS so SetupStack
	// receives an FS rooted at the docker-compose.yml level.
	bundleFS, err := fs.Sub(dockerBundle, "docker-bundle")
	if err != nil {
		return fmt.Errorf("accessing embedded docker bundle: %w", err)
	}

	// progress: print each step to stderr (per UI-SPEC CLI surface).
	progressFn := func(msg string) {
		fmt.Fprintln(os.Stderr, msg)
	}

	auditCfg, err := audit.SetupStack(bundleFS, composeDir, vaultID, progressFn)
	if err != nil {
		return err
	}

	// Write [audit] section to tegata.toml. Per D-03 step 8.
	if err := config.WriteAuditSection(dir, auditCfg); err != nil {
		return fmt.Errorf("writing audit config: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Ledger server started. Audit logging is now active.")
	return nil
}

// newLedgerStopCmd returns the 'tegata ledger stop' command.
func newLedgerStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the ledger server Docker containers",
		Long: `Stop the ScalarDL Ledger Docker containers.

By default, containers are stopped but your audit history is preserved
(docker compose stop, named volume retained).

Use --wipe to permanently delete all audit history (docker compose down -v).
This action cannot be undone.`,
		Example: `  tegata ledger stop
  tegata ledger stop --wipe`,
		Args: cobra.NoArgs,
		RunE: runLedgerStop,
	}
	cmd.Flags().Bool("wipe", false, "Permanently delete all audit history (cannot be undone)")
	return cmd
}

func runLedgerStop(cmd *cobra.Command, _ []string) error {
	wipe, _ := cmd.Flags().GetBool("wipe")

	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}
	dir := filepath.Dir(vaultPath)

	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.Audit.DockerComposePath == "" {
		return fmt.Errorf("audit Docker setup not found. Run 'tegata ledger start' first")
	}

	if wipe {
		// Print prominent warning and require "Yes" confirmation (per D-15).
		fmt.Fprintln(os.Stderr, "WARNING: This will permanently delete all audit history. Type \"Yes\" to confirm:")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if answer != "Yes" {
			fmt.Fprintln(os.Stderr, "Canceled. Audit history was not deleted.")
			return nil
		}
	}

	if err := audit.StopStack(cfg.Audit.DockerComposePath, wipe); err != nil {
		return err
	}

	if wipe {
		fmt.Fprintln(os.Stderr, "Audit history deleted and containers removed.")
	} else {
		fmt.Fprintln(os.Stderr, "Ledger server stopped. Your audit history is preserved.")
	}
	return nil
}
