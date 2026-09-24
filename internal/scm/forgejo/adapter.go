package forgejo

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/forge"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// Adapter is the review source for one Forgejo instance.
type Adapter struct {
	client *Client
	// render builds the platform-neutral markdown comment bodies; it carries
	// the badge host (normalized by reviewmd.NewRenderer).
	render reviewmd.Renderer
}

var _ forge.Source = (*Adapter)(nil)

func NewAdapter(client *Client, assetBaseURL string) *Adapter {
	return &Adapter{client: client, render: reviewmd.NewRenderer(assetBaseURL)}
}

func (a *Adapter) ResolveContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	return a.client.FetchPR(ctx, req.Repo, req.Identifier, req.IncludeComments)
}

func (a *Adapter) ResolveCheckout(ctx context.Context, req model.ReviewRequest) (*model.CheckoutSpec, error) {
	return a.client.FetchPRCheckout(ctx, req.Repo, req.Identifier)
}

// ListOpenRequests implements forge.Source over ListOpenPRs.
func (a *Adapter) ListOpenRequests(ctx context.Context, repo string) ([]model.OpenRequest, error) {
	return a.client.ListOpenPRs(ctx, repo)
}

// ListRequests implements forge.Source over ListPRs.
func (a *Adapter) ListRequests(ctx context.Context, repo string) ([]model.OpenRequest, error) {
	return a.client.ListPRs(ctx, repo)
}

// ReviewResults reassembles the complete ReviewResults previously published to
// a PR, keyed by review id, from the hidden carrier markers on its review
// bodies, inline review comments, and issue comments. All three surfaces are
// read because a publish spreads the markers across them.
//
// Carrier markers are only encoded, not authenticated, so any commenter could
// forge one; to keep attacker-controlled findings out of the output, only
// markers in comments authored by the client token's own user (the bot that
// published the review) are trusted. A fetch failure is returned rather than
// tolerated: reassembly rejects a review whose declared finding count has not
// landed, so a partial read would silently look like "no review here".
func (a *Adapter) ReviewResults(ctx context.Context, repo string, number int) (map[string]*model.ReviewResult, error) {
	return a.reviewResults(ctx, repo, number, true)
}

// ReviewResultsAnyAuthor reads the same markers without the author check, for a
// reader that has to see reviews this token did not publish — forgeable, so
// READ-ONLY use only.
func (a *Adapter) ReviewResultsAnyAuthor(ctx context.Context, repo string, number int) (map[string]*model.ReviewResult, error) {
	return a.reviewResults(ctx, repo, number, false)
}

func (a *Adapter) reviewResults(ctx context.Context, repo string, number int, trustedOnly bool) (map[string]*model.ReviewResult, error) {
	var user *User
	if trustedOnly {
		var err error
		if user, err = a.client.CurrentUser(ctx); err != nil {
			return nil, fmt.Errorf("forgejo: resolving token user for carrier verification: %w", err)
		}
	}
	mine := func(author userRef) bool { return !trustedOnly || ownedBy(author, user) }
	found, err := a.client.fetchSurfaces(ctx, repo, number, false)
	if err != nil {
		return nil, err
	}
	var bodies []string
	for _, review := range found.reviews {
		if mine(review.User) {
			bodies = append(bodies, review.Body)
		}
	}
	for _, comment := range found.inline {
		if mine(comment.User) {
			bodies = append(bodies, comment.Body)
		}
	}
	for _, comment := range found.comments {
		if mine(comment.User) {
			bodies = append(bodies, comment.Body)
		}
	}
	return reviewmd.ReviewResultsByID(bodies), nil
}

// ownedBy reports whether a comment was authored by the token owner. Logins
// are case-insensitive, so the comparison folds case; an empty author (a
// comment whose user was deleted, or a payload without the field) never
// matches, so its markers stay untrusted.
func ownedBy(author userRef, user *User) bool {
	return author.Login != "" && strings.EqualFold(author.Login, user.Login)
}

// ReadBaseFile implements model.BaseFileSource, reading from the pull request's
// base repository at the base commit so a fork cannot control the content.
func (a *Adapter) ReadBaseFile(ctx context.Context, req model.ReviewRequest, path string) ([]byte, bool, error) {
	return a.client.FetchBaseFile(ctx, req.Repo, req.Identifier, path)
}
