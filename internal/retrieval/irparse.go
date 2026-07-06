package retrieval

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
	"github.com/dgrieser/nickpit/internal/retrieval/tsparser"
)

// parseIRFiles parses files (absolute paths under repoRoot) with tsparser in
// parallel and returns the IR keyed by repo-relative slash path.
func parseIRFiles(repoRoot string, files []string) (map[string]*tsparser.FileIR, error) {
	type result struct {
		rel string
		ir  *tsparser.FileIR
		err error
	}
	jobs := make(chan string)
	results := make(chan result)
	var wg sync.WaitGroup
	workers := min(runtime.GOMAXPROCS(0), len(files))
	for range workers {
		wg.Go(func() {
			for fullPath := range jobs {
				rel, err := filepath.Rel(repoRoot, fullPath)
				if err != nil {
					results <- result{err: err}
					continue
				}
				rel = filepath.ToSlash(rel)
				data, err := repofs.ReadFile(repoRoot, fullPath)
				if err != nil {
					results <- result{err: err}
					continue
				}
				ir, err := tsparser.ParseFile(rel, data)
				results <- result{rel: rel, ir: ir, err: err}
			}
		})
	}
	go func() {
		for _, fullPath := range files {
			jobs <- fullPath
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	out := make(map[string]*tsparser.FileIR, len(files))
	var firstErr error
	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		out[res.rel] = res.ir
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// sortSymbolInfos orders symbol results by path, then start line.
func sortSymbolInfos(out []*SymbolInfo) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Path < out[j].Path
	})
}

// symbolsNamed returns SymbolInfo entries for every graph node with the given
// name whose path falls inside scopePath ("" = everywhere). Used to answer
// symbol lookups from the cached call graph instead of re-parsing the tree.
func (g *staticGraph) symbolsNamed(name, scopePath string) []*SymbolInfo {
	var out []*SymbolInfo
	for _, id := range g.byName[name] {
		node := g.nodes[id]
		if scopePath != "" && node.Path != scopePath && !strings.HasPrefix(node.Path, scopePath+"/") {
			continue
		}
		out = append(out, &SymbolInfo{
			Name:      node.Name,
			Path:      node.Path,
			StartLine: node.StartLine,
			EndLine:   node.EndLine,
			Source:    node.Source,
			Language:  g.language,
		})
	}
	sortSymbolInfos(out)
	return out
}
