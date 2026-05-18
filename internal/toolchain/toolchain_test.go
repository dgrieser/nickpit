package toolchain

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dgrieser/nickpit/internal/model"
)

func ctxWithChangedFiles(paths ...string) *model.ReviewContext {
	files := make([]model.ChangedFile, 0, len(paths))
	for _, path := range paths {
		files = append(files, model.ChangedFile{Path: path, Status: model.FileModified})
	}
	return &model.ReviewContext{ChangedFiles: files}
}

func TestRelevantLanguagesOnlyForChangedFiles(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{"go only", []string{"main.go"}, []string{langGo}},
		{"python only", []string{"app.py", "README.md"}, []string{langPython}},
		{"javascript", []string{"web/app.js"}, []string{langJavaScript}},
		{"typescript", []string{"web/app.ts"}, []string{langTypeScript}},
		{"mixed", []string{"a.go", "b.js", "c.ts", "d.py"}, []string{langGo, langPython, langJavaScript, langTypeScript}},
		{"unrelated only", []string{"docs/README.md", "infra/values.yaml"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := relevantLanguages(ctxWithChangedFiles(tc.paths...))
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("relevantLanguages = %v, want %v", got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("relevantLanguages = %v, want %v", got, want)
				}
			}
		})
	}
}

func TestScanFSEmitsUnavailableWhenNoSources(t *testing.T) {
	fsys := fstest.MapFS{}
	got := ScanFS(fsys, ctxWithChangedFiles("main.go", "app.py", "web/app.ts"))
	if len(got) != 3 {
		t.Fatalf("entries = %#v", got)
	}
	for _, entry := range got {
		if !entry.Unavailable {
			t.Errorf("%s should be unavailable: %#v", entry.Language, entry)
		}
		if entry.Source != "" || entry.Version != "" {
			t.Errorf("%s should be empty: %#v", entry.Language, entry)
		}
	}
}

func TestScanFSNoRelevantLanguagesReturnsNil(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module x\n\ngo 1.22\n")},
	}
	if got := ScanFS(fsys, ctxWithChangedFiles("README.md")); got != nil {
		t.Fatalf("entries = %#v, want nil", got)
	}
}

