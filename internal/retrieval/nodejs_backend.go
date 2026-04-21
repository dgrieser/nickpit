package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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
	bodyLines []string
	exported  bool
}

type nodeFile struct {
	path        string
	imports     map[string]nodeImportBinding
	topLevel    map[string]string
	classMethod map[string]map[string]string
	symbols     map[string]*nodeSymbol
	exports     map[string]string
}

var (
	nodeFuncPattern        = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	nodeVarFuncPattern     = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s+)?(?:function\s*\(|\([^)]*\)\s*=>|[A-Za-z_$][A-Za-z0-9_$]*\s*=>)`)
	nodeClassPattern       = regexp.MustCompile(`^(?:export\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	nodeMethodPattern      = regexp.MustCompile(`^(?:async\s+)?(?:static\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	nodeImportNamedPattern = regexp.MustCompile(`^import\s+\{([^}]+)\}\s+from\s+["']([^"']+)["']`)
	nodeImportNSPattern    = regexp.MustCompile(`^import\s+\*\s+as\s+([A-Za-z_$][A-Za-z0-9_$]*)\s+from\s+["']([^"']+)["']`)
	nodeRequireObjPattern  = regexp.MustCompile(`^(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*require\(["']([^"']+)["']\)`)
	nodeRequireDestrPat    = regexp.MustCompile(`^(?:const|let|var)\s+\{([^}]+)\}\s*=\s*require\(["']([^"']+)["']\)`)
	nodeExportListPattern  = regexp.MustCompile(`^export\s+\{([^}]+)\}`)
	nodeExportsAssignPat   = regexp.MustCompile(`^(?:module\.)?exports\.([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*([A-Za-z_$][A-Za-z0-9_$]*)`)
	nodeModuleObjectPat    = regexp.MustCompile(`^module\.exports\s*=\s*\{([^}]+)\}`)
	nodeAttrCallPattern    = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\.([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	nodeBareCallPattern    = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	nodeDynamicCallPattern = regexp.MustCompile(`\)\s*\(|\[[^\]]+\]\s*\(`)
)

var nodeSupportedExts = map[string]struct{}{
	".js":  {},
	".mjs": {},
	".cjs": {},
	".ts":  {},
	".mts": {},
	".cts": {},
}

var nodeIgnoredCalls = map[string]struct{}{
	"if": {}, "for": {}, "while": {}, "switch": {}, "catch": {}, "require": {}, "console": {},
}

func (nodejsBackend) language() string { return "nodejs" }

func (nodejsBackend) supportsExt(ext string) bool {
	_, ok := nodeSupportedExts[ext]
	return ok
}

func (nodejsBackend) findSymbols(_ context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error) {
	files, err := collectFilesByExt(repoRoot, scope, nodeSupportedExts)
	if err != nil {
		return nil, err
	}
	modules, err := parseNodeFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	var out []*SymbolInfo
	for _, module := range modules {
		for _, symbolID := range module.topLevel {
			symbol := module.symbols[symbolID]
			if symbol.name == name {
				out = append(out, nodeSymbolInfo(symbol))
			}
		}
		for _, methods := range module.classMethod {
			for _, symbolID := range methods {
				symbol := module.symbols[symbolID]
				if symbol.name == name {
					out = append(out, nodeSymbolInfo(symbol))
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func (nodejsBackend) findCallers(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := buildNodeGraph(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, true)
}

func (nodejsBackend) findCallees(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := buildNodeGraph(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, false)
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
			lowConfidence := false
			for _, line := range symbol.bodyLines {
				clean := stripNodeLine(line)
				if strings.Contains(clean, "?.") || nodeDynamicCallPattern.MatchString(clean) {
					lowConfidence = true
				}
				for _, match := range nodeAttrCallPattern.FindAllStringSubmatch(clean, -1) {
					targetID, ok := resolveNodeAttrCall(modules, module, symbol, match[1], match[2])
					if ok {
						graph.addEdge(symbol.id, targetID)
						continue
					}
					if match[1] != "this" {
						lowConfidence = true
					}
				}
				for _, match := range nodeBareCallPattern.FindAllStringSubmatchIndex(clean, -1) {
					name := clean[match[2]:match[3]]
					if _, ignored := nodeIgnoredCalls[name]; ignored {
						continue
					}
					if match[0] > 0 && clean[match[0]-1] == '.' {
						continue
					}
					targetID, ok := resolveNodeBareCall(modules, module, name)
					if ok {
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

func parseNodeFiles(repoRoot string, files []string) (map[string]*nodeFile, error) {
	modules := make(map[string]*nodeFile, len(files))
	for _, fullPath := range files {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(repoRoot, fullPath)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		lines := splitLines(string(data))
		module := &nodeFile{
			path:        rel,
			imports:     map[string]nodeImportBinding{},
			topLevel:    map[string]string{},
			classMethod: map[string]map[string]string{},
			symbols:     map[string]*nodeSymbol{},
			exports:     map[string]string{},
		}
		for i := 0; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			if bindings, ok := parseNodeImports(repoRoot, rel, line); ok {
				for name, binding := range bindings {
					module.imports[name] = binding
				}
				continue
			}
			parseNodeExports(module, line)
			if match := nodeFuncPattern.FindStringSubmatch(line); len(match) == 2 {
				end := nodeBlockEnd(lines, i)
				exported := strings.HasPrefix(line, "export ")
				symbol := &nodeSymbol{
					id:        fmt.Sprintf("%s#%s@%d", rel, match[1], i+1),
					name:      match[1],
					path:      rel,
					startLine: i + 1,
					endLine:   end,
					source:    strings.Join(lines[i:end], "\n"),
					bodyLines: lines[i:end],
					exported:  exported,
				}
				module.symbols[symbol.id] = symbol
				module.topLevel[symbol.name] = symbol.id
				if exported {
					module.exports[symbol.name] = symbol.id
				}
				i = end - 1
				continue
			}
			if match := nodeVarFuncPattern.FindStringSubmatch(line); len(match) == 2 {
				end := nodeExpressionEnd(lines, i)
				exported := strings.HasPrefix(line, "export ")
				symbol := &nodeSymbol{
					id:        fmt.Sprintf("%s#%s@%d", rel, match[1], i+1),
					name:      match[1],
					path:      rel,
					startLine: i + 1,
					endLine:   end,
					source:    strings.Join(lines[i:end], "\n"),
					bodyLines: lines[i:end],
					exported:  exported,
				}
				module.symbols[symbol.id] = symbol
				module.topLevel[symbol.name] = symbol.id
				if exported {
					module.exports[symbol.name] = symbol.id
				}
				i = end - 1
				continue
			}
			if match := nodeClassPattern.FindStringSubmatch(line); len(match) == 2 {
				className := match[1]
				end := nodeBlockEnd(lines, i)
				if module.classMethod[className] == nil {
					module.classMethod[className] = map[string]string{}
				}
				classDepth := 0
				for j := i; j < end; j++ {
					if j == i {
						classDepth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
						continue
					}
					trimmed := strings.TrimSpace(lines[j])
					if trimmed == "" {
						classDepth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
						continue
					}
					if classDepth == 1 {
						if methodMatch := nodeMethodPattern.FindStringSubmatch(trimmed); len(methodMatch) == 2 && methodMatch[1] != "constructor" {
							methodEnd := nodeBlockEnd(lines, j)
							symbol := &nodeSymbol{
								id:        fmt.Sprintf("%s#%s.%s@%d", rel, className, methodMatch[1], j+1),
								name:      methodMatch[1],
								path:      rel,
								startLine: j + 1,
								endLine:   methodEnd,
								source:    strings.Join(lines[j:methodEnd], "\n"),
								className: className,
								bodyLines: lines[j:methodEnd],
							}
							module.symbols[symbol.id] = symbol
							module.classMethod[className][symbol.name] = symbol.id
							j = methodEnd - 1
							classDepth = 1
							continue
						}
					}
					classDepth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				}
				i = end - 1
			}
		}
		modules[rel] = module
	}
	return modules, nil
}

func parseNodeImports(repoRoot, currentPath, line string) (map[string]nodeImportBinding, bool) {
	if match := nodeImportNamedPattern.FindStringSubmatch(line); len(match) == 3 {
		modulePath, ok := resolveNodeModulePath(repoRoot, currentPath, match[2])
		if !ok {
			return nil, true
		}
		out := map[string]nodeImportBinding{}
		for _, rawPart := range strings.Split(match[1], ",") {
			part := strings.TrimSpace(rawPart)
			name := part
			alias := ""
			if strings.Contains(part, " as ") {
				chunks := strings.SplitN(part, " as ", 2)
				name = strings.TrimSpace(chunks[0])
				alias = strings.TrimSpace(chunks[1])
			}
			if alias == "" {
				alias = name
			}
			out[alias] = nodeImportBinding{kind: "symbol", modulePath: modulePath, symbolName: name}
		}
		return out, true
	}
	if match := nodeImportNSPattern.FindStringSubmatch(line); len(match) == 3 {
		modulePath, ok := resolveNodeModulePath(repoRoot, currentPath, match[2])
		if !ok {
			return nil, true
		}
		return map[string]nodeImportBinding{
			match[1]: {kind: "module", modulePath: modulePath},
		}, true
	}
	if match := nodeRequireObjPattern.FindStringSubmatch(line); len(match) == 3 {
		modulePath, ok := resolveNodeModulePath(repoRoot, currentPath, match[2])
		if !ok {
			return nil, true
		}
		return map[string]nodeImportBinding{
			match[1]: {kind: "module", modulePath: modulePath},
		}, true
	}
	if match := nodeRequireDestrPat.FindStringSubmatch(line); len(match) == 3 {
		modulePath, ok := resolveNodeModulePath(repoRoot, currentPath, match[2])
		if !ok {
			return nil, true
		}
		out := map[string]nodeImportBinding{}
		for _, rawPart := range strings.Split(match[1], ",") {
			part := strings.TrimSpace(rawPart)
			name := part
			alias := ""
			if strings.Contains(part, ":") {
				chunks := strings.SplitN(part, ":", 2)
				name = strings.TrimSpace(chunks[0])
				alias = strings.TrimSpace(chunks[1])
			}
			if alias == "" {
				alias = name
			}
			out[alias] = nodeImportBinding{kind: "symbol", modulePath: modulePath, symbolName: name}
		}
		return out, true
	}
	return nil, false
}

func parseNodeExports(module *nodeFile, line string) {
	if match := nodeExportListPattern.FindStringSubmatch(line); len(match) == 2 {
		for _, rawPart := range strings.Split(match[1], ",") {
			part := strings.TrimSpace(rawPart)
			name := part
			exportName := part
			if strings.Contains(part, " as ") {
				chunks := strings.SplitN(part, " as ", 2)
				name = strings.TrimSpace(chunks[0])
				exportName = strings.TrimSpace(chunks[1])
			}
			if symbolID, ok := module.topLevel[name]; ok {
				module.exports[exportName] = symbolID
			}
		}
	}
	if match := nodeExportsAssignPat.FindStringSubmatch(line); len(match) == 3 {
		if symbolID, ok := module.topLevel[match[2]]; ok {
			module.exports[match[1]] = symbolID
		}
	}
	if match := nodeModuleObjectPat.FindStringSubmatch(line); len(match) == 2 {
		for _, rawPart := range strings.Split(match[1], ",") {
			part := strings.TrimSpace(rawPart)
			name := part
			exportName := part
			if strings.Contains(part, ":") {
				chunks := strings.SplitN(part, ":", 2)
				exportName = strings.TrimSpace(chunks[0])
				name = strings.TrimSpace(chunks[1])
			}
			if symbolID, ok := module.topLevel[name]; ok {
				module.exports[exportName] = symbolID
			}
		}
	}
}

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
		filepath.ToSlash(joined) + ".ts",
		filepath.ToSlash(joined) + ".mts",
		filepath.ToSlash(joined) + ".cts",
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

func resolveNodeBareCall(modules map[string]*nodeFile, module *nodeFile, name string) (string, bool) {
	if targetID, ok := module.topLevel[name]; ok {
		return targetID, true
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

func resolveNodeAttrCall(modules map[string]*nodeFile, module *nodeFile, symbol *nodeSymbol, base, name string) (string, bool) {
	if base == "this" {
		if symbol.className == "" {
			return "", false
		}
		methods := module.classMethod[symbol.className]
		targetID, ok := methods[name]
		return targetID, ok
	}
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

func nodeBlockEnd(lines []string, start int) int {
	balance := 0
	seenOpen := false
	for i := start; i < len(lines); i++ {
		line := stripNodeLine(lines[i])
		balance += strings.Count(line, "{")
		if strings.Contains(line, "{") {
			seenOpen = true
		}
		balance -= strings.Count(line, "}")
		if seenOpen && balance <= 0 {
			return i + 1
		}
	}
	return len(lines)
}

func nodeExpressionEnd(lines []string, start int) int {
	line := stripNodeLine(lines[start])
	if strings.Contains(line, "{") {
		return nodeBlockEnd(lines, start)
	}
	return start + 1
}

func stripNodeLine(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		return line[:idx]
	}
	return line
}
