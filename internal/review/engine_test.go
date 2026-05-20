package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
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
	mergeTools     int
	mergePayload   map[string]any
	mergeSchema    []byte
	contextSystem  string
	vectorContext  map[string]string
	vectorSystem   map[string]string
	vectorNudge    map[string]string
	contextFailErr error
	vectorFailErr  map[string]error
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
		return &llm.ReviewResponse{
			Findings: []model.Finding{{
				Title:           "Fix " + name,
				Body:            "body",
				ConfidenceScore: 0.9,
				Priority:        intPtr(2),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
			}},
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     name,
			OverallConfidenceScore: 0.9,
			TokensUsed:             model.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
		}, nil
	}
	s.mergeTools = len(req.Tools)
	s.mergeSchema = append([]byte(nil), req.Schema...)
	if len(req.Messages) > 1 {
		_ = json.Unmarshal([]byte(req.Messages[1].Content), &s.mergePayload)
	}
	if s.mergeFailErr != nil {
		return nil, s.mergeFailErr
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
		phaseAOutputs: []string{
			"phase-a-initial",
			"phase-a-nudge-one",
			"phase-a-nudge-two",
		},
		phaseBOutputs: []string{
			"delta from initial",
			"delta from nudge one",
		},
		firstPhaseAStarted: started,
		releaseFirstPhaseA: release,
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
		t.Fatal("phase A did not start")
	}
	time.Sleep(20 * time.Millisecond)
	llmClient.mu.Lock()
	reviewerCallsBeforeRelease := len(llmClient.reviewerMessages)
	phaseBCallsBeforeRelease := len(llmClient.phaseBFullLists)
	llmClient.mu.Unlock()
	if reviewerCallsBeforeRelease != 1 {
		t.Fatalf("reviewer calls before phase A release = %d, want 1", reviewerCallsBeforeRelease)
	}
	if phaseBCallsBeforeRelease != 0 {
		t.Fatalf("phase B calls before phase A release = %d, want 0", phaseBCallsBeforeRelease)
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
	phaseAInputs := append([]string(nil), llmClient.phaseAInputs...)
	phaseBFullLists := append([]string(nil), llmClient.phaseBFullLists...)
	phaseBFindingsJSON := append([]string(nil), llmClient.phaseBFindingsJSON...)
	reviewerMessages := append([]string(nil), llmClient.reviewerMessages...)
	llmClient.mu.Unlock()

	if want := []string{"initial reasoning only", "nudge one reasoning only", "nudge two reasoning only"}; !reflect.DeepEqual(phaseAInputs, want) {
		t.Fatalf("phase A inputs = %#v, want %#v", phaseAInputs, want)
	}
	if want := []string{"phase-a-initial", "phase-a-initial\nphase-a-nudge-one"}; !reflect.DeepEqual(phaseBFullLists, want) {
		t.Fatalf("phase B lists = %#v, want %#v", phaseBFullLists, want)
	}
	if len(phaseBFindingsJSON) != 2 || !strings.Contains(phaseBFindingsJSON[0], "Initial finding") || !strings.Contains(phaseBFindingsJSON[1], "Nudge 1 finding") {
		t.Fatalf("phase B findings JSON = %#v", phaseBFindingsJSON)
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

func TestEngineReusesEffectiveReasoningEffort(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
				RawResponse:     "inspecting",
				ReasoningEffort: "low",
				TokensUsed:      model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.8,
				ReasoningEffort:        "low",
				TokensUsed:             model.TokenUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{
		Model:           "model",
		ReasoningEffort: "high",
	})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		RepoRoot:          ".",
		MaxContextTokens:  10000,
		PriorityThreshold: "p3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want 2", len(llmClient.reqs))
	}
	if llmClient.reqs[0].ReasoningEffort != "high" {
		t.Fatalf("first reasoning effort = %q", llmClient.reqs[0].ReasoningEffort)
	}
	if llmClient.reqs[1].ReasoningEffort != "low" {
		t.Fatalf("second reasoning effort = %q", llmClient.reqs[1].ReasoningEffort)
	}
	if result.ReasoningEffort != "low" {
		t.Fatalf("result reasoning effort = %q", result.ReasoningEffort)
	}
	if result.TokensUsed.TotalTokens != 7 {
		t.Fatalf("total tokens = %d", result.TokensUsed.TotalTokens)
	}
}

func TestEnginePriorityFilter(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		MaxContextTokens:  1000,
		PriorityThreshold: "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d", len(result.Findings))
	}
}

func TestEngineAssignsFindingIDs(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		MaxContextTokens:  1000,
		PriorityThreshold: "p3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(result.Findings))
	}
	seen := map[string]bool{}
	for _, finding := range result.Findings {
		if _, err := uuid.Parse(finding.ID); err != nil {
			t.Fatalf("finding ID %q is not a UUID: %v", finding.ID, err)
		}
		if seen[finding.ID] {
			t.Fatalf("duplicate finding ID %q", finding.ID)
		}
		seen[finding.ID] = true
	}
}

func TestEnginePreservesExistingFindingID(t *testing.T) {
	const existingID = "11111111-1111-4111-8111-111111111111"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{{
			Findings: []model.Finding{{
				ID:              existingID,
				Title:           "x",
				Body:            "y",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
			}},
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     "summary",
			OverallConfidenceScore: 0.8,
		}},
	}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		MaxContextTokens:  1000,
		PriorityThreshold: "p3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Findings[0].ID; got != existingID {
		t.Fatalf("id = %q, want %q", got, existingID)
	}
}

