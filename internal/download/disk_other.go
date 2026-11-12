//go:build !windows

package download

// FreeSpace is not implemented on this platform yet; -1 means unknown.
func FreeSpace(string) int64 { return -1 }
