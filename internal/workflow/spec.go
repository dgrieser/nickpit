// Package workflow defines a portable, file-driven specification of the review
// pipeline: which steps run, in what order, and with what per-step overrides.
//
// The spec is intentionally a thin orchestration layer. Anything a step does
// not override falls through to the active profile (and from there to the
// built-in defaults), so a spec that lists the steps without any `config` blocks
// reproduces the tool's default behavior exactly. DefaultSpec() is the canonical
// embedded workflow and is the single source the engine executes for an ordinary
// review.
package workflow

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/workflows"
)

// SpecVersion is the only supported major spec version.
const SpecVersion = 1

// SmallModelAlias selects profile.small for a step. Unset small fields
// intentionally fall back to the profile's primary model config.
const SmallModelAlias = "@small"

// Step type identifiers. Steps that operate on a single reviewer vector are
// addressed with a "<prefix><vector-id>" type, e.g. "review:security".
const (
	StepCollectContext = "collect-context"
	StepVerify         = "verify"
	StepDedupe         = "dedupe"
	StepMerge          = "merge"
	StepFinalize       = "finalize"
	StepVerdict        = "verdict"
	StepSummarize      = "summarize"

	StepReviewPrefix  = "review:"
	StepExtractPrefix = "reasoning-extract:"
	StepNudgePrefix   = "nudge:"
	StepVerifyPrefix  = "verify:"
	StepDedupePrefix  = "dedupe:"
)

// Scope identifiers name the work unit a step's agents operate on. Scope
// declares how a step's work is divided (what each agent invocation sees), not
// parallelism — LLM concurrency is a run-level cap (--concurrency). Dividing a
// step into N units only makes them eligible to run concurrently.
const (
	ScopeAll      = "all"      // one agent over the whole finding set (no division)
	ScopeCluster  = "cluster"  // one agent per merge cluster
	ScopeFinding  = "finding"  // one agent per finding
	ScopeReviewer = "reviewer" // one agent per reviewer group
)

// legalScopes returns the scope values a step type accepts, and whether the
// step divides its work at all. Steps that operate as a single agent
// (collect-context, review:, reasoning-extract:, nudge:) return ok=false and do
// not accept scope. The set is intentionally faithful to what the engine does
// today: scope is declarative and validated, it does not unlock new fan-out
// paths. cluster-scoped finalize/summarize is reachable only inside a pipeline
// group (the only place the engine shards them); Validate enforces that context.
func legalScopes(stepType string) ([]string, bool) {
	switch {
	case stepType == StepMerge:
		return []string{ScopeCluster}, true
	case stepType == StepFinalize, stepType == StepSummarize:
		return []string{ScopeAll, ScopeCluster}, true
	case stepType == StepVerdict:
		return []string{ScopeAll}, true
	case stepType == StepVerify || strings.HasPrefix(stepType, StepVerifyPrefix):
		return []string{ScopeFinding}, true
	case stepType == StepDedupe || strings.HasPrefix(stepType, StepDedupePrefix):
		return []string{ScopeReviewer}, true
	}
	return nil, false
}

// validateScope checks a step's declared scope value against the step type. It
// validates value legality only; the pipeline-vs-flat context rule (cluster
// finalize/summarize is pipeline-only) is enforced in Validate.
func validateScope(stepType string, cfg *StepOverride) error {
	if cfg == nil || cfg.Scope == nil {
		return nil
	}
	scope := *cfg.Scope
	legal, ok := legalScopes(stepType)
	if !ok {
		return fmt.Errorf("step %q does not divide its work, so scope is not allowed", stepType)
	}
	if !slices.Contains(legal, scope) {
		return fmt.Errorf("invalid scope %q for %q (valid: %s)", scope, stepType, strings.Join(legal, ", "))
	}
	return nil
}

// perVectorPrefixes lists every step prefix that addresses a single reviewer
// vector. Per-vector steps touch only their vector's group/session, so they are
// the only steps allowed inside parallel groups and lanes.
var perVectorPrefixes = []string{StepReviewPrefix, StepExtractPrefix, StepNudgePrefix, StepVerifyPrefix, StepDedupePrefix}

// ReviewVectorIDs is the canonical, ordered set of reviewer vector identifiers.
// The order matches the engine's reviewVectors so DefaultSpec() reproduces the
// historical reviewer ordering. internal/review maps each id to its concrete
// reviewVector.
var ReviewVectorIDs = []string{
	"codequality",
	"security",
	"architecture",
	"performance",
	"testing",
	"bestpractices",
}

// Spec is a complete workflow definition.
type Spec struct {
	Version int
	Profile string
	Steps   []StepEntry
}

// StepEntry is one entry in a step list. It is exactly one of:
//   - a plain step (Type set), optionally with Config/FindingsFrom;
//   - a parallel group (Parallel set) whose children run concurrently;
//   - a lane (Lane set): a sequential chain of plain steps, valid only as a
//     parallel child, so e.g. review→verify→dedupe of one reviewer runs in
//     order while other reviewers' lanes proceed concurrently;
//   - a pipeline (Pipeline set): the post-review tail (merge→finalize→verdict,
//     optionally summarize) executed with cross-step per-cluster streaming and
//     no barriers between the steps. This is the only way to fuse the tail —
//     there is no auto-fusion. Valid only at the top level.
type StepEntry struct {
	Type         string
	Config       *StepOverride
	FindingsFrom []string
	Parallel     []StepEntry
	Lane         []StepEntry
	Pipeline     []StepEntry
}

