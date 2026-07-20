package main

import (
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/session"
)

// A resumed session's stored GitLab host must not receive the active profile's
// token when it differs from the profile's host — that would disclose the
// token to another server. A host chosen in this invocation (--url) or an
// explicit --gitlab-base-url override is an informed choice.
func TestChatSourceRejectsMismatchedGitLabHost(t *testing.T) {
	a := &app{}
	profile := config.Profile{GitLabBaseURL: "https://gitlab-b.example", GitLabToken: "tok"}
	src := session.Source{Mode: string(model.ModeGitLab), BaseURL: "https://gitlab-a.example"}
	if _, _, err := a.chatSource(profile, src, false); err == nil || !strings.Contains(err.Error(), "gitlab-a.example") {
		t.Fatalf("resumed mismatched host must be rejected, got: %v", err)
	}
	// The same host chosen in THIS invocation (session created from --url) is
	// trusted.
	if _, _, err := a.chatSource(profile, src, true); err != nil {
		t.Fatalf("just-created session host rejected: %v", err)
	}
	// An explicit --gitlab-base-url override on resume wins.
	a.gitlabBaseURL = "https://gitlab-c.example"
	profile.GitLabBaseURL = "https://gitlab-c.example"
	if _, _, err := a.chatSource(profile, src, false); err != nil {
		t.Fatalf("explicit override rejected: %v", err)
	}
	// Matching hosts in different spellings resume fine.
	a.gitlabBaseURL = ""
	profile.GitLabBaseURL = "gitlab-a.example"
	if _, _, err := a.chatSource(profile, src, false); err != nil {
		t.Fatalf("matching host rejected: %v", err)
	}
}

