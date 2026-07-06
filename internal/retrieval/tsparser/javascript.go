package tsparser

import (
	"reflect"

	"github.com/ije/esbuild-internal/ast"
	"github.com/ije/esbuild-internal/config"
	"github.com/ije/esbuild-internal/helpers"
	"github.com/ije/esbuild-internal/js_ast"
	"github.com/ije/esbuild-internal/js_parser"
	"github.com/ije/esbuild-internal/logger"
)

// parseJS parses a JavaScript/TypeScript-family file with esbuild's parser.
// JSX stays in preserve mode so no synthetic factory calls (React.createElement)
// leak into call extraction.
func parseJS(lang Lang, path string, src []byte) (*FileIR, error) {
	opts := config.Options{}
	switch lang {
	case LangTS:
		opts.TS.Parse = true
	case LangTSX:
		opts.TS.Parse = true
		opts.JSX.Parse = true
		opts.JSX.Preserve = true
	case LangJS, LangJSX:
		// Plain expression-start "<" is a syntax error in JS, so enabling JSX
		// for .js only widens the accepted language (React codebases commonly
		// keep JSX in .js files).
		opts.JSX.Parse = true
		opts.JSX.Preserve = true
	}

	log := logger.NewDeferLog(logger.DeferLogAll, nil)
	source := logger.Source{
		Contents:       string(src),
		KeyPath:        logger.Path{Text: path},
		IdentifierName: "src",
	}
	tree, ok := js_parser.Parse(log, source, js_parser.OptionsFromConfig(&opts))

	ir := &FileIR{Path: path}
	for _, msg := range log.Done() {
		if msg.Kind == logger.Error {
			ir.HasError = true
		}
	}
	if !ok {
		ir.HasError = true
		return ir, nil
	}

	w := &jsWalker{ir: ir, tree: &tree, ix: newLineIndex(src)}
	for _, part := range tree.Parts {
		for _, stmt := range part.Stmts {
			w.topStmt(stmt)
		}
	}
	if ir.HasError {
		for _, symbol := range ir.Symbols {
			symbol.HasError = true
		}
	}
	return ir, nil
}

type jsWalker struct {
	ir   *FileIR
	tree *js_ast.AST
	ix   *lineIndex
	// stack holds the enclosing symbols; calls attribute to the innermost.
	stack []*Symbol
}

func (w *jsWalker) nameOf(ref ast.Ref) string {
	if int(ref.InnerIndex) >= len(w.tree.Symbols) {
		return ""
	}
	return w.tree.Symbols[ref.InnerIndex].OriginalName
}

func (w *jsWalker) current() *Symbol {
	if len(w.stack) == 0 {
		return nil
	}
	return w.stack[len(w.stack)-1]
}

// addSymbol registers a symbol spanning startLoc..endLoc (byte offsets).
func (w *jsWalker) addSymbol(name, container string, startLoc, endLoc int, exported bool) *Symbol {
	startLine := w.ix.lineOf(startLoc)
	endLine := w.ix.lineOf(endLoc)
	if endLine < startLine {
		endLine = startLine
	}
	symbol := &Symbol{
		Name:      name,
		Container: container,
		StartLine: startLine,
		EndLine:   endLine,
		Source:    w.ix.slice(startLine, endLine),
		Exported:  exported,
		Nested:    len(w.stack) > 0,
	}
	w.ir.Symbols = append(w.ir.Symbols, symbol)
	return symbol
}

// under walks fn with symbol as the innermost enclosing symbol.
func (w *jsWalker) under(symbol *Symbol, fn func()) {
	w.stack = append(w.stack, symbol)
	fn()
	w.stack = w.stack[:len(w.stack)-1]
}

// fnEndLoc returns the byte offset of a function body's closing brace, falling
// back to the maximum known location inside the body (expression-bodied arrows
// have no closing brace; a multi-line final token may truncate the range by a
// few lines, which only shortens the captured Source).
func (w *jsWalker) fnEndLoc(body js_ast.FnBody) int {
	if loc := int(body.Block.CloseBraceLoc.Start); loc > 0 {
		return loc
	}
	max := int(body.Loc.Start)
	for _, stmt := range body.Block.Stmts {
		if end := maxLocIn(stmt); end > max {
			max = end
		}
	}
	return max
}

