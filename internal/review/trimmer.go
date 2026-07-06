package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

type Trimmer struct {
	maxTokens int
	headroom  int
	estimator model.TokenEstimator
	lenEst    model.LengthEstimator
	less      func(a, b evictionCandidate) bool
}

// evictionCandidate describes a diff file (or hunk group) competing for
// eviction: lower Class is evicted earlier, then larger Size, then Path for
// deterministic ties.
type evictionCandidate struct {
	Path  string
	Size  int
	Class int
}

type TrimmerOption func(*Trimmer)

// WithHeadroomTokens reserves tokens off the budget for prompt overhead the
// context text itself does not account for (JSON envelope, instructions).
func WithHeadroomTokens(tokens int) TrimmerOption {
	return func(t *Trimmer) {
		if tokens > 0 {
			t.headroom = tokens
		}
	}
}

func NewTrimmer(maxTokens int, estimator model.TokenEstimator, opts ...TrimmerOption) *Trimmer {
	if estimator == nil {
		estimator = model.SimpleEstimator{}
	}
	if maxTokens <= 0 {
		maxTokens = config.DefaultMaxContextToken
	}
	t := &Trimmer{maxTokens: maxTokens, estimator: estimator, less: defaultEvictionLess}
	t.lenEst, _ = estimator.(model.LengthEstimator)
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Trimmer) budget() int {
	return max(t.maxTokens-t.headroom, 1)
}

func (t *Trimmer) overBudget(ctx *model.ReviewContext) bool {
	return t.estimator.Estimate(renderContextText(ctx)) > t.budget()
}

func (t *Trimmer) Trim(ctx *model.ReviewContext) (*model.ReviewContext, error) {
	cloned, err := model.CloneContext(ctx)
	if err != nil {
		return nil, err
	}
	if cloned == nil {
		return nil, nil
	}
	if !t.overBudget(cloned) {
		return cloned, nil
	}

	cloned = t.dropGeneratedFiles(cloned)
	if !t.overBudget(cloned) {
		return cloned, nil
	}

	t.trimComments(cloned)
	t.trimSupplemental(cloned)
	t.trimCommits(cloned)
	t.trimDiff(cloned)
	return cloned, nil
}

// budgetTracker keeps eviction loops honest without re-rendering the whole
// context per iteration: with a length-based estimator the total length is
// computed once and decremented by exactly what each eviction removes
// (renderContextText is pure concatenation, so lengths are additive). Other
// estimators fall back to a full render per check.
type budgetTracker struct {
	trimmer *Trimmer
	ctx     *model.ReviewContext
	total   int
}

func (t *Trimmer) newTracker(ctx *model.ReviewContext) *budgetTracker {
	tracker := &budgetTracker{trimmer: t, ctx: ctx}
	if t.lenEst != nil {
		tracker.total = len(renderContextText(ctx))
	}
	return tracker
}

func (b *budgetTracker) over() bool {
	if b.trimmer.lenEst != nil {
		return b.trimmer.lenEst.EstimateLen(b.total) > b.trimmer.budget()
	}
	return b.trimmer.overBudget(b.ctx)
}

func (b *budgetTracker) evicted(bytes int) {
	b.total -= bytes
}

func (t *Trimmer) dropGeneratedFiles(ctx *model.ReviewContext) *model.ReviewContext {
	generated := make(map[string]bool)
	for _, file := range ctx.ChangedFiles {
		if file.Generated {
			generated[normalizeReviewPath(file.Path)] = true
		}
	}
	for _, file := range ctx.DiffFiles {
		if file.Generated {
			generated[normalizeReviewPath(file.FilePath)] = true
		}
	}
	if len(generated) == 0 {
		return ctx
	}

	filtered := make([]model.ChangedFile, 0, len(ctx.ChangedFiles))
	dropped := make([]string, 0, len(generated))
	droppedByPath := make(map[string]bool, len(generated))
	for _, file := range ctx.ChangedFiles {
		if generated[normalizeReviewPath(file.Path)] {
			dropped = append(dropped, file.Path)
			droppedByPath[normalizeReviewPath(file.Path)] = true
			continue
		}
		filtered = append(filtered, file)
	}
	keepByPath := make(map[string]bool, len(filtered)+len(ctx.DiffFiles))
	for _, file := range filtered {
		keepByPath[normalizeReviewPath(file.Path)] = true
	}
	for _, file := range ctx.DiffFiles {
		normalized := normalizeReviewPath(file.FilePath)
		if generated[normalized] {
			if !droppedByPath[normalized] {
				dropped = append(dropped, file.FilePath)
				droppedByPath[normalized] = true
			}
			continue
		}
		keepByPath[normalized] = true
	}
	ctx.ChangedFiles = filtered
	ctx.DiffFiles = filterDiffFiles(ctx.DiffFiles, keepByPath)
	ctx.DiffHunks = filterDiffHunks(ctx.DiffHunks, keepByPath)
	ctx.Diff = filterUnifiedDiff(ctx.Diff, keepByPath)
	supplemental := make([]model.SupplementalFile, 0, len(ctx.SupplementalContext))
	for _, file := range ctx.SupplementalContext {
		if file.Kind == "full_file" && generated[normalizeReviewPath(file.Path)] {
			continue
		}
		supplemental = append(supplemental, file)
	}
	ctx.SupplementalContext = supplemental
	ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("generated files omitted: %s", strings.Join(dropped, ", ")))
	return ctx
}

