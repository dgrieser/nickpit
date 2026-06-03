package review

import (
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/prompts"
)

// firstQuestion returns the first bullet of a leaf vector's questions template.
func firstQuestion(t *testing.T, questionsFile string) string {
	t.Helper()
	q, err := prompts.Load(questionsFile)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(strings.Split(strings.TrimSpace(q), "\n")[0])
}

// TestLeafFocusTemplatesContainQuestionsMarker guards the composer: it splits each
// leaf focus template at reviewerQuestionsMarker to keep only the scope. If a
// template edit drops or duplicates the marker, the composite would silently lose
// a scope or its tail — fail loudly here instead.
func TestLeafFocusTemplatesContainQuestionsMarker(t *testing.T) {
	for _, v := range reviewVectors {
		focus, err := prompts.Load(v.focusFile)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(focus, reviewerQuestionsMarker); got != 1 {
			t.Fatalf("%s focus template contains %q %d times, want 1", v.name, reviewerQuestionsMarker, got)
		}
	}
}

// TestSimpleReviewerCombinesAllVectors checks that review:simple builds one agent
// covering every leaf focus area: all six focus headings, the questions of each
// vector, no unresolved placeholder, no leaked testing restriction, empty
// constraints, and combined questions on the spec for nudge re-asks.
func TestSimpleReviewerCombinesAllVectors(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	base, err := prompts.Load("agent_review_general_system_prompt.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	st := newPipelineState(&model.ReviewContext{}, nil)
	st.baseTemplate = base

	v, ok := reviewVectorByID("simple")
	if !ok {
		t.Fatal(`reviewVectorByID("simple") not found`)
	}
	spec, err := engine.buildReviewerAgentSpec(v, st, model.ReviewRequest{UseJSONSchema: true})
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.Count(spec.system, "## FOCUS ON "); got != len(reviewVectors) {
		t.Fatalf("focus headings in simple system = %d, want %d", got, len(reviewVectors))
	}
	for _, leaf := range reviewVectors {
		fq := firstQuestion(t, leaf.questionsFile)
		if !strings.Contains(spec.system, fq) {
			t.Fatalf("simple system missing %s question %q", leaf.name, fq)
		}
		if !strings.Contains(spec.questionsSnippet, fq) {
			t.Fatalf("simple questionsSnippet (nudge) missing %s question %q", leaf.name, fq)
		}
	}
	if strings.Contains(spec.system, "{{.QuestionsSnippet}}") {
		t.Fatal("unresolved questions placeholder in simple system")
	}
	if strings.Contains(spec.system, "DO NOT assign priority 0 or 1") {
		t.Fatal("testing priority restriction leaked into simple reviewer")
	}
	if spec.constraints.MinPriority != nil || spec.constraints.MaxPriority != nil || len(spec.constraints.AllowedCorrectness) > 0 {
		t.Fatalf("simple reviewer must carry empty constraints, got %+v", spec.constraints)
	}
}
