package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
)

func writeSpec(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultSpecsValidate(t *testing.T) {
	if err := DefaultSpec().Validate(); err != nil {
		t.Fatalf("DefaultSpec invalid: %v", err)
	}
	full := DefaultSpec()
	last := full.Steps[len(full.Steps)-1]
	if !last.IsPipeline() {
		t.Fatalf("DefaultSpec last step is not a pipeline: %+v", last)
	}
	wantTail := []string{StepMerge, StepFinalize, StepVerdict, StepSummarize}
	if len(last.Pipeline) != len(wantTail) {
		t.Fatalf("DefaultSpec pipeline has %d steps, want %d", len(last.Pipeline), len(wantTail))
	}
	for i, w := range wantTail {
		if last.Pipeline[i].Type != w {
			t.Fatalf("DefaultSpec pipeline step %d = %q, want %q", i, last.Pipeline[i].Type, w)
		}
	}
}

// TestDefaultSpecMatchesConstants pins the embedded workflows/default.yaml to the
// Go constants. The old Go-built DefaultSpec auto-tracked ReviewVectorIDs; the
// static YAML does not, so reordering/renaming a vector (or changing the step
// sequence) must fail here and force a matching default.yaml edit.
func TestDefaultSpecMatchesConstants(t *testing.T) {
	small := SmallModelAlias
	cluster := ScopeCluster
	all := ScopeAll
	finding := ScopeFinding
	reviewer := ScopeReviewer
	max180 := 180
	max1200 := 1200
	max1500 := 1500
	weight10 := 10
	weight15 := 15
	weight20 := 20
	weight30 := 30
	weight35 := 35
	weight40 := 40
	reviewConfig := func() *StepOverride {
		return &StepOverride{
			MineReasoning:   &AgentOverride{Model: &small},
			CompileFindings: &AgentOverride{Model: &small},
		}
	}
	parallel := make([]StepEntry, len(ReviewVectorIDs))
	laneNames := []string{"Code quality", "Security", "Architecture", "Performance", "Testing", "Best practices"}
	for i, id := range ReviewVectorIDs {
		parallel[i] = StepEntry{Name: laneNames[i], Lane: []StepEntry{
			{Type: StepReviewPrefix + id, Config: reviewConfig()},
			{Type: StepVerifyPrefix + id, Config: &StepOverride{Scope: &finding, TimeBudget: &TimeBudget{Weight: &weight35}}},
			{Type: StepDedupePrefix + id, Config: &StepOverride{Scope: &reviewer, TimeBudget: &TimeBudget{Weight: &weight15}}},
		}, Config: &StepOverride{TimeBudget: &TimeBudget{MaxSeconds: &max1500}}}
	}
	want := Spec{Version: SpecVersion, Name: "Standard review", Steps: []StepEntry{
		{Type: StepCollectContext, Name: "Context", Config: &StepOverride{TimeBudget: &TimeBudget{MaxSeconds: &max180}}},
		{Name: "Review", Parallel: parallel},
		{Name: "Finalize", Pipeline: []StepEntry{
			{Type: StepMerge, Config: &StepOverride{Scope: &cluster, TimeBudget: &TimeBudget{Weight: &weight30}}},
			{Type: StepFinalize, Config: &StepOverride{Model: &small, Scope: &cluster, TimeBudget: &TimeBudget{Weight: &weight40}}},
			{Type: StepVerdict, Config: &StepOverride{Model: &small, Scope: &all, TimeBudget: &TimeBudget{Weight: &weight20}}},
			{Type: StepSummarize, Config: &StepOverride{Model: &small, Scope: &cluster, TimeBudget: &TimeBudget{Weight: &weight10}}},
		}, Config: &StepOverride{TimeBudget: &TimeBudget{MaxSeconds: &max1200}}},
	}}
	if got := DefaultSpec(); !reflect.DeepEqual(got, want) {
		t.Fatalf("embedded default.yaml drifted from constants:\n got %+v\nwant %+v", got, want)
	}
}

func TestLoadParsesPlainStepName(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - type: collect-context
    name: Context
  - type: merge
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := spec.Steps[0].Name; got != "Context" {
		t.Fatalf("plain step name = %q, want %q", got, "Context")
	}
	if got := spec.Steps[1].Name; got != "" {
		t.Fatalf("unnamed step should have no name, got %q", got)
	}
}

func TestLoadParsesLaneAndPipelineNames(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - name: Reviewers
    parallel:
      - name: Security review
        lane:
          - type: review:security
  - name: Review synthesis
    pipeline:
      - type: merge
      - type: finalize
      - type: verdict
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := spec.Steps[0].Name; got != "Reviewers" {
		t.Fatalf("parallel group name = %q, want %q", got, "Reviewers")
	}
	if got := spec.Steps[0].Parallel[0].Name; got != "Security review" {
		t.Fatalf("lane name = %q, want %q", got, "Security review")
	}
	if got := spec.Steps[1].Name; got != "Review synthesis" {
		t.Fatalf("pipeline name = %q, want %q", got, "Review synthesis")
	}
}

// TestExampleFileLoadsToDefault confirms the generated workflow.yaml.example
// round-trips the real loader and decodes to exactly DefaultSpec — the docs and
// banner comments do not change what the file parses to.
func TestExampleFileLoadsToDefault(t *testing.T) {
	spec, err := Load("../../workflow.yaml.example")
	if err != nil {
		t.Fatalf("Load(workflow.yaml.example): %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("workflow.yaml.example invalid: %v", err)
	}
	if !reflect.DeepEqual(spec, DefaultSpec()) {
		t.Fatalf("workflow.yaml.example does not decode to DefaultSpec:\n got %+v\nwant %+v", spec, DefaultSpec())
	}
}

func TestDefaultSpecReviewersAreParallel(t *testing.T) {
	spec := DefaultSpec()
	var parallel *StepEntry
	for i := range spec.Steps {
		if spec.Steps[i].IsParallel() {
			parallel = &spec.Steps[i]
			break
		}
	}
	if parallel == nil {
		t.Fatal("expected a parallel reviewer group")
	}
	if len(parallel.Parallel) != len(ReviewVectorIDs) {
		t.Fatalf("parallel lanes = %d, want %d", len(parallel.Parallel), len(ReviewVectorIDs))
	}
	for i, lane := range parallel.Parallel {
		if !lane.IsLane() || len(lane.Lane) != 3 {
			t.Fatalf("parallel child %d is not a 3-step lane: %+v", i, lane)
		}
	}
}

func TestLoadParsesMappingStepsAndParallel(t *testing.T) {
	path := writeSpec(t, `
version: 1
profile: custom
steps:
  - type: collect-context
  - parallel:
      - type: review:security
      - type: review:performance
        config:
          model: fast-model
          nudge_count: 0
          disable_patch_summary: true
  - type: merge
    findings_from:
      - a.json
      - b.json
  - type: finalize
    findings_from: merged.json
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Profile != "custom" {
		t.Fatalf("profile = %q", spec.Profile)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if spec.Steps[0].Type != StepCollectContext {
		t.Fatalf("step0 = %q", spec.Steps[0].Type)
	}
	par := spec.Steps[1]
	if !par.IsParallel() || len(par.Parallel) != 2 {
		t.Fatalf("step1 not a 2-child parallel: %+v", par)
	}
	perf := par.Parallel[1]
	if perf.Type != "review:performance" || perf.Config == nil || perf.Config.Model == nil || *perf.Config.Model != "fast-model" {
		t.Fatalf("performance override not parsed: %+v", perf)
	}
	if perf.Config.NudgeCount == nil || *perf.Config.NudgeCount != 0 {
		t.Fatalf("nudge_count override not parsed: %+v", perf.Config)
	}
	if perf.Config.DisablePatchSummary == nil || !*perf.Config.DisablePatchSummary {
		t.Fatalf("disable_patch_summary override not parsed: %+v", perf.Config)
	}
	merge := spec.Steps[2]
	if len(merge.FindingsFrom) != 2 {
		t.Fatalf("merge findings_from = %v, want 2 files", merge.FindingsFrom)
	}
	fin := spec.Steps[3]
	if len(fin.FindingsFrom) != 1 || fin.FindingsFrom[0] != "merged.json" {
		t.Fatalf("finalize findings_from = %v", fin.FindingsFrom)
	}
}

func TestLoadParsesReviewInternalOverrides(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - type: review:security
    config:
      model: primary
      mine_reasoning:
        model: "@small"
      compile_findings:
        reasoning_effort: low
      nudge:
        max_output_retries: 2
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := spec.Steps[0].Config
	if cfg == nil || cfg.MineReasoning == nil || cfg.CompileFindings == nil || cfg.Nudge == nil {
		t.Fatalf("internal overrides not parsed: %+v", cfg)
	}
	if cfg.MineReasoning.Model == nil || *cfg.MineReasoning.Model != SmallModelAlias {
		t.Fatalf("mine_reasoning model = %+v", cfg.MineReasoning.Model)
	}
	if cfg.CompileFindings.ReasoningEffort == nil || *cfg.CompileFindings.ReasoningEffort != "low" {
		t.Fatalf("compile_findings effort = %+v", cfg.CompileFindings.ReasoningEffort)
	}
	if cfg.Nudge.MaxOutputRetries == nil || *cfg.Nudge.MaxOutputRetries != 2 {
		t.Fatalf("nudge max_output_retries = %+v", cfg.Nudge.MaxOutputRetries)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	cases := map[string]string{
		"unknown top key":                 "version: 1\nbogus: x\nsteps:\n  - type: merge\n",
		"unknown step key":                "version: 1\nsteps:\n  - type: merge\n    bogus: x\n",
		"unknown config key":              "version: 1\nsteps:\n  - type: merge\n    config:\n      bogus: 1\n",
		"removed verify_drop_confidence":  "version: 1\nsteps:\n  - type: verdict\n    config:\n      verify_drop_confidence: 0.7\n",
		"review-only config on merge":     "version: 1\nsteps:\n  - type: merge\n    config:\n      mine_reasoning: {}\n",
		"unknown review internal key":     "version: 1\nsteps:\n  - type: review:security\n    config:\n      mine_reasoning:\n        bogus: 1\n",
		"non-mapping review internal key": "version: 1\nsteps:\n  - type: review:security\n    config:\n      nudge: small\n",
		"scalar step":                     "version: 1\nsteps:\n  - merge\n",
		"nested parallel":                 "version: 1\nsteps:\n  - parallel:\n      - parallel:\n          - type: merge\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeSpec(t, body)); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateRejections(t *testing.T) {
	cases := map[string]Spec{
		"bad version":       {Version: 2, Steps: []StepEntry{{Type: StepMerge}}},
		"empty steps":       {Version: 1},
		"unknown type":      {Version: 1, Steps: []StepEntry{{Type: "bogus"}}},
		"unknown vector":    {Version: 1, Steps: []StepEntry{{Type: StepReviewPrefix + "bogus"}}},
		"nudge no review":   {Version: 1, Steps: []StepEntry{{Type: StepNudgePrefix + "security"}}},
		"extract no review": {Version: 1, Steps: []StepEntry{{Type: StepExtractPrefix + "security"}}},
		// findings_from on a step that never reads it would be silently ignored.
		"findings_from on review": {Version: 1, Steps: []StepEntry{
			{Type: StepReviewPrefix + "security", FindingsFrom: []string{"a.json"}},
		}},
		// verify/dedupe after merge mutate groups nothing consumes anymore.
		"global verify after merge": {Version: 1, Steps: []StepEntry{
			{Type: StepReviewPrefix + "security"},
			{Type: StepMerge},
			{Type: StepVerify},
		}},
		"vector dedupe after merge": {Version: 1, Steps: []StepEntry{
			{Type: StepReviewPrefix + "security"},
			{Type: StepMerge},
			{Type: StepDedupePrefix + "security"},
		}},
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if err := spec.Validate(); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestValidateRejectsNudgeInSameParallelGroupAsReview(t *testing.T) {
	// review:security and nudge:security run concurrently here (different
	// lanes), so the session the nudge depends on is not yet populated — the
	// vector-ownership rule must reject this.
	spec := Spec{Version: 1, Steps: []StepEntry{
		{Parallel: []StepEntry{
			{Type: StepReviewPrefix + "security"},
			{Type: StepNudgePrefix + "security"},
		}},
	}}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected rejection of nudge in same parallel group as its review")
	}
}

func TestValidateNudgeOrExtractInParallelGroup(t *testing.T) {
	// With the review completed in an earlier unit, a bare nudge/extract child
	// is a one-step lane owning its vector — legal. Without any prior review it
	// stays rejected.
	for _, dep := range []string{StepNudgePrefix + "security", StepExtractPrefix + "security"} {
		spec := Spec{Version: 1, Steps: []StepEntry{
			{Type: StepReviewPrefix + "security"},
			{Parallel: []StepEntry{
				{Type: dep},
				{Type: StepReviewPrefix + "performance"},
			}},
		}}
		if err := spec.Validate(); err != nil {
			t.Fatalf("%q after an earlier review must be valid in a parallel group: %v", dep, err)
		}
		noReview := Spec{Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{
				{Type: dep},
				{Type: StepReviewPrefix + "performance"},
			}},
		}}
		if err := noReview.Validate(); err == nil {
			t.Fatalf("expected rejection of %q without a preceding review", dep)
		}
	}
}

func TestValidateRejectsNonReviewStepsInParallelGroup(t *testing.T) {
	// Only review:<vector> may run concurrently; other steps mutate shared
	// pipeline state and must be sequential.
	for _, bad := range []string{StepCollectContext, StepVerify, StepDedupe, StepMerge, StepFinalize} {
		spec := Spec{Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{
				{Type: StepReviewPrefix + "security"},
				{Type: bad},
			}},
		}}
		if err := spec.Validate(); err == nil {
			t.Fatalf("expected rejection of %q inside a parallel group", bad)
		}
	}
}

func TestLoadParsesPipelineAndScope(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - type: collect-context
  - parallel:
      - lane:
          - type: review:security
          - type: verify:security
            config: { scope: finding }
          - type: dedupe:security
            config: { scope: reviewer }
  - pipeline:
      - type: merge
        config: { scope: cluster }
      - type: finalize
        config: { scope: cluster }
      - type: verdict
        config: { scope: all, confidence_threshold: 0.65 }
      - type: summarize
        config: { scope: cluster }
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	last := spec.Steps[len(spec.Steps)-1]
	if !last.IsPipeline() || len(last.Pipeline) != 4 {
		t.Fatalf("last step is not a 4-step pipeline: %+v", last)
	}
	wantScope := map[string]string{StepMerge: ScopeCluster, StepFinalize: ScopeCluster, StepVerdict: ScopeAll, StepSummarize: ScopeCluster}
	for _, child := range last.Pipeline {
		if child.Config == nil || child.Config.Scope == nil {
			t.Fatalf("pipeline %q missing scope: %+v", child.Type, child)
		}
		if got := *child.Config.Scope; got != wantScope[child.Type] {
			t.Fatalf("pipeline %q scope = %q, want %q", child.Type, got, wantScope[child.Type])
		}
		if child.Type == StepVerdict {
			if child.Config.ConfidenceThreshold == nil || *child.Config.ConfidenceThreshold != 0.65 {
				t.Fatalf("verdict confidence_threshold = %+v, want 0.65", child.Config.ConfidenceThreshold)
			}
		}
	}
	lane := spec.Steps[1].Parallel[0].Lane
	if v := lane[1]; v.Config == nil || v.Config.Scope == nil || *v.Config.Scope != ScopeFinding {
		t.Fatalf("verify scope not parsed: %+v", v)
	}
	if d := lane[2]; d.Config == nil || d.Config.Scope == nil || *d.Config.Scope != ScopeReviewer {
		t.Fatalf("dedupe scope not parsed: %+v", d)
	}
}

func TestLoadNormalizesPriorityThreshold(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - type: collect-context
  - parallel:
      - lane:
          - type: review:security
  - pipeline:
      - type: merge
      - type: finalize
        config: { priority_threshold: "0" }
      - type: verdict
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	finalize := spec.Steps[len(spec.Steps)-1].Pipeline[1]
	if finalize.Type != StepFinalize {
		t.Fatalf("expected finalize step, got %q", finalize.Type)
	}
	if finalize.Config == nil || finalize.Config.PriorityThreshold == nil {
		t.Fatalf("priority_threshold not parsed: %+v", finalize.Config)
	}
	if got := *finalize.Config.PriorityThreshold; got != "p0" {
		t.Fatalf("priority_threshold = %q, want p0 (normalized from 0)", got)
	}
	// The normalized value must reach the request, so the engine's
	// PriorityThresholdRank sees "p0" (rank 0) and not the unparsed "0" (rank 3).
	_, req := finalize.Config.Resolve(config.Profile{}, model.ReviewRequest{})
	if req.PriorityThreshold != "p0" {
		t.Fatalf("resolved req.PriorityThreshold = %q, want p0", req.PriorityThreshold)
	}
}

func TestLoadRejectsInvalidPriorityThreshold(t *testing.T) {
	for _, bad := range []string{"5", "p3", "high", "-1"} {
		path := writeSpec(t, `
version: 1
steps:
  - type: finalize
    config: { priority_threshold: "`+bad+`" }
`)
		if _, err := Load(path); err == nil {
			t.Fatalf("priority_threshold %q: expected load error, got nil", bad)
		}
	}
}

func TestPipelineRejections(t *testing.T) {
	cases := map[string]string{
		"missing verdict":           "version: 1\nsteps:\n  - pipeline:\n      - type: merge\n      - type: finalize\n",
		"wrong order":               "version: 1\nsteps:\n  - pipeline:\n      - type: finalize\n      - type: merge\n      - type: verdict\n",
		"summarize before verdict":  "version: 1\nsteps:\n  - pipeline:\n      - type: merge\n      - type: finalize\n      - type: summarize\n      - type: verdict\n",
		"non-tail member":           "version: 1\nsteps:\n  - pipeline:\n      - type: merge\n      - type: finalize\n      - type: verdict\n      - type: collect-context\n",
		"findings_from on finalize": "version: 1\nsteps:\n  - pipeline:\n      - type: merge\n      - type: finalize\n        findings_from: x.json\n      - type: verdict\n",
		"wrong scope in pipeline":   "version: 1\nsteps:\n  - pipeline:\n      - type: merge\n      - type: finalize\n        config: { scope: all }\n      - type: verdict\n",
		"empty pipeline":            "version: 1\nsteps:\n  - pipeline: []\n",
		"pipeline in parallel":      "version: 1\nsteps:\n  - parallel:\n      - type: review:security\n      - pipeline:\n          - type: merge\n          - type: finalize\n          - type: verdict\n",
		"pipeline in lane":          "version: 1\nsteps:\n  - parallel:\n      - lane:\n          - type: review:security\n          - pipeline:\n              - type: merge\n",
		"nested pipeline":           "version: 1\nsteps:\n  - pipeline:\n      - type: merge\n      - pipeline:\n          - type: finalize\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			spec, err := Load(writeSpec(t, body))
			if err != nil {
				return // rejected at parse — acceptable
			}
			if err := spec.Validate(); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestScopeRejections(t *testing.T) {
	cases := map[string]string{
		"scope on collect-context": "version: 1\nsteps:\n  - type: collect-context\n    config: { scope: all }\n",
		"scope on review":          "version: 1\nsteps:\n  - type: review:security\n    config: { scope: finding }\n",
		"illegal verify scope":     "version: 1\nsteps:\n  - type: review:security\n  - type: verify:security\n    config: { scope: cluster }\n",
		"illegal merge scope":      "version: 1\nsteps:\n  - type: merge\n    config: { scope: finding }\n",
		"flat finalize cluster":    "version: 1\nsteps:\n  - type: finalize\n    config: { scope: cluster }\n",
		"flat summarize cluster":   "version: 1\nsteps:\n  - type: summarize\n    config: { scope: cluster }\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			spec, err := Load(writeSpec(t, body))
			if err != nil {
				return
			}
			if err := spec.Validate(); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestLoadParsesLanes(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - type: collect-context
  - parallel:
      - type: review:security
      - lane:
          - type: review:performance
          - type: verify:performance
            config:
              verify_drop_policy: none
          - type: dedupe:performance
  - type: merge
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	par := spec.Steps[1]
	if !par.IsParallel() || len(par.Parallel) != 2 {
		t.Fatalf("step1 not a 2-child parallel: %+v", par)
	}
	if bare := par.Parallel[0]; bare.IsLane() || bare.Type != "review:security" {
		t.Fatalf("bare child = %+v", bare)
	}
	lane := par.Parallel[1]
	if !lane.IsLane() || len(lane.Lane) != 3 {
		t.Fatalf("lane child = %+v", lane)
	}
	if got := lane.LaneSteps(); len(got) != 3 || got[0].Type != "review:performance" || got[2].Type != "dedupe:performance" {
		t.Fatalf("lane steps = %+v", got)
	}
	verify := lane.Lane[1]
	if verify.Type != "verify:performance" || verify.Config == nil || verify.Config.VerifyDropPolicy == nil || *verify.Config.VerifyDropPolicy != "none" {
		t.Fatalf("lane verify override not parsed: %+v", verify)
	}
	if bare := par.Parallel[0].LaneSteps(); len(bare) != 1 || bare[0].Type != "review:security" {
		t.Fatalf("bare child LaneSteps = %+v", bare)
	}
}

func TestLoadParsesTimeBudgetConfig(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - parallel:
      - lane:
          - type: review:testing
            config:
              time_budget:
                max_seconds: 1200
                weight: 70
              mine_reasoning:
                model: "@small"
                time_budget:
                  weight: 10
                  speedup_threshold: 100
              compile_findings:
                model: "@small"
                time_budget:
                  weight: 10
              nudge:
                model: "@small"
                time_budget:
                  weight: 10
          - type: verify:testing
            config:
              time_budget:
                weight: 0
          - type: dedupe:testing
        config:
          time_budget:
            max_seconds: 1200
            speedup_threshold: 80
  - pipeline:
      - type: merge
        config:
          scope: cluster
          time_budget:
            weight: 40
      - type: finalize
        config:
          scope: cluster
      - type: verdict
        config:
          scope: all
      - type: summarize
        config:
          scope: cluster
    config:
      time_budget:
        max_seconds: 300
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	lane := spec.Steps[0].Parallel[0]
	if lane.Config == nil || lane.Config.TimeBudget == nil || *lane.Config.TimeBudget.MaxSeconds != 1200 {
		t.Fatalf("lane time budget not parsed: %+v", lane.Config)
	}
	review := lane.Lane[0]
	if review.Config == nil || review.Config.TimeBudget == nil || *review.Config.TimeBudget.MaxSeconds != 1200 || *review.Config.TimeBudget.Weight != 70 {
		t.Fatalf("review time budget not parsed: %+v", review.Config)
	}
	if review.Config.MineReasoning == nil || review.Config.MineReasoning.TimeBudget == nil || *review.Config.MineReasoning.TimeBudget.SpeedupThreshold != 100 {
		t.Fatalf("mine_reasoning time budget not parsed: %+v", review.Config.MineReasoning)
	}
	verify := lane.Lane[1]
	if verify.Config == nil || verify.Config.TimeBudget == nil || *verify.Config.TimeBudget.Weight != 0 {
		t.Fatalf("verify weight not parsed: %+v", verify.Config)
	}
	pipeline := spec.Steps[1]
	if pipeline.Config == nil || pipeline.Config.TimeBudget == nil || *pipeline.Config.TimeBudget.MaxSeconds != 300 {
		t.Fatalf("pipeline time budget not parsed: %+v", pipeline.Config)
	}
	if merge := pipeline.Pipeline[0]; merge.Config == nil || merge.Config.TimeBudget == nil || *merge.Config.TimeBudget.Weight != 40 {
		t.Fatalf("pipeline child weight not parsed: %+v", merge.Config)
	}
}

func TestLoadRejectsBadTimeBudgetConfig(t *testing.T) {
	cases := map[string]string{
		"bad threshold low":       "version: 1\nsteps:\n  - type: merge\n    config:\n      time_budget: { speedup_threshold: 49 }\n",
		"bad threshold high":      "version: 1\nsteps:\n  - type: merge\n    config:\n      time_budget: { speedup_threshold: 101 }\n",
		"bad max seconds":         "version: 1\nsteps:\n  - type: merge\n    config:\n      time_budget: { max_seconds: 0 }\n",
		"bad weight":              "version: 1\nsteps:\n  - type: merge\n    config:\n      time_budget: { weight: 101 }\n",
		"lane weights over 100":   "version: 1\nsteps:\n  - parallel:\n      - lane:\n          - type: review:security\n            config: { time_budget: { weight: 80 } }\n          - type: verify:security\n            config: { time_budget: { weight: 30 } }\n",
		"review weights over 100": "version: 1\nsteps:\n  - type: review:security\n    config:\n      time_budget: { weight: 90 }\n      mine_reasoning:\n        time_budget: { weight: 20 }\n",
		"group unknown config":    "version: 1\nsteps:\n  - parallel:\n      - lane:\n          - type: review:security\n        config:\n          model: small\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			spec, err := Load(writeSpec(t, body))
			if err != nil {
				return
			}
			if err := spec.Validate(); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestLoadRejectsBadLanes(t *testing.T) {
	cases := map[string]string{
		"top-level lane":     "version: 1\nsteps:\n  - lane:\n      - type: review:security\n",
		"lane in lane":       "version: 1\nsteps:\n  - parallel:\n      - lane:\n          - lane:\n              - type: review:security\n",
		"parallel in lane":   "version: 1\nsteps:\n  - parallel:\n      - lane:\n          - parallel:\n              - type: review:security\n",
		"empty lane":         "version: 1\nsteps:\n  - parallel:\n      - lane: []\n",
		"lane not a list":    "version: 1\nsteps:\n  - parallel:\n      - lane: review:security\n",
		"verify_concurrency": "version: 1\nsteps:\n  - type: verify\n    config:\n      verify_concurrency: 5\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeSpec(t, body)); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateLanes(t *testing.T) {
	laneFor := func(vector string) StepEntry {
		return StepEntry{Lane: []StepEntry{
			{Type: StepReviewPrefix + vector},
			{Type: StepVerifyPrefix + vector},
			{Type: StepDedupePrefix + vector},
		}}
	}
	accept := map[string]Spec{
		"review-verify-dedupe lanes": {Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{laneFor("security"), laneFor("performance")}},
			{Type: StepMerge},
		}},
		"lane depends on earlier unit": {Version: 1, Steps: []StepEntry{
			{Type: StepReviewPrefix + "security"},
			{Parallel: []StepEntry{
				{Lane: []StepEntry{{Type: StepVerifyPrefix + "security"}, {Type: StepDedupePrefix + "security"}}},
				{Type: StepReviewPrefix + "performance"},
			}},
		}},
		"sequential per-vector verify/dedupe": {Version: 1, Steps: []StepEntry{
			{Type: StepReviewPrefix + "security"},
			{Type: StepVerifyPrefix + "security"},
			{Type: StepDedupePrefix + "security"},
		}},
	}
	for name, spec := range accept {
		t.Run("accept "+name, func(t *testing.T) {
			if err := spec.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
	reject := map[string]Spec{
		"verify without review": {Version: 1, Steps: []StepEntry{
			{Type: StepVerifyPrefix + "security"},
		}},
		"dedupe without review in lane": {Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{{Lane: []StepEntry{{Type: StepDedupePrefix + "security"}}}}},
		}},
		"two lanes own one vector": {Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{
				laneFor("security"),
				{Lane: []StepEntry{{Type: StepNudgePrefix + "security"}}},
			}},
		}},
		"global step inside lane": {Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{
				{Lane: []StepEntry{{Type: StepReviewPrefix + "security"}, {Type: StepMerge}}},
			}},
		}},
		"unknown vector in lane": {Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{{Lane: []StepEntry{{Type: StepVerifyPrefix + "bogus"}}}}},
		}},
		"top-level lane": {Version: 1, Steps: []StepEntry{
			{Lane: []StepEntry{{Type: StepReviewPrefix + "security"}}},
		}},
		"nested lane": {Version: 1, Steps: []StepEntry{
			{Parallel: []StepEntry{
				{Lane: []StepEntry{{Lane: []StepEntry{{Type: StepReviewPrefix + "security"}}}}},
			}},
		}},
	}
	for name, spec := range reject {
		t.Run("reject "+name, func(t *testing.T) {
			if err := spec.Validate(); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestValidateAcceptsReviewThenNudgeAcrossUnits(t *testing.T) {
	// review in a parallel group, then nudge in a later sequential step: ok.
	spec := Spec{Version: 1, Steps: []StepEntry{
		{Parallel: []StepEntry{{Type: StepReviewPrefix + "security"}}},
		{Type: StepNudgePrefix + "security"},
	}}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateAcceptsNudgeAfterReview(t *testing.T) {
	spec := Spec{Version: 1, Steps: []StepEntry{
		{Type: StepReviewPrefix + "security"},
		{Type: StepExtractPrefix + "security"},
		{Type: StepNudgePrefix + "security"},
	}}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestStepOverrideResolveIdentity(t *testing.T) {
	base := config.Profile{Model: "base", ReasoningEffort: "high", MaxToolCalls: 7}
	req := model.ReviewRequest{MaxToolCalls: 7, NudgeCount: 3, VerifyDropPolicy: "refuted-only"}
	gotProfile, gotReq := (*StepOverride)(nil).Resolve(base, req)
	if !reflect.DeepEqual(gotProfile, base) {
		t.Fatalf("nil override changed profile: %+v", gotProfile)
	}
	if !reflect.DeepEqual(gotReq, req) {
		t.Fatalf("nil override changed request: %+v", gotReq)
	}
	empty := &StepOverride{}
	gotProfile, gotReq = empty.Resolve(base, req)
	if !reflect.DeepEqual(gotProfile, base) || !reflect.DeepEqual(gotReq, req) {
		t.Fatalf("empty override changed values: profile=%+v req=%+v", gotProfile, gotReq)
	}
}

func TestStepOverrideResolveApplies(t *testing.T) {
	base := config.Profile{Model: "base", ReasoningEffort: "high", MaxToolCalls: 7}
	req := model.ReviewRequest{MaxToolCalls: 7, NudgeCount: 3, DisablePatchSummary: true, DisableSuggestions: true}
	zero := 0
	model5 := "opus"
	effort := "low"
	disablePatchSummary := false
	disableSuggestions := false
	ov := &StepOverride{
		Model:               &model5,
		ReasoningEffort:     &effort,
		MaxToolCalls:        &zero, // explicit zero must win (unlimited)
		NudgeCount:          &zero,
		DisablePatchSummary: &disablePatchSummary,
		DisableSuggestions:  &disableSuggestions,
	}
	gotProfile, gotReq := ov.Resolve(base, req)
	if gotProfile.Model != "opus" || gotProfile.ReasoningEffort != "low" {
		t.Fatalf("profile model params not applied: %+v", gotProfile)
	}
	if gotProfile.MaxToolCalls != 0 || gotReq.MaxToolCalls != 0 {
		t.Fatalf("explicit zero max_tool_calls not applied: profile=%d req=%d", gotProfile.MaxToolCalls, gotReq.MaxToolCalls)
	}
	if gotReq.NudgeCount != 0 {
		t.Fatalf("explicit zero nudge_count not applied: %d", gotReq.NudgeCount)
	}
	if gotReq.DisablePatchSummary {
		t.Fatal("explicit false disable_patch_summary not applied")
	}
	if gotReq.DisableSuggestions {
		t.Fatal("explicit false disable_suggestions not applied")
	}
}

func TestStepOverrideResolveMaxFindings(t *testing.T) {
	base := config.Profile{Model: "base", MaxFindings: 20}
	req := model.ReviewRequest{MaxFindings: 20}
	five := 5
	gotProfile, gotReq := (&StepOverride{MaxFindings: &five}).Resolve(base, req)
	if gotProfile.MaxFindings != 5 || gotReq.MaxFindings != 5 {
		t.Fatalf("max_findings override not applied: profile=%d req=%d", gotProfile.MaxFindings, gotReq.MaxFindings)
	}
	zero := 0
	gotProfile, gotReq = (&StepOverride{MaxFindings: &zero}).Resolve(base, req)
	if gotProfile.MaxFindings != 0 || gotReq.MaxFindings != 0 {
		t.Fatalf("explicit zero max_findings not applied: profile=%d req=%d", gotProfile.MaxFindings, gotReq.MaxFindings)
	}
	gotProfile, gotReq = (&StepOverride{}).Resolve(base, req)
	if gotProfile.MaxFindings != 20 || gotReq.MaxFindings != 20 {
		t.Fatalf("unset max_findings did not inherit: profile=%d req=%d", gotProfile.MaxFindings, gotReq.MaxFindings)
	}
}

func TestValidateRejectsNegativeMaxFindings(t *testing.T) {
	neg := -1
	spec := Spec{Version: SpecVersion, Steps: []StepEntry{
		{Type: StepReviewPrefix + "security", Config: &StepOverride{MaxFindings: &neg}},
	}}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected negative max_findings to be rejected")
	}
	if !strings.Contains(err.Error(), "max_findings must be non-negative") {
		t.Fatalf("error = %q, want max_findings non-negative message", err)
	}

	pipelineSpec := Spec{Version: SpecVersion, Steps: []StepEntry{
		{Pipeline: []StepEntry{
			{Type: StepMerge, Config: &StepOverride{MaxFindings: &neg}},
			{Type: StepFinalize},
			{Type: StepVerdict},
		}},
	}}
	err = pipelineSpec.Validate()
	if err == nil || !strings.Contains(err.Error(), "max_findings must be non-negative") {
		t.Fatalf("pipeline error = %v, want max_findings non-negative message", err)
	}
}

func TestLoadParsesMaxFindingsOverride(t *testing.T) {
	path := writeSpec(t, `
version: 1
steps:
  - type: review:security
    config:
      max_findings: 10
`)
	spec, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	cfg := spec.Steps[0].Config
	if cfg == nil || cfg.MaxFindings == nil || *cfg.MaxFindings != 10 {
		t.Fatalf("max_findings override not parsed: %+v", cfg)
	}
}

// Disabling response_format is monotonic: a per-step override may turn it off
// but must never re-enable it once the run disabled it (e.g. the model-check
// fallback for a model that lacks json_schema). Otherwise the step would send
// response_format: json_schema to a model already probed as unable to honor it.
func TestStepOverrideResolveDisableJSONResponseFormatMonotonic(t *testing.T) {
	enable := false
	reEnable := &StepOverride{DisableJSONResponseFormat: &enable}
	_, req := reEnable.Resolve(config.Profile{}, model.ReviewRequest{DisableJSONResponseFormat: true})
	if !req.DisableJSONResponseFormat {
		t.Fatal("step disable_json_response_format: false must not re-enable response_format after the run disabled it")
	}

	disable := true
	turnOff := &StepOverride{DisableJSONResponseFormat: &disable}
	_, req2 := turnOff.Resolve(config.Profile{}, model.ReviewRequest{DisableJSONResponseFormat: false})
	if !req2.DisableJSONResponseFormat {
		t.Fatal("step disable_json_response_format: true must disable response_format")
	}
}

func TestStepOverrideResolveSmallModelAlias(t *testing.T) {
	alias := SmallModelAlias
	explicitEffort := "none"
	req := model.ReviewRequest{}
	smallTemp := 0.2
	smallTopK := 40
	smallPresencePenalty := 0.1

	cases := []struct {
		name       string
		base       config.Profile
		override   StepOverride
		wantModel  string
		wantEffort string
	}{
		{
			name:       "configured small model keeps primary effort",
			base:       config.Profile{Model: "primary", Small: config.SmallModelConfig{Model: "small"}, ReasoningEffort: "high"},
			override:   StepOverride{Model: &alias},
			wantModel:  "small",
			wantEffort: "high",
		},
		{
			name:       "missing small model falls back to primary",
			base:       config.Profile{Model: "primary", ReasoningEffort: "high"},
			override:   StepOverride{Model: &alias},
			wantModel:  "primary",
			wantEffort: "high",
		},
		{
			name:       "configured small effort applies",
			base:       config.Profile{Model: "primary", Small: config.SmallModelConfig{Model: "small", ReasoningEffort: "low"}, ReasoningEffort: "high"},
			override:   StepOverride{Model: &alias},
			wantModel:  "small",
			wantEffort: "low",
		},
		{
			name:       "step effort overrides small effort",
			base:       config.Profile{Model: "primary", Small: config.SmallModelConfig{Model: "small", ReasoningEffort: "low"}, ReasoningEffort: "high"},
			override:   StepOverride{Model: &alias, ReasoningEffort: &explicitEffort},
			wantModel:  "small",
			wantEffort: "none",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotProfile, _ := tc.override.Resolve(tc.base, req)
			if gotProfile.Model != tc.wantModel || gotProfile.ReasoningEffort != tc.wantEffort {
				t.Fatalf("resolved profile = model %q effort %q, want model %q effort %q",
					gotProfile.Model, gotProfile.ReasoningEffort, tc.wantModel, tc.wantEffort)
			}
		})
	}

	base := config.Profile{
		Model:           "primary",
		ReasoningEffort: "high",
		Small: config.SmallModelConfig{
			Model:           "small",
			Temperature:     &smallTemp,
			TopK:            &smallTopK,
			PresencePenalty: &smallPresencePenalty,
		},
	}
	gotProfile, _ := (&StepOverride{Model: &alias}).Resolve(base, req)
	if gotProfile.Temperature == nil || *gotProfile.Temperature != smallTemp {
		t.Fatalf("small temperature = %v, want %v", gotProfile.Temperature, smallTemp)
	}
	if gotProfile.TopK == nil || *gotProfile.TopK != smallTopK {
		t.Fatalf("small top_k = %v, want %d", gotProfile.TopK, smallTopK)
	}
	if gotProfile.PresencePenalty == nil || *gotProfile.PresencePenalty != smallPresencePenalty {
		t.Fatalf("small presence_penalty = %v, want %v", gotProfile.PresencePenalty, smallPresencePenalty)
	}
}

func TestAgentOverrideResolveSmallModelAlias(t *testing.T) {
	alias := SmallModelAlias
	effort := "none"
	retries := 2
	base := config.Profile{Model: "primary", Small: config.SmallModelConfig{Model: "small", ReasoningEffort: "low"}, ReasoningEffort: "high"}
	req := model.ReviewRequest{MaxOutputRetries: 5}
	ov := &AgentOverride{Model: &alias, ReasoningEffort: &effort, MaxOutputRetries: &retries}

	gotProfile, gotReq := ov.Resolve(base, req)
	if gotProfile.Model != "small" || gotProfile.ReasoningEffort != "none" {
		t.Fatalf("profile = model %q effort %q, want small/none", gotProfile.Model, gotProfile.ReasoningEffort)
	}
	if gotProfile.MaxOutputRetries != 2 || gotReq.MaxOutputRetries != 2 {
		t.Fatalf("max_output_retries = profile %d req %d, want 2/2", gotProfile.MaxOutputRetries, gotReq.MaxOutputRetries)
	}
}

// Verify judges findings against the changed files (confirm gate, diff scope),
// so even a standalone --step verify on injected findings must resolve the
// source; without it the verifier prompt carries no patch context.
func TestStepNeedsSource(t *testing.T) {
	needs := []string{StepCollectContext, StepVerify, StepReviewPrefix + "codequality", StepVerifyPrefix + "codequality"}
	for _, stepType := range needs {
		if !StepNeedsSource(stepType) {
			t.Errorf("StepNeedsSource(%q) = false, want true", stepType)
		}
	}
	sourceless := []string{StepDedupe, StepMerge, StepFinalize, StepVerdict, StepSummarize}
	for _, stepType := range sourceless {
		if StepNeedsSource(stepType) {
			t.Errorf("StepNeedsSource(%q) = true, want false", stepType)
		}
	}
	if !SingleStepSpec(StepVerify, []string{"findings.json"}).NeedsSource() {
		t.Error("SingleStepSpec(verify).NeedsSource() = false, want true")
	}
}
