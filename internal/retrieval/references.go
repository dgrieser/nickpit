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
	"github.com/dgrieser/nickpit/internal/toollimits"
	"golang.org/x/tools/go/packages"
)

type AmbiguousSymbolError struct {
	Name string
	// Candidates carries at most toollimits.MaxAmbiguousReferenceTargets
	// declarations, because that is all the message prints. Total counts every
	// declaration found: building a target for each one meant reading and
	// joining the declaration source of thousands of them for a common
	// identifier, to name ten.
	Candidates []ReferenceTarget
	Total      int
}

func (e *AmbiguousSymbolError) Error() string {
	count := min(len(e.Candidates), toollimits.MaxAmbiguousReferenceTargets)
	total := max(e.Total, len(e.Candidates))
	locations := make([]string, 0, count+1)
	for _, candidate := range e.Candidates[:count] {
		loc := candidate.Definition
		locations = append(locations, fmt.Sprintf("%s at %s:%d", candidate.Kind, loc.FilePath, loc.LineRange.Start))
	}
	if omitted := total - count; omitted > 0 {
		locations = append(locations, fmt.Sprintf("and %d more", omitted))
	}
	// The candidates are printed as file:line because that pair is what
	// identifies one of them: a line alone cannot separate declarations that
	// share it across files.
	return fmt.Sprintf("symbol %q is ambiguous: %s; retry with the declaring file as path and its line to pick one", e.Name, strings.Join(locations, ", "))
}

// ambiguousSymbol reports total rival declarations while materializing targets
// only for the ones the message can name. target is called with the index of
// each candidate it needs.
func ambiguousSymbol(name string, total int, target func(int) ReferenceTarget) *AmbiguousSymbolError {
	count := min(total, toollimits.MaxAmbiguousReferenceTargets)
	candidates := make([]ReferenceTarget, 0, count)
	for i := range count {
		candidates = append(candidates, target(i))
	}
	return &AmbiguousSymbolError{Name: name, Candidates: candidates, Total: total}
}

// FindReferences resolves one declaration inside the optional lookup path and
// then searches the whole repository for references. Go uses go/types object
// identity. Other supported languages use their parser-derived function spans
// plus a conservative identifier index; uncertain matches are returned as
// possible instead of silently omitted.
func (e *LocalEngine) FindReferences(ctx context.Context, repoRoot string, symbol SymbolRef) (*ReferenceResult, error) {
	return findReferencesWithLoader(ctx, repoRoot, symbol, &referenceLoader{})
}

// FindReferencesBatch resolves all symbols against shared parsed and Go
// snapshots. Results preserve input order.
func (e *LocalEngine) FindReferencesBatch(ctx context.Context, repoRoot string, symbols []SymbolRef) []ReferenceBatchResult {
	loader := &referenceLoader{}
	results := make([]ReferenceBatchResult, len(symbols))
	for i, symbol := range symbols {
		results[i].Result, results[i].Err = findReferencesWithLoader(ctx, repoRoot, symbol, loader)
	}
	return results
}

// ResolveReferenceTargets resolves each symbol's declaration against the same
// shared snapshots, stopping before reference collection. Results preserve
// input order.
func (e *LocalEngine) ResolveReferenceTargets(ctx context.Context, repoRoot string, symbols []SymbolRef) []ReferenceTargetResult {
	loader := &referenceLoader{}
	results := make([]ReferenceTargetResult, len(symbols))
	for i, symbol := range symbols {
		resolved, err := resolveReferenceWithLoader(ctx, repoRoot, symbol, loader)
		if err != nil {
			results[i].Err = err
			continue
		}
		target := resolved.target
		results[i].Target = &target
	}
	return results
}

type referenceLoader struct {
	parsedLoaded bool
	parsed       []*parsedReferenceFile
	parsedErr    error
	goLoaded     bool
	goPackages   []*packages.Package
	goErr        error
	lineRoot     string
	lines        *referenceLineCache
}

// lineCache returns the display-source cache the whole batch shares, so a file
// that several lookups locate is read once rather than once per lookup.
func (l *referenceLoader) lineCache(repoRoot string) *referenceLineCache {
	if l.lines == nil || l.lineRoot != repoRoot {
		l.lines, l.lineRoot = newReferenceLineCache(repoRoot), repoRoot
	}
	return l.lines
}

func (l *referenceLoader) loadParsed(ctx context.Context, repoRoot string) ([]*parsedReferenceFile, error) {
	if !l.parsedLoaded {
		l.parsed, l.parsedErr = loadParsedReferenceFiles(ctx, repoRoot)
		l.parsedLoaded = true
	}
	return l.parsed, l.parsedErr
}

func (l *referenceLoader) loadGo(ctx context.Context, repoRoot string) ([]*packages.Package, error) {
	if !l.goLoaded {
		l.goPackages, l.goErr = loadGoReferencePackages(ctx, repoRoot)
		l.goLoaded = true
	}
	return l.goPackages, l.goErr
}

// resolvedReference is one declaration plus everything needed to go on and
// collect its references, so resolution and collection share one routing path.
type resolvedReference struct {
	target ReferenceTarget
	// Exactly one of the two backends owns the declaration.
	goPackages []*packages.Package
	goSelected goReferenceCandidate
	goComplete bool
	goLines    *referenceLineCache
	parsedAll  []*parsedReferenceFile
	parsed     definitionCandidate
}

func (r *resolvedReference) isGo() bool { return r.goPackages != nil }

func findReferencesWithLoader(ctx context.Context, repoRoot string, symbol SymbolRef, loader *referenceLoader) (*ReferenceResult, error) {
	resolved, err := resolveReferenceWithLoader(ctx, repoRoot, symbol, loader)
	if err != nil {
		return nil, err
	}
	if resolved.isGo() {
		return collectGoReferences(repoRoot, resolved.goPackages, resolved.goSelected, resolved.goComplete, resolved.goLines), nil
	}
	return buildParsedReferenceResult(resolved.parsedAll, resolved.parsed, loader.lineCache(repoRoot)), nil
}