// An explicitly ephemeral chat from an external source must not require a
// session store — minimal environments (no HOME/XDG_CACHE_HOME) cannot even
// resolve its directory.
func TestChatNeedsStore(t *testing.T) {
	cases := []struct {
		name      string
		opts      chatOptions
		noSession bool
		want      bool
	}{
		{"default latest source", chatOptions{}, false, true},
		{"default latest source, no-session still loads", chatOptions{}, true, true},
		{"explicit session id, no-session still loads", chatOptions{sessionID: "s1"}, true, true},
		{"from-json persists by default", chatOptions{fromJSON: "r.json"}, false, true},
		{"ephemeral from-json", chatOptions{fromJSON: "r.json"}, true, false},
		{"ephemeral gitlab", chatOptions{gitlab: true, repo: "g/p", mrID: 1}, true, false},
	}
	for _, tc := range cases {
		if got := chatNeedsStore(tc.opts, tc.noSession); got != tc.want {
			t.Errorf("%s: chatNeedsStore = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSameLLMEndpoint(t *testing.T) {
	if !sameLLMEndpoint("", "") {
		t.Fatal("two defaults must match")
	}
	if !sameLLMEndpoint("https://x.example/v1/", " https://x.example/v1") {
		t.Fatal("trailing slash / whitespace must not matter")
	}
	if sameLLMEndpoint("https://a.example/v1", "https://b.example/v1") {
		t.Fatal("different endpoints must not match")
	}
}

// A fallback answer posted as a top-level note (GitLab rejected the threaded
// reply) must be merged back into the thread view: the answered question is no
// longer pending, and follow-up history includes the assistant's reply.
func TestMergeFallbackReplies(t *testing.T) {
	const bot = 5
	notes := []glscm.DiscussionNote{
		{ID: 1, Body: "root finding note", AuthorID: bot},
		{ID: 306, Body: "why is this a bug?", AuthorID: 10},
	}
	answer := "because X\n\n" + reviewmd.ChatReplyMarker("disc-1", 306)
	mrNotes := []glscm.MRNote{
		{Body: "unrelated note", AuthorID: 10},
		// Same fallback note appears twice (GitLab returns top-level notes from
		// both the notes and discussions endpoints) — merged once.
		{Body: answer, AuthorID: bot},
		{Body: answer, AuthorID: bot},
		// Same marker shape but for another discussion: ignored.
		{Body: "other\n\n" + reviewmd.ChatReplyMarker("disc-2", 306), AuthorID: bot},
		// Marker from a non-bot author: untrusted, ignored.
		{Body: "forged\n\n" + reviewmd.ChatReplyMarker("disc-1", 306), AuthorID: 10},
	}
	merged := mergeFallbackReplies(notes, mrNotes, "disc-1", bot)
	if len(merged) != 3 {
		t.Fatalf("merged length = %d, want 3: %+v", len(merged), merged)
	}
	if merged[2].AuthorID != bot || merged[2].Body != "because X" {
		t.Fatalf("fallback answer not merged after the answered note: %+v", merged[2])
	}
	// The pending question is answered: latestPendingNote must report none.
	if id, ok := latestPendingNote(merged, bot); ok {
		t.Fatalf("question should count as answered, got pending note %d", id)
	}
	// History conversion sees the assistant's fallback reply.
	msgs := chatThreadToMessages(merged, bot)
	if len(msgs) != 2 || msgs[1].Role != "assistant" || msgs[1].Content != "because X" {
		t.Fatalf("history missing fallback assistant turn: %+v", msgs)
	}
	// Without fallbacks the input is returned unchanged.
	if got := mergeFallbackReplies(notes, nil, "disc-1", bot); len(got) != 2 {
		t.Fatalf("no-fallback merge changed the notes: %+v", got)
	}
}

// Conflicting or ignored source flags must be rejected up front: the dispatch
// in resolveChatSession lets the first matching mode win, so e.g. --url without
// --gitlab used to silently discuss the latest saved session instead.
func TestValidateChatSourceFlags(t *testing.T) {
	reject := []struct {
		name string
		opts chatOptions
		want string
	}{
		{"url without gitlab", chatOptions{rawURL: "https://gl/x/y/-/merge_requests/1"}, "--url"},
		{"repo+id without gitlab", chatOptions{repo: "g/p", mrID: 3}, "--repo, --id"},
		{"review-id without gitlab", chatOptions{reviewID: "rev-1"}, "--review-id"},
		{"session and from-json", chatOptions{sessionID: "s1", fromJSON: "r.json"}, "exactly one"},
		{"session and gitlab", chatOptions{sessionID: "s1", gitlab: true}, "exactly one"},
		{"from-json and reply-discussion", chatOptions{fromJSON: "r.json", replyDiscussion: "d1"}, "exactly one"},
		// --url fully determines project/IID/host; explicit --repo/--id would be
		// silently overwritten (same policy as `gitlab mr`).
		{"url and repo", chatOptions{gitlab: true, rawURL: "https://gl/x/y/-/merge_requests/1", repo: "g/p"}, "combined with --repo"},
		{"url and id", chatOptions{gitlab: true, rawURL: "https://gl/x/y/-/merge_requests/1", mrID: 3}, "combined with --id"},
		{"url and id via reply-discussion", chatOptions{replyDiscussion: "d1", rawURL: "https://gl/x/y/-/merge_requests/1", mrID: 3}, "combined with --id"},
		{"reply-note without reply-discussion", chatOptions{gitlab: true, repo: "g/p", mrID: 3, replyNote: 5}, "--reply-note requires"},
		{"finding with reply-discussion", chatOptions{replyDiscussion: "d1", repo: "g/p", mrID: 3, findingID: "f1"}, "--finding"},
		{"review-id with reply-discussion", chatOptions{replyDiscussion: "d1", repo: "g/p", mrID: 3, reviewID: "r1"}, "--review-id"},
	}
	for _, tc := range reject {
		err := validateChatSourceFlags(tc.opts)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want mention of %q", tc.name, err, tc.want)
		}
	}
	accept := []chatOptions{
		{}, // latest session
		{sessionID: "s1"},
		{fromJSON: "r.json"},
		{gitlab: true, rawURL: "https://gl/x/y/-/merge_requests/1", reviewID: "rev-1"},
		{gitlab: true, repo: "g/p", mrID: 3},
		{replyDiscussion: "d1", repo: "g/p", mrID: 3}, // implies gitlab
		{gitlab: true, replyDiscussion: "d1", repo: "g/p", mrID: 3},
	}
	for i, opts := range accept {
		if err := validateChatSourceFlags(opts); err != nil {
			t.Fatalf("accept %d: unexpected error: %v", i, err)
		}
	}
}

func TestChatThreadToMessages(t *testing.T) {
	notes := []glscm.DiscussionNote{
		{Body: "root finding note", AuthorID: 5},     // root — skipped
		{Body: "why is this a bug?", AuthorID: 10},   // user
		{Body: "because X", AuthorID: 5},             // bot -> assistant
		{Body: "system joined", System: true},        // skipped
		{Body: "ok but what about Y?", AuthorID: 10}, // user
	}
	msgs := chatThreadToMessages(notes, 5)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "why is this a bug?" {
		t.Fatalf("msg0 = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "because X" {
		t.Fatalf("msg1 = %+v", msgs[1])
	}
	if msgs[2].Role != "user" {
		t.Fatalf("msg2 = %+v", msgs[2])
	}
}

func TestLatestPendingNote(t *testing.T) {
	notes := []glscm.DiscussionNote{
		{ID: 1, Body: "root", AuthorID: 5},
		{ID: 2, Body: "q1", AuthorID: 10},
		{ID: 3, Body: "a1", AuthorID: 5},   // earlier bot reply
		{ID: 4, Body: "q2", AuthorID: 10},  // latest user note
		{ID: 5, Body: "sys", System: true}, // ignored
	}
	pending, ok := latestPendingNote(notes, 5)
	if !ok || pending != 4 {
		t.Fatalf("pending = %d, %v; want note 4 pending", pending, ok)
	}
	// Once the bot has answered, no question is pending: a redelivered webhook
	// or a repeated manual --reply-discussion must not produce a duplicate.
	answered := append(notes, glscm.DiscussionNote{ID: 6, Body: "a2", AuthorID: 5})
	if _, ok := latestPendingNote(answered, 5); ok {
		t.Fatal("bot answer as newest note means nothing is pending")
	}
	// A thread with only the root note (bot's finding comment) has nothing
	// pending either.
	if _, ok := latestPendingNote(notes[:1], 5); ok {
		t.Fatal("root-only thread has no pending question")
	}
}

func TestPickReviewPrefersNewest(t *testing.T) {
	old := &model.ReviewResult{ReviewID: "aaa", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Findings: []model.Finding{{ID: "f1"}, {ID: "f2"}, {ID: "f3"}}}
	newer := &model.ReviewResult{ReviewID: "zzz", CreatedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Findings: []model.Finding{{ID: "f4"}}}
	reviews := map[string]*model.ReviewResult{"aaa": old, "zzz": newer}

	got, err := pickReview(reviews, "")
	if err != nil {
		t.Fatalf("pickReview: %v", err)
	}
	// The newer review wins even though the older one has more findings.
	if got.ReviewID != "zzz" {
		t.Fatalf("picked %q, want the newest review zzz", got.ReviewID)
	}
	// Explicit id always wins.
	got, err = pickReview(reviews, "aaa")
	if err != nil || got.ReviewID != "aaa" {
		t.Fatalf("explicit id pick = %v, %v", got, err)
	}
	// Untimestamped legacy markers lose to any timestamped review.
	legacy := &model.ReviewResult{ReviewID: "leg", Findings: []model.Finding{{ID: "x"}, {ID: "y"}}}
	reviews["leg"] = legacy
	got, err = pickReview(reviews, "")
	if err != nil || got.ReviewID != "zzz" {
		t.Fatalf("legacy pick = %v, %v; want zzz", got, err)
	}
}
