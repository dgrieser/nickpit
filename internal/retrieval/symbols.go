package retrieval

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dgrieser/nickpit/internal/retrieval/fallback"
	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
)

func (e *LocalEngine) GetSymbol(ctx context.Context, repoRoot string, symbol SymbolRef) (*SymbolInfo, error) {
	if info, err := goparser.FindSymbol(ctx, repoRoot, symbol.Name, symbol.Path); err == nil && info != nil {
		return &SymbolInfo{
			Name:      info.Name,
			Path:      info.Path,
			StartLine: info.StartLine,
			EndLine:   info.EndLine,
			Source:    info.Source,
			Language:  "go",
		}, nil
	}
	info, err := fallback.FindSymbol(ctx, repoRoot, symbol.Name, symbol.Path)
	if err != nil {
		return nil, fmt.Errorf("retrieval: finding symbol %q in %q: %w", symbol.Name, symbol.Path, err)
	}
	info.Path = filepath.ToSlash(info.Path)
	return &SymbolInfo{
		Name:      info.Name,
		Path:      info.Path,
		StartLine: info.StartLine,
		EndLine:   info.EndLine,
		Source:    info.Source,
		Language:  info.Language,
	}, nil
}
