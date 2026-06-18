package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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

type ReviewRequest struct {
	Mode                    ReviewMode
	RepoRoot                string
	Workdir                 string
	Repo                    string
	Identifier              int
	BaseRef                 string
	HeadRef                 string
	IncludeComments         bool
	IncludeCommits          bool
	IncludeFullFiles        bool
	IncludePaths            []string
	ExcludePaths            []string
	IncludeContent          []string
	ExcludeContent          []string
	MaxContextTokens        int
	MaxToolCalls            int
	MaxDuplicateToolCalls   int
	MaxOutputRetries        int
	MaxReasoningSeconds     int
	MaxReasoningLoopRepeats int
	// Concurrency caps concurrent LLM agent loops across the whole pipeline
	// run (one shared limiter: reviewers, verify, dedupe, merge, finalize,
	// summarize); 0 = unlimited.
	Concurrency              int
	VerifyDropPolicy         string
	VerifyDropConfidence     float64
	NudgeCount               int
	DisableParallelToolCalls bool
	DisableReasoningExtract  bool
	DisablePatchSummary      bool
	SkipSuggestions          bool
	ModelEmitsReasoning      bool
	UseJSONSchema            bool
	PriorityThreshold        string
	Offline                  bool
	PostReview               bool
	Submode                  string
	ProfileName              string
}

type ReviewResult struct {
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
	Mode                ReviewMode         `json:"mode"`
	CheckoutRoot        string             `json:"checkout_root,omitempty"`
	Identifier          int                `json:"identifier,omitempty"`
	Repository          RepositoryInfo     `json:"repository"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	Commits             []CommitSummary    `json:"commits,omitempty"`
	ChangedFiles        []ChangedFile      `json:"changed_files"`
	Diff                string             `json:"diff"`
	DiffHunks           []DiffHunk         `json:"diff_hunks,omitempty"`
	Comments            []Comment          `json:"comments,omitempty"`
	SupplementalContext []SupplementalFile `json:"supplemental_context,omitempty"`
	ToolchainVersions   []ToolchainVersion `json:"toolchain_versions,omitempty"`
	OmittedSections     []string           `json:"omitted_sections,omitempty"`
}

type ReviewPromptPayload struct {
	Repository          RepositoryInfo     `json:"repository"`
	Identifier          int                `json:"identifier,omitempty"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	Commits             []CommitSummary    `json:"commits,omitempty"`
	ChangedFiles        []ChangedFile      `json:"changed_files"`
	DiffHunks           []DiffHunk         `json:"diff_hunks,omitempty"`
	StyleGuides         []StyleGuide       `json:"style_guides,omitempty"`
	Comments            []Comment          `json:"comments,omitempty"`
	SupplementalContext []SupplementalFile `json:"supplemental_context,omitempty"`
	ToolchainVersions   []ToolchainVersion `json:"toolchain_versions,omitempty"`
	OmittedSections     []string           `json:"omitted_sections,omitempty"`
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
	Title    string `json:"-"`
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
	}
}

func EnsureFindingIDs(findings []Finding) int {
	seen := make(map[string]struct{}, len(findings))
	overwrote := 0
	for i := range findings {
		if EnsureFindingID(&findings[i]) {
			overwrote++
		}
		if _, ok := seen[findings[i].ID]; ok {
			findings[i].ID = newUniqueUUID(seen)
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

type FindingVerification struct {
	ID              string  `json:"id"`
	Verdict         string  `json:"verdict"`
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
	Title           string  `json:"title"`
	Body            string  `json:"body"`
	Priority        int     `json:"priority"`
	ConfidenceScore float64 `json:"confidence_score"`
	Remarks         string  `json:"remarks"`
}

// FindingSummarization carries the shortened, more readable body produced by the
// summarize pass. Its body is the only field the summarizer LLM emits; every
// other field is copied verbatim in code from the finding's FindingFinalization
// (see applySummarizedFinding in internal/review/summarizer.go), so a
// summarization always mirrors the finalization it shortens apart from Body.
type FindingSummarization struct {
	Title           string  `json:"title"`
	Body            string  `json:"body"`
	Priority        int     `json:"priority"`
	ConfidenceScore float64 `json:"confidence_score"`
	Remarks         string  `json:"remarks"`
}

type CodeLocation struct {
	FilePath  string    `json:"file_path"`
	LineRange LineRange `json:"line_range"`
}

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type Suggestion struct {
	Body      string    `json:"body"`
	LineRange LineRange `json:"line_range"`
}

func (s *Suggestion) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var body string
		if err := json.Unmarshal(trimmed, &body); err != nil {
			return err
		}
		s.Body = body
		s.LineRange = LineRange{}
		return nil
	}
	type suggestion Suggestion
	var parsed suggestion
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return err
	}
	*s = Suggestion(parsed)
	return nil
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

// ReviewPublisher is an optional capability for review sources that can post the
// finished review back to the origin (e.g. as GitLab MR comments). runReview
// type-asserts the source to this interface and only publishes when PostReview
// is set, so non-publishing sources (github, local) are unaffected.
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

type SimpleEstimator struct{}

func (SimpleEstimator) Estimate(text string) int {
	return len(text) / 4
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
	if src == nil {
		return nil
	}
	return &ReviewPromptPayload{
		Repository:          src.Repository,
		Identifier:          src.Identifier,
		Title:               src.Title,
		Description:         src.Description,
		Commits:             src.Commits,
		ChangedFiles:        src.ChangedFiles,
		DiffHunks:           src.DiffHunks,
		Comments:            src.Comments,
		SupplementalContext: src.SupplementalContext,
		ToolchainVersions:   src.ToolchainVersions,
		OmittedSections:     src.OmittedSections,
	}
}
