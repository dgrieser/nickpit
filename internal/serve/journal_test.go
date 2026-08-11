package serve

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// newJournalEnv is newWorkerEnv plus a journal in dir, sharing the fake GitLab
// across "restarts" of the daemon.
func newJournalEnv(t *testing.T, fake *fakeGitLab, dir string) (*Dispatcher, *fakeRunner, *GroupSet) {
	return newJournalEnvWithConfig(t, fake, dir, workerCfg())
}

func newJournalEnvWithConfig(t *testing.T, fake *fakeGitLab, dir string, cfg WorkerConfig) (*Dispatcher, *fakeRunner, *GroupSet) {
	t.Helper()
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	groups := newTestGroupSetWithURL(t, server.URL)
	runner := &fakeRunner{}
	cfg.BaseURL = server.URL
	cfg.LogDir = t.TempDir()
	journal, err := NewJournal(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := NewDispatcher(runner, TopicLookup(GitLabTopicLookup), journal, cfg, discardLogger())
	return dispatcher, runner, groups
}

// A failed coalescing overwrite must invalidate the older canonical snapshot.
// Otherwise restart would resume state that predates already-acknowledged work.
func TestJournalFailedUpdateInvalidatesOlderEntry(t *testing.T) {
	dir := t.TempDir()
	journal, err := NewJournal(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	event := Event{Kind: TriggerAuto, ProjectID: 42, ProjectPath: "platform/api", IID: 7, HeadSHA: "sha-1"}
	if !journal.persist(event) {
		t.Fatal("initial persist failed")
	}
	path := journal.path(event.ProjectID, event.IID)
	if err := os.Mkdir(path+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	event.HeadSHA = "sha-2"
	if journal.persist(event) {
		t.Fatal("update unexpectedly succeeded with a directory at the temp path")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("canonical entry survived failed update: %v", err)
	}
}

func journalFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestNewJournalRejectsUnwritableExistingDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	journal, err := NewJournal(dir, discardLogger())
	if err == nil || journal != nil {
		t.Fatalf("NewJournal = (%v, %v), want startup failure for unwritable state dir", journal, err)
	}
}

func TestJournalLoadUsesLiteralStateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state[")
	journal, err := NewJournal(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	event := Event{Kind: TriggerAuto, ProjectID: 42, ProjectPath: "platform/api", IID: 7, HeadSHA: "sha-1"}
	if !journal.persist(event) {
		t.Fatal("persist failed")
	}
	entries := journal.load()
	if len(entries) != 1 || entries[0].ProjectID != 42 || entries[0].IID != 7 {
		t.Fatalf("entries = %+v, want job from literal metacharacter directory", entries)
	}
}

func TestJournalLoadPreservesFileOnReadFailure(t *testing.T) {
	dir := t.TempDir()
	journal, err := NewJournal(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	path := journal.path(42, 7)
	if err := os.Symlink("missing-target", path); err != nil {
		t.Fatal(err)
	}

	if entries := journal.load(); len(entries) != 0 {
		t.Fatalf("entries = %+v, want none from unreadable file", entries)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("unreadable journal file was removed: %v", err)
	}
}

func TestJournalLoadRemovesMalformedFile(t *testing.T) {
	dir := t.TempDir()
	journal, err := NewJournal(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	path := journal.path(42, 7)
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	if entries := journal.load(); len(entries) != 0 {
		t.Fatalf("entries = %+v, want none from malformed file", entries)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("malformed journal file survived: %v", err)
	}
}

// An accepted job is journaled; settling it removes the file again.
func TestJournalFollowsJobLifecycle(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")

	dispatcher.Enqueue(commandEvent(7, "sha-1", group, 301))
	if got := journalFiles(t, dir); got != 1 {
		t.Fatalf("journal files = %d, want 1 after enqueue", got)
	}
	key := <-dispatcher.queue
	event, ctx, ok := dispatcher.take(key)
	if !ok {
		t.Fatal("queued job could not be taken")
	}
	result := dispatcher.process(ctx, event)
	dispatcher.finish(key, result)
	if got := journalFiles(t, dir); got != 0 {
		t.Fatalf("journal files = %d, want none after the job settled", got)
	}
}

// A successful child may already have published review comments. Before the
// worker starts reaction I/O, its crash snapshot must become cleanup-only so a
// replacement daemon cannot run and publish the paid review a second time.
func TestJournalPersistsCompletionBeforeReactionSettlement(t *testing.T) {
	dir := t.TempDir()
	settleGate := make(chan struct{})
	closeSettleGate := sync.OnceFunc(func() { close(settleGate) })
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	fake.preAward(7, 301, "eyes")
	first, runner, groups := newJournalEnv(t, fake, dir)
	runner.gate = make(chan struct{})
	event := commandEvent(7, "sha-1", groups.Match("platform/api"), 301)
	if !first.Enqueue(event) {
		t.Fatal("event was not accepted")
	}
	key := <-first.queue
	taken, ctx, ok := first.take(key)
	if !ok {
		t.Fatal("queued job could not be taken")
	}
	resultCh := make(chan settlementResult, 1)
	go func() { resultCh <- first.process(ctx, taken) }()
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })

	// The start reaction has completed and the child is blocked. Gate only the
	// later settlement requests, then let the successful child return.
	fake.emojiGate = settleGate
	close(runner.gate)
	waitFor(t, 3*time.Second, func() bool { return fake.emojiArrived.Load() >= 1 })

	entries := first.journal.load()
	if len(entries) != 1 || entries[0].CleanupOutcome != "done" {
		t.Fatalf("journal entries = %+v, want cleanup-only done state during settlement", entries)
	}

	// Restore must schedule only reaction cleanup; it must not enqueue a review.
	second, secondRunner, groups := newJournalEnv(t, fake, dir)
	if restored := second.Restore(groups); restored != 1 {
		t.Fatalf("restored = %d, want one cleanup-only job", restored)
	}
	if len(second.queue) != 0 || len(secondRunner.ran()) != 0 {
		t.Fatal("cleanup-only restore must not enqueue or run a review")
	}

	closeSettleGate()
	result := <-resultCh
	first.finish(key, result)
	waitFor(t, 3*time.Second, func() bool { return journalFiles(t, dir) == 0 })
	first.Shutdown(0)
	second.Shutdown(time.Second)
}

// An abort replaces the runnable journal entry with cleanup-only state, then
// removes it after that cleanup succeeds.
func TestJournalRemovedOnAbort(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")

	dispatcher.Enqueue(autoEvent(7, "sha-1", group))
	dispatcher.Abort(42, 7)
	waitFor(t, 3*time.Second, func() bool { return journalFiles(t, dir) == 0 })
}

func TestJournalAbortBehindCleanupKeepsSeparateOutcome(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	fake.preAward(7, 301, "eyes")
	fake.preAward(7, 302, "eyes")
	first, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")
	oldEvent := commandEvent(7, "sha-1", group, 301)
	oldEvent.AckEmojis = []string{"eyes"}
	pending := commandEvent(7, "sha-2", group, 302)
	pending.AckEmojis = []string{"eyes"}
	cleanup := reactionCleanup{event: oldEvent, outcome: outcomeDone}
	key := jobKey{ProjectID: 42, IID: 7}
	first.states[key] = &jobState{
		status:         stateCleanup,
		latest:         oldEvent,
		pending:        &pending,
		cleanup:        &cleanup,
		cleanupQueued:  true,
		cleanupVersion: 1,
		persisted:      first.journal.persistCleanup(cleanup, &pending),
	}
	if outcome := first.Abort(42, 7); !outcome.Found || outcome.Running {
		t.Fatalf("abort outcome = %+v, want pending review found", outcome)
	}
	entries := first.journal.load()
	if len(entries) != 1 || entries[0].CleanupOutcome != "done" || entries[0].Pending != nil ||
		entries[0].Aborted == nil || !slices.Equal(entries[0].Aborted.AckNoteIDs, []int{302}) {
		t.Fatalf("journal entries = %+v, want done cleanup plus separately aborted note 302", entries)
	}
	first.jobCancel()
	first.cleanupCancel()

	// Restart proves separate outcome is durable: old note gets done; pending
	// note only loses acknowledgement.
	second, _, groups := newJournalEnv(t, fake, dir)
	if restored := second.Restore(groups); restored != 1 {
		t.Fatalf("restored = %d, want 1 cleanup", restored)
	}
	waitFor(t, 3*time.Second, func() bool { return journalFiles(t, dir) == 0 })
	if awards := fake.awardedOn(7, 301); !slices.Equal(awards, []string{"white_check_mark"}) {
		t.Fatalf("old cleanup awards = %v, want done outcome", awards)
	}
	if awards := fake.awardedOn(7, 302); len(awards) != 0 {
		t.Fatalf("aborted pending awards = %v, want acknowledgement revoked", awards)
	}
	second.Shutdown(0)
}

// A failed outcome replacement becomes cleanup-only durable work. Retries must
// not rerun the review, and the journal may disappear only after replacement.
func TestJournalSettlementFailureRetriesCleanupOnly(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	fake.preAward(7, 301, "eyes")
	dispatcher, runner, groups := newJournalEnv(t, fake, dir)
	runner.gate = make(chan struct{})
	event := commandEvent(7, "sha-1", groups.Match("platform/api"), 301)
	dispatcher.Enqueue(event)
	key := <-dispatcher.queue
	taken, ctx, ok := dispatcher.take(key)
	if !ok {
		t.Fatal("queued job could not be taken")
	}
	resultCh := make(chan settlementResult, 1)
	go func() { resultCh <- dispatcher.process(ctx, taken) }()
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })
	fake.emojiFailures.Store(100)
	close(runner.gate)
	result := <-resultCh
	if result.settled {
		t.Fatal("settlement must report the transient reaction failure")
	}
	dispatcher.finish(key, result)

	entries := dispatcher.journal.load()
	if len(entries) != 1 || entries[0].CleanupOutcome != "done" {
		t.Fatalf("journal entries = %+v, want cleanup-only done outcome", entries)
	}
	if got := len(runner.ran()); got != 1 {
		t.Fatalf("runs = %d, want one before cleanup retry", got)
	}

	fake.emojiFailures.Store(0)
	waitFor(t, 3*time.Second, func() bool { return journalFiles(t, dir) == 0 })
	if got := len(runner.ran()); got != 1 {
		t.Fatalf("runs = %d, cleanup retry must not rerun review", got)
	}
	for _, noteID := range []int{0, 301} {
		awards := fake.awardedOn(7, noteID)
		if len(awards) == 0 || slices.ContainsFunc(awards, func(name string) bool { return name != "white_check_mark" }) {
			t.Fatalf("awards on note %d = %v, want only done outcomes", noteID, awards)
		}
	}
	dispatcher.Shutdown(0)
}

// Abort persists cleanup-only state before its asynchronous revoke starts. A
// replacement daemon can finish that cleanup without reviving the review.
func TestJournalAbortCleanupRestoresAfterCrashWindow(t *testing.T) {
	dir := t.TempDir()
	gate := make(chan struct{})
	closeGate := sync.OnceFunc(func() { close(gate) })
	fake := &fakeGitLab{
		topics:    []string{"nickpit"},
		state:     "opened",
		headSHA:   "sha-1",
		emojiGate: gate,
	}
	fake.preAward(7, 301, "eyes")

	first, firstRunner, groups := newJournalEnv(t, fake, dir)
	t.Cleanup(closeGate)
	first.Enqueue(commandEvent(7, "sha-1", groups.Match("platform/api"), 301))
	if outcome := first.Abort(42, 7); !outcome.Found || outcome.Running {
		t.Fatalf("outcome = %+v, want queued abort", outcome)
	}
	waitFor(t, 3*time.Second, func() bool { return fake.emojiArrived.Load() >= 1 })
	entries := first.journal.load()
	if len(entries) != 1 || entries[0].CleanupOutcome != "aborted" {
		t.Fatalf("journal entries = %+v, want cleanup-only aborted state", entries)
	}

	second, secondRunner, groups := newJournalEnv(t, fake, dir)
	t.Cleanup(closeGate)
	if restored := second.Restore(groups); restored != 1 {
		t.Fatalf("restored = %d, want one cleanup", restored)
	}
	waitFor(t, 3*time.Second, func() bool { return fake.emojiArrived.Load() >= 2 })
	if len(firstRunner.ran()) != 0 || len(secondRunner.ran()) != 0 {
		t.Fatal("abort cleanup must not run a review")
	}

	closeGate()
	waitFor(t, 3*time.Second, func() bool { return journalFiles(t, dir) == 0 })
	if awards := fake.awardedOn(7, 301); len(awards) != 0 {
		t.Fatalf("note awards = %v, want ack revoked", awards)
	}
	first.Shutdown(0)
	second.Shutdown(0)
}

// Abort leaves a running state in place until its cancelled worker reaches
// finish. A newer webhook in that window must become pending behind the durable
// abort cleanup, not overwrite the journal with a runnable merged snapshot.
func TestJournalRunningAbortThenEnqueuePreservesCleanup(t *testing.T) {
	dir := t.TempDir()
	gate := make(chan struct{})
	closeGate := sync.OnceFunc(func() { close(gate) })
	fake := &fakeGitLab{
		topics:    []string{"nickpit"},
		state:     "opened",
		headSHA:   "sha-2",
		emojiGate: gate,
	}
	fake.preAward(7, 301, "eyes")
	fake.preAward(7, 302, "eyes")

	first, firstRunner, groups := newJournalEnv(t, fake, dir)
	t.Cleanup(closeGate)
	group := groups.Match("platform/api")
	if !first.Enqueue(commandEvent(7, "sha-1", group, 301)) {
		t.Fatal("initial event was not accepted")
	}
	key := <-first.queue
	if _, _, ok := first.take(key); !ok {
		t.Fatal("initial event could not be marked running")
	}
	if outcome := first.Abort(42, 7); !outcome.Found || !outcome.Running {
		t.Fatalf("abort outcome = %+v, want running abort", outcome)
	}
	if !first.Enqueue(commandEvent(7, "sha-2", group, 302)) {
		t.Fatal("new event was not accepted during abort finish window")
	}

	entries := first.journal.load()
	if len(entries) != 1 || entries[0].CleanupOutcome != "aborted" ||
		!slices.Equal(entries[0].AckNoteIDs, []int{301}) || entries[0].Pending == nil ||
		!slices.Equal(entries[0].Pending.AckNoteIDs, []int{302}) {
		t.Fatalf("journal entries = %+v, want old abort cleanup with new pending review", entries)
	}
	first.jobCancel()
	first.cleanupCancel()

	// Restart first revokes the aborted command, then promotes and runs only the
	// new command. This also proves the old note cannot inherit the new outcome.
	second, secondRunner, groups := newJournalEnv(t, fake, dir)
	if restored := second.Restore(groups); restored != 1 {
		t.Fatalf("restored = %d, want cleanup plus pending review", restored)
	}
	waitFor(t, 3*time.Second, func() bool { return fake.emojiArrived.Load() >= 1 })
	ctx, cancel := context.WithCancel(context.Background())
	second.Start(ctx, 1)
	t.Cleanup(func() {
		cancel()
		second.Shutdown(time.Second)
	})
	closeGate()
	waitFor(t, 3*time.Second, func() bool { return len(secondRunner.ran()) == 1 })
	waitFor(t, 3*time.Second, func() bool {
		return slices.Equal(fake.awardedOn(7, 302), []string{"white_check_mark"})
	})
	if len(firstRunner.ran()) != 0 {
		t.Fatal("aborted review ran before simulated crash")
	}
	if awards := fake.awardedOn(7, 301); len(awards) != 0 {
		t.Fatalf("aborted command awards = %v, want acknowledgement revoked without outcome", awards)
	}
}

// With a journal, shutdown keeps queued jobs on disk (and leaves their ack
// reactions in place — the restart resumes and settles them), instead of
// releasing them like the journal-less path does.
func TestJournalShutdownKeepsJobs(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")
	fake.preAward(7, 301, "eyes")

	dispatcher.Enqueue(commandEvent(7, "sha-1", group, 301))
	dispatcher.Shutdown(0) // no workers started: the queued job cannot run

	if got := journalFiles(t, dir); got != 1 {
		t.Fatalf("journal files = %d, want the job kept for resume", got)
	}
	if awards := fake.awardedOn(7, 301); len(awards) != 1 || awards[0] != "eyes" {
		t.Fatalf("note awards = %v, want the ack kept until the resumed run settles it", awards)
	}
}

// A configured state directory can fail after startup. Such a journal is not
// durable, so shutdown must fall back to releasing acknowledged notes instead
// of leaving them marked in progress for a restart that cannot resume them.
func TestJournalShutdownReleasesAckWhenPersistFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")
	fake.preAward(7, 301, "eyes")

	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	dispatcher.Enqueue(commandEvent(7, "sha-1", group, 301))
	dispatcher.Shutdown(0)

	if awards := fake.awardedOn(7, 301); len(awards) != 0 {
		t.Fatalf("note awards = %v, want the ack released after journal failure", awards)
	}
}

