package github

import (
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// sideRight is the diff side findings are anchored to: the head/new version of
// the file. GitHub also supports LEFT (the base), but nickpit findings always
// refer to the new code.
const sideRight = "RIGHT"

// reviewComment is one inline comment in a GitHub "create review" request. Line
// numbers are new-side file line numbers (not diff offsets); start_line/start_side
// are emitted only for a multi-line comment.
type reviewComment struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line"`
	Side      string `json:"side"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
}

// inlineComment anchors a finding to its diff line(s) on the new side. When both
// endpoints of the range are part of the diff and start < end it becomes a
// multi-line comment (start_line..line); otherwise the first line in [Start,End]
// that is part of the diff is used as a single-line comment. It returns false
// when no line in the range maps, so the caller can fall back to a general PR
// comment — GitHub's create-review call is atomic, so an out-of-diff line would
// otherwise reject the whole review.
func inlineComment(hunks []model.DiffHunk, path string, lr model.LineRange, body string) (reviewComment, bool) {
	if len(hunks) == 0 {
		return reviewComment{}, false
	}
	if lr.End > lr.Start {
		_, startOK := reviewmd.LocateLine(hunks, lr.Start)
		_, endOK := reviewmd.LocateLine(hunks, lr.End)
		if startOK && endOK {
			return reviewComment{
				Path:      path,
				Body:      body,
				Line:      lr.End,
				Side:      sideRight,
				StartLine: lr.Start,
				StartSide: sideRight,
			}, true
		}
	}
	end := max(lr.End, lr.Start)
	for n := lr.Start; n <= end; n++ {
		if _, ok := reviewmd.LocateLine(hunks, n); ok {
			return reviewComment{Path: path, Body: body, Line: n, Side: sideRight}, true
		}
	}
	return reviewComment{}, false
}
