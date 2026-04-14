package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
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

type stubGitRunner struct {
	outputs map[string]string
	calls   [][]string
}

func (r *stubGitRunner) Run(_ context.Context, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	return r.outputs[joinArgs(args)], nil
}

func joinArgs(args []string) string {
	return stringJoin(args, "\x00")
}

func stringJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += sep + part
	}
	return out
}

func TestLocalSourceResolveContextDefaultsBranchBaseFromOriginHEAD(t *testing.T) {
	runner := &stubGitRunner{
		outputs: map[string]string{
			joinArgs([]string{"symbolic-ref", "--short", "refs/remotes/origin/HEAD"}): "origin/main\n",
			joinArgs([]string{"diff", "main...HEAD"}):                                 string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff"))),
		},
	}
	source := &LocalSource{
		repoRoot: t.TempDir(),
		git:      runner,
	}

	ctx, err := source.ResolveContext(context.Background(), model.ReviewRequest{
		Mode:    model.ModeLocal,
		Submode: "branch",
		HeadRef: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}

	if ctx.Repository.BaseRef != "main" {
		t.Fatalf("base ref = %q", ctx.Repository.BaseRef)
	}
	if len(runner.calls) < 2 {
		t.Fatalf("calls = %d", len(runner.calls))
	}
	if got := runner.calls[0]; len(got) != 3 || got[0] != "symbolic-ref" {
		t.Fatalf("symbolic-ref args = %#v", got)
	}
	if got := runner.calls[1]; len(got) != 2 || got[0] != "diff" || got[1] != "main...HEAD" {
		t.Fatalf("diff args = %#v", got)
	}
}
