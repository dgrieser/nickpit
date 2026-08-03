package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

// newTestHistory wires an ExecHistory to a stub runner and pre-answers the
// probes every call makes first: the repository check and the shallow check.
func newTestHistory(runner *stubGitRunner) *ExecHistory {
	if runner.outputs == nil {
		runner.outputs = map[string]string{}
	}
	if _, ok := runner.outputs[joinArgs([]string{"rev-parse", "--git-dir"})]; !ok {
		runner.outputs[joinArgs([]string{"rev-parse", "--git-dir"})] = ".git\n"
	}
	if _, ok := runner.outputs[joinArgs([]string{"rev-parse", "--is-shallow-repository"})]; !ok {
		runner.outputs[joinArgs([]string{"rev-parse", "--is-shallow-repository"})] = "false\n"
	}
	if _, ok := runner.outputs[joinArgs([]string{"rev-parse", "--verify", "--end-of-options", "HEAD^{commit}"})]; !ok {
		resolved(runner, "HEAD", "aaa111")
	}
	history := NewExecHistory(HistoryAuth{})
	history.newRunner = func(string) Runner { return runner }
	return history
}

// resolved registers the rev-parse answer that resolves rev to sha.
func resolved(runner *stubGitRunner, rev, sha string) {
	runner.outputs[joinArgs([]string{"rev-parse", "--verify", "--end-of-options", rev + "^{commit}"})] = sha + "\n"
}

// logRecord renders one commitFormat record; commitDate defaults to date when
// empty, matching the common case where a commit was never rewritten.
func logRecord(sha, short, author, email, date, commitDate, parents, subject, body, files string) string {
	if commitDate == "" {
		commitDate = date
	}
	return recordSeparator + strings.Join([]string{sha, short, author, email, date, commitDate, parents, subject, body}, fieldSeparator) +
		fieldSeparator + files
}

// metadataRecord renders the `git show --no-patch` answer for one commit.
func metadataRecord(sha, short, author, email, date, parents, subject string) string {
	return strings.Join([]string{sha, short, author, email, date, date, parents, subject, ""}, fieldSeparator) + fieldSeparator
}

// showRevision extracts the revision a `git show` invocation targets: the last
// argument before the "--" pathspec separator, or simply the last one.
func showRevision(args []string) string {
	for i, arg := range args {
		if arg == "--" && i > 0 {
			return args[i-1]
		}
	}
	return args[len(args)-1]
}

func findCall(calls [][]string, name string) []string {
	for _, call := range calls {
		for _, arg := range call {
			if arg == name {
				return call
			}
		}
	}
	return nil
}

func countCalls(calls [][]string, first string) int {
	count := 0
	for _, call := range calls {
		if len(call) > 0 && call[0] == first {
			count++
		}
	}
	return count
}

func TestLogParsesCommitsWithChangedFiles(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	files := "\x00\n:100644 100644 1111111 2222222 M\x00internal/a.go\x00:000000 100644 0000000 3333333 A\x00internal/b.go\x00" +
		"\n12\t3\tinternal/a.go\x0040\t0\tinternal/b.go\x00"
	history := newTestHistory(runner)
	// The log call carries many arguments, so answer any log invocation.
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		return logRecord("aaa111", "aaa111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "2026-08-02T11:00:00Z", "bbb222",
			"feat(a): add thing", "body line\n", files), true
	}

	result, err := history.Log(context.Background(), t.TempDir(), LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitCount != 1 || len(result.Commits) != 1 {
		t.Fatalf("commit count = %d", result.CommitCount)
	}
	commit := result.Commits[0]
	if commit.SHA != "aaa111" || commit.Author != "Ada" || commit.AuthorEmail != "ada@example.com" {
		t.Fatalf("commit metadata = %+v", commit)
	}
	if commit.Subject != "feat(a): add thing" || commit.Body != "body line" {
		t.Fatalf("commit message = %q / %q", commit.Subject, commit.Body)
	}
	if commit.IsMerge || len(commit.Parents) != 1 {
		t.Fatalf("parents = %#v, merge = %t", commit.Parents, commit.IsMerge)
	}
	if commit.Date.IsZero() {
		t.Fatalf("date not parsed: %+v", commit.Date)
	}
	if commit.Additions != 52 || commit.Deletions != 3 {
		t.Fatalf("totals = +%d -%d", commit.Additions, commit.Deletions)
	}
	want := []CommitFile{
		{Path: "internal/a.go", Status: model.FileModified, Additions: 12, Deletions: 3},
		{Path: "internal/b.go", Status: model.FileAdded, Additions: 40, Deletions: 0},
	}
	if len(commit.Files) != len(want) {
		t.Fatalf("files = %#v", commit.Files)
	}
	for i, file := range want {
		if commit.Files[i] != file {
			t.Fatalf("file[%d] = %+v, want %+v", i, commit.Files[i], file)
		}
	}
}

