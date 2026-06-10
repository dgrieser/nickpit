package review

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	toolcatalog "github.com/dgrieser/nickpit/internal/tools"
)

func (e *Engine) executeToolCalls(ctx context.Context, repoRoot string, toolCalls []llm.ToolCall, state *toolRoundState) []llm.Message {
	if len(toolCalls) == 0 {
		return nil
	}
	results := make([]llm.Message, len(toolCalls))
	groups := make(map[string][]int, len(toolCalls))
	groupOrder := make([]string, 0, len(toolCalls))
	for i, toolCall := range toolCalls {
		key := toolCallConcurrencyKey(toolCall, i)
		if _, ok := groups[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], i)
	}
	var wg sync.WaitGroup
	wg.Add(len(groupOrder))
	for _, key := range groupOrder {
		indexes := append([]int(nil), groups[key]...)
		go func(indexes []int) {
			defer wg.Done()
			for _, i := range indexes {
				toolCall := toolCalls[i]
				result := e.executeToolCall(ctx, repoRoot, toolCall, state)
				results[i] = llm.Message{
					Role:       "tool",
					ToolCallID: toolCall.ID,
					Name:       toolCall.Name,
					Content:    result,
				}
				e.logToolCall(ctx, toolCall, result)
			}
		}(indexes)
	}
	wg.Wait()
	return results
}

