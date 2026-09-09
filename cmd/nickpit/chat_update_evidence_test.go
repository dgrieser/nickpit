package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/serve"
)

type evidenceCaptureLLM struct{ requests []*llm.ReviewRequest }

func (c *evidenceCaptureLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	c.requests = append(c.requests, req)
	if req.SchemaKind == llm.SchemaKindJSON {
		return &llm.ReviewResponse{RawResponse: `{"updates":[],"review":{"action":"correction_warranted","reason":"Selected evidence supports correction."}}`}, nil
	}
	return nil, context.Canceled
}

func TestUpdateAndVerdictUseSelectedSnapshotOnly(t *testing.T) {
	result := &model.ReviewResult{ReviewID: "review", OverallCorrectness: "patch is correct", OverallExplanation: "Previous verdict.", OverallConfidenceScore: 0.9}
	root, _ := reviewmd.NewRenderer("").ForReview("review").SummaryBodyCarried(result)
	s := &updateJobTestServer{notes: []glscm.DiscussionNote{
		{ID: 1, AuthorID: 7, Body: root}, {ID: 2, AuthorID: 8, Body: "Selected earlier proof"},
		{ID: 10, AuthorID: 8, Body: "Queued question"}, {ID: 11, AuthorID: 8, Body: "Later unrelated question"},
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/merge_requests/1") {
			_ = json.NewEncoder(w).Encode(map[string]any{"diff_refs": map[string]string{"base_sha": "base", "head_sha": "head"}})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/changes") {
			_, _ = w.Write([]byte(`{"changes":[]}`))
			return
		}
		s.handle(w, r)
	}))
	defer server.Close()
	client := glscm.NewClient(server.URL, "token")
	job := &serve.UpdateJob{BaseURL: server.URL, ProjectPath: "g/p", IID: 1, ReviewID: "review", DiscussionID: "thread", NoteID: 10, Question: "Queued question", Reason: "Check review"}
	job.SetID()
	snapshot, err := loadUpdateEvidence(context.Background(), client, job, 7, chatMessageControls{})
	if err != nil {
		t.Fatal(err)
	}
	job.Evidence = snapshot.Fingerprint
	store, err := serve.NewUpdateStore(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	capture := &evidenceCaptureLLM{}
	profile := config.Profile{Model: "test", MaxToolCalls: -1}
	u := &gitLabUpdateExecution{
		gitLabChatUpdate: &gitLabChatUpdate{app: &app{}, profile: profile, adapter: glscm.NewAdapter(client, ""), engine: review.NewEngine(fakeContextOnlySource{}, capture, nil, profile), project: "g/p", iid: 1, botUserID: 7,
			request: review.DiscussRequest{Result: result, ReviewCtx: &model.ReviewContext{DiffBaseSHA: "base", DiffHeadSHA: "head", Comments: []model.Comment{{Body: "UNRELATED MR CONTEXT"}}}, MaxToolCalls: -1},
		}, job: job, store: store, snapshot: snapshot,
	}
	_, err = u.run(context.Background(), review.ReviewUpdateSignal{Reason: "Check review"})
	if err == nil {
		t.Fatal("expected captured verdict cancellation")
	}
	seen := map[llm.SchemaKind]bool{}
	for _, req := range capture.requests {
		raw, _ := json.Marshal(req.Messages)
		for _, forbidden := range []string{"UNRELATED MR CONTEXT", "Later unrelated question"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("leaked evidence: %s", forbidden)
			}
		}
		if !strings.Contains(string(raw), "Queued question") || !strings.Contains(string(raw), "Selected earlier proof") {
			t.Fatalf("missing snapshot in %s", req.SchemaKind)
		}
		seen[req.SchemaKind] = true
	}
	if !seen[llm.SchemaKindJSON] || !seen[llm.SchemaKindVerdict] {
		t.Fatalf("missing stages: %+v", seen)
	}
	if s.posts != 0 {
		t.Fatal("capture test published")
	}
}

