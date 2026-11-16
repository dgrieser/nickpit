package main

import (
	"regexp"
	"strings"

	"github.com/mittwald/cli-kit/theme"
)

// What NickPit paints inside a picker cell: a conventional-commit prefix taken
// apart, the separators of a ref faded, and a trailing note receding — the
// rules the git-color helper applies to a git log, in NickPit's palette.
// cli-kit's list hands each cell to paintCells (Options.PaintRowCell) and a
// header value to paintRef (Options.PaintDetail, Field.Paint).

// cellKind says what a column holds, so its cells can be painted inside and
// not only around.
type cellKind int

const (
	// cellPlain is a cell painted in one colour, by the list itself.
	cellPlain cellKind = iota
	// cellMessage is a commit-style message: a conventional-commit prefix in
	// it is taken apart.
	cellMessage
	// cellRef is a branch, ref or path: its "/" and ":" separators fade back,
	// exactly as a progress line renders "repo @ head → base".
	cellRef
	// cellRefNote is a ref carrying a trailing parenthetical — "feat/x
	// (uncommitted)": the ref is painted as one and the note recedes, so the
	// qualifier does not read as part of the name.
	cellRefNote
)

// paintCells is the Options.PaintRowCell of a list whose columns hold what
// kinds says, indexed like Item.Cells; unlisted columns are cellPlain and
// left to the list. base is the style the list would draw the whole cell in —
// the row's own colour where it has one, such as an author's — so the parts a
// rule does not single out keep it.
func paintCells(kinds ...cellKind) func(row, column int, text string, base theme.Style, selected bool) (string, bool) {
	return func(_, column int, text string, base theme.Style, _ bool) (string, bool) {
		if column >= len(kinds) {
			return "", false
		}
		switch kinds[column] {
		case cellMessage:
			return paintSegments(messageSegments(text, base)), true
		case cellRef:
			return paintSegments(refSegments(text, base)), true
		case cellRefNote:
			return paintSegments(refNoteSegments(text, base)), true
		}
		return "", false
	}
}

// paintRef renders a ref with its separators faded: the parts in base, the
// "/" and ":" in styleSeparator. It is the painter of every header value and
// of the confirmation after a pick, so one rule covers the screen.
func paintRef(text string, base theme.Style) string {
	return paintSegments(refSegments(text, base))
}

// segment is one styled piece of a cell.
type segment struct {
	text  string
	style theme.Style
}

// paintSegments joins the pieces, each in its own colour. Blank text and the
// terminal's own foreground carry no code: the list pads and highlights
// around what is painted here.
func paintSegments(segments []segment) string {
	var b strings.Builder
	for _, part := range segments {
		if part.style == styleTerminalFg || strings.TrimSpace(part.text) == "" {
			b.WriteString(part.text)
			continue
		}
		b.WriteString(part.style.Paint(part.text))
	}
	return b.String()
}

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
	styleMsgType theme.Style = "38;5;120"
	// styleMsgScope is what the commit touched (progressColorKeyTurquoise, the
	// colour a progress line gives the key of a key=value).
	styleMsgScope theme.Style = "38;5;116"
	// styleMsgPunct fades the parentheses and the colon back.
	styleMsgPunct = styleSeparator
	// styleMsgBreaking marks the "!" of a breaking change: git-color paints it
	// like the type, NickPit gives it the error red, because a picker row is
	// the last place to overlook one (progressColorErrorRed).
	styleMsgBreaking theme.Style = "38;5;203"
)

// reTrailingNote matches the " (…)" a ref may end on. It is anchored at the
// end, so a parenthesis inside the value itself is left alone — and it allows
// trailing padding, so a cell padded to its column still has its note found.
var reTrailingNote = regexp.MustCompile(`\s+\([^()]*\)\s*$`)

// refSegments fades the separators of a ref or path back, so the parts of
// "origin/feat/tree-sitter" read as parts. Same rule as
// logging.styleSeparatedText, which paints "head → base" in the progress lines
// this list sits next to.
func refSegments(text string, base theme.Style) []segment {
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

// refNoteSegments paints a ref that ends on a parenthetical: the ref by its own
// rule, the note in light grey. A cell the column had to cut short loses its
// closing parenthesis and is then painted as a plain ref, which is what a
// half-visible note should look like.
func refNoteSegments(text string, base theme.Style) []segment {
	loc := reTrailingNote.FindStringIndex(text)
	if loc == nil {
		return refSegments(text, base)
	}
	return append(refSegments(text[:loc[0]], base), segment{text[loc[0]:], styleNote})
}

// messageSegments splits a message into its conventional-commit prefix and the
// text after it, so the type and the scope carry their own colour while the
// rest keeps base. A message without a known prefix — a plain title, or one cut
// short by a narrow column before its colon — stays a single segment.
func messageSegments(text string, base theme.Style) []segment {
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
	add := func(part string, style theme.Style) {
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
