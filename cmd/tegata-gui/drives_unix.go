//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
)

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
			if !e.IsDir() || e.Name() == "Macintosh HD" {
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
				results = append(results, VaultLocation{
					Path:      filepath.Join(base, e.Name()),
					DriveName: e.Name(),
				})
			}
		}
	}

	return results
}