func TestLogParsesRenamesDeletionsAndBinaries(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	files := "\x00\n:100644 100644 1111111 2222222 R096\x00old/name.go\x00new/name.go\x00" +
		":100644 000000 3333333 0000000 D\x00gone.go\x00:100644 100644 4444444 5555555 M\x00logo.png\x00" +
		"\n4\t2\t\x00old/name.go\x00new/name.go\x000\t9\tgone.go\x00-\t-\tlogo.png\x00"
	history := newTestHistory(runner)
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		return logRecord("aaa111", "aaa111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "", "bbb222",
			"chore: move", "", files), true
	}

	result, err := history.Log(context.Background(), t.TempDir(), LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	files0 := result.Commits[0].Files
	if len(files0) != 3 {
		t.Fatalf("files = %#v", files0)
	}
	if files0[0] != (CommitFile{Path: "new/name.go", OldPath: "old/name.go", Status: model.FileRenamed, Additions: 4, Deletions: 2}) {
		t.Fatalf("rename = %+v", files0[0])
	}
	if files0[1] != (CommitFile{Path: "gone.go", Status: model.FileDeleted, Additions: 0, Deletions: 9}) {
		t.Fatalf("deletion = %+v", files0[1])
	}
	if !files0[2].Binary || files0[2].Path != "logo.png" {
		t.Fatalf("binary = %+v", files0[2])
	}
}

func TestLogAppliesFiltersAndLimit(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	history := newTestHistory(runner)
	var logArgs []string
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		logArgs = append([]string(nil), args...)
		records := make([]string, 0, 3)
		for i := 0; i < 3; i++ {
			records = append(records, logRecord(fmt.Sprintf("sha%d", i), fmt.Sprintf("sha%d", i), "Ada", "ada@example.com",
				"2026-08-01T10:00:00Z", "", "parent", "subject", "", ""))
		}
		return strings.Join(records, ""), true
	}

	result, err := history.Log(context.Background(), t.TempDir(), LogOptions{
		Since:   "2026-01-01",
		Until:   "2026-08-01",
		Author:  "Ada",
		Message: "fix(",
		Paths:   []string{"internal/serve", "cmd"},
		Limit:   2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Commits) != 2 || !result.Truncated {
		t.Fatalf("commits = %d, truncated = %t", len(result.Commits), result.Truncated)
	}
	joined := strings.Join(logArgs, " ")
	for _, want := range []string{
		"--max-count=3", "--since=2026-01-01", "--until=2026-08-01", "--author=Ada",
		"--grep=fix(", "--fixed-strings", "--regexp-ignore-case", "--diff-merges=first-parent",
		"aaa111", "-- internal/serve cmd",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("log args missing %q: %v", want, logArgs)
		}
	}
	if strings.Contains(joined, "--extended-regexp") {
		t.Fatalf("literal message search should not use extended regex: %v", logArgs)
	}
}

func TestLogUsesExtendedRegexForRegexSearch(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	history := newTestHistory(runner)
	var logArgs []string
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		logArgs = append([]string(nil), args...)
		return "", true
	}

	if _, err := history.Log(context.Background(), t.TempDir(), LogOptions{
		Message:       `^feat\(.*\)`,
		MessageRegex:  true,
		CaseSensitive: true,
	}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(logArgs, " ")
	if !strings.Contains(joined, "--extended-regexp") || strings.Contains(joined, "--fixed-strings") {
		t.Fatalf("regex search args = %v", logArgs)
	}
	if strings.Contains(joined, "--regexp-ignore-case") {
		t.Fatalf("case-sensitive search should not ignore case: %v", logArgs)
	}
}

