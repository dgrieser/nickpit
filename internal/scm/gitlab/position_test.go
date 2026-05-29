package gitlab

import (
	"crypto/sha1"
	"fmt"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// hunk: lines new1=context, new2=added, new3=context.
//
//	old1 " a"   -> new1
//	     "+b"   -> new2 (added)
//	old2 " c"   -> new3
func sampleChange() MRChange {
	return MRChange{
		NewPath: "main.go",
		OldPath: "main.go",
		Hunks: []model.DiffHunk{{
			FilePath: "main.go",
			OldStart: 1,
			NewStart: 1,
			Content:  " a\n+b\n c\n",
		}},
	}
}

var sampleRefs = DiffRefs{BaseSHA: "base", HeadSHA: "head", StartSHA: "start"}

func TestMapPositionAddedLine(t *testing.T) {
	pos, ok := mapPosition(sampleChange(), sampleRefs, 2)
	if !ok {
		t.Fatal("expected line 2 to map")
	}
	if pos.NewLine == nil || *pos.NewLine != 2 {
		t.Fatalf("new_line = %v, want 2", pos.NewLine)
	}
	if pos.OldLine != nil {
		t.Fatalf("added line must omit old_line, got %v", *pos.OldLine)
	}
	if pos.BaseSHA != "base" || pos.HeadSHA != "head" || pos.StartSHA != "start" {
		t.Fatalf("position SHAs not propagated: %+v", pos)
	}
}

func TestMapPositionContextLine(t *testing.T) {
	pos, ok := mapPosition(sampleChange(), sampleRefs, 3)
	if !ok {
		t.Fatal("expected line 3 to map")
	}
	if pos.NewLine == nil || *pos.NewLine != 3 {
		t.Fatalf("new_line = %v, want 3", pos.NewLine)
	}
	if pos.OldLine == nil || *pos.OldLine != 2 {
		t.Fatalf("context old_line = %v, want 2", pos.OldLine)
	}
}

func TestMapPositionNotInDiff(t *testing.T) {
	if _, ok := mapPosition(sampleChange(), sampleRefs, 99); ok {
		t.Fatal("line 99 is not in the diff and must not map")
	}
}

func TestMapPositionRenamedKeepsBothPaths(t *testing.T) {
	change := sampleChange()
	change.NewPath = "new.go"
	change.OldPath = "old.go"
	change.Renamed = true
	pos, ok := mapPosition(change, sampleRefs, 2)
	if !ok {
		t.Fatal("expected map")
	}
	if pos.NewPath != "new.go" || pos.OldPath != "old.go" {
		t.Fatalf("rename paths = %q/%q, want new.go/old.go", pos.NewPath, pos.OldPath)
	}
}

func TestBestPositionDeletedFileHasNoNewLine(t *testing.T) {
	// All-deletion hunk: no new-side lines exist, so nothing maps.
	change := MRChange{
		NewPath: "gone.go",
		OldPath: "gone.go",
		Deleted: true,
		Hunks: []model.DiffHunk{{
			FilePath: "gone.go",
			OldStart: 1,
			NewStart: 1,
			Content:  "-a\n-b\n",
		}},
	}
	if _, ok := bestPosition(change, sampleRefs, model.LineRange{Start: 1, End: 2}); ok {
		t.Fatal("deleted file must not yield a new-side position")
	}
}

func TestBestPositionPicksFirstMappableInRange(t *testing.T) {
	// Range 90..2: 90 not in diff, falls through to 2 (added).
	pos, ok := bestPosition(sampleChange(), sampleRefs, model.LineRange{Start: 90, End: 2})
	if ok {
		t.Fatalf("range start>end shouldn't iterate backwards; got %+v", pos)
	}
	pos, ok = bestPosition(sampleChange(), sampleRefs, model.LineRange{Start: 2, End: 90})
	if !ok || pos.NewLine == nil || *pos.NewLine != 2 {
		t.Fatalf("expected first mappable line 2, got ok=%v pos=%+v", ok, pos)
	}
}

func TestMultiLinePositionLineCode(t *testing.T) {
	pos, ok := multiLinePosition(sampleChange(), sampleRefs, model.LineRange{Start: 1, End: 3})
	if !ok {
		t.Fatal("expected multi-line position")
	}
	if pos.LineRange == nil {
		t.Fatal("line_range must be set")
	}
	sum := sha1.Sum([]byte("main.go"))
	wantStart := fmt.Sprintf("%x_%d_%d", sum, 1, 1) // line1 context: old1/new1
	wantEnd := fmt.Sprintf("%x_%d_%d", sum, 2, 3)   // line3 context: old2/new3
	if pos.LineRange.Start.LineCode != wantStart {
		t.Fatalf("start line_code = %q, want %q", pos.LineRange.Start.LineCode, wantStart)
	}
	if pos.LineRange.End.LineCode != wantEnd {
		t.Fatalf("end line_code = %q, want %q", pos.LineRange.End.LineCode, wantEnd)
	}
}