func TestEngineSplitsSystemAndUserPrompts(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}

	req := llmClient.reqs[0]
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[0].Content == req.Messages[1].Content {
		t.Fatal("system and user prompts should differ")
	}
	if req.Messages[0].Content == "" || req.Messages[1].Content == "" {
		t.Fatal("system and user prompts should both be populated")
	}
	if want := "You are acting as a senior engineer performing a thorough code review for a proposed code change made by another engineer."; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt = %q", req.Messages[0].Content)
	}
	if want := "`inspect_file` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing tool instructions: %q", req.Messages[0].Content)
	}
	if want := "`list_files` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing list-files instructions: %q", req.Messages[0].Content)
	}
	if want := "`find_callers` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing callers instructions: %q", req.Messages[0].Content)
	}
	if want := "`find_callees` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing callees instructions: %q", req.Messages[0].Content)
	}
	if want := "call multiple tools in parallel"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing parallel guidance: %q", req.Messages[0].Content)
	}
	if want := "generate one or more `suggestions` entries"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing suggestion instructions: %q", req.Messages[0].Content)
	}
	if want := "Make sure to output the findings in the following JSON format:"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing example JSON instructions: %q", req.Messages[0].Content)
	}
	if want := "\"overall_correctness\": \"patch is correct\""; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing rendered example JSON: %q", req.Messages[0].Content)
	}
	if want := "\"title\": \"Example title\""; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing example finding JSON: %q", req.Messages[0].Content)
	}
	if strings.Contains(req.Messages[0].Content, "[P1] Example title") {
		t.Fatalf("system prompt should not include priority prefix in example title: %q", req.Messages[0].Content)
	}
	if want := "\"suggestions\""; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing example suggestion JSON: %q", req.Messages[0].Content)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &payload); err != nil {
		t.Fatalf("user prompt should be valid json: %v\n%s", err, req.Messages[1].Content)
	}
	repository, ok := payload["repository"].(map[string]any)
	if !ok {
		t.Fatalf("user prompt missing repository object: %#v", payload)
	}
	if repository["full_name"] != "repo" {
		t.Fatalf("repository.full_name = %#v", repository["full_name"])
	}
	if payload["title"] != "title" {
		t.Fatalf("title = %#v", payload["title"])
	}
	if _, ok := payload["changed_files"]; !ok {
		t.Fatalf("user prompt missing changed_files: %#v", payload)
	}
	styleGuides, ok := payload["style_guides"].([]any)
	if !ok || len(styleGuides) != 1 {
		t.Fatalf("user prompt missing Go style guide: %#v", payload["style_guides"])
	}
	goStyleGuide := styleGuides[0].(map[string]any)
	if goStyleGuide["language"] != "go" {
		t.Fatalf("style guide language = %#v", goStyleGuide["language"])
	}
	if content, _ := goStyleGuide["content"].(string); !strings.Contains(content, "# Go Style Guide") {
		t.Fatalf("style guide content = %.80q", content)
	}
	for _, unwanted := range []string{"mode", "checkout_root", "diff"} {
		if _, exists := payload[unwanted]; exists {
			t.Fatalf("user prompt unexpectedly contains %q: %#v", unwanted, payload[unwanted])
		}
	}
	if len(req.Tools) != 5 {
		t.Fatalf("tools = %d", len(req.Tools))
	}
	if req.Tools[0].Name != "inspect_file" {
		t.Fatalf("tool name = %q", req.Tools[0].Name)
	}
	if req.Tools[1].Name != "list_files" {
		t.Fatalf("tool name = %q", req.Tools[1].Name)
	}
	if req.Tools[2].Name != "search" {
		t.Fatalf("tool name = %q", req.Tools[2].Name)
	}
	if req.Tools[3].Name != "find_callers" {
		t.Fatalf("tool name = %q", req.Tools[3].Name)
	}
	if req.Tools[4].Name != "find_callees" {
		t.Fatalf("tool name = %q", req.Tools[4].Name)
	}
	if !req.ParallelToolCalls {
		t.Fatal("parallel tool calls should be enabled by default")
	}
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

func TestEngineCanDisableParallelToolCallsAndGuidance(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:                     model.ModeLocal,
		MaxContextTokens:         1000,
		DisableParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := llmClient.reqs[0]
	if req.ParallelToolCalls {
		t.Fatal("parallel tool calls should be disabled")
	}
	if strings.Contains(req.Messages[0].Content, "call all required tools in the same turn rather than serializing them") {
		t.Fatalf("system prompt should omit parallel guidance: %q", req.Messages[0].Content)
	}
}

func TestEngineRunsContextVectorsMergeWithIndependentToolBudgets(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetMultiAgentReview(true)

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
		contextNote := llmClient.vectorContext[vector.name]
		if !strings.Contains(contextNote, "## Notes") || !strings.Contains(contextNote, "## Assumed Patch Purpose") {
			t.Fatalf("%s prompt missing context agent markdown note: %q", vector.name, contextNote)
		}
	}
}

func TestEngineVectorNudgeRepeatsReviewerQuestions(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetMultiAgentReview(true)

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
	engine.SetMultiAgentReview(true)

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
	engine.SetMultiAgentReview(true)

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
	engine.SetMultiAgentReview(true)

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
	engine.SetMultiAgentReview(true)

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
		system, err := engine.renderReviewSystemWithQuestions(baseTemplate, vector.focusFile, questionsSnippet, model.ReviewRequest{}, false)
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
	engine.SetMultiAgentReview(true)

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

func TestEngineDoesNotUseAPISchemaByDefault(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[0].Schema) != 0 {
		t.Fatalf("schema should be empty by default, got %s", string(llmClient.reqs[0].Schema))
	}
}

func TestEngineExecutesInspectFileToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				Findings: []model.Finding{
					{Title: "error", Body: "high", ConfidenceScore: 0.9, Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 2, End: 2}}},
				},
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
				TokensUsed:             model.TokenUsage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if result.TotalToolCalls != 1 {
		t.Fatalf("tool calls = %d", result.TotalToolCalls)
	}
	if got := result.TokensUsed.TotalTokens; got != 10 {
		t.Fatalf("total tokens = %d", got)
	}

	req := llmClient.reqs[1]
	if len(req.Messages) != 5 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[2].Role != "assistant" {
		t.Fatalf("assistant role = %q", req.Messages[2].Role)
	}
	if len(req.Messages[2].ToolCalls) != 1 || req.Messages[2].ToolCalls[0].Name != "inspect_file" {
		t.Fatalf("assistant tool calls = %#v", req.Messages[2].ToolCalls)
	}
	if req.Messages[3].Role != "tool" {
		t.Fatalf("tool role = %q", req.Messages[3].Role)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v\n%s", err, req.Messages[3].Content)
	}
	if payload["path"] != "extra.go" || payload["content"] != "package extra" || payload["language"] != "go" {
		t.Fatalf("tool payload = %#v", payload)
	}
	if req.Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", req.Messages[4].Role)
	}
	if want := "You used the following tools up to now:"; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", req.Messages[4].Content)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"extra.go\"]; result=[lines=1]"; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", req.Messages[4].Content)
	}
	if want := "Continue calling tools, if the provided context is insufficient."; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up missing continue instruction: %q", req.Messages[4].Content)
	}
	if want := "If you have enough context for the code review, stop calling tools and return the requested JSON format."; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up missing stop instruction: %q", req.Messages[4].Content)
	}
}

