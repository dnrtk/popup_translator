package app

import (
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
