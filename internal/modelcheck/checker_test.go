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

func successfulEffortDiscoveryResponses(after ...scriptedResponse) []scriptedResponse {
	responses := make([]scriptedResponse, 0, len(llm.KnownReasoningEfforts())+len(after))
	for range llm.KnownReasoningEfforts() {
		responses = append(responses, scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}})
	}
	return append(responses, after...)
}

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
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
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
	var firstToolReq *llm.ReviewRequest
	for _, req := range client.reqs {
		if len(req.Tools) > 0 {
			firstToolReq = req
			break
		}
	}
	if firstToolReq == nil || len(firstToolReq.Tools) != 1 || firstToolReq.Tools[0].Name != "list_files" {
		t.Fatalf("tool probe tools = %#v, want only list_files", firstToolReq)
	}
	toolResponseCount := 0
	for _, msg := range client.reqs[len(llm.KnownReasoningEfforts())+1].Messages {
		if msg.Role == "tool" {
			toolResponseCount++
		}
	}
	if toolResponseCount != 1 {
		t.Fatalf("tool responses before final request = %d, want 1", toolResponseCount)
	}
}

func TestCheckerReasoningSnippetIncludesLoopDetectedGuidance(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
		},
	}
	runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
	if len(client.reqs) == 0 || len(client.reqs[0].Messages) == 0 {
		t.Fatal("expected captured model check request")
	}
	systemPrompt := client.reqs[0].Messages[0].Content
	if !strings.Contains(systemPrompt, "AVOID overthinking or repeating yourself") {
		t.Fatalf("system prompt missing loop-detected reasoning guidance:\n%s", systemPrompt)
	}
}

func TestEffortDiscoveryRetriesReasoningLoopWithSameEffort(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{err: &llm.ReasoningLoopDetectedError{ReasoningEffort: "high"}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel, ReasoningEffort: "high"}},
		},
	}
	result := runSequential(client, config.Profile{
		Model:                      "model",
		ReasoningEffort:            "high",
		MaxOutputRetries:           1,
		MaxReasoningSeconds:        12,
		MaxReasoningLoopRepeats:    3,
		MaxOutputRetriesConfigured: true,
	})
	if result.ConfiguredNoTools().Status != StatusOK {
		t.Fatalf("configured no-tools status = %s error=%s", result.ConfiguredNoTools().Status, result.ConfiguredNoTools().Error)
	}
	if len(client.reqs) < 2 {
		t.Fatalf("requests = %d, want retry", len(client.reqs))
	}
	if client.reqs[0].ReasoningEffort != "high" || client.reqs[1].ReasoningEffort != "high" {
		t.Fatalf("reasoning efforts = %q, %q; want same effort high", client.reqs[0].ReasoningEffort, client.reqs[1].ReasoningEffort)
	}
	if !client.reqs[0].DisableReasoningEffortFallback || !client.reqs[1].DisableReasoningEffortFallback {
		t.Fatal("effort discovery requests must disable reasoning-effort fallback")
	}
	if client.reqs[0].MaxReasoning != 12*time.Second || client.reqs[0].MaxReasoningLoopRepeats != 3 {
		t.Fatalf("timeout settings = %s/%d, want 12s/3", client.reqs[0].MaxReasoning, client.reqs[0].MaxReasoningLoopRepeats)
	}
}

