package gitlab

import (
	"context"
	"fmt"
)

// Project is the subset of the GitLab project API used by the serve daemon:
// topics drive the auto-review opt-in check.
type Project struct {
	ID                int      `json:"id"`
	PathWithNamespace string   `json:"path_with_namespace"`
	DefaultBranch     string   `json:"default_branch"`
	Topics            []string `json:"topics"`
}

// GetProject fetches a project by numeric ID. The serve daemon uses numeric IDs
// (from webhook payloads) so project path renames cannot 404 mid-flight.
func (c *Client) GetProject(ctx context.Context, projectID int) (*Project, error) {
	var project Project
	if err := c.Get(ctx, fmt.Sprintf("/projects/%d", projectID), &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// User is the authenticated token owner, used to identify the daemon's bot
// user so its own award-emoji events can be ignored.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// CurrentUser returns the user the client's token authenticates as. The answer
// is memoized for the client's lifetime: it identifies the token, which cannot
// change under it, and callers that verify carrier authorship ask once per
// request they read.
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	c.userMu.Lock()
	defer c.userMu.Unlock()
	if c.user != nil {
		return c.user, nil
	}
	var user User
	if err := c.Get(ctx, "/user", &user); err != nil {
		return nil, err
	}
	c.user = &user
	return c.user, nil
}
