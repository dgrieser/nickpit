package serve

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

func TestUpdateSchedulerAdmitsOrphanRecoveryButNotTransientFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		admit  bool
	}{
		{"deleted", 404, true}, {"empty", 200, true},
		{"forbidden", 403, false}, {"rate limited", 429, false}, {"temporary failure", 500, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := reviewmd.NewRenderer("").ForReview("review").SummaryBodyCarried(&model.ReviewResult{ReviewID: "review"})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/discussions/thread") {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(`{"notes":[]}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"notes": []any{map[string]any{"id": 1, "body": root, "author": map[string]int{"id": 7}}}})
			}))
			defer server.Close()
			dir := filepath.Join(t.TempDir(), "state")
			store, err := NewUpdateStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.Close() }()
			first := testUpdateJob()
			first.BaseURL = server.URL
			first.SetID()
			next := *first
			next.DiscussionID = "next-thread"
			next.Created = first.Created.Add(time.Second)
			next.SetID()
			for _, job := range []*UpdateJob{first, &next} {
				if err := store.Save(job); err != nil {
					t.Fatal(err)
				}
			}
			groups, groupErrors := NewGroupSet(context.Background(), []config.ServeGroup{{Path: "group", Token: "token"}}, server.URL, func(context.Context, *glscm.Client) (int, error) { return 7, nil })
			if len(groupErrors) != 0 {
				t.Fatal(groupErrors)
			}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			runner := &scheduledUpdateRunner{started: make(chan ChatSpec, 2), release: make(chan string), store: store}
			h := NewHandler(groups, nil, HandlerConfig{Responses: NewResponseController(ResponseConfig{Enabled: true}, log)}, runner, ChatConfig{BaseURL: server.URL, UpdateStateDir: dir}, log)
			h.updatePollInterval = 10 * time.Millisecond
			h.StartUpdateWorker()
			defer h.ShutdownChats(0)
			if !tc.admit {
				select {
				case s := <-runner.started:
					t.Fatalf("transient failure admitted or bypassed: %+v", s)
				case <-time.After(100 * time.Millisecond):
				}
				return
			}
			for _, want := range []string{first.ID, next.ID} {
				select {
				case s := <-runner.started:
					if s.UpdateJobID != want {
						t.Fatalf("wrong job: %s want %s", s.UpdateJobID, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("orphan blocked queue")
				}
				if want == first.ID {
					runner.release <- "retired after recovery"
				}
			}
		})
	}
}

type scheduledUpdateRunner struct {
	started chan ChatSpec
	release chan string
	store   *UpdateStore
}

func (r *scheduledUpdateRunner) RunChat(ctx context.Context, spec ChatSpec) (int, string, error) {
	select {
	case r.started <- spec:
	case <-ctx.Done():
		return -1, "", ctx.Err()
	}
	select {
	case <-ctx.Done():
		return -1, "", ctx.Err()
	case <-r.release:
		job, err := r.store.Load(spec.UpdateJobID)
		if err != nil {
			return -1, "", err
		}
		job.Done = true
		return 0, "", r.store.Save(job)
	}
}

func TestUpdateSchedulerStrictFIFOAndIndependentMRs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store, err := NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	first := testUpdateJob()
	first.Created = time.Now()
	first.NextAttempt = time.Now().Add(time.Hour)
	second := *first
	second.NoteID++
	second.Created = first.Created.Add(time.Second)
	second.NextAttempt = time.Time{}
	second.SetID()
	other := *first
	other.IID = 2
	other.Created = first.Created.Add(2 * time.Second)
	other.NextAttempt = time.Time{}
	other.SetID()
	third := other
	third.IID = 3
	third.Created = first.Created.Add(3 * time.Second)
	third.SetID()
	for _, job := range []*UpdateJob{first, &second, &other, &third} {
		if err := store.Save(job); err != nil {
			t.Fatal(err)
		}
	}
	groups, groupErrors := NewGroupSet(context.Background(), []config.ServeGroup{{Path: "group", Token: "current"}}, first.BaseURL, nil)
	if len(groupErrors) != 0 {
		t.Fatal(groupErrors)
	}
	runner := &scheduledUpdateRunner{started: make(chan ChatSpec, 4), release: make(chan string), store: store}
	h := NewHandler(groups, nil, HandlerConfig{}, runner, ChatConfig{BaseURL: first.BaseURL, UpdateStateDir: dir, UpdateMaxConcurrent: 2}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.updatePollInterval = 10 * time.Millisecond
	h.StartUpdateWorker()
	defer h.ShutdownChats(0)
	wait := func() ChatSpec {
		t.Helper()
		select {
		case s := <-runner.started:
			return s
		case <-time.After(2 * time.Second):
			t.Fatal("worker did not start")
			return ChatSpec{}
		}
	}
	a, b := wait(), wait()
	if a.IID == 1 || b.IID == 1 || a.IID == b.IID {
		t.Fatalf("backoff bypass or missing parallelism: %+v %+v", a, b)
	}
	first.NextAttempt = time.Time{}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-runner.started:
		t.Fatalf("exceeded limit: %+v", s)
	case <-time.After(40 * time.Millisecond):
	}
	runner.release <- "finish"
	s := wait()
	if s.UpdateJobID != first.ID {
		t.Fatalf("wrong first job: %+v", s)
	}
	// Both slots are occupied; same-MR successor cannot start while its head runs.
	select {
	case s := <-runner.started:
		t.Fatalf("same MR overlap: %+v", s)
	case <-time.After(40 * time.Millisecond):
	}
	h.ShutdownChats(0)
	queued, err := store.Unfinished()
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range queued {
		if job.ID == second.ID {
			return
		}
	}
	t.Fatal("shutdown lost queued successor")
}

