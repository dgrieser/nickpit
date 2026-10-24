### Citations: Python 3.11: The Complete Production Developer Guide

Sources for `python-3.11.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: What's New In Python 3.11. Python Software Foundation. https://docs.python.org/3.11/whatsnew/3.11.html
    - Overview — paragraph 1: "Python 3.11 was released on October 24, 2022."
    - Overview — bullet 1: "…on average (10–60% depending on workload) with no source changes."
    - Overview — paragraph 3: "…safe-path option, and a 4300-digit limit on int/str conversion (CVE-2020-10735)."
    - Overview — bullet 16: "It was "put on hold indefinitely" in 3.11"
    - New in 3.11: Faster CPython — What It Means for Code Review — paragraph 1: "…on the pyperformance suite, and 10–60% faster depending on workload."
    - New in 3.11: Faster CPython — What It Means for Code Review — table row 2: "Setup moved from bytecode (`SETUP_FINALLY`) to a side exception table."
    - New in 3.11: Faster CPython — What It Means for Code Review — bullet 6: "What's New says so explicitly."
    - New in 3.11: Faster CPython — What It Means for Code Review — bullet 7: "…than 3.10, typically offset by the smaller frames and dicts."
    - New in 3.11: Fine-Grained Error Locations in Tracebacks (PEP 657) — bullet 4: "They remain backward compatible with the old named-tuple interface."
    - New in 3.11: Structured Concurrency — `asyncio.TaskGroup` and `asyncio.timeout()` — paragraph 1: "…New recommends over raw `create_task()`/`gather()` and `wait_for()` for new code."
    - Primitive Types and Numerics › Floats — paragraph 1: "New `math` functions in 3.11:"
    - Primitive Types and Numerics › `fractions` and `Booleans` — paragraph 2: "Implement `__int__` or `__index__` on numeric types instead."
    - Strings › f-strings in 3.11 — Know the Pre-PEP 701 Limits — bullet 3: "…on string literals compiles as fast as an equivalent f-string."
    - Variable Annotations and Type Hints › Baseline: What 3.11 Code Should Look Like — bullet 3: "…f(x: int = None)` reports `{'x': int}` (3.10 reported `Optional[int]`)."
    - Variable Annotations and Type Hints › New in 3.11: `Required` / `NotRequired` for `TypedDict` (PEP 655) — paragraph 2: "Use the class syntax or `TypedDict('TD', {'x': int})`."
    - Dates and Times › New in 3.11: `datetime.UTC` and Much Broader `fromisoformat()` — bullet 2: "…ISO 8601 validator.** It still rejects fractional hours and minutes."
    - Lists — paragraph 2: "…15% faster and simple list comprehensions up to 20–30% faster."
    - Dictionaries — paragraph 2: "…to 272 bytes on 64-bit; exact figures vary by build."
    - Dataclasses › New in 3.11: Stricter Mutable-Default Check — paragraph 1: "…instances of ordinary (non-frozen) dataclasses and of subclasses of `list`/`dict`/`set`."
    - Classes and OOP › New in 3.11: `object.__getstate__()` — paragraph 1: "…`OrderedDict`, `deque`, `WeakSet`, `tzinfo`) now includes attributes stored in `__slots__`."
    - Classes and OOP › Deprecated in 3.11: Chaining `classmethod` Descriptors — paragraph 1: "…emits no runtime warning for it, and **3.13 removes it**."
    - Exception Handling › New in 3.11: `sys.exception()` and the Simplified Exception State — bullet 1: "…reflected in later `exc_info()` calls and in a bare `raise`."
    - Context Managers › New in 3.11: `TypeError` (Not `AttributeError`) for Non-Context-Managers — paragraph 1: "…the same change applies to `async with`, `ExitStack.enter_context()` and `AsyncExitStack.enter_async_context()`."
    - `async`/`await` and `asyncio` › Removed in 3.11: `@asyncio.coroutine` — paragraph 1: "`loop.set_default_executor()` now rejects anything but a `ThreadPoolExecutor`."
    - Concurrency › `threading` and `time` — bullet 1: "…monotonic clock, so system clock changes no longer distort timeouts."
    - Concurrency › `threading` and `time` — bullet 2: "…(1 ns resolution) and high-resolution waitable timers on Windows 8.1+."
    - Security › New in 3.11: `-P` / `PYTHONSAFEPATH` — Don't Import From the Script Directory — paragraph 1: "**New in 3.11:** `python -P` or `PYTHONSAFEPATH=1` disables the prepending."
    - Security › Archive Extraction — Always Pass a `filter` (3.11.4+) — paragraph 1: "…starts in 3.12, and the default becomes `'data'` in 3.14."
    - Regular Expressions › Changed in 3.11: Stricter Parsing — bullet 5: "In rare patterns, captured groups differ from 3.10."
    - Regular Expressions › Changed in 3.11: Stricter Parsing — bullet 6: "…engine uses computed gotos and is up to 10% faster."
    - Memory Management — bullet 4: "…at most about 20% higher than 3.10 and often lower."
    - Performance — bullet 2: "`unicodedata.normalize()` is constant-time for pure ASCII."
    - Testing › Other 3.11 Testing Changes — bullet 2: "Add `__init__.py` or point discovery at a regular package."
    - Virtual Environments and Packaging › `pyproject.toml` — bullet 2: "Tools that create virtual environments should use the `venv` scheme."
