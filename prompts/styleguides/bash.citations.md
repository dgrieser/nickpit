### Citations: Bash Production Coding Guidelines

Sources for `bash.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: GNU Bash Reference Manual — Shell Scripts. https://www.gnu.org/software/bash/manual/html_node/Shell-Scripts.html
    - 1. Shebang & Portability — paragraph 1: "…portability across systems where Bash may not live at `/bin/bash`."
[^2]: GNU Bash Reference Manual — The Set Builtin. https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html
    - 2. Strict Mode — `set` Flags — paragraph 1: "Add to the very top of every non-interactive production script."
    - 2. Strict Mode — `set` Flags — table row 2: "…variables as errors | Catches typos like `$USRNAME` vs `$USERNAME`"
    - 2. Strict Mode — `set` Flags — table row 3: "…it fails | Without this, `false \| true` returns 0"
[^3]: GNU Bash Reference Manual — Bourne Shell Builtins (`trap`, `exit`, `cd`, `.`). https://www.gnu.org/software/bash/manual/html_node/Bourne-Shell-Builtins.html
    - 2. Strict Mode — `set` Flags — table row 5: "ERR` to propagate into functions"
    - 9. Traps & Signal Handling › Canonical production trap pattern — paragraph 1: "**Key rules:**"
[^4]: GNU Bash Reference Manual — Arrays. https://www.gnu.org/software/bash/manual/html_node/Arrays.html
    - 4. Arrays (Indexed) — paragraph 1: "Always use `"${arr[@]}"` (not `*`) to iterate safely."
    - 5. Associative Arrays (Maps) — paragraph 2: "…to 5.3); guard data-derived keys with `[[ -n $key ]]`."
[^5]: Bash NEWS — feature list per release (bash-2.0 … bash-5.3). https://git.savannah.gnu.org/cgit/bash.git/plain/NEWS?h=bash-5.3
    - Version-Aware Review › What's new in Bash 5.3 — paragraph 1: "…completion and job notifications are taken from the release notes)."
    - 5. Associative Arrays (Maps) — paragraph 1: "Not available in `/bin/bash` on macOS (Bash 3.2)."
    - 27. Version Feature Matrix — paragraph 1: "4.0–4.3 come from the official NEWS file;"
    - 28. Removed / Changed in 5.3 — Migration Notes & `BASH_COMPAT` — paragraph 1: "…runtime from 5.2 to 5.3 is low-risk, but review these:"
[^6]: Bash COMPAT — incompatibilities between versions and the compatibility levels. https://git.savannah.gnu.org/cgit/bash.git/plain/COMPAT?h=bash-5.3
    - Version-Aware Review › Do not use or recommend (version gates) — paragraph 2: "`shopt assoc_expand_once` is a deprecated synonym for `array_expand_once` in 5.3."
    - 5. Associative Arrays (Maps) › Untrusted data in array subscripts and arithmetic (command injection) — paragraph 2: "…]]`, which is safe on every version from 4.0 on."
    - 15. Here-Docs & Here-Strings › Here-doc — bullet 5: "`[[ -f /dev/stdin ]]` behaves differently."
    - 28. Removed / Changed in 5.3 — Migration Notes & `BASH_COMPAT` — paragraph 1: "…runtime from 5.2 to 5.3 is low-risk, but review these:"
[^7]: GNU Bash Reference Manual — Word Splitting. https://www.gnu.org/software/bash/manual/html_node/Word-Splitting.html
    - 7. Whitespace Handling & IFS — paragraph 2: "Scope it instead:"
[^8]: "trap ... INT TERM EXIT" really necessary? Unix & Linux Stack Exchange. https://unix.stackexchange.com/questions/57940/trap-int-term-exit-really-necessary
    - 9. Traps & Signal Handling › Canonical production trap pattern — bullet 2: "…cleanup; use separate INT/TERM traps only for graceful shutdown signaling."
[^9]: GNU Bash Reference Manual — Exit Status. https://www.gnu.org/software/bash/manual/html_node/Exit-Status.html
    - 10. Exit Codes — paragraph 1: "**0–255**."
[^10]: GNU Bash Reference Manual — Redirections. https://www.gnu.org/software/bash/manual/html_node/Redirections.html
    - 13. `exec` & File Descriptors — paragraph 1: "…replaces the current shell process or globally redirects file descriptors."
[^11]: GNU Bash Reference Manual — Redirections: Here Documents. https://www.gnu.org/software/bash/manual/html_node/Redirections.html#Here-Documents
    - 15. Here-Docs & Here-Strings › Here-doc — bullet 1: "…**tabs only** — mixing spaces and tabs causes silent misalignment."
[^12]: ShellCheck wiki — SC2162: read without -r will mangle backslashes. https://www.shellcheck.net/wiki/SC2162
    - 16. `read` Builtin — paragraph 1: "…warning SC2162:** Always use `read -r` to avoid mangling backslashes."
