package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/debuglog"
)

func TestClientReview(t *testing.T) {
	var payload map[string]any
	var path string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[{"title":"[P2] Flag issue","body":"Something is wrong","confidence_score":0.9,"priority":2,"code_location":{"absolute_file_path":"/tmp/main.go","line_range":{"start":10,"end":10}}}],"overall_correctness":"patch is incorrect",`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `"overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-3",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		MaxTokens:       10,
		Temperature:     0.25,
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/chat/completions" {
		t.Fatalf("path = %q", path)
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("findings = %d", len(resp.Findings))
	}
	if resp.TokensUsed.TotalTokens != 15 {
		t.Fatalf("total tokens = %d", resp.TokensUsed.TotalTokens)
	}
	if got := payload["reasoning_effort"]; got != "high" {
		t.Fatalf("reasoning_effort = %v", got)
	}
	if got := payload["stream"]; got != true {
		t.Fatalf("stream = %v", got)
	}
	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v", payload["stream_options"])
	}
	if streamOptions["include_usage"] != true {
		t.Fatalf("include_usage = %v", streamOptions["include_usage"])
	}
	responseFormat, ok := payload["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %#v", payload["response_format"])
	}
	if responseFormat["type"] != "json_object" {
		t.Fatalf("response_format.type = %v", responseFormat["type"])
	}
}

func TestClientReviewUsesJSONSchemaWhenProvided(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
		Schema:       FindingsSchema,
		MaxTokens:    10,
	})
	if err != nil {
		t.Fatal(err)
	}

	responseFormat, ok := payload["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %#v", payload["response_format"])
	}
	if responseFormat["type"] != "json_schema" {
		t.Fatalf("response_format.type = %v", responseFormat["type"])
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("json_schema = %#v", responseFormat["json_schema"])
	}
	if jsonSchema["name"] != "review_response" {
		t.Fatalf("json_schema.name = %v", jsonSchema["name"])
	}
	if jsonSchema["strict"] != true {
		t.Fatalf("json_schema.strict = %v", jsonSchema["strict"])
	}
}

func TestClientReviewStreamsReasoningToLogger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"reasoning_content": "Thinking through the diff.",
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-3",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	var buf bytes.Buffer
	client := NewOpenAIClient(server.URL, "token", "model")
	client.SetLogger(debuglog.New(&buf, false, false))

	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	}); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Reasoning...\n") {
		t.Fatalf("reasoning banner missing: %q", got)
	}
	if !strings.Contains(got, "Thinking through the diff.") {
		t.Fatalf("reasoning delta missing: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

func TestClientReviewReturnsSyntheticFindingForInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": "not valid review json",
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Findings) != 1 {
		t.Fatalf("findings = %d", len(resp.Findings))
	}
	if resp.Findings[0].Title != "[P2] Return valid review JSON" {
		t.Fatalf("title = %q", resp.Findings[0].Title)
	}
	if resp.RawResponse != "not valid review json" {
		t.Fatalf("raw_response = %q", resp.RawResponse)
	}
}

func TestClientReviewRetriesRetryableStatus(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "rate limited",
					"type":    "rate_limit_error",
				},
			})
			return
		}

		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	client.retrier.InitialBackoff = 4 * time.Nanosecond
	client.retrier.MaxBackoff = 4 * time.Nanosecond

	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if resp.TokensUsed.TotalTokens != 6 {
		t.Fatalf("total tokens = %d", resp.TokensUsed.TotalTokens)
	}
}

func TestClientReviewRetriesNetworkErrorWhileReadingStream(t *testing.T) {
	attempts := 0
	var logBuf bytes.Buffer

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "text/event-stream")
		if attempts == 1 {
			if _, err := w.Write([]byte("data: ")); err != nil {
				t.Fatalf("write partial chunk: %v", err)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	client.SetLogger(debuglog.New(&logBuf, false, false))
	client.retrier.InitialBackoff = 4 * time.Nanosecond
	client.retrier.MaxBackoff = 4 * time.Nanosecond

	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	}); err != nil {
		t.Fatal(err)
	}

	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if !strings.Contains(logBuf.String(), "LLM stream hit a network error, retrying...") {
		t.Fatalf("retry notice missing: %q", logBuf.String())
	}
}

func TestClientReviewRetriesNetworkErrorOpeningStream(t *testing.T) {
	attempts := 0
	var logBuf bytes.Buffer

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}

		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	client.SetLogger(debuglog.New(&logBuf, false, false))
	client.retrier.InitialBackoff = 4 * time.Nanosecond
	client.retrier.MaxBackoff = 4 * time.Nanosecond

	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	}); err != nil {
		t.Fatal(err)
	}

	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if !strings.Contains(logBuf.String(), "LLM request hit a network error, retrying...") {
		t.Fatalf("network retry notice missing: %q", logBuf.String())
	}
}

func writeSSEChunk(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal chunk: %v", err)
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeSSEDone(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	if _, err := w.Write([]byte("data: [DONE]\n\n")); err != nil {
		t.Fatalf("write done: %v", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
