package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/google/uuid"
)

// The coordinator belongs to the chat loop. Workers only send immutable results;
// they never read or write the live session or write to its terminal.
type cliChatUpdates struct {
	pending   bool
	reserved  *cliUpdateInput
	completed chan cliUpdateCompletion
	cancel    context.CancelFunc
	stopped   chan struct{}
	run       func(context.Context, cliUpdateInput) cliUpdateCompletion
}

type cliUpdateInput struct {
	Source         session.Source
	ContextOptions *session.ContextOptions
	Result         *model.ReviewResult
	Messages       []llm.Message
	Question       string
	Signal         review.ReviewUpdateSignal
}

type cliUpdateCompletion struct {
	Result  *model.ReviewResult
	Context *model.ReviewContext
	Reason  string
	Message string
	Tokens  model.TokenUsage
	Err     error
}

func (u *cliChatUpdates) handler(input cliUpdateInput) func(context.Context, review.ReviewUpdateSignal) (review.ReviewUpdateToolResult, error) {
	return func(_ context.Context, signal review.ReviewUpdateSignal) (review.ReviewUpdateToolResult, error) {
		if u.pending {
			if u.reserved != nil && reflect.DeepEqual(u.reserved.Signal, signal) {
				return review.ReviewUpdateToolResult{Status: review.ReviewUpdateScheduled}, nil
			}
			return review.ReviewUpdateToolResult{Status: review.ReviewUpdateError, Message: "A review update is already pending. Continue discussing it or wait for the result."}, nil
		}
		if err := signal.Validate(input.Result); err != nil {
			return review.ReviewUpdateToolResult{}, err
		}
		input.Signal = signal
		// Clone nested slices, findings, and options before another turn can append.
		raw, err := json.Marshal(input)
		if err != nil {
			return review.ReviewUpdateToolResult{}, err
		}
		var snapshot cliUpdateInput
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return review.ReviewUpdateToolResult{}, err
		}
		u.reserved, u.pending = &snapshot, true
		return review.ReviewUpdateToolResult{Status: review.ReviewUpdateScheduled}, nil
	}
}

func (u *cliChatUpdates) start(ctx context.Context) {
	if u.reserved == nil {
		return
	}
	input := *u.reserved
	u.reserved = nil
	workerCtx, cancel := context.WithCancel(ctx)
	u.cancel = cancel
	u.stopped = make(chan struct{})
	stopped := u.stopped
	go func() {
		defer close(stopped)
		defer cancel()
		u.completed <- u.run(workerCtx, input)
	}()
}

func (u *cliChatUpdates) discardReservation() {
	if u.reserved != nil {
		u.reserved, u.pending = nil, false
	}
}

func (u *cliChatUpdates) stop() {
	if u.cancel != nil {
		u.cancel()
	}
	if u.stopped != nil {
		<-u.stopped
	}
}

// cliUpdateAttempt holds one fresh checkout and its validation/publication hooks.
// The hooks make retries testable without a real provider or detached processes.
type cliUpdateAttempt struct {
	request     review.UpdateWorkflowRequest
	fingerprint string
	validate    func(context.Context) error
	publish     func(context.Context, *model.ReviewResult, string) (*model.ReviewResult, error)
	close       func()
}

type cliUpdatePlan struct {
	before      *model.ReviewResult
	result      *review.UpdateWorkflowResult
	context     *model.ReviewContext
	fingerprint string
}

type cliUpdateRunner struct {
	lock     func(context.Context) (context.Context, func(), error)
	recover  func(context.Context, string) (*model.ReviewResult, error)
	prepare  func(context.Context) (*cliUpdateAttempt, error)
	workflow func(context.Context, review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error)
	wait     func(context.Context, time.Duration) error
}

