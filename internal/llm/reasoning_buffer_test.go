package llm

import (
	"strings"
	"sync"
	"testing"
)

func TestBufferedReasoningSinkAppendStringReset(t *testing.T) {
	var sink BufferedReasoningSink
	sink.Append("first")
	sink.Append(" second")
	if got, want := sink.String(), "first second"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	sink.Reset()
	if got := sink.String(); got != "" {
		t.Fatalf("String() after Reset() = %q, want empty", got)
	}
}

func TestBufferedReasoningSinkConcurrentAppend(t *testing.T) {
	var sink BufferedReasoningSink
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			sink.Append("x")
		})
	}
	wg.Wait()
	if got, want := len(sink.String()), 50; got != want {
		t.Fatalf("buffer length = %d, want %d", got, want)
	}
}

func TestTeeReasoningSinks(t *testing.T) {
	var left, right BufferedReasoningSink
	sink := TeeReasoningSinks(nil, &left, &right)
	sink.Append("delta")
	sink.End()
	for name, got := range map[string]string{
		"left":  left.String(),
		"right": right.String(),
	} {
		if !strings.Contains(got, "delta") {
			t.Fatalf("%s sink = %q, want delta", name, got)
		}
	}
	if got := TeeReasoningSinks(nil); got != nil {
		t.Fatalf("nil-only tee = %#v, want nil", got)
	}
}
