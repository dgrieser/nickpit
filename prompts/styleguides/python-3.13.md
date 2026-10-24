### Python 3.13: The Complete Production Developer Guide

#### Overview

Python 3.13 was released on October 7, 2024.[^1] It is the first release to get the extended lifecycle from the updated PEP 602: **two years of full bugfix support followed by three years of security-only fixes**.[^4] As of October 2026, 3.13 is in its **security-fix-only phase**: 3.13.16 (1 October 2026 per PEP 719) was the final regular bugfix release with binary installers, and source-only security releases continue on an as-needed basis until **end-of-life in October 2029**.[^2] The latest patch release is **3.13.17** (October 21, 2026), a special security release that upgrades the bundled pip (and its certifi CA bundle) after a CA certificate incident.[^3] Always run the latest 3.13.x — many point releases contain security fixes (see the Security section).

Python 3.13's headline changes are mostly **interpreter and library** changes, not new syntax:[^1]

- an **experimental free-threaded build** (`python3.13t`) that can run without the GIL (PEP 703);
- an **experimental JIT compiler** (PEP 744), off by default and absent from standard builds;
- a **new interactive REPL** (multi-line editing, colour, paste mode) and coloured tracebacks;
- **defined `locals()` semantics** (PEP 667) — a behaviour change for code that mutates `locals()` or relies on `exec()` writing into function locals;
- **typing**: type-parameter defaults (PEP 696, including new `[T = int]` syntax), `typing.TypeIs` (PEP 742), `typing.ReadOnly` (PEP 705), and `warnings.deprecated()` (PEP 702);
- `copy.replace()`, queue shutdown for `queue.Queue` and `asyncio.Queue`, `dbm.sqlite3` as the default `dbm` backend, and many `pathlib` additions;
- **stricter TLS defaults** in `ssl.create_default_context()`;
- the **removal of 19 "dead battery" modules** (PEP 594) plus `lib2to3`/`2to3` — the most common reason a 3.12 codebase fails to import on 3.13.

This guide targets Python 3.13 specifically. Features from earlier versions are fully available and used throughout: 3.6 f-strings, 3.7 dataclasses, 3.8 walrus/positional-only parameters, 3.9 built-in generics (`list[int]`) and `zoneinfo`, 3.10 `match`/`case`, `X | Y` unions, `zip(strict=True)` and `dataclass(slots=True, kw_only=True)`, 3.11 `ExceptionGroup`/`except*`, `asyncio.TaskGroup`, `asyncio.timeout()`, `typing.Self`, `tomllib`, 3.12 PEP 695 type-parameter syntax (`def f[T](...)`, `class C[T]`, the `type` statement), `typing.override`, PEP 701 f-string grammar (nested same-type quotes, backslashes in expressions) and `itertools.batched`. Every section that covers a 3.13-specific change is marked **New in 3.13**.

**Do not use anything newer than 3.13.** The reviewer must never suggest these, because a 3.13 interpreter does not have them:

- **3.14:** deferred evaluation of annotations and the `annotationlib` module (PEP 649/749 — in 3.13 annotations are still evaluated eagerly, so forward references need quotes or `from __future__ import annotations`); template strings `t"..."` and `string.templatelib` (PEP 750); `except A, B:` without parentheses (PEP 758); the `SyntaxWarning` for `return`/`break`/`continue` in `finally` (PEP 765); `compression.zstd` (PEP 784); `concurrent.interpreters` and `InterpreterPoolExecutor` (PEP 734); `sys.remote_exec()` (PEP 768); `uuid.uuid7()`; `functools.Placeholder`; `pathlib.Path.copy()`/`move()`/`info`; `io.Reader`/`io.Writer`; `operator.is_none()`; `heapq.heapify_max()` and friends; `datetime.date.strptime()`; `os.reload_environ()`; `sys._jit`; officially *supported* (non-experimental) free threading (PEP 779).
- **3.15:** `lazy import` (PEP 810); unpacking in comprehensions `[*x for x in xs]` (PEP 798); the `frozendict` and `sentinel` built-ins (PEP 814, PEP 661); `typing.TypeForm` (PEP 747), `TypedDict(closed=..., extra_items=...)` (PEP 728), `@typing.disjoint_base` (PEP 800); UTF-8 as the default `open()` encoding (PEP 686).[^28][^29]

Changed defaults that arrive in **3.14** and must not be assumed on 3.13: `pickle.DEFAULT_PROTOCOL` is still **4** (5 in 3.14); the default `multiprocessing` start method on Linux is still **`fork`** (`forkserver` in 3.14); `tarfile` extraction still has **no default filter** (it becomes `'data'` in 3.14); `asyncio.get_event_loop()` without a running/set loop still only emits `DeprecationWarning` (it raises `RuntimeError` in 3.14).[^28]

Global truths about Python 3.13:

- **Everything is an object.** Integers, functions, classes, modules — all are objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing; hints are not enforced.** Annotations (including `ReadOnly`, `TypeIs`, `@deprecated`'s static meaning) are for type checkers — the interpreter does not enforce them.
- **Annotations are evaluated eagerly** when the `def`/`class` executes (lazy evaluation is 3.14). Use string annotations or `from __future__ import annotations` for forward references.
- **The default build still has a GIL.** Free threading exists only in the separate, experimental `python3.13t` build.
- **Dicts are ordered by insertion** (language guarantee since 3.7).
- **`str` is Unicode; `bytes` is binary.** The default `open()` encoding is still the **locale encoding** (UTF-8 by default only from 3.15) — always pass `encoding=`.
- **Removed modules stay removed.** `import cgi`, `import telnetlib`, `import imghdr`, `import crypt`, `import pipes`, `import lib2to3` and friends raise `ModuleNotFoundError` on 3.13.

***

#### PEP 8 Code Style

##### Naming Conventions

```python
# Variables and functions: snake_case
user_name = 'Alice'
def calculate_total(items: list[float]) -> float:
    return sum(items)

# Constants: SCREAMING_SNAKE_CASE (module level)
MAX_CONNECTIONS = 100
DEFAULT_TIMEOUT = 30.0

# Classes and type aliases: PascalCase
class UserAccount:
    pass

type UserId = int          # PEP 695 type alias statement (3.12+)

# Type parameters: short PascalCase (T, K, V) or descriptive (KeyT)
def first[T](items: list[T]) -> T:
    return items[0]

# Internal use: single underscore prefix
class User:
    def __init__(self) -> None:
        self._internal_state: dict[str, object] = {}

# Name mangling: double underscore prefix (not true privacy)
class Base:
    def __init__(self) -> None:
        self.__private = 'name-mangled'  # accessible as _Base__private
```

In Python 3, all classes implicitly inherit from `object`. `class Foo:` and `class Foo(object):` are identical. Never write the latter.

##### Import Ordering

Three blocks separated by blank lines: standard library, third-party, local. Alphabetize within each group. `ruff check --select I` (or `isort --profile=black`) automates this.

```python
# Standard library
import os
import sys
from collections.abc import Callable, Iterator   # NOT typing.Callable/Iterator
from pathlib import Path

# Third-party
import requests

# Local application
from myapp.models import User

# Never wildcard imports — they defeat static analysis and __all__ hygiene
# Bad:  from module import *
# Good: from module import specific_item
```

In 3.13, `Optional`/`Union`/`List`/`Dict` from `typing` are unnecessary: write `int | None`, `str | bytes`, `list[int]`, `dict[str, int]`.

##### Docstrings

Write Google-style docstrings for every public module, class, and function. First line: a one-sentence imperative summary.

```python
def calculate_discount(price: float, discount_percent: float, min_price: float = 0.0) -> float:
    """Calculate the discounted price.

    Args:
        price: Original price of the item.
        discount_percent: Discount percentage (0-100).
        min_price: Minimum price floor. Defaults to 0.0.

    Returns:
        The discounted price, not less than min_price.

    Raises:
        ValueError: If discount_percent is not between 0 and 100.

    Example:
        >>> calculate_discount(100.0, 20.0)
        80.0
    """
    if not 0 <= discount_percent <= 100:
        raise ValueError('Discount must be between 0 and 100')
    return max(price * (1 - discount_percent / 100), min_price)
```

**New in 3.13: the compiler strips common leading indentation from docstrings.** `__doc__` no longer contains the body's indentation (this shrinks `.pyc` files — by around 5% in the What's New example). Code that post-processes `__doc__` with `inspect.cleandoc()`/`textwrap.dedent()` still works, but code (or tests, or doctest-like tools) that compares the **raw** `__doc__` string against an indented literal breaks.[^1]

```python
def spam():
    """
        This is a docstring with
          leading whitespace.

        It even has multiple paragraphs!
    """

print(repr(spam.__doc__))
# 3.13: '\nThis is a docstring with\n  leading whitespace.\n\nIt even has multiple paragraphs!\n'
# 3.12: '\n        This is a docstring with\n          leading whitespace.\n\n ...'
```

Relative indentation is preserved; only the common prefix is removed. Use `inspect.getdoc()` / `inspect.cleandoc()` when you need a version-independent result.

***

#### New in 3.13: Free-Threaded CPython (PEP 703) — Experimental

Python 3.13 ships an **optional, separate build** of CPython with the GIL disabled, usually installed as `python3.13t` (`python3.13t.exe` on Windows). The standard `python3.13` binary **still has a GIL** — nothing changes for code running on it.[^5][^6]

Key facts a reviewer must get right:

- It is **experimental** in 3.13 ("expect some bugs and a substantial single-threaded performance hit"). It only becomes an officially *supported* build in 3.14 (PEP 779) — do not describe 3.13t as production-ready.[^6][^28]
- The single-threaded overhead in 3.13 is **about 40%** on pyperformance, mainly because the specializing adaptive interpreter (PEP 659) is disabled in the free-threaded build.[^6]
- It is **a different ABI**: extension modules must be compiled specifically for it (wheel tag `cp313t`, file suffix `.cpython-313t-…`). pip **24.1 or newer** is required to install packages with C extensions into it.[^1]
- Importing a C extension that has not declared free-threading support **automatically re-enables the GIL** for the whole process (only a `RuntimeWarning` tells you), unless you force it off.[^6]
- The memory allocator is **mimalloc** (the free-threaded build depends on it); `PYTHONMALLOC=pymalloc` is rejected there with a fatal "unknown allocator" error.[^5][^27]

##### Detecting the Build and the GIL State

```python
import sys
import sysconfig

# Is this a free-threaded BUILD? (recommended check for build-config decisions)
free_threaded_build = bool(sysconfig.get_config_var('Py_GIL_DISABLED'))

# Is the GIL actually disabled RIGHT NOW? (new in 3.13; underscore = CPython-specific)
gil_enabled = sys._is_gil_enabled() if hasattr(sys, '_is_gil_enabled') else True

print(free_threaded_build, gil_enabled)
# python3.13   -> False True
# python3.13t  -> True False   (unless something re-enabled the GIL)
print('free-threading' in sys.version)   # 'experimental free-threading build' in 3.13t
```

Controlling the GIL on the free-threaded build:

```bash
python3.13t -X gil=1 app.py      # force the GIL ON (e.g. to compare behaviour)
PYTHON_GIL=0 python3.13t app.py  # keep the GIL OFF even if a legacy extension asks for it
python3.13 -X gil=0 app.py       # Fatal Python error: config_read_gil: Disabling the GIL is not supported by this build
```

`-X gil` takes precedence over `PYTHON_GIL`.[^16] Forcing `PYTHON_GIL=0` with an extension that was not written for free threading is "at your own risk" — expect crashes or memory corruption, not just wrong answers.

##### What Actually Changes for Python Code

The free-threaded build tries to keep Python-level thread-safety behaviour similar to the GIL build: built-in `dict`, `list` and `set` use internal locks. But Python never *guaranteed* atomicity of compound operations, and the GIL often hid races in practice. Without it, races show up immediately:[^6]

```python
import threading

class Counter:
    def __init__(self) -> None:
        self.n = 0

c = Counter()

def bump() -> None:
    for _ in range(500_000):
        c.n += 1          # ❌ read-modify-write: NOT atomic, on any build

threads = [threading.Thread(target=bump) for _ in range(4)]
for t in threads: t.start()
for t in threads: t.join()
print(c.n)   # GIL build: usually 2000000 (the race is hidden, not absent)
             # 3.13t:     e.g. 825470 — lost updates

# ✅ Correct on every build: protect shared mutable state explicitly
lock = threading.Lock()
c.n = 0

def safe_bump() -> None:
    for _ in range(500_000):
        with lock:
            c.n += 1

threads = [threading.Thread(target=safe_bump) for _ in range(4)]
for t in threads: t.start()
for t in threads: t.join()
print(c.n)   # 2000000
```

Known limitations of the 3.13 free-threaded build (from the official HOWTO):[^6]

- **Immortalization:** once a second thread starts, module-level functions, method descriptors, code objects, modules and classes become *immortal* (never freed). Numeric and string literals and `sys.intern()`ed strings are immortalized too. Programs that create many functions or classes dynamically (e.g. `exec`-generated code, per-request classes) can grow in memory; the HOWTO expects this to be addressed in 3.14.
- **Shared iterators are unsafe:** sharing one iterator/generator object between threads may yield duplicates, skip items, or crash. Give each thread its own iterator or hand out work via `queue.Queue`.
- **Frame objects are unsafe across threads:** `sys._current_frames()` is generally not safe; `inspect.currentframe()`/`sys._getframe()` are fine as long as the frame does not leave its thread.
- **CPU-bound speedups require real parallelism in your design** — and C extensions that declare support. One un-ported dependency turns the whole process back into a GIL process.

```python
# Review pattern: fail loudly if a deployment expects no-GIL but did not get it
import sys
import warnings

def require_free_threading() -> None:
    if hasattr(sys, '_is_gil_enabled') and sys._is_gil_enabled():
        warnings.warn(
            'GIL is enabled: a C extension without free-threading support was '
            'imported, or this is not a free-threaded build',
            RuntimeWarning,
            stacklevel=2,
        )
```

##### Extension Modules and the GIL Fallback

C extensions declare free-threading support with the `Py_mod_gil` slot (multi-phase init) or `PyUnstable_Module_SetGIL()` (single-phase init). Without it, importing the module on 3.13t enables the GIL:[^1]

```text
RuntimeWarning: The global interpreter lock (GIL) has been enabled to load module 'legacy',
which has not declared that it can run safely without the GIL. To override this behavior
and keep the GIL disabled (at your own risk), run with PYTHON_GIL=0 or -Xgil=0.
```

