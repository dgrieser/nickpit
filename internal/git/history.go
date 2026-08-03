package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

// History exposes commit history of a checkout: a filtered commit listing
// (metadata plus changed files, no patch) and per-commit diffs. Both agent
// tools (git_log/git_show) and the `nickpit inspect log|show` commands run
// through it so they cannot drift apart.
type History interface {
	Log(ctx context.Context, repoRoot string, opts LogOptions) (*LogResult, error)
	Show(ctx context.Context, repoRoot string, opts ShowOptions) (*ShowResult, error)
}

// LogOptions filters a commit listing. Every field is optional; the zero value
// lists the newest defaultLogLimit commits reachable from HEAD.
type LogOptions struct {
	// Commit is a revision to list history from (default HEAD) or a range
	// ("a..b"/"a...b"). SHAs may be abbreviated to any length.
	Commit string
	// Since and Until bound the commit date (git's committer timestamp, which
	// is what --since/--until filter on — a rebased or cherry-picked commit is
	// selected by when it was last rewritten, not when it was authored). Any
	// value git understands works (RFC3339, "2026-01-02", "2 weeks ago").
	Since string
	Until string
	// Author matches the author name/email.
	Author string
	// Paths limits the listing to commits touching at least one of them.
	Paths []string
	// Message matches the commit message. Literal by default; MessageRegex
	// switches Message (and Author) to git's regex matching.
	Message       string
	MessageRegex  bool
	CaseSensitive bool
	// Limit caps the number of commits returned (default defaultLogLimit,
	// clamped to maxLogLimit).
	Limit int
}

// ShowOptions selects the commits whose diffs are returned. Commit is required.
type ShowOptions struct {
	// Commit is a single revision or a range ("a..b"/"a...b"); SHAs may be
	// abbreviated to any length.
	Commit string
	// To turns Commit into a range without range syntax (Commit..To).
	To string
	// Paths limits each diff to the given repo-relative paths.
	Paths []string
	// MaxCommits caps how many commits of a range are returned (default
	// defaultShowCommits, clamped to maxShowCommits).
	MaxCommits int
	// Format selects the diff shape, exactly like the review prompt payload:
	// DiffFormatGit yields DiffFiles, DiffFormatGitJson yields DiffHunks.
	Format model.DiffFormat
}

// CommitFile is one changed file of a commit.
type CommitFile struct {
	Path      string           `json:"path"`
	OldPath   string           `json:"old_path,omitempty"`
	Status    model.FileStatus `json:"status"`
	Additions int              `json:"additions"`
	Deletions int              `json:"deletions"`
	Binary    bool             `json:"binary,omitempty"`
}

// CommitEntry is a commit's metadata plus its changed files, without any patch
// content.
type CommitEntry struct {
	SHA         string `json:"sha"`
	ShortSHA    string `json:"short_sha"`
	Author      string `json:"author"`
	AuthorEmail string `json:"author_email,omitempty"`
	// Date is the author date; CommitDate is the committer date, which is what
	// the Since/Until filters select on. They differ for rebased, amended and
	// cherry-picked commits.
	Date       time.Time    `json:"date"`
	CommitDate time.Time    `json:"commit_date,omitzero"`
	Subject    string       `json:"subject"`
	Body       string       `json:"body,omitempty"`
	Parents    []string     `json:"parents,omitempty"`
	IsMerge    bool         `json:"is_merge,omitempty"`
	Additions  int          `json:"additions"`
	Deletions  int          `json:"deletions"`
	Files      []CommitFile `json:"files"`
}

// LogResult is a commit listing. Truncated reports that more commits matched
// than Limit allowed; Shallow reports that the checkout's history is cut off.
type LogResult struct {
	Range       string        `json:"range"`
	CommitCount int           `json:"commit_count"`
	Commits     []CommitEntry `json:"commits"`
	Truncated   bool          `json:"truncated,omitempty"`
	Shallow     bool          `json:"shallow,omitempty"`
	Note        string        `json:"note,omitempty"`
}

// CommitDiff is one commit's metadata plus its diff. DiffMode records how the
// patch was produced: "single" for an ordinary commit, "combined" for a merge
// whose combined diff carries hunks, "first-parent" for a merge whose combined
// diff was empty and that was therefore diffed against its first parent.
type CommitDiff struct {
	CommitEntry
	DiffMode  string           `json:"diff_mode"`
	DiffFiles []model.DiffFile `json:"diff_files,omitempty"`
	DiffHunks []model.DiffHunk `json:"diff_hunks,omitempty"`
	Truncated bool             `json:"truncated,omitempty"`
	Note      string           `json:"note,omitempty"`
}

