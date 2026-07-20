package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/dgrieser/nickpit/internal/textsan"
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
	if err := validateChatSourceFlags(opts); err != nil {
		return err
	}
	// A relative --repo-root is resolved against THIS invocation's working
	// directory before it is used or persisted. A session resumed from another
	// directory would otherwise re-resolve the stored relative path there —
	// chatToolset only checks that the directory exists, so the retrieval
	// tools could end up inspecting an unrelated checkout or local files.
	if opts.repoRoot != "" {
		abs, err := filepath.Abs(opts.repoRoot)
		if err != nil {
			return fmt.Errorf("chat: resolving --repo-root: %w", err)
		}
		opts.repoRoot = abs
	}
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
	// The store is opened only when this invocation loads from it or will
	// persist turns. An explicitly ephemeral chat from an external source
	// (--no-session with --from-json or --gitlab) needs neither — and must not
	// fail in minimal environments (no HOME/XDG_CACHE_HOME) where no cache
	// directory resolves.
	var store *session.Store
	if chatNeedsStore(opts, a.noSession) {
		var err error
		store, err = session.NewStore(a.sessionDir)
		if err != nil {
			return err
		}
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
	// The result also records the LLM endpoint the review ran against (a
	// one-off --base-url, or a profile endpoint that has since changed). API
	// keys are never persisted, so a diverging endpoint cannot be restored
	// safely: pairing the ACTIVE profile's key with the SAVED endpoint would
	// disclose the key to another provider, while silently keeping the active
	// endpoint would route the review's code context (and a possibly
	// provider-specific model) elsewhere. Fail with the choice spelled out
	// instead of silently picking either side; an explicit --base-url is an
	// informed override and wins, paired with the active profile's key.
	// (GitLab-reassembled sessions have no saved endpoint: it is deliberately
	// never carried in MR markers.)
	if a.baseURL == "" && sess.Result.BaseURL != "" && !sameLLMEndpoint(sess.Result.BaseURL, profile.BaseURL) {
		return fmt.Errorf("chat: session %s was reviewed against LLM endpoint %q, but the active profile targets %q with its own API key; select the review's profile, or pass --base-url (with matching credentials in the environment) to choose the endpoint explicitly",
			sess.ID, sess.Result.BaseURL, profile.BaseURL)
	}
	if opts.findingID != "" {
		sess.PinnedFindingID = opts.findingID
	}
	// A pinned id that does not exist in the review (a typo, or an id from
	// another review) would start a paid model turn with a contradictory
	// focus_finding_id and no opener; fail clearly before the first turn. This
	// also catches a stale pin persisted in a resumed session.
	if sess.PinnedFindingID != "" && !findingExists(sess.Result, sess.PinnedFindingID) {
		return fmt.Errorf("chat: finding %q not found in review %s (%d finding(s) available; list ids with the review JSON or omit --finding to discuss the whole review)",
			sess.PinnedFindingID, sess.ReviewID, len(sess.Result.Findings))
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
	// A session created in THIS invocation carries a host the user just chose
	// (e.g. parsed from --url); a resumed session's stored host is checked
	// against the active profile before the profile's token is sent to it.
	source, retrievalEngine, err := a.chatSource(profile, sess.Source, created)
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

	// --no-session is honored here too, not only for post-review auto-saves:
	// the user asked for nothing to be persisted, so turns run in memory only
	// and nothing new is written to the store (which may not even be open).
	saveSession := func() {
		if a.noSession || store == nil {
			return
		}
		if err := store.Save(sess); err != nil {
			a.warnf("chat: could not save session (conversation may be lost on resume): %v", err)
		}
	}
	if created || refreshed {
		saveSession()
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
		if err == nil && strings.TrimSpace(res.Reply) == "" {
			// A successful-but-blank completion is a model failure, not an answer:
			// persisting it would save the unanswered question, print nothing, and
			// leave the next attempt with consecutive user turns.
			err = fmt.Errorf("chat: discussion agent returned an empty reply")
		}
		if err != nil {
			// Drop the unanswered question so a retried turn does not leave two
			// consecutive user messages in the transcript.
			sess.Messages = sess.Messages[:len(sess.Messages)-1]
			return err
		}
		for _, m := range res.NewMessages {
			sess.Append(session.FromLLM(m))
		}
		saveSession()
		// LLM output is untrusted for terminal purposes: strip control characters
		// so a reply cannot smuggle escape sequences into the user's terminal.
		fmt.Fprintln(os.Stdout, textsan.StripControl(res.Reply)) //nolint:errcheck // stdout write; nothing actionable on failure
		return nil
	}

	question := strings.TrimSpace(strings.Join(args, " "))
	if question != "" {
		return turn(question)
	}
	if !isTerminal(os.Stdin) {
		return fmt.Errorf("chat: no question given and stdin is not a terminal; pass a question argument for one-shot use")
	}
	return a.chatREPL(ctx, sess, turn)
}

// chatREPL runs the interactive loop, printing session context and the pinned
// opener once before reading questions until EOF, an exit command, or ctx
// cancellation. Ctx must be observed here: a Ctrl+C mid-turn cancels the
// command context permanently, so without these checks every later turn would
// fail with "context canceled" and the session would zombie until /exit. Every
// value echoed here can originate outside this process (markers, saved JSON,
// model output), so it is control-stripped before touching the terminal.
func (a *app) chatREPL(ctx context.Context, sess *session.Session, turn func(string) error) error {
	fmt.Fprintf(os.Stderr, "Discussing review %s", textsan.StripControl(sess.ReviewID))
	if sess.Source.Repo != "" {
		fmt.Fprintf(os.Stderr, " on %s", textsan.StripControl(sess.Source.Repo))
		if sess.Source.Identifier > 0 {
			fmt.Fprintf(os.Stderr, "!%d", sess.Source.Identifier)
		}
	}
	fmt.Fprintf(os.Stderr, " (session %s). Type your question, or /exit to quit.\n", textsan.StripControl(sess.ID))
	if sess.PinnedFindingID != "" {
		if opener := review.DiscussOpener(sess.Result, sess.PinnedFindingID); opener != "" {
			fmt.Fprintf(os.Stderr, "\n%s\n", textsan.StripControl(opener))
		}
	}
	// Stdin is read in a goroutine so the idle prompt can select on ctx too:
	// the root context comes from signal.NotifyContext, and Ctrl+C at the
	// prompt cancels ctx WITHOUT interrupting a blocking stdin read — waiting
	// on Scan directly would leave the process stuck until another line or
	// EOF. The reader goroutine may stay blocked on stdin when ctx ends the
	// loop; the command returns and process exit reaps it.
	lines := make(chan string)
	scanDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scanDone <- scanner.Err()
	}()
	for {
		fmt.Fprint(os.Stderr, "\n> ")
		var line string
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-scanDone:
			return err // EOF or read error
		case line = <-lines:
		}
		line = strings.TrimSpace(line)
		switch line {
		case "":
			continue
		case "/exit", "/quit", "/q":
			return nil
		}
		if err := turn(line); err != nil {
			// A canceled context ends the REPL (the cancellation is permanent —
			// printing it once per turn would loop uselessly)...
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			// ...while ordinary turn errors are shown and the loop continues.
			// Errors can embed upstream response text; strip control characters.
			fmt.Fprintf(os.Stderr, "error: %v\n", textsan.StripControl(err.Error()))
		}
	}
}

