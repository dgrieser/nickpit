package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/git"
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
		key := e.toolCallConcurrencyKey(toolCall, i)
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

func (e *Engine) toolCallConcurrencyKey(toolCall llm.ToolCall, index int) string {
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
			args.Depth = toolcatalog.DefaultListFilesDepth
		}
		return fmt.Sprintf("list_files\x00%s\x00%d", normalizeToolPath(args.Path), args.Depth)
	case "find_callers", "find_callees", "find_references":
		var args struct {
			Path   string `json:"path"`
			Symbol string `json:"symbol"`
			Depth  int    `json:"depth"`
			Line   int    `json:"line"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		if toolCall.Name != "find_references" && args.Depth <= 0 {
			args.Depth = toolcatalog.DefaultCallHierarchyDepth
		}
		if toolCall.Name == "find_references" {
			return referenceDedupKey(normalizeSearchPath(args.Path), strings.TrimSpace(args.Symbol), args.Line)
		}
		return callHierarchyDedupKey(toolCall.Name, normalizeSearchPath(args.Path), strings.TrimSpace(args.Symbol), args.Depth)
	case "search":
		var args searchToolArgs
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		query := retrieval.NormalizeSearchQuery(args.Query)
		if query == "" {
			// Executes as a missing_argument error without dedup state.
			return uniqueKey
		}
		normalizedPath := normalizeSearchPath(args.Path)
		return searchDedupKey(normalizedPath, query, resolveSearchContextLines(args.ContextLines, query), args.MaxResults, args.CaseSensitive)
	case "git_log":
		var args gitLogToolArgs
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		return gitLogDedupKey(args.options())
	case "git_show":
		var args gitShowToolArgs
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		if strings.TrimSpace(args.Commit) == "" {
			// Executes as a missing_argument error without dedup state.
			return uniqueKey
		}
		return gitShowDedupKey(args.options(e.diffFormat()))
	default:
		return uniqueKey
	}
}

func (e *Engine) executeToolCall(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	if e.retrieval == nil {
		return limitToolResultJSON(toolError("", "retrieval_unavailable", toolErrorMessage(toolErrorData{Code: "retrieval_unavailable"})), 0)
	}
	var result string
	switch toolCall.Name {
	case "inspect_file":
		result = e.executeInspectFile(ctx, repoRoot, toolCall, state)
	case "list_files":
		result = e.executeListFiles(ctx, repoRoot, toolCall, state)
	case "search":
		result = e.executeSearch(ctx, repoRoot, toolCall, state)
	case "find_callers":
		result = e.executeCallHierarchy(ctx, repoRoot, toolCall, true, state)
	case "find_references":
		result = e.executeFindReferences(ctx, repoRoot, toolCall, state)
	case "find_callees":
		result = e.executeCallHierarchy(ctx, repoRoot, toolCall, false, state)
	case "git_log":
		result = e.executeGitLog(ctx, repoRoot, toolCall, state)
	case "git_show":
		result = e.executeGitShow(ctx, repoRoot, toolCall, state)
	default:
		result = toolError("", "unsupported_tool", toolErrorMessage(toolErrorData{Code: "unsupported_tool", ToolName: toolCall.Name}))
	}
	// Apply tool-specific item limits here. Context-aware token capping happens
	// after the parallel batch completes, when the agent loop knows current
	// prompt usage and can allocate results against one shared remainder.
	return limitToolResultJSON(result, 0)
}

func (e *Engine) executeFindReferences(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args struct {
		Symbol string `json:"symbol"`
		Path   string `json:"path"`
		Line   int    `json:"line"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Symbol = strings.TrimSpace(args.Symbol)
	normalizedPath := normalizeSearchPath(args.Path)
	if args.Symbol == "" {
		return toolError(normalizedPath, "missing_argument", missingToolArgumentMessage(toolCall.Name, "symbol"))
	}
	key := referenceDedupKey(normalizedPath, args.Symbol, args.Line)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, seen := state.seenToolCalls[key]
	state.mu.Unlock()
	if seen {
		e.logf(ctx, "Skipping duplicate tool call: name=%s path=%s symbol=%q", toolCall.Name, normalizedPath, args.Symbol)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logf(ctx, "Executing tool call: name=%s path=%s symbol=%q", toolCall.Name, normalizedPath, args.Symbol)
	result, err := e.retrieval.FindReferences(ctx, repoRoot, retrieval.SymbolRef{Name: args.Symbol, Path: normalizedPath, Line: args.Line})
	if err != nil {
		var (
			unsupported *retrieval.UnsupportedLanguageError
			notFound    *retrieval.SymbolNotFoundError
		)
		// Both degrade to a literal search, but they mean different things and
		// the model must not be told the file type is unsupported when the
		// analysis ran and simply found no declaration.
		note := ""
		switch {
		case errors.As(err, &unsupported):
			note = "structural reference analysis is unavailable for this file type; showing case-sensitive literal matches instead"
		case errors.As(err, &notFound):
			note = "structural analysis found no declaration of this symbol; showing case-sensitive literal matches instead, which may be empty or defined in a file type without a structural backend"
		}
		if note != "" {
			searchScope := retrieval.FallbackSearchScope(repoRoot, normalizedPath)
			// Identifier spelling is case-sensitive even when the fallback cannot
			// provide structural binding identity.
			matches, searchErr := e.retrieval.Search(ctx, repoRoot, searchScope, args.Symbol, toolcatalog.DefaultSearchContextLines, 0, true)
			if searchErr != nil {
				return toolError(normalizedPath, "retrieval_failed", searchErr.Error())
			}
			state.mu.Lock()
			state.seenToolCalls[key] = struct{}{}
			state.mu.Unlock()
			return mustToolResultJSON(withTruncatedFiles(map[string]any{
				"symbol": args.Symbol, "path": normalizedPath, "fallback": "search",
				"note":         note,
				"result_count": matches.ResultCount, "results": matches.Results,
			}, matches.TruncatedFiles))
		}
		var ambiguous *retrieval.AmbiguousSymbolError
		if errors.As(err, &ambiguous) {
			return toolError(normalizedPath, "ambiguous_symbol", err.Error())
		}
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(result)
}

// referenceDedupKey includes the declaration line so that retrying an ambiguous
// symbol pinned to one declaration is a new call, not a duplicate of the
// unpinned one that failed.
func referenceDedupKey(path, symbol string, line int) string {
	return fmt.Sprintf("find_references\x00%s\x00%s\x00%d", path, symbol, line)
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
	rangedRequest := args.LineStart > 0 || args.LineEnd > 0
	state.mu.Lock()
	seenContent, ok := state.seenFiles[normalizedPath]
	state.mu.Unlock()
	// A stored full-file read dedupes repeated full-file requests always, but
	// only blocks ranged follow-ups when the stored content was complete: a
	// truncated read does not cover ranges past the cap, and its result note
	// explicitly tells the model to request specific line ranges.
	if ok && (!rangedRequest || !seenContent.Truncated) {
		e.logf(ctx, "Skipping duplicate tool call: name=%s path=%s", toolCall.Name, normalizedPath)
		return toolError(seenContent.Path, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_file"}))
	}

	if rangedRequest {
		e.logf(ctx, "Executing tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, args.LineStart, args.LineEnd)
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
			e.logf(ctx, "Skipping duplicate tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, requested.Start, requested.End)
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

	e.logf(ctx, "Executing tool call: name=%s path=%s", toolCall.Name, normalizedPath)
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
		args.Depth = toolcatalog.DefaultListFilesDepth
	}
	normalizedPath := normalizeToolPath(args.Path)
	key := fmt.Sprintf("list_files\x00%s\x00%d", normalizedPath, args.Depth)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, ok := state.seenToolCalls[key]
	state.mu.Unlock()
	if ok {
		e.logf(ctx, "Skipping duplicate tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logf(ctx, "Executing tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
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

// searchToolArgs are the LLM-supplied arguments of the search tool.
// ContextLines is a pointer so an omitted value is distinguishable from an
// explicit 0: the default depends on the query (5 for single-line, 0 for
// multi-line blocks).
type searchToolArgs struct {
	Path          string `json:"path"`
	Query         string `json:"query"`
	ContextLines  *int   `json:"context_lines"`
	MaxResults    int    `json:"max_results"`
	CaseSensitive bool   `json:"case_sensitive"`
}

// SearchOptions configures a standalone search using the same execution path
// as an LLM search tool call.
type SearchOptions struct {
	Path          string
	Query         string
	ContextLines  int
	MaxResults    int
	CaseSensitive bool
}

// MixedSearchResult combines one resolved structural result with literal
// matches that the structural backends could not classify.
type MixedSearchResult struct {
	Callers        *retrieval.CallHierarchy
	References     *retrieval.ReferenceResult
	LiteralResults []retrieval.SearchResult
	TruncatedFiles []string
}

// GroupedSearchResult contains structural enrichments for multiple distinct
// declarations found among one search's returned matches.
type GroupedSearchResult struct {
	StructuralResultCount int                          `json:"structural_result_count"`
	CallHierarchies       []*retrieval.CallHierarchy   `json:"call_hierarchies,omitempty"`
	ReferenceResults      []*retrieval.ReferenceResult `json:"reference_results,omitempty"`
	LiteralResultCount    int                          `json:"literal_result_count"`
	LiteralResults        []retrieval.SearchResult     `json:"literal_results"`
	TruncatedFiles        []string                     `json:"truncated_files,omitempty"`
}

// MarshalJSON keeps the structural result's normal top-level shape and adds
// the preserved literal matches alongside it.
func (r MixedSearchResult) MarshalJSON() ([]byte, error) {
	var primary any = r.Callers
	if r.References != nil {
		primary = r.References
	}
	encoded, err := json.Marshal(primary)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		// Neither structural result was set, so the primary encoded to "null".
		// The literal matches are still worth returning on their own.
		payload = map[string]any{}
	}
	payload["literal_result_count"] = len(r.LiteralResults)
	payload["literal_results"] = r.LiteralResults
	if len(r.TruncatedFiles) > 0 {
		payload["truncated_files"] = r.TruncatedFiles
	}
	return json.Marshal(payload)
}

// ExecuteSearch runs search plus its optional structural result replacement.
// It returns the concrete retrieval result type so non-LLM callers can render
// literal matches, references, and caller hierarchies normally.
func (e *Engine) ExecuteSearch(ctx context.Context, repoRoot string, options SearchOptions) (any, error) {
	arguments := mustToolResultJSON(searchToolArgs{
		Path:          options.Path,
		Query:         options.Query,
		ContextLines:  &options.ContextLines,
		MaxResults:    options.MaxResults,
		CaseSensitive: options.CaseSensitive,
	})
	state := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}
	raw := e.executeSearch(ctx, repoRoot, llm.ToolCall{ID: "standalone-search", Name: "search", Arguments: arguments}, state)
	var envelope searchResultEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decode search result: %w", err)
	}
	if envelope.Status == "error" {
		return nil, errors.New(envelope.Error.Message)
	}
	switch {
	case envelope.CallHierarchies != nil || envelope.ReferenceResults != nil:
		grouped := &GroupedSearchResult{}
		if err := json.Unmarshal([]byte(raw), grouped); err != nil {
			return nil, fmt.Errorf("decode grouped search result: %w", err)
		}
		return grouped, nil
	case envelope.Root != nil:
		callers := &retrieval.CallHierarchy{}
		if err := json.Unmarshal([]byte(raw), callers); err != nil {
			return nil, fmt.Errorf("decode caller hierarchy: %w", err)
		}
		return envelope.withLiteralMatches(&MixedSearchResult{Callers: callers}), nil
	case envelope.Target != nil:
		references := &retrieval.ReferenceResult{}
		if err := json.Unmarshal([]byte(raw), references); err != nil {
			return nil, fmt.Errorf("decode reference result: %w", err)
		}
		return envelope.withLiteralMatches(&MixedSearchResult{References: references}), nil
	default:
		results := &retrieval.SearchResults{}
		if err := json.Unmarshal([]byte(raw), results); err != nil {
			return nil, fmt.Errorf("decode search result: %w", err)
		}
		return results, nil
	}
}

// searchResultEnvelope identifies which shape executeSearch returned. The
// search tool answers with an untagged union — literal matches, one structural
// result, or several grouped together — so the discriminating fields are
// decoded once here instead of re-decoding the whole document per guess.
type searchResultEnvelope struct {
	Status string `json:"status"`
	Error  struct {
		Message string `json:"message"`
	} `json:"error"`
	Root             json.RawMessage          `json:"root"`
	Target           json.RawMessage          `json:"target"`
	CallHierarchies  json.RawMessage          `json:"call_hierarchies"`
	ReferenceResults json.RawMessage          `json:"reference_results"`
	LiteralResults   []retrieval.SearchResult `json:"literal_results"`
	TruncatedFiles   []string                 `json:"truncated_files"`
}

// withLiteralMatches returns the structural result on its own unless literal
// matches were preserved next to it.
func (envelope searchResultEnvelope) withLiteralMatches(mixed *MixedSearchResult) any {
	if len(envelope.LiteralResults) == 0 && len(envelope.TruncatedFiles) == 0 {
		if mixed.Callers != nil {
			return mixed.Callers
		}
		return mixed.References
	}
	mixed.LiteralResults = envelope.LiteralResults
	mixed.TruncatedFiles = envelope.TruncatedFiles
	return mixed
}

// resolveSearchContextLines applies the per-mode default when the model omits
// context_lines or passes a negative value. The query must already be
// normalized.
func resolveSearchContextLines(contextLines *int, query string) int {
	if contextLines == nil || *contextLines < 0 {
		return retrieval.DefaultSearchContextLines(query)
	}
	return *contextLines
}

// normalizeSearchPath canonicalizes an optional scope path: "." is the same
// scope as an omitted path (the repo root), so both dedupe to one key. It is
// shared by search and the structural tools so their path scopes compare
// consistently when a literal result is replaced by callers or references.
func normalizeSearchPath(path string) string {
	normalized := normalizeToolPath(strings.TrimSpace(path))
	if normalized == "." {
		return ""
	}
	return normalized
}

func (e *Engine) executeSearch(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args searchToolArgs
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Query = retrieval.NormalizeSearchQuery(args.Query)
	normalizedPath := normalizeSearchPath(args.Path)
	if args.Query == "" {
		return toolError(normalizedPath, "missing_argument", missingToolArgumentMessage(toolCall.Name, "query"))
	}
	contextLines := resolveSearchContextLines(args.ContextLines, args.Query)
	multiLine := retrieval.FindLinesCount(args.Query) > 1
	key := searchDedupKey(normalizedPath, args.Query, contextLines, args.MaxResults, args.CaseSensitive)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, seen := state.seenToolCalls[key]
	state.mu.Unlock()
	if seen {
		e.logf(ctx, "Skipping duplicate tool call: name=%s path=%s query=%q", toolCall.Name, normalizedPath, args.Query)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logf(ctx, "Executing tool call: name=%s path=%s query=%q query_lines=%d context_lines=%d max_results=%d case_sensitive=%t", toolCall.Name, normalizedPath, args.Query, retrieval.FindLinesCount(args.Query), contextLines, args.MaxResults, args.CaseSensitive)
	results, err := e.retrieval.Search(ctx, repoRoot, normalizedPath, args.Query, contextLines, args.MaxResults, args.CaseSensitive)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}

	// A multi-line block is always matched exactly (lean whitespace-insensitive
	// matching); the regex interpretation only applies to single-line queries.
	if !multiLine && hasRegexMetachar(args.Query) {
		regexPattern := args.Query
		if !args.CaseSensitive {
			regexPattern = "(?i)" + regexPattern
		}
		if compiled, compileErr := regexp.Compile(regexPattern); compileErr == nil {
			e.logf(ctx, "Executing regex search: name=%s path=%s pattern=%q context_lines=%d max_results=%d", toolCall.Name, normalizedPath, compiled.String(), contextLines, args.MaxResults)
			regexResults, err := e.retrieval.SearchRegex(ctx, repoRoot, normalizedPath, compiled, contextLines, args.MaxResults)
			if err != nil {
				return toolError(normalizedPath, "retrieval_failed", err.Error())
			}
			merged := mergeSearchResults(results.Results, regexResults.Results, args.MaxResults)
			results.Results = merged
			results.ResultCount = len(merged)
			results.TruncatedFiles = mergeTruncatedFiles(results.TruncatedFiles, regexResults.TruncatedFiles)
		} else {
			e.logf(ctx, "Skipping regex search: name=%s path=%s pattern=%q error=%v", toolCall.Name, normalizedPath, regexPattern, compileErr)
		}
	}

	if e.searchToolOptimization {
		if optimized, ok := e.optimizeSearchResults(ctx, repoRoot, toolCall.ID, args.Query, results.Results, results.TruncatedFiles, state); ok {
			state.mu.Lock()
			state.seenToolCalls[key] = struct{}{}
			state.mu.Unlock()
			return optimized
		}
	}

	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(withTruncatedFiles(map[string]any{
		"path":           results.Path,
		"query":          results.Query,
		"context_lines":  results.ContextLines,
		"max_results":    results.MaxResults,
		"case_sensitive": results.CaseSensitive,
		"result_count":   results.ResultCount,
		"results":        results.Results,
	}, results.TruncatedFiles))
}

type classifiedSearchSymbol struct {
	name string
	path string
	kind string
	// declarationLine pins the declaration when the same name is declared more
	// than once in its file, so the deferred reference lookup cannot re-ambiguate.
	declarationLine int
	// references is filled only once the symbol is known to need a reference
	// analysis. A function is upgraded to a call hierarchy instead, so
	// computing its references first would be a whole-repository pass thrown
	// away.
	references *retrieval.ReferenceResult
}

// optimizeSearchResults upgrades an identifier-shaped literal search only
// after the search has run. Each matched file is asked to resolve a declaration
// for the spelling found in its returned snippet. Matches are grouped by their
// declaration and enriched with callers or references independently; unsupported
// or unresolved matches remain literal.
func (e *Engine) optimizeSearchResults(ctx context.Context, repoRoot, toolCallID, query string, results []retrieval.SearchResult, truncatedFiles []string, state *toolRoundState) (string, bool) {
	matches := searchIdentifierQueryPattern.FindStringSubmatch(query)
	if len(matches) != 2 || len(results) == 0 {
		return "", false
	}
	queryName := matches[1]
	type lookup struct{ name, path string }
	lookups := make([]lookup, 0, len(results))
	seenLookups := make(map[lookup]struct{}, len(results))
	names := map[string]struct{}{queryName: {}}
	for _, result := range results {
		path := normalizeSearchPath(result.CodeLocation.FilePath)
		spellings := matchingIdentifierSpellings(result.CodeLocation.Content, queryName)
		if len(spellings) == 0 {
			spellings = []string{queryName}
		}
		for _, name := range spellings {
			names[name] = struct{}{}
			candidate := lookup{name: name, path: path}
			if _, duplicate := seenLookups[candidate]; duplicate {
				continue
			}
			seenLookups[candidate] = struct{}{}
			lookups = append(lookups, candidate)
			if len(lookups) > toolcatalog.MaxSearchStructuralLookups {
				e.logf(ctx, "Keeping broad identifier search literal: query=%q structural_candidates>%d", query, toolcatalog.MaxSearchStructuralLookups)
				return "", false
			}
		}
	}

	resolved := make(map[string]*classifiedSearchSymbol)
	lookupCount := 0
	recordResolution := func(target *retrieval.ReferenceTarget, references *retrieval.ReferenceResult, err error) {
		if err != nil || target == nil {
			return
		}
		key := fmt.Sprintf("%s\x00%d\x00%d\x00%s", target.Definition.FilePath, target.Definition.LineRange.Start, target.Definition.LineRange.End, target.Name)
		if _, ok := resolved[key]; ok {
			return
		}
		resolved[key] = &classifiedSearchSymbol{
			name:            target.Name,
			path:            target.Definition.FilePath,
			kind:            target.Kind,
			declarationLine: target.Definition.LineRange.Start,
			references:      references,
		}
	}
	// Only the declaration is needed to decide between a call hierarchy and a
	// reference analysis, so resolve targets when the engine can do that alone.
	resolveBatch := func(candidates []lookup) {
		remaining := toolcatalog.MaxSearchStructuralLookups - lookupCount
		if remaining <= 0 || len(candidates) == 0 {
			return
		}
		candidates = candidates[:min(len(candidates), remaining)]
		lookupCount += len(candidates)
		symbols := make([]retrieval.SymbolRef, len(candidates))
		for i, candidate := range candidates {
			symbols[i] = retrieval.SymbolRef{Name: candidate.name, Path: candidate.path}
		}
		if resolver, ok := e.retrieval.(retrieval.ReferenceTargetResolver); ok {
			for _, targetResult := range resolver.ResolveReferenceTargets(ctx, repoRoot, symbols) {
				recordResolution(targetResult.Target, nil, targetResult.Err)
			}
			return
		}
		if batchEngine, ok := e.retrieval.(retrieval.ReferenceBatchEngine); ok {
			for _, batchResult := range batchEngine.FindReferencesBatch(ctx, repoRoot, symbols) {
				if batchResult.Result == nil {
					recordResolution(nil, nil, batchResult.Err)
					continue
				}
				recordResolution(&batchResult.Result.Target, batchResult.Result, batchResult.Err)
			}
			return
		}
		for _, symbol := range symbols {
			result, err := e.retrieval.FindReferences(ctx, repoRoot, symbol)
			if result == nil {
				recordResolution(nil, nil, err)
				continue
			}
			recordResolution(&result.Target, result, err)
		}
	}
	resolveBatch(lookups)
	// A path-scoped search may contain only usages. Fall back to repo-wide
	// declaration resolution after every returned match has been tried.
	if len(resolved) == 0 {
		fallbacks := make([]lookup, 0, len(names))
		for name := range names {
			fallbacks = append(fallbacks, lookup{name: name})
		}
		sort.Slice(fallbacks, func(i, j int) bool { return fallbacks[i].name < fallbacks[j].name })
		resolveBatch(fallbacks)
	}
	if len(resolved) == 0 {
		return "", false
	}

	keys := make([]string, 0, len(resolved))
	for key := range resolved {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var callHierarchies, referenceResults []json.RawMessage
	classifiedMatches := make(map[int]struct{})
	for _, key := range keys {
		selected := resolved[key]
		if selected.name == "" || selected.path == "" {
			continue
		}
		var optimized string
		if selected.kind == "function" {
			e.logf(ctx, "Replacing search matches with find_callers: query=%q symbol=%q path=%s depth=%d", query, selected.name, selected.path, toolcatalog.DefaultCallHierarchyDepth)
			optimized = e.executeCallHierarchy(ctx, repoRoot, llm.ToolCall{
				ID:   toolCallID,
				Name: "find_callers",
				Arguments: mustToolResultJSON(map[string]any{
					"path":   selected.path,
					"symbol": selected.name,
					"depth":  toolcatalog.DefaultCallHierarchyDepth,
				}),
			}, true, state)
			if toolResultIsError(optimized) {
				continue
			}
			var hierarchy retrieval.CallHierarchy
			if err := json.Unmarshal([]byte(optimized), &hierarchy); err != nil {
				continue
			}
			callHierarchies = append(callHierarchies, json.RawMessage(optimized))
			for resultIndex, result := range results {
				if callHierarchyCoversLocation(hierarchy.Root, result.CodeLocation) {
					classifiedMatches[resultIndex] = struct{}{}
				}
			}
		} else {
			if !e.reserveSearchReferenceResult(selected, state) {
				continue
			}
			if selected.references == nil {
				// Resolution stopped at the declaration; collect its references
				// now that the symbol is known not to be a function.
				references, err := e.retrieval.FindReferences(ctx, repoRoot, retrieval.SymbolRef{
					Name: selected.name, Path: selected.path, Line: selected.declarationLine,
				})
				if err != nil || references == nil {
					continue
				}
				selected.references = references
			}
			e.logf(ctx, "Replacing search matches with find_references: query=%q symbol=%q path=%s kind=%s", query, selected.name, selected.path, selected.references.Target.Kind)
			optimized = limitToolResultJSON(mustToolResultJSON(selected.references), 0)
			referenceResults = append(referenceResults, json.RawMessage(optimized))
			// Classify against every literal match, not just the ones whose
			// lookup produced this declaration: the repo-wide fallback resolves
			// without a path and would otherwise leave a duplicate of each hit.
			for resultIndex, result := range results {
				if referenceResultCoversLocation(selected.references, result.CodeLocation) {
					classifiedMatches[resultIndex] = struct{}{}
				}
			}
		}
	}
	structuralCount := len(callHierarchies) + len(referenceResults)
	if structuralCount == 0 {
		return "", false
	}
	literalResults := make([]retrieval.SearchResult, 0)
	for resultIndex, result := range results {
		if _, classified := classifiedMatches[resultIndex]; !classified {
			literalResults = append(literalResults, result)
		}
	}
	if structuralCount == 1 {
		if len(callHierarchies) == 1 {
			return withLiteralSearchResults(string(callHierarchies[0]), literalResults, truncatedFiles), true
		}
		return withLiteralSearchResults(string(referenceResults[0]), literalResults, truncatedFiles), true
	}
	payload := map[string]any{
		"structural_result_count": structuralCount,
		"literal_result_count":    len(literalResults),
		"literal_results":         literalResults,
	}
	if len(callHierarchies) > 0 {
		payload["call_hierarchies"] = callHierarchies
	}
	if len(referenceResults) > 0 {
		payload["reference_results"] = referenceResults
	}
	return mustToolResultJSON(withTruncatedFiles(payload, truncatedFiles)), true
}

func referenceResultCoversLocation(result *retrieval.ReferenceResult, location retrieval.CodeLocation) bool {
	if codeLocationsOverlap(result.Target.Definition, location) {
		return true
	}
	for _, contexts := range [][]retrieval.ReferenceContext{result.Functions, result.OutsideFunctions} {
		for _, context := range contexts {
			for _, occurrence := range context.References {
				if codeLocationsOverlap(occurrence.CodeLocation, location) {
					return true
				}
			}
		}
	}
	return false
}

func callHierarchyCoversLocation(node retrieval.CallNode, location retrieval.CodeLocation) bool {
	if codeLocationsOverlap(node.CodeLocation, location) {
		return true
	}
	for _, child := range node.Children {
		if callHierarchyCoversLocation(child, location) {
			return true
		}
	}
	return false
}

func codeLocationsOverlap(left, right retrieval.CodeLocation) bool {
	if normalizeSearchPath(left.FilePath) != normalizeSearchPath(right.FilePath) {
		return false
	}
	return left.LineRange.Start <= right.LineRange.End && right.LineRange.Start <= left.LineRange.End
}

func (e *Engine) reserveSearchReferenceResult(selected *classifiedSearchSymbol, state *toolRoundState) bool {
	// The search optimization resolves declarations without pinning a line, so
	// it reserves the unpinned key an explicit find_references would use.
	key := referenceDedupKey(normalizeSearchPath(selected.path), selected.name, 0)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	defer state.mu.Unlock()
	if _, duplicate := state.seenToolCalls[key]; duplicate {
		return false
	}
	state.seenToolCalls[key] = struct{}{}
	return true
}

func withLiteralSearchResults(structural string, results []retrieval.SearchResult, truncatedFiles []string) string {
	if len(results) == 0 && len(truncatedFiles) == 0 {
		return structural
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(structural), &payload); err != nil {
		return structural
	}
	if len(results) > 0 {
		payload["literal_result_count"] = len(results)
		payload["literal_results"] = results
	}
	return mustToolResultJSON(withTruncatedFiles(payload, truncatedFiles))
}

func matchingIdentifierSpellings(content, queryName string) []string {
	seen := make(map[string]struct{})
	var matches []string
	for _, identifier := range searchIdentifierPattern.FindAllString(content, -1) {
		if !strings.EqualFold(identifier, queryName) {
			continue
		}
		if _, duplicate := seen[identifier]; duplicate {
			continue
		}
		seen[identifier] = struct{}{}
		matches = append(matches, identifier)
	}
	return matches
}

func toolResultIsError(raw string) bool {
	var payload struct {
		Status string `json:"status"`
	}
	return json.Unmarshal([]byte(raw), &payload) == nil && payload.Status == "error"
}

// withTruncatedFiles adds the truncated_files field to a hand-built tool
// result payload only when the retrieval layer actually clipped files at its
// read cap, mirroring the omitempty behavior of retrieval.SearchResults so
// the model learns that matches may be missing from those files.
func withTruncatedFiles(payload map[string]any, truncatedFiles []string) map[string]any {
	if len(truncatedFiles) > 0 {
		payload["truncated_files"] = truncatedFiles
	}
	return payload
}

// mergeTruncatedFiles unions the truncated-file lists of the literal and regex
// search passes, preserving order and dropping duplicates.
func mergeTruncatedFiles(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	merged := make([]string, 0, len(a)+len(b))
	for _, lists := range [][]string{a, b} {
		for _, file := range lists {
			if _, dup := seen[file]; dup {
				continue
			}
			seen[file] = struct{}{}
			merged = append(merged, file)
		}
	}
	return merged
}

func hasRegexMetachar(query string) bool {
	return strings.ContainsAny(query, `\.+*?()|[]{}^$`)
}

func mergeSearchResults(literal, regex []retrieval.SearchResult, maxResults int) []retrieval.SearchResult {
	merged := make([]retrieval.SearchResult, 0, len(literal)+len(regex))
	seen := make(map[string]struct{}, len(literal)+len(regex))
	key := func(r retrieval.SearchResult) string {
		loc := r.CodeLocation
		return fmt.Sprintf("%s:%d:%d", loc.FilePath, loc.LineRange.Start, loc.LineRange.End)
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
		return toolError(normalizeSearchPath(args.Path), "missing_argument", missingToolArgumentMessage(toolCall.Name, "symbol"))
	}
	if args.Depth <= 0 {
		args.Depth = toolcatalog.DefaultCallHierarchyDepth
	}
	// normalizeSearchPath (not normalizeToolPath) keeps the key and execution
	// aligned with search calls rewritten into call-hierarchy lookups, which
	// normalize "." to "" — otherwise identical structural lookups would get
	// different concurrency/seen keys and execute twice.
	normalizedPath := normalizeSearchPath(args.Path)
	key := callHierarchyDedupKey(toolCall.Name, normalizedPath, args.Symbol, args.Depth)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, ok := state.seenToolCalls[key]
	state.mu.Unlock()
	if ok {
		e.logf(ctx, "Skipping duplicate tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logf(ctx, "Executing tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)

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
		// A language with no structural backend can't be analyzed as a call graph, but
		// the code still exists. Rather than failing, degrade to a literal search for the
		// symbol so the model gets the definition and its call sites for any file type.
		// The scope is widened to repo-wide for a single file (mirroring the structural
		// backends) so callers/uses in other files are still surfaced.
		var unsupported *retrieval.UnsupportedLanguageError
		if errors.As(err, &unsupported) {
			return e.callHierarchySearchFallback(ctx, repoRoot, normalizedPath, args.Symbol, callers, key, state)
		}

		// Low confidence indicates the analysis ran but has uncertain results due to
		// dynamic call patterns (closures, function pointers). The LLM should treat
		// this as partial information rather than complete failure.
		var lowConf *retrieval.LowConfidenceError
		if errors.As(err, &lowConf) {
			return toolError(normalizedPath, "low_confidence", err.Error())
		}
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

// callHierarchySearchFallback runs a literal search for symbol when no structural
// backend can analyze its file type, returning a search-style payload tagged as a
// fallback. The search scope mirrors scopeForHierarchy: a single file is widened to a
// repo-wide search so callers/uses in other files are still found. The symbol is a
// plain identifier, so the regex-metachar handling that executeSearch performs is not
// needed here.
func (e *Engine) callHierarchySearchFallback(ctx context.Context, repoRoot, normalizedPath, symbol string, callers bool, key string, state *toolRoundState) string {
	mode := "callees"
	if callers {
		mode = "callers"
	}
	searchScope := retrieval.FallbackSearchScope(repoRoot, normalizedPath)
	e.logf(ctx, "Falling back to literal search for unsupported call hierarchy: mode=%s path=%s symbol=%q search_scope=%q", mode, normalizedPath, symbol, searchScope)
	results, err := e.retrieval.Search(ctx, repoRoot, searchScope, symbol, toolcatalog.DefaultSearchContextLines, 0, false)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(withTruncatedFiles(map[string]any{
		"symbol":       symbol,
		"path":         normalizedPath,
		"mode":         mode,
		"fallback":     "search",
		"note":         "structural call hierarchy is unavailable for this file type; showing literal search matches for the symbol instead",
		"query":        results.Query,
		"result_count": results.ResultCount,
		"results":      results.Results,
	}, results.TruncatedFiles))
}

func callHierarchyDedupKey(name, path, symbol string, depth int) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%d", name, path, symbol, depth)
}

// searchDedupKey identifies one literal search invocation. It must be computed
// from the normalized arguments exactly as executeSearch runs them (trimmed
// path/query, defaulted context lines) so identical repeated searches dedupe
// into a single execution plus already_requested errors, consistent with
// inspect_file/list_files.
func searchDedupKey(path, query string, contextLines, maxResults int, caseSensitive bool) string {
	return fmt.Sprintf("search\x00%s\x00%s\x00%d\x00%d\x00%t", path, query, contextLines, maxResults, caseSensitive)
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

// gitLogToolArgs are the LLM-supplied arguments of the git_log tool. Multi-path
// filters arrive as one comma-separated string because a tool schema only
// carries scalar parameters.
type gitLogToolArgs struct {
	Commit        string `json:"commit"`
	Since         string `json:"since"`
	Until         string `json:"until"`
	Author        string `json:"author"`
	Paths         string `json:"paths"`
	Message       string `json:"message"`
	MessageRegex  bool   `json:"message_regex"`
	CaseSensitive bool   `json:"case_sensitive"`
	Limit         int    `json:"limit"`
}

// options normalizes the arguments once, so the dedup key is computed from the
// exact values the provider runs with.
func (a gitLogToolArgs) options() git.LogOptions {
	return git.LogOptions{
		Commit:        strings.TrimSpace(a.Commit),
		Since:         strings.TrimSpace(a.Since),
		Until:         strings.TrimSpace(a.Until),
		Author:        strings.TrimSpace(a.Author),
		Paths:         splitToolPathList(a.Paths),
		Message:       strings.TrimSpace(a.Message),
		MessageRegex:  a.MessageRegex,
		CaseSensitive: a.CaseSensitive,
		Limit:         a.Limit,
	}
}

// gitShowToolArgs are the LLM-supplied arguments of the git_show tool.
type gitShowToolArgs struct {
	Commit     string `json:"commit"`
	To         string `json:"to"`
	Paths      string `json:"paths"`
	MaxCommits int    `json:"max_commits"`
}

func (a gitShowToolArgs) options(format model.DiffFormat) git.ShowOptions {
	return git.ShowOptions{
		Commit:     strings.TrimSpace(a.Commit),
		To:         strings.TrimSpace(a.To),
		Paths:      splitToolPathList(a.Paths),
		MaxCommits: a.MaxCommits,
		Format:     format,
	}
}

// splitToolPathList turns a comma- or newline-separated path list into
// repo-relative paths, normalized like every other tool path.
func splitToolPathList(paths string) []string {
	fields := strings.FieldsFunc(paths, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	cleaned := make([]string, 0, len(fields))
	for _, field := range fields {
		path := normalizeToolPath(strings.TrimSpace(field))
		if path == "" || path == "." {
			continue
		}
		cleaned = append(cleaned, path)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func gitLogDedupKey(opts git.LogOptions) string {
	return fmt.Sprintf("git_log\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%t\x00%t\x00%d",
		opts.Commit, opts.Since, opts.Until, opts.Author, strings.Join(opts.Paths, ","),
		opts.Message, opts.MessageRegex, opts.CaseSensitive, opts.Limit)
}

func gitShowDedupKey(opts git.ShowOptions) string {
	return fmt.Sprintf("git_show\x00%s\x00%s\x00%s\x00%d\x00%s",
		opts.Commit, opts.To, strings.Join(opts.Paths, ","), opts.MaxCommits, opts.Format)
}

// diffFormat is the diff shape the history tools return, matching the shape the
// agent already sees in its prompt payload.
func (e *Engine) diffFormat() model.DiffFormat {
	if e.config.DiffFormat == "" {
		return model.DiffFormatGit
	}
	return e.config.DiffFormat
}

func (e *Engine) executeGitLog(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args gitLogToolArgs
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	if e.history == nil {
		return toolError("", "history_unavailable", "commit history is unavailable for this review")
	}
	opts := args.options()
	key := gitLogDedupKey(opts)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, seen := state.seenToolCalls[key]
	state.mu.Unlock()
	if seen {
		e.logf(ctx, "Skipping duplicate tool call: name=%s commit=%q", toolCall.Name, opts.Commit)
		return toolError("", "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logf(ctx, "Executing tool call: name=%s commit=%q since=%q until=%q author=%q paths=%q message=%q message_regex=%t case_sensitive=%t limit=%d",
		toolCall.Name, opts.Commit, opts.Since, opts.Until, opts.Author, strings.Join(opts.Paths, ","), opts.Message, opts.MessageRegex, opts.CaseSensitive, opts.Limit)
	result, err := e.history.Log(ctx, repoRoot, opts)
	if err != nil {
		return historyToolError(err)
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(result)
}

func (e *Engine) executeGitShow(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args gitShowToolArgs
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	if e.history == nil {
		return toolError("", "history_unavailable", "commit history is unavailable for this review")
	}
	opts := args.options(e.diffFormat())
	if opts.Commit == "" {
		return toolError("", "missing_argument", missingToolArgumentMessage(toolCall.Name, "commit"))
	}
	key := gitShowDedupKey(opts)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, seen := state.seenToolCalls[key]
	state.mu.Unlock()
	if seen {
		e.logf(ctx, "Skipping duplicate tool call: name=%s commit=%q", toolCall.Name, opts.Commit)
		return toolError("", "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logf(ctx, "Executing tool call: name=%s commit=%q to=%q paths=%q max_commits=%d diff_format=%s",
		toolCall.Name, opts.Commit, opts.To, strings.Join(opts.Paths, ","), opts.MaxCommits, opts.Format)
	result, err := e.history.Show(ctx, repoRoot, opts)
	if err != nil {
		return historyToolError(err)
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(result)
}

// historyToolError distinguishes a checkout without git metadata (nothing the
// agent can do about it) from a failed or rejected git command.
func historyToolError(err error) string {
	if errors.Is(err, git.ErrNotAGitRepo) {
		return toolError("", "history_unavailable", "the reviewed checkout has no git history")
	}
	return toolError("", "history_failed", err.Error())
}
