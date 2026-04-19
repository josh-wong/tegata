//go:build !windows

package drives

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IsSystemVolume reports whether a macOS /Volumes entry points to the system
// root. It checks the symlink target first, which covers all modern macOS
// versions. The "Macintosh HD" name fallback handles older macOS versions where
// the symlink may be absent; it may not match on non-English systems or renamed
// volumes — those will be treated as removable drives, which is a safe default.
func IsSystemVolume(name string) bool {
	target, err := os.Readlink(filepath.Join("/Volumes", name))
	if err == nil && target == "/" {
		return true
	}
	return name == "Macintosh HD"
}

// platformIsRemovable reports whether abs (an absolute path) resides on a
// removable drive. Uses mount-point heuristics:
//   - macOS: checks /Volumes, excluding the system root volume.
//   - Linux: checks /media only. /mnt is intentionally excluded because it is
//     commonly used for network shares and non-removable mounts.
func platformIsRemovable(abs string) bool {
	switch runtime.GOOS {
	case "darwin":
		const prefix = "/Volumes/"
		if !strings.HasPrefix(abs, prefix) {
			return false
		}
		rest := strings.TrimPrefix(abs, prefix)
		volName := strings.SplitN(rest, "/", 2)[0]
		return !IsSystemVolume(volName)
	case "linux":
		return strings.HasPrefix(abs, "/media/")
	}
	return false
}
