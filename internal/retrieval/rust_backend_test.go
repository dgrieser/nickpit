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
	if !strings.Contains(parseCallerNames, "is_loopback") || !strings.Contains(parseCallerNames, "same_origin") {
		t.Fatalf("parse callers = %q", parseCallerNames)
	}
}