// ShowResult holds one diff per requested commit, newest first.
type ShowResult struct {
	Range       string       `json:"range"`
	DiffFormat  string       `json:"diff_format"`
	CommitCount int          `json:"commit_count"`
	Commits     []CommitDiff `json:"commits"`
	Truncated   bool         `json:"truncated,omitempty"`
	Shallow     bool         `json:"shallow,omitempty"`
	Note        string       `json:"note,omitempty"`
}

const (
	// defaultLogLimit/maxLogLimit bound a commit listing so a broad filter
	// cannot flood an agent's context.
	defaultLogLimit = 20
	maxLogLimit     = 200
	// defaultShowCommits/maxShowCommits bound how many patches one git_show
	// call returns.
	defaultShowCommits = 10
	maxShowCommits     = 50
	// deepenCommits is how far a shallow checkout is deepened on first use.
	deepenCommits = 200
	// deepenTimeout bounds the network fetch so a slow remote cannot stall an
	// agent loop.
	deepenTimeout = 60 * time.Second
	// maxCommitPatchBytes drops an individual commit's patch bodies once it
	// grows past this; metadata and the changed-file list are kept.
	maxCommitPatchBytes = 512 << 10
	// maxShowPatchBytes bounds the total patch bytes of one git_show call.
	maxShowPatchBytes = 2 << 20
	// maxAmbiguousCandidates bounds how many candidates an ambiguous-prefix
	// error lists.
	maxAmbiguousCandidates = 5

	// nulSeparator delimits every field and file entry git writes in its
	// machine-readable -z output. It is the one byte that cannot appear in a
	// commit message, an author name or a path, which is what makes the framing
	// unambiguous.
	nulSeparator = "\x00"
	// commitFields is the number of %-placeholders in commitFormat.
	commitFields = 9
)

// commitFormat renders one commit's metadata as NUL-delimited fields. Under -z
// git terminates the record with a NUL of its own, so the file entries of
// `git log` and the patch of `git show` start at a deterministic offset without
// a separator that could also occur inside a message or a path.
const commitFormat = "%H%x00%h%x00%an%x00%ae%x00%aI%x00%cI%x00%P%x00%s%x00%b"

// ErrNotAGitRepo reports that the checkout has no git metadata, so no history
// can be read from it.
var ErrNotAGitRepo = errors.New("git: not a git repository")

// numstatEntry matches one `--numstat -z` record: added and deleted line
// counts ("-" for binary files) followed by the path, which is empty for a
// rename/copy whose old and new paths follow as separate NUL-terminated tokens.
// (?s) is required because -z does not quote paths and a path may contain a
// newline, which would otherwise make the entry unrecognizable and desync the
// surrounding record scan.
var numstatEntry = regexp.MustCompile(`(?s)^(\d+|-)\t(\d+|-)\t(.*)$`)

var hexRevision = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// HistoryAuth carries the credentials used to deepen a shallow checkout of a
// private repository, together with the hosts they belong to. A token is only
// ever sent to its own provider's configured host: an origin URL is attacker
// controlled in a fork PR/MR, so matching it loosely (any host merely
// containing "github") would hand the token to https://github.attacker.example.
type HistoryAuth struct {
	GitHubToken string
	GitLabToken string
	// GitLabBaseURL is the configured GitLab API base URL; its host is the only
	// one the GitLab token is sent to. Empty means gitlab.com.
	GitLabBaseURL string
}

// defaultGitHubHost is the only host the GitHub token is sent to. GitHub
// Enterprise hosts are not configurable anywhere in nickpit today, so there is
// no other trusted host to derive.
const defaultGitHubHost = "github.com"

// defaultGitLabHost is used when no GitLab base URL is configured.
const defaultGitLabHost = "gitlab.com"

// ExecHistory reads history by running git. It is safe for concurrent use:
// tool calls execute in parallel, and the one-time deepen of a shallow
// checkout is single-flighted per repository root.
type ExecHistory struct {
	newRunner func(repoRoot string) Runner
	auth      HistoryAuth

	locks  keyedMutex
	mu     sync.Mutex
	deepen map[string]deepenState
}

// deepenState is the memoized outcome of the shallow-history check for one
// repository root.
type deepenState struct {
	shallow bool
	note    string
}

