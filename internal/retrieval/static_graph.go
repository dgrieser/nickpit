package retrieval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
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

type staticGraphCacheEntry struct {
	mu    sync.Mutex
	graph *staticGraph
}

// defaultMaxStaticGraphCacheEntries bounds how many distinct (language, repoRoot,
// scope) graphs staticGraphCache will memoize within a single run. Of the three
// key dimensions, language (python/nodejs/rust — Go uses goparser.BuildGraphCached)
// and repoRoot (one repo per CLI run) are bounded by code; only the scope is not.
// scopeForHierarchy yields either the repo-wide empty scope or a directory scope,
// and directory paths come solely from find_callers/find_callees tool arguments,
// so a pathological agent could query arbitrarily many distinct directories. This
// cap turns that one unbounded axis into a hard bound.
const defaultMaxStaticGraphCacheEntries = 64

// staticGraphCacheCap resolves the admission cap on each cache miss. Misses are
// rare, so reading the env at point of use is cheap and mirrors modelcheck/cache.go;
// NICKPIT_GRAPH_CACHE_MAX_ENTRIES lets large monorepos tune it, and a value <= 0
// disables the cap entirely (unbounded — the pre-cap behavior).
func staticGraphCacheCap() int {
	raw := strings.TrimSpace(os.Getenv("NICKPIT_GRAPH_CACHE_MAX_ENTRIES"))
	if raw == "" {
		return defaultMaxStaticGraphCacheEntries
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultMaxStaticGraphCacheEntries
	}
	return n
}

// staticGraphCacheStore memoizes a statically built call graph per (language, repoRoot,
// scope). The graph is immutable after construction and find() only reads it, so
// the cached value is shared safely across the concurrent reviewer/verifier agents
// that previously each re-read and re-parsed every source file. It mirrors
// goparser.BuildGraphCached; the AST-based backends (rust/python/node) are far cheaper
// to build than Go's type-checked graph, but the redundant file I/O and parsing
// still add up across calls (e.g. find_callers + find_callees for the same symbol
// run as separate tool calls).
//
// The cache is intentionally append-only within a single short-lived run: the CLI
// reviews one repo and exits, reclaiming everything. The soft cap (see
// staticGraphCacheCap) is a backstop against the unbounded directory-scope axis —
// once full, a new directory scope is rebuilt without caching, exactly the
// pre-cache behavior and never a failure. The repo-wide scope is exempt because it
// is the dominant per-graph cost (a full-repo parse) and the most-reused key.
type staticGraphCacheStore struct {
	entries sync.Map     // key -> *staticGraphCacheEntry
	count   atomic.Int64 // number of admitted (stored) keys
}

var staticGraphCache = &staticGraphCacheStore{} // one cache for the whole run

// buildStaticGraphCached returns the graph for (language, repoRoot, scope),
// invoking build at most once per key. The scope participates in the key because
// the static-graph builders are scope-dependent; scopeForHierarchy only ever yields the
// repo-wide empty scope or a directory scope, so Path+IsDir fully identifies it.
// Errors are not cached, so a transient build failure can be retried later.
func buildStaticGraphCached(language, repoRoot string, scope lookupScope, build func() (*staticGraph, error)) (*staticGraph, error) {
	root := repoRoot
	if abs, err := filepath.Abs(repoRoot); err == nil {
		root = abs
	}
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%t", language, root, scope.Path, scope.IsDir)
	repoWide := scope.Path == "" && !scope.IsDir
	return staticGraphCache.getOrBuild(key, repoWide, build)
}

// getOrBuild serves an already-cached key unconditionally, and admits a new key
// only while under the cap (the repo-wide scope is always admitted). A refused key
// is built and returned without being cached, so correctness is unaffected.
func (c *staticGraphCacheStore) getOrBuild(key string, repoWide bool, build func() (*staticGraph, error)) (*staticGraph, error) {
	if actual, ok := c.entries.Load(key); ok {
		return buildLocked(actual.(*staticGraphCacheEntry), build)
	}
	if max := staticGraphCacheCap(); !repoWide && max > 0 && c.count.Load() >= int64(max) {
		return build()
	}
	actual, loaded := c.entries.LoadOrStore(key, &staticGraphCacheEntry{})
	if !loaded {
		c.count.Add(1) // count only genuine stores, so lost races never overcount
	}
	return buildLocked(actual.(*staticGraphCacheEntry), build)
}

// buildLocked runs build at most once per entry and shares the immutable result;
// errors are NOT cached, so a transient failure retries on the next call.
func buildLocked(entry *staticGraphCacheEntry, build func() (*staticGraph, error)) (*staticGraph, error) {
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.graph != nil {
		return entry.graph, nil
	}
	graph, err := build()
	if err != nil {
		return nil, err
	}
	entry.graph = graph
	return graph, nil
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
		return nil, &LowConfidenceError{language: g.language}
	}
	if depth <= 0 {
		depth = 1
	}
	if depth > goparser.MaxCallHierarchyDepth {
		depth = goparser.MaxCallHierarchyDepth
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
		// Mark on the current path, recurse, unmark on backtrack: same
		// path-local cycle avoidance as the previous per-child map copy, without
		// the O(path) allocation per child.
		seen[childID] = struct{}{}
		out.Children = append(out.Children, g.expandNode(childID, depth-1, reverse, seen))
		delete(seen, childID)
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

func addEdge(edges map[string]map[string]struct{}, from, to string) {
	if edges[from] == nil {
		edges[from] = map[string]struct{}{}
	}
	edges[from][to] = struct{}{}
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
