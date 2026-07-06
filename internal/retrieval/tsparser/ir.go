// Package tsparser turns source files into a language-neutral intermediate
// representation (symbols, calls, imports, exports) using real parsers:
// esbuild's parser (github.com/ije/esbuild-internal) for the JavaScript/
// TypeScript family and a pure-Go tree-sitter runtime
// (github.com/odvcencio/gotreesitter) for Python and Rust. Both are CGo-free,
// so the single-static-binary build is preserved. All third-party parser API
// usage is confined to this package; the retrieval backends consume only the
// IR defined here.
package tsparser

// CallKind classifies a call site by how confidently its target name can be
// resolved by a name/import-based resolver.
type CallKind int

const (
	// CallBare is a plain `name(...)` call (including calls whose target is a
	// local alias of an imported symbol, and Rust path calls resolved by their
	// trailing segment).
	CallBare CallKind = iota
	// CallMember is a `base.name(...)` call on a plain identifier base other
	// than this/self/cls.
	CallMember
	// CallSelf is a `this.name(...)` / `self.name(...)` / `cls.name(...)` call.
	CallSelf
	// CallDynamic is a call whose target cannot be resolved statically
	// (optional chaining, computed member, call-of-call, getattr, ...). Its
	// presence makes the enclosing symbol's hierarchy low-confidence.
	CallDynamic
)

// Call is one call site inside a symbol body.
type Call struct {
	// Name is the trailing identifier being invoked ("" for CallDynamic).
	Name string
	// Base is the receiver identifier for CallMember ("this"/"self"/"cls"
	// collapse into CallSelf instead), "" otherwise.
	Base string
	Kind CallKind
}

// Import is one imported binding: Alias is the local name it is bound to.
type Import struct {
	Alias string
	// SymbolName is the name in the source module for per-symbol imports
	// ("" when the whole module is bound, e.g. `import * as m`, `import os`).
	SymbolName string
	// ModuleSpec is the raw module specifier as written ("./util.js",
	// "pkg.helpers", "..mod"); resolving it to a repository path is the
	// backend's job.
	ModuleSpec string
	// Kind is "module" or "symbol".
	Kind string
}

// Export maps an exported name to the local top-level symbol name it refers to.
type Export struct {
	ExportedName string
	LocalName    string
}

// Symbol is one function-like definition.
type Symbol struct {
	Name string
	// Container is the enclosing class/impl/trait name for methods, "" for
	// free functions.
	Container string
	// StartLine/EndLine are 1-based and inclusive.
	StartLine int
	EndLine   int
	// Source is the symbol's source text including decorators/attributes.
	Source string
	// Exported reports an explicit export marker (JS/TS only).
	Exported bool
	// Nested reports a definition inside another function; nested symbols are
	// not addressable from other files (kept out of top-level/export tables).
	Nested bool
	// Calls are the call sites inside the symbol body, innermost-attributed:
	// calls inside a nested named function belong to that nested symbol.
	Calls []Call
	// Dynamic reports that the body contains a dynamic-call construct; the
	// backend marks the symbol low-confidence.
	Dynamic bool
	// HasError reports that the symbol's subtree contained parse errors.
	HasError bool
}

// FileIR is the parse result for one file.
type FileIR struct {
	// Path is the repo-relative, slash-normalized path.
	Path    string
	Symbols []*Symbol
	Imports []Import
	Exports []Export
	// HasError reports that the file contained parse errors anywhere.
	HasError bool
}
