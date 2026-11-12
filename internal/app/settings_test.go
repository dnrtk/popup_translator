package app

import (
	"testing"

	"github.com/dnrtk/popup_translator/internal/config"
)

func TestNeedsRestart(t *testing.T) {
	base := config.Default()
	tests := []struct {
		name   string
		modify func(*config.Config)
		want   bool
	}{
		{"nothing", func(*config.Config) {}, false},
		{"gpu layers", func(c *config.Config) { c.GPULayers = 99 }, true},
		{"model", func(c *config.Config) { c.ModelPath = "x.gguf" }, true},
		{"context", func(c *config.Config) { c.ContextSize = 4096 }, true},
		{"threads", func(c *config.Config) { c.Threads = 4 }, true},
		{"llama path", func(c *config.Config) { c.LlamaServerPath = "x.exe" }, true},
		{"hotkey interval", func(c *config.Config) { c.HotkeyIntervalMs = 300 }, false},
		{"clipboard delay", func(c *config.Config) { c.ClipboardDelayMs = 120 }, false},
		{"kana", func(c *config.Config) { c.KanaThreshold = 0.1 }, false},
		{"target mode", func(c *config.Config) { c.TargetMode = config.TargetModeRemember }, false},
		{"form", func(c *config.Config) { c.Form = "polite" }, false},
		{"debug", func(c *config.Config) { c.Debug = true }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := base
			tt.modify(&next)
			if got := needsRestart(base, next); got != tt.want {
				t.Errorf("needsRestart = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeSettingsKeepsWindowStateAndNormalizes(t *testing.T) {
	current := config.Default()
	current.LastTarget = "ko"
	current.Popup = config.Geometry{X: 5, Y: 6, Width: 700, Height: 500}

	edited := config.Default()
	edited.LastTarget = "fr"                            // not owned by the settings screen
	edited.Popup = config.Geometry{Width: 1, Height: 1} // ditto
	edited.HotkeyIntervalMs = 5                         // out of range
	edited.GPULayers = 99

	got := mergeSettings(current, edited)
	if got.LastTarget != "ko" || got.Popup != current.Popup {
		t.Errorf("window state not preserved: %+v", got)
	}
	if got.HotkeyIntervalMs != config.Default().HotkeyIntervalMs {
		t.Errorf("interval not normalized: %d", got.HotkeyIntervalMs)
	}
	if got.GPULayers != 99 {
		t.Errorf("edited value lost: %d", got.GPULayers)
	}
}

func TestDescribeModel(t *testing.T) {
	tests := []struct {
		file, params, quant string
	}{
		{"plamo-3-translate-0.8b-Q4_K_M.gguf", "0.8B", "Q4_K_M"},
		{"plamo-3-translate-2b-Q2_K.gguf", "2B", "Q2_K"},
		{"plamo-3-translate-31b-Q3_K_M.gguf", "31B", "Q3_K_M"},
		{"other-model.gguf", "", ""},
	}
	for _, tt := range tests {
		m := describeModel(`C:\m\`+tt.file, 123)
		if m.Params != tt.params || m.Quant != tt.quant || m.File != tt.file || m.Hint == "" {
			t.Errorf("%s: %+v", tt.file, m)
		}
	}
}
