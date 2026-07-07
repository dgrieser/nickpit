### Bash Production Coding Guidelines

> **Target audience:** Senior engineers writing production-grade automation, DevOps tooling, and system scripts.
> **Scope:** GNU Bash 3.2 → 5.2, with version annotations on every feature that has constraints.

***

#### Table of Contents

1. [Shebang & Portability](#1-shebang--portability)
2. [Strict Mode — `set` Flags](#2-strict-mode--set-flags)
3. [Variables & Parameter Expansion](#3-variables--parameter-expansion)
4. [Arrays (Indexed)](#4-arrays-indexed)
5. [Associative Arrays (Maps)](#5-associative-arrays-maps)
6. [String Handling & Substitution](#6-string-handling--substitution)
7. [Whitespace Handling & IFS](#7-whitespace-handling--ifs)
8. [Functions](#8-functions)
9. [Traps & Signal Handling](#9-traps--signal-handling)
10. [Exit Codes](#10-exit-codes)
11. [Logging](#11-logging)
12. [Input / Output & Redirection](#12-input--output--redirection)
13. [`exec` & File Descriptors](#13-exec--file-descriptors)
14. [Piping & Process Substitution](#14-piping--process-substitution)
15. [Here-Docs & Here-Strings](#15-here-docs--here-strings)
16. [`read` Builtin](#16-read-builtin)
17. [Environment Handling](#17-environment-handling)
18. [Conditionals & Tests](#18-conditionals--tests)
19. [`sed`](#19-sed)
20. [`awk`](#20-awk)
21. [`find`, `grep`, `rg`](#21-find-grep-rg)
22. [`cat`, `ls` — and when NOT to use them](#22-cat-ls--and-when-not-to-use-them)
23. [Aliases](#23-aliases)
24. [Performance](#24-performance)
25. [POSIX Compliance & Portability](#25-posix-compliance--portability)
26. [What Not to Do — Anti-Patterns](#26-what-not-to-do--anti-patterns)
27. [Version Feature Matrix](#27-version-feature-matrix)

***

#### 1. Shebang & Portability

Always specify the interpreter. Prefer `#!/usr/bin/env bash` for portability across systems where Bash may not live at `/bin/bash`.[^1]

```bash
#!/usr/bin/env bash
# Good — finds bash in PATH; works on macOS, Linux, NixOS, etc.

#!/bin/bash
# Acceptable on known-Linux systems; fails on macOS default /bin/bash (3.2)

#!/bin/sh
# Use ONLY when you intend POSIX sh, not Bash — you lose arrays, [[ ]], etc.
```

**macOS warning:** macOS ships Bash 3.2 at `/bin/bash` (GPLv2). Associative arrays, `mapfile`, `readarray`, and `declare -n` are **not available**. If your scripts must run on macOS, either require Bash 5 from Homebrew or restrict to Bash 3.2-compatible constructs.

***

#### 2. Strict Mode — `set` Flags

Add to the very top of every non-interactive production script.[^2]

```bash
#!/usr/bin/env bash
set -euo pipefail
```

| Flag | Meaning | Notes |
|------|---------|-------|
| `-e` | Exit immediately on non-zero exit code | Does **not** trigger inside `if`, `&&`, `\|\|`, or `while` conditions |
| `-u` | Treat unset variables as errors | Catches typos like `$USRNAME` vs `$USERNAME`[^2] |
| `-o pipefail` | Pipeline fails if **any** command in it fails | Without this, `false \| true` returns 0[^2] |
| `-x` | Trace every command (debug) | Use in CI or dev, not production by default |
| `-E` | Inherit `ERR` trap in functions/subshells | Required for `trap ... ERR` to propagate into functions[^3] |

```bash
# Full production header
#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'       # safer default IFS — no accidental word splitting on spaces
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
```

***

#### 4. Arrays (Indexed)

Indexed arrays are available since Bash **2.0**. Always use `"${arr[@]}"` (not `*`) to iterate safely.[^4]

```bash
# Declaration
declare -a fruits=()

# Inline init
fruits=("apple" "banana" "cherry")

# Append
fruits+=("date")
fruits[${#fruits[@]}]="elderberry"  # same as +=

# Access
echo "${fruits}"         # apple
echo "${fruits[-1]}"        # elderberry (Bash 4.3+ negative index)
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
mapfile -t lines < file.txt         # strips trailing newlines with -t
readarray -t lines < file.txt       # synonym for mapfile

# Read command output into array (process substitution, not pipe — avoids subshell)
mapfile -t pids < <(pgrep nginx)

# mapfile -d delimiter (Bash 4.4+)
mapfile -d '' -t null_separated < <(find . -print0)
```

**Pitfall:** `"${arr[*]}"` in double quotes joins all elements with `IFS`. Almost always use `"${arr[@]}"`.

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

Requires Bash **4.0+**. Not available in `/bin/bash` on macOS (Bash 3.2).[^5][^6]

```bash
# Declaration — -A is mandatory
declare -A config

# Inline init (Bash 4.0+)
declare -A config=(
  [host]="localhost"
  [port]="5432"
  [user]="admin"
)

# Bash 5.1+ allows key/value alternating format:
# declare -A config=( host localhost port 5432 )  # Bash 5.1+

# Set / update
config[db_name]="production"

# Access
echo "${config[host]}"              # localhost

# Default value for missing key
echo "${config[missing_key]:-none}" # none

# Check if key exists
if [[ -v config[port] ]]; then      # -v tests for set variable (Bash 4.2+)
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

**Pitfall:** Empty strings cannot be used as associative array keys.[^4]

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
if [[ "2024-07-07" =~ ^([0-9]{4})-([0-9]{2})-([0-9]{2})$ ]]; then
  year="${BASH_REMATCH[1]}"
  month="${BASH_REMATCH[2]}"
  day="${BASH_REMATCH[3]}"
fi
```

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
# Prefer printf for variables — echo mishandles leading -, -n/-e, and escapes; plain literal echo is fine
var="-n tricky"
echo "$var"          # prints nothing on some systems (interprets -n flag!)

# ALWAYS use printf for reliable output
printf '%s\n' "$var"  # always prints: -n tricky
printf 'Count: %d\n' "$count"
printf '%s\t%s\n' "$key" "$value"
```

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

`IFS` (Internal Field Separator) defaults to space, tab, newline. Changing it affects `read`, `for`, and word splitting.[^7]

```bash
# Safe IFS for production scripts — prevents splitting on spaces
IFS=$'\n\t'

# Always restore IFS after temporary change
old_IFS="$IFS"
IFS=':'
read -ra path_parts <<< "$PATH"
IFS="$old_IFS"

# Or scope IFS change to a single command (does NOT persist)
IFS=',' read -ra csv_fields <<< "a,b,c"
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
for f in $(ls *.txt); do ...  # breaks on spaces

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
  # ...
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
```

##### Error handling in functions

```bash
# Functions inherit set -e and set -u from the calling shell
# Use || to handle failures
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

# EXIT is sufficient for most cleanup — runs on normal exit, errors, and signals
# that cause exit (INT, TERM with set -e)
trap cleanup EXIT

# For graceful shutdown on SIGINT/SIGTERM (e.g., kill background jobs)
handle_signal() {
  echo "Signal received, shutting down..." >&2
  # kill background jobs if any
  jobs -p | xargs -r kill 2>/dev/null || true
  exit 130   # 128 + 2 (SIGINT)
}
trap handle_signal INT TERM

# ERR trap — fires on any command returning non-zero (requires set -E to propagate into functions)
set -E
on_error() {
  local exit_code=$?
  local line_no="${BASH_LINENO}"
  local command="${BASH_COMMAND}"
  echo "ERROR: command '${command}' failed with exit code ${exit_code} at line ${line_no}" >&2
}
trap on_error ERR

TMPDIR_WORK=$(mktemp -d)
```

**Key rules:**[^3]

- `trap ... EXIT` runs on **all** exits including error exits — ideal for cleanup.
- Listing `trap ... INT TERM EXIT` causes cleanup to run **twice** when INT/TERM received. Use `EXIT` alone for cleanup; use separate INT/TERM traps only for graceful shutdown signaling.[^8]
- `trap ... ERR` requires `set -E` to fire inside functions.
- In cleanup functions: use `set +e` and `|| true` to prevent recursive errors.
- Create the temp dir **before** installing its cleanup trap, and never name the variable `TMPDIR`: `mktemp` consults `$TMPDIR`, so a trap that fires before the assignment can `rm -rf` the inherited real temp directory.

##### Lock files

```bash
LOCKFILE="/var/run/myapp.lock"

acquire_lock() {
  if ! mkdir "$LOCKFILE" 2>/dev/null; then
    echo "Script already running (lock: $LOCKFILE)" >&2
    exit 1
  fi
  trap 'rmdir "$LOCKFILE" 2>/dev/null || true' EXIT
}
```

***

#### 10. Exit Codes

Exit codes are 8-bit unsigned integers: **0–255**.[^9]

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General / unspecified error |
| `2` | Misuse of shell builtin / bad arguments |
| `64–78` | `sysexits.h` conventions (rarely enforced) |
| `126` | Command found but not executable |
| `127` | Command not found |
| `128` | Invalid exit argument |
| `128+N` | Fatal signal N (e.g., 137 = SIGKILL = 128+9) |
| `130` | Terminated by Ctrl-C (SIGINT = 128+2) |

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

# Check exit codes explicitly when set -e is off or insufficient
if ! command_that_might_fail; then
  echo "Failed with: $?" >&2
  exit 1
fi

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
  local msg="$*"
  local ts
  printf -v ts '%(%Y-%m-%dT%H:%M:%S%z)T' -1   # Bash 4.2+: builtin, no date fork

  # Numeric comparison for log level filtering
  local current_level="${_LOG_LEVELS[$LOG_LEVEL]:-1}"
  local msg_level="${_LOG_LEVELS[$level]:-1}"
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

# Restore on exit
trap 'exec 1>&3 2>&4; exec 3>&- 4>&-' EXIT
```

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

`exec` replaces the current shell process or globally redirects file descriptors.[^10]

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

# Pass to command directly
mysql -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" <<EOF
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
- `<<-` strips **tabs only** — mixing spaces and tabs causes silent misalignment.[^11]
- Variables and command substitutions are expanded unless delimiter is quoted.
- Trailing newline is always included.

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

# Read with timeout
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

**ShellCheck warning SC2162:** Always use `read -r` to avoid mangling backslashes.[^12]

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

# Source config files safely
config_file="${XDG_CONFIG_HOME:-$HOME/.config}/myapp/config"
[[ -f "$config_file" ]] && source "$config_file"

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
[[ "$count" -gt 0 ]]         # POSIX numeric comparison
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

# Process specific columns of CSV (handles quoted fields — use proper CSV tools for complex cases)
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
rg --null "pattern" . | xargs -0 ... # null-delimited output (rg also supports -0)
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
for f in $(ls *.txt); do ...

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

##### Avoid forks in loops

```bash
# SLOW — spawns a subprocess per iteration
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
wc -l *.txt                  # one wc invocation for all files
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

# Trace with timestamps — identify slow lines (date %N is GNU-only; not on macOS/BSD)
PS4='+ $(date "+%s%N") ${FUNCNAME:+${FUNCNAME}():}${LINENO}: '
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

```bash
# POSIX-safe alternatives
# Instead of [[ ]], use:
[ -f "$file" ]
case "$str" in pattern) ... esac

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
```

***

#### 27. Version Feature Matrix

| Feature | Min. Bash Version | Notes |
|---------|------------------|-------|
| Basic indexed arrays | **2.0** | |
| Here-string `<<<` | **2.05b** | |
| `[[ ]]` conditional | **2.02** | |
| `$(( ))` arithmetic | **2.0** | |
| `declare -a` indexed array | **2.0** | |
| `declare -A` associative array | **4.0** (2009) | Not on macOS `/bin/bash` |
| `mapfile` / `readarray` | **4.0** | |
| `declare -l` / `-u` case attrs | **4.0** | |
| `${var,,}` / `${var^^}` case ops | **4.0** | |
| `&>>` append shorthand | **4.0** | |
| `[[ -v var ]]` set test | **4.2** | |
| `declare -g` set global in func | **4.2** | |
| `declare -n` nameref | **4.3** | |
| `local -n` nameref in function | **4.3** | |
| Negative array indices (`arr[-1]`) | **4.3** | |
| `mapfile -d` custom delimiter | **4.4** | |
| Bash 5.0 `EPOCHSECONDS` / `EPOCHREALTIME` | **5.0** | |
| `SRANDOM` (high-quality random) | **5.1** | |
| Bash 5.1 associative array compound assign | **5.1** | |
| `[[ -R var ]]` nameref test | **4.3** | |
| `printf -v` print to variable | **3.1** | |
| `BASH_REMATCH` / `=~` | **3.0** | 3.2 changed RHS-quoting rule |
| `PIPESTATUS` array | **2.0** | |
| `shopt -s globstar` (`**`) | **4.0** | |
| `shopt -s nullglob` | **2.0** | |
| `shopt -s extglob` | **2.02** | |
| Process substitution `<()` | **2.0** | Not in POSIX `sh` |
| `coproc` (coprocess) | **4.0** | |

##### Checking version at runtime

```bash
# Check Bash version inside a script
if (( BASH_VERSINFO < 4 )); then
  echo "Requires Bash 4.0 or newer (found: $BASH_VERSION)" >&2
  exit 1
fi

# Check for specific feature
if (( BASH_VERSINFO > 4 || (BASH_VERSINFO == 4 && BASH_VERSINFO[1] >= 3) )); then
  # use declare -n
  declare -n ref="$target"
else
  # fallback with eval or indirect expansion
  ref_val="${!target}"
fi
```

***

#### Appendix: Minimal Production Template

```bash
#!/usr/bin/env bash
# ============================================================
# script_name.sh — Brief description
# Usage: script_name.sh [OPTIONS] <arg>
# Author: Your Name
# Requires: Bash 4.3+
# ============================================================
set -euo pipefail
IFS=$'\n\t'

# ── Constants ────────────────────────────────────────────────
readonly SCRIPT_NAME="${0##*/}"
readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
readonly LOG_FILE="${LOG_FILE:-/tmp/${SCRIPT_NAME%.sh}.log}"
DRY_RUN="${DRY_RUN:-false}"   # not readonly: reassigned by --dry-run below

# ── Logging ──────────────────────────────────────────────────
log()  { printf '[%(%T)T] [%s] %s\n' -1 "$1" "${*:2}" >&2; }
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
(( BASH_VERSINFO >= 4 )) || die "Bash 4+ required"

# ── Main ──────────────────────────────────────────────────────
main() {
  _TMPDIR=$(mktemp -d)
  info "Starting ${SCRIPT_NAME} (env=$ENV, dry_run=$DRY_RUN)"

  # ... your logic here ...
  info "Done"
}

main "$@"
```

---

#### References

1. [10 Bash Traps That Are Costing You Hours (And How to Fix Them)](https://dev.to/beta_shorts_7f1150259405a/10-bash-traps-that-are-costing-you-hours-and-how-to-fix-them-3949) - Bash scripts breaking for no reason? Here are 10 common mistakes that waste hours—and how to fix the...

2. [Bash Safety 101: Why set -euo pipefail Should Be in ... - LinkedIn](https://www.linkedin.com/posts/boris-levenzon_bash-safety-101-why-set-euo-pipefail-should-activity-7400210270041645056-LryT) - Bash Safety 101: Why set -euo pipefail Should Be in Every Production Script If you’re writing Bash s...

3. [bash: trap cleanup et signaux](https://data.pm/snippets/bash/bash-trap-cleanup-et-signaux/) - Nettoyer et terminer proprement avec trap: cleanup, ERR, EXIT et signaux INT/TERM.

4. [Arrays (Bash Reference Manual)](https://www.gnu.org/software/bash/manual/html_node/Arrays.html) - Arrays (Bash Reference Manual)

5. [How to Declare and Access Associative Array in Bash](https://phoenixnap.com/kb/bash-associative-array) - Bash associative arrays use key-value pairs to store data. Learn how to declare, access & use associ...

6. [Command Line Associative Arrays - Bash - Codecademy](https://www.codecademy.com/resources/docs/command-line/bash/associative-arrays) - Stores and retrieves data using key-value pairs in Bash scripts

7. [What is the meaning of IFS=$'\n' in bash scripting?](https://unix.stackexchange.com/questions/184863/what-is-the-meaning-of-ifs-n-in-bash-scripting) - At the beginning of a bash shell script is the following line: IFS=$'\n' What is the meaning behind ...

8. ["trap ... INT TERM EXIT" really necessary?](https://unix.stackexchange.com/questions/57940/trap-int-term-exit-really-necessary) - Many examples for trap use trap ... INT TERM EXIT for cleanup tasks. But is it really necessary to l...

9. [Exit Status Codes - OberstKrueger](https://oberstkrueger.com/status) - The homepage for OberstKrueger.

10. [Bash exec File-Descriptor Redirection Logging Prompt](https://devopsaitoolkit.com/prompts/bash-exec-fd-redirection-logging/) - Wire script-wide logging with exec, custom file descriptors, and tee to split stdout/stderr to conso...

11. [3.3. Preventing Weird Behavior in a Here-Document - bash ... - O'Reilly](https://www.oreilly.com/library/view/bash-cookbook/0596526784/ch03s03.html) - Preventing Weird Behavior in a Here-DocumentProblemYour here-document is behaving weirdly. You tried...

12. [Mapfile/Readarray | Poop Sheet](https://poopsheet.co.za/bash/builtins/mapfile-readarray/)

