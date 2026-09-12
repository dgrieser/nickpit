package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// The scopes of the session picker, in the order Tab walks them: from the
// narrowest set a saved session can belong to (the branch in front of the user)
// out to everything the store holds.
const (
	scopeBranch = iota
	scopeRepo
	scopeRemote
	scopeAll
)

// describeLocalSource fills in what a local review's request does not carry but
// a later listing needs to tell its sessions apart: the project the checkout
// pushes to, and the branch that was checked out. A review of the working tree
// names no refs at all — without these its session is indistinguishable from
// every other local one. Remote (MR/PR) sources already carry both and are
// returned untouched, as is a source with no local directory.
func describeLocalSource(ctx context.Context, src session.Source) session.Source {
	if model.ReviewMode(src.Mode) != model.ModeLocal || src.RepoRoot == "" {
		return src
	}
	if src.Repo == "" {
		src.Repo = inferRepoAt(src.RepoRoot)
	}
	if src.Branch == "" {
		// A detached HEAD has no branch to record, which is what the error
		// means here; the listing then falls back to the directory.
		if branch, err := git.CurrentBranch(ctx, src.RepoRoot); err == nil {
			src.Branch = branch
		}
	}
	return src
}

// inferRepoAt is inferRepo for a directory other than the current one: the
// project the origin remote of that checkout points at, empty when there is no
// remote or no checkout.
func inferRepoAt(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return parseRepoFromRemoteURL(strings.TrimSpace(string(out)))
}

// sessionPlace is where this invocation is standing: what a saved session is
// compared against to decide which scopes it belongs to. Outside a checkout it
// is empty and only the "all" scope exists.
type sessionPlace struct {
	// here is the working tree and repository of the current directory. The
	// repository is the shared git directory, so every worktree of one clone
	// answers with the same value and their sessions fold into one scope.
	here git.Location
	// repo is the project the origin remote names, which is what a session
	// recorded on an MR (and so without any local path) can be matched on, and
	// remoteURL is that remote, whose host says which platform to ask for the
	// reviews published on the project's open requests.
	repo      string
	remoteURL string
	branch    string
	// lookup resolves a session's recorded directory to its checkout, cached
	// per directory: a store holds hundreds of sessions but only a handful of
	// distinct directories, and a directory that is gone resolves once.
	lookup func(dir string) git.Location
	// anchors are the directories that hold this repository's checkouts: the
	// clone itself, and the folder its worktrees sit in. They are what a
	// recorded directory that no longer EXISTS is judged by — git can say
	// nothing about a deleted worktree, and a removed feature branch's sessions
	// still belong to the repository they were taken in.
	anchors []string
}

// inRepo reports whether the current directory is inside a repository, which is
// what decides whether the branch and repo scopes exist at all.
func (p sessionPlace) inRepo() bool { return p.here.Root != "" || p.repo != "" }

// sessionLocation reads where the current directory stands. Every field is
// best-effort: outside a checkout, in a repository without an origin remote, or
// on a detached HEAD, what could not be read stays empty and the scopes that
// need it are dropped.
func sessionLocation(ctx context.Context) sessionPlace {
	place := sessionPlace{repo: inferRepo(), remoteURL: originRemoteURL(), branch: currentBranchName(ctx)}
	if dir, err := os.Getwd(); err == nil {
		if here, ok := git.Where(ctx, dir); ok {
			place.here = here
		}
	}
	place.lookup = newLocationCache(ctx)
	place.anchors = repoAnchors(place.here)
	return place
}

