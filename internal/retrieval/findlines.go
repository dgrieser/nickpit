package retrieval

import (
	"context"
	"fmt"
	"strings"
)

// FindLines returns the line ranges whose contents match the given code (a
// single line or a contiguous block). When path names a file only that file is
// scanned; when it names a directory its tree is walked; when it is empty the
// whole repository is searched. Line endings are normalized, so CRLF/CR input
// matches LF files, and leading/trailing whitespace on each line is ignored, so
// the snippet matches regardless of its indentation.
func (e *LocalEngine) FindLines(_ context.Context, repoRoot, path, code string) (*FindLinesResult, error) {
	code = normalizeFindLinesCode(code)
	matches := make([]FindLinesMatch, 0)
	normalizedPath, err := walkRepoTextFiles(repoRoot, path, func(relPath, content string) error {
		matches = append(matches, matchFindLines(relPath, content, code)...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("retrieval: find_lines %s: %w", searchScopeLabel(path, normalizedPath), err)
	}
	return &FindLinesResult{
		Path:          normalizedPath,
		CodeLineCount: findLinesCount(code),
		MatchCount:    len(matches),
		Matches:       matches,
	}, nil
}

// FindLinesIn builds a single-file find_lines result from already-retrieved file
// content, so callers that obtain a FileContent another way (or fakes in tests)
// share the same matching logic as the LocalEngine.
func FindLinesIn(content *FileContent, code string) *FindLinesResult {
	code = normalizeFindLinesCode(code)
	matches := matchFindLines(content.Path, content.Content, code)
	return &FindLinesResult{
		Path:          content.Path,
		CodeLineCount: findLinesCount(code),
		MatchCount:    len(matches),
		Matches:       matches,
	}
}

func matchFindLines(relPath, content, code string) []FindLinesMatch {
	matches := make([]FindLinesMatch, 0)
	// Leading/trailing whitespace on each line is ignored, so a snippet matches
	// regardless of how it is indented in the file.
	codeLines := trimFindLines(splitFindLines(normalizeFindLinesCode(code)))
	if len(codeLines) == 0 || allBlank(codeLines) {
		return matches
	}
	rawFileLines := splitFindLines(normalizeFindLinesContent(content))
	fileLines := trimFindLines(rawFileLines)
	language := detectLanguage(relPath)
	for i := 0; i+len(codeLines) <= len(fileLines); i++ {
		if !findLinesEqual(fileLines[i:i+len(codeLines)], codeLines) {
			continue
		}
		matches = append(matches, FindLinesMatch{
			Path:      relPath,
			StartLine: i + 1,
			EndLine:   i + len(codeLines),
			Language:  language,
			LineCount: len(codeLines),
			Content:   strings.Join(rawFileLines[i:i+len(codeLines)], "\n"),
		})
	}
	return matches
}

func trimFindLines(lines []string) []string {
	trimmed := make([]string, len(lines))
	for i, line := range lines {
		trimmed[i] = strings.TrimSpace(line)
	}
	return trimmed
}

func allBlank(lines []string) bool {
	for _, line := range lines {
		if line != "" {
			return false
		}
	}
	return true
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
