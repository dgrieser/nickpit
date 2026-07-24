package logging

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/output"
	"golang.org/x/term"
)

// LivePlan describes the bounded terminal area available to a review run.
type LivePlan struct {
	Concurrency int
	Units       int
	// Target names what is under review (e.g. "org/repo#42" or a branch), shown
	// on its own line under the header. Empty for source-less runs.
	Target string
	// Workflow identity for the info-line "agent info" (name · source · N steps),
	// mirroring the --show-progress Agent bracket.
	Workflow       string
	WorkflowSource string
	WorkflowSteps  int
	// Models are the review's configured models (primary and, when it differs, the
	// "@small" model), shown in the header exactly like the --show-progress model
	// lines: alias-prefixed name plus reasoning effort.
	Models []LiveModel
}

// LiveModel is a model shown in the dashboard header: its display name (which may
// carry an "@alias " prefix) and reasoning effort.
type LiveModel struct {
	Name   string
	Effort string
}

// WorkflowScope identifies the pipeline work enclosing an agent loop.
type WorkflowScope struct {
	Unit      int
	UnitTotal int
	Lane      string
	Step      string
	// Group is the optional name of the enclosing parallel group; shown in the
	// info line while several of its lanes run at once. Empty for single lanes.
	Group string
	// Workflow is the human-readable workflow name shown above the dashboard.
	Workflow string
}

type workflowScopeKey struct{}

func WithWorkflowScope(ctx context.Context, scope WorkflowScope) context.Context {
	return context.WithValue(ctx, workflowScopeKey{}, scope)
}

func WorkflowScopeFromContext(ctx context.Context) WorkflowScope {
	if ctx == nil {
		return WorkflowScope{}
	}
	scope, _ := ctx.Value(workflowScopeKey{}).(WorkflowScope)
	return scope
}

func contextDeadline(ctx context.Context) time.Time {
	if ctx == nil {
		return time.Time{}
	}
	deadline, _ := ctx.Deadline()
	return deadline
}

// FindingUpdate reports one authoritative finding-set transition.
type FindingUpdate struct {
	Found     int
	Refuted   int
	Duplicate int
	Filtered  int
}

type liveFindingStats struct {
	Found, Refuted, Duplicate, Filtered int
}

// liveAgentLinger keeps a finished agent on the dashboard briefly so short-lived
// agents (e.g. a reasoning-mine pass) remain readable instead of vanishing the
// instant they complete.
const liveAgentLinger = 3 * time.Second

type liveAgent struct {
	key         string
	info        ProgressInfo
	scope       WorkflowScope
	phaseStart  time.Time
	deadline    time.Time
	doneTurns   int
	activeTurn  bool
	visible     bool
	done        bool
	doneAt      time.Time
	turn        int
	lastStarted time.Time
	colorIndex  int
	// shown is the animated bar fraction (also drives the percentage); animAt is
	// when it was last advanced. The bar creeps toward the next step while a turn
	// runs, then snaps up when the step actually lands.
	shown  float64
	animAt time.Time
}

// LiveRenderer owns a fixed dashboard below existing terminal scrollback.
type LiveRenderer struct {
	mu        sync.Mutex
	w         io.Writer
	fd        int
	useANSI   bool
	plan      LivePlan
	started   time.Time
	agents    map[string]*liveAgent
	steps     map[string]WorkflowScope
	findings  liveFindingStats
	lastRows  int
	final     []string
	closed    bool
	wake      chan struct{}
	stop      chan struct{}
	done      chan struct{}
	width     int // tests
	height    int // tests
	now       func() time.Time
	nextColor int
}

func newLiveRenderer(w io.Writer, useANSI bool, plan LivePlan) *LiveRenderer {
	fd := -1
	if f, ok := w.(*os.File); ok && f != nil {
		fd = int(f.Fd())
	}
	r := &LiveRenderer{
		w: w, fd: fd, useANSI: useANSI, plan: plan, started: time.Now(),
		agents: make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
		now: time.Now,
	}
	// Hide the cursor for the life of the dashboard so it does not flicker across
	// the frame on every redraw; Finish/Close restore it.
	if useANSI {
		_, _ = io.WriteString(w, cursorHide)
	}
	go r.run()
	return r
}

const (
	cursorHide = "\x1b[?25l"
	cursorShow = "\x1b[?25h"
)

// showCursorLocked restores the terminal cursor hidden for the dashboard's life.
func (r *LiveRenderer) showCursor() {
	if r.useANSI {
		_, _ = io.WriteString(r.w, cursorShow)
	}
}

// SetLiveProgress enables default TTY progress. Caller owns activation policy.
func (l *Logger) SetLiveProgress(plan LivePlan) {
	if l == nil || l.live != nil {
		return
	}
	l.live = newLiveRenderer(l.w, l.useANSI, plan)
}

func (l *Logger) LiveEnabled() bool { return l != nil && l.live != nil }

