// Package reviewmd renders a nickpit review as platform-neutral markdown for
// publishing back to a pull/merge request, and carries the shared pieces both
// SCM publishers need: hidden idempotency markers, untrusted-text sanitization,
// and the diff hunk line-walk used to anchor inline comments. The markdown is
// identical across GitLab and GitHub; only the API call that posts it differs.
package reviewmd

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

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
// id is used only for exact same-run matching. Cross-run identity intentionally
// uses the coarser file + displayed-title signal because line anchors and
// generated prose drift between runs; preferring that stable key keeps repeated
// runs idempotent even though rare same-file/same-title findings can collide.
// The optional s/e/b fields exist so a later version can carry them without a
// marker grammar change, but current writers deliberately leave them empty.
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
// run compares like-for-like. Location and body are intentionally omitted from
// the marker's cross-run key; see fpPayload for the idempotency tradeoff.
// base64.StdEncoding keeps the payload free of "-->" and the MarkerOpen token,
// so a payload can neither close the marker early nor be forged from untrusted
// finding text.
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
	if out == nil {
		return
	}
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

// ReviewMarkerPrefix opens the summary-note carrier that holds the overall
// verdict and identifying metadata for one review run. Its payload is a gzipped,
// base64-encoded reviewEnvelope. Grouped with the per-finding carriers by the
// shared review id, it lets a later reader (e.g. the discussion agent)
// reconstruct the full ReviewResult straight from the MR/PR notes.
const ReviewMarkerPrefix = MarkerOpen + "review:"

// FindingMarkerPrefix opens the per-finding carrier that holds one complete
// model.Finding (body, suggestions, verification, finalization, summarization).
// Its payload is a gzipped, base64-encoded findingEnvelope.
const FindingMarkerPrefix = MarkerOpen + "finding:"

// maxCarrierDecodedBytes caps how much a single carrier payload may expand to
// when decompressed. Carrier markers are read from arbitrary MR/PR comments, so
// a hostile commenter could craft a small gzip payload that expands to gigabytes
// (a zip bomb). A finding envelope is at most a few hundred KB even for a large
// finding, so this bound is far above any legitimate payload while stopping the
// decompression from exhausting memory.
const maxCarrierDecodedBytes = 8 << 20 // 8 MiB

// maxCarrierTotalDecodedBytes bounds the AGGREGATE decompressed bytes one
// comment body may cause across all its carrier markers, and
// maxCarriersPerBody bounds their count. Without these a commenter could pack
// many small envelopes that each expand just below the per-marker cap and
// exhaust memory in one scan.
const (
	maxCarrierTotalDecodedBytes = 16 << 20 // 16 MiB
	maxCarriersPerBody          = 256
)

// carrierBudget tracks the aggregate decompression work spent on one comment
// body so hostile marker floods stop scanning instead of exhausting memory.
type carrierBudget struct {
	bytes   int
	markers int
}

func (b *carrierBudget) allow() bool {
	return b.bytes < maxCarrierTotalDecodedBytes && b.markers < maxCarriersPerBody
}

func (b *carrierBudget) spend(decodedBytes int) {
	b.bytes += decodedBytes
	b.markers++
}

// ReviewEnvelope is the summary-note carrier payload: the overall verdict plus
// the source identity needed to recreate the diff. Findings are carried
// separately, one per FindingEnvelope, so no single note grows unbounded.
// ReviewResult.BaseURL is deliberately NOT carried: it is the LLM endpoint —
// potentially a private hostname or a URL with credentials — which anyone able
// to view the MR/PR could read out of the hidden comment, and reassembly never
// needs it (the SCM URL comes from the profile at chat time).
type ReviewEnvelope struct {
	ReviewID               string    `json:"rid"`
	CreatedAt              time.Time `json:"at,omitzero"`
	OverallCorrectness     string    `json:"correctness,omitempty"`
	OverallExplanation     string    `json:"explanation,omitempty"`
	OverallConfidenceScore float64   `json:"confidence,omitempty"`
	Repo                   string    `json:"repo,omitempty"`
	Mode                   string    `json:"mode,omitempty"`
	Identifier             int       `json:"identifier,omitempty"`
	BaseRef                string    `json:"base_ref,omitempty"`
	HeadRef                string    `json:"head_ref,omitempty"`
	Model                  string    `json:"model,omitempty"`
}