func TestOverallUpdateSnapshotCutoffAndFallbacks(t *testing.T) {
	trigger := []glscm.DiscussionNote{
		{ID: 1, AuthorID: 7}, {ID: 2, AuthorID: 8, Body: "Earlier question"},
		{AuthorID: 7, Body: "First answer", FallbackNoteID: 3, AnsweredNoteID: 2},
		{AuthorID: 7, Body: "Second answer", FallbackNoteID: 4, AnsweredNoteID: 2},
		{ID: 10, AuthorID: 8, Body: "Queued question"},
		{AuthorID: 7, Body: "Late answer to earlier question", FallbackNoteID: 11, AnsweredNoteID: 2},
		{AuthorID: 7, Body: "Scheduled", FallbackNoteID: 12, AnsweredNoteID: 10},
		{ID: 13, AuthorID: 8, Body: "Later question"},
	}
	got := collectUpdateEvidence(nil, trigger, "review", nil, 10, 7, chatMessageControls{})
	want := []llm.Message{{Role: "user", Content: "Earlier question"}, {Role: "assistant", Content: "First answer"}, {Role: "assistant", Content: "Second answer"}, {Role: "user", Content: "Queued question"}}
	if !reflect.DeepEqual(got.Messages, want) {
		t.Fatalf("messages = %+v", got.Messages)
	}
	trigger[6].Body = "Unrelated later edit"
	if next := collectUpdateEvidence(nil, trigger, "review", nil, 10, 7, chatMessageControls{}); next.Fingerprint != got.Fingerprint {
		t.Fatal("later message changed evidence")
	}
	trigger[1].Body = "Edited earlier evidence"
	if next := collectUpdateEvidence(nil, trigger, "review", nil, 10, 7, chatMessageControls{}); next.Fingerprint == got.Fingerprint {
		t.Fatal("selected edit did not change evidence")
	}
}

func TestFallbackRepliesKeepDistinctSourceIdentities(t *testing.T) {
	notes := []glscm.DiscussionNote{{ID: 1, AuthorID: 7}, {ID: 2, AuthorID: 8, Body: "Question"}, {ID: 10, AuthorID: 8, Body: "Queued question"}}
	body := "Same answer\n\n" + reviewmd.ChatReplyMarker("thread", 2)
	fallbacks := []glscm.MRNote{{ID: 3, AuthorID: 7, Body: body}, {ID: 4, AuthorID: 7, Body: body}, {ID: 3, AuthorID: 7, Body: body}}
	merged := mergeFallbackReplies(notes, fallbacks, "thread", 7)
	if len(merged) != 5 || merged[2].FallbackNoteID != 3 || merged[3].FallbackNoteID != 4 || merged[4].ID != 10 {
		t.Fatalf("lost fallback identities or ordering: %+v", merged)
	}
	snapshot := collectUpdateEvidence(nil, merged, "review", nil, 10, 7, chatMessageControls{})
	if len(snapshot.Messages) != 4 || snapshot.Messages[1].Content != "Same answer" || snapshot.Messages[2].Content != "Same answer" || snapshot.Messages[3].Content != "Queued question" {
		t.Fatalf("corrupt merged transcript: %+v", snapshot.Messages)
	}
}

func TestSelectedUpdateEvidenceIgnoresOtherThreadsAndDeduplicates(t *testing.T) {
	root := reviewmd.FindingReferenceMarker("review", "f")
	trigger := []glscm.DiscussionNote{{ID: 1, AuthorID: 7, Body: root}, {ID: 2, AuthorID: 8, Body: "Evidence"}, {ID: 10, AuthorID: 8, Body: "Question"}}
	discussions := []glscm.MRDiscussion{
		{ID: "selected", Notes: trigger},
		{ID: "unrelated", Notes: []glscm.DiscussionNote{{ID: 3, AuthorID: 8, Body: "Human root"}, {ID: 4, AuthorID: 8, Body: "Other discussion"}}},
	}
	before := collectUpdateEvidence(discussions, trigger, "review", []string{"f"}, 10, 7, chatMessageControls{})
	if len(before.Messages) != 2 {
		t.Fatalf("duplicated messages: %+v", before.Messages)
	}
	discussions[1].Notes[1].Body = "Unrelated edit"
	after := collectUpdateEvidence(discussions, trigger, "review", []string{"f"}, 10, 7, chatMessageControls{})
	if before.Fingerprint != after.Fingerprint {
		t.Fatal("unrelated edit invalidated evidence")
	}
	discussions[0].Notes = discussions[0].Notes[:1]
	after = collectUpdateEvidence(discussions, trigger[:1], "review", []string{"f"}, 10, 7, chatMessageControls{})
	if before.Fingerprint == after.Fingerprint {
		t.Fatal("deleted evidence not detected")
	}
}

