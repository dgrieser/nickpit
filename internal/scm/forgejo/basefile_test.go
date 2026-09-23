package forgejo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// forkPRPayload models a fork PR: the head lives in the contributor's
// repository, the base in the upstream one.
const forkPRPayload = `{
  "title": "Fork PR",
  "base": {"ref": "main", "sha": "basesha", "repo": {"full_name": "owner/repo", "clone_url": "https://example.test/owner/repo.git"}},
  "head": {"ref": "feature", "sha": "headsha", "repo": {"full_name": "attacker/repo", "clone_url": "https://example.test/attacker/repo.git"}}
}`

// The whole point of reading the base: a fork PR must not be able to decide
// what the reviewers are told about the project.
func TestFetchBaseFileReadsBaseRepoAtBaseSHA(t *testing.T) {
	var rawPath, rawRef string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/repos/owner/repo/pulls/7":
			_, _ = w.Write([]byte(forkPRPayload))
		case strings.HasPrefix(r.URL.Path, "/api/v1/repos/owner/repo/raw/"):
			rawPath = r.URL.Path
			rawRef = r.URL.Query().Get("ref")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("deployment: internet-facing\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	data, found, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, ".nickpit/project.yaml")
	if err != nil {
		t.Fatalf("FetchBaseFile returned err: %v", err)
	}
	if !found {
		t.Fatal("found = false, want the file")
	}
	if string(data) != "deployment: internet-facing\n" {
		t.Fatalf("data = %q", data)
	}
	if rawPath != "/api/v1/repos/owner/repo/raw/.nickpit/project.yaml" {
		t.Fatalf("raw path = %q, want the base repository and an unescaped separator", rawPath)
	}
	if rawRef != "basesha" {
		t.Fatalf("ref = %q, want the base SHA rather than the head", rawRef)
	}
}

func TestFetchBaseFileFallsBackToBaseRef(t *testing.T) {
	var rawRef string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/pulls/7":
			_, _ = w.Write([]byte(`{"base": {"ref": "main", "repo": {"full_name": "owner/repo"}}, "head": {"ref": "f", "sha": "h"}}`))
		default:
			rawRef = r.URL.Query().Get("ref")
			_, _ = w.Write([]byte("criticality: high\n"))
		}
	}))
	defer server.Close()

	if _, _, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, ".nickpit/project.yaml"); err != nil {
		t.Fatalf("FetchBaseFile returned err: %v", err)
	}
	if rawRef != "main" {
		t.Fatalf("ref = %q, want the base branch when no SHA is reported", rawRef)
	}
}

func TestFetchBaseFileMissingIsNotAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/owner/repo/pulls/7" {
			_, _ = w.Write([]byte(forkPRPayload))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	data, found, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, ".nickpit/project.yaml")
	if err != nil {
		t.Fatalf("FetchBaseFile returned err: %v, want a missing file to be silent", err)
	}
	if found || data != nil {
		t.Fatalf("found = %v, data = %q; want neither", found, data)
	}
}

func TestFetchBaseFileRejectsOversizedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/owner/repo/pulls/7" {
			_, _ = w.Write([]byte(forkPRPayload))
			return
		}
		_, _ = w.Write(make([]byte, model.MaxBaseFileBytes+1))
	}))
	defer server.Close()

	_, _, err := NewClient(server.URL, "token").FetchBaseFile(context.Background(), "owner/repo", 7, "big.yaml")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want the size cap enforced", err)
	}
}

func TestAdapterImplementsBaseFileSource(t *testing.T) {
	var _ model.BaseFileSource = NewAdapter(NewClient("", ""), "")
}
