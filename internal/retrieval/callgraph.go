package retrieval

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/fallback"
	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
)

func (e *LocalEngine) FindCallers(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error) {
	graph, err := goparser.BuildGraph(ctx, repoRoot)
	if err == nil && graph != nil {
		hierarchy, findErr := graph.Find(symbol.Name, symbol.Path, depth, true)
		if findErr == nil {
			return convertHierarchy(hierarchy), nil
		}
	}
	info, err := fallback.FindSymbol(ctx, repoRoot, symbol.Name, symbol.Path)
	if err != nil {
		return nil, err
	}
	return &CallHierarchy{
		Root: CallNode{
			Name:      info.Name,
			Path:      info.Path,
			StartLine: info.StartLine,
			EndLine:   info.EndLine,
		},
		Mode:  "callers",
		Depth: depth,
	}, nil
}

func (e *LocalEngine) FindCallees(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error) {
	graph, err := goparser.BuildGraph(ctx, repoRoot)
	if err == nil && graph != nil {
		hierarchy, findErr := graph.Find(symbol.Name, symbol.Path, depth, false)
		if findErr == nil {
			return convertHierarchy(hierarchy), nil
		}
	}
	info, err := fallback.FindSymbol(ctx, repoRoot, symbol.Name, symbol.Path)
	if err != nil {
		return nil, err
	}
	return &CallHierarchy{
		Root: CallNode{
			Name:      info.Name,
			Path:      info.Path,
			StartLine: info.StartLine,
			EndLine:   info.EndLine,
		},
		Mode:  "callees",
		Depth: depth,
	}, nil
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
