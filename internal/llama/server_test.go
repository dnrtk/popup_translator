package llama

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The test binary doubles as a fake llama-server when YAKU_FAKE_LLAMA is set.
func TestMain(m *testing.M) {
	if mode := os.Getenv("YAKU_FAKE_LLAMA"); mode != "" {
		os.Exit(fakeLlama(mode, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fakeLlama(mode string, args []string) int {
	port := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--port" {
			port = args[i+1]
		}
	}
	if counter := os.Getenv("YAKU_FAKE_COUNTER"); counter != "" {
		f, err := os.OpenFile(counter, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			f.WriteString("x")
			f.Close()
		}
	}
	switch mode {
	case "crash":
		return 3
	case "crash-once-after-ready":
		marker := os.Getenv("YAKU_FAKE_MARKER")
		if _, err := os.Stat(marker); err != nil {
			os.WriteFile(marker, nil, 0o644)
			go func() {
				time.Sleep(300 * time.Millisecond)
				os.Exit(4)
			}()
		}
	}
	start := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if time.Since(start) < 200*time.Millisecond {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"status":"ok"}`))
	})
	if err := http.ListenAndServe("127.0.0.1:"+port, mux); err != nil {
		return 5
	}
	return 0
}

type stateLog struct {
	mu     sync.Mutex
	states []State
}

func (l *stateLog) add(s State, _ error) {
	l.mu.Lock()
	l.states = append(l.states, s)
	l.mu.Unlock()
}

func (l *stateLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var parts []string
	for _, s := range l.states {
		parts = append(parts, s.String())
	}
	return strings.Join(parts, ",")
}

func newFakeServer(t *testing.T, mode string, log *stateLog) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	model := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(model, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	counter := filepath.Join(dir, "launches")
	t.Setenv("YAKU_FAKE_LLAMA", mode)
	t.Setenv("YAKU_FAKE_COUNTER", counter)
	t.Setenv("YAKU_FAKE_MARKER", filepath.Join(dir, "marker"))
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	s := NewServer(Options{
		Exe:            exe,
		Model:          model,
		ContextSize:    1024,
		Log:            &syncWriter{w: &buf},
		StartupTimeout: 10 * time.Second,
		PollInterval:   50 * time.Millisecond,
	}, log.add)
	return s, counter
}

type syncWriter struct {
	mu sync.Mutex
	w  *bytes.Buffer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func launches(t *testing.T, counter string) int {
	b, err := os.ReadFile(counter)
	if err != nil {
		return 0
	}
	return len(b)
}

func TestServerReadyAndStop(t *testing.T) {
	var log stateLog
	s, counter := newFakeServer(t, "ok", &log)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url, err := s.WaitReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewClient(url).Health(ctx); err != nil {
		t.Errorf("health after ready: %v", err)
	}
	s.Stop()
	if st, _ := s.State(); st != StateStopped {
		t.Errorf("state after stop = %v", st)
	}
	if got := log.String(); got != "starting,ready,stopped" {
		t.Errorf("states = %s", got)
	}
	if n := launches(t, counter); n != 1 {
		t.Errorf("launches = %d", n)
	}
	// The process must be gone.
	if err := NewClient(url).Health(context.Background()); err == nil {
		t.Error("server still answering after Stop")
	}
}

func TestServerRestartsOnceThenFails(t *testing.T) {
	var log stateLog
	s, counter := newFakeServer(t, "crash", &log)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.WaitReady(ctx)
	if !errors.Is(err, ErrServerFailed) {
		t.Fatalf("err = %v, want ErrServerFailed", err)
	}
	if n := launches(t, counter); n != 2 {
		t.Errorf("launches = %d, want 2 (initial + 1 restart)", n)
	}
	if got := log.String(); got != "starting,starting,failed" {
		t.Errorf("states = %s", got)
	}
}

func TestServerRecoversAfterCrash(t *testing.T) {
	var log stateLog
	s, counter := newFakeServer(t, "crash-once-after-ready", &log)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if launches(t, counter) == 2 {
			if st, _ := s.State(); st == StateReady {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if st, err := s.State(); st != StateReady {
		t.Fatalf("state = %v (%v), states=%s", st, err, log.String())
	}
	if got := log.String(); got != "starting,ready,starting,ready" {
		t.Errorf("states = %s", got)
	}
}

func TestServerStartValidatesPaths(t *testing.T) {
	s := NewServer(Options{Exe: filepath.Join(t.TempDir(), "missing.exe"), Model: "x"}, nil)
	if err := s.Start(); err == nil {
		t.Error("expected error for missing executable")
	}
	// Waiters must be released with an error rather than block forever.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := s.WaitReady(ctx); !errors.Is(err, ErrServerFailed) {
		t.Errorf("WaitReady err = %v, want ErrServerFailed", err)
	}
}

// A translation waiting for readiness must survive a settings-triggered
// restart and receive the new server's URL.
func TestServerRestartKeepsWaiters(t *testing.T) {
	var log stateLog
	s, counter := newFakeServer(t, "ok", &log)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first, err := s.WaitReady(ctx)
	if err != nil {
		t.Fatal(err)
	}

	opts := s.opts
	opts.ContextSize = 2048
	if err := s.Restart(opts); err != nil {
		t.Fatal(err)
	}
	second, err := s.WaitReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Errorf("expected a new port after restart, got %s twice", first)
	}
	if n := launches(t, counter); n != 2 {
		t.Errorf("launches = %d", n)
	}
	if got := log.String(); got != "starting,ready,stopped,starting,ready" {
		t.Errorf("states = %s", got)
	}
}

func TestServerFail(t *testing.T) {
	s := NewServer(Options{}, nil)
	cause := errors.New("no model")
	s.Fail(cause)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := s.WaitReady(ctx); !errors.Is(err, ErrServerFailed) || !errors.Is(err, cause) {
		t.Errorf("err = %v; want both ErrServerFailed and the cause in the chain", err)
	}
}

func TestArgs(t *testing.T) {
	got := strings.Join(Options{Model: "m.gguf", GPULayers: 99, ContextSize: 8192, Threads: 4}.Args(1234), " ")
	want := "-m m.gguf -ngl 99 --host 127.0.0.1 --port 1234 -c 8192 -np 1 -t 4"
	if got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}
