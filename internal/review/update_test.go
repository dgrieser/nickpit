package review

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

type updateTestLLM struct {
	responses []*llm.ReviewResponse
	requests  []*llm.ReviewRequest
}

func (s *updateTestLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.requests = append(s.requests, req)
	if len(s.responses) == 0 {
		return nil, errors.New("unexpected model call")
	}
	r := s.responses[0]
	s.responses = s.responses[1:]
	return r, nil
}

func updateTestFinding() model.Finding {
	p := 0
	return model.Finding{ID: "finding", Title: "Old title", Body: "Old evidence", Priority: &p, ConfidenceScore: 0.9,
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}, Content: "bad()"},
		Verification: &model.FindingVerification{ID: "finding", Verdict: "confirmed", Priority: 0, ConfidenceScore: 0.9},
		Finalization: &model.FindingFinalization{Title: "Published title", Body: "Published body", Priority: 0, ConfidenceScore: 0.9}}
}

func TestUpdateFindingsBatchPreservesIdentityAndRemovesStaleEvidence(t *testing.T) {
	f := updateTestFinding()
	other := f
	other.ID = "other"
	replacement := currentFinding(f)
	replacement.Title = "Corrected title"
	replacement.Body = "Corrected evidence"
	*replacement.Priority = 2
	replacement.ConfidenceScore = 0.8
	raw, _ := json.Marshal(map[string]any{"updates": []findingUpdateDecision{
		{ID: f.ID, Action: "updated", Reason: "Only the low-impact path is affected.", Finding: &replacement},
		{ID: other.ID, Action: "resolved", Reason: "The new guard prevents the failure."},
	}})
	client := &updateTestLLM{responses: []*llm.ReviewResponse{{RawResponse: string(raw)}}}
	e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	original := &model.ReviewResult{ReviewID: "review", Findings: []model.Finding{f, other}}
	out, run, err := e.UpdateFindings(context.Background(), UpdateFindingsRequest{DiscussRequest: DiscussRequest{
		Result: original, ReviewCtx: &model.ReviewContext{Diff: "patch"}, Tools: []llm.ToolDefinition{}, Messages: []llm.Message{{Role: "user", Content: "The guard fixes this."}},
	}, Signal: ReviewUpdateSignal{FindingIDs: []string{f.ID, other.ID}, Reason: "Check the guard."}})
	if err != nil {
		t.Fatal(err)
	}
	if run.Role != "update" || out.Findings[0].ID != f.ID || out.Findings[0].Verification != nil || out.Findings[0].Finalization != nil || priorityFloor(out.Findings[0], 3) != 2 || out.Findings[1].Resolution == nil {
		t.Fatalf("wrong update: %+v", out)
	}
	if original.Findings[1].Resolution != nil || original.Findings[0].Title != f.Title {
		t.Fatal("input mutated")
	}
	if strings.Contains(client.requests[0].Messages[1].Content, "Old evidence") {
		t.Fatal("outdated pre-finalization text reached updater")
	}
}

func TestUpdateFindingsUnchangedIsExactNoop(t *testing.T) {
	f := updateTestFinding()
	client := &updateTestLLM{responses: []*llm.ReviewResponse{{RawResponse: `{"updates":[{"id":"finding","action":"unchanged","reason":"The proposed fix is not in current code."}]}`}}}
	e := NewEngine(stubSource{}, client, nil, config.Profile{Model: "test"})
	original := &model.ReviewResult{Findings: []model.Finding{f}}
	out, _, err := e.UpdateFindings(context.Background(), UpdateFindingsRequest{DiscussRequest: DiscussRequest{Result: original, ReviewCtx: &model.ReviewContext{}, Tools: []llm.ToolDefinition{}}, Signal: ReviewUpdateSignal{FindingIDs: []string{f.ID}, Reason: "Proposed fix"}})
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := original.Clone()
	if !reflect.DeepEqual(out, expected) {
		t.Fatal("unchanged finding mutated")
	}
}

func TestUpdateValidation(t *testing.T) {
	f := currentFinding(updateTestFinding())
	selected := &model.ReviewResult{Findings: []model.Finding{f}}
	valid, _ := json.Marshal(map[string]any{"updates": []findingUpdateDecision{{ID: f.ID, Action: "updated", Reason: "Evidence.", Finding: &f}}})
	cases := []string{
		`{"updates":[]}`,
		`{"updates":[{"id":"other","action":"unchanged","reason":"Evidence."}]}`,
		`{"updates":[{"id":"finding","action":"resolved","reason":"First sentence. Second sentence."}]}`,
		strings.Replace(string(valid), `"confidence_score":0.9,`, "", 1),
		strings.Replace(string(valid), `"file_path":"main.go"`, `"file_path":"../secret"`, 1),
	}
	for _, raw := range cases {
		if _, err := parseFindingUpdates(raw, selected, false); err == nil {
			t.Fatalf("accepted invalid update: %s", raw)
		}
	}
	f.Resolution = &model.FindingResolution{Reason: "Fixed."}
	selected.Findings[0] = f
	if err := (ReviewUpdateSignal{FindingIDs: []string{f.ID}, Reason: "Reopen"}).Validate(selected); err == nil {
		t.Fatal("resolved finding reopened")
	}
}