// IsParallel reports whether the entry is a parallel group.
func (s StepEntry) IsParallel() bool { return len(s.Parallel) > 0 }

// IsLane reports whether the entry is a lane (sequential chain in a parallel group).
func (s StepEntry) IsLane() bool { return len(s.Lane) > 0 }

// IsPipeline reports whether the entry is a pipeline group (streamed post-review tail).
func (s StepEntry) IsPipeline() bool { return len(s.Pipeline) > 0 }

// LaneSteps returns the entry's sequential chain: its Lane when set, else the
// entry itself as a single-step chain. A bare parallel child is a one-step lane.
func (s StepEntry) LaneSteps() []StepEntry {
	if s.IsLane() {
		return s.Lane
	}
	return []StepEntry{s}
}

// StepOverride layers per-step overrides onto the resolved base profile and
// review request. Every field is a pointer (or nil-able map) so "unset" is
// distinguishable from an explicit zero — an unset field inherits the
// CLI/config/profile value, exactly as a normal review does.
//
// Only parameters that take effect per step are exposed. base_url, api_key and
// the rate-limit delay are intentionally omitted: the LLM client is constructed
// once per run from the active profile, so those cannot vary per step.
type StepOverride struct {
	// Model parameters (apply to the step's engine clone).
	Model           *string        `yaml:"model"`
	Temperature     *float64       `yaml:"temperature"`
	TopP            *float64       `yaml:"top_p"`
	TopK            *int           `yaml:"top_k"`
	PresencePenalty *float64       `yaml:"presence_penalty"`
	MaxTokens       *int           `yaml:"max_tokens"`
	ExtraBody       map[string]any `yaml:"extra_body"`
	ReasoningEffort *string        `yaml:"reasoning_effort"`
	TimeBudget      *TimeBudget    `yaml:"time_budget"`

	// Scope declares the work unit this step's agents operate on (see ScopeAll
	// etc.). It makes the step's fan-out explicit and is validated against the
	// step type; it does not change behavior on its own (the engine already
	// runs each step at its only legal scope for the given context).
	Scope *string `yaml:"scope"`

	// Budgets / loop detection / retries (apply to the step's review request).
	// max_context_tokens is intentionally not per-step overridable: the review
	// context is resolved and trimmed once, before any step runs, so a per-step
	// value could not affect the prompt size. Set it on the profile / via
	// --max-context-tokens instead.
	MaxToolCalls            *int `yaml:"max_tool_calls"`
	MaxDuplicateToolCalls   *int `yaml:"max_duplicate_tool_calls"`
	MaxOutputRetries        *int `yaml:"max_output_retries"`
	MaxReasoningSeconds     *int `yaml:"max_reasoning_seconds"`
	MaxReasoningLoopRepeats *int `yaml:"max_reasoning_loop_repeats"`

	// Stage-specific tunables.
	NudgeCount                *int     `yaml:"nudge_count"`
	DisableReasoningExtract   *bool    `yaml:"disable_reasoning_extract"`
	DisableParallelToolCalls  *bool    `yaml:"disable_parallel_tool_calls"`
	DisablePatchSummary       *bool    `yaml:"disable_patch_summary"`
	SkipSuggestions           *bool    `yaml:"disable_suggestions"`
	DisableJSONResponseFormat *bool    `yaml:"disable_json_response_format"`
	VerifyDropPolicy          *string  `yaml:"verify_drop_policy"`
	ConfidenceThreshold       *float64 `yaml:"confidence_threshold"`
	PriorityThreshold         *string  `yaml:"priority_threshold"`

	// Review-only internal agent overrides. These keys are accepted only under
	// config on review:<vector> steps. Each inherits the already-resolved review
	// step config, then applies its own agent-level overrides.
	MineReasoning   *AgentOverride `yaml:"mine_reasoning"`
	CompileFindings *AgentOverride `yaml:"compile_findings"`
	Nudge           *AgentOverride `yaml:"nudge"`
}

var stepOverrideKeys = []string{
	"model", "temperature", "top_p", "top_k", "presence_penalty", "max_tokens", "extra_body", "reasoning_effort", "time_budget",
	"scope",
	"max_tool_calls", "max_duplicate_tool_calls",
	"max_output_retries", "max_reasoning_seconds", "max_reasoning_loop_repeats",
	"nudge_count", "disable_reasoning_extract", "disable_parallel_tool_calls",
	"disable_patch_summary", "disable_suggestions", "disable_json_response_format", "verify_drop_policy",
	"confidence_threshold", "priority_threshold",
}

var reviewInternalOverrideKeys = []string{"mine_reasoning", "compile_findings", "nudge"}

// AgentOverride is the subset of per-step config that can sensibly apply to an
// internal agent spawned by a review step.
type AgentOverride struct {
	Model           *string        `yaml:"model"`
	Temperature     *float64       `yaml:"temperature"`
	TopP            *float64       `yaml:"top_p"`
	TopK            *int           `yaml:"top_k"`
	PresencePenalty *float64       `yaml:"presence_penalty"`
	MaxTokens       *int           `yaml:"max_tokens"`
	ExtraBody       map[string]any `yaml:"extra_body"`
	ReasoningEffort *string        `yaml:"reasoning_effort"`
	TimeBudget      *TimeBudget    `yaml:"time_budget"`

	MaxToolCalls            *int `yaml:"max_tool_calls"`
	MaxDuplicateToolCalls   *int `yaml:"max_duplicate_tool_calls"`
	MaxOutputRetries        *int `yaml:"max_output_retries"`
	MaxReasoningSeconds     *int `yaml:"max_reasoning_seconds"`
	MaxReasoningLoopRepeats *int `yaml:"max_reasoning_loop_repeats"`

	DisableParallelToolCalls  *bool `yaml:"disable_parallel_tool_calls"`
	DisableJSONResponseFormat *bool `yaml:"disable_json_response_format"`
}