// The full restart story: a job accepted (and its note acknowledged) by one
// daemon process is picked up by the next one, runs, and settles the note.
func TestJournalRestoreResumesAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	fake.preAward(7, 301, "eyes")

	first, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")
	first.Enqueue(commandEvent(7, "sha-1", group, 301))
	first.Shutdown(0) // "the pod is replaced"

	second, runner, groups := newJournalEnv(t, fake, dir)
	if resumed := second.Restore(groups); resumed != 1 {
		t.Fatalf("resumed = %d, want 1", resumed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	second.Start(ctx, 1)
	t.Cleanup(func() {
		cancel()
		second.Shutdown(2 * time.Second)
	})
	waitFor(t, 3*time.Second, func() bool { return len(runner.ran()) == 1 })
	if kind := runner.ran()[0].Trigger; kind != "manual" {
		t.Fatalf("trigger = %q, want the manual kind to survive the restart", kind)
	}
	// The resumed run settles the note the previous process acknowledged.
	waitFor(t, 3*time.Second, func() bool {
		awards := fake.awardedOn(7, 301)
		return len(awards) == 1 && awards[0] == "white_check_mark"
	})
	waitFor(t, 3*time.Second, func() bool { return journalFiles(t, dir) == 0 })
}

// Reaction names belong to the job that placed them, not the config active at
// restore time. A resumed run must remove old start/ack names while using the
// current outcome name.
func TestJournalRestoreCleansReactionNamesFromOldConfig(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	fake.preAward(7, 0, "old-start")
	fake.preAward(7, 301, "old-ack")

	oldCfg := workerCfg()
	oldCfg.StartEmoji = "old-start"
	oldCfg.AckEmoji = "old-ack"
	first, _, groups := newJournalEnvWithConfig(t, fake, dir, oldCfg)
	first.Enqueue(commandEvent(7, "sha-1", groups.Match("platform/api"), 301))
	key := <-first.queue
	event, _, ok := first.take(key)
	if !ok {
		t.Fatal("old-config job could not be marked running")
	}
	// Simulate the last local step before the old worker's remote request. The
	// preAward above represents that request succeeding before the crash.
	first.recordStartReactionAttempt(&event)
	entries := first.journal.load()
	if len(entries) != 1 || !slices.Contains(entries[0].StartEmojis, "old-start") || !slices.Contains(entries[0].AckEmojis, "old-ack") {
		t.Fatalf("journaled reaction names = %+v, want old start and ack", entries)
	}

	newCfg := workerCfg()
	newCfg.StartEmoji = "new-start"
	newCfg.AckEmoji = "new-ack"
	second, _, groups := newJournalEnvWithConfig(t, fake, dir, newCfg)
	if resumed := second.Restore(groups); resumed != 1 {
		t.Fatalf("resumed = %d, want 1", resumed)
	}
	key = <-second.queue
	event, ctx, ok := second.take(key)
	if !ok {
		t.Fatal("restored job could not be taken")
	}
	result := second.process(ctx, event)
	second.finish(key, result)

	if awards := fake.awardedOn(7, 0); len(awards) != 1 || awards[0] != "white_check_mark" {
		t.Fatalf("MR awards = %v, want only current outcome", awards)
	}
	if awards := fake.awardedOn(7, 301); len(awards) != 1 || awards[0] != "white_check_mark" {
		t.Fatalf("note awards = %v, want only current outcome", awards)
	}
}

// A crash after take but before policy checks finish must not turn the
// configured start-emoji name into evidence that a request reached GitLab.
// The restored skipped job therefore leaves the MR wholly undecorated.
func TestJournalRestoreBeforeStartAttemptDoesNotDecorateSkippedAuto(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"go"}, state: "opened", headSHA: "sha-1"}

	first, _, groups := newJournalEnv(t, fake, dir)
	first.Enqueue(autoEvent(7, "sha-1", groups.Match("platform/api")))
	key := <-first.queue
	if _, _, ok := first.take(key); !ok {
		t.Fatal("queued job could not be marked running")
	}
	entries := first.journal.load()
	if len(entries) != 1 || len(entries[0].StartEmojis) != 0 {
		t.Fatalf("journal entries = %+v, want no start reaction before API attempt", entries)
	}
	first.jobCancel() // simulate process loss without settling or removing state

	second, runner, groups := newJournalEnv(t, fake, dir)
	if resumed := second.Restore(groups); resumed != 1 {
		t.Fatalf("resumed = %d, want 1", resumed)
	}
	key = <-second.queue
	event, ctx, ok := second.take(key)
	if !ok {
		t.Fatal("restored job could not be taken")
	}
	result := second.process(ctx, event)
	second.finish(key, result)

	if len(runner.ran()) != 0 {
		t.Fatal("unopted restored review must not run")
	}
	if posted := fake.awardPosted(); len(posted) != 0 {
		t.Fatalf("award posts = %v, want none before or after restore", posted)
	}
}

