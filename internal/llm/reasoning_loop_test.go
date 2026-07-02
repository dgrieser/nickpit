package llm

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// newTestReasoningLoopDetector returns a detector with a controllable clock.
// The returned advance function moves the simulated stream time to the given
// fraction of the (default 300s) staging budget.
func newTestReasoningLoopDetector() (*reasoningLoopDetector, *bool, func(fraction float64)) {
	canceled := false
	d := newReasoningLoopDetector(func() {
		canceled = true
	}, 0)
	base := time.Unix(0, 0)
	clock := base
	d.now = func() time.Time { return clock }
	// Fix the stream start so advancing works before the first delta arrives.
	d.start = base
	advance := func(fraction float64) {
		clock = base.Add(time.Duration(fraction * float64(d.budget)))
	}
	return d, &canceled, advance
}

func TestReasoningLoopDetectorRepeatedLineEarly(t *testing.T) {
	d, canceled, _ := newTestReasoningLoopDetector()
	needed := loopStages[0].lineCopies
	for range needed - 1 {
		d.onDelta("the same thought again\n")
	}
	if d.Detected() {
		t.Fatal("repeated line loop detected before the early-stage allowance was exhausted")
	}
	d.onDelta("the same thought again\n")
	if !d.Detected() {
		t.Fatal("expected repeated line loop")
	}
	if !*canceled {
		t.Fatal("expected detector to cancel stream")
	}
	err := d.MakeError()
	if !strings.Contains(err.RepeatedContent, "the same thought again") {
		t.Fatalf("repeated content = %q", err.RepeatedContent)
	}
	if err.RepeatedChunk {
		t.Fatal("reasoning-content loop should not be flagged as a repeated chunk")
	}
	if got, want := err.Error(), "llm: reasoning loop detected during streaming"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReasoningLoopDetectorRepeatedLineIgnoresWhitespace(t *testing.T) {
	d, _, advance := newTestReasoningLoopDetector()
	advance(0.9) // late stage: fewest copies needed
	needed := loopStages[len(loopStages)-1].lineCopies
	variants := []string{
		"  the same thought again\n",
		"the same    thought again  \n",
		"\tthe same thought\tagain\n",
		"\n", // empty lines must not break the run
	}
	for i := range needed {
		d.onDelta(variants[i%len(variants)])
		d.onDelta("the same thought again\n")
	}
	if !d.Detected() {
		t.Fatal("expected whitespace-insensitive repeated line loop")
	}
}

func TestReasoningLoopDetectorStagingLowersLineCopies(t *testing.T) {
	early := loopStages[0].lineCopies
	late := loopStages[len(loopStages)-1].lineCopies
	if late >= early {
		t.Fatalf("staging must relax thresholds over time: early=%d late=%d", early, late)
	}

	d, _, advance := newTestReasoningLoopDetector()
	advance(0.9)
	for range late - 1 {
		d.onDelta("another repeating conclusion\n")
	}
	if d.Detected() {
		t.Fatal("late-stage line loop detected below the late allowance")
	}
	d.onDelta("another repeating conclusion\n")
	if !d.Detected() {
		t.Fatal("expected late-stage line loop at the reduced allowance")
	}
}

func TestReasoningLoopDetectorRepeatedBlock(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	block := "first line of the repeated reasoning block\nsecond line with more detail\nthird line drawing the conclusion\n"
	needed := loopStages[0].blockCopies
	for range needed - 1 {
		d.onDelta(block)
	}
	if d.Detected() {
		t.Fatal("block loop detected before the allowance was exhausted")
	}
	d.onDelta(block)
	if !d.Detected() {
		t.Fatal("expected repeated block loop")
	}
}

func TestReasoningLoopDetectorLongExactBlock(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	emitBlock := func() {
		for i := range 55 {
			d.onDelta(fmt.Sprintf("line %02d has useful reasoning about a specific topic\n", i))
		}
	}
	emitBlock()
	if d.Detected() {
		t.Fatal("a single block emission is novel content, not a loop")
	}
	// A long verbatim block re-emitted a few times must fire; with 55 lines
	// per copy the near-verbatim shingle tier catches it within a few copies.
	for copies := 2; copies <= loopStages[0].blockCopies; copies++ {
		emitBlock()
		if d.Detected() {
			return
		}
	}
	t.Fatal("expected long exact block loop")
}

func TestReasoningLoopDetectorShortClosingLinesNeedMoreCopies(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	// Nested code snippets legitimately end in runs of closing braces.
	for range lineShortLineCopies - 1 {
		d.onDelta("}\n")
	}
	if d.Detected() {
		t.Fatal("short closing lines should get a higher repetition allowance")
	}
}

func TestReasoningLoopDetectorRepeatedRune(t *testing.T) {
	d, canceled, _ := newTestReasoningLoopDetector()
	d.onDelta(strings.Repeat("a", charTriggerSpan+32))
	if !d.Detected() {
		t.Fatal("expected repeated rune loop")
	}
	if !*canceled {
		t.Fatal("expected detector to cancel stream")
	}
}

func TestReasoningLoopDetectorNewlineFlood(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	d.onDelta(strings.Repeat("\n", charTriggerSpan+32))
	if !d.Detected() {
		t.Fatal("expected newline flood to trigger")
	}
}

func TestReasoningLoopDetectorShortPeriodRun(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	d.onDelta(strings.Repeat("ab", charTriggerSpan))
	if !d.Detected() {
		t.Fatal("expected short-period character run to trigger")
	}
}

func TestReasoningLoopDetectorIgnoresFormattingRunes(t *testing.T) {
	for _, repeated := range []string{" ", "-", "=", "*", ">", "_", "|", "- ", "=-"} {
		t.Run(fmt.Sprintf("%q", repeated), func(t *testing.T) {
			d, _, _ := newTestReasoningLoopDetector()
			d.onDelta(strings.Repeat(repeated, charWindowSize*2))
			if d.Detected() {
				t.Fatalf("detected formatting run for %q", repeated)
			}
		})
	}
}

func TestReasoningLoopDetectorFuzzyParaphraseCycle(t *testing.T) {
	d, _, advance := newTestReasoningLoopDetector()
	advance(0.6) // paraphrase plateaus arm at the mid-stage threshold
	d.onDelta("Initial analysis before the cycle begins.\n")
	cycles := [][3]string{
		{"AddSession", "DropSession", "Close"},
		{"DropSession", "DropPod", "Close"},
		{"CreateSession", "DeleteSession", "Close"},
		{"OpenSession", "RemoveSession", "Close"},
		{"StartSession", "StopSession", "Close"},
		{"MakeSession", "ClearSession", "Close"},
		{"BuildSession", "TearDownSession", "Close"},
		{"NewSession", "EndSession", "Close"},
	}
	for _, cycle := range cycles {
		if d.Detected() {
			break
		}
		for _, line := range fuzzyReasoningCycle(cycle[0], cycle[1], cycle[2]) {
			d.onDelta(line + "\n")
		}
	}
	if !d.Detected() {
		t.Fatal("expected fuzzy paraphrase loop")
	}
	err := d.MakeError()
	if err.RepeatedContent == "" {
		t.Fatal("expected repeated content to be reported")
	}
	if err.LoopStartContent == "" {
		t.Fatal("expected pre-loop content to be reported")
	}
}

func TestReasoningLoopDetectorFuzzyRequiresPlateau(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	// Distinct content, even with recurring vocabulary, must not fire early.
	// Numbers are masked during normalization, so novelty must come from
	// actual wording, as it does in real reasoning.
	for i := range 400 {
		d.onDelta(fmt.Sprintf("Reviewing function %s which handles a distinct concern such as %s handling and interacts with the %s subsystem in its own specific way.\n",
			testWord(i), testWord(i*7+3), testWord(i*13+5)))
	}
	if d.Detected() {
		t.Fatal("novel reasoning should not trigger the fuzzy detector")
	}
}

func TestReasoningLoopDetectorMaskedNumbersStillLoop(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	// The same sentence with only counters changing is a loop even though no
	// two raw lines are byte-identical.
	for i := range 200 {
		if d.Detected() {
			return
		}
		d.onDelta(fmt.Sprintf("Analyzing item %d with a fresh perspective on concern number %d and some unique detail %d.\n", i, i*3, i*5))
	}
	t.Fatal("expected number-masked repetition to trigger the fuzzy detector")
}

func TestReasoningLoopDetectorIgnoresShortRepeatedHeadings(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	for i := range 120 {
		d.onDelta(fmt.Sprintf("Analyzing the %s topic with a fresh perspective on the %s concern and its unique %s detail.\n",
			testWord(i), testWord(i*3+1), testWord(i*5+2)))
		d.onDelta("**Priority: 2**\n")
	}
	if d.Detected() {
		t.Fatal("interleaved repeated headings should not trigger loop detection")
	}
}

// testWord returns distinct pronounceable words so fixtures vary by wording
// rather than by numbers, which token normalization masks.
func testWord(i int) string {
	consonants := []string{"b", "d", "f", "g", "k", "l", "m", "n", "p", "r", "s", "t"}
	vowels := []string{"a", "e", "i", "o", "u"}
	var b strings.Builder
	for range 3 {
		b.WriteString(consonants[i%len(consonants)])
		i /= len(consonants)
		b.WriteString(vowels[i%len(vowels)])
		i /= len(vowels)
	}
	return b.String()
}

func TestReasoningLoopDetectorWaitsForCompletedLines(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	needed := loopStages[0].lineCopies
	for range needed - 1 {
		d.onDelta("the same thought again\n")
	}
	d.onDelta("the same thought again") // no trailing newline yet
	if d.Detected() {
		t.Fatal("partial final line should not complete the loop")
	}
	d.onDelta("\n")
	if !d.Detected() {
		t.Fatal("expected loop after final newline")
	}
}

func TestReasoningLoopDetectorProviderRepeatedChunkError(t *testing.T) {
	d, canceled, _ := newTestReasoningLoopDetector()
	err := errors.New("error, litellm.MidStreamFallbackError: litellm.InternalServerError: The model is repeating the same chunk = \n\n\n.. Received Model Group=Qwen3.5-122B-A10B-FP8")
	if !d.detectRepeatedChunkError(err) {
		t.Fatal("expected provider repeated chunk error to trigger loop detector")
	}
	if !*canceled {
		t.Fatal("expected detector to cancel stream")
	}
	if got := strings.Count(d.MakeError().RepeatedContent, "\n"); got != 3 {
		t.Fatalf("repeated content newline count = %d, want 3", got)
	}
	loopErr := d.MakeError()
	if !loopErr.RepeatedChunk {
		t.Fatal("provider repeated-chunk error should set RepeatedChunk")
	}
	if got, want := loopErr.Error(), "llm: model repeated output chunk during streaming"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReasoningLoopDetectorProviderRepeatedChunkErrorIncludesPartialLine(t *testing.T) {
	d, _, _ := newTestReasoningLoopDetector()
	d.onDelta("completed reasoning\npartial reasoning before provider error")
	err := errors.New("error, litellm.InternalServerError: The model is repeating the same chunk = repeated chunk.. Received Model Group=Qwen3.5")
	if !d.detectRepeatedChunkError(err) {
		t.Fatal("expected provider repeated chunk error to trigger loop detector")
	}
	if got, want := d.MakeError().LoopStartContent, "completed reasoning\npartial reasoning before provider error"; got != want {
		t.Fatalf("loop start content = %q, want %q", got, want)
	}
	if !d.MakeError().RepeatedChunk {
		t.Fatal("provider repeated-chunk error should set RepeatedChunk")
	}
}

func TestRepeatedChunkFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
		ok   bool
	}{
		{
			name: "missing received model group suffix",
			err:  errors.New("error: The model is repeating the same chunk = truncated repeated chunk"),
			want: "truncated repeated chunk",
			ok:   true,
		},
		{
			name: "escaped newline",
			err:  errors.New(`error: The model is repeating the same chunk = first\nsecond.. Received Model Group=Qwen3.5`),
			want: "first\nsecond",
			ok:   true,
		},
		{
			name: "empty after trim",
			err:  errors.New("error: The model is repeating the same chunk = ... Received Model Group=Qwen3.5"),
			ok:   false,
		},
		{
			name: "ellipsis trim",
			err:  errors.New("error: The model is repeating the same chunk = \n\n\n... Received Model Group=Qwen3.5"),
			want: "\n\n\n",
			ok:   true,
		},
		{
			name: "dots with trailing tab trim",
			err:  errors.New("error: The model is repeating the same chunk = \n.. \tReceived Model Group=Qwen3.5"),
			want: "\n",
			ok:   true,
		},
		{
			name: "available marker before received marker",
			err:  errors.New("error: The model is repeating the same chunk = repeated chunk.. Available Model Group Fallbacks=None Received Model Group=Qwen3.5"),
			want: "repeated chunk",
			ok:   true,
		},
		{
			name: "no marker",
			err:  errors.New("error: provider failed"),
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := repeatedChunkFromError(tt.err)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("chunk = %q, want %q", got, tt.want)
			}
		})
	}
}

