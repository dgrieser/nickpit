package serve

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/testutil"
)

type handlerEnv struct {
	handler    *Handler
	dispatcher *Dispatcher
	lookup     *countingTopicLookup
	gitlab     *fakeGitLab
	group      *Group
	chat       *fakeChatRunner
}

// fakeChatRunner records the chat children the handler would spawn. Calls arrive
// on a channel so a test can wait for the handler's fire-and-forget goroutine.
// exitCodes, when non-empty, is consumed one per call (last value repeats).
type fakeChatRunner struct {
	calls     chan ChatSpec
	mu        sync.Mutex
	exitCodes []int
}

func newFakeChatRunner() *fakeChatRunner {
	return &fakeChatRunner{calls: make(chan ChatSpec, 8)}
}

func (r *fakeChatRunner) RunChat(_ context.Context, spec ChatSpec) (int, string, error) {
	r.mu.Lock()
	exit := 0
	if len(r.exitCodes) > 0 {
		exit = r.exitCodes[0]
		if len(r.exitCodes) > 1 {
			r.exitCodes = r.exitCodes[1:]
		}
	}
	r.mu.Unlock()
	r.calls <- spec
	return exit, "chat.log", nil
}

// newHandlerEnv wires a handler to a dispatcher with no started workers, so
// enqueued jobs stay observable and nothing runs. The groups' API client
// points at the fake GitLab so command acknowledgements and replies are
// observable too.
func newHandlerEnv(t *testing.T) *handlerEnv {
	t.Helper()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	set, _ := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t1", WebhookSecret: "hook-secret"},
		{Path: "platform/legacy", Token: "t2", WebhookSecret: "legacy-secret"},
	}, server.URL, nil)
	lookup := &countingTopicLookup{}
	dispatcher := NewDispatcher(&fakeRunner{}, lookup.fn(), WorkerConfig{Topic: "nickpit"}, discardLogger())
	chat := newFakeChatRunner()
	handler := NewHandler(set, dispatcher, HandlerConfig{
		TriggerEmoji:   "nickpit",
		CommandKeyword: "nickpit",
		AckEmoji:       "white_check_mark",
		AbortEmoji:     "stop_button",
	}, chat, ChatConfig{ConfigPath: "cfg.yaml", BaseURL: "https://gl.example"}, discardLogger())
	return &handlerEnv{
		handler:    handler,
		dispatcher: dispatcher,
		lookup:     lookup,
		gitlab:     fake,
		group:      set.Match("platform/legacy/tool"),
		chat:       chat,
	}
}

type countingTopicLookup struct{ calls int }

func (c *countingTopicLookup) fn() TopicLookup {
	return func(ctx context.Context, group *Group, projectID int) ([]string, error) {
		c.calls++
		return []string{"nickpit"}, nil
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func postWebhook(t *testing.T, handler *Handler, fixture, secret string) *httptest.ResponseRecorder {
	t.Helper()
	body := testutil.LoadFixture(t, filepath.Join("testdata", fixture))
	return postWebhookBody(t, handler, body, secret)
}

func postWebhookBody(t *testing.T, handler *Handler, body []byte, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", bytes.NewReader(body))
	if secret != "" {
		req.Header.Set("X-Gitlab-Token", secret)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func queuedJobs(d *Dispatcher) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.states)
}

func TestHandlerQueuesTrigger(t *testing.T) {
	env := newHandlerEnv(t)
	recorder := postWebhook(t, env.handler, "mr_open.json", "hook-secret")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"queued"`) {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if queuedJobs(env.dispatcher) != 1 {
		t.Fatalf("queued = %d", queuedJobs(env.dispatcher))
	}
	if env.lookup.calls != 0 {
		t.Fatal("handler must not call the topics API")
	}
}

func TestHandlerIgnoresNonTrigger(t *testing.T) {
	env := newHandlerEnv(t)
	cases := map[string]string{
		"mr_open_draft.json":            "hook-secret",
		"mr_close.json":                 "hook-secret",
		"emoji_revoke_eyes.json":        "legacy-secret", // fixture project is under platform/legacy
		"note_plain_no_discussion.json": "legacy-secret", // no thread => not a chat candidate
		"note_system.json":              "legacy-secret",
		"note_on_issue.json":            "legacy-secret",
	}
	for fixture, secret := range cases {
		recorder := postWebhook(t, env.handler, fixture, secret)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ignored"`) {
			t.Fatalf("%s: code=%d body=%s", fixture, recorder.Code, recorder.Body.String())
		}
	}
	if queuedJobs(env.dispatcher) != 0 {
		t.Fatalf("queued = %d, want 0", queuedJobs(env.dispatcher))
	}
	time.Sleep(50 * time.Millisecond)
	if posts := env.gitlab.posted(); len(posts) != 0 {
		t.Fatalf("posts = %v, want none for ignored events", posts)
	}
}

