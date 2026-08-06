package retrieval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindReferencesBatchPreservesInputOrder(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "a.py", "FIRST = 1\n")
	writeRetrievalFile(t, repoRoot, "b.py", "SECOND = 2\n")

	results := NewLocalEngine().FindReferencesBatch(context.Background(), repoRoot, []SymbolRef{
		{Name: "SECOND", Path: "b.py"},
		{Name: "FIRST", Path: "a.py"},
	})
	if len(results) != 2 || results[0].Err != nil || results[1].Err != nil {
		t.Fatalf("batch results = %#v", results)
	}
	if results[0].Result.Target.Name != "SECOND" || results[1].Result.Target.Name != "FIRST" {
		t.Fatalf("batch order = %q, %q", results[0].Result.Target.Name, results[1].Result.Target.Name)
	}
}

func TestFindReferencesResolvesTypeScriptClassMethod(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "client.ts", "class Client {\n  fetch() { return 1 }\n}\nconst client = new Client()\nclient.fetch()\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "fetch", Path: "client.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != "function" || result.Target.Definition.LineRange.Start != 2 {
		t.Fatalf("method target = %#v", result.Target)
	}
}

// A decorated Python definition is reported by the parser with a span starting
// at the decorator and by the declaration regex at the def line; the two must
// collapse into one candidate instead of an ambiguity.
func TestFindReferencesResolvesDecoratedPythonFunction(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", "import functools\n\n@functools.cache\ndef helper(x):\n    return x\n\nhelper(1)\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "helper", Path: "mod.py"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != "function" || result.Target.Definition.LineRange.Start != 3 || result.Target.Definition.LineRange.End != 5 {
		t.Fatalf("decorated target = %#v", result.Target)
	}
	// The `def helper` line sits inside the decorated span but below its start;
	// it is the declaration, not a reference to itself. The call is in the
	// declaring file's own scope, the strongest binding evidence available.
	if result.ExactReferenceCount != 1 || result.PossibleReferenceCount != 0 {
		t.Fatalf("counts = %d exact / %d possible, want the call on line 7 as exact: %#v", result.ExactReferenceCount, result.PossibleReferenceCount, result)
	}
}

func TestRepoWideGoReferencesIgnoreRawStringDeclarations(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/rawrefs\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "real.go", "package rawrefs\n\nfunc Foo() {}\n")
	writeRetrievalFile(t, repoRoot, "raw.go", "package rawrefs\n\nvar example = `\nfunc Foo() {}\n`\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Foo"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Definition.FilePath != "real.go" || result.Target.Definition.LineRange.Start != 3 {
		t.Fatalf("Foo target = %#v", result.Target)
	}
}

func TestFindGoReferencesGroupsFunctionsAndTopLevelUses(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/refs\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "state.go", `package refs

var Shared = 1

func Use() int {
	Shared++
	return Shared
}
`)
	writeRetrievalFile(t, repoRoot, "defaults.go", `package refs

var Default = Shared
`)

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Shared", Path: "state.go"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != "variable" || result.Target.Definition.LineRange.Start != 3 {
		t.Fatalf("target = %#v", result.Target)
	}
	if !result.Complete || result.ExactReferenceCount != 3 || result.PossibleReferenceCount != 0 {
		t.Fatalf("counts/complete = %d/%d/%t", result.ExactReferenceCount, result.PossibleReferenceCount, result.Complete)
	}
	if len(result.Functions) != 1 || result.Functions[0].Name != "Use" || !strings.Contains(result.Functions[0].CodeLocation.Content, "func Use() int") {
		t.Fatalf("functions = %#v", result.Functions)
	}
	if len(result.Functions[0].References) != 2 || result.Functions[0].References[0].Role != "read_write" {
		t.Fatalf("function references = %#v", result.Functions[0].References)
	}
	if len(result.OutsideFunctions) != 1 || result.OutsideFunctions[0].CodeLocation.FilePath != "defaults.go" {
		t.Fatalf("outside functions = %#v", result.OutsideFunctions)
	}
}

func TestFindGoReferencesWithoutPathFindsGroupedConstant(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/grouped\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "main.go", `package grouped

const (
	Limit = 3
)

func Read() int { return Limit }
`)
	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Limit"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != "constant" || result.Target.Definition.LineRange.Start != 4 || result.ExactReferenceCount != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestFindGoReferencesAcrossPackages(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/crossrefs\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "state/state.go", "package state\n\nvar Shared = 1\n")
	writeRetrievalFile(t, repoRoot, "use/use.go", `package use

import "example.com/crossrefs/state"

func Read() int { return state.Shared }
`)
	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Shared", Path: "state/state.go"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExactReferenceCount != 1 || len(result.Functions) != 1 || result.Functions[0].CodeLocation.FilePath != "use/use.go" {
		t.Fatalf("result = %#v", result)
	}
}

