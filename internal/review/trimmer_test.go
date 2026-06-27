package review

import (
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

type exactEstimator struct{}

func (exactEstimator) Estimate(text string) int {
	return len(text)
}

func TestTrimmerDropsGeneratedFilesFromEmbeddedMappings(t *testing.T) {
	trimmer := NewTrimmer(1, model.SimpleEstimator{})
	ctx, err := trimmer.Trim(&model.ReviewContext{
		Title: strings.Repeat("x", 100),
		ChangedFiles: []model.ChangedFile{
			{Path: "pkg/service.go", Status: model.FileModified},
			{Path: "pkg/service.pb.go", Status: model.FileModified},
			{Path: "web/package-lock.json", Status: model.FileModified},
		},
		Diff: "diff --git a/pkg/service.go b/pkg/service.go\n@@ -1 +1 @@\n-old\n+new\n" +
			"diff --git a/pkg/service.pb.go b/pkg/service.pb.go\n@@ -1 +1 @@\n-old\n+new\n" +
			"diff --git a/web/package-lock.json b/web/package-lock.json\n@@ -1 +1 @@\n-old\n+new\n",
		DiffFiles: []model.DiffFile{
			{FilePath: "pkg/service.go", Content: "diff --git a/pkg/service.go b/pkg/service.go\n@@ -1 +1 @@\n-old\n+new\n"},
			{FilePath: "pkg/service.pb.go", Content: "diff --git a/pkg/service.pb.go b/pkg/service.pb.go\n@@ -1 +1 @@\n-old\n+new\n"},
			{FilePath: "web/package-lock.json", Content: "diff --git a/web/package-lock.json b/web/package-lock.json\n@@ -1 +1 @@\n-old\n+new\n"},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "pkg/service.go", Content: "-old\n+new\n"},
			{FilePath: "pkg/service.pb.go", Content: "-old\n+new\n"},
			{FilePath: "web/package-lock.json", Content: "-old\n+new\n"},
		},
	})
	if err != nil {
		t.Fatalf("Trim: %v", err)
	}

	if len(ctx.ChangedFiles) != 1 || ctx.ChangedFiles[0].Path != "pkg/service.go" {
		t.Fatalf("changed files = %#v", ctx.ChangedFiles)
	}
	if len(ctx.DiffFiles) != 1 || ctx.DiffFiles[0].FilePath != "pkg/service.go" {
		t.Fatalf("diff files = %#v", ctx.DiffFiles)
	}
	if len(ctx.DiffHunks) != 1 || ctx.DiffHunks[0].FilePath != "pkg/service.go" {
		t.Fatalf("diff hunks = %#v", ctx.DiffHunks)
	}
	if strings.Contains(ctx.Diff, "service.pb.go") || strings.Contains(ctx.Diff, "package-lock.json") {
		t.Fatalf("diff still includes generated paths: %q", ctx.Diff)
	}
	if len(ctx.OmittedSections) == 0 || !strings.Contains(ctx.OmittedSections[0], "generated files omitted") {
		t.Fatalf("omitted sections = %#v", ctx.OmittedSections)
	}
}

func TestTrimmerDiffFilesPruningUsesPrunedRepresentation(t *testing.T) {
	diffFiles := []model.DiffFile{
		testDiffFile("a.go", 40),
		testDiffFile("b.go", 40),
		testDiffFile("c.go", 80),
	}
	ctx := &model.ReviewContext{
		Diff:      diffFileContent(diffFiles),
		DiffFiles: diffFiles,
		DiffHunks: []model.DiffHunk{
			{FilePath: "a.go", Content: strings.Repeat("a", 40)},
			{FilePath: "b.go", Content: strings.Repeat("b", 40)},
			{FilePath: "c.go", Content: strings.Repeat("c", 80)},
		},
	}
	trimmer := NewTrimmer(len(diffFiles[0].Content)+len(diffFiles[1].Content)+1, exactEstimator{})

	trimmed, err := trimmer.Trim(ctx)
	if err != nil {
		t.Fatalf("Trim: %v", err)
	}

	if len(trimmed.DiffFiles) != 2 {
		t.Fatalf("diff files = %#v, want two files after pruning one oversized file", trimmed.DiffFiles)
	}
	if containsDiffFile(trimmed.DiffFiles, "c.go") {
		t.Fatalf("largest diff file was not pruned: %#v", trimmed.DiffFiles)
	}
	if len(trimmed.DiffHunks) != 2 || containsDiffHunk(trimmed.DiffHunks, "c.go") {
		t.Fatalf("diff hunks not kept in sync: %#v", trimmed.DiffHunks)
	}
	if strings.Contains(trimmed.Diff, "c.go") {
		t.Fatalf("rebuilt diff still contains pruned file: %q", trimmed.Diff)
	}
}

func TestTrimmerDiffHunksPruningUsesPrunedRepresentation(t *testing.T) {
	hunks := []model.DiffHunk{
		{FilePath: "a.go", Content: strings.Repeat("a", 40)},
		{FilePath: "b.go", Content: strings.Repeat("b", 40)},
		{FilePath: "c.go", Content: strings.Repeat("c", 80)},
	}
	ctx := &model.ReviewContext{
		Diff:      strings.Repeat("x", 300),
		DiffHunks: hunks,
	}
	trimmer := NewTrimmer(len(hunks[0].Content)+len(hunks[1].Content)+1, exactEstimator{})

	trimmed, err := trimmer.Trim(ctx)
	if err != nil {
		t.Fatalf("Trim: %v", err)
	}

	if len(trimmed.DiffHunks) != 2 {
		t.Fatalf("diff hunks = %#v, want two hunks after pruning one oversized hunk", trimmed.DiffHunks)
	}
	if containsDiffHunk(trimmed.DiffHunks, "c.go") {
		t.Fatalf("largest hunk was not pruned: %#v", trimmed.DiffHunks)
	}
	if strings.Contains(trimmed.Diff, "c.go") {
		t.Fatalf("rebuilt diff still contains pruned hunk: %q", trimmed.Diff)
	}
}

func testDiffFile(path string, size int) model.DiffFile {
	content := "diff --git a/" + path + " b/" + path + "\n" +
		"@@ -1 +1 @@\n" +
		"-" + strings.Repeat("o", size) + "\n" +
		"+" + strings.Repeat("n", size) + "\n"
	return model.DiffFile{FilePath: path, Language: "go", Content: content}
}

func diffFileContent(files []model.DiffFile) string {
	var out strings.Builder
	for _, file := range files {
		out.WriteString(file.Content)
	}
	return out.String()
}

func containsDiffFile(files []model.DiffFile, path string) bool {
	for _, file := range files {
		if file.FilePath == path {
			return true
		}
	}
	return false
}

func containsDiffHunk(hunks []model.DiffHunk, path string) bool {
	for _, hunk := range hunks {
		if hunk.FilePath == path {
			return true
		}
	}
	return false
}