func TestCapabilityProbesAllowReviewLikeFallback(t *testing.T) {
	client := &scriptedClient{
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
	if result.ConfiguredTools().Status != StatusOK {
		t.Fatalf("tools status = %s error=%s", result.ConfiguredTools().Status, result.ConfiguredTools().Error)
	}
	var effortDiscovery, capability *llm.ReviewRequest
	for _, req := range client.reqs {
		if len(req.Tools) == 0 && req.SchemaKind == llm.SchemaKindText && effortDiscovery == nil {
			effortDiscovery = req
		}
		if len(req.Tools) > 0 {
			capability = req
			break
		}
	}
	if effortDiscovery == nil || !effortDiscovery.DisableReasoningEffortFallback {
		t.Fatalf("effort discovery DisableReasoningEffortFallback = %v, want true", effortDiscovery != nil && effortDiscovery.DisableReasoningEffortFallback)
	}
	if capability == nil || capability.DisableReasoningEffortFallback {
		t.Fatalf("capability DisableReasoningEffortFallback = %v, want false", capability != nil && capability.DisableReasoningEffortFallback)
	}
}

func TestJSONCapabilityProbeRetriesWrongShape(t *testing.T) {
	client := &scriptedClient{
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: `{"check":"wrong"}`}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", MaxOutputRetries: 1, MaxOutputRetriesConfigured: true})
	if result.ConfiguredJSONOutput().Status != StatusOK {
		t.Fatalf("json output status = %s error=%s", result.ConfiguredJSONOutput().Status, result.ConfiguredJSONOutput().Error)
	}
	jsonRequests := 0
	var retryReq *llm.ReviewRequest
	for _, req := range client.reqs {
		if req.SchemaKind == llm.SchemaKindJSON && len(req.Schema) == 0 {
			jsonRequests++
			retryReq = req
		}
	}
	if jsonRequests != 2 {
		t.Fatalf("json output requests = %d, want initial + retry", jsonRequests)
	}
	if retryReq == nil || !strings.Contains(retryReq.Messages[len(retryReq.Messages)-1].Content, "previous response did not match") {
		t.Fatalf("retry request missing feedback: %#v", retryReq)
	}
}

func TestJSONCapabilityProbeFailsAfterRetryExhausted(t *testing.T) {
	client := &scriptedClient{
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: `{"check":"wrong"}`}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: `{"check":"still_wrong"}`}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", MaxOutputRetries: 1, MaxOutputRetriesConfigured: true})
	if result.ConfiguredJSONOutput().Status != StatusFailed {
		t.Fatalf("json output status = %s, want failed", result.ConfiguredJSONOutput().Status)
	}
	if !strings.Contains(result.ConfiguredJSONOutput().Error, "response does not match JSON probe shape") {
		t.Fatalf("json output error = %q", result.ConfiguredJSONOutput().Error)
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

func TestCheckerUsesDiscoveredEffortForSimpleProbes(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{err: errors.New("configured effort unavailable")},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		},
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high"})
	if result.ConfiguredNoTools().Status != StatusFailed {
		t.Fatalf("configured no-tools status = %s, want failed", result.ConfiguredNoTools().Status)
	}
	if result.ConfiguredTools().ReasoningEffort != "max" {
		t.Fatalf("tools effort = %q, want max", result.ConfiguredTools().ReasoningEffort)
	}
	if result.ConfiguredTools().Status != StatusOK {
		t.Fatalf("tools status = %s error=%s", result.ConfiguredTools().Status, result.ConfiguredTools().Error)
	}
	if got, want := strings.Join(result.PassedEfforts, ","), "max"; got != want {
		t.Fatalf("passed efforts = %s, want %s", got, want)
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
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
	}
	client.responses[0].resp.Reasoned = true
	var stderr bytes.Buffer
	logger := logging.New(&stderr, false, false)
	logger.SetShowProgress(true)
	checker := New(client, config.Profile{Model: "model", ReasoningEffort: "high"})
	checker.SetParallel(false)
	checker.SetLogger(logger)

	checker.Run(context.Background())

	got := stderr.String()
	for _, want := range []string{
		"ModelCheck [probe: Probe Response:high · model] start tools=false",
		"Request    [probe: Probe Response:high · model] #1 sent",
		"Response   [probe: Probe Response:high · model] #1 done",
		"ModelCheck [probe: Probe Response:high · model] ok reasoned=true",
		"Tool       [probe: Probe Tool Calling:high · model] ok list_files",
		"ModelCheck [probe: Probe Plain JSON Response:high · model] ok",
		"ModelCheck [probe: Probe Reasoning Efforts:",
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
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_file", Name: "inspect_file", Arguments: `{"path":"README.md"}`}}}},
		),
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
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
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
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: "not json at all"}},
		),
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
				responses: successfulEffortDiscoveryResponses(
					scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
					scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
					scriptedResponse{resp: &llm.ReviewResponse{RawResponse: raw}},
				),
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
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
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
		responses: successfulEffortDiscoveryResponses(
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
	}
	runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: true})
	schemaReq := client.reqs[len(client.reqs)-1]
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
