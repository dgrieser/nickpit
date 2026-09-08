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
		{ID: 9, AuthorID: 7},
		{ID: 10, AuthorID: 8, Body: "Latest question"},
		{ID: 11, AuthorID: 7, Body: "Update scheduled"},
		{ID: 13, AuthorID: 8, Body: "Unrelated later question"},
	}
	messages, err := linkedFindingMessages(context.Background(), glscm.NewClient(server.URL, "token"), "p", 1, "original", []string{"finding"}, trigger, 10, 7, chatMessageControls{})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || !strings.Contains(messages[0].Content, "Previous thread evidence") || !strings.Contains(messages[1].Content, "Current evidence reply") || !strings.Contains(messages[2].Content, "Latest question") {
		t.Fatalf("unexpected linked transcript: %+v", messages)
	}
}
