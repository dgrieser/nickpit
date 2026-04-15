package retrieval

import (
	"strings"
	"testing"
)

func TestCallHierarchyRenderIncludesSource(t *testing.T) {
	hierarchy := &CallHierarchy{
		Root: CallNode{
			Name:      "Run",
			Path:      "b/b.go",
			StartLine: 3,
			EndLine:   5,
			Source:    "func Run() {\n\ta.Run()\n}",
			Children: []CallNode{
				{
					Name:      "Start",
					Path:      "main.go",
					StartLine: 3,
					EndLine:   5,
					Source:    "func Start() {\n\tb.Run()\n}",
				},
			},
		},
	}

	got := hierarchy.Render()
	wantParts := []string{
		"Run (b/b.go:3-5)\nfunc Run() {\n    a.Run()\n}\n",
		"└── Start (main.go:3-5)\n    func Start() {\n",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("render output missing %q\nfull output:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "        b.Run()") {
		t.Fatalf("render output missing child source body:\n%s", got)
	}
	if strings.Contains(got, "\t") {
		t.Fatalf("render output should not contain tabs:\n%q", got)
	}
	if !strings.Contains(got, "}\n") {
		t.Fatalf("render output missing closing brace:\n%s", got)
	}
}

func TestCallHierarchyRenderJSONIncludesSource(t *testing.T) {
	hierarchy := &CallHierarchy{
		Root: CallNode{
			Name:      "Run",
			Path:      "b/b.go",
			StartLine: 3,
			EndLine:   5,
			Source:    "func Run() {}",
		},
	}

	got := hierarchy.RenderJSON()
	if !strings.Contains(got, "\"source\": \"func Run() {}\"") {
		t.Fatalf("json output missing source field:\n%s", got)
	}
}