func TestLogRetriesWithoutDiffMergesOnOldGit(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	history := newTestHistory(runner)
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		return logRecord("aaa111", "aaa111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "", "bbb222", "subject", "", ""), true
	}
	runner.matchErr = func(args []string) error {
		if args[0] == "log" && strings.Contains(strings.Join(args, " "), "--diff-merges=first-parent") {
			return errors.New("error: unknown option `diff-merges=first-parent'")
		}
		return nil
	}

	result, err := history.Log(context.Background(), t.TempDir(), LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("commits = %d", len(result.Commits))
	}
	if attempts := countCalls(runner.recordedCalls(), "log"); attempts != 2 {
		t.Fatalf("log attempts = %d, want 2 (one rejected, one retried without --diff-merges)", attempts)
	}
}

func TestShowReturnsOneDiffPerCommitInRange(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	resolved(runner, "aaa", "aaa111")
	resolved(runner, "bbb", "bbb222")
	runner.outputs[joinArgs([]string{"rev-list", "--max-count=11", "aaa111..bbb222"})] = "bbb222\nccc333\n"
	history := newTestHistory(runner)
	patch := func(path string) string {
		return strings.Join([]string{
			"diff --git a/" + path + " b/" + path,
			"index 111..222 100644",
			"--- a/" + path,
			"+++ b/" + path,
			"@@ -1,2 +1,3 @@",
			" keep",
			"+added",
			" keep",
			"",
		}, "\n")
	}
	runner.match = func(args []string) (string, bool) {
		if args[0] != "show" {
			return "", false
		}
		sha := args[len(args)-1]
		if args[1] == "--no-patch" {
			return metadataRecord(sha, sha[:3], "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "parent111", "subject "+sha), true
		}
		return patch("internal/" + sha + ".go"), true
	}

	result, err := history.Show(context.Background(), t.TempDir(), ShowOptions{Commit: "aaa..bbb", Format: model.DiffFormatGit})
	if err != nil {
		t.Fatal(err)
	}
	if result.Range != "aaa111..bbb222" || result.DiffFormat != "git" {
		t.Fatalf("range/format = %q/%q", result.Range, result.DiffFormat)
	}
	if result.CommitCount != 2 {
		t.Fatalf("commit count = %d", result.CommitCount)
	}
	for i, commit := range result.Commits {
		if commit.DiffMode != "single" {
			t.Fatalf("commit[%d] diff mode = %q", i, commit.DiffMode)
		}
		if len(commit.DiffFiles) != 1 || len(commit.DiffHunks) != 0 {
			t.Fatalf("commit[%d] files/hunks = %d/%d", i, len(commit.DiffFiles), len(commit.DiffHunks))
		}
		if commit.Additions != 1 || commit.Deletions != 0 {
			t.Fatalf("commit[%d] totals = +%d -%d", i, commit.Additions, commit.Deletions)
		}
	}
}

func TestShowSingleCommitIgnoresPathspecForCommitSelection(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	resolved(runner, "aaa", "aaa111")
	history := newTestHistory(runner)
	runner.match = func(args []string) (string, bool) {
		if args[0] != "show" {
			return "", false
		}
		if args[1] == "--no-patch" {
			return metadataRecord("aaa111", "aaa111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "parent", "subject"), true
		}
		return "", true // pathspec matches nothing
	}

	result, err := history.Show(context.Background(), t.TempDir(), ShowOptions{Commit: "aaa", Paths: []string{"other"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls := runner.recordedCalls(); countCalls(calls, "rev-list") != 0 {
		t.Fatalf("single commit should not consult rev-list: %v", calls)
	}
	if len(result.Commits) != 1 || result.Commits[0].SHA != "aaa111" {
		t.Fatalf("commits = %#v", result.Commits)
	}
	if !strings.Contains(result.Commits[0].Note, "no changes in the requested paths") {
		t.Fatalf("note = %q", result.Commits[0].Note)
	}
}

func TestShowFallsBackToFirstParentWhenCombinedDiffIsEmpty(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	resolved(runner, "mmm", "mmm111")
	history := newTestHistory(runner)
	firstParentCalls := 0
	runner.match = func(args []string) (string, bool) {
		if args[0] != "show" {
			return "", false
		}
		joined := strings.Join(args, " ")
		switch {
		case args[1] == "--no-patch":
			return metadataRecord("mmm111", "mmm111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "p1 p2", "Merge branch"), true
		case strings.Contains(joined, "--cc"):
			return "", true
		default:
			firstParentCalls++
			return strings.Join([]string{
				"diff --git a/a.go b/a.go",
				"index 111..222 100644",
				"--- a/a.go",
				"+++ b/a.go",
				"@@ -1 +1,2 @@",
				" keep",
				"+added",
				"",
			}, "\n"), true
		}
	}

	result, err := history.Show(context.Background(), t.TempDir(), ShowOptions{Commit: "mmm"})
	if err != nil {
		t.Fatal(err)
	}
	commit := result.Commits[0]
	if !commit.IsMerge || commit.DiffMode != "first-parent" {
		t.Fatalf("merge = %t, diff mode = %q", commit.IsMerge, commit.DiffMode)
	}
	if firstParentCalls != 1 {
		t.Fatalf("first-parent calls = %d", firstParentCalls)
	}
	if len(commit.DiffFiles) != 1 {
		t.Fatalf("diff files = %#v", commit.DiffFiles)
	}
	if !strings.Contains(commit.Note, "combined diff was empty") {
		t.Fatalf("note = %q", commit.Note)
	}
	if call := findCall(runner.recordedCalls(), "--first-parent"); call == nil {
		t.Fatalf("expected a first-parent show call: %v", runner.recordedCalls())
	}
}

func TestShowKeepsCombinedPatchWhenGitJSONHasNoHunks(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	resolved(runner, "mmm", "mmm111")
	history := newTestHistory(runner)
	runner.match = func(args []string) (string, bool) {
		if args[0] != "show" {
			return "", false
		}
		if args[1] == "--no-patch" {
			return metadataRecord("mmm111", "mmm111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "p1 p2", "Merge branch"), true
		}
		return strings.Join([]string{
			"diff --cc a.go",
			"index 111,222..333",
			"--- a/a.go",
			"+++ b/a.go",
			"@@@ -1,2 -1,2 +1,3 @@@",
			"  keep",
			"++resolved",
			"",
		}, "\n"), true
	}

	result, err := history.Show(context.Background(), t.TempDir(), ShowOptions{Commit: "mmm", Format: model.DiffFormatGitJson})
	if err != nil {
		t.Fatal(err)
	}
	commit := result.Commits[0]
	if commit.DiffMode != "combined" {
		t.Fatalf("diff mode = %q", commit.DiffMode)
	}
	if len(commit.DiffFiles) != 1 || commit.DiffFiles[0].FilePath != "a.go" {
		t.Fatalf("diff files = %#v", commit.DiffFiles)
	}
	if !strings.Contains(commit.Note, "combined hunks") {
		t.Fatalf("note = %q", commit.Note)
	}
}

func TestShowTruncatesOversizedPatch(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	resolved(runner, "aaa", "aaa111")
	history := newTestHistory(runner)
	runner.match = func(args []string) (string, bool) {
		if args[0] != "show" {
			return "", false
		}
		if args[1] == "--no-patch" {
			return metadataRecord("aaa111", "aaa111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "parent", "subject"), true
		}
		return "diff --git a/a.go b/a.go\n" + strings.Repeat("+line\n", maxCommitPatchBytes/6+10), true
	}

	result, err := history.Show(context.Background(), t.TempDir(), ShowOptions{Commit: "aaa"})
	if err != nil {
		t.Fatal(err)
	}
	commit := result.Commits[0]
	if !commit.Truncated || !result.Truncated {
		t.Fatalf("truncated = %t/%t", commit.Truncated, result.Truncated)
	}
	if len(commit.DiffFiles) != 0 || len(commit.DiffHunks) != 0 {
		t.Fatalf("oversized patch should not be returned: %#v", commit.DiffFiles)
	}
	if !strings.Contains(commit.Note, "exceeds") {
		t.Fatalf("note = %q", commit.Note)
	}
}

func TestShowRequiresACommit(t *testing.T) {
	history := newTestHistory(&stubGitRunner{outputs: map[string]string{}})
	if _, err := history.Show(context.Background(), t.TempDir(), ShowOptions{Commit: "  "}); err == nil {
		t.Fatal("expected an error for a missing commit")
	}
}

func TestResolveRevisionFallsBackToPrefixScan(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{
		joinArgs([]string{"log", "--all", "--format=%H"}): "aaa111222333\nbbb444555666\n",
	}}
	runner.errors = map[string]error{
		joinArgs([]string{"rev-parse", "--verify", "--end-of-options", "aa^{commit}"}): errors.New("fatal: needed a single revision"),
		joinArgs([]string{"rev-parse", "--verify", "--end-of-options", "x^{commit}"}):  errors.New("fatal: needed a single revision"),
	}
	history := newTestHistory(runner)

	sha, err := history.resolveRevision(context.Background(), runner, "aa")
	if err != nil {
		t.Fatal(err)
	}
	if sha != "aaa111222333" {
		t.Fatalf("sha = %q", sha)
	}
	if _, err := history.resolveRevision(context.Background(), runner, "x"); err == nil {
		t.Fatal("expected an error for a non-hex unknown revision")
	}
}

func TestResolveRevisionReportsAmbiguousPrefix(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{
		joinArgs([]string{"log", "--all", "--format=%H"}): "aaa111\naaa222\n",
	}}
	runner.errors = map[string]error{
		joinArgs([]string{"rev-parse", "--verify", "--end-of-options", "aaa^{commit}"}): errors.New("fatal: ambiguous"),
	}
	history := newTestHistory(runner)

	_, err := history.resolveRevision(context.Background(), runner, "aaa")
	if err == nil || !strings.Contains(err.Error(), "ambiguous revision") {
		t.Fatalf("error = %v", err)
	}
}

func TestHistoryRejectsOptionInjection(t *testing.T) {
	history := newTestHistory(&stubGitRunner{outputs: map[string]string{}})
	repoRoot := t.TempDir()
	cases := []struct {
		name string
		opts LogOptions
	}{
		{name: "commit", opts: LogOptions{Commit: "--upload-pack=evil"}},
		{name: "author", opts: LogOptions{Author: "--output=/tmp/evil"}},
		{name: "since", opts: LogOptions{Since: "--exec=evil"}},
		{name: "paths", opts: LogOptions{Paths: []string{"-x"}}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := history.Log(context.Background(), repoRoot, tt.opts); err == nil {
				t.Fatalf("expected %s to be rejected", tt.name)
			}
		})
	}
}

func TestHistoryRequiresAGitRepository(t *testing.T) {
	runner := &stubGitRunner{
		outputs: map[string]string{},
		errors: map[string]error{
			joinArgs([]string{"rev-parse", "--git-dir"}): errors.New("fatal: not a git repository"),
		},
	}
	history := NewExecHistory(HistoryAuth{})
	history.newRunner = func(string) Runner { return runner }

	_, err := history.Log(context.Background(), t.TempDir(), LogOptions{})
	if !errors.Is(err, ErrNotAGitRepo) {
		t.Fatalf("error = %v, want ErrNotAGitRepo", err)
	}
}

func TestShallowCheckoutIsDeepenedOnceForConcurrentCalls(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{
		joinArgs([]string{"rev-parse", "--git-dir"}):               ".git\n",
		joinArgs([]string{"rev-parse", "--is-shallow-repository"}): "true\n",
		joinArgs([]string{"rev-list", "--count", "HEAD"}):          "1\n",
		joinArgs([]string{"config", "--get", "remote.origin.url"}): "https://github.example.com/acme/repo.git\n",
		joinArgs([]string{"rev-parse", "HEAD"}):                    "aaa111\n",
	}}
	resolved(runner, "HEAD", "aaa111")
	history := NewExecHistory(HistoryAuth{})
	history.newRunner = func(string) Runner { return runner }
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		return logRecord("aaa111", "aaa111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "", "", "subject", "", ""), true
	}

	repoRoot := t.TempDir()
	var wg sync.WaitGroup
	results := make([]*LogResult, 4)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := history.Log(context.Background(), repoRoot, LogOptions{})
			if err != nil {
				t.Errorf("log: %v", err)
				return
			}
			results[i] = result
		}(i)
	}
	wg.Wait()

	calls := runner.recordedCalls()
	deepens := 0
	for _, call := range calls {
		for _, arg := range call {
			if arg == "fetch" {
				deepens++
				break
			}
		}
	}
	// One plain deepen plus at most one explicit deepen by SHA, from the single
	// memoized attempt shared by all four concurrent calls.
	if deepens == 0 || deepens > 2 {
		t.Fatalf("deepen fetches = %d, want 1 or 2 for four concurrent calls: %v", deepens, calls)
	}
	for i, result := range results {
		if result == nil {
			t.Fatalf("result[%d] missing", i)
		}
		if !result.Shallow || result.Note == "" {
			t.Fatalf("result[%d] shallow = %t, note = %q", i, result.Shallow, result.Note)
		}
	}
}

func TestDeepenSendsTokenOnlyToConfiguredProviderHosts(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		auth    HistoryAuth
		wantSet bool
	}{
		{
			name:    "github.com",
			origin:  "https://github.com/acme/repo.git",
			auth:    HistoryAuth{GitHubToken: "ghp-secret"},
			wantSet: true,
		},
		{
			name:    "github subdomain",
			origin:  "https://api.github.com/acme/repo.git",
			auth:    HistoryAuth{GitHubToken: "ghp-secret"},
			wantSet: true,
		},
		{
			// The origin URL of a fork PR/MR is attacker controlled, so a host
			// that merely contains "github" must never receive the token.
			name:   "lookalike github host",
			origin: "https://github.attacker.example/acme/repo.git",
			auth:   HistoryAuth{GitHubToken: "ghp-secret"},
		},
		{
			name:   "host with github as a prefix",
			origin: "https://github.com.attacker.example/acme/repo.git",
			auth:   HistoryAuth{GitHubToken: "ghp-secret"},
		},
		{
			name:   "unconfigured github enterprise host",
			origin: "https://github.enterprise.internal/acme/repo.git",
			auth:   HistoryAuth{GitHubToken: "ghp-secret"},
		},
		{
			name:    "configured self-hosted gitlab",
			origin:  "https://gitlab.example.com/acme/repo.git",
			auth:    HistoryAuth{GitLabToken: "glpat-secret", GitLabBaseURL: "https://gitlab.example.com/api/v4"},
			wantSet: true,
		},
		{
			name:   "gitlab host other than the configured one",
			origin: "https://gitlab.other.example/acme/repo.git",
			auth:   HistoryAuth{GitLabToken: "glpat-secret", GitLabBaseURL: "https://gitlab.example.com/api/v4"},
		},
		{
			name:    "gitlab.com without a configured base url",
			origin:  "https://gitlab.com/acme/repo.git",
			auth:    HistoryAuth{GitLabToken: "glpat-secret"},
			wantSet: true,
		},
		{
			// ssh remotes authenticate with keys; an Authorization header would
			// only leak the token into the command line for no benefit.
			name:   "ssh remote",
			origin: "git@github.com:acme/repo.git",
			auth:   HistoryAuth{GitHubToken: "ghp-secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &stubGitRunner{outputs: map[string]string{
				joinArgs([]string{"rev-parse", "--git-dir"}):               ".git\n",
				joinArgs([]string{"rev-parse", "--is-shallow-repository"}): "true\n",
				joinArgs([]string{"rev-list", "--count", "HEAD"}):          "1\n",
				joinArgs([]string{"config", "--get", "remote.origin.url"}): tt.origin + "\n",
				joinArgs([]string{"rev-parse", "HEAD"}):                    "aaa111\n",
			}}
			resolved(runner, "HEAD", "aaa111")
			history := NewExecHistory(tt.auth)
			history.newRunner = func(string) Runner { return runner }
			runner.match = func(args []string) (string, bool) {
				return "", args[0] == "log"
			}

			if _, err := history.Log(context.Background(), t.TempDir(), LogOptions{}); err != nil {
				t.Fatal(err)
			}
			call := findCall(runner.recordedCalls(), "fetch")
			if call == nil {
				t.Fatalf("expected a deepen fetch: %v", runner.recordedCalls())
			}
			hasHeader := len(call) > 1 && call[0] == "-c" && strings.HasPrefix(call[1], "http.extraHeader=Authorization: Basic ")
			if hasHeader != tt.wantSet {
				t.Fatalf("credential header present = %t, want %t: %#v", hasHeader, tt.wantSet, call)
			}
			joined := strings.Join(call, " ")
			for _, token := range []string{"ghp-secret", "glpat-secret"} {
				if strings.Contains(joined, token) {
					t.Fatalf("token must travel base64-encoded in the header: %#v", call)
				}
			}
		})
	}
}