// repoAnchors are the directories a deleted checkout of this repository would
// have lived under: the clone's own directory (the parent of its git
// directory), and the folder named after the repository that holds the current
// working tree — the `<…>/nickpit/feat/x` of a worktree-per-branch layout,
// whose siblings are worktrees of the same repository. Nothing that still
// exists is judged by them: a live directory is resolved with git instead.
func repoAnchors(here git.Location) []string {
	var anchors []string
	add := func(dir string) {
		if dir == "" || dir == "." || dir == string(filepath.Separator) {
			return
		}
		if slices.Contains(anchors, dir) {
			return
		}
		anchors = append(anchors, dir)
	}
	name := repoDirName(here)
	if here.Repo != "" {
		add(filepath.Dir(here.Repo))
	}
	// Walk up from the working tree to the folder that carries the
	// repository's name; in a plain checkout that is the working tree itself.
	for dir := here.Root; dir != "" && name != ""; {
		if filepath.Base(dir) == name {
			add(dir)
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return anchors
}

// newLocationCache resolves directories to their checkout, once per directory.
func newLocationCache(ctx context.Context) func(string) git.Location {
	cache := map[string]git.Location{}
	return func(dir string) git.Location {
		if dir == "" {
			return git.Location{}
		}
		if known, seen := cache[dir]; seen {
			return known
		}
		// A failed lookup is cached too (as the zero value): a directory that is
		// gone must not cost one git call per session that recorded it.
		found, _ := git.Where(ctx, dir)
		cache[dir] = found
		return found
	}
}

// sessionOrigin is where a session was recorded, as far as it can still be
// determined: the checkout it ran in while that exists, whether it belongs to
// the repository the user is standing in, and — for a directory that is gone —
// its path below the anchor that claimed it, which is the only thing left to
// name it by.
type sessionOrigin struct {
	where git.Location
	ours  bool
	rel   string
}

// locate resolves a session's recorded directory, with the fast path first: a
// directory inside the current working tree is that working tree, without
// asking git at all.
func (p sessionPlace) locate(dir string) git.Location {
	if dir == "" {
		return git.Location{}
	}
	if p.here.Root != "" && within(p.here.Root, dir) {
		return p.here
	}
	if p.lookup == nil {
		return git.Location{}
	}
	return p.lookup(dir)
}

// origin places a session against where the user is standing. A session is of
// this repository when it names the same project, when it was recorded in a
// live checkout sharing this repository's git directory (which is what folds
// every worktree of a clone, and every subdirectory a review ran in, into one),
// or — once that directory is gone — when it lies under one of the directories
// this repository's checkouts live in.
func (p sessionPlace) origin(info session.Info) sessionOrigin {
	out := sessionOrigin{}
	if p.repo != "" && info.Source.Repo != "" && strings.EqualFold(info.Source.Repo, p.repo) {
		out.ours = true
	}
	root := info.Source.RepoRoot
	if root == "" {
		return out
	}
	out.where = p.locate(root)
	if out.where.Root != "" {
		out.ours = out.ours || (p.here.Repo != "" && out.where.Repo == p.here.Repo)
		// A working tree below one of the anchors is named by its path there —
		// "feat/x" rather than the "x" a bare directory name would give, which
		// is what tells the worktrees of a branch stack apart.
		out.rel = p.below(out.where.Root)
		return out
	}
	for _, anchor := range p.anchors {
		if !within(anchor, root) {
			continue
		}
		out.ours = true
		// The checkout is gone, so the recorded directory itself is all there
		// is to place: its path below the anchor, review subdirectory included.
		if rel, err := filepath.Rel(anchor, root); err == nil && rel != "." {
			out.rel = filepath.ToSlash(rel)
		}
		break
	}
	return out
}

// below is a directory's path under the anchor that holds it, empty when no
// anchor does or when the directory IS the anchor (a clone checked out under
// its own name has nothing to add to it).
func (p sessionPlace) below(dir string) string {
	for _, anchor := range p.anchors {
		if !within(anchor, dir) {
			continue
		}
		if rel, err := filepath.Rel(anchor, dir); err == nil && rel != "." {
			return filepath.ToSlash(rel)
		}
	}
	return ""
}

// matchesRepo reports whether a session was recorded in the repository the user
// is standing in.
func (p sessionPlace) matchesRepo(info session.Info) bool {
	return p.origin(info).ours
}

// within reports whether path is parent or lies inside it.
func within(parent, path string) bool {
	parent, path = filepath.Clean(parent), filepath.Clean(path)
	return path == parent || strings.HasPrefix(path, parent+string(filepath.Separator))
}

// matchesBranch reports whether a session reviewed the checked-out branch. It
// is only asked of sessions that already belong to the repository. A session
// saved before the branch was recorded carries neither a branch nor a head ref
// and stays out of the branch scope rather than being guessed into it.
func (p sessionPlace) matchesBranch(info session.Info) bool {
	if p.branch == "" {
		return false
	}
	if info.Source.Branch != "" {
		return info.Source.Branch == p.branch
	}
	return refIsBranch(info.Source.HeadRef, p.branch)
}

// refIsBranch reports whether ref names branch. A head ref is recorded as the
// local branch, so the comparison is exact once the fully-qualified forms are
// unwrapped: "refs/heads/x" and "refs/remotes/origin/x" are branch x, while a
// bare "feat/x" is the branch of that name and not branch "x" on a remote
// called "feat".
func refIsBranch(ref, branch string) bool {
	if ref == "" || branch == "" {
		return false
	}
	if ref == branch {
		return true
	}
	if rest, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return rest == branch
	}
	if rest, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
		_, after, found := strings.Cut(rest, "/")
		return found && after == branch
	}
	return false
}

