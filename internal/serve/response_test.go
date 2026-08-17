package serve

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

func TestResponseControllerCombinesReactionsCommandsAndFooter(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	rootBody := reviewFindingBody(model.Finding{ID: "finding-1", Title: "Finding", Body: "Details"})
	mrAwards := []map[string]any{}
	noteAwards := []map[string]any{{"id": 1, "name": "mute", "user": map[string]any{"id": 77}}} // own award: ignored

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/discussions/disc-1"):
			_ = json.NewEncoder(w).Encode(map[string]any{"notes": []map[string]any{{
				"id": 100, "body": rootBody, "author": map[string]any{"id": 77, "username": "nickpit"},
			}}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji") && strings.Contains(r.URL.Path, "/notes/100/"):
			_ = json.NewEncoder(w).Encode(noteAwards)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji"):
			_ = json.NewEncoder(w).Encode(mrAwards)
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/discussions/disc-1/notes/100"):
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			rootBody = payload["body"]
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	group := &Group{Path: "42", Client: gitlab.NewClient(server.URL, "token"), BotUserID: 77}
	controller := NewResponseController(ResponseConfig{
		Enabled: true, MuteEmoji: "mute", RequestEmoji: "nickpit", CommandKeyword: "nickpit",
	}, slog.Default())

	state, err := controller.State(context.Background(), group, "42", 9, "disc-1")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Allows(false) || state.Status.ThreadMuted {
		t.Fatalf("own mute reaction changed policy: %+v", state.Status)
	}

	mu.Lock()
	noteAwards = append(noteAwards, map[string]any{"id": 2, "name": "mute", "user": map[string]any{"id": 88}})
	mu.Unlock()
	state, err = controller.State(context.Background(), group, "42", 9, "disc-1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Allows(true) || !state.Status.ThreadMuted {
		t.Fatalf("human root reaction did not mute: %+v", state.Status)
	}

	state, err = controller.SetCommandMuted(context.Background(), group, "42", 9, "disc-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Status.CommandMuted {
		t.Fatal("command mute not persisted")
	}
	mu.Lock()
	persisted := rootBody
	noteAwards = []map[string]any{}
	mu.Unlock()
	if !reviewmd.ThreadCommandMuted(persisted) || !strings.Contains(persisted, "will not respond") {
		t.Fatalf("persisted body lacks mute state/footer: %q", persisted)
	}

	state, err = controller.SetCommandMuted(context.Background(), group, "42", 9, "disc-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Allows(false) || reviewmd.ThreadCommandMuted(state.Root.Body) || !strings.Contains(state.Root.Body, "will respond") {
		t.Fatalf("resume did not restore automatic response mode: %+v body=%q", state.Status, state.Root.Body)
	}
}

func TestResponseControllerIgnoresMuteReactionOnReply(t *testing.T) {
	t.Parallel()
	rootBody := reviewFindingBody(model.Finding{ID: "finding-1", Title: "Finding", Body: "Details"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/discussions/disc-1"):
			_ = json.NewEncoder(w).Encode(map[string]any{"notes": []map[string]any{{
				"id": 100, "body": rootBody, "author": map[string]any{"id": 77},
			}}})
		case strings.HasSuffix(r.URL.Path, "/award_emoji"):
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	group := &Group{Client: gitlab.NewClient(server.URL, "token"), BotUserID: 77}
	controller := NewResponseController(ResponseConfig{Enabled: true, MuteEmoji: "mute"}, slog.Default())

	ours, err := controller.SyncReactedRoot(context.Background(), group, "42", 9, "disc-1", 101)
	if err != nil {
		t.Fatal(err)
	}
	if ours {
		t.Fatal("reaction on a reply was treated as a root reaction")
	}
}
