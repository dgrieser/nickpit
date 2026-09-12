package pick

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

func testList(t *testing.T, opts Options, height int) *list {
	t.Helper()
	if opts.Items == nil {
		opts.Items = []Item{
			{Cells: []string{"★", "!142", "feat(review): cluster merge", "alice", "2d"}},
			{Cells: []string{"", "!139", "fix(llm): retry on 429", "bob", "4d"}},
			{Cells: []string{"", "!131", "chore: bump deps", "carol", "1w"}},
		}
	}
	return newList(opts, height, false)
}

func TestListMovesAndSelects(t *testing.T) {
	l := testList(t, Options{Title: "Open merge requests"}, 24)
	if got := l.selected(); got != 0 {
		t.Fatalf("initial selection = %d, want 0", got)
	}
	l.apply(key{kind: keyDown})
	l.apply(key{kind: keyDown})
	if got := l.selected(); got != 2 {
		t.Fatalf("selection after two downs = %d, want 2", got)
	}
	// The cursor stops at the ends instead of wrapping: a wrap turns a held key
	// into a silent jump across the whole list.
	l.apply(key{kind: keyDown})
	if got := l.selected(); got != 2 {
		t.Fatalf("selection past the end = %d, want 2", got)
	}
	l.apply(key{kind: keyHome})
	if got := l.selected(); got != 0 {
		t.Fatalf("selection after Home = %d, want 0", got)
	}
	l.apply(key{kind: keyEnd})
	if got := l.selected(); got != 2 {
		t.Fatalf("selection after End = %d, want 2", got)
	}
	if action := l.apply(key{kind: keyEnter}); action != actionSelect {
		t.Fatalf("Enter action = %v, want select", action)
	}
	if action := l.apply(key{kind: keyAbort}); action != actionAbort {
		t.Fatalf("abort action = %v, want abort", action)
	}
}

func TestListInitialSelection(t *testing.T) {
	l := testList(t, Options{Initial: 2}, 24)
	if got := l.selected(); got != 2 {
		t.Fatalf("selection = %d, want the preselected 2", got)
	}
	// An out-of-range preselection is a caller bug that must not crash or hide
	// the list; it starts at the top instead.
	l = testList(t, Options{Initial: 99}, 24)
	if got := l.selected(); got != 0 {
		t.Fatalf("selection for out-of-range initial = %d, want 0", got)
	}
}

func TestListFilterNarrowsAndKeepsSelection(t *testing.T) {
	l := testList(t, Options{}, 24)
	l.apply(key{kind: keyDown}) // bob's MR
	for _, r := range "bob" {
		l.apply(key{kind: keyRune, rune: r})
	}
	if len(l.matches) != 1 {
		t.Fatalf("matches = %d, want 1: %v", len(l.matches), l.matches)
	}
	if got := l.selected(); got != 1 {
		t.Fatalf("selection after filtering = %d, want the still-matching 1", got)
	}
	// Every term must match, so a second word narrows instead of widening.
	l.setFilter([]rune("bob retry"))
	if len(l.matches) != 1 {
		t.Fatalf("matches for two terms = %d, want 1", len(l.matches))
	}
	l.setFilter([]rune("bob feat"))
	if len(l.matches) != 0 {
		t.Fatalf("matches for contradicting terms = %d, want 0", len(l.matches))
	}
	// Enter on an empty match set must not select anything.
	if action := l.apply(key{kind: keyEnter}); action != actionNone {
		t.Fatalf("Enter with no match = %v, want none", action)
	}
	if got := l.selected(); got != -1 {
		t.Fatalf("selection with no match = %d, want -1", got)
	}
	if action := l.apply(key{kind: keyClearFilter}); action != actionNone || len(l.matches) != 3 {
		t.Fatalf("Ctrl-U left %d matches (action %v), want all 3 back", len(l.matches), action)
	}
}

func TestListBackspaceOnlyDeletesFilterText(t *testing.T) {
	l := testList(t, Options{}, 24)
	for _, r := range "ab" {
		l.apply(key{kind: keyRune, rune: r})
	}
	if action := l.apply(key{kind: keyBackspace}); action != actionNone {
		t.Fatalf("backspace inside the filter = %v, want none", action)
	}
	l.apply(key{kind: keyBackspace})
	if len(l.filter) != 0 {
		t.Fatalf("filter = %q, want it emptied", string(l.filter))
	}
	// One press too many while clearing a filter must not throw the list away:
	// only Esc and Ctrl-C/Ctrl-D leave it.
	if action := l.apply(key{kind: keyBackspace}); action != actionNone {
		t.Fatalf("backspace on an empty filter = %v, want nothing", action)
	}
	if action := l.apply(key{kind: keyAbort}); action != actionAbort {
		t.Fatalf("abort key = %v, want abort", action)
	}
}

func TestListScrollsWithCursor(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = Item{Cells: []string{string(rune('a' + i))}}
	}
	l := newList(Options{Items: items, MaxVisible: 5}, 24, false)
	for range 7 {
		l.apply(key{kind: keyDown})
	}
	lines := l.render(40)
	// Title, five rows, position line, hint line.
	if len(lines) != 8 {
		t.Fatalf("rendered %d lines, want 8: %q", len(lines), lines)
	}
	if !strings.Contains(lines[5], cursorMarker+"h") {
		t.Fatalf("cursor row = %q, want the 8th item on the last visible row", lines[5])
	}
	if !strings.Contains(lines[6], "8 of 20") {
		t.Fatalf("position line = %q, want it to report 8 of 20", lines[6])
	}
	// Paging back up must bring the window along.
	l.apply(key{kind: keyPageUp})
	lines = l.render(40)
	if !strings.Contains(lines[1], cursorMarker+"c") {
		t.Fatalf("row after PgUp = %q, want the cursor on the third item", lines[1])
	}
}

func TestListRenderTruncatesToWidth(t *testing.T) {
	long := strings.Repeat("x", 200)
	l := newList(Options{Title: "t", Items: []Item{{Cells: []string{"!1", long, "alice", "2d"}}}}, 24, false)
	for _, line := range l.render(40) {
		if displayWidth(line) > 40 {
			t.Fatalf("line wider than the terminal (%d): %q", displayWidth(line), line)
		}
	}
}

