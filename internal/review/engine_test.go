package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/prompts"
	"github.com/google/uuid"
)

type stubSource struct{}

func (stubSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode: model.ModeLocal,
		Repository: model.RepositoryInfo{
			FullName: "repo",
		},
		Title:       "title",
		Description: "description",
		ChangedFiles: []model.ChangedFile{
			{Path: "main.go", Status: model.FileModified, Additions: 1},
		},
		Diff: "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
	}, nil
}

type textSource struct{}

func (textSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "README.txt", Status: model.FileModified, Additions: 1},
		},
		Diff: "diff --git a/README.txt b/README.txt\n@@ -1 +1 @@\n-old\n+new\n",
	}, nil
}

type stubLLM struct{}

func (stubLLM) Review(context.Context, *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	return &llm.ReviewResponse{
		Findings: []model.Finding{
			{Title: "info", Body: "low", ConfidenceScore: 0.5, Priority: intPtr(3), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
			{Title: "error", Body: "high", ConfidenceScore: 0.9, Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "summary",
		OverallConfidenceScore: 0.9,
	}, nil
}

type capturingLLM struct {
	mu    sync.Mutex
	reqs  []*llm.ReviewRequest
	resps []*llm.ReviewResponse
}

type multiAgentLLM struct {
	mu             sync.Mutex
	context        int
	vectorCalls    map[string]int
	verifyCalls    int
	mergeTools     int
	mergePayload   map[string]any
	mergeSchema    []byte
	mergeRequests  []*llm.ReviewRequest
	contextSystem  string
	vectorContext  map[string]string
	vectorSystem   map[string]string
	vectorNudge    map[string]string
	events         []string
	contextFailErr error
	vectorFailErr  map[string]error
	verifyInvalid  map[string]bool
	vectorFindings map[string]int
	mergeResponses []*llm.ReviewResponse
	mergeFailErr   error
}

func (s *multiAgentLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectorCalls == nil {
		s.vectorCalls = make(map[string]int)
	}
	if s.vectorSystem == nil {
		s.vectorSystem = make(map[string]string)
	}
	if s.vectorContext == nil {
		s.vectorContext = make(map[string]string)
	}
	if s.vectorNudge == nil {
		s.vectorNudge = make(map[string]string)
	}
	if req.SchemaKind == llm.SchemaKindVerify {
		s.events = append(s.events, "verify")
		s.verifyCalls++
		valid := true
		for title := range s.verifyInvalid {
			for _, msg := range req.Messages {
				if strings.Contains(msg.Content, title) {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
		}
		verdict := model.VerdictConfirmed
		if !valid {
			verdict = model.VerdictRefuted
		}
		return &llm.ReviewResponse{
			Verification: &model.FindingVerification{Verdict: verdict, Priority: 2, ConfidenceScore: 0.9, Remarks: "verified"},
			TokensUsed:   model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}, nil
	}
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	if strings.Contains(system, "DO NOT produce review findings yourself") {
		s.contextSystem = system
		s.context++
		if s.contextFailErr != nil {
			return nil, s.contextFailErr
		}
		if s.context == 1 {
			return &llm.ReviewResponse{
				ToolCalls: []llm.ToolCall{{ID: "context_call", Name: "list_files", Arguments: `{"path":"internal","depth":1}`}},
			}, nil
		}
		return &llm.ReviewResponse{
			RawResponse: "context inspected internal listing\n\n## Assumed Patch Purpose\nThis is an assumption: the patch appears intended to update review context collection.",
			TokensUsed:  model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}, nil
	}
	if strings.Contains(system, "## FOCUS ON ") {
		name := vectorNameFromSystem(system)
		s.vectorCalls[name]++
		s.vectorSystem[name] = system
		for _, msg := range req.Messages {
			if msg.Role == "user" && strings.Contains(msg.Content, "## Notes") {
				s.vectorContext[name] = msg.Content
			}
			if msg.Role == "user" && strings.Contains(msg.Content, "You may have missed issues") {
				s.vectorNudge[name] = msg.Content
			}
		}
		if err := s.vectorFailErr[name]; err != nil {
			return nil, err
		}
		if s.vectorCalls[name] == 1 {
			return &llm.ReviewResponse{
				ToolCalls: []llm.ToolCall{{ID: "tool_" + name, Name: "inspect_file", Arguments: `{"path":"main.go"}`}},
			}, nil
		}
		count := 1
		if s.vectorFindings != nil && s.vectorFindings[name] > 0 {
			count = s.vectorFindings[name]
		}
		findings := make([]model.Finding, 0, count)
		for i := 0; i < count; i++ {
			title := "Fix " + name
			if i > 0 {
				title = fmt.Sprintf("Fix %s %d", name, i+1)
			}
			findings = append(findings, model.Finding{
				Title:           title,
				Body:            "body",
				ConfidenceScore: 0.9,
				Priority:        intPtr(2),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: i + 1, End: i + 1}},
			})
		}
		return &llm.ReviewResponse{
			Findings:               findings,
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     name,
			OverallConfidenceScore: 0.9,
			TokensUsed:             model.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
		}, nil
	}
	s.mergeTools = len(req.Tools)
	s.events = append(s.events, "merge")
	s.mergeSchema = append([]byte(nil), req.Schema...)
	cloned := *req
	cloned.Messages = cloneTestMessages(req.Messages)
	s.mergeRequests = append(s.mergeRequests, &cloned)
	if len(req.Messages) > 1 {
		_ = json.Unmarshal([]byte(req.Messages[1].Content), &s.mergePayload)
	}
	if s.mergeFailErr != nil {
		return nil, s.mergeFailErr
	}
	if len(s.mergeResponses) > 0 {
		resp := s.mergeResponses[0]
		s.mergeResponses = s.mergeResponses[1:]
		return resp, nil
	}
	return &llm.ReviewResponse{
		Findings: []model.Finding{{
			Title:           "Fix merged issue",
			Body:            "body",
			ConfidenceScore: 0.95,
			Priority:        intPtr(1),
			CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
		}},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "merged",
		OverallConfidenceScore: 0.95,
		TokensUsed:             model.TokenUsage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4},
	}, nil
}

func vectorNameFromSystem(system string) string {
	marker := "## FOCUS ON "
	idx := strings.Index(system, marker)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(system[idx+len(marker):])
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	switch strings.TrimSpace(rest) {
	case "CODE QUALITY AND CORRECTNESS":
		return "Code Quality"
	case "BEST PRACTICES":
		return "Best Practices"
	default:
		return strings.Title(strings.ToLower(strings.TrimSpace(rest)))
	}
}

func (s *capturingLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = cloneTestMessages(req.Messages)
	}
	if len(req.NoToolsMessages) > 0 {
		cloned.NoToolsMessages = cloneTestMessages(req.NoToolsMessages)
	}
	if len(req.Tools) > 0 {
		cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	}
	s.reqs = append(s.reqs, &cloned)
	if len(s.resps) > 0 {
		resp := s.resps[0]
		s.resps = s.resps[1:]
		return resp, nil
	}
	return &llm.ReviewResponse{
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "summary",
		OverallConfidenceScore: 0.5,
	}, nil
}

func cloneTestMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := append([]llm.Message(nil), messages...)
	for i := range cloned {
		if len(messages[i].ToolCalls) > 0 {
			cloned[i].ToolCalls = append([]llm.ToolCall(nil), messages[i].ToolCalls...)
		}
	}
	return cloned
}

