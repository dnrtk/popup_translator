package download

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const repo = "pfnet/plamo-3-translate-0.8b-GGUF"
const file = "plamo-3-translate-0.8b-Q4_K_M.gguf"

type fakeHub struct {
	content     []byte
	sha         string
	rangeReqs   atomic.Int32
	ignoreRange bool
	slow        bool
}

func newFakeHub(t *testing.T, size int) (*fakeHub, *httptest.Server) {
	t.Helper()
	content := bytes.Repeat([]byte("0123456789abcdef"), size/16+1)[:size]
	sum := sha256.Sum256(content)
	h := &fakeHub{content: content, sha: hex.EncodeToString(sum[:])}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/" + repo + "/tree/main":
			fmt.Fprintf(w, `[{"type":"file","path":"README.md","size":10},
				{"type":"file","path":%q,"size":%d,"lfs":{"oid":%q,"size":%d}}]`,
				file, len(h.content), h.sha, len(h.content))
		case "/" + repo + "/resolve/main/" + file:
			if r.Header.Get("Range") != "" {
				h.rangeReqs.Add(1)
				if h.ignoreRange {
					r.Header.Del("Range")
				}
			}
			if h.slow {
				w.Header().Set("Content-Length", fmt.Sprint(len(h.content)))
				w.WriteHeader(http.StatusOK)
				for i := 0; i < len(h.content); i += 1024 {
					end := min(i+1024, len(h.content))
					if _, err := w.Write(h.content[i:end]); err != nil {
						return
					}
					w.(http.Flusher).Flush()
					time.Sleep(5 * time.Millisecond)
				}
				return
			}
			http.ServeContent(w, r, file, time.Time{}, bytes.NewReader(h.content))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return h, srv
}

func client(srv *httptest.Server) *Client { return &Client{BaseURL: srv.URL, HTTP: srv.Client()} }

func TestLookupAndDownload(t *testing.T) {
	hub, srv := newFakeHub(t, 100_000)
	c := client(srv)
	info, err := c.Lookup(context.Background(), repo, file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != 100_000 || info.SHA256 != hub.sha {
		t.Fatalf("info = %+v", info)
	}
	dir := t.TempDir()
	var lastDone, lastTotal int64
	path, err := c.Download(context.Background(), info, dir, func(d, tot int64) { lastDone, lastTotal = d, tot })
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, hub.content) || filepath.Base(path) != file {
		t.Error("content mismatch")
	}
	if lastDone != 100_000 || lastTotal != 100_000 {
		t.Errorf("progress = %d/%d", lastDone, lastTotal)
	}
	if _, err := os.Stat(path + ".partial"); !os.IsNotExist(err) {
		t.Error("partial file left behind")
	}
}

func TestLookupMissingFile(t *testing.T) {
	_, srv := newFakeHub(t, 10)
	if _, err := client(srv).Lookup(context.Background(), repo, "nope.gguf"); err == nil {
		t.Error("expected error")
	}
	if _, err := client(srv).Lookup(context.Background(), "other/repo", file); err == nil {
		t.Error("expected error for missing repo")
	}
}

func TestResume(t *testing.T) {
	hub, srv := newFakeHub(t, 50_000)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, file+".partial"), hub.content[:20_000], 0o644); err != nil {
		t.Fatal(err)
	}
	c := client(srv)
	info, _ := c.Lookup(context.Background(), repo, file)
	path, err := c.Download(context.Background(), info, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hub.rangeReqs.Load() != 1 {
		t.Errorf("range requests = %d", hub.rangeReqs.Load())
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, hub.content) {
		t.Error("resumed content mismatch")
	}
}

func TestResumeServerIgnoresRange(t *testing.T) {
	hub, srv := newFakeHub(t, 30_000)
	hub.ignoreRange = true
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, file+".partial"), hub.content[:10_000], 0o644)
	c := client(srv)
	info, _ := c.Lookup(context.Background(), repo, file)
	path, err := c.Download(context.Background(), info, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, hub.content) {
		t.Error("content mismatch after restart from scratch")
	}
}

func TestChecksumMismatch(t *testing.T) {
	_, srv := newFakeHub(t, 5_000)
	c := client(srv)
	info, _ := c.Lookup(context.Background(), repo, file)
	info.SHA256 = strings.Repeat("0", 64)
	dir := t.TempDir()
	_, err := c.Download(context.Background(), info, dir, nil)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("files left after checksum failure: %v", entries)
	}
}

func TestCancelKeepsPartial(t *testing.T) {
	hub, srv := newFakeHub(t, 400_000)
	hub.slow = true
	c := client(srv)
	info, _ := c.Lookup(context.Background(), repo, file)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	var cancelled atomic.Bool
	_, err := c.Download(ctx, info, dir, func(done, _ int64) {
		if done > 50_000 && !cancelled.Swap(true) {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	st, err := os.Stat(filepath.Join(dir, file+".partial"))
	if err != nil || st.Size() == 0 || st.Size() >= int64(len(hub.content)) {
		t.Fatalf("partial = %v, %v", st, err)
	}
	// A later download resumes and completes.
	hub.slow = false
	path, err := c.Download(context.Background(), info, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, hub.content) {
		t.Error("content mismatch after resume")
	}
}

func TestRejectsPathInFileName(t *testing.T) {
	_, err := (&Client{}).Download(context.Background(), FileInfo{File: `..\evil.gguf`}, t.TempDir(), nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestCatalog(t *testing.T) {
	cat := Catalog()
	if len(cat) != 14 {
		t.Errorf("len = %d", len(cat))
	}
	e, ok := FindEntry("plamo-3-translate-2b-Q4_K_M.gguf")
	if !ok || e.Repo != "pfnet/plamo-3-translate-2b-GGUF" || e.Params != "2B" || e.SizeBytes != 1807203040 {
		t.Errorf("entry = %+v", e)
	}
	if _, ok := FindEntry("plamo-3-translate-8b-Q4_0.gguf"); ok {
		t.Error("8B Q4_0 is not published")
	}
}

func TestFreeSpace(t *testing.T) {
	if FreeSpace(filepath.Join(t.TempDir(), "not", "yet")) == 0 {
		t.Error("free space should not be zero for an existing volume")
	}
}
