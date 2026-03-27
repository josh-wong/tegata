package audit

import (
	"context"
	"fmt"
)

// HistoryRecord is the common shape for a single audit history entry,
// used across CLI, TUI, and GUI.
type HistoryRecord struct {
	ObjectID  string `json:"object_id"`
	Operation string `json:"operation"`
	LabelHash string `json:"label_hash"`
	Timestamp int64  `json:"timestamp"`
	HashValue string `json:"hash_value"`
}

// FetchHistoryResult holds the records and any non-fatal warning.
type FetchHistoryResult struct {
	Records []HistoryRecord
	Warning string // non-empty if some events could not be fetched
}

// FetchHistory retrieves all audit history records for the given entity from
// the ledger. It fetches the entity's collection, then gets each event
// individually. Events that fail to fetch are skipped and counted in Warning.
func FetchHistory(ctx context.Context, client Client, entityID string) (*FetchHistoryResult, error) {
	collectionID := CollectionID(entityID)
	eventIDs, err := client.CollectionGet(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	var records []HistoryRecord
	var skipped int
	for _, id := range eventIDs {
		evts, err := client.Get(ctx, id)
		if err != nil {
			skipped++
			continue
		}
		if len(evts) == 0 {
			continue
		}
		r := evts[0]
		records = append(records, HistoryRecord{
			ObjectID:  r.ObjectID,
			Operation: MetadataString(r.Metadata, "operation"),
			LabelHash: MetadataString(r.Metadata, "label_hash"),
			Timestamp: MetadataInt64(r.Metadata, "timestamp"),
			HashValue: r.HashValue,
		})
	}

	result := &FetchHistoryResult{Records: records}
	if skipped > 0 {
		result.Warning = fmt.Sprintf("%d of %d events could not be fetched", skipped, len(eventIDs))
	}
	return result, nil
}

// VerifyResult holds the outcome of verifying all audit events.
type VerifyResult struct {
	Valid       bool
	EventCount  int
	Faults      []string // per-event error descriptions
	ErrorDetail string   // summary when !Valid
}

// VerifyAll validates the integrity of all audit events for the given entity.
// It fetches the entity's collection, then validates each event individually.
func VerifyAll(ctx context.Context, client Client, entityID string) (*VerifyResult, error) {
	collectionID := CollectionID(entityID)
	eventIDs, err := client.CollectionGet(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	if len(eventIDs) == 0 {
		return &VerifyResult{Valid: true, EventCount: 0}, nil
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

	vr := &VerifyResult{
		Valid:      len(faults) == 0,
		EventCount: len(eventIDs),
		Faults:     faults,
	}
	if !vr.Valid {
		vr.ErrorDetail = fmt.Sprintf("%d of %d events failed", len(faults), len(eventIDs))
	}
	return vr, nil
}
