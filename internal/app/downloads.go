package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/dnrtk/popup_translator/internal/download"
)

// EventDownload reports download progress to the settings window.
const EventDownload = "download:progress"

// Download states.
const (
	DownloadIdle     = "idle"
	DownloadLookup   = "lookup" // fetching size and SHA256
	DownloadRunning  = "running"
	DownloadDone     = "done"
	DownloadError    = "error"
	DownloadCanceled = "canceled"
)

// DownloadStatus is the state of the (single) running download.
type DownloadStatus struct {
	File    string `json:"file"`
	State   string `json:"state"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Message string `json:"message"`
}

// DownloadEntry is a catalog row for the settings screen.
type DownloadEntry struct {
	download.Entry
	Installed bool   `json:"installed"`
	Partial   int64  `json:"partial"` // bytes of an interrupted download
	Hint      string `json:"hint"`
}

// DownloadCatalog is everything the download section needs.
type DownloadCatalog struct {
	Entries   []DownloadEntry `json:"entries"`
	TargetDir string          `json:"targetDir"`
	FreeBytes int64           `json:"freeBytes"` // -1 if unknown
	Status    DownloadStatus  `json:"status"`
}

func init() {
	application.RegisterEvent[DownloadStatus](EventDownload)
}

// downloadDir is where new models are saved: the first existing models
// directory, otherwise <exe>/models.
func downloadDir() string {
	dirs := modelDirs(executableDir())
	for _, d := range dirs {
		if dirExists(d) {
			return d
		}
	}
	return dirs[0]
}

func (a *App) downloadCatalog() DownloadCatalog {
	dir := downloadDir()
	installed := map[string]bool{}
	for _, m := range listModels(executableDir()) {
		installed[m.File] = true
	}
	var entries []DownloadEntry
	for _, e := range download.Catalog() {
		row := DownloadEntry{Entry: e, Installed: installed[e.File], Hint: describeModel(e.File, e.SizeBytes).Hint}
		if st, err := os.Stat(filepath.Join(dir, e.File+".partial")); err == nil {
			row.Partial = st.Size()
		}
		entries = append(entries, row)
	}
	a.dlMu.Lock()
	status := a.dlStatus
	a.dlMu.Unlock()
	if status.State == "" {
		status.State = DownloadIdle
	}
	return DownloadCatalog{Entries: entries, TargetDir: dir, FreeBytes: download.FreeSpace(dir), Status: status}
}

func (a *App) setDownloadStatus(s DownloadStatus) {
	a.dlMu.Lock()
	a.dlStatus = s
	a.dlMu.Unlock()
	if a.wapp != nil {
		a.wapp.Event.Emit(EventDownload, s)
	}
}

// startDownload begins downloading file from Hugging Face. Only one download
// runs at a time. This is the only code path that talks to the network.
func (a *App) startDownload(file string) error {
	entry, ok := download.FindEntry(file)
	if !ok {
		return fmt.Errorf("unknown model %q", file)
	}
	dir := downloadDir()
	if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
		return fmt.Errorf("%s はダウンロード済みです", file)
	}
	if free := download.FreeSpace(dir); free >= 0 && free < entry.SizeBytes+256<<20 {
		return fmt.Errorf("ディスクの空き容量が足りません(必要: 約 %.1f GB、空き: %.1f GB)",
			float64(entry.SizeBytes)/(1<<30), float64(free)/(1<<30))
	}

	a.dlMu.Lock()
	if a.dlCancel != nil {
		a.dlMu.Unlock()
		return errors.New("別のモデルをダウンロード中です")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.dlCancel = cancel
	a.dlMu.Unlock()

	a.log.Info("download start", "file", file, "dir", dir)
	a.setDownloadStatus(DownloadStatus{File: file, State: DownloadLookup, Total: entry.SizeBytes})
	go func() {
		defer func() {
			a.dlMu.Lock()
			a.dlCancel = nil
			a.dlMu.Unlock()
			cancel()
		}()
		client := download.NewClient()
		info, err := client.Lookup(ctx, entry.Repo, entry.File)
		if err == nil {
			_, err = client.Download(ctx, info, dir, func(done, total int64) {
				a.setDownloadStatus(DownloadStatus{File: file, State: DownloadRunning, Done: done, Total: total})
			})
		}
		switch {
		case err == nil:
			a.log.Info("download done", "file", file)
			a.setDownloadStatus(DownloadStatus{File: file, State: DownloadDone, Done: info.Size, Total: info.Size})
		case errors.Is(err, context.Canceled):
			a.log.Info("download canceled", "file", file)
			a.setDownloadStatus(DownloadStatus{File: file, State: DownloadCanceled,
				Message: "中断しました。もう一度ダウンロードすると続きから再開します。"})
		default:
			a.log.Warn("download failed", "file", file, "err", err)
			a.setDownloadStatus(DownloadStatus{File: file, State: DownloadError, Message: err.Error()})
		}
	}()
	return nil
}

func (a *App) cancelDownload() {
	a.dlMu.Lock()
	cancel := a.dlCancel
	a.dlMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
