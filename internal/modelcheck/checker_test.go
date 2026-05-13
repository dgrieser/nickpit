package modelcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	toolcatalog "github.com/dgrieser/nickpit/internal/tools"
)

type scriptedClient struct {
	mu        sync.Mutex
	responses []scriptedResponse
	reqs      []*llm.ReviewRequest
}

type scriptedResponse struct {
	resp *llm.ReviewResponse
	err  error
}

const validJSONProbeResponse = `{"check":"json_capability","status":"ok","confidence_score":0.9}`

func (s *scriptedClient) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func runSequential(client llm.Client, profile config.Profile) Result {
	checker := New(client, profile)
	checker.SetParallel(false)
	return checker.Run(context.Background())
}

type concurrentClient struct {
	mu        sync.Mutex
	active    int
	maxActive int
}

func (c *concurrentClient) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	c.mu.Lock()
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
	}()

	time.Sleep(20 * time.Millisecond)
	if len(req.Tools) > 0 && !hasToolMessage(req.Messages) {
		return &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}, nil
	}
	if req.SchemaKind == llm.SchemaKindJSON {
		return &llm.ReviewResponse{RawResponse: validJSONProbeResponse}, nil
	}
	return &llm.ReviewResponse{RawResponse: finalSentinel}, nil
}

func (c *concurrentClient) MaxActive() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxActive
}

func hasToolMessage(messages []llm.Message) bool {
	for _, msg := range messages {
		if msg.Role == "tool" {
			return true
		}
	}
	return false
}

func TestCheckerRunsToolProbeWithInMemoryFixture(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
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

func TestResultSummaryIncludesCompatibility(t *testing.T) {
	result := Result{
		Probes: []ProbeResult{
			{Name: "configured_no_tools", ReasoningEffort: "high", Status: StatusOK},
			{Name: "configured_tools", ReasoningEffort: "high", Tools: true, Status: StatusOK},
			{Name: "configured_json_output", ReasoningEffort: "high", Status: StatusOK},
			{Name: "configured_json_schema", ReasoningEffort: "high", Status: StatusFailed},
		},
		PassedEfforts: []string{"high"},
	}
	summary := result.Summary()
	if !summary.Compatible {
		t.Fatal("summary should be compatible when response, tools, and JSON output pass")
	}
	if summary.JSONSchema == nil || *summary.JSONSchema {
		t.Fatalf("json schema = %v, want false", summary.JSONSchema)
	}
}

func TestCheckerRunsProbesInParallelByDefault(t *testing.T) {
	client := &concurrentClient{}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if client.MaxActive() < 2 {
		t.Fatalf("max active requests = %d, want parallel probes", client.MaxActive())
	}
	if result.ConfiguredTools().Status != StatusOK {
		t.Fatalf("tools status = %s error=%s", result.ConfiguredTools().Status, result.ConfiguredTools().Error)
	}
}

func TestCheckerCanDisableParallelProbes(t *testing.T) {
	client := &concurrentClient{}
	checker := New(client, config.Profile{Model: "model", ReasoningEffort: "high"})
	checker.SetParallel(false)
	checker.Run(context.Background())
	if client.MaxActive() != 1 {
		t.Fatalf("max active requests = %d, want sequential probes", client.MaxActive())
	}
}

func TestCheckerShowProgressLogsProbeRequestsResponsesAndResults(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel, Reasoned: true}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	var stderr bytes.Buffer
	logger := logging.New(&stderr, false, false)
	logger.SetShowProgress(true)
	checker := New(client, config.Profile{Model: "model", ReasoningEffort: "high"})
	checker.SetParallel(false)
	checker.SetLogger(logger)

	checker.Run(context.Background())

	got := stderr.String()
	for _, want := range []string{
		"ModelCheck: probe=configured_no_tools, effort=high, tools=false",
		"Request: [configured_no_tools:high] #1",
		"Response: [configured_no_tools:high] #1 status=ok",
		"ModelCheck: probe=configured_no_tools, effort=high, status=ok, reasoned=true",
		"Tool: list_files status=ok",
		"ModelCheck: probe=configured_json_output, effort=high, status=ok",
		"ModelCheck: probe=fallback_no_tools, effort=",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("progress log missing %q\nlog:\n%s", want, got)
		}
	}
}

func TestCheckerClassifiesGenericFailure(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{{err: errors.New("boom")}},
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
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
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
	if result.ConfiguredTools().Status != StatusFailed {
		t.Fatalf("tools status = %s, want failed", result.ConfiguredTools().Status)
	}
}

func TestToolDefinitionsCanSelectListFilesOnly(t *testing.T) {
	definitions, err := toolcatalog.Definitions("list_files")
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 || definitions[0].Name != "list_files" {
		t.Fatalf("definitions = %#v, want only list_files", definitions)
	}
}

func TestToolDefinitionsRejectUnknownTool(t *testing.T) {
	if _, err := toolcatalog.Definitions("missing_tool"); err == nil {
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
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: false})
	if result.UseJSONSchema {
		t.Fatal("UseJSONSchema should be false")
	}
	if result.ConfiguredJSONOutput().Status != StatusOK {
		t.Fatalf("json-output status = %s error=%s", result.ConfiguredJSONOutput().Status, result.ConfiguredJSONOutput().Error)
	}
	if result.ConfiguredJSONSchema().Status != StatusOK {
		t.Fatalf("json-schema status = %s error=%s", result.ConfiguredJSONSchema().Status, result.ConfiguredJSONSchema().Error)
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
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
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
			result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
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
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: true})
	if !result.UseJSONSchema {
		t.Fatal("UseJSONSchema should be true")
	}
	if result.ConfiguredJSONSchema().Status != StatusOK {
		t.Fatalf("json-schema status = %s error=%s", result.ConfiguredJSONSchema().Status, result.ConfiguredJSONSchema().Error)
	}
	if result.ConfiguredJSONOutput().Status != StatusOK {
		t.Fatalf("json-output status = %s error=%s", result.ConfiguredJSONOutput().Status, result.ConfiguredJSONOutput().Error)
	}
}

func TestCheckerJSONSchemaProbeSetsSchemOnRequest(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: true})
	// json-schema probe is request index 4 (after no-tools, 2 tools rounds, json-output, json-schema).
	schemaReq := client.reqs[4]
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
