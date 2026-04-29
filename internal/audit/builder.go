package audit

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
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
	client        Submitter      // nil when audit is disabled
	queue         *Queue
	queuePath     string
	queueKey      []byte         // 32-byte key; EventBuilder does NOT own lifecycle
	disabled      bool           // true when client == nil
	lastHash      string         // SHA-256 of the last successfully submitted event JSON
	submitTimeout time.Duration  // timeout for submit operations (default 3s)
	OnHashStored  func(eventID, hashValue string) // called after successful Submit to store hash in vault (D-15)
	// LogVaultLockOnClose, when true, emits a vault-lock event just before the
	// queue is saved and the builder is torn down. Set by setupAuditBuilder so
	// CLI commands (which have no explicit quit path) automatically log the
	// closing of a session without any per-command boilerplate.
	// Leave false in the TUI: the TUI logs vault-lock explicitly before calling
	// Close(), so setting this flag would produce a duplicate event.
	LogVaultLockOnClose bool
}

// callSafe invokes fn, recovering any panic and logging it. Used for best-effort
// callbacks that must not crash the caller (D-14).
func callSafe(fn func(), panicMsg string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error(panicMsg, "err", r)
		}
	}()
	fn()
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
		client:        client,
		submitTimeout: 3 * time.Second,
		queue:     q,
		queuePath: queuePath,
		queueKey:  queueKey,
	}, nil
}

// NewEventBuilderMemQueue creates an EventBuilder backed by an in-memory
// queue (no disk persistence). Use when the vault passphrase is not available
// to derive the persistent queue key — for example, after audit server setup
// within the same session. Events that fail to submit are held in memory only
// and are lost if the builder is closed or the process exits.
func NewEventBuilderMemQueue(client Submitter) (*EventBuilder, error) {
	if client == nil {
		return &EventBuilder{disabled: true}, nil
	}
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, fmt.Errorf("generating in-memory queue key: %w", err)
	}
	q, err := NewQueue(key[:], 1000)
	for i := range key {
		key[i] = 0
	}
	if err != nil {
		return nil, fmt.Errorf("creating in-memory queue: %w", err)
	}
	return &EventBuilder{
		client:        client,
		submitTimeout: 3 * time.Second,
		queue:         q,
		// queuePath intentionally empty — Save calls fail silently.
	}, nil
}

// submitResult is the result of an asynchronous flush+submit attempt.
type submitResult struct {
	err      error
	lastHash string // non-empty only on full success
}

// LogEvent records an authentication operation in the audit log. It builds an
// AuthEvent from the provided fields, then attempts to flush any queued events
// and submit the new event to the ledger within a 3-second deadline. On any
// network error the new event is appended to the offline queue and nil is
// returned — auth operations must succeed regardless of ledger availability.
//
// The flush+submit attempt is run in a goroutine so that a non-cooperative
// Submitter (one that ignores context cancellation) cannot block the caller
// longer than 3 seconds plus negligible goroutine scheduling overhead.
//
// The goroutine works exclusively from a snapshot of the queue entries taken
// before it is spawned, so it never accesses b.queue concurrently with the
// main goroutine. On timeout the main goroutine safely appends to b.queue
// while the spawned goroutine continues only against its local snapshot and
// the Submitter. resultCh is buffered (cap 1) so the goroutine can always
// send its result even if the deadline branch already won the select; that
// result is intentionally discarded — the event has been queued for the next
// LogEvent call.
//
// The PrevHash of each event is the SHA-256 of the previous successfully
// submitted event's JSON, forming a local hash chain for integrity verification.
func (b *EventBuilder) LogEvent(opType, label, service, host string, success bool) error {
	if b.disabled {
		return nil
	}

	evt := NewAuthEvent(opType, label, service, host, success, b.lastHash)

	// Snapshot queued entries before spawning the goroutine. The goroutine
	// submits from this local slice and never touches b.queue.
	snapshot := b.queue.Entries()

	timeout := b.submitTimeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan submitResult, 1)

	go func() {
		// Submit any previously queued events from the snapshot and store
		// their hashes via OnHashStored — queued events failed to submit
		// originally so their hashes were never persisted (D-15).
		for _, e := range snapshot {
			hashValue, err := b.client.Submit(ctx, e)
			if err != nil {
				resultCh <- submitResult{err: err}
				return
			}
			if b.OnHashStored != nil && hashValue != "" {
				id, hv := e.Event.EventID, hashValue
				callSafe(func() { b.OnHashStored(id, hv) }, "audit OnHashStored panic (queued event)")
			}
		}

		entry := QueueEntry{Event: evt}
		hashValue, err := b.client.Submit(ctx, entry)
		if err != nil {
			resultCh <- submitResult{err: err}
			return
		}

		// Call OnHashStored callback for the new event. Best-effort (D-14).
		if b.OnHashStored != nil {
			id, hv := entry.Event.EventID, hashValue
			callSafe(func() { b.OnHashStored(id, hv) }, "audit OnHashStored panic")
		}

		// Success: compute the hash of the newly submitted event for chain.
		// AuthEvent contains only string, bool, and time.Time fields, so
		// Marshal failure is a programming error — panic like EntryHash does.
		eventJSON, err := json.Marshal(evt)
		if err != nil {
			panic(fmt.Sprintf("audit: LogEvent: json.Marshal failed: %v", err))
		}
		sum := sha256.Sum256(eventJSON)
		for i := range eventJSON {
			eventJSON[i] = 0
		}
		resultCh <- submitResult{lastHash: hex.EncodeToString(sum[:])}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil {
			slog.Error("audit submit error", "err", res.err)
			_ = b.queue.Append(evt)
			_ = b.queue.Save(b.queuePath)
			return nil
		}
		if res.lastHash == "" {
			slog.Error("audit submit returned empty hash")
			_ = b.queue.Append(evt)
			_ = b.queue.Save(b.queuePath)
			return nil
		}
		// Full success: drop the snapshot entries that were submitted, then save.
		b.queue.DropFront(len(snapshot))
		b.lastHash = res.lastHash
		_ = b.queue.Save(b.queuePath)
		return nil
	case <-ctx.Done():
		slog.Error("audit submit timed out", "timeout", timeout)
		_ = b.queue.Append(evt)
		_ = b.queue.Save(b.queuePath)
		return nil
	}
}

// Close saves the offline queue to disk. Should be called in a deferred
// function in each auth command after LogEvent. When LogVaultLockOnClose is
// true, a vault-lock event is emitted first so that CLI commands (which have
// no explicit quit path) produce a matched unlock/lock pair in the audit log.
func (b *EventBuilder) Close() error {
	if b.disabled {
		return nil
	}
	if b.LogVaultLockOnClose {
		_ = b.LogEvent("vault-lock", "", "", Hostname(), true)
	}
	return b.queue.Save(b.queuePath)
}
