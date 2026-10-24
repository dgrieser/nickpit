### Citations: Python 3.14: The Complete Production Developer Guide

Sources for `python-3.14.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: What's New in Python 3.14. Python Software Foundation. https://docs.python.org/3.14/whatsnew/3.14.html
    - Overview — paragraph 1: "Python 3.14 was released on October 7, 2025."
    - Overview — paragraph 2: "…APIs is gone (`ast.Num`, asyncio child watchers, `pkgutil.find_loader`, `urllib.request.URLopener`, ...)."
    - Overview — bullet 5: "…an *incremental* collector; 3.14.5 reverted to the 3.13 generational collector.)"
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › The Three Formats (`annotationlib.Format`) — paragraph 3: "That is what `dataclasses` itself does in 3.14."
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › `from __future__ import annotations` in 3.14 — paragraph 1: "…in 2029, and removed at least two releases after that."
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › Typing Annotations: What Changed at Runtime — paragraph 1: "Use `typing.get_origin()`/`get_args()`."
    - New in 3.14: `return`/`break`/`continue` Leaving a `finally` Block (PEP 765) — bullet 2: "…ignore::SyntaxWarning` / `PYTHONWARNINGS=ignore::SyntaxWarning` — but fix your own code instead."
    - Primitive Types and Numerics › Integers — paragraph 2: "…(gh-119743): classes that want `int(obj)` must implement `__int__()` or `__index__()`."
    - Primitive Types and Numerics › Integers — paragraph 4: "Three-argument `pow()` now also tries `__rpow__()` (gh-130104)."
    - Primitive Types and Numerics › Floats — paragraph 1: "…2)`) emits `DeprecationWarning`; pass a single complex positional argument instead."
    - Primitive Types and Numerics › Booleans — paragraph 1: "…since 3.12 (removal pending in 3.16) — use `not x`."
    - Type Hints in 3.14 › New in 3.14: Typing Details — bullet 2: "…reprs as `int | str` and is no longer cached."
    - Type Hints in 3.14 › New in 3.14: Typing Details — bullet 4: "…new minimal protocols — prefer them to `typing.IO`/`TextIO`/`BinaryIO` for parameters."
    - Type Hints in 3.14 › New in 3.14: Typing Details — paragraph 1: "…3.17 — use `collections.abc.Buffer` or `bytes | bytearray | memoryview`."
    - Lists, Tuples and Iteration — paragraph 2: "…that pickled `itertools.count()`/`chain()` objects (e.g., checkpointing generators) now gets `TypeError`."
    - Dictionaries and `collections` › Sets — paragraph 1: "Do not match on the old message text in tests."
    - Control Flow — paragraph 2: "…`async with` on a synchronous context manager and vice versa."
    - Functions › `functools` — New in 3.14: `Placeholder`, and `partial` Is Now a Method Descriptor — paragraph 1: "…through an instance changes meaning — wrap it in `staticmethod()`."
    - Functions › Generators — paragraph 1: "`gen.throw(ValueError('x'))`."
    - Exception Handling › Exception Groups and `except*` (3.11+) — bullet 4: "…`AttributeError`, `ValueError`, etc. — update `except` clauses around `pickle.dumps()` accordingly."
    - File I/O and `pathlib` › Behavior Changes in `pathlib` (3.14) — bullet 3: "…to `relative_to()`/`is_relative_to()` (they used to be joined onto the first)."
    - File I/O and `pathlib` › Behavior Changes in `pathlib` (3.14) — bullet 5: "…now opens files unbuffered — 9–17% faster for whole-file reads."
    - File I/O and `pathlib` › Path Traversal Checks — paragraph 1: "…useful for validating a path you are about to create."
    - File I/O and `pathlib` › Temporary Files and the Environment — paragraph 1: "…access; call it at startup or in single-threaded sections only."
    - File I/O and `pathlib` › Temporary Files and the Environment — paragraph 1: "…are **soft-deprecated** in 3.14 — use `subprocess` in new code."
    - `async`/`await` and `asyncio` › New in 3.14: `asyncio.get_event_loop()` No Longer Creates a Loop — paragraph 2: "`async def main(): await start_server(); await asyncio.Event().wait()` and `asyncio.run(main())`."
    - `async`/`await` and `asyncio` › New in 3.14: Policy System Deprecated (Removal in 3.16) — paragraph 1: "Choose the loop implementation with `loop_factory`:"
    - `async`/`await` and `asyncio` › New in 3.14: Policy System Deprecated (Removal in 3.16) — paragraph 2: "`asyncio.iscoroutinefunction()` — use `inspect.iscoroutinefunction()`."
    - `async`/`await` and `asyncio` › Structured Concurrency (3.11+) — paragraph 1: "Custom task factories must accept `**kwargs`."
    - `async`/`await` and `asyncio` › New in 3.14: Asyncio Introspection — paragraph 1: "Inspect a **running process** without modifying it:"
    - `async`/`await` and `asyncio` › New in 3.14: Asyncio Introspection — paragraph 3: "…at the prompt, and `$_asynctask` refers to the current task."
    - `async`/`await` and `asyncio` › New in 3.14: Asyncio Introspection — paragraph 5: "…free-threaded build (several event loops in different threads scale linearly)."
    - Concurrency › The GIL, and the Officially Supported Free-Threaded Build (PEP 779) — paragraph 1: "…because the specializing adaptive interpreter is now enabled in it."
    - Concurrency › The GIL, and the Officially Supported Free-Threaded Build (PEP 779) — bullet 6: "The experimental JIT is not available in free-threaded builds."
    - Concurrency › `concurrent.futures` and `multiprocessing` — New Default Start Method — paragraph 1: "…`'forkserver'` instead of `'fork'`.** Windows and macOS already used `'spawn'`."
    - Concurrency › `concurrent.futures` and `multiprocessing` — New Default Start Method — paragraph 5: "The `forkserver` control socket is now authenticated (security hardening)."
    - Concurrency › `threading` — paragraph 1: "…to `threading.RLock()` is deprecated (they were ignored; removed in 3.15)."
    - New in 3.14: Debugging and Observability › Safe External Debugger Interface (PEP 768) — bullet 3: "Remote debugging is not enabled in the remote process`."
    - New in 3.14: Debugging and Observability › Other New Debugging Aids — bullet 1: "…and `faulthandler.enable(c_stack=True)` print the **C** stack on crashes (where supported)."
    - New in 3.14: Debugging and Observability › Other New Debugging Aids — bullet 5: "…with `NO_COLOR=1` or `PYTHON_COLORS=0`; disable the new REPL with `PYTHON_BASIC_REPL=1`."
    - Logging — paragraph 1: "`Logger.warn()` remains deprecated — use `warning()`."
    - Security › Other Security-Relevant Changes in 3.14 — bullet 4: "…built-in, formally verified HACL* implementation used when OpenSSL lacks HMAC."
    - JSON and Serialization — paragraph 1: "…errors carry `__notes__` locating the failing element (see Exception Handling)."
    - JSON and Serialization › Pickle — New in 3.14: Default Protocol 5 — paragraph 1: "…exception chained, and errors include notes locating the unpicklable item."
    - Regular Expressions — bullet 3: "…are deprecated and **removed in 3.15** — import only `re`."
    - Memory Management and Garbage Collection › The 3.14 GC Story: Incremental, Then Reverted (3.14.5) — bullet 2: "The free-threaded build's collector was not affected."
    - Memory Management and Garbage Collection › Practical Memory Tools — bullet 1: "…not use `Py_REFCNT(op) == 1` to detect temporaries (use `PyUnstable_Object_IsUniqueReferencedTemporary()`)."
    - Performance — paragraph 1: "…changes for performance (measure before relying on any of it):"
    - Virtual Environments and Packaging › `pyproject.toml` — bullet 3: "…static description of the build for launchers and cross-compilation tools."
    - Removed and Changed in 3.14 — Migration Notes › Removed in 3.14 — paragraph 1: "…of these fails on 3.14 with `AttributeError`, `ImportError` or `TypeError`:"
    - Removed and Changed in 3.14 — Migration Notes › Deprecated in 3.14 — Fix Before 3.15/3.16 — paragraph 2: "`array` `'u'` format code (use `'w'`)."
    - Removed and Changed in 3.14 — Migration Notes › Deprecated in 3.14 — Fix Before 3.15/3.16 — paragraph 4: "…numeric literals immediately followed by keywords (`0in x`, `SyntaxWarning`), `datetime.utcnow()`/`utcfromtimestamp()`."