// topStmt handles a top-level (or namespace-level) statement.
func (w *jsWalker) topStmt(stmt js_ast.Stmt) {
	switch s := stmt.Data.(type) {
	case *js_ast.SFunction:
		w.declareFn(nameOrEmpty(w, s.Fn.Name), stmt.Loc, s.Fn, s.IsExport)
	case *js_ast.SClass:
		w.declareClass(s.Class, stmt.Loc, s.IsExport)
	case *js_ast.SLocal:
		w.local(s)
	case *js_ast.SImport:
		w.importStmt(s)
	case *js_ast.SExportClause:
		for _, item := range s.Items {
			w.ir.Exports = append(w.ir.Exports, Export{
				ExportedName: item.Alias,
				LocalName:    w.nameOf(item.Name.Ref),
			})
		}
	case *js_ast.SExportDefault:
		w.exportDefault(s)
	case *js_ast.SNamespace:
		// TS namespace members are indexed like top-level symbols (parity with
		// the previous line-based extraction, which ignored nesting).
		for _, inner := range s.Stmts {
			w.topStmt(inner)
		}
	case *js_ast.SExpr:
		if w.cjsExport(s.Value) {
			return
		}
		w.walkExpr(s.Value)
	default:
		w.walkStmt(stmt)
	}
}

func nameOrEmpty(w *jsWalker, name *ast.LocRef) string {
	if name == nil {
		return ""
	}
	return w.nameOf(name.Ref)
}

// declareFn registers a named function declaration and walks its body.
func (w *jsWalker) declareFn(name string, loc logger.Loc, fn js_ast.Fn, exported bool) {
	if name == "" {
		// Anonymous: no symbol, but the body may contain named definitions.
		w.walkFnParts(fn.Args, fn.Body)
		return
	}
	symbol := w.addSymbol(name, "", int(loc.Start), w.fnEndLoc(fn.Body), exported)
	if exported {
		w.ir.Exports = append(w.ir.Exports, Export{ExportedName: name, LocalName: name})
	}
	w.under(symbol, func() { w.walkFnParts(fn.Args, fn.Body) })
}

// declareClass registers method symbols for a class declaration. The class
// itself is not a call-graph node.
func (w *jsWalker) declareClass(class js_ast.Class, loc logger.Loc, exported bool) {
	className := nameOrEmpty(w, class.Name)
	_ = loc
	_ = exported
	for _, property := range class.Properties {
		w.classProperty(className, property)
	}
}

func (w *jsWalker) classProperty(className string, property js_ast.Property) {
	if property.ClassStaticBlock != nil {
		for _, stmt := range property.ClassStaticBlock.Block.Stmts {
			w.walkStmt(stmt)
		}
		return
	}
	name, named := w.propertyName(property.Key)
	start := int(property.Loc.Start)
	for _, decorator := range property.Decorators {
		if int(decorator.AtLoc.Start) < start {
			start = int(decorator.AtLoc.Start)
		}
	}
	if fn, isFn := property.ValueOrNil.Data.(*js_ast.EFunction); isFn && property.Kind.IsMethodDefinition() {
		if !named || name == "constructor" || className == "" {
			w.walkFnParts(fn.Fn.Args, fn.Fn.Body)
			return
		}
		symbol := w.addSymbol(name, className, start, w.fnEndLoc(fn.Fn.Body), false)
		w.under(symbol, func() { w.walkFnParts(fn.Fn.Args, fn.Fn.Body) })
		return
	}
	if arrow, isArrow := property.InitializerOrNil.Data.(*js_ast.EArrow); isArrow && named && className != "" {
		// Class field holding an arrow function: `handler = () => {...}`.
		symbol := w.addSymbol(name, className, start, w.fnEndLoc(arrow.Body), false)
		w.under(symbol, func() { w.walkFnParts(arrow.Args, arrow.Body) })
		return
	}
	w.walkExpr(property.Key)
	w.walkExpr(property.ValueOrNil)
	w.walkExpr(property.InitializerOrNil)
}

func (w *jsWalker) propertyName(key js_ast.Expr) (string, bool) {
	switch k := key.Data.(type) {
	case *js_ast.EString:
		return helpers.UTF16ToString(k.Value), true
	case *js_ast.EPrivateIdentifier:
		return w.nameOf(k.Ref), true
	}
	return "", false
}

