package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// feedbackOptions are the flags of `nickpit gitlab feedback` / `nickpit github
// feedback`; the request selectors mirror the review commands so the same
// invocation that produced the review can print it back.
type feedbackOptions struct {
	repo      string
	id        int
	rawURL    string
	pick      bool
	reviewID  string
	clipboard bool
	list      bool
}

// feedbackSource is the platform-specific half of the command: where the
// reviews come from and how to name that place in output and errors.
type feedbackSource struct {
	// noun is the platform's word for the change under review: "merge request"
	// on GitLab, "pull request" on GitHub.
	noun string
	// origin names the concrete request in the clipboard confirmation, e.g.
	// "GitLab MR grp/proj!42".
	origin string
	load   func(context.Context) (map[string]*model.ReviewResult, error)
}

const feedbackLong = "Print the review NickPit published on a %s, reassembled from the hidden " +
	"markers in its own comments — no re-review, no LLM call, and no local session needed, so a " +
	"review posted by the serve daemon or from another machine is readable here too. The output is " +
	"the normal review output (`-o markdown|json|raw`); `--clipboard` copies it instead of printing " +
	"it, which is the quick way to hand the feedback to an editor or coding agent. Only markers in " +
	"comments authored by the token's own user are trusted, so forged ones are ignored. Read-only: " +
	"nothing is posted or changed on the %s."

func (a *app) newGitLabFeedbackCmd() *cobra.Command {
	var opts feedbackOptions
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Print or copy the review NickPit published on a merge request",
		Long:  fmt.Sprintf(feedbackLong, "merge request", "merge request"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := a.resolveRequestTarget(feedbackSelectors(cmd, opts), parseGitLabMRURL, "", "merge request")
			if err != nil {
				return err
			}
			// Set before loading the profile, like `gitlab mr` does: the profile
			// takes the flag/URL host as a CLI override, so a --url on another
			// host reaches the client through the profile like everything else.
			if target.BaseURL != "" {
				a.gitlabBaseURL = target.BaseURL
			}
			profile, err := a.loadProfileWithoutLLM()
			if err != nil {
				return err
			}
			client := glscm.NewClient(profile.GitLabBaseURL, profile.GitLabToken)
			project := target.Repo
			if target.ID == 0 {
				if target.ID, err = a.pickOpenRequest(cmd.Context(), project, openRequestList{
					noun:   "merge request",
					marker: "!",
					list: func(ctx context.Context) ([]model.OpenRequest, error) {
						return client.ListOpenMRs(ctx, project)
					},
				}); err != nil {
					return err
				}
			}
			mrID := target.ID
			adapter := glscm.NewAdapter(client, profile.AssetBaseURL)
			return a.runFeedback(cmd.Context(), cmd.OutOrStdout(), opts, feedbackSource{
				noun:   "merge request",
				origin: fmt.Sprintf("GitLab MR %s!%d", textsan.StripControl(project), mrID),
				load: func(ctx context.Context) (map[string]*model.ReviewResult, error) {
					return adapter.ReviewResults(ctx, project, mrID)
				},
			})
		},
	}
	addFeedbackFlags(cmd, &opts,
		"GitLab project group/name (inferred from git remote if omitted)",
		"Merge request IID (omit in a terminal to pick an open MR from a list)",
		"GitLab merge request URL",
		"merge request")
	return cmd
}

func (a *app) newGitHubFeedbackCmd() *cobra.Command {
	var opts feedbackOptions
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Print or copy the review NickPit published on a pull request",
		Long:  fmt.Sprintf(feedbackLong, "pull request", "pull request"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := a.resolveRequestTarget(feedbackSelectors(cmd, opts), parseGitHubPRURLTarget, "", "pull request")
			if err != nil {
				return err
			}
			profile, err := a.loadProfileWithoutLLM()
			if err != nil {
				return err
			}
			client := ghscm.NewClient("", profile.GitHubToken)
			repo := target.Repo
			if target.ID == 0 {
				if target.ID, err = a.pickOpenRequest(cmd.Context(), repo, openRequestList{
					noun:   "pull request",
					marker: "#",
					list: func(ctx context.Context) ([]model.OpenRequest, error) {
						return client.ListOpenPRs(ctx, repo)
					},
				}); err != nil {
					return err
				}
			}
			pr := target.ID
			adapter := ghscm.NewAdapter(client, profile.AssetBaseURL)
			return a.runFeedback(cmd.Context(), cmd.OutOrStdout(), opts, feedbackSource{
				noun:   "pull request",
				origin: fmt.Sprintf("GitHub PR %s#%d", textsan.StripControl(repo), pr),
				load: func(ctx context.Context) (map[string]*model.ReviewResult, error) {
					return adapter.ReviewResults(ctx, repo, pr)
				},
			})
		},
	}
	addFeedbackFlags(cmd, &opts,
		"GitHub repo owner/name (inferred from git remote if omitted)",
		"Pull request number (omit in a terminal to pick an open PR from a list)",
		"GitHub pull request URL",
		"pull request")
	return cmd
}

// feedbackSelectors projects the feedback flags onto the selector set every
// MR/PR-addressed command resolves the same way, carrying cobra's flag-presence
// state so an explicitly supplied default still counts as supplied.
func feedbackSelectors(cmd *cobra.Command, opts feedbackOptions) requestSelectors {
	return requestSelectors{
		repo:    opts.repo,
		id:      opts.id,
		rawURL:  opts.rawURL,
		pick:    opts.pick,
		changed: cmd.Flags().Changed,
	}
}