func waitCLIUpdate(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r cliUpdateRunner) run(ctx context.Context, reason string) cliUpdateCompletion {
	operation := uuid.NewString()
	var plan *cliUpdatePlan
	var lastErr error
	failures, conflicts := 0, 0
	wait := r.wait
	if wait == nil {
		wait = waitCLIUpdate
	}
	for {
		if ctx.Err() != nil {
			return cliUpdateCompletion{Reason: reason, Err: ctx.Err()}
		}
		attemptCtx, unlock, err := r.lock(ctx)
		if errors.Is(err, glscm.ErrLockBusy) {
			if err = wait(ctx, 100*time.Millisecond); err != nil {
				return cliUpdateCompletion{Reason: reason, Err: err}
			}
			continue
		}
		var completion cliUpdateCompletion
		if err == nil {
			completion, plan, err = r.execute(attemptCtx, operation, plan, failures >= 3 || conflicts >= 5, lastErr)
			unlock()
		}
		if err == nil {
			completion.Reason = reason
			return completion
		}
		if ctx.Err() != nil {
			return cliUpdateCompletion{Reason: reason, Err: ctx.Err()}
		}
		if failures >= 3 || conflicts >= 5 {
			return cliUpdateCompletion{Reason: reason, Err: err}
		}
		lastErr = err
		var conflict *glscm.UpdateConflict
		counter := 0
		if errors.As(err, &conflict) {
			conflicts++
			counter = conflicts
			plan = nil // Conflict checks precede staging.
		} else {
			failures++
			counter = failures
		}
		if err = wait(ctx, time.Duration(min(counter, 6))*10*time.Second); err != nil {
			return cliUpdateCompletion{Reason: reason, Err: err}
		}
	}
}

func (r cliUpdateRunner) execute(ctx context.Context, operation string, plan *cliUpdatePlan, exhausted bool, lastErr error) (cliUpdateCompletion, *cliUpdatePlan, error) {
	// Recovery must precede both retirement and re-evaluation after uncertain writes.
	recovered, err := r.recover(ctx, operation)
	if err != nil {
		return cliUpdateCompletion{}, plan, err
	}
	if recovered != nil {
		out := cliUpdateCompletion{Result: recovered, Message: "Review update completed on GitLab."}
		if plan != nil {
			out.Context, out.Tokens = plan.context, plan.result.Outcome.TokensUsed
			out.Message = updateFollowup(&plan.result.Outcome)
		}
		return out, nil, nil
	}
	if exhausted {
		return cliUpdateCompletion{}, plan, fmt.Errorf("review update retry limit reached: %w", lastErr)
	}
	attempt, err := r.prepare(ctx)
	if err != nil {
		return cliUpdateCompletion{}, plan, err
	}
	defer attempt.close()
	ids, err := activeCLIUpdateIDs(attempt.request.Result, attempt.request.Signal.FindingIDs)
	if err != nil {
		return cliUpdateCompletion{}, plan, err
	}
	if len(attempt.request.Signal.FindingIDs) > 0 && len(ids) == 0 {
		if err := attempt.validate(ctx); err != nil {
			return cliUpdateCompletion{}, plan, err
		}
		return cliUpdateCompletion{Result: attempt.request.Result, Context: attempt.request.ReviewCtx, Message: "The requested findings are already resolved; no further changes were needed."}, nil, nil
	}
	attempt.request.Signal.FindingIDs = ids
	if plan == nil || plan.fingerprint != attempt.fingerprint || !glscm.SameReviewState(plan.before, attempt.request.Result) {
		result, err := r.workflow(ctx, attempt.request)
		if err != nil {
			return cliUpdateCompletion{}, nil, err
		}
		plan = &cliUpdatePlan{before: attempt.request.Result, result: result, context: attempt.request.ReviewCtx, fingerprint: attempt.fingerprint}
	}
	if err := attempt.validate(ctx); err != nil {
		return cliUpdateCompletion{}, plan, err
	}
	after := attempt.request.Result
	if plan.result.Publish {
		after, err = attempt.publish(ctx, plan.result.Review, operation)
		if err != nil {
			return cliUpdateCompletion{}, plan, err
		}
	}
	return cliUpdateCompletion{Result: after, Context: plan.context, Message: updateFollowup(&plan.result.Outcome), Tokens: plan.result.Outcome.TokensUsed}, nil, nil
}

