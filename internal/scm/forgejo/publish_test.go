package forgejo

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
	prBase         = "/api/v1/repos/owner/repo/pulls/123"
	issueBase      = "/api/v1/repos/owner/repo/issues/123"
	prMetadataJSON = `{"head":{"sha":"headsha"}}`
	prDiff         = "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	emptyArrayJSON = `[]`
	createdJSON    = `{"id":1}`
)

type reviewPost struct {
	commitID string
	body     string
	event    string
	comments []reviewComment
}

type publishServer struct {
	t            *testing.T
	server       *httptest.Server
	mu           sync.Mutex
	reviewsBody  []byte // GET /pulls/:n/reviews (dedupe)
	commentsB    []byte // GET /pulls/:n/reviews/1/comments (dedupe)
	issuesBody   []byte // GET /issues/:n/comments (dedupe)
	reviewPosts  []reviewPost
	issuePosts   []string
	deleted      []string
	rejectInline bool // POST /reviews with comments is rejected
	reviewStatus int  // unconditional status for POST /reviews (0 -> 200)
}

func newPublishServer(t *testing.T) *publishServer {
	ps := &publishServer{
		t:           t,
		reviewsBody: []byte(emptyArrayJSON),
		commentsB:   []byte(emptyArrayJSON),
		issuesBody:  []byte(emptyArrayJSON),
	}
	ps.server = httptest.NewServer(http.HandlerFunc(ps.handle))
	t.Cleanup(ps.server.Close)
	return ps
}

func (ps *publishServer) handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && path == prBase:
		_, _ = w.Write([]byte(prMetadataJSON))
	case r.Method == http.MethodGet && path == prBase+".diff":
		_, _ = w.Write([]byte(prDiff))
	case r.Method == http.MethodGet && path == prBase+"/reviews":
		ps.mu.Lock()
		body := ps.reviewsBody
		ps.mu.Unlock()
		_, _ = w.Write(body)
	case r.Method == http.MethodGet && path == prBase+"/reviews/1/comments":
		ps.mu.Lock()
		body := ps.commentsB
		ps.mu.Unlock()
		_, _ = w.Write(body)
	case r.Method == http.MethodGet && path == issueBase+"/comments":
		ps.mu.Lock()
		body := ps.issuesBody
		ps.mu.Unlock()
		_, _ = w.Write(body)
	case r.Method == http.MethodPost && path == prBase+"/reviews":
		ps.handleReviewPost(w, r)
	case r.Method == http.MethodPost && path == issueBase+"/comments":
		var parsed struct {
			Body string `json:"body"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &parsed)
		ps.mu.Lock()
		ps.issuePosts = append(ps.issuePosts, parsed.Body)
		ps.mu.Unlock()
		_, _ = w.Write([]byte(createdJSON))
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/repos/owner/repo/issues/comments/"):
		ps.mu.Lock()
		ps.deleted = append(ps.deleted, strings.TrimPrefix(path, "/api/v1/repos/owner/repo/issues/comments/"))
		ps.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		ps.t.Errorf("unexpected request: %s %s", r.Method, path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (ps *publishServer) handleReviewPost(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var parsed struct {
		CommitID string          `json:"commit_id"`
		Body     string          `json:"body"`
		Event    string          `json:"event"`
		Comments []reviewComment `json:"comments"`
	}
	_ = json.Unmarshal(raw, &parsed)
	ps.mu.Lock()
	status := ps.reviewStatus
	reject := ps.rejectInline && len(parsed.Comments) > 0
	ps.reviewPosts = append(ps.reviewPosts, reviewPost{
		commitID: parsed.CommitID,
		body:     parsed.Body,
		event:    parsed.Event,
		comments: parsed.Comments,
	})
	ps.mu.Unlock()
	if status != 0 {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
		return
	}
	if reject {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"line must be part of the diff"}`))
		return
	}
	_, _ = w.Write([]byte(createdJSON))
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
		// A review id activates the carrier machinery (hidden review/finding
		// envelopes) in every publish test, as real pipeline results do.
		ReviewID:               "rev-pub",
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
	return model.ReviewRequest{Mode: model.ModeForgejo, Repo: "owner/repo", Identifier: 123}
}

func TestPublishReviewHappyPath(t *testing.T) {
	ps := newPublishServer(t)
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// One review (summary body + inline finding-a); finding-b => one issue comment.
	if len(ps.reviewPosts) != 1 {
		t.Fatalf("review posts = %d, want 1", len(ps.reviewPosts))
	}
	review := ps.reviewPosts[0]
	if review.event != "COMMENT" {
		t.Fatalf("event = %q, want COMMENT", review.event)
	}
	if review.commitID != "headsha" {
		t.Fatalf("commit_id = %q, want headsha", review.commitID)
	}
	if !strings.Contains(review.body, reviewmd.SummaryMarker) {
		t.Fatalf("review body is not the summary: %q", review.body)
	}
	if len(review.comments) != 1 {
		t.Fatalf("inline comments = %d, want 1", len(review.comments))
	}
	c := review.comments[0]
	if c.Path != "main.go" || c.NewPosition != 1 {
		t.Fatalf("inline comment anchor wrong: %+v", c)
	}
	if !strings.Contains(c.Body, fpMarker("finding-a", "main.go", "Inline issue")) {
		t.Fatalf("inline comment missing finding marker: %q", c.Body)
	}
	if len(ps.issuePosts) != 1 {
		t.Fatalf("issue comments = %d, want 1 (finding-b)", len(ps.issuePosts))
	}
	if !strings.Contains(ps.issuePosts[0], "`other.go:5`") || !strings.Contains(ps.issuePosts[0], fpMarker("finding-b", "other.go", "Out-of-diff issue")) {
		t.Fatalf("overflow comment missing prefix/marker: %q", ps.issuePosts[0])
	}
}