func TestListShrinksWithTheTerminal(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = Item{Cells: []string{string(rune('a' + i))}}
	}
	l := newList(Options{Items: items}, 24, false)
	before := len(l.render(40))
	l.resize(0, 10)
	if after := len(l.render(40)); after >= before {
		t.Fatalf("rendered %d lines after shrinking to 10 rows, want fewer than %d", after, before)
	}
	// However short the terminal claims to be, some rows stay visible.
	l.resize(0, 1)
	if got := len(l.render(40)); got < minVisible {
		t.Fatalf("rendered %d lines in a 1-row terminal, want at least %d rows", got, minVisible)
	}
}

func TestListNoMatchLine(t *testing.T) {
	l := testList(t, Options{}, 24)
	l.setFilter([]rune("nothing matches this"))
	lines := l.render(40)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "no match") {
		t.Fatalf("render = %q, want it to say there is no match", joined)
	}
	if !strings.Contains(joined, "0 of 3") {
		t.Fatalf("render = %q, want the position line to report 0 of 3", joined)
	}
}

func TestItemFilterTextFallsBackToCells(t *testing.T) {
	item := Item{Cells: []string{"!7", "title"}}
	if got := item.filterText(); got != "!7 title" {
		t.Fatalf("filterText = %q", got)
	}
	item.Match = "explicit"
	if got := item.filterText(); got != "explicit" {
		t.Fatalf("filterText with Match = %q", got)
	}
}

