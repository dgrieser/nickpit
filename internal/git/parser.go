package git

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)

func ParseUnifiedDiff(diff string) ([]model.DiffHunk, []model.ChangedFile, error) {
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
			parts := strings.Split(line, " ")
			currentFile = strings.TrimPrefix(parts[len(parts)-1], "b/")
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
				return nil, nil, err
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
		return nil, nil, err
	}
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}
	if currentEntry != nil {
		files = append(files, *currentEntry)
	}
	return hunks, files, nil
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
