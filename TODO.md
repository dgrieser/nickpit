# TODO

- Improve merge
  1. Fix merge validator — biggest win, also fixes dups. Pre-dedupe mechanically before LLM call: drop incoming findings findingMaterialEqual to accumulator entries, count them absorbed, send only remainder. Removes deadlock, shrinks prompts, v3 saves ~18m.
     Alternatively relax required by exact-dup count?
  2. Kill the linear chain. Tree merge (6 lanes → 3 pairs parallel → 2 → 1: 3 serial rounds instead of 5) ≈ halves merge wall clock. Or single merge call over all 6 lane outputs.
  3. Merge/finalize/summarize are fine with low reasoning

- JSON retries: vLLM endpoint supports guided decoding — response_format/guided_json would eliminate 5–11 retry rounds per run.

- Add feature to give additional instructions to agents
- Add max turns for agents; unlimited by default
- Add suggestion agent to format suggestions for gitlab/github
- Remove unnecessary arguments/config.

- Add man pages for commands used/called
