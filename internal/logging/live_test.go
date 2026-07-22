package logging

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func testLiveRenderer(now time.Time) *LiveRenderer {
	return &LiveRenderer{
		w:       &bytes.Buffer{},
		plan:    LivePlan{Concurrency: 2, Units: 3},
		started: now.Add(-90 * time.Second),
		agents:  make(map[string]*liveAgent),
		steps:   make(map[string]WorkflowScope),
		findings: liveFindingStats{
			CurrentByLane: make(map[string]int),
		},
		width: 120, height: 24, now: func() time.Time { return now },
	}
}

func TestLiveRendererShowsWorkflowAgentBudgetAndFindings(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	r := testLiveRenderer(now)
	scope := WorkflowScope{Unit: 2, UnitTotal: 3, Lane: "security", Step: "review:security"}
	info := ProgressInfo{AgentRole: "review", AgentName: "Security", Group: "Security", NudgeTotal: 2, Turn: 1}
	r.Step(scope, true)
	r.AgentStart(info, scope, now.Add(10*time.Minute))
	r.Progress(info, scope, StageRequest, StateSent, "", now.Add(10*time.Minute))
	r.Findings(FindingUpdate{Lane: "Security", Found: 3, Current: 3, CurrentPresent: true})

	r.mu.Lock()
	lines := r.buildLinesLocked()
	r.mu.Unlock()
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"NickPit reviewing", "Workflow 2/3", "review:security", "review: Security", "nudges 0/2", "00:00 / 10:00", "Security 3", "found 3", "kept 3"} {
		if !strings.Contains(joined, want) {
			t.Errorf("dashboard missing %q:\n%s", want, joined)
		}
	}
	if len(lines) != 5 { // two headers + two bounded slots + findings
		t.Fatalf("lines = %d, want 5: %q", len(lines), lines)
	}
}

func TestLiveRendererFindingLifecycle(t *testing.T) {
	r := testLiveRenderer(time.Now())
	r.Findings(FindingUpdate{Lane: "Code Quality", Found: 4, Current: 4, CurrentPresent: true})
	r.Findings(FindingUpdate{Lane: "Security", Found: 3, Current: 3, CurrentPresent: true})
	r.Findings(FindingUpdate{Lane: "Code Quality", Refuted: 1, Current: 3, CurrentPresent: true})
	r.Findings(FindingUpdate{Lane: "Security", Duplicate: 2, Filtered: 1, Current: 0, CurrentPresent: true})

	r.mu.Lock()
	line := r.findingLineLocked()
	r.mu.Unlock()
	for _, want := range []string{"Code Quality 3", "Security 0", "found 7", "refuted 1", "duplicate 2", "filtered 1", "kept 3"} {
		if !strings.Contains(line, want) {
			t.Errorf("finding line missing %q: %s", want, line)
		}
	}
}

func TestLiveRendererRedrawOwnsOnlyPreviousRows(t *testing.T) {
	now := time.Now()
	r := testLiveRenderer(now)
	buf := r.w.(*bytes.Buffer)
	r.mu.Lock()
	r.writeFrameLocked([]string{"one", "two", "three"})
	r.writeFrameLocked([]string{"final", "counts"})
	r.mu.Unlock()
	out := buf.String()
	if strings.Contains(out, "\x1b[2J") || strings.Contains(out, "\x1b[0J") {
		t.Fatalf("renderer cleared terminal scrollback: %q", out)
	}
	if !strings.Contains(out, "\x1b[3A") {
		t.Fatalf("renderer did not move over exactly its prior frame: %q", out)
	}
}

func TestLiveProgressFractionReservesNudges(t *testing.T) {
	now := time.Now()
	a := &liveAgent{
		info: ProgressInfo{NudgeTotal: 2}, phaseStart: now, deadline: now.Add(time.Minute),
		doneTurns: 1, activeTurn: true, turn: 2,
	}
	line := formatLiveAgent(a, now, false)
	// 1 completed / (1 completed + 1 active + 2 future) fills 5 of 20 cells.
	if !strings.Contains(line, "█████") || !strings.Contains(line, "25%") {
		t.Fatalf("turn+nudge progress not represented as 5/20: %q", line)
	}
}

func TestLiveProgressBarUsesStatuslinePaletteWithoutBlinking(t *testing.T) {
	bar := progressBar(0.5, 20, true)
	for _, want := range []string{"48;2;177;185;249", "48;2;80;83;112", "38;2;40;42;64"} {
		if !strings.Contains(bar, want) {
			t.Errorf("progress bar missing %q: %q", want, bar)
		}
	}
	plain := stripANSI(bar)
	for _, want := range []string{" Progress", " 50%"} {
		if !strings.Contains(plain, want) {
			t.Errorf("progress bar text missing %q: %q", want, plain)
		}
	}
	if strings.ContainsRune(plain, '▓') {
		t.Fatalf("progress bar contains blinking/pulsing cell: %q", bar)
	}
	if got := len([]rune(stripANSI(bar))); got != 20 {
		t.Fatalf("visible progress bar width = %d, want 20", got)
	}
}

func TestLiveAgentsUseDistinctPastelColorsAndAlignedNames(t *testing.T) {
	now := time.Now()
	a := &liveAgent{info: ProgressInfo{AgentRole: "review", AgentName: "Security"}, phaseStart: now, colorIndex: 0}
	b := &liveAgent{info: ProgressInfo{AgentRole: "review", AgentName: "Architecture"}, phaseStart: now, colorIndex: 1}
	lineA := formatLiveAgent(a, now, true)
	lineB := formatLiveAgent(b, now, true)
	if !strings.Contains(lineA, liveAgentPastel(0)) || !strings.Contains(lineB, liveAgentPastel(1)) || liveAgentPastel(0) == liveAgentPastel(1) {
		t.Fatalf("agent colors not distinct:\n%s\n%s", lineA, lineB)
	}
	plainA, plainB := stripANSI(lineA), stripANSI(lineB)
	if strings.Index(plainA, "Progress") != strings.Index(plainB, "Progress") {
		t.Fatalf("progress columns not aligned:\n%s\n%s", plainA, plainB)
	}
}

func TestLoggerFinishLiveLeavesCompactSnapshot(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, false, false)
	logger.SetLiveProgress(LivePlan{Concurrency: 1, Units: 1})
	logger.LiveFindings(FindingUpdate{Lane: "Testing", Found: 2, Current: 2, CurrentPresent: true})
	logger.FinishLive(true, 1, 65*time.Second)
	if logger.LiveEnabled() {
		t.Fatal("live renderer still attached after finish")
	}
	out := buf.String()
	for _, want := range []string{"Review complete", "01:05", "1 findings", "Testing 2", "filtered 1", "kept 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("compact snapshot missing %q: %q", want, out)
		}
	}
}