func TestScanGoMod(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/foo\n\ngo 1.22\n\ntoolchain go1.22.4\n")},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("main.go"))
	if len(got) != 2 {
		t.Fatalf("entries = %#v", got)
	}
	want := []model.ToolchainVersion{
		{Language: langGo, Source: "go.mod", Field: "go", Version: "1.22"},
		{Language: langGo, Source: "go.mod", Field: "toolchain", Version: "go1.22.4"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %#v, want %#v", got, want)
	}
}

func TestScanToolVersions(t *testing.T) {
	body := "# comment\nnodejs 20.10.0\npython 3.11.5\ngolang 1.22.0\nruby 3.2.0\n"
	fsys := fstest.MapFS{".tool-versions": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("main.go", "app.py", "web/app.js"))
	versions := versionsBySourceField(got)
	if versions[".tool-versions|nodejs"] != "20.10.0" {
		t.Errorf("nodejs = %v", versions)
	}
	if versions[".tool-versions|python"] != "3.11.5" {
		t.Errorf("python = %v", versions)
	}
	if versions[".tool-versions|golang"] != "1.22.0" {
		t.Errorf("golang = %v", versions)
	}
}

func TestScanPythonAndNodeVersionAndNvmrc(t *testing.T) {
	fsys := fstest.MapFS{
		".python-version": &fstest.MapFile{Data: []byte("3.10.5\n")},
		".nvmrc":          &fstest.MapFile{Data: []byte("v20.11.0\n")},
		".node-version":   &fstest.MapFile{Data: []byte("20.11.0\n")},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py", "a.js"))
	versions := versionsBySourceField(got)
	if versions[".python-version|"] != "3.10.5" {
		t.Errorf(".python-version = %v", versions)
	}
	if versions[".nvmrc|"] != "v20.11.0" {
		t.Errorf(".nvmrc = %v", versions)
	}
	if versions[".node-version|"] != "20.11.0" {
		t.Errorf(".node-version = %v", versions)
	}
}

func TestScanRuntimeTxt(t *testing.T) {
	fsys := fstest.MapFS{"runtime.txt": &fstest.MapFile{Data: []byte("python-3.10.13\n")}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "runtime.txt", "") != "3.10.13" {
		t.Fatalf("entries = %#v", got)
	}
}

func TestScanPipfile(t *testing.T) {
	body := "[[source]]\nurl = \"https://pypi.org\"\n\n[requires]\npython_version = \"3.11\"\n"
	fsys := fstest.MapFS{"Pipfile": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "Pipfile", "python_version") != "3.11" {
		t.Fatalf("entries = %#v", got)
	}
}

func TestScanPyproject(t *testing.T) {
	body := `[project]
name = "x"
requires-python = ">=3.9,<3.13"

[tool.poetry.dependencies]
python = "^3.11"
`
	fsys := fstest.MapFS{"pyproject.toml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "pyproject.toml", "requires-python") != ">=3.9,<3.13" {
		t.Fatalf("requires-python missing: %#v", got)
	}
	if findVersion(got, "pyproject.toml", "tool.poetry.dependencies.python") != "^3.11" {
		t.Fatalf("poetry python missing: %#v", got)
	}
}

func TestScanSetupCfgAndSetupPy(t *testing.T) {
	fsys := fstest.MapFS{
		"setup.cfg": &fstest.MapFile{Data: []byte("[options]\npython_requires = >=3.8\n")},
		"setup.py":  &fstest.MapFile{Data: []byte("setup(name='x', python_requires='>=3.9')\n")},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "setup.cfg", "python_requires") != ">=3.8" {
		t.Errorf("setup.cfg missing: %#v", got)
	}
	if findVersion(got, "setup.py", "python_requires") != ">=3.9" {
		t.Errorf("setup.py missing: %#v", got)
	}
}

func TestScanPackageJSON(t *testing.T) {
	body := `{
  "engines": {"node": ">=18"},
  "devDependencies": {"typescript": "^5.4.0"}
}`
	fsys := fstest.MapFS{"package.json": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.js", "a.ts"))
	if findVersion(got, "package.json", "engines.node") != ">=18" {
		t.Errorf("node engine missing: %#v", got)
	}
	if findVersion(got, "package.json", "devDependencies.typescript") != "^5.4.0" {
		t.Errorf("typescript missing: %#v", got)
	}
}

func TestScanPackageLockJSON(t *testing.T) {
	body := `{
  "packages": {
    "node_modules/typescript": {"version": "5.4.5"}
  }
}`
	fsys := fstest.MapFS{"package-lock.json": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.ts"))
	if findVersion(got, "package-lock.json", "packages.node_modules/typescript") != "5.4.5" {
		t.Fatalf("entries = %#v", got)
	}
}

func TestScanYarnLock(t *testing.T) {
	body := `# yarn lockfile v1

"typescript@^5.4.0":
  version "5.4.5"
  resolved "https://example/typescript-5.4.5.tgz"
`
	fsys := fstest.MapFS{"yarn.lock": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.ts"))
	if findVersion(got, "yarn.lock", "typescript") != "5.4.5" {
		t.Fatalf("entries = %#v", got)
	}
}

func TestScanPnpmLock(t *testing.T) {
	body := `lockfileVersion: '6.0'
packages:
  /typescript@5.4.5:
    resolution: {integrity: sha512-abc}
`
	fsys := fstest.MapFS{"pnpm-lock.yaml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.ts"))
	if findVersion(got, "pnpm-lock.yaml", "typescript") != "5.4.5" {
		t.Fatalf("entries = %#v", got)
	}
}

func TestScanDockerfile(t *testing.T) {
	body := `FROM golang:1.22-alpine AS build
RUN go build
FROM python:3.11-slim
COPY . .
FROM node:20-bookworm
`
	fsys := fstest.MapFS{"Dockerfile": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.go", "a.py", "a.js"))
	if findVersion(got, "Dockerfile", "FROM golang") != "1.22-alpine" {
		t.Errorf("golang missing: %#v", got)
	}
	if findVersion(got, "Dockerfile", "FROM python") != "3.11-slim" {
		t.Errorf("python missing: %#v", got)
	}
	if findVersion(got, "Dockerfile", "FROM node") != "20-bookworm" {
		t.Errorf("node missing: %#v", got)
	}
}

func TestScanGitlabCI(t *testing.T) {
	body := `image: golang:1.22
stages:
  - test
test:
  image: "python:3.11"
`
	fsys := fstest.MapFS{".gitlab-ci.yml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.go", "a.py"))
	if findVersion(got, ".gitlab-ci.yml", "image golang") != "1.22" {
		t.Errorf("golang image missing: %#v", got)
	}
	if findVersion(got, ".gitlab-ci.yml", "image python") != "3.11" {
		t.Errorf("python image missing: %#v", got)
	}
}

func TestScanGithubWorkflows(t *testing.T) {
	body := `name: ci
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22.4'
      - uses: actions/setup-python@v5
        with:
          python-version: 3.11
      - uses: actions/setup-node@v4
        with:
          node-version: 20.x
`
	fsys := fstest.MapFS{".github/workflows/ci.yml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.go", "a.py", "a.js"))
	if findVersion(got, ".github/workflows/ci.yml", "go-version") != "1.22.4" {
		t.Errorf("go-version missing: %#v", got)
	}
	if findVersion(got, ".github/workflows/ci.yml", "python-version") != "3.11" {
		t.Errorf("python-version missing: %#v", got)
	}
	if findVersion(got, ".github/workflows/ci.yml", "node-version") != "20.x" {
		t.Errorf("node-version missing: %#v", got)
	}
}

func TestScanGithubWorkflowsSkipsMatrixAndTemplate(t *testing.T) {
	body := `name: ci
jobs:
  matrix:
    strategy:
      matrix:
        node-version: [18, 20]
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}
      - uses: actions/setup-python@v5
        with:
          python-version: "3.12"
`
	fsys := fstest.MapFS{".github/workflows/ci.yml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.go", "a.py", "a.js"))
	if v := findVersion(got, ".github/workflows/ci.yml", "node-version"); v != "" {
		t.Errorf("node-version list value should be skipped, got %q", v)
	}
	if v := findVersion(got, ".github/workflows/ci.yml", "go-version"); v != "" {
		t.Errorf("go-version template should be skipped, got %q", v)
	}
	if v := findVersion(got, ".github/workflows/ci.yml", "python-version"); v != "3.12" {
		t.Errorf("python-version should be captured, got %q", v)
	}
}

func TestScanPipfileSingleQuoted(t *testing.T) {
	body := "[requires]\npython_version = '3.11'\n"
	fsys := fstest.MapFS{"Pipfile": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "Pipfile", "python_version") != "3.11" {
		t.Fatalf("single-quoted python_version not captured: %#v", got)
	}
}

func TestScanPyprojectSingleQuoted(t *testing.T) {
	body := "[project]\nrequires-python = '>=3.10'\n\n[tool.poetry.dependencies]\npython = '^3.11'\n"
	fsys := fstest.MapFS{"pyproject.toml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "pyproject.toml", "requires-python") != ">=3.10" {
		t.Errorf("single-quoted requires-python missing: %#v", got)
	}
	if findVersion(got, "pyproject.toml", "tool.poetry.dependencies.python") != "^3.11" {
		t.Errorf("single-quoted poetry python missing: %#v", got)
	}
}

func TestScanPyprojectIgnoresUnrelatedSections(t *testing.T) {
	body := `[project]
name = "x"
requires-python = ">=3.10"

[tool.foo]
python = "not-a-version"

[project.dependencies]
python = "not-a-version"
`
	fsys := fstest.MapFS{"pyproject.toml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "pyproject.toml", "requires-python") != ">=3.10" {
		t.Errorf("requires-python missing: %#v", got)
	}
	for _, entry := range got {
		if entry.Source == "pyproject.toml" && strings.HasSuffix(entry.Field, ".python") {
			t.Errorf("unexpected poetry attribution outside dependencies section: %#v", entry)
		}
	}
}

func TestScanPyprojectPoetryDevDeps(t *testing.T) {
	body := `[tool.poetry.dev-dependencies]
python = "^3.11"
`
	fsys := fstest.MapFS{"pyproject.toml": &fstest.MapFile{Data: []byte(body)}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.py"))
	if findVersion(got, "pyproject.toml", "tool.poetry.dev-dependencies.python") != "^3.11" {
		t.Fatalf("dev-dependencies python missing: %#v", got)
	}
}

func TestScanPackageJSONParseErrorEmitsTSEntry(t *testing.T) {
	fsys := fstest.MapFS{"package.json": &fstest.MapFile{Data: []byte("{not json")}}
	got := ScanFS(fsys, ctxWithChangedFiles("a.js", "a.ts"))
	var jsErr, tsErr bool
	for _, entry := range got {
		if entry.Source == "package.json" && entry.Error != "" {
			if entry.Language == langJavaScript {
				jsErr = true
			}
			if entry.Language == langTypeScript {
				tsErr = true
			}
		}
	}
	if !jsErr || !tsErr {
		t.Fatalf("expected both JS and TS error entries, got %#v", got)
	}
}

func TestScanGoModParseError(t *testing.T) {
	fsys := fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte("this is not a go.mod\n")}}
	got := ScanFS(fsys, ctxWithChangedFiles("main.go"))
	if len(got) == 0 {
		t.Fatal("expected at least one entry on parse failure")
	}
	hasError := false
	for _, entry := range got {
		if entry.Source == "go.mod" && entry.Error != "" {
			hasError = true
		}
	}
	if !hasError {
		t.Fatalf("expected go.mod error entry, got %#v", got)
	}
}

func versionsBySourceField(entries []model.ToolchainVersion) map[string]string {
	out := map[string]string{}
	for _, entry := range entries {
		out[entry.Source+"|"+entry.Field] = entry.Version
	}
	return out
}

func findVersion(entries []model.ToolchainVersion, source, field string) string {
	for _, entry := range entries {
		if entry.Source == source && entry.Field == field {
			return entry.Version
		}
	}
	return ""
}
