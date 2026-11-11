// Command yaku is a local popup translator powered by PLaMo 3 Translate.
//
// main.go lives at the module root (not cmd/app) because Wails embeds
// frontend/dist relative to this package and links the generated .syso
// resource from the root directory. See docs/FINDINGS.md.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "Yaku",
		Description: "Local popup translator powered by PLaMo 3 Translate",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Windows: application.WindowsOptions{
			// Keep running in the tray even when the popup is hidden.
			DisableQuitOnLastWindowClosed: true,
		},
	})

	popup := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:        "popup",
		Title:       "Yaku",
		Width:       520,
		Height:      360,
		Hidden:      true,
		AlwaysOnTop: true,
		URL:         "/",
	})

	menu := app.NewMenu()
	menu.Add("Show popup (debug)").OnClick(func(*application.Context) {
		popup.Show()
		popup.Focus()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		app.Quit()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip("Yaku")
	tray.SetMenu(menu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
