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
	for _, want := range []string{"NickPit", "Standard review · 2/3", "review: Security", "#1", "nudges 0/2", "00:00 / 10:00", "Findings: 3", "final 3"} {
		if !strings.Contains(joined, want) {
			t.Errorf("dashboard missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "reviewing") {
		t.Errorf("header should drop the word \"reviewing\":\n%s", joined)
	}
	if len(lines) != 6 { // blank + header + info + two bounded slots + findings
		t.Fatalf("lines = %d, want 6: %q", len(lines), lines)
	}
}

func TestWriteFinishRuleClampsWidthToFooterBounds(t *testing.T) {
	// A wider-than-max terminal is clamped so the dashboard rule lines up with
	// the clamped review-output footer rule.
	r := &LiveRenderer{w: &bytes.Buffer{}, useANSI: true, width: 200, height: 24}
	r.writeFinishRule()
	if got, want := r.w.(*bytes.Buffer).String(), "\n\x1b[2m"+strings.Repeat("─", 120)+"\x1b[0m\n\n"; got != want {
		t.Fatalf("clamped finish rule = %q, want %q", got, want)
	}

	// A width within bounds is used verbatim.
	r = &LiveRenderer{w: &bytes.Buffer{}, useANSI: true, width: 90, height: 24}
	r.writeFinishRule()
	if got := r.w.(*bytes.Buffer).String(); !strings.Contains(got, strings.Repeat("─", 90)) || strings.Contains(got, strings.Repeat("─", 91)) {
		t.Fatalf("finish rule at width 90 = %q", got)
	}

	// Without ANSI the rule degrades to the plain marker, width-independent.
	r = &LiveRenderer{w: &bytes.Buffer{}, useANSI: false, width: 200, height: 24}
	r.writeFinishRule()
	if got := r.w.(*bytes.Buffer).String(); got != "\n---\n\n" {
		t.Fatalf("non-ANSI finish rule = %q", got)
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
	// Critically, the redraw after the text must NOT move the cursor back up —
	// that would overwrite the just-written external line. The first byte after
	// "external\n" is the frame's carriage return, not a cursor-up escape.
	_, rest, found := strings.Cut(out, "external\n")
	if !found {
		t.Fatalf("WriteOutside dropped the external text: %q", out)
	}
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
	// 1 completed / (1 completed + 1 active + 2 future) = 25%.
	if !strings.Contains(line, "25%") {
		t.Fatalf("turn+nudge progress not represented as 25%%: %q", line)
	}
}

func TestLiveProgressBarUsesAgentColourWithoutBlinking(t *testing.T) {
	bar := progressBar("review: Testing", len("review"), 0.5, liveProgressBarWidth, 0, true)
	// colorIndex 0 = periwinkle (177,185,249): light fill bg + light text, a dark
	// scaled remainder bg, and a dark scaled text for the filled portion.
	for _, want := range []string{"48;2;177;185;249", "38;2;177;185;249", "48;2;74;78;105", "38;2;39;41;55"} {
		if !strings.Contains(bar, want) {
			t.Errorf("progress bar missing %q: %q", want, bar)
		}
	}
	plain := stripANSI(bar)
	for _, want := range []string{"review: Testing", " 50%"} {
		if !strings.Contains(plain, want) {
			t.Errorf("progress bar text missing %q: %q", want, plain)
		}
	}
	if strings.ContainsRune(plain, '▓') {
		t.Fatalf("progress bar contains blinking/pulsing cell: %q", bar)
	}
	if got := len([]rune(plain)); got != liveProgressBarWidth {
		t.Fatalf("visible progress bar width = %d, want %d", got, liveProgressBarWidth)
	}
}

func TestLiveProgressBarEllipsisesLongLabel(t *testing.T) {
	plain := stripANSI(progressBar("review: A Very Long Reviewer Name That Overflows The Bar", len("review"), 0.0, liveProgressBarWidth, 0, true))
	if got := len([]rune(plain)); got != liveProgressBarWidth {
		t.Fatalf("width = %d, want %d", got, liveProgressBarWidth)
	}
	if !strings.Contains(plain, "…") {
		t.Fatalf("overflowing label should be ellipsised: %q", plain)
	}
	if !strings.Contains(plain, "0%") {
		t.Fatalf("percentage should survive ellipsis: %q", plain)
	}
}

func TestLiveAgentsUseDistinctColoursAndAlignedColumns(t *testing.T) {
	now := time.Now()
	a := &liveAgent{info: ProgressInfo{AgentRole: "review", AgentName: "Security"}, phaseStart: now, colorIndex: 0}
	b := &liveAgent{info: ProgressInfo{AgentRole: "review", AgentName: "Architecture"}, phaseStart: now, colorIndex: 1}
	lineA := formatLiveAgent(a, now, true)
	lineB := formatLiveAgent(b, now, true)
	colorA := rgbSGR("38;2", liveAgentPastelColor(0))
	colorB := rgbSGR("38;2", liveAgentPastelColor(1))
	if !strings.Contains(lineA, colorA) || !strings.Contains(lineB, colorB) || colorA == colorB {
		t.Fatalf("agent colors not distinct:\n%s\n%s", lineA, lineB)
	}
	plainA, plainB := stripANSI(lineA), stripANSI(lineB)
	if !strings.Contains(plainA, "review: Security") || !strings.Contains(plainB, "review: Architecture") {
		t.Fatalf("agent label not rendered inside the bar:\n%s\n%s", plainA, plainB)
	}
	// The bar is fixed width, so the phase column starts at the same offset.
	if strings.Index(plainA, "#1") != strings.Index(plainB, "#1") {
		t.Fatalf("phase columns not aligned:\n%s\n%s", plainA, plainB)
	}
}

func TestLiveAgentLingersAfterDoneThenDrops(t *testing.T) {
	start := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	cur := start
	r := &LiveRenderer{
		w: &bytes.Buffer{}, plan: LivePlan{Concurrency: 2, Units: 1},
		started: start.Add(-90 * time.Second),
		agents:  make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		width: 120, height: 24, now: func() time.Time { return cur },
	}
	info := ProgressInfo{AgentRole: "review", AgentName: "Reasoning", Group: "Reasoning"}
	r.AgentStart(info, WorkflowScope{}, time.Time{})
	r.AgentDone(info)

	dashboard := func() string {
		r.mu.Lock()
		defer r.mu.Unlock()
		return strings.Join(r.buildLinesLocked(), "\n")
	}

	// Within the grace window the finished agent stays, showing a full bar.
	cur = start.Add(1 * time.Second)
	if joined := dashboard(); !strings.Contains(joined, "review: Reasoning") || !strings.Contains(joined, "100%") {
		t.Fatalf("finished agent should linger with a full bar within grace:\n%s", joined)
	}

	// Just shy of the window it is still present.
	cur = start.Add(liveAgentLinger - time.Millisecond)
	if joined := dashboard(); !strings.Contains(joined, "review: Reasoning") {
		t.Fatalf("finished agent dropped before the linger window elapsed:\n%s", joined)
	}

	// At exactly the window boundary (drop uses >=) it disappears.
	cur = start.Add(liveAgentLinger)
	if joined := dashboard(); strings.Contains(joined, "review: Reasoning") {
		t.Fatalf("finished agent should drop at exactly the linger window:\n%s", joined)
	}

	// And it is removed from the map, not merely hidden.
	r.mu.Lock()
	_, present := r.agents[liveAgentKey(info)]
	r.mu.Unlock()
	if present {
		t.Fatal("expired agent should be deleted from r.agents, not left to accumulate")
	}
}

func TestLiveRunningAgentsRankAheadOfLingeringWhenSlotsScarce(t *testing.T) {
	start := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	cur := start
	r := &LiveRenderer{
		w: &bytes.Buffer{}, plan: LivePlan{Concurrency: 1, Units: 1},
		started: start.Add(-90 * time.Second),
		agents:  make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		width: 120, height: 24, now: func() time.Time { return cur },
	}
	alpha := ProgressInfo{AgentRole: "review", AgentName: "Alpha", Group: "Alpha"}
	beta := ProgressInfo{AgentRole: "review", AgentName: "Beta", Group: "Beta"}
	// Alpha starts first, then Beta; Alpha finishes and begins lingering while
	// Beta is still running. With a single slot, the running agent must win it.
	r.AgentStart(alpha, WorkflowScope{}, time.Time{})
	cur = start.Add(1 * time.Second)
	r.AgentStart(beta, WorkflowScope{}, time.Time{})
	r.AgentDone(alpha)

	r.mu.Lock()
	joined := strings.Join(r.buildLinesLocked(), "\n")
	r.mu.Unlock()
	if !strings.Contains(joined, "review: Beta") {
		t.Fatalf("running agent should hold the scarce slot:\n%s", joined)
	}
	if strings.Contains(joined, "review: Alpha") {
		t.Fatalf("lingering finished agent should yield the slot to running work:\n%s", joined)
	}
}

func TestProgressBarNonANSIRendersLabelAndPercent(t *testing.T) {
	bar := progressBar("review: Testing", len("review"), 0.5, liveProgressBarWidth, 0, false)
	if strings.Contains(bar, "\x1b[") {
		t.Fatalf("non-ANSI bar must not contain escape codes: %q", bar)
	}
	if got := len([]rune(bar)); got != liveProgressBarWidth {
		t.Fatalf("non-ANSI bar width = %d, want %d", got, liveProgressBarWidth)
	}
	if !strings.Contains(bar, "review: Testing") {
		t.Fatalf("non-ANSI bar missing label: %q", bar)
	}
	if !strings.HasSuffix(bar, "50% ") {
		t.Fatalf("non-ANSI bar percentage not pinned right inside the pad: %q", bar)
	}
	long := progressBar("review: A Very Long Reviewer Name That Overflows The Bar", len("review"), 0.0, liveProgressBarWidth, 0, false)
	if !strings.Contains(long, "…") {
		t.Fatalf("non-ANSI overflowing label should be ellipsised: %q", long)
	}
	if !strings.HasSuffix(long, "0% ") {
		t.Fatalf("non-ANSI percentage should survive ellipsis: %q", long)
	}
}

func TestProgressBarKeepsPercentUnsplitAtHighFill(t *testing.T) {
	c := liveAgentPastelColor(0)
	// A bold percentage digit opens its own run; if that open carries the fill
	// colours, the fill boundary split the suffix.
	fillDigitOpen := "1;" + rgbSGR("38;2", scaleRGB(c, 0.22)) + ";" + rgbSGR("48;2", c) + "m"
	// 94% would place the fill boundary inside " 94%"; the snap must keep the
	// digits on the base background instead — the "9" never opens on fill.
	bar := progressBar("review: Testing", len("review"), 0.94, liveProgressBarWidth, 0, true)
	if strings.Contains(bar, fillDigitOpen+"9") {
		t.Fatalf("percentage digit rendered on the fill background (split): %q", bar)
	}
	// A complete bar does tint the whole content, the percentage "100" included.
	full := progressBar("review: Testing", len("review"), 1.0, liveProgressBarWidth, 0, true)
	if !strings.Contains(full, fillDigitOpen+"1") {
		t.Fatalf("full bar should tint the percentage on the fill background: %q", full)
	}
}

func TestAgentDoneDoesNotResetLingerTimer(t *testing.T) {
	start := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	cur := start
	r := &LiveRenderer{
		w: &bytes.Buffer{}, plan: LivePlan{Concurrency: 2, Units: 1},
		started: start.Add(-90 * time.Second),
		agents:  make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		width: 120, height: 24, now: func() time.Time { return cur },
	}
	info := ProgressInfo{AgentRole: "review", AgentName: "Reasoning", Group: "Reasoning"}
	r.AgentStart(info, WorkflowScope{}, time.Time{})
	r.AgentDone(info) // stamps doneAt at start

	// A second completion arrives later; it must NOT extend the linger window.
	cur = start.Add(2 * time.Second)
	r.AgentDone(info)

	// Past the window measured from the FIRST completion, the agent is gone.
	cur = start.Add(liveAgentLinger + time.Millisecond)
	r.mu.Lock()
	joined := strings.Join(r.buildLinesLocked(), "\n")
	r.mu.Unlock()
	if strings.Contains(joined, "review: Reasoning") {
		t.Fatalf("repeated AgentDone extended the linger window:\n%s", joined)
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