func TestFindGoReferencesRejectsUseSiteAsDeclarationPath(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/multifile\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "a_use.go", "package multifile\n\nfunc read() int { return Shared }\n")
	writeRetrievalFile(t, repoRoot, "z_decl.go", "package multifile\n\nvar Shared = 1\n")

	_, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Shared", Path: "a_use.go"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestFindGoReferencesUsesDefinitionIdentifiersOwnFile(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/multifilelocation\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "a_use.go", "package multifilelocation\n\nfunc read() int { return Shared }\n")
	writeRetrievalFile(t, repoRoot, "z_decl.go", "package multifilelocation\n\nvar Shared = 1\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Shared"})
	if err != nil {
		t.Fatal(err)
	}
	definition := result.Target.Definition
	if definition.FilePath != "z_decl.go" || definition.LineRange.Start > definition.LineRange.End || definition.LineRange.Count != definition.LineRange.End-definition.LineRange.Start+1 || !strings.Contains(definition.Content, "var Shared = 1") {
		t.Fatalf("definition = %#v", definition)
	}
}

func TestFindPythonReferencesFollowsAliasAndReturnsGlobalWrite(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "config.py", "LIMIT = 1\nLIMIT = 2\n")
	writeRetrievalFile(t, repoRoot, "service.py", `from config import LIMIT as cap

def run():
    return cap + cap
`)

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "LIMIT", Path: "config.py"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Complete || result.ExactReferenceCount < 3 || result.PossibleReferenceCount == 0 {
		t.Fatalf("counts/complete = %d/%d/%t", result.ExactReferenceCount, result.PossibleReferenceCount, result.Complete)
	}
	if len(result.Functions) != 1 || result.Functions[0].Name != "run" || len(result.Functions[0].References) != 2 {
		t.Fatalf("functions = %#v", result.Functions)
	}
	foundWrite := false
	for _, context := range result.OutsideFunctions {
		for _, reference := range context.References {
			if reference.CodeLocation.FilePath == "config.py" && reference.CodeLocation.LineRange.Start == 2 && reference.Role == "write" {
				foundWrite = true
			}
		}
	}
	if !foundWrite {
		t.Fatalf("global reassignment missing: %#v", result.OutsideFunctions)
	}
}

func TestFindParsedReferencesInvalidatesCacheAfterEdit(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "config.py", "VALUE = 1\n")
	writeRetrievalFile(t, repoRoot, "use.py", "def read():\n    return VALUE\n")
	engine := NewLocalEngine()

	first, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "VALUE", Path: "config.py"})
	if err != nil {
		t.Fatal(err)
	}
	if got := first.ExactReferenceCount + first.PossibleReferenceCount; got != 1 {
		t.Fatalf("first references = %d, want 1", got)
	}

	writeRetrievalFile(t, repoRoot, "use.py", "def read():\n    return VALUE + VALUE\n")
	second, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "VALUE", Path: "config.py"})
	if err != nil {
		t.Fatal(err)
	}
	if got := second.ExactReferenceCount + second.PossibleReferenceCount; got != 2 {
		t.Fatalf("second references = %d, want 2", got)
	}
}

func TestFindGoReferencesInvalidatesCacheAfterEdit(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/cache\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "state.go", "package cache\n\nvar Shared = 1\n")
	writeRetrievalFile(t, repoRoot, "use.go", "package cache\n\nfunc read() int { return Shared }\n")
	engine := NewLocalEngine()

	first, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Shared", Path: "state.go"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ExactReferenceCount != 1 {
		t.Fatalf("first exact references = %d, want 1", first.ExactReferenceCount)
	}

	writeRetrievalFile(t, repoRoot, "use.go", "package cache\n\nfunc read() int { return Shared + Shared }\n")
	second, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Shared", Path: "state.go"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ExactReferenceCount != 2 {
		t.Fatalf("second exact references = %d, want 2", second.ExactReferenceCount)
	}
}

func TestFindReferencesDirectoryWithoutSupportedDeclarationRequestsFallback(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "legacy/state.rb", "VALUE = 1\n")

	for _, path := range []string{"", "legacy"} {
		_, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "VALUE", Path: path})
		var unsupported *UnsupportedLanguageError
		if !errors.As(err, &unsupported) {
			t.Fatalf("path %q error = %v, want *UnsupportedLanguageError", path, err)
		}
	}
}

