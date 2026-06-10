// Package reviewmd renders a nickpit review as platform-neutral markdown for
// publishing back to a pull/merge request, and carries the shared pieces both
// SCM publishers need: hidden idempotency markers, untrusted-text sanitization,
// and the diff hunk line-walk used to anchor inline comments. The markdown is
// identical across GitLab and GitHub; only the API call that posts it differs.
package reviewmd

import (
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// DefaultAssetBaseURL is the fallback badge host used when NewRenderer is given
// an empty base URL (mirrors config.DefaultAssetBaseURL, kept here so the scm
// packages stay independent of config).
const DefaultAssetBaseURL = "https://dgrieser.github.io/nickpit/"

// MarkerOpen is the token CollectMarkers scans for; both the real markers below
// and any injected lookalike in untrusted text begin with it.
const MarkerOpen = "<!-- nickpit:"

// SummaryMarker tags the overall-verdict comment so re-runs do not repost it.
const SummaryMarker = MarkerOpen + "summary -->"

// FindingMarker tags a per-finding comment with the finding's stable ID so
// re-runs skip findings already posted.
func FindingMarker(id string) string {
	return MarkerOpen + "finding:" + id + " -->"
}

// CollectMarkers scans body for every nickpit marker (`<!-- nickpit:... -->`)
// and adds each to out, so callers can dedupe against comments posted before.
func CollectMarkers(body string, out map[string]struct{}) {
	rest := body
	for {
		i := strings.Index(rest, MarkerOpen)
		if i < 0 {
			return
		}
		rest = rest[i:]
		j := strings.Index(rest, "-->")
		if j < 0 {
			return
		}
		out[strings.TrimSpace(rest[:j+3])] = struct{}{}
		rest = rest[j+3:]
	}
}

// Sanitize prepares untrusted, LLM-generated text for posting as markdown. It
// strips terminal control characters (consistent with the terminal formatter)
// and defuses any embedded nickpit marker so untrusted content cannot inject a
// lookalike that poisons re-run dedupe: the marker's leading "<" is
// HTML-escaped, which renders as a literal "<" while breaking CollectMarkers'
// scan for MarkerOpen.
func Sanitize(s string) string {
	s = textsan.StripControl(s)
	return strings.ReplaceAll(s, MarkerOpen, "&lt;"+strings.TrimPrefix(MarkerOpen, "<"))
}

// hardBreakParagraphs appends a markdown hard break to each rendered prose line
// outside fenced code blocks, so GitHub/GitLab preserve the intended spacing.
func hardBreakParagraphs(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isFenceLine(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = strings.TrimRight(line, " \t") + "  "
	}
	return strings.Join(lines, "\n")
}

func isFenceLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func sanitizeWithHardBreaks(s string) string {
	return hardBreakParagraphs(Sanitize(s))
}

// ConfidenceLine renders a 0..1 confidence score as an italic percentage.
func ConfidenceLine(score float64) string {
	return fmt.Sprintf("_(%.0f%% confidence)_", score*100)
}

// Renderer turns review results into markdown comment bodies. It carries the
// badge host so the verdict/priority badge images resolve.
type Renderer struct {
	// assetBaseURL is the badge SVG host, always normalized to a trailing "/".
	assetBaseURL string
}

// NewRenderer normalizes a user-supplied badge host: empty falls back to
// DefaultAssetBaseURL and a trailing "/" is ensured so badge URLs concatenate.
func NewRenderer(assetBaseURL string) Renderer {
	assetBaseURL = strings.TrimSpace(assetBaseURL)
	if assetBaseURL == "" {
		assetBaseURL = DefaultAssetBaseURL
	}
	if !strings.HasSuffix(assetBaseURL, "/") {
		assetBaseURL += "/"
	}
	return Renderer{assetBaseURL: assetBaseURL}
}

// CorrectnessBadge renders the overall verdict as a badge image. The verdict
// enum is "patch is correct" / "patch is incorrect"; anything containing
// "incorrect" maps to the incorrect badge, else correct.
func (r Renderer) CorrectnessBadge(correctness string) string {
	name := "correct"
	if strings.Contains(strings.ToLower(correctness), "incorrect") {
		name = "incorrect"
	}
	return fmt.Sprintf("![%s](%s%s.svg)", name, r.assetBaseURL, name)
}

// PriorityBadge renders a priority rank as a Pn badge image, clamping to the
// [0,3] range of available SVGs so an out-of-range rank never yields a broken
// image link.
func (r Renderer) PriorityBadge(rank int) string {
	if rank < 0 {
		rank = 0
	} else if rank > 3 {
		rank = 3
	}
	return fmt.Sprintf("![P%d](%sp%d.svg)", rank, r.assetBaseURL, rank)
}

// SummaryBody renders the overall verdict comment, tagged with SummaryMarker.
func (r Renderer) SummaryBody(result *model.ReviewResult) string {
	var b strings.Builder
	b.WriteString(SummaryMarker)
	b.WriteString("\n")
	// The trailing two spaces are a markdown hard break so the badge and the
	// confidence line render stacked rather than joined into one line.
	correctness := strings.TrimSpace(result.OverallCorrectness)
	if correctness == "" {
		// No verdict to badge; fall back to plain text.
		fmt.Fprintf(&b, "**review complete**  \n%s  \n", ConfidenceLine(result.OverallConfidenceScore))
	} else {
		fmt.Fprintf(&b, "%s  \n%s  \n", r.CorrectnessBadge(correctness), ConfidenceLine(result.OverallConfidenceScore))
	}
	if explanation := Sanitize(strings.TrimSpace(result.OverallExplanation)); explanation != "" {
		b.WriteString("\n")
		b.WriteString(hardBreakParagraphs(explanation))
		b.WriteString("\n")
	}
	return b.String()
}

// FindingBody renders a finding as markdown, tagged with its FindingMarker. When
// locationPrefix is non-empty (the general-comment fallback used when a finding
// cannot be anchored inline) it is shown after the badge/confidence block so the
// location is still visible without an inline anchor.
func (r Renderer) FindingBody(finding model.Finding, locationPrefix string) string {
	title, body, rank, confidence := FindingDisplay(finding)
	var b strings.Builder
	b.WriteString(FindingMarker(finding.ID))
	b.WriteString("\n\n")
	// Trailing two spaces: markdown hard break stacking badge over confidence.
	fmt.Fprintf(&b, "%s  \n%s  \n\n", r.PriorityBadge(rank), ConfidenceLine(confidence))
	if locationPrefix != "" {
		// Hard break so the location sits on its own line above the title/body.
		b.WriteString(locationPrefix)
		b.WriteString("  \n\n")
	}
	if title != "" {
		fmt.Fprintf(&b, "### %s  \n\n", Sanitize(title))
	}
	b.WriteString(sanitizeWithHardBreaks(body))
	if len(finding.Suggestions) > 0 {
		b.WriteString("\n\n**Suggestions**  \n")
		for _, suggestion := range finding.Suggestions {
			text := strings.TrimSpace(suggestion.Body)
			if text == "" {
				continue
			}
			formatted := strings.ReplaceAll(sanitizeWithHardBreaks(text), "\n", "\n  ")
			fmt.Fprintf(&b, "\n- %s", formatted)
		}
	}
	return b.String()
}

// FindingDisplay prefers the finalized title/body/priority/confidence when a
// finalization pass produced them, else the original finding fields. The
// summarize pass (a shortened body, other fields copied from finalization) wins
// over finalization for the published comment when present.
func FindingDisplay(finding model.Finding) (title, body string, rank int, confidence float64) {
	title = finding.Title
	body = finding.Body
	confidence = finding.ConfidenceScore
	rank = model.PriorityRank(finding.Priority)
	if finding.Finalization != nil {
		if t := strings.TrimSpace(finding.Finalization.Title); t != "" {
			title = t
		}
		if bodyText := strings.TrimSpace(finding.Finalization.Body); bodyText != "" {
			body = finding.Finalization.Body
		}
		confidence = finding.Finalization.ConfidenceScore
		priority := finding.Finalization.Priority
		rank = model.PriorityRank(&priority)
	}
	if finding.Summarization != nil {
		if t := strings.TrimSpace(finding.Summarization.Title); t != "" {
			title = t
		}
		if bodyText := strings.TrimSpace(finding.Summarization.Body); bodyText != "" {
			body = finding.Summarization.Body
		}
		confidence = finding.Summarization.ConfidenceScore
		priority := finding.Summarization.Priority
		rank = model.PriorityRank(&priority)
	}
	return title, body, rank, confidence
}

// LineLoc is where a new-side line sits in the diff: its new-side number, the
// old-side cursor at that point, and whether it is an added line (new side only).
type LineLoc struct {
	OldLine int
	NewLine int
	Added   bool
}

// LocateLine walks the hunks to find the new-side line newLine, returning its
// location, or false when the line is not part of the diff.
func LocateLine(hunks []model.DiffHunk, newLine int) (LineLoc, bool) {
	for _, hunk := range hunks {
		oldCursor := hunk.OldStart
		newCursor := hunk.NewStart
		// TrimRight (not TrimSuffix) drops the trailing blank produced by the
		// per-file "diff + \n" framing so it is not mistaken for a context line.
		for raw := range strings.SplitSeq(strings.TrimRight(hunk.Content, "\n"), "\n") {
			// A hunk-body line carries a leading marker (' ', '+', '-', '\'). A
			// genuinely empty interior line has none; treat it as a blank context
			// line so the cursors stay in sync — skipping it would desync the
			// new-side number for every line that follows.
			marker := byte(' ')
			if raw != "" {
				marker = raw[0]
			}
			switch marker {
			case '+':
				if newCursor == newLine {
					return LineLoc{OldLine: oldCursor, NewLine: newCursor, Added: true}, true
				}
				newCursor++
			case '-':
				oldCursor++
			case '\\':
				// "\ No newline at end of file": no line on either side.
			default: // ' ' context, an empty line, or any unexpected prefix
				if newCursor == newLine {
					return LineLoc{OldLine: oldCursor, NewLine: newCursor}, true
				}
				oldCursor++
				newCursor++
			}
		}
	}
	return LineLoc{}, false
}
