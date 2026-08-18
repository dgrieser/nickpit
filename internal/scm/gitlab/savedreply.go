package gitlab

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// GitLab's own validation for a saved reply (SavedReplyConcern): name and
// content are required, name is at most 255 characters and unique per owner,
// content is at most 10000. Checking locally turns a server-side 422 into a
// message naming the offending template.
const (
	savedReplyNameMaxLength    = 255
	savedReplyContentMaxLength = 10000
)

// maxSavedReplyPages bounds saved-reply pagination. An owner holding more than
// 100 pages (10k templates) means a broken cursor, not a real inventory.
const maxSavedReplyPages = 100

// SavedReplyScopeKind names an owner of comment templates ("saved replies" in
// the API): the token's own user, a project, or a group. GitLab offers the user
// and group scopes in the comment box's template picker, so a group-scoped
// template reaches every merge request in that group.
type SavedReplyScopeKind string

const (
	SavedReplyScopeUser    SavedReplyScopeKind = "user"
	SavedReplyScopeProject SavedReplyScopeKind = "project"
	SavedReplyScopeGroup   SavedReplyScopeKind = "group"
)

// SavedReplyScope addresses one owner of comment templates. Path is the full
// project or group path and must be empty for the user scope, which is always
// the token's own account.
type SavedReplyScope struct {
	Kind SavedReplyScopeKind
	Path string
}

// UserSavedReplyScope returns the scope of the token owner's own templates.
func UserSavedReplyScope() SavedReplyScope {
	return SavedReplyScope{Kind: SavedReplyScopeUser}
}

// ProjectSavedReplyScope returns the scope of a project's templates.
func ProjectSavedReplyScope(path string) SavedReplyScope {
	return SavedReplyScope{Kind: SavedReplyScopeProject, Path: path}
}

// GroupSavedReplyScope returns the scope of a group's templates.
func GroupSavedReplyScope(path string) SavedReplyScope {
	return SavedReplyScope{Kind: SavedReplyScopeGroup, Path: path}
}

func (s SavedReplyScope) String() string {
	if s.Path == "" {
		return string(s.Kind)
	}
	return string(s.Kind) + " " + s.Path
}

// SavedReply is one GitLab comment template. ID is the GraphQL global ID and is
// empty for a template that exists only as a desired state.
type SavedReply struct {
	ID      string
	Name    string
	Content string
}

// savedReplyDialect carries the GraphQL vocabulary of one scope: saved replies
// have a separate root field, id type, and mutation triple per owner kind.
type savedReplyDialect struct {
	// ownerField selects the owner; ownerArgs and ownerVars are its argument
	// list and the matching variable declarations (both empty for the user
	// scope, which takes no path).
	ownerField string
	ownerArgs  string
	ownerVars  string
	// ownerIDInput is the create-input field carrying the owner's global ID and
	// ownerIDType its GraphQL type. Both are empty for the user scope: its
	// create mutation derives the owner from the token.
	ownerIDInput string
	ownerIDType  string
	// replyIDType is the GraphQL type of a saved reply's global ID.
	replyIDType  string
	createField  string
	updateField  string
	destroyField string
}

func savedReplyDialectFor(scope SavedReplyScope) (savedReplyDialect, error) {
	switch scope.Kind {
	case SavedReplyScopeUser:
		if scope.Path != "" {
			return savedReplyDialect{}, fmt.Errorf("gitlab: user saved replies take no path (got %q)", scope.Path)
		}
		return savedReplyDialect{
			ownerField:   "currentUser",
			replyIDType:  "UsersSavedReplyID",
			createField:  "savedReplyCreate",
			updateField:  "savedReplyUpdate",
			destroyField: "savedReplyDestroy",
		}, nil
	case SavedReplyScopeProject:
		if scope.Path == "" {
			return savedReplyDialect{}, fmt.Errorf("gitlab: project saved replies need a project path")
		}
		return savedReplyDialect{
			ownerField:   "project",
			ownerArgs:    "(fullPath: $path)",
			ownerVars:    "$path: ID!, ",
			ownerIDInput: "projectId",
			ownerIDType:  "ProjectID!",
			replyIDType:  "ProjectsSavedReplyID",
			createField:  "projectSavedReplyCreate",
			updateField:  "projectSavedReplyUpdate",
			destroyField: "projectSavedReplyDestroy",
		}, nil
	case SavedReplyScopeGroup:
		if scope.Path == "" {
			return savedReplyDialect{}, fmt.Errorf("gitlab: group saved replies need a group path")
		}
		return savedReplyDialect{
			ownerField:   "group",
			ownerArgs:    "(fullPath: $path)",
			ownerVars:    "$path: ID!, ",
			ownerIDInput: "groupId",
			ownerIDType:  "GroupID!",
			replyIDType:  "GroupsSavedReplyID",
			createField:  "groupSavedReplyCreate",
			updateField:  "groupSavedReplyUpdate",
			destroyField: "groupSavedReplyDestroy",
		}, nil
	default:
		return savedReplyDialect{}, fmt.Errorf("gitlab: unknown saved reply scope %q", scope.Kind)
	}
}

