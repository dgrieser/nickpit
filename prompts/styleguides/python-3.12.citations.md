### Citations: Python 3.12: The Complete Production Developer Guide

Sources for `python-3.12.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: What's New In Python 3.12. Python Software Foundation. https://docs.python.org/3.12/whatsnew/3.12.html
    - Overview — paragraph 1: "Python 3.12 was released on October 2, 2023."
    - Overview — paragraph 2: "The landmark changes of 3.12:"
    - Overview — bullet 22: "Use raw strings for regexes and Windows paths."
    - New in 3.12: f-strings in the Grammar (PEP 701) — bullet 3: "Custom tokenizer-based tools (linters, code generators) must be updated."
    - New in 3.12: Improved Error Messages — paragraph 1: "Recognize them in logs and bug reports:"
    - New in 3.12: The Buffer Protocol from Python (PEP 688) — bullet 1: "In annotations use `Buffer` (or `bytes | bytearray | memoryview`)."
    - New in 3.12: `sys.monitoring` — Low-Impact Monitoring (PEP 669) — bullet 3: "…`threading.setprofile_all_threads()` to install a trace/profile function in all running threads."
    - New in 3.12: Interpreter Internals That Affect Production › Per-Interpreter GIL (PEP 684) — C API Only — paragraph 1: "…in via multi-phase init to be loaded in such interpreters."
    - New in 3.12: Interpreter Internals That Affect Production › Recursion and the Garbage Collector — bullet 1: "…recursion in pure Python still raises `RecursionError` (default limit 1000)."
    - New in 3.12: Interpreter Internals That Affect Production › Recursion and the Garbage Collector — bullet 2: "…C extensions get a chance to run GC via `PyErr_CheckSignals()`."
    - Primitive Types and Numerics › Floats — New in 3.12: More Accurate `sum()` — paragraph 1: "Golden-file tests comparing exact float output may need updating."
    - Primitive Types and Numerics › Booleans — `~bool` Deprecated — paragraph 1: "…the author meant, and it becomes an error in 3.16."
    - Strings › New in 3.12: Invalid Escape Sequences Are a `SyntaxWarning` — paragraph 1: "Octal escapes above `\377` get the same treatment."
    - Type Hints in 3.12 › What Each `typing` Name Is For in 3.12 — paragraph 1: "`typing.Text` is deprecated (use `str`)."
    - Type Hints in 3.12 › Protocols — 3.12 Runtime-Check Changes — paragraph 1: "Some objects may change classification:"
    - Functions › `functools` — `cached_property` Lost Its Lock in 3.12 — paragraph 1: "…the same instance, **the getter can run more than once**."
    - Functions › Generators and `itertools` — paragraph 1: "`copy`/`deepcopy`/`pickle` support for `itertools` objects."
    - Classes and OOP › `__set_name__` Errors Are No Longer Wrapped (3.12) — paragraph 1: "`except RuntimeError` around class creation no longer catches them."
    - Exception Handling › Exception Groups and `except*` (3.11+) — paragraph 1: "…exception, that exception is no longer wrapped in an `ExceptionGroup`:"
    - Exception Handling › Other 3.12 Exception-Related Changes — bullet 1: "`sys.last_type`, `sys.last_value` and `sys.last_traceback` are deprecated."
    - File I/O and `pathlib` › Other `pathlib` / `os.path` Additions in 3.12 — bullet 2: "…`bytes` or `os.PathLike` work (`TypeError` in 3.12, a `DeprecationWarning` before)."
    - File I/O and `pathlib` › Temporary Files and `shutil` — paragraph 1: "Older 3.12.x releases ignored `mode` on Windows."
    - `async`/`await` and `asyncio` › Structured Concurrency Baseline (3.11+) — bullet 4: "…(`asyncio.TimeoutError` has been an alias of the built-in since 3.11)."
    - `async`/`await` and `asyncio` › New in 3.12: `asyncio.run(loop_factory=...)` and Other Changes — bullet 1: "Use `asyncio.run()` at entry points and `asyncio.get_running_loop()` inside coroutines."
    - `async`/`await` and `asyncio` › New in 3.12: `asyncio.run(loop_factory=...)` and Other Changes — bullet 2: "Delete manual watcher setup."
    - `async`/`await` and `asyncio` › New in 3.12: `asyncio.run(loop_factory=...)` and Other Changes — bullet 5: "Some benchmarks run up to 75% faster."
    - Concurrency › `os.fork()` in Multithreaded Processes Now Warns — paragraph 1: "…locks held by other threads (logging handlers, allocator locks, …)."
    - Concurrency › `os.fork()` in Multithreaded Processes Now Warns — bullet 1: "…which require picklable arguments and `if __name__ == '__main__':` guards."
    - Concurrency › `concurrent.futures` and `threading` — bullet 1: "Use `daemon`, `name`, `is_set()`, `notify_all()`, `current_thread()` and `active_count()`."
    - Security › Security Fixes in 3.12 Point Releases — paragraph 1: "…that has them (from the official changelog and "What's New"):"
    - Security › Removed Insecure APIs (3.12) — bullet 3: "Pass an `ssl.SSLContext` via `context=` (`ssl_context=` in `imaplib`)."
    - Security › Removed Insecure APIs (3.12) — paragraph 1: "The pure-Python `pbkdf2_hmac` fallback was removed (OpenSSL's is always used)."
    - Regular Expressions — paragraph 1: "…and replacements may only contain ASCII letters, digits and underscores."
    - Performance — paragraph 1: "What 3.12 gives you for free:"
    - Testing › Removed in 3.12: Deprecated `unittest` Aliases — paragraph 1: "Old test suites fail with `AttributeError` on 3.12:"
    - Virtual Environments and Packaging › Removed in 3.12: `distutils`; `venv` No Longer Installs `setuptools` — bullet 2: "…build environments, so `pip install` of setuptools-based sdists keeps working."
    - Removed and Changed in 3.12 — Migration Notes › Deprecated in 3.12 — Removed in 3.13 — paragraph 1: "…**gone** in 3.13 (PEP 594 and others), so migrate now:"
[^2]: PEP 693 – Python 3.12 Release Schedule (release dates, security-only phase, end of life October 2028). https://peps.python.org/pep-0693/
    - Overview — paragraph 1: "…**October 2028** (five years after 3.12.0), per the release-schedule PEP."
    - Overview — paragraph 1: "…3.12.13 (March 2026) and 3.12.14 (August 2026) came before it."
[^3]: PEP 683 – Immortal Objects, Using a Fixed Refcount. https://peps.python.org/pep-0683/
    - Overview — bullet 19: "…are *immortal*, and their reference count never changes (PEP 683)."
    - New in 3.12: Interpreter Internals That Affect Production › Immortal Objects (PEP 683) — paragraph 1: "…count is fixed and never changes, which avoids cache-line contention."
[^4]: PEP 695 – Type Parameter Syntax. https://peps.python.org/pep-0695/
    - New in 3.12: Type Parameter Syntax (PEP 695) — paragraph 1: "…declare module-level `TypeVar`s, and variance is inferred instead of declared."
    - New in 3.12: Type Parameter Syntax (PEP 695) › The `type` Statement and Lazy Evaluation — paragraph 1: "…names defined later, including themselves (recursive types), without string quotes:"
    - New in 3.12: Type Parameter Syntax (PEP 695) › Variance Inference — paragraph 1: "…so the `T_co`/`T_contra` naming conventions are obsolete in 3.12 code:"
[^5]: typing — Generics and type parameter lists (Python 3.12). https://docs.python.org/3.12/library/typing.html#generics
    - New in 3.12: Type Parameter Syntax (PEP 695) — paragraph 1: "…declare module-level `TypeVar`s, and variance is inferred instead of declared."
[^6]: Python 3.12 Language Reference — Type parameter lists, the `type` statement, annotation scopes and lazy evaluation. https://docs.python.org/3.12/reference/compound_stmts.html#type-params and https://docs.python.org/3.12/reference/executionmodel.html#annotation-scopes
    - New in 3.12: Type Parameter Syntax (PEP 695) › The `type` Statement and Lazy Evaluation — paragraph 1: "…names defined later, including themselves (recursive types), without string quotes:"
    - New in 3.12: Type Parameter Syntax (PEP 695) › Scoping: Annotation Scopes — paragraph 1: "…in the enclosing module, and they are not class attributes:"
[^7]: mypy changelog — Mypy 1.12: "Enable new type parameter syntax by default". https://mypy.readthedocs.io/en/stable/changelog.html
    - New in 3.12: Type Parameter Syntax (PEP 695) › Pitfalls and Compatibility — bullet 5: "…by default since **mypy 1.12** (it was experimental in 1.11)."
[^8]: Ruff rules non-pep695-type-alias (UP040), non-pep695-generic-class (UP046), non-pep695-generic-function (UP047). https://docs.astral.sh/ruff/rules/non-pep695-type-alias/ , https://docs.astral.sh/ruff/rules/non-pep695-generic-class/ , https://docs.astral.sh/ruff/rules/non-pep695-generic-function/
    - New in 3.12: Type Parameter Syntax (PEP 695) › Pitfalls and Compatibility — bullet 5: "…rewrite old-style code to PEP 695 when `target-version = "py312"`."
    - Code Quality Tools — bullet 1: "…alias is used at runtime (see the PEP 695 section)."
[^9]: typing — Support for type hints (Python 3.12): `TypeAlias` deprecated in favor of the `type` statement; deprecated aliases. https://docs.python.org/3.12/library/typing.html#typing.TypeAlias
    - New in 3.12: Type Parameter Syntax (PEP 695) › Pitfalls and Compatibility — bullet 6: "Keep it only for code that must run on 3.11."
    - Type Hints in 3.12 › What Each `typing` Name Is For in 3.12 — paragraph 1: "`typing.Text` is deprecated (use `str`)."
[^10]: PEP 698 – Override Decorator for Static Typing. https://peps.python.org/pep-0698/
    - New in 3.12: `@typing.override` (PEP 698) — paragraph 1: "…reports an error instead of silently leaving a dead "override"."
    - New in 3.12: `@typing.override` (PEP 698) — bullet 2: "…`def`, below) descriptor decorators such as `@property`, `@classmethod` and `@staticmethod`:"
[^11]: PEP 692 – Using TypedDict for more precise **kwargs typing. https://peps.python.org/pep-0692/
    - New in 3.12: Precise `**kwargs` Typing with `Unpack[TypedDict]` (PEP 692) — paragraph 1: "…since 3.11 (for `TypeVarTuple`). 3.12 standardizes its use on `**kwargs`."
[^12]: PEP 701 – Syntactic formalization of f-strings. https://peps.python.org/pep-0701/
    - New in 3.12: f-strings in the Grammar (PEP 701) — paragraph 1: "The old restrictions are gone:"
[^13]: PEP 709 – Inlined comprehensions. https://peps.python.org/pep-0709/
    - New in 3.12: Comprehension Inlining (PEP 709) — paragraph 1: "Generator expressions are **not** inlined."
[^14]: PEP 688 – Making the buffer protocol accessible in Python. https://peps.python.org/pep-0688/
    - New in 3.12: The Buffer Protocol from Python (PEP 688) — paragraph 1: "`collections.abc.Buffer` is the standard annotation for "anything `memoryview()` accepts"."
[^15]: PEP 669 – Low Impact Monitoring for CPython; `sys.monitoring` documentation. https://peps.python.org/pep-0669/ and https://docs.python.org/3.12/library/sys.monitoring.html
    - New in 3.12: `sys.monitoring` — Low-Impact Monitoring (PEP 669) — paragraph 1: "…gives near-zero overhead once the tool has seen a location."
[^16]: PEP 684 – A Per-Interpreter GIL. https://peps.python.org/pep-0684/
    - New in 3.12: Interpreter Internals That Affect Production › Per-Interpreter GIL (PEP 684) — C API Only — paragraph 1: "…in via multi-phase init to be loaded in such interpreters."
[^17]: Python support for the Linux perf profiler (Python 3.12 HOWTO). https://docs.python.org/3.12/howto/perf_profiling.html
    - New in 3.12: Interpreter Internals That Affect Production › Linux `perf` Profiler Support — paragraph 1: "…`perf` profiler shows **Python function names** in native stack traces."
[^18]: Integer string conversion length limitation (CVE-2020-10735). https://docs.python.org/3.12/library/stdtypes.html#int-max-str-digits
    - Primitive Types and Numerics › Integers — paragraph 1: "…by default**, a guard against quadratic-time denial of service (CVE-2020-10735):"
[^19]: itertools.batched (Python 3.12). https://docs.python.org/3.12/library/itertools.html#itertools.batched
    - Lists, Tuples and Sequences › New in 3.12: `itertools.batched()` — paragraph 1: "…replaces the hand-rolled "chunks" helper found in nearly every codebase:"
[^20]: enum — `EnumType.__contains__` ("Changed in version 3.12"). https://docs.python.org/3.12/library/enum.html#enum.EnumType.__contains__
    - Classes and OOP › Enums — bullet 1: "Code that relied on the `TypeError` must be updated."
[^21]: datetime — `datetime.utcnow()` / `utcfromtimestamp()` deprecated in 3.12. https://docs.python.org/3.12/library/datetime.html#datetime.datetime.utcnow
    - Dates and Times › New in 3.12: `datetime.utcnow()` and `utcfromtimestamp()` Are Deprecated — paragraph 1: "…is planned for a future version without a fixed date."
[^22]: pathlib (Python 3.12): `Path.walk()`, subclassing and `with_segments()`, `relative_to(walk_up=)`. https://docs.python.org/3.12/library/pathlib.html
    - File I/O and `pathlib` › New in 3.12: `Path.walk()` — paragraph 1: "Path, dirnames: list[str], filenames: list[str])`:"
    - File I/O and `pathlib` › New in 3.12: Subclassing `pathlib.Path` — paragraph 1: "…through derived paths (`/`, `.parent`, `.with_name()`, `iterdir()`, …), override `with_segments()`:"
