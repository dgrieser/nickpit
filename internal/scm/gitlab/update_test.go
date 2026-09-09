package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

type updateServer struct {
	mu               sync.Mutex
	server           *httptest.Server
	discussions      []MRDiscussion
	next             int
	failNote         int
	rejectPositions  bool
	positionAttempts int
	visibleWrites    int
}

func newUpdateServer(t *testing.T) (*updateServer, *Adapter, *model.ReviewResult) {
	t.Helper()
	p := 1
	r := &model.ReviewResult{ReviewID: "original-review", OverallCorrectness: "patch is incorrect", OverallExplanation: "The call can fail.", OverallConfidenceScore: 0.9,
		Findings: []model.Finding{{ID: "finding", Title: "Missing guard", Body: "The call fails for nil.", Priority: &p, ConfidenceScore: 0.9,
			CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}, Content: "old()"}}}}
	render := reviewmd.NewRenderer("").ForReview(r.ReviewID)
	root, _ := render.SummaryBodyCarried(r)
	finding, _ := render.FindingBodyCarried(r.Findings[0], "")
	s := &updateServer{next: 10, discussions: []MRDiscussion{
		{ID: "root", Notes: []DiscussionNote{{ID: 1, Body: root, AuthorID: 7, AuthorName: "nickpit"}}},
		{ID: "finding-thread", Notes: []DiscussionNote{{ID: 2, Body: finding, AuthorID: 7, AuthorName: "nickpit"}}},
	}}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s, NewAdapter(NewClient(s.server.URL, "token"), ""), r
}

func (s *updateServer) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	if r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/user" {
		write(map[string]any{"id": 7, "username": "nickpit"})
		return
	}
	tail, ok := strings.CutPrefix(r.URL.EscapedPath(), mrBase)
	if !ok {
		http.Error(w, "unexpected path", 404)
		return
	}
	noteJSON := func(n DiscussionNote) map[string]any {
		return map[string]any{"id": n.ID, "body": n.Body, "system": n.System, "author": map[string]any{"id": n.AuthorID, "username": n.AuthorName}}
	}
	discussionJSON := func(d MRDiscussion) map[string]any {
		notes := []any{}
		for _, n := range d.Notes {
			notes = append(notes, noteJSON(n))
		}
		return map[string]any{"id": d.ID, "notes": notes}
	}
	if r.Method == http.MethodGet {
		switch tail {
		case "":
			_, _ = w.Write([]byte(mrMetadataJSON))
			return
		case "/changes":
			_, _ = w.Write([]byte(`{"changes":[{"new_path":"main.go","old_path":"main.go","diff":"@@ -1,2 +1,3 @@\n old()\n+new()\n tail()"}]}`))
			return
		case "/discussions":
			items := []any{}
			for _, d := range s.discussions {
				items = append(items, discussionJSON(d))
			}
			write(items)
			return
		case "/notes":
			items := []any{}
			for _, d := range s.discussions {
				for _, n := range d.Notes {
					items = append(items, noteJSON(n))
				}
			}
			write(items)
			return
		}
		for _, d := range s.discussions {
			if tail == "/discussions/"+d.ID {
				write(discussionJSON(d))
				return
			}
		}
	}
	if r.Method == http.MethodDelete && strings.HasPrefix(tail, "/notes/") {
		id, _ := strconv.Atoi(strings.TrimPrefix(tail, "/notes/"))
		for i, d := range s.discussions {
			if len(d.Notes) == 1 && d.Notes[0].ID == id {
				s.discussions = append(s.discussions[:i], s.discussions[i+1:]...)
				w.WriteHeader(204)
				return
			}
		}
		w.WriteHeader(404)
		return
	}
	var payload struct {
		Body     string    `json:"body"`
		Position *position `json:"position"`
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	if r.Method == http.MethodPut {
		for i, d := range s.discussions {
			for j, n := range d.Notes {
				if tail != fmt.Sprintf("/discussions/%s/notes/%d", d.ID, n.ID) {
					continue
				}
				if n.ID == s.failNote {
					s.failNote = 0
					http.Error(w, "injected failure", 500)
					return
				}
				s.discussions[i].Notes[j].Body = payload.Body
				s.visibleWrites++
				write(noteJSON(s.discussions[i].Notes[j]))
				return
			}
		}
	}
	if r.Method == http.MethodPost && (tail == "/notes" || tail == "/discussions") {
		if payload.Position != nil {
			s.positionAttempts++
			if s.rejectPositions {
				http.Error(w, "position rejected", http.StatusUnprocessableEntity)
				return
			}
		}
		s.next++
		n := DiscussionNote{ID: s.next, Body: payload.Body, AuthorID: 7, AuthorName: "nickpit"}
		d := MRDiscussion{ID: fmt.Sprintf("new-%d", s.next), Notes: []DiscussionNote{n}}
		s.discussions = append(s.discussions, d)
		if reviewmd.StripMarkers(payload.Body) != "" {
			s.visibleWrites++
		}
		if tail == "/notes" {
			write(noteJSON(n))
		} else {
			write(discussionJSON(d))
		}
		return
	}
	http.Error(w, "unexpected request", 404)
}

