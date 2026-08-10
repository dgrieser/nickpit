package serve

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
)

// newWorkerEnv builds a dispatcher whose fake GitLab state the test controls,
// without started workers: tests drive process() directly for determinism.
func newWorkerEnv(t *testing.T, fake *fakeGitLab, cfg WorkerConfig) (*Dispatcher, *fakeRunner, *Group) {
	t.Helper()
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	group := newTestGroupSetWithURL(t, server.URL).Match("platform/api")
	runner := &fakeRunner{}
	topics := TopicLookup(GitLabTopicLookup)
	if cfg.BaseURL == "" {
		cfg.BaseURL = server.URL
	}
	if cfg.LogDir == "" {
		cfg.LogDir = t.TempDir()
	}
	dispatcher := NewDispatcher(runner, topics, nil, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return dispatcher, runner, group
}

func workerCfg() WorkerConfig {
	return WorkerConfig{
		Topic:      "nickpit",
		StartEmoji: "eyes",
		AckEmoji:   "eyes",
		DoneEmoji:  "white_check_mark",
		FailEmoji:  "x",
	}
}

func TestWorkerTopicMissNoRun(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"go"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 0 {
		t.Fatal("review must not run without opt-in topic")
	}
	// awardPosted, not awarded: an award revoked again at settle time would
	// leave the live view empty, yet the daemon must not have decorated (and
	// notified every watcher of) an MR it skipped, even transiently.
	if posted := fake.awardPosted(); len(posted) != 0 {
		t.Fatalf("award posts = %v, want none without opt-in topic", posted)
	}
}

func TestWorkerManualTriggerSkipsTopicCheck(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"go"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	event := autoEvent(7, "sha-1", group)
	event.Kind = TriggerManual
	dispatcher.process(context.Background(), event)
	if len(runner.ran()) != 1 {
		t.Fatal("manual trigger must run without topic")
	}
	if fake.topicGET != 0 {
		t.Fatal("manual trigger must not query topics")
	}
}

func TestWorkerClosedMRSkipped(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "merged", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	for _, kind := range []TriggerKind{TriggerAuto, TriggerManual} {
		event := autoEvent(7, "sha-1", group)
		event.Kind = kind
		dispatcher.process(context.Background(), event)
	}
	if len(runner.ran()) != 0 {
		t.Fatal("closed/merged MR must never run")
	}
}

func TestWorkerDraftRecheckSkipsAutoNotManual(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", draft: true, headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())

	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 0 {
		t.Fatal("draft MR must not run on auto trigger")
	}

	manual := autoEvent(7, "sha-1", group)
	manual.Kind = TriggerManual
	dispatcher.process(context.Background(), manual)
	if len(runner.ran()) != 1 {
		t.Fatal("draft MR must run on manual trigger")
	}
}

// The outcome emoji REPLACES the start emoji, so disabling the start emoji
// leaves the MR undecorated end to end — no outcome reaction either.
func TestWorkerStartEmojiDisabled(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	cfg := workerCfg()
	cfg.StartEmoji = ""
	dispatcher, runner, group := newWorkerEnv(t, fake, cfg)
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 1 {
		t.Fatal("review must run")
	}
	// awardPosted, not awarded: even an award revoked again at settle time
	// would have decorated the MR transiently and must fail this test.
	if posted := fake.awardPosted(); len(posted) != 0 {
		t.Fatalf("award posts = %v, want none when start_emoji disabled", posted)
	}
}

func TestWorkerUnresolvedBotIDDisablesManagedReactions(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	group.BotUserID = 0
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 1 {
		t.Fatal("review must still run for a defensive direct caller")
	}
	if posted := fake.awardPosted(); len(posted) != 0 {
		t.Fatalf("award posts = %v, want none without bot identity", posted)
	}
}

// commandEvent is a manual event that came from a "/<keyword> review" comment,
// whose note already wears the ack emoji.
func commandEvent(iid int, sha string, group *Group, noteID int) Event {
	event := autoEvent(iid, sha, group)
	event.Kind = TriggerManual
	event.AckNoteIDs = []int{noteID}
	return event
}

func TestWorkerFailedRunAwardsFailEmoji(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	runner.exit = 1

	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if awards := fake.awardedOn(7, 0); len(awards) != 1 || awards[0] != "x" {
		t.Fatalf("MR awards = %v, want the fail emoji", awards)
	}
	if revoked := fake.revokedNames(); len(revoked) != 1 || revoked[0] != "eyes" {
		t.Fatalf("revoked = %v, want the start emoji", revoked)
	}
}

// An abort is the user withdrawing the request, not a failure: the in-progress
// reaction goes away and nothing takes its place.
func TestWorkerAbortedRunOnlyRevokesStartEmoji(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	runner.gate = make(chan struct{}) // never released; only ctx cancel frees it

	ctx, cancel := context.WithCancel(dispatcher.jobCtx)
	done := make(chan struct{})
	go func() {
		dispatcher.process(ctx, commandEvent(7, "sha-1", group, 301))
		close(done)
	}()
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })
	fake.preAward(7, 301, "eyes")
	cancel()
	<-done

	if awards := fake.awarded(); len(awards) != 0 {
		t.Fatalf("awards = %v, want none after an abort", awards)
	}
	if revoked := fake.revokedNames(); len(revoked) != 2 {
		t.Fatalf("revoked = %v, want the MR and the note marker", revoked)
	}
}