// pickSession asks which saved session to use. The scopes are the sessions of
// the checked-out branch, of the repository, and all of them; in a repository
// the list opens on the repository's sessions, which is the set a person in a
// checkout is usually choosing from, with the branch one scope to the left and
// everything one to the right. Outside a repository only the full list exists
// and the switcher is not drawn at all.
//
// infos are the store's sessions, newest first; the caller has already dropped
// the ones it could not use.
func (a *app) pickSession(place sessionPlace, infos []session.Info, title string,
	remote *remoteFinder, resume sessionPickState) (sessionChoice, sessionPickState, error) {
	// title names what one row IS, not what the list holds: the header spells
	// the selected session out, and which scope is on screen is said by the
	// status line under the rows.
	if len(infos) == 0 && !remote.available() {
		return sessionChoice{}, resume, fmt.Errorf("no saved sessions")
	}
	views, index := sessionViews(infos, place, remote)
	if resume.view >= 0 && resume.view < len(views) {
		// Coming back from the action prompt lands where the list was left.
		index = resume.view
	}
	view, row, err := a.selectView(pick.Options{
		Title:      title,
		Views:      views,
		View:       index,
		Initial:    indexOfSession(views[index], resume.id),
		CellStyles: sessionCellStyles,
		ColumnKinds: []pick.ColumnKind{
			pick.KindPlain, pick.KindPlain, pick.KindRef, pick.KindRefNote,
		},
		// What was reviewed and how it ended level down together; the
		// repository gives way first — it is the same value on every row of the
		// branch and repo scopes.
		ColumnPriority: []int{
			pick.PriorityNormal, pick.PriorityNormal, pick.PriorityLow,
			pick.PriorityNormal, pick.PriorityNormal,
		},
	})
	if err != nil {
		return sessionChoice{}, resume, err
	}
	if view < 0 || view >= len(views) || row < 0 || row >= len(views[view].Items) {
		return sessionChoice{}, resume, fmt.Errorf("no session selected")
	}
	chosen := sessionOfRow(views[view], infos, remote, row)
	if chosen.info.ID == "" {
		return sessionChoice{}, resume, fmt.Errorf("no session selected")
	}
	return chosen, sessionPickState{view: view, id: chosen.info.ID}, nil
}

// sessionChoice is what a row turned out to be: a session saved on this
// machine, or a review published on an open request, which the actions read
// from the server instead of from the store.
type sessionChoice struct {
	info   session.Info
	remote *remoteReview
}

// saved reports whether the choice is a session in the local store.
func (c sessionChoice) saved() bool { return c.remote == nil }

// sessionPickState is where the picker stood when it handed a session over, so
// going back from the action prompt returns to the same scope and row instead
// of the top of the default one.
type sessionPickState struct {
	view int
	id   string
}

// newSessionPickState starts a picker that has nothing to return to.
func newSessionPickState() sessionPickState { return sessionPickState{view: -1} }

// indexOfSession finds a session's row in a scope, -1 when the scope does not
// hold it (the picker then opens on its first row).
func indexOfSession(view pick.View, id string) int {
	if id == "" {
		return -1
	}
	for i, item := range view.Items {
		if item.Key == id {
			return i
		}
	}
	return -1
}

// sessionAction is what to do with the session that was picked.
type sessionAction int

const (
	sessionActionPrint sessionAction = iota
	sessionActionCopy
	sessionActionChat
)

// sessionActions are the prompt's rows, in the order they are offered: what the
// command has always done first, then the two that carry the review somewhere
// else.
var sessionActions = []struct {
	action sessionAction
	label  string
	detail string
}{
	{sessionActionPrint, "Print", "the review to stdout"},
	{sessionActionCopy, "Copy", "the review to the clipboard"},
	{sessionActionChat, "Chat", "about the review, resuming this session"},
}

