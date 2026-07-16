package retrieval

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
)

type LocalEngine struct{}

var escapedSearchQueryPattern = regexp.MustCompile(`\\([^\w\s])`)

func NewLocalEngine() *LocalEngine {
	return &LocalEngine{}
}

// maxRetrievedFileBytes bounds how much of a single file is read into memory
// and surfaced to the model. A pathological input (e.g. a multi-MB minified
// bundle) would otherwise be fully buffered and flood the LLM context.
const maxRetrievedFileBytes = 5 << 20 // 5 MiB

func (e *LocalEngine) GetFile(_ context.Context, repoRoot, path string) (*FileContent, error) {
	normalizedPath, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return nil, fmt.Errorf("retrieval: reading %s: %w", path, err)
	}
	data, truncated, err := readFileCapped(repoRoot, fullPath, maxRetrievedFileBytes)
	if err != nil {
		return nil, fmt.Errorf("retrieval: reading %s: %w", normalizedPath, err)
	}
	return &FileContent{
		Path:      normalizedPath,
		Content:   normalizeText(string(data)),
		Language:  detectLanguage(normalizedPath),
		Truncated: truncated,
	}, nil
}

// readFileCapped reads up to limit bytes from fullPath (which must resolve
// inside repoRoot, enforced via repofs.Open), reporting whether the file was
// longer than the limit. It reads at most limit+1 bytes so truncation is
// detected without buffering the whole file.
func readFileCapped(repoRoot, fullPath string, limit int) ([]byte, bool, error) {
	f, err := repofs.Open(repoRoot, fullPath)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > limit {
		truncated := data[:limit]
		// Cutting at a byte boundary can split a multi-byte UTF-8 rune; drop the
		// (at most UTFMax-1) trailing bytes of an incomplete rune so the result
		// stays valid UTF-8.
		for i := 0; i < utf8.UTFMax-1 && len(truncated) > 0; i++ {
			if r, size := utf8.DecodeLastRune(truncated); r == utf8.RuneError && size <= 1 {
				truncated = truncated[:len(truncated)-1]
				continue
			}
			break
		}
		return truncated, true, nil
	}
	return data, false, nil
}

func (e *LocalEngine) ListFiles(_ context.Context, repoRoot, path string, depth int) (*DirectoryListing, error) {
	normalizedPath, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return nil, fmt.Errorf("retrieval: listing %s: %w", path, err)
	}
	if depth <= 0 {
		depth = 1
	}
	ignores := repofs.NewIgnoreMatcher(repoRoot)
	files, err := listFilesRecursive(fullPath, normalizedPath, depth, ignores)
	if err != nil {
		return nil, fmt.Errorf("retrieval: listing %s: %w", normalizedPath, err)
	}
	sort.Strings(files)
	return &DirectoryListing{
		Path:  normalizedPath,
		Files: files,
	}, nil
}

func listFilesRecursive(fullPath, relativePath string, depth int, ignores repofs.IgnoreMatcher) ([]string, error) {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" {
			continue
		}
		displayPath := name
		if relativePath != "" {
			displayPath = relativePath + "/" + name
		}
		if ignores.IsIgnored(displayPath, entry.IsDir()) {
			continue
		}
		if entry.IsDir() {
			files = append(files, displayPath+"/")
			if depth > 1 {
				childFiles, err := listFilesRecursive(filepath.Join(fullPath, name), displayPath, depth-1, ignores)
				if err != nil {
					return nil, err
				}
				files = append(files, childFiles...)
			}
			continue
		}
		files = append(files, displayPath)
	}
	return files, nil
}

func (e *LocalEngine) GetFileSlice(_ context.Context, repoRoot, path string, start, end int) (*FileSlice, error) {
	normalizedPath, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return nil, fmt.Errorf("retrieval: reading %s: %w", path, err)
	}
	if start <= 0 {
		start = 1
	}
	if end > 0 && end < start {
		return nil, fmt.Errorf("retrieval: invalid line range %d-%d", start, end)
	}
	f, err := repofs.Open(repoRoot, fullPath)
	if err != nil {
		return nil, fmt.Errorf("retrieval: reading %s: %w", normalizedPath, err)
	}
	defer func() { _ = f.Close() }()

	// Stream line by line so ranges beyond the whole-file byte cap stay
	// reachable: only the selected lines are buffered, and the returned
	// content itself stays capped at maxRetrievedFileBytes.
	var (
		selected  []string
		byteCount int
		lineNum   int
		truncated bool
	)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), maxRetrievedFileBytes)
	scanner.Split(scanLinesAnyEnding)
	for scanner.Scan() {
		lineNum++
		if lineNum < start {
			continue
		}
		if end > 0 && lineNum > end {
			break
		}
		line := scanner.Text()
		if len(selected) > 0 && byteCount+len(line)+1 > maxRetrievedFileBytes {
			truncated = true
			break
		}
		selected = append(selected, line)
		byteCount += len(line) + 1
	}
	if err := scanner.Err(); err != nil {
		if !errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("retrieval: reading %s: %w", normalizedPath, err)
		}
		// A single line larger than the cap: stop here and report the clip.
		truncated = true
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("retrieval: invalid line range %d-%d", start, lineNum)
	}
	actualEnd := start + len(selected) - 1
	if end > 0 && actualEnd < end {
		truncated = true
	}
	return &FileSlice{
		Path:      normalizedPath,
		StartLine: start,
		EndLine:   actualEnd,
		Content:   strings.Join(selected, "\n"),
		Language:  detectLanguage(normalizedPath),
		Truncated: truncated,
	}, nil
}

