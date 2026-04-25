package vault

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/crypto/guard"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// exportEnvelope is the inner JSON document written into a .tegata-backup file.
// It contains only the credentials array, not the vault header, DEK, or
// rate-limit state, so the backup is fully self-contained and portable.
type exportEnvelope struct {
	Version     int               `json:"version"`
	ExportedAt  time.Time         `json:"exported_at"`
	Credentials []model.Credential `json:"credentials"`
}

// zeroBytes overwrites a byte slice with zeroes to limit the lifetime of
// sensitive data (plaintext JSON, assembled file buffers) on the regular heap.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Manager provides access to an opened vault file. Call Unlock to decrypt the
// payload before performing credential operations. Always defer Close to zero
// sensitive memory.
//
// Concurrency: All mutating operations are serialized by mu. Read-only methods
// (GetCredential, ListCredentials) are not protected and must not be called
// concurrently with writes. External file locking is not performed; concurrent
// writers from separate processes will corrupt the vault.
type Manager struct {
	mu              sync.Mutex
	path            string
	header          *model.VaultHeader
	payload         *model.VaultPayload
	dek             *guard.KeyEnclave
	recoveryWrapped []byte
	params          crypto.KDFParams
}

// Create initializes a new encrypted vault at the given path using the provided
// passphrase. It returns a recovery key display string.
func Create(path string, passphrase []byte, params crypto.KDFParams) (recoveryKey string, err error) {
	// Generate the 32-byte data encryption key (DEK).
	dekRaw := make([]byte, 32)
	if _, err := rand.Read(dekRaw); err != nil {
		return "", fmt.Errorf("generating DEK: %w", err)
	}

	// Generate salts.
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return "", err
	}
	recoverySalt, err := crypto.GenerateSalt()
	if err != nil {
		return "", err
	}

	// Key wrapping design: a random DEK encrypts the payload. The DEK is
	// independently wrapped by both the passphrase-derived key and the
	// recovery-derived key. On unlock, either key can unwrap the DEK, which
	// then decrypts the payload. This allows passphrase changes without
	// re-encrypting the entire payload — only the passphrase-wrapped DEK
	// needs to be re-created.

	// Derive passphrase key.
	passphraseKey := crypto.DeriveKey(passphrase, salt, params)

	// Encrypt DEK with passphrase key (key wrapping). Counter=1 is safe here
	// because each wrapping operation uses a freshly derived key (different
	// salt → different key), so the same counter value never reuses a
	// (key, nonce) pair. Counter-based nonce reuse is only dangerous when the
	// same key is used more than once with the same counter.
	passphraseWrappedDEK, err := crypto.Seal(passphraseKey, 1, dekRaw, nil)
	passphraseKey.Destroy()
	if err != nil {
		return "", fmt.Errorf("wrapping DEK with passphrase: %w", err)
	}

	// Generate recovery key.
	recoveryRaw, recoveryDisplay, err := GenerateRecoveryKey()
	if err != nil {
		return "", err
	}

	// Hash the recovery key and store it in the encrypted payload. The hash
	// is not used during UnlockWithRecoveryKey — GCM authentication provides
	// cryptographic verification there. The stored hash enables a future
	// "verify my backup" command to confirm a known recovery key string
	// matches what was generated at vault creation time, without re-deriving
	// the key or attempting a full unlock.
	recoveryHash := sha256.Sum256(recoveryRaw)
	recoveryHashHex := hex.EncodeToString(recoveryHash[:])

	// Derive recovery key and wrap DEK with it. Zero recoveryRaw afterward
	// since it is high-entropy key material on the regular heap.
	recoveryDerivedKey := crypto.DeriveKey(recoveryRaw, recoverySalt, params)
	for i := range recoveryRaw {
		recoveryRaw[i] = 0
	}
	recoveryWrappedDEK, err := crypto.Seal(recoveryDerivedKey, 1, dekRaw, nil)
	recoveryDerivedKey.Destroy()
	if err != nil {
		return "", fmt.Errorf("wrapping DEK with recovery key: %w", err)
	}

	// Zero the raw DEK, build the SecretBuffer for encryption.
	dekBuf := guard.NewSecretBuffer(dekRaw) // this zeros dekRaw

	// Build payload.
	now := time.Now().UTC()
	payload := &model.VaultPayload{
		Version:         1,
		CreatedAt:       now,
		ModifiedAt:      now,
		Credentials:     []model.Credential{},
		RecoveryKeyHash: recoveryHashHex,
		VaultID:         uuid.New().String(),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		dekBuf.Destroy()
		return "", fmt.Errorf("marshaling payload: %w", err)
	}

	// Encrypt payload with DEK, then zero the plaintext JSON immediately.
	encryptedPayload, err := crypto.Seal(dekBuf, 1, payloadJSON, nil)
	zeroBytes(payloadJSON)
	dekBuf.Destroy()
	if err != nil {
		return "", fmt.Errorf("encrypting payload: %w", err)
	}

	// Build header.
	header := &model.VaultHeader{
		Version:          1,
		ArgonTime:        params.Time,
		ArgonMemory:      params.Memory,
		ArgonParallelism: params.Threads,
		WriteCounter:     1,
	}
	copy(header.Magic[:], magic[:])
	copy(header.Salt[:], salt)
	copy(header.RecoveryKeySalt[:], recoverySalt)

	headerBytes, err := Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshaling header: %w", err)
	}

	// File layout:
	// header(128) + payloadLen(4) + encryptedPayload + wrappedDEKLen(4) + passphraseWrappedDEK + recoveryWrappedDEK
	payloadLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(payloadLenBuf, uint32(len(encryptedPayload)))

	wrappedDEKLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(wrappedDEKLenBuf, uint32(len(passphraseWrappedDEK)))

	var fileData []byte
	fileData = append(fileData, headerBytes...)
	fileData = append(fileData, payloadLenBuf...)
	fileData = append(fileData, encryptedPayload...)
	fileData = append(fileData, wrappedDEKLenBuf...)
	fileData = append(fileData, passphraseWrappedDEK...)
	fileData = append(fileData, recoveryWrappedDEK...)

	if err := atomicWrite(path, fileData); err != nil {
		zeroBytes(fileData)
		return "", err
	}
	zeroBytes(fileData)

	return recoveryDisplay, nil
}