// The bot's own note events are ignored via bot user IDs, exactly like its
// emoji awards: replies the daemon posts must not loop back as commands.
func TestHandlerIgnoresBotNoteByID(t *testing.T) {
	env := newHandlerEnv(t)
	env.handler.groups.botIDs[999] = true
	recorder := postWebhook(t, env.handler, "note_command_bot.json", "legacy-secret")
	if !strings.Contains(recorder.Body.String(), `"ignored"`) {
		t.Fatalf("body=%s", recorder.Body.String())
	}
}

func TestHandlerCommandReview(t *testing.T) {
	env := newHandlerEnv(t)
	recorder := postWebhook(t, env.handler, "note_command_review.json", "legacy-secret")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"command":"review"`) {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if queuedJobs(env.dispatcher) != 1 {
		t.Fatalf("queued = %d, want 1", queuedJobs(env.dispatcher))
	}
	env.dispatcher.mu.Lock()
	state := env.dispatcher.states[jobKey{ProjectID: 43, IID: 11}]
	env.dispatcher.mu.Unlock()
	if state == nil || state.latest.Kind != TriggerManual {
		t.Fatalf("state = %+v, want queued manual job", state)
	}
	// The command note gets the acknowledgement emoji, no reply.
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	post := env.gitlab.posted()[0]
	if post.Path != "/api/v4/projects/43/merge_requests/11/notes/301/award_emoji" || post.Body["name"] != "white_check_mark" {
		t.Fatalf("post = %+v", post)
	}
}

func TestHandlerCommandAbortNothingRunning(t *testing.T) {
	env := newHandlerEnv(t)
	recorder := postWebhook(t, env.handler, "note_command_abort.json", "legacy-secret")
	if !strings.Contains(recorder.Body.String(), `"command":"abort"`) {
		t.Fatalf("body=%s", recorder.Body.String())
	}
	// Ack emoji plus a threaded reply saying there was nothing to abort.
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 2 })
	var reply recordedPost
	var ackEmoji string
	for _, post := range env.gitlab.posted() {
		if post.Body["body"] != "" {
			reply = post
		}
		if strings.HasSuffix(post.Path, "/award_emoji") {
			ackEmoji = post.Body["name"]
		}
	}
	if ackEmoji != "stop_button" {
		t.Fatalf("abort ack emoji = %q, want stop_button", ackEmoji)
	}
	if reply.Path != "/api/v4/projects/43/merge_requests/11/discussions/disc-302/notes" {
		t.Fatalf("reply path = %q, want threaded reply", reply.Path)
	}
	if !strings.Contains(reply.Body["body"], "No review is queued or running") {
		t.Fatalf("reply = %q", reply.Body["body"])
	}
}

func TestHandlerCommandAbortQueuedJob(t *testing.T) {
	env := newHandlerEnv(t)
	env.dispatcher.Enqueue(Event{Kind: TriggerAuto, ProjectID: 43, ProjectPath: "platform/legacy/tool", IID: 11, HeadSHA: "sha-1", Group: env.group})
	if queuedJobs(env.dispatcher) != 1 {
		t.Fatal("job must be queued")
	}
	postWebhook(t, env.handler, "note_command_abort.json", "legacy-secret")
	if queuedJobs(env.dispatcher) != 0 {
		t.Fatalf("queued = %d, want 0 after abort", queuedJobs(env.dispatcher))
	}
	waitFor(t, 3*time.Second, func() bool {
		for _, post := range env.gitlab.posted() {
			if strings.Contains(post.Body["body"], "Removed the queued review") {
				return true
			}
		}
		return false
	})
}

func TestHandlerCommandStatusIdle(t *testing.T) {
	env := newHandlerEnv(t)
	recorder := postWebhook(t, env.handler, "note_command_status.json", "legacy-secret")
	if !strings.Contains(recorder.Body.String(), `"command":"status"`) {
		t.Fatalf("body=%s", recorder.Body.String())
	}
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	post := env.gitlab.posted()[0]
	if !strings.Contains(post.Body["body"], "No review is queued or running") {
		t.Fatalf("reply = %q", post.Body["body"])
	}
}

func TestHandlerCommandStatusQueued(t *testing.T) {
	env := newHandlerEnv(t)
	env.dispatcher.Enqueue(Event{Kind: TriggerAuto, ProjectID: 43, ProjectPath: "platform/legacy/tool", IID: 11, HeadSHA: "sha-1", Group: env.group})
	postWebhook(t, env.handler, "note_command_status.json", "legacy-secret")
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	if post := env.gitlab.posted()[0]; !strings.Contains(post.Body["body"], "queued") {
		t.Fatalf("reply = %q", post.Body["body"])
	}
}

func TestHandlerCommandHelp(t *testing.T) {
	env := newHandlerEnv(t)
	postWebhook(t, env.handler, "note_command_help.json", "legacy-secret")
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	if post := env.gitlab.posted()[0]; !strings.Contains(post.Body["body"], "/nickpit review") {
		t.Fatalf("reply = %q", post.Body["body"])
	}
}

func TestHandlerCommandUnknown(t *testing.T) {
	env := newHandlerEnv(t)
	postWebhook(t, env.handler, "note_command_unknown.json", "legacy-secret")
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	post := env.gitlab.posted()[0]
	if !strings.Contains(post.Body["body"], `Unknown command "frobnicate"`) {
		t.Fatalf("reply = %q", post.Body["body"])
	}
}

// When GitLab rejects the threaded reply the answer falls back to a plain MR
// note.
func TestHandlerReplyFallsBackToPlainNote(t *testing.T) {
	env := newHandlerEnv(t)
	env.gitlab.mu.Lock()
	env.gitlab.failDiscussions = true
	env.gitlab.mu.Unlock()
	postWebhook(t, env.handler, "note_command_help.json", "legacy-secret")
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	if post := env.gitlab.posted()[0]; post.Path != "/api/v4/projects/43/merge_requests/11/notes" {
		t.Fatalf("post path = %q, want plain note fallback", post.Path)
	}
}

// Revoking the trigger emoji aborts the MR's review and confirms with a plain
// MR note (there is no command note to answer).
func TestHandlerEmojiRevokeAborts(t *testing.T) {
	env := newHandlerEnv(t)
	env.dispatcher.Enqueue(Event{Kind: TriggerManual, ProjectID: 43, ProjectPath: "platform/legacy/tool", IID: 11, HeadSHA: "sha-1", Group: env.group})
	recorder := postWebhook(t, env.handler, "emoji_revoke.json", "legacy-secret")
	if !strings.Contains(recorder.Body.String(), `"command":"abort"`) {
		t.Fatalf("body=%s", recorder.Body.String())
	}
	if queuedJobs(env.dispatcher) != 0 {
		t.Fatalf("queued = %d, want 0 after revoke", queuedJobs(env.dispatcher))
	}
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	post := env.gitlab.posted()[0]
	if post.Path != "/api/v4/projects/43/merge_requests/11/notes" || !strings.Contains(post.Body["body"], "Removed the queued review") {
		t.Fatalf("post = %+v", post)
	}
}

// A revoke with nothing to abort stays silent: revoking a stale award is
// routine cleanup, not a question.
func TestHandlerEmojiRevokeNothingSilent(t *testing.T) {
	env := newHandlerEnv(t)
	postWebhook(t, env.handler, "emoji_revoke.json", "legacy-secret")
	time.Sleep(50 * time.Millisecond)
	if posts := env.gitlab.posted(); len(posts) != 0 {
		t.Fatalf("posts = %v, want silence", posts)
	}
}

func TestHandlerSecretPerGroup(t *testing.T) {
	env := newHandlerEnv(t)
	// emoji fixtures live under platform/legacy → legacy-secret required.
	if recorder := postWebhook(t, env.handler, "emoji_award_nickpit.json", "hook-secret"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-group secret: code=%d", recorder.Code)
	}
	if recorder := postWebhook(t, env.handler, "emoji_award_nickpit.json", "legacy-secret"); recorder.Code != http.StatusOK {
		t.Fatalf("correct secret: code=%d", recorder.Code)
	}
	if queuedJobs(env.dispatcher) != 1 {
		t.Fatalf("queued = %d", queuedJobs(env.dispatcher))
	}
}

func TestHandlerRejectsWrongSecret(t *testing.T) {
	env := newHandlerEnv(t)
	for _, secret := range []string{"", "wrong"} {
		for _, fixture := range []string{"mr_open.json", "note_command_review.json"} {
			if recorder := postWebhook(t, env.handler, fixture, secret); recorder.Code != http.StatusUnauthorized {
				t.Fatalf("%s secret %q: code=%d", fixture, secret, recorder.Code)
			}
		}
	}
	if queuedJobs(env.dispatcher) != 0 {
		t.Fatal("nothing must be queued")
	}
}

func TestHandlerRejectsUnknownGroup(t *testing.T) {
	env := newHandlerEnv(t)
	body := []byte(`{"object_kind":"merge_request","project":{"id":1,"path_with_namespace":"other/repo"},"object_attributes":{"action":"open","iid":1}}`)
	if recorder := postWebhookBody(t, env.handler, body, "hook-secret"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", recorder.Code)
	}
}

func TestHandlerRejectsBadJSON(t *testing.T) {
	env := newHandlerEnv(t)
	if recorder := postWebhookBody(t, env.handler, []byte("{nope"), "hook-secret"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", recorder.Code)
	}
}

func TestHandlerRejectsOversizedBody(t *testing.T) {
	env := newHandlerEnv(t)
	big := bytes.Repeat([]byte("x"), maxWebhookBody+1)
	if recorder := postWebhookBody(t, env.handler, big, "hook-secret"); recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code=%d", recorder.Code)
	}
}

// A plain reply inside a nickpit thread spawns a chat child with the group's
// token and the daemon's config, so the daemon answers without loading the LLM.
func TestHandlerChatSpawnsChild(t *testing.T) {
	env := newHandlerEnv(t)
	// A resolved bot id is required (loop guard) and the thread must be a nickpit
	// thread (its root note carries a review marker).
	env.group.BotUserID = 5
	env.gitlab.discussionRoot = reviewmd.NewRenderer("https://host/").ForReview("rev-x").FindingBody(model.Finding{ID: "f1", Title: "Bug"}, "")
	recorder := postWebhook(t, env.handler, "note_plain.json", "legacy-secret")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"chat"`) {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	select {
	case spec := <-env.chat.calls:
		if spec.IID != 11 || spec.DiscussionID != "disc-306" {
			t.Fatalf("chat spec = %+v, want iid 11 discussion disc-306", spec)
		}
		if spec.ProjectPath != "platform/legacy/tool" || spec.Token != "t2" || spec.ConfigPath != "cfg.yaml" {
			t.Fatalf("chat spec = %+v, want project/token/config wired", spec)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("chat child was not spawned")
	}
}

// A reply in a non-nickpit thread (root note has no marker) is not answered.
func TestHandlerChatSkipsForeignThread(t *testing.T) {
	env := newHandlerEnv(t)
	env.group.BotUserID = 5
	env.gitlab.discussionRoot = "just a normal review comment from a human"
	recorder := postWebhook(t, env.handler, "note_plain.json", "legacy-secret")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"chat"`) {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	select {
	case <-env.chat.calls:
		t.Fatal("chat child spawned for a non-nickpit thread")
	case <-time.After(300 * time.Millisecond):
	}
}

// Without a resolved bot id, chat is skipped to avoid a reply loop.
func TestHandlerChatSkipsWhenBotUnresolved(t *testing.T) {
	env := newHandlerEnv(t)
	env.group.BotUserID = 0
	env.gitlab.discussionRoot = reviewmd.NewRenderer("https://host/").ForReview("rev-x").FindingBody(model.Finding{ID: "f1"}, "")
	postWebhook(t, env.handler, "note_plain.json", "legacy-secret")
	select {
	case <-env.chat.calls:
		t.Fatal("chat child spawned despite unresolved bot id")
	case <-time.After(300 * time.Millisecond):
	}
}

// A failed chat child is retried in-process: the webhook was already
// acknowledged with HTTP 200, so GitLab will not redeliver it — the daemon owns
// the retry.
func TestHandlerChatRetriesFailedChild(t *testing.T) {
	env := newHandlerEnv(t)
	env.handler.chatRetryDelay = time.Millisecond
	env.group.BotUserID = 5
	env.gitlab.discussionRoot = reviewmd.NewRenderer("https://host/").ForReview("rev-x").FindingBody(model.Finding{ID: "f1"}, "")
	env.chat.exitCodes = []int{1, 0} // first attempt fails, retry succeeds

	postWebhook(t, env.handler, "note_plain.json", "legacy-secret")
	for i := range 2 {
		select {
		case <-env.chat.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("chat attempt %d never ran", i+1)
		}
	}
	// The successful retry keeps the dedup mark: the same note is a duplicate.
	waitFor(t, 2*time.Second, func() bool { return !env.handler.chatSeen.markNew(306) })
}

// With chat disabled (nil runner), a thread reply is ignored, not spawned.
func TestHandlerChatDisabled(t *testing.T) {
	env := newHandlerEnv(t)
	env.handler.chatRunner = nil
	recorder := postWebhook(t, env.handler, "note_plain.json", "legacy-secret")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "ignored") {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	select {
	case <-env.chat.calls:
		t.Fatal("chat child spawned while disabled")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHandlerRejectsGet(t *testing.T) {
	env := newHandlerEnv(t)
	recorder := httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/webhooks/gitlab", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d", recorder.Code)
	}
}

func TestServerHealthz(t *testing.T) {
	env := newHandlerEnv(t)
	server := NewServer(":0", env.handler, env.dispatcher, 0, discardLogger())
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ok"`) {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestKeyedMutexReleasesIdleEntries(t *testing.T) {
	var k keyedMutex
	unlock := k.lock("proj!disc-1")
	if len(k.locks) != 1 {
		t.Fatalf("locks = %d, want 1 while held", len(k.locks))
	}
	unlock()
	if len(k.locks) != 0 {
		t.Fatalf("locks = %d, want 0 after release — idle entries must not accumulate", len(k.locks))
	}
	// Contended: a waiter keeps the entry alive; the last release removes it.
	unlock1 := k.lock("proj!disc-2")
	done := make(chan func(), 1)
	go func() { done <- k.lock("proj!disc-2") }()
	waitFor(t, 2*time.Second, func() bool {
		k.mu.Lock()
		defer k.mu.Unlock()
		l, ok := k.locks["proj!disc-2"]
		return ok && l.refs == 2
	})
	unlock1()
	unlock2 := <-done
	unlock2()
	k.mu.Lock()
	remaining := len(k.locks)
	k.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("locks = %d, want 0 after all holders released", remaining)
	}
}

func TestNoteDedup(t *testing.T) {
	d := newNoteDedup(2)
	if !d.markNew(1) {
		t.Fatal("first sighting of note 1 should be new")
	}
	if d.markNew(1) {
		t.Fatal("second sighting of note 1 should be a duplicate")
	}
	// A zero id (unknown) is always treated as new, never dropped.
	if !d.markNew(0) {
		t.Fatal("zero id must always be treated as new")
	}
	if !d.markNew(0) {
		t.Fatal("zero id must remain new on repeat")
	}
	// Capacity eviction: adding 2 and 3 evicts 1, so 1 is "new" again.
	d.markNew(2)
	d.markNew(3)
	if !d.markNew(1) {
		t.Fatal("note 1 should have been evicted and seen as new again")
	}
	// forget clears a mark so a redelivery after a failed attempt can retry.
	d.forget(3)
	if !d.markNew(3) {
		t.Fatal("forgotten note 3 should be seen as new again")
	}
	d.forget(0) // zero id is a no-op, never a panic
}

// A forget followed by a re-add must not leave a stale FIFO entry whose later
// eviction deletes the live mark and re-opens the duplicate-reply window.
func TestNoteDedupForgetRemovesQueuedEntry(t *testing.T) {
	d := newNoteDedup(2)
	d.markNew(7) // order [7]
	d.forget(7)  // must remove from order too
	d.markNew(7) // re-added after a retry succeeded; order [7]
	// Fill past capacity: evictions must never clear 7's live mark via a stale
	// queued occurrence.
	d.markNew(8)
	if d.markNew(7) {
		t.Fatal("live mark for note 7 was lost — stale FIFO entry evicted it")
	}
}
