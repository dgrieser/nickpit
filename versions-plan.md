## Add Toolchain Versions To Review Context

### Summary
Add best-effort toolchain version capture to the review context JSON before the first agent runs, so context/reviewer/verify/finalize prompts can see relevant Go/Python/TypeScript/JavaScript build/runtime versions.

### Key Changes
- Add `toolchain_versions` to `ReviewContext` and `ReviewPromptPayload`.
- Populate it in `Engine.RunWithContext` after context resolution and before trimming / first LLM request.
- Detect relevant languages from changed files and diff hunks.
- Run fixed commands from `RepoRoot`, no shell:
  - Go: `go version`
  - Python: `python --version`, fallback `python3 --version`
  - JavaScript: `node --version`
  - TypeScript: local `node_modules/.bin/tsc --version`, fallback `tsc --version`
- Store entries as language, command, version output, and optional error/unavailable status.
- Version capture is best-effort and must never fail review.

### Tests
- Unit test prompt payload includes `toolchain_versions` before context agent in multi-agent mode.
- Unit test missing commands produce unavailable entries without failing.
- Unit test language detection only requests relevant toolchains.
- Keep existing review/verify/finalize prompt tests passing.

### Assumptions
- “Emit” means include in agent diff/context JSON, not terminal output.
- JS version means Node.js runtime version.
- TS version means TypeScript compiler version.
- Best default: collect only languages relevant to reviewed files, not every installed local tool.
