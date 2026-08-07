package gitlab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/42" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"id":42,"path_with_namespace":"group/project","default_branch":"main","topics":["nickpit","go"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	project, err := client.GetProject(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if project.ID != 42 || project.PathWithNamespace != "group/project" || project.DefaultBranch != "main" {
		t.Fatalf("project = %+v", project)
	}
	if len(project.Topics) != 2 || project.Topics[0] != "nickpit" {
		t.Fatalf("topics = %#v", project.Topics)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	_, err := client.GetProject(context.Background(), 42)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("expected 404 APIError, got %v", err)
	}
}

func TestCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"id":7,"username":"nickpit-bot"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != 7 || user.Username != "nickpit-bot" {
		t.Fatalf("user = %+v", user)
	}
}

func TestFetchMRStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/42/merge_requests/7" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"state":"opened","draft":true,"sha":"abc123"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	status, err := client.FetchMRStatus(context.Background(), 42, 7)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "opened" || !status.Draft || status.HeadSHA != "abc123" {
		t.Fatalf("status = %+v", status)
	}
}
