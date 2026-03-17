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
	"time"

	"github.com/google/uuid"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/crypto/guard"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

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
// Concurrency: Manager assumes single-process access per vault file. Each CLI
// invocation opens, operates, and closes the vault independently. No file
// locking is performed; concurrent writers will corrupt the vault.
type Manager struct {
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
	if m.dek != nil {
		m.dek.Destroy()
		m.dek = nil
	}
	m.payload = nil
}

// AddCredential adds a credential to the vault and saves. Returns the assigned
// credential ID.
func (m *Manager) AddCredential(cred model.Credential) (string, error) {
	if m.payload == nil {
		return "", fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
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
	if err := m.Save(); err != nil {
		return "", err
	}
	return cred.ID, nil
}

// RemoveCredential removes a credential by ID and saves.
func (m *Manager) RemoveCredential(id string) error {
	if m.payload == nil {
		return fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}
	for i, c := range m.payload.Credentials {
		if c.ID == id {
			m.payload.Credentials = append(m.payload.Credentials[:i], m.payload.Credentials[i+1:]...)
			m.payload.ModifiedAt = time.Now().UTC()
			return m.Save()
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
	if m.payload == nil {
		return fmt.Errorf("vault not unlocked: %w", errors.ErrVaultLocked)
	}
	for i := range m.payload.Credentials {
		if m.payload.Credentials[i].ID == cred.ID {
			cred.ModifiedAt = time.Now().UTC()
			m.payload.Credentials[i] = *cred
			m.payload.ModifiedAt = cred.ModifiedAt
			return m.Save()
		}
	}
	return fmt.Errorf("credential %q: %w", cred.ID, errors.ErrNotFound)
}

// Header returns the vault header for inspecting rate-limit state.
func (m *Manager) Header() *model.VaultHeader {
	return m.header
}

// Save re-encrypts and writes the vault to disk using temp-file-rename.
func (m *Manager) Save() error {
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
