package forgejo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/testutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	return testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "forgejo", name))
}

func fixtureServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	fixtures := map[string][]byte{
		"/api/v1/repos/owner/repo/pulls/123":                    fixture(t, "pr_metadata.json"),
		"/api/v1/repos/owner/repo/pulls/123/commits":            fixture(t, "pr_commits.json"),
		"/api/v1/repos/owner/repo/pulls/123.diff":               fixture(t, "pr.diff"),
		"/api/v1/repos/owner/repo/pulls/123/reviews":            fixture(t, "pr_reviews.json"),
		"/api/v1/repos/owner/repo/pulls/123/reviews/1/comments": fixture(t, "pr_review_comments.json"),
		"/api/v1/repos/owner/repo/issues/123/comments":          fixture(t, "pr_issue_comments.json"),
	}
	var auth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		data, ok := fixtures[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	return server, &auth
}

func TestFetchPR(t *testing.T) {
	server, auth := fixtureServer(t)

	// The bare host is enough: the client appends /api/v1 itself.
	client := NewClient(server.URL, "token")
	ctx, err := client.FetchPR(context.Background(), "owner/repo", 123, true)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Mode != model.ModeForgejo || ctx.Identifier != 123 || ctx.Title != "Example PR" {
		t.Fatalf("context = mode %q, id %d, title %q", ctx.Mode, ctx.Identifier, ctx.Title)
	}
	if ctx.Repository.URL != "https://codeberg.org/owner/repo/pulls/123" || ctx.Repository.BaseRef != "main" || ctx.Repository.HeadRef != "feature" {
		t.Fatalf("repository = %+v", ctx.Repository)
	}
	if ctx.DiffHeadSHA != "def" || ctx.DiffBaseSHA != "abc" {
		t.Fatalf("diff shas = %q..%q, want the merge base and the head", ctx.DiffBaseSHA, ctx.DiffHeadSHA)
	}
	// The changed files come from the diff itself: the modified file with its
	// line counts, and the pure rename that carries no hunk.
	if len(ctx.ChangedFiles) != 2 {
		t.Fatalf("changed files = %+v", ctx.ChangedFiles)
	}
	if f := ctx.ChangedFiles[0]; f.Path != "main.go" || f.Status != model.FileModified || f.Additions != 1 || f.Deletions != 1 {
		t.Fatalf("changed file = %+v", f)
	}
	if f := ctx.ChangedFiles[1]; f.Path != "docs/renamed.md" || f.Status != model.FileRenamed || f.OldPath != "docs/moved.md" {
		t.Fatalf("renamed file = %+v", f)
	}
	if len(ctx.DiffHunks) != 1 || ctx.DiffHunks[0].FilePath != "main.go" {
		t.Fatalf("hunks = %+v", ctx.DiffHunks)
	}
	if ctx.DiffOmitsFileModes {
		t.Fatal("the downloaded diff carries file modes")
	}
	if len(ctx.Commits) != 1 || ctx.Commits[0].SHA != "abc123" || ctx.Commits[0].Author != "dev" {
		t.Fatalf("commits = %+v", ctx.Commits)
	}
	// One review body (the empty approval is dropped), its inline comment
	// (fetched per review, only for reviews that have any), one issue comment.
	if len(ctx.Comments) != 3 {
		t.Fatalf("comments = %+v", ctx.Comments)
	}
	if c := ctx.Comments[0]; c.Body != "Looks risky" || !c.IsReview || c.Author != "reviewer" {
		t.Fatalf("review comment = %+v", c)
	}
	if c := ctx.Comments[1]; c.Path != "main.go" || c.Line != 1 || c.Side != "RIGHT" || !c.IsReview {
		t.Fatalf("inline comment = %+v", c)
	}
	if c := ctx.Comments[2]; c.Body != "Top level comment" || c.IsReview {
		t.Fatalf("issue comment = %+v", c)
	}
	for _, header := range *auth {
		if header != "token token" {
			t.Fatalf("authorization header = %q, want the token scheme", header)
		}
	}
}

func TestFetchPRWithoutComments(t *testing.T) {
	server, _ := fixtureServer(t)
	ctx, err := NewClient(server.URL, "token").FetchPR(context.Background(), "owner/repo", 123, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Comments) != 0 {
		t.Fatalf("comments = %+v, want none", ctx.Comments)
	}
}

