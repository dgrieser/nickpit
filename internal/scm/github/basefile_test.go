package github

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

// prPayload models a fork PR: the head lives in the contributor's repository,
// the base in the upstream one.
const forkPRPayload = `{
  "title": "Fork PR",
  "base": {"ref": "main", "sha": "basesha", "repo": {"full_name": "owner/repo", "clone_url": "https://example.test/owner/repo.git"}},
  "head": {"ref": "feature", "sha": "headsha", "repo": {"full_name": "attacker/repo", "clone_url": "https://example.test/attacker/repo.git"}}
}`

func contentsBody(t *testing.T, content string) []byte {
	t.Helper()
	// GitHub wraps the base64 payload at 60 columns.
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	var wrapped strings.Builder
	for i := 0; i < len(encoded); i += 60 {
		end := min(i+60, len(encoded))
		wrapped.WriteString(encoded[i:end] + "\n")
	}
	body, err := json.Marshal(baseFileResponse{
		Type:     "file",
		Encoding: "base64",
		Size:     len(content),
		Content:  wrapped.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// The whole point of reading the base: a fork PR must not be able to decide
// what the reviewers are told about the project.
func TestFetchBaseFileReadsBaseRepoAtBaseSHA(t *testing.T) {
	var contentsPath, contentsRef string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/pulls/7":
			_, _ = w.Write([]byte(forkPRPayload))
		case strings.HasPrefix(r.URL.Path, "/repos/owner/repo/contents/"):
			contentsPath = r.URL.Path
			contentsRef = r.URL.Query().Get("ref")
			_, _ = w.Write(contentsBody(t, "deployment: internet-facing\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	data, found, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, ".nickpit/context.yaml")
	if err != nil {
		t.Fatalf("FetchBaseFile returned err: %v", err)
	}
	if !found {
		t.Fatal("found = false, want the file")
	}
	if string(data) != "deployment: internet-facing\n" {
		t.Fatalf("data = %q", data)
	}
	if contentsPath != "/repos/owner/repo/contents/.nickpit/context.yaml" {
		t.Fatalf("contents path = %q, want the base repository and an unescaped separator", contentsPath)
	}
	if contentsRef != "basesha" {
		t.Fatalf("ref = %q, want the base SHA rather than the head", contentsRef)
	}
}

func TestFetchBaseFileFallsBackToBaseRef(t *testing.T) {
	var contentsRef string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/7":
			_, _ = w.Write([]byte(`{"base": {"ref": "main", "repo": {"full_name": "owner/repo"}}, "head": {"ref": "f", "sha": "h"}}`))
		default:
			contentsRef = r.URL.Query().Get("ref")
			_, _ = w.Write(contentsBody(t, "criticality: high\n"))
		}
	}))
	defer server.Close()

	if _, _, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, ".nickpit/context.yaml"); err != nil {
		t.Fatalf("FetchBaseFile returned err: %v", err)
	}
	if contentsRef != "main" {
		t.Fatalf("ref = %q, want the base branch when no SHA is reported", contentsRef)
	}
}

func TestFetchBaseFileMissingIsNotAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls/7" {
			_, _ = w.Write([]byte(forkPRPayload))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	data, found, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, ".nickpit/context.yaml")
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
		"directory":            {body: `{"type":"dir"}`, want: "not a file"},
		"unsupported encoding": {body: `{"type":"file","encoding":"none"}`, want: "unsupported content encoding"},
		"oversized":            {body: `{"type":"file","encoding":"base64","size":2000000}`, want: "exceeds"},
		"corrupt base64":       {body: `{"type":"file","encoding":"base64","size":4,"content":"!!!!"}`, want: "decoding"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/repos/owner/repo/pulls/7" {
					_, _ = w.Write([]byte(forkPRPayload))
					return
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			_, _, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, ".nickpit/context.yaml")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestAdapterImplementsBaseFileSource(t *testing.T) {
	var _ model.BaseFileSource = NewAdapter(NewClient("", ""), "")
}
