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
	dispatcher := NewDispatcher(runner, topics, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return dispatcher, runner, group
}

func workerCfg() WorkerConfig {
	return WorkerConfig{Topic: "nickpit", StartEmoji: "eyes"}
}

func TestWorkerTopicMissNoRun(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"go"}, state: "opened", headSHA: "sha-1"}
	dispatcher, runner, group := newWorkerEnv(t, fake, workerCfg())
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 0 {
		t.Fatal("review must not run without opt-in topic")
	}
	if len(fake.awarded()) != 0 {
		t.Fatal("no emoji must be awarded without opt-in topic")
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

func TestWorkerStartEmojiDisabled(t *testing.T) {
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	cfg := workerCfg()
	cfg.StartEmoji = ""
	dispatcher, runner, group := newWorkerEnv(t, fake, cfg)
	dispatcher.process(context.Background(), autoEvent(7, "sha-1", group))
	if len(runner.ran()) != 1 {
		t.Fatal("review must run")
	}
	if len(fake.awarded()) != 0 {
		t.Fatalf("awards = %v, want none when start_emoji disabled", fake.awarded())
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