[^2]: PEP 745 – Python 3.14 Release Schedule. https://peps.python.org/pep-0745/
    - Overview — paragraph 1: "…shift) — followed by source-only security fixes until **October 2030**."
[^3]: Python downloads — active releases and release dates (3.14.9 and 3.15.0/3.15.1). https://www.python.org/downloads/
    - Overview — paragraph 1: "…3.14 is now the *previous* stable release, not the newest."
[^4]: What's New in Python 3.13 (and the linked 3.10–3.12 What's New documents). https://docs.python.org/3.14/whatsnew/3.13.html
    - Overview — paragraph 3: "…type-parameter defaults (PEP 696), `queue.Queue.shutdown()` and the new REPL (3.13)."
[^5]: What's New in Python 3.15 (lazy imports PEP 810, frozendict PEP 814, sentinel PEP 661, PEP 798, PEP 686 UTF-8 mode, removals). https://docs.python.org/3.15/whatsnew/3.15.html
    - Overview — paragraph 3: "UTF-8 mode becomes the default only in 3.15 (PEP 686)."
    - PEP 8 Code Style › Import Ordering — paragraph 2: "…and a `__lazy_modules__` list is silently ignored (imports stay eager)."
    - Strings › The Two Types — bullet 3: "…to get an `EncodingWarning` for every `open()` that omits `encoding=`."
    - Regular Expressions — bullet 3: "…are deprecated and **removed in 3.15** — import only `re`."
    - Virtual Environments and Packaging › `pyproject.toml` — bullet 2: "…out of `py_limited_api` for that build) and declare `Py_mod_gil` support."
    - Removed and Changed in 3.14 — Migration Notes › Deprecated in 3.14 — Fix Before 3.15/3.16 — paragraph 1: "…a runtime `DeprecationWarning` — the project's next upgrade will break):"
