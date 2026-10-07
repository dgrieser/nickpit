package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

const (
	issueReactions   = "/api/v1/repos/owner/repo/issues/7/reactions"
	commentReactions = "/api/v1/repos/owner/repo/issues/comments/301/reactions"
)

// reactionServer serves a reaction list on one path and records, in order,
// the listings read and the reactions added and removed there.
type reactionServer struct {
	t         *testing.T
	path      string
	reactions []reactionResponse
	// A non-zero status makes that kind of request fail with it.
	listStatus   int
	postStatus   int
	deleteStatus int

	mu      sync.Mutex
	actions []string
}

func (s *reactionServer) start() *Client {
	s.t.Helper()
	server := httptest.NewServer(http.HandlerFunc(s.handle))
	s.t.Cleanup(server.Close)
	return NewClient(server.URL, "token")
}

func (s *reactionServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.path {
		s.t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	data, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(data, &body)
	status := 0
	switch r.Method {
	case http.MethodGet:
		s.record("list")
		status = s.listStatus
	case http.MethodPost:
		s.record("post:" + body.Content)
		status = s.postStatus
	case http.MethodDelete:
		s.record("delete:" + body.Content)
		status = s.deleteStatus
	default:
		s.t.Errorf("unexpected method %s", r.Method)
	}
	if status != 0 {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"refused"}`))
		return
	}
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(s.reactions)
	case http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"content":%q,"user":{"id":9,"login":"nickpit-bot"}}`, body.Content)
	}
	// Forgejo answers a removal with 200 and no body.
}

func (s *reactionServer) record(action string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = append(s.actions, action)
}

func (s *reactionServer) recorded() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprint(s.actions)
}

func reaction(content string, userID int) reactionResponse {
	item := reactionResponse{Content: content}
	item.User.ID = userID
	return item
}

func TestListReactions(t *testing.T) {
	listing := fixture(t, "reactions.json")
	tests := []struct {
		name string
		path string
		list func(*Client) ([]Reaction, error)
	}{
		{
			name: "pull request",
			path: issueReactions,
			list: func(c *Client) ([]Reaction, error) {
				return c.IssueReactions(context.Background(), "owner/repo", 7)
			},
		},
		{
			name: "comment",
			path: commentReactions,
			list: func(c *Client) ([]Reaction, error) {
				return c.CommentReactions(context.Background(), "owner/repo", 301)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				_, _ = w.Write(listing)
			}))
			defer server.Close()

			reactions, err := tt.list(NewClient(server.URL, "token"))
			if err != nil {
				t.Fatal(err)
			}
			if gotMethod != http.MethodGet || gotPath != tt.path {
				t.Fatalf("request = %s %s, want GET %s", gotMethod, gotPath, tt.path)
			}
			want := []Reaction{
				{Content: "eyes", UserID: 9, UserLogin: "nickpit-bot"},
				{Content: "+1", UserID: 5, UserLogin: "reviewer"},
				{Content: "heart", UserID: -1, UserLogin: "Ghost"},
			}
			if fmt.Sprint(reactions) != fmt.Sprint(want) {
				t.Fatalf("reactions = %+v, want %+v", reactions, want)
			}
		})
	}
}

// Forgejo encodes a target without reactions as null, not as an empty list.
func TestListReactionsDecodesNullAsNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("null\n"))
	}))
	defer server.Close()

	reactions, err := NewClient(server.URL, "token").IssueReactions(context.Background(), "owner/repo", 7)
	if err != nil || len(reactions) != 0 {
		t.Fatalf("reactions = %+v, err = %v, want none", reactions, err)
	}
}

func TestListReactionsSurfacesAPIError(t *testing.T) {
	fake := &reactionServer{t: t, path: commentReactions, listStatus: http.StatusNotFound}
	reactions, err := fake.start().CommentReactions(context.Background(), "owner/repo", 301)
	if reactions != nil || !IsNotFound(err) {
		t.Fatalf("reactions = %+v, err = %v, want a 404", reactions, err)
	}
}

