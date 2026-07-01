package llm

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestReasoningLoopDetectorRepeatedLine(t *testing.T) {
	d, canceled := newTestReasoningLoopDetector()
	for range 5 {
		d.onDelta("same thought\n")
	}
	if d.Detected() {
		t.Fatal("repeated line loop detected before allowed repeats were exhausted")
	}
	d.onDelta("same thought\n")
	if !d.Detected() {
		t.Fatal("expected repeated line loop")
	}
	if !*canceled {
		t.Fatal("expected detector to cancel stream")
	}
}

func TestReasoningLoopDetectorRepeatedRuneRate(t *testing.T) {
	d, canceled := newTestReasoningLoopDetector()
	for range loopRepeatedRuneMinCount - 1 {
		d.onDelta("\n")
	}
	if d.Detected() {
		t.Fatal("repeated rune loop detected before threshold")
	}
	d.onDelta("\n")
	if !d.Detected() {
		t.Fatal("expected repeated rune loop")
	}
	if !*canceled {
		t.Fatal("expected detector to cancel stream")
	}
	err := d.MakeError()
	if strings.Count(err.RepeatedContent, "\n") != loopRepeatedRuneMinCount {
		t.Fatalf("repeated content = %q, want %d newlines", err.RepeatedContent, loopRepeatedRuneMinCount)
	}
	if err.RepeatedChunk {
		t.Fatal("reasoning-content loop should not be flagged as a repeated chunk")
	}
	if got, want := err.Error(), "llm: reasoning loop detected during streaming"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReasoningLoopDetectorIgnoresFormattingRunes(t *testing.T) {
	for _, repeated := range []string{" ", "-", "=", "*", ">", "_", "|"} {
		t.Run(fmt.Sprintf("%q", repeated), func(t *testing.T) {
			d, _ := newTestReasoningLoopDetector()
			for range loopRepeatedRuneWindowSize * 2 {
				d.onDelta(repeated)
			}
			if d.Detected() {
				t.Fatalf("detected repeated rune loop for %q", repeated)
			}
		})
	}
}

func TestReasoningLoopDetectorProviderRepeatedChunkError(t *testing.T) {
	d, canceled := newTestReasoningLoopDetector()
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
	d, _ := newTestReasoningLoopDetector()
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

func TestReasoningLoopDetectorLongExactBlock(t *testing.T) {
	d, _ := newTestReasoningLoopDetector()
	for repeat := range 5 {
		for i := range 55 {
			d.onDelta(fmt.Sprintf("line %02d has useful reasoning\n", i))
		}
		if d.Detected() {
			t.Fatalf("exact block loop detected too early after copy %d", repeat+1)
		}
	}
	for i := range 55 {
		d.onDelta(fmt.Sprintf("line %02d has useful reasoning\n", i))
	}
	if !d.Detected() {
		t.Fatal("expected exact long block loop")
	}
}

func TestReasoningLoopDetectorFuzzyDecisionCycle(t *testing.T) {
	d, _ := newTestReasoningLoopDetector()
	d.onDelta("Initial analysis before the cycle begins.\n")
	cycles := [][3]string{
		{"AddSession", "DropSession", "Close"},
		{"DropSession", "DropPod", "Close"},
		{"CreateSession", "DeleteSession", "Close"},
		{"OpenSession", "RemoveSession", "Close"},
		{"StartSession", "StopSession", "Close"},
	}
	detectedAt := 0
	for i, cycle := range cycles {
		for _, line := range fuzzyReasoningCycle(cycle[0], cycle[1], cycle[2]) {
			d.onDelta(line + "\n")
		}
		if d.Detected() {
			detectedAt = i + 1
			break
		}
	}
	if !d.Detected() {
		for _, line := range fuzzyReasoningCycle("MakeSession", "ClearSession", "Close") {
			d.onDelta(line + "\n")
		}
	}
	if !d.Detected() {
		t.Fatal("expected fuzzy decision loop")
	}
	if detectedAt > 0 && detectedAt >= len(cycles) {
		t.Fatalf("fuzzy loop detected after %d cycles, want earlier than old long-window behavior", detectedAt)
	}
	err := d.MakeError()
	if !strings.Contains(err.RepeatedContent, "Finding") {
		t.Fatalf("repeated content missing finding: %q", err.RepeatedContent)
	}
}

func TestReasoningLoopDetectorShortFuzzyDecisionCycleTriggersBeforeOldFloor(t *testing.T) {
	d, _ := newTestReasoningLoopDetectorAtProgress(0.35)
	lines := []string{"Initial analysis before compact fuzzy cycles begin."}
	for _, cycle := range [][3]string{
		{"AddSession", "DropSession", "Close"},
		{"DropSession", "DropPod", "Close"},
		{"CreateSession", "DeleteSession", "Close"},
		{"OpenSession", "RemoveSession", "Close"},
	} {
		lines = append(lines, shortFuzzyReasoningCycle(cycle[0], cycle[1], cycle[2])...)
	}
	for i, line := range lines {
		d.onDelta(line + "\n")
		if d.Detected() {
			if i+1 >= 168 {
				t.Fatalf("short fuzzy loop detected after %d lines, want before old 168-line floor", i+1)
			}
			return
		}
	}
	t.Fatal("expected short fuzzy loop")
}

func TestReasoningLoopDetectorSemanticOverthinkingLoop(t *testing.T) {
	data, err := os.ReadFile("testdata/semantic_overthinking_excerpt.txt")
	if err != nil {
		t.Fatal(err)
	}
	d, canceled := newTestReasoningLoopDetectorAtProgress(0.35)
	d.onDelta(string(data))
	if !d.Detected() {
		t.Fatal("expected semantic overthinking loop")
	}
	if !*canceled {
		t.Fatal("expected detector to cancel stream")
	}
	errResult := d.MakeError()
	if !strings.Contains(errResult.RepeatedContent, "Actually, I should verify") {
		t.Fatalf("repeated content missing reopened analysis: %q", errResult.RepeatedContent)
	}
}

func TestReasoningLoopDetectorSensitivityStages(t *testing.T) {
	lines := strings.Join(oneReopenOverthinkingCycle(), "\n") + "\n"

	conservative, _ := newTestReasoningLoopDetectorAtProgress(0.10)
	conservative.onDelta(lines)
	if conservative.Detected() {
		t.Fatal("conservative sensitivity should not detect a single reopened cycle")
	}

	balanced, _ := newTestReasoningLoopDetectorAtProgress(0.35)
	balanced.onDelta(lines)
	if balanced.Detected() {
		t.Fatal("balanced sensitivity should not detect a single reopened cycle")
	}

	aggressive, _ := newTestReasoningLoopDetectorAtProgress(0.75)
	aggressive.onDelta(lines)
	if !aggressive.Detected() {
		t.Fatal("aggressive sensitivity should detect a dense single reopened cycle")
	}
}

func TestReasoningLoopDetectorIgnoresLinearReviewReasoning(t *testing.T) {
	d, _ := newTestReasoningLoopDetectorAtProgress(0.75)
	lines := []string{
		"I need to verify the changed request path before deciding.",
		"Looking at the handler shows the new branch validates the input first.",
		"The storage layer receives a typed object and does not parse raw text.",
		"The concurrency path uses the existing mutex and does not add shared state.",
		"The error path wraps the lower-level error with enough context.",
		"The tests cover the successful path and one validation failure.",
		"The remaining uncovered branch is minor and not part of this finding.",
		"The verdict is confirmed because the changed behavior is bounded.",
		"Priority is low because the risk is limited to diagnostics.",
		"The finding is ready with a concrete recommendation.",
		"Therefore the review can finalize without more analysis.",
	}
	d.onDelta(strings.Join(lines, "\n") + "\n")
	if d.Detected() {
		t.Fatal("linear reasoning should not trigger semantic loop detection")
	}
}

func TestReasoningLoopDetectorIgnoresShortRepeatedHeadings(t *testing.T) {
	d, _ := newTestReasoningLoopDetector()
	for range 200 {
		d.onDelta("**Priority: 2**\n")
		d.onDelta("**Suggestion**\n")
	}
	if d.Detected() {
		t.Fatal("short repeated headings should not trigger fuzzy loop detection")
	}
}

func TestReasoningLoopDetectorWaitsForCompletedLines(t *testing.T) {
	d, _ := newTestReasoningLoopDetector()
	for range 5 {
		d.onDelta("same thought\n")
	}
	d.onDelta("same thought")
	if d.Detected() {
		t.Fatal("partial final line should not complete loop")
	}
	d.onDelta("\n")
	if !d.Detected() {
		t.Fatal("expected loop after final newline")
	}
}

func newTestReasoningLoopDetector() (*reasoningLoopDetector, *bool) {
	return newTestReasoningLoopDetectorAtProgress(0)
}

func newTestReasoningLoopDetectorAtProgress(progress float64) (*reasoningLoopDetector, *bool) {
	canceled := false
	d := newReasoningLoopDetectorWithProgress(func() {
		canceled = true
	}, 4, func() float64 {
		return progress
	})
	return d, &canceled
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

func shortFuzzyReasoningCycle(first, second, closer string) []string {
	return []string{
		fmt.Sprintf("Actually, I need to verify whether `%s` and `%s` are in this patch before I finalize the finding.", first, second),
		fmt.Sprintf("The main issue is that `%s` can race with concurrent lifecycle calls, but I need to separate new behavior from pre existing behavior.", closer),
		"Let me formulate the finding with enough precision so the reviewer can act on it.",
		fmt.Sprintf("**Finding: %s does not guard concurrent lifecycle calls**", closer),
		fmt.Sprintf("The `%s` method closes shared state while `%s` and `%s` can still try to use it.", closer, first, second),
		fmt.Sprintf("If these methods run after `%s` releases control, they can observe closed resources and return cleanup errors.", closer),
		"**Priority: 2**",
		fmt.Sprintf("**Suggestion**: Add an explicit closed flag and check it before `%s` or `%s` uses shared state.", first, second),
	}
}

func oneReopenOverthinkingCycle() []string {
	return []string{
		"I need to verify the changed cache lifecycle before deciding.",
		"Looking at the close path shows the shared resource is released after cancellation.",
		"The main issue is whether concurrent calls can still enter after close starts.",
		"**Finding: Close can race with concurrent lifecycle calls**",
		"Priority is moderate because the failure only appears during shutdown.",
		"Let me finalize the finding with a concrete recommendation.",
		"Actually, I should verify the helper path before I submit this.",
		"However, the helper path uses the same shared resource after close.",
		"I need to check whether this is pre existing or introduced by the patch.",
		"Looking at the new code confirms the changed close path introduced the ordering.",
		"The verdict is confirmed because the patch changes the lifecycle boundary.",
		"The finding is still ready and the fix remains an explicit closed guard.",
	}
}
