package review

import (
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

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
