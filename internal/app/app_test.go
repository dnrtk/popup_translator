package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dnrtk/popup_translator/internal/prompt"
)

func TestPlaceRect(t *testing.T) {
	// Work area 0,0 1920x1040; window 500x400; offset 16.
	tests := []struct {
		name   string
		cx, cy int
		want   [2]int
	}{
		{"room below right", 100, 100, [2]int{116, 116}},
		{"near right edge flips left", 1800, 100, [2]int{1800 - 16 - 500, 116}},
		{"near bottom flips up", 100, 1000, [2]int{116, 1000 - 16 - 400}},
		{"bottom right corner", 1900, 1030, [2]int{1900 - 16 - 500, 1030 - 16 - 400}},
		{"flip would go off top, clamp", 100, 300, [2]int{116, 300 + 16}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := placeRect(tt.cx, tt.cy, 500, 400, 16, 0, 0, 1920, 1040)
			if got != tt.want {
				t.Errorf("placeRect = %v, want %v", got, tt.want)
			}
		})
	}
	// Window larger than the work area pins to the origin.
	if got := placeRect(10, 10, 3000, 3000, 16, 0, 0, 1920, 1040); got != [2]int{0, 0} {
		t.Errorf("oversized = %v", got)
	}
	// Secondary monitor with negative origin.
	if got := placeRect(-1800, 200, 500, 400, 16, -1920, 0, 1920, 1040); got != [2]int{-1784, 216} {
		t.Errorf("negative origin = %v", got)
	}
}

func TestResolveModel(t *testing.T) {
	root := t.TempDir()
	exeDir := filepath.Join(root, "bin")
	models := filepath.Join(root, "models")
	for _, d := range []string{exeDir, models} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := resolveModel("", exeDir); err == nil {
		t.Error("expected error when no models exist")
	}
	for _, name := range []string{"plamo-3-translate-2b-Q4_K_M.gguf", "plamo-3-translate-0.8b-Q4_K_M.gguf", "plamo-3-translate-0.8b-Q2_K.gguf"} {
		if err := os.WriteFile(filepath.Join(models, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolveModel("", exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "plamo-3-translate-0.8b-Q4_K_M.gguf" {
		t.Errorf("got %s", got)
	}
	explicit := filepath.Join(models, "plamo-3-translate-2b-Q4_K_M.gguf")
	if got, err := resolveModel(explicit, exeDir); err != nil || got != explicit {
		t.Errorf("explicit: %s %v", got, err)
	}
	if _, err := resolveModel(filepath.Join(models, "missing.gguf"), exeDir); err == nil {
		t.Error("expected error for missing explicit model")
	}
}

func TestTrimHistory(t *testing.T) {
	h := []prompt.Pair{{Source: "aaaa", Translation: "bbbb"}, {Source: "cc", Translation: "dd"}, {Source: "e", Translation: "f"}}
	if got := trimHistory(h, 100); len(got) != 3 {
		t.Errorf("fits: got %d pairs", len(got))
	}
	got := trimHistory(h, 6)
	if len(got) != 2 || got[0].Source != "cc" {
		t.Errorf("trim oldest: got %+v", got)
	}
	if got := trimHistory(h, 0); len(got) != 0 {
		t.Errorf("zero budget: got %+v", got)
	}
}

func TestSegmentLimit(t *testing.T) {
	if segmentLimit(8192) != 1638 || segmentLimit(512) != 256 {
		t.Error("unexpected segment limits")
	}
}
