package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// gitFeedbackOptions are the flags of `nickpit git feedback`. A local review
// lives in a saved session rather than on a request, so the selector is a
// session id instead of --repo/--id/--url.
type gitFeedbackOptions struct {
	sessionID string
	clipboard bool
	list      bool
}

func (a *app) newGitFeedbackCmd() *cobra.Command {
	var opts gitFeedbackOptions
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Print or copy a saved review of the checked-out branch",
		Long: "Print a review NickPit produced for this checkout, read back from the saved " +
			"session — no re-review and no LLM call, the local counterpart of `nickpit gitlab " +
			"feedback` and `nickpit github feedback`. Only reviews of this repository whose head " +
			"ref is the checked-out branch are offered, so what is listed belongs to the branch " +
			"you are on; `nickpit session` prints any saved review regardless of branch. The " +
			"output is the normal review output (`-o markdown|json|raw`); `--clipboard` copies it " +
			"instead of printing it.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runGitFeedback(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.sessionID, "session", "", "Print this saved session instead of choosing one from a list")
	cmd.Flags().BoolVar(&opts.list, "list", false, "List the saved reviews of the checked-out branch instead of printing one")
	cmd.Flags().BoolVar(&opts.clipboard, "clipboard", false, "Copy the review to the system clipboard instead of printing it (uses the platform clipboard helper: pbcopy, clip.exe, wl-copy, xclip, xsel, or termux-clipboard-set)")
	cmd.MarkFlagsMutuallyExclusive("list", "clipboard")
	cmd.MarkFlagsMutuallyExclusive("list", "session")
	_ = cmd.RegisterFlagCompletionFunc("session", func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return a.completeSessionIDs(prefix)
	})
	return cmd
}

func (a *app) runGitFeedback(ctx context.Context, w io.Writer, opts gitFeedbackOptions) error {
	store, err := session.NewStore(a.sessionDir)
	if err != nil {
		return err
	}
	if opts.sessionID != "" {
		return a.emitLocalReview(ctx, w, store, opts.sessionID, opts.clipboard)
	}
	checkout, err := currentCheckout(ctx)
	if err != nil {
		return fmt.Errorf("git feedback: %w", err)
	}
	infos, err := store.List()
	if err != nil {
		return err
	}
	matching, otherBranch := localReviewsFor(infos, checkout)
	if opts.list {
		return a.formatLocalReviewList(w, matching, checkout)
	}
	if len(matching) == 0 {
		return fmt.Errorf("git feedback: %s", noLocalReviewMessage(checkout, otherBranch))
	}
	if !a.interactiveSelect() {
		return fmt.Errorf("git feedback: choosing a saved review needs a terminal; pass --session <id> or --list "+
			"(%d saved review(s) of %s)", len(matching), textsan.StripControl(checkout.branch))
	}
	chosen, err := a.pickLocalReview(matching, checkout)
	if err != nil {
		return err
	}
	return a.emitLocalReview(ctx, w, store, chosen.ID, opts.clipboard)
}

// checkout is the repository a local review is looked up against: its working
// tree root and the branch checked out in it.
type checkout struct {
	root   string
	branch string
}

// currentCheckout resolves the working directory to the repository and branch
// a saved review has to belong to. A detached HEAD has no branch, so nothing
// can match it strictly; saying so here beats an empty list.
func currentCheckout(ctx context.Context) (checkout, error) {
	dir, err := os.Getwd()
	if err != nil {
		return checkout{}, err
	}
	root, ok := git.TopLevel(ctx, dir)
	if !ok {
		return checkout{}, fmt.Errorf("%s is not inside a git repository", dir)
	}
	branch, err := git.CurrentBranch(ctx, root)
	if err != nil || strings.TrimSpace(branch) == "" {
		return checkout{}, fmt.Errorf("no branch is checked out in %s (detached HEAD); "+
			"pass --session <id>, or print any saved review with `nickpit session`", root)
	}
	return checkout{root: root, branch: strings.TrimSpace(branch)}, nil
}

