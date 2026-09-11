package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BranchRef is one branch ref offered for interactive selection: enough to tell
// branches apart without reading the log.
type BranchRef struct {
	// Name is the short ref name ("main", "origin/main").
	Name string
	// Remote is the remote a remote-tracking ref belongs to ("origin"), empty
	// for a local branch.
	Remote string
	// Branch is the name without the remote prefix, so a local branch and its
	// remote-tracking counterparts share one value and can be folded into a
	// single choice.
	Branch string
	// Current marks the checked-out branch.
	Current bool
	Subject string
	Author  string
	Date    time.Time
}

// CommitRef is one commit offered for interactive selection.
type CommitRef struct {
	SHA      string
	ShortSHA string
	Subject  string
	Author   string
	Date     time.Time
	// Parent is the commit's first parent, empty for a root commit. A range
	// picker needs it: "base..head" excludes its base, so reviewing a commit
	// means diffing from its parent.
	Parent string
}

// EmptyTree is git's empty tree in this repository: the base a root commit is
// diffed against, since it has no parent. The id depends on the repository's
// object format — a SHA-256 repository has a different empty tree than a
// SHA-1 one — so it is asked for rather than hard-coded. `hash-object` without
// -w only computes the id; git resolves the empty tree of its active hash
// algorithm whether or not the object was ever written.
func EmptyTree(ctx context.Context, repoRoot string) (string, error) {
	return emptyTree(ctx, ExecRunner{RepoRoot: repoRoot})
}

func emptyTree(ctx context.Context, runner Runner) (string, error) {
	out, err := runner.Run(ctx, "hash-object", "-t", "tree", os.DevNull)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return "", fmt.Errorf("git: empty tree id came back empty")
	}
	return id, nil
}

// CurrentBranch returns the checked-out branch of repoRoot. A detached HEAD has
// no branch and yields an error.
func CurrentBranch(ctx context.Context, repoRoot string) (string, error) {
	return currentBranch(ctx, ExecRunner{RepoRoot: repoRoot})
}

// DefaultBranch returns the remote-tracking branch origin/HEAD points at
// ("origin/main"), which is what a branch review diffs against by default.
func DefaultBranch(ctx context.Context, repoRoot string) (string, error) {
	return defaultBranch(ctx, ExecRunner{RepoRoot: repoRoot})
}

func currentBranch(ctx context.Context, runner Runner) (string, error) {
	out, err := runner.Run(ctx, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func defaultBranch(ctx context.Context, runner Runner) (string, error) {
	out, err := runner.Run(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// branchRefFormat and commitRefFormat frame their records with NUL separators:
// a subject or an author name may contain any byte except NUL, so splitting on
// anything else could be fooled by a crafted commit message.
const (
	branchRefFormat = "%(refname:short)%00%(refname)%00%(symref)%00%(committerdate:unix)%00%(authorname)%00%(contents:subject)"
	commitRefFormat = "%H%x00%h%x00%ct%x00%an%x00%P%x00%s"
)

// Branches lists the local and remote-tracking branches of repoRoot, newest
// commit first, with the checked-out branch marked. Local branches come before
// remote ones at equal age so a picker offers the working set first.
func Branches(ctx context.Context, repoRoot string) ([]BranchRef, error) {
	return branches(ctx, ExecRunner{RepoRoot: repoRoot})
}

func branches(ctx context.Context, runner Runner) ([]BranchRef, error) {
	out, err := runner.Run(ctx, "for-each-ref", "--sort=-committerdate",
		"--format="+branchRefFormat, "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	current, _ := currentBranch(ctx, runner)
	var branches []BranchRef
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		fields := strings.Split(line, "\x00")
		if len(fields) < 6 || fields[0] == "" {
			continue
		}
		// A symbolic ref (origin/HEAD, whose short name is just "origin") is an
		// alias for another listed branch; offering it would duplicate that
		// branch under a name no review should record.
		if fields[2] != "" {
			continue
		}
		remote, branch := splitBranchRef(fields[1])
		if branch == "" {
			continue
		}
		branches = append(branches, BranchRef{
			Name:    fields[0],
			Remote:  remote,
			Branch:  branch,
			Current: fields[0] == current && current != "",
			Date:    unixTime(fields[3]),
			Author:  fields[4],
			Subject: fields[5],
		})
	}
	return branches, nil
}

// splitBranchRef splits a full ref name into the remote it belongs to (empty
// for a local branch) and the branch name below it. A ref that names no branch
// — a remote's own HEAD, or anything outside refs/heads and refs/remotes —
// yields an empty branch, which the caller skips.
func splitBranchRef(refName string) (remote, branch string) {
	if rest, ok := strings.CutPrefix(refName, "refs/heads/"); ok {
		return "", rest
	}
	rest, ok := strings.CutPrefix(refName, "refs/remotes/")
	if !ok {
		return "", ""
	}
	remote, branch, ok = strings.Cut(rest, "/")
	if !ok || branch == "HEAD" {
		return remote, ""
	}
	return remote, branch
}

// Commits lists the newest commits reachable from rev (HEAD when empty),
// newest first, capped at limit.
func Commits(ctx context.Context, repoRoot, rev string, limit int) ([]CommitRef, error) {
	return commits(ctx, ExecRunner{RepoRoot: repoRoot}, rev, limit)
}

func commits(ctx context.Context, runner Runner, rev string, limit int) ([]CommitRef, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("git: commit limit must be positive")
	}
	if rev == "" {
		rev = "HEAD"
	}
	out, err := runner.Run(ctx, "log",
		"--max-count="+strconv.Itoa(limit), "--format="+commitRefFormat, rev)
	if err != nil {
		return nil, err
	}
	var commits []CommitRef
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		fields := strings.Split(line, "\x00")
		if len(fields) < 6 || fields[0] == "" {
			continue
		}
		commit := CommitRef{
			SHA:      fields[0],
			ShortSHA: fields[1],
			Date:     unixTime(fields[2]),
			Author:   fields[3],
			Subject:  fields[5],
		}
		// %P lists every parent; a merge's first parent is the one its range
		// follows, which is the same side `git diff a..b` walks.
		if first, _, _ := strings.Cut(fields[4], " "); first != "" {
			commit.Parent = first
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func unixTime(raw string) time.Time {
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

// Remote is one configured git remote: its name and the URL as git stores it.
type Remote struct {
	// Name is the remote's name ("origin").
	Name string
	// URL is the configured URL in whatever form it was written: https://,
	// ssh://, git:// or SCP-style git@host:path.
	URL string
}

// Remotes lists the remotes configured in repoRoot, in git's own config order.
// A repository without remotes yields an empty list, not an error — "this
// checkout has nothing to compare against" is an answer, not a failure.
func Remotes(ctx context.Context, repoRoot string) ([]Remote, error) {
	return remotes(ctx, ExecRunner{RepoRoot: repoRoot})
}

func remotes(ctx context.Context, runner Runner) ([]Remote, error) {
	out, err := runner.Run(ctx, "config", "--get-regexp", `^remote\..*\.url$`)
	if err != nil {
		// `git config --get-regexp` exits 1 when the pattern matches nothing,
		// which is a repository without remotes; every other failure (not a
		// repository, unreadable config) is real and surfaces.
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	var list []Remote
	for line := range strings.SplitSeq(out, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		// "remote.<name>.url": the name may itself contain dots, so it is what
		// remains between the fixed prefix and suffix rather than a field split.
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		if name == "" || name == key {
			continue
		}
		list = append(list, Remote{Name: name, URL: strings.TrimSpace(value)})
	}
	return list, nil
}
