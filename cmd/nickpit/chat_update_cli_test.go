package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

func cliTestReview() *model.ReviewResult {
	return &model.ReviewResult{ReviewID: "review", Findings: []model.Finding{{ID: "finding", Title: "Old title", Body: "Old evidence"}}, OverallCorrectness: "patch is incorrect"}
}

func cliTestRunner(before *model.ReviewResult) cliUpdateRunner {
	return cliUpdateRunner{
		lock:    func(ctx context.Context) (context.Context, func(), error) { return ctx, func() {}, nil },
		recover: func(context.Context, string) (*model.ReviewResult, error) { return nil, nil },
		prepare: func(context.Context) (*cliUpdateAttempt, error) {
			return &cliUpdateAttempt{fingerprint: "snapshot", close: func() {},
				request:  review.UpdateWorkflowRequest{UpdateFindingsRequest: review.UpdateFindingsRequest{DiscussRequest: review.DiscussRequest{Result: before, ReviewCtx: &model.ReviewContext{DiffHeadSHA: "head"}}, Signal: review.ReviewUpdateSignal{FindingIDs: []string{"finding"}, Reason: "Guard prevents failure."}}},
				validate: func(context.Context) error { return nil },
				publish: func(_ context.Context, after *model.ReviewResult, _ string) (*model.ReviewResult, error) {
					return prepareLocalCLIRevision(before, after)
				},
			}, nil
		},
		workflow: func(_ context.Context, req review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error) {
			after, err := req.Result.Clone()
			if err != nil {
				return nil, err
			}
			after.Findings[0].Body = "Corrected evidence"
			return &review.UpdateWorkflowResult{Review: after, Publish: true, Outcome: review.ReviewUpdateOutcome{Changed: after.Findings}}, nil
		},
		wait: func(context.Context, time.Duration) error { return nil },
	}
}

func TestCLIUpdateReservationAndImmutableSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started, release := make(chan cliUpdateInput, 1), make(chan struct{})
	u := &cliChatUpdates{completed: make(chan cliUpdateCompletion, 1), run: func(ctx context.Context, in cliUpdateInput) cliUpdateCompletion {
		started <- in
		select {
		case <-release:
			return cliUpdateCompletion{Message: "done"}
		case <-ctx.Done():
			return cliUpdateCompletion{Err: ctx.Err()}
		}
	}}
	defer u.stop()
	input := cliUpdateInput{Result: cliTestReview(), Messages: []llm.Message{{Role: "user", Content: "Original question"}}, Question: "Original question"}
	handler := u.handler(input)
	signal := review.ReviewUpdateSignal{FindingIDs: []string{"finding"}, Reason: "Evidence"}
	for range 2 {
		out, err := handler(ctx, signal)
		if err != nil || out.Status != review.ReviewUpdateScheduled {
			t.Fatalf("schedule: %+v %v", out, err)
		}
	}
	input.Result.Findings[0].Body = "Mutated later"
	input.Messages[0].Content = "Edited later"
	u.start(ctx)
	select {
	case snapshot := <-started:
		if snapshot.Result.Findings[0].Body != "Old evidence" || snapshot.Messages[0].Content != "Original question" {
			t.Fatalf("snapshot mutated: %+v", snapshot)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	out, err := u.handler(input)(ctx, signal)
	if err != nil || out.Status != review.ReviewUpdateError {
		t.Fatalf("second update admitted: %+v %v", out, err)
	}
	close(release)
	select {
	case result := <-u.completed:
		if result.Message != "done" {
			t.Fatal(result)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	u.pending = false
	if _, err := handler(ctx, signal); err != nil {
		t.Fatal(err)
	}
	u.discardReservation()
	if u.pending || u.reserved != nil {
		t.Fatal("failed discussion retained reservation")
	}
}

func TestCLIUpdateCancellationJoinsWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	u := &cliChatUpdates{pending: true, reserved: &cliUpdateInput{}, completed: make(chan cliUpdateCompletion, 1), run: func(ctx context.Context, _ cliUpdateInput) cliUpdateCompletion {
		<-ctx.Done()
		return cliUpdateCompletion{Err: ctx.Err()}
	}}
	u.start(ctx)
	cancel()
	u.stop()
	if out := <-u.completed; !errors.Is(out.Err, context.Canceled) {
		t.Fatal(out.Err)
	}
}

func TestCLIUpdateRetryBudgetsAndLockContention(t *testing.T) {
	for _, tc := range []struct {
		name     string
		conflict bool
		limit    int
	}{{"failures", false, 3}, {"conflicts", true, 5}} {
		t.Run(tc.name, func(t *testing.T) {
			r := cliTestRunner(cliTestReview())
			locks, calls, recoveries := 0, 0, 0
			held := false
			r.lock = func(ctx context.Context) (context.Context, func(), error) {
				locks++
				if locks <= 2 {
					return ctx, nil, glscm.ErrLockBusy
				}
				held = true
				return ctx, func() { held = false }, nil
			}
			r.recover = func(context.Context, string) (*model.ReviewResult, error) { recoveries++; return nil, nil }
			r.workflow = func(context.Context, review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error) {
				if !held {
					t.Fatal("evaluation ran outside execution lock")
				}
				calls++
				if tc.conflict {
					return nil, &glscm.UpdateConflict{Kind: "evidence"}
				}
				return nil, errors.New("model failed")
			}
			var waits []time.Duration
			r.wait = func(_ context.Context, d time.Duration) error {
				if held {
					t.Fatal("lock held during backoff")
				}
				waits = append(waits, d)
				return nil
			}
			out := r.run(context.Background(), "Reason")
			if out.Err == nil || calls != tc.limit || recoveries != tc.limit+1 || len(waits) != tc.limit+2 {
				t.Fatalf("out=%+v calls=%d recovery=%d waits=%v", out, calls, recoveries, waits)
			}
			for i, delay := range waits[2:] {
				if delay != time.Duration(i+1)*10*time.Second {
					t.Fatal(waits)
				}
			}
		})
	}
}

func TestCLIUpdateRecoversCommitBeforeRetryExhaustion(t *testing.T) {
	before := cliTestReview()
	r := cliTestRunner(before)
	workflow := r.workflow
	modelCalls, posts, recoveries := 0, 0, 0
	committed := false
	var operation string
	var published *model.ReviewResult
	r.workflow = func(ctx context.Context, req review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error) {
		modelCalls++
		return workflow(ctx, req)
	}
	prepare := r.prepare
	r.prepare = func(ctx context.Context) (*cliUpdateAttempt, error) {
		attempt, err := prepare(ctx)
		attempt.publish = func(_ context.Context, after *model.ReviewResult, op string) (*model.ReviewResult, error) {
			if operation != "" && operation != op {
				t.Fatal("operation changed across retries")
			}
			operation = op
			posts++
			if posts == 3 {
				committed = true
				published = after
			}
			return nil, errors.New("uncertain API response")
		}
		return attempt, err
	}
	r.recover = func(_ context.Context, op string) (*model.ReviewResult, error) {
		recoveries++
		if committed {
			if op != operation {
				t.Fatal("wrong recovery operation")
			}
			return published, nil
		}
		return nil, nil
	}
	out := r.run(context.Background(), "Reason")
	if out.Err != nil || out.Result == nil || modelCalls != 1 || posts != 3 || recoveries != 4 {
		t.Fatalf("out=%+v models=%d posts=%d recovery=%d", out, modelCalls, posts, recoveries)
	}
}

func TestCLIUpdateChangedEvidenceDiscardsPreparedResult(t *testing.T) {
	r := cliTestRunner(cliTestReview())
	workflow, prepare := r.workflow, r.prepare
	models, attempts := 0, 0
	r.workflow = func(ctx context.Context, req review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error) {
		models++
		return workflow(ctx, req)
	}
	r.prepare = func(ctx context.Context) (*cliUpdateAttempt, error) {
		attempts++
		a, err := prepare(ctx)
		if attempts == 1 {
			a.publish = func(context.Context, *model.ReviewResult, string) (*model.ReviewResult, error) {
				return nil, errors.New("API failed")
			}
		} else {
			a.fingerprint = "fresh diff"
		}
		return a, err
	}
	if out := r.run(context.Background(), "Reason"); out.Err != nil || models != 2 || attempts != 2 {
		t.Fatalf("out=%+v models=%d attempts=%d", out, models, attempts)
	}
}

func TestCLIUpdateNoopStillValidatesAndResolvedDoesNotBroaden(t *testing.T) {
	for _, resolved := range []bool{false, true} {
		before := cliTestReview()
		if resolved {
			before.Findings[0].Resolution = &model.FindingResolution{Reason: "Already fixed."}
		}
		r := cliTestRunner(before)
		models, validates, posts := 0, 0, 0
		r.workflow = func(_ context.Context, req review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error) {
			models++
			return &review.UpdateWorkflowResult{Review: req.Result}, nil
		}
		prepare := r.prepare
		r.prepare = func(ctx context.Context) (*cliUpdateAttempt, error) {
			a, err := prepare(ctx)
			a.validate = func(context.Context) error { validates++; return nil }
			a.publish = func(context.Context, *model.ReviewResult, string) (*model.ReviewResult, error) {
				posts++
				return nil, errors.New("unexpected publish")
			}
			return a, err
		}
		out := r.run(context.Background(), "Reason")
		wantModels := 1
		if resolved {
			wantModels = 0
		}
		if out.Err != nil || validates != 1 || models != wantModels || posts != 0 || !reflect.DeepEqual(out.Result, before) {
			t.Fatalf("out=%+v validates=%d models=%d posts=%d", out, validates, models, posts)
		}
	}
	if _, err := activeCLIUpdateIDs(cliTestReview(), []string{"missing"}); err == nil {
		t.Fatal("missing finding accepted")
	}
}

func TestCLIUpdateCancellationDuringWait(t *testing.T) {
	for _, lockBusy := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		r := cliTestRunner(cliTestReview())
		if lockBusy {
			r.lock = func(ctx context.Context) (context.Context, func(), error) { return ctx, nil, glscm.ErrLockBusy }
		} else {
			r.workflow = func(context.Context, review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error) {
				return nil, errors.New("retry")
			}
		}
		r.wait = func(ctx context.Context, d time.Duration) error { cancel(); return waitCLIUpdate(ctx, d) }
		out := r.run(ctx, "Reason")
		cancel()
		if !errors.Is(out.Err, context.Canceled) {
			t.Fatal(out.Err)
		}
	}
}

func TestCLIRevisionAndEvidence(t *testing.T) {
	before := cliTestReview()
	after, _ := before.Clone()
	after.Findings[0].Resolution = &model.FindingResolution{Reason: "Fixed."}
	next, err := prepareLocalCLIRevision(before, after)
	if err != nil || next.Revision != 1 || next.Findings[0].Revision != 1 || before.Revision != 0 {
		t.Fatalf("%+v %v", next, err)
	}
	again, err := prepareLocalCLIRevision(next, next)
	if err != nil || again.Revision != 1 {
		t.Fatal("no-op advanced revision")
	}
	messages := []llm.Message{{Role: "user", Content: "Question"}}
	ctx := &model.ReviewContext{Diff: "patch", Comments: []model.Comment{{Body: "Unrelated MR comment"}}}
	original := cliEvidenceFingerprint(messages, ctx)
	ctx.Comments[0].Body = "Unrelated edit"
	if cliEvidenceFingerprint(messages, ctx) != original {
		t.Fatal("unrelated comment changed fingerprint")
	}
	messages[0].Content = "Edited evidence"
	if cliEvidenceFingerprint(messages, ctx) == original || !strings.HasPrefix(original, "cli-v1:") {
		t.Fatal("selected evidence not fingerprinted")
	}
}

func TestCLIUpdateMixedConflictsAndFailuresHaveSeparateBudgets(t *testing.T) {
	r := cliTestRunner(cliTestReview())
	workflow := r.workflow
	calls := 0
	r.workflow = func(ctx context.Context, req review.UpdateWorkflowRequest) (*review.UpdateWorkflowResult, error) {
		calls++
		if calls <= 4 {
			return nil, &glscm.UpdateConflict{Kind: "review"}
		}
		if calls <= 6 {
			return nil, errors.New("temporary model failure")
		}
		return workflow(ctx, req)
	}
	out := r.run(context.Background(), "Reason")
	if out.Err != nil || calls != 7 {
		t.Fatalf("mixed retries: calls=%d err=%v", calls, out.Err)
	}
}

func TestCLIMergePublishedReviewIgnoresFindingOrder(t *testing.T) {
	local := cliTestReview()
	local.Findings = append(local.Findings, model.Finding{ID: "second", Body: "Second issue"})
	local.RuntimeSeconds = 42
	current, _ := local.Clone()
	current.RuntimeSeconds = 0
	current.Findings[0], current.Findings[1] = current.Findings[1], current.Findings[0]
	merged, err := mergeCLIReviewState(local, current)
	if err != nil || !reflect.DeepEqual(merged, local) {
		t.Fatalf("unchanged carrier order replaced local review: %+v %v", merged, err)
	}
	current.Revision++
	current.Findings[0].Body = "Updated second issue"
	merged, err = mergeCLIReviewState(local, current)
	if err != nil || !glscm.SameReviewState(merged, current) || merged.RuntimeSeconds != local.RuntimeSeconds {
		t.Fatalf("new carrier state or local metadata lost: %+v %v", merged, err)
	}
}
