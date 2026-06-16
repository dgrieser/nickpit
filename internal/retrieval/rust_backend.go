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

// rustBackend is a best-effort, regex-based structural backend for Rust, in the
// same spirit as pythonBackend/nodejsBackend: it extracts `fn` symbols (free
// functions, methods inside impl/trait blocks) and builds a name-resolved call
// graph. It does not model Rust's module/trait resolution; calls resolve to a
// same-file definition first, then to an unambiguous repo-wide one.
type rustBackend struct{}

type rustSymbol struct {
	id        string
	name      string
	path      string
	startLine int
	endLine   int
	source    string
	bodyLines []string
}

type rustFile struct {
	path    string
	symbols map[string]*rustSymbol
	byName  map[string][]string // function name -> symbol ids (declaration order)
}

var (
	// rustFnPattern matches a function declaration at the start of a (trimmed)
	// line, tolerating the canonical qualifier order:
	// pub / pub(crate) , default , const , async , unsafe , extern "abi" , fn.
	rustFnPattern = regexp.MustCompile(`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:default\s+)?(?:const\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+(?:"[^"]*"\s+)?)?fn\s+([A-Za-z_][A-Za-z0-9_]*)`)
	// rustCallPattern matches a call site `name(`. Because a `!` between the name
	// and `(` breaks the match, macro invocations (`println!(`) are naturally
	// excluded. Method/path calls (`self.foo(`, `Type::foo(`) match on the bare
	// trailing name, which is what we resolve by.
	rustCallPattern = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	// rustDynamicCallPattern flags genuinely indirect calls (immediately-invoked
	// closures / function pointers / indexed calls). Deliberately narrow so it does
	// not fire on ordinary closure arguments like `.map(|x| ...)`.
	rustDynamicCallPattern = regexp.MustCompile(`\)\s*\(|\]\s*\(`)
)

var rustSupportedExts = map[string]struct{}{
	".rs": {},
}

// rustIgnoredCalls are tokens that can appear as `name(` but are never a function
// we want an edge to (keywords and ubiquitous enum-variant constructors). Most
// unresolved names produce no edge anyway; this just skips the common cases.
var rustIgnoredCalls = map[string]struct{}{
	"if": {}, "while": {}, "for": {}, "match": {}, "return": {}, "fn": {}, "let": {},
	"loop": {}, "else": {}, "move": {}, "as": {}, "in": {}, "where": {}, "dyn": {},
	"Some": {}, "None": {}, "Ok": {}, "Err": {},
}

func (rustBackend) language() string { return "rust" }

func (rustBackend) supportsExt(ext string) bool { return ext == ".rs" }

func (rustBackend) findSymbols(_ context.Context, repoRoot, name string, scope lookupScope) ([]*SymbolInfo, error) {
	files, err := collectFilesByExt(repoRoot, scope, rustSupportedExts)
	if err != nil {
		return nil, err
	}
	modules, err := parseRustFiles(repoRoot, files)
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func (rustBackend) findCallers(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	hierScope := scopeForHierarchy(scope)
	graph, err := buildStaticGraphCached("rust", repoRoot, hierScope, func() (*staticGraph, error) {
		return buildRustGraph(repoRoot, hierScope)
	})
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, true)
}

func (rustBackend) findCallees(_ context.Context, repoRoot string, symbol *SymbolInfo, scope lookupScope, depth int) (*CallHierarchy, error) {
	hierScope := scopeForHierarchy(scope)
	graph, err := buildStaticGraphCached("rust", repoRoot, hierScope, func() (*staticGraph, error) {
		return buildRustGraph(repoRoot, hierScope)
	})
	if err != nil {
		return nil, err
	}
	return graph.find(symbol.Name, symbol.Path, depth, false)
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
		for _, symbol := range module.symbols {
			global[symbol.name] = append(global[symbol.name], symbol.id)
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
			lowConfidence := false
			for _, line := range symbol.bodyLines {
				clean := stripRustLine(line)
				if rustDynamicCallPattern.MatchString(clean) {
					lowConfidence = true
				}
				for _, match := range rustCallPattern.FindAllStringSubmatchIndex(clean, -1) {
					name := clean[match[2]:match[3]]
					if _, ignored := rustIgnoredCalls[name]; ignored {
						continue
					}
					targetID, ok := resolveRustCall(module, global, name)
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

func parseRustFiles(repoRoot string, files []string) (map[string]*rustFile, error) {
	modules := make(map[string]*rustFile, len(files))
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
		module := &rustFile{
			path:    rel,
			symbols: map[string]*rustSymbol{},
			byName:  map[string][]string{},
		}
		for i := 0; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			match := rustFnPattern.FindStringSubmatch(trimmed)
			if len(match) != 2 {
				continue
			}
			end := rustBlockEnd(lines, i)
			symbol := &rustSymbol{
				id:        fmt.Sprintf("%s#%s@%d", rel, match[1], i+1),
				name:      match[1],
				path:      rel,
				startLine: i + 1,
				endLine:   end,
				source:    strings.Join(lines[i:end], "\n"),
				bodyLines: lines[i:end],
			}
			module.symbols[symbol.id] = symbol
			module.byName[symbol.name] = append(module.byName[symbol.name], symbol.id)
			// Skip the body: nested `fn` items (rare) are intentionally not indexed,
			// matching the python/node backends. Methods inside an impl/trait block are
			// still reached because the enclosing block is not itself an `fn`.
			i = end - 1
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

// rustBlockEnd returns the 1-based line after the function body that begins at
// lines[start]. It brace-balances from the signature, and treats a `;` reached
// before any `{` (a bodyless trait-method declaration, `fn foo();`) as the end.
func rustBlockEnd(lines []string, start int) int {
	balance := 0
	seenOpen := false
	for i := start; i < len(lines); i++ {
		line := stripRustLine(lines[i])
		if !seenOpen {
			if idx := strings.IndexByte(line, ';'); idx >= 0 && !strings.Contains(line[:idx], "{") {
				return i + 1
			}
		}
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")
		balance += opens
		if opens > 0 {
			seenOpen = true
		}
		balance -= closes
		if seenOpen && balance <= 0 {
			return i + 1
		}
	}
	return len(lines)
}

// stripRustLine removes a trailing single-line // comment, ignoring // inside a
// double-quoted string ("http://x" is preserved) and honoring \-escapes. A
// complete char literal ('a', '\n', '"') is skipped so an embedded quote does
// not open a phantom string; lifetimes ('a, 'static) have no closing ' at the
// expected offset and fall through. It uses a dedicated scanner rather than the
// shared stripLineComment because Rust's ' is a char-literal/lifetime, not a
// string delimiter. NOT modeled (accepted best-effort gaps): raw strings
// (r#"..."#), /* */ block comments, and any construct spanning multiple lines —
// these need cross-line state and can skew rustBlockEnd's brace balance.
func stripRustLine(line string) string {
	inStr := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inStr:
			if c == '\\' {
				i++ // skip escaped char (\" \\ etc.)
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '\'':
			if i+3 < len(line) && line[i+1] == '\\' && line[i+3] == '\'' {
				i += 3 // escaped char literal: '\n' '\'' '\\'
			} else if i+2 < len(line) && line[i+2] == '\'' {
				i += 2 // simple char literal: 'a' '"'
			}
		case c == '/' && i+1 < len(line) && line[i+1] == '/':
			return line[:i]
		}
	}
	return line
}
