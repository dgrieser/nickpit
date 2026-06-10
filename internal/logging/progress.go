package logging

import (
	"context"
	"fmt"
	"strings"
	"unicode"
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
		return progressColorLightGrey
	case StateError:
		return progressColorErrorRed
	case StateRetry, StateWarn:
		return progressColorWarnYellow
	case StateSkip:
		return progressColorDarkGrey
	default:
		return progressColorLightGrey
	}
}

const (
	progressColorReset             = "\x1b[0m"
	progressColorBold              = "1"
	progressColorGrey              = "38;5;244"
	progressColorDarkGrey          = "38;5;242"
	progressColorLightGrey         = "38;5;252"
	progressColorWhite             = "38;5;255"
	progressColorNumberGreen       = "38;5;118"
	progressColorUnitGreen         = "38;5;71"
	progressColorKeyTurquoise      = "38;5;116"
	progressColorKeyTeal           = "38;5;37"
	progressColorStringGreen       = "38;5;120"
	progressColorBoolGreen         = "38;5;156"
	progressColorTaskPink          = "38;5;218"
	progressColorMutedModel        = "38;5;110"
	progressColorURLPurpleBlue     = "38;5;105"
	progressColorProfile           = "38;5;216"
	progressColorBranchFromGold    = "38;5;214"
	progressColorBranchToAquaGreen = "38;5;48"
	progressColorErrorRed          = "38;5;203"
	progressColorWarnYellow        = "38;5;221"
)

var progressStageStyles = map[Stage]string{
	StageModel:      "1;38;5;81",
	StageAgent:      "1;38;5;111",
	StageModelCheck: "1;38;5;114",
	StageReview:     "1;38;5;219",
	StageRequest:    "1;38;5;214",
	StageReasoning:  "1;38;5;183",
	StageResponse:   "1;38;5;150",
	StageTool:       "1;38;5;80",
	StageVerify:     "1;38;5;123",
	StageFinalize:   "1;38;5;207",
	StageSummarize:  "1;38;5;222",
	StagePublish:    "1;38;5;209",
	StageResult:     "1;38;5;118",
}

func progressStyle(code, text string) string {
	if text == "" {
		return ""
	}
	return "\x1b[" + code + "m" + text + progressColorReset
}

func progressGrey(text string) string {
	return progressStyle(progressColorGrey, text)
}

func progressLight(text string) string {
	return progressStyle(progressColorLightGrey, text)
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
		style := progressStageStyles[stage]
		if style == "" {
			style = progressColorBold + ";" + progressColorLightGrey
		}
		b.WriteString(progressStyle(style, paddedStage))
	} else {
		b.WriteString(paddedStage)
	}
	if bracket := formatProgressBracket(useANSI, info); bracket != "" {
		b.WriteString(" ")
		b.WriteString(bracket)
	}
	if info.Turn > 0 {
		turn := formatProgressTurn(useANSI, info.Turn)
		b.WriteString(" ")
		b.WriteString(turn)
	}
	if state != StateNone {
		word := string(state)
		if useANSI {
			word = progressStyle(stateColor(state), word)
		}
		b.WriteString(" ")
		b.WriteString(word)
	}
	if msg != "" {
		if useANSI {
			msg = colorizeProgressMessage(msg)
		}
		b.WriteString(" ")
		b.WriteString(msg)
	}
	b.WriteString("\n")
	return b.String()
}

func formatProgressBracket(useANSI bool, info ProgressInfo) string {
	var parts []string
	if useANSI {
		if rolePart := formatProgressRolePart(info); rolePart != "" {
			parts = append(parts, rolePart)
		}
	} else if roleName := info.roleName(); roleName != "" {
		parts = append(parts, roleName)
	}
	if modelPart := formatProgressModel(useANSI, info, len(parts) > 0); modelPart != "" {
		parts = append(parts, modelPart)
	}
	if info.Detail != "" {
		detail := info.Detail
		if useANSI {
			detail = progressLight(detail)
		}
		parts = append(parts, detail)
	}
	if len(parts) == 0 {
		return ""
	}
	sep := " · "
	open, close := "[", "]"
	if useANSI {
		sep = progressGrey(" · ")
		open, close = progressGrey("["), progressGrey("]")
	}
	return open + strings.Join(parts, sep) + close
}

