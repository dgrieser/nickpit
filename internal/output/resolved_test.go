package output

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestResolvedFindingDisplayAndImmutableOrder(t *testing.T) {
	high, low := 0, 3
	result := &model.ReviewResult{Findings: []model.Finding{
		{ID: "resolved", Title: "obsolete title", Body: "obsolete body", Priority: &low, Resolution: &model.FindingResolution{Reason: "Guard prevents failure."}, Suggestions: []model.Suggestion{{Body: "obsolete suggestion"}}},
		{ID: "active", Title: "Active issue", Priority: &high},
	}}
	before, _ := result.Clone()
	for _, ansi := range []bool{false, true} {
		var out bytes.Buffer
		if err := NewTerminalFormatter(&out, ansi).FormatFindings(result); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "RESOLVED") || !strings.Contains(out.String(), "Guard prevents failure.") || strings.Contains(out.String(), "obsolete") {
			t.Fatalf("wrong resolved rendering: %s", out.String())
		}
		if !reflect.DeepEqual(before, result) {
			t.Fatal("rendering sorted stored findings in place")
		}
	}
}
