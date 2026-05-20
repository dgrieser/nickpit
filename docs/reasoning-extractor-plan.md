# Plan: Reasoning Extractor Agent

## Context

When the review agent reasons about issues, it often discusses problems it
ultimately omits from the findings JSON it returns. We currently throw that
signal away. The standard nudge prompt (`agent_review_nudge_user_message.tmpl`)
just says "look again" generically.

This plan wires a new auxiliary agent that runs in two phases:

**Phase A — per-turn extraction (async, once per reviewer turn)**
After each reviewer turn (initial call AND all following tuns BUT NOT the nudge calls), an
extractor agent is fired asynchronously. It is shown ONLY that turn's
reasoning content (not the findings JSON) and asked to compile a list
of every issue the model reasoned about in that turn. The reviewer
loop does not block on this — it proceeds while Phase A runs in the
background. Each turn produces its own Phase A list.

**Phase B — delta filtering (once per nudge, synchronous)**
Before each nudge round runs, the engine waits for all in-flight
Phase A extractors to finish, concatenates their lists into a single
combined list, then runs Phase B with (combined list, currently
accumulated findings JSON). Phase B returns only items NOT yet present
in the findings JSON. The agent is instructed to err on the side of
inclusion: when uncertain whether an item is already covered, return
it; when an item touches the same lines but a different aspect, return
it. The filtered output is appended to that round's nudge user message.

Skip rules (no extraction or filtering happens when):

- User passed `--disable-reasoning-extract`, OR
- Model-check pre-flight detected the model does not emit reasoning
  traces (`CheckSummary.Reasoning.Traces == false`), OR
- A given turn produced no reasoning content (`resp.Reasoned == false`
  or empty buffer) — that turn skips Phase A, later turns may still
  fire, OR
- Combined Phase A list is empty before a given nudge — that nudge
  skips Phase B and uses the standard template, OR
- Phase B returned an empty delta for the current round — same
  fallback.

## Architecture

### Where the new agent fits in the call graph

`runAgent` in `internal/review/engine.go` is the single choke point that
runs the initial reviewer call and then the nudge loop. The new agent is
invoked inside this function, only when `agent.role == "reviewer"`
(matching the existing nudge guard).

Sequence inside `runAgent`:

1. If reasoning extraction is enabled (see skip rules), prepare:
   - an `errgroup.Group` (or `sync.WaitGroup`) tracking in-flight
     Phase A goroutines,
   - a mutex-guarded `phaseALists []string` accumulating their results
     across the entire review.
2. **Every reviewer turn** (initial call and each nudge call) follows
   the same `runTurn` pattern:
   - Allocate a fresh `BufferedReasoningSink`, attach to
     `loopReq.ReasoningSink` via the tee.
   - Run `runAgentLoop`.
   - Detach the buffer. If `resp.Reasoned` and the buffer is non-empty,
     snapshot the buffer string and launch a goroutine that calls
     Phase A with that snapshot. The goroutine appends its result
     (skipping empty/`NONE`) to `phaseALists` under the mutex.
3. **Before each nudge** (i = 0 … NudgeCount-1):
   a. Wait for every in-flight Phase A goroutine to finish.
   b. Concatenate `phaseALists` into `combinedList`. If empty, skip
      Phase B and render the nudge with the standard template.
   c. Otherwise run **Phase B** synchronously with (`combinedList`,
      accumulated findings JSON) → `deltaList`.
   d. Build the nudge message; if `deltaList` is non-empty, append it
      to the standard nudge text. Run the nudge call via `runTurn` —
      which itself attaches a fresh buffer and fires another Phase A
      goroutine afterwards.
4. After the final nudge call, drain any still-running Phase A
   goroutines before `runAgent` returns so they do not leak.

`phaseALists` accumulates across the entire review — Phase B sees every
extracted item from every prior turn, not only the latest. The findings
JSON passed into Phase B always reflects the currently accumulated set,
so items the reviewer has since incorporated drop out naturally.

The extractor is never invoked through the nudge loop itself because
its `role` is not `"reviewer"`.

