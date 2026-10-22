// Package toolchain captures best-effort toolchain/runtime version output for
// the languages touched by a review by parsing static manifest files. It never
// executes external binaries. Failures surface as entries with Error or
// Unavailable set; missing files are silently skipped.
package toolchain

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/mappings"
	"golang.org/x/mod/modfile"
)

const (
	langGo         = "go"
	langPython     = "python"
	langJavaScript = "javascript"
	langTypeScript = "typescript"
	langCSharp     = "csharp"
	langHelm       = "helm"
	langKubernetes = "kubernetes"
	langPostgreSQL = "postgresql"
	langMySQL      = "mysql"
)

// languageOrder is the stable output order of toolchain entries.
var languageOrder = []string{langGo, langPython, langJavaScript, langTypeScript, langCSharp, langHelm, langKubernetes, langPostgreSQL, langMySQL}

// Capture reads manifest files from repoRoot and returns toolchain entries for
// every language touched by reviewCtx. Best-effort: file open errors are
// silently ignored, parse errors are emitted as entries with Error set, and
// languages with no matching declarations get a single Unavailable entry.
func Capture(_ context.Context, repoRoot string, reviewCtx *model.ReviewContext) []model.ToolchainVersion {
	if repoRoot == "" || reviewCtx == nil {
		return nil
	}
	return ScanFS(os.DirFS(repoRoot), reviewCtx)
}

// ScanFS is the FS-backed variant of Capture, used in tests.
func ScanFS(fsys fs.FS, reviewCtx *model.ReviewContext) []model.ToolchainVersion {
	if fsys == nil {
		return nil
	}
	relevant, companions := relevantLanguages(fsys, reviewCtx)
	if len(relevant) == 0 && len(companions) == 0 {
		return nil
	}
	relevantSet := map[string]bool{}
	for _, language := range relevant {
		relevantSet[language] = true
	}
	for _, language := range companions {
		relevantSet[language] = true
	}
	var hits []model.ToolchainVersion
	for _, scan := range scanners {
		hits = append(hits, scan(fsys)...)
	}
	if relevantSet[langCSharp] {
		hits = append(hits, scanDotnetProjects(fsys, changedPaths(reviewCtx))...)
	}
	out := make([]model.ToolchainVersion, 0, len(hits))
	seen := map[string]bool{}
	for _, entry := range hits {
		if !relevantSet[entry.Language] {
			continue
		}
		out = append(out, entry)
		seen[entry.Language] = true
	}
	for _, language := range relevant {
		if !seen[language] {
			out = append(out, model.ToolchainVersion{Language: language, Unavailable: true})
		}
	}
	sortEntries(out)
	return out
}

var scanners = []func(fs.FS) []model.ToolchainVersion{
	scanGoMod,
	scanToolVersions,
	scanPythonVersion,
	scanNvmrc,
	scanNodeVersion,
	scanRuntimeTxt,
	scanPipfile,
	scanPyproject,
	scanSetupCfg,
	scanSetupPy,
	scanPackageJSON,
	scanPackageLock,
	scanYarnLock,
	scanPnpmLock,
	scanDockerfile,
	scanGitlabCI,
	scanGithubWorkflows,
	scanGoModKubernetes,
	scanMakefileEnvtest,
	scanDatabaseImages,
}

func readFile(fsys fs.FS, path string) ([]byte, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func scanGoMod(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "go.mod")
	if err != nil {
		return nil
	}
	parsed, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return []model.ToolchainVersion{{Language: langGo, Source: "go.mod", Error: err.Error()}}
	}
	var out []model.ToolchainVersion
	if parsed.Go != nil && parsed.Go.Version != "" {
		out = append(out, model.ToolchainVersion{Language: langGo, Source: "go.mod", Field: "go", Version: parsed.Go.Version})
	}
	if parsed.Toolchain != nil && parsed.Toolchain.Name != "" {
		out = append(out, model.ToolchainVersion{Language: langGo, Source: "go.mod", Field: "toolchain", Version: parsed.Toolchain.Name})
	}
	return out
}