**Review rules for free threading in 3.13:**

- Treat `python3.13t` as an **experiment/benchmark target**, not a production default. Keep CI on the GIL build as the primary target; add a 3.13t job if you want early signal.
- Never rely on "the GIL makes this safe": `x += 1`, check-then-act on dicts (`if k not in d: d[k] = ...`), lazy initialisation, and shared iterators need locks or thread-safe structures (`queue.Queue`) on **both** builds.
- Do not add code paths that branch on `sys._is_gil_enabled()` for correctness — write code that is correct with or without the GIL; branch only for diagnostics or performance tuning.
- Free threading does not help `asyncio` code or I/O-bound threads much; it targets CPU-bound pure-Python work split across threads.

***

#### New in 3.13: Experimental JIT Compiler (PEP 744)

CPython 3.13 contains an experimental copy-and-patch JIT. It is **only compiled in when CPython is built with `--enable-experimental-jit`** — the official 3.13 macOS/Windows binaries and typical distro builds do not include it (python.org binaries only do from 3.14); `uv`-managed (python-build-standalone) 3.13 builds compile it in but leave it **off** (`=yes-off`, enable with `PYTHON_JIT=1`) — and even when enabled "performance improvements are modest".[^1][^7][^28]

- Build options: `--enable-experimental-jit` (JIT on, disable at runtime with `PYTHON_JIT=0`), `=yes-off` (built but off; enable with `PYTHON_JIT=1`), `=interpreter` (Tier 2 interpreter, no JIT), `=no`.
- `PYTHON_JIT` has an effect **only on builds where the JIT is available**; setting it on a normal build does nothing.[^16]
- There is **no `sys._jit` introspection module in 3.13** (added in 3.14). To check whether a build was configured with the JIT, inspect `sysconfig.get_config_var('CONFIG_ARGS')`.
- Building the JIT needs LLVM at build time (not at runtime).

```python
import sysconfig
config_args = sysconfig.get_config_var('CONFIG_ARGS') or ''
print('experimental-jit' in config_args)   # False on python.org/distro builds; True on uv's yes-off builds
```

**Review rule:** do not recommend "enable the JIT" as a performance fix for 3.13 code, and do not claim 3.13 is faster "because of the JIT". Performance work on 3.13 still means profiling, better algorithms, and moving hot loops to vectorised libraries or C extensions.

***

#### New in 3.13: The New Interactive REPL, Colour, and Better Error Messages