// FindingEnvelope is the per-finding carrier payload: the review id it belongs to
// and one complete finding.
type FindingEnvelope struct {
	ReviewID string        `json:"rid"`
	Finding  model.Finding `json:"finding"`
}

// ReviewMarker renders the hidden summary-note carrier for result. It returns ""
// when result has no review id (nothing to group by). gzip keeps large payloads
// inside the platform note-size limits; base64.StdEncoding keeps the payload free
// of "-->" and the MarkerOpen token, so it can neither close the marker early nor
// be forged from untrusted text.
func ReviewMarker(result *model.ReviewResult) string {
	marker, _ := reviewMarkerWithSize(result)
	return marker
}

// reviewMarkerWithSize is ReviewMarker plus the payload's decoded (raw JSON)
// size, for the carrier chunking budget.
func reviewMarkerWithSize(result *model.ReviewResult) (string, int) {
	if result == nil || result.ReviewID == "" {
		return "", 0
	}
	return encodeMarker(ReviewMarkerPrefix, ReviewEnvelope{
		ReviewID:               result.ReviewID,
		CreatedAt:              result.CreatedAt,
		OverallCorrectness:     result.OverallCorrectness,
		OverallExplanation:     result.OverallExplanation,
		OverallConfidenceScore: result.OverallConfidenceScore,
		Repo:                   result.Repo,
		Mode:                   result.Mode,
		Identifier:             result.Identifier,
		BaseRef:                result.BaseRef,
		HeadRef:                result.HeadRef,
		Model:                  result.Model,
	})
}

// FindingMarker renders the hidden per-finding carrier for finding under review
// id reviewID. It returns "" when reviewID is empty.
func FindingMarker(reviewID string, finding model.Finding) string {
	marker, _ := findingMarkerWithSize(reviewID, finding)
	return marker
}

// findingMarkerWithSize is FindingMarker plus the payload's decoded (raw JSON)
// size, which chunked carrier publishing needs to stay inside the reader's
// per-body decompression budget.
func findingMarkerWithSize(reviewID string, finding model.Finding) (string, int) {
	if reviewID == "" {
		return "", 0
	}
	return encodeMarker(FindingMarkerPrefix, FindingEnvelope{ReviewID: reviewID, Finding: finding})
}

// encodeMarker gzips and base64-encodes payload into a hidden marker opened by
// prefix, also returning the payload's decoded (pre-compression) size. A
// marshal/compress failure yields "" so a publish never aborts on it.
func encodeMarker(prefix string, payload any) (string, int) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", 0
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return "", 0
	}
	if err := zw.Close(); err != nil {
		return "", 0
	}
	return prefix + base64.StdEncoding.EncodeToString(buf.Bytes()) + " -->", len(raw)
}

// decodeMarker reverses encodeMarker into out. decodedBytes reports the bytes
// actually decompressed (spent even when the payload turns out not to be valid
// JSON), so callers can budget aggregate decompression work across a body. It
// reports ok=false on any decode failure so one corrupt carrier can never abort
// a scan.
func decodeMarker(raw string, out any) (decodedBytes int, ok bool) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return 0, false
	}
	defer func() { _ = zr.Close() }()
	// Bound decompression to defend against zip-bomb payloads in untrusted
	// comments: read one byte past the cap and reject anything that reaches it.
	decoded, err := io.ReadAll(io.LimitReader(zr, maxCarrierDecodedBytes+1))
	if err != nil {
		return len(decoded), false
	}
	if len(decoded) > maxCarrierDecodedBytes {
		return len(decoded), false
	}
	return len(decoded), json.Unmarshal(decoded, out) == nil
}

// scanMarkers walks body for every carrier opened by prefix, handing each raw
// (undecoded) payload to fn until fn returns false. Payloads cannot contain
// "-->" (base64 has no '-'), so the close scan is unambiguous.
func scanMarkers(body, prefix string, fn func(raw string) bool) {
	rest := body
	for {
		i := strings.Index(rest, prefix)
		if i < 0 {
			return
		}
		rest = rest[i+len(prefix):]
		j := strings.Index(rest, "-->")
		if j < 0 {
			return
		}
		if !fn(strings.TrimSpace(rest[:j])) {
			return
		}
		rest = rest[j+3:]
	}
}

