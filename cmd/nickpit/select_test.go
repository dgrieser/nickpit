package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
)

// interactiveApp is an app that counts as interactive: the select seam stands
// in for the terminal and answers with the row at index.
func interactiveApp(index int, captured *pick.Options) *app {
	return &app{selectFn: func(opts pick.Options) (int, error) {
		if captured != nil {
			*captured = opts
		}
		return index, nil
	}}
}

func TestResolveRequestTargetFlagPolicy(t *testing.T) {
	const mrURL = "https://gitlab.example.com/grp/proj/-/merge_requests/42"
	cases := []struct {
		name        string
		app         *app
		sel         requestSelectors
		wantErr     string
		wantTarget  requestTarget
		wantPicking bool
	}{
		{
			name:    "url with id",
			app:     &app{},
			sel:     requestSelectors{rawURL: mrURL, id: 42},
			wantErr: "--url can not be combined with --id",
		},
		{
			name:    "url with repo",
			app:     &app{},
			sel:     requestSelectors{rawURL: mrURL, repo: "grp/proj"},
			wantErr: "--url can not be combined with --repo",
		},
		{
			name:    "url with select",
			app:     interactiveApp(0, nil),
			sel:     requestSelectors{rawURL: mrURL, pick: true},
			wantErr: "--url can not be combined with --select",
		},
		{
			name:    "select with id",
			app:     interactiveApp(0, nil),
			sel:     requestSelectors{repo: "grp/proj", id: 42, pick: true},
			wantErr: "--select can not be combined with --id",
		},
		{
			// Without a terminal a picker could only block on input nobody can
			// send, so --select fails fast instead.
			name:    "select without a terminal",
			app:     &app{},
			sel:     requestSelectors{repo: "grp/proj", pick: true},
			wantErr: "--select needs a terminal",
		},
		{
			name:    "missing id without a terminal",
			app:     &app{},
			sel:     requestSelectors{repo: "grp/proj"},
			wantErr: "--id must be a positive integer",
		},
		{
			// Only a missing id defers to the picker; a negative one is never a
			// request and must not reach the API as "-1".
			name:    "negative id without a terminal",
			app:     &app{},
			sel:     requestSelectors{repo: "grp/proj", id: -1},
			wantErr: "--id must be a positive integer",
		},
		{
			name:    "negative id in a terminal",
			app:     interactiveApp(0, nil),
			sel:     requestSelectors{repo: "grp/proj", id: -1},
			wantErr: "--id must be a positive integer",
		},
		{
			// Flag presence, not the value: --id=0 was supplied, so it conflicts
			// with --url and does not read as "no id given".
			name:    "url with an explicit zero id",
			app:     &app{},
			sel:     requestSelectors{rawURL: mrURL, changed: changedFlags("url", "id")},
			wantErr: "--url can not be combined with --id",
		},
		{
			name:    "url with an explicit empty repo",
			app:     &app{},
			sel:     requestSelectors{rawURL: mrURL, changed: changedFlags("url", "repo")},
			wantErr: "--url can not be combined with --repo",
		},
		{
			name:    "explicit empty url with an id",
			app:     &app{},
			sel:     requestSelectors{id: 42, changed: changedFlags("url", "id")},
			wantErr: "--url can not be combined with --id",
		},
		{
			name:    "explicit zero id in a terminal",
			app:     interactiveApp(0, nil),
			sel:     requestSelectors{repo: "grp/proj", changed: changedFlags("id")},
			wantErr: "--id must be a positive integer",
		},
		{
			name:    "select with an explicit zero id",
			app:     interactiveApp(0, nil),
			sel:     requestSelectors{repo: "grp/proj", pick: true, changed: changedFlags("id")},
			wantErr: "--select can not be combined with --id",
		},
		{
			name:       "url wins alone",
			app:        &app{},
			sel:        requestSelectors{rawURL: mrURL},
			wantTarget: requestTarget{Repo: "grp/proj", ID: 42, BaseURL: "https://gitlab.example.com"},
		},
		{
			name:       "explicit repo and id",
			app:        &app{},
			sel:        requestSelectors{repo: "grp/proj", id: 7},
			wantTarget: requestTarget{Repo: "grp/proj", ID: 7},
		},
		{
			// An id of 0 with a terminal is the implicit picker: the command
			// resolves the project now and the identifier from the list later.
			name:        "missing id in a terminal defers to the picker",
			app:         interactiveApp(0, nil),
			sel:         requestSelectors{repo: "grp/proj"},
			wantTarget:  requestTarget{Repo: "grp/proj"},
			wantPicking: true,
		},
		{
			name:        "explicit select defers to the picker",
			app:         interactiveApp(0, nil),
			sel:         requestSelectors{repo: "grp/proj", pick: true},
			wantTarget:  requestTarget{Repo: "grp/proj"},
			wantPicking: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target, err := tc.app.resolveRequestTarget(tc.sel, parseGitLabMRURL, "", "merge request")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if target != tc.wantTarget {
				t.Fatalf("target = %+v, want %+v", target, tc.wantTarget)
			}
			if picking := target.ID == 0; picking != tc.wantPicking {
				t.Fatalf("picking = %v, want %v", picking, tc.wantPicking)
			}
		})
	}
}