// Open reads and validates a vault file header without decrypting the payload.
func Open(path string) (*Manager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading vault: %w", err)
	}

	if len(data) < headerSize+4 {
		return nil, fmt.Errorf("vault file too small: %w", errors.ErrVaultCorrupt)
	}

	header, err := Unmarshal(data[:headerSize])
	if err != nil {
		return nil, err
	}

	// Read payload length.
	payloadLen := binary.BigEndian.Uint32(data[headerSize : headerSize+4])
	afterPayload := headerSize + 4 + int(payloadLen)

	if afterPayload+4 > len(data) {
		return nil, fmt.Errorf("file truncated: %w", errors.ErrVaultCorrupt)
	}

	// Read wrapped DEK length.
	wrappedDEKLen := binary.BigEndian.Uint32(data[afterPayload : afterPayload+4])
	afterWrappedDEK := afterPayload + 4 + int(wrappedDEKLen)

	if afterWrappedDEK > len(data) {
		return nil, fmt.Errorf("file truncated at wrapped DEK: %w", errors.ErrVaultCorrupt)
	}

	// Recovery-wrapped DEK is everything after passphrase-wrapped DEK.
	recoveryWrapped := make([]byte, len(data)-afterWrappedDEK)
	copy(recoveryWrapped, data[afterWrappedDEK:])

	// Validate Argon2id parameters to prevent denial-of-service from crafted
	// vault files (e.g., ArgonMemory set to 4 GiB causing OOM).
	if header.ArgonTime < 1 || header.ArgonTime > 100 {
		return nil, fmt.Errorf("invalid argon2 time parameter %d: %w", header.ArgonTime, errors.ErrVaultCorrupt)
	}
	if header.ArgonMemory < 8 || header.ArgonMemory > 4*1024*1024 {
		return nil, fmt.Errorf("invalid argon2 memory parameter %d KiB: %w", header.ArgonMemory, errors.ErrVaultCorrupt)
	}
	if header.ArgonParallelism < 1 {
		return nil, fmt.Errorf("invalid argon2 parallelism parameter %d: %w", header.ArgonParallelism, errors.ErrVaultCorrupt)
	}

	m := &Manager{
		path:            path,
		header:          header,
		recoveryWrapped: recoveryWrapped,
		params: crypto.KDFParams{
			Time:    header.ArgonTime,
			Memory:  header.ArgonMemory,
			Threads: header.ArgonParallelism,
			KeyLen:  32,
		},
	}
	return m, nil
}

