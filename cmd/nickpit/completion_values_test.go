package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootEnumCompletions(t *testing.T) {
	root := newRootCmd()
	tests := []struct {
		flag   string
		prefix string
		want   []string
	}{
		{flag: "output", prefix: "r", want: []string{"raw"}},
		{flag: "diff-format", prefix: "git-", want: []string{"git-json"}},
		{flag: "reasoning-effort", prefix: "m", want: []string{"max", "medium", "minimal"}},
		{flag: "verify-drop-policy", prefix: "refuted-", want: []string{"refuted-only", "refuted-and-unverified"}},
		{flag: "priority-threshold", prefix: "2", want: []string{"2"}},
		{flag: "disable-styleguide", prefix: "sec", want: nil},
		{flag: "step", prefix: "review:sec", want: []string{"review:security"}},
	}
	for _, tc := range tests {
		t.Run(tc.flag, func(t *testing.T) {
			fn, ok := root.GetFlagCompletionFunc(tc.flag)
			if !ok {
				t.Fatalf("no completion registered for --%s", tc.flag)
			}
			got, directive := fn(root, nil, tc.prefix)
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Fatalf("directive = %v", directive)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("completion = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProfileCompletionsIncludeConfigAndBuiltins(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".nickpit.yaml")
	if err := os.WriteFile(configPath, []byte("profiles:\n  team:\n    model: test\n  local: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := (&app{workDir: dir, configPath: ".nickpit.yaml"}).profileCompletions()
	for _, want := range []string{"default", "mittwald", "team", "local"} {
		if !slices.Contains(got, want) {
			t.Fatalf("profiles %v missing %q", got, want)
		}
	}
	if !slices.IsSorted(got) {
		t.Fatalf("profiles not sorted: %v", got)
	}
}

func TestRepoPathCompletions(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"internal/review", "internal/session", ".git"} {
		if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "review.go"), []byte("package internal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &app{workDir: dir}
	if got := a.repoPathCompletions("", false); !slices.Equal(got, []string{"internal/"}) {
		t.Fatalf("root paths = %v", got)
	}
	if got := a.repoPathCompletions("internal/", false); !slices.Equal(got, []string{"internal/review.go", "internal/review/", "internal/session/"}) {
		t.Fatalf("internal paths = %v", got)
	}
	if got := a.repoPathCompletions("internal/re", true); !slices.Equal(got, []string{"internal/review/"}) {
		t.Fatalf("directory paths = %v", got)
	}
	if got := a.repoPathCompletions("../", false); len(got) != 0 {
		t.Fatalf("parent traversal returned %v", got)
	}
}

func TestGitRefCompletions(t *testing.T) {
	dir := t.TempDir()
	runGitTestCommand(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitTestCommand(t, dir, "add", "file.txt")
	runGitTestCommand(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "first commit")
	runGitTestCommand(t, dir, "branch", "feature")
	runGitTestCommand(t, dir, "tag", "v1")

	a := &app{workDir: dir}
	refs := a.gitRefCompletions(false)
	for _, want := range []string{"HEAD", "feature", "v1"} {
		if !slices.Contains(refs, want) {
			t.Fatalf("refs %v missing %q", refs, want)
		}
	}
	withCommits := a.gitRefCompletions(true)
	foundCommit := false
	for _, candidate := range withCommits {
		if strings.Contains(candidate, "\tfirst commit") {
			foundCommit = true
			break
		}
	}
	if !foundCommit {
		t.Fatalf("commit completion missing from %v", withCommits)
	}
}

func runGitTestCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
