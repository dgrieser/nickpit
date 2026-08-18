package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dgrieser/nickpit/internal/config"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/serve"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// templateTarget is one comment-template scope plus the client and command
// keyword to use for it. A serve config contributes one target per configured
// group, each carrying that group's own token, so a single run can seed every
// tenant the daemon serves.
type templateTarget struct {
	scope   glscm.SavedReplyScope
	client  *glscm.Client
	keyword string
}

// templateFlags collects the scope selectors shared by the sync and list
// subcommands.
type templateFlags struct {
	user            bool
	groups          []string
	projects        []string
	serveConfigPath string
	keyword         string
}

func (a *app) newGitLabTemplatesCmd() *cobra.Command {
	var flags templateFlags
	var prune bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage GitLab comment templates for the nickpit commands",
		// Deployment plumbing, not a review workflow: the chart's post-upgrade
		// hook Job runs the sync for every configured group, so the command
		// stays out of the user-facing command list. `--help` on it still works.
		Hidden: true,
		Long: "Seed the nickpit note commands as GitLab comment templates (\"saved replies\"), so the " +
			"comment box offers them from its template picker. GitLab has no extension point for real " +
			"quick actions: the \"/\" autocomplete is fed by the server's own command definitions, and a " +
			"registered quick action would be stripped from the note body before the webhook fires. " +
			"Templates are the supported alternative — inserting one leaves \"/nickpit review\" in the " +
			"comment. User- and group-scoped templates appear in the picker of every merge request the " +
			"scope covers.",
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Create or update the nickpit comment templates in a scope",
		Long: "Converge each selected scope on the nickpit command templates: missing ones are created " +
			"(+), drifted content is updated (~), matching ones are left alone (=), and with --prune " +
			"templates carrying the nickpit name prefix but no longer defined are deleted (-). " +
			"Templates outside that prefix are never touched.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targets, err := a.resolveTemplateTargets(flags)
			if err != nil {
				return err
			}
			return syncCommandTemplates(cmd.Context(), cmd.OutOrStdout(), targets, prune, dryRun)
		},
	}
	syncCmd.Flags().BoolVar(&prune, "prune", false, "Delete templates carrying the nickpit name prefix that are no longer defined")
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report the plan without writing anything")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List the comment templates a scope owns",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targets, err := a.resolveTemplateTargets(flags)
			if err != nil {
				return err
			}
			return listCommandTemplates(cmd.Context(), cmd.OutOrStdout(), targets)
		},
	}

	for _, sub := range []*cobra.Command{syncCmd, listCmd} {
		sub.Flags().BoolVar(&flags.user, "user", false, "Act on the token owner's own comment templates")
		sub.Flags().StringArrayVar(&flags.groups, "group", nil, "Group path whose comment templates to act on (repeatable)")
		sub.Flags().StringArrayVar(&flags.projects, "project", nil, "Project path whose comment templates to act on (repeatable)")
		sub.Flags().StringVar(&flags.serveConfigPath, "serve-config", "", "Serve daemon config file; acts on every configured group with that group's token")
		sub.Flags().StringVar(&flags.keyword, "keyword", "", "Command keyword the templates use (default: the serve config's command_keyword, else "+config.DefaultServeCommandKeyword+")")
		_ = sub.MarkFlagFilename("serve-config", "yaml", "yml")
		cmd.AddCommand(sub)
	}
	return cmd
}

