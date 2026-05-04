# Plan: Multiplexed `--show-reasoning` output via `ReasoningRenderer`

## Context

When `--show-reasoning` is set, multiple goroutines stream reasoning chunks to stderr concurrently with no synchronization, mangling output. Already noted in `TODO.md:3`.

Goal: a central renderer that owns stderr for all reasoning output whenever `--show-reasoning` is enabled — single-stream review/dedupe and parallel verifiers alike. Each active stream gets a labeled section that grows in place. Example live frame:

```
Reasoning [verifier #1: <title>]...
I think so and so is bad.
You should rather
Reasoning [verifier #2: <title>]...
I have to inspect the race condition.
It looks valid.
But let me first check again.
```

On non-TTY (piped) stderr, fall back to per-section buffering and flush each section atomically when it ends.

## Core invariant

**`ReasoningRenderer` is the only path for reasoning output when `--show-reasoning` is enabled.** `PrintReasoningBanner` / `PrintReasoningDelta` / `PrintBlankLine` are removed entirely. No dual paths. Every `collectStream` call — whether from verifier, main review loop, or dedupe — receives a `ReasoningSink`; a nil-safe lazy fallback in `collectStream` catches any future caller that forgets.

## Approach

A single `ReasoningRenderer` lives in `internal/logging`, owned by `Logger` and lazy-initialized on first use. The renderer manages a "live area" of currently-open sections. When every open section ends, the renderer commits the block (trailing newline, resets state) so subsequent normal logs land cleanly below. While at least one section is still streaming, ended siblings stay visible alongside it until the whole batch completes.

A `ReasoningSink` interface is added on `llm.ReviewRequest`. Every `Review` call site (verifier per-finding, engine main loop, engine dedupe) opens a labeled section and passes the resulting sink. As a defensive fallback, `collectStream` lazy-opens a default unlabeled section when no sink is set — so reasoning never bypasses the renderer even if a future caller forgets.

TTY redraw is hand-rolled with ANSI escapes (`CSI nA` cursor-up + `CSI 0J` clear-to-end-of-screen). Wrap-aware line counting strips ANSI sequences and divides visible-rune-width by terminal width (refetched each redraw via `golang.org/x/term`, fallback 80). One mutex guards every public method so compose-and-write is atomic.

## File-by-file

### `internal/logging/reasoning_renderer.go` (new, ~200 LOC)

```go
type ReasoningRenderer struct {
    mu        sync.Mutex
    w         io.Writer
    fd        int      // for term.GetSize
    useANSI   bool
    isTTY     bool
    sections  []*reasoningSection // currently in the live area
    lastLines int                 // wrapped rows drawn last redraw
}
type reasoningSection struct {
    label string
    buf   strings.Builder
    ended bool
}
type SectionID int

func newReasoningRenderer(w io.Writer, fd int, useANSI, isTTY bool) *ReasoningRenderer
func (r *ReasoningRenderer) Begin(label string) SectionID
func (r *ReasoningRenderer) Append(id SectionID, delta string)
func (r *ReasoningRenderer) End(id SectionID)

// internal
func (r *ReasoningRenderer) redrawLocked()
func (r *ReasoningRenderer) commitLocked()     // flushes live area → frozen, clears sections, resets lastLines
func visibleLineCount(s string, width int) int // strips CSI, counts wrapped rows
```

Lifecycle (TTY):
- `Begin` appends a section, redraws.
- `Append` mutates the section buffer, redraws.
- `End` marks the section ended. If **all** open sections are now ended, do one final redraw, then `commitLocked`: emit a trailing `\n`, drop all sections, set `lastLines=0`. Committed block stays on screen as plain text; next `Begin` starts a fresh live area below it.
- If some siblings are still active, ended sections remain rendered in the live area until the whole batch finishes.

Lifecycle (non-TTY): `Begin` is a no-op visually; `Append` buffers under lock; `End` flushes the section atomically: banner line + body + trailing newline. No cursor escapes ever emitted.

Banner format: `\x1b[33mReasoning [%s]\x1b[0m\x1b[90m...\x1b[0m\n` (label non-empty) or `\x1b[33mReasoning\x1b[0m\x1b[90m...\x1b[0m\n` (label empty, matches current single-stream visual). Body: `\x1b[3;90m%s\x1b[0m`.

Renderer is shared across the entire process; no `Close()`. Sections self-commit when their group drains.

### `internal/logging/logger.go` (~40 LOC changed)

**Remove** `PrintReasoningBanner`, `PrintReasoningDelta`, `PrintBlankLine` entirely.

Add accessor:
```go
func (l *Logger) ShowReasoning() bool { return l != nil && l.showReasoning }
```

Add lazy-init field and section wrapper (nil-safe receivers throughout):
```go
type Logger struct {
    // ...existing fields...
    reasoning     *ReasoningRenderer
    reasoningOnce sync.Once
}

type ReasoningSection struct {
    r  *ReasoningRenderer
    id SectionID
}

// Returns nil when !enabled || !showReasoning — all methods nil-safe.
func (l *Logger) OpenReasoningSection(label string) *ReasoningSection
func (s *ReasoningSection) Append(delta string)
func (s *ReasoningSection) End()
```

`OpenReasoningSection` lazy-inits the renderer (TTY detection via `os.Stderr.Stat()` `ModeCharDevice` bit; `term.GetSize` for width on first use, then per-redraw) and calls `Begin(label)`. Same renderer instance reused for every section across the process lifetime.

