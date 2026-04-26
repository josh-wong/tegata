package audit

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	tegerrors "github.com/josh-wong/tegata/internal/errors"
)

// ErrFlushFailed is a sentinel error returned when a Flush submission fails.
// Tests use this to trigger the error path via the mock submitter.
var ErrFlushFailed = errors.New("flush submission failed")

// Submitter is the minimal interface Flush requires. It is implemented by the
// EventBuilder in Plan 03 (the gRPC client), allowing Queue to remain decoupled
// from the network layer.
type Submitter interface {
	Submit(ctx context.Context, entry QueueEntry) (string, error)
}

// QueueEntry is the unit of storage in the offline queue. On disk, only
// Ciphertext, Nonce, and PrevHash are stored (Event is excluded from JSON via
// the "-" tag). After LoadQueue, Event is populated by decrypting Ciphertext.
//
// The PrevHash for chain integrity is computed over the plaintext entry JSON
// before encryption, ensuring chain integrity is verifiable after decryption.
type QueueEntry struct {
	// Event is populated in-memory after decryption; excluded from disk JSON.
	Event AuthEvent `json:"-"`
	// Ciphertext is the AES-256-GCM encryption of the AuthEvent JSON.
	Ciphertext []byte `json:"ciphertext"`
	// Nonce is the 12-byte random GCM nonce used to encrypt this entry.
	Nonce []byte `json:"nonce"`
	// PrevHash is the hex(SHA-256) of the previous entry's plaintext JSON,
	// or "" for the first entry.
	PrevHash string `json:"prev_hash"`
}

// diskEntry is the on-disk representation written to the queue file.
// It mirrors QueueEntry but makes the structure explicit.
type diskEntry struct {
	Ciphertext []byte `json:"ciphertext"`
	Nonce      []byte `json:"nonce"`
	PrevHash   string `json:"prev_hash"`
}

// Queue is an in-memory offline event queue backed by AES-256-GCM encryption
// on disk. Each entry is independently encrypted with a fresh random nonce.
// Entries form a local hash chain: each entry's PrevHash is hex(SHA-256) of
// the previous entry's plaintext JSON.
type Queue struct {
	entries []QueueEntry
	key     []byte // 32-byte AES-256 key, not a SecretBuffer (caller controls lifecycle)
	maxLen  int
	salt    [32]byte // stored in the 32-byte file header; used for key derivation context
}

// NewQueue creates an empty Queue with the given 32-byte key and maximum entry
// capacity. A random 32-byte salt is generated for the queue file header.
// Returns an error if key is not exactly 32 bytes.
func NewQueue(key []byte, maxLen int) (*Queue, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("queue key must be 32 bytes, got %d", len(key))
	}

	q := &Queue{
		key:    make([]byte, 32),
		maxLen: maxLen,
	}
	copy(q.key, key)

	if _, err := io.ReadFull(rand.Reader, q.salt[:]); err != nil {
		return nil, fmt.Errorf("generating queue salt: %w", err)
	}

	return q, nil
}

// LoadQueue reads a queue file from path. The first 32 bytes are the salt
// header; the remainder is a JSON array of diskEntry values. Each ciphertext
// is decrypted with the provided 32-byte key. Returns an empty Queue (not an
// error) if the file does not exist. Returns an error if decryption fails
// (wrong key or tampered data).
func LoadQueue(path string, key []byte) (*Queue, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("queue key must be 32 bytes, got %d", len(key))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return an empty queue when the file doesn't exist yet.
			q := &Queue{
				key:    make([]byte, 32),
				maxLen: 10000,
			}
			copy(q.key, key)
			if _, err2 := io.ReadFull(rand.Reader, q.salt[:]); err2 != nil {
				return nil, fmt.Errorf("generating queue salt: %w", err2)
			}
			return q, nil
		}
		return nil, fmt.Errorf("reading queue file: %w", err)
	}

	if len(data) < 32 {
		return nil, fmt.Errorf("queue file too short: %d bytes", len(data))
	}

	q := &Queue{
		key:    make([]byte, 32),
		maxLen: 10000,
	}
	copy(q.key, key)
	copy(q.salt[:], data[:32])

	// Decode the JSON array of disk entries.
	var diskEntries []diskEntry
	if err := json.Unmarshal(data[32:], &diskEntries); err != nil {
		return nil, fmt.Errorf("parsing queue file: %w", err)
	}

	// Decrypt each entry and rebuild the in-memory QueueEntry slice.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	q.entries = make([]QueueEntry, 0, len(diskEntries))
	for i, de := range diskEntries {
		plaintext, err := gcm.Open(nil, de.Nonce, de.Ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("decrypting queue entry %d: %w", i, tegerrors.ErrDecryptFailed)
		}

		var evt AuthEvent
		if err := json.Unmarshal(plaintext, &evt); err != nil {
			return nil, fmt.Errorf("parsing decrypted event %d: %w", i, err)
		}

		q.entries = append(q.entries, QueueEntry{
			Event:      evt,
			Ciphertext: de.Ciphertext,
			Nonce:      de.Nonce,
			PrevHash:   de.PrevHash,
		})
	}

	return q, nil
}

