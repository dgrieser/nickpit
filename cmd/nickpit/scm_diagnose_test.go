package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

func gitlabNotFound() error {
	return fmt.Errorf("listing open merge requests of grp/proj: %w",
		&glscm.APIError{Method: "GET", URL: "https://gitlab.example.com/api/v4/projects/grp%2Fproj/merge_requests",
			Status: http.StatusNotFound, Body: `{"message":"404 Project Not Found"}`})
}

func githubNotFound() error {
	return fmt.Errorf("listing open pull requests of owner/repo: %w",
		&ghscm.APIError{Method: "GET", URL: "https://api.github.com/repos/owner/repo/pulls",
			Status: http.StatusNotFound, Body: `{"message":"Not Found"}`})
}

func TestSCMNotFoundRecognizesBothPlatforms(t *testing.T) {
	if platform, ok := scmNotFound(gitlabNotFound()); !ok || platform != platformGitLab {
		t.Errorf("gitlab 404 = (%q, %v)", platform, ok)
	}
	if platform, ok := scmNotFound(githubNotFound()); !ok || platform != platformGitHub {
		t.Errorf("github 404 = (%q, %v)", platform, ok)
	}
	other := &glscm.APIError{Status: http.StatusUnauthorized}
	if _, ok := scmNotFound(fmt.Errorf("wrapped: %w", other)); ok {
		t.Error("a 401 must not be diagnosed as a missing project")
	}
	if _, ok := scmNotFound(errors.New("git: no such ref")); ok {
		t.Error("a local failure must not be diagnosed as a missing project")
	}
}

// The diagnosis is added to the error, never in place of it: the raw API line
// is what makes a report reproducible, and errors.As must keep working for the
// callers that branch on the status.
func TestExplainSCMNotFoundKeepsTheOriginalError(t *testing.T) {
	dir := gitRepoWithRemotes(t, map[string]string{"origin": "git@github.com:owner/repo.git"})
	t.Chdir(dir)

	a := &app{}
	err := a.explainSCMNotFound(context.Background(), "nickpit gitlab feedback", gitlabNotFound())
	var apiErr *glscm.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("wrapped error lost its API error: %v", err)
	}
	for _, want := range []string{"404 Project Not Found", "github.com/owner/repo", "nickpit github feedback"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("diagnosed error %q is missing %q", err.Error(), want)
		}
	}
}

func TestExplainSCMNotFoundPassesOtherErrorsThrough(t *testing.T) {
	a := &app{}
	original := errors.New("connection refused")
	if got := a.explainSCMNotFound(context.Background(), "nickpit gitlab feedback", original); got != original {
		t.Fatalf("error was rewritten: %v", got)
	}
	if got := a.explainSCMNotFound(context.Background(), "nickpit gitlab feedback", nil); got != nil {
		t.Fatalf("nil became %v", got)
	}
}

