package output

import (
	"fmt"

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

// ansiBadge renders a label on a truecolor background with black text, the
// terminal equivalent of the published badge SVGs.
func ansiBadge(label string, c rgb) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;0;0;0m %s \x1b[0m", c.r, c.g, c.b, label)
}

// priorityBadge renders a priority rank badge, clamping to [0,3] like
// reviewmd.PriorityBadge so an out-of-range rank never panics.
func priorityBadge(rank int, ansi bool) string {
	if rank < 0 {
		rank = 0
	} else if rank > 3 {
		rank = 3
	}
	if ansi {
		return ansiBadge(priorityBadgeLabels[rank], priorityBadgeColors[rank])
	}
	return "[" + priorityBadgeLabels[rank] + "]"
}

// correctnessBadge renders the overall verdict badge, mapping the verdict via
// reviewmd.CorrectnessName so terminal and published badges cannot drift.
func correctnessBadge(correctness string, ansi bool) string {
	if reviewmd.CorrectnessName(correctness) == "incorrect" {
		if ansi {
			return ansiBadge("✗ INCORRECT", incorrectColor)
		}
		return "[INCORRECT]"
	}
	if ansi {
		return ansiBadge("✓ CORRECT", correctColor)
	}
	return "[CORRECT]"
}
