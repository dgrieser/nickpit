package git

import (
	"path/filepath"
	"testing"

	"github.com/dgrieser/nickpit/internal/testutil"
)

func TestParseUnifiedDiff(t *testing.T) {
	diff := string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff")))
	hunks, files, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Additions != 3 {
		t.Fatalf("additions = %d", files[0].Additions)
	}
}