func TestFindReferencesSupportsNodeAndRustBindings(t *testing.T) {
	tests := []struct {
		name, definitionPath, definition, usePath, use, symbol, function string
	}{
		{
			name: "node", definitionPath: "config.ts", definition: "export const LIMIT = 3\n",
			usePath: "bridge.ts", use: "export { LIMIT as CAP } from \"./config\"\n",
			symbol: "LIMIT", function: "run",
		},
		{
			name: "rust", definitionPath: "src/config.rs", definition: "const LIMIT: i32 = 3;\n",
			usePath: "src/lib.rs", use: "use crate::config::LIMIT as cap;\nfn run() -> i32 { cap }\n",
			symbol: "LIMIT", function: "run",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			writeRetrievalFile(t, repoRoot, tt.definitionPath, tt.definition)
			if tt.usePath != "" {
				writeRetrievalFile(t, repoRoot, tt.usePath, tt.use)
			}
			if tt.name == "node" {
				writeRetrievalFile(t, repoRoot, "service.ts", "import { CAP as cap } from \"./bridge\"\nexport function run() { return cap }\n")
			}
			result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: tt.symbol, Path: tt.definitionPath})
			if err != nil {
				t.Fatal(err)
			}
			if result.Target.Name != tt.symbol || len(result.Functions) != 1 || result.Functions[0].Name != tt.function {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestFindRustReferencesDoesNotMaskLifetimeLine(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "src/config.rs", "pub const LIMIT: i32 = 3;\n")
	writeRetrievalFile(t, repoRoot, "src/lib.rs", "use crate::config::LIMIT;\nfn get() -> &'static i32 { &LIMIT }\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "LIMIT", Path: "src/config.rs"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, function := range result.Functions {
		for _, reference := range function.References {
			if reference.CodeLocation.FilePath == "src/lib.rs" && reference.CodeLocation.LineRange.Start == 2 {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("lifetime-line reference missing: %#v", result.Functions)
	}
}

func TestFindReferencesReportsAmbiguousDeclarations(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "a.py", "VALUE = 1\n")
	writeRetrievalFile(t, repoRoot, "b.py", "VALUE = 2\n")

	_, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "VALUE"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "a.py:1") || !strings.Contains(err.Error(), "b.py:1") {
		t.Fatalf("error = %v", err)
	}
}

func TestAmbiguousSymbolErrorCapsCandidateList(t *testing.T) {
	candidates := make([]ReferenceTarget, 15)
	for i := range candidates {
		candidates[i] = ReferenceTarget{Kind: "variable", Definition: CodeLocation{FilePath: fmt.Sprintf("file-%02d.go", i), LineRange: LineRange{Start: i + 1}}}
	}
	message := (&AmbiguousSymbolError{Name: "value", Candidates: candidates}).Error()
	if !strings.Contains(message, "and 5 more") || strings.Contains(message, "file-10.go") {
		t.Fatalf("message = %q", message)
	}
}

func TestRangeLocationAlwaysReturnsConsistentRange(t *testing.T) {
	for _, tt := range []struct {
		name       string
		lines      []string
		start, end int
	}{
		{name: "beyond end of file", lines: []string{"package short", ""}, start: 910, end: 885},
		{name: "unreadable file", lines: nil, start: 3, end: 7},
		{name: "unreadable file at line one", lines: []string{}, start: 1, end: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			location := rangeLocation("short.go", "go", tt.lines, tt.start, tt.end)
			if location.LineRange.Start < 1 || location.LineRange.Start > location.LineRange.End || location.LineRange.Count != location.LineRange.End-location.LineRange.Start+1 {
				t.Fatalf("location = %#v", location)
			}
		})
	}
}

// A position past the end of the text the reader could return names a line
// that is genuinely unavailable. Moving it onto the last line that was read
// would cite unrelated code under the reported number.
func TestRangeLocationKeepsLineNumbersPastAvailableText(t *testing.T) {
	location := rangeLocation("clipped.go", "go", []string{"package short", "", "var x = 1"}, 200000, 200000)
	if location.LineRange.Start != 200000 || location.LineRange.End != 200000 {
		t.Fatalf("location moved onto readable text: %#v", location)
	}
	if location.Content != "" {
		t.Fatalf("location content = %q, want empty", location.Content)
	}
}