// The comment that asked for the review shows how it ended, on the note itself.
func TestWorkerCommandNoteReactionFollowsOutcome(t *testing.T) {
	for _, tc := range []struct {
		name string
		exit int
		want string
	}{
		{"landed", 0, "white_check_mark"},
		{"failed", 1, "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
			dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
			runner.exit = tc.exit
			fake.preAward(7, 301, "eyes")

			dispatcher.process(context.Background(), commandEvent(7, "sha-1", group, 301))
			if awards := fake.awardedOn(7, 301); len(awards) != 1 || awards[0] != tc.want {
				t.Fatalf("note awards = %v, want %q", awards, tc.want)
			}
			if awards := fake.awardedOn(7, 0); len(awards) != 1 || awards[0] != tc.want {
				t.Fatalf("MR awards = %v, want %q", awards, tc.want)
			}
		})
	}
}

// A review command the worker then rules out (the MR closed meanwhile) must not
// leave the comment reading as "in progress" forever.
func TestWorkerSkippedCommandGetsFailEmoji(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "merged", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	fake.preAward(7, 301, "eyes")

	dispatcher.process(context.Background(), commandEvent(7, "sha-1", group, 301))
	if len(runner.ran()) != 0 {
		t.Fatal("merged MR must not run")
	}
	if awards := fake.awardedOn(7, 301); len(awards) != 1 || awards[0] != "x" {
		t.Fatalf("note awards = %v, want the fail emoji", awards)
	}
	// The MR was never marked in progress, so it is left untouched.
	if awards := fake.awardedOn(7, 0); len(awards) != 0 {
		t.Fatalf("MR awards = %v, want none", awards)
	}
}

// A re-review must not stack outcomes: awarding the start emoji clears whatever
// the previous run left behind.
func TestWorkerRerunClearsPreviousOutcome(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())

	runner.exit = 1
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if awards := fake.awardedOn(7, 0); len(awards) != 1 || awards[0] != "x" {
		t.Fatalf("MR awards after failure = %v, want the fail emoji", awards)
	}

	runner.exit = 0
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if awards := fake.awardedOn(7, 0); len(awards) != 1 || awards[0] != "white_check_mark" {
		t.Fatalf("MR awards after retry = %v, want the done emoji only", awards)
	}
}

// An abort landing while the pre-run MR status check is in flight is still the
// user's abort, not a failure: the note loses its ack and gets no outcome.
func TestWorkerAbortDuringStatusCheckIsNotAFailure(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1", statusGate: make(chan struct{})}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	fake.preAward(7, 301, "eyes")

	ctx, cancel := context.WithCancel(dispatcher.jobCtx)
	done := make(chan struct{})
	go func() {
		dispatcher.process(ctx, commandEvent(7, "sha-1", group, 301))
		close(done)
	}()
	waitFor(t, 3*time.Second, func() bool { return fake.statusArrived.Load() == 1 })
	cancel()
	<-done
	close(fake.statusGate) // release the parked fake handler

	if len(runner.ran()) != 0 {
		t.Fatal("aborted job must not run")
	}
	if posted := fake.awardPosted(); len(posted) != 0 {
		t.Fatalf("award posts = %v, want none: an abort is not a failure", posted)
	}
	if revoked := fake.revokedNames(); len(revoked) != 1 || revoked[0] != "eyes" {
		t.Fatalf("revoked = %v, want the note ack released", revoked)
	}
}