// Append adds evt to the queue. The PrevHash of the new entry is computed as
// hex(SHA-256) of the previous entry's plaintext AuthEvent JSON, or "" for the
// first entry. The AuthEvent is encrypted with a fresh 12-byte random nonce.
// If the queue is at capacity (maxLen), the oldest entry is dropped with a
// warning written to stderr.
func (q *Queue) Append(evt AuthEvent) error {
	if q.maxLen > 0 && len(q.entries) >= q.maxLen {
		_, _ = fmt.Fprintf(os.Stderr, "tegata: offline queue at capacity (%d), dropping oldest entry\n", q.maxLen)
		q.entries = q.entries[1:]
	}

	// Compute PrevHash over the previous entry's plaintext AuthEvent JSON.
	prevHash := ""
	if len(q.entries) > 0 {
		prevHash = EntryHash(q.entries[len(q.entries)-1])
	}

	// Override the event's PrevHash field so chain computation is self-contained.
	evt.PrevHash = prevHash

	// Serialize the AuthEvent to JSON for encryption.
	plaintext, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("serializing event: %w", err)
	}

	// Encrypt with a fresh random nonce.
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}

	block, err := aes.NewCipher(q.key)
	if err != nil {
		return fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("creating GCM: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	for i := range plaintext {
		plaintext[i] = 0
	}

	q.entries = append(q.entries, QueueEntry{
		Event:      evt,
		Ciphertext: ciphertext,
		Nonce:      nonce,
		PrevHash:   prevHash,
	})

	return nil
}

// Save writes the queue to path. The file begins with the 32-byte salt header
// followed by a JSON array of disk entries (Ciphertext, Nonce, PrevHash).
// Each entry's ciphertext is re-encrypted with a fresh random nonce, ensuring
// two Save calls on the same queue produce different ciphertext.
func (q *Queue) Save(path string) error {
	block, err := aes.NewCipher(q.key)
	if err != nil {
		return fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("creating GCM: %w", err)
	}

	diskEntries := make([]diskEntry, 0, len(q.entries))
	for _, entry := range q.entries {
		// Re-encrypt with a fresh random nonce on every Save.
		nonce := make([]byte, 12)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return fmt.Errorf("generating nonce: %w", err)
		}

		plaintext, err := json.Marshal(entry.Event)
		if err != nil {
			return fmt.Errorf("serializing event: %w", err)
		}

		ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
		for i := range plaintext {
			plaintext[i] = 0
		}
		diskEntries = append(diskEntries, diskEntry{
			Ciphertext: ciphertext,
			Nonce:      nonce,
			PrevHash:   entry.PrevHash,
		})
	}

	entriesJSON, err := json.Marshal(diskEntries)
	if err != nil {
		return fmt.Errorf("serializing queue: %w", err)
	}

	fileData := make([]byte, 32+len(entriesJSON))
	copy(fileData[:32], q.salt[:])
	copy(fileData[32:], entriesJSON)

	return os.WriteFile(path, fileData, 0600)
}

// Flush calls client.Submit for each entry in append order. On the first
// submission error, Flush stops and returns the error, leaving the queue
// unchanged. On full success, the entries slice is cleared (Len returns 0).
//
// onHash, if non-nil, is called with (eventID, hashValue) for each successfully
// submitted entry so callers can persist the hash for independent verification.
func (q *Queue) Flush(ctx context.Context, client Submitter, onHash func(eventID, hashValue string)) error {
	for _, entry := range q.entries {
		hashValue, err := client.Submit(ctx, entry)
		if err != nil {
			return fmt.Errorf("submitting queue entry: %w", err)
		}
		if onHash != nil && hashValue != "" {
			onHash(entry.Event.EventID, hashValue)
		}
	}
	q.entries = q.entries[:0]
	return nil
}

// Len returns the number of entries in the queue.
func (q *Queue) Len() int {
	return len(q.entries)
}

// VerifyChain checks that the hash chain linking entries is intact. It returns
// tegerrors.ErrIntegrityViolation if any entry's PrevHash does not match the
// expected value (hex(SHA-256) of the previous entry's AuthEvent JSON).
func (q *Queue) VerifyChain() error {
	for i, entry := range q.entries {
		var expectedPrev string
		if i == 0 {
			expectedPrev = ""
		} else {
			expectedPrev = EntryHash(q.entries[i-1])
		}
		if entry.PrevHash != expectedPrev {
			return fmt.Errorf("queue entry %d: %w", i, tegerrors.ErrIntegrityViolation)
		}
	}
	return nil
}

// Entries returns a copy of the queue entries for inspection (used by tests
// and by Flush to iterate the queue).
func (q *Queue) Entries() []QueueEntry {
	result := make([]QueueEntry, len(q.entries))
	copy(result, q.entries)
	return result
}

// EntryHash returns hex(SHA-256) of the entry's AuthEvent JSON. This is the
// value stored in the next entry's PrevHash field.
//
// AuthEvent contains only string, bool, and time.Time fields that are always
// JSON-serializable, so Marshal failure is a programming error. A panic is
// preferable to silently returning "" and breaking the hash chain invisibly.
func EntryHash(entry QueueEntry) string {
	data, err := json.Marshal(entry.Event)
	if err != nil {
		panic(fmt.Sprintf("audit: EntryHash: json.Marshal failed: %v", err))
	}
	sum := sha256.Sum256(data)
	for i := range data {
		data[i] = 0
	}
	return hex.EncodeToString(sum[:])
}

// DropFront removes the first n entries from the queue. Used by EventBuilder
// after a successful flush to clear only the entries that were submitted.
// If n >= len(entries) the queue is cleared. Negative n is a no-op.
func (q *Queue) DropFront(n int) {
	if n <= 0 {
		return
	}
	if n >= len(q.entries) {
		q.entries = q.entries[:0]
		return
	}
	q.entries = q.entries[n:]
}

// CorruptEntry replaces the PrevHash of the entry at index i. This is exported
// for white-box testing of VerifyChain corruption detection.
func (q *Queue) CorruptEntry(i int, prevHash string) {
	if i < 0 || i >= len(q.entries) {
		return
	}
	q.entries[i].PrevHash = prevHash
}
