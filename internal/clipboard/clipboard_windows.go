//go:build windows

// Package clipboard reads Unicode text from the system clipboard.
package clipboard

import (
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
)

const cfUnicodeText = 13

// ErrNoText means the clipboard holds no text.
var ErrNoText = errors.New("clipboard has no text")

// SequenceNumber returns the clipboard sequence number, which changes every
// time the clipboard content changes.
func SequenceNumber() uint32 {
	r, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(r)
}

// ReadText reads CF_UNICODETEXT, retrying OpenClipboard while another
// application holds the clipboard.
func ReadText() (string, error) {
	const (
		attempts = 10
		backoff  = 20 * time.Millisecond
	)
	var lastErr error
	for i := 0; i < attempts; i++ {
		s, err := readOnce()
		if err == nil || errors.Is(err, ErrNoText) {
			return s, err
		}
		lastErr = err
		time.Sleep(backoff)
	}
	return "", lastErr
}

func readOnce() (string, error) {
	// OpenClipboard/CloseClipboard must run on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if r, _, err := procOpenClipboard.Call(0); r == 0 {
		return "", fmt.Errorf("OpenClipboard: %w", err)
	}
	defer procCloseClipboard.Call()

	if r, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText); r == 0 {
		return "", ErrNoText
	}
	h, _, err := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", fmt.Errorf("GetClipboardData: %w", err)
	}
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		return "", fmt.Errorf("GlobalLock: %w", err)
	}
	defer procGlobalUnlock.Call(h)

	size, _, _ := procGlobalSize.Call(h)
	n := int(size / 2)
	if n == 0 {
		return "", nil
	}
	buf := unsafe.Slice((*uint16)(unsafe.Pointer(p)), n)
	// Stop at the first NUL; GlobalSize may include padding.
	for i, c := range buf {
		if c == 0 {
			buf = buf[:i]
			break
		}
	}
	return windows.UTF16ToString(buf), nil
}
