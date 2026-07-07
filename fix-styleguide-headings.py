#!/usr/bin/env python3
"""Normalise heading levels in styleguide markdown files.

Rather than blindly shifting every heading by a fixed amount (which would
re-shift on every run), the ATX headings *outside* fenced code blocks are
re-levelled to form a valid, gap-free hierarchy anchored at level 3:

  * the outermost heading in the document becomes `###`;
  * each step deeper in the logical nesting adds exactly one `#`
    (`###` -> `####` -> `#####` -> `######`), so no levels are ever skipped;
  * a heading that would exceed level 6 is rendered as bold text instead,
    since Markdown has no heading past `######`.

Nesting is derived from the source `#` counts: a heading with more `#` than
the one before it is treated as a child; equal-or-fewer `#` closes back up to
the matching ancestor. A source jump like `###` -> `#####` (skipping `####`)
is repaired to `###` -> `####`.

Because the output is a valid, anchored hierarchy, running the script again is
a no-op — a document whose top heading is already `###` with correct nesting
maps to itself and is left untouched. Examples:

    #      (outermost)          -> ###
    ##     (child of the above) -> ####
    ###    (already outermost)  -> ###          (unchanged on re-run)
    #####  Deep Header          -> **Deep Header**   (level > 6 -> bold)

Rules:
  * `#` characters inside fenced code blocks (``` or ~~~) are never touched,
    so shell/yaml comments like `# Wrong` stay intact.
  * Closing `#` characters on a heading (`## Title ##`) are stripped.
  * Leading indentation (up to 3 spaces, per CommonMark) is preserved.
  * Empty headings (`#`, `##`, ...) are left as-is.

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
BASE_LEVEL = 3  # the outermost heading is anchored here (###)
MAX_LEVEL = 6   # deeper than this -> rendered as bold text

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


def render_heading(indent: str, level: int, text: str) -> str:
    """Render a heading at `level`, falling back to bold past level 6."""
    if level <= MAX_LEVEL:
        return f"{indent}{'#' * level} {text}"
    return f"{indent}**{text}**"


def process_text(text: str) -> tuple[str, int]:
    """Re-level headings in `text` into a valid hierarchy, skipping code blocks.

    A stack of ``(source_level, output_level)`` tracks the current nesting.
    Each heading is attached one level below its nearest shallower ancestor,
    anchored at BASE_LEVEL for the outermost heading — so the output is always
    gap-free and idempotent. Returns the new text and how many lines changed.
    """
    lines = text.split("\n")
    out: list[str] = []
    changed = 0

    fence_marker: str | None = None  # active fence, e.g. "```" or "~~~~"
    stack: list[tuple[int, int]] = []  # (source_level, output_level)

    for line in lines:
        fm = FENCE_RE.match(line)
        if fm:
            marker = fm.group("fence")
            if fence_marker is None:
                # Opening fence.
                fence_marker = marker
                out.append(line)
                continue
            # Inside a block: a closing fence must be the same char and at
            # least as long, and carry no info string.
            if (
                marker[0] == fence_marker[0]
                and len(marker) >= len(fence_marker)
                and fm.group("info").strip() == ""
            ):
                fence_marker = None
            out.append(line)
            continue

        if fence_marker is not None:
            out.append(line)  # inside code block — untouched
            continue

        m = HEADING_RE.match(line)
        if not m:
            out.append(line)
            continue

        text_content = m.group("text").strip()
        if not text_content:  # empty heading (`#`, `##` ...) — leave as-is
            out.append(line)
            continue

        indent = m.group("indent")
        source_level = len(m.group("hashes"))

        # Close back up to the nearest strictly-shallower ancestor.
        while stack and stack[-1][0] >= source_level:
            stack.pop()
        output_level = BASE_LEVEL if not stack else stack[-1][1] + 1
        stack.append((source_level, output_level))

        new_line = render_heading(indent, output_level, text_content)
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
