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
	// Since and Until bound the author date. Any value git understands works
	// (RFC3339, "2026-01-02", "2 weeks ago").
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
	SHA         string       `json:"sha"`
	ShortSHA    string       `json:"short_sha"`
	Author      string       `json:"author"`
	AuthorEmail string       `json:"author_email,omitempty"`
	Date        time.Time    `json:"date"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body,omitempty"`
	Parents     []string     `json:"parents,omitempty"`
	IsMerge     bool         `json:"is_merge,omitempty"`
	Additions   int          `json:"additions"`
	Deletions   int          `json:"deletions"`
	Files       []CommitFile `json:"files"`
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

	recordSeparator = "\x1e"
	fieldSeparator  = "\x1f"
	// commitFields is the number of %-placeholders in commitFormat.
	commitFields = 8
)

// commitFormat renders one commit's metadata as fieldSeparator-delimited
// fields. The trailing separator terminates the body so whatever git appends
// next (the -z file blocks of `git log`, the patch of `git show`) starts at a
// deterministic offset.
const commitFormat = "%H" + fieldSeparator + "%h" + fieldSeparator + "%an" + fieldSeparator +
	"%ae" + fieldSeparator + "%aI" + fieldSeparator + "%P" + fieldSeparator +
	"%s" + fieldSeparator + "%b" + fieldSeparator

// ErrNotAGitRepo reports that the checkout has no git metadata, so no history
// can be read from it.
var ErrNotAGitRepo = errors.New("git: not a git repository")

// numstatEntry matches one `--numstat -z` record: added and deleted line
// counts ("-" for binary files) followed by the path, which is empty for a
// rename/copy whose old and new paths follow as separate NUL-terminated tokens.
var numstatEntry = regexp.MustCompile(`^(\d+|-)\t(\d+|-)\t(.*)$`)

var hexRevision = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// HistoryAuth carries the tokens used to deepen a shallow checkout of a
// private repository. The provider picks one by the origin remote's host.
type HistoryAuth struct {
	GitHubToken string
	GitLabToken string
}

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
	args := []string{"log", "--no-color", fmt.Sprintf("--max-count=%d", limit+1), "--format=" + recordSeparator + commitFormat,
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
		if opts.MessageRegex {
			// git greps with basic regular expressions by default, where "(" is
			// a literal and "\(" opens a group — the opposite of what a caller
			// writing "^feat\(.*\)" expects. Extended syntax matches the
			// regexes agents and users actually write.
			args = append(args, "--extended-regexp")
		} else {
			args = append(args, "--fixed-strings")
		}
	}
	if !opts.CaseSensitive {
		args = append(args, "--regexp-ignore-case")
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

	shas, truncated, err := h.commitList(ctx, runner, revision, paths, maxCommits)
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
// diffs are returned, newest first.
func (h *ExecHistory) commitList(ctx context.Context, runner Runner, revision string, paths []string, maxCommits int) ([]string, bool, error) {
	if !isRevRange(revision) {
		// A single revision means exactly that commit. It is already a full
		// SHA, and running it through rev-list with a pathspec would walk back
		// to an ancestor that happens to touch those paths instead: the path
		// filter must narrow the diff, never change which commit is shown.
		return []string{revision}, false, nil
	}
	args := []string{"rev-list", fmt.Sprintf("--max-count=%d", maxCommits+1), revision}
	args = appendPathArgs(args, paths)
	out, err := runner.Run(ctx, args...)
	if err != nil {
		return nil, false, err
	}
	shas := make([]string, 0, maxCommits)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
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
		return diff, nil
	case len(patch) > *budget:
		diff.Truncated = true
		diff.Note = joinNotes(diff.Note, "patch omitted: the total patch size limit for this call was reached")
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
	for _, line := range strings.Split(out, "\n") {
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

// authArgsForRepo builds the credential header for the origin remote's host so
// a private repository can be deepened, reusing the header format the checkout
// manager clones with.
func (h *ExecHistory) authArgsForRepo(ctx context.Context, runner Runner) []string {
	remote := remoteURL(ctx, runner)
	if remote == "" {
		return nil
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case strings.Contains(host, "github"):
		return authHeaderArgs(model.ModeGitHub, h.auth.GitHubToken)
	case strings.Contains(host, "gitlab"):
		return authHeaderArgs(model.ModeGitLab, h.auth.GitLabToken)
	default:
		return nil
	}
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

// parseLogRecords parses `git log --format=<recordSeparator><commitFormat>
// --raw --numstat -z` output into commit entries.
func parseLogRecords(out string) []CommitEntry {
	records := strings.Split(out, recordSeparator)
	commits := make([]CommitEntry, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record) == "" {
			continue
		}
		fields := strings.SplitN(record, fieldSeparator, commitFields+1)
		if len(fields) < commitFields {
			continue
		}
		entry := commitEntryFromFields(fields)
		if len(fields) > commitFields {
			entry.Files = parseFileBlocks(fields[commitFields])
			entry.Additions, entry.Deletions = totalChanges(entry.Files)
		}
		commits = append(commits, entry)
	}
	return commits
}

// parseCommitRecord parses one commitFormat record into a commit entry.
func parseCommitRecord(out string) (CommitEntry, error) {
	fields := strings.SplitN(out, fieldSeparator, commitFields+1)
	if len(fields) < commitFields {
		return CommitEntry{}, fmt.Errorf("git: unexpected commit metadata output: %q", truncateForError(out))
	}
	return commitEntryFromFields(fields), nil
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
	date, _ := time.Parse(time.RFC3339, strings.TrimSpace(fields[4]))
	parents := strings.Fields(fields[5])
	return CommitEntry{
		SHA:         strings.TrimSpace(fields[0]),
		ShortSHA:    strings.TrimSpace(fields[1]),
		Author:      fields[2],
		AuthorEmail: fields[3],
		Date:        date,
		Subject:     fields[6],
		Body:        strings.TrimRight(fields[7], "\n"),
		Parents:     parents,
		IsMerge:     len(parents) > 1,
		Files:       []CommitFile{},
	}
}

// parseFileBlocks parses the `--raw --numstat -z` blocks git appends to a log
// record: raw entries carry the status letter, numstat entries the line counts.
// Both are NUL-terminated and keyed by the (new) path.
func parseFileBlocks(block string) []CommitFile {
	tokens := strings.Split(block, "\x00")
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

	for i := 0; i < len(tokens); i++ {
		// git separates the raw block from the numstat block with a newline;
		// the leading NUL of the first block leaves an empty token behind.
		token := strings.TrimLeft(tokens[i], "\n")
		if token == "" {
			continue
		}
		switch {
		case strings.HasPrefix(token, ":"):
			status := rawEntryStatus(token)
			if status == "" {
				continue
			}
			paths := 1
			if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
				paths = 2
			}
			if i+paths >= len(tokens) {
				return files
			}
			file := upsert(tokens[i+paths])
			file.Status = fileStatusFromRawStatus(status)
			if paths == 2 {
				file.OldPath = tokens[i+1]
			}
			i += paths
		case numstatEntry.MatchString(token):
			matches := numstatEntry.FindStringSubmatch(token)
			path := matches[3]
			oldPath := ""
			if path == "" {
				// Rename/copy: the old and new paths follow as separate tokens.
				if i+2 >= len(tokens) {
					return files
				}
				oldPath = tokens[i+1]
				path = tokens[i+2]
				i += 2
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
				continue
			}
			file.Additions = additions
			file.Deletions = deletions
		}
	}
	return files
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
