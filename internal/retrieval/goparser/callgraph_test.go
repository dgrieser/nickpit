package goparser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildGraphResolvesExactCallersAndCallees(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeTestFile(t, dir, "a/a.go", `package a

func Run() {}
`)
	writeTestFile(t, dir, "b/b.go", `package b

import "example.com/test/a"

func Run() {
	a.Run()
}
`)
	writeTestFile(t, dir, "main.go", `package main

import "example.com/test/b"

func Start() {
	b.Run()
}
`)

	graph, err := BuildGraph(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	callees, err := graph.Find("Run", "b/b.go", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(callees.Root.Children) != 1 {
		t.Fatalf("callees children = %d", len(callees.Root.Children))
	}
	if got := callees.Root.Children[0].Path; got != "a/a.go" {
		t.Fatalf("callee path = %q", got)
	}
	if !strings.Contains(callees.Root.Source, "func Run()") || !strings.Contains(callees.Root.Source, "a.Run()") {
		t.Fatalf("callee root source = %q", callees.Root.Source)
	}
	if !strings.Contains(callees.Root.Children[0].Source, "func Run()") {
		t.Fatalf("callee child source = %q", callees.Root.Children[0].Source)
	}

	callers, err := graph.Find("Run", "b/b.go", 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers.Root.Children) != 1 {
		t.Fatalf("callers children = %d", len(callers.Root.Children))
	}
	if got := callers.Root.Children[0].Name; got != "Start" {
		t.Fatalf("caller name = %q", got)
	}
	if !strings.Contains(callers.Root.Children[0].Source, "func Start()") || !strings.Contains(callers.Root.Children[0].Source, "b.Run()") {
		t.Fatalf("caller source = %q", callers.Root.Children[0].Source)
	}

	runCallers, err := graph.Find("Run", "a/a.go", 3, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(runCallers.Root.Children) != 1 {
		t.Fatalf("a.Run callers children = %d", len(runCallers.Root.Children))
	}
	if got := runCallers.Root.Children[0].Path; got != "b/b.go" {
		t.Fatalf("a.Run caller path = %q", got)
	}
	if len(runCallers.Root.Children[0].Children) != 1 {
		t.Fatalf("a.Run caller depth2 children = %d", len(runCallers.Root.Children[0].Children))
	}
	if got := runCallers.Root.Children[0].Children[0].Name; got != "Start" {
		t.Fatalf("a.Run caller grandchild name = %q", got)
	}
	if !strings.Contains(runCallers.Root.Source, "func Run()") {
		t.Fatalf("run root source = %q", runCallers.Root.Source)
	}
	if !strings.Contains(runCallers.Root.Children[0].Source, "func Run()") || !strings.Contains(runCallers.Root.Children[0].Source, "a.Run()") {
		t.Fatalf("run child source = %q", runCallers.Root.Children[0].Source)
	}
	if !strings.Contains(runCallers.Root.Children[0].Children[0].Source, "func Start()") || !strings.Contains(runCallers.Root.Children[0].Children[0].Source, "b.Run()") {
		t.Fatalf("run grandchild source = %q", runCallers.Root.Children[0].Children[0].Source)
	}
}

func TestBuildGraphResolvesRepoWideSymbolWhenPathEmpty(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeTestFile(t, dir, "a/a.go", `package a

func Alpha() {}
`)
	writeTestFile(t, dir, "b/b.go", `package b

import "example.com/test/a"

func Beta() {
	a.Alpha()
}
`)

	graph, err := BuildGraph(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	callers, err := graph.Find("Alpha", "", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := callers.Root.Path; got != "a/a.go" {
		t.Fatalf("root path = %q", got)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "Beta" {
		t.Fatalf("callers = %#v", callers.Root.Children)
	}
}

func TestBuildGraphResolvesDirectoryScopedSymbol(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeTestFile(t, dir, "pkg/one.go", `package pkg

func Shared() {}
`)
	writeTestFile(t, dir, "pkg/nested/two.go", `package nested

func Shared() {}

func Use() {
	Shared()
}
`)

	graph, err := BuildGraph(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	callers, err := graph.Find("Shared", "pkg/nested", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := callers.Root.Path; got != "pkg/nested/two.go" {
		t.Fatalf("root path = %q", got)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "Use" {
		t.Fatalf("callers = %#v", callers.Root.Children)
	}
}

func TestBuildGraphResolvesSymbolsInTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.25.0\n")
	writeTestFile(t, dir, "pkg/a.go", `package pkg

func Alpha() {}

func UseAlpha() {
	Alpha()
}
`)
	writeTestFile(t, dir, "pkg/a_test.go", `package pkg

import "testing"

func testHelper() {
	Alpha()
}

func TestAlpha(t *testing.T) {
	testHelper()
}
`)

	graph, err := BuildGraph(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	// A symbol defined in a _test.go file resolves callees and callers.
	callees, err := graph.Find("testHelper", "pkg/a_test.go", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(callees.Root.Children) != 1 || callees.Root.Children[0].Name != "Alpha" {
		t.Fatalf("testHelper callees = %#v", callees.Root.Children)
	}
	callers, err := graph.Find("testHelper", "pkg/a_test.go", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers.Root.Children) != 1 || callers.Root.Children[0].Name != "TestAlpha" {
		t.Fatalf("testHelper callers = %#v", callers.Root.Children)
	}

	// The base package is loaded twice (pkg and pkg [pkg.test]); non-test
	// symbols and their edges must not be duplicated.
	alphaCallers, err := graph.Find("Alpha", "pkg/a.go", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(alphaCallers.Root.Children))
	for _, child := range alphaCallers.Root.Children {
		names = append(names, child.Name)
	}
	if len(names) != 2 || names[0] != "UseAlpha" || names[1] != "testHelper" {
		t.Fatalf("Alpha callers = %v, want exactly [UseAlpha testHelper]", names)
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
