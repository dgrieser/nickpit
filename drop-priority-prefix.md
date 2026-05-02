# Drop Priority Prefixes From Finding Titles

## Summary
- Change the review contract so `finding.title` is a clean imperative title with no `[P0]`-`[P3]` prefix.
- Keep severity in `finding.priority` as the single source of truth.
- Normalize legacy/model responses by stripping leading priority prefixes from parsed titles before any formatter sees them.

## Key Changes
- Update `prompts/review_system.tmpl` to say titles must be under 80 characters, imperative, and must not include priority labels; keep the existing P0-P3 priority definitions and `priority` field instruction.
- Update `internal/llm/schema.go` so the generated example title is unprefixed, add schema guidance for clean titles, and make `priority` required in each finding.
- Add LLM response normalization in `internal/llm/client.go`: strip one or more leading case-insensitive `[P0]`-`[P3]` tags from each parsed finding title, trim whitespace, and leave `priority` unchanged.
- Add finding-level validation so missing/out-of-range `priority` is reported through the existing invalid-response retry path as `findings[i].priority`.
- Update tests, golden output, and repo examples that represent findings so they use clean titles.

## Public Contract
- `title` changes from `"[P2] Problem"` to `"Problem"`.
- `priority` remains integer `0`-`3` and becomes mandatory in the LLM output schema.
- CLI flags, terminal priority labels, and priority filtering stay unchanged; terminal output will still show `P2 file.go:10-12` before the clean title.
- No Go struct rename or broad model refactor; keep `model.Finding.Priority` shape for compatibility.

## Test Plan
- Update schema tests to assert `priority` is required and example snippets do not contain `[P#]`.
- Add/adjust client tests for legacy prefix stripping and missing priority retry validation.
- Update prompt-rendering tests to expect `"title": "Example title"` and the new no-prefix instruction.
- Update terminal formatter golden output to show `Problem` under the existing `P2` location line.
- Run `go test ./...` and a final `rg "\\[P[0-3]\\]"` check to confirm only intentional priority documentation remains.

## Assumptions
- Per your choice, legacy prefixed titles are stripped, not rejected.
- If a stripped prefix conflicts with the `priority` field, `priority` wins.
- Prefix cleanup only applies at the start of titles; priority-like text elsewhere is preserved.
