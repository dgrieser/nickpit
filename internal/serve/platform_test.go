package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/scm/forge"
	"github.com/dgrieser/nickpit/internal/scm/forgejo"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// stubPlatform is a forge that is not GitLab: its own route, credential header,
// and payload. It proves the review path needs nothing beyond the seam.
type stubPlatform struct{}

func (stubPlatform) Forge() forge.Forge { return forgejo.Forge }

func (stubPlatform) WebhookPath() string { return "/webhooks/stub" }

func (stubPlatform) Decode(_ http.Header, body []byte) (Delivery, error) {
	var delivery stubDelivery
	if err := json.Unmarshal(body, &delivery); err != nil {
		return nil, err
	}
	return &delivery, nil
}

func (stubPlatform) Authenticate(group *Group, header http.Header, _ []byte, _ time.Time) bool {
	return group.CheckSecret(header.Get("X-Stub-Secret"))
}

func (stubPlatform) AuthMethod(*Group) string { return "stub" }

type stubDelivery struct {
	Repo    string `json:"repo"`
	RepoID  int    `json:"repo_id"`
	Number  int    `json:"number"`
	Comment int    `json:"comment"`
	Body    string `json:"body"`
}

func (d *stubDelivery) ProjectID() int { return d.RepoID }

func (d *stubDelivery) ProjectPath() string { return d.Repo }

func (d *stubDelivery) Decide(policy Policy) Decision {
	command, arg := ParseCommand(d.Body, policy.CommandKeyword)
	if command == CommandNone {
		return Decision{Kind: TriggerNone, Reason: "no command"}
	}
	decision := Decision{Command: command, Reason: "command " + command.String(), IID: d.Number, NoteID: d.Comment, UnknownArg: arg}
	if command == CommandReview {
		decision.Kind = TriggerManual
	}
	return decision
}

var errStubRejected = errors.New("stub: reaction not allowed")

// stubRemote records every call the review path makes.
type stubRemote struct {
	mu     sync.Mutex
	status RequestStatus
	// reject is a reaction name the forge refuses to add.
	reject string
	calls  []string
}

func (r *stubRemote) record(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, fmt.Sprintf(format, args...))
}

func (r *stubRemote) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

func (r *stubRemote) saw(call string) bool {
	return slices.Contains(r.recorded(), call)
}

func (r *stubRemote) RequestStatus(_ context.Context, req Request) (RequestStatus, error) {
	r.record("status %s#%d", req.ProjectPath, req.IID)
	return r.status, nil
}

func (r *stubRemote) AckComment(_ context.Context, _ Request, commentID int, emoji string) error {
	r.record("ack %d %s", commentID, emoji)
	return nil
}

func (r *stubRemote) SetRequestReaction(_ context.Context, _ Request, add string, _ ...string) error {
	r.record("request %q", add)
	if add != "" && add == r.reject {
		return errStubRejected
	}
	return nil
}

func (r *stubRemote) SetCommentReaction(_ context.Context, _ Request, commentID int, add string) error {
	r.record("comment %d %q", commentID, add)
	if add != "" && add == r.reject {
		return errStubRejected
	}
	return nil
}

func (r *stubRemote) Reply(_ context.Context, req Request, _ string, body string) error {
	r.record("reply %s#%d %s", req.ProjectPath, req.IID, body)
	return nil
}

func (r *stubRemote) ReactionFailure(err error) ReactionFailure {
	return ReactionFailure{Rejected: errors.Is(err, errStubRejected)}
}

type stubEnv struct {
	handler *Handler
	remote  *stubRemote
	runner  *fakeRunner
}

