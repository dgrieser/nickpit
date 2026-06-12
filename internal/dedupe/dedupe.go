// Package dedupe provides deterministic fuzzy duplicate detection and
// mechanical merging for review findings. It is intentionally rule-based:
// every verdict carries the signals that produced it, so logs can explain why
// a finding was dropped or clustered, and thresholds can be calibrated against
// real run data instead of being tuned blind.
package dedupe

import (
	"sort"
	"strings"
	"unicode"

	"github.com/dgrieser/nickpit/internal/model"
)

// Verdict orders duplicate confidence from clearly-distinct to byte-identical.
// Callers pick the minimum verdict that justifies their action: mechanical
// merging requires Duplicate, routing to an LLM judge requires only Possible.
type Verdict int

const (
	Distinct  Verdict = iota
	Possible          // plausible duplicate — needs LLM confirmation
	Duplicate         // strong duplicate — safe to merge mechanically
	Identical         // identical title, body, and location
)

func (v Verdict) String() string {
	switch v {
	case Possible:
		return "possible"
	case Duplicate:
		return "duplicate"
	case Identical:
		return "identical"
	default:
		return "distinct"
	}
}

// Match reports a comparison outcome plus the raw signals behind it.
type Match struct {
	Verdict     Verdict
	TitleSim    float64
	BodySim     float64
	LocationSim float64
	Reason      string
}

// Thresholds are exported so calibration tests can reference the exact values
// they validate; tuning one shows up as a test diff, not a silent behavior
// change.
const (
	// LocNear marks line ranges that overlap or nearly touch.
	LocNear = 0.8
	// LocSameRegion marks the same general area of a file; also the neutral
	// score when a line range is unknown.
	LocSameRegion = 0.4
	// TitleStrong marks titles that say the same thing.
	TitleStrong = 0.85
	// TitleModerate marks clearly related titles.
	TitleModerate = 0.55
	// BodyStrong marks bodies that describe the same defect.
	BodyStrong = 0.6
	// BodyModerate marks clearly related bodies. Calibrated against MR 1560
	// runs: same-line same-aspect pairs score ~0.36, same-file different-defect
	// pairs ~0.06.
	BodyModerate = 0.3
	// LineGapHorizon is the gap in lines at which location similarity decays
	// to zero.
	LineGapHorizon = 40.0
)

// Compare classifies how likely a and b describe the same issue. Findings in
// different files are always Distinct: a code defect and the test gap covering
// it are separate findings by convention, even when their text agrees.
func Compare(a, b model.Finding) Match {
	if a.CodeLocation.FilePath != b.CodeLocation.FilePath {
		return Match{Verdict: Distinct, Reason: "different file"}
	}

	m := Match{
		TitleSim:    textSimilarity(a.Title, b.Title),
		BodySim:     textSimilarity(a.Body, b.Body),
		LocationSim: lineSimilarity(a.CodeLocation.LineRange, b.CodeLocation.LineRange),
	}

	switch {
	case a.Title == b.Title && a.Body == b.Body && a.CodeLocation == b.CodeLocation:
		m.Verdict, m.Reason = Identical, "identical title, body and location"

	// Same code region and either the titles or the bodies agree strongly.
	case m.LocationSim >= LocNear && (m.TitleSim >= TitleModerate || m.BodySim >= BodyStrong):
		m.Verdict, m.Reason = Duplicate, "overlapping lines with agreeing text"

	// Title is essentially the same sentence; trust it even when the line
	// anchors drifted, as long as both point at the same part of the file.
	case m.TitleSim >= TitleStrong && m.LocationSim >= LocSameRegion:
		m.Verdict, m.Reason = Duplicate, "near-identical title in same region"

	case m.LocationSim >= LocSameRegion && (m.TitleSim >= TitleModerate || m.BodySim >= BodyModerate):
		m.Verdict, m.Reason = Possible, "same region with related text"

	case m.TitleSim >= TitleStrong:
		m.Verdict, m.Reason = Possible, "near-identical title, distant lines"

	default:
		m.Verdict, m.Reason = Distinct, "signals below thresholds"
	}
	return m
}

// FindBest returns the index of the strongest match in pool at or above min,
// or -1. Ties resolve to the higher combined signal.
func FindBest(target model.Finding, pool []model.Finding, min Verdict) (int, Match) {
	best, bestIdx := Match{Verdict: Distinct}, -1
	for i := range pool {
		m := Compare(target, pool[i])
		if m.Verdict < min {
			continue
		}
		if bestIdx == -1 || m.Verdict > best.Verdict ||
			(m.Verdict == best.Verdict && combined(m) > combined(best)) {
			best, bestIdx = m, i
		}
	}
	return bestIdx, best
}

// Clusters groups findings into duplicate clusters via union-find over all
// pairs at or above min. Singleton clusters mean "no duplicate found". Cluster
// order follows the smallest member index, and members stay in input order, so
// output is deterministic.
func Clusters(findings []model.Finding, min Verdict) [][]int {
	parent := make([]int, len(findings))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	for i := range findings {
		for j := i + 1; j < len(findings); j++ {
			if Compare(findings[i], findings[j]).Verdict >= min {
				parent[find(i)] = find(j)
			}
		}
	}
	groups := make(map[int][]int)
	for i := range findings {
		root := find(i)
		groups[root] = append(groups[root], i)
	}
	out := make([][]int, 0, len(groups))
	for _, g := range groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func combined(m Match) float64 { return m.TitleSim + m.BodySim + m.LocationSim }

// lineSimilarity scores 1.0 for overlapping ranges and decays linearly with
// the gap until LineGapHorizon. An unknown range neither confirms nor denies,
// so it scores the same-region neutral value.
func lineSimilarity(a, b model.LineRange) float64 {
	if (a == model.LineRange{}) || (b == model.LineRange{}) {
		return LocSameRegion
	}
	if a.Start <= b.End && b.Start <= a.End {
		return 1.0
	}
	gap := float64(a.Start - b.End)
	if b.Start > a.End {
		gap = float64(b.Start - a.End)
	}
	if gap >= LineGapHorizon {
		return 0
	}
	return 1.0 - gap/LineGapHorizon
}

// textSimilarity is max(Jaccard, overlap coefficient) over normalized token
// sets. The overlap coefficient rewards subset phrasings ("X" vs "X enabling
// Y") that Jaccard alone would punish for the length difference.
func textSimilarity(a, b string) float64 {
	ta, tb := tokens(a), tokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for t := range ta {
		if _, ok := tb[t]; ok {
			inter++
		}
	}
	jaccard := float64(inter) / float64(len(ta)+len(tb)-inter)
	minLen := min(len(ta), len(tb))
	return max(jaccard, float64(inter)/float64(minLen))
}

// stopwords holds glue words only. Negations ("not", "no", "missing",
// "without") are deliberately kept — they carry the finding's meaning.
var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "be": {}, "been": {},
	"in": {}, "on": {}, "of": {}, "for": {}, "to": {}, "and": {}, "or": {},
	"with": {}, "that": {}, "this": {}, "it": {}, "its": {}, "may": {},
	"can": {}, "could": {}, "should": {}, "would": {}, "via": {}, "by": {},
}

func tokens(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.'
	}) {
		if _, stop := stopwords[f]; stop || len(f) < 2 {
			continue
		}
		out[f] = struct{}{}
	}
	return out
}
