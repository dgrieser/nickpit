package review

import (
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// goReviewContext builds a minimal review context touching a Go file with the
// given detected go toolchain versions.
func goReviewContext(versions ...string) *model.ReviewContext {
	ctx := &model.ReviewContext{
		ChangedFiles: []model.ChangedFile{{Path: "main.go", Status: model.FileModified}},
	}
	for _, v := range versions {
		ctx.ToolchainVersions = append(ctx.ToolchainVersions, model.ToolchainVersion{Language: "go", Version: v})
	}
	return ctx
}

func styleGuideContents(guides []model.StyleGuide) string {
	var b strings.Builder
	for _, g := range guides {
		b.WriteString(g.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func TestStyleGuidesForSelectsVersionSpecificBuiltin(t *testing.T) {
	e := &Engine{}
	guides, err := e.styleGuidesFor(goReviewContext("1.19"))
	if err != nil {
		t.Fatal(err)
	}
	all := styleGuideContents(guides)
	if !strings.Contains(all, "### Go 1.19 — Complete Developer Guideline") {
		t.Fatalf("expected go 1.19 guide, got: %.120q", all)
	}
	if strings.Contains(all, "### Go Style Guide") {
		t.Fatalf("default go guide must not appear when 1.19 detected: %.120q", all)
	}
}

func TestStyleGuidesForFallsBackToDefaultBuiltin(t *testing.T) {
	e := &Engine{}
	// A non-matching version and no detected version both fall back to default.
	for _, ctx := range []*model.ReviewContext{goReviewContext("1.20"), goReviewContext()} {
		guides, err := e.styleGuidesFor(ctx)
		if err != nil {
			t.Fatal(err)
		}
		all := styleGuideContents(guides)
		if !strings.Contains(all, "### Go Style Guide") {
			t.Fatalf("expected default go guide, got: %.120q", all)
		}
		if strings.Contains(all, "### Go 1.19 — Complete Developer Guideline") {
			t.Fatalf("version guide must not appear for non-matching version: %.120q", all)
		}
	}
}

func TestStyleGuidesForLowestDetectedVersionWins(t *testing.T) {
	e := &Engine{}
	// go.mod says 1.19, Dockerfile says 1.22: the lowest (1.19) wins, so the
	// version-specific 1.19 guide is selected over the default.
	guides, err := e.styleGuidesFor(goReviewContext("1.22", "1.19"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(styleGuideContents(guides), "### Go 1.19 — Complete Developer Guideline") {
		t.Fatalf("lowest detected version (1.19) should select the 1.19 guide")
	}
}

func TestStyleGuidesForGatesAdditionalGuides(t *testing.T) {
	e := &Engine{}
	e.SetAdditionalStyleGuides([]model.AdditionalStyleGuide{
		{StyleGuide: model.StyleGuide{Content: "UNGATED-GUIDE"}},
		{StyleGuide: model.StyleGuide{Content: "GO-ANY-GUIDE"}, GateLanguage: "go"},
		{StyleGuide: model.StyleGuide{Content: "GO-119-GUIDE"}, GateLanguage: "go", GateVersion: "1.19"},
		{StyleGuide: model.StyleGuide{Content: "GO-122-GUIDE"}, GateLanguage: "go", GateVersion: ">=1.22"},
		{StyleGuide: model.StyleGuide{Content: "PY-GUIDE"}, GateLanguage: "python"},
	})

	// go 1.19: ungated + language-gated go + version-gated 1.19 apply; the
	// >=1.22 gate and the python gate do not.
	all := styleGuideContents(mustStyleGuides(t, e, goReviewContext("1.19")))
	assertContainsAll(t, all, "UNGATED-GUIDE", "GO-ANY-GUIDE", "GO-119-GUIDE")
	assertContainsNone(t, all, "GO-122-GUIDE", "PY-GUIDE")

	// go 1.22: the >=1.22 gate wins its language; the 1.19 gate does not fire.
	all = styleGuideContents(mustStyleGuides(t, e, goReviewContext("1.22")))
	assertContainsAll(t, all, "UNGATED-GUIDE", "GO-ANY-GUIDE", "GO-122-GUIDE")
	assertContainsNone(t, all, "GO-119-GUIDE", "PY-GUIDE")
}

func mustStyleGuides(t *testing.T, e *Engine, ctx *model.ReviewContext) []model.StyleGuide {
	t.Helper()
	guides, err := e.styleGuidesFor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return guides
}

func assertContainsAll(t *testing.T, haystack string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(haystack, want) {
			t.Fatalf("missing %q in styleguides", want)
		}
	}
}

func assertContainsNone(t *testing.T, haystack string, notWants ...string) {
	t.Helper()
	for _, notWant := range notWants {
		if strings.Contains(haystack, notWant) {
			t.Fatalf("unexpected %q in styleguides", notWant)
		}
	}
}
