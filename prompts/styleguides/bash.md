### Bash Production Coding Guidelines

> **Target audience:** Senior engineers writing production-grade automation, DevOps tooling, and system scripts.
> **Scope:** GNU Bash 3.2 → 5.3, with version annotations on every feature that has constraints.

**Release status (October 2026).** Bash **5.3** is the current release. Chet Ramey announced it on July 5, 2025; the latest official patch is **bash53-020, i.e. 5.3.20, published September 14, 2026** (`echo "$BASH_VERSION"` → `5.3.20(1)-release`). Bash 5.2 ended at 5.2.37 (patch published September 23, 2024), 5.1 at 5.1.16, 5.0 at 5.0.18, 4.4 at 4.4.23 and 3.2 at 3.2.57 (October 2014). GNU Bash publishes no support or EOL calendar; in practice official patches appear only for the newest release series, so 5.2 and older receive fixes only through distribution backports. **There is no Bash 5.4 alpha, beta or release as of October 2026**: the upstream `devel` branch has been versioned `bash-5.4-devel` since August 21, 2026, and some distributions package `5.4_alpha_pre*` git snapshots, but these are development snapshots, not releases. Never recommend anything from 5.4 development snapshots.

This guide is self-contained. It covers **every** Bash version from 3.2 to 5.3 in one document because a shebang carries no version: `#!/usr/bin/env bash` runs whatever Bash is first in `PATH`, which is 3.2.57 for `/bin/bash` on macOS, 5.1 or 5.2 on most LTS Linux distributions and their container images, and 5.3 on Ubuntu 26.04, rolling distributions and the `bash:5.3` image. Every feature that is not available in all of these versions carries an inline annotation such as **(4.3+)** or **New in 5.3**.

***

#### Version-Aware Review

##### Establish the minimum Bash version first

Before flagging a construct as "unavailable" or recommending a newer one, determine the oldest Bash the script must run on. Evidence, in order of strength:

1. An explicit runtime guard (`(( BASH_VERSINFO[0] >= 5 ))`, `${BASH_VERSINFO[*]}` checks) or a `Requires: Bash X.Y+` header.
2. The execution environment: Dockerfile base image (`FROM bash:5.3`; Debian 12 bookworm ships 5.2.15, Debian 13 trixie 5.2.37, Ubuntu 22.04 5.1, Ubuntu 24.04 5.2.21, Ubuntu 26.04 5.3; Alpine images have no Bash unless the `bash` package is installed), CI runner images, Ansible/packaging metadata.
3. Platform: anything that must run on stock macOS uses `/bin/bash` **3.2.57** unless it explicitly finds a Homebrew/MacPorts Bash.
4. Constructs already used elsewhere in the same script or repository: if the file already uses `declare -n` (4.3+) or `${var@Q}` (4.4+), its floor is at least that version.

Rules for the reviewer:

- Verify a feature against the pinned/minimum version before claiming it is unavailable. Say "not available in Bash X.Y" only when the minimum version is known to be X.Y and the feature is newer (use the matrix in §27).
- Do not flag a 5.3 feature (`${ cmd; }`, `GLOBSORT`, `BASH_TRAPSIG`, `source -p`, …) as a bug in code that targets 5.3; do flag it when the script must also run on an older Bash — on 5.2 and older most of these are **runtime** errors (`bad substitution`, `invalid option`) or are silently ignored (`GLOBSORT`, `BASH_TRAPSIG`, `BASH_MONOSECONDS` are ordinary variables there), not parse-time errors, so the script starts and fails or misbehaves later.
- Do not recommend a feature newer than the minimum version. When suggesting a fix, prefer the oldest construct that solves the problem.
- Behaviour changes are the dangerous part of an upgrade: `patsub_replacement` (5.2, `&` in `${var/pat/rep}`), `globskipdots` (5.2), `cd ""` failing (5.3), `test` parenthesis parsing (5.3). See §28.

##### What's new in Bash 5.3

Sources: the official NEWS and CHANGES files; every non-interactive item was executed on 5.2.37 and 5.3.20 (interactive-only changes such as `read -E` completion and job notifications are taken from the release notes).

| Change | Where | Notes |
|--------|-------|-------|
| `${ cmd; }` — command substitution **without forking**; runs in the current shell | §6 | Side effects (`cd`, assignments, `exit`) persist; trailing newlines removed |
| `${\| cmd; }` — expands to `$REPLY` set by `cmd`; stdout is **not** captured | §6 | `REPLY` is local to the construct |
| `GLOBSORT` — sort pathname expansion by `name`, `numeric`, `size`, `blocks`, `mtime`, `atime`, `ctime` or `nosort`; `-` prefix reverses | §7 | Invalid values silently fall back to name order |
| `BASH_TRAPSIG` — signal number of the trap being run; `trap -P SIG` prints only the action | §9 | `BASH_TRAPSIG` is `0` in an `EXIT` trap |
| `BASH_MONOSECONDS` — monotonic clock in whole seconds | §24 | |
| `source -p PATH file` / `. -p PATH file` — search `PATH` argument instead of `$PATH` | §17 | |
| `compgen -V array` — store completions in an array instead of printing | §24 | Avoids a `$(compgen …)` subshell |
| `read -E` — `read -e` with Bash's default/programmable completion | §16 | Interactive only |
| `printf` alternate form `%#q` / `%#Q` forces single quoting; `%ls`/`%lc` (`%S`/`%C`) print wide strings/chars in multibyte locales | §6 | |
| `shopt -s array_expand_once` replaces `assoc_expand_once` (now a deprecated synonym) | §5 | |
| `shopt -s bash_source_fullpath` — absolute paths in `BASH_SOURCE` | §17 | Off by default |
| `TIMEFORMAT` precision up to 6 digits (`%6R`) | §24 | 5.2 caps at 3 |
| `test`/`[` with more than four arguments: a parenthesised sub-expression of ≤ 4 arguments is parsed with the POSIX rules | §18 | Reverted by `BASH_COMPAT=52` |
| `[[ str =~ re ]]` prints an error when `re` does not compile | §6 | Status stays `2` |
| `cd ""` is an error (`cd: null directory`, status 1) | §17 | Not reverted by `BASH_COMPAT=52` |
| `local BASH_REMATCH` works: matches inside a function no longer clobber the caller's `BASH_REMATCH` | §6 | |
| Multiple simultaneous coprocesses are enabled by default (no `coproc … still exists` warning) | §14 | |
| New loadable builtins `kv`, `strptime`, `fltexpr` (the 5.3 build also installs a `chmod` loadable) | §24 | Only if the distribution ships the loadables |
| POSIX mode: function names need not be identifiers; `kill` fails if any target is not found; `test` `<`/`>` use the locale; `wait -n` removes reaped jobs; new job-notification rules | §25 | |
| `exit` with no argument at a trap's top level uses the pre-trap `$?` (POSIX interp. 1602) | §9 | 5.2 used the trap's last status |
| `BASH_COMPAT=52` restores 5.2 behaviour for `test` parsing, `bind -p/-P` arguments and job notifications while sourcing | §28 | |

Point releases matter: **bash53-007** fixed no-fork command substitutions whose redirections leaked into the enclosing command, and later patches fix a subshell or background job whose `EXIT` trap calls `wait` looping on inherited jobs (010), `mapfile` callbacks that unset the target array (011/014), a subshell running an inherited `EXIT` trap after a fatal signal (012), `read` buffer indexing (015) and, on macOS, a variable name directly followed by a byte ≥ 0x80 (e.g. `"$nameä"`) being misparsed (019). Prefer 5.3.7 or newer when relying on `${ cmd; }` with redirections.

##### Do not use or recommend (version gates)

Never suggest these to code whose minimum version is below the listed one:

- **5.3:** `${ cmd; }`, `${| cmd; }`, `GLOBSORT`, `BASH_TRAPSIG`, `BASH_MONOSECONDS`, `trap -P`, `source -p`/`. -p`, `compgen -V`, `read -E`, `printf %#q`/`%#Q`, `shopt -s array_expand_once`, `shopt -s bash_source_fullpath`, `TIMEFORMAT` precision above 3, loadables `kv`/`strptime`/`fltexpr`.
- **5.2:** `${var@k}`, `printf %Q`, `shopt -s patsub_replacement` (on by default), `globskipdots` (on by default), `varredir_close`, `noexpand_translation`, `@`/`*` as associative-array keys in assignments and `test -v`.
- **5.1:** `SRANDOM`, `wait -p var`, `wait -n` with job arguments, `${var@U}`/`${var@u}`/`${var@L}`/`${var@K}`, `declare -I`/`local -I`, key-value associative compound assignment `declare -A m=(k1 v1 k2 v2)`, `test -v N` for positional parameters.
- **5.0:** `EPOCHSECONDS`, `EPOCHREALTIME`, `BASH_ARGV0`, `wait -f`, `shopt -s localvar_inherit`, `shopt -s assoc_expand_once`.
- **4.4:** `shopt -s inherit_errexit`, `${var@Q}`/`@E`/`@P`/`@A`/`@a`, `local -`, `mapfile -d`.
- **4.3:** `declare -n`/`local -n` namerefs, `[[ -R var ]]`, assigning/unsetting negative indices (`a[-1]=x`), `wait -n`, `[[ -v arr[i] ]]` with array references.
- **4.2:** `[[ -v var ]]`, `declare -g`, `printf '%(fmt)T'`, `shopt -s lastpipe`, reading negative indices `${a[-1]}`.
- **4.1:** `{fd}>file` automatic file-descriptor allocation, `printf -v 'arr[i]'`.
- **4.0:** associative arrays, `mapfile`/`readarray`, `${var^^}`/`${var,,}`, `declare -l`/`-u`, `&>>`, `|&`, `globstar`, `coproc`, `BASHPID`, fractional `read -t`.

Deprecated or removed: the per-version `shopt` options `compat31` … `compat44` still exist in 5.3 but are deprecated in favour of `BASH_COMPAT` (the only mechanism for levels 50 and later); `BASH_COMPAT` accepts 31–53 in 5.3 (`30` is rejected); `shopt assoc_expand_once` is a deprecated synonym for `array_expand_once` in 5.3.

***

#### 1. Shebang & Portability

Always specify the interpreter. Prefer `#!/usr/bin/env bash` for portability across systems where Bash may not live at `/bin/bash`.

```bash
#!/usr/bin/env bash
# Good — finds bash in PATH; works on macOS, Linux, NixOS, etc.

#!/bin/bash
# Acceptable on known-Linux systems; on macOS this always runs the system
# Bash 3.2.57, even when a newer Bash is installed via Homebrew/MacPorts

#!/bin/sh
# Use ONLY when you intend POSIX sh, not Bash — you lose arrays, [[ ]], etc.
```

**macOS warning:** macOS ships Bash 3.2.57 at `/bin/bash` (the last GPLv2 release; the default interactive shell has been zsh since macOS 10.15). Associative arrays, `mapfile`/`readarray`, `${var,,}`/`${var^^}`, `declare -n`, `[[ -v ]]`, `{fd}>` redirections, `printf '%(…)T'`, fractional `read -t`, `&>>`, `globstar` and `inherit_errexit` are **not available** there. If your scripts must run on stock macOS, either require a newer Bash (Homebrew/MacPorts, found through `#!/usr/bin/env bash` and an explicit `BASH_VERSINFO` check) or restrict yourself to Bash 3.2 constructs.

`#!/usr/bin/env bash` cannot pass more than one argument portably (`#!/usr/bin/env bash -e` fails on Linux, where the kernel passes `bash -e` as one word); put `set -euo pipefail` in the script body instead.

***

#### 2. Strict Mode — `set` Flags

Add to the very top of every non-interactive production script.

```bash
#!/usr/bin/env bash
set -euo pipefail
```

| Flag | Meaning | Notes |
|------|---------|-------|
| `-e` | Exit immediately on non-zero exit code | Ignored in `if`/`while`/`until` conditions, in every command of an `&&`/`\|\|` list except the last, after `!`, and in **everything** (including function bodies) called from those contexts |
| `-u` | Treat unset variables as errors | Catches typos like `$USRNAME` vs `$USERNAME` |
| `-o pipefail` | Pipeline fails if **any** command in it fails | Without this, `false \| true` returns 0 |
| `-x` | Trace every command (debug) | Use in CI or dev, not production by default |
| `-E` | Inherit `ERR` trap in functions/subshells | Required for `trap ... ERR` to propagate into functions |
| `-T` | Inherit `DEBUG`/`RETURN` traps in functions | Like `-E`, but for `DEBUG`/`RETURN` traps |
| `-C` | `noclobber`: refuse to overwrite files with `>` | Force with `>\|`; see §12 |
| `-n` | `noexec`: parse but do not execute | `bash -n script.sh` = syntax check in CI |

Every flag also has a long name via `set -o`: `errexit` (-e), `nounset` (-u),
`xtrace` (-x), `errtrace` (-E), `functrace` (-T), `noclobber` (-C), `noexec` (-n).
The remaining `set -o` names (`allexport`, `noglob`, `monitor`, `posix`,
`histexpand`, `vi`/`emacs`, …) are mostly for interactive or niche use.

```bash
# Full production header
#!/usr/bin/env bash
set -euo pipefail
```

**`set -e` caveats:**

