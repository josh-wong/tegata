package audit_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
)

// makeKey generates a random 32-byte key for testing.
func makeKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	return key
}

// makeEvent creates a test AuthEvent.
func makeEvent(opType string) audit.AuthEvent {
	return audit.NewAuthEvent(opType, "label", "service", "host", true, "")
}

// mockSubmitter is a test double for the Submitter interface.
type mockSubmitter struct {
	calls  []audit.QueueEntry
	errOn  int // return error on this call (1-indexed, 0 = never)
	retErr error
}

func (m *mockSubmitter) Submit(_ context.Context, entry audit.QueueEntry) (string, error) {
	m.calls = append(m.calls, entry)
	if m.errOn > 0 && len(m.calls) == m.errOn {
		return "", m.retErr
	}
	return "fakehash", nil
}

func TestQueue_NewQueueEmpty(t *testing.T) {
	q, err := audit.NewQueue(makeKey(t), 100)
	if err != nil {
		t.Fatalf("NewQueue returned error: %v", err)
	}
	if q.Len() != 0 {
		t.Errorf("NewQueue Len = %d, want 0", q.Len())
	}
}

func TestQueue_NewQueueBadKeyLength(t *testing.T) {
	_, err := audit.NewQueue([]byte("short"), 100)
	if err == nil {
		t.Fatal("NewQueue should return error for key shorter than 32 bytes")
	}
}

func TestQueue_AppendIncreasesLen(t *testing.T) {
	q, err := audit.NewQueue(makeKey(t), 100)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	if err := q.Append(makeEvent("totp")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if q.Len() != 1 {
		t.Errorf("Len = %d after one Append, want 1", q.Len())
	}
}

func TestQueue_HashChain_SecondEntryLinksToPrev(t *testing.T) {
	q, err := audit.NewQueue(makeKey(t), 100)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	if err := q.Append(makeEvent("totp")); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	if err := q.Append(makeEvent("hotp")); err != nil {
		t.Fatalf("second Append: %v", err)
	}

	entries := q.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Second entry's PrevHash must equal SHA-256 of the first entry's canonical JSON.
	first := entries[0]
	expectedPrev := audit.EntryHash(first)
	if entries[1].PrevHash != expectedPrev {
		t.Errorf("second entry PrevHash = %q, want %q", entries[1].PrevHash, expectedPrev)
	}
}

func TestQueue_VerifyChain_IntactChain(t *testing.T) {
	q, _ := audit.NewQueue(makeKey(t), 100)
	_ = q.Append(makeEvent("totp"))
	_ = q.Append(makeEvent("hotp"))
	_ = q.Append(makeEvent("static"))

	if err := q.VerifyChain(); err != nil {
		t.Errorf("VerifyChain on intact chain returned error: %v", err)
	}
}

func TestQueue_VerifyChain_CorruptedPrevHash(t *testing.T) {
	q, _ := audit.NewQueue(makeKey(t), 100)
	_ = q.Append(makeEvent("totp"))
	_ = q.Append(makeEvent("hotp"))

	// Corrupt the second entry's PrevHash.
	q.CorruptEntry(1, "000000000000000000000000000000000000000000000000000000000000dead")

	err := q.VerifyChain()
	if err == nil {
		t.Fatal("VerifyChain should return error for corrupted PrevHash")
	}
}

func TestQueue_Flush_SuccessCallsSubmitter(t *testing.T) {
	q, _ := audit.NewQueue(makeKey(t), 100)
	_ = q.Append(makeEvent("totp"))
	_ = q.Append(makeEvent("hotp"))

	mock := &mockSubmitter{}
	if err := q.Flush(context.Background(), mock); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	if len(mock.calls) != 2 {
		t.Errorf("Submitter called %d times, want 2", len(mock.calls))
	}
	if q.Len() != 0 {
		t.Errorf("Len = %d after successful Flush, want 0", q.Len())
	}
}

func TestQueue_Flush_ErrorLeavesQueueUnchanged(t *testing.T) {
	q, _ := audit.NewQueue(makeKey(t), 100)
	_ = q.Append(makeEvent("totp"))
	_ = q.Append(makeEvent("hotp"))

	mock := &mockSubmitter{errOn: 1, retErr: audit.ErrFlushFailed}
	err := q.Flush(context.Background(), mock)
	if err == nil {
		t.Fatal("Flush should return error when submitter fails")
	}

	if q.Len() != 2 {
		t.Errorf("Len = %d after failed Flush, want 2 (queue unchanged)", q.Len())
	}
}

func TestQueue_Flush_OrderPreserved(t *testing.T) {
	q, _ := audit.NewQueue(makeKey(t), 100)
	_ = q.Append(audit.NewAuthEvent("totp", "first", "svc", "host", true, ""))
	_ = q.Append(audit.NewAuthEvent("hotp", "second", "svc", "host", true, ""))
	_ = q.Append(audit.NewAuthEvent("static", "third", "svc", "host", true, ""))

	mock := &mockSubmitter{}
	_ = q.Flush(context.Background(), mock)

	if len(mock.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(mock.calls))
	}
	for i, entry := range mock.calls {
		if entry.Event.OperationType == "" {
			t.Errorf("call %d has empty OperationType", i)
		}
	}
}