// validateChatSourceFlags rejects flag combinations that would silently target
// the wrong session: more than one source selector at once (the dispatch in
// resolveChatSession would let the first one win), or GitLab selectors without
// GitLab mode (they would be ignored and the latest saved session — possibly an
// unrelated review — would be discussed instead).
func validateChatSourceFlags(opts chatOptions) error {
	gitlabMode := opts.gitlab || opts.replyDiscussion != ""
	var modes []string
	if opts.sessionID != "" {
		modes = append(modes, "--session")
	}
	if opts.fromJSON != "" {
		modes = append(modes, "--from-json")
	}
	if gitlabMode {
		if opts.gitlab {
			modes = append(modes, "--gitlab")
		} else {
			modes = append(modes, "--reply-discussion")
		}
	}
	if len(modes) > 1 {
		return fmt.Errorf("chat: %s select different session sources; pass exactly one", strings.Join(modes, " and "))
	}
	// --url fully determines the project, IID, and host; combined with --repo or
	// --id it would silently overwrite the explicit values, so a stale URL could
	// read or post on the wrong MR and send the token to the wrong host. Same
	// policy as `gitlab mr`.
	if strings.TrimSpace(opts.rawURL) != "" {
		if opts.repo != "" {
			return fmt.Errorf("chat: --url can not be combined with --repo")
		}
		if opts.mrID != 0 {
			return fmt.Errorf("chat: --url can not be combined with --id")
		}
	}
	if !gitlabMode {
		var stray []string
		if strings.TrimSpace(opts.rawURL) != "" {
			stray = append(stray, "--url")
		}
		if opts.repo != "" {
			stray = append(stray, "--repo")
		}
		if opts.mrID != 0 {
			stray = append(stray, "--id")
		}
		if opts.reviewID != "" {
			stray = append(stray, "--review-id")
		}
		if len(stray) > 0 {
			return fmt.Errorf("chat: %s only apply to a GitLab session; add --gitlab (or --reply-discussion)", strings.Join(stray, ", "))
		}
	}
	// --reply-note is meaningful only with --reply-discussion, and the reply
	// path derives its pin and review from the thread's own marker, so
	// --finding/--review-id there would be silently ignored — reject instead.
	if opts.replyNote != 0 && opts.replyDiscussion == "" {
		return fmt.Errorf("chat: --reply-note requires --reply-discussion")
	}
	if opts.replyDiscussion != "" {
		if opts.findingID != "" {
			return fmt.Errorf("chat: --finding can not be combined with --reply-discussion (the pin comes from the thread's marker)")
		}
		if opts.reviewID != "" {
			return fmt.Errorf("chat: --review-id can not be combined with --reply-discussion (the review comes from the thread's marker)")
		}
	}
	return nil
}

