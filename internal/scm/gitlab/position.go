package gitlab

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

// position is a GitLab diff-note position for a text diff. NewLine/OldLine use
// pointers so they can be emitted selectively: an added line sets only new_line,
// a deleted line only old_line, and an unchanged (context) line both — matching
// GitLab's position rules.
type position struct {
	PositionType string     `json:"position_type"`
	BaseSHA      string     `json:"base_sha"`
	HeadSHA      string     `json:"head_sha"`
	StartSHA     string     `json:"start_sha"`
	OldPath      string     `json:"old_path"`
	NewPath      string     `json:"new_path"`
	OldLine      *int       `json:"old_line,omitempty"`
	NewLine      *int       `json:"new_line,omitempty"`
	LineRange    *lineRange `json:"line_range,omitempty"`
}

type lineRange struct {
	Start linePoint `json:"start"`
	End   linePoint `json:"end"`
}

type linePoint struct {
	LineCode string `json:"line_code"`
	Type     string `json:"type,omitempty"`
	OldLine  *int   `json:"old_line,omitempty"`
	NewLine  *int   `json:"new_line,omitempty"`
}

// lineLoc is where a new-side line sits in the diff: its new-side number, the
// old-side cursor at that point, and whether it is an added line (new side only).
type lineLoc struct {
	oldLine int
	newLine int
	added   bool
}

// locateLine walks the change's hunks to find the new-side line `newLine`,
// returning its location, or false when the line is not part of the diff.
func locateLine(change MRChange, newLine int) (lineLoc, bool) {
	for _, hunk := range change.Hunks {
		oldCursor := hunk.OldStart
		newCursor := hunk.NewStart
		for _, raw := range strings.Split(strings.TrimSuffix(hunk.Content, "\n"), "\n") {
			if raw == "" {
				continue
			}
			switch raw[0] {
			case '+':
				if newCursor == newLine {
					return lineLoc{oldLine: oldCursor, newLine: newCursor, added: true}, true
				}
				newCursor++
			case '-':
				oldCursor++
			case '\\':
				// "\ No newline at end of file": no line on either side.
			default: // ' ' context (and any unexpected prefix treated as context)
				if newCursor == newLine {
					return lineLoc{oldLine: oldCursor, newLine: newCursor}, true
				}
				oldCursor++
				newCursor++
			}
		}
	}
	return lineLoc{}, false
}

// mapPosition builds a single-line position for the given new-side line, or
// false when the line is not in the diff.
func mapPosition(change MRChange, refs DiffRefs, newLine int) (position, bool) {
	loc, ok := locateLine(change, newLine)
	if !ok {
		return position{}, false
	}
	pos := basePosition(change, refs)
	nl := loc.newLine
	pos.NewLine = &nl
	if !loc.added {
		ol := loc.oldLine
		pos.OldLine = &ol
	}
	return pos, true
}

// bestPosition returns a single-line position at the first line in the range
// (start..end) that is part of the diff.
func bestPosition(change MRChange, refs DiffRefs, lr model.LineRange) (position, bool) {
	end := max(lr.End, lr.Start)
	for n := lr.Start; n <= end; n++ {
		if pos, ok := mapPosition(change, refs, n); ok {
			return pos, true
		}
	}
	return position{}, false
}

// multiLinePosition builds a multi-line position spanning start..end when both
// endpoints are in the diff. The line_code component is best-effort (GitLab's
// exact formula is version-sensitive); callers fall back to single-line on a 422.
func multiLinePosition(change MRChange, refs DiffRefs, lr model.LineRange) (position, bool) {
	if lr.End <= lr.Start {
		return position{}, false
	}
	startLoc, ok1 := locateLine(change, lr.Start)
	endLoc, ok2 := locateLine(change, lr.End)
	if !ok1 || !ok2 {
		return position{}, false
	}
	pos := basePosition(change, refs)
	// The anchor (new_line/old_line) points at the last line of the range.
	enl := endLoc.newLine
	pos.NewLine = &enl
	if !endLoc.added {
		eol := endLoc.oldLine
		pos.OldLine = &eol
	}
	pos.LineRange = &lineRange{
		Start: newLinePoint(change.NewPath, startLoc),
		End:   newLinePoint(change.NewPath, endLoc),
	}
	return pos, true
}

func basePosition(change MRChange, refs DiffRefs) position {
	return position{
		PositionType: "text",
		BaseSHA:      refs.BaseSHA,
		HeadSHA:      refs.HeadSHA,
		StartSHA:     refs.StartSHA,
		NewPath:      change.NewPath,
		OldPath:      change.OldPath,
	}
}

func newLinePoint(path string, loc lineLoc) linePoint {
	lp := linePoint{LineCode: lineCode(path, loc)}
	nl := loc.newLine
	lp.NewLine = &nl
	if loc.added {
		lp.Type = "new"
	} else {
		ol := loc.oldLine
		lp.OldLine = &ol
	}
	return lp
}

// lineCode mirrors GitLab's `<sha1(path)>_<old_line>_<new_line>` line identifier.
func lineCode(path string, loc lineLoc) string {
	sum := sha1.Sum([]byte(path))
	return fmt.Sprintf("%x_%d_%d", sum, loc.oldLine, loc.newLine)
}
