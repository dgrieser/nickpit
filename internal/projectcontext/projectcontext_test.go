package projectcontext

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

const validYAML = `
version: 1
summary: Billing API
deployment: internet-facing
users: authenticated customers
criticality: high
data:
  - card data
trust_boundaries:
  - handlers accept untrusted bodies
assumptions:
  - proxy terminates TLS
non_goals:
  - single region only
notes: |
  Extra prose.
`

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
		check   func(*testing.T, *model.ProjectContext)
	}{
		{
			name: "full document",
			body: validYAML,
			check: func(t *testing.T, pc *model.ProjectContext) {
				if pc.Deployment != "internet-facing" || pc.Criticality != "high" {
					t.Fatalf("scalars = %+v", pc)
				}
				if len(pc.TrustBoundaries) != 1 || pc.TrustBoundaries[0] != "handlers accept untrusted bodies" {
					t.Fatalf("trust boundaries = %v", pc.TrustBoundaries)
				}
				if pc.Notes != "Extra prose." {
					t.Fatalf("notes = %q", pc.Notes)
				}
				if len(pc.Sources) != 1 || pc.Sources[0] != "src" {
					t.Fatalf("sources = %v", pc.Sources)
				}
			},
		},
		{
			name: "version omitted defaults to current",
			body: "summary: A tool\n",
			check: func(t *testing.T, pc *model.ProjectContext) {
				if pc.Version != Version {
					t.Fatalf("version = %d", pc.Version)
				}
			},
		},
		{
			name: "single field is enough",
			body: "deployment: cli\n",
			check: func(t *testing.T, pc *model.ProjectContext) {
				if pc.Deployment != "cli" {
					t.Fatalf("deployment = %q", pc.Deployment)
				}
			},
		},
		{
			name:    "unknown key rejected",
			body:    "summary: x\ndeploymnet: internet-facing\n",
			wantErr: "deploymnet",
		},
		{
			// A repository must not be able to claim its context came from an
			// operator override.
			name:    "sources cannot be forged",
			body:    "summary: x\nsources: [trusted-operator-file]\n",
			wantErr: "sources",
		},
		{
			name:    "unsupported version",
			body:    "version: 2\nsummary: x\n",
			wantErr: "unsupported version 2",
		},
		{
			name:    "empty document",
			body:    "",
			wantErr: "is empty",
		},
		{
			name:    "comments only",
			body:    "# nothing here\n",
			wantErr: "is empty",
		},
		{
			name:    "no fields set",
			body:    "version: 1\n",
			wantErr: "declares no fields",
		},
		{
			name:    "not yaml",
			body:    "summary: [unclosed\n",
			wantErr: "project context",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(tc.body), "src")
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Parse() = %+v, want error containing %q", got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Parse() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() returned err: %v", err)
			}
			tc.check(t, got)
		})
	}
}

func TestParseRejectsOversizedAndBinary(t *testing.T) {
	oversized := "summary: " + strings.Repeat("x", MaxBytes) + "\n"
	if _, err := Parse([]byte(oversized), "src"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized error = %v, want an 'exceeds' error", err)
	}
	if _, err := Parse([]byte("summary: a\x00b\n"), "src"); err == nil || !strings.Contains(err.Error(), "not text") {
		t.Fatalf("binary error = %v, want a 'not text' error", err)
	}
}

func TestParseNormalizesListsAndScalars(t *testing.T) {
	got, err := Parse([]byte("summary: \"  spaced  \"\ndata: [\"  a  \", \"\", a, b]\n"), "src")
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "spaced" {
		t.Fatalf("summary = %q", got.Summary)
	}
	if len(got.Data) != 2 || got.Data[0] != "a" || got.Data[1] != "b" {
		t.Fatalf("data = %v, want trimmed, empty-dropped and deduped", got.Data)
	}
}

func TestMerge(t *testing.T) {
	repo := &model.ProjectContext{
		Summary:         "repo summary",
		Deployment:      "internal",
		Data:            []string{"pii"},
		TrustBoundaries: []string{"a"},
		Sources:         []string{RepoPath},
	}
	overlay := &model.ProjectContext{
		Deployment:      "internet-facing",
		Criticality:     "high",
		TrustBoundaries: []string{"a", "b"},
		Sources:         []string{"ops.yaml"},
	}

	got := Merge(repo, overlay)
	if got.Summary != "repo summary" {
		t.Fatalf("summary = %q, want the repo value kept when the overlay is silent", got.Summary)
	}
	if got.Deployment != "internet-facing" {
		t.Fatalf("deployment = %q, want the overlay to win", got.Deployment)
	}
	if got.Criticality != "high" {
		t.Fatalf("criticality = %q, want the overlay value added", got.Criticality)
	}
	if len(got.TrustBoundaries) != 2 || got.TrustBoundaries[0] != "a" || got.TrustBoundaries[1] != "b" {
		t.Fatalf("trust boundaries = %v, want concatenated and deduped in order", got.TrustBoundaries)
	}
	if len(got.Data) != 1 || got.Data[0] != "pii" {
		t.Fatalf("data = %v, want the repo list preserved", got.Data)
	}
	if len(got.Sources) != 2 || got.Sources[0] != RepoPath || got.Sources[1] != "ops.yaml" {
		t.Fatalf("sources = %v, want both in application order", got.Sources)
	}
	// The inputs must survive: engine state holds the overlay across a whole run.
	if len(repo.TrustBoundaries) != 1 {
		t.Fatalf("Merge mutated its first argument: %v", repo.TrustBoundaries)
	}
}

