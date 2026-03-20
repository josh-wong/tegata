package main

import (
	"os"
	"path/filepath"
	"runtime"
)

const vaultFilename = "vault.tegata"

// scanMountedDrives looks for vault.tegata files on mounted drives. On Windows
// it checks drive letters D-Z, on macOS it checks /Volumes/*, and on Linux it
// walks /media/ and /mnt/ one level deep.
func scanMountedDrives() []VaultLocation {
	var results []VaultLocation

	switch runtime.GOOS {
	case "windows":
		for letter := 'D'; letter <= 'Z'; letter++ {
			root := string(letter) + ":\\"
			vaultPath := filepath.Join(root, vaultFilename)
			if _, err := os.Stat(vaultPath); err == nil {
				results = append(results, VaultLocation{
					Path:      vaultPath,
					DriveName: string(letter) + ":",
				})
			}
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
			vaultPath := filepath.Join("/Volumes", e.Name(), vaultFilename)
			if _, err := os.Stat(vaultPath); err == nil {
				results = append(results, VaultLocation{
					Path:      vaultPath,
					DriveName: e.Name(),
				})
			}
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

				// Check directly.
				vaultPath := filepath.Join(dir, vaultFilename)
				if _, err := os.Stat(vaultPath); err == nil {
					results = append(results, VaultLocation{
						Path:      vaultPath,
						DriveName: e.Name(),
					})
					continue
				}

				// Check one level deeper (e.g., /media/user/USB/).
				subEntries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, sub := range subEntries {
					if !sub.IsDir() {
						continue
					}
					vaultPath := filepath.Join(dir, sub.Name(), vaultFilename)
					if _, err := os.Stat(vaultPath); err == nil {
						results = append(results, VaultLocation{
							Path:      vaultPath,
							DriveName: sub.Name(),
						})
					}
				}
			}
		}
	}

	return results
}
