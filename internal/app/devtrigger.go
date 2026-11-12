//go:build devtrigger

package app

import (
	"os"
	"time"
)

// Development aid (go build -tags devtrigger). The real hook deliberately
// ignores synthetic key input, so automated checks use these instead:
//
//   - after YAKU_DEV_TRIGGER_DELAY (default 5s): open the settings window if
//     YAKU_DEV_OPEN_SETTINGS is set, otherwise simulate the hotkey once;
//   - if YAKU_DEV_TRIGGER_FILE is set: simulate the hotkey whenever that file
//     appears (the file is deleted after each trigger).
//
// Never part of release builds.
func init() {
	devTrigger = func(a *App) {
		delay := 5 * time.Second
		if d, err := time.ParseDuration(os.Getenv("YAKU_DEV_TRIGGER_DELAY")); err == nil {
			delay = d
		}
		go func() {
			time.Sleep(delay)
			if os.Getenv("YAKU_DEV_OPEN_SETTINGS") != "" {
				a.log.Info("dev trigger: open settings")
				a.ShowSettings()
				return
			}
			if os.Getenv("YAKU_DEV_TRIGGER_FILE") == "" {
				a.log.Info("dev trigger fired")
				a.onHotkey()
			}
		}()
		if path := os.Getenv("YAKU_DEV_TRIGGER_FILE"); path != "" {
			go func() {
				for range time.Tick(300 * time.Millisecond) {
					if _, err := os.Stat(path); err == nil {
						_ = os.Remove(path)
						a.log.Info("dev trigger file fired")
						a.onHotkey()
					}
				}
			}()
		}
	}
}
