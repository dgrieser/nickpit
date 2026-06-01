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
	if err := DefaultReviewSpec().Validate(); err != nil {
		t.Fatalf("DefaultReviewSpec invalid: %v", err)
	}
	full := DefaultSpec()
	if last := full.Steps[len(full.Steps)-1]; last.Type != StepFinalize {
		t.Fatalf("DefaultSpec last step = %q, want finalize", last.Type)
	}
	for _, s := range DefaultReviewSpec().Steps {
		if s.Type == StepFinalize {
			t.Fatal("DefaultReviewSpec must not contain finalize")
		}
	}
}

func TestDefaultSpecReviewersAreParallel(t *testing.T) {
	spec := DefaultReviewSpec()
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
		t.Fatalf("parallel reviewers = %d, want %d", len(parallel.Parallel), len(ReviewVectorIDs))
	}
}

func TestLoadParsesStringMappingAndParallel(t *testing.T) {
	path := writeSpec(t, `
version: 1
profile: custom
steps:
  - collect-context
  - parallel:
      - review:security
      - type: review:performance
        config:
          model: fast-model
          nudge_count: 0
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
	merge := spec.Steps[2]
	if len(merge.FindingsFrom) != 2 {
		t.Fatalf("merge findings_from = %v, want 2 files", merge.FindingsFrom)
	}
	fin := spec.Steps[3]
	if len(fin.FindingsFrom) != 1 || fin.FindingsFrom[0] != "merged.json" {
		t.Fatalf("finalize findings_from = %v", fin.FindingsFrom)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	cases := map[string]string{
		"unknown top key":    "version: 1\nbogus: x\nsteps: [merge]\n",
		"unknown step key":   "version: 1\nsteps:\n  - type: merge\n    bogus: x\n",
		"unknown config key": "version: 1\nsteps:\n  - type: merge\n    config:\n      bogus: 1\n",
		"nested parallel":    "version: 1\nsteps:\n  - parallel:\n      - parallel:\n          - merge\n",
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
	// review:security and nudge:security run concurrently here, so the session
	// the nudge depends on is not yet populated — must be rejected.
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

func TestValidateRejectsNudgeOrExtractInParallelGroup(t *testing.T) {
	// Even with the review completed in an earlier unit, nudge/extract steps
	// mutate the shared session and must not run concurrently.
	for _, dep := range []string{StepNudgePrefix + "security", StepExtractPrefix + "security"} {
		spec := Spec{Version: 1, Steps: []StepEntry{
			{Type: StepReviewPrefix + "security"},
			{Parallel: []StepEntry{
				{Type: dep},
				{Type: StepReviewPrefix + "performance"},
			}},
		}}
		if err := spec.Validate(); err == nil {
			t.Fatalf("expected rejection of %q inside a parallel group", dep)
		}
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
	req := model.ReviewRequest{MaxToolCalls: 7, NudgeCount: 3}
	zero := 0
	model5 := "opus"
	effort := "low"
	ov := &StepOverride{
		Model:           &model5,
		ReasoningEffort: &effort,
		MaxToolCalls:    &zero, // explicit zero must win (unlimited)
		NudgeCount:      &zero,
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
}