// resolveTemplateTargets turns the scope selectors into clients. Explicit
// scopes use the profile's GitLab credentials; a serve config contributes its
// groups with their own per-group tokens, because a daemon token is scoped to
// the group it serves.
func (a *app) resolveTemplateTargets(flags templateFlags) ([]templateTarget, error) {
	var targets []templateTarget

	if flags.serveConfigPath != "" {
		cfg, err := config.LoadServe(flags.serveConfigPath)
		if err != nil {
			return nil, err
		}
		baseURL := cfg.GitLabBaseURL
		if a.gitlabBaseURL != "" {
			baseURL = a.gitlabBaseURL
		}
		keyword := cfg.CommandKeyword
		if flags.keyword != "" {
			keyword = flags.keyword
		}
		if keyword == "" {
			keyword = config.DefaultServeCommandKeyword
		}
		for _, group := range cfg.Groups {
			targets = append(targets, templateTarget{
				scope:   glscm.GroupSavedReplyScope(strings.Trim(group.Path, "/")),
				client:  glscm.NewClient(baseURL, group.Token),
				keyword: keyword,
			})
		}
	}

	if flags.user || len(flags.groups) > 0 || len(flags.projects) > 0 {
		// Seeding templates needs no LLM, only the GitLab credentials, so a
		// profile without a model still works — matching `inspect log`.
		_, profile, err := a.loadProfile()
		if err != nil && !config.IsMissingLLMEndpoint(err) {
			return nil, err
		}
		if profile.GitLabToken == "" {
			return nil, fmt.Errorf("no GitLab token configured; set gitlab_token in the profile, NICKPIT_GITLAB_TOKEN, or --gitlab-token")
		}
		client := glscm.NewClient(profile.GitLabBaseURL, profile.GitLabToken)
		keyword := flags.keyword
		if keyword == "" {
			keyword = config.DefaultServeCommandKeyword
		}
		if flags.user {
			targets = append(targets, templateTarget{scope: glscm.UserSavedReplyScope(), client: client, keyword: keyword})
		}
		for _, group := range flags.groups {
			targets = append(targets, templateTarget{scope: glscm.GroupSavedReplyScope(strings.Trim(group, "/")), client: client, keyword: keyword})
		}
		for _, project := range flags.projects {
			targets = append(targets, templateTarget{scope: glscm.ProjectSavedReplyScope(strings.Trim(project, "/")), client: client, keyword: keyword})
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no scope selected: pass --user, --group, --project, or --serve-config")
	}
	return targets, nil
}

func syncCommandTemplates(ctx context.Context, w io.Writer, targets []templateTarget, prune, dryRun bool) error {
	if dryRun {
		fmt.Fprintln(w, "dry run: no changes written") //nolint:errcheck // stdout write; nothing actionable on failure
	}
	for _, target := range targets {
		result, err := target.client.SyncSavedReplies(ctx, target.scope, glscm.SavedReplySyncOptions{
			Desired: serve.CommandTemplates(target.keyword),
			Prefix:  serve.CommandTemplatePrefix(target.keyword),
			Prune:   prune,
			DryRun:  dryRun,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\n", target.scope) //nolint:errcheck // stdout write; nothing actionable on failure
		printTemplateNames(w, "+", result.Created)
		printTemplateNames(w, "~", result.Updated)
		printTemplateNames(w, "=", result.Unchanged)
		printTemplateNames(w, "-", result.Pruned)
	}
	return nil
}

func printTemplateNames(w io.Writer, marker string, names []string) {
	for _, name := range names {
		fmt.Fprintf(w, "  %s %s\n", marker, textsan.StripControl(name)) //nolint:errcheck // stdout write; nothing actionable on failure
	}
}

func listCommandTemplates(ctx context.Context, w io.Writer, targets []templateTarget) error {
	for _, target := range targets {
		replies, err := target.client.SavedReplies(ctx, target.scope)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\n", target.scope) //nolint:errcheck // stdout write; nothing actionable on failure
		if len(replies) == 0 {
			fmt.Fprintln(w, "  (none)") //nolint:errcheck // stdout write; nothing actionable on failure
			continue
		}
		prefix := serve.CommandTemplatePrefix(target.keyword)
		for _, reply := range replies {
			owned := " "
			if strings.HasPrefix(reply.Name, prefix) {
				owned = "*"
			}
			fmt.Fprintf(w, "  %s %s: %s\n", owned, //nolint:errcheck // stdout write; nothing actionable on failure
				textsan.StripControl(reply.Name), textsan.StripControl(singleLine(reply.Content)))
		}
	}
	return nil
}

// singleLine flattens a template body for one-line listing; a comment template
// may hold several lines.
func singleLine(content string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(content, "\n", " ")), " ")
}
