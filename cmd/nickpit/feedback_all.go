package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/pick"
	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// feedbackProbeConcurrency bounds how many open requests are read at once when
// the merged list checks which of them actually carry a review. It is not the
// LLM concurrency limit (--concurrency): these are small API reads, and the
// forges rate-limit them per token, so the cap stays modest and fixed.
const feedbackProbeConcurrency = 8

type mergedFeedbackOptions struct {
	clipboard bool
	list      bool
}

// newFeedbackCmd is `nickpit feedback`: every review this checkout can show —
// published on its merge requests and pull requests, or saved locally for the
// branch — in one list.
func (a *app) newFeedbackCmd() *cobra.Command {
	var opts mergedFeedbackOptions
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Choose a review to print from everything this checkout has",
		Long: "Collect the reviews NickPit has for this checkout and print the chosen one. " +
			"The git remotes decide where to look: open merge requests on the GitLab instance a " +
			"remote points at, open pull requests on GitHub, and the reviews saved locally for the " +
			"checked-out branch. Only requests that really carry a published review are listed, so " +
			"every row can be printed; reading them costs one API call per open request. Use " +
			"`nickpit gitlab feedback`, `nickpit github feedback` or `nickpit git feedback` to " +
			"address one directly. Read-only: nothing is posted or changed.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runMergedFeedback(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.list, "list", false, "List the reviews found instead of choosing one")
	cmd.Flags().BoolVar(&opts.clipboard, "clipboard", false, "Copy the review to the system clipboard instead of printing it (uses the platform clipboard helper: pbcopy, clip.exe, wl-copy, xclip, xsel, or termux-clipboard-set)")
	cmd.MarkFlagsMutuallyExclusive("list", "clipboard")
	return cmd
}

// feedbackCandidate is one row of the merged list: a review that exists and can
// be printed, wherever it came from.
type feedbackCandidate struct {
	// platform is the forge the review was published on, platformNone for a
	// review saved locally.
	platform scmPlatform
	// identifier is how the platform writes the request ("!42", "#7"); a local
	// review has none and shows its submode instead.
	identifier string
	// title is the request title, or the revision range of a local review.
	title  string
	author string
	// reviewedAt is when the review was produced, which is what the list sorts
	// on: the newest feedback is the one usually wanted.
	reviewedAt time.Time
	findings   int
	verdict    string
	// origin names the review's source in the clipboard confirmation.
	origin string
	// current marks a request whose source branch is the checked-out one.
	current bool
	// result is the review to print. Remote rows carry it already — they were
	// read to prove the row has one — while a local row loads it from its
	// session when chosen, so the merged list never decodes every session file.
	result    *model.ReviewResult
	sessionID string
}

// kind is the column that says where a row came from.
func (c feedbackCandidate) kind() string {
	if name := c.platform.name(); name != "" {
		return name
	}
	return "local"
}

func (a *app) runMergedFeedback(ctx context.Context, w io.Writer, opts mergedFeedbackOptions) error {
	profile, err := a.loadProfileWithoutLLM()
	if err != nil {
		return err
	}
	candidates, problems := a.collectFeedback(ctx, profile)
	if len(candidates) == 0 {
		return fmt.Errorf("feedback: %s", noFeedbackMessage(problems))
	}
	for _, problem := range problems {
		a.warnf("feedback: %v", problem)
	}
	if opts.list {
		return a.formatFeedbackList(w, candidates)
	}
	if !a.interactiveSelect() {
		return fmt.Errorf("feedback: choosing a review needs a terminal; pass --list, or address one with "+
			"`nickpit git feedback --session <id>`, `nickpit gitlab feedback --id <iid>` or "+
			"`nickpit github feedback --id <number>` (%d review(s) found)", len(candidates))
	}
	chosen, err := a.pickFeedback(candidates)
	if err != nil {
		return err
	}
	if chosen.result == nil {
		store, err := session.NewStore(a.sessionDir)
		if err != nil {
			return err
		}
		return a.emitLocalReview(ctx, w, store, chosen.sessionID, opts.clipboard)
	}
	if err := a.emitReview(ctx, w, opts.clipboard, "review", chosen.origin, func(out io.Writer) error {
		return a.formatReview(out, chosen.result)
	}); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	return nil
}

