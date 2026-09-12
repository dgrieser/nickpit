package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/spf13/cobra"
)

func TestSessionLatestAsRawMarkdown(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "older")
	latest := saveSessionReview(t, store, "latest")

	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	if err := a.runSessionTo(context.Background(), sessionOptions{}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "latest") || strings.Contains(out.String(), "older") {
		t.Fatalf("printed wrong session:\n%s", out.String())
	}
	if strings.ContainsRune(out.String(), '\x1b') {
		t.Fatalf("raw Markdown contains ANSI escapes:\n%q", out.String())
	}
	if latest.Result == nil || !strings.Contains(out.String(), "### latest") {
		t.Fatalf("missing raw Markdown title:\n%s", out.String())
	}
}

func TestSessionExplicitAsJSON(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := saveSessionReview(t, store, "chosen")
	saveSessionReview(t, store, "other")

	var out bytes.Buffer
	a := &app{sessionDir: dir, jsonOutput: true}
	if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: sess.ID}, nil, &out); err != nil {
		t.Fatal(err)
	}
	var result model.ReviewResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(result.Findings) != 1 || result.Findings[0].Title != "chosen" {
		t.Fatalf("printed result = %+v", result.Findings)
	}
}

func TestSessionArgumentAndErrors(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := saveSessionReview(t, store, "argument")
	a := &app{sessionDir: dir, outputFormat: "raw"}

	var out bytes.Buffer
	if err := a.runSessionTo(context.Background(), sessionOptions{}, []string{sess.ID}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "argument") {
		t.Fatalf("argument session not printed:\n%s", out.String())
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: sess.ID}, []string{sess.ID}, &out); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("argument/flag conflict error = %v", err)
	}

	empty := &app{sessionDir: t.TempDir()}
	if err := empty.runSessionTo(context.Background(), sessionOptions{}, nil, &out); err == nil || !strings.Contains(err.Error(), "no saved sessions") {
		t.Fatalf("empty store error = %v", err)
	}

	noResult := session.New()
	if err := store.Save(noResult); err != nil {
		t.Fatal(err)
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: noResult.ID}, nil, &out); err == nil || !strings.Contains(err.Error(), "has no saved review") {
		t.Fatalf("missing result error = %v", err)
	}
}

func TestSessionClipboardCopiesUnstyledReview(t *testing.T) {
	// markdown and raw both have to reach the clipboard as Markdown source: the
	// styled variant only ever renders to a terminal, never to the clipboard.
	for _, format := range []string{"markdown", "raw"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			store, err := session.NewStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			sess := saveSessionReview(t, store, "copied")

			var copied []byte
			var out bytes.Buffer
			a := &app{sessionDir: dir, outputFormat: format}
			a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
				copied = data
				return "test-helper", nil
			}
			if err := a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(copied), "### copied") {
				t.Fatalf("clipboard payload missing Markdown review:\n%s", copied)
			}
			if strings.ContainsRune(string(copied), '\x1b') {
				t.Fatalf("clipboard payload contains ANSI escapes:\n%q", copied)
			}
			// The confirmation replaces the review: printing both would defeat the copy.
			if strings.Contains(out.String(), "### copied") {
				t.Fatalf("review printed alongside the copy:\n%s", out.String())
			}
			for _, want := range []string{"Copied review of session " + sess.ID, "via test-helper"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("confirmation %q missing %q", out.String(), want)
				}
			}
		})
	}
}

// failingWriter stands in for a closed pipe or a full disk.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestSessionClipboardSurvivesUnwritableConfirmation(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "copied")

	copied := false
	a := &app{sessionDir: dir, outputFormat: "raw"}
	a.clipboardCopy = func(_ context.Context, _ []byte) (string, error) {
		copied = true
		return "test-helper", nil
	}
	// The review is already on the clipboard, so a confirmation that cannot be
	// written must not report the command as failed.
	if err := a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, failingWriter{}); err != nil {
		t.Fatalf("copy reported as failed after an unwritable confirmation: %v", err)
	}
	if !copied {
		t.Fatal("clipboard was never written")
	}
}

func TestSessionClipboardJSONAndFailure(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "as-json")

	var copied []byte
	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "json"}
	a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
		copied = data
		return "test-helper", nil
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	var result model.ReviewResult
	if err := json.Unmarshal(copied, &result); err != nil {
		t.Fatalf("clipboard payload is not the JSON output: %v\n%s", err, copied)
	}

	a.clipboardCopy = func(_ context.Context, _ []byte) (string, error) {
		return "", errors.New("no clipboard helper found in PATH")
	}
	err = a.runSessionTo(context.Background(), sessionOptions{clipboard: true}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "no clipboard helper found in PATH") {
		t.Fatalf("clipboard failure error = %v", err)
	}
}

func TestSessionRejectsJSONWithConflictingOutput(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "--json", "--output", "raw"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("conflicting output flags error = %v", err)
	}
}

func TestResolveOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		app        app
		outputSet  bool
		wantFormat string
		wantJSON   bool
		wantErr    string
	}{
		{name: "default markdown", app: app{outputFormat: "markdown"}, wantFormat: "markdown"},
		{name: "short raw", app: app{outputFormat: " RAW "}, outputSet: true, wantFormat: "raw"},
		{name: "output json", app: app{outputFormat: "json"}, outputSet: true, wantFormat: "json", wantJSON: true},
		{name: "legacy json", app: app{outputFormat: "markdown", jsonOutput: true}, wantFormat: "json", wantJSON: true},
		{name: "legacy and explicit json", app: app{outputFormat: "json", jsonOutput: true}, outputSet: true, wantFormat: "json", wantJSON: true},
		{name: "legacy conflict", app: app{outputFormat: "raw", jsonOutput: true}, outputSet: true, wantErr: "cannot be combined"},
		{name: "invalid", app: app{outputFormat: "yaml"}, outputSet: true, wantErr: "expected markdown, json, or raw"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.app.resolveOutputFormat(tc.outputSet)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.app.outputFormat != tc.wantFormat || tc.app.jsonOutput != tc.wantJSON {
				t.Fatalf("format/json = %q/%v, want %q/%v", tc.app.outputFormat, tc.app.jsonOutput, tc.wantFormat, tc.wantJSON)
			}
		})
	}
}

func TestCompleteSessionIDs(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := saveSessionReview(t, store, "first")
	second := saveSessionReview(t, store, "second")

	a := &app{sessionDir: dir}
	got, directive := a.completeSessionIDs("")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v", directive)
	}
	if len(got) != 2 || got[0] != second.ID || got[1] != first.ID {
		t.Fatalf("candidates = %v, want newest first [%s %s]", got, second.ID, first.ID)
	}
	got, _ = a.completeSessionIDs(first.ID[:8])
	if len(got) != 1 || got[0] != first.ID {
		t.Fatalf("prefix candidates = %v, want [%s]", got, first.ID)
	}
	got, _ = a.completeSessionIDs("does-not-match")
	if len(got) != 0 {
		t.Fatalf("nonmatching candidates = %v", got)
	}
}

func saveSessionReview(t *testing.T, store *session.Store, title string) *session.Session {
	t.Helper()
	priority := 1
	sess := session.New()
	sess.Result = &model.ReviewResult{
		OverallCorrectness: "patch is incorrect",
		Findings: []model.Finding{{
			Title: title, Body: title + " body", Priority: &priority,
			CodeLocation: model.CodeLocation{FilePath: title + ".go", LineRange: model.LineRange{Start: 1, End: 1}},
		}},
	}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	return sess
}

func saveSessionWarnings(t *testing.T, store *session.Store, warnings ...string) *session.Session {
	t.Helper()
	sess := saveSessionReview(t, store, "warned")
	sess.Result.Warnings = warnings
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestSessionWarningsOnlyPrintsEveryWarning(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionWarnings(t, store,
		"Time budget for step review exhausted after 300s",
		"Verify step failed: context deadline exceeded",
	)

	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"! Warnings: 2 (Budget: 1, Verify: 1)",
		"[Budget] Time budget for step review exhausted after 300s",
		"[Verify] Verify step failed: context deadline exceeded",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("warnings output %q missing %q", out.String(), want)
		}
	}
	// --warnings replaces the review: the findings must not be printed too.
	if strings.Contains(out.String(), "### warned") {
		t.Fatalf("review printed alongside the warnings:\n%s", out.String())
	}
}

func TestSessionWarningsOnlyWithoutWarnings(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "clean")

	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "No warnings." {
		t.Fatalf("output for a warning-free session = %q", out.String())
	}
}