func TestEngineDropsInvalidToolCallsFromHistory(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_empty_name", Name: "", Arguments: `{"path":"extra.go"}`},
					{ID: "call_bad_json", Name: "inspect_file", Arguments: `no json here`},
					{ID: "call_unknown", Name: "unknown_tool", Arguments: `{"path":"extra.go"}`},
					{ID: "call_missing_required", Name: "search", Arguments: `{"path":"."}`},
					{ID: "call_valid", Name: "inspect_file", Arguments: `{"path":"extra.go"}}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     10,
		MaxOutputRetries: defaultMaxOutputRetries,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalToolCalls != 1 {
		t.Fatalf("tool calls = %d", result.TotalToolCalls)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	req := llmClient.reqs[1]
	if len(req.Messages) != 5 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	assistantMessage := req.Messages[2]
	if len(assistantMessage.ToolCalls) != 1 {
		t.Fatalf("assistant tool calls = %#v", assistantMessage.ToolCalls)
	}
	if assistantMessage.ToolCalls[0].ID != "call_valid" {
		t.Fatalf("assistant tool calls = %#v", assistantMessage.ToolCalls)
	}
	if assistantMessage.ToolCalls[0].Arguments != `{"path":"extra.go"}` {
		t.Fatalf("assistant tool call arguments = %q", assistantMessage.ToolCalls[0].Arguments)
	}
	if req.Messages[3].Role != "tool" || req.Messages[3].ToolCallID != "call_valid" {
		t.Fatalf("tool message = %#v", req.Messages[3])
	}
	synthetic := req.Messages[4].Content
	for _, forbidden := range []string{"call_empty_name", "call_bad_json", "call_unknown", "call_missing_required", "unsupported tool", "unknown_tool"} {
		if strings.Contains(synthetic, forbidden) {
			t.Fatalf("synthetic follow-up mentions dropped call %q: %s", forbidden, synthetic)
		}
	}
	if !strings.Contains(synthetic, "call_valid") {
		t.Fatalf("synthetic follow-up missing valid call: %s", synthetic)
	}
}

func TestEngineRetriesInvalidOnlyToolCallsWithoutHistory(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_unknown", Name: "unknown_tool", Arguments: `{"path":"extra.go"}`},
				},
				RawResponse: "bad tool call",
				TokensUsed:  model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
				TokensUsed:             model.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     10,
		MaxOutputRetries: defaultMaxOutputRetries,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalToolCalls != 0 {
		t.Fatalf("tool calls = %d", result.TotalToolCalls)
	}
	if result.TokensUsed.TotalTokens != 5 {
		t.Fatalf("total tokens = %d", result.TokensUsed.TotalTokens)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[1].Messages) != len(llmClient.reqs[0].Messages)+1 {
		t.Fatalf("retry should preserve regular assistant history: first=%d second=%d", len(llmClient.reqs[0].Messages), len(llmClient.reqs[1].Messages))
	}
	last := llmClient.reqs[1].Messages[len(llmClient.reqs[1].Messages)-1]
	if last.Role != "assistant" || last.Content != "bad tool call" || len(last.ToolCalls) != 0 {
		t.Fatalf("retry assistant history = %#v", last)
	}
}

func TestEngineProvidesNoToolsMessagesWithoutSyntheticFollowup(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				RawResponse: "I'll inspect extra.go first.",
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}

	req := llmClient.reqs[1]
	if len(req.Messages) < 5 || !strings.Contains(req.Messages[4].Content, "You used the following tools up to now") {
		t.Fatalf("normal messages should include synthetic follow-up: %#v", req.Messages)
	}
	noTools := req.NoToolsMessages
	if len(noTools) == 0 {
		t.Fatal("expected no-tools messages")
	}
	if strings.Contains(noTools[0].Content, "`inspect_file` tool") {
		t.Fatalf("no-tools system prompt should omit tool instructions: %q", noTools[0].Content)
	}
	if !strings.Contains(noTools[0].Content, "OUTPUT FORMAT") {
		t.Fatalf("no-tools system prompt missing review instructions: %q", noTools[0].Content)
	}

	foundAssistantContent := false
	foundConvertedToolResult := false
	for _, msg := range noTools {
		if strings.Contains(msg.Content, "You called the following tools already") {
			t.Fatalf("no-tools messages should omit synthetic follow-up: %#v", noTools)
		}
		if msg.Role == "tool" {
			t.Fatalf("no-tools messages should convert tool roles: %#v", msg)
		}
		if len(msg.ToolCalls) > 0 {
			t.Fatalf("no-tools messages should strip assistant tool calls: %#v", msg)
		}
		if msg.Role == "assistant" && msg.Content == "I'll inspect extra.go first." {
			foundAssistantContent = true
		}
		if msg.Role == "user" && strings.Contains(msg.Content, `"path":"extra.go"`) {
			foundConvertedToolResult = true
		}
	}
	if !foundAssistantContent {
		t.Fatalf("no-tools messages missing assistant content: %#v", noTools)
	}
	if !foundConvertedToolResult {
		t.Fatalf("no-tools messages missing converted tool result: %#v", noTools)
	}
}

func TestEnginePreservesAssistantContentWithToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				RawResponse: "I'll inspect the extra file before deciding.",
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	assistantMessage := llmClient.reqs[1].Messages[2]
	if assistantMessage.Role != "assistant" {
		t.Fatalf("assistant role = %q", assistantMessage.Role)
	}
	if assistantMessage.Content != "I'll inspect the extra file before deciding." {
		t.Fatalf("assistant content = %q", assistantMessage.Content)
	}
	if len(assistantMessage.ToolCalls) != 1 || assistantMessage.ToolCalls[0].Name != "inspect_file" {
		t.Fatalf("assistant tool calls = %#v", assistantMessage.ToolCalls)
	}
}

func TestEnginePreservesAssistantContentWhenToolLimitFinalizes(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				RawResponse: "I'll inspect extra.go first.",
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				RawResponse: "I still want to inspect main.go before finalizing.",
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"main.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(llmClient.reqs) != 3 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	finalReq := llmClient.reqs[2]
	if len(finalReq.Tools) != 0 {
		t.Fatalf("final call should have no tools, got %d", len(finalReq.Tools))
	}
	if len(finalReq.Messages) < 5 {
		t.Fatalf("final messages = %d", len(finalReq.Messages))
	}
	if finalReq.Messages[2].Role != "assistant" || finalReq.Messages[2].Content != "I'll inspect extra.go first." {
		t.Fatalf("first assistant message = %#v", finalReq.Messages[2])
	}
	lastMessage := finalReq.Messages[len(finalReq.Messages)-1]
	if lastMessage.Role != "assistant" {
		t.Fatalf("last message role = %q", lastMessage.Role)
	}
	if lastMessage.Content != "I still want to inspect main.go before finalizing." {
		t.Fatalf("last assistant content = %q", lastMessage.Content)
	}
	if len(lastMessage.ToolCalls) != 0 {
		t.Fatalf("last assistant message should not include tool calls, got %#v", lastMessage.ToolCalls)
	}
}

func TestEngineExecutesInspectFileToolCallsWithLineRange(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go","line_start":4,"line_end":8}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := llmClient.reqs[1]
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v\n%s", err, req.Messages[3].Content)
	}
	if payload["path"] != "extra.go" || payload["content"] != "package extra" || payload["language"] != "go" {
		t.Fatalf("tool payload = %#v", payload)
	}
	if payload["start_line"] != float64(1) || payload["end_line"] != float64(2) {
		t.Fatalf("tool payload line range = %#v", payload)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"extra.go\", line_start=4, line_end=8]; result=[lines=1]"; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", req.Messages[4].Content)
	}
}

func TestEngineUsesAPISchemaWhenEnabled(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		UseJSONSchema:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if string(llmClient.reqs[0].Schema) != string(llm.FindingsSchema) {
		t.Fatalf("schema = %s", string(llmClient.reqs[0].Schema))
	}
	if strings.Contains(llmClient.reqs[0].Messages[0].Content, "Example JSON output:") {
		t.Fatalf("system prompt should omit example snippet when API schema is enabled: %q", llmClient.reqs[0].Messages[0].Content)
	}
}

func TestEngineReturnsToolErrorsToModel(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, nil, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("tool error status = %#v", payload["status"])
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("tool error payload = %#v", payload)
	}
	if errorPayload["code"] != "retrieval_unavailable" {
		t.Fatalf("tool error code = %#v", errorPayload["code"])
	}
	message, _ := errorPayload["message"].(string)
	if !strings.HasPrefix(message, "retrieval is unavailable") {
		t.Fatalf("tool error payload = %#v", payload)
	}
	if llmClient.reqs[1].Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", llmClient.reqs[1].Messages[4].Role)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"extra.go\"]; error=\"retrieval is unavailable"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
	if want := "Please retry the last tool call."; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up missing retry instruction: %q", llmClient.reqs[1].Messages[4].Content)
	}
	if strings.Contains(llmClient.reqs[1].Messages[4].Content, "If you need more context, continue calling tools.") {
		t.Fatalf("follow-up should not include regular continuation instructions after retryable error: %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineStopsAtToolRoundLimit(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"main.go"}`},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[2].Tools) != 0 {
		t.Fatalf("final call should have no tools, got %d", len(llmClient.reqs[2].Tools))
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if result.TotalToolCalls != 1 {
		t.Fatalf("tool calls = %d", result.TotalToolCalls)
	}
}

func TestEngineCountsParallelToolCallsIndividuallyForLimit(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"main.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[1].Tools) != 0 {
		t.Fatalf("final call should have no tools, got %d", len(llmClient.reqs[1].Tools))
	}
	if result.TotalToolCalls != 0 {
		t.Fatalf("tool calls = %d", result.TotalToolCalls)
	}
	if len(retrievalEngine.paths) != 0 {
		t.Fatalf("retrieval should not run when batch exceeds limit: %#v", retrievalEngine.paths)
	}
}