[^6]: PEP 649 – Deferred Evaluation Of Annotations Using Descriptors. https://peps.python.org/pep-0649/
    - Overview — bullet 4: "Forward references no longer need quotes."
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) — paragraph 1: "…time something reads `__annotations__` (or calls `annotationlib.get_annotations()`, `typing.get_type_hints()`, `inspect.signature()`, ...)."
[^7]: PEP 779 – Criteria for supported status for free-threaded Python. https://peps.python.org/pep-0779/
    - Overview — bullet 6: "A separate, officially supported free-threaded build (`python3.14t`) runs without it."
    - Concurrency › The GIL, and the Officially Supported Free-Threaded Build (PEP 779) — paragraph 1: "…because the specializing adaptive interpreter is now enabled in it."
[^8]: dataclasses — Data Classes (field doc=, make_dataclass decorator=). https://docs.python.org/3.14/library/dataclasses.html
    - PEP 8 Code Style › Docstrings — paragraph 2: "…dataclass fields so `help()` and `pydoc` show them (see Dataclasses)."
    - Dataclasses — paragraph 1: "…on `annotationlib` — fields may use forward references without quotes."
[^9]: annotationlib — Functionality for introspecting annotations. https://docs.python.org/3.14/library/annotationlib.html
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) — paragraph 1: "…time something reads `__annotations__` (or calls `annotationlib.get_annotations()`, `typing.get_type_hints()`, `inspect.signature()`, ...)."
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › The Three Formats (`annotationlib.Format`) — paragraph 1: "`annotationlib` (new module) reads annotations in one of three formats:"
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › The Three Formats (`annotationlib.Format`) — paragraph 2: "…never pass it yourself), `FORWARDREF = 3`, `STRING = 4`."
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › Reading Annotations Correctly (and What Broke) — paragraph 1: "…`annotationlib.get_annotate_from_class_namespace(ns)` + `annotationlib.call_annotate_function(annotate, Format.FORWARDREF)` during class creation, and `annotationlib.get_annotations(cls)` afterwards."
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › Reading Annotations Correctly (and What Broke) — paragraph 3: "Never feed untrusted strings into `ForwardRef`, `__annotations__` or `eval_str=True`."
[^10]: typing — Support for type hints (get_type_hints format=, evaluate_forward_ref, Union changes, NamedTuple super()). https://docs.python.org/3.14/library/typing.html
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › `typing.get_type_hints()`, `inspect.signature()` and `TYPE_CHECKING` Imports — paragraph 2: "…on an *instance* (rather than a class/function/module) now raises `TypeError`."
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › Reading Annotations Correctly (and What Broke) — paragraph 2: "…3.14), which also resolves forward references nested inside the hint."
    - Type Hints in 3.14 › New in 3.14: Typing Details — bullet 1: "`typing.get_type_hints(obj, format=...)` and `typing.evaluate_forward_ref()` (see the annotations section)."
    - Type Hints in 3.14 › New in 3.14: Typing Details — bullet 3: "…`type` alias (3.13 raises `TypeError` when the value is evaluated)."
    - Type Hints in 3.14 › New in 3.14: Typing Details — bullet 5: "…at class creation (it never worked — 3.13 raised `RuntimeError`)."