func TestUpdateSchedulerStartsSuccessorAfterCompletion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	store, err := NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	first := testUpdateJob()
	next := *first
	next.NoteID++
	next.Created = first.Created.Add(time.Second)
	next.SetID()
	for _, j := range []*UpdateJob{first, &next} {
		if err := store.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	groups, groupErrors := NewGroupSet(context.Background(), []config.ServeGroup{{Path: "group", Token: "token"}}, first.BaseURL, nil)
	if len(groupErrors) != 0 {
		t.Fatal(groupErrors)
	}
	runner := &scheduledUpdateRunner{started: make(chan ChatSpec, 2), release: make(chan string), store: store}
	h := NewHandler(groups, nil, HandlerConfig{}, runner, ChatConfig{BaseURL: first.BaseURL, UpdateStateDir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.StartUpdateWorker()
	defer h.ShutdownChats(0)
	select {
	case s := <-runner.started:
		if s.UpdateJobID != first.ID {
			t.Fatal("wrong first job")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no first job")
	}
	runner.release <- "finish"
	select {
	case s := <-runner.started:
		if s.UpdateJobID != next.ID {
			t.Fatal("wrong successor")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("completion did not wake successor")
	}
}

func TestUpdateSchedulerPolicyBlockedHeadPreventsOvertaking(t *testing.T) {
	root, _ := reviewmd.NewRenderer("").ForReview("review").SummaryBodyCarried(&model.ReviewResult{ReviewID: "review"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"notes": []any{map[string]any{"id": 1, "body": root, "author": map[string]int{"id": 7}}}})
	}))
	defer server.Close()
	dir := filepath.Join(t.TempDir(), "state")
	store, err := NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	first := testUpdateJob()
	first.BaseURL, first.Requested = server.URL, false
	first.SetID()
	next := *first
	next.NoteID++
	next.Requested = true
	next.Created = first.Created.Add(time.Second)
	next.SetID()
	other := next
	other.IID++
	other.SetID()
	for _, j := range []*UpdateJob{first, &next, &other} {
		if err := store.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	groups, groupErrors := NewGroupSet(context.Background(), []config.ServeGroup{{Path: "group", Token: "token"}}, server.URL, func(context.Context, *glscm.Client) (int, error) { return 7, nil })
	if len(groupErrors) != 0 {
		t.Fatal(groupErrors)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := &scheduledUpdateRunner{started: make(chan ChatSpec, 3), release: make(chan string), store: store}
	h := NewHandler(groups, nil, HandlerConfig{Responses: NewResponseController(ResponseConfig{Enabled: true, OptIn: true}, log)}, runner, ChatConfig{BaseURL: server.URL, UpdateStateDir: dir}, log)
	h.updatePollInterval = 10 * time.Millisecond
	h.StartUpdateWorker()
	defer h.ShutdownChats(0)
	select {
	case s := <-runner.started:
		if s.UpdateJobID != other.ID {
			t.Fatalf("policy-blocked head was bypassed: %+v", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("independent MR did not run")
	}
	select {
	case s := <-runner.started:
		t.Fatalf("policy-blocked MR ran: %+v", s)
	case <-time.After(50 * time.Millisecond):
	}
}
