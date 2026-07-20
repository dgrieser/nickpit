package reviewmd

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

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

func TestCollectorsBoundAggregateDecoding(t *testing.T) {
	// Many valid envelopes that each decode fine must still be bounded in
	// aggregate: a single body cannot force unbounded total work or output.
	one := FindingMarker("r", model.Finding{ID: "f", Body: strings.Repeat("x", 1024)})
	body := strings.Repeat(one+"\n", maxCarriersPerBody+50)
	got := CollectFindingEnvelopes(body)
	if len(got) == 0 || len(got) > maxCarriersPerBody {
		t.Fatalf("collected %d envelopes, want >0 and <= %d", len(got), maxCarriersPerBody)
	}
}

func TestDetectThreadReviewStopsAtFirstMarker(t *testing.T) {
	// The gate must short-circuit on the first valid marker; trailing garbage
	// markers must not change the result.
	body := FindingMarker("rev-1", model.Finding{ID: "f1"}) + "\n" +
		FindingMarkerPrefix + "!!!garbage -->" + "\n" +
		FindingMarker("rev-2", model.Finding{ID: "f2"})
	rid, fid, ok := DetectThreadReview(body)
	if !ok || rid != "rev-1" || fid != "f1" {
		t.Fatalf("DetectThreadReview = %q %q %v, want first marker rev-1/f1", rid, fid, ok)
	}
}

func TestStripMarkers(t *testing.T) {
	finding := model.Finding{ID: "f1", Title: "T"}
	body := "visible intro\n" + FingerprintMarker(finding, "T") + "\n" +
		FindingMarker("r", finding) + "\nvisible tail"
	got := StripMarkers(body)
	if strings.Contains(got, MarkerOpen) {
		t.Fatalf("markers not stripped: %q", got)
	}
	if !strings.Contains(got, "visible intro") || !strings.Contains(got, "visible tail") {
		t.Fatalf("visible text lost: %q", got)
	}
	// A carrier-only body strips to empty so the comment can be dropped.
	if got := StripMarkers(FindingMarker("r", finding)); got != "" {
		t.Fatalf("carrier-only body should strip to empty, got %q", got)
	}
	// Text without markers passes through untouched.
	if got := StripMarkers("plain comment"); got != "plain comment" {
		t.Fatalf("plain text mangled: %q", got)
	}
}

func TestCarrierNotesReassemble(t *testing.T) {
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
	notes := NewRenderer("https://host/").CarrierNotes(result, result.Findings)
	if len(notes) == 0 {
		t.Fatal("carrier notes empty")
	}
	byID := ReviewResultsByID(notes)
	got := byID["rev-c"]
	if got == nil {
		t.Fatalf("carrier notes did not reassemble: %v", byID)
	}
	if got.OverallExplanation != "carrier note test" || len(got.Findings) != 2 {
		t.Fatalf("carrier reassembly incomplete: %+v", got)
	}
	// Only the passed findings ride in the carrier — the review envelope alone
	// when none are missing.
	envelopeOnly := NewRenderer("https://host/").CarrierNotes(result, nil)
	if len(envelopeOnly) != 1 || len(CollectFindingEnvelopes(envelopeOnly[0])) != 0 {
		t.Fatalf("envelope-only carrier wrong: %v", envelopeOnly)
	}
	if NewRenderer("").CarrierNotes(&model.ReviewResult{}, nil) != nil {
		t.Fatalf("carrier notes should be nil without a review id")
	}
}

