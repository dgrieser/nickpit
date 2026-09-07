package review

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestReviewDisputeRequiresIndependentAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, response            string
		wantCorrection, wantError bool
	}{
		{"accepted", `{"updates":[],"review":{"action":"correction_warranted","reason":"The explanation contradicts the checked guard."}}`, true, false},
		{"rejected", `{"updates":[],"review":{"action":"unchanged","reason":"The proposed guard is absent from current code."}}`, false, false},
		{"missing assessment", `{"updates":[]}`, false, true},
		{"missing evidence", `{"updates":[],"review":{"action":"correction_warranted","reason":""}}`, false, true},
		{"invalid action", `{"updates":[],"review":{"action":"refresh_verdict","reason":"Disputed."}}`, false, true},
		{"unrequested finding", `{"updates":[{"id":"finding","action":"resolved","reason":"The guard prevents failure."}],"review":{"action":"correction_warranted","reason":"Disputed."}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &updateTestLLM{responses: []*llm.ReviewResponse{{RawResponse: tc.response}}}
			e := NewEngine(stubSource{}, client, nil, config.Profile{Model: "test"})
			resolved := updateTestFinding()
			resolved.ID = "resolved"
			resolved.Body = "Obsolete resolved evidence"
			resolved.Resolution = &model.FindingResolution{Reason: "The guard fixes it."}
			original := &model.ReviewResult{OverallCorrectness: "patch is incorrect", OverallExplanation: "Original verdict explanation", Findings: []model.Finding{updateTestFinding(), resolved}}
			out, report, err := e.UpdateFindings(context.Background(), UpdateFindingsRequest{
				DiscussRequest: DiscussRequest{Result: original, ReviewCtx: &model.ReviewContext{}, Tools: []llm.ToolDefinition{}, Messages: []llm.Message{{Role: "user", Content: "The verdict contradicts the guard."}}},
				Signal:         ReviewUpdateSignal{Reason: "Check overall assessment against current guard."},
			})
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v wantError=%v", err, tc.wantError)
			}
			if tc.wantError {
				return
			}
			if len(client.requests) != 1 || report.ReviewCorrectionWarranted() != tc.wantCorrection || report.ReviewCheck == nil {
				t.Fatalf("assessment bypassed or incorrect: %+v", report)
			}
			expected, _ := original.Clone()
			if !reflect.DeepEqual(out, expected) {
				t.Fatal("review assessment changed findings or generated verdict")
			}
			prompt := client.requests[0].Messages[1].Content
			if !strings.Contains(prompt, "Original verdict explanation") || !strings.Contains(prompt, "Published title") || strings.Contains(prompt, "Obsolete resolved evidence") {
				t.Fatalf("wrong review context: %s", prompt)
			}
		})
	}
}

func TestReviewUpdateToolOnlySignalsDispute(t *testing.T) {
	tool := reviewUpdateTool()
	if strings.Contains(string(tool.Parameters), "refresh_verdict") || strings.Contains(tool.Description, "refresh_verdict") {
		t.Fatal("chat tool exposes verdict orchestration")
	}
	if err := (ReviewUpdateSignal{Reason: "Overall assessment contradicts current code."}).Validate(&model.ReviewResult{}); err != nil {
		t.Fatal(err)
	}
	if err := (ReviewUpdateSignal{}).Validate(&model.ReviewResult{}); err == nil {
		t.Fatal("dispute without evidence accepted")
	}
}