func TestQueue_SaveLoad_RoundTrip(t *testing.T) {
	key := makeKey(t)
	q, _ := audit.NewQueue(key, 100)
	_ = q.Append(makeEvent("totp"))
	_ = q.Append(makeEvent("hotp"))

	path := filepath.Join(t.TempDir(), "test.queue")
	if err := q.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	q2, err := audit.LoadQueue(path, key)
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}

	if q2.Len() != q.Len() {
		t.Errorf("Len after round-trip = %d, want %d", q2.Len(), q.Len())
	}

	orig := q.Entries()
	loaded := q2.Entries()
	for i := range orig {
		if orig[i].Event.EventID != loaded[i].Event.EventID {
			t.Errorf("entry[%d] EventID = %q, want %q", i, loaded[i].Event.EventID, orig[i].Event.EventID)
		}
	}
}

func TestQueue_LoadQueue_WrongKey(t *testing.T) {
	key := makeKey(t)
	q, _ := audit.NewQueue(key, 100)
	_ = q.Append(makeEvent("totp"))

	path := filepath.Join(t.TempDir(), "test.queue")
	if err := q.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	wrongKey := makeKey(t)
	_, err := audit.LoadQueue(path, wrongKey)
	if err == nil {
		t.Fatal("LoadQueue with wrong key should return an error")
	}
}

func TestQueue_LoadQueue_NonExistentFile(t *testing.T) {
	key := makeKey(t)
	path := filepath.Join(t.TempDir(), "missing.queue")

	q, err := audit.LoadQueue(path, key)
	if err != nil {
		t.Fatalf("LoadQueue on missing file should not return error, got: %v", err)
	}
	if q.Len() != 0 {
		t.Errorf("LoadQueue on missing file Len = %d, want 0", q.Len())
	}
}

func TestQueue_Save_RandomNonces(t *testing.T) {
	key := makeKey(t)
	q, _ := audit.NewQueue(key, 100)
	_ = q.Append(makeEvent("totp"))

	dir := t.TempDir()
	path1 := filepath.Join(dir, "q1.queue")
	path2 := filepath.Join(dir, "q2.queue")

	if err := q.Save(path1); err != nil {
		t.Fatalf("Save 1: %v", err)
	}
	if err := q.Save(path2); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	data1, err1 := readFile(t, path1)
	if err1 != nil {
		t.Fatalf("readFile 1: %v", err1)
	}
	data2, err2 := readFile(t, path2)
	if err2 != nil {
		t.Fatalf("readFile 2: %v", err2)
	}

	// The two files must differ (different random nonces produce different ciphertext).
	if bytes.Equal(data1, data2) {
		t.Error("two Save calls produced identical ciphertext — random nonces not being used")
	}
}

// readFile reads raw file bytes for binary comparison between two Save calls.
func readFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()
	return os.ReadFile(path)
}
