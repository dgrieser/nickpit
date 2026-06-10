package reviewmd

import (
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestSanitize(t *testing.T) {
	// C0/ESC/BEL control bytes (terminal-escape vectors) are stripped.
	if got := Sanitize("a\x00\x1b\x07b"); got != "ab" {
		t.Fatalf("control strip = %q, want %q", got, "ab")
	}
	// An injected nickpit marker in untrusted text is defused so it cannot
	// poison re-run dedupe.
	got := Sanitize("evil " + SummaryMarker + " text")
	if strings.Contains(got, MarkerOpen) {
		t.Fatalf("marker token not defused: %q", got)
	}
	markers := map[string]struct{}{}
	CollectMarkers(got, markers)
	if len(markers) != 0 {
		t.Fatalf("defused text still yielded markers: %v", markers)
	}
}

func TestCollectMarkers(t *testing.T) {
	markers := map[string]struct{}{}
	CollectMarkers("prefix "+SummaryMarker+" mid "+FindingMarker("x")+" end", markers)
	if _, ok := markers[SummaryMarker]; !ok {
		t.Fatalf("summary marker not collected: %v", markers)
	}
	if _, ok := markers[FindingMarker("x")]; !ok {
		t.Fatalf("finding marker not collected: %v", markers)
	}
	if len(markers) != 2 {
		t.Fatalf("collected %d markers, want 2: %v", len(markers), markers)
	}
}

func TestFindingDisplayPrefersSummarization(t *testing.T) {
	finding := model.Finding{
		Title:           "Original title",
		Body:            "original body",
		ConfidenceScore: 0.5,
		Priority:        func() *int { p := 3; return &p }(),
		Finalization:    &model.FindingFinalization{Title: "Final title", Body: "long finalized body", Priority: 2, ConfidenceScore: 0.7, Remarks: "kept"},
		Summarization:   &model.FindingSummarization{Title: "Final title", Body: "short summary", Priority: 2, ConfidenceScore: 0.7, Remarks: "kept"},
	}
	title, body, rank, confidence := FindingDisplay(finding)
	if body != "short summary" {
		t.Fatalf("body = %q, want short summary", body)
	}
	if title != "Final title" {
		t.Fatalf("title = %q, want Final title", title)
	}
	if rank != 2 {
		t.Fatalf("rank = %d, want 2 (from summarization)", rank)
	}
	if confidence != 0.7 {
		t.Fatalf("confidence = %v, want 0.7 (from summarization)", confidence)
	}
}

func TestNewRendererNormalizesAssetBaseURL(t *testing.T) {
	if r := NewRenderer(""); r.assetBaseURL != DefaultAssetBaseURL {
		t.Fatalf("empty base = %q, want default %q", r.assetBaseURL, DefaultAssetBaseURL)
	}
	if r := NewRenderer("https://host/x"); r.assetBaseURL != "https://host/x/" {
		t.Fatalf("base = %q, want trailing slash added", r.assetBaseURL)
	}
}

func TestRendererBadges(t *testing.T) {
	r := NewRenderer("https://host/")
	if got := r.CorrectnessBadge("patch is incorrect"); got != "![incorrect](https://host/incorrect.svg)" {
		t.Fatalf("incorrect badge = %q", got)
	}
	if got := r.CorrectnessBadge("patch is correct"); got != "![correct](https://host/correct.svg)" {
		t.Fatalf("correct badge = %q", got)
	}
	// Out-of-range ranks clamp to [0,3] so the image link never breaks.
	if got := r.PriorityBadge(9); got != "![P3](https://host/p3.svg)" {
		t.Fatalf("clamp-high badge = %q", got)
	}
	if got := r.PriorityBadge(-1); got != "![P0](https://host/p0.svg)" {
		t.Fatalf("clamp-low badge = %q", got)
	}
}

func TestSummaryBodyTaggedAndBadged(t *testing.T) {
	r := NewRenderer("https://host/")
	body := r.SummaryBody(&model.ReviewResult{
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "boom\n\nsecond paragraph",
		OverallConfidenceScore: 0.9,
	})
	if !strings.HasPrefix(body, SummaryMarker) {
		t.Fatalf("summary not tagged with marker: %q", body)
	}
	if !strings.Contains(body, "incorrect.svg") || !strings.Contains(body, "90% confidence") || !strings.Contains(body, "boom") {
		t.Fatalf("summary missing badge/confidence/explanation: %q", body)
	}
	if !strings.Contains(body, "_(90% confidence)_  \n\nboom  \n\nsecond paragraph  \n") {
		t.Fatalf("summary missing hard breaks after paragraphs: %q", body)
	}
}

func TestFindingBodyPrefixAndMarker(t *testing.T) {
	r := NewRenderer("https://host/")
	body := r.FindingBody(model.Finding{
		ID:              "f1",
		Title:           "Title",
		Body:            "Detail",
		ConfidenceScore: 0.91,
		Priority:        func() *int { p := 1; return &p }(),
		Suggestions:     []model.Suggestion{{Body: "do x"}},
	}, "`file.go:5`")
	if !strings.HasPrefix(body, FindingMarker("f1")) {
		t.Fatalf("finding not tagged: %q", body)
	}
	wantPrefix := FindingMarker("f1") + "\n\n" +
		"![P1](https://host/p1.svg)  \n" +
		"_(91% confidence)_  \n\n" +
		"`file.go:5`  \n\n" +
		"### Title  \n\n" +
		"Detail  "
	if !strings.HasPrefix(body, wantPrefix) {
		t.Fatalf("finding body order = %q, want prefix %q", body, wantPrefix)
	}
	if !strings.Contains(body, "`file.go:5`") || !strings.Contains(body, "### Title") || !strings.Contains(body, "- do x") {
		t.Fatalf("finding body missing prefix/title/suggestion: %q", body)
	}
	if !strings.Contains(body, "\n\n**Suggestions**  \n\n- do x  ") {
		t.Fatalf("finding suggestions missing hard breaks: %q", body)
	}
}

func TestLocateLine(t *testing.T) {
	// new1=context " a", new2=added "+b", new3=context " c".
	hunks := []model.DiffHunk{{FilePath: "main.go", OldStart: 1, NewStart: 1, Content: " a\n+b\n c\n"}}
	if loc, ok := LocateLine(hunks, 2); !ok || !loc.Added || loc.NewLine != 2 {
		t.Fatalf("line 2 = %+v ok=%v, want added new=2", loc, ok)
	}
	if loc, ok := LocateLine(hunks, 3); !ok || loc.Added || loc.NewLine != 3 || loc.OldLine != 2 {
		t.Fatalf("line 3 = %+v ok=%v, want context new=3 old=2", loc, ok)
	}
	if _, ok := LocateLine(hunks, 99); ok {
		t.Fatal("line 99 must not map")
	}
}
