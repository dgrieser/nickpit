package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeBotUserID is the user the fake attributes every award to; the test groups
// resolve the same id as their bot user (newTestGroupSetWithURL), so revokes
// pass the own-award filter — revoking by name alone is refused by the client.
const fakeBotUserID = 77

// mrStatusPath matches the MR-status GET (and nothing below it, like award or
// note subresources), so tests can park that one request on the status gate.
var mrStatusPath = regexp.MustCompile(`/merge_requests/\d+$`)

// fakeGitLab serves the minimal API surface the daemon touches: MR status,
// project topics, award emoji, notes, and discussion replies.
type fakeGitLab struct {
	mu       sync.Mutex
	topics   []string
	state    string
	draft    bool
	headSHA  string
	awards   []recordedAward
	revoked  []recordedAward
	nextID   int
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
	// statusGate, when set (before the server starts), blocks every MR-status
	// GET until the channel is closed — without holding the fake's mutex, so
	// other requests proceed. statusArrived counts requests reaching the gate,
	// letting a test cancel a job while FetchMRStatus is provably in flight.
	statusGate    chan struct{}
	statusArrived atomic.Int32
	// emojiGate blocks award-emoji reads so tests can measure cleanup request
	// concurrency without holding the fake's mutex.
	emojiGate    chan struct{}
	emojiArrived atomic.Int32
	// emojiFailures returns 503 from the next award-list requests. Tests set it
	// after the start reaction succeeds to isolate settlement failures.
	emojiFailures atomic.Int32
	// emojiFailureStatus, when non-zero, fails every award-list request. GET
	// timestamps let retry tests verify shared pacing across different jobs.
	emojiFailureStatus int
	// emojiFailureGate blocks configured failures after their timestamps are
	// recorded, letting retry tests form a complete initial admitted wave.
	emojiFailureGate chan struct{}
	emojiRetryAfter  string
	emojiGETs        atomic.Int32
	emojiGETTimes    []time.Time
	// missingNoteIDs makes every emoji request for selected notes return 404.
	missingNoteIDs map[int]bool
	// emojiFailurePaths returns 503 from selected award-list paths. Protected by
	// mu; tests configure it before requests begin.
	emojiFailurePaths map[string]int
	// emojiPostStatuses rejects selected award names with the configured HTTP
	// status, modeling an invalid or unsupported configured emoji.
	emojiPostStatuses map[string]int
	// emojiWaitForCancel holds award-list requests until client cancellation.
	emojiWaitForCancel bool
	// emojiWaitForCancelPaths restricts that behavior to selected award-list
	// paths, allowing shutdown tests to stall background cleanup while queued-job
	// cleanup remains responsive.
	emojiWaitForCancelPaths map[string]bool
	// emojiPostGate delays the first award POST even after its request context is
	// cancelled, modeling a server-side award that commits after client timeout.
	emojiPostGate    chan struct{}
	emojiPostArrived atomic.Int32
	emojiPostGated   atomic.Bool
}

func (f *fakeGitLab) gateReads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.discussionGETs
}

func (f *fakeGitLab) discussionBody() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.discussionRoot
}

// recordedPost is one captured write request.
type recordedPost struct {
	Path string
	Body map[string]string
}

// recordedAward is one live emoji reaction on the fake's awardable (an MR or a
// note, told apart by Path), so tests can assert what the daemon awarded and
// what it revoked again.
type recordedAward struct {
	ID   int
	Path string
	Name string
}

