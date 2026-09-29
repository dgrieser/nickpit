package forgejo

import (
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// reviewComment is one inline comment in a Forgejo "create review" request.
// new_position is a new-side file line number (not a diff offset); Forgejo
// has no multi-line comments, so a finding's range anchors to one line.
type reviewComment struct {
	Path        string `json:"path"`
	Body        string `json:"body"`
	NewPosition int    `json:"new_position"`
}

// inlineComment anchors a finding to its diff line on the new side: the first
// line in [Start,End] that is part of the diff. It returns false when no line
// in the range maps, so the caller can fall back to a general PR comment.
func inlineComment(hunks []model.DiffHunk, path string, lr model.LineRange, body string) (reviewComment, bool) {
	if len(hunks) == 0 {
		return reviewComment{}, false
	}
	end := max(lr.End, lr.Start)
	for n := lr.Start; n <= end; n++ {
		if _, ok := reviewmd.LocateLine(hunks, n); ok {
			return reviewComment{Path: path, Body: body, NewPosition: n}, true
		}
	}
	return reviewComment{}, false
}
