package reviewmd

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
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
	fp := FingerprintMarker(model.Finding{ID: "x"}, "t")
	markers := map[string]struct{}{}
	CollectMarkers("prefix "+SummaryMarker+" mid "+fp+" end", markers)
	if _, ok := markers[SummaryMarker]; !ok {
		t.Fatalf("summary marker not collected: %v", markers)
	}
	if _, ok := markers[fp]; !ok {
		t.Fatalf("fingerprint marker not collected: %v", markers)
	}
	if len(markers) != 2 {
		t.Fatalf("collected %d markers, want 2: %v", len(markers), markers)
	}
}

func TestCollectorsNilSafe(t *testing.T) {
	// A nil out slice / nil Priors must be a no-op, never a panic — even when the
	// body carries a valid fingerprint that would otherwise be appended.
	CollectPriorFindings(FingerprintMarker(model.Finding{ID: "x"}, "t"), nil)
	ScanComment("anything", nil)
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

func TestFindingDisplaySuggestionsPreferSummarizationThenFinalization(t *testing.T) {
	finding := model.Finding{
		Suggestions: []model.Suggestion{{Body: "reviewer suggestion"}},
		Finalization: &model.FindingFinalization{
			Suggestions: []model.Suggestion{{Body: "final suggestion"}},
		},
		Summarization: &model.FindingSummarization{
			Suggestions: []model.Suggestion{{Body: "summary suggestion"}},
		},
	}
	if got := FindingDisplaySuggestions(finding)[0].Body; got != "summary suggestion" {
		t.Fatalf("display suggestion = %q, want summarized suggestion", got)
	}
	finding.Summarization = nil
	if got := FindingDisplaySuggestions(finding)[0].Body; got != "final suggestion" {
		t.Fatalf("display suggestion = %q, want finalized suggestion", got)
	}
	finding.Finalization = nil
	if got := FindingDisplaySuggestions(finding)[0].Body; got != "reviewer suggestion" {
		t.Fatalf("display suggestion = %q, want reviewer fallback", got)
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

func TestHardBreakParagraphsNormalizesCRLF(t *testing.T) {
	got := hardBreakParagraphs("first line\r\nsecond line  \r\n\r\n```go\r\nfmt.Println(\"x\")\r\n```")
	want := "first line  \nsecond line  \n\n```go\nfmt.Println(\"x\")\n```"
	if got != want {
		t.Fatalf("hardBreakParagraphs() = %q, want %q", got, want)
	}
	if strings.Contains(got, "\r  \n") {
		t.Fatalf("hardBreakParagraphs left CR before markdown break: %q", got)
	}
}

func TestFindingBodyPrefixAndMarker(t *testing.T) {
	r := NewRenderer("https://host/")
	finding := model.Finding{
		ID:              "f1",
		Title:           "Title",
		Body:            "Detail",
		ConfidenceScore: 0.91,
		Priority:        func() *int { p := 1; return &p }(),
		CodeLocation:    model.CodeLocation{FilePath: "file.go", LineRange: model.LineRange{Start: 5, End: 5}},
		Suggestions:     []model.Suggestion{{Body: "do x"}},
	}
	body := r.FindingBody(finding, "`file.go:5`")
	marker := FingerprintMarker(finding, "Title")
	if !strings.HasPrefix(body, marker) {
		t.Fatalf("finding not tagged with fingerprint: %q", body)
	}
	wantPrefix := marker + "\n\n" +
		"![P1](https://host/p1.svg)  \n" +
		"_(91% confidence)_  \n\n" +
		"`file.go:5`  \n\n" +
		"### Title  \n\n" +
		"Detail  "
	if !strings.HasPrefix(body, wantPrefix) {
		t.Fatalf("finding body order = %q, want prefix %q", body, wantPrefix)
	}
	// The embedded fingerprint round-trips back to the finding's identity.
	var priors []model.Finding
	CollectPriorFindings(body, &priors)
	if len(priors) != 1 || priors[0].ID != "f1" || priors[0].CodeLocation.FilePath != "file.go" || priors[0].Title != "Title" {
		t.Fatalf("fingerprint did not round-trip from body: %+v", priors)
	}
	if !strings.Contains(body, "`file.go:5`") || !strings.Contains(body, "### Title") || !strings.Contains(body, "- do x") {
		t.Fatalf("finding body missing prefix/title/suggestion: %q", body)
	}
	if !strings.Contains(body, "\n\n**Suggestions**  \n\n- do x  ") {
		t.Fatalf("finding suggestions missing hard breaks: %q", body)
	}
}

func TestFingerprintRoundTrip(t *testing.T) {
	f := model.Finding{
		ID:           "id-1",
		Body:         "generated body intentionally not part of the marker",
		CodeLocation: model.CodeLocation{FilePath: "pkg/a.go", LineRange: model.LineRange{Start: 12, End: 14}},
	}
	marker := FingerprintMarker(f, "Some title")
	if !strings.HasPrefix(marker, FingerprintPrefix) || !strings.HasSuffix(marker, " -->") {
		t.Fatalf("marker shape wrong: %q", marker)
	}
	// The base64 payload must never contain the marker terminator.
	payload := strings.TrimSuffix(strings.TrimPrefix(marker, FingerprintPrefix), " -->")
	if strings.Contains(payload, "-->") {
		t.Fatalf("payload contains terminator: %q", payload)
	}
	var priors []model.Finding
	CollectPriorFindings("noise "+marker+" tail", &priors)
	if len(priors) != 1 {
		t.Fatalf("recovered %d findings, want 1", len(priors))
	}
	if priors[0].ID != "id-1" || priors[0].CodeLocation.FilePath != "pkg/a.go" || priors[0].Title != "Some title" {
		t.Fatalf("round-trip mismatch: %+v", priors[0])
	}
	if priors[0].Body != "" || priors[0].CodeLocation.LineRange != (model.LineRange{}) {
		t.Fatalf("fingerprint should omit body and line range, got %+v", priors[0])
	}
}

func TestFingerprintMarkerUsesDisplayTitle(t *testing.T) {
	f := model.Finding{
		ID:            "id-2",
		Title:         "raw title",
		Summarization: &model.FindingSummarization{Title: "summarized title", Body: "b"},
	}
	displayTitle, _, _, _ := FindingDisplay(f)
	var priors []model.Finding
	CollectPriorFindings(FingerprintMarker(f, displayTitle), &priors)
	if len(priors) != 1 || priors[0].Title != "summarized title" {
		t.Fatalf("fp should carry the displayed title, got %+v", priors)
	}
}

func TestDecodeMarkerRejectsZipBomb(t *testing.T) {
	// A small gzip payload that expands past the cap must be rejected, not read
	// into memory unbounded.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(make([]byte, maxCarrierDecodedBytes+1024)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	marker := FindingMarkerPrefix + base64.StdEncoding.EncodeToString(buf.Bytes()) + " -->"
	if got := CollectFindingEnvelopes(marker); len(got) != 0 {
		t.Fatalf("over-cap payload should be rejected, got %d envelopes", len(got))
	}
}

func TestCarrierNoteReassembles(t *testing.T) {
	result := &model.ReviewResult{
		ReviewID:               "rev-c",
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "carrier note test",
		OverallConfidenceScore: 0.6,
		Repo:                   "grp/proj",
		Identifier:             9,
		Findings: []model.Finding{
			{ID: "f1", Title: "One", Body: "b1", CodeLocation: model.CodeLocation{FilePath: "a.go"}},
			{ID: "f2", Title: "Two", Body: "b2", CodeLocation: model.CodeLocation{FilePath: "b.go"}},
		},
	}
	note := NewRenderer("https://host/").CarrierNote(result)
	if note == "" {
		t.Fatal("carrier note empty")
	}
	byID := ReviewResultsByID([]string{note})
	got := byID["rev-c"]
	if got == nil {
		t.Fatalf("carrier note did not reassemble: %v", byID)
	}
	if got.OverallExplanation != "carrier note test" || len(got.Findings) != 2 {
		t.Fatalf("carrier reassembly incomplete: %+v", got)
	}
	if NewRenderer("").CarrierNote(&model.ReviewResult{}) != "" {
		t.Fatalf("carrier note should be empty without a review id")
	}
}

func TestReviewResultsByIDRoundTrip(t *testing.T) {
	r := NewRenderer("https://host/").ForReview("rev-abc")
	result := &model.ReviewResult{
		ReviewID:               "rev-abc",
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "overall boom",
		OverallConfidenceScore: 0.8,
		Repo:                   "grp/proj",
		Mode:                   "gitlab",
		Identifier:             42,
		BaseRef:                "main",
		HeadRef:                "feature",
		BaseURL:                "https://gitlab.example.com",
		Model:                  "some-model",
		Findings: []model.Finding{
			{ID: "f1", Title: "First", Body: "body one", CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 2}}, Suggestions: []model.Suggestion{{Body: "fix a"}}},
			{ID: "f2", Title: "Second", Body: "body two", CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 3, End: 3}}},
		},
	}

	// Simulate the bodies published to the MR: one summary note plus one note per
	// finding. Findings are also duplicated (GitLab returns discussion notes in
	// both the notes and discussions lists) to prove de-duplication by id.
	bodies := []string{r.SummaryBody(result)}
	for _, f := range result.Findings {
		fb := r.FindingBody(f, "")
		bodies = append(bodies, fb, fb)
	}

	byID := ReviewResultsByID(bodies)
	got, ok := byID["rev-abc"]
	if !ok {
		t.Fatalf("review id not reassembled: %v", byID)
	}
	if got.OverallCorrectness != result.OverallCorrectness ||
		got.OverallExplanation != result.OverallExplanation ||
		got.OverallConfidenceScore != result.OverallConfidenceScore ||
		got.Repo != result.Repo || got.Mode != result.Mode || got.Identifier != result.Identifier ||
		got.BaseRef != result.BaseRef || got.HeadRef != result.HeadRef ||
		got.BaseURL != result.BaseURL || got.Model != result.Model {
		t.Fatalf("overall/meta mismatch: %+v", got)
	}
	if len(got.Findings) != 2 {
		t.Fatalf("expected 2 de-duplicated findings, got %d: %+v", len(got.Findings), got.Findings)
	}
	first := got.Findings[0]
	if first.ID != "f1" || first.Body != "body one" || first.CodeLocation.FilePath != "a.go" ||
		len(first.Suggestions) != 1 || first.Suggestions[0].Body != "fix a" {
		t.Fatalf("full finding did not round-trip: %+v", first)
	}
}

