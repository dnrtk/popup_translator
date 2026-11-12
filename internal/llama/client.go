// Package llama manages the llama-server child process and talks to its
// /completion endpoint.
package llama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrLoading is returned by Health while the model is still loading.
var ErrLoading = errors.New("llama-server is loading the model")

// ErrStreamClosed is returned when the server closes the stream before a
// final message (e.g. the process crashed).
var ErrStreamClosed = errors.New("llama-server closed the stream unexpectedly")

// Client is an HTTP client for one llama-server instance.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client for baseURL such as "http://127.0.0.1:12345".
func NewClient(baseURL string) *Client {
	// No overall timeout: streams can be long. Callers cancel via context.
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{}}
}

// Health returns nil when the server is ready, ErrLoading while loading.
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusServiceUnavailable:
		return ErrLoading
	default:
		return fmt.Errorf("health: unexpected status %d", resp.StatusCode)
	}
}

// CompletionRequest is the subset of /completion parameters the app uses.
type CompletionRequest struct {
	Prompt   string
	NPredict int
	Stop     []string
}

// Timings are reported by llama-server in the final stream message.
type Timings struct {
	PromptN            int     `json:"prompt_n"`
	PromptMS           float64 `json:"prompt_ms"`
	PromptPerSecond    float64 `json:"prompt_per_second"`
	PredictedN         int     `json:"predicted_n"`
	PredictedMS        float64 `json:"predicted_ms"`
	PredictedPerSecond float64 `json:"predicted_per_second"`
}

// Result summarizes a finished completion.
type Result struct {
	Text            string
	StopType        string // "eos", "word", "limit"
	TokensPredicted int
	Truncated       bool
	Timings         Timings
}

type completionBody struct {
	Prompt      string   `json:"prompt"`
	NPredict    int      `json:"n_predict"`
	Temperature float64  `json:"temperature"`
	CachePrompt bool     `json:"cache_prompt"`
	Stop        []string `json:"stop,omitempty"`
	Stream      bool     `json:"stream"`
}

type streamChunk struct {
	Content         string          `json:"content"`
	Stop            bool            `json:"stop"`
	StopType        string          `json:"stop_type"`
	TokensPredicted int             `json:"tokens_predicted"`
	Truncated       bool            `json:"truncated"`
	Timings         Timings         `json:"timings"`
	Error           json.RawMessage `json:"error"`
}

// Complete runs a streaming greedy completion and calls onToken for every
// text fragment as it arrives. It returns ctx.Err() when cancelled.
func (c *Client) Complete(ctx context.Context, cr CompletionRequest, onToken func(string)) (Result, error) {
	body, err := json.Marshal(completionBody{
		Prompt:      cr.Prompt,
		NPredict:    cr.NPredict,
		Temperature: 0,
		CachePrompt: true,
		Stop:        cr.Stop,
		Stream:      true,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/completion", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, fmt.Errorf("completion request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return Result{}, fmt.Errorf("completion: status %d: %s", resp.StatusCode, errorMessage(msg))
	}

	var text strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		payload, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			if p, isErr := strings.CutPrefix(line, "error: "); isErr {
				return Result{Text: text.String()}, fmt.Errorf("completion: %s", errorMessage([]byte(p)))
			}
			continue // blank lines, comments, event names
		}
		if payload == "[DONE]" {
			break
		}
		var ch streamChunk
		if err := json.Unmarshal([]byte(payload), &ch); err != nil {
			return Result{Text: text.String()}, fmt.Errorf("decode stream chunk: %w", err)
		}
		if len(ch.Error) > 0 && string(ch.Error) != "null" {
			return Result{Text: text.String()}, fmt.Errorf("completion: %s", errorMessage(ch.Error))
		}
		if ch.Content != "" {
			text.WriteString(ch.Content)
			if onToken != nil {
				onToken(ch.Content)
			}
		}
		if ch.Stop {
			return Result{
				Text:            text.String(),
				StopType:        ch.StopType,
				TokensPredicted: ch.TokensPredicted,
				Truncated:       ch.Truncated,
				Timings:         ch.Timings,
			}, nil
		}
	}
	if ctx.Err() != nil {
		return Result{Text: text.String()}, ctx.Err()
	}
	if err := sc.Err(); err != nil {
		return Result{Text: text.String()}, fmt.Errorf("%w: %v", ErrStreamClosed, err)
	}
	return Result{Text: text.String()}, ErrStreamClosed
}

// errorMessage extracts {"error":{"message":...}} or {"message":...} if present.
func errorMessage(raw []byte) string {
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &wrapped) == nil {
		if wrapped.Error.Message != "" {
			return wrapped.Error.Message
		}
		if wrapped.Message != "" {
			return wrapped.Message
		}
	}
	return strings.TrimSpace(string(raw))
}

// NPredictFor returns a generation limit for a source of the given length in
// runes. Translations rarely exceed twice the source length in tokens.
func NPredictFor(runes, contextSize int) int {
	n := runes*2 + 128
	if contextSize > 0 && n > contextSize {
		n = contextSize
	}
	return n
}
