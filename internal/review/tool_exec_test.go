package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func freshToolRoundState() *toolRoundState {
	return &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}
}

func decodeToolPayload(t *testing.T, content string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("tool result not valid json: %v (%s)", err, content)
	}
	return payload
}

// TestExecuteSearchRunsLiterallyForUnsupportedLanguage is the direct regression
// test for the reported bug: a function-name search on a file whose language has
// no structural backend must run as a literal search (and find the definition),
// not be rewritten into a call-graph lookup that can only fail.
func TestExecuteSearchRunsLiterallyForUnsupportedLanguage(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "src/auth.rb", "def redirect_allowed(url)\n  true\nend\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "c1", Name: "search", Arguments: `{"path":"src/auth.rb","query":"redirect_allowed("}`},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	if _, isErr := payload["error"]; isErr {
		t.Fatalf("search returned an error instead of literal matches: %#v", payload)
	}
	if _, ok := payload["result_count"]; !ok {
		t.Fatalf("expected a literal search payload, got %#v", payload)
	}
	if rc, _ := payload["result_count"].(float64); rc < 1 {
		t.Fatalf("expected the Ruby definition to be found, result_count = %v", payload["result_count"])
	}
}

// TestExecuteCallHierarchyFallsBackToSearchForUnsupportedLanguage verifies that
// find_callers on a file whose language has no structural backend degrades to a
// literal search for the symbol instead of failing, so the model still gets the
// definition and any call sites rather than an error it has to recover from.
func TestExecuteCallHierarchyFallsBackToSearchForUnsupportedLanguage(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "src/auth.rb", "def redirect_allowed(url)\n  true\nend\n")
	// A caller in another file confirms the fallback widens a single-file scope to a
	// repo-wide search rather than only inspecting src/auth.rb.
	writeRepoFile(t, repoRoot, "app/login.rb", "redirect_allowed(next_url)\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "c1", Name: "find_callers", Arguments: `{"symbol":"redirect_allowed","path":"src/auth.rb"}`},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	if _, isErr := payload["error"]; isErr {
		t.Fatalf("find_callers returned an error instead of a search fallback: %#v", payload)
	}
	if payload["fallback"] != "search" {
		t.Fatalf("expected fallback=search, got %#v", payload)
	}
	if payload["mode"] != "callers" {
		t.Fatalf("expected mode=callers, got %v", payload["mode"])
	}
	if rc, _ := payload["result_count"].(float64); rc < 2 {
		t.Fatalf("expected the definition and the caller to be found, result_count = %v", payload["result_count"])
	}
}

// TestExecuteSearchStillRewritesForSupportedLanguage guards against regressing
// the optimization for languages that DO have a structural backend.
func TestExecuteSearchStillRewritesForSupportedLanguage(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "src/demo.go", "package demo\n\nfunc Run() int { return 1 }\n")

	counting := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, counting, config.Profile{Model: "test"})
	engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "c1", Name: "search", Arguments: `{"path":"src/demo.go","query":"Run()"}`},
	}, freshToolRoundState())

	rewritten := false
	for _, p := range counting.paths {
		if strings.HasPrefix(p, "callers:") && strings.Contains(p, "Run") {
			rewritten = true
		}
	}
	if !rewritten {
		t.Fatalf("expected a .go function-name search to rewrite to find_callers, recorded paths = %v", counting.paths)
	}
}

// TestExecuteSearchRewritesForRustLanguage guards the optimization for Rust,
// which is supported by rustBackend: a `.rs` function-name search must rewrite to
// find_callers, the same as Go/Python/TypeScript.
func TestExecuteSearchRewritesForRustLanguage(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "src/demo.rs", "pub fn Run() -> i32 { 1 }\n")

	counting := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, counting, config.Profile{Model: "test"})
	engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "c1", Name: "search", Arguments: `{"path":"src/demo.rs","query":"Run()"}`},
	}, freshToolRoundState())

	rewritten := false
	for _, p := range counting.paths {
		if strings.HasPrefix(p, "callers:") && strings.Contains(p, "Run") {
			rewritten = true
		}
	}
	if !rewritten {
		t.Fatalf("expected a .rs function-name search to rewrite to find_callers, recorded paths = %v", counting.paths)
	}
}

