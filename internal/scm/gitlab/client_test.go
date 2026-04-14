package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dgrieser/nickpit/internal/testutil"
)

func TestFetchMR(t *testing.T) {
	fixtures := map[string][]byte{
		"/projects/group%2Fproject/merge_requests/456":             testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_metadata.json")),
		"/projects/group%2Fproject/merge_requests/456/commits":     testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_commits.json")),
		"/projects/group%2Fproject/merge_requests/456/changes":     testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_changes.json")),
		"/projects/group%2Fproject/merge_requests/456/discussions": testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_discussions.json")),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := fixtures[r.URL.EscapedPath()]
		if !ok {
			data, ok = fixtures[r.URL.Path]
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ctx, err := client.FetchMR(context.Background(), "group/project", 456, true)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Title != "Example MR" {
		t.Fatalf("title = %q", ctx.Title)
	}
	if len(ctx.Comments) != 1 {
		t.Fatalf("comments = %d", len(ctx.Comments))
	}
}