func (l *Logger) LiveAgentStart(ctx context.Context, info ProgressInfo) {
	if l != nil && l.live != nil {
		l.live.AgentStart(info, WorkflowScopeFromContext(ctx), contextDeadline(ctx))
	}
}

func (l *Logger) LiveAgentDone(info ProgressInfo) {
	if l != nil && l.live != nil {
		l.live.AgentDone(info)
	}
}

func (l *Logger) LiveStep(scope WorkflowScope, active bool) {
	if l != nil && l.live != nil {
		l.live.Step(scope, active)
	}
}

// SetLiveTarget updates the review target shown in the dashboard info line once
// the resolved repo/branch is known (e.g. "repo @ head → base").
func (l *Logger) SetLiveTarget(target string) {
	if l != nil && l.live != nil {
		l.live.SetTarget(target)
	}
}

func (l *Logger) LiveFindings(update FindingUpdate) {
	if l != nil && l.live != nil {
		l.live.Findings(update)
	}
}

// FinishLive freezes a compact snapshot and stops animation.
func (l *Logger) FinishLive(ok bool, findings int, elapsed time.Duration) {
	if l == nil || l.live == nil {
		return
	}
	l.live.Finish(ok, findings, elapsed)
	l.live = nil
}

// CloseLive stops an unfinished dashboard, normally from an error path.
func (l *Logger) CloseLive() {
	if l == nil || l.live == nil {
		return
	}
	l.live.Close()
	l.live = nil
}

func (r *LiveRenderer) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer close(r.done)
	for {
		select {
		case <-ticker.C:
			r.redraw()
		case <-r.wake:
			r.redraw()
		case <-r.stop:
			r.redraw()
			return
		}
	}
}

func (r *LiveRenderer) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func liveAgentKey(info ProgressInfo) string {
	group := strings.TrimSpace(info.Group)
	if group == "" && info.AgentRole == "review" {
		group = info.AgentName
		if before, _, ok := strings.Cut(group, " · Nudge"); ok {
			group = before
		}
	}
	if group != "" {
		return info.AgentRole + "|" + group
	}
	return info.AgentRole + "|" + info.AgentName
}

func (r *LiveRenderer) AgentStart(info ProgressInfo, scope WorkflowScope, deadline time.Time) {
	r.mu.Lock()
	key := liveAgentKey(info)
	a := r.agents[key]
	if a == nil {
		a = &liveAgent{key: key, colorIndex: r.nextColor}
		r.nextColor++
		r.agents[key] = a
	}
	now := r.now()
	a.info, a.scope, a.phaseStart, a.deadline = info, scope, now, deadline
	a.visible, a.lastStarted = true, now
	a.done, a.doneAt = false, time.Time{}
	a.animAt = now // reset the animation clock; a new agent keeps shown at 0
	r.mu.Unlock()
	r.signal()
}

func (r *LiveRenderer) AgentDone(info ProgressInfo) {
	r.mu.Lock()
	if a := r.agents[liveAgentKey(info)]; a != nil {
		// Keep the agent on the dashboard for a short grace period so it does not
		// blink out the instant it finishes; buildLinesLocked drops it once the
		// linger window elapses. Stamp the deadline only on the first completion so
		// a repeated AgentDone never extends the window.
		a.activeTurn = false
		if !a.done {
			a.done, a.doneAt = true, r.now()
		}
	}
	r.mu.Unlock()
	r.signal()
}

func (r *LiveRenderer) Progress(info ProgressInfo, scope WorkflowScope, stage Stage, state State, _ string, deadline time.Time) {
	if info.AgentRole == "" {
		return
	}
	r.mu.Lock()
	a := r.agents[liveAgentKey(info)]
	if a != nil {
		if scope != (WorkflowScope{}) {
			a.scope = scope
		}
		if !deadline.IsZero() {
			a.deadline = deadline
		}
		if info.Turn > a.turn {
			a.turn = info.Turn
		}
		switch {
		case stage == StageRequest && state == StateSent:
			a.activeTurn = true
		case stage == StageResponse && state == StateDone:
			if a.activeTurn {
				a.doneTurns++
			}
			a.activeTurn = false
		}
	}
	r.mu.Unlock()
	r.signal()
}

func (r *LiveRenderer) Step(scope WorkflowScope, active bool) {
	r.mu.Lock()
	key := scope.Lane
	if key == "" {
		key = scope.Step
	}
	if active {
		r.steps[key] = scope
	} else {
		delete(r.steps, key)
	}
	r.mu.Unlock()
	r.signal()
}

func (r *LiveRenderer) SetTarget(target string) {
	r.mu.Lock()
	r.plan.Target = target
	r.mu.Unlock()
	r.signal()
}

func (r *LiveRenderer) Findings(update FindingUpdate) {
	r.mu.Lock()
	r.findings.Found += max(update.Found, 0)
	r.findings.Refuted += max(update.Refuted, 0)
	r.findings.Duplicate += max(update.Duplicate, 0)
	r.findings.Filtered += max(update.Filtered, 0)
	r.mu.Unlock()
	r.signal()
}