[^23]: asyncio — Eager Task Factory (Python 3.12). https://docs.python.org/3.12/library/asyncio-task.html#eager-task-factory
    - `async`/`await` and `asyncio` › New in 3.12: Eager Task Execution — paragraph 1: "…loop at all, which is 2–5x faster for some workloads."
[^24]: Python 3.12 changelog (all 3.12.x releases, including the Security sections). https://docs.python.org/3.12/whatsnew/changelog.html
    - `async`/`await` and `asyncio` › New in 3.12: Eager Task Execution — bullet 4: "…`TaskGroup` (gh-128550), and created reference cycles for eager tasks (gh-128552)."
    - Security › New in 3.12: Tar Extraction Filters (PEP 706) — Always Pass `filter=` — bullet 2: "…filters had several bypasses that were fixed in point releases:"
    - Security › Security Fixes in 3.12 Point Releases — paragraph 1: "…that has them (from the official changelog and "What's New"):"
[^25]: logging.config — configuring QueueHandler and QueueListener (Python 3.12). https://docs.python.org/3.12/library/logging.config.html#configuring-queuehandler-and-queuelistener
    - Logging — paragraph 1: "This setup keeps slow handlers (network, files) off request threads:"
[^26]: PEP 706 – Filter for tarfile.extractall. https://peps.python.org/pep-0706/
    - Security › New in 3.12: Tar Extraction Filters (PEP 706) — Always Pass `filter=` — paragraph 1: "…take a `filter=` argument that limits what extraction may do:"
