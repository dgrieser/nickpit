### Citations: Python 3.13: The Complete Production Developer Guide

Sources for `python-3.13.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: What's New In Python 3.13. Python Software Foundation. https://docs.python.org/3.13/whatsnew/3.13.html
    - Overview — paragraph 1: "Python 3.13 was released on October 7, 2024."
    - Overview — paragraph 2: "…changes are mostly **interpreter and library** changes, not new syntax:"
    - PEP 8 Code Style › Docstrings — paragraph 2: "…compares the **raw** `__doc__` string against an indented literal breaks."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — bullet 3: "…is required to install packages with C extensions into it."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental › Extension Modules and the GIL Fallback — paragraph 1: "Without it, importing the module on 3.13t enables the GIL:"
    - New in 3.13: Experimental JIT Compiler (PEP 744) — paragraph 1: "…`PYTHON_JIT=1`) — and even when enabled "performance improvements are modest"."
    - New in 3.13: The New Interactive REPL, Colour, and Better Error Messages — paragraph 1: "…shell is now a new Python-based REPL (derived from PyPy's):"
    - New in 3.13: The New Interactive REPL, Colour, and Better Error Messages — paragraph 4: "…that change what you see in logs and CI output:"
    - New in 3.13: Defined `locals()` Semantics (PEP 667) — paragraph 1: "…scopes are unchanged (`locals()` is still the real namespace there)."
    - New in 3.13: Defined `locals()` Semantics (PEP 667) — paragraph 5: "…were a separate nested function (3.12's inlining had changed this)."
    - New in 3.13: "Dead Batteries" Removed (PEP 594) and `lib2to3` Gone — paragraph 1: "…in 3.11) and `tkinter.tix` (deprecated in 3.6) are removed too."
    - New in 3.13: "Dead Batteries" Removed (PEP 594) and `lib2to3` Gone — paragraph 2: "`standard-telnetlib`, `standard-imghdr`), and `audioop` as `audioop-lts`."
    - New in 3.13: "Dead Batteries" Removed (PEP 594) and `lib2to3` Gone — paragraph 3: "Other removals that surface on upgrade (all deprecated for years):"
    - Primitive Types and Numerics › Floats — New in 3.13: `math.fma()` — paragraph 1: "…with a **single rounding** (fused multiply-add), avoiding intermediate precision loss."
    - Primitive Types and Numerics › `fractions` — New in 3.13: Format Specifications — paragraph 1: "…(no presentation type) — on 3.12 those specs raise `ValueError`:"
    - Primitive Types and Numerics › `decimal` — paragraph 1: "…for `Decimal` is deprecated in 3.13 (removal planned for 3.18)."
    - Collections › `itertools` — New in 3.13: `batched(strict=True)` — paragraph 1: "…`strict=True`, which raises `ValueError` if the final batch is short:"
    - Collections › `array` — paragraph 1: "`array.array` is now registered as a `MutableSequence` (it gained `clear()`)."
    - Control Flow — paragraph 2: "…("name 'x' is assigned to before global declaration" on 3.12)."
    - Functions › `functools` — 3.13 Change: `partial` as a Method Emits `FutureWarning` — paragraph 1: "…because a future version will make `partial` a method descriptor:"
    - Functions › Generators — paragraph 1: "…generator code to a plain function) is deprecated in 3.13."
    - Classes and OOP › Removed in 3.13: Chained `classmethod` Descriptors — paragraph 1: "…3.9–3.10, deprecated in 3.11) no longer produces a "class property":"
    - Classes and OOP › Enums — paragraph 1: "…used by `EnumType`) is public in 3.13 for metaclass authors."
    - Exception Handling › New in 3.13: `PythonFinalizationError` — paragraph 1: "…`os.forkpty()`, or `subprocess.Popen` from a finalizer or `__del__` at exit."
    - File I/O and `pathlib` › New in 3.13: `pathlib` Additions and Behaviour Changes — paragraph 1: "Behaviour changes that break or change existing code:"
    - File I/O and `pathlib` › Temporary Files — paragraph 1: "…actually restrict the directory to the current user (CVE-2024-4030 mitigation)."
    - File I/O and `pathlib` › `os` and `os.path` Changes in 3.13 — bullet 1: "…overridden with `PYTHON_CPU_COUNT` / `-X cpu_count=N` — useful in containers."
    - File I/O and `pathlib` › `os` and `os.path` Changes in 3.13 — bullet 5: "…but re-raises other errors (added for the tarfile CVE fixes)."
    - `async`/`await` and `asyncio` › New in 3.13: `TaskGroup` Cancellation Fixes — paragraph 1: "…**closes** the coroutine (no "never awaited" warning) and raises `RuntimeError`."
    - `async`/`await` and `asyncio` › Other asyncio Changes in 3.13 — bullet 5: "…you implement a custom task factory, test against ≥ 3.13.4."
    - Security › New in 3.13: Stricter TLS Defaults — paragraph 1: "…a trusted *intermediate* in the trust store terminate the chain."
    - Security › `tarfile` Extraction Filters — paragraph 2: "…are only as good as the patch level you run."
    - Memory Management — bullet 4: "…objects whose `close()` fails now produce visible "Exception ignored" messages."
    - Performance — paragraph 1: "What 3.13 changes for performance:"
    - Testing — paragraph 1: "3.13 testing notes:"
    - Virtual Environments and Packaging — bullet 2: "…whose names start with a dot (hidden files) are skipped."
    - Removed and Changed in 3.13 — Migration Notes — paragraph 1: "…`sunau`, `telnetlib`, `uu`, `xdrlib`, `lib2to3` (and the `2to3` tool), `tkinter.tix`."
    - Removed and Changed in 3.13 — Migration Notes — paragraph 3: "**Behaviour changes to know:**"
    - Removed and Changed in 3.13 — Migration Notes › Deprecations to Fix Now (Removed in 3.14) — paragraph 1: "…all emit `DeprecationWarning` in 3.13 and are **gone in 3.14**:"
[^2]: PEP 719 – Python 3.13 Release Schedule (bugfix releases through 3.13.16; security fixes until October 2029). https://peps.python.org/pep-0719/
    - Overview — paragraph 1: "…continue on an as-needed basis until **end-of-life in October 2029**."
[^3]: Python 3.13.17 release page (special security release, October 21, 2026) and Python downloads status table. https://www.python.org/downloads/release/python-31317/ and https://www.python.org/downloads/
    - Overview — paragraph 1: "…(and its certifi CA bundle) after a CA certificate incident."
    - Security › Security Fixes in 3.13 Point Releases (selection) — bullet 12: "Upgrade pip/uv inside existing virtual environments too."
[^4]: PEP 602 – Annual Release Cycle for Python (updated: two years of bugfix support from 3.13 on). https://peps.python.org/pep-0602/
    - Overview — paragraph 1: "…full bugfix support followed by three years of security-only fixes**."
[^5]: PEP 703 – Making the Global Interpreter Lock Optional in CPython. https://peps.python.org/pep-0703/
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — paragraph 1: "…a GIL** — nothing changes for code running on it."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — bullet 5: "`PYTHONMALLOC=pymalloc` is rejected there with a fatal "unknown allocator" error."
[^6]: Python experimental support for free threading (3.13 HOWTO). https://docs.python.org/3.13/howto/free-threading-python.html
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — paragraph 1: "…a GIL** — nothing changes for code running on it."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — bullet 1: "…3.14 (PEP 779) — do not describe 3.13t as production-ready."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — bullet 2: "…adaptive interpreter (PEP 659) is disabled in the free-threaded build."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — bullet 4: "…(only a `RuntimeWarning` tells you), unless you force it off."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental › What Actually Changes for Python Code — paragraph 1: "Without it, races show up immediately:"
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental › What Actually Changes for Python Code — paragraph 2: "…limitations of the 3.13 free-threaded build (from the official HOWTO):"
[^7]: PEP 744 – JIT Compilation. https://peps.python.org/pep-0744/
    - New in 3.13: Experimental JIT Compiler (PEP 744) — paragraph 1: "…`PYTHON_JIT=1`) — and even when enabled "performance improvements are modest"."
[^8]: PEP 667 – Consistent views of namespaces. https://peps.python.org/pep-0667/
    - New in 3.13: Defined `locals()` Semantics (PEP 667) — paragraph 1: "…scopes are unchanged (`locals()` is still the real namespace there)."
[^9]: PEP 696 – Type Defaults for Type Parameters. https://peps.python.org/pep-0696/
    - New in 3.13: Type Parameter Defaults (PEP 696) — paragraph 1: "…constructors and through **new PEP 695 syntax** `[T = int]`."
[^10]: PEP 742 – Narrowing types with TypeIs. https://peps.python.org/pep-0742/
    - New in 3.13: `typing.TypeIs` (PEP 742) — paragraph 1: "…a type that is not a subtype of the input."
[^11]: PEP 705 – TypedDict: Read-only items. https://peps.python.org/pep-0705/
    - New in 3.13: `typing.ReadOnly` for `TypedDict` Items (PEP 705) — paragraph 1: "It composes with `Required`/`NotRequired`."
[^12]: PEP 702 – Marking deprecations using the type system; warnings.deprecated. https://peps.python.org/pep-0702/ and https://docs.python.org/3.13/library/warnings.html#warnings.deprecated
    - New in 3.13: `warnings.deprecated()` (PEP 702) — paragraph 1: "…at use sites) and the runtime (a `DeprecationWarning` when used)."
[^13]: copy — Shallow and deep copy operations (copy.replace). https://docs.python.org/3.13/library/copy.html
    - New in 3.13: `copy.replace()` and the `__replace__` Protocol — paragraph 1: "…for dataclasses, `collections.namedtuple()` classes, `datetime.date`/`datetime`/`time`, `types.SimpleNamespace`, `inspect.Signature`/`Parameter` and code objects."
[^14]: queue — A synchronized queue class (Queue.shutdown); asyncio Queues. https://docs.python.org/3.13/library/queue.html and https://docs.python.org/3.13/library/asyncio-queue.html
    - New in 3.13: Queue Shutdown (`queue.Queue` and `asyncio.Queue`) — paragraph 1: "This replaces ad-hoc sentinel values (`None`, `object()`) for stopping consumers."
[^15]: PEP 594 – Removing dead batteries from the standard library. https://peps.python.org/pep-0594/
    - New in 3.13: "Dead Batteries" Removed (PEP 594) and `lib2to3` Gone — paragraph 1: "…in 3.11) and `tkinter.tix` (deprecated in 3.6) are removed too."
[^16]: Command line and environment (PYTHON_GIL, PYTHON_JIT, PYTHON_BASIC_REPL, PYTHON_COLORS, PYTHON_CPU_COUNT, PYTHONMALLOC). https://docs.python.org/3.13/using/cmdline.html
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental › Detecting the Build and the GIL State — paragraph 2: "`-X gil` takes precedence over `PYTHON_GIL`."
    - New in 3.13: Experimental JIT Compiler (PEP 744) — bullet 2: "…is available**; setting it on a normal build does nothing."
    - New in 3.13: The New Interactive REPL, Colour, and Better Error Messages — paragraph 2: "…`FORCE_COLOR=1` control colour for the REPL, tracebacks and doctest output."
[^17]: Python 3.13 changelog (security and bugfix entries per point release). https://docs.python.org/3.13/whatsnew/changelog.html
    - New in 3.13: The New Interactive REPL, Colour, and Better Error Messages — paragraph 3: "Run the latest patch release."
    - `async`/`await` and `asyncio` › New in 3.13: `TaskGroup` Cancellation Fixes — paragraph 2: "…in 3.13.1) — another reason to run the latest patch."
    - `async`/`await` and `asyncio` › Other asyncio Changes in 3.13 — bullet 5: "…you implement a custom task factory, test against ≥ 3.13.4."
    - Security › New in 3.13: Stricter TLS Defaults — paragraph 3: "…identity check), and `asyncio`'s `start_tls()`/`create_connection()` validate `server_hostname` the same way."
    - Security › `tarfile` Extraction Filters — paragraph 2: "…are only as good as the patch level you run."
    - Security › `tarfile` Extraction Filters — paragraph 3: "`shutil.unpack_archive()` on Windows had a drive-prefix traversal fixed in 3.13.14."
    - Security › Security Fixes in 3.13 Point Releases (selection) — paragraph 1: "From the official changelog:"
    - Security › SQL Injection, Subprocess, XML — paragraph 1: "…API — `defusedxml` remains the reviewer-safe default for untrusted input."
    - Command-Line Interfaces: `argparse` and `optparse` › New in 3.13: `deprecated=` for Options, Positionals and Subcommands — bullet 3: "…but `optparse` is legitimate where its stricter POSIX behaviour matters."
    - Memory Management — bullet 1: "…to an incremental collector, and don't recommend tuning for one."
[^18]: typing — Support for type hints (3.13). https://docs.python.org/3.13/library/typing.html
    - Variable Annotations and Type Hints › New in 3.13: Typing Additions at a Glance — paragraph 1: "All of these are documented in the 3.13 `typing` reference."
[^19]: pathlib — Object-oriented filesystem paths (3.13). https://docs.python.org/3.13/library/pathlib.html
    - File I/O and `pathlib` › New in 3.13: `pathlib` Additions and Behaviour Changes — paragraph 1: "Behaviour changes that break or change existing code:"
[^20]: asyncio — Coroutines and Tasks (as_completed, TaskGroup). https://docs.python.org/3.13/library/asyncio-task.html
    - `async`/`await` and `asyncio` › New in 3.13: `as_completed()` Is an Async Iterator — paragraph 1: "Plain `for` iteration (yielding new awaitables) still works."
[^21]: ssl — TLS/SSL wrapper (create_default_context, VERIFY_X509_STRICT). https://docs.python.org/3.13/library/ssl.html
    - Security › New in 3.13: Stricter TLS Defaults — paragraph 1: "…a trusted *intermediate* in the trust store terminate the chain."
[^22]: tarfile — Extraction filters. https://docs.python.org/3.13/library/tarfile.html#tarfile-extraction-filter
    - Security › `tarfile` Extraction Filters — paragraph 1: "…performs a fully trusting extraction; 3.14 makes `'data'` the default."
[^23]: dbm — Interfaces to Unix "databases" (dbm.sqlite3). https://docs.python.org/3.13/library/dbm.html
    - Databases: `sqlite3` and `dbm` › New in 3.13: `dbm.sqlite3` Is the Default `dbm` Backend — paragraph 1: "Existing files are still opened with whatever format `dbm.whichdb()` detects."
[^24]: venv — Creation of virtual environments (scm_ignore_files). https://docs.python.org/3.13/library/venv.html
    - Virtual Environments and Packaging — paragraph 1: "Opt out with `--without-scm-ignore-files`; the `venv.EnvBuilder(scm_ignore_files=...)` API is opt-in."
[^25]: argparse — deprecated parameter of add_argument()/add_parser(). https://docs.python.org/3.13/library/argparse.html
    - Command-Line Interfaces: `argparse` and `optparse` › New in 3.13: `deprecated=` for Options, Positionals and Subcommands — paragraph 1: "…subcommand prints a warning to stderr but still parses it."
[^26]: random — Command-line usage. https://docs.python.org/3.13/library/random.html#command-line-usage
    - `random` and `statistics` › New in 3.13: `python -m random` CLI — paragraph 1: "…module gained a command-line interface for quick choices and numbers."
[^27]: Memory Management — default allocators and the mimalloc allocator (3.13). https://docs.python.org/3.13/c-api/memory.html
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — bullet 5: "`PYTHONMALLOC=pymalloc` is rejected there with a fatal "unknown allocator" error."
    - Memory Management — bullet 2: "The free-threaded build always uses mimalloc."
[^28]: What's New In Python 3.14 (boundary: features and default changes NOT in 3.13). https://docs.python.org/3.14/whatsnew/3.14.html
    - Overview — bullet 10: "UTF-8 as the default `open()` encoding (PEP 686)."
    - Overview — paragraph 5: "…loop still only emits `DeprecationWarning` (it raises `RuntimeError` in 3.14)."
    - New in 3.13: Free-Threaded CPython (PEP 703) — Experimental — bullet 1: "…3.14 (PEP 779) — do not describe 3.13t as production-ready."
    - New in 3.13: Experimental JIT Compiler (PEP 744) — paragraph 1: "…`PYTHON_JIT=1`) — and even when enabled "performance improvements are modest"."
    - `async`/`await` and `asyncio` › Core Patterns for 3.13 — bullet 4: "…3.12/3.13 and raises `RuntimeError` in 3.14 — fix callers now."
    - Memory Management — bullet 1: "…to an incremental collector, and don't recommend tuning for one."
    - Removed and Changed in 3.13 — Migration Notes › Deprecations to Fix Now (Removed in 3.14) — paragraph 1: "…all emit `DeprecationWarning` in 3.13 and are **gone in 3.14**:"
[^29]: What's New In Python 3.15 (boundary: features NOT in 3.13). https://docs.python.org/3.15/whatsnew/3.15.html
    - Overview — bullet 10: "UTF-8 as the default `open()` encoding (PEP 686)."
[^30]: PEP 730 – Adding iOS as a supported platform; PEP 738 – Adding Android as a supported platform. https://peps.python.org/pep-0730/ and https://peps.python.org/pep-0738/
    - Platform Support Changes in 3.13 — bullet 1: "`aarch64-linux-android`, `x86_64-linux-android`)."
[^31]: multiprocessing — Contexts and start methods (3.13). https://docs.python.org/3.13/library/multiprocessing.html#contexts-and-start-methods
    - Concurrency › `multiprocessing` Start Methods — paragraph 1: "…threads running** emits a `DeprecationWarning` because the child can deadlock."
[^32]: sqlite3 — DB-API 2.0 interface for SQLite databases (3.13). https://docs.python.org/3.13/library/sqlite3.html
    - Databases: `sqlite3` and `dbm` › `sqlite3` Changes in 3.13 — bullet 1: "…3.13 now warns (`ResourceWarning`) when a connection is garbage-collected unclosed."
[^33]: Built-in Types — Numeric types (3.13). https://docs.python.org/3.13/library/stdtypes.html#numeric-types-int-float-complex
    - Primitive Types and Numerics › Integers — paragraph 1: "`int` is arbitrary precision; arithmetic never overflows."
[^34]: C API Extension Support for Free Threading (3.13) — Limited C API and Stable ABI not supported. https://docs.python.org/3.13/howto/free-threading-extensions.html
    - Virtual Environments and Packaging — bullet 3: "…wheels cannot serve it — projects need separate `cp313t` builds."
