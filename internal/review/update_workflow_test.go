package review

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestUpdateWorkflowRunsFixedStages(t *testing.T) {
	f := currentFinding(updateTestFinding())
	f.ID = "00000000-0000-4000-8000-000000000001"
	replacement := f
	replacement.Body = "Corrected evidence."
	raw, _ := json.Marshal(map[string]any{"updates": []findingUpdateDecision{{ID: f.ID, Action: "updated", Reason: "Current code confirms correction.", Finding: &replacement}}})
	client := &updateTestLLM{responses: []*llm.ReviewResponse{
		{RawResponse: string(raw)},
		{OverallCorrectness: "patch is incorrect", OverallExplanation: "Updated verdict explanation.", OverallConfidenceScore: 0.9},
		{Findings: []model.Finding{{ID: f.ID, Summarization: &model.FindingSummarization{Body: "Short corrected evidence."}}}},
		{Findings: []model.Finding{{ID: overallSummaryID, Summarization: &model.FindingSummarization{Body: "Short verdict."}}}},
	}}
	e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "primary", Small: config.SmallModelConfig{Model: "small"}})
	result, err := e.RunUpdateWorkflow(context.Background(), UpdateWorkflowRequest{UpdateFindingsRequest: UpdateFindingsRequest{
		DiscussRequest: DiscussRequest{Result: &model.ReviewResult{Findings: []model.Finding{f}}, ReviewCtx: &model.ReviewContext{}, Tools: []llm.ToolDefinition{}},
		Signal:         ReviewUpdateSignal{FindingIDs: []string{f.ID}, Reason: "Correct the evidence."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Publish || len(client.requests) != 4 || len(result.Outcome.Changed) != 1 || result.Outcome.OverallExplanation != "Short verdict." {
		t.Fatalf("wrong workflow result: %+v calls=%d", result, len(client.requests))
	}
	for i, kind := range []llm.SchemaKind{llm.SchemaKindJSON, llm.SchemaKindVerdict, llm.SchemaKindSummarize, llm.SchemaKindSummarize} {
		if client.requests[i].SchemaKind != kind {
			t.Fatalf("stage %d schema=%s want=%s", i, client.requests[i].SchemaKind, kind)
		}
		wantModel := "primary"
		if i >= 2 {
			wantModel = "small"
		}
		if client.requests[i].Model != wantModel {
			t.Fatalf("stage %d model=%s want=%s", i, client.requests[i].Model, wantModel)
		}
	}
	if result.Outcome.Changed[0].Summarization.Body != "Short corrected evidence." {
		t.Fatal("published pre-summary finding")
	}
}

func TestUpdateWorkflowNoopSkipsVerdictSummaryAndPublication(t *testing.T) {
	f := updateTestFinding()
	client := &updateTestLLM{responses: []*llm.ReviewResponse{{RawResponse: `{"updates":[{"id":"finding","action":"unchanged","reason":"No supporting evidence."}]}`}}}
	e := NewEngine(stubSource{}, client, nil, config.Profile{Model: "test"})
	result, err := e.RunUpdateWorkflow(context.Background(), UpdateWorkflowRequest{UpdateFindingsRequest: UpdateFindingsRequest{
		DiscussRequest: DiscussRequest{Result: &model.ReviewResult{Findings: []model.Finding{f}}, ReviewCtx: &model.ReviewContext{}, Tools: []llm.ToolDefinition{}},
		Signal:         ReviewUpdateSignal{FindingIDs: []string{f.ID}, Reason: "Check this."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Publish || len(result.Outcome.Changed) != 0 || len(client.requests) != 1 {
		t.Fatal("unchanged finding triggered downstream stages")
	}
}
