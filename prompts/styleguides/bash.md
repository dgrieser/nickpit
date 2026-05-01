# Bash Style Guide

Defensive Bash programming techniques for production-grade scripts.

## Strict Mode

Enable at the start of every script.

```bash
#!/bin/bash
set -Eeuo pipefail  # Exit on error, unset variables, pipe failures
```

Key flags:
- `set -E`: Inherit ERR trap in functions
- `set -e`: Exit on any error
- `set -u`: Exit on undefined variable reference
- `set -o pipefail`: Pipe fails if any command fails (not just last)

## Error Trapping and Cleanup

```bash
trap 'echo "Error on line $LINENO"' ERR
trap 'rm -rf -- "$TMPDIR"' EXIT

TMPDIR=$(mktemp -d)
```

## Variable Safety

Always quote variables to prevent word splitting and globbing.

```bash
# Wrong
cp $source $dest

# Correct
cp "$source" "$dest"

# Fail with message if unset
: "${REQUIRED_VAR:?REQUIRED_VAR is not set}"
```

## Array Handling

```bash
declare -a items=("item 1" "item 2" "item 3")

for item in "${items[@]}"; do
    echo "Processing: $item"
done

mapfile -t lines < <(some_command)
```

## Conditionals

Use `[[ ]]` for Bash-specific features.

```bash
if [[ -f "$file" && -r "$file" ]]; then
    content=$(<"$file")
fi

if [[ -z "${VAR:-}" ]]; then
    echo "VAR is not set or is empty"
fi
```

## Script Directory Detection

```bash
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
SCRIPT_NAME="$(basename -- "${BASH_SOURCE[0]}")"
```

## Function Template

```bash
validate_file() {
    local -r file="$1"
    local -r message="${2:-File not found: $file}"

    if [[ ! -f "$file" ]]; then
        echo "ERROR: $message" >&2
        return 1
    fi
}

process_files() {
    local -r input_dir="$1"
    local -r output_dir="$2"

    [[ -d "$input_dir" ]] || { echo "ERROR: input_dir not a directory" >&2; return 1; }
    mkdir -p "$output_dir" || { echo "ERROR: Cannot create output_dir" >&2; return 1; }

    while IFS= read -r -d '' file; do
        echo "Processing: $file"
    done < <(find "$input_dir" -maxdepth 1 -type f -print0)
}
```

## Temporary Files

```bash
trap 'rm -rf -- "$TMPDIR"' EXIT
TMPDIR=$(mktemp -d) || { echo "ERROR: Failed to create temp directory" >&2; exit 1; }
TMPFILE="$TMPDIR/temp.txt"
```

## Argument Parsing

```bash
VERBOSE=false
DRY_RUN=false
OUTPUT_FILE=""
THREADS=4

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
    -v, --verbose       Enable verbose output
    -d, --dry-run       Run without making changes
    -o, --output FILE   Output file path
    -j, --jobs NUM      Number of parallel jobs
    -h, --help          Show this help message
EOF
    exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -v|--verbose)   VERBOSE=true; shift ;;
        -d|--dry-run)   DRY_RUN=true; shift ;;
        -o|--output)    OUTPUT_FILE="$2"; shift 2 ;;
        -j|--jobs)      THREADS="$2"; shift 2 ;;
        -h|--help)      usage 0 ;;
        --)             shift; break ;;
        *)              echo "ERROR: Unknown option: $1" >&2; usage 1 ;;
    esac
done

[[ -n "$OUTPUT_FILE" ]] || { echo "ERROR: -o/--output is required" >&2; usage 1; }
```

## Structured Logging

```bash
log_info()  { echo "[$(date +'%Y-%m-%d %H:%M:%S')] INFO: $*" >&2; }
log_warn()  { echo "[$(date +'%Y-%m-%d %H:%M:%S')] WARN: $*" >&2; }
log_error() { echo "[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2; }
log_debug() {
    if [[ "${DEBUG:-0}" == "1" ]]; then
        echo "[$(date +'%Y-%m-%d %H:%M:%S')] DEBUG: $*" >&2
    fi
}
```

## Process Orchestration with Signals

```bash
PIDS=()

cleanup() {
    for pid in "${PIDS[@]}"; do
        kill -0 "$pid" 2>/dev/null && kill -TERM "$pid" 2>/dev/null || true
    done
    for pid in "${PIDS[@]}"; do
        wait "$pid" 2>/dev/null || true
    done
}

trap cleanup SIGTERM SIGINT

background_task &
PIDS+=($!)

wait
```

## Safe File Operations

```bash
safe_move() {
    local -r source="$1"
    local -r dest="$2"
    [[ -e "$source" ]] || { echo "ERROR: Source does not exist: $source" >&2; return 1; }
    [[ ! -e "$dest" ]] || { echo "ERROR: Destination already exists: $dest" >&2; return 1; }
    mv "$source" "$dest"
}

atomic_write() {
    local -r target="$1"
    local tmpfile
    tmpfile=$(mktemp) || return 1
    cat > "$tmpfile"
    mv "$tmpfile" "$target"
}
```

## Idempotent Design

Scripts should be safe to rerun.

```bash
ensure_directory() {
    local -r dir="$1"
    [[ -d "$dir" ]] && return 0
    mkdir -p "$dir" || { log_error "Failed to create directory: $dir"; return 1; }
}
```

## Dry-Run Support

```bash
DRY_RUN="${DRY_RUN:-false}"

run_cmd() {
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY RUN] Would execute: $*"
        return 0
    fi
    "$@"
}

run_cmd cp "$source" "$dest"
```

## Named Parameters Pattern

```bash
process_data() {
    local input_file="" output_dir="" format="json"

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --input=*)  input_file="${1#*=}" ;;
            --output=*) output_dir="${1#*=}" ;;
            --format=*) format="${1#*=}" ;;
            *)          echo "ERROR: Unknown parameter: $1" >&2; return 1 ;;
        esac
        shift
    done

    [[ -n "$input_file" ]] || { echo "ERROR: --input is required" >&2; return 1; }
    [[ -n "$output_dir" ]] || { echo "ERROR: --output is required" >&2; return 1; }
}
```

## Dependency Checking

```bash
check_dependencies() {
    local -a missing_deps=()
    local -a required=("jq" "curl" "git")

    for cmd in "${required[@]}"; do
        command -v "$cmd" &>/dev/null || missing_deps+=("$cmd")
    done

    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        echo "ERROR: Missing required commands: ${missing_deps[*]}" >&2
        return 1
    fi
}
```

## NUL-Safe File Iteration

```bash
while IFS= read -r -d '' file; do
    echo "Processing: $file"
done < <(find /path -type f -print0)
```

## Best Practices

1. Always use strict mode: `set -Eeuo pipefail`
2. Quote all variables: `"$variable"`
3. Use `[[ ]]` conditionals over `[ ]`
4. Implement error trapping; cleanup with `trap`
5. Validate all inputs (file existence, permissions, formats)
6. Use `local -r` for function parameters
7. Implement structured logging with timestamps and levels
8. Support dry-run mode
9. Use `mktemp` for temporary files; clean up with `trap`
10. Design for idempotency — scripts should be safe to rerun
11. Use `command -v` over `which` for checking executables
12. Prefer `printf` over `echo` for predictability
13. Use `mapfile`/`readarray` for reading command output into arrays
14. Always use `--` before file arguments to prevent flag injection