```bash
# set -e does NOT abort here — the if-test context suppresses it
if grep -q "pattern" file; then
  echo "found"
fi

# Deliberately ignore a command's failure
some_command || true          # explicit ignore
some_command || { echo "failed" >&2; exit 1; }  # handle gracefully
```

**`set -e` does not reach into command substitutions.** A `( … )` subshell
inherits `-e` (it exits on the first failure, and on 4.4+ its non-zero status
then aborts the parent; Bash 3.2 continues after a failed subshell — write
`( … ) || exit` there), but in non-POSIX mode Bash clears `-e` inside `$( … )` — and inside
the 5.3 `${ …; }` form — so `x=$(false; echo after)` silently continues.
Re-enable it with `shopt -s inherit_errexit` (**4.4+**; there is no equivalent on
3.2, so check the status of each command inside the substitution there):

```bash
set -euo pipefail
shopt -s inherit_errexit      # 4.4+: $(...) and ${ ...; } now honor set -e
```

With `inherit_errexit`, a failure inside `$( … )` exits only the substitution's
subshell (the assignment then fails and `set -e` aborts the script); a failure
inside the non-forking `${ …; }` form (**5.3+**) exits the **whole shell**
immediately, because the command runs in the current shell. The `EXIT` trap
still runs, but its standard output is still redirected into the substitution
and is discarded, so cleanup/diagnostic traps must write to stderr. Note that an
assignment that is itself tested (`x=$(cmd) || handle`) disables `-e` inside the
substitution even with `inherit_errexit`.

**`set -e` is disabled for an entire function called in a tested context.**
`deploy || handle_failure`, `if deploy; then`, `deploy && next`, and `! deploy`
run *all* of `deploy`'s body with `-e` ignored — a failing command in the middle
of `deploy` does not stop it, and `set -e` inside the function cannot re-enable
it. Functions that are called this way must check every command explicitly
(`cmd || return`).

```bash
set -e
f() { false; echo "still running"; }   # prints "still running"
f || echo "f failed"                    # never prints: f returns echo's status 0
```

**`local`/`declare`/`export`/`readonly` mask a command's exit status.** In
`local x=$(cmd)` the status seen is that of `local` (always 0), so `set -e` never
sees `cmd` fail (ShellCheck SC2155). Declare, then assign, separately:

```bash
get_value() {
  local x
  x=$(cmd) || return          # cmd's failure is now visible to set -e / callers
  printf '%s\n' "$x"
}
```

**`set -u` caveats:**

```bash
# Use default values to avoid unbound variable errors
: "${CONFIG_FILE:=/etc/myapp/config}"
echo "${OPTIONAL_VAR:-}"       # safe: expands to empty string if unset
```

**`pipefail` caveat:** a benign `SIGPIPE` pipeline can still fail — e.g.
`cmd | head -1` returns non-zero when `cmd` writes more than `head` reads. Add
`|| true` when the early exit is expected.

**Do not put strict mode in sourced libraries.** `set -e`/`-u`/`pipefail` are
shell-global and leak into the sourcing shell. Enable strict mode in executable
entry points only, not in files meant to be `source`d.

***

#### 3. Variables & Parameter Expansion

##### Naming conventions

```bash
# SCREAMING_SNAKE_CASE for exported/environment variables
export DATABASE_URL="postgres://localhost/mydb"

# lowercase_snake_case for local/script variables (use `local` only inside functions)
retry_count=0

# _prefixed for "private" helpers
_tmp_file=$(mktemp)
```

##### Declaration attributes

```bash
declare -r  CONST_VAL="immutable"   # readonly
declare -i  counter=0               # integer — arithmetic on assign
declare -l  lower_var="HELLO"       # auto-lowercase (Bash 4.0+)
declare -u  upper_var="hello"       # auto-uppercase (Bash 4.0+)
declare -x  EXPORTED="yes"          # same as export
declare -g  global_from_func="yes"  # set global from within function (Bash 4.2+)
declare -I  inherited               # local that inherits the caller's value/attributes (Bash 5.1+)
```

##### Essential parameter expansions

```bash
var="hello world"

# Default values
${var:-default}       # use default if var is unset OR empty
${var-default}        # use default ONLY if var is unset
${var:=default}       # assign default if var is unset or empty
${var:?error msg}     # abort with error if var is unset or empty
${var:+alternate}     # use alternate if var IS set and non-empty

# Length
${#var}               # length: 11

# Substring
${var:6}              # "world"
${var:0:5}            # "hello"
${var: -5}            # "world" (space before negative is required)

# Pattern removal
${var#pattern}        # remove shortest match from start
${var##pattern}       # remove longest match from start
${var%pattern}        # remove shortest match from end
${var%%pattern}       # remove longest match from end

# Substitution
${var/world/earth}    # replace first match: "hello earth"
${var//l/L}           # replace all:         "heLLo worLd"
${var/#hel/HEL}       # replace only at start: "HELlo world"
${var/%rld/RLD}       # replace only at end:   "hello woRLD"

# Case conversion (Bash 4.0+)
${var^}               # capitalize first char:  "Hello world"
${var^^}              # all uppercase:           "HELLO WORLD"
${var,}               # lowercase first char:    "hello world"
${var,,}              # all lowercase:           "hello world"
```

##### Always quote variables

```bash
# BAD — word splitting and globbing
rm $filename
cp $src $dst

# GOOD
rm -- "$filename"
cp -- "$src" "$dst"

# BAD — especially dangerous with wildcards
rm -rf $dir/*

# GOOD
rm -rf "${dir:?}/"*   # :? aborts if dir is empty — prevents rm -rf /*
```

##### Pattern vs. literal in `#`/`%` stripping and `[[`

`${var#word}`, `${var##word}`, `${var%word}`, `${var%%word}` remove a **glob
pattern**, not a literal string — `word` may contain `*`, `?`, `[...]`. When the
prefix/suffix comes from data and must be literal, quote it *inside* the
expansion:

```bash
rest="${value#"$prefix"}"     # literal prefix (quotes inside the expansion)
rel="${path#"$root"/}"        # literal directory prefix + slash
value="${arg#--name=}"        # static pattern — fine, no data-controlled globs
```

Inside `[[ ... ]]`, the right side of `==` is also a pattern unless quoted. For a
literal prefix followed by anything, quote the prefix and leave the wildcard
outside it:

```bash
[[ $path == "$root"/* ]]      # literal root prefix, wildcard after the slash
[[ $path == $root/* ]]        # WRONG if $root contains glob characters
```

If the prefix must be present, check it explicitly before stripping:

```bash
case "$path" in
  "$root"/*) rel="${path#"$root"/}" ;;
  *) echo "ERROR: path outside root: $path" >&2; return 1 ;;
esac
```

##### `&` in the replacement of `${var/pat/rep}` — Bash 5.2+

See the pattern-substitution rules in the manual.

Since **5.2** the `patsub_replacement` option is **on by default**: an unquoted
`&` in the replacement string expands to the matched text (like `sed`). Code that
inserts data containing `&` (URLs, query strings, HTML entities) silently changes
output on 5.2+:

```bash
s='a.b'; r='&x'
echo "${s//./$r}"     # 3.2–5.1: a&xb    5.2+: a.xb   (& replaced by the match ".")
x=${s//./"$r"}        # GOOD on every version 3.2–5.3: a&xb (quoted replacement is literal)
echo "${s//./\&}"     # literal & via backslash: a&b on 4.4–5.3; 3.2 keeps the backslash: a\&b
```

Quote a data-derived replacement *inside* the expansion. Do this in an
assignment or another context that is not itself double-quoted: in Bash 3.2
(and under `BASH_COMPAT=42` or lower) `"${s//./"$r"}"` keeps the inner quotes
literally (3.2 prints `a"&x"b`). On 5.2+ `shopt -u patsub_replacement` restores the old
behaviour globally, but `shopt` with that name fails on older versions. Since
5.3 the option's default is a build-time setting, so a distribution may ship
it off: code must be correct with either setting, which quoting guarantees.

##### Parameter transformations `${var@op}` (4.4+, extended in 5.1 and 5.2)

```bash
v="it's a b"
printf '%s\n' "${v@Q}"     # 'it'\''s a b' — shell-quoted, re-usable as input (4.4+)
e='a\tb'; printf '%s\n' "${e@E}"   # expands backslash escapes like $'...' (4.4+)
declare -a arr=(1); echo "${arr@a}"  # attributes: a (4.4+)
echo "${v@A}"                        # an assignment statement that recreates v (4.4+)
echo "${v@U} ${v@u} ${v@L}"          # upper / first-upper / lower (5.1+)
declare -A m=([k]=v)
echo "${m[@]@K}"            # k "v"  — quoted key/value pairs, one word (5.1+)
echo "${m[@]@k}"            # k v    — separate words, usable in an assignment (5.2+)
```

`${v@Q}` is the safe way to log or `eval`-construct a command line; prefer
`printf '%q '` when 3.2 support is required.

##### Nameref (indirect variable reference) — Bash 4.3+

```bash
# Bash 4.3+ — declare -n creates a reference (pointer) to another variable
increment() {
  local -n _ref="$1"   # _ref is a nameref to the variable named by $1
  (( _ref++ ))
}

count=5
increment count
echo "$count"  # 6

# WARNING: do NOT name the nameref the same as the target — circular reference
# ("warning: _ref: circular name reference"); prefix nameref names (_ref, __out)
# so a caller's variable cannot collide with them.
```

On 3.2–4.2 use `printf -v "$name" '%s' "$value"` (3.1+) to assign indirectly and
`${!name}` to read; never `eval "$name=\$value"` with an unvalidated `$name`.

***

#### 4. Arrays (Indexed)

Indexed arrays are available since Bash **2.0**. Always use `"${arr[@]}"` (not `*`) to iterate safely.

```bash
# Declaration
declare -a fruits=()

# Inline init
fruits=("apple" "banana" "cherry")

# Append
fruits+=("date")
fruits[${#fruits[@]}]="elderberry"  # same as += ONLY if the array has no gaps; prefer +=

# Access
echo "${fruits[0]}"         # apple  ("${fruits}" also gives element 0 — SC2128, avoid)
echo "${fruits[-1]}"        # elderberry (reading a negative index: 4.2+; a[-1]=x / unset 'a[-1]': 4.3+)
echo "${fruits[@]: -1}"     # elderberry on every version (3.2+): negative slice offset
echo "${fruits[@]}"         # all elements (each as separate word when quoted with @)
echo "${fruits[*]}"         # all elements joined by first char of IFS — avoid in most cases

# Length
echo "${#fruits[@]}"        # number of elements

# Slice
echo "${fruits[@]:1:2}"     # elements 1 and 2: "banana cherry"

# Keys / indices
echo "${!fruits[@]}"        # 0 1 2 3 4

# Delete element (leaves gap in indexed array)
unset 'fruits[2]'

# Delete entire array
unset fruits

# Safe iteration (handles spaces in values)
for fruit in "${fruits[@]}"; do
  echo "$fruit"
done

# Read file into array (Bash 4.0+)
mapfile -t lines < file.txt         # -t strips the trailing newline of each line
readarray -t lines < file.txt       # synonym for mapfile

# Read command output into array (process substitution, not pipe — avoids subshell)
mapfile -t pids < <(pgrep nginx)

# mapfile -d delimiter (Bash 4.4+)
mapfile -d '' -t null_separated < <(find . -print0)
```

**Pitfall:** `"${arr[*]}"` in double quotes joins all elements with the first character of `IFS`. Almost always use `"${arr[@]}"`.

**Pitfall (3.2–4.3):** under `set -u`, expanding an *empty* array (`"${arr[@]}"`)
is an "unbound variable" error before Bash 4.4. Use `${arr[@]+"${arr[@]}"}` when
the script must run on 3.2 (macOS) with `set -u`.

**Bash 3.2 replacement for `mapfile`:**

```bash
lines=()
while IFS= read -r line || [[ -n $line ]]; do   # || handles a missing final newline
  lines+=("$line")
done < file.txt
```

```bash
# Pass array to function — pass by name + nameref (Bash 4.3+)
process_list() {
  local -n _list="$1"
  for item in "${_list[@]}"; do
    echo "Processing: $item"
  done
}
process_list fruits
```

***

#### 5. Associative Arrays (Maps)

Requires Bash **4.0+**. Not available in `/bin/bash` on macOS (Bash 3.2).

```bash
# Declaration — -A is mandatory
declare -A config

# Inline init (Bash 4.0+)
declare -A config=(
  [host]="localhost"
  [port]="5432"
  [user]="admin"
)

# Bash 5.1+ also accepts a key/value list (an error on 4.x–5.0):
# declare -A config=( host localhost port 5432 )  # Bash 5.1+

# Set / update
config[db_name]="production"

# Access
echo "${config[host]}"              # localhost

# Default value for missing key
echo "${config[missing_key]:-none}" # none

# Check if key exists
if [[ -v config[port] ]]; then      # -v: 4.2+; with an array subscript: 4.3+
  echo "port is set"
fi

# All keys
echo "${!config[@]}"

# All values
echo "${config[@]}"

# Iterate key-value pairs
for key in "${!config[@]}"; do
  printf '%s = %s\n' "$key" "${config[$key]}"
done

# Delete key
unset 'config[user]'

# Delete entire map
unset config

# Read-only map
declare -rA CONSTANTS=([VERSION]="1.2.3" [APP]="myapp")

# Inspect / debug
declare -p config    # prints: declare -A config=([host]="localhost" ...)
```

