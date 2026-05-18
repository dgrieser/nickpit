#!/bin/bash

APP_NAME="$(basename "${0}")"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
nickpit_bin="${script_dir}/bin/nickpit"

usage() {
    cat <<EOF 1>&2
usage: ${APP_NAME} --workdir WORKDIR [OPTIONS]

Run repeated local branch reviews against a working tree.

Options:
  -w, --workdir WORKDIR     Working directory to review (required)
  --base BRANCH             Branch to review against
  --head BRANCH             Branch to review
  -m, --model MODEL         Model to use (default: ${DEFAULT_MODEL})
  -e, --effort EFFORT       Reasoning effort to use (default: ${DEFAULT_EFFORT})
  -p, --profile PROFILE     Config profile to use (default: ${DEFAULT_PROFILE})
  --log-file-prefix PREFIX  Prefix for log file names (default: ${DEFAULT_LOG_FILE_PREFIX})
  --log-file-suffix SUFFIX  Suffix for log file names (default: ${DEFAULT_LOG_FILE_SUFFIX})
  --review-count COUNT      Number of reviews to run (default: ${DEFAULT_REVIEW_COUNT})
  -h, --help                Show this help message and exit
EOF
    exit 1
}

error() {
    echo "ERROR: ${1}" 1>&2
}

warning() {
    echo "WARNING: ${1}" 1>&2
}

fail() {
    error "${1}"
    exit 1
}

abort() {
    echo "Abort." 1>&2
    exit 1
}

DEFAULT_MODEL="Qwen3.5-122B-A10B-FP8"
DEFAULT_EFFORT="high"
DEFAULT_PROFILE="mittwald"
DEFAULT_LOG_FILE_PREFIX="${script_dir}/review_test-"
DEFAULT_LOG_FILE_SUFFIX="log"
DEFAULT_REVIEW_COUNT=10

workdir=""
base_branch=""
head_branch=""
model="${DEFAULT_MODEL}"
effort="${DEFAULT_EFFORT}"
profile="${DEFAULT_PROFILE}"
log_file_prefix="${DEFAULT_LOG_FILE_PREFIX}"
log_file_suffix="${DEFAULT_LOG_FILE_SUFFIX}"
review_count="${DEFAULT_REVIEW_COUNT}"

while [ "${#}" -gt 0 ]; do
    case "${1}" in
        -h|--help)
            usage
            ;;
        -w|--workdir)
            shift
            [ -z "${1}" ] && usage
            workdir="${1}"
            ;;
        --base)
            shift
            [ -z "${1}" ] && usage
            base_branch="${1}"
            ;;
        --head)
            shift
            [ -z "${1}" ] && usage
            head_branch="${1}"
            ;;
        -m|--model)
            shift
            [ -z "${1}" ] && usage
            model="${1}"
            ;;
        -e|--effort)
            shift
            [ -z "${1}" ] && usage
            effort="${1}"
            ;;
        -p|--profile)
            shift
            [ -z "${1}" ] && usage
            profile="${1}"
            ;;
        --log-file-prefix)
            shift
            [ -z "${1}" ] && usage
            log_file_prefix="${1}"
            ;;
        --log-file-suffix)
            shift
            [ -z "${1}" ] && usage
            log_file_suffix="${1}"
            ;;
        --review-count)
            shift
            [ -z "${1}" ] && usage
            [[ ! "${1}" =~ ^[0-9]+$ ]] && usage
            [ "${1}" -lt 1 ] && usage
            review_count="${1}"
            ;;
        -*)
            error "Unknown option: ${1}"
            usage
            ;;
        *)
            error "Unexpected argument: ${1}"
            usage
            ;;
    esac
    shift
done

[ -z "${workdir}" ] && usage
[ -d "${workdir}" ] || fail "Workdir does not exist: ${workdir}"

make

nickpit_args=(
    local
    branch
    --workdir "${workdir}"
    --show-progress
    --json
    --profile "${profile}"
    --model "${model}"
    --show-reasoning
    --use-json-schema
    --reasoning-effort "${effort}"
    --verbose
)

[ -n "${base_branch}" ] && nickpit_args+=(--base "${base_branch}")
[ -n "${head_branch}" ] && nickpit_args+=(--head "${head_branch}")

for i in $(seq 1 ${review_count}); do
    log_file="${log_file_prefix}$(date +%Y-%m-%d-%H-%M-%S).${log_file_suffix}"
    "${nickpit_bin}" "${nickpit_args[@]}" 2>&1 | tee "${log_file}"
done
