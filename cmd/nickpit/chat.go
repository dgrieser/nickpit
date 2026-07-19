package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/review"
	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/styleguide"
	"github.com/spf13/cobra"
)

type chatOptions struct {
	sessionID       string
	findingID       string
	fromJSON        string
	gitlab          bool
	rawURL          string
	repo            string
	mrID            int
	reviewID        string
	repoRoot        string
	replyDiscussion string
	replyNote       int
}

func (a *app) newChatCmd() *cobra.Command {
	var opts chatOptions
	cmd := &cobra.Command{
		Use:   "chat [question]",
		Short: "Discuss a review or a single finding with an agent",
		Long: "Start or resume a conversation about a completed review. With no " +
			"question argument and an interactive terminal, chat runs a REPL; with a " +
			"question it answers once and exits. Sessions are cached so they can be " +
			"resumed with --session.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runChat(cmd.Context(), opts, args)
		},
	}
	cmd.Flags().StringVar(&opts.sessionID, "session", "", "Resume an existing session by id")
	cmd.Flags().StringVar(&opts.findingID, "finding", "", "Focus the conversation on a specific finding id")
	cmd.Flags().StringVar(&opts.fromJSON, "from-json", "", "Start a session from a saved review JSON file")
	cmd.Flags().BoolVar(&opts.gitlab, "gitlab", false, "Start a session from a GitLab merge request (use with --url or --repo/--id)")
	cmd.Flags().StringVar(&opts.rawURL, "url", "", "GitLab merge request URL (with --gitlab)")
	cmd.Flags().StringVar(&opts.repo, "repo", "", "GitLab project group/name (with --gitlab)")
	cmd.Flags().IntVar(&opts.mrID, "id", 0, "GitLab merge request IID (with --gitlab)")
	cmd.Flags().StringVar(&opts.reviewID, "review-id", "", "Select a specific review when the MR carries more than one")
	cmd.Flags().StringVar(&opts.repoRoot, "repo-root", "", "Local checkout root the retrieval tools read from (defaults to the current directory for local sessions)")
	cmd.Flags().StringVar(&opts.replyDiscussion, "reply-discussion", "", "GitLab discussion id to answer in-thread: read the thread, run one discussion turn, and post the reply back to the MR (implies --gitlab; non-interactive)")
	cmd.Flags().IntVar(&opts.replyNote, "reply-note", 0, "With --reply-discussion, the triggering note id; the reply is skipped unless this note is still the latest, so racing or redelivered replies do not double-answer")
	return cmd
}