// A use inside a function that binds the same name refers to that binding, not
// to the symbol being looked up, so the declaring file's own scope cannot make
// it exact.
func TestFindPythonReferencesDowngradesShadowedUses(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", strings.Join([]string{
		"config = load()",
		"",
		"def unrelated():",
		"    config = {}",
		"    return config['a']",
		"",
		"def uses_module_config():",
		"    return config",
		"",
	}, "\n"))

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "config", Path: "mod.py", Line: 1})
	if err != nil {
		t.Fatal(err)
	}
	confidenceAt := map[int]string{}
	for _, context := range append(append([]ReferenceContext{}, result.Functions...), result.OutsideFunctions...) {
		for _, occurrence := range context.References {
			confidenceAt[occurrence.CodeLocation.LineRange.Start] = occurrence.Confidence
		}
	}
	for _, line := range []int{4, 5} {
		if confidenceAt[line] != "possible" {
			t.Fatalf("line %d confidence = %q, want possible: %#v", line, confidenceAt[line], confidenceAt)
		}
	}
	if confidenceAt[8] != "exact" {
		t.Fatalf("unshadowed use confidence = %q, want exact: %#v", confidenceAt[8], confidenceAt)
	}
	if result.Complete {
		t.Fatal("result claims completeness with unproven references")
	}
}

// A brace body inside a grouped declaration is not the group's own scope: its
// members keep the group's paren depth, so an interface method or struct field
// would otherwise be reported as a declaration of the group's kind and make
// every real declaration of that name ambiguous.
func TestFindGoReferencesIgnoresBraceBodyInsideGroupedDeclaration(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/grouped\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "iface.go", "package grouped\n\ntype (\n\tRunner interface {\n\t\tRun() error\n\t}\n)\n")
	writeRetrievalFile(t, repoRoot, "run.go", "package grouped\n\nfunc Run() error { return nil }\n\nfunc Call() error { return Run() }\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Run"})
	if err != nil {
		t.Fatalf("interface method was treated as a rival declaration: %v", err)
	}
	if result.Target.Definition.FilePath != "run.go" || result.Target.Kind != "function" {
		t.Fatalf("target = %#v", result.Target)
	}
	if result.ExactReferenceCount != 1 {
		t.Fatalf("exact references = %d, want the call in Call: %s", result.ExactReferenceCount, result.Render())
	}
}

// The loaded snapshot outlives the worktree it was built from. A use whose file
// can no longer be read is still a proven use, and a result that quietly drops
// it must not go on claiming to list them all.
func TestCollectGoReferencesReportsUnreadableSources(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/gone\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "value.go", "package gone\n\nvar Value = 1\n\nfunc Use() int { return Value }\n")

	pkgs, err := loadGoReferencePackages(context.Background(), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	scope := lookupScope{Path: "value.go", IsFile: true}
	selected, complete, err := resolveGoDeclaration(repoRoot, SymbolRef{Name: "Value", Path: "value.go"}, scope, pkgs, newReferenceLineCache(repoRoot))
	if err != nil {
		t.Fatal(err)
	}
	// A cache rooted elsewhere stands in for files the checkout moved away.
	result := collectGoReferences(repoRoot, pkgs, selected, complete, newReferenceLineCache(t.TempDir()))
	if result.ExactReferenceCount == 0 {
		t.Fatalf("unreadable sources dropped every reference: %#v", result)
	}
	if result.Complete {
		t.Fatalf("result claims completeness despite unreadable sources: %#v", result)
	}
}

func TestFindReferencesIgnoresCommentsAndStrings(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "config.py", "VALUE = 1\n# VALUE\ntext = 'VALUE'\n")
	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "VALUE", Path: "config.py"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExactReferenceCount != 0 || result.PossibleReferenceCount != 0 {
		t.Fatalf("unexpected references: %#v", result)
	}
}

func TestFindReferencesIgnoresMultiLineTemplateLiterals(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "docs.ts", "export function render() { return 1 }\nconst usage = `\nrender the page\ncall render()\n`\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "render", Path: "docs.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExactReferenceCount != 0 || result.PossibleReferenceCount != 0 {
		t.Fatalf("template literal reported as references: %#v", result)
	}
}

func TestReferenceCacheEvictsLeastRecentlyUsedRoots(t *testing.T) {
	t.Setenv("NICKPIT_REFERENCE_CACHE_MAX_ENTRIES", "2")
	var cache referenceCacheStore[parsedReferenceCacheEntry]

	first := cache.entry("/repo/a")
	cache.entry("/repo/b")
	if got := cache.entry("/repo/a"); got != first {
		t.Fatal("cached root was not reused")
	}
	cache.entry("/repo/c") // evicts /repo/b, the least recently used

	if len(cache.entries) != 2 {
		t.Fatalf("cache holds %d roots, want 2", len(cache.entries))
	}
	if _, ok := cache.entries["/repo/b"]; ok {
		t.Fatal("least recently used root survived eviction")
	}
	if got := cache.entry("/repo/a"); got != first {
		t.Fatal("recently used root was evicted")
	}
}