var agentOverrideKeys = []string{
	"model", "temperature", "top_p", "top_k", "presence_penalty", "max_tokens", "extra_body", "reasoning_effort", "time_budget",
	"max_tool_calls", "max_duplicate_tool_calls",
	"max_output_retries", "max_reasoning_seconds", "max_reasoning_loop_repeats",
	"disable_parallel_tool_calls", "disable_json_response_format",
}

// TimeBudget controls wall-clock budgeting for workflow steps and groups.
// max_seconds sets a local cap, speedup_threshold controls when urgent retries
// begin (50..100), and weight allocates a lazily-started duration share of a
// parent budget, still capped by the parent deadline.
type TimeBudget struct {
	MaxSeconds       *int `yaml:"max_seconds"`
	SpeedupThreshold *int `yaml:"speedup_threshold"`
	Weight           *int `yaml:"weight"`
}

var groupConfigKeys = []string{"time_budget"}

// Resolve layers the override onto the given base profile and request, returning
// the effective pair for the step. A nil override (or one with no fields set)
// returns the inputs unchanged — the default-inheritance contract.
func (o *StepOverride) Resolve(p config.Profile, req model.ReviewRequest) (config.Profile, model.ReviewRequest) {
	if o == nil {
		return p, req
	}
	if o.Model != nil {
		p = resolveModelAlias(p, *o.Model)
	}
	if o.Temperature != nil {
		v := *o.Temperature
		p.Temperature = &v
	}
	if o.TopP != nil {
		v := *o.TopP
		p.TopP = &v
	}
	if o.TopK != nil {
		v := *o.TopK
		p.TopK = &v
	}
	if o.PresencePenalty != nil {
		v := *o.PresencePenalty
		p.PresencePenalty = &v
	}
	if o.MaxTokens != nil {
		v := *o.MaxTokens
		p.MaxTokens = &v
	}
	if o.ExtraBody != nil {
		p.ExtraBody = o.ExtraBody
	}
	if o.ReasoningEffort != nil {
		p.ReasoningEffort = *o.ReasoningEffort
	}
	if o.MaxToolCalls != nil {
		p.MaxToolCalls = *o.MaxToolCalls
		req.MaxToolCalls = *o.MaxToolCalls
	}
	if o.MaxDuplicateToolCalls != nil {
		p.MaxDuplicateToolCalls = *o.MaxDuplicateToolCalls
		req.MaxDuplicateToolCalls = *o.MaxDuplicateToolCalls
	}
	if o.MaxOutputRetries != nil {
		p.MaxOutputRetries = *o.MaxOutputRetries
		req.MaxOutputRetries = *o.MaxOutputRetries
	}
	if o.MaxReasoningSeconds != nil {
		p.MaxReasoningSeconds = *o.MaxReasoningSeconds
		req.MaxReasoningSeconds = *o.MaxReasoningSeconds
	}
	if o.MaxReasoningLoopRepeats != nil {
		p.MaxReasoningLoopRepeats = *o.MaxReasoningLoopRepeats
		req.MaxReasoningLoopRepeats = *o.MaxReasoningLoopRepeats
	}
	if o.NudgeCount != nil {
		p.NudgeCount = *o.NudgeCount
		req.NudgeCount = *o.NudgeCount
	}
	if o.DisableJSONResponseFormat != nil {
		p.DisableJSONResponseFormat = *o.DisableJSONResponseFormat
		req.DisableJSONResponseFormat = *o.DisableJSONResponseFormat
	}
	if o.DisableReasoningExtract != nil {
		req.DisableReasoningExtract = *o.DisableReasoningExtract
	}
	if o.DisableParallelToolCalls != nil {
		req.DisableParallelToolCalls = *o.DisableParallelToolCalls
	}
	if o.DisablePatchSummary != nil {
		req.DisablePatchSummary = *o.DisablePatchSummary
	}
	if o.SkipSuggestions != nil {
		req.SkipSuggestions = *o.SkipSuggestions
	}
	if o.VerifyDropPolicy != nil {
		req.VerifyDropPolicy = *o.VerifyDropPolicy
	}
	if o.ConfidenceThreshold != nil {
		req.ConfidenceThreshold = *o.ConfidenceThreshold
	}
	if o.PriorityThreshold != nil {
		req.PriorityThreshold = *o.PriorityThreshold
	}
	return p, req
}

