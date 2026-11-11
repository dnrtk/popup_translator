//go:build !windows

package hotkey

import (
	"context"
	"errors"
	"time"
)

// Watch is not implemented on this platform yet (Linux/X11 is phase 3).
func Watch(ctx context.Context, interval time.Duration, paused func() bool) (<-chan time.Time, error) {
	return nil, errors.New("global hotkey is not supported on this platform")
}