func TestFetchPRAnchorsRemovedLineCommentsOnTheOldSide(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/pulls/9":
			_, _ = w.Write([]byte(`{"title":"t","body":"b","base":{"ref":"main"},"head":{"ref":"f","sha":"h"},"html_url":"u"}`))
		case "/api/v1/repos/owner/repo/pulls/9/reviews":
			_, _ = w.Write([]byte(`[{"id":4,"body":"","comments_count":2,"user":{"login":"u"}}]`))
		case "/api/v1/repos/owner/repo/pulls/9/reviews/4/comments":
			_, _ = w.Write([]byte(`[
				{"body":"removed","path":"main.go","position":0,"original_position":7,"user":{"login":"u"}},
				{"body":"current","path":"main.go","position":3,"original_position":9,"user":{"login":"u"}}
			]`))
		case "/api/v1/repos/owner/repo/pulls/9.diff":
			_, _ = w.Write([]byte(""))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	ctx, err := NewClient(server.URL, "token").FetchPR(context.Background(), "owner/repo", 9, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Comments) != 2 {
		t.Fatalf("comments = %+v", ctx.Comments)
	}
	if c := ctx.Comments[0]; c.Line != 7 || c.Side != "LEFT" {
		t.Fatalf("removed-line comment = %+v, want the old-side line", c)
	}
	if c := ctx.Comments[1]; c.Line != 3 || c.Side != "RIGHT" {
		t.Fatalf("current comment = %+v", c)
	}
}

func TestFetchPRCheckout(t *testing.T) {
	server, _ := fixtureServer(t)
	spec, err := NewClient(server.URL, "token").FetchPRCheckout(context.Background(), "owner/repo", 123)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Provider != model.ModeForgejo || spec.CloneURL != "https://codeberg.org/contrib/repo.git" || spec.HeadSHA != "def" || spec.HeadRef != "feature" {
		t.Fatalf("spec = %+v", spec)
	}
}

