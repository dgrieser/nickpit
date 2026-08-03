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

func TestInlineCommentCrossHunkRangeDegradesToSingleLine(t *testing.T) {
	// Two hunks of the same file: lines 1-3 and lines 40-42 (new side). A range
	// whose endpoints map but sit in different hunks must NOT become a
	// multi-line comment — GitHub 422s start_line/line pairs spanning hunks and
	// the atomic create-review POST would drop every inline comment.
	hunks := []model.DiffHunk{
		{FilePath: "main.go", OldStart: 1, NewStart: 1, Content: " a\n+b\n+c\n"},
		{FilePath: "main.go", OldStart: 39, NewStart: 40, Content: " x\n+y\n z\n"},
	}
	c, ok := inlineComment(hunks, "main.go", model.LineRange{Start: 2, End: 41}, "body")
	if !ok {
		t.Fatal("cross-hunk range should still anchor a single-line comment")
	}
	if c.StartLine != 0 || c.StartSide != "" {
		t.Fatalf("cross-hunk range must not emit multi-line comment: %+v", c)
	}
	if c.Line != 2 || c.Side != sideRight {
		t.Fatalf("expected single-line comment at first mappable line 2, got %+v", c)
	}
	// Sanity: the same endpoints inside one hunk still yield multi-line.
	c, ok = inlineComment(hunks, "main.go", model.LineRange{Start: 40, End: 42}, "body")
	if !ok || c.StartLine != 40 || c.Line != 42 {
		t.Fatalf("same-hunk range should stay multi-line: %+v ok=%v", c, ok)
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