**Pitfall:** Empty strings cannot be used as associative array keys (`m[""]=1` → "bad array subscript" in every version up to 5.3); guard data-derived keys with `[[ -n $key ]]`.

##### Untrusted data in array subscripts and arithmetic (command injection)

Indexed-array subscripts and every arithmetic context (`$(( ))`, `(( ))`,
`let`, `declare -i`, `[[ $a -eq $b ]]`) **evaluate** their operands. A value
that contains an array reference such as `a[$(cmd)0]`, read from input, runs
`cmd` when it is used there — in every version through 5.3. (A bare `$(cmd)0`
is only an arithmetic syntax error, because the expanded value is not
re-expanded; the array-reference form is the exploit. `[ "$x" -eq 1 ]` with
single brackets accepts only integer literals and is not affected.)

```bash
idx='a[$(touch /tmp/pwned)0]'
: "${arr[$idx]}"            # BAD: runs touch (indexed subscript is arithmetic)
n='a[$(touch /tmp/pwned)0]'
echo $(( n + 1 ))           # BAD: runs touch (variable value evaluated recursively)

# GOOD: validate before any arithmetic use
[[ $n =~ ^[0-9]+$ ]] || { echo "not a number: $n" >&2; exit 2; }
echo $(( 10#$n + 1 ))       # 10# also stops "08"/"09" from being parsed as octal
```

Associative-array keys used in `(( A[$key]++ ))` or `[[ -v A[$key] ]]` were
expanded twice up to 5.1, so a key like `x$(cmd)` ran `cmd`; 5.0/5.1 need
`shopt -s assoc_expand_once` (on 5.0 it protects only arithmetic commands —
`[[ -v A[$key] ]]` and `[[ A[$key] -gt 0 ]]` still run `cmd` there; 5.1 covers
`[[ ]]` too), and 5.2+ behave that way by default (the option is
named `array_expand_once` in 5.3, with `assoc_expand_once` kept as a deprecated
synonym). `test -v "A[$key]"` and `[ -v "A[$key]" ]` **still** execute the
substitution on 5.3 — use `[[ -v A[$key] ]]` only on 5.2+, or test membership
with `[[ ${A[$key]+set} ]]`, which is safe on every version from 4.0 on.

**Bash 3 workaround** (when you must support macOS or old systems):

```bash
# Simulate map with nameref-style eval (avoid eval in production — use Bash 4+)
# Or use file-backed key=value storage parsed with IFS= read -r
while IFS='=' read -r key val; do
  case "$key" in
    host) HOST="$val" ;;
    port) PORT="$val" ;;
  esac
done < config.env
```

***

#### 6. String Handling & Substitution

##### Built-in operations (no subprocess)

```bash
str="Hello, World!"

# Length
echo "${#str}"              # 13

# Uppercase / lowercase (Bash 4.0+)
echo "${str^^}"             # HELLO, WORLD!
echo "${str,,}"             # hello, world!

# Trim prefix / suffix
path="/usr/local/bin/myapp"
echo "${path##*/}"          # myapp         (basename equivalent)
echo "${path%/*}"           # /usr/local/bin (dirname equivalent)

# Extension stripping
file="archive.tar.gz"
echo "${file%%.*}"          # archive
echo "${file##*.}"          # gz
echo "${file%.*}"           # archive.tar

# Replace
s="foo_bar_foo"
echo "${s/foo/baz}"         # baz_bar_foo   (first only)
echo "${s//foo/baz}"        # baz_bar_baz   (all)
echo "${s/#foo/baz}"        # baz_bar_foo   (only at start)
echo "${s/%foo/baz}"        # foo_bar_baz   (only at end)

# Substring extraction
echo "${str:7:5}"           # World
echo "${str: -6:5}"         # World  (negative offset — space required)
```

##### Regex matching with capture groups

```bash
# =~ operator with BASH_REMATCH (Bash 3.0+; RHS-quoting semantics changed in 3.2)
date_str="2024-07-07"
if [[ $date_str =~ ^([0-9]{4})-([0-9]{2})-([0-9]{2})$ ]]; then
  year="${BASH_REMATCH[1]}"
  month="${BASH_REMATCH[2]}"
  day="${BASH_REMATCH[3]}"
fi
```

- Keep the regex **unquoted** on the right of `=~` (quoted parts match
  literally since 3.2) and store complex patterns in a variable:
  `re='^v([0-9]+)\.([0-9]+)$'; [[ $tag =~ $re ]]`.
- An invalid regex makes `[[ … =~ … ]]` return **2**, which `if` treats as
  "no match". **New in 5.3:** Bash also prints `invalid regular expression` to
  stderr; on 5.2 and older the failure is silent.
- `BASH_REMATCH` is global. **New in 5.3:** `local BASH_REMATCH` inside a
  function keeps the function's matches from overwriting the caller's; on 5.2
  and older the `local` declaration is ignored by `=~` and the global is
  overwritten — copy the captures into locals immediately after the match.

##### String splitting

```bash
# Split on delimiter using read + IFS
IFS=',' read -ra parts <<< "a,b,c,d"
echo "${parts[1]}"           # b

# Split multi-line output into array
mapfile -t lines <<< "$(command)"

# Avoid: for word in $string — word-splits and globs
```

##### `printf` vs `echo`

```bash
# Prefer printf for variables — echo mishandles values that look like options
# (-n, -e, -E, -neE…) and, with xpg_echo or under sh, backslash escapes; plain literal echo is fine
var="-n"
echo "$var"          # Bash prints nothing at all: the value is taken as the -n option

# ALWAYS use printf for reliable output
printf '%s\n' "$var"  # always prints: -n
printf 'Count: %d\n' "$count"
printf '%s\t%s\n' "$key" "$value"

# %q — shell-quote a value so it can be safely reused as shell input.
# Ideal for logging or reconstructing a command line without quoting bugs.
printf '%q\n' "-n tricky"        # -> -n\ tricky
printf 'running:'; printf ' %q' "$@"; printf '\n'   # -> running: cmd arg\ with\ spaces ...
# Bash 5.2+: %Q is like %q but applies the precision to the UNQUOTED argument
printf '%.3Q\n' 'a b c'           # -> a\ b   (%.3q would truncate the quoted text: a\ )
# Bash 5.3+: the alternate form %#q / %#Q always uses single quotes
printf '%#q\n' "it's"             # -> 'it'\''s'   (5.2 ignores #: it\'s)
```

`printf %d` with a non-numeric argument prints a diagnostic (`invalid number`),
outputs `0` — or, on 4.4–5.3, the leading digits (`12abc` → `12`; 3.2 prints
`0`) — and returns status 1 in every version; check the status or validate the
input first.

**5.3:** `%ls`/`%lc` (`%S`/`%C`) treat the argument as a wide string/character
in a multibyte locale, so the precision counts characters instead of bytes:
under `LC_ALL=C.UTF-8`, `printf '%.1ls' 'äb'` prints `ä`, while `%.1s` prints
only the first byte of `ä` (an invalid sequence). On 5.2 `%ls` behaves like
`%s`.

##### Command substitution — `$( … )` and the non-forking `${ …; }` / `${| …; }` (New in 5.3)

`$(cmd)` runs `cmd` in a forked subshell, captures stdout and strips **all**
trailing newlines. Use `$(< file)` instead of `$(cat file)` (no `cat` process).
To keep trailing newlines, append a sentinel: `out=$(cmd; printf x); out=${out%x}`.

**New in 5.3** — two forms run the command **in the current shell**, without
`fork()` and without a pipe:

```bash
# ${ cmd; } — captures stdout like $(...), trailing newlines removed
count=0
out=${ count=5; printf 'hi\n\n'; }
echo "$out $count"        # hi 5   — the assignment persisted

# ${| cmd; } — expands to $REPLY as set by cmd; cmd's stdout is NOT captured
to_upper() { REPLY=${1^^}; }
name=${| to_upper "$USER"; }

# The space/tab/newline (or |) after "${" and the ";" (or newline) before "}" are mandatory
x=${ date +%s; }          # GOOD
x=${date +%s}             # bad substitution: not a funsub
```

Semantics the reviewer must check:

- **Side effects persist.** `cd`, `set --`, `shift`, `set -e`/`shopt`
  changes, traps, variable assignments and `exit` act on the calling shell
  (`${ exit 3; }` terminates the script with status 3). Only use the form when
  that is intended; a refactor from `$(…)` to `${ …; }` changes behaviour if the
  body relies on subshell isolation (`cd` in a subshell is a common example).
- **Function-like scope.** `local` inside the construct creates variables local
  to it, and `return` ends it early; positional parameters are shared with the
  caller. In `${| …; }`, `REPLY` is a fresh local: the caller's `REPLY` is
  restored afterwards, and trailing newlines in `REPLY` are **kept**.
- **Exit status** is that of `cmd`, as with `$( … )`; `set -e` is ignored
  inside unless `inherit_errexit` is set, and then a failure exits the whole
  script (see §2). Any `EXIT` trap that fires there writes its stdout into the
  substitution.
- **Not portable.** Bash 5.2 and older parse the script and fail only at
  **run time** with `bad substitution`: a non-interactive shell abandons that
  line with status 1; without `set -e` (and outside POSIX mode) it continues
  with the **next** line, leaving the variable unchanged, so later code runs
  with stale data.
  Guard with `(( BASH_VERSINFO[0] > 5 || (BASH_VERSINFO[0] == 5 && BASH_VERSINFO[1] >= 3) ))`
  or keep `$( … )` / `printf -v`.
- **Use 5.3.7+** if the body uses redirections: bash53-007 fixed redirections
  in a no-fork substitution leaking into the enclosing command.
- ShellCheck 0.11.0 parses both forms.

On older versions, the fork-free alternatives are `printf -v var …` (3.1+),
namerefs (4.3+) and functions that set a global such as `REPLY`.

##### Quoted newlines are literal, not command separators

A newline inside a correctly quoted word is part of that single argument, not a
command separator. Single quotes preserve every character except `'`; double
quotes preserve embedded newlines too.

```bash
printf '<%s>\n' 'a
b'                     # ONE argument that contains a newline
```

A newline may still be invalid for a file format, log line, or downstream tool.
When a value must be single-line, treat that as an input-validation /
compatibility requirement — **not** a shell-injection issue, when the value is
already correctly quoted.

***

#### 7. Whitespace Handling & IFS

`IFS` (Internal Field Separator) defaults to space, tab, newline.
Changing it affects `read`, `for`, and word splitting.

**Prefer scoping `IFS` to the command or function that needs it over setting a
global default.** A global `IFS=$'\n\t'` is a common recommendation but changes
word-splitting everywhere and has surprising effects: `"$*"` and `"${*:2}"` join
on IFS's **first character**, so under `$'\n\t'` they concatenate with newlines
instead of spaces. Scope it instead:

```bash
# Scope IFS to a single command — does NOT persist (best)
IFS=',' read -ra csv_fields <<< "a,b,c"
IFS=':'  read -ra path_parts <<< "$PATH"

# Scope IFS to a function with `local`
join_words() {
  local IFS=' '        # ensures "$*" joins with spaces regardless of caller's IFS
  printf '%s\n' "$*"
}

# If you must change IFS in the current shell, save and restore it
old_IFS="$IFS"
IFS=':'
read -ra parts <<< "$PATH"
IFS="$old_IFS"
```

##### Trimming whitespace

```bash
# Trim leading and trailing whitespace using read trick (no subprocess)
trim() {
  local var="$*"
  read -r var <<< "$var"   # read strips leading/trailing IFS chars
  printf '%s' "$var"
}
result=$(trim "  hello world  ")

# Pure parameter expansion — trim spaces (Bash, requires extglob)
shopt -s extglob
var="  hello  "
var="${var##*( )}"   # remove leading spaces
var="${var%%*( )}"   # remove trailing spaces

# Trim any whitespace class (tabs, newlines too)
var="${var##+([[:space:]])}"
var="${var%%+([[:space:]])}"

# Trimming with sed (works everywhere, spawns subprocess)
trimmed=$(sed 's/^[[:space:]]*//; s/[[:space:]]*$//' <<< "$var")
```

##### Handling filenames with spaces

```bash
# NEVER parse ls — filenames can contain spaces, newlines, special chars
# BAD
for f in $(ls *.txt); do process "$f"; done  # breaks on spaces

# GOOD — use globs
for f in *.txt; do
  [[ -e "$f" ]] || continue   # handle empty glob (nullglob not set)
  process "$f"
done

# Or use find with null-delimiter
while IFS= read -r -d '' file; do
  process "$file"
done < <(find . -name "*.txt" -print0)
```

##### Glob options and result order

```bash
shopt -s nullglob             # unmatched glob expands to nothing (default: the literal pattern)
files=(*.txt)                 # with nullglob: empty array when nothing matches
shopt -s dotglob              # include dot-files (still never . or ..)
shopt -s globstar             # ** recurses into directories (4.0+)
shopt -s failglob             # an unmatched glob is an error instead
```

