package llm

import (
	"context"
	"hash/fnv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// The detector watches streamed reasoning content and cancels the stream when
// the model has entered a loop. It layers three independent signals, from
// cheap-and-exact to fuzzy:
//
//  1. Character runs: one rune (or a short unit of up to charMaxPeriod runes)
//     repeated back-to-back. Degenerate output, fires at any point in time.
//  2. Exact line repetition: the same normalized line or block of lines
//     repeated consecutively. Whitespace runs are collapsed and empty lines
//     are ignored, so "byte-identical modulo formatting" counts as exact.
//  3. Shingle recurrence: the fraction of recently emitted token shingles that
//     appeared earlier in the same stream. Verbatim loops drive this to ~1.0;
//     paraphrase loops (same decision cycle reworded, identifiers or numbers
//     swapped) still reuse most phrasing and plateau lower. A loop is called
//     when the recurrence stays above a threshold for long enough.
//
// Detection needs no configuration. Instead, thresholds are staged over the
// reasoning time budget (max_reasoning_seconds): early on, only ironclad
// repetition may cancel the stream; the closer the stream gets to the budget
// (where it would be cancelled anyway), the more aggressive detection becomes,
// because the cost of a false positive shrinks while the expected saving
// grows. All constants below were tuned against a corpus of ~1900 reasoning
// traces extracted from real review runs (verbatim loops, paraphrase loops,
// provider-detected repeated chunks, timed-out vacillation loops, and clean
// long reasoning).

const (
	// loopStagingFallbackBudget stages thresholds when no reasoning time limit
	// is configured. It mirrors config.DefaultMaxReasoningSeconds without
	// importing the config package.
	loopStagingFallbackBudget = 300 * time.Second

	// Character-run detection. The window holds the most recent runes; a run
	// unit of up to charMaxPeriod runes must repeat exactly through
	// charTriggerSpan trailing runes to fire. The period stays short so this
	// layer only sees degenerate output ("aaaa", "ab ab ab"); repeated whole
	// sentences belong to the staged line detector. The frequency trigger
	// fires when one rune dominates the window even with interruptions.
	charWindowSize      = 128
	charMaxPeriod       = 8
	charTriggerSpan     = 96
	charMinPeriodCopies = 4
	charFreqMinCount    = 64
	charFreqMinRate     = 0.90

	// Exact line repetition. Periods are measured in normalized non-empty
	// lines; a "copy" is one full repetition of the repeated unit. Single
	// repeated lines need more copies than multi-line blocks because short
	// closers such as "}" legitimately recur.
	lineMaxPeriod        = 256
	lineMinUnitChars     = 12
	lineShortLine        = 4
	lineShortLineCopies  = 12
	lineTrackedPositions = 8

	// Shingle recurrence. Tokens are normalized words (case folded, digit runs
	// collapsed, code spans replaced by a placeholder); a shingle is
	// shingleSize consecutive tokens. Recurrence is the fraction of the last
	// shingleWindow shingles that occurred anywhere earlier in the stream.
	// Once armed at the stage's threshold the plateau keeps growing while
	// recurrence stays above (threshold - shingleHysteresis); it must reach
	// the stage's plateau length to fire.
	shingleSize       = 5
	shingleWindow     = 256
	shingleHysteresis = 0.10

	// The hard tier is stage-independent: recurrence this close to total
	// repetition is never legitimate reasoning, no matter how early it
	// happens. Its plateau is sized above the longest template-style
	// enumeration observed in clean traces (~360 shingles of "check field X,
	// use it" cycles) so structured-but-productive passages survive.
	shingleHardArm     = 0.95
	shingleHardPlateau = 400
)

// loopStage holds the detection thresholds active for one segment of the
// reasoning time budget.
type loopStage struct {
	// until is the budget fraction this stage applies up to.
	until float64
	// blockCopies is the number of consecutive copies of a multi-line unit
	// that count as an exact loop; lineCopies is the same for a single line.
	blockCopies int
	lineCopies  int
	// shingleArm is the recurrence fraction that arms the fuzzy plateau;
	// shinglePlateau is how many shingles the plateau must span to fire.
	shingleArm     float64
	shinglePlateau int
}

// loopStages must be ordered by increasing `until`. The last stage also covers
// streams that exceed the budget (possible when no budget is enforced).
var loopStages = []loopStage{
	{until: 0.25, blockCopies: 5, lineCopies: 10, shingleArm: 0.90, shinglePlateau: 900},
	{until: 0.50, blockCopies: 4, lineCopies: 8, shingleArm: 0.80, shinglePlateau: 600},
	{until: 0.75, blockCopies: 3, lineCopies: 6, shingleArm: 0.70, shinglePlateau: 450},
	{until: 1.01, blockCopies: 3, lineCopies: 5, shingleArm: 0.60, shinglePlateau: 300},
}

func stageFor(fraction float64) loopStage {
	for _, stage := range loopStages {
		if fraction < stage.until {
			return stage
		}
	}
	return loopStages[len(loopStages)-1]
}

const liteLLMRepeatedChunkMarker = "The model is repeating the same chunk = "

// ReasoningLoopDetectedError is returned when the model's streaming reasoning
// content repeats itself, indicating it has entered an infinite loop.
type ReasoningLoopDetectedError struct {
	ReasoningEffort  string
	LoopStartContent string // reasoning before the loop began
	RepeatedContent  string // the repeating portion
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
	mu     sync.Mutex
	cancel context.CancelFunc
	now    func() time.Time
	budget time.Duration
	start  time.Time

	detected         bool
	repeatedChunk    bool
	loopStartContent string
	repeatedContent  string

	// raw stream, kept for error reporting
	raw strings.Builder

	// character-run state
	charRing  [charWindowSize]rune
	charLen   int
	charStart int
	charSince int // runes since the last periodicity scan

	// line state
	currentLine strings.Builder
	// rawLineStart is the byte offset of the current (incomplete) raw line.
	rawLineStart int
	// lines holds normalized non-empty lines; lineStarts the byte offset of
	// each in raw.
	lines      []string
	lineHashes []uint64
	lineStarts []int
	linePos    map[uint64][]int
	// lineMatch maps candidate period -> length of the current suffix of
	// lineHashes that matches itself shifted by that period.
	lineMatch map[int]int

	// shingle state
	tokenTail     []string // last shingleSize-1 tokens
	shingleSeen   map[uint64]struct{}
	shingleRing   [shingleWindow]bool
	shingleLen    int
	shingleStart  int
	shingleRepeat int // repeated shingles inside the ring
	plateauLen    int // shingles since the plateau was armed
	plateauOffset int // byte offset into raw where the plateau was armed
	armedAt       float64
	hardLen       int // shingles since the hard (near-verbatim) tier armed
	hardOffset    int
}

func newReasoningLoopDetector(cancel context.CancelFunc, budget time.Duration) *reasoningLoopDetector {
	if budget <= 0 {
		budget = loopStagingFallbackBudget
	}
	return &reasoningLoopDetector{
		cancel:      cancel,
		now:         time.Now,
		budget:      budget,
		linePos:     make(map[uint64][]int),
		lineMatch:   make(map[int]int),
		shingleSeen: make(map[uint64]struct{}),
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

// budgetFraction reports how much of the reasoning time budget has elapsed,
// clamped to [0, 1].
func (d *reasoningLoopDetector) budgetFractionLocked() float64 {
	if d.start.IsZero() {
		return 0
	}
	f := float64(d.now().Sub(d.start)) / float64(d.budget)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func (d *reasoningLoopDetector) onDelta(delta string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.detected {
		return
	}
	if d.start.IsZero() {
		d.start = d.now()
	}
	for _, r := range delta {
		d.raw.WriteRune(r)
		if d.observeRuneLocked(r) {
			return
		}
		if r == '\n' {
			d.finishLineLocked()
			if d.detected {
				return
			}
			continue
		}
		d.currentLine.WriteRune(r)
	}
}

func (d *reasoningLoopDetector) finishLineLocked() {
	line := d.currentLine.String()
	d.currentLine.Reset()
	lineStart := d.rawLineStart
	d.rawLineStart = d.raw.Len()

	normalized := strings.Join(strings.Fields(line), " ")
	if normalized == "" {
		return
	}
	if d.observeLineLocked(normalized, lineStart) {
		return
	}
	d.observeTokensLocked(normalized)
}

// --- character runs ---

func (d *reasoningLoopDetector) observeRuneLocked(r rune) bool {
	if d.charLen < charWindowSize {
		d.charRing[(d.charStart+d.charLen)%charWindowSize] = r
		d.charLen++
	} else {
		d.charRing[d.charStart] = r
		d.charStart = (d.charStart + 1) % charWindowSize
	}
	d.charSince++
	// Scanning every rune would be wasted work: a run long enough to fire
	// stays detectable a few runes later, so scan on a small stride.
	if d.charSince < 16 || d.charLen < charTriggerSpan {
		return false
	}
	d.charSince = 0
	if unit, span := d.repeatedRunLocked(); unit != "" {
		d.triggerRawLocked(strings.Repeat(unit, span/len([]rune(unit))), d.raw.Len()-span)
		return true
	}
	return false
}

// repeatedRunLocked looks for the smallest period whose repetition covers the
// trailing charTriggerSpan runes of the window. It returns the repeating unit
// and the length of the covered span in runes.
func (d *reasoningLoopDetector) repeatedRunLocked() (string, int) {
	n := d.charLen
	at := func(i int) rune { // i counted from the oldest rune in the window
		return d.charRing[(d.charStart+i)%charWindowSize]
	}
	for p := 1; p <= charMaxPeriod; p++ {
		if p*charMinPeriodCopies > charTriggerSpan {
			break
		}
		span := 0
		for i := n - 1; i >= p; i-- {
			if at(i) != at(i-p) {
				break
			}
			span++
		}
		span += p // the first unit itself
		if span < charTriggerSpan || span/p < charMinPeriodCopies {
			continue
		}
		unit := make([]rune, 0, p)
		for i := n - p; i < n; i++ {
			unit = append(unit, at(i))
		}
		if !meaningfulRunUnit(unit) {
			continue
		}
		return string(unit), span
	}
	// Frequency fallback: one rune dominating the window even when other
	// runes interrupt the exact run.
	counts := make(map[rune]int, 8)
	best := 0
	var bestRune rune
	for i := 0; i < n; i++ {
		r := at(i)
		counts[r]++
		if counts[r] > best {
			best = counts[r]
			bestRune = r
		}
	}
	if best >= charFreqMinCount && float64(best)/float64(n) >= charFreqMinRate && !ignoredRepeatedRune(bestRune) {
		runes := make([]rune, 0, n)
		for i := 0; i < n; i++ {
			runes = append(runes, at(i))
		}
		return string(runes), n
	}
	return "", 0
}

// meaningfulRunUnit rejects units made only of formatting characters, which
// legitimately run long in markdown horizontal rules or ASCII tables.
func meaningfulRunUnit(unit []rune) bool {
	for _, r := range unit {
		if !ignoredRepeatedRune(r) && r != '\n' && r != '\t' && r != '\r' {
			return true
		}
	}
	// All-whitespace units (for example newline floods) are still degenerate.
	for _, r := range unit {
		if r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

func ignoredRepeatedRune(r rune) bool {
	switch r {
	case ' ', '-', '=', '*', '>', '_', '|', '#', '.', '`', '+', '~':
		return true
	default:
		return false
	}
}

// --- exact line repetition ---

func (d *reasoningLoopDetector) observeLineLocked(normalized string, rawStart int) bool {
	h := fnv.New64a()
	_, _ = h.Write([]byte(normalized))
	hash := h.Sum64()

	n := len(d.lineHashes)
	d.lines = append(d.lines, normalized)
	d.lineHashes = append(d.lineHashes, hash)
	d.lineStarts = append(d.lineStarts, rawStart)

	// Update existing candidate periods, drop the ones the new line breaks.
	for p, matched := range d.lineMatch {
		if n >= p && d.lineHashes[n-p] == hash {
			d.lineMatch[p] = matched + 1
		} else {
			delete(d.lineMatch, p)
		}
	}
	// Register new candidate periods from recent occurrences of this line.
	for _, pos := range d.linePos[hash] {
		p := n - pos
		if p >= 1 && p <= lineMaxPeriod {
			if _, ok := d.lineMatch[p]; !ok {
				d.lineMatch[p] = 1
			}
		}
	}
	positions := append(d.linePos[hash], n)
	if len(positions) > lineTrackedPositions {
		positions = positions[len(positions)-lineTrackedPositions:]
	}
	d.linePos[hash] = positions

	stage := stageFor(d.budgetFractionLocked())
	for p, matched := range d.lineMatch {
		copies := matched/p + 1
		if !d.lineLoopLocked(p, copies, stage) {
			continue
		}
		unitStart := len(d.lines) - p
		loopLines := copies * p
		if loopLines > len(d.lines) {
			loopLines = len(d.lines)
		}
		d.triggerRawLocked(
			strings.Join(d.lines[unitStart:], "\n"),
			d.lineStarts[len(d.lines)-loopLines],
		)
		return true
	}
	return false
}

func (d *reasoningLoopDetector) lineLoopLocked(period, copies int, stage loopStage) bool {
	if period == 1 {
		line := d.lines[len(d.lines)-1]
		needed := stage.lineCopies
		if len(line) < lineShortLine {
			// Very short lines such as "}" close nested structures in code
			// snippets and legitimately repeat; ask for much more evidence.
			needed = lineShortLineCopies
		}
		return copies >= needed
	}
	if copies < stage.blockCopies {
		return false
	}
	unit := d.lines[len(d.lines)-period:]
	distinct := make(map[string]struct{}, len(unit))
	chars := 0
	for _, line := range unit {
		distinct[strings.ToLower(line)] = struct{}{}
		chars += len(line)
	}
	// A block loop needs some substance: two lines alternating or a unit made
	// of a few characters is more likely quoted structure than a loop.
	return len(distinct) >= 2 && chars >= lineMinUnitChars
}

// --- shingle recurrence ---

func (d *reasoningLoopDetector) observeTokensLocked(line string) {
	tokens := normalizeReasoningTokens(line)
	if len(tokens) == 0 {
		return
	}
	stage := stageFor(d.budgetFractionLocked())
	for _, token := range tokens {
		d.tokenTail = append(d.tokenTail, token)
		if len(d.tokenTail) < shingleSize {
			continue
		}
		if len(d.tokenTail) > shingleSize {
			d.tokenTail = d.tokenTail[1:]
		}
		h := fnv.New64a()
		for _, t := range d.tokenTail {
			_, _ = h.Write([]byte(t))
			_, _ = h.Write([]byte{0})
		}
		hash := h.Sum64()
		_, repeated := d.shingleSeen[hash]
		d.shingleSeen[hash] = struct{}{}
		d.pushShingleLocked(repeated)

		if d.shingleLen < shingleWindow {
			continue
		}
		frac := float64(d.shingleRepeat) / float64(d.shingleLen)

		switch {
		case d.hardLen == 0:
			if frac >= shingleHardArm {
				d.hardLen = 1
				d.hardOffset = d.rawLineStart
			}
		case frac >= shingleHardArm-shingleHysteresis:
			d.hardLen++
			if d.hardLen >= shingleHardPlateau {
				d.triggerShingleLocked(d.hardOffset)
				return
			}
		default:
			d.hardLen = 0
		}

		switch {
		case d.plateauLen == 0:
			if frac >= stage.shingleArm {
				d.plateauLen = 1
				d.plateauOffset = d.rawLineStart
				d.armedAt = stage.shingleArm
			}
		case frac >= minFloat(d.armedAt, stage.shingleArm)-shingleHysteresis:
			d.plateauLen++
			if d.plateauLen >= stage.shinglePlateau {
				d.triggerShingleLocked(d.plateauOffset)
				return
			}
		default:
			d.plateauLen = 0
		}
	}
}

func (d *reasoningLoopDetector) triggerShingleLocked(offset int) {
	raw := d.raw.String()
	if offset > len(raw) {
		offset = len(raw)
	}
	d.triggerRawLocked(raw[offset:], offset)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (d *reasoningLoopDetector) pushShingleLocked(repeated bool) {
	if d.shingleLen < shingleWindow {
		d.shingleRing[(d.shingleStart+d.shingleLen)%shingleWindow] = repeated
		d.shingleLen++
	} else {
		if d.shingleRing[d.shingleStart] {
			d.shingleRepeat--
		}
		d.shingleRing[d.shingleStart] = repeated
		d.shingleStart = (d.shingleStart + 1) % shingleWindow
	}
	if repeated {
		d.shingleRepeat++
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

// --- provider repeated chunk ---

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
	d.repeatedChunk = true
	d.triggerRawLocked(chunk, d.raw.Len())
	return true
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

// --- triggering ---

// triggerRawLocked fires with the repeated content and the byte offset into
// the raw stream where the loop is considered to start.
func (d *reasoningLoopDetector) triggerRawLocked(repeatedContent string, loopStartOffset int) {
	d.detected = true
	d.repeatedContent = repeatedContent
	raw := d.raw.String()
	if loopStartOffset < 0 {
		loopStartOffset = 0
	}
	if loopStartOffset > len(raw) {
		loopStartOffset = len(raw)
	}
	// Offsets derived from rune counts can land inside a multi-byte rune;
	// snap to the next boundary so the reported split stays valid UTF-8.
	for loopStartOffset < len(raw) && !utf8.RuneStart(raw[loopStartOffset]) {
		loopStartOffset++
	}
	d.loopStartContent = strings.TrimRight(raw[:loopStartOffset], "\n")
	d.cancel()
}