func NewExecHistory(auth HistoryAuth) *ExecHistory {
	return &ExecHistory{
		newRunner: func(repoRoot string) Runner {
			return ExecRunner{RepoRoot: repoRoot}
		},
		auth:   auth,
		deepen: make(map[string]deepenState),
	}
}

func (h *ExecHistory) Log(ctx context.Context, repoRoot string, opts LogOptions) (*LogResult, error) {
	runner := h.newRunner(repoRoot)
	if err := ensureGitRepo(ctx, runner); err != nil {
		return nil, err
	}
	state := h.ensureHistory(ctx, repoRoot, runner)

	limit := clampLimit(opts.Limit, defaultLogLimit, maxLogLimit)
	revision, err := h.resolveRevisionArg(ctx, runner, opts.Commit, "")
	if err != nil {
		return nil, err
	}
	paths, err := sanitizePaths(opts.Paths)
	if err != nil {
		return nil, err
	}
	for field, value := range map[string]string{"since": opts.Since, "until": opts.Until, "author": opts.Author, "message": opts.Message} {
		if err := validateGitArgValue(field, value); err != nil {
			return nil, err
		}
	}

	// Ask for one commit more than requested so Truncated reports whether the
	// limit actually cut the result instead of guessing from a full page.
	args := []string{"log", "--no-color", fmt.Sprintf("--max-count=%d", limit+1), "--format=" + commitFormat,
		"--raw", "--numstat", "-z", "--find-renames"}
	// --diff-merges=first-parent gives merge commits a changed-file list
	// without restricting traversal the way --first-parent would (which would
	// silently hide every commit merged in from a branch).
	args = append(args, "--diff-merges=first-parent")
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}
	if opts.Message != "" {
		args = append(args, "--grep="+opts.Message)
	}
	// The match mode applies to every limiting pattern git received, so it must
	// be selected whenever one is set — an author-only filter would otherwise
	// fall through to git's default basic-regex matching, where "Grie[s]er"
	// silently behaves as a character class instead of literal text.
	if opts.Message != "" || opts.Author != "" {
		if opts.MessageRegex {
			// git greps with basic regular expressions by default, where "(" is
			// a literal and "\(" opens a group — the opposite of what a caller
			// writing "^feat\(.*\)" expects. Extended syntax matches the
			// regexes agents and users actually write.
			args = append(args, "--extended-regexp")
		} else {
			args = append(args, "--fixed-strings")
		}
		if !opts.CaseSensitive {
			args = append(args, "--regexp-ignore-case")
		}
	}
	args = append(args, revision)
	args = appendPathArgs(args, paths)

	out, err := runner.Run(ctx, args...)
	if err != nil {
		// Older git (< 2.31) does not know --diff-merges; retry without it so
		// the listing still works, at the price of empty file lists on merges.
		retry, retryErr := runner.Run(ctx, withoutArg(args, "--diff-merges=first-parent")...)
		if retryErr != nil {
			return nil, err
		}
		out = retry
	}

	commits := parseLogRecords(out)
	result := &LogResult{Range: revision, Shallow: state.shallow, Note: state.note}
	if len(commits) > limit {
		commits = commits[:limit]
		result.Truncated = true
	}
	result.Commits = commits
	result.CommitCount = len(commits)
	return result, nil
}

func (h *ExecHistory) Show(ctx context.Context, repoRoot string, opts ShowOptions) (*ShowResult, error) {
	if strings.TrimSpace(opts.Commit) == "" {
		return nil, fmt.Errorf("git: show requires a commit")
	}
	runner := h.newRunner(repoRoot)
	if err := ensureGitRepo(ctx, runner); err != nil {
		return nil, err
	}
	state := h.ensureHistory(ctx, repoRoot, runner)

	maxCommits := clampLimit(opts.MaxCommits, defaultShowCommits, maxShowCommits)
	revision, err := h.resolveRevisionArg(ctx, runner, opts.Commit, opts.To)
	if err != nil {
		return nil, err
	}
	paths, err := sanitizePaths(opts.Paths)
	if err != nil {
		return nil, err
	}

	shas, truncated, err := h.commitList(ctx, runner, revision, maxCommits)
	if err != nil {
		return nil, err
	}

	format := opts.Format
	if format == "" {
		format = model.DiffFormatGit
	}
	result := &ShowResult{
		Range:      revision,
		DiffFormat: string(format),
		Truncated:  truncated,
		Shallow:    state.shallow,
		Note:       state.note,
	}
	budget := maxShowPatchBytes
	for _, sha := range shas {
		diff, err := h.commitDiff(ctx, runner, sha, paths, format, &budget)
		if err != nil {
			return nil, err
		}
		if diff.Truncated {
			result.Truncated = true
		}
		result.Commits = append(result.Commits, *diff)
	}
	result.CommitCount = len(result.Commits)
	return result, nil
}

