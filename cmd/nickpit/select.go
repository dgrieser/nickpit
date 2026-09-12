package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// selectFlagName is the flag every command with something to pick shares, so
// `--select` means the same everywhere: choose interactively instead of naming
// the target on the command line.
const selectFlagName = "select"

// addSelectFlag registers --select with the wording of what this command picks;
// note adds a command-specific condition to the shared terminal requirement.
func addSelectFlag(cmd *cobra.Command, target *bool, what, note string) {
	usage := "Pick " + what + " interactively from a list (requires a terminal"
	if note != "" {
		usage += "; " + note
	}
	cmd.Flags().BoolVar(target, selectFlagName, false, usage+")")
}

// requestSelectNote is the extra condition of picking a merge or pull request:
// the project has to be known before its open requests can be listed.
const requestSelectNote = "the project comes from --repo or the git remote of the current directory"

// interactiveSelect reports whether this invocation may draw a picker. A test
// seam counts as interactive; otherwise the key source (stdin) and the drawing
// surface (stderr) must both be terminals, so piped, redirected and
// daemon-spawned runs keep failing with their non-interactive error instead of
// waiting for a keypress nobody can send.
//
// The check is the picker's own (a tty ioctl), not "is a character device":
// /dev/null is a character device, so a cron or daemon run with stdin and
// stderr on it would otherwise reach the picker — after listing the open
// requests over the API — only to fail there instead of reporting the missing
// --id up front.
func (a *app) interactiveSelect() bool {
	if a.selectFn != nil || a.selectRangeFn != nil || a.selectViewFn != nil {
		return true
	}
	return isInteractiveTerminal(os.Stdin) && isInteractiveTerminal(os.Stderr)
}

// isInteractiveTerminal reports whether f is a terminal a person could be
// watching, the same test pick.Select gates on.
func isInteractiveTerminal(f *os.File) bool {
	return f != nil && term.IsTerminal(int(f.Fd()))
}

// selectOne draws opts and returns the chosen index. The picker reads keys from
// stdin and draws on stderr, keeping stdout free for the review output the
// command writes afterwards.
func (a *app) selectOne(opts pick.Options) (int, error) {
	first, _, err := a.selectRange(opts)
	return first, err
}

// selectRange draws opts and returns the ends of the chosen range, which are
// the same row unless opts.Range is set.
func (a *app) selectRange(opts pick.Options) (int, int, error) {
	if a.selectRangeFn != nil {
		return a.selectRangeFn(opts)
	}
	if a.selectFn != nil {
		index, err := a.selectFn(opts)
		if err != nil {
			return -1, -1, err
		}
		// A seam that answers with one row means a range of that one row, so a
		// range prompt stays answerable without a terminal.
		return index, index, nil
	}
	return pick.SelectRange(os.Stdin, os.Stderr, opts)
}

// selectView draws a multi-scope list and returns the scope the user ended in
// together with the chosen row inside that scope's items.
func (a *app) selectView(opts pick.Options) (int, int, error) {
	if a.selectViewFn != nil {
		return a.selectViewFn(opts)
	}
	if a.selectFn != nil {
		// A seam that answers with one row answers in the scope the list opened
		// on, so a scoped prompt stays answerable without a terminal.
		index, err := a.selectFn(opts)
		return opts.View, index, err
	}
	return pick.SelectView(os.Stdin, os.Stderr, opts)
}

// requestTarget addresses one merge request or pull request. An ID of 0 means
// the invocation did not name one and the picker has to supply it.
type requestTarget struct {
	Repo    string
	ID      int
	BaseURL string
}

// requestSelectors is the selector flag set shared by every MR/PR-addressed
// command: `gitlab mr`, `github pr`, both `feedback` commands, and
// `chat --gitlab`.
type requestSelectors struct {
	repo   string
	id     int
	rawURL string
	// pick is --select: draw the list even where a target could be resolved
	// without it.
	pick bool
	// changed reports whether a selector flag was actually passed, so an
	// explicitly supplied default — `--url … --id=0`, `--url … --repo=''`,
	// `--url='' --id=42` — still hits the exclusivity policy instead of
	// slipping through as "unset". nil compares values instead, which is what
	// a caller without a cobra command needs (chat validates its own flags
	// that way).
	changed func(flag string) bool
}

