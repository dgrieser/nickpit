package model

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type ReviewMode string

const (
	ModeLocal  ReviewMode = "local"
	ModeGitHub ReviewMode = "github"
	ModeGitLab ReviewMode = "gitlab"
)

type FileStatus string

const (
	FileAdded    FileStatus = "added"
	FileModified FileStatus = "modified"
	FileDeleted  FileStatus = "deleted"
	FileRenamed  FileStatus = "renamed"
)

type DiffFormat string

const (
	DiffFormatGit     DiffFormat = "git"
	DiffFormatGitJson DiffFormat = "git-json"
)

type ReviewRequest struct {
	Mode                  ReviewMode
	RepoRoot              string
	Workdir               string
	Repo                  string
	Identifier            int
	BaseRef               string
	HeadRef               string
	IncludeComments       bool
	IncludeCommits        bool
	IncludeFullFiles      bool
	IncludePaths          []string
	ExcludePaths          []string
	IncludeContent        []string
	ExcludeContent        []string
	DiffFormat            DiffFormat
	MaxContextTokens      int
	MaxToolCalls          int
	MaxDuplicateToolCalls int
	MaxOutputRetries      int
	MaxReasoningSeconds   int
	// Concurrency caps concurrent LLM agent loops across the whole pipeline
	// run (one shared limiter: reviewers, verify, dedupe, merge, finalize,
	// summarize); 0 = unlimited.
	Concurrency         int
	VerifyDropPolicy    string
	ConfidenceThreshold float64
	NudgeCount          int
	// MaxFindings caps the findings each review agent may report across its
	// initial pass and nudges; 0 = unlimited.
	MaxFindings               int
	DisableDiffScope          bool
	DisableParallelToolCalls  bool
	DisableReasoningExtract   bool
	DisablePatchSummary       bool
	DisableSuggestions        bool
	DisableWorkflowTimeBudget bool
	ModelEmitsReasoning       bool
	DisableJSONResponseFormat bool
	PriorityThreshold         string
	PostReview                bool
	Submode                   string
	ProfileName               string
}

type ReviewResult struct {
	// ReviewID uniquely identifies a single completed review run. It is stamped
	// once the pipeline finishes and is carried in the hidden SCM note markers so
	// all findings and the overall verdict for one run can be regrouped later
	// (e.g. when starting a discussion about the review).
	ReviewID string `json:"review_id,omitempty"`
	// CreatedAt records when the review completed. Carried in the hidden SCM
	// note markers so the newest of several reviews on one MR/PR can be selected.
	CreatedAt              time.Time  `json:"created_at,omitzero"`
	Findings               []Finding  `json:"findings"`
	OverallCorrectness     string     `json:"overall_correctness"`
	OverallExplanation     string     `json:"overall_explanation"`
	OverallConfidenceScore float64    `json:"overall_confidence_score"`
	AgentRuns              []AgentRun `json:"agent_runs,omitempty"`
	Warnings               []string   `json:"warnings,omitempty"`
	TokensUsed             TokenUsage `json:"tokens_used"`
	VerifyTokensUsed       TokenUsage `json:"verify_tokens_used"`
	FinalizeTokensUsed     TokenUsage `json:"finalize_tokens_used"`
	VerdictTokensUsed      TokenUsage `json:"verdict_tokens_used"`
	SummarizeTokensUsed    TokenUsage `json:"summarize_tokens_used"`
	// RuntimeSeconds is the whole review command span in seconds (model check,
	// checkout, pipeline through summarize).
	RuntimeSeconds float64 `json:"runtime_seconds,omitempty"`
	// SegmentRuntimes records the wall-clock span of each pipeline unit (a
	// single step or a parallel group) in execution order.
	SegmentRuntimes []SegmentRuntime `json:"segment_runtimes,omitempty"`
	Mode            string           `json:"mode,omitempty"`
	Repo            string           `json:"repo,omitempty"`
	Identifier      int              `json:"identifier,omitempty"`
	BaseRef         string           `json:"base_ref,omitempty"`
	HeadRef         string           `json:"head_ref,omitempty"`
	BaseURL         string           `json:"base_url,omitempty"`
	Model           string           `json:"model,omitempty"`
	ReasoningEffort string           `json:"reasoning_effort,omitempty"`
	TotalToolCalls  int              `json:"total_tool_calls,omitempty"`
}