// commitList resolves a revision (single commit or range) into the SHAs whose
// diffs are returned, newest first. Path filters never reach it: they narrow
// each commit's diff, so a commit that touches none of them is still returned
// (with a note) instead of silently dropping out of the range — and a single
// revision always means exactly that commit, which a pathspec would otherwise
// turn into "the nearest ancestor touching these paths".
func (h *ExecHistory) commitList(ctx context.Context, runner Runner, revision string, maxCommits int) ([]string, bool, error) {
	if !isRevRange(revision) {
		return []string{revision}, false, nil
	}
	args := []string{"rev-list", fmt.Sprintf("--max-count=%d", maxCommits+1), revision}
	out, err := runner.Run(ctx, args...)
	if err != nil {
		return nil, false, err
	}
	shas := make([]string, 0, maxCommits)
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			shas = append(shas, line)
		}
	}
	if len(shas) > maxCommits {
		return shas[:maxCommits], true, nil
	}
	return shas, false, nil
}

// commitDiff renders one commit. Merge commits are shown as a combined diff
// (--cc); git prunes every hunk that matches a parent, so the common merge
// whose combined diff is empty is re-rendered against its first parent instead
// of returning a commit with no changes at all.
func (h *ExecHistory) commitDiff(ctx context.Context, runner Runner, sha string, paths []string, format model.DiffFormat, budget *int) (*CommitDiff, error) {
	// Metadata is read without the pathspec: git prints nothing at all for a
	// commit that touches none of the requested paths, and a commit filtered
	// down to an empty diff must still be reported with its message.
	entry, err := h.commitMetadata(ctx, runner, sha)
	if err != nil {
		return nil, err
	}
	patch, err := h.commitPatch(ctx, runner, sha, paths, false)
	if err != nil {
		return nil, err
	}
	diff := &CommitDiff{CommitEntry: entry, DiffMode: "single"}
	if entry.IsMerge {
		diff.DiffMode = "combined"
		if strings.TrimSpace(patch) == "" {
			firstParent, err := h.commitPatch(ctx, runner, sha, paths, true)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(firstParent) != "" {
				patch = firstParent
				diff.DiffMode = "first-parent"
				diff.Note = "combined diff was empty (every hunk matches a parent); diffed against the first parent instead"
			}
		}
	}
	if strings.TrimSpace(patch) == "" {
		if len(paths) > 0 {
			diff.Note = joinNotes(diff.Note, "commit has no changes in the requested paths")
		}
		return diff, nil
	}

	switch {
	case len(patch) > maxCommitPatchBytes:
		diff.Truncated = true
		diff.Note = joinNotes(diff.Note, fmt.Sprintf("patch omitted: %d bytes exceeds the %d byte per-commit limit", len(patch), maxCommitPatchBytes))
		h.attachFileMetadata(ctx, runner, sha, paths, diff)
		return diff, nil
	case len(patch) > *budget:
		diff.Truncated = true
		diff.Note = joinNotes(diff.Note, "patch omitted: the total patch size limit for this call was reached")
		h.attachFileMetadata(ctx, runner, sha, paths, diff)
		return diff, nil
	}
	*budget -= len(patch)

	diffFiles, diffHunks, changed, err := ParseUnifiedDiffFormats(patch)
	if err != nil {
		return nil, err
	}
	diff.Files = commitFilesFromChanged(changed)
	diff.Additions, diff.Deletions = totalChanges(diff.Files)
	diff.DiffFiles, diff.DiffHunks = model.SelectDiffPayload(diffFiles, diffHunks, format)
	if format == model.DiffFormatGitJson && len(diff.DiffHunks) == 0 && len(diffFiles) > 0 {
		// Combined ("@@@") hunks cannot be represented as two-way hunks, so
		// fall back to the raw per-file patch rather than reporting no changes.
		diff.DiffFiles = diffFiles
		diff.Note = joinNotes(diff.Note, "combined hunks cannot be represented as git-json hunks; returning raw per-file patches")
	}
	return diff, nil
}

