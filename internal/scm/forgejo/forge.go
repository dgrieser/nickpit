package forgejo

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/forge"
)

// Forge is Forgejo as a platform: always self-hosted (Codeberg included), so
// the API base URL is configuration with no default, "pull request" wording,
// and the adapter of this package as the review source.
var Forge forge.Forge = forgejoForge{}

type forgejoForge struct{}

func (forgejoForge) Mode() model.ReviewMode { return model.ModeForgejo }
func (forgejoForge) Name() string           { return "Forgejo" }
func (forgejoForge) Command() string        { return "forgejo" }
func (forgejoForge) RequestCommand() string { return "pr" }
func (forgejoForge) RequestNoun() string    { return "pull request" }
func (forgejoForge) RequestAbbrev() string  { return "PR" }
func (forgejoForge) RequestSigil() string   { return "#" }

func (forgejoForge) RequestHelp() forge.RequestHelp {
	return forge.RequestHelp{
		Short:   "Review a Forgejo PR",
		Repo:    "Forgejo repo owner/name (inferred from git remote if omitted)",
		ID:      "Pull request number (omit in a terminal to pick an open PR from a list)",
		Publish: "Post the review back to the Forgejo PR as a review (summary + one comment per finding)",
	}
}

// ConfigurableBaseURL is true: there is no forgejo.com to default to.
func (forgejoForge) ConfigurableBaseURL() bool { return true }

func (forgejoForge) NormalizeBaseURL(raw string) string { return NormalizeBaseURL(raw) }

// TrustedHost is the host of the configured API base URL, or nothing when no
// instance is configured: a token without a host is never sent anywhere.
func (forgejoForge) TrustedHost(baseURL string) string {
	baseURL = NormalizeBaseURL(baseURL)
	if baseURL == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// ParseRequestURL accepts any http(s) host with a path of the shape
// owner/repo/pulls/N, with any trailing path, query or fragment as copied from
// the browser. An instance served under a subpath puts that subpath before
// owner/repo; it is kept in the returned API base, "scheme://host[/subpath]".
func (forgejoForge) ParseRequestURL(raw string) (string, int, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, "", fmt.Errorf("parsing --url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", 0, "", fmt.Errorf("--url must use http or https")
	}
	if u.Host == "" {
		return "", 0, "", fmt.Errorf("--url must include a Forgejo host")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// The first "pulls" segment with an owner and a repository before it and
	// a number after it; whatever precedes the owner is the instance subpath.
	pulls := -1
	for i := 2; i+1 < len(parts); i++ {
		if parts[i] == "pulls" {
			pulls = i
			break
		}
	}
	if pulls < 0 || parts[pulls-2] == "" || parts[pulls-1] == "" {
		return "", 0, "", fmt.Errorf("--url must be a Forgejo PR URL like https://codeberg.org/owner/repo/pulls/123")
	}
	pr, err := forge.ParseRequestID(parts[pulls+1], "pull request number")
	if err != nil {
		return "", 0, "", err
	}
	baseURL := u.Scheme + "://" + u.Host
	if subpath := parts[:pulls-2]; len(subpath) > 0 {
		baseURL += "/" + strings.Join(subpath, "/")
	}
	return parts[pulls-2] + "/" + parts[pulls-1], pr, baseURL, nil
}

// MatchesRemote claims a remote on the configured host, and nothing when no
// instance is configured — an unreadable remote is then no evidence for
// Forgejo either.
func (forgejoForge) MatchesRemote(remoteURL, baseURL string) bool {
	return baseURL != "" && forge.SameHost(remoteURL, baseURL)
}

// GitCredentials sends the token as the basic-auth password; Forgejo ignores
// the user name for a token, so the oauth2 spelling GitLab documents is used.
func (forgejoForge) GitCredentials(token string) string {
	if token == "" {
		return ""
	}
	return "oauth2:" + token
}

func (forgejoForge) NewSource(baseURL, token, assetBaseURL string) forge.Source {
	return NewAdapter(NewClient(baseURL, token), assetBaseURL)
}