- Unquoted `arr[0]` is a glob: with `nullglob` set, `unset arr[0]` expands to
  plain `unset` and does nothing; with a file named `arr0` in the current
  directory it unsets the variable `arr0`. Always quote: `unset 'arr[0]'`.
- **5.2+:** `globskipdots` is on by default, so `.*` no longer matches `.`
  and `..`; on older versions `rm -rf .*` style code also hits the parent
  directory (GNU `rm` refuses `.`/`..`, other tools may not).

**New in 5.3 — `GLOBSORT`.** Pathname expansion results are sorted by name
(collation order of the locale) unless `GLOBSORT` names another key:
`name`, `numeric` (all-digit names by value, others after them by name),
`size`, `blocks`, `mtime`, `atime`, `ctime`, or `nosort` (directory order); a
leading `-` reverses the order (a leading `+` is ignored; `-` alone means
reverse name order).

```bash
GLOBSORT=-mtime               # newest first (5.3+)
logs=(/var/log/myapp/*.log)
newest=${logs[0]}
unset GLOBSORT                # restore name order for the rest of the script
```

- On 5.2 and older `GLOBSORT` is an ordinary variable: the code still runs,
  but silently returns name order — guard with a version check or fall back
  to GNU `find … -printf '%T@ %p\0' | sort -z -rn` (not on macOS/BSD find).
- An unknown value (including the spelling `none`) is ignored without an
  error and name order is used; only `nosort` disables sorting.
- `GLOBSORT` is global state: set it in a function with `local GLOBSORT=…`
  so other globs in the script keep the expected order.
- Equal keys (same size/mtime) fall back to name order — reversed as well
  under a `-` prefix.

***

#### 8. Functions

##### Declaration syntax

```bash
# POSIX-compatible syntax — preferred
my_function() {
  local param1="$1"
  local param2="${2:-default}"
  # ...
}

# Bash-specific syntax — avoid, not POSIX
function my_function {
  :  # ...
}
```

##### `local` and scoping

```bash
# Always declare local variables — prevents global pollution
process_file() {
  local file="$1"
  local -i line_count=0
  local -a results=()

  while IFS= read -r line; do
    results+=("$line")
    (( line_count++ ))
  done < "$file"

  echo "$line_count"
}

# Return values — use stdout, not return (return only supports 0-255)
get_timestamp() {
  date '+%Y-%m-%dT%H:%M:%S%z'
}
ts=$(get_timestamp)

# Return multiple values via nameref (Bash 4.3+)
get_stats() {
  local -n _out_count="$1"
  local -n _out_sum="$2"
  _out_count=42
  _out_sum=1337
}
declare -i total_count total_sum
get_stats total_count total_sum

# Scope shell options to a function (Bash 4.4+): restored on return
with_strict_globs() {
  local -                     # saves options changed with set (-f, -u, -x, -o pipefail …)
  set -f                      # noglob only inside this function
  local IFS=,                 # IFS scoped the same way
  read -ra parts <<< "$1"
  printf '%s\n' "${parts[@]}"
}
```

`local -` restores only options changed with `set` (including long-only ones
such as `set -o pipefail`); `shopt` options (extglob,
nullglob, …) are **not** restored — save them with `shopt -p extglob` and
`eval` the saved line on return. Bash uses dynamic scoping: a `local` variable is
visible to every function the declaring function calls.

##### Error handling in functions

```bash
# Functions inherit set -e and set -u from the calling shell — but a function
# called as `f || …`, `if f`, `f && …` or `! f` runs with set -e ignored (see §2),
# so check every step explicitly with || return
deploy() {
  local env="$1"
  cd "/srv/${env}" || { log_error "Cannot cd to /srv/${env}"; return 1; }
  git pull --ff-only || { log_error "git pull failed"; return 1; }
  systemctl restart myapp || return 1
}

# Call with error checking
deploy production || { echo "Deployment failed" >&2; exit 1; }
```

##### Recursive functions

```bash
# Always test for depth / termination condition
walk_tree() {
  local dir="${1:-.}"
  local depth="${2:-0}"
  ((depth > 10)) && { echo "Max depth exceeded" >&2; return 1; }
  for item in "$dir"/*/; do
    [[ -d "$item" ]] || continue
    printf '%*s%s\n' "$((depth*2))" '' "${item%/}"
    walk_tree "$item" "$(( depth + 1 ))"
  done
}
```

##### Option parsing: `getopts`

`getopts` is the POSIX builtin for parsing **short** options. It is a valid,
portable alternative to a manual `while`/`case` loop (see the Appendix template)
and handles bundling (`-xv`) and option arguments (`-o file`) for you.

```bash
usage() { echo "usage: $0 [-v] [-o file] [-j n] args..." >&2; exit "${1:-0}"; }

verbose=false output="" jobs=1
# Leading ':' = silent error mode: getopts reports errors via $OPTARG, not stderr.
# A trailing ':' after a letter means that option takes an argument.
while getopts ':vo:j:h' opt; do
  case "$opt" in
    v) verbose=true ;;
    o) output="$OPTARG" ;;
    j) jobs="$OPTARG" ;;
    h) usage 0 ;;
    :)  echo "option -$OPTARG requires an argument" >&2; usage 2 ;;
    \?) echo "unknown option: -$OPTARG" >&2; usage 2 ;;
  esac
done
shift "$((OPTIND - 1))"   # drop parsed options; "$@" is now the positional args
```

**Limitations — use a manual `while`/`case` loop when you need any of these:**

- **No GNU long options** (`--verbose`, `--output=file`). `getopts` handles only
  single-character options.
- Stops at the first non-option argument (no interspersing options and operands).
- `OPTIND` is global; reset `OPTIND=1` before re-parsing in the same shell/function.
- No built-in `--` handling beyond terminating option parsing.

For long options, keep the manual `while [[ $# -gt 0 ]]; do case "$1" in …` pattern
shown in the Appendix.

***

#### 9. Traps & Signal Handling

##### Canonical production trap pattern

```bash
#!/usr/bin/env bash
set -euo pipefail

# ── Cleanup ───────────────────────────────────────────────────────────────────
TMPDIR_WORK=""

cleanup() {
  local exit_code=$?
  # Silence errors in cleanup — must not re-trigger ERR trap
  set +e
  [[ -n "$TMPDIR_WORK" ]] && rm -rf "$TMPDIR_WORK"
  # Restore terminal if script messed with it
  tput cnorm 2>/dev/null || true
  exit "$exit_code"
}

# EXIT is sufficient for most cleanup — runs on normal exit, `exit`, set -e aborts,
# and when the script is killed by an untrapped HUP/INT/TERM (not on SIGKILL)
trap cleanup EXIT

# For graceful shutdown on SIGINT/SIGTERM (e.g., kill background jobs)
handle_signal() {
  echo "Signal received, shutting down..." >&2
  # kill background jobs if any
  jobs -p | xargs -r kill 2>/dev/null || true
  exit "$1"
}
trap 'handle_signal 130' INT    # 128 + 2 (SIGINT)
trap 'handle_signal 143' TERM   # 128 + 15 (SIGTERM)

# ERR trap — fires on any command returning non-zero (requires set -E to propagate into functions)
set -E
on_error() {
  local exit_code=$?
  local line_no="${BASH_LINENO[0]}"   # line of the failing command
  local command="${BASH_COMMAND}"
  echo "ERROR: command '${command}' failed with exit code ${exit_code} at line ${line_no}" >&2
}
trap on_error ERR

TMPDIR_WORK=$(mktemp -d)
```

**Key rules:**

- `trap ... EXIT` runs on **all** exits including error exits — ideal for cleanup.
- Listing `trap ... INT TERM EXIT` causes cleanup to run **twice** when INT/TERM received. Use `EXIT` alone for cleanup; use separate INT/TERM traps only for graceful shutdown signaling.
- `trap ... ERR` requires `set -E` to fire inside functions.
- In cleanup functions: use `set +e` and `|| true` to prevent recursive errors.
- Initialise the cleanup variable to an empty value before installing the trap (as above) or create the temp dir first, and never name the variable `TMPDIR`: `mktemp` consults `$TMPDIR`, so a trap that fires before the assignment can `rm -rf` the inherited real temp directory.
- There is one action per signal: a later `trap … EXIT` (for example in a library or a lock helper) silently **replaces** the earlier cleanup. Keep a single `cleanup` function and register resources with it.
- A trap set inside a function is global; `trap - SIG` resets it. Subshells (`( … )`, `$( … )`, pipeline members) reset caught signals and the `EXIT` trap to their defaults; only `ERR` (with `set -E`) and `DEBUG`/`RETURN` (with `set -T`) are inherited.
- To exit with the conventional status after a handled signal, reset the trap and re-send the signal (`trap - INT; kill -INT "$$"`), so a parent shell sees "killed by SIGINT" rather than an ordinary exit 130.

##### Which signal fired? `BASH_TRAPSIG` and `trap -P` (New in 5.3)

```bash
on_signal() {
  local sig=${BASH_TRAPSIG:-15}          # 5.3+: numeric signal of the running trap
  echo "caught signal $sig, shutting down" >&2
  exit $(( 128 + sig ))
}
trap on_signal INT TERM HUP
trap -P TERM                             # 5.3+: prints only the action: on_signal
```

- `BASH_TRAPSIG` is set only while a trap action runs (it is `0` inside an
  `EXIT` trap). On 5.2 and older it is an ordinary, normally unset variable, so
  the fallback above reports 15 for every signal; code that must run there
  should install one handler per signal instead
  (`trap 'on_signal 2' INT; trap 'on_signal 15' TERM`), which works on 3.2+.
- `trap -p SIG` (all versions) prints a reusable `trap -- '…' SIG` command;
  `trap -P SIG` (5.3+) prints just the action string.
- **Changed in 5.3:** `exit` without an argument at the top level of a trap
  action now exits with the status from *before* the trap ran (POSIX
  interpretation 1602): `trap 'false; exit' USR1` exits 1 on 5.2 (status of
  `false`) but with the value `$?` had when the trap started on 5.3 (for
  example 0 after a successful `kill -USR1 "$BASHPID"`). Pass an explicit
  status (`exit "$rc"`) in every trap that exits.

##### Lock files

```bash
LOCKDIR="/run/lock/myapp.lock"     # mkdir is atomic: exactly one caller succeeds
LOCK_HELD=false

acquire_lock() {
  if ! mkdir "$LOCKDIR" 2>/dev/null; then
    echo "Script already running (lock: $LOCKDIR)" >&2
    exit 1
  fi
  LOCK_HELD=true    # release it from the single EXIT cleanup, e.g.:
}                   # [[ $LOCK_HELD == true ]] && rmdir "$LOCKDIR"
# A stale directory survives SIGKILL/power loss; flock(1) on Linux (§13) does not
# have that problem because the kernel drops the lock when the process dies.
```

***

#### 10. Exit Codes

Exit codes are 8-bit unsigned integers: **0–255**.

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General / unspecified error |
| `2` | Misuse of shell builtin / bad arguments |
| `64–78` | `sysexits.h` conventions (rarely enforced) |
| `126` | Command found but not executable |
| `127` | Command not found |
| `128+N` | Fatal signal N (e.g., 137 = SIGKILL = 128+9, 143 = SIGTERM) |
| `130` | Terminated by Ctrl-C (SIGINT = 128+2) |

Values are taken modulo 256: `exit 256` exits 0 and `exit -1` exits 255. A
non-numeric argument (`exit foo`) prints `numeric argument required` and exits
**2** on 4.4–5.3 (255 on 3.2) — the "128 = invalid exit argument" row found in
many tables does not describe Bash. Never `exit "$count"` with a count that can
reach 256.

```bash
# Exit with meaningful codes
readonly EX_OK=0
readonly EX_USAGE=2
readonly EX_NOINPUT=66
readonly EX_UNAVAILABLE=69
readonly EX_SOFTWARE=70

usage() {
  printf 'Usage: %s [options] <file>\n' "$0" >&2
  exit "$EX_USAGE"
}

[[ $# -eq 0 ]] && usage
[[ -f "$1" ]] || { echo "File not found: $1" >&2; exit "$EX_NOINPUT"; }

# Check exit codes explicitly when set -e is off or insufficient.
# NOTE: inside `if ! cmd; then`, $? is the negated test's status (0), NOT cmd's.
if ! command_that_might_fail; then
  echo "command_that_might_fail failed" >&2   # do NOT read $? here — it is 0
  exit 1
fi

# When you need the actual exit code, capture it without `!`:
command_that_might_fail || { rc=$?; echo "Failed with: $rc" >&2; exit 1; }

# Propagate exit code from subshell
run_in_subshell() (
  set -e
  do_something
)
run_in_subshell
exit_code=$?
```

***

#### 11. Logging

##### Structured logging to stderr

