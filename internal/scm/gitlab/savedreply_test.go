package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// savedReplyServer is a minimal GitLab GraphQL stand-in for the saved-reply
// vocabulary: it serves one owner with a fixed template inventory and records
// every mutation it is asked to run.
type savedReplyServer struct {
	owner     string
	pages     [][]SavedReply
	mutations []savedReplyMutationCall
	// missingOwner makes the owner selection resolve to null, as GitLab does
	// for a path the token cannot see.
	missingOwner bool
	// queryErrors, when set, is returned as the GraphQL "errors" array.
	queryErrors []string
}

type savedReplyMutationCall struct {
	field     string
	variables map[string]any
}

func (s *savedReplyServer) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/graphql" {
			t.Errorf("graphql path = %q, want /api/graphql", r.URL.Path)
		}
		if got := r.Header.Get("PRIVATE-TOKEN"); got != "token" {
			t.Errorf("token header = %q", got)
		}
		var request struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if len(s.queryErrors) > 0 {
			messages := make([]string, 0, len(s.queryErrors))
			for _, message := range s.queryErrors {
				messages = append(messages, fmt.Sprintf(`{"message": %q}`, message))
			}
			_, _ = fmt.Fprintf(w, `{"errors": [%s]}`, strings.Join(messages, ","))
			return
		}
		if strings.Contains(request.Query, "savedReplies(first: 100") {
			s.writeOwner(t, w, request.Variables)
			return
		}
		for _, field := range []string{
			"savedReplyCreate", "savedReplyUpdate", "savedReplyDestroy",
			"projectSavedReplyCreate", "projectSavedReplyUpdate", "projectSavedReplyDestroy",
			"groupSavedReplyCreate", "groupSavedReplyUpdate", "groupSavedReplyDestroy",
		} {
			if strings.Contains(request.Query, field+"(input:") {
				s.mutations = append(s.mutations, savedReplyMutationCall{field: field, variables: request.Variables})
				_, _ = fmt.Fprint(w, `{"data": {"result": {"errors": []}}}`)
				return
			}
		}
		t.Fatalf("unexpected query: %s", request.Query)
	}
}

func (s *savedReplyServer) writeOwner(t *testing.T, w http.ResponseWriter, variables map[string]any) {
	t.Helper()
	if s.missingOwner {
		_, _ = fmt.Fprint(w, `{"data": {"owner": null}}`)
		return
	}
	page := 0
	if after, ok := variables["after"].(string); ok {
		if _, err := fmt.Sscanf(after, "cursor-%d", &page); err != nil {
			t.Fatalf("unexpected cursor %q", after)
		}
	}
	nodes := []SavedReply{}
	if page < len(s.pages) {
		nodes = s.pages[page]
	}
	hasNext := page+1 < len(s.pages)
	response := map[string]any{"data": map[string]any{"owner": map[string]any{
		"id": s.owner,
		"savedReplies": map[string]any{
			"nodes": nodes,
			"pageInfo": map[string]any{
				"hasNextPage": hasNext,
				"endCursor":   fmt.Sprintf("cursor-%d", page+1),
			},
		},
	}}}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Fatalf("encoding response: %v", err)
	}
}

func (s *savedReplyServer) start(t *testing.T) *Client {
	t.Helper()
	server := httptest.NewServer(s.handler(t))
	t.Cleanup(server.Close)
	return NewClient(server.URL, "token")
}

func desiredTemplates() []SavedReply {
	return []SavedReply{
		{Name: "nickpit: review", Content: "/nickpit review"},
		{Name: "nickpit: abort", Content: "/nickpit abort"},
	}
}