// local handles a variable declaration: function-valued bindings become
// symbols, and `require(...)` initializers become import bindings.
func (w *jsWalker) local(s *js_ast.SLocal) {
	for _, decl := range s.Decls {
		if w.requireBinding(decl) {
			continue
		}
		binding, isIdent := decl.Binding.Data.(*js_ast.BIdentifier)
		if isIdent && decl.ValueOrNil.Data != nil {
			name := w.nameOf(binding.Ref)
			switch value := decl.ValueOrNil.Data.(type) {
			case *js_ast.EArrow:
				symbol := w.addSymbol(name, "", int(decl.Binding.Loc.Start), w.fnEndLoc(value.Body), s.IsExport)
				if s.IsExport && !symbol.Nested {
					w.ir.Exports = append(w.ir.Exports, Export{ExportedName: name, LocalName: name})
				}
				w.under(symbol, func() { w.walkFnParts(value.Args, value.Body) })
				continue
			case *js_ast.EFunction:
				symbol := w.addSymbol(name, "", int(decl.Binding.Loc.Start), w.fnEndLoc(value.Fn.Body), s.IsExport)
				if s.IsExport && !symbol.Nested {
					w.ir.Exports = append(w.ir.Exports, Export{ExportedName: name, LocalName: name})
				}
				w.under(symbol, func() { w.walkFnParts(value.Fn.Args, value.Fn.Body) })
				continue
			}
		}
		w.walkBinding(decl.Binding)
		w.walkExpr(decl.ValueOrNil)
	}
}

// requireBinding recognizes `const x = require("spec")` and
// `const {a, b: c} = require("spec")`, recording import bindings.
func (w *jsWalker) requireBinding(decl js_ast.Decl) bool {
	spec, ok := w.requireSpec(decl.ValueOrNil)
	if !ok {
		return false
	}
	switch binding := decl.Binding.Data.(type) {
	case *js_ast.BIdentifier:
		w.ir.Imports = append(w.ir.Imports, Import{
			Alias:      w.nameOf(binding.Ref),
			ModuleSpec: spec,
			Kind:       "module",
		})
		return true
	case *js_ast.BObject:
		for _, property := range binding.Properties {
			value, isIdent := property.Value.Data.(*js_ast.BIdentifier)
			if !isIdent || property.IsComputed {
				continue
			}
			key, named := w.propertyName(property.Key)
			if !named {
				continue
			}
			w.ir.Imports = append(w.ir.Imports, Import{
				Alias:      w.nameOf(value.Ref),
				SymbolName: key,
				ModuleSpec: spec,
				Kind:       "symbol",
			})
		}
		return true
	}
	return false
}

func (w *jsWalker) requireSpec(value js_ast.Expr) (string, bool) {
	switch v := value.Data.(type) {
	case *js_ast.ERequireString:
		if int(v.ImportRecordIndex) < len(w.tree.ImportRecords) {
			return w.tree.ImportRecords[v.ImportRecordIndex].Path.Text, true
		}
	case *js_ast.ECall:
		target, isIdent := v.Target.Data.(*js_ast.EIdentifier)
		if !isIdent || w.nameOf(target.Ref) != "require" || len(v.Args) != 1 {
			return "", false
		}
		if str, isString := v.Args[0].Data.(*js_ast.EString); isString {
			return helpers.UTF16ToString(str.Value), true
		}
	}
	return "", false
}

func (w *jsWalker) importStmt(s *js_ast.SImport) {
	if int(s.ImportRecordIndex) >= len(w.tree.ImportRecords) {
		return
	}
	spec := w.tree.ImportRecords[s.ImportRecordIndex].Path.Text
	if s.DefaultName != nil {
		w.ir.Imports = append(w.ir.Imports, Import{
			Alias:      w.nameOf(s.DefaultName.Ref),
			SymbolName: "default",
			ModuleSpec: spec,
			Kind:       "symbol",
		})
	}
	if s.StarNameLoc != nil {
		w.ir.Imports = append(w.ir.Imports, Import{
			Alias:      w.nameOf(s.NamespaceRef),
			ModuleSpec: spec,
			Kind:       "module",
		})
	}
	if s.Items != nil {
		for _, item := range *s.Items {
			w.ir.Imports = append(w.ir.Imports, Import{
				Alias:      w.nameOf(item.Name.Ref),
				SymbolName: item.Alias,
				ModuleSpec: spec,
				Kind:       "symbol",
			})
		}
	}
}

