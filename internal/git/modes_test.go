package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestSymlinkPathsAtRevReadsTreeModes(t *testing.T) {
	runner := &stubGitRunner{
		match: func(args []string) (string, bool) {
			if args[0] != "ls-tree" {
				return "", false
			}
			return strings.Join([]string{
				"120000 blob 32f64f4d836716819dc5fa9a1e09a29b428881df\tdeploy/chart/templates\x00",
				"100644 blob 45b983be36b73c0788dc9cbcb76cbb80fc7bb057\tmain.go\x00",
				"100755 blob 45b983be36b73c0788dc9cbcb76cbb80fc7bb057\tscripts/run.sh\x00",
			}, ""), true
		},
	}

	marks, err := SymlinkPathsAtRev(context.Background(), runner, "head111", []string{"deploy/chart/templates", "main.go", "scripts/run.sh"})
	if err != nil {
		t.Fatal(err)
	}
	if marks["deploy/chart/templates"] != "32f64f4d836716819dc5fa9a1e09a29b428881df" {
		t.Fatalf("symlink not marked with its blob: %#v", marks)
	}
	if marks["main.go"] != "" || marks["scripts/run.sh"] != "" {
		t.Fatalf("regular files marked as symlinks: %#v", marks)
	}
	// The tree is addressed by SHA, not by whatever the checkout holds.
	if !slices.Contains(runner.calls[0], "head111") {
		t.Fatalf("ls-tree did not target the reviewed revision: %v", runner.calls[0])
	}
}

// The paths come from SCM payloads, where a filename may itself look like
// pathspec magic. "--" stops option parsing but not magic, so each path has to be
// passed literally: ":(literal)link" must not be reparsed, and ":!foo" must not
// become an exclusion that enumerates the rest of the tree.
func TestSymlinkPathsAtRevPassesLiteralPathspecs(t *testing.T) {
	runner := &stubGitRunner{}
	if _, err := SymlinkPathsAtRev(context.Background(), runner, "head111", []string{":(literal)link", ":!foo", "plain.go"}); err != nil {
		t.Fatal(err)
	}
	want := []string{":(literal):(literal)link", ":(literal):!foo", ":(literal)plain.go"}
	args := runner.calls[0]
	sep := slices.Index(args, "--")
	if sep < 0 {
		t.Fatalf("no pathspec separator in %v", args)
	}
	if got := args[sep+1:]; !slices.Equal(got, want) {
		t.Fatalf("pathspecs = %v, want %v", got, want)
	}
}

// A large change set must not overflow the command line, and one failing chunk
// must not discard the marks the other chunks produced.
func TestSymlinkPathsAtRevChunksAndSurvivesFailures(t *testing.T) {
	paths := make([]string, 0, maxTreeQueryPaths+1)
	for i := range maxTreeQueryPaths {
		paths = append(paths, "file"+strconv.Itoa(i))
	}
	paths = append(paths, "link")
	runner := &stubGitRunner{}
	runner.matchErr = func(args []string) error {
		if args[0] == "ls-tree" && len(runner.calls) == 1 {
			return errors.New("ls-tree failed")
		}
		return nil
	}
	runner.match = func(args []string) (string, bool) {
		if args[0] != "ls-tree" {
			return "", false
		}
		return "120000 blob 32f64f4\tlink\x00", true
	}

	marks, err := SymlinkPathsAtRev(context.Background(), runner, "head111", paths)
	if err == nil {
		t.Fatal("expected the failing chunk to be reported")
	}
	if len(runner.calls) != 2 {
		t.Fatalf("ls-tree calls = %d, want one per chunk", len(runner.calls))
	}
	// "ls-tree -z <rev> --" plus the chunk's pathspecs.
	if got := len(runner.calls[0]) - 4; got != maxTreeQueryPaths {
		t.Fatalf("first chunk carried %d paths, want %d", got, maxTreeQueryPaths)
	}
	if got := len(runner.calls[1]) - 4; got != 1 {
		t.Fatalf("second chunk carried %d paths, want 1", got)
	}
	if marks["link"] == "" {
		t.Fatalf("marks from the surviving chunk were dropped: %#v", marks)
	}
}

