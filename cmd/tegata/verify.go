package main

import (
	"context"
	"fmt"
	"os"
	"time"

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

	client, err := buildAuditClient(cfg.Audit)
	if err != nil {
		return fmt.Errorf("%w: connecting to ledger: %s", tegerrors.ErrNetworkFailed, err)
	}
	if closer, ok := client.(interface{ Close() error }); ok {
		defer func() { _ = closer.Close() }()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collectionID := "tegata-audit-" + cfg.Audit.EntityID
	eventIDs, err := client.CollectionGet(ctx, collectionID)
	if err != nil {
		return err
	}

	if len(eventIDs) == 0 {
		fmt.Fprintln(os.Stdout, "No audit events found. Nothing to verify.")
		return nil
	}

	var faults []string
	for _, id := range eventIDs {
		result, err := client.Validate(ctx, id)
		if err != nil {
			faults = append(faults, fmt.Sprintf("%s: error: %v", id, err))
			continue
		}
		if !result.Valid {
			faults = append(faults, fmt.Sprintf("%s: %s", id, result.ErrorDetail))
		}
	}

	if len(faults) == 0 {
		fmt.Printf("Audit log integrity verified. %d events checked.\n", len(eventIDs))
		return nil
	}

	fmt.Fprintf(os.Stderr, "Integrity violation detected in %d of %d events:\n", len(faults), len(eventIDs))
	for _, f := range faults {
		fmt.Fprintf(os.Stderr, "  %s\n", f)
	}
	return tegerrors.ErrIntegrityViolation
}
