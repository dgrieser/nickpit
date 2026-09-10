package pick

import (
	"regexp"
	"strings"
)

// conventionalTypes are the commit types a message prefix may carry — the same
// set the git-color helper recognizes, so a subject reads the same in a picker
// as in a coloured git log. The type has to be one of these: without that
// check, any "name(args):" in a title would pass for a commit prefix.
var conventionalTypes = []string{
	"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert",
	"style", "test", "wip",
}

// reConventional matches a conventional-commit prefix at the start of a
// message: the type, an optional "(scope)", an optional "!" for a breaking
// change, and the colon that ends it. The trailing whitespace is captured
// rather than looked ahead at, because Go's regexp has no lookahead, and it is
// re-emitted unpainted.
var reConventional = regexp.MustCompile(
	`^(\s*)(` + strings.Join(conventionalTypes, "|") + `)(?:(\()([^()]*)(\)))?(!?)(:)(\s|$)`)

// Colours of a commit prefix. They mirror the git-color roles (msgtype,
// msgscope, msgpunct) in NickPit's own palette, so the type and what it touched
// step out of the message without a second font.
const (
	// styleMsgType is the commit type (progressColorStringGreen).
	styleMsgType = "38;5;120"
	// styleMsgScope is what the commit touched (progressColorKeyTurquoise, the
	// colour a progress line gives the key of a key=value).
	styleMsgScope = "38;5;116"
	// styleMsgPunct fades the parentheses and the colon back.
	styleMsgPunct = StyleSeparator
	// styleMsgBreaking marks the "!" of a breaking change: git-color paints it
	// like the type, NickPit gives it the error red, because a picker row is
	// the last place to overlook one (progressColorErrorRed).
	styleMsgBreaking = "38;5;203"
)

// StyleSeparator fades the punctuation that holds a value together back: the
// parentheses and colon of a commit prefix, and the "/" and ":" inside a ref
// (progressColorGrey, the punctuation grey of a progress line). Exported
// because the same ref is painted outside a list too — the confirmation line
// after a pick.
const StyleSeparator = "38;5;244"

// PaintRef renders a ref with its separators faded, the way a picker row and a
// title line show one: the parts in base, the "/" and ":" in StyleSeparator.
// Callers use it wherever a ref appears outside the list, so one rule covers
// the screen.
func PaintRef(text, base string) string {
	var b strings.Builder
	for _, part := range refSegments(text, base) {
		if part.style == "" {
			b.WriteString(part.text)
			continue
		}
		b.WriteString("\x1b[" + part.style + "m" + part.text + attrReset)
	}
	return b.String()
}

// ColumnKind says what a column holds, so the picker can paint inside a cell
// and not only around it. Callers declare it per column in
// Options.ColumnKinds; the zero value is a plain cell.
type ColumnKind int

const (
	// KindPlain is a cell painted in one colour.
	KindPlain ColumnKind = iota
	// KindMessage is a commit-style message: a conventional-commit prefix in it
	// is taken apart.
	KindMessage
	// KindRef is a branch, ref or path: its "/" and ":" separators fade back,
	// exactly as a progress line renders "repo @ head → base".
	KindRef
)

// cellSegments paints one cell according to what the column holds.
func cellSegments(kind ColumnKind, text, base string) []segment {
	switch kind {
	case KindMessage:
		return messageSegments(text, base)
	case KindRef:
		return refSegments(text, base)
	default:
		return []segment{{text, base}}
	}
}

// refSegments fades the separators of a ref or path back, so the parts of
// "origin/feat/tree-sitter" read as parts. Same rule as
// logging.styleSeparatedText, which paints "head → base" in the progress lines
// this list sits next to.
func refSegments(text, base string) []segment {
	var segments []segment
	part := 0
	for i, r := range text {
		if r != '/' && r != ':' {
			continue
		}
		if i > part {
			segments = append(segments, segment{text[part:i], base})
		}
		segments = append(segments, segment{string(r), styleMsgPunct})
		part = i + len(string(r))
	}
	if part == 0 {
		// No separator at all: one segment, and no allocation on the way.
		return []segment{{text, base}}
	}
	if part < len(text) {
		segments = append(segments, segment{text[part:], base})
	}
	return segments
}

// messageSegments splits a message into its conventional-commit prefix and the
// text after it, so the type and the scope carry their own colour while the
// rest keeps base. A message without a known prefix — a plain title, or one cut
// short by a narrow column before its colon — stays a single segment.
func messageSegments(text, base string) []segment {
	match := reConventional.FindStringSubmatchIndex(text)
	if match == nil {
		return []segment{{text, base}}
	}
	group := func(i int) string {
		if match[2*i] < 0 {
			return ""
		}
		return text[match[2*i]:match[2*i+1]]
	}
	segments := make([]segment, 0, 8)
	add := func(part, style string) {
		if part != "" {
			segments = append(segments, segment{part, style})
		}
	}
	add(group(1), "")               // leading whitespace, if any
	add(group(2), styleMsgType)     // feat
	add(group(3), styleMsgPunct)    // (
	add(group(4), styleMsgScope)    // review
	add(group(5), styleMsgPunct)    // )
	add(group(6), styleMsgBreaking) // !
	add(group(7), styleMsgPunct)    // :
	add(group(8), "")               // the space after the colon
	add(text[match[1]:], base)      // what changed
	return segments
}
