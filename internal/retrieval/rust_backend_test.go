package retrieval

import (
	"context"
	"strings"
	"testing"
)

// rustFixture exercises free functions, an impl-block method, and intra-file
// calls — the shape of the relatum oidc.rs change that originally tripped the
// reviewer when Rust had no structural backend.
const rustFixture = `pub fn redirect_allowed(url: &str) -> bool {
    is_loopback(url) || same_origin(url)
}

fn is_loopback(url: &str) -> bool {
    parse(url)
}

fn same_origin(url: &str) -> bool {
    parse(url)
}

fn parse(_url: &str) -> bool {
    true
}

fn fetch_redirect(url: &str) -> bool {
    let _endpoint = "https://idp.example.com//callback"; parse(url)
}

struct Flow;

impl Flow {
    pub fn begin(&self, redirect: &str) -> bool {
        redirect_allowed(redirect)
    }
}
`

func TestRustBackendGetSymbol(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "src/sso.rs", rustFixture)
	engine := NewLocalEngine()

	sym, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "redirect_allowed", Path: "src/sso.rs"})
	if err != nil {
		t.Fatal(err)
	}
	if sym.Language != "rust" || sym.Path != "src/sso.rs" {
		t.Fatalf("symbol = %#v", sym)
	}
	if !strings.Contains(sym.Source, "is_loopback(url) || same_origin(url)") {
		t.Fatalf("symbol source missing body: %q", sym.Source)
	}

	// A method defined inside an impl block is also indexed.
	method, err := engine.GetSymbol(context.Background(), repoRoot, SymbolRef{Name: "begin", Path: "src/sso.rs"})
	if err != nil {
		t.Fatal(err)
	}
	if method.Name != "begin" {
		t.Fatalf("method symbol = %#v", method)
	}
}

func TestRustBackendCallHierarchy(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "src/sso.rs", rustFixture)
	engine := NewLocalEngine()

	callees, err := engine.FindCallees(context.Background(), repoRoot, SymbolRef{Name: "redirect_allowed", Path: "src/sso.rs"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	calleeNames := renderNames(callees.Root.Children)
	if !strings.Contains(calleeNames, "is_loopback") || !strings.Contains(calleeNames, "same_origin") {
		t.Fatalf("redirect_allowed callees = %q", calleeNames)
	}

	callers, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "redirect_allowed", Path: "src/sso.rs"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	callerNames := renderNames(callers.Root.Children)
	if !strings.Contains(callerNames, "begin") {
		t.Fatalf("redirect_allowed callers = %q", callerNames)
	}

	// parse is reached from both loopback/origin helpers.
	parseCallers, err := engine.FindCallers(context.Background(), repoRoot, SymbolRef{Name: "parse", Path: "src/sso.rs"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	parseCallerNames := renderNames(parseCallers.Root.Children)
	// fetch_redirect calls parse(url) on a line whose string literal contains
	// "//" (a URL). The old strings.Cut(line, "//") truncated the line before
	// the call, dropping this edge; stripRustLine must now preserve it.
	if !strings.Contains(parseCallerNames, "is_loopback") ||
		!strings.Contains(parseCallerNames, "same_origin") ||
		!strings.Contains(parseCallerNames, "fetch_redirect") {
		t.Fatalf("parse callers = %q", parseCallerNames)
	}
}

func TestStripRustLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no comment", "foo()", "foo()"},
		{"comment only", "// c", ""},
		// The trailing space before // is intentionally preserved; the call
		// regexes tolerate surrounding whitespace.
		{"trailing comment", "foo() // c", "foo() "},
		{"url in string", `let u = "http://x";`, `let u = "http://x";`},
		{"string then comment", `"http://x" foo() // c`, `"http://x" foo() `},
		{"escaped quote in string", `"a\"b" // c`, `"a\"b" `},
		{"escaped backslash then close", `"a\\" // c`, `"a\\" `},
		{"comment before later quote", `foo() // "bar`, "foo() "},
		// '"' is a char literal, not the start of a string; the guard prevents a
		// phantom string from swallowing the real trailing comment.
		{"quote char literal", `let q = '"'; // c`, `let q = '"'; `},
		{"lifetime not char literal", `fn f<'a>(x: &'a str) {} // c`, `fn f<'a>(x: &'a str) {} `},
		{"unterminated string", `let s = "start`, `let s = "start`},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripRustLine(tc.in); got != tc.want {
				t.Fatalf("stripRustLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
