package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
)

type languageBackend interface {
	language() string
	supportsExt(ext string) bool
	findSymbols(ctx context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error)
	findCallers(ctx context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error)
	findCallees(ctx context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error)
}

type lookupScope struct {
	Path   string
	IsFile bool
	IsDir  bool
}

type resolvedSymbol struct {
	info    *SymbolInfo
	backend languageBackend
}

type lowConfidenceError struct {
	language string
}

func (e *lowConfidenceError) Error() string {
	return fmt.Sprintf("symbol found but call hierarchy could not be resolved confidently for %s", e.language)
}

func languageBackends() []languageBackend {
	return []languageBackend{
		goBackend{},
		pythonBackend{},
		nodejsBackend{},
	}
}

func resolveLookupScope(repoRoot, path string) (lookupScope, error) {
	normalized, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return lookupScope{}, err
	}
	if normalized == "" {
		return lookupScope{}, nil
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return lookupScope{}, fmt.Errorf("stat %q: %w", normalized, err)
	}
	return lookupScope{
		Path:   normalized,
		IsFile: !info.IsDir(),
		IsDir:  info.IsDir(),
	}, nil
}

func candidateBackends(scope lookupScope) ([]languageBackend, error) {
	backends := languageBackends()
	if !scope.IsFile {
		return backends, nil
	}

	ext := strings.ToLower(filepath.Ext(scope.Path))
	for _, backend := range backends {
		if backend.supportsExt(ext) {
			return []languageBackend{backend}, nil
		}
	}
	return nil, fmt.Errorf("no retrieval backend supports %q", scope.Path)
}

func resolveSymbol(ctx context.Context, repoRoot string, symbol SymbolRef) (*resolvedSymbol, error) {
	scope, err := resolveLookupScope(repoRoot, symbol.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving %q: %w", symbol.Path, err)
	}
	backends, err := candidateBackends(scope)
	if err != nil {
		return nil, err
	}

	var matches []resolvedSymbol
	for _, backend := range backends {
		symbols, err := backend.findSymbols(ctx, repoRoot, symbol.Name, scope)
		if err != nil {
			return nil, err
		}
		for _, info := range symbols {
			matches = append(matches, resolvedSymbol{info: info, backend: backend})
		}
	}

	if len(matches) == 0 {
		if scope.Path != "" {
			return nil, fmt.Errorf("symbol %q not found in %q", symbol.Name, scope.Path)
		}
		return nil, fmt.Errorf("symbol %q not found", symbol.Name)
	}

	sort.Slice(matches, func(i, j int) bool {
		left := matches[i].info
		right := matches[j].info
		if left.Path == right.Path {
			if left.StartLine == right.StartLine {
				return left.Name < right.Name
			}
			return left.StartLine < right.StartLine
		}
		return left.Path < right.Path
	})
	return &matches[0], nil
}

type goBackend struct{}

func (goBackend) language() string { return "go" }

func (goBackend) supportsExt(ext string) bool { return ext == ".go" }

func (goBackend) findSymbols(ctx context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error) {
	symbols, err := goparser.FindSymbols(ctx, repoRoot, name, scope.Path)
	if err != nil {
		return nil, err
	}
	out := make([]*SymbolInfo, 0, len(symbols))
	for _, symbol := range symbols {
		s := symbol
		out = append(out, &SymbolInfo{
			Name:      s.Name,
			Path:      s.Path,
			StartLine: s.StartLine,
			EndLine:   s.EndLine,
			Source:    s.Source,
			Language:  "go",
		})
	}
	return out, nil
}

func (goBackend) findCallers(ctx context.Context, repoRoot string, symbol *SymbolInfo, _ lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := goparser.BuildGraph(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("building go call graph: %w", err)
	}
	hierarchy, err := graph.Find(symbol.Name, symbol.Path, depth, true)
	if err != nil {
		return nil, err
	}
	return convertHierarchy(hierarchy), nil
}

func (goBackend) findCallees(ctx context.Context, repoRoot string, symbol *SymbolInfo, _ lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := goparser.BuildGraph(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("building go call graph: %w", err)
	}
	hierarchy, err := graph.Find(symbol.Name, symbol.Path, depth, false)
	if err != nil {
		return nil, err
	}
	return convertHierarchy(hierarchy), nil
}