### New components

#### 1. Buffered reasoning sink — `internal/llm/reasoning_buffer.go`

```go
type BufferedReasoningSink struct {
    mu  sync.Mutex
    buf strings.Builder
}
func (b *BufferedReasoningSink) Append(delta string) { ... }
func (b *BufferedReasoningSink) End()                { /* no-op */ }
func (b *BufferedReasoningSink) String() string       { ... }
func (b *BufferedReasoningSink) Reset()               { ... }
```

Implements the existing `llm.ReasoningSink` interface
(`internal/llm/client.go`). Safe for concurrent `Append` calls because
streaming runs on a goroutine.

#### 2. Tee sink helper — same file

```go
type teeReasoningSink struct{ sinks []ReasoningSink }
func TeeReasoningSinks(sinks ...ReasoningSink) ReasoningSink { ... }
```

Used to multiplex deltas to both the existing display sink (`callSec`)
and our buffer. Nil entries are filtered.

#### 3. Wire the sink through the agent loop

- Add `ReasoningSink llm.ReasoningSink` field on `agentLoopRequest`
  (`internal/review/agent_loop.go`). Populate `llmReq.ReasoningSink`
  from it in `runAgentLoop`.
- Modify `loggedReview` (`internal/review/engine.go`) to tee instead of
  overwrite:
  ```go
  previousSink := req.ReasoningSink
  callSec := e.openReviewRequestReasoningSection(label, callNum)
  req.ReasoningSink = llm.TeeReasoningSinks(callSec, previousSink)
  defer func() { req.ReasoningSink = previousSink; callSec.End() }()
  ```
- In `runAgent`, allocate a fresh `BufferedReasoningSink` before each
  reviewer turn (initial + every nudge) and place it on
  `loopReq.ReasoningSink`. Detach (set to nil) immediately after the
  turn returns, then snapshot the buffer string for the async Phase A
#### 4. Reasoning-supported signal from model check + disable flag

Add two fields to `model.ReviewRequest` (`internal/model/types.go`):

- `ModelEmitsReasoning bool` — populated from the pre-flight model check.
- `DisableReasoningExtract bool` — populated from a new CLI flag.

CLI wiring in `cmd/nickpit/main.go`:

- Persistent flag (mirroring `disableParallelToolCalls` /
  `skipModelCheck`):
  ```go
  disableReasoningExtract bool
  // …
  root.PersistentFlags().BoolVar(&cli.disableReasoningExtract,
      "disable-reasoning-extract", false,
      "Disable the reasoning-extractor agent that augments nudge prompts "+
      "with issues the reviewer only reasoned about")
  ```
- In `runReview`, with the other `req.*` assignments:
  `req.DisableReasoningExtract = a.disableReasoningExtract`.
- After `checker.Run`:
  `req.ModelEmitsReasoning = checkResult.Summary().Reasoning.Traces`.
  When `--skip-model-check` is used, leave `ModelEmitsReasoning` at its
  zero value (`false`) — the extractor is conservatively skipped without
  a model check.

When either `req.DisableReasoningExtract` is true or
`req.ModelEmitsReasoning` is false, `runAgent` skips both the buffer
allocation and the extractor entirely.

#### 5. The extractor agent

Reuses the existing `agentSpec` / `e.runAgent` machinery. Because its
`role` is not `"reviewer"`, the nudge guard keeps it single-shot.

```go
spec := agentSpec{
    name:       fmt.Sprintf("reasoning-extract:%s", parentName),
    role:       "reasoning_extract",
    system:     renderedSystem,
    user:       renderedUser,
    schemaKind: llm.SchemaKindText,
    hasTools:   false,
}
```

Output is read from `result.contentMessages` (joined). `SchemaKindText`
is already supported.

The extractor's own reasoning is NOT collected (no buffer sink set on
its `loopReq`). Phase A goroutines all share the parent context — if
`runAgent` returns or the caller cancels, in-flight extractors are
cancelled too.

#### 6. New prompt templates (in `prompts/`, embedded via existing `embed.FS`)

