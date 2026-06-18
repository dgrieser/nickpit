package gitlab

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

const (
	mrBase          = "/api/v4/projects/group%2Fproject/merge_requests/456"
	mrMetadataJSON  = `{"diff_refs":{"base_sha":"basesha","head_sha":"headsha","start_sha":"startsha"}}`
	mrChangesJSON   = `{"changes":[{"new_path":"main.go","old_path":"main.go","diff":"@@ -1 +1 @@\n-old\n+new"}]}`
	emptyArrayJSON  = `[]`
	createdNoteJSON = `{"id":1}`
)

type postRecord struct {
	body     string
	position map[string]any
}

type publishServer struct {
	t           *testing.T
	server      *httptest.Server
	mu          sync.Mutex
	noteBody    []byte // GET /notes payload (dedupe)
	changesJSON []byte // GET /changes payload (default mrChangesJSON)
	notePosts   []postRecord
	discPosts   []postRecord
	discStatus  int // status for POST /discussions (0 -> 201)
}

func newPublishServer(t *testing.T) *publishServer {
	ps := &publishServer{t: t, noteBody: []byte(emptyArrayJSON), changesJSON: []byte(mrChangesJSON)}
	ps.server = httptest.NewServer(http.HandlerFunc(ps.handle))
	t.Cleanup(ps.server.Close)
	return ps
}

