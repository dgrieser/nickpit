package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// carrierServer serves the three comment surfaces ReviewResults reads, plus
// /user for the author check.
type carrierServer struct {
	login    string
	userCode int // non-zero -> /user answers with this status instead of the login
	reviews  []map[string]any
	comments []map[string]any
	issues   []map[string]any
}

func (cs *carrierServer) start(t *testing.T) *Client {
	t.Helper()
	write := func(w http.ResponseWriter, payload any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			if cs.userCode != 0 {
				w.WriteHeader(cs.userCode)
				return
			}
			write(w, map[string]any{"login": cs.login, "id": 7})
		case "/repos/owner/repo/pulls/123/reviews":
			write(w, cs.reviews)
		case "/repos/owner/repo/pulls/123/comments":
			write(w, cs.comments)
		case "/repos/owner/repo/issues/123/comments":
			write(w, cs.issues)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL, "token")
}

func comment(login, body string) map[string]any {
	return map[string]any{"body": body, "user": map[string]any{"login": login}}
}

func carrierReview(t *testing.T) (*model.ReviewResult, []string) {
	t.Helper()
	result := &model.ReviewResult{
		ReviewID:           "rev-gh",
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "github carrier test",
		Repo:               "owner/repo",
		Identifier:         123,
		Findings: []model.Finding{
			{ID: "f1", Title: "One", Body: "b1", CodeLocation: model.CodeLocation{FilePath: "a.go"}},
			{ID: "f2", Title: "Two", Body: "b2", CodeLocation: model.CodeLocation{FilePath: "b.go"}},
		},
	}
	notes := reviewmd.NewRenderer("https://host/").CarrierNotes(result, result.Findings)
	if len(notes) == 0 {
		t.Fatal("renderer produced no carrier notes")
	}
	return result, notes
}

func TestReviewResultsReadsEveryCommentSurface(t *testing.T) {
	want, notes := carrierReview(t)
	// Spread the carriers over all three surfaces: a real publish puts the
	// summary envelope in the review body and the findings in inline comments,
	// with oversized ones falling back to issue comments.
	cs := &carrierServer{login: "nickpit-bot"}
	for i, note := range notes {
		switch i % 3 {
		case 0:
			cs.reviews = append(cs.reviews, comment("nickpit-bot", note))
		case 1:
			cs.comments = append(cs.comments, comment("NickPit-Bot", note)) // login casing folds
		default:
			cs.issues = append(cs.issues, comment("nickpit-bot", note))
		}
	}
	adapter := NewAdapter(cs.start(t), "https://host/")

	reviews, err := adapter.ReviewResults(context.Background(), "owner/repo", 123)
	if err != nil {
		t.Fatalf("ReviewResults: %v", err)
	}
	got := reviews["rev-gh"]
	if got == nil {
		t.Fatalf("review did not reassemble: %v", reviews)
	}
	if got.OverallExplanation != want.OverallExplanation || len(got.Findings) != len(want.Findings) {
		t.Fatalf("reassembled review = %+v", got)
	}
}

func TestReviewResultsIgnoresForeignAndUnknownAuthors(t *testing.T) {
	_, notes := carrierReview(t)
	cs := &carrierServer{login: "nickpit-bot"}
	for _, note := range notes {
		// The very same markers, planted by someone else and by a deleted user.
		cs.issues = append(cs.issues, comment("attacker", note), comment("", note))
	}
	adapter := NewAdapter(cs.start(t), "https://host/")

	reviews, err := adapter.ReviewResults(context.Background(), "owner/repo", 123)
	if err != nil {
		t.Fatalf("ReviewResults: %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("forged carriers were trusted: %v", reviews)
	}
}

func TestReviewResultsFailsWhenTokenUserIsUnknown(t *testing.T) {
	// Without an author to compare against, every marker would have to be
	// trusted or none: fail instead of guessing (a GitHub App installation
	// token gets a 403 here).
	cs := &carrierServer{login: "nickpit-bot", userCode: http.StatusForbidden}
	adapter := NewAdapter(cs.start(t), "https://host/")

	_, err := adapter.ReviewResults(context.Background(), "owner/repo", 123)
	if err == nil || !strings.Contains(err.Error(), "resolving token user") {
		t.Fatalf("error = %v, want a token-user failure", err)
	}
}