func (r *LiveRenderer) Finish(ok bool, findings int, elapsed time.Duration) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	kept := r.keptLocked()
	if findings < kept {
		r.findings.Filtered += kept - findings
	}
	mark := "✓"
	word := "Review complete"
	if !ok {
		mark, word = "✗", "Review stopped"
	}
	r.final = []string{
		r.finalHeaderLocked(mark, word, elapsed, findings, true),
		r.findingLineLocked(),
	}
	r.closed = true
	r.mu.Unlock()
	close(r.stop)
	<-r.done
	r.showCursor()
	r.writeFinishRule()
}

// writeFinishRule draws a horizontal rule below the frozen dashboard, matching
// the review-output footer rule, so the live progress block is visibly
// separated from the review output that follows it on stdout.
func (r *LiveRenderer) writeFinishRule() {
	rule := "---"
	if r.useANSI {
		width, _ := r.termSize()
		rule = "\x1b[2m" + strings.Repeat("─", output.ClampWidth(width)) + progressColorReset
	}
	_, _ = io.WriteString(r.w, "\n"+rule+"\n\n")
}

func (r *LiveRenderer) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	elapsed := r.now().Sub(r.started)
	r.final = []string{r.finalHeaderLocked("✗", "Review stopped", elapsed, 0, false), r.findingLineLocked()}
	r.closed = true
	r.mu.Unlock()
	close(r.stop)
	<-r.done
	r.showCursor()
}

// finalHeaderLocked renders the frozen snapshot's headline: a green ✓ (or red ✗
// on abort), the status word in bold white (distinct from the turquoise elapsed
// time), grey middle dots, and — when shown — the findings count in green.
func (r *LiveRenderer) finalHeaderLocked(mark, word string, elapsed time.Duration, findings int, showFindings bool) string {
	if !r.useANSI {
		if showFindings {
			return fmt.Sprintf("%s %s · %s · %d findings", mark, word, shortDuration(elapsed), findings)
		}
		return fmt.Sprintf("%s %s · %s", mark, word, shortDuration(elapsed))
	}
	markColor := progressColorNumberGreen
	if mark == "✗" {
		markColor = progressColorErrorRed
	}
	sep := progressGrey(" · ")
	line := progressStyle(markColor, mark) + " " +
		progressStyle(progressColorBold+";"+progressColorWhite, word) + sep +
		progressStyle(progressColorKeyTurquoise, shortDuration(elapsed))
	if showFindings {
		line += sep + progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", findings)) + " " + progressLight("findings")
	}
	return line
}

func (r *LiveRenderer) keptLocked() int {
	return max(r.findings.Found-r.findings.Refuted-r.findings.Duplicate-r.findings.Filtered, 0)
}

func (r *LiveRenderer) redraw() {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := r.buildLinesLocked()
	r.writeFrameLocked(lines)
}

func (r *LiveRenderer) buildLinesLocked() []string {
	width, height := r.termSize()
	if len(r.final) > 0 {
		return styleLiveLines(r.useANSI, fitLines(r.final, width), true)
	}
	now := r.now()
	visible := make([]*liveAgent, 0)
	for key, a := range r.agents {
		if !a.visible {
			continue
		}
		// Drop finished agents once their linger window has elapsed. Deleting the
		// map entry (safe during range in Go) keeps r.agents from growing without
		// bound over a long run with many short-lived agents.
		if a.done && now.Sub(a.doneAt) >= liveAgentLinger {
			delete(r.agents, key)
			continue
		}
		visible = append(visible, a)
	}
	sort.Slice(visible, func(i, j int) bool {
		// Running agents rank ahead of lingering finished ones, so a just-completed
		// agent's grace period never hides live work when slots are scarce.
		if visible[i].done != visible[j].done {
			return !visible[i].done
		}
		if visible[i].lastStarted.Equal(visible[j].lastStarted) {
			return visible[i].key < visible[j].key
		}
		return visible[i].lastStarted.Before(visible[j].lastStarted)
	})
	// A leading blank line separates the dashboard from prior scrollback, then the
	// header, an optional target line, the info line, a blank spacer, the agent
	// rows, another blank spacer, and the findings footer.
	head := []string{"", r.headerLineLocked(now)}
	if target := r.targetLineLocked(); target != "" {
		head = append(head, target)
	}
	head = append(head, r.stepLineLocked(), "")
	slots := r.plan.Concurrency
	// Leave one spare row beyond the chrome (head lines + blank spacer + findings).
	if maxSlots := height - len(head) - 2; slots <= 0 || slots > maxSlots {
		slots = maxSlots
	}
	if slots < 1 {
		slots = 1
	}
	lines := head
	for i := 0; i < slots; i++ {
		if i < len(visible) {
			lines = append(lines, formatLiveAgent(visible[i], now, r.useANSI))
		} else {
			lines = append(lines, "")
		}
	}
	// One blank spacer separates the agent rows from the findings footer.
	lines = append(lines, "", r.findingLineLocked())
	return styleLiveLines(r.useANSI, fitLines(lines, width), false)
}

