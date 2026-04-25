package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/vault"
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
		printAuditNotEnabledHint(os.Stderr)
		return nil
	}

	passphrase, err := promptPassphrase("Passphrase: ")
	if err != nil {
		return err
	}
	mgr, err := openAndUnlock(vaultPath, passphrase)
	zeroBytes(passphrase)
	if err != nil {
		return err
	}
	defer mgr.Close()

	hashes := mgr.AuditHashes()
	defer vault.ZeroAuditHashes(hashes)

	client, err := audit.NewClientFromConfig(cfg.Audit)
	if err != nil {
		return fmt.Errorf("%w: connecting to ledger: %s", tegerrors.ErrNetworkFailed, err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := audit.VerifyAll(ctx, client, cfg.Audit.EntityID, hashes)
	if err != nil {
		return err
	}

	if result.EventCount == 0 && result.Skipped == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No audit events found. Nothing to verify.")
		return nil
	}

	if result.Skipped > 0 {
		fmt.Fprintf(os.Stderr, "Note: %d events pre-date independent hash storage and were not verified.\n", result.Skipped)
	}

	if result.Valid {
		if result.EventCount > 0 {
			fmt.Printf("Audit log integrity verified. %d events checked.\n", result.EventCount)
		} else {
			fmt.Printf("No events could be verified — all %d events pre-date independent hash storage.\n", result.Skipped)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "Integrity violation detected in %d of %d events:\n", len(result.Faults), result.EventCount)
	for _, f := range result.Faults {
		fmt.Fprintf(os.Stderr, "  %s\n", f)
	}
	return tegerrors.ErrIntegrityViolation
}
