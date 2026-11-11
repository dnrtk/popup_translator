// Package app wires the hotkey, clipboard, language detection, llama-server
// and the popup window together.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/dnrtk/popup_translator/internal/clipboard"
	"github.com/dnrtk/popup_translator/internal/config"
	"github.com/dnrtk/popup_translator/internal/hotkey"
	"github.com/dnrtk/popup_translator/internal/langdetect"
	"github.com/dnrtk/popup_translator/internal/llama"
	"github.com/dnrtk/popup_translator/internal/prompt"
)

// Event names sent to the frontend.
const (
	EventShow   = "popup:show"
	EventStatus = "translate:status"
	EventChunk  = "translate:chunk"
	EventDone   = "translate:done"
	EventError  = "translate:error"
)

// Status values shown in the popup.
const (
	StatusLoading     = "loading"     // model is loading
	StatusTranslating = "translating" // waiting for / receiving tokens
	StatusDone        = "done"
	StatusError       = "error"
)

// PopupState is the full popup content sent on every new translation.
type PopupState struct {
	ID       uint64 `json:"id"`
	Source   string `json:"source"`
	Detected string `json:"detected"` // "ja" or "other"
	Target   string `json:"target"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

// StatusEvent updates the status of translation ID.
type StatusEvent struct {
	ID     uint64 `json:"id"`
	Status string `json:"status"`
}

// ChunkEvent carries a streamed fragment of translation ID.
type ChunkEvent struct {
	ID   uint64 `json:"id"`
	Text string `json:"text"`
}

// DoneEvent carries the final translation text of ID.
type DoneEvent struct {
	ID   uint64 `json:"id"`
	Text string `json:"text"`
}

// ErrorEvent reports a failure of translation ID.
type ErrorEvent struct {
	ID      uint64 `json:"id"`
	Message string `json:"message"`
}

func init() {
	application.RegisterEvent[PopupState](EventShow)
	application.RegisterEvent[StatusEvent](EventStatus)
	application.RegisterEvent[ChunkEvent](EventChunk)
	application.RegisterEvent[DoneEvent](EventDone)
	application.RegisterEvent[ErrorEvent](EventError)
}

// App is the application core.
type App struct {
	wapp  *application.App
	popup *application.WebviewWindow
	tray  *application.SystemTray
	log   *slog.Logger

	cfgPath string
	cfgMu   sync.Mutex
	cfg     config.Config

	server     *llama.Server
	serverLog  io.WriteCloser
	fatalShown bool

	mu       sync.Mutex
	nextID   uint64
	cancelTx context.CancelFunc
	state    PopupState

	stopHotkey context.CancelFunc
}

// New creates the core. Call Attach after the Wails app and windows exist.
func New(cfgPath string, cfg config.Config, logger *slog.Logger) *App {
	return &App{cfgPath: cfgPath, cfg: cfg, log: logger}
}

// Config returns a copy of the current configuration.
func (a *App) Config() config.Config {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.cfg
}

func (a *App) updateConfig(fn func(*config.Config)) {
	a.cfgMu.Lock()
	fn(&a.cfg)
	cfg := a.cfg
	a.cfgMu.Unlock()
	if err := config.Save(a.cfgPath, cfg); err != nil {
		a.log.Error("save config", "err", err)
	}
}

// Attach connects the core to Wails objects and starts background work.
func (a *App) Attach(wapp *application.App, popup *application.WebviewWindow, tray *application.SystemTray) {
	a.wapp, a.popup, a.tray = wapp, popup, tray

	popup.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		// Closing the popup only hides it; the app keeps running in the tray.
		e.Cancel()
		a.HidePopup()
	})
	popup.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) {
		if a.Config().CloseOnBlur {
			a.HidePopup()
		}
	})
}

// devTrigger is set by the devtrigger build tag (see devtrigger.go).
var devTrigger func(*App)

// Start launches llama-server and the hotkey hook.
func (a *App) Start() {
	a.startServer()
	if devTrigger != nil {
		devTrigger(a)
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.stopHotkey = cancel
	interval := time.Duration(a.Config().HotkeyIntervalMs) * time.Millisecond
	fired, err := hotkey.Watch(ctx, interval, nil)
	if err != nil {
		a.log.Error("hotkey hook", "err", err)
		a.setTooltip("Yaku - ホットキーを登録できませんでした")
		return
	}
	go func() {
		for range fired {
			a.onHotkey()
		}
	}()
}

// Shutdown stops background work and the server.
func (a *App) Shutdown() {
	if a.stopHotkey != nil {
		a.stopHotkey()
	}
	a.mu.Lock()
	if a.cancelTx != nil {
		a.cancelTx()
	}
	a.mu.Unlock()
	if a.server != nil {
		a.server.Stop()
	}
	if a.serverLog != nil {
		a.serverLog.Close()
	}
}

func (a *App) startServer() {
	cfg := a.Config()
	exeDir := executableDir()
	llamaPath := cfg.LlamaServerPath
	if llamaPath == "" {
		llamaPath = filepath.Join(exeDir, "llama", "llama-server.exe")
	}
	model, err := resolveModel(cfg.ModelPath, exeDir)
	if err != nil {
		a.onServerState(llama.StateFailed, err)
		return
	}
	logDir, err := LogDir()
	if err == nil {
		err = os.MkdirAll(logDir, 0o755)
	}
	var logw io.Writer
	if err == nil {
		f, ferr := os.Create(filepath.Join(logDir, "llama-server.log"))
		if ferr == nil {
			a.serverLog = f
			logw = f
		}
	}
	a.log.Info("starting llama-server", "exe", llamaPath, "model", model, "ngl", cfg.GPULayers, "ctx", cfg.ContextSize)
	a.server = llama.NewServer(llama.Options{
		Exe:         llamaPath,
		Model:       model,
		GPULayers:   cfg.GPULayers,
		ContextSize: cfg.ContextSize,
		Threads:     cfg.Threads,
		Log:         logw,
	}, a.onServerState)
	if err := a.server.Start(); err != nil {
		a.server = nil
		a.onServerState(llama.StateFailed, err)
	}
}

func (a *App) onServerState(st llama.State, err error) {
	a.log.Info("llama-server state", "state", st.String(), "err", err)
	switch st {
	case llama.StateStarting:
		a.setTooltip("Yaku - モデル読み込み中")
	case llama.StateReady:
		a.setTooltip("Yaku - 準備完了 (Ctrl+C+C で翻訳)")
	case llama.StateFailed:
		a.setTooltip("Yaku - 翻訳エンジンを起動できません")
		a.mu.Lock()
		show := !a.fatalShown
		a.fatalShown = true
		a.mu.Unlock()
		if show && a.wapp != nil {
			msg := "翻訳エンジン(llama-server)を起動できませんでした。"
			if err != nil {
				msg += "\n\n" + err.Error()
			}
			if dir, derr := LogDir(); derr == nil {
				msg += "\n\nログ: " + dir
			}
			application.InvokeAsync(func() {
				a.wapp.Dialog.Error().SetTitle("Yaku").SetMessage(msg).Show()
			})
		}
	}
}

func (a *App) setTooltip(s string) {
	if a.tray != nil {
		application.InvokeAsync(func() { a.tray.SetTooltip(s) })
	}
}

func (a *App) onHotkey() {
	cfg := a.Config()
	time.Sleep(time.Duration(cfg.ClipboardDelayMs) * time.Millisecond)
	text, err := clipboard.ReadText()
	if err != nil {
		if !errors.Is(err, clipboard.ErrNoText) {
			a.log.Warn("read clipboard", "err", err)
		}
		return
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	isJa := langdetect.IsJapanese(text, cfg.KanaThreshold)
	target := langdetect.DefaultTarget(isJa)
	if cfg.TargetMode == config.TargetModeRemember && prompt.IsSupported(cfg.LastTarget) {
		target = cfg.LastTarget
	}
	a.translate(text, target, true)
}

// translate cancels any running request and starts a new one. When show is
// true the popup is positioned near the cursor and focused.
func (a *App) translate(text, target string, show bool) {
	cfg := a.Config()
	isJa := langdetect.IsJapanese(text, cfg.KanaThreshold)
	detected := "other"
	if isJa {
		detected = "ja"
	}

	a.mu.Lock()
	if a.cancelTx != nil {
		a.cancelTx()
		a.cancelTx = nil
	}
	a.nextID++
	id := a.nextID
	st := PopupState{ID: id, Source: text, Detected: detected, Target: target, Status: StatusTranslating}

	var buildErr error
	var promptText string
	if n := utf8.RuneCountInString(text); n > cfg.MaxChars {
		buildErr = fmt.Errorf("テキストが長すぎます(%d 文字、上限 %d 文字)", n, cfg.MaxChars)
	} else {
		form := prompt.FormNone
		if target == "ja" {
			form = prompt.Form(cfg.Form)
		}
		promptText, buildErr = prompt.Build(prompt.Request{Text: text, TargetLang: target, Form: form})
		if errors.Is(buildErr, prompt.ErrReservedToken) {
			buildErr = errors.New("翻訳できない文字列(<|plamo:)が含まれています")
		}
	}
	if buildErr != nil {
		st.Status, st.Message = StatusError, buildErr.Error()
	} else if s, _ := a.serverState(); s != llama.StateReady {
		st.Status = StatusLoading
	}
	a.state = st
	var ctx context.Context
	if buildErr == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		a.cancelTx = cancel
	}
	a.mu.Unlock()

	if cfg.Debug {
		a.log.Debug("translate", "id", id, "target", target, "detected", detected, "text", text)
	} else {
		a.log.Info("translate", "id", id, "target", target, "detected", detected, "chars", utf8.RuneCountInString(text))
	}

	a.wapp.Event.Emit(EventShow, st)
	if show {
		a.showPopup()
	}
	if buildErr != nil {
		return
	}
	go a.run(ctx, id, promptText, utf8.RuneCountInString(text), cfg)
}

func (a *App) serverState() (llama.State, error) {
	if a.server == nil {
		return llama.StateFailed, errors.New("llama-server is not configured")
	}
	return a.server.State()
}

func (a *App) run(ctx context.Context, id uint64, promptText string, runes int, cfg config.Config) {
	if a.server == nil {
		a.emitError(id, "翻訳エンジンを起動できませんでした。ログを確認してください。")
		return
	}
	baseURL, err := a.server.WaitReady(ctx)
	if err != nil {
		if ctx.Err() == nil {
			a.emitError(id, "翻訳エンジンを起動できませんでした: "+err.Error())
		}
		return
	}
	a.emitIfCurrent(id, EventStatus, StatusEvent{ID: id, Status: StatusTranslating})

	start := time.Now()
	res, err := llama.NewClient(baseURL).Complete(ctx, llama.CompletionRequest{
		Prompt:   promptText,
		NPredict: llama.NPredictFor(runes, cfg.ContextSize),
		Stop:     prompt.StopWords,
	}, func(tok string) {
		a.emitIfCurrent(id, EventChunk, ChunkEvent{ID: id, Text: tok})
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		a.log.Warn("completion failed", "id", id, "err", err)
		a.emitError(id, "翻訳に失敗しました: "+err.Error())
		return
	}
	out := prompt.TrimStop(res.Text)
	a.log.Info("translated", "id", id, "tokens", res.TokensPredicted, "stop", res.StopType,
		"elapsed_ms", time.Since(start).Milliseconds())
	if cfg.Debug {
		a.log.Debug("translation", "id", id, "text", out)
	}
	a.mu.Lock()
	if a.state.ID == id {
		a.state.Status = StatusDone
	}
	a.mu.Unlock()
	a.emitIfCurrent(id, EventDone, DoneEvent{ID: id, Text: out})
}

func (a *App) emitError(id uint64, msg string) {
	a.mu.Lock()
	if a.state.ID == id {
		a.state.Status, a.state.Message = StatusError, msg
	}
	a.mu.Unlock()
	a.emitIfCurrent(id, EventError, ErrorEvent{ID: id, Message: msg})
}

func (a *App) emitIfCurrent(id uint64, name string, data any) {
	a.mu.Lock()
	current := a.nextID == id
	a.mu.Unlock()
	if current {
		a.wapp.Event.Emit(name, data)
	}
}

// HidePopup hides the popup, cancels the running translation and stores the
// window geometry.
func (a *App) HidePopup() {
	a.mu.Lock()
	if a.cancelTx != nil {
		a.cancelTx()
		a.cancelTx = nil
	}
	a.mu.Unlock()
	application.InvokeSync(func() {
		if !a.popup.IsVisible() {
			return
		}
		b := a.popup.Bounds()
		a.updateConfig(func(c *config.Config) {
			c.Popup = config.Geometry{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
		})
		a.popup.Hide()
	})
}

func (a *App) showPopup() {
	cfg := a.Config()
	application.InvokeSync(func() {
		w, h := cfg.Popup.Width, cfg.Popup.Height
		var rect application.Rect
		if cfg.PopupPosition == config.PopupPositionRemember && (cfg.Popup.X != 0 || cfg.Popup.Y != 0) {
			rect = application.Rect{X: cfg.Popup.X, Y: cfg.Popup.Y, Width: w, Height: h}
		} else {
			rect = a.rectNearCursor(w, h)
		}
		a.popup.SetBounds(rect)
		a.popup.Show()
		forceForeground(a.popup.NativeWindow())
		a.popup.Focus()
	})
}

// rectNearCursor places a w×h (DIP) window just below-right of the mouse
// cursor, flipped or clamped so it stays inside the monitor's work area.
func (a *App) rectNearCursor(w, h int) application.Rect {
	px, py, ok := cursorPos()
	if !ok {
		return application.Rect{X: 100, Y: 100, Width: w, Height: h}
	}
	p := application.Point{X: px, Y: py}
	screen := a.wapp.Screen.ScreenNearestPhysicalPoint(p)
	if screen == nil {
		return application.Rect{X: 100, Y: 100, Width: w, Height: h}
	}
	scale := float64(screen.ScaleFactor)
	if scale <= 0 {
		scale = 1
	}
	pw, ph := int(float64(w)*scale), int(float64(h)*scale)
	offset := int(16 * scale)
	work := screen.PhysicalWorkArea
	r := placeRect(px, py, pw, ph, offset, work.X, work.Y, work.Width, work.Height)
	return a.wapp.Screen.PhysicalToDipRect(application.Rect{X: r[0], Y: r[1], Width: pw, Height: ph})
}

// placeRect returns the top-left of a w×h box near (cx, cy) inside the work
// area (wx, wy, ww, wh). All values are in physical pixels.
func placeRect(cx, cy, w, h, offset, wx, wy, ww, wh int) [2]int {
	x, y := cx+offset, cy+offset
	if x+w > wx+ww {
		x = cx - offset - w // flip to the left of the cursor
	}
	if y+h > wy+wh {
		y = cy - offset - h // flip above the cursor
	}
	x = clamp(x, wx, wx+ww-w)
	y = clamp(y, wy, wy+wh-h)
	return [2]int{x, y}
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// LogDir returns %APPDATA%\Yaku\logs.
func LogDir() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// resolveModel returns configured, or the best *.gguf in <exe>/models or
// <exe>/../models (the latter matches the development layout bin/yaku.exe).
func resolveModel(configured, exeDir string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err != nil {
			return "", fmt.Errorf("model_path %q: %w", configured, err)
		}
		return configured, nil
	}
	dirs := []string{filepath.Join(exeDir, "models"), filepath.Join(exeDir, "..", "models")}
	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.gguf"))
		if len(matches) == 0 {
			continue
		}
		sort.SliceStable(matches, func(i, j int) bool {
			return modelRank(matches[i]) < modelRank(matches[j])
		})
		return filepath.Clean(matches[0]), nil
	}
	return "", fmt.Errorf("no *.gguf model found in %s (run scripts/fetch-model.ps1 or set model_path)", strings.Join(dirs, ", "))
}

// modelRank prefers the default 0.8B Q4_K_M, then smaller models.
func modelRank(path string) int {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(name, "0.8b-q4_k_m"):
		return 0
	case strings.Contains(name, "0.8b"):
		return 1
	case strings.Contains(name, "2b"):
		return 2
	case strings.Contains(name, "8b"):
		return 3
	}
	return 4
}