func TestEngineStopsAtDuplicateToolCallLimit(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"./extra.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:                  model.ModeLocal,
		MaxContextTokens:      1000,
		MaxToolCalls:          3,
		MaxDuplicateToolCalls: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[2].Tools) != 0 {
		t.Fatalf("final call should have no tools, got %d", len(llmClient.reqs[2].Tools))
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if result.TotalToolCalls != 2 {
		t.Fatalf("tool calls = %d", result.TotalToolCalls)
	}
	if len(result.AgentRuns) != 1 {
		t.Fatalf("agent runs = %d", len(result.AgentRuns))
	}
	if result.AgentRuns[0].DuplicateToolCalls != 1 {
		t.Fatalf("duplicate tool calls = %d", result.AgentRuns[0].DuplicateToolCalls)
	}
	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d", len(retrievalEngine.paths))
	}
}

func TestEngineReturnsErrorForDuplicateFileRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"./extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d", len(retrievalEngine.paths))
	}
	if retrievalEngine.paths[0] != "extra.go" {
		t.Fatalf("retrieval path = %q", retrievalEngine.paths[0])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[2].Messages[5].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("duplicate tool payload = %#v", payload)
	}
	if payload["path"] != "extra.go" {
		t.Fatalf("duplicate tool path = %#v", payload["path"])
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("duplicate tool error payload = %#v", payload)
	}
	if errorPayload["code"] != "already_requested" {
		t.Fatalf("duplicate tool error code = %#v", errorPayload["code"])
	}
	if errorPayload["message"] != "file contents were already provided for this review" {
		t.Fatalf("duplicate tool error message = %#v", errorPayload["message"])
	}
	if _, ok := payload["content"]; ok {
		t.Fatalf("duplicate tool payload should omit content: %#v", payload)
	}
	if _, ok := payload["language"]; ok {
		t.Fatalf("duplicate tool payload should omit language: %#v", payload)
	}
	if llmClient.reqs[2].Messages[6].Role != "user" {
		t.Fatalf("duplicate follow-up role = %q", llmClient.reqs[2].Messages[6].Role)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"./extra.go\"]; result=[lines=1]"; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate follow-up missing first tool call = %q", llmClient.reqs[2].Messages[6].Content)
	}
	if want := "2. inspect_file: tool_call_id=\"call_2\", arguments=[path=\"extra.go\"]; error=\"file contents were already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
	if want := "Continue calling tools, if the provided context is insufficient."; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate follow-up missing regular continuation instructions: %q", llmClient.reqs[2].Messages[6].Content)
	}
	if strings.Contains(llmClient.reqs[2].Messages[6].Content, "Please retry the last tool call.") {
		t.Fatalf("duplicate follow-up should not request retry: %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineReturnsErrorForAlreadyCoveredFileRangeRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go","line_start":1,"line_end":10}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"extra.go","line_start":2,"line_end":3}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[2].Messages[5].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("duplicate range tool payload = %#v", payload)
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("duplicate range error payload = %#v", payload)
	}
	if errorPayload["code"] != "already_requested" {
		t.Fatalf("duplicate range error code = %#v", errorPayload["code"])
	}
	if errorPayload["message"] != "file contents were already provided for this review" {
		t.Fatalf("duplicate range error message = %#v", errorPayload["message"])
	}
	if want := "2. inspect_file: tool_call_id=\"call_2\", arguments=[path=\"extra.go\", line_start=2, line_end=3]; error=\"file contents were already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate range follow-up = %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineExecutesInspectListFilesToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "list:pkg:1" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["path"] != "pkg" {
		t.Fatalf("list payload path = %#v", payload["path"])
	}
	if payload["depth"] != float64(1) {
		t.Fatalf("list payload depth = %#v", payload["depth"])
	}
	files, ok := payload["files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("list payload files = %#v", payload["files"])
	}
	if llmClient.reqs[1].Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", llmClient.reqs[1].Messages[4].Role)
	}
	if want := "1. list_files: tool_call_id=\"call_1\", arguments=[path=\"pkg\", depth=1]; result=[files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
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

func TestEngineReturnsErrorForDuplicateListFilesRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "list_files", Arguments: `{"path":"./pkg","depth":1}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "list:pkg:1" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
	if want := "2. list_files: tool_call_id=\"call_2\", arguments=[path=\"./pkg\", depth=1]; error=\"tool result was already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineAllowsEmptyPathForListFilesToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "list::1" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["path"] != "" {
		t.Fatalf("list payload path = %#v", payload["path"])
	}
	if payload["depth"] != float64(1) {
		t.Fatalf("list payload depth = %#v", payload["depth"])
	}
	if llmClient.reqs[1].Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", llmClient.reqs[1].Messages[4].Role)
	}
	if want := "1. list_files: tool_call_id=\"call_1\", arguments=[path=\".\", depth=1]; result=[files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineExecutesCallersToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "find_callers", Arguments: `{"symbol":"Run","path":"pkg","depth":2}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:2" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["symbol"] != "Run" || payload["path"] != "pkg" || payload["mode"] != "callers" {
		t.Fatalf("callers payload = %#v", payload)
	}
	if want := "1. find_callers: tool_call_id=\"call_1\", arguments=[path=\"pkg\", symbol=\"Run\", depth=2]; result=[lines=2, files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineReturnsErrorForDuplicateFindCallersRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "find_callers", Arguments: `{"symbol":"Run","path":"pkg","depth":2}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "find_callers", Arguments: `{"symbol":"Run","path":"./pkg","depth":2}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:2" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
	if want := "2. find_callers: tool_call_id=\"call_2\", arguments=[path=\"./pkg\", symbol=\"Run\", depth=2]; error=\"tool result was already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineAllowsEmptyPathForCalleesToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "find_callees", Arguments: `{"symbol":"Run"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callees::Run:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["symbol"] != "Run" || payload["path"] != "" || payload["mode"] != "callees" {
		t.Fatalf("callees payload = %#v", payload)
	}
	if want := "1. find_callees: tool_call_id=\"call_1\", arguments=[path=\".\", symbol=\"Run\", depth=10]; result=[lines=2, files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEnginePrintsToolCallsWhenEnabled(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	logger.SetShowProgress(true)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Tool: list_files(path=\"pkg\", depth=1) → result=[files=2]") {
		t.Fatalf("tool call banner missing: %q", got)
	}
	if strings.Contains(got, `"files": [`) || strings.Contains(got, "pkg/a.go") {
		t.Fatalf("tool call output should omit content payloads: %q", got)
	}
}