func TestUpdateSuggestionsOmissionPreservesAndEmptyClears(t *testing.T) {
	f := currentFinding(updateTestFinding())
	f.Suggestions = []model.Suggestion{{Body: "Keep guard", CodeLocation: f.CodeLocation}}
	selected := &model.ReviewResult{Findings: []model.Finding{f}}
	for _, clear := range []bool{false, true} {
		replacement := f
		replacement.Suggestions = nil
		encoded, _ := json.Marshal(replacement)
		var fields map[string]any
		_ = json.Unmarshal(encoded, &fields)
		if clear {
			fields["suggestions"] = []any{}
		}
		raw, _ := json.Marshal(map[string]any{"updates": []any{map[string]any{"id": f.ID, "action": "updated", "reason": "Evidence.", "finding": fields}}})
		decisions, err := parseFindingUpdates(string(raw), selected, false)
		if err != nil {
			t.Fatal(err)
		}
		if (len(decisions[0].Finding.Suggestions) == 0) != clear {
			t.Fatalf("clear=%v: suggestions=%+v", clear, decisions[0].Finding.Suggestions)
		}
	}
}

func TestDiscussUpdateInstructionsMatchToolRegistration(t *testing.T) {
	for _, tc := range []struct {
		name     string
		callback bool
		maxTools int
		want     bool
	}{
		{"terminal", false, 0, false},
		{"gitlab", true, 0, true},
		{"tools disabled", true, -1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &updateTestLLM{responses: []*llm.ReviewResponse{{RawResponse: "Answer."}}}
			e := NewEngine(stubSource{}, client, nil, config.Profile{Model: "test"})
			req := DiscussRequest{Result: &model.ReviewResult{}, ReviewCtx: &model.ReviewContext{}, Messages: []llm.Message{{Role: "user", Content: "Explain."}}, Tools: []llm.ToolDefinition{}, MaxToolCalls: tc.maxTools}
			if tc.callback {
				req.UpdateReview = func(context.Context, ReviewUpdateSignal) (*ReviewUpdateOutcome, error) {
					t.Fatal("unexpected update call")
					return nil, nil
				}
			}
			if _, err := e.Discuss(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			request := client.requests[0]
			registered := false
			for _, tool := range request.Tools {
				registered = registered || tool.Name == reviewUpdateToolName
			}
			prompt := request.Messages[0].Content
			if registered != tc.want || strings.Contains(prompt, reviewUpdateToolName) != tc.want {
				t.Fatalf("tool=%v instructions=%v want=%v", registered, strings.Contains(prompt, reviewUpdateToolName), tc.want)
			}
			if strings.Contains(prompt, "is available and evidence") {
				t.Fatal("availability check reached agent")
			}
		})
	}
}

func TestDiscussUpdateToolUsesActualOutcomeAndPropagatesFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			client := &updateTestLLM{responses: []*llm.ReviewResponse{
				{ToolCalls: []llm.ToolCall{{ID: "call", Name: reviewUpdateToolName, Arguments: `{"finding_ids":["finding"],"reason":"Guard proves it safe."}`}}},
				{RawResponse: "The finding has been resolved."},
			}}
			e := NewEngine(stubSource{}, client, nil, config.Profile{Model: "test"})
			called := 0
			_, err := e.Discuss(context.Background(), DiscussRequest{Result: &model.ReviewResult{Findings: []model.Finding{updateTestFinding()}}, ReviewCtx: &model.ReviewContext{}, Messages: []llm.Message{{Role: "user", Content: "Check the guard."}}, Tools: []llm.ToolDefinition{}, UpdateReview: func(context.Context, ReviewUpdateSignal) (*ReviewUpdateOutcome, error) {
				called++
				if fail {
					return nil, errors.New("publish failed")
				}
				return &ReviewUpdateOutcome{OverallCorrectness: "patch is correct"}, nil
			}})
			if called != 1 || (err != nil) != fail {
				t.Fatalf("callback calls=%d err=%v", called, err)
			}
			if !fail {
				last := client.requests[1].Messages
				found := false
				for _, msg := range last {
					if msg.Role == "tool" && strings.Contains(msg.Content, "patch is correct") {
						found = true
					}
				}
				if !found {
					t.Fatal("actual outcome not supplied to chat")
				}
			}
		})
	}
}

func TestVerdictExcludesResolvedFindingAndOldPromptContent(t *testing.T) {
	f := updateTestFinding()
	f.Resolution = &model.FindingResolution{Reason: "The guard prevents the failure."}
	e := NewEngine(stubSource{}, nil, nil, config.Profile{})
	out, _, err := e.Verdict(context.Background(), &model.ReviewContext{}, &model.ReviewResult{Findings: []model.Finding{f}}, VerdictOptions{DisablePatchSummary: true})
	if err != nil || out.OverallCorrectness != "patch is correct" || len(out.Findings) != 0 {
		t.Fatalf("resolved finding affected verdict: %+v %v", out, err)
	}
	prompt := discussReviewForPrompt(&model.ReviewResult{Findings: []model.Finding{f}}, false)
	raw, _ := json.Marshal(prompt)
	if strings.Contains(string(raw), "Old evidence") || !strings.Contains(string(raw), f.Resolution.Reason) {
		t.Fatal("resolved prompt includes obsolete evidence")
	}
}