```bash
#!/usr/bin/env bash
# Logging helpers — all to stderr to keep stdout clean for data
readonly LOG_LEVEL="${LOG_LEVEL:-INFO}"
declare -rA _LOG_LEVELS=([DEBUG]=0 [INFO]=1 [WARN]=2 [ERROR]=3)  # -A is required; without it the keys collapse to index 0

log() {
  local level="$1"; shift
  local IFS=' '        # scope IFS: keep "$*" space-joined regardless of caller's IFS
  local msg="$*"
  local ts
  printf -v ts '%(%Y-%m-%dT%H:%M:%S%z)T' -1   # Bash 4.2+: builtin, no date fork

  # Numeric comparison for log level filtering
  local current_level="${_LOG_LEVELS[${LOG_LEVEL:-INFO}]:-1}"  # default guards empty subscript
  local msg_level="${_LOG_LEVELS[${level:-INFO}]:-1}"          # empty assoc key = "bad array subscript"
  (( msg_level < current_level )) && return 0

  printf '[%s] [%s] %s\n' "$ts" "$level" "$msg" >&2
}

log_debug() { log DEBUG "$@"; }
log_info()  { log INFO  "$@"; }
log_warn()  { log WARN  "$@"; }
log_error() { log ERROR "$@"; }

# Usage
log_info "Starting deployment to ${ENV:-unknown}"
log_error "Failed to connect to database: $DB_HOST"
```

##### Redirecting entire script output to a log file

```bash
# Redirect all stdout+stderr to log while keeping console output (Bash 3.2+)
LOG_FILE="/var/log/myapp/deploy-$(date +%Y%m%d-%H%M%S).log"
mkdir -p "$(dirname "$LOG_FILE")"

# Save originals
exec 3>&1 4>&2

# Redirect through tee — live console + log file
exec 1> >(tee -a "$LOG_FILE") 2> >(tee -a "$LOG_FILE" >&2)

# Restore on exit (put this in your single cleanup function — a second
# `trap … EXIT` would replace the existing one)
trap 'exec 1>&3 2>&4; exec 3>&- 4>&-' EXIT
```

The `tee` processes run asynchronously: the last lines can reach the log after
the script has exited, and stdout/stderr ordering between the two `tee`s is not
preserved.

***

#### 12. Input / Output & Redirection

##### File descriptor overview

| FD | Name | Default |
|----|------|---------|
| `0` | stdin | keyboard |
| `1` | stdout | terminal |
| `2` | stderr | terminal |

```bash
# Redirect stdout to file (overwrite)
command > output.txt

# Redirect stdout to file (append)
command >> output.txt

# Redirect stderr to file
command 2> errors.txt

# Redirect both stdout and stderr to same file
command > output.txt 2>&1     # POSIX-compatible (portable, preferred)
command &> output.txt         # Bash shorthand (predates 4.0; non-POSIX)
command &>> output.txt        # Bash append shorthand (Bash 4.0+)

# Discard output
command > /dev/null 2>&1
command &>/dev/null           # Bash shorthand (non-POSIX)

# stderr to stdout (useful in pipes)
command 2>&1 | grep ERROR

# Redirect from file (stdin)
command < input.txt

# noclobber — prevents accidental overwrite with >
set -o noclobber
command > existing.txt        # fails if file exists
command >| existing.txt       # force overwrite even with noclobber
```

##### Order of redirections matters

```bash
# WRONG — redirects stderr to original stdout (terminal), not the file
command 2>&1 > file.txt

# RIGHT — redirect stdout first, then dup stderr to the new stdout (file)
command > file.txt 2>&1
```

##### Atomic file writes & safe moves

Write to a temp file in the **same directory** as the target, then `mv` it into
place. A `mv` within one filesystem is an atomic rename; across filesystems it is
copy+unlink and **not** atomic — so the temp file must live beside the target.
`mktemp` creates mode 0600, so set the intended mode before the move.

```bash
atomic_write() {
  local -r target="$1"
  local tmpfile
  tmpfile=$(mktemp -- "$(dirname -- "$target")/.tmp.XXXXXX") || return 1
  cat > "$tmpfile"
  chmod 644 "$tmpfile"          # mktemp is 0600; set the intended mode first
  mv -f "$tmpfile" "$target"    # atomic rename within the same filesystem
}

safe_move() {
  local -r src="$1" dst="$2"
  [[ -e "$src" ]]  || { echo "ERROR: source missing: $src" >&2; return 1; }
  [[ ! -e "$dst" ]] || { echo "ERROR: destination exists: $dst" >&2; return 1; }
  mv -- "$src" "$dst"
}
```

***

#### 13. `exec` & File Descriptors

`exec` replaces the current shell process or globally redirects file descriptors.

```bash
# Replace current process (no fork) — use for wrapping
exec /usr/bin/myprogram "$@"

# Globally redirect all stdout in current script
exec > /var/log/myapp.log 2>&1

# Open custom file descriptors
exec 3< input.txt             # fd 3 for reading
exec 4> output.txt            # fd 4 for writing
exec 5>> append.txt           # fd 5 for appending
exec 9>/var/run/myapp.lock    # fd 9 for lock file

# Use custom fd
read -r line <&3              # read from fd 3
echo "data" >&4               # write to fd 4

# Close file descriptor
exec 3>&-
exec 4<&-

# Let Bash pick a free fd (4.1+) — avoids clobbering fds the caller uses
exec {lock_fd}>"/run/lock/myapp.lock"
flock -n "$lock_fd" || { echo "already running" >&2; exit 1; }   # flock(1): util-linux, not on macOS
exec {lock_fd}>&-             # close it again by name

# Production logging pattern with exec
setup_logging() {
  local log_file="$1"
  exec 3>&1 4>&2              # save originals
  exec 1> >(tee -a "$log_file") 2> >(tee -a "$log_file" >&2)
}

teardown_logging() {
  exec 1>&3 2>&4              # restore
  exec 3>&- 4>&-              # close saved fds
}
```

`{var}>file` on a command other than `exec` leaves the descriptor **open** after
the command (a descriptor leak in loops) unless `shopt -s varredir_close` (5.2+)
is set; close it explicitly with `{var}>&-`. `{var}>&-` with a non-numeric `$var`
silently closes **stdin** on 5.2 and older; since 5.3 it fails with
`ambiguous redirect` (status 1) and leaves stdin open.

***

#### 14. Piping & Process Substitution

##### Pipe pitfalls with `set -e` and `pipefail`

```bash
# Without pipefail — only last command's exit code is checked
false | true   # exits 0 without pipefail!

# With set -o pipefail
set -o pipefail
false | true   # now exits 1

# Get individual pipe exit codes
cmd1 | cmd2 | cmd3
echo "${PIPESTATUS[@]}"   # e.g. "0 1 0" — Bash-specific, not POSIX
```

##### Process substitution — avoid subshell variable scoping issues

```bash
# BAD — variable set inside pipe is lost (runs in subshell)
count=0
some_command | while read -r line; do
  (( count++ ))
done
echo "$count"   # still 0!

# GOOD — process substitution keeps the loop in current shell
count=0
while IFS= read -r line; do
  (( count++ ))
done < <(some_command)
echo "$count"   # correct
```

##### Parallel execution

```bash
# Run commands in parallel, collect exit codes
pids=()
for target in "${TARGETS[@]}"; do
  deploy "$target" &
  pids+=($!)
done

# Wait for all and check
for pid in "${pids[@]}"; do
  wait "$pid" || { echo "Job $pid failed" >&2; exit 1; }
done
```

Bounded parallelism and "first failure wins" (Bash **5.1+** for `wait -p`;
`wait -n` alone is 4.3+):

```bash
max_jobs=4
for target in "${TARGETS[@]}"; do
  while (( $(jobs -pr | wc -l) >= max_jobs )); do
    wait -n -p done_pid || { echo "job $done_pid failed" >&2; exit 1; }
  done
  deploy "$target" &
done
rc=0
while true; do
  status=0
  wait -n -p done_pid || status=$?
  [[ -n ${done_pid-} ]] || break                 # no children left: wait unsets done_pid
  (( status == 0 )) || { echo "job $done_pid failed ($status)" >&2; rc=1; }
done
exit "$rc"
```

- `wait` without arguments always returns 0 — it does **not** report failed
  jobs; wait for each PID (or use `wait -n` in a loop) to collect statuses.
- `wait -n` returns 127 when there are no unwaited-for children.
  **5.3:** `wait -n` can also return terminated process substitutions and jobs
  already reported to the user; in POSIX mode it removes the reaped job.
- `wait -f PID` (5.0+) waits until the job terminates even when job control is
  enabled, instead of returning on a stop.
- Coprocesses (`coproc NAME { …; }`, 4.0+): before 5.3 a second simultaneous
  coproc prints `warning: execute_coproc: coproc [PID:NAME] still exists`;
  multiple coprocs are supported by default since 5.3. Close coproc fds you no
  longer need (`exec {NAME[1]}>&-`) or the child never sees EOF.

***

#### 15. Here-Docs & Here-Strings

##### Here-doc

```bash
# Standard here-doc — interpolates variables
cat <<EOF
Server: ${SERVER}
Port:   ${PORT}
EOF

# Indented here-doc (<<- strips leading TABS only, not spaces — POSIX, all Bash versions)
if true; then
	cat <<-EOF
		This line is indented with tabs.
		Server: ${SERVER}
	EOF
fi

# No interpolation — quote the delimiter
cat <<'EOF'
This is literal: $HOME ${PATH} $(command)
EOF

# Pass to command directly — keep secrets out of argv (visible in ps / /proc)
# BAD:  mysql -p"$DB_PASS" …      GOOD: credentials from an option file or env
mysql --defaults-extra-file="$creds_file" "$DB_NAME" <<'EOF'
SELECT COUNT(*) FROM users WHERE active = 1;
EOF

# Write to file
cat > /etc/myapp/config.yaml <<EOF
host: ${DB_HOST}
port: ${DB_PORT}
EOF

# Here-doc in function
generate_config() {
  local env="$1"
  cat <<EOF
environment: ${env}
debug: false
log_level: warn
EOF
}
```

**Pitfalls:**
- `<<-` strips **tabs only** — mixing spaces and tabs causes silent misalignment.
- Variables and command substitutions are expanded unless delimiter is quoted.
- Trailing newline is always included.
- Expanded data is **not** escaped: `cat <<EOF` around `${user_input}` that
  produces YAML/SQL/JSON is an injection point — generate structured formats
  with a real tool (`jq -n --arg`, parameterised SQL).
- Bash 5.1+ feeds small here-docs/here-strings through a **pipe** instead of a
  temporary file (`BASH_COMPAT=50` restores temp files on 5.2+; 5.1 itself
  ignores that setting); code that seeks on stdin or checks
  `[[ -f /dev/stdin ]]` behaves differently.

##### Here-string (Bash 2.05b+)

```bash
# Feed string to stdin of a command — no temp file
grep "pattern" <<< "$variable"

# Read parsing without temp file
read -r first second <<< "hello world"
echo "$first $second"   # hello world

# Trim whitespace via read
trimmed_val="  spaces  "
read -r trimmed_val <<< "$trimmed_val"
```

***

#### 16. `read` Builtin

```bash
# Basic read — always use -r to prevent backslash interpretation
read -r line

# Read with prompt
read -r -p "Enter value: " user_input

# Read password silently
read -r -s -p "Password: " password
echo  # print newline after silent read

# Read with timeout (integer seconds on 3.2; fractional values such as -t 0.5 need 4.0+)
# Status > 128 means the timeout expired
if ! read -r -t 10 -p "Proceed? [y/N] " answer; then
  echo "Timeout — defaulting to No"
  answer="n"
fi

# Read into array (splits on IFS)
IFS=',' read -r -a csv_fields <<< "a,b,c"

# Read all lines into array (Bash 4.0+)
mapfile -t lines < file.txt     # strips trailing newlines
readarray -t lines < file.txt   # identical to mapfile

# mapfile -d for null-delimited (Bash 4.4+)
mapfile -d '' -t files < <(find . -name "*.go" -print0)

# Safe line-by-line file reading
while IFS= read -r line; do
  # IFS= prevents stripping leading/trailing whitespace
  # -r prevents backslash escaping
  printf '%s\n' "$line"
done < "$file"

# Read from file descriptor
exec 3< file.txt
while IFS= read -r -u3 line; do
  process "$line"
done
exec 3<&-

# Read with delimiter (read up to delimiter, not newline)
IFS= read -r -d $'\0' content < <(printf '%s\0' "$data")
```

**ShellCheck warning SC2162:** Always use `read -r` to avoid mangling backslashes.

- `read` returns non-zero at end of file even when it read a final line that
  lacks a trailing newline — use `while IFS= read -r line || [[ -n $line ]]`
  for files that may not end in `\n`.
- `read -e` (readline editing) and the **5.3** `read -E` (readline plus Bash's
  default and programmable completion) only matter for interactive prompts;
  `-E` is an "invalid option" error on 5.2 and older.
- `read -p` writes the prompt only when input comes from a terminal.

***

#### 17. Environment Handling

```bash
# Check if variable is set and non-empty
[[ -n "${MY_VAR:-}" ]] || { echo "MY_VAR required" >&2; exit 1; }

# Require environment variables with :?
: "${DATABASE_URL:?DATABASE_URL must be set}"
: "${API_KEY:?API_KEY must be set}"

# Export only what's needed — don't export everything
export PATH="/usr/local/bin:$PATH"

# Unexport a variable (make local-only)
export -n VAR_NAME

# Sanitize environment in production — use env -i for clean slate
env -i HOME="$HOME" PATH="$PATH" bash ./clean_env_script.sh

# Sourcing a config file EXECUTES it — only source files owned by a trusted user
# and not writable by others; parse untrusted key=value files with read instead
config_file="${XDG_CONFIG_HOME:-$HOME/.config}/myapp/config"
# Use if, not `[[ -f … ]] && source …`: as the last command of a function or
# script, the && form returns 1 when the file is absent
if [[ -f "$config_file" ]]; then
  # shellcheck source=/dev/null
  source "$config_file"
fi

# Show all exports
declare -x       # or: export -p

# Subshell environment isolation
(
  export SOME_VAR="subshell_only"
  do_something
)  # SOME_VAR gone here

# Common environment variables to NOT override
# Never: PATH= (use PATH="new:$PATH"), HOME=, IFS=, BASH=, SHELL=
```