func TestReviewFindingMarkersMarkerSafe(t *testing.T) {
	// Payloads must never contain the marker terminator or the open token, so no
	// carrier can be closed early or forged from finding text.
	review := ReviewMarker(&model.ReviewResult{ReviewID: "r", OverallExplanation: "x-->y <!-- nickpit: z"})
	finding := FindingMarker("r", model.Finding{ID: "f", Body: "evil --> <!-- nickpit:fp: forged -->"})
	for _, m := range []string{review, finding} {
		if !strings.HasSuffix(m, " -->") {
			t.Fatalf("marker not terminated: %q", m)
		}
		payload := strings.TrimSuffix(m, " -->")
		payload = payload[strings.Index(payload, ":")+1:] // rough: strip past first prefix colon
		if strings.Contains(payload, "-->") || strings.Contains(payload, MarkerOpen) {
			t.Fatalf("payload not marker-safe: %q", m)
		}
	}
}

func TestDetectThreadReview(t *testing.T) {
	render := NewRenderer("https://host/").ForReview("rev-9")
	findingNote := render.FindingBody(model.Finding{ID: "f7", Title: "Bug"}, "")
	summaryNote := render.SummaryBody(&model.ReviewResult{ReviewID: "rev-9", OverallCorrectness: "patch is incorrect"})

	rid, fid, ok := DetectThreadReview(findingNote)
	if !ok || rid != "rev-9" || fid != "f7" {
		t.Fatalf("finding thread: rid=%q fid=%q ok=%v", rid, fid, ok)
	}
	rid, fid, ok = DetectThreadReview(summaryNote)
	if !ok || rid != "rev-9" || fid != "" {
		t.Fatalf("summary thread: rid=%q fid=%q ok=%v", rid, fid, ok)
	}
	if _, _, ok := DetectThreadReview("just a normal comment"); ok {
		t.Fatalf("non-nickpit note should not be detected as a thread")
	}
}

