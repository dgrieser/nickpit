package serve

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
)

func testUpdateJob() *UpdateJob {
	job := &UpdateJob{ProjectPath: "group/project", BaseURL: "https://gitlab.example", IID: 1, ReviewID: "review", DiscussionID: "thread", NoteID: 2, Reason: "Evidence", Created: time.Now()}
	job.SetID()
	return job
}

func TestUpdateStoreDurabilityAndRetirement(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "journal")
	store, err := NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	job := testUpdateJob()
	if err := store.Save(job); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	store, err = NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	jobs, err := store.Pending()
	if err != nil || len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("restart lost job: %+v %v", jobs, err)
	}
	job.Followup, job.Done = "Checked; unchanged.", true
	if err := store.Save(job); err != nil {
		t.Fatal(err)
	}
	jobs, err = store.Pending()
	if err != nil || len(jobs) != 0 {
		t.Fatalf("completed job replayed: %+v %v", jobs, err)
	}
	info, err := os.Stat(filepath.Join(dir, "update-"+job.ID+".json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private state permissions: %v %v", info, err)
	}
	if _, err := store.Load("../escape"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestUpdateStoreFailedWriteKeepsCheckpoint(t *testing.T) {
	store, err := NewUpdateStore(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	job := testUpdateJob()
	if err := store.Save(job); err != nil {
		t.Fatal(err)
	}
	dir := store.journal.Dir()
	_ = store.Close()
	job.Done = true
	if err := store.Save(job); err == nil {
		t.Fatal("closed storage acknowledged write")
	}
	reopened, err := NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	loaded, err := reopened.Load(job.ID)
	if err != nil || loaded.Done {
		t.Fatalf("failed write destroyed checkpoint: %+v %v", loaded, err)
	}
}

type updateWorkerRunner struct{ started chan ChatSpec }

func (r *updateWorkerRunner) RunChat(ctx context.Context, spec ChatSpec) (int, string, error) {
	r.started <- spec
	<-ctx.Done()
	return -1, "", ctx.Err()
}

func TestUpdateWorkerRestoresJobsAndUsesCurrentCredentials(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "journal")
	store, err := NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	job := testUpdateJob()
	if err := store.Save(job); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	groups, _ := NewGroupSet(context.Background(), []config.ServeGroup{{Path: "group", Token: "current-token"}}, job.BaseURL, nil)
	runner := &updateWorkerRunner{started: make(chan ChatSpec, 1)}
	h := NewHandler(groups, nil, HandlerConfig{}, runner, ChatConfig{BaseURL: job.BaseURL, UpdateStateDir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.StartUpdateWorker()
	select {
	case spec := <-runner.started:
		if spec.UpdateJobID != job.ID || spec.Token != "current-token" || spec.UpdateStateDir != dir {
			t.Fatalf("wrong restored job: %+v", spec)
		}
	case <-time.After(3 * time.Second):
		h.ShutdownChats(0)
		t.Fatal("durable job not resumed")
	}
	h.ShutdownChats(0)
	store, err = NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	loaded, err := store.Load(job.ID)
	if err != nil || loaded.Done {
		t.Fatalf("shutdown discarded job: %+v %v", loaded, err)
	}
}
