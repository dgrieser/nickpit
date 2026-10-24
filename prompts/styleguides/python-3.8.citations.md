### Citations: Python 3.8: The Complete Production Developer Guide

Sources for `python-3.8.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Python 3.8 What's New. Python Software Foundation. https://docs.python.org/3/whatsnew/3.8.html
    - Overview — paragraph 1: "Python 3.8 was released on October 14, 2019."
    - Primitive Types and Numerics › Integers — paragraph 4: "…when `base` and `mod` are coprime, computing the modular inverse:"
    - Primitive Types and Numerics › Booleans — paragraph 3: "…use `is` or `is not` with string or integer literals:"
    - Strings: `str` and `bytes` › f-strings in Python 3.8: The `=` Specifier (new in 3.8) — paragraph 1: "…expression text, an `=`, and the repr of the value**:"
    - Tuples — paragraph 2: "**Iterable unpacking in `return` and `yield` (new in 3.8):**"
    - Dictionaries — paragraph 2: "**`reversed()` now works on dicts (new in 3.8):**"
    - Control Flow › `continue` now legal in `finally` (new in 3.8) — paragraph 1: "It is now allowed:"
    - Functions › `functools.lru_cache` — New Simplified Syntax (new in 3.8) — paragraph 1: "…can now be used as a bare decorator without parentheses:"
    - Functions › `functools.singledispatchmethod` (new in 3.8) — paragraph 1: "`singledispatchmethod` extends `singledispatch` to class methods:"
    - Exception Handling › Exception Hierarchy — paragraph 1: "**Python 3.8-specific:** `asyncio.CancelledError` now inherits from `BaseException`, not `Exception`."
    - `async`/`await` and `asyncio` (Python 3.8) › `asyncio.run()` — Now Stable (new in 3.8) — paragraph 1: "…a provisional API; in Python 3.8 it graduates to stable."
    - `async`/`await` and `asyncio` (Python 3.8) › Task Naming (new in 3.8) — paragraph 1: "Tasks can now be named for observability:"
    - `async`/`await` and `asyncio` (Python 3.8) › Windows asyncio: `ProactorEventLoop` default (new in 3.8) — paragraph 1: "…event loop to `ProactorEventLoop`, which supports subprocess and IOCP natively."
    - Logging — paragraph 1: "…root logger — useful in interactive environments and Jupyter notebooks."
    - Concurrency › Standard `multiprocessing` — paragraph 1: "…uses `spawn` instead of `fork` as the default start method."
    - JSON, CSV, and Data Serialization › Pickle — paragraph 1: "…Protocol 4 by default in 3.8** (up from Protocol 3)."
    - Performance Patterns and Optimization › Profiling First — paragraph 1: "…context manager (new in 3.8).** No more manual `enable()`/`disable()` calls."
    - Testing — paragraph 1: "**`AsyncMock` and `IsolatedAsyncioTestCase` are both new in Python 3.8.**"
[^2]: Python 3.7 language guarantee: dict insertion order. https://docs.python.org/3.7/whatsnew/3.7.html
    - Overview — bullet 6: "…is guaranteed by the language spec as of Python 3.7."
    - Dictionaries — paragraph 1: "…just a CPython implementation detail as it was in 3.6)."
[^3]: Python 3 Built-in Types. https://docs.python.org/3.8/library/stdtypes.html
    - PEP 8 Code Style › Naming Conventions — paragraph 1: "`class Foo:` and `class Foo(object):` are identical."
    - Primitive Types and Numerics › Integers — paragraph 1: "It is **arbitrary precision** — there is no separate `long`."
    - Primitive Types and Numerics › Floats — paragraph 1: "Floats are C `double` (64-bit IEEE 754)."
    - Primitive Types and Numerics › Booleans — paragraph 1: "`True == 1` and `False == 0`."
    - Primitive Types and Numerics › Booleans — paragraph 2: "**Falsy values** in Python 3.8:"
    - Division — paragraph 1: "In Python 3, `/` **always performs true (float) division**."
    - Strings: `str` and `bytes` › The Two Types — bullet 1: "All undecorated string literals are `str`."
    - Lists — paragraph 1: "Lists are mutable, ordered, heterogeneous sequences."
    - Tuples — paragraph 1: "Tuples are immutable, ordered sequences."
    - Sets — paragraph 1: "Sets are mutable, unordered collections of unique, hashable elements."
    - `range` in Python 3 — paragraph 1: "`range()` is always a lazy object — no `xrange()` exists."
    - Generators and Iterators › The Iterator Protocol — paragraph 1: "An iterator in Python 3 uses `__next__()`:"
    - Classes and Object-Oriented Programming › `__slots__` — paragraph 1: "…per-instance `__dict__`, reducing memory significantly for classes with many instances:"
[^4]: PEP 572 – Assignment Expressions. https://peps.python.org/pep-0572/
    - New in 3.8: The Walrus Operator `:=` (PEP 572) — paragraph 1: "…"walrus operator" — are the headline feature of Python 3.8."
    - New in 3.8: The Walrus Operator `:=` (PEP 572) › Walrus Operator: Scoping and Pitfalls — paragraph 1: "…target in the **enclosing** function scope, not inside the comprehension:"
[^5]: PEP 570 – Python Positional-Only Parameters. https://peps.python.org/pep-0570/
    - New in 3.8: Positional-Only Parameters (PEP 570) — paragraph 1: "…as positional-only — they cannot be passed as keyword arguments."
[^6]: PEP 544 (Protocol), PEP 586 (Literal), PEP 589 (TypedDict), PEP 591 (Final). https://peps.python.org/
    - Type Hints (PEP 484 + Python 3.8 additions) › Python 3.8 Typing Additions: `TypedDict`, `Literal`, `Final`, `Protocol` — paragraph 1: "…constructs entered the standard library `typing` module in Python 3.8."
    - Type Hints (PEP 484 + Python 3.8 additions) › Python 3.8 Typing Additions: `TypedDict`, `Literal`, `Final`, `Protocol` › `TypedDict` (PEP 589) — bullet 2: "Never use `isinstance()` with `TypedDict`."
    - Type Hints (PEP 484 + Python 3.8 additions) › Python 3.8 Typing Additions: `TypedDict`, `Literal`, `Final`, `Protocol` › `Protocol` (PEP 544) — Structural Subtyping — paragraph 1: "…`Protocol` if it has the required methods/attributes, **regardless of inheritance**."
[^7]: PEP 557 – Data Classes. https://peps.python.org/pep-0557/
    - Dataclasses (Python 3.7+ — stdlib in 3.8) — paragraph 1: "No backport needed in 3.8."
[^8]: Python pathlib module. https://docs.python.org/3.8/library/pathlib.html
    - File I/O › `pathlib` — Preferred Path API — paragraph 1: "…objects implement `os.PathLike` and work with all standard library functions:"
[^9]: Python io module. https://docs.python.org/3.8/library/io.html
    - File I/O › Text Files — paragraph 1: "Always pass `encoding` explicitly:"
[^10]: Python secrets module. https://docs.python.org/3.8/library/secrets.html
    - The `secrets` Module — paragraph 1: "Always use it for security-sensitive values:"
[^11]: Python GIL. https://docs.python.org/3/glossary.html#term-global-interpreter-lock
    - Concurrency › The GIL — paragraph 1: "…means only one thread executes Python bytecode at a time:"
[^12]: multiprocessing.shared_memory. https://docs.python.org/3.8/library/multiprocessing.shared_memory.html
    - Concurrency › `multiprocessing` with Shared Memory (new in 3.8) — paragraph 1: "Python 3.8 introduces `multiprocessing.shared_memory` — zero-copy shared memory between processes:"
[^13]: functools.cached_property. https://docs.python.org/3.8/library/functools.html#functools.cached_property (locking behavior: bpo-43468; removed in 3.12)
    - Functions › `functools.cached_property` (new in 3.8) — paragraph 1: "…and caches it on the instance for the object's lifetime:"
    - Functions › `functools.cached_property` (new in 3.8) — paragraph 3: "…many instances serializes on one lock and can contend badly."
[^14]: importlib.metadata (provisional in 3.8). https://docs.python.org/3.8/library/importlib.metadata.html
    - New in 3.8: `importlib.metadata` — paragraph 1: "…API) for reading installed package metadata without importing the package:"
[^15]: XML vulnerabilities and bpo-17239 (external entities disabled by default in xml.sax/xml.dom since 3.7.1/3.6.8). https://docs.python.org/3/library/xml.html#xml-vulnerabilities
    - Secure Coding Patterns › XML External Entity (XXE) Handling — paragraph 1: "…parsers no longer process external entities by default either (bpo-17239)."
