package gitlab

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/forge"
)

// Forge is GitLab as a platform: a configurable (self-hostable) API host,
// "merge request" wording, and the adapter of this package as the review
// source.
var Forge forge.Forge = gitlabForge{}

// defaultHost is the host a token goes to when no base URL is configured.
const defaultHost = "gitlab.com"

type gitlabForge struct{}

func (gitlabForge) Mode() model.ReviewMode { return model.ModeGitLab }
func (gitlabForge) Name() string           { return "GitLab" }
func (gitlabForge) Command() string        { return "gitlab" }
func (gitlabForge) RequestCommand() string { return "mr" }
func (gitlabForge) RequestNoun() string    { return "merge request" }
func (gitlabForge) RequestAbbrev() string  { return "MR" }
func (gitlabForge) RequestSigil() string   { return "!" }

func (gitlabForge) RequestHelp() forge.RequestHelp {
	return forge.RequestHelp{
		Short:   "Review a GitLab merge request",
		Repo:    "GitLab project group/name (inferred from git remote if omitted)",
		ID:      "Merge request IID (omit in a terminal to pick an open MR from a list)",
		Publish: "Post the review back to the GitLab MR as comments (summary + one per finding)",
	}
}

// ConfigurableBaseURL is true: GitLab is commonly self-hosted.
func (gitlabForge) ConfigurableBaseURL() bool { return true }

func (gitlabForge) NormalizeBaseURL(raw string) string { return NormalizeBaseURL(raw) }

// TrustedHost is the host of the configured API base URL. The scheme of that
// URL is irrelevant here — it only names the instance to trust; whether a
// credential may travel is decided by the remote's own scheme. Only genuinely
// empty configuration falls back to gitlab.com: resolving a self-hosted
// instance to the public host would send its token to gitlab.com origins.
func (gitlabForge) TrustedHost(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultHost
	}
	// config canonicalizes gitlab_base_url on load, so a scheme is normally
	// present. This repeats the prepend for callers that pass a raw value:
	// without a scheme url.Parse reads "gitlab.internal:8443" as a scheme, and
	// silently yielding no host would fall back to gitlab.com — the one outcome
	// a self-hosted token must never have.
	if !strings.Contains(baseURL, "://") {
		baseURL = "https://" + baseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// ParseRequestURL accepts any http(s) host with a path of the shape
// group/project/-/merge_requests/N, with any trailing path, query or fragment
// as copied from the browser, and returns "scheme://host" as the API base.
func (gitlabForge) ParseRequestURL(raw string) (string, int, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, "", fmt.Errorf("parsing --url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", 0, "", fmt.Errorf("--url must use http or https")
	}
	if u.Host == "" {
		return "", 0, "", fmt.Errorf("--url must include a GitLab host")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || parts[i+1] != "merge_requests" {
			continue
		}
		if i == 0 {
			return "", 0, "", fmt.Errorf("--url must include a GitLab project path before /-/merge_requests/")
		}
		mr, err := forge.ParseRequestID(parts[i+2], "merge request IID")
		if err != nil {
			return "", 0, "", err
		}
		return strings.Join(parts[:i], "/"), mr, u.Scheme + "://" + u.Host, nil
	}
	return "", 0, "", fmt.Errorf("--url must be a GitLab MR URL like https://gitlab.example.com/group/project/-/merge_requests/123")
}

// MatchesRemote claims a remote on the configured host, so a token is only
// ever offered to the server it belongs to.
func (gitlabForge) MatchesRemote(remoteURL, baseURL string) bool {
	return forge.SameHost(remoteURL, baseURL)
}

// GitCredentials uses the oauth2 user GitLab documents for token basic auth.
func (gitlabForge) GitCredentials(token string) string {
	if token == "" {
		return ""
	}
	return "oauth2:" + token
}

func (gitlabForge) NewSource(baseURL, token, assetBaseURL string) forge.Source {
	return NewAdapter(NewClient(baseURL, token), assetBaseURL)
}
