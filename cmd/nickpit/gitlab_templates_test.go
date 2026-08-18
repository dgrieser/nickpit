package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

func writeTemplateServeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveTemplateTargetsRequiresScope(t *testing.T) {
	a := &app{}
	_, err := a.resolveTemplateTargets(templateFlags{})
	if err == nil || !strings.Contains(err.Error(), "no scope selected") {
		t.Fatalf("error = %v, want a missing-scope error", err)
	}
}

// A serve config contributes one group target per configured group, each with
// that group's own token, and the config's command_keyword names the templates.
func TestResolveTemplateTargetsFromServeConfig(t *testing.T) {
	path := writeTemplateServeConfig(t, `
gitlab_base_url: "https://gitlab.example.com"
command_keyword: "bot"
groups:
  - path: "/platform/"
    token: "tok-a"
    webhook_secret: "sec"
  - path: "tenant"
    token: "tok-b"
    webhook_secret: "sec"
`)
	a := &app{}
	targets, err := a.resolveTemplateTargets(templateFlags{serveConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %d, want one per group", len(targets))
	}
	for i, want := range []string{"platform", "tenant"} {
		if targets[i].scope.Kind != glscm.SavedReplyScopeGroup || targets[i].scope.Path != want {
			t.Fatalf("target %d scope = %v, want group %q", i, targets[i].scope, want)
		}
		if targets[i].keyword != "bot" {
			t.Fatalf("target %d keyword = %q, want the config keyword", i, targets[i].keyword)
		}
	}
	if targets[0].client == targets[1].client {
		t.Fatal("groups must not share a client: each carries its own token")
	}
}

// --keyword overrides the serve config, so an operator can seed templates for a
// keyword the file does not (yet) declare.
func TestResolveTemplateTargetsKeywordOverride(t *testing.T) {
	path := writeTemplateServeConfig(t, `
command_keyword: "bot"
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`)
	a := &app{}
	targets, err := a.resolveTemplateTargets(templateFlags{serveConfigPath: path, keyword: "review-bot"})
	if err != nil {
		t.Fatal(err)
	}
	if targets[0].keyword != "review-bot" {
		t.Fatalf("keyword = %q, want the flag value", targets[0].keyword)
	}
}

func TestResolveTemplateTargetsServeConfigWithoutGroups(t *testing.T) {
	path := writeTemplateServeConfig(t, "listen: \":9090\"\n")
	a := &app{}
	_, err := a.resolveTemplateTargets(templateFlags{serveConfigPath: path})
	if err == nil || !strings.Contains(err.Error(), "at least one group must be configured") {
		t.Fatalf("error = %v, want the serve config's no-groups error", err)
	}
}

// Explicit scopes read the profile's GitLab credentials and default the keyword.
func TestResolveTemplateTargetsExplicitScopes(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".nickpit.yaml")
	if err := os.WriteFile(configPath, []byte("profiles:\n  default:\n    gitlab_base_url: \"https://gitlab.example.com\"\n    gitlab_token: \"tok\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// No model is configured: the profile only has to supply GitLab credentials.
	a := &app{configPath: configPath, profile: "default"}
	targets, err := a.resolveTemplateTargets(templateFlags{
		user:     true,
		groups:   []string{"acme"},
		projects: []string{"acme/widget"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantScopes := []glscm.SavedReplyScope{
		glscm.UserSavedReplyScope(),
		glscm.GroupSavedReplyScope("acme"),
		glscm.ProjectSavedReplyScope("acme/widget"),
	}
	if len(targets) != len(wantScopes) {
		t.Fatalf("targets = %d, want %d", len(targets), len(wantScopes))
	}
	for i, want := range wantScopes {
		if targets[i].scope != want {
			t.Fatalf("target %d scope = %v, want %v", i, targets[i].scope, want)
		}
		if targets[i].keyword != "nickpit" {
			t.Fatalf("target %d keyword = %q, want the default", i, targets[i].keyword)
		}
	}
}

func TestSingleLineFlattensContent(t *testing.T) {
	if got := singleLine("/nickpit review\n\nplease"); got != "/nickpit review please" {
		t.Fatalf("singleLine = %q", got)
	}
}

// Explicit scopes need a GitLab token; without one every call would 401 late.
func TestResolveTemplateTargetsRequiresToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".nickpit.yaml")
	if err := os.WriteFile(configPath, []byte("profiles:\n  default:\n    gitlab_base_url: \"https://gitlab.example.com\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NICKPIT_GITLAB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	a := &app{configPath: configPath, profile: "default"}
	_, err := a.resolveTemplateTargets(templateFlags{user: true})
	if err == nil || !strings.Contains(err.Error(), "no GitLab token configured") {
		t.Fatalf("error = %v, want a missing-token error", err)
	}
}

// A keyword the daemon could never parse must be rejected before anything is
// seeded: templates built from it would hold bodies no configuration accepts.
func TestResolveTemplateTargetsRejectsUnusableKeyword(t *testing.T) {
	path := writeTemplateServeConfig(t, `
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`)
	cases := map[string]string{
		"leading slash": "/bot",
		"whitespace":    "nick pit",
	}
	for name, keyword := range cases {
		t.Run(name, func(t *testing.T) {
			a := &app{}
			_, err := a.resolveTemplateTargets(templateFlags{serveConfigPath: path, keyword: keyword})
			if err == nil || !strings.Contains(err.Error(), "command keyword") {
				t.Fatalf("error = %v, want a keyword rejection", err)
			}
		})
	}
}

// A malformed command_keyword in the file is rejected by LoadServe itself, so
// no target is ever built from it.
func TestResolveTemplateTargetsRejectsUnusableConfigKeyword(t *testing.T) {
	path := writeTemplateServeConfig(t, `
command_keyword: "/bot"
groups:
  - path: "platform"
    token: "tok"
    webhook_secret: "sec"
`)
	a := &app{}
	_, err := a.resolveTemplateTargets(templateFlags{serveConfigPath: path})
	if err == nil || !strings.Contains(err.Error(), "command_keyword must not start with") {
		t.Fatalf("error = %v, want a keyword rejection", err)
	}
}
