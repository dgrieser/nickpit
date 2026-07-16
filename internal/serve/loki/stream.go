package loki

import (
	"bytes"
	"io"
	"strconv"
	"sync"
	"time"
)

// maxLineBytes caps a single unterminated line held in the partial buffer. A
// child that emits a huge run of bytes without a newline (e.g. a progress bar)
// is force-flushed at this size so the daemon can never be driven to OOM by
// child output.
const maxLineBytes = 1 << 20 // 1 MiB

// NewStream opens a per-review log stream. The returned writer splits child
// output into lines, batches them, and pushes them to Loki from a background
// goroutine. streamLabels are merged over the client's static labels (which
// include "app"); callers pass the low-cardinality per-review identity
// (project, iid, trigger). The caller MUST Close the writer exactly once to
// flush the tail and stop the goroutine.
func (c *Client) NewStream(streamLabels map[string]string) io.WriteCloser {
	labels := make(map[string]string, len(c.base)+len(streamLabels))
	for k, v := range c.base {
		labels[k] = v
	}
	for k, v := range streamLabels {
		labels[k] = v
	}
	w := &lineWriter{
		client: c,
		labels: labels,
		ch:     make(chan logLine, c.cfg.BufferLines),
	}
	w.wg.Add(1)
	go w.run()
	return w
}

type logLine struct {
	ts   int64
	text string
}

// lineWriter is the io.WriteCloser returned by NewStream. Its Write never
// blocks and never returns an error: lines are handed to a bounded channel
// with a non-blocking send, and dropped (counted) when the channel is full.
// This is what lets io.MultiWriter(logFile, stream) never fail the on-disk
// write when Loki is slow or down.
type lineWriter struct {
	client *Client
	labels map[string]string
	ch     chan logLine

	wg        sync.WaitGroup
	closeOnce sync.Once

	mu      sync.Mutex
	partial []byte // carry of an unterminated line across Write calls
	lastTS  int64  // last stamped timestamp, kept monotonic non-decreasing
	dropped int64  // lines discarded because the channel was full
}

// Write always returns (len(p), nil). It appends p to any carried partial
// line, emits every complete (newline-terminated) line, and stashes the
// remainder for the next call.
func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	buf := append(w.partial, p...)
	start := 0
	for {
		idx := bytes.IndexByte(buf[start:], '\n')
		if idx < 0 {
			break
		}
		end := start + idx
		line := bytes.TrimSuffix(buf[start:end], []byte("\r"))
		w.enqueue(string(line))
		start = end + 1
	}

	rem := buf[start:]
	if len(rem) > maxLineBytes {
		w.enqueue(string(rem))
		rem = nil
	}
	// Copy the leftover into a fresh slice so we don't retain buf's (possibly
	// large) backing array between writes.
	w.partial = append([]byte(nil), rem...)
	return len(p), nil
}

// enqueue stamps a monotonic timestamp and does a non-blocking send. Callers
// must hold w.mu. Timestamps are clamped non-decreasing per stream so batching
// and retries never present out-of-order entries to stricter Loki versions.
func (w *lineWriter) enqueue(text string) {
	ts := time.Now().UnixNano()
	if ts <= w.lastTS {
		ts = w.lastTS + 1
	}
	w.lastTS = ts
	select {
	case w.ch <- logLine{ts: ts, text: text}:
	default:
		w.dropped++
	}
}

// run is the flusher goroutine: it batches incoming lines and pushes on either
// a full batch or the batch-wait timer, and drains + pushes the remainder when
// the channel is closed.
func (w *lineWriter) run() {
	defer w.wg.Done()
	timer := time.NewTimer(w.client.cfg.BatchWait)
	defer timer.Stop()

	batch := make([][2]string, 0, w.client.cfg.BatchMaxLines)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := w.client.push(w.labels, batch); err != nil {
			w.client.log.Warn("loki push failed", "error", err, "lines", len(batch), "labels", w.labels)
		}
		batch = batch[:0]
	}

	for {
		select {
		case line, ok := <-w.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, [2]string{strconv.FormatInt(line.ts, 10), line.text})
			if len(batch) >= w.client.cfg.BatchMaxLines {
				flush()
				resetTimer(timer, w.client.cfg.BatchWait)
			}
		case <-timer.C:
			flush()
			timer.Reset(w.client.cfg.BatchWait)
		}
	}
}

// Close flushes any trailing unterminated line, stops the flusher, and waits
// for the final batch to be pushed. It is idempotent and safe to call once via
// the runner's defer regardless of how the review ended (success, error,
// SIGTERM). A closed channel still delivers its buffered lines before the
// flusher observes closure, so nothing already enqueued is lost.
func (w *lineWriter) Close() error {
	w.closeOnce.Do(func() {
		w.mu.Lock()
		if len(w.partial) > 0 {
			w.enqueue(string(w.partial))
			w.partial = nil
		}
		dropped := w.dropped
		w.mu.Unlock()

		close(w.ch)
		w.wg.Wait()

		if dropped > 0 {
			w.client.log.Warn("loki stream dropped lines (buffer full)", "dropped", dropped, "labels", w.labels)
		}
	})
	return nil
}

// resetTimer safely resets a timer that may or may not have fired, draining a
// pending tick so the next select doesn't see a stale one.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
