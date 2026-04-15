package goparser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGraphResolvesExactCallersAndCallees(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22.0\n")
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
