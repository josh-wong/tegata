package main

import "github.com/josh-wong/tegata/internal/drives"

// isRemovablePath reports whether path appears to reside on a removable drive
// (USB, microSD, etc.). Detection is heuristic and platform-specific; it
// returns false when the check cannot be performed.
func isRemovablePath(path string) bool {
	return drives.IsRemovablePath(path)
}
