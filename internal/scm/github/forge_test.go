package github

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/scm/forge"
)

var _ forge.Source = (*Adapter)(nil)

func TestParseRequestURL(t *testing.T) {
	tests := []struct {
		raw  string
		repo string
		id   int
	}{
		{
			raw:  "https://github.com/dgrieser/nickpit/pull/60",
			repo: "dgrieser/nickpit",
			id:   60,
		},
		{
			raw:  "https://github.com/dgrieser/nickpit/pull/60/changes#diff-97bc82c1e601dde3195cc516a4ce8e58bb37ce0904b143123cc284fa568debe7L11",
			repo: "dgrieser/nickpit",
			id:   60,
		},
		{
			raw:  "https://github.com/dgrieser/nickpit/pull/60/files?plain=1",
			repo: "dgrieser/nickpit",
			id:   60,
		},
		{
			raw:  "https://www.github.com/dgrieser/nickpit/pull/60",
			repo: "dgrieser/nickpit",
			id:   60,
		},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			repo, id, baseURL, err := Forge.ParseRequestURL(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if repo != tt.repo || id != tt.id || baseURL != "" {
				t.Fatalf("ParseRequestURL() = %q, %d, %q; want %q, %d, \"\"", repo, id, baseURL, tt.repo, tt.id)
			}
		})
	}
}

func TestParseRequestURLRejectsInvalid(t *testing.T) {
	tests := []string{
		"",
		"http://github.com/dgrieser/nickpit/pull/60",
		"https://git.example.com/dgrieser/nickpit/pull/60",
		"https://github.com/dgrieser/nickpit/issues/60",
		"https://github.com/dgrieser/pull/60",
		"https://github.com/dgrieser/nickpit/pull/not-a-number",
		"https://github.com/dgrieser/nickpit/pull/0",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, _, _, err := Forge.ParseRequestURL(raw); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMatchesRemote(t *testing.T) {
	cases := []struct {
		remote string
		github bool
	}{
		{"git@github.com:owner/repo.git", true},
		{"https://github.com/owner/repo.git", true},
		{"git@gitlab.example.com:grp/proj.git", false},
		{"https://gitlab.example.com/grp/proj.git", false},
		{"", false},
	}
	for _, c := range cases {
		if got := Forge.MatchesRemote(c.remote, ""); got != c.github {
			t.Fatalf("MatchesRemote(%q) = %v", c.remote, got)
		}
	}
}

func TestGitCredentials(t *testing.T) {
	if got := Forge.GitCredentials("secret"); got != "x-access-token:secret" {
		t.Fatalf("GitCredentials = %q", got)
	}
	if got := Forge.GitCredentials(""); got != "" {
		t.Fatalf("empty token must yield no credentials, got %q", got)
	}
	if Forge.TrustedHost("https://anything.example") != "github.com" {
		t.Fatal("the GitHub token belongs to github.com only")
	}
}
