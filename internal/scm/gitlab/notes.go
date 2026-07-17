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

// MRNoteBodies returns the body of every note and discussion note on a merge
// request. It reads both the notes and discussions endpoints (a note appears in
// both on GitLab); callers de-duplicate on the decoded content. project accepts a
// numeric id or a group/name path.
func (c *Client) MRNoteBodies(ctx context.Context, project string, iid int) ([]string, error) {
	escaped := escapeProject(project)
	var bodies []string
	var notes []struct {
		Body string `json:"body"`
	}
	if err := c.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/notes", escaped, iid), &notes); err != nil {
		return nil, fmt.Errorf("gitlab: listing MR notes: %w", err)
	}
	for _, note := range notes {
		bodies = append(bodies, note.Body)
	}
	var discussions discussionsResponse
	if err := c.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", escaped, iid), &discussions); err != nil {
		return nil, fmt.Errorf("gitlab: listing MR discussions: %w", err)
	}
	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			bodies = append(bodies, note.Body)
		}
	}
	return bodies, nil
}

// DiscussionNoteBodies returns the bodies of the notes in a single discussion, in
// order (oldest first). It powers reading back an existing chat thread.
func (c *Client) DiscussionNoteBodies(ctx context.Context, project string, iid int, discussionID string) ([]DiscussionNote, error) {
	escaped := escapeProject(project)
	var discussion struct {
		Notes []struct {
			Body   string `json:"body"`
			System bool   `json:"system"`
			Author struct {
				Username string `json:"username"`
				ID       int    `json:"id"`
			} `json:"author"`
		} `json:"notes"`
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/discussions/%s", escaped, iid, url.PathEscape(discussionID))
	if err := c.Get(ctx, path, &discussion); err != nil {
		return nil, fmt.Errorf("gitlab: reading discussion: %w", err)
	}
	notes := make([]DiscussionNote, 0, len(discussion.Notes))
	for _, note := range discussion.Notes {
		notes = append(notes, DiscussionNote{
			Body:       note.Body,
			System:     note.System,
			AuthorName: note.Author.Username,
			AuthorID:   note.Author.ID,
		})
	}
	return notes, nil
}

// DiscussionNote is one note within a discussion thread.
type DiscussionNote struct {
	Body       string
	System     bool
	AuthorName string
	AuthorID   int
}
