# TODO

- Fix issues:
  1. Reject or drop “No issue / No findings / works correctly / is sound” pseudo-findings before verify, and make verifier mark them invalid if they slip
     through.

  2. Consider lower effort or small model for merge micro-clusters, or cap merge reasoning lower than 300s.
  3. Decide if fused finalizer token cost is acceptable: wall is good, but v15/v19 spend ~570–634k tail tokens on finalize shards versus v14’s 70k serial
     finalize.

- Add feature to give additional instructions to agents
- Add max turns for agents; unlimited by default
- Add suggestion agent to format suggestions for gitlab/github
- Remove unnecessary arguments/config.

- Add man pages for commands used/called
