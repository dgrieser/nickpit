package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestExecuteFindLinesFindsLineAndBlock(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "cmd/main.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "line", Name: "find_lines", Arguments: mustToolResultJSON(map[string]any{
			"path": "cmd/main.go",
			"code": "\tfmt.Println(\"hi\")",
		})},
		{ID: "block", Name: "find_lines", Arguments: mustToolResultJSON(map[string]any{
			"path": "cmd/main.go",
			"code": "\nfunc main() {\r\n\tfmt.Println(\"hi\")\r\n}\n",
		})},
	}, freshToolRoundState())

	linePayload := decodeToolPayload(t, results[0].Content)
	assertFindLinesPayload(t, linePayload, 1, []retrieval.FindLinesMatch{{StartLine: 4, EndLine: 4, LineCount: 1}})

	blockPayload := decodeToolPayload(t, results[1].Content)
	assertFindLinesPayload(t, blockPayload, 3, []retrieval.FindLinesMatch{{StartLine: 3, EndLine: 5, LineCount: 3}})
}

func TestExecuteFindLinesReturnsDuplicateMatches(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/repeat.go", "x := 1\nkeep()\nx := 1\nkeep()\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "dupes", Name: "find_lines", Arguments: mustToolResultJSON(map[string]any{
			"path": "pkg/repeat.go",
			"code": "x := 1\nkeep()",
		})},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	assertFindLinesPayload(t, payload, 2, []retrieval.FindLinesMatch{
		{StartLine: 1, EndLine: 2, LineCount: 2},
		{StartLine: 3, EndLine: 4, LineCount: 2},
	})
}

func TestExecuteFindLinesReturnsZeroMatches(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/a.go", "package pkg\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "missing", Name: "find_lines", Arguments: mustToolResultJSON(map[string]any{
			"path": "pkg/a.go",
			"code": "func Missing() {}",
		})},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	assertFindLinesPayload(t, payload, 1, nil)
}

type nilResultRetrieval struct {
	stubRetrieval
}

func (nilResultRetrieval) FindLines(context.Context, string, string, string) (*retrieval.FindLinesResult, error) {
	return nil, nil
}

func TestExecuteFindLinesHandlesNilResult(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, nilResultRetrieval{}, config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "nil_file", Name: "find_lines", Arguments: `{"path":"pkg/a.go","code":"package pkg"}`},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	if got := nestedString(payload, "error", "code"); got != "retrieval_failed" {
		t.Fatalf("nil result error code = %q, payload = %#v", got, payload)
	}
	if got := nestedString(payload, "error", "message"); got != "find_lines result is nil" {
		t.Fatalf("nil result error message = %q, payload = %#v", got, payload)
	}
}

func TestExecuteFindLinesValidatesRequiredArguments(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "missing_path", Name: "find_lines", Arguments: `{"code":"x"}`},
		{ID: "missing_code", Name: "find_lines", Arguments: `{"path":"pkg/a.go","code":"\n"}`},
	}, freshToolRoundState())

	pathPayload := decodeToolPayload(t, results[0].Content)
	if got := nestedString(pathPayload, "error", "code"); got != "missing_argument" {
		t.Fatalf("missing path error code = %q, payload = %#v", got, pathPayload)
	}

	codePayload := decodeToolPayload(t, results[1].Content)
	if got := nestedString(codePayload, "error", "code"); got != "missing_argument" {
		t.Fatalf("missing code error code = %q, payload = %#v", got, codePayload)
	}
}

func TestExecuteFindLinesDedupesDuplicateCalls(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "call_1", Name: "find_lines", Arguments: `{"path":"extra.go","code":"package extra"}`},
		{ID: "call_2", Name: "find_lines", Arguments: `{"path":"./extra.go","code":"package extra"}`},
	}, freshToolRoundState())

	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d, want 1", len(retrievalEngine.paths))
	}
	firstPayload := decodeToolPayload(t, results[0].Content)
	assertFindLinesPayload(t, firstPayload, 1, []retrieval.FindLinesMatch{{StartLine: 1, EndLine: 1, LineCount: 1}})

	secondPayload := decodeToolPayload(t, results[1].Content)
	if got := nestedString(secondPayload, "error", "code"); got != "already_requested" {
		t.Fatalf("duplicate error code = %q, payload = %#v", got, secondPayload)
	}
}

func assertFindLinesPayload(t *testing.T, payload map[string]any, wantCodeLines int, wantMatches []retrieval.FindLinesMatch) {
	t.Helper()
	if _, isErr := payload["error"]; isErr {
		t.Fatalf("find_lines returned error: %#v", payload)
	}
	if got := intFromJSON(payload["code_line_count"]); got != wantCodeLines {
		t.Fatalf("code_line_count = %d, want %d; payload = %#v", got, wantCodeLines, payload)
	}
	if got := intFromJSON(payload["match_count"]); got != len(wantMatches) {
		t.Fatalf("match_count = %d, want %d; payload = %#v", got, len(wantMatches), payload)
	}
	rawMatches, ok := payload["matches"].([]any)
	if !ok {
		t.Fatalf("matches missing or wrong type: %#v", payload["matches"])
	}
	if len(rawMatches) != len(wantMatches) {
		t.Fatalf("matches length = %d, want %d; payload = %#v", len(rawMatches), len(wantMatches), payload)
	}
	for i, want := range wantMatches {
		got, ok := rawMatches[i].(map[string]any)
		if !ok {
			t.Fatalf("match[%d] wrong type: %#v", i, rawMatches[i])
		}
		if intFromJSON(got["start_line"]) != want.StartLine ||
			intFromJSON(got["end_line"]) != want.EndLine ||
			intFromJSON(got["line_count"]) != want.LineCount {
			t.Fatalf("match[%d] = %#v, want %#v", i, got, want)
		}
	}
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
			name: "find_lines",
			call: llm.ToolCall{ID: "loc", Name: "find_lines", Arguments: `{"path":"./src/demo.go","code":"func Run() int { return 1 }\n"}`},
			want: findLinesDedupKey(goPath, "func Run() int { return 1 }"),
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
			name:   "search ruby function name stays unique",
			call:   llm.ToolCall{ID: "j", Name: "search", Arguments: `{"path":"src/demo.rb","query":"Run()"}`},
			unique: true,
		},
		{
			name:   "search java function name stays unique",
			call:   llm.ToolCall{ID: "k", Name: "search", Arguments: `{"path":"src/Demo.java","query":"Run()"}`},
			unique: true,
		},
		{
			name:   "search non-function query stays unique",
			call:   llm.ToolCall{ID: "l", Name: "search", Arguments: `{"path":"src/demo.go","query":"return x"}`},
			unique: true,
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
