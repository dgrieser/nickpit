package gitlab

import (
	"context"
	"errors"
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

// CurrentUser returns the user the client's token authenticates as.
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	var user User
	if err := c.Get(ctx, "/user", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// AwardMREmoji awards an emoji reaction on a merge request. Any 4xx response
// is treated as success: GitLab rejects double-awards by the same user (and
// the exact status varies across versions), and a missing award must never
// fail the review that triggered it.
func (c *Client) AwardMREmoji(ctx context.Context, projectID, iid int, name string) error {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/award_emoji", projectID, iid)
	err := c.Post(ctx, path, map[string]string{"name": name}, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		return nil
	}
	return err
}
