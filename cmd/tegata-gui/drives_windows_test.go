//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestPlatformScanRemovable_Windows(t *testing.T) {
	results := platformScanRemovable()

	// CI runners typically have no removable drives, so an empty result is
	// expected. The test verifies that the Windows kernel32 calls complete
	// without panicking and that any returned entries are well-formed.
	for _, r := range results {
		if r.Path == "" {
			t.Error("VaultLocation.Path must not be empty")
		}
		// Windows drive paths should be a letter followed by a colon.
		if len(r.Path) < 2 || r.Path[1] != ':' {
			t.Errorf("expected Windows drive letter path, got %q", r.Path)
		}
		if r.DriveName == "" {
			t.Error("VaultLocation.DriveName must not be empty")
		}
	}
}

func TestGetVolumeLabel_InvalidRoot(t *testing.T) {
	// Calling getVolumeLabel with a path that is not a valid volume root
	// should return an empty string, not panic.
	label := getVolumeLabel("Z:\\nonexistent\\")
	if label != "" {
		t.Errorf("expected empty label for invalid root, got %q", label)
	}
}

func TestScanMountedDrives_WindowsDriveLetters(t *testing.T) {
	results := scanMountedDrives()
	for _, r := range results {
		// Every result path on Windows should contain a drive letter and colon.
		if !strings.Contains(r.Path, ":") {
			t.Errorf("expected Windows-style path with colon, got %q", r.Path)
		}
	}
}
