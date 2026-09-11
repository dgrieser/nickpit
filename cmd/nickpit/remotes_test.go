package main

import (
	"context"
	"os/exec"
	"testing"
)

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantHost string
		wantRepo string
	}{
		{"https", "https://github.com/owner/repo.git", "github.com", "owner/repo"},
		{"https without suffix", "https://github.com/owner/repo", "github.com", "owner/repo"},
		{"https with www", "https://WWW.GitHub.com/owner/repo.git", "github.com", "owner/repo"},
		{"ssh with port", "ssh://git@gitlab.example.com:29418/group/sub/project.git", "gitlab.example.com", "group/sub/project"},
		{"scp style", "git@github.com:owner/repo.git", "github.com", "owner/repo"},
		{"scp style nested", "git@gitlab.example.com:group/sub/project.git", "gitlab.example.com", "group/sub/project"},
		{"api base url", "https://gitlab.example.com/api/v4", "gitlab.example.com", "api/v4"},
		{"local path", "/srv/git/repo.git", "", ""},
		{"empty", "  ", "", ""},
	}
	for _, tc := range cases {
		host, repo := parseRemoteURL(tc.raw)
		if host != tc.wantHost || repo != tc.wantRepo {
			t.Errorf("%s: parseRemoteURL(%q) = (%q, %q), want (%q, %q)",
				tc.name, tc.raw, host, repo, tc.wantHost, tc.wantRepo)
		}
	}
}

func TestClassifyRemoteHost(t *testing.T) {
	cases := []struct {
		name       string
		host       string
		gitlabHost string
		want       scmPlatform
	}{
		{"github", "github.com", "gitlab.example.com", platformGitHub},
		{"configured gitlab", "gitlab.example.com", "gitlab.example.com", platformGitLab},
		// A self-hosted GitLab nobody configured still has to be recognizable:
		// that is exactly the run whose failure has to be explained.
		{"unconfigured gitlab", "gitlab.example.com", "", platformGitLab},
		{"gitlab.com", "gitlab.com", "", platformGitLab},
		{"unrelated host", "git.example.com", "gitlab.example.com", platformNone},
		{"empty", "", "gitlab.example.com", platformNone},
	}
	for _, tc := range cases {
		if got := classifyRemoteHost(tc.host, tc.gitlabHost); got != tc.want {
			t.Errorf("%s: classifyRemoteHost(%q, %q) = %q, want %q",
				tc.name, tc.host, tc.gitlabHost, got, tc.want)
		}
	}
}

// gitRepoWithRemotes creates a repository whose remotes are the given
// name/URL pairs, so the classification can be exercised against real git
// output rather than a hand-built struct.
func gitRepoWithRemotes(t *testing.T, remotes map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main", ".")
	for name, url := range remotes {
		run("remote", "add", name, url)
	}
	return dir
}

func TestCheckoutRemotesClassifiesEveryRemote(t *testing.T) {
	dir := gitRepoWithRemotes(t, map[string]string{
		"origin":   "git@github.com:owner/repo.git",
		"internal": "https://gitlab.example.com/group/project.git",
		"backup":   "https://git.example.com/owner/repo.git",
	})
	byName := map[string]checkoutRemote{}
	for _, remote := range checkoutRemotes(context.Background(), dir, "gitlab.example.com") {
		byName[remote.name] = remote
	}
	if len(byName) != 3 {
		t.Fatalf("remotes = %+v", byName)
	}
	if got := byName["origin"]; got.platform != platformGitHub || got.repo != "owner/repo" || got.host != "github.com" {
		t.Errorf("origin = %+v", got)
	}
	if got := byName["internal"]; got.platform != platformGitLab || got.repo != "group/project" {
		t.Errorf("internal = %+v", got)
	}
	if got := byName["backup"]; got.platform != platformNone {
		t.Errorf("backup = %+v", got)
	}
	if got := remotesOn(checkoutRemotes(context.Background(), dir, "gitlab.example.com"), platformGitHub); len(got) != 1 {
		t.Errorf("remotesOn(github) = %+v", got)
	}
}

// A directory without a repository, and a repository without remotes, both
// yield nothing: the classification only ever explains a failure, so it must
// not produce one of its own.
func TestCheckoutRemotesToleratesMissingRepositoryAndRemotes(t *testing.T) {
	if got := checkoutRemotes(context.Background(), t.TempDir(), ""); len(got) != 0 {
		t.Errorf("outside a repository: %+v", got)
	}
	if got := checkoutRemotes(context.Background(), gitRepoWithRemotes(t, nil), ""); len(got) != 0 {
		t.Errorf("repository without remotes: %+v", got)
	}
}