func activeCLIUpdateIDs(result *model.ReviewResult, ids []string) ([]string, error) {
	active := make([]string, 0, len(ids))
	for _, id := range ids {
		found := false
		for _, finding := range result.Findings {
			if finding.ID == id {
				found = true
				if finding.Resolution == nil {
					active = append(active, id)
				}
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("finding %s no longer available in original review", id)
		}
	}
	return active, nil
}

func (a *app) runCLIReviewUpdate(ctx context.Context, profile config.Profile, input cliUpdateInput, trustedHost bool) cliUpdateCompletion {
	fail := func(err error) cliUpdateCompletion { return cliUpdateCompletion{Reason: input.Signal.Reason, Err: err} }
	source, retrieval, err := a.chatSource(profile, input.Source, trustedHost)
	if err != nil {
		return fail(err)
	}
	// Background model streams must not interleave with interactive input/replies.
	logger := logging.New(io.Discard, false, false)
	engine, err := a.chatEngine(ctx, profile, source, retrieval, logger)
	if err != nil {
		return fail(err)
	}
	if small, smallProfile, distinct := newSmallLLMClient(profile, logger); distinct {
		engine.SetSmallClient(small, smallProfile)
	}
	adapter, remote := source.(*glscm.Adapter)
	loadPublished := func(ctx context.Context) (*model.ReviewResult, error) {
		reviews, err := adapter.ReviewResults(ctx, input.Source.Repo, input.Source.Identifier)
		if err != nil {
			return nil, err
		}
		current := reviews[input.Result.ReviewID]
		if current == nil {
			return nil, fmt.Errorf("original review %s is not available on GitLab", input.Result.ReviewID)
		}
		return mergeCLIReviewState(input.Result, current)
	}
	runner := cliUpdateRunner{workflow: engine.RunUpdateWorkflow}
	runner.lock = func(ctx context.Context) (context.Context, func(), error) {
		if remote {
			return adapter.Client().TryLockMR(ctx, "update-execution/"+input.Source.Repo, input.Source.Identifier)
		}
		return ctx, func() {}, nil
	}
	runner.recover = func(ctx context.Context, operation string) (*model.ReviewResult, error) {
		if !remote {
			return nil, nil
		}
		if err := adapter.RecoverReviewUpdates(ctx, input.Source.Repo, input.Source.Identifier); err != nil {
			return nil, err
		}
		committed, err := adapter.ReviewUpdateCommitted(ctx, input.Source.Repo, input.Source.Identifier, input.Result.ReviewID, operation)
		if err != nil || !committed {
			return nil, err
		}
		return loadPublished(ctx)
	}
	runner.prepare = func(ctx context.Context) (*cliUpdateAttempt, error) {
		before, err := input.Result.Clone()
		if err != nil {
			return nil, err
		}
		if remote {
			before, err = loadPublished(ctx)
			if err != nil {
				return nil, err
			}
		}
		options := input.ContextOptions
		if remote && before.ContextOptions != nil {
			options = before.ContextOptions
		}
		req := a.chatReviewRequest(profile, input.Source, options)
		req.IncludeComments = false
		sourceCtx, err := source.ResolveContext(ctx, req)
		if err != nil {
			return nil, err
		}
		var co chatCheckout
		clean, err := a.chatPrepareContext(ctx, engine, source, profile, req, &co)
		// A reverted local change can leave an empty diff while the old finding
		// still needs assessment against the current checkout.
		if errors.Is(err, review.ErrEmptyDiff) && len(sourceCtx.ChangedFiles) == 0 && sourceCtx.Diff == "" {
			clean, err = model.CloneContext(sourceCtx)
		}
		if err != nil {
			co.release()
			return nil, err
		}
		clean.Comments = nil
		root := input.Source.RepoRoot
		if root == "" {
			root = co.root
		}
		// Compare a separately resolved source snapshot, avoiding timestamps and
		// presentation fields introduced by context preparation.
		checked, err := source.ResolveContext(ctx, req)
		if err != nil {
			co.release()
			return nil, err
		}
		if !sameCLIDiff(clean, sourceCtx) || cliEvidenceFingerprint(input.Messages, checked) != cliEvidenceFingerprint(input.Messages, sourceCtx) {
			co.release()
			return nil, &glscm.UpdateConflict{Kind: "source diff"}
		}
		fingerprint := cliEvidenceFingerprint(input.Messages, sourceCtx)
		attempt := &cliUpdateAttempt{close: co.release, fingerprint: fingerprint}
		attempt.request = review.UpdateWorkflowRequest{
			UpdateFindingsRequest: review.UpdateFindingsRequest{DiscussRequest: review.DiscussRequest{
				Result: before, ReviewCtx: clean, Messages: input.Messages, RepoRoot: root, Tools: chatToolset(root), DiffFormat: req.DiffFormat,
				DisableSuggestions: profile.DisableSuggestions, DisableParallelToolCalls: a.disableParallelToolCalls,
				MaxToolCalls: profile.MaxToolCalls, MaxDuplicateToolCalls: profile.MaxDuplicateToolCalls, MaxOutputRetries: profile.MaxOutputRetries, MaxReasoningSeconds: profile.MaxReasoningSeconds,
			}, Signal: input.Signal},
			InitiatingMessage: input.Question, PriorityThreshold: a.priorityThreshold, ConfidenceThreshold: a.confidenceThreshold, DisablePatchSummary: profile.DisablePatchSummary,
		}
		attempt.validate = func(ctx context.Context) error {
			current, err := source.ResolveContext(ctx, req)
			if err != nil {
				return err
			}
			if cliEvidenceFingerprint(input.Messages, current) != fingerprint {
				return &glscm.UpdateConflict{Kind: "source diff"}
			}
			if remote {
				return adapter.ValidateReviewState(ctx, input.Source.Repo, input.Source.Identifier, before)
			}
			return nil
		}
		attempt.publish = func(ctx context.Context, after *model.ReviewResult, operation string) (*model.ReviewResult, error) {
			if remote {
				published, err := adapter.UpdateReview(ctx, input.Source.Repo, input.Source.Identifier, glscm.ReviewUpdateRequest{
					Operation: operation, Before: before, After: after, BaseSHA: clean.DiffBaseSHA, HeadSHA: clean.DiffHeadSHA, Validate: attempt.validate,
				})
				if err != nil {
					return nil, err
				}
				// A no-op can return only the remote carrier. Keep local metadata
				// so it neither disappears nor creates a spurious history entry.
				return mergeCLIReviewState(before, published)
			}
			return prepareLocalCLIRevision(before, after)
		}
		return attempt, nil
	}
	return runner.run(ctx, input.Signal.Reason)
}

func sameCLIDiff(a, b *model.ReviewContext) bool {
	// Context filtering/trimming may alter Diff. Commit identity, when present,
	// still must match the fresh source used for the evaluation.
	return a.DiffBaseSHA == b.DiffBaseSHA && a.DiffHeadSHA == b.DiffHeadSHA
}

func cliEvidenceFingerprint(messages []llm.Message, ctx *model.ReviewContext) string {
	raw, _ := json.Marshal(struct {
		Messages         []llm.Message
		Base, Head, Diff string
		Files            []model.ChangedFile
	}{messages, ctx.DiffBaseSHA, ctx.DiffHeadSHA, ctx.Diff, ctx.ChangedFiles})
	return fmt.Sprintf("cli-v1:%x", sha256.Sum256(raw))
}

func prepareLocalCLIRevision(before, after *model.ReviewResult) (*model.ReviewResult, error) {
	out, err := after.Clone()
	if err != nil {
		return nil, err
	}
	if glscm.SameReviewState(before, out) {
		return before, nil
	}
	out.Revision = before.Revision + 1
	for i := range out.Findings {
		for _, old := range before.Findings {
			if old.ID == out.Findings[i].ID && !reflect.DeepEqual(old, out.Findings[i]) {
				out.Findings[i].Revision = out.Revision
				break
			}
		}
	}
	return out, nil
}

// Preserve local run metadata while adopting authoritative carrier fields.
func mergeCLIReviewState(local, current *model.ReviewResult) (*model.ReviewResult, error) {
	out, err := local.Clone()
	if err != nil {
		return nil, err
	}
	if glscm.SameReviewState(out, current) {
		// GitLab's discussion order can differ from the saved finding order.
		// Reordering the same review is not a correction to archive.
		return out, nil
	}
	remote, err := current.Clone()
	if err != nil {
		return nil, err
	}
	out.Revision, out.Findings, out.ContextOptions = remote.Revision, remote.Findings, remote.ContextOptions
	out.OverallCorrectness, out.OverallExplanation, out.OverallConfidenceScore = remote.OverallCorrectness, remote.OverallExplanation, remote.OverallConfidenceScore
	return out, nil
}
