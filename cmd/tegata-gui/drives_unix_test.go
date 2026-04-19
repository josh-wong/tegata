//go:build !windows

package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestPlatformScanRemovable_Unix(t *testing.T) {
	// CI runners typically have no removable drives, so an empty result is
	// expected. The test verifies that OS-specific directory reads complete
	// without panicking and that any returned entries are well-formed.
	results := platformScanRemovable()

	for _, r := range results {
		if r.Path == "" {
			t.Error("VaultLocation.Path must not be empty")
		}
		if r.DriveName == "" {
			t.Error("VaultLocation.DriveName must not be empty")
		}

		// Verify platform-appropriate path prefixes.
		switch runtime.GOOS {
		case "darwin":
			if !strings.HasPrefix(r.Path, "/Volumes/") {
				t.Errorf("expected macOS path under /Volumes/, got %q", r.Path)
			}
		case "linux":
			if !strings.HasPrefix(r.Path, "/media/") && !strings.HasPrefix(r.Path, "/mnt/") {
				t.Errorf("expected Linux path under /media/ or /mnt/, got %q", r.Path)
			}
		}
	}
}