// changedFlags fakes cobra's Flags().Changed for the named flags.
func changedFlags(names ...string) func(string) bool {
	return func(flag string) bool { return slices.Contains(names, flag) }
}

func TestResolveRequestTargetPrefixesErrors(t *testing.T) {
	_, err := (&app{}).resolveRequestTarget(requestSelectors{repo: "g/p"}, parseGitLabMRURL, "chat", "merge request")
	if err == nil || !strings.HasPrefix(err.Error(), "chat: ") {
		t.Fatalf("err = %v, want it prefixed with the command name", err)
	}
}

// Only a checkout can supply the project, which is what makes omitting --repo
// work at all.
func TestResolveRequestTargetInfersRepoFromRemote(t *testing.T) {
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "remote", "add", "origin", "git@gitlab.example.com:grp/proj.git")
	t.Chdir(dir)

	target, err := interactiveApp(0, nil).resolveRequestTarget(requestSelectors{pick: true}, parseGitLabMRURL, "", "merge request")
	if err != nil {
		t.Fatal(err)
	}
	if target.Repo != "grp/proj" {
		t.Fatalf("repo = %q, want it inferred from the origin remote", target.Repo)
	}
}

func TestResolveRequestTargetWithoutRemote(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := (&app{}).resolveRequestTarget(requestSelectors{id: 4}, parseGitLabMRURL, "", "merge request")
	if err == nil || !strings.Contains(err.Error(), "--repo is required") {
		t.Fatalf("err = %v, want it to ask for --repo", err)
	}
}

func openRequests() []model.OpenRequest {
	now := time.Now()
	return []model.OpenRequest{
		{Identifier: 142, Title: "feat(review): cluster merge", Author: "alice",
			SourceBranch: "feat/cluster", TargetBranch: "main", UpdatedAt: now.Add(-48 * time.Hour)},
		{Identifier: 139, Title: "fix(llm): retry on 429", Author: "bob",
			SourceBranch: "fix/retry", TargetBranch: "main", Draft: true, UpdatedAt: now.Add(-96 * time.Hour)},
	}
}

