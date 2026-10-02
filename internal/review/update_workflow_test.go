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

func TestUpdateWorkflowReviewOnlyCorrectionDoesNotRepublishPinnedVerdict(t *testing.T) {
	for _, tc := range []struct {
		name        string
		floor       int
		wantBlocked bool
	}{
		// The finding displays as P1, but its verifier-confirmed P0 floor pins the verdict.
		{"p0 floor blocks", 0, true},
		{"p1 floor reruns verdict", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := updateTestFinding()
			display := 1
			f.Priority, f.Finalization.Priority = &display, display
			f.Verification.Priority = tc.floor
			client := &updateTestLLM{responses: []*llm.ReviewResponse{
				{RawResponse: `{"updates":[],"review":{"action":"correction_warranted","reason":"Finding finding is fixed by abc123."}}`},
				{OverallCorrectness: "patch is incorrect", OverallExplanation: "Still blocked.", OverallConfidenceScore: 0.9},
				{Findings: []model.Finding{{ID: overallSummaryID, Summarization: &model.FindingSummarization{Body: "Short verdict."}}}},
			}}
			e := NewEngine(stubSource{}, client, nil, config.Profile{Model: "test"})
			result, err := e.RunUpdateWorkflow(context.Background(), UpdateWorkflowRequest{DisablePatchSummary: true, UpdateFindingsRequest: UpdateFindingsRequest{
				DiscussRequest: DiscussRequest{Result: &model.ReviewResult{OverallCorrectness: "patch is incorrect", Findings: []model.Finding{f}}, ReviewCtx: &model.ReviewContext{}, Tools: []llm.ToolDefinition{}},
				Signal:         ReviewUpdateSignal{Reason: "Why is the patch still incorrect?"},
			}})
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantBlocked {
				if result.Publish || len(client.requests) != 1 || len(result.Outcome.BlockedBy) != 1 || result.Outcome.BlockedBy[0] != f.ID {
					t.Fatalf("pinned verdict republished: publish=%v calls=%d blocked=%v", result.Publish, len(client.requests), result.Outcome.BlockedBy)
				}
				return
			}
			if !result.Publish || len(client.requests) < 2 || client.requests[1].SchemaKind != llm.SchemaKindVerdict || len(result.Outcome.BlockedBy) != 0 {
				t.Fatalf("open verdict not re-run: publish=%v calls=%d blocked=%v", result.Publish, len(client.requests), result.Outcome.BlockedBy)
			}
		})
	}
}