// CollectReviewEnvelopes decodes the review carriers found in body, bounded by
// the per-body carrier budget.
func CollectReviewEnvelopes(body string) []ReviewEnvelope {
	var out []ReviewEnvelope
	budget := &carrierBudget{}
	scanMarkers(body, ReviewMarkerPrefix, func(raw string) bool {
		if !budget.allow() {
			return false
		}
		var env ReviewEnvelope
		n, ok := decodeMarker(raw, &env)
		budget.spend(n)
		if ok {
			out = append(out, env)
		}
		return true
	})
	return out
}

// CollectFindingEnvelopes decodes the finding carriers found in body, bounded by
// the per-body carrier budget.
func CollectFindingEnvelopes(body string) []FindingEnvelope {
	var out []FindingEnvelope
	budget := &carrierBudget{}
	scanMarkers(body, FindingMarkerPrefix, func(raw string) bool {
		if !budget.allow() {
			return false
		}
		var env FindingEnvelope
		n, ok := decodeMarker(raw, &env)
		budget.spend(n)
		if ok {
			out = append(out, env)
		}
		return true
	})
	return out
}

// DetectThreadReview inspects a discussion's root note body for a nickpit carrier
// marker. A finding carrier pins a chat to that finding; a review carrier means a
// whole-review chat. It reports ok=false when the note carries no nickpit marker,
// i.e. the thread was not started by nickpit. Scanning stops at the first valid
// envelope and shares the per-body decompression budget, so a hostile body cannot
// force unbounded work.
func DetectThreadReview(rootBody string) (reviewID, findingID string, ok bool) {
	budget := &carrierBudget{}
	scanMarkers(rootBody, FindingMarkerPrefix, func(raw string) bool {
		if !budget.allow() {
			return false
		}
		var env FindingEnvelope
		n, good := decodeMarker(raw, &env)
		budget.spend(n)
		if good && env.ReviewID != "" {
			reviewID, findingID, ok = env.ReviewID, env.Finding.ID, true
			return false
		}
		return true
	})
	if ok {
		return reviewID, findingID, true
	}
	scanMarkers(rootBody, ReviewMarkerPrefix, func(raw string) bool {
		if !budget.allow() {
			return false
		}
		var env ReviewEnvelope
		n, good := decodeMarker(raw, &env)
		budget.spend(n)
		if good && env.ReviewID != "" {
			reviewID, ok = env.ReviewID, true
			return false
		}
		return true
	})
	return reviewID, findingID, ok
}

// StripMarkers removes every nickpit hidden marker (`<!-- nickpit:... -->`) from
// s. SCM adapters apply it when normalizing existing comments into prompt
// context, so the (potentially large) carrier payloads are never re-sent to the
// model as opaque comment text; the raw bodies remain available separately for
// carrier reassembly.
func StripMarkers(s string) string {
	if !strings.Contains(s, MarkerOpen) {
		return s
	}
	var b strings.Builder
	rest := s
	for {
		i := strings.Index(rest, MarkerOpen)
		if i < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:i])
		j := strings.Index(rest[i:], "-->")
		if j < 0 {
			// Unterminated marker: drop the tail rather than re-sending it.
			break
		}
		rest = rest[i+j+3:]
	}
	return strings.TrimSpace(b.String())
}

