// Package dedupe provides deterministic fuzzy duplicate detection and
// mechanical merging for review findings. It is intentionally rule-based:
// every verdict carries the signals that produced it, so logs can explain why
// a finding was dropped or clustered, and thresholds can be calibrated against
// real run data instead of being tuned blind.
package dedupe

import (
	"path"
	"regexp"
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
	Verdict      Verdict
	TitleSim     float64
	BodySim      float64
	LocationSim  float64
	RootCauseSim float64
	Reason       string
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
	// RootCauseStrong marks cross-file findings with shared concrete anchors
	// and issue vocabulary. It is only used to route related files to the LLM;
	// cross-file pairs still cap at Possible.
	RootCauseStrong = 0.45
	// LineGapHorizon is the gap in lines at which location similarity decays
	// to zero.
	LineGapHorizon = 40.0
	// OverlapMinTokens and OverlapMinRatio gate the overlap-coefficient term
	// for title/body similarity. Ungated, any subset scores 1.0, so a short
	// generic title ("Race condition") would absorb every longer title that
	// contains it — permanently suppressing new findings at publish time. The
	// overlap term only applies when the smaller token set is substantial and
	// the set sizes are comparable.
	OverlapMinTokens = 4
	OverlapMinRatio  = 0.5
)

// Compare classifies how likely a and b describe the same issue. Findings in
// different files are Distinct by convention — a code defect and the test gap
// covering it are separate findings even when their text agrees — with two
// exceptions: near-identical titles, moderately related title and body signals,
// or strong shared root-cause signals in related files are at most Possible, so
// the merge agent judges them instead of duplicate-looking findings surviving
// to the final review. Cross-file pairs never reach Duplicate: mechanical
// folding assumes one file (extendRange), so the LLM is always in the loop.
// Findings without any file path are judged on text alone and only fold
// mechanically when both title and body agree strongly.
func Compare(a, b model.Finding) Match {
	return compare(prepare(a), prepare(b))
}

// prepared caches the derived signals of one finding so Clusters' O(n²) pair
// loop tokenizes each finding once instead of once per pair.
type prepared struct {
	f     model.Finding
	path  string
	title map[string]struct{}
	body  map[string]struct{}
}

func prepare(f model.Finding) prepared {
	return prepared{
		f:     f,
		path:  cleanFilePath(f.CodeLocation.FilePath),
		title: tokens(f.Title),
		body:  tokens(f.Body),
	}
}

// cleanFilePath normalizes lexical variants ("./x.go" vs "x.go") so they
// compare as the same file; empty stays empty ("no location", not ".").
func cleanFilePath(p string) string {
	if p == "" {
		return ""
	}
	return path.Clean(p)
}

