package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/tsparser"
)

// nodejsBackend is the structural backend for the JavaScript/TypeScript family.
// Parsing is AST-based (see internal/retrieval/tsparser); this file resolves
// the extracted IR across files (import bindings, export tables, class
// methods) into the shared static call graph.
type nodejsBackend struct{}

type nodeImportBinding struct {
	kind       string
	modulePath string
	symbolName string
}

type nodeSymbol struct {
	id        string
	name      string
	path      string
	startLine int
	endLine   int
	source    string
	className string
	calls     []tsparser.Call
	dynamic   bool
	hasError  bool
	nested    bool
	exported  bool
}

type nodeFile struct {
	path        string
	imports     map[string]nodeImportBinding
	topLevel    map[string]string
	classMethod map[string]map[string]string
	symbols     map[string]*nodeSymbol
	exports     map[string]string
	// byName lists symbol ids per name in declaration order, including nested
	// definitions (used as a same-file fallback for bare-call resolution).
	byName map[string][]string
}

var nodeSupportedExts = map[string]struct{}{
	".js":  {},
	".mjs": {},
	".cjs": {},
	".jsx": {},
	".ts":  {},
	".mts": {},
	".cts": {},
	".tsx": {},
}

func (nodejsBackend) language() string { return "nodejs" }

func (nodejsBackend) supportsExt(ext string) bool {
	_, ok := nodeSupportedExts[ext]
	return ok
}

func (nodejsBackend) findSymbols(_ context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error) {
	if scope.IsFile {
		modules, err := parseNodeFiles(repoRoot, []string{filepath.Join(repoRoot, filepath.FromSlash(scope.Path))})
		if err != nil {
			return nil, err
		}
		return nodeSymbolsNamed(modules, name), nil
	}
	graph, err := nodeGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.symbolsNamed(name, scope.Path), nil
}

func (nodejsBackend) findCallers(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := nodeGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, true)
}

func (nodejsBackend) findCallees(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := nodeGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, false)
}

func nodeGraphCached(repoRoot string, hierScope lookupScope) (*staticGraph, error) {
	return buildStaticGraphCached("nodejs", repoRoot, hierScope, func() (*staticGraph, error) {
		return buildNodeGraph(repoRoot, hierScope)
	})
}

func nodeSymbolsNamed(modules map[string]*nodeFile, name string) []*SymbolInfo {
	var out []*SymbolInfo
	for _, module := range modules {
		for _, id := range module.byName[name] {
			out = append(out, nodeSymbolInfo(module.symbols[id]))
		}
	}
	sortSymbolInfos(out)
	return out
}

func buildNodeGraph(repoRoot string, scope lookupScope) (*staticGraph, error) {
	files, err := collectFilesByExt(repoRoot, scope, nodeSupportedExts)
	if err != nil {
		return nil, err
	}
	modules, err := parseNodeFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	graph := newStaticGraph("nodejs", repoRoot)
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
				switch call.Kind {
				case tsparser.CallSelf:
					if symbol.className == "" {
						continue
					}
					if targetID, ok := module.classMethod[symbol.className][call.Name]; ok {
						graph.addEdge(symbol.id, targetID)
					}
				case tsparser.CallMember:
					// An unresolved member call (a method on a local value,
					// console.log, ...) yields no edge but does not degrade
					// confidence: the parser attributes calls precisely, so
					// only truly dynamic constructs (symbol.dynamic) do.
					if targetID, ok := resolveNodeAttrCall(modules, module, call.Base, call.Name); ok {
						graph.addEdge(symbol.id, targetID)
					}
				case tsparser.CallBare:
					if targetID, ok := resolveNodeBareCall(modules, module, call.Name); ok {
						graph.addEdge(symbol.id, targetID)
					}
				}
			}
			if lowConfidence {
				graph.markLowConfidence(symbol.id)
			}
		}
	}
	return graph, nil
}

