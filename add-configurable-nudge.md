# Add configurable nudge-N for review agents

## Context

Review agents return JSON findings and stop. They may miss issues on a single pass. Add a configurable "nudge N times" mechanism so each reviewer is asked again to look more thoroughly after producing its first JSON result. Default `N = 3`; `0` disables.

Constraints from the user:
- Keep first JSON before any nudge.
- Reset budget limits (max tool calls, JSON retries, reasoning timeouts, reasoning loop repeats) for each nudge round.
- Findings only ADD across rounds — the LLM may regurgitate prior findings into the new JSON; preserve the remembered list by appending NEW findings from each round; check by UUID and/or title (rather prefer duplicates over missing new findings, since merge agent will dedupe later).
- Scope: only the "reviewer" role. Skip context, merge, finalize, verify.

## Decisions (locked with user)
- `toolRoundState` (file/tool dedup) **resets** each nudge round.
- Reasoning effort **resets** to `e.config.ReasoningEffort` each round.
- One AgentRun per reviewer with summed `ToolCalls`, `DuplicateToolCalls`, `TokensUsed`; `Findings` is the accumulated length.

## Changes

### 1. Config knob — mirror `max_rate_limit_delay_seconds`

`internal/config/config.go`
- Add `const DefaultNudgeCount = 3` near line 23.
- `Profile`: add `NudgeCount int \`yaml:"nudge_count"\`` and `NudgeCountConfigured bool \`yaml:"-"\`` (around lines 53, 66).
- `Overrides`: add `NudgeCount *int` (around line 85).
- `applyOverrides` (lines 306–375): copy `NudgeCount` through, set `NudgeCountConfigured = true`.
- `normalizeProfile` (lines 377–431): if `NudgeCount == 0 && !NudgeCountConfigured` → `DefaultNudgeCount`. Reject negative.
- `markConfiguredFields` (lines 458–482): track yaml presence of `nudge_count`.

`internal/config/profiles.go:80` — extend the override-merge block to handle `NudgeCount` like `MaxRateLimitDelaySeconds`.

`internal/config/example.go:58,88` — add `nudge_count` to yaml example and default fallback.

### 2. ReviewRequest plumbing

`internal/model/types.go:29-52` — add `NudgeCount int` to `ReviewRequest`.

`cmd/nickpit/main.go`
- Add `nudgeCount int` + `nudgeCountSet bool` fields on `app` (near lines 57–58).
- Default `nudgeCount: config.DefaultNudgeCount` in `newRootCmd` (near line 91).
- Register flag with `newTrackedIntValue(&cli.nudgeCount, &cli.nudgeCountSet)` named `--nudge-count`, description: "Number of nudge rounds asking each reviewer to look again (0 disables)" (near line 128).
- In `loadProfile`, build `nudgeCount *int` from `nudgeCountSet`, pass as `Overrides.NudgeCount` (near lines 179–216).
- In all three `ReviewRequest` literals (local L314, github L369, gitlab L418), set `NudgeCount: profile.NudgeCount`.

Do NOT add to `VerifyOptions` / `FinalizeOptions` — nudge is reviewer-scoped.

### 3. Nudge loop in `runReviewAgent` (engine.go:581-660)

After the existing `runAgentLoop` call succeeds, if `agent.role == "reviewer"` and `req.NudgeCount > 0`, run N additional rounds. Pseudocode:

```go
loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{ ...existing... })
if err != nil { return reviewAgentResult{}, err }

totalFindings := append([]model.Finding(nil), loopResult.resp.Findings...)
totalTokens := loopResult.tokensUsed
totalToolCalls := loopResult.toolCalls
totalDuplicates := loopResult.duplicateToolCalls
latestResp := loopResult.resp
historyMessages := loopResult.messages

if agent.role == "reviewer" && req.NudgeCount > 0 {
    nudgeText, err := e.loadPrompt("agent_review_nudge_user_message.tmpl")
    if err != nil { return reviewAgentResult{}, err }
    for i := 0; i < req.NudgeCount; i++ {
        e.logf("Nudge round: agent=%s round=%d/%d", agent.name, i+1, req.NudgeCount)
        nudged := append(append([]llm.Message(nil), historyMessages...),
            llm.Message{Role: "user", Content: nudgeText})
        sub, err := e.runAgentLoop(ctx, agentLoopRequest{
            ...same fields...,
            Messages:        nudged,
            ReasoningEffort: e.config.ReasoningEffort, // reset
        })
        if err != nil {
            return reviewAgentResult{}, fmt.Errorf("nudge %d: %w", i+1, err)
        }
        totalFindings = // do light dedup here
        totalTokens = addTokenUsage(totalTokens, sub.tokensUsed)
        totalToolCalls += sub.toolCalls
        totalDuplicates += sub.duplicateToolCalls
        latestResp = sub.resp
        historyMessages = sub.messages
    }
    latestResp.Findings = totalFindings
}

// build reviewAgentResult using latestResp + summed counters
```