// pickSessionAction asks what to do with the session that was chosen. Esc and
// backspace return pick.ErrAborted, which the caller reads as "back to the
// list" rather than as a failure.
func (a *app) pickSessionAction(place sessionPlace, choice sessionChoice) (sessionAction, error) {
	info := choice.info
	details := sessionDetail(info)
	items := make([]pick.Item, 0, len(sessionActions))
	for _, action := range sessionActions {
		items = append(items, pick.Item{
			Cells:   []string{action.label, action.detail},
			Details: details,
		})
	}
	index, err := a.selectOne(pick.Options{
		Title:              "Session",
		Items:              items,
		CellStyles:         []string{pick.StyleIdentifier, pick.StyleAge},
		Hint:               "↑/↓ move · Enter select · Esc/Backspace back",
		DismissOnBackspace: true,
	})
	if err != nil {
		return 0, err
	}
	if index < 0 || index >= len(sessionActions) {
		return 0, fmt.Errorf("no action selected")
	}
	chosen := sessionActions[index]
	// The whole id, not the shortened one the column shows: this line is what a
	// later `--session` is copied from, and it names the action it stands for.
	a.printSelection("Session ", info.ID, pick.StyleHash,
		sessionSuffix(place, info)+" · "+strings.ToLower(chosen.label))
	return chosen.action, nil
}

// sessionCellStyles colours the picker's columns: the marker, the session id,
// the repository and what was reviewed (both painted as refs, like a branch
// name in the branch picker), how the review ended, and its age. The last three
// carry a colour per row — the kind of review, the verdict, the freshness —
// which is what these column defaults give way to.
var sessionCellStyles = []string{
	pick.StyleMark, pick.StyleHash, "", pick.StyleIdentifier, pick.StyleText, pick.StyleAge,
}

// verdictStyle colours the outcome by the verdict alone — how many findings a
// review holds says nothing about whether it passed. The two verdicts wear the
// green and red the review output badges them in; a verdict that is neither is
// a caveat, and a review that recorded none recedes into grey.
func verdictStyle(info session.Info) string {
	if !info.HasResult {
		return pick.StyleAge
	}
	switch sessionVerdict(info.Verdict) {
	case "Correct":
		return pick.StyleFresh
	case "Incorrect":
		return pick.StyleError
	case "No verdict":
		return pick.StyleAge
	}
	return pick.StyleCaveat
}

// sourceStyle colours what was reviewed by the kind of review it was, so an MR,
// a branch and a working-tree review are told apart before the value is read.
func sourceStyle(info session.Info) string {
	return sessionKindStyles[sessionKindOf(info)]
}

// sessionOfRow resolves a chosen row back to what it stands for: the item keys
// are session and review ids, so a row found in one scope is the same thing in
// every other, whether it came from the store or from an open request.
func sessionOfRow(view pick.View, infos []session.Info, remote *remoteFinder, row int) sessionChoice {
	id := view.Items[row].Key
	for _, info := range infos {
		if info.ID == id {
			return sessionChoice{info: info}
		}
	}
	if review := remote.byID(id); review != nil {
		return sessionChoice{info: review.info, remote: review}
	}
	return sessionChoice{}
}

// sessionViews builds the scopes and says which one to open on: the
// repository's sessions in a checkout, the full list everywhere else — and the
// full list too when the repository has nothing saved, so the prompt never
// opens on an empty scope while sessions exist elsewhere.
func sessionViews(infos []session.Info, place sessionPlace, remote *remoteFinder) ([]pick.View, int) {
	all := pick.View{
		Label: "all",
		Items: sessionItems(infos, place),
		Empty: "no saved sessions",
	}
	if !place.inRepo() {
		return []pick.View{all}, 0
	}
	var branch, repo []session.Info
	for _, info := range infos {
		if !place.matchesRepo(info) {
			continue
		}
		repo = append(repo, info)
		if place.matchesBranch(info) {
			branch = append(branch, info)
		}
	}
	views := []pick.View{
		{
			Label: "branch",
			Items: sessionItems(branch, place),
			Empty: "no sessions for " + textsan.StripControl(firstNonEmpty(place.branch, "this branch")) +
				" (older sessions recorded no branch and list under the repository)",
		},
		{
			Label: "repository",
			Items: sessionItems(repo, place),
			Empty: "no sessions for " + textsan.StripControl(place.name()),
		},
		remoteView(place, remote),
		all,
	}
	if len(repo) == 0 {
		return views, scopeAll
	}
	return views, scopeRepo
}

// remoteView is the scope of what lives on the server rather than here: the
// reviews NickPit published on the project's open merge or pull requests. It is
// its own scope because those are not sessions — nothing of them is saved on
// this machine — and because reading them costs a round trip nobody should pay
// for while looking at their own sessions. The rows of the checked-out branch
// lead, since that is the request being worked on; the rest follow newest
// first.
func remoteView(place sessionPlace, remote *remoteFinder) pick.View {
	view := pick.View{
		Label:   "remote",
		Loading: "reading published reviews…",
		Empty: "no reviews published on " + remote.scopeNoun() + " " +
			textsan.StripControl(place.name()),
	}
	if !remote.available() {
		view.Empty = "no project to ask: this scope lists the reviews published on " + remote.scopeNoun()
		return view
	}
	view.Load = func() ([]pick.Item, error) {
		reviews, err := remote.find(context.Background())
		sortRemoteReviews(reviews, place.branch)
		rows := make([]session.Info, 0, len(reviews))
		for _, review := range reviews {
			rows = append(rows, review.info)
		}
		return sessionItems(rows, place), err
	}
	return view
}

