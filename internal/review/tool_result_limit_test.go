package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/tokenestimate"
	"github.com/dgrieser/nickpit/internal/toollimits"
)

func limitedPayload(t *testing.T, value any, maxTokens int) (string, map[string]any) {
	t.Helper()
	raw := mustToolResultJSON(value)
	limited := limitToolResultJSON(raw, maxTokens)
	if maxTokens > 0 && tokenestimate.Estimate(limited) > maxTokens {
		t.Fatalf("limited payload = %d tokens, cap = %d", tokenestimate.Estimate(limited), maxTokens)
	}
	if !utf8.ValidString(limited) {
		t.Fatal("limited payload is not valid UTF-8")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(limited), &payload); err != nil {
		t.Fatalf("limited payload is not valid JSON: %v", err)
	}
	return limited, payload
}

func TestToolResultLimiterLeavesSmallPayloadUnchanged(t *testing.T) {
	raw := `{"path":"a.go","content":"small"}`
	if got := limitToolResultJSON(raw, 128); got != raw {
		t.Fatalf("result changed: %s", got)
	}
}

// A cap too small for any structure still has to explain itself: a bare "{}"
// reads as a successful empty result and the model reruns the same call.
func TestToolResultLimiterExplainsItselfUnderTinyTokenLimit(t *testing.T) {
	got := limitToolResultJSON(`{"path":"a.go","content":"too large"}`, 1)
	if !json.Valid([]byte(got)) {
		t.Fatalf("result is not valid JSON: %s", got)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatal(err)
	}
	note, _ := payload["truncated_note"].(string)
	if payload["truncated"] != true || !strings.Contains(note, "narrower arguments") {
		t.Fatalf("result = %s", got)
	}
}

func TestToolResultLimiterZeroDisablesTokensButKeepsReferenceCountLimit(t *testing.T) {
	functions := make([]map[string]any, 30)
	for i := range functions {
		functions[i] = map[string]any{"name": fmt.Sprintf("f%d", i)}
	}
	_, payload := limitedPayload(t, map[string]any{
		"target": map[string]any{"name": "Shared"}, "functions": functions, "outside_functions": []any{},
	}, 0)
	if got := len(payload["functions"].([]any)); got != toollimits.MaxReferenceFunctions {
		t.Fatalf("functions = %d", got)
	}
	if payload["truncated"] != true || intFromJSON(payload["omitted_contexts"]) != 5 {
		t.Fatalf("payload = %#v", payload)
	}
}

// Both reference context lists carry contexts, so capping only one leaves the
// payload unbounded whenever token capping is disabled.
func TestToolResultLimiterCapsOutsideFunctionContexts(t *testing.T) {
	contexts := make([]map[string]any, 40)
	for i := range contexts {
		contexts[i] = map[string]any{"code_location": map[string]any{"file_path": fmt.Sprintf("f%d.go", i)}}
	}
	_, payload := limitedPayload(t, map[string]any{
		"target": map[string]any{"name": "Shared"}, "functions": []any{}, "outside_functions": contexts,
	}, 0)
	if got := len(payload["outside_functions"].([]any)); got != toollimits.MaxReferenceFunctions {
		t.Fatalf("outside_functions = %d, want %d", got, toollimits.MaxReferenceFunctions)
	}
	if payload["truncated"] != true || intFromJSON(payload["omitted_contexts"]) != 15 {
		t.Fatalf("payload = %#v", payload)
	}
}

// Dropping contexts contradicts a completeness claim: the reader could
// conclude a symbol is never written when the writes were in the dropped tail.
func TestToolResultLimiterClearsCompleteWhenContextsAreDropped(t *testing.T) {
	functions := make([]map[string]any, 40)
	for i := range functions {
		functions[i] = map[string]any{"name": fmt.Sprintf("f%d", i)}
	}
	_, payload := limitedPayload(t, map[string]any{
		"target": map[string]any{"name": "Shared"}, "functions": functions, "outside_functions": []any{},
		"exact_reference_count": 340, "complete": true,
	}, 0)
	if payload["complete"] != false {
		t.Fatalf("complete = %v alongside %d omitted contexts", payload["complete"], intFromJSON(payload["omitted_contexts"]))
	}
}

