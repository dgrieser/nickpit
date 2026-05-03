package retrieval

import (
	"context"
	"regexp"
)

type Engine interface {
	GetFile(ctx context.Context, repoRoot, path string) (*FileContent, error)
	ListFiles(ctx context.Context, repoRoot, path string, depth int) (*DirectoryListing, error)
	GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error)
	Search(ctx context.Context, repoRoot, path, query string, contextLines, maxResults int, caseSensitive bool) (*SearchResults, error)
	SearchRegex(ctx context.Context, repoRoot, path string, pattern *regexp.Regexp, contextLines, maxResults int) (*SearchResults, error)
	GetSymbol(ctx context.Context, repoRoot string, symbol SymbolRef) (*SymbolInfo, error)
	FindCallers(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
	FindCallees(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
}

type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
}

type FileSlice struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
	Language  string `json:"language"`
}

type DirectoryListing struct {
	Path  string   `json:"path"`
	Files []string `json:"files"`
}

type SearchResults struct {
	Path          string         `json:"path"`
	Query         string         `json:"query"`
	ContextLines  int            `json:"context_lines"`
	MaxResults    int            `json:"max_results,omitempty"`
	CaseSensitive bool           `json:"case_sensitive,omitempty"`
	ResultCount   int            `json:"result_count"`
	Results       []SearchResult `json:"results"`
}

type SearchResult struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Language  string `json:"language"`
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

type CallHierarchy struct {
	Root  CallNode `json:"root"`
	Mode  string   `json:"mode"`
	Depth int      `json:"depth"`
}

type CallNode struct {
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	Source    string     `json:"source"`
	Children  []CallNode `json:"children"`
}
