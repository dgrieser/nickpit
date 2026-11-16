package main

import (
	"github.com/mittwald/cli-kit/pick"
	"github.com/mittwald/cli-kit/theme"
)

// The look every NickPit picker shares, on top of cli-kit's list. The cursor
// row carries a background rather than the theme's selected colours, so the
// row keeps its own column colours: the live dashboard's lavender pastel
// dimmed to 35%, the same scale a progress bar dims its unfilled half by, so a
// picker row and an agent bar belong to one family. tools/print_colors.sh
// reproduces it.
var (
	// pickCursorRow is the highlight background of the selected row.
	pickCursorRow = theme.BgRGB(69, 56, 86)
	// pickCursorMark is the ❯ in front of it: the bold white of a banner line
	// (progressColorBold + progressColorWhite).
	pickCursorMark = theme.Join(theme.Bold, theme.Fg256(255))
	// pickActiveView picks the current scope out of the status line: the pale
	// lavender the text column wears, bold.
	pickActiveView = theme.Join(theme.Bold, theme.Fg256(189))
	// pickTitle is the bold white of a banner line (progressColorBold +
	// progressColorWhite).
	pickTitle = theme.Join(theme.Bold, theme.Fg256(255))
	// pickHint recedes the key line and the scopes not shown below the
	// position (progressColorDarkGrey).
	pickHint = theme.Fg256(242)
	// pickEmpty warns that the list shows no rows: no match, nothing here,
	// still loading (progressColorWarnYellow).
	pickEmpty = theme.Fg256(221)
)

// pickKeys words the key line the way NickPit always has: Esc ends the run
// ("abort"), and the scope switch closes the line. A nested prompt keeps
// cli-kit's "Esc back".
var pickKeys = pick.KeyLabels{
	Cancel:    "Esc abort",
	View:      "Tab/←/→ scope",
	ViewShort: "Tab scope",
	ViewLast:  true,
}

// withPickerLook applies the shared look to one list. Every picker goes
// through it (selectRange and selectView call it), so no prompt can drift
// from the others. A Detail without a painter of its own is a ref in every
// NickPit picker, so its separators fade.
func withPickerLook(opts pick.Options) pick.Options {
	opts.CursorRow = pickCursorRow
	opts.CursorMark = pickCursorMark
	opts.Compact = true
	opts.Keys = pickKeys
	opts.ActiveView = pickActiveView
	opts.TitleStyle = pickTitle
	opts.HintStyle = pickHint
	opts.EmptyStyle = pickEmpty
	if opts.PaintDetail == nil {
		opts.PaintDetail = paintRef
	}
	return opts
}

// Cell styles a picker assigns to its columns. They are the 256-colour message
// palette of internal/logging/progress.go — a picker row should read like the
// progress lines of the same run — named by what the column means rather than
// by its colour. tools/print_colors.sh reproduces them.
const (
	// styleIdentifier is an MR/PR number (progressColorNumberGreen).
	styleIdentifier theme.Style = "38;5;118"
	// styleHash is a commit SHA (progressColorHashDarkGreen).
	styleHash theme.Style = "38;5;71"
	// styleText is a title, commit subject or branch tip message — the column
	// a reader actually reads, so it gets a light tone of its own: a pale
	// lavender, the one code the picker adds to the message palette.
	styleText theme.Style = "38;5;189"
	// styleAuthor is a username (progressColorTaskPink); a row overrides it
	// with the author's own colour from theme.Hashed.
	styleAuthor theme.Style = "38;5;218"
	// styleAge is a timestamp column (progressColorGrey), styleFresh the same
	// column when it just changed (progressColorBoolGreen).
	styleAge   theme.Style = "38;5;244"
	styleFresh theme.Style = "38;5;156"
	// styleCaveat marks a qualifier such as a draft request, or a value that
	// needs a second look (progressColorWarnYellow).
	styleCaveat theme.Style = "38;5;221"
	// styleError marks a value that reports a failure — a verdict of
	// incorrect, the same thing the review output badges red
	// (progressColorErrorRed).
	styleError theme.Style = "38;5;203"
	// styleDefaultRef marks the repository's default branch — the only ref
	// coloured in a branch list (progressColorBranchToAquaGreen).
	styleDefaultRef theme.Style = "38;5;48"
	// styleMark is a marker column: the ★ on the checked-out branch, and on
	// the request of the branch you are on. It is the gold a progress line
	// paints the head ref in.
	styleMark theme.Style = "38;5;214"
	// styleDetail is the light sky blue a value stands in after the bold
	// title it follows.
	styleDetail theme.Style = "38;5;117"
	// styleBaseRef and styleHeadRef name the two sides of a comparison in the
	// aqua green and gold a progress line paints "base → head" in, so a base
	// prompt and a head prompt are told apart at a glance — in the title line
	// and in the confirmation that follows.
	styleBaseRef theme.Style = "38;5;48"
	styleHeadRef theme.Style = "38;5;214"
	// styleSeparator fades the punctuation that holds a value together back:
	// the parentheses and colon of a commit prefix, and the "/" and ":" inside
	// a ref (progressColorGrey).
	styleSeparator theme.Style = "38;5;244"
	// styleNote is the light grey a trailing parenthetical recedes into
	// (progressColorLightGrey), the tone the pick confirmation line is set in.
	styleNote theme.Style = "38;5;252"
	// styleTerminalFg is the terminal's own foreground, spelled out. A column
	// without a style of its own would be drawn in the theme's receding
	// qualifier colour; one that wears this keeps the plain tone it has always
	// had (a branch name, a project). The painters treat it as no colour at
	// all, so what they paint carries no code for it.
	styleTerminalFg theme.Style = "39"
)
