#!/usr/bin/env python3
"""Fix heading levels in styleguide markdown files.

Every ATX heading found *outside* fenced code blocks is shifted down by two
levels:

    #      -> ###
    ##     -> ####
    ###    -> #####
    ####   -> ######

Any heading that would exceed level 6 is rendered as bold text instead, since
Markdown has no heading levels beyond `######`:

    #####  Current Header -> **Current Header**
    ###### Current Header -> **Current Header**

Rules:
  * `#` characters inside fenced code blocks (``` or ~~~) are never touched,
    so shell/yaml comments like `# Wrong` stay intact.
  * Closing `#` characters on a heading (`## Title ##`) are stripped.
  * Leading indentation (up to 3 spaces, per CommonMark) is preserved.

Usage:
    ./fix-styleguide-headings.py                 # fix prompts/styleguides/*.md
    ./fix-styleguide-headings.py path/to/file.md # fix specific files
    ./fix-styleguide-headings.py some/dir        # fix *.md under a directory
    ./fix-styleguide-headings.py --dry-run       # preview, write nothing
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

DEFAULT_DIR = Path("prompts/styleguides")
SHIFT = 2
MAX_LEVEL = 6

# ATX heading: up to 3 leading spaces, 1-6 '#', a space, the text, optional
# trailing '#' sequence. Text is required (empty headings are left untouched).
HEADING_RE = re.compile(
    r"^(?P<indent>\s{0,3})"
    r"(?P<hashes>#{1,6})"
    r"[ \t]+"
    r"(?P<text>.*?)"
    r"(?:[ \t]+#+)?"
    r"[ \t]*$"
)

# Opening/closing fence: up to 3 leading spaces then >=3 backticks or tildes.
FENCE_RE = re.compile(r"^(?P<indent>\s{0,3})(?P<fence>`{3,}|~{3,})(?P<info>.*)$")


def transform_line(line: str) -> str:
    """Return the heading-shifted version of a single non-code-block line."""
    m = HEADING_RE.match(line)
    if not m:
        return line

    text = m.group("text").strip()
    if not text:  # empty heading (`#`, `##` ...) — leave as-is
        return line

    indent = m.group("indent")
    new_level = len(m.group("hashes")) + SHIFT

    if new_level <= MAX_LEVEL:
        return f"{indent}{'#' * new_level} {text}"
    return f"{indent}**{text}**"


def process_text(text: str) -> tuple[str, int]:
    """Shift headings in `text`, skipping fenced code blocks.

    Returns the new text and the number of headings changed.
    """
    lines = text.split("\n")
    out: list[str] = []
    changed = 0

    fence_marker: str | None = None  # active fence char, e.g. "```" or "~~~~"

    for line in lines:
        fm = FENCE_RE.match(line)
        if fm:
            marker = fm.group("fence")
            fence_char = marker[0]
            if fence_marker is None:
                # Opening fence.
                fence_marker = marker
                out.append(line)
                continue
            # Inside a block: a closing fence must be the same char and at
            # least as long, and carry no info string.
            if (
                fence_char == fence_marker[0]
                and len(marker) >= len(fence_marker)
                and fm.group("info").strip() == ""
            ):
                fence_marker = None
            out.append(line)
            continue

        if fence_marker is not None:
            out.append(line)  # inside code block — untouched
            continue

        new_line = transform_line(line)
        if new_line != line:
            changed += 1
        out.append(new_line)

    return "\n".join(out), changed


def collect_files(args: list[str]) -> list[Path]:
    if not args:
        targets = [DEFAULT_DIR]
    else:
        targets = [Path(a) for a in args]

    files: list[Path] = []
    for t in targets:
        if t.is_dir():
            files.extend(sorted(t.rglob("*.md")))
        elif t.is_file():
            files.append(t)
        else:
            print(f"warning: skipping missing path {t}", file=sys.stderr)
    return files


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("paths", nargs="*", help="markdown files or dirs (default: prompts/styleguides)")
    parser.add_argument("-n", "--dry-run", action="store_true", help="preview changes without writing")
    opts = parser.parse_args()

    files = collect_files(opts.paths)
    if not files:
        print("no markdown files found", file=sys.stderr)
        return 1

    total_files = 0
    total_headings = 0
    for f in files:
        original = f.read_text(encoding="utf-8")
        updated, changed = process_text(original)
        if changed == 0:
            print(f"  = {f} (no headings changed)")
            continue
        total_files += 1
        total_headings += changed
        action = "would change" if opts.dry_run else "changed"
        print(f"  ✓ {f} ({action} {changed} heading(s))")
        if not opts.dry_run:
            f.write_text(updated, encoding="utf-8")

    verb = "would update" if opts.dry_run else "updated"
    print(f"\n{verb} {total_headings} heading(s) across {total_files} file(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