// StripSuggestions removes code suggestions from every finding in the result.
func (r *ReviewResult) StripSuggestions() {
	if r == nil {
		return
	}
	StripSuggestions(r.Findings)
}

type AgentRun struct {
	Name                  string     `json:"name"`
	Role                  string     `json:"role"`
	Findings              int        `json:"findings,omitempty"`
	MaxToolCalls          int        `json:"max_tool_calls"`
	MaxDuplicateToolCalls int        `json:"max_duplicate_tool_calls,omitempty"`
	ToolCalls             int        `json:"tool_calls,omitempty"`
	DuplicateToolCalls    int        `json:"duplicate_tool_calls"`
	TokensUsed            TokenUsage `json:"tokens_used"`
	// RuntimeSeconds is the agent's total wall-clock runtime in seconds
	// (reviewers: initial pass through nudges and reasoning extraction).
	RuntimeSeconds float64 `json:"runtime_seconds,omitempty"`
	// Status is one of AgentRunStatus*. Empty = implicit ok (preserves
	// backward compatibility with pre-failure-tolerance consumers).
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

// SegmentRuntime is the wall-clock span of one pipeline unit: a single step
// or a parallel group (whose runtime is the span of its slowest lane). Each
// Steps entry is one lane — a sequential chain joined with "→", e.g.
// "review:security→verify:security→dedupe:security".
type SegmentRuntime struct {
	Steps          []string `json:"steps"`
	RuntimeSeconds float64  `json:"runtime_seconds"`
}

const (
	AgentRunStatusOK      = ""
	AgentRunStatusPartial = "partial"
	AgentRunStatusFailed  = "failed"
	AgentRunStatusSkipped = "skipped"
)

type ReviewContext struct {
	Mode         ReviewMode      `json:"mode"`
	CheckoutRoot string          `json:"checkout_root,omitempty"`
	Identifier   int             `json:"identifier,omitempty"`
	Repository   RepositoryInfo  `json:"repository"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Commits      []CommitSummary `json:"commits,omitempty"`
	ChangedFiles []ChangedFile   `json:"changed_files"`
	Diff         string          `json:"diff"`
	DiffFiles    []DiffFile      `json:"diff_files,omitempty"`
	DiffHunks    []DiffHunk      `json:"diff_hunks,omitempty"`
	// DiffScopeHunks retains the filtered, pre-trimming diff used for
	// deterministic finding-scope checks. It is runtime-only so prompt payloads
	// and serialized review context keep their existing shape. A non-nil empty
	// slice means scope checking is available but the diff has no line hunks.
	DiffScopeHunks []DiffHunk `json:"-"`
	// DiffBaseSHA and DiffHeadSHA identify the exact commits this context's diff
	// was built between, when the source knows them (GitLab MRs). They are
	// deliberately NOT copied into prompt payloads; chat sessions persist them so
	// a cached context's freshness can be checked against the live MR without a
	// spurious first-resume refresh.
	DiffBaseSHA         string             `json:"diff_base_sha,omitempty"`
	DiffHeadSHA         string             `json:"diff_head_sha,omitempty"`
	Comments            []Comment          `json:"comments,omitempty"`
	SupplementalContext []SupplementalFile `json:"supplemental_context,omitempty"`
	ToolchainVersions   []ToolchainVersion `json:"toolchain_versions,omitempty"`
	OmittedSections     []string           `json:"omitted_sections,omitempty"`
}

type ReviewPromptPayload struct {
	Repository          RepositoryInfo     `json:"repository"`
	ToolchainVersions   []ToolchainVersion `json:"toolchain_versions,omitempty"`
	ChangedFiles        []ChangedFile      `json:"changed_files"`
	DiffFiles           []DiffFile         `json:"diff_files,omitempty"`
	DiffHunks           []DiffHunk         `json:"diff_hunks,omitempty"`
	Commits             []CommitSummary    `json:"commits,omitempty"`
	Identifier          int                `json:"identifier,omitempty"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	Comments            []Comment          `json:"comments,omitempty"`
	SupplementalContext []SupplementalFile `json:"supplemental_context,omitempty"`
	OmittedSections     []string           `json:"omitted_sections,omitempty"`
	StyleGuides         []StyleGuide       `json:"style_guides,omitempty"`
}

type ToolchainVersion struct {
	Language    string `json:"language"`
	Source      string `json:"source,omitempty"`
	Field       string `json:"field,omitempty"`
	Version     string `json:"version,omitempty"`
	Error       string `json:"error,omitempty"`
	Unavailable bool   `json:"unavailable,omitempty"`
}

type RepositoryInfo struct {
	FullName string `json:"full_name"`
	BaseRef  string `json:"base_ref,omitempty"`
	HeadRef  string `json:"head_ref,omitempty"`
	URL      string `json:"url,omitempty"`
}

type ChangedFile struct {
	Path      string     `json:"path"`
	Status    FileStatus `json:"status"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	PatchURL  string     `json:"patch_url,omitempty"`
	Generated bool       `json:"generated,omitempty"`
}

type DiffFile struct {
	FilePath  string `json:"file_path"`
	Language  string `json:"language,omitempty"`
	Content   string `json:"content"`
	Generated bool   `json:"generated,omitempty"`
}

type DiffHunk struct {
	FilePath string `json:"file_path"`
	Language string `json:"language,omitempty"`
	OldStart int    `json:"old_start"`
	OldLines int    `json:"old_lines"`
	NewStart int    `json:"new_start"`
	NewLines int    `json:"new_lines"`
	Content  string `json:"content"`
}

type StyleGuide struct {
	Language string `json:"language"`
	Content  string `json:"content"`
}

// StyleGuideSpec is a user-configured additional styleguide. In YAML it is
// either a scalar string (a file path or http(s) URL applied to every agent,
// back-compat) or a mapping that gates the guide to a language and optionally a
// version/semver range, so it is injected only when that language changed and
// (when set) the detected toolchain version matches.
type StyleGuideSpec struct {
	Source   string `yaml:"source"`
	Language string `yaml:"language,omitempty"`
	Version  string `yaml:"version,omitempty"`
}

// UnmarshalYAML accepts a scalar (=> Source only) or a mapping (full spec).
func (s *StyleGuideSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.Source = node.Value
		return nil
	}
	type raw StyleGuideSpec
	return node.Decode((*raw)(s))
}

// AdditionalStyleGuide is a resolved user-configured styleguide plus the
// gating metadata that decides when it is injected. GateLanguage == "" means
// unconditional (back-compat scalar spec); GateVersion == "" means inject
// whenever GateLanguage is among the changed languages.
type AdditionalStyleGuide struct {
	StyleGuide
	GateLanguage string
	GateVersion  string
}

type Comment struct {
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Path      string    `json:"path,omitempty"`
	Line      int       `json:"line,omitempty"`
	Side      string    `json:"side,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	IsReview  bool      `json:"is_review"`
	ThreadID  string    `json:"thread_id,omitempty"`
}

type CommitSummary struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
}

