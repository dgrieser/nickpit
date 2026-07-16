package serve

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/serve/loki"
)

// TestExecRunnerStreamsToRealLokiClient exercises the full production tee path:
// a real ExecRunner running a child process, tee'd through NewLokiSink and an
// actual loki.Client that pushes over HTTP to a stand-in Loki server. No mocks
// between the runner and the wire.
func TestExecRunnerStreamsToRealLokiClient(t *testing.T) {
	type pushBody struct {
		Streams []struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"`
		} `json:"streams"`
	}

	var mu sync.Mutex
	var lines []string
	var labels map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/push" {
			t.Errorf("unexpected push path %q", r.URL.Path)
		}
		var body pushBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		for _, s := range body.Streams {
			if labels == nil {
				labels = s.Stream
			}
			for _, v := range s.Values {
				lines = append(lines, v[1])
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := loki.NewClient(loki.Config{
		URL:          srv.URL,
		StaticLabels: map[string]string{"env": "test"},
		BatchWait:    20 * time.Millisecond,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	runner := &ExecRunner{Executable: writeFakeReview(t), sink: NewLokiSink(client), now: time.Now}
	spec := testSpec(t)
	spec.HeadSHA = "cafebabe"
	spec.Trigger = "manual"

	exitCode, _, err := runner.Run(context.Background(), spec)
	if err != nil || exitCode != 0 {
		t.Fatalf("run: exit=%d err=%v", exitCode, err)
	}

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "review head=cafebabe") {
		t.Fatalf("missing head-SHA preamble line:\n%s", joined)
	}
	if !strings.Contains(joined, "args:gitlab mr --repo platform/api") {
		t.Fatalf("child output not streamed to Loki:\n%s", joined)
	}
	for k, want := range map[string]string{"app": "nickpit", "env": "test", "project": "platform/api", "iid": "7", "trigger": "manual"} {
		if labels[k] != want {
			t.Errorf("label %s = %q, want %q", k, labels[k], want)
		}
	}
}
