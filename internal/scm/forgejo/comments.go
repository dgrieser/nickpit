package forgejo

import (
	"context"
	"fmt"
)

// IssueComment is the part of a created comment the daemon addresses reactions by.
type IssueComment struct {
	ID int `json:"id"`
}

// CreateIssueComment posts a top-level comment on a pull request (an issue, to
// this endpoint). With created == nil the response body is ignored, so a 2xx
// with an empty or non-JSON body still counts as posted and the publisher never
// retries (and duplicates) a comment the server committed.
func (c *Client) CreateIssueComment(ctx context.Context, repo string, index int, body string, created *IssueComment) error {
	// A typed nil in out is not == nil, so Post would still try to decode.
	var out any
	if created != nil {
		out = created
	}
	path := fmt.Sprintf("/repos/%s/issues/%d/comments", escapeRepo(repo), index)
	return c.Post(ctx, path, map[string]string{"body": body}, out)
}