// localReviewsFor splits the saved sessions into the local reviews of this
// checkout's branch and those of the same repository on another ref, newest
// review first. Sessions of another repository, without a review, or from a
// remote review are neither. The order is the review's own timestamp, not the
// session's: a session touched later by a chat did not produce a newer review.
func localReviewsFor(infos []session.Info, target checkout) (matching, otherBranch []session.Info) {
	for _, info := range infos {
		if !info.HasResult || info.Source.Mode != string(model.ModeLocal) {
			continue
		}
		if !sameCheckout(info.Source.RepoRoot, target.root) {
			continue
		}
		if headRefIsBranch(info.Source.HeadRef, target.branch) {
			matching = append(matching, info)
			continue
		}
		otherBranch = append(otherBranch, info)
	}
	sort.SliceStable(matching, func(i, j int) bool {
		return reviewTime(matching[i]).After(reviewTime(matching[j]))
	})
	return matching, otherBranch
}

// sameCheckout reports whether a recorded review directory belongs to the
// working tree rooted at root. A review may have been run from a subdirectory,
// which is the same checkout.
func sameCheckout(recorded, root string) bool {
	if recorded == "" || root == "" {
		return false
	}
	recorded, root = filepath.Clean(recorded), filepath.Clean(root)
	if recorded == root {
		return true
	}
	return strings.HasPrefix(recorded, root+string(filepath.Separator))
}

// headRefIsBranch reports whether a saved review's head ref is the branch
// checked out now. "HEAD" counts: it is what the range submodes record when
// the head was not named, and it meant the checked-out branch then exactly as
// it does now — the picker shows the row's refs and age, so a review of an
// older branch that recorded "HEAD" is visible as such before it is chosen.
// An empty head ref never counts: the working-tree submodes (uncommitted,
// staged, unstaged) record no refs at all, and `nickpit session` is where
// those are read back.
func headRefIsBranch(headRef, branch string) bool {
	headRef, branch = strings.TrimSpace(headRef), strings.TrimSpace(branch)
	if headRef == "" || branch == "" {
		return false
	}
	return headRef == branch || headRef == "HEAD" || headRef == "refs/heads/"+branch
}

// noLocalReviewMessage explains an empty list, naming the reviews that exist
// for this repository but not for this branch rather than leaving the reader
// wondering whether anything was saved at all.
func noLocalReviewMessage(target checkout, otherBranch []session.Info) string {
	message := fmt.Sprintf("no saved review of branch %s in %s",
		textsan.StripControl(target.branch), textsan.StripControl(target.root))
	if len(otherBranch) > 0 {
		return message + fmt.Sprintf(" (%d saved review(s) of this repository are of another ref "+
			"or of the working tree; `nickpit session` prints those)", len(otherBranch))
	}
	return message + " (run `nickpit git branch` or `nickpit git commits` to produce one)"
}

// pickLocalReview draws the saved reviews of the branch, newest first.
func (a *app) pickLocalReview(reviews []session.Info, target checkout) (session.Info, error) {
	items := make([]pick.Item, len(reviews))
	for i, info := range reviews {
		items[i] = pick.Item{
			// The session id is what --session takes, so the title line keeps it
			// in full even where the column is too narrow for it.
			Detail: info.ID,
			Cells: []string{
				info.Source.Submode,
				localReviewRange(info),
				strconv.Itoa(info.Findings) + " findings",
				textsan.StripControl(info.Model),
				relativeAge(reviewTime(info)),
			},
			Match: info.ID + " " + info.Source.Submode + " " + localReviewRange(info) + " " + info.Model,
			CellStyles: rowStyles(5, map[int]string{
				4: ageStyle(reviewTime(info)),
			}),
		}
	}
	index, err := a.selectOne(pick.Options{
		Title: fmt.Sprintf("Saved reviews of %s in %s", textsan.StripControl(target.branch),
			textsan.StripControl(filepath.Base(target.root))),
		Items: items,
		// Gold says which kind of review the row is, the way a draft is
		// labelled in a request list; the range is a ref, the finding count the
		// value being compared, and the model is who wrote the review.
		CellStyles:  []string{pick.StyleCaveat, pick.StyleText, pick.StyleDetail, pick.StyleAuthor, pick.StyleAge},
		ColumnKinds: []pick.ColumnKind{pick.KindPlain, pick.KindRef},
		DetailStyle: pick.StyleHash,
	})
	if err != nil {
		return session.Info{}, err
	}
	chosen := reviews[index]
	a.printSelection("Selected review ", chosen.ID, pick.StyleHash, " "+localReviewRange(chosen))
	return chosen, nil
}

