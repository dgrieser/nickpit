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
	scope := WorkflowScope{Unit: 2, UnitTotal: 3, Lane: "Security", Step: "review:security", Workflow: "Standard review"}
	info := ProgressInfo{AgentRole: "review", AgentName: "Security", Group: "Security", NudgeTotal: 2, Turn: 1}
	r.Step(scope, true)
	r.AgentStart(info, scope, now.Add(10*time.Minute))
	r.Progress(info, scope, StageRequest, StateSent, "", now.Add(10*time.Minute))
	r.Findings(FindingUpdate{Found: 3})

	r.mu.Lock()
	lines := r.buildLinesLocked()
	r.mu.Unlock()
	joined := strings.Join(lines, "\n")
	// The info line names the current lane (not the static workflow name).
	for _, want := range []string{"NickPit", "Security · 2/3", "Security", "#1", "nudges 0/2", "00:00 / 10:00", "Findings 3", "final 3"} {
		if !strings.Contains(joined, want) {
			t.Errorf("dashboard missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "reviewing") {
		t.Errorf("header should drop the word \"reviewing\":\n%s", joined)
	}
	if strings.Contains(joined, "Standard review") {
		t.Errorf("info line should show the lane name, not the workflow name:\n%s", joined)
	}
	if len(lines) != 8 { // blank + header + info + blank + two bounded slots + blank + findings
		t.Fatalf("lines = %d, want 8: %q", len(lines), lines)
	}
}

func TestStepLineNamesCurrentLaneOrCount(t *testing.T) {
	now := time.Now()
	newR := func() *LiveRenderer {
		return &LiveRenderer{
			w: &bytes.Buffer{}, plan: LivePlan{Concurrency: 6, Units: 3, Target: "org/repo#42"},
			started: now, agents: make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
			width: 150, height: 24, now: func() time.Time { return now },
		}
	}
	// No active steps yet → preparing.
	if got := newR().stepLineLocked(); !strings.Contains(got, "Preparing review") {
		t.Fatalf("preparing = %q", got)
	}
	// A single named lane (e.g. the synthesis pipeline) → its name. The target is
	// now a separate line, so the info line ends at the unit progress.
	r := newR()
	r.Step(WorkflowScope{Unit: 3, UnitTotal: 3, Lane: "Review synthesis", Step: "merge"}, true)
	if got := r.stepLineLocked(); !strings.Contains(got, "Review synthesis · 3/3") || strings.Contains(got, "org/repo#42") {
		t.Fatalf("single named lane = %q", got)
	}
	// An unnamed lane (the laneN fallback) → its step label instead of "lane0".
	r = newR()
	r.Step(WorkflowScope{Unit: 1, UnitTotal: 3, Lane: "lane0", Step: "collect-context"}, true)
	if got := r.stepLineLocked(); !strings.Contains(got, "collect-context · 1/3") || strings.Contains(got, "lane0") {
		t.Fatalf("unnamed lane should fall back to the step label = %q", got)
	}
	// Several lanes in an UNNAMED parallel group → a count (names are on the bars).
	r = newR()
	for _, n := range []string{"Code quality", "Security", "Architecture"} {
		r.Step(WorkflowScope{Unit: 2, UnitTotal: 3, Lane: n, Step: "review:" + n}, true)
	}
	if got := r.stepLineLocked(); !strings.Contains(got, "3 lanes · 2/3") {
		t.Fatalf("unnamed parallel group should show a count = %q", got)
	}
	// Several lanes in a NAMED parallel group → the group name, not a count.
	r = newR()
	for _, n := range []string{"Code quality", "Security", "Architecture"} {
		r.Step(WorkflowScope{Unit: 2, UnitTotal: 3, Lane: n, Step: "review:" + n, Group: "Reviewers"}, true)
	}
	if got := r.stepLineLocked(); !strings.Contains(got, "Reviewers · 2/3") || strings.Contains(got, "3 lanes") {
		t.Fatalf("named parallel group should show its name = %q", got)
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
	for _, want := range []string{"Findings 7", "refuted 1", "duplicate 2", "filtered 1", "final 3"} {
		if !strings.Contains(line, want) {
			t.Errorf("finding line missing %q: %s", want, line)
		}
	}
	if strings.Contains(line, "Code Quality") || strings.Contains(line, "Security") {
		t.Errorf("finding line should not mention reviewers: %s", line)
	}
}

func TestStyleLiveTargetUsesBranchPalette(t *testing.T) {
	got := styleLiveTarget("nickpit @ feat/x → origin/main")
	// Same palette as the --show-progress context line.
	want := styleBranchTarget("nickpit", "feat/x", "origin/main")
	if got != want {
		t.Fatalf("branch target styling drifted from show-progress:\n got %q\nwant %q", got, want)
	}
	if stripANSI(got) != "nickpit @ feat/x → origin/main" {
		t.Fatalf("branch target text = %q", stripANSI(got))
	}
	// A non-branch target falls back to a single turquoise tint.
	fb := styleLiveTarget("org/repo#42")
	if fb != progressStyle(progressColorKeyTurquoise, "org/repo#42") {
		t.Fatalf("non-branch target should use the turquoise fallback: %q", fb)
	}
}

func TestSetTargetShownOnItsOwnLine(t *testing.T) {
	r := &LiveRenderer{
		w: &bytes.Buffer{}, plan: LivePlan{Concurrency: 1, Units: 1},
		agents: make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		width: 120, height: 24, now: time.Now,
	}
	r.Step(WorkflowScope{Unit: 1, UnitTotal: 1, Lane: "collect-context", Step: "collect-context"}, true)
	r.SetTarget("nickpit @ feat/x → origin/main")
	// The target is its own line, not part of the info (step) line.
	if line := r.stepLineLocked(); strings.Contains(line, "nickpit @ feat/x") {
		t.Fatalf("target should not be on the info line: %q", line)
	}
	if tl := r.targetLineLocked(); !strings.Contains(tl, "nickpit @ feat/x → origin/main") {
		t.Fatalf("target line missing resolved target: %q", tl)
	}
	// It renders as the line right under the header in the full dashboard.
	r.mu.Lock()
	lines := r.buildLinesLocked()
	r.mu.Unlock()
	if len(lines) < 3 || !strings.Contains(lines[2], "nickpit @ feat/x → origin/main") {
		t.Fatalf("target should be the second content line (index 2):\n%q", lines)
	}
}

func TestInfoLineCarriesWorkflowAgentInfo(t *testing.T) {
	r := &LiveRenderer{
		w: &bytes.Buffer{}, plan: LivePlan{Units: 3, Workflow: "Standard review", WorkflowSource: "embedded", WorkflowSteps: 23},
		agents: make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		width: 150, height: 24, now: time.Now,
	}
	r.Step(WorkflowScope{Unit: 2, UnitTotal: 3, Lane: "Reviewers", Step: "review", Group: "Reviewers"}, true)
	if got := r.stepLineLocked(); !strings.Contains(got, "Reviewers · 2/3 · Standard review · embedded · 23 steps") {
		t.Fatalf("info line should carry the workflow agent info: %q", got)
	}
	// The lane/pipeline name uses the --show-progress "Agent" stage colour.
	r.useANSI = true
	if got := r.stepLineLocked(); !strings.Contains(got, progressStyle(progressStageStyles[StageAgent], "Reviewers")) {
		t.Fatalf("lane name should use the Agent stage colour: %q", got)
	}
}

func TestHeaderShowsModelsWithEffortAndAliasInsteadOfCounter(t *testing.T) {
	now := time.Now()
	r := &LiveRenderer{
		w: &bytes.Buffer{}, now: func() time.Time { return now }, started: now,
		agents: make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		plan: LivePlan{Concurrency: 4, Units: 1, Models: []LiveModel{
			{Name: "BigModel", Effort: "high"},
			{Name: "@small SmallModel", Effort: "none"},
		}},
	}
	r.mu.Lock()
	header := r.headerLineLocked(now)
	r.mu.Unlock()
	// Names carry the reasoning effort and the "@small" alias, like show-progress.
	if !strings.Contains(header, "BigModel:high, @small SmallModel:none") {
		t.Fatalf("header should list models with effort and alias: %q", header)
	}
	if strings.Contains(header, "agents") || strings.Contains(header, "active") {
		t.Fatalf("header should no longer show an agent counter: %q", header)
	}
}

func TestLiveAgentLabelIsNameOnly(t *testing.T) {
	if got := liveAgentLabel(&liveAgent{info: ProgressInfo{AgentRole: "review", AgentName: "Security"}}); got != "Security" {
		t.Fatalf("label should drop the role/kind and show the name only, got %q", got)
	}
	if got := liveAgentLabel(&liveAgent{info: ProgressInfo{AgentRole: "review", AgentName: "Security · Nudge 2"}}); got != "Security" {
		t.Fatalf("nudge suffix should be stripped, got %q", got)
	}
	if got := liveAgentLabel(&liveAgent{info: ProgressInfo{AgentRole: "merge"}, scope: WorkflowScope{Step: "merge"}}); got == "" {
		t.Fatalf("a blank name should fall back to step/role, got %q", got)
	}
}

func TestStyleModelWithAliasLightensAlias(t *testing.T) {
	got := styleModelWithAlias("@small Foo-1B", progressColorKeyTeal)
	if !strings.Contains(got, progressStyle(progressColorKeyTurquoise, "@small")) {
		t.Fatalf("alias should use the lighter turquoise shade: %q", got)
	}
	if !strings.Contains(got, progressStyle(progressColorKeyTeal, "Foo-1B")) {
		t.Fatalf("model name should use the name colour: %q", got)
	}
	if plain := styleModelWithAlias("Foo-1B", progressColorKeyTeal); plain != progressStyle(progressColorKeyTeal, "Foo-1B") {
		t.Fatalf("a non-aliased model should be a single colour: %q", plain)
	}
}

func TestFinalHeaderColours(t *testing.T) {
	r := &LiveRenderer{useANSI: true}
	// Abort: red ✗, bold-white word, turquoise time, grey dot.
	stop := r.finalHeaderLocked("✗", "Review stopped", 65*time.Second, 0, false)
	if !strings.Contains(stop, progressStyle(progressColorErrorRed, "✗")) {
		t.Fatalf("✗ should be red: %q", stop)
	}
	if !strings.Contains(stop, progressStyle(progressColorBold+";"+progressColorWhite, "Review stopped")) {
		t.Fatalf("status word should be bold white: %q", stop)
	}
	if !strings.Contains(stop, progressStyle(progressColorKeyTurquoise, "01:05")) {
		t.Fatalf("time should be turquoise (distinct from the word): %q", stop)
	}
	if !strings.Contains(stop, progressGrey(" · ")) {
		t.Fatalf("middle dot should be grey: %q", stop)
	}
	// Success: green ✓.
	done := r.finalHeaderLocked("✓", "Review complete", time.Second, 4, true)
	if !strings.Contains(done, progressStyle(progressColorNumberGreen, "✓")) {
		t.Fatalf("✓ should be green: %q", done)
	}
}

func TestFindingLineHasSemanticColoursAndGreyDots(t *testing.T) {
	r := &LiveRenderer{useANSI: true, findings: liveFindingStats{Found: 3, Refuted: 1}}
	line := r.findingLineLocked()
	if !strings.Contains(line, progressGrey(" · ")) {
		t.Fatalf("findings separators should be grey: %q", line)
	}
	for _, want := range []string{
		progressStyle(progressColorKeyTeal, "Findings 3"),     // total: teal
		progressStyle(progressColorErrorRed, "refuted 1"),     // dropped: red
		progressStyle(progressColorWarnYellow, "duplicate 0"), // amber
		progressStyle(progressColorProfile, "filtered 0"),     // peach
		progressStyle(progressColorNumberGreen, "final 2"),    // kept: green
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("findings line missing coloured metric %q:\n%q", want, line)
		}
	}
}

func TestNickPitWordmarkAnimates(t *testing.T) {
	f0 := nickPitWordmark(0)
	if got := stripANSI(f0); got != "NickPit" {
		t.Fatalf("wordmark letters = %q, want NickPit", got)
	}
	if !strings.Contains(f0, "\x1b[1;38;2;") {
		t.Fatalf("wordmark should be bold truecolour: %q", f0)
	}
	// It animates: adjacent frames and a stage/style switch both differ.
	if nickPitWordmark(0) == nickPitWordmark(3) {
		t.Fatalf("wordmark should animate between adjacent frames")
	}
	if nickPitWordmark(0) == nickPitWordmark(2*nickPitStageFrames) {
		t.Fatalf("wordmark should change gradient/style across stages")
	}
	// Every stage/style (and a mid-fade frame) still renders exactly seven letters.
	for _, frame := range []int{0, 40, 49, 98, 147, 196} {
		if got := stripANSI(nickPitWordmark(frame)); got != "NickPit" {
			t.Fatalf("stage at frame %d dropped letters: %q", frame, got)
		}
	}
}

func TestNickPitWordmarkFadesWithoutAbruptJumps(t *testing.T) {
	// Reconstruct the per-letter colour the wordmark uses (stage colour + the
	// boundary cross-fade) and assert adjacent frames never jump hard.
	colorAt := func(frame, i, n int) [3]int {
		pos := frame % nickPitStageFrames
		fadeStart := nickPitStageFrames - nickPitFadeFrames
		rgb := nickPitStageColor(frame/nickPitStageFrames, frame, i, n)
		if pos >= fadeStart {
			t := smoothstep(float64(pos-fadeStart+1) / float64(nickPitFadeFrames))
			rgb = lerpRGB(rgb, nickPitStageColor(frame/nickPitStageFrames+1, frame, i, n), t)
		}
		return rgb
	}
	n := len([]rune("NickPit"))
	// Span two full stage boundaries (a flow→sweep and a sweep→flow switch).
	for f := 0; f <= 2*nickPitStageFrames+2; f++ {
		for i := range n {
			a, b := colorAt(f, i, n), colorAt(f+1, i, n)
			for c := range 3 {
				if d := a[c] - b[c]; d > 64 || d < -64 {
					t.Fatalf("abrupt colour jump at frame %d letter %d channel %d: %v→%v", f, i, c, a, b)
				}
			}
		}
	}
}

func TestLivePhaseTurnMatchesShowProgress(t *testing.T) {
	got := formatLivePhase(&liveAgent{turn: 3, info: ProgressInfo{NudgeTotal: 3}}, true)
	// The "#N" round reuses the show-progress turn colouring exactly.
	if !strings.Contains(got, formatProgressTurn(true, 3)) {
		t.Fatalf("turn should reuse formatProgressTurn: %q", got)
	}
	if !strings.Contains(got, progressStyle(progressColorUnitGreen, "#")) {
		t.Fatalf("# should be the darker unit-green: %q", got)
	}
}

func TestLivePhaseReservesTwoDigitRoundForAlignment(t *testing.T) {
	now := time.Now()
	single := &liveAgent{info: ProgressInfo{NudgeTotal: 3}, turn: 3, phaseStart: now}
	double := &liveAgent{info: ProgressInfo{NudgeTotal: 3}, turn: 12, phaseStart: now}
	for _, useANSI := range []bool{false, true} {
		ps := stripANSI(formatLivePhase(single, useANSI))
		pd := stripANSI(formatLivePhase(double, useANSI))
		if i, j := strings.Index(ps, "nudges"), strings.Index(pd, "nudges"); i != j {
			t.Fatalf("nudges misaligned (useANSI=%v) between #3 (%q) and #12 (%q)", useANSI, ps, pd)
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
	bar := progressBar("review: Testing", 0.5, liveProgressBarWidth, 0, true)
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
	plain := stripANSI(progressBar("review: A Very Long Reviewer Name That Overflows The Bar", 0.0, liveProgressBarWidth, 0, true))
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
	if !strings.Contains(plainA, "Security") || !strings.Contains(plainB, "Architecture") {
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
	if joined := dashboard(); !strings.Contains(joined, "Reasoning") || !strings.Contains(joined, "100%") {
		t.Fatalf("finished agent should linger with a full bar within grace:\n%s", joined)
	}

	// Just shy of the window it is still present.
	cur = start.Add(liveAgentLinger - time.Millisecond)
	if joined := dashboard(); !strings.Contains(joined, "Reasoning") {
		t.Fatalf("finished agent dropped before the linger window elapsed:\n%s", joined)
	}

	// At exactly the window boundary (drop uses >=) it disappears.
	cur = start.Add(liveAgentLinger)
	if joined := dashboard(); strings.Contains(joined, "Reasoning") {
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
	if !strings.Contains(joined, "Beta") {
		t.Fatalf("running agent should hold the scarce slot:\n%s", joined)
	}
	if strings.Contains(joined, "Alpha") {
		t.Fatalf("lingering finished agent should yield the slot to running work:\n%s", joined)
	}
}

func TestProgressBarNonANSIRendersLabelAndPercent(t *testing.T) {
	bar := progressBar("review: Testing", 0.5, liveProgressBarWidth, 0, false)
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
	long := progressBar("review: A Very Long Reviewer Name That Overflows The Bar", 0.0, liveProgressBarWidth, 0, false)
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
	bar := progressBar("review: Testing", 0.94, liveProgressBarWidth, 0, true)
	if strings.Contains(bar, fillDigitOpen+"9") {
		t.Fatalf("percentage digit rendered on the fill background (split): %q", bar)
	}
	// A complete bar does tint the whole content, the percentage "100" included.
	full := progressBar("review: Testing", 1.0, liveProgressBarWidth, 0, true)
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
	if strings.Contains(joined, "Reasoning") {
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
	for _, want := range []string{"Review complete", "01:05", "1 findings", "Findings 2", "filtered 1", "final 1"} {
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
	if !strings.Contains(line, "Performance") || strings.Contains(line, "Nudge") {
		t.Errorf("agent name should drop the nudge suffix: %q", line)
	}
	if !strings.Contains(line, "#2") || strings.Contains(line, "turn") {
		t.Errorf("turn should render as #N, never the word turn: %q", line)
	}
	if !strings.Contains(line, "nudges 2/3") {
		t.Errorf("nudge progress should appear to the right of the bar: %q", line)
	}
}