func TestSymlinkPathsAtRevWithoutRunnerRevOrPaths(t *testing.T) {
	marks, err := SymlinkPathsAtRev(context.Background(), nil, "head111", []string{"link"})
	if err != nil || marks != nil {
		t.Fatalf("marks = %#v, err = %v, want no marks without a runner", marks, err)
	}
	runner := &stubGitRunner{}
	marks, err = SymlinkPathsAtRev(context.Background(), runner, "", []string{"link"})
	if err != nil || marks != nil {
		t.Fatalf("marks = %#v, err = %v, want no marks without a revision", marks, err)
	}
	marks, err = SymlinkPathsAtRev(context.Background(), runner, "head111", nil)
	if err != nil || marks != nil {
		t.Fatalf("marks = %#v, err = %v, want no marks without paths", marks, err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("git ran anyway: %v", runner.calls)
	}
}

func TestParseRawFileModes(t *testing.T) {
	out := strings.Join([]string{
		":000000 120000 0000000 32f64f4 A\x00link\x00",
		":100644 100644 45b983b c93cae4 M\x00main.go\x00",
		":120000 120000 78bc337 78bc337 R100\x00dir/sub/old\x00dir/new\x00",
		":120000 000000 32f64f4 0000000 D\x00gone\x00",
		":120000 100644 32f64f4 c93cae4 T\x00swapped\x00",
	}, "")

	modes := ParseRawFileModes(out)

	if !modes.Symlink("link") {
		t.Fatalf("added symlink missed: %#v", modes)
	}
	if modes.Symlink("main.go") {
		t.Fatalf("regular file marked: %#v", modes)
	}
	// A rename is keyed by its destination, which is the path the patch shows.
	if !modes.Symlink("dir/new") || modes.Symlink("dir/sub/old") {
		t.Fatalf("renamed symlink keyed wrong: %#v", modes)
	}
	// A deletion has an all-zero destination mode, so the source side describes
	// what was there.
	if !modes.Symlink("gone") {
		t.Fatalf("deleted symlink missed: %#v", modes)
	}
	// A symlink replaced by a regular file is text on the reviewed side.
	if modes.Symlink("swapped") {
		t.Fatalf("replaced symlink still marked: %#v", modes)
	}
	if modes.Symlink("never-seen") {
		t.Fatalf("unknown path marked: %#v", modes)
	}
	if FileModes(nil).Symlink("link") {
		t.Fatal("nil modes marked a path")
	}
}

// With core.symlinks=false — the default on Windows and a valid setting on Unix
// — git materializes a mode-120000 blob as a regular file holding the target
// path. The committed tree still records the symlink, which is why it and not the
// worktree inode is queried.
func TestSymlinkPathsAtRevSeesSymlinkMaterializedAsRegularFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	checkout := filepath.Join(root, "checkout")
	runGit := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
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
	head := runGit(source, "rev-parse", "HEAD")
	runGit(root, "clone", "-q", "-c", "core.symlinks=false", source, checkout)

	info, err := os.Lstat(filepath.Join(checkout, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Skip("this git honors symlinks despite core.symlinks=false, so the worktree is not degraded here")
	}

	marks, err := SymlinkPathsAtRev(context.Background(), ExecRunner{RepoRoot: checkout}, head, []string{"link", "main.go"})
	if err != nil {
		t.Fatal(err)
	}
	if marks["link"] == "" {
		t.Fatalf("tree symlink missed while the worktree holds a regular file: %#v", marks)
	}
	if marks["main.go"] != "" {
		t.Fatalf("regular file marked as symlink: %#v", marks)
	}

	// A revision the checkout does not have must fail the lookup rather than
	// fall back to whatever is on disk.
	if marks, err := SymlinkPathsAtRev(context.Background(), ExecRunner{RepoRoot: checkout}, "0000000000000000000000000000000000000000", []string{"link"}); err == nil || len(marks) != 0 {
		t.Fatalf("unknown revision produced marks = %#v, err = %v", marks, err)
	}
}

// A combined merge deletion has no destination mode and one source column per
// parent. If only the first parent were inspected, a merge whose symlink sits in a
// later parent would report no symlink — and when the size limit drops the patch,
// this metadata is the only symlink signal left.
func TestSymlinkModeFromRawEntryChecksEveryParent(t *testing.T) {
	tests := []struct {
		name string
		meta string
		want bool
	}{
		{name: "two-way add", meta: ":000000 120000 0000000 32f64f4 A", want: true},
		{name: "two-way delete", meta: ":120000 000000 32f64f4 0000000 D", want: true},
		{name: "combined delete, symlink in first parent", meta: "::120000 100644 000000 1de5659 2cc7521 0000000 DD", want: true},
		{name: "combined delete, symlink in later parent", meta: "::100644 120000 000000 2cc7521 1de5659 0000000 DD", want: true},
		{name: "combined delete of regular parents", meta: "::100644 100644 000000 2cc7521 45b983b 0000000 DD"},
		{name: "combined merge into a regular file", meta: "::120000 100644 100644 1de5659 2cc7521 45b983b MM"},
		{name: "combined merge into a symlink", meta: "::100644 100644 120000 2cc7521 45b983b 1de5659 MM", want: true},
		{name: "not a raw entry", meta: "1\t0\tmain.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parents, dst, _, ok := RawEntryModes(tc.meta)
			if !ok {
				if tc.want {
					t.Fatalf("RawEntryModes(%q) refused a real entry", tc.meta)
				}
				return
			}
			if got := SymlinkModeFromRawEntry(parents, dst) == SymlinkFileMode; got != tc.want {
				t.Fatalf("symlink for %q = %v, want %v", tc.meta, got, tc.want)
			}
		})
	}
}

