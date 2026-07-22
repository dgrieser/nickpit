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
		width:   120, height: 24, now: func() time.Time { return now },
	}
}

func TestLiveRendererShowsWorkflowAgentBudgetAndFindings(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	r := testLiveRenderer(now)
	scope := WorkflowScope{Unit: 2, UnitTotal: 3, Lane: "security", Step: "review:security", Workflow: "Standard review"}
	info := ProgressInfo{AgentRole: "review", AgentName: "Security", Group: "Security", NudgeTotal: 2, Turn: 1}
	r.Step(scope, true)
	r.AgentStart(info, scope, now.Add(10*time.Minute))
	r.Progress(info, scope, StageRequest, StateSent, "", now.Add(10*time.Minute))
	r.Findings(FindingUpdate{Found: 3})

	r.mu.Lock()
	lines := r.buildLinesLocked()
	r.mu.Unlock()
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"NickPit reviewing", "Standard review · 2/3", "review: Security", "#1", "nudges 0/2", "00:00 / 10:00", "Findings: 3", "final 3"} {
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
	r.Findings(FindingUpdate{Found: 4})
	r.Findings(FindingUpdate{Found: 3})
	r.Findings(FindingUpdate{Refuted: 1})
	r.Findings(FindingUpdate{Duplicate: 2, Filtered: 1})

	r.mu.Lock()
	line := r.findingLineLocked()
	r.mu.Unlock()
	for _, want := range []string{"Findings: 7", "refuted 1", "duplicate 2", "filtered 1", "final 3"} {
		if !strings.Contains(line, want) {
			t.Errorf("finding line missing %q: %s", want, line)
		}
	}
	if strings.Contains(line, "Code Quality") || strings.Contains(line, "Security") {
		t.Errorf("finding line should not mention reviewers: %s", line)
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

func TestWriteOutsidePreservesTextBelowClearedDashboard(t *testing.T) {
	now := time.Now()
	r := testLiveRenderer(now)
	buf := r.w.(*bytes.Buffer)
	// Draw a 3-row frame so lastRows is non-zero and the cursor is parked below it.
	r.mu.Lock()
	r.writeFrameLocked([]string{"one", "two", "three"})
	r.mu.Unlock()
	buf.Reset()

	r.WriteOutside("external\n")

	out := buf.String()
	// The old dashboard (3 rows) is cleared by moving up over exactly those rows.
	if !strings.Contains(out, "\x1b[3A") {
		t.Fatalf("WriteOutside did not clear the prior 3-row frame: %q", out)
	}
	idx := strings.Index(out, "external\n")
	if idx < 0 {
		t.Fatalf("WriteOutside dropped the external text: %q", out)
	}
	// Critically, the redraw after the text must NOT move the cursor back up —
	// that would overwrite the just-written external line. The first byte after
	// "external\n" is the frame's carriage return, not a cursor-up escape.
	rest := out[idx+len("external\n"):]
	if strings.HasPrefix(rest, "\x1b[") {
		t.Fatalf("dashboard redraw moved the cursor over the external text: %q", rest)
	}
	if r.lastRows == 0 {
		t.Fatal("dashboard was not redrawn after the external text")
	}
}

func TestWriteOutsideAfterFinishJustWritesText(t *testing.T) {
	now := time.Now()
	r := testLiveRenderer(now)
	buf := r.w.(*bytes.Buffer)
	r.mu.Lock()
	r.lastRows = 3
	r.closed = true
	r.mu.Unlock()
	buf.Reset()

	r.WriteOutside("boom")

	if out := buf.String(); out != "boom\n" {
		t.Fatalf("closed WriteOutside = %q, want plain %q with no cursor codes", out, "boom\n")
	}
	if r.lastRows != 3 {
		t.Fatalf("closed WriteOutside changed lastRows to %d, want 3 (frozen)", r.lastRows)
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
	logger.LiveFindings(FindingUpdate{Found: 2})
	logger.FinishLive(true, 1, 65*time.Second)
	if logger.LiveEnabled() {
		t.Fatal("live renderer still attached after finish")
	}
	out := buf.String()
	for _, want := range []string{"Review complete", "01:05", "1 findings", "Findings: 2", "filtered 1", "final 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("compact snapshot missing %q: %q", want, out)
		}
	}
}

func TestLiveAgentNameDropsNudgeAndTurnUsesHash(t *testing.T) {
	now := time.Now()
	a := &liveAgent{
		info:       ProgressInfo{AgentRole: "review", AgentName: "Performance · Nudge 2", NudgeIndex: 2, NudgeTotal: 3},
		phaseStart: now, deadline: now.Add(10 * time.Minute), turn: 2,
	}
	line := stripANSI(formatLiveAgent(a, now, false))
	if !strings.Contains(line, "review: Performance") || strings.Contains(line, "Nudge") {
		t.Errorf("agent name should drop the nudge suffix: %q", line)
	}
	if !strings.Contains(line, "#2") || strings.Contains(line, "turn") {
		t.Errorf("turn should render as #N, never the word turn: %q", line)
	}
	if !strings.Contains(line, "nudges 2/3") {
		t.Errorf("nudge progress should appear to the right of the bar: %q", line)
	}
}
