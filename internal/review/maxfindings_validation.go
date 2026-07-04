package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

// newMaxFindingsValidator returns a stateful validator that enforces the
// per-agent finding limit. It rejects at most once — asking the model to retry
// with only its strongest findings — then lets an over-limit response pass so
// enforceMaxFindingsResponse cuts the weakest findings instead of burning the
// whole retry budget. Callers create a fresh validator per agent-loop turn
// (responseValidator), so each turn gets its own single retry.
func newMaxFindingsValidator(limit int) func([]model.Finding, *llm.ReviewResponse) *llm.InvalidResponseError {
	retried := false
	return func(existing []model.Finding, resp *llm.ReviewResponse) *llm.InvalidResponseError {
		if resp == nil || retried {
			return nil
		}
		appendable := appendableFindings(existing, resp.Findings)
		if len(existing)+len(appendable) <= limit {
			return nil
		}
		retried = true
		allowed := max(limit-len(existing), 0)
		return &llm.InvalidResponseError{
			RawContent:            resp.RawResponse,
			Reason:                fmt.Sprintf("max_findings_exceeded limit=%d existing=%d new=%d", limit, len(existing), len(appendable)),
			ReasoningEffort:       resp.ReasoningEffort,
			ValidationFailure:     true,
			RetryGuidanceTemplate: "max_findings_retry_guidance.tmpl",
			RetryGuidanceData: struct {
				Limit    int
				Existing int
				Got      int
				Allowed  int
			}{
				Limit:    limit,
				Existing: len(existing),
				Got:      len(appendable),
				Allowed:  allowed,
			},
		}
	}
}

// enforceMaxFindingsResponse truncates resp.Findings so the session total
// (existing plus genuinely new findings) stays within limit, dropping the
// weakest findings first: lowest priority (highest PriorityRank), then lowest
// confidence. Returns a partial-run message when findings were dropped.
func enforceMaxFindingsResponse(agentName string, limit int, existing []model.Finding, resp *llm.ReviewResponse) string {
	if resp == nil || limit <= 0 {
		return ""
	}
	appendable := appendableFindings(existing, resp.Findings)
	budget := max(limit-len(existing), 0)
	if len(appendable) <= budget {
		// Shed duplicate entries even within budget: reviewerInitial stores
		// resp.Findings verbatim, and duplicates would inflate the session
		// count against the limit (findingLimitReached, nudge budgets).
		if len(appendable) != len(resp.Findings) {
			resp.Findings = appendable
		}
		return ""
	}
	kept, dropped := splitStrongestFindings(appendable, budget)
	// Replacing resp.Findings with the kept appendable subset also sheds
	// duplicates of already-accumulated findings; appendNewFindings would have
	// dropped those downstream anyway.
	resp.Findings = kept
	parts := make([]string, 0, len(dropped))
	for _, finding := range dropped {
		parts = append(parts, fmt.Sprintf("%s %q", normalizeReviewPath(finding.CodeLocation.FilePath), testingFindingTitle(finding)))
	}
	return fmt.Sprintf("%s findings over the max_findings limit (%d) dropped after retry exhaustion: %s", agentName, limit, strings.Join(parts, "; "))
}

// splitStrongestFindings splits findings into the n strongest — most critical
// priority first, then highest confidence, original order on ties — and the
// rest, each preserving the original relative order.
func splitStrongestFindings(findings []model.Finding, n int) ([]model.Finding, []model.Finding) {
	if n <= 0 {
		return nil, findings
	}
	if len(findings) <= n {
		return findings, nil
	}
	order := make([]int, len(findings))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		fa, fb := findings[order[a]], findings[order[b]]
		ra, rb := model.PriorityRank(fa.Priority), model.PriorityRank(fb.Priority)
		if ra != rb {
			return ra < rb
		}
		return fa.ConfidenceScore > fb.ConfidenceScore
	})
	keep := make(map[int]struct{}, n)
	for _, idx := range order[:n] {
		keep[idx] = struct{}{}
	}
	kept := make([]model.Finding, 0, n)
	dropped := make([]model.Finding, 0, len(findings)-n)
	for i, finding := range findings {
		if _, ok := keep[i]; ok {
			kept = append(kept, finding)
		} else {
			dropped = append(dropped, finding)
		}
	}
	return kept, dropped
}