[^11]: PEP 749 – Implementing PEP 649 (future-import deprecation plan). https://peps.python.org/pep-0749/
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › `from __future__ import annotations` in 3.14 — paragraph 1: "…in 2029, and removed at least two releases after that."
[^12]: Python 3.14 changelog (3.14.1–3.14.9 entries, including Security sections). https://docs.python.org/3.14/whatsnew/changelog.html
    - New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749) › Dataclasses with Forward References — paragraph 1: "Do not pin 3.14.1."
    - Dictionaries and `collections` — paragraph 2: "…the factory ran (gh-142495) — relevant on the free-threaded build."
    - File I/O and `pathlib` › Temporary Files and the Environment — paragraph 2: "…errors; 3.14.9 removes files relative to open directory descriptors (gh-157579)."
    - New in 3.14: The `compression` Package and Zstandard (PEP 784) › Zstandard in `tarfile`, `zipfile` and `shutil` — bullet 3: "…3.14.0–3.14.7 a tiny member could force an unbounded allocation (gh-156002)."
    - `async`/`await` and `asyncio` › New in 3.14: Asyncio Introspection — paragraph 4: "`ssl.SSLContext.wrap_bio()` got the same validation."
    - Concurrency › `concurrent.futures` and `multiprocessing` — New Default Start Method — paragraph 3: "…in 3.14.3) and propagates all `-X` options to child processes."
    - New in 3.14: Debugging and Observability › Safe External Debugger Interface (PEP 768) — bullet 6: "…offset tables read from the target before using them (gh-148178)."
    - Security › New in 3.14: `tarfile` Extracts with the `'data'` Filter by Default (PEP 706) — bullet 4: "…when a link falls back to extracting a member, gh-157265)."
    - Security › Security Fixes in 3.14 Point Releases — paragraph 1: "…— the reason "latest patch release" is a hard requirement:"
    - JSON and Serialization › Pickle — New in 3.14: Default Protocol 5 — paragraph 2: "…`UnpicklingError` for malformed frame layouts it used to accept (gh-154848)."
    - Regular Expressions — bullet 4: "…results and will be forbidden in a future version (gh-140797)."
    - Memory Management and Garbage Collection › The 3.14 GC Story: Incremental, Then Reverted (3.14.5) — bullet 2: "The free-threaded build's collector was not affected."
    - Removed and Changed in 3.14 — Migration Notes › Deprecated in 3.14 — Fix Before 3.15/3.16 — paragraph 5: "…just to silence a warning seen on an early 3.14.x."
[^13]: PEP 750 – Template Strings. https://peps.python.org/pep-0750/
    - New in 3.14: Template String Literals — t-strings (PEP 750) — paragraph 1: "…can escape, parameterize or validate each value before combining them."
[^14]: string.templatelib — Support for template string literals. https://docs.python.org/3.14/library/string.templatelib.html
    - New in 3.14: Template String Literals — t-strings (PEP 750) — paragraph 1: "…can escape, parameterize or validate each value before combining them."
    - New in 3.14: Template String Literals — t-strings (PEP 750) — paragraph 2: "`string.templatelib.convert(value, conversion)` reproduces f-string conversion semantics."
[^15]: PEP 758 – Allow except and except* expressions without brackets. https://peps.python.org/pep-0758/
    - New in 3.14: `except A, B:` Without Parentheses (PEP 758) — paragraph 1: "…without parentheses — **only when there is no `as` clause**."
[^16]: PEP 765 – Disallow return/break/continue that exit a finally block. https://peps.python.org/pep-0765/
    - New in 3.14: `return`/`break`/`continue` Leaving a `finally` Block (PEP 765) — paragraph 1: "…silently swallow any in-flight exception — including `KeyboardInterrupt` and `SystemExit`."
[^17]: heapq — Heap queue algorithm (max-heap functions). https://docs.python.org/3.14/library/heapq.html
    - Lists, Tuples and Iteration › New in 3.14: Max-Heaps in `heapq` — paragraph 1: "…— no more negating keys or using the private `_heapify_max`:"
