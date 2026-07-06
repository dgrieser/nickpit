package tsparser

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, path, src string) *FileIR {
	t.Helper()
	ir, err := ParseFile(path, []byte(src))
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return ir
}

func findSymbol(t *testing.T, ir *FileIR, name string) *Symbol {
	t.Helper()
	for _, symbol := range ir.Symbols {
		if symbol.Name == name {
			return symbol
		}
	}
	t.Fatalf("symbol %q not found; have %v", name, symbolNames(ir))
	return nil
}

func symbolNames(ir *FileIR) []string {
	out := make([]string, 0, len(ir.Symbols))
	for _, symbol := range ir.Symbols {
		out = append(out, symbol.Name)
	}
	return out
}

func callNames(symbol *Symbol) []string {
	out := make([]string, 0, len(symbol.Calls))
	for _, call := range symbol.Calls {
		out = append(out, call.Name)
	}
	return out
}

func hasCall(symbol *Symbol, name string, kind CallKind) bool {
	for _, call := range symbol.Calls {
		if call.Name == name && call.Kind == kind {
			return true
		}
	}
	return false
}

func TestUnsupportedExtension(t *testing.T) {
	if _, err := ParseFile("a.txt", []byte("hello")); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

// --- JavaScript / TypeScript ---

func TestTSTypedArrowAndMultilineSignature(t *testing.T) {
	ir := mustParse(t, "a.ts", `
const format = (n: string): string => n.trim();

export async function greet(
  name: string,
  opts: { loud?: boolean } = {},
): Promise<string> {
  return format(name);
}
`)
	if ir.HasError {
		t.Fatal("unexpected parse error")
	}
	format := findSymbol(t, ir, "format")
	if format.StartLine != 2 || format.Nested {
		t.Fatalf("format: start=%d nested=%v", format.StartLine, format.Nested)
	}
	greet := findSymbol(t, ir, "greet")
	if greet.StartLine != 4 || greet.EndLine != 9 {
		t.Fatalf("greet range = %d-%d, want 4-9", greet.StartLine, greet.EndLine)
	}
	if !greet.Exported {
		t.Fatal("greet should be exported")
	}
	if !hasCall(greet, "format", CallBare) {
		t.Fatalf("greet calls = %v, want format", callNames(greet))
	}
	if !strings.Contains(greet.Source, "opts: { loud?: boolean }") {
		t.Fatalf("greet source truncated: %q", greet.Source)
	}
}

func TestTSClassMethodsAndDecorators(t *testing.T) {
	ir := mustParse(t, "a.ts", `
export class Greeter {
  private prefix = "hi";
  handler = (x: string) => this.compose(x);
  constructor(private n: number) {}
  static make(): Greeter {
    return new Greeter(1);
  }
  @log()
  greet(name: string) {
    return this.compose(name);
  }
  compose(n: string) {
    return format(n);
  }
}
const format = (n: string): string => n;
`)
	greet := findSymbol(t, ir, "greet")
	if greet.Container != "Greeter" {
		t.Fatalf("greet container = %q", greet.Container)
	}
	if greet.StartLine != 9 {
		t.Fatalf("greet start = %d, want 9 (decorator included)", greet.StartLine)
	}
	if !strings.Contains(greet.Source, "@log()") {
		t.Fatalf("greet source missing decorator: %q", greet.Source)
	}
	if !hasCall(greet, "compose", CallSelf) {
		t.Fatalf("greet calls = %v", greet.Calls)
	}
	handler := findSymbol(t, ir, "handler")
	if handler.Container != "Greeter" || !hasCall(handler, "compose", CallSelf) {
		t.Fatalf("handler = %+v", handler)
	}
	findSymbol(t, ir, "make")
	for _, symbol := range ir.Symbols {
		if symbol.Name == "constructor" {
			t.Fatal("constructor must not be indexed")
		}
	}
}

func TestJSImportsAndExports(t *testing.T) {
	ir := mustParse(t, "a.js", `
import { helper, other as renamed } from "./util.js";
import * as ns from "./ns.js";
import dflt from "./dflt.js";
const mod = require("./cjs.js");
const { one, two: alias } = require("./two.js");

export function first() {
  helper();
  renamed();
  ns.deep();
  mod.run();
  one();
  alias();
}
function second() {}
export { second as renamedExport };
`)
	wantImports := map[string]Import{
		"helper":  {Alias: "helper", SymbolName: "helper", ModuleSpec: "./util.js", Kind: "symbol"},
		"renamed": {Alias: "renamed", SymbolName: "other", ModuleSpec: "./util.js", Kind: "symbol"},
		"ns":      {Alias: "ns", ModuleSpec: "./ns.js", Kind: "module"},
		"dflt":    {Alias: "dflt", SymbolName: "default", ModuleSpec: "./dflt.js", Kind: "symbol"},
		"mod":     {Alias: "mod", ModuleSpec: "./cjs.js", Kind: "module"},
		"one":     {Alias: "one", SymbolName: "one", ModuleSpec: "./two.js", Kind: "symbol"},
		"alias":   {Alias: "alias", SymbolName: "two", ModuleSpec: "./two.js", Kind: "symbol"},
	}
	got := map[string]Import{}
	for _, imp := range ir.Imports {
		got[imp.Alias] = imp
	}
	for alias, want := range wantImports {
		if got[alias] != want {
			t.Fatalf("import %q = %+v, want %+v", alias, got[alias], want)
		}
	}
	first := findSymbol(t, ir, "first")
	for _, name := range []string{"helper", "renamed", "one", "alias"} {
		if !hasCall(first, name, CallBare) {
			t.Fatalf("first calls = %v, missing %s", first.Calls, name)
		}
	}
	if !hasCall(first, "deep", CallMember) || !hasCall(first, "run", CallMember) {
		t.Fatalf("first member calls = %+v", first.Calls)
	}
	exports := map[string]string{}
	for _, export := range ir.Exports {
		exports[export.ExportedName] = export.LocalName
	}
	if exports["first"] != "first" || exports["renamedExport"] != "second" {
		t.Fatalf("exports = %v", exports)
	}
}

func TestCJSExports(t *testing.T) {
	ir := mustParse(t, "a.cjs", `
function alpha() {}
function beta() {}
function gamma() {}
module.exports = { alpha, renamed: beta };
module.exports.direct = gamma;
exports.short = alpha;
`)
	exports := map[string]string{}
	for _, export := range ir.Exports {
		exports[export.ExportedName] = export.LocalName
	}
	want := map[string]string{"alpha": "alpha", "renamed": "beta", "direct": "gamma", "short": "alpha"}
	for name, local := range want {
		if exports[name] != local {
			t.Fatalf("exports[%q] = %q, want %q (all: %v)", name, exports[name], local, exports)
		}
	}
}

func TestJSNestedFunctionsAndDynamicCalls(t *testing.T) {
	ir := mustParse(t, "a.js", `
function outer() {
  function inner() {
    used();
  }
  const innerArrow = () => used();
  inner();
  maybe?.run();
  handlers[name]();
  factory()();
}
function used() {}
`)
	inner := findSymbol(t, ir, "inner")
	if !inner.Nested || !hasCall(inner, "used", CallBare) {
		t.Fatalf("inner = %+v", inner)
	}
	innerArrow := findSymbol(t, ir, "innerArrow")
	if !innerArrow.Nested {
		t.Fatal("innerArrow should be nested")
	}
	outer := findSymbol(t, ir, "outer")
	if !hasCall(outer, "inner", CallBare) {
		t.Fatalf("outer calls = %v", callNames(outer))
	}
	if hasCall(outer, "used", CallBare) {
		t.Fatal("used() belongs to inner, not outer")
	}
	if !outer.Dynamic {
		t.Fatal("outer should be dynamic (optional chain, computed, curried calls)")
	}
	if inner.Dynamic || innerArrow.Dynamic {
		t.Fatal("nested symbols should not inherit dynamic flags")
	}
}

func TestJSIIFEBodyIndexedAtTopLevel(t *testing.T) {
	// UMD-style bundles define everything inside an anonymous IIFE; those
	// definitions are still addressable by name.
	ir := mustParse(t, "a.js", `
(function (global) {
  function api() {
    helper();
  }
  function helper() {}
  global.api = api;
})(this);
`)
	api := findSymbol(t, ir, "api")
	if api.Nested {
		t.Fatal("api sits in an anonymous wrapper, not a named symbol; must not be nested")
	}
	if !hasCall(api, "helper", CallBare) {
		t.Fatalf("api calls = %v", callNames(api))
	}
}

func TestTSXComponent(t *testing.T) {
	ir := mustParse(t, "a.tsx", `
import { helper } from "./util";

export function App({ items }: { items: string[] }) {
  return (
    <ul>
      {items.map((it) => (
        <li key={it}>{helper(it)}</li>
      ))}
    </ul>
  );
}
`)
	if ir.HasError {
		t.Fatal("unexpected parse error")
	}
	app := findSymbol(t, ir, "App")
	if !app.Exported {
		t.Fatal("App should be exported")
	}
	if !hasCall(app, "helper", CallBare) {
		t.Fatalf("App calls = %v", app.Calls)
	}
	if !hasCall(app, "map", CallMember) {
		t.Fatalf("App calls = %+v, want member map call", app.Calls)
	}
}

func TestJSTemplateLiteralCalls(t *testing.T) {
	ir := mustParse(t, "a.js", "function fmt() { return `hi ${world(1)}`; }\nfunction world() {}\n")
	fmtSym := findSymbol(t, ir, "fmt")
	if !hasCall(fmtSym, "world", CallBare) {
		t.Fatalf("fmt calls = %v", callNames(fmtSym))
	}
}

func TestJSBrokenFile(t *testing.T) {
	ir := mustParse(t, "a.js", "function broken( {{{\n")
	if !ir.HasError {
		t.Fatal("expected HasError for broken file")
	}
}

func TestTSDefaultExport(t *testing.T) {
	ir := mustParse(t, "a.ts", `
export default function main() {
  return 1;
}
`)
	main := findSymbol(t, ir, "main")
	if !main.Exported {
		t.Fatal("main should be exported")
	}
	exports := map[string]string{}
	for _, export := range ir.Exports {
		exports[export.ExportedName] = export.LocalName
	}
	if exports["default"] != "main" || exports["main"] != "main" {
		t.Fatalf("exports = %v", exports)
	}
}

// --- Python ---

func TestPythonSymbolsAndCalls(t *testing.T) {
	ir := mustParse(t, "a.py", `
import os
import os.path as osp
from helpers import (
    format_name,
    trim as t,
)
from . import sibling
from ..pkg import thing

@decorator(arg=1)
def greet(name):
    def inner(x):
        return t(x)
    return format_name(inner(name))

class Greeter:
    async def greet(self, name):
        return self.compose(name)

    def compose(self, n):
        cfg = osp.join("a", "b")
        return format_name(n)

async def top(a,
              b):
    match a:
        case 1:
            return greet(b)
    return getattr(a, "x")()
`)
	if ir.HasError {
		t.Fatal("unexpected parse error")
	}
	greet := findSymbol(t, ir, "greet")
	if greet.StartLine != 11 {
		t.Fatalf("greet start = %d, want 11 (decorator included)", greet.StartLine)
	}
	if !strings.Contains(greet.Source, "@decorator(arg=1)") {
		t.Fatalf("greet source missing decorator: %q", greet.Source)
	}
	if !hasCall(greet, "format_name", CallBare) || !hasCall(greet, "inner", CallBare) {
		t.Fatalf("greet calls = %v", callNames(greet))
	}
	inner := findSymbol(t, ir, "inner")
	if !inner.Nested || !hasCall(inner, "t", CallBare) {
		t.Fatalf("inner = %+v", inner)
	}
	compose := findSymbol(t, ir, "compose")
	if compose.Container != "Greeter" {
		t.Fatalf("compose container = %q", compose.Container)
	}
	if !hasCall(compose, "join", CallMember) {
		t.Fatalf("compose calls = %+v", compose.Calls)
	}
	var methodGreet *Symbol
	for _, symbol := range ir.Symbols {
		if symbol.Name == "greet" && symbol.Container == "Greeter" {
			methodGreet = symbol
		}
	}
	if methodGreet == nil || !hasCall(methodGreet, "compose", CallSelf) {
		t.Fatalf("method greet = %+v", methodGreet)
	}
	top := findSymbol(t, ir, "top")
	if top.StartLine != 25 || top.EndLine != 30 {
		t.Fatalf("top range = %d-%d, want 25-30 (multi-line signature)", top.StartLine, top.EndLine)
	}
	if !hasCall(top, "greet", CallBare) {
		t.Fatalf("top calls = %v", callNames(top))
	}
	if !top.Dynamic {
		t.Fatal("top should be dynamic (getattr)")
	}

	imports := map[string]Import{}
	for _, imp := range ir.Imports {
		imports[imp.Alias] = imp
	}
	wantImports := map[string]Import{
		"os":          {Alias: "os", ModuleSpec: "os", Kind: "module"},
		"osp":         {Alias: "osp", ModuleSpec: "os.path", Kind: "module"},
		"format_name": {Alias: "format_name", SymbolName: "format_name", ModuleSpec: "helpers", Kind: "symbol"},
		"t":           {Alias: "t", SymbolName: "trim", ModuleSpec: "helpers", Kind: "symbol"},
		"sibling":     {Alias: "sibling", SymbolName: "sibling", ModuleSpec: ".", Kind: "symbol"},
		"thing":       {Alias: "thing", SymbolName: "thing", ModuleSpec: "..pkg", Kind: "symbol"},
	}
	for alias, want := range wantImports {
		if imports[alias] != want {
			t.Fatalf("import %q = %+v, want %+v", alias, imports[alias], want)
		}
	}
}

func TestPythonDynamicCalls(t *testing.T) {
	ir := mustParse(t, "a.py", `
def caller():
    factory()()
    handlers[name]()
`)
	caller := findSymbol(t, ir, "caller")
	if !caller.Dynamic {
		t.Fatal("caller should be dynamic")
	}
}

func TestPythonBrokenFile(t *testing.T) {
	ir := mustParse(t, "a.py", "def broken(:\n    pass\n")
	if !ir.HasError {
		t.Fatal("expected HasError")
	}
}

// --- Rust ---

func TestRustSymbolsAndCalls(t *testing.T) {
	ir := mustParse(t, "a.rs", `
pub fn greet(name: &str) -> String
where
    String: Clone,
{
    format_name::<String>(name)
}

struct Greeter;

impl Greeter {
    pub fn greet(&self, name: &str) -> String {
        self.compose(name)
    }
    fn compose(&self, n: &str) -> String {
        let raw = r#"weird { ( stuff"#;
        println!("{}", helper(n));
        format_name(n)
    }
}

trait Speak {
    fn speak(&self);
}

fn format_name(n: &str) -> String {
    n.to_string()
}

fn helper(n: &str) -> String {
    n.to_string()
}

fn dynamic_case() {
    let fns = [helper];
    fns[0]("x");
    (make_fn())("y");
}

fn make_fn() -> fn(&str) -> String {
    helper
}
`)
	if ir.HasError {
		t.Fatal("unexpected parse error")
	}
	greet := findSymbol(t, ir, "greet")
	if greet.StartLine != 2 || greet.EndLine != 7 {
		t.Fatalf("greet range = %d-%d, want 2-7 (where clause)", greet.StartLine, greet.EndLine)
	}
	if !hasCall(greet, "format_name", CallBare) {
		t.Fatalf("greet calls = %v (turbofish)", callNames(greet))
	}
	compose := findSymbol(t, ir, "compose")
	if compose.Container != "Greeter" {
		t.Fatalf("compose container = %q", compose.Container)
	}
	if !hasCall(compose, "format_name", CallBare) {
		t.Fatalf("compose calls = %v", callNames(compose))
	}
	if !hasCall(compose, "helper", CallBare) {
		t.Fatalf("compose calls = %v, want helper from println! args", callNames(compose))
	}
	speak := findSymbol(t, ir, "speak")
	if speak.Container != "Speak" {
		t.Fatalf("speak container = %q", speak.Container)
	}
	dynamicCase := findSymbol(t, ir, "dynamic_case")
	if !dynamicCase.Dynamic {
		t.Fatal("dynamic_case should be dynamic")
	}
	methodGreet := 0
	for _, symbol := range ir.Symbols {
		if symbol.Name == "greet" {
			methodGreet++
		}
	}
	if methodGreet != 2 {
		t.Fatalf("want 2 greet symbols (free + method), got %d", methodGreet)
	}
}

func TestRustScopedAndFieldCalls(t *testing.T) {
	ir := mustParse(t, "a.rs", `
fn caller(g: Greeter) {
    Greeter::compose(&g, "x");
    g.compose("y");
}
struct Greeter;
impl Greeter {
    fn compose(&self, n: &str) -> String { n.to_string() }
}
`)
	caller := findSymbol(t, ir, "caller")
	count := 0
	for _, call := range caller.Calls {
		if call.Name == "compose" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("caller compose calls = %d, want 2 (scoped + field)", count)
	}
}

func TestRustBrokenFile(t *testing.T) {
	ir := mustParse(t, "a.rs", "fn broken( {{{\n")
	if !ir.HasError {
		t.Fatal("expected HasError")
	}
}