[^27]: tarfile — Extraction filters (Python 3.12). https://docs.python.org/3.12/library/tarfile.html#tarfile-extraction-filter
    - Security › New in 3.12: Tar Extraction Filters (PEP 706) — Always Pass `filter=` — paragraph 1: "…take a `filter=` argument that limits what extraction may do:"
[^28]: sqlite3 — Transaction control via the autocommit attribute (Python 3.12). https://docs.python.org/3.12/library/sqlite3.html#sqlite3-transaction-control-autocommit
    - `sqlite3` › New in 3.12: `autocommit` — PEP 249-Compliant Transactions — paragraph 1: "The default is `sqlite3.LEGACY_TRANSACTION_CONTROL`, which keeps the old `isolation_level`-based behavior:"
[^29]: PEP 632 – Deprecate distutils module (migration advice). https://peps.python.org/pep-0632/
    - Virtual Environments and Packaging › Removed in 3.12: `distutils`; `venv` No Longer Installs `setuptools` — bullet 1: "…→ `shutil.which`, `distutils.dir_util.copy_tree` → `shutil.copytree(..., dirs_exist_ok=True)`, and `distutils.sysconfig` → `sysconfig`."
[^30]: PEP 594 – Removing dead batteries from the standard library; What's New In Python 3.13 (Removed). https://peps.python.org/pep-0594/ and https://docs.python.org/3.13/whatsnew/3.13.html
    - Removed and Changed in 3.12 — Migration Notes › Deprecated in 3.12 — Removed in 3.13 — paragraph 1: "…**gone** in 3.13 (PEP 594 and others), so migrate now:"
[^31]: dataclasses — `slots=True` and zero-argument `super()` warning (gh-90562), Python 3.12. https://docs.python.org/3.12/library/dataclasses.html#dataclasses.dataclass
    - Dataclasses — paragraph 2: "…`dataclasses` documentation lists this limitation and the two-argument workaround (gh-90562)."
