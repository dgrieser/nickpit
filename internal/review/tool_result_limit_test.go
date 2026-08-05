package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

func limitedPayload(t *testing.T, value any, maxKiB int) (string, map[string]any) {
	t.Helper()
	raw := mustToolResultJSON(value)
	limited := limitToolResultJSON(raw, maxKiB)
	if maxKiB > 0 && len(limited) > maxKiB<<10 {
		t.Fatalf("limited payload = %d bytes, cap = %d", len(limited), maxKiB<<10)
	}
	if !utf8.ValidString(limited) {
		t.Fatal("limited payload is not valid UTF-8")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(limited), &payload); err != nil {
		t.Fatalf("limited payload is not valid JSON: %v", err)
	}
	return limited, payload
}

func TestToolResultLimiterLeavesSmallPayloadUnchanged(t *testing.T) {
	raw := `{"path":"a.go","content":"small"}`
	if got := limitToolResultJSON(raw, 128); got != raw {
		t.Fatalf("result changed: %s", got)
	}
}

func TestToolResultLimiterZeroDisablesBytesButKeepsReferenceCountLimit(t *testing.T) {
	functions := make([]map[string]any, 30)
	for i := range functions {
		functions[i] = map[string]any{"name": fmt.Sprintf("f%d", i)}
	}
	_, payload := limitedPayload(t, map[string]any{
		"target": map[string]any{"name": "Shared"}, "functions": functions, "outside_functions": []any{},
	}, 0)
	if got := len(payload["functions"].([]any)); got != maxReferenceFunctions {
		t.Fatalf("functions = %d", got)
	}
	if payload["truncated"] != true || intFromJSON(payload["omitted_contexts"]) != 5 {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestToolResultLimiterReducesToolPayloadShapes(t *testing.T) {
	large := strings.Repeat("ü", 12<<10)
	searchResults := make([]map[string]any, 20)
	files := make([]string, 500)
	children := make([]map[string]any, 12)
	commits := make([]map[string]any, 12)
	for i := range 20 {
		searchResults[i] = map[string]any{"code_location": map[string]any{"file_path": fmt.Sprintf("f%d.go", i), "content": large}}
	}
	for i := range files {
		files[i] = fmt.Sprintf("pkg/%04d/very-long-file-name.go", i)
	}
	for i := range children {
		children[i] = map[string]any{"name": fmt.Sprintf("caller%d", i), "code_location": map[string]any{"content": large}, "children": []any{}}
	}
	for i := range commits {
		commits[i] = map[string]any{"sha": fmt.Sprintf("%040d", i), "body": large, "files": files[:20]}
	}

	tests := []struct {
		name  string
		value map[string]any
		check func(*testing.T, map[string]any)
	}{
		{
			name:  "inspect",
			value: map[string]any{"path": "large.go", "start_line": 10, "end_line": 99, "content": large},
			check: func(t *testing.T, payload map[string]any) {
				if payload["path"] != "large.go" || intFromJSON(payload["end_line"]) < 10 {
					t.Fatalf("inspect payload = %#v", payload)
				}
			},
		},
		{
			name:  "list",
			value: map[string]any{"path": "", "files": files},
			check: func(t *testing.T, payload map[string]any) {
				if intFromJSON(payload["omitted_files"]) == 0 {
					t.Fatalf("list payload = %#v", payload)
				}
			},
		},
		{
			name:  "search",
			value: map[string]any{"query": "Shared", "result_count": len(searchResults), "results": searchResults},
			check: func(t *testing.T, payload map[string]any) {
				if intFromJSON(payload["result_count"]) != len(payload["results"].([]any)) || intFromJSON(payload["omitted_results"]) == 0 {
					t.Fatalf("search payload = %#v", payload)
				}
			},
		},
		{
			name:  "hierarchy",
			value: map[string]any{"mode": "callers", "root": map[string]any{"name": "Shared", "code_location": map[string]any{"content": large}, "children": children}},
			check: func(t *testing.T, payload map[string]any) {
				root, _ := payload["root"].(map[string]any)
				if root["name"] != "Shared" {
					t.Fatalf("hierarchy payload = %#v", payload)
				}
			},
		},
		{
			name:  "git log",
			value: map[string]any{"range": "HEAD", "commit_count": len(commits), "commits": commits},
			check: func(t *testing.T, payload map[string]any) {
				if intFromJSON(payload["commit_count"]) != len(payload["commits"].([]any)) || intFromJSON(payload["omitted_commits"]) == 0 {
					t.Fatalf("git payload = %#v", payload)
				}
			},
		},
		{
			name: "git show",
			value: map[string]any{
				"range": "HEAD", "diff_format": "git", "commit_count": 1,
				"commits": []any{map[string]any{
					"sha": "abc", "diff_files": []any{map[string]any{"file_path": "large.go", "content": large}},
				}},
			},
			check: func(t *testing.T, payload map[string]any) {
				commits, _ := payload["commits"].([]any)
				if len(commits) != 1 || commits[0].(map[string]any)["sha"] != "abc" {
					t.Fatalf("git show payload = %#v", payload)
				}
			},
		},
		{
			name:  "future tool",
			value: map[string]any{"kind": "future", "widgets": searchResults},
			check: func(t *testing.T, payload map[string]any) {
				if payload["kind"] != "future" {
					t.Fatalf("future payload = %#v", payload)
				}
			},
		},
		{
			name: "error",
			value: map[string]any{
				"status": "error", "path": "large.go",
				"error": map[string]any{"code": "retrieval_failed", "message": large},
			},
			check: func(t *testing.T, payload map[string]any) {
				if payload["status"] != "error" || nestedString(payload, "error", "code") != "retrieval_failed" {
					t.Fatalf("error payload = %#v", payload)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, payload := limitedPayload(t, tt.value, 4)
			if payload["truncated"] != true || payload["truncated_note"] == "" {
				t.Fatalf("missing truncation metadata: %#v", payload)
			}
			tt.check(t, payload)
		})
	}
}

func TestToolResultLimiterPreservesExistingTruncationNote(t *testing.T) {
	_, payload := limitedPayload(t, map[string]any{
		"path": "large.go", "content": strings.Repeat("x", 8<<10),
		"truncated": true, "truncated_note": "retrieval clipped source file",
	}, 2)
	note, _ := payload["truncated_note"].(string)
	if !strings.Contains(note, "retrieval clipped source file") || !strings.Contains(note, toolResultTruncatedNote) {
		t.Fatalf("note = %q", note)
	}
}

func TestExecuteToolCallAppliesConfiguredToolResultLimit(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "large.txt", strings.Repeat("content\n", 4<<10))
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test", MaxToolResultKiB: 2})
	result := engine.executeToolCall(context.Background(), repoRoot, llm.ToolCall{
		ID: "inspect", Name: "inspect_file", Arguments: `{"path":"large.txt"}`,
	}, freshToolRoundState())
	if len(result) > 2<<10 {
		t.Fatalf("result = %d bytes", len(result))
	}
	payload := decodeToolPayload(t, result)
	if payload["truncated"] != true || payload["path"] != "large.txt" {
		t.Fatalf("payload = %#v", payload)
	}
}
