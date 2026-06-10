package logging

import (
	"bytes"
	"sync"
	"testing"
)

// TestLoggerReasoningConcurrentNoRace guards against the data race that existed
// when l.reasoning was lazily initialized under sync.Once inside
// OpenReasoningSection while PrintProgress/PrintProgressToolCall/writeRaw read
// the field without synchronization. The reviewer/verifier fan-out calls these
// from many goroutines at once. Run with -race.
func TestLoggerReasoningConcurrentNoRace(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, true, false)
	// Both flags on, as in `--show-reasoning --show-progress`. These are set
	// during setup, before the goroutines launch — mirroring main.go.
	l.SetShowReasoning(true)
	l.SetShowProgress(true)

	const workers = 16
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			sec := l.OpenReasoningSection(ProgressInfo{AgentRole: "agent"})
			sec.Append("reasoning delta\n")
			l.PrintProgress("Reasoning", "thinking about something")
			l.PrintProgressToolCall("inspect_file foo.go", "ok")
			l.Printf("status: working")
			sec.End()
		})
	}
	wg.Wait()
}
