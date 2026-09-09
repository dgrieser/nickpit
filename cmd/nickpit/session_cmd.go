package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/clipboard"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/textsan"
	"github.com/spf13/cobra"
)

type sessionOptions struct {
	sessionID string
	clipboard bool
	warnings  bool
	history   bool
}

func (a *app) newSessionCmd() *cobra.Command {
	var opts sessionOptions
	cmd := &cobra.Command{
		Use:   "session [session-id]",
		Short: "Print a saved review",
		Long: "Print a review from a saved chat session. Omit the session id to " +
			"print the most recently updated session. With --warnings only the " +
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
	var sess *session.Session
	if sessionID == "" {
		sess, err = store.Latest()
		if err == nil && sess == nil {
			return fmt.Errorf("session: no saved sessions")
		}
	} else {
		sess, err = store.Load(sessionID)
	}
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
	if opts.clipboard {
		return a.copySessionToClipboard(ctx, sess, w, render, subject)
	}
	return render(w, sess.Result)
}

// copySessionToClipboard renders the session in the selected --output format
// and hands it to the platform clipboard helper, printing a one-line
// confirmation instead of the content itself. Rendering into a buffer (not a
// *os.File) makes the formatter pick the unstyled form, so the clipboard
// carries Markdown or JSON source rather than terminal escapes. subject names
// what was copied (review or warnings) in that confirmation.
func (a *app) copySessionToClipboard(ctx context.Context, sess *session.Session, w io.Writer,
	render func(io.Writer, *model.ReviewResult) error, subject string) error {
	var buf bytes.Buffer
	if err := render(&buf, sess.Result); err != nil {
		return err
	}
	copyFn := a.clipboardCopy
	if copyFn == nil {
		copyFn = clipboard.Copy
	}
	helper, err := copyFn(ctx, buf.Bytes())
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Copied %s of session %s to the clipboard (%d bytes) via %s.\n",
		subject, textsan.StripControl(sess.ID), buf.Len(), helper); err != nil {
		// The clipboard already holds the content; a confirmation that could not be
		// written (closed pipe, full disk) is not a failed copy, so warn instead of
		// reporting the command as failed.
		a.warnf("session: could not print the clipboard confirmation: %v", err)
	}
	return nil
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
