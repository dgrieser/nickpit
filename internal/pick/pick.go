// Package pick renders a keyboard-driven single-choice list on a terminal: the
// interactive counterpart of the --id/--url/--base selectors, used when a
// command needs one merge request, pull request, branch or commit and the user
// did not name it. It draws with plain SGR attributes only (bold, dim,
// reverse), so it introduces no new colour to the terminal palette.
package pick

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// Item is one selectable row. Cells are drawn as aligned columns — a marker
// column is how a row says why it stands out — and Match overrides what
// type-to-filter matches against, by default the cells.
type Item struct {
	Cells []string
	Match string
	// CellStyles overrides Options.CellStyles for this row, indexed like Cells:
	// what makes one row's column stand out (the default branch, a person's
	// colour, a timestamp from the last hour). "" falls back to the column's
	// style.
	CellStyles []string
	// Detail is the row's full value, shown after the title while the row is
	// the selected one: what a column had to shorten (a long branch name) is
	// still readable somewhere, so the columns can stay narrow.
	Detail string
}

func (i Item) filterText() string {
	if i.Match != "" {
		return i.Match
	}
	return strings.Join(i.Cells, " ")
}

// Options configures one selection.
type Options struct {
	// Title names what is being chosen; it stays visible above the rows.
	Title string
	Items []Item
	// Initial is the row the cursor starts on. Out-of-range values start at the
	// first row.
	Initial int
	// MaxVisible caps how many rows are drawn at once; 0 derives the cap from
	// the terminal height.
	MaxVisible int
	// CellStyles colours the columns, indexed like Item.Cells: one of the
	// Style* codes, or "" to leave a column unstyled. Shorter than Cells is
	// fine — the remaining columns stay unstyled.
	CellStyles []string
	// DetailStyle colours Item.Detail in the title line; StyleDetail when empty.
	DetailStyle string
	// Range turns the list into a range selection: the first Enter opens the
	// range on the row under the cursor, moving then covers the rows between,
	// and the second Enter ends the list on both ends. Esc closes an open range
	// before it leaves the list. Use SelectRange to read the pair back.
	Range bool
	// RangeUnit names what the range covers, in the singular ("commit"), for
	// the summary the title line shows while the range is open; the picker
	// appends an "s" for any count but one.
	RangeUnit string
	// ColumnKinds says what each column holds, indexed like Item.Cells, so the
	// picker can paint inside a cell: a KindMessage column has its
	// conventional-commit prefix taken apart, a KindRef column its separators
	// faded. Unlisted columns are KindPlain.
	ColumnKinds []ColumnKind
	// ColumnPriority ranks the columns by how long they keep their width when
	// the row does not fit, indexed like Item.Cells: the lowest-ranked column
	// is shortened first and may collapse to a stub before a higher-ranked one
	// loses a cell. Unranked columns count as PriorityNormal, so a nil slice
	// makes every column equal and they level down together.
	ColumnPriority []int
}

// Column importance for Options.ColumnPriority. PriorityLow suits a column
// whose full value is available elsewhere — a branch name the title line
// spells out — and PriorityHigh the one a reader is actually scanning.
const (
	PriorityLow = iota
	PriorityNormal
	PriorityHigh
)

// authorStyles are the colours AuthorStyle hands out: distinct hues from the
// same message palette, all readable on a dark background.
var authorStyles = []string{
	"38;5;218", // progressColorTaskPink
	"38;5;116", // progressColorKeyTurquoise
	"38;5;216", // progressColorProfile — apricot
	"38;5;105", // progressColorURLPurpleBlue
	"38;5;120", // progressColorStringGreen
	"38;5;177", // progressColorSkipPurple
	"38;5;110", // progressColorMutedModel — muted blue
}

