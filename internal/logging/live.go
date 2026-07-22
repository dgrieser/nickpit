package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/term"
)

// LivePlan describes the bounded terminal area available to a review run.
type LivePlan struct {
	Concurrency int
	Units       int
}

// WorkflowScope identifies the pipeline work enclosing an agent loop.
type WorkflowScope struct {
	Unit      int
	UnitTotal int
	Lane      string
	Step      string
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
	Lane           string
	Found          int
	Refuted        int
	Duplicate      int
	Filtered       int
	Current        int
	CurrentPresent bool
}

type liveFindingStats struct {
	Found, Refuted, Duplicate, Filtered int
	CurrentByLane                       map[string]int
}

type liveAgent struct {
	key         string
	info        ProgressInfo
	scope       WorkflowScope
	phaseStart  time.Time
	deadline    time.Time
	doneTurns   int
	activeTurn  bool
	visible     bool
	turn        int
	lastStarted time.Time
}

// LiveRenderer owns a fixed dashboard below existing terminal scrollback.
type LiveRenderer struct {
	mu       sync.Mutex
	w        io.Writer
	fd       int
	useANSI  bool
	plan     LivePlan
	started  time.Time
	agents   map[string]*liveAgent
	steps    map[string]WorkflowScope
	findings liveFindingStats
	lastRows int
	final    []string
	closed   bool
	wake     chan struct{}
	stop     chan struct{}
	done     chan struct{}
	width    int // tests
	height   int // tests
	now      func() time.Time
}

func newLiveRenderer(w io.Writer, useANSI bool, plan LivePlan) *LiveRenderer {
	fd := -1
	if f, ok := w.(*os.File); ok && f != nil {
		fd = int(f.Fd())
	}
	r := &LiveRenderer{
		w: w, fd: fd, useANSI: useANSI, plan: plan, started: time.Now(),
		agents: make(map[string]*liveAgent), steps: make(map[string]WorkflowScope),
		findings: liveFindingStats{CurrentByLane: make(map[string]int)},
		wake:     make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
		now: time.Now,
	}
	go r.run()
	return r
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
		a = &liveAgent{key: key}
		r.agents[key] = a
	}
	now := r.now()
	a.info, a.scope, a.phaseStart, a.deadline = info, scope, now, deadline
	a.visible, a.lastStarted = true, now
	r.mu.Unlock()
	r.signal()
}

func (r *LiveRenderer) AgentDone(info ProgressInfo) {
	r.mu.Lock()
	if a := r.agents[liveAgentKey(info)]; a != nil {
		a.visible = false
		a.activeTurn = false
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

func (r *LiveRenderer) Findings(update FindingUpdate) {
	r.mu.Lock()
	r.findings.Found += max(update.Found, 0)
	r.findings.Refuted += max(update.Refuted, 0)
	r.findings.Duplicate += max(update.Duplicate, 0)
	r.findings.Filtered += max(update.Filtered, 0)
	if update.CurrentPresent && update.Lane != "" {
		r.findings.CurrentByLane[update.Lane] = max(update.Current, 0)
	}
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
		fmt.Sprintf("%s %s · %s · %d findings", mark, word, shortDuration(elapsed), findings),
		r.findingLineLocked(),
	}
	r.closed = true
	r.mu.Unlock()
	close(r.stop)
	<-r.done
}

func (r *LiveRenderer) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	elapsed := r.now().Sub(r.started)
	r.final = []string{fmt.Sprintf("✗ Review stopped · %s", shortDuration(elapsed)), r.findingLineLocked()}
	r.closed = true
	r.mu.Unlock()
	close(r.stop)
	<-r.done
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
	for _, a := range r.agents {
		if a.visible {
			visible = append(visible, a)
		}
	}
	sort.Slice(visible, func(i, j int) bool {
		if visible[i].lastStarted.Equal(visible[j].lastStarted) {
			return visible[i].key < visible[j].key
		}
		return visible[i].lastStarted.Before(visible[j].lastStarted)
	})
	slots := r.plan.Concurrency
	if slots <= 0 || slots > height-4 {
		slots = height - 4
	}
	if slots < 1 {
		slots = 1
	}
	hidden := max(len(visible)-slots, 0)
	activeLabel := fmt.Sprintf("%d active", len(visible))
	if r.plan.Concurrency > 0 {
		activeLabel = fmt.Sprintf("%d/%d agents", len(visible), r.plan.Concurrency)
	}
	if hidden > 0 {
		activeLabel += fmt.Sprintf(" · +%d hidden", hidden)
	}
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spin := spinner[int(now.Sub(r.started)/(100*time.Millisecond))%len(spinner)]
	lines := []string{fmt.Sprintf("%s NickPit reviewing · %s · %s", spin, shortDuration(now.Sub(r.started)), activeLabel), r.stepLineLocked()}
	for i := 0; i < slots; i++ {
		if i < len(visible) {
			lines = append(lines, formatLiveAgent(visible[i], now))
		} else {
			lines = append(lines, "")
		}
	}
	lines = append(lines, r.findingLineLocked())
	return styleLiveLines(r.useANSI, fitLines(lines, width), false)
}