func formatProgressRolePart(info ProgressInfo) string {
	switch {
	case info.AgentRole == "":
		return styleSeparatedText(info.AgentName, progressColorTaskPink)
	case info.AgentName == "":
		return progressStyle(progressColorKeyTeal, info.AgentRole)
	case strings.HasPrefix(info.AgentName, "#"):
		return progressStyle(progressColorKeyTeal, info.AgentRole) + " " + formatProgressCounter(info.AgentName)
	default:
		return progressStyle(progressColorKeyTeal, info.AgentRole) + progressGrey(": ") + styleSeparatedText(info.AgentName, progressColorTaskPink)
	}
}

func styleSeparatedText(text, color string) string {
	var b strings.Builder
	var segment strings.Builder
	flush := func() {
		if segment.Len() == 0 {
			return
		}
		b.WriteString(progressStyle(color, segment.String()))
		segment.Reset()
	}
	for _, r := range text {
		switch r {
		case ':':
			flush()
			b.WriteString(progressGrey(":"))
		case '/':
			flush()
			b.WriteString(progressGrey("/"))
		default:
			segment.WriteRune(r)
		}
	}
	flush()
	return b.String()
}

func formatProgressModel(useANSI bool, info ProgressInfo, scoped bool) string {
	if info.Model == "" {
		return ""
	}
	var b strings.Builder
	if useANSI {
		modelColor := progressColorKeyTeal
		effortColor := progressColorTaskPink
		if scoped {
			modelColor = progressColorMutedModel
			effortColor = progressColorMutedModel
		}
		b.WriteString(progressStyle(modelColor, info.Model))
		if info.Effort != "" {
			b.WriteString(progressGrey(":") + progressStyle(effortColor, info.Effort))
		}
		if info.BaseURL != "" {
			b.WriteString(progressGrey(" @ ") + progressStyle(progressColorURLPurpleBlue, info.BaseURL))
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

func formatProgressTurn(useANSI bool, turn int) string {
	if !useANSI {
		return fmt.Sprintf("#%d", turn)
	}
	return progressStyle(progressColorUnitGreen, "#") + progressStyle(progressColorNumberGreen, fmt.Sprintf("%d", turn))
}

func formatProgressCounter(counter string) string {
	if strings.HasPrefix(counter, "#") && len(counter) > 1 {
		return progressStyle(progressColorUnitGreen, "#") + progressStyle(progressColorNumberGreen, counter[1:])
	}
	return progressLight(counter)
}

func colorizeProgressMessage(text string) string {
	if text == "" {
		return ""
	}
	if reviewCtx, ok := colorizeReviewContextMessage(text); ok {
		return reviewCtx
	}
	return colorizeProgressText(text)
}

func colorizeReviewContextMessage(text string) (string, bool) {
	mode, rest, ok := strings.Cut(text, " [")
	if !ok {
		return "", false
	}
	profileAndThreshold, rest, ok := strings.Cut(rest, "] on ")
	if !ok {
		return "", false
	}
	repo, rest, ok := strings.Cut(rest, " @ ")
	if !ok {
		return "", false
	}
	head, base, ok := strings.Cut(rest, " → ")
	if !ok {
		return "", false
	}
	profile, threshold, ok := strings.Cut(profileAndThreshold, ", ")
	if !ok {
		return "", false
	}

	var b strings.Builder
	b.WriteString(styleModeWithSubmode(mode))
	b.WriteString(" ")
	b.WriteString(progressGrey("["))
	b.WriteString(progressStyle(progressColorProfile, profile))
	b.WriteString(progressGrey(", "))
	b.WriteString(colorizeProgressText(threshold))
	b.WriteString(progressGrey("]"))
	b.WriteString(" ")
	b.WriteString(progressLight("on"))
	b.WriteString(" ")
	b.WriteString(stylePathParts(repo, progressColorTaskPink))
	b.WriteString(progressGrey(" @ "))
	b.WriteString(stylePathParts(head, progressColorBranchFromGold))
	b.WriteString(progressGrey(" → "))
	b.WriteString(stylePathParts(base, progressColorBranchToAquaGreen))
	return b.String(), true
}

func styleModeWithSubmode(mode string) string {
	before, after, ok := strings.Cut(mode, ":")
	if !ok {
		return progressLight(mode)
	}
	return progressLight(before) + progressGrey(":") + progressLight(after)
}

func stylePathParts(path, color string) string {
	return styleSeparatedText(path, color)
}

func colorizeProgressText(text string) string {
	runes := []rune(text)
	var b strings.Builder
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			b.WriteRune(r)
			i++
		case r == '"':
			i = writeQuotedProgressString(&b, runes, i)
		case isProgressSeparator(r):
			b.WriteString(progressGrey(string(r)))
			i++
		case r == '#':
			i = writeProgressHashNumber(&b, runes, i)
		case r == '≤' || r == '≥':
			i = writeProgressComparator(&b, runes, i)
		case r == '∞':
			b.WriteString(progressStyle(progressColorUnitGreen, string(r)))
			i++
		case unicode.IsDigit(r):
			i = writeProgressNumber(&b, runes, i, true)
		case isProgressWordStart(r):
			i = writeProgressWord(&b, runes, i)
		default:
			b.WriteString(progressLight(string(r)))
			i++
		}
	}
	return b.String()
}

func writeQuotedProgressString(b *strings.Builder, runes []rune, i int) int {
	j := i + 1
	escaped := false
	for j < len(runes) {
		switch {
		case escaped:
			escaped = false
		case runes[j] == '\\':
			escaped = true
		case runes[j] == '"':
			j++
			b.WriteString(progressStyle(progressColorStringGreen, string(runes[i:j])))
			return j
		}
		j++
	}
	b.WriteString(progressStyle(progressColorStringGreen, string(runes[i:])))
	return len(runes)
}

func writeProgressHashNumber(b *strings.Builder, runes []rune, i int) int {
	if i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
		b.WriteString(progressStyle(progressColorUnitGreen, "#"))
		return writeProgressNumber(b, runes, i+1, true)
	}
	b.WriteString(progressLight("#"))
	return i + 1
}