// Resolve layers the internal-agent override onto a review step's already
// resolved profile/request pair.
func (o *AgentOverride) Resolve(p config.Profile, req model.ReviewRequest) (config.Profile, model.ReviewRequest) {
	if o == nil {
		return p, req
	}
	if o.Model != nil {
		p = resolveModelAlias(p, *o.Model)
	}
	if o.Temperature != nil {
		v := *o.Temperature
		p.Temperature = &v
	}
	if o.TopP != nil {
		v := *o.TopP
		p.TopP = &v
	}
	if o.TopK != nil {
		v := *o.TopK
		p.TopK = &v
	}
	if o.PresencePenalty != nil {
		v := *o.PresencePenalty
		p.PresencePenalty = &v
	}
	if o.MaxTokens != nil {
		v := *o.MaxTokens
		p.MaxTokens = &v
	}
	if o.ExtraBody != nil {
		p.ExtraBody = o.ExtraBody
	}
	if o.ReasoningEffort != nil {
		p.ReasoningEffort = *o.ReasoningEffort
	}
	if o.MaxToolCalls != nil {
		p.MaxToolCalls = *o.MaxToolCalls
		req.MaxToolCalls = *o.MaxToolCalls
	}
	if o.MaxDuplicateToolCalls != nil {
		p.MaxDuplicateToolCalls = *o.MaxDuplicateToolCalls
		req.MaxDuplicateToolCalls = *o.MaxDuplicateToolCalls
	}
	if o.MaxOutputRetries != nil {
		p.MaxOutputRetries = *o.MaxOutputRetries
		req.MaxOutputRetries = *o.MaxOutputRetries
	}
	if o.MaxReasoningSeconds != nil {
		p.MaxReasoningSeconds = *o.MaxReasoningSeconds
		req.MaxReasoningSeconds = *o.MaxReasoningSeconds
	}
	if o.MaxReasoningLoopRepeats != nil {
		p.MaxReasoningLoopRepeats = *o.MaxReasoningLoopRepeats
		req.MaxReasoningLoopRepeats = *o.MaxReasoningLoopRepeats
	}
	if o.DisableJSONResponseFormat != nil {
		p.DisableJSONResponseFormat = *o.DisableJSONResponseFormat
		req.DisableJSONResponseFormat = *o.DisableJSONResponseFormat
	}
	if o.DisableParallelToolCalls != nil {
		req.DisableParallelToolCalls = *o.DisableParallelToolCalls
	}
	return p, req
}

func resolveModelAlias(p config.Profile, model string) config.Profile {
	if strings.TrimSpace(model) == SmallModelAlias {
		return config.EffectiveSmallProfile(p)
	}
	p.Model = model
	return p
}

// DefaultSpec is the embedded workflow that reproduces the tool's standard
// review end to end: collect context, run the six vector reviewers concurrently,
// verify, dedupe, merge, finalize, verdict, summarize. Most steps inherit the
// active profile/request; final polish steps may carry cheap-model overrides. This is
// the single spec the engine runs for an ordinary (no --spec/--step) review,
// and the canonical full workflow shown to users.
//
// It is parsed from the embedded workflows/default.yaml through the same loader
// as any user-supplied spec, so the default is a real spec artifact rather than
// bespoke Go. The asset is static and test-covered (see TestDefaultSpecMatchesConstants),
// so a parse failure is a build error, not a runtime condition — hence the panic.
func DefaultSpec() Spec {
	spec, err := parseSpec(workflows.Default())
	if err != nil {
		panic(fmt.Sprintf("workflow: invalid embedded default workflow: %v", err))
	}
	return spec
}

// SingleStepSpec builds a one-step spec for `--step <type>`. findingsFrom seeds
// the step's injection inputs (e.g. --findings files for a standalone merge).
func SingleStepSpec(stepType string, findingsFrom []string) Spec {
	return Spec{
		Version: SpecVersion,
		Steps:   []StepEntry{{Type: stepType, FindingsFrom: findingsFrom}},
	}
}

// Load reads and parses a spec file, rejecting unknown keys.
func Load(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("workflow: reading %s: %w", path, err)
	}
	spec, err := parseSpec(data)
	if err != nil {
		return Spec{}, fmt.Errorf("workflow: parsing %s: %w", path, err)
	}
	return spec, nil
}

// parseSpec decodes spec YAML bytes, rejecting unknown keys. It is the single
// parse path shared by Load (disk specs) and DefaultSpec (the embedded default).
func parseSpec(data []byte) (Spec, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return Spec{}, err
	}
	if len(root.Content) == 0 {
		return Spec{}, fmt.Errorf("empty document")
	}
	return decodeSpec(root.Content[0])
}

func decodeSpec(node *yaml.Node) (Spec, error) {
	if node.Kind != yaml.MappingNode {
		return Spec{}, fmt.Errorf("spec must be a mapping")
	}
	if err := checkAllowedKeys(node, "version", "profile", "steps"); err != nil {
		return Spec{}, err
	}
	var spec Spec
	if v := mappingValue(node, "version"); v != nil {
		if err := v.Decode(&spec.Version); err != nil {
			return Spec{}, fmt.Errorf("version: %w", err)
		}
	}
	if v := mappingValue(node, "profile"); v != nil {
		if err := v.Decode(&spec.Profile); err != nil {
			return Spec{}, fmt.Errorf("profile: %w", err)
		}
	}
	stepsNode := mappingValue(node, "steps")
	if stepsNode == nil {
		return Spec{}, fmt.Errorf("steps is required")
	}
	if stepsNode.Kind != yaml.SequenceNode {
		return Spec{}, fmt.Errorf("steps must be a list")
	}
	for i, child := range stepsNode.Content {
		entry, err := decodeStepEntry(child)
		if err != nil {
			return Spec{}, fmt.Errorf("step %d: %w", i+1, err)
		}
		spec.Steps = append(spec.Steps, entry)
	}
	return spec, nil
}

func decodeStepEntry(node *yaml.Node) (StepEntry, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return StepEntry{}, fmt.Errorf("plain steps must use mapping form: type: %s", node.Value)
	case yaml.MappingNode:
		if mappingValue(node, "lane") != nil {
			return StepEntry{}, fmt.Errorf("lane is only allowed inside a parallel group")
		}
		if mappingValue(node, "pipeline") != nil {
			return decodePipelineEntry(node)
		}
		if mappingValue(node, "parallel") != nil {
			return decodeParallelEntry(node)
		}
		return decodePlainStep(node)
	default:
		return StepEntry{}, fmt.Errorf("must be a step mapping")
	}
}

