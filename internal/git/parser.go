package git

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/model"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)

func ParseUnifiedDiff(diff string) ([]model.DiffHunk, []model.ChangedFile, error) {
	_, hunks, files, err := ParseUnifiedDiffFormats(diff)
	return hunks, files, err
}

func ParseUnifiedDiffFormats(diff string) ([]model.DiffFile, []model.DiffHunk, []model.ChangedFile, error) {
	var (
		hunks        []model.DiffHunk
		files        []model.ChangedFile
		currentFile  string
		currentHunk  *model.DiffHunk
		currentEntry *model.ChangedFile
	)

	scanner := bufio.NewScanner(strings.NewReader(diff))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
			if currentEntry != nil {
				files = append(files, *currentEntry)
			}
			currentFile = parseDiffGitPath(line)
			currentEntry = &model.ChangedFile{Path: currentFile, Status: model.FileModified}
		case strings.HasPrefix(line, "new file mode "):
			if currentEntry != nil {
				currentEntry.Status = model.FileAdded
			}
		case strings.HasPrefix(line, "deleted file mode "):
			if currentEntry != nil {
				currentEntry.Status = model.FileDeleted
			}
		case strings.HasPrefix(line, "rename to "):
			if currentEntry != nil {
				currentEntry.Status = model.FileRenamed
				currentEntry.Path = strings.TrimPrefix(line, "rename to ")
				currentFile = currentEntry.Path
			}
		case strings.HasPrefix(line, "@@@"):
			// Combined-diff ("merge state") hunk header, e.g.
			// "@@@ -1,4 -1,4 +1,4 @@@". The two-way parsing below cannot
			// represent it; previously the header and the whole hunk body were
			// swallowed as content of the preceding hunk. Skip this file's
			// combined hunks instead of misparsing them — the ChangedFile
			// entry survives without hunk or line-count data.
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		case strings.HasPrefix(line, "@@ "):
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			parsed, err := parseHunkHeader(currentFile, line)
			if err != nil {
				return nil, nil, nil, err
			}
			currentHunk = parsed
		default:
			if currentHunk != nil {
				currentHunk.Content += line + "\n"
				if currentEntry != nil {
					switch {
					case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
						currentEntry.Additions++
					case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
						currentEntry.Deletions++
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, nil, err
	}
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}
	if currentEntry != nil {
		files = append(files, *currentEntry)
	}
	diffFiles := DiffFilesFromUnifiedDiff(diff)
	// Hunk languages come from path-only detection; refine them with the
	// content-aware per-file classification.
	langByPath := make(map[string]string, len(diffFiles))
	for _, file := range diffFiles {
		langByPath[file.FilePath] = file.Language
	}
	for i := range hunks {
		if language, ok := langByPath[hunks[i].FilePath]; ok {
			hunks[i].Language = language
		}
	}
	return diffFiles, hunks, files, nil
}

func DiffFilesFromUnifiedDiff(diff string) []model.DiffFile {
	sections := splitUnifiedDiff(diff)
	files := make([]model.DiffFile, 0, len(sections))
	for _, section := range sections {
		if section.path == "" {
			continue
		}
		classification := filetype.Classify(section.path, section.text)
		files = append(files, model.DiffFile{
			FilePath:  section.path,
			Language:  classification.Language,
			Content:   section.text,
			Generated: classification.Generated,
		})
	}
	return files
}

type diffSection struct {
	path string
	text string
}

func splitUnifiedDiff(diff string) []diffSection {
	var sections []diffSection
	var current strings.Builder
	currentPath := ""
	inSection := false
	flush := func() {
		if !inSection {
			return
		}
		sections = append(sections, diffSection{path: currentPath, text: current.String()})
		current.Reset()
	}
	remaining := diff
	for len(remaining) > 0 {
		idx := strings.IndexByte(remaining, '\n')
		var line string
		if idx == -1 {
			line = remaining
			remaining = ""
		} else {
			line = remaining[:idx+1]
			remaining = remaining[idx+1:]
		}
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			inSection = true
			currentPath = parseDiffGitPath(line)
		}
		if inSection {
			current.WriteString(line)
		}
	}
	flush()
	return sections
}

