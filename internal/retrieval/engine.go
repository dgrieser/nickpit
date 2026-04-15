package retrieval

import "context"

type Engine interface {
	GetFile(ctx context.Context, repoRoot, path string) (*FileContent, error)
	GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error)
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
