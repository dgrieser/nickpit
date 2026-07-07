package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if hunks[0].Language != "go" {
		t.Fatalf("hunk language = %q", hunks[0].Language)
	}
	if files[0].Additions != 3 {
		t.Fatalf("additions = %d", files[0].Additions)
	}
}

func TestParseUnifiedDiffFormatsBuildsDiffFiles(t *testing.T) {
	diff := string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff")))
	diffFiles, hunks, files, err := ParseUnifiedDiffFormats(diff)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffFiles) != 1 {
		t.Fatalf("diff files = %d, want 1", len(diffFiles))
	}
	if diffFiles[0].FilePath != "main.go" {
		t.Fatalf("diff file path = %q", diffFiles[0].FilePath)
	}
	if diffFiles[0].Language != "go" {
		t.Fatalf("diff file language = %q", diffFiles[0].Language)
	}
	if !strings.HasPrefix(diffFiles[0].Content, "diff --git a/main.go b/main.go\n") {
		t.Fatalf("diff file content missing git header: %.80q", diffFiles[0].Content)
	}
	if !strings.Contains(diffFiles[0].Content, "@@") {
		t.Fatalf("diff file content missing hunk header: %.80q", diffFiles[0].Content)
	}
	if len(hunks) != 1 || len(files) != 1 {
		t.Fatalf("hunks/files = %d/%d, want 1/1", len(hunks), len(files))
	}
}

type stubGitRunner struct {
	outputs map[string]string
	errors  map[string]error
	calls   [][]string
}

func (r *stubGitRunner) Run(_ context.Context, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	key := joinArgs(args)
	if err := r.errors[key]; err != nil {
		return "", err
	}
	return r.outputs[key], nil
}

func joinArgs(args []string) string {
	return stringJoin(args, "\x00")
}