// Unlock decrypts the vault payload using the provided passphrase.
func (m *Manager) Unlock(passphrase []byte) error {
	// Check rate limit.
	wait, err := CheckRateLimit(m.header)
	if err != nil {
		return err
	}
	if wait > 0 {
		return fmt.Errorf("rate-limited for %v: %w", wait.Round(time.Second), errors.ErrAuthFailed)
	}

	// Derive key from passphrase.
	derivedKey := crypto.DeriveKey(passphrase, m.header.Salt[:], m.params)

	// Read the file to get wrapped DEK and encrypted payload.
	data, err := os.ReadFile(m.path)
	if err != nil {
		derivedKey.Destroy()
		return fmt.Errorf("reading vault: %w", err)
	}

	payloadLen := binary.BigEndian.Uint32(data[headerSize : headerSize+4])
	encryptedPayload := data[headerSize+4 : headerSize+4+int(payloadLen)]
	afterPayload := headerSize + 4 + int(payloadLen)
	wrappedDEKLen := binary.BigEndian.Uint32(data[afterPayload : afterPayload+4])
	passphraseWrappedDEK := data[afterPayload+4 : afterPayload+4+int(wrappedDEKLen)]

	// Unwrap DEK using passphrase-derived key.
	dekBytes, err := crypto.Open(derivedKey, 1, passphraseWrappedDEK, nil)
	derivedKey.Destroy()
	if err != nil {
		RecordFailure(m.header)
		if herr := m.saveHeader(); herr != nil {
			slog.Warn("failed to persist header", "error", herr)
		}
		return fmt.Errorf("wrong passphrase: %w", errors.ErrAuthFailed)
	}

	// Use DEK to decrypt payload.
	dekBuf := guard.NewSecretBuffer(dekBytes)
	plaintext, err := crypto.Open(dekBuf, m.header.WriteCounter, encryptedPayload, nil)
	if err != nil {
		dekBuf.Destroy()
		RecordFailure(m.header)
		if herr := m.saveHeader(); herr != nil {
			slog.Warn("failed to persist header", "error", herr)
		}
		return fmt.Errorf("decrypting payload: %w", errors.ErrAuthFailed)
	}

	var payload model.VaultPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		zeroBytes(plaintext)
		dekBuf.Destroy()
		return fmt.Errorf("parsing payload: %w", errors.ErrVaultCorrupt)
	}
	zeroBytes(plaintext)

	m.payload = &payload
	m.dek = guard.Seal(dekBuf)

	ResetAttempts(m.header)
	if herr := m.saveHeader(); herr != nil {
		slog.Warn("failed to persist header", "error", herr)
	}

	return nil
}

