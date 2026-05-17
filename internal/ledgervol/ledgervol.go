// Package ledgervol manages the AES-256-GCM encrypted backing volume for the
// ScalarDL ledger data directory. The encrypted archive is stored on disk at a
// fixed path; on vault unlock the archive is decrypted into a plaintext
// directory that the Docker postgres container accesses via a bind mount. On
// vault lock the plaintext directory is re-encrypted and deleted.
//
// Format: nonce[12] || AES-256-GCM-ciphertext, where the plaintext is a
// gzip-compressed tar archive of the data directory contents.
//
// The encryption key is a 32-byte random key generated once during
// `tegata ledger start` and stored in the vault's encrypted key storage as
// "audit.ledger_volume_key". It is loaded from the vault on every unlock and
// kept in memory while the vault is open. It is never written to disk in
// plaintext or to tegata.toml.
package ledgervol

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const nonceSize = 12 // GCM standard nonce size in bytes

// Unlock decrypts the encrypted archive at encPath into dataDir.
// If encPath does not exist (first-time setup), Unlock just creates dataDir so
// the postgres container can initialize a fresh database there.
// key must be exactly 32 bytes (AES-256).
func Unlock(encPath, dataDir string, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("ledger volume key must be 32 bytes, got %d", len(key))
	}

	if _, err := os.Stat(encPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// First-time setup: no archive yet. Create the empty data directory
			// so postgres can initialize into it.
			if mkErr := os.MkdirAll(dataDir, 0700); mkErr != nil {
				return fmt.Errorf("creating ledger data directory: %w", mkErr)
			}
			return nil
		}
		return fmt.Errorf("checking ledger volume: %w", err)
	}

	// Archive exists. Remove any stale plaintext directory and recreate it
	// cleanly before extracting so there are no leftover files from a
	// previous session.
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("clearing stale ledger data directory: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("creating ledger data directory: %w", err)
	}

	data, err := os.ReadFile(encPath)
	if err != nil {
		return fmt.Errorf("reading ledger volume: %w", err)
	}

	plaintext, err := decryptVolume(key, data)
	if err != nil {
		return fmt.Errorf("decrypting ledger volume (wrong key or corrupted file): %w", err)
	}
	defer zeroBytes(plaintext)

	if err := untarGzip(plaintext, dataDir); err != nil {
		return fmt.Errorf("extracting ledger volume: %w", err)
	}
	return nil
}

// Lock encrypts dataDir into encPath as a gzip-compressed tar archive, then
// removes the plaintext dataDir. If dataDir does not exist or is empty (nothing
// has been written by postgres yet), Lock stores an empty archive so the file
// always exists after the first ledger start.
// key must be exactly 32 bytes (AES-256).
func Lock(encPath, dataDir string, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("ledger volume key must be 32 bytes, got %d", len(key))
	}

	if _, err := os.Stat(dataDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Nothing to encrypt.
			return nil
		}
		return fmt.Errorf("checking ledger data directory: %w", err)
	}

	plaintext, err := tarGzip(dataDir)
	if err != nil {
		return fmt.Errorf("archiving ledger data: %w", err)
	}
	defer zeroBytes(plaintext)

	ciphertext, err := encryptVolume(key, plaintext)
	if err != nil {
		return fmt.Errorf("encrypting ledger data: %w", err)
	}

	// Write atomically using a temp file and rename.
	tmp := encPath + ".tmp"
	if writeErr := os.WriteFile(tmp, ciphertext, 0600); writeErr != nil {
		return fmt.Errorf("writing ledger volume: %w", writeErr)
	}
	if renameErr := os.Rename(tmp, encPath); renameErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("finalizing ledger volume: %w", renameErr)
	}

	// Remove the plaintext data directory now that the ciphertext is safely on disk.
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("removing plaintext ledger data: %w", err)
	}
	return nil
}

// IsLocked returns true when the encrypted archive exists but the plaintext
// data directory does not. Returns false when neither exists (not initialized),
// or when the data directory is present (currently unlocked/mounted).
func IsLocked(encPath, dataDir string) bool {
	_, encErr := os.Stat(encPath)
	_, datErr := os.Stat(dataDir)
	return encErr == nil && errors.Is(datErr, os.ErrNotExist)
}

// encryptVolume encrypts plaintext with AES-256-GCM using a random nonce.
// Returns nonce[12] || GCM-ciphertext.
func encryptVolume(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	// Seal appends the ciphertext to nonce, so the result is nonce || ciphertext.
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptVolume decrypts data = nonce[12] || GCM-ciphertext with AES-256-GCM.
func decryptVolume(key, data []byte) ([]byte, error) {
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ledger volume data too short to contain nonce")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := data[:nonceSize]
	ct := data[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// tarGzip creates a gzip-compressed tar archive of the directory at dir and
// returns the archive bytes. Symbolic links are skipped.
func tarGzip(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Skip symbolic links to avoid security issues and cross-device complexity.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !d.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// untarGzip extracts a gzip-compressed tar archive into dir. Path traversal
// attempts in archive entries are rejected with an error.
func untarGzip(data []byte, dir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

	// Ensure dir ends with the path separator for prefix checks below.
	cleanDir := filepath.Clean(dir)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dir, filepath.FromSlash(hdr.Name))

		// Security: reject any entry that escapes the destination directory.
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDir+string(os.PathSeparator)) &&
			filepath.Clean(target) != cleanDir {
			return fmt.Errorf("path traversal in ledger archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		default:
			// Skip non-regular entries (symlinks, fifos, etc.) for safety.
		}
	}
	return nil
}

// zeroBytes overwrites a byte slice with zeroes to limit the lifetime of
// sensitive data on the heap.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