// collectFeedback gathers every printable review for this checkout, newest
// first, together with the reasons a source contributed nothing. A source that
// fails never hides the others: a GitLab token that cannot list merge requests
// still leaves the local reviews and the GitHub ones listed.
func (a *app) collectFeedback(ctx context.Context, profile config.Profile) ([]feedbackCandidate, []error) {
	var candidates []feedbackCandidate
	var problems []error

	local, err := a.localFeedback(ctx)
	if err != nil {
		problems = append(problems, err)
	}
	candidates = append(candidates, local...)

	branch := currentBranchName(ctx)
	for _, remote := range dedupeRemotes(a.currentCheckoutRemotes(ctx)) {
		found, err := a.remoteFeedback(ctx, profile, remote, branch)
		if err != nil {
			problems = append(problems, err)
		}
		candidates = append(candidates, found...)
	}
	sortFeedback(candidates)
	return candidates, problems
}

// sortFeedback orders the merged rows by when their review was produced,
// newest first — the one usually wanted, wherever it came from.
func sortFeedback(candidates []feedbackCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].reviewedAt.After(candidates[j].reviewedAt)
	})
}

// dedupeRemotes keeps one entry per project a platform knows: a fork and its
// upstream are different projects, but "origin" and a second remote pointing
// at the same one are not, and probing it twice would list every review twice.
func dedupeRemotes(remotes []checkoutRemote) []checkoutRemote {
	var unique []checkoutRemote
	seen := map[string]bool{}
	for _, remote := range remotes {
		if remote.platform == platformNone || remote.repo == "" {
			continue
		}
		key := string(remote.platform) + " " + remote.host + " " + strings.ToLower(remote.repo)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, remote)
	}
	return unique
}

// localFeedback lists the saved reviews of the checked-out branch as rows.
func (a *app) localFeedback(ctx context.Context) ([]feedbackCandidate, error) {
	target, err := currentCheckout(ctx)
	if err != nil {
		return nil, err
	}
	store, err := session.NewStore(a.sessionDir)
	if err != nil {
		return nil, err
	}
	infos, err := store.List()
	if err != nil {
		return nil, err
	}
	matching, _ := localReviewsFor(infos, target)
	candidates := make([]feedbackCandidate, 0, len(matching))
	for _, info := range matching {
		candidates = append(candidates, feedbackCandidate{
			identifier: info.Source.Submode,
			title:      localReviewRange(info),
			reviewedAt: reviewTime(info),
			findings:   info.Findings,
			verdict:    info.Verdict,
			origin:     "session " + textsan.StripControl(info.ID),
			// A local review is of the working tree in front of you, so it is
			// always the branch you are on.
			current:   true,
			sessionID: info.ID,
		})
	}
	return candidates, nil
}

// remoteFeedback lists the open requests of one remote's project that carry a
// published review.
func (a *app) remoteFeedback(ctx context.Context, profile config.Profile, remote checkoutRemote,
	branch string) ([]feedbackCandidate, error) {
	source, err := a.requestFeedbackSource(profile, remote)
	if err != nil {
		return nil, err
	}
	return probeRequests(ctx, source, remote, branch)
}