// scanLinesAnyEnding is a bufio.SplitFunc that terminates lines on "\n",
// "\r\n", or a lone "\r", mirroring NormalizeLineEndings for streamed reads.
func scanLinesAnyEnding(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\n' {
			return i + 1, data[:i], nil
		}
		// "\r": one more byte is needed to distinguish "\r\n" from a lone "\r".
		if i+1 < len(data) {
			if data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
			return i + 1, data[:i], nil
		}
		if atEOF {
			return i + 1, data[:i], nil
		}
		return 0, nil, nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// Search locates a literal query in the repo. A single-line query matches any
// line containing it as a substring; a multi-line query matches consecutive
// lines equal to the query lines after per-line whitespace trimming (lean
// matching), so a code block matches regardless of its indentation. Both modes
// are case-insensitive unless caseSensitive is set. A negative contextLines
// selects the per-mode default (5 for single-line, 0 for multi-line).
func (e *LocalEngine) Search(_ context.Context, repoRoot, path, query string, contextLines, maxResults int, caseSensitive bool) (*SearchResults, error) {
	query = NormalizeSearchQuery(query)
	if query == "" {
		return nil, fmt.Errorf("retrieval: missing search query")
	}
	if contextLines < 0 {
		contextLines = DefaultSearchContextLines(query)
	}

	results, normalizedPath, err := runFileSearch(repoRoot, path, contextLines, maxResults, literalQueryMatcher(query, caseSensitive))
	if err != nil {
		return nil, err
	}

	effectiveQuery := query
	unescapedQuery := unescapeSearchQuery(query)
	if len(results) == 0 && unescapedQuery != query {
		fallback, _, err := runFileSearch(repoRoot, path, contextLines, maxResults, literalQueryMatcher(unescapedQuery, caseSensitive))
		if err != nil {
			return nil, err
		}
		if len(fallback) > 0 {
			results = fallback
			effectiveQuery = unescapedQuery
		}
	}

	return &SearchResults{
		Path:          normalizedPath,
		Query:         effectiveQuery,
		ContextLines:  contextLines,
		MaxResults:    maxResults,
		CaseSensitive: caseSensitive,
		ResultCount:   len(results),
		Results:       results,
	}, nil
}

// literalQueryMatcher builds the span matcher for an already-normalized
// literal query: substring matching per line for a single-line query, lean
// block matching (see codeBlockMatcher) for a multi-line one.
func literalQueryMatcher(query string, caseSensitive bool) func(rawLines []string) []lineSpan {
	queryLines := SplitFindLines(query)
	if len(queryLines) > 1 {
		return codeBlockMatcher(queryLines, caseSensitive)
	}
	needle := foldSearchCase(query, caseSensitive)
	return func(rawLines []string) []lineSpan {
		var spans []lineSpan
		for i, line := range rawLines {
			if strings.Contains(foldSearchCase(line, caseSensitive), needle) {
				spans = append(spans, lineSpan{start: i, end: i})
			}
		}
		return spans
	}
}

func (e *LocalEngine) SearchRegex(_ context.Context, repoRoot, path string, pattern *regexp.Regexp, contextLines, maxResults int) (*SearchResults, error) {
	if pattern == nil {
		return nil, fmt.Errorf("retrieval: missing search pattern")
	}
	if contextLines < 0 {
		contextLines = DefaultSearchContextLines(pattern.String())
	}
	matcher := func(rawLines []string) []lineSpan {
		var spans []lineSpan
		for i, line := range rawLines {
			if pattern.MatchString(line) {
				spans = append(spans, lineSpan{start: i, end: i})
			}
		}
		return spans
	}
	results, normalizedPath, err := runFileSearch(repoRoot, path, contextLines, maxResults, matcher)
	if err != nil {
		return nil, err
	}
	return &SearchResults{
		Path:         normalizedPath,
		Query:        pattern.String(),
		ContextLines: contextLines,
		MaxResults:   maxResults,
		ResultCount:  len(results),
		Results:      results,
	}, nil
}

