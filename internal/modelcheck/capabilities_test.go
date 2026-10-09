package modelcheck

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
)

// capableClient is a scripted client that reports its wire API's
// capabilities, as llm.APIClient does.
type capableClient struct {
	*scriptedClient
	caps llm.Capabilities
}

func (c capableClient) Capabilities() llm.Capabilities { return c.caps }

func TestCheckerSkipsProbesTheAPICannotExpress(t *testing.T) {
	client := capableClient{
		scriptedClient: &scriptedClient{responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel, Reasoned: true}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		}},
		caps: llm.Capabilities{Reasoning: llm.ReasoningSummary},
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})

	// One effort probe (no effort can be requested, so lower ones would repeat
	// it), the plain JSON probe, and no tools or schema request at all.
	if got := len(client.reqs); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
	for _, req := range client.reqs {
		if len(req.Tools) > 0 || len(req.Schema) > 0 {
			t.Fatalf("probe sent a feature the API cannot express: %#v", req)
		}
	}
	if p := result.ConfiguredTools(); p.Status != StatusUnsupported {
		t.Fatalf("tools probe = %s (%s), want unsupported", p.Status, p.Error)
	}
	if p := result.ConfiguredJSONSchema(); p.Status != StatusUnsupported {
		t.Fatalf("json schema probe = %s (%s), want unsupported", p.Status, p.Error)
	}
	summary := result.Summary()
	if !summary.Reasoning.Traces || summary.Reasoning.Kind != llm.ReasoningSummary {
		t.Fatalf("reasoning summary = %#v", summary.Reasoning)
	}
	if summary.JSONSchema == nil || *summary.JSONSchema {
		t.Fatalf("json schema summary = %v", summary.JSONSchema)
	}
}
