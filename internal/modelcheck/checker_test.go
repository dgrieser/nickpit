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

// successfulEffortDiscoveryResponses scripts the no-tools effort-discovery
// responses for the given configured effort: one for the configured effort plus
// one for each lower effort Run() probes (LowerReasoningEfforts). Any extra
// responses for the simple probes follow via after.
func successfulEffortDiscoveryResponses(effort string, after ...scriptedResponse) []scriptedResponse {
	count := 1 + len(llm.LowerReasoningEfforts(effort))
	responses := make([]scriptedResponse, 0, count+len(after))
	for range count {
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
		responses: successfulEffortDiscoveryResponses("high",
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
	for _, req := range client.reqs {
		count := 0
		for _, msg := range req.Messages {
			if msg.Role == "tool" {
				count++
			}
		}
		if count > toolResponseCount {
			toolResponseCount = count
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
		responses: successfulEffortDiscoveryResponses("high",
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
		responses: successfulEffortDiscoveryResponses("high",
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
		responses: successfulEffortDiscoveryResponses("high",
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

func TestSimpleProbeEffortFallsBackToConfiguredWhenNoToolsFails(t *testing.T) {
	result := Result{
		ConfiguredEffort: "medium",
		Probes: []ProbeResult{
			{Name: "configured_no_tools", ReasoningEffort: "medium", Status: StatusFailed},
		},
		// A lower effort passed during discovery; simpleProbeEffort must NOT use
		// it (the highest-passing fallback was removed) and return the configured
		// effort instead.
		PassedEfforts: []string{"low"},
	}
	if got := result.simpleProbeEffort(); got != "medium" {
		t.Fatalf("simpleProbeEffort() = %q, want medium (configured)", got)
	}
}

func TestSimpleProbeEffortUsesHighestPassingWhenConfiguredUnsupported(t *testing.T) {
	result := Result{
		ConfiguredEffort: "high",
		Probes: []ProbeResult{
			{Name: "configured_no_tools", ReasoningEffort: "high", Status: StatusUnsupported},
		},
		// Configured effort is permanently unsupported; runtime falls back to the
		// highest passing lower effort, so simple probes must run there.
		PassedEfforts: []string{"medium", "low"},
	}
	if got := result.simpleProbeEffort(); got != "medium" {
		t.Fatalf("simpleProbeEffort() = %q, want medium (highest passing fallback)", got)
	}
}

func TestSimpleProbeEffortEmptyWhenNoEffortPassed(t *testing.T) {
	for _, status := range []Status{StatusFailed, StatusUnsupported} {
		result := Result{
			ConfiguredEffort: "high",
			Probes: []ProbeResult{
				{Name: "configured_no_tools", ReasoningEffort: "high", Status: status},
			},
			PassedEfforts: nil,
		}
		if got := result.simpleProbeEffort(); got != "" {
			t.Fatalf("simpleProbeEffort() with status %s and no passed efforts = %q, want empty", status, got)
		}
	}
}

func TestAnyErrorRetryable(t *testing.T) {
	if !anyErrorRetryable(errors.New("transient upstream blip"), "high") {
		t.Fatal("plain non-rejection error should be retryable")
	}
	if !anyErrorRetryable(&llm.ReasoningLoopDetectedError{ReasoningEffort: "high", RepeatedChunk: true}, "high") {
		t.Fatal("repeated-chunk loop error should be retryable")
	}
}

func TestSimpleProbeRetriesTransientLoopError(t *testing.T) {
	client := &scriptedClient{
		responses: successfulEffortDiscoveryResponses("high",
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			// schema probe: first call hits a repeated-chunk loop, retry succeeds
			scriptedResponse{err: &llm.ReasoningLoopDetectedError{ReasoningEffort: "high", RepeatedChunk: true}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
		),
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: true, MaxOutputRetries: 1, MaxOutputRetriesConfigured: true})
	if result.ConfiguredJSONSchema().Status != StatusOK {
		t.Fatalf("json schema status = %s error=%s, want ok", result.ConfiguredJSONSchema().Status, result.ConfiguredJSONSchema().Error)
	}
	if got := schemaProbeRequestCount(client); got != 2 {
		t.Fatalf("json schema requests = %d, want initial + retry", got)
	}
}

func TestSimpleProbeRetriesExhaustThenFail(t *testing.T) {
	client := &scriptedClient{
		responses: successfulEffortDiscoveryResponses("high",
			scriptedResponse{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			scriptedResponse{resp: &llm.ReviewResponse{RawResponse: validJSONProbeResponse}},
			// schema probe: persistent loop on every attempt
			scriptedResponse{err: &llm.ReasoningLoopDetectedError{ReasoningEffort: "high", RepeatedChunk: true}},
			scriptedResponse{err: &llm.ReasoningLoopDetectedError{ReasoningEffort: "high", RepeatedChunk: true}},
		),
	}
	result := runSequential(client, config.Profile{Model: "model", ReasoningEffort: "high", UseJSONSchema: true, MaxOutputRetries: 1, MaxOutputRetriesConfigured: true})
	if result.ConfiguredJSONSchema().Status != StatusFailed {
		t.Fatalf("json schema status = %s, want failed", result.ConfiguredJSONSchema().Status)
	}
	if got := schemaProbeRequestCount(client); got != 2 {
		t.Fatalf("json schema requests = %d, want MaxOutputRetries+1", got)
	}
}

func schemaProbeRequestCount(client *scriptedClient) int {
	count := 0
	for _, req := range client.reqs {
		if req.SchemaKind == llm.SchemaKindJSON && len(req.Schema) > 0 {
			count++
		}
	}
	return count
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

func TestCheckerRunsSimpleProbesAtConfiguredEffort(t *testing.T) {
	// Configured effort "high" fails the no-tools probe, but the simple probes
	// must still run at the configured effort (never a higher one), and only the
	// configured effort plus lower efforts are probed during discovery.
	client := &scriptedClient{
		responses: []scriptedResponse{
			// configured_no_tools (high) fails
			{err: errors.New("configured effort unavailable")},
			// fallback_no_tools: medium passes, low/minimal/none/off fail
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			{err: errors.New("effort unavailable")},
			// simple probes run at configured "high"
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
	if result.ConfiguredTools().ReasoningEffort != "high" {
		t.Fatalf("tools effort = %q, want high (configured)", result.ConfiguredTools().ReasoningEffort)
	}
	if result.ConfiguredTools().Status != StatusOK {
		t.Fatalf("tools status = %s error=%s", result.ConfiguredTools().Status, result.ConfiguredTools().Error)
	}
	for _, e := range result.PassedEfforts {
		if e == "max" || e == "xhigh" {
			t.Fatalf("passed efforts %v must not contain efforts above configured", result.PassedEfforts)
		}
	}
	// PassedEfforts reflects the no-tools discovery probes (computed before the
	// simple probes run): configured "high" failed, only "medium" passed.
	if got, want := strings.Join(result.PassedEfforts, ","), "medium"; got != want {
		t.Fatalf("passed efforts = %s, want %s", got, want)
	}
}

func TestNewForModelProbesGivenModelAndEffort(t *testing.T) {
	client := &scriptedClient{}
	profile := config.Profile{Model: "big-model", ReasoningEffort: "high"}
	checker := NewForModel(client, profile, "small-model", "low")
	checker.SetParallel(false)
	result := checker.Run(context.Background())

	if result.Model != "small-model" {
		t.Fatalf("result model = %q, want small-model", result.Model)
	}
	if result.ConfiguredEffort != "low" {
		t.Fatalf("configured effort = %q, want low", result.ConfiguredEffort)
	}
	for _, req := range client.reqs {
		if req.Model != "small-model" {
			t.Fatalf("probe request model = %q, want small-model", req.Model)
		}
	}
	if !sawEffort(client.reqs, "low") {
		t.Fatal("expected a probe at the configured effort \"low\"")
	}
}

func sawEffort(reqs []*llm.ReviewRequest, effort string) bool {
	for _, req := range reqs {
		if req.ReasoningEffort == effort {
			return true
		}
	}
	return false
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
		responses: successfulEffortDiscoveryResponses("high",
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
		responses: successfulEffortDiscoveryResponses("high",
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
		responses: successfulEffortDiscoveryResponses("high",
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
		responses: successfulEffortDiscoveryResponses("high",
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
				responses: successfulEffortDiscoveryResponses("high",
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
		responses: successfulEffortDiscoveryResponses("high",
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
		responses: successfulEffortDiscoveryResponses("high",
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