func TestRendererRedrawsInPlace(t *testing.T) {
	var out strings.Builder
	r := &renderer{w: &out}
	if err := r.draw([]string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	first := out.String()
	// Nothing was drawn before, so the first block must not move the cursor up
	// over output the command already printed.
	if !strings.HasPrefix(first, "\x1b[2K") {
		t.Fatalf("first draw = %q, want it to start by erasing its own line", first)
	}
	out.Reset()
	if err := r.draw([]string{"a"}); err != nil {
		t.Fatal(err)
	}
	second := out.String()
	if !strings.HasPrefix(second, "\r\x1b[2A") {
		t.Fatalf("redraw = %q, want it to move back up two lines first", second)
	}
	// Erasing to the end of the screen is what removes the rows a narrowed
	// filter no longer draws.
	if !strings.HasSuffix(second, "\x1b[J") {
		t.Fatalf("redraw = %q, want it to erase below the block", second)
	}
	out.Reset()
	if err := r.clear(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "\r\x1b[J" {
		t.Fatalf("clear = %q", got)
	}
}

// The long descriptive column absorbs a narrow terminal so the short trailing
// columns stay visible instead of being pushed off the right edge.
func TestFitWidthsShrinksTheWidestColumn(t *testing.T) {
	got := fitWidths([]int{4, 60, 6, 3}, nil, 60)
	if got[0] != 4 || got[2] != 6 || got[3] != 3 {
		t.Fatalf("fitted = %v, want the narrow columns untouched", got)
	}
	total := 2 + 3*len(columnGap)
	for _, w := range got {
		total += w
	}
	if total > 60 {
		t.Fatalf("fitted row width = %d, want it within 60: %v", total, got)
	}
	// Nothing to shrink below the readable floor: the line truncation takes over
	// rather than the loop spinning.
	if got := fitWidths([]int{8, 8, 8}, nil, 10); len(got) != 3 || got[0] != 8 {
		t.Fatalf("fitted = %v, want the floors kept", got)
	}
	if got := fitWidths(nil, nil, 80); got != nil {
		t.Fatalf("fitted = %v, want nil for no columns", got)
	}
}

func TestRenderKeepsTrailingColumnsVisible(t *testing.T) {
	l := newList(Options{Title: "t", Items: []Item{
		{Cells: []string{"!142", strings.Repeat("long title ", 12), "alice", "2d"}},
	}}, 24, false)
	row := l.render(80)[1]
	if !strings.Contains(row, "alice") || !strings.Contains(row, "2d") {
		t.Fatalf("row = %q, want the author and the age still on screen", row)
	}
}

func TestHintShortensForNarrowTerminals(t *testing.T) {
	l := newList(Options{Items: scriptedItems}, 24, false)
	if got := l.hint(200); got != longHint {
		t.Fatalf("hint = %q, want the full key list", got)
	}
	if got := l.hint(60); got != shortHint {
		t.Fatalf("hint = %q, want the short key list", got)
	}
	if got := l.hint(20); displayWidth(got) > 20 {
		t.Fatalf("hint = %q, want it truncated to the width", got)
	}
	// A range list says what Enter does there, and says it again once a range
	// is open.
	r := newList(Options{Items: scriptedItems, Range: true}, 24, false)
	if got := r.hint(200); got != longRangeHint {
		t.Fatalf("hint = %q, want the range key list", got)
	}
	r.apply(key{kind: keyEnter})
	if got := r.hint(200); got != longSpanHint {
		t.Fatalf("hint = %q, want the open-range key list", got)
	}
}

// Two long columns must end up comparable rather than one keeping its full
// width while the other is cut to the floor.
func TestFitWidthsLevelsWideColumns(t *testing.T) {
	got := fitWidths([]int{43, 60, 13, 3}, nil, 80)
	if diff := got[0] - got[1]; diff > 1 || diff < -1 {
		t.Fatalf("fitted = %v, want the two wide columns levelled", got)
	}
	if got[2] != 13 || got[3] != 3 {
		t.Fatalf("fitted = %v, want the narrow columns untouched", got)
	}
	total := 2 + 3*len(columnGap)
	for _, w := range got {
		total += w
	}
	if total > 80 {
		t.Fatalf("row width = %d, want it within 80: %v", total, got)
	}
}

// The marker explains why a row is preselected, so it must survive a narrow
// terminal instead of being the first thing cut.
func TestRenderKeepsTheMarkerVisible(t *testing.T) {
	l := newList(Options{Title: "t", Items: []Item{
		{Cells: []string{"★", "!142", strings.Repeat("long title ", 12), "David Grieser", "37m"}},
		{Cells: []string{"", "!139", strings.Repeat("other title ", 12), "David Grieser", "46m"}},
	}}, 24, false)
	row := l.render(80)[1]
	if !strings.Contains(row, "★") {
		t.Fatalf("row = %q, want the marker intact", row)
	}
	if displayWidth(row) > 80 {
		t.Fatalf("row is %d wide: %q", displayWidth(row), row)
	}
}

// scriptedInput replays chunks of terminal input; graces is what the short
// second read returns for each pending ESC, in order.
type scriptedInput struct {
	chunks [][]byte
	graces [][]byte
	err    error
	reads  int
}

func (s *scriptedInput) read() ([]byte, error) {
	if s.reads >= len(s.chunks) {
		if s.err != nil {
			return nil, s.err
		}
		return nil, io.EOF
	}
	chunk := s.chunks[s.reads]
	s.reads++
	return chunk, nil
}

func (s *scriptedInput) grace() []byte {
	if len(s.graces) == 0 {
		return nil
	}
	next := s.graces[0]
	s.graces = s.graces[1:]
	return next
}

func selectScripted(t *testing.T, items []Item, source input) (int, error) {
	t.Helper()
	first, _, err := selectRangeScripted(t, Options{Title: "t", Items: items}, source)
	return first, err
}

func selectRangeScripted(t *testing.T, opts Options, source input) (int, int, error) {
	t.Helper()
	state := newList(opts, 24, false)
	return selectFrom(state, &renderer{w: &strings.Builder{}}, func() (int, int) { return 80, 24 }, 0, source)
}

var scriptedItems = []Item{
	{Cells: []string{"!142", "first"}},
	{Cells: []string{"!139", "second"}},
}

func TestSelectFromArrowAndEnter(t *testing.T) {
	index, err := selectScripted(t, scriptedItems, &scriptedInput{chunks: [][]byte{[]byte("\x1b[B"), []byte("\r")}})
	if err != nil {
		t.Fatal(err)
	}
	if index != 1 {
		t.Fatalf("index = %d, want the second row", index)
	}
}

// A lone Esc must abort on its own: it once sat in the buffer waiting for a
// continuation byte that never comes, leaving the key looking dead.
func TestSelectFromLoneEscapeAborts(t *testing.T) {
	source := &scriptedInput{chunks: [][]byte{[]byte("\x1b")}}
	if _, err := selectScripted(t, scriptedItems, source); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
}

// An escape sequence split across reads must still move the cursor, which is
// what the grace period buys.
func TestSelectFromSplitEscapeSequence(t *testing.T) {
	source := &scriptedInput{
		chunks: [][]byte{[]byte("\x1b"), []byte("\r")},
		graces: [][]byte{[]byte("[B")},
	}
	index, err := selectScripted(t, scriptedItems, source)
	if err != nil {
		t.Fatal(err)
	}
	if index != 1 {
		t.Fatalf("index = %d, want the split arrow key to have moved the cursor", index)
	}
}

// A closed terminal is not a selection; it ends the list the way Esc does.
func TestSelectFromEOFAborts(t *testing.T) {
	if _, err := selectScripted(t, scriptedItems, &scriptedInput{}); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
}

func TestSelectFromReadFailure(t *testing.T) {
	want := errors.New("tty gone")
	_, err := selectScripted(t, scriptedItems, &scriptedInput{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want it to wrap %v", err, want)
	}
}

func TestSelectRejectsEmptyListAndNonTerminals(t *testing.T) {
	if _, err := Select(nil, nil, Options{}); !errors.Is(err, ErrNoItems) {
		t.Fatalf("err = %v, want ErrNoItems", err)
	}
	if _, err := Select(nil, nil, Options{Items: scriptedItems}); !errors.Is(err, ErrNotATerminal) {
		t.Fatalf("err = %v, want ErrNotATerminal", err)
	}
}

// Rune count is not cell width: a row of CJK text that "fits" by rune count
// would wrap, and a wrapped row desynchronizes the in-place redraw from the
// screen.
func TestWidthHelpersCountDisplayCells(t *testing.T) {
	const wide = "日本語テキスト" // 7 runes, 14 cells
	if got := displayWidth(wide); got != 14 {
		t.Fatalf("displayWidth = %d, want 14", got)
	}
	got := truncate(wide, 8)
	if displayWidth(got) > 8 {
		t.Fatalf("truncate = %q (%d cells), want at most 8", got, displayWidth(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncate = %q, want the cut marked", got)
	}
	// A double-width rune on the boundary leaves the result a cell short; pad
	// closes the gap so columns still line up.
	if padded := pad(got, 8); displayWidth(padded) != 8 {
		t.Fatalf("pad = %q (%d cells), want exactly 8", padded, displayWidth(padded))
	}
	if got := pad("日本", 6); displayWidth(got) != 6 {
		t.Fatalf("pad = %q (%d cells), want 6", got, displayWidth(got))
	}
	// A combining mark occupies no cell of its own.
	if got := displayWidth("é"); got != 1 {
		t.Fatalf("displayWidth of a combining sequence = %d, want 1", got)
	}
}

func TestRenderRowsStayWithinWidthWithWideRunes(t *testing.T) {
	l := newList(Options{Title: "レビュー", Items: []Item{
		{Cells: []string{"★", "!142", strings.Repeat("日本語のタイトル", 6), "たなか", "2d"}},
		{Cells: []string{"", "!139", strings.Repeat("🚀 emoji title ", 6), "bob", "4d"}},
	}}, 24, false)
	for _, line := range l.render(60) {
		if displayWidth(line) > 60 {
			t.Fatalf("line is %d cells wide: %q", displayWidth(line), line)
		}
	}
}

func TestRenderColoursColumnsAndCursorRow(t *testing.T) {
	opts := Options{
		Title: "Open merge requests in grp/proj",
		Items: []Item{
			{Cells: []string{"★", "!142", "feat: x", "alice", "2d"}},
			{Cells: []string{"", "!139", "fix: y", "bob", "4d"}},
		},
		CellStyles: []string{StyleMark, StyleIdentifier, StyleText, StyleAuthor, StyleAge},
	}
	lines := newList(opts, 24, true).render(80)
	if !strings.HasPrefix(lines[0], "\x1b["+styleTitle+"m") {
		t.Fatalf("title = %q, want it styled", lines[0])
	}
	cursorRow, plainRow := lines[1], lines[2]
	// Every segment of the cursor row carries the highlight background, so the
	// bar is continuous instead of ending at the first colour reset.
	if strings.Count(cursorRow, styleCursorRow) < 4 {
		t.Fatalf("cursor row = %q, want the background on every segment", cursorRow)
	}
	if !strings.Contains(cursorRow, styleCursorRow+";"+StyleIdentifier) {
		t.Fatalf("cursor row = %q, want the column colour kept under the highlight", cursorRow)
	}
	if !strings.Contains(cursorRow, styleCursorRow+";"+StyleMark) {
		t.Fatalf("cursor row = %q, want the marker column coloured under the highlight", cursorRow)
	}
	for _, want := range []string{StyleIdentifier, StyleText, StyleAuthor, StyleAge} {
		if !strings.Contains(plainRow, "\x1b["+want+"m") {
			t.Fatalf("row = %q, want column style %q", plainRow, want)
		}
	}
	if strings.Contains(plainRow, styleCursorRow) {
		t.Fatalf("row = %q, want the highlight only on the cursor row", plainRow)
	}
	// The highlight must span the full width, or it ends mid-row.
	if displayWidth(stripANSI(cursorRow)) != 80 {
		t.Fatalf("cursor row is %d cells: %q", displayWidth(stripANSI(cursorRow)), cursorRow)
	}
}

// With colour off the layout is unchanged and no escape sequence is emitted:
// NO_COLOR, a pipe, or a terminal that cannot colour must still show the list.
func TestRenderWithoutColour(t *testing.T) {
	opts := Options{
		Title:      "t",
		Items:      []Item{{Cells: []string{"★", "!142", "feat: x"}}},
		CellStyles: []string{StyleMark, StyleIdentifier, StyleText},
	}
	for _, line := range newList(opts, 24, false).render(80) {
		if strings.Contains(line, "\x1b[") {
			t.Fatalf("line = %q, want no escape sequences", line)
		}
	}
}

// A caller may style fewer columns than it fills; the rest stay unstyled
// instead of reading past the slice.
func TestCellStyleShorterThanCells(t *testing.T) {
	item := Item{Cells: []string{"a", "b", "c"}}
	l := newList(Options{Items: []Item{item}, CellStyles: []string{StyleIdentifier}}, 24, true)
	if got := l.cellStyle(item, 2); got != "" {
		t.Fatalf("cellStyle(2) = %q, want unstyled", got)
	}
	if got := l.cellStyle(item, 0); got != StyleIdentifier {
		t.Fatalf("cellStyle(0) = %q", got)
	}
}

// A row's own colours win over the column's, and an empty entry falls back
// instead of clearing the column's colour.
func TestRowStylesOverrideColumnStyles(t *testing.T) {
	item := Item{Cells: []string{"a", "b", "c"}, CellStyles: []string{"", StyleFresh}}
	l := newList(Options{Items: []Item{item}, CellStyles: []string{StyleIdentifier, StyleAge, StyleText}}, 24, true)
	if got := l.cellStyle(item, 0); got != StyleIdentifier {
		t.Fatalf("cellStyle(0) = %q, want the column style", got)
	}
	if got := l.cellStyle(item, 1); got != StyleFresh {
		t.Fatalf("cellStyle(1) = %q, want the row style", got)
	}
	if got := l.cellStyle(item, 2); got != StyleText {
		t.Fatalf("cellStyle(2) = %q, want the column style past the row slice", got)
	}
}

// The same person keeps one colour; different people get different ones.
func TestAuthorStyleIsStablePerPerson(t *testing.T) {
	alice := AuthorStyle("Alice")
	if alice == "" {
		t.Fatal("a named author must get a colour")
	}
	if got := AuthorStyle(" alice "); got != alice {
		t.Fatalf("AuthorStyle(%q) = %q, want the same colour as %q", " alice ", got, alice)
	}
	if !slices.Contains(authorStyles, alice) {
		t.Fatalf("AuthorStyle = %q, want one of the palette entries", alice)
	}
	distinct := map[string]bool{}
	for _, name := range []string{"alice", "bob", "carol", "dave", "erin", "frank", "grace"} {
		distinct[AuthorStyle(name)] = true
	}
	if len(distinct) < 4 {
		t.Fatalf("seven names produced %d colours, want the palette spread out", len(distinct))
	}
	if got := AuthorStyle("  "); got != "" {
		t.Fatalf("AuthorStyle of a blank name = %q, want none", got)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// A narrow name column loses the tail of a long value, so the selected row's
// full value is shown after the title — and follows the cursor.
func TestRenderTitleCarriesTheSelectedDetail(t *testing.T) {
	opts := Options{
		Title: "Base branch — reviewed against:",
		Items: []Item{
			{Cells: []string{"main"}, Detail: "origin/main"},
			{Cells: []string{"feat/…"}, Detail: "origin/feat/tree-sitter-parse-cap-and-cache"},
		},
	}
	l := newList(opts, 24, true)
	title := l.render(120)[0]
	if !strings.Contains(title, "Base branch — reviewed against:") {
		t.Fatalf("title = %q", title)
	}
	// The detail is a ref, so its separators fade the way a progress line
	// renders one, and the parts keep the prompt's colour.
	if !strings.Contains(title, " \x1b["+StyleDetail+"morigin\x1b[0m\x1b["+styleMsgPunct+"m/\x1b[0m\x1b["+StyleDetail+"mmain") {
		t.Fatalf("title = %q, want the detail in its own colour with a faded separator", title)
	}
	l.apply(key{kind: keyDown})
	if title := l.render(120)[0]; !strings.Contains(title, "tree-sitter-parse-cap-and-cache") {
		t.Fatalf("title = %q, want the detail to follow the cursor", title)
	}
	// A caller's own colour wins, and rows without a detail leave the title bare.
	custom := newList(Options{Title: "t", Items: []Item{{Cells: []string{"a"}, Detail: "d"}}, DetailStyle: StyleMark}, 24, true)
	if title := custom.render(80)[0]; !strings.Contains(title, " \x1b["+StyleMark+"md") {
		t.Fatalf("title = %q, want the caller's detail style", title)
	}
	bare := newList(Options{Title: "t", Items: []Item{{Cells: []string{"a"}}}}, 24, true)
	if title := bare.render(80)[0]; strings.Contains(title, StyleDetail) {
		t.Fatalf("title = %q, want no detail segment", title)
	}
}

// An unimportant column collapses to a stub before an important one loses a
// cell: a branch name is spelled out in the title line, the tip message is not.
func TestFitWidthsSpendsTheUnimportantColumnFirst(t *testing.T) {
	// mark, name (low), message, author, age
	widths := []int{1, 43, 60, 13, 3}
	priorities := []int{PriorityNormal, PriorityLow}
	got := fitWidths(widths, priorities, 80)
	if got[1] != stubColumnWidth {
		t.Fatalf("name column = %d, want it down to its floor %d: %v", got[1], stubColumnWidth, got)
	}
	// A small overflow is taken out of the name alone: the message is not
	// touched until the name has nothing left to give.
	if small := fitWidths(widths, priorities, 125); small[2] != widths[2] || small[1] >= widths[1] {
		t.Fatalf("fitted = %v, want only the name shortened", small)
	}
	if got[3] != 13 || got[0] != 1 || got[4] != 3 {
		t.Fatalf("fitted = %v, want the other columns untouched", got)
	}
	total := 2 + 4*len(columnGap)
	for _, w := range got {
		total += w
	}
	if total > 80 {
		t.Fatalf("row width = %d, want it within 80: %v", total, got)
	}
	// A wide terminal needs none of that: nothing is shortened at all.
	if wide := fitWidths(widths, priorities, 160); !slices.Equal(wide, widths) {
		t.Fatalf("fitted = %v, want the full widths on a wide terminal", wide)
	}
	// Once the low column is spent, the rest level as before.
	tight := fitWidths(widths, priorities, 50)
	if tight[1] != stubColumnWidth || tight[2] >= got[2] {
		t.Fatalf("fitted = %v, want the message to give way only after the name", tight)
	}
	if tight[2] < tight[3] {
		t.Fatalf("fitted = %v, want the message and the author levelled, not one collapsed", tight)
	}
}

func TestColumnPriorityDefaultsToNormal(t *testing.T) {
	if got := columnPriority(nil, 3); got != PriorityNormal {
		t.Fatalf("columnPriority = %d, want PriorityNormal", got)
	}
	if got := columnPriority([]int{PriorityLow}, 0); got != PriorityLow {
		t.Fatalf("columnPriority = %d, want PriorityLow", got)
	}
	if got := columnPriority([]int{PriorityLow}, 2); got != PriorityNormal {
		t.Fatalf("columnPriority past the slice = %d, want PriorityNormal", got)
	}
}

// A conventional-commit prefix is taken apart the way the git-color helper
// renders a git log: the type and what it touched step out of the message.
func TestMessageSegments(t *testing.T) {
	got := messageSegments("feat(review): cluster merge", StyleText)
	want := []segment{
		{"feat", styleMsgType},
		{"(", styleMsgPunct},
		{"review", styleMsgScope},
		{")", styleMsgPunct},
		{":", styleMsgPunct},
		{" ", ""},
		{"cluster merge", StyleText},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %+v, want %+v", got, want)
	}
	// A prefix without a scope, and a breaking change.
	if got := messageSegments("fix!: drop the flag", StyleText); !slices.Equal(got, []segment{
		{"fix", styleMsgType},
		{"!", styleMsgBreaking},
		{":", styleMsgPunct},
		{" ", ""},
		{"drop the flag", StyleText},
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
		if got := messageSegments(plain, StyleText); !slices.Equal(got, []segment{{plain, StyleText}}) {
			t.Fatalf("segments for %q = %+v, want one plain segment", plain, got)
		}
	}
	// The padding a column adds stays part of the trailing text, unpainted or
	// not, so the column width is unchanged by the split.
	padded := messageSegments("docs: readme   ", StyleText)
	var rebuilt string
	for _, part := range padded {
		rebuilt += part.text
	}
	if rebuilt != "docs: readme   " {
		t.Fatalf("segments rebuild to %q", rebuilt)
	}
}

// A ref reads as parts: the separators fade the way a progress line renders
// "repo @ head → base".
func TestRefSegments(t *testing.T) {
	got := refSegments("origin/feat/pick", StyleDetail)
	want := []segment{
		{"origin", StyleDetail},
		{"/", styleMsgPunct},
		{"feat", StyleDetail},
		{"/", styleMsgPunct},
		{"pick", StyleDetail},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %+v, want %+v", got, want)
	}
	// Colons separate too (a refspec, a submodule path).
	if got := refSegments("refs/heads/main:main", StyleDetail); len(got) != 7 {
		t.Fatalf("segments = %+v, want every separator split out", got)
	}
	// Nothing to split: one segment.
	if got := refSegments("main", StyleDetail); !slices.Equal(got, []segment{{"main", StyleDetail}}) {
		t.Fatalf("segments = %+v", got)
	}
}

func TestCellSegmentsFollowTheColumnKind(t *testing.T) {
	if got := cellSegments(KindPlain, "feat(x): y", StyleText); len(got) != 1 {
		t.Fatalf("a plain column must not be split: %+v", got)
	}
	if got := cellSegments(KindMessage, "feat(x): y", StyleText); len(got) < 5 {
		t.Fatalf("a message column must be split: %+v", got)
	}
	if got := cellSegments(KindRef, "origin/main", StyleText); len(got) != 3 {
		t.Fatalf("a ref column must be split: %+v", got)
	}
}

// The split survives the row rendering, colours and highlight included.
func TestRenderRowPaintsCommitPrefixAndRef(t *testing.T) {
	l := newList(Options{
		Title:       "t",
		Items:       []Item{{Cells: []string{"origin/feat/pick", "feat(review): cluster merge"}}},
		CellStyles:  []string{StyleText, StyleText},
		ColumnKinds: []ColumnKind{KindRef, KindMessage},
	}, 24, true)
	row := l.render(120)[1]
	for _, want := range []string{
		styleCursorRow + ";" + styleMsgType,  // the type, under the highlight
		styleCursorRow + ";" + styleMsgScope, // the scope
		styleCursorRow + ";" + styleMsgPunct, // the parens, colon and the ref's slashes
	} {
		if !strings.Contains(row, want) {
			t.Fatalf("row = %q, want %q", row, want)
		}
	}
}

func rangeItems() []Item {
	return []Item{
		{Cells: []string{"e796cdf", "fix: newest"}, Detail: "e796cdf"},
		{Cells: []string{"91ecb94", "feat: middle"}, Detail: "91ecb94"},
		{Cells: []string{"1a2b3c4", "chore: oldest"}, Detail: "1a2b3c4"},
	}
}

// The first Enter opens the range, moving covers the rows between, and the
// second Enter ends the list on both ends — ordered as the rows are.
func TestSelectRangeAnchorsThenExtends(t *testing.T) {
	source := &scriptedInput{chunks: [][]byte{[]byte("\r"), []byte("\x1b[B"), []byte("\r")}}
	first, last, err := selectRangeScripted(t, Options{Title: "t", Items: rangeItems(), Range: true}, source)
	if err != nil {
		t.Fatal(err)
	}
	if first != 0 || last != 1 {
		t.Fatalf("range = %d..%d, want the first two rows", first, last)
	}
	// Extending upward from a lower row yields the same pair, so the caller
	// never has to order them.
	source = &scriptedInput{chunks: [][]byte{[]byte("\x1b[B\x1b[B"), []byte("\r"), []byte("\x1b[A"), []byte("\r")}}
	first, last, err = selectRangeScripted(t, Options{Title: "t", Items: rangeItems(), Range: true}, source)
	if err != nil {
		t.Fatal(err)
	}
	if first != 1 || last != 2 {
		t.Fatalf("range = %d..%d, want rows 1 and 2", first, last)
	}
	// A range of one row is a single choice, which is what Enter twice means.
	source = &scriptedInput{chunks: [][]byte{[]byte("\r"), []byte("\r")}}
	if first, last, err = selectRangeScripted(t, Options{Items: rangeItems(), Range: true}, source); err != nil || first != 0 || last != 0 {
		t.Fatalf("range = %d..%d, err = %v; want row 0 twice", first, last, err)
	}
}

// Esc closes an open range before it leaves the list, so a mis-anchored range
// costs one keypress, not the whole selection.
func TestSelectRangeEscapeClosesTheRangeFirst(t *testing.T) {
	source := &scriptedInput{chunks: [][]byte{
		[]byte("\r"),     // open the range on row 0
		[]byte("\x1b[B"), // extend
		[]byte("\x1b"),   // drop the range
		[]byte("\x1b[B"), // move on
		[]byte("\r"),     // open a new range on row 2
		[]byte("\r"),     // and take it
	}}
	first, last, err := selectRangeScripted(t, Options{Items: rangeItems(), Range: true}, source)
	if err != nil {
		t.Fatal(err)
	}
	if first != 2 || last != 2 {
		t.Fatalf("range = %d..%d, want the row the second range opened on", first, last)
	}
	// With no range open, Esc still leaves the list.
	if _, _, err := selectRangeScripted(t, Options{Items: rangeItems(), Range: true},
		&scriptedInput{chunks: [][]byte{[]byte("\x1b")}}); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
}

// Without Options.Range a single Enter still ends the list, and both results
// are that row.
func TestSelectRangeWithoutRangeMode(t *testing.T) {
	first, last, err := selectRangeScripted(t, Options{Items: rangeItems()},
		&scriptedInput{chunks: [][]byte{[]byte("\x1b[B"), []byte("\r")}})
	if err != nil || first != 1 || last != 1 {
		t.Fatalf("range = %d..%d, err = %v; want row 1 twice", first, last, err)
	}
}

// A filter may find the first end, but it cannot survive the range: a hidden
// row between the ends would be reviewed without ever being shown.
func TestSelectRangeClearsTheFilterOnAnchor(t *testing.T) {
	l := newList(Options{Items: rangeItems(), Range: true}, 24, false)
	l.setFilter([]rune("oldest"))
	if len(l.matches) != 1 {
		t.Fatalf("matches = %d, want the filter to have narrowed the list", len(l.matches))
	}
	l.apply(key{kind: keyEnter})
	if len(l.filter) != 0 || len(l.matches) != 3 {
		t.Fatalf("filter = %q over %d matches, want it cleared", string(l.filter), len(l.matches))
	}
	// The anchor followed its row through the reset.
	if first, last := l.selectedRange(); first != 2 || last != 2 {
		t.Fatalf("range = %d..%d, want the row the filter had found", first, last)
	}
	// Typing does not reopen the filter while the range is open.
	l.apply(key{kind: keyRune, rune: 'x'})
	l.apply(key{kind: keyClearFilter})
	l.apply(key{kind: keyBackspace})
	if len(l.filter) != 0 || len(l.matches) != 3 {
		t.Fatalf("filter = %q over %d matches, want the visible set frozen", string(l.filter), len(l.matches))
	}
}

// The whole span is highlighted, so the range reads as one block, and the title
// line says how much it covers.
func TestRenderShowsTheOpenSpan(t *testing.T) {
	l := newList(Options{Title: "Commits to review:", Items: rangeItems(), Range: true, RangeUnit: "commit"}, 24, true)
	l.apply(key{kind: keyEnter})
	l.apply(key{kind: keyDown})
	lines := l.render(100)
	if !strings.Contains(lines[0], "2 commits") {
		t.Fatalf("title = %q, want the span size", lines[0])
	}
	// One row is one commit, not "1 commits".
	l.apply(key{kind: keyUp})
	if title := l.render(100)[0]; !strings.Contains(title, "1 commit\x1b") {
		t.Fatalf("title = %q, want a singular count", title)
	}
	if !strings.Contains(lines[0], "1a2b3c4") && !strings.Contains(lines[0], "91ecb94") {
		t.Fatalf("title = %q, want the ends of the span", lines[0])
	}
	for _, row := range lines[1:3] {
		if !strings.Contains(row, styleCursorRow) {
			t.Fatalf("row = %q, want it inside the highlight", row)
		}
	}
	if strings.Contains(lines[3], styleCursorRow) {
		t.Fatalf("row = %q, want it outside the highlight", lines[3])
	}
	// The marker stays on the row the keys move.
	if !strings.Contains(lines[2], cursorMarker) || strings.Contains(lines[1], cursorMarker) {
		t.Fatalf("rows = %q / %q, want the marker on the cursor row only", lines[1], lines[2])
	}
}

// viewOptions is a two-scope list where the narrow scope is a subset of the
// wide one, the shape every scoped picker has.
func viewOptions() Options {
	return Options{
		Title: "Sessions",
		Views: []View{
			{Label: "branch", Title: "Sessions on feat/x", Items: []Item{
				{Key: "b", Cells: []string{"b2", "feat/x"}},
			}, Empty: "no sessions for this branch"},
			{Label: "all", Items: []Item{
				{Key: "a", Cells: []string{"a1", "main"}},
				{Key: "b", Cells: []string{"b2", "feat/x"}},
				{Key: "c", Cells: []string{"c3", "fix/y"}},
			}},
		},
	}
}

func TestViewSwitchKeepsTheRowAndTheFilter(t *testing.T) {
	l := newList(viewOptions(), 24, false)
	if got := l.title; got != "Sessions on feat/x" {
		t.Fatalf("title = %q, want the view's own title", got)
	}
	l.setFilter([]rune("feat"))
	l.apply(key{kind: keyNextView})
	if l.view != 1 {
		t.Fatalf("view = %d, want the second scope", l.view)
	}
	if got := l.title; got != "Sessions" {
		t.Fatalf("title = %q, want the Options title for a view without one", got)
	}
	if got := string(l.filter); got != "feat" {
		t.Fatalf("filter = %q, want it carried across the switch", got)
	}
	if got := l.items[l.selected()].Key; got != "b" {
		t.Fatalf("selected key = %q, want the row the cursor was on", got)
	}
	// Widening past the last scope wraps back to the first, and the row the
	// cursor sits on survives that too.
	l.setFilter(nil)
	l.move(2) // "c3", which the branch scope does not hold
	l.apply(key{kind: keyNextView})
	if l.view != 0 {
		t.Fatalf("view = %d, want the wrap back to the first scope", l.view)
	}
	if got := l.items[l.selected()].Key; got != "b" {
		t.Fatalf("selected key = %q, want the top of the scope that lacks the row", got)
	}
	l.apply(key{kind: keyPrevView})
	if l.view != 1 {
		t.Fatalf("view = %d, want the backwards wrap", l.view)
	}
}

func TestViewSwitchIgnoredWithoutScopesAndInsideARange(t *testing.T) {
	single := newList(Options{Items: scriptedItems}, 24, false)
	single.apply(key{kind: keyNextView})
	if single.view != 0 {
		t.Fatalf("view = %d, want a one-scope list to stay put", single.view)
	}
	opts := viewOptions()
	opts.Range, opts.RangeUnit = true, "session"
	ranged := newList(opts, 24, false)
	ranged.apply(key{kind: keyEnter}) // opens the range
	ranged.apply(key{kind: keyNextView})
	if ranged.view != 0 {
		t.Fatalf("view = %d, want the scope frozen while a range is open", ranged.view)
	}
}

func TestStatusLineCarriesPositionScopesAndFilter(t *testing.T) {
	l := newList(viewOptions(), 24, false)
	status := func() string {
		lines := l.render(80)
		return lines[len(lines)-2]
	}
	// Position, scopes and filter share one line, and the filter half of it
	// only appears once something is typed.
	if got := status(); got != "1 of 1 · [branch] · all" {
		t.Fatalf("status = %q", got)
	}
	l.apply(key{kind: keyNextView})
	l.setFilter([]rune("feat"))
	if got := status(); got != "1 of 1 · branch · [all] · filter: feat" {
		t.Fatalf("status = %q", got)
	}
	if !strings.Contains(l.hint(200), "scope") {
		t.Fatalf("hint = %q, want the scope key named", l.hint(200))
	}
	// One scope is a plain list: the line is the position alone, and no key is
	// offered for switching.
	plain := newList(Options{Items: scriptedItems}, 24, false)
	lines := plain.render(80)
	if got := lines[len(lines)-2]; got != "1 of 2" {
		t.Fatalf("status = %q, want the position alone", got)
	}
	if strings.Contains(plain.hint(200), "scope") {
		t.Fatalf("hint = %q, want no scope key in a one-scope list", plain.hint(200))
	}
}

func TestEmptyScopeSaysSoAndSelectsNothing(t *testing.T) {
	opts := viewOptions()
	opts.Views[0].Items = nil
	l := newList(opts, 24, false)
	lines := l.render(80)
	if !strings.Contains(lines[1], "no sessions for this branch") {
		t.Fatalf("line = %q, want the scope's own empty wording", lines[1])
	}
	if action := l.apply(key{kind: keyEnter}); action != actionNone {
		t.Fatalf("Enter in an empty scope = %v, want nothing selected", action)
	}
	// A filter that matches nothing is a different situation and says so.
	l.apply(key{kind: keyNextView})
	l.setFilter([]rune("zzz"))
	if got := l.render(80)[1]; !strings.Contains(got, "no match") {
		t.Fatalf("line = %q, want the no-match wording", got)
	}
}

func TestSelectViewReportsTheScopeItEndedIn(t *testing.T) {
	state := newList(viewOptions(), 24, false)
	source := &scriptedInput{chunks: [][]byte{[]byte("\t"), []byte("\x1b[B"), []byte("\r")}}
	first, _, err := selectFrom(state, &renderer{w: &strings.Builder{}}, func() (int, int) { return 80, 24 }, 0, source)
	if err != nil {
		t.Fatal(err)
	}
	if state.view != 1 {
		t.Fatalf("view = %d, want the scope Tab moved to", state.view)
	}
	if first != 2 {
		t.Fatalf("index = %d, want the row below the one the cursor followed", first)
	}
}

func TestSelectViewWithoutAnyItemsIsNoItems(t *testing.T) {
	_, _, err := SelectView(nil, nil, Options{Views: []View{{Label: "a"}, {Label: "b"}}})
	if !errors.Is(err, ErrNoItems) {
		t.Fatalf("err = %v, want ErrNoItems", err)
	}
	// One scope with rows is enough for the list to be worth drawing; without a
	// terminal it fails on that instead.
	_, _, err = SelectView(nil, nil, Options{Views: []View{{Label: "a"}, {Label: "b", Items: scriptedItems}}})
	if !errors.Is(err, ErrNotATerminal) {
		t.Fatalf("err = %v, want ErrNotATerminal", err)
	}
}

func TestRefNoteFadesTheTrailingParenthetical(t *testing.T) {
	item := Item{Cells: []string{"feat/x (uncommitted)"}}
	l := newList(Options{Items: []Item{item}, CellStyles: []string{StyleIdentifier},
		ColumnKinds: []ColumnKind{KindRefNote}}, 24, true)
	row := l.render(80)[1]
	// The cursor row carries its highlight in the same sequence, so the note's
	// own code is matched without assuming it starts one.
	if !strings.Contains(row, StyleNote+"m (uncommitted)") {
		t.Fatalf("row = %q, want the note in its own light grey", row)
	}
	if !strings.Contains(row, StyleSeparator+"m/") {
		t.Fatalf("row = %q, want the ref's separator still faded", row)
	}
	// A parenthesis that is not a trailing note is part of the ref.
	inner := newList(Options{Items: []Item{{Cells: []string{"feat/(x) more"}}},
		ColumnKinds: []ColumnKind{KindRefNote}}, 24, true).render(80)[1]
	if strings.Contains(inner, StyleNote) {
		t.Fatalf("row = %q, want no note painted", inner)
	}
	// Every row's note is painted, not just the widest one's: a column pads the
	// cells it fits, which puts the note somewhere other than the very end.
	padded := newList(Options{
		Items: []Item{
			{Cells: []string{"short (uncommitted)", "a"}},
			{Cells: []string{"a-much-longer-branch-name (uncommitted)", "b"}},
		},
		CellStyles:  []string{StyleIdentifier, ""},
		ColumnKinds: []ColumnKind{KindRefNote},
	}, 24, true).render(120)
	for _, row := range padded[1:3] {
		if !strings.Contains(row, StyleNote+"m (uncommitted)") {
			t.Fatalf("row = %q, want the note painted despite the column padding", row)
		}
	}
}

func TestTruncateMiddleKeepsBothEnds(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"archiefmeester", 14, "archiefmeester"},
		{"archiefmeester", 7, "arc…ter"},
		{"archiefmeester", 6, "arc…er"},
		{"archiefmeester", 2, "a…"},
		{"archiefmeester", 1, "…"},
		{"archiefmeester", 0, ""},
		// A double-width rune never lands half inside the kept ends.
		{"レビューを見る", 5, "レ…る"},
	}
	for _, c := range cases {
		got := TruncateMiddle(c.in, c.width)
		if got != c.want {
			t.Fatalf("TruncateMiddle(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
		if displayWidth(got) > c.width {
			t.Fatalf("TruncateMiddle(%q, %d) = %q, wider than asked", c.in, c.width, got)
		}
	}
}

func TestDetailFieldsCarryTheirOwnColours(t *testing.T) {
	l := newList(Options{
		Title: "Session",
		Items: []Item{{
			Cells: []string{"57e092c6"},
			Details: []Field{
				{Text: "57e092c6-e0f9-4b14-9b88-00b3cacb410a", Style: StyleHash},
				{Text: "/src/nickpit/cmd", Kind: KindRef},
				{Text: "", Style: StyleAge}, // an empty field adds no separator
			},
		}},
	}, 24, true)
	header := l.render(120)[0]
	if !strings.Contains(header, StyleHash+"m57e092c6-e0f9") {
		t.Fatalf("header = %q, want the id in the id column's colour", header)
	}
	// A field without a colour stays in the terminal's own, with a ref's
	// separators faded — the way an unstyled column shows a path.
	if !strings.Contains(header, "src\x1b["+StyleSeparator+"m/") {
		t.Fatalf("header = %q, want the path unstyled with faded separators", header)
	}
	plain := stripANSI(header)
	if plain != "Session · 57e092c6-e0f9-4b14-9b88-00b3cacb410a · /src/nickpit/cmd" {
		t.Fatalf("header = %q", plain)
	}
}

func TestMatchFieldsHonourTheirMinimumTerm(t *testing.T) {
	items := []Item{
		{Cells: []string{"row one"}, MatchFields: []MatchField{
			{Text: "feat/abc"},
			{Text: "3f1a9c22-0000-4000-8000-000000000001", MinTerm: 4},
		}},
		{Cells: []string{"row two"}, MatchFields: []MatchField{
			{Text: "main"},
			{Text: "aaaabbbb-0000-4000-8000-000000000002", MinTerm: 4},
		}},
	}
	l := newList(Options{Items: items}, 24, false)
	matched := func(filter string) []int {
		l.setFilter([]rune(filter))
		return append([]int(nil), l.matches...)
	}
	// Long enough to be an id, so it reaches one.
	if got := matched("3f1a"); len(got) != 1 || got[0] != 0 {
		t.Fatalf("matches for an id term = %v, want the first row", got)
	}
	// Too short: the id is not searched, and nothing else holds it.
	if got := matched("3f1"); len(got) != 0 {
		t.Fatalf("matches for a short term = %v, want none", got)
	}
	// A short term still searches the fields that set no minimum.
	if got := matched("abc"); len(got) != 1 || got[0] != 0 {
		t.Fatalf("matches for a short term in an open field = %v, want the first row", got)
	}
	// Terms still narrow each other, across fields.
	if got := matched("feat 3f1a"); len(got) != 1 || got[0] != 0 {
		t.Fatalf("matches for two terms = %v, want the row carrying both", got)
	}
	if got := matched("main 3f1a"); len(got) != 0 {
		t.Fatalf("matches for terms from two rows = %v, want none", got)
	}
	// An item without fields still matches on its cells, as before.
	plain := newList(Options{Items: scriptedItems}, 24, false)
	plain.setFilter([]rune("first"))
	if len(plain.matches) != 1 {
		t.Fatalf("matches without fields = %v", plain.matches)
	}
}

func TestLazyViewLoadsOnceWhenShown(t *testing.T) {
	loads := 0
	opts := Options{
		Views: []View{
			{Label: "local", Items: scriptedItems},
			{Label: "remote", Loading: "asking the server…", Load: func() ([]Item, error) {
				loads++
				return []Item{{Cells: []string{"!42", "from the server"}}}, nil
			}},
		},
	}
	state := newList(opts, 24, false)
	// Tab, then Enter on the row the scope brought in.
	source := &scriptedInput{chunks: [][]byte{[]byte("\t"), []byte("\r")}}
	index, _, err := selectFrom(state, &renderer{w: &strings.Builder{}}, func() (int, int) { return 80, 24 }, 0, source)
	if err != nil {
		t.Fatal(err)
	}
	if loads != 1 {
		t.Fatalf("Load ran %d times, want once", loads)
	}
	if index != 0 || state.view != 1 || state.items[0].Cells[1] != "from the server" {
		t.Fatalf("selected %d of view %d: %+v", index, state.view, state.items)
	}
	// Going away and coming back does not fetch again.
	state.apply(key{kind: keyPrevView})
	state.apply(key{kind: keyNextView})
	state.runPendingLoad()
	if loads != 1 {
		t.Fatalf("Load ran %d times, want the rows kept", loads)
	}
}

func TestLazyViewShowsLoadingThenItsFailure(t *testing.T) {
	state := newList(Options{
		Views: []View{
			{Label: "local", Items: scriptedItems},
			{Label: "remote", Loading: "asking the server…", Load: func() ([]Item, error) {
				return nil, errors.New("no token for gitlab.example.com")
			}},
		},
	}, 24, false)
	state.apply(key{kind: keyNextView})
	if got := state.render(80)[1]; !strings.Contains(got, "asking the server…") {
		t.Fatalf("line = %q, want the loading line before the fetch", got)
	}
	state.runPendingLoad()
	if got := state.render(80)[1]; !strings.Contains(got, "no token for gitlab.example.com") {
		t.Fatalf("line = %q, want the failure in place of the rows", got)
	}
	// The other scope is untouched by the one that could not load.
	state.apply(key{kind: keyPrevView})
	if len(state.matches) != len(scriptedItems) {
		t.Fatalf("matches = %d, want the local scope intact", len(state.matches))
	}
}

func TestSelectViewWithOnlyALazyScopeIsNotEmpty(t *testing.T) {
	_, _, err := SelectView(nil, nil, Options{Views: []View{
		{Label: "remote", Load: func() ([]Item, error) { return nil, nil }},
	}})
	if !errors.Is(err, ErrNotATerminal) {
		t.Fatalf("err = %v, want the list to be worth drawing", err)
	}
}