// The same rule has to reach the mode map the local diff fallback uses.
func TestParseRawFileModesUsesEveryParentOfACombinedDeletion(t *testing.T) {
	modes := ParseRawFileModes("::100644 120000 000000 2cc7521 1de5659 0000000 DD\x00f\x00")
	if !modes.Symlink("f") {
		t.Fatalf("combined deletion with a symlink parent unmarked: %#v", modes)
	}
}

// The blob name rides along with the mode because a pure rename shows no content:
// reading that blob is the only way to recover the link target.
func TestParseRawFileModesKeepsBlobNames(t *testing.T) {
	out := strings.Join([]string{
		":120000 120000 78bc337 78bc337 R100\x00dir/sub/old\x00dir/new\x00",
		":120000 000000 32f64f4 0000000 D\x00gone\x00",
		":000000 120000 0000000 1de5659 A\x00added\x00",
	}, "")

	modes := ParseRawFileModes(out)

	if got := modes.Blob("dir/new"); got != "78bc337" {
		t.Fatalf("renamed blob = %q, want the destination object", got)
	}
	if got := modes.Blob("added"); got != "1de5659" {
		t.Fatalf("added blob = %q", got)
	}
	// A deletion has no destination blob, and an unknown path has no entry.
	if got := modes.Blob("gone"); got != "" {
		t.Fatalf("deleted blob = %q, want empty", got)
	}
	if got := modes.Blob("never-seen"); got != "" {
		t.Fatalf("unknown blob = %q, want empty", got)
	}
}

// A symlink's blob IS its target: git appends no separator, and POSIX permits a
// newline in a pathname, so nothing may be trimmed off a blob read.
func TestReadBlobKeepsBytesVerbatim(t *testing.T) {
	runner := &stubGitRunner{
		outputs: map[string]string{
			joinArgs([]string{"cat-file", "blob", "32f64f4"}): "../weird target\n",
		},
	}

	target, err := ReadBlob(context.Background(), runner, "32f64f4", 4096)
	if err != nil {
		t.Fatal(err)
	}
	if target != "../weird target\n" {
		t.Fatalf("target = %q, want the blob byte for byte", target)
	}
	// A blob past the limit is not a link target, and no runner means no read.
	if _, err := ReadBlob(context.Background(), runner, "32f64f4", 4); err == nil {
		t.Fatal("oversized blob was accepted")
	}
	if _, err := ReadBlob(context.Background(), nil, "32f64f4", 4096); err == nil {
		t.Fatal("blob read without a runner")
	}
	if _, err := ReadBlob(context.Background(), runner, "", 4096); err == nil {
		t.Fatal("blob read without an object name")
	}
}
