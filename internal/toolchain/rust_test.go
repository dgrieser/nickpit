package toolchain

import (
	"testing"
	"testing/fstest"
)

func TestScanRustProjectsUsesOwningCrate(t *testing.T) {
	fsys := fstest.MapFS{
		"crates/api/Cargo.toml": &fstest.MapFile{Data: []byte(`[package]
name = "api"
version = "0.1.0"
edition = "2021"
rust-version = "1.75"
`)},
		"crates/api/src/handlers/users.rs": &fstest.MapFile{Data: []byte("pub fn list() {}\n")},
		"legacy/Cargo.toml":                &fstest.MapFile{Data: []byte("[package]\nname = \"legacy\"\nedition = \"2018\"\n")},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("crates/api/src/handlers/users.rs"))
	if v := findVersion(got, "crates/api/Cargo.toml", "edition"); v != "2021" {
		t.Fatalf("edition = %q, entries %#v", v, got)
	}
	if v := findVersion(got, "crates/api/Cargo.toml", "rust-version"); v != "1.75" {
		t.Fatalf("rust-version = %q, entries %#v", v, got)
	}
	for _, entry := range got {
		if entry.Source == "legacy/Cargo.toml" {
			t.Fatalf("unrelated crate must not be scanned: %#v", got)
		}
	}
}

func TestScanRustProjectsInheritsWorkspaceFields(t *testing.T) {
	fsys := fstest.MapFS{
		"Cargo.toml": &fstest.MapFile{Data: []byte(`[workspace]
members = ["crates/*"]
resolver = "3"

[workspace.package]
edition = "2024"
rust-version = "1.85"
`)},
		"crates/core/Cargo.toml": &fstest.MapFile{Data: []byte(`[package]
name = "core"
edition.workspace = true
rust-version = { workspace = true }
`)},
		"crates/core/src/lib.rs": &fstest.MapFile{Data: []byte("pub fn f() {}\n")},
	}
	got := scanRustProjects(fsys, []string{"crates/core/src/lib.rs"})
	if v := findVersion(got, "Cargo.toml", "workspace.package.edition"); v != "2024" {
		t.Fatalf("inherited edition = %q, entries %#v", v, got)
	}
	if v := findVersion(got, "Cargo.toml", "workspace.package.rust-version"); v != "1.85" {
		t.Fatalf("inherited rust-version = %q, entries %#v", v, got)
	}
}

func TestScanRustProjectsDefaultsToEdition2015(t *testing.T) {
	fsys := fstest.MapFS{
		"Cargo.toml":  &fstest.MapFile{Data: []byte("[package]\nname = \"old\"\nversion = \"0.1.0\"\n")},
		"src/main.rs": &fstest.MapFile{Data: []byte("fn main() {}\n")},
	}
	got := scanRustProjects(fsys, []string{"src/main.rs"})
	if v := findVersion(got, "Cargo.toml", "edition (Cargo default)"); v != "2015" {
		t.Fatalf("missing edition should default to 2015, entries %#v", got)
	}
}

func TestScanRustProjectsParseErrorAndChangedManifest(t *testing.T) {
	fsys := fstest.MapFS{"Cargo.toml": &fstest.MapFile{Data: []byte("[package]\nedition = \"2021\"\n")}}
	if v := findVersion(scanRustProjects(fsys, []string{"Cargo.toml"}), "Cargo.toml", "edition"); v != "2021" {
		t.Fatalf("a changed Cargo.toml should report its own edition")
	}
	// A manifest that no longer parses has no [package] the scan can trust,
	// so it is skipped rather than guessed at.
	broken := fstest.MapFS{"Cargo.toml": &fstest.MapFile{Data: []byte("[package\nedition = \"2021\"\n")}, "src/lib.rs": &fstest.MapFile{}}
	if got := scanRustProjects(broken, []string{"src/lib.rs"}); len(got) != 0 {
		t.Fatalf("unparsable manifest must not produce entries: %#v", got)
	}
}

func TestRustToolchainSources(t *testing.T) {
	fsys := fstest.MapFS{
		"rust-toolchain.toml": &fstest.MapFile{Data: []byte("[toolchain]\nchannel = \"1.80.1\"\ncomponents = [\"clippy\"]\n")},
		"rust-toolchain":      &fstest.MapFile{Data: []byte("nightly-2024-05-01\n")},
		".tool-versions":      &fstest.MapFile{Data: []byte("rust 1.82.0\n")},
		"Dockerfile":          &fstest.MapFile{Data: []byte("FROM rust:1.83-slim AS build\n")},
		"Cargo.toml":          &fstest.MapFile{Data: []byte("[package]\nname = \"x\"\nedition = \"2021\"\n")},
		"src/lib.rs":          &fstest.MapFile{Data: []byte("pub fn f() {}\n")},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("src/lib.rs"))
	checks := []struct{ source, field, want string }{
		{"rust-toolchain.toml", "toolchain.channel", "1.80.1"},
		{".tool-versions", "rust", "1.82.0"},
		{"Cargo.toml", "edition", "2021"},
	}
	for _, c := range checks {
		if v := findVersion(got, c.source, c.field); v != c.want {
			t.Errorf("%s %s = %q, want %q (entries %#v)", c.source, c.field, v, c.want, got)
		}
	}
	for _, entry := range got {
		if entry.Source == "rust-toolchain" {
			t.Errorf("a named channel does not pin a release: %#v", entry)
		}
		if entry.Language == langRust && entry.Field != "edition" {
			t.Errorf("rust entries must be editions only: %#v", entry)
		}
	}
	foundImage := false
	for _, entry := range got {
		if entry.Language == langRustc && entry.Source == "Dockerfile" && entry.Version == "1.83-slim" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Errorf("rust image missing: %#v", got)
	}
	if other := ScanFS(fsys, ctxWithChangedFiles("README.md")); len(other) != 0 {
		t.Fatalf("non-Rust change must not report Rust toolchains: %#v", other)
	}
}