// chatNeedsStore reports whether this invocation must open the session store:
// it loads from it (--session, or the default latest-session source), or it
// will persist turns (persistence enabled). An ephemeral chat from an external
// source (--no-session with --from-json or --gitlab) needs no store.
func chatNeedsStore(opts chatOptions, noSession bool) bool {
	loadsFromStore := opts.sessionID != "" || (opts.fromJSON == "" && !opts.gitlab)
	return loadsFromStore || !noSession
}

// resolveChatSession loads or creates the session for this invocation. The bool
// reports whether a new session was created (so the caller persists it). store
// may be nil only when the selected source does not read from it (guaranteed by
// chatNeedsStore).
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
		return nil, fmt.Errorf("chat: no complete nickpit review found on the merge request (no markers, or a publish is still in progress)")
	}
	if reviewID != "" {
		if r, ok := reviews[reviewID]; ok {
			return r, nil
		}
		return nil, fmt.Errorf("chat: review id %q not found on the merge request (or its carrier data is incomplete)", reviewID)
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
// The SCM API URL is never ReviewResult.BaseURL (the LLM endpoint). trustedHost
// marks src.BaseURL as chosen explicitly in THIS invocation (a session just
// created from --url); a host merely restored from a stored session is not
// trusted with the active profile's token — see the mismatch check below.
func (a *app) chatSource(profile config.Profile, src session.Source, trustedHost bool) (model.ReviewSource, retrieval.Engine, error) {
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
		// The token always comes from the active profile and belongs to the
		// profile's host. Sending it to a DIFFERENT host restored from a stored
		// session would disclose it to another server, so a mismatch fails with
		// the choice spelled out rather than silently picking either host. An
		// explicit --gitlab-base-url is an informed override and wins
		// (profile.GitLabBaseURL already carries it).
		apiBaseURL := firstNonEmpty(src.BaseURL, profile.GitLabBaseURL)
		if !trustedHost && a.gitlabBaseURL == "" && src.BaseURL != "" &&
			glscm.NormalizeBaseURL(src.BaseURL) != glscm.NormalizeBaseURL(profile.GitLabBaseURL) {
			return nil, nil, fmt.Errorf("chat: session was created against GitLab host %s, but the active profile targets %s and its token belongs there; select the session's profile, or pass --gitlab-base-url (with a matching token) to choose the host explicitly",
				glscm.NormalizeBaseURL(src.BaseURL), glscm.NormalizeBaseURL(profile.GitLabBaseURL))
		}
		// On resume an explicit --gitlab-base-url wins over the stored host. A
		// just-created session's host already incorporates the override
		// (chatSessionFromGitLab falls back to the profile host, which carries
		// it), so it is left as chosen.
		if a.gitlabBaseURL != "" && !trustedHost {
			apiBaseURL = profile.GitLabBaseURL
		}
		adapter := glscm.NewAdapter(glscm.NewClient(apiBaseURL, profile.GitLabToken), profile.AssetBaseURL)
		return adapter, retrieval.NewLocalEngine(), nil
	case model.ModeGitHub:
		adapter := ghscm.NewAdapter(ghscm.NewClient("", profile.GitHubToken), profile.AssetBaseURL)
		return adapter, retrieval.NewLocalEngine(), nil
	default:
		return nil, nil, fmt.Errorf("chat: unsupported session mode %q", src.Mode)
	}
}

