package tsparser

import (
	"strings"

	sitter "github.com/odvcencio/gotreesitter"
)

// rustSkippedCalls are enum-variant constructors that read as calls; edges to
// same-named functions would be noise (parity with the regex backend).
var rustSkippedCalls = map[string]struct{}{
	"Some": {}, "None": {}, "Ok": {}, "Err": {},
}

// parseRust extracts the IR from a Rust file using the tree-sitter grammar.
func parseRust(path string, src []byte) (*FileIR, error) {
	ir := &FileIR{Path: path}
	bt, err := tsParse("x.rs", src)
	if err != nil || bt == nil {
		ir.HasError = true
		return ir, nil
	}
	defer bt.Release()

	root := bt.RootNode()
	if root == nil {
		return ir, nil
	}
	w := &rustWalker{ir: ir, bt: bt, ix: newLineIndex(src)}
	if subtreeHasError(root) {
		ir.HasError = true
	}
	w.walkChildren(root, "", nil)
	return ir, nil
}

type rustWalker struct {
	ir *FileIR
	bt *sitter.BoundTree
	ix *lineIndex
}

func (w *rustWalker) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return w.bt.NodeText(n)
}

func (w *rustWalker) walkChildren(n *sitter.Node, container string, cur *Symbol) {
	for _, child := range namedChildren(n) {
		w.walk(child, container, cur)
	}
}

func (w *rustWalker) walk(n *sitter.Node, container string, cur *Symbol) {
	switch w.bt.NodeType(n) {
	case "impl_item":
		w.walkChildren(n, w.text(field(w.bt, n, "type")), cur)
	case "trait_item":
		w.walkChildren(n, w.text(field(w.bt, n, "name")), cur)
	case "function_item", "function_signature_item":
		w.function(n, container, cur)
	case "macro_invocation":
		w.macroTokens(n, cur)
	case "call_expression":
		w.call(n, container, cur)
	default:
		w.walkChildren(n, container, cur)
	}
}

func (w *rustWalker) function(n *sitter.Node, container string, cur *Symbol) {
	name := w.text(field(w.bt, n, "name"))
	if name == "" {
		return
	}
	startLine := int(n.StartPoint().Row) + 1
	endLine := int(n.EndPoint().Row) + 1
	symbol := &Symbol{
		Name:      name,
		Container: container,
		StartLine: startLine,
		EndLine:   endLine,
		Source:    w.ix.slice(startLine, endLine),
		Nested:    cur != nil,
		HasError:  subtreeHasError(n),
	}
	w.ir.Symbols = append(w.ir.Symbols, symbol)
	if body := field(w.bt, n, "body"); body != nil {
		w.walkChildren(body, container, symbol)
	}
}

// call classifies one call site. All resolvable Rust calls collapse to a bare
// trailing name (parity with the regex backend, which resolved `self.foo(...)`
// and `Type::foo(...)` by their trailing identifier alike).
func (w *rustWalker) call(n *sitter.Node, container string, cur *Symbol) {
	defer w.walkChildren(n, container, cur)
	if cur == nil {
		return
	}
	fn := field(w.bt, n, "function")
	if fn == nil {
		return
	}
	if name, ok := w.callTargetName(fn); ok {
		if _, skipped := rustSkippedCalls[name]; skipped {
			return
		}
		cur.Calls = append(cur.Calls, Call{Name: name, Kind: CallBare})
		return
	}
	switch w.bt.NodeType(fn) {
	case "call_expression", "index_expression":
		// Immediately-invoked closure / indexed call: f(...)(...), fns[i](...).
		cur.Dynamic = true
	}
}

// callTargetName extracts the trailing identifier from a call target.
func (w *rustWalker) callTargetName(fn *sitter.Node) (string, bool) {
	switch w.bt.NodeType(fn) {
	case "identifier":
		return w.text(fn), true
	case "scoped_identifier":
		name := w.text(field(w.bt, fn, "name"))
		return name, name != ""
	case "generic_function":
		inner := field(w.bt, fn, "function")
		if inner == nil {
			return "", false
		}
		return w.callTargetName(inner)
	case "field_expression":
		name := w.text(field(w.bt, fn, "field"))
		return name, name != ""
	}
	return "", false
}

// macroTokens scans a macro invocation's token tree for `name(...)` shapes so
// calls inside println!/format!/assert! arguments keep producing edges (the
// line-based extraction saw them; the grammar leaves macro bodies unparsed).
func (w *rustWalker) macroTokens(n *sitter.Node, cur *Symbol) {
	if cur == nil {
		return
	}
	var scan func(node *sitter.Node)
	scan = func(node *sitter.Node) {
		children := namedChildren(node)
		for i, child := range children {
			if w.bt.NodeType(child) == "token_tree" {
				scan(child)
				continue
			}
			if w.bt.NodeType(child) != "identifier" || i+1 >= len(children) {
				continue
			}
			next := children[i+1]
			if w.bt.NodeType(next) == "token_tree" && strings.HasPrefix(w.text(next), "(") {
				name := w.text(child)
				if _, skipped := rustSkippedCalls[name]; !skipped {
					cur.Calls = append(cur.Calls, Call{Name: name, Kind: CallBare})
				}
			}
		}
	}
	scan(n)
}
