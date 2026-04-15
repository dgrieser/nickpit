package model

import (
	"context"
	"encoding/json"
	"time"
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
	Mode                     ReviewMode
	RepoRoot                 string
	LocalRepo                string
	Repo                     string
	Identifier               int
	BaseRef                  string
	HeadRef                  string
	IncludeComments          bool
	IncludeCommits           bool
	IncludeFullFiles         bool
	MaxContextTokens         int
	FollowUpRounds           int
	PriorityThreshold        string
	ReviewSystemPromptFile   string
	ReviewUserPromptFile     string
	FollowUpSystemPromptFile string
	FollowUpUserPromptFile   string
	Offline                  bool
	Submode                  string
}

type ReviewResult struct {
	Findings               []Finding  `json:"findings"`
	OverallCorrectness     string     `json:"overall_correctness"`
	OverallExplanation     string     `json:"overall_explanation"`
	OverallConfidenceScore float64    `json:"overall_confidence_score"`
	TokensUsed             TokenUsage `json:"tokens_used,omitempty"`
	Model                  string     `json:"model,omitempty"`
	Mode                   string     `json:"mode,omitempty"`
	Repo                   string     `json:"repo,omitempty"`
	Identifier             int        `json:"identifier,omitempty"`
	FollowUpRound          int        `json:"followup_rounds,omitempty"`
}

type ReviewContext struct {
	Mode                ReviewMode         `json:"mode"`
	CheckoutRoot        string             `json:"checkout_root,omitempty"`
	Repository          RepositoryInfo     `json:"repository"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	Commits             []CommitSummary    `json:"commits,omitempty"`
	ChangedFiles        []ChangedFile      `json:"changed_files"`
	Diff                string             `json:"diff"`
	DiffHunks           []DiffHunk         `json:"diff_hunks,omitempty"`
	Comments            []Comment          `json:"comments,omitempty"`
	SupplementalContext []SupplementalFile `json:"supplemental_context,omitempty"`
	OmittedSections     []string           `json:"omitted_sections,omitempty"`
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
	OldStart int    `json:"old_start"`
	OldLines int    `json:"old_lines"`
	NewStart int    `json:"new_start"`
	NewLines int    `json:"new_lines"`
	Content  string `json:"content"`
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
	Title           string       `json:"title"`
	Body            string       `json:"body"`
	ConfidenceScore float64      `json:"confidence_score"`
	Priority        *int         `json:"priority,omitempty"`
	CodeLocation    CodeLocation `json:"code_location"`
}

type CodeLocation struct {
	AbsoluteFilePath string    `json:"absolute_file_path"`
	LineRange        LineRange `json:"line_range"`
}

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type FollowUpRequest struct {
	Type      string `json:"type"`
	Path      string `json:"path,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Depth     int    `json:"depth,omitempty"`
	Reason    string `json:"reason"`
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

func CloneContext(src *ReviewContext) *ReviewContext {
	if src == nil {
		return nil
	}
	data, _ := json.Marshal(src)
	var out ReviewContext
	_ = json.Unmarshal(data, &out)
	return &out
}
