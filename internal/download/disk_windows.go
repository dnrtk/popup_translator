//go:build windows

package download

import (
	"os"

	"golang.org/x/sys/windows"
)

// FreeSpace returns the bytes available to the user on the volume of dir
// (or its nearest existing parent), or -1 if unknown.
func FreeSpace(dir string) int64 {
	for dir != "" {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		parent := parentDir(dir)
		if parent == dir {
			return -1
		}
		dir = parent
	}
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return -1
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return -1
	}
	return int64(free)
}
