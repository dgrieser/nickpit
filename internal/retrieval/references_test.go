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
	location := rangeLocation("short.go", "go", []string{"package short", ""}, 910, 885)
	if location.LineRange.Start > location.LineRange.End || location.LineRange.Count != location.LineRange.End-location.LineRange.Start+1 {
		t.Fatalf("location = %#v", location)
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
