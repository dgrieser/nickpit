package pick

import (
	"fmt"
	"strings"
)

// action is what a keypress asked of the list.
type action int

const (
	actionNone action = iota
	actionSelect
	actionAbort
)

// The picker's own colours. They are the 256-colour message palette of
// internal/logging/progress.go — a picker row should read like the progress
// lines of the same run — with truecolor left to the stage column there.
// tools/print_colors.sh reproduces them.
const (
	attrReset = "\x1b[0m"
	// styleTitle is the bold white of a banner line (progressColorBold +
	// progressColorWhite).
	styleTitle = "1;38;5;255"
	// stylePosition and styleHint recede below the rows: progressColorGrey for
	// the position, progressColorDarkGrey for the key list.
	stylePosition = "38;5;244"
	styleHint     = "38;5;242"
	// styleNoMatch warns that the filter matched nothing (progressColorWarnYellow).
	styleNoMatch = "38;5;221"
	// styleCursorRow is the highlight background of the selected row, and
	// styleCursorMark its marker: a background rather than reverse video, so
	// the row keeps its own column colours. The background is the live
	// dashboard's lavender pastel dimmed to 35%, the same scale a progress bar
	// dims its unfilled half by, so a picker row and an agent bar belong to one
	// family.
	styleCursorRow  = "48;2;69;56;86"
	styleCursorMark = "1;38;5;255"
)

// Cell styles a caller assigns to its columns via Options.CellStyles. Same
// palette, named by what the column means rather than by its colour.
const (
	// StyleIdentifier is an MR/PR number (progressColorNumberGreen).
	StyleIdentifier = "38;5;118"
	// StyleHash is a commit SHA (progressColorHashDarkGreen).
	StyleHash = "38;5;71"
	// StyleText is a title, commit subject or branch tip message — the column a
	// reader actually reads, so it gets a light tone of its own: a pale
	// lavender, the one code the picker adds to the message palette.
	StyleText = "38;5;189"
	// StyleAuthor is a username (progressColorTaskPink).
	StyleAuthor = "38;5;218"
	// StyleAge is a timestamp column (progressColorGrey), StyleFresh the same
	// column when it just changed (progressColorBoolGreen).
	StyleAge   = "38;5;244"
	StyleFresh = "38;5;156"
	// StyleCaveat marks a qualifier such as a draft request
	// (progressColorWarnYellow).
	StyleCaveat = "38;5;221"
	// StyleDefaultRef marks the repository's default branch — the only ref
	// coloured in a branch list (progressColorBranchToAquaGreen).
	StyleDefaultRef = "38;5;48"
	// StyleMark is a marker column: the ★ on the checked-out branch, and on the
	// request of the branch you are on. It is the gold a progress line paints
	// the head ref in.
	StyleMark = "38;5;214"
	// StyleDetail is the default colour of Item.Detail in the title line: a
	// light sky blue, so the value stands out from the bold white title it
	// follows.
	StyleDetail = "38;5;117"
	// StyleBaseRef and StyleHeadRef name the two sides of a comparison in the
	// aqua green and gold a progress line paints "base → head" in, so a base
	// prompt and a head prompt are told apart at a glance — in the title line
	// and in the confirmation that follows. StyleHeadRef shares its code with
	// StyleMark, which only ever appears in a marker column.
	StyleBaseRef = "38;5;48"
	StyleHeadRef = "38;5;214"
)

const (
	// columnGap separates the aligned cells of a row.
	columnGap = "  "
	// cursorMarker replaces the two leading blanks of an unselected row.
	cursorMarker = "❯ "
	// minVisible is the smallest row window the list draws; a terminal too
	// short for it scrolls rather than dropping the list.
	minVisible = 3
	// defaultVisible caps the rows drawn when the caller gave no cap and the
	// terminal height is unknown.
	defaultVisible = 15
)

// list is the selection state: the items, the current filter, and where the
// cursor and the scroll window sit inside the filtered subset. It is pure —
// keys go in through apply, lines come out of render — so the behaviour is
// testable without a terminal.
type list struct {
	items  []Item
	title  string
	widths []int
	// cellStyles holds the caller's per-column SGR codes, indexed like Cells.
	cellStyles  []string
	kinds       []ColumnKind
	detailStyle string
	// priorities ranks the columns by how long they keep their width.
	priorities []int
	color      bool
	visible    int

	filter  []rune
	matches []int
	cursor  int
	top     int
}

