package gitlab

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// AwardEmoji is one emoji reaction on a merge request or note. The user id lets
// callers pick out their own awards: GitLab only permits revoking those.
type AwardEmoji struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	User struct {
		ID int `json:"id"`
	} `json:"user"`
}

// AwardMREmoji awards an emoji reaction on a merge request.
func (c *Client) AwardMREmoji(ctx context.Context, projectID, iid int, name string) error {
	return c.awardEmoji(ctx, mrEmojiPath(projectID, iid), name)
}

// AwardNoteEmoji awards an emoji reaction on a merge request note.
func (c *Client) AwardNoteEmoji(ctx context.Context, projectID, iid, noteID int, name string) error {
	return c.awardEmoji(ctx, noteEmojiPath(projectID, iid, noteID), name)
}

// ReplaceMREmoji awards add on a merge request and revokes every remove name the
// client's own user awarded there, so a reaction can be flipped to a new one
// (e.g. the in-progress marker to the review outcome). An empty add only
// revokes; empty remove names are ignored. userID is the client's own user id
// and is REQUIRED for revoking: with 0 (unknown) only the name would be left to
// match, and an administrator/owner token CAN delete another user's award — so
// the revoke is refused (reported as an error) and only add is awarded.
func (c *Client) ReplaceMREmoji(ctx context.Context, projectID, iid, userID int, add string, remove ...string) error {
	return c.replaceEmoji(ctx, mrEmojiPath(projectID, iid), userID, add, remove)
}

// ReplaceNoteEmoji is ReplaceMREmoji for a merge request note.
func (c *Client) ReplaceNoteEmoji(ctx context.Context, projectID, iid, noteID, userID int, add string, remove ...string) error {
	return c.replaceEmoji(ctx, noteEmojiPath(projectID, iid, noteID), userID, add, remove)
}

func mrEmojiPath(projectID, iid int) string {
	return fmt.Sprintf("/projects/%d/merge_requests/%d/award_emoji", projectID, iid)
}

func noteEmojiPath(projectID, iid, noteID int) string {
	return fmt.Sprintf("/projects/%d/merge_requests/%d/notes/%d/award_emoji", projectID, iid, noteID)
}

// awardEmoji posts one award. A 4xx response is treated as success — GitLab
// rejects double-awards by the same user, and the exact status varies across
// versions — EXCEPT the ones that mean the award never happened for reasons a
// caller must get to log: 401/403 (a token that lost access) and 429 (rate
// limited). Swallowing those would leave e.g. a replace with a successful
// revoke but a silently dropped award: no reaction at all, and no trace.
func (c *Client) awardEmoji(ctx context.Context, basePath, name string) error {
	err := c.Post(ctx, basePath, map[string]string{"name": name}, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		switch apiErr.Status {
		case 401, 403, 429:
			return err
		}
		return nil
	}
	return err
}

// replaceEmoji revokes the named awards, then awards add. The award happens even
// when a revoke failed — the new reaction is the informative half — and every
// error is joined so the caller can log what went wrong. The list request is
// skipped entirely when there is nothing to revoke.
func (c *Client) replaceEmoji(ctx context.Context, basePath string, userID int, add string, remove []string) error {
	var errs []error
	wanted := slices.DeleteFunc(slices.Clone(remove), func(name string) bool { return name == "" })
	switch {
	case len(wanted) > 0 && userID == 0:
		// Matching on the name alone is not safe: an administrator or owner
		// token CAN delete another user's award, so a human's genuine reaction
		// of the same name would be revoked.
		errs = append(errs, errors.New("gitlab: refusing to revoke award emoji by name alone: own user id unresolved"))
	case len(wanted) > 0:
		var awards []AwardEmoji
		if err := c.GetPaginated(ctx, basePath, &awards); err != nil {
			errs = append(errs, fmt.Errorf("gitlab: listing award emoji: %w", err))
		}
		for _, award := range awards {
			if !slices.Contains(wanted, award.Name) {
				continue
			}
			if award.User.ID != userID {
				continue
			}
			if err := c.revokeEmoji(ctx, basePath, award.ID); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if add != "" {
		if err := c.awardEmoji(ctx, basePath, add); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// revokeEmoji deletes one award. 403 (not ours) and 404 (already gone) are
// treated as success: both mean there is nothing left to revoke.
func (c *Client) revokeEmoji(ctx context.Context, basePath string, awardID int) error {
	err := c.Delete(ctx, fmt.Sprintf("%s/%d", basePath, awardID))
	var apiErr *APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 403 || apiErr.Status == 404) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("gitlab: revoking award emoji %d: %w", awardID, err)
	}
	return nil
}
