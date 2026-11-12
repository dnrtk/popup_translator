package llama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func sse(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", b)
	w.(http.Flusher).Flush()
}

func TestCompleteStreaming(t *testing.T) {
	var got completionBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/completion" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, tok := range []string{"The ", "meeting ", "moved."} {
			sse(w, map[string]any{"content": tok, "stop": false})
		}
		sse(w, map[string]any{"content": "", "stop": true, "stop_type": "eos", "tokens_predicted": 3,
			"timings": map[string]any{"prompt_n": 20, "predicted_n": 3, "predicted_per_second": 120.5}})
	}))
	defer srv.Close()

	var tokens []string
	res, err := NewClient(srv.URL).Complete(context.Background(),
		CompletionRequest{Prompt: "P", NPredict: 64, Stop: []string{"<|plamo:tag|>"}},
		func(s string) { tokens = append(tokens, s) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "The meeting moved." || res.StopType != "eos" || res.TokensPredicted != 3 ||
		res.Timings.PredictedPerSecond != 120.5 || res.Timings.PromptN != 20 {
		t.Errorf("result = %+v", res)
	}
	if strings.Join(tokens, "|") != "The |meeting |moved." {
		t.Errorf("tokens = %q", tokens)
	}
	if got.Prompt != "P" || got.NPredict != 64 || !got.Stream || !got.CachePrompt || got.Temperature != 0 ||
		len(got.Stop) != 1 {
		t.Errorf("request body = %+v", got)
	}
}

func TestCompleteCancel(t *testing.T) {
	var serverSawCancel atomic.Bool
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sse(w, map[string]any{"content": "partial", "stop": false})
		select {
		case <-r.Context().Done():
			serverSawCancel.Store(true)
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := NewClient(srv.URL).Complete(ctx, CompletionRequest{Prompt: "P"}, func(string) { close(first) })
		done <- err
	}()
	<-first
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Complete did not return after cancel")
	}
	deadline := time.Now().Add(2 * time.Second)
	for !serverSawCancel.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !serverSawCancel.Load() {
		t.Error("server did not observe the cancelled connection")
	}
}

func TestCompleteServerDiesMidStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sse(w, map[string]any{"content": "half", "stop": false})
		// Abort the connection without a final message.
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("no hijacker")
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	defer srv.Close()

	res, err := NewClient(srv.URL).Complete(context.Background(), CompletionRequest{Prompt: "P"}, nil)
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("err = %v, want ErrStreamClosed", err)
	}
	if res.Text != "half" {
		t.Errorf("partial text = %q", res.Text)
	}
}

func TestCompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":400,"message":"the request exceeds the available context size","type":"exceed_context_size_error"}}`)
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL).Complete(context.Background(), CompletionRequest{Prompt: "P"}, nil)
	if err == nil || !strings.Contains(err.Error(), "exceeds the available context size") {
		t.Errorf("err = %v", err)
	}
}

func TestCompleteErrorInStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sse(w, map[string]any{"error": map[string]any{"message": "boom"}})
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL).Complete(context.Background(), CompletionRequest{Prompt: "P"}, nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v", err)
	}
}

func TestHealth(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusServiceUnavailable)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(status.Load()))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	if err := c.Health(context.Background()); !errors.Is(err, ErrLoading) {
		t.Errorf("loading: err = %v", err)
	}
	status.Store(http.StatusOK)
	if err := c.Health(context.Background()); err != nil {
		t.Errorf("ready: err = %v", err)
	}
	status.Store(http.StatusInternalServerError)
	if err := c.Health(context.Background()); err == nil {
		t.Error("500 should be an error")
	}
}

func TestNPredictFor(t *testing.T) {
	if got := NPredictFor(10, 8192); got != 148 {
		t.Errorf("got %d", got)
	}
	if got := NPredictFor(8000, 8192); got != 8192 {
		t.Errorf("capped: got %d", got)
	}
}
