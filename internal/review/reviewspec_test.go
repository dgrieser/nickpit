package review

import (
	"context"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// reviewerOnlySpec is the reviewer stage of the embedded workflow without
// finalize/verdict/summarize: collect context, run the vector reviewers, verify,
// dedupe, merge. Tests use it to exercise the reviewer pipeline and assert the
// pre-finalize result shape without mocking finalize/summarize LLM calls.
// Production always runs the full workflow.DefaultSpec through the same path.
func reviewerOnlySpec() workflow.Spec {
	parallel := make([]workflow.StepEntry, len(workflow.ReviewVectorIDs))
	for i, id := range workflow.ReviewVectorIDs {
		parallel[i] = workflow.StepEntry{Type: workflow.StepReviewPrefix + id}
	}
	return workflow.Spec{
		Version: workflow.SpecVersion,
		Steps: []workflow.StepEntry{
			{Type: workflow.StepCollectContext},
			{Parallel: parallel},
			{Type: workflow.StepVerify},
			{Type: workflow.StepDedupe},
			{Type: workflow.StepMerge},
		},
	}
}

// runReviewPipeline runs reviewerOnlySpec through the single execution path,
// returning the same (result, enrichedContext, err) shape the removed
// RunWithContext did. Test-only convenience.
func runReviewPipeline(e *Engine, ctx context.Context, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	p, err := e.BuildPipeline(reviewerOnlySpec())
	if err != nil {
		return nil, nil, err
	}
	return e.RunSpecPipeline(ctx, p, req)
}