func TestReactionRequests(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		call   func(*Client) error
		method string
		path   string
	}{
		{
			name:   "add to pull request",
			call:   func(c *Client) error { return c.AddIssueReaction(ctx, "owner/repo", 7, "eyes") },
			method: http.MethodPost,
			path:   issueReactions,
		},
		{
			name:   "remove from pull request",
			call:   func(c *Client) error { return c.RemoveIssueReaction(ctx, "owner/repo", 7, "eyes") },
			method: http.MethodDelete,
			path:   issueReactions,
		},
		{
			name:   "add to comment",
			call:   func(c *Client) error { return c.AddCommentReaction(ctx, "owner/repo", 301, "eyes") },
			method: http.MethodPost,
			path:   commentReactions,
		},
		{
			name:   "remove from comment",
			call:   func(c *Client) error { return c.RemoveCommentReaction(ctx, "owner/repo", 301, "eyes") },
			method: http.MethodDelete,
			path:   commentReactions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotContentType, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotContentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
				data, _ := io.ReadAll(r.Body)
				gotBody = string(data)
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"content":"eyes","user":{"id":9,"login":"nickpit-bot"}}`))
				}
			}))
			defer server.Close()

			if err := tt.call(NewClient(server.URL, "token")); err != nil {
				t.Fatal(err)
			}
			if gotMethod != tt.method || gotPath != tt.path {
				t.Fatalf("request = %s %s, want %s %s", gotMethod, gotPath, tt.method, tt.path)
			}
			// The removal names its reaction in a JSON body too, which Forgejo
			// only parses as such under the JSON content type.
			if gotBody != `{"content":"eyes"}` || gotContentType != "application/json" {
				t.Fatalf("body = %s (%s)", gotBody, gotContentType)
			}
		})
	}
}

func TestAddReactionOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus int // 0: success
	}{
		{name: "created", status: http.StatusCreated, body: `{"content":"eyes"}`},
		// The same user's repeated reaction is answered with the existing one.
		{name: "already there", status: http.StatusOK, body: `{"content":"eyes"}`},
		{name: "not an allowed reaction", status: http.StatusForbidden, body: `{"message":"'eyes' is not an allowed reaction"}`, wantStatus: http.StatusForbidden},
		{name: "target gone", status: http.StatusNotFound, body: `{"message":"not found"}`, wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			err := NewClient(server.URL, "token").AddCommentReaction(context.Background(), "owner/repo", 301, "eyes")
			if tt.wantStatus == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want an *APIError", err)
			}
			if apiErr.Status != tt.wantStatus || apiErr.Method != http.MethodPost {
				t.Fatalf("api error = %s status %d, want POST status %d", apiErr.Method, apiErr.Status, tt.wantStatus)
			}
		})
	}
}

// A removal without a content would match every reaction the user left on the
// target, so nothing is sent at all.
func TestReactionsRefuseEmptyContent(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(*Client) error{
		"add to pull request":      func(c *Client) error { return c.AddIssueReaction(ctx, "owner/repo", 7, "") },
		"remove from pull request": func(c *Client) error { return c.RemoveIssueReaction(ctx, "owner/repo", 7, "") },
		"add to comment":           func(c *Client) error { return c.AddCommentReaction(ctx, "owner/repo", 301, "") },
		"remove from comment":      func(c *Client) error { return c.RemoveCommentReaction(ctx, "owner/repo", 301, "") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}))
			defer server.Close()

			if err := call(NewClient(server.URL, "token")); err == nil {
				t.Fatal("expected the empty reaction to be refused")
			}
		})
	}
}

func TestReplaceOwnReaction(t *testing.T) {
	const bot, human = 9, 5
	ctx := context.Background()
	targets := []struct {
		name    string
		path    string
		replace func(c *Client, userID int, add string, keep ...string) error
	}{
		{
			name: "pull request",
			path: issueReactions,
			replace: func(c *Client, userID int, add string, keep ...string) error {
				return c.ReplaceOwnIssueReaction(ctx, "owner/repo", 7, userID, add, keep...)
			},
		},
		{
			name: "comment",
			path: commentReactions,
			replace: func(c *Client, userID int, add string, keep ...string) error {
				return c.ReplaceOwnCommentReaction(ctx, "owner/repo", 301, userID, add, keep...)
			},
		},
	}
	tests := []struct {
		name         string
		reactions    []reactionResponse
		userID       int
		add          string
		keep         []string
		listStatus   int
		postStatus   int
		deleteStatus int
		wantActions  string
		// wantStatus is the API status the returned error carries; 0 with
		// wantErr set means an error that is no API answer.
		wantErr    bool
		wantStatus int
	}{
		{
			// The add is confirmed before anything is removed.
			name: "adds the new one and removes the bot's others",
			reactions: []reactionResponse{
				reaction("eyes", bot),
				reaction("eyes", human),
				reaction("rocket", bot),
			},
			userID:      bot,
			add:         "hooray",
			wantActions: "[list post:hooray delete:eyes delete:rocket]",
		},
		{
			name: "honours keep",
			reactions: []reactionResponse{
				reaction("eyes", bot),
				reaction("heart", bot),
				reaction("+1", bot),
			},
			userID:      bot,
			add:         "hooray",
			keep:        []string{"heart", "+1"},
			wantActions: "[list post:hooray delete:eyes]",
		},
		{
			name: "leaves other users' reactions alone",
			reactions: []reactionResponse{
				reaction("eyes", human),
				reaction("rocket", human),
				reaction("confused", -1), // a deleted account's
				reaction("laugh", 0),     // migrated, no local user
			},
			userID:      bot,
			add:         "hooray",
			wantActions: "[list post:hooray]",
		},
		{
			name:        "does not remove the reaction it just added",
			reactions:   []reactionResponse{reaction("hooray", bot), reaction("eyes", bot)},
			userID:      bot,
			add:         "hooray",
			wantActions: "[list post:hooray delete:eyes]",
		},
		{
			name: "empty add only removes",
			reactions: []reactionResponse{
				reaction("eyes", bot),
				reaction("heart", bot),
				reaction("eyes", human),
			},
			userID:      bot,
			keep:        []string{"heart"},
			wantActions: "[list delete:eyes]",
		},
		{
			// A failed add leaves the old status marker in place.
			name:        "removes nothing when the add is refused",
			reactions:   []reactionResponse{reaction("eyes", bot)},
			userID:      bot,
			add:         "tada",
			postStatus:  http.StatusForbidden,
			wantActions: "[list post:tada]",
			wantErr:     true,
			wantStatus:  http.StatusForbidden,
		},
		{
			// The new reaction is the informative half, so it is still added.
			name:        "adds despite a failed list",
			userID:      bot,
			add:         "hooray",
			listStatus:  http.StatusInternalServerError,
			wantActions: "[list post:hooray]",
			wantErr:     true,
			wantStatus:  http.StatusInternalServerError,
		},
		{
			name:         "a target gone while removing is settled",
			reactions:    []reactionResponse{reaction("eyes", bot)},
			userID:       bot,
			add:          "hooray",
			deleteStatus: http.StatusNotFound,
			wantActions:  "[list post:hooray delete:eyes]",
		},
		{
			name:         "a refused removal surfaces",
			reactions:    []reactionResponse{reaction("eyes", bot), reaction("rocket", bot)},
			userID:       bot,
			add:          "hooray",
			deleteStatus: http.StatusForbidden,
			wantActions:  "[list post:hooray delete:eyes delete:rocket]",
			wantErr:      true,
			wantStatus:   http.StatusForbidden,
		},
		{
			// Without the bot's id nothing listed can be told to be its own.
			name:        "refuses an unresolved user id",
			reactions:   []reactionResponse{reaction("laugh", 0)},
			add:         "hooray",
			wantActions: "[]",
			wantErr:     true,
		},
	}
	for _, target := range targets {
		for _, tt := range tests {
			t.Run(target.name+"/"+tt.name, func(t *testing.T) {
				fake := &reactionServer{
					t:            t,
					path:         target.path,
					reactions:    tt.reactions,
					listStatus:   tt.listStatus,
					postStatus:   tt.postStatus,
					deleteStatus: tt.deleteStatus,
				}
				err := target.replace(fake.start(), tt.userID, tt.add, tt.keep...)
				if got := fake.recorded(); got != tt.wantActions {
					t.Fatalf("actions = %s, want %s", got, tt.wantActions)
				}
				if !tt.wantErr {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				if err == nil {
					t.Fatal("expected an error")
				}
				var apiErr *APIError
				if errors.As(err, &apiErr) != (tt.wantStatus != 0) || (apiErr != nil && apiErr.Status != tt.wantStatus) {
					t.Fatalf("err = %v, want API status %d", err, tt.wantStatus)
				}
			})
		}
	}
}

func TestAllowedReactions(t *testing.T) {
	settings := fixture(t, "settings_ui.json")
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write(settings)
	}))
	defer server.Close()

	allowed, err := NewClient(server.URL, "token").AllowedReactions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/settings/ui" {
		t.Fatalf("path = %q", gotPath)
	}
	if fmt.Sprint(allowed) != "[+1 -1 laugh hooray confused heart rocket eyes]" {
		t.Fatalf("allowed = %v", allowed)
	}
}
