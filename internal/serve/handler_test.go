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
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
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
type fakeChatRunner struct {
	calls chan ChatSpec
}

func newFakeChatRunner() *fakeChatRunner {
	return &fakeChatRunner{calls: make(chan ChatSpec, 4)}
}

func (r *fakeChatRunner) RunChat(_ context.Context, spec ChatSpec) (int, string, error) {
	r.calls <- spec
	return 0, "chat.log", nil
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

// A plain reply inside a discussion thread spawns a chat child with the group's
// token and the daemon's config, so the daemon answers without loading the LLM.
func TestHandlerChatSpawnsChild(t *testing.T) {
	env := newHandlerEnv(t)
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
