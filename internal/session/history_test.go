package session

import (
	"reflect"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestReviewHistoryRoundTripAndIsolation(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New()
	s.Result = &model.ReviewResult{ReviewID: "review", Findings: []model.Finding{{ID: "finding", Body: "old"}}}
	original := s.Result
	next, _ := original.Clone()
	next.Revision = 1
	next.Findings[0].Body = "new"
	s.Append(UserMessage("Question"))
	if err := s.RecordReviewUpdate(next, "Correction evidence"); err != nil {
		t.Fatal(err)
	}
	s.Append(UserMessage("Later conversation"))
	original.Findings[0].Body = "mutated original"
	next.Findings[0].Body = "mutated worker"
	if s.Result.Findings[0].Body != "new" || s.ReviewHistory[0].Result.Findings[0].Body != "old" {
		t.Fatal("history shares mutable data")
	}
	if err := s.RecordReviewUpdate(s.Result, "No change"); err != nil {
		t.Fatal(err)
	}
	if len(s.ReviewHistory) != 1 || len(s.Messages) != 2 {
		t.Fatal("no-op archived or messages lost")
	}
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.ReviewHistory, s.ReviewHistory) || !reflect.DeepEqual(restored.Messages, s.Messages) {
		t.Fatal("history or messages lost on resume")
	}
	if restored.Version != Version {
		t.Fatal("unexpected schema migration")
	}
	if err := s.RecordReviewUpdate(&model.ReviewResult{ReviewID: "other"}, "Wrong review"); err == nil {
		t.Fatal("unrelated review accepted")
	}
}

func TestHistorySavePreservesSessionConflictDetection(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	s := New()
	s.Result = &model.ReviewResult{ReviewID: "review"}
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}
	first, _ := store.Load(s.ID)
	second, _ := store.Load(s.ID)
	first.Append(UserMessage("Concurrent turn"))
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	if err := second.RecordReviewUpdate(&model.ReviewResult{ReviewID: "review", Revision: 1}, "Correction"); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(second); err == nil {
		t.Fatal("update overwrote another process's conversation")
	}
	loaded, _ := store.Load(s.ID)
	if len(loaded.Messages) != 1 || len(loaded.ReviewHistory) != 0 {
		t.Fatal("conflicting save changed stored session")
	}
}