// Running and pending events share one crash snapshot. If their combined note
// set exceeds the cap, every omitted acknowledgement must be released now;
// after a crash no restored state could settle it.
func TestJournalRunningSnapshotReleasesOmittedAck(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")
	first := commandEvent(7, "sha-1", group, 0)
	first.AckNoteIDs = make([]int, maxAckNotes)
	for i := range first.AckNoteIDs {
		first.AckNoteIDs[i] = i + 1
	}
	dispatcher.Enqueue(first)
	key := <-dispatcher.queue
	if _, _, ok := dispatcher.take(key); !ok {
		t.Fatal("queued job could not be marked running")
	}

	fake.preAward(7, 9001, "eyes")
	dispatcher.Enqueue(commandEvent(7, "sha-2", group, 9001))
	waitFor(t, 3*time.Second, func() bool { return len(fake.awardedOn(7, 9001)) == 0 })

	dispatcher.mu.Lock()
	pending := slices.Clone(dispatcher.states[key].pending.AckNoteIDs)
	dispatcher.mu.Unlock()
	if slices.Contains(pending, 9001) {
		t.Fatalf("pending notes = %v, omitted note must not be settled twice", pending)
	}
	entries := dispatcher.journal.load()
	if len(entries) != 1 || slices.Contains(entries[0].AckNoteIDs, 9001) {
		t.Fatalf("journal entries = %+v, omitted note must not appear in crash snapshot", entries)
	}
}