// TestExecuteSearchLiteralWhenOptimizationDisabled covers the otherwise-untested
// disabled branch: with the optimization off, even a function-name search on a
// supported language must run literally rather than rewriting to find_callers.
func TestExecuteSearchLiteralWhenOptimizationDisabled(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "src/demo.go", "package demo\n\nfunc Run() int { return 1 }\n")

	counting := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, counting, config.Profile{Model: "test"})
	engine.SetSearchToolOptimization(false)
	engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "c1", Name: "search", Arguments: `{"path":"src/demo.go","query":"Run()"}`},
	}, freshToolRoundState())

	sawLiteralSearch := false
	for _, p := range counting.paths {
		if strings.HasPrefix(p, "callers:") {
			t.Fatalf("expected literal search when optimization disabled, but got a find_callers rewrite: %v", counting.paths)
		}
		if strings.HasPrefix(p, "search:") {
			sawLiteralSearch = true
		}
	}
	if !sawLiteralSearch {
		t.Fatalf("expected a literal search path, recorded paths = %v", counting.paths)
	}
}

func TestExecuteSearchFindsLineAndBlock(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "cmd/main.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "line", Name: "search", Arguments: mustToolResultJSON(map[string]any{
			"path":  "cmd/main.go",
			"query": "\tfmt.Println(\"hi\")",
		})},
		{ID: "block", Name: "search", Arguments: mustToolResultJSON(map[string]any{
			"path":  "cmd/main.go",
			"query": "\nfunc main() {\r\n\tfmt.Println(\"hi\")\r\n}\n",
		})},
	}, freshToolRoundState())

	linePayload := decodeToolPayload(t, results[0].Content)
	assertSearchPayload(t, linePayload, []retrieval.CodeLocation{{LineRange: retrieval.LineRange{Start: 4, End: 4, Count: 1}}})
	// A single-line query defaults to 5 context lines, carried outside the
	// exact code_location.
	lineResult := searchPayloadResult(t, linePayload, 0)
	if _, ok := lineResult["context_before"]; !ok {
		t.Fatalf("single-line result misses context_before: %#v", lineResult)
	}
	if _, ok := lineResult["context_after"]; !ok {
		t.Fatalf("single-line result misses context_after: %#v", lineResult)
	}

	blockPayload := decodeToolPayload(t, results[1].Content)
	assertSearchPayload(t, blockPayload, []retrieval.CodeLocation{{LineRange: retrieval.LineRange{Start: 3, End: 5, Count: 3}}})
	// A multi-line query defaults to 0 context lines.
	blockResult := searchPayloadResult(t, blockPayload, 0)
	if _, ok := blockResult["context_before"]; ok {
		t.Fatalf("multi-line result should not carry context_before by default: %#v", blockResult)
	}
	if _, ok := blockResult["context_after"]; ok {
		t.Fatalf("multi-line result should not carry context_after by default: %#v", blockResult)
	}
}

func TestExecuteSearchReturnsDuplicateBlockMatches(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/repeat.go", "x := 1\nkeep()\nx := 1\nkeep()\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "dupes", Name: "search", Arguments: mustToolResultJSON(map[string]any{
			"path":  "pkg/repeat.go",
			"query": "x := 1\nkeep()",
		})},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	assertSearchPayload(t, payload, []retrieval.CodeLocation{
		{LineRange: retrieval.LineRange{Start: 1, End: 2, Count: 2}},
		{LineRange: retrieval.LineRange{Start: 3, End: 4, Count: 2}},
	})
}

