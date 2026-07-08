package serve

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// signatureTolerance bounds the clock skew accepted between the signed
// webhook-timestamp and now; deliveries outside it are rejected as replays.
const signatureTolerance = 5 * time.Minute

// Group is one configured GitLab group: its path prefix, credentials, and the
// API client built from its token. BotUserID is the token's user (0 when the
// startup lookup failed) and feeds the emoji-loop guard.
type Group struct {
	Path      string
	Token     string
	secret    []byte
	signKey   []byte
	Client    *gitlab.Client
	BotUserID int
}

// UsesSigning reports whether this group verifies webhooks via a GitLab signing
// token (HMAC-SHA256) rather than the plaintext secret token.
func (g *Group) UsesSigning() bool {
	return len(g.signKey) > 0
}

// CheckSecret compares a webhook's X-Gitlab-Token against the group secret in
// constant time.
func (g *Group) CheckSecret(token string) bool {
	return subtle.ConstantTimeCompare([]byte(token), g.secret) == 1
}

// CheckSignature verifies a GitLab signing-token delivery (Standard Webhooks):
// the signed content is "<id>.<timestamp>.<body>", the signature is
// HMAC-SHA256 keyed by the decoded signing token, base64-encoded and carried as
// one or more space-separated "v1,<sig>" entries. The timestamp must be within
// signatureTolerance of now to bound replays. All comparisons are constant time.
func (g *Group) CheckSignature(id, timestamp, header string, body []byte, now time.Time) bool {
	if id == "" || timestamp == "" || header == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if delta := now.Sub(time.Unix(ts, 0)); delta > signatureTolerance || delta < -signatureTolerance {
		return false
	}

	mac := hmac.New(sha256.New, g.signKey)
	mac.Write([]byte(id))
	mac.Write([]byte("."))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// The header is a space-separated list; each entry is "<version>,<sig>".
	// We only issue v1. Compare every candidate in constant time and OR the
	// results so a match anywhere passes without early-exit timing leaks.
	matched := false
	for entry := range strings.FieldsSeq(header) {
		version, sig, ok := strings.Cut(entry, ",")
		if !ok || version != "v1" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(sig), []byte(want)) == 1 {
			matched = true
		}
	}
	return matched
}

// GroupSet resolves a project's path_with_namespace to its configured group
// by longest-prefix match at a '/' boundary. Immutable after construction, so
// safe for concurrent use by handler and workers.
type GroupSet struct {
	groups []*Group
	botIDs map[int]bool
}

// NewGroupSet builds one Group per configured entry, ordered longest path
// first, and resolves each token's bot user ID via lookup (nil lookup skips
// resolution, e.g. in tests).
func NewGroupSet(ctx context.Context, cfgs []config.ServeGroup, baseURL string, lookup func(ctx context.Context, client *gitlab.Client) (int, error)) (*GroupSet, []error) {
	set := &GroupSet{botIDs: make(map[int]bool)}
	var warnings []error
	for _, cfg := range cfgs {
		group := &Group{
			Path:   strings.Trim(cfg.Path, "/"),
			Token:  cfg.Token,
			secret: []byte(cfg.WebhookSecret),
			Client: gitlab.NewClient(baseURL, cfg.Token),
		}
		if cfg.SigningToken != "" {
			// LoadServe already validated the format; decode defensively and
			// warn rather than crash if a caller bypassed validation. A group
			// with no usable credential rejects every webhook (fail closed).
			key, err := config.ParseSigningKey(cfg.SigningToken)
			if err != nil {
				warnings = append(warnings, err)
			} else {
				group.signKey = key
			}
		}
		if lookup != nil {
			id, err := lookup(ctx, group.Client)
			if err != nil {
				warnings = append(warnings, err)
			} else {
				group.BotUserID = id
				set.botIDs[id] = true
			}
		}
		set.groups = append(set.groups, group)
	}
	sort.SliceStable(set.groups, func(i, j int) bool {
		return len(set.groups[i].Path) > len(set.groups[j].Path)
	})
	return set, warnings
}

// BotIDs returns the set of resolved bot user IDs for the emoji-loop guard.
func (s *GroupSet) BotIDs() map[int]bool {
	return s.botIDs
}

// Match returns the group whose path is the longest prefix of the project's
// path_with_namespace, where the prefix must end at a '/' boundary
// ("platform/legacy" matches "platform/legacy/tool" but not
// "platform/legacy-x"). Nil when no group matches.
func (s *GroupSet) Match(pathWithNamespace string) *Group {
	project := strings.Trim(pathWithNamespace, "/")
	for _, group := range s.groups {
		if project == group.Path || strings.HasPrefix(project, group.Path+"/") {
			return group
		}
	}
	return nil
}