type Finding struct {
	// ID is required at serialization boundaries; regenerate legacy artifacts
	// that predate UUID finding IDs.
	ID              string                `json:"id"`
	Title           string                `json:"title"`
	Body            string                `json:"body"`
	ConfidenceScore float64               `json:"confidence_score"`
	Priority        *int                  `json:"priority,omitempty"`
	CodeLocation    CodeLocation          `json:"code_location"`
	Suggestions     []Suggestion          `json:"suggestions,omitempty"`
	Verification    *FindingVerification  `json:"verification,omitempty"`
	Finalization    *FindingFinalization  `json:"finalization,omitempty"`
	Summarization   *FindingSummarization `json:"summarization,omitempty"`
	// MergedFrom is merge-step provenance only: the ids of cluster findings
	// absorbed into this one. Validation consumes it to detect silently
	// dropped findings; the merge step strips it before findings leave the
	// step, so it never reaches results or posted reviews.
	MergedFrom []string `json:"merged_from,omitempty"`
}

// StripSuggestions removes code suggestions from every finding.
func StripSuggestions(findings []Finding) {
	for i := range findings {
		findings[i].Suggestions = nil
		if findings[i].Finalization != nil {
			findings[i].Finalization.Suggestions = nil
		}
		if findings[i].Summarization != nil {
			findings[i].Summarization.Suggestions = nil
		}
	}
}

// KeepFirstSuggestion limits every suggestion-bearing representation of each
// finding to its first entry. Callers that must preserve their input should
// clone the findings before calling this helper.
func KeepFirstSuggestion(findings []Finding) {
	for i := range findings {
		findings[i].Suggestions = firstSuggestion(findings[i].Suggestions)
		if findings[i].Finalization != nil {
			findings[i].Finalization.Suggestions = firstSuggestion(findings[i].Finalization.Suggestions)
		}
		if findings[i].Summarization != nil {
			findings[i].Summarization.Suggestions = firstSuggestion(findings[i].Summarization.Suggestions)
		}
	}
}