// UnlockWithRecoveryKey decrypts the vault using a raw recovery key (32 bytes).
// Recovery key unlock is subject to the same rate limiting as passphrase unlock
// to prevent brute-force attempts.
func (m *Manager) UnlockWithRecoveryKey(recoveryRaw []byte) error {
	// Check rate limit (shared with passphrase unlock).
	wait, err := CheckRateLimit(m.header)
	if err != nil {
		return err
	}
	if wait > 0 {
		return fmt.Errorf("rate-limited for %v: %w", wait.Round(time.Second), errors.ErrAuthFailed)
	}

	// Derive key from recovery key using recovery salt.
	recoveryDerivedKey := crypto.DeriveKey(recoveryRaw, m.header.RecoveryKeySalt[:], m.params)

	// Decrypt the wrapped DEK.
	dekBytes, err := crypto.Open(recoveryDerivedKey, 1, m.recoveryWrapped, nil)
	recoveryDerivedKey.Destroy()
	if err != nil {
		RecordFailure(m.header)
		if herr := m.saveHeader(); herr != nil {
			slog.Warn("failed to persist header", "error", herr)
		}
		return fmt.Errorf("recovery key invalid: %w", errors.ErrAuthFailed)
	}

	// Use the DEK to decrypt the payload.
	dekBuf := guard.NewSecretBuffer(dekBytes)

	data, err := os.ReadFile(m.path)
	if err != nil {
		dekBuf.Destroy()
		return fmt.Errorf("reading vault: %w", err)
	}

	payloadLen := binary.BigEndian.Uint32(data[headerSize : headerSize+4])
	encryptedPayload := data[headerSize+4 : headerSize+4+int(payloadLen)]

	plaintext, err := crypto.Open(dekBuf, m.header.WriteCounter, encryptedPayload, nil)
	if err != nil {
		dekBuf.Destroy()
		return fmt.Errorf("decrypting payload: %w", errors.ErrAuthFailed)
	}

	var payload model.VaultPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		zeroBytes(plaintext)
		dekBuf.Destroy()
		return fmt.Errorf("parsing payload: %w", errors.ErrVaultCorrupt)
	}
	zeroBytes(plaintext)

	m.payload = &payload
	m.dek = guard.Seal(dekBuf)

	ResetAttempts(m.header)
	if herr := m.saveHeader(); herr != nil {
		slog.Warn("failed to persist header", "error", herr)
	}

	return nil
}

// Close zeroes all sensitive memory held by the manager.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dek != nil {
		m.dek.Destroy()
		m.dek = nil
	}
	m.payload = nil
}

// AddCredential adds a credential to the vault and saves. Returns the assigned
// credential ID.
func (m *Manager) AddCredential(cred model.Credential) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.payload == nil {
		return "", fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}
	for _, existing := range m.payload.Credentials {
		if strings.EqualFold(existing.Label, cred.Label) {
			return "", fmt.Errorf("credential %q already exists: %w", cred.Label, errors.ErrInvalidInput)
		}
	}
	cred.ID = uuid.New().String()
	now := time.Now().UTC()
	cred.CreatedAt = now
	cred.ModifiedAt = now
	if cred.Tags == nil {
		cred.Tags = []string{}
	}
	m.payload.Credentials = append(m.payload.Credentials, cred)
	m.payload.ModifiedAt = now
	if err := m.saveLocked(); err != nil {
		return "", err
	}
	return cred.ID, nil
}

// RemoveCredential removes a credential by ID and saves.
func (m *Manager) RemoveCredential(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.payload == nil {
		return fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}
	for i, c := range m.payload.Credentials {
		if c.ID == id {
			m.payload.Credentials = append(m.payload.Credentials[:i], m.payload.Credentials[i+1:]...)
			m.payload.ModifiedAt = time.Now().UTC()
			return m.saveLocked()
		}
	}
	return fmt.Errorf("credential %q: %w", id, errors.ErrNotFound)
}

