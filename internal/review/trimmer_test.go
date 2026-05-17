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
	})
	if err != nil {
		t.Fatalf("Trim: %v", err)
	}

	if len(ctx.ChangedFiles) != 1 || ctx.ChangedFiles[0].Path != "pkg/service.go" {
		t.Fatalf("changed files = %#v", ctx.ChangedFiles)
	}
	if len(ctx.OmittedSections) == 0 || !strings.Contains(ctx.OmittedSections[0], "generated files omitted") {
		t.Fatalf("omitted sections = %#v", ctx.OmittedSections)
	}
}