func (a *app) runChat(ctx context.Context, opts chatOptions, args []string) error {
	profileName, profile, err := a.loadProfileForSpec()
	if err != nil {
		return err
	}
	// Non-interactive GitLab thread-reply mode: read a discussion, answer it, and
	// post the reply back into the thread. This is what the serve daemon spawns,
	// and it is directly runnable from the terminal too.
	if opts.replyDiscussion != "" {
		return a.runChatGitLabReply(ctx, profile, opts)
	}
	store, err := session.NewStore(a.sessionDir)
	if err != nil {
		return err
	}
	logger := logging.New(os.Stderr, a.verbose, isTerminal(os.Stderr))
	logger.SetShowReasoning(a.showReasoning)
	logger.SetShowProgress(a.showProgress)
	// a.logf routes through a.logger; without this assignment every save-failure
	// warning in this command would be silently dropped.
	a.logger = logger

	sess, created, err := a.resolveChatSession(ctx, store, profile, opts)
	if err != nil {
		return err
	}
	if sess.Result == nil {
		return fmt.Errorf("chat: session has no review to discuss")
	}
	// A session records the profile its review ran with; resume under that
	// profile (model, key, tokens, filters, styleguides) so the chat matches the
	// review being discussed — unless the user explicitly passed --profile. A
	// stored profile that no longer exists falls back to the active one with a
	// warning rather than stranding the session.
	if !created && !a.profileSet && sess.Profile != "" && sess.Profile != profileName {
		if storedName, stored, err := a.loadProfileNamed(sess.Profile); err != nil {
			a.logf(ctx, "chat: stored profile %q unavailable, using %q: %v", sess.Profile, profileName, err)
		} else {
			profileName, profile = storedName, stored
		}
	}
	// The session records the model the review actually ran with (including any
	// --model override at review time, or a profile model that has since
	// changed). Reproduce it unless the user explicitly overrides --model for
	// this chat.
	if a.model == "" && sess.Model != "" {
		profile.Model = sess.Model
	}
	if opts.findingID != "" {
		sess.PinnedFindingID = opts.findingID
	}
	// --repo-root applies to resumed sessions too: it is the only way to point
	// the retrieval tools at a checkout for a resumed remote session.
	if opts.repoRoot != "" {
		sess.Source.RepoRoot = opts.repoRoot
	}
	if sess.Profile == "" {
		sess.Profile = profileName
	}

	// Build the engine first so context resolution goes through the same
	// preparation pipeline a review uses (path/content filters, generated-file
	// stamping, toolchain capture, context-budget trimming) rather than a raw
	// source fetch that would leak withheld files and blow the context budget.
	source, retrievalEngine, err := a.chatSource(profile, sess.Source)
	if err != nil {
		return err
	}
	engine, err := a.chatEngine(ctx, profile, source, retrievalEngine, logger)
	if err != nil {
		return err
	}

	reviewCtx, refreshed, err := a.chatContext(ctx, engine, source, profile, sess)
	if err != nil {
		return err
	}

	// Tools read from a real checkout only; without one, an empty root would
	// resolve to the process working directory and expose unrelated files.
	tools := chatToolset(sess.Source.RepoRoot)

	if created || refreshed {
		if err := store.Save(sess); err != nil {
			a.logf(ctx, "chat: could not save session: %v", err)
		}
	}

	turn := func(question string) error {
		question = strings.TrimSpace(question)
		if question == "" {
			return nil
		}
		sess.Append(session.UserMessage(question))
		res, err := engine.Discuss(ctx, review.DiscussRequest{
			ReviewCtx:                reviewCtx,
			Result:                   sess.Result,
			PinnedFindingID:          sess.PinnedFindingID,
			Messages:                 sess.Conversation(),
			RepoRoot:                 sess.Source.RepoRoot,
			Tools:                    tools,
			DiffFormat:               profile.DiffFormat,
			DisableSuggestions:       profile.DisableSuggestions,
			DisableParallelToolCalls: a.disableParallelToolCalls,
			MaxToolCalls:             profile.MaxToolCalls,
			MaxDuplicateToolCalls:    profile.MaxDuplicateToolCalls,
			MaxOutputRetries:         profile.MaxOutputRetries,
			MaxReasoningSeconds:      profile.MaxReasoningSeconds,
		})
		if err != nil {
			// Drop the unanswered question so a retried turn does not leave two
			// consecutive user messages in the transcript.
			sess.Messages = sess.Messages[:len(sess.Messages)-1]
			return err
		}
		for _, m := range res.NewMessages {
			sess.Append(session.FromLLM(m))
		}
		if err := store.Save(sess); err != nil {
			a.logf(ctx, "chat: could not save session: %v", err)
		}
		fmt.Fprintln(os.Stdout, res.Reply) //nolint:errcheck // stdout write; nothing actionable on failure
		return nil
	}

	question := strings.TrimSpace(strings.Join(args, " "))
	if question != "" {
		return turn(question)
	}
	if !isTerminal(os.Stdin) {
		return fmt.Errorf("chat: no question given and stdin is not a terminal; pass a question argument for one-shot use")
	}
	return a.chatREPL(sess, store, turn)
}