// given reports whether a selector was supplied: by flag presence where the
// caller could tell us, by value otherwise.
func (sel requestSelectors) given(flag string, hasValue bool) bool {
	if sel.changed != nil {
		return sel.changed(flag)
	}
	return hasValue
}

// resolveRequestTarget applies the selector policy those commands share.
//
// --url fully determines project, id and host, so it combines with neither
// --repo nor --id: a stale URL silently outvoting an explicit value could read
// or post on the wrong request, or send the token to the wrong host. A missing
// repo is inferred from the git remote of the current directory. A missing id
// is left at 0 for the picker when this invocation may draw one; without a
// terminal the long-standing error stands, so scripts and the serve daemon see
// no new behaviour.
//
// prefix names the command in errors ("chat" for the chat command, empty for
// the ones whose cobra wrapper already prefixes them), noun the change under
// review.
func (a *app) resolveRequestTarget(sel requestSelectors, parse func(string) (string, int, string, error),
	prefix, noun string) (requestTarget, error) {
	fail := func(format string, args ...any) (requestTarget, error) {
		if prefix != "" {
			format = prefix + ": " + format
		}
		return requestTarget{}, fmt.Errorf(format, args...)
	}
	target := requestTarget{Repo: sel.repo, ID: sel.id}
	urlGiven := sel.given("url", sel.rawURL != "")
	idGiven := sel.given("id", sel.id != 0)
	repoGiven := sel.given("repo", sel.repo != "")
	switch {
	case urlGiven:
		if idGiven {
			return fail("--url can not be combined with --id")
		}
		if repoGiven {
			return fail("--url can not be combined with --repo")
		}
		if sel.pick {
			return fail("--url can not be combined with --%s", selectFlagName)
		}
		var err error
		if target.Repo, target.ID, target.BaseURL, err = parse(sel.rawURL); err != nil {
			return requestTarget{}, err
		}
	case sel.pick:
		if idGiven {
			return fail("--%s can not be combined with --id", selectFlagName)
		}
		if !a.interactiveSelect() {
			return fail("--%s needs a terminal; pass --id or --url instead", selectFlagName)
		}
		// Left at 0: the caller picks once its client exists.
	case idGiven:
		// An id that was passed has to be a request; only a missing one defers
		// to the picker, so --id=0 and --id=-1 fail here rather than reach the
		// API as request "0" or "-1".
		if target.ID <= 0 {
			return fail("--id must be a positive integer")
		}
	case !a.interactiveSelect():
		return fail("--id must be a positive integer (pass --url instead, or run in a terminal to pick an open %s from a list)", noun)
	}
	if target.Repo == "" {
		if target.Repo = inferRepo(); target.Repo == "" {
			return fail("--repo is required (could not infer from git remote)")
		}
	}
	return target, nil
}

// ensureInteractive rejects an explicit --select where no list can be drawn,
// instead of blocking on a terminal nobody is watching.
func (a *app) ensureInteractive() error {
	if a.interactiveSelect() {
		return nil
	}
	return fmt.Errorf("--%s needs a terminal", selectFlagName)
}

// localRefs are the revision selectors of a local review: the branch pair of
// `git branch`, or the commit pair of `git commits`. The picker fills the ones
// this invocation left unset, so --select combines with an explicit --base or
// --to.
type localRefs struct {
	base *string
	head *string
}