func firstSuggestion(suggestions []Suggestion) []Suggestion {
	if len(suggestions) <= 1 {
		return suggestions
	}
	return suggestions[:1:1]
}

// EnsureFindingIDs backfills invalid IDs and remints duplicates so every
// finding is uniquely addressable downstream. It returns the number of
// findings whose ID changed (invalid or duplicate). Any rewrite also re-syncs
// Verification.ID, which mirrors the parent Finding.ID by design.
func EnsureFindingIDs(findings []Finding) int {
	seen := make(map[string]struct{}, len(findings))
	overwrote := 0
	for i := range findings {
		rewrote := EnsureFindingID(&findings[i])
		if _, ok := seen[findings[i].ID]; ok {
			findings[i].ID = newUniqueUUID(seen)
			rewrote = true
		}
		if rewrote {
			overwrote++
			if findings[i].Verification != nil {
				findings[i].Verification.ID = findings[i].ID
			}
		}
		seen[findings[i].ID] = struct{}{}
	}
	return overwrote
}

func EnsureFindingID(f *Finding) bool {
	if f == nil {
		return false
	}
	if _, err := uuid.Parse(f.ID); err != nil {
		overwrote := f.ID != ""
		f.ID = uuid.NewString()
		return overwrote
	}
	return false
}

func newUniqueUUID(seen map[string]struct{}) string {
	for {
		id := uuid.NewString()
		if _, ok := seen[id]; !ok {
			return id
		}
	}
}

const (
	VerdictConfirmed  = "confirmed"
	VerdictRefuted    = "refuted"
	VerdictUnverified = "unverified"
)

// Verify decision-order gates. The verifier must name the gate that decided
// its verdict; the gate dictates the verdict (see VerdictForGate).
const (
	GateNonFinding              = "non-finding"
	GateDiffScope               = "diff-scope"
	GateStyleguideContradiction = "styleguide-contradiction"
	GateCompileError            = "compile-error"
	GateConfirm                 = "confirm"
	GateRefute                  = "refute"
	GateUnverified              = "unverified"
)

// VerdictForGate returns the verdict a decision-order gate dictates and
// whether the gate is known. Every gate except confirm and unverified refutes.
func VerdictForGate(gate string) (string, bool) {
	switch gate {
	case GateConfirm:
		return VerdictConfirmed, true
	case GateUnverified:
		return VerdictUnverified, true
	case GateNonFinding, GateDiffScope, GateStyleguideContradiction, GateCompileError, GateRefute:
		return VerdictRefuted, true
	}
	return "", false
}

type FindingVerification struct {
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
	// Gate is the decision-order gate that produced the verdict. Optional in
	// serialized artifacts (predates the field; fallback verifications carry
	// no gate), required from the verify agent.
	Gate            string  `json:"gate,omitempty"`
	Priority        int     `json:"priority"`
	ConfidenceScore float64 `json:"confidence_score"`
	Remarks         string  `json:"remarks"`
}

// MergeFrom implements the merge-aware contract used by jsonx.Mergeable.
// Multi-block verify refinements overwrite only the fields a candidate
// actually emitted, so a partial later block (e.g. only "remarks") does
// not blank earlier ID/Verdict/Priority/ConfidenceScore.
func (v *FindingVerification) MergeFrom(other any, presentKeys map[string]bool) (bool, error) {
	src, ok := other.(*FindingVerification)
	if !ok || src == nil {
		return false, nil
	}
	claimed := false
	if presentKeys["id"] {
		v.ID = src.ID
		claimed = true
	}
	if presentKeys["verdict"] {
		v.Verdict = src.Verdict
		claimed = true
	}
	if presentKeys["gate"] {
		v.Gate = src.Gate
		claimed = true
	}
	if presentKeys["priority"] {
		v.Priority = src.Priority
		claimed = true
	}
	if presentKeys["confidence_score"] {
		v.ConfidenceScore = src.ConfidenceScore
		claimed = true
	}
	if presentKeys["remarks"] {
		v.Remarks = src.Remarks
		claimed = true
	}
	return claimed, nil
}

