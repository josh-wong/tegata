package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
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
	testObjID := fmt.Sprintf("tegata-setup-test-%d", time.Now().UnixNano())
	return client.Put(ctx, testObjID, "0000000000000000000000000000000000000000000000000000000000000000")
}