// pickLocalRefs fills the unset revision selectors of a local review from a
// list. On a terminal both range submodes prompt for what the command line left
// open; the picker starts on the value the command would have defaulted to (the
// default branch as the base, the checked-out branch as the head), so accepting
// twice reproduces `nickpit git branch` as it behaves without a terminal —
// including on the default branch itself, where that pair is
// origin/main..main, the commits not pushed yet. explicit is --select, which
// only adds the terminal requirement where nothing would prompt otherwise.
func (a *app) pickLocalRefs(ctx context.Context, submode, repoRoot string, explicit bool,
	baseSet, headSet bool, refs localRefs) error {
	if explicit {
		if err := a.ensureInteractive(); err != nil {
			return err
		}
	}
	prompt := explicit || a.interactiveSelect()
	switch submode {
	case "branch":
		if prompt && !baseSet {
			preferred, _ := git.DefaultBranch(ctx, repoRoot)
			chosen, err := a.pickBranch(ctx, repoRoot, branchPrompt{
				title:     "Base to review against:",
				label:     "Base branch ",
				preferred: preferred,
				remote:    true,
				style:     pick.StyleBaseRef,
			})
			if err != nil {
				return err
			}
			*refs.base = chosen
		}
		if prompt && !headSet {
			preferred, _ := git.CurrentBranch(ctx, repoRoot)
			// Reviewing a branch against itself is an empty diff, so when the
			// base is the branch we are on, the prompt opens on the newest
			// branch instead of preselecting that same one.
			if baseIsCurrentBranch(ctx, repoRoot, *refs.base) {
				preferred = ""
			}
			chosen, err := a.pickBranch(ctx, repoRoot, branchPrompt{
				title:     "Branch to review:",
				label:     "Head branch ",
				preferred: preferred,
				style:     pick.StyleHeadRef,
			})
			if err != nil {
				return err
			}
			*refs.head = chosen
		}
	case "commits":
		if *refs.base != "" || !prompt {
			// An explicit --from settles the range; nothing to ask.
			return nil
		}
		if headSet {
			// The head is fixed, so only the other end is open: pick the oldest
			// commit to review out of that head's history.
			base, err := a.pickCommitStart(ctx, repoRoot, *refs.head)
			if err != nil {
				return err
			}
			*refs.base = base
			return nil
		}
		// Both ends are open: one list, where the range is marked out on the
		// commits themselves.
		base, head, err := a.pickCommitRange(ctx, repoRoot)
		if err != nil {
			return err
		}
		*refs.base, *refs.head = base, head
	}
	return nil
}

// openRequestList is the platform's half of picking a request: how to list the
// open ones and how to name and mark them.
type openRequestList struct {
	// noun is the platform's word for the change under review ("merge request",
	// "pull request").
	noun string
	// marker prefixes the identifier the way the platform writes it: "!42" on
	// GitLab, "#42" on GitHub.
	marker string
	list   func(ctx context.Context) ([]model.OpenRequest, error)
}

