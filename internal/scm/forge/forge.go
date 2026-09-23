// Package forge defines the interface every code-hosting platform ("forge")
// implements so the CLI, the session picker and the git layer can address a
// pull or merge request without knowing which platform serves it. The
// interface deliberately depends on internal/model only: both scm packages
// implement it, and internal/config imports one of them, so anything heavier
// here would close an import cycle.
package forge

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

// Forge describes one platform and builds its review source. Every method is
// pure platform knowledge; credentials come in as plain strings so the
// implementations stay independent of internal/config.
type Forge interface {
	// Mode is the platform's ReviewMode. It is persisted in session files, so it
	// never changes once released.
	Mode() model.ReviewMode
	// Name is the platform's proper name, "GitHub" or "GitLab".
	Name() string
	// Command is the cobra parent command that groups the platform's
	// subcommands, "github" or "gitlab".
	Command() string
	// RequestCommand is the subcommand that reviews a request, "pr" or "mr".
	RequestCommand() string
	// RequestNoun is the platform's word for a change under review, "pull
	// request" or "merge request".
	RequestNoun() string
	// RequestAbbrev is the short form of RequestNoun, "PR" or "MR".
	RequestAbbrev() string
	// RequestSigil is the character the platform writes before a request
	// number, "#" or "!".
	RequestSigil() string
	// ConfigurableBaseURL reports whether the platform can be self-hosted, so
	// the API base URL is configuration: a --<mode>-base-url flag exists, the
	// host a session talked to is persisted, and a resumed session checks that
	// host against the active profile's before it sends the token.
	ConfigurableBaseURL() bool
	// NormalizeBaseURL canonicalizes a configured API base URL; the empty
	// string yields the platform's default. Every consumer of the URL must see
	// the same canonical form.
	NormalizeBaseURL(raw string) string
	// TrustedHost is the only host a token for baseURL may be sent to, in
	// lowercase. An origin URL is attacker controlled in a fork request, so the
	// match is exact: a broad configured host is never widened to subdomains.
	TrustedHost(baseURL string) string
	// ParseRequestURL reads a request's web URL into the repository path, the
	// request number and, for a self-hosted platform, the "scheme://host" API
	// base the URL names (empty where the host is fixed).
	ParseRequestURL(raw string) (repo string, id int, baseURL string, err error)
	// MatchesRemote reports whether a git remote URL points at this platform's
	// instance at baseURL, so a checkout can be attributed to a platform.
	MatchesRemote(remoteURL, baseURL string) bool
	// GitCredentials renders a token as the "user:password" pair git sends as
	// HTTP basic auth, or "" for an empty token.
	GitCredentials(token string) string
	// NewSource builds the review source for one API host and token.
	NewSource(baseURL, token, assetBaseURL string) Source
}

// Source is the review-path surface every platform adapter provides: resolve
// and check out a request, read files from its base, publish a review, list
// the repository's requests, and read published reviews back.
type Source interface {
	model.RemoteCheckoutSource
	model.BaseFileSource
	model.ReviewPublisher
	// ListOpenRequests lists the open requests of a repository, most recently
	// updated first.
	ListOpenRequests(ctx context.Context, repo string) ([]model.OpenRequest, error)
	// ListRequests is ListOpenRequests over every state, merged and closed too.
	ListRequests(ctx context.Context, repo string) ([]model.OpenRequest, error)
	// ReviewResults reassembles the reviews previously published to a request,
	// keyed by review id, trusting only comments the token's own user wrote.
	ReviewResults(ctx context.Context, repo string, id int) (map[string]*model.ReviewResult, error)
	// ReviewResultsAnyAuthor is ReviewResults without the author check, for a
	// read-only caller that has to see reviews this token did not publish.
	ReviewResultsAnyAuthor(ctx context.Context, repo string, id int) (map[string]*model.ReviewResult, error)
}

// Registry is the ordered set of known platforms. Order matters for Detect:
// a platform with a fixed host (GitHub) precedes those that claim whatever
// host they are configured with.
type Registry []Forge

// Lookup returns the platform serving mode.
func (r Registry) Lookup(mode model.ReviewMode) (Forge, bool) {
	for _, f := range r {
		if f.Mode() == mode {
			return f, true
		}
	}
	return nil, false
}

// Detect attributes a git remote to the first platform whose MatchesRemote
// accepts it, asking baseURL for each platform's configured API base.
func (r Registry) Detect(remoteURL string, baseURL func(Forge) string) (Forge, bool) {
	for _, f := range r {
		if f.MatchesRemote(remoteURL, baseURL(f)) {
			return f, true
		}
	}
	return nil, false
}

// RemoteHost is the host of a git remote URL, in either of the two shapes git
// writes: a URL with a scheme, or the scp-style "git@host:group/project.git".
func RemoteHost(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil {
			return ""
		}
		return parsed.Hostname()
	}
	before, _, ok := strings.Cut(remote, ":")
	if !ok {
		return ""
	}
	if _, host, found := strings.Cut(before, "@"); found {
		return host
	}
	return before
}

// SameHost reports whether a git remote and an API base URL name the same
// server, so a token is only ever offered to the host it belongs to. An
// unreadable remote counts as the same host: the git remote is then no
// evidence either way, and the configured host is what the rest of NickPit
// uses.
func SameHost(remote, baseURL string) bool {
	host := RemoteHost(remote)
	if host == "" {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	return strings.EqualFold(host, parsed.Hostname())
}

// ParseRequestID reads the request number out of a URL path segment, rejecting
// anything that is not a positive integer. label names the number in the error.
func ParseRequestID(raw, label string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("--url has invalid %s %q", label, raw)
	}
	return id, nil
}