// newStubEnv runs a handler and one worker against the stub platform. The
// group has no GitLab client at all, so any stray gitlabClient use panics.
func newStubEnv(t *testing.T, status RequestStatus) *stubEnv {
	t.Helper()
	remote := &stubRemote{status: status}
	set, warnings := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "owner", Token: "bot-token", WebhookSecret: "s3cret"},
	}, "https://forge.example", nil)
	if len(warnings) > 0 {
		t.Fatal(warnings)
	}
	group := set.Match("owner/repo")
	group.Remote = remote
	group.BotUserID = 9

	runner := &fakeRunner{}
	topics := func(context.Context, *Group, int) ([]string, error) {
		t.Error("topic lookup on a manual review")
		return nil, nil
	}
	dispatcher := NewDispatcher(runner, topics, nil, WorkerConfig{
		StartEmoji: "eyes",
		AckEmoji:   "eyes",
		DoneEmoji:  "rocket",
		FailEmoji:  "confused",
		LogDir:     t.TempDir(),
	}, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher.Start(ctx, 1)
	t.Cleanup(func() {
		cancel()
		dispatcher.Shutdown(2 * time.Second)
	})
	handler := NewHandler(stubPlatform{}, set, dispatcher, HandlerConfig{CommandKeyword: "nickpit"}, nil, ChatConfig{}, discardLogger())
	return &stubEnv{handler: handler, remote: remote, runner: runner}
}

func (e *stubEnv) post(t *testing.T, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(stubDelivery{Repo: "owner/repo", RepoID: 5, Number: 3, Comment: 11, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, e.handler.WebhookPath(), bytes.NewReader(payload))
	req.Header.Set("X-Stub-Secret", secret)
	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, req)
	return recorder
}

func (e *stubEnv) specs() []ReviewSpec {
	e.runner.mu.Lock()
	defer e.runner.mu.Unlock()
	return slices.Clone(e.runner.specs)
}

func TestPlatformReviewCommandRunsThroughRemote(t *testing.T) {
	env := newStubEnv(t, RequestStatus{Open: true, State: "open", HeadSHA: "sha-9"})

	if recorder := env.post(t, "wrong", "/nickpit review"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("bad credential: status = %d, want 401", recorder.Code)
	}
	recorder := env.post(t, "s3cret", "/nickpit review")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"command":"review"`) {
		t.Fatalf("review command: status = %d body = %s", recorder.Code, recorder.Body)
	}

	waitFor(t, 2*time.Second, func() bool {
		return env.remote.saw(`request "rocket"`) && env.remote.saw(`comment 11 "rocket"`)
	})
	specs := env.specs()
	if len(specs) != 1 {
		t.Fatalf("runner specs = %d, want 1", len(specs))
	}
	if spec := specs[0]; spec.ProjectPath != "owner/repo" || spec.IID != 3 || spec.Token != "bot-token" || spec.HeadSHA != "sha-9" {
		t.Fatalf("review spec = %+v", spec)
	}
	want := []string{"ack 11 eyes", "status owner/repo#3", `request "eyes"`}
	if got := env.remote.recorded(); len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
		t.Fatalf("calls before the review ran = %q, want prefix %q", got, want)
	}
}

func TestPlatformClosedRequestIsNotReviewed(t *testing.T) {
	env := newStubEnv(t, RequestStatus{State: "closed"})

	if recorder := env.post(t, "s3cret", "/nickpit review"); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	waitFor(t, 2*time.Second, func() bool { return env.remote.saw(`comment 11 "confused"`) })
	if specs := env.specs(); len(specs) != 0 {
		t.Fatalf("closed request was reviewed: %+v", specs)
	}
	if env.remote.saw(`request "eyes"`) {
		t.Fatalf("start reaction on a closed request: %q", env.remote.recorded())
	}
}

func TestPlatformRejectedOutcomeFallsBackToRevoke(t *testing.T) {
	env := newStubEnv(t, RequestStatus{Open: true, State: "open"})
	env.remote.reject = "rocket"

	if recorder := env.post(t, "s3cret", "/nickpit review"); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	// The forge refuses the outcome reaction; the in-progress marker must still
	// come off, on the request and on the command comment.
	waitFor(t, 2*time.Second, func() bool {
		calls := env.remote.recorded()
		request := slices.Index(calls, `request "rocket"`)
		comment := slices.Index(calls, `comment 11 "rocket"`)
		return request >= 0 && comment >= 0 &&
			slices.Contains(calls[request:], `request ""`) &&
			slices.Contains(calls[comment:], `comment 11 ""`)
	})
}

func TestPlatformStatusCommandReplies(t *testing.T) {
	env := newStubEnv(t, RequestStatus{Open: true, State: "open"})

	recorder := env.post(t, "s3cret", "/nickpit status")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"command":"status"`) {
		t.Fatalf("status command: status = %d body = %s", recorder.Code, recorder.Body)
	}
	want := "reply owner/repo#3 " + statusText(JobInfo{})
	waitFor(t, 2*time.Second, func() bool { return env.remote.saw(want) })
	if specs := env.specs(); len(specs) != 0 {
		t.Fatalf("status command started a review: %+v", specs)
	}
}