// pickOpenRequest lists the open requests of a project and returns the
// identifier of the chosen one. The request whose source branch is the
// checked-out branch is marked and preselected: in a checkout of the repository
// that is nearly always the one meant.
func (a *app) pickOpenRequest(ctx context.Context, repo string, source openRequestList) (int, error) {
	requests, err := source.list(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing open %ss of %s: %w", source.noun, repo, err)
	}
	if len(requests) == 0 {
		return 0, fmt.Errorf("no open %s in %s (pass --id or --url to address a merged or closed one)", source.noun, repo)
	}
	branch := currentBranchName(ctx)
	items := make([]pick.Item, len(requests))
	drafts := false
	for _, request := range requests {
		if request.Draft {
			drafts = true
			break
		}
	}
	// -1, not 0: index 0 is a legitimate match, and "no match yet" has to be
	// distinguishable from it or a later match would overwrite the first.
	initial := -1
	for i, request := range requests {
		mark := ""
		if branch != "" && request.SourceBranch == branch {
			mark = checkedOutMark
			// Only the first match preselects: several open requests can share a
			// source branch, and the newest (this listing's first) is the one to
			// offer.
			if initial < 0 {
				initial = i
			}
		}
		cells := []string{mark, source.marker + strconv.Itoa(request.Identifier)}
		if drafts {
			// The column exists only when some request is a draft, so a list
			// without drafts carries no blank column.
			draft := ""
			if request.Draft {
				draft = "draft"
			}
			cells = append(cells, draft)
		}
		cells = append(cells,
			textsan.StripControl(request.Title),
			textsan.StripControl(request.Author),
			relativeAge(request.UpdatedAt))
		items[i] = pick.Item{
			Cells: cells,
			// The branches are not shown (they rarely fit next to the title) but
			// they are worth filtering on: typing a branch name finds its request.
			Match: fmt.Sprintf("%s%d %s %s %s %s", source.marker, request.Identifier, request.Title,
				request.Author, request.SourceBranch, request.TargetBranch),
			CellStyles: rowStyles(len(cells), map[int]string{
				len(cells) - 2: pick.AuthorStyle(request.Author),
				len(cells) - 1: ageStyle(request.UpdatedAt),
			}),
		}
	}
	styles := []string{pick.StyleMark, pick.StyleIdentifier, pick.StyleText, pick.StyleAuthor, pick.StyleAge}
	kinds := []pick.ColumnKind{pick.KindPlain, pick.KindPlain, pick.KindMessage}
	if drafts {
		styles = slices.Insert(styles, 2, pick.StyleCaveat)
		kinds = slices.Insert(kinds, 2, pick.KindPlain)
	}
	index, err := a.selectOne(pick.Options{
		Title:       fmt.Sprintf("Open %ss in %s", source.noun, textsan.StripControl(repo)),
		Items:       items,
		Initial:     max(initial, 0),
		CellStyles:  styles,
		ColumnKinds: kinds,
	})
	if err != nil {
		return 0, err
	}
	chosen := requests[index]
	a.printSelection("Selected "+source.noun+" ",
		source.marker+strconv.Itoa(chosen.Identifier), pick.StyleIdentifier,
		" "+textsan.StripControl(chosen.Title))
	return chosen.Identifier, nil
}

// checkedOutMark flags what belongs to the branch you are on — the checked-out
// branch in a branch list, its open request in a request list — in its own
// column left of everything else, so the eye finds it without reading the row.
const checkedOutMark = "★"

// branchChoice is one row of the branch picker: a branch with whatever refs
// exist for it. A local branch and its remote-tracking counterparts are one
// choice, not three rows of the same name — `main`, `origin/main` and
// `origin`'s HEAD alias all mean "main".
type branchChoice struct {
	// name is what the row shows: the plain branch name when it exists locally,
	// the remote-tracking ref when it only exists on a remote.
	name string
	// local and remote are the refs a review can actually be run against;
	// either may be empty.
	local   string
	remote  string
	current bool
	// isDefault marks the repository's default branch.
	isDefault bool
	subject   string
	author    string
	date      time.Time
}

// ref resolves a chosen row to the ref the review records. preferRemote picks
// the remote-tracking side, which is what a base branch resolves to anyway
// (`--base main` becomes origin/main when that ref exists); a head keeps the
// local branch, which is the one being worked on.
func (c branchChoice) ref(preferRemote bool) string {
	if preferRemote && c.remote != "" {
		return c.remote
	}
	if c.local != "" {
		return c.local
	}
	if c.remote != "" {
		return c.remote
	}
	return c.name
}

// groupBranches folds every ref of a branch into one choice, keeping the input
// order (git lists newest tip first). defaultRef is the default branch's ref
// name, so its row can be marked. A branch tracked on several remotes takes
// origin's ref, else the first one seen; the newest tip of the group supplies
// the row's subject, author and date, which is what a reader wants to see when
// a local branch and its remote have drifted apart.
func groupBranches(refs []git.BranchRef, defaultRef string) []branchChoice {
	choices := make([]branchChoice, 0, len(refs))
	index := map[string]int{}
	for _, ref := range refs {
		position, seen := index[ref.Branch]
		if !seen {
			index[ref.Branch] = len(choices)
			choices = append(choices, branchChoice{name: ref.Branch})
			position = len(choices) - 1
		}
		choice := &choices[position]
		switch {
		case ref.Remote == "":
			choice.local = ref.Name
		case choice.remote == "" || ref.Remote == "origin":
			choice.remote = ref.Name
		}
		choice.current = choice.current || ref.Current
		if defaultRef != "" && ref.Name == defaultRef {
			choice.isDefault = true
		}
		if ref.Date.After(choice.date) {
			choice.subject, choice.author, choice.date = ref.Subject, ref.Author, ref.Date
		}
	}
	for i := range choices {
		if choices[i].local == "" {
			// Nothing local to name it by: show the ref that exists.
			choices[i].name = choices[i].remote
		}
	}
	return choices
}

