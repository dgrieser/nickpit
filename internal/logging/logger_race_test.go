package logging

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

// TestLoggerReasoningConcurrentNoRace guards against the data race that existed
// when l.reasoning was lazily initialized under sync.Once inside
// OpenReasoningSection while Progress/ProgressToolCall/writeRaw read the field
// without synchronization. The reviewer/verifier fan-out calls these from many
// goroutines at once. Run with -race.
func TestLoggerReasoningConcurrentNoRace(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, true, false)
	// Both flags on, as in `--show-reasoning --show-progress`. These are set
	// during setup, before the goroutines launch — mirroring main.go.
	l.SetShowReasoning(true)
	l.SetShowProgress(true)

	const workers = 16
	ctx := WithProgressInfo(context.Background(), ProgressInfo{AgentRole: "reviewer", AgentName: "#1"})
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			sec := l.OpenReasoningSection(ProgressInfo{AgentRole: "agent"})
			sec.Append("reasoning delta\n")
			l.Progress(ctx, StageReasoning, StateNone, "thinking about something")
			l.ProgressFor(ProgressInfo{AgentRole: "verifier", AgentName: "#2"}, StageVerify, StateDone, "conf=0.9")
			l.ProgressToolCall(ctx, "inspect_file(path=foo.go)", "result=[ok]")
			l.Printf("status: working")
			sec.End()
		})
	}
	wg.Wait()
}
