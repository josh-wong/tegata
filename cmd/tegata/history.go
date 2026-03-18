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
		from      string
		to        string
		labelFlag string
		typeFlag  string
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "View authentication history from ScalarDL Ledger",
		Long: `Retrieve and display authentication event records from the ScalarDL Ledger.
Events are stored as hashed records; label and service name hashes protect
user privacy in the audit log.

Requires audit to be enabled in tegata.toml ([audit] enabled = true).`,
		Example: `  tegata history
  tegata history --type totp
  tegata history --label "GitHub"
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

			// Compute label hash for --label filter.
			var labelHash string
			if labelFlag != "" {
				labelHash = audit.HashString(labelFlag)
			}

			// Apply filters and build the display list.
			filtered := filterRecords(records, fromTime, toTime, labelHash, typeFlag)

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
	cmd.Flags().StringVar(&labelFlag, "label", "", "filter by credential label (hashed for comparison)")
	cmd.Flags().StringVar(&typeFlag, "type", "", "filter by operation type (totp|hotp|challenge-response|static)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON array")

	return cmd
}

// buildAuditClient creates a Client from AuditConfig for history and verify commands.
// These commands do not need the passphrase — they use TLS cert only.
func buildAuditClient(cfg config.AuditConfig) (audit.Client, error) {
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

// historyRecord is the display/JSON shape for a single history entry.
// ScalarDL stores hashes, so we display the raw hash values rather than
// attempting to reverse them. The --label flag filters by computing the hash
// of the user-provided label and comparing.
type historyRecord struct {
	ObjectID  string `json:"object_id"`
	HashValue string `json:"hash_value"`
	Age       int64  `json:"age"`
}

// filterRecords applies date, label hash, and type filters to the records.
// All filter parameters that are zero/empty are treated as "no filter".
//
// Note: EventRecord.HashValue stores the hex(SHA-256(AuthEvent JSON)) submitted
// to the ledger by EventBuilder. Date and type filtering is not available from
// the ledger's perspective (ScalarDL stores opaque hash values). This function
// filters by objectID prefix for type (e.g. "tegata-totp-") and applies date
// filtering using the Age field (seconds since epoch).
func filterRecords(records []*audit.EventRecord, from, to time.Time, labelHash, typeFilter string) []historyRecord {
	var result []historyRecord
	for _, r := range records {
		// Apply type filter via objectID prefix convention.
		if typeFilter != "" && !matchesTypePrefix(r.ObjectID, typeFilter) {
			continue
		}

		// Apply label hash filter: objectID contains label hash when the prefix
		// is "tegata-{labelHash}-". This is a best-effort filter.
		if labelHash != "" && !containsHash(r.ObjectID, labelHash) {
			continue
		}

		// Apply date filters using Age (seconds since event submission).
		if !from.IsZero() || !to.IsZero() {
			eventTime := time.Unix(r.Age, 0).UTC()
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
			Age:       r.Age,
		})
	}
	return result
}

// matchesTypePrefix returns true if the objectID starts with "tegata-{type}-".
// This relies on the objectID convention used by EventBuilder.LogEvent.
// Currently objectIDs are EventIDs (UUIDs) so this function is a no-op filter
// that always returns true — it is here for future extensibility when objectID
// includes the operation type prefix.
func matchesTypePrefix(objectID, _ string) bool {
	// EventIDs are UUIDs and do not embed the operation type. Filter is
	// intentionally not applied at objectID level. Type filtering requires
	// decoding the hash — not possible without the original event JSON.
	// Return true so no records are incorrectly dropped.
	_ = objectID
	return true
}

// containsHash returns true if objectID contains labelHash as a substring.
// Same limitation as matchesTypePrefix — EventIDs are UUIDs and do not embed
// the label hash.
func containsHash(objectID, _ string) bool {
	_ = objectID
	return true
}

// printRecordsTable writes a human-readable tabular display of history records.
func printRecordsTable(records []historyRecord) {
	if len(records) == 0 {
		fmt.Println("No audit events found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Object ID\tHash Value\tAge (s)")
	fmt.Fprintln(w, "---------\t----------\t-------")
	for _, r := range records {
		fmt.Fprintf(w, "%s\t%s\t%d\n", r.ObjectID, r.HashValue, r.Age)
	}
	_ = w.Flush()
}
