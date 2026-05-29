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
	"sync"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
	"golang.org/x/tools/go/packages"
)

// MaxCallHierarchyDepth bounds traversal depth. Depth flows in from an
// LLM/CLI-supplied argument and only the low side was clamped, so a runaway
// value could request an explosively large hierarchy on a dense graph. The
// graph is the trusted local repo, so this is a generous safety ceiling.
// Exported so the regex backends and the tool schema share one source of truth.
const MaxCallHierarchyDepth = 50

type graphCacheEntry struct {
	mu    sync.Mutex
	graph *Graph
}

// graphCache memoizes the type-checked call graph per repository root. The
// graph is immutable after construction and Find only reads it, so the cached
// value is shared safely across the concurrent reviewer/verifier agents that
// previously each triggered a full packages.Load("./...") type-check.
var graphCache sync.Map // abs repoRoot -> *graphCacheEntry

// BuildGraphCached returns the call graph for repoRoot, building it at most
// once per root. Errors are not cached, so a transient build failure can be
// retried on a later call.
func BuildGraphCached(ctx context.Context, repoRoot string) (*Graph, error) {
	key := repoRoot
	if abs, err := filepath.Abs(repoRoot); err == nil {
		key = abs
	}
	actual, _ := graphCache.LoadOrStore(key, &graphCacheEntry{})
	entry := actual.(*graphCacheEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.graph != nil {
		return entry.graph, nil
	}
	// If the context was cancelled while waiting for the lock, don't start the
	// expensive type-check.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	graph, err := BuildGraph(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	entry.graph = graph
	return graph, nil
}

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
	if depth > MaxCallHierarchyDepth {
		depth = MaxCallHierarchyDepth
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
		// Mark the child as on the current path, recurse, then unmark on the way
		// back up. This is the same path-local cycle avoidance the previous
		// per-child map copy provided, without the O(path) allocation per child.
		seen[childID] = struct{}{}
		node.Children = g.expand(childID, depth-1, reverse, seen)
		delete(seen, childID)
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