func newList(opts Options, height int, color bool) *list {
	l := &list{
		items:       opts.Items,
		title:       opts.Title,
		widths:      columnWidths(opts.Items),
		cellStyles:  opts.CellStyles,
		kinds:       opts.ColumnKinds,
		detailStyle: firstStyle(opts.DetailStyle, StyleDetail),
		priorities:  opts.ColumnPriority,
		color:       color,
		visible:     visibleRows(opts.MaxVisible, height),
	}
	l.matches = make([]int, len(l.items))
	for i := range l.items {
		l.matches[i] = i
	}
	l.cursor = opts.Initial
	if l.cursor < 0 || l.cursor >= len(l.matches) {
		l.cursor = 0
	}
	l.scrollToCursor()
	return l
}

// visibleRows leaves room for the title, the filter line, and the hint line so
// the whole block fits on screen: a block taller than the terminal would
// scroll, and the cursor-up redraw would then repaint the wrong lines.
func visibleRows(maxVisible, height int) int {
	rows := defaultVisible
	if height > 0 {
		rows = height - 4
	}
	if maxVisible > 0 && maxVisible < rows {
		rows = maxVisible
	}
	if rows < minVisible {
		rows = minVisible
	}
	return rows
}

func columnWidths(items []Item) []int {
	var widths []int
	for _, item := range items {
		for i, cell := range item.Cells {
			for len(widths) <= i {
				widths = append(widths, 0)
			}
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

// selected is the index into the caller's items, or -1 when the filter matches
// nothing.
// resize re-derives the row window after a terminal size change, so a block
// drawn into a shrunken terminal keeps fitting on screen.
func (l *list) resize(maxVisible, height int) {
	rows := visibleRows(maxVisible, height)
	if rows == l.visible {
		return
	}
	l.visible = rows
	l.scrollToCursor()
}

func (l *list) selected() int {
	if l.cursor < 0 || l.cursor >= len(l.matches) {
		return -1
	}
	return l.matches[l.cursor]
}

func (l *list) apply(k key) action {
	switch k.kind {
	case keyEnter:
		if l.selected() < 0 {
			return actionNone
		}
		return actionSelect
	case keyAbort:
		return actionAbort
	case keyUp:
		l.move(-1)
	case keyDown:
		l.move(1)
	case keyPageUp:
		l.move(-l.visible)
	case keyPageDown:
		l.move(l.visible)
	case keyHome:
		l.move(-len(l.matches))
	case keyEnd:
		l.move(len(l.matches))
	case keyBackspace:
		if len(l.filter) == 0 {
			// Backspace on an empty filter is the second way out, for
			// terminals that swallow Esc.
			return actionAbort
		}
		l.setFilter(l.filter[:len(l.filter)-1])
	case keyClearFilter:
		if len(l.filter) > 0 {
			l.setFilter(nil)
		}
	case keyRune:
		l.setFilter(append(l.filter, k.rune))
	}
	return actionNone
}

func (l *list) move(delta int) {
	if len(l.matches) == 0 {
		l.cursor = 0
		return
	}
	l.cursor += delta
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.cursor >= len(l.matches) {
		l.cursor = len(l.matches) - 1
	}
	l.scrollToCursor()
}

// setFilter recomputes the matching subset, keeping the cursor on the item it
// pointed at when that item still matches — narrowing a filter must not move
// the selection to an unrelated row.
func (l *list) setFilter(filter []rune) {
	previous := l.selected()
	l.filter = filter
	terms := strings.Fields(strings.ToLower(string(filter)))
	l.matches = l.matches[:0]
	for i, item := range l.items {
		if matchesTerms(item.filterText(), terms) {
			l.matches = append(l.matches, i)
		}
	}
	l.cursor = 0
	for i, index := range l.matches {
		if index == previous {
			l.cursor = i
			break
		}
	}
	l.scrollToCursor()
}

// matchesTerms requires every whitespace-separated term to appear in text, so
// typing more words narrows instead of widening.
func matchesTerms(text string, terms []string) bool {
	lowered := strings.ToLower(text)
	for _, term := range terms {
		if !strings.Contains(lowered, term) {
			return false
		}
	}
	return true
}

func (l *list) scrollToCursor() {
	if l.cursor < l.top {
		l.top = l.cursor
	}
	if l.cursor >= l.top+l.visible {
		l.top = l.cursor - l.visible + 1
	}
	if limit := len(l.matches) - l.visible; l.top > limit {
		l.top = limit
	}
	if l.top < 0 {
		l.top = 0
	}
}

const (
	// minColumnWidth is how narrow a shrunken column may get before another one
	// is shortened instead: below this nothing readable is left.
	minColumnWidth = 8
	// stubColumnWidth is the floor of a PriorityLow column. It is the wider one
	// because such a column is shortened first and would otherwise end up the
	// narrowest thing on the row: a branch name has to stay readable well past
	// its first segment — "feat/tree-sitter-parse-cap-and-…" tells the branches
	// of a feature stack apart, "feat/tree-sitter…" does not — even though the
	// title line spells the selected one out in full.
	stubColumnWidth = 35
)

// columnMin is how far a column may be shrunk, which depends on how early it
// is chosen: the column that gives up its width first keeps a wider floor.
func columnMin(priorities []int, column int) int {
	if columnPriority(priorities, column) == PriorityLow {
		return stubColumnWidth
	}
	return minColumnWidth
}

// fitWidths shrinks columns until a row fits the terminal. Narrow, fixed
// columns (a marker, an age) are already at or below the floor and keep their
// width. Of the rest, the least important column gives up its width first —
// a branch name whose full value the title line spells out anyway, before the
// message that says what the branch is about — and columns of equal importance
// level down together rather than one of them collapsing next to another that
// kept everything.
func fitWidths(widths, priorities []int, width int) []int {
	if len(widths) == 0 {
		return nil
	}
	fitted := append([]int(nil), widths...)
	// The cursor prefix plus the gap between each pair of columns is space no
	// column can use.
	overhead := 2 + (len(fitted)-1)*len(columnGap)
	for {
		total := overhead
		for _, w := range fitted {
			total += w
		}
		if total <= width {
			return fitted
		}
		target := shrinkTarget(fitted, priorities)
		if target < 0 {
			// Nothing left to give; the line truncation takes over.
			return fitted
		}
		reduce := total - width
		if floor := shrinkFloor(fitted, priorities, target); fitted[target]-reduce < floor {
			reduce = fitted[target] - floor
		}
		if reduce <= 0 {
			reduce = 1
		}
		fitted[target] -= reduce
	}
}

// shrinkTarget picks the column to shorten next: the least important one that
// still has width to give, and the widest among those of equal importance.
func shrinkTarget(fitted, priorities []int) int {
	target := -1
	for i, w := range fitted {
		if w <= columnMin(priorities, i) {
			continue
		}
		if target < 0 {
			target = i
			continue
		}
		switch {
		case columnPriority(priorities, i) < columnPriority(priorities, target):
			target = i
		case columnPriority(priorities, i) == columnPriority(priorities, target) && w > fitted[target]:
			target = i
		}
	}
	return target
}

// shrinkFloor is how far the target may fall in one step: down to the widest
// other column of the same importance — including one of equal width, so a
// pair levels a cell at a time instead of one of them collapsing — and never
// below that column's own floor. A column that is the only one of its class
// goes straight to the floor, which is what lets an unimportant column give up
// its width before a vital one is touched at all.
func shrinkFloor(fitted, priorities []int, target int) int {
	floor := columnMin(priorities, target)
	for i, w := range fitted {
		if i == target || columnPriority(priorities, i) != columnPriority(priorities, target) {
			continue
		}
		if w <= fitted[target] && w > floor {
			floor = w
		}
	}
	return floor
}

// columnPriority is a column's declared importance, PriorityNormal for every
// column the caller did not rank.
func columnPriority(priorities []int, column int) int {
	if column < len(priorities) {
		return priorities[column]
	}
	return PriorityNormal
}

// render returns the block to draw, one string per terminal line, already
// truncated to width. The caller owns cursor movement and erasing.
func (l *list) render(width int) []string {
	if width <= 0 {
		width = 80
	}
	widths := fitWidths(l.widths, l.priorities, width)
	lines := make([]string, 0, l.visible+3)
	lines = append(lines, l.renderTitle(width))
	end := min(l.top+l.visible, len(l.matches))
	for i := l.top; i < end; i++ {
		lines = append(lines, l.renderRow(i, width, widths))
	}
	if len(l.matches) == 0 {
		lines = append(lines, l.styled(styleNoMatch, truncate("  no match", width)))
	}
	lines = append(lines, l.styled(stylePosition, truncate(l.filterLine(), width)))
	lines = append(lines, l.styled(styleHint, hint(width)))
	return lines
}

const (
	longHint  = "↑/↓ move · PgUp/PgDn page · type to filter · Ctrl-U clear · Enter select · Esc abort"
	shortHint = "↑/↓ move · type to filter · Enter select · Esc abort"
)

// hint states the keys, in as much detail as the terminal has room for: a
// truncated key list is worse than a shorter complete one.
func hint(width int) string {
	if displayWidth(longHint) <= width {
		return longHint
	}
	return truncate(shortHint, width)
}

func (l *list) filterLine() string {
	position := fmt.Sprintf("%d of %d", l.cursor+1, len(l.matches))
	if len(l.matches) == 0 {
		position = fmt.Sprintf("0 of %d", len(l.items))
	}
	if len(l.filter) == 0 {
		return position
	}
	return position + " · filter: " + string(l.filter)
}

// renderTitle draws the title plus the selected row's Detail: the full value
// behind a column the row had to shorten.
func (l *list) renderTitle(width int) string {
	segments := []segment{{l.title, styleTitle}}
	if index := l.selected(); index >= 0 && l.items[index].Detail != "" {
		segments = append(segments, segment{" ", l.detailStyle})
		// The detail is the full value a column had to shorten — a ref, in
		// every picker that sets one — so its separators fade like a ref's.
		segments = append(segments, refSegments(l.items[index].Detail, l.detailStyle)...)
	}
	return l.emitRow(segments, width, false)
}

// firstStyle returns the caller's style, or the fallback when it gave none.
func firstStyle(style, fallback string) string {
	if style != "" {
		return style
	}
	return fallback
}

// segment is one styled piece of a row. Rows are assembled from segments (not
// from one string that is styled afterwards) because each column carries its
// own colour and the cursor row carries a background across all of them.
type segment struct {
	text  string
	style string
}

func (l *list) renderRow(position, width int, widths []int) string {
	item := l.items[l.matches[position]]
	cursor := position == l.cursor
	segments := make([]segment, 0, 2*len(item.Cells)+2)
	if cursor {
		segments = append(segments, segment{cursorMarker, styleCursorMark})
	} else {
		segments = append(segments, segment{strings.Repeat(" ", displayWidth(cursorMarker)), ""})
	}
	for i, cell := range item.Cells {
		if i > 0 {
			segments = append(segments, segment{gapBefore(widths, i), ""})
		}
		text := cell
		if i < len(item.Cells)-1 {
			// The last column needs no padding: nothing follows it but the
			// note, which reads better next to the text than after a gap.
			text = pad(truncate(cell, widths[i]), widths[i])
		}
		segments = append(segments, cellSegments(l.columnKind(i), text, l.cellStyle(item, i))...)
	}
	return l.emitRow(segments, width, cursor)
}

// gapBefore is the space between column i-1 and column i: a single space after
// a one-cell marker column, the normal gap everywhere else. A marker belongs to
// the row it marks, so pushing the first real column two cells away from it
// only widens the empty lane on every unmarked row.
func gapBefore(widths []int, column int) string {
	if column > 0 && column-1 < len(widths) && widths[column-1] <= 1 {
		return " "
	}
	return columnGap
}

// emitRow renders segments within width cells, truncating where the row runs
// out of room and — on the cursor row — filling the rest, so the highlight
// spans the whole line instead of ending mid-row.
func (l *list) emitRow(segments []segment, width int, cursor bool) string {
	var b strings.Builder
	left := width
	for _, part := range segments {
		if left <= 0 {
			break
		}
		text := truncate(part.text, left)
		left -= displayWidth(text)
		b.WriteString(l.paint(text, part.style, cursor))
	}
	if cursor && left > 0 {
		b.WriteString(l.paint(strings.Repeat(" ", left), "", cursor))
	}
	return b.String()
}

// columnKind is what a column holds, KindPlain for every column the caller did
// not declare.
func (l *list) columnKind(column int) ColumnKind {
	if column < len(l.kinds) {
		return l.kinds[column]
	}
	return KindPlain
}

// cellStyle is the row's own colour for a column when it set one, else the
// column's colour.
func (l *list) cellStyle(item Item, column int) string {
	if column < len(item.CellStyles) && item.CellStyles[column] != "" {
		return item.CellStyles[column]
	}
	if column < len(l.cellStyles) {
		return l.cellStyles[column]
	}
	return ""
}

// paint wraps one segment in its SGR codes. On the cursor row the highlight
// background joins the segment's own code in a single sequence: a nested reset
// would end the background for the rest of the row.
func (l *list) paint(text, style string, cursor bool) string {
	if !l.color || text == "" {
		return text
	}
	codes := style
	if strings.TrimSpace(text) == "" {
		// Blank padding needs no foreground; on the cursor row the background
		// still has to cover it, everywhere else it stays bare.
		codes = ""
	}
	if cursor {
		if codes == "" {
			codes = styleCursorRow
		} else {
			codes = styleCursorRow + ";" + codes
		}
	}
	if codes == "" {
		return text
	}
	return "\x1b[" + codes + "m" + text + attrReset
}

// styled wraps a whole line in one SGR code, or returns it unchanged when
// colour is off (NO_COLOR, or a non-terminal writer in tests).
func (l *list) styled(style, text string) string {
	if !l.color || text == "" || style == "" {
		return text
	}
	return "\x1b[" + style + "m" + text + attrReset
}
