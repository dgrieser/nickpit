# TODO

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
