package logging

import (
	"context"
	"fmt"
	"strings"
)

// ProgressInfo is the unified identity carried in context.Context for progress
// and verbose logging. Fields are plain strings so logging stays free of
// config/model imports.
type ProgressInfo struct {
	AgentRole string // "context", "review", "verify", "dedupe", "merge", "finalize", "summarize", "extract", "probe"
	AgentName string // "#1", "Verify Findings", "Probe Response:high", ...
	Detail    string // finding title or similar, caller-truncated
	Model     string
	Effort    string
	BaseURL   string // rendered as "@ url" after model:effort
	Turn      int    // LLM call number, 0 = absent
}

func (p ProgressInfo) IsZero() bool {
	return p == ProgressInfo{}
}

// WithTurn returns a copy with Turn set.
func (p ProgressInfo) WithTurn(turn int) ProgressInfo {
	p.Turn = turn
	return p
}

// roleName joins agent role and name: "reviewer #1" when the name is a
// "#N"-style counter, "verify: Verify Findings" otherwise.
func (p ProgressInfo) roleName() string {
	switch {
	case p.AgentRole == "":
		return p.AgentName
	case p.AgentName == "":
		return p.AgentRole
	case strings.HasPrefix(p.AgentName, "#"):
		return p.AgentRole + " " + p.AgentName
	default:
		return p.AgentRole + ": " + p.AgentName
	}
}

// Label renders the reasoning-section banner label, e.g.
// "verifier #2: Missing error handling #1".
func (p ProgressInfo) Label() string {
	label := p.roleName()
	if p.Detail != "" {
		if label != "" {
			label += ": " + p.Detail
		} else {
			label = p.Detail
		}
	}
	if p.Turn > 0 {
		if label != "" {
			label += fmt.Sprintf(" #%d", p.Turn)
		} else {
			label = fmt.Sprintf("#%d", p.Turn)
		}
	}
	return label
}

// VerbosePrefix renders the verbose-log agent prefix, byte-identical to the
// previous formatAgentTag output: "[role: name, turn: #N] ".
func (p ProgressInfo) VerbosePrefix() string {
	if p.AgentRole == "" && p.AgentName == "" {
		return ""
	}
	head := fmt.Sprintf("%s: %s", p.AgentRole, p.AgentName)
	if p.Turn > 0 {
		head = fmt.Sprintf("%s, turn: #%d", head, p.Turn)
	}
	return "[" + head + "] "
}

type progressInfoKey struct{}

func WithProgressInfo(ctx context.Context, info ProgressInfo) context.Context {
	return context.WithValue(ctx, progressInfoKey{}, info)
}

func ProgressInfoFromContext(ctx context.Context) (ProgressInfo, bool) {
	if ctx == nil {
		return ProgressInfo{}, false
	}
	info, ok := ctx.Value(progressInfoKey{}).(ProgressInfo)
	if !ok || info.IsZero() {
		return ProgressInfo{}, false
	}
	return info, true
}

// Stage identifies the pipeline stage a progress line belongs to. Rendered as
// the first, fixed-width column so the context brackets align.
type Stage string

const (
	StageModel      Stage = "Model"
	StageAgent      Stage = "Agent"
	StageReview     Stage = "Review"
	StageModelCheck Stage = "ModelCheck"
	StageRequest    Stage = "Request"
	StageResponse   Stage = "Response"
	StageReasoning  Stage = "Reasoning"
	StageTool       Stage = "Tool"
	StageResult     Stage = "Result"
	StagePublish    Stage = "Publish"
	StageVerify     Stage = "Verify"
	StageFinalize   Stage = "Finalize"
	StageSummarize  Stage = "Summarize"
)

// allStages exists for the column-width guard test.
var allStages = []Stage{
	StageModel, StageAgent, StageReview, StageModelCheck, StageRequest,
	StageResponse, StageReasoning, StageTool, StageResult, StagePublish,
	StageVerify, StageFinalize, StageSummarize,
}

// stageColumnWidth is the width of the stage column: len(StageModelCheck),
// the longest stage name. Guarded by a test over allStages.
const stageColumnWidth = 10

// State is the colored state word of a progress line.
type State string

const (
	StateNone  State = ""
	StateStart State = "start"
	StateSent  State = "sent"
	StateDone  State = "done"
	StateOK    State = "ok"
	StateReady State = "ready"
	StateError State = "error"
	StateRetry State = "retry"
	StateWarn  State = "warn"
	StateSkip  State = "skip"
)

