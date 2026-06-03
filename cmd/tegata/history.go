package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newHistoryCmd() *cobra.Command {
	var (
		from    string
		to      string
		opType  string
		sortBy  string
		order   string
		limit   int
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "history",
		Short:   i18n.T("cmd.history.short"),
		Long:    i18n.T("cmd.history.long"),
		Example: i18n.T("cmd.history.example"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			vaultPath, err := resolveVaultPath(cmd)
			if err != nil {
				return err
			}

			cfg, err := config.Load(vaultDir(vaultPath))
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.history.error.loadConfig"), err)
			}

			if !cfg.Audit.Enabled {
				printAuditNotEnabledHint(os.Stderr)
				return nil
			}

			validSortCols := map[string]bool{"operation": true, "label": true, "timestamp": true, "hash": true}
			if sortBy != "" && !validSortCols[sortBy] {
				return fmt.Errorf("%s: %w",
					i18n.Tf("cmd.history.error.invalidSort",
						map[string]any{"Value": sortBy}), tegerrors.ErrInvalidInput)
			}
			if order != "" && order != "asc" && order != "desc" {
				return fmt.Errorf("%s: %w",
					i18n.Tf("cmd.history.error.invalidOrder",
						map[string]any{"Value": order}), tegerrors.ErrInvalidInput)
			}

			passphrase, err := promptPassphrase(i18n.T("cmd.verifyRecovery.prompt.passphrase"))
			if err != nil {
				return err
			}
			mgr, err := openAndUnlock(vaultPath, passphrase)
			zeroBytes(passphrase)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.history.error.unlockVault"), err)
			}
			defer mgr.Close()

			secretFromVault := mgr.GetSecret("audit.secret_key")
			if secretFromVault != "" {
				cfg.Audit.SecretKey = secretFromVault
			}
			defer func() { cfg.Audit.SecretKey = "" }()

			creds := mgr.ListCredentials()
			labels := make([]string, len(creds))
			for i, c := range creds {
				labels[i] = c.Label
			}
			labelMap := audit.BuildLabelMap(labels)
			deletedMap := mgr.DeletedLabels()

			if err := audit.CheckLedgerAvailability(cfg.Audit); err != nil {
				return err
			}
			client, err := audit.NewClientFromConfig(cfg.Audit)
			if err != nil {
				return fmt.Errorf("%w: %s", tegerrors.ErrNetworkFailed, err)
			}
			defer func() { _ = client.Close() }()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := audit.FetchHistory(ctx, client, cfg.Audit.EntityID)
			if err != nil {
				return err
			}
			if result.Warning != "" {
				fmt.Fprintf(os.Stderr, "%s", i18n.Tf("cmd.history.warn", map[string]any{"Msg": result.Warning}))
			}

			records := make([]historyRecord, len(result.Records))
			for i, r := range result.Records {
				records[i] = historyRecord{
					ObjectID:  r.ObjectID,
					Operation: r.Operation,
					LabelHash: r.LabelHash,
					Timestamp: r.Timestamp,
					HashValue: r.HashValue,
				}
			}

			var fromTime, toTime time.Time
			if from != "" {
				fromTime, err = time.ParseInLocation("2006-01-02", from, time.Local)
				if err != nil {
					return fmt.Errorf("%s: %w",
						i18n.Tf("cmd.history.error.invalidFrom",
							map[string]any{"Value": from}), tegerrors.ErrInvalidInput)
				}
			}
			if to != "" {
				toTime, err = time.ParseInLocation("2006-01-02", to, time.Local)
				if err != nil {
					return fmt.Errorf("%s: %w",
						i18n.Tf("cmd.history.error.invalidTo",
							map[string]any{"Value": to}), tegerrors.ErrInvalidInput)
				}
				toTime = toTime.Add(24*time.Hour - time.Nanosecond)
			}

			filtered := filterRecords(records, fromTime, toTime, opType)
			sortRecords(filtered, labelMap, deletedMap, sortBy, order)

			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			printRecordsTable(filtered, labelMap, deletedMap)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", i18n.T("cmd.history.flag.from"))
	cmd.Flags().StringVar(&to, "to", "", i18n.T("cmd.history.flag.to"))
	cmd.Flags().StringVar(&opType, "type", "", i18n.T("cmd.history.flag.type"))
	cmd.Flags().StringVar(&sortBy, "sort", "", i18n.T("cmd.history.flag.sort"))
	cmd.Flags().StringVar(&order, "order", "", i18n.T("cmd.history.flag.order"))
	cmd.Flags().IntVar(&limit, "limit", 0, i18n.T("cmd.history.flag.limit"))
	cmd.Flags().BoolVar(&jsonOut, "json", false, i18n.T("cmd.history.flag.json"))

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

// filterRecords applies date and operation type filtering using the metadata
// timestamp and operation fields.
func filterRecords(records []historyRecord, from, to time.Time, opType string) []historyRecord {
	if from.IsZero() && to.IsZero() && opType == "" {
		return records
	}
	var filtered []historyRecord
	for _, r := range records {
		if opType != "" && !strings.EqualFold(r.Operation, opType) {
			continue
		}
		t := time.Unix(r.Timestamp, 0).Local()
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

// sortRecords sorts records in-place by the given column and order. When sortBy
// is empty the records remain in their default order (timestamp descending, set
// by FetchHistory). labelMap and deletedMap are used to resolve label hashes for
// label-column sorting.
func sortRecords(records []historyRecord, labelMap, deletedMap map[string]string, sortBy, order string) {
	if sortBy == "" {
		if order == "asc" {
			for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
				records[i], records[j] = records[j], records[i]
			}
		}
		return
	}

	sort.SliceStable(records, func(i, j int) bool {
		switch sortBy {
		case "operation":
			a := audit.FormatOperation(records[i].Operation)
			b := audit.FormatOperation(records[j].Operation)
			if order == "desc" {
				return a > b
			}
			return a < b
		case "label":
			a := audit.ResolveLabelWithDeleted(records[i].LabelHash, labelMap, deletedMap)
			b := audit.ResolveLabelWithDeleted(records[j].LabelHash, labelMap, deletedMap)
			if order == "desc" {
				return a > b
			}
			return a < b
		case "hash":
			if order == "desc" {
				return records[i].HashValue > records[j].HashValue
			}
			return records[i].HashValue < records[j].HashValue
		default: // "timestamp"
			if order == "asc" {
				return records[i].Timestamp < records[j].Timestamp
			}
			return records[i].Timestamp > records[j].Timestamp
		}
	})
}

// printRecordsTable writes a human-readable tabular display of history records.
func printRecordsTable(records []historyRecord, labelMap, deletedMap map[string]string) {
	if len(records) == 0 {
		fmt.Println(i18n.T("cmd.history.empty"))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, i18n.T("cmd.history.header"))
	_, _ = fmt.Fprintln(w, i18n.T("cmd.history.headerSep"))
	for _, r := range records {
		label := audit.ResolveLabelWithDeleted(r.LabelHash, labelMap, deletedMap)
		op := audit.FormatOperation(r.Operation)
		ts := time.Unix(r.Timestamp, 0).Local().Format("2006-01-02 15:04:05")
		hash := r.HashValue
		if len(hash) > 16 {
			hash = hash[:16] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", op, label, ts, hash)
	}
	_ = w.Flush()
}
