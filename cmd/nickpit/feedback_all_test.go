package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	"github.com/dgrieser/nickpit/internal/session"
)

// openRequestsWithReviews is a project where only some open requests carry a
// published review, which is what the merged list has to filter on.
func openRequestsWithReviews(t *testing.T, failing int) requestFeedbackSource {
	t.Helper()
	requests := []model.OpenRequest{
		{Identifier: 42, Title: "reviewed request", Author: "alice", SourceBranch: "feat/one",
			UpdatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{Identifier: 43, Title: "unreviewed request", Author: "bob", SourceBranch: "feat/two",
			UpdatedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)},
		{Identifier: 44, Title: "unreadable request", Author: "carol", SourceBranch: "feat/three",
			UpdatedAt: time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)},
	}
	return requestFeedbackSource{
		noun: "merge request", shortNoun: "MR", marker: "!",
		list: func(context.Context) ([]model.OpenRequest, error) { return requests, nil },
		reviews: func(_ context.Context, id int) (map[string]*model.ReviewResult, error) {
			switch id {
			case 42:
				return map[string]*model.ReviewResult{"rev-42": {
					ReviewID: "rev-42", CreatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
					OverallCorrectness: "patch is correct",
					Findings:           []model.Finding{{ID: "f1", Title: "published finding"}},
				}}, nil
			case failing:
				return nil, errors.New("boom")
			default:
				return map[string]*model.ReviewResult{}, nil
			}
		},
	}
}

func TestProbeRequestsKeepsOnlyRequestsCarryingAReview(t *testing.T) {
	remote := checkoutRemote{name: "origin", host: "gitlab.example.com", repo: "grp/proj", platform: platformGitLab}
	got, err := probeRequests(context.Background(), openRequestsWithReviews(t, 0), remote, "feat/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %+v", got)
	}
	candidate := got[0]
	if candidate.identifier != "!42" || candidate.findings != 1 || candidate.kind() != "GitLab" {
		t.Errorf("candidate = %+v", candidate)
	}
	if candidate.origin != "GitLab MR grp/proj!42" {
		t.Errorf("origin = %q", candidate.origin)
	}
	if !candidate.current {
		t.Error("the request of the checked-out branch was not marked")
	}
	if !candidate.reviewedAt.Equal(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("reviewedAt = %s, want the review's own timestamp", candidate.reviewedAt)
	}
}

// One unreadable request must not hide the rows that were read: the failure is
// reported alongside them, not instead of them.
func TestProbeRequestsReportsFailuresWithoutDroppingRows(t *testing.T) {
	remote := checkoutRemote{name: "origin", host: "gitlab.example.com", repo: "grp/proj", platform: platformGitLab}
	got, err := probeRequests(context.Background(), openRequestsWithReviews(t, 44), remote, "")
	if err == nil || !strings.Contains(err.Error(), "1 of 3 open merge requests") {
		t.Fatalf("error = %v", err)
	}
	if len(got) != 1 || got[0].identifier != "!42" {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestDedupeRemotesKeepsOneEntryPerProject(t *testing.T) {
	remotes := []checkoutRemote{
		{name: "origin", host: "github.com", repo: "owner/repo", platform: platformGitHub},
		{name: "mirror", host: "github.com", repo: "Owner/Repo", platform: platformGitHub},
		{name: "upstream", host: "github.com", repo: "upstream/repo", platform: platformGitHub},
		{name: "internal", host: "gitlab.example.com", repo: "group/project", platform: platformGitLab},
		{name: "backup", host: "git.example.com", repo: "owner/repo", platform: platformNone},
	}
	got := dedupeRemotes(remotes)
	names := make([]string, len(got))
	for i, remote := range got {
		names[i] = remote.name
	}
	want := []string{"origin", "upstream", "internal"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("dedupeRemotes = %v, want %v", names, want)
	}
}

func TestCollectFeedbackMergesLocalAndRemoteNewestFirst(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	savedLocalReview(t, store, repo, "branch", "origin/main", "main", "local finding",
		time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))

	a := &app{sessionDir: sessionDir}
	local, err := a.localFeedback(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	remote := checkoutRemote{name: "origin", host: "gitlab.example.com", repo: "grp/proj", platform: platformGitLab}
	published, err := probeRequests(context.Background(), openRequestsWithReviews(t, 0), remote, "main")
	if err != nil {
		t.Fatal(err)
	}
	candidates := append(local, published...)
	sortFeedback(candidates)
	if len(candidates) != 2 {
		t.Fatalf("candidates = %+v", candidates)
	}
	if candidates[0].kind() != "local" || candidates[1].identifier != "!42" {
		t.Fatalf("merged order = %+v", candidates)
	}
}

func TestRunMergedFeedbackPrintsTheChosenReview(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	savedLocalReview(t, store, repo, "branch", "origin/main", "main", "local finding", time.Now().UTC())

	var offered pick.Options
	var out bytes.Buffer
	a := &app{sessionDir: sessionDir, outputFormat: "raw"}
	a.selectFn = func(opts pick.Options) (int, error) {
		offered = opts
		return 0, nil
	}
	if err := a.runMergedFeedback(context.Background(), &out, mergedFeedbackOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(offered.Items) != 1 {
		t.Fatalf("picker offered %d rows: %+v", len(offered.Items), offered.Items)
	}
	if !strings.Contains(out.String(), "local finding") {
		t.Fatalf("chosen review was not printed:\n%s", out.String())
	}
}

func TestRunMergedFeedbackWithoutReviewsExplainsWhy(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)

	a := &app{sessionDir: t.TempDir(), outputFormat: "raw"}
	err := a.runMergedFeedback(context.Background(), &bytes.Buffer{}, mergedFeedbackOptions{})
	if err == nil || !strings.Contains(err.Error(), "no review found for this checkout") {
		t.Fatalf("error = %v", err)
	}

	// A source that failed is named instead: "nothing found" means something
	// else when a token was rejected than when nothing was ever reviewed.
	problems := []error{errors.New("listing open merge requests of grp/proj: status 401")}
	if got := noFeedbackMessage(problems); !strings.Contains(got, "status 401") {
		t.Errorf("message = %q", got)
	}
}

func TestMergedFeedbackListAsJSON(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	saved := savedLocalReview(t, store, repo, "branch", "origin/main", "main", "local finding", time.Now().UTC())

	var out bytes.Buffer
	a := &app{sessionDir: sessionDir, jsonOutput: true}
	if err := a.runMergedFeedback(context.Background(), &out, mergedFeedbackOptions{list: true}); err != nil {
		t.Fatal(err)
	}
	var entries []feedbackListEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(entries) != 1 || entries[0].Kind != "local" || entries[0].SessionID != saved.ID {
		t.Fatalf("entries = %+v", entries)
	}
}

// Without a terminal the merged list cannot ask, so it names the commands that
// address a review directly rather than choosing one by itself.
func TestRunMergedFeedbackWithoutTerminalNamesTheDirectCommands(t *testing.T) {
	repo := gitRepoOnBranch(t, "main")
	t.Chdir(repo)
	sessionDir := t.TempDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	savedLocalReview(t, store, repo, "branch", "origin/main", "main", "local finding", time.Now().UTC())

	a := &app{sessionDir: sessionDir, outputFormat: "raw"}
	err = a.runMergedFeedback(context.Background(), &bytes.Buffer{}, mergedFeedbackOptions{})
	if err == nil || !strings.Contains(err.Error(), "nickpit git feedback --session") ||
		!strings.Contains(err.Error(), "--list") {
		t.Fatalf("error = %v", err)
	}
}
