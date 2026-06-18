package llm

import (
	"context"
	"strings"
	"sync"
	"unicode"
)

const (
	// Exact block detection compares the most recent k completed lines against
	// the k lines immediately before them. The max is high enough for review
	// loops that repeat an entire "finding / priority / suggestion" cycle, but
	// still bounded so each newline keeps predictable work.
	loopBlockMinLines       = 2
	loopBlockMaxLines       = 80
	loopBlockMinUniqueLines = 3

	// Fuzzy detection exists for loops that are semantically identical but not
	// byte-identical. Example: the model repeats the same review decision cycle,
	// while swapping method names or rewording "after Close returns" into
	// "after Close releases the mutex".
	//
	// The minimum token and shingle counts keep short repeated boilerplate from
	// firing the detector. Repeated headings like "Priority: 2" and "Suggestion"
	// are not enough signal.
	loopFuzzyMinTokens         = 120
	loopFuzzyMinUniqueShingles = 60

	// Shingles are contiguous token groups. Four-token shingles are long enough
	// to represent phrasing, but short enough that small wording changes still
	// leave overlap between repeated reasoning windows.
	loopFuzzyShingleSize = 4

	// Strict threshold triggers on very similar windows without extra evidence.
	// Marker threshold is lower, but requires both windows to contain repeated
	// review-decision markers such as findings, priorities, suggestions, or
	// "actually/wait" reconsideration. This catches loop.log-style behavior
	// while reducing false positives from normal long reasoning.
	loopFuzzyMarkerThreshold = 0.68
	loopFuzzyStrictThreshold = 0.82

	// Repeated-rune detection catches degenerate streams like 96 consecutive
	// newlines that never form meaningful repeated lines or fuzzy windows.
	loopRepeatedRuneWindowSize = 96
	loopRepeatedRuneMinCount   = 64
	loopRepeatedRuneMinRate    = 0.90
)

const liteLLMRepeatedChunkMarker = "The model is repeating the same chunk = "

// loopFuzzyWindowSizes are measured in completed reasoning lines. Each window
// is compared only with the immediately preceding same-sized window. Multiple
// sizes let us catch both compact and longer decision cycles without searching
// the whole history or comparing unrelated sections.
var loopFuzzyWindowSizes = []int{24, 32, 48, 64}

// ReasoningLoopDetectedError is returned when the model's streaming reasoning
// content repeats itself, indicating it has entered an infinite loop.
type ReasoningLoopDetectedError struct {
	ReasoningEffort  string
	LoopStartContent string // reasoning before the loop began
	RepeatedContent  string // the repeating line(s)
	// RepeatedChunk is true when the loop was reported by the upstream provider
	// as a repeated output chunk (LiteLLM marker) rather than detected in the
	// model's own reasoning content. Such loops occur in the completion stream
	// even when the model emits no reasoning at all.
	RepeatedChunk bool
}

func (e *ReasoningLoopDetectedError) Error() string {
	if e.RepeatedChunk {
		return "llm: model repeated output chunk during streaming"
	}
	return "llm: reasoning loop detected during streaming"
}

type reasoningLoopDetector struct {
	mu                sync.Mutex
	cancel            context.CancelFunc
	maxRepeats        int
	detected          bool
	repeatedChunk     bool
	loopStartContent  string
	repeatedContent   string
	lines             []string
	currentLine       strings.Builder
	recentRunes       []rune
	recentRuneStart   int
	runeCounts        map[rune]int
	runeCountBuckets  [loopRepeatedRuneWindowSize + 1]int
	maxRuneCount      int
	fuzzyRepeats      map[int]int
	fuzzyLastMatchEnd map[int]int
}

func newReasoningLoopDetector(cancel context.CancelFunc, maxRepeats int) *reasoningLoopDetector {
	return &reasoningLoopDetector{
		cancel:            cancel,
		maxRepeats:        maxRepeats,
		runeCounts:        make(map[rune]int, loopRepeatedRuneWindowSize),
		fuzzyRepeats:      make(map[int]int, len(loopFuzzyWindowSizes)),
		fuzzyLastMatchEnd: make(map[int]int, len(loopFuzzyWindowSizes)),
	}
}

func (d *reasoningLoopDetector) Detected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.detected
}

