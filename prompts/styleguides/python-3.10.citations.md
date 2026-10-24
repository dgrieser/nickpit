### Citations: Python 3.10: The Complete Production Developer Guide

Sources for `python-3.10.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: What's New In Python 3.10. Python Software Foundation. https://docs.python.org/3/whatsnew/3.10.html
    - Overview — paragraph 1: "…(April 2023); everything after that (3.10.12–3.10.22) was source-only security releases."
    - Overview — paragraph 2: "…API — the two removals that break most 3.9 code."
    - New in 3.10: Parenthesized Context Managers — paragraph 1: "…backslashes, no `ExitStack` needed for a fixed set of managers."
    - New in 3.10: Better Error Messages — paragraph 1: "…tests that assert on exact `SyntaxError`/`NameError` messages may need updating."
    - Primitive Types and Numerics › Deprecated in 3.10: Numeric Literals Glued to Keywords — paragraph 1: "…a module fails to import in a strict test run:"
    - Lists › New in 3.10: `bisect` with `key=` — paragraph 1: "…accept a `key` function — no more parallel key lists."
    - Classes and OOP › Class Definition — paragraph 1: "…now directly callable (e.g. from a class-body dict of handlers)."
    - `async`/`await` and `asyncio` › Removed in 3.10: the `loop=` Parameter — paragraph 1: "…`Condition()`, `Semaphore()`, `BoundedSemaphore()` — is **removed**; passing it raises `TypeError`."
    - Concurrency › `threading` — Deprecated camelCase Aliases (3.10) — paragraph 1: "`currentThread()`, `activeCount()`, `Condition.notifyAll()`, `Event.isSet()`, `Thread.setName()/getName()`, `Thread.isDaemon()/setDaemon()` emit `DeprecationWarning` in 3.10."
    - Security › New in 3.10: OpenSSL 1.1.1+ Required and Stricter `ssl` Defaults (PEP 644) — paragraph 2: "The `ssl` module also got secure-by-default settings:"
    - Security › Security Fixes in 3.10 and Its Point Releases — paragraph 1: "What's New 3.10 and the 3.10 changelog."
    - Performance — paragraph 1: "…3.11, with the specializing adaptive interpreter). 3.10's gains are targeted:"
    - Removed and Changed in 3.10 — Migration Notes — paragraph 1: "**Removed in 3.10 (code that worked on 3.9 breaks):**"
[^2]: PEP 619 – Python 3.10 Release Schedule (3.10.22 final release, end-of-life 2026-10-01). https://peps.python.org/pep-0619/ ; release list: https://www.python.org/downloads/
    - Overview — paragraph 1: "…(April 2023); everything after that (3.10.12–3.10.22) was source-only security releases."
[^3]: Python 3.7 What's New — dict insertion order is part of the language spec. https://docs.python.org/3/whatsnew/3.7.html
    - Overview — bullet 11: "**Dicts are ordered by insertion** (language guarantee since 3.7)."
[^4]: PEP 634 – Structural Pattern Matching: Specification. https://peps.python.org/pep-0634/
    - Overview — bullet 12: "`match = re.match(...)` and a variable named `case` remain valid."
    - New in 3.10: Structural Pattern Matching (PEP 634, 635, 636) — paragraph 1: "…nothing — there is no implicit error and no fall-through."
[^5]: `__future__` — Future statement definitions (annotations: mandatory release "TBD", postponed twice by the Steering Council). https://docs.python.org/3.10/library/__future__.html
    - Overview — bullet 13: "…was postponed, and eventually superseded by PEP 649 in 3.14)."
    - Variable Annotations and Type Hints › Postponed Evaluation (`from __future__ import annotations`) in 3.10 — paragraph 1: "In 3.10 the future import still changes behavior **per module**:"
[^6]: PEP 597 – Add optional EncodingWarning. https://peps.python.org/pep-0597/ ; PEP 686 – Make UTF-8 mode default (Python 3.15). https://peps.python.org/pep-0686/
    - Overview — bullet 14: "UTF-8 mode only becomes the default in Python 3.15."
    - File I/O and `pathlib` › New in 3.10: `EncodingWarning` and `encoding="locale"` (PEP 597) — paragraph 1: "…3.10 adds an **opt-in** `EncodingWarning` to find every such call."
[^7]: The `match` statement — Python 3.10 Language Reference. https://docs.python.org/3.10/reference/compound_stmts.html#the-match-statement
    - New in 3.10: Structural Pattern Matching (PEP 634, 635, 636) — paragraph 1: "…nothing — there is no implicit error and no fall-through."
    - New in 3.10: Structural Pattern Matching (PEP 634, 635, 636) › Destructuring Sequences and Mappings — paragraph 1: "Rules worth knowing for review:"
[^8]: PEP 636 – Structural Pattern Matching: Tutorial. https://peps.python.org/pep-0636/ ; PEP 635 – Motivation and Rationale. https://peps.python.org/pep-0635/
    - New in 3.10: Structural Pattern Matching (PEP 634, 635, 636) — paragraph 1: "…nothing — there is no implicit error and no fall-through."
[^9]: PEP 604 – Allow writing union types as X | Y. https://peps.python.org/pep-0604/ ; Union Type — Built-in Types. https://docs.python.org/3.10/library/stdtypes.html#types-union
    - New in 3.10: Union Types as `X | Y` (PEP 604) — paragraph 1: "…`isinstance()` also accepts `typing.Union[int, str]`; on 3.9 that raised `TypeError`)."
[^10]: PEP 618 – Add Optional Length-Checking To zip. https://peps.python.org/pep-0618/
    - New in 3.10: `zip(strict=True)` (PEP 618) — paragraph 1: "`strict=True` raises `ValueError` when the iterables have different lengths."
[^11]: Integer string conversion length limitation (3.10.7, CVE-2020-10735). https://docs.python.org/3.10/library/stdtypes.html#int-max-str-digits
    - Primitive Types and Numerics › Security Change in 3.10.7: Integer/String Conversion Limit — paragraph 1: "`hex()`, `oct()`, `bin()` and `int(s, 16)` are not limited."
[^12]: PEP 612 – Parameter Specification Variables. https://peps.python.org/pep-0612/
    - Variable Annotations and Type Hints › New in 3.10: `ParamSpec` and `Concatenate` (PEP 612) — paragraph 1: "`ParamSpec` captures the full parameter list."
[^13]: Python 3.10 changelog (3.10.3: bpo-44791, bpo-46581 ParamSpec/Concatenate substitution fixes). https://docs.python.org/3.10/whatsnew/changelog.html
    - Variable Annotations and Type Hints › New in 3.10: `ParamSpec` and `Concatenate` (PEP 612) — bullet 4: "…(bpo-44791, bpo-46581) — another reason to run the latest patch."
[^14]: PEP 647 – User-Defined Type Guards. https://peps.python.org/pep-0647/ ; typing.TypeGuard documentation. https://docs.python.org/3.10/library/typing.html#typing.TypeGuard
    - Variable Annotations and Type Hints › New in 3.10: User-Defined Type Guards — `TypeGuard` (PEP 647) — paragraph 1: "Semantics to keep in mind:"
[^15]: PEP 563 – Postponed Evaluation of Annotations (Superseded by PEP 649 and PEP 749). https://peps.python.org/pep-0563/
    - Variable Annotations and Type Hints › Postponed Evaluation (`from __future__ import annotations`) in 3.10 — paragraph 1: "In 3.10 the future import still changes behavior **per module**:"
[^16]: Annotations Best Practices (HOWTO, new in 3.10). https://docs.python.org/3.10/howto/annotations.html ; inspect.get_annotations. https://docs.python.org/3.10/library/inspect.html#inspect.get_annotations
    - Variable Annotations and Type Hints › New in 3.10: `inspect.get_annotations()` — Reading Annotations Safely — paragraph 1: "Never read `cls.__dict__['__annotations__']` or `cls.__annotations__` directly in framework code."
[^17]: collections.Counter.total() (new in 3.10). https://docs.python.org/3.10/library/collections.html#collections.Counter.total
    - Dictionaries › `collections` — paragraph 1: "`Counter.total()` replaces `sum(c.values())` (new in 3.10)."
[^18]: dataclasses — Data Classes (3.10: slots, kw_only, KW_ONLY, match_args). https://docs.python.org/3.10/library/dataclasses.html ; gh-90562 zero-argument super() with slots=True. https://github.com/python/cpython/issues/90562
    - Dataclasses › New in 3.10: `slots=True`, `kw_only=True`, `KW_ONLY`, `match_args` — bullet 3: "Consequences on 3.10:"
[^19]: What's New In Python 3.11 — enum changes (`Enum.__format__`, `IntEnum.__str__`), `asyncio.TimeoutError` alias, new typing features. https://docs.python.org/3/whatsnew/3.11.html
    - Classes and OOP › Enums — Behavior That Changes in 3.11 — paragraph 1: "…for `(str, Enum)` mixins (and changed `IntEnum.__str__` to the value)."
[^20]: io — Text Encoding / opt-in EncodingWarning (3.10). https://docs.python.org/3.10/library/io.html#io-text-encoding
    - File I/O and `pathlib` › New in 3.10: `EncodingWarning` and `encoding="locale"` (PEP 597) — paragraph 1: "…3.10 adds an **opt-in** `EncodingWarning` to find every such call."
[^21]: os.path.realpath() — "Changed in version 3.10.18: The ALLOW_MISSING value for the strict parameter was added" (tarfile fixes: gh-135034 in the 3.10.18 changelog). https://docs.python.org/3.10/library/os.path.html#os.path.realpath ; https://docs.python.org/3.10/whatsnew/changelog.html
    - File I/O and `pathlib` › Path Traversal Safety — paragraph 1: "…`FileNotFoundError`) — added as part of the tarfile CVE-2025-4517 fixes."
[^22]: asyncio.get_event_loop() — Python 3.10 documentation (deprecation note for 3.10.0–3.10.8 and 3.10.9). https://docs.python.org/3.10/library/asyncio-eventloop.html#asyncio.get_event_loop
    - `async`/`await` and `asyncio` › `get_event_loop()` — the Patch-Level Story — paragraph 1: "…in 3.10, and its behavior **changed across 3.10 patch releases**:"
[^23]: Python 3.10 changelog (gh-93453 in 3.10.9; gh-100160 in 3.10.10 removing the get_event_loop() warnings; security fixes 3.10.19–3.10.22). https://docs.python.org/3.10/whatsnew/changelog.html
    - `async`/`await` and `asyncio` › `get_event_loop()` — the Patch-Level Story — paragraph 1: "…in 3.10, and its behavior **changed across 3.10 patch releases**:"
    - Security › Security Fixes in 3.10 and Its Point Releases — paragraph 1: "What's New 3.10 and the 3.10 changelog."
[^24]: bpo-43727 — futures cancelled by shutdown(cancel_futures=True) are not yielded by as_completed()/wait(). https://bugs.python.org/issue43727
    - Concurrency › `concurrent.futures` — paragraph 1: "…`as_completed()`/`wait()` hang on drained futures (bpo-43727) — filter `f.cancelled()` first."
[^25]: bpo-37324 — Remove ABC aliases from collections (3.10). https://bugs.python.org/issue37324
    - `collections.abc` — Aliases Removed in 3.10 — paragraph 1: "…is the most common import-time failure when upgrading from 3.9:"
[^26]: PEP 626 – Precise line numbers for debugging and other tools. https://peps.python.org/pep-0626/
    - New in 3.10: Precise Line Numbers (PEP 626) — paragraph 1: "`frame.f_lineno` is always correct."
[^27]: PEP 644 – Require OpenSSL 1.1.1 or newer. https://peps.python.org/pep-0644/
    - Security › New in 3.10: OpenSSL 1.1.1+ Required and Stricter `ssl` Defaults (PEP 644) — paragraph 1: "…reports that the ssl module in Python is not available)."
[^28]: ssl — TLS/SSL wrapper for socket objects (Python 3.10). https://docs.python.org/3.10/library/ssl.html
    - Security › New in 3.10: OpenSSL 1.1.1+ Required and Stricter `ssl` Defaults (PEP 644) — paragraph 2: "The `ssl` module also got secure-by-default settings:"
[^29]: tarfile — Extraction filters (added in 3.10.12, PEP 706). https://docs.python.org/3.10/library/tarfile.html#tarfile-extraction-filter ; PEP 706. https://peps.python.org/pep-0706/
    - Security › `tarfile` Extraction — Always Pass `filter=` (3.10.12+) — paragraph 1: "…the code might run on a patch older than 3.10.12."
[^30]: PEP 632 – Deprecate distutils module. https://peps.python.org/pep-0632/
    - Virtual Environments and Packaging › Deprecated in 3.10: `distutils` (PEP 632) — paragraph 1: "…`DeprecationWarning` on import in 3.10 and is **removed in 3.12**."
[^31]: Python 3.11 changelog (gh-87999: numeric literal followed by a keyword now emits SyntaxWarning instead of DeprecationWarning). https://docs.python.org/3.11/whatsnew/changelog.html
    - Primitive Types and Numerics › Deprecated in 3.10: Numeric Literals Glued to Keywords — paragraph 1: "…a module fails to import in a strict test run:"