func (t *Trimmer) trimComments(ctx *model.ReviewContext) {
	if len(ctx.Comments) < 2 {
		return
	}
	tracker := t.newTracker(ctx)
	if !tracker.over() {
		return
	}
	sort.Slice(ctx.Comments, func(i, j int) bool {
		return ctx.Comments[i].CreatedAt.After(ctx.Comments[j].CreatedAt)
	})
	for len(ctx.Comments) > 0 && tracker.over() {
		removed := len(ctx.Comments) / 2
		if removed == 0 {
			removed = 1
		}
		for _, comment := range ctx.Comments[len(ctx.Comments)-removed:] {
			tracker.evicted(len(comment.Body))
		}
		ctx.Comments = ctx.Comments[:len(ctx.Comments)-removed]
		ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("%d older comments omitted", removed))
	}
}

func (t *Trimmer) trimSupplemental(ctx *model.ReviewContext) {
	if len(ctx.SupplementalContext) == 0 {
		return
	}
	tracker := t.newTracker(ctx)
	if !tracker.over() {
		return
	}
	var dropped []string
	for len(ctx.SupplementalContext) > 0 && tracker.over() {
		last := ctx.SupplementalContext[len(ctx.SupplementalContext)-1]
		tracker.evicted(len(last.Content))
		name := last.Path
		if name == "" {
			name = last.Kind
		}
		dropped = append(dropped, name)
		ctx.SupplementalContext = ctx.SupplementalContext[:len(ctx.SupplementalContext)-1]
	}
	if len(dropped) > 0 {
		ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("supplemental context omitted: %s", strings.Join(dropped, ", ")))
	}
}

func (t *Trimmer) trimCommits(ctx *model.ReviewContext) {
	if len(ctx.Commits) == 0 {
		return
	}
	tracker := t.newTracker(ctx)
	if !tracker.over() {
		return
	}
	flattened := 0
	for i := range ctx.Commits {
		if idx := strings.IndexByte(ctx.Commits[i].Message, '\n'); idx > 0 {
			tracker.evicted(len(ctx.Commits[i].Message) - idx)
			ctx.Commits[i].Message = ctx.Commits[i].Message[:idx]
			flattened++
		}
	}
	if flattened > 0 {
		ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("%d commit message bodies omitted (subjects kept)", flattened))
	}
	dropped := 0
	for len(ctx.Commits) > 0 && tracker.over() {
		tracker.evicted(len(ctx.Commits[len(ctx.Commits)-1].Message))
		ctx.Commits = ctx.Commits[:len(ctx.Commits)-1]
		dropped++
	}
	if dropped > 0 {
		ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("%d commits omitted", dropped))
	}
}