// baseIsCurrentBranch reports whether the chosen base ref belongs to the
// checked-out branch — "origin/main" while on main included, since a base and
// its local branch are one branch. The grouping already knows which branch is
// checked out, so the answer needs no prefix guessing.
func baseIsCurrentBranch(ctx context.Context, repoRoot, baseRef string) bool {
	if baseRef == "" {
		return false
	}
	refs, err := git.Branches(ctx, repoRoot)
	if err != nil {
		return false
	}
	for _, choice := range groupBranches(refs, "") {
		if choice.local == baseRef || choice.remote == baseRef || choice.name == baseRef {
			return choice.current
		}
	}
	return false
}

// branchPrompt is one side of a branch review: what the prompt asks, what it
// opens on, which ref a folded row resolves to, and the colour and wording that
// side wears — so a base prompt and a head prompt are told apart in the title
// line and in the confirmation alike.
type branchPrompt struct {
	title string
	// label names the side in the confirmation ("Base branch ").
	label string
	// preferred is the ref the command would have used, preselected wherever it
	// appears in the list.
	preferred string
	// remote resolves a folded row to its remote-tracking side, which is what a
	// base is; a head keeps the local branch.
	remote bool
	style  string
}

// pickBranch offers the repository's branches — one row per branch, whichever
// refs it has — and returns the chosen ref.
func (a *app) pickBranch(ctx context.Context, repoRoot string, prompt branchPrompt) (string, error) {
	refs, err := git.Branches(ctx, repoRoot)
	if err != nil {
		return "", fmt.Errorf("listing branches: %w", err)
	}
	defaultRef, _ := git.DefaultBranch(ctx, repoRoot)
	choices := groupBranches(refs, defaultRef)
	if len(choices) == 0 {
		return "", fmt.Errorf("no branches found in %s", repoRoot)
	}
	items := make([]pick.Item, len(choices))
	initial := 0
	for i, choice := range choices {
		mark := ""
		if choice.current {
			mark = checkedOutMark
		}
		name := ""
		if choice.isDefault {
			name = pick.StyleDefaultRef
		}
		items[i] = pick.Item{
			// The name column is narrow enough to lose the tail of a long
			// branch; the detail keeps the ref the review would record — the
			// side this prompt is for — readable in the title line.
			Detail: choice.ref(prompt.remote),
			Cells: []string{
				mark,
				choice.name,
				textsan.StripControl(choice.subject),
				textsan.StripControl(choice.author),
				relativeAge(choice.date),
			},
			// Both refs of a folded row stay findable by typing.
			Match: choice.name + " " + choice.local + " " + choice.remote + " " + choice.subject + " " + choice.author,
			CellStyles: rowStyles(5, map[int]string{
				1: name,
				3: pick.AuthorStyle(choice.author),
				4: ageStyle(choice.date),
			}),
		}
		if prompt.preferred != "" && (choice.local == prompt.preferred ||
			choice.remote == prompt.preferred || choice.name == prompt.preferred) {
			initial = i
		}
	}
	index, err := a.selectOne(pick.Options{
		Title:      prompt.title,
		Items:      items,
		Initial:    initial,
		CellStyles: []string{pick.StyleMark, "", pick.StyleText, pick.StyleAuthor, pick.StyleAge},
		// The name is a ref, so its separators fade like a progress line's; the
		// tip message is a commit subject, so its prefix is taken apart.
		ColumnKinds: []pick.ColumnKind{pick.KindPlain, pick.KindRef, pick.KindMessage},
		DetailStyle: prompt.style,
		// The name column gives up its width first — down to a stub if need be:
		// the title line spells the selected branch out in full, while the tip
		// message is only ever visible in its own column.
		ColumnPriority: []int{pick.PriorityNormal, pick.PriorityLow},
	})
	if err != nil {
		return "", err
	}
	chosen := choices[index].ref(prompt.remote)
	a.printSelection(prompt.label, chosen, prompt.style, "")
	return chosen, nil
}