func TestHandlerWebhookPath(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		want     string
	}{
		{name: "gitlab", platform: GitLab, want: "/webhooks/gitlab"},
		{name: "other platform", platform: stubPlatform{}, want: "/webhooks/stub"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(test.platform, nil, nil, HandlerConfig{}, nil, ChatConfig{}, discardLogger())
			if got := handler.WebhookPath(); got != test.want {
				t.Fatalf("WebhookPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExecRunnerUsesForgeCommandAndCredentials(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake not portable to windows")
	}
	script := filepath.Join(t.TempDir(), "fake-nickpit")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
echo "args:$@"
echo "token:$NICKPIT_FORGEJO_TOKEN"
echo "base_url:$NICKPIT_FORGEJO_BASE_URL"
echo "gitlab_token:${NICKPIT_GITLAB_TOKEN-unset}"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	// An inherited value would reach the child; Setenv restores it afterwards.
	t.Setenv("NICKPIT_GITLAB_TOKEN", "")
	if err := os.Unsetenv("NICKPIT_GITLAB_TOKEN"); err != nil {
		t.Fatal(err)
	}
	runner := &ExecRunner{Executable: script, Forge: forgejo.Forge, now: time.Now}

	exitCode, logPath, err := runner.Run(context.Background(), ReviewSpec{
		ProjectPath: "owner/repo",
		IID:         3,
		Token:       "bot-token",
		BaseURL:     "https://forge.example",
		LogDir:      t.TempDir(),
	})
	if err != nil || exitCode != 0 {
		t.Fatalf("Run() = %d, %v", exitCode, err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"args:forgejo pr --repo owner/repo --id 3 --publish --require-publish",
		"token:bot-token",
		"base_url:https://forge.example",
		"gitlab_token:unset",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("child output misses %q:\n%s", want, data)
		}
	}
}

func TestGitLabReactionFailure(t *testing.T) {
	retryAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name string
		err  error
		want ReactionFailure
	}{
		{name: "nil", err: nil, want: ReactionFailure{}},
		{name: "rejected award", err: &gitlab.APIError{Method: http.MethodPost, Status: http.StatusBadRequest}, want: ReactionFailure{Rejected: true}},
		{name: "unprocessable award", err: &gitlab.APIError{Method: http.MethodPost, Status: http.StatusUnprocessableEntity}, want: ReactionFailure{Rejected: true}},
		{name: "bad request on delete is not a rejection", err: &gitlab.APIError{Method: http.MethodDelete, Status: http.StatusBadRequest}, want: ReactionFailure{}},
		{name: "target gone", err: fmt.Errorf("replace: %w", &gitlab.APIError{Method: http.MethodGet, Status: http.StatusNotFound}), want: ReactionFailure{TargetGone: true}},
		{name: "rate limited", err: &gitlab.APIError{Method: http.MethodPost, Status: http.StatusTooManyRequests, RetryAfter: retryAt}, want: ReactionFailure{RetryAfter: retryAt}},
		{
			name: "joined list and award failures",
			err: errors.Join(
				&gitlab.APIError{Method: http.MethodGet, Status: http.StatusInternalServerError, RetryAfter: retryAt},
				&gitlab.APIError{Method: http.MethodPost, Status: http.StatusBadRequest},
			),
			want: ReactionFailure{Rejected: true, RetryAfter: retryAt},
		},
		{name: "timeout stays retryable", err: context.DeadlineExceeded, want: ReactionFailure{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (gitlabRemote{}).ReactionFailure(test.err); got != test.want {
				t.Fatalf("ReactionFailure() = %+v, want %+v", got, test.want)
			}
		})
	}
}
