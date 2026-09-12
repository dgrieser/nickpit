package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/session"
)

// fakeRemote builds a finder over a source that answers from memory, so the
// listing can be exercised without a server.
func fakeRemote(requests []model.OpenRequest, reviews map[int]map[string]*model.ReviewResult,
	threads []reviewThread, listErr error) *remoteFinder {
	return &remoteFinder{source: &remoteSource{
		mode: model.ModeGitLab,
		repo: "grp/nickpit",
		list: func(context.Context) ([]model.OpenRequest, error) { return requests, listErr },
		reviews: func(_ context.Context, id int) (map[string]*model.ReviewResult, error) {
			return reviews[id], nil
		},
		threads: func(context.Context, int) ([]reviewThread, error) { return threads, nil },
	}}
}

func publishedReview(id, verdict string, findings int, age time.Duration) *model.ReviewResult {
	result := &model.ReviewResult{ReviewID: id, OverallCorrectness: verdict, CreatedAt: time.Now().Add(-age)}
	for range findings {
		result.Findings = append(result.Findings, model.Finding{Title: "finding of " + id})
	}
	return result
}

func TestRemoteReviewsLeadWithTheCheckedOutBranch(t *testing.T) {
	requests := []model.OpenRequest{
		{Identifier: 7, SourceBranch: "feat/other", Title: "other work", UpdatedAt: time.Now()},
		{Identifier: 42, SourceBranch: "feat/x", Title: "this work", UpdatedAt: time.Now()},
	}
	reviews := map[int]map[string]*model.ReviewResult{
		7:  {"r-other": publishedReview("r-other", "patch is correct", 0, time.Minute)},
		42: {"r-old": publishedReview("r-old", "patch is incorrect", 2, 3*time.Hour), "r-new": publishedReview("r-new", "patch is correct", 1, time.Hour)},
	}
	remote := fakeRemote(requests, reviews, nil, nil)
	rows, err := remote.find(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want one per review", len(rows))
	}
	sortRemoteReviews(rows, "feat/x")
	var order []string
	for _, row := range rows {
		order = append(order, row.info.ID)
	}
	// The checked-out branch first, newest review of it leading, then the rest.
	if order[0] != "r-new" || order[1] != "r-old" || order[2] != "r-other" {
		t.Fatalf("order = %v", order)
	}
	// The rows carry what the columns and the scopes read.
	row := rows[0]
	if row.info.Source.Identifier != 42 || row.info.Source.Branch != "feat/x" ||
		row.info.Verdict != "patch is correct" || row.info.Findings != 1 || !row.info.HasResult {
		t.Fatalf("row = %+v", row.info)
	}
	if got := sessionSourceLabel(sessionPlace{}, row.info, sessionOrigin{}); got != "GitLab MR !42" {
		t.Fatalf("source label = %q", got)
	}
}

func TestSessionViewsCarryRemoteReviews(t *testing.T) {
	place := sessionPlace{
		here:   git.Location{Root: "/src/nickpit", Repo: "/src/nickpit/.git"},
		repo:   "grp/nickpit",
		branch: "feat/x",
	}
	local := []session.Info{{
		ID: "local-1", HasResult: true,
		Source:    session.Source{Mode: "local", RepoRoot: "/src/nickpit", Branch: "feat/x"},
		UpdatedAt: time.Now(),
	}}
	remote := fakeRemote(
		[]model.OpenRequest{
			{Identifier: 42, SourceBranch: "feat/x", UpdatedAt: time.Now()},
			{Identifier: 7, SourceBranch: "feat/other", UpdatedAt: time.Now()},
		},
		map[int]map[string]*model.ReviewResult{
			42: {"r-mine": publishedReview("r-mine", "patch is correct", 1, time.Hour)},
			7:  {"r-theirs": publishedReview("r-theirs", "patch is correct", 0, time.Minute)},
		}, nil, nil)

	views, initial := sessionViews(local, place, remote)
	if initial != scopeRepo {
		t.Fatalf("initial = %d, want the repository scope", initial)
	}
	// The published reviews are their own scope, and nothing is fetched until
	// it is drawn.
	if views[scopeRemote].Label != "remote" || len(views[scopeRemote].Items) != 0 {
		t.Fatalf("remote scope = %+v", views[scopeRemote])
	}
	// The scopes of saved sessions never show them, before or after the fetch.
	for _, scope := range []int{scopeBranch, scopeRepo, scopeAll} {
		if views[scope].Load != nil {
			t.Fatalf("scope %q fetches published reviews", views[scope].Label)
		}
		if len(views[scope].Items) != 1 || views[scope].Items[0].Key != "local-1" {
			t.Fatalf("scope %q = %+v, want the saved session alone", views[scope].Label, views[scope].Items)
		}
	}
	items, err := views[scopeRemote].Load()
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, item := range items {
		keys = append(keys, item.Key)
	}
	// The checked-out branch's request leads; no saved session is mixed in.
	if len(keys) != 2 || keys[0] != "r-mine" || keys[1] != "r-theirs" {
		t.Fatalf("remote scope = %v", keys)
	}
	// A row is resolved back to the review it stands for, not to a session.
	choice := sessionOfRow(pick.View{Items: items}, local, remote, 0)
	if choice.saved() || choice.remote == nil || choice.remote.id != 42 {
		t.Fatalf("choice = %+v", choice)
	}
	saved := sessionOfRow(views[scopeRepo], local, remote, 0)
	if !saved.saved() || saved.info.ID != "local-1" {
		t.Fatalf("choice = %+v, want the saved session", saved)
	}
}