func scanToolVersions(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, ".tool-versions")
	if err != nil {
		return nil
	}
	var out []model.ToolchainVersion
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		plugin := strings.ToLower(parts[0])
		var language string
		switch plugin {
		case "golang", "go":
			language = langGo
		case "python":
			language = langPython
		case "nodejs", "node":
			language = langJavaScript
		case "helm":
			language = langHelm
		case "kubectl":
			language = langKubernetes
		case "postgres", "postgresql":
			language = langPostgreSQL
		case "mysql":
			language = langMySQL
		case "dotnet", "dotnet-core":
			// The SDK version (8.0.100) maps to its default C# version.
			if version := csharpForDotnetImageTag(parts[1]); version != "" {
				out = append(out, model.ToolchainVersion{Language: langCSharp, Source: ".tool-versions", Field: plugin + " " + parts[1], Version: version})
			}
			continue
		default:
			continue
		}
		out = append(out, model.ToolchainVersion{Language: language, Source: ".tool-versions", Field: plugin, Version: parts[1]})
	}
	return out
}

func scanPlainVersion(fsys fs.FS, path, language string) []model.ToolchainVersion {
	data, err := readFile(fsys, path)
	if err != nil {
		return nil
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return nil
	}
	return []model.ToolchainVersion{{Language: language, Source: path, Version: value}}
}

func scanPythonVersion(fsys fs.FS) []model.ToolchainVersion {
	return scanPlainVersion(fsys, ".python-version", langPython)
}

func scanNvmrc(fsys fs.FS) []model.ToolchainVersion {
	return scanPlainVersion(fsys, ".nvmrc", langJavaScript)
}

func scanNodeVersion(fsys fs.FS) []model.ToolchainVersion {
	return scanPlainVersion(fsys, ".node-version", langJavaScript)
}

var runtimePythonRe = regexp.MustCompile(`(?i)^python-(.+)$`)

func scanRuntimeTxt(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "runtime.txt")
	if err != nil {
		return nil
	}
	match := runtimePythonRe.FindStringSubmatch(strings.TrimSpace(string(data)))
	if match == nil {
		return nil
	}
	return []model.ToolchainVersion{{Language: langPython, Source: "runtime.txt", Version: match[1]}}
}

var pipfilePyRe = regexp.MustCompile(`(?m)^\s*python_(version|full_version)\s*=\s*['"]([^'"]+)['"]`)

func scanPipfile(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "Pipfile")
	if err != nil {
		return nil
	}
	match := pipfilePyRe.FindStringSubmatch(string(data))
	if match == nil {
		return nil
	}
	return []model.ToolchainVersion{{Language: langPython, Source: "Pipfile", Field: "python_" + match[1], Version: match[2]}}
}

var (
	pyprojectSectionRe   = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*$`)
	pyprojectRequiresRe  = regexp.MustCompile(`^\s*requires-python\s*=\s*['"]([^'"]+)['"]`)
	pyprojectPyAssignRe  = regexp.MustCompile(`^\s*python\s*=\s*['"]([^'"]+)['"]`)
	poetryDepsSectionSet = map[string]bool{
		"tool.poetry.dependencies":     true,
		"tool.poetry.dev-dependencies": true,
	}
)

func scanPyproject(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "pyproject.toml")
	if err != nil {
		return nil
	}
	var out []model.ToolchainVersion
	section := ""
	seenPoetry := false
	for line := range strings.SplitSeq(string(data), "\n") {
		if match := pyprojectSectionRe.FindStringSubmatch(line); match != nil {
			section = strings.TrimSpace(match[1])
			continue
		}
		if match := pyprojectRequiresRe.FindStringSubmatch(line); match != nil {
			out = append(out, model.ToolchainVersion{Language: langPython, Source: "pyproject.toml", Field: "requires-python", Version: match[1]})
			continue
		}
		if seenPoetry || !poetryDepsSectionSet[section] {
			continue
		}
		if match := pyprojectPyAssignRe.FindStringSubmatch(line); match != nil {
			out = append(out, model.ToolchainVersion{Language: langPython, Source: "pyproject.toml", Field: section + ".python", Version: match[1]})
			seenPoetry = true
		}
	}
	return out
}

var setupCfgPyRe = regexp.MustCompile(`(?m)^\s*python_requires\s*=\s*([^\r\n]+?)\s*$`)

func scanSetupCfg(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "setup.cfg")
	if err != nil {
		return nil
	}
	match := setupCfgPyRe.FindStringSubmatch(string(data))
	if match == nil {
		return nil
	}
	return []model.ToolchainVersion{{Language: langPython, Source: "setup.cfg", Field: "python_requires", Version: strings.TrimSpace(match[1])}}
}

var setupPyRe = regexp.MustCompile(`python_requires\s*=\s*['"]([^'"]+)['"]`)

func scanSetupPy(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "setup.py")
	if err != nil {
		return nil
	}
	match := setupPyRe.FindStringSubmatch(string(data))
	if match == nil {
		return nil
	}
	return []model.ToolchainVersion{{Language: langPython, Source: "setup.py", Field: "python_requires", Version: match[1]}}
}

type packageJSON struct {
	Engines         map[string]string `json:"engines"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func scanPackageJSON(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "package.json")
	if err != nil {
		return nil
	}
	var parsed packageJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		msg := err.Error()
		return []model.ToolchainVersion{
			{Language: langJavaScript, Source: "package.json", Error: msg},
			{Language: langTypeScript, Source: "package.json", Error: msg},
		}
	}
	var out []model.ToolchainVersion
	if value := strings.TrimSpace(parsed.Engines["node"]); value != "" {
		out = append(out, model.ToolchainVersion{Language: langJavaScript, Source: "package.json", Field: "engines.node", Version: value})
	}
	if value := strings.TrimSpace(parsed.DevDependencies["typescript"]); value != "" {
		out = append(out, model.ToolchainVersion{Language: langTypeScript, Source: "package.json", Field: "devDependencies.typescript", Version: value})
	} else if value := strings.TrimSpace(parsed.Dependencies["typescript"]); value != "" {
		out = append(out, model.ToolchainVersion{Language: langTypeScript, Source: "package.json", Field: "dependencies.typescript", Version: value})
	}
	return out
}

