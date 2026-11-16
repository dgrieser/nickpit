package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/mittwald/cli-kit/pick"
	"github.com/mittwald/cli-kit/theme"
)

// paintedKinds reads back what a list paints inside its first n columns by
// handing each column probe cells: a commit message, a ref and a ref with a
// note, the three things a NickPit picker paints inside a cell.
func paintedKinds(opts pick.Options, n int) []cellKind {
	kinds := make([]cellKind, n)
	if opts.PaintRowCell == nil {
		return kinds
	}
	painted := func(column int, text string, want theme.Style) bool {
		out, ok := opts.PaintRowCell(0, column, text, styleText, false)
		return ok && strings.Contains(out, want.Sequence())
	}
	for column := range kinds {
		switch {
		case painted(column, "feat: x", styleMsgType):
			kinds[column] = cellMessage
		case painted(column, "a/b (note)", styleNote):
			kinds[column] = cellRefNote
		case painted(column, "a/b", styleSeparator):
			kinds[column] = cellRef
		}
	}
	return kinds
}

// A conventional-commit prefix is taken apart the way the git-color helper
// renders a git log: the type and what it touched step out of the message.
func TestMessageSegments(t *testing.T) {
	got := messageSegments("feat(review): cluster merge", styleText)
	want := []segment{
		{"feat", styleMsgType},
		{"(", styleMsgPunct},
		{"review", styleMsgScope},
		{")", styleMsgPunct},
		{":", styleMsgPunct},
		{" ", ""},
		{"cluster merge", styleText},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %+v, want %+v", got, want)
	}
	// A prefix without a scope, and a breaking change.
	if got := messageSegments("fix!: drop the flag", styleText); !slices.Equal(got, []segment{
		{"fix", styleMsgType},
		{"!", styleMsgBreaking},
		{":", styleMsgPunct},
		{" ", ""},
		{"drop the flag", styleText},
	}) {
		t.Fatalf("segments = %+v", got)
	}
	// A type nobody uses stays plain text, or any "name(args):" in a title
	// would be painted as a commit prefix.
	for _, plain := range []string{
		"Draft: add backup-kontrolle",
		"read_file(path): returns lines",
		"feature(review): not a conventional type",
		"feat(review) cluster merge",
		// Cut short by a narrow column before its colon.
		"feat(revi…",
	} {
		if got := messageSegments(plain, styleText); !slices.Equal(got, []segment{{plain, styleText}}) {
			t.Fatalf("segments for %q = %+v, want one plain segment", plain, got)
		}
	}
}

// A ref reads as parts: the separators fade the way a progress line renders
// "repo @ head → base".
func TestRefSegments(t *testing.T) {
	got := refSegments("origin/feat/pick", styleDetail)
	want := []segment{
		{"origin", styleDetail},
		{"/", styleMsgPunct},
		{"feat", styleDetail},
		{"/", styleMsgPunct},
		{"pick", styleDetail},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %+v, want %+v", got, want)
	}
	// Colons separate too (a refspec, a submodule path).
	if got := refSegments("refs/heads/main:main", styleDetail); len(got) != 7 {
		t.Fatalf("segments = %+v, want every separator split out", got)
	}
	// Nothing to split: one segment.
	if got := refSegments("main", styleDetail); !slices.Equal(got, []segment{{"main", styleDetail}}) {
		t.Fatalf("segments = %+v", got)
	}
}

// A trailing parenthetical recedes; one inside the value is part of the ref.
func TestRefNoteSegments(t *testing.T) {
	got := refNoteSegments("feat/x (uncommitted)", styleIdentifier)
	want := []segment{
		{"feat", styleIdentifier},
		{"/", styleMsgPunct},
		{"x", styleIdentifier},
		{" (uncommitted)", styleNote},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %+v, want %+v", got, want)
	}
	for _, part := range refNoteSegments("feat/(x) more", styleIdentifier) {
		if part.style == styleNote {
			t.Fatalf("segments = %+v, want no note painted inside the value", part)
		}
	}
}

