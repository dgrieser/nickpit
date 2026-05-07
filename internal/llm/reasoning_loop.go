package llm

import (
	"context"
	"strings"
	"sync"
)

const (
	loopRepeatLineThreshold = 5   // same non-empty line N times in a row
	loopBlockMinLines       = 2   // minimum block size for block repetition
	loopBlockMaxLines       = 20  // max block size checked (caps O(n) work per line)
)

// ReasoningLoopDetectedError is returned when the model's streaming reasoning
// content repeats itself, indicating it has entered an infinite loop.
type ReasoningLoopDetectedError struct {
	ReasoningEffort  string
	LoopStartContent string // reasoning before the loop began
	RepeatedContent  string // the repeating line(s)
}

func (e *ReasoningLoopDetectedError) Error() string {
	return "llm: reasoning loop detected during streaming"
}

type reasoningLoopDetector struct {
	mu               sync.Mutex
	cancel           context.CancelFunc
	detected         bool
	loopStartContent string
	repeatedContent  string
	lines            []string
	currentLine      strings.Builder
}

func newReasoningLoopDetector(cancel context.CancelFunc) *reasoningLoopDetector {
	return &reasoningLoopDetector{cancel: cancel}
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
	}
}

func (d *reasoningLoopDetector) onDelta(delta string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.detected {
		return
	}
	for _, ch := range delta {
		if ch == '\n' {
			line := d.currentLine.String()
			d.currentLine.Reset()
			d.lines = append(d.lines, line)
			if d.checkLoopLocked() {
				return
			}
		} else {
			d.currentLine.WriteRune(ch)
		}
	}
}

func (d *reasoningLoopDetector) checkLoopLocked() bool {
	n := len(d.lines)

	// Strategy 1: same non-empty line repeated loopRepeatLineThreshold times consecutively.
	if n >= loopRepeatLineThreshold {
		last := d.lines[n-1]
		if strings.TrimSpace(last) != "" {
			allSame := true
			for i := n - loopRepeatLineThreshold; i < n-1; i++ {
				if d.lines[i] != last {
					allSame = false
					break
				}
			}
			if allSame {
				d.trigger(strings.Join(d.lines[n-loopRepeatLineThreshold:], "\n"), n-loopRepeatLineThreshold)
				return true
			}
		}
	}

	// Strategy 2: block of k lines appearing twice consecutively.
	maxK := n / 2
	if maxK > loopBlockMaxLines {
		maxK = loopBlockMaxLines
	}
	for k := loopBlockMinLines; k <= maxK; k++ {
		b1 := n - 2*k
		b2 := n - k
		match := true
		for i := 0; i < k; i++ {
			if d.lines[b1+i] != d.lines[b2+i] {
				match = false
				break
			}
		}
		if match {
			d.trigger(strings.Join(d.lines[b2:], "\n"), b1)
			return true
		}
	}
	return false
}

func (d *reasoningLoopDetector) trigger(repeatedContent string, loopStartLine int) {
	d.detected = true
	d.repeatedContent = repeatedContent
	if loopStartLine > 0 {
		d.loopStartContent = strings.Join(d.lines[:loopStartLine], "\n")
	}
	d.cancel()
}
