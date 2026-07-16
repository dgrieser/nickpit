package retrieval

import (
	"strings"
	"testing"
)

func TestCallHierarchyRenderIncludesSource(t *testing.T) {
	hierarchy := &CallHierarchy{
		Root: CallNode{
			Name:         "Run",
			CodeLocation: callNodeLocation("b/b.go", 3, 5, "func Run() {\n\ta.Run()\n}"),
			Children: []CallNode{
				{
					Name:         "Start",
					CodeLocation: callNodeLocation("main.go", 3, 5, "func Start() {\n\tb.Run()\n}"),
				},
				{
					Name:         "Finish",
					CodeLocation: callNodeLocation("main.go", 7, 9, "func Finish() {\n\tcleanup()\n}"),
				},
			},
		},
	}

	got := hierarchy.Render()
	wantParts := []string{
		"Run (b/b.go:3-5)\nfunc Run() {\n    a.Run()\n}\n",
		"├── Start (main.go:3-5)\n│   func Start() {\n",
		"│       b.Run()\n│   }\n│   \n└── Finish (main.go:7-9)\n    func Finish() {\n",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("render output missing %q\nfull output:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "    cleanup()") {
		t.Fatalf("render output missing second child source body:\n%s", got)
	}
	if strings.Contains(got, "\t") {
		t.Fatalf("render output should not contain tabs:\n%q", got)
	}
	if !strings.Contains(got, "}\n") {
		t.Fatalf("render output missing closing brace:\n%s", got)
	}
}

func TestCallHierarchyRenderJSONIncludesCodeLocation(t *testing.T) {
	hierarchy := &CallHierarchy{
		Root: CallNode{
			Name:         "Run",
			CodeLocation: callNodeLocation("b/b.go", 3, 5, "func Run() {}"),
		},
	}

	got := hierarchy.RenderJSON()
	if !strings.Contains(got, "\"content\": \"func Run() {}\"") {
		t.Fatalf("json output missing code_location content field:\n%s", got)
	}
	if !strings.Contains(got, "\"file_path\": \"b/b.go\"") {
		t.Fatalf("json output missing code_location file_path field:\n%s", got)
	}
	if !strings.Contains(got, "\"count\": 3") {
		t.Fatalf("json output missing code_location line_range count field:\n%s", got)
	}
}

func TestCallNodeLocationDerivesLanguageAndCount(t *testing.T) {
	loc := callNodeLocation("pkg/a.go", 10, 12, "func A() {}")
	if loc.Language != "go" {
		t.Fatalf("language = %q, want go", loc.Language)
	}
	if loc.LineRange != (LineRange{Start: 10, End: 12, Count: 3}) {
		t.Fatalf("line range = %+v", loc.LineRange)
	}
	if zero := callNodeLocation("pkg/a.go", 0, 0, ""); zero.LineRange.Count != 0 {
		t.Fatalf("zero-range count = %d, want 0", zero.LineRange.Count)
	}
}
