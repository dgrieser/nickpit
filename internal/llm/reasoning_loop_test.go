package llm

import (
	"fmt"
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
	for _, cycle := range cycles {
		for _, line := range fuzzyReasoningCycle(cycle[0], cycle[1], cycle[2]) {
			d.onDelta(line + "\n")
		}
		if d.Detected() {
			t.Fatal("fuzzy decision loop detected before allowed repeats were exhausted")
		}
	}
	for _, line := range fuzzyReasoningCycle("MakeSession", "ClearSession", "Close") {
		d.onDelta(line + "\n")
	}
	if !d.Detected() {
		t.Fatal("expected fuzzy decision loop")
	}
	err := d.MakeError()
	if !strings.Contains(err.RepeatedContent, "Finding") {
		t.Fatalf("repeated content missing finding: %q", err.RepeatedContent)
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
	cycleA := strings.Join(fuzzyReasoningCycle("AddSession", "DropSession", "Close"), "\n")
	d.onDelta(cycleA + "\n")
	for _, cycle := range [][3]string{
		{"DropSession", "DropPod", "Close"},
		{"CreateSession", "DeleteSession", "Close"},
		{"OpenSession", "RemoveSession", "Close"},
		{"StartSession", "StopSession", "Close"},
	} {
		d.onDelta(strings.Join(fuzzyReasoningCycle(cycle[0], cycle[1], cycle[2]), "\n") + "\n")
	}
	cycleB := strings.Join(fuzzyReasoningCycle("MakeSession", "ClearSession", "Close"), "\n")
	d.onDelta(cycleB)
	if d.Detected() {
		t.Fatal("partial final line should not complete fuzzy loop")
	}
	d.onDelta("\n")
	if !d.Detected() {
		t.Fatal("expected fuzzy loop after final newline")
	}
}

func newTestReasoningLoopDetector() (*reasoningLoopDetector, *bool) {
	canceled := false
	d := newReasoningLoopDetector(func() {
		canceled = true
	}, 4)
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
