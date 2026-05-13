package modelcheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
)

type scriptedClient struct {
	responses []scriptedResponse
	reqs      []*llm.ReviewRequest
}

type scriptedResponse struct {
	resp *llm.ReviewResponse
	err  error
}

const validJSONProbeResponse = `{"check":"json_capability","status":"ok","confidence_score":0.9}`

func (s *scriptedClient) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	cloned := *req
	cloned.Messages = append([]llm.Message(nil), req.Messages...)
	cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	s.reqs = append(s.reqs, &cloned)
	if len(s.responses) == 0 {
		return &llm.ReviewResponse{RawResponse: finalSentinel}, nil
	}
	next := s.responses[0]
	s.responses = s.responses[1:]
	return next.resp, next.err
}

func TestCheckerRunsToolProbeWithInMemoryFixture(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredNoTools().Status != StatusOK {
		t.Fatalf("no-tools status = %s", result.ConfiguredNoTools().Status)
	}
	if result.ConfiguredTools().Status != StatusOK {
		t.Fatalf("tools status = %s error=%s", result.ConfiguredTools().Status, result.ConfiguredTools().Error)
	}
	if result.ConfiguredJSONOutput().Status != StatusOK {
		t.Fatalf("json-output status = %s error=%s", result.ConfiguredJSONOutput().Status, result.ConfiguredJSONOutput().Error)
	}
	if len(client.reqs) < 3 {
		t.Fatalf("requests = %d, want at least 3", len(client.reqs))
	}
	if len(client.reqs[1].Tools) != 1 || client.reqs[1].Tools[0].Name != "list_files" {
		t.Fatalf("tool probe tools = %#v, want only list_files", client.reqs[1].Tools)
	}
	toolResponseCount := 0
	for _, msg := range client.reqs[2].Messages {
		if msg.Role == "tool" {
			toolResponseCount++
		}
	}
	if toolResponseCount != 1 {
		t.Fatalf("tool responses before final request = %d, want 1", toolResponseCount)
	}
}

func TestCheckerClassifiesGenericFailure(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{{err: errors.New("boom")}},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredNoTools().Status != StatusFailed {
		t.Fatalf("status = %s, want failed", result.ConfiguredNoTools().Status)
	}
}

func TestCheckerClassifiesUnsupportedReasoningEffort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "reasoning_effort unsupported"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := llm.NewOpenAIClient(server.URL, "token", "model")
	result := New(client, config.Profile{Model: "model", BaseURL: server.URL, APIKey: "token", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredNoTools().Status != StatusUnsupported {
		t.Fatalf("status = %s error=%s, want unsupported", result.ConfiguredNoTools().Status, result.ConfiguredNoTools().Error)
	}
}

func TestCheckerRequiresToolSequence(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_file", Name: "inspect_file", Arguments: `{"path":"README.md"}`}}}},
		},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredTools().Status != StatusFailed {
		t.Fatalf("tools status = %s, want failed", result.ConfiguredTools().Status)
	}
}

func TestToolDefinitionsCanSelectListFilesOnly(t *testing.T) {
	definitions, err := toolDefinitions("list_files")
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 || definitions[0].Name != "list_files" {
		t.Fatalf("definitions = %#v, want only list_files", definitions)
	}
}

func TestToolDefinitionsRejectUnknownTool(t *testing.T) {
	if _, err := toolDefinitions("missing_tool"); err == nil {
		t.Fatal("toolDefinitions should reject unknown tools")
	}
}

func TestCheckerRunsJSONOutputProbeWhenSchemaDisabled(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: false}).Run(context.Background())
	if result.UseJSONSchema {
		t.Fatal("UseJSONSchema should be false")
	}
	if result.ConfiguredJSONOutput().Status != StatusOK {
		t.Fatalf("json-output status = %s error=%s", result.ConfiguredJSONOutput().Status, result.ConfiguredJSONOutput().Error)
	}
	if result.ConfiguredJSONSchema().Error != "probe did not run" {
		t.Fatalf("json-schema probe should not have run, got error=%s", result.ConfiguredJSONSchema().Error)
	}
}

func TestCheckerJSONOutputProbeFailsOnUnparseable(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: "not json at all"}},
		},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredJSONOutput().Status != StatusFailed {
		t.Fatalf("json-output status = %s, want failed", result.ConfiguredJSONOutput().Status)
	}
}

func TestCheckerJSONOutputProbeFailsOnWrongShape(t *testing.T) {
	for _, raw := range []string{`{}`, `[]`, `{"check":"json_capability","status":"ok","confidence_score":0.9,"extra":true}`} {
		t.Run(raw, func(t *testing.T) {
			client := &scriptedClient{
				responses: []scriptedResponse{
					{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
					{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
					{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
					{resp: &llm.ReviewResponse{RawResponse: raw}},
				},
			}
			result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
			if result.ConfiguredJSONOutput().Status != StatusFailed {
				t.Fatalf("json-output status = %s, want failed", result.ConfiguredJSONOutput().Status)
			}
		})
	}
}

func TestCheckerRunsJSONSchemaProbeWhenSchemaEnabled(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: true}).Run(context.Background())
	if !result.UseJSONSchema {
		t.Fatal("UseJSONSchema should be true")
	}
	if result.ConfiguredJSONSchema().Status != StatusOK {
		t.Fatalf("json-schema status = %s error=%s", result.ConfiguredJSONSchema().Status, result.ConfiguredJSONSchema().Error)
	}
	if result.ConfiguredJSONOutput().Error != "probe did not run" {
		t.Fatalf("json-output probe should not have run, got error=%s", result.ConfiguredJSONOutput().Error)
	}
}

func TestCheckerJSONSchemaProbeSetsSchemOnRequest(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	New(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: true}).Run(context.Background())
	// json-schema probe is request index 3 (after no-tools, 2 tools rounds, json-schema).
	schemaReq := client.reqs[3]
	if len(schemaReq.Schema) == 0 {
		t.Fatal("json-schema probe request must have Schema set")
	}
	if schemaReq.SchemaKind != llm.SchemaKindJSON {
		t.Fatalf("schema kind = %v, want json", schemaReq.SchemaKind)
	}
	if string(schemaReq.Schema) == string(llm.FindingsSchema) {
		t.Fatal("json-schema probe should use simple modelcheck schema, not findings schema")
	}
}