func parseDiffGitPath(line string) string {
	const prefix = "diff --git "
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	rest := line[len(prefix):]
	// Git C-quotes a path containing special characters (quotes, backslashes,
	// control chars, and — with the default core.quotePath — non-ASCII bytes
	// as octal escapes): `diff --git "a/x y" "b/x y"`. Either side may be
	// quoted independently.
	if path, ok := parseQuotedDiffGitPath(rest); ok {
		return path
	}
	if idx := strings.LastIndex(rest, " b/"); idx >= 0 {
		return rest[idx+len(" b/"):]
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	value := fields[len(fields)-1]
	value = strings.TrimPrefix(value, "b/")
	value = strings.TrimPrefix(value, "a/")
	return value
}

// parseQuotedDiffGitPath extracts the b-side path from the remainder of a
// "diff --git " line when at least one side is C-quoted. It returns ok=false
// when neither side is quoted (the caller then uses the unquoted heuristics).
func parseQuotedDiffGitPath(rest string) (string, bool) {
	// a-side quoted: `"a/..." <b-side>`.
	if strings.HasPrefix(rest, `"`) {
		aPath, consumed, ok := unquoteCStyle(rest)
		if !ok {
			return "", false
		}
		remainder := strings.TrimPrefix(rest[consumed:], " ")
		if strings.HasPrefix(remainder, `"`) {
			bPath, _, ok := unquoteCStyle(remainder)
			if !ok {
				return "", false
			}
			return strings.TrimPrefix(bPath, "b/"), true
		}
		if remainder != "" {
			return strings.TrimPrefix(remainder, "b/"), true
		}
		// Only one token; fall back to the a-side path.
		return strings.TrimPrefix(aPath, "a/"), true
	}
	// a-side unquoted, b-side quoted: `a/... "b/..."`. The quoted b token is
	// the one that ends exactly at the end of the line.
	for idx := strings.Index(rest, ` "`); idx >= 0; {
		candidate := rest[idx+1:]
		if bPath, consumed, ok := unquoteCStyle(candidate); ok && consumed == len(candidate) {
			return strings.TrimPrefix(bPath, "b/"), true
		}
		next := strings.Index(rest[idx+1:], ` "`)
		if next < 0 {
			break
		}
		idx += 1 + next
	}
	return "", false
}

// unquoteCStyle decodes a git C-style quoted string starting at s[0] == '"'.
// It handles doubled backslashes, escaped quotes, the usual control escapes,
// and 1-3 digit octal escapes (git's default encoding for non-ASCII bytes).
// It returns the decoded value and the number of bytes consumed including both
// quotes.
func unquoteCStyle(s string) (string, int, bool) {
	if len(s) < 2 || s[0] != '"' {
		return "", 0, false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return b.String(), i + 1, true
		}
		if c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		i++
		if i >= len(s) {
			return "", 0, false
		}
		switch e := s[i]; e {
		case '"', '\\':
			b.WriteByte(e)
			i++
		case 'a':
			b.WriteByte('\a')
			i++
		case 'b':
			b.WriteByte('\b')
			i++
		case 'f':
			b.WriteByte('\f')
			i++
		case 'n':
			b.WriteByte('\n')
			i++
		case 'r':
			b.WriteByte('\r')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		case 'v':
			b.WriteByte('\v')
			i++
		default:
			if e < '0' || e > '7' {
				return "", 0, false
			}
			value := 0
			digits := 0
			for i < len(s) && digits < 3 && s[i] >= '0' && s[i] <= '7' {
				value = value*8 + int(s[i]-'0')
				i++
				digits++
			}
			b.WriteByte(byte(value))
		}
	}
	return "", 0, false
}

func parseHunkHeader(path, line string) (*model.DiffHunk, error) {
	matches := hunkHeader.FindStringSubmatch(line)
	if len(matches) != 5 {
		return nil, fmt.Errorf("git: invalid hunk header %q", line)
	}
	oldStart, _ := strconv.Atoi(matches[1])
	oldLines := toCount(matches[2])
	newStart, _ := strconv.Atoi(matches[3])
	newLines := toCount(matches[4])
	return &model.DiffHunk{
		FilePath: path,
		Language: filetype.DetectLanguage(path),
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, nil
}

func toCount(value string) int {
	if value == "" {
		return 1
	}
	count, _ := strconv.Atoi(value)
	return count
}
