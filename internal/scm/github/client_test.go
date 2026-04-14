package github

import (
	"context"
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
