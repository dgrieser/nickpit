package review

import (
	"cmp"
	"slices"
	"strings"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/model"
)

// allowedDiffCodeLocations returns the authoritative old- and new-side hunk
// windows as complete code_location values. The same values drive scope
// validation, deterministic repair, and reviewer retry guidance.
func allowedDiffCodeLocations(hunks []model.DiffHunk) []model.CodeLocation {
	locations := make([]model.CodeLocation, 0, len(hunks)*2)
	for _, hunk := range hunks {
		path := normalizeReviewPath(hunk.FilePath)
		if path == "" {
			continue
		}
		language := hunk.Language
		if language == "" {
			language = filetype.DetectLanguage(path)
		}
		oldContent, newContent, oldCount, newCount := splitDiffHunkSides(hunk.Content)
		if hunk.OldLines > 0 {
			oldCount = hunk.OldLines
		}
		if hunk.NewLines > 0 {
			newCount = hunk.NewLines
		}
		if oldCount > 0 && hunk.OldStart > 0 {
			locations = append(locations, model.CodeLocation{
				FilePath: path,
				LineRange: model.LineRange{
					Start: hunk.OldStart,
					End:   hunk.OldStart + oldCount - 1,
					Count: oldCount,
				},
				Language: language,
				Content:  oldContent,
			})
		}
		if newCount > 0 && hunk.NewStart > 0 {
			locations = append(locations, model.CodeLocation{
				FilePath: path,
				LineRange: model.LineRange{
					Start: hunk.NewStart,
					End:   hunk.NewStart + newCount - 1,
					Count: newCount,
				},
				Language: language,
				Content:  newContent,
			})
		}
	}
	slices.SortFunc(locations, func(a, b model.CodeLocation) int {
		if n := cmp.Compare(a.FilePath, b.FilePath); n != 0 {
			return n
		}
		if n := cmp.Compare(a.LineRange.Start, b.LineRange.Start); n != 0 {
			return n
		}
		if n := cmp.Compare(a.LineRange.End, b.LineRange.End); n != 0 {
			return n
		}
		return cmp.Compare(a.Content, b.Content)
	})
	return slices.CompactFunc(locations, func(a, b model.CodeLocation) bool {
		return a.FilePath == b.FilePath &&
			a.LineRange.SameAnchor(b.LineRange) &&
			a.Content == b.Content
	})
}

func splitDiffHunkSides(content string) (oldContent, newContent string, oldCount, newCount int) {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return "", "", 0, 0
	}
	var oldLines, newLines []string
	for raw := range strings.SplitSeq(content, "\n") {
		marker := byte(' ')
		line := raw
		if raw != "" {
			marker = raw[0]
			if marker == ' ' || marker == '+' || marker == '-' || marker == '\\' {
				line = raw[1:]
			}
		}
		switch marker {
		case '+':
			newLines = append(newLines, line)
		case '-':
			oldLines = append(oldLines, line)
		case '\\':
			continue
		default:
			oldLines = append(oldLines, line)
			newLines = append(newLines, line)
		}
	}
	return strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"), len(oldLines), len(newLines)
}

// codeLocationOverlapsAllowed reports whether any line in loc overlaps any of
// the allowed code locations (for diff scoping: the old-side or new-side line
// ranges of the diff hunks, see allowedDiffCodeLocations). Ranges are
// intentionally accepted as-is when only part overlaps or when they span gaps
// between hunks.
func codeLocationOverlapsAllowed(loc model.CodeLocation, allowed []model.CodeLocation) bool {
	path := normalizeReviewPath(loc.FilePath)
	start := loc.LineRange.Start
	if path == "" || start <= 0 {
		return false
	}
	end := max(loc.LineRange.End, start)
	for _, candidate := range allowed {
		if normalizeReviewPath(candidate.FilePath) != path {
			continue
		}
		if rangesOverlap(start, end, candidate.LineRange.Start, candidate.LineRange.EffectiveCount()) {
			return true
		}
	}
	return false
}

func rangesOverlap(start, end, hunkStart, hunkLines int) bool {
	if hunkStart <= 0 || hunkLines <= 0 {
		return false
	}
	hunkEnd := hunkStart + hunkLines - 1
	return start <= hunkEnd && end >= hunkStart
}

func filterFindingsByDiffScope(findings []model.Finding, hunks []model.DiffHunk) ([]model.Finding, []model.Finding) {
	if len(findings) == 0 {
		return findings, nil
	}
	allowed := allowedDiffCodeLocations(hunks)
	kept := make([]model.Finding, 0, len(findings))
	dropped := make([]model.Finding, 0)
	for _, finding := range findings {
		if codeLocationOverlapsAllowed(finding.CodeLocation, allowed) {
			kept = append(kept, finding)
			continue
		}
		dropped = append(dropped, finding)
	}
	return kept, dropped
}
