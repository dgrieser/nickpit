package gitlab

import (
	"context"
	"fmt"

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
	return a.client.FetchMR(ctx, req.Repo, req.Identifier, req.IncludeComments)
}

func (a *Adapter) ResolveCheckout(ctx context.Context, req model.ReviewRequest) (*model.CheckoutSpec, error) {
	return a.client.FetchMRCheckout(ctx, req.Repo, req.Identifier)
}

// Client exposes the underlying GitLab client for callers that need note-level
// operations (reading a chat thread, posting a reply) beyond the review flow.
func (a *Adapter) Client() *Client { return a.client }

// ReviewResults reassembles the complete ReviewResults previously published to
// an MR, keyed by review id, from the hidden carrier markers in its notes.
// Carrier markers are only encoded, not authenticated, so any commenter could
// forge one; to prevent attacker-controlled findings from entering a chat
// prompt, only markers in notes authored by the client token's own user (the
// bot that published the review) are trusted. It returns an empty map when the
// MR has no trusted nickpit review markers (e.g. it was reviewed before carrier
// markers existed, or the reviews were posted by a different user than this
// token's).
func (a *Adapter) ReviewResults(ctx context.Context, project string, iid int) (map[string]*model.ReviewResult, error) {
	return a.reviewResults(ctx, project, iid, true)
}

// ReviewResultsAnyAuthor reads the same markers without the author check, for a
// reader that has to see reviews this token did not publish: a project reviewed
// by another group's bot, or by a colleague's token, carries markers no local
// token can claim, and refusing them would show nothing at all.
//
// The trade-off is the one the author check exists for — anyone who can comment
// on the request can forge a marker, and what comes back is then their text —
// so this is for READ-ONLY use (listing and printing what a request carries).
// Publishing and correcting keep the strict reader.
func (a *Adapter) ReviewResultsAnyAuthor(ctx context.Context, project string, iid int) (map[string]*model.ReviewResult, error) {
	return a.reviewResults(ctx, project, iid, false)
}

func (a *Adapter) reviewResults(ctx context.Context, project string, iid int, trustedOnly bool) (map[string]*model.ReviewResult, error) {
	userID := 0
	if trustedOnly {
		user, err := a.client.CurrentUser(ctx)
		if err != nil {
			return nil, fmt.Errorf("gitlab: resolving token user for carrier verification: %w", err)
		}
		userID = user.ID
	}
	notes, err := a.client.MRNotes(ctx, project, iid)
	if err != nil {
		return nil, err
	}
	var bodies []string
	for _, note := range notes {
		if !trustedOnly || note.AuthorID == userID {
			bodies = append(bodies, note.Body)
		}
	}
	return reviewmd.ReviewResultsByID(bodies), nil
}