func TestFindingBodyOmitsCarrierWhenOversized(t *testing.T) {
	render := NewRenderer("https://host/").ForReview("rev-big")
	// Incompressible body so the carrier marker alone would exceed the platform
	// comment limit.
	rng := rand.New(rand.NewSource(7))
	raw := make([]byte, 80*1024)
	rng.Read(raw)
	huge := model.Finding{ID: "f-huge", Title: "Huge", Body: base64.StdEncoding.EncodeToString(raw)}

	body, carried := render.FindingBodyCarried(huge, "")
	if carried {
		t.Fatal("oversized finding must not carry the full marker inline")
	}
	// The visible publication must survive: fingerprint, title, and body intact,
	// with the full payload replaced by a tiny routing-only reference.
	if !strings.HasPrefix(body, FingerprintPrefix) || !strings.Contains(body, "### Huge") {
		t.Fatalf("visible body degraded: %.120q", body)
	}
	envs := CollectFindingEnvelopes(body)
	if len(envs) != 1 || !envs[0].Ref || envs[0].ReviewID != "rev-big" || envs[0].Finding.ID != "f-huge" {
		t.Fatalf("expected one routing ref envelope, got %+v", envs)
	}
	if envs[0].Finding.Body != "" {
		t.Fatal("routing ref must not carry the finding payload")
	}
	// Replies beneath the visible comment must still route to the discussion
	// agent via the ref, even though the full payload is externalized.
	rid, fid, ok := DetectThreadReview(body)
	if !ok || rid != "rev-big" || fid != "f-huge" {
		t.Fatalf("thread routing lost: rid=%q fid=%q ok=%v", rid, fid, ok)
	}
	// Reassembly must not be shadowed by the stub: the full finding from the
	// carrier note wins regardless of body order.
	carrierNotes := render.CarrierNotes(&model.ReviewResult{ReviewID: "rev-big", Findings: []model.Finding{huge}}, []model.Finding{huge})
	got := ReviewResultsByID(append([]string{body}, carrierNotes...))["rev-big"]
	if got == nil || len(got.Findings) != 1 || got.Findings[0].Body != huge.Body {
		t.Fatalf("stub shadowed the full finding: %+v", got)
	}
	// A small finding still carries inline (a full, non-ref envelope).
	small := model.Finding{ID: "f-small", Title: "Small", Body: "tiny"}
	sbody, scarried := render.FindingBodyCarried(small, "")
	senvs := CollectFindingEnvelopes(sbody)
	if !scarried || len(senvs) != 1 || senvs[0].Ref {
		t.Fatalf("small finding should carry the full envelope inline (carried=%v, envs=%+v)", scarried, senvs)
	}
}

func TestSummaryBodyOmitsCarrierWhenOversized(t *testing.T) {
	// A near-limit overall explanation must still publish visibly; the review
	// envelope is externalized instead of pushing the comment past the platform
	// cap.
	rng := rand.New(rand.NewSource(11))
	raw := make([]byte, 44*1024)
	rng.Read(raw)
	result := &model.ReviewResult{
		ReviewID:           "rev-sum",
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: base64.StdEncoding.EncodeToString(raw), // ~59KB visible, incompressible
	}
	body, carried := NewRenderer("https://host/").SummaryBodyCarried(result)
	if carried {
		t.Fatal("oversized summary must not embed the review envelope")
	}
	// The full envelope is externalized, but a tiny routing-only ref must remain
	// so replies under the summary still pass the daemon's thread gate.
	envs := CollectReviewEnvelopes(body)
	if !strings.HasPrefix(body, SummaryMarker) || len(envs) != 1 || !envs[0].Ref || envs[0].OverallExplanation != "" {
		t.Fatalf("visible summary should carry exactly one routing ref: %+v", envs)
	}
	if rid, findingID, ok := DetectThreadReview(body); !ok || rid != "rev-sum" || findingID != "" {
		t.Fatalf("oversized summary must still route its thread: rid=%q finding=%q ok=%v", rid, findingID, ok)
	}
	if len(body) > carrierNoteMaxBytes+1024 {
		t.Fatalf("summary body unexpectedly large: %d bytes", len(body))
	}
	// Reassembly over the visible summary plus the externalized carrier notes
	// yields the full envelope — the ref never blanks it, in either order.
	carrier := NewRenderer("https://host/").CarrierNotes(result, nil)
	if len(carrier) == 0 {
		t.Fatal("expected externalized carrier notes")
	}
	for i, bodies := range [][]string{append([]string{body}, carrier...), append(append([]string{}, carrier...), body)} {
		got := ReviewResultsByID(bodies)["rev-sum"]
		if got == nil || got.OverallExplanation != result.OverallExplanation {
			t.Fatalf("ref blanked the full envelope (order %d)", i)
		}
	}
	// A short summary carries the envelope inline.
	small := &model.ReviewResult{ReviewID: "rev-sum", OverallCorrectness: "patch is correct", OverallExplanation: "fine"}
	sbody, scarried := NewRenderer("https://host/").SummaryBodyCarried(small)
	if !scarried || len(CollectReviewEnvelopes(sbody)) != 1 {
		t.Fatalf("short summary should carry the envelope inline (carried=%v)", scarried)
	}
}