func (f *fakeGitLab) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.emojiPostGate != nil && r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/award_emoji") && f.emojiPostGated.CompareAndSwap(false, true) {
			f.emojiPostArrived.Add(1)
			<-f.emojiPostGate
		}
		if f.statusGate != nil && r.Method == http.MethodGet && mrStatusPath.MatchString(r.URL.Path) {
			f.statusArrived.Add(1)
			<-f.statusGate
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji") {
			f.emojiGETs.Add(1)
			f.mu.Lock()
			f.emojiGETTimes = append(f.emojiGETTimes, time.Now())
			failureStatus := f.emojiFailureStatus
			failureGate := f.emojiFailureGate
			retryAfter := f.emojiRetryAfter
			f.mu.Unlock()
			if failureStatus != 0 {
				if failureGate != nil {
					<-failureGate
				}
				if retryAfter != "" {
					w.Header().Set("Retry-After", retryAfter)
				}
				w.WriteHeader(failureStatus)
				return
			}
		}
		if f.emojiGate != nil && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji") {
			f.emojiArrived.Add(1)
			<-f.emojiGate
		}
		if f.emojiWaitForCancel && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji") {
			f.emojiArrived.Add(1)
			<-r.Context().Done()
			return
		}
		if f.emojiWaitForCancelPaths[r.URL.Path] && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji") {
			f.emojiArrived.Add(1)
			<-r.Context().Done()
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji") {
			for failures := f.emojiFailures.Load(); failures > 0; failures = f.emojiFailures.Load() {
				if f.emojiFailures.CompareAndSwap(failures, failures-1) {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
			}
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for noteID := range f.missingNoteIDs {
			if strings.Contains(r.URL.Path, fmt.Sprintf("/notes/%d/award_emoji", noteID)) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"404 Not found"}`))
				return
			}
		}
		if r.Method == http.MethodGet && f.emojiFailurePaths[r.URL.Path] > 0 {
			f.emojiFailurePaths[r.URL.Path]--
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		switch {
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/discussions/"):
			var body map[string]string
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &body)
			f.discussionRoot = body["body"]
			f.posts = append(f.posts, recordedPost{Path: r.URL.Path, Body: body})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
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
			if status := f.emojiPostStatuses[body["name"]]; status != 0 && strings.HasSuffix(r.URL.Path, "/award_emoji") {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"Name is invalid"}`))
				return
			}
			if name := body["name"]; name != "" {
				f.nextID++
				f.awards = append(f.awards, recordedAward{ID: f.nextID, Path: r.URL.Path, Name: name})
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/award_emoji/"):
			base, rawID, _ := strings.Cut(r.URL.Path, "/award_emoji/")
			id, _ := strconv.Atoi(rawID)
			index := slices.IndexFunc(f.awards, func(award recordedAward) bool {
				return award.ID == id && award.Path == base+"/award_emoji"
			})
			if index < 0 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"404 Not found"}`))
				return
			}
			f.revoked = append(f.revoked, f.awards[index])
			f.awards = slices.Delete(f.awards, index, index+1)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji"):
			live := make([]map[string]any, 0, len(f.awards))
			for _, award := range f.awards {
				if award.Path != r.URL.Path {
					continue
				}
				live = append(live, map[string]any{"id": award.ID, "name": award.Name, "user": map[string]any{"id": fakeBotUserID}})
			}
			_ = json.NewEncoder(w).Encode(live)
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
					{"id": 900, "body": f.discussionRoot, "system": false, "author": map[string]any{"id": 5, "username": "someone"}},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"state": f.state, "draft": f.draft, "sha": f.headSHA})
		}
	})
}

// awarded returns the names of the reactions currently live on the fake, in the
// order they were awarded; revoked ones are gone.
func (f *fakeGitLab) awarded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.awards))
	for _, award := range f.awards {
		names = append(names, award.Name)
	}
	return names
}

// awardedOn is awarded() restricted to one awardable: the merge request itself
// for noteID 0, otherwise that note.
func (f *fakeGitLab) awardedOn(iid, noteID int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	suffix := fmt.Sprintf("/merge_requests/%d/award_emoji", iid)
	if noteID != 0 {
		suffix = fmt.Sprintf("/merge_requests/%d/notes/%d/award_emoji", iid, noteID)
	}
	names := make([]string, 0, len(f.awards))
	for _, award := range f.awards {
		if strings.HasSuffix(award.Path, suffix) {
			names = append(names, award.Name)
		}
	}
	return names
}

// preAward seeds a live reaction, standing in for the ack emoji the handler
// awarded on a command note before a worker picked the job up.
func (f *fakeGitLab) preAward(iid, noteID int, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	path := fmt.Sprintf("/api/v4/projects/42/merge_requests/%d/award_emoji", iid)
	if noteID != 0 {
		path = fmt.Sprintf("/api/v4/projects/42/merge_requests/%d/notes/%d/award_emoji", iid, noteID)
	}
	f.awards = append(f.awards, recordedAward{ID: f.nextID, Path: path, Name: name})
}

// awardPosted returns the name of every award POST ever made, including awards
// revoked again later — awarded() is the live view and reads empty after an
// award-then-revoke sequence that a "must not decorate" test still has to fail.
func (f *fakeGitLab) awardPosted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var names []string
	for _, post := range f.posts {
		if strings.HasSuffix(post.Path, "/award_emoji") && post.Body["name"] != "" {
			names = append(names, post.Body["name"])
		}
	}
	return names
}

// revokedNames returns the names of the reactions the daemon revoked.
func (f *fakeGitLab) revokedNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.revoked))
	for _, award := range f.revoked {
		names = append(names, award.Name)
	}
	return names
}

func (f *fakeGitLab) posted() []recordedPost {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedPost(nil), f.posts...)
}

func (f *fakeGitLab) emojiRequestTimes() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.emojiGETTimes)
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
	cfg := workerCfg()
	cfg.BaseURL = server.URL
	cfg.ConfigPath = ".nickpit.yaml"
	cfg.LogDir = t.TempDir()
	dispatcher := NewDispatcher(runner, topics, nil, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

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
	// The start emoji marked the MR while the review ran and was replaced by the
	// done emoji once it landed. Settle runs on the worker goroutine after the
	// runner returns, so wait for the flip instead of asserting right away.
	waitFor(t, 3*time.Second, func() bool {
		awards := env.gitlab.awarded()
		return len(awards) == 1 && awards[0] == "white_check_mark"
	})
	if revoked := env.gitlab.revokedNames(); len(revoked) != 1 || revoked[0] != "eyes" {
		t.Fatalf("revoked = %v, want the start emoji", revoked)
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

// Coalescing keeps every acknowledged command note: each one wears the
// in-progress reaction and has to be flipped when the coalesced review ends.
func TestEnqueueCoalesceAccumulatesAckNotes(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())

	dispatcher.Enqueue(commandEvent(7, "sha-1", group, 301))
	dispatcher.Enqueue(commandEvent(7, "sha-2", group, 302))
	dispatcher.Enqueue(commandEvent(7, "sha-3", group, 301)) // redelivery of the first

	dispatcher.mu.Lock()
	latest := dispatcher.states[jobKey{ProjectID: 42, IID: 7}].latest
	dispatcher.mu.Unlock()
	if fmt.Sprint(latest.AckNoteIDs) != "[301 302]" {
		t.Fatalf("ack notes = %v, want both notes once each", latest.AckNoteIDs)
	}
}

// A flood of review comments on one MR must not grow the event without bound;
// notes over the cap are reported back so their ack reaction can be released.
func TestMergeAckNotesCapped(t *testing.T) {
	existing := make([]int, maxAckNotes)
	for i := range existing {
		existing[i] = i + 1
	}
	merged, dropped := mergeAckNotes(existing, []int{existing[0], 9001})
	if len(merged) != maxAckNotes || slices.Contains(merged, 9001) {
		t.Fatalf("merged %d notes, want the first %d kept", len(merged), maxAckNotes)
	}
	if fmt.Sprint(dropped) != "[9001]" {
		t.Fatalf("dropped = %v, want the overflowing note (never an already-tracked one)", dropped)
	}
}

// A note dropped over the cap was already acknowledged by the handler; its ack
// reaction is released so the comment does not read as in progress forever.
func TestEnqueueCapOverflowReleasesDroppedAck(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())

	first := commandEvent(7, "sha-1", group, 0)
	first.AckNoteIDs = make([]int, maxAckNotes)
	for i := range first.AckNoteIDs {
		first.AckNoteIDs[i] = i + 1
	}
	dispatcher.Enqueue(first)
	fake.preAward(7, 9001, "eyes")
	dispatcher.Enqueue(commandEvent(7, "sha-2", group, 9001))

	waitFor(t, 3*time.Second, func() bool { return len(fake.awardedOn(7, 9001)) == 0 })
	if revoked := fake.revokedNames(); len(revoked) != 1 || revoked[0] != "eyes" {
		t.Fatalf("revoked = %v, want the dropped note's ack", revoked)
	}
}

// A capped note whose award timed out must not be revoked until the uncertainty
// window closes. GitLab can commit that POST after an earlier revoke returned
// success; the delayed cleanup must then observe and remove it.
func TestEnqueueCapOverflowPreservesUncertainAckUntilFinalSweep(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())

	first := commandEvent(7, "sha-1", group, 0)
	first.AckNoteIDs = make([]int, maxAckNotes)
	for index := range first.AckNoteIDs {
		first.AckNoteIDs[index] = index + 1
	}
	dispatcher.Enqueue(first)

	deadline := time.Now().Add(100 * time.Millisecond)
	overflow := commandEvent(7, "sha-2", group, 9001)
	overflow.UncertainAckNoteIDs = []int{9001}
	overflow.AckCleanupUntil = deadline
	dispatcher.Enqueue(overflow)

	time.Sleep(25 * time.Millisecond)
	if requests := fake.emojiGETs.Load(); requests != 0 {
		t.Fatalf("cleanup requests before uncertainty deadline = %d, want 0", requests)
	}
	// Timed-out handler request commits after enqueue and after an immediate
	// cleanup would have incorrectly declared success.
	fake.preAward(7, 9001, "eyes")
	waitFor(t, time.Second, func() bool {
		return time.Now().After(deadline) && fake.emojiGETs.Load() > 0 && len(fake.awardedOn(7, 9001)) == 0
	})
	if revoked := fake.revokedNames(); !slices.Contains(revoked, "eyes") {
		t.Fatalf("revoked = %v, want delayed acknowledgement removed", revoked)
	}
}

// A pending review can be aborted while an older review's cleanup is already
// in flight. Its timed-out acknowledgement remains uncertain on the nested
// aborted event, so durable cleanup must wait for that event's final sweep too.
func TestDurableCleanupDelaysAbortedUncertainAckUntilFinalSweep(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	t.Cleanup(dispatcher.jobCancel)
	t.Cleanup(dispatcher.cleanupCancel)

	key := jobKey{ProjectID: 42, IID: 7}
	event := autoEvent(7, "sha-1", group)
	aborted := commandEvent(7, "sha-2", group, 301)
	deadline := time.Now().Add(100 * time.Millisecond)
	aborted.UncertainAckNoteIDs = []int{301}
	aborted.AckCleanupUntil = deadline
	cleanup := reactionCleanup{event: event, outcome: outcomeDone, aborted: &aborted}
	dispatcher.states[key] = &jobState{
		status:         stateCleanup,
		latest:         event,
		cleanup:        &cleanup,
		cleanupVersion: 1,
	}
	dispatcher.queueDurableCleanup(key)

	time.Sleep(25 * time.Millisecond)
	if requests := fake.emojiGETs.Load(); requests != 0 {
		t.Fatalf("cleanup requests before aborted uncertainty deadline = %d, want 0", requests)
	}
	fake.preAward(7, 301, "eyes")
	waitFor(t, time.Second, func() bool {
		return time.Now().After(deadline) && fake.emojiGETs.Load() > 0 && len(fake.awardedOn(7, 301)) == 0
	})
}

// Overflow acknowledgement cleanup must have finite goroutine, queue, and
// outbound-request bounds even when authenticated commands keep arriving.
func TestEnqueueCapOverflowCleanupIsBounded(t *testing.T) {
	fake := &fakeGitLab{
		topics:    []string{"nickpit"},
		state:     "opened",
		headSHA:   "sha-1",
		emojiGate: make(chan struct{}),
	}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())

	first := commandEvent(7, "sha-1", group, 0)
	first.AckNoteIDs = make([]int, maxAckNotes)
	for i := range first.AckNoteIDs {
		first.AckNoteIDs[i] = i + 1
	}
	dispatcher.Enqueue(first)
	for i := range ackCleanupQueueCapacity + maxAckCleanupWorkers + 32 {
		noteID := 10_000 + i
		fake.preAward(7, noteID, "eyes")
		dispatcher.Enqueue(commandEvent(7, "sha-2", group, noteID))
	}

	waitFor(t, 3*time.Second, func() bool {
		return fake.emojiArrived.Load() == maxAckCleanupWorkers
	})
	dispatcher.cleanupMu.Lock()
	workers := dispatcher.cleanupWorkers
	queued := len(dispatcher.cleanupQueue)
	dispatcher.cleanupMu.Unlock()
	if workers != maxAckCleanupWorkers {
		t.Fatalf("cleanup workers = %d, want %d", workers, maxAckCleanupWorkers)
	}
	if queued != ackCleanupQueueCapacity {
		t.Fatalf("cleanup queue = %d, want bounded capacity %d", queued, ackCleanupQueueCapacity)
	}
	if requests := fake.emojiArrived.Load(); requests != maxAckCleanupWorkers {
		t.Fatalf("live cleanup requests = %d, want at most %d", requests, maxAckCleanupWorkers)
	}

	close(fake.emojiGate)
	dispatcher.Shutdown(0)
}

// Cleanup-only states remain admitted work even though they no longer occupy
// the review queue. They must apply backpressure during a prolonged outage.
func TestEnqueueCleanupStatesConsumeBacklogCapacity(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	t.Cleanup(dispatcher.jobCancel)
	t.Cleanup(dispatcher.cleanupCancel)

	for iid := 1; iid <= queueCapacity; iid++ {
		event := autoEvent(iid, "sha-1", group)
		cleanup := reactionCleanup{event: event, outcome: outcomeDone}
		dispatcher.states[jobKey{ProjectID: 42, IID: iid}] = &jobState{
			status:  stateCleanup,
			latest:  event,
			cleanup: &cleanup,
		}
	}
	if dispatcher.Enqueue(autoEvent(queueCapacity+1, "sha-1", group)) {
		t.Fatal("new MR accepted beyond cleanup-backed admission capacity")
	}
	if got := len(dispatcher.states); got != queueCapacity {
		t.Fatalf("states = %d, want bounded capacity %d", got, queueCapacity)
	}
}

// Retry pacing belongs to the dispatcher, not each cleanup state. During a
// fast GitLab outage, distinct backlog entries must share one exponentially
// increasing gate instead of each firing once per second.
func TestDurableCleanupBackoffIsSharedAcrossBacklog(t *testing.T) {
	failureGate := make(chan struct{})
	closeFailureGate := sync.OnceFunc(func() { close(failureGate) })
	defer closeFailureGate()
	fake := &fakeGitLab{
		topics:             []string{"nickpit"},
		state:              "opened",
		headSHA:            "sha-1",
		emojiFailureStatus: http.StatusServiceUnavailable,
		emojiFailureGate:   failureGate,
	}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.cleanupRetryInitial = 30 * time.Millisecond
	dispatcher.cleanupRetryMax = 60 * time.Millisecond
	dispatcher.cleanupRetryJitter = func(delay, _ time.Duration) time.Duration { return delay }

	var keys []jobKey
	for iid := 1; iid <= maxAckCleanupWorkers+1; iid++ {
		event := commandEvent(iid, "sha-1", group, iid*100+1)
		cleanup := reactionCleanup{event: event, outcome: outcomeAborted}
		key := jobKey{ProjectID: 42, IID: iid}
		keys = append(keys, key)
		dispatcher.states[key] = &jobState{status: stateCleanup, latest: event, cleanup: &cleanup}
	}
	for _, key := range keys {
		dispatcher.queueDurableCleanup(key)
	}
	// Hold every initially admitted request at the fake server until all four
	// timestamps exist. This makes slots 0-3 a deterministic initial wave even
	// when race instrumentation delays individual goroutines.
	waitFor(t, time.Second, func() bool { return len(fake.emojiRequestTimes()) == maxAckCleanupWorkers })
	dispatcher.cleanupMu.Lock()
	active := dispatcher.cleanupAttemptsActive
	dispatcher.cleanupMu.Unlock()
	if active != maxAckCleanupWorkers {
		t.Fatalf("active cleanup attempts = %d, want initial wave of %d", active, maxAckCleanupWorkers)
	}
	closeFailureGate()
	waitFor(t, time.Second, func() bool { return len(fake.emojiRequestTimes()) >= maxAckCleanupWorkers+1 })
	times := fake.emojiRequestTimes()
	firstAfterWave := times[maxAckCleanupWorkers]
	lastInWave := times[maxAckCleanupWorkers-1]
	if gap := firstAfterWave.Sub(lastInWave); gap < 50*time.Millisecond {
		t.Fatalf("shared retry gap after initial bounded wave = %s, want exponential backoff near 60ms", gap)
	}

	dispatcher.CloseIntake()
	dispatcher.cleanupCancel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dispatcher.stopAckCleanup(ctx)
	dispatcher.jobCancel()
}

func TestCleanupBackoffHonorsServerRetryAfter(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, _ := newWorkerEnv(t, fake, workerCfg())
	dispatcher.cleanupRetryInitial = time.Millisecond
	dispatcher.cleanupRetryMax = 2 * time.Millisecond
	dispatcher.cleanupRetryJitter = func(delay, _ time.Duration) time.Duration { return delay }
	retryAt := time.Now().Add(5 * time.Second)
	dispatcher.cleanupMu.Lock()
	dispatcher.cleanupAttemptsActive = 2
	dispatcher.cleanupMu.Unlock()
	dispatcher.finishAckCleanupAttempt(0, true, retryAt)
	dispatcher.cleanupMu.Lock()
	got := dispatcher.cleanupRetryUntil
	dispatcher.cleanupMu.Unlock()
	if got.Before(retryAt) {
		t.Fatalf("shared retry time = %v, want server delay through %v", got, retryAt)
	}
	// A concurrent 5xx may finish later with only a short local delay. It must
	// preserve the longer Retry-After already established by its peer.
	dispatcher.finishAckCleanupAttempt(0, true, time.Time{})
	dispatcher.cleanupMu.Lock()
	afterPeerFailure := dispatcher.cleanupRetryUntil
	dispatcher.cleanupMu.Unlock()
	if afterPeerFailure.Before(retryAt) {
		t.Fatalf("concurrent failure shortened retry-after: got %v, want at least %v", afterPeerFailure, retryAt)
	}
	// A successful peer from the same admitted wave started before this failure;
	// it must not erase the newer server instruction when it finishes later.
	dispatcher.finishAckCleanupAttempt(0, false, time.Time{})
	dispatcher.cleanupMu.Lock()
	afterPeerSuccess := dispatcher.cleanupRetryUntil
	dispatcher.cleanupMu.Unlock()
	if afterPeerSuccess.Before(retryAt) {
		t.Fatalf("concurrent success cleared retry-after: got %v, want at least %v", afterPeerSuccess, retryAt)
	}
	dispatcher.jobCancel()
	dispatcher.cleanupCancel()
}

// Repeated aborts merge pending acknowledgement sets behind durable cleanup.
// Notes omitted by the cap must be revoked instead of disappearing from state.
func TestDispatcherRepeatedAbortReleasesDroppedAck(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	key := jobKey{ProjectID: 42, IID: 7}
	event := autoEvent(7, "sha-1", group)
	cleanup := reactionCleanup{event: event, outcome: outcomeDone}
	dispatcher.states[key] = &jobState{
		status:        stateCleanup,
		latest:        event,
		cleanup:       &cleanup,
		cleanupQueued: true,
	}

	first := commandEvent(7, "sha-2", group, 0)
	first.AckNoteIDs = make([]int, maxAckNotes)
	for index := range first.AckNoteIDs {
		first.AckNoteIDs[index] = index + 1
	}
	if !dispatcher.Enqueue(first) || !dispatcher.Abort(42, 7).Found {
		t.Fatal("first pending review was not accepted and aborted")
	}

	fake.preAward(7, 9001, "eyes")
	if !dispatcher.Enqueue(commandEvent(7, "sha-3", group, 9001)) || !dispatcher.Abort(42, 7).Found {
		t.Fatal("second pending review was not accepted and aborted")
	}
	waitFor(t, 3*time.Second, func() bool { return len(fake.awardedOn(7, 9001)) == 0 })

	dispatcher.mu.Lock()
	abortedNotes := slices.Clone(dispatcher.states[key].cleanup.aborted.AckNoteIDs)
	dispatcher.mu.Unlock()
	if len(abortedNotes) != maxAckNotes || slices.Contains(abortedNotes, 9001) {
		t.Fatalf("durable aborted notes = %v, want capped original set", abortedNotes)
	}
	if revoked := fake.revokedNames(); !slices.Contains(revoked, "eyes") {
		t.Fatalf("revoked = %v, want overflow acknowledgement released", revoked)
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

// Aborting a QUEUED review must release its acknowledged notes: process/settle
// never run for it, so nothing else would ever take the ack reaction back.
func TestDispatcherAbortQueuedReleasesAckNotes(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	fake.preAward(7, 301, "eyes")
	dispatcher.Enqueue(commandEvent(7, "sha-1", group, 301))

	if outcome := dispatcher.Abort(42, 7); !outcome.Found || outcome.Running {
		t.Fatalf("outcome = %+v, want found queued job", outcome)
	}
	waitFor(t, 3*time.Second, func() bool { return len(fake.awardedOn(7, 301)) == 0 })
	if awards := fake.awarded(); len(awards) != 0 {
		t.Fatalf("awards = %v, want none after an abort", awards)
	}
}

// Aborting a RUNNING review also clears its pending re-run, whose notes were
// acknowledged too; the running job settles only its own notes, so the pending
// ones are released by the abort itself.
func TestDispatcherAbortRunningReleasesPendingAckNotes(t *testing.T) {
	env := newDispatcherEnv(t, 1, true) // gate never closed: only ctx cancel frees the runner
	env.gitlab.preAward(7, 301, "eyes")
	env.dispatcher.Enqueue(commandEvent(7, "sha-1", env.group, 301))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })
	env.gitlab.preAward(7, 302, "eyes")
	env.dispatcher.Enqueue(commandEvent(7, "sha-2", env.group, 302))

	if outcome := env.dispatcher.Abort(42, 7); !outcome.Running {
		t.Fatalf("outcome = %+v, want running abort", outcome)
	}
	// The pending note's ack is released by the abort, the running job's by its
	// own (cancelled) settle, and the MR start emoji goes with it. Nothing gets
	// an outcome: nothing went wrong.
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.awarded()) == 0 })
	if posted := env.gitlab.awardPosted(); slices.Contains(posted, "x") || slices.Contains(posted, "white_check_mark") {
		t.Fatalf("award posts = %v, want no outcome after an abort", posted)
	}
}

// Without a journal, shutdown must not strand queued jobs' acknowledged notes:
// take refuses queued jobs once closed, so their ack reaction is revoked on the
// way out instead of reading as in progress forever.
func TestDispatcherShutdownReleasesQueuedAckNotes(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	fake.preAward(7, 301, "eyes")
	dispatcher.Enqueue(commandEvent(7, "sha-1", group, 301))

	dispatcher.Shutdown(0) // no workers started: queued job can never run

	if awards := fake.awardedOn(7, 301); len(awards) != 0 {
		t.Fatalf("note awards = %v, want the ack released at shutdown", awards)
	}
	if revoked := fake.revokedNames(); len(revoked) != 1 || revoked[0] != "eyes" {
		t.Fatalf("revoked = %v, want the ack emoji", revoked)
	}
}

// Journal-less shutdown cleanup must not bypass the normal remote-request
// concurrency cap, even though each queued job carries several reactions.
func TestDispatcherShutdownReactionCleanupIsBounded(t *testing.T) {
	fake := &fakeGitLab{
		topics:    []string{"nickpit"},
		state:     "opened",
		headSHA:   "sha-1",
		emojiGate: make(chan struct{}),
	}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	closeGate := sync.OnceFunc(func() { close(fake.emojiGate) })
	defer closeGate()

	for iid := 1; iid <= maxAckCleanupWorkers+1; iid++ {
		event := commandEvent(iid, "sha-1", group, 0)
		for note := 1; note <= maxAckCleanupWorkers; note++ {
			noteID := iid*100 + note
			event.AckNoteIDs = append(event.AckNoteIDs, noteID)
			fake.preAward(iid, noteID, "eyes")
		}
		dispatcher.Enqueue(event)
	}

	done := make(chan struct{})
	go func() {
		dispatcher.Shutdown(0)
		close(done)
	}()
	waitFor(t, time.Second, func() bool { return fake.emojiArrived.Load() >= maxAckCleanupWorkers })
	time.Sleep(20 * time.Millisecond)
	if requests := fake.emojiArrived.Load(); requests != maxAckCleanupWorkers {
		t.Fatalf("live shutdown cleanup requests = %d, want at most %d", requests, maxAckCleanupWorkers)
	}

	closeGate()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown cleanup did not drain")
	}
	if awards := fake.awarded(); len(awards) != 0 {
		t.Fatalf("awards = %v, want all shutdown acknowledgements released", awards)
	}
}

// All journal-less fallback cleanup shares one deadline. A large queue and a
// stalled GitLab must not multiply the per-request timeout across batches.
func TestDispatcherReleaseUnfinishedStopsAtDeadline(t *testing.T) {
	fake := &fakeGitLab{
		topics:             []string{"nickpit"},
		state:              "opened",
		headSHA:            "sha-1",
		emojiWaitForCancel: true,
	}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	for iid := 1; iid <= maxAckCleanupWorkers+1; iid++ {
		event := commandEvent(iid, "sha-1", group, 0)
		for note := 1; note <= maxAckCleanupWorkers; note++ {
			event.AckNoteIDs = append(event.AckNoteIDs, iid*100+note)
		}
		dispatcher.Enqueue(event)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	dispatcher.releaseUnfinished(ctx)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("unfinished cleanup took %s, want shared deadline", elapsed)
	}
	if requests := fake.emojiArrived.Load(); requests != maxAckCleanupWorkers {
		t.Fatalf("cleanup requests = %d, want only initial bounded batch", requests)
	}
	dispatcher.jobCancel()
	dispatcher.cleanupCancel()
}

func TestStopAckCleanupCancelsBacklogAtDeadline(t *testing.T) {
	fake := &fakeGitLab{
		topics:             []string{"nickpit"},
		state:              "opened",
		headSHA:            "sha-1",
		emojiWaitForCancel: true,
	}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	event := commandEvent(7, "sha-1", group, 1)
	notes := make([]int, ackCleanupQueueCapacity)
	for index := range notes {
		notes[index] = index + 1
	}
	dispatcher.queueAckCleanup(event, notes)
	waitFor(t, time.Second, func() bool { return fake.emojiArrived.Load() == maxAckCleanupWorkers })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	dispatcher.stopAckCleanup(ctx)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup stop took %s, want bounded cancellation", elapsed)
	}
	dispatcher.cleanupMu.Lock()
	workers := dispatcher.cleanupWorkers
	dispatcher.cleanupMu.Unlock()
	if workers != 0 {
		t.Fatalf("cleanup workers = %d, want 0 after stop", workers)
	}
	dispatcher.jobCancel()
}

// A stalled background cleanup must not consume the deadline needed to revoke
// acknowledgements for queued journal-less jobs.
func TestCleanupOnShutdownReservesTimeForUnfinishedJobs(t *testing.T) {
	stalledPath := "/api/v4/projects/42/merge_requests/7/notes/301/award_emoji"
	fake := &fakeGitLab{
		topics:                  []string{"nickpit"},
		state:                   "opened",
		headSHA:                 "sha-1",
		emojiWaitForCancelPaths: map[string]bool{stalledPath: true},
	}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	fake.preAward(7, 301, "eyes")
	dispatcher.queueAckCleanup(commandEvent(7, "sha-1", group, 301), []int{301})
	waitFor(t, time.Second, func() bool { return fake.emojiArrived.Load() == 1 })

	fake.preAward(8, 401, "eyes")
	dispatcher.Enqueue(commandEvent(8, "sha-1", group, 401))
	dispatcher.cleanupOnShutdown(200 * time.Millisecond)

	if awards := fake.awardedOn(8, 401); len(awards) != 0 {
		t.Fatalf("queued note awards = %v, want acknowledgement revoked with reserved budget", awards)
	}
	if revoked := fake.revokedNames(); !slices.Contains(revoked, "eyes") {
		t.Fatalf("revoked = %v, want queued acknowledgement cleanup", revoked)
	}
	dispatcher.jobCancel()
}

// A cleanup state may also carry a newer pending review. If neither state was
// journaled, shutdown must settle old work and separately revoke pending acks.
func TestDispatcherShutdownCleanupFallbackReleasesPendingAckNotes(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	fake.preAward(7, 301, "eyes")
	fake.preAward(7, 302, "eyes")
	oldEvent := commandEvent(7, "sha-1", group, 301)
	oldEvent.AckEmojis = []string{"eyes"}
	pending := commandEvent(7, "sha-2", group, 302)
	pending.AckEmojis = []string{"eyes"}
	key := jobKey{ProjectID: 42, IID: 7}
	dispatcher.states[key] = &jobState{
		status:  stateCleanup,
		latest:  oldEvent,
		pending: &pending,
		cleanup: &reactionCleanup{event: oldEvent, outcome: outcomeDone},
	}

	dispatcher.Shutdown(0)
	if awards := fake.awardedOn(7, 301); !slices.Equal(awards, []string{"white_check_mark"}) {
		t.Fatalf("old cleanup awards = %v, want done outcome", awards)
	}
	if awards := fake.awardedOn(7, 302); len(awards) != 0 {
		t.Fatalf("pending awards = %v, want acknowledgement revoked", awards)
	}
}

// Journal-less cleanup cannot hand an uncertain timed-out award to a restart.
// Shutdown must stay alive through the uncertainty deadline and perform the
// final revoke after GitLab commits the late POST.
func TestDispatcherShutdownCleanupFallbackSweepsLateUncertainAck(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	deadline := time.Now().Add(100 * time.Millisecond)
	event := commandEvent(7, "sha-1", group, 301)
	event.AckEmojis = []string{"eyes"}
	event.UncertainAckNoteIDs = []int{301}
	event.AckCleanupUntil = deadline
	cleanup := reactionCleanup{event: event, outcome: outcomeAborted}
	key := jobKey{ProjectID: 42, IID: 7}
	dispatcher.states[key] = &jobState{
		status:  stateCleanup,
		latest:  event,
		cleanup: &cleanup,
	}

	lateCommit := time.AfterFunc(40*time.Millisecond, func() { fake.preAward(7, 301, "eyes") })
	defer lateCommit.Stop()
	started := time.Now()
	dispatcher.cleanupOnShutdown(40 * time.Millisecond)
	if elapsed := time.Since(started); elapsed < 80*time.Millisecond {
		t.Fatalf("shutdown fallback returned after %s, before uncertainty deadline", elapsed)
	}
	if awards := fake.awardedOn(7, 301); len(awards) != 0 {
		t.Fatalf("late acknowledgement awards = %v, want final sweep to revoke them", awards)
	}
	if revoked := fake.revokedNames(); !slices.Contains(revoked, "eyes") {
		t.Fatalf("revoked = %v, want late acknowledgement revoked", revoked)
	}
	dispatcher.jobCancel()
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

// Enqueue reports acceptance: queued, coalesced, and deliberate dedup drops
// are accepted; a full queue and a closed dispatcher are rejections the
// handler turns into a 503 so GitLab redelivers.
func TestEnqueueReportsAcceptance(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())

	if !dispatcher.Enqueue(autoEvent(1, "sha-1", group)) {
		t.Fatal("fresh enqueue must be accepted")
	}
	if !dispatcher.Enqueue(autoEvent(1, "sha-2", group)) {
		t.Fatal("coalescing onto a queued job must be accepted")
	}
	// An already-reviewed head is dropped on purpose; a redelivery would only
	// be dropped again, so it counts as accepted (no 503, no GitLab retry).
	dispatcher.markReviewed(42, 2, "sha-x")
	if !dispatcher.Enqueue(autoEvent(2, "sha-x", group)) {
		t.Fatal("dedup drop must be accepted")
	}
}

func TestEnqueueRejectsWhenQueueFull(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	// Fill the queue channel directly (no workers are draining it).
	for len(dispatcher.queue) < cap(dispatcher.queue) {
		dispatcher.queue <- jobKey{ProjectID: -1, IID: len(dispatcher.queue)}
	}
	if dispatcher.Enqueue(autoEvent(7, "sha-1", group)) {
		t.Fatal("full queue must reject the event")
	}
	dispatcher.mu.Lock()
	dropped := dispatcher.dropped
	dispatcher.mu.Unlock()
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}
}

func TestEnqueueRejectsAfterShutdown(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.Shutdown(0) // no workers started: returns immediately, marks closed
	if dispatcher.Enqueue(autoEvent(7, "sha-1", group)) {
		t.Fatal("closed dispatcher must reject the event")
	}
}

// CloseIntake runs before the HTTP drain: handlers still in flight during the
// drain must have their events rejected (503 → GitLab redelivers), because
// the workers have already stopped and would never process them.
func TestEnqueueRejectsAfterCloseIntake(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.CloseIntake()
	if dispatcher.Enqueue(autoEvent(7, "sha-1", group)) {
		t.Fatal("dispatcher with closed intake must reject the event")
	}
	dispatcher.Shutdown(0) // idempotent after CloseIntake
	if dispatcher.Enqueue(autoEvent(7, "sha-2", group)) {
		t.Fatal("closed dispatcher must reject the event")
	}
}

// A pending re-run re-queues at the back, behind other waiting MRs: a busy MR
// must not monopolize a worker by having its re-runs jump the queue.
func TestDispatcherPendingRerunDoesNotStarveQueuedMRs(t *testing.T) {
	env := newDispatcherEnv(t, 1, true)
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	// While MR 7 runs: MR 8 queues up, then a new head for MR 7 parks as its
	// pending re-run.
	env.dispatcher.Enqueue(autoEvent(8, "sha-1", env.group))
	env.gitlab.mu.Lock()
	env.gitlab.headSHA = "sha-2"
	env.gitlab.mu.Unlock()
	env.dispatcher.Enqueue(autoEvent(7, "sha-2", env.group))

	close(env.runner.gate)
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 3 })
	var iids []int
	for _, spec := range env.runner.ran() {
		iids = append(iids, spec.IID)
	}
	if iids[0] != 7 || iids[1] != 8 || iids[2] != 7 {
		t.Fatalf("run order = %v, want [7 8 7] (re-run behind queued MR 8)", iids)
	}
}

// A pending re-run was acknowledged with a 2xx at enqueue time, so it must
// not be dropped when the queue happens to be full at finish time: it parks
// in the overflow and re-enters the queue as slots free up.
func TestDispatcherPendingRerunSurvivesFullQueue(t *testing.T) {
	env := newDispatcherEnv(t, 1, true)
	env.dispatcher.Enqueue(autoEvent(7, "sha-1", env.group))
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) == 1 })

	// New head arrives while MR 7 is being reviewed → pending re-run.
	env.gitlab.mu.Lock()
	env.gitlab.headSHA = "sha-2"
	env.gitlab.mu.Unlock()
	env.dispatcher.Enqueue(autoEvent(7, "sha-2", env.group))

	// Fill the queue channel completely while the single worker is busy, so a
	// re-queue of the pending event would be impossible.
	env.dispatcher.mu.Lock()
	for len(env.dispatcher.queue) < cap(env.dispatcher.queue) {
		env.dispatcher.queue <- jobKey{ProjectID: -1, IID: len(env.dispatcher.queue)}
	}
	env.dispatcher.mu.Unlock()

	close(env.runner.gate)
	waitFor(t, 3*time.Second, func() bool { return len(env.runner.ran()) >= 2 })
	second := env.runner.ran()[1]
	if second.IID != 7 || second.HeadSHA != "sha-2" {
		t.Fatalf("second run = IID %d sha %q, want the pending re-run of MR 7 at sha-2", second.IID, second.HeadSHA)
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
