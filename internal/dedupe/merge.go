package dedupe

import (
	"sort"

	"github.com/dgrieser/nickpit/internal/model"
)

// secondOpinionPrefix separates the absorbed finding's verification remarks
// from the base finding's so both reviewers' reasoning survives the merge.
const secondOpinionPrefix = "\n\n[second reviewer] "

// MergeFindings mechanically merges two findings judged to be duplicates. The
// higher-confidence finding (ties: longer body) provides identity and text;
// signals that benefit from agreement are combined:
//   - confidence: noisy-or, capped at 0.99 — two independent reviewers
//     agreeing is stronger evidence than either alone
//   - priority: the most critical of the two
//   - line range: extended to cover both findings
//   - suggestions: the more detailed side only, never concatenated
//   - verification: both remark texts kept, confidence noisy-or
func MergeFindings(a, b model.Finding) model.Finding {
	base, other := a, b
	if b.ConfidenceScore > a.ConfidenceScore ||
		(b.ConfidenceScore == a.ConfidenceScore && len(b.Body) > len(a.Body)) {
		base, other = b, a
	}

	out := base
	out.ConfidenceScore = noisyOr(base.ConfidenceScore, other.ConfidenceScore)
	out.Priority = mostCriticalPriority(base.Priority, other.Priority)
	out.CodeLocation.LineRange = extendRange(base.CodeLocation.LineRange, other.CodeLocation.LineRange)
	if suggestionWeight(other.Suggestions) > suggestionWeight(base.Suggestions) {
		out.Suggestions = other.Suggestions
	}
	out.Verification = mergeVerifications(base.Verification, other.Verification, out.ID)
	return out
}

// FoldCluster merges all findings of a duplicate cluster into one, folding in
// descending confidence order so the strongest finding provides the base text.
// Empty input returns a zero finding; callers guard for it.
func FoldCluster(findings []model.Finding) model.Finding {
	if len(findings) == 0 {
		return model.Finding{}
	}
	ordered := append([]model.Finding(nil), findings...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].ConfidenceScore > ordered[j].ConfidenceScore
	})
	out := ordered[0]
	for _, f := range ordered[1:] {
		out = MergeFindings(out, f)
	}
	return out
}

// noisyOr combines two confidence scores as independent agreeing signals:
// 1-(1-a)(1-b), clamped to [0, 0.99] so merged findings never claim certainty.
func noisyOr(a, b float64) float64 {
	a = clamp01(a)
	b = clamp01(b)
	combined := 1 - (1-a)*(1-b)
	if combined > 0.99 {
		return 0.99
	}
	return combined
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func mostCriticalPriority(a, b *int) *int {
	rankA, rankB := model.PriorityRank(a), model.PriorityRank(b)
	rank := min(rankA, rankB)
	// Preserve "unset" when both sides are unset; PriorityRank treats nil as
	// the default rank, so only materialize a pointer when one side had one.
	if a == nil && b == nil {
		return nil
	}
	out := rank
	return &out
}

// extendRange widens to the union of both ranges, ignoring unknown (zero)
// ranges so they cannot drag Start to 0.
func extendRange(a, b model.LineRange) model.LineRange {
	if (a == model.LineRange{}) {
		return b
	}
	if (b == model.LineRange{}) {
		return a
	}
	return model.LineRange{
		Start: min(a.Start, b.Start),
		End:   max(a.End, b.End),
	}
}

func suggestionWeight(suggestions []model.Suggestion) int {
	total := 0
	for _, s := range suggestions {
		total += len(s.Body)
	}
	return total
}

func mergeVerifications(base, other *model.FindingVerification, findingID string) *model.FindingVerification {
	if base == nil && other == nil {
		return nil
	}
	if base == nil {
		v := *other
		v.ID = findingID
		return &v
	}
	v := *base
	v.ID = findingID
	if other == nil {
		return &v
	}
	v.ConfidenceScore = noisyOr(base.ConfidenceScore, other.ConfidenceScore)
	v.Priority = min(base.Priority, other.Priority)
	if other.Remarks != "" && other.Remarks != base.Remarks {
		v.Remarks = base.Remarks + secondOpinionPrefix + other.Remarks
	}
	return &v
}