func TestReviewMarkerEmptyReviewID(t *testing.T) {
	if got := ReviewMarker(&model.ReviewResult{OverallCorrectness: "x"}); got != "" {
		t.Fatalf("empty review id should yield no marker, got %q", got)
	}
	if got := FindingMarker("", model.Finding{ID: "f"}); got != "" {
		t.Fatalf("empty review id should yield no finding marker, got %q", got)
	}
}

func TestCollectFindingEnvelopesTolerant(t *testing.T) {
	valid := FindingMarker("r", model.Finding{ID: "ok"})
	body := FindingMarkerPrefix + "!!!not-base64!!! -->" +
		FindingMarkerPrefix + "Zm9v -->" + // valid base64 but not gzip
		valid
	got := CollectFindingEnvelopes(body)
	if len(got) != 1 || got[0].Finding.ID != "ok" {
		t.Fatalf("tolerant scan should recover only the valid carrier, got %+v", got)
	}
}

func TestCollectPriorFindingsTolerant(t *testing.T) {
	valid := FingerprintMarker(model.Finding{ID: "ok", CodeLocation: model.CodeLocation{FilePath: "x.go"}}, "t")
	body := FingerprintPrefix + "!!!not-base64!!! -->" + // invalid base64
		FingerprintPrefix + "Zm9v -->" + // base64 of "foo", not JSON
		valid // a well-formed marker still recovered after the bad ones
	var priors []model.Finding
	CollectPriorFindings(body, &priors)
	if len(priors) != 1 || priors[0].ID != "ok" {
		t.Fatalf("tolerant scan should recover only the valid fp, got %+v", priors)
	}
}

