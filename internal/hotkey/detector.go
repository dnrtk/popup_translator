// Package hotkey detects "hold Ctrl and press C twice".
//
// Detector is pure and OS independent; platform hooks feed it key events.
package hotkey

import "time"

// Virtual-key codes used by the detector (Windows values; other platforms map
// their key codes to these).
const (
	VKControl  uint32 = 0x11
	VKLControl uint32 = 0xA2
	VKRControl uint32 = 0xA3
	VKShift    uint32 = 0x10
	VKLShift   uint32 = 0xA0
	VKRShift   uint32 = 0xA1
	VKMenu     uint32 = 0x12
	VKLMenu    uint32 = 0xA4
	VKRMenu    uint32 = 0xA5
	VKLWin     uint32 = 0x5B
	VKRWin     uint32 = 0x5C
	VKC        uint32 = 0x43
)

// DefaultInterval is the default maximum time between the two C presses.
const DefaultInterval = 400 * time.Millisecond

// Event is a single key transition.
type Event struct {
	VK       uint32
	Down     bool
	Injected bool
	Time     time.Time
}

// Detector is a state machine fed with key events. It is not safe for
// concurrent use; the OS hook calls it from a single thread.
type Detector struct {
	Interval time.Duration

	lctrl, rctrl bool
	cHeld        bool
	count        int
	first        time.Time
}

// NewDetector returns a Detector with the given interval (DefaultInterval if <= 0).
func NewDetector(interval time.Duration) *Detector {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Detector{Interval: interval}
}

// Feed processes one event and reports whether the hotkey fired.
func (d *Detector) Feed(ev Event) bool {
	if ev.Injected {
		return false
	}
	switch ev.VK {
	case VKLControl, VKControl:
		d.setCtrl(&d.lctrl, ev.Down)
		return false
	case VKRControl:
		d.setCtrl(&d.rctrl, ev.Down)
		return false
	case VKShift, VKLShift, VKRShift, VKMenu, VKLMenu, VKRMenu, VKLWin, VKRWin:
		return false
	case VKC:
		if !ev.Down {
			d.cHeld = false
			return false
		}
		if d.cHeld {
			return false // auto-repeat
		}
		d.cHeld = true
		if !d.ctrlHeld() {
			d.reset()
			return false
		}
		if d.count == 0 || ev.Time.Sub(d.first) > d.Interval {
			d.count = 1
			d.first = ev.Time
			return false
		}
		d.reset()
		return true
	default:
		// Any other key between the two presses (e.g. Ctrl+C, Ctrl+V)
		// cancels the sequence.
		if ev.Down {
			d.reset()
		}
		return false
	}
}

func (d *Detector) setCtrl(flag *bool, down bool) {
	*flag = down
	if !d.ctrlHeld() {
		d.reset()
	}
}

func (d *Detector) ctrlHeld() bool { return d.lctrl || d.rctrl }

func (d *Detector) reset() {
	d.count = 0
	d.first = time.Time{}
}
