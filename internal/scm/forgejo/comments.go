package forgejo

import (
	"context"
	"fmt"
)

// CreateIssueComment posts a top-level comment on a pull request (an issue, to
// this endpoint) and returns the new comment's id, the handle its reactions
// are addressed by.
func (c *Client) CreateIssueComment(ctx context.Context, repo string, index int, body string) (commentID int, err error) {
	var created struct {
		ID int `json:"id"`
	}
	path := fmt.Sprintf("/repos/%s/issues/%d/comments", escapeRepo(repo), index)
	if err := c.Post(ctx, path, map[string]string{"body": body}, &created); err != nil {
		return 0, err
	}
	return created.ID, nil
}
