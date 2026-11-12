#!/usr/bin/env bash
# Checks the vendored copy of cloudflare/security-audit-skill.
#
# 1. Verifies every file under third_party/security-audit-skill/ still matches
#    the sha256 recorded in PROVENANCE.yaml, so a local edit to the pinned
#    upstream text cannot slip in unnoticed.
# 2. Fetches the upstream repository at REF (default: main) into a temporary
#    directory and prints a diff of the pinned commit against it, so a bump is
#    reviewed like any other prompt change before prompts/security/*.md is
#    re-condensed from it.
#
# Never updates anything by itself. Usage: tools/check_security_upstream.sh [REF]
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
vendor="$root/third_party/security-audit-skill"
provenance="$vendor/PROVENANCE.yaml"
ref=${1:-main}

repo=$(sed -n 's/^repository: //p' "$provenance")
pinned=$(sed -n 's/^commit: //p' "$provenance")
if [[ -z "$repo" || -z "$pinned" ]]; then
	echo "error: repository or commit missing in $provenance" >&2
	exit 1
fi

status=0
while read -r name hash; do
	name=${name%:}
	file="$vendor/$name"
	if [[ ! -f "$file" ]]; then
		echo "MISSING  $name"
		status=1
		continue
	fi
	actual=$(sha256sum "$file" | cut -d' ' -f1)
	if [[ "$actual" != "$hash" ]]; then
		echo "MODIFIED $name"
		status=1
	fi
done < <(sed -n '/^files:/,$p' "$provenance" | sed -n 's/^  \([^ ]*\) \([0-9a-f]\{64\}\)$/\1 \2/p')
for file in "$vendor"/*; do
	name=$(basename "$file")
	[[ "$name" == "PROVENANCE.yaml" ]] && continue
	if ! grep -q "^  $name: " "$provenance"; then
		echo "UNLISTED $name"
		status=1
	fi
done
if [[ $status -ne 0 ]]; then
	echo "error: vendored files do not match $provenance" >&2
	exit "$status"
fi
echo "vendored files match pinned commit $pinned"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
git -C "$tmp" init -q
git -C "$tmp" remote add origin "$repo"
git -C "$tmp" fetch -q --filter=blob:none origin "$pinned"
git -C "$tmp" fetch -q --filter=blob:none origin "$ref"
head=$(git -C "$tmp" rev-parse FETCH_HEAD)
if [[ "$head" == "$pinned" ]]; then
	echo "upstream $ref is at the pinned commit; nothing to review"
	exit 0
fi
echo "upstream $ref is at $head; changes since pinned $pinned:"
git -C "$tmp" --no-pager log --oneline "$pinned..$head"
git -C "$tmp" --no-pager diff --stat "$pinned" "$head" -- skills/security-audit LICENSE
echo
echo "Review with: git -C <clone> diff $pinned $head -- skills/security-audit"
