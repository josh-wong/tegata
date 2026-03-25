package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// mockSubmitter is a test double for the Submitter interface. It records calls
// and can be configured to return a fixed error.
type mockSubmitter struct {
	mu      sync.Mutex
	calls   []QueueEntry
	failErr error
	delay   time.Duration // if non-zero, Sleep before returning
}

func (m *mockSubmitter) Submit(_ context.Context, entry QueueEntry) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failErr != nil {
		return m.failErr
	}
	m.calls = append(m.calls, entry)
	return nil
}

func (m *mockSubmitter) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// testKey is a 32-byte AES-256 key used only in tests.
var testKey = []byte("test-key-12345678901234567890123") // exactly 32 bytes

// newTestBuilder creates an EventBuilder backed by a temp queue file.
func newTestBuilder(t *testing.T, sub Submitter) (*EventBuilder, string) {
	t.Helper()
	dir := t.TempDir()
	queuePath := dir + "/queue.tegata"
	b, err := NewEventBuilder(sub, queuePath, testKey, 100)
	if err != nil {
		t.Fatalf("NewEventBuilder: %v", err)
	}
	return b, queuePath
}

// TestEventBuilder_SuccessfulSubmit verifies that when Submitter succeeds,
// LogEvent calls Submit once and the queue remains empty.
func TestEventBuilder_SuccessfulSubmit(t *testing.T) {
	sub := &mockSubmitter{}
	b, _ := newTestBuilder(t, sub)

	if err := b.LogEvent("totp", "github", "GitHub", "testhost", true); err != nil {
		t.Fatalf("LogEvent returned error: %v", err)
	}

	if sub.CallCount() != 1 {
		t.Errorf("Submit call count: got %d, want 1", sub.CallCount())
	}
	if b.queue.Len() != 0 {
		t.Errorf("queue length after success: got %d, want 0", b.queue.Len())
	}
}

// TestEventBuilder_SubmitFailure verifies that when Submitter returns an error,
// the event is appended to the queue (not lost).
func TestEventBuilder_SubmitFailure(t *testing.T) {
	sub := &mockSubmitter{failErr: ErrFlushFailed}
	b, _ := newTestBuilder(t, sub)

	if err := b.LogEvent("totp", "github", "GitHub", "testhost", true); err != nil {
		t.Fatalf("LogEvent must not return error even on Submitter failure, got: %v", err)
	}

	if b.queue.Len() != 1 {
		t.Errorf("queue length after submit failure: got %d, want 1", b.queue.Len())
	}
}

// TestEventBuilder_FlushBeforeSubmit verifies that when the queue is non-empty,
// LogEvent flushes it first. After a successful Flush+Submit, the queue empties.
func TestEventBuilder_FlushBeforeSubmit(t *testing.T) {
	// Start with a sub that fails so we can prime the queue.
	failSub := &mockSubmitter{failErr: ErrFlushFailed}
	b, _ := newTestBuilder(t, failSub)

	// Append one event directly to the queue to simulate a previously queued event.
	evt0 := NewAuthEvent("totp", "prior", "PriorSvc", "host", true, "")
	if err := b.queue.Append(evt0); err != nil {
		t.Fatalf("queue.Append: %v", err)
	}
	// Queue now has 1 entry; replace submitter with a succeeding one.
	succeedSub := &mockSubmitter{}
	b.client = succeedSub

	// LogEvent should flush the queued event then submit the new one.
	if err := b.LogEvent("hotp", "aws", "AWS", "host", true); err != nil {
		t.Fatalf("LogEvent: %v", err)
	}

	// succeedSub received: 1 flush call + 1 new submit call = 2 total.
	if succeedSub.CallCount() != 2 {
		t.Errorf("Submit calls: got %d, want 2 (1 flush + 1 new)", succeedSub.CallCount())
	}
	if b.queue.Len() != 0 {
		t.Errorf("queue length after flush+submit: got %d, want 0", b.queue.Len())
	}
}

