// Package loki ships per-review log lines to a Grafana Loki instance via its
// HTTP push API. It is deliberately best-effort: a stream never blocks the
// process feeding it and never surfaces a network error to its writer, so a
// slow or unreachable Loki can neither stall nor fail a review. The on-disk
// review log remains the authoritative record; Loki is an additive, durable
// mirror.
package loki

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBatchWait     = time.Second
	defaultBatchMaxLines = 1000
	defaultTimeout       = 10 * time.Second
	defaultBufferLines   = 4096
	defaultAppLabel      = "nickpit"

	// maxErrSnippet bounds how much of an error response body is read for the
	// log message (like the GitLab client's error path).
	maxErrSnippet = 1024
	// pushRetries is how many additional attempts a failed push makes. Kept
	// small and bounded: a wedged Loki must never accumulate goroutines or
	// stall a stream's Close beyond a predictable ceiling.
	pushRetries = 2
	// retryBackoff is multiplied by the attempt number for a short, capped
	// backoff between push retries. All retries run on the flusher goroutine,
	// never on the review's critical path.
	retryBackoff = 200 * time.Millisecond
)

// Config configures a Client. Zero values fall back to the defaults above.
// URL is the Loki base (the "/loki/api/v1/push" suffix is appended). BasicUser
// / BasicPass set HTTP basic auth; TenantID sets the X-Scope-OrgID header for
// multi-tenant Loki. StaticLabels are merged into every stream's label set.
type Config struct {
	URL           string
	TenantID      string
	BasicUser     string
	BasicPass     string
	StaticLabels  map[string]string
	AppLabel      string
	BatchWait     time.Duration
	BatchMaxLines int
	Timeout       time.Duration
	BufferLines   int
	Gzip          bool
}

// Client is safe for concurrent use: it holds only immutable configuration and
// an *http.Client. All per-review mutable state lives in the lineWriter
// returned by NewStream, so one Client is shared by every review.
type Client struct {
	pushURL    string
	httpClient *http.Client
	cfg        Config
	base       map[string]string // static labels + app, merged into every stream
	log        *slog.Logger
}

// NewClient builds a Client, applying defaults for any unset Config field.
func NewClient(cfg Config, log *slog.Logger) *Client {
	if cfg.BatchWait <= 0 {
		cfg.BatchWait = defaultBatchWait
	}
	if cfg.BatchMaxLines <= 0 {
		cfg.BatchMaxLines = defaultBatchMaxLines
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.BufferLines <= 0 {
		cfg.BufferLines = defaultBufferLines
	}
	appLabel := cfg.AppLabel
	if appLabel == "" {
		appLabel = defaultAppLabel
	}
	base := make(map[string]string, len(cfg.StaticLabels)+1)
	for k, v := range cfg.StaticLabels {
		base[k] = v
	}
	base["app"] = appLabel

	return &Client{
		pushURL:    strings.TrimRight(cfg.URL, "/") + "/loki/api/v1/push",
		httpClient: &http.Client{Timeout: cfg.Timeout},
		cfg:        cfg,
		base:       base,
		log:        log,
	}
}

// pushStream/pushBody mirror Loki's push JSON: each value is a
// [unix-nanoseconds, line] pair.
type pushStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

type pushBody struct {
	Streams []pushStream `json:"streams"`
}

// push sends one batch of values under the given labels. It retries transport
// errors and 5xx/429 responses a bounded number of times; a 4xx is permanent
// and returned immediately. The returned error is for logging only — callers
// swallow it.
func (c *Client) push(labels map[string]string, values [][2]string) error {
	payload, err := json.Marshal(pushBody{Streams: []pushStream{{Stream: labels, Values: values}}})
	if err != nil {
		return fmt.Errorf("loki: encoding push body: %w", err)
	}
	if c.cfg.Gzip {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(payload); err != nil {
			return fmt.Errorf("loki: gzip: %w", err)
		}
		if err := gz.Close(); err != nil {
			return fmt.Errorf("loki: gzip close: %w", err)
		}
		payload = buf.Bytes()
	}

	var lastErr error
	for attempt := 0; attempt <= pushRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * retryBackoff)
		}
		status, err := c.send(payload)
		if err == nil {
			return nil
		}
		lastErr = err
		// Only transport errors (status 0) and 5xx/429 are worth retrying; a
		// 4xx (bad labels, auth) will fail identically on retry.
		retryable := status == 0 || status == http.StatusTooManyRequests || status >= 500
		if !retryable {
			return err
		}
	}
	return lastErr
}

// send performs one push attempt. It returns the HTTP status (0 on transport
// failure) and a non-nil error for any transport failure or >= 300 response.
func (c *Client) send(payload []byte) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.pushURL, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("loki: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.Gzip {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if c.cfg.TenantID != "" {
		req.Header.Set("X-Scope-OrgID", c.cfg.TenantID)
	}
	if c.cfg.BasicUser != "" {
		req.SetBasicAuth(c.cfg.BasicUser, c.cfg.BasicPass)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("loki: push: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrSnippet))
		return resp.StatusCode, fmt.Errorf("loki: push status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	// Drain a bounded amount so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrSnippet))
	return resp.StatusCode, nil
}
