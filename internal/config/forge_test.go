package config

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestForgeCredentialAccessors(t *testing.T) {
	profile := Profile{GitHubToken: "ghp", GitLabToken: "glpat", GitLabBaseURL: "https://gitlab.example.com/api/v4"}
	if profile.ForgeToken(model.ModeGitHub) != "ghp" || profile.ForgeToken(model.ModeGitLab) != "glpat" {
		t.Fatalf("tokens = %q, %q", profile.ForgeToken(model.ModeGitHub), profile.ForgeToken(model.ModeGitLab))
	}
	if profile.ForgeBaseURL(model.ModeGitLab) != "https://gitlab.example.com/api/v4" {
		t.Fatalf("gitlab base url = %q", profile.ForgeBaseURL(model.ModeGitLab))
	}
	// GitHub has no configurable host, and an unknown platform has nothing.
	if profile.ForgeBaseURL(model.ModeGitHub) != "" || profile.ForgeToken(model.ModeLocal) != "" {
		t.Fatal("unexpected credentials for a platform without them")
	}
}

func TestForgeOverridesApply(t *testing.T) {
	profile, err := applyOverrides(Profile{Model: "m", BaseURL: "https://llm.example", GitHubToken: "old"}, Overrides{
		ForgeTokens:   map[model.ReviewMode]string{model.ModeGitHub: "new-gh", model.ModeGitLab: "new-gl"},
		ForgeBaseURLs: map[model.ReviewMode]string{model.ModeGitLab: "gitlab.internal"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.GitHubToken != "new-gh" || profile.GitLabToken != "new-gl" {
		t.Fatalf("tokens = %q, %q", profile.GitHubToken, profile.GitLabToken)
	}
	// The override goes through the same canonicalization as the config value.
	if profile.GitLabBaseURL != "https://gitlab.internal/api/v4" {
		t.Fatalf("gitlab base url = %q", profile.GitLabBaseURL)
	}
	// An empty override leaves the configured value alone.
	profile, err = applyOverrides(Profile{Model: "m", BaseURL: "https://llm.example", GitHubToken: "old"}, Overrides{
		ForgeTokens: map[model.ReviewMode]string{model.ModeGitHub: ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.GitHubToken != "old" {
		t.Fatalf("github token = %q", profile.GitHubToken)
	}
}

func TestForgejoCredentialsLoadAndCanonicalize(t *testing.T) {
	t.Setenv("NICKPIT_MODEL", "test-model")
	t.Setenv("FORGEJO_TOKEN", "bare-forgejo")
	t.Setenv("NICKPIT_FORGEJO_TOKEN", "prefixed-forgejo")
	t.Setenv("NICKPIT_FORGEJO_BASE_URL", "codeberg.org")
	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	// The NICKPIT_ name wins, and the base URL is canonicalized once on load
	// like the GitLab one.
	if profile.ForgeToken(model.ModeForgejo) != "prefixed-forgejo" {
		t.Fatalf("forgejo token = %q", profile.ForgejoToken)
	}
	if profile.ForgeBaseURL(model.ModeForgejo) != "https://codeberg.org/api/v1" {
		t.Fatalf("forgejo base url = %q", profile.ForgejoBaseURL)
	}
	// Unlike GitLab there is no default instance: unset stays empty.
	t.Setenv("NICKPIT_FORGEJO_BASE_URL", "")
	if _, profile, err = Load("", Overrides{}); err != nil {
		t.Fatal(err)
	}
	if profile.ForgejoBaseURL != "" {
		t.Fatalf("forgejo base url = %q, want no default host", profile.ForgejoBaseURL)
	}
	// The flag overrides land in the same fields.
	profile, err = applyOverrides(Profile{Model: "m", BaseURL: "https://llm.example"}, Overrides{
		ForgeTokens:   map[model.ReviewMode]string{model.ModeForgejo: "flag-token"},
		ForgeBaseURLs: map[model.ReviewMode]string{model.ModeForgejo: "http://forge.internal:3000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.ForgejoToken != "flag-token" || profile.ForgejoBaseURL != "http://forge.internal:3000/api/v1" {
		t.Fatalf("forgejo overrides = %q, %q", profile.ForgejoToken, profile.ForgejoBaseURL)
	}
}