func TestLogAppliesMatchModeToAuthorOnlyFilters(t *testing.T) {
	tests := []struct {
		name     string
		opts     LogOptions
		want     string
		unwanted string
	}{
		{
			// git's default is basic-regex matching, under which a literal
			// "Grie[s]er" silently becomes a character class.
			name:     "literal author",
			opts:     LogOptions{Author: "Grie[s]er"},
			want:     "--fixed-strings",
			unwanted: "--extended-regexp",
		},
		{
			name:     "regex author",
			opts:     LogOptions{Author: "Grie(s)er", MessageRegex: true},
			want:     "--extended-regexp",
			unwanted: "--fixed-strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &stubGitRunner{outputs: map[string]string{}}
			history := newTestHistory(runner)
			var logArgs []string
			runner.match = func(args []string) (string, bool) {
				if args[0] != "log" {
					return "", false
				}
				logArgs = append([]string(nil), args...)
				return "", true
			}

			if _, err := history.Log(context.Background(), t.TempDir(), tt.opts); err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(logArgs, " ")
			if !strings.Contains(joined, tt.want) {
				t.Fatalf("log args missing %q: %v", tt.want, logArgs)
			}
			if strings.Contains(joined, tt.unwanted) {
				t.Fatalf("log args should not contain %q: %v", tt.unwanted, logArgs)
			}
			if !strings.Contains(joined, "--regexp-ignore-case") {
				t.Fatalf("case-insensitive default missing: %v", logArgs)
			}
		})
	}

	// With no pattern at all there is nothing to select a match mode for.
	runner := &stubGitRunner{outputs: map[string]string{}}
	history := newTestHistory(runner)
	var logArgs []string
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		logArgs = append([]string(nil), args...)
		return "", true
	}
	if _, err := history.Log(context.Background(), t.TempDir(), LogOptions{}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(logArgs, " ")
	for _, flag := range []string{"--fixed-strings", "--extended-regexp", "--regexp-ignore-case"} {
		if strings.Contains(joined, flag) {
			t.Fatalf("unfiltered log should not pass %q: %v", flag, logArgs)
		}
	}
}