### `internal/llm/client.go` (~25 LOC modified)

Add public interface near `ReviewRequest`:
```go
type ReasoningSink interface {
    Append(delta string)
    End()
}
```

Add field to `ReviewRequest`:
```go
ReasoningSink ReasoningSink
```

Update `cloneReviewRequest` to copy the field. Change `collectStream` to receive the sink:
```go
func (c *OpenAIClient) collectStream(stream *openai.ChatCompletionStream, sink ReasoningSink) (*streamedResponse, error)
```

In `collectStream` (lines 866–963), replace all 5 reasoning-related logger calls (banner at 946, delta at 950, blank-lines at 898/907/912) with sink calls. Lazy fallback when caller passes nil:
```go
ensureSink := func() ReasoningSink {
    if sink == nil && c.logger != nil {
        sink = c.logger.OpenReasoningSection("") // unlabeled → "Reasoning..."
    }
    return sink
}
// on first reasoning delta:
if !reasoningStarted {
    reasoningStarted = true
}
if s := ensureSink(); s != nil { s.Append(delta) }
// EOF / error paths:
if reasoningStarted {
    if s := ensureSink(); s != nil { s.End() }
}
```

Note: `Banner()` is gone — `OpenReasoningSection` calls `Begin` which fires the banner immediately, so the sink just needs `Append` and `End`.

Thread the sink from `reviewOnce` into `collectStream`.

### `internal/review/verifier.go` (~20 LOC modified)

Add to `VerifyRequest`:
```go
ReasoningSink llm.ReasoningSink
```

In `Verify`, set `llmReq.ReasoningSink = req.ReasoningSink` before the tool loop — same sink across all loop iterations so all tool-loop reasoning appends to one section per finding.

In `VerifyAll`, inside the goroutine before constructing `req`:
```go
sec := e.logger.OpenReasoningSection(labelForFinding(idx, f))
defer sec.End() // nil-safe
req := VerifyRequest{
    // ...
    ReasoningSink: sec,
}
```

Helper:
```go
func labelForFinding(idx int, f model.Finding) string {
    title := strings.TrimSpace(f.Title)
    if title == "" { return fmt.Sprintf("verifier #%d", idx+1) }
    if len(title) > 60 { title = title[:57] + "..." }
    return fmt.Sprintf("verifier #%d: %s", idx+1, title)
}
```

### `internal/review/engine.go` (~15 LOC modified)

For each non-verifier `e.llm.Review` call site (main review loop at line 319, `reviewWithoutTools` at line 457), open a labeled section before constructing `llmReq` and close it after:
```go
sec := e.logger.OpenReasoningSection("review") // or "dedupe"
defer sec.End()
llmReq.ReasoningSink = sec
```

Labels: `"review"`, `"dedupe"`. Single-section runs render as `Reasoning [review]...\n<delta>\n\n` — same visual shape as today but routed through the renderer. The `defer sec.End()` must be inside the function scope that owns the request, not a goroutine.

### `go.mod` / `go.sum`

Add `golang.org/x/term` (consistent with existing `golang.org/x/sync`, `x/mod`, `x/tools`). Run `go mod tidy`.

## Concurrency model

- Renderer mutex covers `Begin`, `Append`, `End`, `redrawLocked`, `commitLocked`. Compose+write is atomic.
- `End` is idempotent (guarded by `section.ended` flag).
- Single shared renderer for the process — no batch lifecycle, no `Close()`. Sections self-commit when the active set drains to all-ended.
- All section/sink methods are nil-safe so callers can `defer sec.End()` without nil checks.

## Non-TTY fallback

Renderer never emits cursor escapes. `Begin` records the section but prints nothing. `Append` buffers under lock. `End` flushes the whole section atomically: banner line + body + trailing newline. Sections appear in completion order.

## Behavior when `--show-reasoning` is off

`OpenReasoningSection` returns nil. All sink method calls are nil-safe no-ops. No reasoning output. No renderer allocated. Identical to today.

## Verification

1. `go build ./... && go vet ./...` clean.
2. `go test ./internal/logging/... ./internal/llm/... ./internal/review/...` — existing tests pass.
3. **Live TTY, parallel**: run review with ≥3 findings, `--show-reasoning --verify-concurrency=4`. Expect: ≤4 stable labeled blocks growing in place, no interleaving, blocks stay visible after `Verify: done findings=...`.
4. **Live TTY, single stream**: run review with `--show-reasoning`, no verifier. Expect: `Reasoning [review]...\n<delta>\n\n` — visually equivalent to today (label is new, intentional).
5. **Piped stderr**: `... 2> reasoning.log` — each section appears as a complete contiguous block in completion order, no ANSI cursor escapes in the file.
6. **Resize**: resize terminal mid-stream — subsequent redraws re-wrap correctly (width refetched each redraw).
7. **Disabled flag**: run without `--show-reasoning` — no output changes; new code paths not entered.
8. **`NO_COLOR=1` + TTY**: cursor escapes still emitted for grouping (not color), no ANSI styling on text. Matches existing Logger semantics.

## Critical files

- `internal/logging/reasoning_renderer.go` (new)
- `internal/logging/logger.go`
- `internal/llm/client.go`
- `internal/review/verifier.go`
- `internal/review/engine.go`
- `go.mod` / `go.sum`

## Out of scope

- Interleaving non-reasoning logs (warnings etc.) above the live area — assumed nothing else logs to stderr during `VerifyAll`. Known limitation.
- Removing the `TODO.md:3` entry — do as part of the implementation commit.
