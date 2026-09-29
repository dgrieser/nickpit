package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/scm/forge"
	"github.com/dgrieser/nickpit/internal/scm/forges"
	"github.com/spf13/cobra"
)

// forgeFlags holds the per-platform persistent flags: --<mode>-token for every
// platform and --<mode>-base-url for the self-hostable ones. The values are
// pointers because cobra binds each flag to one addressable string.
type forgeFlags struct {
	tokens   map[model.ReviewMode]*string
	baseURLs map[model.ReviewMode]*string
}

// registerForgeFlags adds the credential flags of every known platform to the
// root command, keeping the flag names the platforms have always had.
func (a *app) registerForgeFlags(root *cobra.Command) {
	a.forgeFlags = forgeFlags{
		tokens:   map[model.ReviewMode]*string{},
		baseURLs: map[model.ReviewMode]*string{},
	}
	for _, f := range forges.All {
		token := new(string)
		a.forgeFlags.tokens[f.Mode()] = token
		root.PersistentFlags().StringVar(token, string(f.Mode())+"-token", "", f.Name()+" token override")
		if f.ConfigurableBaseURL() {
			baseURL := new(string)
			a.forgeFlags.baseURLs[f.Mode()] = baseURL
			root.PersistentFlags().StringVar(baseURL, string(f.Mode())+"-base-url", "", f.Name()+" API base URL")
		}
	}
}

// forgeBaseURL is the API base URL override in force for a platform: the
// --<mode>-base-url flag, or the host a --url of this invocation named.
func (a *app) forgeBaseURL(mode model.ReviewMode) string {
	if value := a.forgeFlags.baseURLs[mode]; value != nil {
		return *value
	}
	return ""
}

// setForgeBaseURL records a base URL override chosen in this invocation, so
// the profile takes it like every other setting.
func (a *app) setForgeBaseURL(mode model.ReviewMode, baseURL string) {
	if a.forgeFlags.baseURLs == nil {
		a.forgeFlags.baseURLs = map[model.ReviewMode]*string{}
	}
	if value := a.forgeFlags.baseURLs[mode]; value != nil {
		*value = baseURL
		return
	}
	a.forgeFlags.baseURLs[mode] = &baseURL
}

// forgeOverrides renders the platform flags as config overrides.
func (a *app) forgeOverrides() (tokens, baseURLs map[model.ReviewMode]string) {
	tokens = map[model.ReviewMode]string{}
	for mode, value := range a.forgeFlags.tokens {
		if value != nil && *value != "" {
			tokens[mode] = *value
		}
	}
	baseURLs = map[model.ReviewMode]string{}
	for mode, value := range a.forgeFlags.baseURLs {
		if value != nil && *value != "" {
			baseURLs[mode] = *value
		}
	}
	return tokens, baseURLs
}

// requireBaseURL fails a self-hosted platform that has no instance configured:
// GitLab defaults to gitlab.com, but Forgejo has no public default, so its
// commands need the host from the profile, the environment, a flag, or --url.
func requireBaseURL(f forge.Forge, baseURL string) error {
	if !f.ConfigurableBaseURL() || baseURL != "" {
		return nil
	}
	mode := string(f.Mode())
	return fmt.Errorf("%s: no API base URL configured; set %s_base_url in the profile, NICKPIT_%s_BASE_URL, --%s-base-url, or pass --url",
		f.Name(), mode, strings.ToUpper(mode), mode)
}

// checkoutCredentials renders the profile's token for the platform serving
// mode as the basic-auth pair a remote checkout clones with, empty when the
// mode has no platform or no token.
func checkoutCredentials(mode model.ReviewMode, profile config.Profile) string {
	f, ok := forges.All.Lookup(mode)
	if !ok {
		return ""
	}
	token, _ := forges.Credentials(f, profile)
	return f.GitCredentials(token)
}

// requestSigil is how the platform serving mode writes a request number, "!"
// for a merge request and "#" for a pull request.
func requestSigil(mode model.ReviewMode) string {
	if f, ok := forges.All.Lookup(mode); ok {
		return f.RequestSigil()
	}
	return "#"
}