// sameLLMEndpoint compares two LLM endpoint URLs ignoring insignificant
// differences (surrounding whitespace, trailing slash). Two empty values both
// mean the client default and match.
func sameLLMEndpoint(a, b string) bool {
	norm := func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }
	return norm(a) == norm(b)
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

	notes, err := discussionWithFallbacks(ctx, client, project, mrID, opts.replyDiscussion, botUserID)
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
		// Also hit while a publish is still in flight: reassembly rejects a
		// review whose declared finding count has not landed yet. The non-zero
		// exit makes the daemon retry, by which time the carriers are usually
		// all posted.
		return fmt.Errorf("chat: review %q not found on MR (its carrier data may be incomplete or still publishing)", reviewID)
	}
	// The thread may pin a finding whose full payload is not on the MR: an
	// oversized finding comment carries only a routing ref, and its fallback
	// carrier note may still be publishing or may have failed to post (legacy
	// envelopes without a finding count are not caught by reassembly's
	// completeness gate). Spending an LLM turn on a pinned id the prompt cannot
	// resolve would answer without the finding — fail retryably instead.
	if findingID != "" && !findingExists(result, findingID) {
		return fmt.Errorf("chat: pinned finding %q not available in review %s (its carrier may be incomplete or still publishing)", findingID, reviewID)
	}
	// The carrier records the model the review actually ran with (including any
	// --model override at review time, or a profile model that has since
	// changed). Answer the thread with that model — mirroring the interactive
	// resume path — unless --model was explicitly passed for this invocation.
	if a.model == "" && result.Model != "" {
		profile.Model = result.Model
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
		// An empty completion means the model failed to answer, not that nothing
		// should be posted. Exiting successfully would let the daemon keep the
		// note's dedup mark and suppress redeliveries, leaving the question
		// permanently unanswered; a non-zero exit makes the handler forget the
		// note so a redelivery retries.
		return fmt.Errorf("chat: discussion agent returned an empty reply")
	}
	// Revalidate immediately before posting: if a newer user note arrived while
	// the LLM turn was running, posting now would answer out of order AND make
	// the newer note's child see a bot reply as the latest activity and skip —
	// permanently dropping that question. Abort instead; the newer note's child
	// answers with the full history, including the question this turn covered.
	// (A tiny window between this check and the POST remains; the SCM offers no
	// atomic check-and-post.)
	fresh, err := discussionWithFallbacks(ctx, client, project, mrID, opts.replyDiscussion, botUserID)
	if err != nil {
		return fmt.Errorf("chat: rechecking thread before reply: %w", err)
	}
	if freshPending, stillOK := latestPendingNote(fresh, botUserID); !stillOK || freshPending != pending {
		return nil
	}
	// Sanitize defuses marker lookalikes and control characters;
	// EscapeQuickActions defuses GitLab quick actions (/merge, /close, ...) —
	// the reply is model-generated, and a commenter could prompt the model into
	// emitting one, which the Notes API would execute under the BOT's identity
	// and privileges.
	posted := reviewmd.EscapeQuickActions(reviewmd.Sanitize(reply))
	err = client.ReplyToMRDiscussionPath(ctx, project, mrID, opts.replyDiscussion, posted)
	if err == nil {
		return nil
	}
	// Some GitLab versions reject replies to individual-note discussions with a
	// 4xx — exactly the roots chat threads can have (summary and general-finding
	// comments are created through the notes endpoint). Mirror the daemon's
	// Handler.reply: fall back to a plain MR note so the answer is delivered
	// (unthreaded) instead of the daemon retrying the same failing request three
	// times and never answering. 5xx and transport errors stay fatal, which makes
	// the daemon retry. The note carries a hidden ChatReplyEnvelope binding it to
	// the discussion and the answered note, so the threaded conversation state
	// survives: discussionWithFallbacks merges it back on the next read, the
	// answered question is not re-answered on redelivery, and follow-up turns see
	// this reply in their history.
	var apiErr *glscm.APIError
	if !errors.As(err, &apiErr) || apiErr.Status >= 500 {
		return err
	}
	a.logf(ctx, "chat: threaded reply rejected (%v), posting as a plain MR note", err)
	body := posted
	if marker := reviewmd.ChatReplyMarker(opts.replyDiscussion, pending); marker != "" {
		body += "\n\n" + marker
	}
	return client.CreateMRNotePath(ctx, project, mrID, body)
}

