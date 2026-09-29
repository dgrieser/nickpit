package review

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestAllowedDiffCodeLocationsAreCompleteCodeLocationJSON(t *testing.T) {
	hunks := []model.DiffHunk{{
		FilePath: "pkg/demo.go",
		Language: "go",
		OldStart: 10,
		OldLines: 2,
		NewStart: 10,
		NewLines: 2,
		Content:  " context\n-old()\n+new()\n",
	}}

	allowed := allowedDiffCodeLocations(hunks, nil)
	if len(allowed) != 2 {
		t.Fatalf("allowed locations = %#v, want old and new code_location", allowed)
	}
	contents := map[string]bool{}
	for _, got := range allowed {
		if got.FilePath != "pkg/demo.go" ||
			got.LineRange != (model.LineRange{Start: 10, End: 11, Count: 2}) ||
			got.Language != "go" {
			t.Fatalf("code_location = %#v", got)
		}
		contents[got.Content] = true
	}
	if !contents["context\nold()"] || !contents["context\nnew()"] {
		t.Fatalf("allowed contents = %#v, want old and new sides", contents)
	}

	encoded, err := json.Marshal(allowed)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []model.CodeLocation
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("allowed windows are not code_location JSON: %v", err)
	}
	jsonText := string(encoded)
	for _, field := range []string{`"file_path"`, `"line_range"`, `"start"`, `"end"`, `"count"`, `"language"`, `"content"`} {
		if !strings.Contains(jsonText, field) {
			t.Fatalf("allowed JSON %s missing %s", jsonText, field)
		}
	}
}

func TestAllowedDiffCodeLocationsSkipsEmptySides(t *testing.T) {
	hunks := []model.DiffHunk{
		{FilePath: "added.go", NewStart: 4, NewLines: 1, Content: "+added()\n"},
		{FilePath: "deleted.go", OldStart: 7, OldLines: 1, Content: "-deleted()\n"},
	}
	allowed := allowedDiffCodeLocations(hunks, nil)
	if len(allowed) != 2 {
		t.Fatalf("allowed locations = %#v, want one non-empty side per hunk", allowed)
	}
}

func TestCodeLocationOverlapsDiffAcceptsAnyOldOrNewSideIntersection(t *testing.T) {
	hunks := []model.DiffHunk{
		{FilePath: "f.go", OldStart: 10, OldLines: 3, NewStart: 10, NewLines: 2},
		{FilePath: "f.go", OldStart: 30, OldLines: 2, NewStart: 29, NewLines: 3},
	}
	tests := []struct {
		name  string
		path  string
		start int
		end   int
		want  bool
	}{
		{name: "new-side line", path: "f.go", start: 11, end: 11, want: true},
		{name: "deleted-only old-side line", path: "f.go", start: 12, end: 12, want: true},
		{name: "partial overlap", path: "f.go", start: 1, end: 10, want: true},
		{name: "range spans hunk gap", path: "f.go", start: 11, end: 30, want: true},
		{name: "wholly between hunks", path: "f.go", start: 13, end: 28, want: false},
		{name: "outside after hunks", path: "f.go", start: 40, end: 50, want: false},
		{name: "wrong file", path: "other.go", start: 10, end: 10, want: false},
	}
	allowed := allowedDiffCodeLocations(hunks, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := model.CodeLocation{FilePath: tt.path, LineRange: model.LineRange{Start: tt.start, End: tt.end}}
			if got := codeLocationOverlapsAllowed(loc, allowed); got != tt.want {
				t.Fatalf("codeLocationOverlapsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodeLocationOverlapsLegacyContentOnlyHunk(t *testing.T) {
	hunks := []model.DiffHunk{{
		FilePath: "f.go",
		OldStart: 5,
		NewStart: 5,
		Content:  " old\n-removed\n+added\n context\n",
	}}
	allowed := allowedDiffCodeLocations(hunks, nil)
	for _, line := range []int{5, 6, 7} {
		loc := model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: line, End: line}}
		if !codeLocationOverlapsAllowed(loc, allowed) {
			t.Fatalf("line %d should overlap legacy hunk", line)
		}
	}
}

func TestPipelineAssembleAppliesFinalDiffScopeSafeguard(t *testing.T) {
	ctx := &model.ReviewContext{DiffScopeHunks: []model.DiffHunk{{
		FilePath: "f.go",
		OldStart: 10,
		OldLines: 1,
		NewStart: 10,
		NewLines: 1,
	}}}
	st := newPipelineState(ctx, nil)
	st.result = &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "inside", CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 10, End: 10}}},
			{Title: "outside", CodeLocation: model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		},
		OverallCorrectness: "patch is incorrect",
	}
	pipeline := &Pipeline{engine: &Engine{}}
	result := pipeline.assemble(st, model.ReviewRequest{})
	if len(result.Findings) != 1 || result.Findings[0].Title != "inside" {
		t.Fatalf("findings = %#v", result.Findings)
	}
}

