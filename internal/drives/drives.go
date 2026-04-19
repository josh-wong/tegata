// Package drives provides heuristic detection of removable storage devices
// (USB drives, microSD cards, etc.). Detection is platform-specific and
// best-effort; it never blocks vault creation — only provides advisory input.
package drives

import "path/filepath"

// IsRemovablePath reports whether path appears to reside on a removable drive
// (USB, microSD, etc.). Detection is heuristic and platform-specific; it
// returns false when the check cannot be performed.
func IsRemovablePath(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return platformIsRemovable(abs)
}