func TestRemoteListingFailureLeavesTheRestAlone(t *testing.T) {
	remote := fakeRemote(nil, nil, nil, errors.New("401 unauthorized"))
	if _, err := remote.find(context.Background()); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want the listing failure", err)
	}
	// The failure is remembered rather than retried on every scope switch.
	remote.source.list = func(context.Context) ([]model.OpenRequest, error) {
		t.Fatal("the listing was retried")
		return nil, nil
	}
	if _, err := remote.find(context.Background()); err == nil {
		t.Fatal("the remembered failure was dropped")
	}
	// A picker still opens: only the remote scope reports the failure, where
	// its rows would have been.
	views, _ := sessionViews(nil, sessionPlace{repo: "grp/nickpit", branch: "feat/x"}, remote)
	if _, err := views[scopeRemote].Load(); err == nil {
		t.Fatal("the scope hid the failure")
	}
	for _, scope := range []int{scopeBranch, scopeRepo, scopeAll} {
		if views[scope].Load != nil {
			t.Fatalf("scope %q was dragged into the failure", views[scope].Label)
		}
	}
}

func TestRemoteReviewCarriesTheRepliesToIt(t *testing.T) {
	result := publishedReview("r", "patch is correct", 2, time.Hour)
	result.OverallExplanation = "looks fine"
	result.Findings[0].ID, result.Findings[0].Body = "f-1", "the lock is taken twice"
	result.Findings[1].ID, result.Findings[1].Body = "f-2", "unused import"
	when := time.Date(2026, 9, 8, 14, 12, 0, 0, time.UTC)
	threads := []reviewThread{
		{reviewID: "r", replies: []model.Reply{{Author: "Alice Adams", Body: "thanks", CreatedAt: when}}},
		{reviewID: "r", findingID: "f-1", replies: []model.Reply{
			{Author: "Bob Brown", Body: "fixed in 3a1c2f", CreatedAt: when},
			{Author: "NickPit Bot", Body: "agreed", CreatedAt: when},
		}},
		// Another review's threads add nothing.
		{reviewID: "other", findingID: "f-2", replies: []model.Reply{{Author: "Carol", Body: "not ours"}}},
	}

	withReplies, err := reviewWithReplies(result, threads)
	if err != nil {
		t.Fatal(err)
	}
	if len(withReplies.Replies) != 1 || len(withReplies.Findings[0].Replies) != 2 ||
		len(withReplies.Findings[1].Replies) != 0 {
		t.Fatalf("review = %+v", withReplies)
	}
	// The published review the row points at is left as it was.
	if len(result.Replies) != 0 || len(result.Findings[0].Replies) != 0 {
		t.Fatalf("the published review was rewritten: %+v", result)
	}

	// One document, in whichever form the run asked for. The markdown one
	// writes each answer as a message of its own, after the finding and its
	// suggestions.
	remote := fakeRemote(nil, nil, threads, nil)
	row := remoteReview{mode: model.ModeGitLab, repo: "grp/nickpit", id: 42, result: result}
	var out bytes.Buffer
	if err := (&app{outputFormat: "raw"}).writeRemoteReview(context.Background(), remote.source, row, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "the lock is taken twice") {
		t.Fatalf("markdown lacks the review:\n%s", text)
	}
	for _, want := range []string{"> **Bob Brown** · ", "> fixed in 3a1c2f", "> **NickPit Bot** · "} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "- **Bob Brown**") {
		t.Fatalf("replies still read as a bullet list:\n%s", text)
	}
	out.Reset()
	if err := (&app{jsonOutput: true}).writeRemoteReview(context.Background(), remote.source, row, &out); err != nil {
		t.Fatal(err)
	}
	var decoded model.ReviewResult
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(decoded.Replies) != 1 || len(decoded.Findings[0].Replies) != 2 ||
		decoded.Findings[0].Replies[0].Author != "Bob Brown" {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestGitLabReviewThreadsKeepOnlyAnswersToNickpit(t *testing.T) {
	render := reviewmd.NewRenderer("")
	root := func(reviewID, findingID string) string {
		if findingID == "" {
			// The summary note as the publisher writes it, marker and all.
			body, ok := render.SummaryBodyCarried(&model.ReviewResult{
				ReviewID: reviewID, OverallCorrectness: "patch is correct",
			})
			if !ok {
				t.Fatal("the summary marker did not fit")
			}
			return body
		}
		return "the finding text" + reviewmd.FindingMarker(reviewID, model.Finding{ID: findingID})
	}
	discussions := []glscm.MRDiscussion{
		{ID: "d1", Notes: []glscm.DiscussionNote{
			{Body: root("r", ""), AuthorName: "nickpit"},
			{Body: "thanks", AuthorName: "alice", AuthorDisplay: "Alice Adams"},
			{Body: "", AuthorName: "bob"},
			{Body: "a branch was pushed", AuthorName: "gitlab", System: true},
		}},
		{ID: "d2", Notes: []glscm.DiscussionNote{
			{Body: root("r", "f-1"), AuthorName: "group_20_bot_ba1ace7", AuthorDisplay: "****", AuthorID: 20},
			// No display name on record: the account name stands in.
			{Body: "fixed", AuthorName: "bob"},
			// The account that opened the thread answering in it is NickPit —
			// its display name is masked and its username is a hash.
			{Body: "confirmed", AuthorName: "group_20_bot_ba1ace7", AuthorDisplay: "****", AuthorID: 20},
		}},
		// Somebody else's thread, and one of NickPit's that nobody answered.
		{ID: "d3", Notes: []glscm.DiscussionNote{
			{Body: "unrelated question", AuthorName: "carol"},
			{Body: "unrelated answer", AuthorName: "dave"},
		}},
		{ID: "d4", Notes: []glscm.DiscussionNote{{Body: root("r", "f-2"), AuthorName: "nickpit"}}},
	}
	threads := gitlabReviewThreads(discussions)
	if len(threads) != 2 {
		t.Fatalf("threads = %+v, want the two that were answered", threads)
	}
	// A person is read by their display name, not by an account name — which
	// for a bot is a hash nobody recognizes.
	if threads[0].findingID != "" || len(threads[0].replies) != 1 ||
		threads[0].replies[0].Author != "Alice Adams" {
		t.Fatalf("summary thread = %+v", threads[0])
	}
	if threads[1].findingID != "f-1" || len(threads[1].replies) != 2 ||
		threads[1].replies[0].Body != "fixed" || threads[1].replies[0].Author != "bob" ||
		threads[1].replies[1].Author != "NickPit" {
		t.Fatalf("finding thread = %+v", threads[1])
	}
}

func TestRemoteHostDetection(t *testing.T) {
	cases := []struct {
		remote string
		github bool
	}{
		{"git@github.com:owner/repo.git", true},
		{"https://github.com/owner/repo.git", true},
		{"git@gitlab.example.com:grp/proj.git", false},
		{"https://gitlab.example.com/grp/proj.git", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isGitHubRemote(c.remote); got != c.github {
			t.Fatalf("isGitHubRemote(%q) = %v", c.remote, got)
		}
	}
	// A token only ever goes to the host the remote names; an unreadable remote
	// leaves the profile's own host in charge.
	if !sameHost("git@gitlab.example.com:grp/proj.git", "https://gitlab.example.com/api/v4") {
		t.Fatal("the project's own host was rejected")
	}
	if sameHost("git@gitlab.other.com:grp/proj.git", "https://gitlab.example.com/api/v4") {
		t.Fatal("a token would have gone to a foreign host")
	}
	if !sameHost("", "https://gitlab.example.com/api/v4") {
		t.Fatal("an unknown remote must not disable the configured host")
	}
}

func TestClosedWidensTheRemoteListing(t *testing.T) {
	place := sessionPlace{repo: "grp/nickpit", remoteURL: "git@gitlab.example.com:grp/nickpit.git", branch: "feat/x"}
	a := &app{}
	// Without credentials there is nothing to ask, whichever flag is set — the
	// scope says so instead of failing.
	if source := a.remoteSourceFor(place, true); source != nil {
		t.Fatalf("source = %+v, want none without a token", source)
	}

	// The flag reaches the listing and the scope's own wording.
	open := &remoteFinder{source: &remoteSource{mode: model.ModeGitLab, repo: "grp/nickpit"}}
	closed := &remoteFinder{source: &remoteSource{mode: model.ModeGitLab, repo: "grp/nickpit", closed: true}}
	if got := open.scopeNoun(); got != "the open merge requests of the project" {
		t.Fatalf("scope noun = %q", got)
	}
	if got := closed.scopeNoun(); got != "the merge requests of the project" {
		t.Fatalf("scope noun = %q", got)
	}
	openView, closedView := remoteView(place, open), remoteView(place, closed)
	if !strings.Contains(openView.Empty, "open merge requests") {
		t.Fatalf("empty line = %q", openView.Empty)
	}
	if strings.Contains(closedView.Empty, "open merge requests") {
		t.Fatalf("empty line = %q, want the widened wording", closedView.Empty)
	}
}
