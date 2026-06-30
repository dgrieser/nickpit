package retrieval

import (
	"context"
	"fmt"
	"strings"
)

// FindLines returns the line ranges in path whose contents exactly match the
// given code (a single line or a contiguous block). Line endings are
// normalized, so CRLF/CR input matches LF files. The result mirrors the
// find_lines review tool so both share one implementation.
func (e *LocalEngine) FindLines(ctx context.Context, repoRoot, path, code string) (*FindLinesResult, error) {
	content, err := e.GetFile(ctx, repoRoot, path)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("retrieval: file content is nil for %s", path)
	}
	return FindLinesIn(content, code), nil
}

// FindLinesIn builds a find_lines result from already-retrieved file content,
// so callers that obtain a FileContent another way (or fakes in tests) share
// the same matching logic as the LocalEngine.
func FindLinesIn(content *FileContent, code string) *FindLinesResult {
	code = normalizeFindLinesCode(code)
	matches := matchFindLines(content.Content, code)
	result := &FindLinesResult{
		Path:          content.Path,
		Language:      content.Language,
		CodeLineCount: findLinesCount(code),
		MatchCount:    len(matches),
		Matches:       matches,
	}
	if content.Truncated {
		result.Truncated = true
		result.TruncatedNote = "file was too large and was truncated; matches beyond the retrieved prefix may be absent"
	}
	return result
}

func matchFindLines(content, code string) []FindLinesMatch {
	matches := make([]FindLinesMatch, 0)
	codeLines := splitFindLines(normalizeFindLinesCode(code))
	if len(codeLines) == 0 {
		return matches
	}
	fileLines := splitFindLines(normalizeFindLinesContent(content))
	for i := 0; i+len(codeLines) <= len(fileLines); i++ {
		if !findLinesEqual(fileLines[i:i+len(codeLines)], codeLines) {
			continue
		}
		matches = append(matches, FindLinesMatch{
			StartLine: i + 1,
			EndLine:   i + len(codeLines),
			LineCount: len(codeLines),
		})
	}
	return matches
}

func normalizeFindLinesCode(code string) string {
	code = strings.ReplaceAll(code, "\r\n", "\n")
	code = strings.ReplaceAll(code, "\r", "\n")
	return strings.Trim(code, "\n")
}

func normalizeFindLinesContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.TrimSuffix(content, "\n")
}

func findLinesCount(code string) int {
	code = normalizeFindLinesCode(code)
	if code == "" {
		return 0
	}
	return strings.Count(code, "\n") + 1
}

func splitFindLines(text string) []string {
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
