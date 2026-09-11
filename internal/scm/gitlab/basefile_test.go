package gitlab

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// forkMRPayload models a fork MR: the source project belongs to the
// contributor, the target to the upstream group.
const forkMRPayload = `{
  "title": "Fork MR",
  "sha": "headsha",
  "source_branch": "feature",
  "target_branch": "main",
  "source_project_id": 99,
  "target_project_id": 42,
  "diff_refs": {"base_sha": "basesha", "head_sha": "headsha", "start_sha": "basesha"}
}`

func filesBody(t *testing.T, content string) []byte {
	t.Helper()
	body, err := json.Marshal(baseFileResponse{
		Encoding: "base64",
		Size:     len(content),
		Content:  base64.StdEncoding.EncodeToString([]byte(content)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// The whole point of reading the base: a fork MR must not be able to decide
// what the reviewers are told about the project.
func TestFetchBaseFileReadsTargetProjectAtBaseSHA(t *testing.T) {
	var filesPath, filesRef string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/merge_requests/7"):
			_, _ = w.Write([]byte(forkMRPayload))
		case strings.Contains(r.URL.Path, "/repository/files/"):
			filesPath = r.URL.EscapedPath()
			filesRef = r.URL.Query().Get("ref")
			_, _ = w.Write(filesBody(t, "deployment: internet-facing\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	data, found, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "group/proj", 7, ".nickpit/context.yaml")
	if err != nil {
		t.Fatalf("FetchBaseFile returned err: %v", err)
	}
	if !found {
		t.Fatal("found = false, want the file")
	}
	if string(data) != "deployment: internet-facing\n" {
		t.Fatalf("data = %q", data)
	}
	if !strings.Contains(filesPath, "/projects/42/repository/files/") {
		t.Fatalf("files path = %q, want the target project (42), not the source (99)", filesPath)
	}
	if !strings.Contains(filesPath, ".nickpit%2Fcontext.yaml") {
		t.Fatalf("files path = %q, want the file path in one encoded segment", filesPath)
	}
	if filesRef != "basesha" {
		t.Fatalf("ref = %q, want the diff base SHA rather than the head", filesRef)
	}
}

func TestFetchBaseFileFallsBackToTargetBranch(t *testing.T) {
	var filesRef string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/merge_requests/7") {
			_, _ = w.Write([]byte(`{"target_branch": "main", "target_project_id": 42, "sha": "headsha"}`))
			return
		}
		filesRef = r.URL.Query().Get("ref")
		_, _ = w.Write(filesBody(t, "criticality: high\n"))
	}))
	defer server.Close()

	if _, _, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "group/proj", 7, ".nickpit/context.yaml"); err != nil {
		t.Fatalf("FetchBaseFile returned err: %v", err)
	}
	if filesRef != "main" {
		t.Fatalf("ref = %q, want the target branch when no base SHA is reported", filesRef)
	}
}

func TestFetchBaseFileMissingIsNotAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/merge_requests/7") {
			_, _ = w.Write([]byte(forkMRPayload))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	data, found, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "group/proj", 7, ".nickpit/context.yaml")
	if err != nil {
		t.Fatalf("FetchBaseFile returned err: %v, want a missing file to be silent", err)
	}
	if found || data != nil {
		t.Fatalf("found = %v, data = %q; want neither", found, data)
	}
}

func TestFetchBaseFileRejectsUnusableResponses(t *testing.T) {
	tests := map[string]struct {
		body string
		want string
	}{
		"unsupported encoding": {body: `{"encoding":"text"}`, want: "unsupported content encoding"},
		"oversized":            {body: `{"encoding":"base64","size":2000000}`, want: "exceeds"},
		"corrupt base64":       {body: `{"encoding":"base64","size":4,"content":"!!!!"}`, want: "decoding"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/merge_requests/7") {
					_, _ = w.Write([]byte(forkMRPayload))
					return
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			_, _, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "group/proj", 7, ".nickpit/context.yaml")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestAdapterImplementsBaseFileSource(t *testing.T) {
	var _ model.BaseFileSource = NewAdapter(NewClient("https://gitlab.example", ""), "")
}