func TestReferenceCacheCapZeroDisablesEviction(t *testing.T) {
	t.Setenv("NICKPIT_REFERENCE_CACHE_MAX_ENTRIES", "0")
	var cache referenceCacheStore[parsedReferenceCacheEntry]
	for i := range 20 {
		cache.entry(fmt.Sprintf("/repo/%d", i))
	}
	if len(cache.entries) != 20 {
		t.Fatalf("cache holds %d roots, want 20", len(cache.entries))
	}
}

// Two same-named Go declarations in one file cannot be told apart by --path, so
// the declaration line is the only usable disambiguator.
func TestFindGoReferencesPinsAmbiguousDeclarationByLine(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/pinned\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "types.go", "package pinned\n\ntype A struct{}\n\nfunc (A) String() string { return \"a\" }\n\ntype B struct{}\n\nfunc (B) String() string { return \"b\" }\n")

	engine := NewLocalEngine()
	_, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "String", Path: "types.go"})
	var ambiguous *AmbiguousSymbolError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("error = %v, want ambiguous", err)
	}
	if !strings.Contains(err.Error(), "line") {
		t.Fatalf("ambiguity message does not point at the disambiguator: %v", err)
	}

	result, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "String", Path: "types.go", Line: 9})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Definition.LineRange.Start != 9 {
		t.Fatalf("pinned target = %#v", result.Target)
	}
}

func TestFindReferencesPinsAmbiguousParsedDeclarationByLine(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "handlers.ts", "class A {\n  run() { return 1 }\n}\nclass B {\n  run() { return 2 }\n}\n")

	engine := NewLocalEngine()
	if _, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "run", Path: "handlers.ts"}); err == nil {
		t.Fatal("expected an ambiguity")
	}
	result, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "run", Path: "handlers.ts", Line: 5})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Definition.LineRange.Start != 5 {
		t.Fatalf("pinned target = %#v", result.Target)
	}
}

// A line matching no declaration must be reported, not silently resolved to
// some other declaration of the same name.
func TestFindReferencesReportsLineThatPinsNothing(t *testing.T) {
	for _, tt := range []struct{ name, file, content, symbol string }{
		{"several declarations", "handlers.ts", "class A {\n  run() { return 1 }\n}\nclass B {\n  run() { return 2 }\n}\n", "run"},
		{"one declaration", "single.ts", "export function run() { return 1 }\n", "run"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			writeRetrievalFile(t, repoRoot, tt.file, tt.content)

			_, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: tt.symbol, Path: tt.file, Line: 99})
			var notFound *SymbolNotFoundError
			if !errors.As(err, &notFound) || notFound.Reason != "no declaration at line 99" {
				t.Fatalf("error = %v (%T)", err, err)
			}
		})
	}
}

// `for range x` leaves RangeStmt.Key and .Value nil, which ast.Inspect refuses
// to walk. Reaching a reference through one used to kill the process.
func TestFindGoReferencesHandlesRangeWithoutKeyOrValue(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/rangenil\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "loop.go", "package rangenil\n\nvar Items = []int{1}\n\nfunc Count() int {\n\tn := 0\n\tfor range Items {\n\t\tn++\n\t}\n\treturn n\n}\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Items", Path: "loop.go"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExactReferenceCount != 1 {
		t.Fatalf("reference count = %d, want the range clause: %#v", result.ExactReferenceCount, result)
	}
}

// Passing a symbol as an argument is a use, not a parameter declaration.
func TestFindReferencesDoesNotTreatCallArgumentsAsParameters(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "app.ts", "export const total = 1\nfunction show(v: number) { return v }\nshow(total)\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "total", Path: "app.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != "variable" || result.Target.Definition.LineRange.Start != 1 {
		t.Fatalf("target = %#v", result.Target)
	}
}

// Multiplication is spaced on both sides; only a pointer type binds to its name.
func TestFindGoReferencesDoesNotTreatMultiplicationAsParameter(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/mul\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "a.go", "package mul\n\nvar Factor = 2\n")
	writeRetrievalFile(t, repoRoot, "b.go", "package mul\n\nfunc Scale(n int) int { return Factor * n }\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Factor"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Definition.FilePath != "a.go" || result.Target.Kind != "variable" {
		t.Fatalf("target = %#v", result.Target)
	}
}

