package llm

import (
	"context"
	"testing"
)

type namedClient struct{ name string }

func (c *namedClient) Review(context.Context, *ReviewRequest) (*ReviewResponse, error) {
	return &ReviewResponse{}, nil
}

func clientName(t *testing.T, c Client) string {
	t.Helper()
	if c == nil {
		return ""
	}
	named, ok := c.(*namedClient)
	if !ok {
		t.Fatalf("unexpected client type %T", c)
	}
	return named.name
}

func TestClientSetResolvesRegisteredEndpoint(t *testing.T) {
	primary := &namedClient{name: "primary"}
	small := &namedClient{name: "small"}
	set := NewClientSet(primary, Endpoint{BaseURL: "http://primary/v1", APIKey: "k1"}).
		With(Endpoint{BaseURL: "http://small/v1", APIKey: "k2"}, small)

	tests := []struct {
		name     string
		endpoint Endpoint
		want     string
	}{
		{"primary", Endpoint{BaseURL: "http://primary/v1", APIKey: "k1"}, "primary"},
		{"primary trailing slash", Endpoint{BaseURL: "http://primary/v1/", APIKey: "k1"}, "primary"},
		{"small", Endpoint{BaseURL: "http://small/v1", APIKey: "k2"}, "small"},
		{"small trailing slash and spaces", Endpoint{BaseURL: " http://small/v1/ ", APIKey: "k2"}, "small"},
		// Same host, other credential: not the registered endpoint, so it must not
		// borrow the small client's key.
		{"small host other key", Endpoint{BaseURL: "http://small/v1", APIKey: "other"}, "primary"},
		{"unknown host", Endpoint{BaseURL: "http://elsewhere/v1", APIKey: "k3"}, "primary"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientName(t, set.For(tc.endpoint)); got != tc.want {
				t.Fatalf("For(%+v) = %q, want %q", tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestClientSetWithDoesNotMutateOriginal(t *testing.T) {
	primary := &namedClient{name: "primary"}
	small := &namedClient{name: "small"}
	base := NewClientSet(primary, Endpoint{BaseURL: "http://primary/v1", APIKey: "k1"})
	extended := base.With(Endpoint{BaseURL: "http://small/v1", APIKey: "k2"}, small)

	smallEndpoint := Endpoint{BaseURL: "http://small/v1", APIKey: "k2"}
	if got := clientName(t, base.For(smallEndpoint)); got != "primary" {
		t.Fatalf("original set resolved the small endpoint to %q, want the primary client", got)
	}
	if got := clientName(t, extended.For(smallEndpoint)); got != "small" {
		t.Fatalf("extended set resolved the small endpoint to %q, want small", got)
	}
}

// Registering the primary endpoint again must not shadow the primary client: it is
// how a profile whose small model shares the endpoint arrives here.
func TestClientSetWithPrimaryEndpointIsNoOp(t *testing.T) {
	primary := &namedClient{name: "primary"}
	set := NewClientSet(primary, Endpoint{BaseURL: "http://primary/v1", APIKey: "k1"}).
		With(Endpoint{BaseURL: "http://primary/v1/", APIKey: "k1"}, &namedClient{name: "duplicate"})
	if got := clientName(t, set.For(Endpoint{BaseURL: "http://primary/v1", APIKey: "k1"})); got != "primary" {
		t.Fatalf("primary endpoint resolved to %q, want primary", got)
	}
}

// Commands that build an engine only for its non-LLM helpers pass no client.
func TestClientSetToleratesNilPrimary(t *testing.T) {
	set := NewClientSet(nil, Endpoint{})
	if c := set.For(Endpoint{}); c != nil {
		t.Fatalf("For on a nil-primary set = %v, want nil", c)
	}
	if c := set.For(Endpoint{BaseURL: "http://elsewhere/v1"}); c != nil {
		t.Fatalf("For on an unknown endpoint = %v, want nil", c)
	}
	// A nil client must not become a resolvable entry either.
	if c := set.With(Endpoint{BaseURL: "http://small/v1"}, nil).For(Endpoint{BaseURL: "http://small/v1"}); c != nil {
		t.Fatalf("For after registering a nil client = %v, want nil", c)
	}
}

// attemptReasoningEffortAllowed distinguishes "never restricted" (nil) from
// "nothing allowed" (empty), and SetAllowedReasoningEfforts always installs a
// non-nil map. Callers must therefore skip the setter when a probe produced no
// passed efforts — this test pins the behaviour that makes that necessary.
func TestAllowedReasoningEffortsEmptySetBlocksEverything(t *testing.T) {
	if !attemptReasoningEffortAllowed("high", nil) {
		t.Fatal("an unset allowlist must allow every effort")
	}
	client := NewOpenAIClient("http://host/v1", "key", "model")
	if !attemptReasoningEffortAllowed("high", client.allowedEfforts) {
		t.Fatal("a fresh client must not restrict efforts")
	}
	client.SetAllowedReasoningEfforts(nil)
	if attemptReasoningEffortAllowed("high", client.allowedEfforts) {
		t.Fatal("SetAllowedReasoningEfforts(nil) installs an empty allowlist that blocks every effort")
	}
}