func TestPublishReviewDedupeSkipsExisting(t *testing.T) {
	ps := newPublishServer(t)
	// Summary already posted as a prior review; finding-a already an inline
	// comment of that review (read per review, so the review has to say it
	// carries comments).
	ps.reviewsBody = []byte(`[{"id":1,"comments_count":1,"body":"` + reviewmd.SummaryMarker + `"}]`)
	ps.commentsB = []byte(`[{"body":"prefix ` + fpMarker("finding-a", "main.go", "Inline issue") + ` suffix"}]`)
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Summary + finding-a deduped, finding-a was the only inline => no review at all.
	if len(ps.reviewPosts) != 0 {
		t.Fatalf("review posts = %d, want 0 (all inline deduped)", len(ps.reviewPosts))
	}
	// One visible comment (finding-b) plus hidden carrier chunk(s) covering the
	// suppressed summary/finding so a chat can still reassemble this run.
	visible, _ := splitCarrierPosts(ps.issuePosts)
	if len(visible) != 1 || !strings.Contains(visible[0], fpMarker("finding-b", "other.go", "Out-of-diff issue")) {
		t.Fatalf("expected only finding-b visible comment, got %v", visible)
	}
	if got := reviewmd.ReviewResultsByID(ps.issuePosts)["rev-pub"]; got == nil || len(got.Findings) != 2 {
		t.Fatalf("carriers must cover the deduped run: %+v", got)
	}
}

// splitCarrierPosts partitions posted bodies into visible comments and pure
// hidden-carrier chunks (bodies that strip to nothing).
func splitCarrierPosts(posts []string) (visible, carriers []string) {
	for _, post := range posts {
		if reviewmd.StripMarkers(post) == "" {
			carriers = append(carriers, post)
			continue
		}
		visible = append(visible, post)
	}
	return visible, carriers
}

func TestPublishReviewNewInlineAfterSummaryUsesFallbackBody(t *testing.T) {
	ps := newPublishServer(t)
	// Summary already posted, but finding-a is new => review must carry a
	// non-empty body.
	ps.reviewsBody = []byte(`[{"id":1,"body":"` + reviewmd.SummaryMarker + `"}]`)
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(ps.reviewPosts) != 1 {
		t.Fatalf("review posts = %d, want 1", len(ps.reviewPosts))
	}
	review := ps.reviewPosts[0]
	if strings.TrimSpace(review.body) == "" {
		t.Fatal("review body must not be blank when summary was already posted")
	}
	if strings.Contains(review.body, reviewmd.SummaryMarker) {
		t.Fatalf("must not repost the summary marker: %q", review.body)
	}
}

func TestPublishReviewRejectedAnchorFallsBackToIssueComments(t *testing.T) {
	ps := newPublishServer(t)
	ps.rejectInline = true
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish should tolerate the fallback: %v", err)
	}
	// First review (with comments) is rejected; body-only summary review retried.
	if len(ps.reviewPosts) != 2 {
		t.Fatalf("review posts = %d, want 2 (rejected + body-only retry)", len(ps.reviewPosts))
	}
	if len(ps.reviewPosts[1].comments) != 0 || !strings.Contains(ps.reviewPosts[1].body, reviewmd.SummaryMarker) {
		t.Fatalf("retry review should be body-only summary: %+v", ps.reviewPosts[1])
	}
	// finding-a (degraded from inline) + finding-b (overflow) => 2 issue comments.
	if len(ps.issuePosts) != 2 {
		t.Fatalf("issue comments = %d, want 2", len(ps.issuePosts))
	}
	var sawA bool
	for _, body := range ps.issuePosts {
		if strings.Contains(body, fpMarker("finding-a", "main.go", "Inline issue")) && strings.Contains(body, "`main.go:1`") {
			sawA = true
		}
	}
	if !sawA {
		t.Fatal("finding-a did not fall back to a file:line issue comment")
	}
}

// Forgejo does not commit to one status for a bad anchor, so any answer from
// the API on the review with inline comments takes the fallback; a body-only
// review that still fails is a real failure and is reported.
func TestPublishReviewServerErrorStillReported(t *testing.T) {
	ps := newPublishServer(t)
	ps.reviewStatus = http.StatusInternalServerError
	err := ps.adapter().PublishReview(context.Background(), req(), sampleResult())
	if err == nil || !strings.Contains(err.Error(), "summary review") {
		t.Fatalf("err = %v, want the failed summary retry reported", err)
	}
	// The inline finding degraded to an issue comment on the way.
	var sawA bool
	for _, body := range ps.issuePosts {
		if strings.Contains(body, fpMarker("finding-a", "main.go", "Inline issue")) {
			sawA = true
		}
	}
	if !sawA {
		t.Fatal("finding-a should have fallen back to an issue comment")
	}
}

// A previous run's fallback carriers are pruned once the current run's are
// posted; a carrier the current run wrote, and one a visible ref still points
// at, stay.
func TestPublishReviewPrunesStaleCarriers(t *testing.T) {
	ps := newPublishServer(t)
	stale := reviewmd.NewRenderer("").CarrierNotes(&model.ReviewResult{ReviewID: "rev-old", OverallCorrectness: "patch is correct"}, nil)
	if len(stale) != 1 {
		t.Fatalf("carrier notes = %d", len(stale))
	}
	ps.issuesBody = []byte(`[{"id":77,"body":` + jsonString(stale[0]) + `}]`)
	if err := ps.adapter().PublishReview(context.Background(), req(), sampleResult()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(ps.deleted) != 1 || ps.deleted[0] != "77" {
		t.Fatalf("deleted = %v, want the stale carrier 77", ps.deleted)
	}
}

func jsonString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}