// name is what to call the repository the user is standing in: the project of
// its remote, else the directory the repository lives in.
func (p sessionPlace) name() string {
	if p.repo != "" {
		return p.repo
	}
	return repoDirName(p.here)
}

// repoDirName names a repository by its directory: the folder holding the git
// directory ("/src/nickpit/.git" is nickpit), which is the clone's own name and
// is the same seen from any of its worktrees. The working tree is the fallback
// for a layout that keeps the git directory elsewhere.
func repoDirName(where git.Location) string {
	if where.Repo != "" {
		if parent := filepath.Dir(where.Repo); parent != "" && parent != "." && parent != string(filepath.Separator) {
			return filepath.Base(parent)
		}
	}
	if where.Root != "" {
		return filepath.Base(where.Root)
	}
	return ""
}

// sessionItems renders one scope's rows.
func sessionItems(infos []session.Info, place sessionPlace) []pick.Item {
	items := make([]pick.Item, 0, len(infos))
	for _, info := range infos {
		mark := ""
		if place.matchesBranch(info) {
			mark = checkedOutMark
		}
		origin := place.origin(info)
		repo := textsan.StripControl(sessionRepoLabel(place, info, origin))
		shownRepo := fitRepoLabel(repo)
		source := textsan.StripControl(sessionSourceLabel(place, info, origin))
		shown := fitSourceLabel(source)
		items = append(items, pick.Item{
			// The id is what every scope knows this row by, so the cursor stays
			// on the same session when the scope widens or narrows.
			Key: info.ID,
			// The columns show the id shortened and the directory not at all;
			// the header keeps both.
			Details: sessionDetail(info),
			Cells: []string{
				mark,
				shortSessionID(info.ID),
				shownRepo,
				shown,
				sessionOutcome(info),
				relativeAge(info.UpdatedAt),
			},
			// What a row can be typed at: the project and the directory it
			// belongs to (both in full, whatever the columns had to shorten),
			// what was reviewed, the verdict word and the number of findings —
			// so "incorrect" narrows the list to the reviews that found the
			// patch wrong, without the finding text dragging in the ones that
			// merely mention it. The word "findings" itself is left out: every
			// row carries it, so typing it would narrow nothing, while a branch
			// or project of that name still matches. The id answers only to a
			// term long enough to be one.
			MatchFields: []pick.MatchField{
				{Text: strings.Join([]string{
					repo, source, info.Source.RepoRoot,
					sessionVerdictWord(info), sessionFindingsTerm(info),
				}, " ")},
				{Text: info.ID, MinTerm: minSessionIDTerm},
			},
			CellStyles: rowStyles(6, map[int]string{
				3: sourceStyle(info),
				4: verdictStyle(info),
				5: ageStyle(info.UpdatedAt),
			}),
		})
	}
	return items
}

// minSessionIDTerm is the shortest filter term that searches a session id. A
// uuid is opaque: a term of one to three characters hits a third of the store
// by accident and buries what the same term finds in a project, a branch or a
// finding count. Four characters are already a deliberate id.
const minSessionIDTerm = 4

// shortSessionIDLen is how much of a session id the picker shows: enough to be
// unique among a person's sessions, short enough to leave the row to what was
// reviewed. The header carries the full id.
const shortSessionIDLen = 8

func shortSessionID(id string) string {
	if len(id) <= shortSessionIDLen {
		return id
	}
	return id[:shortSessionIDLen]
}

// sessionRepoLabel names the project a session belongs to: the one it recorded,
// the project of the current directory when the session turns out to be of that
// same repository (which is what gives a session saved before the project was
// recorded its real name), else the directory of the repository it ran in.
func sessionRepoLabel(place sessionPlace, info session.Info, origin sessionOrigin) string {
	if info.Source.Repo != "" {
		return info.Source.Repo
	}
	if place.repo != "" && origin.ours {
		return place.repo
	}
	if name := repoDirName(origin.where); name != "" {
		return name
	}
	if info.Source.RepoRoot != "" {
		return filepath.Base(info.Source.RepoRoot)
	}
	return ""
}

