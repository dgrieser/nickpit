package toolchain

import (
	"testing"
	"testing/fstest"
)

func TestCSharpForTFM(t *testing.T) {
	tests := []struct{ tfm, want string }{
		{"net48", "7.3"},
		{"net472", "7.3"},
		{"netstandard2.0", "7.3"},
		{"netcoreapp2.1", "7.3"},
		{"netstandard2.1", "8.0"},
		{"netcoreapp3.1", "8.0"},
		{"net5.0", "9.0"},
		{"net6.0", "10.0"},
		{"net7.0", "11.0"},
		{"net8.0", "12.0"},
		{"net8.0-windows", "12.0"},
		{"NET9.0", "13.0"},
		{"net10.0", "14.0"},
		{"net11.0", "15.0"},
		{"$(DefaultTargetFramework)", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := csharpForTFM(tc.tfm); got != tc.want {
			t.Errorf("csharpForTFM(%q) = %q, want %q", tc.tfm, got, tc.want)
		}
	}
}

func TestScanDotnetProjectsUsesOwningProject(t *testing.T) {
	fsys := fstest.MapFS{
		"src/Api/Api.csproj": &fstest.MapFile{Data: []byte(`<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`)},
		"src/Api/Controllers/Users.cs": &fstest.MapFile{Data: []byte("class Users {}")},
		"legacy/Old.csproj":            &fstest.MapFile{Data: []byte(`<Project><PropertyGroup><TargetFramework>net48</TargetFramework></PropertyGroup></Project>`)},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("src/Api/Controllers/Users.cs"))
	if v := findVersion(got, "src/Api/Api.csproj", "TargetFramework net8.0"); v != "12.0" {
		t.Fatalf("expected C# 12 from net8.0, got %#v", got)
	}
	for _, entry := range got {
		if entry.Source == "legacy/Old.csproj" {
			t.Fatalf("unrelated project must not be scanned: %#v", got)
		}
	}
}

func TestScanDotnetProjectsExplicitLangVersionWins(t *testing.T) {
	fsys := fstest.MapFS{
		"App.csproj": &fstest.MapFile{Data: []byte(`<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFrameworks>net6.0;net8.0</TargetFrameworks>
    <LangVersion>11</LangVersion>
  </PropertyGroup>
</Project>`)},
	}
	got := scanDotnetProjects(fsys, []string{"Program.cs"})
	if len(got) != 1 || got[0].Field != "LangVersion" || got[0].Version != "11" {
		t.Fatalf("explicit LangVersion should be the only entry, got %#v", got)
	}
}

func TestScanDotnetProjectsMultiTargeting(t *testing.T) {
	fsys := fstest.MapFS{
		"lib/Lib.csproj": &fstest.MapFile{Data: []byte(`<Project><PropertyGroup><TargetFrameworks>netstandard2.0;net8.0</TargetFrameworks><LangVersion>latest</LangVersion></PropertyGroup></Project>`)},
	}
	got := scanDotnetProjects(fsys, []string{"lib/Thing.cs"})
	if findVersion(got, "lib/Lib.csproj", "TargetFramework netstandard2.0") != "7.3" || findVersion(got, "lib/Lib.csproj", "TargetFramework net8.0") != "12.0" {
		t.Fatalf("symbolic LangVersion should fall back to every framework default, got %#v", got)
	}
}

func TestScanDotnetProjectsInheritsDirectoryBuildProps(t *testing.T) {
	fsys := fstest.MapFS{
		"Directory.Build.props":     &fstest.MapFile{Data: []byte(`<Project><PropertyGroup><LangVersion>12.0</LangVersion><TargetFramework>net6.0</TargetFramework></PropertyGroup></Project>`)},
		"src/Directory.Build.props": &fstest.MapFile{Data: []byte(`<Project><PropertyGroup><TargetFramework>net9.0</TargetFramework></PropertyGroup></Project>`)},
		"src/Svc/Svc.csproj":        &fstest.MapFile{Data: []byte(`<Project Sdk="Microsoft.NET.Sdk"></Project>`)},
	}
	got := scanDotnetProjects(fsys, []string{"src/Svc/Worker.cs"})
	if len(got) != 1 || got[0].Source != "Directory.Build.props" || got[0].Version != "12.0" {
		t.Fatalf("inherited LangVersion should apply, got %#v", got)
	}

	delete(fsys, "Directory.Build.props")
	got = scanDotnetProjects(fsys, []string{"src/Svc/Worker.cs"})
	if findVersion(got, "src/Svc/Svc.csproj", "TargetFramework net9.0") != "13.0" {
		t.Fatalf("nearest Directory.Build.props framework should apply, got %#v", got)
	}
}

func TestScanDotnetProjectsParseError(t *testing.T) {
	fsys := fstest.MapFS{"Bad.csproj": &fstest.MapFile{Data: []byte(`<Project><PropertyGroup>`)}}
	got := scanDotnetProjects(fsys, []string{"Bad.csproj"})
	if len(got) != 1 || got[0].Error == "" {
		t.Fatalf("expected an error entry, got %#v", got)
	}
}

func TestDotnetImagesAndSetupDotnet(t *testing.T) {
	fsys := fstest.MapFS{
		"Dockerfile": &fstest.MapFile{Data: []byte("FROM mcr.microsoft.com/dotnet/sdk:9.0 AS build\nFROM mcr.microsoft.com/dotnet/aspnet:9.0-alpine\n")},
		".github/workflows/ci.yml": &fstest.MapFile{Data: []byte(`jobs:
  build:
    steps:
      - uses: actions/setup-dotnet@v4
        with:
          dotnet-version: '8.0.x'
`)},
		".tool-versions": &fstest.MapFile{Data: []byte("dotnet 10.0.100\n")},
	}
	got := ScanFS(fsys, ctxWithChangedFiles("Program.cs"))
	if findVersion(got, "Dockerfile", "FROM mcr.microsoft.com/dotnet/sdk") != "13.0" {
		t.Errorf("sdk image missing: %#v", got)
	}
	if findVersion(got, "Dockerfile", "FROM mcr.microsoft.com/dotnet/aspnet") != "13.0" {
		t.Errorf("aspnet image missing: %#v", got)
	}
	if findVersion(got, ".github/workflows/ci.yml", "dotnet-version 8.0.x") != "12.0" {
		t.Errorf("setup-dotnet missing: %#v", got)
	}
	if findVersion(got, ".tool-versions", "dotnet 10.0.100") != "14.0" {
		t.Errorf(".tool-versions dotnet missing: %#v", got)
	}
}
