package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// diagnoseSCMErrors makes cmd and everything under it explain a "not found"
// from an SCM API in terms of the checkout the command ran in. The platform is
// read off the error itself, so a command that can talk to either forge — or
// to one of them only under a flag, like `chat --gitlab` — needs no
// declaration here.
func (a *app) diagnoseSCMErrors(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		a.diagnoseSCMErrors(sub)
	}
	run := cmd.RunE
	if run == nil {
		return
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		return a.explainSCMNotFound(c.Context(), c.CommandPath(), run(c, args))
	}
}

// explainSCMNotFound appends what this checkout says about a 404 from a GitHub
// or GitLab API: the request's project is looked for on the platform the
// command addressed, while the remotes say which platform the repository
// actually lives on. Anything else — a different status, a local failure —
// passes through untouched.
func (a *app) explainSCMNotFound(ctx context.Context, cmdPath string, err error) error {
	platform, ok := scmNotFound(err)
	if !ok {
		return err
	}
	hint := a.notFoundHint(ctx, cmdPath, platform)
	if hint == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, hint)
}

// scmNotFound reports whether err is a 404 from one of the SCM APIs, and from
// which one.
func scmNotFound(err error) (scmPlatform, bool) {
	var gitlabErr *glscm.APIError
	if errors.As(err, &gitlabErr) && gitlabErr.Status == http.StatusNotFound {
		return platformGitLab, true
	}
	var githubErr *ghscm.APIError
	if errors.As(err, &githubErr) && githubErr.Status == http.StatusNotFound {
		return platformGitHub, true
	}
	return platformNone, false
}

// notFoundHint is the sentence explaining the 404 from what this checkout's
// remotes say, or "" when the remotes add nothing the error does not already
// carry — which is the case when the repository really does live on the
// platform that answered 404, where the cause is a wrong project path, a wrong
// id, or a token without access.
func (a *app) notFoundHint(ctx context.Context, cmdPath string, platform scmPlatform) string {
	gitlabHost := a.gitlabAPIHost()
	remotes := a.currentCheckoutRemotes(ctx)
	switch {
	case len(remotes) == 0:
		return fmt.Sprintf("The current directory has no git remote that could say whether this project is on %s at all. "+
			"Check the project path (--repo or --url), the request id, and the token's access — "+
			"or read a review of a local checkout with `nickpit git feedback`.", platform.name())
	case len(remotesOn(remotes, platform)) > 0:
		return a.samePlatformHint(remotes, platform, gitlabHost)
	case len(remotesOn(remotes, platform.other())) > 0:
		return wrongPlatformHint(cmdPath, remotesOn(remotes, platform.other())[0], platform)
	default:
		return unknownHostHint(remotes[0], platform, gitlabHost)
	}
}

// samePlatformHint covers a checkout that does belong to the platform that
// answered 404. On GitLab that still leaves one thing the error cannot know:
// the remote names a different GitLab instance than the one this run talked
// to, so the project exists — elsewhere. Otherwise the API error's own advice
// (project path, id, token access) is complete and nothing is added.
func (a *app) samePlatformHint(remotes []checkoutRemote, platform scmPlatform, gitlabHost string) string {
	if platform != platformGitLab {
		return ""
	}
	for _, remote := range remotes {
		if remote.platform != platformGitLab || gitlabHost == "" || normalizeHost(remote.host) == normalizeHost(gitlabHost) {
			continue
		}
		return fmt.Sprintf("The git remote %q of this checkout points at the GitLab instance %s, "+
			"but this run talked to %s. Pass --gitlab-base-url https://%s/api/v4 "+
			"(or set NICKPIT_GITLAB_BASE_URL) to reach the project where it lives.",
			textsan.StripControl(remote.name), textsan.StripControl(remote.host),
			textsan.StripControl(gitlabHost), textsan.StripControl(remote.host))
	}
	return ""
}

// wrongPlatformHint covers the plain mix-up: the command asked one forge for a
// project that lives on the other.
func wrongPlatformHint(cmdPath string, remote checkoutRemote, asked scmPlatform) string {
	found := remote.platform
	hint := fmt.Sprintf("The git remote %q of this checkout points at %s, a %s repository, "+
		"so there is nothing to find on %s. ",
		textsan.StripControl(remote.name), textsan.StripControl(remoteLocation(remote)),
		found.name(), asked.name())
	if counterpart := counterpartCommand(cmdPath, found); counterpart != "" {
		return hint + fmt.Sprintf("Run `%s` instead, or pass --repo/--url to name a %s project.",
			counterpart, asked.name())
	}
	// Not every command has a counterpart to name (`chat`, `gitlab templates`),
	// so the platform is named instead of promising a command that may not
	// exist there.
	return hint + fmt.Sprintf("This repository lives on %s (`nickpit %s ...`); pass --repo/--url to name a %s project.",
		found.name(), string(found), asked.name())
}

// unknownHostHint covers a checkout whose remotes point at neither forge: a
// self-hosted GitLab that was never configured looks exactly like an unrelated
// git host, so both ways out are named.
func unknownHostHint(remote checkoutRemote, asked scmPlatform, gitlabHost string) string {
	configured := "no GitLab host is configured"
	if gitlabHost != "" {
		configured = "the configured GitLab host is " + textsan.StripControl(gitlabHost)
	}
	return fmt.Sprintf("The git remote %q of this checkout points at %s, which is neither github.com nor a GitLab host "+
		"this run knows (%s), so nothing here says the repository is on %s. "+
		"Point --gitlab-base-url at it if that host is your GitLab, name the project with --repo or --url, "+
		"or read a review of the local checkout with `nickpit git feedback`.",
		textsan.StripControl(remote.name), textsan.StripControl(remoteLocation(remote)), configured, asked.name())
}

// remoteLocation names where a remote points in the shortest form that still
// identifies it: "github.com/owner/repo", falling back to the configured URL
// when it could not be parsed.
func remoteLocation(remote checkoutRemote) string {
	if remote.host == "" {
		return remote.url
	}
	if remote.repo == "" {
		return remote.host
	}
	return remote.host + "/" + remote.repo
}

// counterpartCommand rewrites the invoked command path to the same command on
// target, e.g. "nickpit gitlab feedback" to "nickpit github feedback" and
// "nickpit gitlab mr" to "nickpit github pr". A command with no counterpart
// there — `chat`, `gitlab serve`, `gitlab templates` — yields "".
func counterpartCommand(cmdPath string, target scmPlatform) string {
	if target == platformNone {
		return ""
	}
	segments := strings.Fields(cmdPath)
	platformAt := -1
	for i, segment := range segments {
		if segment == string(platformGitLab) || segment == string(platformGitHub) {
			platformAt = i
			break
		}
	}
	if platformAt < 0 || platformAt != len(segments)-2 {
		return ""
	}
	last := segments[len(segments)-1]
	switch last {
	case "feedback":
	case "mr", "pr":
		last = map[scmPlatform]string{platformGitLab: "mr", platformGitHub: "pr"}[target]
	default:
		return ""
	}
	segments[platformAt] = string(target)
	segments[len(segments)-1] = last
	return strings.Join(segments, " ")
}
