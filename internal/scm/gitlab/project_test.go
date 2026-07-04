package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

func TestAwardMREmoji(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1,"name":"eyes"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	if err := client.AwardMREmoji(context.Background(), 42, 7, "eyes"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v4/projects/42/merge_requests/7/award_emoji" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["name"] != "eyes" {
		t.Fatalf("body = %#v", gotBody)
	}
}

func TestAwardMREmojiToleratesAlreadyAwarded(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusConflict} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"Award Emoji Name has already been taken"}`))
		}))
		client := NewClient(server.URL, "token")
		if err := client.AwardMREmoji(context.Background(), 42, 7, "eyes"); err != nil {
			t.Fatalf("status %d: expected nil, got %v", status, err)
		}
		server.Close()
	}
}

func TestAwardMREmojiServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	if err := client.AwardMREmoji(context.Background(), 42, 7, "eyes"); err == nil {
		t.Fatal("expected error on 500")
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
