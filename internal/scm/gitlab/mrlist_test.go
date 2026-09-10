package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListOpenMRs(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		_, _ = w.Write([]byte(`[
			{"iid":139,"title":"fix(llm): retry on 429","web_url":"https://gl/x/-/merge_requests/139",
			 "source_branch":"fix/retry","target_branch":"main","draft":false,
			 "updated_at":"2026-09-01T10:00:00Z","author":{"username":"bob"}},
			{"iid":142,"title":"feat(review): cluster merge","web_url":"https://gl/x/-/merge_requests/142",
			 "source_branch":"feat/cluster","target_branch":"main","draft":true,
			 "updated_at":"2026-09-08T10:00:00Z","author":{"username":"alice"}},
			{"iid":0,"title":"malformed"}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	requests, err := client.ListOpenMRs(context.Background(), "group/sub/project")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v4/projects/group%2Fsub%2Fproject/merge_requests" {
		t.Fatalf("path = %q, want the URL-escaped project path", gotPath)
	}
	for _, want := range []string{"state=opened", "order_by=updated_at", "sort=desc", "per_page=100"} {
		if !strings.Contains(gotQuery, want) {
			t.Fatalf("query = %q, want it to carry %q", gotQuery, want)
		}
	}
	// An entry without an IID cannot be addressed by --id, so it is dropped
	// rather than offered as an unusable row.
	if len(requests) != 2 {
		t.Fatalf("requests = %+v, want 2", requests)
	}
	// Newest activity first, regardless of the order the server sent.
	if requests[0].Identifier != 142 || requests[1].Identifier != 139 {
		t.Fatalf("order = %d, %d; want 142 before 139", requests[0].Identifier, requests[1].Identifier)
	}
	first := requests[0]
	if first.Title != "feat(review): cluster merge" || first.Author != "alice" || !first.Draft {
		t.Fatalf("first request = %+v", first)
	}
	if first.SourceBranch != "feat/cluster" || first.TargetBranch != "main" {
		t.Fatalf("branches = %+v", first)
	}
	if !first.UpdatedAt.Equal(time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("updated at = %s", first.UpdatedAt)
	}
	if first.WebURL != "https://gl/x/-/merge_requests/142" {
		t.Fatalf("web url = %q", first.WebURL)
	}
}

// Equal timestamps must not leave the order to the map/server: the higher IID
// (the newer MR) comes first, every run.
func TestListOpenMRsBreaksTiesDeterministically(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"iid":7,"title":"a","updated_at":"2026-09-01T10:00:00Z"},
			{"iid":9,"title":"b","updated_at":"2026-09-01T10:00:00Z"}
		]`))
	}))
	defer server.Close()

	requests, err := NewClient(server.URL, "token").ListOpenMRs(context.Background(), "g/p")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0].Identifier != 9 {
		t.Fatalf("requests = %+v, want 9 first", requests)
	}
}

func TestListOpenMRsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	requests, err := NewClient(server.URL, "token").ListOpenMRs(context.Background(), "g/p")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests = %+v, want none", requests)
	}
}

func TestListOpenMRsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, "token").ListOpenMRs(context.Background(), "g/p"); err == nil {
		t.Fatal("expected an error")
	}
}