[^13]: Chet Ramey, bash-5.3 release announcement ("The first public release of bash-5.3 is now available"), bug-bash mailing list, July 5, 2025. https://lists.gnu.org/archive/html/bug-bash/2025-07/msg00005.html
    - Bash Production Coding Guidelines — paragraph 1: "…i.e. 5.3.20, published September 14, 2026** (`echo "$BASH_VERSION"` → `5.3.20(1)-release`)."
[^14]: GNU FTP — official bash-5.3 patches (bash53-001 … bash53-020). https://ftp.gnu.org/gnu/bash/bash-5.3-patches/
    - Bash Production Coding Guidelines — paragraph 1: "…i.e. 5.3.20, published September 14, 2026** (`echo "$BASH_VERSION"` → `5.3.20(1)-release`)."
    - Version-Aware Review › What's new in Bash 5.3 — paragraph 2: "`"$nameä"`) being misparsed (019)."
    - 6. String Handling & Substitution › Command substitution — `$( … )` and the non-forking `${ …; }` / `${| …; }` (New in 5.3) — bullet 5: "in a no-fork substitution leaking into the enclosing command."
[^15]: GNU Bash Reference Manual — Command Substitution. https://www.gnu.org/software/bash/manual/html_node/Command-Substitution.html
    - 6. String Handling & Substitution › Command substitution — `$( … )` and the non-forking `${ …; }` / `${| …; }` (New in 5.3) — paragraph 2: "`fork()` and without a pipe:"
[^16]: GNU Bash Reference Manual — Bash Variables (`GLOBSORT`, `BASH_TRAPSIG`, `BASH_MONOSECONDS`, `BASH_COMPAT`). https://www.gnu.org/software/bash/manual/html_node/Bash-Variables.html
    - 7. Whitespace Handling & IFS › Glob options and result order — paragraph 1: "reverse name order)."
[^17]: GNU Bash Reference Manual — Shell Compatibility Mode. https://www.gnu.org/software/bash/manual/html_node/Shell-Compatibility-Mode.html
    - Version-Aware Review › Do not use or recommend (version gates) — paragraph 2: "`shopt assoc_expand_once` is a deprecated synonym for `array_expand_once` in 5.3."
    - 28. Removed / Changed in 5.3 — Migration Notes & `BASH_COMPAT` — paragraph 2: "…of array subscripts) and should not appear in new code."
[^18]: Bash CHANGES — detailed change log bash-5.2 → bash-5.3-rc2. https://git.savannah.gnu.org/cgit/bash.git/plain/CHANGES?h=bash-5.3
    - Version-Aware Review › What's new in Bash 5.3 — paragraph 1: "…completion and job notifications are taken from the release notes)."
    - 13. `exec` & File Descriptors — paragraph 2: "`ambiguous redirect` (status 1) and leaves stdin open."
[^19]: Bash git repository — `devel` branch log ("changed version to bash-5.4-devel", August 21, 2026) and release tags. https://git.savannah.gnu.org/cgit/bash.git/log/?h=devel
    - Bash Production Coding Guidelines — paragraph 1: "…`5.4_alpha_pre*` git snapshots, but these are development snapshots, not releases."
[^20]: GNU Bash Reference Manual — Bash POSIX Mode. https://www.gnu.org/software/bash/manual/html_node/Bash-POSIX-Mode.html
    - 25. POSIX Compliance & Portability — paragraph 2: "no arguments"."
[^21]: POSIX.1-2024 (IEEE Std 1003.1-2024), Shell Command Language — Dollar-Single-Quotes and the `pipefail` option. https://pubs.opengroup.org/onlinepubs/9799919799/utilities/V3_chap02.html
    - 25. POSIX Compliance & Portability — table row 18: "…✅ | ✅ since POSIX.1-2024 (older `sh` implementations lack it)"
[^22]: ShellCheck v0.11.0 release. https://github.com/koalaman/shellcheck/releases/tag/v0.11.0
    - 6. String Handling & Substitution › Command substitution — `$( … )` and the non-forking `${ …; }` / `${| …; }` (New in 5.3) — bullet 6: "ShellCheck 0.11.0 parses both forms."
[^23]: Debian sources (bash versions per suite) https://sources.debian.org/src/bash/ and Ubuntu Launchpad (bash source package) https://launchpad.net/ubuntu/+source/bash
    - Version-Aware Review › Establish the minimum Bash version first — bullet 2: "…5.2.37, Ubuntu 22.04 5.1, Ubuntu 24.04 5.2.21, Ubuntu 26.04 5.3"
[^24]: GNU Bash Reference Manual — Shell Parameter Expansion. https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html
    - 3. Variables & Parameter Expansion › `&` in the replacement of `${var/pat/rep}` — Bash 5.2+ — paragraph 1: "See the pattern-substitution rules in the manual."
