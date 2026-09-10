package git

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBranchesParsesRefRecords(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{
		joinArgs([]string{"for-each-ref", "--sort=-committerdate", "--format=" + branchRefFormat, "refs/heads", "refs/remotes"}): strings.Join([]string{
			"feature/pick\x00refs/heads/feature/pick\x00\x001757000000\x00Alice\x00feat(cli): add a picker",
			"main\x00refs/heads/main\x00\x001756000000\x00Bob\x00fix(llm): retry on 429",
			// origin/HEAD is symbolic — and its short name is just "origin", so
			// only the symref field gives it away. Offering it would duplicate
			// its target under a name no review should record.
			"origin\x00refs/remotes/origin/HEAD\x00refs/remotes/origin/main\x001756000000\x00Bob\x00fix(llm): retry on 429",
			"origin/main\x00refs/remotes/origin/main\x00\x001755000000\x00Bob\x00fix(llm): retry on 429",
			// A record git could not fill completely is skipped, not half-read.
			"broken\x00refs/heads/broken",
		}, "\n") + "\n",
		joinArgs([]string{"symbolic-ref", "--short", "HEAD"}): "feature/pick\n",
	}}
	branches, err := branches(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 3 {
		t.Fatalf("branches = %d (%+v), want 3", len(branches), branches)
	}
	first := branches[0]
	if first.Name != "feature/pick" || !first.Current || first.Remote != "" {
		t.Fatalf("first branch = %+v, want the checked-out local feature/pick", first)
	}
	if first.Branch != "feature/pick" {
		t.Fatalf("first branch name = %q, want the slash kept", first.Branch)
	}
	if first.Author != "Alice" || first.Subject != "feat(cli): add a picker" {
		t.Fatalf("first branch metadata = %+v", first)
	}
	if !first.Date.Equal(time.Unix(1757000000, 0)) {
		t.Fatalf("first branch date = %s", first.Date)
	}
	if branches[1].Name != "main" || branches[1].Current {
		t.Fatalf("second branch = %+v, want main unmarked", branches[1])
	}
	if branches[2].Remote != "origin" || branches[2].Name != "origin/main" {
		t.Fatalf("third branch = %+v, want the remote origin/main", branches[2])
	}
	// The local main and origin/main share a branch name, which is what lets a
	// picker fold them into one row.
	if branches[2].Branch != "main" || branches[1].Branch != "main" {
		t.Fatalf("branch names = %q / %q, want both to be main", branches[1].Branch, branches[2].Branch)
	}
}

// A subject may contain anything but NUL, so the framing must not be fooled by
// a crafted commit message.
func TestBranchesSubjectWithSeparatorLookalikes(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{
		joinArgs([]string{"for-each-ref", "--sort=-committerdate", "--format=" + branchRefFormat, "refs/heads", "refs/remotes"}): "wip\x00refs/heads/wip\x00\x001757000000\x00Alice\x00fix: handle a\ttab and | pipe\n",
	}}
	got, err := branches(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Subject != "fix: handle a\ttab and | pipe" {
		t.Fatalf("branches = %+v", got)
	}
}

func TestCommitsParsesLogRecords(t *testing.T) {
	runner := &stubGitRunner{outputs: map[string]string{
		joinArgs([]string{"log", "--max-count=2", "--format=" + commitRefFormat, "HEAD"}): strings.Join([]string{
			"1111111111111111111111111111111111111111\x001111111\x001757000000\x00Alice\x00feat(cli): add a picker",
			"2222222222222222222222222222222222222222\x002222222\x001756000000\x00Bob\x00fix(llm): retry on 429",
		}, "\n") + "\n",
	}}
	commits, err := commits(context.Background(), runner, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("commits = %d (%+v), want 2", len(commits), commits)
	}
	if commits[0].SHA != "1111111111111111111111111111111111111111" || commits[0].ShortSHA != "1111111" {
		t.Fatalf("first commit = %+v", commits[0])
	}
	if commits[0].Author != "Alice" || commits[0].Subject != "feat(cli): add a picker" {
		t.Fatalf("first commit metadata = %+v", commits[0])
	}
	if !commits[1].Date.Equal(time.Unix(1756000000, 0)) {
		t.Fatalf("second commit date = %s", commits[1].Date)
	}
}

func TestCommitsRejectsNonPositiveLimit(t *testing.T) {
	if _, err := commits(context.Background(), &stubGitRunner{}, "HEAD", 0); err == nil {
		t.Fatal("expected an error for a zero limit")
	}
}

func TestBranchesPropagatesGitFailure(t *testing.T) {
	want := errors.New("boom")
	runner := &stubGitRunner{errors: map[string]error{
		joinArgs([]string{"for-each-ref", "--sort=-committerdate", "--format=" + branchRefFormat, "refs/heads", "refs/remotes"}): want,
	}}
	if _, err := branches(context.Background(), runner); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// A detached HEAD has no branch; the listing still works, it just marks nothing.
func TestBranchesWithoutCurrentBranch(t *testing.T) {
	runner := &stubGitRunner{
		outputs: map[string]string{
			joinArgs([]string{"for-each-ref", "--sort=-committerdate", "--format=" + branchRefFormat, "refs/heads", "refs/remotes"}): "main\x00refs/heads/main\x00\x001756000000\x00Bob\x00fix\n",
		},
		errors: map[string]error{
			joinArgs([]string{"symbolic-ref", "--short", "HEAD"}): errors.New("detached"),
		},
	}
	got, err := branches(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Current {
		t.Fatalf("branches = %+v, want main unmarked", got)
	}
}

func TestSplitBranchRef(t *testing.T) {
	cases := []struct {
		refName    string
		wantRemote string
		wantBranch string
	}{
		{"refs/heads/main", "", "main"},
		{"refs/heads/feat/a/b", "", "feat/a/b"},
		{"refs/remotes/origin/main", "origin", "main"},
		{"refs/remotes/origin/feat/a/b", "origin", "feat/a/b"},
		// A remote's own HEAD names no branch of its own.
		{"refs/remotes/origin/HEAD", "origin", ""},
		{"refs/remotes/origin", "origin", ""},
		{"refs/tags/v1", "", ""},
	}
	for _, tc := range cases {
		remote, branch := splitBranchRef(tc.refName)
		if remote != tc.wantRemote || branch != tc.wantBranch {
			t.Fatalf("splitBranchRef(%q) = %q, %q; want %q, %q",
				tc.refName, remote, branch, tc.wantRemote, tc.wantBranch)
		}
	}
}

func TestUnixTimeIgnoresGarbage(t *testing.T) {
	if got := unixTime("not-a-number"); !got.IsZero() {
		t.Fatalf("unixTime = %s, want the zero time", got)
	}
}