// The tightened parameter patterns must still recognize real declarations.
func TestFindReferencesClassifiesParameterDeclarations(t *testing.T) {
	for _, tt := range []struct{ name, file, content, symbol string }{
		{"js function", "a.js", "function show(value) { return value }\n", "value"},
		{"ts arrow", "c.ts", "const show = (value: number) => value\n", "value"},
		{"ts class method", "e.ts", "class A {\n  show(value: number) { return value }\n}\n", "value"},
		{"go pointer", "p.go", "package params\n\ntype Thing struct{}\n\nfunc Use(item *Thing) {}\n", "item"},
		{"go value", "q.go", "package params\n\nfunc Count(total int) int { return total }\n", "total"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/params\n\ngo 1.25\n")
			writeRetrievalFile(t, repoRoot, tt.file, tt.content)

			result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: tt.symbol, Path: tt.file})
			if err != nil {
				t.Fatal(err)
			}
			if result.Target.Kind != "parameter" {
				t.Fatalf("target = %#v, want a parameter", result.Target)
			}
		})
	}
}

// Occurrences are collected by iterating maps, so anything short of a total
// order under sort.Slice reshuffles the result between runs.
func TestFindReferencesOrdersSameLineOccurrencesDeterministically(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/ordering\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "calc.go", "package ordering\n\nvar Base = 1\n\nfunc Sum() int { return Base + Base + Base }\n")

	var first string
	for range 20 {
		result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Base", Path: "calc.go"})
		if err != nil {
			t.Fatal(err)
		}
		rendered := fmt.Sprintf("%#v", result)
		if first == "" {
			first = rendered
			continue
		}
		if rendered != first {
			t.Fatalf("reference order is not stable:\nfirst: %s\nnow:   %s", first, rendered)
		}
	}
}

// Resolving a declaration must agree with the full analysis while skipping the
// occurrence scan, so callers that only need the kind can stop early.
func TestResolveReferenceTargetsMatchesFullAnalysis(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/resolve\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "app.go", "package resolve\n\nvar Limit = 1\n\nfunc Run() int { return Limit }\n")
	writeRetrievalFile(t, repoRoot, "app.ts", "export const label = \"x\"\n")

	engine := NewLocalEngine()
	symbols := []SymbolRef{
		{Name: "Run", Path: "app.go"},
		{Name: "Limit", Path: "app.go"},
		{Name: "label", Path: "app.ts"},
		{Name: "Missing", Path: "app.go"},
	}
	targets := engine.ResolveReferenceTargets(context.Background(), repoRoot, symbols)
	if len(targets) != len(symbols) {
		t.Fatalf("got %d results for %d symbols", len(targets), len(symbols))
	}

	var notFound *SymbolNotFoundError
	if !errors.As(targets[3].Err, &notFound) {
		t.Fatalf("absent symbol error = %v", targets[3].Err)
	}
	for i, symbol := range symbols[:3] {
		if targets[i].Err != nil {
			t.Fatalf("%s: %v", symbol.Name, targets[i].Err)
		}
		full, err := engine.FindReferences(context.Background(), repoRoot, symbol)
		if err != nil {
			t.Fatalf("%s: %v", symbol.Name, err)
		}
		if *targets[i].Target != full.Target {
			t.Fatalf("%s: resolved %#v, full analysis %#v", symbol.Name, *targets[i].Target, full.Target)
		}
	}
	if targets[0].Target.Kind != "function" || targets[1].Target.Kind != "variable" {
		t.Fatalf("kinds = %q, %q", targets[0].Target.Kind, targets[1].Target.Kind)
	}
}

func TestResolveReferenceTargetsReportsAmbiguity(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "handlers.ts", "class A {\n  run() { return 1 }\n}\nclass B {\n  run() { return 2 }\n}\n")

	targets := NewLocalEngine().ResolveReferenceTargets(context.Background(), repoRoot, []SymbolRef{{Name: "run", Path: "handlers.ts"}})
	var ambiguous *AmbiguousSymbolError
	if !errors.As(targets[0].Err, &ambiguous) {
		t.Fatalf("error = %v, want ambiguous", targets[0].Err)
	}
}

// Any package in the repository can carry load errors. Letting that turn the
// miss into a plain error would suppress the caller's literal-search fallback.
func TestFindGoReferencesReportsAbsentSymbolDespiteBrokenPackage(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/broken\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "app.go", "package broken\n\nfunc Run() {}\n")
	writeRetrievalFile(t, repoRoot, "bad/bad.go", "package bad\n\nfunc Oops() { undefinedCall() }\n")

	_, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "Missing", Path: "app.go"})
	var notFound *SymbolNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("error = %v (%T), want *SymbolNotFoundError", err, err)
	}
	if notFound.Reason == "" || !strings.Contains(err.Error(), "package analysis failed") {
		t.Fatalf("error lost the load-failure detail: %v", err)
	}
}

