package review

import (
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

// codeLocationOverlapsDiff reports whether any line in loc overlaps any
// old-side or new-side line represented by the matching file's diff hunks.
// Ranges are intentionally accepted as-is when only part overlaps or when they
// span gaps between hunks.
func codeLocationOverlapsDiff(loc model.CodeLocation, hunks []model.DiffHunk) bool {
	path := normalizeReviewPath(loc.FilePath)
	start := loc.LineRange.Start
	if path == "" || start <= 0 {
		return false
	}
	end := max(loc.LineRange.End, start)
	for _, hunk := range hunks {
		if normalizeReviewPath(hunk.FilePath) != path {
			continue
		}
		if hunkOverlapsLineRange(hunk, start, end) {
			return true
		}
	}
	return false
}

func hunkOverlapsLineRange(hunk model.DiffHunk, start, end int) bool {
	// Header ranges are authoritative and already describe every old/new-side
	// line in the window, including removed lines. Fall back to walking content
	// only for legacy/test hunks without parsed counts.
	if hunk.OldLines > 0 || hunk.NewLines > 0 {
		return rangesOverlap(start, end, hunk.OldStart, hunk.OldLines) ||
			rangesOverlap(start, end, hunk.NewStart, hunk.NewLines)
	}
	content := strings.TrimRight(hunk.Content, "\n")
	if content == "" {
		return rangesOverlap(start, end, hunk.OldStart, hunk.OldLines) ||
			rangesOverlap(start, end, hunk.NewStart, hunk.NewLines)
	}

	oldLine := hunk.OldStart
	newLine := hunk.NewStart
	for raw := range strings.SplitSeq(content, "\n") {
		marker := byte(' ')
		if raw != "" {
			marker = raw[0]
		}
		switch marker {
		case '+':
			if lineInRange(newLine, start, end) {
				return true
			}
			newLine++
		case '-':
			if lineInRange(oldLine, start, end) {
				return true
			}
			oldLine++
		case '\\':
			// "No newline at end of file" marker has no line on either side.
		default:
			if lineInRange(oldLine, start, end) || lineInRange(newLine, start, end) {
				return true
			}
			oldLine++
			newLine++
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

func lineInRange(line, start, end int) bool {
	return line > 0 && line >= start && line <= end
}

func filterFindingsByDiffScope(findings []model.Finding, hunks []model.DiffHunk) ([]model.Finding, int) {
	if len(findings) == 0 {
		return findings, 0
	}
	kept := make([]model.Finding, 0, len(findings))
	for _, finding := range findings {
		if codeLocationOverlapsDiff(finding.CodeLocation, hunks) {
			kept = append(kept, finding)
		}
	}
	return kept, len(findings) - len(kept)
}
