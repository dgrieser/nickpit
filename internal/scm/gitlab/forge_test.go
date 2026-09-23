package gitlab

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/scm/forge"
)

var _ forge.Source = (*Adapter)(nil)

func TestParseRequestURL(t *testing.T) {
	tests := []struct {
		raw     string
		repo    string
		id      int
		baseURL string
	}{
		{
			raw:     "https://gitlab.mittwald.it/asylum/services/kopieerapparaat/-/merge_requests/366",
			repo:    "asylum/services/kopieerapparaat",
			id:      366,
			baseURL: "https://gitlab.mittwald.it",
		},
		{
			raw:     "https://gitlab.mittwald.it/asylum/services/kopieerapparaat/-/merge_requests/366/diffs?file_path=pkg%2Frestic%2Frestic-cmd-directory-target-dir-overwrite.tpl#line_5578594d5_3",
			repo:    "asylum/services/kopieerapparaat",
			id:      366,
			baseURL: "https://gitlab.mittwald.it",
		},
		{
			raw:     "http://localhost:8080/group/project/-/merge_requests/7/commits",
			repo:    "group/project",
			id:      7,
			baseURL: "http://localhost:8080",
		},
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
		"ssh://gitlab.mittwald.it/group/project/-/merge_requests/366",
		"https:///group/project/-/merge_requests/366",
		"https://gitlab.mittwald.it/group/project/merge_requests/366",
		"https://gitlab.mittwald.it/-/merge_requests/366",
		"https://gitlab.mittwald.it/group/project/-/merge_requests/not-a-number",
		"https://gitlab.mittwald.it/group/project/-/merge_requests/0",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, _, _, err := Forge.ParseRequestURL(raw); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestTrustedHost(t *testing.T) {
	cases := []struct {
		baseURL string
		host    string
	}{
		{"", "gitlab.com"},
		{"https://gitlab.example.com/api/v4", "gitlab.example.com"},
		{"http://gitlab.example.com/api/v4", "gitlab.example.com"},
		{"gitlab.internal", "gitlab.internal"},
		{"gitlab.internal:8443/api/v4", "gitlab.internal"},
		{"GitLab.Example.COM", "gitlab.example.com"},
	}
	for _, c := range cases {
		if got := Forge.TrustedHost(c.baseURL); got != c.host {
			t.Fatalf("TrustedHost(%q) = %q, want %q", c.baseURL, got, c.host)
		}
	}
}

func TestMatchesRemote(t *testing.T) {
	// A token only ever goes to the host the remote names; an unreadable remote
	// leaves the profile's own host in charge.
	if !Forge.MatchesRemote("git@gitlab.example.com:grp/proj.git", "https://gitlab.example.com/api/v4") {
		t.Fatal("the project's own host was rejected")
	}
	if Forge.MatchesRemote("git@gitlab.other.com:grp/proj.git", "https://gitlab.example.com/api/v4") {
		t.Fatal("a token would have gone to a foreign host")
	}
	if !Forge.MatchesRemote("", "https://gitlab.example.com/api/v4") {
		t.Fatal("an unknown remote must not disable the configured host")
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
