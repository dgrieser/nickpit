package gitlab

import (
	"context"

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

// ReviewResults reassembles the complete ReviewResults previously published to an
// MR, keyed by review id, from the hidden carrier markers in its notes. It
// returns an empty map when the MR has no nickpit review markers (e.g. it was
// reviewed before carrier markers existed).
func (a *Adapter) ReviewResults(ctx context.Context, project string, iid int) (map[string]*model.ReviewResult, error) {
	bodies, err := a.client.MRNoteBodies(ctx, project, iid)
	if err != nil {
		return nil, err
	}
	return reviewmd.ReviewResultsByID(bodies), nil
}
