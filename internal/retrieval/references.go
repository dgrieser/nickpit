package retrieval

import (
	"context"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
	"github.com/dgrieser/nickpit/internal/retrieval/tsparser"
	"golang.org/x/tools/go/packages"
)

type AmbiguousSymbolError struct {
	Name       string
	Candidates []ReferenceTarget
}

func (e *AmbiguousSymbolError) Error() string {
	const maxCandidates = 10
	count := min(len(e.Candidates), maxCandidates)
	locations := make([]string, 0, count+1)
	for _, candidate := range e.Candidates[:count] {
		loc := candidate.Definition
		locations = append(locations, fmt.Sprintf("%s at %s:%d", candidate.Kind, loc.FilePath, loc.LineRange.Start))
	}
	if omitted := len(e.Candidates) - count; omitted > 0 {
		locations = append(locations, fmt.Sprintf("and %d more", omitted))
	}
	return fmt.Sprintf("symbol %q is ambiguous: %s", e.Name, strings.Join(locations, ", "))
}

// FindReferences resolves one declaration inside the optional lookup path and
// then searches the whole repository for references. Go uses go/types object
// identity. Other supported languages use their parser-derived function spans
// plus a conservative identifier index; uncertain matches are returned as
// possible instead of silently omitted.
func (e *LocalEngine) FindReferences(ctx context.Context, repoRoot string, symbol SymbolRef) (*ReferenceResult, error) {
	symbol.Name = strings.TrimSpace(symbol.Name)
	if symbol.Name == "" {
		return nil, fmt.Errorf("finding references: symbol name is empty")
	}
	scope, err := resolveLookupScope(repoRoot, symbol.Path)
	if err != nil {
		return nil, fmt.Errorf("finding references for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	if scope.IsFile {
		ext := strings.ToLower(filepath.Ext(scope.Path))
		if ext == ".go" {
			return findGoReferences(ctx, repoRoot, symbol, scope)
		}
		if _, ok := referenceExtensions[ext]; !ok {
			return nil, &UnsupportedLanguageError{Path: scope.Path}
		}
		return findParsedReferences(ctx, repoRoot, symbol, scope)
	}

	// A directory/repo lookup can span parser families. Resolve declarations
	// conservatively first; a selected Go declaration is then re-run through
	// go/types for exact references.
	result, err := findParsedReferences(ctx, repoRoot, symbol, scope)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(result.Target.Definition.FilePath), ".go") {
		return findGoReferences(ctx, repoRoot, SymbolRef{Name: symbol.Name, Path: result.Target.Definition.FilePath}, lookupScope{Path: result.Target.Definition.FilePath, IsFile: true})
	}
	return result, nil
}

var referenceExtensions = map[string]struct{}{
	".go": {}, ".py": {}, ".js": {}, ".mjs": {}, ".cjs": {}, ".jsx": {},
	".ts": {}, ".mts": {}, ".cts": {}, ".tsx": {}, ".rs": {},
}

type parsedReferenceFile struct {
	path      string
	language  string
	source    string
	lines     []string
	masked    []string
	functions []parsedFunction
	imports   []tsparser.Import
	exports   []tsparser.Export
}

type parsedFunction struct {
	name       string
	start, end int
}

type definitionCandidate struct {
	target ReferenceTarget
	file   *parsedReferenceFile
}

type parsedReferenceCacheEntry struct {
	mu          sync.Mutex
	fingerprint [sha256.Size]byte
	initialized bool
	files       []*parsedReferenceFile
}

var parsedReferenceCache sync.Map // absolute repo root -> *parsedReferenceCacheEntry

func findParsedReferences(ctx context.Context, repoRoot string, symbol SymbolRef, scope lookupScope) (*ReferenceResult, error) {
	parsed, err := loadParsedReferenceFiles(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("finding references for %q: %w", symbol.Name, err)
	}

	var candidates []definitionCandidate
	for _, file := range parsed {
		if !pathInLookupScope(file.path, scope) {
			continue
		}
		candidates = append(candidates, findDefinitionCandidates(file, symbol.Name)...)
	}
	if len(candidates) == 0 {
		if !scope.IsFile {
			// Directory/repository scopes may contain languages this structural
			// pass does not parse. Let callers perform their literal-search
			// fallback instead of incorrectly concluding the symbol is absent.
			return nil, &UnsupportedLanguageError{Path: scope.Path}
		}
		if scope.Path != "" {
			return nil, fmt.Errorf("symbol %q not found in %q", symbol.Name, scope.Path)
		}
		return nil, fmt.Errorf("symbol %q not found", symbol.Name)
	}
	candidates = dedupeDefinitionCandidates(candidates)
	if len(candidates) > 1 {
		targets := make([]ReferenceTarget, 0, len(candidates))
		for _, candidate := range candidates {
			targets = append(targets, candidate.target)
		}
		return nil, &AmbiguousSymbolError{Name: symbol.Name, Candidates: targets}
	}

	selected := candidates[0]
	aliases := referenceAliases(repoRoot, parsed, selected)
	result := &ReferenceResult{
		Target:           selected.target,
		Functions:        []ReferenceContext{},
		OutsideFunctions: []ReferenceContext{},
		Complete:         selected.file.language == "go",
	}
	if selected.file.language != "go" {
		result.Notes = []string{"dynamic-language references include conservative same-name candidates when binding identity cannot be proven"}
	}
	collectParsedOccurrences(result, parsed, selected, aliases)
	sortReferenceResult(result)
	return result, nil
}