// TestEventBuilder_FlushFailureQueuesNewEvent verifies that when Flush fails
// (queue non-empty + Submitter fails), the new event is also queued (queue grows
// by 1, not replaced).
func TestEventBuilder_FlushFailureQueuesNewEvent(t *testing.T) {
	sub := &mockSubmitter{failErr: ErrFlushFailed}
	b, _ := newTestBuilder(t, sub)

	// Prime queue with one event.
	evt0 := NewAuthEvent("totp", "prior", "PriorSvc", "host", true, "")
	if err := b.queue.Append(evt0); err != nil {
		t.Fatalf("queue.Append: %v", err)
	}

	if err := b.LogEvent("totp", "github", "GitHub", "host", true); err != nil {
		t.Fatalf("LogEvent must not return error: %v", err)
	}

	// Queue should have grown from 1 to 2.
	if b.queue.Len() != 2 {
		t.Errorf("queue length: got %d, want 2", b.queue.Len())
	}
}

// TestEventBuilder_Disabled verifies that when NewEventBuilder is called with
// nil client, LogEvent is a no-op: returns nil and does not modify the queue.
func TestEventBuilder_Disabled(t *testing.T) {
	b, err := NewEventBuilder(nil, "", nil, 0)
	if err != nil {
		t.Fatalf("NewEventBuilder(nil): %v", err)
	}

	err = b.LogEvent("totp", "github", "GitHub", "host", true)
	if err != nil {
		t.Errorf("LogEvent on disabled builder returned error: %v", err)
	}
	// No panic, no side effects.
}

// TestEventBuilder_DeadlineEnforced verifies that LogEvent returns within the
// configured timeout even when the Submitter takes much longer to respond.
func TestEventBuilder_DeadlineEnforced(t *testing.T) {
	slowSub := &mockSubmitter{delay: 5 * time.Second}
	b, _ := newTestBuilder(t, slowSub)
	b.submitTimeout = 500 * time.Millisecond // short timeout for test speed

	start := time.Now()
	if err := b.LogEvent("totp", "github", "GitHub", "host", true); err != nil {
		t.Fatalf("LogEvent returned error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed >= 1*time.Second {
		t.Errorf("LogEvent took %v, want < 1s with 500ms timeout", elapsed)
	}
}

// TestEventBuilder_PrevHashChain verifies that two sequential LogEvent calls
// produce the correct PrevHash chain: the second event's PrevHash is the
// SHA-256 of the first event's JSON.
func TestEventBuilder_PrevHashChain(t *testing.T) {
	var submitted []QueueEntry
	sub := &mockSubmitter{}

	b, _ := newTestBuilder(t, sub)

	if err := b.LogEvent("totp", "github", "GitHub", "host", true); err != nil {
		t.Fatalf("LogEvent 1: %v", err)
	}
	if err := b.LogEvent("hotp", "aws", "AWS", "host", true); err != nil {
		t.Fatalf("LogEvent 2: %v", err)
	}

	sub.mu.Lock()
	submitted = append(submitted, sub.calls...)
	sub.mu.Unlock()

	if len(submitted) != 2 {
		t.Fatalf("expected 2 submitted entries, got %d", len(submitted))
	}

	// The second event's PrevHash must equal SHA-256 of the first event's JSON.
	firstJSON, err := json.Marshal(submitted[0].Event)
	if err != nil {
		t.Fatalf("marshal first event: %v", err)
	}
	sum := sha256.Sum256(firstJSON)
	expectedPrevHash := hex.EncodeToString(sum[:])

	secondEvent := submitted[1].Event
	if secondEvent.PrevHash != expectedPrevHash {
		t.Errorf("second event PrevHash: got %q, want %q", secondEvent.PrevHash, expectedPrevHash)
	}
}

// TestEventBuilder_Close verifies that Close saves the queue file when entries
// are present, allowing LoadQueue to restore them.
func TestEventBuilder_Close(t *testing.T) {
	sub := &mockSubmitter{failErr: ErrFlushFailed}
	b, queuePath := newTestBuilder(t, sub)

	// Append an event to the queue via LogEvent (submit will fail → queued).
	if err := b.LogEvent("totp", "github", "GitHub", "host", true); err != nil {
		t.Fatalf("LogEvent: %v", err)
	}

	if b.queue.Len() != 1 {
		t.Fatalf("queue length before Close: %d, want 1", b.queue.Len())
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reload and verify the entry persisted.
	q2, err := LoadQueue(queuePath, testKey)
	if err != nil {
		t.Fatalf("LoadQueue after Close: %v", err)
	}
	if q2.Len() != 1 {
		t.Errorf("reloaded queue length: got %d, want 1", q2.Len())
	}
}
