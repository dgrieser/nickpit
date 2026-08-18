package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSymlinkPathsReadsIndexModes(t *testing.T) {
	runner := &stubGitRunner{
		match: func(args []string) (string, bool) {
			if args[0] != "ls-files" {
				return "", false
			}
			return strings.Join([]string{
				"120000 32f64f4d836716819dc5fa9a1e09a29b428881df 0\tdeploy/chart/templates\x00",
				"100644 45b983be36b73c0788dc9cbcb76cbb80fc7bb057 0\tmain.go\x00",
				"100755 45b983be36b73c0788dc9cbcb76cbb80fc7bb057 0\tscripts/run.sh\x00",
			}, ""), true
		},
	}

	marks, err := SymlinkPaths(context.Background(), runner, []string{"deploy/chart/templates", "main.go", "scripts/run.sh"})
	if err != nil {
		t.Fatal(err)
	}
	if !marks["deploy/chart/templates"] {
		t.Fatalf("symlink not marked: %#v", marks)
	}
	if marks["main.go"] || marks["scripts/run.sh"] {
		t.Fatalf("regular files marked as symlinks: %#v", marks)
	}
}

// A large change set must not overflow the command line, and one failing chunk
// must not discard the marks the other chunks produced.
func TestSymlinkPathsChunksAndSurvivesFailures(t *testing.T) {
	paths := make([]string, 0, maxLsFilesPaths+1)
	for i := range maxLsFilesPaths {
		paths = append(paths, "file"+strconv.Itoa(i))
	}
	paths = append(paths, "link")
	runner := &stubGitRunner{}
	runner.matchErr = func(args []string) error {
		if args[0] == "ls-files" && len(runner.calls) == 1 {
			return errors.New("ls-files failed")
		}
		return nil
	}
	runner.match = func(args []string) (string, bool) {
		if args[0] != "ls-files" {
			return "", false
		}
		return "120000 32f64f4 0\tlink\x00", true
	}

	marks, err := SymlinkPaths(context.Background(), runner, paths)
	if err == nil {
		t.Fatal("expected the failing chunk to be reported")
	}
	if len(runner.calls) != 2 {
		t.Fatalf("ls-files calls = %d, want one per chunk", len(runner.calls))
	}
	// "ls-files --stage -z --" plus the chunk's pathspecs.
	if got := len(runner.calls[0]) - 4; got != maxLsFilesPaths {
		t.Fatalf("first chunk carried %d paths, want %d", got, maxLsFilesPaths)
	}
	if got := len(runner.calls[1]) - 4; got != 1 {
		t.Fatalf("second chunk carried %d paths, want 1", got)
	}
	if !marks["link"] {
		t.Fatalf("marks from the surviving chunk were dropped: %#v", marks)
	}
}

func TestSymlinkPathsWithoutRunnerOrPaths(t *testing.T) {
	marks, err := SymlinkPaths(context.Background(), nil, []string{"link"})
	if err != nil || marks != nil {
		t.Fatalf("marks = %#v, err = %v, want no marks without a runner", marks, err)
	}
	marks, err = SymlinkPaths(context.Background(), &stubGitRunner{}, nil)
	if err != nil || marks != nil {
		t.Fatalf("marks = %#v, err = %v, want no marks without paths", marks, err)
	}
}

// With core.symlinks=false — the default on Windows and a valid setting on Unix
// — git materializes a mode-120000 blob as a regular file holding the target
// path. The index still records the symlink, which is why it and not the
// worktree inode is queried.
func TestSymlinkPathsSeesSymlinkMaterializedAsRegularFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	checkout := filepath.Join(root, "checkout")
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(source, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(root, "init", "-q", source)
	if err := os.Symlink("target", filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(source, "add", "-A")
	runGit(source, "-c", "commit.gpgsign=false", "commit", "-qm", "init")
	runGit(root, "clone", "-q", "-c", "core.symlinks=false", source, checkout)

	info, err := os.Lstat(filepath.Join(checkout, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Skip("this git honors symlinks despite core.symlinks=false, so the worktree is not degraded here")
	}

	marks, err := SymlinkPaths(context.Background(), ExecRunner{RepoRoot: checkout}, []string{"link", "main.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !marks["link"] {
		t.Fatalf("index symlink missed while the worktree holds a regular file: %#v", marks)
	}
	if marks["main.go"] {
		t.Fatalf("regular file marked as symlink: %#v", marks)
	}
}