// GetCredential returns the credential matching the given label (case-insensitive).
func (m *Manager) GetCredential(label string) (*model.Credential, error) {
	if m.payload == nil {
		return nil, fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}
	for i := range m.payload.Credentials {
		if strings.EqualFold(m.payload.Credentials[i].Label, label) {
			return &m.payload.Credentials[i], nil
		}
	}
	return nil, fmt.Errorf("credential %q: %w", label, errors.ErrNotFound)
}

// ListCredentials returns a copy of all credentials in the vault.
func (m *Manager) ListCredentials() []model.Credential {
	if m.payload == nil {
		return nil
	}
	result := make([]model.Credential, len(m.payload.Credentials))
	copy(result, m.payload.Credentials)
	return result
}

// UpdateCredential replaces the credential with the matching ID and saves.
func (m *Manager) UpdateCredential(cred *model.Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.payload == nil {
		return fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}
	for i := range m.payload.Credentials {
		if m.payload.Credentials[i].ID == cred.ID {
			cred.ModifiedAt = time.Now().UTC()
			m.payload.Credentials[i] = *cred
			m.payload.ModifiedAt = cred.ModifiedAt
			return m.saveLocked()
		}
	}
	return fmt.Errorf("credential %q: %w", cred.ID, errors.ErrNotFound)
}

// VaultID returns the stable unique identifier for this vault, set at
// creation time. Returns empty string if the vault is locked (payload nil).
func (m *Manager) VaultID() string {
	if m.payload == nil {
		return ""
	}
	return m.payload.VaultID
}

// Header returns the vault header for inspecting rate-limit state.
func (m *Manager) Header() *model.VaultHeader {
	return m.header
}

// Save re-encrypts and writes the vault to disk using temp-file-rename.
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

// saveLocked is the internal Save implementation. Callers must hold m.mu.
func (m *Manager) saveLocked() error {
	if m.dek == nil || m.payload == nil {
		return fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}

	dekBuf, err := m.dek.Open()
	if err != nil {
		return fmt.Errorf("opening DEK: %w", err)
	}

	m.header.WriteCounter++

	payloadJSON, err := json.Marshal(m.payload)
	if err != nil {
		dekBuf.Destroy()
		return fmt.Errorf("marshaling payload: %w", err)
	}

	// Encrypt payload, then zero the plaintext JSON immediately.
	encryptedPayload, err := crypto.Seal(dekBuf, m.header.WriteCounter, payloadJSON, nil)
	zeroBytes(payloadJSON)
	dekBuf.Destroy()
	if err != nil {
		return fmt.Errorf("encrypting payload: %w", err)
	}

	// Read the existing file to preserve the passphrase-wrapped DEK, which
	// only changes on passphrase change (we don't hold the derived key).
	oldData, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("reading vault for save: %w", err)
	}

	// Extract the passphrase-wrapped DEK from the existing file.
	oldPayloadLen := binary.BigEndian.Uint32(oldData[headerSize : headerSize+4])
	oldAfterPayload := headerSize + 4 + int(oldPayloadLen)
	oldWrappedDEKLen := binary.BigEndian.Uint32(oldData[oldAfterPayload : oldAfterPayload+4])
	if oldWrappedDEKLen == 0 {
		// The passphrase-wrapped DEK is missing from the on-disk file — this
		// indicates prior corruption. Refuse to write and propagate the state.
		return fmt.Errorf("vault file corrupt: passphrase-wrapped DEK is missing: %w", errors.ErrVaultCorrupt)
	}
	passphraseWrappedDEK := oldData[oldAfterPayload+4 : oldAfterPayload+4+int(oldWrappedDEKLen)]

	headerBytes, err := Marshal(m.header)
	if err != nil {
		return fmt.Errorf("marshaling header: %w", err)
	}

	payloadLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(payloadLenBuf, uint32(len(encryptedPayload)))

	wrappedDEKLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(wrappedDEKLenBuf, uint32(len(passphraseWrappedDEK)))

	var fileData []byte
	fileData = append(fileData, headerBytes...)
	fileData = append(fileData, payloadLenBuf...)
	fileData = append(fileData, encryptedPayload...)
	fileData = append(fileData, wrappedDEKLenBuf...)
	fileData = append(fileData, passphraseWrappedDEK...)
	fileData = append(fileData, m.recoveryWrapped...)

	err = atomicWrite(m.path, fileData)
	zeroBytes(fileData)
	return err
}