// Verification.ID mirrors parent Finding.ID by design; uniqueness is enforced
// upstream by EnsureFindingIDs.
func EnsureVerificationID(v *FindingVerification, fallback string) {
	if v == nil {
		return
	}
	if _, err := uuid.Parse(v.ID); err == nil {
		return
	}
	if _, err := uuid.Parse(fallback); err == nil {
		v.ID = fallback
		return
	}
	v.ID = uuid.NewString()
}

type FindingFinalization struct {
	Title           string       `json:"title"`
	Body            string       `json:"body"`
	Priority        int          `json:"priority"`
	ConfidenceScore float64      `json:"confidence_score"`
	Remarks         string       `json:"remarks"`
	Suggestions     []Suggestion `json:"suggestions,omitempty"`
}

// FindingSummarization carries the shortened, more readable body produced by the
// summarize pass. Its body is the only field the summarizer LLM emits; every
// other field is copied verbatim in code from the finding's FindingFinalization
// (see applySummarizedFinding in internal/review/summarizer.go), so a
// summarization always mirrors the finalization it shortens apart from Body and
// any suggestion bodies shortened in the same pass.
type FindingSummarization struct {
	Title           string       `json:"title"`
	Body            string       `json:"body"`
	Priority        int          `json:"priority"`
	ConfidenceScore float64      `json:"confidence_score"`
	Remarks         string       `json:"remarks"`
	Suggestions     []Suggestion `json:"suggestions,omitempty"`
}

type CodeLocation struct {
	FilePath  string    `json:"file_path"`
	LineRange LineRange `json:"line_range"`
	Language  string    `json:"language,omitempty"`
	Content   string    `json:"content"`
}

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
	Count int `json:"count"`
}

func (r LineRange) MarshalJSON() ([]byte, error) {
	type lineRange struct {
		Start int `json:"start"`
		End   int `json:"end"`
		Count int `json:"count"`
	}
	return json.Marshal(lineRange{Start: r.Start, End: r.End, Count: r.EffectiveCount()})
}

func (r *LineRange) UnmarshalJSON(data []byte) error {
	type lineRange struct {
		Start int `json:"start"`
		End   int `json:"end"`
		Count int `json:"count"`
	}
	var parsed lineRange
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*r = LineRange(parsed)
	if r.Count == 0 {
		r.Count = r.EffectiveCount()
	}
	return nil
}

func (r LineRange) EffectiveCount() int {
	if r.Count > 0 {
		return r.Count
	}
	if r.Start > 0 && r.End >= r.Start {
		return r.End - r.Start + 1
	}
	return 0
}

func (r LineRange) SameAnchor(other LineRange) bool {
	return r.Start == other.Start && r.End == other.End
}

type Suggestion struct {
	Body         string       `json:"body"`
	CodeLocation CodeLocation `json:"code_location"`
	LineRange    LineRange    `json:"-"`
}

func (s Suggestion) MarshalJSON() ([]byte, error) {
	type suggestion struct {
		Body         string       `json:"body"`
		CodeLocation CodeLocation `json:"code_location"`
	}
	return json.Marshal(suggestion{
		Body:         s.Body,
		CodeLocation: s.effectiveCodeLocation(),
	})
}

func (s *Suggestion) UnmarshalJSON(data []byte) error {
	var body string
	if err := json.Unmarshal(data, &body); err == nil {
		s.Body = body
		s.CodeLocation = CodeLocation{}
		s.LineRange = LineRange{}
		return nil
	}

	type suggestion struct {
		Body         string       `json:"body"`
		CodeLocation CodeLocation `json:"code_location"`
		LineRange    LineRange    `json:"line_range"`
	}
	var parsed suggestion
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	s.Body = parsed.Body
	s.CodeLocation = parsed.CodeLocation
	s.LineRange = s.CodeLocation.LineRange
	if s.LineRange == (LineRange{}) {
		s.LineRange = parsed.LineRange
	}
	return nil
}

func (s Suggestion) effectiveCodeLocation() CodeLocation {
	loc := s.CodeLocation
	if loc.LineRange == (LineRange{}) {
		loc.LineRange = s.LineRange
	}
	return loc
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt"`
	CompletionTokens int `json:"completion"`
	TotalTokens      int `json:"total"`
}

