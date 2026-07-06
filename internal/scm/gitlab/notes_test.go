package gitlab

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// recordingServer captures the last request's path and JSON body.
func recordingServer(t *testing.T, status int) (*httptest.Server, *string, *map[string]string) {
	t.Helper()
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	t.Cleanup(server.Close)
	return server, &gotPath, &gotBody
}

func TestAwardNoteEmoji(t *testing.T) {
	server, gotPath, gotBody := recordingServer(t, http.StatusCreated)
	client := NewClient(server.URL, "token")
	if err := client.AwardNoteEmoji(context.Background(), 42, 7, 314, "white_check_mark"); err != nil {
		t.Fatal(err)
	}
	if *gotPath != "/api/v4/projects/42/merge_requests/7/notes/314/award_emoji" {
		t.Fatalf("path = %q", *gotPath)
	}
	if (*gotBody)["name"] != "white_check_mark" {
		t.Fatalf("body = %#v", *gotBody)
	}
}

func TestAwardNoteEmojiToleratesAlreadyAwarded(t *testing.T) {
	server, _, _ := recordingServer(t, http.StatusNotFound)
	client := NewClient(server.URL, "token")
	if err := client.AwardNoteEmoji(context.Background(), 42, 7, 314, "white_check_mark"); err != nil {
		t.Fatalf("expected nil on 4xx, got %v", err)
	}
}

func TestAwardNoteEmojiServerError(t *testing.T) {
	server, _, _ := recordingServer(t, http.StatusInternalServerError)
	client := NewClient(server.URL, "token")
	if err := client.AwardNoteEmoji(context.Background(), 42, 7, 314, "white_check_mark"); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestCreateMRNote(t *testing.T) {
	server, gotPath, gotBody := recordingServer(t, http.StatusCreated)
	client := NewClient(server.URL, "token")
	if err := client.CreateMRNote(context.Background(), 42, 7, "review aborted"); err != nil {
		t.Fatal(err)
	}
	if *gotPath != "/api/v4/projects/42/merge_requests/7/notes" {
		t.Fatalf("path = %q", *gotPath)
	}
	if (*gotBody)["body"] != "review aborted" {
		t.Fatalf("body = %#v", *gotBody)
	}
}

func TestCreateMRNoteError(t *testing.T) {
	server, _, _ := recordingServer(t, http.StatusUnauthorized)
	client := NewClient(server.URL, "token")
	if err := client.CreateMRNote(context.Background(), 42, 7, "x"); err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestReplyToMRDiscussion(t *testing.T) {
	server, gotPath, gotBody := recordingServer(t, http.StatusCreated)
	client := NewClient(server.URL, "token")
	if err := client.ReplyToMRDiscussion(context.Background(), 42, 7, "abc123", "hello"); err != nil {
		t.Fatal(err)
	}
	if *gotPath != "/api/v4/projects/42/merge_requests/7/discussions/abc123/notes" {
		t.Fatalf("path = %q", *gotPath)
	}
	if (*gotBody)["body"] != "hello" {
		t.Fatalf("body = %#v", *gotBody)
	}
}

func TestReplyToMRDiscussionError(t *testing.T) {
	server, _, _ := recordingServer(t, http.StatusNotFound)
	client := NewClient(server.URL, "token")
	if err := client.ReplyToMRDiscussion(context.Background(), 42, 7, "abc123", "hello"); err == nil {
		t.Fatal("expected error on 404")
	}
}
