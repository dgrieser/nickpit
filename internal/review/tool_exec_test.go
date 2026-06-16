package review

import (
	"context"
	"encoding/json"
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

// TestExecuteCallHierarchyUnsupportedLanguage verifies the distinct, actionable
// signal so the model never reads "can't analyze this language" as "the symbol
// does not exist".
func TestExecuteCallHierarchyUnsupportedLanguage(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "src/auth.rb", "def redirect_allowed(url)\n  true\nend\n")

	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	results := engine.executeToolCalls(context.Background(), repoRoot, []llm.ToolCall{
		{ID: "c1", Name: "find_callers", Arguments: `{"symbol":"redirect_allowed","path":"src/auth.rb"}`},
	}, freshToolRoundState())

	payload := decodeToolPayload(t, results[0].Content)
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected an error payload, got %#v", payload)
	}
	if errObj["code"] != "unsupported_language" {
		t.Fatalf("error code = %v, want unsupported_language", errObj["code"])
	}
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "inspect_file") {
		t.Fatalf("error message should steer to inspect_file: %q", msg)
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