func TestEngineLogsReasoningProgressForEachLLMCall(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
				Reasoned: true,
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
				Reasoned:               true,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	logger.SetShowProgress(true)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	for _, want := range []string{
		"Reasoning: [reviewer #1: repo@] #1\n",
		"Reasoning: [reviewer #1: repo@] #1 Done ",
		"Reasoning: [reviewer #1: repo@] #2\n",
		"Reasoning: [reviewer #1: repo@] #2 Done ",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("reasoning progress %q missing: %q", want, got)
		}
	}
}

func TestEngineUsesFreshReasoningSectionForEachLLMCall(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
				Reasoned: true,
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
				Reasoned:               true,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	logger.SetShowReasoning(true)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(llmClient.reqs))
	}
	if llmClient.reqs[0].ReasoningSink == nil || llmClient.reqs[1].ReasoningSink == nil {
		t.Fatalf("reasoning sinks should be set for each request: %#v %#v", llmClient.reqs[0].ReasoningSink, llmClient.reqs[1].ReasoningSink)
	}
	if llmClient.reqs[0].ReasoningSink == llmClient.reqs[1].ReasoningSink {
		t.Fatal("reasoning sink should not be reused across LLM calls")
	}
	got := buf.String()
	for _, want := range []string{
		"Reasoning for reviewer #1: repo@ #1...\n",
		"Reasoning for reviewer #1: repo@ #2...\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("per-call reasoning banner %q missing: %q", want, got)
		}
	}
}

func TestEnginePrintsOptimizedSearchReplacementWhenEnabled(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	logger.SetShowProgress(true)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Tool: find_callers(instead_of=\"search\", path=\"pkg\", symbol=\"Run\", depth=10) → result=[lines=2, files=2]") {
		t.Fatalf("optimized tool call banner missing: %q", got)
	}
}

func TestEngineDoesNotPrintToolCallsByDefault(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); strings.Contains(got, "Tool call:") {
		t.Fatalf("tool calls should be hidden by default: %q", got)
	}
}

func TestEngineExecutesSearchToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"","query":"ttlExtenders","context_lines":5,"max_results":20,"case_sensitive":false}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"search::ttlExtenders:5:20:false"}
	if !reflect.DeepEqual(retrievalEngine.paths, wantPaths) {
		t.Fatalf("retrieval paths = %#v, want %#v", retrievalEngine.paths, wantPaths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["query"] != "ttlExtenders" {
		t.Fatalf("search payload query = %#v", payload["query"])
	}
	if payload["context_lines"] != float64(5) {
		t.Fatalf("search payload context_lines = %#v", payload["context_lines"])
	}
	if payload["max_results"] != float64(20) {
		t.Fatalf("search payload max_results = %#v", payload["max_results"])
	}
	if payload["case_sensitive"] != false {
		t.Fatalf("search payload case_sensitive = %#v", payload["case_sensitive"])
	}
	if payload["result_count"] != float64(1) {
		t.Fatalf("search payload result_count = %#v", payload["result_count"])
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("search payload results = %#v", payload["results"])
	}
	firstResult, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("search payload first result = %#v", results[0])
	}
	if firstResult["language"] != "go" {
		t.Fatalf("search payload result language = %#v", firstResult["language"])
	}
	if firstResult["content"] != "before\nttlExtenders\nafter" {
		t.Fatalf("search payload result content = %#v", firstResult["content"])
	}
	if want := "1. search: tool_call_id=\"call_1\", arguments=[path=\".\", query=\"ttlExtenders\", context_lines=5, max_results=20, case_sensitive=false]; result=[files=1, result_count=1]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineRewritesSearchFunctionQueryToFindCallers(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["mode"] != "callers" || payload["symbol"] != "Run" || payload["path"] != "pkg" {
		t.Fatalf("tool payload = %#v", payload)
	}
}

func TestEngineRewritesSearchMethodQueryToFindCallers(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Close()"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Close:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["mode"] != "callers" || payload["symbol"] != "Close" || payload["path"] != "pkg" {
		t.Fatalf("tool payload = %#v", payload)
	}
}

