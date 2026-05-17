package ledgervol

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestUnlock_FirstRun_CreatesEmptyDir(t *testing.T) {
	// First run: archive doesn't exist, should create empty directory
	encPath := filepath.Join(t.TempDir(), "ledger-data.enc")
	dataDir := filepath.Join(t.TempDir(), "ledger-data")

	// Generate a random key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// First unlock should succeed and create empty dir
	if err := Unlock(encPath, dataDir, key); err != nil {
		t.Fatalf("Unlock (first run): %v", err)
	}

	// Verify data directory was created
	if stat, err := os.Stat(dataDir); err != nil || !stat.IsDir() {
		t.Errorf("data directory not created or not a directory")
	}

	// Verify no encrypted file exists yet
	if _, err := os.Stat(encPath); err == nil {
		t.Errorf("encrypted file should not exist after first-run unlock")
	}
}

func TestUnlock_ExistingArchive(t *testing.T) {
	// Setup: create encrypted archive
	encPath := filepath.Join(t.TempDir(), "ledger-data.enc")
	originalDir := filepath.Join(t.TempDir(), "original")
	dataDir := filepath.Join(t.TempDir(), "decrypted")

	if err := os.Mkdir(originalDir, 0755); err != nil {
		t.Fatalf("mkdir original: %v", err)
	}

	// Create test files in original directory
	testFiles := map[string]string{
		"base/PG_VERSION":     "16\n",
		"base/pg_control":     "binary data",
		"global/pg_filenode.map": "mapping",
	}
	for path, content := range testFiles {
		fullPath := filepath.Join(originalDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	// Generate a random key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// Lock (encrypt) the original directory
	if err := Lock(encPath, originalDir, key); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Verify original directory was deleted
	if _, err := os.Stat(originalDir); err == nil {
		t.Errorf("original directory should be deleted after lock")
	}

	// Verify encrypted file exists
	if _, err := os.Stat(encPath); err != nil {
		t.Errorf("encrypted file not found: %v", err)
	}

	// Unlock (decrypt) to new directory
	if err := Unlock(encPath, dataDir, key); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Verify decrypted files match originals
	for path, expectedContent := range testFiles {
		fullPath := filepath.Join(dataDir, path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("ReadFile %s: %v", path, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("content mismatch for %s: got %q, want %q", path, string(content), expectedContent)
		}
	}
}

func TestLockThenUnlock_RoundTrip(t *testing.T) {
	// Create original directory with test files
	originalDir := filepath.Join(t.TempDir(), "original")
	if err := os.Mkdir(originalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create nested directory structure with files
	files := []string{"README.md", "subdir/config.json", "subdir/data.txt"}
	for _, file := range files {
		path := filepath.Join(originalDir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		content := "test content: " + file
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	// Generate key and encrypt
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	encPath := filepath.Join(t.TempDir(), "data.enc")
	if err := Lock(encPath, originalDir, key); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Decrypt to new location
	decryptedDir := filepath.Join(t.TempDir(), "decrypted")
	if err := Unlock(encPath, decryptedDir, key); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Verify all files exist and have correct content
	for _, file := range files {
		originalPath := filepath.Join(originalDir, file)
		decryptedPath := filepath.Join(decryptedDir, file)

		// Original should not exist
		if _, err := os.Stat(originalPath); err == nil {
			t.Errorf("original file should not exist: %s", file)
		}

		// Decrypted should exist with correct content
		decryptedContent, err := os.ReadFile(decryptedPath)
		if err != nil {
			t.Errorf("failed to read decrypted file %s: %v", file, err)
			continue
		}

		expectedContent := "test content: " + file
		if string(decryptedContent) != expectedContent {
			t.Errorf("content mismatch for %s: got %q, want %q", file, string(decryptedContent), expectedContent)
		}
	}
}

func TestPathTraversalPrevention(t *testing.T) {
	// Create a tar archive with a malicious path traversal attempt
	encPath := filepath.Join(t.TempDir(), "malicious.enc")
	dataDir := filepath.Join(t.TempDir(), "data")

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// Create a malicious tar with path traversal
	maliciousTar := createMaliciousTar(t, []string{
		"../../../etc/passwd",
		"normal_file.txt",
		"subdir/../../dangerous.txt",
	})

	// Encrypt the malicious tar
	encrypted, err := encryptVolume(key, maliciousTar)
	if err != nil {
		t.Fatalf("encryptVolume: %v", err)
	}

	// Write encrypted file
	if err := os.WriteFile(encPath, encrypted, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Attempt to decrypt - path traversal entries should be rejected.
	// Unlock may return an error when traversal is detected; that is acceptable.
	if err := Unlock(encPath, dataDir, key); err != nil {
		t.Logf("Unlock returned expected error for traversal archive: %v", err)
	}

	// Verify that traversal attempts were blocked:
	// Files should not exist outside the data directory
	etcPasswd := filepath.Join(filepath.Dir(dataDir), "etc", "passwd")
	if _, err := os.Stat(etcPasswd); err == nil {
		t.Errorf("path traversal was not blocked: %s created outside data directory", etcPasswd)
	}

	// Normal files in the archive should be extracted
	if _, err := os.Stat(filepath.Join(dataDir, "normal_file.txt")); err != nil {
		t.Logf("note: normal_file.txt not found (may be due to tar rejection of entire archive)")
	}
}

func TestNonceRandomness(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	plaintext := []byte("test data to encrypt")

	// Encrypt the same plaintext multiple times
	ciphertext1, err := encryptVolume(key, plaintext)
	if err != nil {
		t.Fatalf("encryptVolume 1: %v", err)
	}

	ciphertext2, err := encryptVolume(key, plaintext)
	if err != nil {
		t.Fatalf("encryptVolume 2: %v", err)
	}

	ciphertext3, err := encryptVolume(key, plaintext)
	if err != nil {
		t.Fatalf("encryptVolume 3: %v", err)
	}

	// Ciphertexts should be different (due to different nonces)
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Errorf("ciphertexts should differ due to random nonces, but ciphertext1 == ciphertext2")
	}
	if bytes.Equal(ciphertext2, ciphertext3) {
		t.Errorf("ciphertexts should differ due to random nonces, but ciphertext2 == ciphertext3")
	}

	// But all should decrypt to the same plaintext
	decrypted1, err := decryptVolume(key, ciphertext1)
	if err != nil {
		t.Fatalf("decryptVolume 1: %v", err)
	}
	if !bytes.Equal(decrypted1, plaintext) {
		t.Errorf("decrypted1 mismatch: got %v, want %v", decrypted1, plaintext)
	}

	decrypted2, err := decryptVolume(key, ciphertext2)
	if err != nil {
		t.Fatalf("decryptVolume 2: %v", err)
	}
	if !bytes.Equal(decrypted2, plaintext) {
		t.Errorf("decrypted2 mismatch: got %v, want %v", decrypted2, plaintext)
	}

	decrypted3, err := decryptVolume(key, ciphertext3)
	if err != nil {
		t.Fatalf("decryptVolume 3: %v", err)
	}
	if !bytes.Equal(decrypted3, plaintext) {
		t.Errorf("decrypted3 mismatch: got %v, want %v", decrypted3, plaintext)
	}
}

func TestSymlinkSkipping(t *testing.T) {
	// Create a directory with a symlink
	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.Mkdir(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a regular file
	if err := os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a symlink (if supported on this platform)
	linkPath := filepath.Join(sourceDir, "link.txt")
	targetPath := filepath.Join(sourceDir, "file.txt")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	// Encrypt the directory
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	encPath := filepath.Join(t.TempDir(), "data.enc")
	if err := Lock(encPath, sourceDir, key); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Decrypt to new location
	decryptedDir := filepath.Join(t.TempDir(), "decrypted")
	if err := Unlock(encPath, decryptedDir, key); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Verify regular file was extracted
	regularFilePath := filepath.Join(decryptedDir, "file.txt")
	if _, err := os.Stat(regularFilePath); err != nil {
		t.Errorf("regular file not extracted: %v", err)
	}

	// Verify symlink was not extracted (should not exist)
	linkFilePath := filepath.Join(decryptedDir, "link.txt")
	if _, err := os.Stat(linkFilePath); err == nil {
		t.Errorf("symlink should not be extracted for security")
	}
}

func TestIsLocked(t *testing.T) {
	tempDir := t.TempDir()
	encPath := filepath.Join(tempDir, "data.enc")
	dataDir := filepath.Join(tempDir, "data")

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// Create source directory and encrypt it
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.Mkdir(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Lock it
	if err := Lock(encPath, sourceDir, key); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Should be locked now
	if !IsLocked(encPath, dataDir) {
		t.Errorf("IsLocked should return true when encrypted file exists and data dir doesn't")
	}

	// Unlock it
	if err := Unlock(encPath, dataDir, key); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Should no longer be locked
	if IsLocked(encPath, dataDir) {
		t.Errorf("IsLocked should return false when data dir exists (decrypted)")
	}

	// Lock again
	if err := Lock(encPath, dataDir, key); err != nil {
		t.Fatalf("Lock (second time): %v", err)
	}

	// Should be locked again
	if !IsLocked(encPath, dataDir) {
		t.Errorf("IsLocked should return true after locking again")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	key2 := make([]byte, 32)
	if _, err := rand.Read(key2); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	plaintext := []byte("secret data")

	// Encrypt with key1
	ciphertext, err := encryptVolume(key1, plaintext)
	if err != nil {
		t.Fatalf("encryptVolume: %v", err)
	}

	// Try to decrypt with key2 - should fail or produce garbage
	decrypted, err := decryptVolume(key2, ciphertext)
	if err == nil && bytes.Equal(decrypted, plaintext) {
		t.Errorf("decryption with wrong key should fail or produce different plaintext")
	}
}

func TestEncryptDecryptLargeData(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// Create large plaintext (10MB)
	plaintext := make([]byte, 10*1024*1024)
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// Encrypt
	ciphertext, err := encryptVolume(key, plaintext)
	if err != nil {
		t.Fatalf("encryptVolume: %v", err)
	}

	// Decrypt
	decrypted, err := decryptVolume(key, ciphertext)
	if err != nil {
		t.Fatalf("decryptVolume: %v", err)
	}

	// Verify
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data does not match plaintext")
	}
}

// createMaliciousTar builds a gzip-compressed tar archive whose entry names
// contain the raw path strings in paths — including traversal sequences like
// "../../../etc/passwd" — without creating any real files on the filesystem.
func createMaliciousTar(t *testing.T, paths []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, name := range paths {
		content := []byte("content")
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     name, // Intentionally raw; may include traversal sequences.
			Size:     int64(len(content)),
			Mode:     0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader %q: %v", name, err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("Write %q: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip.Close: %v", err)
	}
	return buf.Bytes()
}

// Test that zeroBytes actually zeros memory
func TestZeroBytes(t *testing.T) {
	// Create a buffer with known content
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}

	// Zero it
	zeroBytes(b)

	// Verify all bytes are zero
	for i, v := range b {
		if v != 0 {
			t.Errorf("byte at index %d is not zero after zeroBytes: %d", i, v)
		}
	}
}

// Test that encryption produces valid AES-GCM ciphertexts with proper structure
func TestEncryptionStructure(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	plaintext := []byte("test")

	// Encrypt
	ciphertext, err := encryptVolume(key, plaintext)
	if err != nil {
		t.Fatalf("encryptVolume: %v", err)
	}

	// Structure: nonce[12] || ciphertext
	if len(ciphertext) < 12 {
		t.Errorf("ciphertext too short: %d bytes (need at least 12 for nonce)", len(ciphertext))
	}

	// Verify we can decrypt it
	decrypted, err := decryptVolume(key, ciphertext)
	if err != nil {
		t.Fatalf("decryptVolume: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted plaintext does not match: got %v, want %v", decrypted, plaintext)
	}
}

// Test that invalid ciphertext is rejected during decryption
func TestDecryptInvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	// Try to decrypt corrupted data
	invalidCiphertext := []byte("not a valid ciphertext")
	_, err := decryptVolume(key, invalidCiphertext)
	if err == nil {
		t.Errorf("decryptVolume should return an error for invalid ciphertext")
	}
}

// Test with empty plaintext
func TestEncryptDecryptEmpty(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	plaintext := []byte("")

	// Encrypt empty data
	ciphertext, err := encryptVolume(key, plaintext)
	if err != nil {
		t.Fatalf("encryptVolume: %v", err)
	}

	// Decrypt
	decrypted, err := decryptVolume(key, ciphertext)
	if err != nil {
		t.Fatalf("decryptVolume: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted empty plaintext mismatch")
	}
}
