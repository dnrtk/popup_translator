package app

import (
	"fmt"
	"runtime"

	"github.com/dnrtk/popup_translator/internal/config"
)

func runtimeNumCPU() int { return runtime.NumCPU() }

// SettingsService is bound to the settings window.
type SettingsService struct {
	app *App
}

// NewSettingsService returns the frontend binding for the settings window.
func NewSettingsService(a *App) *SettingsService { return &SettingsService{app: a} }

// Load returns the current settings, available models and GPU devices.
func (s *SettingsService) Load() SettingsData {
	return s.app.settingsData()
}

// Save stores the settings. Restarted is true when llama-server is being
// restarted because the device, model, context size or threads changed.
func (s *SettingsService) Save(cfg config.Config, autostart bool) (SaveResult, error) {
	return s.app.applySettings(cfg, autostart)
}

// Defaults returns the default configuration (for a "reset" button).
func (s *SettingsService) Defaults() config.Config {
	return config.Default()
}

// ServerState returns the llama-server state.
func (s *SettingsService) ServerState() ServerStateEvent {
	return s.app.ServerState()
}

// OpenModelsFolder opens the first existing models directory in Explorer.
func (s *SettingsService) OpenModelsFolder() error {
	for _, d := range modelDirs(executableDir()) {
		if dirExists(d) {
			return openFolder(d)
		}
	}
	return openFolder(modelDirs(executableDir())[0])
}

// OpenLogFolder opens the log directory in Explorer.
func (s *SettingsService) OpenLogFolder() error {
	return s.app.OpenLogFolder()
}

// Close hides the settings window.
func (s *SettingsService) Close() {
	s.app.HideSettings()
}

// allowedURLs are the only pages the settings screen may open in a browser.
var allowedURLs = map[string]bool{
	"https://plamo.preferredai.jp/info/plamo-community-license-ja": true,
	"https://plamo.preferredai.jp/info/plamo-community-license-en": true,
	"https://huggingface.co/pfnet/plamo-3-translate-0.8b":          true,
	"https://forms.gle/mTL8tBLrMYXKNZD56":                          true,
}

// OpenURL opens one of the license or model card pages in the browser.
func (s *SettingsService) OpenURL(url string) error {
	if !allowedURLs[url] {
		return fmt.Errorf("URL not allowed: %s", url)
	}
	return s.app.wapp.Browser.OpenURL(url)
}

// DownloadCatalog lists downloadable models and the download status.
func (s *SettingsService) DownloadCatalog() DownloadCatalog {
	return s.app.downloadCatalog()
}

// StartDownload downloads a model from Hugging Face (user-initiated only).
func (s *SettingsService) StartDownload(file string) error {
	return s.app.startDownload(file)
}

// CancelDownload stops the running download; it can be resumed later.
func (s *SettingsService) CancelDownload() {
	s.app.cancelDownload()
}
