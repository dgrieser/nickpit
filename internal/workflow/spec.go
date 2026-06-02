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
)

// SpecVersion is the only supported major spec version.
const SpecVersion = 1

// Step type identifiers. Steps that operate on a single reviewer vector are
// addressed with a "<prefix><vector-id>" type, e.g. "review:security".
const (
	StepCollectContext = "collect-context"
	StepVerify         = "verify"
	StepDedupe         = "dedupe"
	StepMerge          = "merge"
	StepFinalize       = "finalize"
	StepSummarize      = "summarize"

	StepReviewPrefix  = "review:"
	StepExtractPrefix = "reasoning-extract:"
	StepNudgePrefix   = "nudge:"
)

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
//   - a parallel group (Parallel set) whose children run concurrently.
type StepEntry struct {
	Type         string
	Config       *StepOverride
	FindingsFrom []string
	Parallel     []StepEntry
}

// IsParallel reports whether the entry is a parallel group.
func (s StepEntry) IsParallel() bool { return len(s.Parallel) > 0 }

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
	MaxTokens       *int           `yaml:"max_tokens"`
	ExtraBody       map[string]any `yaml:"extra_body"`
	ReasoningEffort *string        `yaml:"reasoning_effort"`

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
	NudgeCount               *int     `yaml:"nudge_count"`
	DisableReasoningExtract  *bool    `yaml:"disable_reasoning_extract"`
	DisableParallelToolCalls *bool    `yaml:"disable_parallel_tool_calls"`
	UseJSONSchema            *bool    `yaml:"use_json_schema"`
	VerifyConcurrency        *int     `yaml:"verify_concurrency"`
	VerifyDropPolicy         *string  `yaml:"verify_drop_policy"`
	VerifyDropConfidence     *float64 `yaml:"verify_drop_confidence"`
	PriorityThreshold        *string  `yaml:"priority_threshold"`
}

var stepOverrideKeys = []string{
	"model", "temperature", "top_p", "max_tokens", "extra_body", "reasoning_effort",
	"max_tool_calls", "max_duplicate_tool_calls",
	"max_output_retries", "max_reasoning_seconds", "max_reasoning_loop_repeats",
	"nudge_count", "disable_reasoning_extract", "disable_parallel_tool_calls",
	"use_json_schema", "verify_concurrency", "verify_drop_policy",
	"verify_drop_confidence", "priority_threshold",
}

