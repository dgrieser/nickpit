### Citations: Python 3.15: The Complete Production Developer Guide

Sources for `python-3.15.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: What's New in Python 3.15. Python Software Foundation. https://docs.python.org/3.15/whatsnew/3.15.html
    - Overview — paragraph 2: "…scheduled years ago (see the migration notes near the end)."
    - Overview — bullet 5: "`locale.getdefaultlocale()` was **not removed** in 3.15 — it was *un-deprecated*."
    - Overview — bullet 7: "…in 3.14.5, and 3.15 uses the generational GC from 3.13."
    - New in 3.15: `frozendict` Built-in Type (PEP 814) › `isinstance` Checks Must Be Updated — paragraph 1: "`type()` and `str.maketrans()` accept it where they took a dict."
    - New in 3.15: UTF-8 Is the Default Encoding (PEP 686) — paragraph 1: "…locale (Windows code pages like cp1252 no longer leak in)."
    - Primitive Types and Numerics › Floats — paragraph 1: "…754 binary64). 3.15 adds `math.isnormal()`, `math.issubnormal()`, `math.fmax()`, `math.fmin()` and `math.signbit()`:"
    - Primitive Types and Numerics › `bytes` and `bytearray` — New in 3.15: `bytearray.take_bytes()` — paragraph 1: "…the idiom for buffers in protocol parsers and stream decoders:"
    - Functions › `functools` — paragraph 1: "…method accessed through the class, dispatches on the second argument."
    - Classes and OOP › Class Definition — paragraph 1: "…subclasses of `tuple` (including `namedtuple` classes) may define arbitrary `__slots__`."
    - File I/O and `pathlib` › Archives: `tarfile` Hardening — paragraph 1: "…New) and were backported to security releases of older branches."
    - `async`/`await` and `asyncio` › Structured Concurrency — New in 3.15: `TaskGroup.cancel()` — paragraph 1: "…the old "raise a private exception and suppress it" boilerplate:"
    - `async`/`await` and `asyncio` › asyncio APIs That Disappear in 3.16 — paragraph 1: "…`DeprecationWarning` and are **removed in 3.16** — flag every use:"
    - Concurrency › Free-Threading Status in 3.15 — bullet 2: "The official **macOS installers now install free-threading support by default**."
    - Performance › Interpreter and JIT — bullet 1: "…per process with `PYTHON_JIT=1` on JIT-capable builds; measure before adopting."
    - Performance › Interpreter and JIT — bullet 2: "…built with MSVC 18's tail-calling interpreter (15–20% pyperformance speedup reported)."
    - Performance › Idioms That Still Matter — paragraph 1: "…≥ 5.3, kqueue on macOS/BSD) instead of a sleep loop."
    - Security › Security Content of 3.15 Point Releases — bullet 2: "…new in 3.15, but also backported to 3.14.x/3.13.x patch releases)."
    - Virtual Environments and Packaging — paragraph 1: "…symlink, so purelib and platlib no longer share a directory."
    - Virtual Environments and Packaging › C Extensions on 3.15 — bullet 3: "…`Py_GetPath`, `Py_GetPrefix`, `PyImport_ImportModuleNoBlock`, ...) were removed — `pythoncapi-compat` provides replacements."
    - Removed and Changed in 3.15 — Migration Notes — paragraph 1: "…**removed** in 3.15 — code using them fails on 3.15:"
[^2]: PEP 790 – Python 3.15 Release Schedule. https://peps.python.org/pep-0790/
    - Overview — paragraph 1: "…2, 2026) was cut to fix last-minute lazy-import release blockers."
    - Overview — paragraph 1: "…with binary installers), then source-only security releases until **October 2031**."
[^3]: Python 3.15.0 release page (October 9, 2026). https://www.python.org/downloads/release/python-3150/
    - Overview — paragraph 1: "Python 3.15 was released on October 9, 2026"
[^4]: Python 3.15.1 release page (October 21, 2026; expedited security release). https://www.python.org/downloads/release/python-3151/
    - Overview — paragraph 1: "…330 bug fixes — several of them for lazy imports."
    - Security › Security Content of 3.15 Point Releases — bullet 1: "…environment** too — upgrading the interpreter does not touch venvs."
[^5]: Python 3.15.0rc3 release page (October 2, 2026; lazy-import release blockers). https://www.python.org/downloads/release/python-3150rc3/
    - Overview — paragraph 1: "…2, 2026) was cut to fix last-minute lazy-import release blockers."
[^6]: PEP 602 – Annual Release Cycle for Python (2 years of full support starting with 3.13). https://peps.python.org/pep-0602/
    - Overview — paragraph 1: "…with binary installers), then source-only security releases until **October 2031**."
[^7]: PEP 810 – Explicit lazy imports. https://peps.python.org/pep-0810/
    - New in 3.15: Explicit Lazy Imports (PEP 810) — paragraph 1: "…is loaded ("reified") the first time the name is used."
[^8]: The Python Language Reference 3.15, §7.11.1 Lazy imports and `__lazy_modules__`. https://docs.python.org/3.15/reference/simple_stmts.html#lazy-imports
    - New in 3.15: Explicit Lazy Imports (PEP 810) — paragraph 1: "…is loaded ("reified") the first time the name is used."
    - New in 3.15: Explicit Lazy Imports (PEP 810) › Version-Compatible Opt-In: `__lazy_modules__` — paragraph 2: "…inside functions, class bodies and `try` blocks stay eager regardless."
[^9]: PEP 814 – Add frozendict built-in type. https://peps.python.org/pep-0814/
    - New in 3.15: `frozendict` Built-in Type (PEP 814) — paragraph 1: "…and values are hashable, and compares equal regardless of order."
[^10]: PEP 661 – Sentinel Values; built-in `sentinel` documentation. https://peps.python.org/pep-0661/ and https://docs.python.org/3.15/library/functions.html#sentinel
    - New in 3.15: `sentinel` Built-in Type (PEP 661) — paragraph 1: "…readable repr — the standard replacement for `_MISSING = object()`."
[^11]: PEP 798 – Unpacking in Comprehensions. https://peps.python.org/pep-0798/
    - New in 3.15: Unpacking in Comprehensions (PEP 798) — paragraph 1: "…— a flat, readable alternative to nested comprehensions and `itertools.chain.from_iterable()`."
[^12]: PEP 686 – Make UTF-8 mode default. https://peps.python.org/pep-0686/
    - New in 3.15: UTF-8 Is the Default Encoding (PEP 686) — paragraph 1: "…locale (Windows code pages like cp1252 no longer leak in)."
[^13]: PEP 791 – math.integer; `math.integer` documentation. https://peps.python.org/pep-0791/ and https://docs.python.org/3.15/library/math.integer.html
    - Overview — bullet 2: "`intmath` is only a third-party backport package on PyPI)."
    - Primitive Types and Numerics › New in 3.15: `math.integer` (PEP 791) — paragraph 1: "…no removal planned — but new code should use `math.integer`)."
[^14]: PEP 728 – TypedDict with Typed Extra Items. https://peps.python.org/pep-0728/
    - Type Hints › New in 3.15: `TypedDict` with `closed` and `extra_items` (PEP 728) — paragraph 1: "…`TypedDict` is still a plain `dict` and nothing is validated."
[^15]: PEP 747 – Annotating Type Forms. https://peps.python.org/pep-0747/
    - Type Hints › New in 3.15: `typing.TypeForm` (PEP 747) — paragraph 1: "At runtime `TypeForm(x)` just returns `x`."
[^16]: PEP 800 – Disjoint bases in the type system. https://peps.python.org/pep-0800/
    - Type Hints › New in 3.15: `@typing.disjoint_base` (PEP 800) and `TypeVarTuple` Options — paragraph 1: "…`TypeVar` (bound semantics are not yet specified for type checkers)."
[^17]: PEP 799 – A dedicated profiling package for organizing Python profiling tools. https://peps.python.org/pep-0799/
    - New in 3.15: The `profiling` Package and Tachyon (PEP 799) — paragraph 1: "The new `profiling` package groups the built-in profilers:"
[^18]: profiling.sampling — Statistical profiler (Tachyon), including platform permission requirements. https://docs.python.org/3.15/library/profiling.sampling.html
    - New in 3.15: The `profiling` Package and Tachyon (PEP 799) — paragraph 1: "The new `profiling` package groups the built-in profilers:"
[^19]: Python support for free threading (3.15 HOWTO). https://docs.python.org/3.15/howto/free-threading-python.html
    - Concurrency › Free-Threading Status in 3.15 — paragraph 1: "…ABI tag `cp315t`); the default `python3.15` still has a GIL."
    - Concurrency › Free-Threading Status in 3.15 — paragraph 3: "…roughly 1–8% depending on platform, and memory use is higher."
    - Memory Management — bullet 2: "`gc.collect()` releases memory held for delayed freeing."
[^20]: PEP 779 – Criteria for supported status for free-threaded Python. https://peps.python.org/pep-0779/
    - Concurrency › Free-Threading Status in 3.15 — paragraph 1: "…ABI tag `cp315t`); the default `python3.15` still has a GIL."
[^21]: PEP 803 – Stable ABI for free-threaded builds (abi3t). https://peps.python.org/pep-0803/
    - Concurrency › Free-Threading Status in 3.15 — bullet 1: "…include meson-python, scikit-build-core and Maturin; otherwise build `cp315t` wheels separately."
    - Virtual Environments and Packaging › C Extensions on 3.15 — bullet 1: "…the extension can use PEP 697/793/820 APIs; otherwise build `cp315t`."
[^22]: PEP 829 – Package Startup Configuration Files. https://peps.python.org/pep-0829/
    - Security › New in 3.15: Auditable Startup Code (PEP 829) — paragraph 1: "…deprecated, will be ignored in 3.18 and warn from 3.20."
[^23]: Python 3.15 changelog (3.15.1 entries: gh-158824, gh-153888, gh-158936, gh-159288, gh-159894, gh-157953, gh-159428). https://docs.python.org/3.15/whatsnew/changelog.html
    - Overview — paragraph 1: "…330 bug fixes — several of them for lazy imports."
    - New in 3.15: Explicit Lazy Imports (PEP 810) › Reification Semantics Reviewers Must Know — paragraph 1: "…*outside* calls `__getattr__('json')` instead of reifying the lazy import (gh-158824)."
    - Concurrency › Free-Threading Status in 3.15 — bullet 4: "…`bytes.join()` mutation, split-dict iteration) — run the latest patch release."
    - Security › Security Content of 3.15 Point Releases — bullet 1: "…environment** too — upgrading the interpreter does not touch venvs."
[^24]: What's New in Python 3.14 (incl. the 3.14.5 GC revert note). https://docs.python.org/3.14/whatsnew/3.14.html
    - File I/O and `pathlib` › Archives: `tarfile` Hardening — paragraph 1: "…New) and were backported to security releases of older branches."
    - Performance › Interpreter and JIT — bullet 4: "`gc.get_threshold()` is `(2000, 10, 10)` again."
    - Security › Security Content of 3.15 Point Releases — bullet 2: "…new in 3.15, but also backported to 3.14.x/3.13.x patch releases)."
[^25]: PEP 649 – Deferred Evaluation Of Annotations Using Descriptors; PEP 749 – Implementing PEP 649. https://peps.python.org/pep-0649/ and https://peps.python.org/pep-0749/
    - Type Hints › Annotations Are Lazy (PEP 649/749, 3.14+) — paragraph 1: "Use `annotationlib` to read annotations safely:"
[^26]: PEP 750 – Template Strings. https://peps.python.org/pep-0750/
    - Strings › Template Strings (PEP 750, 3.14+) — paragraph 1: "…be an injection risk. 3.15 adds t-string support to `pprint`."
[^27]: threading — serialize_iterator, synchronized_iterator, concurrent_tee (3.15). https://docs.python.org/3.15/library/threading.html
    - Concurrency › Free-Threading Status in 3.15 — bullet 6: "Wrap it with the new 3.15 helpers:"
[^28]: PEP 831 – Frame Pointers Everywhere: Enabling System-Level Observability for Python. https://peps.python.org/pep-0831/
    - Performance › Interpreter and JIT — bullet 3: "…`sysconfig` inherit `-fno-omit-frame-pointer`; custom native builds should add it too."
    - Virtual Environments and Packaging › C Extensions on 3.15 — bullet 2: "…included), or native stack unwinding breaks for the whole process."
[^29]: python.org Downloads — active releases (3.16 pre-release, first release planned for October 2027 — 2027-10-06 on the downloads page, 2027-10-05 in PEP 826; 3.15 end of support 2031-10). https://www.python.org/downloads/ and https://peps.python.org/pep-0826/
    - Overview — paragraph 1: "…3.16 is only a pre-release (final planned for October 2027)."
[^30]: contextlib — ContextDecorator/AsyncContextDecorator with generator and coroutine functions (3.15). https://docs.python.org/3.15/library/contextlib.html
    - Context Managers › New in 3.15: Context-Manager Decorators Respect Generators and Coroutines — paragraph 1: "…generator object was created, so the body ran *outside* it."
[^31]: re — prefixmatch() vs. match(). https://docs.python.org/3.15/library/re.html
    - Regular Expressions › New in 3.15: `re.prefixmatch()` (and soft-deprecated `re.match()`) — paragraph 1: "…code should prefer `prefixmatch()`, code supporting older versions keeps `match()`."