// headerLineLocked renders the top dashboard line: the animated "NickPit"
// wordmark (which replaces the old spinner as the motion cue), the elapsed time,
// then the model names in use (comma-separated) — the same names and colours the
// --show-progress model line uses.
func (r *LiveRenderer) headerLineLocked(now time.Time) string {
	dur := shortDuration(now.Sub(r.started))
	if !r.useANSI {
		s := fmt.Sprintf("  NickPit · %s", dur)
		if names := plainModelList(r.plan.Models); names != "" {
			s += " · " + names
		}
		return s
	}
	sep := progressGrey(" · ")
	frame := max(int(now.Sub(r.started)/(100*time.Millisecond)), 0)
	line := "  " + nickPitWordmark(frame) + sep + progressStyle(progressColorKeyTurquoise, dur)
	if len(r.plan.Models) > 0 {
		line += sep + styleModelList(r.plan.Models)
	}
	return line
}

func plainModelList(models []LiveModel) string {
	parts := make([]string, 0, len(models))
	for _, m := range models {
		if m.Name == "" {
			continue
		}
		s := m.Name
		if m.Effort != "" {
			s += ":" + m.Effort
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// styleModelList joins models with grey commas, each rendered exactly like the
// --show-progress model line: "@alias" prefix a shade lighter, name teal, a grey
// ":" and the reasoning effort. The endpoint URL is intentionally omitted.
func styleModelList(models []LiveModel) string {
	parts := make([]string, 0, len(models))
	for _, m := range models {
		if m.Name == "" {
			continue
		}
		parts = append(parts, formatProgressModel(true, ProgressInfo{Model: m.Name, Effort: m.Effort}, false))
	}
	return strings.Join(parts, progressGrey(", "))
}

// nickPitGradients are smooth colour ramps the wordmark animation flows through.
// Each is a cyclic gradient (last stop blends back to the first).
var nickPitGradients = [][][3]int{
	{{255, 66, 106}, {255, 128, 170}, {255, 176, 120}}, // rose → pink → coral
	{{56, 128, 255}, {80, 210, 255}, {120, 255, 224}},  // blue → cyan → aqua
	{{72, 210, 130}, {192, 240, 120}, {110, 224, 200}}, // green → lime → teal
	{{255, 150, 66}, {255, 214, 96}, {255, 118, 176}},  // orange → gold → magenta
	{{150, 96, 255}, {198, 120, 255}, {255, 122, 214}}, // indigo → violet → magenta
}

const (
	// nickPitStageFrames is how long each gradient/style stage runs (~4.8s at the
	// 100ms redraw tick); nickPitFadeFrames is the tail of each stage that
	// cross-fades into the next so the switch is never abrupt.
	nickPitStageFrames = 48
	nickPitFadeFrames  = 18
)

// nickPitWordmark renders "NickPit" bold with a per-letter colour that animates
// smoothly with the frame counter — the dashboard's motion cue in place of the
// old braille spinner. Every stage advances to the next gradient and alternates
// between two smooth styles (a gradient flowing through the letters and a soft
// highlight gliding back and forth over the ramp); the last few frames of a
// stage cross-fade into the next stage so transitions never snap.
func nickPitWordmark(frame int) string {
	letters := []rune("NickPit")
	n := len(letters)
	stage := frame / nickPitStageFrames
	posInStage := frame % nickPitStageFrames
	fadeStart := nickPitStageFrames - nickPitFadeFrames
	var b strings.Builder
	for i := range letters {
		rgb := nickPitStageColor(stage, frame, i, n)
		if posInStage >= fadeStart {
			// Reaches a full blend to the next stage exactly at the boundary.
			t := smoothstep(float64(posInStage-fadeStart+1) / float64(nickPitFadeFrames))
			rgb = lerpRGB(rgb, nickPitStageColor(stage+1, frame, i, n), t)
		}
		b.WriteString(progressStyle(progressColorBold+";"+rgbSGR("38;2", rgb), string(letters[i])))
	}
	return b.String()
}

// nickPitStageColor is the per-letter colour a given stage would show at frame,
// evaluated continuously in frame so cross-fading two stages stays smooth. Even
// stages flow the gradient through the letters; odd stages glide a highlight.
func nickPitStageColor(stage, frame, i, n int) [3]int {
	grad := nickPitGradients[stage%len(nickPitGradients)]
	if stage%2 == 1 {
		// A soft highlight glides back and forth; letters fade into the ramp's deep
		// end as they move away from it.
		peak := triangleF(float64(frame)*0.16, float64(n-1))
		t := 1 - math.Abs(float64(i)-peak)/float64(n)
		return lerpRGB(scaleRGB(grad[0], 0.4), grad[len(grad)-1], smoothstep(t))
	}
	// The whole ramp flows smoothly through the letters.
	return sampleGradient(grad, frac(float64(frame)*0.032+float64(i)/float64(n)))
}

// sampleGradient samples a cyclic colour ramp at t in [0,1), smoothstepping
// between adjacent stops (the last stop wraps to the first).
func sampleGradient(colors [][3]int, t float64) [3]int {
	n := len(colors)
	if n == 0 {
		return [3]int{255, 255, 255}
	}
	if n == 1 {
		return colors[0]
	}
	scaled := frac(t) * float64(n)
	idx := int(scaled)
	return lerpRGB(colors[idx%n], colors[(idx+1)%n], smoothstep(scaled-float64(idx)))
}

// triangleF ping-pongs x over [0, span].
func triangleF(x, span float64) float64 {
	if span <= 0 {
		return 0
	}
	m := math.Mod(math.Mod(x, 2*span)+2*span, 2*span)
	if m > span {
		m = 2*span - m
	}
	return m
}

func frac(x float64) float64 { return x - math.Floor(x) }

func smoothstep(t float64) float64 {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func lerpRGB(a, b [3]int, t float64) [3]int {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return [3]int{
		a[0] + int(float64(b[0]-a[0])*t),
		a[1] + int(float64(b[1]-a[1])*t),
		a[2] + int(float64(b[2]-a[2])*t),
	}
}

func (r *LiveRenderer) stepLineLocked() string {
	// Each map entry is one active lane, keyed by its label; the info line names
	// the current lane/pipeline. When several lanes run in parallel, show the
	// enclosing group's name if it has one, otherwise a count — the individual
	// lanes are already visible in the per-agent bars below.
	unit, total := 0, r.plan.Units
	var laneName, laneStep, group string
	for _, scope := range r.steps {
		if scope.Unit > unit {
			unit = scope.Unit
		}
		if scope.UnitTotal > total {
			total = scope.UnitTotal
		}
		laneName, laneStep = scope.Lane, scope.Step
		if scope.Group != "" {
			group = scope.Group
		}
	}
	count := len(r.steps)
	total = max(total, 1)

	// The workflow "agent info" (name · source · N steps) trails the line, in the
	// same shape and colours as the --show-progress Agent bracket.
	wf := formatProgressWorkflow(r.useANSI, r.workflowInfo())
	if !r.useANSI {
		s := "  " + stepDisplayName(count, group, laneName, laneStep)
		if count > 0 {
			// No dot between the name and its N/M progress.
			s += fmt.Sprintf(" %d/%d", unit, total)
		}
		if len(wf) > 0 {
			s += " · " + strings.Join(wf, " · ")
		}
		return s
	}
	sep := progressGrey(" · ")
	var namePart string
	switch {
	case count > 1 && strings.TrimSpace(group) == "":
		namePart = progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", count)) + " " + progressLight("lanes")
	default:
		// The current lane/pipeline/step name uses the --show-progress "Agent"
		// stage colour (bold blue).
		namePart = progressStyle(progressStageStyles[StageAgent], stepDisplayName(count, group, laneName, laneStep))
	}
	// The N/M progress joins the name with a space (no middle dot).
	if count > 0 {
		namePart += " " + progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", unit)) +
			progressGrey("/") + progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", total))
	}
	parts := append([]string{namePart}, wf...)
	return "  " + strings.Join(parts, sep)
}

// workflowInfo packages the workflow identity the dashboard shows on the info
// line, reusing formatProgressWorkflow for identical rendering to show-progress.
func (r *LiveRenderer) workflowInfo() ProgressInfo {
	return ProgressInfo{
		Workflow:       r.plan.Workflow,
		WorkflowSource: r.plan.WorkflowSource,
		WorkflowSteps:  r.plan.WorkflowSteps,
	}
}

// targetLineLocked is the dedicated review-target line (repo @ head → base),
// shown just under the header; empty when no target is set.
func (r *LiveRenderer) targetLineLocked() string {
	if r.plan.Target == "" {
		return ""
	}
	if !r.useANSI {
		return "  " + r.plan.Target
	}
	return "  " + styleLiveTarget(r.plan.Target)
}

// stepDisplayName names the info line. For a named parallel group it is the
// group name; for several unnamed lanes, a count; for a single lane its
// configured name, falling back to the step label for an unnamed ("laneN")
// lane so a plain step like collect-context reads sensibly.
func stepDisplayName(count int, group, laneName, laneStep string) string {
	switch {
	case count == 0:
		return "Preparing review"
	case count > 1:
		if g := strings.TrimSpace(group); g != "" {
			return g
		}
		return fmt.Sprintf("%d lanes", count)
	}
	if laneName == "" || isFallbackLaneLabel(laneName) {
		if laneStep != "" {
			return laneStep
		}
	}
	if laneName != "" {
		return laneName
	}
	if laneStep != "" {
		return laneStep
	}
	return "Workflow"
}

// isFallbackLaneLabel reports whether s is the "laneN" placeholder the pipeline
// emits for an unnamed lane (see review.liveLaneLabel), not a real lane name.
func isFallbackLaneLabel(s string) bool {
	rest, ok := strings.CutPrefix(s, "lane")
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// styleLiveTarget colours a review target for the info line. A "repo @ head →
// base" target uses the shared branch palette (matching --show-progress); any
// other form falls back to a single turquoise tint.
func styleLiveTarget(target string) string {
	if repo, rest, ok := strings.Cut(target, " @ "); ok {
		if head, base, ok := strings.Cut(rest, " → "); ok {
			return styleBranchTarget(repo, head, base)
		}
	}
	return progressStyle(progressColorKeyTurquoise, target)
}

// liveAgentLabel is the bar's agent name — the name only, without the role/kind
// prefix. Any nudge suffix is stripped (nudge progress shows to the right of the
// bar), and empty names fall back to the step/role so the bar is never blank.
func liveAgentLabel(a *liveAgent) string {
	name := a.info.AgentName
	if before, _, ok := strings.Cut(name, " · Nudge"); ok {
		name = before
	}
	if strings.TrimSpace(name) != "" {
		return name
	}
	return firstNonEmptyLive(a.scope.Step, a.info.AgentRole)
}

// liveProgressBarWidth is the width of the agent progress bar. The agent label
// now lives inside the bar (the old separate left column is gone), so the bar is
// wide enough to hold a role:name plus the trailing percentage.
const liveProgressBarWidth = 44

const (
	// While a turn runs, the bar creeps this far toward the next step's fraction
	// (never quite reaching it) with the slow time constant; once the step lands,
	// it catches up to the confirmed fraction with the fast constant.
	liveBarCreepFraction = 0.9
	liveBarCreepSeconds  = 15.0
	liveBarSnapSeconds   = 0.5
)

// animateFraction returns the bar's displayed fraction, easing it toward the
// upcoming step while a turn runs and snapping it up once a step is reached. It
// mutates the agent's animation state and so must be called once per redraw
// under the renderer lock.
func (a *liveAgent) animateFraction(now time.Time) float64 {
	if a.done {
		a.shown = 1
		return 1
	}
	future := max(a.info.NudgeTotal-a.info.NudgeIndex, 0)
	denom := a.doneTurns + future
	if a.activeTurn {
		denom++
	}
	reached := 0.0
	if denom > 0 {
		reached = float64(a.doneTurns) / float64(denom)
	}
	goal := reached
	if a.activeTurn && denom > 0 {
		next := float64(a.doneTurns+1) / float64(denom)
		goal = reached + (next-reached)*liveBarCreepFraction
	}
	// First observation (e.g. a freshly constructed agent in tests): show the
	// confirmed fraction immediately, then animate from there.
	if a.animAt.IsZero() {
		a.shown, a.animAt = reached, now
		return a.shown
	}
	dt := now.Sub(a.animAt).Seconds()
	a.animAt = now
	// Only ever move the bar forward. denom grows by one whenever a turn goes
	// active, which lowers reached/goal for the same doneTurns; without this guard
	// the bar (and percentage) would visibly run backwards. When goal dips below
	// the current fraction we simply hold until real progress overtakes it.
	if dt > 0 && goal > a.shown {
		tau := liveBarCreepSeconds
		if a.shown < reached {
			tau = liveBarSnapSeconds // a step landed; catch up fast
		}
		a.shown += (goal - a.shown) * (1 - math.Exp(-dt/tau))
	}
	a.shown = min(max(a.shown, 0), 1)
	return a.shown
}

func formatLiveAgent(a *liveAgent, now time.Time, useANSI bool) string {
	bar := progressBar(liveAgentLabel(a), a.animateFraction(now), liveProgressBarWidth, a.colorIndex, useANSI)
	elapsed := now.Sub(a.phaseStart)
	limit := "∞"
	if !a.deadline.IsZero() {
		limit = shortDuration(a.deadline.Sub(a.phaseStart))
	}
	phase := formatLivePhase(a, useANSI)
	timing := fmt.Sprintf("%s / %s", shortDuration(elapsed), limit)
	if useANSI {
		timing = progressGrey(timing)
	}
	return "  " + bar + "  " + phase + "  " + timing
}

// liveAgentPhaseWidth right-pads the turn/nudge column so the timing column
// aligns; sized for the widest realistic value ("#N · nudges N/N").
const liveAgentPhaseWidth = 18

// formatLivePhase renders the turn counter and nudge progress. The "#N" round
// uses the same colouring as --show-progress (formatProgressTurn: a darker-green
// "#", bright-green number); nudges follow with grey separators/slash.
func formatLivePhase(a *liveAgent, useANSI bool) string {
	turnNum := max(a.turn, 1)
	// Reserve room for a two-digit round from the start so "· nudges" lines up
	// whether the round is #3 or #12.
	turn := fmt.Sprintf("#%-2d", turnNum)
	visible := turn
	if a.info.NudgeTotal > 0 {
		visible = fmt.Sprintf("%s · nudges %d/%d", turn, a.info.NudgeIndex, a.info.NudgeTotal)
	}
	pad := strings.Repeat(" ", max(liveAgentPhaseWidth-len([]rune(visible)), 0))
	if !useANSI {
		return visible + pad
	}
	// formatProgressTurn has no padding; add the two-digit-alignment spaces back.
	styled := formatProgressTurn(true, turnNum) +
		strings.Repeat(" ", max(2-len(fmt.Sprintf("%d", turnNum)), 0))
	if a.info.NudgeTotal > 0 {
		styled += progressGrey(" · ") + progressLight("nudges ") +
			progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", a.info.NudgeIndex)) +
			progressGrey("/") +
			progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", a.info.NudgeTotal))
	}
	return styled + pad
}

// progressBar renders the agent label inside a filled bar tinted with the
// agent's own colour. The filled portion is a light shade of the agent colour;
// the unfilled portion is a dark shade. The label (ellipsised to fit) reads dark
// on the light fill and light on the dark remainder, with the percentage pinned
// to the right. Only the percentage digits are bold — the label, spaces and "%"
// sign stay regular weight.
func progressBar(label string, fraction float64, width, colorIndex int, useANSI bool) string {
	percent := min(max(int(fraction*100+0.5), 0), 100)
	right := []rune(fmt.Sprintf(" %3d%%", percent)) // 5 columns: "   0%" … " 100%"
	// One space of padding at each end insets the label/percentage from the bar's
	// coloured edges. Callers use a fixed width comfortably larger than the two
	// pads plus the percentage suffix (liveProgressBarWidth = 44); labelWidth just
	// absorbs any slack.
	labelWidth := max(width-2-len(right), 0)
	labelStr := padOrTrim(label, labelWidth)
	text := make([]rune, 0, width)
	bold := make([]bool, 0, width)
	text = append(text, ' ')
	bold = append(bold, false)
	for _, r := range labelStr {
		text = append(text, r)
		bold = append(bold, false)
	}
	for _, r := range right {
		text = append(text, r)
		bold = append(bold, r >= '0' && r <= '9')
	}
	text = append(text, ' ')
	bold = append(bold, false)
	if len(text) > width {
		text, bold = text[:width], bold[:width]
	}
	n := len(text)
	filled := int(fraction*float64(n) + 0.5)
	filled = min(max(filled, 0), n)
	// Keep the percentage suffix a single visual piece: a partial fill sweeps
	// under the label but stops before the digits, so the percentage never
	// straddles two backgrounds. Only a complete bar tints the suffix too.
	percentStart := min(1+labelWidth, n)
	if filled > percentStart && filled < n {
		filled = percentStart
	}
	if !useANSI {
		return string(text)
	}
	c := liveAgentPastelColor(colorIndex)
	fillFg := rgbSGR("38;2", scaleRGB(c, 0.22)) // dark text on light fill
	fillBg := rgbSGR("48;2", c)                 // light fill
	baseFg := rgbSGR("38;2", c)                 // light text on dark remainder
	baseBg := rgbSGR("48;2", scaleRGB(c, 0.42)) // dark remainder
	var b strings.Builder
	lastSGR := ""
	for i, r := range text {
		fg, bg := baseFg, baseBg
		if i < filled {
			fg, bg = fillFg, fillBg
		}
		weight := ""
		if bold[i] {
			weight = "1;"
		}
		sgr := weight + fg + ";" + bg
		if sgr != lastSGR {
			fmt.Fprintf(&b, "\x1b[0m\x1b[%sm", sgr)
			lastSGR = sgr
		}
		b.WriteRune(r)
	}
	b.WriteString(progressColorReset)
	return b.String()
}

// liveAgentPastelRGB holds the per-agent accent colours. Each is used both as a
// light fill and, scaled down, as a dark remainder/text shade in the bar.
var liveAgentPastelRGB = [][3]int{
	{177, 185, 249}, // periwinkle
	{166, 209, 137}, // green
	{244, 184, 228}, // pink
	{239, 159, 118}, // peach
	{129, 200, 190}, // teal
	{198, 160, 246}, // lavender
	{238, 212, 159}, // yellow
	{138, 173, 244}, // blue
}

func liveAgentPastelColor(index int) [3]int {
	return liveAgentPastelRGB[index%len(liveAgentPastelRGB)]
}

func scaleRGB(c [3]int, f float64) [3]int {
	return [3]int{
		min(max(int(float64(c[0])*f+0.5), 0), 255),
		min(max(int(float64(c[1])*f+0.5), 0), 255),
		min(max(int(float64(c[2])*f+0.5), 0), 255),
	}
}

func rgbSGR(prefix string, c [3]int) string {
	return fmt.Sprintf("%s;%d;%d;%d", prefix, c[0], c[1], c[2])
}

func (r *LiveRenderer) findingLineLocked() string {
	f, kept := r.findings, r.keptLocked()
	if !r.useANSI {
		return fmt.Sprintf("  Findings %d · refuted %d · duplicate %d · filtered %d · final %d",
			f.Found, f.Refuted, f.Duplicate, f.Filtered, kept)
	}
	// Each label gets a semantic colour — Findings white, refuted red, duplicate a
	// dim gold, filtered peach, final green — while every count is green and the
	// middle dots stay grey.
	const dupGold = "38;5;179" // dimmer yellow/orange than the warn yellow
	seg := func(labelColor, label string, n int) string {
		return progressStyle(labelColor, label) + " " + progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", n))
	}
	sep := progressGrey(" · ")
	return "  " + seg(progressColorWhite, "Findings", f.Found) + sep +
		seg(progressColorErrorRed, "refuted", f.Refuted) + sep +
		seg(dupGold, "duplicate", f.Duplicate) + sep +
		seg(progressColorProfile, "filtered", f.Filtered) + sep +
		seg(progressColorNumberGreen, "final", kept)
}

func firstNonEmptyLive(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "agent"
}

func padOrTrim(text string, width int) string {
	runes := []rune(text)
	if len(runes) > width {
		if width <= 1 {
			return string(runes[:width])
		}
		return string(runes[:width-1]) + "…"
	}
	return text + strings.Repeat(" ", width-len(runes))
}

func shortDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Round(time.Second).Seconds())
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, (seconds/60)%60, seconds%60)
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func fitLines(lines []string, width int) []string {
	if width <= 0 {
		width = 80
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = truncateANSI(line, width)
	}
	return out
}

func truncateANSI(text string, width int) string {
	if width <= 0 {
		return ""
	}
	visible := 0
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\x1b' && i+1 < len(text) && text[i+1] == '[' {
			start := i
			i += 2
			for i < len(text) && !isANSIFinalByte(text[i]) {
				i++
			}
			if i < len(text) {
				i++
			}
			b.WriteString(text[start:i])
			continue
		}
		if visible >= width {
			break
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		b.WriteRune(r)
		i += size
		visible++
	}
	if strings.Contains(text, "\x1b[") {
		b.WriteString(progressColorReset)
	}
	return b.String()
}

func styleLiveLines(useANSI bool, lines []string, final bool) []string {
	if !useANSI {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.Contains(line, "\x1b[") {
			out[i] = line
			continue
		}
		code := progressColorLightGrey
		if i == 0 {
			code = "1;38;5;81"
		} else if i == len(lines)-1 {
			code = progressColorNumberGreen
		} else if !final && i == 1 {
			code = progressColorMutedModel
		}
		if line != "" {
			out[i] = progressStyle(code, line)
		}
	}
	return out
}

func (r *LiveRenderer) termSize() (int, int) {
	if r.width > 0 && r.height > 0 {
		return r.width, r.height
	}
	if r.fd >= 0 {
		if width, height, err := term.GetSize(r.fd); err == nil && width > 0 && height > 0 {
			return width, height
		}
	}
	return 80, 24
}

func (r *LiveRenderer) writeFrameLocked(lines []string) {
	maxRows := max(r.lastRows, len(lines))
	var b strings.Builder
	if r.lastRows > 0 {
		fmt.Fprintf(&b, "\x1b[%dA", r.lastRows)
	}
	for i := range maxRows {
		b.WriteByte('\r')
		if i < len(lines) {
			b.WriteString(lines[i])
		}
		b.WriteString("\x1b[0K\n")
	}
	if maxRows > len(lines) {
		fmt.Fprintf(&b, "\x1b[%dA", maxRows-len(lines))
	}
	_, _ = io.WriteString(r.w, b.String())
	r.lastRows = len(lines)
}

// WriteOutside preserves scrollback while warning/error output arrives.
func (r *LiveRenderer) WriteOutside(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Once finished, the dashboard is a frozen snapshot: emit the text below it
	// without any cursor manipulation, so a late (racing) write cannot redraw
	// the final frame on top of scrollback.
	if r.closed {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		_, _ = io.WriteString(r.w, text)
		return
	}
	var b strings.Builder
	if r.lastRows > 0 {
		fmt.Fprintf(&b, "\x1b[%dA", r.lastRows)
		for i := 0; i < r.lastRows; i++ {
			b.WriteString("\r\x1b[0K\n")
		}
		fmt.Fprintf(&b, "\x1b[%dA", r.lastRows)
	}
	b.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		b.WriteByte('\n')
	}
	_, _ = io.WriteString(r.w, b.String())
	r.lastRows = 0
	r.writeFrameLocked(r.buildLinesLocked())
}