func TestExecuteSearchBlockIgnoresIndentationWhitespace(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/block.go", "func outer() {\n\tif cond {\n\t\tdoThing()\n\t}\n}\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		// Snippet is unindented and carries trailing spaces; it should still match
		// the indented block in the file. The braces/parens must NOT be treated as
		// a regex for a multi-line query.
		{ID: "block", Name: "search", Arguments: mustToolResultJSON(map[string]any{
			"path":  "pkg/block.go",
			"query": "if cond {  \ndoThing()\n}",
		})},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	assertSearchPayload(t, payload, []retrieval.CodeLocation{
		// Content preserves the file's original indentation, not the trimmed query.
		{FilePath: "pkg/block.go", LineRange: retrieval.LineRange{Start: 2, End: 4, Count: 3}, Content: "\tif cond {\n\t\tdoThing()\n\t}"},
	})
}

func TestExecuteSearchIgnoresWhitespaceOnlyBoundaryLines(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/run.go", "func Run() {}\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		// Blank boundary lines are trimmed during normalization, leaving a
		// single-line query.
		{ID: "run", Name: "search", Arguments: mustToolResultJSON(map[string]any{
			"path":  "pkg/run.go",
			"query": " \nfunc Run() {}\n ",
		})},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	assertSearchPayload(t, payload, []retrieval.CodeLocation{
		{FilePath: "pkg/run.go", LineRange: retrieval.LineRange{Start: 1, End: 1, Count: 1}, Content: "func Run() {}"},
	})
}

func TestExecuteSearchBlockReturnsZeroMatches(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/a.go", "package pkg\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "missing", Name: "search", Arguments: mustToolResultJSON(map[string]any{
			"path":  "pkg/a.go",
			"query": "func Missing() {\nreturn\n}",
		})},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	assertSearchPayload(t, payload, nil)
}

func TestExecuteSearchRequiresQueryButNotPath(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/a.go", "package pkg\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "missing_query", Name: "search", Arguments: `{"path":"pkg/a.go","query":"\n"}`},
		{ID: "no_path", Name: "search", Arguments: `{"query":"package pkg"}`},
	}, freshToolRoundState())

	queryPayload := decodeToolPayload(t, results[0].Content)
	if got := nestedString(queryPayload, "error", "code"); got != "missing_argument" {
		t.Fatalf("missing query error code = %q, payload = %#v", got, queryPayload)
	}

	// An omitted path is valid and searches the whole repo.
	noPathPayload := decodeToolPayload(t, results[1].Content)
	assertSearchPayload(t, noPathPayload, []retrieval.CodeLocation{
		{FilePath: "pkg/a.go", LineRange: retrieval.LineRange{Start: 1, End: 1, Count: 1}},
	})
}

func TestExecuteSearchBlockSearchesWholeRepoWhenPathOmitted(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "a/one.go", "package a\n\nfunc Run() {\n\twork()\n}\n")
	writeRepoFile(t, repoRoot, "b/two.go", "package b\n\nfunc Run() {\n\twork()\n}\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "repo_wide", Name: "search", Arguments: mustToolResultJSON(map[string]any{
			"query": "func Run() {\nwork()\n}",
		})},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	assertSearchPayload(t, payload, []retrieval.CodeLocation{
		{FilePath: "a/one.go", LineRange: retrieval.LineRange{Start: 3, End: 5, Count: 3}},
		{FilePath: "b/two.go", LineRange: retrieval.LineRange{Start: 3, End: 5, Count: 3}},
	})
}

