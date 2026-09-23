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
	profile := config.Profile{GitLabBaseURL: "https://gitlab.example.com/api/v4"}
	baseURL := func(f forge.Forge) string { _, u := Credentials(f, profile); return u }
	cases := []struct {
		remote string
		mode   model.ReviewMode
	}{
		{"git@github.com:owner/repo.git", model.ModeGitHub},
		{"https://gitlab.example.com/grp/proj.git", model.ModeGitLab},
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
	auth := HistoryAuth(config.Profile{GitHubToken: "ghp", GitLabToken: "glpat", GitLabBaseURL: "https://gitlab.example.com/api/v4"})
	want := []git.HostCredential{
		{Host: "github.com", Credentials: "x-access-token:ghp"},
		{Host: "gitlab.example.com", Credentials: "oauth2:glpat"},
	}
	if len(auth.Hosts) != len(want) {
		t.Fatalf("hosts = %#v", auth.Hosts)
	}
	for i := range want {
		if auth.Hosts[i] != want[i] {
			t.Fatalf("hosts[%d] = %#v, want %#v", i, auth.Hosts[i], want[i])
		}
	}
	// Without a base URL the GitLab token belongs to gitlab.com; without a
	// token a platform contributes nothing.
	auth = HistoryAuth(config.Profile{GitLabToken: "glpat"})
	if len(auth.Hosts) != 1 || auth.Hosts[0].Host != "gitlab.com" {
		t.Fatalf("hosts = %#v", auth.Hosts)
	}
	if auth = HistoryAuth(config.Profile{}); len(auth.Hosts) != 0 {
		t.Fatalf("hosts = %#v", auth.Hosts)
	}
}