func loadParsedReferenceFiles(ctx context.Context, repoRoot string) ([]*parsedReferenceFile, error) {
	key, err := filepath.Abs(repoRoot)
	if err != nil {
		key = repoRoot
	}
	actual, _ := parsedReferenceCache.LoadOrStore(key, &parsedReferenceCacheEntry{})
	entry := actual.(*parsedReferenceCacheEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	files, err := collectFilesByExt(repoRoot, lookupScope{}, referenceExtensions)
	if err != nil {
		return nil, err
	}
	fingerprint, sources, err := referenceSourceSnapshot(ctx, repoRoot, files)
	if err != nil {
		return nil, err
	}
	if entry.initialized && entry.fingerprint == fingerprint {
		return entry.files, nil
	}
	parsed := make([]*parsedReferenceFile, 0, len(files))
	for _, source := range sources {
		parsed = append(parsed, parseReferenceFile(source.path, string(source.data)))
	}
	entry.fingerprint = fingerprint
	entry.initialized = true
	entry.files = parsed
	return parsed, nil
}

type referenceSource struct {
	path string
	data []byte
}

func referenceSourceSnapshot(ctx context.Context, repoRoot string, files []string) ([sha256.Size]byte, []referenceSource, error) {
	hash := sha256.New()
	sources := make([]referenceSource, 0, len(files))
	for _, fullPath := range files {
		if err := ctx.Err(); err != nil {
			return [sha256.Size]byte{}, nil, err
		}
		rel, err := repofs.RelPath(repoRoot, fullPath)
		if err != nil {
			return [sha256.Size]byte{}, nil, err
		}
		data, err := repofs.ReadFile(repoRoot, fullPath)
		if err != nil {
			return [sha256.Size]byte{}, nil, err
		}
		fmt.Fprintf(hash, "%d:%s:%d:", len(rel), rel, len(data))
		_, _ = hash.Write(data)
		sources = append(sources, referenceSource{path: rel, data: data})
	}
	return [sha256.Size]byte(hash.Sum(nil)), sources, nil
}

func parseReferenceFile(path, source string) *parsedReferenceFile {
	language := detectLanguage(path)
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	file := &parsedReferenceFile{
		path: path, language: language, source: source, lines: lines,
		masked: maskReferenceSource(lines, language),
	}
	if ir, err := tsparser.ParseFile(path, []byte(source)); err == nil {
		file.imports = append(file.imports, ir.Imports...)
		file.exports = append(file.exports, ir.Exports...)
		for _, symbol := range ir.Symbols {
			file.functions = append(file.functions, parsedFunction{name: symbol.Name, start: symbol.StartLine, end: symbol.EndLine})
		}
	}
	return file
}

func pathInLookupScope(path string, scope lookupScope) bool {
	if scope.Path == "" {
		return true
	}
	if scope.IsFile {
		return path == scope.Path
	}
	return path == scope.Path || strings.HasPrefix(path, scope.Path+"/")
}

func findDefinitionCandidates(file *parsedReferenceFile, name string) []definitionCandidate {
	quoted := regexp.QuoteMeta(name)
	patterns := definitionPatterns(file.language, quoted)
	goGroupedDefinition := regexp.MustCompile(`^\s*` + quoted + `\b`)
	seenScope := map[string]struct{}{}
	var out []definitionCandidate
	goGroupKind := ""
	for i, line := range file.masked {
		lineNo := i + 1
		if file.language == "go" {
			trimmed := strings.TrimSpace(line)
			switch trimmed {
			case "const (":
				goGroupKind = "constant"
			case "var (":
				goGroupKind = "variable"
			case "type (":
				goGroupKind = "type"
			case ")":
				goGroupKind = ""
			default:
				if goGroupKind != "" && goGroupedDefinition.MatchString(line) {
					loc := lineLocation(file.path, file.language, file.lines, lineNo)
					out = append(out, definitionCandidate{target: ReferenceTarget{Name: name, Kind: goGroupKind, Definition: loc}, file: file})
					continue
				}
			}
		}
		for _, kind := range []string{"function", "type", "constant", "parameter", "field", "import", "variable"} {
			pattern, ok := patterns[kind]
			if !ok {
				continue
			}
			if !pattern.MatchString(line) {
				continue
			}
			fn := enclosingParsedFunction(file.functions, lineNo)
			// Python assignments rebind one lexical name; keep the first in a
			// function/module as the declaration and report later ones as writes.
			key := kind + ":module"
			if fn != nil {
				key = fmt.Sprintf("%s:%d", kind, fn.start)
			}
			if _, exists := seenScope[key]; exists && file.language == "python" && kind == "variable" {
				continue
			}
			seenScope[key] = struct{}{}
			loc := lineLocation(file.path, file.language, file.lines, lineNo)
			for _, symbol := range file.functions {
				if symbol.name == name && symbol.start == lineNo {
					loc = rangeLocation(file.path, file.language, file.lines, symbol.start, symbol.end)
					break
				}
			}
			out = append(out, definitionCandidate{target: ReferenceTarget{Name: name, Kind: kind, Definition: loc}, file: file})
			break
		}
	}
	return out
}

func definitionPatterns(language, name string) map[string]*regexp.Regexp {
	word := `\b` + name + `\b`
	compile := func(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }
	switch language {
	case "python":
		return map[string]*regexp.Regexp{
			"function":  compile(`^\s*(?:async\s+)?def\s+` + word),
			"type":      compile(`^\s*class\s+` + word),
			"import":    compile(`^\s*(?:from\s+\S+\s+)?import\b.*(?:\bas\s+)?` + word),
			"parameter": compile(`^\s*(?:async\s+)?def\b[^:]*\([^)]*` + word + `(?:\s*[:=,)]|$)`),
			"field":     compile(`\b(?:self|cls)\.` + name + `\s*(?::[^=]+)?=`),
			"variable":  compile(`^\s*` + word + `\s*(?::[^=]+)?(?:=|:)`),
		}
	case "nodejs":
		return map[string]*regexp.Regexp{
			"function":  compile(`\bfunction\s+` + word + `|\b(?:const|let|var)\s+` + word + `\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>`),
			"type":      compile(`\b(?:class|interface|type|enum|namespace)\s+` + word),
			"import":    compile(`^\s*import\b.*` + word + `|\brequire\s*\([^)]*\).*` + word),
			"parameter": compile(`\b(?:function\b[^()]*)?\([^)]*` + word + `(?:\s*[:=,)]|$)`),
			"variable":  compile(`\b(?:const|let|var)\s+` + word),
			"field":     compile(`^\s*(?:public\s+|private\s+|protected\s+|static\s+|readonly\s+)*` + word + `\s*(?:[?:=!]|$)`),
		}
	case "rust":
		return map[string]*regexp.Regexp{
			"function":  compile(`\bfn\s+` + word),
			"type":      compile(`\b(?:struct|enum|trait|type|mod)\s+` + word),
			"constant":  compile(`\b(?:const|static)\s+(?:mut\s+)?` + word),
			"variable":  compile(`\blet\s+(?:mut\s+)?` + word),
			"parameter": compile(`\bfn\b[^{}]*\([^)]*` + word + `\s*:`),
			"field":     compile(`^\s*(?:pub(?:\([^)]*\))?\s+)?` + word + `\s*:`),
			"import":    compile(`^\s*use\b.*(?:\bas\s+)?` + word),
		}
	default: // Go is used only to select a language in repo-wide lookup.
		return map[string]*regexp.Regexp{
			"function":  compile(`\bfunc\s+(?:\([^)]*\)\s*)?` + word + `\s*\(`),
			"type":      compile(`\btype\s+` + word),
			"constant":  compile(`\bconst\s+` + word),
			"variable":  compile(`\bvar\s+` + word + `|\b` + word + `\s*:=`),
			"parameter": compile(`\b` + word + `\s+(?:\*|\[|map\[|chan\s+|func\b|interface\b|[A-Za-z_])`),
		}
	}
}

func dedupeDefinitionCandidates(in []definitionCandidate) []definitionCandidate {
	seen := map[string]struct{}{}
	out := make([]definitionCandidate, 0, len(in))
	for _, candidate := range in {
		loc := candidate.target.Definition
		key := fmt.Sprintf("%s:%d:%s", loc.FilePath, loc.LineRange.Start, candidate.target.Kind)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		li, lj := out[i].target.Definition, out[j].target.Definition
		if li.FilePath == lj.FilePath {
			return li.LineRange.Start < lj.LineRange.Start
		}
		return li.FilePath < lj.FilePath
	})
	return out
}

