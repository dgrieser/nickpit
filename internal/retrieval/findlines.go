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
	// Leading/trailing whitespace on each line is ignored, so a snippet matches
	// regardless of how it is indented in the file.
	codeLines := trimFindLines(SplitFindLines(NormalizeFindLinesCode(code)))
	if len(codeLines) == 0 || allBlank(codeLines) {
		return matches
	}
	rawFileLines := SplitFindLines(normalizeFindLinesContent(content))
	fileLines := trimFindLines(rawFileLines)
	language := detectLanguage(relPath)
	for i := 0; i+len(codeLines) <= len(fileLines); i++ {
		if !findLinesEqual(fileLines[i:i+len(codeLines)], codeLines) {
			continue
		}
		matches = append(matches, FindLinesMatch{
			CodeLocation: FindLinesLocation{
				FilePath: relPath,
				LineRange: FindLinesRange{
					Start: i + 1,
					End:   i + len(codeLines),
					Count: len(codeLines),
				},
				Language: language,
				Content:  strings.Join(rawFileLines[i:i+len(codeLines)], "\n"),
			},
		})
		if maxMatches > 0 && len(matches) >= maxMatches {
			return matches
		}
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
