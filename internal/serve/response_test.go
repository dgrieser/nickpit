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

// A publish must reconcile only the roots it just added. Re-reading reactions
// for every root the MR ever accumulated makes post-publish work grow with all
// historical findings; SyncMR stays the full scan for MR-wide state changes.
func TestSyncNewRootsSkipsAlreadyStampedRoots(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	stamped := reviewmd.UpsertResponseFooter(
		reviewFindingBody(model.Finding{ID: "old", Title: "Old", Body: "Details"}),
		reviewmd.ResponseStatus{Enabled: true, MuteEmoji: "mute", CommandKeyword: "nickpit"})
	fresh := reviewFindingBody(model.Finding{ID: "new", Title: "New", Body: "Details"})
	noteEmojiReads := map[string]int{}
	updated := map[int]string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/discussions"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "disc-old", "notes": []map[string]any{{
					"id": 100, "body": stamped, "author": map[string]any{"id": 77},
				}}},
				{"id": "disc-new", "notes": []map[string]any{{
					"id": 200, "body": fresh, "author": map[string]any{"id": 77},
				}}},
				// A human thread is never a nickpit review root.
				{"id": "disc-human", "notes": []map[string]any{{
					"id": 300, "body": "unrelated", "author": map[string]any{"id": 88},
				}}},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji"):
			if i := strings.Index(r.URL.Path, "/notes/"); i >= 0 {
				noteEmojiReads[strings.TrimSuffix(r.URL.Path[i+len("/notes/"):], "/award_emoji")]++
			}
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/discussions/"):
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if strings.HasSuffix(r.URL.Path, "/notes/200") {
				updated[200] = payload["body"]
			} else {
				updated[100] = payload["body"]
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	group := &Group{Client: gitlab.NewClient(server.URL, "token"), BotUserID: 77}
	controller := NewResponseController(ResponseConfig{
		Enabled: true, MuteEmoji: "mute", CommandKeyword: "nickpit",
	}, slog.Default())

	if err := controller.SyncNewRoots(context.Background(), group, "42", 9); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := updated[100]; ok {
		t.Fatal("already-stamped root was rewritten after publish")
	}
	if !reviewmd.HasResponseFooter(updated[200]) {
		t.Fatalf("newly published root was not stamped: %q", updated[200])
	}
	if noteEmojiReads["100"] != 0 {
		t.Fatalf("reactions were read for an already-stamped root: %d", noteEmojiReads["100"])
	}
	if noteEmojiReads["200"] != 1 {
		t.Fatalf("new root reaction reads = %d, want 1", noteEmojiReads["200"])
	}
}

// With nothing new to stamp, a publish must not read MR reactions at all.
func TestSyncNewRootsSkipsMREmojisWhenNothingMissing(t *testing.T) {
	t.Parallel()
	stamped := reviewmd.UpsertResponseFooter(
		reviewFindingBody(model.Finding{ID: "old", Title: "Old", Body: "Details"}),
		reviewmd.ResponseStatus{Enabled: true, MuteEmoji: "mute", CommandKeyword: "nickpit"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/discussions") {
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "disc-old", "notes": []map[string]any{{
				"id": 100, "body": stamped, "author": map[string]any{"id": 77},
			}}}})
			return
		}
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	group := &Group{Client: gitlab.NewClient(server.URL, "token"), BotUserID: 77}
	controller := NewResponseController(ResponseConfig{
		Enabled: true, MuteEmoji: "mute", CommandKeyword: "nickpit",
	}, slog.Default())
	if err := controller.SyncNewRoots(context.Background(), group, "42", 9); err != nil {
		t.Fatalf("nothing-to-do sync made unexpected calls: %v", err)
	}
}

// A footer stamped under earlier settings advertises controls the daemon no
// longer honors — a renamed mute emoji is never decoded by Decide, so no other
// event ever reconciles that root. The stamped policy fingerprint makes the
// next publish pick it up instead of skipping it forever.
func TestSyncNewRootsRestampsAfterPolicyChange(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	body := reviewmd.UpsertResponseFooter(
		reviewFindingBody(model.Finding{ID: "old", Title: "Old", Body: "Details"}),
		reviewmd.ResponseStatus{Enabled: true, MuteEmoji: "mute", CommandKeyword: "nickpit"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/discussions"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "disc-old", "notes": []map[string]any{{
				"id": 100, "body": body, "author": map[string]any{"id": 77},
			}}}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/award_emoji"):
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPut:
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			body = payload["body"]
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	group := &Group{Client: gitlab.NewClient(server.URL, "token"), BotUserID: 77}
	renamed := ResponseConfig{Enabled: true, MuteEmoji: "no_bell", CommandKeyword: "nickpit"}
	if err := NewResponseController(renamed, slog.Default()).SyncNewRoots(context.Background(), group, "42", 9); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Contains(body, ":mute:") || !strings.Contains(body, ":no_bell:") {
		t.Fatalf("footer still advertises the former mute emoji: %q", body)
	}
	if !reviewmd.FooterMatchesPolicy(body, reviewmd.ResponseStatus{
		Enabled: true, MuteEmoji: "no_bell", CommandKeyword: "nickpit",
	}) {
		t.Fatalf("re-stamped footer does not carry the current policy: %q", body)
	}
}
