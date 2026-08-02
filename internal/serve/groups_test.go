package serve

import (
	"context"
	"errors"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

func newTestGroupSet(t *testing.T, cfgs []config.ServeGroup) *GroupSet {
	t.Helper()
	set, warnings := NewGroupSet(context.Background(), cfgs, "https://gitlab.example.com", nil)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	return set
}

// newTestGroupSetWithURL builds a one-group set whose client points at a test
// server.
func newTestGroupSetWithURL(t *testing.T, baseURL string) *GroupSet {
	t.Helper()
	set, warnings := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t", WebhookSecret: "s"},
	}, baseURL, nil)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	return set
}

func TestGroupSetMatch(t *testing.T) {
	set := newTestGroupSet(t, []config.ServeGroup{
		{Path: "platform", Token: "t1", WebhookSecret: "s1"},
		{Path: "platform/legacy", Token: "t2", WebhookSecret: "s2"},
	})
	cases := []struct {
		project string
		want    string
	}{
		{"platform/api", "platform"},
		{"platform/legacy/tool", "platform/legacy"},
		{"platform/legacy", "platform/legacy"},
		{"platform/legacy-x", "platform"}, // '/' boundary: not the legacy group
		{"platformx/api", ""},             // prefix must end at boundary
		{"other/repo", ""},
	}
	for _, tc := range cases {
		group := set.Match(tc.project)
		got := ""
		if group != nil {
			got = group.Path
		}
		if got != tc.want {
			t.Fatalf("Match(%q) = %q, want %q", tc.project, got, tc.want)
		}
	}
}

func TestGroupSetMatchExactGroupPath(t *testing.T) {
	set := newTestGroupSet(t, []config.ServeGroup{{Path: "platform", Token: "t", WebhookSecret: "s"}})
	if set.Match("platform") == nil {
		t.Fatal("group path itself must match")
	}
}

func TestGroupCheckSecret(t *testing.T) {
	set := newTestGroupSet(t, []config.ServeGroup{{Path: "platform", Token: "t", WebhookSecret: "hook-secret"}})
	group := set.Match("platform/api")
	if !group.CheckSecret("hook-secret") {
		t.Fatal("correct secret rejected")
	}
	if group.CheckSecret("wrong") || group.CheckSecret("") {
		t.Fatal("wrong secret accepted")
	}
}

// A group whose signing token cannot be parsed and that has no webhook secret
// holds no usable credential: it must fail closed. In particular the empty
// stored secret must not compare equal to an absent X-Gitlab-Token header.
func TestNewGroupSetUnparseableSigningTokenFailsClosed(t *testing.T) {
	set, warnings := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t", SigningToken: "whsec_not!!base64"},
	}, "https://gitlab.example.com", nil)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want the unparseable-token warning", warnings)
	}
	group := set.Match("platform/api")
	if group == nil {
		t.Fatal("group must still exist")
	}
	if group.UsesSigning() {
		t.Fatal("unparseable signing token must not enable signing")
	}
	if group.CheckSecret("") {
		t.Fatal("request without X-Gitlab-Token must be rejected (fail closed)")
	}
	if group.CheckSecret("anything") {
		t.Fatal("no token may authenticate against a group without a credential")
	}
}

// An empty stored secret rejects every token, including the empty one — the
// fail-closed guarantee CheckSecret provides for groups without a usable
// credential.
func TestGroupCheckSecretEmptyStoredSecretRejectsAll(t *testing.T) {
	group := &Group{Path: "platform"}
	if group.CheckSecret("") || group.CheckSecret("whatever") {
		t.Fatal("empty stored secret must reject every token")
	}
}

func TestNewGroupSetBotLookup(t *testing.T) {
	lookup := func(ctx context.Context, client *gitlab.Client) (int, error) {
		return 999, nil
	}
	set, warnings := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t", WebhookSecret: "s"},
	}, "https://gitlab.example.com", lookup)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if !set.BotIDs()[999] {
		t.Fatalf("bot ids = %v", set.BotIDs())
	}
	if set.Match("platform/api").BotUserID != 999 {
		t.Fatal("group bot id not set")
	}
}

func TestNewGroupSetBotLookupFailureIsWarning(t *testing.T) {
	lookup := func(ctx context.Context, client *gitlab.Client) (int, error) {
		return 0, errors.New("boom")
	}
	set, warnings := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t", WebhookSecret: "s"},
	}, "https://gitlab.example.com", lookup)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v", warnings)
	}
	if set.Match("platform/api") == nil {
		t.Fatal("group must still be usable without bot id")
	}
}