func (t *Trimmer) trimDiff(ctx *model.ReviewContext) {
	if len(ctx.DiffFiles) > 0 {
		t.trimDiffFiles(ctx)
		return
	}
	if len(ctx.DiffHunks) == 0 {
		tracker := t.newTracker(ctx)
		originalLen := len(ctx.Diff)
		for len(ctx.Diff) > 0 && tracker.over() {
			cut := len(ctx.Diff) / 4
			if cut == 0 {
				break
			}
			tracker.evicted(cut)
			ctx.Diff = ctx.Diff[:len(ctx.Diff)-cut]
		}
		if removed := originalLen - len(ctx.Diff); removed > 0 {
			ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("unified diff truncated: removed %d of %d bytes", removed, originalLen))
		}
		return
	}
	ctx.Diff = ""
	tracker := t.newTracker(ctx)

	fileSizes := map[string]int{}
	for _, hunk := range ctx.DiffHunks {
		fileSizes[hunk.FilePath] += len(hunk.Content)
	}
	classByPath := evictionClasses(fileSizes)

	droppedBytes := map[string]int{}
	for tracker.over() && len(ctx.DiffHunks) > 1 {
		worst, ok := worstCandidate(fileSizes, classByPath, t.less)
		if !ok {
			break
		}
		pruned := make([]model.DiffHunk, 0, len(ctx.DiffHunks))
		removed := false
		for _, hunk := range ctx.DiffHunks {
			if !removed && hunk.FilePath == worst.Path {
				fileSizes[hunk.FilePath] -= len(hunk.Content)
				if fileSizes[hunk.FilePath] <= 0 {
					delete(fileSizes, hunk.FilePath)
				}
				tracker.evicted(len(hunk.Content))
				droppedBytes[hunk.FilePath] += len(hunk.Content)
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
	appendEvictionOmission(ctx, "diff hunks omitted to fit context budget", droppedBytes)

	var diff strings.Builder
	for _, hunk := range ctx.DiffHunks {
		fmt.Fprintf(&diff, "--- %s\n", hunk.FilePath)
		fmt.Fprintf(&diff, "@@ -%d,%d +%d,%d @@\n", hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines)
		diff.WriteString(hunk.Content)
		if !strings.HasSuffix(hunk.Content, "\n") {
			diff.WriteByte('\n')
		}
	}
	ctx.Diff = diff.String()
}

func (t *Trimmer) trimDiffFiles(ctx *model.ReviewContext) {
	ctx.Diff = ""
	tracker := t.newTracker(ctx)

	fileSizes := map[string]int{}
	for _, file := range ctx.DiffFiles {
		fileSizes[file.FilePath] += len(file.Content)
	}
	classByPath := evictionClasses(fileSizes)

	droppedBytes := map[string]int{}
	for tracker.over() && len(ctx.DiffFiles) > 1 {
		worst, ok := worstCandidate(fileSizes, classByPath, t.less)
		if !ok {
			break
		}
		pruned := make([]model.DiffFile, 0, len(ctx.DiffFiles))
		removed := false
		for _, file := range ctx.DiffFiles {
			if !removed && file.FilePath == worst.Path {
				fileSizes[file.FilePath] -= len(file.Content)
				if fileSizes[file.FilePath] <= 0 {
					delete(fileSizes, file.FilePath)
				}
				tracker.evicted(len(file.Content))
				droppedBytes[file.FilePath] += len(file.Content)
				removed = true
				continue
			}
			pruned = append(pruned, file)
		}
		if !removed {
			break
		}
		ctx.DiffFiles = pruned
	}
	appendEvictionOmission(ctx, "diff files omitted to fit context budget", droppedBytes)

	keepByPath := make(map[string]bool, len(ctx.DiffFiles))
	var diff strings.Builder
	for _, file := range ctx.DiffFiles {
		keepByPath[normalizeReviewPath(file.FilePath)] = true
		diff.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			diff.WriteByte('\n')
		}
	}
	ctx.Diff = diff.String()
	ctx.DiffHunks = filterDiffHunks(ctx.DiffHunks, keepByPath)
}

func defaultEvictionLess(a, b evictionCandidate) bool {
	if a.Class != b.Class {
		return a.Class < b.Class
	}
	if a.Size != b.Size {
		return a.Size > b.Size
	}
	return a.Path < b.Path
}

func evictionClasses(fileSizes map[string]int) map[string]int {
	classes := make(map[string]int, len(fileSizes))
	for path := range fileSizes {
		classes[path] = filetype.EvictionClass(path)
	}
	return classes
}

// worstCandidate returns the next file to evict: the minimum under less.
func worstCandidate(fileSizes map[string]int, classByPath map[string]int, less func(a, b evictionCandidate) bool) (evictionCandidate, bool) {
	var worst evictionCandidate
	found := false
	for path, size := range fileSizes {
		candidate := evictionCandidate{Path: path, Size: size, Class: classByPath[path]}
		if !found || less(candidate, worst) {
			worst = candidate
			found = true
		}
	}
	return worst, found
}

func appendEvictionOmission(ctx *model.ReviewContext, label string, droppedBytes map[string]int) {
	if len(droppedBytes) == 0 {
		return
	}
	paths := make([]string, 0, len(droppedBytes))
	for path := range droppedBytes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	entries := make([]string, 0, len(paths))
	for _, path := range paths {
		entries = append(entries, fmt.Sprintf("%s (%d bytes)", path, droppedBytes[path]))
	}
	ctx.OmittedSections = append(ctx.OmittedSections, fmt.Sprintf("%s: %s", label, strings.Join(entries, ", ")))
}

// promptOverheadTokens measures how many tokens the JSON prompt envelope adds
// on top of the raw context text, so the trimmer can reserve headroom for it.
// The payload is rendered with the diff format the review will actually use —
// git-json carries per-hunk metadata the default format does not.
func promptOverheadTokens(estimator model.TokenEstimator, ctx *model.ReviewContext, format model.DiffFormat, maxTokens int) int {
	if ctx == nil || estimator == nil {
		return 0
	}
	payload, err := llm.RenderJSON(model.PromptPayloadFromContextWithDiffFormat(ctx, format))
	if err != nil {
		return 0
	}
	overhead := estimator.Estimate(payload) - estimator.Estimate(renderContextText(ctx))
	if overhead <= 0 {
		return 0
	}
	if limit := maxTokens / 4; overhead > limit {
		return limit
	}
	return overhead
}

func renderContextText(ctx *model.ReviewContext) string {
	var b strings.Builder
	b.WriteString(ctx.Title)
	b.WriteString(ctx.Description)
	b.WriteString(ctx.Diff)
	if ctx.Diff == "" {
		if len(ctx.DiffFiles) > 0 {
			for _, file := range ctx.DiffFiles {
				b.WriteString(file.Content)
			}
		} else {
			for _, hunk := range ctx.DiffHunks {
				b.WriteString(hunk.Content)
			}
		}
	}
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
