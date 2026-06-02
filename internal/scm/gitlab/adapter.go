package gitlab

import (
	"context"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

// defaultAssetBaseURL is the fallback badge host used when NewAdapter is given
// an empty base URL (mirrors config.DefaultAssetBaseURL, kept here so the scm
// package stays independent of config).
const defaultAssetBaseURL = "https://dgrieser.github.io/nickpit/"

type Adapter struct {
	client *Client
	// assetBaseURL is the badge SVG host, always normalized to a trailing "/".
	assetBaseURL string
}

func NewAdapter(client *Client, assetBaseURL string) *Adapter {
	assetBaseURL = strings.TrimSpace(assetBaseURL)
	if assetBaseURL == "" {
		assetBaseURL = defaultAssetBaseURL
	}
	if !strings.HasSuffix(assetBaseURL, "/") {
		assetBaseURL += "/"
	}
	return &Adapter{client: client, assetBaseURL: assetBaseURL}
}

func (a *Adapter) ResolveContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	return a.client.FetchMR(ctx, req.Repo, req.Identifier, req.IncludeComments && !req.Offline)
}

func (a *Adapter) ResolveCheckout(ctx context.Context, req model.ReviewRequest) (*model.CheckoutSpec, error) {
	return a.client.FetchMRCheckout(ctx, req.Repo, req.Identifier)
}
