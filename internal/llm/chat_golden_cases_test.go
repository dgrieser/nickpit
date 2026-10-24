package llm

import "encoding/json"

// chatGoldenCases are representative Chat Completions requests whose wire
// bodies are pinned in testdata/chat_completions_golden.json, captured from
// the client before the Protocol refactor.
func chatGoldenCases() map[string]*ReviewRequest {
	f := func(v float64) *float64 { return &v }
	i := func(v int) *int { return &v }
	return map[string]*ReviewRequest{
		"single_turn_review_schema_all_knobs": {
			SystemPrompt: "system prompt", UserContent: "review this diff",
			Schema: json.RawMessage(`{"type":"object","properties":{"findings":{"type":"array"}}}`), SchemaKind: SchemaKindReview,
			MaxTokens: i(4096), Temperature: f(0), TopP: f(0.95), TopK: i(20), MinP: f(0), PresencePenalty: f(1.5), RepetitionPenalty: f(1.05),
			ReasoningEffort: "high",
			ExtraBody:       map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": true}, "seed": 7},
		},
		"tool_history_parallel": {
			Messages: []Message{
				{Role: "system", Content: "system"},
				{Role: "user", Content: "find callers"},
				{Role: "assistant", Content: "", ToolCalls: []ToolCall{
					{ID: "call_bad", Name: "inspect_file", Arguments: "not json"},
					{ID: "call_ok", Name: "find_callers", Arguments: `{"symbol": "Run", "path": "cmd"}`},
					{ID: "call_empty", Name: "list_files", Arguments: ""},
				}},
				{Role: "tool", ToolCallID: "call_bad", Name: "inspect_file", Content: "dropped"},
				{Role: "tool", ToolCallID: "call_ok", Name: "find_callers", Content: `{"callers":[]}`},
				{Role: "tool", ToolCallID: "call_empty", Name: "list_files", Content: "README.md"},
				{Role: "user", Content: "continue"},
			},
			Tools: []ToolDefinition{
				{Name: "find_callers", Description: "Find callers", Parameters: json.RawMessage(`{"type":"object","properties":{"symbol":{"type":"string"}}}`)},
				{Name: "list_files", Description: "List files", Parameters: json.RawMessage(`{"type":"object"}`)},
			},
			ParallelToolCalls: true,
			ReasoningEffort:   "medium",
			Temperature:       f(0.7),
		},
		"finalize_strips_tool_extras": {
			SystemPrompt: "finalize", UserContent: "polish",
			Tools:      []ToolDefinition{{Name: "list_files", Parameters: json.RawMessage(`{"type":"object"}`)}},
			Finalize:   true,
			ExtraBody:  map[string]any{"tools": []any{"x"}, "tool_choice": "auto", "parallel_tool_calls": true, "reasoning_effort": "max", "top_k": 5},
			Schema:     json.RawMessage(`{"type":"object"}`),
			SchemaKind: SchemaKindJSON,
		},
		"plain_text_no_effort": {
			SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText,
		},
	}
}