func TestFetchPRCheckoutDeletedHeadBranchUsesBaseRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"number": 5,
			"head": {"ref": "refs/pull/5/head", "sha": "def", "repo": {"clone_url": "https://codeberg.org/contrib/repo.git"}},
			"base": {"ref": "main", "sha": "abc", "repo": {"clone_url": "https://codeberg.org/owner/repo.git"}}
		}`))
	}))
	defer server.Close()

	spec, err := NewClient(server.URL, "token").FetchPRCheckout(context.Background(), "owner/repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if spec.CloneURL != "https://codeberg.org/owner/repo.git" || spec.HeadRef != "refs/pull/5/head" {
		t.Fatalf("spec = %+v, want the pull ref fetched from the base repository", spec)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := map[string]string{
		"":                                  "",
		"codeberg.org":                      "https://codeberg.org/api/v1",
		"https://codeberg.org":              "https://codeberg.org/api/v1",
		"https://codeberg.org/":             "https://codeberg.org/api/v1",
		"https://codeberg.org/api/v1":       "https://codeberg.org/api/v1",
		"http://forge.internal:3000/api/v1": "http://forge.internal:3000/api/v1",
		"  forge.internal  ":                "https://forge.internal/api/v1",
	}
	for raw, want := range cases {
		if got := NormalizeBaseURL(raw); got != want {
			t.Fatalf("NormalizeBaseURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestClientWithoutBaseURLFails(t *testing.T) {
	var out any
	err := NewClient("", "token").Get(context.Background(), "/user", &out)
	if err == nil || !strings.Contains(err.Error(), "no API base URL") {
		t.Fatalf("err = %v, want the missing base URL named", err)
	}
}

func TestGetRawRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunk := make([]byte, 32<<10)
		for remaining := maxResponseBytes + 1; remaining > 0; {
			n := min(remaining, len(chunk))
			if _, err := w.Write(chunk[:n]); err != nil {
				return
			}
			remaining -= n
		}
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "").GetRaw(context.Background(), "/oversized.diff")
	if err == nil || !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("GetRaw() error = %v, want response size limit error", err)
	}
}

// Forgejo's rel="next" links are absolute, AppURL plus the full request URI,
// so they repeat the API base's path. The server here only answers on the real
// endpoint path, so following a link that doubles the base would 404.
func TestGetPaginatedFollowsLinkAndAsksForFullPages(t *testing.T) {
	for _, tt := range []struct {
		name     string
		basePath string // where the client is configured to find the API
		linkPath string // the API path prefix the server writes into its links
	}{
		{name: "root instance", basePath: "", linkPath: "/api/v1"},
		{name: "subpath instance", basePath: "/forgejo", linkPath: "/forgejo/api/v1"},
		{name: "proxy rewrites the prefix", basePath: "/forgejo", linkPath: "/api/v1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := tt.basePath + "/api/v1/repos/o/r/pulls"
			var queries []string
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != endpoint {
					http.NotFound(w, r)
					return
				}
				queries = append(queries, r.URL.RawQuery)
				switch r.URL.Query().Get("page") {
				case "", "1":
					w.Header().Set("Link", "<"+server.URL+tt.linkPath+`/repos/o/r/pulls?limit=50&page=2&state=open>; rel="next"`)
					_, _ = w.Write([]byte(`[1,2]`))
				default:
					_, _ = w.Write([]byte(`[3]`))
				}
			}))
			defer server.Close()

			var out []int
			if err := NewClient(server.URL+tt.basePath, "").GetPaginated(context.Background(), "/repos/o/r/pulls?state=open", &out); err != nil {
				t.Fatal(err)
			}
			if len(out) != 3 || out[2] != 3 {
				t.Fatalf("out = %v", out)
			}
			if len(queries) != 2 || !strings.Contains(queries[0], "limit=50") || !strings.Contains(queries[0], "state=open") {
				t.Fatalf("queries = %q, want the first page to ask for a full page", queries)
			}
		})
	}
}

func TestGetPaginatedRejectsLinkCycles(t *testing.T) {
	var requests int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		next := "/api/v1/b"
		if r.URL.Path == "/api/v1/b" {
			next = "/api/v1/a" // A -> B -> A cycle
		}
		w.Header().Set("Link", "<"+server.URL+next+`>; rel="next"`)
		_, _ = w.Write([]byte(`[1]`))
	}))
	defer server.Close()

	var out []int
	err := NewClient(server.URL, "").GetPaginated(context.Background(), "/a", &out)
	if err == nil || !strings.Contains(err.Error(), "pagination cycle") {
		t.Fatalf("err = %v, want pagination cycle error", err)
	}
}

func TestGetPaginatedEnforcesPageCap(t *testing.T) {
	var requests int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Link", fmt.Sprintf(`<%s/api/v1/p%d>; rel="next"`, server.URL, requests))
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	var out []int
	if err := NewClient(server.URL, "").GetPaginated(context.Background(), "/p0", &out); err == nil {
		t.Fatal("expected page-cap error for endless pagination")
	}
	if requests != maxPaginatedPages {
		t.Fatalf("requests = %d, want %d", requests, maxPaginatedPages)
	}
}

func TestAPIErrorCarriesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	var out any
	err := NewClient(server.URL, "token").Get(context.Background(), "/repos/o/r/pulls/1", &out)
	if !IsNotFound(err) || !IsAPIError(err) {
		t.Fatalf("err = %v, want a 404 API error", err)
	}
	if !strings.Contains(err.Error(), "base URL") {
		t.Fatalf("a 404 should hint at the configuration: %v", err)
	}
}

func TestRetryAfterTime(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name   string
		header string // "" leaves the header out
		want   time.Time
	}{
		{name: "seconds", header: "120", want: now.Add(2 * time.Minute)},
		{name: "zero seconds", header: "0", want: now},
		{name: "padded seconds", header: " 5 ", want: now.Add(5 * time.Second)},
		{name: "HTTP date", header: "Fri, 02 Jan 2026 03:10:00 GMT", want: time.Date(2026, 1, 2, 3, 10, 0, 0, time.UTC)},
		{name: "absent"},
		{name: "negative seconds", header: "-5"},
		{name: "malformed", header: "soon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.header != "" {
				header.Set("Retry-After", tt.header)
			}
			if got := retryAfterTime(header, now); !got.Equal(tt.want) {
				t.Fatalf("retryAfterTime(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestAPIErrorPreservesRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		check  func(t *testing.T, before, after, got time.Time)
	}{
		{
			name:   "seconds",
			header: "2",
			check: func(t *testing.T, before, after, got time.Time) {
				if got.Before(before.Add(2*time.Second)) || got.After(after.Add(2*time.Second)) {
					t.Fatalf("retry after = %v, want two seconds after the response", got)
				}
			},
		},
		{
			name:   "HTTP date",
			header: "Fri, 02 Jan 2026 03:10:00 GMT",
			check: func(t *testing.T, _, _, got time.Time) {
				if !got.Equal(time.Date(2026, 1, 2, 3, 10, 0, 0, time.UTC)) {
					t.Fatalf("retry after = %v", got)
				}
			},
		},
		{
			name: "absent",
			check: func(t *testing.T, _, _, got time.Time) {
				if !got.IsZero() {
					t.Fatalf("retry after = %v, want none", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.header != "" {
					w.Header().Set("Retry-After", tt.header)
				}
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("slow down"))
			}))
			defer server.Close()

			before := time.Now()
			err := NewClient(server.URL, "token").Post(context.Background(), "/repos/o/r/issues/1/reactions", map[string]string{"content": "eyes"}, nil)
			after := time.Now()
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != http.StatusTooManyRequests {
				t.Fatalf("err = %v, want a 429 API error", err)
			}
			tt.check(t, before, after, apiErr.RetryAfter)
			// The retry time travels in the field only; the message is unchanged.
			if want := "forgejo: POST " + server.URL + "/api/v1/repos/o/r/issues/1/reactions: status 429: slow down"; err.Error() != want {
				t.Fatalf("message = %q, want %q", err.Error(), want)
			}
		})
	}
}
