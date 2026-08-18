package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/testutil"
)

func TestAPIErrorPreservesRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := NewClient(server.URL, "token")
	before := time.Now()
	err := client.AwardMREmoji(context.Background(), 42, 7, "eyes")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want APIError", err)
	}
	if apiErr.RetryAfter.Before(before.Add(1900*time.Millisecond)) || apiErr.RetryAfter.After(before.Add(3*time.Second)) {
		t.Fatalf("retry after = %v, want about two seconds after response", apiErr.RetryAfter)
	}
}

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
	// The diff identity rides on the context so chat sessions can persist it and
	// verify cache freshness without a spurious first-resume refresh.
	if ctx.DiffHeadSHA != "abc123" || ctx.DiffBaseSHA != "base456" {
		t.Fatalf("diff identity = head %q base %q, want abc123/base456", ctx.DiffHeadSHA, ctx.DiffBaseSHA)
	}
}

func TestFetchMRSurfacesDiffOverflow(t *testing.T) {
	fixtures := map[string][]byte{
		"/api/v4/projects/group%2Fproject/merge_requests/456":         testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_metadata.json")),
		"/api/v4/projects/group%2Fproject/merge_requests/456/commits": testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_commits.json")),
		"/api/v4/projects/group%2Fproject/merge_requests/456/changes": []byte(`{"overflow": true, "changes": [{"new_path": "a.go", "diff": "@@ -1 +1 @@\n-x\n+y\n"}]}`),
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
	ctx, err := client.FetchMR(context.Background(), "group/project", 456, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, note := range ctx.OmittedSections {
		if strings.Contains(strings.ToLower(note), "truncated") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected diff-overflow warning in OmittedSections, got %#v", ctx.OmittedSections)
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

func TestGetPaginatedRejectsPageCycles(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		next := "2"
		if r.URL.Query().Get("page") == "2" {
			next = "1" // 1 -> 2 -> 1 cycle (page 1 carries no page param)
		}
		w.Header().Set("X-Next-Page", next)
		_, _ = w.Write([]byte(`[1]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	var out []int
	err := client.GetPaginated(context.Background(), "/items", &out)
	if err == nil || !strings.Contains(err.Error(), "pagination cycle") {
		t.Fatalf("err = %v, want pagination cycle error", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 (cycle detected when revisiting page 1)", requests)
	}
}

func TestGetPaginatedRejectsSelfLoop(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// X-Next-Page never advances past the first page.
		w.Header().Set("X-Next-Page", "1")
		_, _ = w.Write([]byte(`[1]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	var out []int
	err := client.GetPaginated(context.Background(), "/items", &out)
	if err == nil || !strings.Contains(err.Error(), "pagination cycle") {
		t.Fatalf("err = %v, want pagination cycle error", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (self-loop detected before refetching)", requests)
	}
}

func TestGetPaginatedEnforcesPageCap(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// Always advance to a fresh page so only the cap can stop the loop.
		w.Header().Set("X-Next-Page", strconv.Itoa(requests+1))
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	var out []int
	err := client.GetPaginated(context.Background(), "/items", &out)
	if err == nil {
		t.Fatal("expected page-cap error for endless pagination")
	}
	if requests != maxPaginatedPages {
		t.Fatalf("requests = %d, want %d", requests, maxPaginatedPages)
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

func TestPost(t *testing.T) {
	var gotMethod, gotContentType, gotToken string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 7}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	var out struct {
		ID int `json:"id"`
	}
	if err := client.Post(context.Background(), "/projects/1/merge_requests/2/notes", map[string]string{"body": "hi"}, &out); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	if gotToken != "token" {
		t.Fatalf("token header = %q", gotToken)
	}
	if gotBody["body"] != "hi" {
		t.Fatalf("request body = %#v", gotBody)
	}
	if out.ID != 7 {
		t.Fatalf("decoded id = %d, want 7", out.ID)
	}
}

func TestPostStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"line not part of the diff"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	err := client.Post(context.Background(), "/projects/1/merge_requests/2/discussions", map[string]string{"body": "x"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %v", err)
	}
	if apiErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", apiErr.Status)
	}
	if !strings.Contains(err.Error(), "gitlab: POST") {
		t.Fatalf("error %q must report POST method", err.Error())
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

// GitLab's per-change `diff` starts at the first hunk header, so the only place
// a symlink shows up is a_mode/b_mode. Both the changed-file flag and a mode
// header line in the synthesized diff must carry that fact to the reviewer:
// without it the link target reads as a one-line text file missing a newline.
func TestFetchMRMarksSymlinkChange(t *testing.T) {
	changes := `{"changes": [
		{"new_path": "deploy/crd-chart/templates", "old_path": "deploy/crd-chart/templates",
		 "new_file": true, "deleted_file": false, "renamed_file": false,
		 "a_mode": "0", "b_mode": "120000",
		 "diff": "@@ -0,0 +1 @@\n+../../config/crd/bases\n\\ No newline at end of file\n"},
		{"new_path": "deploy/crd-chart/Chart.yaml", "old_path": "deploy/crd-chart/Chart.yaml",
		 "new_file": true, "deleted_file": false, "renamed_file": false,
		 "a_mode": "0", "b_mode": "100644",
		 "diff": "@@ -0,0 +1 @@\n+apiVersion: v2\n"}
	]}`
	fixtures := map[string][]byte{
		"/api/v4/projects/group%2Fproject/merge_requests/456":         testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_metadata.json")),
		"/api/v4/projects/group%2Fproject/merge_requests/456/commits": testutil.LoadFixture(t, filepath.Join("..", "..", "..", "testdata", "fixtures", "gitlab", "mr_commits.json")),
		"/api/v4/projects/group%2Fproject/merge_requests/456/changes": []byte(changes),
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
	ctx, err := client.FetchMR(context.Background(), "group/project", 456, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.ChangedFiles) != 2 {
		t.Fatalf("changed files = %#v, want two entries", ctx.ChangedFiles)
	}
	if !ctx.ChangedFiles[0].Symlink {
		t.Fatalf("symlink change not marked: %#v", ctx.ChangedFiles[0])
	}
	if ctx.ChangedFiles[1].Symlink {
		t.Fatalf("regular file marked as symlink: %#v", ctx.ChangedFiles[1])
	}
	if !strings.Contains(ctx.Diff, "new file mode 120000") {
		t.Fatalf("diff lacks symlink mode header:\n%s", ctx.Diff)
	}
	// Only symlink changes gain a mode line; the common shape stays untouched.
	if strings.Contains(ctx.Diff, "new file mode 100644") {
		t.Fatalf("regular file gained a mode header:\n%s", ctx.Diff)
	}
	for _, file := range ctx.DiffFiles {
		if file.FilePath == "deploy/crd-chart/templates" && !file.Symlink {
			t.Fatalf("diff file not marked as symlink: %#v", file)
		}
	}
}

func TestSymlinkModeHeader(t *testing.T) {
	tests := []struct {
		name    string
		aMode   string
		bMode   string
		newFile bool
		deleted bool
		want    string
	}{
		{name: "added symlink", aMode: "0", bMode: "120000", newFile: true, want: "new file mode 120000\n"},
		{name: "deleted symlink", aMode: "120000", bMode: "0", deleted: true, want: "deleted file mode 120000\n"},
		{name: "changed symlink target", aMode: "120000", bMode: "120000", want: "old mode 120000\nnew mode 120000\n"},
		{name: "symlink turned into file", aMode: "120000", bMode: "100644", want: "old mode 120000\nnew mode 100644\n"},
		{name: "regular file", aMode: "100644", bMode: "100644", want: ""},
		{name: "missing modes", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := symlinkModeHeader(tc.aMode, tc.bMode, tc.newFile, tc.deleted); got != tc.want {
				t.Fatalf("symlinkModeHeader() = %q, want %q", got, tc.want)
			}
		})
	}
}