// attachFileMetadata fills in the changed-file list of a commit whose patch was
// dropped for size. The patch is the usual source of that list, so without this
// an oversized commit would arrive with no indication of what it touched. It
// reads the compact `--raw --numstat` blocks instead of parsing a patch that is
// large by definition; a failure only costs the file list, so it is noted
// rather than propagated.
func (h *ExecHistory) attachFileMetadata(ctx context.Context, runner Runner, sha string, paths []string, diff *CommitDiff) {
	args := []string{"show", "--no-color", "--find-renames", "--raw", "--numstat", "-z", "--format="}
	if diff.DiffMode == "first-parent" {
		args = append(args, "-m", "--first-parent")
	} else {
		args = append(args, "--cc")
	}
	args = append(args, sha)
	args = appendPathArgs(args, paths)
	out, err := runner.Run(ctx, args...)
	if err != nil {
		diff.Note = joinNotes(diff.Note, "changed-file list unavailable: "+err.Error())
		return
	}
	diff.Files, _ = parseFileEntries(strings.Split(out, nulSeparator))
	diff.Additions, diff.Deletions = totalChanges(diff.Files)
}

func (h *ExecHistory) commitMetadata(ctx context.Context, runner Runner, sha string) (CommitEntry, error) {
	out, err := runner.Run(ctx, "show", "--no-patch", "--format="+commitFormat, sha)
	if err != nil {
		return CommitEntry{}, err
	}
	return parseCommitRecord(out)
}

// commitPatch renders a commit's patch alone (an empty --format suppresses the
// metadata header). Merges are rendered as a combined diff unless firstParent
// asks for the diff against parent one.
func (h *ExecHistory) commitPatch(ctx context.Context, runner Runner, sha string, paths []string, firstParent bool) (string, error) {
	args := []string{"show", "--no-color", "--find-renames", "--patch", "--format="}
	if firstParent {
		args = append(args, "-m", "--first-parent")
	} else {
		args = append(args, "--cc")
	}
	args = append(args, sha)
	args = appendPathArgs(args, paths)
	out, err := runner.Run(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimLeft(out, "\n"), nil
}

// resolveRevisionArg turns user input (single revision, abbreviated SHA, or
// range) plus an optional end revision into a revision argument built from
// full SHAs.
func (h *ExecHistory) resolveRevisionArg(ctx context.Context, runner Runner, commit, to string) (string, error) {
	commit = strings.TrimSpace(commit)
	to = strings.TrimSpace(to)
	if to != "" {
		if isRevRange(commit) {
			return "", fmt.Errorf("git: pass either a commit range or a separate end revision, not both: %q and %q", commit, to)
		}
		from, err := h.resolveRevision(ctx, runner, commit)
		if err != nil {
			return "", err
		}
		end, err := h.resolveRevision(ctx, runner, to)
		if err != nil {
			return "", err
		}
		return from + ".." + end, nil
	}
	if separator := rangeSeparator(commit); separator != "" {
		from, end, _ := strings.Cut(commit, separator)
		resolvedFrom, err := h.resolveRevision(ctx, runner, from)
		if err != nil {
			return "", err
		}
		resolvedEnd, err := h.resolveRevision(ctx, runner, end)
		if err != nil {
			return "", err
		}
		return resolvedFrom + separator + resolvedEnd, nil
	}
	return h.resolveRevision(ctx, runner, commit)
}

// resolveRevision resolves a ref or an abbreviated SHA to a full commit SHA.
// git resolves unique prefixes of four characters or more itself; shorter or
// ambiguous input is matched against the reachable commits so an agent can
// pass whatever prefix it read in a log or a comment.
func (h *ExecHistory) resolveRevision(ctx context.Context, runner Runner, rev string) (string, error) {
	rev = strings.TrimSpace(rev)
	if rev == "" {
		rev = "HEAD"
	}
	if err := validateGitRef("revision", rev); err != nil {
		return "", err
	}
	if out, err := runner.Run(ctx, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}"); err == nil {
		if sha := strings.TrimSpace(out); sha != "" {
			return sha, nil
		}
	}
	// git's failure message is localized, so never inspect it: fall back to a
	// prefix scan whenever the input could be an abbreviated SHA.
	if !hexRevision.MatchString(rev) {
		return "", fmt.Errorf("git: unknown revision %q", rev)
	}
	out, err := runner.Run(ctx, "log", "--all", "--format=%H")
	if err != nil {
		return "", fmt.Errorf("git: unknown revision %q", rev)
	}
	prefix := strings.ToLower(rev)
	var matches []string
	for line := range strings.SplitSeq(out, "\n") {
		sha := strings.TrimSpace(line)
		if sha != "" && strings.HasPrefix(sha, prefix) {
			matches = append(matches, sha)
		}
	}
	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) > 1:
		candidates := matches
		if len(candidates) > maxAmbiguousCandidates {
			candidates = candidates[:maxAmbiguousCandidates]
		}
		return "", fmt.Errorf("git: ambiguous revision %q matches %d commits (%s); pass a longer prefix", rev, len(matches), strings.Join(candidates, ", "))
	default:
		return "", fmt.Errorf("git: unknown revision %q", rev)
	}
}

