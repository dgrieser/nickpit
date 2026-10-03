package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateIssueComment(t *testing.T) {
	created := fixture(t, "issue_comment_created.json")
	var gotMethod, gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(created)
	}))
	defer server.Close()

	id, err := NewClient(server.URL, "token").CreateIssueComment(context.Background(), "owner/repo", 123, "On it.")
	if err != nil {
		t.Fatal(err)
	}
	if id != 77 {
		t.Fatalf("comment id = %d, want 77", id)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/repos/owner/repo/issues/123/comments" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if len(gotBody) != 1 || gotBody["body"] != "On it." {
		t.Fatalf("body = %#v", gotBody)
	}
}

func TestCreateIssueCommentSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"This issue is locked."}`))
	}))
	defer server.Close()

	id, err := NewClient(server.URL, "token").CreateIssueComment(context.Background(), "owner/repo", 123, "On it.")
	var apiErr *APIError
	if id != 0 || !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("id = %d, err = %v, want a 403 API error", id, err)
	}
}