func TestUniqueFindingsByID(t *testing.T) {
	in := []model.Finding{{ID: "a"}, {ID: "b"}, {ID: "a"}, {ID: ""}, {ID: ""}}
	got := UniqueFindingsByID(in)
	if len(got) != 4 || got[0].ID != "a" || got[1].ID != "b" || got[2].ID != "" || got[3].ID != "" {
		t.Fatalf("dedupe wrong: %+v", got)
	}
}

func TestCarrierNotesChunkOnDecodedBudget(t *testing.T) {
	// Highly compressible findings stay tiny encoded but expand hugely decoded;
	// packing them into one note under the encoded bound alone would blow the
	// reader's per-body decompression budget, silently dropping the tail. The
	// chunker must split on decoded size so everything reassembles.
	result := &model.ReviewResult{ReviewID: "rev-zip", OverallCorrectness: "patch is incorrect"}
	for i := range 10 {
		result.Findings = append(result.Findings, model.Finding{
			ID:   fmt.Sprintf("z-%02d", i),
			Body: strings.Repeat("compressible ", 150_000), // ~2MB decoded, ~KBs encoded
		})
	}
	notes := NewRenderer("https://host/").CarrierNotes(result, result.Findings)
	if len(notes) < 2 {
		t.Fatalf("expected decoded-budget chunking, got %d note(s)", len(notes))
	}
	got := ReviewResultsByID(notes)["rev-zip"]
	if got == nil || len(got.Findings) != len(result.Findings) {
		t.Fatalf("decoded-budget reassembly incomplete: got %d findings, want %d", len(got.Findings), len(result.Findings))
	}
}

func TestCarrierNotesChunkLargeReviews(t *testing.T) {
	// Many sizable findings must split into multiple bounded chunks — one giant
	// note would exceed SCM comment limits and the reader's per-body budget —
	// while still reassembling completely.
	result := &model.ReviewResult{ReviewID: "rev-big", OverallCorrectness: "patch is incorrect"}
	// Incompressible bodies (seeded PRNG) so gzip cannot collapse them and the
	// per-note byte bound actually forces chunking.
	rng := rand.New(rand.NewSource(42))
	for i := range 40 {
		raw := make([]byte, 8*1024)
		rng.Read(raw)
		result.Findings = append(result.Findings, model.Finding{
			ID:   fmt.Sprintf("f-%03d", i),
			Body: base64.StdEncoding.EncodeToString(raw),
		})
	}
	notes := NewRenderer("https://host/").CarrierNotes(result, result.Findings)
	if len(notes) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(notes))
	}
	for i, note := range notes {
		if len(note) > carrierNoteMaxBytes+maxCarrierDecodedBytes/8 { // generous sanity bound
			t.Fatalf("chunk %d oversized: %d bytes", i, len(note))
		}
	}
	got := ReviewResultsByID(notes)["rev-big"]
	if got == nil || len(got.Findings) != len(result.Findings) {
		t.Fatalf("chunked reassembly incomplete: got %d findings, want %d", len(got.Findings), len(result.Findings))
	}
}

func TestReviewResultsByIDRoundTrip(t *testing.T) {
	r := NewRenderer("https://host/").ForReview("rev-abc")
	result := &model.ReviewResult{
		ReviewID:               "rev-abc",
		CreatedAt:              time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
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
		got.Model != result.Model {
		t.Fatalf("overall/meta mismatch: %+v", got)
	}
	// The LLM endpoint must never ride in a published carrier: it can name a
	// private host or carry credentials, readable by anyone viewing the MR.
	if got.BaseURL != "" {
		t.Fatalf("LLM endpoint leaked into carrier: %q", got.BaseURL)
	}
	if !got.CreatedAt.Equal(result.CreatedAt) {
		t.Fatalf("created_at not round-tripped: got %v want %v", got.CreatedAt, result.CreatedAt)
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
