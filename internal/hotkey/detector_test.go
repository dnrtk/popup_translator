package hotkey

import (
	"testing"
	"time"
)

type step struct {
	at       int // milliseconds from start
	vk       uint32
	down     bool
	injected bool
}

func run(t *testing.T, interval time.Duration, steps []step) []int {
	t.Helper()
	d := NewDetector(interval)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var fired []int
	for _, s := range steps {
		ev := Event{VK: s.vk, Down: s.down, Injected: s.injected, Time: base.Add(time.Duration(s.at) * time.Millisecond)}
		if d.Feed(ev) {
			fired = append(fired, s.at)
		}
	}
	return fired
}

func down(at int, vk uint32) step { return step{at: at, vk: vk, down: true} }
func up(at int, vk uint32) step   { return step{at: at, vk: vk} }

func TestDetector(t *testing.T) {
	tests := []struct {
		name  string
		steps []step
		want  []int
	}{
		{
			name: "basic double press fires on second press",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(60, VKC), down(200, VKC), up(250, VKC), up(300, VKLControl),
			},
			want: []int{200},
		},
		{
			name: "right ctrl works",
			steps: []step{
				down(0, VKRControl), down(10, VKC), up(60, VKC), down(150, VKC),
			},
			want: []int{150},
		},
		{
			name:  "single press does not fire",
			steps: []step{down(0, VKLControl), down(10, VKC), up(60, VKC), up(100, VKLControl)},
			want:  nil,
		},
		{
			name: "key repeat is ignored",
			steps: []step{
				down(0, VKLControl), down(10, VKC), down(40, VKC), down(70, VKC), down(100, VKC), up(130, VKC),
			},
			want: nil,
		},
		{
			name: "repeat then real second press fires",
			steps: []step{
				down(0, VKLControl), down(10, VKC), down(40, VKC), up(60, VKC), down(150, VKC),
			},
			want: []int{150},
		},
		{
			name: "releasing ctrl between presses resets",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), up(80, VKLControl),
				down(100, VKLControl), down(150, VKC),
			},
			want: nil,
		},
		{
			name: "switching from left to right ctrl while holding keeps sequence",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), down(60, VKRControl), up(70, VKLControl), down(150, VKC),
			},
			want: []int{150},
		},
		{
			name: "second press too late does not fire",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), down(500, VKC), up(550, VKC),
			},
			want: nil,
		},
		{
			name: "late press starts a new sequence",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), down(500, VKC), up(550, VKC), down(700, VKC),
			},
			want: []int{700},
		},
		{
			name: "exactly at interval fires",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), down(410, VKC),
			},
			want: []int{410},
		},
		{
			name:  "c without ctrl does not fire",
			steps: []step{down(10, VKC), up(50, VKC), down(100, VKC), up(150, VKC)},
			want:  nil,
		},
		{
			name: "injected events are ignored",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC),
				{at: 100, vk: VKC, down: true, injected: true},
			},
			want: nil,
		},
		{
			name: "other key between presses resets",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), down(100, 0x56 /* V */), up(120, 0x56), down(200, VKC),
			},
			want: nil,
		},
		{
			name: "shift between presses does not reset",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), down(100, VKLShift), down(200, VKC),
			},
			want: []int{200},
		},
		{
			name: "third press after fire starts new sequence",
			steps: []step{
				down(0, VKLControl), down(10, VKC), up(50, VKC), down(100, VKC), up(150, VKC), down(200, VKC), up(250, VKC), down(300, VKC),
			},
			want: []int{100, 300},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, 400*time.Millisecond, tt.steps)
			if len(got) != len(tt.want) {
				t.Fatalf("fired at %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("fired at %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNewDetectorDefaultInterval(t *testing.T) {
	if d := NewDetector(0); d.Interval != DefaultInterval {
		t.Errorf("Interval = %v", d.Interval)
	}
}