Counter/budget reset is automatic: each `runAgentLoop` invocation allocates fresh `toolState`, `jsonRetries`, `toolCalls`, `duplicateToolCalls`. No new code needed.

### 4. Nudge user-message template

New file: `prompts/agent_review_nudge_user_message.tmpl` (no template variables). Suggested content (one shot, blunt):

```
You may have missed issues.

- Re-examine the diff thoroughly
- Look for subtler issues you may have skipped
- Remember the original questions you should ask yourself
- Return JSON in the same schema as before
```

Embedded automatically by `prompts/embed.go` glob (verify it picks `*.tmpl`).

### 5. Concurrency note

Vector reviewers run as parallel goroutines (`runVectorAgents`, engine.go:454-503). The nudge loop runs sequentially **inside each goroutine**. The totalFindings slice is a goroutine-local variable. No shared mutable state — no new concurrency primitives needed. Aligns with the project guidance to route single-thread paths through concurrency-safe abstractions: nothing here introduces a new dual path.

## Critical files

- `internal/review/engine.go` (`runReviewAgent` ~L581)
- `internal/review/agent_loop.go` (read-only — confirms per-call counter reset)
- `internal/config/config.go`
- `internal/config/profiles.go`
- `internal/config/example.go`
- `internal/model/types.go`
- `cmd/nickpit/main.go`
- `prompts/agent_review_nudge_user_message.tmpl` (new)

## Risks

- **Context growth**: history doubles in size per nudge (assistant JSON + tool messages + nudge). N=3 + heavy tool use can approach provider limits. Mitigation: document interaction with `max_tokens`; revisit history trimming if reports surface.
- **Cost / latency**: roughly `(N+1)×` LLM spend per reviewer, ×6 vector reviewers. Default 3 = 4× baseline. Acceptable per user intent ("ask N additional times to make sure"); easily disabled via `--nudge-count 0`.
- **Wasted tool budget**: dedup state resets each round per user's choice — model may re-fetch same files. Accepted trade-off for strict "reset all limits" semantics.

## Verification

Unit tests in `internal/review/engine_test.go` (mock LLM):
1. `TestRunReviewAgent_NudgeDuplicate` — rounds return `[A]`, `[B]`, `[A]` (duplicate), `[C]` ⇒ final findings `[A,B,C]` (light dedup); summed token usage; one AgentRun.
2. `TestRunReviewAgent_NudgeKeepDuplicate` — rounds return `[A]`, `[B]`, `[A]` (duplicate), `[C]` ⇒ final findings `[A,B,A,C]` (light dedup fails; expected); summed token usage; one AgentRun.
3. `TestRunReviewAgent_NudgeZeroDisables` — `NudgeCount=0` ⇒ exactly one `runAgentLoop` call.
4. `TestRunReviewAgent_NudgeReviewerOnly` — role `"context"` ignores `NudgeCount`.
5. `TestRunReviewAgent_NudgeErrorPropagates` — error in round 2 ⇒ error returned, partial state not silently surfaced.

Config tests in `internal/config/config_test.go`:
- Default 3 applied; explicit `0` honored (with tracking flag set); negative rejected; CLI override beats YAML.

Manual end-to-end:
```bash
go build -o bin/nickpit ./cmd/nickpit
./bin/nickpit local branch --nudge-count=2 --verbose
# expect log lines: "Nudge round: agent=<name> round=1/2" then "round=2/2"
./bin/nickpit local branch --nudge-count=0 --verbose
# expect no "Nudge round" lines
```
