package modelcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/retrieval"
	toolcatalog "github.com/dgrieser/nickpit/internal/tools"
	"github.com/dgrieser/nickpit/prompts"
)

func mustRenderCheckPrompt(name string, data any) string {
	tmpl, err := prompts.Load(name)
	if err != nil {
		panic(fmt.Sprintf("modelcheck: %v", err))
	}
	rendered, err := llm.RenderPrompt(tmpl, data)
	if err != nil {
		panic(fmt.Sprintf("modelcheck: rendering %s: %v", name, err))
	}
	return rendered
}

func checkReasoningSnippet() string {
	return mustRenderCheckPrompt("helper_reasoning_snippet.tmpl", struct {
		LoopDetected bool
	}{})
}

type Status string

const (
	StatusOK          Status = "ok"
	StatusUnsupported Status = "unsupported"
	StatusFailed      Status = "failed"
)

const finalSentinel = "NICKPIT_MODEL_CHECK_OK"

const jsonProbeExample = `{
  "check": "json_capability",
  "status": "ok",
  "confidence_score": 0.9
}`

var jsonProbeSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "check": {"type": "string", "enum": ["json_capability"]},
    "status": {"type": "string", "enum": ["ok"]},
    "confidence_score": {"type": "number", "minimum": 0, "maximum": 1}
  },
  "required": ["check", "status", "confidence_score"],
  "additionalProperties": false
}`)

type ProbeResult struct {
	Name            string `json:"name"`
	ReasoningEffort string `json:"reasoning_effort"`
	Tools           bool   `json:"tools"`
	Reasoned        bool   `json:"reasoned"`
	Status          Status `json:"status"`
	Error           string `json:"error,omitempty"`
}

type Result struct {
	Model            string        `json:"model"`
	ConfiguredEffort string        `json:"configured_reasoning_effort"`
	UseJSONSchema    bool          `json:"use_json_schema"`
	Probes           []ProbeResult `json:"probes"`
	PassedEfforts    []string      `json:"passed_efforts"`
}

type ReasoningSummary struct {
	Traces  bool     `json:"traces"`
	Efforts []string `json:"efforts"`
}

type CheckSummary struct {
	Compatible   bool             `json:"compatible"`
	Response     bool             `json:"response"`
	Reasoning    ReasoningSummary `json:"reasoning"`
	Tools        bool             `json:"tools"`
	JSONSchema   *bool            `json:"json_schema,omitempty"`
	JSONResponse *bool            `json:"json_response,omitempty"`
}

func (r Result) Summary() CheckSummary {
	noTools := r.ConfiguredNoTools()
	tools := r.ConfiguredTools()
	traces := noTools.Reasoned || tools.Reasoned
	s := CheckSummary{
		Response: noTools.Status == StatusOK,
		Reasoning: ReasoningSummary{
			Traces:  traces,
			Efforts: r.PassedEfforts,
		},
		Tools: tools.Status == StatusOK,
	}
	if p := r.ConfiguredJSONOutput(); p.Error != "probe did not run" {
		ok := p.Status == StatusOK
		s.JSONResponse = &ok
	}
	if p := r.ConfiguredJSONSchema(); p.Error != "probe did not run" {
		ok := p.Status == StatusOK
		s.JSONSchema = &ok
	}
	s.Compatible = s.Response && s.Tools && s.JSONResponse != nil && *s.JSONResponse
	return s
}

type Checker struct {
	client   llm.Client
	profile  config.Profile
	logger   *logging.Logger
	parallel bool
}

func New(client llm.Client, profile config.Profile) *Checker {
	return &Checker{client: client, profile: profile, parallel: true}
}

func (c *Checker) SetLogger(logger *logging.Logger) {
	c.logger = logger
}

func (c *Checker) SetParallel(enabled bool) {
	c.parallel = enabled
}

func (c *Checker) logProgress(label, summary string) {
	if c.logger != nil {
		c.logger.PrintProgress(label, summary)
	}
}

func (c *Checker) openSection(name string) *logging.ReasoningSection {
	if c.logger == nil {
		return nil
	}
	return c.logger.OpenReasoningSection(name)
}

func probeLabel(name, effort string) string {
	if effort == "" {
		return name
	}
	return fmt.Sprintf("%s:%s", name, effort)
}

func (c *Checker) logProbeStart(probe ProbeResult) {
	c.logProgress("ModelCheck", fmt.Sprintf("probe=%s, effort=%s, tools=%t", probe.Name, probe.ReasoningEffort, probe.Tools))
}

func (c *Checker) logProbeResult(probe ProbeResult) {
	parts := []string{
		fmt.Sprintf("probe=%s", probe.Name),
		fmt.Sprintf("effort=%s", probe.ReasoningEffort),
		fmt.Sprintf("status=%s", probe.Status),
		fmt.Sprintf("reasoned=%t", probe.Reasoned),
	}
	if probe.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%q", probe.Error))
	}
	c.logProgress("ModelCheck", strings.Join(parts, ", "))
}

func (c *Checker) reviewProbe(ctx context.Context, req *llm.ReviewRequest, sec *logging.ReasoningSection, probe ProbeResult) (*llm.ReviewResponse, error) {
	callNum := sec.IncrCallNum()
	label := sec.Label()
	if label == "" {
		label = probeLabel(probe.Name, probe.ReasoningEffort)
	}
	c.logProgress("Request", fmt.Sprintf("[%s] #%d", label, callNum))
	start := time.Now()
	resp, err := c.client.Review(ctx, req)
	elapsed := time.Since(start).Truncate(time.Second)
	status := "ok"
	if err != nil {
		status = "error"
	}
	c.logProgress("Response", fmt.Sprintf("[%s] #%d status=%s after %s", label, callNum, status, elapsed))
	return resp, err
}

func (c *Checker) Run(ctx context.Context) Result {
	configured := strings.ToLower(strings.TrimSpace(c.profile.ReasoningEffort))
	if configured == "" {
		configured = config.DefaultReasoningEffort
	}
	result := Result{
		Model:            c.profile.Model,
		ConfiguredEffort: configured,
		UseJSONSchema:    c.profile.UseJSONSchema,
	}

	probes := []func() ProbeResult{
		func() ProbeResult { return c.noToolsProbe(ctx, "configured_no_tools", configured) },
		func() ProbeResult { return c.toolsProbe(ctx, configured) },
		func() ProbeResult { return c.jsonOutputProbe(ctx, configured) },
		func() ProbeResult { return c.jsonSchemaProbe(ctx, configured) },
	}
	for _, effort := range llm.KnownReasoningEfforts() {
		if effort == configured {
			continue
		}
		effort := effort
		probes = append(probes, func() ProbeResult { return c.noToolsProbe(ctx, "fallback_no_tools", effort) })
	}
	result.Probes = c.runProbes(probes)
	result.PassedEfforts = passedEfforts(result.Probes)
	return result
}

func (c *Checker) runProbes(probes []func() ProbeResult) []ProbeResult {
	results := make([]ProbeResult, len(probes))
	if !c.parallel {
		for i, probe := range probes {
			results[i] = probe()
		}
		return results
	}

	var wg sync.WaitGroup
	for i, probe := range probes {
		wg.Add(1)
		go func(i int, probe func() ProbeResult) {
			defer wg.Done()
			results[i] = probe()
		}(i, probe)
	}
	wg.Wait()
	return results
}

func (r Result) ConfiguredNoTools() ProbeResult {
	return r.probeByName("configured_no_tools")
}

func (r Result) ConfiguredTools() ProbeResult {
	return r.probeByName("configured_tools")
}

func (r Result) ConfiguredJSONOutput() ProbeResult {
	return r.probeByName("configured_json_output")
}

func (r Result) ConfiguredJSONSchema() ProbeResult {
	return r.probeByName("configured_json_schema")
}

func (r Result) probeByName(name string) ProbeResult {
	for _, probe := range r.Probes {
		if probe.Name == name {
			return probe
		}
	}
	return ProbeResult{Name: name, Status: StatusFailed, Error: "probe did not run"}
}

func (c *Checker) noToolsProbe(ctx context.Context, name, effort string) ProbeResult {
	probe := ProbeResult{Name: name, ReasoningEffort: effort}
	c.logProbeStart(probe)
	defer func() { c.logProbeResult(probe) }()
	sec := c.openSection(probeLabel(name, effort))
	defer sec.End()
	rs := checkReasoningSnippet()
	req := c.baseRequest(effort, []llm.Message{
		{Role: "system", Content: mustRenderCheckPrompt("check_no_tools_system.tmpl", struct{ ReasoningSnippet string }{rs})},
		{Role: "user", Content: mustRenderCheckPrompt("check_no_tools_user.tmpl", struct{ Sentinel string }{finalSentinel})},
	}, nil)
	if sec != nil {
		req.ReasoningSink = sec
	}
	resp, err := c.reviewProbe(ctx, req, sec, probe)
	if err != nil {
		probe = classifyProbeError(probe, err)
		return probe
	}
	probe.Reasoned = resp.Reasoned
	if !strings.Contains(resp.RawResponse, finalSentinel) {
		probe.Status = StatusFailed
		probe.Error = "response did not contain sentinel"
		return probe
	}
	probe.Status = StatusOK
	return probe
}

func (c *Checker) toolsProbe(ctx context.Context, effort string) ProbeResult {
	probe := ProbeResult{Name: "configured_tools", ReasoningEffort: effort, Tools: true}
	c.logProbeStart(probe)
	defer func() { c.logProbeResult(probe) }()
	sec := c.openSection(probeLabel("configured_tools", effort))
	defer sec.End()
	rs := checkReasoningSnippet()
	engine := newMemoryEngine(map[string]string{
		"README.md":       "# Fixture\nNickPit model check fixture.\n",
		"internal/app.go": "package internal\n\nfunc Check() string { return \"ok\" }\n",
	})
	messages := []llm.Message{
		{Role: "system", Content: mustRenderCheckPrompt("check_tools_system.tmpl", struct{ ReasoningSnippet string }{rs})},
		{Role: "user", Content: mustRenderCheckPrompt("check_tools_user.tmpl", struct{ Sentinel string }{finalSentinel})},
	}
	listed := false
	tools, err := toolcatalog.Definitions("list_files")
	if err != nil {
		probe.Status = StatusFailed
		probe.Error = err.Error()
		return probe
	}
	allowedTools := toolSet(tools)
	for round := 0; round < 8; round++ {
		req := c.baseRequest(effort, messages, tools)
		if sec != nil {
			req.ReasoningSink = sec
		}
		resp, err := c.reviewProbe(ctx, req, sec, probe)
		if err != nil {
			probe = classifyProbeError(probe, err)
			return probe
		}
		probe.Reasoned = probe.Reasoned || resp.Reasoned
		if len(resp.ToolCalls) == 0 {
			if listed && strings.Contains(resp.RawResponse, finalSentinel) {
				probe.Status = StatusOK
				return probe
			}
			probe.Status = StatusFailed
			probe.Error = "model stopped before required tool sequence completed"
			return probe
		}
		messages = append(messages, llm.Message{Role: "assistant", ToolCalls: resp.ToolCalls})
		for _, call := range resp.ToolCalls {
			content, err := executeToolCall(ctx, engine, call, allowedTools, &listed)
			if err != nil {
				c.logProgress("Tool", fmt.Sprintf("%s status=error, error=%q", call.Name, err.Error()))
				probe.Status = StatusFailed
				probe.Error = err.Error()
				return probe
			}
			c.logProgress("Tool", fmt.Sprintf("%s status=ok", call.Name))
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Name: call.Name, Content: content})
		}
	}
	probe.Status = StatusFailed
	probe.Error = "tool probe exceeded maximum rounds"
	return probe
}

func (c *Checker) jsonOutputProbe(ctx context.Context, effort string) ProbeResult {
	probe := ProbeResult{Name: "configured_json_output", ReasoningEffort: effort}
	c.logProbeStart(probe)
	defer func() { c.logProbeResult(probe) }()
	sec := c.openSection(probeLabel("configured_json_output", effort))
	defer sec.End()
	rs := checkReasoningSnippet()
	req := c.baseRequest(effort, []llm.Message{
		{Role: "system", Content: mustRenderCheckPrompt("check_json_output_system.tmpl", struct{ ReasoningSnippet string }{rs})},
		{Role: "user", Content: mustRenderCheckPrompt("check_json_output_user.tmpl", struct{ OutputSchemaSnippet string }{jsonProbeExample})},
	}, nil)
	req.SchemaKind = llm.SchemaKindJSON
	if sec != nil {
		req.ReasoningSink = sec
	}
	resp, err := c.reviewProbe(ctx, req, sec, probe)
	if err != nil {
		probe = classifyProbeError(probe, err)
		return probe
	}
	probe.Reasoned = resp.Reasoned
	if err := validateJSONProbeResponse(resp.RawResponse); err != nil {
		probe.Status = StatusFailed
		probe.Error = err.Error()
		return probe
	}
	probe.Status = StatusOK
	return probe
}

func (c *Checker) jsonSchemaProbe(ctx context.Context, effort string) ProbeResult {
	probe := ProbeResult{Name: "configured_json_schema", ReasoningEffort: effort}
	c.logProbeStart(probe)
	defer func() { c.logProbeResult(probe) }()
	sec := c.openSection(probeLabel("configured_json_schema", effort))
	defer sec.End()
	rs := checkReasoningSnippet()
	req := c.baseRequest(effort, []llm.Message{
		{Role: "system", Content: mustRenderCheckPrompt("check_json_schema_system.tmpl", struct{ ReasoningSnippet string }{rs})},
		{Role: "user", Content: mustRenderCheckPrompt("check_json_schema_user.tmpl", struct{ OutputSchemaSnippet string }{jsonProbeExample})},
	}, nil)
	req.Schema = jsonProbeSchema
	req.SchemaKind = llm.SchemaKindJSON
	if sec != nil {
		req.ReasoningSink = sec
	}
	resp, err := c.reviewProbe(ctx, req, sec, probe)
	if err != nil {
		probe = classifyProbeError(probe, err)
		return probe
	}
	probe.Reasoned = resp.Reasoned
	if err := validateJSONProbeResponse(resp.RawResponse); err != nil {
		probe.Status = StatusFailed
		probe.Error = err.Error()
		return probe
	}
	probe.Status = StatusOK
	return probe
}

func (c *Checker) baseRequest(effort string, messages []llm.Message, tools []llm.ToolDefinition) *llm.ReviewRequest {
	maxReasoning := time.Duration(c.profile.MaxReasoningSeconds) * time.Second
	return &llm.ReviewRequest{
		Messages:          append([]llm.Message(nil), messages...),
		Tools:             tools,
		SchemaKind:        llm.SchemaKindText,
		Model:             c.profile.Model,
		MaxTokens:         c.profile.MaxTokens,
		Temperature:       c.profile.Temperature,
		TopP:              c.profile.TopP,
		ExtraBody:         c.profile.ExtraBody,
		ParallelToolCalls: true,
		ReasoningEffort:   effort,
		MaxReasoning:      maxReasoning,
		SingleAttempt:     true,
	}
}

func classifyProbeError(probe ProbeResult, err error) ProbeResult {
	if llm.IsReasoningEffortRejection(err, probe.ReasoningEffort) {
		probe.Status = StatusUnsupported
	} else {
		probe.Status = StatusFailed
	}
	probe.Error = err.Error()
	return probe
}

func passedEfforts(probes []ProbeResult) []string {
	seen := map[string]struct{}{}
	for _, probe := range probes {
		if probe.Status == StatusOK {
			seen[probe.ReasoningEffort] = struct{}{}
		}
	}
	efforts := make([]string, 0, len(seen))
	for _, effort := range llm.KnownReasoningEfforts() {
		if _, ok := seen[effort]; ok {
			efforts = append(efforts, effort)
			delete(seen, effort)
		}
	}
	extra := make([]string, 0, len(seen))
	for effort := range seen {
		extra = append(extra, effort)
	}
	sort.Strings(extra)
	efforts = append(extra, efforts...)
	return efforts
}

func toolSet(definitions []llm.ToolDefinition) map[string]struct{} {
	set := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		set[definition.Name] = struct{}{}
	}
	return set
}

type toolArgs struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}

func executeToolCall(ctx context.Context, engine *memoryEngine, call llm.ToolCall, allowed map[string]struct{}, listed *bool) (string, error) {
	if _, ok := allowed[call.Name]; !ok {
		return "", fmt.Errorf("unsupported tool %q", call.Name)
	}
	var args toolArgs
	if strings.TrimSpace(call.Arguments) != "" {
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments for %s: %w", call.Name, err)
		}
	}
	switch call.Name {
	case "list_files":
		if *listed {
			return "", fmt.Errorf("list_files called more than once")
		}
		if strings.Trim(args.Path, "./") != "" {
			return "", fmt.Errorf("list_files must target repo root, got %q", args.Path)
		}
		*listed = true
		listing, err := engine.ListFiles(ctx, "", "", args.Depth)
		if err != nil {
			return "", err
		}
		return marshalToolResult(listing)
	case "inspect_file":
		if !*listed {
			return "", fmt.Errorf("inspect_file called before list_files")
		}
		content, err := engine.GetFile(ctx, "", args.Path)
		if err != nil {
			return "", err
		}
		return marshalToolResult(content)
	default:
		return "", fmt.Errorf("unsupported tool %q", call.Name)
	}
}

func marshalToolResult(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func validateJSONProbeResponse(content string) error {
	var raw map[string]json.RawMessage
	if err := llm.LenientUnmarshal(content, &raw); err != nil {
		return fmt.Errorf("response is not parseable JSON object: %w", err)
	}
	if len(raw) != 3 {
		return fmt.Errorf("response does not match JSON probe shape")
	}
	var check string
	if err := json.Unmarshal(raw["check"], &check); err != nil || check != "json_capability" {
		return fmt.Errorf("response does not match JSON probe shape")
	}
	var status string
	if err := json.Unmarshal(raw["status"], &status); err != nil || status != "ok" {
		return fmt.Errorf("response does not match JSON probe shape")
	}
	var confidence float64
	if err := json.Unmarshal(raw["confidence_score"], &confidence); err != nil || confidence < 0 || confidence > 1 {
		return fmt.Errorf("response does not match JSON probe shape")
	}
	return nil
}

type memoryEngine struct {
	files map[string]string
}

func newMemoryEngine(files map[string]string) *memoryEngine {
	return &memoryEngine{files: files}
}

func (e *memoryEngine) GetFile(_ context.Context, _, path string) (*retrieval.FileContent, error) {
	path = strings.TrimPrefix(path, "./")
	content, ok := e.files[path]
	if !ok {
		return nil, fmt.Errorf("file %q not found", path)
	}
	return &retrieval.FileContent{Path: path, Content: content, Language: languageForPath(path)}, nil
}

func (e *memoryEngine) ListFiles(context.Context, string, string, int) (*retrieval.DirectoryListing, error) {
	files := make([]string, 0, len(e.files))
	for path := range e.files {
		files = append(files, path)
	}
	sort.Strings(files)
	return &retrieval.DirectoryListing{Path: ".", Files: files}, nil
}

func (e *memoryEngine) GetFileSlice(ctx context.Context, repoRoot, path string, _, _ int) (*retrieval.FileSlice, error) {
	content, err := e.GetFile(ctx, repoRoot, path)
	if err != nil {
		return nil, err
	}
	return &retrieval.FileSlice{Path: content.Path, Content: content.Content, Language: content.Language}, nil
}

func (*memoryEngine) Search(context.Context, string, string, string, int, int, bool) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{}, nil
}

func (*memoryEngine) SearchRegex(context.Context, string, string, *regexp.Regexp, int, int) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{}, nil
}

func (*memoryEngine) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, fmt.Errorf("symbols unavailable")
}

func (*memoryEngine) FindCallers(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, fmt.Errorf("callers unavailable")
}

func (*memoryEngine) FindCallees(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, fmt.Errorf("callees unavailable")
}

func languageForPath(path string) string {
	if strings.HasSuffix(path, ".go") {
		return "go"
	}
	if strings.HasSuffix(path, ".md") {
		return "markdown"
	}
	return "text"
}