// ensureHistory deepens a shallow checkout once per repository root. Remote
// reviews clone with --depth 1, which leaves no history for these tools to
// read; the first history call trades one fetch for a usable log. The outcome
// (including a failure) is memoized so a refused or slow remote is not retried
// on every tool call.
func (h *ExecHistory) ensureHistory(ctx context.Context, repoRoot string, runner Runner) deepenState {
	unlock := h.locks.lock(repoRoot)
	defer unlock()
	h.mu.Lock()
	state, ok := h.deepen[repoRoot]
	h.mu.Unlock()
	if ok {
		return state
	}

	state = h.deepenRepository(ctx, runner)
	if ctx.Err() != nil {
		// A cancelled context says nothing about the repository; leave the
		// state unmemoized so the next call can try again.
		return state
	}
	h.mu.Lock()
	h.deepen[repoRoot] = state
	h.mu.Unlock()
	return state
}

func (h *ExecHistory) deepenRepository(ctx context.Context, runner Runner) deepenState {
	if !isShallowRepo(ctx, runner) {
		return deepenState{}
	}
	before := reachableCommits(ctx, runner)
	auth := h.authArgsForRepo(ctx, runner)

	fetchCtx, cancel := context.WithTimeout(ctx, deepenTimeout)
	defer cancel()
	args := append(append([]string(nil), auth...), "fetch", "--deepen="+strconv.Itoa(deepenCommits))
	_, err := runner.Run(fetchCtx, args...)
	if err == nil && reachableCommits(fetchCtx, runner) <= before {
		// A plain --deepen extends the refs origin advertises. A remote review
		// checkout instead holds a bare commit fetched by SHA, so ask for that
		// commit's history explicitly before giving up.
		if remote := remoteURL(fetchCtx, runner); remote != "" {
			if head, headErr := runner.Run(fetchCtx, "rev-parse", "HEAD"); headErr == nil {
				explicit := append(append([]string(nil), auth...), "fetch", "--deepen="+strconv.Itoa(deepenCommits), "--", remote, strings.TrimSpace(head))
				_, err = runner.Run(fetchCtx, explicit...)
			}
		}
	}

	state := deepenState{shallow: isShallowRepo(ctx, runner)}
	switch {
	case err != nil:
		state.note = fmt.Sprintf("repository is a shallow checkout and could not be deepened (%v); only the commits present locally are listed", err)
	case state.shallow && reachableCommits(ctx, runner) <= before:
		state.note = "repository is a shallow checkout; deepening returned no additional commits, so only the commits present locally are listed"
	case state.shallow:
		state.note = fmt.Sprintf("repository is a shallow checkout deepened by %d commits; older history is unavailable", deepenCommits)
	}
	return state
}

// authArgsForRepo builds the credential header for the origin remote so a
// private repository can be deepened, reusing the header format the checkout
// manager clones with. A token travels only to its provider's configured host
// and only over http(s), where the header is what git would send anyway.
func (h *ExecHistory) authArgsForRepo(ctx context.Context, runner Runner) []string {
	host, ok := httpHost(remoteURL(ctx, runner))
	if !ok {
		return nil
	}
	switch {
	case hostMatches(host, defaultGitHubHost):
		return authHeaderArgs(model.ModeGitHub, h.auth.GitHubToken)
	case hostMatches(host, gitLabHost(h.auth.GitLabBaseURL)):
		return authHeaderArgs(model.ModeGitLab, h.auth.GitLabToken)
	default:
		return nil
	}
}

// httpHost extracts the host of an http(s) remote URL. Other transports (ssh,
// git, scp-like "git@host:path") authenticate through their own mechanisms and
// never receive an Authorization header from us.
func httpHost(remote string) (string, bool) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", false
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", false
	}
	return host, true
}

// hostMatches accepts the trusted host itself and its subdomains, and nothing
// else — "github.attacker.example" and "notgithub.com" both fail.
func hostMatches(host, trusted string) bool {
	trusted = strings.ToLower(strings.TrimSpace(trusted))
	if trusted == "" {
		return false
	}
	return host == trusted || strings.HasSuffix(host, "."+trusted)
}

