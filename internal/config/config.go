// Package config reads and writes %APPDATA%\Yaku\config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// AppName is used for the config directory and other per-user paths.
const AppName = "Yaku"

// Target language behavior after a new hotkey trigger.
const (
	TargetModeAuto     = "auto"     // always reset to the automatic rule
	TargetModeRemember = "remember" // keep the last selected language
)

// Popup position behavior.
const (
	PopupPositionCursor   = "cursor"   // near the mouse cursor
	PopupPositionRemember = "remember" // last saved position
)

// Geometry is a window rectangle in screen DIP coordinates.
type Geometry struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Config is the persisted user configuration. Fields missing from the JSON
// file keep their default values.
type Config struct {
	// LlamaServerPath is the llama-server executable. Empty means
	// "<exe dir>/llama/llama-server.exe".
	LlamaServerPath string `json:"llama_server_path"`
	// ModelPath is the GGUF file. Empty means the first *.gguf found in the
	// models directory (see app.resolveModel).
	ModelPath string `json:"model_path"`
	// GPULayers is passed as -ngl (0 = CPU only, 99 = everything on GPU).
	GPULayers   int `json:"gpu_layers"`
	ContextSize int `json:"context_size"`
	// Threads is passed as -t; 0 lets llama.cpp decide.
	Threads int `json:"threads"`

	HotkeyIntervalMs int     `json:"hotkey_interval_ms"`
	ClipboardDelayMs int     `json:"clipboard_delay_ms"`
	KanaThreshold    float64 `json:"kana_threshold"`
	MaxChars         int     `json:"max_chars"`

	TargetMode string `json:"target_mode"`
	LastTarget string `json:"last_target"`
	// Form is the Japanese style when translating into Japanese: "", "plain" or "polite".
	Form string `json:"form"`

	CloseOnBlur   bool     `json:"close_on_blur"`
	PopupPosition string   `json:"popup_position"`
	Popup         Geometry `json:"popup"`

	// Debug enables logging of source and translated text.
	Debug bool `json:"debug"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		GPULayers:        0,
		ContextSize:      8192,
		Threads:          0,
		HotkeyIntervalMs: 400,
		ClipboardDelayMs: 80,
		KanaThreshold:    0.05,
		MaxChars:         20000,
		TargetMode:       TargetModeAuto,
		PopupPosition:    PopupPositionCursor,
		Popup:            Geometry{Width: 560, Height: 420},
	}
}

// Dir returns %APPDATA%\Yaku (or the platform equivalent).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(base, AppName), nil
}

// DefaultPath returns the config file path.
func DefaultPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads path. A missing file yields defaults and is created.
// Invalid values are replaced by defaults.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := Save(path, cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	case err != nil:
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.Normalize()
	return cfg, nil
}

// Normalize replaces out-of-range values with defaults.
func (c *Config) Normalize() {
	d := Default()
	if c.GPULayers < 0 {
		c.GPULayers = d.GPULayers
	}
	if c.ContextSize < 512 {
		c.ContextSize = d.ContextSize
	}
	if c.Threads < 0 {
		c.Threads = d.Threads
	}
	if c.HotkeyIntervalMs < 50 || c.HotkeyIntervalMs > 2000 {
		c.HotkeyIntervalMs = d.HotkeyIntervalMs
	}
	if c.ClipboardDelayMs < 0 || c.ClipboardDelayMs > 2000 {
		c.ClipboardDelayMs = d.ClipboardDelayMs
	}
	if c.KanaThreshold <= 0 || c.KanaThreshold > 1 {
		c.KanaThreshold = d.KanaThreshold
	}
	if c.MaxChars <= 0 {
		c.MaxChars = d.MaxChars
	}
	if c.TargetMode != TargetModeAuto && c.TargetMode != TargetModeRemember {
		c.TargetMode = d.TargetMode
	}
	if c.Form != "" && c.Form != "plain" && c.Form != "polite" {
		c.Form = ""
	}
	if c.PopupPosition != PopupPositionCursor && c.PopupPosition != PopupPositionRemember {
		c.PopupPosition = d.PopupPosition
	}
	if c.Popup.Width < 200 || c.Popup.Height < 150 {
		c.Popup.Width, c.Popup.Height = d.Popup.Width, d.Popup.Height
	}
}

// Save writes cfg to path atomically.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
