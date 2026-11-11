//go:build devtrigger

package app

import (
	"os"
	"time"
)

// Development aid (go build -tags devtrigger): simulates the hotkey after
// YAKU_DEV_TRIGGER_DELAY (default 5s), because the real hook deliberately
// ignores synthetic key input. Never part of release builds.
func init() {
	devTrigger = func(a *App) {
		delay := 5 * time.Second
		if d, err := time.ParseDuration(os.Getenv("YAKU_DEV_TRIGGER_DELAY")); err == nil {
			delay = d
		}
		go func() {
			time.Sleep(delay)
			a.log.Info("dev trigger fired")
			a.onHotkey()
		}()
	}
}
