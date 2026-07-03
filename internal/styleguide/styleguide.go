// Package styleguide resolves user-supplied additional styleguides — local
// files or http(s) URLs — into prompt-ready model.StyleGuide values. The
// built-in language styleguides live in prompts/styleguides and are selected
// by language detection; the guides resolved here are appended for every
// agent regardless of the languages in the diff.
package styleguide

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/model"
)

// MaxBytes caps one guide's size; mirrors the review engine's cap on built-in
// styleguide probes (review.maxStyleGuideProbeBytes).
const MaxBytes = 1 << 20

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Resolve loads every spec (local file path or http(s):// URL) into a
// prompt-ready StyleGuide, preserving order. Any failure aborts the whole run
// (fail fast): a review without an explicitly requested guide would silently
// judge by incomplete rules. URLs are fetched fresh on every call with a
// plain unauthenticated GET.
func Resolve(ctx context.Context, specs []string) ([]model.StyleGuide, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	guides := make([]model.StyleGuide, 0, len(specs))
	for _, spec := range specs {
		var content string
		var err error
		if isURL(spec) {
			content, err = fetchURL(ctx, spec)
		} else {
			content, err = readFile(spec)
		}
		if err != nil {
			return nil, err
		}
		content = strings.TrimSpace(content)
		if content == "" {
			return nil, fmt.Errorf("styleguide %q is empty", spec)
		}
		if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
			return nil, fmt.Errorf("styleguide %q is not text", spec)
		}
		guides = append(guides, model.StyleGuide{
			Language: spec,
			Content:  decorate(spec, content),
		})
	}
	return guides, nil
}

// isURL reports whether a spec addresses a remote guide. Only an explicit
// http(s):// prefix counts; everything else is a file path, so Windows drive
// paths or odd strings never turn into fetches.
func isURL(spec string) bool {
	lower := strings.ToLower(spec)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// decorate prepends a deterministic provenance heading so each guide starts
// with the same "### " level-3 heading the built-in styleguides are required
// to carry, and so agents can attribute rules to their source.
func decorate(spec, content string) string {
	return "### Additional styleguide: " + spec + "\n\n" + content
}

func fetchURL(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("styleguide %q: %w", rawURL, err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("styleguide %q: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("styleguide %q: HTTP %d: %s", rawURL, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	// Content-Type is deliberately ignored: common hosts serve markdown as
	// text/plain or application/octet-stream; the text checks in Resolve catch
	// actual binaries.
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("styleguide %q: reading body: %w", rawURL, err)
	}
	if len(body) > MaxBytes {
		return "", fmt.Errorf("styleguide %q exceeds %d bytes", rawURL, MaxBytes)
	}
	return string(body), nil
}

func readFile(path string) (string, error) {
	expanded := expandPath(path)
	// Stat before open: opening a FIFO blocks until a writer appears, so the
	// regular-file check must happen on the path first.
	info, err := os.Stat(expanded)
	if err != nil {
		return "", fmt.Errorf("styleguide %q: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("styleguide %q is a directory", path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("styleguide %q is not a regular file", path)
	}
	file, err := os.Open(expanded)
	if err != nil {
		return "", fmt.Errorf("styleguide %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	// Re-check on the descriptor so a swap between Stat and Open cannot
	// bypass the checks, and enforce the cap while reading: Stat sizes are 0
	// for procfs-style files regardless of content.
	info, err = file.Stat()
	if err != nil {
		return "", fmt.Errorf("styleguide %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("styleguide %q is not a regular file", path)
	}
	if info.Size() > MaxBytes {
		return "", fmt.Errorf("styleguide %q exceeds %d bytes", path, MaxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("styleguide %q: reading: %w", path, err)
	}
	if len(data) > MaxBytes {
		return "", fmt.Errorf("styleguide %q exceeds %d bytes", path, MaxBytes)
	}
	return string(data), nil
}

// expandPath mirrors config's unexported helper: only "~" and "~/..." are
// expanded; "~user/..." is left untouched (expanding it against the current
// user's home would mangle the path).
func expandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
