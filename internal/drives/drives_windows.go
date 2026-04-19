//go:build windows

package drives

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGetDriveType = kernel32.NewProc("GetDriveTypeW")
)

const driveRemovable = 2

// platformIsRemovable reports whether abs resides on a removable drive on
// Windows, using GetDriveTypeW.
func platformIsRemovable(abs string) bool {
	vol := filepath.VolumeName(abs)
	if vol == "" {
		return false
	}
	root := vol + "\\"
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return false
	}
	driveType, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(rootPtr)))
	return driveType == driveRemovable
}
