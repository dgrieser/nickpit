#!/usr/bin/env python3
"""Extract reasoning blocks from nickpit review_test logs into a JSONL corpus.

Ground truth comes from in-log markers tied to (agent label, call number):
  loopdet - "Reasoning loop detected, retrying with lower effort"  (old detector fired)
  chunk   - "Repeated chunk detected" / provider repeated-chunk retry
  timeout - "Reasoning time limit exceeded"                        (undetected loop, worst case)
  empty   - "Reasoning-only empty response, retrying"              (reasoning ended with no output)
  clean   - no marker for that logical call

Each record: {id, log, header_line, label, duration_s, kind, chars, text}
A block may concatenate reasoning from several attempts of one logical call
(the looping attempt plus its retries), so kinds mark "this call contained a
<kind> attempt"; precedence: chunk > loopdet > timeout > empty.
"""
import json
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(REPO, "loop_corpus.jsonl")

LOGLINE = re.compile(
    r"^(?:\+ \[|(?:Reasoning|Response|Tool|Agent|Model|ModelCheck|Progress|Review|Verify|Context|Findings|Summary|Nudge|Retry|Publish|Dedupe|Collect|Filter|Sanitize|Repair|Location|Time|Budget|Error|Warning|Info|Debug|Git|SCM|Step|Workflow|Engine|Pipeline|Lane|Extract|Judge)\s+\[)"
)
HEADER = re.compile(r"^Reasoning for (?P<label>.+?)\.\.\.$")
DONE = re.compile(r"^Reasoning\s+\[(?P<agent>[^\]]+)\]\s+#(?P<attempt>\d+) done (?:(?P<hours>\d+)h)?(?:(?P<mins>\d+)m)?(?P<secs>\d+(?:\.\d+)?)s")
MARKER = re.compile(r"^\+ \[(?P<agent>.+?)\] #(?P<call>\d+) (?P<msg>Reasoning loop detected, retrying|Repeated chunk|Reasoning time limit exceeded|Reasoning-only empty response, retrying)")

KIND_RANK = {"clean": 0, "empty": 1, "timeout": 2, "loopdet": 3, "chunk": 4}


def done_seconds(m):
    return (int(m.group("hours") or 0) * 3600
            + int(m.group("mins") or 0) * 60
            + float(m.group("secs")))


def marker_kind(msg):
    if msg.startswith("Reasoning loop detected"):
        return "loopdet"
    if msg.startswith("Repeated chunk"):
        return "chunk"
    if msg.startswith("Reasoning time limit"):
        return "timeout"
    return "empty"


def agent_to_label_prefix(agent):
    """Map a marker's agent tag to the printed block-label prefix.

    'verify: Testing #10 · Qwen... · humanReadableBytes edge...' -> 'verify: Testing #10: humanReadableBytes edge...'
    'review: Security · Nudge 1/3 · Qwen...'                     -> 'review: Security · Nudge 1/3'
    'review: Architecture · Qwen...'                             -> 'review: Architecture'
    """
    parts = agent.split(" · ")
    keep = [p for p in parts if "Qwen" not in p and "FP8" not in p]
    if not keep:
        return agent
    if keep[0].startswith("verify:") and len(keep) >= 2:
        return keep[0] + ": " + " · ".join(keep[1:])
    return " · ".join(keep)


def extract_blocks(log_path):
    with open(log_path, errors="replace") as f:
        lines = f.readlines()
    n = len(lines)

    # pass 1: markers -> {(label_prefix, call_no): kind}
    marks = {}
    for line in lines:
        m = MARKER.match(line)
        if not m:
            continue
        key = (agent_to_label_prefix(m.group("agent")), int(m.group("call")))
        kind = marker_kind(m.group("msg"))
        if KIND_RANK[kind] > KIND_RANK.get(marks.get(key, "clean"), 0):
            marks[key] = kind

    # pass 2: blocks
    blocks = []
    last_done = None
    i = 0
    while i < n:
        line = lines[i].rstrip("\n")
        m = DONE.match(line)
        if m:
            last_done = (i + 1, done_seconds(m))
        hm = HEADER.match(line)
        if hm:
            header_lineno = i + 1
            label = hm.group("label")
            body = []
            j = i + 1
            while j < n:
                l2 = lines[j].rstrip("\n")
                if LOGLINE.match(l2) or HEADER.match(l2):
                    break
                body.append(lines[j])
                j += 1
            dur = None
            if last_done is not None and header_lineno - last_done[0] <= 4:
                dur = last_done[1]
            # label ends with " #N"
            call_no = None
            base = label
            lm = re.match(r"^(?P<base>.*) #(?P<no>\d+)$", label)
            if lm:
                base = lm.group("base")
                call_no = int(lm.group("no"))
            kind = marks.get((base, call_no), "clean")
            blocks.append((header_lineno, label, dur, kind, "".join(body).strip("\n")))
            i = j
            continue
        i += 1
    return blocks, marks


def main():
    logs = sorted(
        p for p in os.listdir(REPO)
        if re.match(r"review_test-2026-(06-30|07-01)-.*\.log$", p)
    )
    records = []
    unmatched_marks = []
    for name in logs:
        blocks, marks = extract_blocks(os.path.join(REPO, name))
        seen_keys = set()
        for header_lineno, label, dur, kind, text in blocks:
            lm = re.match(r"^(?P<base>.*) #(?P<no>\d+)$", label)
            if lm:
                seen_keys.add((lm.group("base"), int(lm.group("no"))))
            if not text:
                continue
            stamp = name.replace("review_test-2026-", "").replace(".log", "")
            records.append({
                "id": f"{stamp}:{header_lineno}",
                "log": name,
                "header_line": header_lineno,
                "label": label,
                "duration_s": dur,
                "kind": kind,
                "chars": len(text),
                "text": text,
            })
        for key, kind in marks.items():
            if key not in seen_keys:
                unmatched_marks.append((name, key, kind))

    with open(OUT, "w") as f:
        for r in records:
            f.write(json.dumps(r) + "\n")
    counts = {}
    for r in records:
        counts[r["kind"]] = counts.get(r["kind"], 0) + 1
    print(f"wrote {len(records)} records to {OUT}: {counts}")
    if unmatched_marks:
        print(f"{len(unmatched_marks)} markers had no matching reasoning block:")
        for name, key, kind in unmatched_marks[:20]:
            print(f"  {name}: {key} -> {kind}")


if __name__ == "__main__":
    main()
