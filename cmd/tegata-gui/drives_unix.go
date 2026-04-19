//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/josh-wong/tegata/internal/drives"
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
			if !e.IsDir() || drives.IsSystemVolume(e.Name()) {
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