// maxPickedCommits bounds the commit picker: enough history to reach the point
// a review should start from, few enough to stay scrollable.
const maxPickedCommits = 100

// commitList loads the newest commits reachable from rev (HEAD when empty) and
// the picker rows for them.
func commitList(ctx context.Context, repoRoot, rev string) ([]git.CommitRef, []pick.Item, error) {
	commits, err := git.Commits(ctx, repoRoot, rev, maxPickedCommits)
	if err != nil {
		return nil, nil, fmt.Errorf("listing commits: %w", err)
	}
	if len(commits) == 0 {
		return nil, nil, fmt.Errorf("no commits found in %s", repoRoot)
	}
	items := make([]pick.Item, len(commits))
	for i, commit := range commits {
		items[i] = pick.Item{
			// The short SHA again, so the span summary in the title line can
			// name the ends of an open range.
			Detail: commit.ShortSHA,
			Cells: []string{
				commit.ShortSHA,
				textsan.StripControl(commit.Subject),
				textsan.StripControl(commit.Author),
				relativeAge(commit.Date),
			},
			Match: commit.SHA + " " + commit.Subject + " " + commit.Author,
			CellStyles: rowStyles(4, map[int]string{
				2: pick.AuthorStyle(commit.Author),
				3: ageStyle(commit.Date),
			}),
		}
	}
	return commits, items, nil
}

// commitOptions are the picker settings both commit prompts share.
func commitOptions(title string, items []pick.Item) pick.Options {
	return pick.Options{
		Title:       title,
		Items:       items,
		CellStyles:  []string{pick.StyleHash, pick.StyleText, pick.StyleAuthor, pick.StyleAge},
		ColumnKinds: []pick.ColumnKind{pick.KindPlain, pick.KindMessage},
	}
}

// baseOf is the ref a review diffs from to include commit: its first parent,
// or this repository's empty tree when there is none, so selecting down to a
// repository's first commit reviews that commit instead of failing.
func baseOf(ctx context.Context, repoRoot string, commit git.CommitRef) (string, error) {
	if commit.Parent != "" {
		return commit.Parent, nil
	}
	empty, err := git.EmptyTree(ctx, repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolving the empty tree to diff the first commit against: %w", err)
	}
	return empty, nil
}

// pickCommitRange asks for both ends of a range in one list: the range is
// marked out on the commits to review, and the exclusive base a review records
// is derived from the oldest of them. Returns the base and head refs.
func (a *app) pickCommitRange(ctx context.Context, repoRoot string) (string, string, error) {
	commits, items, err := commitList(ctx, repoRoot, "")
	if err != nil {
		return "", "", err
	}
	options := commitOptions("Commits to review:", items)
	options.Range = true
	options.RangeUnit = "commit"
	newest, oldest, err := a.selectRange(options)
	if err != nil {
		return "", "", err
	}
	head, first := commits[newest], commits[oldest]
	base, err := baseOf(ctx, repoRoot, first)
	if err != nil {
		return "", "", err
	}
	if newest == oldest {
		a.printSelection("Commit to review ", head.ShortSHA, pick.StyleHash,
			" "+textsan.StripControl(head.Subject))
	} else {
		a.printSelection("Commits to review ", first.ShortSHA+".."+head.ShortSHA, pick.StyleHash,
			fmt.Sprintf(" · %d commits", oldest-newest+1))
	}
	return base, head.SHA, nil
}

