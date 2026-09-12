package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/clipboard"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/textsan"
	"github.com/spf13/cobra"
)

type sessionOptions struct {
	sessionID string
	clipboard bool
	warnings  bool
	history   bool
	// closed widens the picker's remote scope past the open merge and pull
	// requests to the merged and closed ones: the review published on a request
	// that has since been merged is still the review of that work.
	closed bool
}

func (a *app) newSessionCmd() *cobra.Command {
	var opts sessionOptions
	cmd := &cobra.Command{
		Use:   "session [session-id]",
		Short: "Print a saved review",
		Long: "Print a review from a saved chat session. Omit the session id to pick " +
			"one from a list: in a checkout it opens on the sessions of that repository, " +
			"with the sessions of the checked-out branch and all saved sessions one scope " +
			"away (Tab, or the left and right arrows); outside a checkout it lists them all. " +
			"The chosen session then asks what to do with it — print the review, copy it to " +
			"the clipboard, or chat about it — with Esc or backspace going back to the list. " +
			"The remote scope of that list carries the reviews published on the project's open " +
			"merge and pull requests; --include-closed widens it to the merged and closed ones too. " +
			"Without a terminal the most recently updated session is printed instead. " +
			"With --warnings only the " +
			"run's warnings are printed instead of the review. With --clipboard the " +
			"output is copied to the system clipboard instead of printed. With --history " +
			"archived review revisions are printed oldest first, with their correction reasons.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSession(cmd.Context(), opts, args)
		},
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return a.completeSessionIDs(toComplete)
		},
	}
	cmd.Flags().StringVar(&opts.sessionID, "session", "", "Print an existing session by id")
	cmd.Flags().BoolVar(&opts.history, "history", false, "Print archived review revisions, oldest first")
	cmd.Flags().BoolVar(&opts.closed, "include-closed", false, "In the picker's remote scope, list the reviews on merged and closed merge/pull requests too, not only the open ones")
	cmd.Flags().BoolVar(&opts.warnings, "warnings", false, "Print only the warnings the run recorded, not the review")
	cmd.Flags().BoolVar(&opts.clipboard, "clipboard", false, "Copy the output to the system clipboard instead of printing it (uses the platform clipboard helper: pbcopy, clip.exe, wl-copy, xclip, xsel, or termux-clipboard-set)")
	cmd.MarkFlagsMutuallyExclusive("history", "warnings")
	_ = cmd.RegisterFlagCompletionFunc("session", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return a.completeSessionIDs(toComplete)
	})
	return cmd
}

func (a *app) completeSessionIDs(prefix string) ([]string, cobra.ShellCompDirective) {
	store, err := session.NewStore(a.sessionDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	infos, err := store.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(infos))
	for _, info := range infos {
		if strings.HasPrefix(info.ID, prefix) {
			candidates = append(candidates, info.ID)
		}
	}
	return candidates, cobra.ShellCompDirectiveNoFileComp
}

// reviewedSessions lists the sessions this command can act on, newest first:
// one that holds no review has nothing to print, copy or discuss. An empty
// result is not an error here — the picker can still offer the reviews
// published on the project's requests, which is the whole point of a machine
// that reviewed nothing locally — so the caller decides what emptiness means.
// stored says how many sessions the store holds either way, which is what
// tells "nothing saved" from "nothing printable".
func reviewedSessions(store *session.Store) (reviewed []session.Info, stored int, err error) {
	infos, err := store.List()
	if err != nil {
		return nil, 0, err
	}
	reviewed = make([]session.Info, 0, len(infos))
	for _, info := range infos {
		if info.HasResult {
			reviewed = append(reviewed, info)
		}
	}
	return reviewed, len(infos), nil
}

// noReviewedSessionError says what an empty listing means, so a store holding
// only unfinished sessions does not read as an empty one.
func noReviewedSessionError(stored int) error {
	if stored > 0 {
		return fmt.Errorf("session: no saved session holds a review")
	}
	return fmt.Errorf("session: no saved sessions")
}

// chooseSession runs the interactive half of the command: pick a session, then
// pick what to do with it. Backspace or Esc in the action prompt goes back to
// the list, landing on the scope and row it was left at; Esc in the list itself
// leaves without doing anything. The chosen action is carried out here, so the
// caller only handles the non-interactive path.
func (a *app) chooseSession(ctx context.Context, store *session.Store, opts sessionOptions,
	infos []session.Info, stored int, w io.Writer) error {
	place := sessionLocation(ctx)
	remote := newRemoteFinder(a, place, opts.closed)
	if len(infos) == 0 && !remote.available() {
		// Nothing saved here and no project to ask: there is no list to draw.
		return noReviewedSessionError(stored)
	}
	state := newSessionPickState()
	for {
		chosen, next, err := a.pickSession(place, infos, "Session", remote, state)
		if err != nil {
			return fmt.Errorf("session: %w", err)
		}
		state = next
		action, err := a.pickSessionAction(place, chosen)
		if errors.Is(err, pick.ErrAborted) {
			// "Back", not "never mind": the list opens again where it was.
			continue
		}
		if err != nil {
			return fmt.Errorf("session: %w", err)
		}
		if !chosen.saved() {
			return a.actOnRemoteReview(ctx, remote, *chosen.remote, action, opts, w)
		}
		if action == sessionActionChat {
			return a.runChat(ctx, chatOptions{sessionID: chosen.info.ID}, nil)
		}
		return a.printSession(ctx, store, chosen.info.ID, opts, action == sessionActionCopy, w)
	}
}

