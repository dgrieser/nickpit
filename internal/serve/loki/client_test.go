package loki

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// captured records one received push request.
type captured struct {
	method      string
	path        string
	contentType string
	orgID       string
	authUser    string
	authOK      bool
	encoding    string
	body        pushBody
}

// capturingServer returns an httptest server that decodes each push body (de-
// gzipping when needed) and appends it to a slice, plus a getter for the
// captured requests.
func capturingServer(t *testing.T, status int) (*httptest.Server, func() []captured) {
	t.Helper()
	var mu sync.Mutex
	var reqs []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reader io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Errorf("gzip reader: %v", err)
				http.Error(w, "bad gzip", http.StatusBadRequest)
				return
			}
			defer func() { _ = gz.Close() }()
			reader = gz
		}
		data, _ := io.ReadAll(reader)
		var body pushBody
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("decoding push body: %v (%s)", err, data)
		}
		user, _, ok := r.BasicAuth()
		mu.Lock()
		reqs = append(reqs, captured{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			orgID:       r.Header.Get("X-Scope-OrgID"),
			authUser:    user,
			authOK:      ok,
			encoding:    r.Header.Get("Content-Encoding"),
			body:        body,
		})
		mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []captured {
		mu.Lock()
		defer mu.Unlock()
		out := make([]captured, len(reqs))
		copy(out, reqs)
		return out
	}
}

// blockingServer returns an httptest server whose handler blocks until release
// is closed, simulating a stalled Loki backend.
func blockingServer(t *testing.T, release <-chan struct{}) (*httptest.Server, func() int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv, hits.Load
}

func TestStreamPushesLinesWithLabels(t *testing.T) {
	srv, getReqs := capturingServer(t, http.StatusNoContent)
	client := NewClient(Config{
		URL:          srv.URL,
		StaticLabels: map[string]string{"env": "test"},
	}, discardLogger())

	stream := client.NewStream(map[string]string{"project": "platform/api", "iid": "7", "trigger": "auto"})
	if _, err := io.WriteString(stream, "first line\nsecond line\n"); err != nil {
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}

	reqs := getReqs()
	if len(reqs) == 0 {
		t.Fatal("no push received")
	}
	req := reqs[0]
	if req.method != http.MethodPost || req.path != "/loki/api/v1/push" {
		t.Fatalf("method/path = %s %s", req.method, req.path)
	}
	if req.contentType != "application/json" {
		t.Fatalf("content-type = %q", req.contentType)
	}
	if len(req.body.Streams) != 1 {
		t.Fatalf("streams = %d", len(req.body.Streams))
	}
	labels := req.body.Streams[0].Stream
	for k, want := range map[string]string{"app": "nickpit", "env": "test", "project": "platform/api", "iid": "7", "trigger": "auto"} {
		if labels[k] != want {
			t.Errorf("label %s = %q, want %q", k, labels[k], want)
		}
	}

	// Collect the line texts across all received batches.
	var texts []string
	for _, r := range reqs {
		for _, s := range r.body.Streams {
			for _, v := range s.Values {
				texts = append(texts, v[1])
			}
		}
	}
	if len(texts) != 2 || texts[0] != "first line" || texts[1] != "second line" {
		t.Fatalf("lines = %#v", texts)
	}
}

func TestStreamSetsAuthAndTenantHeaders(t *testing.T) {
	srv, getReqs := capturingServer(t, http.StatusNoContent)
	client := NewClient(Config{
		URL:       srv.URL,
		TenantID:  "team-a",
		BasicUser: "loki",
		BasicPass: "s3cret",
	}, discardLogger())

	stream := client.NewStream(nil)
	_, _ = io.WriteString(stream, "line\n")
	_ = stream.Close()

	reqs := getReqs()
	if len(reqs) == 0 {
		t.Fatal("no push received")
	}
	if reqs[0].orgID != "team-a" {
		t.Errorf("X-Scope-OrgID = %q", reqs[0].orgID)
	}
	if !reqs[0].authOK || reqs[0].authUser != "loki" {
		t.Errorf("basic auth = %q (ok=%v)", reqs[0].authUser, reqs[0].authOK)
	}
}

