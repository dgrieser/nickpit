package forgejo

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestForgeIdentity(t *testing.T) {
	if Forge.Mode() != model.ModeForgejo || Forge.Command() != "forgejo" || Forge.RequestCommand() != "pr" || Forge.RequestSigil() != "#" {
		t.Fatalf("identity = %s %s %s %s", Forge.Mode(), Forge.Command(), Forge.RequestCommand(), Forge.RequestSigil())
	}
	if !Forge.ConfigurableBaseURL() {
		t.Fatal("every Forgejo is self-hosted")
	}
}

func TestParseRequestURL(t *testing.T) {
	tests := []struct {
		raw     string
		repo    string
		id      int
		baseURL string
	}{
		{"https://codeberg.org/forgejo/forgejo/pulls/1234", "forgejo/forgejo", 1234, "https://codeberg.org"},
		{"https://codeberg.org/forgejo/forgejo/pulls/1234/files?style=split#diff-abc", "forgejo/forgejo", 1234, "https://codeberg.org"},
		{"http://forge.internal:3000/team/app/pulls/7/commits", "team/app", 7, "http://forge.internal:3000"},
		{"https://git.example.com/forgejo/team/app/pulls/7", "team/app", 7, "https://git.example.com/forgejo"},
		{"https://git.example.com/tools/forgejo/team/app/pulls/7/files", "team/app", 7, "https://git.example.com/tools/forgejo"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			repo, id, baseURL, err := Forge.ParseRequestURL(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if repo != tt.repo || id != tt.id || baseURL != tt.baseURL {
				t.Fatalf("ParseRequestURL() = %q, %d, %q; want %q, %d, %q", repo, id, baseURL, tt.repo, tt.id, tt.baseURL)
			}
		})
	}
}

func TestParseRequestURLRejectsInvalid(t *testing.T) {
	tests := []string{
		"",
		"ssh://codeberg.org/owner/repo/pulls/1",
		"https:///owner/repo/pulls/1",
		"https://codeberg.org/owner/repo/pull/1",
		"https://codeberg.org/owner/repo/issues/1",
		"https://codeberg.org/owner/pulls/1",
		"https://codeberg.org/owner/repo/pulls/not-a-number",
		"https://codeberg.org/owner/repo/pulls/0",
		"https://codeberg.org/owner/repo/pulls",
		"https://codeberg.org/sub/owner/repo/pulls/",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, _, _, err := Forge.ParseRequestURL(raw); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestTrustedHostAndRemoteDetection(t *testing.T) {
	if got := Forge.TrustedHost("codeberg.org"); got != "codeberg.org" {
		t.Fatalf("TrustedHost = %q", got)
	}
	if got := Forge.TrustedHost("https://Forge.Internal:3000/api/v1"); got != "forge.internal" {
		t.Fatalf("TrustedHost = %q", got)
	}
	// No configured instance means no host to trust and no remote to claim —
	// not even an unreadable one.
	if Forge.TrustedHost("") != "" {
		t.Fatal("an unconfigured instance has no trusted host")
	}
	if Forge.MatchesRemote("", "") || Forge.MatchesRemote("git@codeberg.org:o/r.git", "") {
		t.Fatal("an unconfigured instance must not claim a remote")
	}
	if !Forge.MatchesRemote("git@codeberg.org:o/r.git", "https://codeberg.org/api/v1") {
		t.Fatal("the configured host was not claimed")
	}
	if Forge.MatchesRemote("git@gitlab.example.com:o/r.git", "https://codeberg.org/api/v1") {
		t.Fatal("a foreign host was claimed")
	}
}

func TestGitCredentials(t *testing.T) {
	if got := Forge.GitCredentials("secret"); got != "oauth2:secret" {
		t.Fatalf("GitCredentials = %q", got)
	}
	if got := Forge.GitCredentials(""); got != "" {
		t.Fatalf("empty token must yield no credentials, got %q", got)
	}
}
