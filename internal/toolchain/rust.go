package toolchain

import (
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dgrieser/nickpit/internal/model"
)

// cargoManifest holds the Cargo.toml fields that decide which Rust edition a
// crate is written in and which compiler it supports. A field set to
// `{ workspace = true }` (or `edition.workspace = true`) inherits the value
// from the workspace root's [workspace.package] table.
type cargoManifest struct {
	Package *struct {
		Edition     inheritable `toml:"edition"`
		RustVersion inheritable `toml:"rust-version"`
	} `toml:"package"`
	Workspace *struct {
		Package *struct {
			Edition     string `toml:"edition"`
			RustVersion string `toml:"rust-version"`
		} `toml:"package"`
	} `toml:"workspace"`
}

// inheritable is a Cargo package field that is either a literal string or a
// `{ workspace = true }` table.
type inheritable struct {
	Value     string
	Workspace bool
}

func (v *inheritable) UnmarshalTOML(data any) error {
	switch value := data.(type) {
	case string:
		v.Value = value
	case map[string]any:
		v.Workspace, _ = value["workspace"].(bool)
	}
	return nil
}

// cargoDefaultEdition is the edition Cargo assumes for a package without an
// `edition` key.
const cargoDefaultEdition = "2015"

// scanRustProjects resolves the edition and minimum supported Rust version of
// the crates that own the changed files: for each file it walks up to the
// nearest Cargo.toml with a [package] table, following `workspace = true`
// fields to the workspace root above it. Editions are reported as rust
// entries (they select the styleguide); rust-version is reported as rustc,
// next to the toolchain pins found by scanRustToolchainFiles.
func scanRustProjects(fsys fs.FS, paths []string) []model.ToolchainVersion {
	manifests := map[string]bool{}
	for _, p := range paths {
		if manifest := owningCargoManifest(fsys, p); manifest != "" {
			manifests[manifest] = true
		}
	}
	names := make([]string, 0, len(manifests))
	for name := range manifests {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []model.ToolchainVersion
	for _, manifest := range names {
		out = append(out, crateRustVersions(fsys, manifest)...)
	}
	return out
}

// owningCargoManifest returns the first Cargo.toml with a [package] table
// found walking up from the file's directory, or "" when there is none.
func owningCargoManifest(fsys fs.FS, file string) string {
	file = path.Clean(strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "/"))
	for dir := path.Dir(file); ; dir = path.Dir(dir) {
		candidate := path.Join(dir, "Cargo.toml")
		if manifest, ok := readCargoManifest(fsys, candidate); ok && manifest.Package != nil {
			return candidate
		}
		if dir == "." || dir == "/" || dir == "" {
			return ""
		}
	}
}

func readCargoManifest(fsys fs.FS, name string) (cargoManifest, bool) {
	data, err := readFile(fsys, name)
	if err != nil {
		return cargoManifest{}, false
	}
	var manifest cargoManifest
	if _, err := toml.Decode(string(data), &manifest); err != nil {
		return cargoManifest{}, false
	}
	return manifest, true
}

func crateRustVersions(fsys fs.FS, manifestPath string) []model.ToolchainVersion {
	data, err := readFile(fsys, manifestPath)
	if err != nil {
		return nil
	}
	var manifest cargoManifest
	if _, err := toml.Decode(string(data), &manifest); err != nil {
		return []model.ToolchainVersion{{Language: langRust, Source: manifestPath, Error: err.Error()}}
	}
	pkg := manifest.Package
	edition := model.ToolchainVersion{Language: langRust, Source: manifestPath, Field: "edition", Version: pkg.Edition.Value}
	rustVersion := model.ToolchainVersion{Language: langRustc, Source: manifestPath, Field: "rust-version", Version: pkg.RustVersion.Value}
	if pkg.Edition.Workspace || pkg.RustVersion.Workspace {
		root, workspace := cargoWorkspaceRoot(fsys, manifestPath)
		if pkg.Edition.Workspace {
			edition.Source, edition.Field, edition.Version = root, "workspace.package.edition", workspace.edition
		}
		if pkg.RustVersion.Workspace {
			rustVersion.Source, rustVersion.Field, rustVersion.Version = root, "workspace.package.rust-version", workspace.rustVersion
		}
	}
	if edition.Version == "" && !pkg.Edition.Workspace {
		edition.Field, edition.Version = "edition (Cargo default)", cargoDefaultEdition
	}
	var out []model.ToolchainVersion
	if edition.Version != "" {
		out = append(out, edition)
	}
	if rustVersion.Version != "" {
		out = append(out, rustVersion)
	}
	return out
}

type workspacePackage struct {
	edition     string
	rustVersion string
}

// cargoWorkspaceRoot finds the nearest Cargo.toml with a [workspace] table at
// or above the member's directory and returns its [workspace.package] values.
func cargoWorkspaceRoot(fsys fs.FS, memberManifest string) (string, workspacePackage) {
	for dir := path.Dir(memberManifest); ; dir = path.Dir(dir) {
		candidate := path.Join(dir, "Cargo.toml")
		if manifest, ok := readCargoManifest(fsys, candidate); ok && manifest.Workspace != nil {
			var values workspacePackage
			if manifest.Workspace.Package != nil {
				values = workspacePackage{edition: manifest.Workspace.Package.Edition, rustVersion: manifest.Workspace.Package.RustVersion}
			}
			return candidate, values
		}
		if dir == "." || dir == "/" || dir == "" {
			return memberManifest, workspacePackage{}
		}
	}
}

// rustToolchainChannelRe accepts pinned release channels ("1.75", "1.75.0");
// named channels (stable, beta, nightly-2024-01-01) do not pin a release.
var rustToolchainChannelRe = regexp.MustCompile(`^\d+\.\d+(?:\.\d+)?$`)

// scanRustToolchainFiles reads the rustup override files at the repository
// root: rust-toolchain.toml ([toolchain] channel) and the legacy
// rust-toolchain file, which holds either that TOML or a bare channel name.
func scanRustToolchainFiles(fsys fs.FS) []model.ToolchainVersion {
	var out []model.ToolchainVersion
	for _, name := range []string{"rust-toolchain.toml", "rust-toolchain"} {
		data, err := readFile(fsys, name)
		if err != nil {
			continue
		}
		channel := strings.TrimSpace(string(data))
		var file struct {
			Toolchain struct {
				Channel string `toml:"channel"`
			} `toml:"toolchain"`
		}
		if _, err := toml.Decode(string(data), &file); err == nil && file.Toolchain.Channel != "" {
			channel = file.Toolchain.Channel
		} else if name == "rust-toolchain.toml" {
			if err != nil {
				out = append(out, model.ToolchainVersion{Language: langRustc, Source: name, Error: err.Error()})
			}
			continue
		}
		if rustToolchainChannelRe.MatchString(channel) {
			out = append(out, model.ToolchainVersion{Language: langRustc, Source: name, Field: "toolchain.channel", Version: channel})
		}
	}
	return out
}