func decodeParallelEntry(node *yaml.Node) (StepEntry, error) {
	if err := checkAllowedKeys(node, "parallel"); err != nil {
		return StepEntry{}, err
	}
	seq := mappingValue(node, "parallel")
	if seq.Kind != yaml.SequenceNode {
		return StepEntry{}, fmt.Errorf("parallel must be a list")
	}
	if len(seq.Content) == 0 {
		return StepEntry{}, fmt.Errorf("parallel group is empty")
	}
	entry := StepEntry{}
	for i, child := range seq.Content {
		sub, err := decodeParallelChild(child)
		if err != nil {
			return StepEntry{}, fmt.Errorf("parallel child %d: %w", i+1, err)
		}
		entry.Parallel = append(entry.Parallel, sub)
	}
	return entry, nil
}

// decodeParallelChild decodes one parallel-group child: a plain step (scalar or
// type-mapping) or a lane (sequential chain of plain steps).
func decodeParallelChild(node *yaml.Node) (StepEntry, error) {
	if node.Kind == yaml.MappingNode {
		if mappingValue(node, "parallel") != nil {
			return StepEntry{}, fmt.Errorf("parallel groups cannot be nested")
		}
		if mappingValue(node, "pipeline") != nil {
			return StepEntry{}, fmt.Errorf("pipeline groups cannot appear inside a parallel group")
		}
		if mappingValue(node, "lane") != nil {
			return decodeLaneEntry(node)
		}
	}
	return decodeStepEntry(node)
}

func decodeLaneEntry(node *yaml.Node) (StepEntry, error) {
	if err := checkAllowedKeys(node, "lane", "config"); err != nil {
		return StepEntry{}, err
	}
	seq := mappingValue(node, "lane")
	if seq.Kind != yaml.SequenceNode || len(seq.Content) == 0 {
		return StepEntry{}, fmt.Errorf("lane must be a non-empty list")
	}
	entry := StepEntry{}
	for i, child := range seq.Content {
		if child.Kind == yaml.MappingNode {
			if mappingValue(child, "parallel") != nil {
				return StepEntry{}, fmt.Errorf("lane step %d: parallel groups cannot be nested", i+1)
			}
			if mappingValue(child, "lane") != nil {
				return StepEntry{}, fmt.Errorf("lane step %d: lanes cannot be nested", i+1)
			}
			if mappingValue(child, "pipeline") != nil {
				return StepEntry{}, fmt.Errorf("lane step %d: pipelines cannot be nested", i+1)
			}
		}
		sub, err := decodeStepEntry(child)
		if err != nil {
			return StepEntry{}, fmt.Errorf("lane step %d: %w", i+1, err)
		}
		entry.Lane = append(entry.Lane, sub)
	}
	if cfg := mappingValue(node, "config"); cfg != nil {
		override, err := decodeGroupConfig(cfg)
		if err != nil {
			return StepEntry{}, fmt.Errorf("config: %w", err)
		}
		entry.Config = override
	}
	return entry, nil
}

// decodePipelineEntry decodes a pipeline group: a sequential chain of plain
// post-review steps (merge→finalize→verdict, optionally summarize) that the
// engine streams per cluster. Structural validity (allowed members, order,
// scopes) is checked in Spec.Validate.
func decodePipelineEntry(node *yaml.Node) (StepEntry, error) {
	if err := checkAllowedKeys(node, "pipeline", "config"); err != nil {
		return StepEntry{}, err
	}
	seq := mappingValue(node, "pipeline")
	if seq.Kind != yaml.SequenceNode || len(seq.Content) == 0 {
		return StepEntry{}, fmt.Errorf("pipeline must be a non-empty list")
	}
	entry := StepEntry{}
	for i, child := range seq.Content {
		if child.Kind == yaml.MappingNode {
			if mappingValue(child, "parallel") != nil {
				return StepEntry{}, fmt.Errorf("pipeline step %d: parallel groups cannot be nested", i+1)
			}
			if mappingValue(child, "lane") != nil {
				return StepEntry{}, fmt.Errorf("pipeline step %d: lanes cannot be nested", i+1)
			}
			if mappingValue(child, "pipeline") != nil {
				return StepEntry{}, fmt.Errorf("pipeline step %d: pipelines cannot be nested", i+1)
			}
		}
		sub, err := decodeStepEntry(child)
		if err != nil {
			return StepEntry{}, fmt.Errorf("pipeline step %d: %w", i+1, err)
		}
		entry.Pipeline = append(entry.Pipeline, sub)
	}
	if cfg := mappingValue(node, "config"); cfg != nil {
		override, err := decodeGroupConfig(cfg)
		if err != nil {
			return StepEntry{}, fmt.Errorf("config: %w", err)
		}
		entry.Config = override
	}
	return entry, nil
}

func decodeGroupConfig(node *yaml.Node) (*StepOverride, error) {
	if err := checkAllowedKeys(node, groupConfigKeys...); err != nil {
		return nil, err
	}
	var override StepOverride
	if err := node.Decode(&override); err != nil {
		return nil, err
	}
	return &override, nil
}