func TestPrepareFindingsForVerificationAggregatesOutOfDiffWarnings(t *testing.T) {
	ctx := &model.ReviewContext{DiffScopeHunks: []model.DiffHunk{{
		FilePath: "f.go",
		OldStart: 10,
		OldLines: 1,
		NewStart: 10,
		NewLines: 1,
		Content:  " changed()\n",
	}}}
	results := []agentResult{{
		run: model.AgentRun{Name: "Security"},
		resp: &llm.ReviewResponse{Findings: []model.Finding{
			{Title: "outside one", CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 1, End: 1}, Content: "one()"}},
			{Title: "outside two", CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 2, End: 2}, Content: "two()"}},
		}},
	}}
	engine := NewEngine(nil, nil, nil, config.Profile{})

	warnings := engine.prepareFindingsForVerification(context.Background(), ctx, results, nil, model.ReviewRequest{})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Dropped 2 out-of-diff finding(s) from Security") {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(results[0].resp.Findings) != 0 {
		t.Fatalf("findings = %#v, want both dropped", results[0].resp.Findings)
	}
}

// Renaming a symlink emits no hunk at all, so without a scopeable location the
// reviewer is told the entry is a symlink and then has every finding about it
// dropped. The single line of a symlink blob is its target, so line 1 is the file.
func TestAllowedDiffCodeLocationsCoversMetadataOnlySymlinkChanges(t *testing.T) {
	changed := []model.ChangedFile{
		{Path: "dir/link2", Status: model.FileRenamed, OldPath: "dir/sub/link", Symlink: true, SymlinkTarget: "../target"},
		// A regular rename keeps the existing policy: no hunk, no scope.
		{Path: "docs/moved.md", Status: model.FileRenamed, OldPath: "docs/old.md"},
		// A symlink that does have a hunk is scoped by that hunk alone.
		{Path: "added-link", Status: model.FileAdded, Symlink: true},
	}
	hunks := []model.DiffHunk{
		{FilePath: "added-link", NewStart: 1, NewLines: 1, Content: "+target", Symlink: true},
	}

	allowed := allowedDiffCodeLocations(hunks, changed)

	var renamedLink *model.CodeLocation
	for i := range allowed {
		if allowed[i].FilePath == "dir/link2" {
			renamedLink = &allowed[i]
		}
		if allowed[i].FilePath == "docs/moved.md" {
			t.Fatalf("a regular rename gained a scope: %#v", allowed[i])
		}
	}
	if renamedLink == nil {
		t.Fatalf("renamed symlink has no allowed location: %#v", allowed)
	}
	if renamedLink.LineRange.Start != 1 || renamedLink.LineRange.End != 1 || renamedLink.Content != "../target" {
		t.Fatalf("allowed location = %#v, want line 1 carrying the target", *renamedLink)
	}
	// One location per path, not one per hunk-less entry plus a duplicate.
	count := 0
	for _, loc := range allowed {
		if loc.FilePath == "added-link" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("added symlink locations = %d, want only its hunk", count)
	}
}

// The whole point: a finding about a renamed symlink must survive the gate.
func TestFilterFindingsByDiffScopeKeepsRenamedSymlinkFinding(t *testing.T) {
	changed := []model.ChangedFile{
		{Path: "dir/link2", Status: model.FileRenamed, OldPath: "dir/sub/link", Symlink: true, SymlinkTarget: "../target"},
	}
	findings := []model.Finding{
		{
			Title: "Relative symlink target no longer resolves after the move",
			CodeLocation: model.CodeLocation{
				FilePath:  "dir/link2",
				LineRange: model.LineRange{Start: 1, End: 1, Count: 1},
			},
		},
	}

	kept, dropped := filterFindingsByDiffScope(findings, nil, changed)

	if len(kept) != 1 || len(dropped) != 0 {
		t.Fatalf("kept = %d, dropped = %d, want the finding kept", len(kept), len(dropped))
	}
}