// localReviewRange is the revision span a saved review covered, in the form
// its command took it: "base..head", or just one end when the other is
// implicit.
func localReviewRange(info session.Info) string {
	base, head := textsan.StripControl(info.Source.BaseRef), textsan.StripControl(info.Source.HeadRef)
	switch {
	case base != "" && head != "":
		return base + ".." + head
	case head != "":
		return head
	default:
		return base
	}
}

// reviewTime is when the review was produced, falling back to the session's
// last update for a session saved before results carried a timestamp.
func reviewTime(info session.Info) time.Time {
	if !info.CreatedAt.IsZero() {
		return info.CreatedAt
	}
	return info.UpdatedAt
}

// localReviewEntry is one line of `--list`: enough to tell the saved reviews of
// a branch apart and to name one with --session.
type localReviewEntry struct {
	SessionID string    `json:"session_id"`
	ReviewID  string    `json:"review_id,omitempty"`
	Submode   string    `json:"submode,omitempty"`
	BaseRef   string    `json:"base_ref,omitempty"`
	HeadRef   string    `json:"head_ref,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Revision  uint64    `json:"revision"`
	Findings  int       `json:"findings"`
	Verdict   string    `json:"verdict,omitempty"`
	Model     string    `json:"model,omitempty"`
}

func (a *app) formatLocalReviewList(w io.Writer, reviews []session.Info, target checkout) error {
	entries := make([]localReviewEntry, 0, len(reviews))
	for _, info := range reviews {
		entries = append(entries, localReviewEntry{
			SessionID: info.ID,
			ReviewID:  info.ReviewID,
			Submode:   info.Source.Submode,
			BaseRef:   info.Source.BaseRef,
			HeadRef:   info.Source.HeadRef,
			CreatedAt: reviewTime(info),
			Revision:  info.Revision,
			Findings:  info.Findings,
			Verdict:   info.Verdict,
			Model:     info.Model,
		})
	}
	if a.jsonOutput || a.outputFormat == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintf(w, "No saved review of branch %s in %s.\n",
			textsan.StripControl(target.branch), textsan.StripControl(target.root))
		return err
	}
	if _, err := fmt.Fprintf(w, "%d saved review(s) of %s, newest first; print one with --session <id>.\n\n",
		len(entries), textsan.StripControl(target.branch)); err != nil {
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "SESSION ID\tREVIEWED\tMODE\tRANGE\tREV\tFINDINGS\tVERDICT\tMODEL"); err != nil {
		return err
	}
	for i, entry := range entries {
		reviewed := "unknown"
		if !entry.CreatedAt.IsZero() {
			reviewed = entry.CreatedAt.Local().Format(time.RFC3339)
		}
		// The refs, the model and the verdict are whatever the session file
		// holds; strip control characters so a crafted value cannot rewrite the
		// table with escapes.
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\n",
			entry.SessionID, reviewed, textsan.StripControl(entry.Submode),
			localReviewRange(reviews[i]), entry.Revision, entry.Findings,
			textsan.StripControl(entry.Verdict), textsan.StripControl(entry.Model)); err != nil {
			return err
		}
	}
	return table.Flush()
}

// emitLocalReview prints or copies the review saved in one session. A session
// id is accepted from anywhere the user may have copied it, so a session that
// holds a published review rather than a local one is pointed at the commands
// that read those instead of being printed here under the wrong name.
func (a *app) emitLocalReview(ctx context.Context, w io.Writer, store *session.Store, sessionID string, clip bool) error {
	sess, err := store.Load(sessionID)
	if err != nil {
		return err
	}
	if sess.Result == nil {
		return fmt.Errorf("git feedback: session %s has no saved review", textsan.StripControl(sess.ID))
	}
	if mode := sess.Source.Mode; mode != "" && mode != string(model.ModeLocal) {
		return fmt.Errorf("git feedback: session %s is a %s review of %s, not a local one; "+
			"print it with `nickpit session %s`, or read it back from the request with `nickpit %s feedback`",
			textsan.StripControl(sess.ID), textsan.StripControl(mode), textsan.StripControl(sess.Source.Repo),
			textsan.StripControl(sess.ID), textsan.StripControl(mode))
	}
	origin := "session " + textsan.StripControl(sess.ID)
	if err := a.emitReview(ctx, w, clip, "review", origin, func(out io.Writer) error {
		return a.formatReview(out, sess.Result)
	}); err != nil {
		return fmt.Errorf("git feedback: %w", err)
	}
	return nil
}