// AuthorStyle gives a person a stable colour: the same name keeps the same hue
// across rows, pickers and runs, so a list can be scanned by who wrote what.
// Names are folded to lower case, so "Alice" and "alice" are one person. An
// empty name gets no colour.
func AuthorStyle(name string) string {
	folded := strings.ToLower(strings.TrimSpace(name))
	if folded == "" {
		return ""
	}
	digest := fnv.New32a()
	// Hashing a string never fails, and Write's error is documented as nil.
	_, _ = digest.Write([]byte(folded))
	return authorStyles[digest.Sum32()%uint32(len(authorStyles))]
}

// ErrAborted reports that the list was dismissed (Esc, Ctrl-C, Ctrl-D, or
// backspace on an empty filter) without a choice. Callers turn it into their
// own "nothing selected" message rather than a failure.
var ErrAborted = errors.New("selection aborted")

// ErrNoItems reports that there was nothing to choose from; the caller knows
// what its empty list means and says so itself.
var ErrNoItems = errors.New("nothing to select")

// ErrNotATerminal reports that in or out is not a terminal, so no list can be
// drawn. Callers fall back to their non-interactive error.
var ErrNotATerminal = errors.New("not a terminal")

// Select draws the list on out, reads keys from in, and returns the index into
// opts.Items the user chose. in is switched to raw mode for the duration and
// restored before returning, including on error, and the drawn block is erased
// so the caller's own output starts on a clean line.
func Select(in, out *os.File, opts Options) (int, error) {
	first, _, err := SelectRange(in, out, opts)
	return first, err
}

// SelectRange is Select for a list that ends on two rows: with Options.Range it
// returns the ends of the chosen range as indexes into opts.Items, ordered as
// the rows are — first is the higher row, last the lower one. Without
// Options.Range both results are the single chosen row.
func SelectRange(in, out *os.File, opts Options) (int, int, error) {
	if len(opts.Items) == 0 {
		return -1, -1, ErrNoItems
	}
	if in == nil || out == nil || !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return -1, -1, ErrNotATerminal
	}
	_, height := terminalSize(out)
	_, noColor := os.LookupEnv("NO_COLOR")
	state := newList(opts, height, !noColor)

	previous, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return -1, -1, fmt.Errorf("pick: switching the terminal to raw mode: %w", err)
	}
	screen := &renderer{w: out}
	defer func() {
		// Order matters: the block is erased while the terminal is still raw
		// (so the carriage returns land), then the mode and the cursor are put
		// back exactly as they were found.
		_ = screen.clear()
		_, _ = io.WriteString(out, showCursor)
		_ = term.Restore(int(in.Fd()), previous)
	}()
	if _, err := io.WriteString(out, hideCursor); err != nil {
		return -1, -1, err
	}

	return selectFrom(state, screen, func() (int, int) { return terminalSize(out) }, opts.MaxVisible, &fileInput{file: in})
}

// input is the picker's key source: one blocking read plus the short second
// read that decides a pending ESC. A seam, so the key loop is testable without
// a terminal — where the Esc key would otherwise be unreachable.
type input interface {
	read() ([]byte, error)
	grace() []byte
}

// selectFrom drives the list until a key selects or aborts.
func selectFrom(state *list, screen *renderer, size func() (int, int), maxVisible int, source input) (int, int, error) {
	var pending []byte
	for {
		width, height := size()
		state.resize(maxVisible, height)
		if err := screen.draw(state.render(width)); err != nil {
			return -1, -1, err
		}
		chunk, readErr := source.read()
		pending = append(pending, chunk...)
		for len(pending) > 0 {
			k, consumed := decode(pending, true)
			if consumed == 0 {
				// An incomplete sequence: either a split escape sequence or the
				// Esc key itself. Give the continuation a moment to arrive, then
				// decide — without this, pressing Esc would look dead until the
				// next keypress.
				if readErr == nil {
					if extra := source.grace(); len(extra) > 0 {
						pending = append(pending, extra...)
						continue
					}
				}
				if k, consumed = decode(pending, false); consumed == 0 {
					break
				}
			}
			pending = pending[consumed:]
			switch state.apply(k) {
			case actionSelect:
				first, last := state.selectedRange()
				return first, last, nil
			case actionAbort:
				return -1, -1, ErrAborted
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return -1, -1, ErrAborted
			}
			return -1, -1, fmt.Errorf("pick: reading the terminal: %w", readErr)
		}
	}
}

