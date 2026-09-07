package reviewmd

import (
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestHistoryWithoutArchiveAllowsExactNoteBudget(t *testing.T) {
	current := strings.Repeat("x", NoteMaxBytes)
	got, err := WithHistory("", current, "Finding", time.Now(), false)
	if err != nil || got != current {
		t.Fatalf("exact-size current comment rejected: %v", err)
	}
}

func TestHistoryFlatAndExcludedFromReassembly(t *testing.T) {
	p := 1
	f := model.Finding{ID: "finding", Title: "Old title", Body: "Old evidence", Priority: &p}
	r := &model.ReviewResult{ReviewID: "review", Findings: []model.Finding{f}, OverallCorrectness: "patch is incorrect", OverallExplanation: "Original verdict"}
	render := NewRenderer("").ForReview(r.ReviewID)
	original, _ := render.FindingBodyCarried(f, "")
	original += "\n\n<details>\n<summary>Suggestions</summary>\n\nNested old suggestion\n\n</details>"
	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	f.Revision = 1
	f.Title = "New title"
	f.Body = "New evidence"
	current, _ := render.FindingBodyCarried(f, "")
	first, err := WithHistory(original, current, "Finding", at, true)
	if err != nil {
		t.Fatal(err)
	}
	f.Revision = 2
	f.Title = "Latest title"
	f.Body = "Latest evidence"
	current, _ = render.FindingBodyCarried(f, "")
	second, err := WithHistory(first, current, "Finding", at.Add(time.Minute), true)
	if err != nil {
		t.Fatal(err)
	}
	h := ReadHistory(second)
	if len(h.Entries) != 2 || strings.Contains(h.Entries[0].Body, historyStart) || h.Entries[1].Body != original {
		t.Fatalf("history not flat: %#v", h)
	}
	visible := StripMarkers(second)
	if strings.Contains(visible, "Old") || strings.Contains(visible, "Nested") || !strings.Contains(visible, "Latest title") {
		t.Fatalf("history leaked: %s", visible)
	}
	if rid, fid, ok := DetectThreadReview(second); !ok || rid != "review" || fid != "finding" {
		t.Fatalf("lost route: %q %q", rid, fid)
	}
	r.Revision = 2
	r.Findings = []model.Finding{f}
	root, _ := render.SummaryBodyCarried(r)
	for _, bodies := range [][]string{{root, original, second}, {second, original, root}} {
		got := ReviewResultsByID(bodies)["review"]
		if got == nil || got.Revision != 2 || len(got.Findings) != 1 || got.Findings[0].Title != "Latest title" {
			t.Fatalf("stale carrier won: %+v", got)
		}
	}
}

func TestHistoryDropsOldestAndKeepsNotice(t *testing.T) {
	previous := strings.Repeat("old evidence ", 5000)
	current := "current evidence"
	body, err := WithHistory(previous, current, "Finding", time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > NoteMaxBytes || !ReadHistory(body).Omitted || !strings.Contains(body, HistoryOmittedNotice) || StripMarkers(body) != current {
		t.Fatal("history not bounded or notice missing")
	}
	next, err := WithHistory(body, "next evidence", "Finding", time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !ReadHistory(next).Omitted || len(ReadHistory(next).Entries) != 1 {
		t.Fatal("omission state lost")
	}
}

func TestResolvedRenderingAndFooter(t *testing.T) {
	f := model.Finding{ID: "f", Title: "Old title", Body: "Obsolete detail", Resolution: &model.FindingResolution{Reason: "The new guard prevents a nil dereference."}}
	render := NewRenderer("https://assets.example/").ForReview("r")
	body, carried := render.FindingBodyCarried(f, "file:2")
	status := ResponseStatus{Enabled: true, CommandMuted: true, CommandKeyword: "nickpit"}
	body = UpsertResponseFooter(body, status)
	visible := StripMarkers(body)
	if !carried || visible != "![RESOLVED](https://assets.example/resolved.svg)\n\n"+f.Resolution.Reason || !ThreadCommandMuted(body) {
		t.Fatalf("resolved output: %s", body)
	}
	var priors Priors
	priors.Markers = map[string]struct{}{}
	ScanComment(body, &priors)
	if len(priors.Findings) > 0 {
		t.Fatal("resolved finding still suppresses later reviews")
	}
}

func TestPendingUpdateRejectsPartialReview(t *testing.T) {
	r := &model.ReviewResult{ReviewID: "r", OverallExplanation: "before"}
	render := NewRenderer("").ForReview("r")
	root, _ := render.SummaryBodyCarried(r)
	marker, err := UpdateRecordMarker(UpdateRecord{ReviewID: "r", Operation: "op", Revision: 1, Parts: 1, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := ReviewResultsByID([]string{root, marker}); got["r"] != nil {
		t.Fatal("pending review exposed")
	}
	r.Revision = 1
	r.OverallExplanation = "after"
	committed, _ := render.SummaryBodyCarried(r)
	if got := ReviewResultsByID([]string{root, marker, committed})["r"]; got == nil || got.OverallExplanation != "after" {
		t.Fatal("committed revision missing")
	}
}
