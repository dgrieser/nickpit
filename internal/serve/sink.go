package serve

import (
	"fmt"
	"io"
	"strconv"

	"github.com/dgrieser/nickpit/internal/serve/loki"
)

// StreamMeta is the low-cardinality identity of one review's log stream.
// HeadSHA is deliberately not a label (it would explode Loki's stream
// cardinality); it is emitted as the stream's first log line instead.
type StreamMeta struct {
	Project string
	IID     int
	Trigger string
	HeadSHA string
}

// LogSink opens a per-review log stream. Open must never block on I/O and must
// never fail a review: a dead backend yields a working (dropping) writer, not
// an error. The runner tees child output into the returned writer alongside the
// authoritative on-disk log.
type LogSink interface {
	Open(meta StreamMeta) io.WriteCloser
}

// NoopSink is the default sink when log shipping is disabled. Its writer
// discards everything, so the runner's tee is a no-op and behavior matches a
// build with no sink at all.
type NoopSink struct{}

func (NoopSink) Open(StreamMeta) io.WriteCloser { return noopWriteCloser{} }

type noopWriteCloser struct{}

func (noopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (noopWriteCloser) Close() error                { return nil }

// lokiSink adapts a *loki.Client to the LogSink interface.
type lokiSink struct {
	client *loki.Client
}

// NewLokiSink wraps a Loki client as a LogSink.
func NewLokiSink(client *loki.Client) LogSink {
	return &lokiSink{client: client}
}

func (s *lokiSink) Open(meta StreamMeta) io.WriteCloser {
	stream := s.client.NewStream(map[string]string{
		"project": meta.Project,
		"iid":     strconv.Itoa(meta.IID),
		"trigger": meta.Trigger,
	})
	// The head SHA rides in the first log line rather than a label, so it is
	// greppable in Grafana without inflating stream cardinality.
	if meta.HeadSHA != "" {
		fmt.Fprintf(stream, "nickpit: review head=%s project=%s iid=%d trigger=%s\n",
			meta.HeadSHA, meta.Project, meta.IID, meta.Trigger)
	}
	return stream
}
