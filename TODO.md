# TODO

1. But duplicates still leak semantically:

  - 16:55: #5 and #7 both “target-dir test uses wrong template”.
  - 17:46: #7/#8/#11 all special-char coverage variants.
  - 18:01: #6/#7/#11 similar special-char coverage.
  - 18:38: #5/#6 both user-controlled shell injection; separate variables, but likely one root-cause finding.
  - 18:50: #2/#3/#4 all shell injection root cause; #5/#10 prefix stripping conflict/overlap.
  - 19:39: #1/#8 near-duplicate file restore shell injection.
  - 19:52: #1/#6 overlap shell injection.
  - 20:02: possible #1/#5 dry-run client/wrapper, #4/#7 List error handling, but less clear.

  - Dedupe/merge reasoning conservative: “different variable/file/aspect” wins over “same root cause” too often.

  Diagnosis: merge prompt + clustering still keeps “same root cause, different affected input/file” apart too often, especially security/test variants.

2. Reasoning issues seen:

  - Some agents treat “pre-existing but adjacent” bugs as review findings. Example file templates not modified but same root cause. This may be desired,
    but if not, prompt needs stricter “introduced or directly left inconsistent by this patch”.

3. Some reasoning shows mild uncertainty around shell quoting semantics, especially prefix stripping.

---

- Add feature to give additional instructions to agents
- Add max turns for agents; unlimited by default
- Add suggestion agent to format suggestions for gitlab/github
- Remove unnecessary arguments/config.

- Add man pages for commands used/called