// resolveReferenceWithLoader picks the declaration symbol names and reports
// which backend owns it. Callers that only need the declaration stop here.
func resolveReferenceWithLoader(ctx context.Context, repoRoot string, symbol SymbolRef, loader *referenceLoader) (*resolvedReference, error) {
	symbol.Name = strings.TrimSpace(symbol.Name)
	if symbol.Name == "" {
		return nil, fmt.Errorf("finding references: symbol name is empty")
	}
	scope, err := resolveLookupScope(repoRoot, symbol.Path)
	if err != nil {
		return nil, fmt.Errorf("finding references for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	resolveGo := func(symbol SymbolRef, scope lookupScope) (*resolvedReference, error) {
		pkgs, loadErr := loader.loadGo(ctx, repoRoot)
		if loadErr != nil {
			return nil, loadErr
		}
		lines := loader.lineCache(repoRoot)
		selected, complete, resolveErr := resolveGoDeclaration(repoRoot, symbol, scope, pkgs, lines)
		if resolveErr != nil {
			return nil, resolveErr
		}
		// Building the target here also materializes the winner's parent map,
		// which the stored candidate carries on to reference collection.
		target := goCandidateTarget(&selected, lines)
		return &resolvedReference{
			target:     target,
			goPackages: pkgs, goSelected: selected, goComplete: complete, goLines: lines,
		}, nil
	}
	if scope.IsFile {
		ext := strings.ToLower(filepath.Ext(scope.Path))
		if ext == ".go" {
			return resolveGo(symbol, scope)
		}
		if _, ok := referenceExtensions[ext]; !ok {
			return nil, &UnsupportedLanguageError{Path: scope.Path}
		}
		parsed, loadErr := loader.loadParsed(ctx, repoRoot)
		if loadErr != nil {
			return nil, fmt.Errorf("finding references for %q: %w", symbol.Name, loadErr)
		}
		selected, resolveErr := resolveParsedDefinition(symbol, scope, parsed, loader.lineCache(repoRoot))
		if resolveErr != nil {
			return nil, resolveErr
		}
		return &resolvedReference{target: selected.target, parsedAll: parsed, parsed: selected}, nil
	}

	// A directory/repo lookup can span parser families. Resolve declarations
	// conservatively first; a selected Go declaration is then re-run through
	// go/types for exact references.
	parsed, err := loader.loadParsed(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("finding references for %q: %w", symbol.Name, err)
	}
	selected, err := resolveParsedDefinition(symbol, scope, parsed, loader.lineCache(repoRoot))
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(selected.file.path), ".go") {
		return resolveGo(
			SymbolRef{Name: symbol.Name, Path: selected.file.path, Line: symbol.Line},
			lookupScope{Path: selected.file.path, IsFile: true},
		)
	}
	return &resolvedReference{target: selected.target, parsedAll: parsed, parsed: selected}, nil
}

var referenceExtensions = map[string]struct{}{
	".go": {}, ".py": {}, ".js": {}, ".mjs": {}, ".cjs": {}, ".jsx": {},
	".ts": {}, ".mts": {}, ".cts": {}, ".tsx": {}, ".rs": {},
}

type parsedReferenceFile struct {
	path     string
	language string
	// masked is the source with strings and comments blanked out, one entry per
	// original line. The original text is deliberately not retained: the parsed
	// snapshot is cached for the life of the process, and keeping both forms
	// doubled the resident source bytes of every repository the daemon has
	// reviewed. Location content is read back per lookup for the handful of
	// files that actually produce one.
	masked    []string
	functions []parsedFunction
	imports   []tsparser.Import
	exports   []tsparser.Export
	// resolvedImports maps a symbol import's module spec to its repo-relative
	// path, empty when the spec does not resolve inside the repository.
	// Resolution stats up to a dozen candidate filenames per spec, so every
	// spec — including the ones that resolve to nothing — is recorded once per
	// parsed snapshot rather than re-stat'd per binding.
	resolvedImports map[string]string
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

var parsedReferenceCache referenceCacheStore[parsedReferenceCacheEntry] // absolute repo root -> entry

// referenceCacheStore memoizes one expensive per-repository artifact — parsed
// sources or type-checked Go packages — keyed by absolute repository root. Both
// artifacts retain the whole repository, so the daemon, which reviews a new
// worktree per merge request, would otherwise grow without bound for the life of
// the process. Least-recently-used roots are dropped once the cap is exceeded;
// an evicted entry stays alive for whoever still holds it and is simply rebuilt
// on the next lookup, so eviction can never produce a wrong answer.
type referenceCacheStore[T any] struct {
	mu      sync.Mutex
	entries map[string]*referenceCacheSlot[T]
	clock   uint64
}

type referenceCacheSlot[T any] struct {
	value    *T
	lastUsed uint64
}

// entry returns the cached entry for key, creating it when absent. Callers
// serialize their own use of the returned entry.
func (c *referenceCacheStore[T]) entry(key string) *T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]*referenceCacheSlot[T]{}
	}
	c.clock++
	if slot, ok := c.entries[key]; ok {
		slot.lastUsed = c.clock
		return slot.value
	}
	value := new(T)
	c.entries[key] = &referenceCacheSlot[T]{value: value, lastUsed: c.clock}
	c.evictLocked()
	return value
}

// evictLocked drops least-recently-used roots until the cache fits its cap.
// The cap counts roots per cache, so the parsed and Go snapshots each keep up
// to that many. NICKPIT_REFERENCE_CACHE_MAX_ENTRIES tunes it; a value <= 0
// disables eviction.
func (c *referenceCacheStore[T]) evictLocked() {
	limit := cacheCapFromEnv("NICKPIT_REFERENCE_CACHE_MAX_ENTRIES", toollimits.DefaultReferenceCacheEntries)
	if limit <= 0 {
		return
	}
	for len(c.entries) > limit {
		oldestKey, oldest := "", uint64(0)
		for key, slot := range c.entries {
			if oldestKey == "" || slot.lastUsed < oldest {
				oldestKey, oldest = key, slot.lastUsed
			}
		}
		delete(c.entries, oldestKey)
	}
}

