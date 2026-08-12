package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
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
// the whole replacement is refused. Awarding add without removing the old
// marker would leave contradictory reactions behind.
func (c *Client) ReplaceMREmoji(ctx context.Context, projectID, iid, userID int, add string, remove ...string) error {
	return c.replaceEmoji(ctx, mrEmojiPath(projectID, iid), userID, add, remove)
}

// ReplaceNoteEmoji is ReplaceMREmoji for a merge request note.
func (c *Client) ReplaceNoteEmoji(ctx context.Context, projectID, iid, noteID, userID int, add string, remove ...string) error {
	return c.replaceEmoji(ctx, noteEmojiPath(projectID, iid, noteID), userID, add, remove)
}

// ReplaceOwnMREmoji awards add and revokes every other reaction owned by
// userID on the merge request, except explicitly kept names. It is intended
// for awardables where this dedicated bot owns all status reactions; unlike a
// name-based replacement it also cleans outcomes left by older configurations.
func (c *Client) ReplaceOwnMREmoji(ctx context.Context, projectID, iid, userID int, add string, keep ...string) error {
	return c.replaceOwnEmoji(ctx, mrEmojiPath(projectID, iid), userID, add, keep)
}

// ReplaceOwnNoteEmoji is ReplaceOwnMREmoji for a merge request note.
func (c *Client) ReplaceOwnNoteEmoji(ctx context.Context, projectID, iid, noteID, userID int, add string, keep ...string) error {
	return c.replaceOwnEmoji(ctx, noteEmojiPath(projectID, iid, noteID), userID, add, keep)
}

func mrEmojiPath(projectID, iid int) string {
	return fmt.Sprintf("/projects/%d/merge_requests/%d/award_emoji", projectID, iid)
}

func noteEmojiPath(projectID, iid, noteID int) string {
	return fmt.Sprintf("/projects/%d/merge_requests/%d/notes/%d/award_emoji", projectID, iid, noteID)
}

// awardEmoji posts one award. GitLab rejects double-awards by the same user,
// with a status that varies across versions, so only the canonical duplicate
// validation response is treated as success. Other client errors can mean the
// emoji is invalid or the awardable vanished and must reach the caller.
func (c *Client) awardEmoji(ctx context.Context, basePath, name string) error {
	err := c.Post(ctx, basePath, map[string]string{"name": name}, nil)
	if isDuplicateAwardError(err) {
		return nil
	}
	return err
}

func isDuplicateAwardError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status < 400 || apiErr.Status >= 500 {
		return false
	}

	var response struct {
		Message json.RawMessage `json:"message"`
	}
	if json.Unmarshal([]byte(apiErr.Body), &response) != nil {
		return false
	}

	var message string
	if json.Unmarshal(response.Message, &message) != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(message), "Award Emoji Name has already been taken")
}

// replaceEmoji finds the named awards, confirms add, then revokes the old
// awards. A failed add leaves the old status marker intact. The add still
// happens after a list failure because the new reaction is the informative
// half, and every error is joined so the caller can log what went wrong. The
// list request is skipped entirely when there is nothing to revoke.
func (c *Client) replaceEmoji(ctx context.Context, basePath string, userID int, add string, remove []string) error {
	wanted := slices.DeleteFunc(slices.Clone(remove), func(name string) bool { return name == "" })
	if len(wanted) > 0 && userID == 0 {
		// Matching on the name alone is not safe: an administrator or owner
		// token CAN delete another user's award, so a human's genuine reaction
		// of the same name would be revoked.
		return errors.New("gitlab: refusing to replace award emoji: own user id unresolved")
	}

	return c.replaceEmojiWhere(ctx, basePath, userID, add, len(wanted) > 0, func(award AwardEmoji) bool {
		return slices.Contains(wanted, award.Name)
	})
}

func (c *Client) replaceOwnEmoji(ctx context.Context, basePath string, userID int, add string, keep []string) error {
	if userID == 0 {
		return errors.New("gitlab: refusing to replace award emoji: own user id unresolved")
	}
	protected := append(slices.Clone(keep), add)
	return c.replaceEmojiWhere(ctx, basePath, userID, add, true, func(award AwardEmoji) bool {
		return !slices.Contains(protected, award.Name)
	})
}

// replaceEmojiWhere confirms add, then revokes matching awards owned by
// userID. A failed add preserves old status markers; a failed list still tries
// the informative add and returns the list error to the caller.
func (c *Client) replaceEmojiWhere(ctx context.Context, basePath string, userID int, add string, list bool, remove func(AwardEmoji) bool) error {
	var errs []error
	var awards []AwardEmoji
	if list {
		if err := c.GetPaginated(ctx, basePath, &awards); err != nil {
			errs = append(errs, fmt.Errorf("gitlab: listing award emoji: %w", err))
		}
	}
	if add != "" {
		if err := c.awardEmoji(ctx, basePath, add); err != nil {
			errs = append(errs, err)
			return errors.Join(errs...)
		}
	}
	for _, award := range awards {
		if award.Name == add || !remove(award) {
			continue
		}
		if award.User.ID != userID {
			continue
		}
		if err := c.revokeEmoji(ctx, basePath, award.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// revokeEmoji deletes one award. A 404 means it is already gone. A 403 must
// surface: replaceEmojiWhere already selected an award owned by this user, so
// permission refusal can leave a managed reaction live.
func (c *Client) revokeEmoji(ctx context.Context, basePath string, awardID int) error {
	err := c.Delete(ctx, fmt.Sprintf("%s/%d", basePath, awardID))
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == 404 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("gitlab: revoking award emoji %d: %w", awardID, err)
	}
	return nil
}
