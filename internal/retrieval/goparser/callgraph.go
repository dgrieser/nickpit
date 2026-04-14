package goparser

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
)

type Graph struct {
	nodes   map[string]Node
	callers map[string]map[string]struct{}
	callees map[string]map[string]struct{}
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
	Children  []Node
}

func BuildGraph(_ context.Context, repoRoot string) (*Graph, error) {
	graph := &Graph{
		nodes:   map[string]Node{},
		callers: map[string]map[string]struct{}{},
		callees: map[string]map[string]struct{}{},
	}
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}
			name := fn.Name.Name
			graph.nodes[name] = Node{
				Name:      name,
				Path:      filepath.ToSlash(rel),
				StartLine: fset.Position(fn.Pos()).Line,
				EndLine:   fset.Position(fn.End()).Line,
			}
			ast.Inspect(fn.Body, func(child ast.Node) bool {
				call, ok := child.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee := exprName(call.Fun)
				if callee == "" {
					return true
				}
				addEdge(graph.callees, name, callee)
				addEdge(graph.callers, callee, name)
				return true
			})
			return false
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(graph.nodes) == 0 {
		return nil, fmt.Errorf("no Go functions found")
	}
	return graph, nil
}

func (g *Graph) Find(name string, depth int, reverse bool) (*Hierarchy, error) {
	root, ok := g.nodes[name]
	if !ok {
		return nil, fmt.Errorf("symbol %q not found", name)
	}
	if depth <= 0 {
		depth = 1
	}
	seen := map[string]struct{}{name: {}}
	mode := "callees"
	if reverse {
		mode = "callers"
	}
	root.Children = g.expand(name, depth, reverse, seen)
	return &Hierarchy{
		Root:  root,
		Mode:  mode,
		Depth: depth,
	}, nil
}

func (g *Graph) expand(name string, depth int, reverse bool, seen map[string]struct{}) []Node {
	if depth == 0 {
		return nil
	}
	edges := g.callees[name]
	if reverse {
		edges = g.callers[name]
	}
	names := make([]string, 0, len(edges))
	for candidate := range edges {
		names = append(names, candidate)
	}
	sort.Strings(names)
	var out []Node
	for _, childName := range names {
		if _, exists := seen[childName]; exists {
			continue
		}
		node, ok := g.nodes[childName]
		if !ok {
			continue
		}
		nextSeen := copySeen(seen)
		nextSeen[childName] = struct{}{}
		node.Children = g.expand(childName, depth-1, reverse, nextSeen)
		out = append(out, node)
	}
	return out
}

func addEdge(index map[string]map[string]struct{}, from, to string) {
	if index[from] == nil {
		index[from] = map[string]struct{}{}
	}
	index[from][to] = struct{}{}
}

func exprName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}

func copySeen(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}
