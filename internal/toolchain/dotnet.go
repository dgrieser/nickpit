package toolchain

import (
	"encoding/xml"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

// msbuildProject holds the properties of an SDK-style project or a
// Directory.Build.props file that decide the C# language version. Conditional
// PropertyGroups are read like unconditional ones: the scan is best-effort and
// lowest-version-wins resolves multi-targeting conservatively.
type msbuildProject struct {
	PropertyGroups []struct {
		LangVersion      string `xml:"LangVersion"`
		TargetFramework  string `xml:"TargetFramework"`
		TargetFrameworks string `xml:"TargetFrameworks"`
	} `xml:"PropertyGroup"`
}

type msbuildProps struct {
	langVersion string
	frameworks  []string
}

func parseMSBuild(data []byte) (msbuildProps, error) {
	var project msbuildProject
	if err := xml.Unmarshal(data, &project); err != nil {
		return msbuildProps{}, err
	}
	var props msbuildProps
	for _, group := range project.PropertyGroups {
		if value := strings.TrimSpace(group.LangVersion); value != "" && props.langVersion == "" {
			props.langVersion = value
		}
		for _, list := range []string{group.TargetFramework, group.TargetFrameworks} {
			for tfm := range strings.SplitSeq(list, ";") {
				if tfm = strings.TrimSpace(tfm); tfm != "" {
					props.frameworks = append(props.frameworks, tfm)
				}
			}
		}
	}
	return props, nil
}

// numericLangVersionRe accepts explicit C# versions ("7.3", "10", "12.0").
// Symbolic values (latest, latestMajor, preview, default) depend on the SDK
// that happens to build the project, so they fall back to the framework default.
var numericLangVersionRe = regexp.MustCompile(`^\d+(?:\.\d+)?$`)

// netTFMRe matches .NET 5+ target frameworks, including OS-specific ones such
// as net8.0-windows or net9.0-android35.0.
var netTFMRe = regexp.MustCompile(`^net(\d+)\.\d+(?:-.*)?$`)

// netFrameworkTFMRe matches .NET Framework monikers: net20 … net481.
var netFrameworkTFMRe = regexp.MustCompile(`^net[1-4]\d*$`)

// csharpForTFM returns the default C# language version the compiler picks for
// a target framework moniker, or "" when the moniker is unknown.
// https://learn.microsoft.com/dotnet/csharp/language-reference/configure-language-version
func csharpForTFM(tfm string) string {
	tfm = strings.ToLower(strings.TrimSpace(tfm))
	if match := netTFMRe.FindStringSubmatch(tfm); match != nil {
		major, err := strconv.Atoi(match[1])
		if err != nil || major < 5 {
			return ""
		}
		return csharpForDotnetMajor(major)
	}
	switch {
	case strings.HasPrefix(tfm, "netcoreapp3."), tfm == "netstandard2.1":
		return "8.0"
	case strings.HasPrefix(tfm, "netcoreapp"), strings.HasPrefix(tfm, "netstandard"):
		return "7.3"
	case netFrameworkTFMRe.MatchString(tfm):
		return "7.3"
	}
	return ""
}

// csharpForDotnetMajor maps a .NET (5+) major version to its default C#
// version: .NET 5 shipped C# 9 and every later major adds one.
func csharpForDotnetMajor(major int) string {
	if major < 5 {
		return ""
	}
	return strconv.Itoa(major+4) + ".0"
}

// scanDotnetProjects resolves the C# language version of the projects that own
// the changed C# files: for each file it walks up to the nearest *.csproj and
// layers the Directory.Build.props files above it (nearest wins, the project
// file wins over all of them), mirroring MSBuild's property precedence. An
// explicit numeric LangVersion is reported as is; otherwise each target
// framework is mapped to its default C# version.
func scanDotnetProjects(fsys fs.FS, paths []string) []model.ToolchainVersion {
	projects := map[string]bool{}
	for _, p := range paths {
		if project := owningProject(fsys, p); project != "" {
			projects[project] = true
		}
	}
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []model.ToolchainVersion
	for _, project := range names {
		out = append(out, projectCSharpVersions(fsys, project)...)
	}
	return out
}

// owningProject returns the first *.csproj found walking up from the file's
// directory, or "" when none exists below the repository root.
func owningProject(fsys fs.FS, file string) string {
	dir := path.Dir(path.Clean(strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "/")))
	if strings.HasSuffix(strings.ToLower(file), ".csproj") {
		return path.Clean(file)
	}
	for {
		matches, _ := fs.Glob(fsys, path.Join(dir, "*.csproj"))
		if len(matches) > 0 {
			sort.Strings(matches)
			return matches[0]
		}
		if dir == "." || dir == "/" || dir == "" {
			return ""
		}
		dir = path.Dir(dir)
	}
}

func projectCSharpVersions(fsys fs.FS, project string) []model.ToolchainVersion {
	data, err := readFile(fsys, project)
	if err != nil {
		return nil
	}
	props, err := parseMSBuild(data)
	if err != nil {
		return []model.ToolchainVersion{{Language: langCSharp, Source: project, Error: err.Error()}}
	}
	source := project
	// Fill unset properties from the Directory.Build.props chain, nearest first.
	for dir := path.Dir(project); ; dir = path.Dir(dir) {
		if props.langVersion != "" && len(props.frameworks) > 0 {
			break
		}
		propsPath := path.Join(dir, "Directory.Build.props")
		if inherited, ok := readMSBuildProps(fsys, propsPath); ok {
			if props.langVersion == "" && inherited.langVersion != "" {
				props.langVersion = inherited.langVersion
				source = propsPath
			}
			if len(props.frameworks) == 0 {
				props.frameworks = inherited.frameworks
			}
		}
		if dir == "." || dir == "/" {
			break
		}
	}
	if numericLangVersionRe.MatchString(props.langVersion) {
		return []model.ToolchainVersion{{Language: langCSharp, Source: source, Field: "LangVersion", Version: props.langVersion}}
	}
	var out []model.ToolchainVersion
	for _, tfm := range props.frameworks {
		if version := csharpForTFM(tfm); version != "" {
			out = append(out, model.ToolchainVersion{Language: langCSharp, Source: project, Field: "TargetFramework " + tfm, Version: version})
		}
	}
	return out
}

func readMSBuildProps(fsys fs.FS, name string) (msbuildProps, bool) {
	data, err := readFile(fsys, name)
	if err != nil {
		return msbuildProps{}, false
	}
	props, err := parseMSBuild(data)
	if err != nil {
		return msbuildProps{}, false
	}
	return props, true
}

// dotnetImageRe extracts the .NET major version from an official image tag
// ("8.0", "8.0-alpine", "9.0.1-bookworm-slim").
var dotnetImageRe = regexp.MustCompile(`^(\d+)\.\d+`)

// csharpForDotnetImageTag maps an mcr.microsoft.com/dotnet/* image tag to the
// default C# version of that .NET major, or "" when the tag carries none.
func csharpForDotnetImageTag(tag string) string {
	match := dotnetImageRe.FindStringSubmatch(tag)
	if match == nil {
		return ""
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return ""
	}
	return csharpForDotnetMajor(major)
}