func scanPackageLock(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "package-lock.json")
	if err != nil {
		return nil
	}
	var parsed struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	if entry, ok := parsed.Packages["node_modules/typescript"]; ok && entry.Version != "" {
		return []model.ToolchainVersion{{Language: langTypeScript, Source: "package-lock.json", Field: "packages.node_modules/typescript", Version: entry.Version}}
	}
	if entry, ok := parsed.Dependencies["typescript"]; ok && entry.Version != "" {
		return []model.ToolchainVersion{{Language: langTypeScript, Source: "package-lock.json", Field: "dependencies.typescript", Version: entry.Version}}
	}
	return nil
}

var yarnLockRe = regexp.MustCompile(`(?m)^"?typescript@[^\n]*?:\s*\n(?:\s+[^\n]+\n)*?\s+version\s+"([^"]+)"`)

func scanYarnLock(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "yarn.lock")
	if err != nil {
		return nil
	}
	match := yarnLockRe.FindStringSubmatch(string(data))
	if match == nil {
		return nil
	}
	return []model.ToolchainVersion{{Language: langTypeScript, Source: "yarn.lock", Field: "typescript", Version: match[1]}}
}

var pnpmTsRe = regexp.MustCompile(`(?m)^\s*/?typescript@([^:\s(]+)[:(]`)

func scanPnpmLock(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "pnpm-lock.yaml")
	if err != nil {
		return nil
	}
	match := pnpmTsRe.FindStringSubmatch(string(data))
	if match == nil {
		return nil
	}
	return []model.ToolchainVersion{{Language: langTypeScript, Source: "pnpm-lock.yaml", Field: "typescript", Version: match[1]}}
}

// dockerfileFromRe captures the whole image token after FROM. Go's regexp has
// no lookahead, so the image name cannot be validated in the pattern without
// also matching prefixes (e.g. "golangci/golangci-lint" starts with "golang");
// splitImageToken plus imageLanguage do the exact-name check in code instead.
var dockerfileFromRe = regexp.MustCompile(`(?im)^\s*FROM\s+(?:--platform=\S+\s+)?(\S+)`)

func scanDockerfile(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, "Dockerfile")
	if err != nil {
		return nil
	}
	var out []model.ToolchainVersion
	for _, match := range dockerfileFromRe.FindAllStringSubmatch(string(data), -1) {
		image, tag := splitImageToken(match[1])
		language, version := imageLanguage(image, tag)
		if language == "" {
			continue
		}
		out = append(out, model.ToolchainVersion{Language: language, Source: "Dockerfile", Field: "FROM " + image, Version: version})
	}
	return out
}

// gitlabImageRe captures the whole (optionally quoted) image token; the exact
// image-name check happens in code, see dockerfileFromRe.
var gitlabImageRe = regexp.MustCompile(`(?im)^\s*image:\s*["']?([^\s"'\r\n]+)`)