[^2]: PEP 664 – Python 3.11 Release Schedule (bugfix and security releases, lifespan). https://peps.python.org/pep-0664/ ; Python downloads — active releases table (3.11: security, end of support 2027-10; latest 3.11.17, 2026-10-01). https://www.python.org/downloads/
    - Overview — paragraph 1: "…security support ends in **October 2027** (five years after 3.11.0)."
[^3]: What's New In Python 3.7 — dict insertion order is part of the language spec. https://docs.python.org/3.7/whatsnew/3.7.html
    - Overview — bullet 15: "…keep insertion order.** This is a language guarantee since 3.7."
[^4]: Python 3.11 Built-in Types. https://docs.python.org/3.11/library/stdtypes.html
    - PEP 8 Code Style › Naming Conventions — paragraph 1: "Never write the latter in new code."
    - Primitive Types and Numerics › Integers — paragraph 1: "Arithmetic never overflows, and memory grows as needed."
[^5]: PEP 659 – Specializing Adaptive Interpreter. https://peps.python.org/pep-0659/
    - New in 3.11: Faster CPython — What It Means for Code Review — table row 1: "It de-specializes when types change."
[^6]: PEP 657 – Include Fine Grained Error Locations in Tracebacks. https://peps.python.org/pep-0657/
    - New in 3.11: Fine-Grained Error Locations in Tracebacks (PEP 657) — paragraph 1: "…to the exact sub-expression that failed, not just the line:"
    - New in 3.11: Fine-Grained Error Locations in Tracebacks (PEP 657) — bullet 2: "…and lose the carets, run with `-X no_debug_ranges` or `PYTHONNODEBUGRANGES=1`."
[^7]: PEP 654 – Exception Groups and except*. https://peps.python.org/pep-0654/ ; Built-in Exceptions — Exception groups (derive, split, subgroup). https://docs.python.org/3.11/library/exceptions.html#exception-groups
    - New in 3.11: Exception Groups and `except*` (PEP 654) — paragraph 1: "…`except*` clause matches the *parts* of a group by type."
    - New in 3.11: Exception Groups and `except*` (PEP 654) › `split()`, `subgroup()` and Nested Groups — paragraph 3: "`split()`/`subgroup()` do that for you."
[^8]: PEP 678 – Enriching Exceptions with Notes. https://peps.python.org/pep-0678/
    - New in 3.11: Exception Notes — `add_note()` (PEP 678) — paragraph 1: "Notes appear in the standard traceback below the message."
[^9]: PEP 680 – tomllib: Support for Parsing TOML in the Standard Library. https://peps.python.org/pep-0680/ ; tomllib documentation. https://docs.python.org/3.11/library/tomllib.html
    - New in 3.11: `tomllib` — TOML Parsing in the Standard Library (PEP 680) — paragraph 1: "…the third-party `tomli-w` or `tomlkit` (which preserves comments and style)."
[^10]: Coroutines and Tasks — Task Groups, Timeouts. https://docs.python.org/3.11/library/asyncio-task.html ; wait_for() reimplemented on asyncio.timeout() in 3.12 (gh-96764). https://docs.python.org/3.12/whatsnew/changelog.html
    - New in 3.11: Structured Concurrency — `asyncio.TaskGroup` and `asyncio.timeout()` — paragraph 1: "…New recommends over raw `create_task()`/`gather()` and `wait_for()` for new code."
    - New in 3.11: Structured Concurrency — `asyncio.TaskGroup` and `asyncio.timeout()` › `asyncio.TaskGroup` — paragraph 1: "Semantics that reviewers must know:"
    - New in 3.11: Structured Concurrency — `asyncio.TaskGroup` and `asyncio.timeout()` › `asyncio.timeout()` and `asyncio.timeout_at()` — paragraph 1: "Key facts:"
