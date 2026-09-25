package gitlab

import (
	"context"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

func reconcileFinding(id, title string) model.Finding {
	p := 2
	return model.Finding{ID: id, Title: title, Body: title + " body.", Priority: &p, ConfidenceScore: 0.8,
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 2, End: 2}, Content: "new()"}}
}

func TestPublishedReviewReadsNewestSummaryAndForeignFindings(t *testing.T) {
	s, a, before := newUpdateServer(t)
	legacy := reviewmd.NewRenderer("").ForReview("legacy-run")
	body, _ := legacy.FindingBodyCarried(reconcileFinding("legacy-finding", "Legacy thread"), "")
	s.discussions = append(s.discussions, MRDiscussion{ID: "legacy", Notes: []DiscussionNote{{ID: 3, Body: body, AuthorID: 7}}})

	got, err := a.PublishedReview(context.Background(), model.ReviewRequest{Repo: "group/project", Identifier: 456})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Review == nil || got.Review.ReviewID != before.ReviewID || len(got.Review.Findings) != 1 {
		t.Fatalf("published review = %+v", got)
	}
	if len(got.Foreign) != 1 || got.Foreign[0].Title != "Legacy thread" {
		t.Fatalf("foreign findings = %+v", got.Foreign)
	}
}

func TestPublishedReviewNilWithoutSummary(t *testing.T) {
	s, a, _ := newUpdateServer(t)
	s.discussions = s.discussions[1:]
	got, err := a.PublishedReview(context.Background(), model.ReviewRequest{Repo: "group/project", Identifier: 456})
	if err != nil || got != nil {
		t.Fatalf("PublishedReview = %+v, %v; want nil", got, err)
	}
}

func TestPublishReconciledUpdatesReviewInPlace(t *testing.T) {
	s, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.Findings = append(after.Findings, reconcileFinding("added", "New nil dereference"))
	after.OverallExplanation = "The re-review found one more problem."
	after.Reconciliation = &model.Reconciliation{Before: before, HeadSHA: "0123456789abcdef", Added: []string{"added"}}

	if err := a.PublishReview(context.Background(), model.ReviewRequest{Repo: "group/project", Identifier: 456}, after); err != nil {
		t.Fatal(err)
	}
	discussions := s.snapshot()
	got := reviewmd.ReviewResultsByID(ownedBodies(discussions, 7))[before.ReviewID]
	if got == nil || got.Revision != 1 || len(got.Findings) != 2 || got.OverallExplanation != after.OverallExplanation {
		t.Fatalf("reassembled review = %+v", got)
	}
	if len(reviewmd.ReviewResultsByID(ownedBodies(discussions, 7))) != 1 {
		t.Fatal("re-review created a second review")
	}
	root := findUpdateTarget(discussions, 7, before.ReviewID, "")
	if len(reviewmd.ReadHistory(root.Body).Entries) != 1 {
		t.Fatal("summary did not archive the previous verdict")
	}
	if kept := findUpdateTarget(discussions, 7, before.ReviewID, "finding"); len(reviewmd.ReadHistory(kept.Body).Entries) != 0 {
		t.Fatal("unchanged published finding was rewritten")
	}
	if findUpdateTarget(discussions, 7, before.ReviewID, "added").NoteID == 0 {
		t.Fatal("added finding has no thread of its own")
	}
	var reply string
	for _, d := range discussions {
		if d.ID == root.DiscussionID && len(d.Notes) > 1 {
			reply = d.Notes[len(d.Notes)-1].Body
		}
	}
	if !strings.Contains(reply, "Review updated for `01234567`") || !strings.Contains(reply, "New nil dereference") {
		t.Fatalf("summary reply = %q", reply)
	}
	for _, d := range discussions {
		if reviewmd.StripMarkers(d.Notes[0].Body) == "" {
			t.Fatalf("marker-only note left behind: %q", d.Notes[0].Body)
		}
	}
	for _, body := range s.internalBodies {
		if !strings.HasPrefix(body, reviewmd.CarrierNotice) {
			t.Fatalf("internal note lacks notice: %q", body)
		}
	}
}

func TestPublishReconciledReplaysOntoConcurrentCorrection(t *testing.T) {
	s, a, before := newUpdateServer(t)
	// A chat correction lands while the re-review runs.
	corrected, _ := before.Clone()
	corrected.Findings[0].Title = "Corrected by chat"
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(before, corrected)); err != nil {
		t.Fatal(err)
	}
	after, _ := before.Clone()
	after.Findings = append(after.Findings, reconcileFinding("added", "Second problem"))
	after.Reconciliation = &model.Reconciliation{Before: before, HeadSHA: "headsha", Added: []string{"added"}}
	if err := a.PublishReview(context.Background(), model.ReviewRequest{Repo: "group/project", Identifier: 456}, after); err != nil {
		t.Fatal(err)
	}
	got := reviewmd.ReviewResultsByID(ownedBodies(s.snapshot(), 7))[before.ReviewID]
	if got == nil || len(got.Findings) != 2 {
		t.Fatalf("reassembled review = %+v", got)
	}
	titles := map[string]string{}
	for _, f := range got.Findings {
		titles[f.ID] = f.Title
	}
	if titles["finding"] != "Corrected by chat" || titles["added"] != "Second problem" {
		t.Fatalf("replay lost a change: %v", titles)
	}
}

func TestUpdateReviewAdditionsCannotRemoveFindings(t *testing.T) {
	_, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.Findings = []model.Finding{reconcileFinding("added", "Replacement")}
	req := updateRequest(before, after)
	req.AllowAdditions = true
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, req); err == nil || !strings.Contains(err.Error(), "cannot remove") {
		t.Fatalf("err = %v, want removal rejection", err)
	}
}

func TestUpdateReviewRejectsAdditionsByDefault(t *testing.T) {
	_, a, before := newUpdateServer(t)
	after, _ := before.Clone()
	after.Findings = append(after.Findings, reconcileFinding("added", "Extra"))
	if _, err := a.UpdateReview(context.Background(), "group/project", 456, updateRequest(before, after)); err == nil {
		t.Fatal("chat correction added a finding")
	}
}

func TestRebaseReconciledKeepsPublishedCorrections(t *testing.T) {
	before := &model.ReviewResult{ReviewID: "r", Findings: []model.Finding{reconcileFinding("a", "A"), reconcileFinding("b", "B")}}
	current, _ := before.Clone()
	current.Findings[0].Title = "A corrected"
	after, _ := before.Clone()
	after.Findings[0].Title = "A from re-review"
	after.Findings[1].Resolution = &model.FindingResolution{Reason: "Fixed."}
	after.Findings = append(after.Findings, reconcileFinding("c", "C"))
	after.OverallExplanation = "re-review"

	got, err := rebaseReconciled(before, after, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 3 || got.Findings[0].Title != "A corrected" || got.Findings[1].Resolution == nil || got.Findings[2].ID != "c" || got.OverallExplanation != "re-review" {
		t.Fatalf("rebased = %+v", got.Findings)
	}
}

func TestReconciledReplyBodyWithoutChanges(t *testing.T) {
	body := reconciledReplyBody(&model.ReviewResult{}, &model.Reconciliation{HeadSHA: "abc"})
	if body != "Review updated for `abc`. No new findings; the existing findings are unchanged." {
		t.Fatalf("body = %q", body)
	}
}