func TestSyncSavedRepliesCreatesUpdatesAndKeeps(t *testing.T) {
	stub := &savedReplyServer{
		owner: "gid://gitlab/Group/7",
		pages: [][]SavedReply{{
			{ID: "gid://gitlab/Groups::SavedReply/1", Name: "nickpit: review", Content: "/nickpit review"},
			{ID: "gid://gitlab/Groups::SavedReply/2", Name: "nickpit: abort", Content: "/old abort"},
		}},
	}
	client := stub.start(t)
	result, err := client.SyncSavedReplies(context.Background(), GroupSavedReplyScope("acme"), SavedReplySyncOptions{
		Desired: desiredTemplates(),
		Prefix:  "nickpit: ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 0 {
		t.Fatalf("created = %v, want none", result.Created)
	}
	if want := []string{"nickpit: review"}; !equalStrings(result.Unchanged, want) {
		t.Fatalf("unchanged = %v, want %v", result.Unchanged, want)
	}
	if want := []string{"nickpit: abort"}; !equalStrings(result.Updated, want) {
		t.Fatalf("updated = %v, want %v", result.Updated, want)
	}
	if len(stub.mutations) != 1 || stub.mutations[0].field != "groupSavedReplyUpdate" {
		t.Fatalf("mutations = %+v, want a single groupSavedReplyUpdate", stub.mutations)
	}
	call := stub.mutations[0]
	if call.variables["id"] != "gid://gitlab/Groups::SavedReply/2" {
		t.Fatalf("update id = %v", call.variables["id"])
	}
	if call.variables["content"] != "/nickpit abort" {
		t.Fatalf("update content = %v", call.variables["content"])
	}
}

func TestSyncSavedRepliesCreatesWithOwnerID(t *testing.T) {
	stub := &savedReplyServer{owner: "gid://gitlab/Project/42", pages: [][]SavedReply{{}}}
	client := stub.start(t)
	result, err := client.SyncSavedReplies(context.Background(), ProjectSavedReplyScope("acme/widget"), SavedReplySyncOptions{
		Desired: desiredTemplates(),
		Prefix:  "nickpit: ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"nickpit: review", "nickpit: abort"}; !equalStrings(result.Created, want) {
		t.Fatalf("created = %v, want %v", result.Created, want)
	}
	if len(stub.mutations) != 2 {
		t.Fatalf("mutations = %+v, want two creates", stub.mutations)
	}
	for _, call := range stub.mutations {
		if call.field != "projectSavedReplyCreate" {
			t.Fatalf("mutation field = %q", call.field)
		}
		if call.variables["ownerId"] != "gid://gitlab/Project/42" {
			t.Fatalf("ownerId = %v, want the project global id", call.variables["ownerId"])
		}
	}
}

// The user scope has no owner argument: its create mutation derives the owner
// from the token, so passing one would be a GraphQL error.
func TestSyncSavedRepliesUserScopeOmitsOwner(t *testing.T) {
	stub := &savedReplyServer{owner: "gid://gitlab/User/9", pages: [][]SavedReply{{}}}
	client := stub.start(t)
	if _, err := client.SyncSavedReplies(context.Background(), UserSavedReplyScope(), SavedReplySyncOptions{
		Desired: desiredTemplates()[:1],
	}); err != nil {
		t.Fatal(err)
	}
	if len(stub.mutations) != 1 || stub.mutations[0].field != "savedReplyCreate" {
		t.Fatalf("mutations = %+v, want a single savedReplyCreate", stub.mutations)
	}
	if _, ok := stub.mutations[0].variables["ownerId"]; ok {
		t.Fatalf("user-scope create passed an owner id: %+v", stub.mutations[0].variables)
	}
}

// Pruning deletes stale templates carrying the prefix and leaves everything
// else — including hand-written templates in the same scope — alone.
func TestSyncSavedRepliesPrunesOnlyPrefixed(t *testing.T) {
	stub := &savedReplyServer{
		owner: "gid://gitlab/Group/7",
		pages: [][]SavedReply{{
			{ID: "1", Name: "nickpit: review", Content: "/nickpit review"},
			{ID: "2", Name: "nickpit: abort", Content: "/nickpit abort"},
			{ID: "3", Name: "nickpit: legacy", Content: "/nickpit legacy"},
			{ID: "4", Name: "team: ship it", Content: "LGTM"},
		}},
	}
	client := stub.start(t)
	result, err := client.SyncSavedReplies(context.Background(), GroupSavedReplyScope("acme"), SavedReplySyncOptions{
		Desired: desiredTemplates(),
		Prefix:  "nickpit: ",
		Prune:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"nickpit: legacy"}; !equalStrings(result.Pruned, want) {
		t.Fatalf("pruned = %v, want %v", result.Pruned, want)
	}
	if len(stub.mutations) != 1 || stub.mutations[0].field != "groupSavedReplyDestroy" {
		t.Fatalf("mutations = %+v, want a single destroy", stub.mutations)
	}
	if stub.mutations[0].variables["id"] != "3" {
		t.Fatalf("destroyed id = %v, want the stale prefixed template", stub.mutations[0].variables["id"])
	}
}

func TestSyncSavedRepliesDryRunWritesNothing(t *testing.T) {
	stub := &savedReplyServer{
		owner: "gid://gitlab/Group/7",
		pages: [][]SavedReply{{{ID: "1", Name: "nickpit: stale", Content: "/nickpit stale"}}},
	}
	client := stub.start(t)
	result, err := client.SyncSavedReplies(context.Background(), GroupSavedReplyScope("acme"), SavedReplySyncOptions{
		Desired: desiredTemplates(),
		Prefix:  "nickpit: ",
		Prune:   true,
		DryRun:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"nickpit: review", "nickpit: abort"}; !equalStrings(result.Created, want) {
		t.Fatalf("created = %v, want %v", result.Created, want)
	}
	if want := []string{"nickpit: stale"}; !equalStrings(result.Pruned, want) {
		t.Fatalf("pruned = %v, want %v", result.Pruned, want)
	}
	if len(stub.mutations) != 0 {
		t.Fatalf("dry run ran mutations: %+v", stub.mutations)
	}
}

func TestSavedRepliesFollowsPagination(t *testing.T) {
	stub := &savedReplyServer{
		owner: "gid://gitlab/Group/7",
		pages: [][]SavedReply{
			{{ID: "1", Name: "a", Content: "a"}},
			{{ID: "2", Name: "b", Content: "b"}},
		},
	}
	client := stub.start(t)
	replies, err := client.SavedReplies(context.Background(), GroupSavedReplyScope("acme"))
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 2 || replies[0].Name != "a" || replies[1].Name != "b" {
		t.Fatalf("replies = %+v, want both pages in order", replies)
	}
}

func TestSavedRepliesMissingOwner(t *testing.T) {
	stub := &savedReplyServer{missingOwner: true}
	client := stub.start(t)
	_, err := client.SavedReplies(context.Background(), GroupSavedReplyScope("acme"))
	if err == nil || !strings.Contains(err.Error(), "not found or not visible") {
		t.Fatalf("error = %v, want a not-visible error", err)
	}
}

// GitLab reports an unlicensed feature or a denied permission as HTTP 200 with
// an "errors" array; that must surface as a failure, not an empty inventory.
func TestSavedRepliesGraphQLErrors(t *testing.T) {
	stub := &savedReplyServer{queryErrors: []string{"The resource that you are attempting to access does not exist"}}
	client := stub.start(t)
	_, err := client.SavedReplies(context.Background(), GroupSavedReplyScope("acme"))
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("error = %v, want the graphql message", err)
	}
	var graphQLErr *GraphQLError
	if !errors.As(err, &graphQLErr) {
		t.Fatalf("error = %v, want a GraphQLError", err)
	}
}

func TestSavedReplyScopeValidation(t *testing.T) {
	client := NewClient("https://gitlab.example.com", "token")
	cases := []struct {
		name  string
		scope SavedReplyScope
		want  string
	}{
		{"group without path", SavedReplyScope{Kind: SavedReplyScopeGroup}, "need a group path"},
		{"project without path", SavedReplyScope{Kind: SavedReplyScopeProject}, "need a project path"},
		{"user with path", SavedReplyScope{Kind: SavedReplyScopeUser, Path: "acme"}, "take no path"},
		{"unknown kind", SavedReplyScope{Kind: "namespace"}, "unknown saved reply scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.SavedReplies(context.Background(), tc.scope)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidateDesiredSavedReplies(t *testing.T) {
	cases := []struct {
		name    string
		desired []SavedReply
		want    string
	}{
		{"empty name", []SavedReply{{Content: "x"}}, "name must not be empty"},
		{"empty content", []SavedReply{{Name: "a"}}, "empty content"},
		{"long name", []SavedReply{{Name: strings.Repeat("a", 256), Content: "x"}}, "exceeds 255"},
		{"long content", []SavedReply{{Name: "a", Content: strings.Repeat("x", 10001)}}, "exceeds 10000"},
		{"duplicate", []SavedReply{{Name: "a", Content: "x"}, {Name: "a", Content: "y"}}, "duplicate saved reply name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDesiredSavedReplies(tc.desired)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestGraphQLURL(t *testing.T) {
	cases := map[string]string{
		"https://gitlab.example.com/api/v4": "https://gitlab.example.com/api/graphql",
		"http://localhost:8080/api/v4":      "http://localhost:8080/api/graphql",
		"https://gitlab.example.com":        "https://gitlab.example.com/api/graphql",
	}
	for in, want := range cases {
		if got := graphQLURL(in); got != want {
			t.Errorf("graphQLURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
