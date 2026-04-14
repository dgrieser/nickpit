package retrieval

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dgrieser/nickpit/internal/retrieval/fallback"
	"github.com/dgrieser/nickpit/internal/retrieval/goparser"
)

func (e *LocalEngine) GetSymbol(ctx context.Context, repoRoot string, symbol string) (*SymbolInfo, error) {
	if info, err := goparser.FindSymbol(ctx, repoRoot, symbol); err == nil && info != nil {
		return &SymbolInfo{
			Name:      info.Name,
			Path:      info.Path,
			StartLine: info.StartLine,
			EndLine:   info.EndLine,
			Source:    info.Source,
			Language:  "go",
		}, nil
	}
	info, err := fallback.FindSymbol(ctx, repoRoot, symbol)
	if err != nil {
		return nil, fmt.Errorf("retrieval: finding symbol %q: %w", symbol, err)
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

func (e *LocalEngine) ExpandFunctions(ctx context.Context, repoRoot string, refs []FunctionRef, depth int) (*FunctionBundle, error) {
	out := &FunctionBundle{Functions: make([]SymbolInfo, 0, len(refs))}
	for _, ref := range refs {
		info, err := e.GetSymbol(ctx, repoRoot, ref.Name)
		if err != nil {
			return nil, err
		}
		out.Functions = append(out.Functions, *info)
	}
	return out, nil
}