func writeProgressComparator(b *strings.Builder, runes []rune, i int) int {
	b.WriteString(progressStyle(progressColorUnitGreen, string(runes[i])))
	i++
	if i >= len(runes) {
		return i
	}
	if unicode.IsDigit(runes[i]) {
		return writeProgressNumber(b, runes, i, true)
	}
	if unicode.IsLetter(runes[i]) {
		j := i + 1
		for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j])) {
			j++
		}
		b.WriteString(progressStyle(progressColorNumberGreen, string(runes[i:j])))
		return j
	}
	return i
}

func writeProgressNumber(b *strings.Builder, runes []rune, i int, withUnit bool) int {
	j := i
	for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '.') {
		j++
	}
	b.WriteString(progressStyle(progressColorNumberGreen, string(runes[i:j])))
	if j < len(runes) && runes[j] == '/' && j+1 < len(runes) && unicode.IsDigit(runes[j+1]) {
		b.WriteString(progressStyle(progressColorUnitGreen, "/"))
		return writeProgressNumber(b, runes, j+1, false)
	}
	if withUnit {
		k := j
		for k < len(runes) && unicode.IsLetter(runes[k]) {
			k++
		}
		if k > j {
			b.WriteString(progressStyle(progressColorUnitGreen, string(runes[j:k])))
			return k
		}
	}
	return j
}

func writeProgressWord(b *strings.Builder, runes []rune, i int) int {
	j := i + 1
	for j < len(runes) && isProgressWordChar(runes[j]) {
		j++
	}
	word := string(runes[i:j])
	if j < len(runes) && runes[j] == '=' {
		b.WriteString(progressStyle(progressColorKeyTurquoise, word))
		return j
	}
	switch word {
	case "true", "false":
		b.WriteString(progressStyle(progressColorBoolGreen, word))
	default:
		b.WriteString(progressLight(word))
	}
	return j
}

func isProgressSeparator(r rune) bool {
	switch r {
	case '[', ']', '(', ')', ',', ':', '=', '@', '→', '·', '/':
		return true
	default:
		return false
	}
}

func isProgressWordStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isProgressWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}