func scanGitlabCI(fsys fs.FS) []model.ToolchainVersion {
	data, err := readFile(fsys, ".gitlab-ci.yml")
	if err != nil {
		return nil
	}
	var out []model.ToolchainVersion
	for _, match := range gitlabImageRe.FindAllStringSubmatch(string(data), -1) {
		image, tag := splitImageToken(match[1])
		language, version := imageLanguage(image, tag)
		if language == "" {
			continue
		}
		out = append(out, model.ToolchainVersion{Language: language, Source: ".gitlab-ci.yml", Field: "image " + image, Version: version})
	}
	for _, match := range kindNodeImageRe.FindAllStringSubmatch(string(data), -1) {
		out = append(out, model.ToolchainVersion{Language: langKubernetes, Source: ".gitlab-ci.yml", Field: "image kindest/node", Version: match[1]})
	}
	return out
}

// splitImageToken splits an image reference token into its lower-cased name and
// tag (defaulting to "latest" when no tag is present). The name is everything
// up to the first ":", so tokens where a known image name is only a prefix
// ("golangci/golangci-lint", "nodesource/distributions", "golang@sha256:...")
// keep their full name and are rejected by imageLanguage.
func splitImageToken(token string) (name, tag string) {
	name, tag, _ = strings.Cut(token, ":")
	if tag == "" {
		tag = "latest"
	}
	return strings.ToLower(name), tag
}

var (
	workflowGoVerRe     = regexp.MustCompile(`(?m)^\s*go-version:\s*['"]?([^'"#\r\n]+?)['"]?\s*(?:#|$)`)
	workflowPythonVerRe = regexp.MustCompile(`(?m)^\s*python-version:\s*['"]?([^'"#\r\n]+?)['"]?\s*(?:#|$)`)
	workflowNodeVerRe   = regexp.MustCompile(`(?m)^\s*node-version:\s*['"]?([^'"#\r\n]+?)['"]?\s*(?:#|$)`)
	workflowDotnetVerRe = regexp.MustCompile(`(?m)^\s*dotnet-version:\s*['"]?([^'"#\r\n]+?)['"]?\s*(?:#|$)`)
)

func scanGithubWorkflows(fsys fs.FS) []model.ToolchainVersion {
	var out []model.ToolchainVersion
	for _, path := range workflowPaths(fsys) {
		data, err := readFile(fsys, path)
		if err != nil {
			continue
		}
		content := string(data)
		out = append(out, workflowMatches(content, path, workflowGoVerRe, langGo, "go-version")...)
		out = append(out, workflowMatches(content, path, workflowPythonVerRe, langPython, "python-version")...)
		out = append(out, workflowMatches(content, path, workflowNodeVerRe, langJavaScript, "node-version")...)
		for _, entry := range workflowMatches(content, path, workflowDotnetVerRe, langCSharp, "dotnet-version") {
			// actions/setup-dotnet installs an SDK (8.0.x); report its default C# version.
			if version := csharpForDotnetImageTag(entry.Version); version != "" {
				entry.Field += " " + entry.Version
				entry.Version = version
				out = append(out, entry)
			}
		}
		for _, version := range setupHelmVersions(content) {
			out = append(out, model.ToolchainVersion{Language: langHelm, Source: path, Field: "azure/setup-helm version", Version: version})
		}
		for _, match := range kindNodeImageRe.FindAllStringSubmatch(content, -1) {
			out = append(out, model.ToolchainVersion{Language: langKubernetes, Source: path, Field: "kindest/node", Version: match[1]})
		}
	}
	return out
}

func workflowMatches(content, source string, re *regexp.Regexp, language, field string) []model.ToolchainVersion {
	var out []model.ToolchainVersion
	for _, match := range re.FindAllStringSubmatch(content, -1) {
		value := strings.TrimSpace(match[1])
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "[") || strings.Contains(value, "${{") {
			continue
		}
		out = append(out, model.ToolchainVersion{Language: language, Source: source, Field: field, Version: value})
	}
	return out
}

// imageLanguage maps an exact image name to its language and the version the
// tag stands for. .NET images carry the runtime version, which is reported as
// that runtime's default C# version.
func imageLanguage(image, tag string) (language, version string) {
	switch image {
	case "golang":
		return langGo, tag
	case "python":
		return langPython, tag
	case "node", "nodejs":
		return langJavaScript, tag
	case "alpine/helm", "dtzar/helm-kubectl":
		return langHelm, tag
	case "mcr.microsoft.com/dotnet/sdk", "mcr.microsoft.com/dotnet/aspnet", "mcr.microsoft.com/dotnet/runtime", "mcr.microsoft.com/dotnet/runtime-deps":
		if version := csharpForDotnetImageTag(tag); version != "" {
			return langCSharp, version
		}
	}
	return "", ""
}

