package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGitLab serves the minimal API surface the daemon touches: MR status,
// project topics, award emoji, notes, and discussion replies.
type fakeGitLab struct {
	mu       sync.Mutex
	topics   []string
	state    string
	draft    bool
	headSHA  string
	awards   []string
	posts    []recordedPost
	topicGET int
	// failDiscussions makes discussion-reply POSTs 404 to exercise the
	// plain-note fallback.
	failDiscussions bool
	// discussionRoot, when set, is returned as the single root note of a
	// discussion GET, so the chat thread gate can be exercised.
	discussionRoot string
	// failDiscussionGET makes the chat thread gate's discussion GET fail with a
	// 429, exercising the unconfirmed-gate paths. discussionGETs counts the
	// gate's read attempts.
	failDiscussionGET bool
	discussionGETs    int
}

func (f *fakeGitLab) gateReads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.discussionGETs
}

// recordedPost is one captured POST request.
type recordedPost struct {
	Path string
	Body map[string]string
}

func (f *fakeGitLab) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.Method == http.MethodPost:
			var body map[string]string
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &body)
			if f.failDiscussions && strings.Contains(r.URL.Path, "/discussions/") {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"404 Not found"}`))
				return
			}
			f.posts = append(f.posts, recordedPost{Path: r.URL.Path, Body: body})
			if name := body["name"]; name != "" {
				f.awards = append(f.awards, name)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/api/v4/projects/42":
			f.topicGET++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 42, "topics": f.topics})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/discussions/"):
			// Single-discussion fetch used by the chat thread gate.
			f.discussionGETs++
			if f.failDiscussionGET {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"notes": []map[string]any{
					{"body": f.discussionRoot, "system": false, "author": map[string]any{"id": 5, "username": "someone"}},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"state": f.state, "draft": f.draft, "sha": f.headSHA})
		}
	})
}

func (f *fakeGitLab) awarded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.awards...)
}

func (f *fakeGitLab) posted() []recordedPost {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedPost(nil), f.posts...)
}

// fakeRunner records specs and optionally blocks until released.
type fakeRunner struct {
	mu    sync.Mutex
	specs []ReviewSpec
	gate  chan struct{}
	exit  int
	peak  atomic.Int64
	live  atomic.Int64
}

func (r *fakeRunner) Run(ctx context.Context, spec ReviewSpec) (int, string, error) {
	live := r.live.Add(1)
	defer r.live.Add(-1)
	for {
		peak := r.peak.Load()
		if live <= peak || r.peak.CompareAndSwap(peak, live) {
			break
		}
	}
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	if r.gate != nil {
		select {
		case <-r.gate:
		case <-ctx.Done():
		}
	}
	return r.exit, "fake.log", nil
}

func (r *fakeRunner) ran() []ReviewSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ReviewSpec(nil), r.specs...)
}

type dispatcherEnv struct {
	dispatcher *Dispatcher
	runner     *fakeRunner
	gitlab     *fakeGitLab
	group      *Group
	cancel     context.CancelFunc
}

func newDispatcherEnv(t *testing.T, workers int, gate bool) *dispatcherEnv {
	t.Helper()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	runner := &fakeRunner{}
	if gate {
		runner.gate = make(chan struct{})
	}
	groupSet := newTestGroupSetWithURL(t, server.URL)
	group := groupSet.Match("platform/api")
	topics := TopicLookup(GitLabTopicLookup)
	dispatcher := NewDispatcher(runner, topics, WorkerConfig{
		Topic:      "nickpit",
		StartEmoji: "eyes",
		BaseURL:    server.URL,
		ConfigPath: ".nickpit.yaml",
		LogDir:     t.TempDir(),
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	dispatcher.Start(ctx, workers)
	t.Cleanup(func() {
		cancel()
		dispatcher.Shutdown(2 * time.Second)
	})
	return &dispatcherEnv{dispatcher: dispatcher, runner: runner, gitlab: fake, group: group, cancel: cancel}
}

func autoEvent(iid int, sha string, group *Group) Event {
	return Event{Kind: TriggerAuto, ProjectID: 42, ProjectPath: "platform/api", IID: iid, HeadSHA: sha, Group: group}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestDispatcherRunsReview(t *testing.T) {
	env := newDispatcherEnv(t, 1, false)
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	spec := env.runner.ran()[0]
	if spec.ProjectPath != "platform/api" || spec.IID != 7 || spec.Token != "t" {
		t.Fatalf("spec = %+v", spec)
	}
	awards := env.gitlab.awarded()
	if len(awards) != 1 || awards[0] != "eyes" {
		t.Fatalf("awards = %v", awards)
	}
}

func TestDispatcherCoalescesQueuedEvents(t *testing.T) {
	env := newDispatcherEnv(t, 1, true)
	// First event occupies the single worker.
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	// Three updates for another MR while the worker is busy: they coalesce
	// into one queued job.
	env.gitlab.mu.Lock()
	env.gitlab.headSHA = "sha-4"
	env.gitlab.mu.Unlock()
	env.dispatcher.Enqueue(autoEvent(8, "sha-2", env.group))
	env.dispatcher.Enqueue(autoEvent(8, "sha-3", env.group))
	env.dispatcher.Enqueue(autoEvent(8, "sha-4", env.group))

	close(env.runner.gate)
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 2 })
	time.Sleep(50 * time.Millisecond)
	if got := len(env.runner.ran()); got != 2 {
		t.Fatalf("runs = %d, want 2 (coalesced)", got)
	}
}

func TestDispatcherPendingRerunAfterInFlight(t *testing.T) {
	env := newDispatcherEnv(t, 1, true)
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	// New head arrives while MR 7 is being reviewed → pending re-run.
	env.gitlab.mu.Lock()
	env.gitlab.headSHA = "sha-2"
	env.gitlab.mu.Unlock()
	env.dispatcher.Enqueue(autoEvent(7, "sha-2", env.group))

	close(env.runner.gate)
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 2 })
}

func TestDispatcherDropsAlreadyReviewedAuto(t *testing.T) {
	env := newDispatcherEnv(t, 1, false)
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	// Same head again (webhook retry): dropped. Manual trigger: runs.
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	time.Sleep(50 * time.Millisecond)
	if got := len(env.runner.ran()); got != 1 {
		t.Fatalf("runs = %d, want 1 after duplicate auto", got)
	}

	manual := autoEvent(7, "sha-1", env.group)
	manual.Kind = TriggerManual
	env.dispatcher.Enqueue(manual)
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 2 })
}

func TestDispatcherConcurrencyBound(t *testing.T) {
	env := newDispatcherEnv(t, 2, true)
	for iid := 1; iid <= 6; iid++ {
		env.dispatcher.Enqueue(autoEvent(iid, fmt.Sprintf("sha-%d", iid), env.group))
	}
	waitFor(t, 3*time.Second, func() bool { return env.runner.live.Load() == 2 })
	time.Sleep(50 * time.Millisecond)
	close(env.runner.gate)
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 6 })
	if peak := env.runner.peak.Load(); peak > 2 {
		t.Fatalf("peak concurrency = %d, want <= 2", peak)
	}
}

// A queued manual trigger must not be downgraded by a later auto event for
// the same MR — the auto rules (topic, draft, SHA LRU) would drop the review
// the user explicitly requested. The newer event's payload still wins.
func TestEnqueueCoalescePreservesManualKind(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())

	manual := autoEvent(7, "sha-1", group)
	manual.Kind = TriggerManual
	dispatcher.Enqueue(manual)
	dispatcher.Enqueue(autoEvent(7, "sha-2", group))

	dispatcher.mu.Lock()
	state := dispatcher.states[jobKey{ProjectID: 42, IID: 7}]
	latest := state.latest
	dispatcher.mu.Unlock()
	if latest.Kind != TriggerManual {
		t.Fatalf("kind = %v, manual must survive coalescing", latest.Kind)
	}
	if latest.HeadSHA != "sha-2" {
		t.Fatalf("sha = %q, newest payload must win", latest.HeadSHA)
	}
}

func TestEnqueuePendingPreservesManualKind(t *testing.T) {
	env := newDispatcherEnv(t, 1, true)
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	// While MR 7 runs: manual arrives, then auto — pending must stay manual.
	manual := autoEvent(7, "sha-2", env.group)
	manual.Kind = TriggerManual
	env.dispatcher.Enqueue(manual)
	env.dispatcher.Enqueue(autoEvent(7, "sha-3", env.group))

	env.dispatcher.mu.Lock()
	pending := env.dispatcher.states[jobKey{ProjectID: 42, IID: 7}].pending
	kind, sha := pending.Kind, pending.HeadSHA
	env.dispatcher.mu.Unlock()
	if kind != TriggerManual || sha != "sha-3" {
		t.Fatalf("pending = kind %v sha %q, want manual with newest sha", kind, sha)
	}
	close(env.runner.gate)
}

// Workers must not start queued jobs once the intake context is cancelled —
// only already-running reviews get the shutdown grace period.
func TestWorkersDoNotStartQueuedJobsAfterCancel(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	for iid := 1; iid <= 5; iid++ {
		dispatcher.Enqueue(autoEvent(iid, fmt.Sprintf("sha-%d", iid), group))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dispatcher.Start(ctx, 2)
	dispatcher.Shutdown(time.Second)
	if got := len(runner.ran()); got != 0 {
		t.Fatalf("runs = %d, want 0 after pre-cancelled context", got)
	}
}

func TestDispatcherAbortQueued(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.Enqueue(autoEvent(7, "sha-1", group))

	outcome := dispatcher.Abort(42, 7)
	if !outcome.Found || outcome.Running {
		t.Fatalf("outcome = %+v, want found queued job", outcome)
	}

	// The stale key left in the queue channel must be skipped by take.
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher.Start(ctx, 1)
	time.Sleep(50 * time.Millisecond)
	cancel()
	dispatcher.Shutdown(time.Second)
	if got := len(runner.ran()); got != 0 {
		t.Fatalf("runs = %d, want 0 after abort of queued job", got)
	}
}

func TestDispatcherAbortRunning(t *testing.T) {
	env := newDispatcherEnv(t, 1, true) // gate never closed: only ctx cancel frees the runner
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	outcome := env.dispatcher.Abort(42, 7)
	if !outcome.Found || !outcome.Running || outcome.Since < 0 {
		t.Fatalf("outcome = %+v, want running abort", outcome)
	}
	// The cancelled job finishes and clears its state.
	waitFor(t, 3*time.Second, func() bool {
		env.dispatcher.mu.Lock()
		defer env.dispatcher.mu.Unlock()
		_, ok := env.dispatcher.states[jobKey{ProjectID: 42, IID: 7}]
		return !ok
	})
	// An aborted run must not mark the head reviewed: the same SHA stays
	// re-reviewable by a later auto event.
	if env.dispatcher.alreadyReviewed(42, 7, "sha-1") {
		t.Fatal("aborted run must not mark the SHA reviewed")
	}
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 2 })
}

func TestDispatcherAbortNothing(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, _ := newWorkerEnv(t, fake, workerCfg())
	if outcome := dispatcher.Abort(42, 7); outcome.Found || outcome.Running {
		t.Fatalf("outcome = %+v, want zero", outcome)
	}
}

func TestDispatcherAbortClearsPending(t *testing.T) {
	env := newDispatcherEnv(t, 1, true)
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })
	// Parked re-run behind the running review; abort must clear both.
	env.dispatcher.Enqueue(autoEvent(7, "sha-2", env.group))

	if outcome := env.dispatcher.Abort(42, 7); !outcome.Running {
		t.Fatalf("outcome = %+v, want running abort", outcome)
	}
	waitFor(t, 3*time.Second, func() bool {
		env.dispatcher.mu.Lock()
		defer env.dispatcher.mu.Unlock()
		return len(env.dispatcher.states) == 0
	})
	time.Sleep(50 * time.Millisecond)
	if got := len(env.runner.ran()); got != 1 {
		t.Fatalf("runs = %d, want 1 (pending cleared by abort)", got)
	}
}

func TestDispatcherAbortThenReenqueue(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.Enqueue(autoEvent(7, "sha-1", group))
	dispatcher.Abort(42, 7)
	// Fresh request after the abort: exactly one run despite the stale key
	// still sitting in the queue channel.
	dispatcher.Enqueue(autoEvent(7, "sha-1", group))

	ctx, cancel := context.WithCancel(context.Background())
	dispatcher.Start(ctx, 1)
	t.Cleanup(func() {
		cancel()
		dispatcher.Shutdown(time.Second)
	})
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })
	time.Sleep(50 * time.Millisecond)
	if got := len(runner.ran()); got != 1 {
		t.Fatalf("runs = %d, want exactly 1", got)
	}
}

func TestDispatcherJobInfo(t *testing.T) {
	env := newDispatcherEnv(t, 1, true)
	if info := env.dispatcher.JobInfo(42, 7); info.Queued || info.Running {
		t.Fatalf("info = %+v, want idle", info)
	}

	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return env.dispatcher.JobInfo(42, 7).Running })
	if info := env.dispatcher.JobInfo(42, 7); info.Since < 0 || info.Pending {
		t.Fatalf("info = %+v, want running without pending", info)
	}

	// Second event parks behind the running review.
	env.dispatcher.Enqueue(autoEvent(7, "sha-2", env.group))
	if info := env.dispatcher.JobInfo(42, 7); !info.Running || !info.Pending {
		t.Fatalf("info = %+v, want running with pending", info)
	}

	// A job queued behind the busy worker reports queued.
	env.dispatcher.Enqueue(autoEvent(8, "sha-3", env.group))
	if info := env.dispatcher.JobInfo(42, 8); !info.Queued || info.Running {
		t.Fatalf("info = %+v, want queued", info)
	}
	close(env.runner.gate)
}

// Once shutdown began, take must refuse queued jobs even when a worker wins
// the select race and hands it a key.
func TestTakeRefusesJobsAfterShutdown(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.Enqueue(autoEvent(7, "sha-1", group))
	dispatcher.Shutdown(0) // no workers started: returns immediately, marks closed
	if _, _, ok := dispatcher.take(jobKey{ProjectID: 42, IID: 7}); ok {
		t.Fatal("take must refuse jobs once shutdown began")
	}
}

func TestSHALRUEviction(t *testing.T) {
	lru := newSHALRU(2)
	lru.Add("a")
	lru.Add("b")
	lru.Add("c")
	if lru.Contains("a") {
		t.Fatal("oldest entry must be evicted")
	}
	if !lru.Contains("b") || !lru.Contains("c") {
		t.Fatal("recent entries must remain")
	}
}
