package app

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ModelInfo describes a GGUF file found in the models directory.
type ModelInfo struct {
	Path      string `json:"path"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
	Params    string `json:"params"` // "0.8B", "2B", "8B", "31B" or ""
	Quant     string `json:"quant"`  // "Q4_K_M" etc.
	// Hint is a short note on CPU/GPU suitability for the settings screen.
	Hint string `json:"hint"`
}

var modelName = regexp.MustCompile(`(?i)plamo-3-translate-(0\.8b|2b|8b|31b)-([A-Za-z0-9_]+)\.gguf$`)

// describeModel derives size class and hints from the file name.
func describeModel(path string, size int64) ModelInfo {
	m := ModelInfo{Path: path, File: filepath.Base(path), SizeBytes: size}
	if g := modelName.FindStringSubmatch(m.File); g != nil {
		m.Params = strings.ToUpper(g[1])
		m.Quant = strings.ToUpper(g[2])
	}
	// Hints are based on the measurements in docs/FINDINGS.md.
	switch m.Params {
	case "0.8B":
		m.Hint = "CPU で快適に動作"
	case "2B":
		m.Hint = "CPU でも実用的(GPU ならより高速)"
	case "8B":
		m.Hint = "CPU では遅い。GPU(VRAM 6GB 以上)推奨"
	case "31B":
		m.Hint = "GPU 必須(VRAM 20GB 以上が目安)"
	default:
		m.Hint = "不明なモデル"
	}
	if m.Quant == "Q2_K" {
		m.Hint += "・Q2_K は精度が低め"
	}
	return m
}

// modelDirs returns the directories searched for *.gguf files.
func modelDirs(exeDir string) []string {
	return []string{filepath.Join(exeDir, "models"), filepath.Clean(filepath.Join(exeDir, "..", "models"))}
}

// listModels returns all *.gguf files in the model directories, best first.
func listModels(exeDir string) []ModelInfo {
	seen := map[string]bool{}
	var out []ModelInfo
	for _, dir := range modelDirs(exeDir) {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.gguf"))
		for _, p := range matches {
			p = filepath.Clean(p)
			if seen[strings.ToLower(p)] {
				continue
			}
			seen[strings.ToLower(p)] = true
			var size int64
			if st, err := os.Stat(p); err == nil {
				size = st.Size()
			}
			out = append(out, describeModel(p, size))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := modelRank(out[i].Path), modelRank(out[j].Path)
		if ri != rj {
			return ri < rj
		}
		return out[i].File < out[j].File
	})
	return out
}
