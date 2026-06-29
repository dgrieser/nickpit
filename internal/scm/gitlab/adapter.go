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