// referenceAliases follows direct repository-local symbol imports. Existing
// module resolvers handle extension substitution and Python package paths.
func referenceAliases(repoRoot string, files []*parsedReferenceFile, selected definitionCandidate) map[string]map[string]string {
	locals := map[string]map[string]string{selected.file.path: {selected.target.Name: "exact"}}
	exported := map[string]map[string]string{}
	if selected.file.language == "python" {
		exported[selected.file.path] = map[string]string{selected.target.Name: "exact"}
	}
	for _, export := range selected.file.exports {
		if export.LocalName == selected.target.Name {
			if exported[selected.file.path] == nil {
				exported[selected.file.path] = map[string]string{}
			}
			exported[selected.file.path][export.ExportedName] = "exact"
		}
	}
	for changed := true; changed; {
		changed = false
		for _, file := range files {
			for _, binding := range file.imports {
				if binding.Kind != "symbol" {
					continue
				}
				resolved, ok := resolveReferenceImport(repoRoot, file, binding.ModuleSpec)
				if !ok || exported[resolved][binding.SymbolName] == "" {
					continue
				}
				if locals[file.path] == nil {
					locals[file.path] = map[string]string{}
				}
				if locals[file.path][binding.Alias] == "" {
					locals[file.path][binding.Alias] = "exact"
					changed = true
				}
			}
			if file.language == "python" {
				if exported[file.path] == nil {
					exported[file.path] = map[string]string{}
				}
				for name, confidence := range locals[file.path] {
					if exported[file.path][name] == "" {
						exported[file.path][name] = confidence
						changed = true
					}
				}
			}
			for _, export := range file.exports {
				confidence := locals[file.path][export.LocalName]
				if confidence == "" {
					continue
				}
				if exported[file.path] == nil {
					exported[file.path] = map[string]string{}
				}
				if exported[file.path][export.ExportedName] == "" {
					exported[file.path][export.ExportedName] = confidence
					changed = true
				}
			}
		}
	}
	// Rust module resolution is intentionally conservative in the existing
	// backend. Preserve renamed `use` bindings as possible references instead
	// of dropping them when a module cannot be proven.
	rustAliasPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(selected.target.Name) + `\b(?:\s+as\s+([A-Za-z_][A-Za-z0-9_]*))?`)
	for _, file := range files {
		if file.language != "rust" {
			continue
		}
		for _, line := range file.masked {
			if !strings.HasPrefix(strings.TrimSpace(line), "use ") {
				continue
			}
			match := rustAliasPattern.FindStringSubmatch(line)
			if len(match) == 2 && match[1] != "" {
				if locals[file.path] == nil {
					locals[file.path] = map[string]string{}
				}
				locals[file.path][match[1]] = "possible"
			}
		}
	}
	delete(locals[selected.file.path], selected.target.Name)
	return locals
}

