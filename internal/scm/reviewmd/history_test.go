package reviewmd

import (
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestHistoryWithoutArchiveAllowsExactNoteBudget(t *testing.T) {
	current := strings.Repeat("x", NoteMaxBytes)
	got, err := WithHistory("", current, "Finding", time.Time{}, time.Now(), false)
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
	first, err := WithHistory(original, current, "Finding", time.Time{}, at, true)
	if err != nil {
		t.Fatal(err)
	}
	f.Revision = 2
	f.Title = "Latest title"
	f.Body = "Latest evidence"
	current, _ = render.FindingBodyCarried(f, "")
	second, err := WithHistory(first, current, "Finding", time.Time{}, at.Add(time.Minute), true)
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

// Each archived version is stamped with the time it was written, not the time
// a later version replaced it: the first with the note's creation, later ones
// with the time the edit that wrote them happened.
func TestHistoryStampsEntriesWithWrittenTime(t *testing.T) {
	posted := time.Date(2026, 9, 22, 14, 58, 0, 0, time.UTC)
	edited := time.Date(2026, 9, 23, 19, 27, 0, 0, time.UTC)
	rereviewed := time.Date(2026, 9, 29, 10, 46, 0, 0, time.UTC)
	first, err := WithHistory("original", "edited", "Review", posted, edited, true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := WithHistory(first, "rereviewed", "Review", posted, rereviewed, true)
	if err != nil {
		t.Fatal(err)
	}
	h := ReadHistory(second)
	if len(h.Entries) != 2 || !h.Entries[0].At.Equal(edited) || !h.Entries[1].At.Equal(posted) || !h.Since.Equal(rereviewed) {
		t.Fatalf("history = %+v", h)
	}
	if !strings.Contains(second, "**2026-09-23T19:27:00Z**\n\nedited") || !strings.Contains(second, "**2026-09-22T14:58:00Z**\n\noriginal") {
		t.Fatalf("rendered stamps do not match the archived versions: %s", second)
	}
	// Without a creation time the first version falls back to its replacement.
	fallback, err := WithHistory("original", "edited", "Review", time.Time{}, edited, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := ReadHistory(fallback).Entries[0].At; !got.Equal(edited) {
		t.Fatalf("fallback stamp = %s", got)
	}
}

// An archive from before Since existed carries replacement times; the next
// write shifts them onto the versions they belong to.
func TestHistoryRestampsLegacyArchive(t *testing.T) {
	posted := time.Date(2026, 9, 22, 14, 58, 0, 0, time.UTC)
	firstEdit := time.Date(2026, 9, 23, 19, 26, 0, 0, time.UTC)
	secondEdit := time.Date(2026, 9, 23, 19, 27, 0, 0, time.UTC)
	now := time.Date(2026, 9, 29, 10, 46, 0, 0, time.UTC)
	legacy := CommentHistory{Entries: []HistoryEntry{{At: secondEdit, Body: "first edit"}, {At: firstEdit, Body: "original"}}}
	marker, _ := encodeMarker(historyPrefix, legacy)
	previous := historyStart + "\n" + marker + "\n" + historyEnd + "\n\nsecond edit"
	body, err := WithHistory(previous, "rereviewed", "Review", posted, now, true)
	if err != nil {
		t.Fatal(err)
	}
	h := ReadHistory(body)
	want := []time.Time{secondEdit, firstEdit, posted}
	if len(h.Entries) != len(want) || !h.Since.Equal(now) {
		t.Fatalf("history = %+v", h)
	}
	for i, at := range want {
		if !h.Entries[i].At.Equal(at) {
			t.Fatalf("entry %d (%q) stamped %s, want %s", i, h.Entries[i].Body, h.Entries[i].At, at)
		}
	}
}

func TestHistoryDropsOldestAndKeepsNotice(t *testing.T) {
	previous := strings.Repeat("old evidence ", 5000)
	current := "current evidence"
	body, err := WithHistory(previous, current, "Finding", time.Time{}, time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > NoteMaxBytes || !ReadHistory(body).Omitted || !strings.Contains(body, HistoryOmittedNotice) || StripMarkers(body) != current {
		t.Fatal("history not bounded or notice missing")
	}
	next, err := WithHistory(body, "next evidence", "Finding", time.Time{}, time.Now(), true)
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
