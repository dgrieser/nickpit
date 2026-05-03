# Plan: Verifier agent — re-check each finding individually

## Context

Today `review.Engine.Run` (`internal/review/engine.go:172`, returns at `:434`) drives an LLM agent that emits a list of `model.Finding` objects. The reviewer model is the only judge — there is no second pass.

The user wants a *fresh* second-pass agent that, per finding, decides:
- Is this finding actually true?
- Is the priority correct?
- How confident is the verifier in its judgement?
- Free-text remarks.

The verifier sees the same diff, has the same retrieval tools, and runs with a senior-engineer system prompt. Its output is attached to each `Finding` as a `verification` block in the existing `ReviewResult` JSON, so downstream consumers (terminal output, JSON output, future GitHub/GitLab posters) can use the data without breaking.

User decisions captured up-front:
- Always-on, opt-out via `--no-verify`.
- Parallel-bounded across findings (worker pool, default 4).
- Keep all findings; attach a `verification` block. Do not mutate original priority/confidence.
- Reuse the same limits (`max_tool_calls`, `max_duplicate_tool_calls`) per verifier call, starting from 0.

## Verifier I/O contract

### Input given to the verifier model

System prompt: new template `prompts/verify_system.tmpl` (senior-engineer voice, "verify a single finding").

User prompt JSON: same `model.ReviewPromptPayload` already produced by `model.PromptPayloadFromContext` (`internal/model/types.go:249`) — diff hunks, changed files, repo info, style guides — **plus** single `finding` object with `confidence_score` stripped:

```json
{
  "repository": {...},
  "diff_hunks": [...],
  "changed_files": [...],
  "style_guides": [...],
  "finding": {
    "title": "...",
    "body": "...",
    "priority": 1,
    "code_location": {"file_path": "x.go", "line_range": {"start": 10, "end": 12}},
    "suggestion": {...}
  }
}
```

The same five tools (`inspect_file`, `list_files`, `search`, `find_callers`, `find_callees`) are registered with identical parameter schemas.

### Output schema (returned by the verifier)

New JSON schema in `internal/llm/verify_schema.go`:

```json
{
  "valid": true,
  "priority": 1,
  "confidence_score": 0.9,
  "remarks": "..."
}
```

Semantics:
- `valid` (bool, required): finding is a real issue caused by the patch.
- `priority` (int 0–3, required): verifier's chosen priority. Equal to original `finding.priority` if unchanged; different if the verifier disagrees with priority.
- `confidence_score` (float 0–1, required): verifier's confidence that its `valid`/`priority` judgement is correct.
- `remarks` (string, required): one short paragraph explaining the verdict.

The system prompt explicitly says: "Set `priority` to the same value as the original finding's `priority` unless you believe the priority should change."

## Files to add / modify

### New files

1. **`prompts/verify_system.tmpl`** — Senior-engineer system prompt for verification. Mirrors `review_system.tmpl` structure (guidelines, `{{if .HasTools}}` tools block, `OUTPUT FORMAT` with optional `{{.OutputSchemaSnippet}}`). Tells the model: input contains one finding; verify by re-reading the diff and using tools; output ONLY the verification JSON.

2. **`internal/llm/verify_schema.go`** — `VerifySchema` (JSON schema bytes) + `VerifyExamplePromptSnippet()`, mirroring `internal/llm/schema.go:10` (`FindingsSchema`). Required fields: `valid`, `priority`, `confidence_score`, `remarks`.

3. **`internal/review/verifier.go`** — Verifier engine. Reuses tool parameter schemas from `engine.go:36-123` (`inspectFileToolParameters`, `listFilesToolParameters`, `searchToolParameters`, `callHierarchyToolParameters`) and tool execution path `executeToolCalls` (`engine.go:629`). New types and entry point:
   - `type VerifyRequest struct { ReviewCtx *model.ReviewContext; Finding model.Finding; UseJSONSchema bool; MaxToolCalls int; MaxDuplicateToolCalls int; DisableParallelToolCalls bool }`
   - `func (e *Engine) Verify(ctx context.Context, req VerifyRequest) (*model.FindingVerification, error)` — drives the same agent loop currently inlined in `Engine.Run` (body roughly `engine.go:240-433`) but with the verifier system prompt, the trimmed payload + single-finding JSON, and the verify schema. Reuses helpers `executeToolCalls`, `noToolsMessages`, `buildJSONRetryFeedback`. Must allocate its own `toolRoundState` per call (each finding is an independent agent session).
   - `func (e *Engine) VerifyAll(ctx context.Context, reviewCtx *model.ReviewContext, findings []model.Finding, opts VerifyOptions) ([]model.FindingVerification, model.TokenUsage, error)` — fans out across findings with bounded `chan struct{}` semaphore (default concurrency 4, configurable via `opts.Concurrency`). Aggregates token usage. Errors per-finding are *not* fatal: verification failure produces a `FindingVerification{ Valid: false, Remarks: "verification failed: <err>" }` with sentinel low confidence and loop continues — never drop user's findings.

   To keep tool wiring DRY, refactor tool registration block (`engine.go:246-271`) into helper `func (e *Engine) reviewerToolDefinitions() []llm.ToolDefinition` and call from both `Run` and `Verify`.

