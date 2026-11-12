//go:build windows

package llama

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runFakeParent is invoked via YAKU_FAKE_PARENT: it supervises a fake
// llama-server, prints its URL and then blocks until killed.
func runFakeParent() int {
	exe, err := os.Executable()
	if err != nil {
		return 10
	}
	os.Setenv("YAKU_FAKE_LLAMA", "ok")
	os.Unsetenv("YAKU_FAKE_PARENT")
	s := NewServer(Options{Exe: exe, Model: os.Getenv("YAKU_FAKE_MODEL"), ContextSize: 1024,
		PollInterval: 50 * time.Millisecond}, nil)
	if err := s.Start(); err != nil {
		return 11
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url, err := s.WaitReady(ctx)
	if err != nil {
		return 12
	}
	os.Stdout.WriteString("URL " + url + "\n")
	select {}
}

func init() {
	if os.Getenv("YAKU_FAKE_PARENT") != "" {
		os.Exit(runFakeParent())
	}
}

// TestChildDiesWithParent kills the supervising process with
// TerminateProcess and checks that the child llama-server goes away too.
func TestChildDiesWithParent(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(model, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(exe)
	parent.Env = append(os.Environ(), "YAKU_FAKE_PARENT=1", "YAKU_FAKE_MODEL="+model)
	out, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if u, ok := strings.CutPrefix(sc.Text(), "URL "); ok {
				lines <- u
				return
			}
		}
		close(lines)
	}()
	var url string
	select {
	case u, ok := <-lines:
		if !ok {
			t.Fatal("parent exited without printing URL")
		}
		url = u
	case <-time.After(15 * time.Second):
		parent.Process.Kill()
		t.Fatal("timeout waiting for fake server")
	}
	if err := NewClient(url).Health(context.Background()); err != nil {
		t.Fatalf("child not healthy before kill: %v", err)
	}

	if err := parent.Process.Kill(); err != nil { // TerminateProcess
		t.Fatal(err)
	}
	_ = parent.Wait()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if NewClient(url).Health(context.Background()) != nil {
			return // child is gone
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("child llama-server survived the parent's termination")
}