func TestPickOpenRequestBuildsRowsAndReturnsChoice(t *testing.T) {
	var captured pick.Options
	a := interactiveApp(1, &captured)
	id, err := a.pickOpenRequest(context.Background(), "grp/proj", openRequestList{
		noun:   "merge request",
		marker: "!",
		list:   func(context.Context) ([]model.OpenRequest, error) { return openRequests(), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != 139 {
		t.Fatalf("id = %d, want the identifier of the chosen row", id)
	}
	if !strings.Contains(captured.Title, "grp/proj") || !strings.Contains(captured.Title, "merge request") {
		t.Fatalf("title = %q, want the project and the platform's noun", captured.Title)
	}
	if len(captured.Items) != 2 {
		t.Fatalf("items = %+v, want one per request", captured.Items)
	}
	first := captured.Items[0]
	if first.Cells[1] != "!142" {
		t.Fatalf("identifier cell = %q, want the platform marker", first.Cells[1])
	}
	if !strings.Contains(strings.Join(first.Cells, " "), "feat(review): cluster merge") {
		t.Fatalf("row = %q, want the title", first.Cells)
	}
	if !strings.Contains(strings.Join(first.Cells, " "), "alice") {
		t.Fatalf("row = %q, want the author", first.Cells)
	}
	// The branches are not shown but must stay findable by typing.
	if !strings.Contains(first.Match, "feat/cluster") || !strings.Contains(first.Match, "main") {
		t.Fatalf("match text = %q, want both branches", first.Match)
	}
	// One request is a draft, so the column exists and marks only that row.
	if captured.Items[0].Cells[2] != "" || captured.Items[1].Cells[2] != "draft" {
		t.Fatalf("draft column = %q / %q", captured.Items[0].Cells[2], captured.Items[1].Cells[2])
	}
	// The colours follow the columns, draft column included.
	want := []string{pick.StyleMark, pick.StyleIdentifier, pick.StyleCaveat, pick.StyleText,
		pick.StyleAuthor, pick.StyleAge}
	if !slices.Equal(captured.CellStyles, want) {
		t.Fatalf("cell styles = %q, want %q", captured.CellStyles, want)
	}
	// The title is a commit-style message; the draft column shifts it right.
	wantKinds := []pick.ColumnKind{pick.KindPlain, pick.KindPlain, pick.KindPlain, pick.KindMessage}
	if !slices.Equal(captured.ColumnKinds, wantKinds) {
		t.Fatalf("column kinds = %v, want %v", captured.ColumnKinds, wantKinds)
	}
}

// A list without drafts carries no blank column.
func TestPickOpenRequestOmitsDraftColumnWhenUnused(t *testing.T) {
	requests := openRequests()
	requests[1].Draft = false
	var captured pick.Options
	if _, err := interactiveApp(0, &captured).pickOpenRequest(context.Background(), "grp/proj", openRequestList{
		noun:   "merge request",
		marker: "!",
		list:   func(context.Context) ([]model.OpenRequest, error) { return requests, nil },
	}); err != nil {
		t.Fatal(err)
	}
	if got := captured.Items[0].Cells[2]; got != "feat(review): cluster merge" {
		t.Fatalf("third cell = %q, want the title with no draft column in between", got)
	}
	want := []string{pick.StyleMark, pick.StyleIdentifier, pick.StyleText, pick.StyleAuthor, pick.StyleAge}
	if !slices.Equal(captured.CellStyles, want) {
		t.Fatalf("cell styles = %q, want %q", captured.CellStyles, want)
	}
	wantKinds := []pick.ColumnKind{pick.KindPlain, pick.KindPlain, pick.KindMessage}
	if !slices.Equal(captured.ColumnKinds, wantKinds) {
		t.Fatalf("column kinds = %v, want %v", captured.ColumnKinds, wantKinds)
	}
}

// In a checkout the request of the current branch is nearly always the one
// meant, so it is marked and preselected.
func TestPickOpenRequestPreselectsCurrentBranch(t *testing.T) {
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "checkout", "-q", "-b", "fix/retry")
	t.Chdir(dir)

	var captured pick.Options
	if _, err := interactiveApp(0, &captured).pickOpenRequest(context.Background(), "grp/proj", openRequestList{
		noun:   "merge request",
		marker: "!",
		list:   func(context.Context) ([]model.OpenRequest, error) { return openRequests(), nil },
	}); err != nil {
		t.Fatal(err)
	}
	if captured.Initial != 1 {
		t.Fatalf("initial row = %d, want the request of the checked-out branch", captured.Initial)
	}
	// The same star the branch picker uses, in the same column.
	if captured.Items[1].Cells[0] != checkedOutMark || captured.Items[0].Cells[0] != "" {
		t.Fatalf("markers = %q / %q, want only the current branch starred",
			captured.Items[0].Cells[0], captured.Items[1].Cells[0])
	}
}

// Several open requests can share a source branch. The newest — the first row —
// is the one to offer, so a later match must not overwrite it, not even when the
// first match is at index 0.
func TestPickOpenRequestPreselectsTheFirstBranchMatch(t *testing.T) {
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "checkout", "-q", "-b", "feat/cluster")
	t.Chdir(dir)

	requests := openRequests()
	// Both rows now belong to the checked-out branch; the newest is row 0.
	requests[1].SourceBranch = "feat/cluster"

	var captured pick.Options
	if _, err := interactiveApp(0, &captured).pickOpenRequest(context.Background(), "grp/proj", openRequestList{
		noun:   "merge request",
		marker: "!",
		list:   func(context.Context) ([]model.OpenRequest, error) { return requests, nil },
	}); err != nil {
		t.Fatal(err)
	}
	if captured.Initial != 0 {
		t.Fatalf("initial row = %d, want the newest matching request", captured.Initial)
	}
	if captured.Items[0].Cells[0] != checkedOutMark || captured.Items[1].Cells[0] != checkedOutMark {
		t.Fatalf("markers = %q / %q, want both matches starred",
			captured.Items[0].Cells[0], captured.Items[1].Cells[0])
	}
}

func TestPickOpenRequestWithoutOpenRequests(t *testing.T) {
	_, err := interactiveApp(0, nil).pickOpenRequest(context.Background(), "grp/proj", openRequestList{
		noun:   "merge request",
		marker: "!",
		list:   func(context.Context) ([]model.OpenRequest, error) { return nil, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "no open merge request in grp/proj") {
		t.Fatalf("err = %v, want it to report an empty list", err)
	}
	if !strings.Contains(err.Error(), "--id") {
		t.Fatalf("err = %v, want it to point at --id for merged or closed requests", err)
	}
}

func TestPickOpenRequestReportsListFailure(t *testing.T) {
	want := errors.New("403 Forbidden")
	_, err := interactiveApp(0, nil).pickOpenRequest(context.Background(), "grp/proj", openRequestList{
		noun:   "merge request",
		marker: "!",
		list:   func(context.Context) ([]model.OpenRequest, error) { return nil, want },
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want it to wrap %v", err, want)
	}
}

// A dismissed picker is a user abort: it must travel out unchanged so main can
// exit quietly instead of printing a failure.
func TestPickOpenRequestPropagatesAbort(t *testing.T) {
	a := &app{selectFn: func(pick.Options) (int, error) { return -1, pick.ErrAborted }}
	_, err := a.pickOpenRequest(context.Background(), "grp/proj", openRequestList{
		noun:   "merge request",
		marker: "!",
		list:   func(context.Context) ([]model.OpenRequest, error) { return openRequests(), nil },
	})
	if !errors.Is(err, pick.ErrAborted) {
		t.Fatalf("err = %v, want pick.ErrAborted", err)
	}
	if code, quiet := quietExitCode(context.Background(), err); !quiet || code != 130 {
		t.Fatalf("exit = %d, quiet = %v; want the interrupt code", code, quiet)
	}
}

// A branch review prompts for both refs on a terminal, with or without
// --select.
func TestPickLocalRefsBranchModePromptsOnATerminal(t *testing.T) {
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "branch", "feature")
	for _, explicit := range []bool{true, false} {
		base, head := "", "HEAD"
		if err := interactiveApp(0, nil).pickLocalRefs(context.Background(), "branch", dir, explicit, false, false,
			localRefs{base: &base, head: &head}); err != nil {
			t.Fatal(err)
		}
		if base == "" || head == "HEAD" {
			t.Fatalf("--select=%v: base = %q, head = %q; want both picked", explicit, base, head)
		}
	}
	// Without a terminal nothing prompts and the review keeps its defaults, so
	// scripts and the daemon are unaffected.
	base, head := "", "HEAD"
	if err := (&app{}).pickLocalRefs(context.Background(), "branch", dir, false, false, false,
		localRefs{base: &base, head: &head}); err != nil {
		t.Fatal(err)
	}
	if base != "" || head != "HEAD" {
		t.Fatalf("base = %q, head = %q; want the defaults untouched", base, head)
	}
}

// The prompts open on what the command would have defaulted to, so accepting
// twice reproduces the non-interactive behaviour — including on the default
// branch, where the pair is origin/main..main.
func TestPickLocalRefsBranchModePreselectsTheDefaults(t *testing.T) {
	dir := newTestRepoWithOrigin(t)
	var prompts []pick.Options
	a := &app{selectFn: func(opts pick.Options) (int, error) {
		prompts = append(prompts, opts)
		return opts.Initial, nil
	}}
	base, head := "", "HEAD"
	if err := a.pickLocalRefs(context.Background(), "branch", dir, false, false, false,
		localRefs{base: &base, head: &head}); err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 2 {
		t.Fatalf("prompts = %d, want one for the base and one for the head", len(prompts))
	}
	if base != "origin/main" {
		t.Fatalf("base = %q, want the default branch preselected", base)
	}
	current := gitTestOutput(t, dir, "symbolic-ref", "--short", "HEAD")
	if head != current {
		t.Fatalf("head = %q, want the checked-out branch %q preselected", head, current)
	}
	// The local branch and its remote-tracking ref are one row, starred in the
	// marker column left of the name, and only the default branch is coloured.
	starred := 0
	for _, item := range prompts[1].Items {
		if item.Cells[0] == checkedOutMark {
			starred++
			if item.Cells[1] != current {
				t.Fatalf("starred row = %q, want the checked-out branch %q", item.Cells[1], current)
			}
		}
		if item.Cells[1] == "origin/main" {
			t.Fatalf("items = %+v, want origin/main folded into its local branch", prompts[1].Items)
		}
	}
	if starred != 1 {
		t.Fatalf("starred rows = %d, want exactly the checked-out branch", starred)
	}
	if got := prompts[0].CellStyles[0]; got != pick.StyleMark {
		t.Fatalf("marker column style = %q, want the mark colour", got)
	}
	if got := prompts[0].CellStyles[1]; got != "" {
		t.Fatalf("name column style = %q, want it plain except for the default branch", got)
	}
	for _, item := range prompts[0].Items {
		style := ""
		if len(item.CellStyles) > 1 {
			style = item.CellStyles[1]
		}
		if isDefault := item.Cells[1] == current; isDefault != (style == pick.StyleDefaultRef) {
			t.Fatalf("row %q: name style %q, want green only on the default branch", item.Cells[1], style)
		}
	}
}

// Every ref of a branch is one row: `main`, `origin/main` and origin's HEAD
// alias all mean main, and a branch that exists only on a remote is named by
// the ref that exists.
func TestGroupBranches(t *testing.T) {
	newest := time.Now()
	refs := []git.BranchRef{
		{Name: "main", Branch: "main", Current: true, Subject: "local tip", Author: "alice", Date: newest},
		{Name: "origin/main", Remote: "origin", Branch: "main", Subject: "pushed tip", Author: "bob", Date: newest.Add(-time.Hour)},
		{Name: "upstream/main", Remote: "upstream", Branch: "main", Date: newest.Add(-2 * time.Hour)},
		{Name: "origin/only-remote", Remote: "origin", Branch: "only-remote", Subject: "remote only", Date: newest.Add(-3 * time.Hour)},
	}
	choices := groupBranches(refs, "origin/main")
	if len(choices) != 2 {
		t.Fatalf("choices = %+v, want one per branch", choices)
	}
	main := choices[0]
	if main.name != "main" || main.local != "main" || !main.current || !main.isDefault {
		t.Fatalf("main = %+v", main)
	}
	// origin wins over another remote tracking the same branch.
	if main.remote != "origin/main" {
		t.Fatalf("main remote = %q, want origin's ref", main.remote)
	}
	// The newest tip of the group describes the row.
	if main.subject != "local tip" || main.author != "alice" {
		t.Fatalf("main metadata = %+v, want the newest tip's", main)
	}
	// A base resolves to the remote-tracking ref (what --base main resolves to
	// as well), a head to the local branch.
	if got := main.ref(true); got != "origin/main" {
		t.Fatalf("base ref = %q", got)
	}
	if got := main.ref(false); got != "main" {
		t.Fatalf("head ref = %q", got)
	}
	remoteOnly := choices[1]
	if remoteOnly.name != "origin/only-remote" || remoteOnly.local != "" || remoteOnly.isDefault {
		t.Fatalf("remote-only choice = %+v", remoteOnly)
	}
	// With nothing local, both sides resolve to the ref that exists.
	if remoteOnly.ref(true) != "origin/only-remote" || remoteOnly.ref(false) != "origin/only-remote" {
		t.Fatalf("remote-only refs = %q / %q", remoteOnly.ref(true), remoteOnly.ref(false))
	}
}

// A branch review runs against the remote-tracking base and the local head,
// which is what the non-interactive defaults resolve to.
func TestPickBranchResolvesTheRequestedSide(t *testing.T) {
	dir := newTestRepoWithOrigin(t)
	base, err := interactiveApp(0, nil).pickBranch(context.Background(), dir, branchPrompt{
		title: "base", label: "Base branch ", preferred: "origin/main", remote: true, style: pick.StyleBaseRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if base != "origin/main" {
		t.Fatalf("base = %q, want the remote-tracking ref", base)
	}
	head, err := interactiveApp(0, nil).pickBranch(context.Background(), dir, branchPrompt{
		title: "head", label: "Head branch ", style: pick.StyleHeadRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if head != gitTestOutput(t, dir, "symbolic-ref", "--short", "HEAD") {
		t.Fatalf("head = %q, want the local branch", head)
	}
}

// The title line carries the ref the prompt would record, so a truncated name
// column costs nothing: the base prompt shows the remote-tracking side, the
// head prompt the local one.
func TestPickBranchDetailNamesTheResolvedRef(t *testing.T) {
	dir := newTestRepoWithOrigin(t)
	current := gitTestOutput(t, dir, "symbolic-ref", "--short", "HEAD")
	for _, tc := range []struct {
		name         string
		preferRemote bool
		style        string
		want         string
	}{
		{"base", true, pick.StyleBaseRef, "origin/main"},
		{"head", false, pick.StyleHeadRef, current},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var captured pick.Options
			if _, err := interactiveApp(0, &captured).pickBranch(context.Background(), dir, branchPrompt{
				title: "t", remote: tc.preferRemote, style: tc.style,
			}); err != nil {
				t.Fatal(err)
			}
			if got := captured.Items[0].Detail; got != tc.want {
				t.Fatalf("detail = %q, want %q", got, tc.want)
			}
			// The two prompts wear different colours, so a base prompt is never
			// mistaken for a head prompt.
			if captured.DetailStyle != tc.style {
				t.Fatalf("detail style = %q, want %q", captured.DetailStyle, tc.style)
			}
		})
	}
	if pick.StyleBaseRef == pick.StyleHeadRef {
		t.Fatal("the base and head sides must not share a colour")
	}
}

// The two prompts of a branch review name their side and colour it differently.
func TestPickLocalRefsBranchPromptsDifferBySide(t *testing.T) {
	dir := newTestRepoWithOrigin(t)
	var prompts []pick.Options
	a := &app{selectFn: func(opts pick.Options) (int, error) {
		prompts = append(prompts, opts)
		return opts.Initial, nil
	}}
	base, head := "", "HEAD"
	if err := a.pickLocalRefs(context.Background(), "branch", dir, false, false, false,
		localRefs{base: &base, head: &head}); err != nil {
		t.Fatal(err)
	}
	if prompts[0].Title != "Base to review against:" || prompts[1].Title != "Branch to review:" {
		t.Fatalf("titles = %q / %q", prompts[0].Title, prompts[1].Title)
	}
	if prompts[0].DetailStyle != pick.StyleBaseRef || prompts[1].DetailStyle != pick.StyleHeadRef {
		t.Fatalf("detail styles = %q / %q, want the base and head colours",
			prompts[0].DetailStyle, prompts[1].DetailStyle)
	}
	// The name column is the first to give way; the tip message outranks it.
	if got := prompts[0].ColumnPriority; len(got) < 2 || got[1] != pick.PriorityLow {
		t.Fatalf("column priorities = %v, want the name column ranked lowest", got)
	}
}

// The confirmation is an aside about the run: italic light grey, with the
// chosen value in the colour the picker gave it.
func TestWriteSelection(t *testing.T) {
	var styled strings.Builder
	writeSelection(&styled, true, "Base branch ", "origin/main", pick.StyleDetail, "")
	// The ref keeps the faded separators it had in the list.
	want := "\x1b[" + selectionStyle + "mBase branch \x1b[0m" +
		"\x1b[" + pick.StyleDetail + "morigin\x1b[0m" +
		"\x1b[" + pick.StyleSeparator + "m/\x1b[0m" +
		"\x1b[" + pick.StyleDetail + "mmain\x1b[0m" +
		"\x1b[" + selectionStyle + "m\x1b[0m\n"
	if got := styled.String(); got != want {
		t.Fatalf("styled = %q, want %q", got, want)
	}
	// Without colour (no terminal, or NO_COLOR) the same words are printed bare.
	var plain strings.Builder
	writeSelection(&plain, false, "Selected merge request ", "!142", pick.StyleIdentifier, " feat: x")
	if got := plain.String(); got != "Selected merge request !142 feat: x\n" {
		t.Fatalf("plain = %q", got)
	}
}

// Reviewing a branch against itself is an empty diff, so the head prompt opens
// on the newest branch when the base is the branch we are on.
func TestPickLocalRefsHeadSkipsTheBaseBranch(t *testing.T) {
	dir := newTestRepoWithOrigin(t)
	current := gitTestOutput(t, dir, "symbolic-ref", "--short", "HEAD")
	// A newer branch, so the listing does not start on the checked-out one.
	runGitTestCommand(t, dir, "checkout", "-q", "-b", "feature")
	runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", "newer")
	runGitTestCommand(t, dir, "checkout", "-q", current)

	var prompts []pick.Options
	a := &app{selectFn: func(opts pick.Options) (int, error) {
		prompts = append(prompts, opts)
		return opts.Initial, nil
	}}
	base, head := "", "HEAD"
	if err := a.pickLocalRefs(context.Background(), "branch", dir, false, false, false,
		localRefs{base: &base, head: &head}); err != nil {
		t.Fatal(err)
	}
	if base != "origin/main" {
		t.Fatalf("base = %q, want the default branch", base)
	}
	if prompts[1].Initial != 0 {
		t.Fatalf("head prompt starts on row %d, want the newest branch", prompts[1].Initial)
	}
	if head != "feature" {
		t.Fatalf("head = %q, want the newest branch, not the base branch %q", head, current)
	}

	// Off the base branch the current branch is still the obvious head.
	runGitTestCommand(t, dir, "checkout", "-q", "feature")
	prompts = nil
	base, head = "", "HEAD"
	if err := a.pickLocalRefs(context.Background(), "branch", dir, false, false, false,
		localRefs{base: &base, head: &head}); err != nil {
		t.Fatal(err)
	}
	if head != "feature" {
		t.Fatalf("head = %q, want the checked-out branch preselected", head)
	}
	if prompts[1].Items[prompts[1].Initial].Cells[0] != checkedOutMark {
		t.Fatalf("head prompt starts on %q, want the starred row", prompts[1].Items[prompts[1].Initial].Cells[1])
	}
}

// The branch name is a ref and the tip message a commit subject, so the picker
// paints inside those columns.
func TestPickBranchDeclaresColumnKinds(t *testing.T) {
	dir := newTestRepoWithOrigin(t)
	var captured pick.Options
	if _, err := interactiveApp(0, &captured).pickBranch(context.Background(), dir, branchPrompt{
		title: "t", style: pick.StyleHeadRef,
	}); err != nil {
		t.Fatal(err)
	}
	want := []pick.ColumnKind{pick.KindPlain, pick.KindRef, pick.KindMessage}
	if !slices.Equal(captured.ColumnKinds, want) {
		t.Fatalf("column kinds = %v, want %v", captured.ColumnKinds, want)
	}
}

func TestAgeStyle(t *testing.T) {
	if got := ageStyle(time.Now().Add(-30 * time.Minute)); got != pick.StyleFresh {
		t.Fatalf("ageStyle inside the hour = %q, want the fresh colour", got)
	}
	if got := ageStyle(time.Now().Add(-3 * time.Hour)); got != "" {
		t.Fatalf("ageStyle beyond the hour = %q, want the column default", got)
	}
	if got := ageStyle(time.Time{}); got != "" {
		t.Fatalf("ageStyle of no timestamp = %q, want the column default", got)
	}
}

func TestRowStyles(t *testing.T) {
	if got := rowStyles(4, map[int]string{1: pick.StyleFresh}); !slices.Equal(got, []string{"", pick.StyleFresh, "", ""}) {
		t.Fatalf("rowStyles = %q", got)
	}
	// Nothing deviating means no per-row slice at all.
	if got := rowStyles(4, map[int]string{2: ""}); got != nil {
		t.Fatalf("rowStyles = %q, want nil", got)
	}
	// An out-of-range column is dropped rather than growing the row.
	if got := rowStyles(2, map[int]string{5: pick.StyleFresh}); got != nil {
		t.Fatalf("rowStyles = %q, want nil", got)
	}
}

// --select fills only what the command line left open.
func TestPickLocalRefsKeepsExplicitRefs(t *testing.T) {
	dir := newTestRepo(t)
	base, head := "some/base", "HEAD"
	if err := interactiveApp(0, nil).pickLocalRefs(context.Background(), "branch", dir, true, true, false,
		localRefs{base: &base, head: &head}); err != nil {
		t.Fatal(err)
	}
	if base != "some/base" {
		t.Fatalf("base = %q, want the explicit value kept", base)
	}
	if head == "HEAD" {
		t.Fatalf("head = %q, want the unset selector picked", head)
	}
}

// `git commits` without --from cannot run at all, so the base commit is picked
// even without --select — but only where a list can be drawn. The base list
// starts below the head: a range whose base is its own head reviews nothing,
// and the log being newest-first is exactly where the cursor would land.
func TestPickLocalRefsCommitsModeImplicitBase(t *testing.T) {
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", "second")
	head := gitTestOutput(t, dir, "rev-parse", "HEAD")
	parent := gitTestOutput(t, dir, "rev-parse", "HEAD~1")

	from, to := "", "HEAD"
	if err := interactiveApp(0, nil).pickLocalRefs(context.Background(), "commits", dir, false, false, false,
		localRefs{base: &from, head: &to}); err != nil {
		t.Fatal(err)
	}
	// A range of the one row the seam answers with: the newest commit, whose
	// base is its parent.
	if from != parent {
		t.Fatalf("from = %q, want the head's parent %q so accepting reviews the head commit", from, parent)
	}
	if from == head {
		t.Fatalf("from = %q, want a base that is not the range head", from)
	}
	if to != head {
		t.Fatalf("to = %q, want the chosen head %q recorded", to, head)
	}

	from, to = "", "HEAD"
	if err := (&app{}).pickLocalRefs(context.Background(), "commits", dir, false, false, false,
		localRefs{base: &from, head: &to}); err != nil {
		t.Fatal(err)
	}
	if from != "" || to != "HEAD" {
		t.Fatalf("from = %q, to = %q; want no picker without a terminal", from, to)
	}
}

// A repository's first commit has no parent, so the base is git's empty tree
// and the commit is reviewed instead of the range failing.
func TestPickCommitRangeOverTheRootCommit(t *testing.T) {
	dir := newTestRepo(t)
	base, head, err := interactiveApp(0, nil).pickCommitRange(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	// The empty tree of this repository, not a hard-coded SHA-1 one: a
	// SHA-256 repository has a different id for it.
	if want := gitTestOutput(t, dir, "hash-object", "-t", "tree", os.DevNull); base != want {
		t.Fatalf("base = %q, want the repository's empty tree %q", base, want)
	}
	if head != gitTestOutput(t, dir, "rev-parse", "HEAD") {
		t.Fatalf("head = %q, want the only commit", head)
	}
}

// Both ends open: one list, the range marked out on the commits to review, and
// the exclusive base derived from the oldest of them.
func TestPickCommitRangeCoversTheChosenCommits(t *testing.T) {
	dir := newTestRepo(t)
	for _, message := range []string{"second", "third", "fourth"} {
		runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", message)
	}
	var captured pick.Options
	a := &app{selectRangeFn: func(opts pick.Options) (int, int, error) {
		captured = opts
		// Rows 0..1: the two newest commits.
		return 0, 1, nil
	}}
	base, head, err := a.pickCommitRange(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !captured.Range || captured.RangeUnit != "commit" {
		t.Fatalf("options = %+v, want a range prompt counting commits", captured)
	}
	if captured.Title != "Commits to review:" {
		t.Fatalf("title = %q", captured.Title)
	}
	// The span's oldest commit is reviewed too, so the recorded base is its
	// parent — HEAD~2 for a range of the two newest commits.
	if want := gitTestOutput(t, dir, "rev-parse", "HEAD~2"); base != want {
		t.Fatalf("base = %q, want %q", base, want)
	}
	if want := gitTestOutput(t, dir, "rev-parse", "HEAD"); head != want {
		t.Fatalf("head = %q, want %q", head, want)
	}
	// Every row carries its short SHA as the detail the span summary names.
	if captured.Items[0].Detail != gitTestOutput(t, dir, "rev-parse", "--short", "HEAD") {
		t.Fatalf("detail = %q, want the short SHA", captured.Items[0].Detail)
	}
}

// The base of a range must come from the history the range ends at: with an
// explicit --to, offering the checked-out branch's commits would let an
// unrelated (or invalid) range be picked.
func TestPickLocalRefsCommitsBaseFollowsTheExplicitHead(t *testing.T) {
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "checkout", "-q", "-b", "release")
	runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", "release only")
	releaseHead := gitTestOutput(t, dir, "rev-parse", "HEAD")
	runGitTestCommand(t, dir, "checkout", "-q", "-")
	runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", "main only")
	mainHead := gitTestOutput(t, dir, "rev-parse", "HEAD")

	releaseParent := gitTestOutput(t, dir, "rev-parse", "release~1")

	var captured pick.Options
	from, to := "", "release"
	if err := interactiveApp(0, &captured).pickLocalRefs(context.Background(), "commits", dir, false, false, true,
		localRefs{base: &from, head: &to}); err != nil {
		t.Fatal(err)
	}
	// A fixed head leaves one end open, so the prompt asks for the first commit
	// to review out of that head's history — and its parent is the base.
	if captured.Title != "First commit to review:" || captured.Range {
		t.Fatalf("options = %+v, want the single-end prompt", captured)
	}
	if from != releaseParent {
		t.Fatalf("base = %q, want the parent of the --to branch %q", from, releaseParent)
	}
	if from == releaseHead || from == mainHead {
		t.Fatalf("base = %q, want neither the range head nor a commit of the checkout", from)
	}
	if to != "release" {
		t.Fatalf("head = %q, want the explicit --to kept", to)
	}
}

// With both ends open there is one prompt, not two: the range is chosen on the
// commits themselves.
func TestPickLocalRefsCommitsAsksOnce(t *testing.T) {
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", "second")
	runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", "third")
	var titles []string
	a := &app{selectRangeFn: func(opts pick.Options) (int, int, error) {
		titles = append(titles, opts.Title)
		return 0, 1, nil
	}}
	from, to := "", "HEAD"
	if err := a.pickLocalRefs(context.Background(), "commits", dir, true, false, false,
		localRefs{base: &from, head: &to}); err != nil {
		t.Fatal(err)
	}
	if len(titles) != 1 || titles[0] != "Commits to review:" {
		t.Fatalf("prompts = %q, want a single range prompt", titles)
	}
	if len(from) != 40 || len(to) != 40 {
		t.Fatalf("from = %q, to = %q; want both resolved to full SHAs", from, to)
	}
}

func TestPickLocalRefsRejectsSelectWithoutTerminal(t *testing.T) {
	base, head := "", "HEAD"
	err := (&app{}).pickLocalRefs(context.Background(), "branch", t.TempDir(), true, false, false,
		localRefs{base: &base, head: &head})
	if err == nil || !strings.Contains(err.Error(), "--select needs a terminal") {
		t.Fatalf("err = %v, want it to require a terminal", err)
	}
}

func TestRelativeAge(t *testing.T) {
	now := time.Now()
	cases := []struct {
		in   time.Time
		want string
	}{
		{time.Time{}, ""},
		{now, "now"},
		{now.Add(-90 * time.Second), "1m"},
		{now.Add(-3 * time.Hour), "3h"},
		{now.Add(-50 * time.Hour), "2d"},
		{now.Add(-60 * 24 * time.Hour), "2mo"},
		{now.Add(-800 * 24 * time.Hour), "2y"},
		// A clock skew must not render as a negative age.
		{now.Add(time.Hour), "now"},
	}
	for _, tc := range cases {
		if got := relativeAge(tc.in); got != tc.want {
			t.Fatalf("relativeAge(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// gitTestOutput runs git and returns its trimmed stdout.
func gitTestOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// newTestRepoWithOrigin adds the remote-tracking refs a default-branch lookup
// needs: origin/main plus the origin/HEAD symref that names it.
func newTestRepoWithOrigin(t *testing.T) string {
	t.Helper()
	dir := newTestRepo(t)
	runGitTestCommand(t, dir, "remote", "add", "origin", "git@gitlab.example.com:grp/proj.git")
	runGitTestCommand(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	runGitTestCommand(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	return dir
}

// newTestRepo creates a repository with one commit, which the ref and commit
// pickers need something to list.
func newTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	runGitTestCommand(t, dir, "init", "--quiet", ".")
	runGitTestCommand(t, dir, "config", "user.email", "test@example.com")
	runGitTestCommand(t, dir, "config", "user.name", "Test")
	runGitTestCommand(t, dir, "commit", "-q", "--allow-empty", "-m", "first commit")
	return dir
}
