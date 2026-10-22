package toolchain

import (
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"golang.org/x/mod/modfile"
)

// kubeClientModules are the Kubernetes client libraries whose v0.Y.Z module
// version tracks Kubernetes 1.Y; a controller compiles against that API level.
var kubeClientModules = map[string]bool{
	"k8s.io/api":          true,
	"k8s.io/apimachinery": true,
	"k8s.io/client-go":    true,
}

var kubeModuleVersionRe = regexp.MustCompile(`^v0\.(\d+)\.\d+`)

// scanGoModKubernetes maps go.mod requirements on the Kubernetes client
// libraries to the Kubernetes minor they target (k8s.io/api v0.33.2 → 1.33).
func scanGoModKubernetes(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "go.mod")
	if err != nil {
		return nil
	}
	parsed, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		// scanGoMod already reports the parse error for Go.
		return nil
	}
	var out []model.ToolchainVersion
	for _, req := range parsed.Require {
		if !kubeClientModules[req.Mod.Path] {
			continue
		}
		match := kubeModuleVersionRe.FindStringSubmatch(req.Mod.Version)
		if match == nil {
			continue
		}
		out = append(out, model.ToolchainVersion{Language: langKubernetes, Source: "go.mod", Field: "require " + req.Mod.Path + " " + req.Mod.Version, Version: "1." + match[1]})
	}
	return out
}

var envtestVersionRe = regexp.MustCompile(`(?m)^\s*ENVTEST_K8S_VERSION\s*[?:]?=\s*v?(\d+\.\d+(?:\.\d+)?)\s*(?:#|$)`)

// scanMakefileEnvtest reads the Kubebuilder scaffold's ENVTEST_K8S_VERSION,
// the API server version the controller's integration tests run against.
func scanMakefileEnvtest(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "Makefile")
	if err != nil {
		return nil
	}
	match := envtestVersionRe.FindStringSubmatch(string(data))
	if match == nil {
		return nil
	}
	return []model.ToolchainVersion{{Language: langKubernetes, Source: "Makefile", Field: "ENVTEST_K8S_VERSION", Version: match[1]}}
}

// kindNodeImageRe matches kind node images, whose tag is the Kubernetes version.
var kindNodeImageRe = regexp.MustCompile(`kindest/node:v(\d+\.\d+(?:\.\d+)?)`)

// setupHelmRe finds the azure/setup-helm action in a workflow step.
var setupHelmRe = regexp.MustCompile(`^(\s*)(-\s+)?uses:\s*['"]?azure/setup-helm@`)

var stepVersionRe = regexp.MustCompile(`^\s*version:\s*['"]?v?(\d[^'"\s#]*)`)

// setupHelmVersions returns the `version:` inputs of azure/setup-helm steps.
// The input belongs to the step while lines stay indented at least as deep as
// the step's `uses:` key; the next list item ends the step.
func setupHelmVersions(content string) []string {
	lines := strings.Split(content, "\n")
	var out []string
	for i, line := range lines {
		match := setupHelmRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		keyIndent := len(match[1]) + len(match[2])
		for _, next := range lines[i+1:] {
			trimmed := strings.TrimLeft(next, " \t")
			if trimmed == "" {
				continue
			}
			if len(next)-len(trimmed) < keyIndent {
				break
			}
			if version := stepVersionRe.FindStringSubmatch(next); version != nil {
				out = append(out, version[1])
				break
			}
		}
	}
	return out
}

// imageRefRe captures image references with a numeric tag anywhere in a
// container or CI file ("postgres:16", "bitnami/postgresql:16.2.0",
// image: "mysql:8.4"). Host:port strings such as "mysql:3306" also match and
// are filtered by databaseImageVersion.
var imageRefRe = regexp.MustCompile(`(?m)(?:^|[\s"'=\[,])([a-z0-9][a-z0-9._/-]*):(\d[A-Za-z0-9._-]*)`)

// databaseImages maps a database image's base name to its styleguide language.
var databaseImages = map[string]string{
	"postgres":     langPostgreSQL,
	"postgresql":   langPostgreSQL,
	"postgis":      langPostgreSQL,
	"mysql":        langMySQL,
	"mysql-server": langMySQL,
}

var leadingNumberRe = regexp.MustCompile(`^\d+`)

// databaseImageVersion reports the styleguide language of a database image
// reference, rejecting tags whose major is implausible for a release (ports).
func databaseImageVersion(name, tag string) (string, bool) {
	language, ok := databaseImages[path.Base(name)]
	if !ok {
		return "", false
	}
	major, err := strconv.Atoi(leadingNumberRe.FindString(tag))
	if err != nil || major == 0 || major >= 100 {
		return "", false
	}
	return language, true
}

// composeFiles are the Compose file names Docker Compose looks for.
var composeFiles = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

// scanDatabaseImages reports PostgreSQL and MySQL server versions pinned by
// container images in Compose files, the Dockerfile and CI configuration.
func scanDatabaseImages(fsys fs.FS) []model.ToolchainVersion {
	sources := append([]string{}, composeFiles...)
	sources = append(sources, "Dockerfile", ".gitlab-ci.yml")
	sources = append(sources, workflowPaths(fsys)...)
	var out []model.ToolchainVersion
	for _, source := range sources {
		data, err := readFile(fsys, source)
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, match := range imageRefRe.FindAllStringSubmatch(string(data), -1) {
			name, tag := strings.ToLower(match[1]), match[2]
			language, ok := databaseImageVersion(name, tag)
			if !ok || seen[name+":"+tag] {
				continue
			}
			seen[name+":"+tag] = true
			out = append(out, model.ToolchainVersion{Language: language, Source: source, Field: "image " + name, Version: tag})
		}
	}
	return out
}

func workflowPaths(fsys fs.FS) []string {
	ymlPaths, _ := fs.Glob(fsys, ".github/workflows/*.yml")
	yamlPaths, _ := fs.Glob(fsys, ".github/workflows/*.yaml")
	paths := append(ymlPaths, yamlPaths...)
	sort.Strings(paths)
	return paths
}