func (w *jsWalker) exportDefault(s *js_ast.SExportDefault) {
	switch value := s.Value.Data.(type) {
	case *js_ast.SFunction:
		name := nameOrEmpty(w, value.Fn.Name)
		w.declareFn(name, s.Value.Loc, value.Fn, true)
		if name != "" {
			w.ir.Exports = append(w.ir.Exports, Export{ExportedName: "default", LocalName: name})
		}
	case *js_ast.SClass:
		w.declareClass(value.Class, s.Value.Loc, true)
	case *js_ast.SExpr:
		w.walkExpr(value.Value)
	}
}

// cjsExport recognizes `module.exports = {...}`, `module.exports.x = f`, and
// `exports.x = f` assignments; returns true if the statement was consumed.
func (w *jsWalker) cjsExport(value js_ast.Expr) bool {
	binary, isBinary := value.Data.(*js_ast.EBinary)
	if !isBinary || binary.Op != js_ast.BinOpAssign {
		return false
	}
	left, isDot := binary.Left.Data.(*js_ast.EDot)
	if !isDot {
		return false
	}
	if w.isModuleExports(binary.Left) {
		// module.exports = { a, b: c }
		object, isObject := binary.Right.Data.(*js_ast.EObject)
		if !isObject {
			return false
		}
		for _, property := range object.Properties {
			name, named := w.propertyName(property.Key)
			if !named {
				continue
			}
			if ident, isIdent := property.ValueOrNil.Data.(*js_ast.EIdentifier); isIdent {
				w.ir.Exports = append(w.ir.Exports, Export{ExportedName: name, LocalName: w.nameOf(ident.Ref)})
			}
		}
		return true
	}
	// module.exports.x = f  |  exports.x = f
	if base, isIdent := left.Target.Data.(*js_ast.EIdentifier); (isIdent && w.nameOf(base.Ref) == "exports") || w.isModuleExports(left.Target) {
		if ident, isIdent := binary.Right.Data.(*js_ast.EIdentifier); isIdent {
			w.ir.Exports = append(w.ir.Exports, Export{ExportedName: left.Name, LocalName: w.nameOf(ident.Ref)})
			return true
		}
	}
	return false
}

func (w *jsWalker) isModuleExports(expr js_ast.Expr) bool {
	dot, isDot := expr.Data.(*js_ast.EDot)
	if !isDot || dot.Name != "exports" {
		return false
	}
	base, isIdent := dot.Target.Data.(*js_ast.EIdentifier)
	return isIdent && w.nameOf(base.Ref) == "module"
}

// walkFnParts walks a function's argument defaults and body statements.
func (w *jsWalker) walkFnParts(args []js_ast.Arg, body js_ast.FnBody) {
	for _, arg := range args {
		w.walkBinding(arg.Binding)
		w.walkExpr(arg.DefaultOrNil)
	}
	for _, stmt := range body.Block.Stmts {
		w.walkStmt(stmt)
	}
}

func (w *jsWalker) walkBinding(binding js_ast.Binding) {
	switch b := binding.Data.(type) {
	case *js_ast.BArray:
		for _, item := range b.Items {
			w.walkBinding(item.Binding)
			w.walkExpr(item.DefaultValueOrNil)
		}
	case *js_ast.BObject:
		for _, property := range b.Properties {
			w.walkExpr(property.Key)
			w.walkBinding(property.Value)
			w.walkExpr(property.DefaultValueOrNil)
		}
	}
}

