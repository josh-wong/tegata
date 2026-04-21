package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRemovableDrives_NoPanic(t *testing.T) {
	// platformScanRemovable must not panic on any platform, even when no
	// removable drives are connected (the usual case in CI).
	results := scanRemovableDrives()
	for _, r := range results {
		if r.Path == "" {
			t.Error("VaultLocation.Path must not be empty")
		}
		if r.DriveName == "" {
			t.Error("VaultLocation.DriveName must not be empty")
		}
	}
}

func TestScanMountedDrives_NoPanic(t *testing.T) {
	// scanMountedDrives must not panic on any platform.
	results := scanMountedDrives()
	for _, r := range results {
		if r.Path == "" {
			t.Error("VaultLocation.Path must not be empty")
		}
	}
}

func TestFindVaultsInDir_Empty(t *testing.T) {
	dir := t.TempDir()
	results := findVaultsInDir(dir, "test-drive")
	if len(results) != 0 {
		t.Errorf("expected 0 results in empty dir, got %d", len(results))
	}
}

func TestFindVaultsInDir_MatchesVaultFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a .tegata file and a non-vault file.
	if err := os.WriteFile(filepath.Join(dir, "vault.tegata"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := findVaultsInDir(dir, "USB")
	if len(results) != 1 {
		t.Fatalf("expected 1 vault, got %d", len(results))
	}
	if results[0].DriveName != "USB" {
		t.Errorf("expected DriveName 'USB', got %q", results[0].DriveName)
	}
	if filepath.Base(results[0].Path) != "vault.tegata" {
		t.Errorf("expected path ending in vault.tegata, got %q", results[0].Path)
	}
}

func TestFindVaultsInDir_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "MyVault.TEGATA"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := findVaultsInDir(dir, "SD")
	if len(results) != 1 {
		t.Fatalf("expected 1 vault for uppercase extension, got %d", len(results))
	}
}

func TestFindVaultsInDir_NonexistentDir(t *testing.T) {
	results := findVaultsInDir("/nonexistent/path/12345", "ghost")
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent dir, got %d", len(results))
	}
}

func TestFindVaultsInDir_FiltersMacOSMetadata(t *testing.T) {
	dir := t.TempDir()

	// Create a real vault file and a macOS metadata file.
	if err := os.WriteFile(filepath.Join(dir, "vault.tegata"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "._vault.tegata"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := findVaultsInDir(dir, "USB")
	if len(results) != 1 {
		t.Fatalf("expected 1 vault (metadata files excluded), got %d", len(results))
	}
	if filepath.Base(results[0].Path) != "vault.tegata" {
		t.Errorf("expected vault.tegata, got %q", filepath.Base(results[0].Path))
	}
}
