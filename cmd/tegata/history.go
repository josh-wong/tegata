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
			records, err := client.Get(ctx, "tegata-")
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
// These commands do not need the passphrase — they use TLS cert only.
// In TLS mode the private key PEM is read once and shared between the ECDSA
// signer and the TLS config to avoid a second disk read and extra heap copy.
func buildAuditClient(cfg config.AuditConfig) (audit.Client, error) {
	if cfg.Insecure {
		signer, err := buildSigner(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("building ECDSA signer: %w", err)
		}
		return audit.NewLedgerClientInsecure(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, signer)
	}

	// Read key PEM once; shared between signer and TLS config.
	keyPEM, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading private key from %s: %w", cfg.KeyPath, err)
	}
	defer zeroBytes(keyPEM)

	signer, err := audit.NewECDSASigner(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("building ECDSA signer: %w", err)
	}

	tlsCfg, err := buildTLSConfigFromBytes(cfg.CertPath, keyPEM, cfg.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("building TLS config: %w", err)
	}

	return audit.NewLedgerClient(cfg.Server, cfg.PrivilegedServer, tlsCfg, cfg.EntityID, cfg.KeyVersion, signer)
}

// historyRecord is the display/JSON shape for a single history entry.
// ScalarDL stores hashes, so we display the raw hash values rather than
// attempting to reverse them.
type historyRecord struct {
	ObjectID  string `json:"object_id"`
	HashValue string `json:"hash_value"`
	Timestamp int64  `json:"timestamp"`
}

// filterRecords applies date filters to the records using the Timestamp field
// (unix epoch seconds). From/to values that are zero are treated as no filter.
func filterRecords(records []*audit.EventRecord, from, to time.Time) []historyRecord {
	var result []historyRecord
	for _, r := range records {
		if !from.IsZero() || !to.IsZero() {
			eventTime := time.Unix(r.Timestamp, 0).UTC()
			if !from.IsZero() && eventTime.Before(from) {
				continue
			}
			if !to.IsZero() && eventTime.After(to) {
				continue
			}
		}

		result = append(result, historyRecord{
			ObjectID:  r.ObjectID,
			HashValue: r.HashValue,
			Timestamp: r.Timestamp,
		})
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
	_, _ = fmt.Fprintln(w, "Object ID\tHash Value\tTimestamp")
	_, _ = fmt.Fprintln(w, "---------\t----------\t---------")
	for _, r := range records {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", r.ObjectID, r.HashValue, r.Timestamp)
	}
	_ = w.Flush()
}
