package github

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/forge"
)

// Forge is GitHub as a platform: fixed host, "pull request" wording, and the
// adapter of this package as the review source.
var Forge forge.Forge = githubForge{}

// defaultBaseURL is the only API host: GitHub Enterprise is not configurable
// anywhere in nickpit today, so there is no other trusted host to derive.
const defaultBaseURL = "https://api.github.com"

// trustedHost is the only host the GitHub token is sent to by git.
const trustedHost = "github.com"

type githubForge struct{}

func (githubForge) Mode() model.ReviewMode { return model.ModeGitHub }
func (githubForge) Name() string           { return "GitHub" }
func (githubForge) Command() string        { return "github" }
func (githubForge) RequestCommand() string { return "pr" }
func (githubForge) RequestNoun() string    { return "pull request" }
func (githubForge) RequestAbbrev() string  { return "PR" }
func (githubForge) RequestSigil() string   { return "#" }

// ConfigurableBaseURL is false: only github.com is served.
func (githubForge) ConfigurableBaseURL() bool { return false }

// NormalizeBaseURL ignores its input: the API host is fixed.
func (githubForge) NormalizeBaseURL(string) string { return defaultBaseURL }

// TrustedHost ignores its input for the same reason.
func (githubForge) TrustedHost(string) string { return trustedHost }

// ParseRequestURL accepts https://github.com/owner/repo/pull/N (and the
// www. spelling) with any trailing path, query or fragment, as copied from the
// browser. The base URL is always empty: GitHub has no per-host API base.
func (githubForge) ParseRequestURL(raw string) (string, int, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, "", fmt.Errorf("parsing --url: %w", err)
	}
	if u.Scheme != "https" || (!strings.EqualFold(u.Host, "github.com") && !strings.EqualFold(u.Host, "www.github.com")) {
		return "", 0, "", fmt.Errorf("--url must use https://github.com or https://www.github.com")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || parts[2] != "pull" {
		return "", 0, "", fmt.Errorf("--url must be a GitHub PR URL like https://github.com/owner/repo/pull/123")
	}
	pr, err := forge.ParseRequestID(parts[3], "pull request number")
	if err != nil {
		return "", 0, "", err
	}
	return parts[0] + "/" + parts[1], pr, "", nil
}

// MatchesRemote reports whether a remote URL points at github.com.
func (githubForge) MatchesRemote(remoteURL, _ string) bool {
	host := forge.RemoteHost(remoteURL)
	return host == "github.com" || strings.HasSuffix(host, ".github.com")
}

// GitCredentials uses the x-access-token user GitHub documents for token
// basic auth.
func (githubForge) GitCredentials(token string) string {
	if token == "" {
		return ""
	}
	return "x-access-token:" + token
}

func (githubForge) NewSource(_, token, assetBaseURL string) forge.Source {
	return NewAdapter(NewClient("", token), assetBaseURL)
}