// The author date is what agents read; the commit date is what --since/--until
// select on, so both must reach the result.
func TestLogReportsAuthorAndCommitDates(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	history := newTestHistory(runner)
	runner.match = func(args []string) (string, bool) {
		if args[0] != "log" {
			return "", false
		}
		return logRecord("aaa111", "aaa111", "Ada", "ada@example.com",
			"2026-07-29T20:22:35Z", "2026-07-31T13:38:06Z", "bbb222", "subject", "", ""), true
	}

	result, err := history.Log(context.Background(), t.TempDir(), LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	commit := result.Commits[0]
	if got := commit.Date.Format(time.RFC3339); got != "2026-07-29T20:22:35Z" {
		t.Fatalf("author date = %q", got)
	}
	if got := commit.CommitDate.Format(time.RFC3339); got != "2026-07-31T13:38:06Z" {
		t.Fatalf("commit date = %q", got)
	}
}

// Path filters narrow each diff; they must not decide which commits of a range
// are returned, or a range would silently skip commits.
func TestShowRangeKeepsCommitsThatMissThePathFilter(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	resolved(runner, "aaa", "aaa111")
	resolved(runner, "bbb", "bbb222")
	runner.outputs[joinArgs([]string{"rev-list", "--max-count=11", "aaa111..bbb222"})] = "bbb222\nccc333\n"
	history := newTestHistory(runner)
	runner.match = func(args []string) (string, bool) {
		if args[0] != "show" {
			return "", false
		}
		sha := showRevision(args)
		if len(args) > 1 && args[1] == "--no-patch" {
			return metadataRecord(sha, sha[:3], "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "parent", "subject "+sha), true
		}
		if sha != "bbb222" {
			return "", true // ccc333 touched none of the requested paths
		}
		return strings.Join([]string{
			"diff --git a/internal/serve/server.go b/internal/serve/server.go",
			"index 111..222 100644",
			"--- a/internal/serve/server.go",
			"+++ b/internal/serve/server.go",
			"@@ -1 +1,2 @@",
			" keep",
			"+added",
			"",
		}, "\n"), true
	}

	result, err := history.Show(context.Background(), t.TempDir(), ShowOptions{
		Commit: "aaa..bbb",
		Paths:  []string{"internal/serve"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.recordedCalls() {
		if call[0] == "rev-list" && slices.Contains(call, "--") {
			t.Fatalf("rev-list must not receive the pathspec: %#v", call)
		}
	}
	if result.CommitCount != 2 {
		t.Fatalf("commit count = %d, want both range commits", result.CommitCount)
	}
	if len(result.Commits[0].DiffFiles) != 1 {
		t.Fatalf("matching commit lost its diff: %#v", result.Commits[0])
	}
	empty := result.Commits[1]
	if len(empty.DiffFiles) != 0 || !strings.Contains(empty.Note, "no changes in the requested paths") {
		t.Fatalf("non-matching commit = %+v", empty)
	}
	if empty.SHA != "ccc333" {
		t.Fatalf("second commit = %q", empty.SHA)
	}
}

// An oversized patch is dropped, but what the commit changed must still be
// reported or an agent cannot tell whether the commit matters.
func TestShowKeepsChangedFilesWhenPatchIsOmitted(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{}}
	resolved(runner, "aaa", "aaa111")
	history := newTestHistory(runner)
	rawCalls := 0
	runner.match = func(args []string) (string, bool) {
		if args[0] != "show" {
			return "", false
		}
		joined := strings.Join(args, " ")
		switch {
		case args[1] == "--no-patch":
			return metadataRecord("aaa111", "aaa111", "Ada", "ada@example.com", "2026-08-01T10:00:00Z", "parent", "subject"), true
		case strings.Contains(joined, "--raw"):
			rawCalls++
			return "\n:100644 100644 1111111 2222222 M\x00big.go\x00\n900\t120\tbig.go\x00", true
		default:
			return "diff --git a/big.go b/big.go\n" + strings.Repeat("+line\n", maxCommitPatchBytes/6+10), true
		}
	}

	result, err := history.Show(context.Background(), t.TempDir(), ShowOptions{Commit: "aaa"})
	if err != nil {
		t.Fatal(err)
	}
	commit := result.Commits[0]
	if !commit.Truncated || len(commit.DiffFiles) != 0 {
		t.Fatalf("oversized patch should be dropped: %+v", commit)
	}
	if rawCalls != 1 {
		t.Fatalf("raw metadata calls = %d, want 1", rawCalls)
	}
	want := []CommitFile{{Path: "big.go", Status: model.FileModified, Additions: 900, Deletions: 120}}
	if len(commit.Files) != 1 || commit.Files[0] != want[0] {
		t.Fatalf("files = %#v, want %#v", commit.Files, want)
	}
	if commit.Additions != 900 || commit.Deletions != 120 {
		t.Fatalf("totals = +%d -%d", commit.Additions, commit.Deletions)
	}
}
