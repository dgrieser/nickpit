package pick

import (
	"fmt"
	"strconv"
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
	// styleViewActive picks the current scope out of the status line: the same
	// pale lavender the text column wears, bold, against the dark grey of the
	// scopes the user could switch to.
	styleViewActive = "1;38;5;189"
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
	// StyleCaveat marks a qualifier such as a draft request, or a value that
	// needs a second look (progressColorWarnYellow).
	StyleCaveat = "38;5;221"
	// StyleError marks a value that reports a failure — a verdict of incorrect,
	// the same thing the review output badges red (progressColorErrorRed).
	StyleError = "38;5;203"
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
	// views are the scopes the list can show; a plain list is one view. items,
	// title and widths always belong to the one on screen.
	views []View
	view  int
	items []Item
	// title is the current view's title and fallback the one Options supplied,
	// used by every view that names none.
	title    string
	fallback string
	widths   []int
	// cellStyles holds the caller's per-column SGR codes, indexed like Cells.
	cellStyles  []string
	kinds       []ColumnKind
	detailStyle string
	// priorities ranks the columns by how long they keep their width.
	priorities []int
	color      bool
	visible    int
	// rangeMode turns the list into a range selection, and unit names what its
	// summary counts.
	rangeMode bool
	unit      string
	// keyLine replaces the hint when the caller supplied one, and
	// dismissOnBackspace makes backspace a way out of a nested prompt.
	keyLine            string
	dismissOnBackspace bool
	// loaded marks the scopes whose Load has run, failures marks the ones it
	// failed for, and pending says a scope is waiting to be loaded — the draw
	// loop runs it after the "loading" line is on screen, never before.
	loaded   []bool
	failures []string
	pending  bool

	// fields are the searchable parts of every item of the current view,
	// lowered once at load so a keystroke only compares.
	fields  [][]MatchField
	filter  []rune
	matches []int
	cursor  int
	// anchor is the row an open range started on, -1 while none is open.
	anchor int
	top    int
}

func newList(opts Options, height int, color bool) *list {
	views := opts.Views
	if len(views) == 0 {
		views = []View{{Items: opts.Items}}
	}
	l := &list{
		views:              views,
		fallback:           opts.Title,
		cellStyles:         opts.CellStyles,
		kinds:              opts.ColumnKinds,
		detailStyle:        firstNonEmpty(opts.DetailStyle, StyleDetail),
		priorities:         opts.ColumnPriority,
		rangeMode:          opts.Range,
		unit:               opts.RangeUnit,
		keyLine:            opts.Hint,
		dismissOnBackspace: opts.DismissOnBackspace,
		anchor:             -1,
		color:              color,
	}
	l.loaded = make([]bool, len(views))
	l.failures = make([]string, len(views))
	l.view = opts.View
	if l.view < 0 || l.view >= len(l.views) {
		l.view = 0
	}
	l.pending = l.needsLoad()
	l.loadView("")
	l.visible = visibleRows(opts.MaxVisible, height, l.chrome())
	l.cursor = opts.Initial
	if l.cursor < 0 || l.cursor >= len(l.matches) {
		l.cursor = 0
	}
	l.scrollToCursor()
	return l
}

// loadView makes the current scope the one on screen: its rows, its title and
// its column widths. The filter carries over — the same typed text in a wider
// scope is what switching is for — and the cursor lands on the row carrying
// key, which the caller read before the scope changed, or at the top when that
// row is not in this scope.
func (l *list) loadView(key string) {
	view := l.views[l.view]
	l.items = view.Items
	l.title = firstNonEmpty(view.Title, l.fallback)
	l.widths = columnWidths(view.Items)
	l.fields = make([][]MatchField, len(view.Items))
	for i, item := range view.Items {
		l.fields[i] = item.fields()
	}
	l.cursor, l.top = 0, 0
	l.applyFilter()
	if key == "" {
		return
	}
	for position, index := range l.matches {
		if l.items[index].Key == key {
			l.cursor = position
			break
		}
	}
	l.scrollToCursor()
}

// switchView moves delta scopes along, wrapping at both ends. It does nothing
// while a range is open (the visible set is frozen there) or when there is only
// one scope.
func (l *list) switchView(delta int) {
	if len(l.views) < 2 || l.anchor >= 0 {
		return
	}
	key := ""
	if index := l.selected(); index >= 0 {
		key = l.items[index].Key
	}
	l.view = (l.view + delta + len(l.views)) % len(l.views)
	l.pending = l.needsLoad()
	l.loadView(key)
}

