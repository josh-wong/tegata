package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/josh-wong/tegata/internal/drives"
)

const vaultExtension = ".tegata"

// scanRemovableDrives returns mounted removable/USB drives regardless of
// whether they contain vault files. Used during vault creation to offer
// drive selection. Platform-specific implementation in drives_windows.go
// and drives_unix.go.
func scanRemovableDrives() []VaultLocation {
	return platformScanRemovable()
}

// isRemovablePath reports whether path appears to reside on a removable drive
// (USB, microSD, etc.). Detection is heuristic and platform-specific; it
// returns false when the check cannot be performed.
func isRemovablePath(path string) bool {
	return drives.IsRemovablePath(path)
}

// scanMountedDrives looks for *.tegata files on mounted drives. On Windows it
// checks drive letters D-Z, on macOS it checks /Volumes/*, and on Linux it
// walks /media/ and /mnt/ one level deep.
func scanMountedDrives() []VaultLocation {
	var results []VaultLocation

	switch runtime.GOOS {
	case "windows":
		for letter := 'D'; letter <= 'Z'; letter++ {
			root := string(letter) + ":\\"
			results = append(results, findVaultsInDir(root, string(letter)+":")...)
		}

	case "darwin":
		entries, err := os.ReadDir("/Volumes")
		if err != nil {
			return results
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join("/Volumes", e.Name())
			results = append(results, findVaultsInDir(dir, e.Name())...)
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
				dir := filepath.Join(base, e.Name())
				results = append(results, findVaultsInDir(dir, e.Name())...)

				// Check one level deeper (e.g., /media/user/USB/).
				subEntries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, sub := range subEntries {
					if !sub.IsDir() {
						continue
					}
					subDir := filepath.Join(dir, sub.Name())
					results = append(results, findVaultsInDir(subDir, sub.Name())...)
				}
			}
		}
	}

	return results
}

// findVaultsInDir returns VaultLocation entries for all *.tegata files in dir.
func findVaultsInDir(dir, driveName string) []VaultLocation {
	var results []VaultLocation
	entries, err := os.ReadDir(dir)
	if err != nil {
		return results
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), vaultExtension) {
			results = append(results, VaultLocation{
				Path:      filepath.Join(dir, e.Name()),
				DriveName: driveName,
			})
		}
	}
	return results
}