// ReviewResultsByID reassembles complete ReviewResults from the carrier markers
// spread across the given note/comment bodies, keyed by review id. The overall
// verdict and metadata come from each review carrier; findings are collected from
// the per-finding carriers. Findings are de-duplicated by id within a review so a
// finding that appears in both the notes list and the discussions list (GitLab
// returns discussion notes in both) is not counted twice.
func ReviewResultsByID(bodies []string) map[string]*model.ReviewResult {
	byID := make(map[string]*model.ReviewResult)
	seen := make(map[string]map[string]struct{})
	get := func(rid string) *model.ReviewResult {
		r := byID[rid]
		if r == nil {
			r = &model.ReviewResult{ReviewID: rid}
			byID[rid] = r
			seen[rid] = make(map[string]struct{})
		}
		return r
	}
	for _, body := range bodies {
		for _, env := range CollectReviewEnvelopes(body) {
			if env.ReviewID == "" {
				continue
			}
			r := get(env.ReviewID)
			r.CreatedAt = env.CreatedAt
			r.OverallCorrectness = env.OverallCorrectness
			r.OverallExplanation = env.OverallExplanation
			r.OverallConfidenceScore = env.OverallConfidenceScore
			r.Repo = env.Repo
			r.Mode = env.Mode
			r.Identifier = env.Identifier
			r.BaseRef = env.BaseRef
			r.HeadRef = env.HeadRef

			r.Model = env.Model
		}
	}
	for _, body := range bodies {
		for _, env := range CollectFindingEnvelopes(body) {
			if env.ReviewID == "" {
				continue
			}
			r := get(env.ReviewID)
			if id := env.Finding.ID; id != "" {
				if _, dup := seen[env.ReviewID][id]; dup {
					continue
				}
				seen[env.ReviewID][id] = struct{}{}
			}
			r.Findings = append(r.Findings, env.Finding)
		}
	}
	return byID
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
	if p == nil {
		return
	}
	if p.Markers == nil {
		p.Markers = map[string]struct{}{}
	}
	CollectMarkers(body, p.Markers)
	CollectPriorFindings(body, &p.Findings)
}

// AlreadyPosted reports whether finding was already published in p. A finding
// from the same run matches exactly on its id. Across runs, identity is the
// deterministic file+displayed-title fuzzy match at dedupe.Duplicate; cross-file
// pairs are capped below Duplicate by dedupe.Compare, so a different file never
// matches. This intentionally ignores line range and body because SCM anchors
// and generated text are less stable than the displayed title. Two distinct
// same-file findings with near-identical displayed titles can collide and the
// later one can be suppressed; that is the accepted tradeoff for avoiding
// reposts across repeated runs. displayTitle is the title shown in the comment,
// so the comparison is like-for-like with the title each prior carries.
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
	// reviewID, when set (see ForReview), is embedded as a hidden per-finding
	// carrier marker so findings can later be regrouped by review run. SummaryBody
	// reads the id from the result directly and does not need this.
	reviewID string
}

// ForReview returns a copy of the renderer bound to reviewID, so FindingBody
// emits the hidden per-finding carrier marker for that run. Renderer is a value
// type, so this never mutates the shared renderer.
func (r Renderer) ForReview(reviewID string) Renderer {
	r.reviewID = reviewID
	return r
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
	if marker := ReviewMarker(result); marker != "" {
		b.WriteString("\n")
		b.WriteString(marker)
	}
	return b.String()
}

// carrierNoteMaxBytes bounds one carrier note body. GitHub caps comments at
// 65,536 characters (GitLab at ~1M); staying under the stricter limit keeps
// carrier chunks postable on both platforms. A single oversized marker still
// gets its own chunk — the post may fail, which surfaces as a publish warning
// rather than silently dropping the finding.
const carrierNoteMaxBytes = 60_000

// carrierNoteMaxDecodedBytes bounds the DECODED (pre-compression) payload total
// of one carrier note. Highly compressible envelopes can pack far more than the
// reader's per-body decompression budget under the encoded byte bound alone;
// the reader would then stop mid-body and silently drop the rest. Half the
// reader budget leaves comfortable margin.
const carrierNoteMaxDecodedBytes = maxCarrierTotalDecodedBytes / 2