**`PATH` hygiene.** An empty `PATH`, an empty element (`::`, a leading or
trailing `:`) or a `.` element makes Bash search the **current directory** —
`PATH="$extra:$PATH"` with an empty `$extra` silently adds the cwd. Build
`PATH` only from non-empty absolute directories.

**Locating library files.** Resolve sourced files relative to the script, not
the caller's working directory:

```bash
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P) || exit 1
source "$script_dir/lib/common.sh"

# New in 5.3: search an explicit directory list instead of $PATH, with no
# current-directory fallback (an empty list element still means the cwd)
source -p "$script_dir/lib:/usr/local/share/myapp" common.sh
```

- `source file` without a slash searches `$PATH` first (and the current
  directory as a fallback in non-POSIX mode, `sourcepath` on by default) — always
  pass a path containing `/`, or use `source -p DIRS file` on **5.3+**
  (`-p` is an invalid option on 5.2 and older).
- **New in 5.3:** `shopt -s bash_source_fullpath` stores absolute pathnames
  in `BASH_SOURCE` (off by default, so `${BASH_SOURCE[0]}` is still whatever
  path the script was invoked or sourced with).

**`cd` checks.** Always `cd -- "$dir" || exit`. **New in 5.3:** `cd ""` is an
error (`cd: null directory`, status 1, also with `BASH_COMPAT=52`); 5.2 and
older treat it as a successful no-op, so `cd "$maybe_empty"` used to keep running
in the current directory — guard with `${dir:?}`.

***

#### 18. Conditionals & Tests

##### Double brackets `[[ ... ]]`

```bash
# Use [[ ]] in Bash scripts — safer, richer, no word splitting
# [ ] is POSIX — required for sh compatibility
# test is identical to [ ]

# File tests
[[ -f "$file" ]]    # is a regular file
[[ -d "$dir" ]]     # is a directory
[[ -e "$path" ]]    # exists (any type)
[[ -r "$file" ]]    # readable
[[ -w "$file" ]]    # writable
[[ -x "$file" ]]    # executable
[[ -s "$file" ]]    # exists and non-empty (size > 0)
[[ -L "$link" ]]    # is a symbolic link
[[ -z "$var" ]]     # string is empty
[[ -n "$var" ]]     # string is non-empty
[[ -v var ]]        # variable is set (Bash 4.2+, no $ prefix)
[[ -R var ]]        # variable is a nameref (Bash 4.3+)

# String comparisons (always quote right side in [[ to avoid pattern matching)
[[ "$a" == "$b" ]]           # equal
[[ "$a" != "$b" ]]           # not equal
[[ "$a" == "prefix"* ]]      # glob match (no quotes on pattern)
[[ "$a" =~ ^[0-9]+$ ]]       # regex match (no quotes on pattern)

# Arithmetic
(( count > 0 ))              # preferred for arithmetic tests
[[ "$count" -gt 0 ]]         # numeric; inside [[ ]] operands are arithmetic expressions (§5)
[[ "$count" -eq 0 ]]         # equal (numeric)

# Logical operators
[[ -f "$file" && -r "$file" ]]   # AND
[[ -z "$a" || -z "$b" ]]         # OR
[[ ! -d "$dir" ]]                # NOT

# Case statement (faster than chained if/elif for string matching)
case "$env" in
  prod|production)
    log_level="error"
    ;;
  staging|stage)
    log_level="warn"
    ;;
  dev|development|"")
    log_level="debug"
    ;;
  *)
    echo "Unknown environment: $env" >&2
    exit 2
    ;;
esac
```

##### `test` / `[` pitfalls and the 5.3 parsing change

- Do not combine tests with `-a`/`-o` inside `[ … ]`; the result depends on
  the number of arguments and on the operands' values (`!`, `(`, `=` as data).
  Use `[ "$a" = x ] && [ "$b" = y ]` or `[[ … && … ]]`.
- **Changed in 5.3:** with more than four arguments, `test`/`[` now scans a
  parenthesised sub-expression and, if it holds four or fewer arguments, parses
  it with the POSIX rules (as coreutils `test` does). An expression such as
  `[ \( "!" = "!" \) -a x ]` fails with a "`)` expected" error (status 2) on 5.2 and
  succeeds on 5.3; `BASH_COMPAT=52` restores the old algorithm. Code whose
  result changes is already ambiguous — rewrite it with `[[ ]]` or separate
  `[ ]` commands.
- `[[ $a < $b ]]` and `[[ $a > $b ]]` compare by the current locale's collation
  (since 4.1); **5.3 POSIX mode** makes `test`/`[` `<`/`>` locale-aware too. For
  byte order use `LC_ALL=C`. For numbers use `(( a < b ))` or `-lt`.
- `[ $var = x ]` with an empty or multi-word `$var` is a syntax error or a wrong
  result — quote every operand of `[`.

***

#### 19. `sed`

```bash
# Basic substitution — first occurrence per line
sed 's/old/new/' file.txt

# Global substitution (all occurrences per line)
sed 's/old/new/g' file.txt

# Case-insensitive (GNU sed only — not POSIX)
sed 's/old/new/gI' file.txt

# In-place editing
sed -i 's/old/new/g' file.txt          # GNU sed
sed -i '' 's/old/new/g' file.txt       # BSD/macOS sed (requires empty suffix)

# In-place with backup
sed -i.bak 's/old/new/g' file.txt

# Delete lines matching pattern
sed '/^#/d' file.txt                   # delete comment lines
sed '/^$/d' file.txt                   # delete blank lines
sed '/pattern/d' file.txt

# Print specific lines
sed -n '5p' file.txt                   # line 5
sed -n '5,10p' file.txt                # lines 5–10
sed -n '/start/,/end/p' file.txt       # between patterns

# Multiple expressions
sed -e 's/foo/bar/' -e 's/baz/qux/' file.txt

# Extended regex (use -E or -r)
sed -E 's/([0-9]{4})-([0-9]{2})-([0-9]{2})/\3\/\2\/\1/' dates.txt

# Insert line before/after match
sed '/pattern/i\new line before' file.txt
sed '/pattern/a\new line after'  file.txt

# Strip trailing whitespace
sed 's/[[:space:]]*$//' file.txt

# Strip ANSI color codes
sed 's/\x1b\[[0-9;]*m//g' file.txt

# Use with here-string (avoids temp file)
result=$(sed 's/old/new/g' <<< "$variable")

# Production: validate sed substitution worked
if ! sed -i 's/VERSION=.*/VERSION='"$NEW_VER"'/' config.txt; then
  echo "sed substitution failed" >&2
  exit 1
fi
```

***

#### 20. `awk`

```bash
# Print specific field (whitespace-delimited)
awk '{print $2}' file.txt
awk '{print $NF}' file.txt          # last field

# Custom field separator
awk -F':' '{print $1}' /etc/passwd  # first field, colon-delimited
awk -F',' '{print $3}' data.csv     # CSV third column

# Filter + print
awk '$3 > 100 {print $1, $3}' data.txt

# Pattern match
awk '/ERROR/ {print NR": "$0}' app.log   # print matching lines with line number

# BEGIN and END blocks
awk 'BEGIN {sum=0} {sum+=$2} END {printf "Total: %d\n", sum}' data.txt

# Associative arrays in awk
awk '{counts[$1]++} END {for (k in counts) print k, counts[k]}' log.txt

# Multiple field separators (ERE)
awk -F'[,;]' '{print $1}' data.txt

# In-place editing (GNU awk 4.1.0+ with -i inplace)
awk -i inplace '/old/{sub(/old/,"new")} {print}' file.txt

# printf in awk for formatted output
awk '{printf "%-20s %5d\n", $1, $2}' data.txt

# Named variables
awk -v threshold=50 '$2 > threshold {print $1}' data.txt

# Process CSV columns — WARNING: awk -F',' splits on EVERY comma and does NOT handle
# quoted commas; use a CSV-aware tool (csvkit, miller, python csv) whenever fields may be quoted
awk -F',' 'NR>1 {print $1, $3}' data.csv   # skip header, print cols 1 and 3

# Multiline records
awk 'BEGIN{RS=""; FS="\n"} {print $1}' paragraphs.txt

# Performance: awk is significantly faster than Python for stream processing
# Use awk for: field extraction, aggregation, format conversion
# Use Python for: complex logic, JSON, structured data
```

***

#### 21. `find`, `grep`, `rg`

##### `find`

```bash
# Find files by name
find /path -name "*.log"
find /path -iname "*.log"           # case-insensitive

# Find by type
find /path -type f                  # regular files
find /path -type d                  # directories
find /path -type l                  # symlinks

# Find by time
find /path -mtime -7                # modified within 7 days
find /path -newer reference.txt     # newer than reference file
find /path -mmin -60                # modified within 60 minutes

# Find by size
find /path -size +100M              # over 100 MB
find /path -size +1k -size -10M     # between 1K and 10M

# Safe execution — use + (batched) instead of \; (one per file)
find . -name "*.log" -exec rm {} +
find . -name "*.tmp" -exec gzip {} +

# Null-delimited output — safe for filenames with spaces
find . -name "*.txt" -print0 | xargs -0 grep "pattern"

# Process substitution with null delimiters (Bash 4.4+)
mapfile -d '' -t files < <(find . -name "*.go" -print0)

# Exclude directories
find . -path "./.git" -prune -o -name "*.go" -print

# Find and process in parallel
find . -name "*.log" -print0 | xargs -0 -P4 gzip
```

##### `grep`

```bash
# Basic
grep "pattern" file.txt
grep -r "pattern" directory/         # recursive
grep -rl "pattern" directory/        # recursive, filenames only

# Extended regex
grep -E "^[0-9]+" file.txt           # ERE
grep -P "(?<=prefix)\w+" file.txt    # PCRE (GNU grep only)

# Context
grep -A 3 "ERROR" log.txt            # 3 lines after
grep -B 2 "ERROR" log.txt            # 2 lines before
grep -C 2 "ERROR" log.txt            # 2 lines around

# Count / suppress
grep -c "pattern" file.txt           # count matches
grep -q "pattern" file.txt           # quiet — exit code only
grep -v "pattern" file.txt           # invert match

# Fixed string (faster, no regex interpretation)
grep -F "literal.string" file.txt

# Exit codes: 0=found, 1=not found, 2=error
if grep -q "error" "$logfile"; then
  echo "Errors found"
fi
```

##### `rg` (ripgrep) — preferred for large repos

```bash
# ripgrep is significantly faster than grep for large codebases
rg "pattern" .                       # recursive by default, respects .gitignore
rg -l "pattern" .                    # files only
rg -t go "pattern" .                 # filter by file type
rg --no-ignore "pattern" .           # ignore .gitignore/.rgignore
rg -l --null "pattern" . | xargs -0 ... # -l = NUL-terminated FILE LIST (without -l, rg NUL-terminates only the path and still prints match text; -0 is a short alias for --null)
rg -e "pattern1" -e "pattern2" .    # multiple patterns
rg --hidden "pattern" .             # include hidden files
rg --glob "*.go" "pattern" .        # glob filter
rg -c "pattern" .                   # count per file
rg --stats "pattern" .              # show statistics

# Fixed string search
rg -F "literal.string[with.brackets]" .

# Multiline
rg -U "start.*\nend" .              # multiline regex

# Replace (does not modify files — pipe or use --replace for output)
rg "old" --replace "new" file.txt
```

***

#### 22. `cat`, `ls` — and when NOT to use them

##### Useless Use of Cat (UUOC)

```bash
# BAD — spawns unnecessary subprocess
cat file.txt | grep "pattern"
cat file.txt | wc -l

# GOOD — direct input
grep "pattern" file.txt
wc -l < file.txt

# cat IS appropriate for concatenation
cat header.txt body.txt footer.txt > output.txt
cat a.log b.log | grep ERROR

# cat IS appropriate for here-docs to file
cat > config.yaml <<EOF
key: value
EOF
```

##### Never parse `ls`

```bash
# BAD — breaks on spaces, newlines, special chars in filenames
for f in $(ls *.txt); do process "$f"; done

# GOOD
for f in *.txt; do
  [[ -f "$f" ]] || continue
  process "$f"
done

# GOOD — with find for subdirectories
while IFS= read -r -d '' f; do
  process "$f"
done < <(find . -name "*.txt" -print0)
```

***

#### 23. Aliases

Aliases are for interactive shells. **Do not use aliases in scripts.**