Each phase has its own system prompt; both phases share the same agent
role (`reasoning-extract`) and are invoked through `e.runAgent` with
`SchemaKindText`. The output format is identical for both phases: a
plain-text list, one item per line, no headers, no JSON, no markdown;
the literal token `NONE` when the list is empty.

- `agent_reasoning_extract_phase_a_system_prompt.tmpl`
  Phase A. Defines the task as: read the reviewer's reasoning trace and
  output every distinct issue the reviewer reasoned about, regardless
  of whether the reviewer might have included it in its findings. The
  agent is explicitly told it will NOT see the findings JSON and must
  not speculate about what was or was not reported.

- `agent_reasoning_extract_phase_a_user_message.tmpl`
  Phase A. Field: `ReasoningContent string`. No findings, no prior list.

- `agent_reasoning_extract_phase_b_system_prompt.tmpl`
  Phase B. Defines the task as: given a list of issues and the
  reviewer's findings JSON, return only the items from the list that
  are NOT represented in the findings. Explicit bias toward inclusion:
    - When unsure whether an item is already covered, return it.
    - When an item touches the same file/lines as an existing finding
      but addresses a different aspect, return it.
    - Only drop an item when it is clearly the same issue as one
      already in the findings.
  Do not invent new items not present in the input list.

- `agent_reasoning_extract_phase_b_user_message.tmpl`
  Phase B. Fields: `FullList string`, `FindingsJSON string`.

#### 7. Nudge template change

Modify `prompts/agent_review_nudge_user_message.tmpl` to accept a new
field `ReasoningFindings string`. Append (only when non-empty)
something like:

```
{{if .ReasoningFindings}}

In your prior reasoning you discussed these issues but did not include
them in your findings — add them now:
{{.ReasoningFindings}}
{{end}}
```

Update the `renderPromptFile` call site in `runAgent` to pass the new
field. Pass empty string for rounds where the list is empty.

### Skip-path summary inside `runAgent`

```
extractEnabled := !req.DisableReasoningExtract && req.ModelEmitsReasoning

phaseALists := []string{}        # mutex-guarded
group       := errgroup.Group    # tracks in-flight Phase A goroutines

runTurn(loopReq):
    var buf *BufferedReasoningSink
    if extractEnabled:
        buf = new BufferedReasoningSink
        attach buf to loopReq.ReasoningSink (tee)
    resp := runAgentLoop(loopReq)
    detach buf
    if extractEnabled && resp.Reasoned && buf non-empty:
        snapshot := buf.String()
        group.Go(func() {
            list := extractor(phase A, snapshot)   # findings NOT passed in
            if list non-empty && list != "NONE":
                lock; phaseALists = append(phaseALists, list); unlock
        })
    return resp

runTurn(initial loopReq)

per-nudge round i:
    group.Wait()                             # block on all in-flight Phase A
    deltaList := ""
    combined := join(phaseALists, "\n")
    if extractEnabled && combined non-empty:
        deltaList = extractor(phase B, combined, accumulated findings JSON)
    nudge text uses deltaList (empty → unchanged template)
```

`NONE` / empty output from either phase is treated as an empty list.

### Telemetry

Each extractor run's `model.AgentRun` (tokens, etc.) is appended to the
parent reviewer's accumulated totals via the existing `addTokenUsage` /
totals tracking in `runAgent`, mirroring how nudge token usage is folded
in. Because Phase A runs concurrently, totals updates from extractor
goroutines must be routed through a concurrency-safe helper (either
guarded by the same mutex used for `phaseALists` or by an existing
single-writer abstraction). No dual paths — Phase B (synchronous) and
Phase A (async) both update totals through the same helper.

## Files to modify

- `internal/llm/reasoning_buffer.go` — **new file** (buffer + tee sinks).
- `internal/review/agent_loop.go` — add `ReasoningSink` field on
  `agentLoopRequest`; forward into `llmReq.ReasoningSink`.