func decodePlainStep(node *yaml.Node) (StepEntry, error) {
	if err := checkAllowedKeys(node, "type", "config", "findings_from"); err != nil {
		return StepEntry{}, err
	}
	typeNode := mappingValue(node, "type")
	if typeNode == nil {
		return StepEntry{}, fmt.Errorf("missing type")
	}
	entry := StepEntry{Type: typeNode.Value}
	if ff := mappingValue(node, "findings_from"); ff != nil {
		paths, err := decodeStringOrList(ff)
		if err != nil {
			return StepEntry{}, fmt.Errorf("findings_from: %w", err)
		}
		entry.FindingsFrom = paths
	}
	if cfg := mappingValue(node, "config"); cfg != nil {
		if err := checkAllowedKeys(cfg, allowedStepOverrideKeys(entry.Type)...); err != nil {
			return StepEntry{}, fmt.Errorf("config: %w", err)
		}
		if err := checkReviewInternalOverrides(entry.Type, cfg); err != nil {
			return StepEntry{}, fmt.Errorf("config: %w", err)
		}
		var override StepOverride
		if err := cfg.Decode(&override); err != nil {
			return StepEntry{}, fmt.Errorf("config: %w", err)
		}
		entry.Config = &override
	}
	return entry, nil
}

func allowedStepOverrideKeys(stepType string) []string {
	allowed := slices.Clone(stepOverrideKeys)
	if strings.HasPrefix(stepType, StepReviewPrefix) {
		allowed = append(allowed, reviewInternalOverrideKeys...)
	}
	return allowed
}

func checkReviewInternalOverrides(stepType string, cfg *yaml.Node) error {
	if !strings.HasPrefix(stepType, StepReviewPrefix) {
		return nil
	}
	for _, key := range reviewInternalOverrideKeys {
		node := mappingValue(cfg, key)
		if node == nil {
			continue
		}
		if node.Kind != yaml.MappingNode {
			return fmt.Errorf("%s: must be a mapping", key)
		}
		if err := checkAllowedKeys(node, agentOverrideKeys...); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}

func decodeStringOrList(node *yaml.Node) ([]string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return []string{node.Value}, nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(node.Content))
		for _, child := range node.Content {
			if child.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("must be a string or list of strings")
			}
			out = append(out, child.Value)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("must be a string or list of strings")
	}
}

// Validate performs structural validation that does not depend on runtime inputs
// (source availability, injected findings). Runtime feasibility (e.g. a step
// that needs grouped input receiving none) is checked by the executor.
func (s Spec) Validate() error {
	if s.Version != SpecVersion {
		return fmt.Errorf("workflow: unsupported spec version %d (supported: %d)", s.Version, SpecVersion)
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("workflow: steps is empty")
	}
	reviewed := map[string]bool{}
	idx := 0
	// validate checks one plain step's type and dependencies, returning the
	// vector it reviews (if any). Per-vector follow-up steps (verify, dedupe,
	// nudge, reasoning-extract) depend on a review of the same vector, which
	// must have completed either in an EARLIER unit or earlier in the SAME lane
	// — concurrent lanes' reviews are not yet available.
	validate := func(entry StepEntry, laneReviewed map[string]bool, inParallel bool) (string, error) {
		idx++
		if entry.IsParallel() || entry.IsLane() || entry.IsPipeline() {
			return "", fmt.Errorf("workflow: step %d: parallel groups, lanes, and pipelines cannot be nested", idx)
		}
		if err := validateStepType(entry.Type); err != nil {
			return "", fmt.Errorf("workflow: step %d: %w", idx, err)
		}
		if err := validateScope(entry.Type, entry.Config); err != nil {
			return "", fmt.Errorf("workflow: step %d: %w", idx, err)
		}
		if err := validateStepTimeBudgets(entry); err != nil {
			return "", fmt.Errorf("workflow: step %d: %w", idx, err)
		}
		// cluster-scoped finalize/summarize is only reachable inside a pipeline
		// group (the only place the engine shards them). A flat step must be
		// scope: all (or unset, which defaults to all).
		if (entry.Type == StepFinalize || entry.Type == StepSummarize) &&
			entry.Config != nil && entry.Config.Scope != nil && *entry.Config.Scope == ScopeCluster {
			return "", fmt.Errorf("workflow: step %d: scope %q for %q is only valid inside a pipeline: group", idx, ScopeCluster, entry.Type)
		}
		// Only per-vector steps are safe to run concurrently — each touches only
		// its own vector's group/session. Every global step (collect-context,
		// verify, dedupe, merge, finalize, verdict, summarize) mutates shared
		// pipeline state (the enriched context, the flat result, or all groups)
		// and must run sequentially.
		if inParallel && !isPerVectorStep(entry.Type) {
			return "", fmt.Errorf("workflow: step %d: %q cannot run inside a parallel group; only per-vector steps (review:/verify:/dedupe:/nudge:/reasoning-extract:) may run concurrently", idx, entry.Type)
		}
		for _, prefix := range []string{StepExtractPrefix, StepNudgePrefix, StepVerifyPrefix, StepDedupePrefix} {
			if v, ok := vectorOf(entry.Type, prefix); ok && !reviewed[v] && !laneReviewed[v] {
				return "", fmt.Errorf("workflow: step %d: %q requires a preceding %s%s step (in an earlier step or earlier in the same lane)", idx, entry.Type, StepReviewPrefix, v)
			}
		}
		if v, ok := vectorOf(entry.Type, StepReviewPrefix); ok {
			return v, nil
		}
		return "", nil
	}
	for _, entry := range s.Steps {
		if entry.IsLane() {
			return fmt.Errorf("workflow: lane is only allowed inside a parallel group")
		}
		if entry.IsPipeline() {
			if err := validateGroupTimeBudget("pipeline", entry.Config); err != nil {
				return err
			}
			if err := validateChildWeights("pipeline", entry.Pipeline); err != nil {
				return err
			}
			if err := validatePipelineGroup(entry, &idx); err != nil {
				return err
			}
			continue
		}
		if entry.IsParallel() {
			// Validate every lane against the pre-group reviewer set plus its
			// own earlier steps; publish the group's reviewers only after the
			// whole group. Each vector may be touched by at most one lane, so
			// concurrent lanes cannot race on a group or session.
			var produced []string
			vectorOwner := map[string]int{}
			for laneIdx, sub := range entry.Parallel {
				if sub.IsLane() {
					if err := validateGroupTimeBudget("lane", sub.Config); err != nil {
						return err
					}
					if err := validateChildWeights("lane", sub.Lane); err != nil {
						return err
					}
				}
				laneReviewed := map[string]bool{}
				for _, ls := range sub.LaneSteps() {
					v, err := validate(ls, laneReviewed, true)
					if err != nil {
						return err
					}
					if v != "" {
						laneReviewed[v] = true
						produced = append(produced, v)
					}
					if vec, ok := stepVectorAny(ls.Type); ok {
						if owner, claimed := vectorOwner[vec]; claimed && owner != laneIdx {
							return fmt.Errorf("workflow: vector %q is used by more than one lane in the same parallel group", vec)
						}
						vectorOwner[vec] = laneIdx
					}
				}
			}
			for _, v := range produced {
				reviewed[v] = true
			}
			continue
		}
		v, err := validate(entry, nil, false)
		if err != nil {
			return err
		}
		if v != "" {
			reviewed[v] = true
		}
	}
	return nil
}