// The same claim has to be cleared inside a grouped search result: its nested
// analyses are the ones being shortened, and left to the generic reducer they
// were tail-dropped while still reporting themselves as complete.
func TestToolResultLimiterMarksNestedGroupedAnalysesIncomplete(t *testing.T) {
	referenceResult := func(prefix string) map[string]any {
		contexts := make([]map[string]any, 20)
		for i := range contexts {
			contexts[i] = map[string]any{
				"name": fmt.Sprintf("%s%d", prefix, i),
				"code_location": map[string]any{
					"file_path": fmt.Sprintf("%s%d.go", prefix, i),
					"content":   strings.Repeat("line of source\n", 20),
				},
			}
		}
		return map[string]any{
			"target": map[string]any{"name": "Shared"}, "functions": contexts,
			"outside_functions": []any{}, "exact_reference_count": 40, "complete": true,
		}
	}
	_, payload := limitedPayload(t, map[string]any{
		"structural_result_count": 2,
		"reference_results":       []any{referenceResult("a"), referenceResult("b")},
		"literal_result_count":    0,
		"literal_results":         []any{},
	}, 512)

	nested, _ := payload["reference_results"].([]any)
	if len(nested) != 2 {
		t.Fatalf("grouped payload lost an analysis: %#v", payload)
	}
	for i, item := range nested {
		analysis, _ := item.(map[string]any)
		contexts, _ := analysis["functions"].([]any)
		if len(contexts) == 20 {
			continue // this one was not the analysis that had to shrink
		}
		if analysis["complete"] != false || intFromJSON(analysis["omitted_contexts"]) == 0 {
			t.Fatalf("shortened analysis %d still claims completeness: %#v", i, analysis)
		}
	}
}

// The meter reuses what it measured on earlier rounds, so it has to keep
// answering exactly what encoding the whole request would.
func TestToolContextMeterMatchesFullMeasurement(t *testing.T) {
	tools := []llm.ToolDefinition{{Name: "inspect_file", Description: "read a file"}}
	schema := []byte(`{"type":"object"}`)
	fullMeasurement := func(messages []llm.Message) int {
		return jsonByteLen(struct {
			Messages []llm.Message        `json:"messages"`
			Tools    []llm.ToolDefinition `json:"tools,omitempty"`
			Schema   json.RawMessage      `json:"schema,omitempty"`
		}{Messages: messages, Tools: tools, Schema: schema})
	}
	meter := &toolContextMeter{}
	messages := []llm.Message{}
	if got, want := meter.contextByteLen(messages, tools, schema), fullMeasurement(messages); got != want {
		t.Fatalf("empty transcript = %d bytes, want %d", got, want)
	}
	for i := range 6 {
		messages = append(messages, llm.Message{
			Role: "tool", ToolCallID: fmt.Sprintf("c%d", i), Content: strings.Repeat("x", 100+i),
		})
		if got, want := meter.contextByteLen(messages, tools, schema), fullMeasurement(messages); got != want {
			t.Fatalf("round %d = %d bytes, want %d", i, got, want)
		}
	}
	// A JSON repair rewinds the history, so one index can come back holding a
	// different message of the same length.
	messages[2] = llm.Message{Role: "user", Content: strings.Repeat("\"", len(messages[2].Content))}
	if got, want := meter.contextByteLen(messages, tools, schema), fullMeasurement(messages); got != want {
		t.Fatalf("rewound transcript = %d bytes, want %d", got, want)
	}
}

// The per-result floor keeps one result readable; applied to every result of a
// wide batch against a full window it added up to an unbounded overshoot.
func TestToolResultBatchBoundsTheFloorAcrossOneBatch(t *testing.T) {
	messages := []llm.Message{{Role: "system", Content: strings.Repeat("x", 40_000)}}
	raw := fmt.Sprintf(`{"path":"a.go","start_line":1,"end_line":500,"content":%q}`, strings.Repeat("line content\n", 500))
	batch := make([]llm.Message, 12)
	for i := range batch {
		batch[i] = llm.Message{Role: "tool", Content: raw}
	}
	limited := limitToolResultBatch(&toolContextMeter{}, messages, nil, nil, batch, 1000, 10)

	total := 0
	for _, message := range limited {
		total += tokenestimate.Estimate(message.Content)
	}
	// Nothing of the window is left, so everything the batch spends is overshoot
	// and has to fit inside the budget the floor is allowed to add.
	if total > maxToolResultFloorTokens {
		t.Fatalf("batch spent %d tokens against a full window, budget = %d", total, maxToolResultFloorTokens)
	}
	if payload := decodeToolPayload(t, limited[0].Content); payload["truncated"] != true {
		t.Fatalf("first result is not marked truncated: %s", limited[0].Content)
	}
	if payload := decodeToolPayload(t, limited[len(limited)-1].Content); payload["truncated"] != true {
		t.Fatalf("last result does not explain itself: %s", limited[len(limited)-1].Content)
	}
}

