package dedupe

import (
	"sort"

	"github.com/dgrieser/nickpit/internal/model"
)

// MergeFindings mechanically merges two findings judged to be duplicates. The
// higher-confidence finding (ties: longer body) provides identity and text;
// the rest follows the merge rules the dedupe/cluster-merge prompts state:
//   - confidence: noisy-or, capped at 0.99 — two independent reviewers
//     agreeing is stronger evidence than either alone
//   - priority: the most critical of the two
//   - location: exact find_lines-backed locations are preserved; legacy
//     no-content line ranges are extended to cover both findings
//   - suggestions: included from both sides, no merge attempts
//   - verification: verdict and remarks from the side with the higher
//     verification confidence, highest confidence, most critical priority
func MergeFindings(a, b model.Finding) model.Finding {
	base, other := a, b
	if b.ConfidenceScore > a.ConfidenceScore ||
		(b.ConfidenceScore == a.ConfidenceScore && len(b.Body) > len(a.Body)) {
		base, other = b, a
	}

	out := base
	out.ConfidenceScore = noisyOr(base.ConfidenceScore, other.ConfidenceScore)
	out.Priority = mostCriticalPriority(base.Priority, other.Priority)
	out.CodeLocation = mergeCodeLocation(base.CodeLocation, other.CodeLocation)
	if len(other.Suggestions) > 0 {
		out.Suggestions = append(append([]model.Suggestion(nil), base.Suggestions...), other.Suggestions...)
	}
	out.Verification = mergeVerifications(base.Verification, other.Verification, out.ID)
	return out
}

func mergeCodeLocation(base, other model.CodeLocation) model.CodeLocation {
	if base.FilePath != other.FilePath {
		return base
	}
	if base.Content != "" || other.Content != "" {
		return base
	}
	out := base
	out.LineRange = extendRange(base.LineRange, other.LineRange)
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

// mergeVerifications follows the prompt-stated rules: verdict and remarks
// from the side with the higher verification confidence (base wins ties),
// the highest confidence score, and the most critical priority.
func mergeVerifications(base, other *model.FindingVerification, findingID string) *model.FindingVerification {
	if base == nil && other == nil {
		return nil
	}
	if base == nil {
		v := *other
		v.ID = findingID
		return &v
	}
	if other == nil {
		v := *base
		v.ID = findingID
		return &v
	}
	primary, secondary := base, other
	if other.ConfidenceScore > base.ConfidenceScore {
		primary, secondary = other, base
	}
	v := *primary
	v.ID = findingID
	v.Priority = min(primary.Priority, secondary.Priority)
	return &v
}
