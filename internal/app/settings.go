package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dnrtk/popup_translator/internal/autostart"
	"github.com/dnrtk/popup_translator/internal/config"
	"github.com/dnrtk/popup_translator/internal/hotkey"
	"github.com/dnrtk/popup_translator/internal/llama"
)

const autostartName = "Yaku"

// SettingsData is everything the settings screen needs.
type SettingsData struct {
	Config      config.Config    `json:"config"`
	Autostart   bool             `json:"autostart"`
	Models      []ModelInfo      `json:"models"`
	ModelDirs   []string         `json:"modelDirs"`
	ActiveModel string           `json:"activeModel"`
	Devices     []llama.Device   `json:"devices"`
	DeviceError string           `json:"deviceError"`
	Server      ServerStateEvent `json:"server"`
	CPUThreads  int              `json:"cpuThreads"`
}

// SaveResult tells the settings screen what happened.
type SaveResult struct {
	Restarted bool `json:"restarted"`
}

// needsRestart reports whether switching from old to new requires a
// llama-server restart (device, model, context size or threads).
func needsRestart(old, new config.Config) bool {
	return old.GPULayers != new.GPULayers ||
		old.ModelPath != new.ModelPath ||
		old.ContextSize != new.ContextSize ||
		old.Threads != new.Threads ||
		old.LlamaServerPath != new.LlamaServerPath
}

// mergeSettings applies the user-editable fields of edited onto current and
// keeps state that the settings screen does not own (window geometry, last
// target language).
func mergeSettings(current, edited config.Config) config.Config {
	edited.LastTarget = current.LastTarget
	edited.Popup = current.Popup
	edited.Normalize()
	return edited
}

// settingsData collects data for the settings screen.
func (a *App) settingsData() SettingsData {
	cfg := a.Config()
	exeDir := executableDir()
	d := SettingsData{
		Config:     cfg,
		Models:     listModels(exeDir),
		ModelDirs:  modelDirs(exeDir),
		Server:     a.ServerState(),
		CPUThreads: runtimeNumCPU(),
	}
	if on, err := autostart.Enabled(autostartName); err == nil {
		d.Autostart = on
	} else {
		a.log.Warn("read autostart", "err", err)
	}
	a.mu.Lock()
	d.ActiveModel = a.serverOpts.Model
	a.mu.Unlock()
	// GPU detection only needs llama-server, not a model.
	if exe := llamaServerPath(cfg); exe != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		devs, err := llama.ListDevices(ctx, exe)
		if err != nil {
			d.DeviceError = err.Error()
		}
		d.Devices = devs
	}
	return d
}

// applySettings saves edited settings, applies live-changeable ones and
// restarts llama-server only when needed.
func (a *App) applySettings(edited config.Config, autostartOn bool) (SaveResult, error) {
	old := a.Config()
	merged := mergeSettings(old, edited)

	if err := config.Save(a.cfgPath, merged); err != nil {
		return SaveResult{}, fmt.Errorf("設定を保存できませんでした: %w", err)
	}
	a.cfgMu.Lock()
	a.cfg = merged
	a.cfgMu.Unlock()

	hotkey.SetInterval(time.Duration(merged.HotkeyIntervalMs) * time.Millisecond)

	if cur, err := autostart.Enabled(autostartName); err != nil || cur != autostartOn {
		exe, err := os.Executable()
		if err == nil {
			err = autostart.Set(autostartName, exe, autostartOn)
		}
		if err != nil {
			return SaveResult{}, fmt.Errorf("自動起動の設定に失敗しました: %w", err)
		}
	}

	a.log.Info("settings saved", "restart", needsRestart(old, merged))
	if !needsRestart(old, merged) {
		return SaveResult{}, nil
	}

	// Remember an in-flight translation so it can run again on the new server.
	a.mu.Lock()
	st := a.state
	inFlight := a.cancelTx != nil && (st.Status == StatusTranslating || st.Status == StatusLoading)
	a.mu.Unlock()

	go func() {
		a.restartServer()
		if inFlight {
			a.translate(st.Source, st.Target, false)
		}
	}()
	return SaveResult{Restarted: true}, nil
}