func TestMergeEdgeCases(t *testing.T) {
	if got := Merge(); got != nil {
		t.Fatalf("Merge() = %+v, want nil", got)
	}
	if got := Merge(nil, nil); got != nil {
		t.Fatalf("Merge(nil, nil) = %+v, want nil", got)
	}
	if got := Merge(&model.ProjectContext{Version: Version}); got != nil {
		t.Fatalf("Merge(empty) = %+v, want nil so no heading is rendered", got)
	}
	only := &model.ProjectContext{Summary: "x"}
	if got := Merge(nil, only); got == nil || got.Summary != "x" {
		t.Fatalf("Merge(nil, entry) = %+v", got)
	}
}

func TestResolveFileAndURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.yaml")
	if err := os.WriteFile(path, []byte("criticality: critical\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("deployment: internal\n"))
	}))
	defer server.Close()

	got, err := Resolve(context.Background(), []string{"ops.yaml", server.URL}, dir)
	if err != nil {
		t.Fatalf("Resolve() returned err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Resolve() returned %d entries, want 2", len(got))
	}
	if got[0].Criticality != "critical" || got[1].Deployment != "internal" {
		t.Fatalf("Resolve() = %+v, %+v", got[0], got[1])
	}
	if got[0].Sources[0] != "ops.yaml" {
		t.Fatalf("source = %v, want the spec as written", got[0].Sources)
	}
}

func TestResolveFailsFast(t *testing.T) {
	dir := t.TempDir()
	tests := map[string]struct {
		specs []string
		want  string
	}{
		"missing file":  {specs: []string{"absent.yaml"}, want: "absent.yaml"},
		"directory":     {specs: []string{"."}, want: "is a directory"},
		"malformed":     {specs: []string{"bad.yaml"}, want: "bad.yaml"},
		"http failure":  {specs: []string{"https://127.0.0.1:1/x.yaml"}, want: "project context"},
		"empty entries": {specs: []string{"  "}, want: ""},
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("nope: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Resolve(context.Background(), tc.specs, dir)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Resolve() returned err: %v", err)
				}
				if len(got) != 0 {
					t.Fatalf("Resolve() = %+v, want blank specs skipped", got)
				}
				return
			}
			if err == nil {
				t.Fatalf("Resolve() = %+v, want an error", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Resolve() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestResolveRejectsOversizedURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("summary: " + strings.Repeat("x", MaxBytes) + "\n"))
	}))
	defer server.Close()
	if _, err := Resolve(context.Background(), []string{server.URL}, ""); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want an 'exceeds' error", err)
	}
}

// stubSource satisfies model.ReviewSource without a BaseFileSource; LoadRepo
// must treat it as "no context", not as a failure.
type stubSource struct{}

func (stubSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return nil, nil
}

type stubBaseFileSource struct {
	stubSource
	data  []byte
	found bool
	err   error
	path  string
}

func (s *stubBaseFileSource) ReadBaseFile(_ context.Context, _ model.ReviewRequest, path string) ([]byte, bool, error) {
	s.path = path
	return s.data, s.found, s.err
}

func TestLoadRepo(t *testing.T) {
	t.Run("source without a base reader", func(t *testing.T) {
		got, warnings := LoadRepo(context.Background(), stubSource{}, model.ReviewRequest{})
		if got != nil || warnings != nil {
			t.Fatalf("LoadRepo() = %+v, %v; want no context and no warning", got, warnings)
		}
	})
	t.Run("file absent", func(t *testing.T) {
		got, warnings := LoadRepo(context.Background(), &stubBaseFileSource{}, model.ReviewRequest{})
		if got != nil || warnings != nil {
			t.Fatalf("LoadRepo() = %+v, %v; want no context and no warning", got, warnings)
		}
	})
	t.Run("file present", func(t *testing.T) {
		src := &stubBaseFileSource{data: []byte(validYAML), found: true}
		got, warnings := LoadRepo(context.Background(), src, model.ReviewRequest{})
		if len(warnings) != 0 {
			t.Fatalf("warnings = %v", warnings)
		}
		if got == nil || got.Deployment != "internet-facing" {
			t.Fatalf("LoadRepo() = %+v", got)
		}
		if src.path != RepoPath {
			t.Fatalf("read path = %q, want %q", src.path, RepoPath)
		}
	})
	t.Run("malformed file degrades rather than fails", func(t *testing.T) {
		src := &stubBaseFileSource{data: []byte("nope: 1\n"), found: true}
		got, warnings := LoadRepo(context.Background(), src, model.ReviewRequest{})
		if got != nil {
			t.Fatalf("LoadRepo() = %+v, want no context", got)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "ignored") {
			t.Fatalf("warnings = %v, want one 'ignored' warning", warnings)
		}
	})
	t.Run("read error degrades rather than fails", func(t *testing.T) {
		src := &stubBaseFileSource{err: os.ErrPermission}
		got, warnings := LoadRepo(context.Background(), src, model.ReviewRequest{})
		if got != nil {
			t.Fatalf("LoadRepo() = %+v, want no context", got)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "not read") {
			t.Fatalf("warnings = %v, want one 'not read' warning", warnings)
		}
	})
}
