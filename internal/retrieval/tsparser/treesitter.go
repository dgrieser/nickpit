package tsparser

import (
	sitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// tsParse parses src with the grammar registered for the canonical filename
// (grammar lookup is extension-based; callers pass "x.py"/"x.rs"). It uses the
// library's per-language parser pool, which is safe for concurrent use.
// The caller must Release() the returned tree.
func tsParse(canonicalName string, src []byte) (*sitter.BoundTree, error) {
	return grammars.ParseFilePooled(canonicalName, src)
}

// namedChildren returns the named children of n.
func namedChildren(n *sitter.Node) []*sitter.Node {
	count := n.NamedChildCount()
	out := make([]*sitter.Node, 0, count)
	for i := range count {
		out = append(out, n.NamedChild(i))
	}
	return out
}

// field returns the child of n for the named grammar field, or nil. Unlike
// the BoundTree accessors, Node.ChildByFieldName is not nil-safe, so guard
// here once for every call site.
func field(bt *sitter.BoundTree, n *sitter.Node, name string) *sitter.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName(name, bt.Language())
}

// subtreeHasError reports whether n's subtree contains an ERROR or MISSING
// node. Node.HasError alone is not enough: the runtime does not propagate
// MISSING (recovered) tokens into the ancestor error flag.
func subtreeHasError(n *sitter.Node) bool {
	if n == nil {
		return false
	}
	found := false
	sitter.Walk(n, func(node *sitter.Node, _ int) sitter.WalkAction {
		if node.IsError() || node.IsMissing() {
			found = true
			return sitter.WalkStop
		}
		return sitter.WalkContinue
	})
	return found
}
