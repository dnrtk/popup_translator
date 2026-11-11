//go:build windows

package hotkey

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
)

const (
	whKeyboardLL  = 13
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmQuit        = 0x0012
	llkhfInjected = 0x00000010
	llkhfLowerIL  = 0x00000002 // injected from a lower integrity process
	hcAction      = 0
)

type kbdllhookstruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// The hook callback cannot carry a closure through the Win32 API, so the
// active watcher lives in package state. Only one watcher may run at a time.
var (
	activeMu sync.Mutex
	active   *watcher
	hookProc = windows.NewCallback(lowLevelKeyboardProc)
)

type watcher struct {
	detector *Detector
	out      chan<- time.Time
	paused   func() bool
}

// Watch installs a WH_KEYBOARD_LL hook on a dedicated OS thread and sends on
// the returned channel every time the hotkey fires. Keys are never
// swallowed. The hook is removed when ctx is cancelled. paused may be nil.
func Watch(ctx context.Context, interval time.Duration, paused func() bool) (<-chan time.Time, error) {
	out := make(chan time.Time, 1)
	w := &watcher{detector: NewDetector(interval), out: out, paused: paused}

	activeMu.Lock()
	if active != nil {
		activeMu.Unlock()
		return nil, fmt.Errorf("hotkey watcher already running")
	}
	active = w
	activeMu.Unlock()

	type started struct {
		tid uint32
		err error
	}
	startCh := make(chan started, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer func() {
			activeMu.Lock()
			active = nil
			activeMu.Unlock()
			close(out)
		}()

		hook, _, callErr := procSetWindowsHookExW.Call(whKeyboardLL, hookProc, 0, 0)
		if hook == 0 {
			startCh <- started{err: fmt.Errorf("SetWindowsHookExW: %w", callErr)}
			return
		}
		defer procUnhookWindowsHookEx.Call(hook)
		startCh <- started{tid: windows.GetCurrentThreadId()}

		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
				return
			}
		}
	}()

	st := <-startCh
	if st.err != nil {
		return nil, st.err
	}
	go func() {
		<-ctx.Done()
		procPostThreadMessageW.Call(uintptr(st.tid), wmQuit, 0, 0)
	}()
	return out, nil
}

// lowLevelKeyboardProc must return quickly: it only updates the detector and
// does a non-blocking send.
func lowLevelKeyboardProc(nCode uintptr, wParam uintptr, lParam uintptr) uintptr {
	// nCode is a C int; only the low 32 bits are meaningful.
	if int32(nCode) == hcAction {
		kb := (*kbdllhookstruct)(unsafe.Pointer(lParam))
		var down bool
		valid := true
		switch wParam {
		case wmKeyDown, wmSysKeyDown:
			down = true
		case wmKeyUp, wmSysKeyUp:
		default:
			valid = false
		}
		// active is only written by the hook thread's start/exit and Watch,
		// both of which happen while no callback is running on this thread.
		w := active
		if valid && w != nil {
			ev := Event{
				VK:       kb.VkCode,
				Down:     down,
				Injected: kb.Flags&(llkhfInjected|llkhfLowerIL) != 0,
				Time:     time.Now(),
			}
			if w.detector.Feed(ev) && (w.paused == nil || !w.paused()) {
				select {
				case w.out <- ev.Time:
				default:
				}
			}
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, nCode, wParam, lParam)
	return r
}