func TestExecuteSearchDedupesNormalizedBlockCalls(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "call_1", Name: "search", Arguments: `{"path":"extra.go","query":"package extra\nfunc A() {}"}`},
		// Identical after normalization: "./" path prefix, CRLF endings, and
		// surrounding blank lines.
		{ID: "call_2", Name: "search", Arguments: `{"path":"./extra.go","query":"\npackage extra\r\nfunc A() {}\n"}`},
	}, freshToolRoundState())

	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d, want 1 (%v)", len(retrievalEngine.paths), retrievalEngine.paths)
	}
	if _, isErr := decodeToolPayload(t, results[0].Content)["error"]; isErr {
		t.Fatalf("first search errored: %s", results[0].Content)
	}
	secondPayload := decodeToolPayload(t, results[1].Content)
	if got := nestedString(secondPayload, "error", "code"); got != "already_requested" {
		t.Fatalf("duplicate error code = %q, payload = %#v", got, secondPayload)
	}
}

func TestExecuteSearchDedupesRepoRootAlias(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "call_1", Name: "search", Arguments: `{"query":"package extra\nfunc A() {}"}`},
		{ID: "call_2", Name: "search", Arguments: `{"path":".","query":"package extra\nfunc A() {}"}`},
	}, freshToolRoundState())

	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d, want 1 (%v)", len(retrievalEngine.paths), retrievalEngine.paths)
	}
	if !strings.HasPrefix(retrievalEngine.paths[0], "search::") {
		t.Fatalf("retrieval path = %q, want repo root", retrievalEngine.paths[0])
	}
	if _, isErr := decodeToolPayload(t, results[0].Content)["error"]; isErr {
		t.Fatalf("first search errored: %s", results[0].Content)
	}
	secondPayload := decodeToolPayload(t, results[1].Content)
	if got := nestedString(secondPayload, "error", "code"); got != "already_requested" {
		t.Fatalf("duplicate error code = %q, payload = %#v", got, secondPayload)
	}
}

// TestExecuteSearchContextLinesDefaults pins the per-mode defaulting: an
// omitted context_lines resolves to 5 for single-line queries and 0 for
// multi-line blocks, while explicit values (including 0) are honored as-is.
func TestExecuteSearchContextLinesDefaults(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		want      int
	}{
		{name: "single-line omitted", arguments: `{"query":"needle"}`, want: 5},
		{name: "single-line explicit zero", arguments: `{"query":"needle","context_lines":0}`, want: 0},
		{name: "single-line explicit", arguments: `{"query":"needle","context_lines":2}`, want: 2},
		{name: "single-line negative", arguments: `{"query":"needle","context_lines":-1}`, want: 5},
		{name: "multi-line omitted", arguments: `{"query":"needle\nthread"}`, want: 0},
		{name: "multi-line explicit", arguments: `{"query":"needle\nthread","context_lines":3}`, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievalEngine := &countingRetrieval{}
			engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
			engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
				{ID: "c1", Name: "search", Arguments: tt.arguments},
			}, freshToolRoundState())
			if len(retrievalEngine.paths) == 0 {
				t.Fatal("search did not execute")
			}
			parts := strings.Split(retrievalEngine.paths[0], ":")
			if got := parts[3]; got != fmt.Sprintf("%d", tt.want) {
				t.Fatalf("context_lines = %s, want %d (recorded %q)", got, tt.want, retrievalEngine.paths[0])
			}
		})
	}
}

