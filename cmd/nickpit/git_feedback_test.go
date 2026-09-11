package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	"github.com/dgrieser/nickpit/internal/session"
)

// savedLocalReview writes a session the way a local review does, so the
// listing and the matcher are exercised against real session files.
func savedLocalReview(t *testing.T, store *session.Store, repoRoot, submode, base, head, title string,
	when time.Time) *session.Session {
	t.Helper()
	sess := session.New()
	sess.Model = "test-model"
	sess.Source = session.Source{
		Mode: string(model.ModeLocal), Submode: submode,
		BaseRef: base, HeadRef: head, RepoRoot: repoRoot,
	}
	sess.Result = &model.ReviewResult{
		ReviewID: sess.ID, Revision: 1, CreatedAt: when, OverallCorrectness: "patch is correct",
		Findings: []model.Finding{{ID: "f1", Title: title, Body: "body", CodeLocation: model.CodeLocation{FilePath: "a.go"}}},
	}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestHeadRefIsBranch(t *testing.T) {
	cases := []struct {
		headRef string
		branch  string
		want    bool
	}{
		{"main", "main", true},
		{"refs/heads/main", "main", true},
		// The range submodes record "HEAD" when the head was not named, which
		// meant the checked-out branch then as it does now.
		{"HEAD", "main", true},
		{"feat/other", "main", false},
		// The working-tree submodes record no refs; `nickpit session` reads those.
		{"", "main", false},
		{"main", "", false},
	}
	for _, tc := range cases {
		if got := headRefIsBranch(tc.headRef, tc.branch); got != tc.want {
			t.Errorf("headRefIsBranch(%q, %q) = %v, want %v", tc.headRef, tc.branch, got, tc.want)
		}
	}
}

func TestSameCheckout(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "home", "dev", "repo")
	cases := []struct {
		recorded string
		want     bool
	}{
		{root, true},
		// A review run from a subdirectory is the same checkout.
		{filepath.Join(root, "internal", "review"), true},
		{filepath.Join(root, "..", "other"), false},
		{root + "-fork", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := sameCheckout(tc.recorded, root); got != tc.want {
			t.Errorf("sameCheckout(%q, %q) = %v, want %v", tc.recorded, root, got, tc.want)
		}
	}
}

func TestLocalReviewsForSelectsThisBranchOfThisRepository(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	now := time.Now().UTC()
	wanted := savedLocalReview(t, store, root, "branch", "origin/main", "main", "on this branch", now)
	alsoWanted := savedLocalReview(t, store, filepath.Join(root, "internal"), "commits", "abc123", "HEAD",
		"from a subdirectory", now.Add(-time.Hour))
	savedLocalReview(t, store, root, "branch", "origin/main", "feat/other", "another branch", now)
	savedLocalReview(t, store, root, "uncommitted", "", "", "the working tree", now)
	savedLocalReview(t, store, t.TempDir(), "branch", "origin/main", "main", "another repository", now)
	// A remote review saved from this checkout is not a local review.
	remote := session.New()
	remote.Source = session.Source{Mode: string(model.ModeGitLab), Repo: "grp/proj", Identifier: 7, HeadRef: "main"}
	remote.Result = &model.ReviewResult{ReviewID: remote.ID}
	if err := store.Save(remote); err != nil {
		t.Fatal(err)
	}

	infos, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	matching, otherBranch := localReviewsFor(infos, checkout{root: root, branch: "main"})
	gotIDs := make([]string, len(matching))
	for i, info := range matching {
		gotIDs[i] = info.ID
	}
	want := []string{wanted.ID, alsoWanted.ID}
	if len(gotIDs) != len(want) || gotIDs[0] != want[0] || gotIDs[1] != want[1] {
		t.Fatalf("matching = %v, want %v (newest first)", gotIDs, want)
	}
	// The reviews of this repository that are not of this branch are counted,
	// so an empty list can say what exists instead.
	if len(otherBranch) != 2 {
		t.Fatalf("otherBranch = %d, want 2", len(otherBranch))
	}
	if matching[0].Findings != 1 || matching[0].Verdict != "patch is correct" || matching[0].Model != "test-model" {
		t.Errorf("listing entry lost its review summary: %+v", matching[0])
	}
}

// gitRepoOnBranch creates a repository with one commit, so the command can
// resolve a working-tree root and a checked-out branch.
func gitRepoOnBranch(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", branch, ".")
	run("commit", "-q", "--allow-empty", "-m", "init")
	// The recorded review directory is compared against `rev-parse
	// --show-toplevel`, which resolves symlinks (/var on macOS); resolve here
	// too so the fixture matches what the command sees.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestRunGitFeedbackPrintsTheChosenReview(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	newest := savedLocalReview(t, store, repo, "commits", "abc123", "HEAD", "newest review", now)
	older := savedLocalReview(t, store, repo, "branch", "origin/main", "main", "older review", now.Add(-time.Hour))

	var out bytes.Buffer
	a := &app{sessionDir: sessionDir, outputFormat: "raw"}
	// The picker always chooses; the seam stands in for the terminal.
	var offered pick.Options
	a.selectFn = func(opts pick.Options) (int, error) {
		offered = opts
		return 1, nil
	}
	if err := a.runGitFeedback(context.Background(), &out, gitFeedbackOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(offered.Items) != 2 {
		t.Fatalf("picker offered %d rows", len(offered.Items))
	}
	if offered.Items[0].Detail != newest.ID {
		t.Errorf("picker is not newest-first: %+v", offered.Items[0].Cells)
	}
	if !strings.Contains(out.String(), "older review") || strings.Contains(out.String(), "newest review") {
		t.Fatalf("the chosen row was not printed:\n%s", out.String())
	}

	// --session addresses one directly, without a terminal.
	out.Reset()
	plain := &app{sessionDir: sessionDir, outputFormat: "raw"}
	if err := plain.runGitFeedback(context.Background(), &out, gitFeedbackOptions{sessionID: older.ID}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "older review") {
		t.Fatalf("--session printed the wrong review:\n%s", out.String())
	}
}

func TestRunGitFeedbackWithoutTerminalSaysHowToAddressOne(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	savedLocalReview(t, store, repo, "branch", "origin/main", "main", "a review", time.Now().UTC())

	a := &app{sessionDir: sessionDir, outputFormat: "raw"}
	err = a.runGitFeedback(context.Background(), &bytes.Buffer{}, gitFeedbackOptions{})
	if err == nil || !strings.Contains(err.Error(), "--session") || !strings.Contains(err.Error(), "--list") {
		t.Fatalf("error = %v", err)
	}
}

// An empty result says what exists for the repository but not for the branch,
// so "nothing" is not confused with "nothing was ever saved".
func TestRunGitFeedbackEmptyMentionsTheReviewsOfOtherRefs(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	savedLocalReview(t, store, repo, "uncommitted", "", "", "the working tree", time.Now().UTC())

	a := &app{sessionDir: sessionDir, outputFormat: "raw"}
	err = a.runGitFeedback(context.Background(), &bytes.Buffer{}, gitFeedbackOptions{})
	if err == nil || !strings.Contains(err.Error(), "another ref") || !strings.Contains(err.Error(), "nickpit session") {
		t.Fatalf("error = %v", err)
	}

	empty := &app{sessionDir: t.TempDir(), outputFormat: "raw"}
	err = empty.runGitFeedback(context.Background(), &bytes.Buffer{}, gitFeedbackOptions{})
	if err == nil || !strings.Contains(err.Error(), "nickpit git branch") {
		t.Fatalf("error without any saved review = %v", err)
	}
}

func TestGitFeedbackListAsJSON(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	saved := savedLocalReview(t, store, repo, "branch", "origin/main", "main", "a review", time.Now().UTC())

	var out bytes.Buffer
	a := &app{sessionDir: sessionDir, jsonOutput: true}
	if err := a.runGitFeedback(context.Background(), &out, gitFeedbackOptions{list: true}); err != nil {
		t.Fatal(err)
	}
	var entries []localReviewEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(entries) != 1 || entries[0].SessionID != saved.ID || entries[0].Findings != 1 ||
		entries[0].BaseRef != "origin/main" || entries[0].HeadRef != "main" {
		t.Fatalf("entries = %+v", entries)
	}
}

// A detached HEAD has no branch to match against; saying so beats an empty list.
func TestRunGitFeedbackOnDetachedHeadExplainsItself(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	detach := exec.Command("git", "checkout", "-q", "--detach")
	detach.Dir = repo
	if out, err := detach.CombinedOutput(); err != nil {
		t.Fatalf("git checkout --detach: %v\n%s", err, out)
	}
	t.Chdir(repo)

	a := &app{sessionDir: t.TempDir(), outputFormat: "raw"}
	err := a.runGitFeedback(context.Background(), &bytes.Buffer{}, gitFeedbackOptions{})
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("error = %v", err)
	}
}

// A session id from a published review is not a local review; the command says
// where to read it instead of printing it under the wrong name.
func TestGitFeedbackSessionRejectsARemoteReview(t *testing.T) {
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	remote := session.New()
	remote.Source = session.Source{Mode: string(model.ModeGitLab), Repo: "grp/proj", Identifier: 42}
	remote.Result = &model.ReviewResult{ReviewID: remote.ID}
	if err := store.Save(remote); err != nil {
		t.Fatal(err)
	}

	a := &app{sessionDir: sessionDir, outputFormat: "raw"}
	err = a.runGitFeedback(context.Background(), &bytes.Buffer{}, gitFeedbackOptions{sessionID: remote.ID})
	if err == nil || !strings.Contains(err.Error(), "nickpit gitlab feedback") {
		t.Fatalf("error = %v", err)
	}
}