// uncommittedHeadRef is the head a review of the working tree records in its
// context: there is no commit to name, so the ref says what was reviewed
// instead. Its base is then the branch the review ran on.
const uncommittedHeadRef = "uncommitted"

// uncommittedNote qualifies what a working-tree review looked at. It is never
// shortened away: what was reviewed reads differently without it.
const uncommittedNote = " (uncommitted)"

// sessionKind is what kind of review a session holds. It decides both how the
// "what was reviewed" column reads and what colour it wears, so the kinds are
// told apart at a glance without reading the value.
type sessionKind int

const (
	// kindUnknownReview is a review that recorded neither a request, refs nor a
	// branch — all that is left of it is the directory it ran in.
	kindUnknownReview sessionKind = iota
	kindGitLabRequest
	kindGitHubRequest
	kindBranchReview
	kindCommitReview
	kindWorkingTree
)

// sessionKindStyles colour the column per kind, from the 256-colour message
// palette the picker draws in (tools/print_colors.sh): the two request kinds in
// hues of their own, a branch in the aqua green a branch picker gives the
// default ref, a commit range in the green a SHA wears, the working tree in
// turquoise, and a review that can only be placed by its directory in the grey
// of a path.
var sessionKindStyles = map[sessionKind]string{
	kindGitLabRequest: "38;5;216",           // apricot
	kindGitHubRequest: "38;5;105",           // purple-blue
	kindBranchReview:  pick.StyleDefaultRef, // aqua green
	kindCommitReview:  pick.StyleHash,       // hash green
	kindWorkingTree:   "38;5;116",           // turquoise
	kindUnknownReview: pick.StyleAge,        // grey
}

// sessionKindOf classifies a session. The submode says what a review was of
// where it recorded one; a session saved before that is read from what it
// carries — two commit SHAs are a range, a branch is a branch review, and a
// context whose head is not a commit at all was of the working tree.
func sessionKindOf(info session.Info) sessionKind {
	src := info.Source
	switch model.ReviewMode(src.Mode) {
	case model.ModeGitLab:
		if src.Identifier > 0 {
			return kindGitLabRequest
		}
	case model.ModeGitHub:
		if src.Identifier > 0 {
			return kindGitHubRequest
		}
	}
	if src.Submode == uncommittedHeadRef || info.ContextHeadRef == uncommittedHeadRef {
		return kindWorkingTree
	}
	switch src.Submode {
	case "commits":
		return kindCommitReview
	case "branch":
		return kindBranchReview
	}
	base, head := sessionRangeRefs(info)
	if isCommitSHA(base) && isCommitSHA(head) {
		return kindCommitReview
	}
	if sessionBranch(info) != "" || head != "" || base != "" {
		return kindBranchReview
	}
	return kindUnknownReview
}

// sessionSourceLabel names what a session reviewed: the request on a remote
// review, the commit range or the branch on a local one — with "(uncommitted)"
// appended where the review was of the working tree rather than of commits —
// and, for a local review that recorded none of those, the working tree it ran
// in, so sessions from different worktrees of one repository are still told
// apart.
func sessionSourceLabel(place sessionPlace, info session.Info, origin sessionOrigin) string {
	src := info.Source
	switch sessionKindOf(info) {
	case kindGitLabRequest:
		return "GitLab MR !" + strconv.Itoa(src.Identifier)
	case kindGitHubRequest:
		return "GitHub PR #" + strconv.Itoa(src.Identifier)
	case kindWorkingTree:
		where := firstNonEmpty(sessionBranch(info), sessionWhereLabel(src, origin))
		if where == "" {
			where = unknownSourceLabel
		}
		return where + uncommittedNote
	case kindCommitReview:
		if span := sessionRangeLabel(info); span != "" {
			return span
		}
	case kindBranchReview:
		// A branch review names both ends of what it compared; a review that
		// recorded only the branch it ran on names that.
		if span := sessionRangeLabel(info); span != "" {
			return span
		}
		if branch := sessionBranch(info); branch != "" {
			return branch
		}
	}
	return sessionUnknownLabel(place, info, origin)
}

// unknownSourceLabel marks a review that recorded nothing about what it looked
// at. Where its directory adds something the repository column does not already
// say, it follows the marker; where it is just the repository's own folder
// again, the marker stands alone.
const unknownSourceLabel = "<unknown>"

func sessionUnknownLabel(place sessionPlace, info session.Info, origin sessionOrigin) string {
	where := sessionWhereLabel(info.Source, origin)
	if where == "" || where == sessionRepoFolder(place, origin) {
		return unknownSourceLabel
	}
	return unknownSourceLabel + " " + where
}