// assertSearchPayload checks the search tool payload: result_count and each
// result's code_location (line_range always; file_path/content when the
// expectation sets them).
func assertSearchPayload(t *testing.T, payload map[string]any, wantLocations []retrieval.CodeLocation) {
	t.Helper()
	if _, isErr := payload["error"]; isErr {
		t.Fatalf("search returned error: %#v", payload)
	}
	if got := intFromJSON(payload["result_count"]); got != len(wantLocations) {
		t.Fatalf("result_count = %d, want %d; payload = %#v", got, len(wantLocations), payload)
	}
	rawResults, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("results missing or wrong type: %#v", payload["results"])
	}
	if len(rawResults) != len(wantLocations) {
		t.Fatalf("results length = %d, want %d; payload = %#v", len(rawResults), len(wantLocations), payload)
	}
	for i, wantLoc := range wantLocations {
		result := searchPayloadResult(t, payload, i)
		loc, ok := result["code_location"].(map[string]any)
		if !ok {
			t.Fatalf("result[%d] code_location missing or wrong type: %#v", i, result["code_location"])
		}
		lineRange, ok := loc["line_range"].(map[string]any)
		if !ok {
			t.Fatalf("result[%d] line_range missing or wrong type: %#v", i, loc["line_range"])
		}
		if intFromJSON(lineRange["start"]) != wantLoc.LineRange.Start ||
			intFromJSON(lineRange["end"]) != wantLoc.LineRange.End ||
			intFromJSON(lineRange["count"]) != wantLoc.LineRange.Count {
			t.Fatalf("result[%d] line_range = %#v, want %#v", i, lineRange, wantLoc.LineRange)
		}
		if wantLoc.FilePath != "" && loc["file_path"] != wantLoc.FilePath {
			t.Fatalf("result[%d] file_path = %#v, want %q", i, loc["file_path"], wantLoc.FilePath)
		}
		if wantLoc.Content != "" && loc["content"] != wantLoc.Content {
			t.Fatalf("result[%d] content = %#v, want %q", i, loc["content"], wantLoc.Content)
		}
	}
}

func searchPayloadResult(t *testing.T, payload map[string]any, index int) map[string]any {
	t.Helper()
	rawResults, ok := payload["results"].([]any)
	if !ok || index >= len(rawResults) {
		t.Fatalf("results[%d] missing: %#v", index, payload["results"])
	}
	result, ok := rawResults[index].(map[string]any)
	if !ok {
		t.Fatalf("results[%d] wrong type: %#v", index, rawResults[index])
	}
	return result
}

func nestedString(payload map[string]any, keys ...string) string {
	var current any = payload
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	value, _ := current.(string)
	return value
}

