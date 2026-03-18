package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// EventBuilder coordinates the flush-before-submit pattern for audit events.
// It wraps a Submitter (the live ledger client) and a Queue (the offline
// fallback). Auth commands call LogEvent after each successful operation.
//
// When LogEvent is called:
//  1. If disabled (nil client), return nil immediately.
//  2. Flush any queued events to the ledger with a 500ms deadline.
//  3. If flush succeeds, submit the new event. On any error, append to queue.
//
// Auth commands are never blocked or returned errors due to audit failures
// — audit errors are silently absorbed, keeping the event in the queue.
type EventBuilder struct {
	client    Submitter // nil when audit is disabled
	queue     *Queue
	queuePath string
	queueKey  []byte // 32-byte key; EventBuilder does NOT own lifecycle
	disabled  bool   // true when client == nil
	lastHash  string // SHA-256 of the last successfully submitted event JSON
}

// NewEventBuilder creates an EventBuilder. If client is nil the builder is
// disabled and LogEvent becomes a no-op. When client is non-nil, the offline
// queue is loaded from queuePath (creating an empty queue if the file does not
// exist). queueKey must be 32 bytes.
func NewEventBuilder(client Submitter, queuePath string, queueKey []byte, maxLen int) (*EventBuilder, error) {
	if client == nil {
		return &EventBuilder{disabled: true}, nil
	}

	q, err := LoadQueue(queuePath, queueKey)
	if err != nil {
		return nil, err
	}

	return &EventBuilder{
		client:    client,
		queue:     q,
		queuePath: queuePath,
		queueKey:  queueKey,
	}, nil
}

// submitResult is the result of an asynchronous flush+submit attempt.
type submitResult struct {
	err      error
	lastHash string // non-empty only on full success
}

// LogEvent records an authentication operation in the audit log. It builds an
// AuthEvent from the provided fields, then attempts to flush any queued events
// and submit the new event to the ledger within a 500ms deadline. On any
// network error the new event is appended to the offline queue and nil is
// returned — auth operations must succeed regardless of ledger availability.
//
// The flush+submit attempt is run in a goroutine so that a non-cooperative
// Submitter (one that ignores context cancellation) cannot block the caller
// longer than 500ms plus negligible goroutine scheduling overhead.
//
// The PrevHash of each event is the SHA-256 of the previous successfully
// submitted event's JSON, forming a local hash chain for integrity verification.
func (b *EventBuilder) LogEvent(opType, label, service, host string, success bool) error {
	if b.disabled {
		return nil
	}

	evt := NewAuthEvent(opType, label, service, host, success, b.lastHash)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	resultCh := make(chan submitResult, 1)
	// Take a snapshot of the fields the goroutine needs to avoid a data race
	// with the queue (b.queue is not goroutine-safe).
	qLen := b.queue.Len()

	go func() {
		// Flush any queued events first.
		if qLen > 0 {
			if err := b.queue.Flush(ctx, b.client); err != nil {
				resultCh <- submitResult{err: err}
				return
			}
		}

		entry := QueueEntry{Event: evt}
		if err := b.client.Submit(ctx, entry); err != nil {
			resultCh <- submitResult{err: err}
			return
		}

		// Success: compute the hash of the newly submitted event.
		eventJSON, _ := json.Marshal(evt)
		sum := sha256.Sum256(eventJSON)
		resultCh <- submitResult{lastHash: hex.EncodeToString(sum[:])}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil || res.lastHash == "" {
			// Network/submit failure — append to queue and persist.
			_ = b.queue.Append(evt)
			_ = b.queue.Save(b.queuePath)
			return nil
		}
		b.lastHash = res.lastHash
		_ = b.queue.Save(b.queuePath)
		return nil
	case <-ctx.Done():
		// Deadline exceeded — queue the event and return immediately.
		_ = b.queue.Append(evt)
		_ = b.queue.Save(b.queuePath)
		return nil
	}
}

// Close saves the offline queue to disk. Should be called in a deferred
// function in each auth command after LogEvent.
func (b *EventBuilder) Close() error {
	if b.disabled {
		return nil
	}
	return b.queue.Save(b.queuePath)
}