func (d *reasoningLoopDetector) MakeError() *ReasoningLoopDetectedError {
	d.mu.Lock()
	defer d.mu.Unlock()
	return &ReasoningLoopDetectedError{
		LoopStartContent: d.loopStartContent,
		RepeatedContent:  d.repeatedContent,
		RepeatedChunk:    d.repeatedChunk,
	}
}

func (d *reasoningLoopDetector) onDelta(delta string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.detected {
		return
	}
	for _, r := range delta {
		if d.observeRepeatedRuneLocked(r) {
			return
		}
		if r == '\n' {
			line := d.currentLine.String()
			d.currentLine.Reset()
			d.lines = append(d.lines, line)
			if d.checkLoopLocked() {
				return
			}
			continue
		}
		d.currentLine.WriteRune(r)
	}
}

func (d *reasoningLoopDetector) detectRepeatedChunkError(err error) bool {
	chunk, ok := repeatedChunkFromError(err)
	if !ok {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.detected {
		return true
	}
	if d.currentLine.Len() > 0 {
		d.lines = append(d.lines, d.currentLine.String())
		d.currentLine.Reset()
	}
	d.repeatedChunk = true
	d.trigger(chunk, len(d.lines))
	return true
}

func (d *reasoningLoopDetector) observeRepeatedRuneLocked(r rune) bool {
	if ignoredRepeatedRune(r) {
		return false
	}
	d.appendRecentRuneLocked(r)
	if len(d.recentRunes) < loopRepeatedRuneMinCount {
		return false
	}
	if d.maxRuneCount < loopRepeatedRuneMinCount {
		return false
	}
	if float64(d.maxRuneCount)/float64(len(d.recentRunes)) < loopRepeatedRuneMinRate {
		return false
	}
	d.trigger(d.recentRunesStringLocked(), len(d.lines))
	return true
}

func ignoredRepeatedRune(r rune) bool {
	// Ignore common formatting-only runs that can be benign in markdown output
	// or ASCII tables. Newlines are intentionally not ignored.
	switch r {
	case ' ', '-', '=', '*', '>', '_', '|':
		return true
	default:
		return false
	}
}

func (d *reasoningLoopDetector) appendRecentRuneLocked(r rune) {
	if len(d.recentRunes) < loopRepeatedRuneWindowSize {
		d.recentRunes = append(d.recentRunes, r)
		d.incrementRuneCountLocked(r)
		return
	}

	evicted := d.recentRunes[d.recentRuneStart]
	d.decrementRuneCountLocked(evicted)
	d.recentRunes[d.recentRuneStart] = r
	d.recentRuneStart = (d.recentRuneStart + 1) % loopRepeatedRuneWindowSize
	d.incrementRuneCountLocked(r)
}

func (d *reasoningLoopDetector) incrementRuneCountLocked(r rune) {
	if d.runeCounts == nil {
		d.runeCounts = make(map[rune]int, loopRepeatedRuneWindowSize)
	}
	oldCount := d.runeCounts[r]
	if oldCount > 0 {
		d.runeCountBuckets[oldCount]--
	}
	newCount := oldCount + 1
	d.runeCounts[r] = newCount
	d.runeCountBuckets[newCount]++
	if newCount > d.maxRuneCount {
		d.maxRuneCount = newCount
	}
}

func (d *reasoningLoopDetector) decrementRuneCountLocked(r rune) {
	oldCount := d.runeCounts[r]
	if oldCount == 0 {
		return
	}
	d.runeCountBuckets[oldCount]--
	newCount := oldCount - 1
	if newCount == 0 {
		delete(d.runeCounts, r)
	} else {
		d.runeCounts[r] = newCount
		d.runeCountBuckets[newCount]++
	}
	if oldCount == d.maxRuneCount && d.runeCountBuckets[oldCount] == 0 {
		for d.maxRuneCount > 0 && d.runeCountBuckets[d.maxRuneCount] == 0 {
			d.maxRuneCount--
		}
	}
}

func (d *reasoningLoopDetector) recentRunesStringLocked() string {
	if len(d.recentRunes) < loopRepeatedRuneWindowSize || d.recentRuneStart == 0 {
		return string(d.recentRunes)
	}
	ordered := make([]rune, 0, len(d.recentRunes))
	ordered = append(ordered, d.recentRunes[d.recentRuneStart:]...)
	ordered = append(ordered, d.recentRunes[:d.recentRuneStart]...)
	return string(ordered)
}

func repeatedChunkFromError(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	message := err.Error()
	_, after, ok := strings.Cut(message, liteLLMRepeatedChunkMarker)
	if !ok {
		return "", false
	}
	chunk := after
	end := len(chunk)
	for _, marker := range []string{"Received Model Group=", "Available Model Group Fallbacks="} {
		if idx := strings.Index(chunk, marker); idx >= 0 {
			end = min(end, idx)
		}
	}
	chunk = strings.TrimRight(chunk[:end], ". \t\r")
	chunk = strings.ReplaceAll(chunk, `\n`, "\n")
	chunk = strings.ReplaceAll(chunk, `\r`, "\r")
	chunk = strings.ReplaceAll(chunk, `\t`, "\t")
	if chunk == "" {
		return "", false
	}
	return chunk, true
}

func (d *reasoningLoopDetector) checkLoopLocked() bool {
	n := len(d.lines)

	// Strategy 1: same non-empty line repeated beyond the configured allowance.
	lineThreshold := d.maxRepeats + 2
	if n >= lineThreshold {
		last := d.lines[n-1]
		if strings.TrimSpace(last) != "" {
			allSame := true
			for i := n - lineThreshold; i < n-1; i++ {
				if d.lines[i] != last {
					allSame = false
					break
				}
			}
			if allSame {
				d.trigger(strings.Join(d.lines[n-lineThreshold:], "\n"), n-lineThreshold)
				return true
			}
		}
	}

	// Strategy 2: block of k lines appearing consecutively beyond the allowance.
	requiredCopies := d.maxRepeats + 2
	maxK := min(n/requiredCopies, loopBlockMaxLines)
	for k := loopBlockMinLines; k <= maxK; k++ {
		recentStart := n - k
		if !hasRepeatedBlockSignal(d.lines[recentStart:]) {
			continue
		}
		copies := 1
		for copyStart := recentStart - k; copyStart >= 0; copyStart -= k {
			match := true
			for i := 0; i < k; i++ {
				if d.lines[copyStart+i] != d.lines[recentStart+i] {
					match = false
					break
				}
			}
			if !match {
				break
			}
			copies++
			if copies >= requiredCopies {
				d.trigger(strings.Join(d.lines[recentStart:], "\n"), n-copies*k)
				return true
			}
		}
	}

	return d.checkFuzzyLoopLocked(n)
}

func hasRepeatedBlockSignal(lines []string) bool {
	unique := make(map[string]struct{})
	for _, line := range lines {
		normalized := strings.ToLower(strings.TrimSpace(line))
		if normalized != "" {
			unique[normalized] = struct{}{}
		}
	}
	return len(unique) >= loopBlockMinUniqueLines
}

func (d *reasoningLoopDetector) checkFuzzyLoopLocked(n int) bool {
	for _, k := range loopFuzzyWindowSizes {
		if n < 2*k {
			continue
		}
		prevStart := n - 2*k
		recentStart := n - k

		// Compare only adjacent windows. A repeated reasoning loop is expected
		// to recur immediately; comparing every pair of historical windows would
		// be slower and would flag legitimate revisits to earlier topics.
		prev := fuzzyReasoningWindow(d.lines[prevStart:recentStart])
		recent := fuzzyReasoningWindow(d.lines[recentStart:])
		if !prev.enoughSignal() || !recent.enoughSignal() {
			d.fuzzyRepeats[k] = 0
			continue
		}

		// Jaccard similarity is intersection / union of the shingle sets. It is
		// insensitive to repeated copies of the same phrase inside one window,
		// which helps focus on breadth of overlap rather than raw length.
		score := jaccardSimilarity(prev.shingles, recent.shingles)
		if score >= loopFuzzyStrictThreshold || score >= loopFuzzyMarkerThreshold && shareDecisionMarkers(prev.markers, recent.markers) {
			if lastEnd := d.fuzzyLastMatchEnd[k]; lastEnd == 0 || n-lastEnd >= k {
				d.fuzzyRepeats[k]++
				d.fuzzyLastMatchEnd[k] = n
				if d.fuzzyRepeats[k] > d.maxRepeats {
					d.trigger(strings.Join(d.lines[recentStart:], "\n"), prevStart-(d.maxRepeats*k))
					return true
				}
			}
			continue
		}
		d.fuzzyRepeats[k] = 0
		d.fuzzyLastMatchEnd[k] = 0
	}
	return false
}

type fuzzyWindow struct {
	// tokens are normalized words from the raw reasoning lines. They are kept so
	// enoughSignal can reject tiny windows before shingle similarity is trusted.
	tokens   []string
	shingles map[string]struct{}

	// markers summarize high-level review states observed in the window. They
	// are not model semantics; they are simple phrase buckets for repeated
	// review behavior that previously escaped exact matching.
	markers map[string]struct{}
}

func (w fuzzyWindow) enoughSignal() bool {
	return len(w.tokens) >= loopFuzzyMinTokens && len(w.shingles) >= loopFuzzyMinUniqueShingles
}

func fuzzyReasoningWindow(lines []string) fuzzyWindow {
	tokens := make([]string, 0, len(lines)*8)
	markers := make(map[string]struct{})
	for _, line := range lines {
		for _, marker := range decisionMarkers(line) {
			markers[marker] = struct{}{}
		}
		tokens = append(tokens, normalizeReasoningTokens(line)...)
	}
	return fuzzyWindow{
		tokens:   tokens,
		shingles: tokenShingles(tokens, loopFuzzyShingleSize),
		markers:  markers,
	}
}

// normalizeReasoningTokens intentionally removes details that often change
// between loop iterations while preserving the structure of the reasoning:
// case and punctuation disappear, code spans become "ident", and digit runs
// become "num". One-character tokens are dropped because they mostly add noise.
func normalizeReasoningTokens(s string) []string {
	var tokens []string
	var b strings.Builder
	inCode := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		token := b.String()
		b.Reset()
		if len(token) > 1 {
			tokens = append(tokens, token)
		}
	}
	for _, r := range s {
		if r == '`' {
			flush()
			if inCode {
				// Treat the full code span as one placeholder. In review loops,
				// identifiers often vary across iterations even when the model is
				// following the same reasoning script.
				tokens = append(tokens, "ident")
			}
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r):
			if b.String() != "num" {
				flush()
				// Collapse each run of digits into one token so line numbers,
				// priorities, counters, and attempt numbers do not prevent a match.
				b.WriteString("num")
			}
		default:
			flush()
		}
	}
	flush()
	return tokens
}

