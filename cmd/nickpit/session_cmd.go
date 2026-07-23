package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dgrieser/nickpit/internal/session"
	"github.com/spf13/cobra"
)

type sessionOptions struct {
	sessionID string
}

func (a *app) newSessionCmd() *cobra.Command {
	var opts sessionOptions
	cmd := &cobra.Command{
		Use:   "session [session-id]",
		Short: "Print a saved review",
		Long: "Print a review from a saved chat session. Omit the session id to " +
			"print the most recently updated session.",
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

func (a *app) runSession(_ context.Context, opts sessionOptions, args []string) error {
	return a.runSessionTo(opts, args, os.Stdout)
}

func (a *app) runSessionTo(opts sessionOptions, args []string, w io.Writer) error {
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
	return a.formatReview(w, sess.Result)
}