func TestToolResultLimiterReducesToolPayloadShapes(t *testing.T) {
	large := strings.Repeat("ü", 12<<10)
	searchResults := make([]map[string]any, 20)
	files := make([]string, 500)
	children := make([]map[string]any, 12)
	commits := make([]map[string]any, 12)
	for i := range 20 {
		searchResults[i] = map[string]any{"code_location": map[string]any{"file_path": fmt.Sprintf("f%d.go", i), "content": large}}
	}
	for i := range files {
		files[i] = fmt.Sprintf("pkg/%04d/very-long-file-name.go", i)
	}
	for i := range children {
		children[i] = map[string]any{"name": fmt.Sprintf("caller%d", i), "code_location": map[string]any{"content": large}, "children": []any{}}
	}
	for i := range commits {
		commits[i] = map[string]any{"sha": fmt.Sprintf("%040d", i), "body": large, "files": files[:20]}
	}

	tests := []struct {
		name  string
		value map[string]any
		check func(*testing.T, map[string]any)
	}{
		{
			name:  "inspect",
			value: map[string]any{"path": "large.go", "start_line": 10, "end_line": 99, "content": large},
			check: func(t *testing.T, payload map[string]any) {
				if payload["path"] != "large.go" || intFromJSON(payload["end_line"]) < 10 {
					t.Fatalf("inspect payload = %#v", payload)
				}
			},
		},
		{
			name:  "list",
			value: map[string]any{"path": "", "files": files},
			check: func(t *testing.T, payload map[string]any) {
				if intFromJSON(payload["omitted_files"]) == 0 {
					t.Fatalf("list payload = %#v", payload)
				}
			},
		},
		{
			name:  "search",
			value: map[string]any{"query": "Shared", "result_count": len(searchResults), "results": searchResults},
			check: func(t *testing.T, payload map[string]any) {
				if intFromJSON(payload["result_count"]) != len(payload["results"].([]any)) || intFromJSON(payload["omitted_results"]) == 0 {
					t.Fatalf("search payload = %#v", payload)
				}
			},
		},
		{
			name:  "hierarchy",
			value: map[string]any{"mode": "callers", "root": map[string]any{"name": "Shared", "code_location": map[string]any{"content": large}, "children": children}},
			check: func(t *testing.T, payload map[string]any) {
				root, _ := payload["root"].(map[string]any)
				if root["name"] != "Shared" {
					t.Fatalf("hierarchy payload = %#v", payload)
				}
			},
		},
		{
			name:  "git log",
			value: map[string]any{"range": "HEAD", "commit_count": len(commits), "commits": commits},
			check: func(t *testing.T, payload map[string]any) {
				if intFromJSON(payload["commit_count"]) != len(payload["commits"].([]any)) || intFromJSON(payload["omitted_commits"]) == 0 {
					t.Fatalf("git payload = %#v", payload)
				}
			},
		},
		{
			name: "git show",
			value: map[string]any{
				"range": "HEAD", "diff_format": "git", "commit_count": 1,
				"commits": []any{map[string]any{
					"sha": "abc", "diff_files": []any{map[string]any{"file_path": "large.go", "content": large}},
				}},
			},
			check: func(t *testing.T, payload map[string]any) {
				commits, _ := payload["commits"].([]any)
				if len(commits) != 1 || commits[0].(map[string]any)["sha"] != "abc" {
					t.Fatalf("git show payload = %#v", payload)
				}
			},
		},
		{
			name:  "future tool",
			value: map[string]any{"kind": "future", "widgets": searchResults},
			check: func(t *testing.T, payload map[string]any) {
				if payload["kind"] != "future" {
					t.Fatalf("future payload = %#v", payload)
				}
			},
		},
		{
			name: "error",
			value: map[string]any{
				"status": "error", "path": "large.go",
				"error": map[string]any{"code": "retrieval_failed", "message": large},
			},
			check: func(t *testing.T, payload map[string]any) {
				if payload["status"] != "error" || nestedString(payload, "error", "code") != "retrieval_failed" {
					t.Fatalf("error payload = %#v", payload)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, payload := limitedPayload(t, tt.value, 1024)
			if payload["truncated"] != true || payload["truncated_note"] == "" {
				t.Fatalf("missing truncation metadata: %#v", payload)
			}
			tt.check(t, payload)
		})
	}
}

func TestToolResultLimiterPreservesExistingTruncationNote(t *testing.T) {
	_, payload := limitedPayload(t, map[string]any{
		"path": "large.go", "content": strings.Repeat("x", 8<<10),
		"truncated": true, "truncated_note": "retrieval clipped source file",
	}, 512)
	note, _ := payload["truncated_note"].(string)
	if !strings.Contains(note, "retrieval clipped source file") || !strings.Contains(note, toolResultTruncatedNote) {
		t.Fatalf("note = %q", note)
	}
}

func TestToolResultBatchUsesPercentageOfRemainingContext(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "large.txt", strings.Repeat("content\n", 4<<10))
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	result := engine.executeToolCall(context.Background(), repoRoot, llm.ToolCall{
		ID: "inspect", Name: "inspect_file", Arguments: `{"path":"large.txt"}`,
	}, freshToolRoundState())
	messages := []llm.Message{{Role: "system", Content: strings.Repeat("x", 2000)}}
	tools := []llm.ToolDefinition{{Name: "inspect_file", Description: strings.Repeat("tool", 50)}}
	schema := []byte(`{"type":"object"}`)
	// Recomputed with the same helpers the limiter uses, so the expectation
	// cannot drift away from the production accounting.
	remaining := 10000 - tokenestimate.EstimateLen((&toolContextMeter{}).contextByteLen(messages, tools, schema))
	batch := limitToolResultBatch(&toolContextMeter{}, messages, tools, schema, []llm.Message{
		{Role: "tool", Content: result},
		{Role: "tool", Content: result},
	}, 10000, 10)
	firstLimit := max(remaining*10/100, 1)
	if got := tokenestimate.Estimate(batch[0].Content); got > firstLimit {
		t.Fatalf("first result = %d tokens, cap = %d", got, firstLimit)
	}
	secondRemaining := 10000 - tokenestimate.EstimateLen((&toolContextMeter{}).contextByteLen(append(messages, batch[0]), tools, schema))
	secondLimit := max(secondRemaining*10/100, 1)
	if got := tokenestimate.Estimate(batch[1].Content); got > secondLimit {
		t.Fatalf("second result = %d tokens, cap = %d", got, secondLimit)
	}
	payload := decodeToolPayload(t, batch[0].Content)
	if payload["truncated"] != true || payload["path"] != "large.txt" {
		t.Fatalf("payload = %#v", payload)
	}
}

