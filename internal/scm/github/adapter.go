package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

type Adapter struct {
	client *Client
	// render builds the platform-neutral markdown comment bodies; it carries
	// the badge host (normalized by reviewmd.NewRenderer).
	render reviewmd.Renderer
}

func NewAdapter(client *Client, assetBaseURL string) *Adapter {
	return &Adapter{client: client, render: reviewmd.NewRenderer(assetBaseURL)}
}

func (a *Adapter) ResolveContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	return a.client.FetchPR(ctx, req.Repo, req.Identifier, req.IncludeComments)
}

func (a *Adapter) ResolveCheckout(ctx context.Context, req model.ReviewRequest) (*model.CheckoutSpec, error) {
	return a.client.FetchPRCheckout(ctx, req.Repo, req.Identifier)
}

// ReviewResults reassembles the complete ReviewResults previously published to
// a PR, keyed by review id, from the hidden carrier markers on its review
// bodies, inline review comments, and issue comments. All three surfaces are
// read because a publish spreads the markers across them: the review body
// carries the summary envelope, inline comments carry the findings anchored in
// the diff, and issue comments carry the rest plus the oversized-finding
// fallback carriers.
//
// Carrier markers are only encoded, not authenticated, so any commenter could
// forge one; to keep attacker-controlled findings out of the output, only
// markers in comments authored by the client token's own user (the bot that
// published the review) are trusted — the same restriction the GitLab twin
// makes. A fetch failure is returned rather than tolerated: reassembly rejects
// a review whose declared finding count has not landed, so a partial read
// would silently look like "no review here".
//
// It returns an empty map when the PR carries no trusted nickpit review markers
// (reviewed before carrier markers existed, or published by a different user
// than this token's).
func (a *Adapter) ReviewResults(ctx context.Context, repo string, number int) (map[string]*model.ReviewResult, error) {
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: resolving token user for carrier verification: %w", err)
	}
	escaped := escapeRepo(repo)
	var bodies []string

	var reviews []reviewResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", escaped, number), &reviews); err != nil {
		return nil, err
	}
	for _, review := range reviews {
		if ownedBy(review.User, user) {
			bodies = append(bodies, review.Body)
		}
	}
	var comments []commentResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", escaped, number), &comments); err != nil {
		return nil, err
	}
	for _, comment := range comments {
		if ownedBy(comment.User, user) {
			bodies = append(bodies, comment.Body)
		}
	}
	var issueComments []issueCommentResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", escaped, number), &issueComments); err != nil {
		return nil, err
	}
	for _, comment := range issueComments {
		if ownedBy(comment.User, user) {
			bodies = append(bodies, comment.Body)
		}
	}
	return reviewmd.ReviewResultsByID(bodies), nil
}

// ownedBy reports whether a comment was authored by the token owner. GitHub
// logins are case-insensitive, so the comparison folds case; an empty author
// (a comment whose user was deleted, or a payload without the field) never
// matches, so its markers stay untrusted.
func ownedBy(author userRef, user *User) bool {
	return author.Login != "" && strings.EqualFold(author.Login, user.Login)
}
