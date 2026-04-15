package goparser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFindSymbol(t *testing.T) {
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
	symbol, err := FindSymbol(context.Background(), dir, "beta", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if symbol.Name != "beta" {
		t.Fatalf("symbol = %q", symbol.Name)
	}
}
