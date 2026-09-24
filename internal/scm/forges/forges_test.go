package forges

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/forge"
)

func TestRegistryLookupAndDetect(t *testing.T) {
	if f, ok := All.Lookup(model.ModeGitHub); !ok || f.Name() != "GitHub" {
		t.Fatalf("Lookup(github) = %v, %t", f, ok)
	}
	if f, ok := All.Lookup(model.ModeGitLab); !ok || f.Name() != "GitLab" {
		t.Fatalf("Lookup(gitlab) = %v, %t", f, ok)
	}
	if _, ok := All.Lookup(model.ModeLocal); ok {
		t.Fatal("local is not a platform")
	}
	profile := config.Profile{GitLabBaseURL: "https://gitlab.example.com/api/v4", ForgejoBaseURL: "https://codeberg.org/api/v1"}
	baseURL := func(f forge.Forge) string { _, u := Credentials(f, profile); return u }
	cases := []struct {
		remote string
		mode   model.ReviewMode
	}{
		{"git@github.com:owner/repo.git", model.ModeGitHub},
		{"https://gitlab.example.com/grp/proj.git", model.ModeGitLab},
		{"git@codeberg.org:grp/proj.git", model.ModeForgejo},
		// An unreadable remote is no evidence, so the configured GitLab host
		// claims it, as it always did.
		{"", model.ModeGitLab},
	}
	for _, c := range cases {
		f, ok := All.Detect(c.remote, baseURL)
		if !ok || f.Mode() != c.mode {
			t.Fatalf("Detect(%q) = %v, %t; want %s", c.remote, f, ok, c.mode)
		}
	}
	if f, ok := All.Detect("https://gitlab.other.com/grp/proj.git", baseURL); ok {
		t.Fatalf("a remote on an unconfigured host was attributed to %s", f.Name())
	}
}

func TestHistoryAuthBindsEachTokenToItsHost(t *testing.T) {
	auth := HistoryAuth(config.Profile{GitHubToken: "ghp", GitLabToken: "glpat", GitLabBaseURL: "https://gitlab.example.com/api/v4", ForgejoToken: "fj", ForgejoBaseURL: "https://codeberg.org/api/v1"})
	want := []git.HostCredential{
		{Host: "github.com", Credentials: "x-access-token:ghp"},
		{Host: "gitlab.example.com", Credentials: "oauth2:glpat"},
		{Host: "codeberg.org", Credentials: "oauth2:fj"},
	}
	if len(auth.Hosts) != len(want) {
		t.Fatalf("hosts = %#v", auth.Hosts)
	}
	for i := range want {
		if auth.Hosts[i] != want[i] {
			t.Fatalf("hosts[%d] = %#v, want %#v", i, auth.Hosts[i], want[i])
		}
	}
	// Without a base URL the GitLab token belongs to gitlab.com; a platform
	// without a token still claims its host, with no credentials.
	auth = HistoryAuth(config.Profile{GitLabToken: "glpat"})
	want = []git.HostCredential{
		{Host: "github.com"},
		{Host: "gitlab.com", Credentials: "oauth2:glpat"},
		{},
	}
	if len(auth.Hosts) != len(want) {
		t.Fatalf("hosts = %#v", auth.Hosts)
	}
	for i := range want {
		if auth.Hosts[i] != want[i] {
			t.Fatalf("hosts[%d] = %#v, want %#v", i, auth.Hosts[i], want[i])
		}
	}
	// A Forgejo token without an instance has no host to travel to, and the
	// history provider never matches an entry without one.
	auth = HistoryAuth(config.Profile{ForgejoToken: "fj"})
	if got := auth.Hosts[2]; got.Host != "" || got.Credentials != "oauth2:fj" {
		t.Fatalf("forgejo entry = %#v", got)
	}
}
