package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
)

func (e *LocalEngine) FindCallers(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error) {
	resolved, err := resolveSymbol(ctx, repoRoot, symbol)
	if err != nil {
		return nil, fmt.Errorf("finding callers for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	scope, err := resolveLookupScope(repoRoot, symbol.Path)
	if err != nil {
		return nil, fmt.Errorf("finding callers for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	hierarchy, err := resolved.backend.findCallers(ctx, repoRoot, resolved.info, scope, depth)
	if err != nil {
		return nil, fmt.Errorf("finding callers for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	return hierarchy, nil
}

func (e *LocalEngine) FindCallees(ctx context.Context, repoRoot string, symbol SymbolRef, depth int) (*CallHierarchy, error) {
	resolved, err := resolveSymbol(ctx, repoRoot, symbol)
	if err != nil {
		return nil, fmt.Errorf("finding callees for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	scope, err := resolveLookupScope(repoRoot, symbol.Path)
	if err != nil {
		return nil, fmt.Errorf("finding callees for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	hierarchy, err := resolved.backend.findCallees(ctx, repoRoot, resolved.info, scope, depth)
	if err != nil {
		return nil, fmt.Errorf("finding callees for %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	return hierarchy, nil
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
		Source:    src.Source,
		Children:  []CallNode{},
	}
	for _, child := range src.Children {
		node.Children = append(node.Children, convertNode(child))
	}
	return node
}

func (h *CallHierarchy) RenderJSON() string {
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return h.Render()
	}
	return string(b)
}

func (h *CallHierarchy) Render() string {
	var b strings.Builder
	renderNode(&b, h.Root, "", true, true)
	return strings.TrimRight(b.String(), "\n")
}

func renderNode(b *strings.Builder, node CallNode, prefix string, last, root bool) {
	connector := "├── "
	nextPrefix := prefix + "│   "
	if root {
		connector = ""
		nextPrefix = ""
	} else if last {
		connector = "└── "
		nextPrefix = prefix + "    "
	}
	fmt.Fprintf(b, "%s%s%s (%s:%d-%d)\n", prefix, connector, node.Name, node.Path, node.StartLine, node.EndLine)
	if node.Source != "" {
		sourcePrefix := nextPrefix
		separatorPrefix := nextPrefix
		if root {
			sourcePrefix = ""
			separatorPrefix = ""
		}
		renderSource(b, node.Source, sourcePrefix, separatorPrefix, len(node.Children) > 0 || !last)
	}
	for i, child := range node.Children {
		renderNode(b, child, nextPrefix, i == len(node.Children)-1, false)
	}
}

func renderSource(b *strings.Builder, source, prefix, separatorPrefix string, addSeparator bool) {
	lines := strings.Split(source, "\n")
	for _, line := range lines {
		normalized := strings.ReplaceAll(line, "\t", "    ")
		fmt.Fprintf(b, "%s%s\n", prefix, normalized)
	}
	if addSeparator {
		fmt.Fprintf(b, "%s\n", separatorPrefix)
	}
}