// sessionRepoFolder is the folder the repository itself is named by, which is
// what a directory has to differ from to be worth showing.
func sessionRepoFolder(place sessionPlace, origin sessionOrigin) string {
	if name := repoDirName(origin.where); name != "" {
		return name
	}
	return repoDirName(place.here)
}

// maxSourceWidth caps what the "what was reviewed" column may claim, in
// terminal cells, before the picker's own fitting even sees it: a branch stack
// produces names long enough to push every other column off a wide terminal,
// and past this much of one nothing more is learned. It covers the whole cell,
// qualifier included — a column wider than its widest cell would leave the next
// one stranded behind the padding of every row.
const maxSourceWidth = 40

// fitSourceLabel shortens a label to maxSourceWidth by its parts rather than
// from the end: the qualifier of a working-tree review stays whole and is paid
// for out of the branch's width, and the two ends of a range share what is
// left — each keeps what it needs while it is the shorter one, and two long
// names level down together instead of one of them surviving intact next to a
// stub.
func fitSourceLabel(label string) string {
	note := ""
	if value, ok := strings.CutSuffix(label, uncommittedNote); ok {
		label, note = value, uncommittedNote
	}
	budget := maxSourceWidth - pick.DisplayWidth(note)
	if pick.DisplayWidth(label) <= budget {
		return label + note
	}
	if budget < minSourceWidth {
		// The qualifier alone all but fills the cap; keep a readable stub of
		// what it qualifies rather than cutting the name away entirely.
		budget = minSourceWidth
	}
	base, head, isRange := strings.Cut(label, "..")
	if !isRange {
		return pick.Truncate(label, budget) + note
	}
	baseWidth, headWidth := shareWidth(pick.DisplayWidth(base), pick.DisplayWidth(head), budget-len(".."))
	return pick.Truncate(base, baseWidth) + ".." + pick.Truncate(head, headWidth) + note
}

// minSourceWidth is how much of the value itself survives whatever else the
// cell has to carry.
const minSourceWidth = 12

// maxRepoWidth caps the project column. A nested group path is longer than what
// it adds: which project a session belongs to is carried by the name at its
// end, and the group at its front says which one of those it is.
const maxRepoWidth = 24

// fitRepoLabel shortens a project by dropping what a reader needs least, in
// order: the groups in the middle of the path ("asylum/…/archiefmeester"), then
// the one at its front ("…/archiefmeester"), and only then the name itself,
// which is cut in its middle ("…/arc…ter") because a project's name is told
// apart by both of its ends.
func fitRepoLabel(label string) string {
	if pick.DisplayWidth(label) <= maxRepoWidth {
		return label
	}
	parts := strings.Split(label, "/")
	name := parts[len(parts)-1]
	if len(parts) > 2 {
		if folded := parts[0] + "/…/" + name; pick.DisplayWidth(folded) <= maxRepoWidth {
			return folded
		}
	}
	if len(parts) > 1 {
		if tail := "…/" + name; pick.DisplayWidth(tail) <= maxRepoWidth {
			return tail
		}
		return "…/" + pick.TruncateMiddle(name, maxRepoWidth-pick.DisplayWidth("…/"))
	}
	return pick.TruncateMiddle(name, maxRepoWidth)
}

// shareWidth splits a budget between the two ends of a range: an end that fits
// in its half keeps its full width and leaves the rest to the other, and two
// ends that both overrun split the budget evenly, the odd cell going to the
// head — the side that says what was reviewed rather than what it was compared
// against.
func shareWidth(base, head, budget int) (int, int) {
	if budget <= 0 {
		return 0, 0
	}
	if base+head <= budget {
		return base, head
	}
	if half := budget / 2; base <= half {
		return base, budget - base
	} else if head <= half {
		return budget - head, head
	}
	return budget / 2, budget - budget/2
}

// sessionWhereLabel places a review that named nothing: where its checkout sits
// among this repository's others — in a layout of one worktree per branch that
// is as close to the branch as such a session gets, and all that is left of a
// worktree that has since been removed.
func sessionWhereLabel(src session.Source, origin sessionOrigin) string {
	switch {
	case origin.rel != "":
		return origin.rel
	case origin.where.Root != "":
		return filepath.Base(origin.where.Root)
	case src.RepoRoot != "":
		return filepath.Base(src.RepoRoot)
	}
	return ""
}

// sessionRangeRefs are the two ends of what a review diffed: the refs the
// source recorded, falling back to the cached context, which is where they
// survive for a session whose source recorded none.
func sessionRangeRefs(info session.Info) (string, string) {
	return firstNonEmpty(info.Source.BaseRef, info.ContextBaseRef),
		firstNonEmpty(info.Source.HeadRef, info.ContextHeadRef)
}

