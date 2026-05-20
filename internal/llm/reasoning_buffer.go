package llm

import (
	"strings"
	"sync"
)

type BufferedReasoningSink struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *BufferedReasoningSink) Append(delta string) {
	if b == nil || delta == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.WriteString(delta)
}

func (b *BufferedReasoningSink) End() {}

func (b *BufferedReasoningSink) String() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *BufferedReasoningSink) Reset() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

type teeReasoningSink struct {
	sinks []ReasoningSink
}

func TeeReasoningSinks(sinks ...ReasoningSink) ReasoningSink {
	filtered := make([]ReasoningSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return &teeReasoningSink{sinks: filtered}
	}
}

func (t *teeReasoningSink) Append(delta string) {
	if t == nil {
		return
	}
	for _, sink := range t.sinks {
		sink.Append(delta)
	}
}

func (t *teeReasoningSink) End() {
	if t == nil {
		return
	}
	for _, sink := range t.sinks {
		sink.End()
	}
}