func fuzzyReasoningCycle(first, second, closer string) []string {
	lines := []string{
		fmt.Sprintf("Actually, I need to check whether `%s` and `%s` are truly part of this patch before I finalize the review.", first, second),
		fmt.Sprintf("The main issue is a potential race condition between `%s` and concurrent methods, but I need to separate new behavior from pre existing behavior.", closer),
		"Let me formulate the finding with enough precision so the reviewer can act on it.",
		fmt.Sprintf("**Finding: %s does not prevent concurrent %s or %s calls**", closer, first, second),
		fmt.Sprintf("The `%s` method cancels all extenders and closes the Redis client, but it does not prevent concurrent calls to `%s` or `%s`.", closer, first, second),
		fmt.Sprintf("If these methods run after `%s` releases the mutex, they can try to use the closed Redis client and surface errors during cleanup.", closer),
		"**Priority: 2**",
		fmt.Sprintf("**Suggestion**: Add a `closed` flag that is set in `%s` while holding the mutex, then check this flag before using Redis.", closer),
		"Wait, I need to reconsider the priority because shutdown ordering may mean callers normally stop creating new sessions first.",
		"However, from a defensive programming perspective, the guard is still better because it makes the lifecycle explicit.",
		"Let me also check if there are other issues introduced by the patch.",
		fmt.Sprintf("1. Missing error handling in `%s` and `%s` is pre existing, not introduced by the patch.", first, second),
		"2. Potential goroutine leak is pre existing, not introduced by the patch.",
		"3. Test coverage gap is real but secondary to the lifecycle race.",
		"4. Architecture issue with context ownership is pre existing, not introduced by the patch.",
		fmt.Sprintf("So the main issue introduced by the patch is the potential race condition between `%s` and concurrent methods.", closer),
		"Let me finalize the finding.",
		fmt.Sprintf("**Finding: %s does not prevent concurrent %s or %s calls**", closer, first, second),
		fmt.Sprintf("The `%s` method cancels all extenders and closes the Redis client, but it does not prevent concurrent calls to `%s` or `%s`.", closer, first, second),
		fmt.Sprintf("If these methods are called after `%s` returns, they might try to use the closed Redis client, leading to errors.", closer),
		"**Priority: 2**",
		fmt.Sprintf("**Suggestion**: Add a `closed` flag to the cache struct and check it in `%s` and `%s` before trying to use the Redis client.", first, second),
		"Actually, I think I have been overthinking this, but the fix remains the same and the finding should be submitted.",
		"The better solution is still to make the closed state explicit and avoid letting cleanup methods touch Redis after shutdown.",
	}
	return lines
}
