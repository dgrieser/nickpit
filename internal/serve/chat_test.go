package serve

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

func TestDetectThreadReview(t *testing.T) {
	render := reviewmd.NewRenderer("https://host/").ForReview("rev-9")
	findingNote := render.FindingBody(model.Finding{ID: "f7", Title: "Bug"}, "")
	summaryNote := render.SummaryBody(&model.ReviewResult{ReviewID: "rev-9", OverallCorrectness: "patch is incorrect"})

	rid, fid, ok := detectThreadReview(findingNote)
	if !ok || rid != "rev-9" || fid != "f7" {
		t.Fatalf("finding thread: rid=%q fid=%q ok=%v", rid, fid, ok)
	}
	rid, fid, ok = detectThreadReview(summaryNote)
	if !ok || rid != "rev-9" || fid != "" {
		t.Fatalf("summary thread: rid=%q fid=%q ok=%v", rid, fid, ok)
	}
	if _, _, ok := detectThreadReview("just a normal comment"); ok {
		t.Fatalf("non-nickpit note should not be detected as a thread")
	}
}

func TestChatThreadToMessages(t *testing.T) {
	notes := []gitlab.DiscussionNote{
		{Body: "root finding note", AuthorID: 5},     // root — skipped
		{Body: "why is this a bug?", AuthorID: 10},   // user
		{Body: "because X", AuthorID: 5},             // bot -> assistant
		{Body: "system joined", System: true},        // skipped
		{Body: "ok but what about Y?", AuthorID: 10}, // user
	}
	msgs := chatThreadToMessages(notes, 5)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "why is this a bug?" {
		t.Fatalf("msg0 = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "because X" {
		t.Fatalf("msg1 = %+v", msgs[1])
	}
	if msgs[2].Role != "user" {
		t.Fatalf("msg2 = %+v", msgs[2])
	}
}
