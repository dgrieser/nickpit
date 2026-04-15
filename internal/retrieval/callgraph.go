package retrieval

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
)

func (e *LocalEngine) FindCallers(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error) {
	graph, err := goparser.BuildGraph(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("building Go call graph: %w", err)
	}
	hierarchy, err := graph.Find(symbol.Name, symbol.Path, depth, true)
	if err != nil {
		return nil, fmt.Errorf("finding callers for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	return convertHierarchy(hierarchy), nil
}

func (e *LocalEngine) FindCallees(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error) {
	graph, err := goparser.BuildGraph(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("building Go call graph: %w", err)
	}
	hierarchy, err := graph.Find(symbol.Name, symbol.Path, depth, false)
	if err != nil {
		return nil, fmt.Errorf("finding callees for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	return convertHierarchy(hierarchy), nil
}

func convertHierarchy(src *goparser.Hierarchy) *CallHierarchy {
	if src == nil {
		return nil
	}
	return &CallHierarchy{
		Root:  convertNode(src.Root),
		Mode:  src.Mode,
		Depth: src.Depth,
	}
}

func convertNode(src goparser.Node) CallNode {
	node := CallNode{
		Name:      src.Name,
		Path:      src.Path,
		StartLine: src.StartLine,
		EndLine:   src.EndLine,
	}
	for _, child := range src.Children {
		node.Children = append(node.Children, convertNode(child))
	}
	return node
}

func (h *CallHierarchy) Render() string {
	var b strings.Builder
	renderNode(&b, h.Root, "", true)
	return strings.TrimRight(b.String(), "\n")
}

func renderNode(b *strings.Builder, node CallNode, prefix string, last bool) {
	connector := "├── "
	nextPrefix := prefix + "│   "
	if prefix == "" {
		connector = ""
		nextPrefix = ""
	} else if last {
		connector = "└── "
		nextPrefix = prefix + "    "
	}
	fmt.Fprintf(b, "%s%s%s (%s:%d-%d)\n", prefix, connector, node.Name, node.Path, node.StartLine, node.EndLine)
	for i, child := range node.Children {
		renderNode(b, child, nextPrefix, i == len(node.Children)-1)
	}
}