// SavedReplies lists every comment template the scope owns.
func (c *Client) SavedReplies(ctx context.Context, scope SavedReplyScope) ([]SavedReply, error) {
	dialect, err := savedReplyDialectFor(scope)
	if err != nil {
		return nil, err
	}
	_, replies, err := c.savedReplyOwner(ctx, scope, dialect)
	return replies, err
}

// savedReplyOwner returns the scope owner's global ID together with all its
// templates, following the connection's cursor to the last page.
func (c *Client) savedReplyOwner(ctx context.Context, scope SavedReplyScope, dialect savedReplyDialect) (string, []SavedReply, error) {
	query := fmt.Sprintf(`query(%s$after: String) {
  owner: %s%s {
    id
    savedReplies(first: 100, after: $after) {
      nodes { id name content }
      pageInfo { hasNextPage endCursor }
    }
  }
}`, dialect.ownerVars, dialect.ownerField, dialect.ownerArgs)

	var replies []SavedReply
	ownerID := ""
	cursor := ""
	for page := 0; ; page++ {
		if page >= maxSavedReplyPages {
			return "", nil, fmt.Errorf("gitlab: listing saved replies for %s exceeded %d pages", scope, maxSavedReplyPages)
		}
		variables := map[string]any{}
		if scope.Path != "" {
			variables["path"] = scope.Path
		}
		if cursor != "" {
			variables["after"] = cursor
		}
		var response struct {
			Owner *struct {
				ID           string `json:"id"`
				SavedReplies struct {
					Nodes    []SavedReply `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"savedReplies"`
			} `json:"owner"`
		}
		if err := c.GraphQL(ctx, query, variables, &response); err != nil {
			return "", nil, fmt.Errorf("gitlab: listing saved replies for %s: %w", scope, err)
		}
		if response.Owner == nil {
			return "", nil, fmt.Errorf("gitlab: %s not found or not visible to the token", scope)
		}
		ownerID = response.Owner.ID
		replies = append(replies, response.Owner.SavedReplies.Nodes...)
		if !response.Owner.SavedReplies.PageInfo.HasNextPage || response.Owner.SavedReplies.PageInfo.EndCursor == "" {
			break
		}
		cursor = response.Owner.SavedReplies.PageInfo.EndCursor
	}
	return ownerID, replies, nil
}

// SavedReplySyncOptions describes the desired comment templates of one scope.
type SavedReplySyncOptions struct {
	// Desired is the template set to converge on, matched to existing templates
	// by name.
	Desired []SavedReply
	// Prefix marks the templates this sync owns. Only names carrying it are
	// eligible for pruning, so hand-written templates in the same scope survive.
	Prefix string
	// Prune deletes prefixed templates absent from Desired.
	Prune bool
	// DryRun reports the plan without writing anything.
	DryRun bool
}

// SavedReplySync reports what a sync did (or, in a dry run, would do), by
// template name. Created/Updated/Unchanged follow the desired order; Pruned is
// sorted.
type SavedReplySync struct {
	Created   []string
	Updated   []string
	Unchanged []string
	Pruned    []string
}

// SyncSavedReplies converges the scope's comment templates on opts.Desired:
// missing names are created, drifted content is updated, and prefixed templates
// no longer desired are deleted when opts.Prune is set. Templates outside the
// prefix are never touched.
func (c *Client) SyncSavedReplies(ctx context.Context, scope SavedReplyScope, opts SavedReplySyncOptions) (SavedReplySync, error) {
	dialect, err := savedReplyDialectFor(scope)
	if err != nil {
		return SavedReplySync{}, err
	}
	if err := validateDesiredSavedReplies(opts.Desired); err != nil {
		return SavedReplySync{}, err
	}
	ownerID, existing, err := c.savedReplyOwner(ctx, scope, dialect)
	if err != nil {
		return SavedReplySync{}, err
	}
	byName := make(map[string]SavedReply, len(existing))
	for _, reply := range existing {
		byName[reply.Name] = reply
	}

	var result SavedReplySync
	for _, want := range opts.Desired {
		current, ok := byName[want.Name]
		switch {
		case !ok:
			if !opts.DryRun {
				if err := c.createSavedReply(ctx, scope, dialect, ownerID, want); err != nil {
					return result, err
				}
			}
			result.Created = append(result.Created, want.Name)
		case current.Content != want.Content:
			if !opts.DryRun {
				if err := c.updateSavedReply(ctx, scope, dialect, current.ID, want); err != nil {
					return result, err
				}
			}
			result.Updated = append(result.Updated, want.Name)
		default:
			result.Unchanged = append(result.Unchanged, want.Name)
		}
	}

	if opts.Prune && opts.Prefix != "" {
		desiredNames := make(map[string]bool, len(opts.Desired))
		for _, want := range opts.Desired {
			desiredNames[want.Name] = true
		}
		for _, reply := range existing {
			if desiredNames[reply.Name] || !strings.HasPrefix(reply.Name, opts.Prefix) {
				continue
			}
			if !opts.DryRun {
				if err := c.destroySavedReply(ctx, scope, dialect, reply.ID); err != nil {
					return result, err
				}
			}
			result.Pruned = append(result.Pruned, reply.Name)
		}
		slices.Sort(result.Pruned)
	}
	return result, nil
}

func validateDesiredSavedReplies(desired []SavedReply) error {
	seen := make(map[string]bool, len(desired))
	for _, reply := range desired {
		switch {
		case strings.TrimSpace(reply.Name) == "":
			return fmt.Errorf("gitlab: saved reply name must not be empty")
		case strings.TrimSpace(reply.Content) == "":
			return fmt.Errorf("gitlab: saved reply %q has empty content", reply.Name)
		case len(reply.Name) > savedReplyNameMaxLength:
			return fmt.Errorf("gitlab: saved reply name %q exceeds %d characters", reply.Name, savedReplyNameMaxLength)
		case len(reply.Content) > savedReplyContentMaxLength:
			return fmt.Errorf("gitlab: saved reply %q content exceeds %d characters", reply.Name, savedReplyContentMaxLength)
		case seen[reply.Name]:
			return fmt.Errorf("gitlab: duplicate saved reply name %q", reply.Name)
		}
		seen[reply.Name] = true
	}
	return nil
}

func (c *Client) createSavedReply(ctx context.Context, scope SavedReplyScope, dialect savedReplyDialect, ownerID string, reply SavedReply) error {
	variables := map[string]any{"name": reply.Name, "content": reply.Content}
	ownerVar, ownerInput := "", ""
	if dialect.ownerIDInput != "" {
		if ownerID == "" {
			return fmt.Errorf("gitlab: %s returned no id, cannot create saved reply %q", scope, reply.Name)
		}
		ownerVar = fmt.Sprintf("$ownerId: %s, ", dialect.ownerIDType)
		ownerInput = fmt.Sprintf("%s: $ownerId, ", dialect.ownerIDInput)
		variables["ownerId"] = ownerID
	}
	query := fmt.Sprintf(`mutation(%s$name: String!, $content: String!) {
  result: %s(input: {%sname: $name, content: $content}) {
    errors
  }
}`, ownerVar, dialect.createField, ownerInput)
	if err := c.savedReplyMutation(ctx, query, variables); err != nil {
		return fmt.Errorf("gitlab: creating saved reply %q in %s: %w", reply.Name, scope, err)
	}
	return nil
}

func (c *Client) updateSavedReply(ctx context.Context, scope SavedReplyScope, dialect savedReplyDialect, id string, reply SavedReply) error {
	query := fmt.Sprintf(`mutation($id: %s!, $name: String!, $content: String!) {
  result: %s(input: {id: $id, name: $name, content: $content}) {
    errors
  }
}`, dialect.replyIDType, dialect.updateField)
	variables := map[string]any{"id": id, "name": reply.Name, "content": reply.Content}
	if err := c.savedReplyMutation(ctx, query, variables); err != nil {
		return fmt.Errorf("gitlab: updating saved reply %q in %s: %w", reply.Name, scope, err)
	}
	return nil
}

func (c *Client) destroySavedReply(ctx context.Context, scope SavedReplyScope, dialect savedReplyDialect, id string) error {
	query := fmt.Sprintf(`mutation($id: %s!) {
  result: %s(input: {id: $id}) {
    errors
  }
}`, dialect.replyIDType, dialect.destroyField)
	if err := c.savedReplyMutation(ctx, query, map[string]any{"id": id}); err != nil {
		return fmt.Errorf("gitlab: deleting saved reply in %s: %w", scope, err)
	}
	return nil
}

// savedReplyMutation runs one mutation and reports its payload errors. GitLab
// answers a rejected mutation with HTTP 200 and a non-empty "errors" list
// inside the payload, so that list has to be checked separately from the
// transport-level GraphQL errors.
func (c *Client) savedReplyMutation(ctx context.Context, query string, variables map[string]any) error {
	var response struct {
		Result struct {
			Errors []string `json:"errors"`
		} `json:"result"`
	}
	if err := c.GraphQL(ctx, query, variables, &response); err != nil {
		return err
	}
	if len(response.Result.Errors) > 0 {
		return fmt.Errorf("%s", strings.Join(response.Result.Errors, "; "))
	}
	return nil
}
