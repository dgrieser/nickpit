package review

import (
	"context"
	"reflect"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestUpdateSummaryUsesDefaultPassesAndSmallModel(t *testing.T) {
	f := updateTestFinding()
	f.ID = "00000000-0000-4000-8000-000000000001"
	resolved := f
	resolved.ID = "resolved"
	resolved.Resolution = &model.FindingResolution{Reason: "The guard fixes it."}
	unchanged := f
	unchanged.ID = "unchanged"
	client := &capturingLLM{resps: []*llm.ReviewResponse{
		{Findings: []model.Finding{{ID: f.ID, Summarization: &model.FindingSummarization{Body: "Short finding."}}}},
		{Findings: []model.Finding{{ID: overallSummaryID, Summarization: &model.FindingSummarization{Body: "Short verdict."}}}},
	}}
	e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "primary", Small: config.SmallModelConfig{Model: "small"}})
	in := &model.ReviewResult{Findings: []model.Finding{f, resolved, unchanged}, OverallExplanation: "Long verdict explanation."}
	out, _, err := e.SummarizeUpdate(context.Background(), in, []model.Finding{f, resolved}, model.AgentRun{Status: model.AgentRunStatusOK}, true, model.ReviewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.reqs) != 2 || client.reqs[0].Model != "small" || client.reqs[1].Model != "small" {
		t.Fatal("default small-model passes not used")
	}
	if out.Findings[0].Summarization == nil || out.Findings[0].Summarization.Body != "Short finding." || out.OverallExplanation != "Short verdict." {
		t.Fatalf("summary missing: %+v", out)
	}
	expected, _ := in.Clone()
	if !reflect.DeepEqual(out.Findings[1:], expected.Findings[1:]) {
		t.Fatal("unrelated or resolved findings changed")
	}
}

func TestUpdateSummaryOverallOnlyAndFailureFallback(t *testing.T) {
	for _, agentWrote := range []bool{false, true} {
		client := &updateTestLLM{} // No response: exercise default warning-only fallback.
		e := NewEngine(stubSource{}, client, nil, config.Profile{Model: "test"})
		run := model.AgentRun{Status: model.AgentRunStatusSkipped}
		if agentWrote {
			run.Status = model.AgentRunStatusOK
		}
		in := &model.ReviewResult{OverallExplanation: "Original verdict."}
		out, _, err := e.SummarizeUpdate(context.Background(), in, nil, run, false, model.ReviewRequest{})
		if err != nil || out.OverallExplanation != in.OverallExplanation {
			t.Fatalf("fallback changed: %+v %v", out, err)
		}
		if (len(client.requests) > 0) != agentWrote || (len(out.Warnings) > 0) != agentWrote {
			t.Fatal("static-stub skip or failure warning differs from workflow")
		}
	}
}
