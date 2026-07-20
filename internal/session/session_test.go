package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestStoreSaveLoadRoundTrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sess := New()
	sess.ReviewID = "rev-1"
	sess.PinnedFindingID = "f1"
	sess.Source = Source{Mode: "gitlab", Repo: "grp/proj", Identifier: 42, BaseURL: "https://gl"}
	sess.Result = &model.ReviewResult{ReviewID: "rev-1", OverallCorrectness: "patch is incorrect"}
	sess.Append(UserMessage("why is this a bug?"))
	sess.Append(FromLLM(llm.Message{Role: "assistant", Content: "because X", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "search", Arguments: "{}"}}}))

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := store.Load(sess.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ReviewID != "rev-1" || loaded.PinnedFindingID != "f1" || loaded.Source.Identifier != 42 {
		t.Fatalf("metadata not persisted: %+v", loaded)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded.Messages))
	}
	conv := loaded.Conversation()
	if conv[0].Role != "user" || conv[0].Content != "why is this a bug?" {
		t.Fatalf("user message not round-tripped: %+v", conv[0])
	}
	if len(conv[1].ToolCalls) != 1 || conv[1].ToolCalls[0].Name != "search" {
		t.Fatalf("tool call not round-tripped: %+v", conv[1])
	}
	if loaded.UpdatedAt.Before(loaded.CreatedAt) {
		t.Fatalf("UpdatedAt should be >= CreatedAt")
	}
}

func TestStoreListLatest(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	a := New()
	a.Source.Repo = "a/a"
	if err := store.Save(a); err != nil {
		t.Fatalf("save a: %v", err)
	}
	b := New()
	b.Source.Repo = "b/b"
	if err := store.Save(b); err != nil {
		t.Fatalf("save b: %v", err)
	}
	// Re-save a so it becomes the most recently updated.
	if err := store.Save(a); err != nil {
		t.Fatalf("re-save a: %v", err)
	}
	infos, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(infos))
	}
	latest, err := store.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if latest.ID != a.ID {
		t.Fatalf("latest should be the re-saved session %q, got %q", a.ID, latest.ID)
	}
}

func TestStoreListEmptyDir(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	infos, err := store.List()
	if err != nil {
		t.Fatalf("List on empty: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected no sessions, got %d", len(infos))
	}
	latest, err := store.Latest()
	if err != nil || latest != nil {
		t.Fatalf("Latest on empty should be (nil,nil), got (%v,%v)", latest, err)
	}
}

func TestStorePrunesOldest(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	// Create one more session than the cap; backdate each file so modification
	// times are strictly ordered and the oldest is unambiguous.
	var first string
	for i := 0; i <= maxStoredSessions; i++ {
		sess := New()
		if i == 0 {
			first = sess.ID
		}
		if err := store.Save(sess); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		path, _ := store.Path(sess.ID)
		stamp := time.Now().Add(time.Duration(i-maxStoredSessions-1) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	// One more save triggers the prune over the backdated files.
	if err := store.Save(New()); err != nil {
		t.Fatalf("final save: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	if count != maxStoredSessions {
		t.Fatalf("stored sessions = %d, want %d", count, maxStoredSessions)
	}
	if _, err := store.Load(first); err == nil {
		t.Fatalf("oldest session %s should have been pruned", first)
	}
}

func TestListIgnoresForeignJSONFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	// A bare "{}" (valid JSON, no session shape) in a user-chosen --session-dir
	// must not list as a session — previously it produced an empty-id entry and
	// Latest failed with `session: empty id` instead of "no saved sessions".
	if err := os.WriteFile(filepath.Join(dir, "notes.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write foreign: %v", err)
	}
	infos, err := store.List()
	if err != nil || len(infos) != 0 {
		t.Fatalf("List = %v, %v; want empty", infos, err)
	}
	latest, err := store.Latest()
	if err != nil || latest != nil {
		t.Fatalf("Latest = %v, %v; want (nil,nil)", latest, err)
	}
	// Real sessions still list alongside the foreign file.
	sess := New()
	if err := store.Save(sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	infos, err = store.List()
	if err != nil || len(infos) != 1 || infos[0].ID != sess.ID {
		t.Fatalf("List after save = %v, %v", infos, err)
	}
}

func TestPruneIgnoresForeignJSONFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	// Unrelated JSON files in the directory (e.g. --session-dir pointed at a
	// config directory) must never be deleted by pruning.
	foreign := filepath.Join(dir, "important-config.json")
	if err := os.WriteFile(foreign, []byte(`{"keep":"me"}`), 0o600); err != nil {
		t.Fatalf("write foreign: %v", err)
	}
	stamp := time.Now().Add(-100 * time.Hour) // oldest file in the dir by far
	if err := os.Chtimes(foreign, stamp, stamp); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	for i := 0; i <= maxStoredSessions; i++ {
		if err := store.Save(New()); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign JSON file was deleted by pruning: %v", err)
	}
}

func TestValidateID(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	for _, bad := range []string{"", "../escape", "a/b", `a\b`, ".", ".."} {
		if _, err := store.Path(bad); err == nil {
			t.Fatalf("expected error for id %q", bad)
		}
	}
	if _, err := store.Path("valid-id_123"); err != nil {
		t.Fatalf("valid id rejected: %v", err)
	}
}
