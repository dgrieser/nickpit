package retrieval

import (
	"context"
	"regexp"
)

type Engine interface {
	GetFile(ctx context.Context, repoRoot, path string) (*FileContent, error)
	ListFiles(ctx context.Context, repoRoot, path string, depth int) (*DirectoryListing, error)
	GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error)
	FindLines(ctx context.Context, repoRoot, path, code string) (*FindLinesResult, error)
	Search(ctx context.Context, repoRoot, path, query string, contextLines, maxResults int, caseSensitive bool) (*SearchResults, error)
	SearchRegex(ctx context.Context, repoRoot, path string, pattern *regexp.Regexp, contextLines, maxResults int) (*SearchResults, error)
	GetSymbol(ctx context.Context, repoRoot string, symbol SymbolRef) (*SymbolInfo, error)
	FindReferences(ctx context.Context, repoRoot string, symbol SymbolRef) (*ReferenceResult, error)
	FindCallers(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
	FindCallees(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
}

type FileContent struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Language  string `json:"language"`
	Truncated bool   `json:"truncated,omitempty"`
}

type FileSlice struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
	Language  string `json:"language"`
	// Truncated is set when the requested range was clipped: the file ended
	// before the requested end line, or the slice hit the byte cap.
	Truncated bool `json:"truncated,omitempty"`
}

type DirectoryListing struct {
	Path  string   `json:"path"`
	Files []string `json:"files"`
}

type FindLinesResult struct {
	// Path is the resolved search scope: a file, a directory, or "" for the
	// whole repository when the caller omits a path.
	Path string `json:"path"`
	// Code is the code line or block that was searched for.
	Code       string           `json:"code"`
	MatchCount int              `json:"match_count"`
	Matches    []FindLinesMatch `json:"matches"`
	// TruncatedFiles lists scanned files whose content was clipped at the
	// per-file byte cap, so matches past the cap may be missing (mirrors
	// FileContent.Truncated for whole-file reads).
	TruncatedFiles []string `json:"truncated_files,omitempty"`
}

type FindLinesMatch struct {
	CodeLocation CodeLocation `json:"code_location"`
}

// CodeLocation is the canonical line-grounded location shape shared by the
// retrieval tools; it mirrors model.CodeLocation so tool results can be copied
// verbatim into findings and suggestions.
type CodeLocation struct {
	FilePath  string    `json:"file_path"`
	LineRange LineRange `json:"line_range"`
	Language  string    `json:"language"`
	// Content is the matched file text with its original indentation, so the
	// caller can verify the hit and reuse the snippet verbatim.
	Content string `json:"content"`
}

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
	Count int `json:"count"`
}

type SearchResults struct {
	Path          string         `json:"path"`
	Query         string         `json:"query"`
	ContextLines  int            `json:"context_lines"`
	MaxResults    int            `json:"max_results,omitempty"`
	CaseSensitive bool           `json:"case_sensitive,omitempty"`
	ResultCount   int            `json:"result_count"`
	Results       []SearchResult `json:"results"`
	// TruncatedFiles lists scanned files whose content was clipped at the
	// per-file byte cap, so matches past the cap may be missing (mirrors
	// FileContent.Truncated for whole-file reads).
	TruncatedFiles []string `json:"truncated_files,omitempty"`
}

// SearchResult locates one match: CodeLocation spans exactly the matched
// line(s) so it can be cited as-is, while the surrounding context requested
// via context_lines is carried separately and never widens the location.
type SearchResult struct {
	CodeLocation  CodeLocation   `json:"code_location"`
	ContextBefore *SearchContext `json:"context_before,omitempty"`
	ContextAfter  *SearchContext `json:"context_after,omitempty"`
}

type SearchContext struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

type SymbolInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Source    string `json:"source"`
	Language  string `json:"language"`
}

type SymbolRef struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

// ReferenceResult is a flat, declaration-centered view of a symbol. References
// inside functions are grouped so source is returned once even when a function
// mentions the symbol many times; package/module/class-level references are
// grouped by their containing statement or declaration.
type ReferenceResult struct {
	Target                 ReferenceTarget    `json:"target"`
	Functions              []ReferenceContext `json:"functions"`
	OutsideFunctions       []ReferenceContext `json:"outside_functions"`
	ExactReferenceCount    int                `json:"exact_reference_count"`
	PossibleReferenceCount int                `json:"possible_reference_count"`
	Complete               bool               `json:"complete"`
	Notes                  []string           `json:"notes,omitempty"`
}

type ReferenceTarget struct {
	Name       string       `json:"name"`
	Kind       string       `json:"kind"`
	Definition CodeLocation `json:"definition"`
}

type ReferenceContext struct {
	Name         string                `json:"name,omitempty"`
	CodeLocation CodeLocation          `json:"code_location"`
	References   []ReferenceOccurrence `json:"references"`
}

type ReferenceOccurrence struct {
	Role         string       `json:"role"`
	Confidence   string       `json:"confidence"`
	Column       int          `json:"column,omitempty"`
	CodeLocation CodeLocation `json:"code_location"`
}

type CallHierarchy struct {
	Root  CallNode `json:"root"`
	Mode  string   `json:"mode"`
	Depth int      `json:"depth"`
}

type CallNode struct {
	Name         string       `json:"name"`
	CodeLocation CodeLocation `json:"code_location"`
	Children     []CallNode   `json:"children"`
}
