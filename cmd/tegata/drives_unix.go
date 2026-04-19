//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// isSystemVolume reports whether a macOS /Volumes entry points to the system
// root. It checks the symlink target first, then falls back to a well-known
// name for older macOS versions.
func isSystemVolume(name string) bool {
	target, err := os.Readlink(filepath.Join("/Volumes", name))
	if err == nil && target == "/" {
		return true
	}
	return name == "Macintosh HD"
}

// platformIsRemovable reports whether abs (an absolute path) resides on a
// removable drive. Uses mount-point heuristics: on macOS it checks /Volumes,
// on Linux it checks /media and /mnt.
func platformIsRemovable(abs string) bool {
	switch runtime.GOOS {
	case "darwin":
		const prefix = "/Volumes/"
		if !strings.HasPrefix(abs, prefix) {
			return false
		}
		rest := strings.TrimPrefix(abs, prefix)
		volName := strings.SplitN(rest, "/", 2)[0]
		return !isSystemVolume(volName)
	case "linux":
		return strings.HasPrefix(abs, "/media/") || strings.HasPrefix(abs, "/mnt/")
	}
	return false
}
