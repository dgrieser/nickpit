package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/tsparser"
)

// pythonBackend is the structural backend for Python. Parsing is AST-based
// (see internal/retrieval/tsparser); this file resolves the extracted IR
// across files (import bindings, class methods) into the shared static call
// graph.
type pythonBackend struct{}

type pythonImportBinding struct {
	kind       string
	modulePath string
	symbolName string
}

type pythonSymbol struct {
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
}

type pythonFile struct {
	path        string
	imports     map[string]pythonImportBinding
	topLevel    map[string]string
	classMethod map[string]map[string]string
	symbols     map[string]*pythonSymbol
	// byName lists symbol ids per name in declaration order, including nested
	// definitions (used as a same-file fallback for bare-call resolution).
	byName map[string][]string
}

var pythonSupportedExts = map[string]struct{}{".py": {}}

func (pythonBackend) language() string { return "python" }

func (pythonBackend) supportsExt(ext string) bool { return ext == ".py" }

func (pythonBackend) findSymbols(_ context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error) {
	if scope.IsFile {
		modules, err := parsePythonFiles(repoRoot, []string{filepath.Join(repoRoot, filepath.FromSlash(scope.Path))})
		if err != nil {
			return nil, err
		}
		var out []*SymbolInfo
		for _, module := range modules {
			for _, id := range module.byName[name] {
				out = append(out, pythonSymbolInfo(module.symbols[id]))
			}
		}
		sortSymbolInfos(out)
		return out, nil
	}
	graph, err := pythonGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.symbolsNamed(name, scope.Path), nil
}

func (pythonBackend) findCallers(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := pythonGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, true)
}

func (pythonBackend) findCallees(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := pythonGraphCached(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, false)
}

func pythonGraphCached(repoRoot string, hierScope lookupScope) (*staticGraph, error) {
	return buildStaticGraphCached("python", repoRoot, hierScope, func() (*staticGraph, error) {
		return buildPythonGraph(repoRoot, hierScope)
	})
}

func buildPythonGraph(repoRoot string, scope lookupScope) (*staticGraph, error) {
	files, err := collectFilesByExt(repoRoot, scope, pythonSupportedExts)
	if err != nil {
		return nil, err
	}
	modules, err := parsePythonFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	graph := newStaticGraph("python", repoRoot)
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
					// An unresolved member call (a method on a local value)
					// yields no edge but does not degrade confidence: the
					// parser attributes calls precisely, so only truly
					// dynamic constructs (symbol.dynamic) do.
					if targetID, ok := resolvePythonAttrCall(modules, module, call.Base, call.Name); ok {
						graph.addEdge(symbol.id, targetID)
					}
				case tsparser.CallBare:
					if targetID, ok := resolvePythonBareCall(modules, module, call.Name); ok {
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

// parsePythonFiles parses the given files into per-file symbol/import tables,
// resolving import specifiers to repository paths.
func parsePythonFiles(repoRoot string, files []string) (map[string]*pythonFile, error) {
	irs, err := parseIRFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	modules := make(map[string]*pythonFile, len(irs))
	for rel, ir := range irs {
		module := &pythonFile{
			path:        rel,
			imports:     map[string]pythonImportBinding{},
			topLevel:    map[string]string{},
			classMethod: map[string]map[string]string{},
			symbols:     map[string]*pythonSymbol{},
			byName:      map[string][]string{},
		}
		for _, irSymbol := range ir.Symbols {
			symbol := &pythonSymbol{
				name:      irSymbol.Name,
				path:      rel,
				startLine: irSymbol.StartLine,
				endLine:   irSymbol.EndLine,
				source:    irSymbol.Source,
				className: irSymbol.Container,
				calls:     irSymbol.Calls,
				dynamic:   irSymbol.Dynamic,
				hasError:  irSymbol.HasError,
				nested:    irSymbol.Nested,
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
				}
			}
			module.symbols[symbol.id] = symbol
			module.byName[symbol.name] = append(module.byName[symbol.name], symbol.id)
		}
		for _, irImport := range ir.Imports {
			modulePath, ok := resolvePythonModulePath(repoRoot, rel, irImport.ModuleSpec)
			if !ok {
				continue
			}
			module.imports[irImport.Alias] = pythonImportBinding{
				kind:       irImport.Kind,
				modulePath: modulePath,
				symbolName: irImport.SymbolName,
			}
		}
		modules[rel] = module
	}
	return modules, nil
}

// resolvePythonModulePath resolves a module spec (absolute "pkg.mod" or
// relative "..mod") to a repo-relative file path.
func resolvePythonModulePath(repoRoot, currentPath, module string) (string, bool) {
	dots := 0
	for dots < len(module) && module[dots] == '.' {
		dots++
	}
	trimmed := module[dots:]
	var baseDir string
	if dots > 0 {
		baseDir = filepath.Dir(currentPath)
		for i := 1; i < dots; i++ {
			baseDir = filepath.Dir(baseDir)
		}
	}
	rel := strings.ReplaceAll(trimmed, ".", "/")
	candidates := []string{}
	if baseDir != "" {
		if rel != "" {
			candidates = append(candidates, filepath.ToSlash(filepath.Join(baseDir, rel+".py")))
			candidates = append(candidates, filepath.ToSlash(filepath.Join(baseDir, rel, "__init__.py")))
		} else {
			candidates = append(candidates, filepath.ToSlash(filepath.Join(baseDir, "__init__.py")))
		}
	} else if rel != "" {
		candidates = append(candidates, rel+".py", filepath.ToSlash(filepath.Join(rel, "__init__.py")))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(candidate))); err == nil {
			return candidate, true
		}
	}
	return "", false
}

// resolvePythonAttrCall resolves a `base.name(...)` call through a
// whole-module import binding to the target module's top-level definitions
// (self./cls. calls are resolved by the caller via classMethod).
func resolvePythonAttrCall(modules map[string]*pythonFile, module *pythonFile, base, name string) (string, bool) {
	binding, ok := module.imports[base]
	if !ok || binding.kind != "module" {
		return "", false
	}
	targetModule, ok := modules[binding.modulePath]
	if !ok {
		return "", false
	}
	targetID, ok := targetModule.topLevel[name]
	return targetID, ok
}

// resolvePythonBareCall resolves a plain `name(...)` call: top-level
// definitions win, then same-file nested definitions, then imported symbols.
func resolvePythonBareCall(modules map[string]*pythonFile, module *pythonFile, name string) (string, bool) {
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
	targetID, ok := targetModule.topLevel[binding.symbolName]
	return targetID, ok
}

func pythonSymbolInfo(symbol *pythonSymbol) *SymbolInfo {
	return &SymbolInfo{
		Name:      symbol.name,
		Path:      symbol.path,
		StartLine: symbol.startLine,
		EndLine:   symbol.endLine,
		Source:    symbol.source,
		Language:  "python",
	}
}
