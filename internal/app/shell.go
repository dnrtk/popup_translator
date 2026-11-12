package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// openFolder shows dir in the system file manager.
func openFolder(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", dir, err)
	}
	go func() { _ = cmd.Wait() }() // explorer.exe exits with 1 even on success
	return nil
}

// OpenLogFolder opens %APPDATA%\Yaku\logs.
func (a *App) OpenLogFolder() error {
	dir, err := LogDir()
	if err != nil {
		return err
	}
	return openFolder(dir)
}

// AttachSettings connects the settings window.
func (a *App) AttachSettings(w *application.WebviewWindow) {
	a.settings = w
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		a.HideSettings()
	})
}

// ShowSettings shows and focuses the settings window and asks it to reload.
func (a *App) ShowSettings() {
	if a.settings == nil {
		return
	}
	// The popup is always on top and would cover the settings window.
	a.HidePopup()
	a.wapp.Event.Emit(EventSettingsOpen, true)
	application.InvokeSync(func() {
		a.settings.Show()
		forceForeground(a.settings.NativeWindow())
		a.settings.Focus()
	})
}

// HideSettings hides the settings window.
func (a *App) HideSettings() {
	if a.settings == nil {
		return
	}
	application.InvokeSync(func() { a.settings.Hide() })
}