// resolveParsedDefinition picks the single declaration of symbol inside scope.
// It is separate from occurrence collection so a repo-wide lookup that lands on
// a Go declaration can hand off to go/types without paying for a parsed-language
// reference analysis it would discard.
func resolveParsedDefinition(symbol SymbolRef, scope lookupScope, parsed []*parsedReferenceFile, source *referenceLineCache) (definitionCandidate, error) {
	var candidates []definitionCandidate
	analyzed := 0
	// The declaration patterns depend only on the language and the symbol, so
	// they are compiled once per language rather than once per file.
	patterns := map[string]declarationPatterns{}
	for _, file := range parsed {
		if !pathInLookupScope(file.path, scope) {
			continue
		}
		analyzed++
		compiled, ok := patterns[file.language]
		if !ok {
			compiled = compileDeclarationPatterns(file.language, symbol.Name)
			patterns[file.language] = compiled
		}
		candidates = append(candidates, findDefinitionCandidates(file, symbol.Name, compiled, source)...)
	}
	if len(candidates) == 0 {
		if analyzed == 0 && !scope.IsFile {
			// Nothing in this directory/repository scope has a structural
			// backend, so its absence says nothing about the symbol.
			return definitionCandidate{}, &UnsupportedLanguageError{Path: scope.Path}
		}
		return definitionCandidate{}, &SymbolNotFoundError{Name: symbol.Name, Path: scope.Path}
	}
	candidates = dedupeDefinitionCandidates(candidates)
	// An import binds a name declared somewhere else, so it is a usage kind,
	// not a rival declaration. Keeping it as one made every symbol imported
	// anywhere in the repository resolve as ambiguous with itself. It still
	// answers a lookup scoped to a file that only imports the symbol, where
	// there is nothing better to point at.
	if declarations := candidatesOtherThanImports(candidates); len(declarations) > 0 {
		candidates = declarations
	}
	if pinned, requested := pinCandidatesToLine(candidates, symbol.Line, func(i int) LineRange {
		return candidates[i].target.Definition.LineRange
	}); requested {
		if len(pinned) == 0 {
			return definitionCandidate{}, &SymbolNotFoundError{
				Name: symbol.Name, Path: scope.Path,
				Reason: fmt.Sprintf("no declaration at line %d", symbol.Line),
			}
		}
		candidates = pinned
	}
	if len(candidates) > 1 {
		return definitionCandidate{}, ambiguousSymbol(symbol.Name, len(candidates), func(i int) ReferenceTarget {
			return candidates[i].target
		})
	}
	return candidates[0], nil
}

func candidatesOtherThanImports(candidates []definitionCandidate) []definitionCandidate {
	out := make([]definitionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.target.Kind != "import" {
			out = append(out, candidate)
		}
	}
	return out
}

// pinCandidatesToLine narrows same-named declarations to the one the caller
// pinned, which is the only way to disambiguate two declarations in one file.
// A declaration line wins over a span that merely contains it, so a method
// pinned inside an enclosing declaration still resolves. The second result
// reports whether a pin was requested at all; when it was and the first result
// is empty, the line matched no declaration and the caller must say so rather
// than quietly resolving to some other one.
//
// rangeOf takes the candidate's index rather than its value so a backend that
// derives the range expensively can memoize the work into its own slice.
func pinCandidatesToLine[T any](candidates []T, line int, rangeOf func(int) LineRange) ([]T, bool) {
	if line <= 0 {
		return nil, false
	}
	var exact, spanning []T
	for i, candidate := range candidates {
		lines := rangeOf(i)
		switch {
		case lines.Start == line:
			exact = append(exact, candidate)
		case line > lines.Start && line <= lines.End:
			spanning = append(spanning, candidate)
		}
	}
	if len(exact) > 0 {
		return exact, true
	}
	return spanning, true
}

// buildParsedReferenceResult collects occurrences for a declaration the parser
// backends resolved. Go never reaches here — both callers route a Go
// declaration to go/types first — so the result is always the conservative,
// incomplete kind.
func buildParsedReferenceResult(parsed []*parsedReferenceFile, selected definitionCandidate, source *referenceLineCache) *ReferenceResult {
	aliases := referenceAliases(parsed, selected)
	result := &ReferenceResult{
		Target:           selected.target,
		Functions:        []ReferenceContext{},
		OutsideFunctions: []ReferenceContext{},
		Notes:            []string{"dynamic-language references include conservative same-name candidates when binding identity cannot be proven"},
	}
	collectParsedOccurrences(result, parsed, selected, aliases, source)
	sortReferenceResult(result)
	return result
}

