package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

func (e *LocalEngine) GetFile(_ context.Context, repoRoot, path string) (*FileContent, error) {
	normalizedPath, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return nil, fmt.Errorf("retrieval: reading %s: %w", path, err)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("retrieval: reading %s: %w", normalizedPath, err)
	}
	return &FileContent{
		Path:     normalizedPath,
		Content:  normalizeText(string(data)),
		Language: detectLanguage(normalizedPath),
	}, nil
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

func (e *LocalEngine) GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error) {
	full, err := e.GetFile(ctx, repoRoot, path)
	if err != nil {
		return nil, err
	}
	lines := splitLines(full.Content)
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return nil, fmt.Errorf("retrieval: invalid line range %d-%d", start, end)
	}
	return &FileSlice{
		Path:      path,
		StartLine: start,
		EndLine:   end,
		Content:   strings.Join(lines[start-1:end], "\n"),
		Language:  full.Language,
	}, nil
}

func (e *LocalEngine) Search(_ context.Context, repoRoot, path, query string, contextLines, maxResults int, caseSensitive bool) (*SearchResults, error) {
	if query == "" {
		return nil, fmt.Errorf("retrieval: missing search query")
	}

	literalMatcher := func(rawQuery string) func(string) bool {
		needle := rawQuery
		if !caseSensitive {
			needle = strings.ToLower(rawQuery)
		}
		return func(line string) bool {
			haystack := line
			if !caseSensitive {
				haystack = strings.ToLower(line)
			}
			return strings.Contains(haystack, needle)
		}
	}

	results, normalizedPath, contextLines, err := runFileSearch(repoRoot, path, contextLines, maxResults, literalMatcher(query))
	if err != nil {
		return nil, err
	}

	effectiveQuery := query
	unescapedQuery := unescapeSearchQuery(query)
	if len(results) == 0 && unescapedQuery != query {
		fallback, _, _, err := runFileSearch(repoRoot, path, contextLines, maxResults, literalMatcher(unescapedQuery))
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

func (e *LocalEngine) SearchRegex(_ context.Context, repoRoot, path string, pattern *regexp.Regexp, contextLines, maxResults int) (*SearchResults, error) {
	if pattern == nil {
		return nil, fmt.Errorf("retrieval: missing search pattern")
	}
	results, normalizedPath, contextLines, err := runFileSearch(repoRoot, path, contextLines, maxResults, pattern.MatchString)
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

func runFileSearch(repoRoot, path string, contextLines, maxResults int, match func(string) bool) ([]SearchResult, string, int, error) {
	normalizedPath, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return nil, "", contextLines, fmt.Errorf("retrieval: searching %s: %w", path, err)
	}
	if contextLines < 0 {
		contextLines = 5
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, normalizedPath, contextLines, fmt.Errorf("retrieval: searching %s: %w", normalizedPath, err)
	}
	ignores := repofs.NewIgnoreMatcher(repoRoot)

	results := make([]SearchResult, 0)
	appendMatches := func(relPath string) error {
		if ignores.IsIgnored(relPath, false) {
			return nil
		}
		_, fullPath, err := repofs.ResolvePath(repoRoot, relPath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil
		}
		if !isTextContent(data) {
			return nil
		}
		content := normalizeText(string(data))
		lines := splitLines(content)
		for i, line := range lines {
			if !match(line) {
				continue
			}
			start := i + 1 - contextLines
			if start <= 0 {
				start = 1
			}
			end := i + 1 + contextLines
			if end > len(lines) {
				end = len(lines)
			}
			results = append(results, SearchResult{
				Path:      relPath,
				StartLine: start,
				EndLine:   end,
				Language:  detectLanguage(relPath),
				Content:   strings.Join(lines[start-1:end], "\n"),
			})
			if maxResults > 0 && len(results) >= maxResults {
				return errSearchLimitReached
			}
		}
		return nil
	}

	if !info.IsDir() {
		if err := appendMatches(normalizedPath); err != nil && err != errSearchLimitReached {
			return nil, normalizedPath, contextLines, err
		}
		return results, normalizedPath, contextLines, nil
	}

	walkErr := filepath.WalkDir(fullPath, func(currentPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
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
		return appendMatches(relPath)
	})
	if walkErr != nil && walkErr != errSearchLimitReached {
		return nil, normalizedPath, contextLines, fmt.Errorf("retrieval: searching %s: %w", normalizedPath, walkErr)
	}
	return results, normalizedPath, contextLines, nil
}

func unescapeSearchQuery(query string) string {
	return escapedSearchQueryPattern.ReplaceAllString(query, `$1`)
}

var errSearchLimitReached = fmt.Errorf("search result limit reached")

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
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
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}
