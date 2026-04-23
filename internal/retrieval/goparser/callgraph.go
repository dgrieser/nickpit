package goparser

import (
	"context"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
	"golang.org/x/tools/go/packages"
)

type Graph struct {
	repoRoot   string
	nodes      map[string]Node
	callers    map[string]map[string]struct{}
	callees    map[string]map[string]struct{}
	byPathName map[string][]string
	byName     map[string][]string
}

type Hierarchy struct {
	Root  Node
	Mode  string
	Depth int
}

type Node struct {
	Name      string
	Path      string
	StartLine int
	EndLine   int
	Source    string
	Children  []Node
}

func BuildGraph(ctx context.Context, repoRoot string) (*Graph, error) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     repoRoot,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	graph := &Graph{
		repoRoot:   repoRoot,
		nodes:      map[string]Node{},
		callers:    map[string]map[string]struct{}{},
		callees:    map[string]map[string]struct{}{},
		byPathName: map[string][]string{},
		byName:     map[string][]string{},
	}

	for _, pkg := range pkgs {
		if pkg == nil || pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}
				obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok || obj == nil {
					return false
				}
				node, id, ok := buildNode(repoRoot, pkg.Fset, fn, obj)
				if !ok {
					return false
				}
				graph.nodes[id] = node
				graph.byPathName[pathNameKey(node.Name, node.Path)] = append(graph.byPathName[pathNameKey(node.Name, node.Path)], id)
				graph.byName[node.Name] = append(graph.byName[node.Name], id)
				return false
			})
		}
	}

	for _, pkg := range pkgs {
		if pkg == nil || pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					return true
				}
				obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok || obj == nil {
					return false
				}
				callerID := objectID(pkg.Fset, obj)
				if _, ok := graph.nodes[callerID]; !ok {
					return false
				}

				ast.Inspect(fn.Body, func(child ast.Node) bool {
					call, ok := child.(*ast.CallExpr)
					if !ok {
						return true
					}
					callee := resolveCallee(pkg.TypesInfo, call.Fun)
					if callee == nil {
						return true
					}
					calleeID := objectID(pkg.Fset, callee)
					if _, ok := graph.nodes[calleeID]; !ok {
						return true
					}
					addEdge(graph.callees, callerID, calleeID)
					addEdge(graph.callers, calleeID, callerID)
					return true
				})
				return false
			})
		}
	}

	if len(graph.nodes) == 0 {
		return nil, fmt.Errorf("no Go functions found")
	}
	return graph, nil
}

func (g *Graph) Find(name, path string, depth int, reverse bool) (*Hierarchy, error) {
	key, err := g.resolveKey(name, path)
	if err != nil {
		return nil, err
	}
	root, ok := g.nodes[key]
	if !ok {
		if path != "" {
			return nil, fmt.Errorf("symbol %q not found in %q", name, path)
		}
		return nil, fmt.Errorf("symbol %q not found", name)
	}
	if depth <= 0 {
		depth = 1
	}
	seen := map[string]struct{}{key: {}}
	mode := "callees"
	if reverse {
		mode = "callers"
	}
	root.Children = g.expand(key, depth, reverse, seen)
	return &Hierarchy{
		Root:  root,
		Mode:  mode,
		Depth: depth,
	}, nil
}

func (g *Graph) expand(id string, depth int, reverse bool, seen map[string]struct{}) []Node {
	if depth == 0 {
		return nil
	}
	edges := g.callees[id]
	if reverse {
		edges = g.callers[id]
	}
	childIDs := make([]string, 0, len(edges))
	for candidate := range edges {
		childIDs = append(childIDs, candidate)
	}
	sort.Slice(childIDs, func(i, j int) bool {
		left := g.nodes[childIDs[i]]
		right := g.nodes[childIDs[j]]
		if left.Path == right.Path {
			return left.StartLine < right.StartLine
		}
		return left.Path < right.Path
	})

	var out []Node
	for _, childID := range childIDs {
		if _, exists := seen[childID]; exists {
			continue
		}
		node, ok := g.nodes[childID]
		if !ok {
			continue
		}
		nextSeen := copySeen(seen)
		nextSeen[childID] = struct{}{}
		node.Children = g.expand(childID, depth-1, reverse, nextSeen)
		out = append(out, node)
	}
	return out
}

