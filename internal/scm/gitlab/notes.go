package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

// AwardNoteEmoji awards an emoji reaction on a merge request note. Any 4xx
// response is treated as success, same policy as AwardMREmoji: GitLab rejects
// double-awards, and a missing acknowledgement must never fail the command
// that triggered it.
func (c *Client) AwardNoteEmoji(ctx context.Context, projectID, iid, noteID int, name string) error {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes/%d/award_emoji", projectID, iid, noteID)
	err := c.Post(ctx, path, map[string]string{"name": name}, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		return nil
	}
	return err
}

// CreateMRNote posts a top-level comment on a merge request.
func (c *Client) CreateMRNote(ctx context.Context, projectID, iid int, body string) error {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes", projectID, iid)
	return c.Post(ctx, path, map[string]string{"body": body}, nil)
}

// ReplyToMRDiscussion adds a note to an existing merge request discussion so
// command replies land threaded under the comment that issued them.
func (c *Client) ReplyToMRDiscussion(ctx context.Context, projectID, iid int, discussionID, body string) error {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/discussions/%s/notes", projectID, iid, url.PathEscape(discussionID))
	return c.Post(ctx, path, map[string]string{"body": body}, nil)
}