// probeRequests turns the open requests of one project into rows, keeping the
// ones that actually carry a published review. The requests are read
// concurrently: the list is only useful once every row is known to be
// printable, and that is one API read per request.
func probeRequests(ctx context.Context, source requestFeedbackSource, remote checkoutRemote,
	branch string) ([]feedbackCandidate, error) {
	requests, err := source.list(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing open %ss of %s: %w", source.noun, remote.repo, err)
	}
	results := make([]*feedbackCandidate, len(requests))
	failures := make([]error, len(requests))
	var wg sync.WaitGroup
	slots := make(chan struct{}, feedbackProbeConcurrency)
	for i, request := range requests {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			reviews, err := source.reviews(ctx, request.Identifier)
			if err != nil {
				failures[i] = fmt.Errorf("reading the reviews on %s%s%d: %w",
					remote.repo, source.marker, request.Identifier, err)
				return
			}
			result, err := pickReview(reviews, "", source.noun)
			if err != nil {
				// No review on this request: not a failure, just not a row.
				return
			}
			results[i] = &feedbackCandidate{
				platform:   remote.platform,
				identifier: source.marker + strconv.Itoa(request.Identifier),
				title:      request.Title,
				author:     request.Author,
				reviewedAt: reviewTimeOf(result, request.UpdatedAt),
				findings:   len(result.Findings),
				verdict:    result.OverallCorrectness,
				origin: fmt.Sprintf("%s %s %s%s%d", remote.platform.name(), source.shortNoun,
					textsan.StripControl(remote.repo), source.marker, request.Identifier),
				current: branch != "" && request.SourceBranch == branch,
				result:  result,
			}
		})
	}
	wg.Wait()
	var candidates []feedbackCandidate
	var problems []error
	for i := range requests {
		if failures[i] != nil {
			problems = append(problems, failures[i])
		}
		if results[i] != nil {
			candidates = append(candidates, *results[i])
		}
	}
	if len(problems) > 0 {
		return candidates, fmt.Errorf("%d of %d open %ss of %s could not be read, first: %w",
			len(problems), len(requests), source.noun, remote.repo, problems[0])
	}
	return candidates, nil
}

// requestFeedbackSource is one project's half of the merged list: how to list
// its open requests, how to read the reviews on one, and how the platform
// writes them.
type requestFeedbackSource struct {
	noun      string
	shortNoun string
	marker    string
	list      func(context.Context) ([]model.OpenRequest, error)
	reviews   func(context.Context, int) (map[string]*model.ReviewResult, error)
}

// requestFeedbackSource builds the platform client for a remote. A GitLab
// remote pointing at another instance than the configured one is talked to at
// its own host: the project lives there, and its API is the only place its
// reviews can be read from.
func (a *app) requestFeedbackSource(profile config.Profile, remote checkoutRemote) (requestFeedbackSource, error) {
	switch remote.platform {
	case platformGitHub:
		client := ghscm.NewClient("", profile.GitHubToken)
		adapter := ghscm.NewAdapter(client, profile.AssetBaseURL)
		return requestFeedbackSource{
			noun: "pull request", shortNoun: "PR", marker: "#",
			list: func(ctx context.Context) ([]model.OpenRequest, error) {
				return client.ListOpenPRs(ctx, remote.repo)
			},
			reviews: func(ctx context.Context, id int) (map[string]*model.ReviewResult, error) {
				return adapter.ReviewResults(ctx, remote.repo, id)
			},
		}, nil
	case platformGitLab:
		baseURL := profile.GitLabBaseURL
		if host := a.gitlabAPIHost(); host == "" || normalizeHost(remote.host) != normalizeHost(host) {
			baseURL = "https://" + remote.host
		}
		client := glscm.NewClient(baseURL, profile.GitLabToken)
		adapter := glscm.NewAdapter(client, profile.AssetBaseURL)
		return requestFeedbackSource{
			noun: "merge request", shortNoun: "MR", marker: "!",
			list: func(ctx context.Context) ([]model.OpenRequest, error) {
				return client.ListOpenMRs(ctx, remote.repo)
			},
			reviews: func(ctx context.Context, id int) (map[string]*model.ReviewResult, error) {
				return adapter.ReviewResults(ctx, remote.repo, id)
			},
		}, nil
	default:
		return requestFeedbackSource{}, fmt.Errorf("remote %q points at no known platform", remote.name)
	}
}

// reviewTimeOf is when a published review was produced, falling back to the
// request's last activity for a review published before results carried a
// timestamp.
func reviewTimeOf(result *model.ReviewResult, fallback time.Time) time.Time {
	if result != nil && !result.CreatedAt.IsZero() {
		return result.CreatedAt
	}
	return fallback
}

// noFeedbackMessage explains an empty merged list: the sources that failed
// come first, since "nothing found" means something different when a token was
// rejected than when the checkout simply has no reviews yet.
func noFeedbackMessage(problems []error) string {
	if len(problems) == 0 {
		return "no review found for this checkout: no open merge request or pull request carries one, " +
			"and no review of the checked-out branch is saved locally"
	}
	reasons := make([]string, 0, len(problems))
	for _, problem := range problems {
		reasons = append(reasons, problem.Error())
	}
	return "no review found for this checkout; the sources that could not be read: " + strings.Join(reasons, "; ")
}