func TestRunAgent_NudgeDuplicate(t *testing.T) {
	first := nudgeFinding("A", 1)
	second := nudgeFinding("B", 2)
	duplicate := nudgeFinding("A", 1)
	third := nudgeFinding("C", 3)
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, first)},
			{resp: func() *llm.ReviewResponse {
				resp := nudgeReviewResponse("second", 2, second)
				resp.ReasoningEffort = "low"
				return resp
			}()},
			{resp: nudgeReviewResponse("duplicate", 3, duplicate)},
			{resp: nudgeReviewResponse("third", 4, third)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("reviewer"), model.ReviewRequest{NudgeCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "B", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if result.run.Findings != 3 {
		t.Fatalf("agent run findings = %d", result.run.Findings)
	}
	if result.run.TokensUsed.TotalTokens != 10 {
		t.Fatalf("tokens = %d", result.run.TokensUsed.TotalTokens)
	}
	if len(llmClient.reqs) != 4 {
		t.Fatalf("llm calls = %d, want 4", len(llmClient.reqs))
	}
	wantEfforts := []string{"high", "high", "low", "low"}
	for i, req := range llmClient.reqs {
		if req.ReasoningEffort != wantEfforts[i] {
			t.Fatalf("call %d reasoning effort = %q, want %q", i+1, req.ReasoningEffort, wantEfforts[i])
		}
	}
	if got := llmClient.reqs[1].Messages; len(got) < 4 || !strings.Contains(got[len(got)-1].Content, "missed issues") {
		t.Fatalf("first nudge messages = %#v", got)
	}
}

func TestRunAgent_NudgeKeepDuplicate(t *testing.T) {
	first := nudgeFinding("A", 1)
	changedLocation := nudgeFinding("A", 2)
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, first)},
			{resp: nudgeReviewResponse("changed", 1, changedLocation)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("reviewer"), model.ReviewRequest{NudgeCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
}

func TestRunAgent_NudgeZeroDisables(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, nudgeFinding("A", 1))},
			{resp: nudgeReviewResponse("second", 1, nudgeFinding("B", 2))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("reviewer"), model.ReviewRequest{NudgeCount: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("llm calls = %d, want 1", len(llmClient.reqs))
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
}

func TestRunAgent_NudgeBudgetsResetOnceBeforeNudges(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, nudgeFinding("A", 1))},
			{resp: &llm.ReviewResponse{
				ToolCalls:   []llm.ToolCall{{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`}},
				RawResponse: "inspect first nudge",
			}},
			{resp: nudgeReviewResponse("second", 1, nudgeFinding("B", 2))},
			{resp: &llm.ReviewResponse{
				ToolCalls:   []llm.ToolCall{{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"main.go"}`}},
				RawResponse: "inspect second nudge",
			}},
			{resp: nudgeReviewResponse("third", 1, nudgeFinding("C", 3))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestToolAgent("reviewer"), model.ReviewRequest{NudgeCount: 2, MaxToolCalls: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.run.ToolCalls != 1 {
		t.Fatalf("tool calls = %d, want one shared nudge-phase budget", result.run.ToolCalls)
	}
	if len(result.toolMessages) != 1 {
		t.Fatalf("tool messages = %d, want only first nudge tool execution", len(result.toolMessages))
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "B", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 5 {
		t.Fatalf("llm calls = %d, want 5", len(llmClient.reqs))
	}
	if len(llmClient.reqs[4].Tools) != 0 {
		t.Fatalf("second nudge final call should run without tools, got %d tools", len(llmClient.reqs[4].Tools))
	}
}

func TestRunAgent_NudgeReviewerOnly(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("context", 1)},
			{resp: nudgeReviewResponse("unused", 1, nudgeFinding("B", 2))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	_, err := engine.runAgent(context.Background(), nudgeTestAgent("context"), model.ReviewRequest{NudgeCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("llm calls = %d, want 1", len(llmClient.reqs))
	}
}

func TestRunAgent_ReasoningExtractorAugmentsNudges(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	llmClient := &reasoningExtractLLM{
		reviewerReasoning: []string{
			"initial reasoning only",
			"nudge one reasoning only",
			"nudge two reasoning only",
		},
		collectOutputs: []string{
			"collect-initial",
		},
		updateOutputs: []string{
			"delta from initial",
			"delta from nudge one",
		},
		firstCollectStarted: started,
		releaseFirstCollect: release,
	}
	engine := nudgeTestEngine(llmClient)

	type runResult struct {
		result agentResult
		err    error
	}
	done := make(chan runResult, 1)
	go func() {
		result, err := engine.runAgent(context.Background(), nudgeTestAgent("reviewer"), model.ReviewRequest{
			NudgeCount:          2,
			ModelEmitsReasoning: true,
		})
		done <- runResult{result: result, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("collect did not start")
	}
	time.Sleep(20 * time.Millisecond)
	llmClient.mu.Lock()
	reviewerCallsBeforeRelease := len(llmClient.reviewerMessages)
	updateCallsBeforeRelease := len(llmClient.updateFullLists)
	llmClient.mu.Unlock()
	if reviewerCallsBeforeRelease != 1 {
		t.Fatalf("reviewer calls before collect release = %d, want 1", reviewerCallsBeforeRelease)
	}
	if updateCallsBeforeRelease != 0 {
		t.Fatalf("update findings calls before collect release = %d, want 0", updateCallsBeforeRelease)
	}
	close(release)

	var got runResult
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runAgent did not finish")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}

	llmClient.mu.Lock()
	collectInputs := append([]string(nil), llmClient.collectInputs...)
	updateFullLists := append([]string(nil), llmClient.updateFullLists...)
	updateFindingsJSON := append([]string(nil), llmClient.updateFindingsJSON...)
	reviewerMessages := append([]string(nil), llmClient.reviewerMessages...)
	llmClient.mu.Unlock()

	if want := []string{"initial reasoning only"}; !reflect.DeepEqual(collectInputs, want) {
		t.Fatalf("collect inputs = %#v, want %#v", collectInputs, want)
	}
	if want := []string{"collect-initial", "collect-initial"}; !reflect.DeepEqual(updateFullLists, want) {
		t.Fatalf("update findings lists = %#v, want %#v", updateFullLists, want)
	}
	if len(updateFindingsJSON) != 2 || !strings.Contains(updateFindingsJSON[0], "Initial finding") || !strings.Contains(updateFindingsJSON[1], "Nudge 1 finding") {
		t.Fatalf("update findings JSON = %#v", updateFindingsJSON)
	}
	if len(reviewerMessages) != 3 {
		t.Fatalf("reviewer messages = %d, want 3", len(reviewerMessages))
	}
	if !strings.Contains(reviewerMessages[1], "delta from initial") {
		t.Fatalf("first nudge missing reasoning delta: %q", reviewerMessages[1])
	}
	if !strings.Contains(reviewerMessages[2], "delta from nudge one") {
		t.Fatalf("second nudge missing reasoning delta: %q", reviewerMessages[2])
	}
	if got.result.run.TokensUsed.TotalTokens == 0 {
		t.Fatal("extractor token usage should be folded into reviewer run")
	}
}

func TestRunAgent_ReasoningExtractorVerboseLogsFindingsWithContext(t *testing.T) {
	llmClient := &reasoningExtractLLM{
		reviewerReasoning: []string{"initial reasoning only"},
		collectOutputs:    []string{"collected issue\nsecond collected issue"},
		updateOutputs:     []string{"reduced issue"},
	}
	engine := nudgeTestEngine(llmClient)
	var buf lockedTestBuffer
	engine.SetLogger(logging.New(&buf, true, false))

	_, err := engine.runAgent(context.Background(), nudgeTestAgent("reviewer"), model.ReviewRequest{
		NudgeCount:          1,
		ModelEmitsReasoning: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	for _, want := range []string{
		"[reasoning_extract: reasoning-extract:reviewer:collect:turn-1] Extracted reasoning findings:",
		"collected issue",
		"second collected issue",
		"[reviewer: reviewer - Nudge 1/1] Extracted reasoning findings sent to nudge:",
		"- reduced issue",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("verbose log missing %q:\n%s", want, got)
		}
	}
}

type lockedTestBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedTestBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedTestBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunAgent_PerIterCollectInsideInitialReviewer(t *testing.T) {
	llmClient := &multiIterReasoningExtractLLM{
		reviewerReasoning: []string{
			"iter 1 reasoning",
			"iter 2 reasoning",
			"nudge reasoning must not extract",
		},
		collectOutputs: []string{"iter-1-list", "iter-2-list"},
		updateOutputs:  []string{"delta after nudge"},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestToolAgent("reviewer"), model.ReviewRequest{
		NudgeCount:          1,
		ModelEmitsReasoning: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	llmClient.mu.Lock()
	collectInputs := append([]string(nil), llmClient.collectInputs...)
	updateFullLists := append([]string(nil), llmClient.updateFullLists...)
	reviewerCallNum := llmClient.reviewerCallNum
	reviewerMessages := append([]string(nil), llmClient.reviewerMessages...)
	llmClient.mu.Unlock()

	sort.Strings(collectInputs)
	if want := []string{"iter 1 reasoning", "iter 2 reasoning"}; !reflect.DeepEqual(collectInputs, want) {
		t.Fatalf("collect inputs = %#v, want %#v (nudge iter must not fire collect)", collectInputs, want)
	}
	if len(updateFullLists) != 1 {
		t.Fatalf("update findings calls = %d, want 1", len(updateFullLists))
	}
	gotLines := strings.Split(updateFullLists[0], "\n")
	sort.Strings(gotLines)
	if want := []string{"iter-1-list", "iter-2-list"}; !reflect.DeepEqual(gotLines, want) {
		t.Fatalf("update findings combined list lines = %#v, want %#v", gotLines, want)
	}
	if reviewerCallNum != 3 {
		t.Fatalf("reviewer LLM calls = %d, want 3 (iter1+iter2+nudge1)", reviewerCallNum)
	}
	if len(reviewerMessages) < 3 || !strings.Contains(reviewerMessages[2], "delta after nudge") {
		t.Fatalf("nudge message missing update findings delta: %#v", reviewerMessages)
	}
	if result.run.TokensUsed.TotalTokens == 0 {
		t.Fatal("extractor token usage should be folded into reviewer run")
	}
}

func TestRunAgent_NudgeErrorKeepsPriorFindingsAsPartial(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, nudgeFinding("A", 1))},
			{resp: nudgeReviewResponse("second", 1, nudgeFinding("B", 2))},
			{err: errors.New("boom")},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("reviewer"), model.ReviewRequest{NudgeCount: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := result.run.Status, model.AgentRunStatusPartial; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if !strings.Contains(result.run.Error, "nudge 2") {
		t.Fatalf("run.Error = %q, want containing %q", result.run.Error, "nudge 2")
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("llm calls = %d, want 3", len(llmClient.reqs))
	}
}

func TestAppendNewFindingsDuplicateKeys(t *testing.T) {
	base := nudgeFinding("Same", 1)
	sameIDSameTitle := nudgeFinding(" same ", 2)
	sameIDSameTitle.ID = base.ID
	sameIDDifferentTitle := nudgeFinding("Different", 1)
	sameIDDifferentTitle.ID = base.ID
	sameTitleSameLocation := nudgeFinding("SAME", 1)
	sameTitleDifferentLocation := nudgeFinding("Same", 3)

	got := appendNewFindings([]model.Finding{base}, []model.Finding{
		sameIDSameTitle,
		sameIDDifferentTitle,
		sameTitleSameLocation,
		sameTitleDifferentLocation,
	})
	if gotTitles, want := findingTitles(got), []string{"Same", "Different", "Same"}; !reflect.DeepEqual(gotTitles, want) {
		t.Fatalf("findings = %#v, want %#v", gotTitles, want)
	}
}

func TestFormatReasoningFindingsList(t *testing.T) {
	got := formatReasoningFindingsList("first possible issue\n\n- already listed\n  second possible issue  ")
	want := "- first possible issue\n- already listed\n- second possible issue"
	if got != want {
		t.Fatalf("formatted findings = %q, want %q", got, want)
	}
}

func TestFormatReasoningFindingsListNormalizesBullets(t *testing.T) {
	input := strings.Join([]string{
		"* asterisk bullet",
		"+ plus bullet",
		"-no space after dash",
		"1. numbered dot",
		"12) numbered paren",
		"  * indented asterisk",
		"-",
		"plain line",
	}, "\n")
	want := strings.Join([]string{
		"- asterisk bullet",
		"- plus bullet",
		"- no space after dash",
		"- numbered dot",
		"- numbered paren",
		"- indented asterisk",
		"- plain line",
	}, "\n")
	if got := formatReasoningFindingsList(input); got != want {
		t.Fatalf("formatted findings = %q, want %q", got, want)
	}
}

func nudgeTestEngine(llmClient llm.Client) *Engine {
	return &Engine{
		llm:       llmClient,
		retrieval: stubRetrieval{},
		config: config.Profile{
			Model:           "test-model",
			ReasoningEffort: "high",
		},
	}
}

func nudgeTestAgent(role string) agentSpec {
	return agentSpec{
		name:       role,
		role:       role,
		system:     "system",
		user:       "user",
		schemaKind: llm.SchemaKindReview,
		hasTools:   false,
	}
}

func nudgeTestToolAgent(role string) agentSpec {
	agent := nudgeTestAgent(role)
	agent.hasTools = true
	return agent
}

func nudgeReviewResponse(label string, tokens int, findings ...model.Finding) *llm.ReviewResponse {
	return &llm.ReviewResponse{
		Findings:               findings,
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     label,
		OverallConfidenceScore: 0.9,
		RawResponse:            `{"label":"` + label + `"}`,
		TokensUsed:             model.TokenUsage{PromptTokens: tokens, CompletionTokens: tokens, TotalTokens: tokens},
	}
}

func nudgeFinding(title string, line int) model.Finding {
	return model.Finding{
		ID:              uuid.NewString(),
		Title:           title,
		Body:            "body",
		ConfidenceScore: 0.9,
		Priority:        intPtr(2),
		CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: line, End: line}},
	}
}

func findingTitles(findings []model.Finding) []string {
	titles := make([]string, 0, len(findings))
	for _, finding := range findings {
		titles = append(titles, finding.Title)
	}
	return titles
}

type stubRetrieval struct{}

func (stubRetrieval) GetFile(context.Context, string, string) (*retrieval.FileContent, error) {
	return &retrieval.FileContent{
		Path:     "extra.go",
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (stubRetrieval) ListFiles(context.Context, string, string, int) (*retrieval.DirectoryListing, error) {
	return &retrieval.DirectoryListing{
		Path:  "pkg",
		Files: []string{"pkg/a.go", "pkg/b.go"},
	}, nil
}

func (stubRetrieval) Search(context.Context, string, string, string, int, int, bool) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{
		Path:          "",
		Query:         "match",
		ContextLines:  5,
		CaseSensitive: false,
		ResultCount:   1,
		Results: []retrieval.SearchResult{
			{Path: "pkg/a.go", StartLine: 10, EndLine: 20, Language: "go", Content: "before\nmatch\nafter"},
		},
	}, nil
}

func (stubRetrieval) SearchRegex(context.Context, string, string, *regexp.Regexp, int, int) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{
		Path:         "",
		ContextLines: 5,
		ResultCount:  0,
		Results:      []retrieval.SearchResult{},
	}, nil
}

type countingRetrieval struct {
	mu               sync.Mutex
	paths            []string
	literalResults   []retrieval.SearchResult
	regexResults     []retrieval.SearchResult
	hasCustomResults bool
}

func (r *countingRetrieval) GetFile(_ context.Context, _ string, path string) (*retrieval.FileContent, error) {
	r.mu.Lock()
	r.paths = append(r.paths, path)
	r.mu.Unlock()
	return &retrieval.FileContent{
		Path:     path,
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (r *countingRetrieval) ListFiles(_ context.Context, _ string, path string, depth int) (*retrieval.DirectoryListing, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("list:%s:%d", path, depth))
	r.mu.Unlock()
	return &retrieval.DirectoryListing{
		Path:  path,
		Files: []string{path + "/a.go", path + "/b.go"},
	}, nil
}

func (r *countingRetrieval) Search(_ context.Context, _ string, path, query string, contextLines, maxResults int, caseSensitive bool) (*retrieval.SearchResults, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("search:%s:%s:%d:%d:%t", path, query, contextLines, maxResults, caseSensitive))
	custom := r.hasCustomResults
	customResults := r.literalResults
	r.mu.Unlock()
	var results []retrieval.SearchResult
	if custom {
		results = customResults
	} else {
		results = []retrieval.SearchResult{
			{Path: path + "/a.go", StartLine: 10, EndLine: 20, Language: "go", Content: "before\n" + query + "\nafter"},
		}
		if query == "missing" {
			results = nil
		}
	}
	return &retrieval.SearchResults{
		Path:          path,
		Query:         query,
		ContextLines:  contextLines,
		MaxResults:    maxResults,
		CaseSensitive: caseSensitive,
		ResultCount:   len(results),
		Results:       results,
	}, nil
}

func (r *countingRetrieval) SearchRegex(_ context.Context, _ string, path string, pattern *regexp.Regexp, contextLines, maxResults int) (*retrieval.SearchResults, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("search_regex:%s:%s:%d:%d", path, pattern.String(), contextLines, maxResults))
	results := r.regexResults
	r.mu.Unlock()
	if results == nil {
		results = []retrieval.SearchResult{}
	}
	return &retrieval.SearchResults{
		Path:         path,
		Query:        pattern.String(),
		ContextLines: contextLines,
		MaxResults:   maxResults,
		ResultCount:  len(results),
		Results:      results,
	}, nil
}

func (*countingRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return &retrieval.FileSlice{
		Path:      "extra.go",
		StartLine: 1,
		EndLine:   2,
		Content:   "package extra",
		Language:  "go",
	}, nil
}

func (*countingRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, errors.New("unexpected GetSymbol call")
}

func (r *countingRetrieval) FindCallers(_ context.Context, _ string, symbol retrieval.SymbolRef, depth int) (*retrieval.CallHierarchy, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("callers:%s:%s:%d", symbol.Path, symbol.Name, depth))
	r.mu.Unlock()
	return &retrieval.CallHierarchy{
		Mode:  "callers",
		Depth: depth,
		Root: retrieval.CallNode{
			Name:      symbol.Name,
			Path:      pathOrDefault(symbol.Path, "pkg/root.go"),
			StartLine: 10,
			EndLine:   12,
			Source:    "func Run() {}",
			Children: []retrieval.CallNode{
				{
					Name:      "Start",
					Path:      "pkg/caller.go",
					StartLine: 20,
					EndLine:   24,
					Source:    "func Start() {}",
				},
			},
		},
	}, nil
}

func (r *countingRetrieval) FindCallees(_ context.Context, _ string, symbol retrieval.SymbolRef, depth int) (*retrieval.CallHierarchy, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("callees:%s:%s:%d", symbol.Path, symbol.Name, depth))
	r.mu.Unlock()
	return &retrieval.CallHierarchy{
		Mode:  "callees",
		Depth: depth,
		Root: retrieval.CallNode{
			Name:      symbol.Name,
			Path:      pathOrDefault(symbol.Path, "pkg/root.go"),
			StartLine: 10,
			EndLine:   12,
			Source:    "func Run() {}",
			Children: []retrieval.CallNode{
				{
					Name:      "Helper",
					Path:      "pkg/callee.go",
					StartLine: 30,
					EndLine:   34,
					Source:    "func Helper() {}",
				},
			},
		},
	}, nil
}

func (stubRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return &retrieval.FileSlice{
		Path:      "extra.go",
		StartLine: 1,
		EndLine:   2,
		Content:   "package extra",
		Language:  "go",
	}, nil
}

func (stubRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, errors.New("unexpected GetSymbol call")
}

func (stubRetrieval) FindCallers(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallers call")
}

func (stubRetrieval) FindCallees(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallees call")
}

func TestReviewerToolDefinitionsComeFromCatalogInStableOrder(t *testing.T) {
	definitions := reviewerToolDefinitions()
	wantNames := []string{"inspect_file", "list_files", "search", "find_callers", "find_callees"}
	if len(definitions) != len(wantNames) {
		t.Fatalf("tool definitions = %d", len(definitions))
	}
	for i, wantName := range wantNames {
		if definitions[i].Name != wantName {
			t.Fatalf("tool[%d].Name = %q, want %q", i, definitions[i].Name, wantName)
		}
		if definitions[i].Description == "" {
			t.Fatalf("tool[%d] missing description", i)
		}
	}
}

func TestReviewerToolDefinitionsContainValidCatalogSchemas(t *testing.T) {
	definitions := reviewerToolDefinitions()
	requiredByTool := map[string][]string{
		"inspect_file": []string{"path"},
		"search":       []string{"query"},
		"find_callers": []string{"symbol"},
		"find_callees": []string{"symbol"},
	}
	for _, definition := range definitions {
		var schema map[string]any
		if err := json.Unmarshal(definition.Parameters, &schema); err != nil {
			t.Fatalf("%s schema should be valid JSON: %v", definition.Name, err)
		}
		if schema["type"] != "object" {
			t.Fatalf("%s schema type = %#v", definition.Name, schema["type"])
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("%s schema additionalProperties = %#v", definition.Name, schema["additionalProperties"])
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok || len(properties) == 0 {
			t.Fatalf("%s schema missing properties: %#v", definition.Name, schema["properties"])
		}
		if definition.Name == "search" {
			maxResults, ok := properties["max_results"].(map[string]any)
			if !ok {
				t.Fatalf("search schema missing max_results property: %#v", properties)
			}
			if maxResults["minimum"] != float64(0) {
				t.Fatalf("search max_results minimum = %#v, want 0", maxResults["minimum"])
			}
		}
		required := map[string]bool{}
		for _, value := range schemaStringSlice(schema["required"]) {
			required[value] = true
		}
		for _, name := range requiredByTool[definition.Name] {
			if !required[name] {
				t.Fatalf("%s schema missing required field %q: %#v", definition.Name, name, schema["required"])
			}
		}
	}
}

func TestToolInstructionsTemplateUsesGeneratedListing(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, nil, config.Profile{})
	rendered, err := engine.renderToolInstructions(toolInstructionsConfig{kind: "review", parallelToolCallGuidance: true})
	if err != nil {
		t.Fatal(err)
	}
	listing := toolInstructionsListing()
	if !strings.Contains(rendered, listing) {
		t.Fatalf("tool instructions missing generated listing:\n%s", rendered)
	}
	if !strings.Contains(listing, "- `search` tool with a repo-relative `path` and a `query`") {
		t.Fatalf("generated listing missing search tool: %q", listing)
	}
}

func TestToolErrorMessagesComeFromCatalog(t *testing.T) {
	tests := []struct {
		name string
		data toolErrorData
		want string
	}{
		{
			name: "retrieval unavailable",
			data: toolErrorData{Code: "retrieval_unavailable"},
			want: "retrieval is unavailable for this review",
		},
		{
			name: "unsupported tool",
			data: toolErrorData{Code: "unsupported_tool", ToolName: "bad_tool"},
			want: `unsupported tool "bad_tool"`,
		},
		{
			name: "missing argument",
			data: toolErrorData{Code: "missing_argument", Argument: "query", Schema: toolArgumentSchema("search")},
			want: `missing required argument: query; expected {"path"?: "<repo-relative path>", "query": "<text>", "context_lines"?: int, "max_results"?: int, "case_sensitive"?: bool}`,
		},
		{
			name: "already requested file",
			data: toolErrorData{Code: "already_requested_file"},
			want: "file contents were already provided for this review",
		},
		{
			name: "already requested tool",
			data: toolErrorData{Code: "already_requested_tool"},
			want: "tool result was already provided for this review",
		},
		{
			name: "encoding failed",
			data: toolErrorData{Code: "encoding_failed"},
			want: "failed to encode tool result",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolErrorMessage(tt.data); got != tt.want {
				t.Fatalf("toolErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSyntheticToolFollowupRendersBranches(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, nil, config.Profile{})
	baseHistory := []toolCallHistoryEntry{
		{
			ToolCall: llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run(","context_lines":0}`},
			Result:   toolResultSummary{Lines: 2, Files: 1},
		},
	}
	optimizedHistory := []toolCallHistoryEntry{
		{
			ToolCall:    llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run(","context_lines":0}`},
			Result:      toolResultSummary{Lines: 2, Files: 1},
			OptimizedTo: "find_callers",
		},
	}
	errorHistory := []toolCallHistoryEntry{
		{
			ToolCall: llm.ToolCall{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"missing.go"}`},
			Result:   toolResultSummary{IsError: true, Code: "retrieval_failed", Message: "not found"},
		},
	}

	retry, err := engine.renderSyntheticToolFollowup(errorHistory, "review")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(retry, "Please retry the last tool call.") {
		t.Fatalf("retry follow-up = %q", retry)
	}

	contextRendered, err := engine.renderSyntheticToolFollowup(baseHistory, "context")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contextRendered, "return the summary of what you believe the patch is intended to do") {
		t.Fatalf("context follow-up = %q", contextRendered)
	}

	reviewRendered, err := engine.renderSyntheticToolFollowup(optimizedHistory, "review")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reviewRendered, "1. search (replaced by find_callers): tool_call_id=\"call_1\"") {
		t.Fatalf("review follow-up missing structured history: %q", reviewRendered)
	}
	if !strings.Contains(reviewRendered, "return the requested JSON format") {
		t.Fatalf("review follow-up = %q", reviewRendered)
	}

	verifyRendered, err := engine.renderSyntheticToolFollowup(baseHistory, "verify")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(verifyRendered, "return the requested JSON format") || !strings.Contains(verifyRendered, "verification") {
		t.Fatalf("verify follow-up = %q", verifyRendered)
	}
}

func TestEngineRunsContextVectorsMergeWithIndependentToolBudgets(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, trimmed, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Title != "Fix merged issue" {
		t.Fatalf("merged findings = %#v", result.Findings)
	}
	if len(result.AgentRuns) != 8 {
		t.Fatalf("agent runs = %d, want context + 6 reviewers + merge", len(result.AgentRuns))
	}
	if result.TotalToolCalls != 7 {
		t.Fatalf("tool calls = %d, want context + one per vector", result.TotalToolCalls)
	}
	if llmClient.verifyCalls != len(reviewVectors) {
		t.Fatalf("verify calls = %d, want one per vector finding", llmClient.verifyCalls)
	}
	if result.VerifyTokensUsed.TotalTokens != len(reviewVectors)*2 {
		t.Fatalf("verify tokens = %d, want %d", result.VerifyTokensUsed.TotalTokens, len(reviewVectors)*2)
	}
	if got := llmClient.events[len(llmClient.events)-1]; got != "merge" {
		t.Fatalf("last event = %q, want merge after verification", got)
	}
	if len(trimmed.SupplementalContext) == 0 {
		t.Fatal("context was not attached")
	}
	if trimmed.SupplementalContext[0].Kind != "context_tool_result" {
		t.Fatalf("first supplemental context = %#v, want context tool result", trimmed.SupplementalContext[0])
	}
	if !strings.Contains(llmClient.contextSystem, "DO NOT produce review findings yourself") {
		t.Fatalf("context prompt missing standalone instructions: %q", llmClient.contextSystem)
	}
	if strings.Contains(llmClient.contextSystem, "Make sure to output the findings") {
		t.Fatalf("context prompt should not include review output instructions: %q", llmClient.contextSystem)
	}
	if llmClient.mergeTools != 0 {
		t.Fatalf("merge tools = %d, want 0", llmClient.mergeTools)
	}
	if _, ok := llmClient.mergePayload["context_response"]; ok {
		t.Fatalf("merge payload should not include context_response: %#v", llmClient.mergePayload)
	}
	if notes, _ := llmClient.mergePayload["context_agent_notes"].(string); !strings.Contains(notes, "## Notes") {
		t.Fatalf("merge payload context_agent_notes = %#v", llmClient.mergePayload["context_agent_notes"])
	}
	vectorReviews, ok := llmClient.mergePayload["vector_reviews"].([]any)
	if !ok || len(vectorReviews) != 6 {
		t.Fatalf("merge payload vector_reviews = %#v", llmClient.mergePayload["vector_reviews"])
	}
	for _, vector := range reviewVectors {
		if llmClient.vectorCalls[vector.name] != 2 {
			t.Fatalf("%s calls = %d, want tool + final", vector.name, llmClient.vectorCalls[vector.name])
		}
		system := llmClient.vectorSystem[vector.name]
		if !strings.Contains(system, "You are acting as a senior engineer performing a thorough code review") {
			t.Fatalf("%s prompt lost base review prompt", vector.name)
		}
		if !strings.Contains(system, "## FOCUS ON ") {
			t.Fatalf("%s prompt missing focus snippet", vector.name)
		}
		if !strings.Contains(system, "- Go Style Guide") {
			t.Fatalf("%s prompt missing styleguide reminder: %q", vector.name, system)
		}
		if !strings.Contains(system, "provided `toolchain_versions`") {
			t.Fatalf("%s prompt missing toolchain reminder: %q", vector.name, system)
		}
		contextNote := llmClient.vectorContext[vector.name]
		if !strings.Contains(contextNote, "## Notes") || !strings.Contains(contextNote, "## Assumed Patch Purpose") {
			t.Fatalf("%s prompt missing context agent markdown note: %q", vector.name, contextNote)
		}
	}
}

func TestMultiAgentVerifiesBeforeMergeAndDropsInvalidFindings(t *testing.T) {
	llmClient := &multiAgentLLM{
		verifyInvalid: map[string]bool{"Fix Security": true},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		RepoRoot:          ".",
		MaxContextTokens:  1000,
		MaxToolCalls:      1,
		VerifyConcurrency: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if llmClient.verifyCalls != len(reviewVectors) {
		t.Fatalf("verify calls = %d, want %d", llmClient.verifyCalls, len(reviewVectors))
	}
	for i, event := range llmClient.events {
		if event == "merge" && i != len(llmClient.events)-1 {
			t.Fatalf("merge event at %d before verification complete: %#v", i, llmClient.events)
		}
	}
	vectorReviews, ok := llmClient.mergePayload["vector_reviews"].([]any)
	if !ok {
		t.Fatalf("merge payload vector_reviews = %#v", llmClient.mergePayload["vector_reviews"])
	}
	var mergedFindingCount int
	var sawSecurity bool
	for _, raw := range vectorReviews {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("vector review entry = %#v", raw)
		}
		findings, _ := entry["findings"].([]any)
		mergedFindingCount += len(findings)
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				t.Fatalf("finding = %#v", rawFinding)
			}
			if finding["title"] == "Fix Security" {
				sawSecurity = true
			}
			if _, ok := finding["verification"].(map[string]any); !ok {
				t.Fatalf("finding missing verification in merge input: %#v", finding)
			}
		}
	}
	if mergedFindingCount != len(reviewVectors)-1 {
		t.Fatalf("merge input findings = %d, want %d", mergedFindingCount, len(reviewVectors)-1)
	}
	if sawSecurity {
		t.Fatal("invalid Security finding reached merge input")
	}
}

func TestMultiAgentMergeValidationRetriesWhenBelowLargestReviewerCount(t *testing.T) {
	llmClient := &multiAgentLLM{
		vectorFindings: map[string]int{"Code Quality": 3},
		mergeResponses: []*llm.ReviewResponse{
			{
				Findings:           []model.Finding{mergeTestFinding("Fix Code Quality", 1)},
				OverallCorrectness: "patch is incorrect",
			},
			{
				Findings: []model.Finding{
					mergeTestFinding("Fix Code Quality", 1),
					mergeTestFinding("Fix Code Quality 2", 2),
					mergeTestFinding("Fix Code Quality 3", 3),
				},
				OverallCorrectness:     "patch is incorrect",
				OverallExplanation:     "merged",
				OverallConfidenceScore: 0.9,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
		MaxOutputRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.mergeRequests) != 2 {
		t.Fatalf("merge requests = %d, want retry", len(llmClient.mergeRequests))
	}
	retryMessage := llmClient.mergeRequests[1].Messages[len(llmClient.mergeRequests[1].Messages)-1].Content
	if !strings.Contains(retryMessage, "3") {
		t.Fatalf("retry message missing merge count nudge: %q", retryMessage)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("findings = %d, want retry merge output with 3 findings", len(result.Findings))
	}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "Merge validation warning") {
			t.Fatalf("unexpected merge validation warning after successful retry: %#v", result.Warnings)
		}
	}
}

func TestMultiAgentMergeValidationWarnsAfterRetryExhausted(t *testing.T) {
	ghost := model.Finding{
		Title:           "Ghost",
		Body:            "new",
		ConfidenceScore: 0.8,
		Priority:        intPtr(2),
		CodeLocation:    model.CodeLocation{FilePath: "ghost.go", LineRange: model.LineRange{Start: 99, End: 99}},
	}
	llmClient := &multiAgentLLM{
		mergeResponses: []*llm.ReviewResponse{
			{Findings: []model.Finding{ghost}, OverallCorrectness: "patch is incorrect"},
			{Findings: []model.Finding{ghost}, OverallCorrectness: "patch is incorrect"},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
		MaxOutputRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.mergeRequests) != 2 {
		t.Fatalf("merge requests = %d, want initial + retry", len(llmClient.mergeRequests))
	}
	foundWarning := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "Merge validation warning") && strings.Contains(warning, "unmatched_finding") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("warnings = %#v, want nonfatal merge validation warning", result.Warnings)
	}
	if len(result.Findings) != 1 || result.Findings[0].Title != "Ghost" {
		t.Fatalf("merge output should be accepted after exhausted retry, got %#v", result.Findings)
	}
}

func TestMergeValidationAllowsDuplicatesAndNoUpperBound(t *testing.T) {
	input := mergeTestFinding("Fix Code Quality", 1)
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{input}},
		run:  model.AgentRun{Status: model.AgentRunStatusOK},
	}}
	resp := &llm.ReviewResponse{Findings: []model.Finding{input, input, input}}

	if invalid := validateMergeResponse(resp, vectorResults); invalid != nil {
		t.Fatalf("validateMergeResponse returned %v, want nil for duplicate/high-count output", invalid)
	}
}

func mergeTestFinding(title string, line int) model.Finding {
	return model.Finding{
		Title:           title,
		Body:            "body",
		ConfidenceScore: 0.9,
		Priority:        intPtr(2),
		CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: line, End: line}},
	}
}

func TestEngineVectorNudgeRepeatsReviewerQuestions(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
		NudgeCount:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"Code Quality": "- Does it work correctly?",
		"Security":     "- Are there security concerns?",
		"Testing":      "- Are changed behaviors covered by focused tests?",
	}
	for name, want := range checks {
		nudge := llmClient.vectorNudge[name]
		if !strings.Contains(nudge, "Ask yourself these original questions again:") {
			t.Fatalf("%s nudge missing question header: %q", name, nudge)
		}
		if !strings.Contains(nudge, want) {
			t.Fatalf("%s nudge missing question %q: %q", name, want, nudge)
		}
	}
}

func TestMultiAgentToleratesVectorFailure(t *testing.T) {
	llmClient := &multiAgentLLM{
		vectorFailErr: map[string]error{"Security": errors.New("security upstream fail")},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	var securityRun *model.AgentRun
	successfulReviewers := 0
	for i := range result.AgentRuns {
		run := &result.AgentRuns[i]
		if run.Name == "Security" {
			securityRun = run
			continue
		}
		if run.Role == "reviewer" && run.Status == model.AgentRunStatusOK {
			successfulReviewers++
		}
	}
	if securityRun == nil {
		t.Fatal("missing Security AgentRun")
	}
	if securityRun.Status != model.AgentRunStatusFailed {
		t.Fatalf("Security status = %q, want failed", securityRun.Status)
	}
	if !strings.Contains(securityRun.Error, "security upstream fail") {
		t.Fatalf("Security run error = %q", securityRun.Error)
	}
	if successfulReviewers != 5 {
		t.Fatalf("successful reviewer runs = %d, want 5", successfulReviewers)
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "Security reviewer failed") && strings.Contains(w, "security upstream fail") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected Security warning in %#v", result.Warnings)
	}
}

func TestMultiAgentToleratesContextFailure(t *testing.T) {
	llmClient := &multiAgentLLM{contextFailErr: errors.New("context upstream fail")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "Context agent failed") && strings.Contains(w, "context upstream fail") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected context warning in %#v", result.Warnings)
	}
	for _, name := range []string{"Code Quality", "Security", "Architecture", "Performance", "Testing", "Best Practices"} {
		if llmClient.vectorContext[name] != "" {
			t.Fatalf("%s reviewer received non-empty context notes: %q", name, llmClient.vectorContext[name])
		}
	}
}

func TestMultiAgentToleratesMergeFailure(t *testing.T) {
	llmClient := &multiAgentLLM{mergeFailErr: errors.New("merge upstream fail")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected fallback findings from vector union")
	}
	if !strings.Contains(result.OverallExplanation, "Merge agent unavailable") {
		t.Fatalf("OverallExplanation = %q", result.OverallExplanation)
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "Merge agent failed") && strings.Contains(w, "merge upstream fail") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected merge warning in %#v", result.Warnings)
	}
	var mergeRun *model.AgentRun
	for i := range result.AgentRuns {
		if result.AgentRuns[i].Role == "merge" {
			mergeRun = &result.AgentRuns[i]
		}
	}
	if mergeRun == nil || mergeRun.Status != model.AgentRunStatusFailed {
		t.Fatalf("merge AgentRun = %#v, want Status=failed", mergeRun)
	}
}

func TestMultiAgentToleratesAllVectorsFailing(t *testing.T) {
	llmClient := &multiAgentLLM{
		vectorFailErr: map[string]error{
			"Code Quality":   errors.New("cq fail"),
			"Security":       errors.New("sec fail"),
			"Architecture":   errors.New("arch fail"),
			"Performance":    errors.New("perf fail"),
			"Testing":        errors.New("test fail"),
			"Best Practices": errors.New("bp fail"),
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	failedReviewers := 0
	for _, run := range result.AgentRuns {
		if run.Role == "reviewer" && run.Status == model.AgentRunStatusFailed {
			failedReviewers++
		}
	}
	if failedReviewers != 6 {
		t.Fatalf("failed reviewer runs = %d, want 6", failedReviewers)
	}
	if len(result.Warnings) < 6 {
		t.Fatalf("warnings = %d, want at least 6", len(result.Warnings))
	}
	// All-vectors-failed must short-circuit the merge LLM call to avoid
	// hallucinated findings from an empty payload.
	foundShortCircuit := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "All vector reviewers failed") && strings.Contains(w, "skipped merge agent") {
			foundShortCircuit = true
		}
	}
	if !foundShortCircuit {
		t.Fatalf("expected short-circuit warning in %#v", result.Warnings)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected empty findings on short-circuit, got %d", len(result.Findings))
	}
	var mergeRun *model.AgentRun
	for i := range result.AgentRuns {
		if result.AgentRuns[i].Role == "merge" {
			mergeRun = &result.AgentRuns[i]
		}
	}
	if mergeRun == nil || mergeRun.Status != model.AgentRunStatusSkipped {
		t.Fatalf("merge AgentRun = %#v, want Status=skipped", mergeRun)
	}
}

func TestReviewerQuestionsRenderFromSeparateTemplates(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	baseTemplate, err := prompts.Load("agent_review_general_system_prompt.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, vector := range reviewVectors {
		focusTemplate, err := prompts.Load(vector.focusFile)
		if err != nil {
			t.Fatal(err)
		}
		questionsTemplate, err := prompts.Load(vector.questionsFile)
		if err != nil {
			t.Fatal(err)
		}
		for _, question := range strings.Split(questionsTemplate, "\n") {
			question = strings.TrimSpace(question)
			if question == "" {
				continue
			}
			if strings.Contains(focusTemplate, question) {
				t.Fatalf("%s focus template still contains question %q", vector.name, question)
			}
		}
		questionsSnippet, err := engine.renderReviewerQuestionsSnippet(vector.questionsFile)
		if err != nil {
			t.Fatal(err)
		}
		system, err := engine.renderReviewSystemWithQuestions(baseTemplate, vector.focusFile, questionsSnippet, model.ReviewRequest{}, false, "reviewer", nil, false)
		if err != nil {
			t.Fatal(err)
		}
		firstQuestion := strings.TrimSpace(strings.Split(strings.TrimSpace(questionsTemplate), "\n")[0])
		if !strings.Contains(system, firstQuestion) {
			t.Fatalf("%s rendered system missing first question %q", vector.name, firstQuestion)
		}
		if strings.Contains(system, "{{.QuestionsSnippet}}") {
			t.Fatalf("%s rendered system contains unresolved questions placeholder", vector.name)
		}
	}
}

func TestEngineMergeSchemaHonorsPriorityThreshold(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, _, err := engine.RunWithContext(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		RepoRoot:          ".",
		MaxContextTokens:  1000,
		UseJSONSchema:     true,
		PriorityThreshold: "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.mergeSchema) == 0 {
		t.Fatal("merge schema empty")
	}
	priority := schemaFindingProperty(t, llmClient.mergeSchema, "priority")
	if got := priority["minimum"]; got != float64(0) {
		t.Fatalf("priority minimum = %#v, want 0", got)
	}
	if got := priority["maximum"]; got != float64(1) {
		t.Fatalf("priority maximum = %#v, want 1", got)
	}
}

func schemaFindingProperty(t *testing.T, schema []byte, property string) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(schema, &root); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	properties := root["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	findingProps := items["properties"].(map[string]any)
	out, ok := findingProps[property].(map[string]any)
	if !ok {
		t.Fatalf("finding property %q missing: %#v", property, findingProps[property])
	}
	return out
}

type blockingRetrieval struct {
	started chan string
	release chan struct{}
}

func (r *blockingRetrieval) GetFile(_ context.Context, _ string, path string) (*retrieval.FileContent, error) {
	r.started <- path
	<-r.release
	return &retrieval.FileContent{
		Path:     path,
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (blockingRetrieval) ListFiles(context.Context, string, string, int) (*retrieval.DirectoryListing, error) {
	return nil, errors.New("unexpected ListFiles call")
}

func (blockingRetrieval) SearchRegex(context.Context, string, string, *regexp.Regexp, int, int) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{Results: []retrieval.SearchResult{}}, nil
}

func (blockingRetrieval) Search(context.Context, string, string, string, int, int, bool) (*retrieval.SearchResults, error) {
	return nil, errors.New("unexpected Search call")
}

func (blockingRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return nil, errors.New("unexpected GetFileSlice call")
}

func (blockingRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, errors.New("unexpected GetSymbol call")
}

func (blockingRetrieval) FindCallers(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallers call")
}

func (blockingRetrieval) FindCallees(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallees call")
}

func TestEngineExecutesIndependentToolCallsConcurrently(t *testing.T) {
	retrievalEngine := &blockingRetrieval{
		started: make(chan string, 2),
		release: make(chan struct{}),
	}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}

	done := make(chan []llm.Message, 1)
	go func() {
		done <- engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
			{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"a.go"}`},
			{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"b.go"}`},
		}, state)
	}()

	started := map[string]struct{}{}
	for len(started) < 2 {
		path := <-retrievalEngine.started
		started[path] = struct{}{}
	}
	close(retrievalEngine.release)

	results := <-done
	if len(results) != 2 {
		t.Fatalf("tool messages = %d", len(results))
	}
	if results[0].ToolCallID != "call_1" || results[1].ToolCallID != "call_2" {
		t.Fatalf("tool message order = %#v", results)
	}
}

func TestEngineDedupesDuplicateToolCallsWithinParallelRound(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}

	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
		{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"./extra.go"}`},
	}, state)

	if len(results) != 2 {
		t.Fatalf("tool messages = %d", len(results))
	}
	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d", len(retrievalEngine.paths))
	}
	var firstPayload map[string]any
	if err := json.Unmarshal([]byte(results[0].Content), &firstPayload); err != nil {
		t.Fatalf("first tool payload should be valid json: %v", err)
	}
	if firstPayload["path"] != "extra.go" {
		t.Fatalf("first tool payload = %#v", firstPayload)
	}
	var secondPayload map[string]any
	if err := json.Unmarshal([]byte(results[1].Content), &secondPayload); err != nil {
		t.Fatalf("second tool payload should be valid json: %v", err)
	}
	if secondPayload["status"] != "error" {
		t.Fatalf("duplicate tool payload = %#v", secondPayload)
	}
	errorPayload, ok := secondPayload["error"].(map[string]any)
	if !ok {
		t.Fatalf("duplicate error payload = %#v", secondPayload)
	}
	if errorPayload["code"] != "already_requested" {
		t.Fatalf("duplicate error code = %#v", errorPayload["code"])
	}
}

func pathOrDefault(path, fallback string) string {
	if path == "" {
		return fallback
	}
	return path + "/root.go"
}

type scriptedLLM struct {
	reqs    []*llm.ReviewRequest
	results []scriptedLLMResult
}

type scriptedLLMResult struct {
	resp *llm.ReviewResponse
	err  error
}

func (s *scriptedLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = cloneTestMessages(req.Messages)
	}
	if len(req.NoToolsMessages) > 0 {
		cloned.NoToolsMessages = cloneTestMessages(req.NoToolsMessages)
	}
	if len(req.Tools) > 0 {
		cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	}
	s.reqs = append(s.reqs, &cloned)
	if len(s.results) == 0 {
		return &llm.ReviewResponse{
			OverallCorrectness:     "patch is correct",
			OverallExplanation:     "ok",
			OverallConfidenceScore: 0.5,
		}, nil
	}
	next := s.results[0]
	s.results = s.results[1:]
	return next.resp, next.err
}

type reasoningExtractLLM struct {
	mu                      sync.Mutex
	reviewerReasoning       []string
	reviewerMessages        []string
	collectInputs           []string
	collectOutputs          []string
	updateFullLists         []string
	updateFindingsJSON      []string
	updateOutputs           []string
	firstCollectStarted     chan struct{}
	releaseFirstCollect     chan struct{}
	firstCollectStartedOnce sync.Once
}

func (s *reasoningExtractLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	user := lastUserContent(req.Messages)
	switch {
	case isReasoningCollectPrompt(system):
		input := collectPayloadReasoning(user)
		s.mu.Lock()
		idx := len(s.collectInputs)
		s.collectInputs = append(s.collectInputs, input)
		output := outputAt(s.collectOutputs, idx)
		started := s.firstCollectStarted
		release := s.releaseFirstCollect
		s.mu.Unlock()
		if idx == 0 && started != nil {
			s.firstCollectStartedOnce.Do(func() { close(started) })
		}
		if idx == 0 && release != nil {
			<-release
		}
		return textResponse(output, 1), nil
	case isReasoningUpdatePrompt(system):
		fullList, findingsJSON := updatePayloadParts(user)
		s.mu.Lock()
		idx := len(s.updateFullLists)
		s.updateFullLists = append(s.updateFullLists, fullList)
		s.updateFindingsJSON = append(s.updateFindingsJSON, findingsJSON)
		output := outputAt(s.updateOutputs, idx)
		s.mu.Unlock()
		return textResponse(output, 1), nil
	default:
		s.mu.Lock()
		idx := len(s.reviewerMessages)
		s.reviewerMessages = append(s.reviewerMessages, user)
		reasoning := outputAt(s.reviewerReasoning, idx)
		s.mu.Unlock()
		if req.ReasoningSink != nil {
			req.ReasoningSink.Append(reasoning)
		}
		title := "Initial finding"
		if idx > 0 {
			title = fmt.Sprintf("Nudge %d finding", idx)
		}
		resp := nudgeReviewResponse(title, 10, nudgeFinding(title, idx+1))
		resp.Reasoned = true
		return resp, nil
	}
}

type multiIterReasoningExtractLLM struct {
	mu                sync.Mutex
	reviewerCallNum   int
	reviewerMessages  []string
	reviewerReasoning []string
	collectInputs     []string
	collectOutputs    []string
	updateFullLists   []string
	updateOutputs     []string
}

func (s *multiIterReasoningExtractLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	user := lastUserContent(req.Messages)
	switch {
	case isReasoningCollectPrompt(system):
		input := collectPayloadReasoning(user)
		s.mu.Lock()
		idx := len(s.collectInputs)
		s.collectInputs = append(s.collectInputs, input)
		output := outputAt(s.collectOutputs, idx)
		s.mu.Unlock()
		return textResponse(output, 1), nil
	case isReasoningUpdatePrompt(system):
		fullList, _ := updatePayloadParts(user)
		s.mu.Lock()
		idx := len(s.updateFullLists)
		s.updateFullLists = append(s.updateFullLists, fullList)
		output := outputAt(s.updateOutputs, idx)
		s.mu.Unlock()
		return textResponse(output, 1), nil
	default:
		s.mu.Lock()
		idx := s.reviewerCallNum
		s.reviewerCallNum++
		s.reviewerMessages = append(s.reviewerMessages, user)
		reasoning := outputAt(s.reviewerReasoning, idx)
		s.mu.Unlock()
		if req.ReasoningSink != nil {
			req.ReasoningSink.Append(reasoning)
		}
		if idx == 0 {
			return &llm.ReviewResponse{
				RawResponse: "tool-call-iter-1",
				ToolCalls: []llm.ToolCall{{
					ID:        "iter1",
					Name:      "inspect_file",
					Arguments: `{"path":"main.go"}`,
				}},
				Reasoned:   true,
				TokensUsed: model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 1},
			}, nil
		}
		title := fmt.Sprintf("Finding %d", idx)
		resp := nudgeReviewResponse(title, 10, nudgeFinding(title, idx+1))
		resp.Reasoned = true
		return resp, nil
	}
}

func textResponse(content string, tokens int) *llm.ReviewResponse {
	return &llm.ReviewResponse{
		RawResponse: content,
		TokensUsed:  model.TokenUsage{PromptTokens: tokens, CompletionTokens: tokens, TotalTokens: tokens},
	}
}

func outputAt(values []string, idx int) string {
	if idx < len(values) {
		return values[idx]
	}
	return "NONE"
}

func lastUserContent(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func isReasoningCollectPrompt(system string) bool {
	return strings.Contains(system, "extracting possible findings in the notes")
}

func isReasoningUpdatePrompt(system string) bool {
	return strings.Contains(system, "making sure that all findings noted by one engineer")
}

func collectPayloadReasoning(content string) string {
	const notesStart = "Notes made by an engineer while doing a thorough code review:"
	if _, rest, ok := strings.Cut(content, notesStart); ok {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(strings.TrimPrefix(content, "Reviewer reasoning trace:"))
}

func updatePayloadParts(content string) (string, string) {
	listStart := "List of findings noted down by one engineer:"
	findingsStart := "JSON with the findings of a code review done by another engineer:"
	if !strings.Contains(content, listStart) || !strings.Contains(content, findingsStart) {
		listStart = "Extracted issue list:"
		findingsStart = "Current findings JSON:"
	}
	_, rest, _ := strings.Cut(content, listStart)
	list, findings, _ := strings.Cut(rest, findingsStart)
	return strings.TrimSpace(list), strings.TrimSpace(findings)
}

func schemaStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok {
			result = append(result, text)
		}
	}
	return result
}

func TestShouldDropFinding(t *testing.T) {
	cases := []struct {
		name       string
		verdict    string
		confidence float64
		policy     string
		threshold  float64
		wantDrop   bool
		wantReason string
	}{
		{"confirmed never drops (refuted-only)", model.VerdictConfirmed, 0.99, "refuted-only", 0.8, false, "kept"},
		{"confirmed never drops (both)", model.VerdictConfirmed, 0.99, "refuted-and-unverified", 0.8, false, "kept"},
		{"refuted above floor drops", model.VerdictRefuted, 0.85, "refuted-only", 0.8, true, model.VerdictRefuted},
		{"refuted at floor drops", model.VerdictRefuted, 0.80, "refuted-only", 0.8, true, model.VerdictRefuted},
		{"refuted below floor kept", model.VerdictRefuted, 0.79, "refuted-only", 0.8, false, "below_confidence"},
		{"unverified kept (refuted-only)", model.VerdictUnverified, 0.95, "refuted-only", 0.8, false, "kept"},
		{"unverified above floor drops (both)", model.VerdictUnverified, 0.9, "refuted-and-unverified", 0.8, true, model.VerdictUnverified},
		{"unverified below floor kept (both)", model.VerdictUnverified, 0.5, "refuted-and-unverified", 0.8, false, "below_confidence"},
		{"refuted policy=none kept", model.VerdictRefuted, 0.99, "none", 0.8, false, "kept"},
		{"missing verdict treated as unverified (refuted-only)", "", 0.99, "refuted-only", 0.8, false, "kept"},
		{"missing verdict treated as unverified (both)", "", 0.99, "refuted-and-unverified", 0.8, true, model.VerdictUnverified},
		{"bogus policy defaults to refuted-only behavior", model.VerdictRefuted, 0.9, "garbage", 0.8, true, model.VerdictRefuted},
		{"threshold zero drops anything refuted", model.VerdictRefuted, 0.0, "refuted-only", 0.0, true, model.VerdictRefuted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &model.FindingVerification{Verdict: tc.verdict, ConfidenceScore: tc.confidence}
			drop, reason := shouldDropFinding(v, tc.policy, tc.threshold)
			if drop != tc.wantDrop {
				t.Fatalf("drop = %v, want %v", drop, tc.wantDrop)
			}
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestShouldDropFindingNilVerification(t *testing.T) {
	drop, reason := shouldDropFinding(nil, "refuted-and-unverified", 0.0)
	if drop || reason != "kept" {
		t.Fatalf("nil verification: drop=%v reason=%q", drop, reason)
	}
}

func TestNormalizeDropPolicyFallback(t *testing.T) {
	for _, p := range []string{"", "garbage", "REFUTED-ONLY"} {
		if got := normalizeDropPolicy(p); got != "refuted-only" {
			t.Fatalf("normalizeDropPolicy(%q) = %q, want refuted-only", p, got)
		}
	}
	for _, p := range []string{"none", "refuted-only", "refuted-and-unverified"} {
		if got := normalizeDropPolicy(p); got != p {
			t.Fatalf("normalizeDropPolicy(%q) = %q, want passthrough", p, got)
		}
	}
}

func TestValidateDropPolicy(t *testing.T) {
	for _, p := range []string{"none", "refuted-only", "refuted-and-unverified"} {
		if err := ValidateDropPolicy(p); err != nil {
			t.Fatalf("ValidateDropPolicy(%q) = %v, want nil", p, err)
		}
	}
	for _, p := range []string{"", "garbage", "REFUTED-ONLY", "refuted_only"} {
		if err := ValidateDropPolicy(p); err == nil {
			t.Fatalf("ValidateDropPolicy(%q) = nil, want error", p)
		}
	}
}