```bash
# ~/.bashrc or ~/.bash_aliases — interactive use only
alias ll='ls -lahF --color=auto'
alias gs='git status'
alias k='kubectl'

# Expand aliases in scripts (bad practice — use functions instead)
shopt -s expand_aliases   # enables alias expansion in non-interactive scripts
                          # AVOID: creates hidden dependencies

# INSTEAD — use functions in scripts
k() { kubectl "$@"; }
deploy() { kubectl apply -f "$@"; }
```

***

#### 24. Performance

##### Avoid forks in loop bodies

Commands executed inside a loop body run once per iteration. Repeated
subprocesses can become expensive; prefer builtins, parameter expansion, or
batching when practical.

```bash
# SLOW — wc and cut run once for every file
for f in *.txt; do
  count=$(wc -l < "$f")     # fork for every file
  ext=$(echo "$f" | cut -d. -f2)  # two forks
done

# FAST — use builtins and parameter expansion
for f in *.txt; do
  # wc -l in bulk outside loop, or use mapfile + arithmetic
  ext="${f##*.}"            # no fork — parameter expansion
  base="${f%.*}"
done

# FAST — batch external commands
wc -l -- *.txt               # one wc invocation for all files
```

A command substitution used to construct the loop input runs once before the
loop starts, not once per iteration:

```bash
# FINE — seq runs once before this bounded loop
# seq is an external utility, so availability is independent of the Bash version
for _ in $(seq 1 30); do
  poll_service
done

# ALSO FINE — arithmetic for is a builtin in every supported Bash version (3.2–5.3)
for ((i = 1; i <= 30; i++)); do
  poll_service
done
```

##### String operations — builtins beat subprocesses

```bash
# SLOW
upper=$(echo "$str" | tr '[:lower:]' '[:upper:]')  # 2 forks

# FAST (Bash 4.0+)
upper="${str^^}"              # zero forks

# SLOW
len=$(echo -n "$str" | wc -c)

# FAST
len="${#str}"                 # zero forks
```

##### Capturing output without a fork

`$( … )` always forks. In hot loops prefer, in order of availability:

```bash
printf -v stamp '%(%F %T)T' -1          # 4.2+: no date(1) process
printf -v padded '%05d' "$n"            # 3.1+: formatted assignment, no subshell
out=${ some_function; }                 # 5.3+: function output captured in-process
compgen -V matches -W "$words" -- "$prefix"   # 5.3+: array result, no $(compgen …)
```

A `${ …; }` substitution of a function is markedly cheaper than `$( … )` in a
loop of thousands of iterations, but only where 5.3 is the guaranteed minimum.

##### Clocks and timing

| Need | Construct | Version |
|------|-----------|---------|
| Wall-clock seconds | `$EPOCHSECONDS` (or `printf '%(%s)T' -1` on 4.2+) | 5.0+ |
| Microsecond timestamp | `$EPOCHREALTIME` | 5.0+ |
| Elapsed time immune to clock changes | `$BASH_MONOSECONDS` (whole seconds; falls back to `EPOCHSECONDS` without a monotonic clock) | **5.3+** |
| Elapsed seconds since start | `$SECONDS` | all |
| Timing a command | `TIMEFORMAT='%3R'` + `time cmd`; `%6R` (microseconds) needs **5.3+**, 5.2 caps the precision at 3 digits | |

```bash
start=$BASH_MONOSECONDS                   # 5.3+
run_job
(( BASH_MONOSECONDS - start > 300 )) && echo "job took more than 5 minutes" >&2
```

Bash arithmetic is integer-only. For floating point use `awk`/`bc`, or on 5.3
the optional `fltexpr` loadable (`enable fltexpr` when
`BASH_LOADABLES_PATH` contains the distribution's loadables directory, then
`fltexpr 'x = 1.5 * 3'`). Loadables (`kv`, `strptime`, `fltexpr`, …) are
often not installed — never depend on them without checking `enable` succeeds.

##### `xargs` for parallelism

```bash
# Process files in parallel — 4 workers
find . -name "*.log" -print0 | xargs -0 -P4 gzip

# With replacement string
find . -name "*.png" -print0 | xargs -0 -P8 -I{} convert {} {}.webp
```

##### Profiling

```bash
# Time a script
time bash script.sh

# Trace with timestamps — identify slow lines (5.0+: EPOCHREALTIME, no fork per line)
PS4='+ ${EPOCHREALTIME} ${FUNCNAME:+${FUNCNAME}():}${LINENO}: '
# Before 5.0: PS4='+ $(date "+%s%N") …' (date %N is GNU-only; forks once per traced line)
set -x
# ... your code ...
set +x

# Profile with bash -x and external tool
bash -x script.sh 2>&1 | ts '[%Y-%m-%d %H:%M:%.S]' > trace.log
```

***

#### 25. POSIX Compliance & Portability

When writing scripts targeting `sh` (not Bash), avoid all of the following Bash-specific features:

| Feature | Bash | POSIX `sh` |
|---------|------|-----------|
| `[[ ]]` double brackets | ✅ | ❌ use `[ ]` |
| Arrays | ✅ | ❌ |
| Associative arrays | ✅ Bash 4+ | ❌ |
| `$(( ))` arithmetic | ✅ | ✅ |
| `let` / `(( ))` | ✅ | ❌ |
| `local` keyword | ✅ | ❌ (available in most sh) |
| `declare` | ✅ | ❌ |
| `source` | ✅ | ❌ use `.` |
| `&>` redirect | ✅ | ❌ use `> f 2>&1` |
| Here-string `<<<` | ✅ | ❌ |
| Process substitution `<()` | ✅ | ❌ |
| `mapfile`/`readarray` | ✅ Bash 4+ | ❌ |
| `=~` regex in `[[ ]]` | ✅ | ❌ |
| `${var,,}` case ops | ✅ Bash 4+ | ❌ |
| `declare -n` nameref | ✅ Bash 4.3+ | ❌ |
| `printf -v` | ✅ | ❌ |
| `PIPESTATUS` | ✅ | ❌ |
| `$'…'` ANSI-C quoting | ✅ | ✅ since POSIX.1-2024 (older `sh` implementations lack it) |
| `set -o pipefail` | ✅ | ✅ since POSIX.1-2024 (older `sh` implementations lack it) |
| `${ cmd; }` / `${\| cmd; }` | ✅ Bash 5.3+ | ❌ |
| `GLOBSORT`, `BASH_*` variables | ✅ (version-specific) | ❌ |

```bash
# POSIX-safe alternatives
# Instead of [[ ]], use:
[ -f "$file" ]
case "$str" in pattern*) echo match ;; esac

# Instead of arrays, use positional params or IFS-delimited strings:
set -- one two three
for arg; do echo "$arg"; done

# Instead of source, use:
. ./lib.sh

# Instead of &>, use:
command > output 2>&1

# Check for POSIX compliance with shellcheck
shellcheck --shell=sh script.sh
shellcheck --shell=bash script.sh   # Bash-specific checks
```

**Bash's POSIX mode** (`set -o posix`, `bash --posix`, or Bash invoked as `sh`)
changes Bash's own behaviour; it does not reject Bash extensions. Notable 5.3
changes in POSIX mode: function names no longer need to be valid identifiers
(5.2 exits with "not a valid identifier"), `kill` returns failure if **any**
PID/job argument is not found (in default mode `kill 123 456` still returns 0
when one target exists), `test`'s `<`/`>` use the locale, `wait -n` removes the
reaped job from the job table, and `set -e` exits "as if executing `exit` with
no arguments".

***

#### 26. What Not to Do — Anti-Patterns

##### The critical anti-patterns

```bash
# ❌ 1. Unquoted variable expansion — causes word splitting and globbing
rm $file                    # ✅ rm "$file"
cp $src $dst                # ✅ cp "$src" "$dst"
[[ $var == "x" ]]           # ✅ [[ "$var" == "x" ]] — though [[ protects against splitting

# ❌ 2. ls in loops
for f in $(ls)              # ✅ for f in *

# ❌ 3. Useless use of cat
cat file | grep             # ✅ grep file or grep < file

# ❌ 4. Backtick command substitution — opaque nesting
result=`cmd`                # ✅ result=$(cmd)
nested=`cmd1 \`cmd2\``      # ✅ nested=$(cmd1 $(cmd2))

# ❌ 5. Ignoring exit codes
mkdir /important/dir
cd /important/dir           # runs even if mkdir failed!
# ✅ cd /important/dir || exit 1
# ✅ Or use set -e

# ❌ 6. Unprotected rm -rf
rm -rf "$DIR/"*             # if $DIR is empty: rm -rf /*  !!!
# ✅ rm -rf "${DIR:?Directory not set}/"*

# ❌ 7. Comparing strings without quotes
[[ $a == $b ]]              # right side is treated as glob pattern!
# ✅ [[ "$a" == "$b" ]]    — quote the right side

# ❌ 8. eval with user input
eval "$user_provided"       # arbitrary code execution
# ✅ never eval user input; use arrays, declare, or printf -v instead

# ❌ 9. Parsing HTML/JSON/XML with grep/sed/awk
grep -o '"name":"[^"]*"' data.json   # breaks on complex structures
# ✅ Use jq for JSON, xmllint for XML, python/perl for complex parsing

# ❌ 10. Variable in printf format string
printf "$user_message"      # format injection
# ✅ printf '%s\n' "$user_message"

# ❌ 11. Changing IFS globally without restoring
IFS=','
for item in $list; do ...
# ✅ Scope with: IFS=',' read -ra parts <<< "$list"
# ✅ Or: old_IFS=$IFS; IFS=','; ...; IFS=$old_IFS

# ❌ 12. for loop over command output with spaces
for line in $(cat file)     # splits on spaces, not lines!
# ✅ while IFS= read -r line; do ... done < file
# ✅ mapfile -t lines < file; for line in "${lines[@]}"; do ...

# ❌ 13. Hardcoded /tmp paths
tmpfile=/tmp/myapp_data     # race condition, collision risk
# ✅ tmpfile=$(mktemp) || exit 1
# ✅ tmpdir=$(mktemp -d) || exit 1

# ❌ 14. Piping to read (subshell loses variables)
some_cmd | read -r var      # var lost after pipe
# ✅ read -r var < <(some_cmd)

# ❌ 15. [ ] with == instead of =
[ "$a" == "$b" ]            # == is not POSIX in single brackets
# ✅ [ "$a" = "$b" ]   — POSIX
# ✅ [[ "$a" == "$b" ]] — Bash

# ❌ 16. Using $? after multiple commands
cmd1
cmd2
if [ $? -eq 0 ]; then ...   # checks cmd2, not cmd1!
# ✅ if cmd2; then ...
# ✅ Or: cmd1; result1=$?; cmd2; result2=$?

# ❌ 17. which instead of command -v
if which jq; then ...       # which is an external tool, not always present, inconsistent output
# ✅ command -v jq >/dev/null || { echo "jq required" >&2; exit 1; }

# ❌ 18. Missing -- before user/variable operands that may begin with -
rm "$file"                  # if $file is "-rf", it is parsed as options
# ✅ rm -- "$file"          — end-of-options guard for -/user-controlled operands

# ❌ 19. Non-idempotent scripts (fail or corrupt on rerun)
mkdir "$dir"                # errors if the directory already exists
# ✅ mkdir -p "$dir"        — safe to rerun; design every step to be re-runnable