func toolCallConcurrencyKey(toolCall llm.ToolCall, index int) string {
	uniqueKey := fmt.Sprintf("unique\x00%d\x00%s", index, toolCall.ID)
	switch toolCall.Name {
	case "inspect_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		return fmt.Sprintf("inspect_file\x00%s", normalizeToolPath(args.Path))
	case "list_files":
		var args struct {
			Path  string `json:"path"`
			Depth int    `json:"depth"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		if args.Depth <= 0 {
			args.Depth = 1
		}
		return fmt.Sprintf("list_files\x00%s\x00%d", normalizeToolPath(args.Path), args.Depth)
	case "find_callers", "find_callees":
		var args struct {
			Path   string `json:"path"`
			Symbol string `json:"symbol"`
			Depth  int    `json:"depth"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		if args.Depth <= 0 {
			args.Depth = defaultCallHierarchyDepth
		}
		return callHierarchyDedupKey(toolCall.Name, normalizeToolPath(args.Path), strings.TrimSpace(args.Symbol), args.Depth)
	case "search":
		var args struct {
			Path  string `json:"path"`
			Query string `json:"query"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		query := strings.TrimSpace(args.Query)
		if matches := searchFunctionQueryPattern.FindStringSubmatch(query); len(matches) == 2 {
			return callHierarchyDedupKey("find_callers", normalizeToolPath(args.Path), matches[1], defaultCallHierarchyDepth)
		}
		return uniqueKey
	default:
		return uniqueKey
	}
}

func (e *Engine) executeToolCall(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	if e.retrieval == nil {
		return toolError("", "retrieval_unavailable", toolErrorMessage(toolErrorData{Code: "retrieval_unavailable"}))
	}
	switch toolCall.Name {
	case "inspect_file":
		return e.executeInspectFile(ctx, repoRoot, toolCall, state)
	case "list_files":
		return e.executeListFiles(ctx, repoRoot, toolCall, state)
	case "search":
		return e.executeSearch(ctx, repoRoot, toolCall, state)
	case "find_callers":
		return e.executeCallHierarchy(ctx, repoRoot, toolCall, true, state)
	case "find_callees":
		return e.executeCallHierarchy(ctx, repoRoot, toolCall, false, state)
	default:
		return toolError("", "unsupported_tool", toolErrorMessage(toolErrorData{Code: "unsupported_tool", ToolName: toolCall.Name}))
	}
}

func (e *Engine) executeInspectFile(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {

	var args struct {
		Path      string `json:"path"`
		LineStart int    `json:"line_start"`
		LineEnd   int    `json:"line_end"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Path = strings.TrimSpace(args.Path)
	if args.Path == "" {
		return toolError("", "missing_argument", missingToolArgumentMessage(toolCall.Name, "path"))
	}
	normalizedPath := normalizeToolPath(args.Path)
	unlock := state.fileLocks.lock(normalizedPath)
	defer unlock()
	state.mu.Lock()
	seenContent, ok := state.seenFiles[normalizedPath]
	state.mu.Unlock()
	if ok {
		e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s", toolCall.Name, normalizedPath)
		return toolError(seenContent.Path, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_file"}))
	}

	if args.LineStart > 0 || args.LineEnd > 0 {
		e.logfCtx(ctx, "Executing tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, args.LineStart, args.LineEnd)
		content, err := e.retrieval.GetFileSlice(ctx, repoRoot, normalizedPath, args.LineStart, args.LineEnd)
		if err != nil {
			return toolError(normalizedPath, "retrieval_failed", err.Error())
		}
		requested := model.LineRange{Start: content.StartLine, End: content.EndLine}
		state.mu.Lock()
		covered := rangeAlreadyCovered(state.seenFileRanges[normalizedPath], requested)
		if !covered {
			state.seenFileRanges[normalizedPath] = append(state.seenFileRanges[normalizedPath], requested)
		}
		state.mu.Unlock()
		if covered {
			e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, requested.Start, requested.End)
			return toolError(content.Path, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_file"}))
		}
		return mustToolResultJSON(map[string]any{
			"path":       content.Path,
			"start_line": content.StartLine,
			"end_line":   content.EndLine,
			"language":   content.Language,
			"content":    content.Content,
		})
	}

	e.logfCtx(ctx, "Executing tool call: name=%s path=%s", toolCall.Name, normalizedPath)
	content, err := e.retrieval.GetFile(ctx, repoRoot, normalizedPath)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	result := map[string]any{
		"path":     content.Path,
		"language": content.Language,
		"content":  content.Content,
	}
	if content.Truncated {
		result["truncated"] = true
		result["truncated_note"] = "file was too large and was truncated; request specific line ranges for the remainder"
	}
	payload := mustToolResultJSON(result)
	state.mu.Lock()
	state.seenFiles[normalizedPath] = *content
	state.mu.Unlock()
	return payload
}

func (e *Engine) executeListFiles(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Path = strings.TrimSpace(args.Path)
	if args.Depth <= 0 {
		args.Depth = 1
	}
	normalizedPath := normalizeToolPath(args.Path)
	key := fmt.Sprintf("list_files\x00%s\x00%d", normalizedPath, args.Depth)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, ok := state.seenToolCalls[key]
	state.mu.Unlock()
	if ok {
		e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logfCtx(ctx, "Executing tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
	listing, err := e.retrieval.ListFiles(ctx, repoRoot, normalizedPath, args.Depth)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(map[string]any{
		"path":  listing.Path,
		"depth": args.Depth,
		"files": listing.Files,
	})
}

func (e *Engine) executeSearch(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args struct {
		Path          string `json:"path"`
		Query         string `json:"query"`
		ContextLines  int    `json:"context_lines"`
		MaxResults    int    `json:"max_results"`
		CaseSensitive bool   `json:"case_sensitive"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Path = strings.TrimSpace(args.Path)
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return toolError(normalizeToolPath(args.Path), "missing_argument", missingToolArgumentMessage(toolCall.Name, "query"))
	}
	if args.ContextLines < 0 {
		args.ContextLines = defaultSearchContextLines
	}
	normalizedPath := normalizeToolPath(args.Path)
	if e.searchToolOptimization {
		if matches := searchFunctionQueryPattern.FindStringSubmatch(args.Query); len(matches) == 2 {
			symbol := matches[1]
			key := callHierarchyDedupKey("find_callers", normalizedPath, symbol, defaultCallHierarchyDepth)
			state.mu.Lock()
			_, ok := state.seenToolCalls[key]
			state.mu.Unlock()
			if ok {
				e.logfCtx(ctx, "Skipping duplicate optimized tool call: name=%s path=%s query=%q rewritten=find_callers symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Query, symbol, defaultCallHierarchyDepth)
				return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
			}
			e.logfCtx(ctx, "Rewriting tool call: name=%s path=%s query=%q rewritten=find_callers symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Query, symbol, defaultCallHierarchyDepth)
			return e.executeCallHierarchy(ctx, repoRoot, llm.ToolCall{
				ID:   toolCall.ID,
				Name: "find_callers",
				Arguments: mustToolResultJSON(map[string]any{
					"path":   normalizedPath,
					"symbol": symbol,
					"depth":  defaultCallHierarchyDepth,
				}),
			}, true, state)
		}
	}
	e.logfCtx(ctx, "Executing tool call: name=%s path=%s query=%q context_lines=%d max_results=%d case_sensitive=%t", toolCall.Name, normalizedPath, args.Query, args.ContextLines, args.MaxResults, args.CaseSensitive)
	results, err := e.retrieval.Search(ctx, repoRoot, normalizedPath, args.Query, args.ContextLines, args.MaxResults, args.CaseSensitive)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}

	if hasRegexMetachar(args.Query) {
		regexPattern := args.Query
		if !args.CaseSensitive {
			regexPattern = "(?i)" + regexPattern
		}
		if compiled, compileErr := regexp.Compile(regexPattern); compileErr == nil {
			e.logfCtx(ctx, "Executing regex search: name=%s path=%s pattern=%q context_lines=%d max_results=%d", toolCall.Name, normalizedPath, compiled.String(), args.ContextLines, args.MaxResults)
			regexResults, err := e.retrieval.SearchRegex(ctx, repoRoot, normalizedPath, compiled, args.ContextLines, args.MaxResults)
			if err != nil {
				return toolError(normalizedPath, "retrieval_failed", err.Error())
			}
			merged := mergeSearchResults(results.Results, regexResults.Results, args.MaxResults)
			results.Results = merged
			results.ResultCount = len(merged)
		} else {
			e.logfCtx(ctx, "Skipping regex search: name=%s path=%s pattern=%q error=%v", toolCall.Name, normalizedPath, regexPattern, compileErr)
		}
	}

	return mustToolResultJSON(map[string]any{
		"path":           results.Path,
		"query":          results.Query,
		"context_lines":  results.ContextLines,
		"max_results":    results.MaxResults,
		"case_sensitive": results.CaseSensitive,
		"result_count":   results.ResultCount,
		"results":        results.Results,
	})
}

func hasRegexMetachar(query string) bool {
	return strings.ContainsAny(query, `\.+*?()|[]{}^$`)
}

func mergeSearchResults(literal, regex []retrieval.SearchResult, maxResults int) []retrieval.SearchResult {
	merged := make([]retrieval.SearchResult, 0, len(literal)+len(regex))
	seen := make(map[string]struct{}, len(literal)+len(regex))
	key := func(r retrieval.SearchResult) string {
		return fmt.Sprintf("%s:%d:%d", r.Path, r.StartLine, r.EndLine)
	}
	for _, r := range literal {
		k := key(r)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		merged = append(merged, r)
	}
	for _, r := range regex {
		k := key(r)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		merged = append(merged, r)
	}
	if maxResults > 0 && len(merged) > maxResults {
		merged = merged[:maxResults]
	}
	return merged
}

func (e *Engine) executeCallHierarchy(ctx context.Context, repoRoot string, toolCall llm.ToolCall, callers bool, state *toolRoundState) string {
	var args struct {
		Symbol string `json:"symbol"`
		Path   string `json:"path"`
		Depth  int    `json:"depth"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Symbol = strings.TrimSpace(args.Symbol)
	args.Path = strings.TrimSpace(args.Path)
	if args.Symbol == "" {
		return toolError(normalizeToolPath(args.Path), "missing_argument", missingToolArgumentMessage(toolCall.Name, "symbol"))
	}
	if args.Depth <= 0 {
		args.Depth = defaultCallHierarchyDepth
	}
	normalizedPath := normalizeToolPath(args.Path)
	key := callHierarchyDedupKey(toolCall.Name, normalizedPath, args.Symbol, args.Depth)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, ok := state.seenToolCalls[key]
	state.mu.Unlock()
	if ok {
		e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logfCtx(ctx, "Executing tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)

	symbol := retrieval.SymbolRef{Name: args.Symbol, Path: normalizedPath}
	var (
		hierarchy *retrieval.CallHierarchy
		err       error
	)
	if callers {
		hierarchy, err = e.retrieval.FindCallers(ctx, repoRoot, symbol, args.Depth)
	} else {
		hierarchy, err = e.retrieval.FindCallees(ctx, repoRoot, symbol, args.Depth)
	}
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(map[string]any{
		"symbol": args.Symbol,
		"path":   normalizedPath,
		"mode":   hierarchy.Mode,
		"depth":  hierarchy.Depth,
		"root":   hierarchy.Root,
	})
}

func callHierarchyDedupKey(name, path, symbol string, depth int) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%d", name, path, symbol, depth)
}

func mustToolResultJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return mustToolResultJSON(map[string]any{
			"status": "error",
			"error": map[string]any{
				"code":    "encoding_failed",
				"message": toolErrorMessage(toolErrorData{Code: "encoding_failed"}),
			},
		})
	}
	return string(data)
}

type toolErrorData = toolcatalog.ErrorData

func toolErrorMessage(data toolErrorData) string {
	return toolcatalog.ErrorMessage(data)
}

func toolArgumentSchema(name string) string {
	return toolcatalog.ArgumentSchema(name)
}

func missingToolArgumentMessage(toolName, argument string) string {
	return toolErrorMessage(toolErrorData{
		Code:     "missing_argument",
		Argument: argument,
		Schema:   toolArgumentSchema(toolName),
	})
}

func parseToolArguments(toolName string, raw string, dst any) error {
	if err := llm.LenientUnmarshal(raw, dst); err != nil {
		schema := toolArgumentSchema(toolName)
		if schema == "" {
			return fmt.Errorf("invalid tool arguments for %s: %v; received: %s", toolName, err, raw)
		}
		return fmt.Errorf("invalid tool arguments for %s: %v; expected %s; received: %s", toolName, err, schema, raw)
	}
	return nil
}

func toolError(path, code, message string) string {
	payload := map[string]any{
		"status": "error",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if path != "" {
		payload["path"] = path
	}
	return mustToolResultJSON(payload)
}
