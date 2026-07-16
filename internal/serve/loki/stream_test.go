package loki

import (
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

// collectLines returns every line text pushed to the capturing server, in
// order across batches.
func collectLines(reqs []captured) []string {
	var out []string
	for _, r := range reqs {
		for _, s := range r.body.Streams {
			for _, v := range s.Values {
				out = append(out, v[1])
			}
		}
	}
	return out
}

func TestPartialLineReassembledAcrossWrites(t *testing.T) {
	srv, getReqs := capturingServer(t, 204)
	client := NewClient(Config{URL: srv.URL}, discardLogger())
	stream := client.NewStream(nil)

	_, _ = io.WriteString(stream, "part")
	_, _ = io.WriteString(stream, "ial line\nnext\n")
	_ = stream.Close()

	lines := collectLines(getReqs())
	if len(lines) != 2 || lines[0] != "partial line" || lines[1] != "next" {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestTrailingUnterminatedLineFlushedOnClose(t *testing.T) {
	srv, getReqs := capturingServer(t, 204)
	client := NewClient(Config{URL: srv.URL}, discardLogger())
	stream := client.NewStream(nil)

	_, _ = io.WriteString(stream, "no trailing newline")
	// Before Close nothing should be forced out (there is no complete line).
	if got := collectLines(getReqs()); len(got) != 0 {
		t.Fatalf("expected no push before close, got %#v", got)
	}
	_ = stream.Close()

	lines := collectLines(getReqs())
	if len(lines) != 1 || lines[0] != "no trailing newline" {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestCarriageReturnStripped(t *testing.T) {
	srv, getReqs := capturingServer(t, 204)
	client := NewClient(Config{URL: srv.URL}, discardLogger())
	stream := client.NewStream(nil)

	_, _ = io.WriteString(stream, "windows line\r\nunix line\n")
	_ = stream.Close()

	lines := collectLines(getReqs())
	if len(lines) != 2 || lines[0] != "windows line" || lines[1] != "unix line" {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestOversizedLineForceFlushed(t *testing.T) {
	srv, getReqs := capturingServer(t, 204)
	client := NewClient(Config{URL: srv.URL}, discardLogger())
	stream := client.NewStream(nil)

	big := strings.Repeat("x", maxLineBytes+10) // no newline
	_, _ = io.WriteString(stream, big)
	_ = stream.Close()

	lines := collectLines(getReqs())
	if len(lines) != 1 || len(lines[0]) != len(big) {
		t.Fatalf("oversized line not force-flushed: got %d lines, first len %d", len(lines), func() int {
			if len(lines) == 0 {
				return -1
			}
			return len(lines[0])
		}())
	}
}

func TestBatchingByMaxLines(t *testing.T) {
	srv, getReqs := capturingServer(t, 204)
	// Large BatchWait so only the count threshold or Close can flush.
	client := NewClient(Config{URL: srv.URL, BatchMaxLines: 2, BatchWait: time.Minute}, discardLogger())
	stream := client.NewStream(nil)

	for _, l := range []string{"a", "b", "c", "d"} {
		_, _ = io.WriteString(stream, l+"\n")
	}
	_ = stream.Close()

	reqs := getReqs()
	if got := collectLines(reqs); len(got) != 4 {
		t.Fatalf("expected 4 lines total, got %#v", got)
	}
	for _, r := range reqs {
		for _, s := range r.body.Streams {
			if len(s.Values) > 2 {
				t.Fatalf("batch exceeded BatchMaxLines: %d", len(s.Values))
			}
		}
	}
}

func TestBatchingByWaitTimer(t *testing.T) {
	srv, getReqs := capturingServer(t, 204)
	client := NewClient(Config{URL: srv.URL, BatchMaxLines: 1000, BatchWait: 40 * time.Millisecond}, discardLogger())
	stream := client.NewStream(nil)
	defer func() { _ = stream.Close() }()

	_, _ = io.WriteString(stream, "solo line\n")

	// The batch-wait timer should flush the single line without a Close.
	deadline := time.After(2 * time.Second)
	for {
		if len(collectLines(getReqs())) == 1 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("batch-wait timer did not flush the line")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestTimestampsMonotonic(t *testing.T) {
	srv, getReqs := capturingServer(t, 204)
	client := NewClient(Config{URL: srv.URL, BatchWait: time.Minute}, discardLogger())
	stream := client.NewStream(nil)

	for i := 0; i < 200; i++ {
		_, _ = io.WriteString(stream, "line\n")
	}
	_ = stream.Close()

	var prev int64
	var count int
	for _, r := range getReqs() {
		for _, s := range r.body.Streams {
			for _, v := range s.Values {
				ts, err := strconv.ParseInt(v[0], 10, 64)
				if err != nil {
					t.Fatalf("bad timestamp %q: %v", v[0], err)
				}
				if count > 0 && ts <= prev {
					t.Fatalf("timestamps not strictly increasing: %d then %d", prev, ts)
				}
				prev = ts
				count++
			}
		}
	}
	if count != 200 {
		t.Fatalf("got %d lines, want 200", count)
	}
}

// A full buffer must drop lines (counted) rather than block, and Close must
// still complete cleanly.
func TestDropsWhenBufferFull(t *testing.T) {
	release := make(chan struct{})
	srv, _ := blockingServer(t, release)
	client := NewClient(Config{URL: srv.URL, BufferLines: 2, BatchMaxLines: 1, Timeout: time.Second}, discardLogger())
	stream := client.NewStream(nil)

	lw := stream.(*lineWriter)
	for i := 0; i < 500; i++ {
		_, _ = io.WriteString(stream, "line\n")
	}

	lw.mu.Lock()
	dropped := lw.dropped
	lw.mu.Unlock()
	if dropped == 0 {
		t.Fatal("expected some dropped lines with a stalled backend and tiny buffer")
	}

	close(release)
	done := make(chan struct{})
	go func() { _ = stream.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not complete")
	}
}

func TestCloseIdempotent(t *testing.T) {
	srv, _ := capturingServer(t, 204)
	client := NewClient(Config{URL: srv.URL}, discardLogger())
	stream := client.NewStream(nil)
	_, _ = io.WriteString(stream, "x\n")
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	// Second Close must not panic (double channel close) and must be a no-op.
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}