func TestSessionWarningsOnlyAsJSON(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionWarnings(t, store, "Publish failed: 403")

	var out bytes.Buffer
	a := &app{sessionDir: dir, jsonOutput: true}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Warnings []string        `json:"warnings"`
		Findings []model.Finding `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(payload.Warnings) != 1 || payload.Warnings[0] != "Publish failed: 403" {
		t.Fatalf("warnings = %#v", payload.Warnings)
	}
	if len(payload.Findings) != 0 {
		t.Fatalf("findings leaked into the warnings output: %#v", payload.Findings)
	}
}

func TestSessionWarningsToClipboard(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := saveSessionWarnings(t, store, "Verify step failed: context deadline exceeded")

	var copied []byte
	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "markdown"}
	a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
		copied = data
		return "test-helper", nil
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{warnings: true, clipboard: true}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(copied), "[Verify] Verify step failed") {
		t.Fatalf("clipboard payload missing the warnings:\n%s", copied)
	}
	if strings.ContainsRune(string(copied), '\x1b') {
		t.Fatalf("clipboard payload contains ANSI escapes:\n%q", copied)
	}
	if !strings.Contains(out.String(), "Copied warnings of session "+sess.ID) {
		t.Fatalf("confirmation does not name the warnings: %q", out.String())
	}
}

func TestSessionReviewHistoryOutput(t *testing.T) {
	dir := t.TempDir()
	store, _ := session.NewStore(dir)
	sess := saveSessionReview(t, store, "original version")
	next, _ := sess.Result.Clone()
	next.Revision = 1
	next.Findings[0].Title = "corrected version"
	if err := sess.RecordReviewUpdate(next, "Evidence explains correction."); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"raw", "markdown", "json"} {
		for _, copy := range []bool{false, true} {
			var out bytes.Buffer
			a := &app{sessionDir: dir, outputFormat: format}
			var copied string
			a.clipboardCopy = func(_ context.Context, b []byte) (string, error) { copied = string(b); return "test", nil }
			if err := a.runSessionTo(context.Background(), sessionOptions{sessionID: sess.ID, history: true, clipboard: copy}, nil, &out); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if copy {
				text = copied
			}
			if !strings.Contains(text, "original version") || strings.Contains(text, "corrected version") || !strings.Contains(text, "Evidence explains correction") {
				t.Fatalf("wrong history: %s", text)
			}
			if format == "json" {
				var entries []session.ReviewRevision
				if err := json.Unmarshal([]byte(text), &entries); err != nil || len(entries) != 1 {
					t.Fatalf("%s %v", text, err)
				}
			}
		}
	}
	var out bytes.Buffer
	a := &app{sessionDir: dir}
	if err := a.runSessionTo(context.Background(), sessionOptions{history: true, warnings: true}, nil, &out); err == nil {
		t.Fatal("conflicting flags accepted")
	}
	a.outputFormat = "json"
	if err := a.formatReviewHistory(&out, nil); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("empty history: %q %v", out.String(), err)
	}
}

// saveSourcedSession saves a session whose review carries a source, so the
// picker's scopes have something to sort it into.
func saveSourcedSession(t *testing.T, store *session.Store, title string, source session.Source) *session.Session {
	t.Helper()
	sess := saveSessionReview(t, store, title)
	sess.Source = source
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestSessionViewsScopeByRepoAndBranch(t *testing.T) {
	// The worktree is a second working tree of the same clone, so it answers
	// with the same repository directory and folds into the same scope.
	place := sessionPlace{
		here:   git.Location{Root: "/src/nickpit", Repo: "/src/nickpit/.git"},
		repo:   "grp/nickpit",
		branch: "feat/x",
		lookup: func(dir string) git.Location {
			switch dir {
			case "/wt/feature":
				return git.Location{Root: "/wt/feature", Repo: "/src/nickpit/.git"}
			case "/other/project":
				return git.Location{Root: "/other/project", Repo: "/other/project/.git"}
			}
			return git.Location{}
		},
	}
	infos := []session.Info{
		// A review records the directory it ran in, which is usually a
		// subdirectory of the working tree.
		{ID: "here-branch", Source: session.Source{Mode: "local", RepoRoot: "/src/nickpit/cmd", Branch: "feat/x"}},
		{ID: "here-other", Source: session.Source{Mode: "local", RepoRoot: "/src/nickpit", Branch: "main"}},
		{ID: "worktree", Source: session.Source{Mode: "local", RepoRoot: "/wt/feature", HeadRef: "feat/x"}},
		{ID: "remote", Source: session.Source{Mode: "gitlab", Repo: "grp/nickpit", Identifier: 42}},
		{ID: "foreign", Source: session.Source{Mode: "local", RepoRoot: "/other/project", Branch: "feat/x"}},
	}
	views, initial := sessionViews(infos, place, nil)
	if len(views) != 4 {
		t.Fatalf("views = %d, want branch, repository, remote and all", len(views))
	}
	if initial != scopeRepo {
		t.Fatalf("initial view = %d, want the repository scope", initial)
	}
	if got := viewKeys(views[scopeBranch]); !slices.Equal(got, []string{"here-branch", "worktree"}) {
		t.Fatalf("branch scope = %v, want the sessions of feat/x in this repository", got)
	}
	if got := viewKeys(views[scopeRepo]); !slices.Equal(got, []string{"here-branch", "here-other", "worktree", "remote"}) {
		t.Fatalf("repository scope = %v, want every session of this repository", got)
	}
	if len(views[scopeAll].Items) != len(infos) {
		t.Fatalf("all scope = %d rows, want every session", len(views[scopeAll].Items))
	}
	// A scope that turns out empty says which branch or repository it found
	// nothing for; the header is left to the row the cursor is on.
	if !strings.Contains(views[scopeBranch].Empty, "feat/x") || !strings.Contains(views[scopeRepo].Empty, "grp/nickpit") {
		t.Fatalf("empty lines = %q / %q, want the branch and the repository named",
			views[scopeBranch].Empty, views[scopeRepo].Empty)
	}
	// The ★ marks the sessions of the checked-out branch wherever they appear.
	if mark := views[scopeAll].Items[0].Cells[0]; mark != checkedOutMark {
		t.Fatalf("marker = %q, want the current branch marked in the wide scope", mark)
	}
	if mark := views[scopeAll].Items[1].Cells[0]; mark != "" {
		t.Fatalf("marker = %q, want another branch unmarked", mark)
	}
}

func TestSessionViewsOutsideARepositoryAndWithoutRepoSessions(t *testing.T) {
	infos := []session.Info{{ID: "one", Source: session.Source{Mode: "local", RepoRoot: "/src/other"}}}
	views, initial := sessionViews(infos, sessionPlace{}, nil)
	if len(views) != 1 || initial != 0 {
		t.Fatalf("views = %d, initial = %d, want only the full list outside a repository", len(views), initial)
	}
	// In a repository that has nothing saved the prompt still opens on rows
	// rather than on an empty scope.
	views, initial = sessionViews(infos, sessionPlace{
		here: git.Location{Root: "/src/nickpit", Repo: "/src/nickpit/.git"}, branch: "main",
	}, nil)
	if len(views) != 4 || initial != scopeAll {
		t.Fatalf("views = %d, initial = %d, want the full list preselected", len(views), initial)
	}
}

func viewKeys(view pick.View) []string {
	keys := make([]string, 0, len(view.Items))
	for _, item := range view.Items {
		keys = append(keys, item.Key)
	}
	return keys
}

func TestRefIsBranch(t *testing.T) {
	cases := []struct {
		ref, branch string
		want        bool
	}{
		{"feat/x", "feat/x", true},
		{"refs/heads/feat/x", "feat/x", true},
		{"refs/remotes/origin/feat/x", "feat/x", true},
		{"feat/x", "x", false},
		{"main", "feat/x", false},
		{"", "feat/x", false},
		{"feat/x", "", false},
	}
	for _, c := range cases {
		if got := refIsBranch(c.ref, c.branch); got != c.want {
			t.Fatalf("refIsBranch(%q, %q) = %v, want %v", c.ref, c.branch, got, c.want)
		}
	}
}

func TestSessionSourceLabel(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	here := sessionOrigin{where: git.Location{Root: "/src/nickpit", Repo: "/src/nickpit/.git"}, ours: true}
	place := sessionPlace{here: git.Location{Root: "/src/nickpit", Repo: "/src/nickpit/.git"}, repo: "grp/nickpit"}
	cases := []struct {
		name   string
		info   session.Info
		origin sessionOrigin
		want   string
	}{
		{
			name: "gitlab merge request",
			info: session.Info{Source: session.Source{Mode: "gitlab", Identifier: 42}},
			want: "GitLab MR !42",
		},
		{
			name: "github pull request",
			info: session.Info{Source: session.Source{Mode: "github", Identifier: 7}},
			want: "GitHub PR #7",
		},
		{
			name: "branch review names both ends",
			info: session.Info{Source: session.Source{Mode: "local", Submode: "branch", BaseRef: "origin/main", HeadRef: "feat/x"}},
			want: "origin/main..feat/x",
		},
		{
			name: "commit range is abbreviated the way git does",
			info: session.Info{Source: session.Source{Mode: "local", Submode: "commits", BaseRef: sha, HeadRef: sha}},
			want: "01234567..01234567",
		},
		{
			name: "commit range survives in the cached context",
			info: session.Info{
				Source:         session.Source{Mode: "local", Submode: "commits"},
				ContextBaseRef: sha, ContextHeadRef: sha,
			},
			want: "01234567..01234567",
		},
		{
			name:   "recorded branch",
			info:   session.Info{Source: session.Source{Mode: "local", Branch: "feat/x", RepoRoot: "/src/nickpit/cmd"}},
			origin: here, want: "feat/x",
		},
		{
			name: "working-tree review is marked as such",
			info: session.Info{Source: session.Source{Mode: "local", Submode: "uncommitted", Branch: "feat/x"}},
			want: "feat/x (uncommitted)",
		},
		{
			// Saved before the branch was recorded: an uncommitted review's
			// context names the branch as the base of its diff.
			name: "branch recovered from the cached context",
			info: session.Info{
				Source:         session.Source{Mode: "local", RepoRoot: "/src/nickpit/cmd"},
				ContextBaseRef: "feat/x", ContextHeadRef: "uncommitted",
			},
			origin: here, want: "feat/x (uncommitted)",
		},
		{
			// The directory is the repository's own folder, so naming it adds
			// nothing the repository column does not already say.
			name:   "nothing but the repository's own folder",
			info:   session.Info{Source: session.Source{Mode: "local", RepoRoot: "/src/nickpit/cmd"}},
			origin: here, want: "<unknown>",
		},
		{
			// That checkout is gone: its path below the folder this
			// repository's worktrees live in still says which one it was.
			name:   "deleted worktree keeps its path",
			info:   session.Info{Source: session.Source{Mode: "local", RepoRoot: "/wt/nickpit/feat/x/cmd"}},
			origin: sessionOrigin{ours: true, rel: "feat/x/cmd"},
			want:   "<unknown> feat/x/cmd",
		},
		{
			name: "nothing at all",
			info: session.Info{Source: session.Source{Mode: "local"}},
			want: "<unknown>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sessionSourceLabel(place, c.info, c.origin); got != c.want {
				t.Fatalf("sessionSourceLabel = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRepoAnchorsClaimDeletedWorktrees(t *testing.T) {
	here := git.Location{
		Root: "/home/u/worktrees/nickpit/feat/x",
		Repo: "/home/u/src/nickpit/.git",
	}
	place := sessionPlace{here: here, repo: "grp/nickpit", anchors: repoAnchors(here)}
	if !slices.Equal(place.anchors, []string{"/home/u/src/nickpit", "/home/u/worktrees/nickpit"}) {
		t.Fatalf("anchors = %v, want the clone and the folder its worktrees live in", place.anchors)
	}
	// A sibling worktree that has since been removed: git can say nothing about
	// it, but it was a checkout of this repository and its sessions belong here.
	gone := session.Info{Source: session.Source{Mode: "local", RepoRoot: "/home/u/worktrees/nickpit/feat/gone/cmd"}}
	origin := place.origin(gone)
	if !origin.ours || origin.rel != "feat/gone/cmd" {
		t.Fatalf("origin = %+v, want the deleted worktree claimed by its anchor", origin)
	}
	if got := sessionRepoLabel(place, gone, origin); got != "grp/nickpit" {
		t.Fatalf("repo label = %q, want the project of this repository", got)
	}
	// A deleted directory of some other repository is not claimed.
	foreign := session.Info{Source: session.Source{Mode: "local", RepoRoot: "/home/u/worktrees/other/feat/y"}}
	if place.origin(foreign).ours {
		t.Fatal("a directory outside every anchor was claimed")
	}
	// A plain checkout anchors on itself, with no worktree folder above it.
	plain := git.Location{Root: "/home/u/src/nickpit", Repo: "/home/u/src/nickpit/.git"}
	if got := repoAnchors(plain); !slices.Equal(got, []string{"/home/u/src/nickpit"}) {
		t.Fatalf("anchors = %v, want the checkout itself once", got)
	}
}

func TestSessionOutcome(t *testing.T) {
	cases := []struct {
		info session.Info
		want string
	}{
		{session.Info{}, "no review"},
		{session.Info{HasResult: true, Verdict: "patch is correct"}, "Correct, 0 findings"},
		{session.Info{HasResult: true, Verdict: "patch is incorrect", Findings: 1}, "Incorrect, 1 finding"},
		{session.Info{HasResult: true, Verdict: "patch is incorrect", Findings: 4}, "Incorrect, 4 findings"},
		// A workflow without a verdict step records none.
		{session.Info{HasResult: true, Findings: 2}, "No verdict, 2 findings"},
		// The verdict is free text a model wrote: one that is neither is shown
		// as it stands instead of being collapsed into a verdict it never gave.
		{session.Info{HasResult: true, Verdict: "error"}, "Error, 0 findings"},
	}
	for _, c := range cases {
		if got := sessionOutcome(c.info); got != c.want {
			t.Fatalf("sessionOutcome(%+v) = %q, want %q", c.info, got, c.want)
		}
	}
	// The verdict alone decides the colour, and the verdicts differ: the green
	// and red of the review output badges, a caveat for a verdict that is
	// neither, grey for a review that recorded none.
	styles := map[string]string{
		"patch is correct":              pick.StyleFresh,
		"patch is correct, 12 findings": pick.StyleFresh,
		"patch is incorrect":            pick.StyleError,
		"error: the reviewer gave up":   pick.StyleCaveat,
		"":                              pick.StyleAge,
	}
	for verdict, want := range styles {
		if got := verdictStyle(session.Info{HasResult: true, Verdict: verdict}); got != want {
			t.Fatalf("verdictStyle(%q) = %q, want %q", verdict, got, want)
		}
	}
	if got := verdictStyle(session.Info{}); got != pick.StyleAge {
		t.Fatalf("verdict style without a review = %q, want grey", got)
	}
	// The finding count never enters the choice.
	many := verdictStyle(session.Info{HasResult: true, Verdict: "patch is correct", Findings: 9})
	if many != verdictStyle(session.Info{HasResult: true, Verdict: "patch is correct"}) {
		t.Fatalf("verdict style changed with the finding count: %q", many)
	}
}

func TestSessionKindDecidesTheSourceColour(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		name string
		info session.Info
		want sessionKind
	}{
		{"gitlab", session.Info{Source: session.Source{Mode: "gitlab", Identifier: 42}}, kindGitLabRequest},
		{"github", session.Info{Source: session.Source{Mode: "github", Identifier: 7}}, kindGitHubRequest},
		{"branch", session.Info{Source: session.Source{Mode: "local", Submode: "branch", BaseRef: "origin/main", HeadRef: "feat/x"}}, kindBranchReview},
		{"commits", session.Info{Source: session.Source{Mode: "local", Submode: "commits", BaseRef: sha, HeadRef: sha}}, kindCommitReview},
		{"uncommitted", session.Info{Source: session.Source{Mode: "local", Submode: "uncommitted", Branch: "feat/x"}}, kindWorkingTree},
		// Saved before the submode was recorded: two SHAs are a range, a
		// context head that is not a commit was the working tree, and a lone
		// branch is a branch review.
		{"legacy range", session.Info{Source: session.Source{Mode: "local"}, ContextBaseRef: sha, ContextHeadRef: sha}, kindCommitReview},
		{"legacy working tree", session.Info{Source: session.Source{Mode: "local"}, ContextBaseRef: "feat/x", ContextHeadRef: "uncommitted"}, kindWorkingTree},
		{"legacy branch", session.Info{Source: session.Source{Mode: "local", Branch: "feat/x"}}, kindBranchReview},
		{"nothing recorded", session.Info{Source: session.Source{Mode: "local", RepoRoot: "/src/x"}}, kindUnknownReview},
	}
	seen := map[string]sessionKind{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sessionKindOf(c.info); got != c.want {
				t.Fatalf("sessionKindOf = %v, want %v", got, c.want)
			}
			style := sourceStyle(c.info)
			if style == "" {
				t.Fatalf("kind %v has no colour", c.want)
			}
			// Every kind has to be told apart by colour alone; two ways of
			// recording the same kind share one, which is the point.
			if other, clash := seen[style]; clash && other != c.want {
				t.Fatalf("kinds %v and %v share colour %q", c.want, other, style)
			}
			seen[style] = c.want
		})
	}
}

func TestSessionRepoLabel(t *testing.T) {
	local := session.Info{Source: session.Source{Mode: "local", RepoRoot: "/wt/feature/cmd"}}
	clone := sessionOrigin{where: git.Location{Root: "/wt/feature", Repo: "/src/nickpit/.git"}}
	if got := sessionRepoLabel(sessionPlace{}, local, clone); got != "nickpit" {
		t.Fatalf("repo label = %q, want the clone's own directory", got)
	}
	// Standing in that same repository, the project of its remote names it —
	// which is what a session saved before the project was recorded lacks.
	here := sessionPlace{here: git.Location{Root: "/src/nickpit", Repo: "/src/nickpit/.git"}, repo: "grp/nickpit"}
	clone.ours = true
	if got := sessionRepoLabel(here, local, clone); got != "grp/nickpit" {
		t.Fatalf("repo label = %q, want the project of the current repository", got)
	}
	if got := sessionRepoLabel(sessionPlace{}, local, sessionOrigin{}); got != "cmd" {
		t.Fatalf("repo label of a checkout that is gone = %q", got)
	}
	recorded := session.Info{Source: session.Source{Mode: "local", Repo: "grp/nickpit"}}
	if got := sessionRepoLabel(sessionPlace{}, recorded, sessionOrigin{}); got != "grp/nickpit" {
		t.Fatalf("repo label = %q, want the recorded project", got)
	}
}

func TestSessionWithoutArgumentPrompts(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	older := saveSourcedSession(t, store, "older", session.Source{Mode: "local", RepoRoot: dir, HeadRef: "main"})
	saveSessionReview(t, store, "latest")

	var seen, actions pick.Options
	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	a.selectViewFn = func(opts pick.Options) (int, int, error) {
		seen = opts
		// Pick the older session out of the full scope — the row the
		// latest-session fallback would never have chosen.
		for i, item := range opts.Views[len(opts.Views)-1].Items {
			if item.Key == older.ID {
				return len(opts.Views) - 1, i, nil
			}
		}
		t.Fatalf("session %s not offered", older.ID)
		return 0, 0, nil
	}
	// The session is picked first, then what to do with it.
	a.selectFn = func(opts pick.Options) (int, error) {
		actions = opts
		return int(sessionActionPrint), nil
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if len(actions.Items) != 3 || actions.Items[0].Cells[0] != "Print" ||
		actions.Items[1].Cells[0] != "Copy" || actions.Items[2].Cells[0] != "Chat" {
		t.Fatalf("action prompt = %+v", actions.Items)
	}
	if !actions.DismissOnBackspace {
		t.Fatal("the action prompt must let backspace go back")
	}
	if !strings.Contains(out.String(), "older") || strings.Contains(out.String(), "latest") {
		t.Fatalf("printed the wrong session:\n%s", out.String())
	}
	if len(seen.Views) == 0 {
		t.Fatal("the picker was not offered any scope")
	}
	// An id on the command line still skips the prompt entirely.
	a.selectViewFn = func(pick.Options) (int, int, error) {
		t.Fatal("named session must not prompt")
		return 0, 0, nil
	}
	a.selectFn = func(pick.Options) (int, error) {
		t.Fatal("named session must not ask for an action")
		return 0, nil
	}
	out.Reset()
	if err := a.runSessionTo(context.Background(), sessionOptions{}, []string{older.ID}, &out); err != nil {
		t.Fatal(err)
	}
}

func TestSessionPromptAbortAndEmptyStore(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saveSessionReview(t, store, "one")
	a := &app{sessionDir: dir, outputFormat: "raw"}
	a.selectViewFn = func(pick.Options) (int, int, error) { return -1, -1, pick.ErrAborted }
	a.selectFn = func(pick.Options) (int, error) {
		t.Fatal("an abandoned list must not ask for an action")
		return 0, nil
	}
	var out bytes.Buffer
	err = a.runSessionTo(context.Background(), sessionOptions{}, nil, &out)
	if !errors.Is(err, pick.ErrAborted) {
		t.Fatalf("err = %v, want the abort to travel up as pick.ErrAborted", err)
	}
	// A store holding only sessions without a review has nothing this command
	// could print, and says so instead of prompting.
	empty := t.TempDir()
	emptyStore, err := session.NewStore(empty)
	if err != nil {
		t.Fatal(err)
	}
	if err := emptyStore.Save(session.New()); err != nil {
		t.Fatal(err)
	}
	b := &app{sessionDir: empty, outputFormat: "raw", selectViewFn: func(pick.Options) (int, int, error) {
		t.Fatal("a store without a printable session must not prompt")
		return 0, 0, nil
	}, selectFn: func(pick.Options) (int, error) {
		t.Fatal("a store without a printable session must not prompt")
		return 0, nil
	}}
	if err := b.runSessionTo(context.Background(), sessionOptions{}, nil, &out); err == nil ||
		!strings.Contains(err.Error(), "no saved session holds a review") {
		t.Fatalf("err = %v, want the empty-store error", err)
	}
}

func TestDescribeLocalSourceRecordsProjectAndBranch(t *testing.T) {
	dir := newTestRepoWithOrigin(t)
	runGitTestCommand(t, dir, "checkout", "-q", "-b", "feat/x")
	sub := filepath.Join(dir, "cmd", "nickpit")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// A working-tree review records neither refs nor a project; without this
	// its session would be indistinguishable from every other local one.
	got := describeLocalSource(ctx, session.Source{Mode: "local", RepoRoot: sub})
	if got.Repo != "grp/proj" {
		t.Fatalf("repo = %q, want the project of the origin remote", got.Repo)
	}
	if got.Branch != "feat/x" {
		t.Fatalf("branch = %q, want the checked-out branch", got.Branch)
	}

	// What the review already knew is never overwritten, and a remote review's
	// source is left alone entirely.
	kept := describeLocalSource(ctx, session.Source{
		Mode: "local", RepoRoot: sub, Repo: "grp/other", Branch: "main",
	})
	if kept.Repo != "grp/other" || kept.Branch != "main" {
		t.Fatalf("source = %+v, want the recorded values kept", kept)
	}
	remote := session.Source{Mode: "gitlab", Repo: "grp/proj", Identifier: 42}
	if got := describeLocalSource(ctx, remote); got != remote {
		t.Fatalf("source = %+v, want a remote source untouched", got)
	}
	// A directory that is not a checkout answers with neither, without failing.
	outside := describeLocalSource(ctx, session.Source{Mode: "local", RepoRoot: t.TempDir()})
	if outside.Repo != "" || outside.Branch != "" {
		t.Fatalf("source = %+v, want nothing recorded outside a repository", outside)
	}
}

func TestFitSourceLabelShortensByItsParts(t *testing.T) {
	long := "feat/rules-triggers-conditions-actions-buttons"
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{"short label is untouched", "feat/x", "feat/x"},
		{"a long branch loses its tail", long, "feat/rules-triggers-conditions-actions-…"},
		{
			// The qualifier says what kind of review this was, so it survives
			// the cut and the branch pays for it — out of the same cap, so the
			// whole cell still fits in it.
			"the uncommitted note is never cut",
			long + " (uncommitted)",
			"feat/rules-triggers-condi… (uncommitted)",
		},
		{
			// One end that fits keeps its full width; the other takes the rest.
			"a short base leaves the budget to the head",
			"main.." + long,
			"main..feat/rules-triggers-conditions-ac…",
		},
		{
			"two long ends level down together",
			"release/2026-05-candidate-one..release/2026-06-candidate-two",
			"release/2026-05-ca…..release/2026-06-ca…",
		},
		{"a request is far inside the cap", "GitLab MR !702", "GitLab MR !702"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fitSourceLabel(c.label)
			if got != c.want {
				t.Fatalf("fitSourceLabel(%q) = %q, want %q", c.label, got, c.want)
			}
			// The cap covers the whole cell, qualifier included: a column wider
			// than its widest cell strands the next one behind the padding.
			if width := pick.DisplayWidth(got); width > maxSourceWidth {
				t.Fatalf("width = %d, want at most %d", width, maxSourceWidth)
			}
			if note := " (uncommitted)"; strings.HasSuffix(c.label, note) && !strings.HasSuffix(got, note) {
				t.Fatalf("got = %q, want the qualifier kept whole", got)
			}
		})
	}
}

func TestShareWidthSplitsARangeFairly(t *testing.T) {
	if base, head := shareWidth(4, 60, 38); base != 4 || head != 34 {
		t.Fatalf("share = %d/%d, want the short end kept whole", base, head)
	}
	if base, head := shareWidth(60, 4, 38); base != 34 || head != 4 {
		t.Fatalf("share = %d/%d, want the short end kept whole", base, head)
	}
	if base, head := shareWidth(60, 60, 39); base != 19 || head != 20 {
		t.Fatalf("share = %d/%d, want an even split with the odd cell to the head", base, head)
	}
	if base, head := shareWidth(10, 10, 25); base != 10 || head != 10 {
		t.Fatalf("share = %d/%d, want both ends untouched inside the budget", base, head)
	}
	if base, head := shareWidth(10, 10, 0); base != 0 || head != 0 {
		t.Fatalf("share = %d/%d, want nothing from an empty budget", base, head)
	}
}

func TestFitRepoLabelFoldsThePathBeforeTheName(t *testing.T) {
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{"inside the cap", "dgrieser/nickpit", "dgrieser/nickpit"},
		{"middle groups go first", "asylum/services/archiefmeester", "asylum/…/archiefmeester"},
		{"then the leading group", "asylum-platform-team/services/archiefmeester", "…/archiefmeester"},
		{
			// Nothing but the name is left, so the cut moves into it — both of
			// its ends say which project this is.
			"then the name itself",
			"asylum/services/archiefmeester-document-service",
			"…/archiefmees…nt-service",
		},
		{"a name with no path at all", "archiefmeester-document-service", "archiefmeest…ent-service"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fitRepoLabel(c.label)
			if got != c.want {
				t.Fatalf("fitRepoLabel(%q) = %q, want %q", c.label, got, c.want)
			}
			if width := pick.DisplayWidth(got); width > maxRepoWidth {
				t.Fatalf("width = %d, want at most %d", width, maxRepoWidth)
			}
		})
	}
}

func TestSessionRowsAreTypedAtByWhatTheyShow(t *testing.T) {
	place := sessionPlace{
		here: git.Location{Root: "/src/nickpit", Repo: "/src/nickpit/.git"},
		repo: "grp/nickpit",
	}
	infos := []session.Info{{
		ID:      "3f1a9c22-0000-4000-8000-000000000001",
		Source:  session.Source{Mode: "local", Submode: "branch", Repo: "asylum/services/archiefmeester", RepoRoot: "/wt/nickpit/feat/x/cmd", Branch: "feat/x"},
		Verdict: "patch is incorrect", HasResult: true, Findings: 3, Headline: "correct use of the wrong lock",
	}}
	item := sessionItems(infos, place)[0]
	// The project and the directory are matched in full, whatever the columns
	// had to fold away; so are the id, the verdict and the finding count.
	for _, term := range []string{
		"3f1a9c22-0000-4000-8000-000000000001", "3f1a9c22",
		"asylum/services/archiefmeester", "archiefmeester",
		"/wt/nickpit/feat/x/cmd", "feat/x", "incorrect", "3",
	} {
		if !itemMatches(item, term) {
			t.Fatalf("item does not answer to %q: %+v", term, item.MatchFields)
		}
	}
	// The finding text is not part of it: "incorrect" has to mean the verdict.
	// Nor is the word beside the count, which every row carries.
	for _, term := range []string{"wrong lock", "finding", "findings"} {
		if itemMatches(item, term) {
			t.Fatalf("item answers to %q: %+v", term, item.MatchFields)
		}
	}
	// A term too short to be an id searches everything but the id: "3f1" is in
	// this session's, and must still find nothing.
	for _, term := range []string{"3f1", "f1a", "00"} {
		if itemMatches(item, term) {
			t.Fatalf("short term %q reached the session id", term)
		}
	}
	// A session that holds no review is typed at by what its column says.
	none := sessionItems([]session.Info{{ID: "x", Source: session.Source{Mode: "local"}}}, place)[0]
	if !itemMatches(none, "no review") || none.Cells[4] != "no review" {
		t.Fatalf("match = %+v, cell = %q", none.MatchFields, none.Cells[4])
	}
}

// itemMatches applies one filter term the way the picker does.
func itemMatches(item pick.Item, term string) bool {
	for _, field := range item.MatchFields {
		if field.MinTerm > 0 && len([]rune(term)) < field.MinTerm {
			continue
		}
		if strings.Contains(strings.ToLower(field.Text), strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func TestSessionActionPromptGoesBackAndCopies(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := saveSourcedSession(t, store, "first", session.Source{Mode: "local", RepoRoot: dir, Branch: "main"})
	second := saveSourcedSession(t, store, "second", session.Source{Mode: "local", RepoRoot: dir, Branch: "main"})

	var copied []byte
	var out bytes.Buffer
	a := &app{sessionDir: dir, outputFormat: "raw"}
	a.clipboardCopy = func(_ context.Context, data []byte) (string, error) {
		copied = data
		return "test-helper", nil
	}
	// The list is shown twice: the first choice is taken back out of the action
	// prompt, and the second one is copied.
	lists, wanted := 0, []string{first.ID, second.ID}
	a.selectViewFn = func(opts pick.Options) (int, int, error) {
		id := wanted[lists]
		lists++
		for view, scope := range opts.Views {
			for row, item := range scope.Items {
				if item.Key == id {
					return view, row, nil
				}
			}
		}
		t.Fatalf("session %s not offered", id)
		return 0, 0, nil
	}
	prompts := 0
	a.selectFn = func(pick.Options) (int, error) {
		prompts++
		if prompts == 1 {
			return -1, pick.ErrAborted
		}
		return int(sessionActionCopy), nil
	}
	if err := a.runSessionTo(context.Background(), sessionOptions{}, nil, &out); err != nil {
		t.Fatal(err)
	}
	if lists != 2 || prompts != 2 {
		t.Fatalf("lists = %d, prompts = %d, want the list shown again after going back", lists, prompts)
	}
	if !strings.Contains(string(copied), "### second") {
		t.Fatalf("clipboard payload = %q, want the second session's review", copied)
	}
	if strings.Contains(out.String(), "### second") {
		t.Fatalf("the review was printed as well as copied:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Copied review of session "+second.ID) {
		t.Fatalf("confirmation = %q", out.String())
	}
}

func TestPickSessionActionOffersChat(t *testing.T) {
	var seen pick.Options
	a := &app{selectFn: func(opts pick.Options) (int, error) {
		seen = opts
		return int(sessionActionChat), nil
	}}
	info := session.Info{ID: "3f1a9c22", Source: session.Source{Mode: "local", RepoRoot: "/src/nickpit"}}
	action, err := a.pickSessionAction(sessionPlace{}, sessionChoice{info: info})
	if err != nil {
		t.Fatal(err)
	}
	if action != sessionActionChat {
		t.Fatalf("action = %v, want chat", action)
	}
	// The prompt names the session it is about, in the header the list uses.
	if len(seen.Items) == 0 || len(seen.Items[0].Details) != 2 ||
		seen.Items[0].Details[0].Text != info.ID {
		t.Fatalf("prompt header = %+v", seen.Items)
	}
	// Esc travels up as an abort, which the caller reads as "back".
	b := &app{selectFn: func(pick.Options) (int, error) { return -1, pick.ErrAborted }}
	if _, err := b.pickSessionAction(sessionPlace{}, sessionChoice{info: info}); !errors.Is(err, pick.ErrAborted) {
		t.Fatalf("err = %v, want pick.ErrAborted", err)
	}
}

func TestClipboardConfirmationIsAnAside(t *testing.T) {
	line := "Copied review of session abc to the clipboard (12 bytes) via pbcopy."
	styled := noteText(line, true)
	if styled != "\x1b["+selectionStyle+"m"+line+"\x1b[0m" {
		t.Fatalf("styled = %q, want the whole line in the aside's italic light grey", styled)
	}
	if got := noteText(line, false); got != line {
		t.Fatalf("unstyled = %q, want the line untouched", got)
	}
	// A buffer, a pipe and a redirect are not a terminal, so what a script
	// reads back stays plain — which is what the existing tests assert.
	if useColor(&bytes.Buffer{}) {
		t.Fatal("a buffer must not be coloured")
	}
	r, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = wr.Close() }()
	if useColor(wr) {
		t.Fatal("a pipe must not be coloured")
	}
}
