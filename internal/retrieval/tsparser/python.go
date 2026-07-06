package tsparser

import (
	"strings"

	sitter "github.com/odvcencio/gotreesitter"
)

// parsePython extracts the IR from a Python file using the tree-sitter grammar.
func parsePython(path string, src []byte) (*FileIR, error) {
	ir := &FileIR{Path: path}
	bt, err := tsParse("x.py", src)
	if err != nil {
		// The runtime failed outright (tree-sitter itself is error-tolerant);
		// degrade to "no structural information" rather than failing the file.
		ir.HasError = true
		return ir, nil
	}
	defer bt.Release()

	root := bt.RootNode()
	if root == nil {
		return ir, nil
	}
	w := &pyWalker{ir: ir, bt: bt, ix: newLineIndex(src)}
	if subtreeHasError(root) {
		ir.HasError = true
	}
	w.walkChildren(root, "", nil)
	return ir, nil
}

type pyWalker struct {
	ir *FileIR
	bt *sitter.BoundTree
	ix *lineIndex
}

func (w *pyWalker) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return w.bt.NodeText(n)
}

func (w *pyWalker) walkChildren(n *sitter.Node, class string, cur *Symbol) {
	for _, child := range namedChildren(n) {
		w.walk(child, class, cur)
	}
}

// walk dispatches on node type. class is the enclosing class name for direct
// members; cur is the innermost enclosing function symbol.
func (w *pyWalker) walk(n *sitter.Node, class string, cur *Symbol) {
	switch w.bt.NodeType(n) {
	case "class_definition":
		w.class(n, cur, n)
	case "decorated_definition":
		def := field(w.bt, n, "definition")
		switch w.bt.NodeType(def) {
		case "function_definition":
			w.function(def, n, class, cur)
		case "class_definition":
			w.class(def, cur, n)
		default:
			w.walkChildren(n, class, cur)
		}
	case "function_definition":
		w.function(n, n, class, cur)
	case "import_statement":
		if cur == nil && class == "" {
			w.importStmt(n)
		}
	case "import_from_statement":
		if cur == nil && class == "" {
			w.fromImportStmt(n)
		}
	case "call":
		w.call(n, class, cur)
	default:
		w.walkChildren(n, class, cur)
	}
}

// class indexes the direct methods of a class body; span is the node whose
// range covers the whole definition (the decorated_definition for decorated
// classes).
func (w *pyWalker) class(n *sitter.Node, cur *Symbol, span *sitter.Node) {
	_ = span
	name := w.text(field(w.bt, n, "name"))
	body := field(w.bt, n, "body")
	if body == nil {
		return
	}
	w.walkChildren(body, name, cur)
}

// function registers a function/method symbol. span covers decorators when the
// definition is decorated.
func (w *pyWalker) function(n, span *sitter.Node, class string, cur *Symbol) {
	name := w.text(field(w.bt, n, "name"))
	if name == "" {
		return
	}
	startLine := int(span.StartPoint().Row) + 1
	endLine := int(span.EndPoint().Row) + 1
	symbol := &Symbol{
		Name:      name,
		Container: class,
		StartLine: startLine,
		EndLine:   endLine,
		Source:    w.ix.slice(startLine, endLine),
		Nested:    cur != nil,
		HasError:  subtreeHasError(span),
	}
	w.ir.Symbols = append(w.ir.Symbols, symbol)
	if body := field(w.bt, n, "body"); body != nil {
		w.walkChildren(body, class, symbol)
	}
}

// importStmt handles `import a.b, c as d`.
func (w *pyWalker) importStmt(n *sitter.Node) {
	for _, child := range namedChildren(n) {
		switch w.bt.NodeType(child) {
		case "dotted_name":
			module := w.text(child)
			pieces := strings.Split(module, ".")
			w.ir.Imports = append(w.ir.Imports, Import{
				Alias:      pieces[len(pieces)-1],
				ModuleSpec: module,
				Kind:       "module",
			})
		case "aliased_import":
			module := w.text(field(w.bt, child, "name"))
			alias := w.text(field(w.bt, child, "alias"))
			if module == "" || alias == "" {
				continue
			}
			w.ir.Imports = append(w.ir.Imports, Import{
				Alias:      alias,
				ModuleSpec: module,
				Kind:       "module",
			})
		}
	}
}

// fromImportStmt handles `from .mod import a, b as c` (including parenthesized
// multi-line name lists, which the grammar flattens for us).
func (w *pyWalker) fromImportStmt(n *sitter.Node) {
	module := field(w.bt, n, "module_name")
	if module == nil {
		return
	}
	spec := w.text(module)
	sawModule := false
	for _, child := range namedChildren(n) {
		if child == module && !sawModule {
			sawModule = true
			continue
		}
		switch w.bt.NodeType(child) {
		case "dotted_name":
			name := w.text(child)
			w.ir.Imports = append(w.ir.Imports, Import{
				Alias:      name,
				SymbolName: name,
				ModuleSpec: spec,
				Kind:       "symbol",
			})
		case "aliased_import":
			name := w.text(field(w.bt, child, "name"))
			alias := w.text(field(w.bt, child, "alias"))
			if name == "" || alias == "" {
				continue
			}
			w.ir.Imports = append(w.ir.Imports, Import{
				Alias:      alias,
				SymbolName: name,
				ModuleSpec: spec,
				Kind:       "symbol",
			})
		}
	}
}

// call classifies one call site, mirroring the previous regex semantics:
// getattr / call-of-call / subscript calls are dynamic; member calls resolve
// by their trailing `base.name` identifier pair; anything else is skipped.
func (w *pyWalker) call(n *sitter.Node, class string, cur *Symbol) {
	defer w.walkChildren(n, class, cur)
	if cur == nil {
		return
	}
	fn := field(w.bt, n, "function")
	if fn == nil {
		return
	}
	switch w.bt.NodeType(fn) {
	case "identifier":
		name := w.text(fn)
		if name == "getattr" {
			cur.Dynamic = true
			return
		}
		cur.Calls = append(cur.Calls, Call{Name: name, Kind: CallBare})
	case "attribute":
		object := field(w.bt, fn, "object")
		name := w.text(field(w.bt, fn, "attribute"))
		if object == nil || name == "" {
			return
		}
		var base string
		switch w.bt.NodeType(object) {
		case "identifier":
			base = w.text(object)
		case "attribute":
			base = w.text(field(w.bt, object, "attribute"))
		default:
			return
		}
		if base == "self" || base == "cls" {
			cur.Calls = append(cur.Calls, Call{Name: name, Base: base, Kind: CallSelf})
			return
		}
		cur.Calls = append(cur.Calls, Call{Name: name, Base: base, Kind: CallMember})
	case "call":
		// Immediately-invoked result: f(...)(...).
		cur.Dynamic = true
	case "subscript":
		// Computed target: handlers[name](...).
		cur.Dynamic = true
	}
}
