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
	"unicode/utf8"

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
	colorIndex  int
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
		a = &liveAgent{key: key, colorIndex: r.nextColor}
		r.nextColor++
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
			lines = append(lines, formatLiveAgent(visible[i], now, r.useANSI))
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
	name := ""
	unit, total := 0, r.plan.Units
	for _, scope := range r.steps {
		if name == "" && scope.Workflow != "" {
			name = scope.Workflow
		}
		if scope.Unit > unit {
			unit = scope.Unit
		}
		if scope.UnitTotal > total {
			total = scope.UnitTotal
		}
	}
	if name == "" {
		name = "Workflow"
	}
	return fmt.Sprintf("  %s · %d/%d", name, unit, max(total, 1))
}

// liveAgentLabel is the left-hand agent name. Any nudge suffix is stripped —
// nudge progress is reported to the right of the bar, never in the name.
func liveAgentLabel(a *liveAgent) string {
	info := a.info
	if before, _, ok := strings.Cut(info.AgentName, " · Nudge"); ok {
		info.AgentName = before
	}
	label := info.roleName()
	if label == "" {
		label = firstNonEmptyLive(info.AgentName, a.scope.Step, info.AgentRole)
	}
	return label
}

func formatLiveAgent(a *liveAgent, now time.Time, useANSI bool) string {
	label := liveAgentLabel(a)
	phase := fmt.Sprintf("#%d", max(a.turn, 1))
	if a.info.NudgeTotal > 0 {
		phase += fmt.Sprintf(" · nudges %d/%d", a.info.NudgeIndex, a.info.NudgeTotal)
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
	bar := progressBar(fraction, 20, useANSI)
	elapsed := now.Sub(a.phaseStart)
	limit := "∞"
	if !a.deadline.IsZero() {
		limit = shortDuration(a.deadline.Sub(a.phaseStart))
	}
	label = padOrTrim(label, 44)
	phase = padOrTrim(phase, 27)
	timing := fmt.Sprintf("%s / %s", shortDuration(elapsed), limit)
	if useANSI {
		label = progressStyle(liveAgentPastel(a.colorIndex), label)
		phase = progressLight(phase)
		timing = progressGrey(timing)
	}
	return "  " + label + "  " + bar + "  " + phase + "  " + timing
}

func progressBar(fraction float64, width int, useANSI bool) string {
	filled := int(fraction * float64(width))
	filled = min(max(filled, 0), width)
	percent := min(max(int(fraction*100+0.5), 0), 100)
	if !useANSI {
		return strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + fmt.Sprintf(" %3d%%", percent)
	}
	left := " Progress"
	right := fmt.Sprintf("%3d%% ", percent)
	middle := max(width-len(left)-len(right), 0)
	text := left + strings.Repeat(" ", middle) + right
	if len(text) > width {
		text = text[:width]
	}
	var b strings.Builder
	lastSGR := ""
	for i := range width {
		fg, bg := "38;2;255;255;255", "48;2;80;83;112"
		if i < filled {
			fg, bg = "38;2;40;42;64", "48;2;177;185;249"
		}
		bold := ""
		if i >= 1 && i <= len("Progress") {
			bold = "1;"
		}
		sgr := bold + fg + ";" + bg
		if sgr != lastSGR {
			fmt.Fprintf(&b, "\x1b[0m\x1b[%sm", sgr)
			lastSGR = sgr
		}
		b.WriteByte(text[i])
	}
	b.WriteString(progressColorReset)
	return b.String()
}

var liveAgentPastels = []string{
	"38;2;177;185;249", // periwinkle
	"38;2;166;209;137", // green
	"38;2;244;184;228", // pink
	"38;2;239;159;118", // peach
	"38;2;129;200;190", // teal
	"38;2;198;160;246", // lavender
	"38;2;238;212;159", // yellow
	"38;2;138;173;244", // blue
}

func liveAgentPastel(index int) string {
	return liveAgentPastels[index%len(liveAgentPastels)]
}

func (r *LiveRenderer) findingLineLocked() string {
	return fmt.Sprintf("  Findings: %d · refuted %d · duplicate %d · filtered %d · final %d",
		r.findings.Found, r.findings.Refuted, r.findings.Duplicate, r.findings.Filtered, r.keptLocked())
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