// A prompt that already fills the context leaves a zero percentage share. The
// results still have to be readable rather than collapsing to a bare "{}".
func TestToolResultBatchKeepsResultsReadableWhenContextIsExhausted(t *testing.T) {
	messages := []llm.Message{{Role: "system", Content: strings.Repeat("x", 40_000)}}
	raw := fmt.Sprintf(`{"path":"a.go","start_line":1,"end_line":500,"content":%q}`, strings.Repeat("line content\n", 500))
	batch := limitToolResultBatch(&toolContextMeter{}, messages, nil, nil, []llm.Message{{Role: "tool", Content: raw}}, 1000, 10)

	payload := decodeToolPayload(t, batch[0].Content)
	if payload["truncated"] != true {
		t.Fatalf("exhausted-context result is not marked truncated: %s", batch[0].Content)
	}
	if payload["path"] != "a.go" {
		t.Fatalf("exhausted-context result lost its payload: %s", batch[0].Content)
	}
}

func TestToolResultBatchZeroPercentDisablesContextCap(t *testing.T) {
	raw := `{"path":"large.go","content":"unchanged"}`
	batch := []llm.Message{{Role: "tool", Content: raw}}
	got := limitToolResultBatch(&toolContextMeter{}, nil, nil, nil, batch, 100, 0)
	if got[0].Content != raw {
		t.Fatalf("result changed: %s", got[0].Content)
	}
}

func TestLimitedFullFileAllowsOmittedRangeRetry(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "large.txt", strings.Repeat("line content\n", 500))
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	state := freshToolRoundState()
	call := llm.ToolCall{ID: "full", Name: "inspect_file", Arguments: `{"path":"large.txt"}`}
	raw := llm.Message{ToolCallID: call.ID, Content: engine.executeToolCall(context.Background(), repoRoot, call, state)}
	limited := raw
	limited.Content = limitToolResultJSON(raw.Content, 64)
	reconcileLimitedInspectFileCoverage([]llm.ToolCall{call}, []llm.Message{raw}, []llm.Message{limited}, state)

	retry := engine.executeToolCall(context.Background(), repoRoot, llm.ToolCall{
		ID: "range", Name: "inspect_file", Arguments: `{"path":"large.txt","line_start":400,"line_end":410}`,
	}, state)
	if payload := decodeToolPayload(t, retry); payload["status"] == "error" {
		t.Fatalf("omitted full-file range rejected: %#v", payload)
	}
}

