// Package download fetches PLaMo 3 Translate GGUF files from Hugging Face.
// It is only used when the user explicitly presses the download button.
package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultBaseURL is the Hugging Face endpoint.
const DefaultBaseURL = "https://huggingface.co"

// ErrChecksum is returned when the downloaded file does not match the
// SHA256 published by Hugging Face.
var ErrChecksum = errors.New("SHA256 mismatch")

// FileInfo describes a file in a Hugging Face repository.
type FileInfo struct {
	Repo   string
	File   string
	Size   int64
	SHA256 string
}

// Client downloads files from a Hugging Face compatible server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client for the public Hugging Face hub.
func NewClient() *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{}}
}

type treeEntry struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	LFS  *struct {
		OID  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs"`
}

// Lookup returns the size and SHA256 (LFS oid) of file in repo.
func (c *Client) Lookup(ctx context.Context, repo, file string) (FileInfo, error) {
	u := fmt.Sprintf("%s/api/models/%s/tree/main", strings.TrimRight(c.BaseURL, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return FileInfo{}, err
	}
	req.Header.Set("User-Agent", "Yaku")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return FileInfo{}, fmt.Errorf("list %s: %w", repo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return FileInfo{}, fmt.Errorf("list %s: HTTP %d", repo, resp.StatusCode)
	}
	var entries []treeEntry
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&entries); err != nil {
		return FileInfo{}, fmt.Errorf("decode file list: %w", err)
	}
	for _, e := range entries {
		if e.Path != file {
			continue
		}
		if e.LFS == nil || len(e.LFS.OID) != 64 {
			return FileInfo{}, fmt.Errorf("%s has no SHA256 in the file list", file)
		}
		return FileInfo{Repo: repo, File: file, Size: e.LFS.Size, SHA256: strings.ToLower(e.LFS.OID)}, nil
	}
	return FileInfo{}, fmt.Errorf("%s is not published in %s", file, repo)
}

// Progress is called periodically with bytes written so far and the total.
type Progress func(done, total int64)

// Download saves info into dir and returns the final path. An existing
// "<file>.partial" is resumed with an HTTP Range request. On cancellation the
// partial file is kept for a later resume; on checksum mismatch it is removed.
func (c *Client) Download(ctx context.Context, info FileInfo, dir string, progress Progress) (string, error) {
	if strings.ContainsAny(info.File, `/\`) || info.File == "" {
		return "", fmt.Errorf("invalid file name %q", info.File)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	final := filepath.Join(dir, info.File)
	partial := final + ".partial"

	hasher := sha256.New()
	var offset int64
	if f, err := os.Open(partial); err == nil {
		n, err := io.Copy(hasher, f)
		f.Close()
		if err != nil || n > info.Size {
			hasher.Reset()
			n = 0
			_ = os.Remove(partial)
		}
		offset = n
	}

	u := fmt.Sprintf("%s/%s/resolve/main/%s", strings.TrimRight(c.BaseURL, "/"), info.Repo, url.PathEscape(info.File))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Yaku")
	if offset > 0 && offset < info.Size {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	if offset < info.Size {
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return "", fmt.Errorf("download %s: %w", info.File, err)
		}
		defer resp.Body.Close()

		flags := os.O_CREATE | os.O_WRONLY
		switch {
		case resp.StatusCode == http.StatusPartialContent && offset > 0:
			flags |= os.O_APPEND
		case resp.StatusCode == http.StatusOK:
			// Server ignored the range: start over.
			flags |= os.O_TRUNC
			offset = 0
			hasher.Reset()
		default:
			return "", fmt.Errorf("download %s: HTTP %d", info.File, resp.StatusCode)
		}
		f, err := os.OpenFile(partial, flags, 0o644)
		if err != nil {
			return "", fmt.Errorf("open %s: %w", partial, err)
		}
		pw := &progressWriter{done: offset, total: info.Size, fn: progress}
		_, copyErr := io.Copy(io.MultiWriter(f, hasher, pw), resp.Body)
		closeErr := f.Close()
		pw.report(true)
		if copyErr != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", fmt.Errorf("download %s: %w", info.File, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("write %s: %w", partial, closeErr)
		}
		offset = pw.done
	}

	if offset != info.Size {
		return "", fmt.Errorf("download %s: got %d bytes, want %d", info.File, offset, info.Size)
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); got != info.SHA256 {
		_ = os.Remove(partial)
		return "", fmt.Errorf("%w for %s: got %s, want %s", ErrChecksum, info.File, got, info.SHA256)
	}
	if err := os.Rename(partial, final); err != nil {
		return "", fmt.Errorf("finalize %s: %w", final, err)
	}
	return final, nil
}

type progressWriter struct {
	done, total int64
	fn          Progress
	last        time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	p.report(false)
	return len(b), nil
}

func (p *progressWriter) report(force bool) {
	if p.fn == nil {
		return
	}
	if force || time.Since(p.last) >= 200*time.Millisecond {
		p.last = time.Now()
		p.fn(p.done, p.total)
	}
}
