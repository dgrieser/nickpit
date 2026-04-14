package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

func ExecuteRetrievals(
	ctx context.Context,
	engine retrieval.Engine,
	repoRoot string,
	requests []model.FollowUpRequest,
) []model.SupplementalFile {
	results := make([]model.SupplementalFile, 0, len(requests))
	for _, req := range requests {
		switch req.Type {
		case "file":
			content, err := engine.GetFile(ctx, repoRoot, req.Path)
			if err != nil {
				results = append(results, retrievalError(req, err))
				continue
			}
			results = append(results, model.SupplementalFile{
				Path:     req.Path,
				Language: content.Language,
				Content:  strings.Join(content.Lines, "\n"),
				Kind:     "file",
				Reason:   req.Reason,
			})
		case "lines":
			slice, err := engine.GetFileSlice(ctx, repoRoot, req.Path, req.StartLine, req.EndLine)
			if err != nil {
				results = append(results, retrievalError(req, err))
				continue
			}
			results = append(results, model.SupplementalFile{
				Path:      req.Path,
				StartLine: slice.StartLine,
				EndLine:   slice.EndLine,
				Language:  slice.Language,
				Content:   strings.Join(slice.Lines, "\n"),
				Kind:      "lines",
				Reason:    req.Reason,
			})
		case "function":
			info, err := engine.GetSymbol(ctx, repoRoot, req.Symbol)
			if err != nil {
				results = append(results, retrievalError(req, err))
				continue
			}
			results = append(results, model.SupplementalFile{
				Path:      info.Path,
				StartLine: info.StartLine,
				EndLine:   info.EndLine,
				Language:  info.Language,
				Content:   info.Source,
				Kind:      "function",
				Reason:    req.Reason,
			})
		case "callers":
			hierarchy, err := engine.FindCallers(ctx, repoRoot, retrieval.SymbolRef{Name: req.Symbol}, reqDepth(req.Depth))
			if err != nil {
				results = append(results, retrievalError(req, err))
				continue
			}
			results = append(results, model.SupplementalFile{
				Path:    hierarchy.Root.Path,
				Content: hierarchy.Render(),
				Kind:    "callers",
				Reason:  req.Reason,
			})
		case "callees":
			hierarchy, err := engine.FindCallees(ctx, repoRoot, retrieval.SymbolRef{Name: req.Symbol}, reqDepth(req.Depth))
			if err != nil {
				results = append(results, retrievalError(req, err))
				continue
			}
			results = append(results, model.SupplementalFile{
				Path:    hierarchy.Root.Path,
				Content: hierarchy.Render(),
				Kind:    "callees",
				Reason:  req.Reason,
			})
		}
	}
	return results
}

func retrievalError(req model.FollowUpRequest, err error) model.SupplementalFile {
	target := req.Path
	if target == "" {
		target = req.Symbol
	}
	return model.SupplementalFile{
		Path:    target,
		Kind:    req.Type,
		Content: fmt.Sprintf("retrieval error: %v", err),
		Reason:  req.Reason,
	}
}

func reqDepth(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}