The default interactive shell is now a new Python-based REPL (derived from PyPy's):[^1]

- multi-line editing with history that keeps whole blocks;
- `help`, `exit`, `quit` work **without parentheses**;
- **F1** help browser, **F2** history browsing without prompts/output, **F3** paste mode for large blocks;
- coloured prompts and tracebacks.

Controls: `PYTHON_BASIC_REPL=1` restores the old REPL (useful for terminals or tools that script the REPL); `PYTHON_HISTORY` relocates `.python_history`; `PYTHON_COLORS=0`, `NO_COLOR=1` or `FORCE_COLOR=1` control colour for the REPL, tracebacks and doctest output.[^16]

**Security note (3.13.1):** the new REPL used to leave the current directory on `sys.path`; 3.13.1 removed it. Run the latest patch release.[^17]

Error-message improvements that change what you see in logs and CI output:[^1]

```python
# 1. Script shadowing a stdlib module now gets an explicit hint:
#    $ python random.py   (a file named random.py that does `import random`)
#    AttributeError: module 'random' has no attribute 'randint' (consider renaming
#    '/home/me/random.py' since it has the same name as the standard library module
#    named 'random' and prevents importing that standard library module)

# 2. Misspelled keyword arguments get suggestions:
try:
    'Better error messages!'.split(max_split=1)
except TypeError as e:
    print(e)   # split() got an unexpected keyword argument 'max_split'. Did you mean 'maxsplit'?
```

Tests that assert on exact exception messages may need updating when moving to 3.13; prefer asserting on exception **types** and stable substrings.

***

#### New in 3.13: Defined `locals()` Semantics (PEP 667)

PEP 667 standardises what mutating the result of `locals()` does. In **optimized scopes** — functions, generators, coroutines, comprehensions, generator expressions — `locals()` now returns an **independent snapshot on every call**. Module and class scopes are unchanged (`locals()` is still the real namespace there).[^8][^1]

```python
def f() -> None:
    x = 1
    snapshot = locals()
    snapshot['x'] = 2           # changes the snapshot only
    snapshot['y'] = 3
    print(x)                    # 1 — the real local is untouched
    print(locals())             # {'x': 1, 'snapshot': {'x': 2, 'y': 3}}
    # ^ 3.12 printed {'x': 1, 'y': 3, 'snapshot': {...}} — the old shared, cached dict

f()
```

**The breaking case: `exec()`/`eval()` "writing into" function locals.** Code like this sometimes appeared to work in CPython ≤ 3.12 and now **never** does:

```python
def load_config_wrong(src: str) -> int:
    exec(src)                      # ❌ writes into a throwaway snapshot
    return locals()['timeout']     # KeyError in 3.13

def load_config(src: str) -> int:
    ns: dict[str, object] = {}
    exec(src, {}, ns)              # ✅ pass an explicit namespace
    return ns['timeout']           # type: ignore[return-value]

print(load_config('timeout = 30'))   # 30
try:
    load_config_wrong('timeout = 30')
except KeyError as e:
    print('KeyError', e)              # KeyError 'timeout'
```

(Also: never `exec()` untrusted strings at all — this is about trusted code generation.) For running a Python file and reading its globals, `runpy.run_path()` returns the resulting namespace.

**`frame.f_locals` is now a write-through proxy** (`FrameLocalsProxy`) in optimized scopes, so debuggers can reliably modify locals. If you need a snapshot, copy it explicitly with `dict(frame.f_locals)`.

```python
import sys

def g() -> None:
    a = 1
    print(type(sys._getframe().f_locals).__name__)   # FrameLocalsProxy
    snap = dict(sys._getframe().f_locals)              # explicit snapshot

g()
```

A related porting note: `locals()` inside a comprehension at module or class scope once more behaves as if the comprehension were a separate nested function (3.12's inlining had changed this).[^1]

**Review rule:** flag any code that mutates `locals()` expecting an effect, or reads back variables created by `exec()`/`eval()` via `locals()`. Pass explicit dicts.

***

#### New in 3.13: Type Parameter Defaults (PEP 696)

`TypeVar`, `ParamSpec` and `TypeVarTuple` can now have **defaults**, both through the `typing` constructors and through **new PEP 695 syntax** `[T = int]`.[^9] The `[T = default]` syntax is a `SyntaxError` on 3.12, and `TypeVar(..., default=...)` raises `TypeError` on 3.12.

```python
from typing import TypeVar, ParamSpec, NoDefault

# New syntax (3.13+)
class Box[T = int]:
    def __init__(self, item: T) -> None:
        self.item = item

type StrMap[V = str] = dict[str, V]

def identity[U = float](x: U) -> U:
    return x

class Handler[**P = [int, str], R = None]:   # ParamSpec default is a parameter list
    pass

class Shape[*Ts = *tuple[int, ...]]:         # TypeVarTuple default is an unpacked tuple
    pass

# Equivalent constructor form (needed for libraries that avoid the new syntax)
T = TypeVar('T', default=int)
P = ParamSpec('P', default=[int, str])

print(Box.__type_params__[0].__default__)        # <class 'int'>
print(T.has_default(), TypeVar('X').has_default())  # True False
print(TypeVar('X').__default__ is NoDefault)     # True — the "no default" sentinel
```

Rules and pitfalls:

- **Ordering:** a parameter without a default cannot follow one with a default — `class A[T = int, U]` is a `SyntaxError: non-default type parameter 'U' follows default type parameter`.
- **After a `TypeVarTuple`:** a defaulted type parameter after `*Ts` raises `TypeError: Type parameter with a default follows TypeVarTuple` when the class is created.
- **Defaults are for type checkers.** At runtime, `Box('x')` is just a `Box`; nothing checks that `item` is an `int`. The default only decides what an un-parameterized `Box` means to the checker (`Box` ≡ `Box[int]`).
- Use `typing.NoDefault` (new in 3.13) when introspecting — not `None`, which is a valid default (`R = None` above).

Use defaults to keep generic APIs ergonomic: e.g. `class Response[T = dict[str, object]]` lets most callers write `Response` while specialised callers write `Response[User]`.

***

#### New in 3.13: `typing.TypeIs` (PEP 742)

`TypeIs[T]` is a return annotation for user-defined type predicates that narrows **in both branches**, like `isinstance()` does. It is the right tool for most "is this a …?" helpers; `TypeGuard` (3.10) remains available (it is not deprecated) for the rarer case of narrowing to a type that is not a subtype of the input.[^10]

```python
from typing import TypeGuard, TypeIs

def is_str(val: object) -> TypeIs[str]:
    return isinstance(val, str)

def process(val: int | str) -> str:
    if is_str(val):
        return val.upper()     # narrowed to str
    return str(val + 1)        # ✅ narrowed to int in the else branch (TypeGuard can't do this)

# TypeGuard: only the positive branch is narrowed; the return type need not be a subtype
def is_str_list(val: list[object]) -> TypeGuard[list[str]]:
    return all(isinstance(x, str) for x in val)

print(process('a'), process(1))   # A 2
```

Rules:

- With `TypeIs[T]`, the narrowed type **must be consistent with the parameter type** (a checker rejects `def f(x: int) -> TypeIs[str]`). That rule is what makes the negative branch sound.
- The function must return `True` **exactly when** the argument is of type `T`. A `TypeIs` function that returns `False` for some valid `T` values makes the else-branch narrowing lie. Partial checks (e.g. "non-empty str") belong in `TypeGuard`, or in a plain `bool` function.
- Runtime behaviour is just a normal function returning `bool`; nothing is enforced.

***

#### New in 3.13: `typing.ReadOnly` for `TypedDict` Items (PEP 705)

`ReadOnly[...]` marks individual `TypedDict` keys as read-only for type checkers. It composes with `Required`/`NotRequired`.[^11]

```python
from typing import NotRequired, ReadOnly, TypedDict

class Movie(TypedDict):
    title: ReadOnly[str]
    year: ReadOnly[NotRequired[int]]
    rating: float

m: Movie = {'title': 'Blade Runner', 'rating': 8.1}
m['rating'] = 8.2          # fine
# m['title'] = 'Other'     # type checker error: "title" is read-only

# Runtime introspection
print(Movie.__readonly_keys__)   # frozenset({'title', 'year'})
print(Movie.__mutable_keys__)    # frozenset({'rating'})

# Runtime does NOT enforce it — a TypedDict is a plain dict
m['title'] = 'Other'
print(m['title'])                # Other
```

Key benefit: a function that accepts `ReadOnly` items can accept TypedDicts whose corresponding items are *subtypes* (e.g. `ReadOnly[float]` accepts a dict whose item is `int`), because it promises not to write. Use it on API boundaries that only read.

***

#### New in 3.13: `warnings.deprecated()` (PEP 702)

`@warnings.deprecated(msg)` marks functions, methods, overloads and classes as deprecated for **both** type checkers (static error/notice at use sites) and the runtime (a `DeprecationWarning` when used).[^12]

```python
import warnings
from warnings import deprecated

@deprecated('Use fetch_user() instead; removal planned for 4.0')
def get_user(user_id: int) -> dict[str, object]:
    return {'id': user_id}

@deprecated('LegacyClient is replaced by Client')
class LegacyClient:
    pass

print(get_user.__deprecated__)   # Use fetch_user() instead; removal planned for 4.0

with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter('always')
    get_user(1)                    # warns at CALL time, not at decoration time
    LegacyClient()                 # warns on instantiation
    class Child(LegacyClient):     # warns on subclassing
        pass
print([type(w.message).__name__ for w in caught])
# ['DeprecationWarning', 'DeprecationWarning', 'DeprecationWarning']

# Static-only deprecation (no runtime warning), e.g. for hot paths:
@deprecated('Prefer the vectorised API', category=None)
def slow_path() -> None: ...
```

Rules:

- The default category is `DeprecationWarning`, which is **hidden by default** outside `__main__` and test runners. Use `category=FutureWarning` if end users (not developers) must see it.
- Pass `stacklevel=` correctly if you wrap `deprecated` in your own helper so the warning points at the caller.
- For libraries supporting Python < 3.13, import from `typing_extensions` (`from typing_extensions import deprecated`) — the decorator is the same.
- Do not hand-roll `warnings.warn(..., DeprecationWarning)` in new 3.13 code when `@deprecated` expresses it: you lose the type-checker signal.

***

#### New in 3.13: `copy.replace()` and the `__replace__` Protocol

`copy.replace(obj, **changes)` creates a modified copy of an object. It works for any class defining `__replace__()`, and the stdlib implements it for dataclasses, `collections.namedtuple()` classes, `datetime.date`/`datetime`/`time`, `types.SimpleNamespace`, `inspect.Signature`/`Parameter` and code objects.[^13]

```python
import copy
import datetime
import types
from collections import namedtuple
from dataclasses import dataclass

@dataclass(frozen=True)
class Point:
    x: int
    y: int = 0

p = Point(1, 2)
print(copy.replace(p, y=5))                       # Point(x=1, y=5)
print(copy.replace(datetime.date(2024, 1, 31), month=2, day=29))  # 2024-02-29
NT = namedtuple('NT', 'a b')
print(copy.replace(NT(1, 2), b=3))                # NT(a=1, b=3)
print(copy.replace(types.SimpleNamespace(a=1), a=2, b=3))  # namespace(a=2, b=3)

# Your own immutable value types: implement __replace__
class Money:
    __slots__ = ('amount', 'currency')
    def __init__(self, amount: int, currency: str) -> None:
        self.amount, self.currency = amount, currency
    def __replace__(self, **changes: object) -> 'Money':
        fields = {'amount': self.amount, 'currency': self.currency} | changes
        return Money(**fields)   # type: ignore[arg-type]
    def __repr__(self) -> str:
        return f'Money({self.amount}, {self.currency!r})'

print(copy.replace(Money(5, 'EUR'), amount=7))    # Money(7, 'EUR')
```

Pitfalls:

- **Not generic:** `copy.replace({'a': 1}, a=2)` raises `TypeError: replace() does not support dict objects`. Plain classes need `__replace__`.
- It **re-runs construction and validation**: `copy.replace(date(2024, 1, 31), month=2)` raises `ValueError: day is out of range for month`; dataclass `__post_init__` runs again.
- Dataclass fields with `init=False` cannot be replaced (`TypeError`), and unknown names raise `TypeError` from `__init__` — same as `dataclasses.replace()`.
- Copies are **shallow**: unchanged fields are shared references.

Prefer `copy.replace()` in new code that must work across dataclasses, namedtuples and datetimes uniformly; `dataclasses.replace()` and `namedtuple._replace()` remain fine where the type is known.

***

#### New in 3.13: Queue Shutdown (`queue.Queue` and `asyncio.Queue`)

Both `queue.Queue` (and its subclasses) and `asyncio.Queue` gained `shutdown(immediate=False)`, with new exceptions `queue.ShutDown` and `asyncio.QueueShutDown`. This replaces ad-hoc sentinel values (`None`, `object()`) for stopping consumers.[^14]

Semantics:

- After `shutdown()`, `put()` raises `ShutDown`/`QueueShutDown`.
- With `immediate=False` (default), `get()` keeps returning remaining items and raises only once the queue is **empty**; blocked getters are woken up.
- With `immediate=True`, the queue is **drained**: pending items are discarded, `get()` raises immediately, and the unfinished-task count is reduced by the number of discarded items (so `join()` returns once any in-flight items are `task_done()`).

```python
import queue
import threading

work: queue.Queue[str] = queue.Queue()
results: list[str] = []

def consumer() -> None:
    while True:
        try:
            item = work.get()
        except queue.ShutDown:      # ✅ clean exit, no sentinel juggling
            return
        results.append(item.upper())
        work.task_done()

t = threading.Thread(target=consumer)
t.start()
for word in ['a', 'b', 'c']:
    work.put(word)
work.join()          # wait until all items are processed
work.shutdown()      # wakes the blocked consumer; it gets ShutDown
t.join()
print(results)       # ['A', 'B', 'C']

try:
    work.put('late')
except queue.ShutDown:
    print('put after shutdown raises ShutDown')
```

```python
import asyncio

async def main() -> None:
    q: asyncio.Queue[int] = asyncio.Queue()

    async def consumer() -> list[int]:
        seen: list[int] = []
        while True:
            try:
                item = await q.get()
            except asyncio.QueueShutDown:
                return seen
            seen.append(item)
            q.task_done()

    async with asyncio.TaskGroup() as tg:
        task = tg.create_task(consumer())
        for i in range(3):
            await q.put(i)
        q.shutdown()          # consumer drains 0,1,2 then exits
    print(task.result())      # [0, 1, 2]

asyncio.run(main())
```

Note the name difference: `queue.ShutDown` vs `asyncio.QueueShutDown`. `multiprocessing.Queue` did **not** gain `shutdown()` in 3.13.

***

#### New in 3.13: "Dead Batteries" Removed (PEP 594) and `lib2to3` Gone

Nineteen legacy modules deprecated in 3.11 are **removed** in 3.13; importing them raises `ModuleNotFoundError`. `2to3`/`lib2to3` (deprecated in 3.11) and `tkinter.tix` (deprecated in 3.6) are removed too.[^1][^15]

| Removed in 3.13 | Replacement |
|---|---|
| `cgi` | `urllib.parse.parse_qsl()`/`parse_qs()` for query strings; `email.message.EmailMessage` for header parsing (`cgi.parse_header`); a real framework or the `multipart` package for form uploads |
| `cgitb` | `traceback`/`faulthandler`, framework error pages |
| `crypt` (+ `_crypt`) | `hashlib.scrypt`/`hashlib.pbkdf2_hmac`, or `argon2-cffi`/`bcrypt` for password hashing |
| `imghdr` | `filetype`, `puremagic`, `python-magic`, or Pillow |
| `sndhdr` | `filetype`, `puremagic`, `python-magic` |
| `pipes` | `subprocess` (and `shlex.quote()` instead of `pipes.quote`) |
| `telnetlib` | `telnetlib3`, `Exscript` (or better, SSH) |
| `nntplib` | `pynntp` |
| `uu` | `base64` |
| `xdrlib` | a maintained serialization format (`struct`, protobuf, msgpack) |
| `mailcap` | `mimetypes` |
| `spwd` | `python-pam` |
| `nis`, `msilib`, `ossaudiodev` | none in stdlib (third-party / platform tools) |
| `aifc`, `audioop`, `chunk`, `sunau` | `audioop-lts` (for `audioop`), PyPI audio libraries |
| `lib2to3`, `2to3` | `ast` + `libcst`/`parso` for refactoring tools |
| `tkinter.tix` | `tkinter.ttk` |

Unmodified copies of most removed modules are republished on PyPI as `standard-<name>` packages (e.g. `standard-telnetlib`, `standard-imghdr`), and `audioop` as `audioop-lts`.[^1] They are a **stop-gap** for unmaintained code, not a recommendation: `crypt`, `cgi` and `telnetlib` were removed partly because they are insecure or unmaintained.

```python
import importlib

for name in ['cgi', 'crypt', 'imghdr', 'pipes', 'telnetlib', 'uu', 'lib2to3']:
    try:
        importlib.import_module(name)
        print(name, 'available')
    except ModuleNotFoundError:
        print(name, 'removed in 3.13')    # every one of these on 3.13
```

```python
# ❌ 3.12 code that breaks on import in 3.13
# import cgi
# ctype, params = cgi.parse_header('text/html; charset="utf-8"')

# ✅ 3.13 replacement using the email package (same MIME RFCs)
from email.message import EmailMessage

msg = EmailMessage()
msg['content-type'] = 'text/html; charset="utf-8"'
print(msg.get_content_type(), msg['content-type'].params)   # text/html {'charset': 'utf-8'}

# ❌ import pipes; pipes.quote(user_arg)
# ✅
import shlex
print(shlex.quote("it's; rm -rf /"))   # 'it'"'"'s; rm -rf /'
```

Other removals that surface on upgrade (all deprecated for years):[^1]

- `locale.resetlocale()` → `locale.setlocale(locale.LC_ALL, '')`.
- `typing.io` and `typing.re` namespaces → import from `typing` (or better, `typing.IO`, `re.Pattern`).
- `TypedDict('TD', x=int)` keyword-argument form → `TypedDict('TD', {'x': int})` or class syntax (now a `TypeError`).
- `unittest.makeSuite()`, `findTestCases()`, `getTestCaseNames()` → `unittest.TestLoader` methods.
- `urllib.request.urlopen(cafile=..., capath=..., cadefault=...)` → `context=ssl.create_default_context(cafile=...)`.
- Using `pathlib.Path` as a context manager (a no-op since 3.9) → now a `TypeError`.
- Chained `@classmethod` descriptors (e.g. `@classmethod` over `@property`) → no longer pass through: `C.attr` is now a bound method, not the property value.
- `re.template()`/`re.TEMPLATE`, `configparser.LegacyInterpolation`, `turtle.RawTurtle.settiltangle()`, `importlib.metadata` `EntryPoint` subscript access, `webbrowser.MacOSX`.

***

#### Primitive Types and Numerics

##### Integers

`int` is arbitrary precision; arithmetic never overflows.[^33]

```python
import sys

print(sys.maxsize)          # 9223372036854775807 on 64-bit (C ssize_t max, NOT the int limit)
x = 10 ** 1000              # fine — arbitrary precision
one_million = 1_000_000     # underscores in literals (3.6+)

# int <-> str conversion limit (since 3.11 and in security releases of older versions):
# decimal strings longer than 4300 digits raise ValueError — a DoS guard.
print(sys.get_int_max_str_digits())   # 4300
try:
    int('1' * 5000)
except ValueError as e:
    print('ValueError:', str(e)[:60])
# Raise deliberately only where needed: sys.set_int_max_str_digits(0) disables the limit
# (or -X int_max_str_digits / PYTHONINTMAXSTRDIGITS). Never disable it for untrusted input.
```

```python
import math

print(math.isqrt(10**20 + 1))     # 10000000000 — exact integer sqrt (3.8+)
print(pow(38, -1, 137))           # 119 — modular inverse (3.8+)
print(math.lcm(4, 6, 10))         # 60 (3.9+)

# Deprecated since 3.12 — bitwise inversion of bool:
# ~True == -2 (an int!), with DeprecationWarning. Use `not x`.
```

##### Floats — New in 3.13: `math.fma()`

`math.fma(x, y, z)` computes `x * y + z` with a **single rounding** (fused multiply-add), avoiding intermediate precision loss.[^1]

```python
import math

print(0.1 * 10 - 1.0)            # 0.0 — the product rounds to exactly 1.0 first
print(math.fma(0.1, 10, -1.0))   # 5.551115123125783e-17 — the exact residual of 0.1*10
print(math.fma(2.0, 3.0, 1.0))   # 7.0

# Float comparison — always math.isclose, never ==
assert math.isclose(0.1 + 0.2, 0.3)
```

Use `fma` in numerical kernels (error-compensated summation, polynomial evaluation); it does not make ordinary business arithmetic "more correct" — use `decimal.Decimal` for money.

##### `fractions` — New in 3.13: Format Specifications

3.12 added float-style presentation types (`'.3f'`, `'e'`, `'%'`) for `Fraction`. 3.13 adds fill, alignment, sign, width and grouping when formatting **as a fraction** (no presentation type) — on 3.12 those specs raise `ValueError`:[^1]

```python
from fractions import Fraction

print(repr(f'{Fraction(1, 3):>10}'))          # '       1/3'   — 3.13+ (ValueError on 3.12)
print(format(Fraction(1_000_001, 3), '_'))    # 1_000_001/3   — 3.13+
print(format(Fraction(22, 7), '.3f'))         # 3.143         — 3.12+
```

##### `decimal`

```python
from decimal import Decimal, ROUND_HALF_UP, localcontext

price = Decimal('19.99')              # ✅ construct from str
# Decimal(19.99) captures the binary float error — ❌ never construct from float
total = (price * 3).quantize(Decimal('0.01'), rounding=ROUND_HALF_UP)
print(total)                          # 59.97

with localcontext(prec=6) as ctx:     # keyword arguments to localcontext() since 3.11
    print(Decimal(1) / Decimal(7))    # 0.142857
```

The undocumented C-only `'N'` format specifier for `Decimal` is deprecated in 3.13 (removal planned for 3.18).[^1]

##### Booleans

`bool` is a subtype of `int` (`True == 1`). Falsy values: `None`, `False`, numeric zeros, empty sequences/mappings/sets, and objects whose `__bool__` returns `False` or `__len__` returns `0`.

```python
print(isinstance(True, int))   # True
print(True + True)             # 2

# Deprecated since 3.9, still only a warning in 3.13, TypeError in 3.14:
# bool(NotImplemented)  -> DeprecationWarning in 3.13
```

***

#### Strings

##### The Two Types

- **`str`**: Unicode code points. **`bytes`**: raw 8-bit values. Never mixed implicitly.

**The Unicode sandwich:** decode at input, work in `str`, encode at output. Always pass `encoding='utf-8'` to `open()` — in 3.13 the default is still the locale encoding (it becomes UTF-8 only in 3.15). Run tests with `-X warn_default_encoding` (3.10+) to find `open()` calls that rely on the locale.

```python
text = 'hello'
data = b'hello'
# text + data   # TypeError — always
print(text.encode('utf-8') == data)   # True
```

##### String Methods

```python
s = 'Hello, World!'

s.casefold()                            # aggressive lowercase for comparisons
s.removeprefix('Hello, ')               # 'World!' (3.9+) — NOT lstrip('Hello, ')
s.removesuffix('!')                     # 'Hello, World'
s.split(', ', maxsplit=1)               # ['Hello', 'World!']
s.partition(', ')                       # ('Hello', ', ', 'World!')
'a-b-c'.replace('-', '+', count=1)      # 'a+b-c' — New in 3.13: count accepted as keyword
'  hi  '.strip()                        # 'hi'
'42'.zfill(6)                           # '000042'
'café'.encode('utf-8')                  # b'caf\xc3\xa9'
```

**`lstrip`/`rstrip` take a character set, not a prefix** — `'https://x.com'.lstrip('https://')` strips any of `h t p s : /`. Use `removeprefix()`.

##### f-strings (PEP 701 grammar, 3.12+)

Since 3.12, f-string expressions can reuse the outer quote type, contain backslashes and comments, and span lines. Valid in 3.13:

```python
d = {'key': 'value'}
items = ['line1', 'line2']
name, score = 'Alice', 98.765

print(f'Value: {d['key']}')          # same quotes inside — valid since 3.12
print(f'lines: {'\n'.join(items)}')  # backslash in expression — valid since 3.12
print(f'{score:.2f}')                # '98.77'
print(f'{1_000_000:_}')              # '1_000_000'
x = 42
print(f'{x = }')                     # 'x = 42' — debugging only, never in prod logs
```

Note: if your project must still run on 3.11 or older (e.g. a library with `requires-python = ">=3.10"`), these forms are a `SyntaxError` there — match the syntax to the oldest supported version, not to 3.13.

##### Template strings are NOT available

`t'...'` template strings (PEP 750) and `string.templatelib` arrived in **3.14**. In 3.13, `t'Hello {name}'` is a `SyntaxError`. For safe interpolation into SQL/HTML/shell use parameterized APIs, `html.escape()`, `shlex.quote()`, or `string.Template`.

***

#### Variable Annotations and Type Hints

##### Modern Syntax Available in 3.13

```python
from collections.abc import Callable, Iterable, Mapping, Sequence
from typing import Any, Literal, Self, override

# Built-in generics (3.9+), X | Y unions (3.10+)
def parse(raw: str | bytes, *, strict: bool = False) -> dict[str, Any] | None:
    ...

# PEP 695 generics (3.12+) — no TypeVar boilerplate
def first[T](xs: Sequence[T]) -> T:
    return xs[0]

class Stack[T]:
    def __init__(self) -> None:
        self._items: list[T] = []
    def push(self, item: T) -> Self:          # typing.Self (3.11+)
        self._items.append(item)
        return self

type Handler = Callable[[str], None]          # PEP 695 type alias (3.12+), lazily evaluated
type Pair[K, V] = tuple[K, V]

class Base:
    def run(self) -> None: ...

class Child(Base):
    @override                                  # typing.override (3.12+)
    def run(self) -> None: ...

Mode = Literal['r', 'w']
```

`TypeVar` with bounds/constraints in the new syntax: `def f[T: (int, str)](x: T) -> T` (constraints), `class C[T: Hashable]` (bound).

##### Annotations Are Evaluated Eagerly in 3.13

Function and class annotations are evaluated when the `def`/`class` statement runs. A forward reference to a name that does not exist yet raises `NameError` at import time in 3.13 (3.14's PEP 649 makes this lazy):

```python
# ❌ NameError in 3.13 — Node is not defined yet when the def runs
# class Tree:
#     def add(self, child: Node) -> None: ...
# class Node: ...

# ✅ Option 1: string annotation
class Tree:
    def add(self, child: 'Node') -> None: ...

class Node: ...

# ✅ Option 2: postponed evaluation for the whole module (PEP 563)
#    from __future__ import annotations   # must be the first statement

# ✅ Inside a class, Self (3.11+) avoids the need to name the class
```

**Do not** recommend `annotationlib`, `inspect.get_annotations(format=...)` with `Format.FORWARDREF`, or "just reference the class, annotations are lazy" — that is 3.14. `type X = ...` aliases (3.12+) *are* lazily evaluated already in 3.13, which is one reason to prefer them over `X: TypeAlias = ...`.

`from __future__ import annotations` is still supported and common in 3.13 code. Caveat: tools that evaluate annotations at runtime (`typing.get_type_hints()`, pydantic, FastAPI, dataclass introspection) must be able to resolve the names in the module's globals — locally defined or `TYPE_CHECKING`-only imports fail there.

##### New in 3.13: Typing Additions at a Glance

All of these are documented in the 3.13 `typing` reference.[^18]

| Feature | Use |
|---|---|
| `[T = int]`, `TypeVar(default=...)` | Defaults for type parameters (PEP 696) |
| `typing.TypeIs[T]` | Two-sided narrowing predicates (PEP 742) |
| `typing.ReadOnly[T]` | Read-only `TypedDict` items (PEP 705) |
| `warnings.deprecated` | Deprecation for checkers + runtime (PEP 702) |
| `typing.NoDefault` | Sentinel for "type parameter has no default" |
| `typing.get_protocol_members(P)` | `frozenset` of a Protocol's members |
| `typing.is_protocol(C)` | Is `C` a `Protocol` class? |
| `ClassVar[Final[int]]` / `Final[ClassVar[int]]` | Nesting now allowed |

```python
from typing import Protocol, get_protocol_members, is_protocol

class Closeable(Protocol):
    def close(self) -> None: ...
    name: str

print(sorted(get_protocol_members(Closeable)))   # ['close', 'name']
print(is_protocol(Closeable), is_protocol(int))  # True False
```

##### Deprecated Typing Patterns (fix now)

```python
import typing

# DeprecationWarning in 3.13, TypeError in 3.15:
# Point = typing.NamedTuple('Point', x=int, y=int)     # keyword-argument form
# TD = typing.TypedDict('TD')                           # omitted fields
# TD = typing.TypedDict('TD', None)

# ✅ Use these forms instead
Point = typing.NamedTuple('Point', [('x', int), ('y', int)])
TD = typing.TypedDict('TD', {})

# @typing.no_type_check_decorator: deprecated in 3.13 (removed in 3.15)
# typing.AnyStr: deprecated in 3.13 (documentation only — no runtime warning yet);
#   use a PEP 695 constrained type parameter: def concat[S: (str, bytes)](a: S, b: S) -> S
# typing.List/Dict/Set/Tuple/Callable/...: deprecated aliases since 3.9 — use builtins/collections.abc
# typing.Text: deprecated — use str
```

***

#### Collections

##### Lists, Tuples, Dicts, Sets

```python
lst: list[int] = [3, 1, 4, 1, 5]
lst.sort()                       # in-place, stable
print(sorted(lst, reverse=True)) # new list
print(lst[::-1])                 # reversed copy

first, *rest = [1, 2, 3, 4]      # extended unpacking
a, b = 1, 2
a, b = b, a                      # swap

defaults = {'host': 'localhost', 'port': 5432}
overrides = {'port': 9432}
config = defaults | overrides    # dict merge (3.9+) — right side wins
config |= {'debug': True}

for k in reversed(config):       # reversed dict iteration (3.8+)
    pass

s: set[int] = {1, 2, 3}
empty: set[int] = set()          # NOT {} — that is a dict
fs = frozenset(s)                # hashable
```

**Pitfalls (unchanged):** `[[0] * 3] * 3` shares inner lists; never mutate a dict/set while iterating it (`RuntimeError`); `Counter.__or__` is multiset max, not merge.

```python
matrix = [[0] * 3 for _ in range(3)]   # ✅ independent rows
d = {'a': 1, 'b': None}
d = {k: v for k, v in d.items() if v is not None}   # ✅ rebuild instead of deleting while iterating
print(matrix, d)
```

##### `collections`

```python
from collections import ChainMap, Counter, OrderedDict, defaultdict, deque

word_count: defaultdict[str, int] = defaultdict(int)
for word in 'a b a'.split():
    word_count[word] += 1

c = Counter('abracadabra')
print(c.most_common(2))           # [('a', 5), ('b', 2)]

q: deque[str] = deque(maxlen=1000)  # O(1) appends/pops at both ends
q.append('x'); q.appendleft('y')

env = ChainMap({'color': 'red'}, {'color': 'blue', 'user': 'guest'})
print(env['color'], env['user'])  # red guest
```

`collections.abc.ByteString` and `typing.ByteString` are deprecated (removal in 3.17) — use `collections.abc.Buffer` (3.12+) for "anything supporting the buffer protocol", or an explicit `bytes | bytearray | memoryview` union.

##### `itertools` — New in 3.13: `batched(strict=True)`

`itertools.batched()` (3.12) gains `strict=True`, which raises `ValueError` if the final batch is short:[^1]

```python
from itertools import batched, pairwise

print(list(batched('ABCDEFG', 3)))           # [('A','B','C'), ('D','E','F'), ('G',)]
try:
    list(batched('ABCDEFG', 3, strict=True))
except ValueError as e:
    print(e)                                  # batched(): incomplete batch

print(list(pairwise([1, 2, 3])))              # [(1, 2), (2, 3)] (3.10+)
```

`itertools` objects no longer support `copy`/`deepcopy`/`pickle` from 3.14 (deprecated with a warning in 3.13) — don't pickle `itertools.count()` and friends.

##### `array`

The `'w'` type code (`Py_UCS4`) is new in 3.13; the `'u'` (`wchar_t`) type code now emits `DeprecationWarning` and is scheduled for removal in 3.16. `array.array` is now registered as a `MutableSequence` (it gained `clear()`).[^1]

```python
from array import array
from collections.abc import MutableSequence

a = array('w', 'héllo')     # ✅ 3.13 — not array('u', ...)
a.clear()
print(len(a), isinstance(a, MutableSequence))   # 0 True
```

***

#### Control Flow

```python
# match/case (3.10+) — structural pattern matching
def describe(event: dict[str, object]) -> str:
    match event:
        case {'type': 'click', 'pos': (x, y)}:
            return f'click at {x},{y}'
        case {'type': 'key', 'key': str(k)} if k.isalpha():
            return f'letter {k}'
        case {'type': str(t)}:
            return f'other {t}'
        case _:
            return 'invalid'

print(describe({'type': 'click', 'pos': (1, 2)}))   # click at 1,2

# for/else — else runs only if no break
for n in range(2, 10):
    if 10 % n == 0:
        break
else:
    n = -1

# Walrus (3.8+)
import io
stream = io.BytesIO(b'x' * 100_000)
while chunk := stream.read(65_536):
    pass
```

**Pitfall:** `case name:` with a bare name is a **capture pattern** that matches anything — use dotted names (`case Color.RED:`) or literals for constants.

**3.13 fix:** a `global` declaration inside an `except` block, for a name assigned in the `else` block, no longer raises a spurious `SyntaxError` ("name 'x' is assigned to before global declaration" on 3.12).[^1]

**Deprecated (compile-time `SyntaxWarning` in 3.13, planned to become a `SyntaxError`):** numeric literals immediately followed by keywords (`0in x`, `1if x else 2`) — always put a space.

***

#### Functions

```python
def greet(name: str, greeting: str = 'Hello', /, *, punctuation: str = '!') -> str:
    """Positional-only before /, keyword-only after *."""
    return f'{greeting}, {name}{punctuation}'

print(greet('Alice'), greet('Bob', 'Hi', punctuation='.'))
```

**Pitfall: mutable defaults are evaluated once:**

```python
def append_bad(x: int, acc: list[int] = []) -> list[int]:   # ❌ shared list
    acc.append(x)
    return acc

def append_good(x: int, acc: list[int] | None = None) -> list[int]:   # ✅
    acc = [] if acc is None else acc
    acc.append(x)
    return acc

print(append_bad(1), append_bad(2))     # [1, 2] [1, 2]
print(append_good(1), append_good(2))   # [1] [2]
```

##### Closures and Late Binding

```python
funcs = [lambda: i for i in range(3)]
print([f() for f in funcs])        # [2, 2, 2] — late binding
funcs = [lambda i=i: i for i in range(3)]
print([f() for f in funcs])        # [0, 1, 2]
```

##### `functools` — 3.13 Change: `partial` as a Method Emits `FutureWarning`

A `functools.partial` object stored as a class attribute currently does **not** bind `self` (it is not a descriptor). In 3.13, accessing it through an instance emits `FutureWarning`, because a future version will make `partial` a method descriptor:[^1]

```python
import functools
import warnings

def log(prefix: str, msg: str) -> str:
    return f'{prefix}: {msg}'

class Service:
    info = functools.partial(log, 'INFO')                    # ❌ FutureWarning on access
    warn = staticmethod(functools.partial(log, 'WARN'))      # ✅ keeps today's behaviour

with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter('always')
    print(Service().info('started'))        # INFO: started (for now)
    print(Service().warn('disk'))           # WARN: disk
print([type(x.message).__name__ for x in w])  # ['FutureWarning']
```

If you *want* method binding, use `functools.partialmethod`.

```python
import functools

@functools.cache                   # unbounded memoization (3.9+) — only for bounded inputs
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)

@functools.lru_cache(maxsize=1024) # bounded
def lookup(key: str) -> str:
    return key.upper()

class Dataset:
    def __init__(self, data: list[float]) -> None:
        self.data = data

    @functools.cached_property     # computed once per instance; needs __dict__ (no __slots__)
    def mean(self) -> float:
        return sum(self.data) / len(self.data)

@functools.singledispatch
def render(obj: object) -> str:
    raise TypeError(type(obj).__name__)

@render.register
def _(obj: int) -> str:            # dispatch from the annotation
    return f'int:{obj}'

@render.register
def _(obj: str | bytes) -> str:    # union types in register (3.11+)
    return f'text:{obj!r}'

print(fib(30), Dataset([1.0, 3.0]).mean, render(3), render('x'))
```

`cached_property` does not lock (since 3.12) — with threads (and especially on the free-threaded build) the getter can run more than once concurrently; make it idempotent or protect it.

##### Generators

```python
from collections.abc import Generator, Iterator

def countdown(n: int) -> Iterator[int]:
    while n > 0:
        yield n
        n -= 1

def accumulate() -> Generator[int, int, str]:
    total = 0
    while True:
        value = yield total
        if value is None:
            return 'done'
        total += value

print(list(countdown(3)))   # [3, 2, 1]
```

Deprecated: the 3-argument `gen.throw(type, value, tb)` signature — call `gen.throw(exc)`. Assigning a `__code__` object of a different kind (e.g. generator code to a plain function) is deprecated in 3.13.[^1]

***

#### Dataclasses

```python
from dataclasses import KW_ONLY, dataclass, field
import copy

@dataclass(slots=True, frozen=True)       # slots/kw_only since 3.10
class Coordinate:
    lat: float
    lon: float

@dataclass(kw_only=True)
class User:
    id: int
    name: str
    tags: list[str] = field(default_factory=list)   # ✅ never `= []`

@dataclass
class Request:
    url: str
    _: KW_ONLY                       # everything after is keyword-only
    timeout: float = 10.0
    retries: int = 3

c = Coordinate(52.5, 13.4)
moved = copy.replace(c, lon=13.5)    # New in 3.13: works on any dataclass (calls __init__)
print(moved, Request('https://x', timeout=5))
```

Pitfalls:

```python
from dataclasses import dataclass, field

try:
    @dataclass
    class Bad:
        items: list[int] = []        # ValueError: mutable default
except ValueError as e:
    print('ValueError:', e)

try:
    @dataclass
    class BadOrder:
        x: int = 0
        y: int                       # TypeError: non-default argument follows default
except TypeError as e:
    print('TypeError:', e)
```

`@dataclass(slots=True)` creates a **new class**. Zero-argument `super()` inside methods of a slotted dataclass raised `TypeError: super(type, obj): obj (instance of B) is not an instance or subtype of type (B).` up to **3.13.13**; it works from **3.13.14** on (the 3.13 docs still carry the old warning, quoting an older wording of the message). If you must support older 3.13 patch releases, use the two-argument form `super(Cls, self)` there. `frozen=True` + `slots=True` instances are compact and hashable — good value objects.

***

#### Classes and OOP

##### Class Definition

```python
import math
from typing import Self

class Shape:
    def __init__(self, color: str = 'black') -> None:
        self.color = color

    def __repr__(self) -> str:
        return f'{type(self).__name__}(color={self.color!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, type(self)):
            return NotImplemented
        return self.__dict__ == other.__dict__
    # Defining __eq__ sets __hash__ = None: mutable instances stay unhashable (correct).

class Circle(Shape):
    def __init__(self, radius: float, **kwargs: str) -> None:
        super().__init__(**kwargs)
        self.radius = radius

    @property
    def area(self) -> float:
        return math.pi * self.radius ** 2

    @classmethod
    def unit(cls) -> Self:
        return cls(1.0)

print(Circle.unit(), round(Circle(2).area, 2))
```

##### New in 3.13: `__static_attributes__`, `__firstlineno__`, `property.__name__`

```python
class Account:
    def __init__(self) -> None:
        self.owner = ''
        self.balance = 0

    def close(self) -> None:
        self.closed = True

    @property
    def label(self) -> str:
        return self.owner

print(Account.__static_attributes__)   # ('balance', 'closed', 'owner') — names assigned via self.X
print(type(Account.__firstlineno__))    # <class 'int'> — line of the class statement
print(Account.label.__name__)           # label — property objects now have __name__
```

These are useful for tooling (linters, ORMs, serializers). `__static_attributes__` is computed by the compiler from `self.<name> = ...` assignments in the class body's functions; it does not see attributes set via `setattr()` or from outside the class.

##### Removed in 3.13: Chained `classmethod` Descriptors

Stacking `@classmethod` on top of `@property` (allowed in 3.9–3.10, deprecated in 3.11) no longer produces a "class property":[^1]

```python
class Config:
    @classmethod
    @property
    def name(cls) -> str:      # ❌ 3.13: not a class property any more
        return 'cfg'

print(type(Config.name).__name__)   # method — a bound method, not 'cfg'

# ✅ Use a plain class attribute, a classmethod you call, or a metaclass property
class Config2:
    name = 'cfg'
```

##### Abstract Base Classes and Protocols

```python
from abc import ABC, abstractmethod
from typing import Protocol, runtime_checkable

class Repository(ABC):
    @abstractmethod
    def get(self, id: int) -> dict[str, object] | None: ...

    def exists(self, id: int) -> bool:
        return self.get(id) is not None

@runtime_checkable
class SupportsClose(Protocol):
    def close(self) -> None: ...

class Conn:
    def close(self) -> None: ...

print(isinstance(Conn(), SupportsClose))   # True — structural check of method names only
```

`isinstance()` against a `runtime_checkable` Protocol only checks that the attributes exist, not their signatures.

##### Enums

```python
from enum import Enum, Flag, IntEnum, StrEnum, auto

class Color(Enum):
    RED = auto()
    GREEN = auto()

class Status(StrEnum):        # 3.11+
    ACTIVE = auto()           # value 'active'

class Perm(Flag):
    READ = auto()
    WRITE = auto()

print(Status.ACTIVE == 'active', Perm.READ | Perm.WRITE)
```

`enum.EnumDict` (the class namespace used by `EnumType`) is public in 3.13 for metaclass authors.[^1]

***

#### Exception Handling

```python
import logging

logger = logging.getLogger(__name__)

class AppError(Exception):
    """Base for all application-specific errors."""

class ConfigError(AppError):
    def __init__(self, key: str, message: str = '') -> None:
        self.key = key
        super().__init__(f'Config error for {key!r}' + (f': {message}' if message else ''))

def parse_port(raw: str) -> int:
    try:
        return int(raw)
    except ValueError as e:
        err = ConfigError('port', 'must be an integer')
        err.add_note(f'raw value: {raw!r}')     # notes (3.11+) show in tracebacks
        raise err from e                         # explicit chaining

try:
    parse_port('N/A')
except ConfigError as e:
    print(e, e.__notes__, type(e.__cause__).__name__)
```

##### Exception Groups (3.11+)

```python
def run_all() -> None:
    errors = [ValueError('bad value'), TypeError('bad type')]
    raise ExceptionGroup('batch failed', errors)

try:
    run_all()
except* ValueError as eg:
    print('values:', [str(e) for e in eg.exceptions])
except* TypeError as eg:
    print('types:', [str(e) for e in eg.exceptions])
```

`asyncio.TaskGroup` raises `ExceptionGroup`s, so code using it must handle failures with `except*` (or by catching `ExceptionGroup` and inspecting `.exceptions`). In 3.13 the parenthesised form `except (A, B):` is still mandatory — `except A, B:` is 3.14 syntax.

##### New in 3.13: `PythonFinalizationError`

A new `RuntimeError` subclass raised when an operation is blocked during interpreter shutdown — starting a thread, `os.fork()`, `os.forkpty()`, or `subprocess.Popen` from a finalizer or `__del__` at exit.[^1]

```python
import sys
print(issubclass(PythonFinalizationError, RuntimeError))   # True — existing `except RuntimeError` still works
# Typical trigger: a __del__ or atexit-time cleanup that starts a thread:
#   PythonFinalizationError: can't create new thread at interpreter shutdown
```

Review rule: shutdown-time cleanup must not spawn threads or processes; do that work in an explicit `close()`/context manager before exit.

##### Re-raise Correctly

```python
import logging
logger = logging.getLogger(__name__)

def risky() -> None:
    raise OSError('disk')

try:
    try:
        risky()
    except Exception:
        logger.exception('Unexpected error')   # logs traceback
        raise                                   # ✅ bare raise keeps the traceback
except OSError:
    pass
```

***

#### Context Managers

```python
import contextlib
import tempfile
import time
from pathlib import Path

# Parenthesized multi-item with (official since 3.10)
with tempfile.TemporaryDirectory() as tmp:
    src, dst = Path(tmp, 'in.txt'), Path(tmp, 'out.txt')
    src.write_text('hello\n', encoding='utf-8')
    with (
        open(src, encoding='utf-8') as fin,
        open(dst, 'w', encoding='utf-8') as fout,
    ):
        for line in fin:
            fout.write(line.upper())
    print(dst.read_text(encoding='utf-8'))   # HELLO

@contextlib.contextmanager
def timed(name: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        print(f'{name}: {time.perf_counter() - start:.3f}s')

with timed('work'), contextlib.suppress(FileNotFoundError):
    Path('/nonexistent/file').unlink()

# contextlib.chdir (3.11+) — NOT thread-safe: changes the process-wide cwd
```

`contextlib.ExitStack` remains the tool for a dynamic number of resources. Note that `pathlib.Path` objects are **no longer context managers** in 3.13 (`with Path(...):` raises `TypeError`).

***

#### File I/O and `pathlib`

Always pass `encoding='utf-8'` to `open()`, `Path.read_text()` and `Path.write_text()`.

```python
import tempfile
from pathlib import Path

with tempfile.TemporaryDirectory() as tmp:
    p = Path(tmp) / 'reports' / 'output.csv'
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text('a,b\n', encoding='utf-8')
    print(p.name, p.stem, p.suffix)              # output.csv output .csv
    print(p.read_text(encoding='utf-8'))
    print(p.with_stem('summary').name)           # summary.csv (3.9+)
    for root, dirs, files in Path(tmp).walk():   # Path.walk (3.12+)
        print(root.name, files)
    p.unlink(missing_ok=True)
```

##### New in 3.13: `pathlib` Additions and Behaviour Changes

```python
from pathlib import Path, PurePath, PurePosixPath

# Path.from_uri — parse file: URIs (absolute only)
print(Path.from_uri('file:///etc/hosts'))        # /etc/hosts
for uri in ['file:etc/hosts', 'https://example.com/x']:
    try:
        Path.from_uri(uri)
    except ValueError as e:
        print('ValueError:', e)
# ValueError: URI is not absolute: 'file:etc/hosts'
# ValueError: URI does not start with 'file:': 'https://example.com/x'

# PurePath.full_match — whole-path glob matching with recursive **
p = PurePosixPath('src/pkg/sub/mod.py')
print(p.full_match('src/**/*.py'))   # True
print(p.full_match('*.py'))          # False — must match the WHOLE path
print(p.match('*.py'))               # True  — match() is relative, from the right
print(p.match('src/**/*.py'))        # False — match() treats ** like *, not recursive

# PurePath.parser — the os.path flavour module (posixpath or ntpath)
print(PurePosixPath('x').parser.__name__)   # posixpath
```

Behaviour changes that break or change existing code:[^19][^1]

- **`Path.glob('**')` and `rglob('**')` now return files *and* directories** (previously only directories). Add a trailing slash (`'**/'`) to keep the old "directories only" behaviour.
- `Path.glob()`/`rglob()` accept `recurse_symlinks=` (keyword-only); `is_file()`, `is_dir()`, `owner()`, `group()` accept `follow_symlinks=`.
- Unsupported operations raise `pathlib.UnsupportedOperation` (subclass of `NotImplementedError`) instead of plain `NotImplementedError`.
- `PurePath.is_reserved()` is deprecated (removed in 3.15) — on Windows use `os.path.isreserved()` (new in 3.13, Windows-only).
- Passing extra positional arguments to `relative_to()`/`is_relative_to()` is deprecated (removed in 3.14).
- `with Path(...) as p:` raises `TypeError` (removed; it had been a no-op since 3.9).

```python
import tempfile
from pathlib import Path

with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    (root / 'a' / 'b').mkdir(parents=True)
    (root / 'a' / 'f.txt').write_text('x', encoding='utf-8')
    rel = lambda it: sorted(str(x.relative_to(root)) for x in it)
    print(rel(root.glob('**')))    # ['.', 'a', 'a/b', 'a/f.txt'] — 3.13: files included
    print(rel(root.glob('**/')))   # ['.', 'a', 'a/b']            — directories only
    print(rel(root.rglob('*.txt')))  # ['a/f.txt']
```

`glob.translate()` (new in 3.13) converts a shell-style pattern to a regex — useful for matching path strings without touching the filesystem:

```python
import glob
import re

rx = re.compile(glob.translate('src/**/*.py', recursive=True, include_hidden=True))
print(bool(rx.match('src/pkg/mod.py')), bool(rx.match('docs/x.py')))   # True False
```

##### Binary Files and Chunked Reads

```python
import io

CHUNK = 65_536
stream = io.BytesIO(b'\x00' * 200_000)
total = 0
while chunk := stream.read(CHUNK):
    total += len(chunk)
print(total)   # 200000
```

##### Temporary Files

```python
import os
import tempfile

with tempfile.NamedTemporaryFile('w', suffix='.csv', encoding='utf-8', delete_on_close=False) as tmp:
    tmp.write('col1,col2\n')
    tmp.close()                      # delete_on_close=False (3.12+): reopen by name, also on Windows
    with open(tmp.name, encoding='utf-8') as f:
        print(f.read().strip())      # col1,col2
print(os.path.exists(tmp.name))      # False — deleted when the with block exits

with tempfile.TemporaryDirectory() as d:
    pass

# NEVER use tempfile.mktemp() — TOCTOU race
```

3.13 note: on Windows, `os.mkdir(mode=0o700)` and therefore `tempfile.mkdtemp()` now actually restrict the directory to the current user (CVE-2024-4030 mitigation).[^1]

##### `os` and `os.path` Changes in 3.13

- `os.process_cpu_count()` — CPUs usable by *this process* (respects affinity). `concurrent.futures`, `multiprocessing` and `compileall` now size their default worker pools from it. Both `cpu_count()` and `process_cpu_count()` can be overridden with `PYTHON_CPU_COUNT` / `-X cpu_count=N` — useful in containers.[^1]
- Linux timer file descriptors: `os.timerfd_create()`, `timerfd_settime()`, etc.
- `os.posix_spawn()` accepts `env=None`; `subprocess` uses `posix_spawn()` in more cases (including the default `close_fds=True` on recent Linux/FreeBSD/Solaris).
- On Windows: `os.path.isabs()` no longer treats `\foo` (one leading slash) as absolute; `os.lchmod()`, `os.fchmod()` and fd/`follow_symlinks` support in `chmod()` are available.
- **3.13.4:** `os.path.realpath(path, strict=os.path.ALLOW_MISSING)` — resolves symlinks, tolerates a missing final path, but re-raises other errors (added for the tarfile CVE fixes).[^1]

```python
import os
print(os.process_cpu_count() <= os.cpu_count())   # True
print(hasattr(os.path, 'ALLOW_MISSING'))          # True on 3.13.4+
```

***

#### `async`/`await` and `asyncio`

##### Core Patterns for 3.13

```python
import asyncio

async def fetch(i: int) -> int:
    await asyncio.sleep(0.01)
    return i * 2

async def main() -> None:
    # Structured concurrency (3.11+): all children finish or all are cancelled
    async with asyncio.TaskGroup() as tg:
        tasks = [tg.create_task(fetch(i), name=f'fetch-{i}') for i in range(3)]
    print([t.result() for t in tasks])          # [0, 2, 4]

    # Timeouts (3.11+)
    try:
        async with asyncio.timeout(0.05):
            await asyncio.sleep(1)
    except TimeoutError:                         # asyncio.TimeoutError is an alias of TimeoutError since 3.11
        print('timed out')

    # Bounded concurrency
    sem = asyncio.Semaphore(10)
    async def bounded(i: int) -> int:
        async with sem:
            return await fetch(i)
    print(await asyncio.gather(*(bounded(i) for i in range(5))))

asyncio.run(main())
```

Rules that still apply in 3.13:

- Use `asyncio.run()` (or `asyncio.Runner`) as the entry point; inside coroutines use `asyncio.get_running_loop()`.
- **Keep references to tasks** created with `asyncio.create_task()` — the loop holds only weak references; prefer `TaskGroup`.
- Always re-raise `CancelledError` after cleanup; never swallow it.
- `asyncio.get_event_loop()` with no current loop emits `DeprecationWarning` in 3.12/3.13 and raises `RuntimeError` in 3.14 — fix callers now.[^28]

##### New in 3.13: `TaskGroup` Cancellation Fixes

`TaskGroup` handles an **external cancellation colliding with an internal one** correctly: when a task group is cancelled from outside and must also raise an `ExceptionGroup`, it now calls the parent task's `cancel()` so the cancellation is not lost; nested task groups that fail simultaneously no longer hang; and the task's cancellation count (`cancelling()`) is preserved. `TaskGroup.create_task()` on an inactive group now **closes** the coroutine (no "never awaited" warning) and raises `RuntimeError`.[^1]

```python
import asyncio

async def main() -> None:
    tg = asyncio.TaskGroup()
    async with tg:
        pass
    async def job() -> None: ...
    try:
        tg.create_task(job())          # group already finished
    except RuntimeError as e:
        print('RuntimeError:', e)      # coroutine is closed for you in 3.13

asyncio.run(main())
```

Point releases also fixed `TaskGroup` corner cases (eager tasks: missed cancellations fixed in 3.13.2; reference cycles in raised exceptions fixed in 3.13.1) — another reason to run the latest patch.[^17]

##### New in 3.13: `as_completed()` Is an Async Iterator

`asyncio.as_completed()` can now be used with `async for`, and it yields the **original task/future objects** you passed in, so you can map results back to their inputs. Plain `for` iteration (yielding new awaitables) still works.[^20]

```python
import asyncio

async def work(n: int) -> int:
    await asyncio.sleep(0.01 * (3 - n))
    return n

async def main() -> None:
    tasks = {asyncio.create_task(work(n)): f'job-{n}' for n in range(3)}
    async for task in asyncio.as_completed(tasks):
        print(tasks[task], await task)     # original Task objects: dict lookup works
    # job-2 2 / job-1 1 / job-0 0

asyncio.run(main())
```

##### Other asyncio Changes in 3.13

- `asyncio.Queue.shutdown()` / `asyncio.QueueShutDown` (see the queue section).
- `Server.close_clients()` and `Server.abort_clients()` close connections forcefully; `loop.create_unix_server()` now removes the socket file on close.
- `StreamReader.readuntil()` accepts a **tuple of separators**.
- `DatagramTransport.sendto(b'')` now sends a zero-length datagram.
- **Point-release regression to know about:** 3.13.3 accidentally changed how `create_task()` passes keyword arguments to custom task factories; **3.13.4 restored the old factory contract**. If you implement a custom task factory, test against ≥ 3.13.4.[^1][^17]
- Child watchers (`asyncio.get_child_watcher()`, `SafeChildWatcher`, ...) are deprecated and **removed in 3.14**.

***

#### Concurrency

##### The GIL in 3.13

The default build still has a GIL: threads give concurrency for I/O, not CPU parallelism. For CPU-bound pure-Python work use `ProcessPoolExecutor`/`multiprocessing`, or experiment with the free-threaded `python3.13t` build (see above). Subinterpreters with a per-interpreter GIL (PEP 684) are reachable only through the C API and the private, unsupported `_interpreters` module in 3.13 — the public `concurrent.interpreters` module and `InterpreterPoolExecutor` are 3.14.

##### `concurrent.futures`

```python
from concurrent.futures import ProcessPoolExecutor, ThreadPoolExecutor, as_completed

def square(n: int) -> int:
    return n * n

def main() -> None:
    # Default max_workers in 3.13: min(32, os.process_cpu_count() + 4) for threads,
    # os.process_cpu_count() for processes.
    with ThreadPoolExecutor() as pool:
        futures = {pool.submit(square, n): n for n in range(5)}
        results = {futures[f]: f.result() for f in as_completed(futures)}
    print(dict(sorted(results.items())))

    with ProcessPoolExecutor(max_workers=2) as pool:
        print(list(pool.map(square, range(5))))

if __name__ == '__main__':   # ✅ ALL top-level work behind the guard: spawn/forkserver re-import __main__
    main()
```

Keep *all* side-effecting module-level code behind `if __name__ == '__main__':`, not just the pool creation. With `fork` (the 3.13 Linux default) unguarded top-level code happens to run once; with `spawn` (macOS, Windows) or `forkserver` (Linux default from 3.14) every worker re-imports the main module and re-runs it.

##### `multiprocessing` Start Methods

On Linux the default start method in 3.13 is still **`fork`** (it changes to `forkserver` in 3.14; macOS has used `spawn` since 3.8, Windows always `spawn`). Since 3.12, calling `os.fork()` (including via the `fork` start method) in a process that has **other threads running** emits a `DeprecationWarning` because the child can deadlock.[^31]

```python
import multiprocessing as mp

def work(x: int) -> int:
    return x * 2

if __name__ == '__main__':               # REQUIRED for spawn/forkserver
    ctx = mp.get_context('spawn')        # ✅ explicit: same behaviour on every OS and on 3.14
    with ctx.Pool(2) as pool:
        print(pool.map(work, range(4)))
    print(mp.get_start_method())         # 'fork' on Linux 3.13
```

**Review rule:** code that *depends* on `fork` semantics (inheriting globals, open sockets, unpicklable closures) must request `get_context('fork')` explicitly, or it will break on 3.14. Code that mixes threads and `fork` should move to `spawn`/`forkserver` now.

##### `threading`

```python
import threading

lock = threading.Lock()
results: list[int] = []

def worker(n: int) -> None:
    value = n * n
    with lock:
        results.append(value)

threads = [threading.Thread(target=worker, args=(i,)) for i in range(4)]
for t in threads: t.start()
for t in threads: t.join()
print(sorted(results))   # [0, 1, 4, 9]
```

Deprecated camelCase aliases (`currentThread()`, `activeCount()`, `Thread.setDaemon()`, `Condition.notifyAll()`, `Event.isSet()`) still exist in 3.13 but should be replaced (`current_thread()`, `active_count()`, `daemon=`, `notify_all()`, `is_set()`).

***

#### Logging

```python
import logging

def setup_logging(level: str = 'INFO') -> None:
    logging.basicConfig(
        level=level.upper(),
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        force=True,
    )

setup_logging()
logger = logging.getLogger(__name__)

item, count = {'id': 1}, 3
logger.info('Loaded %d records for %r', count, item)   # ✅ lazy %-style args
# logger.info(f'Loaded {count} records')                # ❌ formats even when filtered

try:
    1 / 0
except ZeroDivisionError:
    logger.exception('Computation failed')              # includes traceback
```

`logger.warn()` is a deprecated alias of `warning()`. `logging.getHandlerByName()` and `getHandlerNames()` (3.12+) help when configuring handlers by name. 3.13 did not change the logging API in ways that affect typical application code.

***

#### Security

##### New in 3.13: Stricter TLS Defaults

`ssl.create_default_context()` now sets **`VERIFY_X509_STRICT`** and **`VERIFY_X509_PARTIAL_CHAIN`** by default. Strict mode rejects malformed or pre-RFC-5280 certificates that OpenSSL would otherwise accept (e.g. certificates missing required extensions — common with home-made internal CAs); partial-chain lets a trusted *intermediate* in the trust store terminate the chain.[^1][^21]

```python
import ssl

ctx = ssl.create_default_context()
print(bool(ctx.verify_flags & ssl.VERIFY_X509_STRICT))         # True (False on 3.12)
print(bool(ctx.verify_flags & ssl.VERIFY_X509_PARTIAL_CHAIN))  # True
print(ctx.minimum_version is ssl.TLSVersion.TLSv1_2, ctx.check_hostname, ctx.verify_mode == ssl.CERT_REQUIRED)
# True True True

# If an internal service fails ONLY on 3.13 with a verification error, fix the
# certificate (proper CA, AKI/SKI extensions). Disabling strict mode is a last resort:
legacy_ctx = ssl.create_default_context()
legacy_ctx.verify_flags &= ~ssl.VERIFY_X509_STRICT   # ⚠️ document why, scope narrowly
```

`urllib.request`, `http.client` and most libraries that call `create_default_context()` inherit these defaults. **Never** "fix" a 3.13 TLS error with `CERT_NONE`/`check_hostname = False`.

**Point releases:** 3.13.16 makes `SSLContext.wrap_bio()` raise `ValueError` when `check_hostname` is enabled but no `server_hostname` is given (previously the handshake silently skipped the identity check), and `asyncio`'s `start_tls()`/`create_connection()` validate `server_hostname` the same way.[^17]

##### `tarfile` Extraction Filters

Since 3.12 (and security releases of older versions), `TarFile.extractall()`/`extract()` accept `filter=`. In 3.13, **omitting `filter` emits `DeprecationWarning`** and performs a fully trusting extraction; 3.14 makes `'data'` the default.[^22]

```python
import io
import tarfile
import tempfile

buf = io.BytesIO()
with tarfile.open(fileobj=buf, mode='w') as tf:
    payload = b'pwned'
    info = tarfile.TarInfo('../evil.txt')        # path traversal attempt
    info.size = len(payload)
    tf.addfile(info, io.BytesIO(payload))

with tempfile.TemporaryDirectory() as dest, tarfile.open(fileobj=io.BytesIO(buf.getvalue())) as tf:
    try:
        tf.extractall(dest, filter='data')       # ✅ always pass filter='data' for untrusted archives
    except tarfile.OutsideDestinationError as e:
        print('blocked:', type(e).__name__)
```

The `data` filter blocks absolute paths, `..` escapes, links outside the destination, device files, and strips dangerous permission bits. Multiple bypasses of these filters were fixed in point releases — **3.13.4** (CVE-2024-12718, CVE-2025-4138, CVE-2025-4330, CVE-2025-4435, CVE-2025-4517; adds `tarfile.LinkFallbackError` and `os.path.ALLOW_MISSING`), and further hardening in **3.13.14, 3.13.15 and 3.13.16** (symlink/hardlink tricks, directories created outside the destination). Filters are only as good as the patch level you run.[^1][^17]

For ZIP files, `zipfile.ZipFile.extractall()` sanitizes member names, but still validate sizes (zip bombs) for untrusted input; `shutil.unpack_archive()` on Windows had a drive-prefix traversal fixed in 3.13.14.[^17]

##### Security Fixes in 3.13 Point Releases (selection)

From the official changelog:[^17]

- **3.13.1:** new REPL no longer puts the current directory on `sys.path`; IPv4-mapped `IPv6Address` properties (`is_global`, `is_private`, ...) fixed.
- **3.13.2:** `urlsplit()`/`urlparse()` reject square brackets in non-IPv6 hosts; `email` header folding of quoted strings fixed (header spoofing); `imaplib` memory DoS fixed.
- **3.13.3:** `email` RFC 2047 folding fix (header spoofing); `SpooledTemporaryFile.writelines()` no longer buffers unboundedly.
- **3.13.4:** `tarfile` filter bypasses (five CVEs, above); use-after-free in the `unicode-escape` decoder; long-IPv6-address DoS in `ipaddress`.
- **3.13.6 / 3.13.8 / 3.13.10:** `html.parser.HTMLParser` aligned with HTML5 for comments, CDATA, script/style raw text (sanitizer bypass risk) and quadratic-time inputs; `os.path.expandvars()` quadratic complexity; `plistlib` memory DoS.
- **3.13.11:** memory-exhaustion DoS in `http.client` (malicious server) fixed.
- **3.13.12:** `email` generator refuses to serialize unsafely folded headers (header injection); control characters rejected in `http.cookies.Morsel`, `wsgiref.headers` and `data:` URL media types.
- **3.13.13:** `webbrowser.open()` rejects URLs with leading dashes; `wsgiref` rejects control characters in status lines; `SourcelessFileLoader` uses `io.open_code()` (CVE-2026-2297); deep-nesting crash in `xml.parsers.expat` fixed.
- **3.13.14:** `shutil.move()` symlink guard bypass, `configparser` ReDoS, `unicodedata.normalize()` quadratic DoS, CR/LF injection in `HTTPConnection.set_tunnel()`, `http.cookies` `js_output()` injection, bundled libexpat 2.8.x.
- **3.13.15:** `http.client` caps chunked trailers and 1xx responses at 100 (infinite-stream hang), `configparser` multi-line value line endings normalized, IMAP command injection (CR/LF/NUL) rejected, `html.parser` quadratic DoS.
- **3.13.16:** `ssl.wrap_bio()` hostname check (above), `tarfile` hard-link-to-symlink escape (CVE-2026-82049), `zipfile` bounded decompression for bzip2/LZMA members, `urllib.request` HTTP credentials scoped to the URL scheme, `ssl` SNI-callback crash.
- **3.13.17:** bundled pip upgraded (certifi CA bundle update). Upgrade pip/uv inside existing virtual environments too.[^3]

##### Input Validation and Path Traversal

```python
from pathlib import Path

def safe_resolve(base_dir: str, user_path: str) -> Path:
    base = Path(base_dir).resolve()
    target = (base / user_path).resolve()
    if not target.is_relative_to(base):        # lexical check — resolve() first!
        raise PermissionError(f'Access denied: {user_path!r}')
    return target

print(safe_resolve('/srv/app', 'static/logo.png'))
try:
    safe_resolve('/srv/app', '../../etc/passwd')
except PermissionError as e:
    print(e)
```

##### Secrets and Cryptography

```python
import hashlib
import hmac
import secrets

token = secrets.token_urlsafe(32)            # session tokens, reset links
api_key = secrets.token_hex(32)
print(secrets.compare_digest(token, token))  # constant-time comparison

# Password hashing: the crypt module is GONE in 3.13. Use a memory-hard KDF:
salt = secrets.token_bytes(16)
digest = hashlib.scrypt(b'correct horse', salt=salt, n=2**14, r=8, p=1, dklen=32)
print(len(digest))                            # 32
# (or argon2-cffi / bcrypt from PyPI)

sig = hmac.new(b'key', b'message', hashlib.sha256).hexdigest()
```

Never use `random` for security values — it is a deterministic Mersenne Twister.

##### SQL Injection, Subprocess, XML

```python
import sqlite3
import subprocess

conn = sqlite3.connect(':memory:')
conn.execute('CREATE TABLE users (id INTEGER, name TEXT)')
user_id = 1
# ❌ conn.execute(f'SELECT * FROM users WHERE id = {user_id}')
rows = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,)).fetchall()   # ✅
conn.close()

# ✅ list form, no shell
result = subprocess.run(['echo', 'hello'], capture_output=True, text=True, timeout=30, check=True)
print(result.stdout.strip())
# ❌ subprocess.run(f'ls {user_input}', shell=True) — command injection
```

For untrusted XML use `defusedxml`. The bundled libexpat in 3.13 has protections against billion-laughs (expat ≥ 2.4.1) and gets hash-flooding hardening with expat ≥ 2.8.0 (3.13.14+), but external entity and DTD processing rules still differ per API — `defusedxml` remains the reviewer-safe default for untrusted input.[^17]

***

#### JSON and Serialization

```python
import json
from datetime import datetime, timezone

data = {'name': 'Alice', 'tags': ['admin'], 'at': datetime(2024, 1, 1, tzinfo=timezone.utc)}

class AppEncoder(json.JSONEncoder):
    def default(self, obj: object) -> object:
        if isinstance(obj, datetime):
            return obj.isoformat()
        if isinstance(obj, set):
            return sorted(obj)
        return super().default(obj)

text = json.dumps(data, cls=AppEncoder, indent=2, ensure_ascii=False)
print(json.loads(text)['at'])   # 2024-01-01T00:00:00+00:00
```

`python -m json.tool` is the JSON CLI in 3.13 (`python -m json` is 3.14).

##### Pickle and Marshal

```python
import marshal
import pickle

print(pickle.DEFAULT_PROTOCOL)    # 4 in 3.13 (5 only from 3.14)
blob = pickle.dumps({'a': 1}, protocol=pickle.HIGHEST_PROTOCOL)   # 5

# New in 3.13: marshal can refuse code objects
try:
    marshal.dumps(compile('1', '<x>', 'eval'), allow_code=False)
except ValueError as e:
    print('ValueError:', e)       # marshalling code objects is disallowed
```

**Never unpickle untrusted data** — `pickle.loads()` executes arbitrary code. `marshal` is not a secure or stable format either; `allow_code=False` only limits what it handles.

***

#### CSV

```python
import csv
import io

buf = io.StringIO(newline='')
writer = csv.DictWriter(buf, fieldnames=['name', 'age'])
writer.writeheader()
writer.writerow({'name': 'Alice', 'age': 30})
buf.seek(0)
for row in csv.DictReader(buf):
    print(row)   # {'name': 'Alice', 'age': '30'} — values are always str
# With files: open(path, 'r', encoding='utf-8', newline='')
```

CSV injection: when exporting user data for spreadsheets, neutralize cells starting with `=`, `+`, `-`, `@`.

***

#### Regular Expressions

```python
import re

DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')
m = DATE_RE.match('2024-06-15')
if m:
    print(m['year'], m.groupdict())

# New in 3.13: re.PatternError is the new name of re.error (alias kept)
try:
    re.compile('(unclosed')
except re.PatternError as e:
    print('PatternError:', e.msg)
print(re.PatternError is re.error)   # True

# Deprecated in 3.13: passing count/maxsplit/flags POSITIONALLY to module-level functions
print(re.sub(r'a', 'b', 'aaa', count=1))        # ✅ 'baa'
print(re.split(r',', 'a,b,c', maxsplit=1))      # ✅ ['a', 'b,c']
# re.sub(r'a', 'b', 'aaa', 1)                   # ❌ DeprecationWarning: 'count' is passed as positional argument
# re.sub(r'a', 'b', 'aaa', re.IGNORECASE)       # ❌ classic bug: flags passed as COUNT
```

The positional-argument deprecation catches a real bug class: `re.sub(pattern, repl, s, re.I)` silently passes `re.I` (value 2) as `count`. Always use keywords.

The internal `sre_compile`, `sre_constants` and `sre_parse` modules are deprecated (removed in 3.15) — do not import them.

***

#### `random` and `statistics`

##### New in 3.13: `python -m random` CLI

The `random` module gained a command-line interface for quick choices and numbers.[^26]

```bash
python -m random -c red green blue   # random choice
python -m random -i 6                # random integer 1..6
python -m random -f 1.5              # random float 0..1.5
python -m random 6                   # auto: integer input -> --integer
python -m random a b c               # auto: strings -> --choice
```

```python
import random
import secrets

rng = random.Random(42)              # reproducible simulations/tests
print(rng.randint(1, 6))
print(len(random.randbytes(16)))     # 16 — NOT for security; use secrets.token_bytes()
print(len(secrets.token_bytes(16)))
```

##### New in 3.13: `statistics.kde()` and `kde_random()`

```python
import statistics

samples = [1.0, 2.0, 2.5, 4.0]
pdf = statistics.kde(samples, h=0.5)            # kernel density estimate (default kernel 'normal')
print(round(pdf(2.0), 4))                        # 0.3475
draw = statistics.kde_random(samples, h=0.5, seed=1)
print(isinstance(draw(), float))                 # True
```

***

#### Databases: `sqlite3` and `dbm`

##### `sqlite3` Changes in 3.13

```python
import contextlib
import sqlite3

# ✅ Close connections explicitly — 3.13 emits ResourceWarning for unclosed connections
with contextlib.closing(sqlite3.connect(':memory:', timeout=5.0)) as conn:   # keyword args
    with conn:                                   # transaction: commit/rollback, does NOT close
        conn.execute('CREATE TABLE t (x INTEGER)')
        conn.executemany('INSERT INTO t VALUES (?)', [(1,), (2,)])
    print(conn.execute('SELECT sum(x) FROM t').fetchone())   # (3,)
    dump = list(conn.iterdump(filter='t'))       # New in 3.13: filter= for iterdump
    print(dump[1])                               # CREATE TABLE t (x INTEGER);
```

- **`with conn:` manages a transaction, it does not close the connection** — combine with `contextlib.closing()` or call `close()`; 3.13 now warns (`ResourceWarning`) when a connection is garbage-collected unclosed.[^32]
- **Deprecated in 3.13** (become errors in 3.15): more than one positional argument to `sqlite3.connect()` (`connect(db, 5.0)` → use `timeout=5.0`); passing `name`/`narg`/`func` by keyword to `create_function()`/`create_aggregate()`; passing the callback by keyword to `set_authorizer()`, `set_progress_handler()`, `set_trace_callback()`.
- Deprecated (removed in 3.14): `sqlite3.version`/`version_info` (the module version, not SQLite's — use `sqlite3.sqlite_version`), and named placeholders (`:name`) with a sequence of parameters.

##### New in 3.13: `dbm.sqlite3` Is the Default `dbm` Backend

`dbm.open(path, 'c')` creates a new **SQLite**-backed database by default (previously `dbm.gnu`/`dbm.ndbm`/`dbm.dumb` depending on the platform). Existing files are still opened with whatever format `dbm.whichdb()` detects.[^23]

```python
import dbm
import os
import tempfile

with tempfile.TemporaryDirectory() as d:
    path = os.path.join(d, 'cache')
    with dbm.open(path, 'c') as db:
        db['key'] = 'value'               # str keys/values are encoded to bytes
        print(db['key'])                  # b'value'
    print(dbm.whichdb(path))              # dbm.sqlite3
```

Consequences: files are no longer interchangeable with the same code on 3.12 (a 3.12 interpreter cannot open a `dbm.sqlite3` file — the module does not exist there); `shelve` (built on `dbm`) inherits the new default; GDBM/NDBM objects gained `clear()`. If you need a specific format for interoperability, open it explicitly (`dbm.gnu.open(...)`, `dbm.dumb.open(...)`).

***

#### Command-Line Interfaces: `argparse` and `optparse`

##### New in 3.13: `deprecated=` for Options, Positionals and Subcommands

`add_argument()` and `add_parser()` accept `deprecated=True`; using the option or subcommand prints a warning to stderr but still parses it.[^25]

```python
import argparse
import contextlib
import io

parser = argparse.ArgumentParser(prog='tool')
parser.add_argument('--old-name', deprecated=True, help=argparse.SUPPRESS)
parser.add_argument('--new-name')
sub = parser.add_subparsers(dest='cmd')
sub.add_parser('legacy-sync', deprecated=True)

err = io.StringIO()
with contextlib.redirect_stderr(err):
    args = parser.parse_args(['--old-name', 'x'])
print(err.getvalue().strip())     # tool: warning: option '--old-name' is deprecated
print(args.old_name)              # x — still parsed; only a warning is printed
```

Other notes:

- The `type`, `choices` and `metavar` parameters of `argparse.BooleanOptionalAction` are deprecated (removed in 3.14).
- Nesting argument groups / mutually exclusive groups is deprecated.
- **`optparse` and `getopt` are not deprecated** (any more): 3.13.2 dropped the "soft deprecated" marking. `argparse` remains the recommended choice for new applications, but `optparse` is legitimate where its stricter POSIX behaviour matters.[^17]
- `argparse` features like `suggest_on_error=` and colourised help are 3.14 — not available.

***

#### Memory Management

```python
import gc
import sys
import tracemalloc
import weakref

x = [1, 2, 3]
print(sys.getrefcount(x) >= 2)          # refcount includes the argument reference

print(gc.get_threshold())               # (2000, 10, 10) in 3.13 — was (700, 10, 10) in 3.12

tracemalloc.start()
blob = [bytes(1000) for _ in range(100)]
current, peak = tracemalloc.get_traced_memory()
print(current > 0, peak >= current)
tracemalloc.stop()

class Node:
    pass

n = Node()
ref = weakref.ref(n)
del n
print(ref() is None)                    # True — collected (refcount reached zero)
```

3.13 facts:

- The **incremental garbage collector was reverted before 3.13.0** (in 3.13.0rc3) — 3.13 uses the classic generational collector. (3.14.0–3.14.4 shipped an incremental GC, which was reverted again in 3.14.5.) Don't attribute GC pause behaviour in 3.13 to an incremental collector, and don't recommend tuning for one.[^17][^28]
- CPython **bundles mimalloc** in 3.13. The default build still uses `pymalloc` by default; `PYTHONMALLOC=mimalloc` opts in. The free-threaded build always uses mimalloc.[^27]
- In the free-threaded build, some objects become immortal (see the free-threading section) — monitor memory if you generate functions/classes dynamically.
- The `IOBase` finalizer now reports exceptions from `close()` via `sys.unraisablehook` instead of silently ignoring them — unclosed file objects whose `close()` fails now produce visible "Exception ignored" messages.[^1]

***

#### Performance

```python
import cProfile
import io
import pstats
import timeit

def work() -> int:
    return sum(i * i for i in range(10_000))

with cProfile.Profile() as pr:
    work()
stream = io.StringIO()
pstats.Stats(pr, stream=stream).sort_stats('cumulative').print_stats(5)
print(timeit.timeit(work, number=10) > 0)

VALID = frozenset({'admin', 'editor'})   # O(1) membership
parts = ''.join(str(i) for i in range(10))   # join, never += in loops
```

What 3.13 changes for performance:[^1]

- **Import time:** `typing` imports about a third faster; `email.utils`, `enum`, `functools`, `importlib.metadata`, `threading` also faster to import.
- `textwrap.indent()` ~30% faster for large input; `subprocess` uses `posix_spawn()` more often (faster process launch, notably FreeBSD/Solaris).
- Windows: `time.monotonic()` and `time.time()` have ~1 µs resolution (were ~15.6 ms).
- **No free speedup from the JIT or free threading** in standard deployments (see above). The free-threaded build is ~40% slower single-threaded.
- `perf` profiler support without frame pointers: `PYTHON_PERF_JIT_SUPPORT=1` / `-X perf_jit`.

***

#### Testing

```python
import unittest
from unittest.mock import AsyncMock, patch

class TestMath(unittest.TestCase):
    def test_add(self) -> None:
        self.assertEqual(1 + 1, 2)

class TestAsync(unittest.IsolatedAsyncioTestCase):
    async def test_fetch(self) -> None:
        fetch = AsyncMock(return_value={'id': 1})
        result = await fetch(1)
        fetch.assert_awaited_once_with(1)
        self.assertEqual(result['id'], 1)

# 3.13: unittest.makeSuite / findTestCases / getTestCaseNames are REMOVED
loader = unittest.TestLoader()
suite = unittest.TestSuite()
suite.addTests(loader.loadTestsFromTestCase(TestMath))      # ✅ replaces makeSuite(TestMath)
suite.addTests(loader.loadTestsFromTestCase(TestAsync))
result = unittest.TextTestRunner(verbosity=0).run(suite)
print(result.wasSuccessful())   # True
```

```python
# pytest style (third-party)
import pytest

@pytest.mark.parametrize(('items', 'expected'), [([1, 2, 3], 6), ([], 0)])
def test_sum(items: list[int], expected: int) -> None:
    assert sum(items) == expected

def test_warns_deprecated() -> None:
    from warnings import deprecated

    @deprecated('old')
    def old() -> None: ...

    with pytest.deprecated_call():
        old()
```

3.13 testing notes:[^1]

- **Run the suite with warnings as errors** (`python -W error::DeprecationWarning -m pytest`, or pytest's `filterwarnings = ["error"]`) — 3.13 adds many new `DeprecationWarning`s (`re` positional args, `sqlite3` positional args, `ast` node constructors, `NamedTuple` keywords) that become errors in 3.14, 3.15 or later.
- **Docstring dedent** changes `__doc__`; tests asserting on raw docstrings need updating.
- `doctest` output is coloured by default in terminals (`PYTHON_COLORS=0` to disable); `DocTestRunner` now counts skipped tests (`TestResults.skipped`).
- Tests that assert exact error/traceback text may break on the improved messages.
- Returning a non-`None` value from a test method (sync or async) triggers `DeprecationWarning` — usually a forgotten `await` or a test written as a generator.
- `ResourceWarning` for unclosed `sqlite3` connections shows up under `-X dev` / `-W default`; fix the leak rather than filtering it.

***

#### Virtual Environments and Packaging

```bash
python3.13 -m venv .venv
source .venv/bin/activate          # Unix/macOS
.venv\Scripts\activate             # Windows
python -m pip install -U pip       # especially after the 3.13.17 certifi update

# uv: fast resolver/installer that also manages interpreters
uv venv --python 3.13
uv pip install -r requirements.txt
uv python install 3.13t            # experimental free-threaded interpreter
```

**New in 3.13: `venv` writes a `.gitignore`** containing `*` into the environment directory, so the venv is ignored by Git even without a project-level ignore rule. Opt out with `--without-scm-ignore-files`; the `venv.EnvBuilder(scm_ignore_files=...)` API is opt-in.[^24]

```text
$ python3.13 -m venv .venv && cat .venv/.gitignore
# Created by venv; see https://docs.python.org/3/library/venv.html
*
```

Other packaging facts for 3.13:

- `distutils` is gone (removed in 3.12): use `setuptools` (which provides its own `distutils` shim) or another PEP 517 backend. `imp`, `asynchat`, `asyncore` and `smtpd` were also removed in 3.12 — a codebase jumping from 3.11 to 3.13 hits those *and* the PEP 594 removals.
- `.pth` files are now decoded as UTF-8 first (locale encoding as fallback), and `.pth` files whose names start with a dot (hidden files) are skipped.[^1]
- Free-threaded wheels use the `cp313t` ABI tag; a `cp313` wheel will not install into `python3.13t`. The free-threaded build does **not** support the Limited C API / stable ABI, so `abi3` wheels cannot serve it — projects need separate `cp313t` builds.[^34]
- The `PYTHON_CPU_COUNT` / `-X cpu_count` override helps CI containers that report the host CPU count.

##### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=69"]
build-backend = "setuptools.build_meta"

[project]
name = "myapp"
version = "1.0.0"
requires-python = ">=3.13"
dependencies = ["requests>=2.32"]

[tool.ruff]
target-version = "py313"
line-length = 88

[tool.ruff.lint]
select = ["E", "F", "I", "UP", "B", "S", "PTH"]

[tool.mypy]
python_version = "3.13"
strict = true

[tool.pytest.ini_options]
testpaths = ["tests"]
filterwarnings = ["error"]
```

**`requires-python` drives what syntax is allowed.** If it says `>=3.11`, code must not use 3.12 PEP 695 syntax or 3.13 `[T = int]` defaults, `warnings.deprecated`, `copy.replace`, etc. — use `typing_extensions` backports (`TypeIs`, `ReadOnly`, `deprecated`, `TypeVar(default=...)`) in that case.

##### Project Layout

```
project/
├── .venv/              # self-ignored via .venv/.gitignore (3.13 venv)
├── src/
│   └── myapp/
│       ├── __init__.py
│       └── main.py
├── tests/
│   └── test_main.py
└── pyproject.toml
```

***

#### Code Quality Tools

```bash
pip install ruff mypy bandit

ruff check --target-version py313 src/ tests/   # lint, incl. pyupgrade (UP) rules
ruff format src/ tests/                         # formatter
mypy --python-version 3.13 --strict src/
bandit -r src/ -q

# Find code that breaks on 3.13 before deploying
python3.13 -W error::DeprecationWarning -X dev -m pytest
python3.13 -X warn_default_encoding -m pytest   # open() without encoding=
grep -rnE "^\s*(import|from) (cgi|cgitb|crypt|imghdr|sndhdr|pipes|telnetlib|nntplib|uu|xdrlib|mailcap|aifc|audioop|chunk|sunau|spwd|nis|msilib|ossaudiodev|lib2to3)\b" src/
```

Ruff's `UP` (pyupgrade) rules rewrite `Optional[X]` → `X | None`, `typing.List` → `list` and other outdated idioms for the configured target version. Type checkers need a version that understands PEP 696/705/742 and `@deprecated` to benefit from the 3.13 typing features; with an older checker, they are silently treated as `Any`-like or flagged as unknown.

***

#### Platform Support Changes in 3.13

- **iOS (PEP 730)** and **Android (PEP 738)** are officially supported at **tier 3** (iOS: `arm64-apple-ios`, `arm64-apple-ios-simulator`; Android: `aarch64-linux-android`, `x86_64-linux-android`).[^30]
- `wasm32-wasi` is now **tier 2**; `wasm32-emscripten` is **no longer** officially supported (it returns as tier 3 in 3.14).
- Building CPython requires a C11 compiler with atomics; SQLite ≥ 3.15.2 for `sqlite3`.
- Many modules are unavailable on mobile/WASM (`subprocess`, `multiprocessing`, `curses`, ...) — guard optional imports if you target them, and check `sys.platform` (`'ios'`; on Android it is now `'android'` instead of `'linux'`).

***

#### Removed and Changed in 3.13 — Migration Notes

**Removed modules (import fails):** `aifc`, `audioop`, `cgi`, `cgitb`, `chunk`, `crypt`, `imghdr`, `mailcap`, `msilib`, `nis`, `nntplib`, `ossaudiodev`, `pipes`, `sndhdr`, `spwd`, `sunau`, `telnetlib`, `uu`, `xdrlib`, `lib2to3` (and the `2to3` tool), `tkinter.tix`.[^1]

**Removed APIs:**

| Removed in 3.13 | Replacement |
|---|---|
| `locale.resetlocale()` | `locale.setlocale(locale.LC_ALL, '')` |
| `typing.io`, `typing.re` | import names from `typing` / use `re.Pattern` |
| `TypedDict('TD', a=int)` | `TypedDict('TD', {'a': int})` or class syntax |
| `unittest.makeSuite()` / `findTestCases()` / `getTestCaseNames()` | `unittest.TestLoader().loadTestsFromTestCase()` / `loadTestsFromModule()` / `getTestCaseNames()` |
| `urlopen(cafile=, capath=, cadefault=)` | `urlopen(url, context=ssl.create_default_context(cafile=...))` |
| `with Path(...):` | nothing (it was a no-op) |
| chained `@classmethod` + `@property` | class attribute, explicit classmethod, or metaclass property |
| `re.template()`, `re.TEMPLATE`/`re.T` | normal `re` functions |
| `configparser.LegacyInterpolation` | `BasicInterpolation`/`ExtendedInterpolation` |
| `importlib.metadata` `EntryPoint[...]` subscripting | attribute access (`ep.name`, `ep.value`) |
| `webbrowser.MacOSX` | `webbrowser.MacOSXOSAScript` |
| `turtle.RawTurtle.settiltangle()` | `tiltangle()` |

**Behaviour changes to know:**[^1]

- `locals()` in functions returns snapshots; `exec()`/`eval()` without an explicit namespace can't modify function locals (PEP 667).
- Docstrings have common indentation stripped.
- `Path.glob('**')` returns files as well as directories.
- `ssl.create_default_context()` enables `VERIFY_X509_STRICT` and `VERIFY_X509_PARTIAL_CHAIN`.
- `dbm.open(..., 'c')` creates `dbm.sqlite3` files.
- `functools.partial` accessed as a method emits `FutureWarning`.
- `getpass.getuser()` raises `OSError` for every failure (was `ImportError`/`KeyError`).
- `gzip.GzipFile.mode` is a string (`'rb'`/`'wb'`), and the read-mode `mode` of `ZipFile.open()` objects is `'rb'`.
- `email.utils.getaddresses()`/`parseaddr()` are strict by default (return `('', '')` for malformed input; `strict=False` for old behaviour). Also backported to security releases of older branches — check `email.utils.supports_strict_parsing`.
- `email` generator refuses to serialize headers that would be parsed as multiple headers (`HeaderWriteError`; `verify_generated_headers` policy flag) — likewise backported as a security fix.
- `ast` node constructors: omitted optional fields default to `None`/`[]`/`Load()`; omitting required fields or passing unknown keywords emits `DeprecationWarning` (error in 3.15).
- `ipaddress` `is_private`/`is_global` corrected for many special ranges.
- `mailbox.Maildir` ignores files starting with `.`.
- `breakpoint()`/`pdb.set_trace()` stop immediately (not at the next line).
- `frame.clear()` on a suspended frame raises `RuntimeError`.
- `os.fork()`, `subprocess.Popen`, thread creation during finalization raise `PythonFinalizationError`.
- Many functions now emit `RuntimeWarning: bool is used as a file descriptor` when given `True`/`False` as an fd (e.g. `open(False)`).
- `ctypes` internal metaclasses initialise in `__init__` instead of `__new__` (affects code subclassing them).

```python
import ast
import email.utils
import getpass
import gzip
import io

print(ast.dump(ast.Name('x')))                      # Name(id='x', ctx=Load()) — ctx defaults to Load()
print(email.utils.parseaddr('alice@example.org)<bob@example.org>'))        # ('', '')
print(email.utils.parseaddr('alice@example.org)<bob@example.org>', strict=False))  # ('', 'alice@example.org')
with gzip.GzipFile(fileobj=io.BytesIO(), mode='wb') as g:
    print(repr(g.mode))                             # 'wb' (was 2)
```

##### Deprecations to Fix Now (Removed in 3.14)

These all emit `DeprecationWarning` in 3.13 and are **gone in 3.14**:[^1][^28]

| Deprecated (3.13 warns) | Fix |
|---|---|
| `ast.Num`, `ast.Str`, `ast.Bytes`, `ast.NameConstant`, `ast.Ellipsis` | `ast.Constant` |
| `asyncio` child watchers, `get_child_watcher()`/`set_child_watcher()` | nothing needed (default handles subprocesses) |
| `pkgutil.find_loader()` / `get_loader()` | `importlib.util.find_spec()` |
| `importlib.abc.ResourceReader`/`Traversable`/`TraversableResources` | `importlib.resources.abc` |
| `sqlite3.version`, `sqlite3.version_info` | `sqlite3.sqlite_version` |
| sqlite3 named placeholders with a sequence of params | pass a `dict` |
| `bool(NotImplemented)` | compare explicitly; return `NotImplemented` only from binary dunders |
| `copy`/`deepcopy`/`pickle` of `itertools` objects | materialise data first |
| `pty.master_open()` / `slave_open()` | `pty.openpty()` |
| `email.utils.localtime(isdst=...)` | drop the argument |
| `argparse.BooleanOptionalAction(type=, choices=, metavar=)` | drop them |
| `urllib.parse.Quoter` | `urllib.parse.quote()` |
| extra args to `PurePath.relative_to()`/`is_relative_to()` | join the path first |

**Removed in 3.15** (deprecated in 3.13): `NamedTuple` keyword syntax and omitted/`None` fields for `NamedTuple`/`TypedDict`; `typing.no_type_check_decorator`; `http.server.CGIHTTPRequestHandler` and `--cgi`; `PurePath.is_reserved()`; `platform.java_ver()`; `ctypes.SetPointerType()`; `wave` `getmark()`/`setmark()`/`getmarkers()`; positional `sqlite3.connect()` parameters after the first; `ast` constructor leniency; `glob.glob0()`/`glob1()`. Also gone in 3.15: the `sre_compile`/`sre_constants`/`sre_parse` modules (deprecated since 3.11).

**Later or unscheduled:** `array('u')` (3.16), `asyncio.iscoroutinefunction()` (deprecated in 3.14 — fine to use in 3.13, but `inspect.iscoroutinefunction()` is the future-proof choice), `typing.AnyStr` (3.18), `Decimal` `'N'` format (3.18), `datetime.utcnow()`/`utcfromtimestamp()` (use `datetime.now(UTC)`), `re` positional `count`/`maxsplit`/`flags`, `gettext` non-integer plurals, `~bool` (3.16).

```python
from datetime import UTC, datetime

now = datetime.now(UTC)                    # ✅ aware UTC timestamp (datetime.UTC since 3.11)
# datetime.utcnow()                        # ❌ naive, DeprecationWarning since 3.12
ts = datetime.fromtimestamp(0, tz=UTC)     # ✅ instead of utcfromtimestamp(0)
print(now.tzinfo, ts.isoformat())          # UTC 1970-01-01T00:00:00+00:00
```

***

#### Production Checklist

##### Python 3.13-Specific Items

- [ ] Running the **latest 3.13.x** (3.13.17 as of October 2026); 3.13 is security-fix-only and reaches end-of-life in October 2029 — plan the move to 3.14/3.15
- [ ] No imports of PEP 594 "dead battery" modules (`cgi`, `crypt`, `imghdr`, `pipes`, `telnetlib`, `uu`, ...) or `lib2to3`; replacements chosen deliberately (`standard-*` PyPI shims only as a stop-gap)
- [ ] No code relies on mutating `locals()` or on `exec()`/`eval()` writing into function locals — explicit namespace dicts are passed (PEP 667)
- [ ] Nothing compares raw `__doc__` strings against indented literals (3.13 strips docstring indentation)
- [ ] No 3.14+ features: no `t''` strings, no unparenthesized `except A, B:`, no `annotationlib`, no `compression.zstd`, no `concurrent.interpreters`, no `uuid7`, no `Path.copy()`; forward-reference annotations are quoted or the module uses `from __future__ import annotations`
- [ ] No 3.15 features: no `lazy import`, `frozendict`, `sentinel`, `[*x for x in xs]`, `typing.TypeForm`
- [ ] Type parameter defaults (`[T = int]` / `TypeVar(default=...)`) only where `requires-python >= 3.13`; otherwise imported from `typing_extensions`
- [ ] `TypeIs` used for two-sided narrowing predicates; predicate returns `True` exactly for the narrowed type
- [ ] `@warnings.deprecated` used for API deprecations (with a meaningful message and the right `category`)
- [ ] `copy.replace()` only used on types that support `__replace__`
- [ ] Worker shutdown uses `Queue.shutdown()` + `ShutDown`/`QueueShutDown` instead of sentinels where 3.13 is the minimum
- [ ] `ssl` errors on 3.13 investigated as certificate problems (VERIFY_X509_STRICT) — not "fixed" by disabling verification
- [ ] `tarfile` extraction always passes `filter='data'` (3.13 warns without it; 3.14 defaults to it)
- [ ] `multiprocessing` start method chosen explicitly where `fork` semantics matter (default becomes `forkserver` on Linux in 3.14)
- [ ] `Path.glob('**')` call sites reviewed (now also yields files); `'**/'` used where only directories are wanted
- [ ] `dbm`/`shelve` files: format expectations reviewed (new files are `dbm.sqlite3`)
- [ ] `re.sub`/`re.split`/`re.subn` pass `count=`/`maxsplit=`/`flags=` as keywords
- [ ] `sqlite3` connections closed explicitly (no `ResourceWarning`); `connect()` options passed as keywords
- [ ] `functools.partial` class attributes wrapped in `staticmethod()` (or replaced by `partialmethod`)
- [ ] `asyncio.get_event_loop()` not called without a running loop; child watchers removed
- [ ] Custom asyncio task factories tested on ≥ 3.13.4 (3.13.3 regression)
- [ ] `PythonFinalizationError` not triggered by shutdown-time thread/process creation
- [ ] Free-threaded build (`python3.13t`), if used at all, treated as experimental: shared state locked, no shared iterators, `sys._is_gil_enabled()` checked in diagnostics, all C extensions declare `Py_mod_gil`
- [ ] JIT not assumed — standard builds don't include it, or ship it switched off
- [ ] Test suite runs with `-W error::DeprecationWarning` so 3.14/3.15 removals surface now

##### Code Correctness

- [ ] `bytes` and `str` never mixed without explicit encode/decode
- [ ] `encoding='utf-8'` passed to every text `open()` (default is still the locale encoding in 3.13)
- [ ] `newline=''` for CSV files
- [ ] No mutable default arguments; dataclass mutable fields use `field(default_factory=...)`
- [ ] `__hash__` only defined on immutable classes
- [ ] `match` statements don't use bare names as constants
- [ ] `ExceptionGroup`s from `TaskGroup` handled with `except*`

##### Safety and Security

- [ ] SQL queries parameterized
- [ ] `pickle`/`marshal` never used on untrusted data
- [ ] No `shell=True` with user input; `shlex.quote()` where a shell is unavoidable
- [ ] `secrets` (not `random`) for tokens; `hashlib.scrypt`/argon2/bcrypt for passwords (no `crypt`)
- [ ] `secrets.compare_digest()` for token comparison
- [ ] `defusedxml` for untrusted XML
- [ ] Path traversal checks use `resolve()` then `is_relative_to()`
- [ ] `sys.set_int_max_str_digits(0)` never applied to untrusted input paths

##### Performance

- [ ] Profiled before optimizing (`cProfile`, `perf` with `-X perf_jit` if needed)
- [ ] CPU-bound work uses processes (or a deliberate free-threading experiment), not GIL-bound threads
- [ ] Executor/pool sizes rely on `os.process_cpu_count()` defaults or `PYTHON_CPU_COUNT` in containers
- [ ] Logging uses `%`-style lazy arguments

##### Observability

- [ ] `logging.getLogger(__name__)` in every module; no `print()` in production paths
- [ ] `logger.exception()` inside `except` blocks; bare `raise` to re-raise
- [ ] Custom exceptions inherit from `Exception`; context added with `add_note()`
- [ ] Colour output disabled in log collectors that can't render ANSI (`NO_COLOR=1` / `PYTHON_COLORS=0`)