func TestEngineDedupesOptimizedSearchAgainstFindCallers(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "find_callers", Arguments: `{"path":"./pkg","symbol":"Run","depth":10}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
	if want := "1. search (replaced by find_callers): tool_call_id=\"call_1\", arguments=[path=\"pkg\", query=\"Run(\", context_lines=0, max_results=0, case_sensitive=false]"; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
	if want := "2. find_callers: tool_call_id=\"call_2\", arguments=[path=\"./pkg\", symbol=\"Run\", depth=10]; error=\"tool result was already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineReportsZeroSearchResultsExplicitly(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"missing"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if want := "1. search: tool_call_id=\"call_1\", arguments=[path=\"pkg\", query=\"missing\", context_lines=0, max_results=0, case_sensitive=false]; result=[result_count=0]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineCanDisableSearchToolOptimization(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})
	engine.SetSearchToolOptimization(false)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "search:pkg:Run(:0:0:false" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
}

func TestEngineTreatsZeroToolRoundsAsUnlimited(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "list_files", Arguments: `{"path":"pkg"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if result.TotalToolCalls != 2 {
		t.Fatalf("tool calls = %d", result.TotalToolCalls)
	}
	if len(retrievalEngine.paths) != 2 {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
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
	mu                     sync.Mutex
	reviewerReasoning      []string
	reviewerMessages       []string
	phaseAInputs           []string
	phaseAOutputs          []string
	phaseBFullLists        []string
	phaseBFindingsJSON     []string
	phaseBOutputs          []string
	firstPhaseAStarted     chan struct{}
	releaseFirstPhaseA     chan struct{}
	firstPhaseAStartedOnce sync.Once
}

func (s *reasoningExtractLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	user := lastUserContent(req.Messages)
	switch {
	case strings.Contains(system, "Read one reviewer reasoning trace"):
		input := strings.TrimSpace(strings.TrimPrefix(user, "Reviewer reasoning trace:"))
		s.mu.Lock()
		idx := len(s.phaseAInputs)
		s.phaseAInputs = append(s.phaseAInputs, input)
		output := outputAt(s.phaseAOutputs, idx)
		started := s.firstPhaseAStarted
		release := s.releaseFirstPhaseA
		s.mu.Unlock()
		if idx == 0 && started != nil {
			s.firstPhaseAStartedOnce.Do(func() { close(started) })
		}
		if idx == 0 && release != nil {
			<-release
		}
		return textResponse(output, 1), nil
	case strings.Contains(system, "Given a list of issues extracted from reviewer reasoning"):
		fullList, findingsJSON := phaseBPayloadParts(user)
		s.mu.Lock()
		idx := len(s.phaseBFullLists)
		s.phaseBFullLists = append(s.phaseBFullLists, fullList)
		s.phaseBFindingsJSON = append(s.phaseBFindingsJSON, findingsJSON)
		output := outputAt(s.phaseBOutputs, idx)
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

func phaseBPayloadParts(content string) (string, string) {
	const listStart = "Extracted issue list:"
	const findingsStart = "Current findings JSON:"
	_, rest, _ := strings.Cut(content, listStart)
	list, findings, _ := strings.Cut(rest, findingsStart)
	return strings.TrimSpace(list), strings.TrimSpace(findings)
}

func TestEngineRetriesOnInvalidJSONResponse(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{
				err: &llm.InvalidResponseError{
					RawContent: "Sure! Here it is:\n\n{not valid json}",
					Reason:     "could not parse JSON: unexpected token",
				},
			},
			{
				resp: &llm.ReviewResponse{
					OverallCorrectness:     "patch is incorrect",
					OverallExplanation:     "fixed",
					OverallConfidenceScore: 0.7,
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxOutputRetries: defaultMaxOutputRetries,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OverallCorrectness != "patch is incorrect" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[1].Tools) == 0 {
		t.Fatal("normal JSON retry should keep tools")
	}
	if !llmClient.reqs[1].ParallelToolCalls {
		t.Fatal("normal JSON retry should keep parallel tool calls enabled")
	}
	retryMessages := llmClient.reqs[1].Messages
	if len(retryMessages) < 4 {
		t.Fatalf("retry messages = %d, want at least 4", len(retryMessages))
	}
	assistantMsg := retryMessages[len(retryMessages)-2]
	if assistantMsg.Role != "assistant" {
		t.Fatalf("retry assistant role = %q", assistantMsg.Role)
	}
	if !strings.Contains(assistantMsg.Content, "{not valid json}") {
		t.Fatalf("retry assistant content = %q", assistantMsg.Content)
	}
	userMsg := retryMessages[len(retryMessages)-1]
	if userMsg.Role != "user" {
		t.Fatalf("retry user role = %q", userMsg.Role)
	}
	if !strings.Contains(userMsg.Content, "could not be parsed") {
		t.Fatalf("retry user content missing reason: %q", userMsg.Content)
	}
	if !strings.Contains(userMsg.Content, "ONLY a JSON object") {
		t.Fatalf("retry user content missing instruction: %q", userMsg.Content)
	}
}

func TestEngineRetriesToolsOmittedInvalidJSONWithoutTools(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{
				resp: &llm.ReviewResponse{
					RawResponse: "I'll inspect extra.go first.",
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
					},
					ReasoningEffort: "medium",
				},
			},
			{
				err: &llm.InvalidResponseError{
					RawContent:      "not json",
					Reason:          "could not parse JSON: unexpected token",
					ReasoningEffort: "low",
					ToolsOmitted:    true,
				},
			},
			{
				resp: &llm.ReviewResponse{
					OverallCorrectness:     "patch is correct",
					OverallExplanation:     "fixed",
					OverallConfidenceScore: 0.7,
					ReasoningEffort:        "low",
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test", ReasoningEffort: "high"})
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxOutputRetries: defaultMaxOutputRetries,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ReasoningEffort != "low" {
		t.Fatalf("result reasoning effort = %q", result.ReasoningEffort)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	retryReq := llmClient.reqs[2]
	if len(retryReq.Tools) != 0 {
		t.Fatalf("JSON repair request should have no tools, got %d", len(retryReq.Tools))
	}
	if retryReq.ParallelToolCalls {
		t.Fatal("JSON repair request should disable parallel tool calls")
	}
	if retryReq.ReasoningEffort != "low" {
		t.Fatalf("JSON repair reasoning effort = %q", retryReq.ReasoningEffort)
	}
	if strings.Contains(retryReq.Messages[0].Content, "`inspect_file` tool") {
		t.Fatalf("system prompt should omit tool instructions: %q", retryReq.Messages[0].Content)
	}
	if !strings.Contains(retryReq.Messages[0].Content, "OUTPUT FORMAT") {
		t.Fatalf("system prompt missing review instructions: %q", retryReq.Messages[0].Content)
	}

	foundToolResult := false
	for _, msg := range retryReq.Messages {
		if msg.Role == "tool" {
			t.Fatalf("JSON repair request should convert tool messages to user messages: %#v", msg)
		}
		if len(msg.ToolCalls) > 0 {
			t.Fatalf("JSON repair request should strip assistant tool calls: %#v", msg)
		}
		if msg.Role == "user" && strings.Contains(msg.Content, `"path":"extra.go"`) {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("JSON repair request missing converted tool result: %#v", retryReq.Messages)
	}
	if got := retryReq.Messages[len(retryReq.Messages)-2]; got.Role != "assistant" || got.Content != "not json" {
		t.Fatalf("repair raw response message = %#v", got)
	}
	feedback := retryReq.Messages[len(retryReq.Messages)-1]
	if feedback.Role != "user" {
		t.Fatalf("repair feedback role = %q", feedback.Role)
	}
	if !strings.Contains(feedback.Content, "could not be parsed") || !strings.Contains(feedback.Content, "ONLY a JSON object") {
		t.Fatalf("repair feedback content = %q", feedback.Content)
	}
}

func TestEngineFailsAfterMaxJSONRetries(t *testing.T) {
	results := make([]scriptedLLMResult, 0, defaultMaxOutputRetries+1)
	for i := 0; i <= defaultMaxOutputRetries; i++ {
		results = append(results, scriptedLLMResult{
			err: &llm.InvalidResponseError{
				RawContent: "still bad",
				Reason:     "could not parse JSON",
			},
		})
	}
	llmClient := &scriptedLLM{results: results}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxOutputRetries: defaultMaxOutputRetries,
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !errors.Is(err, llm.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
	if len(llmClient.reqs) != defaultMaxOutputRetries+1 {
		t.Fatalf("requests = %d, want %d", len(llmClient.reqs), defaultMaxOutputRetries+1)
	}
}

func TestEngineTreatsZeroMaxOutputRetriesAsUnlimited(t *testing.T) {
	results := make([]scriptedLLMResult, 0, defaultMaxOutputRetries+2)
	for i := 0; i <= defaultMaxOutputRetries; i++ {
		results = append(results, scriptedLLMResult{
			err: &llm.InvalidResponseError{
				RawContent: "still bad",
				Reason:     "could not parse JSON",
			},
		})
	}
	results = append(results, scriptedLLMResult{
		resp: &llm.ReviewResponse{
			OverallCorrectness:     "patch is correct",
			OverallExplanation:     "recovered",
			OverallConfidenceScore: 0.7,
		},
	})
	llmClient := &scriptedLLM{results: results}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxOutputRetries: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OverallExplanation != "recovered" {
		t.Fatalf("overall explanation = %q", result.OverallExplanation)
	}
	if len(llmClient.reqs) != defaultMaxOutputRetries+2 {
		t.Fatalf("requests = %d, want %d", len(llmClient.reqs), defaultMaxOutputRetries+2)
	}
}

func TestEngineParsesLenientToolArguments(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Name: "inspect_file",
						// Trailing comma + single quotes + prose wrapper — LenientUnmarshal should recover this.
						Arguments: "Sure: {'path': 'main.go',}",
					},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "ok",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})
	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "main.go" {
		t.Fatalf("expected lenient tool arguments to be parsed, got %#v", retrievalEngine.paths)
	}
}

func TestEngineMergesRegexAndLiteralSearchResults(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"foo.*bar"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{
		hasCustomResults: true,
		literalResults: []retrieval.SearchResult{
			{Path: "pkg/a.go", StartLine: 1, EndLine: 1, Language: "go", Content: "foo.*bar literal hit"},
		},
		regexResults: []retrieval.SearchResult{
			{Path: "pkg/a.go", StartLine: 5, EndLine: 5, Language: "go", Content: "fooXbar regex hit"},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	if _, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	}); err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload["result_count"] != float64(2) {
		t.Fatalf("result_count = %#v", payload["result_count"])
	}
	results := payload["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results len = %d", len(results))
	}
	if got := results[0].(map[string]any)["start_line"]; got != float64(1) {
		t.Fatalf("first result start_line = %#v", got)
	}
	if got := results[1].(map[string]any)["start_line"]; got != float64(5) {
		t.Fatalf("second result start_line = %#v", got)
	}
}

func TestEngineDedupesOverlappingRegexAndLiteralHits(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"user(Name)?"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	hit := retrieval.SearchResult{Path: "pkg/a.go", StartLine: 7, EndLine: 9, Language: "go", Content: "userName"}
	retrievalEngine := &countingRetrieval{
		hasCustomResults: true,
		literalResults:   []retrieval.SearchResult{hit},
		regexResults:     []retrieval.SearchResult{hit},
	}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	if _, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	}); err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload["result_count"] != float64(1) {
		t.Fatalf("result_count = %#v (regex+literal hits should dedup)", payload["result_count"])
	}
}

func TestEngineFallsBackToLiteralWhenRegexInvalid(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"foo[bar"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	if _, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	}); err != nil {
		t.Fatal(err)
	}

	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("regex compile must fail and skip SearchRegex; paths = %#v", retrievalEngine.paths)
	}
	if !strings.HasPrefix(retrievalEngine.paths[0], "search:") {
		t.Fatalf("expected literal search path, got %q", retrievalEngine.paths[0])
	}
}

func TestEngineMergeTruncatesToMaxResults(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"x.*","max_results":2}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{
		hasCustomResults: true,
		literalResults: []retrieval.SearchResult{
			{Path: "pkg/a.go", StartLine: 1, EndLine: 1, Language: "go", Content: "a"},
			{Path: "pkg/a.go", StartLine: 2, EndLine: 2, Language: "go", Content: "b"},
		},
		regexResults: []retrieval.SearchResult{
			{Path: "pkg/a.go", StartLine: 3, EndLine: 3, Language: "go", Content: "c"},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	if _, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	}); err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload["result_count"] != float64(2) {
		t.Fatalf("result_count = %#v, want 2 (merged set capped at max_results)", payload["result_count"])
	}
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