# ❌ 20. Unquoted data as a pattern-substitution replacement (5.2+: & = match)
out=${url//@HOST@/$host}    # a "&" in $host is replaced by "@HOST@" on 5.2+
# ✅ out=${url//@HOST@/"$host"}

# ❌ 21. Untrusted input in arithmetic or indexed-array subscripts
(( count += $user_value ))  # "a[$(rm -rf ~)0]" executes the command
# ✅ [[ $user_value =~ ^[0-9]+$ ]] && (( count += 10#$user_value ))

# ❌ 22. Secrets on the command line
curl -u "admin:$PASSWORD" "$url"        # visible in ps, /proc/PID/cmdline, xtrace
# ✅ pass secrets via stdin, a 0600 file, or an environment variable the tool reads
# ✅ disable xtrace around secret handling: { set +x; } 2>/dev/null

# ❌ 23. Newer-version syntax in a script that must run on an older Bash
x=${ date +%s; }            # bad substitution on 5.2 and older (run time, not parse time)
GLOBSORT=-mtime; f=(*)      # silently ignored on 5.2 and older
# ✅ guard with BASH_VERSINFO or use $(…) / find | sort when the minimum is < 5.3
```

***

#### 27. Version Feature Matrix

"≤ 3.2" means available in every version this guide covers. Boundaries for
4.0–4.3 come from the official NEWS file; everything else was executed on
3.2.57, 4.4.23, 5.0.18, 5.1.16, 5.2.37 and 5.3.20.

| Feature | Min. Bash Version | Notes |
|---------|------------------|-------|
| Indexed arrays, `$(( ))`, `[[ ]]`, `PIPESTATUS`, process substitution `<( )`, `extglob`, `nullglob`, here-string `<<<` | **≤ 3.2** | Here-strings date from 2.05b |
| `printf -v var` | **≤ 3.2** (3.1) | Assigning to `arr[i]` with `-v`: 4.1 |
| `+=` append (scalars and arrays) | **≤ 3.2** (3.1) | |
| `=~` / `BASH_REMATCH` | **≤ 3.2** (3.0) | 3.2 made quoted RHS parts literal |
| `declare -A` associative arrays | **4.0** | Not on macOS `/bin/bash` |
| `mapfile` / `readarray` | **4.0** | `-d delim`: 4.4 |
| `declare -l` / `-u`, `${var,,}` / `${var^^}` | **4.0** | |
| `&>>`, `\|&` | **4.0** | |
| `globstar` (`**`), `coproc`, `BASHPID` | **4.0** | Multiple coprocs without warning: 5.3 |
| Fractional `read -t 0.5` | **4.0** | 3.2: integer seconds only |
| `{fd}>file` fd allocation | **4.1** | `varredir_close`: 5.2 |
| `[[ -v var ]]`, `declare -g`, `printf '%(fmt)T'`, `lastpipe` | **4.2** | |
| Reading negative indices `${a[-1]}` | **4.2** | `${a[@]: -1}` works on 3.2 |
| `declare -n` / `local -n`, `[[ -R ]]` | **4.3** | |
| Assign/unset negative indices, `[[ -v a[i] ]]`, `wait -n` | **4.3** | |
| `inherit_errexit`, `local -`, `${v@Q}` `@E` `@P` `@A` `@a` | **4.4** | |
| Empty `"${arr[@]}"` allowed under `set -u` | **4.4** | 3.2–4.3: "unbound variable" |
| `EPOCHSECONDS`, `EPOCHREALTIME`, `BASH_ARGV0`, `wait -f` | **5.0** | |
| `localvar_inherit`, `assoc_expand_once` | **5.0** | `assoc_expand_once` deprecated in 5.3 |
| `SRANDOM`, `wait -p`, `declare -I` | **5.1** | |
| `${v@U}` `@u` `@L` `@K`, key/value list `declare -A m=(k v)` | **5.1** | |
| Here-docs/strings via pipe when small | **5.1** | `BASH_COMPAT=50` reverts (5.2+) |
| `${v@k}`, `printf %Q` | **5.2** | |
| `patsub_replacement` (default **on**), `globskipdots` (default **on**) | **5.2** | Behaviour change for existing code |
| `varredir_close`, `noexpand_translation` | **5.2** | |
| Double expansion of array subscripts in `(( ))`/`[[ -v ]]` prevented by default | **5.2** | `BASH_COMPAT=51` reverts |
| `${ cmd; }`, `${\| cmd; }` (no-fork command substitution) | **5.3** | Use 5.3.7+ with redirections |
| `GLOBSORT`, `BASH_TRAPSIG`, `BASH_MONOSECONDS` | **5.3** | Plain variables on older versions |
| `trap -P`, `source -p`/`. -p`, `compgen -V`, `read -E` | **5.3** | |
| `printf %#q`/`%#Q`, `%ls`/`%lc` | **5.3** | |
| `array_expand_once`, `bash_source_fullpath` | **5.3** | |
| `TIMEFORMAT` precision 4–6 | **5.3** | |
| `local BASH_REMATCH` honoured | **5.3** | |
| `cd ""` is an error | **5.3** | Not reverted by `BASH_COMPAT` |
| `test` POSIX parsing of short parenthesised sub-expressions | **5.3** | `BASH_COMPAT=52` reverts |

##### Checking version at runtime

```bash
# Check Bash version inside a script (BASH_VERSINFO exists in every version)
if (( BASH_VERSINFO[0] < 4 )); then
  echo "Requires Bash 4.0 or newer (found: $BASH_VERSION)" >&2
  exit 1
fi

# Generic "at least X.Y" helper — works on 3.2+
bash_at_least() {
  (( BASH_VERSINFO[0] > $1 || (BASH_VERSINFO[0] == $1 && BASH_VERSINFO[1] >= $2) ))
}

if bash_at_least 4 3; then
  declare -n ref="$target"     # nameref
else
  ref_val="${!target}"         # indirect expansion fallback (read-only)
fi

if bash_at_least 5 3; then
  GLOBSORT=-mtime              # newest first
fi
```

Prefer feature checks over version checks only where Bash offers them without
side effects (`shopt -q name 2>/dev/null` tells whether an option exists). Never
probe new **syntax** with `eval` in production code just to pick a code path —
choose one minimum version and enforce it once at the top.

***

#### 28. Removed / Changed in 5.3 — Migration Notes & `BASH_COMPAT`

Upgrading a script's runtime from 5.2 to 5.3 is low-risk, but review these:

| 5.2 behaviour | 5.3 behaviour | Action |
|---------------|---------------|--------|
| `cd ""` succeeds and stays in the current directory | Error `cd: null directory`, status 1 | Guard: `cd -- "${dir:?}"`; not reverted by `BASH_COMPAT` |
| `test`/`[` with > 4 args uses the historical algorithm for `( … )` | POSIX parsing of parenthesised sub-expressions with ≤ 4 args | Rewrite with `[[ ]]` or split `[ ]`; `BASH_COMPAT=52` reverts |
| `local BASH_REMATCH` ignored by `=~` | Honoured | Callers that relied on a function leaking `BASH_REMATCH` must change |
| Invalid regex in `=~` silent (status 2) | Error message on stderr, still status 2 | Validate regexes; do not rely on silence |
| `printf %#q` same as `%q` | `#` forces single quotes | Only affects code that already used `#` |
| `assoc_expand_once` | Deprecated synonym of `array_expand_once` | Use the new name on 5.3-only code; the old name still works |
| Second concurrent `coproc` prints a warning | Supported silently | — |
| `exit` without argument at a trap's top level exits with the last command's status inside the trap | Exits with the status from before the trap (POSIX interp. 1602) | Always pass an explicit status |
| POSIX mode: invalid function names fatal; `kill` succeeds if one target exists | Names allowed; `kill` fails if any target is missing | Check `kill` status per PID |
| Interactive: job notifications while sourcing | Deferred until sourcing finishes | `BASH_COMPAT=52` reverts |

**`BASH_COMPAT`.** Setting `BASH_COMPAT=52` (or `5.2`) makes 5.3 emulate 5.2 for
the items marked above — `test` parsing, `bind -p`/`-P` argument handling and
job notifications while sourcing — not for anything else. Levels accumulate:
`BASH_COMPAT=50` additionally restores, for example, the 5.0 `$RANDOM` sequence
and temporary files for here-documents; `BASH_COMPAT=51` restores 5.1 subscript
double expansion and `unset a[@]` semantics. In 5.3 the variable accepts 31–53
(older values are rejected with "compatibility value out of range"); the
`shopt compat31` … `compat44` options are deprecated and there are no
`compat50`+ options. Treat `BASH_COMPAT` as a temporary migration aid — it
re-enables known-unsafe behaviour (e.g. `BASH_COMPAT=51` brings back double
expansion of array subscripts) and should not appear in new code.

**Deprecated / removed across the covered range** (reviewer flags):

- Backtick command substitution (labelled "deprecated" in the 5.3 manual) and
  `$[ expr ]` (an undocumented, obsolete arithmetic syntax that 5.3 still
  accepts) — use `$( )` and `$(( ))`.
- `test`/`[` with `-a`, `-o` or parentheses — the 5.3 manual notes that POSIX
  deprecated them and that scripts should no longer use them; use `&&`/`||`
  between single-primary tests or `[[ ]]`.
- `egrep`/`fgrep` — GNU grep 3.8+ warns that they are obsolescent; use
  `grep -E`/`grep -F`.
- `shopt -s compat4x` options — use `BASH_COMPAT`.

***

#### 29. Production / Review Checklist

**Versions**

- [ ] Minimum Bash version is known (runtime guard, `Requires:` header, base image, macOS 3.2) and no construct exceeds it (§27).
- [ ] 5.3-only syntax (`${ cmd; }`, `${| cmd; }`, `source -p`, `compgen -V`, `trap -P`, `read -E`) appears only where 5.3 is guaranteed; `GLOBSORT`/`BASH_TRAPSIG`/`BASH_MONOSECONDS` have a fallback on older versions.
- [ ] Code upgraded to 5.2+ was checked for `&` in `${var/pat/rep}` replacements and for `.*` globs relying on `.`/`..`.
- [ ] Code upgraded to 5.3 was checked for `cd ""`, `test` with `( … )` and > 4 arguments, and `local BASH_REMATCH` assumptions.
- [ ] `BASH_COMPAT` is not set in new code.

**Correctness**

- [ ] `set -euo pipefail` (entry scripts only), plus `shopt -s inherit_errexit` on 4.4+; functions called in `if`/`||`/`&&` contexts check each step.
- [ ] `local x=$(cmd)` split into declaration + assignment (SC2155).
- [ ] Every expansion quoted; arrays expanded as `"${arr[@]}"`; `--` before operands that may start with `-`.
- [ ] `read -r` everywhere; `IFS=` when whitespace matters; loops over files use globs or `find -print0` + `read -d ''`.
- [ ] `[[ ]]` RHS quoted unless a pattern/regex is intended; regex stored in a variable.
- [ ] Exit statuses are 0–255; `wait` collects each background job's status.
- [ ] `cd` checked (`cd -- "$d" || exit`); `${var:?}` guards before `rm -rf`.

**Security**

- [ ] No `eval` of untrusted data; no untrusted data in arithmetic, indexed subscripts or `test -v "A[$k]"`.
- [ ] Data-derived pattern-substitution replacements quoted (`${s//x/"$r"}`).
- [ ] Temp files via `mktemp`, cleaned in an `EXIT` trap; no fixed `/tmp` names.
- [ ] Secrets never in argv, xtrace output or here-docs written to world-readable files.
- [ ] `PATH` contains no empty/relative elements; sourced files are trusted and addressed by absolute path or `source -p` (5.3).

**Robustness & tooling**

- [ ] `EXIT` trap for cleanup (not `INT TERM EXIT` with an `exit` inside); trap output to stderr.
- [ ] `{fd}>` redirections closed or `varredir_close` (5.2+) set.
- [ ] `shellcheck` (0.11.0 understands 5.3 funsubs) passes with `--shell=bash`; `bash -n` in CI; tests run on the minimum and the newest supported Bash (e.g. `bash:3.2`/`bash:5.3` images).

***

#### Appendix: Minimal Production Template

```bash
#!/usr/bin/env bash
# ============================================================
# script_name.sh — Brief description
# Usage: script_name.sh [OPTIONS] <arg>
# Author: Your Name
# Requires: Bash 4.2+ (printf %(…)T)
# ============================================================
set -euo pipefail
if (( BASH_VERSINFO[0] < 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] < 2) )); then
  echo "Bash 4.2+ required (found $BASH_VERSION)" >&2   # plain echo: works on 3.2
  exit 1
fi

# ── Constants ────────────────────────────────────────────────
readonly SCRIPT_NAME="${0##*/}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" || exit 1
readonly SCRIPT_DIR                # assign first, then mark readonly (SC2155)
readonly LOG_FILE="${LOG_FILE:-/tmp/${SCRIPT_NAME%.sh}.log}"
DRY_RUN="${DRY_RUN:-false}"   # not readonly: reassigned by --dry-run below

# ── Logging ──────────────────────────────────────────────────
log()  { local IFS=' '; printf '[%(%T)T] [%s] %s\n' -1 "$1" "${*:2}" >&2; }
info() { log INFO  "$@"; }
warn() { log WARN  "$@"; }
err()  { log ERROR "$@"; }
die()  { err "$@"; exit 1; }

# ── Cleanup ──────────────────────────────────────────────────
_TMPDIR=""
cleanup() {
  local code=$?
  set +e
  [[ -n "$_TMPDIR" ]] && rm -rf "$_TMPDIR"
  exit "$code"
}
trap cleanup EXIT

# ── Utilities ────────────────────────────────────────────────
require_cmd() {
  command -v "$1" &>/dev/null || die "Required command not found: $1"
}

run() {
  if [[ "$DRY_RUN" == "true" ]]; then
    local IFS=' '
    info "[dry-run] $*"
  else
    "$@"
  fi
}

# ── Argument parsing ─────────────────────────────────────────
usage() {
  cat <<EOF >&2
Usage: $SCRIPT_NAME [OPTIONS] <arg>

OPTIONS:
  -e, --env ENV    Target environment (default: dev)
  -n, --dry-run    Dry run mode
  -h, --help       Show this help

ENVIRONMENT:
  LOG_FILE         Log file path (default: /tmp/${SCRIPT_NAME%.sh}.log)
  DRY_RUN          Set to 'true' for dry-run mode
EOF
  exit "${1:-0}"
}

ENV="dev"
POSITIONAL=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -e|--env)      ENV="${2:?--env requires a value}"; shift 2 ;;
    -n|--dry-run)  DRY_RUN="true"; shift ;;
    -h|--help)     usage 0 ;;
    --)            shift; POSITIONAL+=("$@"); break ;;
    -*)            die "Unknown option: $1" ;;
    *)             POSITIONAL+=("$1"); shift ;;
  esac
done

[[ ${#POSITIONAL[@]} -gt 0 ]] || usage 2

# ── Prerequisites ────────────────────────────────────────────
require_cmd git
require_cmd kubectl

# ── Main ──────────────────────────────────────────────────────
main() {
  _TMPDIR=$(mktemp -d)
  info "Starting ${SCRIPT_NAME} (env=$ENV, dry_run=$DRY_RUN)"

  # ... your logic here ...
  info "Done"
}

main "$@"
```
