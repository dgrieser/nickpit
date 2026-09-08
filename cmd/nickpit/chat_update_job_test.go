package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/serve"
)

type updateJobTestServer struct {
	awards          []map[string]any
	failEyesRemoval bool
	notes           []glscm.DiscussionNote
	posts           int
	failAfterPost   bool
	onPost          func()
}

func TestReviewUpdatesRequireDurableState(t *testing.T) {
	for _, tc := range []struct {
		name, dir string
		want      bool
	}{
		{"disabled without state", "", false},
		{"blank state", "  ", false},
		{"durable queue enabled", filepath.Join(t.TempDir(), "journal"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := &gitLabChatUpdate{opts: chatOptions{updateStateDir: tc.dir}}
			if got := u.discussionUpdateHandler() != nil; got != tc.want {
				t.Fatalf("update tool enabled=%v want=%v", got, tc.want)
			}
			if _, err := (&gitLabUpdateExecution{gitLabChatUpdate: u}).run(context.Background(), review.ReviewUpdateSignal{}); err == nil {
				t.Fatal("synchronous execution without a durable job accepted")
			}
		})
	}
}

func (s *updateJobTestServer) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if strings.HasSuffix(r.URL.Path, "/award_emoji") {
		if r.Method == http.MethodGet {
			write(s.awards)
			return
		}
		if r.Method == http.MethodPost {
			for _, award := range s.awards {
				if award["id"] == 77 {
					w.WriteHeader(400)
					write(map[string]string{"message": "Award Emoji Name has already been taken"})
					return
				}
			}
			s.awards = append(s.awards, map[string]any{"id": 77, "name": "eyes", "user": map[string]int{"id": 7}})
			write(map[string]int{"id": 77})
			return
		}
	}
	if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/award_emoji/") {
		if !strings.HasSuffix(r.URL.Path, "/77") {
			w.WriteHeader(403)
			return
		}
		if s.failEyesRemoval {
			s.failEyesRemoval = false
			w.WriteHeader(500)
			return
		}
		for i, award := range s.awards {
			if award["id"] == 77 {
				s.awards = append(s.awards[:i], s.awards[i+1:]...)
				break
			}
		}
		w.WriteHeader(204)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/user") {
		write(map[string]any{"id": 7})
		return
	}
	notes := []any{}
	for _, n := range s.notes {
		notes = append(notes, map[string]any{"id": n.ID, "body": n.Body, "author": map[string]any{"id": n.AuthorID}})
	}
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/discussions/thread"):
		write(map[string]any{"id": "thread", "notes": notes})
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/discussions"):
		write([]any{map[string]any{"id": "thread", "notes": notes}})
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/notes"):
		write(notes)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/discussions/thread/notes"):
		if s.onPost != nil {
			s.onPost()
		}
		var body struct {
			Body string `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.posts++
		s.notes = append(s.notes, glscm.DiscussionNote{ID: 10 + s.posts, AuthorID: 7, Body: body.Body})
		if s.failAfterPost {
			s.failAfterPost = false
			w.WriteHeader(500)
			return
		}
		write(map[string]any{"id": 10 + s.posts})
	default:
		w.WriteHeader(404)
	}
}

func TestUpdateEnqueueReturnsStatusWithoutPostingAndFollowupResumes(t *testing.T) {
	result := &model.ReviewResult{ReviewID: "review", Findings: []model.Finding{{ID: "finding"}}}
	root, _ := reviewmd.NewRenderer("").ForReview("review").FindingBodyCarried(result.Findings[0], "")
	s := &updateJobTestServer{notes: []glscm.DiscussionNote{{ID: 1, AuthorID: 7, Body: root}, {ID: 2, AuthorID: 8, Body: "Check the guard."}}, failAfterPost: true}
	server := httptest.NewServer(http.HandlerFunc(s.handle))
	defer server.Close()
	dir := filepath.Join(t.TempDir(), "journal")
	store, err := serve.NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	s.onPost = func() {
		jobs, err := store.Pending()
		if err != nil || len(jobs) != 1 {
			t.Errorf("ack before durable acceptance: %+v %v", jobs, err)
		}
	}
	profile := config.Profile{GitLabBaseURL: server.URL, GitLabToken: "token"}
	adapter := glscm.NewAdapter(glscm.NewClient(server.URL, "token"), "")
	opts := chatOptions{repo: "g/p", mrID: 1, replyDiscussion: "thread", replyNote: 2, updateStateDir: dir}
	u := &gitLabChatUpdate{app: &app{}, adapter: adapter, profile: profile, project: "g/p", iid: 1, botUserID: 7, pending: 2, opts: opts,
		request: review.DiscussRequest{Result: result}, triggerNotes: append([]glscm.DiscussionNote(nil), s.notes...)}
	outcome, err := u.enqueue(context.Background(), review.ReviewUpdateSignal{FindingIDs: []string{"finding"}, Reason: "Check guard."})
	if err != nil || outcome.Status != review.ReviewUpdateScheduled {
		t.Fatalf("enqueue: %+v %v", outcome, err)
	}
	defer func() {
		if u.queuedRelease != nil {
			u.queuedRelease()
		}
	}()
	jobs, err := store.Pending()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job missing: %+v %v", jobs, err)
	}
	job := &jobs[0]
	if s.posts != 0 {
		t.Fatal("enqueue posted a fixed acknowledgement")
	}
	lockCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	_, release, lockErr := adapter.Client().LockMR(lockCtx, "update-job/"+job.ID, 1)
	cancel()
	if lockErr == nil {
		release()
		t.Fatal("worker admitted before chat reply finished")
	}
	u.queuedRelease()
	u.queuedRelease = nil
	u.queuedJob = nil
	u.opts.updateStateDir = ""
	failed, err := u.enqueue(context.Background(), review.ReviewUpdateSignal{FindingIDs: []string{"finding"}, Reason: "Check guard."})
	if err != nil || failed.Status != review.ReviewUpdateQueueFailed || s.posts != 0 {
		t.Fatalf("queue failure: %+v %v posts=%d", failed, err, s.posts)
	}
	// Simulate restart after evaluation, before follow-up delivery. No LLM is
	// configured: a saved result must be delivered without re-evaluation.
	job.Followup = "Checked the guard; the finding remains valid."
	if err := store.Save(job); err != nil {
		t.Fatal(err)
	}
	opts.updateJobID = job.ID
	s.failAfterPost = true
	a := &app{}
	_ = a.runUpdateJob(context.Background(), profile, opts)
	if err := a.runUpdateJob(context.Background(), profile, opts); err != nil {
		t.Fatal(err)
	}
	if s.posts != 1 {
		t.Fatalf("follow-up duplicated: %d posts", s.posts)
	}
	loaded, err := store.Load(job.ID)
	if err != nil || !loaded.Done {
		t.Fatalf("completion not durable: %+v %v", loaded, err)
	}
}

func TestAsyncFollowupDoesNotConsumeNewerQuestion(t *testing.T) {
	marker := reviewmd.UpdateJobReplyMarker(reviewmd.UpdateJobReply{JobID: "job", NoteID: 2, Phase: "followup"})
	for _, newer := range []bool{false, true} {
		notes := []glscm.DiscussionNote{{ID: 1, AuthorID: 7}, {ID: 2, AuthorID: 8, Body: "Original question"}}
		if newer {
			notes = append(notes, glscm.DiscussionNote{ID: 3, AuthorID: 8, Body: "New question"})
		}
		notes = append(notes, glscm.DiscussionNote{ID: 4, AuthorID: 7, Body: "Checked.\n" + marker})
		id, pending := latestPendingNote(notes, 7)
		if pending != newer || newer && id != 3 {
			t.Fatalf("newer=%v pending=%v id=%d", newer, pending, id)
		}
	}
}

func TestUpdateEyesOnChatReplyAndDurableCleanup(t *testing.T) {
	root, _ := reviewmd.NewRenderer("").ForReview("review").SummaryBodyCarried(&model.ReviewResult{ReviewID: "review"})
	s := &updateJobTestServer{notes: []glscm.DiscussionNote{{ID: 1, AuthorID: 7, Body: root}, {ID: 2, AuthorID: 8, Body: "Check verdict."}}, awards: []map[string]any{
		{"id": 81, "name": "eyes", "user": map[string]int{"id": 8}},
		{"id": 82, "name": "thumbsup", "user": map[string]int{"id": 7}},
	}}
	server := httptest.NewServer(http.HandlerFunc(s.handle))
	defer server.Close()
	client := glscm.NewClient(server.URL, "token")
	dir := filepath.Join(t.TempDir(), "journal")
	store, err := serve.NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	job := &serve.UpdateJob{ProjectPath: "g/p", IID: 1, BaseURL: server.URL, ReviewID: "review", DiscussionID: "thread", NoteID: 2, Followup: "The review has been updated."}
	job.SetID()
	if err := store.Save(job); err != nil {
		t.Fatal(err)
	}
	marker := reviewmd.UpdateJobReplyMarker(reviewmd.UpdateJobReply{JobID: job.ID, NoteID: 2, Phase: "scheduled"})
	a := &app{}
	if err := a.postChatReplyUnchecked(context.Background(), client, "g/p", 1, "thread", 2, "I scheduled an update.", marker); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := syncUpdateEyes(context.Background(), client, job, 7, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.awards) != 3 {
		t.Fatalf("missing or duplicate eyes: %+v", s.awards)
	}
	s.failEyesRemoval = true
	opts := chatOptions{repo: "g/p", mrID: 1, replyDiscussion: "thread", updateStateDir: dir, updateJobID: job.ID}
	profile := config.Profile{GitLabBaseURL: server.URL}
	if err := a.runUpdateJob(context.Background(), profile, opts); err == nil {
		t.Fatal("expected cleanup failure")
	}
	loaded, err := store.Load(job.ID)
	if err != nil || loaded.Done {
		t.Fatal("job retired before eyes cleanup")
	}
	if err := a.runUpdateJob(context.Background(), profile, opts); err != nil {
		t.Fatal(err)
	}
	if s.posts != 2 || len(s.awards) != 2 {
		t.Fatalf("duplicated follow-up or wrong cleanup: posts=%d awards=%+v", s.posts, s.awards)
	}
	loaded, err = store.Load(job.ID)
	if err != nil || !loaded.Done {
		t.Fatal("cleanup not completed durably")
	}
}

func TestUpdateJobExhaustedChecksDeliverFailure(t *testing.T) {
	root, _ := reviewmd.NewRenderer("").ForReview("review").SummaryBodyCarried(&model.ReviewResult{ReviewID: "review"})
	s := &updateJobTestServer{notes: []glscm.DiscussionNote{{ID: 1, AuthorID: 7, Body: root}, {ID: 2, AuthorID: 8, Body: "Check verdict."}}}
	server := httptest.NewServer(http.HandlerFunc(s.handle))
	defer server.Close()
	dir := filepath.Join(t.TempDir(), "journal")
	store, err := serve.NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	job := &serve.UpdateJob{ProjectPath: "g/p", IID: 1, BaseURL: server.URL, ReviewID: "review", DiscussionID: "thread", NoteID: 2, Attempts: 3, Reason: "Check verdict.", Question: "Check verdict."}
	job.SetID()
	if err := store.Save(job); err != nil {
		t.Fatal(err)
	}
	opts := chatOptions{repo: "g/p", mrID: 1, replyDiscussion: "thread", updateStateDir: dir, updateJobID: job.ID}
	if err := (&app{}).runUpdateJob(context.Background(), config.Profile{GitLabBaseURL: server.URL}, opts); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(job.ID)
	if err != nil || !loaded.Done || loaded.Followup != updateFailed {
		t.Fatalf("failure not delivered durably: %+v %v", loaded, err)
	}
	if s.posts != 1 || !strings.Contains(s.notes[len(s.notes)-1].Body, updateFailed) {
		t.Fatal("missing failure follow-up")
	}
}