type SupplementalFile struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Language  string `json:"language,omitempty"`
	Content   string `json:"content"`
	Kind      string `json:"kind,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ReviewSource interface {
	ResolveContext(ctx context.Context, req ReviewRequest) (*ReviewContext, error)
}

type RemoteCheckoutSource interface {
	ReviewSource
	ResolveCheckout(ctx context.Context, req ReviewRequest) (*CheckoutSpec, error)
}

// ReviewPublisher is an optional capability for review sources that can post
// the finished review back to the origin (GitLab MR and GitHub PR comments).
// runReview type-asserts the source to this interface and only publishes when
// PostReview is set, so non-publishing sources (local) are unaffected.
type ReviewPublisher interface {
	PublishReview(ctx context.Context, req ReviewRequest, result *ReviewResult) error
}

type CheckoutSpec struct {
	Provider ReviewMode
	Repo     string
	CloneURL string
	HeadRef  string
	HeadSHA  string
}

type TokenEstimator interface {
	Estimate(text string) int
}

// LengthEstimator enables incremental token accounting: for estimators whose
// Estimate depends only on len(text), EstimateLen(n) must equal Estimate of
// any string of length n.
type LengthEstimator interface {
	EstimateLen(length int) int
}

type SimpleEstimator struct{}

func (SimpleEstimator) Estimate(text string) int {
	return len(text) / 4
}

func (SimpleEstimator) EstimateLen(length int) int {
	return length / 4
}

func PriorityRank(priority *int) int {
	if priority == nil {
		return 3
	}
	if *priority < 0 {
		return 0
	}
	if *priority > 3 {
		return 3
	}
	return *priority
}

// NormalizeConfidence coerces an LLM-provided confidence into [0,1]. Models
// sometimes emit a 0–100 scale (e.g. 95 or 100); rescale those, then clamp.
// Values already in [0,1] are returned unchanged.
func NormalizeConfidence(v float64) float64 {
	if v > 1 {
		v /= 100
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func PriorityThresholdRank(value string) int {
	switch value {
	case "p0":
		return 0
	case "p1":
		return 1
	case "p2":
		return 2
	default:
		return 3
	}
}

// NormalizePriorityThreshold maps the numeric priority-threshold form
// (0 highest .. 3 lowest) to the internal "pN" representation consumed by
// PriorityThresholdRank and the finding badges. Non-numeric or out-of-range
// input is rejected; there is no legacy "pN" fallback. It is shared by CLI flag
// parsing and workflow-spec decoding so both reach the engine as "pN".
func NormalizePriorityThreshold(value string) (string, error) {
	switch value {
	case "0", "1", "2", "3":
		return "p" + value, nil
	default:
		return "", fmt.Errorf("must be one of 0, 1, 2, 3 (got %q)", value)
	}
}

func CloneContext(src *ReviewContext) (*ReviewContext, error) {
	if src == nil {
		return nil, nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return nil, fmt.Errorf("model: CloneContext marshal: %w", err)
	}
	var out ReviewContext
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("model: CloneContext unmarshal: %w", err)
	}
	if src.DiffScopeHunks != nil {
		out.DiffScopeHunks = append([]DiffHunk{}, src.DiffScopeHunks...)
	}
	return &out, nil
}

func (r *ReviewResult) Clone() (*ReviewResult, error) {
	if r == nil {
		return nil, nil
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("model: Clone marshal: %w", err)
	}
	var out ReviewResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("model: Clone unmarshal: %w", err)
	}
	return &out, nil
}

func PromptPayloadFromContext(src *ReviewContext) *ReviewPromptPayload {
	return PromptPayloadFromContextWithDiffFormat(src, DiffFormatGit)
}

func PromptPayloadFromContextWithDiffFormat(src *ReviewContext, format DiffFormat) *ReviewPromptPayload {
	if src == nil {
		return nil
	}
	payload := &ReviewPromptPayload{
		Repository:          src.Repository,
		ToolchainVersions:   src.ToolchainVersions,
		ChangedFiles:        src.ChangedFiles,
		Commits:             src.Commits,
		Identifier:          src.Identifier,
		Title:               src.Title,
		Description:         src.Description,
		Comments:            src.Comments,
		SupplementalContext: src.SupplementalContext,
		OmittedSections:     src.OmittedSections,
	}
	switch format {
	case DiffFormatGitJson:
		payload.DiffHunks = src.DiffHunks
	default:
		if len(src.DiffFiles) > 0 {
			payload.DiffFiles = src.DiffFiles
		} else {
			payload.DiffHunks = src.DiffHunks
		}
	}
	return payload
}
