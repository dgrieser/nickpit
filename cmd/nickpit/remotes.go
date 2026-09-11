package main

import (
	"context"
	"net/url"
	"os"
	"strings"

	"github.com/dgrieser/nickpit/internal/git"
)

// scmPlatform names the forge a command talks to. It is what tells "this
// project does not exist here" apart from "this project is not here at all":
// the platform a checkout belongs to is decided by its remotes, not by the
// subcommand that was typed.
type scmPlatform string

const (
	platformNone   scmPlatform = ""
	platformGitHub scmPlatform = "github"
	platformGitLab scmPlatform = "gitlab"
)

// name is the platform's own spelling, for error text a person reads.
func (p scmPlatform) name() string {
	switch p {
	case platformGitHub:
		return "GitHub"
	case platformGitLab:
		return "GitLab"
	default:
		return ""
	}
}

// other is the platform a command should be pointed at when this one turned
// out to be the wrong forge for the checkout.
func (p scmPlatform) other() scmPlatform {
	switch p {
	case platformGitHub:
		return platformGitLab
	case platformGitLab:
		return platformGitHub
	default:
		return platformNone
	}
}

// checkoutRemote is one git remote of the current checkout, split into the
// parts a platform check needs.
type checkoutRemote struct {
	// name is the remote's name ("origin").
	name string
	// url is the remote as configured, kept for error text that has to show
	// what was actually found.
	url string
	// host is the remote's host without a port or user ("github.com").
	host string
	// repo is the project path the platform APIs take ("owner/name",
	// "group/sub/project").
	repo string
	// platform is what host classifies as, platformNone when it is neither a
	// known GitHub nor a recognizable GitLab host.
	platform scmPlatform
}

// checkoutRemotes lists the remotes of dir classified against gitlabHost, the
// host the active configuration's GitLab API base URL points at. It is
// best-effort: outside a repository, or when git cannot be run at all, the
// result is empty, because every caller uses it to explain a failure that has
// already happened rather than to decide whether to make a request.
func checkoutRemotes(ctx context.Context, dir, gitlabHost string) []checkoutRemote {
	found, err := git.Remotes(ctx, dir)
	if err != nil || len(found) == 0 {
		return nil
	}
	remotes := make([]checkoutRemote, 0, len(found))
	for _, remote := range found {
		host, repo := parseRemoteURL(remote.URL)
		remotes = append(remotes, checkoutRemote{
			name:     remote.Name,
			url:      remote.URL,
			host:     host,
			repo:     repo,
			platform: classifyRemoteHost(host, gitlabHost),
		})
	}
	return remotes
}

// currentCheckoutRemotes is checkoutRemotes for the working directory.
func (a *app) currentCheckoutRemotes(ctx context.Context) []checkoutRemote {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	return checkoutRemotes(ctx, dir, a.gitlabAPIHost())
}

// remotesOn returns the remotes that belong to platform, in config order.
func remotesOn(remotes []checkoutRemote, platform scmPlatform) []checkoutRemote {
	var matching []checkoutRemote
	for _, remote := range remotes {
		if remote.platform == platform {
			matching = append(matching, remote)
		}
	}
	return matching
}

// classifyRemoteHost decides which platform a remote host belongs to.
// gitlabHost is the configured GitLab API host and is authoritative for
// GitLab; the "gitlab.*" fallback recognizes a self-hosted instance that was
// never configured, which is exactly the case where a command fails and the
// reason has to be explained.
func classifyRemoteHost(host, gitlabHost string) scmPlatform {
	host = normalizeHost(host)
	switch {
	case host == "":
		return platformNone
	case host == "github.com":
		return platformGitHub
	case gitlabHost != "" && host == normalizeHost(gitlabHost):
		return platformGitLab
	case host == "gitlab.com" || strings.HasPrefix(host, "gitlab."):
		return platformGitLab
	default:
		return platformNone
	}
}

// normalizeHost lowercases a host and drops the "www." prefix, so hosts
// compare the way a person reads them.
func normalizeHost(host string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
}

// gitlabAPIHost is the host the GitLab API calls of this invocation would go
// to: the --gitlab-base-url override when given, else the active profile's.
// Loading the profile is best-effort — this only ever feeds an explanation, so
// a broken or missing config yields "" rather than an error.
func (a *app) gitlabAPIHost() string {
	base := strings.TrimSpace(a.gitlabBaseURL)
	if base == "" {
		profile, err := a.loadProfileWithoutLLM()
		if err != nil {
			return ""
		}
		base = strings.TrimSpace(profile.GitLabBaseURL)
	}
	if base == "" {
		return ""
	}
	host, _ := parseRemoteURL(base)
	return host
}

// parseRemoteURL splits a git remote URL into its host (without port or user)
// and the project path the platform APIs take. Anything it cannot read — a
// local path, a URL with no host — yields empty strings.
func parseRemoteURL(raw string) (host, repo string) {
	raw = strings.TrimSpace(raw)
	// URL schemes: https://, ssh://, git://
	// e.g. https://github.com/owner/repo.git
	//      ssh://git@gitlab.example.com:29418/group/project.git
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", ""
		}
		path := strings.TrimPrefix(u.Path, "/")
		return normalizeHost(u.Hostname()), strings.TrimSuffix(path, ".git")
	}
	// SCP-style: git@github.com:owner/repo.git
	//            git@gitlab.com:group/project.git
	before, after, ok := strings.Cut(raw, ":")
	if !ok {
		return "", ""
	}
	if _, afterUser, hasUser := strings.Cut(before, "@"); hasUser {
		before = afterUser
	}
	return normalizeHost(before), strings.TrimSuffix(after, ".git")
}
