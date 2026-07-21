package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/review"
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

// fakeRemoteSource is a model.RemoteCheckoutSource for chat checkout tests.
type fakeRemoteSource struct {
	spec          *model.CheckoutSpec
	resolveErr    error
	checkoutCalls int
}

func (s *fakeRemoteSource) ResolveContext(_ context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode:         req.Mode,
		Title:        "t",
		ChangedFiles: []model.ChangedFile{{Path: "a.go"}},
		Diff:         "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\n",
	}, nil
}

func (s *fakeRemoteSource) ResolveCheckout(context.Context, model.ReviewRequest) (*model.CheckoutSpec, error) {
	s.checkoutCalls++
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return s.spec, nil
}

// fakeContextOnlySource implements only model.ReviewSource (no checkout support).
type fakeContextOnlySource struct{}

func (fakeContextOnlySource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{}, nil
}

func TestChatCheckoutRelease(t *testing.T) {
	var zero chatCheckout
	zero.release() // zero value must not panic

	calls := 0
	co := chatCheckout{root: "/tmp/x", cleanup: func() { calls++ }}
	co.release()
	co.release()
	if calls != 1 {
		t.Fatalf("cleanup ran %d times, want exactly once", calls)
	}
	if co.root != "" || co.cleanup != nil {
		t.Fatalf("release must clear the holder, got root=%q cleanup set=%t", co.root, co.cleanup != nil)
	}
}

func TestChatEnsureCheckout(t *testing.T) {
	ctx := context.Background()
	profile := config.Profile{Workdir: "/work", GitLabToken: "tok"}
	req := model.ReviewRequest{Mode: model.ModeGitLab}
	spec := &model.CheckoutSpec{Provider: model.ModeGitLab, Repo: "g/p", CloneURL: "https://x/y.git", HeadSHA: "abc"}

	t.Run("source without checkout support is a no-op", func(t *testing.T) {
		a := &app{prepareCheckout: func(context.Context, model.CheckoutSpec, git.CheckoutOptions) (string, func(), error) {
			t.Fatal("prepare must not be called for a non-remote source")
			return "", nil, nil
		}}
		var co chatCheckout
		if err := a.chatEnsureCheckout(ctx, fakeContextOnlySource{}, profile, req, &co); err != nil {
			t.Fatalf("chatEnsureCheckout: %v", err)
		}
		if co.root != "" {
			t.Fatalf("co.root = %q, want empty", co.root)
		}
	})

	t.Run("prepares once with profile workdir and token, then reuses", func(t *testing.T) {
		src := &fakeRemoteSource{spec: spec}
		prepares := 0
		a := &app{prepareCheckout: func(_ context.Context, gotSpec model.CheckoutSpec, opts git.CheckoutOptions) (string, func(), error) {
			prepares++
			if gotSpec.CloneURL != spec.CloneURL {
				t.Fatalf("spec.CloneURL = %q, want %q", gotSpec.CloneURL, spec.CloneURL)
			}
			if opts.Workdir != "/work" || opts.Token != "tok" {
				t.Fatalf("opts = %+v, want profile workdir and GitLab token", opts)
			}
			return "/tmp/clone", func() {}, nil
		}}
		var co chatCheckout
		if err := a.chatEnsureCheckout(ctx, src, profile, req, &co); err != nil {
			t.Fatalf("chatEnsureCheckout: %v", err)
		}
		if co.root != "/tmp/clone" {
			t.Fatalf("co.root = %q, want /tmp/clone", co.root)
		}
		// A held checkout is reused: no second resolve or prepare.
		if err := a.chatEnsureCheckout(ctx, src, profile, req, &co); err != nil {
			t.Fatalf("chatEnsureCheckout (again): %v", err)
		}
		if prepares != 1 || src.checkoutCalls != 1 {
			t.Fatalf("prepare/resolve ran %d/%d times, want 1/1", prepares, src.checkoutCalls)
		}
	})

	t.Run("resolve and prepare errors propagate, holder stays empty", func(t *testing.T) {
		src := &fakeRemoteSource{resolveErr: errors.New("boom")}
		a := &app{}
		var co chatCheckout
		if err := a.chatEnsureCheckout(ctx, src, profile, req, &co); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("resolve error not propagated, got: %v", err)
		}
		if co.root != "" {
			t.Fatalf("co.root = %q after resolve failure, want empty", co.root)
		}
		a.prepareCheckout = func(context.Context, model.CheckoutSpec, git.CheckoutOptions) (string, func(), error) {
			return "", nil, errors.New("clone failed")
		}
		if err := a.chatEnsureCheckout(ctx, &fakeRemoteSource{spec: spec}, profile, req, &co); err == nil || !strings.Contains(err.Error(), "clone failed") {
			t.Fatalf("prepare error not propagated, got: %v", err)
		}
		if co.root != "" {
			t.Fatalf("co.root = %q after prepare failure, want empty", co.root)
		}
	})
}

