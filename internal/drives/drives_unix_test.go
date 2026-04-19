//go:build !windows

package drives

import (
	"runtime"
	"testing"
)

func TestIsSystemVolume_MacintoshHD(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("IsSystemVolume check only meaningful on macOS")
	}
	// On most macOS hosts /Volumes/Macintosh HD is a symlink to "/", so the
	// Readlink path fires first. The hard-coded name fallback only activates
	// when Readlink fails (older macOS or unusual disk names).
	if !IsSystemVolume("Macintosh HD") {
		t.Error("expected 'Macintosh HD' to be identified as system volume")
	}
}

func TestIsSystemVolume_NonSystem(t *testing.T) {
	// A random name should not be considered a system volume. This test runs on
	// all non-Windows platforms: on Linux the Readlink call will fail (no
	// /Volumes directory) and the name won't match "Macintosh HD", so the
	// function correctly returns false without any OS-specific behavior.
	if IsSystemVolume("MY_USB_DRIVE") {
		t.Error("expected 'MY_USB_DRIVE' not to be identified as system volume")
	}
}

func TestPlatformIsRemovable_Linux_MediaPrefix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific heuristic test")
	}
	// /media paths should be considered removable on Linux.
	if !platformIsRemovable("/media/user/MYUSB") {
		t.Error("expected /media/user/MYUSB to be removable")
	}
}

func TestPlatformIsRemovable_Linux_RunMediaPrefix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific heuristic test")
	}
	// /run/media paths should be considered removable on Linux (udisks2 mount
	// point used by Fedora and newer Ubuntu).
	if !platformIsRemovable("/run/media/user/MYUSB") {
		t.Error("expected /run/media/user/MYUSB to be removable")
	}
}

func TestPlatformIsRemovable_Linux_MntExcluded(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific heuristic test")
	}
	// /mnt paths must not be treated as removable — they are commonly used
	// for network shares and non-removable mounts.
	if platformIsRemovable("/mnt/nas") {
		t.Error("expected /mnt/nas not to be removable")
	}
}

func TestPlatformIsRemovable_SystemPath(t *testing.T) {
	// Common system paths must not be considered removable.
	systemPaths := []string{"/home/user/docs", "/Users/alice/vault", "/tmp/test"}
	for _, p := range systemPaths {
		if platformIsRemovable(p) {
			t.Errorf("expected %q not to be removable", p)
		}
	}
}
