package forgejo

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// Reaction is one emoji reaction on a pull request or comment. The user id
// lets callers pick out their own: Forgejo only ever removes the reactions of
// the token's user. A reaction migrated from another forge has no local user
// and reports id 0, a deleted account's reports -1.
type Reaction struct {
	Content   string
	UserID    int
	UserLogin string
}

type reactionResponse struct {
	Content string `json:"content"`
	User    struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"user"`
}

// IssueReactions lists all reactions currently present on a pull request (an
// issue, to this endpoint).
func (c *Client) IssueReactions(ctx context.Context, repo string, index int) ([]Reaction, error) {
	return c.listReactions(ctx, issueReactionsPath(repo, index))
}

// CommentReactions lists all reactions currently present on one comment.
func (c *Client) CommentReactions(ctx context.Context, repo string, commentID int) ([]Reaction, error) {
	return c.listReactions(ctx, commentReactionsPath(repo, commentID))
}

// AddIssueReaction reacts to a pull request as the token's user.
func (c *Client) AddIssueReaction(ctx context.Context, repo string, index int, content string) error {
	return c.addReaction(ctx, issueReactionsPath(repo, index), content)
}

// AddCommentReaction reacts to a comment as the token's user.
func (c *Client) AddCommentReaction(ctx context.Context, repo string, commentID int, content string) error {
	return c.addReaction(ctx, commentReactionsPath(repo, commentID), content)
}

// RemoveIssueReaction takes the token user's own reaction off a pull request.
func (c *Client) RemoveIssueReaction(ctx context.Context, repo string, index int, content string) error {
	return c.removeReaction(ctx, issueReactionsPath(repo, index), content)
}

// RemoveCommentReaction takes the token user's own reaction off a comment.
func (c *Client) RemoveCommentReaction(ctx context.Context, repo string, commentID int, content string) error {
	return c.removeReaction(ctx, commentReactionsPath(repo, commentID), content)
}

// ReplaceOwnIssueReaction adds add and removes every other reaction owned by
// userID on the pull request, except explicitly kept names. It is intended for
// targets where this dedicated bot owns all status reactions, so outcomes left
// by older configurations are cleaned up too. An empty add only removes.
// userID is the token's own user id and is REQUIRED: without it no listed
// reaction can be told to be the bot's, and adding the outcome without
// removing the old marker would leave contradictory reactions behind, so the
// whole replacement is refused.
func (c *Client) ReplaceOwnIssueReaction(ctx context.Context, repo string, index, userID int, add string, keep ...string) error {
	return c.replaceOwnReaction(ctx, issueReactionsPath(repo, index), userID, add, keep)
}

// ReplaceOwnCommentReaction is ReplaceOwnIssueReaction for a comment.
func (c *Client) ReplaceOwnCommentReaction(ctx context.Context, repo string, commentID, userID int, add string, keep ...string) error {
	return c.replaceOwnReaction(ctx, commentReactionsPath(repo, commentID), userID, add, keep)
}

// AllowedReactions returns the reaction names the instance accepts (its
// [ui] REACTIONS setting: +1, -1, laugh, hooray, confused, heart, rocket and
// eyes unless the operator changed it). Adding any other is answered with 403.
func (c *Client) AllowedReactions(ctx context.Context) ([]string, error) {
	var settings struct {
		AllowedReactions []string `json:"allowed_reactions"`
	}
	if err := c.Get(ctx, "/settings/ui", &settings); err != nil {
		return nil, err
	}
	return settings.AllowedReactions, nil
}

func issueReactionsPath(repo string, index int) string {
	return fmt.Sprintf("/repos/%s/issues/%d/reactions", escapeRepo(repo), index)
}

// commentReactionsPath needs no pull request index: comment ids are unique
// within the repository.
func commentReactionsPath(repo string, commentID int) string {
	return fmt.Sprintf("/repos/%s/issues/comments/%d/reactions", escapeRepo(repo), commentID)
}

// listReactions reads every reaction of one target. Both listings come whole
// in one response (the issue one pages only when asked for a page, the comment
// one never does); GetPaginated merely follows a Link header should a server
// send one.
func (c *Client) listReactions(ctx context.Context, path string) ([]Reaction, error) {
	var raw []reactionResponse
	if err := c.GetPaginated(ctx, path, &raw); err != nil {
		return nil, err
	}
	reactions := make([]Reaction, 0, len(raw))
	for _, item := range raw {
		reactions = append(reactions, Reaction{Content: item.Content, UserID: item.User.ID, UserLogin: item.User.Login})
	}
	return reactions, nil
}

// errEmptyReaction refuses a request the server would misread: a removal
// without a content matches every reaction the user left on the target.
var errEmptyReaction = errors.New("forgejo: empty reaction content")

// addReaction posts one reaction. A repeated reaction by the same user is
// answered with 200 and the existing one, so a double add is a success without
// special casing. A 403 (a name the instance does not allow, a locked
// conversation, a blocked user) reaches the caller.
func (c *Client) addReaction(ctx context.Context, path, content string) error {
	if content == "" {
		return errEmptyReaction
	}
	return c.Post(ctx, path, map[string]string{"content": content}, nil)
}

// removeReaction deletes the token user's own reaction of that name, which the
// request body carries. Forgejo answers 200 whether or not there was one.
func (c *Client) removeReaction(ctx context.Context, path, content string) error {
	if content == "" {
		return errEmptyReaction
	}
	return c.DeleteJSON(ctx, path, map[string]string{"content": content})
}

// replaceOwnReaction confirms add, then removes the other reactions owned by
// userID. A failed add preserves old status markers; a failed list still tries
// the informative add and returns the list error to the caller.
func (c *Client) replaceOwnReaction(ctx context.Context, path string, userID int, add string, keep []string) error {
	if userID == 0 {
		return errors.New("forgejo: refusing to replace reactions: own user id unresolved")
	}
	var errs []error
	reactions, err := c.listReactions(ctx, path)
	if err != nil {
		errs = append(errs, fmt.Errorf("forgejo: listing reactions: %w", err))
	}
	if add != "" {
		if err := c.addReaction(ctx, path, add); err != nil {
			errs = append(errs, err)
			return errors.Join(errs...)
		}
	}
	for _, reaction := range reactions {
		if reaction.UserID != userID || reaction.Content == add || slices.Contains(keep, reaction.Content) {
			continue
		}
		// A 404 means the target itself is gone, and its reactions with it. A
		// 403 must surface: the reaction is the bot's own and stays live.
		if err := c.removeReaction(ctx, path, reaction.Content); err != nil && !IsNotFound(err) {
			errs = append(errs, fmt.Errorf("forgejo: removing reaction %q: %w", reaction.Content, err))
		}
	}
	return errors.Join(errs...)
}
