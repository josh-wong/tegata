//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// isSystemVolume checks whether a macOS /Volumes entry is the system root.
func isSystemVolume(name string) bool {
	target, err := os.Readlink(filepath.Join("/Volumes", name))
	if err == nil && target == "/" {
		return true
	}
	// Fall back to well-known name for older macOS versions.
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

// platformScanRemovable returns removable drives on macOS and Linux.
func platformScanRemovable() []VaultLocation {
	var results []VaultLocation

	switch runtime.GOOS {
	case "darwin":
		entries, err := os.ReadDir("/Volumes")
		if err != nil {
			return results
		}
		for _, e := range entries {
			if !e.IsDir() || isSystemVolume(e.Name()) {
				continue
			}
			results = append(results, VaultLocation{
				Path:      filepath.Join("/Volumes", e.Name()),
				DriveName: e.Name(),
			})
		}

	case "linux":
		for _, base := range []string{"/media", "/mnt"} {
			entries, err := os.ReadDir(base)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				topDir := filepath.Join(base, e.Name())
				results = append(results, VaultLocation{
					Path:      topDir,
					DriveName: e.Name(),
				})

				// Check one level deeper (e.g., /media/user/USB/).
				subEntries, err := os.ReadDir(topDir)
				if err != nil {
					continue
				}
				for _, sub := range subEntries {
					if !sub.IsDir() {
						continue
					}
					results = append(results, VaultLocation{
						Path:      filepath.Join(topDir, sub.Name()),
						DriveName: sub.Name(),
					})
				}
			}
		}
	}

	return results
}
