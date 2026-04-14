package github

import (
	"context"

	"github.com/dgrieser/nickpit/internal/model"
)

type Adapter struct {
	client *Client
}

func NewAdapter(client *Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) ResolveContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	return a.client.FetchPR(ctx, req.Repo, req.Identifier, req.IncludeComments && !req.Offline)
}

func (a *Adapter) ResolveCheckout(ctx context.Context, req model.ReviewRequest) (*model.CheckoutSpec, error) {
	return a.client.FetchPRCheckout(ctx, req.Repo, req.Identifier)
}
