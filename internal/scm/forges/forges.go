// Package forges is the wiring of every known platform: the ordered registry
// the CLI iterates, and the helpers that join a platform to the configuration
// that holds its credentials. It is the only package that imports both
// internal/config and the platform packages, so nothing below it (the scm
// packages, internal/git) has to know how credentials are configured.
package forges

import (
	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/scm/forge"
	"github.com/dgrieser/nickpit/internal/scm/forgejo"
	"github.com/dgrieser/nickpit/internal/scm/github"
	"github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// All lists the platforms in detection order: GitHub first, because it owns a
// fixed host, then the self-hostable platforms, which claim whatever host they
// are configured with.
var All = forge.Registry{github.Forge, gitlab.Forge, forgejo.Forge}

// Credentials reads the token and API base URL the profile holds for f. The
// base URL is the profile's canonical one (config normalizes it on load) and
// empty for a platform with a fixed host.
func Credentials(f forge.Forge, profile config.Profile) (token, baseURL string) {
	return profile.ForgeToken(f.Mode()), profile.ForgeBaseURL(f.Mode())
}

// HistoryAuth builds the per-host credentials the history provider deepens a
// shallow checkout with: one entry per platform, in registry order, bound to
// the host its configured base URL names. A platform without a token still
// claims its host with empty credentials, so a later platform configured with
// the same host never lends it its token.
func HistoryAuth(profile config.Profile) git.HistoryAuth {
	var auth git.HistoryAuth
	for _, f := range All {
		token, baseURL := Credentials(f, profile)
		auth.Hosts = append(auth.Hosts, git.HostCredential{
			Host:        f.TrustedHost(baseURL),
			Credentials: f.GitCredentials(token),
		})
	}
	return auth
}
