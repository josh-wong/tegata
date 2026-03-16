// Package vault implements the encrypted vault manager for Tegata. It provides
// vault creation, opening, locking/unlocking, credential CRUD, recovery key
// generation, rate limiting, and crash-safe file writes.
package vault

import (
	"encoding/binary"
	"fmt"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// headerSize is the fixed size of a serialized vault header in bytes.
const headerSize = 128

// magic is the 8-byte file signature for Tegata vault files.
var magic = [8]byte{'T', 'E', 'G', 'A', 'T', 'A', 0, 0}

// Marshal serializes a VaultHeader into exactly 128 bytes using big-endian
// encoding at explicit byte offsets. The layout matches the design doc section
// 3.1 and avoids binary.Read/Write struct padding issues.
func Marshal(h *model.VaultHeader) ([]byte, error) {
	buf := make([]byte, headerSize)
	off := 0

	// magic (8 bytes)
	copy(buf[off:off+8], h.Magic[:])
	off += 8

	// version (2 bytes)
	binary.BigEndian.PutUint16(buf[off:off+2], h.Version)
	off += 2

	// argonTime (4 bytes)
	binary.BigEndian.PutUint32(buf[off:off+4], h.ArgonTime)
	off += 4

	// argonMemory (4 bytes)
	binary.BigEndian.PutUint32(buf[off:off+4], h.ArgonMemory)
	off += 4

	// argonParallelism (1 byte)
	buf[off] = h.ArgonParallelism
	off++

	// salt (32 bytes)
	copy(buf[off:off+32], h.Salt[:])
	off += 32

	// recoverySalt (32 bytes)
	copy(buf[off:off+32], h.RecoveryKeySalt[:])
	off += 32

	// writeCounter (8 bytes)
	binary.BigEndian.PutUint64(buf[off:off+8], h.WriteCounter)
	off += 8

	// nonce (12 bytes)
	copy(buf[off:off+12], h.Nonce[:])
	off += 12

	// failedAttempts (1 byte)
	buf[off] = h.FailedAttempts
	off++

	// lastAttemptTime (8 bytes)
	binary.BigEndian.PutUint64(buf[off:off+8], uint64(h.LastAttemptTime))
	off += 8

	// reserved (16 bytes) - already zeroed from make
	copy(buf[off:off+16], h.Reserved[:])
	off += 16

	if off != headerSize {
		return nil, fmt.Errorf("header serialization offset %d != %d", off, headerSize)
	}

	return buf, nil
}

// Unmarshal deserializes 128 bytes into a VaultHeader. Returns
// ErrVaultCorrupt if the input is too short or the magic bytes do not match.
func Unmarshal(data []byte) (*model.VaultHeader, error) {
	if len(data) < headerSize {
		return nil, fmt.Errorf("header too short (%d bytes): %w", len(data), errors.ErrVaultCorrupt)
	}

	h := &model.VaultHeader{}
	off := 0

	// magic (8 bytes)
	copy(h.Magic[:], data[off:off+8])
	off += 8
	if h.Magic != magic {
		return nil, fmt.Errorf("invalid magic bytes: %w", errors.ErrVaultCorrupt)
	}

	// version (2 bytes)
	h.Version = binary.BigEndian.Uint16(data[off : off+2])
	off += 2

	// argonTime (4 bytes)
	h.ArgonTime = binary.BigEndian.Uint32(data[off : off+4])
	off += 4

	// argonMemory (4 bytes)
	h.ArgonMemory = binary.BigEndian.Uint32(data[off : off+4])
	off += 4

	// argonParallelism (1 byte)
	h.ArgonParallelism = data[off]
	off++

	// salt (32 bytes)
	copy(h.Salt[:], data[off:off+32])
	off += 32

	// recoverySalt (32 bytes)
	copy(h.RecoveryKeySalt[:], data[off:off+32])
	off += 32

	// writeCounter (8 bytes)
	h.WriteCounter = binary.BigEndian.Uint64(data[off : off+8])
	off += 8

	// nonce (12 bytes)
	copy(h.Nonce[:], data[off:off+12])
	off += 12

	// failedAttempts (1 byte)
	h.FailedAttempts = data[off]
	off++

	// lastAttemptTime (8 bytes)
	h.LastAttemptTime = int64(binary.BigEndian.Uint64(data[off : off+8]))
	off += 8

	// reserved (16 bytes)
	copy(h.Reserved[:], data[off:off+16])

	return h, nil
}
