package github

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
	if c.Path != "main.go" || c.Line != 2 || c.Side != sideRight {
		t.Fatalf("single-line comment = %+v", c)
	}
	if c.StartLine != 0 || c.StartSide != "" {
		t.Fatalf("single-line comment must omit start_line/start_side: %+v", c)
	}
}

func TestInlineCommentMultiLine(t *testing.T) {
	c, ok := inlineComment(sampleHunks(), "main.go", model.LineRange{Start: 2, End: 3}, "body")
	if !ok {
		t.Fatal("range 2..3 should map")
	}
	if c.StartLine != 2 || c.StartSide != sideRight || c.Line != 3 || c.Side != sideRight {
		t.Fatalf("multi-line comment = %+v, want start 2 / end 3 on RIGHT", c)
	}
}

func TestInlineCommentPartialRangeUsesFirstMappable(t *testing.T) {
	// End is outside the diff: no multi-line; the first mappable line (2) is
	// used as a single-line comment.
	c, ok := inlineComment(sampleHunks(), "main.go", model.LineRange{Start: 2, End: 99}, "body")
	if !ok {
		t.Fatal("expected first mappable line in range")
	}
	if c.Line != 2 || c.StartLine != 0 {
		t.Fatalf("expected single-line at 2, got %+v", c)
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
