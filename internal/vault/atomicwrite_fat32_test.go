//go:build fat32 && linux

// Package vault - FAT32 atomic write integration test.
//
// This test requires Linux with root privileges (for loopback mount).
// It is NOT included in CI or default go test runs (guarded by the fat32 build tag).
//
// Run with:
//
//	sudo go test ./internal/vault/ -tags fat32 -run TestAtomicWrite -count=1 -v
//
// Verifies SECURITY-AUDIT.md Area 3 NOTE N-2: FAT32 does not support
// atomic rename, so the backup-and-rename strategy must handle partial
// failures gracefully.
package vault

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// setupFAT32 creates a 10 MB FAT32 disk image, formats it, and mounts it
// via loopback. It returns the mount directory path. Cleanup (unmount and
// removal) is registered via t.Cleanup.
func setupFAT32(t *testing.T) string {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("FAT32 loopback test requires Linux")
	}
	if os.Getuid() != 0 {
		t.Skip("FAT32 loopback test requires root (for mount)")
	}

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "fat32.img")
	mountDir := filepath.Join(tmpDir, "mnt")

	if err := os.MkdirAll(mountDir, 0755); err != nil {
		t.Fatalf("creating mount dir: %v", err)
	}

	// Create a 10 MB disk image.
	dd := exec.Command("dd", "if=/dev/zero", "of="+imgPath, "bs=1M", "count=10")
	dd.Stderr = nil
	if out, err := dd.CombinedOutput(); err != nil {
		t.Fatalf("dd failed: %v\n%s", err, out)
	}

	// Format as FAT32 (vfat).
	mkfs := exec.Command("mkfs.vfat", imgPath)
	if out, err := mkfs.CombinedOutput(); err != nil {
		t.Fatalf("mkfs.vfat failed: %v\n%s", err, out)
	}

	// Mount the image via loopback.
	if err := syscall.Mount(imgPath, mountDir, "vfat", 0, ""); err != nil {
		t.Fatalf("mount failed: %v", err)
	}

	t.Cleanup(func() {
		_ = syscall.Unmount(mountDir, 0)
	})

	return mountDir
}

func TestAtomicWrite_FAT32_HappyPath(t *testing.T) {
	mountDir := setupFAT32(t)
	vaultPath := filepath.Join(mountDir, "test.vault")

	// Write initial data.
	if err := atomicWrite(vaultPath, []byte("original-data")); err != nil {
		t.Fatalf("first atomicWrite failed: %v", err)
	}

	got, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("reading vault after first write: %v", err)
	}
	if string(got) != "original-data" {
		t.Fatalf("expected 'original-data', got %q", got)
	}

	// Overwrite with new data.
	if err := atomicWrite(vaultPath, []byte("updated-data")); err != nil {
		t.Fatalf("second atomicWrite failed: %v", err)
	}

	got, err = os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("reading vault after second write: %v", err)
	}
	if string(got) != "updated-data" {
		t.Fatalf("expected 'updated-data', got %q", got)
	}

	// Verify no residual .bak or .tmp files remain.
	if _, err := os.Stat(vaultPath + ".bak"); !os.IsNotExist(err) {
		t.Fatalf(".bak file should not exist after successful write")
	}
	if _, err := os.Stat(vaultPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf(".tmp file should not exist after successful write")
	}
}

func TestAtomicWrite_FAT32_RecoveryFromMissingTmp(t *testing.T) {
	mountDir := setupFAT32(t)
	vaultPath := filepath.Join(mountDir, "test.vault")

	// Write initial data.
	if err := atomicWrite(vaultPath, []byte("original-data")); err != nil {
		t.Fatalf("initial atomicWrite failed: %v", err)
	}

	// Simulate a previous failed write: .bak exists (copy of original),
	// but .tmp was already cleaned up or never completed.
	bakPath := vaultPath + ".bak"
	if err := os.WriteFile(bakPath, []byte("original-data"), 0600); err != nil {
		t.Fatalf("creating .bak file: %v", err)
	}

	// Now call atomicWrite with new data. It should succeed and handle
	// the pre-existing .bak file from the simulated failure.
	if err := atomicWrite(vaultPath, []byte("new-data")); err != nil {
		t.Fatalf("atomicWrite after simulated failure: %v", err)
	}

	got, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("reading vault after recovery write: %v", err)
	}
	if string(got) != "new-data" {
		t.Fatalf("expected 'new-data', got %q", got)
	}
}

func TestAtomicWrite_FAT32_BackupRecovery(t *testing.T) {
	mountDir := setupFAT32(t)
	vaultPath := filepath.Join(mountDir, "test.vault")

	// Write initial data.
	if err := atomicWrite(vaultPath, []byte("original-data")); err != nil {
		t.Fatalf("initial atomicWrite failed: %v", err)
	}

	// Simulate a crash scenario: primary vault file is gone, but .bak
	// remains from a previous backup step.
	bakPath := vaultPath + ".bak"
	if err := os.WriteFile(bakPath, []byte("original-data"), 0600); err != nil {
		t.Fatalf("creating .bak file: %v", err)
	}
	if err := os.Remove(vaultPath); err != nil {
		t.Fatalf("removing primary vault file: %v", err)
	}

	// atomicWrite should succeed: no existing file means no rename-to-backup
	// step, so the .tmp -> primary rename proceeds directly.
	if err := atomicWrite(vaultPath, []byte("recovered-data")); err != nil {
		t.Fatalf("atomicWrite during recovery scenario: %v", err)
	}

	got, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("reading vault after backup recovery: %v", err)
	}
	if string(got) != "recovered-data" {
		t.Fatalf("expected 'recovered-data', got %q", got)
	}
}