// newForgeCmd builds the command tree of one platform: `<command>
// <request-command>` reviews a request addressed by --repo/--id, by --url, or
// picked from the open ones. Platform-only subcommands (the GitLab daemon and
// templates) are added by the caller.
func (a *app) newForgeCmd(f forge.Forge) *cobra.Command {
	var repo string
	var id int
	var rawURL string
	var publish bool
	var pick bool
	help := f.RequestHelp()
	cmd := &cobra.Command{
		Use:   f.Command(),
		Short: fmt.Sprintf("Review %s %ss", f.Name(), f.RequestNoun()),
	}
	requestCmd := &cobra.Command{
		Use:   f.RequestCommand(),
		Short: help.Short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := a.resolveRequestTarget(requestSelectors{
				repo:    repo,
				id:      id,
				rawURL:  rawURL,
				pick:    pick,
				changed: cmd.Flags().Changed,
			}, f.ParseRequestURL, "", f.RequestNoun())
			if err != nil {
				return err
			}
			repo = target.Repo
			// Set before loading the profile: the profile takes the URL host as
			// a CLI override, so a --url on another host reaches the client
			// through the profile like every other setting.
			if target.BaseURL != "" && f.ConfigurableBaseURL() {
				a.setForgeBaseURL(f.Mode(), target.BaseURL)
			}
			profileName, profile, err := a.loadProfileForSpec()
			if err != nil {
				return err
			}
			token, baseURL := forges.Credentials(f, profile)
			if err := requireBaseURL(f, baseURL); err != nil {
				return err
			}
			source := f.NewSource(baseURL, token, profile.AssetBaseURL)
			if target.ID == 0 {
				if target.ID, err = a.pickOpenRequest(cmd.Context(), target.Repo, openRequestList{
					noun:   f.RequestNoun(),
					marker: f.RequestSigil(),
					list: func(ctx context.Context) ([]model.OpenRequest, error) {
						return source.ListOpenRequests(ctx, target.Repo)
					},
				}); err != nil {
					return err
				}
			}
			id = target.ID
			req := model.ReviewRequest{
				Mode:                      f.Mode(),
				Workdir:                   profile.Workdir,
				Repo:                      repo,
				Identifier:                id,
				IncludeComments:           a.includeComments,
				IncludeCommits:            a.includeCommits,
				IncludeFullFiles:          a.includeFullFiles,
				IncludePaths:              profile.IncludePaths,
				ExcludePaths:              profile.ExcludePaths,
				IncludeContent:            profile.IncludeContent,
				ExcludeContent:            profile.ExcludeContent,
				DiffFormat:                profile.DiffFormat,
				MaxContextTokens:          profile.MaxContextTokens,
				MaxToolCalls:              profile.MaxToolCalls,
				MaxDuplicateToolCalls:     profile.MaxDuplicateToolCalls,
				MaxOutputRetries:          profile.MaxOutputRetries,
				MaxReasoningSeconds:       profile.MaxReasoningSeconds,
				NudgeCount:                profile.NudgeCount,
				MaxFindings:               profile.MaxFindings,
				DisablePatchSummary:       profile.DisablePatchSummary,
				DisableSuggestions:        profile.DisableSuggestions,
				DisableJSONResponseFormat: profile.DisableJSONResponseFormat,
				PriorityThreshold:         a.priorityThreshold,
				ConfidenceThreshold:       a.confidenceThreshold,
				PostReview:                publish,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	requestCmd.Flags().StringVar(&repo, "repo", "", help.Repo)
	requestCmd.Flags().IntVar(&id, "id", 0, help.ID)
	requestCmd.Flags().StringVar(&rawURL, "url", "", fmt.Sprintf("%s %s URL", f.Name(), f.RequestNoun()))
	addSelectFlag(requestCmd, &pick, "an open "+f.RequestNoun(), requestSelectNote)
	requestCmd.Flags().BoolVar(&publish, "publish", false, help.Publish)
	cmd.AddCommand(requestCmd)
	return cmd
}

// forgeRequestNoun is the platform's word for a change under review.
func forgeRequestNoun(mode model.ReviewMode) string {
	if f, ok := forges.All.Lookup(mode); ok {
		return f.RequestNoun()
	}
	return "request"
}

// forgeRequestLabel names a request kind the way its platform writes it,
// "GitLab MR" or "GitHub PR".
func forgeRequestLabel(mode model.ReviewMode) string {
	if f, ok := forges.All.Lookup(mode); ok {
		return f.Name() + " " + f.RequestAbbrev()
	}
	return string(mode)
}
