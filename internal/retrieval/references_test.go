package retrieval

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
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
	// it is the declaration, not a reference to itself.
	if result.PossibleReferenceCount != 1 {
		t.Fatalf("reference count = %d, want 1 (the call on line 7): %#v", result.PossibleReferenceCount, result)
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

// A line matching no declaration must not silently resolve to the wrong one.
func TestFindReferencesIgnoresLineThatPinsNothing(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "handlers.ts", "class A {\n  run() { return 1 }\n}\nclass B {\n  run() { return 2 }\n}\n")

	_, err := NewLocalEngine().FindReferences(context.Background(), repoRoot, SymbolRef{Name: "run", Path: "handlers.ts", Line: 99})
	var ambiguous *AmbiguousSymbolError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("error = %v, want ambiguous", err)
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
