//go:build !windows

// Package clipboard reads Unicode text from the system clipboard.
package clipboard

import "errors"

// ErrNoText means the clipboard holds no text.
var ErrNoText = errors.New("clipboard has no text")

// SequenceNumber is not available on this platform.
func SequenceNumber() uint32 { return 0 }

// ReadText is not implemented on this platform yet.
func ReadText() (string, error) {
	return "", errors.New("clipboard is not supported on this platform")
}
