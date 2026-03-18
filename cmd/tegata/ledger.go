package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

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
		Short: "Register client certificate and verify connectivity",
		Long: `Register the client TLS certificate with the ScalarDL LedgerPrivileged
service and verify that the ledger is reachable.

This command must be run once before audit logging is active. It reads the
[audit] section from tegata.toml (located in the vault directory) and uses
the configured certificate paths and server address.

See docs/scalardl-setup.md for certificate generation and configuration steps.`,
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

	// Read client certificate PEM.
	certPEM, err := os.ReadFile(cfg.Audit.CertPath)
	if err != nil {
		return fmt.Errorf("reading client certificate from %s: %w", cfg.Audit.CertPath, err)
	}

	// Read private key PEM.
	keyPEM, err := os.ReadFile(cfg.Audit.KeyPath)
	if err != nil {
		return fmt.Errorf("reading private key from %s: %w", cfg.Audit.KeyPath, err)
	}

	// Build TLS config from cert + key.
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("loading client TLS key pair: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Load CA certificate if provided.
	if cfg.Audit.CACertPath != "" {
		caPEM, err := os.ReadFile(cfg.Audit.CACertPath)
		if err != nil {
			return fmt.Errorf("reading CA certificate from %s: %w", cfg.Audit.CACertPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return fmt.Errorf("parsing CA certificate from %s: no valid PEM block found", cfg.Audit.CACertPath)
		}
		tlsCfg.RootCAs = pool
	}

	// Create ECDSA signer from private key.
	signer, err := audit.NewECDSASigner(keyPEM)
	if err != nil {
		return fmt.Errorf("creating ECDSA signer: %w", err)
	}

	// Dial the ScalarDL Ledger.
	fmt.Fprintf(os.Stderr, "Connecting to ScalarDL Ledger at %s...\n", cfg.Audit.Server)
	client, err := audit.NewLedgerClient(cfg.Audit.Server, tlsCfg, cfg.Audit.EntityID, cfg.Audit.KeyVersion, signer)
	if err != nil {
		return fmt.Errorf("%w: connecting to ledger: %s", tegerrors.ErrNetworkFailed, err)
	}
	defer client.Close()

	// Register the client certificate with the LedgerPrivileged service.
	ctx := context.Background()
	fmt.Fprintf(os.Stderr, "Registering certificate for entity %q (key version %d)...\n",
		cfg.Audit.EntityID, cfg.Audit.KeyVersion)
	if err := client.RegisterCert(ctx, cfg.Audit.EntityID, cfg.Audit.KeyVersion, string(certPEM)); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Certificate registered successfully.")

	// Ping the Ledger service to confirm connectivity.
	fmt.Fprintln(os.Stderr, "Verifying ledger connectivity...")
	if err := client.Ping(ctx); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "ScalarDL Ledger is reachable. Audit setup complete.")
	return nil
}
