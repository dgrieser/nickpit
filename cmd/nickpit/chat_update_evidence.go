package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dgrieser/nickpit/internal/llm"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/serve"
)

type updateEvidenceNote struct {
	ID     int
	Anchor int
	Author int
	Role   string
	Body   string
}

type updateEvidenceSnapshot struct {
	Messages    []llm.Message
	Fingerprint string
	Question    string
}

func loadUpdateEvidence(ctx context.Context, client *glscm.Client, job *serve.UpdateJob, bot int, controls chatMessageControls) (*updateEvidenceSnapshot, error) {
	discussions, err := client.MRDiscussions(ctx, job.ProjectPath, job.IID)
	if err != nil {
		return nil, err
	}
	fallback, err := client.MRNotes(ctx, job.ProjectPath, job.IID)
	if err != nil {
		return nil, err
	}
	var trigger []glscm.DiscussionNote
	for i := range discussions {
		d := &discussions[i]
		d.Notes = mergeFallbackReplies(d.Notes, fallback, d.ID, bot)
		if d.ID == job.DiscussionID {
			trigger = d.Notes
		}
	}
	if len(trigger) == 0 || trigger[0].AuthorID != bot {
		return nil, errChatReplySuppressed
	}
	rid, _, ok := reviewmd.DetectThreadReview(trigger[0].Body)
	if !ok || rid != job.ReviewID {
		return nil, errChatReplySuppressed
	}
	found := false
	for _, n := range trigger {
		if n.ID != job.NoteID {
			continue
		}
		found = true
		if n.Body != job.Question {
			return nil, fmt.Errorf("original question was edited")
		}
		if !chatNoteDirectives(trigger, job.NoteID, controls).AllowsReply(job.Requested) {
			return nil, errChatReplySuppressed
		}
	}
	if !found {
		return nil, errChatReplySuppressed
	}
	snapshot := collectUpdateEvidence(discussions, trigger, job.ReviewID, job.FindingIDs, job.NoteID, bot, controls)
	snapshot.Question = stripChatMessageControls(job.Question, controls)
	return snapshot, nil
}

// The same selected, normalized notes determine both prompt input and freshness.
func collectUpdateEvidence(discussions []glscm.MRDiscussion, trigger []glscm.DiscussionNote, rid string, findingIDs []string, cutoff, bot int, controls chatMessageControls) *updateEvidenceSnapshot {
	wanted := map[string]bool{}
	for _, id := range findingIDs {
		wanted[id] = true
	}
	var selected []updateEvidenceNote
	seen := map[int]bool{}
	add := func(notes []glscm.DiscussionNote) {
		anchor := 0
		for _, n := range notes {
			if n.ID > 0 {
				anchor = n.ID
			}
			id, order := n.ID, n.ID
			if n.ID == 0 {
				id = n.FallbackNoteID
				order = n.AnsweredNoteID
				if order == 0 {
					order = anchor
				}
				// Answers to the queued question belong to its subsequent conversation.
				if order <= 0 || order >= cutoff {
					continue
				}
			}
			if id > cutoff || order > cutoff || n.System {
				continue
			}
			body := stripChatMessageControls(n.Body, controls)
			if body == "" || (id > 0 && seen[id]) {
				continue
			}
			if id > 0 {
				seen[id] = true
			}
			role := "user"
			if bot != 0 && n.AuthorID == bot {
				role = "assistant"
			}
			selected = append(selected, updateEvidenceNote{ID: id, Anchor: order, Author: n.AuthorID, Role: role, Body: body})
		}
	}
	for _, d := range discussions {
		if len(d.Notes) == 0 || d.Notes[0].AuthorID != bot || reviewmd.StripMarkers(d.Notes[0].Body) == "" {
			continue
		}
		r, f, ok := reviewmd.DetectThreadReview(d.Notes[0].Body)
		if ok && r == rid && wanted[f] {
			add(d.Notes[1:])
		}
	}
	if len(trigger) > 0 {
		add(trigger[1:])
	}
	sort.SliceStable(selected, func(i, j int) bool {
		a, b := selected[i], selected[j]
		if a.Anchor != b.Anchor {
			return a.Anchor < b.Anchor
		}
		ar, br := a.ID == a.Anchor, b.ID == b.Anchor
		if ar != br {
			return ar
		}
		return a.ID < b.ID
	})
	raw, _ := json.Marshal(selected)
	out := &updateEvidenceSnapshot{Fingerprint: fmt.Sprintf("v2:%x", sha256.Sum256(raw))}
	for _, n := range selected {
		out.Messages = append(out.Messages, llm.Message{Role: n.Role, Content: n.Body})
	}
	return out
}