func (ps *publishServer) handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.EscapedPath()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && path == mrBase:
		_, _ = w.Write([]byte(mrMetadataJSON))
	case r.Method == http.MethodGet && path == mrBase+"/changes":
		ps.mu.Lock()
		changes := ps.changesJSON
		ps.mu.Unlock()
		_, _ = w.Write(changes)
	case r.Method == http.MethodGet && path == mrBase+"/notes":
		ps.mu.Lock()
		body := ps.noteBody
		ps.mu.Unlock()
		_, _ = w.Write(body)
	case r.Method == http.MethodGet && path == mrBase+"/discussions":
		_, _ = w.Write([]byte(emptyArrayJSON))
	case r.Method == http.MethodPost && path == mrBase+"/notes":
		ps.record(r, &ps.notePosts)
		_, _ = w.Write([]byte(createdNoteJSON))
	case r.Method == http.MethodPost && path == mrBase+"/discussions":
		ps.mu.Lock()
		status := ps.discStatus
		ps.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"line not part of the diff"}`))
			return
		}
		ps.record(r, &ps.discPosts)
		_, _ = w.Write([]byte(createdNoteJSON))
	default:
		ps.t.Errorf("unexpected request: %s %s", r.Method, path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (ps *publishServer) record(r *http.Request, dst *[]postRecord) {
	raw, _ := io.ReadAll(r.Body)
	var parsed struct {
		Body     string         `json:"body"`
		Position map[string]any `json:"position"`
	}
	_ = json.Unmarshal(raw, &parsed)
	ps.mu.Lock()
	*dst = append(*dst, postRecord{body: parsed.Body, position: parsed.Position})
	ps.mu.Unlock()
}

func (ps *publishServer) adapter() *Adapter {
	return NewAdapter(NewClient(ps.server.URL, "token"), "")
}

func intPtr(v int) *int { return &v }

// fpMarker builds the hidden carrier marker a posted finding comment carries, so
// tests can assert on (or seed) it the same way the renderer emits it.
func fpMarker(id, file, title string) string {
	return reviewmd.FingerprintMarker(model.Finding{ID: id, CodeLocation: model.CodeLocation{FilePath: file}}, title)
}

func sampleResult() *model.ReviewResult {
	return &model.ReviewResult{
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "Two issues found.",
		OverallConfidenceScore: 0.9,
		Findings: []model.Finding{
			{
				ID:           "finding-a",
				Title:        "Inline issue",
				Body:         "On a changed line.",
				Priority:     intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
			},
			{
				ID:           "finding-b",
				Title:        "Out-of-diff issue",
				Body:         "On a file not in the diff.",
				Priority:     intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 5, End: 5}},
			},
		},
	}
}

func req() model.ReviewRequest {
	return model.ReviewRequest{Mode: model.ModeGitLab, Repo: "group/project", Identifier: 456}
}

func TestPublishReviewHappyPath(t *testing.T) {
	ps := newPublishServer(t)
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Summary + out-of-diff finding => 2 notes; inline finding => 1 discussion.
	if len(ps.notePosts) != 2 {
		t.Fatalf("note posts = %d, want 2", len(ps.notePosts))
	}
	if len(ps.discPosts) != 1 {
		t.Fatalf("discussion posts = %d, want 1", len(ps.discPosts))
	}
	if !strings.Contains(ps.notePosts[0].body, reviewmd.SummaryMarker) {
		t.Fatalf("first note is not the summary: %q", ps.notePosts[0].body)
	}
	disc := ps.discPosts[0]
	if disc.position["base_sha"] != "basesha" || disc.position["head_sha"] != "headsha" || disc.position["start_sha"] != "startsha" {
		t.Fatalf("position SHAs wrong: %#v", disc.position)
	}
	if disc.position["new_line"].(float64) != 1 {
		t.Fatalf("new_line = %v, want 1", disc.position["new_line"])
	}
	if !strings.Contains(disc.body, fpMarker("finding-a", "main.go", "Inline issue")) {
		t.Fatalf("discussion missing finding marker: %q", disc.body)
	}
	// out-of-diff note carries file:line prefix
	fallback := ps.notePosts[1]
	if !strings.Contains(fallback.body, "`other.go:5`") || !strings.Contains(fallback.body, fpMarker("finding-b", "other.go", "Out-of-diff issue")) {
		t.Fatalf("fallback note missing prefix/marker: %q", fallback.body)
	}
}

func TestPublishReview422FallsBackToNote(t *testing.T) {
	ps := newPublishServer(t)
	ps.discStatus = http.StatusUnprocessableEntity
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish should tolerate 422 fallback: %v", err)
	}
	if len(ps.discPosts) != 0 {
		t.Fatalf("422 discussion should not be recorded, got %d", len(ps.discPosts))
	}
	// summary + finding-a (fell back) + finding-b => 3 notes
	if len(ps.notePosts) != 3 {
		t.Fatalf("note posts = %d, want 3", len(ps.notePosts))
	}
	var sawA bool
	for _, n := range ps.notePosts {
		if strings.Contains(n.body, fpMarker("finding-a", "main.go", "Inline issue")) && strings.Contains(n.body, "`main.go:1`") {
			sawA = true
		}
	}
	if !sawA {
		t.Fatal("finding-a did not fall back to a file:line note")
	}
}

func TestPublishReviewDedupeSkipsExisting(t *testing.T) {
	ps := newPublishServer(t)
	ps.noteBody = []byte(`[{"body":"` + reviewmd.SummaryMarker + `"},{"body":"prefix ` + fpMarker("finding-a", "main.go", "Inline issue") + ` suffix"}]`)
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// summary + finding-a already present => only finding-b (out-of-diff note) posts.
	if len(ps.discPosts) != 0 {
		t.Fatalf("finding-a was deduped; no discussion expected, got %d", len(ps.discPosts))
	}
	if len(ps.notePosts) != 1 {
		t.Fatalf("note posts = %d, want 1 (only finding-b)", len(ps.notePosts))
	}
	if !strings.Contains(ps.notePosts[0].body, fpMarker("finding-b", "other.go", "Out-of-diff issue")) {
		t.Fatalf("expected only finding-b note, got %q", ps.notePosts[0].body)
	}
}

// TestPublishReviewDedupeCrossRun proves the file+title fingerprint skips a
// finding posted on a PRIOR run, whose random id differs from this run's.
func TestPublishReviewDedupeCrossRun(t *testing.T) {
	ps := newPublishServer(t)
	ps.noteBody = []byte(`[{"body":"` + reviewmd.SummaryMarker + `"},{"body":"x ` + fpMarker("prior-run-id", "main.go", "Inline issue") + ` y"}]`)
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(ps.discPosts) != 0 {
		t.Fatalf("finding-a should match across runs by file+title; got %d discussions", len(ps.discPosts))
	}
	if len(ps.notePosts) != 1 || !strings.Contains(ps.notePosts[0].body, fpMarker("finding-b", "other.go", "Out-of-diff issue")) {
		t.Fatalf("expected only finding-b note, got %v", ps.notePosts)
	}
}

// A non-422 error (e.g. 500) on the inline POST is a real failure: it must be
// returned for that finding, not silently swallowed by the file:line fallback.
func TestPublishFindingNon422Propagates(t *testing.T) {
	ps := newPublishServer(t)
	ps.discStatus = http.StatusInternalServerError
	err := ps.adapter().PublishReview(context.Background(), req(), sampleResult())
	if err == nil {
		t.Fatal("expected the 500 on finding-a to propagate")
	}
	if !strings.Contains(err.Error(), "finding-a") {
		t.Fatalf("error should name finding-a, got %v", err)
	}
	if len(ps.discPosts) != 0 {
		t.Fatalf("500 discussion should not be recorded, got %d", len(ps.discPosts))
	}
	// finding-a errored without falling back; only summary + finding-b post.
	if len(ps.notePosts) != 2 {
		t.Fatalf("note posts = %d, want 2 (summary + finding-b)", len(ps.notePosts))
	}
	for _, n := range ps.notePosts {
		if strings.Contains(n.body, fpMarker("finding-a", "main.go", "Inline issue")) {
			t.Fatalf("finding-a must not fall back on a non-422 error: %q", n.body)
		}
	}
}

func TestPublishReviewMultiLineInline(t *testing.T) {
	ps := newPublishServer(t)
	// Hunk new-side: line1=" a" context, line2="+b", line3="+c".
	ps.changesJSON = []byte(`{"changes":[{"new_path":"multi.go","old_path":"multi.go","diff":"@@ -1,1 +1,3 @@\n a\n+b\n+c"}]}`)
	result := &model.ReviewResult{
		OverallExplanation: "spanning finding",
		Findings: []model.Finding{{
			ID:           "finding-span",
			Title:        "Multi-line issue",
			Body:         "Spans two added lines.",
			Priority:     intPtr(1),
			CodeLocation: model.CodeLocation{FilePath: "multi.go", LineRange: model.LineRange{Start: 2, End: 3}},
		}},
	}
	if err := ps.adapter().PublishReview(context.Background(), req(), result); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(ps.notePosts) != 1 {
		t.Fatalf("note posts = %d, want 1 (summary only)", len(ps.notePosts))
	}
	if len(ps.discPosts) != 1 {
		t.Fatalf("discussion posts = %d, want 1 (inline multi-line)", len(ps.discPosts))
	}
	pos := ps.discPosts[0].position
	if pos["new_line"].(float64) != 3 {
		t.Fatalf("anchor new_line = %v, want 3 (end of range)", pos["new_line"])
	}
	lr, ok := pos["line_range"].(map[string]any)
	if !ok {
		t.Fatalf("multi-line position missing line_range: %#v", pos)
	}
	start, _ := lr["start"].(map[string]any)
	end, _ := lr["end"].(map[string]any)
	if start["line_code"] == "" || end["line_code"] == "" {
		t.Fatalf("line_range endpoints missing line_code: %#v", lr)
	}
	if start["new_line"].(float64) != 2 || end["new_line"].(float64) != 3 {
		t.Fatalf("line_range new_line span = %v..%v, want 2..3", start["new_line"], end["new_line"])
	}
}

// Pure-markdown rendering (sanitize, marker collection, finding-field
// precedence) is exercised in the shared internal/scm/reviewmd package.
