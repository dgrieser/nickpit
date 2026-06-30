# TODO

  1. Tighten reviewer/verdict policy: performance and test-gap findings need concrete user-visible failure, regression risk, or compile/runtime breakage. Otherwise downgrade/drop.
  2. Add handling for reasoning-only empty responses: retry those immediately with lower/no reasoning, instead of same-mode retries.
  3. Merge test-gap families harder: same component + same behavior under test should usually become one finding with variants.
  4. Test-gaps should only be one finding per file, and maybe a different scale. Highest be P2 or so.
  5. Fix body/suggestion duplication: body should summarize the issue; full patch/code belongs only in finalization.suggestions.
  6. If 429s persist, lower effective verifier burst slightly. Only 2/10 runs had real 429s, so not urgent.

- Add code line verification agent
- Add max turns for agents; unlimited by default
- Add feature to give additional instructions to agents

- Add man pages for commands used/called
