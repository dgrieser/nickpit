package retrieval

import (
	"context"
	"fmt"
	"strings"
)

const maxFindLinesMatches = 100

// FindLines returns the line ranges whose contents match the given code (a
// single line or a contiguous block). When path names a file only that file is
// scanned; when it names a directory its tree is walked; when it is empty the
// whole repository is searched. Line endings are normalized, so CRLF/CR input
// matches LF files, and leading/trailing whitespace on each line is ignored, so
// the snippet matches regardless of its indentation.
func (e *LocalEngine) FindLines(_ context.Context, repoRoot, path, code string) (*FindLinesResult, error) {
	specifiedCode := code
	code = NormalizeFindLinesCode(code)
	matches := make([]FindLinesMatch, 0)
	normalizedPath, err := walkRepoTextFiles(repoRoot, path, func(relPath, content string) error {
		remaining := maxFindLinesMatches - len(matches)
		if remaining <= 0 {
			return errSearchLimitReached
		}
		fileMatches := matchFindLinesLimit(relPath, content, code, remaining)
		matches = append(matches, fileMatches...)
		if len(fileMatches) >= remaining {
			return errSearchLimitReached
		}
		return nil
	})
	if err != nil && err != errSearchLimitReached {
		return nil, fmt.Errorf("retrieval: find_lines %s: %w", searchScopeLabel(path, normalizedPath), err)
	}
	return &FindLinesResult{
		Path:       normalizedPath,
		Code:       specifiedCode,
		MatchCount: len(matches),
		Matches:    matches,
	}, nil
}

// FindLinesIn builds a single-file find_lines result from already-retrieved file
// content, so callers that obtain a FileContent another way (or fakes in tests)
// share the same matching logic as the LocalEngine.
func FindLinesIn(content *FileContent, code string) *FindLinesResult {
	if content == nil {
		return &FindLinesResult{
			Code: code,
		}
	}
	normalizedCode := NormalizeFindLinesCode(code)
	matches := matchFindLinesLimit(content.Path, content.Content, normalizedCode, maxFindLinesMatches)
	return &FindLinesResult{
		Path:       content.Path,
		Code:       code,
		MatchCount: len(matches),
		Matches:    matches,
	}
}

func matchFindLinesLimit(relPath, content, code string, maxMatches int) []FindLinesMatch {
	matches := make([]FindLinesMatch, 0)
	matcher := codeBlockMatcher(SplitFindLines(NormalizeFindLinesCode(code)), true)
	rawFileLines := SplitFindLines(normalizeFindLinesContent(content))
	language := detectLanguage(relPath)
	for _, span := range matcher(rawFileLines) {
		matches = append(matches, FindLinesMatch{
			CodeLocation: codeLocationForSpan(relPath, language, rawFileLines, span),
		})
		if maxMatches > 0 && len(matches) >= maxMatches {
			return matches
		}
	}
	return matches
}

// lineSpan is a 0-based inclusive range of matched line indexes within a file.
type lineSpan struct {
	start int
	end   int
}

// codeBlockMatcher returns a matcher locating a code line or block within a
// file's raw lines using lean matching: leading/trailing whitespace on each
// line is ignored, so the snippet matches regardless of how it is indented in
// the file. Matching is case-insensitive unless caseSensitive is set.
func codeBlockMatcher(codeLines []string, caseSensitive bool) func(rawLines []string) []lineSpan {
	trimmedCode := make([]string, len(codeLines))
	for i, line := range codeLines {
		trimmedCode[i] = foldSearchCase(strings.TrimSpace(line), caseSensitive)
	}
	if len(trimmedCode) == 0 || allBlank(trimmedCode) {
		return func([]string) []lineSpan { return nil }
	}
	return func(rawLines []string) []lineSpan {
		fileLines := make([]string, len(rawLines))
		for i, line := range rawLines {
			fileLines[i] = foldSearchCase(strings.TrimSpace(line), caseSensitive)
		}
		var spans []lineSpan
		for i := 0; i+len(trimmedCode) <= len(fileLines); i++ {
			if !findLinesEqual(fileLines[i:i+len(trimmedCode)], trimmedCode) {
				continue
			}
			spans = append(spans, lineSpan{start: i, end: i + len(trimmedCode) - 1})
		}
		return spans
	}
}

// codeLocationForSpan builds the canonical code_location for a matched span:
// the line range covers exactly the matched lines, and the content preserves
// the file's original indentation.
func codeLocationForSpan(relPath, language string, rawLines []string, span lineSpan) CodeLocation {
	return CodeLocation{
		FilePath: relPath,
		LineRange: LineRange{
			Start: span.start + 1,
			End:   span.end + 1,
			Count: span.end - span.start + 1,
		},
		Language: language,
		Content:  strings.Join(rawLines[span.start:span.end+1], "\n"),
	}
}

func foldSearchCase(text string, caseSensitive bool) string {
	if caseSensitive {
		return text
	}
	return strings.ToLower(text)
}

func allBlank(lines []string) bool {
	for _, line := range lines {
		if line != "" {
			return false
		}
	}
	return true
}

// NormalizeFindLinesCode normalizes a find_lines code argument: it converts
// CRLF/CR line endings to LF and trims surrounding blank lines, so callers in
// other packages share one definition of the canonical form.
func NormalizeFindLinesCode(code string) string {
	code = NormalizeLineEndings(code)
	lines := strings.Split(code, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func normalizeFindLinesContent(content string) string {
	content = NormalizeLineEndings(content)
	return strings.TrimSuffix(content, "\n")
}

// NormalizeSearchQuery returns the canonical form of a search query: line
// endings are normalized and surrounding blank lines removed; a single-line
// query is additionally trimmed of surrounding whitespace (a multi-line block
// keeps its indentation, which lean matching ignores anyway). Callers use it
// so dedup keys and execution agree on one canonical query.
func NormalizeSearchQuery(query string) string {
	query = NormalizeFindLinesCode(query)
	if FindLinesCount(query) <= 1 {
		return strings.TrimSpace(query)
	}
	return query
}

// DefaultSearchContextLines returns the context_lines default applied when a
// search caller does not specify one: 5 for a single-line query, 0 for a
// multi-line block, whose exact match already carries its own context.
func DefaultSearchContextLines(query string) int {
	if FindLinesCount(query) > 1 {
		return 0
	}
	return 5
}

// FindLinesCount returns the number of lines in a find_lines code argument
// after normalization, so a raw or CRLF snippet is counted the same as its
// canonical form.
func FindLinesCount(code string) int {
	code = NormalizeFindLinesCode(code)
	if code == "" {
		return 0
	}
	return strings.Count(code, "\n") + 1
}

// NormalizeLineEndings converts CRLF and CR line endings to LF without trimming
// surrounding content.
func NormalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

// SplitFindLines splits already-normalized find_lines text, returning nil for
// empty input to match find_lines matching semantics.
func SplitFindLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func findLinesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