func resolveReferenceImport(repoRoot string, file *parsedReferenceFile, spec string) (string, bool) {
	switch file.language {
	case "nodejs":
		return resolveNodeModulePath(repoRoot, file.path, spec)
	case "python":
		return resolvePythonModulePath(repoRoot, file.path, spec)
	default:
		return "", false
	}
}

func collectParsedOccurrences(result *ReferenceResult, files []*parsedReferenceFile, selected definitionCandidate, aliases map[string]map[string]string) {
	functionContexts := map[string]*ReferenceContext{}
	outsideContexts := map[string]*ReferenceContext{}
	for _, file := range files {
		names := map[string]string{selected.target.Name: "possible"}
		if selected.file.language == "go" && file.path == selected.file.path {
			names[selected.target.Name] = "exact"
		}
		maps.Copy(names, aliases[file.path])
		for lineIndex, line := range file.masked {
			lineNo := lineIndex + 1
			for name, confidence := range names {
				for _, column := range identifierColumns(line, name) {
					if file.path == selected.file.path && lineNo == selected.target.Definition.LineRange.Start && name == selected.target.Name {
						// Skip one canonical declaration occurrence but keep a
						// self-reference later on the same line.
						if column <= strings.Index(line, name)+1 {
							continue
						}
					}
					role := referenceRole(line, column-1, name)
					if name != selected.target.Name && isImportReferenceLine(file.language, line) {
						role = "import"
					}
					occurrence := ReferenceOccurrence{
						Role: role, Confidence: confidence, Column: column,
						CodeLocation: lineLocation(file.path, file.language, file.lines, lineNo),
					}
					if confidence == "exact" {
						result.ExactReferenceCount++
					} else {
						result.PossibleReferenceCount++
						result.Complete = false
					}
					if fn := enclosingParsedFunction(file.functions, lineNo); fn != nil {
						key := fmt.Sprintf("%s:%d:%d", file.path, fn.start, fn.end)
						ctx := functionContexts[key]
						if ctx == nil {
							ctx = &ReferenceContext{Name: fn.name, CodeLocation: rangeLocation(file.path, file.language, file.lines, fn.start, fn.end), References: []ReferenceOccurrence{}}
							functionContexts[key] = ctx
						}
						ctx.References = appendUniqueOccurrence(ctx.References, occurrence)
					} else {
						key := fmt.Sprintf("%s:%d", file.path, lineNo)
						ctx := outsideContexts[key]
						if ctx == nil {
							ctx = &ReferenceContext{CodeLocation: occurrence.CodeLocation, References: []ReferenceOccurrence{}}
							outsideContexts[key] = ctx
						}
						ctx.References = appendUniqueOccurrence(ctx.References, occurrence)
					}
				}
			}
		}
	}
	for _, ctx := range functionContexts {
		result.Functions = append(result.Functions, *ctx)
	}
	for _, ctx := range outsideContexts {
		result.OutsideFunctions = append(result.OutsideFunctions, *ctx)
	}
}

func identifierColumns(line, name string) []int {
	var out []int
	for start := 0; start < len(line); {
		idx := strings.Index(line[start:], name)
		if idx < 0 {
			break
		}
		idx += start
		beforeOK := idx == 0 || !isIdentifierByte(line[idx-1])
		after := idx + len(name)
		afterOK := after == len(line) || !isIdentifierByte(line[after])
		if beforeOK && afterOK {
			out = append(out, idx+1)
		}
		start = idx + len(name)
	}
	return out
}

func isIdentifierByte(b byte) bool {
	return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func referenceRole(line string, zeroColumn int, name string) string {
	after := strings.TrimSpace(line[zeroColumn+len(name):])
	if strings.HasPrefix(after, "++") || strings.HasPrefix(after, "--") || compoundAssignmentPattern.MatchString(after) {
		return "read_write"
	}
	if strings.HasPrefix(after, "=") && !strings.HasPrefix(after, "==") && !strings.HasPrefix(after, "=>") {
		return "write"
	}
	return "read"
}

var compoundAssignmentPattern = regexp.MustCompile(`^(?:\+=|-=|\*=|/=|%=|&=|\|=|\^=|<<=|>>=)`)

func isImportReferenceLine(language, line string) bool {
	trimmed := strings.TrimSpace(line)
	switch language {
	case "python":
		return strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") && strings.Contains(trimmed, " import ")
	case "rust":
		return strings.HasPrefix(trimmed, "use ")
	case "nodejs":
		return strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "export ") && strings.Contains(trimmed, " from ")
	default:
		return false
	}
}