[^11]: asyncio Exceptions — asyncio.TimeoutError is an alias of TimeoutError since 3.11. https://docs.python.org/3.11/library/asyncio-exceptions.html
    - New in 3.11: Structured Concurrency — `asyncio.TaskGroup` and `asyncio.timeout()` › `asyncio.timeout()` and `asyncio.timeout_at()` — bullet 2: "**In 3.11, `asyncio.TimeoutError` *is* the builtin `TimeoutError`.** So is `concurrent.futures.TimeoutError`."
[^12]: Integer string conversion length limitation. https://docs.python.org/3.11/library/stdtypes.html#int-max-str-digits
    - Primitive Types and Numerics › New in 3.11: The Integer/String Conversion Length Limit (CVE-2020-10735) — paragraph 1: "…3.11 raises `ValueError` when such a conversion exceeds **4300 digits**."
[^13]: Python releases 3.10.7, 3.9.14, 3.8.14 and 3.7.14 (CVE-2020-10735 backport). https://discuss.python.org/t/python-versions-3-10-7-3-9-14-3-8-14-3-7-14-now-available/18844 ; Prevent DoS by large str-int conversions. https://python-security.readthedocs.io/vuln/large-int-str-dos.html
    - Primitive Types and Numerics › New in 3.11: The Integer/String Conversion Length Limit (CVE-2020-10735) — paragraph 1: "…the first feature release that has it from day one."
[^14]: PEP 701 – Syntactic formalization of f-strings (Python 3.12). https://peps.python.org/pep-0701/
    - Strings › f-strings in 3.11 — Know the Pre-PEP 701 Limits — paragraph 1: "…code written for 3.12+ often fails to compile on 3.11:"
[^15]: PEP 673 – Self Type. https://peps.python.org/pep-0673/
    - Variable Annotations and Type Hints › New in 3.11: `typing.Self` (PEP 673) — paragraph 1: "…the bound-`TypeVar` boilerplate for fluent methods, alternative constructors and `__enter__`:"
[^16]: PEP 675 – Arbitrary Literal String Type. https://peps.python.org/pep-0675/
    - Variable Annotations and Type Hints › New in 3.11: `LiteralString` (PEP 675) — paragraph 1: "…at the type level for SQL, shell commands and templates:"
[^17]: PEP 646 – Variadic Generics. https://peps.python.org/pep-0646/
    - Variable Annotations and Type Hints › New in 3.11: Variadic Generics — `TypeVarTuple` and `Unpack` (PEP 646) — paragraph 1: "…is mainly for array-shape typing and for typing `*args` precisely:"
[^18]: PEP 681 – Data Class Transforms. https://peps.python.org/pep-0681/
    - Variable Annotations and Type Hints › New in 3.11: `dataclass_transform` (PEP 681) — paragraph 1: "…for ORMs and model libraries (Pydantic, SQLAlchemy, attrs use it):"
[^19]: enum — Support for enumerations (3.11). https://docs.python.org/3.11/library/enum.html ; What's New In Python 3.11 — enum. https://docs.python.org/3.11/whatsnew/3.11.html#enum
    - New in 3.11: `enum` Overhaul — `StrEnum`, Formatting Changes, `verify()` — paragraph 1: "The 3.11 `enum` module was largely rewritten."
[^20]: Command line and environment — -P and PYTHONSAFEPATH. https://docs.python.org/3.11/using/cmdline.html#cmdoption-P
    - Security › New in 3.11: `-P` / `PYTHONSAFEPATH` — Don't Import From the Script Directory — paragraph 1: "**New in 3.11:** `python -P` or `PYTHONSAFEPATH=1` disables the prepending."
[^21]: Python 3.11 changelog (Security sections of each 3.11.x release). https://docs.python.org/3.11/whatsnew/changelog.html
    - Security › Security-Relevant Changes Across 3.11 Point Releases — paragraph 1: "These fixes exist only in later patch releases:"
[^22]: What's New In Python 3.11 — Notable changes in 3.11.4, 3.11.5, 3.11.10, 3.11.13; PEP 706 – Filter for tarfile.extractall. https://docs.python.org/3.11/whatsnew/3.11.html#notable-changes-in-3-11-4 ; https://peps.python.org/pep-0706/
    - Security › Security-Relevant Changes Across 3.11 Point Releases — paragraph 1: "These fixes exist only in later patch releases:"
[^23]: re — Regular expression operations (atomic groups, possessive quantifiers; 3.11). https://docs.python.org/3.11/library/re.html
    - Regular Expressions › New in 3.11: Atomic Groups and Possessive Quantifiers — paragraph 1: "They are the standard tool against **catastrophic backtracking** (ReDoS)."