// chatPrepareContext must retain the checkout in the holder for the retrieval
// tools (single shared clone per invocation) while scrubbing its path from the
// returned context, which may be persisted in the session store.
func TestChatPrepareContextSharedCheckout(t *testing.T) {
	ctx := context.Background()
	spec := &model.CheckoutSpec{Provider: model.ModeGitLab, Repo: "g/p", CloneURL: "https://x/y.git", HeadSHA: "abc"}
	src := &fakeRemoteSource{spec: spec}
	root := t.TempDir()
	prepares, cleanups := 0, 0
	a := &app{prepareCheckout: func(context.Context, model.CheckoutSpec, git.CheckoutOptions) (string, func(), error) {
		prepares++
		return root, func() { cleanups++ }, nil
	}}
	engine := review.NewEngine(src, nil, retrieval.NewLocalEngine(), config.Profile{})
	engine.SetToolchainCapture(func(context.Context, string, *model.ReviewContext) []model.ToolchainVersion { return nil })

	profile := config.Profile{} // MaxToolCalls 0 = unlimited: tools enabled
	req := model.ReviewRequest{Mode: model.ModeGitLab, Repo: "g/p", Identifier: 1}
	var co chatCheckout
	reviewCtx, err := a.chatPrepareContext(ctx, engine, src, profile, req, &co)
	if err != nil {
		t.Fatalf("chatPrepareContext: %v", err)
	}
	if co.root != root {
		t.Fatalf("co.root = %q, want the prepared checkout %q", co.root, root)
	}
	if cleanups != 0 {
		t.Fatal("checkout must be retained for the caller, not cleaned up")
	}
	if reviewCtx.CheckoutRoot != "" {
		t.Fatalf("CheckoutRoot = %q leaked into the (persistable) context", reviewCtx.CheckoutRoot)
	}
	// The tools path reuses the same clone instead of preparing a second one.
	if err := a.chatEnsureCheckout(ctx, src, profile, req, &co); err != nil {
		t.Fatalf("chatEnsureCheckout: %v", err)
	}
	if prepares != 1 {
		t.Fatalf("prepare ran %d times, want a single shared checkout", prepares)
	}
	co.release()
	if cleanups != 1 {
		t.Fatalf("cleanup ran %d times after release, want 1", cleanups)
	}
}

// With tools disabled and nothing else needing files, chatPrepareContext must
// not prepare a checkout — the tools-off escape hatch (max_tool_calls < 0)
// also skips the clone.
func TestChatPrepareContextSkipsCheckoutWhenNothingNeedsOne(t *testing.T) {
	src := &fakeRemoteSource{spec: &model.CheckoutSpec{CloneURL: "https://x/y.git", HeadSHA: "abc"}}
	a := &app{prepareCheckout: func(context.Context, model.CheckoutSpec, git.CheckoutOptions) (string, func(), error) {
		t.Fatal("prepare must not be called when nothing needs a checkout")
		return "", nil, nil
	}}
	engine := review.NewEngine(src, nil, retrieval.NewLocalEngine(), config.Profile{})
	engine.SetToolchainCapture(func(context.Context, string, *model.ReviewContext) []model.ToolchainVersion { return nil })

	profile := config.Profile{MaxToolCalls: -1}
	req := model.ReviewRequest{Mode: model.ModeGitLab, Repo: "g/p", Identifier: 1}
	var co chatCheckout
	reviewCtx, err := a.chatPrepareContext(context.Background(), engine, src, profile, req, &co)
	if err != nil {
		t.Fatalf("chatPrepareContext: %v", err)
	}
	if co.root != "" {
		t.Fatalf("co.root = %q, want no checkout", co.root)
	}
	if src.checkoutCalls != 0 {
		t.Fatalf("ResolveCheckout ran %d times, want 0", src.checkoutCalls)
	}
	if reviewCtx == nil {
		t.Fatal("context must still be prepared without a checkout")
	}
}

// fakeNoteClient is a chatNoteClient that returns canned notes and captures the
// bodies posted to it. replyErr/noteErr are returned (and cleared) per call so
// the same fake can drive a threaded-post-then-fallback sequence.
type fakeNoteClient struct {
	discussionNotes []glscm.DiscussionNote
	mrNotes         []glscm.MRNote
	replyErr        error
	noteErr         error
	replyBody       string
	noteBody        string
}

func (f *fakeNoteClient) DiscussionNotes(context.Context, string, int, string) ([]glscm.DiscussionNote, error) {
	return f.discussionNotes, nil
}