// pickCommitStart asks for the oldest commit to review when the head is already
// fixed (an explicit --to), and returns the base ref for it.
func (a *app) pickCommitStart(ctx context.Context, repoRoot, rev string) (string, error) {
	commits, items, err := commitList(ctx, repoRoot, rev)
	if err != nil {
		return "", err
	}
	index, err := a.selectOne(commitOptions("First commit to review:", items))
	if err != nil {
		return "", err
	}
	first := commits[index]
	base, err := baseOf(ctx, repoRoot, first)
	if err != nil {
		return "", err
	}
	a.printSelection("First commit ", first.ShortSHA, pick.StyleHash,
		" "+textsan.StripControl(first.Subject))
	return base, nil
}

// freshAge is how recent a timestamp has to be for its column to be coloured:
// what changed within the hour is what a reader is usually looking for.
const freshAge = time.Hour

// ageStyle colours an age column: grey normally, green while the change is
// still fresh. "" leaves the column's own colour in place.
func ageStyle(t time.Time) string {
	if t.IsZero() || time.Since(t) > freshAge {
		return ""
	}
	return pick.StyleFresh
}

// rowStyles builds a row's per-column style slice from the columns that deviate
// from their column default; empty values are dropped, so a row that deviates
// nowhere carries no styles at all.
func rowStyles(columns int, overrides map[int]string) []string {
	styles := make([]string, columns)
	any := false
	for column, style := range overrides {
		if style == "" || column < 0 || column >= columns {
			continue
		}
		styles[column] = style
		any = true
	}
	if !any {
		return nil
	}
	return styles
}

// printSelection confirms a choice on stderr, where the picker drew: the run's
// own output on stdout stays exactly what it would be with --id or --base. The
// sentence is light grey italic with the chosen value in the colour the
// picker's own header gave it, so the line reads as an aside to the run that
// follows.
func (a *app) printSelection(prefix, highlight, highlightStyle, suffix string) {
	if a.selectFn != nil || a.selectViewFn != nil {
		// A test seam replaces the terminal; there is nothing to confirm to.
		return
	}
	_, noColor := os.LookupEnv("NO_COLOR")
	writeSelection(os.Stderr, isInteractiveTerminal(os.Stderr) && !noColor, prefix, highlight, highlightStyle, suffix)
}

// writeSelection renders the confirmation, styled or plain.
func writeSelection(w io.Writer, color bool, prefix, highlight, highlightStyle, suffix string) {
	if !color {
		// A confirmation that could not be written changes nothing about the
		// selection that was made, so the error is dropped rather than surfaced.
		_, _ = fmt.Fprintf(w, "%s%s%s\n", prefix, highlight, suffix)
		return
	}
	// The highlight is painted as a ref, so a branch keeps the faded separators
	// it had in the list. An identifier or a SHA carries none and comes out
	// exactly as before.
	_, _ = fmt.Fprintf(w, "\x1b[%sm%s\x1b[0m%s\x1b[%sm%s\x1b[0m\n",
		selectionStyle, prefix, pick.PaintRef(highlight, highlightStyle), selectionStyle, suffix)
}

// selectionStyle is the aside the confirmation is set in: italic light grey
// (progressColorLightGrey), so it reads as a note about the run rather than
// part of its output.
const selectionStyle = "3;38;5;252"

// currentBranchName returns the checked-out branch of the working directory, or
// "" outside a repository or on a detached HEAD — the picker then simply marks
// nothing.
func currentBranchName(ctx context.Context) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	branch, err := git.CurrentBranch(ctx, dir)
	if err != nil {
		return ""
	}
	return branch
}

// relativeAge renders a timestamp as the compact age a listing row shows
// ("3h", "2d", "5mo"). A zero or future timestamp yields "now" rather than a
// negative age.
func relativeAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	case d < 30*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + "d"
	case d < 365*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/(24*30))) + "mo"
	default:
		return strconv.Itoa(int(d.Hours()/(24*365))) + "y"
	}
}