// saveHeader writes only the header portion of the vault file. It must read
// the full file and rewrite it atomically because atomicWrite uses
// temp-file-rename, which is the only crash-safe write strategy on FAT32.
// In-place partial writes (e.g., pwrite) are not atomic on FAT32 and could
// leave the vault in a torn state if power is lost mid-write.
func (m *Manager) saveHeader() error {
	headerBytes, err := Marshal(m.header)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}

	copy(data[:headerSize], headerBytes)
	return atomicWrite(m.path, data)
}

// ExportCredentials produces an encrypted .tegata-backup payload containing all
// credentials in the vault. The output is self-contained: it does not include
// the vault header, the data encryption key, or rate-limit state. The caller
// provides a new exportPassphrase independent of the vault passphrase.
//
// Binary layout (outer wrapper):
//
//	salt[32] | time_BE4[4] | memory_BE4[4] | parallelism[1] | ciphertext[...]
//
// The KDF always uses crypto.DefaultParams (not the vault's own params) so
// the backup can be imported on machines with different vault KDF settings.
// The encryption counter is fixed at 1 — safe because each export uses a
// freshly generated random salt, guaranteeing a unique key per export.
func (m *Manager) ExportCredentials(exportPassphrase []byte) ([]byte, error) {
	if m.payload == nil {
		return nil, fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}

	creds := m.ListCredentials()

	envelope := exportEnvelope{
		Version:     1,
		ExportedAt:  time.Now().UTC(),
		Credentials: creds,
	}

	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshaling export envelope: %w", err)
	}

	salt, err := crypto.GenerateSalt()
	if err != nil {
		zeroBytes(envelopeJSON)
		return nil, err
	}

	// Always use DefaultParams so the backup is portable across machines with
	// different vault KDF tuning.
	params := crypto.DefaultParams
	exportKey := crypto.DeriveKey(exportPassphrase, salt, params)
	defer exportKey.Destroy()

	ciphertext, err := crypto.Seal(exportKey, 1, envelopeJSON, nil)
	zeroBytes(envelopeJSON)
	if err != nil {
		return nil, fmt.Errorf("encrypting export: %w", err)
	}

	// Assemble: salt(32) + time_BE4(4) + memory_BE4(4) + parallelism(1) + ciphertext
	out := make([]byte, 0, 41+len(ciphertext))
	out = append(out, salt...)
	timeBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(timeBuf, params.Time)
	out = append(out, timeBuf...)
	memBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(memBuf, params.Memory)
	out = append(out, memBuf...)
	out = append(out, params.Threads)
	out = append(out, ciphertext...)

	return out, nil
}

