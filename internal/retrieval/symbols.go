package retrieval

import (
	"context"
	"fmt"

	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
)

func (e *LocalEngine) GetSymbol(ctx context.Context, repoRoot string, symbol SymbolRef) (*SymbolInfo, error) {
	info, err := goparser.FindSymbol(ctx, repoRoot, symbol.Name, symbol.Path)
	if err != nil {
		return nil, fmt.Errorf("retrieval: finding symbol %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	return &SymbolInfo{
		Name:      info.Name,
		Path:      info.Path,
		StartLine: info.StartLine,
		EndLine:   info.EndLine,
		Source:    info.Source,
		Language:  "go",
	}, nil
}
