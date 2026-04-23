package retrieval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
)

type staticNode struct {
	Name      string
	Path      string
	StartLine int
	EndLine   int
	Source    string
}

type staticGraph struct {
	language      string
	repoRoot      string
	nodes         map[string]staticNode
	callers       map[string]map[string]struct{}
	callees       map[string]map[string]struct{}
	byPathName    map[string][]string
	byName        map[string][]string
	lowConfidence map[string]bool
}

func newStaticGraph(language, repoRoot string) *staticGraph {
	return &staticGraph{
		language:      language,
		repoRoot:      repoRoot,
		nodes:         map[string]staticNode{},
		callers:       map[string]map[string]struct{}{},
		callees:       map[string]map[string]struct{}{},
		byPathName:    map[string][]string{},
		byName:        map[string][]string{},
		lowConfidence: map[string]bool{},
	}
}

func (g *staticGraph) addNode(id string, node staticNode) {
	g.nodes[id] = node
	g.byPathName[pathNameKey(node.Name, node.Path)] = append(g.byPathName[pathNameKey(node.Name, node.Path)], id)
	g.byName[node.Name] = append(g.byName[node.Name], id)
}

func (g *staticGraph) addEdge(callerID, calleeID string) {
	addEdge(g.callees, callerID, calleeID)
	addEdge(g.callers, calleeID, callerID)
}

func (g *staticGraph) markLowConfidence(id string) {
	g.lowConfidence[id] = true
}

func (g *staticGraph) find(name, path string, depth int, reverse bool) (*CallHierarchy, error) {
	key, err := g.resolveKey(name, path)
	if err != nil {
		return nil, err
	}
	_, ok := g.nodes[key]
	if !ok {
		if path != "" {
			return nil, fmt.Errorf("symbol %q not found in %q", name, path)
		}
		return nil, fmt.Errorf("symbol %q not found", name)
	}
	if g.lowConfidence[key] {
		return nil, &lowConfidenceError{language: g.language}
	}
	if depth <= 0 {
		depth = 1
	}
	seen := map[string]struct{}{key: {}}
	mode := "callees"
	if reverse {
		mode = "callers"
	}
	return &CallHierarchy{
		Root:  g.expandNode(key, depth, reverse, seen),
		Mode:  mode,
		Depth: depth,
	}, nil
}

func (g *staticGraph) expandNode(id string, depth int, reverse bool, seen map[string]struct{}) CallNode {
	node := g.nodes[id]
	out := CallNode{
		Name:      node.Name,
		Path:      node.Path,
		StartLine: node.StartLine,
		EndLine:   node.EndLine,
		Source:    node.Source,
		Children:  []CallNode{},
	}
	if depth == 0 {
		return out
	}
	edges := g.callees[id]
	if reverse {
		edges = g.callers[id]
	}
	childIDs := make([]string, 0, len(edges))
	for childID := range edges {
		childIDs = append(childIDs, childID)
	}
	sortNodeIDs(childIDs, g.nodes)
	for _, childID := range childIDs {
		if _, exists := seen[childID]; exists {
			continue
		}
		nextSeen := copySeen(seen)
		nextSeen[childID] = struct{}{}
		out.Children = append(out.Children, g.expandNode(childID, depth-1, reverse, nextSeen))
	}
	return out
}

func (g *staticGraph) resolveKey(name, path string) (string, error) {
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
	if len(candidates) == 0 {
		return "", fmt.Errorf("symbol %q not found", name)
	}
	sortNodeIDs(candidates, g.nodes)
	return candidates[0], nil
}

func (g *staticGraph) resolveScopedCandidates(name, path string) ([]string, error) {
	info, err := os.Stat(filepath.Join(g.repoRoot, filepath.FromSlash(path)))
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

func addEdge(edges map[string]map[string]struct{}, from, to string) {
	if edges[from] == nil {
		edges[from] = map[string]struct{}{}
	}
	edges[from][to] = struct{}{}
}

func copySeen(src map[string]struct{}) map[string]struct{} {
	dst := make(map[string]struct{}, len(src)+1)
	for key := range src {
		dst[key] = struct{}{}
	}
	return dst
}

func sortNodeIDs(ids []string, nodes map[string]staticNode) {
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

func pathNameKey(name, path string) string {
	return filepath.ToSlash(path) + "\x00" + name
}
