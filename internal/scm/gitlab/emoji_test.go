package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

// 401/403/429 mean the award never happened for a reason worth logging (lost
// token access, rate limit) — unlike a double-award, they must surface, or a
// replace could revoke successfully and drop the new reaction in silence.
func TestAwardMREmojiSurfacesAuthAndRateLimit(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		client := NewClient(server.URL, "token")
		if err := client.AwardMREmoji(context.Background(), 42, 7, "eyes"); err == nil {
			t.Fatalf("status %d: expected an error", status)
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

// emojiServer serves an award list and records the awards posted and deleted.
type emojiServer struct {
	awards  []AwardEmoji
	posted  []string
	deleted []string
	// listStatus, when non-zero, makes the list request fail with that status.
	listStatus int
}

func (e *emojiServer) start(t *testing.T) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body map[string]string
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &body)
			e.posted = append(e.posted, body["name"])
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case http.MethodDelete:
			_, id, _ := strings.Cut(r.URL.Path, "/award_emoji/")
			e.deleted = append(e.deleted, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			if e.listStatus != 0 {
				w.WriteHeader(e.listStatus)
				return
			}
			_ = json.NewEncoder(w).Encode(e.awards)
		}
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL, "token")
}

func award(id int, name string, userID int) AwardEmoji {
	emoji := AwardEmoji{ID: id, Name: name}
	emoji.User.ID = userID
	return emoji
}

func TestReplaceMREmojiRevokesOwnAwardsOnly(t *testing.T) {
	fake := &emojiServer{awards: []AwardEmoji{
		award(1, "eyes", 9),     // ours
		award(2, "eyes", 5),     // a human's, must survive
		award(3, "thumbsup", 9), // not a name we replace
		award(4, "x", 9),        // a previous outcome, also ours
	}}
	client := fake.start(t)

	err := client.ReplaceMREmoji(context.Background(), 42, 7, 9, "white_check_mark", "eyes", "x")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(fake.deleted) != "[1 4]" {
		t.Fatalf("deleted = %v, want only our eyes and x awards", fake.deleted)
	}
	if fmt.Sprint(fake.posted) != "[white_check_mark]" {
		t.Fatalf("posted = %v", fake.posted)
	}
}

// Without a resolved bot user id the name would be the only filter left, and an
// administrator/owner token CAN delete another user's award. Refuse the whole
// replacement: adding the outcome without revoking the marker would leave
// contradictory reactions behind.
func TestReplaceMREmojiWithoutUserIDRefusesRevoke(t *testing.T) {
	fake := &emojiServer{awards: []AwardEmoji{award(1, "eyes", 5), award(2, "rocket", 5)}}
	client := fake.start(t)

	err := client.ReplaceMREmoji(context.Background(), 42, 7, 0, "white_check_mark", "eyes")
	if err == nil {
		t.Fatal("expected the refused revoke to be reported")
	}
	if len(fake.deleted) != 0 {
		t.Fatalf("deleted = %v, want no revoke without a resolved user id", fake.deleted)
	}
	if len(fake.posted) != 0 {
		t.Fatalf("posted = %v, want no add after the unsafe revoke was refused", fake.posted)
	}
}

// Nothing to revoke means the list request is not worth making.
func TestReplaceMREmojiSkipsListWhenOnlyAwarding(t *testing.T) {
	fake := &emojiServer{listStatus: http.StatusInternalServerError}
	client := fake.start(t)

	if err := client.ReplaceMREmoji(context.Background(), 42, 7, 9, "eyes", "", ""); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(fake.posted) != "[eyes]" {
		t.Fatalf("posted = %v", fake.posted)
	}
}

// A failed revoke must not swallow the award: the new reaction is the half that
// tells the user what happened.
func TestReplaceMREmojiAwardsDespiteListFailure(t *testing.T) {
	fake := &emojiServer{listStatus: http.StatusInternalServerError}
	client := fake.start(t)

	err := client.ReplaceMREmoji(context.Background(), 42, 7, 9, "white_check_mark", "eyes")
	if err == nil {
		t.Fatal("expected the list failure to be reported")
	}
	if fmt.Sprint(fake.posted) != "[white_check_mark]" {
		t.Fatalf("posted = %v, want the outcome awarded anyway", fake.posted)
	}
}

func TestReplaceNoteEmojiPath(t *testing.T) {
	var listPath, deletePath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			deletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			listPath = r.URL.Path
			_ = json.NewEncoder(w).Encode([]AwardEmoji{award(3, "eyes", 9)})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	if err := client.ReplaceNoteEmoji(context.Background(), 42, 7, 301, 9, "x", "eyes"); err != nil {
		t.Fatal(err)
	}
	if listPath != "/api/v4/projects/42/merge_requests/7/notes/301/award_emoji" {
		t.Fatalf("list path = %q", listPath)
	}
	if deletePath != "/api/v4/projects/42/merge_requests/7/notes/301/award_emoji/3" {
		t.Fatalf("delete path = %q", deletePath)
	}
}

// A reaction someone else already removed (404) or one that is not ours (403)
// leaves nothing to do — neither is an error worth reporting.
func TestReplaceMREmojiToleratesGoneAndForbidden(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodDelete:
				w.WriteHeader(status)
			case http.MethodPost:
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{}`))
			default:
				_ = json.NewEncoder(w).Encode([]AwardEmoji{award(1, "eyes", 9)})
			}
		}))
		client := NewClient(server.URL, "token")
		if err := client.ReplaceMREmoji(context.Background(), 42, 7, 9, "x", "eyes"); err != nil {
			t.Fatalf("status %d: expected nil, got %v", status, err)
		}
		server.Close()
	}
}