func sortEntries(out []model.ToolchainVersion) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Language != out[j].Language {
			return langOrder(out[i].Language) < langOrder(out[j].Language)
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Field < out[j].Field
	})
}

func langOrder(language string) int {
	for i, candidate := range languageOrder {
		if candidate == language {
			return i
		}
	}
	return len(languageOrder)
}

// relevantLanguages returns the languages whose toolchain versions matter for
// the review: languages of the changed files plus languages the styleguide
// detectors find in their paths or content (Kubernetes manifests, SQL, ...).
// companions are languages that only refine another one's guide (a SQL change
// in a project pinning postgres:16 gets the PostgreSQL 16 guide); they are
// scanned but never reported as unavailable.
func relevantLanguages(fsys fs.FS, reviewCtx *model.ReviewContext) (relevant, companions []string) {
	if reviewCtx == nil {
		return nil, nil
	}
	seen := map[string]struct{}{}
	detected := map[string]struct{}{}
	consider := func(path, hint, content string) {
		for _, language := range languagesForPath(path, hint) {
			seen[language] = struct{}{}
		}
		for _, language := range mappings.StyleGuideDetectorLanguages(path, content) {
			detected[language] = struct{}{}
		}
	}
	for _, file := range reviewCtx.DiffFiles {
		consider(file.FilePath, file.Language, file.Content)
	}
	for _, hunk := range reviewCtx.DiffHunks {
		consider(hunk.FilePath, hunk.Language, hunk.Content)
	}
	for _, file := range reviewCtx.ChangedFiles {
		content := ""
		if fsys != nil && file.Status != model.FileDeleted && mappings.StyleGuideDetectorProbePath(file.Path) {
			if data, err := readFile(fsys, file.Path); err == nil {
				content = string(data)
			}
		}
		consider(file.Path, "", content)
	}
	companionSet := map[string]struct{}{}
	for language := range detected {
		if slices.Contains(languageOrder, language) {
			seen[language] = struct{}{}
		}
		for _, companion := range mappings.ToolchainCompanions(language) {
			companionSet[companion] = struct{}{}
		}
	}
	for _, language := range languageOrder {
		if _, ok := seen[language]; ok {
			relevant = append(relevant, language)
		} else if _, ok := companionSet[language]; ok {
			companions = append(companions, language)
		}
	}
	return relevant, companions
}

// changedPaths lists every changed path of the review once.
func changedPaths(reviewCtx *model.ReviewContext) []string {
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	for _, file := range reviewCtx.DiffFiles {
		add(file.FilePath)
	}
	for _, hunk := range reviewCtx.DiffHunks {
		add(hunk.FilePath)
	}
	for _, file := range reviewCtx.ChangedFiles {
		if file.Status != model.FileDeleted {
			add(file.Path)
		}
	}
	return out
}

func languagesForPath(path, hint string) []string {
	if path == "" && hint == "" {
		return nil
	}
	ext := ""
	if path != "" {
		ext = strings.ToLower(extOf(path))
	}
	switch ext {
	case ".go":
		return []string{langGo}
	case ".py":
		return []string{langPython}
	}
	styleLang := ""
	if path != "" {
		styleLang = mappings.StyleGuideLanguageForPath(path, filetype.DetectLanguage)
	}
	switch styleLang {
	case langJavaScript, langTypeScript, langCSharp, langHelm:
		return []string{styleLang}
	}
	if ext == ".csproj" || strings.EqualFold(filepath.Base(path), "Directory.Build.props") {
		return []string{langCSharp}
	}
	switch strings.ToLower(hint) {
	case langGo:
		return []string{langGo}
	case langPython:
		return []string{langPython}
	case langJavaScript, "js":
		return []string{langJavaScript}
	case langTypeScript, "ts":
		return []string{langTypeScript}
	case langCSharp, langHelm:
		return []string{strings.ToLower(hint)}
	case "nodejs":
		switch ext {
		case ".ts", ".tsx", ".mts", ".cts":
			return []string{langTypeScript}
		}
		return []string{langJavaScript}
	}
	return nil
}

// extOf returns the extension of the path's base name. Using the base name
// matters: a dot in a directory (dir.v1/Makefile) must not yield
// ".v1/makefile" as the extension. Lowercasing stays at the call site.
func extOf(path string) string {
	return filepath.Ext(filepath.Base(path))
}
