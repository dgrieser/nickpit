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
