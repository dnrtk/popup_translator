package llama

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// State is the lifecycle state of the llama-server process.
type State int

const (
	StateStopped State = iota
	StateStarting
	StateReady
	StateFailed
)

func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateReady:
		return "ready"
	case StateFailed:
		return "failed"
	}
	return "unknown"
}

// ErrServerFailed is returned by WaitReady when the server cannot be started.
var ErrServerFailed = errors.New("llama-server failed to start")

// Options configure the llama-server process.
type Options struct {
	Exe         string
	Model       string
	GPULayers   int
	ContextSize int
	Threads     int // 0 = llama.cpp default
	// Log receives stdout and stderr of the process. May be nil.
	Log io.Writer
	// ExtraArgs are appended after the generated arguments (used by tests).
	ExtraArgs []string

	// StartupTimeout bounds model loading (default 3 minutes).
	StartupTimeout time.Duration
	// MaxRestarts is the number of automatic restarts after an unexpected
	// exit (default 1, per the spec).
	MaxRestarts int
	// StableAfter resets the restart budget once the server has been ready
	// this long (default 5 minutes).
	StableAfter time.Duration
	// PollInterval for /health while starting (default 200ms).
	PollInterval time.Duration
}

// Server supervises one llama-server process.
type Server struct {
	opts    Options
	onState func(State, error)

	mu       sync.Mutex
	state    State
	lastErr  error
	baseURL  string
	changed  chan struct{} // closed and replaced on every state change
	cancel   context.CancelFunc
	done     chan struct{}
	stopping bool
}

// NewServer creates a supervisor. onState (may be nil) is called on every
// state change from the supervisor goroutine.
func NewServer(opts Options, onState func(State, error)) *Server {
	if opts.StartupTimeout <= 0 {
		opts.StartupTimeout = 3 * time.Minute
	}
	if opts.MaxRestarts == 0 {
		opts.MaxRestarts = 1
	}
	if opts.MaxRestarts < 0 {
		opts.MaxRestarts = 0
	}
	if opts.StableAfter <= 0 {
		opts.StableAfter = 5 * time.Minute
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 200 * time.Millisecond
	}
	return &Server{opts: opts, onState: onState, changed: make(chan struct{})}
}

// Args returns the llama-server command-line arguments for port.
func (o Options) Args(port int) []string {
	args := []string{
		"-m", o.Model,
		"-ngl", strconv.Itoa(o.GPULayers),
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"-c", strconv.Itoa(o.ContextSize),
		// One slot: the whole context is available to a single translation.
		"-np", "1",
	}
	if o.Threads > 0 {
		args = append(args, "-t", strconv.Itoa(o.Threads))
	}
	return append(args, o.ExtraArgs...)
}

// Start launches the supervisor. It returns immediately; use WaitReady.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done != nil {
		return errors.New("server already started")
	}
	if _, err := os.Stat(s.opts.Exe); err != nil {
		return fmt.Errorf("llama-server executable: %w", err)
	}
	if _, err := os.Stat(s.opts.Model); err != nil {
		return fmt.Errorf("model file: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	s.stopping = false
	go s.supervise(ctx)
	return nil
}

// Stop terminates the process and waits for the supervisor to exit.
func (s *Server) Stop() {
	s.mu.Lock()
	if s.done == nil {
		s.mu.Unlock()
		return
	}
	s.stopping = true
	cancel, done := s.cancel, s.done
	s.mu.Unlock()

	cancel()
	<-done

	s.mu.Lock()
	s.done = nil
	s.mu.Unlock()
	s.setState(StateStopped, nil, "")
}

// State returns the current state and the last error.
func (s *Server) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.lastErr
}

// WaitReady blocks until the server is ready and returns its base URL.
func (s *Server) WaitReady(ctx context.Context) (string, error) {
	for {
		s.mu.Lock()
		state, url, err, ch := s.state, s.baseURL, s.lastErr, s.changed
		s.mu.Unlock()
		switch state {
		case StateReady:
			return url, nil
		case StateFailed:
			return "", fmt.Errorf("%w: %v", ErrServerFailed, err)
		case StateStopped:
			if err != nil {
				return "", fmt.Errorf("%w: %v", ErrServerFailed, err)
			}
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (s *Server) setState(st State, err error, url string) {
	s.mu.Lock()
	s.state = st
	s.lastErr = err
	s.baseURL = url
	close(s.changed)
	s.changed = make(chan struct{})
	cb := s.onState
	s.mu.Unlock()
	if cb != nil {
		cb(st, err)
	}
}

func (s *Server) supervise(ctx context.Context) {
	defer close(s.done)
	restarts := 0
	for {
		s.setState(StateStarting, nil, "")
		readyAt, err := s.runOnce(ctx)
		if ctx.Err() != nil {
			return // Stop() was called
		}
		if !readyAt.IsZero() && time.Since(readyAt) >= s.opts.StableAfter {
			restarts = 0
		}
		if restarts >= s.opts.MaxRestarts {
			s.setState(StateFailed, err, "")
			return
		}
		restarts++
		s.logf("llama-server exited unexpectedly (%v); restarting (%d/%d)", err, restarts, s.opts.MaxRestarts)
	}
}

// runOnce starts the process, waits for readiness and then for exit. It
// returns when the process exits, with the time it became ready (zero if it
// never did).
func (s *Server) runOnce(ctx context.Context) (time.Time, error) {
	port, err := freePort()
	if err != nil {
		return time.Time{}, err
	}
	args := s.opts.Args(port)
	cmd := exec.Command(s.opts.Exe, args...)
	cmd.SysProcAttr = sysProcAttr()
	if s.opts.Log != nil {
		cmd.Stdout = s.opts.Log
		cmd.Stderr = s.opts.Log
	}
	s.logf("starting %s %v", s.opts.Exe, args)
	if err := cmd.Start(); err != nil {
		return time.Time{}, fmt.Errorf("start llama-server: %w", err)
	}
	if err := attachToJob(cmd.Process); err != nil {
		// Not fatal: the server still works, it just may outlive a crash.
		s.logf("warning: could not assign llama-server to job object: %v", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	client := NewClient(baseURL)
	startCtx, cancelStart := context.WithTimeout(ctx, s.opts.StartupTimeout)
	defer cancelStart()

	var readyAt time.Time
	ticker := time.NewTicker(s.opts.PollInterval)
	defer ticker.Stop()
poll:
	for {
		select {
		case err := <-exited:
			return time.Time{}, exitError(err)
		case <-startCtx.Done():
			_ = cmd.Process.Kill()
			<-exited
			if ctx.Err() != nil {
				return time.Time{}, ctx.Err()
			}
			return time.Time{}, fmt.Errorf("llama-server did not become ready within %v", s.opts.StartupTimeout)
		case <-ticker.C:
			if client.Health(startCtx) == nil {
				readyAt = time.Now()
				s.setState(StateReady, nil, baseURL)
				break poll
			}
		}
	}
	// Idle: no polling once ready, just wait for exit or stop.
	select {
	case err := <-exited:
		return readyAt, exitError(err)
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-exited
		return readyAt, ctx.Err()
	}
}

func exitError(err error) error {
	if err == nil {
		return errors.New("llama-server exited with status 0")
	}
	return fmt.Errorf("llama-server exited: %w", err)
}

func (s *Server) logf(format string, args ...any) {
	if s.opts.Log == nil {
		return
	}
	fmt.Fprintf(s.opts.Log, "[yaku %s] %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find free port: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
