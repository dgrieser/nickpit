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
			key := nodeKey(name, rel)
			graph.nodes[key] = Node{
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
				addEdge(graph.callees, key, callee)
				addEdge(graph.callers, nodeKey(callee, ""), key)
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

func (g *Graph) Find(name, path string, depth int, reverse bool) (*Hierarchy, error) {
	key := g.resolveKey(name, path)
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

func (g *Graph) expand(name string, depth int, reverse bool, seen map[string]struct{}) []Node {
	if depth == 0 {
		return nil
	}
	edges := g.callees[name]
	if reverse {
		edges = g.callers[name]
		if len(edges) == 0 {
			if node, ok := g.nodes[name]; ok {
				edges = g.callers[node.Name]
			}
		}
	}
	names := make([]string, 0, len(edges))
	for candidate := range edges {
		names = append(names, candidate)
	}
	sort.Strings(names)
	var out []Node
	for _, childName := range names {
		childKey, node, ok := g.lookupNode(childName)
		if _, exists := seen[childKey]; exists {
			continue
		}
		if !ok {
			continue
		}
		nextSeen := copySeen(seen)
		nextSeen[childKey] = struct{}{}
		node.Children = g.expand(childKey, depth-1, reverse, nextSeen)
		out = append(out, node)
	}
	return out
}

func (g *Graph) resolveKey(name, path string) string {
	if path != "" {
		return nodeKey(name, path)
	}
	if _, ok := g.nodes[name]; ok {
		return name
	}
	for key, node := range g.nodes {
		if node.Name == name {
			return key
		}
	}
	return name
}

func (g *Graph) lookupNode(key string) (string, Node, bool) {
	if node, ok := g.nodes[key]; ok {
		return key, node, true
	}
	for candidateKey, node := range g.nodes {
		if node.Name == key {
			return candidateKey, node, true
		}
	}
	return "", Node{}, false
}

func addEdge(index map[string]map[string]struct{}, from, to string) {
	if index[from] == nil {
		index[from] = map[string]struct{}{}
	}
	index[from][to] = struct{}{}
}

func nodeKey(name, path string) string {
	path = filepath.ToSlash(path)
	if path == "" {
		return name
	}
	return path + ":" + name
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
