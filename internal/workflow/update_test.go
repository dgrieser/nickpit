package workflow

import "testing"

func TestUpdateSpecIsBuiltInOnly(t *testing.T) {
	spec := UpdateSpec()
	if spec.Name != "Review update" || len(spec.Steps) != 3 {
		t.Fatalf("unexpected update spec: %+v", spec)
	}
	for i, kind := range []string{"update", StepVerdict, StepSummarize} {
		if spec.Steps[i].Type != kind {
			t.Fatalf("stage %d: got %s, want %s", i, spec.Steps[i].Type, kind)
		}
	}
	if cfg := spec.Steps[2].Config; cfg == nil || cfg.Model == nil || *cfg.Model != SmallModelAlias {
		t.Fatal("summary must use the small model")
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("update workflow must not be accepted as a public review spec")
	}
	*spec.Steps[2].Config.Model = "changed"
	if *UpdateSpec().Steps[2].Config.Model != SmallModelAlias {
		t.Fatal("caller mutation changed the built-in workflow")
	}
}
