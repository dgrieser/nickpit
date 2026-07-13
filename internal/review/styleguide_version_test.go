package review

import (
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// goReviewContext builds a minimal review context touching a Go file with the
// given detected go toolchain versions. Entries carry no Source, so they all
// share the lowest priority tier.
func goReviewContext(versions ...string) *model.ReviewContext {
	var entries []model.ToolchainVersion
	for _, v := range versions {
		entries = append(entries, model.ToolchainVersion{Language: "go", Version: v})
	}
	return goReviewContextEntries(entries...)
}

// goReviewContextEntries builds a minimal review context touching a Go file
// with fully specified toolchain entries (including Source).
func goReviewContextEntries(entries ...model.ToolchainVersion) *model.ReviewContext {
	return &model.ReviewContext{
		ChangedFiles:      []model.ChangedFile{{Path: "main.go", Status: model.FileModified}},
		ToolchainVersions: entries,
	}
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
	if strings.Contains(all, "### Go — Common Developer Guideline") {
		t.Fatalf("default go guide must not appear when 1.19 detected: %.120q", all)
	}
}

func TestStyleGuidesForFallsBackToDefaultBuiltin(t *testing.T) {
	e := &Engine{}
	// A non-matching version and no detected version both fall back to default.
	for _, ctx := range []*model.ReviewContext{goReviewContext("1.30"), goReviewContext()} {
		guides, err := e.styleGuidesFor(ctx)
		if err != nil {
			t.Fatal(err)
		}
		all := styleGuideContents(guides)
		if !strings.Contains(all, "### Go — Common Developer Guideline") {
			t.Fatalf("expected default go guide, got: %.120q", all)
		}
		if strings.Contains(all, "### Go 1.19 — Complete Developer Guideline") {
			t.Fatalf("version guide must not appear for non-matching version: %.120q", all)
		}
	}
}

func TestStyleGuidesForLowestVersionWinsWithinTier(t *testing.T) {
	e := &Engine{}
	// Both entries carry no Source, so they share one priority tier; within a
	// tier the lowest version (1.19) wins.
	guides, err := e.styleGuidesFor(goReviewContext("1.22", "1.19"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(styleGuideContents(guides), "### Go 1.19 — Complete Developer Guideline") {
		t.Fatalf("lowest detected version (1.19) should select the 1.19 guide")
	}
}

func TestStyleGuidesForAuthoritativeSourceBeatsLowerVersion(t *testing.T) {
	e := &Engine{}
	// go.mod says 1.22, a stale Dockerfile says 1.19: go.mod outranks the
	// Dockerfile, so the 1.22 guide is selected — not the old lowest-wins 1.19.
	guides, err := e.styleGuidesFor(goReviewContextEntries(
		model.ToolchainVersion{Language: "go", Source: "go.mod", Field: "go", Version: "1.22"},
		model.ToolchainVersion{Language: "go", Source: "Dockerfile", Field: "FROM golang", Version: "1.19"},
	))
	if err != nil {
		t.Fatal(err)
	}
	all := styleGuideContents(guides)
	if !strings.Contains(all, "### Go 1.22 — Complete Developer Guideline") {
		t.Fatalf("go.mod version (1.22) should win over Dockerfile: %.120q", all)
	}
	if strings.Contains(all, "### Go 1.19 — Complete Developer Guideline") {
		t.Fatalf("Dockerfile version (1.19) must not select a guide when go.mod is present")
	}
}

func TestStyleGuidesForFallsThroughTiers(t *testing.T) {
	e := &Engine{}
	// Without go.mod the next available tier decides.
	guides, err := e.styleGuidesFor(goReviewContextEntries(
		model.ToolchainVersion{Language: "go", Source: "Dockerfile", Field: "FROM golang", Version: "1.19"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(styleGuideContents(guides), "### Go 1.19 — Complete Developer Guideline") {
		t.Fatalf("Dockerfile version should decide when no higher-priority source exists")
	}

	// A go.mod that failed to parse yields no usable version either.
	guides, err = e.styleGuidesFor(goReviewContextEntries(
		model.ToolchainVersion{Language: "go", Source: "go.mod", Error: "parse error"},
		model.ToolchainVersion{Language: "go", Source: "Dockerfile", Field: "FROM golang", Version: "1.22"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(styleGuideContents(guides), "### Go 1.22 — Complete Developer Guideline") {
		t.Fatalf("errored go.mod entry should fall through to the Dockerfile version")
	}
}

func TestStyleGuidesForGoModDirectivesTieBreakLowest(t *testing.T) {
	e := &Engine{}
	// go directive and toolchain directive share the go.mod tier; the lower
	// language version (the one the code is compiled against) wins.
	guides, err := e.styleGuidesFor(goReviewContextEntries(
		model.ToolchainVersion{Language: "go", Source: "go.mod", Field: "go", Version: "1.21"},
		model.ToolchainVersion{Language: "go", Source: "go.mod", Field: "toolchain", Version: "go1.22.1"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(styleGuideContents(guides), "### Go 1.21 — Complete Developer Guideline") {
		t.Fatalf("go directive (1.21) should win over the higher toolchain directive within go.mod")
	}
}

func TestStyleGuidesForLowerTierCannotResurrectVersionedGuide(t *testing.T) {
	e := &Engine{}
	// go.mod's 1.30 matches no configured version key; the Dockerfile's 1.22
	// must not step in — the default guide applies.
	guides, err := e.styleGuidesFor(goReviewContextEntries(
		model.ToolchainVersion{Language: "go", Source: "go.mod", Field: "go", Version: "1.30"},
		model.ToolchainVersion{Language: "go", Source: "Dockerfile", Field: "FROM golang", Version: "1.22"},
	))
	if err != nil {
		t.Fatal(err)
	}
	all := styleGuideContents(guides)
	if !strings.Contains(all, "### Go — Common Developer Guideline") {
		t.Fatalf("expected default go guide when go.mod version matches no key: %.120q", all)
	}
	if strings.Contains(all, "### Go 1.22 — Complete Developer Guideline") {
		t.Fatalf("lower-priority Dockerfile must not resurrect a versioned guide")
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

	// go.mod 1.22 with a stale Dockerfile 1.19: gating follows the weighted
	// selection, so the >=1.22 gate fires and the 1.19 gate does not.
	all = styleGuideContents(mustStyleGuides(t, e, goReviewContextEntries(
		model.ToolchainVersion{Language: "go", Source: "go.mod", Field: "go", Version: "1.22"},
		model.ToolchainVersion{Language: "go", Source: "Dockerfile", Field: "FROM golang", Version: "1.19"},
	)))
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