// parseNodeFiles parses the given files into per-file symbol/import/export
// tables, resolving import specifiers to repository paths.
func parseNodeFiles(repoRoot string, files []string) (map[string]*nodeFile, error) {
	irs, err := parseIRFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	modules := make(map[string]*nodeFile, len(irs))
	for rel, ir := range irs {
		module := &nodeFile{
			path:        rel,
			imports:     map[string]nodeImportBinding{},
			topLevel:    map[string]string{},
			classMethod: map[string]map[string]string{},
			symbols:     map[string]*nodeSymbol{},
			exports:     map[string]string{},
			byName:      map[string][]string{},
		}
		for _, irSymbol := range ir.Symbols {
			symbol := &nodeSymbol{
				name:      irSymbol.Name,
				path:      rel,
				startLine: irSymbol.StartLine,
				endLine:   irSymbol.EndLine,
				source:    irSymbol.Source,
				className: irSymbol.Container,
				calls:     irSymbol.Calls,
				dynamic:   irSymbol.Dynamic,
				hasError:  irSymbol.HasError || ir.HasError,
				nested:    irSymbol.Nested,
				exported:  irSymbol.Exported,
			}
			if symbol.className != "" {
				symbol.id = fmt.Sprintf("%s#%s.%s@%d", rel, symbol.className, symbol.name, symbol.startLine)
				if module.classMethod[symbol.className] == nil {
					module.classMethod[symbol.className] = map[string]string{}
				}
				module.classMethod[symbol.className][symbol.name] = symbol.id
			} else {
				symbol.id = fmt.Sprintf("%s#%s@%d", rel, symbol.name, symbol.startLine)
				if !symbol.nested {
					module.topLevel[symbol.name] = symbol.id
					if symbol.exported {
						module.exports[symbol.name] = symbol.id
					}
				}
			}
			module.symbols[symbol.id] = symbol
			module.byName[symbol.name] = append(module.byName[symbol.name], symbol.id)
		}
		for _, irImport := range ir.Imports {
			modulePath, ok := resolveNodeModulePath(repoRoot, rel, irImport.ModuleSpec)
			if !ok {
				continue
			}
			module.imports[irImport.Alias] = nodeImportBinding{
				kind:       irImport.Kind,
				modulePath: modulePath,
				symbolName: irImport.SymbolName,
			}
		}
		for _, irExport := range ir.Exports {
			if targetID, ok := module.topLevel[irExport.LocalName]; ok {
				module.exports[irExport.ExportedName] = targetID
			}
		}
		modules[rel] = module
	}
	return modules, nil
}

// resolveNodeModulePath resolves a relative import specifier to a repo-relative
// file path; bare specifiers (packages) resolve to nothing.
func resolveNodeModulePath(repoRoot, currentPath, spec string) (string, bool) {
	if !strings.HasPrefix(spec, ".") {
		return "", false
	}
	base := filepath.Dir(currentPath)
	joined := filepath.Clean(filepath.Join(base, spec))
	candidates := []string{
		filepath.ToSlash(joined),
		filepath.ToSlash(joined) + ".js",
		filepath.ToSlash(joined) + ".mjs",
		filepath.ToSlash(joined) + ".cjs",
		filepath.ToSlash(joined) + ".jsx",
		filepath.ToSlash(joined) + ".ts",
		filepath.ToSlash(joined) + ".mts",
		filepath.ToSlash(joined) + ".cts",
		filepath.ToSlash(joined) + ".tsx",
		filepath.ToSlash(filepath.Join(joined, "index.js")),
		filepath.ToSlash(filepath.Join(joined, "index.ts")),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(candidate))); err == nil {
			if _, ok := nodeSupportedExts[filepath.Ext(candidate)]; ok {
				return candidate, true
			}
		}
	}
	return "", false
}

// resolveNodeBareCall resolves a plain `name(...)` call: top-level definitions
// win, then same-file nested definitions, then imported symbols.
func resolveNodeBareCall(modules map[string]*nodeFile, module *nodeFile, name string) (string, bool) {
	if targetID, ok := module.topLevel[name]; ok {
		return targetID, true
	}
	for _, id := range module.byName[name] {
		if symbol := module.symbols[id]; symbol.className == "" {
			return id, true
		}
	}
	binding, ok := module.imports[name]
	if !ok || binding.kind != "symbol" {
		return "", false
	}
	targetModule, ok := modules[binding.modulePath]
	if !ok {
		return "", false
	}
	targetID, ok := targetModule.exports[binding.symbolName]
	return targetID, ok
}

// resolveNodeAttrCall resolves a `base.name(...)` call through a whole-module
// import binding to the target module's exports.
func resolveNodeAttrCall(modules map[string]*nodeFile, module *nodeFile, base, name string) (string, bool) {
	binding, ok := module.imports[base]
	if !ok || binding.kind != "module" {
		return "", false
	}
	targetModule, ok := modules[binding.modulePath]
	if !ok {
		return "", false
	}
	targetID, ok := targetModule.exports[name]
	return targetID, ok
}

func nodeSymbolInfo(symbol *nodeSymbol) *SymbolInfo {
	return &SymbolInfo{
		Name:      symbol.name,
		Path:      symbol.path,
		StartLine: symbol.startLine,
		EndLine:   symbol.endLine,
		Source:    symbol.source,
		Language:  "nodejs",
	}
}