- `internal/review/engine.go`
  - `loggedReview`: tee instead of overwrite.
  - `runAgent`: allocate a fresh buffer per reviewer turn (excl. 
    nudge); launch Phase A asynchronously after every turn that
    emitted reasoning; before first nudge, wait for all in-flight
    Phase A goroutines, then run Phase B synchronously against the
    combined list and the accumulated findings JSON; thread the delta
    into the nudge template render call. Use a single mutex-guarded
    `phaseALists` slice — no per-turn local list paths.
  - Add helpers `runReasoningExtractPhaseA(ctx, reasoning, parentName, turnIdx)`
    and `runReasoningExtractPhaseB(ctx, combinedList, findingsJSON, parentName)`
    near other helpers.
- `internal/model/types.go`: add `ModelEmitsReasoning bool` and
  `DisableReasoningExtract bool` to `ReviewRequest`.
- `cmd/nickpit/main.go`
  - Add `disableReasoningExtract bool` field on `app` and persistent
    flag `--disable-reasoning-extract`.
  - In `runReview`: plumb the flag into `req.DisableReasoningExtract`.
  - After `checker.Run`: set
    `req.ModelEmitsReasoning = checkResult.Summary().Reasoning.Traces`.
    When `--skip-model-check` is set, leave it `false`.
- `prompts/agent_review_nudge_user_message.tmpl` — add
  `{{if .ReasoningFindings}}…{{end}}` block.
- `prompts/agent_reasoning_extract_phase_a_system_prompt.tmpl` — **new**.
- `prompts/agent_reasoning_extract_phase_a_user_message.tmpl` — **new**.
- `prompts/agent_reasoning_extract_phase_b_system_prompt.tmpl` — **new**.
- `prompts/agent_reasoning_extract_phase_b_user_message.tmpl` — **new**.

## Reused existing utilities

- `llm.ReasoningSink` interface — `internal/llm/client.go`.
- `agentSpec` + `e.runAgent` machinery — `internal/review/engine.go`.
- `llm.SchemaKindText` — `internal/llm/client.go`.
- `renderPromptFile` / `e.loadPrompt` — already used for all prompts.
- `prompts.FS` (`go:embed`) — `prompts/embed.go`; new templates are
  picked up automatically.
- `modelcheck.CheckSummary.Reasoning.Traces` —
  `internal/modelcheck/checker.go`.
- `appendNewFindings` dedupe — already merges nudge findings.

## Verification

1. `go build ./...` — compile clean.
2. `go test ./internal/review/... ./internal/llm/...` — existing tests
   stay green; in particular `engine_test.go` (which already checks
   `ReasoningSink` is set per request) should keep passing once the
   tee plumbing is correct.
3. Add a unit test for `BufferedReasoningSink` (Append + String +
   Reset + concurrent Append safety).
4. Add an engine-level test that fakes a reviewer response with both
   findings and a known reasoning trace (via a fake `ReasoningSink`
   feeder), drives at least two reviewer turns, and asserts: Phase A
   is invoked once per turn with that turn's reasoning only; the first
   nudge blocks until all Phase A calls finish; Phase B is called with
   the joined list of all prior Phase A outputs and the accumulated
   findings JSON; and the rendered nudge message contains the appended
   delta.
5. Manual end-to-end: run `./review_test.sh` (or `nickpit local …`)
   against a model known to emit reasoning. Confirm from logs: Phase A
   fires once per reviewer turn (initial + every nudge), runs
   concurrently with the reviewer loop, and every in-flight Phase A is
   awaited before the next nudge starts; Phase B runs once per nudge
   round against the accumulated combined list; the nudge prompt
   includes the Phase B delta when non-empty; and the delta shrinks
   across rounds as the reviewer picks items up. No Phase A goroutine
   leaks past `runAgent` return.
6. Manual end-to-end with `--skip-model-check` OR a model that does
   not emit reasoning: confirm the extractor is skipped and nudges use
   the standard template unchanged.
7. Manual end-to-end with `--disable-reasoning-extract` against a
   model that DOES emit reasoning: confirm the extractor never runs
   and the nudge messages are unchanged.
