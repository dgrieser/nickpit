// Package reviewmd renders a nickpit review as platform-neutral markdown for
// publishing back to a pull/merge request, and carries the shared pieces both
// SCM publishers need: hidden idempotency markers, untrusted-text sanitization,
// and the diff hunk line-walk used to anchor inline comments. The markdown is
// identical across GitLab and GitHub; only the API call that posts it differs.
package reviewmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/dedupe"
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

// FingerprintPrefix opens the per-finding carrier marker; its base64 payload is
// an fpPayload. Re-runs decode it from existing comments to recover prior
// findings and skip the ones already posted.
const FingerprintPrefix = MarkerOpen + "fp:"

// fpPayload is the structured finding identity carried in each finding comment.
// id is used only for exact same-run matching; f and t drive cross-run fuzzy
// matching (dedupe.Compare on file + title). Line range and body are deliberately
// left out of cross-run identity because they drift between runs; the optional
// s/e/b fields exist so a later version can carry them without a grammar change.
type fpPayload struct {
	ID string `json:"id"`
	F  string `json:"f"`
	T  string `json:"t"`
	S  int    `json:"s,omitempty"`
	E  int    `json:"e,omitempty"`
	B  string `json:"b,omitempty"`
}

// FingerprintMarker renders the hidden carrier marker for a finding. displayTitle
// is the title actually shown in the comment (FindingDisplay output), so the next
// run compares like-for-like. base64.StdEncoding keeps the payload free of "-->"
// and the MarkerOpen token, so a payload can neither close the marker early nor
// be forged from untrusted finding text.
func FingerprintMarker(finding model.Finding, displayTitle string) string {
	payload, err := json.Marshal(fpPayload{ID: finding.ID, F: finding.CodeLocation.FilePath, T: displayTitle})
	if err != nil {
		return ""
	}
	return FingerprintPrefix + base64.StdEncoding.EncodeToString(payload) + " -->"
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

// CollectPriorFindings scans body for finding carrier markers (FingerprintPrefix)
// and appends a reconstructed finding shell (id, file path, displayed title) for
// each. A marker that fails to decode is skipped, so one corrupt comment can
// never abort dedup or the publish.
func CollectPriorFindings(body string, out *[]model.Finding) {
	rest := body
	for {
		i := strings.Index(rest, FingerprintPrefix)
		if i < 0 {
			return
		}
		rest = rest[i+len(FingerprintPrefix):]
		j := strings.Index(rest, "-->")
		if j < 0 {
			return
		}
		raw := strings.TrimSpace(rest[:j])
		rest = rest[j+3:]
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			continue
		}
		var p fpPayload
		if err := json.Unmarshal(decoded, &p); err != nil {
			continue
		}
		*out = append(*out, model.Finding{
			ID:           p.ID,
			Title:        p.T,
			Body:         p.B,
			CodeLocation: model.CodeLocation{FilePath: p.F, LineRange: model.LineRange{Start: p.S, End: p.E}},
		})
	}
}

// Priors holds what a prior run left on a pull/merge request: the raw markers
// (for the exact summary-marker check) and the reconstructed finding shells (for
// per-finding dedup). Both SCM publishers build it from the existing comments.
type Priors struct {
	Markers  map[string]struct{}
	Findings []model.Finding
}

// ScanComment folds one existing comment body into p, collecting both its markers
// and its finding fingerprints.
func ScanComment(body string, p *Priors) {
	if p.Markers == nil {
		p.Markers = map[string]struct{}{}
	}
	CollectMarkers(body, p.Markers)
	CollectPriorFindings(body, &p.Findings)
}

// AlreadyPosted reports whether finding was already published in p. A finding
// from the same run matches exactly on its id; across runs, identity is the
// deterministic file+title fuzzy match at dedupe.Duplicate (cross-file pairs are
// capped below Duplicate by dedupe.Compare, so a different file never matches).
// displayTitle is the title shown in the comment so the comparison is
// like-for-like with the title each prior carries. Two distinct same-file
// findings with ~identical displayed titles can collide; the Duplicate floor
// keeps that rare, and a missed match only reposts a comment, never drops a
// finding.
func AlreadyPosted(finding model.Finding, displayTitle string, p Priors) bool {
	for i := range p.Findings {
		if p.Findings[i].ID != "" && p.Findings[i].ID == finding.ID {
			return true
		}
	}
	probe := finding
	probe.Title = displayTitle
	idx, _ := dedupe.FindBest(probe, p.Findings, dedupe.Duplicate)
	return idx >= 0
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
	s = strings.ReplaceAll(s, "\r\n", "\n")
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

// ConfidencePercent renders a 0..1 confidence score as "(NN% confidence)".
func ConfidencePercent(score float64) string {
	return fmt.Sprintf("(%.0f%% confidence)", score*100)
}

// ConfidenceLine renders a 0..1 confidence score as an italic percentage.
func ConfidenceLine(score float64) string {
	return "_" + ConfidencePercent(score) + "_"
}

// CorrectnessName maps the overall verdict to its badge name. The verdict enum
// is "patch is correct" / "patch is incorrect"; anything containing
// "incorrect" maps to "incorrect", else "correct".
func CorrectnessName(correctness string) string {
	if strings.Contains(strings.ToLower(correctness), "incorrect") {
		return "incorrect"
	}
	return "correct"
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

// CorrectnessBadge renders the overall verdict as a badge image, mapping the
// verdict via CorrectnessName.
func (r Renderer) CorrectnessBadge(correctness string) string {
	name := CorrectnessName(correctness)
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

// FindingBody renders a finding as markdown, tagged with its FingerprintMarker. When
// locationPrefix is non-empty (the general-comment fallback used when a finding
// cannot be anchored inline) it is shown after the badge/confidence block so the
// location is still visible without an inline anchor.
func (r Renderer) FindingBody(finding model.Finding, locationPrefix string) string {
	title, body, rank, confidence := FindingDisplay(finding)
	var b strings.Builder
	b.WriteString(FingerprintMarker(finding, title))
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
