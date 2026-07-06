package retrieval

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dgrieser/nickpit/internal/retrieval/tsparser"
)

// rustBackend is the structural backend for Rust. Parsing is AST-based (see
// internal/retrieval/tsparser); it extracts `fn` symbols (free functions,
// methods inside impl/trait blocks) and builds a name-resolved call graph. It
// does not model Rust's module/trait resolution; calls resolve to a same-file
// definition first, then to an unambiguous repo-wide one.
type rustBackend struct{}

type rustSymbol struct {
	id        string
	name      string
	path      string
	startLine int
	endLine   int
	source    string
	calls     []tsparser.Call
	dynamic   bool
	hasError  bool
	nested    bool
}

type rustFile struct {
	path    string
	symbols map[string]*rustSymbol
	// byName maps function name -> symbol ids (declaration order); nested
	// functions are kept out so same-file resolution matches addressable
	// definitions.
	byName map[string][]string
}

var rustSupportedExts = map[string]struct{}{
	".rs": {},
}

func (rustBackend) language() string { return "rust" }

func (rustBackend) supportsExt(ext string) bool { return ext == ".rs" }

func (rustBackend) findSymbols(_ context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error) {
	if scope.IsFile {
		modules, err := parseRustFiles(repoRoot, []string{filepath.Join(repoRoot, filepath.FromSlash(scope.Path))})
		if err != nil {
			return nil, err
		}
		var out []*SymbolInfo
		for _, module := range modules {
			for _, symbol := range module.symbols {
				if symbol.name == name {
					out = append(out, rustSymbolInfo(symbol))
				}
			}
		}
		sortSymbolInfos(out)
		return out, nil
	}
	graph, err := rustGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.symbolsNamed(name, scope.Path), nil
}

func (rustBackend) findCallers(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := rustGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, true)
}

func (rustBackend) findCallees(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := rustGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, false)
}

func rustGraphCached(repoRoot string, hierScope lookupScope) (*staticGraph, error) {
	return buildStaticGraphCached("rust", repoRoot, hierScope, func() (*staticGraph, error) {
		return buildRustGraph(repoRoot, hierScope)
	})
}

func buildRustGraph(repoRoot string, scope lookupScope) (*staticGraph, error) {
	files, err := collectFilesByExt(repoRoot, scope, rustSupportedExts)
	if err != nil {
		return nil, err
	}
	modules, err := parseRustFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	global := map[string][]string{}
	for _, module := range modules {
		for name, ids := range module.byName {
			global[name] = append(global[name], ids...)
		}
	}
	graph := newStaticGraph("rust", repoRoot)
	for _, module := range modules {
		for _, symbol := range module.symbols {
			graph.addNode(symbol.id, staticNode{
				Name:      symbol.name,
				Path:      symbol.path,
				StartLine: symbol.startLine,
				EndLine:   symbol.endLine,
				Source:    symbol.source,
			})
		}
	}
	for _, module := range modules {
		for _, symbol := range module.symbols {
			lowConfidence := symbol.dynamic || symbol.hasError
			for _, call := range symbol.calls {
				if call.Kind != tsparser.CallBare {
					continue
				}
				if targetID, ok := resolveRustCall(module, global, call.Name); ok {
					graph.addEdge(symbol.id, targetID)
				}
			}
			if lowConfidence {
				graph.markLowConfidence(symbol.id)
			}
		}
	}
	return graph, nil
}

// parseRustFiles parses the given files into per-file symbol tables.
func parseRustFiles(repoRoot string, files []string) (map[string]*rustFile, error) {
	irs, err := parseIRFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	modules := make(map[string]*rustFile, len(irs))
	for rel, ir := range irs {
		module := &rustFile{
			path:    rel,
			symbols: map[string]*rustSymbol{},
			byName:  map[string][]string{},
		}
		for _, irSymbol := range ir.Symbols {
			symbol := &rustSymbol{
				id:        fmt.Sprintf("%s#%s@%d", rel, irSymbol.Name, irSymbol.StartLine),
				name:      irSymbol.Name,
				path:      rel,
				startLine: irSymbol.StartLine,
				endLine:   irSymbol.EndLine,
				source:    irSymbol.Source,
				calls:     irSymbol.Calls,
				dynamic:   irSymbol.Dynamic,
				hasError:  irSymbol.HasError,
				nested:    irSymbol.Nested,
			}
			module.symbols[symbol.id] = symbol
			if !symbol.nested {
				module.byName[symbol.name] = append(module.byName[symbol.name], symbol.id)
			}
		}
		modules[rel] = module
	}
	return modules, nil
}

// resolveRustCall maps a called name to a defined symbol, preferring a same-file
// definition, then an unambiguous repo-wide one. Ambiguous repo-wide names yield
// no edge (best-effort, avoids fabricating relationships).
func resolveRustCall(module *rustFile, global map[string][]string, name string) (string, bool) {
	if ids := module.byName[name]; len(ids) > 0 {
		return ids[0], true
	}
	if ids := global[name]; len(ids) == 1 {
		return ids[0], true
	}
	return "", false
}

func rustSymbolInfo(symbol *rustSymbol) *SymbolInfo {
	return &SymbolInfo{
		Name:      symbol.name,
		Path:      symbol.path,
		StartLine: symbol.startLine,
		EndLine:   symbol.endLine,
		Source:    symbol.source,
		Language:  "rust",
	}
}