// chatREPL runs the interactive loop, printing session context and the pinned
// opener once before reading questions until EOF or an exit command.
func (a *app) chatREPL(sess *session.Session, store *session.Store, turn func(string) error) error {
	fmt.Fprintf(os.Stderr, "Discussing review %s", sess.ReviewID)
	if sess.Source.Repo != "" {
		fmt.Fprintf(os.Stderr, " on %s", sess.Source.Repo)
		if sess.Source.Identifier > 0 {
			fmt.Fprintf(os.Stderr, "!%d", sess.Source.Identifier)
		}
	}
	fmt.Fprintf(os.Stderr, " (session %s). Type your question, or /exit to quit.\n", sess.ID)
	if sess.PinnedFindingID != "" {
		if opener := review.DiscussOpener(sess.Result, sess.PinnedFindingID); opener != "" {
			fmt.Fprintf(os.Stderr, "\n%s\n", opener)
		}
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for {
		fmt.Fprint(os.Stderr, "\n> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "":
			continue
		case "/exit", "/quit", "/q":
			return nil
		}
		if err := turn(line); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	return scanner.Err()
}

// resolveChatSession loads or creates the session for this invocation. The bool
// reports whether a new session was created (so the caller persists it).
func (a *app) resolveChatSession(ctx context.Context, store *session.Store, profile config.Profile, opts chatOptions) (*session.Session, bool, error) {
	switch {
	case opts.sessionID != "":
		sess, err := store.Load(opts.sessionID)
		return sess, false, err
	case opts.fromJSON != "":
		sess, err := a.chatSessionFromJSON(opts)
		return sess, true, err
	case opts.gitlab:
		sess, err := a.chatSessionFromGitLab(ctx, profile, opts)
		return sess, true, err
	default:
		sess, err := store.Latest()
		if err != nil {
			return nil, false, err
		}
		if sess == nil {
			return nil, false, fmt.Errorf("chat: no saved sessions; start one with --from-json, --gitlab, or run a review first")
		}
		return sess, false, nil
	}
}

// chatSessionFromJSON builds a session from a saved review result. The result's
// own metadata (mode, repo, refs) reconstructs the source so the diff can be
// re-resolved at chat time.
func (a *app) chatSessionFromJSON(opts chatOptions) (*session.Session, error) {
	data, err := os.ReadFile(opts.fromJSON)
	if err != nil {
		return nil, fmt.Errorf("chat: reading %s: %w", opts.fromJSON, err)
	}
	var result model.ReviewResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("chat: parsing %s: %w", opts.fromJSON, err)
	}
	sess := session.New()
	sess.Result = &result
	sess.ReviewID = result.ReviewID
	sess.Model = result.Model
	sess.PinnedFindingID = opts.findingID
	sess.Source = sourceFromResult(&result, opts.repoRoot)
	return sess, nil
}

// chatSessionFromGitLab builds a session by reassembling the review from the
// hidden carrier markers on an MR's notes.
func (a *app) chatSessionFromGitLab(ctx context.Context, profile config.Profile, opts chatOptions) (*session.Session, error) {
	project, mrID, baseURL := opts.repo, opts.mrID, ""
	if strings.TrimSpace(opts.rawURL) != "" {
		var err error
		project, mrID, baseURL, err = parseGitLabMRURL(opts.rawURL)
		if err != nil {
			return nil, err
		}
	}
	if project == "" || mrID <= 0 {
		return nil, fmt.Errorf("chat --gitlab requires --url or both --repo and --id")
	}
	apiBaseURL := firstNonEmpty(baseURL, profile.GitLabBaseURL)
	adapter := glscm.NewAdapter(glscm.NewClient(apiBaseURL, profile.GitLabToken), profile.AssetBaseURL)
	reviews, err := adapter.ReviewResults(ctx, project, mrID)
	if err != nil {
		return nil, fmt.Errorf("chat: reading MR reviews: %w", err)
	}
	result, err := pickReview(reviews, opts.reviewID)
	if err != nil {
		return nil, err
	}
	sess := session.New()
	sess.Result = result
	sess.ReviewID = result.ReviewID
	sess.Model = result.Model
	sess.PinnedFindingID = opts.findingID
	sess.Source = session.Source{
		Mode:       string(model.ModeGitLab),
		Repo:       project,
		Identifier: mrID,
		// The effective host, not just the --url-parsed one, so resuming in a
		// fresh process (where a one-off --gitlab-base-url is gone) still talks
		// to the same server.
		BaseURL:  apiBaseURL,
		RepoRoot: opts.repoRoot,
	}
	return sess, nil
}

// pickReview selects one review from those reassembled on an MR. An explicit id
// wins; otherwise the NEWEST review by its carried creation timestamp is chosen
// — after a re-review the latest run is what the user wants to discuss, even
// when it has fewer findings than an older run. Reviews without a timestamp
// (markers written before timestamps existed) lose to any timestamped review;
// remaining ties fall back to most findings, then lexicographic id, for
// determinism.
func pickReview(reviews map[string]*model.ReviewResult, reviewID string) (*model.ReviewResult, error) {
	if len(reviews) == 0 {
		return nil, fmt.Errorf("chat: no nickpit review markers found on the merge request")
	}
	if reviewID != "" {
		if r, ok := reviews[reviewID]; ok {
			return r, nil
		}
		return nil, fmt.Errorf("chat: review id %q not found on the merge request", reviewID)
	}
	ids := make([]string, 0, len(reviews))
	for id := range reviews {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	best := reviews[ids[0]]
	for _, id := range ids[1:] {
		candidate := reviews[id]
		switch {
		case candidate.CreatedAt.After(best.CreatedAt):
			best = candidate
		case candidate.CreatedAt.Equal(best.CreatedAt) && len(candidate.Findings) > len(best.Findings):
			best = candidate
		}
	}
	return best, nil
}

// sourceFromResult reconstructs a session source from a review result's metadata.
// ReviewResult.BaseURL is the LLM endpoint, not the SCM API URL, so it is
// deliberately not copied here; the SCM URL is resolved from the profile at chat
// time. RepoRoot is only meaningful for a local session (a remote review's
// checkout is a temporary clone that no longer exists).
func sourceFromResult(result *model.ReviewResult, repoRoot string) session.Source {
	mode, submode := result.Mode, ""
	if idx := strings.Index(mode, ":"); idx >= 0 {
		mode, submode = mode[:idx], mode[idx+1:]
	}
	if mode == string(model.ModeLocal) && repoRoot == "" {
		if wd, err := os.Getwd(); err == nil {
			repoRoot = wd
		}
	} else if mode != string(model.ModeLocal) {
		repoRoot = "" // remote checkouts are temporary; do not persist a stale path
	}
	return session.Source{
		Mode:       mode,
		Submode:    submode,
		Repo:       result.Repo,
		Identifier: result.Identifier,
		BaseRef:    result.BaseRef,
		HeadRef:    result.HeadRef,
		RepoRoot:   repoRoot,
	}
}

// chatContext returns the review context for a session, preferring the cached
// snapshot and recreating the diff when it is missing or stale. For GitLab
// sessions the MR's live head AND diff-base SHAs are compared against the SHAs
// the cache was built at, so a resumed chat picks up new commits and also an MR
// retargeting (base moved, head unchanged — the diff still changes). Local and
// GitHub sessions use the cache when present: a local diff may no longer be
// reproducible (the working tree moved on), and GitHub staleness detection
// lands with the GitHub chat front-end. refreshed reports that the session's
// cache was updated and should be saved.
func (a *app) chatContext(ctx context.Context, engine *review.Engine, source model.ReviewSource, profile config.Profile, sess *session.Session) (reviewCtx *model.ReviewContext, refreshed bool, err error) {
	refresh := sess.Context == nil
	if adapter, ok := source.(*glscm.Adapter); ok && model.ReviewMode(sess.Source.Mode) == model.ModeGitLab &&
		sess.Source.Repo != "" && sess.Source.Identifier > 0 {
		status, err := adapter.Client().FetchMRStatusByPath(ctx, sess.Source.Repo, sess.Source.Identifier)
		switch {
		case err != nil:
			// The status probe is a freshness optimization. Without a cache the
			// error is fatal (resolution needs the same API anyway); with one,
			// fall back to the cached context.
			if sess.Context == nil {
				return nil, false, fmt.Errorf("chat: checking MR status: %w", err)
			}
			a.logf(ctx, "chat: MR status check failed, using cached context: %v", err)
		case sess.Context == nil || sess.ContextHeadSHA != status.HeadSHA || sess.ContextBaseSHA != status.BaseSHA:
			refresh = true
		}
	}
	if !refresh {
		return sess.Context, false, nil
	}
	reviewCtx, err = a.chatPrepareContext(ctx, engine, source, profile, a.chatReviewRequest(profile, sess.Source, sess.ContextOptions))
	if err != nil {
		// A refresh failure must not block the chat when a cached context exists:
		// stale-but-real context beats no conversation. Without a cache the error
		// is fatal.
		if sess.Context != nil {
			a.logf(ctx, "chat: context refresh failed, using cached context: %v", err)
			return sess.Context, false, nil
		}
		return nil, false, fmt.Errorf("chat: resolving review context: %w", err)
	}
	sess.Context = reviewCtx
	// Record the refreshed diff identity from the resolved context itself (more
	// accurate than the earlier status probe if the MR moved in between).
	if reviewCtx.DiffHeadSHA != "" || reviewCtx.DiffBaseSHA != "" {
		sess.ContextHeadSHA = reviewCtx.DiffHeadSHA
		sess.ContextBaseSHA = reviewCtx.DiffBaseSHA
	}
	return reviewCtx, true, nil
}

// chatPrepareContext prepares a review context through the review pipeline. A
// remote session normally has no checkout, but the review being discussed had
// one under the same conditions resolveRepoRoot uses — and preparation depends
// on it: content filters need file contents, and toolchain capture (which
// selects version-gated styleguides) needs manifests. So the review's checkout
// condition is mirrored here: a temporary remote checkout is prepared for the
// duration of the preparation and cleaned up immediately after, keeping the
// rebuilt context (toolchain versions, styleguide gating, filtering) aligned
// with the review being discussed. Its path never leaks into the returned
// context.
func (a *app) chatPrepareContext(ctx context.Context, engine *review.Engine, source model.ReviewSource, profile config.Profile, req model.ReviewRequest) (*model.ReviewContext, error) {
	maxToolCalls := req.MaxToolCalls
	if maxToolCalls == 0 {
		maxToolCalls = profile.MaxToolCalls
	}
	// Inverse of resolveRepoRoot's skip condition: the review skipped its
	// checkout only when nothing needed one.
	needsCheckout := req.IncludeFullFiles || hasContentFilters(req) || maxToolCalls >= 0
	tempCheckout := false
	if req.RepoRoot == "" && req.Mode != model.ModeLocal && needsCheckout {
		if remote, ok := source.(model.RemoteCheckoutSource); ok {
			spec, err := remote.ResolveCheckout(ctx, req)
			if err != nil {
				return nil, err
			}
			repoRoot, cleanup, err := git.NewCheckoutManager().Prepare(ctx, *spec, git.CheckoutOptions{
				Workdir: profile.Workdir,
				Token:   checkoutToken(req.Mode, profile),
			})
			if err != nil {
				return nil, err
			}
			if cleanup != nil {
				defer cleanup()
			}
			req.RepoRoot = repoRoot
			tempCheckout = true
		}
	}
	reviewCtx, err := engine.PrepareContext(ctx, req)
	if err != nil {
		return nil, err
	}
	if tempCheckout {
		reviewCtx.CheckoutRoot = ""
	}
	return reviewCtx, nil
}

// chatReviewRequest builds the request used to re-resolve the review context
// from a session source. When the session recorded the review's context options
// they win over the current invocation's flags and profile, so a refresh
// recreates the SAME filtered context the review used (e.g. a review run with
// --include-comments=false must not refresh with comments added back). opts is
// nil for sessions with no recorded options (built from JSON or MR markers),
// which fall back to the current configuration — the best available.
func (a *app) chatReviewRequest(profile config.Profile, src session.Source, opts *session.ContextOptions) model.ReviewRequest {
	req := model.ReviewRequest{
		Mode:         model.ReviewMode(src.Mode),
		Submode:      src.Submode,
		RepoRoot:     src.RepoRoot,
		Repo:         src.Repo,
		Identifier:   src.Identifier,
		BaseRef:      src.BaseRef,
		HeadRef:      src.HeadRef,
		MaxToolCalls: profile.MaxToolCalls,
	}
	if opts != nil {
		req.IncludeComments = opts.IncludeComments
		req.IncludeCommits = opts.IncludeCommits
		req.IncludeFullFiles = opts.IncludeFullFiles
		req.IncludePaths = opts.IncludePaths
		req.ExcludePaths = opts.ExcludePaths
		req.IncludeContent = opts.IncludeContent
		req.ExcludeContent = opts.ExcludeContent
		req.MaxContextTokens = opts.MaxContextTokens
		req.DiffFormat = model.DiffFormat(opts.DiffFormat)
	} else {
		req.IncludeComments = a.includeComments
		req.IncludeCommits = a.includeCommits
		req.IncludeFullFiles = a.includeFullFiles
		req.IncludePaths = profile.IncludePaths
		req.ExcludePaths = profile.ExcludePaths
		req.IncludeContent = profile.IncludeContent
		req.ExcludeContent = profile.ExcludeContent
		req.MaxContextTokens = profile.MaxContextTokens
		req.DiffFormat = profile.DiffFormat
	}
	if req.DiffFormat == "" {
		req.DiffFormat = profile.DiffFormat
	}
	return req
}

// chatSource builds the review source and retrieval engine for a session source.
// The SCM API URL comes from the profile (or, for GitLab, an explicit URL parsed
// from --url stored on the source) — never from ReviewResult.BaseURL, which is
// the LLM endpoint.
func (a *app) chatSource(profile config.Profile, src session.Source) (model.ReviewSource, retrieval.Engine, error) {
	switch model.ReviewMode(src.Mode) {
	case model.ModeLocal:
		root := src.RepoRoot
		if root == "" {
			wd, err := os.Getwd()
			if err != nil {
				return nil, nil, err
			}
			root = wd
		}
		return git.NewLocalSource(root), retrieval.NewLocalEngine(), nil
	case model.ModeGitLab:
		apiBaseURL := firstNonEmpty(src.BaseURL, profile.GitLabBaseURL)
		adapter := glscm.NewAdapter(glscm.NewClient(apiBaseURL, profile.GitLabToken), profile.AssetBaseURL)
		return adapter, retrieval.NewLocalEngine(), nil
	case model.ModeGitHub:
		adapter := ghscm.NewAdapter(ghscm.NewClient("", profile.GitHubToken), profile.AssetBaseURL)
		return adapter, retrieval.NewLocalEngine(), nil
	default:
		return nil, nil, fmt.Errorf("chat: unsupported session mode %q", src.Mode)
	}
}

// chatToolset returns the discussion agent's tool set for a session: all reviewer
// tools when repoRoot is a real directory the retrieval layer can read, or an
// explicit empty (disabled) set otherwise. An empty root would resolve to the
// process working directory and let tools inspect unrelated files, so tools stay
// off until a real checkout is provided (e.g. via --repo-root).
func chatToolset(repoRoot string) []llm.ToolDefinition {
	if repoRoot == "" {
		return []llm.ToolDefinition{}
	}
	if info, err := os.Stat(repoRoot); err != nil || !info.IsDir() {
		return []llm.ToolDefinition{}
	}
	return nil // nil => Discuss enables all reviewer tools
}

// chatEngine builds a review engine wired for the discussion agent, mirroring
// runReview's engine setup: rate-limit backoff, search-tool optimization, and —
// crucially — the user-configured additional styleguides, resolved from the
// current configuration so the chat's styleguide set matches what a review run
// today would use.
func (a *app) chatEngine(ctx context.Context, profile config.Profile, source model.ReviewSource, retrievalEngine retrieval.Engine, logger *logging.Logger) (*review.Engine, error) {
	additionalGuides, err := styleguide.Resolve(ctx, profile.StyleGuides, profile.Workdir)
	if err != nil {
		return nil, err
	}
	client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
	client.SetLogger(logger)
	client.SetMaxRateLimitDelay(time.Duration(profile.MaxRateLimitDelaySeconds) * time.Second)
	engine := review.NewEngine(source, client, retrievalEngine, profile)
	engine.SetLogger(logger)
	engine.SetSearchToolOptimization(!a.disableSearchToolOptimization)
	engine.SetAdditionalStyleGuides(additionalGuides)
	engine.SetDisabledStyleGuides(profile.DisableStyleGuides)
	return engine, nil
}

// runChatGitLabReply answers a GitLab discussion thread and posts the reply back
// into it. It is non-interactive and self-contained: it reads the thread, gates
// on the thread's root marker (a thread nickpit did not start is a quiet no-op),
// reassembles the review by id from the MR notes, rebuilds the context from the
// live MR, runs one discussion turn seeded with the thread, and posts the answer.
func (a *app) runChatGitLabReply(ctx context.Context, profile config.Profile, opts chatOptions) error {
	project, mrID, baseURL := opts.repo, opts.mrID, ""
	if strings.TrimSpace(opts.rawURL) != "" {
		var err error
		project, mrID, baseURL, err = parseGitLabMRURL(opts.rawURL)
		if err != nil {
			return err
		}
	}
	if project == "" || mrID <= 0 {
		return fmt.Errorf("chat --reply-discussion requires --url or both --repo and --id")
	}
	apiBaseURL := firstNonEmpty(baseURL, profile.GitLabBaseURL)
	client := glscm.NewClient(apiBaseURL, profile.GitLabToken)
	adapter := glscm.NewAdapter(client, profile.AssetBaseURL)

	logger := logging.New(os.Stderr, a.verbose, isTerminal(os.Stderr))
	logger.SetShowReasoning(a.showReasoning)
	logger.SetShowProgress(a.showProgress)
	// a.logf routes through a.logger; wire it so warnings are not dropped.
	a.logger = logger

	// Carrier markers are only encoded, not authenticated, so provenance is
	// verified against the token's own user: the bot that posted the review. A
	// failed lookup fails closed rather than risking an attacker-opened thread
	// triggering paid chat calls.
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("chat: resolving token user for thread verification: %w", err)
	}
	botUserID := user.ID

	notes, err := client.DiscussionNotes(ctx, project, mrID, opts.replyDiscussion)
	if err != nil {
		return fmt.Errorf("chat: reading thread: %w", err)
	}
	if len(notes) == 0 {
		return nil
	}
	// Quiet no-ops so the daemon can spawn this for any thread reply without
	// producing noise: the thread must have been started by the bot itself and
	// its root note must carry a nickpit marker.
	if notes[0].AuthorID != botUserID {
		return nil
	}
	reviewID, findingID, ok := reviewmd.DetectThreadReview(notes[0].Body)
	if !ok {
		return nil
	}
	reviews, err := adapter.ReviewResults(ctx, project, mrID)
	if err != nil {
		return fmt.Errorf("chat: reading MR reviews: %w", err)
	}
	result := reviews[reviewID]
	if result == nil {
		return fmt.Errorf("chat: review %q not found on MR", reviewID)
	}

	// Answer only when a user question is pending and (when a triggering note id
	// was given) that note is still the latest. When two replies race, or a
	// webhook is redelivered, or --reply-discussion is re-run manually after the
	// bot already answered, the superseded invocation bows out so the thread gets
	// exactly one answer covering the conversation instead of duplicates.
	pending, pendingOK := latestPendingNote(notes, botUserID)
	if !pendingOK {
		return nil
	}
	if opts.replyNote > 0 && pending != opts.replyNote {
		return nil
	}
	history := chatThreadToMessages(notes, botUserID)
	if len(history) == 0 {
		return nil
	}

	engine, err := a.chatEngine(ctx, profile, adapter, retrieval.NewLocalEngine(), logger)
	if err != nil {
		return err
	}
	// Prepare the context through the review pipeline (filters, trimming,
	// toolchain) rather than a raw fetch, so the chat never sees withheld files
	// or an over-budget patch.
	// chatReviewRequest carries the configured --include-comments policy; the
	// triggering thread itself is already supplied through the conversation
	// history, so no unconditional comment inclusion is needed here.
	req := a.chatReviewRequest(profile, session.Source{
		Mode:       string(model.ModeGitLab),
		Repo:       project,
		Identifier: mrID,
		RepoRoot:   opts.repoRoot,
	}, nil)
	reviewCtx, err := a.chatPrepareContext(ctx, engine, adapter, profile, req)
	if err != nil {
		return fmt.Errorf("chat: resolving MR context: %w", err)
	}

	res, err := engine.Discuss(ctx, review.DiscussRequest{
		ReviewCtx:                reviewCtx,
		Result:                   result,
		PinnedFindingID:          findingID,
		Messages:                 history,
		RepoRoot:                 opts.repoRoot,
		Tools:                    chatToolset(opts.repoRoot),
		DiffFormat:               profile.DiffFormat,
		DisableSuggestions:       profile.DisableSuggestions,
		DisableParallelToolCalls: a.disableParallelToolCalls,
		MaxToolCalls:             profile.MaxToolCalls,
		MaxDuplicateToolCalls:    profile.MaxDuplicateToolCalls,
		MaxOutputRetries:         profile.MaxOutputRetries,
		MaxReasoningSeconds:      profile.MaxReasoningSeconds,
	})
	if err != nil {
		return fmt.Errorf("chat: discussion agent: %w", err)
	}
	reply := strings.TrimSpace(res.Reply)
	if reply == "" {
		return nil
	}
	// Revalidate immediately before posting: if a newer user note arrived while
	// the LLM turn was running, posting now would answer out of order AND make
	// the newer note's child see a bot reply as the latest activity and skip —
	// permanently dropping that question. Abort instead; the newer note's child
	// answers with the full history, including the question this turn covered.
	// (A tiny window between this check and the POST remains; the SCM offers no
	// atomic check-and-post.)
	fresh, err := client.DiscussionNotes(ctx, project, mrID, opts.replyDiscussion)
	if err != nil {
		return fmt.Errorf("chat: rechecking thread before reply: %w", err)
	}
	if freshPending, stillOK := latestPendingNote(fresh, botUserID); !stillOK || freshPending != pending {
		return nil
	}
	return client.ReplyToMRDiscussionPath(ctx, project, mrID, opts.replyDiscussion, reviewmd.Sanitize(reply))
}

// latestPendingNote returns the id of the thread's newest non-system note when
// that note is a user's — i.e. a question is pending an answer. ok=false when
// the newest activity is the bot's own reply (the pending question was already
// answered; answering again would duplicate) or the thread has no user notes.
func latestPendingNote(notes []glscm.DiscussionNote, botUserID int) (noteID int, ok bool) {
	for _, note := range slices.Backward(notes) {
		if note.System {
			continue
		}
		if botUserID != 0 && note.AuthorID == botUserID {
			return 0, false
		}
		return note.ID, true
	}
	return 0, false
}

// chatThreadToMessages maps a GitLab discussion's notes to conversation messages
// for the discussion agent. The root note (the finding or summary comment) is
// skipped — it is the review context, represented by the agent's own opener. The
// bot's own prior replies (author == botUserID) become assistant turns; everyone
// else's notes become user turns. System notes are dropped.
func chatThreadToMessages(notes []glscm.DiscussionNote, botUserID int) []llm.Message {
	var msgs []llm.Message
	for i, note := range notes {
		if i == 0 || note.System {
			continue
		}
		body := strings.TrimSpace(note.Body)
		if body == "" {
			continue
		}
		role := "user"
		if botUserID != 0 && note.AuthorID == botUserID {
			role = "assistant"
		}
		msgs = append(msgs, llm.Message{Role: role, Content: body})
	}
	return msgs
}

// persistChatSession saves a resumable chat session after a review, unless
// disabled with --no-session. It is best-effort and never fails the review.
// reviewCtx is the prepared context the pipeline actually reviewed; caching it
// gives an exact-context resume even when the diff is no longer reproducible
// (e.g. a local uncommitted review whose working tree moved on). headSHA records
// the remote head the context was built at, so a later chat can detect new
// commits and recreate the diff.
func (a *app) persistChatSession(ctx context.Context, profile config.Profile, req model.ReviewRequest, result *model.ReviewResult, reviewCtx *model.ReviewContext, headSHA string) {
	if a.noSession || result == nil {
		return
	}
	if len(result.Findings) == 0 && strings.TrimSpace(result.OverallExplanation) == "" {
		return
	}
	store, err := session.NewStore(a.sessionDir)
	if err != nil {
		a.logf(ctx, "chat: session store unavailable: %v", err)
		return
	}
	sess := session.New()
	sess.ReviewID = result.ReviewID
	sess.Result = result
	sess.Model = result.Model
	sess.Profile = req.ProfileName
	if reviewCtx != nil {
		ctxCopy := *reviewCtx
		if req.Mode != model.ModeLocal {
			// The remote checkout is a temporary clone deleted right after this
			// call; do not leak its path into the cached context.
			ctxCopy.CheckoutRoot = ""
		}
		sess.Context = &ctxCopy
		// Record the exact diff identity the cache was built at — the context's
		// own SHAs when the source provided them (GitLab), else the checkout
		// head. Persisting BOTH keeps the first resume from treating a perfectly
		// fresh cache as stale (chatContext compares head and base).
		sess.ContextHeadSHA = firstNonEmpty(reviewCtx.DiffHeadSHA, headSHA)
		sess.ContextBaseSHA = reviewCtx.DiffBaseSHA
	}
	// Record the review's context-shaping options so a later refresh recreates
	// the same filtered context instead of whatever the then-current flags say.
	sess.ContextOptions = &session.ContextOptions{
		IncludeComments:  req.IncludeComments,
		IncludeCommits:   req.IncludeCommits,
		IncludeFullFiles: req.IncludeFullFiles,
		IncludePaths:     req.IncludePaths,
		ExcludePaths:     req.ExcludePaths,
		IncludeContent:   req.IncludeContent,
		ExcludeContent:   req.ExcludeContent,
		MaxContextTokens: req.MaxContextTokens,
		DiffFormat:       string(req.DiffFormat),
	}
	// RepoRoot is persisted only for a local review: a remote review's RepoRoot is
	// a temporary clone deleted right after this call, so a resumed session must
	// not point retrieval tools at it. Source.BaseURL is the EFFECTIVE SCM API
	// host the review talked to (profile.GitLabBaseURL already carries any --url
	// or --gitlab-base-url override) — persisting it keeps a resumed chat in a
	// fresh process from falling back to the config host and sending the GitLab
	// token to the wrong server. It is never result.BaseURL, the LLM endpoint.
	repoRoot := ""
	if req.Mode == model.ModeLocal {
		repoRoot = req.RepoRoot
	}
	scmBaseURL := ""
	if req.Mode == model.ModeGitLab {
		scmBaseURL = profile.GitLabBaseURL
	}
	sess.Source = session.Source{
		Mode:       string(req.Mode),
		Submode:    req.Submode,
		Repo:       req.Repo,
		Identifier: req.Identifier,
		BaseRef:    req.BaseRef,
		HeadRef:    req.HeadRef,
		BaseURL:    scmBaseURL,
		RepoRoot:   repoRoot,
	}
	if err := store.Save(sess); err != nil {
		a.logf(ctx, "chat: could not save session: %v", err)
		return
	}
	a.logf(ctx, "chat: session saved: id=%s (resume with `nickpit chat --session %s`)", sess.ID, sess.ID)
}