func loadParsedReferenceFiles(ctx context.Context, repoRoot string) ([]*parsedReferenceFile, error) {
	key, err := filepath.Abs(repoRoot)
	if err != nil {
		key = repoRoot
	}
	entry := parsedReferenceCache.entry(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	files, err := collectFilesByExt(repoRoot, lookupScope{}, referenceExtensions)
	if err != nil {
		return nil, err
	}
	// Fingerprint first: a hit costs one stat per file and never touches the
	// contents, and a miss parses each file as it is read instead of buffering
	// every source at once.
	fingerprint, err := referenceSourceFingerprint(ctx, repoRoot, files)
	if err != nil {
		return nil, err
	}
	if entry.initialized && entry.fingerprint == fingerprint {
		return entry.files, nil
	}
	parsed := make([]*parsedReferenceFile, 0, len(files))
	if err := forEachReferenceSource(ctx, repoRoot, files, func(source referenceSource) {
		parsed = append(parsed, parseReferenceFile(repoRoot, source.path, string(source.data)))
	}); err != nil {
		return nil, err
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

// forEachReferenceSource reads the files in order and hands each to visit,
// which must not retain the bytes: only one file's contents is live at a time.
// Reads honor the same per-file byte cap as the rest of retrieval, so one
// generated or vendored blob cannot blow up the parsed snapshot; a clipped file
// simply yields no declarations past the cap.
//
// A file that cannot be read is skipped, matching walkRepoTextFiles. Enumeration
// and reading are separate steps, and the daemon reviews worktrees that a
// checkout or build can still be touching, so failing the whole lookup would
// turn that race into a hard tool failure for the entire review.
func forEachReferenceSource(ctx context.Context, repoRoot string, files []string, visit func(referenceSource)) error {
	for _, fullPath := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := repofs.RelPath(repoRoot, fullPath)
		if err != nil {
			continue
		}
		data, _, err := readFileCapped(repoRoot, fullPath, toollimits.MaxRetrievedFileBytes)
		if err != nil {
			continue
		}
		visit(referenceSource{path: rel, data: data})
	}
	return nil
}

// referenceSourceFingerprint identifies a file set by each file's path, size
// and modification time. Hashing the contents would mean reading the entire
// repository on every lookup purely to discover the cache is still valid — and
// twice over for a Go lookup, which fingerprints its own file set as well.
// Checkouts, edits and branch switches all move mtime, so stat metadata
// notices the changes that matter; a rewrite that preserves both size and
// mtime is the one case it cannot see.
func referenceSourceFingerprint(ctx context.Context, repoRoot string, files []string) ([sha256.Size]byte, error) {
	hash := sha256.New()
	for _, fullPath := range files {
		if err := ctx.Err(); err != nil {
			return [sha256.Size]byte{}, err
		}
		rel, err := repofs.RelPath(repoRoot, fullPath)
		if err != nil {
			continue
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			// Skipped here and in forEachReferenceSource alike, so the
			// fingerprint keeps describing exactly the set that gets parsed.
			continue
		}
		_, _ = fmt.Fprintf(hash, "%d:%s:%d:%d;", len(rel), rel, info.Size(), info.ModTime().UnixNano())
	}
	return [sha256.Size]byte(hash.Sum(nil)), nil
}

func parseReferenceFile(repoRoot, path, source string) *parsedReferenceFile {
	language := detectLanguage(path)
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	file := &parsedReferenceFile{
		path: path, language: language,
		masked: maskReferenceSource(lines, language),
	}
	if ir, err := tsparser.ParseFile(path, []byte(source)); err == nil {
		file.imports = append(file.imports, ir.Imports...)
		file.exports = append(file.exports, ir.Exports...)
		for _, symbol := range ir.Symbols {
			file.functions = append(file.functions, parsedFunction{name: symbol.Name, start: symbol.StartLine, end: symbol.EndLine})
		}
	}
	for _, binding := range file.imports {
		if binding.Kind != "symbol" {
			continue
		}
		if _, done := file.resolvedImports[binding.ModuleSpec]; done {
			continue
		}
		resolved, _ := resolveReferenceImport(repoRoot, file, binding.ModuleSpec)
		if file.resolvedImports == nil {
			file.resolvedImports = map[string]string{}
		}
		file.resolvedImports[binding.ModuleSpec] = resolved
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

// declarationPatterns holds the compiled matchers for one (language, symbol)
// pair, which every file of that language reuses.
type declarationPatterns struct {
	byKind map[string]*regexp.Regexp
	// goGrouped matches a name inside a Go `const (`/`var (`/`type (` block.
	goGrouped *regexp.Regexp
}

func compileDeclarationPatterns(language, name string) declarationPatterns {
	quoted := regexp.QuoteMeta(name)
	return declarationPatterns{
		byKind:    definitionPatterns(language, quoted),
		goGrouped: regexp.MustCompile(`^\s*` + quoted + `\b`),
	}
}

func findDefinitionCandidates(file *parsedReferenceFile, name string, compiled declarationPatterns, source *referenceLineCache) []definitionCandidate {
	patterns, goGroupedDefinition := compiled.byKind, compiled.goGrouped
	lines := newLazySourceLines(source, file.path)
	seenScope := map[string]struct{}{}
	var out []definitionCandidate
	// Parser symbols are the authoritative function declarations: they cover
	// forms the regex fallback misses, such as JavaScript class methods, and
	// they span decorators and attributes the declaration line excludes.
	var declared []parsedFunction
	for _, symbol := range file.functions {
		if symbol.name != name {
			continue
		}
		declared = append(declared, symbol)
		out = append(out, definitionCandidate{
			target: ReferenceTarget{Name: name, Kind: "function", Definition: rangeLocation(file.path, file.language, lines.get(), symbol.start, symbol.end)},
			file:   file,
		})
	}
	goGroupKind, goGroupDepth, goBraceDepth := "", 0, 0
	for i, line := range file.masked {
		lineNo := i + 1
		if file.language == "go" {
			trimmed := strings.TrimSpace(line)
			switch {
			case goGroupKind == "" && trimmed == "const (":
				goGroupKind, goGroupDepth, goBraceDepth = "constant", 1, 0
			case goGroupKind == "" && trimmed == "var (":
				goGroupKind, goGroupDepth, goBraceDepth = "variable", 1, 0
			case goGroupKind == "" && trimmed == "type (":
				goGroupKind, goGroupDepth, goBraceDepth = "type", 1, 0
			case goGroupKind != "":
				// Only the paren balancing the opening one ends the group: an
				// entry whose value spans lines opens and closes parens of its
				// own, and treating those as the end hides every later entry.
				// A declaration only ever starts at the group's own depth, and
				// outside any brace body: a struct or interface written inside
				// the group keeps the paren depth of its entry, so its fields
				// would otherwise each be reported as a declaration of the
				// group's kind.
				kind := goGroupKind
				declares := goGroupDepth == 1 && goBraceDepth == 0 && goGroupedDefinition.MatchString(line)
				goGroupDepth += strings.Count(line, "(") - strings.Count(line, ")")
				goBraceDepth = max(0, goBraceDepth+strings.Count(line, "{")-strings.Count(line, "}"))
				if goGroupDepth <= 0 {
					goGroupKind, goGroupDepth, goBraceDepth = "", 0, 0
				}
				if declares {
					loc := lineLocation(file.path, file.language, lines.get(), lineNo)
					out = append(out, definitionCandidate{target: ReferenceTarget{Name: name, Kind: kind, Definition: loc}, file: file})
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
			// The parser already reported this declaration; its span is the
			// wider one, so do not add the declaration line as a rival.
			if kind == "function" && lineWithinParsedFunctions(declared, lineNo) {
				break
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
			loc := lineLocation(file.path, file.language, lines.get(), lineNo)
			for _, symbol := range file.functions {
				if symbol.name == name && symbol.start == lineNo {
					loc = rangeLocation(file.path, file.language, lines.get(), symbol.start, symbol.end)
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
			"function": compile(`\bfunction\s+` + word + `|\b(?:const|let|var)\s+` + word + `\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>`),
			"type":     compile(`\b(?:class|interface|type|enum|namespace)\s+` + word),
			"import":   compile(`^\s*import\b.*` + word + `|\brequire\s*\([^)]*\).*` + word),
			// Every alternative anchors on something only a parameter list has —
			// the `function` keyword, a trailing `=>`, or a type annotation on a
			// method signature. An optional prefix would match `show(total)` and
			// report every call that passes the symbol as its declaration.
			"parameter": compile(`\bfunction\b[^()]*\([^)]*` + word + `(?:\s*[:=,)]|$)` +
				`|\([^()]*` + word + `[^()]*\)\s*(?::[^=]*)?=>` +
				`|^\s*(?:(?:public|private|protected|readonly|static|async|get|set)\s+)*[A-Za-z_$][\w$]*\s*\([^(){}]*` + word + `\s*\??\s*:`),
			"variable": compile(`\b(?:const|let|var)\s+` + word),
			"field":    compile(`^\s*(?:public\s+|private\s+|protected\s+|static\s+|readonly\s+)*` + word + `\s*(?:[?:=!]|$)`),
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
			"function": compile(`\bfunc\s+(?:\([^)]*\)\s*)?` + word + `\s*\(`),
			"type":     compile(`\btype\s+` + word),
			"constant": compile(`\bconst\s+` + word),
			"variable": compile(`\bvar\s+` + word + `|\b` + word + `\s*:=`),
			// A pointer type binds to its name (`n *T`), while a multiplication is
			// spaced on both sides (`n * m`); requiring the pointee keeps ordinary
			// arithmetic from being reported as a declaration.
			"parameter": compile(`\b` + word + `\s+(?:\*[\w\[(]|\[|map\[|chan\s+|func\b|interface\b|[A-Za-z_])`),
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
		if li.FilePath != lj.FilePath {
			return li.FilePath < lj.FilePath
		}
		if li.LineRange.Start != lj.LineRange.Start {
			return li.LineRange.Start < lj.LineRange.Start
		}
		// The dedup key already made (file, line, kind) unique, so the kind
		// completes the order and keeps ambiguity listings stable.
		return out[i].target.Kind < out[j].target.Kind
	})
	return out
}

// referenceAliases follows direct repository-local symbol imports, using the
// module paths each file resolved when it was parsed.
func referenceAliases(files []*parsedReferenceFile, selected definitionCandidate) map[string]map[string]string {
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
	// Only a file of the declaration's own language can bind it — no import
	// crosses parser families — and a file picks the binding up solely through
	// a symbol import, so the fixpoint walks that subset instead of every file
	// in the repository on every pass. The declaring file joins it for its own
	// re-export propagation.
	candidates := make([]*parsedReferenceFile, 0, len(files))
	for _, file := range files {
		if file.language != selected.file.language {
			continue
		}
		if file.path == selected.file.path || fileImportsSymbols(file) {
			candidates = append(candidates, file)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, file := range candidates {
			for _, binding := range file.imports {
				if binding.Kind != "symbol" {
					continue
				}
				resolved := file.resolvedImports[binding.ModuleSpec]
				if resolved == "" || exported[resolved][binding.SymbolName] == "" {
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
	// The declaring file's own lexical scope is the strongest binding evidence
	// there is, so its uses are marked at least as confidently as the ones
	// reached by following an import into another file. Dropping the name here
	// inverted that: same-file uses came out possible, cross-file ones exact.
	//
	// Rust module resolution is intentionally conservative in the existing
	// backend. Preserve renamed `use` bindings as possible references instead
	// of dropping them when a module cannot be proven. Only a Rust declaration
	// can be bound this way, so a lookup in another language never pays for the
	// scan.
	if selected.file.language == "rust" {
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
	}
	return locals
}

func fileImportsSymbols(file *parsedReferenceFile) bool {
	for _, binding := range file.imports {
		if binding.Kind == "symbol" {
			return true
		}
	}
	return false
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

func collectParsedOccurrences(result *ReferenceResult, files []*parsedReferenceFile, selected definitionCandidate, aliases map[string]map[string]string, source *referenceLineCache) {
	functionContexts := map[string]*ReferenceContext{}
	outsideContexts := map[string]*ReferenceContext{}
	declarationLine := declarationNameLine(selected)
	declarationSkipped := false
	// Exact confidence rests on a binding proof: the declaring file's own
	// lexical scope, or an import chain followed into another file. Neither
	// survives a function that declares the same name itself, so uses inside
	// such a scope fall back to possible rather than claiming certainty about a
	// symbol they cannot refer to. The spans are found per file and name only
	// when an exact binding is in play.
	shadowed := map[string][]parsedFunction{}
	shadowingSpans := func(file *parsedReferenceFile, name string) []parsedFunction {
		key := file.path + "\x00" + name
		spans, ok := shadowed[key]
		if !ok {
			spans = shadowingFunctionSpans(file, name, selected)
			shadowed[key] = spans
		}
		return spans
	}
	for _, file := range files {
		// A same-named identifier in another language is a different symbol —
		// no import can bind a Python name to a Rust one — so scanning across
		// parser families only manufactures false positives.
		if file.language != selected.file.language {
			continue
		}
		// Only go/types can prove binding identity, and it owns every Go
		// lookup, so a declaration resolved here is never better than possible.
		names := map[string]string{selected.target.Name: "possible"}
		maps.Copy(names, aliases[file.path])
		lines := newLazySourceLines(source, file.path)
		for lineIndex, line := range file.masked {
			lineNo := lineIndex + 1
			for name, bound := range names {
				columns := identifierColumns(line, name)
				if len(columns) == 0 {
					continue
				}
				confidence := bound
				if confidence == "exact" && lineWithinParsedFunctions(shadowingSpans(file, name), lineNo) {
					confidence = "possible"
				}
				for _, column := range columns {
					// The declaration is the target, not a reference to it. Only
					// the line that spells the declaration is skipped, and only
					// its first occurrence: the rest of the span belongs to
					// decorators and attributes that name other symbols, and a
					// rebinding such as `count = count + 1` reads the name it
					// declares.
					if !declarationSkipped && file.path == selected.file.path && name == selected.target.Name && lineNo == declarationLine {
						declarationSkipped = true
						continue
					}
					role := referenceRole(file.language, line, column-1, name)
					// An import binds the symbol whether or not it renames it;
					// restricting the role to renamed aliases left the common
					// `from mod import NAME` reported as an ordinary read.
					if isImportReferenceLine(file.language, line) {
						role = "import"
					}
					occurrence := ReferenceOccurrence{
						Role: role, Confidence: confidence, Column: column,
						CodeLocation: lineLocation(file.path, file.language, lines.get(), lineNo),
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
							ctx = &ReferenceContext{Name: fn.name, CodeLocation: rangeLocation(file.path, file.language, lines.get(), fn.start, fn.end), References: []ReferenceOccurrence{}}
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

// sourceLines returns the file's current text for building locations, or nil
// when it cannot be read: a location then carries its true line numbers without
// content, which beats dropping the reference or inventing a snippet.
func sourceLines(cache *referenceLineCache, path string) []string {
	if cache == nil {
		return nil
	}
	lines, _ := cache.get(path)
	return lines
}

// lazySourceLines defers reading a file's display text until a location
// actually needs it. Both parsed-language passes visit every file in scope but
// build a location only for the few that produce a candidate or an occurrence,
// so reading up front put a second full copy of the repository's source next to
// the masked snapshot — the doubling parsedReferenceFile.masked exists to avoid.
type lazySourceLines struct {
	cache  *referenceLineCache
	path   string
	lines  []string
	loaded bool
}

func newLazySourceLines(cache *referenceLineCache, path string) *lazySourceLines {
	return &lazySourceLines{cache: cache, path: path}
}

func (l *lazySourceLines) get() []string {
	if !l.loaded {
		l.lines, l.loaded = sourceLines(l.cache, l.path), true
	}
	return l.lines
}

// declarationNameLine returns the line inside the declaration's span that
// actually spells the declaration: the first one that names the symbol and is
// not a decorator or attribute. A parser span starts at the first annotation
// above the declaration, and an annotation can name the symbol itself —
// `@value.setter` refers to the property the setter extends — so taking the
// span's first occurrence as the declaration both dropped a real reference and
// left the declaring line reported as a use of itself.
func declarationNameLine(selected definitionCandidate) int {
	span := selected.target.Definition.LineRange
	if span.End <= span.Start {
		return span.Start
	}
	file, name := selected.file, selected.target.Name
	for line := span.Start; line <= span.End && line <= len(file.masked); line++ {
		masked := file.masked[line-1]
		if len(identifierColumns(masked, name)) == 0 || isAnnotationLine(masked) {
			continue
		}
		return line
	}
	return span.Start
}

// isAnnotationLine reports whether a line decorates the declaration below it
// rather than being part of it: Python and TypeScript decorators, Rust
// attributes.
func isAnnotationLine(masked string) bool {
	trimmed := strings.TrimSpace(masked)
	return strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "#[") || strings.HasPrefix(trimmed, "#![")
}

// shadowingFunctionSpans returns the function spans in file that declare name
// themselves. Inside one of them the name binds to that declaration, so a use
// there is not a proven reference to the symbol being looked up. The span that
// holds the looked-up declaration is excluded — that one is the binding.
func shadowingFunctionSpans(file *parsedReferenceFile, name string, selected definitionCandidate) []parsedFunction {
	definition := selected.target.Definition.LineRange
	declaring := file.path == selected.file.path
	seen := map[string]struct{}{}
	var spans []parsedFunction
	// Locations are not needed here, only declaration lines, so the candidates
	// are built without reading the file back.
	for _, candidate := range findDefinitionCandidates(file, name, compileDeclarationPatterns(file.language, name), nil) {
		line := candidate.target.Definition.LineRange.Start
		if declaring && line >= definition.Start && line <= definition.End {
			continue
		}
		fn := enclosingParsedFunction(file.functions, line)
		if fn == nil {
			// A module-level rebinding is the same lexical scope as a
			// module-level declaration, and an import binds the symbol itself.
			continue
		}
		if declaring && fn.start <= definition.Start && definition.End <= fn.end {
			continue
		}
		key := fmt.Sprintf("%d:%d", fn.start, fn.end)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		spans = append(spans, *fn)
	}
	return spans
}

func identifierColumns(line, name string) []int {
	if name == "" {
		// strings.Index would match at every position without advancing.
		return nil
	}
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

func referenceRole(language, line string, zeroColumn int, name string) string {
	after := strings.TrimSpace(line[zeroColumn+len(name):])
	if strings.HasPrefix(after, "++") || strings.HasPrefix(after, "--") || compoundAssignmentPattern.MatchString(after) {
		return "read_write"
	}
	if strings.HasPrefix(after, "=") && !strings.HasPrefix(after, "==") && !strings.HasPrefix(after, "=>") {
		// Python spells keyword arguments and parameter defaults with the same
		// `=`: `process(count=1)` and `def f(count=10)` name a parameter rather
		// than assigning to the symbol under lookup, and reporting them as
		// writes told a reviewer the symbol is mutated at call sites that only
		// pass it by name. Only an unclosed bracket on the same line proves the
		// position, so an argument list wrapped over several lines still reads
		// as an assignment. Every other language here assigns with a bare `=`
		// inside parentheses too, so the exception is Python's alone.
		if language == "python" && withinOpenBracket(line[:zeroColumn]) {
			return "read"
		}
		return "write"
	}
	return "read"
}

// withinOpenBracket reports whether prefix leaves a bracket open, meaning the
// position that follows it sits inside an argument or parameter list, a
// subscript, or a literal. The line is masked, so brackets in strings and
// comments are already gone.
func withinOpenBracket(prefix string) bool {
	depth := 0
	for i := range len(prefix) {
		switch prefix[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		}
	}
	return depth > 0
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

func lineWithinParsedFunctions(functions []parsedFunction, line int) bool {
	for _, fn := range functions {
		if line >= fn.start && line <= fn.end {
			return true
		}
	}
	return false
}

func maskReferenceSource(lines []string, language string) []string {
	out := make([]string, len(lines))
	// Backtick literals — Go raw strings and JavaScript template literals — span
	// lines, so their state has to survive the per-line reset that single- and
	// double-quoted strings rely on.
	inBlockComment, inBacktick, backtickEscaped, inTriple := false, false, false, byte(0)
	for i, line := range lines {
		buf := []byte(line)
		masked := append([]byte(nil), buf...)
		quote := byte(0)
		escaped := false
		for j := 0; j < len(buf); j++ {
			if inBacktick {
				if buf[j] != '\t' {
					masked[j] = ' '
				}
				switch {
				case backtickEscaped:
					backtickEscaped = false
				case language == "nodejs" && buf[j] == '\\':
					backtickEscaped = true
				case buf[j] == '`':
					inBacktick = false
				}
				continue
			}
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
			if buf[j] == '`' && (language == "go" || language == "nodejs") {
				inBacktick = true
				masked[j] = ' '
				continue
			}
			if buf[j] == '\'' || buf[j] == '"' {
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

// rangeLocation never indexes past the text it was given: a caller that could
// not read the file passes no lines at all, and a file clipped at the read cap
// ends before a position the analyzer reported. Either way the location keeps
// the line numbers it was asked for and simply carries no content — moving the
// range to wherever the text happens to end would hand back a citation pointing
// at unrelated code.
func rangeLocation(path, language string, lines []string, start, end int) CodeLocation {
	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}
	content := ""
	if start <= len(lines) {
		end = min(end, len(lines))
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
		return referenceContextLess(result.Functions[i], result.Functions[j])
	})
	sort.Slice(result.OutsideFunctions, func(i, j int) bool {
		return referenceContextLess(result.OutsideFunctions[i], result.OutsideFunctions[j])
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
	if !r.Complete && r.PossibleReferenceCount > 0 {
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

// sortOccurrences orders occurrences totally. Several can share one line, and
// they are collected by iterating a map of candidate spellings, so anything
// short of a total order under an unstable sort reshuffles output run to run.
func sortOccurrences(in []ReferenceOccurrence) {
	sort.Slice(in, func(i, j int) bool {
		left, right := in[i], in[j]
		if !locationEqual(left.CodeLocation, right.CodeLocation) {
			return locationLess(left.CodeLocation, right.CodeLocation)
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.Role != right.Role {
			return left.Role < right.Role
		}
		return left.Confidence < right.Confidence
	})
}

// referenceContextLess totally orders reference contexts, which are likewise
// assembled from a map.
func referenceContextLess(a, b ReferenceContext) bool {
	if !locationEqual(a.CodeLocation, b.CodeLocation) {
		return locationLess(a.CodeLocation, b.CodeLocation)
	}
	return a.Name < b.Name
}

func locationLess(a, b CodeLocation) bool {
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	if a.LineRange.Start != b.LineRange.Start {
		return a.LineRange.Start < b.LineRange.Start
	}
	return a.LineRange.End < b.LineRange.End
}

func locationEqual(a, b CodeLocation) bool {
	return a.FilePath == b.FilePath && a.LineRange.Start == b.LineRange.Start && a.LineRange.End == b.LineRange.End
}

// Go implementation ---------------------------------------------------------

type goReferenceCandidate struct {
	key string
	// name and line mirror the key's cheapest components so a use can be
	// rejected without formatting one.
	name  string
	line  int
	obj   types.Object
	pkg   *packages.Package
	ident *ast.Ident
	path  string
	// file is held instead of its parent map. A common identifier such as `err`
	// is declared in nearly every file of a repository, and building one parent
	// map per matching file — each retained for as long as its candidate is —
	// cost hundreds of megabytes to produce a ten-entry ambiguity message. The
	// map is built for the candidates that actually become a target.
	file    *ast.File
	parents map[ast.Node]ast.Node
}

// parentMap builds the candidate's child-to-parent index on first use. The
// candidate must be addressed by pointer for the result to be reused.
func (c *goReferenceCandidate) parentMap() map[ast.Node]ast.Node {
	if c.parents == nil && c.file != nil {
		c.parents = goParentMap(c.file)
	}
	return c.parents
}

type goReferenceCacheEntry struct {
	mu          sync.Mutex
	fingerprint [sha256.Size]byte
	initialized bool
	pkgs        []*packages.Package
}

var goReferenceCache referenceCacheStore[goReferenceCacheEntry] // absolute repo root -> entry

// resolveGoDeclaration picks the single go/types object symbol names, without
// collecting its references: a caller that only needs the declaration's kind or
// location should not pay for a whole-repository occurrence scan.
func resolveGoDeclaration(repoRoot string, symbol SymbolRef, scope lookupScope, pkgs []*packages.Package, linesCache *referenceLineCache) (goReferenceCandidate, bool, error) {
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
			candidates = append(candidates, goReferenceCandidate{
				key: key, name: ident.Name, line: pkg.Fset.Position(obj.Pos()).Line,
				obj: obj, pkg: pkg, ident: ident, path: path, file: file,
			})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].path != candidates[j].path {
			return candidates[i].path < candidates[j].path
		}
		left := candidates[i].pkg.Fset.Position(candidates[i].ident.Pos())
		right := candidates[j].pkg.Fset.Position(candidates[j].ident.Pos())
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	if len(candidates) == 0 {
		// Always the not-found type, so callers can degrade to a literal
		// search. A package that failed to load may be why the symbol was
		// missed, so say so — but any package in the repository can carry load
		// errors, and letting that hide the fallback would defeat it.
		notFound := &SymbolNotFoundError{Name: symbol.Name, Path: symbol.Path}
		for _, pkg := range pkgs {
			if pkg != nil && len(pkg.Errors) > 0 {
				notFound.Reason = "package analysis failed: " + pkg.Errors[0].Msg
				break
			}
		}
		return goReferenceCandidate{}, complete, notFound
	}
	if pinned, requested := pinCandidatesToLine(candidates, symbol.Line, func(i int) LineRange {
		return goCandidateTarget(&candidates[i], linesCache).Definition.LineRange
	}); requested {
		if len(pinned) == 0 {
			return goReferenceCandidate{}, complete, &SymbolNotFoundError{
				Name: symbol.Name, Path: symbol.Path,
				Reason: fmt.Sprintf("no declaration at line %d", symbol.Line),
			}
		}
		candidates = pinned
	}
	if len(candidates) > 1 {
		return goReferenceCandidate{}, complete, ambiguousSymbol(symbol.Name, len(candidates), func(i int) ReferenceTarget {
			// The message prints kind, file and line only, so the declaration
			// source is not read back for a listing nobody sees.
			return goCandidateTarget(&candidates[i], nil)
		})
	}
	return candidates[0], complete, nil
}

// collectGoReferences gathers every use of an already-resolved declaration.
func collectGoReferences(repoRoot string, pkgs []*packages.Package, selected goReferenceCandidate, complete bool, linesCache *referenceLineCache) *ReferenceResult {
	result := &ReferenceResult{Target: goCandidateTarget(&selected, linesCache), Functions: []ReferenceContext{}, OutsideFunctions: []ReferenceContext{}, Complete: complete}
	if !complete {
		result.Notes = []string{"Go package loading reported errors; returned references may be incomplete"}
	}
	functionContexts := map[string]*ReferenceContext{}
	outsideContexts := map[string]*ReferenceContext{}
	seenOccurrences := map[string]struct{}{}
	unreadable := false
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
			// The snapshot outlives the worktree it was loaded from, so a file
			// can disappear between loading and reading. The use is still a
			// proven one: report it with its position and no content instead of
			// dropping it from a result that would still claim to be complete.
			lines, readable := linesCache.get(path)
			if !readable {
				unreadable = true
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
	if unreadable {
		result.Complete = false
		result.Notes = append(result.Notes, "source for at least one referencing file could not be read; those references are reported without content")
	}
	sortReferenceResult(result)
	return result
}

func loadGoReferencePackages(ctx context.Context, repoRoot string) ([]*packages.Package, error) {
	key, err := filepath.Abs(repoRoot)
	if err != nil {
		key = repoRoot
	}
	entry := goReferenceCache.entry(key)
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
	return referenceSourceFingerprint(ctx, repoRoot, files)
}

// sameGoObject runs for every identifier use in every loaded package, so it
// rejects on name and line — the key's cheapest components — before formatting
// a key. The key itself stays necessary: loading with tests gives a package and
// its test variant distinct objects for one declaration, so pointer identity
// alone would miss uses from the other variant.
func sameGoObject(fset *token.FileSet, obj types.Object, selected goReferenceCandidate) bool {
	if obj == selected.obj {
		return true
	}
	if obj.Name() != selected.name || fset.Position(obj.Pos()).Line != selected.line {
		return false
	}
	return goObjectKey(fset, obj) == selected.key
}

// goCandidateTarget describes one candidate declaration. A nil line cache asks
// for the location without its source, which is what an ambiguity listing needs.
func goCandidateTarget(candidate *goReferenceCandidate, lines *referenceLineCache) ReferenceTarget {
	parents := candidate.parentMap()
	node := goDefinitionNode(candidate.ident, parents)
	start, end := candidate.pkg.Fset.Position(candidate.ident.Pos()).Line, candidate.pkg.Fset.Position(candidate.ident.End()).Line
	if node != nil {
		start, end = candidate.pkg.Fset.Position(node.Pos()).Line, candidate.pkg.Fset.Position(node.End()).Line
	}
	source := sourceLines(lines, candidate.path)
	return ReferenceTarget{Name: candidate.ident.Name, Kind: goObjectKind(candidate.obj, candidate.ident, parents), Definition: rangeLocation(candidate.path, "go", source, start, end)}
}

// referenceLineCache reads each file at most once per lookup. Building a target
// needs the declaration's source, and every candidate is turned into a target
// at least once during line pinning and ambiguity reporting.
type referenceLineCache struct {
	repoRoot string
	lines    map[string][]string
	failed   map[string]struct{}
}

func newReferenceLineCache(repoRoot string) *referenceLineCache {
	return &referenceLineCache{repoRoot: repoRoot, lines: map[string][]string{}, failed: map[string]struct{}{}}
}

// get reports false when the file could not be read; callers decide whether to
// skip the occurrence or report a location without content.
func (c *referenceLineCache) get(path string) ([]string, bool) {
	if lines, ok := c.lines[path]; ok {
		return lines, true
	}
	if _, unreadable := c.failed[path]; unreadable {
		return nil, false
	}
	lines, err := readReferenceLines(c.repoRoot, path)
	if err != nil {
		c.failed[path] = struct{}{}
		return nil, false
	}
	c.lines[path] = lines
	return lines, true
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
				if !astContains(lhs, ident) {
					continue
				}
				if goAssignedIdent(lhs) != ident {
					// Appearing under a target is not being one: `m[key] = 1`
					// writes a map entry and only reads key and m.
					return "read"
				}
				if parent.Tok == token.ASSIGN || parent.Tok == token.DEFINE {
					return "write"
				}
				return "read_write"
			}
			return "read"
		case *ast.IncDecStmt:
			if goAssignedIdent(parent.X) == ident {
				return "read_write"
			}
			if astContains(parent.X, ident) {
				return "read"
			}
		case *ast.RangeStmt:
			if goAssignedIdent(parent.Key) == ident || goAssignedIdent(parent.Value) == ident {
				return "write"
			}
			if astContains(parent.Key, ident) || astContains(parent.Value, ident) {
				return "read"
			}
		case *ast.FuncDecl, *ast.FuncLit, *ast.GenDecl:
			return "read"
		}
	}
	return "read"
}

// goAssignedIdent returns the identifier an assignment target writes, or nil
// when the write goes through a container: an index or a pointer dereference
// stores into the value the expression selects, leaving every identifier in it
// merely read. A selector writes its field, not the operand it selects from.
func goAssignedIdent(target ast.Expr) *ast.Ident {
	switch typed := target.(type) {
	case *ast.Ident:
		return typed
	case *ast.ParenExpr:
		return goAssignedIdent(typed.X)
	case *ast.SelectorExpr:
		return typed.Sel
	}
	return nil
}

// astContains reports whether target appears under root. root is optional: a
// `for range x` has no Key or Value, and ast.Inspect panics on a nil node.
func astContains(root ast.Node, target ast.Node) bool {
	if root == nil {
		return false
	}
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
	data, _, err := readFileCapped(repoRoot, fullPath, toollimits.MaxRetrievedFileBytes)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), nil
}
