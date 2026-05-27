package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/testutil"
)

func TestFetchMR(t *testing.T) {
	fixtures := map[string][]byte{
		"/api/v4/projects/group%2Fproject/merge_requests/456":             testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_metadata.json")),
		"/api/v4/projects/group%2Fproject/merge_requests/456/commits":     testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_commits.json")),
		"/api/v4/projects/group%2Fproject/merge_requests/456/changes":     testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_changes.json")),
		"/api/v4/projects/group%2Fproject/merge_requests/456/discussions": testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_discussions.json")),
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
	if ctx.Identifier != 456 {
		t.Fatalf("identifier = %d", ctx.Identifier)
	}
	if ctx.Repository.URL != "https://gitlab.com/group/project/-/merge_requests/456" {
		t.Fatalf("repository url = %q", ctx.Repository.URL)
	}
	if len(ctx.Comments) != 1 {
		t.Fatalf("comments = %d", len(ctx.Comments))
	}
}

func TestFetchMRCheckout(t *testing.T) {
	fixtures := map[string][]byte{
		"/api/v4/projects/group%2Fproject/merge_requests/456": testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_metadata.json")),
		"/api/v4/projects/99": []byte(`{"http_url_to_repo":"https://gitlab.com/fork/project.git"}`),
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
	spec, err := client.FetchMRCheckout(context.Background(), "group/project", 456)
	if err != nil {
		t.Fatal(err)
	}
	if spec.CloneURL != "https://gitlab.com/fork/project.git" {
		t.Fatalf("clone url = %q", spec.CloneURL)
	}
	if spec.HeadSHA != "abc123" {
		t.Fatalf("head sha = %q", spec.HeadSHA)
	}
}

func TestFetchMRErrorIncludesRequestHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Project Not Found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	_, err := client.FetchMR(context.Background(), "group/project", 456, false)
	if err == nil {
		t.Fatal("expected error")
	}
	got := err.Error()
	for _, want := range []string{
		"gitlab: GET " + server.URL + "/api/v4/projects/group%2Fproject/merge_requests/456: status 404",
		`{"message":"404 Project Not Found"}`,
		"check --repo, --id, --gitlab-base-url, and token project access",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error %q does not contain %q", got, want)
		}
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "https://gitlab.com/api/v4"},
		{"gitlab.example.com", "https://gitlab.example.com/api/v4"},
		{"gitlab.example.com/", "https://gitlab.example.com/api/v4"},
		{"https://gitlab.example.com", "https://gitlab.example.com/api/v4"},
		{"https://gitlab.example.com/", "https://gitlab.example.com/api/v4"},
		{"https://gitlab.example.com/api/v4", "https://gitlab.example.com/api/v4"},
		{"https://gitlab.example.com/api/v4/", "https://gitlab.example.com/api/v4"},
		{"https://gitlab.example.com/api/v3", "https://gitlab.example.com/api/v3"},
		{"http://localhost:8080", "http://localhost:8080/api/v4"},
		{"  https://gitlab.example.com  ", "https://gitlab.example.com/api/v4"},
	}
	for _, tc := range cases {
		got := NormalizeBaseURL(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFetchMRErrorOnNonJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign in</body></html>"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	_, err := client.FetchMR(context.Background(), "group/project", 456, false)
	if err == nil {
		t.Fatal("expected error")
	}
	got := err.Error()
	for _, want := range []string{
		"non-JSON body",
		"content-type=text/html",
		"check --gitlab-base-url",
		"/api/v4",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error %q does not contain %q", got, want)
		}
	}
}
