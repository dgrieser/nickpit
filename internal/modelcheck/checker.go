package modelcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

type Status string

const (
	StatusOK          Status = "ok"
	StatusUnsupported Status = "unsupported"
	StatusFailed      Status = "failed"
)

const finalSentinel = "NICKPIT_MODEL_CHECK_OK"

type ProbeResult struct {
	Name            string `json:"name"`
	ReasoningEffort string `json:"reasoning_effort"`
	Tools           bool   `json:"tools"`
	Status          Status `json:"status"`
	Error           string `json:"error,omitempty"`
}

type Result struct {
	Model            string        `json:"model"`
	ConfiguredEffort string        `json:"configured_reasoning_effort"`
	Probes           []ProbeResult `json:"probes"`
	PassedEfforts    []string      `json:"passed_efforts"`
}

type Checker struct {
	client  llm.Client
	profile config.Profile
}

func New(client llm.Client, profile config.Profile) *Checker {
	return &Checker{client: client, profile: profile}
}

func (c *Checker) Run(ctx context.Context) Result {
	configured := strings.ToLower(strings.TrimSpace(c.profile.ReasoningEffort))
	if configured == "" {
		configured = config.DefaultReasoningEffort
	}
	result := Result{
		Model:            c.profile.Model,
		ConfiguredEffort: configured,
	}

	result.Probes = append(result.Probes, c.noToolsProbe(ctx, "configured_no_tools", configured))
	result.Probes = append(result.Probes, c.toolsProbe(ctx, configured))
	for _, effort := range llm.KnownReasoningEfforts() {
		if effort == configured {
			continue
		}
		result.Probes = append(result.Probes, c.noToolsProbe(ctx, "fallback_no_tools", effort))
	}
	result.PassedEfforts = passedEfforts(result.Probes)
	return result
}

func (r Result) ConfiguredNoTools() ProbeResult {
	return r.probeByName("configured_no_tools")
}

func (r Result) ConfiguredTools() ProbeResult {
	return r.probeByName("configured_tools")
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
	resp, err := c.client.Review(ctx, c.baseRequest(effort, []llm.Message{
		{Role: "system", Content: "You are checking whether this model can answer simple NickPit health checks."},
		{Role: "user", Content: "Reply exactly with " + finalSentinel + "."},
	}, nil))
	if err != nil {
		return classifyProbeError(probe, err)
	}
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
	engine := newMemoryEngine(map[string]string{
		"README.md":       "# Fixture\nNickPit model check fixture.\n",
		"internal/app.go": "package internal\n\nfunc Check() string { return \"ok\" }\n",
	})
	messages := []llm.Message{
		{Role: "system", Content: "You are checking whether this model can use NickPit retrieval tools."},
		{Role: "user", Content: "Call list_files for the repo root. Then call inspect_file for every listed file. After all files are inspected, reply exactly with " + finalSentinel + "."},
	}
	listed := false
	inspected := map[string]struct{}{}
	for round := 0; round < 8; round++ {
		resp, err := c.client.Review(ctx, c.baseRequest(effort, messages, toolDefinitions()))
		if err != nil {
			return classifyProbeError(probe, err)
		}
		if len(resp.ToolCalls) == 0 {
			if listed && allInspected(engine.files, inspected) && strings.Contains(resp.RawResponse, finalSentinel) {
				probe.Status = StatusOK
				return probe
			}
			probe.Status = StatusFailed
			probe.Error = "model stopped before required tool sequence completed"
			return probe
		}
		messages = append(messages, llm.Message{Role: "assistant", ToolCalls: resp.ToolCalls})
		for _, call := range resp.ToolCalls {
			content, err := executeToolCall(ctx, engine, call, &listed, inspected)
			if err != nil {
				probe.Status = StatusFailed
				probe.Error = err.Error()
				return probe
			}
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Name: call.Name, Content: content})
		}
	}
	probe.Status = StatusFailed
	probe.Error = "tool probe exceeded maximum rounds"
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

func toolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "list_files",
			Description: "List files in the in-memory fixture repository",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"depth":{"type":"integer","minimum":1}},"additionalProperties":false}`),
		},
		{
			Name:        "inspect_file",
			Description: "Read one file from the in-memory fixture repository",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`),
		},
	}
}

type toolArgs struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}

func executeToolCall(ctx context.Context, engine *memoryEngine, call llm.ToolCall, listed *bool, inspected map[string]struct{}) (string, error) {
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
		inspected[content.Path] = struct{}{}
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

func allInspected(files map[string]string, inspected map[string]struct{}) bool {
	if len(files) != len(inspected) {
		return false
	}
	for path := range files {
		if _, ok := inspected[path]; !ok {
			return false
		}
	}
	return true
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
