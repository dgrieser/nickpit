package main

import (
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

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
