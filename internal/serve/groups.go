package serve

import (
	"context"
	"crypto/subtle"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// Group is one configured GitLab group: its path prefix, credentials, and the
// API client built from its token. BotUserID is the token's user (0 when the
// startup lookup failed) and feeds the emoji-loop guard.
type Group struct {
	Path      string
	Token     string
	secret    []byte
	Client    *gitlab.Client
	BotUserID int
}

// CheckSecret compares a webhook's X-Gitlab-Token against the group secret in
// constant time.
func (g *Group) CheckSecret(token string) bool {
	return subtle.ConstantTimeCompare([]byte(token), g.secret) == 1
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