[^18]: functools — Placeholder, partial. https://docs.python.org/3.14/library/functools.html
    - Functions › `functools` — New in 3.14: `Placeholder`, and `partial` Is Now a Method Descriptor — paragraph 1: "…through an instance changes meaning — wrap it in `staticmethod()`."
[^19]: pathlib — Object-oriented filesystem paths (copy/move, info, query methods). https://docs.python.org/3.14/library/pathlib.html
    - File I/O and `pathlib` › New in 3.14: `Path.copy()`, `copy_into()`, `move()`, `move_into()` — paragraph 1: "…finally copy and move files **and directory trees** without `shutil`:"
    - File I/O and `pathlib` › New in 3.14: `Path.info` — Cached File-Type Queries — paragraph 1: "…never use it to re-check state that may have changed."
    - File I/O and `pathlib` › Behavior Changes in `pathlib` (3.14) — bullet 1: "…the exception to detect permission problems must call `path.stat()` instead."
[^20]: compression.zstd — Compression compatible with the Zstandard format; PEP 784. https://docs.python.org/3.14/library/compression.zstd.html , https://peps.python.org/pep-0784/
    - New in 3.14: The `compression` Package and Zstandard (PEP 784) — paragraph 1: "…sooner than five years after PEP 784's acceptance (April 2025)."
    - New in 3.14: The `compression` Package and Zstandard (PEP 784) › Zstandard in `tarfile`, `zipfile` and `shutil` — bullet 1: "…`try: from compression import zstd` / `except ImportError` in libraries."
[^21]: asyncio Event Loop (get_event_loop RuntimeError; 3.14.8 server_hostname check). https://docs.python.org/3.14/library/asyncio-eventloop.html
    - `async`/`await` and `asyncio` › New in 3.14: `asyncio.get_event_loop()` No Longer Creates a Loop — paragraph 1: "…implicitly creating one (3.12–3.13 emitted a `DeprecationWarning` and created it)."
    - `async`/`await` and `asyncio` › New in 3.14: Asyncio Introspection — paragraph 4: "`ssl.SSLContext.wrap_bio()` got the same validation."
[^22]: asyncio call graph introspection. https://docs.python.org/3.14/library/asyncio-graph.html
    - `async`/`await` and `asyncio` › New in 3.14: Asyncio Introspection — paragraph 1: "Inspect a **running process** without modifying it:"
[^23]: Python support for free threading (HOWTO). https://docs.python.org/3.14/howto/free-threading-python.html
    - Concurrency › The GIL, and the Officially Supported Free-Threaded Build (PEP 779) — paragraph 2: "Rules for free-threaded deployments:"
    - Memory Management and Garbage Collection › Practical Memory Tools — bullet 3: "RSS may still not shrink immediately."
[^24]: PEP 734 – Multiple Interpreters in the Stdlib. https://peps.python.org/pep-0734/
    - Concurrency › New in 3.14: Multiple Interpreters — `concurrent.interpreters` (PEP 734) — paragraph 1: "…3.12): "the isolation of processes with the efficiency of threads"."
[^25]: concurrent.interpreters — Multiple interpreters in the same process. https://docs.python.org/3.14/library/concurrent.interpreters.html
    - Concurrency › New in 3.14: Multiple Interpreters — `concurrent.interpreters` (PEP 734) — paragraph 1: "…3.12): "the isolation of processes with the efficiency of threads"."
    - Concurrency › New in 3.14: Multiple Interpreters — `concurrent.interpreters` (PEP 734) — paragraph 2: "…runs a function in the interpreter on a new thread."
    - Concurrency › New in 3.14: Multiple Interpreters — `concurrent.interpreters` (PEP 734) — paragraph 4: "…as a security boundary — they share one address space."
[^26]: concurrent.futures — InterpreterPoolExecutor, Executor.map buffersize, terminate_workers/kill_workers. https://docs.python.org/3.14/library/concurrent.futures.html
    - Concurrency › New in 3.14: Multiple Interpreters — `concurrent.interpreters` (PEP 734) — paragraph 3: "…CPU-bound work runs in parallel on the default (GIL) build:"
    - Concurrency › `concurrent.futures` and `multiprocessing` — New Default Start Method — paragraph 4: "Other 3.14 executor additions:"