// Settle removes a stale outcome a previous run left behind: the review-start
// replace only cleans it up when its own call succeeds, and a redelivered
// command note never gets a start-time cleanup at all — without this, the MR
// or note would wear the old and the new outcome side by side.
func TestWorkerSettleClearsStaleOutcome(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	runner.gate = make(chan struct{})
	fake.preAward(7, 301, "eyes")

	done := make(chan struct{})
	go func() {
		dispatcher.process(context.Background(), commandEvent(7, "sha-1", group, 301))
		close(done)
	}()
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })
	// Leftovers the start-time cleanup missed, appearing mid-run.
	fake.preAward(7, 0, "x")
	fake.preAward(7, 301, "x")
	close(runner.gate)
	<-done

	if awards := fake.awardedOn(7, 0); len(awards) != 1 || awards[0] != "white_check_mark" {
		t.Fatalf("MR awards = %v, want the stale outcome replaced by the done emoji", awards)
	}
	if awards := fake.awardedOn(7, 301); len(awards) != 1 || awards[0] != "white_check_mark" {
		t.Fatalf("note awards = %v, want the stale outcome replaced by the done emoji", awards)
	}
}

// Every emoji off: the daemon must then touch no reactions at all, and in
// particular must not list or revoke reactions it never placed.
func TestWorkerOutcomeEmojiDisabled(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	cfg := workerCfg()
	cfg.DoneEmoji = ""
	cfg.FailEmoji = ""
	dispatcher, _, group := newWorkerEnv(t, fake, cfg)

	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	// The start emoji is still revoked when the review ends: it means
	// "in progress", and the review is not.
	if awards := fake.awarded(); len(awards) != 0 {
		t.Fatalf("awards = %v, want none", awards)
	}
	if revoked := fake.revokedNames(); len(revoked) != 1 || revoked[0] != "eyes" {
		t.Fatalf("revoked = %v, want the start emoji", revoked)
	}
}

// A failed run must not mark the head as reviewed: the next auto event for
// the same SHA has to retry instead of being dropped.
func TestWorkerFailedRunDoesNotMarkReviewed(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	runner.exit = 1

	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 1 {
		t.Fatal("review must have been attempted")
	}
	if dispatcher.alreadyReviewed(42, 7, "sha-1") {
		t.Fatal("failed run must not mark the SHA reviewed")
	}

	runner.exit = 0
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 2 {
		t.Fatal("retry after failure must run")
	}
	if !dispatcher.alreadyReviewed(42, 7, "sha-1") {
		t.Fatal("successful run must mark the SHA reviewed")
	}
}

func TestWorkerAuthoritativeSHABeatsPayload(t *testing.T) {
	// Payload carried sha-1 but the MR moved on to sha-2 before the worker
	// ran: the LRU must record sha-2 so the follow-up webhook for sha-2 is
	// deduplicated.
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-2"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 1 {
		t.Fatal("review must run")
	}
	if !dispatcher.alreadyReviewed(42, 7, "sha-2") {
		t.Fatal("authoritative head SHA must be recorded")
	}
}

// An aborted run (per-job cancel while the pool is alive) must not mark the
// head reviewed, even though the fake runner exits 0.
func TestWorkerAbortedRunNotMarkedReviewed(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	runner.gate = make(chan struct{}) // never released; only ctx cancel frees it

	ctx, cancel := context.WithCancel(dispatcher.jobCtx)
	done := make(chan struct{})
	go func() {
		dispatcher.process(ctx, autoEvent(7, "sha-1", group))
		close(done)
	}()
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })
	cancel()
	<-done
	if dispatcher.alreadyReviewed(42, 7, "sha-1") {
		t.Fatal("aborted run must not mark the SHA reviewed")
	}
}

func TestDispatcherShutdownGraceKillsChild(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	runner.gate = make(chan struct{}) // never released; only ctx cancel frees it

	ctx, cancel := context.WithCancel(context.Background())
	dispatcher.Start(ctx, 1)
	dispatcher.Enqueue(autoEvent(7, "sha-1", group))
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })

	cancel()
	done := make(chan struct{})
	go func() {
		dispatcher.Shutdown(50 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not cancel the stuck review")
	}
}