func runFileSearch(repoRoot, path string, contextLines, maxResults int, matcher func(rawLines []string) []lineSpan) ([]SearchResult, string, error) {
	if contextLines < 0 {
		contextLines = 0
	}
	results := make([]SearchResult, 0)
	appendMatches := func(relPath, content string) error {
		lines := splitLines(content)
		language := detectLanguage(relPath)
		for _, span := range matcher(lines) {
			result := SearchResult{
				CodeLocation: codeLocationForSpan(relPath, language, lines, span),
			}
			if contextLines > 0 {
				if start := max(span.start-contextLines, 0); start < span.start {
					result.ContextBefore = &SearchContext{
						StartLine: start + 1,
						EndLine:   span.start,
						Content:   strings.Join(lines[start:span.start], "\n"),
					}
				}
				if end := min(span.end+contextLines, len(lines)-1); end > span.end {
					result.ContextAfter = &SearchContext{
						StartLine: span.end + 2,
						EndLine:   end + 1,
						Content:   strings.Join(lines[span.end+1:end+1], "\n"),
					}
				}
			}
			results = append(results, result)
			if maxResults > 0 && len(results) >= maxResults {
				return errSearchLimitReached
			}
		}
		return nil
	}

	normalizedPath, err := walkRepoTextFiles(repoRoot, path, appendMatches)
	if err != nil && err != errSearchLimitReached {
		return nil, normalizedPath, fmt.Errorf("retrieval: searching %s: %w", searchScopeLabel(path, normalizedPath), err)
	}
	return results, normalizedPath, nil
}

// walkRepoTextFiles resolves path within repoRoot and invokes visit with the
// repo-relative path and normalized text content of each non-ignored text file.
// When path is a file, only that file is visited; when it is a directory (or
// empty, meaning the repo root), its tree is walked. The returned string is the
// normalized path that was resolved. A non-nil error from visit stops the walk
// and is returned to the caller.
func walkRepoTextFiles(repoRoot, path string, visit func(relPath, content string) error) (string, error) {
	normalizedPath, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return normalizedPath, err
	}
	ignores := repofs.NewIgnoreMatcher(repoRoot)

	visitFile := func(relPath string, respectIgnores bool) error {
		if respectIgnores && ignores.IsIgnored(relPath, false) {
			return nil
		}
		_, fileFullPath, err := repofs.ResolvePath(repoRoot, relPath)
		if err != nil {
			return err
		}
		data, _, err := readFileCapped(repoRoot, fileFullPath, maxRetrievedFileBytes)
		if err != nil {
			return nil
		}
		if !isTextContent(data) {
			return nil
		}
		return visit(relPath, normalizeText(string(data)))
	}

	if !info.IsDir() {
		if err := visitFile(normalizedPath, false); err != nil {
			return normalizedPath, err
		}
		return normalizedPath, nil
	}

	walkErr := filepath.WalkDir(fullPath, func(currentPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" && currentPath != fullPath {
				return filepath.SkipDir
			}
			relDir, err := repofs.RelPath(repoRoot, currentPath)
			if err == nil && relDir != "" && ignores.IsIgnored(relDir, true) {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, err := repofs.RelPath(repoRoot, currentPath)
		if err != nil {
			return err
		}
		return visitFile(relPath, true)
	})
	if walkErr != nil {
		return normalizedPath, walkErr
	}
	return normalizedPath, nil
}

// searchScopeLabel renders the path used in scope error messages, falling back
// to the repo root when an empty path resolves to "".
func searchScopeLabel(path, normalizedPath string) string {
	if normalizedPath != "" {
		return normalizedPath
	}
	if path != "" {
		return path
	}
	return "."
}

func unescapeSearchQuery(query string) string {
	return escapedSearchQueryPattern.ReplaceAllString(query, `$1`)
}

var errSearchLimitReached = fmt.Errorf("search result limit reached")

func normalizeText(text string) string {
	text = NormalizeLineEndings(text)
	return strings.TrimSuffix(text, "\n")
}

func splitLines(text string) []string {
	text = normalizeText(text)
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

func detectLanguage(path string) string {
	return filetype.DetectLanguage(path)
}

func isTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if !utf8.Valid(data) {
		return false
	}
	return !slices.Contains(data, 0)
}
