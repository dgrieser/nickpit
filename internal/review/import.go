package review

import (
	"context"
	"fmt"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// publishedFindingNote is the provenance the verifier is told for findings
// imported from the published review, unless the step sets its own note.
const publishedFindingNote = "an earlier review of this change published it, possibly against an older revision, " +
	"so it might be outdated. Confirm the problem still exists in the current code."

// groupProvenance describes where an imported group's findings came from and
// how the generic group steps treat them.
type groupProvenance struct {
	Source string
	// Note is told to the verifier for every finding of the group.
	Note string
	// ExemptDiffScope skips the diff-scope filter for the group, in
	// verification and in the final safeguard: the findings were in scope
	// when they were first reported, and leaving the diff proves nothing.
	ExemptDiffScope bool
	// Baseline is the published review the findings came from; reconcile
	// folds the run into it.
	Baseline *model.PublishedReview
}

// importConfig is an import-findings step's configuration.
type importConfig struct {
	group, source, note string
	findingsFrom        []string
}

func importConfigFrom(entry workflow.StepEntry) importConfig {
	cfg := importConfig{source: workflow.ImportSourceFile, findingsFrom: entry.FindingsFrom}
	if entry.Config != nil {
		if entry.Config.Group != nil {
			cfg.group = *entry.Config.Group
		}
		if entry.Config.Source != nil {
			cfg.source = *entry.Config.Source
		}
		if entry.Config.Note != nil {
			cfg.note = *entry.Config.Note
		}
	}
	return cfg
}

// importFindingsStepFunc runs an import-findings step: it fills the named
// group with the findings importFindings returns, together with their
// provenance. An empty import still registers the group, so the steps that
// address it are no-ops rather than errors.
func (e *Engine) importFindingsStepFunc(entry workflow.StepEntry) stepFunc {
	cfg := importConfigFrom(entry)
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		findings, provenance, err := e.importFindings(ctx, sc.Req, cfg)
		if err != nil {
			return err
		}
		run := model.AgentRun{Name: "Imported Findings (" + cfg.group + ")", Role: "import", Findings: len(findings), Status: model.AgentRunStatusOK}
		if len(findings) == 0 {
			run.Status = model.AgentRunStatusSkipped
		}
		st.setImportedGroup(cfg.group, agentResult{resp: &llm.ReviewResponse{Findings: findings}, run: run}, provenance)
		sc.Engine.logProgress(logging.StageReview, logging.StateDone, fmt.Sprintf("imported group=%s source=%s findings=%d", cfg.group, cfg.source, len(findings)))
		return nil
	}
}

// importFindings brings existing findings into the run from cfg's source.
//   - file: the findings_from files (the findings_from shorthand on verify,
//     dedupe, and merge reads them through the same loader).
//   - published-review: the open findings of the review this token already
//     published on the change request, read only by a publishing run whose
//     source supports it. A read failure is a warning: the run then publishes
//     as a fresh review.
func (e *Engine) importFindings(ctx context.Context, req model.ReviewRequest, cfg importConfig) ([]model.Finding, groupProvenance, error) {
	provenance := groupProvenance{Source: cfg.source, Note: cfg.note}
	switch cfg.source {
	case workflow.ImportSourceFile:
		groups, err := loadFindingsFiles(cfg.findingsFrom)
		if err != nil {
			return nil, provenance, err
		}
		findings := flattenInjectedGroups(groups).findings
		if req.DisableSuggestions {
			model.StripSuggestions(findings)
		}
		return findings, provenance, nil
	case workflow.ImportSourcePublishedReview:
		if provenance.Note == "" {
			provenance.Note = publishedFindingNote
		}
		provenance.ExemptDiffScope = true
		source, ok := e.source.(model.PublishedReviewSource)
		if !req.PostReview || !ok {
			return nil, provenance, nil
		}
		published, err := source.PublishedReview(ctx, req)
		if err != nil {
			warningsFromContext(ctx).addf("Could not read the published review: %v; publishing this run as a new review", err)
			return nil, provenance, nil
		}
		if published == nil || published.Review == nil {
			return nil, provenance, nil
		}
		provenance.Baseline = published
		var findings []model.Finding
		for _, f := range published.Review.Findings {
			if f.Resolution == nil {
				// The verifier and the later steps see the text as published,
				// not the provenance of the run that first produced it.
				findings = append(findings, currentFinding(f))
			}
		}
		return findings, provenance, nil
	default:
		return nil, provenance, fmt.Errorf("workflow: unknown import source %q", cfg.source)
	}
}
