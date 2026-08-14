package output

import (
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

type rgb struct{ r, g, b uint8 }

// Badge colors and wording mirror the published SVG badges in assets/.
var (
	priorityBadgeColors = [4]rgb{
		{255, 7, 58},   // P0 assets/p0.svg #FF073A
		{251, 20, 139}, // P1 assets/p1.svg #FB148B
		{255, 81, 0},   // P2 assets/p2.svg #FF5100
		{255, 234, 0},  // P3 assets/p3.svg #FFEA00
	}
	priorityBadgeLabels = [4]string{"BLOCKING", "HIGH", "MEDIUM", "LOW"}

	correctColor   = rgb{0, 255, 13} // assets/correct.svg #00FF0D
	incorrectColor = rgb{255, 7, 58} // assets/incorrect.svg #FF073A
)

// badgeWidth is the fixed visible width of every badge, mirroring the uniform
// width of the published badge SVGs.
const badgeWidth = 16

// ansiBadge renders a pre-padded label on a truecolor background with black
// text, the terminal equivalent of the published badge SVGs.
func ansiBadge(label string, c rgb) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;0;0;0m%s\x1b[0m", c.r, c.g, c.b, label)
}

// ansiVerdictBadge renders a correctness badge whose trailing glyph is bold —
// SGR 1 turns the weight on and 22 turns it off again without disturbing the
// badge's fore/background, so only the glyph is emphasised.
func ansiVerdictBadge(word, glyph string, c rgb) string {
	pad := max(badgeWidth-len([]rune(word))-3, 0) // "WORD g " tail
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;0;0;0m%s%s \x1b[1m%s\x1b[22m \x1b[0m",
		c.r, c.g, c.b, strings.Repeat(" ", pad), word, glyph)
}

// centerBadgeLabel centers a label inside the fixed badge width.
func centerBadgeLabel(label string) string {
	pad := max(badgeWidth-len([]rune(label)), 0)
	left := pad / 2
	return strings.Repeat(" ", left) + label + strings.Repeat(" ", pad-left)
}

// Verdict glyphs, shared with the live dashboard's frozen headline so both mark
// a run the same way. The check mark renders as a single cell in the fonts we
// care about, while "✗" is wide or misaligned in many of them, so failure uses a
// plain ASCII "x". Both are emitted bold.
const (
	GlyphCorrect   = "✓"
	GlyphIncorrect = "x"
)

// priorityBadge renders a priority rank badge, clamping to [0,3] like
// reviewmd.PriorityBadge so an out-of-range rank never panics.
func priorityBadge(rank int, ansi bool) string {
	if rank < 0 {
		rank = 0
	} else if rank > 3 {
		rank = 3
	}
	if ansi {
		return ansiBadge(centerBadgeLabel(priorityBadgeLabels[rank]), priorityBadgeColors[rank])
	}
	return "[" + priorityBadgeLabels[rank] + "]"
}

// correctnessBadge renders the overall verdict badge, mapping the verdict via
// reviewmd.CorrectnessName so terminal and published badges cannot drift.
func correctnessBadge(correctness string, ansi bool) string {
	if reviewmd.CorrectnessName(correctness) == "incorrect" {
		if ansi {
			return ansiVerdictBadge("INCORRECT", GlyphIncorrect, incorrectColor)
		}
		return "[INCORRECT]"
	}
	if ansi {
		return ansiVerdictBadge("CORRECT", GlyphCorrect, correctColor)
	}
	return "[CORRECT]"
}
