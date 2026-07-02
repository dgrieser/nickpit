package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dgrieser/nickpit/internal/testutil"
)

func TestFetchPR(t *testing.T) {
	fixtures := map[string][]byte{
		"/repos/owner/repo/pulls/123":           testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "github", "pr_metadata.json")),
		"/repos/owner/repo/pulls/123/commits":   testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "github", "pr_commits.json")),
		"/repos/owner/repo/pulls/123/files":     testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "github", "pr_files.json")),
		"/repos/owner/repo/pulls/123/reviews":   testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "github", "pr_reviews.json")),
		"/repos/owner/repo/pulls/123/comments":  testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "github", "pr_review_comments.json")),
		"/repos/owner/repo/issues/123/comments": testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "github", "pr_issue_comments.json")),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := fixtures[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ctx, err := client.FetchPR(context.Background(), "owner/repo", 123, true)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Title != "Example PR" {
		t.Fatalf("title = %q", ctx.Title)
	}
	if ctx.Identifier != 123 {
		t.Fatalf("identifier = %d", ctx.Identifier)
	}
	if ctx.Repository.URL != "https://github.com/owner/repo/pull/123" {
		t.Fatalf("repository url = %q", ctx.Repository.URL)
	}
	if len(ctx.ChangedFiles) != 1 {
		t.Fatalf("changed files = %d", len(ctx.ChangedFiles))
	}
}

func TestFetchPRCheckout(t *testing.T) {
	fixtures := map[string][]byte{
		"/repos/owner/repo/pulls/123": testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "github", "pr_metadata.json")),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := fixtures[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	spec, err := client.FetchPRCheckout(context.Background(), "owner/repo", 123)
	if err != nil {
		t.Fatal(err)
	}
	if spec.CloneURL != "https://github.com/contrib/repo.git" {
		t.Fatalf("clone url = %q", spec.CloneURL)
	}
	if spec.HeadSHA != "def" {
		t.Fatalf("head sha = %q", spec.HeadSHA)
	}
}

func TestGetPaginatedBreaksLinkCycles(t *testing.T) {
	var requests int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		next := "/b"
		if r.URL.Path == "/b" {
			next = "/a" // A -> B -> A cycle
		}
		w.Header().Set("Link", "<"+server.URL+next+`>; rel="next"`)
		_, _ = w.Write([]byte(`[1]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	var out []int
	if err := client.GetPaginated(context.Background(), "/a", &out); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 (cycle broken after revisiting /a)", requests)
	}
	if len(out) != 2 {
		t.Fatalf("items = %d, want 2", len(out))
	}
}

func TestGetPaginatedStopsSelfLoop(t *testing.T) {
	var requests int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Link", "<"+server.URL+r.URL.Path+`>; rel="next"`)
		_, _ = w.Write([]byte(`[1]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	var out []int
	if err := client.GetPaginated(context.Background(), "/self", &out); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(out) != 1 {
		t.Fatalf("requests/items = %d/%d, want 1/1", requests, len(out))
	}
}

func TestGetPaginatedEnforcesPageCap(t *testing.T) {
	var requests int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Link", fmt.Sprintf(`<%s/p%d>; rel="next"`, server.URL, requests))
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	var out []int
	err := client.GetPaginated(context.Background(), "/p0", &out)
	if err == nil {
		t.Fatal("expected page-cap error for endless pagination")
	}
	if requests != maxPaginatedPages {
		t.Fatalf("requests = %d, want %d", requests, maxPaginatedPages)
	}
}

func TestFetchPRFallsBackToOriginalLineForOutdatedComments(t *testing.T) {
	responses := map[string]string{
		"/repos/owner/repo/pulls/9":           `{"title":"t","body":"b","base":{"ref":"main"},"head":{"ref":"f"},"html_url":"u"}`,
		"/repos/owner/repo/pulls/9/commits":   `[]`,
		"/repos/owner/repo/pulls/9/files":     `[]`,
		"/repos/owner/repo/pulls/9/reviews":   `[]`,
		"/repos/owner/repo/issues/9/comments": `[]`,
		"/repos/owner/repo/pulls/9/comments": `[
			{"body":"outdated","path":"main.go","line":null,"original_line":7,"side":"RIGHT","user":{"login":"u"}},
			{"body":"current","path":"main.go","line":3,"original_line":9,"side":"RIGHT","user":{"login":"u"}}
		]`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := responses[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(data))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ctx, err := client.FetchPR(context.Background(), "owner/repo", 9, true)
	if err != nil {
		t.Fatal(err)
	}
	byBody := map[string]int{}
	for _, comment := range ctx.Comments {
		byBody[comment.Body] = comment.Line
	}
	if byBody["outdated"] != 7 {
		t.Fatalf("outdated comment line = %d, want fallback to original_line 7", byBody["outdated"])
	}
	if byBody["current"] != 3 {
		t.Fatalf("current comment line = %d, want 3", byBody["current"])
	}
}