// paintCells paints the columns it was told about, in the cell's own base
// colour, and leaves every other column to the list.
func TestPaintCells(t *testing.T) {
	paint := paintCells(cellPlain, cellRef, cellMessage, cellRefNote)
	author := theme.Hashed("alice", nil)
	if _, ok := paint(0, 0, "★", styleMark, true); ok {
		t.Fatal("a plain column must be left to the list")
	}
	if _, ok := paint(0, 9, "x", "", false); ok {
		t.Fatal("a column past the kinds must be left to the list")
	}
	ref, ok := paint(0, 1, "feat/x", author, false)
	if want := author.Paint("feat") + styleSeparator.Paint("/") + author.Paint("x"); !ok || ref != want {
		t.Fatalf("ref = %q, want %q", ref, want)
	}
	msg, _ := paint(0, 2, "fix(llm): retry", styleText, false)
	if want := styleMsgType.Paint("fix") + styleSeparator.Paint("(") + styleMsgScope.Paint("llm") +
		styleSeparator.Paint(")") + styleSeparator.Paint(":") + " " + styleText.Paint("retry"); msg != want {
		t.Fatalf("message = %q, want %q", msg, want)
	}
	note, _ := paint(0, 3, "main (uncommitted)", styleDefaultRef, false)
	if want := styleDefaultRef.Paint("main") + styleNote.Paint(" (uncommitted)"); note != want {
		t.Fatalf("ref note = %q, want %q", note, want)
	}
	// The terminal's own tone carries no code: only the separators are painted.
	if plain, _ := paint(0, 1, "feat/x", styleTerminalFg, false); plain != "feat"+styleSeparator.Paint("/")+"x" {
		t.Fatalf("ref in the terminal's tone = %q", plain)
	}
}

// paintRef is what a header value and a confirmation use.
func TestPaintRef(t *testing.T) {
	if got, want := paintRef("origin/main", styleBaseRef), styleBaseRef.Paint("origin")+
		styleSeparator.Paint("/")+styleBaseRef.Paint("main"); got != want {
		t.Fatalf("paintRef = %q, want %q", got, want)
	}
	if got := paintRef("a/b", ""); got != "a"+styleSeparator.Paint("/")+"b" {
		t.Fatalf("paintRef without a base = %q", got)
	}
}

// Every list is drawn with NickPit's look, whichever prompt opens it, and the
// seams see the options the terminal would get.
func TestPickerLookIsShared(t *testing.T) {
	var seen []pick.Options
	a := &app{
		selectFn: func(opts pick.Options) (int, error) {
			seen = append(seen, opts)
			return 0, nil
		},
		selectViewFn: func(opts pick.Options) (int, int, error) {
			seen = append(seen, opts)
			return 0, 0, nil
		},
	}
	items := []pick.Item{{Cells: []string{"x"}}}
	if _, err := a.selectOne(pick.Options{Items: items}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.selectView(pick.Options{Views: []pick.View{{Items: items}}}); err != nil {
		t.Fatal(err)
	}
	for _, opts := range seen {
		if opts.CursorRow != theme.BgRGB(69, 56, 86) || opts.CursorMark != theme.Join(theme.Bold, theme.Fg256(255)) ||
			!opts.Compact || opts.ActiveView != theme.Join(theme.Bold, theme.Fg256(189)) {
			t.Fatalf("look = %+v", opts)
		}
		if opts.Keys != (pick.KeyLabels{Cancel: "Esc abort", View: "Tab/←/→ scope", ViewShort: "Tab scope", ViewLast: true}) {
			t.Fatalf("keys = %+v", opts.Keys)
		}
		// A nested prompt keeps cli-kit's "Esc back".
		if opts.Keys.Back != "" {
			t.Fatalf("back label = %q, want cli-kit's default", opts.Keys.Back)
		}
		if opts.PaintDetail == nil || opts.PaintDetail("a/b", styleDetail) != paintRef("a/b", styleDetail) {
			t.Fatal("a detail must be painted as a ref")
		}
	}
	// A prompt's own detail painter is kept.
	own := func(text string, _ theme.Style) string { return text }
	if opts := withPickerLook(pick.Options{PaintDetail: own}); opts.PaintDetail("a/b", "") != "a/b" {
		t.Fatal("withPickerLook replaced the prompt's own detail painter")
	}
}
