### Citations: Python 3.9: The Complete Production Developer Guide

Sources for `python-3.9.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Python 3.9 What's New. Python Software Foundation. https://docs.python.org/3/whatsnew/3.9.html (release/EOL dates: PEP 596, https://peps.python.org/pep-0596/; annual cadence: PEP 602, https://peps.python.org/pep-0602/)
    - Overview — paragraph 1: "…31, 2025 with the final release 3.9.25; plan upgrades accordingly."
    - Overview — paragraph 1: "…long-deprecated Python-2-era APIs (see the migration notes near the end)."
    - New in 3.9: Relaxed Decorator Grammar (PEP 614) — paragraph 1: "…not just a dotted name optionally followed by a call."
    - File I/O and `pathlib` › New in 3.9: `Path.is_relative_to()` — paragraph 1: "…— the recommended way to do path-traversal checks in 3.9+:"
    - `async`/`await` and `asyncio` › New in 3.9: `asyncio.to_thread()` — paragraph 1: "…wrapper around `loop.run_in_executor()` for running blocking code in a thread:"
    - `async`/`await` and `asyncio` › Removed/Deprecated asyncio APIs in 3.9 — bullet 1: "…3.9** — use the module-level `asyncio.current_task()` and `asyncio.all_tasks()` (both 3.7+)."
    - `async`/`await` and `asyncio` › Removed/Deprecated asyncio APIs in 3.9 — bullet 2: "…no longer works in 3.9 — use `async with lock:`."
    - Concurrency › `concurrent.futures` — New in 3.9: `cancel_futures` — paragraph 3: "…longer use daemon threads — fixing hangs/races at interpreter shutdown."
    - The New PEG Parser (CPython Internals) — paragraph 1: "From an application developer's perspective:"
    - Logging › New in 3.9: `logging.getLogger('root')` change — paragraph 1: "…change for any code that explicitly names a logger `'root'`."
    - `random` Module (New in 3.9: `randbytes`) — paragraph 1: "…do not use it for security tokens — use `secrets.token_bytes()`."
    - Memory Management — paragraph 1: "…for specific code sections rather than the entire program run."
    - Removed and Changed in 3.9 — Migration Notes — paragraph 1: "…in 3.9 — old code breaks here, not at 3.10:"
[^2]: Python 3.7 language guarantee: dict insertion order. https://docs.python.org/3.7/whatsnew/3.7.html
    - Overview — bullet 6: "…by insertion.** Guaranteed by the language spec since Python 3.7."
[^3]: collections ABC aliases: DeprecationWarning in 3.9, removed in 3.10 (bpo-37324). https://docs.python.org/3/whatsnew/3.10.html#removed
    - Overview — bullet 7: "…now or your 3.9 codebase will not run on 3.10."
    - `collections.abc` Migration (Critical for 3.9 Compatibility) — paragraph 1: "…and `collections.Mapping` attribute access raises `AttributeError`.** Fix all occurrences now."
[^4]: Python 3.9 Built-in Types. https://docs.python.org/3.9/library/stdtypes.html
    - PEP 8 Code Style › Naming Conventions — paragraph 1: "Never write the latter."
    - Primitive Types and Numerics › Integers — paragraph 1: "Python allocates more memory silently."
    - Primitive Types and Numerics › Floats — paragraph 1: "New `math` additions in 3.9:"
    - Primitive Types and Numerics › Booleans — paragraph 1: "`True == 1` and `False == 0`."
[^5]: PEP 585 – Type Hinting Generics In Standard Collections. https://peps.python.org/pep-0585/
    - New in 3.9: Built-in Generic Type Hints (PEP 585) — paragraph 1: "…generics in type annotations**, without importing capitalized equivalents from `typing`."
[^6]: PEP 584 – Add Union Operators To dict. https://peps.python.org/pep-0584/
    - New in 3.9: Dict Merge and Update Operators (PEP 584) — paragraph 1: "…(merge, returns new dict) and `|=` (update in-place) to `dict`."
[^7]: PEP 616 – String methods to remove prefixes and suffixes. https://peps.python.org/pep-0616/
    - New in 3.9: `str.removeprefix()` and `str.removesuffix()` (PEP 616) — paragraph 1: "Two long-requested string methods that do exactly one thing, clearly."
[^8]: PEP 593 – Flexible function and variable annotations (typing.Annotated). https://peps.python.org/pep-0593/
    - Variable Annotations and Type Hints › New in 3.9: `typing.Annotated` (PEP 593) — paragraph 1: "…first argument is the actual type; subsequent arguments are metadata."
[^9]: PEP 615 – Support for the IANA Time Zone Database in the Standard Library. https://peps.python.org/pep-0615/
    - New in 3.9: `zoneinfo` — IANA Time Zone Support (PEP 615) — paragraph 1: "This replaces the long-standing dependence on the third-party `pytz` library."
[^10]: zoneinfo — IANA time zone support. https://docs.python.org/3.9/library/zoneinfo.html
    - New in 3.9: `zoneinfo` — IANA Time Zone Support (PEP 615) › Key `zoneinfo` Rules — paragraph 5: "Some minimal container images also lack it."
[^11]: graphlib — Functionality for topological sorting. https://docs.python.org/3.9/library/graphlib.html
    - New in 3.9: `graphlib` — Topological Sorting — paragraph 1: "…powerful because it supports **parallel execution** via an incremental API."
[^12]: concurrent.futures — Executor.shutdown(cancel_futures=...). https://docs.python.org/3.9/library/concurrent.futures.html
    - Concurrency › `concurrent.futures` — New in 3.9: `cancel_futures` — paragraph 1: "…all pending (not-yet-started) futures immediately instead of waiting for them:"
[^13]: bpo-43727 — futures cancelled by shutdown(cancel_futures=True) are not yielded by as_completed()/wait(); unresolved. https://bugs.python.org/issue43727
    - Concurrency › `concurrent.futures` — New in 3.9: `cancel_futures` — paragraph 2: "…never wakes up (bpo-43727 — still unfixed as of 2026)."