// sessionRangeLabel writes those two ends the way a range is written, with raw
// SHAs abbreviated as git abbreviates them.
func sessionRangeLabel(info session.Info) string {
	base, head := sessionRangeRefs(info)
	base, head = shortRev(base), shortRev(head)
	switch {
	case base != "" && head != "":
		return base + ".." + head
	case head != "":
		return head
	}
	return base
}

// sessionBranch is the branch a local review ran on, through the three places
// it can be found: the branch the session recorded, its head ref, and — for a
// review of the working tree, whose head is not a ref at all — the base ref of
// the cached context, which is that branch.
func sessionBranch(info session.Info) string {
	if info.Source.Branch != "" {
		return info.Source.Branch
	}
	if head := info.Source.HeadRef; head != "" && head != uncommittedHeadRef {
		// A range names two ends; only a lone head ref is a branch.
		if info.Source.BaseRef == "" {
			return head
		}
		return ""
	}
	if info.ContextHeadRef == uncommittedHeadRef {
		return info.ContextBaseRef
	}
	return ""
}

// isCommitSHA reports whether a ref is a raw commit id rather than a name.
func isCommitSHA(ref string) bool {
	return len(ref) == 40 && strings.TrimLeft(ref, "0123456789abcdefABCDEF") == ""
}

// sessionOutcome is how the review ended: its verdict and how many findings it
// holds — "Correct, 0 findings", "Incorrect, 3 findings".
func sessionOutcome(info session.Info) string {
	if !info.HasResult {
		return sessionVerdictWord(info)
	}
	return sessionVerdictWord(info) + ", " + sessionCountLabel(info.Findings)
}

// sessionVerdictWord is the outcome column without its finding count: the part
// a filter is typed at.
func sessionVerdictWord(info session.Info) string {
	if !info.HasResult {
		return "no review"
	}
	return sessionVerdict(info.Verdict)
}

// sessionFindingsTerm is the finding count as a filter term — the number alone,
// since the word beside it is on every row. A session holding no review counts
// nothing.
func sessionFindingsTerm(info session.Info) string {
	if !info.HasResult {
		return ""
	}
	return strconv.Itoa(info.Findings)
}

// sessionVerdict names the overall verdict for a listing. The verdict is free
// text a model wrote ("patch is correct"), so it is read the way the rendered
// output reads it — "incorrect" beats "correct", since the first contains the
// second — and anything that is neither is shown as it stands rather than
// collapsed into a verdict it never gave. A review whose workflow has no
// verdict step records none at all.
func sessionVerdict(verdict string) string {
	verdict = textsan.StripControl(strings.TrimSpace(verdict))
	switch {
	case verdict == "":
		return "No verdict"
	case strings.Contains(strings.ToLower(verdict), "incorrect"):
		return "Incorrect"
	case strings.Contains(strings.ToLower(verdict), "correct"):
		return "Correct"
	}
	return strings.ToUpper(verdict[:1]) + verdict[1:]
}

func sessionCountLabel(findings int) string {
	if findings == 1 {
		return "1 finding"
	}
	return strconv.Itoa(findings) + " findings"
}

// sessionDetail is what the header says about the selected row: the session id
// `--session` takes and the directory the review ran in, each in the colour its
// own column wears — the id in the id column's green, the directory in the
// project column's plain tone with its separators faded.
func sessionDetail(info session.Info) []pick.Field {
	return []pick.Field{
		{Text: info.ID, Style: pick.StyleHash},
		{Text: textsan.StripControl(info.Source.RepoRoot), Kind: pick.KindRef},
	}
}

// sessionSuffix is the aside the selection confirmation ends on: what the
// chosen session reviewed, so the short id is not the only thing named.
func sessionSuffix(place sessionPlace, info session.Info) string {
	origin := place.origin(info)
	label := sessionSourceLabel(place, info, origin)
	if repo := sessionRepoLabel(place, info, origin); repo != "" {
		if label == "" {
			label = repo
		} else {
			label = repo + " " + label
		}
	}
	if label == "" {
		return ""
	}
	return " · " + textsan.StripControl(label)
}

// shortRev abbreviates a raw commit SHA the way git does, leaving every other
// ref (a branch name, a tag) as it is.
func shortRev(ref string) string {
	if len(ref) != 40 || strings.TrimLeft(ref, "0123456789abcdefABCDEF") != "" {
		return ref
	}
	return ref[:8]
}