4. **`internal/llm/verify.go`** *(only if needed — see "LLM client wiring" below)*.

### Modified files

5. **`internal/model/types.go`**
   - Add `FindingVerification` struct:
     ```go
     type FindingVerification struct {
         Valid           bool    `json:"valid"`
         Severity        int     `json:"priority"`
         ConfidenceScore float64 `json:"confidence_score"`
         Remarks         string  `json:"remarks"`
     }
     ```
   - Add `Verification *FindingVerification \`json:"verification,omitempty"\`` to `Finding` (struct at `internal/model/types.go:146`, append after `Suggestion` field at `:152`). `omitempty` means existing JSON tests/output without verification continue to round-trip.

6. **`internal/review/engine.go`**
   - Extract `reviewerToolDefinitions()` helper from the inline slice at line 246.
   - No behavioural change to `Run`; the verifier orchestration stays *outside* of `Run` so `Run` keeps a single responsibility.

7. **`internal/llm/client.go`** — Decision: **reuse `Review` rather than add parallel `Verify` method**. Reasoning:
   - Whole streaming/tool-call loop, retry ladder, JSON repair, reasoning-effort fallback (`Review` at `client.go:391`) is generic over schema shape. Only *post-parse* step at `parseReviewResponse` (`client.go:1439`) is review-specific.
   - Cleanest split: factor `parseReviewResponse` (or its caller in `reviewOnce` at `client.go:578`) so schema validation step is pluggable. Add `ReviewRequest.ResponseValidator func([]byte) (parsedFindings, parsedFields, missing []string, reason string)` *or* simpler: add `ReviewRequest.SchemaKind` enum (`"review"` | `"verify"`) and branch in `parseReviewResponse`.
   - Pragmatic minimum: extend `ReviewResponse` (`client.go:113`) with optional `Verification *model.FindingVerification` and have `parseReviewResponse` handle verify schema when `req.Schema` matches `VerifySchema`. Verifier agent reads `resp.Verification` instead of `resp.Findings`.

   Going with **`SchemaKind`** approach — clearer than reflection on schema bytes, keeps `ReviewResponse` as single response struct.

8. **`internal/output/json.go`** — No change required; `Verification` rides through with `omitempty`.

9. **`internal/output/terminal.go`** — Render verification per finding (current `FormatFindings` at `:28`, finding loop at `:42`).

   Header counter (extend `:33`) gain verification tally:
   ```
   NickPit Review

   8 findings (2 P0, 3 P1, 2 P2, 1 P3) · verified: 6 valid, 2 invalid
   ```
   Only append `· verified: X valid, Y invalid` when at least one finding has `Verification != nil`. Skip when all `nil` (`--no-verify` path) so existing terminal-output golden tests stay unchanged.

   Per-finding block: insert one verifier line between header and title. Show priority disagreement explicitly via arrow:
   ```
   P1 internal/auth/token.go:42-48
     ✓ verified  conf 0.92                       ← Valid, original priority kept
   Token expiry uses < instead of <=
   <body>
   Confidence: 0.85

   P2 internal/cache/lru.go:10-15
     ✗ invalid  conf 0.88  P2→P3                 ← Verifier disagrees + downgraded priority
     remark: not reachable from public API; only invoked in test fixture.
   Cache eviction races on resize
   <body>
   Confidence: 0.71
   ```

   Rules:
   - Print verifier line only when `finding.Verification != nil`.
   - Glyph: `✓` for valid, `✗` for invalid. Drop emoji for ASCII terminals when `useANSI == false` → fall back to `[ok]` / `[bad]`.
   - Priority diff `Pa→Pb` only when `Verification.Priority != PriorityRank(finding.Priority)`. Otherwise omit.
   - Show `remark:` line indented two spaces, only when `Verification.Remarks != ""`. Truncate to 200 chars + `…` to keep terminal flow tight; full remark stays in JSON output.
   - Color (`colorize` at `:61`):
     - valid → dim green (`2;32`)
     - invalid → red (`31`) + bold when `Verification.ConfidenceScore >= 0.8` (`1;31`) so high-confidence rejections stand out
     - priority arrow → yellow (`33`)
   - Sort: extend sort at `:39` so invalid findings drop below valid ones at equal priority. Tiebreak key: `(PriorityRank, !Verification.Valid, -ConfidenceScore)`. Keeps real bugs at top.

   Optional CLI flag `--hide-invalid` (bool, default false) skips invalid findings entirely in terminal render. JSON formatter ignores the flag — invalid findings always ride through structurally.

   Add small helper `func (f *TerminalFormatter) renderVerification(v *model.FindingVerification, originalRank int) string` to keep `FormatFindings` readable; covered by golden tests in `output_test.go`.