func compare(a, b prepared) Match {
	// No file path on either side: repo-level findings. There is no location
	// evidence to corroborate a fold, so text must carry the whole verdict —
	// mechanical Duplicate only when title AND body agree strongly, otherwise
	// route to the LLM at most.
	if a.path == "" && b.path == "" {
		m := Match{
			TitleSim:    textSetSimilarity(a.title, b.title),
			BodySim:     textSetSimilarity(a.body, b.body),
			LocationSim: LocSameRegion,
		}
		switch {
		case a.f.Title == b.f.Title && a.f.Body == b.f.Body && a.f.CodeLocation == b.f.CodeLocation:
			m.Verdict, m.Reason = Identical, "identical title, body and location"
		case m.TitleSim >= TitleStrong && m.BodySim >= BodyStrong:
			m.Verdict, m.Reason = Duplicate, "near-identical text without location"
		case m.TitleSim >= TitleStrong || (m.TitleSim >= TitleModerate && m.BodySim >= BodyModerate):
			m.Verdict, m.Reason = Possible, "related text without location"
		default:
			m.Verdict, m.Reason = Distinct, "signals below thresholds"
		}
		return m
	}

	if a.path != b.path {
		m := Match{
			TitleSim: textSetSimilarity(a.title, b.title),
			BodySim:  textSetSimilarity(a.body, b.body),
		}
		if m.TitleSim >= TitleStrong {
			m.Verdict, m.Reason = Possible, "near-identical title across files"
		} else if m.TitleSim >= TitleModerate && m.BodySim >= BodyModerate {
			m.Verdict, m.Reason = Possible, "related title and body across files"
		} else if sameReviewFileKind(a.path, b.path) && relatedFiles(a.path, b.path) {
			m.RootCauseSim = rootCauseSimilarity(a.f, b.f)
			if m.RootCauseSim >= RootCauseStrong {
				m.Verdict, m.Reason = Possible, "same root-cause signals across related files"
			} else {
				m.Verdict, m.Reason = Distinct, "different file"
			}
		} else {
			m.Verdict, m.Reason = Distinct, "different file"
		}
		return m
	}

	m := Match{
		TitleSim:    textSetSimilarity(a.title, b.title),
		BodySim:     textSetSimilarity(a.body, b.body),
		LocationSim: lineSimilarity(a.f.CodeLocation.LineRange, b.f.CodeLocation.LineRange),
	}

	switch {
	case a.f.Title == b.f.Title && a.f.Body == b.f.Body && a.f.CodeLocation == b.f.CodeLocation:
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
	pt := prepare(target)
	for i := range pool {
		m := compare(pt, prepare(pool[i]))
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
	preps := make([]prepared, len(findings))
	for i := range findings {
		preps[i] = prepare(findings[i])
	}
	for i := range preps {
		for j := i + 1; j < len(preps); j++ {
			if compare(preps[i], preps[j]).Verdict >= min {
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

func combined(m Match) float64 {
	return m.TitleSim + m.BodySim + m.LocationSim + m.RootCauseSim
}

// lineSimilarity scores 1.0 for overlapping ranges and decays linearly with
// the gap until LineGapHorizon. An unknown or malformed range (zero/negative
// start, inverted bounds — possible from unvalidated LLM output) neither
// confirms nor denies, so it scores the same-region neutral value.
func lineSimilarity(a, b model.LineRange) float64 {
	if !plausibleRange(a) || !plausibleRange(b) {
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

func plausibleRange(r model.LineRange) bool {
	return r.Start > 0 && r.End >= r.Start
}

// textSetSimilarity scores normalized token sets of titles/bodies as
// max(Jaccard, gated overlap coefficient). The overlap coefficient rewards
// subset phrasings ("X" vs "X enabling Y") that Jaccard alone would punish
// for the length difference, but only when the smaller set has at least
// OverlapMinTokens tokens and the sizes are within OverlapMinRatio —
// otherwise a short generic phrase would score 1.0 against every superset.
func textSetSimilarity(a, b map[string]struct{}) float64 {
	jaccard, overlap := setScores(a, b)
	minLen, maxLen := min(len(a), len(b)), max(len(a), len(b))
	if minLen < OverlapMinTokens || float64(minLen)/float64(maxLen) < OverlapMinRatio {
		return jaccard
	}
	return max(jaccard, overlap)
}

// setSimilarity is the ungated max(Jaccard, overlap coefficient). It stays
// in use for filename-family and root-cause term/anchor sets, which are
// small by nature and calibrated with the overlap term intact.
func setSimilarity(a, b map[string]struct{}) float64 {
	jaccard, overlap := setScores(a, b)
	return max(jaccard, overlap)
}

func setScores(a, b map[string]struct{}) (jaccard, overlap float64) {
	if len(a) == 0 || len(b) == 0 {
		return 0, 0
	}
	inter := 0
	for t := range a {
		if _, ok := b[t]; ok {
			inter++
		}
	}
	jaccard = float64(inter) / float64(len(a)+len(b)-inter)
	overlap = float64(inter) / float64(min(len(a), len(b)))
	return jaccard, overlap
}

func rootCauseSimilarity(a, b model.Finding) float64 {
	termsA, termsB := rootCauseTerms(a), rootCauseTerms(b)
	anchorsA, anchorsB := codeAnchors(a), codeAnchors(b)
	for term := range anchorsA {
		delete(termsA, term)
	}
	for term := range anchorsB {
		delete(termsB, term)
	}
	sharedTerms := intersectionCount(termsA, termsB)
	sharedAnchors := intersectionCount(anchorsA, anchorsB)
	requiredTerms := min(3, min(len(termsA), len(termsB)))
	if requiredTerms < 2 || sharedTerms < requiredTerms || sharedAnchors == 0 {
		return 0
	}
	termScore := setSimilarity(termsA, termsB)
	anchorScore := setSimilarity(anchorsA, anchorsB)
	return 0.65*termScore + 0.35*anchorScore
}

func rootCauseTerms(f model.Finding) map[string]struct{} {
	out := tokens(f.Title + " " + f.Body)
	for word := range rootCauseStopwords {
		delete(out, word)
	}
	return out
}

func codeAnchors(f model.Finding) map[string]struct{} {
	text := f.Title + "\n" + f.Body
	out := map[string]struct{}{}
	for _, re := range anchorRegexps {
		for _, match := range re.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 {
				addAnchorTerms(out, match[1])
			} else {
				addAnchorTerms(out, match[0])
			}
		}
	}
	addStructuredIdentifierAnchors(out, text)
	return out
}

func addAnchorTerms(out map[string]struct{}, raw string) {
	for term := range tokens(raw) {
		out[term] = struct{}{}
	}
}

func addStructuredIdentifierAnchors(out map[string]struct{}, text string) {
	for field := range strings.FieldsSeq(text) {
		field = strings.Trim(field, " \t\r\n,;:()[]{}<>")
		// Edge dots are sentence punctuation, not structure: without this trim
		// every sentence-final word ("paths.") counts as a dotted identifier
		// and gets deleted from the root-cause term sets. Interior dots
		// (pkg.Func) survive.
		field = strings.Trim(field, ".")
		if !strings.ContainsAny(field, "._") || !hasLetter(field) {
			continue
		}
		addAnchorTerms(out, field)
	}
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func intersectionCount(a, b map[string]struct{}) int {
	count := 0
	for t := range a {
		if _, ok := b[t]; ok {
			count++
		}
	}
	return count
}

func relatedFiles(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	dirA, dirB := path.Dir(a), path.Dir(b)
	if dirA == dirB {
		return true
	}
	if basenameFamily(a, b) && commonDirPrefixSegments(dirA, dirB) >= 1 {
		return true
	}
	extA, extB := path.Ext(a), path.Ext(b)
	return extA != "" && extA == extB && commonDirPrefixSegments(dirA, dirB) >= 2
}

func sameReviewFileKind(a, b string) bool {
	return isTestLikeFile(a) == isTestLikeFile(b)
}

func isTestLikeFile(file string) bool {
	base := strings.ToLower(path.Base(file))
	if strings.Contains(base, "_test.") || strings.Contains(base, ".test.") ||
		strings.HasPrefix(base, "test_") || strings.HasPrefix(base, "test-") ||
		strings.Contains(base, "_spec.") || strings.Contains(base, ".spec.") {
		return true
	}
	for _, segment := range pathSegments(path.Dir(file)) {
		segment = strings.ToLower(segment)
		if segment == "test" || segment == "tests" || segment == "__tests__" ||
			segment == "spec" || segment == "specs" {
			return true
		}
	}
	return false
}

func commonDirPrefixSegments(a, b string) int {
	partsA, partsB := pathSegments(a), pathSegments(b)
	n := min(len(partsA), len(partsB))
	for i := range n {
		if partsA[i] != partsB[i] {
			return i
		}
	}
	return n
}

func pathSegments(p string) []string {
	p = path.Clean(p)
	if p == "." || p == "/" {
		return nil
	}
	return strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
}

func basenameFamily(a, b string) bool {
	tokensA, tokensB := filenameTokens(path.Base(a)), filenameTokens(path.Base(b))
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return false
	}
	return setSimilarity(tokensA, tokensB) >= 0.6
}

func filenameTokens(name string) map[string]struct{} {
	name = strings.TrimSuffix(name, path.Ext(name))
	return tokens(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, name))
}

// stopwords holds glue words only. Negations ("not", "no", "missing",
// "without") are deliberately kept — they carry the finding's meaning.
var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "be": {}, "been": {},
	"in": {}, "on": {}, "of": {}, "for": {}, "to": {}, "and": {}, "or": {},
	"with": {}, "that": {}, "this": {}, "it": {}, "its": {}, "may": {},
	"can": {}, "could": {}, "should": {}, "would": {}, "via": {}, "by": {},
}

var rootCauseStopwords = map[string]struct{}{
	"finding": {}, "findings": {}, "issue": {}, "issues": {}, "bug": {}, "bugs": {},
	"code": {}, "patch": {}, "line": {}, "lines": {}, "file": {}, "files": {},
	"change": {}, "changes": {}, "fix": {}, "fixes": {}, "problem": {}, "problems": {},
	"logic": {}, "case": {}, "cases": {}, "template": {}, "templates": {},
	"command": {}, "commands": {}, "argument": {}, "arguments": {},
}

var anchorRegexps = []*regexp.Regexp{
	regexp.MustCompile("`([^`]+)`"),
	regexp.MustCompile(`"([^"\n]{1,80})"`),
	regexp.MustCompile(`(\$[A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`\b([A-Z][A-Z0-9_]{1,})\b`),
	regexp.MustCompile(`\b([A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+)\b`),
}

func tokens(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.'
	}) {
		// Dots are token characters so qualified identifiers (pkg.Func)
		// survive, but that also glues sentence punctuation onto ordinary
		// words ("element." vs "element"). Trim edge dots; interior stay.
		f = strings.Trim(f, ".")
		if _, stop := stopwords[f]; stop || len(f) < 2 {
			continue
		}
		out[f] = struct{}{}
	}
	return out
}
