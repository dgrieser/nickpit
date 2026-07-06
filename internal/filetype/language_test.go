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
		"templates/values.yaml":             "helm",
		"app/templates/main.go":             "go",
		"src/charts/line.ts":                "nodejs",
		"config/values.yaml":                "yaml",
		"app/templates/index.html":          "html",
		"api/v1/api.proto":                  "protobuf",
		"infra/main.tf":                     "terraform",
		"infra/vars.hcl":                    "terraform",
		"CMakeLists.txt":                    "cmake",
		"cmake/deps.cmake":                  "cmake",
		"scripts/build.ps1":                 "powershell",
		"lib/app.ex":                        "elixir",
		"test/app_test.exs":                 "elixir",
		"lib/util.lua":                      "lua",
		"app/main.dart":                     "dart",
		"build.gradle":                      "groovy",
		"schema/api.graphql":                "graphql",
		"scripts/legacy.pl":                 "perl",
		"src/Main.hs":                       "haskell",
		"analysis/plot.R":                   "r",
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

func TestClassify(t *testing.T) {
	tests := []struct {
		path    string
		content string
		want    Classification
	}{
		{"pkg/service.pb.go", "", Classification{Language: "go", Generated: true}},
		{"bin/deploy", "#!/usr/bin/env python\n", Classification{Language: "python", Generated: false}},
		{"api/zz_generated.deepcopy.go", "", Classification{Language: "go", Generated: true}},
		{"Cargo.lock", "", Classification{Language: "text", Generated: true}},
		{"pkg/service.go", "+package pkg\n", Classification{Language: "go", Generated: false}},
	}
	for _, tc := range tests {
		if got := Classify(tc.path, tc.content); got != tc.want {
			t.Fatalf("Classify(%q) = %+v, want %+v", tc.path, got, tc.want)
		}
	}
}
