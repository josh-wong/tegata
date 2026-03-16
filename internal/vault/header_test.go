package vault

import (
	"testing"
	"time"

	"github.com/josh-wong/tegata/pkg/model"
)

func testHeader() *model.VaultHeader {
	h := &model.VaultHeader{
		Version:          1,
		ArgonTime:        3,
		ArgonMemory:      65536,
		ArgonParallelism: 4,
		WriteCounter:     42,
		FailedAttempts:   2,
		LastAttemptTime:  1700000000,
	}
	copy(h.Magic[:], []byte("TEGATA\x00\x00"))
	for i := range h.Salt {
		h.Salt[i] = byte(i)
	}
	for i := range h.RecoveryKeySalt {
		h.RecoveryKeySalt[i] = byte(i + 100)
	}
	copy(h.Nonce[:], []byte("123456789012"))
	return h
}

func TestHeaderMarshalSize(t *testing.T) {
	h := testHeader()
	data, err := Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != 128 {
		t.Fatalf("Marshal produced %d bytes, want 128", len(data))
	}
}

func TestHeaderRoundTrip(t *testing.T) {
	h := testHeader()
	data, err := Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Magic != h.Magic {
		t.Errorf("Magic: got %v, want %v", got.Magic, h.Magic)
	}
	if got.Version != h.Version {
		t.Errorf("Version: got %d, want %d", got.Version, h.Version)
	}
	if got.ArgonTime != h.ArgonTime {
		t.Errorf("ArgonTime: got %d, want %d", got.ArgonTime, h.ArgonTime)
	}
	if got.ArgonMemory != h.ArgonMemory {
		t.Errorf("ArgonMemory: got %d, want %d", got.ArgonMemory, h.ArgonMemory)
	}
	if got.ArgonParallelism != h.ArgonParallelism {
		t.Errorf("ArgonParallelism: got %d, want %d", got.ArgonParallelism, h.ArgonParallelism)
	}
	if got.Salt != h.Salt {
		t.Errorf("Salt mismatch")
	}
	if got.RecoveryKeySalt != h.RecoveryKeySalt {
		t.Errorf("RecoveryKeySalt mismatch")
	}
	if got.WriteCounter != h.WriteCounter {
		t.Errorf("WriteCounter: got %d, want %d", got.WriteCounter, h.WriteCounter)
	}
	if got.Nonce != h.Nonce {
		t.Errorf("Nonce mismatch")
	}
	if got.FailedAttempts != h.FailedAttempts {
		t.Errorf("FailedAttempts: got %d, want %d", got.FailedAttempts, h.FailedAttempts)
	}
	if got.LastAttemptTime != h.LastAttemptTime {
		t.Errorf("LastAttemptTime: got %d, want %d", got.LastAttemptTime, h.LastAttemptTime)
	}
}

func TestHeaderInvalidMagic(t *testing.T) {
	h := testHeader()
	data, err := Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Corrupt magic bytes.
	data[0] = 'X'
	_, err = Unmarshal(data)
	if err == nil {
		t.Fatal("expected error for invalid magic, got nil")
	}
}

func TestHeaderUnmarshalShortInput(t *testing.T) {
	_, err := Unmarshal(make([]byte, 64))
	if err == nil {
		t.Fatal("expected error for short input, got nil")
	}
}

func TestRateLimitNoFailures(t *testing.T) {
	h := testHeader()
	h.FailedAttempts = 0
	h.LastAttemptTime = 0
	wait, err := CheckRateLimit(h)
	if err != nil {
		t.Fatalf("CheckRateLimit: %v", err)
	}
	if wait != 0 {
		t.Errorf("expected zero wait, got %v", wait)
	}
}

func TestRateLimitAfterFailures(t *testing.T) {
	h := testHeader()
	h.FailedAttempts = 3
	// Set last attempt to now so the full backoff applies.
	h.LastAttemptTime = time.Now().Unix()
	wait, err := CheckRateLimit(h)
	if err != nil {
		t.Fatalf("CheckRateLimit: %v", err)
	}
	// 3 failures = 2^(3-1) = 4 seconds backoff.
	if wait < 3*time.Second || wait > 5*time.Second {
		t.Errorf("expected ~4s wait for 3 failures, got %v", wait)
	}
}

func TestRateLimitCap(t *testing.T) {
	h := testHeader()
	h.FailedAttempts = 20 // 2^19 would be huge, but cap is 300s.
	h.LastAttemptTime = time.Now().Unix()
	wait, err := CheckRateLimit(h)
	if err != nil {
		t.Fatalf("CheckRateLimit: %v", err)
	}
	if wait > 300*time.Second {
		t.Errorf("expected wait capped at 300s, got %v", wait)
	}
}

func TestRecordFailure(t *testing.T) {
	h := testHeader()
	h.FailedAttempts = 0
	RecordFailure(h)
	if h.FailedAttempts != 1 {
		t.Errorf("FailedAttempts: got %d, want 1", h.FailedAttempts)
	}
	if h.LastAttemptTime == 0 {
		t.Error("LastAttemptTime not set")
	}
}

func TestRecordFailureSaturates(t *testing.T) {
	h := testHeader()
	h.FailedAttempts = 255
	RecordFailure(h)
	if h.FailedAttempts != 255 {
		t.Errorf("FailedAttempts should saturate at 255, got %d", h.FailedAttempts)
	}
}

func TestResetAttempts(t *testing.T) {
	h := testHeader()
	h.FailedAttempts = 5
	ResetAttempts(h)
	if h.FailedAttempts != 0 {
		t.Errorf("FailedAttempts: got %d, want 0", h.FailedAttempts)
	}
}
