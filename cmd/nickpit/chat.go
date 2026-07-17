package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/session"
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

	sess, created, err := a.resolveChatSession(ctx, store, profile, opts)
	if err != nil {
		return err
	}
	if sess.Result == nil {
		return fmt.Errorf("chat: session has no review to discuss")
	}
	if opts.findingID != "" {
		sess.PinnedFindingID = opts.findingID
	}
	if sess.Profile == "" {
		sess.Profile = profileName
	}

	// Resolve the source and the review context (diff). Prefer a cached context
	// for a quick resume; otherwise resolve it now and cache it.
	source, retrievalEngine, err := a.chatSource(profile, sess.Source)
	if err != nil {
		return err
	}
	reviewCtx := sess.Context
	if reviewCtx == nil {
		reviewCtx, err = source.ResolveContext(ctx, a.chatReviewRequest(profile, sess.Source))
		if err != nil {
			return fmt.Errorf("chat: resolving review context: %w", err)
		}
		sess.Context = reviewCtx
	}

	engine := a.chatEngine(profile, source, retrievalEngine, logger)

	if created {
		if err := store.Save(sess); err != nil {
			a.logf(ctx, "chat: could not save new session: %v", err)
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
			DiffFormat:               profile.DiffFormat,
			DisableSuggestions:       profile.DisableSuggestions,
			DisableParallelToolCalls: a.disableParallelToolCalls,
			MaxToolCalls:             profile.MaxToolCalls,
			MaxDuplicateToolCalls:    profile.MaxDuplicateToolCalls,
			MaxOutputRetries:         profile.MaxOutputRetries,
			MaxReasoningSeconds:      profile.MaxReasoningSeconds,
		})
		if err != nil {
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
		BaseURL:    baseURL,
		RepoRoot:   opts.repoRoot,
	}
	return sess, nil
}

// pickReview selects one review from those reassembled on an MR. An explicit id
// wins; otherwise the review with the most findings is chosen (ties broken by id
// for determinism), which is almost always the latest full review.
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
	for _, id := range ids {
		if len(reviews[id].Findings) > len(best.Findings) {
			best = reviews[id]
		}
	}
	return best, nil
}

// sourceFromResult reconstructs a session source from a review result's metadata.
func sourceFromResult(result *model.ReviewResult, repoRoot string) session.Source {
	mode, submode := result.Mode, ""
	if idx := strings.Index(mode, ":"); idx >= 0 {
		mode, submode = mode[:idx], mode[idx+1:]
	}
	if repoRoot == "" && mode == string(model.ModeLocal) {
		if wd, err := os.Getwd(); err == nil {
			repoRoot = wd
		}
	}
	return session.Source{
		Mode:       mode,
		Submode:    submode,
		Repo:       result.Repo,
		Identifier: result.Identifier,
		BaseRef:    result.BaseRef,
		HeadRef:    result.HeadRef,
		BaseURL:    result.BaseURL,
		RepoRoot:   repoRoot,
	}
}

// chatReviewRequest builds the request used to re-resolve the review context from
// a session source.
func (a *app) chatReviewRequest(profile config.Profile, src session.Source) model.ReviewRequest {
	return model.ReviewRequest{
		Mode:            model.ReviewMode(src.Mode),
		Submode:         src.Submode,
		RepoRoot:        src.RepoRoot,
		Repo:            src.Repo,
		Identifier:      src.Identifier,
		BaseRef:         src.BaseRef,
		HeadRef:         src.HeadRef,
		IncludeComments: a.includeComments,
		IncludeCommits:  a.includeCommits,
		DiffFormat:      profile.DiffFormat,
	}
}

// chatSource builds the review source and retrieval engine for a session source.
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
	default:
		return nil, nil, fmt.Errorf("chat: unsupported session mode %q", src.Mode)
	}
}

// chatEngine builds a review engine wired for the discussion agent.
func (a *app) chatEngine(profile config.Profile, source model.ReviewSource, retrievalEngine retrieval.Engine, logger *logging.Logger) *review.Engine {
	client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
	client.SetLogger(logger)
	engine := review.NewEngine(source, client, retrievalEngine, profile)
	engine.SetLogger(logger)
	engine.SetSearchToolOptimization(!a.disableSearchToolOptimization)
	engine.SetDisabledStyleGuides(profile.DisableStyleGuides)
	return engine
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

	notes, err := client.DiscussionNoteBodies(ctx, project, mrID, opts.replyDiscussion)
	if err != nil {
		return fmt.Errorf("chat: reading thread: %w", err)
	}
	if len(notes) == 0 {
		return nil
	}
	reviewID, findingID, ok := reviewmd.DetectThreadReview(notes[0].Body)
	if !ok {
		// Not a nickpit thread: nothing to answer. Quiet no-op so the daemon can
		// spawn this for any thread reply without producing noise.
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
	reviewCtx, err := adapter.ResolveContext(ctx, model.ReviewRequest{
		Mode:            model.ModeGitLab,
		Repo:            project,
		Identifier:      mrID,
		IncludeComments: true,
		DiffFormat:      profile.DiffFormat,
	})
	if err != nil {
		return fmt.Errorf("chat: resolving MR context: %w", err)
	}

	var botUserID int
	if user, err := client.CurrentUser(ctx); err == nil {
		botUserID = user.ID
	}
	history := chatThreadToMessages(notes, botUserID)
	if len(history) == 0 {
		return nil
	}

	logger := logging.New(os.Stderr, a.verbose, isTerminal(os.Stderr))
	logger.SetShowReasoning(a.showReasoning)
	engine := a.chatEngine(profile, adapter, retrieval.NewLocalEngine(), logger)
	res, err := engine.Discuss(ctx, review.DiscussRequest{
		ReviewCtx:                reviewCtx,
		Result:                   result,
		PinnedFindingID:          findingID,
		Messages:                 history,
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
	return client.ReplyToMRDiscussionPath(ctx, project, mrID, opts.replyDiscussion, reviewmd.Sanitize(reply))
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
func (a *app) persistChatSession(ctx context.Context, req model.ReviewRequest, result *model.ReviewResult) {
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
	sess.Source = session.Source{
		Mode:       string(req.Mode),
		Submode:    req.Submode,
		Repo:       req.Repo,
		Identifier: req.Identifier,
		BaseRef:    req.BaseRef,
		HeadRef:    req.HeadRef,
		BaseURL:    result.BaseURL,
		RepoRoot:   req.RepoRoot,
	}
	if err := store.Save(sess); err != nil {
		a.logf(ctx, "chat: could not save session: %v", err)
		return
	}
	a.logf(ctx, "chat: session saved: id=%s (resume with `nickpit chat --session %s`)", sess.ID, sess.ID)
}