func enclosingParsedFunction(functions []parsedFunction, line int) *parsedFunction {
	var best *parsedFunction
	for i := range functions {
		fn := &functions[i]
		if line < fn.start || line > fn.end {
			continue
		}
		if best == nil || fn.end-fn.start < best.end-best.start {
			best = fn
		}
	}
	return best
}

func maskReferenceSource(lines []string, language string) []string {
	out := make([]string, len(lines))
	inBlockComment, inTriple := false, byte(0)
	for i, line := range lines {
		buf := []byte(line)
		masked := append([]byte(nil), buf...)
		quote := byte(0)
		escaped := false
		for j := 0; j < len(buf); j++ {
			if inTriple != 0 {
				if j+2 < len(buf) && buf[j] == inTriple && buf[j+1] == inTriple && buf[j+2] == inTriple {
					masked[j], masked[j+1], masked[j+2] = ' ', ' ', ' '
					j += 2
					inTriple = 0
				} else if buf[j] != '\t' {
					masked[j] = ' '
				}
				continue
			}
			if inBlockComment {
				if j+1 < len(buf) && buf[j] == '*' && buf[j+1] == '/' {
					masked[j], masked[j+1] = ' ', ' '
					j++
					inBlockComment = false
				} else if buf[j] != '\t' {
					masked[j] = ' '
				}
				continue
			}
			if quote != 0 {
				if buf[j] != '\t' {
					masked[j] = ' '
				}
				if escaped {
					escaped = false
				} else if buf[j] == '\\' {
					escaped = true
				} else if buf[j] == quote {
					quote = 0
				}
				continue
			}
			if language == "python" && j+2 < len(buf) && (buf[j] == '\'' || buf[j] == '"') && buf[j+1] == buf[j] && buf[j+2] == buf[j] {
				inTriple = buf[j]
				masked[j], masked[j+1], masked[j+2] = ' ', ' ', ' '
				j += 2
				continue
			}
			if language == "python" && buf[j] == '#' || language != "python" && j+1 < len(buf) && buf[j] == '/' && buf[j+1] == '/' {
				for k := j; k < len(masked); k++ {
					if masked[k] != '\t' {
						masked[k] = ' '
					}
				}
				break
			}
			if language != "python" && j+1 < len(buf) && buf[j] == '/' && buf[j+1] == '*' {
				masked[j], masked[j+1] = ' ', ' '
				j++
				inBlockComment = true
				continue
			}
			if buf[j] == '\'' && language == "rust" && rustCharLiteralEnd(buf, j) < 0 {
				continue
			}
			if buf[j] == '\'' || buf[j] == '"' || language == "nodejs" && buf[j] == '`' {
				quote = buf[j]
				masked[j] = ' '
			}
		}
		out[i] = string(masked)
	}
	return out
}

// rustCharLiteralEnd distinguishes character literals from lifetimes. A Rust
// lifetime starts with the same apostrophe but has no closing apostrophe after
// exactly one character or one escape sequence.
func rustCharLiteralEnd(line []byte, start int) int {
	i := start + 1
	if i >= len(line) {
		return -1
	}
	if line[i] == '\\' {
		i++
		if i >= len(line) {
			return -1
		}
		switch line[i] {
		case 'x':
			i += 3
		case 'u':
			i++
			if i >= len(line) || line[i] != '{' {
				return -1
			}
			for i < len(line) && line[i] != '}' {
				i++
			}
			i++
		default:
			i++
		}
	} else {
		_, size := utf8.DecodeRune(line[i:])
		if size == 0 {
			return -1
		}
		i += size
	}
	if i < len(line) && line[i] == '\'' {
		return i
	}
	return -1
}

func lineLocation(path, language string, lines []string, line int) CodeLocation {
	return rangeLocation(path, language, lines, line, line)
}

func rangeLocation(path, language string, lines []string, start, end int) CodeLocation {
	if start < 1 {
		start = 1
	}
	if start > len(lines) {
		start = len(lines)
	}
	if end > len(lines) {
		end = len(lines)
	}
	if end < start {
		end = start
	}
	content := ""
	if start <= end && start <= len(lines) {
		content = strings.Join(lines[start-1:end], "\n")
	}
	return CodeLocation{FilePath: path, LineRange: LineRange{Start: start, End: end, Count: max(0, end-start+1)}, Language: language, Content: content}
}

func appendUniqueOccurrence(in []ReferenceOccurrence, occurrence ReferenceOccurrence) []ReferenceOccurrence {
	for _, existing := range in {
		if existing.Role == occurrence.Role && existing.Confidence == occurrence.Confidence && existing.Column == occurrence.Column && existing.CodeLocation.FilePath == occurrence.CodeLocation.FilePath && existing.CodeLocation.LineRange.Start == occurrence.CodeLocation.LineRange.Start {
			return in
		}
	}
	return append(in, occurrence)
}

