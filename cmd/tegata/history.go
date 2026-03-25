package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/spf13/cobra"
)

func newHistoryCmd() *cobra.Command {
	var (
		from    string
		to      string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "View authentication history from ScalarDL Ledger",
		Long: `Retrieve and display authentication event records from the ScalarDL Ledger.
Events are stored as hashed records; label and service name hashes protect
user privacy in the audit log.

Requires audit to be enabled in tegata.toml ([audit] enabled = true).`,
		Example: `  tegata history
  tegata history --from 2026-01-01 --to 2026-03-31
  tegata history --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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
				return fmt.Errorf("%w: %s", tegerrors.ErrNetworkFailed, err)
			}
			if closer, ok := client.(interface{ Close() error }); ok {
				defer func() { _ = closer.Close() }()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Retrieve all records from the ledger using the "tegata-" prefix.
			records, err := client.Get(ctx, "tegata-"+cfg.Audit.EntityID)
			if err != nil {
				return err
			}

			// Parse --from and --to date filters.
			var fromTime, toTime time.Time
			if from != "" {
				fromTime, err = time.Parse("2006-01-02", from)
				if err != nil {
					return fmt.Errorf("invalid --from date %q (expected YYYY-MM-DD): %w",
						from, tegerrors.ErrInvalidInput)
				}
			}
			if to != "" {
				toTime, err = time.Parse("2006-01-02", to)
				if err != nil {
					return fmt.Errorf("invalid --to date %q (expected YYYY-MM-DD): %w",
						to, tegerrors.ErrInvalidInput)
				}
				// Include the full end day.
				toTime = toTime.Add(24*time.Hour - time.Nanosecond)
			}

			// Apply date filters and build the display list.
			filtered := filterRecords(records, fromTime, toTime)

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			printRecordsTable(filtered)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "start date filter (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date filter (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON array")

	return cmd
}

// buildAuditClient creates a Client from AuditConfig for history and verify commands.
func buildAuditClient(cfg config.AuditConfig) (audit.Client, error) {
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("audit.secret_key is required")
	}
	signer := audit.NewHMACSigner(cfg.SecretKey)

	if cfg.Insecure {
		return audit.NewLedgerClientInsecure(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, signer)
	}

	return nil, fmt.Errorf("TLS mode not yet supported with HMAC auth — set insecure = true")
}

// historyRecord is the display/JSON shape for a single history entry.
// ScalarDL stores hashes, so we display the raw hash values rather than
// attempting to reverse them.
type historyRecord struct {
	HashValue string `json:"hash_value"`
	Version   int64  `json:"version"`
}

// filterRecords converts EventRecords to historyRecords. Date filtering is
// not available since ScalarDL stores version numbers, not timestamps.
func filterRecords(records []*audit.EventRecord, from, to time.Time) []historyRecord {
	result := make([]historyRecord, len(records))
	for i, r := range records {
		result[i] = historyRecord{
			HashValue: r.HashValue,
			Version:   r.Version,
		}
	}
	return result
}

// printRecordsTable writes a human-readable tabular display of history records.
func printRecordsTable(records []historyRecord) {
	if len(records) == 0 {
		fmt.Println("No audit events found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Version\tHash Value")
	_, _ = fmt.Fprintln(w, "-------\t----------")
	for _, r := range records {
		_, _ = fmt.Fprintf(w, "%d\t%s\n", r.Version, r.HashValue)
	}
	_ = w.Flush()
}