// ImportCredentials restores credentials from an encrypted .tegata-backup
// payload (produced by ExportCredentials) into the unlocked vault. Credentials
// whose label already exists in the vault are silently skipped. Nil Tags fields
// are normalized to []string{} for consistency with AddCredential behaviour.
//
// The importPassphrase must match the passphrase used during export. It is
// independent of the vault passphrase. Returns the count of newly imported
// and skipped (duplicate) credentials.
func (m *Manager) ImportCredentials(data, importPassphrase []byte) (imported, skipped int, err error) {
	if m.payload == nil {
		return 0, 0, fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}

	const prefixLen = 41 // salt(32) + time(4) + memory(4) + parallelism(1)
	if len(data) <= prefixLen {
		return 0, 0, fmt.Errorf("backup data too short: %w", errors.ErrVaultCorrupt)
	}

	salt := data[0:32]
	timeVal := binary.BigEndian.Uint32(data[32:36])
	memVal := binary.BigEndian.Uint32(data[36:40])
	parallelism := data[40]

	params := crypto.KDFParams{
		Time:    timeVal,
		Memory:  memVal,
		Threads: parallelism,
		KeyLen:  32,
	}

	importKey := crypto.DeriveKey(importPassphrase, salt, params)
	defer importKey.Destroy()

	plaintext, err := crypto.Open(importKey, 1, data[prefixLen:], nil)
	if err != nil {
		return 0, 0, fmt.Errorf("decrypting backup: %w", err)
	}

	var envelope exportEnvelope
	if err := json.Unmarshal(plaintext, &envelope); err != nil {
		zeroBytes(plaintext)
		return 0, 0, fmt.Errorf("parsing backup envelope: %w", errors.ErrVaultCorrupt)
	}
	zeroBytes(plaintext)

	if envelope.Version != 1 {
		return 0, 0, fmt.Errorf("unsupported backup version %d: %w", envelope.Version, errors.ErrVaultCorrupt)
	}

	for _, cred := range envelope.Credentials {
		if _, gerr := m.GetCredential(cred.Label); gerr == nil {
			// Label already exists — skip.
			skipped++
			continue
		}
		if cred.Tags == nil {
			cred.Tags = []string{}
		}
		if _, aerr := m.AddCredential(cred); aerr != nil {
			return imported, skipped, fmt.Errorf("importing credential %q: %w", cred.Label, aerr)
		}
		imported++
	}

	return imported, skipped, nil
}

// ChangePassphrase re-wraps the DEK under a new passphrase without re-encrypting
// the payload. Only the passphrase-derived key and the header salt are replaced.
// The WriteCounter is deliberately NOT incremented because the encrypted payload
// is not touched — the counter-based nonce inside the payload ciphertext remains
// valid. Recovery key wrapping is preserved unchanged.
//
// This design (key wrapping vs. payload re-encryption) is what makes passphrase
// rotation fast and safe: even a power failure during the atomic write cannot
// corrupt the payload, because the old file is preserved as a .bak until the
// rename succeeds.
func (m *Manager) ChangePassphrase(newPassphrase []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dek == nil || m.payload == nil {
		return fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}

	// Open the DEK from guarded memory.
	dekBuf, err := m.dek.Open()
	if err != nil {
		return fmt.Errorf("opening DEK: %w", err)
	}
	defer dekBuf.Destroy()

	// Generate new salt for the new passphrase-derived key.
	newSalt, err := crypto.GenerateSalt()
	if err != nil {
		return fmt.Errorf("generating new salt: %w", err)
	}

	// Derive new passphrase key and re-wrap the DEK.
	newKey := crypto.DeriveKey(newPassphrase, newSalt, m.params)
	defer newKey.Destroy()

	// Counter=1 is safe here because each wrapping uses a fresh salt, so the
	// (key, nonce) pair is unique regardless of reusing counter=1.
	newWrappedDEK, err := crypto.Seal(newKey, 1, dekBuf.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("re-wrapping DEK: %w", err)
	}

	// Read the current vault file to extract the existing encryptedPayload
	// without re-encrypting it. This preserves the WriteCounter.
	oldData, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("reading vault for passphrase change: %w", err)
	}

	oldPayloadLen := binary.BigEndian.Uint32(oldData[headerSize : headerSize+4])
	encryptedPayload := oldData[headerSize+4 : headerSize+4+int(oldPayloadLen)]

	// Update header salt — the only header field that changes.
	copy(m.header.Salt[:], newSalt)

	headerBytes, err := Marshal(m.header)
	if err != nil {
		return fmt.Errorf("marshaling header: %w", err)
	}

	// Reassemble: header(128) + payloadLen(4) + encryptedPayload +
	// wrappedDEKLen(4) + newWrappedDEK + recoveryWrappedDEK
	payloadLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(payloadLenBuf, uint32(len(encryptedPayload)))

	wrappedDEKLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(wrappedDEKLenBuf, uint32(len(newWrappedDEK)))

	var fileData []byte
	fileData = append(fileData, headerBytes...)
	fileData = append(fileData, payloadLenBuf...)
	fileData = append(fileData, encryptedPayload...)
	fileData = append(fileData, wrappedDEKLenBuf...)
	fileData = append(fileData, newWrappedDEK...)
	fileData = append(fileData, m.recoveryWrapped...)

	if err := atomicWrite(m.path, fileData); err != nil {
		zeroBytes(fileData)
		return err
	}
	zeroBytes(fileData)

	// Re-seal the DEK enclave so in-memory state remains valid. The enclave
	// content is unchanged; this just refreshes the guarded-memory handle.
	newDEKBuf := guard.NewSecretBuffer(append([]byte(nil), dekBuf.Bytes()...))
	if m.dek != nil {
		m.dek.Destroy()
	}
	m.dek = guard.Seal(newDEKBuf)

	return nil
}

