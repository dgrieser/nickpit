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
	return DiffFilesFromUnifiedDiff(diff), hunks, files, nil
}

func DiffFilesFromUnifiedDiff(diff string) []model.DiffFile {
	sections := splitUnifiedDiff(diff)
	files := make([]model.DiffFile, 0, len(sections))
	for _, section := range sections {
		if section.path == "" {
			continue
		}
		files = append(files, model.DiffFile{
			FilePath: section.path,
			Language: filetype.DetectLanguage(section.path),
			Content:  section.text,
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