func (s *updateServer) snapshot() []MRDiscussion {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MRDiscussion, len(s.discussions))
	for i, d := range s.discussions {
		out[i] = MRDiscussion{ID: d.ID, Notes: append([]DiscussionNote(nil), d.Notes...)}
	}
	return out
}

func updateRequest(before, after *model.ReviewResult) ReviewUpdateRequest {
	return ReviewUpdateRequest{Before: before, After: after, HeadSHA: "headsha", BaseSHA: "basesha"}
}

func TestUpdateReviewResolutionAndIdempotency(t *testing.T) {
	s, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.Findings[0].Resolution = &model.FindingResolution{Reason: "The new guard prevents the failure."}
	after.OverallCorrectness = "patch is correct"
	after.OverallExplanation = "The guard makes the call safe."
	got, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(before, after))
	if err != nil {
		t.Fatal(err)
	}
	discussions := s.snapshot()
	root := findUpdateTarget(discussions, 7, before.ReviewID, "")
	finding := findUpdateTarget(discussions, 7, before.ReviewID, "finding")
	if len(reviewmd.ReadHistory(root.Body).Entries) != 1 || len(reviewmd.ReadHistory(finding.Body).Entries) != 1 || !strings.Contains(reviewmd.StripMarkers(finding.Body), "resolved.svg") {
		t.Fatal("missing updated comments or history")
	}
	result := reviewmd.ReviewResultsByID(ownedBodies(discussions, 7))[before.ReviewID]
	if result == nil || result.Revision != 1 || result.Findings[0].Resolution == nil || result.OverallCorrectness != "patch is correct" {
		t.Fatalf("bad reassembly: %+v", result)
	}
	for _, body := range ownedBodies(discussions, 7) {
		if len(reviewmd.CollectUpdateRecords(body)) > 0 {
			t.Fatal("finished recovery data retained")
		}
	}
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(got, got)); err != nil {
		t.Fatal(err)
	}
	root = findUpdateTarget(s.snapshot(), 7, before.ReviewID, "")
	if len(reviewmd.ReadHistory(root.Body).Entries) != 1 {
		t.Fatal("no-op duplicated history")
	}
}

func TestUpdateReviewRecoversPartialWrite(t *testing.T) {
	s, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.Findings[0].Title = "Corrected title"
	after.OverallExplanation = "Corrected verdict."
	s.failNote = 1
	req := updateRequest(before, after)
	staged := 0
	req.OnStaged = func() error {
		staged++
		if s.visibleWrites != 0 {
			t.Fatal("staged callback ran after visible writes")
		}
		return nil
	}
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, req); err == nil {
		t.Fatal("expected root write failure")
	}
	if staged != 1 {
		t.Fatalf("staged callback count = %d", staged)
	}
	if got := reviewmd.ReviewResultsByID(ownedBodies(s.snapshot(), 7))[before.ReviewID]; got != nil {
		t.Fatal("partial review exposed")
	}
	if err := a.RecoverReviewUpdates(context.Background(), "group/project", 456); err != nil {
		t.Fatal(err)
	}
	got := reviewmd.ReviewResultsByID(ownedBodies(s.snapshot(), 7))[before.ReviewID]
	if got == nil || got.Findings[0].Title != "Corrected title" || got.OverallExplanation != "Corrected verdict." {
		t.Fatalf("recovery failed: %+v", got)
	}
	for _, fid := range []string{"", "finding"} {
		target := findUpdateTarget(s.snapshot(), 7, before.ReviewID, fid)
		if len(reviewmd.ReadHistory(target.Body).Entries) != 1 {
			t.Fatal("retry duplicated history")
		}
	}
}