// walkStmt walks a statement inside (or outside) a symbol body, registering
// nested named function definitions as symbols and recording calls.
func (w *jsWalker) walkStmt(stmt js_ast.Stmt) {
	switch s := stmt.Data.(type) {
	case nil:
	case *js_ast.SBlock:
		for _, inner := range s.Stmts {
			w.walkStmt(inner)
		}
	case *js_ast.SExpr:
		w.walkExpr(s.Value)
	case *js_ast.SLocal:
		w.local(s)
	case *js_ast.SFunction:
		w.declareFn(nameOrEmpty(w, s.Fn.Name), stmt.Loc, s.Fn, false)
	case *js_ast.SClass:
		w.declareClass(s.Class, stmt.Loc, false)
	case *js_ast.SReturn:
		w.walkExpr(s.ValueOrNil)
	case *js_ast.SThrow:
		w.walkExpr(s.Value)
	case *js_ast.SIf:
		w.walkExpr(s.Test)
		w.walkStmt(s.Yes)
		w.walkStmt(s.NoOrNil)
	case *js_ast.SFor:
		w.walkStmt(s.InitOrNil)
		w.walkExpr(s.TestOrNil)
		w.walkExpr(s.UpdateOrNil)
		w.walkStmt(s.Body)
	case *js_ast.SForIn:
		w.walkStmt(s.Init)
		w.walkExpr(s.Value)
		w.walkStmt(s.Body)
	case *js_ast.SForOf:
		w.walkStmt(s.Init)
		w.walkExpr(s.Value)
		w.walkStmt(s.Body)
	case *js_ast.SWhile:
		w.walkExpr(s.Test)
		w.walkStmt(s.Body)
	case *js_ast.SDoWhile:
		w.walkStmt(s.Body)
		w.walkExpr(s.Test)
	case *js_ast.SSwitch:
		w.walkExpr(s.Test)
		for _, c := range s.Cases {
			w.walkExpr(c.ValueOrNil)
			for _, inner := range c.Body {
				w.walkStmt(inner)
			}
		}
	case *js_ast.STry:
		for _, inner := range s.Block.Stmts {
			w.walkStmt(inner)
		}
		if s.Catch != nil {
			for _, inner := range s.Catch.Block.Stmts {
				w.walkStmt(inner)
			}
		}
		if s.Finally != nil {
			for _, inner := range s.Finally.Block.Stmts {
				w.walkStmt(inner)
			}
		}
	case *js_ast.SLabel:
		w.walkStmt(s.Stmt)
	case *js_ast.SExportDefault:
		w.exportDefault(s)
	case *js_ast.SNamespace:
		for _, inner := range s.Stmts {
			w.walkStmt(inner)
		}
	case *js_ast.SEnum:
		for _, value := range s.Values {
			w.walkExpr(value.ValueOrNil)
		}
	}
}

// walkExpr walks an expression, recording calls against the innermost symbol.
func (w *jsWalker) walkExpr(expr js_ast.Expr) {
	switch e := expr.Data.(type) {
	case nil:
	case *js_ast.ECall:
		w.recordCall(e)
		w.walkExpr(e.Target)
		for _, arg := range e.Args {
			w.walkExpr(arg)
		}
	case *js_ast.ENew:
		w.walkExpr(e.Target)
		for _, arg := range e.Args {
			w.walkExpr(arg)
		}
	case *js_ast.EDot:
		w.walkExpr(e.Target)
	case *js_ast.EIndex:
		w.walkExpr(e.Target)
		w.walkExpr(e.Index)
	case *js_ast.EArrow:
		w.walkFnParts(e.Args, e.Body)
	case *js_ast.EFunction:
		w.walkFnParts(e.Fn.Args, e.Fn.Body)
	case *js_ast.EClass:
		w.declareClass(e.Class, expr.Loc, false)
	case *js_ast.EBinary:
		w.walkExpr(e.Left)
		w.walkExpr(e.Right)
	case *js_ast.EUnary:
		w.walkExpr(e.Value)
	case *js_ast.EIf:
		w.walkExpr(e.Test)
		w.walkExpr(e.Yes)
		w.walkExpr(e.No)
	case *js_ast.EArray:
		for _, item := range e.Items {
			w.walkExpr(item)
		}
	case *js_ast.EObject:
		for _, property := range e.Properties {
			w.walkExpr(property.Key)
			w.walkExpr(property.ValueOrNil)
			w.walkExpr(property.InitializerOrNil)
		}
	case *js_ast.ESpread:
		w.walkExpr(e.Value)
	case *js_ast.EAwait:
		w.walkExpr(e.Value)
	case *js_ast.EYield:
		w.walkExpr(e.ValueOrNil)
	case *js_ast.ETemplate:
		w.walkExpr(e.TagOrNil)
		for _, part := range e.Parts {
			w.walkExpr(part.Value)
		}
	case *js_ast.EJSXElement:
		w.walkExpr(e.TagOrNil)
		for _, property := range e.Properties {
			w.walkExpr(property.Key)
			w.walkExpr(property.ValueOrNil)
			w.walkExpr(property.InitializerOrNil)
		}
		for _, child := range e.NullableChildren {
			w.walkExpr(child)
		}
	case *js_ast.EImportCall:
		w.walkExpr(e.Expr)
		w.walkExpr(e.OptionsOrNil)
	}
}

