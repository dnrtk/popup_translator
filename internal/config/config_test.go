package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingCreatesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("got %+v, want defaults", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := Default()
	want.ModelPath = `C:\models\plamo-3-translate-2b-Q4_K_M.gguf`
	want.GPULayers = 99
	want.TargetMode = TargetModeRemember
	want.LastTarget = "ko"
	want.Form = "polite"
	want.Popup = Geometry{X: 10, Y: 20, Width: 600, Height: 400}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestLoadFillsMissingFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"gpu_layers": 99, "model_path": "m.gguf"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.GPULayers = 99
	want.ModelPath = "m.gguf"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestLoadNormalizesInvalidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"context_size": 10, "hotkey_interval_ms": -1, "kana_threshold": 3,
	          "target_mode": "weird", "form": "casual", "popup": {"width": 1, "height": 1}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	d := Default()
	if got.ContextSize != d.ContextSize || got.HotkeyIntervalMs != d.HotkeyIntervalMs ||
		got.KanaThreshold != d.KanaThreshold || got.TargetMode != d.TargetMode ||
		got.Form != "" || got.Popup.Width != d.Popup.Width {
		t.Errorf("not normalized: %+v", got)
	}
}

func TestLoadBrokenJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{broken`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Error("broken config should fall back to defaults")
	}
}