func intFromJSON(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// TestToolCallConcurrencyKey directly exercises the dedup-key generator across
// every tool and the language-aware search rewrite. The search branch hits the
// filesystem via SupportsStructuralAnalysis, so real fixture files are written.
func TestToolCallConcurrencyKey(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "src/demo.go", "package demo\n\nfunc Run() int { return 1 }\n")
	writeRepoFile(t, repoRoot, "src/demo.py", "def Run():\n    return 1\n")
	writeRepoFile(t, repoRoot, "src/demo.ts", "export function Run() { return 1 }\n")
	writeRepoFile(t, repoRoot, "src/demo.rs", "pub fn Run() -> i32 { 1 }\n")
	writeRepoFile(t, repoRoot, "src/demo.rb", "def Run\n  1\nend\n")
	writeRepoFile(t, repoRoot, "src/Demo.java", "class Demo {}\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, &countingRetrieval{}, config.Profile{Model: "test"})

	goPath := normalizeToolPath("src/demo.go")
	searchRewriteKey := func(path string) string {
		return callHierarchyDedupKey("find_callers", normalizeToolPath(path), "Run", defaultCallHierarchyDepth)
	}

	tests := []struct {
		name   string
		call   llm.ToolCall
		want   string // exact match when unique == false
		unique bool   // when true, only the "unique\x00" prefix is asserted
	}{
		{
			name: "inspect_file",
			call: llm.ToolCall{ID: "a", Name: "inspect_file", Arguments: `{"path":"src/demo.go"}`},
			want: fmt.Sprintf("inspect_file\x00%s", goPath),
		},
		{
			name: "list_files default depth",
			call: llm.ToolCall{ID: "b", Name: "list_files", Arguments: `{"path":"src"}`},
			want: fmt.Sprintf("list_files\x00%s\x00%d", normalizeToolPath("src"), 1),
		},
		{
			name: "list_files explicit depth",
			call: llm.ToolCall{ID: "c", Name: "list_files", Arguments: `{"path":"src","depth":3}`},
			want: fmt.Sprintf("list_files\x00%s\x00%d", normalizeToolPath("src"), 3),
		},
		{
			name: "find_callers default depth",
			call: llm.ToolCall{ID: "d", Name: "find_callers", Arguments: `{"path":"src/demo.go","symbol":"Run"}`},
			want: callHierarchyDedupKey("find_callers", goPath, "Run", defaultCallHierarchyDepth),
		},
		{
			name: "find_callees explicit depth",
			call: llm.ToolCall{ID: "e", Name: "find_callees", Arguments: `{"path":"src/demo.go","symbol":"Run","depth":4}`},
			want: callHierarchyDedupKey("find_callees", goPath, "Run", 4),
		},
		{
			name: "search go function name rewrites",
			call: llm.ToolCall{ID: "f", Name: "search", Arguments: `{"path":"src/demo.go","query":"Run()"}`},
			want: searchRewriteKey("src/demo.go"),
		},
		{
			name: "search python function name rewrites",
			call: llm.ToolCall{ID: "g", Name: "search", Arguments: `{"path":"src/demo.py","query":"Run()"}`},
			want: searchRewriteKey("src/demo.py"),
		},
		{
			name: "search typescript function name rewrites",
			call: llm.ToolCall{ID: "h", Name: "search", Arguments: `{"path":"src/demo.ts","query":"Run()"}`},
			want: searchRewriteKey("src/demo.ts"),
		},
		{
			name: "search rust function name rewrites",
			call: llm.ToolCall{ID: "i", Name: "search", Arguments: `{"path":"src/demo.rs","query":"Run()"}`},
			want: searchRewriteKey("src/demo.rs"),
		},
		{
			name: "search ruby function name keys as literal search",
			call: llm.ToolCall{ID: "j", Name: "search", Arguments: `{"path":"src/demo.rb","query":"Run()"}`},
			want: searchDedupKey("src/demo.rb", "Run()", defaultSearchContextLines, 0, false),
		},
		{
			name: "search java function name keys as literal search",
			call: llm.ToolCall{ID: "k", Name: "search", Arguments: `{"path":"src/Demo.java","query":"Run()"}`},
			want: searchDedupKey("src/Demo.java", "Run()", defaultSearchContextLines, 0, false),
		},
		{
			name: "search non-function query keys as literal search",
			call: llm.ToolCall{ID: "l", Name: "search", Arguments: `{"path":"src/demo.go","query":"return x"}`},
			want: searchDedupKey("src/demo.go", "return x", defaultSearchContextLines, 0, false),
		},
		{
			name: "search normalizes path and query for the dedup key",
			call: llm.ToolCall{ID: "m", Name: "search", Arguments: `{"path":"./src/demo.go","query":"  return x ","context_lines":-1}`},
			want: searchDedupKey("src/demo.go", "return x", defaultSearchContextLines, 0, false),
		},
		{
			name:   "search empty query stays unique",
			call:   llm.ToolCall{ID: "n", Name: "search", Arguments: `{"path":"src/demo.go","query":"  "}`},
			unique: true,
		},
		{
			name: "search omitted context_lines defaults per single-line query",
			call: llm.ToolCall{ID: "o", Name: "search", Arguments: `{"path":"src/demo.go","query":"return x"}`},
			want: searchDedupKey("src/demo.go", "return x", defaultSearchContextLines, 0, false),
		},
		{
			name: "search multi-line block keys with zero default context",
			call: llm.ToolCall{ID: "p", Name: "search", Arguments: `{"path":"./src/demo.go","query":"func Run() int { return 1 }\nfunc Two() {}\n"}`},
			want: searchDedupKey(goPath, "func Run() int { return 1 }\nfunc Two() {}", 0, 0, false),
		},
		{
			name: "search repo-root alias keys as empty path",
			call: llm.ToolCall{ID: "q", Name: "search", Arguments: `{"path":".","query":"return x"}`},
			want: searchDedupKey("", "return x", defaultSearchContextLines, 0, false),
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.toolCallConcurrencyKey(tt.call, i, repoRoot)
			if tt.unique {
				if !strings.HasPrefix(got, "unique\x00") {
					t.Fatalf("key = %q, want a unique\\x00 key", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("key = %q, want %q", got, tt.want)
			}
		})
	}
}

// truncatingRetrieval returns a truncated full-file read and records slice
// requests, for the truncated-file lockout regression tests.
type truncatingRetrieval struct {
	stubRetrieval
	mu        sync.Mutex
	truncated bool
	fullReads int
	slices    []string
}

func (r *truncatingRetrieval) GetFile(_ context.Context, _ string, path string) (*retrieval.FileContent, error) {
	r.mu.Lock()
	r.fullReads++
	r.mu.Unlock()
	return &retrieval.FileContent{Path: path, Content: "package big", Language: "go", Truncated: r.truncated}, nil
}

func (r *truncatingRetrieval) GetFileSlice(_ context.Context, _ string, path string, start, end int) (*retrieval.FileSlice, error) {
	r.mu.Lock()
	r.slices = append(r.slices, fmt.Sprintf("%s:%d-%d", path, start, end))
	r.mu.Unlock()
	return &retrieval.FileSlice{Path: path, StartLine: start, EndLine: end, Content: "sliced", Language: "go"}, nil
}

// TestExecuteInspectFileTruncatedFullReadAllowsRangedFollowups is the direct
// regression test for the truncated-file lockout: a full-file inspect of an
// over-cap file tells the model to "request specific line ranges for the
// remainder", so subsequent ranged requests on that path must execute instead
// of returning already_requested. Full-file repeats and repeated identical
// ranges still dedupe.
func TestExecuteInspectFileTruncatedFullReadAllowsRangedFollowups(t *testing.T) {
	retrievalEngine := &truncatingRetrieval{truncated: true}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := freshToolRoundState()

	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "full", Name: "inspect_file", Arguments: `{"path":"big.go"}`},
		{ID: "ranged", Name: "inspect_file", Arguments: `{"path":"big.go","line_start":100,"line_end":120}`},
		{ID: "ranged_dup", Name: "inspect_file", Arguments: `{"path":"big.go","line_start":100,"line_end":120}`},
		{ID: "full_dup", Name: "inspect_file", Arguments: `{"path":"big.go"}`},
	}, state)

	fullPayload := decodeToolPayload(t, results[0].Content)
	if fullPayload["truncated"] != true {
		t.Fatalf("full read payload = %#v, want truncated=true", fullPayload)
	}
	rangedPayload := decodeToolPayload(t, results[1].Content)
	if _, isErr := rangedPayload["error"]; isErr {
		t.Fatalf("ranged follow-up after truncated full read = %#v, want content instead of already_requested", rangedPayload)
	}
	if rangedPayload["content"] != "sliced" {
		t.Fatalf("ranged payload = %#v, want sliced content", rangedPayload)
	}
	if got := nestedString(decodeToolPayload(t, results[2].Content), "error", "code"); got != "already_requested" {
		t.Fatalf("repeated identical range error code = %q, want already_requested", got)
	}
	if got := nestedString(decodeToolPayload(t, results[3].Content), "error", "code"); got != "already_requested" {
		t.Fatalf("repeated full read error code = %q, want already_requested", got)
	}
	if retrievalEngine.fullReads != 1 {
		t.Fatalf("full reads = %d, want 1", retrievalEngine.fullReads)
	}
	// Ranged dedup intentionally fetches before checking coverage (the returned
	// range may be clamped), so both ranged requests hit retrieval even though
	// the second reports already_requested.
	if len(retrievalEngine.slices) != 2 {
		t.Fatalf("slice reads = %#v, want two fetches with the second deduped after clamping", retrievalEngine.slices)
	}
}

