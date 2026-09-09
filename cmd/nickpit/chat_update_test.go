package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

func TestLinkedFindingMessagesTrustScopeAndHistory(t *testing.T) {
	render := reviewmd.NewRenderer("").ForReview("original")
	root, _ := render.FindingBodyCarried(model.Finding{ID: "finding", Title: "Current", Body: "Current evidence"}, "")
	root, err := reviewmd.WithHistory("Obsolete archived evidence", root, "Finding", time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	note := func(id, author int, body string) map[string]any {
		return map[string]any{"id": id, "body": body, "author": map[string]any{"id": author, "username": "author"}}
	}
	discussion := func(id string, notes ...map[string]any) map[string]any {
		return map[string]any{"id": id, "notes": notes}
	}
	other, _ := reviewmd.NewRenderer("").ForReview("later-review").FindingBodyCarried(model.Finding{ID: "finding", Title: "Other"}, "")
	redirect := "Finding moved.\n" + reviewmd.FindingReferenceMarker("original", "finding")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/discussions") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode([]any{
			discussion("current", note(3, 7, root), note(8, 8, "Current evidence reply"), note(12, 8, "Too new")),
			discussion("old", note(1, 7, redirect), note(4, 8, "Previous thread evidence")),
			discussion("other", note(2, 7, other), note(6, 8, "Wrong review")),
			discussion("forged", note(5, 8, root), note(7, 8, "Untrusted root")),
		})
	}))
	defer server.Close()
	trigger := []glscm.DiscussionNote{
		{ID: 1, AuthorID: 7},
		{ID: 5, AuthorID: 8, Body: "Earlier question"},
		{AuthorID: 7, Body: "First fallback answer"},
		{AuthorID: 7, Body: "Second fallback answer"},
		{ID: 10, AuthorID: 8, Body: "Latest question"},
		{ID: 11, AuthorID: 7, Body: "Update scheduled"},
		{ID: 13, AuthorID: 8, Body: "Unrelated later question"},
	}
	messages, err := linkedFindingMessages(context.Background(), glscm.NewClient(server.URL, "token"), "p", 1, "original", []string{"finding"}, trigger, 10, 7, chatMessageControls{})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		role, content string
	}{
		{"user", "Previous thread evidence"},
		{"user", "Earlier question"},
		{"assistant", "First fallback answer"},
		{"assistant", "Second fallback answer"},
		{"user", "Current evidence reply"},
		{"user", "Latest question"},
	}
	if len(messages) != len(want) {
		t.Fatalf("unexpected linked transcript: %+v", messages)
	}
	for i, expected := range want {
		if messages[i].Role != expected.role || !strings.Contains(messages[i].Content, expected.content) {
			t.Fatalf("message %d = %+v, want role=%s content=%q", i, messages[i], expected.role, expected.content)
		}
	}
}