func TestIdentifierColumnsRejectsEmptyName(t *testing.T) {
	if got := identifierColumns("value = 1", ""); got != nil {
		t.Fatalf("columns = %v, want none", got)
	}
}

// A same-named identifier in another language is a different symbol; no import
// binds a Python name to a TypeScript one.
func TestFindReferencesIgnoresOtherLanguages(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "settings.py", "TIMEOUT = 1\nprint(TIMEOUT)\n")
	writeRetrievalFile(t, repoRoot, "client.ts", "const TIMEOUT = 2\nconsole.log(TIMEOUT)\n")
	writeRetrievalFile(t, repoRoot, "lib.rs", "pub const TIMEOUT: u32 = 3;\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "TIMEOUT", Path: "settings.py"})
	if err != nil {
		t.Fatal(err)
	}
	for _, context := range append(append([]ReferenceContext{}, result.Functions...), result.OutsideFunctions...) {
		if context.CodeLocation.FilePath != "settings.py" {
			t.Fatalf("reference from another language: %#v", context.CodeLocation)
		}
	}
	if result.ExactReferenceCount+result.PossibleReferenceCount != 1 {
		t.Fatalf("reference count = %d exact / %d possible, want only the Python use", result.ExactReferenceCount, result.PossibleReferenceCount)
	}
}

// The cache must notice an edit. mtime moves on any real write, so an explicit
// timestamp bump stands in for the wall-clock gap a real edit has.
func TestParsedReferenceSnapshotRefreshesAfterEdit(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", "VALUE = 1\n")

	first, err := loadParsedReferenceFiles(context.Background(), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(first[0].masked) < 1 || !strings.Contains(first[0].masked[0], "VALUE = 1") {
		t.Fatalf("first snapshot = %#v", first)
	}

	path := filepath.Join(repoRoot, "mod.py")
	if err := os.WriteFile(path, []byte("VALUE = 2\nOTHER = 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	second, err := loadParsedReferenceFiles(context.Background(), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || !strings.Contains(second[0].masked[0], "VALUE = 2") {
		t.Fatalf("snapshot was not refreshed: %#v", second[0].masked)
	}
}

// A grouped entry whose value spans lines closes parens of its own; treating
// the first of those as the end of the group hides every later entry.
func TestFindGoReferencesFindsEntryAfterMultiLineGroupedValue(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/grouped\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "decls.go", "package grouped\n\nimport \"fmt\"\n\nvar (\n\tfirst = fmt.Sprintf(\n\t\t\"%d\", 1,\n\t)\n\tsecond = 2\n)\n\nfunc Use() string { return first + fmt.Sprint(second) }\n")

	engine := NewLocalEngine()
	for _, name := range []string{"first", "second"} {
		result, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: name})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Target.Kind != "variable" || result.ExactReferenceCount != 1 {
			t.Fatalf("%s target = %#v (%d refs)", name, result.Target, result.ExactReferenceCount)
		}
	}
}

// An import binds a name declared elsewhere; counting it as a rival
// declaration made every widely-imported symbol resolve as ambiguous.
func TestFindReferencesResolvesSymbolImportedElsewhere(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", "LIMIT = 10\n")
	writeRetrievalFile(t, repoRoot, "other.py", "from mod import LIMIT\n\ndef use():\n    return LIMIT\n")
	writeRetrievalFile(t, repoRoot, "third.py", "from mod import LIMIT\n\ndef also():\n    return LIMIT\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "LIMIT"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != "variable" || result.Target.Definition.FilePath != "mod.py" {
		t.Fatalf("target = %#v", result.Target)
	}
}

// A file that only imports the symbol still has to answer a lookup scoped to
// it: the import is the only thing there to point at.
func TestFindReferencesResolvesImportWhenScopedToImportingFile(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", "LIMIT = 10\n")
	writeRetrievalFile(t, repoRoot, "other.py", "from mod import LIMIT\n\ndef use():\n    return LIMIT\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "LIMIT", Path: "other.py"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Kind != "import" || result.Target.Definition.FilePath != "other.py" {
		t.Fatalf("target = %#v", result.Target)
	}
}