// CarrierNotes renders hidden note bodies carrying the review envelope plus one
// finding envelope per given finding, split into chunks bounded by size and by
// the reader's per-body marker budget. It exists so a run whose visible posts
// are incomplete — a re-review with the summary and duplicate findings
// suppressed for idempotency, or a publish where some posts failed — still
// leaves the full data on the MR/PR for a later chat to reassemble by review
// id. Callers pass only the findings that lack their own per-finding carrier.
// The bodies are only HTML-comment markers, so they render empty. Returns nil
// when the result has no review id.
func (r Renderer) CarrierNotes(result *model.ReviewResult, findings []model.Finding) []string {
	if result == nil || result.ReviewID == "" {
		return nil
	}
	var notes []string
	var b strings.Builder
	markers, decoded := 0, 0
	flush := func() {
		if b.Len() > 0 {
			notes = append(notes, b.String())
			b.Reset()
			markers, decoded = 0, 0
		}
	}
	if marker, size := reviewMarkerWithSize(result); marker != "" {
		b.WriteString(marker)
		markers++
		decoded += size
	}
	for _, finding := range findings {
		marker, size := findingMarkerWithSize(result.ReviewID, finding)
		if marker == "" {
			continue
		}
		// Flush on any bound: encoded note size (SCM comment limits), marker
		// count, or decoded payload total (the reader's per-body decompression
		// budget — highly compressible envelopes can blow it while staying small
		// encoded, and the reader would silently drop the tail).
		if b.Len() > 0 && (b.Len()+1+len(marker) > carrierNoteMaxBytes ||
			markers >= maxCarriersPerBody ||
			decoded+size > carrierNoteMaxDecodedBytes) {
			flush()
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(marker)
		markers++
		decoded += size
	}
	flush()
	return notes
}

// FindingBody renders a finding as markdown, tagged with its FingerprintMarker. When
// locationPrefix is non-empty (the general-comment fallback used when a finding
// cannot be anchored inline) it is shown after the badge/confidence block so the
// location is still visible without an inline anchor.
func (r Renderer) FindingBody(finding model.Finding, locationPrefix string) string {
	body, _ := r.FindingBodyCarried(finding, locationPrefix)
	return body
}

// FindingBodyCarried is FindingBody plus whether the full-finding carrier
// marker actually rode along. The carrier is omitted when it would push the
// comment past the platform size limit — an unusually long (e.g. imported)
// finding must still publish its visible text; carrier metadata is never worth
// losing the comment over. Publishers use carried=false to route the finding
// into the chunked fallback carrier notes instead.
func (r Renderer) FindingBodyCarried(finding model.Finding, locationPrefix string) (string, bool) {
	title, body, rank, confidence := FindingDisplay(finding)
	fingerprint := FingerprintMarker(finding, title)
	var b strings.Builder
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
	suggestions := FindingDisplaySuggestions(finding)
	if len(suggestions) > 0 {
		b.WriteString("\n\n**Suggestions**  \n")
		for _, suggestion := range suggestions {
			text := strings.TrimSpace(suggestion.Body)
			if text == "" {
				continue
			}
			formatted := strings.ReplaceAll(sanitizeWithHardBreaks(text), "\n", "\n  ")
			fmt.Fprintf(&b, "\n- %s", formatted)
		}
	}
	visible := b.String()
	carrier := FindingMarker(r.reviewID, finding)
	// The carried decision must be independent of the locationPrefix variant:
	// the same finding is rendered with and without a "`file:line`" prefix
	// across inline posts and their general-comment fallbacks, and a
	// near-boundary finding must not carry in one render while silently dropping
	// the carrier in the other (publishers record the decision once). The prefix
	// block is therefore excluded from the measurement, and a slack larger than
	// any OS-length path keeps the real, prefixed body under the platform limit.
	const carrierNoteSizeSlack = 8192
	prefixBlockLen := 0
	if locationPrefix != "" {
		prefixBlockLen = len(locationPrefix) + len("  \n\n")
	}
	decisionLen := len(fingerprint) + 1 + len(carrier) + len(visible) - prefixBlockLen
	carried := carrier != "" && decisionLen+carrierNoteSizeSlack <= carrierNoteMaxBytes
	if carried {
		return fingerprint + "\n" + carrier + visible, true
	}
	return fingerprint + visible, false
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

// FindingDisplaySuggestions returns the suggestion text that should be shown in
// output, preferring the latest downstream pass while keeping the original
// reviewer suggestions as a fallback.
func FindingDisplaySuggestions(finding model.Finding) []model.Suggestion {
	if finding.Summarization != nil && len(finding.Summarization.Suggestions) > 0 {
		return finding.Summarization.Suggestions
	}
	if finding.Finalization != nil && len(finding.Finalization.Suggestions) > 0 {
		return finding.Finalization.Suggestions
	}
	return finding.Suggestions
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
