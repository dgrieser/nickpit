package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/textsan"
)

type Logger struct {
	w             io.Writer
	useANSI       bool
	enabled       bool
	showReasoning bool
	showProgress  bool
	reasoning     *ReasoningRenderer
}

// ReasoningSection is a handle to one labeled block in the ReasoningRenderer.
// All methods are nil-safe so callers can defer End() without nil checks.
type ReasoningSection struct {
	r         *ReasoningRenderer
	id        SectionID
	logger    *Logger
	info      ProgressInfo // identity snapshot taken when the section opens
	startTime time.Time
	ended     bool
	callNum   int // incremented by IncrCallNum on each LLM request
}

func (s *ReasoningSection) Append(delta string) {
	if s == nil || s.r == nil {
		return
	}
	s.r.Append(s.id, delta)
}

// Info returns the section's identity snapshot, or the zero value for nil
// sections.
func (s *ReasoningSection) Info() ProgressInfo {
	if s == nil {
		return ProgressInfo{}
	}
	return s.info
}

// IncrCallNum increments the per-section LLM call counter and returns the new value.
func (s *ReasoningSection) IncrCallNum() int {
	if s == nil {
		return 0
	}
	s.callNum++
	return s.callNum
}

func (s *ReasoningSection) End() {
	if s == nil || s.ended {
		return
	}
	s.ended = true
	if s.r != nil {
		s.r.End(s.id)
	}
	if !s.info.IsZero() {
		info := s.info
		if s.callNum > 0 {
			info = info.WithTurn(s.callNum)
		}
		elapsed := time.Since(s.startTime).Truncate(time.Second)
		s.logger.ProgressFor(info, StageReasoning, StateDone, elapsed.String())
	}
}

func New(w io.Writer, enabled bool, useANSI bool) *Logger {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		useANSI = false
	}
	return &Logger{
		w:       w,
		useANSI: useANSI,
		enabled: enabled,
	}
}

func (l *Logger) SetShowReasoning(enabled bool) {
	if l == nil {
		return
	}
	l.showReasoning = enabled
	// Construct the renderer eagerly here. SetShowReasoning is called during
	// command setup, before any reviewer/verifier goroutine starts, so the
	// l.reasoning field is written once before the concurrent readers
	// (emitProgress, writeRaw) ever run. Lazy
	// initialization under sync.Once raced with those unsynchronized readers,
	// since Once only establishes happens-before for goroutines that call Do.
	if enabled && l.reasoning == nil {
		l.reasoning = newReasoningRenderer(l.w, l.useANSI)
	}
}

func (l *Logger) SetShowProgress(enabled bool) {
	if l == nil {
		return
	}
	l.showProgress = enabled
}

func (l *Logger) Enabled() bool {
	return l != nil && l.enabled
}

func (l *Logger) ShowReasoning() bool {
	return l != nil && l.showReasoning
}

// OpenReasoningSection opens a new labeled reasoning section. Returns nil when
// neither --show-reasoning nor --show-progress is enabled.
// All ReasoningSection methods are nil-safe.
func (l *Logger) OpenReasoningSection(info ProgressInfo) *ReasoningSection {
	if l == nil || (!l.showReasoning && !l.showProgress) {
		return nil
	}
	sec := l.NewReasoningTracker(info)
	if l.showReasoning && l.reasoning != nil {
		sec.id = l.reasoning.Begin(info.Label())
		sec.r = l.reasoning
	}
	return sec
}

// NewReasoningTracker returns a progress-only reasoning section. It tracks
// the identity, elapsed time, and call numbers without opening a renderer
// section.
func (l *Logger) NewReasoningTracker(info ProgressInfo) *ReasoningSection {
	if l == nil || (!l.showReasoning && !l.showProgress) {
		return nil
	}
	sec := &ReasoningSection{
		logger:    l,
		info:      info,
		startTime: time.Now(),
	}
	return sec
}

func (l *Logger) PrintError(err error) {
	if l == nil || err == nil {
		return
	}
	if l.useANSI {
		l.writeRaw(progressStyle(progressColorErrorRed, "ERROR") + progressGrey(":") + " " + progressLight(err.Error()) + "\n")
		return
	}
	l.writeRaw(fmt.Sprintf("ERROR: %s\n", err))
}

func (l *Logger) writeRaw(text string) {
	if l == nil {
		return
	}
	if l.reasoning != nil {
		l.reasoning.WriteOutside(text)
		return
	}
	_, _ = io.WriteString(l.w, text)
}

type renderedJSONLine struct {
	text       string
	stringOnly bool
	// prefixLen is the length of the structural prefix (indentation plus
	// `"key": `) on the first line of a multiline string. Renderers colorize
	// text[:prefixLen] as JSON structure and text[prefixLen:] as string
	// content; 0 means the whole line is string content.
	prefixLen int
}

func normalizeEmbeddedJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeEmbeddedJSON(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = normalizeEmbeddedJSON(item)
		}
		return out
	case string:
		trimmed := strings.TrimSpace(typed)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			var embedded any
			if err := json.Unmarshal([]byte(trimmed), &embedded); err == nil {
				return normalizeEmbeddedJSON(embedded)
			}
		}
		return strings.ReplaceAll(typed, "\r\n", "\n")
	default:
		return typed
	}
}

func renderJSONLines(value any, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	switch typed := value.(type) {
	case map[string]any:
		return renderJSONObjectLines(typed, indent, prefix, trailingComma)
	case []any:
		return renderJSONArrayLines(typed, indent, prefix, trailingComma)
	case string:
		return renderJSONStringLines(typed, prefix, trailingComma)
	default:
		return []renderedJSONLine{{
			text: prefix + marshalJSONScalar(typed) + trailingCommaSuffix(trailingComma),
		}}
	}
}

func renderJSONObjectLines(value map[string]any, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	if len(value) == 0 {
		return []renderedJSONLine{{
			text: prefix + "{}" + trailingCommaSuffix(trailingComma),
		}}
	}

	lines := []renderedJSONLine{{text: prefix + "{"}}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		childPrefix := strings.Repeat("  ", indent+1) + marshalJSONString(key) + ": "
		lines = append(lines, renderJSONLines(value[key], indent+1, childPrefix, i < len(keys)-1)...)
	}
	lines = append(lines, renderedJSONLine{
		text: strings.Repeat("  ", indent) + "}" + trailingCommaSuffix(trailingComma),
	})
	return lines
}

func renderJSONArrayLines(value []any, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	if len(value) == 0 {
		return []renderedJSONLine{{
			text: prefix + "[]" + trailingCommaSuffix(trailingComma),
		}}
	}

	lines := []renderedJSONLine{{text: prefix + "["}}
	for i, item := range value {
		childPrefix := strings.Repeat("  ", indent+1)
		lines = append(lines, renderJSONLines(item, indent+1, childPrefix, i < len(value)-1)...)
	}
	lines = append(lines, renderedJSONLine{
		text: strings.Repeat("  ", indent) + "]" + trailingCommaSuffix(trailingComma),
	})
	return lines
}

func renderJSONStringLines(value, prefix string, trailingComma bool) []renderedJSONLine {
	if !strings.Contains(value, "\n") {
		return []renderedJSONLine{{
			text: prefix + marshalJSONString(value) + trailingCommaSuffix(trailingComma),
		}}
	}

	parts := strings.Split(value, "\n")
	continuation := strings.Repeat(" ", len(prefix)+1)
	lines := make([]renderedJSONLine, 0, len(parts))
	lines = append(lines, renderedJSONLine{
		text:       prefix + `"` + escapeJSONStringFragment(parts[0]),
		stringOnly: true,
		prefixLen:  len(prefix),
	})
	for _, part := range parts[1 : len(parts)-1] {
		lines = append(lines, renderedJSONLine{
			text:       continuation + escapeJSONStringFragment(part),
			stringOnly: true,
		})
	}
	lines = append(lines, renderedJSONLine{
		text:       continuation + escapeJSONStringFragment(parts[len(parts)-1]) + `"` + trailingCommaSuffix(trailingComma),
		stringOnly: true,
	})
	return lines
}

func marshalJSONString(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}

func escapeJSONStringFragment(value string) string {
	marshaled := marshalJSONString(value)
	unquoted, err := strconv.Unquote(marshaled)
	if err != nil {
		return textsan.StripControl(strings.TrimSuffix(strings.TrimPrefix(marshaled, `"`), `"`))
	}
	// strconv.Unquote decodes  back into a raw ESC byte; untrusted
	// multi-line LLM/PR text reaches this display fragment, so strip control
	// characters (newlines/tabs are kept for layout) before it is written raw.
	return textsan.StripControl(unquoted)
}

func marshalJSONScalar(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(data)
}

func trailingCommaSuffix(enabled bool) string {
	if enabled {
		return ","
	}
	return ""
}

func colorizeJSON(line string) string {
	var out strings.Builder
	inString := false
	escaped := false
	token := &bytes.Buffer{}

	flushToken := func() {
		if token.Len() == 0 {
			return
		}
		text := token.String()
		switch text {
		case "true", "false":
			out.WriteString(progressStyle(progressColorBoolGreen, text))
		case "null":
			out.WriteString(progressStyle(progressColorDarkGrey, text))
		default:
			out.WriteString(progressStyle(progressColorNumberGreen, text))
		}
		token.Reset()
	}

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inString {
			token.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
				j := i + 1
				for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
					j++
				}
				color := progressColorStringGreen
				if j < len(line) && line[j] == ':' {
					color = progressColorKeyTurquoise
				}
				out.WriteString(progressStyle(color, token.String()))
				token.Reset()
			}
			continue
		}
		switch {
		case ch == '"':
			flushToken()
			inString = true
			token.WriteByte(ch)
		case (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || (ch >= 'a' && ch <= 'z'):
			token.WriteByte(ch)
		default:
			flushToken()
			out.WriteByte(ch)
		}
	}
	flushToken()
	return out.String()
}