func TestLimitedFileRangeAllowsOmittedLinesRetry(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "large.txt", strings.Repeat("line content\n", 500))
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	state := freshToolRoundState()
	call := llm.ToolCall{ID: "first", Name: "inspect_file", Arguments: `{"path":"large.txt","line_start":1,"line_end":400}`}
	raw := llm.Message{ToolCallID: call.ID, Content: engine.executeToolCall(context.Background(), repoRoot, call, state)}
	limited := raw
	limited.Content = limitToolResultJSON(raw.Content, 64)
	reconcileLimitedInspectFileCoverage([]llm.ToolCall{call}, []llm.Message{raw}, []llm.Message{limited}, state)

	visible := decodeToolPayload(t, limited.Content)
	retryStart := max(intFromJSON(visible["end_line"])+1, 1)
	retryCall := llm.ToolCall{ID: "retry", Name: "inspect_file", Arguments: fmt.Sprintf(`{"path":"large.txt","line_start":%d,"line_end":400}`, retryStart)}
	retry := engine.executeToolCall(context.Background(), repoRoot, retryCall, state)
	if payload := decodeToolPayload(t, retry); payload["status"] == "error" {
		t.Fatalf("omitted ranged lines rejected: %#v", payload)
	}
}

// A mixed search result keeps its literal matches alongside the reference
// analysis. Once the reference-specific fields are exhausted, the payload must
// keep shrinking through the generic reducer instead of being discarded whole.
func TestToolResultLimiterKeepsShrinkingExhaustedReferencePayload(t *testing.T) {
	literals := make([]map[string]any, 40)
	for i := range literals {
		literals[i] = map[string]any{"code_location": map[string]any{
			"file_path": fmt.Sprintf("notes/%02d.txt", i),
			"content":   strings.Repeat("literal match text ", 20),
		}}
	}
	raw := mustToolResultJSON(map[string]any{
		"target": map[string]any{
			"name":       "Value",
			"kind":       "variable",
			"definition": map[string]any{"file_path": "a.go", "content": ""},
		},
		"functions":            []any{},
		"outside_functions":    []any{},
		"literal_result_count": len(literals),
		"literal_results":      literals,
	})

	got := limitToolResultJSON(raw, 256)
	var payload map[string]any
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["target"]; !ok {
		t.Fatalf("payload was discarded instead of shrinking: %s", got)
	}
	if tokenestimate.Estimate(got) > 256 {
		t.Fatalf("payload = %d tokens, cap = 256: %s", tokenestimate.Estimate(got), got)
	}
}

// A search that resolved a declaration keeps its structural analysis at the top
// level and the matches it could not classify beside it, so the payload
// classifies as the analysis. Its literal matches still have to be shortened
// through the step that keeps literal_result_count true — the generic reducer
// reaches the same array and leaves the count claiming matches the model can no
// longer see.
func TestToolResultLimiterKeepsLiteralCountTrueOnMixedSearchResult(t *testing.T) {
	literals := make([]map[string]any, 40)
	for i := range literals {
		literals[i] = map[string]any{"code_location": map[string]any{
			"file_path": fmt.Sprintf("notes-%d.txt", i),
			"content":   strings.Repeat("mentioned here\n", 20),
		}}
	}
	_, payload := limitedPayload(t, map[string]any{
		"target": map[string]any{"name": "Shared"}, "functions": []any{}, "outside_functions": []any{},
		"literal_result_count": len(literals), "literal_results": literals,
	}, 512)
	kept, _ := payload["literal_results"].([]any)
	if len(kept) == len(literals) {
		t.Fatalf("literal matches were not shortened: %d", len(kept))
	}
	if intFromJSON(payload["literal_result_count"]) != len(kept) {
		t.Fatalf("literal_result_count = %v alongside %d matches", payload["literal_result_count"], len(kept))
	}
	if intFromJSON(payload["omitted_literal_results"]) != len(literals)-len(kept) {
		t.Fatalf("omitted_literal_results = %v, want %d", payload["omitted_literal_results"], len(literals)-len(kept))
	}
}