func TestStreamGzip(t *testing.T) {
	srv, getReqs := capturingServer(t, http.StatusNoContent)
	client := NewClient(Config{URL: srv.URL, Gzip: true}, discardLogger())

	stream := client.NewStream(nil)
	_, _ = io.WriteString(stream, "compress me\n")
	_ = stream.Close()

	reqs := getReqs()
	if len(reqs) == 0 {
		t.Fatal("no push received")
	}
	if reqs[0].encoding != "gzip" {
		t.Fatalf("content-encoding = %q", reqs[0].encoding)
	}
	// The capturing server already gunzipped and decoded the body, so a
	// well-formed stream proves the payload round-tripped.
	if len(reqs[0].body.Streams) != 1 || len(reqs[0].body.Streams[0].Values) != 1 {
		t.Fatalf("gzip body did not decode: %#v", reqs[0].body)
	}
}

func TestNoTenantOrAuthHeadersWhenUnset(t *testing.T) {
	srv, getReqs := capturingServer(t, http.StatusNoContent)
	client := NewClient(Config{URL: srv.URL}, discardLogger())

	stream := client.NewStream(nil)
	_, _ = io.WriteString(stream, "line\n")
	_ = stream.Close()

	reqs := getReqs()
	if len(reqs) == 0 {
		t.Fatal("no push received")
	}
	if reqs[0].orgID != "" {
		t.Errorf("unexpected X-Scope-OrgID = %q", reqs[0].orgID)
	}
	if reqs[0].authOK {
		t.Error("unexpected basic auth header")
	}
}

func TestPushRetriesServerErrorThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(Config{URL: srv.URL}, discardLogger())
	if err := client.push(map[string]string{"app": "nickpit"}, [][2]string{{"1", "x"}}); err != nil {
		t.Fatalf("push should succeed after retry: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestPushDoesNotRetryClientError(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(Config{URL: srv.URL}, discardLogger())
	if err := client.push(map[string]string{"app": "nickpit"}, [][2]string{{"1", "x"}}); err == nil {
		t.Fatal("push should return the 400 error")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (no retry on 4xx)", got)
	}
}

func TestNewClientAppliesDefaults(t *testing.T) {
	client := NewClient(Config{URL: "http://loki:3100"}, discardLogger())
	if client.cfg.BatchWait != defaultBatchWait ||
		client.cfg.BatchMaxLines != defaultBatchMaxLines ||
		client.cfg.Timeout != defaultTimeout ||
		client.cfg.BufferLines != defaultBufferLines {
		t.Fatalf("defaults not applied: %+v", client.cfg)
	}
	if client.base["app"] != defaultAppLabel {
		t.Fatalf("app label = %q", client.base["app"])
	}
	if client.pushURL != "http://loki:3100/loki/api/v1/push" {
		t.Fatalf("push url = %q", client.pushURL)
	}
	// A trailing slash on the base URL must not double up.
	client = NewClient(Config{URL: "http://loki:3100/"}, discardLogger())
	if client.pushURL != "http://loki:3100/loki/api/v1/push" {
		t.Fatalf("push url = %q", client.pushURL)
	}
}

// A slow backend must not delay Write: the buffer fills and lines drop, but
// Write returns immediately.
func TestWriteNeverBlocksOnSlowBackend(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	client := NewClient(Config{URL: srv.URL, BufferLines: 4, BatchMaxLines: 1, Timeout: time.Second}, discardLogger())
	stream := client.NewStream(nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			_, _ = io.WriteString(stream, "spammy line that would block a naive writer\n")
		}
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Write blocked on a stalled Loki backend")
	}
}
