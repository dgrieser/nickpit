// Package projectcontext loads the description a project gives of how it is
// actually built, deployed and used, and merges the sources that can supply it.
//
// There are two kinds of source, and they are trusted differently:
//
//   - The reviewed repository's own file (RepoPath), read through
//     model.BaseFileSource so it always comes from the base revision. It is
//     repo-authored, so a malformed file degrades the review rather than
//     failing it: a contributor must not be able to break every review by
//     committing bad YAML.
//   - Operator-supplied entries from the profile or --project-context, which
//     are local files or http(s) URLs. These fail fast, matching
//     internal/styleguide: judging a change under context the operator asked
//     for but did not get is worse than stopping.
//
// The file is deliberately NOT parsed through internal/config. config.loadFile
// runs os.ExpandEnv over the whole document before parsing, which would let a
// repository-supplied file pull operator environment values into a prompt.
package projectcontext

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/model"
	"gopkg.in/yaml.v3"
)

// RepoPath is where a reviewed repository declares its own context. It lives
// in a directory rather than beside .nickpit.yaml so it is never mistaken for
// the config file, which holds credentials and is gitignored.
const RepoPath = ".nickpit/context.yaml"

// MaxBytes caps one context document. It is far tighter than
// styleguide.MaxBytes because this text is injected into the system prompt of
// every agent, and the default workflow runs six review lanes in parallel.
const MaxBytes = 16 << 10

// Version is the only schema version understood. An absent version means 1.
const Version = 1

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Parse decodes one context document. Unknown top-level keys are rejected so a
// misspelled field is reported rather than silently ignored — a dropped
// `trust_boundaries` would quietly change how findings are judged.
func Parse(data []byte, source string) (*model.ProjectContext, error) {
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("project context %q exceeds %d bytes", source, MaxBytes)
	}
	if !utf8.Valid(data) || bytes.ContainsRune(data, 0) {
		return nil, fmt.Errorf("project context %q is not text", source)
	}
	var out model.ProjectContext
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&out); err != nil {
		// An empty or comment-only document decodes to EOF, not an error value.
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("project context %q is empty", source)
		}
		return nil, fmt.Errorf("project context %q: %w", source, err)
	}
	if out.Version != 0 && out.Version != Version {
		return nil, fmt.Errorf("project context %q: unsupported version %d (want %d)", source, out.Version, Version)
	}
	out.Version = Version
	normalize(&out)
	if out.Empty() {
		return nil, fmt.Errorf("project context %q declares no fields", source)
	}
	out.Sources = []string{source}
	return &out, nil
}

// LoadRepo reads the reviewed repository's own context through the source's
// base-revision reader, so the file always comes from the revision the target
// project controls. A source with no such reader contributes nothing.
//
// Failures degrade rather than abort, and are returned as warnings for
// OmittedSections: the file is repo-authored, and a contributor who commits
// unparseable YAML must not be able to fail every review of that project.
func LoadRepo(ctx context.Context, src model.ReviewSource, req model.ReviewRequest) (*model.ProjectContext, []string) {
	reader, ok := src.(model.BaseFileSource)
	if !ok {
		return nil, nil
	}
	data, found, err := reader.ReadBaseFile(ctx, req, RepoPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("project context %s not read: %v", RepoPath, err)}
	}
	if !found {
		return nil, nil
	}
	parsed, err := Parse(data, RepoPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("project context ignored: %v", err)}
	}
	return parsed, nil
}

