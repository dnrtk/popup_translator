//go:build windows

package app

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procSetFocus                 = user32.NewProc("SetFocus")
	procShowWindow               = user32.NewProc("ShowWindow")
)

const swShow = 5

// cursorPos returns the cursor position in physical pixels (the process is
// per-monitor DPI aware via the Wails manifest).
func cursorPos() (int, int, bool) {
	var pt struct{ X, Y int32 }
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return 0, 0, false
	}
	return int(pt.X), int(pt.Y), true
}

// forceForeground brings hwnd to the foreground even though the user's input
// went to another application. Windows only lets the foreground thread change
// the foreground window, so we briefly attach to its input queue. Must be
// called on the thread that owns hwnd (the Wails main thread).
func forceForeground(hwndPtr unsafe.Pointer) {
	hwnd := uintptr(hwndPtr)
	if hwnd == 0 {
		return
	}
	fg, _, _ := procGetForegroundWindow.Call()
	cur := uintptr(windows.GetCurrentThreadId())
	var fgThread uintptr
	if fg != 0 && fg != hwnd {
		fgThread, _, _ = procGetWindowThreadProcessId.Call(fg, 0)
	}
	if fgThread != 0 && fgThread != cur {
		if r, _, _ := procAttachThreadInput.Call(cur, fgThread, 1); r != 0 {
			defer procAttachThreadInput.Call(cur, fgThread, 0)
		}
	}
	procShowWindow.Call(hwnd, swShow)
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	procSetFocus.Call(hwnd)
}
