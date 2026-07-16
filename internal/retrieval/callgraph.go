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
		Name:         src.Name,
		CodeLocation: callNodeLocation(src.Path, src.StartLine, src.EndLine, src.Source),
		Children:     []CallNode{},
	}
	for _, child := range src.Children {
		node.Children = append(node.Children, convertNode(child))
	}
	return node
}

// callNodeLocation builds the canonical code_location for a call-hierarchy
// node from its path, line range, and source text.
func callNodeLocation(path string, startLine, endLine int, source string) CodeLocation {
	count := 0
	if startLine > 0 && endLine >= startLine {
		count = endLine - startLine + 1
	}
	return CodeLocation{
		FilePath: path,
		LineRange: LineRange{
			Start: startLine,
			End:   endLine,
			Count: count,
		},
		Language: detectLanguage(path),
		Content:  source,
	}
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
	loc := node.CodeLocation
	fmt.Fprintf(b, "%s%s%s (%s:%d-%d)\n", prefix, connector, node.Name, loc.FilePath, loc.LineRange.Start, loc.LineRange.End)
	if loc.Content != "" {
		sourcePrefix := nextPrefix
		separatorPrefix := nextPrefix
		if root {
			sourcePrefix = ""
			separatorPrefix = ""
		}
		renderSource(b, loc.Content, sourcePrefix, separatorPrefix, len(node.Children) > 0 || !last)
	}
	for i, child := range node.Children {
		renderNode(b, child, nextPrefix, i == len(node.Children)-1, false)
	}
}

func renderSource(b *strings.Builder, source, prefix, separatorPrefix string, addSeparator bool) {
	lines := strings.SplitSeq(source, "\n")
	for line := range lines {
		normalized := strings.ReplaceAll(line, "\t", "    ")
		fmt.Fprintf(b, "%s%s\n", prefix, normalized)
	}
	if addSeparator {
		fmt.Fprintf(b, "%s\n", separatorPrefix)
	}
}
