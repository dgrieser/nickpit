package gitlab

import (
	"context"
	"errors"
	"testing"

	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

func TestUpdateRecoveryAfterUncertainActivationOrCheckpointFailure(t *testing.T) {
	for _, scenario := range []string{"lost activation response", "failed local checkpoint"} {
		t.Run(scenario, func(t *testing.T) {
			s, a, before := newUpdateServer(t)
			after, err := before.Clone()
			if err != nil {
				t.Fatal(err)
			}
			after.Findings[0].Title = "Corrected"
			req := updateRequest(before, after)
			req.Operation = "uncertain-operation"
			staged := false
			if scenario == "lost activation response" {
				s.loseActivationResponse = true
			}
			req.OnStaged = func() error {
				staged = true
				if scenario == "failed local checkpoint" {
					return errors.New("checkpoint failed")
				}
				return nil
			}
			if _, err := a.UpdateReview(context.Background(), "group/project", 456, req); err == nil {
				t.Fatal("expected uncertain publication")
			}
			if staged != (scenario == "failed local checkpoint") {
				t.Fatalf("staged=%v", staged)
			}
			if s.visibleWrites != 0 {
				t.Fatal("visible writes before durable checkpoint")
			}
			if err := a.RecoverReviewUpdates(context.Background(), "group/project", 456); err != nil {
				t.Fatal(err)
			}
			committed, err := a.ReviewUpdateCommitted(context.Background(), "group/project", 456, before.ReviewID, req.Operation)
			if err != nil || !committed {
				t.Fatalf("lost operation: %v %v", committed, err)
			}
			current := reviewmd.ReviewResultsByID(ownedBodies(s.snapshot(), 7))[before.ReviewID]
			if current == nil || current.Findings[0].Title != "Corrected" {
				t.Fatal("recovery lost correction")
			}
		})
	}
}