func addFeedbackFlags(cmd *cobra.Command, opts *feedbackOptions, repoUsage, idUsage, urlUsage, noun string) {
	cmd.Flags().StringVar(&opts.repo, "repo", "", repoUsage)
	cmd.Flags().IntVar(&opts.id, "id", 0, idUsage)
	cmd.Flags().StringVar(&opts.rawURL, "url", "", urlUsage)
	addSelectFlag(cmd, &opts.pick, "an open "+noun, requestSelectNote)
	cmd.Flags().StringVar(&opts.reviewID, "review-id", "", "Select a specific review when the "+noun+" carries more than one")
	cmd.Flags().BoolVar(&opts.list, "list", false, "List the reviews found on the "+noun+" instead of printing one")
	cmd.Flags().BoolVar(&opts.clipboard, "clipboard", false, "Copy the review to the system clipboard instead of printing it (uses the platform clipboard helper: pbcopy, clip.exe, wl-copy, xclip, xsel, or termux-clipboard-set)")
	cmd.MarkFlagsMutuallyExclusive("list", "clipboard")
	cmd.MarkFlagsMutuallyExclusive("list", "review-id")
}

// parseGitHubPRURLTarget adapts parseGitHubPRURL to the shape
// resolveRequestTarget expects. GitHub has no per-host API base URL flag, so
// the third result is always empty.
func parseGitHubPRURLTarget(raw string) (string, int, string, error) {
	repo, number, err := parseGitHubPRURL(raw)
	return repo, number, "", err
}

// loadProfileWithoutLLM loads the active profile for a command that needs only
// SCM credentials. A profile without a model or endpoint still works — nothing
// here talks to an LLM — mirroring `gitlab templates` and `inspect log`.
func (a *app) loadProfileWithoutLLM() (config.Profile, error) {
	_, profile, err := a.loadProfileForSpec()
	if err != nil && !config.IsMissingLLMEndpoint(err) {
		return config.Profile{}, err
	}
	return profile, nil
}

func (a *app) runFeedback(ctx context.Context, w io.Writer, opts feedbackOptions, source feedbackSource) error {
	reviews, err := source.load(ctx)
	if err != nil {
		return fmt.Errorf("feedback: reading published reviews: %w", err)
	}
	if opts.list {
		return a.formatReviewList(w, reviews, source.noun)
	}
	result, err := pickReview(reviews, opts.reviewID, source.noun)
	if err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	if err := a.emitReview(ctx, w, opts.clipboard, "review", source.origin, func(out io.Writer) error {
		return a.formatReview(out, result)
	}); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	return nil
}

// reviewListEntry is one line of `--list`: enough to tell the reviews on a
// request apart and to pick one with --review-id.
type reviewListEntry struct {
	ReviewID   string    `json:"review_id"`
	Revision   uint64    `json:"revision"`
	CreatedAt  time.Time `json:"created_at"`
	Findings   int       `json:"findings"`
	Verdict    string    `json:"verdict"`
	Model      string    `json:"model,omitempty"`
	NickpitVer string    `json:"nickpit_version,omitempty"`
}

// formatReviewList prints the reviews reassembled from a request, newest first
// — so the first entry is the one printed without --review-id, matching
// pickReview's choice.
func (a *app) formatReviewList(w io.Writer, reviews map[string]*model.ReviewResult, noun string) error {
	entries := reviewListEntries(reviews)
	if a.jsonOutput || a.outputFormat == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintf(w, "No complete NickPit review found on the %s.\n", noun)
		return err
	}
	if _, err := fmt.Fprintf(w, "%d review(s) on the %s, newest first; the first is printed when --review-id is omitted.\n\n",
		len(entries), noun); err != nil {
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "REVIEW ID\tPUBLISHED\tREV\tFINDINGS\tVERDICT\tMODEL\tNICKPIT"); err != nil {
		return err
	}
	for _, entry := range entries {
		published := "unknown"
		if !entry.CreatedAt.IsZero() {
			published = entry.CreatedAt.Local().Format(time.RFC3339)
		}
		// Every string here comes from a carrier marker on the request, which the
		// author check trusts but does not sanitize; strip control characters so a
		// crafted verdict cannot rewrite the table with escapes.
		if _, err := fmt.Fprintf(table, "%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
			textsan.StripControl(entry.ReviewID), published, entry.Revision, entry.Findings,
			textsan.StripControl(entry.Verdict), textsan.StripControl(entry.Model),
			textsan.StripControl(entry.NickpitVer)); err != nil {
			return err
		}
	}
	return table.Flush()
}

// reviewListEntries orders the reviews the way pickReview ranks them: newest
// first, then more findings, then by id for determinism.
func reviewListEntries(reviews map[string]*model.ReviewResult) []reviewListEntry {
	entries := make([]reviewListEntry, 0, len(reviews))
	for id, result := range reviews {
		if result == nil {
			continue
		}
		entries = append(entries, reviewListEntry{
			ReviewID:   id,
			Revision:   result.Revision,
			CreatedAt:  result.CreatedAt,
			Findings:   len(result.Findings),
			Verdict:    result.OverallCorrectness,
			Model:      result.Model,
			NickpitVer: result.NickpitVersion,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		}
		if entries[i].Findings != entries[j].Findings {
			return entries[i].Findings > entries[j].Findings
		}
		return entries[i].ReviewID < entries[j].ReviewID
	})
	return entries
}