// The declaring file's own scope is the strongest binding evidence there is;
// it must not be reported as less certain than a use reached via an import.
func TestFindReferencesDoesNotInvertConfidence(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", "LIMIT = 10\n\ndef local():\n    return LIMIT\n")
	writeRetrievalFile(t, repoRoot, "other.py", "from mod import LIMIT\n\ndef remote():\n    return LIMIT\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "LIMIT"})
	if err != nil {
		t.Fatal(err)
	}
	confidence := map[string]string{}
	for _, context := range append(append([]ReferenceContext{}, result.Functions...), result.OutsideFunctions...) {
		for _, reference := range context.References {
			confidence[fmt.Sprintf("%s:%d", reference.CodeLocation.FilePath, reference.CodeLocation.LineRange.Start)] = reference.Confidence
		}
	}
	if got := confidence["mod.py:4"]; got != "exact" {
		t.Fatalf("declaring file use = %q, want exact: %#v", got, confidence)
	}
	if result.PossibleReferenceCount != 0 {
		t.Fatalf("possible count = %d, want none: %#v", result.PossibleReferenceCount, confidence)
	}
	if strings.Contains(result.Render(), "analysis incomplete") {
		t.Fatalf("clean lookup reported as incomplete:\n%s", result.Render())
	}
}

// Writing through a container reads the identifiers that select it. Reporting
// them as writes fabricates mutation sites for the exact question this tool
// exists to answer.
func TestFindGoReferencesRolesForContainerAssignments(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "go.mod", "module example.com/roles\n\ngo 1.25\n")
	writeRetrievalFile(t, repoRoot, "roles.go", `package roles

type Box struct{ Field int }

func Mutate(m map[string]int, arr []int, p *Box, key string, idx int, direct int) {
	m[key] = 1
	arr[idx] = 5
	p.Field = 2
	m[key]++
	direct = 3
	direct++
	_ = direct
}
`)

	engine := NewLocalEngine()
	for _, tt := range []struct{ symbol, want string }{
		{"key", "read"},
		{"idx", "read"},
		{"p", "read"},
		{"m", "read"},
	} {
		result, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: tt.symbol, Path: "roles.go"})
		if err != nil {
			t.Fatalf("%s: %v", tt.symbol, err)
		}
		for _, context := range result.Functions {
			for _, reference := range context.References {
				if reference.Role != tt.want {
					t.Fatalf("%s at line %d = %q, want %q", tt.symbol, reference.CodeLocation.LineRange.Start, reference.Role, tt.want)
				}
			}
		}
	}

	// A plain identifier target is still a write, and ++ on it a read_write.
	result, err := engine.FindReferences(context.Background(), repoRoot, SymbolRef{Name: "direct", Path: "roles.go"})
	if err != nil {
		t.Fatal(err)
	}
	roles := map[int]string{}
	for _, context := range result.Functions {
		for _, reference := range context.References {
			roles[reference.CodeLocation.LineRange.Start] = reference.Role
		}
	}
	if roles[10] != "write" || roles[11] != "read_write" || roles[12] != "read" {
		t.Fatalf("direct roles = %#v", roles)
	}
}

// The catalog advertises `import` as a usage kind; an unrenamed import is the
// common case and must carry that role, not be filed as an ordinary read.
func TestFindReferencesLabelsUnrenamedImports(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", "LIMIT = 10\n")
	writeRetrievalFile(t, repoRoot, "other.py", "from mod import LIMIT\n\ndef use():\n    return LIMIT\n")

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "LIMIT"})
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, context := range append(append([]ReferenceContext{}, result.Functions...), result.OutsideFunctions...) {
		for _, reference := range context.References {
			roles[fmt.Sprintf("%s:%d", reference.CodeLocation.FilePath, reference.CodeLocation.LineRange.Start)] = reference.Role
		}
	}
	if roles["other.py:1"] != "import" {
		t.Fatalf("import line role = %q, want import: %#v", roles["other.py:1"], roles)
	}
	if roles["other.py:4"] != "read" {
		t.Fatalf("use role = %q, want read: %#v", roles["other.py:4"], roles)
	}
}

// Enumeration and reading are separate steps, and the daemon reviews worktrees
// a checkout can still be touching. A file that vanishes in between must not
// fail every lookup in the process.
func TestParsedReferenceSnapshotSkipsUnreadableFiles(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "mod.py", "LIMIT = 10\n\ndef use():\n    return LIMIT\n")
	writeRetrievalFile(t, repoRoot, "gone.py", "OTHER = 1\n")

	unreadable := filepath.Join(repoRoot, "gone.py")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })
	if _, err := os.ReadFile(unreadable); err == nil {
		t.Skip("running as a user that ignores file permissions")
	}

	result, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "LIMIT"})
	if err != nil {
		t.Fatalf("unreadable sibling failed the lookup: %v", err)
	}
	if result.Target.Definition.FilePath != "mod.py" {
		t.Fatalf("target = %#v", result.Target)
	}
}
