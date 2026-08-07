package serve

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// newJournalEnv is newWorkerEnv plus a journal in dir, sharing the fake GitLab
// across "restarts" of the daemon.
func newJournalEnv(t *testing.T, fake *fakeGitLab, dir string) (*Dispatcher, *fakeRunner, *GroupSet) {
	t.Helper()
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	groups := newTestGroupSetWithURL(t, server.URL)
	runner := &fakeRunner{}
	cfg := workerCfg()
	cfg.BaseURL = server.URL
	cfg.LogDir = t.TempDir()
	journal, err := NewJournal(dir, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := NewDispatcher(runner, TopicLookup(GitLabTopicLookup), journal, cfg, discardLogger())
	return dispatcher, runner, groups
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