// escapeGrace is how long the continuation of a split escape sequence may take
// before a pending ESC counts as the Esc key. Terminals write a sequence in one
// go, so this only matters on a link that fragments it: long enough to cover
// that, short enough that Esc still feels instant.
const escapeGrace = 50 * time.Millisecond

// fileInput reads keys from a terminal already switched to raw mode.
type fileInput struct {
	file *os.File
	buf  [64]byte
}

func (i *fileInput) read() ([]byte, error) {
	n, err := i.file.Read(i.buf[:])
	return i.buf[:n], err
}

// grace waits escapeGrace for more input and returns whatever arrived, or
// nothing on a timeout. Where the platform cannot put a deadline on a terminal
// (a Windows console, a file type the poller does not take), it returns
// immediately with nothing: the pending ESC then resolves as the Esc key, which
// is what a lone ESC at the end of a read chunk means anyway.
func (i *fileInput) grace() []byte {
	if err := i.file.SetReadDeadline(time.Now().Add(escapeGrace)); err != nil {
		return nil
	}
	// The deadline must not outlive this read: the next one waits for a
	// keypress that may be minutes away.
	defer func() { _ = i.file.SetReadDeadline(time.Time{}) }()
	n, err := i.file.Read(i.buf[:])
	if n == 0 || (err != nil && !errors.Is(err, os.ErrDeadlineExceeded)) {
		return nil
	}
	return i.buf[:n]
}

const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
)

func terminalSize(out *os.File) (int, int) {
	width, height, err := term.GetSize(int(out.Fd()))
	if err != nil || width <= 0 || height <= 0 {
		return 80, 24
	}
	return width, height
}

// renderer draws successive versions of the block in place: it moves back to
// the block's first line, rewrites every line, and erases whatever the previous
// version left below (the block shrinks when a filter narrows the list).
type renderer struct {
	w     io.Writer
	lines int
}

func (r *renderer) draw(lines []string) error {
	var b strings.Builder
	b.WriteString(r.moveToStart())
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\r\n")
		}
		b.WriteString("\x1b[2K")
		b.WriteString(line)
	}
	b.WriteString("\x1b[J")
	r.lines = len(lines)
	_, err := io.WriteString(r.w, b.String())
	return err
}

// clear erases the block, leaving the cursor where it started.
func (r *renderer) clear() error {
	if r.lines == 0 {
		return nil
	}
	out := r.moveToStart() + "\x1b[J"
	r.lines = 0
	_, err := io.WriteString(r.w, out)
	return err
}

func (r *renderer) moveToStart() string {
	if r.lines == 0 {
		return ""
	}
	if r.lines == 1 {
		return "\r"
	}
	return fmt.Sprintf("\r\x1b[%dA", r.lines-1)
}

// displayWidth is how many terminal cells s occupies. Not its rune count: a CJK
// character or an emoji takes two cells and a combining mark none, so counting
// runes would let a "truncated" row wrap — and a wrapped row desynchronizes the
// in-place redraw, which counts logical lines, from the screen.
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

func pad(s string, width int) string {
	if gap := width - displayWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// truncate shortens s to width display cells, marking the cut with an ellipsis
// the way the live progress lines do. The result can come out one cell short of
// width when a double-width rune sits on the boundary; column alignment is
// pad's job, so the missing cell is left to it rather than guessed here.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		cells := runewidth.RuneWidth(r)
		// One cell stays free for the ellipsis that marks the cut.
		if used+cells > width-1 {
			break
		}
		b.WriteRune(r)
		used += cells
	}
	return b.String() + "…"
}