// Resolve loads every operator-supplied spec (local path or http(s) URL) in
// order, preserving it. Any failure aborts the run.
func Resolve(ctx context.Context, specs []string, baseDir string) ([]*model.ProjectContext, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	out := make([]*model.ProjectContext, 0, len(specs))
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		var data []byte
		var err error
		if isURL(spec) {
			data, err = fetchURL(ctx, spec)
		} else {
			data, err = readFile(spec, baseDir)
		}
		if err != nil {
			return nil, err
		}
		parsed, err := Parse(data, spec)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

// Merge applies entries in order: the reviewed repository's own context first,
// then each operator entry. A later non-empty scalar replaces an earlier one,
// so an operator can correct what a repository claims about itself; lists
// concatenate and dedupe, so an operator can add a trust boundary without
// having to restate the ones the repository already documented.
func Merge(entries ...*model.ProjectContext) *model.ProjectContext {
	var out *model.ProjectContext
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if out == nil {
			clone := *entry
			clone.Data = append([]string(nil), entry.Data...)
			clone.TrustBoundaries = append([]string(nil), entry.TrustBoundaries...)
			clone.Assumptions = append([]string(nil), entry.Assumptions...)
			clone.NonGoals = append([]string(nil), entry.NonGoals...)
			clone.Sources = append([]string(nil), entry.Sources...)
			out = &clone
			continue
		}
		replaceScalar(&out.Summary, entry.Summary)
		replaceScalar(&out.Deployment, entry.Deployment)
		replaceScalar(&out.Users, entry.Users)
		replaceScalar(&out.Criticality, entry.Criticality)
		replaceScalar(&out.Notes, entry.Notes)
		out.Data = appendUnique(out.Data, entry.Data)
		out.TrustBoundaries = appendUnique(out.TrustBoundaries, entry.TrustBoundaries)
		out.Assumptions = appendUnique(out.Assumptions, entry.Assumptions)
		out.NonGoals = appendUnique(out.NonGoals, entry.NonGoals)
		out.Sources = appendUnique(out.Sources, entry.Sources)
	}
	if out.Empty() {
		return nil
	}
	return out
}

func replaceScalar(dst *string, value string) {
	if value != "" {
		*dst = value
	}
}

func appendUnique(dst, extra []string) []string {
	seen := make(map[string]struct{}, len(dst)+len(extra))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range extra {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}

func normalize(pc *model.ProjectContext) {
	pc.Summary = strings.TrimSpace(pc.Summary)
	pc.Deployment = strings.TrimSpace(pc.Deployment)
	pc.Users = strings.TrimSpace(pc.Users)
	pc.Criticality = strings.TrimSpace(pc.Criticality)
	pc.Notes = strings.TrimSpace(pc.Notes)
	pc.Data = normalizeList(pc.Data)
	pc.TrustBoundaries = normalizeList(pc.TrustBoundaries)
	pc.Assumptions = normalizeList(pc.Assumptions)
	pc.NonGoals = normalizeList(pc.NonGoals)
	pc.Sources = nil
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isURL mirrors styleguide.isURL: only an explicit http(s) prefix is remote, so
// a Windows drive path never turns into a fetch.
func isURL(spec string) bool {
	lower := strings.ToLower(spec)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func fetchURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("project context %q: %w", rawURL, err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("project context %q: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("project context %q: HTTP %d: %s", rawURL, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("project context %q: reading body: %w", rawURL, err)
	}
	if len(body) > MaxBytes {
		return nil, fmt.Errorf("project context %q exceeds %d bytes", rawURL, MaxBytes)
	}
	return body, nil
}

// readFile mirrors styleguide.readFile: stat before open so a FIFO cannot block
// on a missing writer, re-check on the descriptor so a swap between stat and
// open cannot bypass the checks, and enforce the cap while reading because
// procfs-style files report size 0 regardless of content.
func readFile(path, baseDir string) ([]byte, error) {
	expanded := expandPath(path)
	if baseDir != "" && !filepath.IsAbs(expanded) {
		expanded = filepath.Join(baseDir, expanded)
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return nil, fmt.Errorf("project context %q: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("project context %q is a directory", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("project context %q is not a regular file", path)
	}
	file, err := os.Open(expanded)
	if err != nil {
		return nil, fmt.Errorf("project context %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil {
		return nil, fmt.Errorf("project context %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("project context %q is not a regular file", path)
	}
	if info.Size() > MaxBytes {
		return nil, fmt.Errorf("project context %q exceeds %d bytes", path, MaxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("project context %q: reading: %w", path, err)
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("project context %q exceeds %d bytes", path, MaxBytes)
	}
	return data, nil
}

// expandPath mirrors config's unexported helper: only "~" and "~/..." expand;
// "~user/..." is left untouched.
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
