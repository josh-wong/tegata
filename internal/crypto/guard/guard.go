// Package guard provides secure memory management for cryptographic key material.
//
// This is the ONLY package in Tegata that imports memguard directly. All other
// packages that need guarded memory must use the types defined here. This
// encapsulation ensures that if the memguard API changes (it is marked
// experimental), only this package needs updating.
//
// SecretBuffer wraps memguard.LockedBuffer, which uses OS-level memory
// protections (mlock, guard pages, canary values) to protect sensitive data.
// KeyEnclave wraps memguard.Enclave, which encrypts data at rest in memory
// without consuming mlock quota.
//
// Design guidance: prefer KeyEnclave for storage and only Open to SecretBuffer
// when actively using the key material. On Linux, the default mlock limit is
// typically 64KB per process, so minimize concurrent LockedBuffer allocations.
package guard

import "github.com/awnumar/memguard"

// SecretBuffer holds sensitive data in OS-protected memory (mlocked, with guard
// pages and canary values). The slice returned by Bytes is backed by the
// guarded allocation and becomes invalid after Destroy is called.
type SecretBuffer struct {
	buf *memguard.LockedBuffer
}

// NewSecretBuffer creates a guarded buffer from the given data. The input slice
// is explicitly zeroed after the data is copied into protected memory.
func NewSecretBuffer(data []byte) *SecretBuffer {
	lb := memguard.NewBufferFromBytes(data)
	// memguard.NewBufferFromBytes may or may not zero the input. Zero it
	// explicitly to guarantee the caller's slice is wiped.
	for i := range data {
		data[i] = 0
	}
	return &SecretBuffer{buf: lb}
}

// NewSecretBufferFromSize creates a zeroed guarded buffer of the given size.
func NewSecretBufferFromSize(size int) *SecretBuffer {
	lb := memguard.NewBuffer(size)
	return &SecretBuffer{buf: lb}
}

// Bytes returns the underlying byte slice. The returned slice must not be
// retained or used after Destroy is called on this SecretBuffer.
func (s *SecretBuffer) Bytes() []byte {
	if s.buf == nil {
		return nil
	}
	return s.buf.Bytes()
}

// Size returns the length of the guarded buffer.
func (s *SecretBuffer) Size() int {
	if s.buf == nil {
		return 0
	}
	return s.buf.Size()
}

// Destroy zeroes and releases the guarded memory. After calling Destroy,
// Bytes returns nil and Size returns 0.
func (s *SecretBuffer) Destroy() {
	if s.buf != nil {
		s.buf.Destroy()
		s.buf = nil
	}
}

// KeyEnclave holds sensitive data encrypted at rest in memory. Unlike
// SecretBuffer, an Enclave does not consume mlock quota and is suitable for
// longer-term storage of key material that is not actively in use.
type KeyEnclave struct {
	enc *memguard.Enclave
}

// Seal encrypts a SecretBuffer into a KeyEnclave. The SecretBuffer is destroyed
// after sealing, so the caller must not use it afterward.
func Seal(sb *SecretBuffer) *KeyEnclave {
	// Copy bytes out of the guarded buffer before creating the enclave.
	// memguard.NewEnclave wipes its input, which would segfault on the
	// write-protected LockedBuffer memory.
	raw := make([]byte, sb.Size())
	copy(raw, sb.buf.Bytes())
	sb.Destroy()
	enc := memguard.NewEnclave(raw)
	return &KeyEnclave{enc: enc}
}

// Open decrypts the enclave into a new SecretBuffer. The caller is responsible
// for calling Destroy on the returned SecretBuffer when done. The same enclave
// can be opened multiple times.
func (ke *KeyEnclave) Open() (*SecretBuffer, error) {
	lb, err := ke.enc.Open()
	if err != nil {
		return nil, err
	}
	return &SecretBuffer{buf: lb}, nil
}