func (r *LiveRenderer) stepLineLocked() string {
	if len(r.steps) == 0 {
		return "  Preparing review"
	}
	counts := map[string]int{}
	unit, total := 0, r.plan.Units
	for _, scope := range r.steps {
		counts[scope.Step]++
		if scope.Unit > unit {
			unit = scope.Unit
		}
		if scope.UnitTotal > total {
			total = scope.UnitTotal
		}
	}
	keys := make([]string, 0, len(counts))
	for step := range counts {
		keys = append(keys, step)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, step := range keys {
		if counts[step] > 1 {
			parts = append(parts, fmt.Sprintf("%s×%d", step, counts[step]))
		} else {
			parts = append(parts, step)
		}
	}
	return fmt.Sprintf("  Workflow %d/%d · %s", unit, max(total, 1), strings.Join(parts, " · "))
}

func formatLiveAgent(a *liveAgent, now time.Time) string {
	label := shortLane(firstNonEmptyLive(a.scope.Lane, a.info.Group, a.info.AgentName))
	step := firstNonEmptyLive(a.scope.Step, a.info.AgentRole)
	phase := fmt.Sprintf("turn %d", max(a.turn, 1))
	if a.info.NudgeTotal > 0 {
		if a.info.NudgeIndex > 0 {
			phase = fmt.Sprintf("nudge %d/%d · turn %d", a.info.NudgeIndex, a.info.NudgeTotal, max(a.turn, 1))
		} else {
			phase += fmt.Sprintf(" · nudges 0/%d", a.info.NudgeTotal)
		}
	}
	future := max(a.info.NudgeTotal-a.info.NudgeIndex, 0)
	denom := a.doneTurns + future
	if a.activeTurn {
		denom++
	}
	fraction := 0.0
	if denom > 0 {
		fraction = float64(a.doneTurns) / float64(denom)
	}
	bar := progressBar(fraction, a.activeTurn, 12, now)
	elapsed := now.Sub(a.phaseStart)
	limit := "∞"
	if !a.deadline.IsZero() {
		limit = shortDuration(a.deadline.Sub(a.phaseStart))
	}
	return fmt.Sprintf("  %-5s %-18s %s %s · %s/%s", label, step, bar, phase, shortDuration(elapsed), limit)
}

func progressBar(fraction float64, active bool, width int, now time.Time) string {
	filled := int(fraction * float64(width))
	filled = min(max(filled, 0), width)
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < width; i++ {
		switch {
		case i < filled:
			b.WriteRune('█')
		case active && i == filled && int(now.UnixMilli()/100)%2 == 0:
			b.WriteRune('▓')
		default:
			b.WriteRune('░')
		}
	}
	b.WriteByte(']')
	return b.String()
}

func (r *LiveRenderer) findingLineLocked() string {
	lanes := make([]string, 0, len(r.findings.CurrentByLane))
	for lane := range r.findings.CurrentByLane {
		lanes = append(lanes, lane)
	}
	sort.Strings(lanes)
	parts := make([]string, 0, len(lanes))
	for _, lane := range lanes {
		parts = append(parts, fmt.Sprintf("%s %d", shortLane(lane), r.findings.CurrentByLane[lane]))
	}
	prefix := "Findings"
	if len(parts) > 0 {
		prefix += " " + strings.Join(parts, " ") + " ·"
	}
	return fmt.Sprintf("  %s found %d · refuted %d · duplicate %d · filtered %d · kept %d",
		prefix, r.findings.Found, r.findings.Refuted, r.findings.Duplicate, r.findings.Filtered, r.keptLocked())
}

func shortLane(name string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	known := map[string]string{
		"code quality": "CQ", "codequality": "CQ", "security": "SEC", "architecture": "ARCH",
		"performance": "PERF", "testing": "TEST", "best practices": "BP", "bestpractices": "BP",
	}
	if v := known[clean]; v != "" {
		return v
	}
	var out []rune
	for _, ch := range strings.ToUpper(name) {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			out = append(out, ch)
			if len(out) == 5 {
				break
			}
		}
	}
	if len(out) == 0 {
		return "AGENT"
	}
	return string(out)
}

func firstNonEmptyLive(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "agent"
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
		runes := []rune(line)
		if len(runes) > width {
			if width > 1 {
				line = string(runes[:width-1]) + "…"
			} else {
				line = string(runes[:width])
			}
		}
		out[i] = line
	}
	return out
}

func styleLiveLines(useANSI bool, lines []string, final bool) []string {
	if !useANSI {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
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
	for i := 0; i < maxRows; i++ {
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