func TestUpdateJobCommitSurvivesRecoveryAndLaterRootRevision(t *testing.T) {
	s, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.Findings[0].Title = "Corrected title"
	req := updateRequest(before, after)
	req.Operation = "durable-job"
	s.failNote = 1
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, req); err == nil {
		t.Fatal("expected interrupted publication")
	}
	if committed, err := a.ReviewUpdateCommitted(context.Background(), "group/project", 456, before.ReviewID, req.Operation); err != nil || committed {
		t.Fatalf("partial operation reported committed: %v %v", committed, err)
	}
	if err := a.RecoverReviewUpdates(context.Background(), "group/project", 456); err != nil {
		t.Fatal(err)
	}
	current := reviewmd.ReviewResultsByID(ownedBodies(s.snapshot(), 7))[before.ReviewID]
	next, _ := current.Clone()
	next.Findings[0].Title = "Second correction"
	req2 := updateRequest(current, next)
	req2.Operation = "next-job"
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, req2); err != nil {
		t.Fatal(err)
	}
	committed, err := a.ReviewUpdateCommitted(context.Background(), "group/project", 456, before.ReviewID, req.Operation)
	if err != nil || !committed {
		t.Fatalf("lost durable commit receipt: %v %v", committed, err)
	}
	if committed, err := a.ReviewUpdateCommitted(context.Background(), "group/project", 456, "another-review", req.Operation); err != nil || committed {
		t.Fatalf("commit leaked across reviews: %v %v", committed, err)
	}
}

func TestUpdateReviewReplacesLocationAndRecoversRedirect(t *testing.T) {
	s, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.Findings[0].CodeLocation.LineRange = model.LineRange{Start: 2, End: 2}
	after.Findings[0].CodeLocation.Content = "new()"
	s.failNote = 2
	s.rejectPositions = true
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(before, after)); err == nil {
		t.Fatal("expected redirect failure")
	}
	if err := a.RecoverReviewUpdates(context.Background(), "group/project", 456); err != nil {
		t.Fatal(err)
	}
	discussions := s.snapshot()
	current := findUpdateTarget(discussions, 7, before.ReviewID, "finding")
	if current.DiscussionID == "finding-thread" || current.NoteID == 0 {
		t.Fatal("replacement not created")
	}
	if s.positionAttempts != 1 {
		t.Fatalf("unexpected position attempts: %d", s.positionAttempts)
	}
	visible := 0
	for _, d := range discussions {
		if len(d.Notes) > 0 && reviewmd.StripMarkers(d.Notes[0].Body) != "" {
			visible++
		}
		if d.ID == "finding-thread" {
			ref := reviewmd.ReadThreadReference(d.Notes[0].Body)
			if ref.Next != current.DiscussionID || !strings.Contains(d.Notes[0].Body, "#note_") {
				t.Fatal("redirect missing")
			}
		}
	}
	if visible != 3 {
		t.Fatalf("retry duplicated replacement: visible=%d", visible)
	}
	if len(reviewmd.ReadHistory(current.Body).Entries) != 1 {
		t.Fatal("replacement lost history")
	}
}

func TestUpdateReviewRootOnlyAndStaleState(t *testing.T) {
	s, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.OverallExplanation = "Fresh explanation."
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(before, after)); err != nil {
		t.Fatal(err)
	}
	if len(reviewmd.ReadHistory(findUpdateTarget(s.snapshot(), 7, before.ReviewID, "finding").Body).Entries) != 0 {
		t.Fatal("root-only update touched finding")
	}
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(before, after)); err == nil || !strings.Contains(err.Error(), "changed during") {
		t.Fatalf("stale update accepted: %v", err)
	}
}

func TestUpdateReviewConcurrentCorrections(t *testing.T) {
	_, a, before := newUpdateServer(t)
	errs := make(chan error, 2)
	for _, title := range []string{"one", "two"} {
		after, _ := before.Clone()
		after.Findings[0].Title = title
		go func() {
			_, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(before, after))
			errs <- err
		}()
	}
	success := 0
	for range 2 {
		if <-errs == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("expected one committed update, got %d", success)
	}
}

func TestMRLockCancellationAndReentrancy(t *testing.T) {
	c := NewClient("https://lock-test.example", "token")
	ctx, release, err := c.LockMR(context.Background(), t.Name(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, nested, err := c.LockMR(ctx, t.Name(), 1)
	if err != nil {
		t.Fatal(err)
	}
	nested()
	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, _, err := c.LockMR(timeout, t.Name(), 1); err == nil {
		t.Fatal("competing lock did not block")
	}
}
