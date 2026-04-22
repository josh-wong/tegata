package audit

import (
	"context"
	"fmt"
	"sort"

	tegerrors "github.com/josh-wong/tegata/internal/errors"
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
// If the collection does not exist (e.g. after a wipe), returns empty results.
func FetchHistory(ctx context.Context, client Client, entityID string) (*FetchHistoryResult, error) {
	collectionID := CollectionID(entityID)
	eventIDs, err := client.CollectionGet(ctx, collectionID)
	if err != nil {
		// If the collection doesn't exist, that's not an error — just return empty results.
		// This happens after a wipe when the collection is deleted along with its events.
		if tegerrors.Is(err, errCollectionNotFound) {
			return &FetchHistoryResult{Records: []HistoryRecord{}}, nil
		}
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

	// Sort records by timestamp in descending order (newest first).
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp
	})

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
	Skipped     int      // events without vault hash (pre-existing, per D-09)
	Faults      []string // per-event error descriptions
	ErrorDetail string   // summary when !Valid
}

// VerifyByLabelHash validates the integrity of audit events for the given
// entity that match labelHash. Only events whose label_hash metadata field
// equals labelHash are validated; all others are ignored. If no matching
// events exist, returns valid with zero events. Events without a vault hash
// entry are skipped (pre-existing events, per D-09).
func VerifyByLabelHash(ctx context.Context, client Client, entityID, labelHash string, vaultHashes map[string]string) (*VerifyResult, error) {
	history, err := FetchHistory(ctx, client, entityID)
	if err != nil {
		return nil, err
	}

	var matchingIDs []string
	for _, r := range history.Records {
		if r.LabelHash == labelHash {
			matchingIDs = append(matchingIDs, r.ObjectID)
		}
	}

	if len(matchingIDs) == 0 {
		return &VerifyResult{Valid: true, EventCount: 0}, nil
	}

	var faults []string
	var verified int
	var skipped int
	for _, id := range matchingIDs {
		expectedHash, ok := vaultHashes[id]
		if !ok {
			skipped++
			continue
		}
		verified++
		result, err := client.Validate(ctx, id, expectedHash)
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
		EventCount: verified,
		Skipped:    skipped,
		Faults:     faults,
	}
	if !vr.Valid {
		vr.ErrorDetail = fmt.Sprintf("%d of %d events failed", len(faults), verified)
	}
	return vr, nil
}

// VerifyAll validates the integrity of all audit events for the given entity.
// It fetches the entity's collection, then validates each event individually
// using the caller-supplied vaultHashes map. Events without a vault hash entry
// are skipped (pre-existing events, per D-09). If the collection does not exist
// (e.g. after a wipe), returns valid with zero events.
func VerifyAll(ctx context.Context, client Client, entityID string, vaultHashes map[string]string) (*VerifyResult, error) {
	collectionID := CollectionID(entityID)
	eventIDs, err := client.CollectionGet(ctx, collectionID)
	if err != nil {
		// If the collection doesn't exist, that's not an error — just return valid with zero events.
		// This happens after a wipe when the collection is deleted along with its events.
		if tegerrors.Is(err, errCollectionNotFound) {
			return &VerifyResult{Valid: true, EventCount: 0}, nil
		}
		return nil, err
	}

	if len(eventIDs) == 0 {
		return &VerifyResult{Valid: true, EventCount: 0}, nil
	}

	var faults []string
	var verified int
	var skipped int
	for _, id := range eventIDs {
		expectedHash, ok := vaultHashes[id]
		if !ok {
			skipped++
			continue
		}
		verified++
		result, err := client.Validate(ctx, id, expectedHash)
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
		EventCount: verified,
		Skipped:    skipped,
		Faults:     faults,
	}
	if !vr.Valid {
		vr.ErrorDetail = fmt.Sprintf("%d of %d events failed", len(faults), verified)
	}
	return vr, nil
}