func sortReferenceResult(result *ReferenceResult) {
	sort.Slice(result.Functions, func(i, j int) bool {
		return locationLess(result.Functions[i].CodeLocation, result.Functions[j].CodeLocation)
	})
	sort.Slice(result.OutsideFunctions, func(i, j int) bool {
		return locationLess(result.OutsideFunctions[i].CodeLocation, result.OutsideFunctions[j].CodeLocation)
	})
	for i := range result.Functions {
		sortOccurrences(result.Functions[i].References)
	}
	for i := range result.OutsideFunctions {
		sortOccurrences(result.OutsideFunctions[i].References)
	}
}

func (r *ReferenceResult) Render() string {
	var b strings.Builder
	definition := r.Target.Definition
	fmt.Fprintf(&b, "%s %s definition (%s:%d-%d)\n", r.Target.Kind, r.Target.Name, definition.FilePath, definition.LineRange.Start, definition.LineRange.End)
	if definition.Content != "" {
		b.WriteString(definition.Content)
		b.WriteString("\n")
	}
	for _, context := range r.Functions {
		b.WriteString("\n")
		fmt.Fprintf(&b, "function %s (%s:%d-%d)%s\n", context.Name, context.CodeLocation.FilePath, context.CodeLocation.LineRange.Start, context.CodeLocation.LineRange.End, renderReferenceKinds(context.References))
		b.WriteString(context.CodeLocation.Content)
		b.WriteString("\n")
	}
	for _, context := range r.OutsideFunctions {
		b.WriteString("\n")
		fmt.Fprintf(&b, "outside function (%s:%d-%d)%s\n", context.CodeLocation.FilePath, context.CodeLocation.LineRange.Start, context.CodeLocation.LineRange.End, renderReferenceKinds(context.References))
		b.WriteString(context.CodeLocation.Content)
		b.WriteString("\n")
	}
	if !r.Complete {
		fmt.Fprintf(&b, "\nanalysis incomplete: %d possible reference(s)\n", r.PossibleReferenceCount)
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderReferenceKinds(references []ReferenceOccurrence) string {
	parts := make([]string, 0, len(references))
	for _, reference := range references {
		part := fmt.Sprintf("%s@%d", reference.Role, reference.CodeLocation.LineRange.Start)
		if reference.Confidence != "exact" {
			part += "?"
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

func sortOccurrences(in []ReferenceOccurrence) {
	sort.Slice(in, func(i, j int) bool { return locationLess(in[i].CodeLocation, in[j].CodeLocation) })
}

func locationLess(a, b CodeLocation) bool {
	if a.FilePath == b.FilePath {
		return a.LineRange.Start < b.LineRange.Start
	}
	return a.FilePath < b.FilePath
}

// Go implementation ---------------------------------------------------------

type goReferenceCandidate struct {
	key    string
	obj    types.Object
	pkg    *packages.Package
	ident  *ast.Ident
	path   string
	parent map[ast.Node]ast.Node
}

type goReferenceCacheEntry struct {
	mu          sync.Mutex
	fingerprint [sha256.Size]byte
	initialized bool
	pkgs        []*packages.Package
}

var goReferenceCache sync.Map // absolute repo root -> *goReferenceCacheEntry

func findGoReferences(ctx context.Context, repoRoot string, symbol SymbolRef, scope lookupScope) (*ReferenceResult, error) {
	pkgs, err := loadGoReferencePackages(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	var candidates []goReferenceCandidate
	seenCandidates := map[string]struct{}{}
	complete := true
	for _, pkg := range pkgs {
		if pkg == nil || pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		if len(pkg.Errors) > 0 {
			complete = false
		}
		parentCache := map[*ast.File]map[ast.Node]ast.Node{}
		for ident, obj := range pkg.TypesInfo.Defs {
			if ident == nil || obj == nil || ident.Name != symbol.Name {
				continue
			}
			file := goSyntaxFileAt(pkg, ident.Pos())
			if file == nil {
				continue
			}
			path := goPositionPath(repoRoot, pkg.Fset, ident.Pos())
			if path == "" || !pathInLookupScope(path, scope) {
				continue
			}
			key := goObjectKey(pkg.Fset, obj)
			if _, ok := seenCandidates[key]; ok {
				continue
			}
			seenCandidates[key] = struct{}{}
			parents := parentCache[file]
			if parents == nil {
				parents = goParentMap(file)
				parentCache[file] = parents
			}
			candidates = append(candidates, goReferenceCandidate{key: key, obj: obj, pkg: pkg, ident: ident, path: path, parent: parents})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].path == candidates[j].path {
			return candidates[i].pkg.Fset.Position(candidates[i].ident.Pos()).Line < candidates[j].pkg.Fset.Position(candidates[j].ident.Pos()).Line
		}
		return candidates[i].path < candidates[j].path
	})
	if len(candidates) == 0 {
		for _, pkg := range pkgs {
			if pkg != nil && len(pkg.Errors) > 0 {
				return nil, fmt.Errorf("symbol %q not found in %q: package analysis failed: %s", symbol.Name, symbol.Path, pkg.Errors[0].Msg)
			}
		}
		return nil, fmt.Errorf("symbol %q not found in %q", symbol.Name, symbol.Path)
	}
	if len(candidates) > 1 {
		targets := make([]ReferenceTarget, 0, len(candidates))
		for _, candidate := range candidates {
			targets = append(targets, goCandidateTarget(repoRoot, candidate))
		}
		return nil, &AmbiguousSymbolError{Name: symbol.Name, Candidates: targets}
	}
	selected := candidates[0]
	result := &ReferenceResult{Target: goCandidateTarget(repoRoot, selected), Functions: []ReferenceContext{}, OutsideFunctions: []ReferenceContext{}, Complete: complete}
	if !complete {
		result.Notes = []string{"Go package loading reported errors; returned references may be incomplete"}
	}
	functionContexts := map[string]*ReferenceContext{}
	outsideContexts := map[string]*ReferenceContext{}
	seenOccurrences := map[string]struct{}{}
	linesCache := map[string][]string{}
	for _, pkg := range pkgs {
		if pkg == nil || pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		parentCache := map[*ast.File]map[ast.Node]ast.Node{}
		for ident, obj := range pkg.TypesInfo.Uses {
			if ident == nil || obj == nil || !sameGoObject(pkg.Fset, obj, selected) {
				continue
			}
			file := goSyntaxFileAt(pkg, ident.Pos())
			if file == nil {
				continue
			}
			path := goPositionPath(repoRoot, pkg.Fset, ident.Pos())
			if path == "" {
				continue
			}
			pos := pkg.Fset.Position(ident.Pos())
			occKey := fmt.Sprintf("%s:%d:%d", path, pos.Line, pos.Column)
			if _, ok := seenOccurrences[occKey]; ok {
				continue
			}
			seenOccurrences[occKey] = struct{}{}
			lines := linesCache[path]
			if lines == nil {
				var readErr error
				lines, readErr = readReferenceLines(repoRoot, path)
				if readErr != nil {
					continue
				}
				linesCache[path] = lines
			}
			parents := parentCache[file]
			if parents == nil {
				parents = goParentMap(file)
				parentCache[file] = parents
			}
			occurrence := ReferenceOccurrence{Role: goReferenceRole(ident, parents), Confidence: "exact", Column: pos.Column, CodeLocation: lineLocation(path, "go", lines, pos.Line)}
			result.ExactReferenceCount++
			if fn := enclosingGoFunction(ident, parents); fn != nil {
				start, end := pkg.Fset.Position(fn.Pos()).Line, pkg.Fset.Position(fn.End()).Line
				key := fmt.Sprintf("%s:%d:%d", path, start, end)
				ctx := functionContexts[key]
				if ctx == nil {
					ctx = &ReferenceContext{Name: goFunctionName(fn), CodeLocation: rangeLocation(path, "go", lines, start, end), References: []ReferenceOccurrence{}}
					functionContexts[key] = ctx
				}
				ctx.References = appendUniqueOccurrence(ctx.References, occurrence)
			} else {
				node := enclosingGoTopLevel(ident, parents)
				start, end := pos.Line, pos.Line
				if node != nil {
					start, end = pkg.Fset.Position(node.Pos()).Line, pkg.Fset.Position(node.End()).Line
				}
				key := fmt.Sprintf("%s:%d:%d", path, start, end)
				ctx := outsideContexts[key]
				if ctx == nil {
					ctx = &ReferenceContext{CodeLocation: rangeLocation(path, "go", lines, start, end), References: []ReferenceOccurrence{}}
					outsideContexts[key] = ctx
				}
				ctx.References = appendUniqueOccurrence(ctx.References, occurrence)
			}
		}
	}
	for _, ctx := range functionContexts {
		result.Functions = append(result.Functions, *ctx)
	}
	for _, ctx := range outsideContexts {
		result.OutsideFunctions = append(result.OutsideFunctions, *ctx)
	}
	sortReferenceResult(result)
	return result, nil
}

func loadGoReferencePackages(ctx context.Context, repoRoot string) ([]*packages.Package, error) {
	key, err := filepath.Abs(repoRoot)
	if err != nil {
		key = repoRoot
	}
	actual, _ := goReferenceCache.LoadOrStore(key, &goReferenceCacheEntry{})
	entry := actual.(*goReferenceCacheEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fingerprint, err := goReferenceSourceFingerprint(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	if entry.initialized && entry.fingerprint == fingerprint {
		return entry.pkgs, nil
	}
	cfg := &packages.Config{Context: ctx, Dir: repoRoot, Tests: true, Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("finding Go references: %w", err)
	}
	entry.fingerprint = fingerprint
	entry.initialized = true
	entry.pkgs = pkgs
	return pkgs, nil
}

func goReferenceSourceFingerprint(ctx context.Context, repoRoot string) ([sha256.Size]byte, error) {
	files, err := collectFilesByExt(repoRoot, lookupScope{}, map[string]struct{}{".go": {}})
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	for _, name := range []string{"go.mod", "go.sum", "go.work", "go.work.sum"} {
		_, fullPath, resolveErr := repofs.ResolvePath(repoRoot, name)
		if resolveErr != nil {
			return [sha256.Size]byte{}, resolveErr
		}
		if _, statErr := os.Stat(fullPath); statErr == nil {
			files = append(files, fullPath)
		} else if !os.IsNotExist(statErr) {
			return [sha256.Size]byte{}, statErr
		}
	}
	sort.Strings(files)
	fingerprint, _, err := referenceSourceSnapshot(ctx, repoRoot, files)
	return fingerprint, err
}

func sameGoObject(fset *token.FileSet, obj types.Object, selected goReferenceCandidate) bool {
	return obj == selected.obj || goObjectKey(fset, obj) == selected.key
}

func goCandidateTarget(repoRoot string, candidate goReferenceCandidate) ReferenceTarget {
	node := goDefinitionNode(candidate.ident, candidate.parent)
	start, end := candidate.pkg.Fset.Position(candidate.ident.Pos()).Line, candidate.pkg.Fset.Position(candidate.ident.End()).Line
	if node != nil {
		start, end = candidate.pkg.Fset.Position(node.Pos()).Line, candidate.pkg.Fset.Position(node.End()).Line
	}
	lines, _ := readReferenceLines(repoRoot, candidate.path)
	return ReferenceTarget{Name: candidate.ident.Name, Kind: goObjectKind(candidate.obj, candidate.ident, candidate.parent), Definition: rangeLocation(candidate.path, "go", lines, start, end)}
}

func goObjectKind(obj types.Object, ident *ast.Ident, parents map[ast.Node]ast.Node) string {
	switch typed := obj.(type) {
	case *types.Const:
		return "constant"
	case *types.Func:
		return "function"
	case *types.TypeName:
		return "type"
	case *types.PkgName:
		return "import"
	case *types.Label:
		return "label"
	case *types.Var:
		if typed.IsField() {
			return "field"
		}
		for node := ast.Node(ident); node != nil; node = parents[node] {
			if field, ok := node.(*ast.Field); ok {
				if parent := parents[field]; parent != nil {
					if _, ok := parent.(*ast.FieldList); ok {
						return "parameter"
					}
				}
			}
			if _, ok := node.(*ast.FuncDecl); ok {
				break
			}
			if _, ok := node.(*ast.FuncLit); ok {
				break
			}
		}
		return "variable"
	default:
		return "symbol"
	}
}

func goObjectKey(fset *token.FileSet, obj types.Object) string {
	pos := fset.Position(obj.Pos())
	pkgPath := ""
	if obj.Pkg() != nil {
		pkgPath = obj.Pkg().Path()
	}
	return fmt.Sprintf("%s:%s:%d:%d:%T:%s", pkgPath, filepath.Clean(pos.Filename), pos.Line, pos.Column, obj, obj.Name())
}

func goPositionPath(repoRoot string, fset *token.FileSet, pos token.Pos) string {
	filename := fset.Position(pos).Filename
	if filename == "" {
		return ""
	}
	rel, err := repofs.RelPath(repoRoot, filename)
	if err != nil {
		return ""
	}
	return rel
}

func goSyntaxFileAt(pkg *packages.Package, pos token.Pos) *ast.File {
	for _, file := range pkg.Syntax {
		if file != nil && pos >= file.Pos() && pos <= file.End() {
			return file
		}
	}
	return nil
}

func goParentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func goDefinitionNode(ident *ast.Ident, parents map[ast.Node]ast.Node) ast.Node {
	for node := ast.Node(ident); node != nil; node = parents[node] {
		switch node.(type) {
		case *ast.ValueSpec, *ast.TypeSpec, *ast.FuncDecl, *ast.Field, *ast.AssignStmt, *ast.ImportSpec, *ast.LabeledStmt:
			return node
		case *ast.BlockStmt, *ast.File:
			return ident
		}
	}
	return ident
}

func enclosingGoFunction(node ast.Node, parents map[ast.Node]ast.Node) ast.Node {
	for current := node; current != nil; current = parents[current] {
		switch current.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return current
		}
	}
	return nil
}

func goFunctionName(node ast.Node) string {
	if fn, ok := node.(*ast.FuncDecl); ok {
		return fn.Name.Name
	}
	return "<anonymous>"
}

func enclosingGoTopLevel(node ast.Node, parents map[ast.Node]ast.Node) ast.Node {
	var candidate ast.Node
	for current := node; current != nil; current = parents[current] {
		if _, ok := parents[current].(*ast.File); ok {
			return current
		}
		candidate = current
	}
	return candidate
}

func goReferenceRole(ident *ast.Ident, parents map[ast.Node]ast.Node) string {
	for node := ast.Node(ident); node != nil; node = parents[node] {
		switch parent := parents[node].(type) {
		case *ast.AssignStmt:
			for _, lhs := range parent.Lhs {
				if astContains(lhs, ident) {
					if parent.Tok == token.ASSIGN || parent.Tok == token.DEFINE {
						return "write"
					}
					return "read_write"
				}
			}
			return "read"
		case *ast.IncDecStmt:
			if astContains(parent.X, ident) {
				return "read_write"
			}
		case *ast.RangeStmt:
			if astContains(parent.Key, ident) || astContains(parent.Value, ident) {
				return "write"
			}
		case *ast.FuncDecl, *ast.FuncLit, *ast.GenDecl:
			return "read"
		}
	}
	return "read"
}

func astContains(root ast.Node, target ast.Node) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if node == target {
			found = true
			return false
		}
		return !found
	})
	return found
}

func readReferenceLines(repoRoot, path string) ([]string, error) {
	_, fullPath, err := repofs.ResolvePath(repoRoot, path)
	if err != nil {
		return nil, err
	}
	data, err := repofs.ReadFile(repoRoot, fullPath)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), nil
}
