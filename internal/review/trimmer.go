package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

type Trimmer struct {
	maxTokens int
	estimator model.TokenEstimator
}

func NewTrimmer(maxTokens int, estimator model.TokenEstimator) *Trimmer {
	if estimator == nil {
		estimator = model.SimpleEstimator{}
	}
	if maxTokens <= 0 {
		maxTokens = 120000
	}
	return &Trimmer{maxTokens: maxTokens, estimator: estimator}
}

func (t *Trimmer) Trim(ctx *model.ReviewContext) *model.ReviewContext {
	cloned := model.CloneContext(ctx)
	if cloned == nil {
		return nil
	}
	if t.estimator.Estimate(renderContextText(cloned)) <= t.maxTokens {
		return cloned
	}

	cloned = t.dropGeneratedFiles(cloned)
	if t.estimator.Estimate(renderContextText(cloned)) <= t.maxTokens {
		return cloned
	}

	t.trimComments(cloned)
	t.trimCommits(cloned)
	t.trimSupplemental(cloned)
	t.trimDiff(cloned)
	return cloned
}

func (t *Trimmer) dropGeneratedFiles(ctx *model.ReviewContext) *model.ReviewContext {
	filtered := make([]model.ChangedFile, 0, len(ctx.ChangedFiles))
	dropped := make([]string, 0)
	for _, file := range ctx.ChangedFiles {
		if isGeneratedFile(file.Path) {
			dropped = append(dropped, file.Path)
			continue
		}
		filtered = append(filtered, file)
	}
	if len(dropped) == 0 {
		return ctx
	}
	ctx.ChangedFiles = filtered
	ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("generated files omitted: %s", strings.Join(dropped, ", ")))
	return ctx
}

func (t *Trimmer) trimComments(ctx *model.ReviewContext) {
	if len(ctx.Comments) < 2 {
		return
	}
	sort.Slice(ctx.Comments, func(i, j int) bool {
		return ctx.Comments[i].CreatedAt.After(ctx.Comments[j].CreatedAt)
	})
	for len(ctx.Comments) > 0 && t.estimator.Estimate(renderContextText(ctx)) > t.maxTokens {
		removed := len(ctx.Comments) / 2
		if removed == 0 {
			removed = 1
		}
		ctx.Comments = ctx.Comments[:len(ctx.Comments)-removed]
		ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("%d older comments omitted", removed))
	}
}

func (t *Trimmer) trimCommits(ctx *model.ReviewContext) {
	for i := range ctx.Commits {
		if idx := strings.IndexByte(ctx.Commits[i].Message, '\n'); idx > 0 {
			ctx.Commits[i].Message = ctx.Commits[i].Message[:idx]
		}
	}
	for len(ctx.Commits) > 0 && t.estimator.Estimate(renderContextText(ctx)) > t.maxTokens {
		ctx.Commits = ctx.Commits[:len(ctx.Commits)-1]
	}
}

func (t *Trimmer) trimSupplemental(ctx *model.ReviewContext) {
	for len(ctx.SupplementalContext) > 0 && t.estimator.Estimate(renderContextText(ctx)) > t.maxTokens {
		ctx.SupplementalContext = ctx.SupplementalContext[:len(ctx.SupplementalContext)-1]
	}
}

func (t *Trimmer) trimDiff(ctx *model.ReviewContext) {
	if len(ctx.DiffHunks) == 0 {
		for len(ctx.Diff) > 0 && t.estimator.Estimate(renderContextText(ctx)) > t.maxTokens {
			cut := len(ctx.Diff) / 4
			if cut == 0 {
				break
			}
			ctx.Diff = ctx.Diff[:len(ctx.Diff)-cut]
		}
		return
	}

	fileSizes := map[string]int{}
	for _, hunk := range ctx.DiffHunks {
		fileSizes[hunk.FilePath] += len(hunk.Content)
	}

	for t.estimator.Estimate(renderContextText(ctx)) > t.maxTokens && len(ctx.DiffHunks) > 1 {
		var worstPath string
		worstSize := -1
		for path, size := range fileSizes {
			if size > worstSize {
				worstPath = path
				worstSize = size
			}
		}
		pruned := make([]model.DiffHunk, 0, len(ctx.DiffHunks))
		removed := false
		for _, hunk := range ctx.DiffHunks {
			if !removed && hunk.FilePath == worstPath {
				fileSizes[hunk.FilePath] -= len(hunk.Content)
				removed = true
				continue
			}
			pruned = append(pruned, hunk)
		}
		if !removed {
			break
		}
		ctx.DiffHunks = pruned
	}

	var diff strings.Builder
	for _, hunk := range ctx.DiffHunks {
		diff.WriteString(fmt.Sprintf("--- %s\n", hunk.FilePath))
		diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines))
		diff.WriteString(hunk.Content)
		if !strings.HasSuffix(hunk.Content, "\n") {
			diff.WriteByte('\n')
		}
	}
	ctx.Diff = diff.String()
}

func renderContextText(ctx *model.ReviewContext) string {
	var b strings.Builder
	b.WriteString(ctx.Title)
	b.WriteString(ctx.Description)
	b.WriteString(ctx.Diff)
	for _, file := range ctx.ChangedFiles {
		b.WriteString(file.Path)
	}
	for _, commit := range ctx.Commits {
		b.WriteString(commit.Message)
	}
	for _, comment := range ctx.Comments {
		b.WriteString(comment.Body)
	}
	for _, supplemental := range ctx.SupplementalContext {
		b.WriteString(supplemental.Content)
	}
	return b.String()
}

func isGeneratedFile(path string) bool {
	return strings.HasSuffix(path, ".pb.go") ||
		strings.HasSuffix(path, "go.sum") ||
		strings.HasSuffix(path, "package-lock.json") ||
		strings.HasSuffix(path, "yarn.lock") ||
		strings.HasSuffix(path, "_pb2.py") ||
		strings.HasSuffix(path, "_pb2_grpc.py") ||
		strings.HasSuffix(path, "poetry.lock") ||
		strings.HasSuffix(path, "Pipfile.lock") ||
		strings.HasSuffix(path, "uv.lock")
}
