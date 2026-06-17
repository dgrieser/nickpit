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

type LowConfidenceError struct {
	language string
}

func (e *LowConfidenceError) Error() string {
	return fmt.Sprintf("symbol found but call hierarchy could not be resolved confidently for %s", e.language)
}

// UnsupportedLanguageError reports that no structural retrieval backend (symbol
// resolution / call graph) recognizes the target file's language. It is distinct
// from "symbol not found": the code may well exist, it simply cannot be analyzed
// structurally, so callers should fall back to literal file inspection/search
// instead of treating it as evidence the symbol is absent.
type UnsupportedLanguageError struct {
	Path string
}

func (e *UnsupportedLanguageError) Error() string {
	return fmt.Sprintf("no structural retrieval backend supports %q", e.Path)
}

func languageBackends() []languageBackend {
	return []languageBackend{
		goBackend{},
		pythonBackend{},
		nodejsBackend{},
		rustBackend{},
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
	return nil, &UnsupportedLanguageError{Path: scope.Path}
}

// SupportsStructuralAnalysis reports whether a structural retrieval backend
// (symbol resolution / call graph) exists for path within repoRoot. A concrete
// file in an unsupported language returns false; a directory or the empty
// (repo-wide) scope returns true, since at least one backend can attempt a
// repo-wide lookup. It mirrors the resolution candidateBackends performs, so the
// search-tool optimization only rewrites a function-name search into a call-graph
// lookup when that lookup can actually resolve the target language.
func SupportsStructuralAnalysis(repoRoot, path string) bool {
	scope, err := resolveLookupScope(repoRoot, path)
	if err != nil {
		return false
	}
	_, err = candidateBackends(scope)
	return err == nil
}

// FallbackSearchScope returns the repo-relative path a literal search should use
// when call-hierarchy analysis is unavailable for path. It mirrors scopeForHierarchy:
// a single file widens to the repo-wide scope ("") so callers/uses in other files are
// still found; a directory (or the already repo-wide scope) is kept as-is. Used by the
// review layer to degrade find_callers/find_callees into a literal search for
// languages with no structural backend instead of failing.
func FallbackSearchScope(repoRoot, path string) string {
	scope, err := resolveLookupScope(repoRoot, path)
	if err != nil {
		return path
	}
	if scope.IsFile {
		return ""
	}
	return scope.Path
}

// scopeForHierarchy converts a lookup scope into the scope used to build a call
// graph. A file scope is widened to repo-wide (the empty scope), because call
// hierarchy traversal is not meaningful when restricted to a single file's
// definitions; a directory scope is kept as-is. Shared by the regex backends
// (python/node/rust).
func scopeForHierarchy(scope lookupScope) lookupScope {
	if scope.IsDir {
		return scope
	}
	return lookupScope{}
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
	graph, err := goparser.BuildGraphCached(ctx, repoRoot)
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
	graph, err := goparser.BuildGraphCached(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("building go call graph: %w", err)
	}
	hierarchy, err := graph.Find(symbol.Name, symbol.Path, depth, false)
	if err != nil {
		return nil, err
	}
	return convertHierarchy(hierarchy), nil
}
