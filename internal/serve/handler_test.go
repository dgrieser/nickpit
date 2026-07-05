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

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/testutil"
)

// newHandlerEnv wires a handler to a dispatcher with no started workers, so
// enqueued jobs stay observable and nothing runs.
func newHandlerEnv(t *testing.T) (*Handler, *Dispatcher, *countingTopicLookup) {
	t.Helper()
	set, _ := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t1", WebhookSecret: "hook-secret"},
		{Path: "platform/legacy", Token: "t2", WebhookSecret: "legacy-secret"},
	}, "https://gitlab.example.com", nil)
	lookup := &countingTopicLookup{}
	dispatcher := NewDispatcher(&fakeRunner{}, lookup.fn(), WorkerConfig{Topic: "nickpit"}, discardLogger())
	handler := NewHandler(set, dispatcher, "nickpit", discardLogger())
	return handler, dispatcher, lookup
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
	handler, dispatcher, lookup := newHandlerEnv(t)
	recorder := postWebhook(t, handler, "mr_open.json", "hook-secret")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"queued"`) {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if queuedJobs(dispatcher) != 1 {
		t.Fatalf("queued = %d", queuedJobs(dispatcher))
	}
	if lookup.calls != 0 {
		t.Fatal("handler must not call the topics API")
	}
}

func TestHandlerIgnoresNonTrigger(t *testing.T) {
	handler, dispatcher, _ := newHandlerEnv(t)
	cases := map[string]string{
		"mr_open_draft.json": "hook-secret",
		"mr_close.json":      "hook-secret",
		"emoji_revoke.json":  "legacy-secret", // fixture project is under platform/legacy
	}
	for fixture, secret := range cases {
		recorder := postWebhook(t, handler, fixture, secret)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ignored"`) {
			t.Fatalf("%s: code=%d body=%s", fixture, recorder.Code, recorder.Body.String())
		}
	}
	if queuedJobs(dispatcher) != 0 {
		t.Fatalf("queued = %d, want 0", queuedJobs(dispatcher))
	}
}

func TestHandlerSecretPerGroup(t *testing.T) {
	handler, dispatcher, _ := newHandlerEnv(t)
	// emoji fixtures live under platform/legacy → legacy-secret required.
	if recorder := postWebhook(t, handler, "emoji_award_nickpit.json", "hook-secret"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-group secret: code=%d", recorder.Code)
	}
	if recorder := postWebhook(t, handler, "emoji_award_nickpit.json", "legacy-secret"); recorder.Code != http.StatusOK {
		t.Fatalf("correct secret: code=%d", recorder.Code)
	}
	if queuedJobs(dispatcher) != 1 {
		t.Fatalf("queued = %d", queuedJobs(dispatcher))
	}
}

func TestHandlerRejectsWrongSecret(t *testing.T) {
	handler, dispatcher, _ := newHandlerEnv(t)
	for _, secret := range []string{"", "wrong"} {
		if recorder := postWebhook(t, handler, "mr_open.json", secret); recorder.Code != http.StatusUnauthorized {
			t.Fatalf("secret %q: code=%d", secret, recorder.Code)
		}
	}
	if queuedJobs(dispatcher) != 0 {
		t.Fatal("nothing must be queued")
	}
}

func TestHandlerRejectsUnknownGroup(t *testing.T) {
	handler, _, _ := newHandlerEnv(t)
	body := []byte(`{"object_kind":"merge_request","project":{"id":1,"path_with_namespace":"other/repo"},"object_attributes":{"action":"open","iid":1}}`)
	if recorder := postWebhookBody(t, handler, body, "hook-secret"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", recorder.Code)
	}
}

func TestHandlerRejectsBadJSON(t *testing.T) {
	handler, _, _ := newHandlerEnv(t)
	if recorder := postWebhookBody(t, handler, []byte("{nope"), "hook-secret"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", recorder.Code)
	}
}

func TestHandlerRejectsOversizedBody(t *testing.T) {
	handler, _, _ := newHandlerEnv(t)
	big := bytes.Repeat([]byte("x"), maxWebhookBody+1)
	if recorder := postWebhookBody(t, handler, big, "hook-secret"); recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code=%d", recorder.Code)
	}
}

func TestHandlerRejectsGet(t *testing.T) {
	handler, _, _ := newHandlerEnv(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/webhooks/gitlab", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d", recorder.Code)
	}
}

func TestServerHealthz(t *testing.T) {
	handler, dispatcher, _ := newHandlerEnv(t)
	server := NewServer(":0", handler, dispatcher, 0, discardLogger())
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ok"`) {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
