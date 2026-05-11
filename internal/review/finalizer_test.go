package review

import (
	"context"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestFinalizePromptIncludesInlineFinalizeSchema(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title:           "Fix issue",
						Body:            "body",
						ConfidenceScore: 0.7,
						Priority:        intPtr(1),
						CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
						Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
						Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
					},
				},
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "ok",
				OverallConfidenceScore: 0.7,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
			},
		},
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "ok",
		OverallConfidenceScore: 0.7,
	}

	_, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.reqs))
	}
	req := llmClient.reqs[0]
	if req.SchemaKind != llm.SchemaKindFinalize {
		t.Fatalf("schema kind = %v, want finalize", req.SchemaKind)
	}
	systemPrompt := req.Messages[0].Content
	for _, want := range []string{`"verification"`, `"finalization"`, `"title"`, `"body"`, `"remarks"`} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("finalize system prompt missing %s:\n%s", want, systemPrompt)
		}
	}
}

func TestExampleSnippetForFinalizeIncludesFinalization(t *testing.T) {
	snippet := exampleSnippetFor(llm.SchemaKindFinalize)
	if !strings.Contains(snippet, `"finalization"`) {
		t.Fatalf("finalize retry example missing finalization: %s", snippet)
	}
}