// Backslashes and spaces are legal in Unix filenames, so a symlink named `a\b` and
// a regular file `a/b` are two different files. Folding them would let one file's
// hunk cancel the other's location, or lend it scope for a finding about the wrong
// file.
func TestAllowedDiffCodeLocationsKeepsPathsLiteral(t *testing.T) {
	changed := []model.ChangedFile{
		{Path: `a\b`, Status: model.FileRenamed, OldPath: `dir/a\b`, Symlink: true, SymlinkTarget: "../target"},
	}
	hunks := []model.DiffHunk{
		{FilePath: "a/b", NewStart: 4, NewLines: 2, Content: "+text\n+more"},
	}

	allowed := allowedDiffCodeLocations(hunks, changed)

	var symlinkLoc, hunkLoc *model.CodeLocation
	for i := range allowed {
		switch allowed[i].FilePath {
		case `a\b`:
			symlinkLoc = &allowed[i]
		case "a/b":
			hunkLoc = &allowed[i]
		default:
			t.Fatalf("path was rewritten: %q", allowed[i].FilePath)
		}
	}
	if symlinkLoc == nil {
		t.Fatalf("the other file's hunk cancelled the symlink location: %#v", allowed)
	}
	if hunkLoc == nil {
		t.Fatalf("hunk location missing: %#v", allowed)
	}
	// A finding about the regular file must not borrow the symlink's line-1 scope.
	borrowed := model.CodeLocation{FilePath: "a/b", LineRange: model.LineRange{Start: 1, End: 1, Count: 1}}
	if codeLocationOverlapsAllowed(borrowed, allowed) {
		t.Fatal("a finding on a/b was authorized by the symlink's location")
	}
	// The symlink's own line 1 is in scope.
	own := model.CodeLocation{FilePath: `a\b`, LineRange: model.LineRange{Start: 1, End: 1, Count: 1}}
	if !codeLocationOverlapsAllowed(own, allowed) {
		t.Fatal("the symlink's own location is out of scope")
	}
	// Model-added noise on the finding side is still tolerated.
	noisy := model.CodeLocation{FilePath: "./a/b", LineRange: model.LineRange{Start: 4, End: 4, Count: 1}}
	if !codeLocationOverlapsAllowed(noisy, allowed) {
		t.Fatal("a ./-prefixed finding path no longer matches its hunk")
	}
}

// An allowed location's path comes from an SCM payload or a synthesized
// "diff --git" line, so it can carry the same noise a model adds. Comparing a
// cleaned finding path against a raw candidate would match nothing and drop every
// finding about that file as out-of-scope — a silent whole-file blackout.
func TestCodeLocationOverlapsAllowedCleansTheCandidateSide(t *testing.T) {
	allowed := []model.CodeLocation{
		{FilePath: "./dir//x.go", LineRange: model.LineRange{Start: 4, End: 5, Count: 2}},
	}
	loc := model.CodeLocation{FilePath: "dir/x.go", LineRange: model.LineRange{Start: 4, End: 4, Count: 1}}

	if !codeLocationOverlapsAllowed(loc, allowed) {
		t.Fatal("a finding was dropped because the allowed path carried payload noise")
	}
	if got := allowedPathFor(allowed, "dir/x.go"); got != "./dir//x.go" {
		t.Fatalf("allowedPathFor = %q, want git's own spelling of the path", got)
	}
	// Whitespace is part of a git filename, so it is never trimmed off the
	// authoritative side: " foo" and "foo" are two different files.
	padded := []model.CodeLocation{
		{FilePath: " foo", LineRange: model.LineRange{Start: 1, End: 1, Count: 1}},
	}
	trimmed := model.CodeLocation{FilePath: "foo", LineRange: model.LineRange{Start: 1, End: 1, Count: 1}}
	if codeLocationOverlapsAllowed(trimmed, padded) {
		t.Fatal(`a finding on "foo" was authorized by the location of " foo"`)
	}
	// The whitespace-named file itself stays reviewable.
	literal := model.CodeLocation{FilePath: " foo", LineRange: model.LineRange{Start: 1, End: 1, Count: 1}}
	if !codeLocationOverlapsAllowed(literal, padded) {
		t.Fatal("the whitespace-named file is out of scope for its own finding")
	}
	// Separators are still not folded on the candidate side: `a\b` and `a/b` are
	// two different files on Unix, and one must not authorize the other.
	separators := []model.CodeLocation{
		{FilePath: `a\b`, LineRange: model.LineRange{Start: 1, End: 1, Count: 1}},
	}
	other := model.CodeLocation{FilePath: "a/b", LineRange: model.LineRange{Start: 1, End: 1, Count: 1}}
	if codeLocationOverlapsAllowed(other, separators) {
		t.Fatal(`a finding on a/b was authorized by the location of a\b`)
	}
}