// discussionWithFallbacks reads a discussion's notes and merges in the bot's
// fallback top-level answers — posted when GitLab rejected a threaded reply,
// each carrying a ChatReplyEnvelope bound to this discussion. Every fallback
// answer is inserted right after the note it answered, as a synthetic bot note,
// so pending-question detection and history conversion see the thread as if
// the reply had landed threaded.
func discussionWithFallbacks(ctx context.Context, client *glscm.Client, project string, mrID int, discussionID string, botUserID int) ([]glscm.DiscussionNote, error) {
	notes, err := client.DiscussionNotes(ctx, project, mrID, discussionID)
	if err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		return notes, nil
	}
	mrNotes, err := client.MRNotes(ctx, project, mrID)
	if err != nil {
		return nil, fmt.Errorf("reading MR notes for fallback replies: %w", err)
	}
	return mergeFallbackReplies(notes, mrNotes, discussionID, botUserID), nil
}

// mergeFallbackReplies is the pure merge behind discussionWithFallbacks. Only
// notes authored by the bot itself are trusted as fallback answers (markers are
// encoded, not authenticated), and MRNotes returning the same note from both
// the notes and discussions endpoints is de-duplicated on content.
func mergeFallbackReplies(notes []glscm.DiscussionNote, mrNotes []glscm.MRNote, discussionID string, botUserID int) []glscm.DiscussionNote {
	replies := make(map[int][]string)
	dup := make(map[string]bool)
	for _, note := range mrNotes {
		if botUserID == 0 || note.AuthorID != botUserID {
			continue
		}
		for _, env := range reviewmd.CollectChatReplyEnvelopes(note.Body) {
			if env.DiscussionID != discussionID {
				continue
			}
			body := reviewmd.StripMarkers(note.Body)
			key := fmt.Sprintf("%d\x00%s", env.AnsweredNoteID, body)
			if body == "" || dup[key] {
				continue
			}
			dup[key] = true
			replies[env.AnsweredNoteID] = append(replies[env.AnsweredNoteID], body)
		}
	}
	if len(replies) == 0 {
		return notes
	}
	merged := make([]glscm.DiscussionNote, 0, len(notes)+len(replies))
	for _, note := range notes {
		merged = append(merged, note)
		for _, body := range replies[note.ID] {
			merged = append(merged, glscm.DiscussionNote{Body: body, AuthorID: botUserID})
		}
	}
	return merged
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

// findingExists reports whether the review contains a finding with the given id.
func findingExists(result *model.ReviewResult, findingID string) bool {
	if result == nil {
		return false
	}
	for _, f := range result.Findings {
		if f.ID == findingID {
			return true
		}
	}
	return false
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
		a.warnf("chat: session store unavailable (review will not be resumable with `nickpit chat`): %v", err)
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
		a.warnf("chat: could not save session (review will not be resumable with `nickpit chat`): %v", err)
		return
	}
	a.logf(ctx, "chat: session saved: id=%s (resume with `nickpit chat --session %s`)", sess.ID, sess.ID)
}
