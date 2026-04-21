package retrieval

import (
	"context"
	"fmt"
)

func (e *LocalEngine) GetSymbol(ctx context.Context, repoRoot string, symbol SymbolRef) (*SymbolInfo, error) {
	resolved, err := resolveSymbol(ctx, repoRoot, symbol)
	if err != nil {
		return nil, fmt.Errorf("retrieval: finding symbol %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	return resolved.info, nil
}