[^27]: multiprocessing — Contexts and start methods (forkserver default; 3.14.9 spawn fallback). https://docs.python.org/3.14/library/multiprocessing.html#contexts-and-start-methods
    - Concurrency › `concurrent.futures` and `multiprocessing` — New Default Start Method — paragraph 1: "…`'forkserver'` instead of `'fork'`.** Windows and macOS already used `'spawn'`."
[^28]: gh-155717 / gh-143706 / gh-146004 — multiprocessing start-method fixes in 3.14.x (see changelog). https://docs.python.org/3.14/whatsnew/changelog.html
    - Concurrency › `concurrent.futures` and `multiprocessing` — New Default Start Method — paragraph 3: "…in 3.14.3) and propagates all `-X` options to child processes."
[^29]: warnings — Concurrent safety of context managers (context_aware_warnings). https://docs.python.org/3.14/library/warnings.html
    - Concurrency › New in 3.14: Concurrency-Safe Warnings Control — paragraph 1: "…threads start with a copy of the caller's `contextvars` context."
[^30]: PEP 768 – Safe external debugger interface for CPython. https://peps.python.org/pep-0768/
    - New in 3.14: Debugging and Observability › Safe External Debugger Interface (PEP 768) — paragraph 1: "…at the next safe point, with no overhead when unused:"
    - New in 3.14: Debugging and Observability › Safe External Debugger Interface (PEP 768) — bullet 2: "…the ptrace scope setting); comparable privileges on macOS and Windows."
[^31]: sys — sys.remote_exec(), sys.flags, sys.prefix under venvs. https://docs.python.org/3.14/library/sys.html
    - New in 3.14: Debugging and Observability › Safe External Debugger Interface (PEP 768) — paragraph 1: "…at the next safe point, with no overhead when unused:"
    - New in 3.14: Debugging and Observability › Safe External Debugger Interface (PEP 768) — bullet 5: "…hook in the target can log or block injected code."
    - Virtual Environments and Packaging › `pyproject.toml` — bullet 5: "…that compute paths before `site` runs see the venv prefix."
[^32]: tarfile — Extraction filters (default 'data' in 3.14). https://docs.python.org/3.14/library/tarfile.html#tarfile-extraction-filter
    - Security › New in 3.14: `tarfile` Extracts with the `'data'` Filter by Default (PEP 706) — paragraph 1: "…the default was `fully_trusted` (with a `DeprecationWarning` announcing this change)."
[^33]: PEP 706 – Filter for tarfile.extractall. https://peps.python.org/pep-0706/
    - Security › New in 3.14: `tarfile` Extracts with the `'data'` Filter by Default (PEP 706) — paragraph 1: "…the default was `fully_trusted` (with a `DeprecationWarning` announcing this change)."
[^34]: PEP 761 – Deprecating PGP signatures for CPython artifacts. https://peps.python.org/pep-0761/
    - Security › Other Security-Relevant Changes in 3.14 — bullet 1: "…(`sigstore verify identity ...`), which CPython has used since 3.11."
    - Virtual Environments and Packaging › `pyproject.toml` — bullet 4: "Verify CPython downloads with **Sigstore**, not PGP (PEP 761)."
[^35]: uuid — UUID objects according to RFC 9562 (uuid6/7/8, NIL, MAX). https://docs.python.org/3.14/library/uuid.html
    - Security › Secrets, Hashing and Tokens — paragraph 1: "…a UUIDv7 also **exposes its creation time** (see UUIDs below)."
    - New in 3.14: UUID Versions 6, 7 and 8 (RFC 9562) — bullet 1: "…(unlike random `uuid4()`), and its random parts come from `os.urandom()`."
    - New in 3.14: UUID Versions 6, 7 and 8 (RFC 9562) — bullet 3: "…a custom-layout format; its default randomness is **not** cryptographically secure."
[^36]: pickle — Data stream format (protocol 5 default in 3.14). https://docs.python.org/3.14/library/pickle.html#data-stream-format
    - JSON and Serialization › Pickle — New in 3.14: Default Protocol 5 — paragraph 1: "…or older**; pass `protocol=4` if a legacy consumer reads them."
[^37]: C API Extension Support for Free Threading (limited API not supported in free-threaded 3.14). https://docs.python.org/3.14/howto/free-threading-extensions.html
    - Virtual Environments and Packaging › `pyproject.toml` — bullet 2: "…out of `py_limited_api` for that build) and declare `Py_mod_gil` support."