// needsLoad reports whether the current scope still has to fetch its rows.
func (l *list) needsLoad() bool {
	return l.views[l.view].Load != nil && !l.loaded[l.view]
}

// runPendingLoad fetches the current scope's rows. The caller runs it between
// two draws, so the list is already showing the scope (and its loading line)
// when the fetch blocks.
func (l *list) runPendingLoad() {
	l.pending = false
	view := l.views[l.view]
	if view.Load == nil || l.loaded[l.view] {
		return
	}
	l.loaded[l.view] = true
	items, err := view.Load()
	if err != nil {
		// Whatever was gathered before it failed is still worth showing, and
		// the line says what is missing. With nothing gathered the rows the
		// scope already had (a local listing the fetch was to be merged into)
		// stay untouched.
		l.failures[l.view] = err.Error()
		if len(items) == 0 {
			return
		}
	}
	l.views[l.view].Items = items
	l.loadView("")
}

// chrome is how many lines the block spends on something other than rows: the
// title, the status line (position, scopes and filter in one) and the hint
// line.
func (l *list) chrome() int { return 3 }

// visibleRows leaves room for the lines around the rows — the title, the
// filter line, the hint line and, in a multi-scope list, the switcher — plus
// one spare, so the whole block fits on screen: a block taller than the
// terminal would scroll, and the cursor-up redraw would then repaint the wrong
// lines. chrome is how many of those lines this list draws.
func visibleRows(maxVisible, height, chrome int) int {
	rows := defaultVisible
	if height > 0 {
		rows = height - chrome - 1
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
	rows := visibleRows(maxVisible, height, l.chrome())
	if rows == l.visible {
		return
	}
	l.visible = rows
	l.scrollToCursor()
}

// selected is the index into the caller's items, or -1 when the filter matches
// nothing.
func (l *list) selected() int {
	if l.cursor < 0 || l.cursor >= len(l.matches) {
		return -1
	}
	return l.matches[l.cursor]
}

// selectedRange is the pair of item indexes the list ends on, ordered as the
// rows are (first is the higher row). Outside range mode both are the cursor.
func (l *list) selectedRange() (int, int) {
	first := l.selected()
	if !l.rangeMode || l.anchor < 0 {
		return first, first
	}
	low, high := min(l.anchor, l.cursor), max(l.anchor, l.cursor)
	return l.matches[low], l.matches[high]
}

// inSpan reports whether a row lies inside the open range, which is what the
// highlight covers while the second end is being chosen.
func (l *list) inSpan(position int) bool {
	if l.anchor < 0 {
		return position == l.cursor
	}
	return position >= min(l.anchor, l.cursor) && position <= max(l.anchor, l.cursor)
}

// spanSize is how many rows the open range covers, 0 while none is open.
func (l *list) spanSize() int {
	if l.anchor < 0 {
		return 0
	}
	return max(l.anchor, l.cursor) - min(l.anchor, l.cursor) + 1
}

// indexOf finds a value in a match set, -1 when it is gone.
func indexOf(matches []int, item int) int {
	for i, candidate := range matches {
		if candidate == item {
			return i
		}
	}
	return -1
}

func (l *list) apply(k key) action {
	switch k.kind {
	case keyEnter:
		if l.selected() < 0 {
			return actionNone
		}
		if l.rangeMode && l.anchor < 0 {
			// The first Enter opens the range instead of ending the list, and
			// the filter goes with it: a row hidden between the two ends would
			// end up inside a range nobody could see.
			item := l.selected()
			l.anchor = l.cursor
			if len(l.filter) > 0 {
				l.setFilter(nil)
				l.anchor = max(indexOf(l.matches, item), 0)
				l.cursor = l.anchor
				l.scrollToCursor()
			}
			return actionNone
		}
		return actionSelect
	case keyAbort:
		if l.anchor >= 0 {
			// Esc closes an open range before it leaves the list.
			l.anchor = -1
			return actionNone
		}
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
		if l.anchor >= 0 {
			// The visible set is frozen while a range is open.
			return actionNone
		}
		if len(l.filter) == 0 {
			// Nothing left to delete. In a nested prompt that is the way back;
			// anywhere else a key pressed one time too many while clearing a
			// filter must not throw the list away.
			if l.dismissOnBackspace {
				return actionAbort
			}
			return actionNone
		}
		l.setFilter(l.filter[:len(l.filter)-1])
	case keyClearFilter:
		if l.anchor < 0 && len(l.filter) > 0 {
			l.setFilter(nil)
		}
	case keyNextView:
		l.switchView(1)
	case keyPrevView:
		l.switchView(-1)
	case keyRune:
		if l.anchor < 0 {
			l.setFilter(append(l.filter, k.rune))
		}
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
	l.applyFilter()
	for i, index := range l.matches {
		if index == previous {
			l.cursor = i
			break
		}
	}
	l.scrollToCursor()
}

// applyFilter rebuilds the matching subset of the current scope's items and
// puts the cursor at the top; callers that have a row to return to move it
// afterwards. It never reuses a match index across a scope switch, where the
// same index means a different row.
func (l *list) applyFilter() {
	terms := strings.Fields(strings.ToLower(string(l.filter)))
	l.matches = l.matches[:0]
	for i := range l.items {
		if matchesTerms(l.fields[i], terms) {
			l.matches = append(l.matches, i)
		}
	}
	l.cursor = 0
}

// matchesTerms requires every whitespace-separated term to appear in one of the
// row's fields, so typing more words narrows instead of widening. A field only
// answers terms long enough for it: too short a term simply looks elsewhere,
// which is what keeps a two-character term out of an opaque id.
func matchesTerms(fields []MatchField, terms []string) bool {
	for _, term := range terms {
		if !matchesTerm(fields, term) {
			return false
		}
	}
	return true
}

func matchesTerm(fields []MatchField, term string) bool {
	for _, field := range fields {
		if field.MinTerm > 0 && len([]rune(term)) < field.MinTerm {
			continue
		}
		if strings.Contains(field.Text, term) {
			return true
		}
	}
	return false
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
	lines := make([]string, 0, l.visible+l.chrome())
	lines = append(lines, l.renderTitle(width))
	end := min(l.top+l.visible, len(l.matches))
	for i := l.top; i < end; i++ {
		lines = append(lines, l.renderRow(i, width, widths))
	}
	if len(l.matches) == 0 {
		// An empty scope and a filter that matched nothing are different
		// things: the first is answered by switching scope, the second by
		// typing less.
		lines = append(lines, l.styled(styleNoMatch, truncate(l.emptyLine(), width)))
	}
	lines = append(lines, l.statusLine(width))
	lines = append(lines, l.styled(styleHint, l.hint(width)))
	return lines
}

// emptyLine says why there are no rows: the scope holds none (in the caller's
// own words where it supplied them), or the filter matched none of the rows it
// does hold.
func (l *list) emptyLine() string {
	if l.pending {
		if loading := l.views[l.view].Loading; loading != "" {
			return "  " + loading
		}
		return "  loading…"
	}
	if len(l.items) > 0 {
		return "  no match"
	}
	if failure := l.failures[l.view]; failure != "" {
		return "  " + failure
	}
	if empty := l.views[l.view].Empty; empty != "" {
		return "  " + empty
	}
	return "  nothing here"
}

// statusLine is the one line under the rows: where the cursor stands, the
// scopes the list has with the current one in brackets, and the filter — but
// only while something is typed, so a list nobody is filtering says nothing
// about filtering.
func (l *list) statusLine(width int) string {
	segments := []segment{{l.position(), stylePosition}}
	if len(l.views) > 1 {
		for i, view := range l.views {
			label := view.Label
			if label == "" {
				label = strconv.Itoa(i + 1)
			}
			style := styleHint
			if i == l.view {
				label = "[" + label + "]"
				style = styleViewActive
			}
			segments = append(segments, segment{" · ", StyleSeparator}, segment{label, style})
		}
	}
	if note, style := l.loadNote(); note != "" {
		segments = append(segments, segment{" · ", StyleSeparator}, segment{note, style})
	}
	if len(l.filter) > 0 {
		segments = append(segments, segment{" · ", StyleSeparator},
			segment{"filter: " + string(l.filter), stylePosition})
	}
	return l.emitRow(segments, width, false)
}

// loadNote is what the status line says about a scope that fetches its rows:
// that it is fetching them, or that it could not. A scope that loaded cleanly
// says nothing.
func (l *list) loadNote() (string, string) {
	if l.pending {
		return firstNonEmpty(l.views[l.view].Loading, "loading…"), stylePosition
	}
	if failure := l.failures[l.view]; failure != "" && len(l.items) > 0 {
		// With no rows at all the failure stands in their place instead.
		return failure, styleNoMatch
	}
	return "", ""
}

const (
	longHint  = "↑/↓ move · PgUp/PgDn page · type to filter · Ctrl-U clear · Enter select · Esc abort"
	shortHint = "↑/↓ move · type to filter · Enter select · Esc abort"
	// The range hints replace them once a range can be, or has been, opened:
	// Enter means something else there, and the filter is out of play while the
	// range is open.
	longRangeHint  = "↑/↓ move · PgUp/PgDn page · type to filter · Enter opens the range · Esc abort"
	shortRangeHint = "↑/↓ move · type to filter · Enter opens the range · Esc abort"
	longSpanHint   = "↑/↓ extend · PgUp/PgDn page · Enter selects the range · Esc drops it"
	shortSpanHint  = "↑/↓ extend · Enter selects · Esc drops the range"
	// The scope hints are appended in a list that has more than one scope.
	longScopeHint  = "Tab/←/→ scope"
	shortScopeHint = "Tab scope"
)

// hint states the keys of the state the list is in, in as much detail as the
// terminal has room for: a truncated key list is worse than a shorter complete
// one.
func (l *list) hint(width int) string {
	if l.keyLine != "" {
		return truncate(l.keyLine, width)
	}
	long, short := longHint, shortHint
	switch {
	case l.anchor >= 0:
		long, short = longSpanHint, shortSpanHint
	case l.rangeMode:
		long, short = longRangeHint, shortRangeHint
	}
	if len(l.views) > 1 && l.anchor < 0 {
		long += " · " + longScopeHint
		short += " · " + shortScopeHint
	}
	if displayWidth(long) <= width {
		return long
	}
	return truncate(short, width)
}

// position is where the cursor stands in the visible set; with nothing
// matching it counts the scope's own rows, so the line still says how much the
// filter is hiding.
func (l *list) position() string {
	if len(l.matches) == 0 {
		return fmt.Sprintf("0 of %d", len(l.items))
	}
	return fmt.Sprintf("%d of %d", l.cursor+1, len(l.matches))
}

// renderTitle draws the title plus the selected row's Detail: the full value
// behind a column the row had to shorten.
func (l *list) renderTitle(width int) string {
	segments := []segment{{l.title, styleTitle}}
	if l.anchor >= 0 {
		return l.emitRow(append(segments, l.spanSegments()...), width, false)
	}
	if index := l.selected(); index >= 0 {
		segments = append(segments, l.detailSegments(l.items[index])...)
	}
	return l.emitRow(segments, width, false)
}

// detailSegments renders what the title line says about the selected row: the
// parts of Details in their own colours, or the single Detail value, each
// behind the separator the list uses everywhere else.
func (l *list) detailSegments(item Item) []segment {
	if len(item.Details) == 0 {
		if item.Detail == "" {
			return nil
		}
		// The detail is the full value a column had to shorten — a ref, in
		// every picker that sets one — so its separators fade like a ref's.
		return append([]segment{{" ", l.detailStyle}}, refSegments(item.Detail, l.detailStyle)...)
	}
	var segments []segment
	for _, field := range item.Details {
		if field.Text == "" {
			continue
		}
		segments = append(segments, segment{" · ", StyleSeparator})
		// A field's colour is exactly what the caller set: an empty one leaves
		// the value in the terminal's own colour, the way an unstyled column
		// shows it.
		segments = append(segments, cellSegments(field.Kind, field.Text, field.Style)...)
	}
	return segments
}

// spanSegments summarises an open range after the title: how many rows it
// covers, and the details of its two ends, so what is about to be selected is
// readable without counting rows.
func (l *list) spanSegments() []segment {
	size := l.spanSize()
	count := strconv.Itoa(size)
	if l.unit != "" {
		count += " " + l.unit
		if size != 1 {
			count += "s"
		}
	}
	segments := []segment{{" " + count, stylePosition}}
	first, last := l.selectedRange()
	from, to := l.items[last].Detail, l.items[first].Detail
	if from == "" || to == "" {
		return segments
	}
	// Oldest first, the way a range is written: "base..head".
	segments = append(segments, segment{" · ", StyleSeparator})
	segments = append(segments, refSegments(from, l.detailStyle)...)
	segments = append(segments, segment{"..", StyleSeparator})
	segments = append(segments, refSegments(to, l.detailStyle)...)
	return segments
}

// firstNonEmpty returns the caller's value — a style, a title — or the
// fallback when it gave none.
func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
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
	// The highlight covers every row of an open range, so the span is visible
	// as one block; the marker stays on the row the keys move.
	highlight := l.inSpan(position)
	segments := make([]segment, 0, 2*len(item.Cells)+2)
	if position == l.cursor {
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
	return l.emitRow(segments, width, highlight)
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