func TestNotFoundHintExplainsEveryCheckout(t *testing.T) {
	cases := []struct {
		name       string
		remotes    map[string]string
		asked      scmPlatform
		cmdPath    string
		gitlabHost string
		want       []string
		unwanted   []string
	}{
		{
			name:       "gitlab command in a github checkout",
			remotes:    map[string]string{"origin": "git@github.com:owner/repo.git"},
			asked:      platformGitLab,
			cmdPath:    "nickpit gitlab feedback",
			gitlabHost: "gitlab.example.com",
			want:       []string{"github.com/owner/repo", "a GitHub repository", "nickpit github feedback"},
		},
		{
			name:       "github command in a gitlab checkout",
			remotes:    map[string]string{"origin": "git@gitlab.example.com:group/project.git"},
			asked:      platformGitHub,
			cmdPath:    "nickpit github pr",
			gitlabHost: "gitlab.example.com",
			want:       []string{"gitlab.example.com/group/project", "a GitLab repository", "nickpit gitlab mr"},
		},
		{
			name:       "neither platform",
			remotes:    map[string]string{"origin": "https://git.example.com/owner/repo.git"},
			asked:      platformGitLab,
			cmdPath:    "nickpit gitlab feedback",
			gitlabHost: "gitlab.example.com",
			want: []string{"git.example.com/owner/repo", "neither github.com nor a GitLab host",
				"the configured GitLab host is gitlab.example.com", "nickpit git feedback"},
		},
		{
			name:    "no remote at all",
			asked:   platformGitHub,
			cmdPath: "nickpit github feedback",
			want:    []string{"no git remote", "nickpit git feedback"},
		},
		{
			name:       "gitlab project on another instance",
			remotes:    map[string]string{"origin": "git@gitlab.internal.example:group/project.git"},
			asked:      platformGitLab,
			cmdPath:    "nickpit gitlab feedback",
			gitlabHost: "gitlab.example.com",
			want:       []string{"gitlab.internal.example", "--gitlab-base-url https://gitlab.internal.example/api/v4"},
		},
		{
			// The repository is where the command looked, so the API error's own
			// advice is complete and nothing is added to it.
			name:       "same platform, same host",
			remotes:    map[string]string{"origin": "git@gitlab.example.com:group/project.git"},
			asked:      platformGitLab,
			cmdPath:    "nickpit gitlab feedback",
			gitlabHost: "gitlab.example.com",
			unwanted:   []string{"remote"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(gitRepoWithRemotes(t, tc.remotes))
			a := &app{gitlabBaseURL: gitlabBaseURLFor(tc.gitlabHost)}
			hint := a.notFoundHint(context.Background(), tc.cmdPath, tc.asked)
			for _, want := range tc.want {
				if !strings.Contains(hint, want) {
					t.Errorf("hint %q is missing %q", hint, want)
				}
			}
			for _, unwanted := range tc.unwanted {
				if strings.Contains(hint, unwanted) {
					t.Errorf("hint %q should have been empty, contains %q", hint, unwanted)
				}
			}
		})
	}
}

func gitlabBaseURLFor(host string) string {
	if host == "" {
		return ""
	}
	return "https://" + host + "/api/v4"
}

// Without a configured GitLab host the hint must say so rather than name one,
// since pointing --gitlab-base-url somewhere is then the actual fix.
func TestUnknownHostHintNamesTheMissingConfiguration(t *testing.T) {
	remote := checkoutRemote{name: "origin", host: "git.example.com", repo: "owner/repo"}
	hint := unknownHostHint(remote, platformGitLab, "")
	if !strings.Contains(hint, "no GitLab host is configured") {
		t.Errorf("hint = %q", hint)
	}
}

func TestCounterpartCommand(t *testing.T) {
	cases := []struct {
		cmdPath string
		target  scmPlatform
		want    string
	}{
		{"nickpit gitlab feedback", platformGitHub, "nickpit github feedback"},
		{"nickpit github feedback", platformGitLab, "nickpit gitlab feedback"},
		{"nickpit gitlab mr", platformGitHub, "nickpit github pr"},
		{"nickpit github pr", platformGitLab, "nickpit gitlab mr"},
		// Commands with no counterpart on the other platform.
		{"nickpit gitlab templates sync", platformGitHub, ""},
		{"nickpit gitlab serve", platformGitHub, ""},
		{"nickpit chat", platformGitLab, ""},
		{"nickpit feedback", platformGitHub, ""},
	}
	for _, tc := range cases {
		if got := counterpartCommand(tc.cmdPath, tc.target); got != tc.want {
			t.Errorf("counterpartCommand(%q, %q) = %q, want %q", tc.cmdPath, tc.target, got, tc.want)
		}
	}
}

// A command with no counterpart still has to point somewhere: the platform the
// checkout is on.
func TestWrongPlatformHintWithoutCounterpartNamesThePlatform(t *testing.T) {
	remote := checkoutRemote{name: "origin", host: "github.com", repo: "owner/repo", platform: platformGitHub}
	hint := wrongPlatformHint("nickpit chat", remote, platformGitLab)
	if !strings.Contains(hint, "nickpit github ...") {
		t.Errorf("hint = %q", hint)
	}
}
