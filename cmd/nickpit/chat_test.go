package main

import (
	"testing"

	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

func TestChatThreadToMessages(t *testing.T) {
	notes := []glscm.DiscussionNote{
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

func TestIsLatestReply(t *testing.T) {
	notes := []glscm.DiscussionNote{
		{ID: 1, Body: "root", AuthorID: 5},
		{ID: 2, Body: "q1", AuthorID: 10},
		{ID: 3, Body: "a1", AuthorID: 5},   // bot reply, ignored
		{ID: 4, Body: "q2", AuthorID: 10},  // latest user note
		{ID: 5, Body: "sys", System: true}, // ignored
	}
	if !isLatestReply(notes, 5, 4) {
		t.Fatal("note 4 should be the latest reply")
	}
	if isLatestReply(notes, 5, 2) {
		t.Fatal("note 2 is superseded by note 4")
	}
	if isLatestReply(notes, 5, 999) {
		t.Fatal("unknown note is not the latest")
	}
}
