package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/session"
)

// Exercise the real CLI, streaming provider, local source, update workflow,
// persistence, and resume. No paid model or remote repository is involved.
func TestCLIChatCorrectsAndResumesReview(t *testing.T) {
	for _, ephemeral := range []bool{false, true} {
		t.Run(fmt.Sprintf("no_session=%v", ephemeral), func(t *testing.T) {
			root := t.TempDir()
			for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "test@example.invalid"}, {"config", "user.name", "Test"}} {
				if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("git: %s %v", out, err)
				}
			}
			source := filepath.Join(root, "sample.go")
			if err := os.WriteFile(source, []byte("package sample\nfunc Value() int { return 1 }\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"add", "sample.go"}, {"commit", "-qm", "base"}} {
				if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("git: %s %v", out, err)
				}
			}
			if err := os.WriteFile(source, []byte("package sample\nfunc Value() int { return 2 }\n"), 0600); err != nil {
				t.Fatal(err)
			}
			const findingID = "00000000-0000-4000-8000-000000000001"
			entered, release := make(chan struct{}), make(chan struct{})
			var mu sync.Mutex
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				requests = append(requests, string(body))
				call := len(requests)
				mu.Unlock()
				delta := map[string]any{}
				finish := "stop"
				switch call {
				case 1:
					if !strings.Contains(string(body), "request_review_update") {
						t.Error("CLI did not register correction tool")
					}
					delta["tool_calls"] = []any{map[string]any{"index": 0, "id": "update-call", "type": "function", "function": map[string]any{"name": "request_review_update", "arguments": `{"finding_ids":["` + findingID + `"],"reason":"Current guard prevents the failure."}`}}}
					finish = "tool_calls"
				case 2:
					delta["content"] = "I scheduled an update."
				case 3:
					if strings.Contains(string(body), "I scheduled an update.") || !strings.Contains(string(body), "Please check the guard.") {
						t.Error("update evidence crossed initiating-question cutoff")
					}
					close(entered)
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
					delta["content"] = `{"updates":[{"id":"` + findingID + `","action":"resolved","reason":"Current guard prevents the failure."}]}`
				default:
					if !strings.Contains(string(body), "Current guard prevents the failure.") {
						t.Error("resumed discussion did not see corrected review")
					}
					delta["content"] = "The finding is resolved."
				}
				w.Header().Set("Content-Type", "text/event-stream")
				data, _ := json.Marshal(map[string]any{"id": "test", "object": "chat.completion.chunk", "model": "test", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": nil}}})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				end, _ := json.Marshal(map[string]any{"id": "test", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}})
				_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", end)
			}))
			defer server.Close()
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			cfg := fmt.Sprintf("profiles:\n  default:\n    base_url: %s\n    api_key: test\n    model: test\n    max_tool_calls: 5\n    max_output_retries: 1\n    disable_patch_summary: true\n", server.URL)
			if err := os.WriteFile(configPath, []byte(cfg), 0600); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			store, _ := session.NewStore(dir)
			sess := session.New()
			rank := 1
			sess.ReviewID = "review"
			sess.Source = session.Source{Mode: "local", Submode: "uncommitted", RepoRoot: root}
			sess.Result = &model.ReviewResult{ReviewID: "review", Model: "test", Findings: []model.Finding{{ID: findingID, Title: "Guard missing", Body: "Old evidence", Priority: &rank, ConfidenceScore: 0.9, CodeLocation: model.CodeLocation{FilePath: "sample.go", LineRange: model.LineRange{Start: 2, End: 2}, Content: "func Value() int { return 2 }"}}}, OverallCorrectness: "patch is incorrect", OverallExplanation: "Old verdict."}
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			stdout, err := os.CreateTemp(t.TempDir(), "output")
			if err != nil {
				t.Fatal(err)
			}
			oldStdout := os.Stdout
			os.Stdout = stdout
			defer func() { os.Stdout = oldStdout; _ = stdout.Close() }()
			a := &app{configPath: configPath, profile: "default", sessionDir: dir, noSession: ephemeral, priorityThreshold: "3", confidenceThreshold: 0.7}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- a.runChat(ctx, chatOptions{sessionID: sess.ID}, []string{"Please check the guard."}) }()
			select {
			case <-entered:
			case err := <-done:
				t.Fatalf("chat ended before update: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err := <-done:
				t.Fatalf("one-shot exited while update pending: %v", err)
			default:
			}
			close(release)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			loaded, err := store.Load(sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			if ephemeral {
				if loaded.Result.Revision != 0 || len(loaded.Messages) != 0 || len(loaded.ReviewHistory) != 0 {
					t.Fatal("--no-session persisted changes")
				}
			} else {
				if loaded.Result.Findings[0].Resolution == nil || loaded.Result.Revision != 1 || len(loaded.ReviewHistory) != 1 {
					t.Fatalf("correction not saved: %+v", loaded)
				}
				if loaded.ReviewHistory[0].Result.Findings[0].Resolution != nil {
					t.Fatal("archive includes new resolution")
				}
				if err := a.runChat(ctx, chatOptions{sessionID: sess.ID}, []string{"What is the current state?"}); err != nil {
					t.Fatal(err)
				}
			}
			printed, err := os.ReadFile(stdout.Name())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Index(string(printed), "I scheduled an update.") > strings.Index(string(printed), "Review updated.") || !strings.Contains(string(printed), "Review updated.") {
				t.Fatalf("wrong completion order: %s", printed)
			}
		})
	}
}
