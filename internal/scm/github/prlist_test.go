package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListOpenPRs(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		_, _ = w.Write([]byte(`[
			{"number":60,"title":"fix(llm): retry on 429","html_url":"https://github.com/o/r/pull/60",
			 "draft":false,"updated_at":"2026-09-01T10:00:00Z","user":{"login":"bob"},
			 "head":{"ref":"fix/retry"},"base":{"ref":"main"}},
			{"number":61,"title":"feat(review): cluster merge","html_url":"https://github.com/o/r/pull/61",
			 "draft":true,"updated_at":"2026-09-08T10:00:00Z","user":{"login":"alice"},
			 "head":{"ref":"feat/cluster"},"base":{"ref":"main"}},
			{"number":0,"title":"malformed"}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	requests, err := client.ListOpenPRs(context.Background(), "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/repos/owner/repo/pulls" {
		t.Fatalf("path = %q", gotPath)
	}
	for _, want := range []string{"state=open", "sort=updated", "direction=desc", "per_page=100"} {
		if !strings.Contains(gotQuery, want) {
			t.Fatalf("query = %q, want it to carry %q", gotQuery, want)
		}
	}
	// A number-less entry cannot be addressed by --id, so it is dropped.
	if len(requests) != 2 {
		t.Fatalf("requests = %+v, want 2", requests)
	}
	if requests[0].Identifier != 61 || requests[1].Identifier != 60 {
		t.Fatalf("order = %d, %d; want 61 before 60", requests[0].Identifier, requests[1].Identifier)
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
	if first.WebURL != "https://github.com/o/r/pull/61" {
		t.Fatalf("web url = %q", first.WebURL)
	}
}

func TestListOpenPRsBreaksTiesDeterministically(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"number":7,"title":"a","updated_at":"2026-09-01T10:00:00Z"},
			{"number":9,"title":"b","updated_at":"2026-09-01T10:00:00Z"}
		]`))
	}))
	defer server.Close()

	requests, err := NewClient(server.URL, "token").ListOpenPRs(context.Background(), "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0].Identifier != 9 {
		t.Fatalf("requests = %+v, want 9 first", requests)
	}
}

func TestListOpenPRsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	requests, err := NewClient(server.URL, "token").ListOpenPRs(context.Background(), "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests = %+v, want none", requests)
	}
}

func TestListOpenPRsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, "token").ListOpenPRs(context.Background(), "o/r"); err == nil {
		t.Fatal("expected an error")
	}
}

// ListPRs is the same listing over every state, for a caller reading what was
// published on a request rather than looking for one to review.
func TestListPRsAsksForEveryState(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"number":7,"title":"merged work","updated_at":"2026-09-01T10:00:00Z",
			"user":{"login":"bob"},"head":{"ref":"feat/x"},"base":{"ref":"main"}}]`))
	}))
	defer server.Close()

	requests, err := NewClient(server.URL, "token").ListPRs(context.Background(), "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "state=all") {
		t.Fatalf("query = %q, want every state", gotQuery)
	}
	if len(requests) != 1 || requests[0].Identifier != 7 {
		t.Fatalf("requests = %+v", requests)
	}
}
