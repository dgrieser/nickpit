package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dgrieser/nickpit/internal/clipboard"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/textsan"
	"github.com/spf13/cobra"
)

type sessionOptions struct {
	sessionID string
	clipboard bool
}

func (a *app) newSessionCmd() *cobra.Command {
	var opts sessionOptions
	cmd := &cobra.Command{
		Use:   "session [session-id]",
		Short: "Print a saved review",
		Long: "Print a review from a saved chat session. Omit the session id to " +
			"print the most recently updated session. With --clipboard the review " +
			"is copied to the system clipboard instead of printed.",
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
	cmd.Flags().BoolVar(&opts.clipboard, "clipboard", false, "Copy the review to the system clipboard instead of printing it (uses the platform clipboard helper: pbcopy, clip.exe, wl-copy, xclip, xsel, or termux-clipboard-set)")
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
	if opts.clipboard {
		return a.copyReviewToClipboard(ctx, sess, w)
	}
	return a.formatReview(w, sess.Result)
}

// copyReviewToClipboard renders the review in the selected --output format and
// hands it to the platform clipboard helper, printing a one-line confirmation
// instead of the review itself. Rendering into a buffer (not a *os.File) makes
// formatReview pick the unstyled form, so the clipboard carries Markdown or
// JSON source rather than terminal escapes.
func (a *app) copyReviewToClipboard(ctx context.Context, sess *session.Session, w io.Writer) error {
	var buf bytes.Buffer
	if err := a.formatReview(&buf, sess.Result); err != nil {
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
	if _, err := fmt.Fprintf(w, "Copied review of session %s to the clipboard (%d bytes) via %s.\n",
		textsan.StripControl(sess.ID), buf.Len(), helper); err != nil {
		// The clipboard already holds the review; a confirmation that could not be
		// written (closed pipe, full disk) is not a failed copy, so warn instead of
		// reporting the command as failed.
		a.warnf("session: could not print the clipboard confirmation: %v", err)
	}
	return nil
}