// recordCall classifies a call site and attributes it to the innermost
// enclosing symbol. Classification mirrors the previous regex semantics:
// optional chaining, call-of-call, and computed-member calls are dynamic;
// member calls keep their single identifier base for backend resolution.
func (w *jsWalker) recordCall(call *js_ast.ECall) {
	symbol := w.current()
	if symbol == nil {
		return
	}
	if call.OptionalChain != js_ast.OptionalChainNone {
		symbol.Dynamic = true
		return
	}
	switch target := call.Target.Data.(type) {
	case *js_ast.EIdentifier:
		name := w.nameOf(target.Ref)
		if name == "require" {
			return
		}
		symbol.Calls = append(symbol.Calls, Call{Name: name, Kind: CallBare})
	case *js_ast.EImportIdentifier:
		symbol.Calls = append(symbol.Calls, Call{Name: w.nameOf(target.Ref), Kind: CallBare})
	case *js_ast.EDot:
		if target.OptionalChain != js_ast.OptionalChainNone {
			symbol.Dynamic = true
			return
		}
		switch base := target.Target.Data.(type) {
		case *js_ast.EThis:
			symbol.Calls = append(symbol.Calls, Call{Name: target.Name, Base: "this", Kind: CallSelf})
		case *js_ast.EIdentifier:
			symbol.Calls = append(symbol.Calls, Call{Name: target.Name, Base: w.nameOf(base.Ref), Kind: CallMember})
		case *js_ast.EImportIdentifier:
			symbol.Calls = append(symbol.Calls, Call{Name: target.Name, Base: w.nameOf(base.Ref), Kind: CallMember})
		case *js_ast.EDot:
			// Chained member call a.b.c(): resolve by the trailing pair, like
			// the line-based extraction did.
			symbol.Calls = append(symbol.Calls, Call{Name: target.Name, Base: base.Name, Kind: CallMember})
		}
	case *js_ast.EIndex:
		if _, isPrivate := target.Index.Data.(*js_ast.EPrivateIdentifier); isPrivate {
			if _, isThis := target.Target.Data.(*js_ast.EThis); isThis {
				name, _ := w.propertyName(js_ast.Expr{Data: target.Index.Data})
				symbol.Calls = append(symbol.Calls, Call{Name: name, Base: "this", Kind: CallSelf})
				return
			}
		}
		symbol.Dynamic = true
	case *js_ast.ECall, *js_ast.EArrow, *js_ast.EFunction:
		// Immediately-invoked or curried call: f()(), (() => {})().
		symbol.Dynamic = true
	}
}

// maxLocIn returns the maximum logger.Loc.Start found anywhere in the given
// AST fragment, via reflection. Used only to bound expression-bodied arrow
// functions, which carry no closing-brace location.
func maxLocIn(fragment any) int {
	max := 0
	seen := map[uintptr]struct{}{}
	var visit func(v reflect.Value)
	visit = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				return
			}
			ptr := v.Pointer()
			if _, ok := seen[ptr]; ok {
				return
			}
			seen[ptr] = struct{}{}
			visit(v.Elem())
		case reflect.Interface:
			if !v.IsNil() {
				visit(v.Elem())
			}
		case reflect.Struct:
			if v.Type() == locType {
				if start := int(v.FieldByName("Start").Int()); start > max {
					max = start
				}
				return
			}
			for i := 0; i < v.NumField(); i++ {
				visit(v.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				visit(v.Index(i))
			}
		}
	}
	visit(reflect.ValueOf(fragment))
	return max
}

var locType = reflect.TypeOf(logger.Loc{})
