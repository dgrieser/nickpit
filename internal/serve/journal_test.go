package serve

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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
	dispatcher.process(context.Background(), commandEvent(7, "sha-1", group, 301))
	dispatcher.finish(jobKey{ProjectID: 42, IID: 7})
	if got := journalFiles(t, dir); got != 0 {
		t.Fatalf("journal files = %d, want none after the job settled", got)
	}
}

// An abort removes the journal entry: nothing of the job may resume later.
func TestJournalRemovedOnAbort(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeGitLab{topics: []string{"nickpit"}, state: "opened", headSHA: "sha-1"}
	dispatcher, _, groups := newJournalEnv(t, fake, dir)
	group := groups.Match("platform/api")

	dispatcher.Enqueue(autoEvent(7, "sha-1", group))
	dispatcher.Abort(42, 7)
	if got := journalFiles(t, dir); got != 0 {
		t.Fatalf("journal files = %d, want none after abort", got)
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
	second.process(ctx, event)
	second.finish(key)

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
	second.process(ctx, event)
	second.finish(key)

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
		second.finish(key)
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
