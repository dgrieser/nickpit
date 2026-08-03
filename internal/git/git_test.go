package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOutputSnippet(t *testing.T) {
	if got := outputSnippet(nil); got != "" {
		t.Fatalf("empty output snippet = %q", got)
	}
	if got := outputSnippet([]byte("  \n")); got != "" {
		t.Fatalf("whitespace-only snippet = %q", got)
	}
	if got := outputSnippet([]byte("fatal: bad revision\n")); got != ": fatal: bad revision" {
		t.Fatalf("snippet = %q", got)
	}
	long := strings.Repeat("x", maxErrorOutputBytes+100) + "tail"
	got := outputSnippet([]byte(long))
	if !strings.HasPrefix(got, ": ...") {
		t.Fatalf("long snippet missing ellipsis prefix: %.20q", got)
	}
	if !strings.HasSuffix(got, "tail") {
		t.Fatalf("long snippet lost the tail: %.20q", got[len(got)-20:])
	}
	if len(got) > maxErrorOutputBytes+len(": ...") {
		t.Fatalf("snippet not truncated: %d bytes", len(got))
	}
	// The truncation cut must land on a rune boundary: place a multi-byte
	// rune exactly across the byte cut and require valid UTF-8 output.
	multibyte := strings.Repeat("ü", maxErrorOutputBytes) + "Zusammenführung"
	trimmed := outputSnippet([]byte(multibyte))
	if !utf8.ValidString(trimmed) {
		t.Fatalf("snippet is not valid UTF-8: %q", trimmed[:12])
	}
	if !strings.HasSuffix(trimmed, "Zusammenführung") {
		t.Fatalf("snippet lost the tail: %q", trimmed[len(trimmed)-20:])
	}
}

func TestExecRunnerErrorIncludesCommandOutput(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	// Force untranslated git messages so the assertion is locale-independent.
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANGUAGE", "C")
	runner := ExecRunner{RepoRoot: t.TempDir()}
	_, err := runner.Run(context.Background(), "rev-parse", "--verify", "definitely-not-a-ref")
	if err == nil {
		t.Fatal("expected error for git command in non-repo")
	}
	// Outside a repository git prints "not a git repository" on stderr; the
	// wrapped error must carry that reason, not just the exit status.
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
		t.Fatalf("error lacks git output: %v", err)
	}
}

// newRealRepo creates a repository with one commit whose patch is far larger
// than the caps RunLimited is asked to enforce.
func newRealRepo(t *testing.T, contentBytes int) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	runner := ExecRunner{RepoRoot: root}
	ctx := context.Background()
	for _, args := range [][]string{
		{"init", "--quiet", "."},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		if _, err := runner.Run(ctx, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	line := strings.Repeat("x", 63) + "\n"
	var content strings.Builder
	for content.Len() < contentBytes {
		content.WriteString(line)
	}
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(ctx, "add", "big.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(ctx, "commit", "--quiet", "-m", "add big file"); err != nil {
		t.Fatal(err)
	}
	return root
}

// RunLimited must stop reading at the cap instead of buffering a whole patch,
// and report that output was cut without turning the killed command into an
// error.
func TestExecRunnerRunLimitedStopsAtTheCap(t *testing.T) {
	root := newRealRepo(t, 1<<20)
	runner := ExecRunner{RepoRoot: root}

	const limit = 4096
	out, truncated, err := runner.RunLimited(context.Background(), limit, "show", "--format=", "--patch", "HEAD")
	if err != nil {
		t.Fatalf("RunLimited: %v", err)
	}
	if !truncated {
		t.Fatal("truncated = false for a patch far larger than the cap")
	}
	if len(out) != limit {
		t.Fatalf("read %d bytes, want exactly %d", len(out), limit)
	}

	// A cap the output fits under returns everything and reports no truncation.
	full, truncated, err := runner.RunLimited(context.Background(), 1<<24, "show", "--format=", "--patch", "HEAD")
	if err != nil {
		t.Fatalf("RunLimited: %v", err)
	}
	if truncated {
		t.Fatal("truncated = true for output below the cap")
	}
	if len(full) <= limit {
		t.Fatalf("full patch = %d bytes, expected the large diff", len(full))
	}
	if !strings.HasPrefix(full, string(out)) {
		t.Fatal("capped read is not a prefix of the full output")
	}

	// A limit of zero means unlimited, matching Run.
	unlimited, truncated, err := runner.RunLimited(context.Background(), 0, "show", "--format=", "--patch", "HEAD")
	if err != nil || truncated || unlimited != full {
		t.Fatalf("unlimited read: truncated = %t, err = %v, equal = %t", truncated, err, unlimited == full)
	}
}

func TestExecRunnerRunLimitedReportsFailures(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANGUAGE", "C")
	runner := ExecRunner{RepoRoot: t.TempDir()}

	out, truncated, err := runner.RunLimited(context.Background(), 4096, "rev-parse", "--verify", "definitely-not-a-ref")
	if err == nil {
		t.Fatal("expected error for git command in non-repo")
	}
	if out != "" || truncated {
		t.Fatalf("failed command returned out = %q, truncated = %t", out, truncated)
	}
	// stderr is captured separately from stdout here, so the reason must still
	// reach the error.
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
		t.Fatalf("error lacks git output: %v", err)
	}
}

func TestTopLevelResolvesTheWorkingTreeRoot(t *testing.T) {
	root := newRealRepo(t, 1024)
	sub := filepath.Join(root, "internal", "config")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := TopLevel(context.Background(), sub)
	if !ok {
		t.Fatal("TopLevel reported no working tree for a subdirectory of a repository")
	}
	// macOS temp dirs are symlinked, and git reports the resolved path.
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gotRoot, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("TopLevel = %q, want %q", gotRoot, wantRoot)
	}

	if _, ok := TopLevel(context.Background(), t.TempDir()); ok {
		t.Fatal("TopLevel reported a working tree outside any repository")
	}
}