// tokenShingles returns a set, not a multiset. We only care whether a phrase
// shape appears in both windows, not how many times it appears inside one
// window.
func tokenShingles(tokens []string, size int) map[string]struct{} {
	shingles := make(map[string]struct{})
	if size <= 0 || len(tokens) < size {
		return shingles
	}
	for i := 0; i+size <= len(tokens); i++ {
		shingles[strings.Join(tokens[i:i+size], " ")] = struct{}{}
	}
	return shingles
}

// jaccardSimilarity returns 0..1 overlap between two shingle sets. Identical
// sets score 1. Disjoint sets score 0.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for shingle := range a {
		if _, ok := b[shingle]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// decisionMarkers are coarse signs that the model is repeating the review
// decision process itself. They let lower fuzzy similarity trigger only when
// both windows share important review states, avoiding false positives from
// ordinary long explanations that happen to reuse vocabulary.
func decisionMarkers(s string) []string {
	normalized := strings.Join(normalizeReasoningTokens(s), " ")
	var markers []string
	for _, marker := range []struct {
		name    string
		phrases []string
	}{
		{"finding", []string{"finding", "formulate the finding", "finalize the finding", "finalize my findings"}},
		{"priority", []string{"priority"}},
		{"suggestion", []string{"suggestion"}},
		{"reconsider", []string{"actually", "wait", "reconsider", "overthinking"}},
		{"old_new_code", []string{"old code", "new code"}},
		{"main_issue", []string{"main issue", "pre existing", "introduced by the patch"}},
	} {
		for _, phrase := range marker.phrases {
			if strings.Contains(normalized, phrase) {
				markers = append(markers, marker.name)
				break
			}
		}
	}
	return markers
}

// shareDecisionMarkers requires more than one shared marker so one repeated
// word, such as "finding", cannot make a weak fuzzy match trigger by itself.
func shareDecisionMarkers(a, b map[string]struct{}) bool {
	shared := 0
	for marker := range a {
		if _, ok := b[marker]; ok {
			shared++
		}
	}
	return shared >= 2
}

func (d *reasoningLoopDetector) trigger(repeatedContent string, loopStartLine int) {
	d.detected = true
	d.repeatedContent = repeatedContent
	if loopStartLine > 0 {
		d.loopStartContent = strings.Join(d.lines[:loopStartLine], "\n")
	}
	d.cancel()
}
