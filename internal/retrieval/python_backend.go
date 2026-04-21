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
	bodyLines []string
}

type pythonFile struct {
	path        string
	imports     map[string]pythonImportBinding
	topLevel    map[string]string
	classMethod map[string]map[string]string
	symbols     map[string]*pythonSymbol
}

var (
	pythonClassPattern      = regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	pythonFuncPattern       = regexp.MustCompile(`^(?:async\s+def|def)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pythonImportPattern     = regexp.MustCompile(`^import\s+(.+)$`)
	pythonFromImportPattern = regexp.MustCompile(`^from\s+([\.A-Za-z0-9_]+)\s+import\s+(.+)$`)
	pythonAttrCallPattern   = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pythonBareCallPattern   = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pythonDynamicCallPat    = regexp.MustCompile(`\)\s*\(|\[[^\]]+\]\s*\(`)
)

var pythonIgnoredCalls = map[string]struct{}{
	"if": {}, "for": {}, "while": {}, "return": {}, "print": {}, "len": {}, "str": {}, "int": {}, "float": {},
	"dict": {}, "list": {}, "set": {}, "tuple": {}, "range": {}, "enumerate": {}, "super": {},
}

func (pythonBackend) language() string { return "python" }

func (pythonBackend) supportsExt(ext string) bool { return ext == ".py" }

func (pythonBackend) findSymbols(_ context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error) {
	files, err := collectFilesByExt(repoRoot, scope, map[string]struct{}{".py": {}})
	if err != nil {
		return nil, err
	}
	modules, err := parsePythonFiles(repoRoot, files)
	if err != nil {
		return nil, err
	}
	var out []*SymbolInfo
	for _, module := range modules {
		for _, symbolID := range module.topLevel {
			symbol := module.symbols[symbolID]
			if symbol.name != name {
				continue
			}
			out = append(out, pythonSymbolInfo(symbol))
		}
		for _, methods := range module.classMethod {
			for _, symbolID := range methods {
				symbol := module.symbols[symbolID]
				if symbol.name != name {
					continue
				}
				out = append(out, pythonSymbolInfo(symbol))
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

func (pythonBackend) findCallers(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := buildPythonGraph(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, true)
}

func (pythonBackend) findCallees(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	graph, err := buildPythonGraph(repoRoot, scopeForHierarchy(scope))
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, false)
}

func buildPythonGraph(repoRoot string, scope lookupScope) (*staticGraph, error) {
	files, err := collectFilesByExt(repoRoot, scope, map[string]struct{}{".py": {}})
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
			lowConfidence := false
			for _, line := range symbol.bodyLines {
				clean := stripPythonComment(line)
				if strings.Contains(clean, "getattr(") || pythonDynamicCallPat.MatchString(clean) {
					lowConfidence = true
				}
				for _, match := range pythonAttrCallPattern.FindAllStringSubmatch(clean, -1) {
					targetID, ok := resolvePythonAttrCall(modules, module, symbol, match[1], match[2])
					if ok {
						graph.addEdge(symbol.id, targetID)
						continue
					}
					if match[1] != "self" && match[1] != "cls" {
						lowConfidence = true
					}
				}
				for _, match := range pythonBareCallPattern.FindAllStringSubmatchIndex(clean, -1) {
					name := clean[match[2]:match[3]]
					if _, ignored := pythonIgnoredCalls[name]; ignored {
						continue
					}
					if match[0] > 0 && clean[match[0]-1] == '.' {
						continue
					}
					targetID, ok := resolvePythonBareCall(modules, module, name)
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

func parsePythonFiles(repoRoot string, files []string) (map[string]*pythonFile, error) {
	modules := make(map[string]*pythonFile, len(files))
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
		module := &pythonFile{
			path:        rel,
			imports:     map[string]pythonImportBinding{},
			topLevel:    map[string]string{},
			classMethod: map[string]map[string]string{},
			symbols:     map[string]*pythonSymbol{},
		}
		lines := splitLines(string(data))
		for i := 0; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			indent := pythonIndent(lines[i])
			if indent != 0 {
				continue
			}
			if binding, ok := parsePythonImport(repoRoot, rel, trimmed); ok {
				for name, target := range binding {
					module.imports[name] = target
				}
				continue
			}
			if match := pythonFuncPattern.FindStringSubmatch(trimmed); len(match) == 2 {
				end := pythonBlockEnd(lines, i, indent)
				symbol := &pythonSymbol{
					id:        fmt.Sprintf("%s#%s@%d", rel, match[1], i+1),
					name:      match[1],
					path:      rel,
					startLine: i + 1,
					endLine:   end,
					source:    strings.Join(lines[i:end], "\n"),
					bodyLines: lines[i:end],
				}
				module.symbols[symbol.id] = symbol
				module.topLevel[symbol.name] = symbol.id
				i = end - 1
				continue
			}
			if match := pythonClassPattern.FindStringSubmatch(trimmed); len(match) == 2 {
				className := match[1]
				classEnd := pythonBlockEnd(lines, i, indent)
				if module.classMethod[className] == nil {
					module.classMethod[className] = map[string]string{}
				}
				for j := i + 1; j < classEnd; j++ {
					classLine := strings.TrimSpace(lines[j])
					if classLine == "" || strings.HasPrefix(classLine, "#") {
						continue
					}
					if pythonIndent(lines[j]) != indent+4 {
						continue
					}
					if fnMatch := pythonFuncPattern.FindStringSubmatch(classLine); len(fnMatch) == 2 {
						end := pythonBlockEnd(lines, j, indent+4)
						symbol := &pythonSymbol{
							id:        fmt.Sprintf("%s#%s.%s@%d", rel, className, fnMatch[1], j+1),
							name:      fnMatch[1],
							path:      rel,
							startLine: j + 1,
							endLine:   end,
							source:    strings.Join(lines[j:end], "\n"),
							className: className,
							bodyLines: lines[j:end],
						}
						module.symbols[symbol.id] = symbol
						module.classMethod[className][symbol.name] = symbol.id
						j = end - 1
					}
				}
				i = classEnd - 1
			}
		}
		modules[rel] = module
	}
	return modules, nil
}

func parsePythonImport(repoRoot, currentPath, line string) (map[string]pythonImportBinding, bool) {
	if match := pythonImportPattern.FindStringSubmatch(line); len(match) == 2 {
		out := map[string]pythonImportBinding{}
		parts := strings.Split(match[1], ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			name := part
			alias := ""
			if strings.Contains(part, " as ") {
				chunks := strings.SplitN(part, " as ", 2)
				name = strings.TrimSpace(chunks[0])
				alias = strings.TrimSpace(chunks[1])
			}
			modulePath, ok := resolvePythonModulePath(repoRoot, currentPath, name)
			if !ok {
				continue
			}
			if alias == "" {
				pieces := strings.Split(name, ".")
				alias = pieces[len(pieces)-1]
			}
			out[alias] = pythonImportBinding{kind: "module", modulePath: modulePath}
		}
		return out, true
	}
	if match := pythonFromImportPattern.FindStringSubmatch(line); len(match) == 3 {
		modulePath, ok := resolvePythonModulePath(repoRoot, currentPath, strings.TrimSpace(match[1]))
		if !ok {
			return nil, true
		}
		out := map[string]pythonImportBinding{}
		for _, rawPart := range strings.Split(match[2], ",") {
			part := strings.TrimSpace(rawPart)
			if part == "*" || part == "" {
				continue
			}
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
			out[alias] = pythonImportBinding{kind: "symbol", modulePath: modulePath, symbolName: name}
		}
		return out, true
	}
	return nil, false
}

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

func resolvePythonAttrCall(modules map[string]*pythonFile, module *pythonFile, symbol *pythonSymbol, base, name string) (string, bool) {
	if base == "self" || base == "cls" {
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
	targetID, ok := targetModule.topLevel[name]
	return targetID, ok
}

func resolvePythonBareCall(modules map[string]*pythonFile, module *pythonFile, name string) (string, bool) {
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

func pythonIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
			continue
		}
		if ch == '\t' {
			count += 4
			continue
		}
		break
	}
	return count
}

func pythonBlockEnd(lines []string, start, indent int) int {
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if pythonIndent(lines[i]) <= indent {
			end = i
			break
		}
	}
	return end
}

func stripPythonComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func scopeForHierarchy(scope lookupScope) lookupScope {
	if scope.IsDir {
		return scope
	}
	return lookupScope{}
}