func validateStepTimeBudgets(entry StepEntry) error {
	if entry.Config != nil {
		if err := validateTimeBudget(entry.Config.TimeBudget); err != nil {
			return fmt.Errorf("time_budget: %w", err)
		}
	}
	if strings.HasPrefix(entry.Type, StepReviewPrefix) && entry.Config != nil {
		phaseBudgets := []*TimeBudget{nil, nil, nil, nil}
		if entry.Config.TimeBudget != nil {
			phaseBudgets[0] = entry.Config.TimeBudget
		}
		if entry.Config.MineReasoning != nil {
			if err := validateTimeBudget(entry.Config.MineReasoning.TimeBudget); err != nil {
				return fmt.Errorf("mine_reasoning.time_budget: %w", err)
			}
			phaseBudgets[1] = entry.Config.MineReasoning.TimeBudget
		}
		if entry.Config.CompileFindings != nil {
			if err := validateTimeBudget(entry.Config.CompileFindings.TimeBudget); err != nil {
				return fmt.Errorf("compile_findings.time_budget: %w", err)
			}
			phaseBudgets[2] = entry.Config.CompileFindings.TimeBudget
		}
		if entry.Config.Nudge != nil {
			if err := validateTimeBudget(entry.Config.Nudge.TimeBudget); err != nil {
				return fmt.Errorf("nudge.time_budget: %w", err)
			}
			phaseBudgets[3] = entry.Config.Nudge.TimeBudget
		}
		if err := validateWeightSet("review phases", phaseBudgets); err != nil {
			return err
		}
	}
	return nil
}

func validateGroupTimeBudget(kind string, cfg *StepOverride) error {
	if cfg == nil {
		return nil
	}
	if err := validateTimeBudget(cfg.TimeBudget); err != nil {
		return fmt.Errorf("workflow: %s config time_budget: %w", kind, err)
	}
	return nil
}

func validateChildWeights(kind string, children []StepEntry) error {
	budgets := make([]*TimeBudget, 0, len(children))
	for i := range children {
		if children[i].Config == nil {
			budgets = append(budgets, nil)
			continue
		}
		budgets = append(budgets, children[i].Config.TimeBudget)
	}
	if err := validateWeightSet(kind+" children", budgets); err != nil {
		return fmt.Errorf("workflow: %w", err)
	}
	return nil
}

func validateTimeBudget(tb *TimeBudget) error {
	if tb == nil {
		return nil
	}
	if tb.MaxSeconds != nil && *tb.MaxSeconds <= 0 {
		return fmt.Errorf("max_seconds must be positive")
	}
	if tb.SpeedupThreshold != nil && (*tb.SpeedupThreshold < 50 || *tb.SpeedupThreshold > 100) {
		return fmt.Errorf("speedup_threshold must be between 50 and 100")
	}
	if tb.Weight != nil && (*tb.Weight < 0 || *tb.Weight > 100) {
		return fmt.Errorf("weight must be between 0 and 100")
	}
	return nil
}

func validateWeightSet(label string, budgets []*TimeBudget) error {
	explicit := 0
	sum := 0
	for _, budget := range budgets {
		if budget == nil || budget.Weight == nil {
			continue
		}
		explicit++
		sum += *budget.Weight
	}
	if explicit == 0 {
		return nil
	}
	if sum > 100 {
		return fmt.Errorf("%s time_budget weights sum to %d, must be <= 100", label, sum)
	}
	if explicit == len(budgets) && sum != 100 {
		return fmt.Errorf("%s time_budget weights sum to %d, must be 100 when all weights are set", label, sum)
	}
	return nil
}

// pipelineMember is one required step of a pipeline group: its type and the one
// scope value it must carry (when scope is set).
type pipelineMember struct {
	typ   string
	scope string
}