func stringJoin(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

func TestLocalSourceDiffForLocalChangeSubmodes(t *testing.T) {
	diff := string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff")))
	tests := []struct {
		name    string
		submode string
		args    []string
	}{
		{
			name:    "uncommitted",
			submode: "uncommitted",
			args:    []string{"diff", "HEAD"},
		},
		{
			name:    "staged",
			submode: "staged",
			args:    []string{"diff", "--cached"},
		},
		{
			name:    "unstaged",
			submode: "unstaged",
			args:    []string{"diff"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &stubGitRunner{
				outputs: map[string]string{
					joinArgs(tt.args): diff,
				},
			}
			source := &LocalSource{
				repoRoot: t.TempDir(),
				git:      runner,
			}

			got, err := source.diffForRequest(context.Background(), model.ReviewRequest{Submode: tt.submode})
			if err != nil {
				t.Fatal(err)
			}
			if got != diff {
				t.Fatalf("diff output = %.80q", got)
			}
			if len(runner.calls) != 1 {
				t.Fatalf("calls = %d, want 1", len(runner.calls))
			}
			if gotArgs := joinArgs(runner.calls[0]); gotArgs != joinArgs(tt.args) {
				t.Fatalf("diff args = %#v, want %#v", runner.calls[0], tt.args)
			}
		})
	}
}

func TestLocalSourceResolveContextDefaultsLocalChangeRefs(t *testing.T) {
	diff := string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff")))
	tests := []struct {
		name    string
		submode string
		args    []string
	}{
		{
			name:    "uncommitted",
			submode: "uncommitted",
			args:    []string{"diff", "HEAD"},
		},
		{
			name:    "staged",
			submode: "staged",
			args:    []string{"diff", "--cached"},
		},
		{
			name:    "unstaged",
			submode: "unstaged",
			args:    []string{"diff"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &stubGitRunner{
				outputs: map[string]string{
					joinArgs([]string{"symbolic-ref", "--short", "HEAD"}): "feature/local-modes\n",
					joinArgs(tt.args): diff,
				},
			}
			source := &LocalSource{
				repoRoot: t.TempDir(),
				git:      runner,
			}

			ctx, err := source.ResolveContext(context.Background(), model.ReviewRequest{
				Mode:    model.ModeLocal,
				Submode: tt.submode,
			})
			if err != nil {
				t.Fatal(err)
			}
			if ctx.Repository.BaseRef != "feature/local-modes" {
				t.Fatalf("base ref = %q", ctx.Repository.BaseRef)
			}
			if ctx.Repository.HeadRef != tt.submode {
				t.Fatalf("head ref = %q", ctx.Repository.HeadRef)
			}
			if gotArgs := joinArgs(runner.calls[1]); gotArgs != joinArgs(tt.args) {
				t.Fatalf("diff args = %#v, want %#v", runner.calls[1], tt.args)
			}
		})
	}
}

func TestLocalSourceResolveContextDefaultsBranchBaseFromOriginHEAD(t *testing.T) {
	runner := &stubGitRunner{
		outputs: map[string]string{
			joinArgs([]string{"symbolic-ref", "--short", "refs/remotes/origin/HEAD"}): "origin/main\n",
			joinArgs([]string{"symbolic-ref", "--short", "HEAD"}):                     "fix/memleak\n",
			joinArgs([]string{"diff", "origin/main...fix/memleak"}):                   string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff"))),
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

	if ctx.Repository.BaseRef != "origin/main" {
		t.Fatalf("base ref = %q", ctx.Repository.BaseRef)
	}
	if ctx.Repository.HeadRef != "fix/memleak" {
		t.Fatalf("head ref = %q", ctx.Repository.HeadRef)
	}
	if len(ctx.DiffFiles) != 1 || ctx.DiffFiles[0].FilePath != "main.go" {
		t.Fatalf("diff files = %#v", ctx.DiffFiles)
	}
	if len(runner.calls) < 3 {
		t.Fatalf("calls = %d", len(runner.calls))
	}
	if got := runner.calls[0]; len(got) != 3 || got[0] != "symbolic-ref" {
		t.Fatalf("symbolic-ref args = %#v", got)
	}
	if got := runner.calls[1]; len(got) != 3 || got[0] != "symbolic-ref" || got[2] != "HEAD" {
		t.Fatalf("head symbolic-ref args = %#v", got)
	}
	if got := runner.calls[2]; len(got) != 2 || got[0] != "diff" || got[1] != "origin/main...fix/memleak" {
		t.Fatalf("diff args = %#v", got)
	}
}

func TestLocalSourceResolveContextPrefersOriginForExplicitBranchBase(t *testing.T) {
	runner := &stubGitRunner{
		outputs: map[string]string{
			joinArgs([]string{"show-ref", "--verify", "--quiet", "refs/remotes/origin/main"}): "",
			joinArgs([]string{"symbolic-ref", "--short", "HEAD"}):                             "fix/memleak\n",
			joinArgs([]string{"diff", "origin/main...fix/memleak"}):                           string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff"))),
		},
	}
	source := &LocalSource{
		repoRoot: t.TempDir(),
		git:      runner,
	}

	ctx, err := source.ResolveContext(context.Background(), model.ReviewRequest{
		Mode:    model.ModeLocal,
		Submode: "branch",
		BaseRef: "main",
		HeadRef: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}

	if ctx.Repository.BaseRef != "origin/main" {
		t.Fatalf("base ref = %q", ctx.Repository.BaseRef)
	}
	if got := runner.calls[0]; len(got) != 4 || got[0] != "show-ref" || got[3] != "refs/remotes/origin/main" {
		t.Fatalf("show-ref args = %#v", got)
	}
	if got := runner.calls[2]; len(got) != 2 || got[0] != "diff" || got[1] != "origin/main...fix/memleak" {
		t.Fatalf("diff args = %#v", got)
	}
}

func TestLocalSourceResolveContextKeepsExplicitBranchBaseWhenOriginMissing(t *testing.T) {
	runner := &stubGitRunner{
		outputs: map[string]string{
			joinArgs([]string{"symbolic-ref", "--short", "HEAD"}): "fix/memleak\n",
			joinArgs([]string{"diff", "main...fix/memleak"}):      string(testutil.LoadFixture(t, filepath.Join("..", "..", "testdata", "diffs", "simple_add.diff"))),
		},
		errors: map[string]error{
			joinArgs([]string{"show-ref", "--verify", "--quiet", "refs/remotes/origin/main"}): errors.New("missing ref"),
		},
	}
	source := &LocalSource{
		repoRoot: t.TempDir(),
		git:      runner,
	}

	ctx, err := source.ResolveContext(context.Background(), model.ReviewRequest{
		Mode:    model.ModeLocal,
		Submode: "branch",
		BaseRef: "main",
		HeadRef: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}

	if ctx.Repository.BaseRef != "main" {
		t.Fatalf("base ref = %q", ctx.Repository.BaseRef)
	}
	if got := runner.calls[2]; len(got) != 2 || got[0] != "diff" || got[1] != "main...fix/memleak" {
		t.Fatalf("diff args = %#v", got)
	}
}

func TestParseCommitsParsesAuthorDate(t *testing.T) {
	out := "abc123\x1fAlice\x1f2026-01-02T03:04:05+01:00\x1ffix: something\n" +
		"def456\x1fBob\x1fnot-a-date\x1fchore: other\n"
	commits := parseCommits(out)
	if len(commits) != 2 {
		t.Fatalf("commits = %d, want 2", len(commits))
	}
	first := commits[0]
	if first.SHA != "abc123" || first.Author != "Alice" || first.Message != "fix: something" {
		t.Fatalf("first commit = %#v", first)
	}
	if first.Date.IsZero() {
		t.Fatal("author date not parsed")
	}
	if got := first.Date.Format(time.RFC3339); got != "2026-01-02T03:04:05+01:00" {
		t.Fatalf("date = %q", got)
	}
	// An unparsable date degrades to the zero value instead of dropping the commit.
	if !commits[1].Date.IsZero() {
		t.Fatalf("invalid date parsed to %v", commits[1].Date)
	}
}