// gitLabHost is the host of the configured GitLab API base URL.
func gitLabHost(baseURL string) string {
	if host, ok := httpHost(baseURL); ok {
		return host
	}
	return defaultGitLabHost
}

func remoteURL(ctx context.Context, runner Runner) string {
	out, err := runner.Run(ctx, "config", "--get", "remote.origin.url")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func isShallowRepo(ctx context.Context, runner Runner) bool {
	out, err := runner.Run(ctx, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

func reachableCommits(ctx context.Context, runner Runner) int {
	out, err := runner.Run(ctx, "rev-list", "--count", "HEAD")
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return count
}

func ensureGitRepo(ctx context.Context, runner Runner) error {
	if _, err := runner.Run(ctx, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("%w: %v", ErrNotAGitRepo, err)
	}
	return nil
}

// parseLogRecords parses `git log -z --format=<commitFormat> --raw --numstat`
// output into commit entries.
//
// Framing is positional over the NUL-delimited stream, never a marker byte in
// the payload: a commit message and a path may both contain any byte except
// NUL, so a printable or control-character record separator (0x1e/0x1f) would
// let a crafted or merely unusual commit desync the parse and drop or invent
// records. Each commit contributes exactly commitFields NUL-terminated metadata
// fields; whatever file entries follow are consumed structurally, and the next
// non-entry token starts the next commit.
func parseLogRecords(out string) []CommitEntry {
	tokens := strings.Split(out, nulSeparator)
	commits := make([]CommitEntry, 0, len(tokens)/commitFields+1)
	for i := 0; i+commitFields <= len(tokens); {
		entry := commitEntryFromFields(tokens[i : i+commitFields])
		i += commitFields
		files, consumed := parseFileEntries(tokens[i:])
		i += consumed
		if entry.SHA == "" {
			continue
		}
		entry.Files = files
		entry.Additions, entry.Deletions = totalChanges(files)
		commits = append(commits, entry)
	}
	return commits
}

// parseCommitRecord parses one commitFormat record into a commit entry.
func parseCommitRecord(out string) (CommitEntry, error) {
	fields := strings.Split(out, nulSeparator)
	if len(fields) < commitFields {
		return CommitEntry{}, fmt.Errorf("git: unexpected commit metadata output: %q", truncateForError(out))
	}
	return commitEntryFromFields(fields[:commitFields]), nil
}

// truncateForError keeps an unexpected-output error message readable.
func truncateForError(out string) string {
	const limit = 120
	out = strings.TrimSpace(out)
	if len(out) <= limit {
		return out
	}
	return out[:limit] + "..."
}

func commitEntryFromFields(fields []string) CommitEntry {
	authorDate, _ := time.Parse(time.RFC3339, strings.TrimSpace(fields[4]))
	commitDate, _ := time.Parse(time.RFC3339, strings.TrimSpace(fields[5]))
	parents := strings.Fields(fields[6])
	return CommitEntry{
		SHA:         strings.TrimSpace(fields[0]),
		ShortSHA:    strings.TrimSpace(fields[1]),
		Author:      fields[2],
		AuthorEmail: fields[3],
		Date:        authorDate,
		CommitDate:  commitDate,
		Subject:     fields[7],
		Body:        strings.TrimRight(fields[8], "\n"),
		Parents:     parents,
		IsMerge:     len(parents) > 1,
		Files:       []CommitFile{},
	}
}

// parseFileEntries consumes the `--raw --numstat -z` entries at the front of a
// NUL-token stream and reports how many tokens belonged to them, so a caller
// walking a multi-commit stream knows where the next commit begins. Raw entries
// carry the status letter, numstat entries the line counts; both are keyed by
// the (new) path. A token that is neither ends the block: it is the first
// metadata field of the next commit.
func parseFileEntries(tokens []string) ([]CommitFile, int) {
	files := make([]CommitFile, 0, len(tokens)/2)
	index := make(map[string]int, len(tokens)/2)
	upsert := func(path string) *CommitFile {
		if position, ok := index[path]; ok {
			return &files[position]
		}
		files = append(files, CommitFile{Path: path, Status: model.FileModified})
		index[path] = len(files) - 1
		return &files[len(files)-1]
	}

	i := 0
	for i < len(tokens) {
		// git separates the raw block from the numstat block with a newline, and
		// the stream ends with an empty token; neither is an entry, and neither
		// can be the start of a commit (%H is never empty), so both are skipped.
		token := strings.TrimLeft(tokens[i], "\n")
		if token == "" {
			i++
			continue
		}
		switch {
		case strings.HasPrefix(token, ":"):
			status := rawEntryStatus(token)
			if status == "" {
				return files, i
			}
			paths := 1
			if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
				paths = 2
			}
			if i+paths >= len(tokens) {
				return files, len(tokens)
			}
			file := upsert(tokens[i+paths])
			file.Status = fileStatusFromRawStatus(status)
			if paths == 2 {
				file.OldPath = tokens[i+1]
			}
			i += paths + 1
		case numstatEntry.MatchString(token):
			matches := numstatEntry.FindStringSubmatch(token)
			path := matches[3]
			oldPath := ""
			consumed := 1
			if path == "" {
				// Rename/copy: the old and new paths follow as separate tokens.
				if i+2 >= len(tokens) {
					return files, len(tokens)
				}
				oldPath = tokens[i+1]
				path = tokens[i+2]
				consumed = 3
			}
			file := upsert(path)
			if oldPath != "" {
				file.OldPath = oldPath
				file.Status = model.FileRenamed
			}
			additions, additionsErr := strconv.Atoi(matches[1])
			deletions, deletionsErr := strconv.Atoi(matches[2])
			if additionsErr != nil || deletionsErr != nil {
				file.Binary = true
			} else {
				file.Additions = additions
				file.Deletions = deletions
			}
			i += consumed
		default:
			// Start of the next commit's metadata.
			return files, i
		}
	}
	return files, i
}

// rawEntryStatus extracts the status letter from a raw entry such as
// ":100644 100644 abc123 def456 M" or ":100644 100644 abc123 def456 R096".
func rawEntryStatus(token string) string {
	fields := strings.Fields(token)
	if len(fields) < 2 {
		return ""
	}
	return fields[len(fields)-1]
}

func fileStatusFromRawStatus(status string) model.FileStatus {
	switch status[0] {
	case 'A', 'C':
		return model.FileAdded
	case 'D':
		return model.FileDeleted
	case 'R':
		return model.FileRenamed
	default:
		return model.FileModified
	}
}

func commitFilesFromChanged(changed []model.ChangedFile) []CommitFile {
	files := make([]CommitFile, 0, len(changed))
	for _, file := range changed {
		files = append(files, CommitFile{
			Path:      file.Path,
			Status:    file.Status,
			Additions: file.Additions,
			Deletions: file.Deletions,
		})
	}
	return files
}

func totalChanges(files []CommitFile) (int, int) {
	additions, deletions := 0, 0
	for _, file := range files {
		additions += file.Additions
		deletions += file.Deletions
	}
	return additions, deletions
}

func clampLimit(value, fallback, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value > max {
		return max
	}
	return value
}

func isRevRange(revision string) bool {
	return rangeSeparator(revision) != ""
}

// rangeSeparator reports which range operator a revision uses. "..." is
// checked first because it contains "..".
func rangeSeparator(revision string) string {
	for _, separator := range []string{"...", ".."} {
		if strings.Contains(revision, separator) {
			return separator
		}
	}
	return ""
}

// sanitizePaths trims path filters and rejects values git would read as
// options. Paths are still passed after "--" so they cannot become options.
func sanitizePaths(paths []string) ([]string, error) {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if err := validateGitArgValue("path", path); err != nil {
			return nil, err
		}
		cleaned = append(cleaned, path)
	}
	return cleaned, nil
}

func appendPathArgs(args []string, paths []string) []string {
	if len(paths) == 0 {
		return args
	}
	args = append(args, "--")
	return append(args, paths...)
}

func withoutArg(args []string, drop string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != drop {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

func joinNotes(existing, addition string) string {
	if existing == "" {
		return addition
	}
	if addition == "" {
		return existing
	}
	return existing + "; " + addition
}

// validateGitArgValue rejects a filter value that git would interpret as an
// option. Filters reach git as "--flag=value" pairs, but a bare leading "-"
// still indicates the caller (an LLM, or an SCM payload) is trying to inject
// one.
func validateGitArgValue(field, value string) error {
	if strings.HasPrefix(strings.TrimSpace(value), "-") {
		return fmt.Errorf("git: refusing %s that starts with '-': %q", field, value)
	}
	return nil
}

// keyedMutex serializes work per key. The deepen path uses it so concurrent
// tool calls against one checkout perform a single fetch.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (k *keyedMutex) lock(key string) func() {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = make(map[string]*sync.Mutex)
	}
	lock, ok := k.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		k.locks[key] = lock
	}
	k.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}
