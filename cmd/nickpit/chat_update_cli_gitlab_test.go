package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/session"
)

func TestCLIUpdatePublishesGitLabReviewWithoutMirroringChat(t *testing.T) {
	for _, tc := range []struct {
		name               string
		overall, unchanged bool
	}{{name: "resolution"}, {name: "overall", overall: true}, {name: "unchanged verdict", overall: true, unchanged: true}} {
		t.Run(tc.name, func(t *testing.T) {
			overall := tc.overall
			before := cliTestReview()
			before.RuntimeSeconds = 42 // Local telemetry is absent from GitLab carriers.
			if tc.unchanged {
				before.OverallExplanation = "Short corrected verdict."
				before.OverallConfidenceScore = 0.9
			}
			before.Findings[0].CodeLocation = model.CodeLocation{FilePath: "sample.go", LineRange: model.LineRange{Start: 1, End: 1}, Content: "safe()"}
			before.Findings[0].Priority = new(int)
			before.Findings[0].ConfidenceScore = 0.9
			render := reviewmd.NewRenderer("").ForReview(before.ReviewID)
			root, _ := render.SummaryBodyCarried(before)
			finding, _ := render.FindingBodyCarried(before.Findings[0], "")
			var mu sync.Mutex
			notes := map[int]string{1: root, 2: finding}
			next, visibleWrites, models := 10, 0, 0
			noteJSON := func(id int, body string) map[string]any {
				return map[string]any{"id": id, "body": body, "author": map[string]any{"id": 7, "username": "nickpit"}}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if strings.HasSuffix(r.URL.Path, "/chat/completions") {
					models++
					var raw json.RawMessage
					if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
						t.Error(err)
						return
					}
					if (!strings.Contains(r.URL.Path, "/small/") && !strings.Contains(string(raw), "PRIVATE CLI QUESTION")) || strings.Contains(string(raw), "UNRELATED REMOTE THREAD") {
						t.Error("incorrect correction evidence")
					}
					response := `{"updates":[{"id":"finding","action":"resolved","reason":"Current guard prevents the failure."}]}`
					if overall {
						switch models {
						case 1:
							response = `{"updates":[],"review":{"action":"correction_warranted","reason":"The overall explanation omits current evidence."}}`
						case 2:
							response = `{"overall_correctness":"patch is incorrect","overall_explanation":"Updated verdict based on the initiating question."}`
						case 3:
							if !strings.Contains(r.URL.Path, "/small/") || r.Header.Get("Authorization") != "Bearer small-token" {
								t.Error("summary used wrong model endpoint or credential")
							}
							response = `{"findings":[{"id":"00000000-0000-4000-8000-0000000a11ee","summarization":{"body":"Short corrected verdict."}}]}`
						default:
							t.Error("unexpected extra model call")
						}
					}
					w.Header().Set("Content-Type", "text/event-stream")
					data, _ := json.Marshal(map[string]any{"id": "test", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": response}, "finish_reason": "stop"}}})
					_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
				if r.URL.Path == "/api/v4/user" {
					write(map[string]any{"id": 7, "username": "nickpit"})
					return
				}
				const base = "/api/v4/projects/g/p/merge_requests/1"
				tail, ok := strings.CutPrefix(r.URL.Path, base)
				if !ok {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if r.Method == http.MethodGet {
					switch tail {
					case "":
						write(map[string]any{"title": "Review", "sha": "head", "diff_refs": map[string]string{"base_sha": "base", "head_sha": "head", "start_sha": "base"}})
						return
					case "/commits":
						write([]any{})
						return
					case "/changes":
						write(map[string]any{"changes": []any{map[string]any{"old_path": "sample.go", "new_path": "sample.go", "diff": "@@ -1 +1 @@\n-old()\n+safe()"}}})
						return
					case "/discussions":
						ds := []any{}
						for id, body := range notes {
							ds = append(ds, map[string]any{"id": fmt.Sprintf("d%d", id), "notes": []any{noteJSON(id, body)}})
						}
						ds = append(ds, map[string]any{"id": "unrelated", "notes": []any{map[string]any{"id": 1000, "body": "UNRELATED REMOTE THREAD", "author": map[string]any{"id": 8}}}})
						write(ds)
						return
					case "/notes":
						ns := []any{}
						for id, body := range notes {
							ns = append(ns, noteJSON(id, body))
						}
						write(ns)
						return
					}
				}
				if r.Method == http.MethodDelete && strings.HasPrefix(tail, "/notes/") {
					id, _ := strconv.Atoi(strings.TrimPrefix(tail, "/notes/"))
					delete(notes, id)
					w.WriteHeader(204)
					return
				}
				var payload struct {
					Body string `json:"body"`
				}
				if r.Method == http.MethodPost || r.Method == http.MethodPut {
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						http.Error(w, "bad JSON", 400)
						return
					}
					if strings.Contains(payload.Body, "PRIVATE CLI QUESTION") {
						t.Error("mirrored terminal conversation")
					}
				}
				if r.Method == http.MethodPost && tail == "/notes" {
					if reviewmd.StripMarkers(payload.Body) != "" {
						t.Error("created visible conversation post")
					}
					next++
					notes[next] = payload.Body
					write(noteJSON(next, payload.Body))
					return
				}
				if r.Method == http.MethodPut {
					for id := range notes {
						if tail == fmt.Sprintf("/discussions/d%d/notes/%d", id, id) {
							notes[id] = payload.Body
							visibleWrites++
							write(noteJSON(id, payload.Body))
							return
						}
					}
				}
				t.Errorf("unexpected write/thread/emoji request: %s %s", r.Method, tail)
				http.NotFound(w, r)
			}))
			defer server.Close()
			profile := config.Profile{Model: "test", APIKey: "test", BaseURL: server.URL + "/llm", GitLabBaseURL: server.URL, GitLabToken: "test", MaxToolCalls: -1, DisablePatchSummary: true, MaxOutputRetries: 1}
			if overall {
				profile.DisablePatchSummary = false
				profile.Small = config.SmallModelConfig{Model: "small", BaseURL: server.URL + "/small", APIKey: "small-token"}
			}
			input := cliUpdateInput{Source: session.Source{Mode: "gitlab", Repo: "g/p", Identifier: 1, BaseURL: server.URL, RepoRoot: t.TempDir()}, Result: before, Messages: []llm.Message{{Role: "user", Content: "PRIVATE CLI QUESTION"}}, Question: "PRIVATE CLI QUESTION", Signal: review.ReviewUpdateSignal{FindingIDs: []string{"finding"}, Reason: "Check guard."}}
			if overall {
				input.Signal.FindingIDs = nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out := (&app{priorityThreshold: "3"}).runCLIReviewUpdate(ctx, profile, input, true)
			if out.Err != nil {
				t.Fatal(out.Err)
			}
			wantRevision := uint64(1)
			if tc.unchanged {
				wantRevision = 0
			}
			if out.Result == nil || out.Result.Revision != wantRevision || ((out.Result.Findings[0].Resolution == nil) != overall) {
				t.Fatalf("wrong published result: %+v", out)
			}
			if out.Result.RuntimeSeconds != before.RuntimeSeconds {
				t.Fatal("publication discarded local metadata")
			}
			mu.Lock()
			defer mu.Unlock()
			wantWrites, wantModels := 2, 1
			if overall {
				wantWrites, wantModels = 1, 3
			}
			if tc.unchanged {
				wantWrites = 0
				sess := &session.Session{Result: before}
				if err := sess.RecordReviewUpdate(out.Result, "Rechecked verdict"); err != nil || len(sess.ReviewHistory) != 0 {
					t.Fatalf("no-op created session history: %+v %v", sess.ReviewHistory, err)
				}
			}
			if visibleWrites != wantWrites || models != wantModels {
				t.Fatalf("writes=%d models=%d", visibleWrites, models)
			}
			wantFindingHistory := 1
			if overall {
				wantFindingHistory = 0
				if out.Result.OverallExplanation != "Short corrected verdict." {
					t.Fatal("small-model summary not applied")
				}
			}
			if len(reviewmd.ReadHistory(notes[1]).Entries) != int(wantRevision) || len(reviewmd.ReadHistory(notes[2]).Entries) != wantFindingHistory {
				t.Fatal("published revisions lost history")
			}
		})
	}
}

func TestCLIUpdateSharesDaemonExecutionLock(t *testing.T) {
	client := glscm.NewClient("https://gitlab.example.invalid", "test")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, release, err := client.TryLockMR(ctx, "update-execution/g/p", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	second := glscm.NewClient("https://gitlab.example.invalid", "test")
	_, unlock, err := second.TryLockMR(ctx, "update-execution/g/p", 1)
	if unlock != nil {
		unlock()
	}
	if err != glscm.ErrLockBusy {
		t.Fatalf("execution lock did not serialize independent clients: %v", err)
	}
}