// Resolve layers the override onto the given base profile and request, returning
// the effective pair for the step. A nil override (or one with no fields set)
// returns the inputs unchanged — the default-inheritance contract.
func (o *StepOverride) Resolve(p config.Profile, req model.ReviewRequest) (config.Profile, model.ReviewRequest) {
	if o == nil {
		return p, req
	}
	if o.Model != nil {
		p.Model = *o.Model
	}
	if o.Temperature != nil {
		v := *o.Temperature
		p.Temperature = &v
	}
	if o.TopP != nil {
		v := *o.TopP
		p.TopP = &v
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
	if o.UseJSONSchema != nil {
		p.UseJSONSchema = *o.UseJSONSchema
		req.UseJSONSchema = *o.UseJSONSchema
	}
	if o.DisableReasoningExtract != nil {
		req.DisableReasoningExtract = *o.DisableReasoningExtract
	}
	if o.DisableParallelToolCalls != nil {
		req.DisableParallelToolCalls = *o.DisableParallelToolCalls
	}
	if o.VerifyConcurrency != nil {
		req.VerifyConcurrency = *o.VerifyConcurrency
	}
	if o.VerifyDropPolicy != nil {
		req.VerifyDropPolicy = *o.VerifyDropPolicy
	}
	if o.VerifyDropConfidence != nil {
		req.VerifyDropConfidence = *o.VerifyDropConfidence
	}
	if o.PriorityThreshold != nil {
		req.PriorityThreshold = *o.PriorityThreshold
	}
	return p, req
}

// DefaultSpec is the embedded workflow that reproduces the tool's standard
// review end to end: collect context, run the six vector reviewers concurrently,
// verify, dedupe, merge, finalize, summarize. It carries no overrides, so every
// step inherits the active profile/request. This is the canonical spec shown to
// users and run by `--spec` with the full workflow.
func DefaultSpec() Spec {
	spec := DefaultReviewSpec()
	spec.Steps = append(spec.Steps, StepEntry{Type: StepFinalize}, StepEntry{Type: StepSummarize})
	return spec
}

// DefaultReviewSpec is DefaultSpec without the finalize step. It is what the
// no-spec review path executes; finalize is applied by the caller afterward so
// the engine's RunWithContext contract (returns the pre-finalize result) is
// preserved. The two compose into the same end result as DefaultSpec.
func DefaultReviewSpec() Spec {
	parallel := make([]StepEntry, len(ReviewVectorIDs))
	for i, id := range ReviewVectorIDs {
		parallel[i] = StepEntry{Type: StepReviewPrefix + id}
	}
	return Spec{
		Version: SpecVersion,
		Steps: []StepEntry{
			{Type: StepCollectContext},
			{Parallel: parallel},
			{Type: StepVerify},
			{Type: StepDedupe},
			{Type: StepMerge},
		},
	}
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
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return Spec{}, fmt.Errorf("workflow: parsing %s: %w", path, err)
	}
	if len(root.Content) == 0 {
		return Spec{}, fmt.Errorf("workflow: %s is empty", path)
	}
	spec, err := decodeSpec(root.Content[0])
	if err != nil {
		return Spec{}, fmt.Errorf("workflow: parsing %s: %w", path, err)
	}
	return spec, nil
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
		return StepEntry{Type: node.Value}, nil
	case yaml.MappingNode:
		if mappingValue(node, "parallel") != nil {
			return decodeParallelEntry(node)
		}
		return decodePlainStep(node)
	default:
		return StepEntry{}, fmt.Errorf("must be a step name or mapping")
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
		sub, err := decodeStepEntry(child)
		if err != nil {
			return StepEntry{}, fmt.Errorf("parallel child %d: %w", i+1, err)
		}
		if sub.IsParallel() {
			return StepEntry{}, fmt.Errorf("parallel groups cannot be nested")
		}
		entry.Parallel = append(entry.Parallel, sub)
	}
	return entry, nil
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
		if err := checkAllowedKeys(cfg, stepOverrideKeys...); err != nil {
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
	// validate checks one step's type and dependencies against the set of
	// reviewers available so far, returning the vector it reviews (if any). A
	// nudge/extract step depends on a review of the same vector, which must have
	// completed in an EARLIER unit — reviews inside the same parallel group run
	// concurrently and are therefore not yet available.
	validate := func(entry StepEntry, available map[string]bool, inParallel bool) (string, error) {
		idx++
		if err := validateStepType(entry.Type); err != nil {
			return "", fmt.Errorf("workflow: step %d: %w", idx, err)
		}
		// Only the per-vector reviewers are safe to run concurrently — each owns
		// its own session. Every other step (collect-context, verify, dedupe,
		// merge, finalize, nudge, reasoning-extract) mutates shared pipeline
		// state (the enriched context, the flat result, or a shared session) and
		// must run sequentially.
		if inParallel && !strings.HasPrefix(entry.Type, StepReviewPrefix) {
			return "", fmt.Errorf("workflow: step %d: %q cannot run inside a parallel group; only review:<vector> steps may run concurrently", idx, entry.Type)
		}
		for _, prefix := range []string{StepExtractPrefix, StepNudgePrefix} {
			if v, ok := vectorOf(entry.Type, prefix); ok && !available[v] {
				return "", fmt.Errorf("workflow: step %d: %q requires a preceding %s%s step (in an earlier step, not the same parallel group)", idx, entry.Type, StepReviewPrefix, v)
			}
		}
		if v, ok := vectorOf(entry.Type, StepReviewPrefix); ok {
			return v, nil
		}
		return "", nil
	}
	for _, entry := range s.Steps {
		if entry.IsParallel() {
			// Validate every child against the pre-group reviewer set, then
			// publish the group's reviewers only after the whole group.
			var produced []string
			for _, sub := range entry.Parallel {
				v, err := validate(sub, reviewed, true)
				if err != nil {
					return err
				}
				if v != "" {
					produced = append(produced, v)
				}
			}
			for _, v := range produced {
				reviewed[v] = true
			}
			continue
		}
		v, err := validate(entry, reviewed, false)
		if err != nil {
			return err
		}
		if v != "" {
			reviewed[v] = true
		}
	}
	return nil
}

func validateStepType(t string) error {
	switch t {
	case StepCollectContext, StepVerify, StepDedupe, StepMerge, StepFinalize, StepSummarize:
		return nil
	case "":
		return fmt.Errorf("missing step type")
	}
	for _, prefix := range []string{StepReviewPrefix, StepExtractPrefix, StepNudgePrefix} {
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
	for _, entry := range s.Steps {
		if entry.IsParallel() {
			for _, sub := range entry.Parallel {
				if StepNeedsSource(sub.Type) {
					return true
				}
			}
			continue
		}
		if StepNeedsSource(entry.Type) {
			return true
		}
	}
	return false
}

// StepConsumesFindings reports whether a step type reads injected findings
// (findings_from / --findings). Only these steps load findings; collect-context
// and review/nudge/reasoning-extract steps ignore them.
func StepConsumesFindings(stepType string) bool {
	switch stepType {
	case StepVerify, StepDedupe, StepMerge, StepFinalize, StepSummarize:
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
