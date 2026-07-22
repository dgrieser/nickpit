package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
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
	live          *LiveRenderer
}

// ReasoningSection is a handle to one labeled block in the ReasoningRenderer.
// All methods are nil-safe so callers can defer End() without nil checks.
type ReasoningSection struct {
	r         *ReasoningRenderer
	id        SectionID
	logger    *Logger
	info      ProgressInfo // identity snapshot taken when the section opens
	startTime time.Time
	mu        sync.Mutex // guards ended against Append racing End
	ended     bool
	callNum   int // incremented by IncrCallNum on each LLM request
}

func (s *ReasoningSection) Append(delta string) {
	if s == nil || s.r == nil {
		return
	}
	// Once the section ended the renderer may have recycled its section slice,
	// so s.id can refer to a newer section. Holding mu across the renderer
	// call keeps End's "mark ended, then close the renderer section" atomic
	// with respect to appends: after End returns, no Append can reach the
	// renderer with the stale id.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
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
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	if s.r != nil {
		s.r.End(s.id)
	}
	s.mu.Unlock()
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
	if l == nil || (!l.showReasoning && !l.showProgress && l.live == nil) {
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
	// Errors can embed upstream/LLM response text; strip control characters so
	// they cannot smuggle escape sequences into the terminal.
	msg := textsan.StripControl(err.Error())
	if l.useANSI {
		l.writeRaw(progressStyle(progressColorErrorRed, "ERROR") + progressGrey(":") + " " + progressLight(msg) + "\n")
		return
	}
	l.writeRaw(fmt.Sprintf("ERROR: %s\n", msg))
}

// PrintWarning surfaces a non-fatal problem regardless of verbosity — unlike
// Verbosef, which a default (non --verbose) run drops. Like PrintError, the
// message may embed upstream text, so control characters are stripped.
func (l *Logger) PrintWarning(msg string) {
	if l == nil || msg == "" {
		return
	}
	msg = textsan.StripControl(msg)
	if l.useANSI {
		l.writeRaw(progressStyle(progressColorWarnYellow, "WARNING") + progressGrey(":") + " " + progressLight(msg) + "\n")
		return
	}
	l.writeRaw(fmt.Sprintf("WARNING: %s\n", msg))
}

func (l *Logger) writeRaw(text string) {
	if l == nil {
		return
	}
	if l.reasoning != nil {
		l.reasoning.WriteOutside(text)
		return
	}
	if l.live != nil {
		l.live.WriteOutside(text)
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

type orderedJSONKind int

const (
	orderedJSONScalar orderedJSONKind = iota
	orderedJSONString
	orderedJSONObject
	orderedJSONArray
)

type orderedJSONMember struct {
	key   string
	value orderedJSONValue
}

type orderedJSONValue struct {
	kind   orderedJSONKind
	scalar any
	str    string
	object []orderedJSONMember
	array  []orderedJSONValue
}

func decodeOrderedJSON(data []byte) (orderedJSONValue, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeOrderedJSONValue(decoder)
	if err != nil {
		return orderedJSONValue{}, err
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return orderedJSONValue{}, err
		}
		return orderedJSONValue{}, fmt.Errorf("unexpected trailing JSON token %v", token)
	}
	return value, nil
}

func decodeOrderedJSONValue(decoder *json.Decoder) (orderedJSONValue, error) {
	token, err := decoder.Token()
	if err != nil {
		return orderedJSONValue{}, err
	}
	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			var members []orderedJSONMember
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return orderedJSONValue{}, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return orderedJSONValue{}, fmt.Errorf("object key token %v is not a string", keyToken)
				}
				value, err := decodeOrderedJSONValue(decoder)
				if err != nil {
					return orderedJSONValue{}, err
				}
				members = append(members, orderedJSONMember{key: key, value: value})
			}
			end, err := decoder.Token()
			if err != nil {
				return orderedJSONValue{}, err
			}
			if end != json.Delim('}') {
				return orderedJSONValue{}, fmt.Errorf("object ended with %v", end)
			}
			return orderedJSONValue{kind: orderedJSONObject, object: members}, nil
		case '[':
			var items []orderedJSONValue
			for decoder.More() {
				value, err := decodeOrderedJSONValue(decoder)
				if err != nil {
					return orderedJSONValue{}, err
				}
				items = append(items, value)
			}
			end, err := decoder.Token()
			if err != nil {
				return orderedJSONValue{}, err
			}
			if end != json.Delim(']') {
				return orderedJSONValue{}, fmt.Errorf("array ended with %v", end)
			}
			return orderedJSONValue{kind: orderedJSONArray, array: items}, nil
		default:
			return orderedJSONValue{}, fmt.Errorf("unexpected JSON delimiter %q", typed)
		}
	case string:
		return orderedJSONValue{kind: orderedJSONString, str: typed}, nil
	default:
		return orderedJSONValue{kind: orderedJSONScalar, scalar: typed}, nil
	}
}

func normalizeEmbeddedJSON(value orderedJSONValue) orderedJSONValue {
	switch value.kind {
	case orderedJSONObject:
		for i := range value.object {
			value.object[i].value = normalizeEmbeddedJSON(value.object[i].value)
		}
		return value
	case orderedJSONArray:
		for i := range value.array {
			value.array[i] = normalizeEmbeddedJSON(value.array[i])
		}
		return value
	case orderedJSONString:
		trimmed := strings.TrimSpace(value.str)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			if embedded, err := decodeOrderedJSON([]byte(trimmed)); err == nil {
				return normalizeEmbeddedJSON(embedded)
			}
		}
		value.str = strings.ReplaceAll(value.str, "\r\n", "\n")
		return value
	default:
		return value
	}
}

func renderJSONLines(value orderedJSONValue, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	switch value.kind {
	case orderedJSONObject:
		return renderJSONObjectLines(value.object, indent, prefix, trailingComma)
	case orderedJSONArray:
		return renderJSONArrayLines(value.array, indent, prefix, trailingComma)
	case orderedJSONString:
		return renderJSONStringLines(value.str, prefix, trailingComma)
	default:
		return []renderedJSONLine{{
			text: prefix + marshalJSONScalar(value.scalar) + trailingCommaSuffix(trailingComma),
		}}
	}
}

func renderJSONObjectLines(value []orderedJSONMember, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	if len(value) == 0 {
		return []renderedJSONLine{{
			text: prefix + "{}" + trailingCommaSuffix(trailingComma),
		}}
	}

	lines := []renderedJSONLine{{text: prefix + "{"}}
	for i, member := range value {
		childPrefix := strings.Repeat("  ", indent+1) + marshalJSONString(member.key) + ": "
		lines = append(lines, renderJSONLines(member.value, indent+1, childPrefix, i < len(value)-1)...)
	}
	lines = append(lines, renderedJSONLine{
		text: strings.Repeat("  ", indent) + "}" + trailingCommaSuffix(trailingComma),
	})
	return lines
}

func renderJSONArrayLines(value []orderedJSONValue, indent int, prefix string, trailingComma bool) []renderedJSONLine {
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
