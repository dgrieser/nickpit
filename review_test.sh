#!/bin/bash

APP_NAME="$(basename "${0}")"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
nickpit_bin="${script_dir}/bin/nickpit"

usage() {
    cat <<EOF 1>&2
usage: ${APP_NAME} --workdir WORKDIR [OPTIONS]

Run repeated local branch reviews against a working tree.

Options:
  -p, --profile PROFILE     Config profile to use (default: ${DEFAULT_PROFILE})
  --gitlab-id ID            GitLab Merge Request ID
  --gitlab-repo REPO        GitLab repository
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

DEFAULT_PROFILE="mittwald"
DEFAULT_LOG_FILE_PREFIX="${script_dir}/review_test-"
DEFAULT_LOG_FILE_SUFFIX="log"
DEFAULT_REVIEW_COUNT=10

profile="${DEFAULT_PROFILE}"
gitlab_id=""
gitlab_repo=""
log_file_prefix="${DEFAULT_LOG_FILE_PREFIX}"
log_file_suffix="${DEFAULT_LOG_FILE_SUFFIX}"
review_count="${DEFAULT_REVIEW_COUNT}"

while [ "${#}" -gt 0 ]; do
    case "${1}" in
        -h|--help)
            usage
            ;;
        --gitlab-id)
            shift
            [ -z "${1}" ] && usage
            gitlab_id="${1}"
            ;;
        --gitlab-repo)
            shift
            [ -z "${1}" ] && usage
            gitlab_repo="${1}"
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

[ -z "${gitlab_id}" ] || [ -z "${gitlab_repo}" ] && usage

make

nickpit_args=(
    gitlab
    mr
    --id "${gitlab_id}"
    --repo "${gitlab_repo}"
    --profile "${profile}"
    --show-progress
    --show-reasoning
    --max-findings 6
    --verbose
)

for i in $(seq 1 ${review_count}); do
    log_file="${log_file_prefix}$(date +%Y-%m-%d-%H-%M-%S).${log_file_suffix}"
    "${nickpit_bin}" "${nickpit_args[@]}" 2>&1 | tee "${log_file}"
done
