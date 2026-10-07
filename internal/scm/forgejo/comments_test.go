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

	var comment IssueComment
	if err := NewClient(server.URL, "token").CreateIssueComment(context.Background(), "owner/repo", 123, "On it.", &comment); err != nil {
		t.Fatal(err)
	}
	if comment.ID != 77 {
		t.Fatalf("comment id = %d, want 77", comment.ID)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/repos/owner/repo/issues/123/comments" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if len(gotBody) != 1 || gotBody["body"] != "On it." {
		t.Fatalf("body = %#v", gotBody)
	}
}

// Only a caller that asked for the created comment fails on a 2xx body it
// cannot decode; the comment itself was committed either way.
func TestCreateIssueCommentResponseBody(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		decode  bool
		wantErr bool
	}{
		{name: "empty body, not decoded", body: "", decode: false},
		{name: "non-JSON body, not decoded", body: "<html>ok</html>", decode: false},
		{name: "non-JSON body, decoded", body: "<html>ok</html>", decode: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			var created *IssueComment
			if tc.decode {
				created = &IssueComment{}
			}
			err := NewClient(server.URL, "token").CreateIssueComment(context.Background(), "owner/repo", 123, "On it.", created)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestCreateIssueCommentSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"This issue is locked."}`))
	}))
	defer server.Close()

	var comment IssueComment
	err := NewClient(server.URL, "token").CreateIssueComment(context.Background(), "owner/repo", 123, "On it.", &comment)
	var apiErr *APIError
	if comment.ID != 0 || !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("id = %d, err = %v, want a 403 API error", comment.ID, err)
	}
}
