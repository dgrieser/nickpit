package filetype

import "testing"

func TestDetectLanguage(t *testing.T) {
	tests := map[string]string{
		"main.go":                           "go",
		"pkg/worker.py":                     "python",
		"web/app.tsx":                       "nodejs",
		"src/main.zig":                      "zig",
		"src/main.c":                        "c",
		"include/main.h":                    "c",
		"src/main.cpp":                      "cpp",
		"include/main.hpp":                  "cpp",
		"Views/Home.cshtml":                 "html",
		"web/index.html":                    "html",
		"web/index.htm":                     "html",
		"web/styles.css":                    "css",
		"web/styles.scss":                   "scss",
		"web/styles.sass":                   "scss",
		"k8s/deployment.yaml":               "yaml",
		"config/settings.yml":               "yaml",
		"package.json":                      "json",
		"tsconfig.jsonc":                    "json",
		"README.md":                         "markdown",
		"docs/guide.markdown":               "markdown",
		"charts/api/Chart.yaml":             "helm",
		"charts/api/Chart.lock":             "helm",
		"charts/api/values.yaml":            "helm",
		"charts/api/values-prod.yaml":       "helm",
		"charts/api/templates/deploy.yaml":  "helm",
		"charts/api/templates/NOTES.txt":    "helm",
		"charts/api/templates/_helpers.tpl": "helm",
		"Dockerfile":                        "dockerfile",
		"build.Containerfile":               "dockerfile",
		"Makefile":                          "makefile",
		"unknown.bin":                       "text",
	}
	for path, want := range tests {
		if got := DetectLanguage(path); got != want {
			t.Fatalf("DetectLanguage(%q) = %q, want %q", path, got, want)
		}
	}
}