func (a *app) runSession(ctx context.Context, opts sessionOptions, args []string) error {
	return a.runSessionTo(ctx, opts, args, os.Stdout)
}

func (a *app) runSessionTo(ctx context.Context, opts sessionOptions, args []string, w io.Writer) error {
	if opts.history && opts.warnings {
		return fmt.Errorf("session: --history cannot be combined with --warnings")
	}
	if opts.sessionID != "" && len(args) > 0 {
		return fmt.Errorf("session: pass the session id as an argument or with --session, not both")
	}
	sessionID := opts.sessionID
	if len(args) > 0 {
		sessionID = args[0]
	}

	store, err := session.NewStore(a.sessionDir)
	if err != nil {
		return err
	}
	if sessionID == "" {
		infos, stored, err := reviewedSessions(store)
		if err != nil {
			return err
		}
		// A prompt needs a terminal, and --clipboard already says what to do
		// with the session; either way the newest one is the answer a pipe, a
		// redirect or a daemon-spawned run has always got. The prompt is
		// offered even with nothing saved here: the reviews published on the
		// project's requests are a list of their own.
		if a.interactiveSelect() && !opts.clipboard {
			return a.chooseSession(ctx, store, opts, infos, stored, w)
		}
		if len(infos) == 0 {
			return noReviewedSessionError(stored)
		}
		sessionID = infos[0].ID
	}
	return a.printSession(ctx, store, sessionID, opts, opts.clipboard, w)
}

// printSession renders one session's review — or its history or warnings — to
// w, or hands it to the clipboard instead when clip is set.
func (a *app) printSession(ctx context.Context, store *session.Store, sessionID string,
	opts sessionOptions, clip bool, w io.Writer) error {
	sess, err := store.Load(sessionID)
	if err != nil {
		return err
	}
	if sess.Result == nil {
		return fmt.Errorf("session: %s has no saved review", sess.ID)
	}
	render, subject := a.formatReview, "review"
	if opts.warnings {
		render, subject = a.formatWarnings, "warnings"
	}
	if opts.history {
		subject = "review history"
		render = func(w io.Writer, _ *model.ReviewResult) error { return a.formatReviewHistory(w, sess.ReviewHistory) }
	}
	origin := "session " + textsan.StripControl(sess.ID)
	if err := a.emitReview(ctx, w, clip, subject, origin, func(out io.Writer) error {
		return render(out, sess.Result)
	}); err != nil {
		return fmt.Errorf("session: %w", err)
	}
	return nil
}

// emitReview prints rendered review output, or — with clip — hands it to the
// platform clipboard helper and prints a one-line confirmation instead of the
// content itself. Rendering into a buffer (not a *os.File) makes the formatter
// pick the unstyled form, so the clipboard carries Markdown or JSON source
// rather than terminal escapes. subject names what was copied (review or
// warnings) and origin where it came from ("session abc123", "GitLab MR
// grp/proj!42") in that confirmation.
func (a *app) emitReview(ctx context.Context, w io.Writer, clip bool, subject, origin string,
	render func(io.Writer) error) error {
	if !clip {
		return render(w)
	}
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		return err
	}
	copyFn := a.clipboardCopy
	if copyFn == nil {
		copyFn = clipboard.Copy
	}
	helper, err := copyFn(ctx, buf.Bytes())
	if err != nil {
		return err
	}
	confirmation := fmt.Sprintf("Copied %s of %s to the clipboard (%d bytes) via %s.",
		subject, origin, buf.Len(), helper)
	if _, err := fmt.Fprintln(w, noteText(confirmation, useColor(w))); err != nil {
		// The clipboard already holds the content; a confirmation that could not be
		// written (closed pipe, full disk) is not a failed copy, so warn instead of
		// reporting the command as failed.
		a.warnf("could not print the clipboard confirmation: %v", err)
	}
	return nil
}

// noteText sets an aside — a line about the run rather than part of its output
// — in the italic light grey the pick confirmation is set in, so the two lines
// that talk ABOUT a run read alike and neither reads as its output. The reset
// lands before the newline so the colour cannot bleed into what follows.
func noteText(text string, color bool) string {
	if !color || text == "" {
		return text
	}
	return "\x1b[" + selectionStyle + "m" + text + "\x1b[0m"
}

// useColor reports whether a writer is a terminal a person is watching, with
// colour not switched off — the same test the pick confirmation applies before
// styling its line.
func useColor(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || !isInteractiveTerminal(file) {
		return false
	}
	_, noColor := os.LookupEnv("NO_COLOR")
	return !noColor
}

func (a *app) formatReviewHistory(w io.Writer, history []session.ReviewRevision) error {
	if a.jsonOutput || a.outputFormat == "json" {
		if history == nil {
			history = []session.ReviewRevision{}
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(history)
	}
	if len(history) == 0 {
		_, err := fmt.Fprintln(w, "No previous review versions.")
		return err
	}
	for _, revision := range history {
		if revision.Result == nil {
			return fmt.Errorf("session: history contains an empty review")
		}
		if _, err := fmt.Fprintf(w, "## Review revision %d\n\nReplaced: %s\n\nReason: %s\n\n", revision.Result.Revision, revision.ReplacedAt.Format(time.RFC3339), textsan.StripControl(revision.Reason)); err != nil {
			return err
		}
		if err := a.formatReview(w, revision.Result); err != nil {
			return err
		}
	}
	return nil
}