// pickFeedback draws the merged list, newest review first, with the rows of the
// checked-out branch marked.
func (a *app) pickFeedback(candidates []feedbackCandidate) (feedbackCandidate, error) {
	items := make([]pick.Item, len(candidates))
	initial := -1
	for i, candidate := range candidates {
		mark := ""
		if candidate.current {
			mark = checkedOutMark
			if initial < 0 {
				initial = i
			}
		}
		cells := []string{
			mark,
			candidate.kind(),
			textsan.StripControl(candidate.identifier),
			textsan.StripControl(candidate.title),
			strconv.Itoa(candidate.findings) + " findings",
			relativeAge(candidate.reviewedAt),
		}
		items[i] = pick.Item{
			Cells: cells,
			Match: candidate.kind() + " " + candidate.identifier + " " + candidate.title + " " + candidate.author,
			CellStyles: rowStyles(len(cells), map[int]string{
				5: ageStyle(candidate.reviewedAt),
			}),
		}
	}
	index, err := a.selectOne(pick.Options{
		Title:   "Reviews for this checkout",
		Items:   items,
		Initial: max(initial, 0),
		// Same reading as the local list: gold for which kind of review the row
		// is, green for what addresses it, lavender for its text, blue for the
		// finding count.
		CellStyles: []string{pick.StyleMark, pick.StyleCaveat, pick.StyleIdentifier, pick.StyleText,
			pick.StyleDetail, pick.StyleAge},
		ColumnKinds: []pick.ColumnKind{pick.KindPlain, pick.KindPlain, pick.KindPlain, pick.KindMessage},
	})
	if err != nil {
		return feedbackCandidate{}, err
	}
	chosen := candidates[index]
	a.printSelection("Selected ", chosen.kind()+" "+chosen.identifier, pick.StyleIdentifier,
		" "+textsan.StripControl(chosen.title))
	return chosen, nil
}

// feedbackListEntry is one line of `--list`: where a review is, what it says,
// and which command prints it.
type feedbackListEntry struct {
	Kind       string    `json:"kind"`
	Identifier string    `json:"identifier,omitempty"`
	Title      string    `json:"title,omitempty"`
	Author     string    `json:"author,omitempty"`
	ReviewedAt time.Time `json:"reviewed_at"`
	Findings   int       `json:"findings"`
	Verdict    string    `json:"verdict,omitempty"`
	SessionID  string    `json:"session_id,omitempty"`
	ReviewID   string    `json:"review_id,omitempty"`
}

func (a *app) formatFeedbackList(w io.Writer, candidates []feedbackCandidate) error {
	entries := make([]feedbackListEntry, 0, len(candidates))
	for _, candidate := range candidates {
		entry := feedbackListEntry{
			Kind:       candidate.kind(),
			Identifier: candidate.identifier,
			Title:      candidate.title,
			Author:     candidate.author,
			ReviewedAt: candidate.reviewedAt,
			Findings:   candidate.findings,
			Verdict:    candidate.verdict,
			SessionID:  candidate.sessionID,
		}
		if candidate.result != nil {
			entry.ReviewID = candidate.result.ReviewID
		}
		entries = append(entries, entry)
	}
	if a.jsonOutput || a.outputFormat == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	}
	if _, err := fmt.Fprintf(w, "%d review(s) for this checkout, newest first.\n\n", len(entries)); err != nil {
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "WHERE\tID\tREVIEWED\tFINDINGS\tVERDICT\tTITLE"); err != nil {
		return err
	}
	for _, entry := range entries {
		reviewed := "unknown"
		if !entry.ReviewedAt.IsZero() {
			reviewed = entry.ReviewedAt.Local().Format(time.RFC3339)
		}
		identifier := entry.Identifier
		if entry.SessionID != "" {
			identifier = entry.SessionID
		}
		// Titles, verdicts and refs come from the forge or from a session file;
		// strip control characters so none of them can rewrite the table.
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%d\t%s\t%s\n",
			entry.Kind, textsan.StripControl(identifier), reviewed, entry.Findings,
			textsan.StripControl(entry.Verdict), textsan.StripControl(entry.Title)); err != nil {
			return err
		}
	}
	return table.Flush()
}
