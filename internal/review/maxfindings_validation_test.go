package review

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
)

func maxFindingsTestFinding(title string, priority int, confidence float64) model.Finding {
	finding := nudgeFindingInFile(title, title+".go", 1)
	finding.Priority = intPtr(priority)
	finding.ConfidenceScore = confidence
	return finding
}

func maxFindingsNudgeTestAgent(limit int) agentSpec {
	agent := nudgeTestAgent("review")
	agent.name = "Code Quality"
	agent.maxFindings = limit
	return agent
}

func TestMaxFindingsValidatorRejectsOnceThenPasses(t *testing.T) {
	validate := newMaxFindingsValidator(2)
	resp := nudgeReviewResponse("over", 1,
		maxFindingsTestFinding("A", 0, 0.9),
		maxFindingsTestFinding("B", 2, 0.5),
		maxFindingsTestFinding("C", 1, 0.8),
	)

	invalid := validate(nil, resp)
	if invalid == nil {
		t.Fatal("validator accepted response over the finding limit")
	}
	if !strings.Contains(invalid.Reason, "max_findings_exceeded limit=2") {
		t.Fatalf("reason = %q, want max_findings_exceeded", invalid.Reason)
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"at most 2 findings in total", "included 3 findings", "keep only the 2 most critical"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("retry guidance missing %q:\n%s", want, rendered)
		}
	}

	// The single retry is spent: the same over-limit response now passes so the
	// enforce step cuts it instead of burning further retries.
	if invalid := validate(nil, resp); invalid != nil {
		t.Fatalf("validator rejected again after its one retry: %v", invalid)
	}
}

func TestMaxFindingsValidatorCountsOnlyAppendableFindings(t *testing.T) {
	existing := []model.Finding{maxFindingsTestFinding("Existing", 1, 0.8)}
	replay := existing[0]

	// A replayed existing finding is deduplicated by appendNewFindings and must
	// not count against the limit.
	validate := newMaxFindingsValidator(2)
	if invalid := validate(existing, nudgeReviewResponse("replay", 1, replay, maxFindingsTestFinding("New", 0, 0.9))); invalid != nil {
		t.Fatalf("validator counted replayed finding: %v", invalid)
	}

	validate = newMaxFindingsValidator(2)
	invalid := validate(existing, nudgeReviewResponse("over", 1, maxFindingsTestFinding("New", 0, 0.9), maxFindingsTestFinding("Extra", 2, 0.4)))
	if invalid == nil {
		t.Fatal("validator accepted response exceeding the remaining budget")
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"already reported 1 finding", "may add at most 1 new finding", "keep only the 1 most critical"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("retry guidance missing %q:\n%s", want, rendered)
		}
	}
}

func TestSplitStrongestFindings(t *testing.T) {
	weak := maxFindingsTestFinding("Weak", 2, 0.9)
	critical := maxFindingsTestFinding("Critical", 0, 0.4)
	lowConfidence := maxFindingsTestFinding("LowConfidence", 1, 0.3)
	confident := maxFindingsTestFinding("Confident", 1, 0.8)
	unset := maxFindingsTestFinding("Unset", 0, 0.5)
	unset.Priority = nil // nil priority ranks lowest (rank 3)

	kept, dropped := splitStrongestFindings([]model.Finding{weak, critical, lowConfidence, confident, unset}, 2)
	if got, want := findingTitles(kept), []string{"Critical", "Confident"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kept = %#v, want %#v", got, want)
	}
	if got, want := findingTitles(dropped), []string{"Weak", "LowConfidence", "Unset"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dropped = %#v, want %#v", got, want)
	}

	kept, dropped = splitStrongestFindings([]model.Finding{weak, critical}, 0)
	if len(kept) != 0 || len(dropped) != 2 {
		t.Fatalf("zero budget: kept=%d dropped=%d", len(kept), len(dropped))
	}
}

func TestEnforceMaxFindingsResponseCutsWeakest(t *testing.T) {
	resp := nudgeReviewResponse("over", 1,
		maxFindingsTestFinding("Weak", 2, 0.5),
		maxFindingsTestFinding("Critical", 0, 0.9),
		maxFindingsTestFinding("Medium", 1, 0.8),
	)
	msg := enforceMaxFindingsResponse("Code Quality", 2, nil, resp)
	if got, want := findingTitles(resp.Findings), []string{"Critical", "Medium"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	for _, want := range []string{"Code Quality", "max_findings limit (2)", "Weak"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %q", want, msg)
		}
	}

	within := nudgeReviewResponse("ok", 1, maxFindingsTestFinding("Only", 1, 0.8))
	if msg := enforceMaxFindingsResponse("Code Quality", 2, nil, within); msg != "" {
		t.Fatalf("within-limit message = %q, want none", msg)
	}
	if got := len(within.Findings); got != 1 {
		t.Fatalf("within-limit findings = %d, want untouched", got)
	}

	existing := []model.Finding{maxFindingsTestFinding("Existing", 1, 0.8), maxFindingsTestFinding("Existing2", 1, 0.7)}
	full := nudgeReviewResponse("full", 1, maxFindingsTestFinding("New", 0, 0.9))
	if msg := enforceMaxFindingsResponse("Code Quality", 2, existing, full); msg == "" {
		t.Fatal("expected drop message when session already at the limit")
	}
	if len(full.Findings) != 0 {
		t.Fatalf("at-limit findings = %d, want all cut", len(full.Findings))
	}
}

