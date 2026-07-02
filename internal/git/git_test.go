package git

import (
	"context"
	"os/exec"
	"strings"
	"testing"
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