// validatePipelineGroup validates a pipeline group — the streamed post-review
// tail. The only shape the engine implements is merge → finalize → verdict,
// optionally followed by summarize, with merge/finalize/summarize at
// scope: cluster and verdict at scope: all. findings_from is accepted on the
// merge step only. idx is advanced so step numbering in later errors stays
// global and stable.
func validatePipelineGroup(entry StepEntry, idx *int) error {
	want := []pipelineMember{
		{StepMerge, ScopeCluster},
		{StepFinalize, ScopeCluster},
		{StepVerdict, ScopeAll},
	}
	n := len(entry.Pipeline)
	if n == 4 {
		want = append(want, pipelineMember{StepSummarize, ScopeCluster})
	} else if n != 3 {
		return fmt.Errorf("workflow: pipeline must be merge → finalize → verdict, optionally followed by summarize")
	}
	for i, child := range entry.Pipeline {
		*idx++
		if child.IsParallel() || child.IsLane() || child.IsPipeline() {
			return fmt.Errorf("workflow: step %d: parallel groups, lanes, and pipelines cannot be nested in a pipeline", *idx)
		}
		exp := want[i]
		if child.Type != exp.typ {
			return fmt.Errorf("workflow: step %d: pipeline step %d must be %q, got %q", *idx, i+1, exp.typ, child.Type)
		}
		if err := validateScope(child.Type, child.Config); err != nil {
			return fmt.Errorf("workflow: step %d: %w", *idx, err)
		}
		if err := validateStepTimeBudgets(child); err != nil {
			return fmt.Errorf("workflow: step %d: %w", *idx, err)
		}
		if child.Config != nil && child.Config.Scope != nil && *child.Config.Scope != exp.scope {
			return fmt.Errorf("workflow: step %d: %q in a pipeline must have scope %q", *idx, child.Type, exp.scope)
		}
		if child.Type != StepMerge && len(child.FindingsFrom) > 0 {
			return fmt.Errorf("workflow: step %d: findings_from is only allowed on the merge step of a pipeline", *idx)
		}
	}
	return nil
}

// stepVectorAny returns the vector id when t is a per-vector step of any kind.
func stepVectorAny(t string) (string, bool) {
	for _, prefix := range perVectorPrefixes {
		if v, ok := vectorOf(t, prefix); ok {
			return v, true
		}
	}
	return "", false
}

// isPerVectorStep reports whether t addresses a single reviewer vector.
func isPerVectorStep(t string) bool {
	_, ok := stepVectorAny(t)
	return ok
}

func validateStepType(t string) error {
	switch t {
	case StepCollectContext, StepVerify, StepDedupe, StepMerge, StepFinalize, StepVerdict, StepSummarize:
		return nil
	case "":
		return fmt.Errorf("missing step type")
	}
	for _, prefix := range perVectorPrefixes {
		if v, ok := vectorOf(t, prefix); ok {
			if !validVector(v) {
				return fmt.Errorf("unknown reviewer vector %q (valid: %s)", v, strings.Join(ReviewVectorIDs, ", "))
			}
			return nil
		}
	}
	return fmt.Errorf("unknown step type %q", t)
}

// StepNeedsSource reports whether a step type requires a review source. Only
// context collection and the reviewers read the source; the post-reviewer steps
// operate on in-memory or injected findings.
func StepNeedsSource(stepType string) bool {
	if stepType == StepCollectContext {
		return true
	}
	_, ok := vectorOf(stepType, StepReviewPrefix)
	return ok
}

// NeedsSource reports whether any step in the spec requires a review source.
// When false, the workflow operates purely on injected findings and the caller
// can skip source/repo resolution and the upfront model-credential requirement.
func (s Spec) NeedsSource() bool {
	for _, entry := range s.FlatSteps() {
		if StepNeedsSource(entry.Type) {
			return true
		}
	}
	return false
}

// FlatSteps returns pointers to every plain step entry in document order:
// top-level steps, parallel children, and lane steps. Pointer access lets
// callers mutate entries in place (e.g. findings seeding).
func (s *Spec) FlatSteps() []*StepEntry {
	var out []*StepEntry
	var walk func(entry *StepEntry)
	walk = func(entry *StepEntry) {
		switch {
		case entry.IsParallel():
			for i := range entry.Parallel {
				walk(&entry.Parallel[i])
			}
		case entry.IsLane():
			for i := range entry.Lane {
				walk(&entry.Lane[i])
			}
		case entry.IsPipeline():
			for i := range entry.Pipeline {
				walk(&entry.Pipeline[i])
			}
		default:
			out = append(out, entry)
		}
	}
	for i := range s.Steps {
		walk(&s.Steps[i])
	}
	return out
}

// StepConsumesFindings reports whether a step type reads injected findings
// (findings_from / --findings). Only these steps load findings; collect-context
// and review/nudge/reasoning-extract steps ignore them.
func StepConsumesFindings(stepType string) bool {
	switch stepType {
	case StepVerify, StepDedupe, StepMerge, StepFinalize, StepVerdict, StepSummarize:
		return true
	default:
		return false
	}
}

// vectorOf returns the vector id when t has the given prefix.
func vectorOf(t, prefix string) (string, bool) {
	return strings.CutPrefix(t, prefix)
}

func validVector(id string) bool {
	return slices.Contains(ReviewVectorIDs, id)
}

func checkAllowedKeys(node *yaml.Node, allowed ...string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected a mapping")
	}
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if _, ok := set[key]; !ok {
			return fmt.Errorf("unknown key %q (allowed: %s)", key, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
