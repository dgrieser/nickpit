package retrieval

import "context"

type AdjacencyMode int

const (
	SameDir AdjacencyMode = iota
	Imports
	Siblings
)

type Engine interface {
	GetFile(ctx context.Context, repoRoot, path string) (*FileContent, error)
	GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error)
	GetAdjacentFiles(ctx context.Context, repoRoot, path string, mode AdjacencyMode) ([]FileRef, error)
	GetSymbol(ctx context.Context, repoRoot string, symbol string) (*SymbolInfo, error)
	ExpandFunctions(ctx context.Context, repoRoot string, refs []FunctionRef, depth int) (*FunctionBundle, error)
	FindCallers(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
	FindCallees(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error)
}

type FileContent struct {
	Path     string
	Lines    []string
	LineMap  map[int]int
	Language string
}

type FileSlice struct {
	Path      string
	StartLine int
	EndLine   int
	Lines     []string
	Language  string
}

type FileRef struct {
	Path string `json:"path"`
}

type SymbolInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Source    string `json:"source"`
	Language  string `json:"language"`
}

type FunctionRef struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type FunctionBundle struct {
	Functions []SymbolInfo `json:"functions"`
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
	Children  []CallNode `json:"children,omitempty"`
}
