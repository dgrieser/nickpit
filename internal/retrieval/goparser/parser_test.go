package goparser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFindSymbols(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
func alpha() {}
func beta() {
	alpha()
}
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := FindSymbols(context.Background(), dir, "beta", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0].Name != "beta" {
		t.Fatalf("symbols = %#v", symbols)
	}
}

func TestFindSymbolsSkipsGitDirectory(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main\nfunc beta() {}\n")
	write(".git/junk.go", "package junk\nfunc beta() {}\n")

	symbols, err := FindSymbols(context.Background(), dir, "beta", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0].Path != "main.go" {
		t.Fatalf("symbols = %#v, want only main.go", symbols)
	}
}
