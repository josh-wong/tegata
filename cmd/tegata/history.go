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

			client, err := audit.NewClientFromConfig(cfg.Audit.Server, cfg.Audit.PrivilegedServer, cfg.Audit.EntityID, cfg.Audit.KeyVersion, cfg.Audit.SecretKey, cfg.Audit.Insecure)
			if err != nil {
				return fmt.Errorf("%w: %s", tegerrors.ErrNetworkFailed, err)
			}
			defer func() { _ = client.Close() }()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Retrieve event IDs from the entity's audit collection.
			collectionID := "tegata-audit-" + cfg.Audit.EntityID
			eventIDs, err := client.CollectionGet(ctx, collectionID)
			if err != nil {
				return err
			}

			// Fetch each event individually and build history records.
			var records []historyRecord
			for _, id := range eventIDs {
				evts, err := client.Get(ctx, id)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to get event %s: %v\n", id, err)
					continue
				}
				if len(evts) == 0 {
					continue
				}
				r := evts[0]
				records = append(records, historyRecord{
					ObjectID:  r.ObjectID,
					Operation: audit.MetadataString(r.Metadata, "operation"),
					LabelHash: audit.MetadataString(r.Metadata, "label_hash"),
					Timestamp: audit.MetadataInt64(r.Metadata, "timestamp"),
					HashValue: r.HashValue,
				})
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

			// Apply date filters.
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

// historyRecord is the display/JSON shape for a single history entry.
// Each record corresponds to one ScalarDL object with metadata.
type historyRecord struct {
	ObjectID  string `json:"object_id"`
	Operation string `json:"operation"`
	LabelHash string `json:"label_hash"`
	Timestamp int64  `json:"timestamp"`
	HashValue string `json:"hash_value"`
}

// filterRecords applies date filtering using the metadata timestamp.
func filterRecords(records []historyRecord, from, to time.Time) []historyRecord {
	if from.IsZero() && to.IsZero() {
		return records
	}
	var filtered []historyRecord
	for _, r := range records {
		t := time.Unix(r.Timestamp, 0).UTC()
		if !from.IsZero() && t.Before(from) {
			continue
		}
		if !to.IsZero() && t.After(to) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// printRecordsTable writes a human-readable tabular display of history records
// with operation, label hash, timestamp, and hash columns.
func printRecordsTable(records []historyRecord) {
	if len(records) == 0 {
		fmt.Println("No audit events found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Operation\tLabel\tTimestamp\tHash")
	_, _ = fmt.Fprintln(w, "---------\t-----\t---------\t----")
	for _, r := range records {
		label := r.LabelHash
		if len(label) > 12 {
			label = label[:12]
		}
		ts := time.Unix(r.Timestamp, 0).UTC().Format("2006-01-02 15:04:05")
		hash := r.HashValue
		if len(hash) > 16 {
			hash = hash[:16] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Operation, label, ts, hash)
	}
	_ = w.Flush()
}