10. **`cmd/nickpit/main.go`** —
    - Add flag `--no-verify` (bool, default false) to `runRoot` flag setup near other review flags (`main.go:92-120`).
    - Optional flag `--verify-concurrency` (int, default 4) for worker pool.
    - Optional flag `--hide-invalid` (bool, default false) — passed into `TerminalFormatter` (see step 9).
    - In `runReview` (`main.go:520`), after `engine.Run` (`main.go:559`) succeeds and *before* formatter selection (`main.go:569`), if `!noVerify`:
      1. Resolve `*model.ReviewContext` again (or have `Engine.Run` return it) — **pick option (b): change `Engine.Run` to also return trimmed context** so we don't re-resolve & re-trim (saves expensive round-trip and avoids re-walking diff).
      2. Call `engine.VerifyAll(ctx, trimmedCtx, result.Findings, verifyOpts)`.
      3. Attach each verification to its finding by index: `result.Findings[i].Verification = &verifications[i]`.
      4. Add verifier's token usage to `result.TokensUsed` (or expose separately as `result.VerifyTokensUsed` — see open question below).

    To avoid invasive changes to `Run`'s signature, alternative: introduce `Engine.RunWithContext(ctx, req) (*ReviewResult, *model.ReviewContext, error)` and keep `Run` as thin wrapper. Cleaner.

11. **`prompts/embed.go`** — Already embeds the prompts directory; auto-picks up `verify_system.tmpl`. Sanity-check no explicit allow-list exists.

12. **`TODO.md`** — Strike the verifier line.

## Reuse — what we are NOT rewriting

These are the load-bearing pieces from the existing review path that the verifier reuses verbatim:

- `executeToolCalls` and per-call dispatch (`engine.go:629`) — same tools, same dedup, same parallelism.
- `toolRoundState` (`engine.go:145`) — fresh instance per verifier call.
- `mustToolResultJSON`, `parseToolArguments`, `toolError`, `normalizeToolPath` (`engine.go:964`, `:980`, `:991`, `:1264`).
- `noToolsMessages`, `buildJSONRetryFeedback` (`engine.go:468`, `:501`) — verifier prompt template uses same `{{.HasTools}}` / `{{.OutputSchemaSnippet}}` variables so helpers re-render correctly.
- `llm.OpenAIClient.Review` and reasoning-effort fallback ladder (`client.go:391`).
- `llm.RenderPrompt`, `llm.RenderJSON`, `llm.LenientUnmarshal`.
- `Trimmer` (`internal/review/trimmer.go`) — trimmed `ReviewContext` returned by `Engine.Run` reused for every verifier call. Single trim, many verifications.

## Verification (how to test)

1. **Unit tests**
   - `internal/llm/verify_schema_test.go` mirroring `schema_test.go` — round-trip a sample verification JSON, confirm required-field validation rejects missing fields.
   - `internal/review/verifier_test.go` — table-driven test using a fake `llm.Client` that returns canned verification JSON; assert `VerifyAll` attaches verifications by index, handles per-finding errors without aborting, and accumulates token usage.
   - `internal/output/output_test.go` — extend with a finding that has a `Verification` block and assert both JSON and terminal formatters render it correctly (invalid finding shows differently than valid).

2. **Integration / smoke**
   - `go build ./...` and `go test ./...`.
   - Manual run on a small local diff:
     ```
     ./bin/nickpit local --json | jq '.findings[].verification'
     ```
     Expect each finding to have a `verification` block. Then re-run with `--no-verify` and confirm the field is absent.
   - Run on a contrived diff with both a real bug and a synthetic non-issue (stylistic only) and confirm the verifier flips `valid: false` for the non-issue while keeping `valid: true` on the real bug.

3. **Cost sanity check**
   - On a 5-finding review, log total verifier tool calls and token spend. Confirm total stays within (5 × reviewer budget) — this is the worst case the user accepted.

## Open / minor questions deferred to implementation

- Whether to surface verifier token usage as a separate field on `ReviewResult` (`VerifyTokensUsed`) vs. fold into `TokensUsed`. Lean: separate field, so existing dashboards aren't surprised by inflated numbers. Will choose during implementation.
- Whether the verifier prompt should suppress its own `priority` rewrite when `valid: false` (priority becomes meaningless). Lean: still required by schema; verifier should set it equal to the original to avoid forcing a non-decision.

## Critical paths to touch

- `internal/review/engine.go:172` (Run, ends `:434`) — return context, factor tool defs out of slice at `:246-271`.
- `internal/review/verifier.go` (new) — verifier engine.
- `internal/llm/client.go:391` (`Review`), `:578` (`reviewOnce`), `:1439` (`parseReviewResponse`) — `SchemaKind` branch for verify response.
- `internal/llm/verify_schema.go` (new) — mirrors `internal/llm/schema.go:10`.
- `internal/model/types.go:146` (Finding) — add `Verification` field; new `FindingVerification` struct.
- `cmd/nickpit/main.go:92-120` (flags), `:520` (`runReview`), `:559` (`engine.Run` call), `:569` (formatter selection) — flag + orchestration.
- `prompts/verify_system.tmpl` (new).
- `internal/output/terminal.go:28` (`FormatFindings`), `:33` (header), `:39` (sort), `:42` (finding loop), `:61` (`colorize`) — render verification block, extend header counter, secondary sort key.