func TestRunAgent_MaxFindingsInitialRetry(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("over", 1,
				maxFindingsTestFinding("A", 0, 0.9),
				maxFindingsTestFinding("B", 2, 0.5),
				maxFindingsTestFinding("C", 1, 0.8),
			)},
			{resp: nudgeReviewResponse("retry", 1,
				maxFindingsTestFinding("A", 0, 0.9),
				maxFindingsTestFinding("C", 1, 0.8),
			)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), maxFindingsNudgeTestAgent(2), model.ReviewRequest{MaxOutputRetries: 1, MaxFindings: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if got, want := result.run.Status, model.AgentRunStatusOK; got != want {
		t.Fatalf("status = %q, want clean run", got)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want initial plus retry", len(llmClient.reqs))
	}
	retryMessage := llmClient.reqs[1].Messages[len(llmClient.reqs[1].Messages)-1].Content
	if !strings.Contains(retryMessage, "valid JSON but failed response validation") {
		t.Fatalf("retry message missing semantic validation framing:\n%s", retryMessage)
	}
	if !strings.Contains(retryMessage, "at most 2 findings in total") {
		t.Fatalf("retry message missing max-findings guidance:\n%s", retryMessage)
	}
}

func TestRunAgent_MaxFindingsRetryExhaustionCutsWeakest(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("over", 1,
				maxFindingsTestFinding("B", 2, 0.5),
				maxFindingsTestFinding("A", 0, 0.9),
				maxFindingsTestFinding("C", 1, 0.8),
			)},
			{resp: nudgeReviewResponse("still over", 1,
				maxFindingsTestFinding("Retry B", 2, 0.5),
				maxFindingsTestFinding("Retry A", 0, 0.9),
				maxFindingsTestFinding("Retry C", 1, 0.8),
			)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), maxFindingsNudgeTestAgent(2), model.ReviewRequest{MaxOutputRetries: 5, MaxFindings: 2})
	if err != nil {
		t.Fatal(err)
	}
	// One retry only, even with budget left: the validator lets the second
	// over-limit response through and the weakest finding is cut instead.
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want initial plus one retry", len(llmClient.reqs))
	}
	if got, want := findingTitles(result.resp.Findings), []string{"Retry A", "Retry C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if got, want := result.run.Status, model.AgentRunStatusPartial; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if !strings.Contains(result.run.Error, "max_findings limit (2)") || !strings.Contains(result.run.Error, "Retry B") {
		t.Fatalf("run error = %q, want dropped finding details", result.run.Error)
	}
}

func TestRunAgent_MaxFindingsNudgeBudgetAndRetry(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("initial", 1, maxFindingsTestFinding("A", 1, 0.8))},
			{resp: nudgeReviewResponse("nudge over", 1,
				maxFindingsTestFinding("N1", 2, 0.4),
				maxFindingsTestFinding("N2", 0, 0.9),
			)},
			{resp: nudgeReviewResponse("nudge retry", 1, maxFindingsTestFinding("N2", 0, 0.9))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), maxFindingsNudgeTestAgent(2), model.ReviewRequest{NudgeCount: 1, MaxOutputRetries: 1, MaxFindings: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "N2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("llm calls = %d, want initial, nudge, retry", len(llmClient.reqs))
	}
	nudgeMessage := llmClient.reqs[1].Messages[len(llmClient.reqs[1].Messages)-1].Content
	if !strings.Contains(nudgeMessage, "finding limit for this review is 2") || !strings.Contains(nudgeMessage, "at most 1 more finding") {
		t.Fatalf("nudge message missing remaining-budget line:\n%s", nudgeMessage)
	}
	retryMessage := llmClient.reqs[2].Messages[len(llmClient.reqs[2].Messages)-1].Content
	if !strings.Contains(retryMessage, "already reported 1 finding") || !strings.Contains(retryMessage, "may add at most 1 new finding") {
		t.Fatalf("nudge retry message missing session budget guidance:\n%s", retryMessage)
	}
}

func TestRunAgent_MaxFindingsSkipsNudgesAtLimit(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("initial", 1, maxFindingsTestFinding("A", 1, 0.8))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), maxFindingsNudgeTestAgent(1), model.ReviewRequest{NudgeCount: 2, MaxOutputRetries: 1, MaxFindings: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("llm calls = %d, want initial only (nudges skipped at finding limit)", len(llmClient.reqs))
	}
	if got, want := result.run.Status, model.AgentRunStatusOK; got != want {
		t.Fatalf("status = %q, want clean run", got)
	}
}

func TestRunAgent_MaxFindingsSkipsRemainingNudgesWhenLimitHit(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("initial", 1, maxFindingsTestFinding("A", 1, 0.8))},
			{resp: nudgeReviewResponse("nudge", 1, maxFindingsTestFinding("N1", 0, 0.9))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), maxFindingsNudgeTestAgent(2), model.ReviewRequest{NudgeCount: 2, MaxOutputRetries: 1, MaxFindings: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "N1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want initial plus one nudge (second skipped at limit)", len(llmClient.reqs))
	}
}

func TestReviewSystemPromptMaxFindingsLine(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	template, err := engine.loadPrompt("agent_review_general_system_prompt.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	limited, err := engine.renderReviewSystemWithFocus(template, "", model.ReviewRequest{MaxFindings: 5}, false, "review", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## FINDING LIMIT", "Report at most 5 findings in total", "5 most critical, highest-confidence ones"} {
		if !strings.Contains(limited, want) {
			t.Fatalf("limited prompt missing %q:\n%s", want, limited)
		}
	}

	unlimited, err := engine.renderReviewSystemWithFocus(template, "", model.ReviewRequest{}, false, "review", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(unlimited, "FINDING LIMIT") {
		t.Fatalf("unlimited prompt contains finding limit section:\n%s", unlimited)
	}
}
