package forgejo

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// new-side: line1=context " a", line2=added "+b", line3=added "+c".
func sampleHunks() []model.DiffHunk {
	return []model.DiffHunk{{FilePath: "main.go", OldStart: 1, NewStart: 1, Content: " a\n+b\n+c\n"}}
}

func TestInlineCommentSingleLine(t *testing.T) {
	c, ok := inlineComment(sampleHunks(), "main.go", model.LineRange{Start: 2, End: 2}, "body")
	if !ok {
		t.Fatal("line 2 should map")
	}
	if c.Path != "main.go" || c.NewPosition != 2 || c.Body != "body" {
		t.Fatalf("single-line comment = %+v", c)
	}
}

// Forgejo has no multi-line comments: a range anchors to its first line that
// is part of the diff.
func TestInlineCommentRangeUsesFirstMappable(t *testing.T) {
	c, ok := inlineComment(sampleHunks(), "main.go", model.LineRange{Start: 2, End: 3}, "body")
	if !ok || c.NewPosition != 2 {
		t.Fatalf("range comment = %+v ok=%v, want line 2", c, ok)
	}
	c, ok = inlineComment(sampleHunks(), "main.go", model.LineRange{Start: 2, End: 99}, "body")
	if !ok || c.NewPosition != 2 {
		t.Fatalf("partial range comment = %+v ok=%v, want line 2", c, ok)
	}
}

func TestInlineCommentNotInDiff(t *testing.T) {
	if _, ok := inlineComment(sampleHunks(), "main.go", model.LineRange{Start: 50, End: 60}, "body"); ok {
		t.Fatal("lines outside the diff must not map")
	}
	if _, ok := inlineComment(nil, "main.go", model.LineRange{Start: 1, End: 1}, "body"); ok {
		t.Fatal("absent file (no hunks) must not map")
	}
}