func (g *Graph) resolveKey(name, path string) (string, error) {
	normalizedPath := normalizeLookupPath(path)
	if normalizedPath != "" {
		candidates := g.byPathName[pathNameKey(name, normalizedPath)]
		switch len(candidates) {
		case 0:
			scopeCandidates, err := g.resolveScopedCandidates(name, normalizedPath)
			if err != nil {
				return "", err
			}
			if len(scopeCandidates) == 0 {
				return "", fmt.Errorf("symbol %q not found in %q", name, normalizedPath)
			}
			return scopeCandidates[0], nil
		case 1:
			return candidates[0], nil
		default:
			sortNodeIDs(candidates, g.nodes)
			return candidates[0], nil
		}
	}

	candidates := g.byName[name]
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("symbol %q not found", name)
	default:
		sortNodeIDs(candidates, g.nodes)
		return candidates[0], nil
	}
}

func (g *Graph) resolveScopedCandidates(name, path string) ([]string, error) {
	_, fullPath, err := repofs.ResolvePath(g.repoRoot, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	prefix := path
	if prefix != "" {
		prefix += "/"
	}
	candidates := make([]string, 0)
	for _, id := range g.byName[name] {
		node, ok := g.nodes[id]
		if !ok {
			continue
		}
		if node.Path == path || strings.HasPrefix(node.Path, prefix) {
			candidates = append(candidates, id)
		}
	}
	sortNodeIDs(candidates, g.nodes)
	return candidates, nil
}

func sortNodeIDs(ids []string, nodes map[string]Node) {
	sort.Slice(ids, func(i, j int) bool {
		left := nodes[ids[i]]
		right := nodes[ids[j]]
		if left.Path == right.Path {
			return left.StartLine < right.StartLine
		}
		return left.Path < right.Path
	})
}

func normalizeLookupPath(path string) string {
	return repofs.NormalizePath(path)
}

func buildNode(repoRoot string, fset *token.FileSet, fn *ast.FuncDecl, obj *types.Func) (Node, string, bool) {
	start := fset.Position(fn.Pos())
	end := fset.Position(fn.End())
	if start.Filename == "" {
		return Node{}, "", false
	}
	var buf strings.Builder
	if err := printer.Fprint(&buf, fset, fn); err != nil {
		return Node{}, "", false
	}
	rel, err := filepath.Rel(repoRoot, start.Filename)
	if err != nil {
		return Node{}, "", false
	}
	node := Node{
		Name:      fn.Name.Name,
		Path:      filepath.ToSlash(rel),
		StartLine: start.Line,
		EndLine:   end.Line,
		Source:    buf.String(),
	}
	return node, objectID(fset, obj), true
}

func resolveCallee(info *types.Info, expr ast.Expr) *types.Func {
	switch value := expr.(type) {
	case *ast.Ident:
		obj, _ := info.Uses[value].(*types.Func)
		return obj
	case *ast.SelectorExpr:
		if sel := info.Selections[value]; sel != nil {
			obj, _ := sel.Obj().(*types.Func)
			return obj
		}
		obj, _ := info.Uses[value.Sel].(*types.Func)
		return obj
	default:
		return nil
	}
}

func objectID(fset *token.FileSet, obj *types.Func) string {
	pkgPath := ""
	if obj.Pkg() != nil {
		pkgPath = obj.Pkg().Path()
	}
	pos := fset.Position(obj.Pos())
	return fmt.Sprintf("%s:%s:%d:%d", pkgPath, filepath.ToSlash(pos.Filename), pos.Line, pos.Column)
}

func pathNameKey(name, path string) string {
	return filepath.ToSlash(path) + "\x00" + name
}

func addEdge(index map[string]map[string]struct{}, from, to string) {
	if index[from] == nil {
		index[from] = map[string]struct{}{}
	}
	index[from][to] = struct{}{}
}

func copySeen(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}
