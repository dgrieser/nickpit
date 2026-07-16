package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectLanguageNormalizesNodeFamily(t *testing.T) {
	tests := map[string]string{
		"main.py":   "python",
		"main.js":   "nodejs",
		"main.mjs":  "nodejs",
		"main.cjs":  "nodejs",
		"main.ts":   "nodejs",
		"main.mts":  "nodejs",
		"main.cts":  "nodejs",
		"main.jsx":  "nodejs",
		"main.tsx":  "nodejs",
		"main.go":   "go",
		"README.md": "markdown",
	}
	for path, want := range tests {
		if got := detectLanguage(path); got != want {
			t.Fatalf("detectLanguage(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestLocalEngineGetSymbolSupportsPythonAndNodeScopes(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "pkg/worker.py", `async def Fetch():
    return 1

class Service:
    def Run(self):
        return Fetch()
`)
	writeRetrievalFile(t, repoRoot, "pkg/web/index.ts", `export const Run = () => {
  return helper()
}

export function helper() {
  return 1
}
`)

	engine := NewLocalEngine()

	pythonSymbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "Run", Path: "pkg/worker.py"})
	if err != nil {
		t.Fatal(err)
	}
	if pythonSymbol.Path != "pkg/worker.py" || pythonSymbol.Language != "python" {
		t.Fatalf("python symbol = %#v", pythonSymbol)
	}

	nodeSymbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "Run", Path: "pkg/web"})
	if err != nil {
		t.Fatal(err)
	}
	if nodeSymbol.Path != "pkg/web/index.ts" || nodeSymbol.Language != "nodejs" {
		t.Fatalf("node symbol = %#v", nodeSymbol)
	}
}

func TestLocalEngineGetSymbolUsesDeterministicRepoWideOrdering(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "a.py", "def Shared():\n    return 1\n")
	writeRetrievalFile(t, repoRoot, "b.ts", "export function Shared() {\n  return 1\n}\n")

	engine := NewLocalEngine()
	symbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "Shared"})
	if err != nil {
		t.Fatal(err)
	}
	if symbol.Path != "a.py" || symbol.Language != "python" {
		t.Fatalf("symbol = %#v", symbol)
	}
}

func TestLocalEngineFindPythonCallHierarchy(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "helpers.py", `def imported():
    return 1
`)
	writeRetrievalFile(t, repoRoot, "service.py", `from helpers import imported

def helper():
    return 1

class Service:
    def run(self):
        helper()
        self.other()
        imported()

    def other(self):
        return 2
`)

	engine := NewLocalEngine()
	callees, err := engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "run", Path: "service.py"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(callees.Root.Children); got != 3 {
		t.Fatalf("callee child count = %d", got)
	}
	joined := renderNames(callees.Root.Children)
	for _, want := range []string{"helper@service.py", "imported@helpers.py", "other@service.py"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("callees = %s", joined)
		}
	}

	callers, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "imported", Path: "helpers.py"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "run" || callers.Root.Children[0].CodeLocation.FilePath != "service.py" {
		t.Fatalf("callers = %#v", callers.Root.Children)
	}
}

func TestLocalEngineFindNodeCallHierarchyAcrossESMAndCommonJS(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "lib/util.js", `export function helper() {
  return 1
}
`)
	writeRetrievalFile(t, repoRoot, "lib/runner.js", `import { helper } from "./util.js"

export function run() {
  helper()
}
`)
	writeRetrievalFile(t, repoRoot, "lib/cjs.cjs", `function helper2() {
  return 1
}

module.exports = { helper2 }
`)
	writeRetrievalFile(t, repoRoot, "lib/main.ts", `const cjs = require("./cjs.cjs")

export const start = () => {
  cjs.helper2()
}
`)

	engine := NewLocalEngine()

	esm, err := engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "run", Path: "lib/runner.js"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(esm.Root.Children) != 1 || esm.Root.Children[0].Name != "helper" || esm.Root.Children[0].CodeLocation.FilePath != "lib/util.js" {
		t.Fatalf("esm callees = %#v", esm.Root.Children)
	}

	cjs, err := engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "start", Path: "lib/main.ts"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cjs.Root.Children) != 1 || cjs.Root.Children[0].Name != "helper2" || cjs.Root.Children[0].CodeLocation.FilePath != "lib/cjs.cjs" {
		t.Fatalf("cjs callees = %#v", cjs.Root.Children)
	}
}

func TestLocalEngineReturnsLowConfidenceErrorsForDynamicPythonAndNodeCalls(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "dynamic.py", `def run():
    getattr(factory, name)()
`)
	writeRetrievalFile(t, repoRoot, "dynamic.ts", `export function start() {
  factory()()
}
`)

	engine := NewLocalEngine()

	_, err := engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "run", Path: "dynamic.py"}, 2)
	if err == nil || !strings.Contains(err.Error(), "could not be resolved confidently for python") {
		t.Fatalf("python error = %v", err)
	}

	_, err = engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "start", Path: "dynamic.ts"}, 2)
	if err == nil || !strings.Contains(err.Error(), "could not be resolved confidently for nodejs") {
		t.Fatalf("node error = %v", err)
	}
}

func TestLocalEnginePathScopeSelectsIntendedBackend(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "pkg/run.py", `def Run():
    return 1
`)
	writeRetrievalFile(t, repoRoot, "pkg/run.ts", `export function Run() {
  return 1
}
`)

	engine := NewLocalEngine()
	pythonSymbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "Run", Path: "pkg/run.py"})
	if err != nil {
		t.Fatal(err)
	}
	if pythonSymbol.Language != "python" {
		t.Fatalf("python symbol = %#v", pythonSymbol)
	}
	nodeSymbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "Run", Path: "pkg/run.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if nodeSymbol.Language != "nodejs" {
		t.Fatalf("node symbol = %#v", nodeSymbol)
	}
}

func TestLocalEngineSymbolAndCallHierarchyRejectPathsOutsideRepo(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "pkg/run.py", "def Run():\n    return 1\n")

	engine := NewLocalEngine()
	if _, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "Run", Path: "../pkg/run.py"}); err == nil {
		t.Fatal("expected GetSymbol error")
	}
	if _, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "Run", Path: "../pkg/run.py"}, 1); err == nil {
		t.Fatal("expected FindCallers error")
	}
	if _, err := engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "Run", Path: "../pkg/run.py"}, 1); err == nil {
		t.Fatal("expected FindCallees error")
	}
}

func TestLocalEngineGetSymbolSkipsIgnoredDirectoriesDuringRepoWideSearch(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, ".gitignore", "ignored/\n")
	writeRetrievalFile(t, repoRoot, "pkg/run.py", "def Run():\n    return 1\n")
	writeRetrievalFile(t, repoRoot, "ignored/run.py", "def Run():\n    return 2\n")

	engine := NewLocalEngine()
	symbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "Run"})
	if err != nil {
		t.Fatal(err)
	}
	if symbol.Path != "pkg/run.py" {
		t.Fatalf("symbol = %#v", symbol)
	}
}

func TestLocalEngineFindTSXComponentCallHierarchy(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "src/util.ts", `export function helper(it: string): string {
  return it.trim()
}
`)
	writeRetrievalFile(t, repoRoot, "src/App.tsx", `import { helper } from "./util"

export function App({ items }: { items: string[] }) {
  return (
    <ul>
      {items.map((it) => (
        <li key={it}>{helper(it)}</li>
      ))}
    </ul>
  )
}
`)

	engine := NewLocalEngine()
	callers, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "helper", Path: "src/util.ts"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "App" || callers.Root.Children[0].CodeLocation.FilePath != "src/App.tsx" {
		t.Fatalf("tsx callers = %#v", callers.Root.Children)
	}

	symbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "App", Path: "src/App.tsx"})
	if err != nil {
		t.Fatal(err)
	}
	if symbol.Language != "nodejs" || !strings.Contains(symbol.Source, "<ul>") {
		t.Fatalf("tsx symbol = %#v", symbol)
	}
}

func TestLocalEngineFindsMultiLineTypedSignatureCallers(t *testing.T) {
	// Multi-line signatures and typed arrow functions broke the previous
	// line-regex extraction; the AST parser must resolve them.
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "svc/format.ts", `export const format = (
  value: string,
  options: { upper?: boolean } = {},
): string => value.trim()
`)
	writeRetrievalFile(t, repoRoot, "svc/run.ts", `import { format } from "./format"

export async function run(
  input: string,
): Promise<string> {
  return format(input)
}
`)

	engine := NewLocalEngine()
	callers, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "format", Path: "svc/format.ts"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "run" {
		t.Fatalf("callers = %#v", callers.Root.Children)
	}
	symbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "run", Path: "svc/run.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if symbol.StartLine != 3 || symbol.EndLine != 7 {
		t.Fatalf("run range = %d-%d, want 3-7", symbol.StartLine, symbol.EndLine)
	}
}

func TestLocalEngineFindDecoratedPythonMethodCallers(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "app.py", `class Service:
    @property
    def name(self):
        return self.compose()

    @staticmethod
    def compose():
        return "x"
`)

	engine := NewLocalEngine()
	callers, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "compose", Path: "app.py"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "name" {
		t.Fatalf("callers = %#v", callers.Root.Children)
	}
	symbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "compose", Path: "app.py"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(symbol.Source, "@staticmethod") {
		t.Fatalf("decorator missing from source: %q", symbol.Source)
	}
}

func TestLocalEngineOptionalChainingStaysLowConfidence(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "opt.ts", `export function start(client?: { run: () => void }) {
  client?.run()
}
`)

	engine := NewLocalEngine()
	_, err := engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "start", Path: "opt.ts"}, 1)
	if err == nil || !strings.Contains(err.Error(), "could not be resolved confidently for nodejs") {
		t.Fatalf("error = %v", err)
	}
}

func TestLocalEngineFindRustCallHierarchy(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "src/lib.rs", `pub fn format_name(n: &str) -> String
where
    String: Clone,
{
    n.to_string()
}
`)
	writeRetrievalFile(t, repoRoot, "src/main.rs", `struct Greeter;

impl Greeter {
    fn greet(&self, n: &str) -> String {
        crate::format_name(n)
    }
}
`)

	engine := NewLocalEngine()
	callers, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "format_name", Path: "src/lib.rs"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "greet" || callers.Root.Children[0].CodeLocation.FilePath != "src/main.rs" {
		t.Fatalf("rust callers = %#v", callers.Root.Children)
	}
	symbol, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "format_name", Path: "src/lib.rs"})
	if err != nil {
		t.Fatal(err)
	}
	if symbol.StartLine != 1 || symbol.EndLine != 6 {
		t.Fatalf("format_name range = %d-%d, want 1-6 (where clause)", symbol.StartLine, symbol.EndLine)
	}
}

func writeRetrievalFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func renderNames(nodes []CallNode) string {
	parts := make([]string, 0, len(nodes))
	for _, node := range nodes {
		parts = append(parts, node.Name+"@"+node.CodeLocation.FilePath)
	}
	return strings.Join(parts, ",")
}