func (f *fakeNoteClient) MRNotes(context.Context, string, int) ([]glscm.MRNote, error) {
	return f.mrNotes, nil
}

func (f *fakeNoteClient) ReplyToMRDiscussionPath(_ context.Context, _ string, _ int, _ string, body string) error {
	f.replyBody = body
	err := f.replyErr
	f.replyErr = nil
	return err
}

func (f *fakeNoteClient) CreateMRNotePath(_ context.Context, _ string, _ int, body string) error {
	f.noteBody = body
	err := f.noteErr
	f.noteErr = nil
	return err
}

func TestChatFailureReplyBody(t *testing.T) {
	err := errors.New("clone failed: dial tcp: connection refused")
	body := chatFailureReplyBody(err)

	if !strings.Contains(body, "could not answer this question") {
		t.Errorf("missing general failure message: %q", body)
	}
	if !strings.Contains(body, "Please ask again") {
		t.Errorf("missing re-ask guidance: %q", body)
	}
	if !strings.Contains(body, "clone failed: dial tcp: connection refused") {
		t.Errorf("missing underlying error text: %q", body)
	}
	if !strings.Contains(body, "<details>") || !strings.Contains(body, "</details>") {
		t.Errorf("error not wrapped in a <details> block: %q", body)
	}
	if !strings.Contains(body, "<summary>") {
		t.Errorf("missing <summary> for the collapsed block: %q", body)
	}
	// The error must live inside a fenced code block so it renders literally.
	if !strings.Contains(body, "```\nclone failed: dial tcp: connection refused\n```") {
		t.Errorf("error not inside a code fence: %q", body)
	}
}

func TestPostChatReply(t *testing.T) {
	const (
		botUserID = 5
		pending   = 10
	)
	// Latest non-system note is a user question (id 10) → still pending.
	userPending := []glscm.DiscussionNote{{ID: 10, AuthorID: 7}}
	// Latest non-system note is the bot's own → already answered (superseded).
	answered := []glscm.DiscussionNote{
		{ID: 10, AuthorID: 7},
		{ID: 11, AuthorID: botUserID},
	}

	t.Run("posts threaded reply and escapes quick actions", func(t *testing.T) {
		c := &fakeNoteClient{discussionNotes: userPending}
		a := &app{}
		if err := a.postChatReply(context.Background(), c, "g/p", 1, "d1", pending, botUserID, "ok\n/merge\n/end"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.replyBody == "" {
			t.Fatal("threaded reply not posted")
		}
		if !strings.Contains(c.replyBody, `\`+"/merge") {
			t.Errorf("quick action /merge not escaped: %q", c.replyBody)
		}
		if c.noteBody != "" {
			t.Errorf("plain note should not be posted when the threaded reply succeeds: %q", c.noteBody)
		}
	})

	t.Run("falls back to a plain note carrying the marker on 4xx", func(t *testing.T) {
		c := &fakeNoteClient{
			discussionNotes: userPending,
			replyErr:        &glscm.APIError{Status: 422},
		}
		a := &app{}
		if err := a.postChatReply(context.Background(), c, "g/p", 1, "d1", pending, botUserID, "answer"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.noteBody == "" {
			t.Fatal("fallback plain note not posted")
		}
		if !strings.Contains(c.noteBody, "answer") {
			t.Errorf("fallback note missing the body: %q", c.noteBody)
		}
		if marker := reviewmd.ChatReplyMarker("d1", pending); marker != "" && !strings.Contains(c.noteBody, marker) {
			t.Errorf("fallback note missing ChatReplyEnvelope marker %q: %q", marker, c.noteBody)
		}
	})

	t.Run("returns 5xx error without fallback", func(t *testing.T) {
		c := &fakeNoteClient{
			discussionNotes: userPending,
			replyErr:        &glscm.APIError{Status: 503},
		}
		a := &app{}
		err := a.postChatReply(context.Background(), c, "g/p", 1, "d1", pending, botUserID, "answer")
		if err == nil {
			t.Fatal("expected error on 5xx, got nil")
		}
		if c.noteBody != "" {
			t.Errorf("no fallback note expected on 5xx: %q", c.noteBody)
		}
	})

	t.Run("posts nothing when superseded", func(t *testing.T) {
		c := &fakeNoteClient{discussionNotes: answered}
		a := &app{}
		if err := a.postChatReply(context.Background(), c, "g/p", 1, "d1", pending, botUserID, "answer"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.replyBody != "" || c.noteBody != "" {
			t.Errorf("no reply expected when a newer note superseded the pending one: reply=%q note=%q", c.replyBody, c.noteBody)
		}
	})
}