// Restore runs before workers start, so more journal entries than queue slots
// must be parked in overflow and admitted as workers free those slots. Leaving
// excess files only on disk would strand them until another daemon restart.
func TestJournalRestoreBeyondQueueCapacity(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	first, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")
	const excess = 3
	total := queueCapacity + excess
	for iid := 1; iid <= total; iid++ {
		if !first.journal.persist(autoEvent(iid, "sha-1", group)) {
			t.Fatalf("persist job %d failed", iid)
		}
	}

	second, _, groups := newJournalEnv(t, fake, dir)
	if resumed := second.Restore(groups); resumed != total {
		t.Fatalf("resumed = %d, want %d", resumed, total)
	}
	if got := len(second.queue); got != queueCapacity {
		t.Fatalf("queue length = %d, want %d", got, queueCapacity)
	}
	if got := len(second.overflow); got != excess {
		t.Fatalf("overflow length = %d, want %d", got, excess)
	}

	for range total {
		key := <-second.queue
		if _, _, ok := second.take(key); !ok {
			t.Fatalf("restored job %+v could not be taken", key)
		}
		second.finish(key, settlementResult{settled: true})
	}
	if got := len(second.overflow); got != 0 {
		t.Fatalf("overflow length after drain = %d, want 0", got)
	}
	if got := journalFiles(t, dir); got != 0 {
		t.Fatalf("journal files after drain = %d, want 0", got)
	}
}

// A journaled job whose group vanished from the config cannot run; it is
// dropped (with its file) instead of poisoning every restart.
func TestJournalRestoreDropsUnmatchedGroup(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")
	event := autoEvent(7, "sha-1", group)
	event.ProjectPath = "elsewhere/tool"
	dispatcher.Enqueue(event)

	second, _, _ := newJournalEnv(t, fake, dir)
	if resumed := second.Restore(newTestGroupSet(t, nil)); resumed != 0 {
		t.Fatalf("resumed = %d, want 0", resumed)
	}
	if got := journalFiles(t, dir); got != 0 {
		t.Fatalf("journal files = %d, want the unmatched job dropped", got)
	}
}
