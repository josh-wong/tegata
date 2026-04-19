package main

import "path/filepath"

// isRemovablePath reports whether path appears to reside on a removable drive
// (USB, microSD, etc.). Detection is heuristic and platform-specific; it
// returns false when the check cannot be performed.
func isRemovablePath(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return platformIsRemovable(abs)
}