func TestUpdateRetryBudgetsSurviveRestart(t *testing.T) {
	store, err := serve.NewUpdateStore(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	job := &serve.UpdateJob{BaseURL: "https://example.test", ProjectPath: "g/p", IID: 1, ReviewID: "r", DiscussionID: "d", NoteID: 1}
	job.SetID()
	now := time.Now().UTC()
	for i := 1; i <= 5; i++ {
		job.Attempts++
		recordUpdateRetry(job, fmt.Errorf("wrapped: %w", &glscm.UpdateConflict{Kind: "evidence"}), now)
		if job.Attempts != 0 || job.ConflictRetries != i || !job.NextAttempt.Equal(now.Add(time.Duration(i)*10*time.Second)) {
			t.Fatalf("wrong counters: %+v", job)
		}
		if err := store.Save(job); err != nil {
			t.Fatal(err)
		}
		job, err = store.Load(job.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	failUpdateJob(job)
	if job.Followup != updateConflicted {
		t.Fatal("missing churn outcome")
	}
	job.ConflictRetries = 0
	for i := 1; i <= 3; i++ {
		job.Attempts++
		recordUpdateRetry(job, errors.New("publication failed"), now)
	}
	if job.Attempts != 3 || job.ConflictRetries != 0 {
		t.Fatalf("mixed counters: %+v", job)
	}
	failUpdateJob(job)
	if job.Followup != updateFailed {
		t.Fatal("missing failure outcome")
	}
}

func TestUpdateJobDefersBehindOlderAndContendedJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("deferred job made API call: %s", r.URL.Path)
		w.WriteHeader(500)
	}))
	defer server.Close()
	dir := filepath.Join(t.TempDir(), "state")
	store, err := serve.NewUpdateStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	first := &serve.UpdateJob{BaseURL: server.URL, ProjectPath: "g/p", IID: 1, ReviewID: "r", DiscussionID: "d1", NoteID: 1, Created: time.Now(), NextAttempt: time.Now().Add(time.Hour)}
	first.SetID()
	second := *first
	second.DiscussionID = "d2"
	second.NoteID = 2
	second.Created = first.Created.Add(time.Second)
	second.NextAttempt = time.Time{}
	second.SetID()
	for _, j := range []*serve.UpdateJob{first, &second} {
		if err := store.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	profile := config.Profile{GitLabBaseURL: server.URL}
	opts := chatOptions{repo: "g/p", mrID: 1, replyDiscussion: "d2", updateJobID: second.ID, updateStateDir: dir}
	if err := (&app{}).runUpdateJob(context.Background(), profile, opts); !errors.Is(err, errUpdateDeferred) {
		t.Fatalf("overtook older job: %v", err)
	}
	client := glscm.NewClient(server.URL, "")
	_, release, err := client.LockMR(context.Background(), "update-execution/g/p", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := (&app{}).runUpdateJob(context.Background(), profile, opts); !errors.Is(err, errUpdateDeferred) {
		t.Fatalf("contention: %v", err)
	}
	loaded, err := store.Load(second.ID)
	if err != nil || loaded.Attempts != 0 || loaded.ConflictRetries != 0 {
		t.Fatalf("deferred counters: %+v %v", loaded, err)
	}
}
