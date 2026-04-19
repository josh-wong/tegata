//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetDriveType     = kernel32.NewProc("GetDriveTypeW")
	procGetVolumeInfo    = kernel32.NewProc("GetVolumeInformationW")
	procGetLogicalDrives = kernel32.NewProc("GetLogicalDrives")
)

const (
	driveRemovable = 2
)

// platformScanRemovable returns only removable drives (USB, microSD) on
// Windows, with volume labels.
func platformScanRemovable() []VaultLocation {
	mask, _, _ := procGetLogicalDrives.Call()
	var results []VaultLocation

	for i := 3; i < 26; i++ { // D=3 through Z=25
		if mask&(1<<uint(i)) == 0 {
			continue
		}

		letter := rune('A' + i)
		root := string(letter) + ":\\"
		rootPtr, _ := syscall.UTF16PtrFromString(root)

		driveType, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(rootPtr)))
		if driveType != driveRemovable {
			continue
		}

		label := getVolumeLabel(root)
		letterStr := string(letter) + ":"
		var driveName string
		if label != "" {
			driveName = label + " (" + letterStr + ")"
		} else {
			driveName = "USB/SD/microSD (" + letterStr + ")"
		}

		results = append(results, VaultLocation{
			Path:      letterStr,
			DriveName: driveName,
		})
	}

	return results
}

func getVolumeLabel(root string) string {
	rootPtr, _ := syscall.UTF16PtrFromString(root)
	var volumeName [256]uint16

	ret, _, _ := procGetVolumeInfo.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&volumeName[0])),
		uintptr(len(volumeName)),
		0, 0, 0, 0, 0,
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(volumeName[:])
}
