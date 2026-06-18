package workflow

import (
	"os"
	"path/filepath"
	"reflect"
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
	reviewConfig := func() *StepOverride {
		return &StepOverride{
			MineReasoning:   &AgentOverride{Model: &small},
			CompileFindings: &AgentOverride{Model: &small},
		}
	}
	parallel := make([]StepEntry, len(ReviewVectorIDs))
	for i, id := range ReviewVectorIDs {
		parallel[i] = StepEntry{Lane: []StepEntry{
			{Type: StepReviewPrefix + id, Config: reviewConfig()},
			{Type: StepVerifyPrefix + id, Config: &StepOverride{Scope: &finding}},
			{Type: StepDedupePrefix + id, Config: &StepOverride{Scope: &reviewer}},
		}}
	}
	want := Spec{Version: SpecVersion, Steps: []StepEntry{
		{Type: StepCollectContext},
		{Parallel: parallel},
		{Pipeline: []StepEntry{
			{Type: StepMerge, Config: &StepOverride{Scope: &cluster}},
			{Type: StepFinalize, Config: &StepOverride{Model: &small, Scope: &cluster}},
			{Type: StepVerdict, Config: &StepOverride{Model: &small, Scope: &all}},
			{Type: StepSummarize, Config: &StepOverride{Model: &small, Scope: &cluster}},
		}},
	}}
	if got := DefaultSpec(); !reflect.DeepEqual(got, want) {
		t.Fatalf("embedded default.yaml drifted from constants:\n got %+v\nwant %+v", got, want)
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
        config: { scope: all }
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
	}
	lane := spec.Steps[1].Parallel[0].Lane
	if v := lane[1]; v.Config == nil || v.Config.Scope == nil || *v.Config.Scope != ScopeFinding {
		t.Fatalf("verify scope not parsed: %+v", v)
	}
	if d := lane[2]; d.Config == nil || d.Config.Scope == nil || *d.Config.Scope != ScopeReviewer {
		t.Fatalf("dedupe scope not parsed: %+v", d)
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
	req := model.ReviewRequest{MaxToolCalls: 7, NudgeCount: 3, DisablePatchSummary: true, SkipSuggestions: true}
	zero := 0
	model5 := "opus"
	effort := "low"
	disablePatchSummary := false
	skipSuggestions := false
	ov := &StepOverride{
		Model:               &model5,
		ReasoningEffort:     &effort,
		MaxToolCalls:        &zero, // explicit zero must win (unlimited)
		NudgeCount:          &zero,
		DisablePatchSummary: &disablePatchSummary,
		SkipSuggestions:     &skipSuggestions,
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
	if gotReq.SkipSuggestions {
		t.Fatal("explicit false skip_suggestions not applied")
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
