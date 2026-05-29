package goparser

import (
	"context"
	"sync"
	"testing"
)

func TestBuildGraphCachedReusesAndIsConcurrencySafe(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/cache\n\ngo 1.25.0\n")
	writeTestFile(t, dir, "a/a.go", "package a\n\nfunc Run() {}\n")

	first, err := BuildGraphCached(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildGraphCached(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("BuildGraphCached rebuilt the graph instead of reusing the cached value")
	}

	// Concurrent first-access for many roots / repeated access for one root must
	// be race-free (run with -race).
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if _, err := BuildGraphCached(context.Background(), dir); err != nil {
				t.Errorf("concurrent BuildGraphCached: %v", err)
			}
		})
	}
	wg.Wait()
}

func TestFindClampsExcessiveDepth(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/depth\n\ngo 1.25.0\n")
	writeTestFile(t, dir, "a/a.go", "package a\n\nfunc Run() {}\n")

	graph, err := BuildGraph(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Find("Run", "a/a.go", 1_000_000_000, false)
	if err != nil {
		t.Fatal(err)
	}
	if h.Depth != MaxCallHierarchyDepth {
		t.Fatalf("depth = %d, want clamp to %d", h.Depth, MaxCallHierarchyDepth)
	}
}

// TestExpandKeepsPathLocalReexpansion verifies the shared-visited rewrite still
// shows a node reached via two sibling paths under BOTH parents (diamond),
// rather than collapsing to a single global visit.
func TestExpandKeepsPathLocalReexpansion(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/diamond\n\ngo 1.25.0\n")
	writeTestFile(t, dir, "d/d.go", `package d

func Leaf() {}

func A() { Leaf() }

func B() { Leaf() }

func Root() {
	A()
	B()
}
`)
	graph, err := BuildGraph(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Find("Root", "d/d.go", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Root.Children) != 2 {
		t.Fatalf("Root children = %d, want 2 (A,B)", len(h.Root.Children))
	}
	leafCount := 0
	for _, child := range h.Root.Children {
		for _, gc := range child.Children {
			if gc.Name == "Leaf" {
				leafCount++
			}
		}
	}
	if leafCount != 2 {
		t.Fatalf("Leaf appeared %d times under A/B, want 2 (path-local re-expansion)", leafCount)
	}
}
