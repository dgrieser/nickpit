package llm

import (
	"github.com/dgrieser/nickpit/internal/model"
)

// Endpoint identifies an LLM endpoint a client talks to. Both fields matter: two
// endpoints can share a base URL and differ only by credential (another tenant
// or quota), which still needs its own client because the key is baked into the
// client at construction.
type Endpoint struct {
	BaseURL string
	APIKey  string
}

// NewEndpoint canonicalizes the base URL so a trailing slash or stray
// whitespace cannot split one endpoint into two clients.
func NewEndpoint(baseURL, apiKey string) Endpoint {
	return Endpoint{BaseURL: model.NormalizeBaseURL(baseURL), APIKey: apiKey}
}

type clientSetEntry struct {
	endpoint Endpoint
	client   Client
}

// ClientSet resolves an endpoint to the client that talks to it. A run has one
// primary endpoint and may add a second one (the profile's small model living on
// another host), so lookups walk a tiny slice rather than a map.
//
// A set is built before the pipeline starts and never mutated afterwards — With
// returns a copy — so concurrent agent loops can share it without locking, the
// same write-once contract the engine's styleguide fields use.
type ClientSet struct {
	primaryEndpoint Endpoint
	primary         Client
	extra           []clientSetEntry
}

// NewClientSet creates a set holding just the primary client. A nil primary is
// allowed: commands that build an engine only for its non-LLM helpers pass no
// client at all.
func NewClientSet(primary Client, primaryEndpoint Endpoint) *ClientSet {
	return &ClientSet{
		primaryEndpoint: NewEndpoint(primaryEndpoint.BaseURL, primaryEndpoint.APIKey),
		primary:         primary,
	}
}

// With returns a copy of the set that also resolves endpoint to client. A nil
// client, or an endpoint equal to one the set already resolves, is ignored: the
// existing resolution wins, so registering the small endpoint of a profile whose
// small model shares the primary endpoint is a no-op.
func (s *ClientSet) With(endpoint Endpoint, client Client) *ClientSet {
	if s == nil {
		return NewClientSet(client, endpoint)
	}
	clone := &ClientSet{
		primaryEndpoint: s.primaryEndpoint,
		primary:         s.primary,
		extra:           append([]clientSetEntry(nil), s.extra...),
	}
	if client == nil {
		return clone
	}
	endpoint = NewEndpoint(endpoint.BaseURL, endpoint.APIKey)
	if endpoint == clone.primaryEndpoint {
		return clone
	}
	for _, entry := range clone.extra {
		if entry.endpoint == endpoint {
			return clone
		}
	}
	clone.extra = append(clone.extra, clientSetEntry{endpoint: endpoint, client: client})
	return clone
}

// For returns the client for endpoint, falling back to the primary client when
// no exact match is registered. The fallback direction is deliberate: it pairs
// the primary key with the primary host, so a miss can never send a credential
// somewhere it does not belong — the worst case is a request on the primary
// model instead of the intended one.
func (s *ClientSet) For(endpoint Endpoint) Client {
	if s == nil {
		return nil
	}
	endpoint = NewEndpoint(endpoint.BaseURL, endpoint.APIKey)
	if endpoint == s.primaryEndpoint {
		return s.primary
	}
	for _, entry := range s.extra {
		if entry.endpoint == endpoint {
			return entry.client
		}
	}
	return s.primary
}
