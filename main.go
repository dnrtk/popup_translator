// Command yaku is a local popup translator powered by PLaMo 3 Translate.
//
// main.go lives at the module root (not cmd/app) because Wails embeds
// frontend/dist relative to this package and links the generated .syso
// resource from the root directory. See docs/FINDINGS.md.
package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/dnrtk/popup_translator/internal/app"
	"github.com/dnrtk/popup_translator/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "yaku:", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, cfgErr := config.Load(cfgPath)

	logger, closeLog := newLogger(cfg.Debug)
	defer closeLog()
	if cfgErr != nil {
		logger.Error("load config; using defaults", "path", cfgPath, "err", cfgErr)
	}

	core := app.New(cfgPath, cfg, logger)

	wapp := application.New(application.Options{
		Name:        "Yaku",
		Description: "Local popup translator powered by PLaMo 3 Translate",
		Services: []application.Service{
			application.NewService(app.NewPopupService(core)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			// Windows: named mutex "com.dnrtk.yaku-sim".
			UniqueID: "com.dnrtk.yaku",
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		OnShutdown: core.Shutdown,
	})

	geo := cfg.Popup
	popup := wapp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:        "popup",
		Title:       "Yaku",
		Width:       geo.Width,
		Height:      geo.Height,
		MinWidth:    320,
		MinHeight:   220,
		Hidden:      true,
		AlwaysOnTop: true,
		URL:         "/",
	})

	tray := wapp.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Yaku - 起動中")
	menu := wapp.NewMenu()
	menu.Add("終了").OnClick(func(*application.Context) { wapp.Quit() })
	tray.SetMenu(menu)

	core.Attach(wapp, popup, tray)
	core.Start()

	return wapp.Run()
}

// newLogger writes to %APPDATA%\Yaku\logs\yaku.log (falls back to stderr).
func newLogger(debug bool) (*slog.Logger, func()) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	dir, err := app.LogDir()
	if err == nil {
		err = os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, opts)), func() {}
	}
	// Append (a second instance runs this before Wails' single-instance
	// check exits it) and rotate once the file grows past 1 MiB.
	path := filepath.Join(dir, "yaku.log")
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, opts)), func() {}
	}
	return slog.New(slog.NewTextHandler(f, opts)), func() { f.Close() }
}