// VerifyRecoveryKey checks whether the provided raw recovery key bytes match
// the SHA-256 hash stored in the vault payload at creation time. Returns
// true/nil if the key matches, false/nil if it does not (a mismatch is a
// valid outcome, not an error). Returns an error only if the vault is locked.
func (m *Manager) VerifyRecoveryKey(recoveryRaw []byte) (bool, error) {
	if m.payload == nil {
		return false, fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}

	hash := sha256.Sum256(recoveryRaw)
	hashHex := hex.EncodeToString(hash[:])

	return hashHex == m.payload.RecoveryKeyHash, nil
}

// SetAuditHash stores an audit event hash in the vault for independent
// verification. The vault is saved immediately after the update. If the
// vault is locked, the hash is silently dropped (per D-14: vault write
// failure is non-fatal).
func (m *Manager) SetAuditHash(eventID, hashValue string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.payload == nil {
		return fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}
	if m.payload.AuditHashes == nil {
		m.payload.AuditHashes = make(map[string]string)
	}
	m.payload.AuditHashes[eventID] = hashValue
	return m.saveLocked()
}

// AuditHashes returns a copy of the vault's audit hash map. The caller
// is responsible for zeroing the returned map after use (D-16).
// Returns nil if the vault is locked or no hashes exist.
func (m *Manager) AuditHashes() map[string]string {
	if m.payload == nil || m.payload.AuditHashes == nil {
		return nil
	}
	cp := make(map[string]string, len(m.payload.AuditHashes))
	for k, v := range m.payload.AuditHashes {
		cp[k] = v
	}
	return cp
}

// ZeroAuditHashes overwrites all keys and values in a hash map with empty
// strings to limit sensitive data lifetime in memory (D-16).
func ZeroAuditHashes(m map[string]string) {
	for k := range m {
		m[k] = ""
		delete(m, k)
	}
}

// atomicWrite writes data to path using temp-file-rename for crash safety.
func atomicWrite(path string, data []byte) error {
	tmpPath := path + ".tmp"
	bakPath := path + ".bak"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		_ = os.Rename(path, bakPath)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		if _, bakErr := os.Stat(bakPath); bakErr == nil {
			_ = os.Rename(bakPath, path)
		}
		return fmt.Errorf("renaming temp file: %w", err)
	}

	// Zero the backup file contents before removal so the encrypted vault
	// bytes do not persist in unallocated disk blocks (defense in depth).
	if f, err := os.OpenFile(bakPath, os.O_WRONLY, 0600); err == nil {
		if info, err := f.Stat(); err == nil {
			zeros := make([]byte, info.Size())
			_, _ = f.Write(zeros)
			_ = f.Sync()
		}
		_ = f.Close()
	}
	_ = os.Remove(bakPath)
	return nil
}