func stateColor(state State) string {
	switch state {
	case StateDone, StateOK, StateReady:
		return "32"
	case StateError:
		return "31"
	case StateRetry, StateWarn:
		return "33"
	case StateSkip:
		return "90"
	default:
		return "34"
	}
}

// Progress emits a progress line using the ProgressInfo carried in ctx.
func (l *Logger) Progress(ctx context.Context, stage Stage, state State, msg string) {
	if l == nil || !l.showProgress {
		return
	}
	info, _ := ProgressInfoFromContext(ctx)
	l.emitProgress(info, stage, state, msg)
}

// ProgressFor emits a progress line with explicitly provided ProgressInfo.
func (l *Logger) ProgressFor(info ProgressInfo, stage Stage, state State, msg string) {
	if l == nil || !l.showProgress {
		return
	}
	l.emitProgress(info, stage, state, msg)
}

// ProgressToolCall emits a tool-call progress line: "call → result".
func (l *Logger) ProgressToolCall(ctx context.Context, call, result string) {
	if l == nil || !l.showProgress {
		return
	}
	info, _ := ProgressInfoFromContext(ctx)
	l.emitProgress(info, StageTool, StateNone, call+" → "+result)
}

// emitProgress is the single formatting and writing path for progress lines.
func (l *Logger) emitProgress(info ProgressInfo, stage Stage, state State, msg string) {
	line := formatProgressLine(l.useANSI, info, stage, state, msg)
	if l.reasoning != nil {
		l.reasoning.WriteProgress(line)
		return
	}
	l.writeRaw(line)
}

func formatProgressLine(useANSI bool, info ProgressInfo, stage Stage, state State, msg string) string {
	var b strings.Builder
	paddedStage := fmt.Sprintf("%-*s", stageColumnWidth, string(stage))
	if useANSI {
		b.WriteString("\x1b[33m" + paddedStage + "\x1b[0m")
	} else {
		b.WriteString(paddedStage)
	}
	if bracket := formatProgressBracket(useANSI, info); bracket != "" {
		b.WriteString(" ")
		b.WriteString(bracket)
	}
	if info.Turn > 0 {
		turn := fmt.Sprintf("#%d", info.Turn)
		if useANSI {
			turn = "\x1b[32m" + turn + "\x1b[0m"
		}
		b.WriteString(" ")
		b.WriteString(turn)
	}
	if state != StateNone {
		word := string(state)
		if useANSI {
			word = "\x1b[" + stateColor(state) + "m" + word + "\x1b[0m"
		}
		b.WriteString(" ")
		b.WriteString(word)
	}
	if msg != "" {
		if useANSI {
			msg = colorizeKeyValueSummary(msg)
		}
		b.WriteString(" ")
		b.WriteString(msg)
	}
	b.WriteString("\n")
	return b.String()
}

func formatProgressBracket(useANSI bool, info ProgressInfo) string {
	var parts []string
	if roleName := info.roleName(); roleName != "" {
		if useANSI {
			roleName = "\x1b[34m" + roleName + "\x1b[0m"
		}
		parts = append(parts, roleName)
	}
	if modelPart := formatProgressModel(useANSI, info); modelPart != "" {
		parts = append(parts, modelPart)
	}
	if info.Detail != "" {
		detail := info.Detail
		if useANSI {
			detail = "\x1b[90m" + detail + "\x1b[0m"
		}
		parts = append(parts, detail)
	}
	if len(parts) == 0 {
		return ""
	}
	sep := " · "
	open, close := "[", "]"
	if useANSI {
		sep = "\x1b[90m · \x1b[0m"
		open, close = "\x1b[90m[\x1b[0m", "\x1b[90m]\x1b[0m"
	}
	return open + strings.Join(parts, sep) + close
}

func formatProgressModel(useANSI bool, info ProgressInfo) string {
	if info.Model == "" {
		return ""
	}
	var b strings.Builder
	if useANSI {
		b.WriteString("\x1b[34m" + info.Model + "\x1b[0m")
		if info.Effort != "" {
			b.WriteString("\x1b[90m:\x1b[0m\x1b[32m" + info.Effort + "\x1b[0m")
		}
		if info.BaseURL != "" {
			b.WriteString("\x1b[90m @ \x1b[0m\x1b[35m" + info.BaseURL + "\x1b[0m")
		}
		return b.String()
	}
	b.WriteString(info.Model)
	if info.Effort != "" {
		b.WriteString(":" + info.Effort)
	}
	if info.BaseURL != "" {
		b.WriteString(" @ " + info.BaseURL)
	}
	return b.String()
}