// TestExecuteInspectFileCompleteFullReadStillDedupesRangedFollowups pins the
// unchanged half of the behavior: after a complete (untruncated) full-file
// read, ranged follow-ups remain duplicates because the whole file is already
// in context.
func TestExecuteInspectFileCompleteFullReadStillDedupesRangedFollowups(t *testing.T) {
	retrievalEngine := &truncatingRetrieval{truncated: false}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := freshToolRoundState()

	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "full", Name: "inspect_file", Arguments: `{"path":"small.go"}`},
		{ID: "ranged", Name: "inspect_file", Arguments: `{"path":"small.go","line_start":1,"line_end":2}`},
	}, state)

	if _, isErr := decodeToolPayload(t, results[0].Content)["error"]; isErr {
		t.Fatalf("full read errored: %s", results[0].Content)
	}
	if got := nestedString(decodeToolPayload(t, results[1].Content), "error", "code"); got != "already_requested" {
		t.Fatalf("ranged follow-up error code = %q, want already_requested after complete read", got)
	}
	if len(retrievalEngine.slices) != 0 {
		t.Fatalf("slice reads = %#v, want none", retrievalEngine.slices)
	}
}

// TestExecuteSearchDedupesIdenticalCalls covers the search dedup gap: identical
// repeated searches must execute once and return the standard already_requested
// payload afterwards, consistent with inspect_file/find_lines/list_files.
func TestExecuteSearchDedupesIdenticalCalls(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := freshToolRoundState()

	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "s1", Name: "search", Arguments: `{"path":"pkg","query":"needle"}`},
		// Identical after normalization: leading "./" and query whitespace.
		{ID: "s2", Name: "search", Arguments: `{"path":"./pkg","query":" needle "}`},
		{ID: "s3", Name: "search", Arguments: `{"path":"pkg","query":"other"}`},
	}, state)

	firstPayload := decodeToolPayload(t, results[0].Content)
	if _, isErr := firstPayload["error"]; isErr {
		t.Fatalf("first search errored: %#v", firstPayload)
	}
	if got := nestedString(decodeToolPayload(t, results[1].Content), "error", "code"); got != "already_requested" {
		t.Fatalf("duplicate search error code = %q, want already_requested", got)
	}
	thirdPayload := decodeToolPayload(t, results[2].Content)
	if _, isErr := thirdPayload["error"]; isErr {
		t.Fatalf("distinct search errored: %#v", thirdPayload)
	}

	searches := 0
	for _, p := range retrievalEngine.paths {
		if strings.HasPrefix(p, "search:") {
			searches++
		}
	}
	if searches != 2 {
		t.Fatalf("search executions = %d (%v), want 2 (needle once, other once)", searches, retrievalEngine.paths)
	}
}

// TestExecuteSearchDedupesAcrossRounds mirrors the duplicate-call cascade seen
// in real runs: the same search repeated in a later round of the same agent
// loop must hit the shared seen-state.
func TestExecuteSearchDedupesAcrossRounds(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := freshToolRoundState()

	first := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "r1", Name: "search", Arguments: `{"path":"pkg","query":"needle","context_lines":3,"max_results":10}`},
	}, state)
	second := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "r2", Name: "search", Arguments: `{"path":"pkg","query":"needle","context_lines":3,"max_results":10}`},
		// Different max_results is a different invocation and must run.
		{ID: "r3", Name: "search", Arguments: `{"path":"pkg","query":"needle","context_lines":3,"max_results":20}`},
	}, state)

	if _, isErr := decodeToolPayload(t, first[0].Content)["error"]; isErr {
		t.Fatalf("first search errored: %s", first[0].Content)
	}
	if got := nestedString(decodeToolPayload(t, second[0].Content), "error", "code"); got != "already_requested" {
		t.Fatalf("cross-round duplicate error code = %q, want already_requested", got)
	}
	if _, isErr := decodeToolPayload(t, second[1].Content)["error"]; isErr {
		t.Fatalf("search with different params errored: %s", second[1].Content)
	}
}