// A filename of nothing but spaces is legal in git. Trimming it away would leave
// the entry with no allowed location at all, so every finding about that rename
// would be dropped as out-of-scope while the prompt still shows the change.
func TestMetadataOnlySymlinkLocationsKeepWhitespaceOnlyPaths(t *testing.T) {
	changed := []model.ChangedFile{
		{Path: "  ", Status: model.FileRenamed, OldPath: "old/link", Symlink: true, SymlinkTarget: "../target"},
		// An absent path still identifies nothing and is skipped.
		{Path: "", Status: model.FileRenamed, OldPath: "old/other", Symlink: true, SymlinkTarget: "../other"},
	}

	locations := metadataOnlySymlinkLocations(nil, changed)
	if len(locations) != 1 || locations[0].FilePath != "  " {
		t.Fatalf("locations = %#v, want the whitespace-named file only", locations)
	}
	finding := model.CodeLocation{FilePath: "  ", LineRange: model.LineRange{Start: 1, End: 1, Count: 1}}
	if !codeLocationOverlapsAllowed(finding, locations) {
		t.Fatal("a finding on the whitespace-named symlink is out of scope")
	}
}

// A pathname may legally contain a newline, and git then renders the blob as
// several lines. Scoping only line 1 would drop a valid finding anchored on the
// second — the patch has no hunk to fall back on.
func TestMetadataOnlySymlinkLocationsSpanMultiLineTargets(t *testing.T) {
	changed := []model.ChangedFile{
		{Path: "link", Status: model.FileRenamed, OldPath: "old/link", Symlink: true, SymlinkTarget: "dir/one\ntwo\n"},
	}

	locations := metadataOnlySymlinkLocations(nil, changed)
	if len(locations) != 1 {
		t.Fatalf("locations = %#v, want one", locations)
	}
	if got := locations[0].LineRange; got.Start != 1 || got.End != 2 || got.Count != 2 {
		t.Fatalf("line range = %#v, want both of the target's lines", got)
	}
	second := model.CodeLocation{FilePath: "link", LineRange: model.LineRange{Start: 2, End: 2, Count: 1}}
	if !codeLocationOverlapsAllowed(second, locations) {
		t.Fatal("a finding on the target's second line is out of scope")
	}
	// A single-line target still occupies exactly one line.
	single := []model.ChangedFile{{Path: "link", Status: model.FileRenamed, Symlink: true, SymlinkTarget: "../target"}}
	if got := metadataOnlySymlinkLocations(nil, single)[0].LineRange; got.End != 1 || got.Count != 1 {
		t.Fatalf("line range = %#v, want one line", got)
	}
}

// Scope without evidence would invite a finding the prompt cannot ground: with no
// hunk, the target and the old path are the entire change.
func TestMetadataOnlySymlinkLocationsRequireEvidence(t *testing.T) {
	bare := []model.ChangedFile{{Path: "link", Status: model.FileRenamed, Symlink: true}}
	if locs := metadataOnlySymlinkLocations(nil, bare); len(locs) != 0 {
		t.Fatalf("locations = %#v, want none without a target or an old path", locs)
	}
	withOldPath := []model.ChangedFile{{Path: "link", Status: model.FileRenamed, OldPath: "old/link", Symlink: true}}
	if locs := metadataOnlySymlinkLocations(nil, withOldPath); len(locs) != 1 {
		t.Fatalf("locations = %#v, want one once the move is visible", locs)
	}
}
