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

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify hash-chain integrity of the audit log",
		Long: `Call the ScalarDL Ledger to verify the integrity of all audit records.
Reports the number of events checked and whether the hash chain is intact.

Exits with code 0 on success, code 9 on integrity violation, or code 8 on
network failure.

Requires audit to be enabled in tegata.toml ([audit] enabled = true).`,
		Example: `  tegata verify
  tegata verify --vault /media/usb`,
		Args: cobra.NoArgs,
		RunE: runVerify,
	}
}

func runVerify(cmd *cobra.Command, _ []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.Load(vaultDir(vaultPath))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if !cfg.Audit.Enabled {
		fmt.Fprintln(os.Stderr, "Audit is not enabled. Add [audit] enabled = true to tegata.toml.")
		fmt.Fprintln(os.Stderr, "See docs/scalardl-setup.md for configuration instructions.")
		return nil
	}

	client, err := audit.NewClientFromConfig(cfg.Audit.Server, cfg.Audit.PrivilegedServer, cfg.Audit.EntityID, cfg.Audit.KeyVersion, cfg.Audit.SecretKey, cfg.Audit.Insecure)
	if err != nil {
		return fmt.Errorf("%w: connecting to ledger: %s", tegerrors.ErrNetworkFailed, err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := audit.VerifyAll(ctx, client, cfg.Audit.EntityID)
	if err != nil {
		return err
	}

	if result.EventCount == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No audit events found. Nothing to verify.")
		return nil
	}

	if result.Valid {
		fmt.Printf("Audit log integrity verified. %d events checked.\n", result.EventCount)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Integrity violation detected in %d of %d events:\n", len(result.Faults), result.EventCount)
	for _, f := range result.Faults {
		fmt.Fprintf(os.Stderr, "  %s\n", f)
	}
	return tegerrors.ErrIntegrityViolation
}