// priorsFrom builds a Priors as if the given finding had been posted with the
// given displayed title (the cross-run dedup input).
func priorsFrom(f model.Finding, displayTitle string) Priors {
	var p Priors
	ScanComment(FingerprintMarker(f, displayTitle), &p)
	return p
}

func TestAlreadyPostedSameRunID(t *testing.T) {
	prior := priorsFrom(model.Finding{ID: "same", CodeLocation: model.CodeLocation{FilePath: "a.go"}}, "Title A")
	// Different file and title, but the same id => exact same-run match.
	f := model.Finding{ID: "same", CodeLocation: model.CodeLocation{FilePath: "z.go"}}
	if !AlreadyPosted(f, "totally different", prior) {
		t.Fatal("same id should match exactly regardless of content")
	}
}

func TestAlreadyPostedHeuristicSkip(t *testing.T) {
	// Same file, titles that normalize to the same tokens but differ as raw
	// strings => fuzzy Duplicate via the title-strong rule (not the Identical case).
	priorFinding := model.Finding{
		ID:           "old",
		Body:         "prior generated explanation",
		CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 10, End: 11}},
	}
	prior := priorsFrom(priorFinding, "Null pointer dereference in the handler")
	if len(prior.Findings) != 1 || prior.Findings[0].Body != "" || prior.Findings[0].CodeLocation.LineRange != (model.LineRange{}) {
		t.Fatalf("prior fingerprint should carry only id/file/title, got %+v", prior.Findings)
	}
	f := model.Finding{
		ID:           "new",
		Body:         "different generated explanation",
		CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 200, End: 201}},
	}
	if !AlreadyPosted(f, "Null pointer dereference in handler", prior) {
		t.Fatal("same file + near-identical title should match across runs without location/body signal")
	}
}

func TestAlreadyPostedDistinctSameFile(t *testing.T) {
	prior := priorsFrom(model.Finding{ID: "old", CodeLocation: model.CodeLocation{FilePath: "a.go"}}, "Null pointer dereference in handler")
	f := model.Finding{ID: "new", CodeLocation: model.CodeLocation{FilePath: "a.go"}}
	if AlreadyPosted(f, "Race condition in cache eviction", prior) {
		t.Fatal("a clearly different title on the same file must not match")
	}
}

func TestAlreadyPostedCrossFileNeverSkips(t *testing.T) {
	prior := priorsFrom(model.Finding{ID: "old", CodeLocation: model.CodeLocation{FilePath: "a.go"}}, "Null pointer dereference in handler")
	f := model.Finding{ID: "new", CodeLocation: model.CodeLocation{FilePath: "b.go"}}
	if AlreadyPosted(f, "Null pointer dereference in handler", prior) {
		t.Fatal("identical title on a different file must not match (cross-file capped below Duplicate)")
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
