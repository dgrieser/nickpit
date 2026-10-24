### Python 3.11: The Complete Production Developer Guide

#### Overview

Python 3.11 was released on October 24, 2022. Its regular bugfix phase ended with 3.11.9 (April 2024), the last release with python.org binary installers; since then it gets **source-only security releases**. The latest is **3.11.17 (October 1, 2026)**, and security support ends in **October 2027** (five years after 3.11.0). As of October 2026, 3.11 is in its final security-only year. Treat it as a supported but legacy target: plan the move to 3.12+ now, and always run the newest 3.11.x patch, because several security fixes (tarfile extraction filters, `email` header injection, `http.client` limits) only exist in late point releases.

3.11 is best known for two things:

- **Speed.** The Faster CPython project (PEP 659 specializing adaptive interpreter, zero-cost exceptions, cheaper frames, inlined Python-to-Python calls) makes 3.11 about **1.25× faster than 3.10** on average (10–60% depending on workload) with no source changes.
- **Better failure handling.** It adds exception groups and `except*` (PEP 654), exception notes (PEP 678), fine-grained traceback locations (PEP 657), and structured concurrency with `asyncio.TaskGroup` and `asyncio.timeout()`.

It also adds `tomllib` (PEP 680) to the standard library, a set of typing features (`Self`, `LiteralString`, `Required`/`NotRequired`, `TypeVarTuple`, `dataclass_transform`, `Never`/`assert_never`/`reveal_type`/`assert_type`), `StrEnum` and a large `enum` overhaul, the `-P`/`PYTHONSAFEPATH` safe-path option, and a 4300-digit limit on int/str conversion (CVE-2020-10735).

This guide targets **Python 3.11 specifically**. Everything from earlier versions remains available and is used freely here:

- 3.6 f-strings, 3.7 dataclasses and guaranteed dict ordering, 3.8 walrus operator and positional-only parameters.
- 3.9 built-in generics (`list[int]`), dict `|`, `str.removeprefix()`, `zoneinfo`.
- 3.10 `match`/`case`, `X | Y` union syntax, parenthesized context managers, `zip(strict=True)`, `itertools.pairwise()`, `dataclass(slots=True, kw_only=True)`, `ParamSpec`/`TypeAlias`/`TypeGuard`, and `EncodingWarning`.

Every section that covers something new or changed in 3.11 is marked **New in 3.11**.

**Do not use anything newer than 3.11.** The following are *not* available on a 3.11 interpreter and must never be recommended for 3.11 code:

- **3.12:** PEP 695 type-parameter syntax (`def f[T](x: T)`, `class C[T]:`) and the `type X = ...` statement; PEP 701 f-strings (reusing the outer quote inside `{}`, backslashes or comments in replacement fields); `typing.override` (PEP 698); `**kwargs: Unpack[TD]` (PEP 692, a type-checker feature of 3.12); `itertools.batched()`; `pathlib.Path.walk()`; `asyncio.eager_task_factory`; `math.sumprod()`; `sys.monitoring` (PEP 669); `int.is_integer()`; `logging.getHandlerByName()`; `contextlib.suppress` catching exceptions inside an `ExceptionGroup`.
- **3.13:** `typing.TypeIs`, `typing.ReadOnly`, `warnings.deprecated()`, `copy.replace()`, type-parameter defaults (PEP 696), the free-threaded build and the JIT.
- **3.14:** template strings (`t"..."`), `annotationlib` and deferred evaluation of annotations (PEP 649), `except A, B:` without parentheses (PEP 758), `compression.zstd`, `concurrent.interpreters`, `uuid.uuid7()`.
- **3.15:** `lazy import` (PEP 810), the `frozendict` built-in (PEP 814), the `sentinel` built-in (PEP 661), `*`-unpacking in comprehensions (PEP 798), and UTF-8 mode on by default (PEP 686).

Several typing features from later versions are backported to 3.11 by the third-party `typing_extensions` package (`override`, `TypeIs`, `ReadOnly`, `deprecated`). Use them only through that import, never from `typing`.

Global truths about Python 3.11:

- **Everything is an object.** Integers, functions, classes and modules are all objects.
- **Indentation is syntax.** Blocks are delimited by indentation, not braces.
- **Dynamic typing with optional static checking.** Annotations are not enforced at runtime. A type checker (mypy, pyright) configured for 3.11 enforces them.
- **Reference counting plus a cyclic GC, and one GIL.** CPython 3.11 has a single global interpreter lock and no free-threaded build (that starts with 3.13).
- **`str` is Unicode and `bytes` is binary.** The two never mix implicitly.
- **Dicts keep insertion order.** This is a language guarantee since 3.7.
- **Annotations are evaluated eagerly** unless you write `from __future__ import annotations` (PEP 563). PEP 563 did not become the default. It was "put on hold indefinitely" in 3.11 and replaced by PEP 649 in 3.14.
- **Exceptions can now travel in groups.** Any API built on `asyncio.TaskGroup`, or one that opts into `ExceptionGroup`, raises a group that a plain `except ValueError:` does **not** catch. Review error handling around such APIs.

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

# Classes (including exception and enum classes): PascalCase
class UserAccount:
    pass

class PaymentDeclinedError(Exception):
    pass

# Internal use: single underscore prefix
class User:
    def __init__(self) -> None:
        self._internal_state: dict[str, int] = {}

# Name mangling: double underscore prefix (not true privacy)
class Base:
    def __init__(self) -> None:
        self.__private = 'name-mangled'   # stored as _Base__private

# Module-level "private": excluded from __all__
_module_cache: dict[str, bytes] = {}
```

In Python 3, every class implicitly inherits from `object`, so `class Foo:` and `class Foo(object):` are identical. Never write the latter in new code.

##### Import Ordering

Use three blocks separated by blank lines: standard library, third-party, then local. Sort within each block. `ruff check --select I` (or `isort --profile=black`) automates this.

```python
# Standard library
import asyncio
import tomllib                      # new in 3.11
from collections.abc import Callable, Iterator
from pathlib import Path
from typing import Self             # new in 3.11

# Third-party
# import httpx

# Local application
# from myapp.models import User
```

Since 3.9, import abstract collection types from `collections.abc`, not `typing`. Since 3.10, write `X | None` instead of `Optional[X]`. You no longer need `typing.List`, `typing.Dict`, `typing.Optional` or `typing.Union` in 3.11 code.

Never use wildcard imports (`from module import *`). They defeat static analysis and `__all__` hygiene.

##### Docstrings

Write Google-style docstrings for every public module, class and function. The first line is a one-sentence imperative summary.

```python
def calculate_discount(
    price: float,
    discount_percent: float,
    min_price: float = 0.0,
) -> float:
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
    discounted = price * (1 - discount_percent / 100)
    return max(discounted, min_price)


class UserService:
    """Service for managing user operations.

    Attributes:
        db: Database connection instance.
        cache: Optional cache for user lookups.
    """

    def __init__(self, db: object, cache: dict[int, str] | None = None) -> None:
        self.db = db
        self.cache = cache
```

***

#### New in 3.11: Faster CPython — What It Means for Code Review

CPython 3.11 is on average **25% faster than 3.10** on the pyperformance suite, and 10–60% faster depending on workload. Nothing has to change in your code. The speedup comes from interpreter internals:

| Change | Effect |
|---|---|
| **PEP 659 specializing adaptive interpreter** | Hot bytecode is rewritten in place into type-specialized instructions (e.g. `int + int`, `list[i]`, cached attribute and global lookups). It de-specializes when types change. |
| **"Zero-cost" exceptions** | A `try` block costs nothing when no exception is raised. Setup moved from bytecode (`SETUP_FINALLY`) to a side exception table. |
| **Cheaper, lazy frames** | Frame objects are created only when something asks for them (`sys._getframe()`, `inspect.currentframe()`, tracebacks, debuggers). |
| **Inlined Python-to-Python calls** | Most Python function calls use no C stack, so recursion is about 1.7× faster in simple cases. |
| **Frozen startup modules** | Interpreter startup is 10–15% faster. |
| **Smaller objects** | Instance `__dict__`s are created lazily and share keys more. Dicts whose keys are all `str` no longer store hashes. |

```python
import dis

def parse(x: str) -> int:
    try:
        return int(x)
    except ValueError:
        return 0

opnames = [ins.opname for ins in dis.get_instructions(parse)]
print('SETUP_FINALLY' in opnames)                  # False — the try block costs no instructions
print(hasattr(parse.__code__, 'co_exceptiontable'))  # True — handlers live in a side table
```

**Review guidance:**

- **Don't hoist `try` out of hot loops for speed.** In 3.11 the happy path of `try` is free. Choose between EAFP and LBYL for clarity and correctness. *Raising* an exception is still far more expensive than a branch, so don't use exceptions for ordinary control flow in hot paths.
- **Old micro-optimizations have less value.** Aliasing globals or methods to locals (`append = lst.append`), and avoiding attribute access in loops, now gain much less, because the specializer caches global and attribute lookups. Remove such tricks when they hurt readability. Keep them only where a benchmark proves a win.
- **Type-stable code is fastest.** The specializer works best when a given line sees the same types every time. Code that alternates between `int` and `float` at the same operation, or that mutates class attributes in hot loops, de-specializes.
- **Introspection has a cost.** Calling `sys._getframe()` or `inspect.stack()` in hot paths forces frame objects to materialize, which undoes the lazy-frame savings. Avoid it in logging adapters and decorators that run on every call.
- **Deep recursion.** Pure-Python recursion no longer consumes C stack per call, so raising `sys.setrecursionlimit()` is safer than before. The default limit is still 1000, and an iterative algorithm is still the right fix for unbounded depth.
- **Workloads that don't benefit.** Code that is I/O-bound or spends its time in C extensions (NumPy, pandas) sees little change. What's New says so explicitly.
- **Memory.** Expect at most about 20% more memory than 3.10, typically offset by the smaller frames and dicts. PEP 657 column tables also make `.pyc` files somewhat larger.
- **Bytecode tools break.** Bytecode changed heavily (new `CACHE` entries, `BINARY_OP`, `CALL`/`PRECALL`, relative jumps). Anything that parses or rewrites bytecode (old `bytecode` versions, some coverage, tracing and decompiler tools) needs a 3.11-aware release.

***

#### New in 3.11: Fine-Grained Error Locations in Tracebacks (PEP 657)

Tracebacks now point to the exact sub-expression that failed, not just the line:

```text
Traceback (most recent call last):
  File "query.py", line 32, in query_user
    return 1 + query_count(db, response['a']['b']['c']['user'], retry=True)
                               ~~~~~~~~~~~~~~~~~~^^^^^
TypeError: 'NoneType' object is not subscriptable
```

```python
def first_word(text: str | None) -> str:
    return text.split()[0]

def column_positions() -> None:
    # The same data is available programmatically via co_positions():
    # (start_line, end_line, start_col, end_col) per bytecode instruction
    positions = list(first_word.__code__.co_positions())
    print(all(len(p) == 4 for p in positions))   # True

column_positions()
```

**Review guidance:**

- With PEP 657, putting several risky operations on one line no longer makes the traceback ambiguous. Readability is still the reason to split them up.
- The column tables cost memory and disk. To drop them (for example in memory-constrained containers) and lose the carets, run with `-X no_debug_ranges` or `PYTHONNODEBUGRANGES=1`.
- Tools that render tracebacks themselves should use `traceback.TracebackException` / `traceback.format_exception()` so the carets and notes are preserved. `TracebackException.print()` is new in 3.11.
- `inspect.getframeinfo()`, `inspect.stack()` and related functions now return `FrameInfo`/`Traceback` objects with a `positions` attribute. They remain backward compatible with the old named-tuple interface.

***

#### New in 3.11: Exception Groups and `except*` (PEP 654)

`ExceptionGroup` and `BaseExceptionGroup` bundle several unrelated exceptions, so they can be raised and handled together. The new `except*` clause matches the *parts* of a group by type. This is the error model of `asyncio.TaskGroup`. It is also the right tool for any "run many things, report all failures" code: validation of many fields, retries across several hosts, batch jobs.

##### Creating and Raising Groups

```python
def validate(record: dict[str, object]) -> None:
    errors: list[Exception] = []
    if not record.get('name'):
        errors.append(ValueError('name is required'))
    if not isinstance(record.get('age'), int):
        errors.append(TypeError('age must be an int'))
    if errors:
        raise ExceptionGroup('invalid record', errors)

try:
    validate({'age': 'x'})
except ExceptionGroup as eg:
    print(eg.message)                       # invalid record
    print(len(eg.exceptions))               # 2
    print([type(e).__name__ for e in eg.exceptions])   # ['ValueError', 'TypeError']
```

**Construction rules (enforced at runtime):**

```python
# ExceptionGroup may only contain Exception instances
try:
    ExceptionGroup('m', [KeyboardInterrupt()])
except TypeError as e:
    print(e)        # Cannot nest BaseExceptions in an ExceptionGroup

# BaseExceptionGroup picks the right class automatically
print(type(BaseExceptionGroup('m', [ValueError()])).__name__)        # ExceptionGroup
print(type(BaseExceptionGroup('m', [KeyboardInterrupt()])).__name__) # BaseExceptionGroup

# The list must be non-empty
try:
    ExceptionGroup('m', [])
except ValueError as e:
    print(e)        # second argument (exceptions) must be a non-empty sequence
```

##### `except*` Semantics

Each `except*` clause takes the matching leaf exceptions out of the group. **Every** matching clause runs (not just the first). Whatever no clause matched is re-raised automatically as a new group.

```python
def run() -> None:
    raise ExceptionGroup('batch', [ValueError(1), TypeError(2), KeyError(3)])

try:
    try:
        run()
    except* ValueError as eg:
        print('values:', eg.exceptions)     # values: (ValueError(1),)
    except* TypeError as eg:
        print('types:', eg.exceptions)      # types: (TypeError(2),)
except ExceptionGroup as rest:
    print('unhandled:', repr(rest))         # unhandled: ExceptionGroup('batch', [KeyError(3)])
```

Inside an `except*` clause the bound name is always an **exception group** (the matching subgroup), never the bare exception. Even a naked exception gets wrapped:

```python
try:
    raise ValueError('plain')
except* ValueError as eg:
    print(type(eg).__name__, eg.exceptions)   # ExceptionGroup (ValueError('plain'),)
```

**Pitfall — plain `except` does not look inside a group.** This is the most common bug when code starts using `TaskGroup` or any API that raises groups:

```python
try:
    raise ExceptionGroup('g', [OSError('disk full')])
except OSError:                # ❌ a group is not an OSError — this clause never matches
    print('never reached')
except ExceptionGroup:
    print('caught the group')  # caught the group
```

Use `except* OSError:`, or catch the group and inspect it with `split()`/`subgroup()`.

**Syntax restrictions** (all `SyntaxError` at compile time):

```python
bad_sources = {
    'mixing except and except*':
        'try:\n    pass\nexcept ValueError:\n    pass\nexcept* TypeError:\n    pass\n',
    'return in except*':
        'def f():\n    try:\n        pass\n    except* ValueError:\n        return 1\n',
    'bare except*':
        'try:\n    pass\nexcept*:\n    pass\n',
}
for label, src in bad_sources.items():
    try:
        compile(src, label, 'exec')
    except SyntaxError as e:
        print(f'{label}: {e.msg}')
# mixing except and except*: cannot have both 'except' and 'except*' on the same 'try'
# return in except*: 'break', 'continue' and 'return' cannot appear in an except* block
# bare except*: expected one or more exception types
```

`except* ExceptionGroup:` compiles, but raises `TypeError` at runtime ("catching ExceptionGroup with except* is not allowed"). Use plain `except ExceptionGroup:` for that.

##### `split()`, `subgroup()` and Nested Groups

Groups nest, and `split()`/`subgroup()` preserve the nesting structure:

```python
eg = ExceptionGroup('top', [
    ValueError('a'),
    ExceptionGroup('inner', [TypeError('b'), ValueError('c')]),
])
match, rest = eg.split(ValueError)
print(repr(match))   # ExceptionGroup('top', [ValueError('a'), ExceptionGroup('inner', [ValueError('c')])])
print(repr(rest))    # ExceptionGroup('top', [ExceptionGroup('inner', [TypeError('b')])])

only_types = eg.subgroup(lambda e: isinstance(e, TypeError))
print(repr(only_types))   # ExceptionGroup('top', [ExceptionGroup('inner', [TypeError('b')])])

def leaves(exc: BaseException) -> list[BaseException]:
    """Flatten a (possibly nested) group into its leaf exceptions."""
    if isinstance(exc, BaseExceptionGroup):
        return [leaf for sub in exc.exceptions for leaf in leaves(sub)]
    return [exc]

print([str(e) for e in leaves(eg)])   # ['a', 'b', 'c']
```

**Pitfall — custom group subclasses lose their type when split** unless you override `derive()`:

```python
class BatchError(ExceptionGroup):
    def derive(self, excs):
        return BatchError(self.message, excs)   # ✅ keep the subclass for split/except* remainders

class LossyError(ExceptionGroup):
    pass                                        # ❌ remainders come back as plain ExceptionGroup

for cls in (BatchError, LossyError):
    try:
        try:
            raise cls('m', [ValueError(), TypeError()])
        except* ValueError:
            pass
    except ExceptionGroup as rest:
        print(cls.__name__, '->', type(rest).__name__)
# BatchError -> BatchError
# LossyError -> ExceptionGroup
```

`derive()` does not copy `__traceback__`, `__cause__`, `__context__` or `__notes__` to the new group; `split()`/`subgroup()` do that for you.

##### `contextlib.suppress` and Other Tools Don't Unwrap Groups in 3.11

```python
import contextlib

try:
    with contextlib.suppress(ValueError):
        raise ExceptionGroup('g', [ValueError(1)])
    print('suppressed')
except ExceptionGroup:
    print('NOT suppressed in 3.11')   # NOT suppressed in 3.11
```

In 3.11 `contextlib.suppress` does **not** handle groups whose leaves all match. That was added in 3.12. For 3.11, use `except* ValueError: pass`. Similarly, `unittest`'s `assertRaises(ValueError)` does not match a group containing a `ValueError` (see Testing).

##### When to Raise Groups — Guidance

- **Raise a group only for genuinely concurrent or independent failures.** A function that fails for one reason should raise one exception. Changing a public API from raising `ValueError` to raising `ExceptionGroup` is a **breaking change** for every caller's `except ValueError:`.
- **Library code:** if you start using `TaskGroup` internally, decide whether to let the group escape (document it) or to unwrap a single-failure group at the boundary.
- **Don't swallow the remainder.** `except* A:` re-raises the parts that aren't `A`. Don't wrap `except*` in a broad `except Exception: pass`.
- **Logging:** `logging.exception()` and `traceback.print_exception()` render groups as a tree (`+-+---- 1 ----`). Inside an `except*` handler, `sys.exception()` is the *matched subgroup*, so only that part is logged there.
- **Third-party backports:** libraries supporting older Pythons depend on the `exceptiongroup` backport package. On 3.11 the built-ins are used, and the backport's `catch()` helper is unnecessary.

***

#### New in 3.11: Exception Notes — `add_note()` (PEP 678)

`BaseException.add_note(str)` attaches extra context to an exception *after* it was raised, without changing its type or message. Notes appear in the standard traceback below the message.

```python
import traceback

def parse_row(line_no: int, raw: str) -> int:
    try:
        return int(raw)
    except ValueError as e:
        e.add_note(f'while parsing line {line_no} of data.csv')   # ✅ keep the original type
        raise

try:
    parse_row(3, 'abc')
except ValueError as e:
    print(e.__notes__)          # ['while parsing line 3 of data.csv']
    print(''.join(traceback.format_exception_only(e)), end='')
    # ValueError: invalid literal for int() with base 10: 'abc'
    # while parsing line 3 of data.csv
```

**Rules and pitfalls:**

```python
e = ValueError('bad')
print(hasattr(e, '__notes__'))   # False — the attribute only exists after the first add_note()
e.add_note('first')
e.add_note('second')
print(e.__notes__)               # ['first', 'second']
print(str(e))                    # bad — str() and args are unchanged; notes only show in tracebacks

try:
    e.add_note(42)
except TypeError as err:
    print(err)                   # note must be a str, not 'int'
```

- **Prefer notes over re-wrapping when the type matters.** `raise ConfigError(...) from e` changes the type, which is right at an API boundary. `add_note()` + bare `raise` keeps the type that callers already catch.
- **Read notes defensively:** `getattr(exc, '__notes__', [])`.
- **Notes are not in `str(exc)`.** If your error reporting sends `str(exc)` to an external system (Sentry, JSON logs), include the notes explicitly or use `traceback.format_exception()`.
- **Don't put secrets in notes.** They are printed in every traceback.
- **Notes add up across retries.** In retry loops, add notes to a fresh exception or add only one summary note. Calling `add_note()` on the same object each iteration accumulates duplicates.

***

#### New in 3.11: `tomllib` — TOML Parsing in the Standard Library (PEP 680)

`tomllib` parses TOML 1.0 (`pyproject.toml`, tool configs). It is **read-only**: there is no `dump`/`dumps`. To write TOML, use the third-party `tomli-w` or `tomlkit` (which preserves comments and style).

```python
import tomllib

doc = '''
title = "service"

[server]
host = "0.0.0.0"
port = 8080
started = 1979-05-27T07:32:00Z
local = 1979-05-27T07:32:00
ratios = [0.1, 0.25]
'''
config = tomllib.loads(doc)
print(config['server']['port'])                    # 8080
print(config['server']['started'].tzinfo)          # UTC — offset datetimes become aware
print(config['server']['local'].tzinfo)            # None — local datetimes stay naive
print(hasattr(tomllib, 'dumps'))                   # False — read-only module
```

**Rule 1: `tomllib.load()` requires a binary file.** This is deliberate: TOML is always UTF-8, so the parser decodes the bytes itself.

```python
import io
import tomllib

try:
    tomllib.load(io.StringIO('a = 1'))          # ❌ text-mode file object
except TypeError as e:
    print(e)    # File must be opened in binary mode, e.g. use `open('foo.toml', 'rb')`

with open(__file__, 'rb') as f:                 # ✅ always 'rb' for tomllib.load()
    f.read(0)
```

**Rule 2: Catch `tomllib.TOMLDecodeError` (a `ValueError` subclass) for bad input:**

```python
import tomllib

try:
    tomllib.loads('name = ')
except tomllib.TOMLDecodeError as e:
    print(isinstance(e, ValueError), e)   # True Invalid value (at end of document)
```

**Rule 3: Use `parse_float=decimal.Decimal` for money and exact decimal values:**

```python
import decimal
import tomllib

prices = tomllib.loads('unit_price = 0.1', parse_float=decimal.Decimal)
print(prices['unit_price'])          # 0.1 (a Decimal, not the binary float 0.1)
```

**Supporting older Pythons:** `tomllib` is the vendored `tomli` package. For projects that also support < 3.11, use the conditional import below and declare `tomli; python_version < "3.11"` as a dependency:

```python
import sys

if sys.version_info >= (3, 11):
    import tomllib
else:  # pragma: no cover
    import tomli as tomllib

print(tomllib.loads('x = 1'))   # {'x': 1}
```

***

#### New in 3.11: Structured Concurrency — `asyncio.TaskGroup` and `asyncio.timeout()`

3.11 adds two context managers that What's New recommends over raw `create_task()`/`gather()` and `wait_for()` for new code.

##### `asyncio.TaskGroup`

```python
import asyncio

async def fetch(n: int, delay: float) -> int:
    await asyncio.sleep(delay)
    return n * 10

async def main() -> None:
    async with asyncio.TaskGroup() as tg:
        t1 = tg.create_task(fetch(1, 0.01))
        t2 = tg.create_task(fetch(2, 0.02), name='fetch-2')
    # Leaving the block waits for every task; results are ready here
    print(t1.result(), t2.result())   # 10 20

asyncio.run(main())
```

Semantics that reviewers must know:

1. **When one task fails, the others are cancelled.** On the first exception other than `CancelledError`, the group cancels the remaining tasks. If the `async with` body is still running, the task containing it is cancelled too. That `CancelledError` does not escape the block.
2. **Failures always arrive as a group, even a single one.** After all tasks finish, the failures are raised as `ExceptionGroup`/`BaseExceptionGroup`. `except ValueError:` around a `TaskGroup` never fires. Use `except*`.
3. **`KeyboardInterrupt` and `SystemExit` are special.** They are re-raised as-is, not wrapped.
4. **An exception from the body counts like a task failure.** Siblings are cancelled, and the body's exception is included in the group.
5. **A finished group refuses new tasks.** `create_task()` on it raises `RuntimeError`. Tasks may still be added while the group is waiting, for example by passing `tg` into a running task.

```python
import asyncio

async def slow(cancelled: list[bool]) -> None:
    try:
        await asyncio.sleep(10)
    except asyncio.CancelledError:
        cancelled.append(True)
        raise                              # always re-raise CancelledError

async def boom() -> None:
    await asyncio.sleep(0.01)
    raise ValueError('upstream returned 500')

async def main() -> None:
    cancelled: list[bool] = []
    try:
        async with asyncio.TaskGroup() as tg:
            tg.create_task(slow(cancelled))
            tg.create_task(boom())
    except* ValueError as eg:              # ✅ except*, not except ValueError
        print('failed:', eg.exceptions)    # failed: (ValueError('upstream returned 500'),)
        print('sibling cancelled:', cancelled)   # sibling cancelled: [True]

    # ❌ A plain except clause misses the wrapped error
    try:
        async with asyncio.TaskGroup() as tg:
            tg.create_task(boom())
    except ValueError:
        print('never reached')
    except ExceptionGroup as eg:
        print('got a group:', type(eg).__name__)   # got a group: ExceptionGroup

asyncio.run(main())
```

**`TaskGroup` vs `gather()` — when to use which:**

| | `asyncio.TaskGroup` (3.11+) | `asyncio.gather()` |
|---|---|---|
| Sibling cancellation on failure | Automatic | No — other awaitables keep running |
| Error reporting | All failures, as `ExceptionGroup` | First exception only (others lost unless `return_exceptions=True`) |
| Results | `task.result()` per task | Ordered list |
| Partial results on failure | Not available (siblings cancelled) | `return_exceptions=True` gives a mixed list |
| Leaked tasks | Impossible — block waits for all | Possible — after the first failure the remaining awaitables keep running unawaited |

Use `TaskGroup` by default: all-or-nothing, no orphaned tasks. Keep `gather(..., return_exceptions=True)` when you deliberately want best-effort partial results.

**Bounded concurrency** still needs a semaphore. `TaskGroup` does not limit parallelism:

```python
import asyncio

async def bounded_fetch_all(urls: list[str], limit: int = 10) -> dict[str, int]:
    sem = asyncio.Semaphore(limit)
    results: dict[str, int] = {}

    async def one(url: str) -> None:
        async with sem:
            await asyncio.sleep(0)          # stand-in for real I/O
            results[url] = len(url)

    async with asyncio.TaskGroup() as tg:
        for url in urls:
            tg.create_task(one(url))
    return results

print(asyncio.run(bounded_fetch_all(['a', 'bb', 'ccc'])))   # {'a': 1, 'bb': 2, 'ccc': 3}
```

**Pitfall — don't create one task per item for millions of items.** Each task costs memory. Use a fixed pool of worker tasks that read from an `asyncio.Queue`.

**Pitfall — a task that catches `CancelledError` and doesn't re-raise it** keeps the group waiting and breaks cancellation. Always re-raise after cleanup.

##### `asyncio.timeout()` and `asyncio.timeout_at()`

```python
import asyncio

async def main() -> None:
    try:
        async with asyncio.timeout(0.05):
            await asyncio.sleep(1)
    except TimeoutError:                   # builtin TimeoutError — see below
        print('timed out')                 # timed out

    # Unknown deadline at entry: start with None, reschedule later
    loop = asyncio.get_running_loop()
    try:
        async with asyncio.timeout(None) as cm:
            cm.reschedule(loop.time() + 0.01)
            await asyncio.sleep(1)
    except TimeoutError:
        print('expired:', cm.expired())   # expired: True

    # Absolute deadline shared across several steps
    deadline = loop.time() + 1.0
    async with asyncio.timeout_at(deadline):
        await asyncio.sleep(0)
        await asyncio.sleep(0)
    print('both steps done before the deadline')

asyncio.run(main())
```

Key facts:

- **The timeout works by cancelling the current task.** Inside the block the code sees `CancelledError`. The context manager converts it into `TimeoutError` on exit, so `TimeoutError` can only be caught **outside** the `async with`.
- **In 3.11, `asyncio.TimeoutError` *is* the builtin `TimeoutError`.** So is `concurrent.futures.TimeoutError`. Write `except TimeoutError:`. In 3.10 they were different classes, so if you share code with 3.10, catch `asyncio.TimeoutError` explicitly.
- **It must run inside a task on a running loop.** Calling `asyncio.timeout()` outside a running loop raises `RuntimeError: no running event loop`.
- **Prefer `timeout()` over `wait_for()` in 3.11.** `wait_for()` wraps the awaitable in a new task. In 3.11 it still has cancellation races, and was reimplemented on top of `timeout()` only in 3.12.

**Pitfall — swallowing `CancelledError` inside a timeout silently disables it:**

```python
import asyncio

async def swallow() -> str:
    try:
        await asyncio.sleep(1)
    except asyncio.CancelledError:
        return 'swallowed'                 # ❌ never swallow CancelledError

async def main() -> None:
    try:
        async with asyncio.timeout(0.01) as cm:
            result = await swallow()
        print('no TimeoutError raised;', result, '| expired:', cm.expired())
        # no TimeoutError raised; swallowed | expired: True
    except TimeoutError:
        print('not reached')

asyncio.run(main())
```

***

#### Primitive Types and Numerics

##### Integers

`int` has arbitrary precision. Arithmetic never overflows, and memory grows as needed.

```python
import sys

print(sys.maxsize)     # 9223372036854775807 on 64-bit (C ssize_t max, NOT the int limit)
x = 10 ** 1000         # fine — arbitrary precision arithmetic
print(type(x))         # <class 'int'>

one_million = 1_000_000          # underscores in literals (3.6+)
mask = 0xFF_FF_FF_FF

# New in 3.11: int.to_bytes() defaults to length=1, byteorder='big'
print((1).to_bytes())            # b'\x01'
print((258).to_bytes(2))         # b'\x01\x02'
print(int.from_bytes(b'\x01\x02'))   # 258 — byteorder now defaults to 'big'
```

##### New in 3.11: The Integer/String Conversion Length Limit (CVE-2020-10735)

Converting between `int` and `str` in a non-power-of-two base, such as decimal, has quadratic cost. A request containing a 1-million-digit number could pin a CPU, so 3.11 raises `ValueError` when such a conversion exceeds **4300 digits**. The limit shipped in 3.11.0 and was **backported to security releases 3.10.7, 3.9.14, 3.8.14 and 3.7.14** (September 2022), so it is not strictly new behavior. 3.11 is the first feature release that has it from day one.

```python
import json
import sys

print(sys.get_int_max_str_digits())     # 4300 (default; sys.int_info.default_max_str_digits)

try:
    int('1' * 5000)                      # str -> int
except ValueError as e:
    print(str(e)[:61])   # Exceeds the limit (4300 digits) for integer string conversion

big = 10 ** 5000
try:
    str(big)                             # int -> str; also f'{big}', '%d' % big, repr(big)
except ValueError:
    print('str() of a 5001-digit int fails')    # str() of a 5001-digit int fails

print(len(hex(big)) > 0)                 # True — hex/oct/bin (power-of-two bases) are unlimited

try:
    json.loads('1' * 5000)               # json goes through int() — also limited
except ValueError:
    print('json.loads rejects the huge integer')   # json.loads rejects the huge integer
```

**Configuring the limit** (the minimum non-zero value is 640; 0 disables it):

```python
import sys

old = sys.get_int_max_str_digits()
sys.set_int_max_str_digits(10_000)       # process-wide — affects every thread
print(len(str(10 ** 5000)))              # 5001
sys.set_int_max_str_digits(old)          # restore

try:
    sys.set_int_max_str_digits(100)
except ValueError as e:
    print(e)                             # maxdigits must be 0 or larger than 640
```

Other ways to set it: `python -X int_max_str_digits=0` and the environment variable `PYTHONINTMAXSTRDIGITS`.

**Review guidance:**

- **Keep the limit on in services that parse untrusted input.** Don't add `sys.set_int_max_str_digits(0)` to a web app to "fix" a failing test.
- Code that legitimately prints huge integers (math, crypto tooling, `math.factorial(2000)`) must raise the limit **locally and deliberately**, or use `hex()` / `int.to_bytes()`.
- `decimal.Decimal` and `float` parsing are not affected.

##### Floats

Floats are C `double` (IEEE 754 binary64). New `math` functions in 3.11:

```python
import math

print(math.exp2(10))          # 1024.0 — 2 ** x, new in 3.11
print(math.cbrt(-8.0))        # -2.0 — real cube root (works for negatives, unlike x ** (1/3))
print((-8.0) ** (1 / 3))      # a complex number — the reason cbrt() exists
print(math.isclose(math.cbrt(27.0), 3.0))   # True — compare with isclose; cbrt is libm-backed and may be off by 1 ulp

# Changed in 3.11: IEEE 754 corner cases now return inf instead of raising ValueError
print(math.pow(0.0, -math.inf))   # inf

assert math.isclose(0.1 + 0.2, 0.3)   # never compare floats with ==
```

**New in 3.11: the `z` format option (PEP 682)** turns negative zero into positive zero after rounding. Use it for user-facing numbers:

```python
value = -0.0001
print(f'{value:.2f}')      # -0.00 — confusing in reports
print(f'{value:z.2f}')     # 0.00
```

##### `fractions` and `Booleans`

```python
from fractions import Fraction

print(Fraction('1_000/3_000'))   # 1/3 — New in 3.11: PEP 515 underscores accepted in strings
print(int(Fraction(7, 2)))       # 3 — Fraction now implements __int__ (SupportsInt)

print(isinstance(True, int))     # True — bool is a subclass of int
print(True + True)               # 2
```

**Falsy values:** `None`, `False`, zero of any numeric type (`0`, `0.0`, `0j`, `Decimal(0)`, `Fraction(0)`), empty sequences and collections (`''`, `b''`, `()`, `[]`, `{}`, `set()`, `range(0)`), and objects whose `__bool__` returns `False` or whose `__len__` returns `0`.

**Deprecated in 3.11:** `int()` delegating to `__trunc__()` emits `DeprecationWarning` (the delegation is removed in 3.14). Implement `__int__` or `__index__` on numeric types instead.

***

#### Strings

##### The Two Types

- **`str`** is a sequence of Unicode code points. Every undecorated literal is a `str`.
- **`bytes`** is a sequence of raw 8-bit values, written with the `b''` prefix. It never mixes implicitly with `str`.

```python
text = 'hello'
data = b'hello'
try:
    text + data          # ❌
except TypeError as e:
    print(e)             # can only concatenate str (not "bytes") to str
```

**The Unicode sandwich:** decode `bytes` to `str` at input boundaries, work in `str` internally, and encode back to `bytes` at output. Always pass `encoding='utf-8'` to `open()`: the default is still the locale encoding in 3.11, and UTF-8 mode is not the default until 3.15. Run tests with `-X warn_default_encoding` to get an `EncodingWarning` (3.10+) for every `open()` that omits it.

##### String Methods (Essential Reference)

```python
s = 'Hello, World!'

s.casefold()                      # aggressive lowercase — use for case-insensitive comparison
s.removeprefix('Hello, ')         # 'World!' (3.9+) — removes an exact prefix once
s.removesuffix('!')               # 'Hello, World' (3.9+)
'https://x.com'.lstrip('https://')   # 'x.com' only by luck — lstrip strips a CHARACTER SET
s.startswith(('Hello', 'Hi'))     # tuple of prefixes
s.partition(', ')                 # ('Hello', ', ', 'World!')
s.split(', ', maxsplit=1)         # ['Hello', 'World!']
', '.join(['a', 'b'])             # 'a, b'
'42'.zfill(6)                     # '000042'
'café'.encode('utf-8')            # b'caf\xc3\xa9'
print(s.removeprefix('Hello, '), s.partition(', ')[2])   # World! World!
```

**Rule:** to remove a known prefix or suffix, use `removeprefix()`/`removesuffix()`. `lstrip()`/`rstrip()`/`strip()` remove *any characters in the given set*, repeatedly.

##### f-strings in 3.11 — Know the Pre-PEP 701 Limits

3.11 f-strings follow the **old grammar**. PEP 701 (3.12) lifted all of these restrictions, so code written for 3.12+ often fails to compile on 3.11:

```python
d = {'key': 'value'}
items = ['line1', 'line2']

bad = [
    "f'{d['key']}'",                 # same quote type reused inside {} — SyntaxError in 3.11
    "f'{\"\\n\".join(items)}'",      # backslash inside the expression — SyntaxError in 3.11
]
for src in bad:
    try:
        compile(src, '<f>', 'eval')
    except SyntaxError as e:
        print(e.msg)
# f-string: unmatched '['
# f-string expression part cannot include a backslash

# ✅ 3.11-compatible forms
print(f"Value: {d['key']}")          # Value: value — switch the outer quote type
nl = '\n'
print(f'{nl.join(items)!r}')         # 'line1\nline2' — hoist the backslash into a variable
```

Other f-string facts that still hold in 3.11:

- `f'{x=}'` (3.8+) is for debugging. Don't use it in production log messages.
- `f'{value:z.2f}'` (the `z` option, new in 3.11) suppresses negative zero.
- **New in 3.11 optimization:** simple `%s`/`%r`/`%a` formatting on string literals compiles as fast as an equivalent f-string. Performance is no longer a reason to rewrite `'%s' % x`.

##### String Literal Deprecations in 3.11

```python
import warnings

with warnings.catch_warnings(record=True) as rec:
    warnings.simplefilter('always')
    eval(r'"\777"')        # octal escape > 0o377 — DeprecationWarning, new in 3.11
    eval(r'"\d"')          # invalid escape — DeprecationWarning (SyntaxWarning from 3.12)
print([str(w.message) for w in rec])
# ["invalid octal escape sequence '\\777'", "invalid escape sequence '\\d'"]
```

Always write regex patterns as raw strings (`r'\d+'`). The invalid-escape warning becomes a visible `SyntaxWarning` in 3.12 and is planned to become a `SyntaxError` later.

##### New in 3.11: `string.Template` Introspection

```python
from string import Template

t = Template('Hello $name, your order ${order_id} ships $$5')
print(t.get_identifiers())               # ['name', 'order_id']
print(t.is_valid())                      # True
print(Template('price: $').is_valid())   # False — a dangling $ would make substitute() raise
```

Use `Template` (not `str.format`) for **user-supplied** format strings. `str.format` on untrusted templates can read attributes (`'{0.__class__.__init__.__globals__}'`).

***

#### Variable Annotations and Type Hints

##### Baseline: What 3.11 Code Should Look Like

Built-in generics (3.9) and `X | Y` unions (3.10) are fully usable at runtime in 3.11, including in `isinstance()` checks with unions:

```python
from collections.abc import Callable, Iterable, Mapping

def process(
    items: Iterable[str],
    mapping: Mapping[str, int],
    on_error: Callable[[Exception], None] | None = None,
) -> dict[str, list[int]]:
    out: dict[str, list[int]] = {}
    for item in items:
        out.setdefault(item, []).append(mapping.get(item, 0))
    return out

print(isinstance(3, int | str))      # True — union types work in isinstance (3.10+)
print(process(['a'], {'a': 1}))      # {'a': [1]}
```

- Use `X | None`, not `Optional[X]`. Use `list[int]`, not `typing.List[int]`. Use `collections.abc.Callable`, not `typing.Callable`. Ruff's `UP` rules with `target-version = "py311"` rewrite all of these automatically.
- `isinstance(x, list[int])` still raises `TypeError`. Parameterized generics can't be used in `isinstance`.
- **New in 3.11:** `typing.get_type_hints()` no longer adds `Optional[...]` for a `None` default. `def f(x: int = None)` reports `{'x': int}` (3.10 reported `Optional[int]`). Code that introspects hints, such as serializers and CLI builders, may see different types after upgrading. Annotate `x: int | None = None` explicitly.

##### New in 3.11: `typing.Self` (PEP 673)

`Self` annotates "an instance of the current class". It replaces the bound-`TypeVar` boilerplate for fluent methods, alternative constructors and `__enter__`:

```python
from typing import Self, TypeVar

# ❌ Pre-3.11 boilerplate
TShape = TypeVar('TShape', bound='ShapeOld')
class ShapeOld:
    def set_scale(self: TShape, scale: float) -> TShape:
        self.scale = scale
        return self

# ✅ 3.11
class Shape:
    def __init__(self) -> None:
        self.scale = 1.0
        self.children: list[Self] = []

    def set_scale(self, scale: float) -> Self:
        self.scale = scale
        return self

    @classmethod
    def unit(cls) -> Self:
        return cls()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *exc: object) -> None:
        pass

class Circle(Shape):
    pass

c = Circle.unit().set_scale(2.0)     # type checkers infer Circle, not Shape
print(type(c).__name__, c.scale)     # Circle 2.0
```

**Rules:** `Self` is valid only inside a class body (methods, attributes, class-level annotations). Type checkers reject it in `@staticmethod`s, in module-level functions and in metaclasses. At runtime it is a special form that can't be used with `isinstance()`. **Return `Self` only when the method really returns `self` or `cls(...)`.** A method that returns `Shape(...)` explicitly should be annotated `-> Shape`, or subclasses will be mistyped.

##### New in 3.11: `LiteralString` (PEP 675)

`LiteralString` accepts only strings that a type checker can prove come from literals, or from combining literals. It exists to stop injection at the type level for SQL, shell commands and templates:

```python
from typing import LiteralString

def run_query(sql: LiteralString, params: tuple[object, ...] = ()) -> None:
    print('executing:', sql, params)

def get_user(user_id: int, table: LiteralString = 'users') -> None:
    run_query('SELECT * FROM ' + table + ' WHERE id = ?', (user_id,))   # ✅ literal + literal

get_user(42)    # executing: SELECT * FROM users WHERE id = ? (42,)

def bad(user_input: str) -> None:
    run_query(f'SELECT * FROM users WHERE name = {user_input}')   # ❌ pyright: "str" is not assignable to "LiteralString"
```

`LiteralString` has **no runtime enforcement**. `bad()` runs fine at runtime, and only a static checker catches it. **Checker support varies:** pyright enforces PEP 675 and rejects the f-string call above. mypy accepts `LiteralString` annotations but treats them as plain `str` (verified with mypy 2.4), so it reports nothing. If you rely on `LiteralString` as a security control, run pyright (or a checker that enforces it) in CI. It complements parameterized queries but doesn't replace them.

##### New in 3.11: `Required` / `NotRequired` for `TypedDict` (PEP 655)

```python
from typing import NotRequired, Required, TypedDict

class Movie(TypedDict):
    title: str
    year: NotRequired[int]           # optional key in an otherwise total TypedDict

class Patch(TypedDict, total=False):
    id: Required[int]                # required key in an otherwise partial TypedDict
    title: str

print(Movie.__required_keys__, Movie.__optional_keys__)   # frozenset({'title'}) frozenset({'year'})
print(Patch.__required_keys__)                            # frozenset({'id'})

m: Movie = {'title': 'Alien'}        # OK — year may be missing
print(m.get('year', 'unknown'))      # unknown — always .get() NotRequired keys
```

`NotRequired[int]` means "the key may be absent". It does **not** mean "the value may be `None`". If both are possible, write `NotRequired[int | None]`.

**Deprecated in 3.11:** the keyword-argument form `TypedDict('TD', x=int)` emits `DeprecationWarning`, and 3.13 rejects it with `TypeError`. Use the class syntax or `TypedDict('TD', {'x': int})`.

##### New in 3.11: Generic `TypedDict` and `NamedTuple`

```python
from typing import Generic, NamedTuple, TypedDict, TypeVar

T = TypeVar('T')

class Envelope(TypedDict, Generic[T]):
    data: T
    request_id: str

class Pair(NamedTuple, Generic[T]):
    first: T
    second: T

resp: Envelope[list[int]] = {'data': [1, 2], 'request_id': 'r-1'}
p = Pair[int](1, 2)
print(p, resp['data'])    # Pair(first=1, second=2) [1, 2]
```

Both raise `TypeError` on 3.10 at class-creation time. The `class Envelope[T](TypedDict)` spelling is PEP 695 syntax (3.12) and is a `SyntaxError` on 3.11.

##### New in 3.11: Variadic Generics — `TypeVarTuple` and `Unpack` (PEP 646)

`TypeVarTuple` stands for an arbitrary number of types. It is mainly for array-shape typing and for typing `*args` precisely:

```python
from typing import Generic, TypeVar, TypeVarTuple

DType = TypeVar('DType')
Shape = TypeVarTuple('Shape')

class Array(Generic[DType, *Shape]):       # star-unpacking in subscripts is new 3.11 syntax
    pass

def args_to_tuple(*args: *Shape) -> tuple[*Shape]:
    return args

print(Array[float, int, int])              # __main__.Array[float, int, int]
print(args_to_tuple(1, 'a'))               # (1, 'a')
```

`typing.Unpack[Shape]` is the equivalent spelling for code that must parse on older Pythons. **Not in 3.11:** `**kwargs: Unpack[SomeTypedDict]` (PEP 692) is a **3.12** type-system feature. `typing.Unpack` exists at runtime in 3.11, but use it for kwargs only with `typing_extensions` and a type checker that supports it.

##### New in 3.11: `dataclass_transform` (PEP 681)

Marks a decorator, base class or metaclass as creating dataclass-like classes, so type checkers synthesize `__init__` for ORMs and model libraries (Pydantic, SQLAlchemy, attrs use it):

```python
from typing import Any, dataclass_transform

@dataclass_transform(kw_only_default=True)
def model(cls: type) -> type:
    def __init__(self: Any, **kwargs: Any) -> None:
        for name in cls.__annotations__:
            setattr(self, name, kwargs[name])
    cls.__init__ = __init__   # type: ignore[assignment]
    return cls

@model
class Customer:
    id: int
    name: str

c = Customer(id=1, name='Eric')          # type checkers now validate these keywords
print(c.name, model.__dataclass_transform__['kw_only_default'])   # Eric True
```

Application code rarely needs this. It matters when reviewing in-house model or ORM frameworks.

##### New in 3.11: `Never`, `assert_never()`, `reveal_type()`, `assert_type()`

```python
from enum import Enum
from typing import Never, NoReturn, assert_never, assert_type, reveal_type

class Status(Enum):
    ACTIVE = 'active'
    BANNED = 'banned'

def label(s: Status) -> str:
    match s:
        case Status.ACTIVE:
            return 'ok'
        case Status.BANNED:
            return 'blocked'
        case _:
            assert_never(s)          # type checker errors if a new member is unhandled

print(label(Status.BANNED))          # blocked

try:
    assert_never(5)                  # at runtime: AssertionError
except AssertionError as e:
    print(e)                         # Expected code to be unreachable, but got: 5

value = assert_type([1, 2], list[int])   # runtime no-op: returns its argument
x = reveal_type(3)                   # prints "Runtime type is 'int'" to stderr, returns 3
print(x, Never is NoReturn)          # 3 False — distinct objects, same meaning to checkers
```

**Review rule:** `reveal_type()` is a debugging aid. Type checkers have always understood it without an import, but at runtime an un-imported call is a `NameError`. Since 3.11, `from typing import reveal_type` makes it a real function that prints to stderr and returns its argument. Never leave it in committed code.

##### Other 3.11 Typing Changes

- **`typing.Any` can be subclassed.** This is useful for mocks and highly dynamic proxies.
- **`@typing.final`** now sets `__final__ = True` on the decorated object, for runtime introspection.
- **`typing.get_overloads(func)`** and **`typing.clear_overloads()`** let you introspect `@overload` variants at runtime.
- **`typing.Text` is deprecated.** It was a Python-2 compatibility alias, so use `str`. It emits no runtime warning.
- **`functools.singledispatch`** accepts union types (`int | float`, `Union[list, set]`) in `register()` annotations.

```python
import functools

@functools.singledispatch
def describe(x: object) -> str:
    return 'other'

@describe.register
def _(x: int | float) -> str:          # union dispatch, new in 3.11
    return 'number'

print(describe(1), describe(2.5), describe('s'))   # number number other
```

##### Typing Features That Are NOT in 3.11

| Feature | Version | 3.11 alternative |
|---|---|---|
| `def f[T](x: T) -> T`, `class C[T]:` (PEP 695) | 3.12 | `T = TypeVar('T')` + `Generic[T]` |
| `type Alias = ...` statement | 3.12 | `Alias: TypeAlias = ...` (3.10) |
| `@typing.override` (PEP 698) | 3.12 | `from typing_extensions import override` |
| `**kwargs: Unpack[TD]` (PEP 692) | 3.12 | `typing_extensions.Unpack` + checker support |
| `typing.TypeIs` (PEP 742) | 3.13 | `typing.TypeGuard` (3.10) or `typing_extensions.TypeIs` |
| `typing.ReadOnly` (PEP 705) | 3.13 | `typing_extensions.ReadOnly` |
| `TypeVar('T', default=...)` (PEP 696) | 3.13 | `typing_extensions.TypeVar` |
| `@warnings.deprecated` (PEP 702) | 3.13 | `typing_extensions.deprecated` |
| Lazy annotations by default (PEP 649) | 3.14 | `from __future__ import annotations` or string forward refs |

***

#### New in 3.11: `enum` Overhaul — `StrEnum`, Formatting Changes, `verify()`

The 3.11 `enum` module was largely rewritten. Two changes are **behavior breaks** that silently change output in upgraded code.

##### Breaking Change 1: `IntEnum`/`IntFlag` `str()` Now Returns the Value

```python
from enum import IntEnum, IntFlag

class HttpStatus(IntEnum):
    OK = 200
    NOT_FOUND = 404

class Perm(IntFlag):
    R = 4
    W = 2

print(str(HttpStatus.OK))        # 200 — in 3.10 this was 'HttpStatus.OK'
print(f'{HttpStatus.OK}')        # 200 — unchanged (format() already used the value)
print(repr(HttpStatus.OK))       # <HttpStatus.OK: 200>
print(str(Perm.R | Perm.W))      # 6 — in 3.10 'Perm.R|W'
print(HttpStatus.OK.name)        # OK — use .name when you want the name
```

`IntEnum`, `IntFlag` and the new `StrEnum` derive from the new `ReprEnum`. It changes only `repr()`, and keeps the *value* for `str()` and `format()`, so they act as drop-in replacements for the constants they replace.

##### Breaking Change 2: `format()` / f-strings of Mixed-in Enums Now Use `str()`

For enums that mix in a data type *without* being `ReprEnum` (the classic `class Color(str, Enum)` pattern), `format()` now matches `str()`, which is `ClassName.MEMBER`, **not the value**:

```python
from enum import Enum

class Color(str, Enum):              # the pre-3.11 "string enum" idiom
    RED = 'red'

print(f'{Color.RED}')                # Color.RED — in 3.10 this printed 'red'
print(format(Color.RED))             # Color.RED
print(Color.RED.value)               # red
print(Color.RED == 'red')            # True — equality is unchanged
```

This silently changes URLs, SQL fragments, log fields and file names built with f-strings, for example `f'/api/{Color.RED}'`. JSON is safe: `json.dumps(Color.RED)` still emits `"red"`, because it uses the `str` value, not `format()`.

**Fix: migrate `(str, Enum)` to `StrEnum`:**

##### `StrEnum` (New in 3.11)

```python
import json
from enum import StrEnum, auto

class Color(StrEnum):
    RED = 'red'
    DARK_BLUE = auto()               # auto() produces the lower-cased member name

print(f'{Color.RED}', str(Color.DARK_BLUE))   # red dark_blue
print(Color.RED == 'red', isinstance(Color.RED, str))   # True True
print(json.dumps({'c': Color.RED}))  # {"c": "red"}
print(Color('red') is Color.RED)     # True — lookup by value

try:
    class Bad(StrEnum):
        A = 1                        # members must be strings
except TypeError as e:
    print(e)                         # 1 is not a string
```

**Review rule:** any `class X(str, Enum)` in a codebase moving to 3.11 must either become `StrEnum`, or have every f-string / `format()` use audited (`.value`). The `(str, Enum)` mixin remains valid, but its formatting changed.

##### New in 3.11: `verify()`, `member()`/`nonmember()`, `enum.property`, `global_enum`

```python
from enum import CONTINUOUS, UNIQUE, Enum, member, nonmember, verify

try:
    @verify(UNIQUE)                  # reject aliases at class creation
    class Level(Enum):
        LOW = 1
        MINIMAL = 1
except ValueError as e:
    print(e)                         # aliases found in <enum 'Level'>: MINIMAL -> LOW

try:
    @verify(CONTINUOUS)              # reject gaps in integer values
    class Step(Enum):
        A = 1
        C = 3
except ValueError as e:
    print(e)                         # invalid enum 'Step': missing values 2

class Op(Enum):
    ADD = member(lambda a, b: a + b)   # functions are normally NOT members — member() forces it
    VERSION = nonmember(2)             # plain class attribute, not a member

    def __call__(self, *args: int) -> int:
        return self.value(*args)

print(list(Op), Op.VERSION, Op.ADD(2, 3))   # [<Op.ADD: <function Op.<lambda> at 0x...>>] 2 5
```

`@enum.unique` still works. `@verify(UNIQUE)` is its more general successor, adding `CONTINUOUS` and `NAMED_FLAGS` checks.

##### `Flag` Changes in 3.11

```python
from enum import CONFORM, Flag, IntFlag, auto

class Perm(Flag):
    R = auto()
    W = auto()
    X = auto()

rw = Perm.R | Perm.W
print(list(rw), len(rw))     # [<Perm.R: 1>, <Perm.W: 2>] 2 — iteration and len() are new in 3.11
print(Perm.R in rw)          # True
print(~Perm.R)               # Perm.W|X — the positive equivalent within the defined flags

try:
    Perm(8)                  # default boundary for Flag is STRICT
except ValueError as e:
    print('ValueError')      # ValueError

class Mode(Flag, boundary=CONFORM):
    A = 1
    B = 2
print(repr(Mode(7)))         # <Mode.A|B: 3> — CONFORM drops unknown bits

class Bits(IntFlag):         # IntFlag defaults to KEEP — unknown bits are preserved
    A = 1
print(repr(Bits(5)))         # <Bits.A|4: 5>
```

- Only single-bit (power-of-two) values are canonical members. Multi-bit values defined in the class are aliases, and iterating a flag yields only its canonical members.
- `Flag` membership tests take members, not ints: `1 in rw` raises `TypeError`.
- `EnumMeta` was renamed `EnumType`. The old name remains as an alias.

***

#### Dates and Times

##### New in 3.11: `datetime.UTC` and Much Broader `fromisoformat()`

```python
from datetime import UTC, date, datetime, time, timezone

print(UTC is timezone.utc)                    # True — just a shorter alias
now = datetime.now(UTC)                       # ✅ aware "now"
print(now.tzinfo)                             # UTC

# 3.11 fromisoformat() parses most ISO 8601 forms (3.10 accepted only isoformat() output)
print(datetime.fromisoformat('2011-11-04T00:05:23Z'))      # 2011-11-04 00:05:23+00:00 — 'Z' accepted
print(datetime.fromisoformat('20111104T000523'))           # 2011-11-04 00:05:23 — basic format
print(date.fromisoformat('2011-W01-2'))                    # 2011-01-04 — ISO week date
print(time.fromisoformat('04:23:01.000384+00:00'))         # 04:23:01.000384+00:00
```

**Pitfalls:**

- **Don't call `datetime.utcnow()` or `utcfromtimestamp()`.** They return *naive* datetimes that many libraries treat as local time. They are not yet deprecated in 3.11 (the `DeprecationWarning` arrives in 3.12), but they are already wrong. Use `datetime.now(UTC)` and `datetime.fromtimestamp(ts, UTC)`.
- **`fromisoformat()` isn't a full ISO 8601 validator.** It still rejects fractional hours and minutes. Don't use it as an input validator for strict formats. It accepts many variants, so a 3.11 service accepts inputs that a 3.10 service rejected. That can split behavior across a mixed fleet.
- **For named zones, use `zoneinfo` (3.9).** Attach with `tzinfo=ZoneInfo('Europe/Berlin')` or convert with `.astimezone()`. Never use `pytz`'s `.localize()` idiom with `zoneinfo`. Declare the `tzdata` package as a dependency for Windows and minimal containers.

```python
from datetime import UTC, datetime
from zoneinfo import ZoneInfo

utc_dt = datetime(2024, 3, 10, 7, 30, tzinfo=UTC)
print(utc_dt.astimezone(ZoneInfo('America/New_York')))   # 2024-03-10 03:30:00-04:00
```

***

#### Lists

```python
lst: list[int] = [1, 2, 3, 4, 5]

print(lst[0], lst[-1], lst[1:3], lst[::-1])   # 1 5 [2, 3] [5, 4, 3, 2, 1]

lst.append(6)
lst.extend([7, 8])
lst.insert(0, 0)
lst.remove(3)          # removes the first occurrence; ValueError if absent
last = lst.pop()       # removes and returns the last element

nums = [3, 1, 4, 1, 5, 9]
nums.sort()                        # in place, stable
by_desc = sorted(nums, reverse=True)   # new list
squares = [x * x for x in range(10)]
flat = [item for row in [[1, 2], [3, 4]] for item in row]   # [1, 2, 3, 4]
print(by_desc[:3], flat)           # [9, 5, 4] [1, 2, 3, 4]
```

**Pitfall: list multiplication shares references.**

```python
matrix = [[0] * 3] * 3             # ❌ three references to ONE inner list
matrix[0][0] = 99
print(matrix)                      # [[99, 0, 0], [99, 0, 0], [99, 0, 0]]

matrix = [[0] * 3 for _ in range(3)]   # ✅
matrix[0][0] = 99
print(matrix)                      # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

**3.11 performance note:** `list.append()` is about 15% faster and simple list comprehensions up to 20–30% faster. Comprehensions are still not inlined in 3.11; that is PEP 709 in 3.12. Write the clearest version.

***

#### Tuples

```python
from typing import NamedTuple

t = (1, 2, 3)
single = (42,)                     # trailing comma makes a 1-tuple
first, *rest = [1, 2, 3, 4]        # extended unpacking
a, b = 1, 2
a, b = b, a                        # swap

class Point(NamedTuple):
    x: float
    y: float
    label: str = ''

p = Point(1.0, 2.0, 'origin')
print(p._asdict(), p._replace(x=5.0))
# {'x': 1.0, 'y': 2.0, 'label': 'origin'} Point(x=5.0, y=2.0, label='origin')
```

Annotate homogeneous variable-length tuples as `tuple[int, ...]` and fixed shapes as `tuple[str, int]`. **New in 3.11:** `typing.get_args(typing.Tuple[()])` is `()` (was `((),)`; the built-in `tuple[()]` already gave `()` in 3.10), which matters only for introspection code.

***

#### Dictionaries

Dicts preserve insertion order. Merge with `|` and `|=` (3.9+).

```python
defaults = {'host': 'localhost', 'port': 5432}
overrides = {'port': 9432}
config = defaults | overrides      # right side wins
print(config)                      # {'host': 'localhost', 'port': 9432}

d = {'a': 1, 'b': 2}
print(d.get('c', 0), d.setdefault('c', 3))     # 0 3

# Never mutate a dict while iterating it — rebuild instead
d = {k: v for k, v in d.items() if v != 2}
print(d)                           # {'a': 1, 'c': 3}
```

**3.11 memory note:** dicts whose keys are all `str` no longer store per-entry hashes. What's New gives `sys.getsizeof(dict.fromkeys('abcdefg'))` shrinking from 352 to 272 bytes on 64-bit; exact figures vary by build.

##### `collections`

```python
from collections import ChainMap, Counter, defaultdict, deque

word_count: defaultdict[str, int] = defaultdict(int)
for word in 'a b a'.split():
    word_count[word] += 1

c = Counter('abracadabra')
print(c.most_common(2))            # [('a', 5), ('b', 2)]

env = ChainMap({'color': 'red'}, {'color': 'blue', 'user': 'guest'})
print(env['color'], env['user'])   # red guest

q: deque[str] = deque(maxlen=1000) # O(1) appends/pops at both ends
q.append('item')
print(q.popleft())                 # item
```

`Counter.__or__` is element-wise **max**, not a "last wins" merge. Never use `|` on `Counter`s expecting dict-merge semantics.

***

#### Sets

```python
a = {1, 2, 3, 4}
b = {3, 4, 5, 6}
print(a | b, a & b, a - b, a ^ b)  # {1, 2, 3, 4, 5, 6} {3, 4} {1, 2} {1, 2, 5, 6}
empty = set()                      # {} is an empty dict
frozen = frozenset({1, 2})         # hashable — usable as a dict key
```

***

#### Control Flow

```python
# for/else — else runs only if the loop did not break
def find_first_negative(values: list[int]) -> int | None:
    for v in values:
        if v < 0:
            result = v
            break
    else:
        result = None
    return result

print(find_first_negative([3, -1, -2]))   # -1

# Walrus (3.8+)
import io
stream = io.BytesIO(b'x' * 100_000)
total = 0
while chunk := stream.read(65_536):
    total += len(chunk)
print(total)                        # 100000
```

##### Structural Pattern Matching (3.10, fully available)

```python
def handle(event: dict[str, object]) -> str:
    match event:
        case {'type': 'click', 'pos': (x, y)}:
            return f'click at {x},{y}'
        case {'type': 'key', 'key': str(key)} if key.isprintable():
            return f'key {key}'
        case {'type': str(other)}:
            return f'unhandled {other}'
        case _:
            return 'invalid'

print(handle({'type': 'click', 'pos': (1, 2)}))   # click at 1,2
print(handle({'type': 'key', 'key': 'a'}))        # key a
print(handle({}))                                  # invalid
```

**Pitfalls:** a bare name in a `case` pattern *captures*; it doesn't compare. `case RED:` matches anything and binds `RED`. Compare against constants with dotted names (`case Color.RED:`) or literals. Pair exhaustive `match` statements over enums with `assert_never()` (3.11, see Type Hints).

***

#### Functions

```python
def greet(name: str, greeting: str = 'Hello', /, *, punctuation: str = '!') -> str:
    """Positional-only before '/', keyword-only after '*'."""
    return f'{greeting}, {name}{punctuation}'

print(greet('Alice', 'Hi', punctuation='.'))   # Hi, Alice.

# Pitfall: mutable defaults are evaluated once
def append_bad(item: int, bucket: list[int] = []) -> list[int]:   # ❌
    bucket.append(item)
    return bucket

def append_good(item: int, bucket: list[int] | None = None) -> list[int]:   # ✅
    if bucket is None:
        bucket = []
    bucket.append(item)
    return bucket

append_bad(1)
print(append_bad(2), append_good(2))   # [1, 2] [2]
```

##### New in 3.11: `operator.call`

```python
import operator

print(operator.call(pow, 2, 10))       # 1024 — operator.call(f, *a, **kw) == f(*a, **kw)

callbacks = [lambda: 'a', lambda: 'b']
print(list(map(operator.call, callbacks)))   # ['a', 'b']
```

##### Closures and `functools`

```python
import functools
from collections.abc import Callable

def make_counter(start: int = 0) -> Callable[[], int]:
    count = start
    def counter() -> int:
        nonlocal count
        count += 1
        return count
    return counter

c = make_counter()
print(c(), c(), c())               # 1 2 3

funcs = [lambda: i for i in range(3)]          # ❌ late binding
print([f() for f in funcs])                    # [2, 2, 2]
funcs = [lambda i=i: i for i in range(3)]      # ✅ bind now
print([f() for f in funcs])                    # [0, 1, 2]

@functools.cache                   # unbounded (3.9+) — only for bounded argument spaces
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)

@functools.lru_cache(maxsize=1024) # bounded
def lookup(key: str) -> int:
    return len(key)

print(fib(50), lookup('abc'))      # 12586269025 3
```

**`functools.cached_property` pitfall (3.8–3.11):** it takes an undocumented lock that is **per property, not per instance**, while computing. Concurrent first accesses on *different* instances serialize behind the one lock. 3.12 removed the lock. It also fails on classes with `__slots__` and no `__dict__`.

**`@lru_cache` on methods** keeps `self` alive for the cache's lifetime and shares one cache across all instances. Prefer `cached_property`, or a module-level cache keyed by a stable identifier.

##### Generators and `itertools`

```python
import itertools
from collections.abc import Iterator

def fibonacci() -> Iterator[int]:
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

print(list(itertools.islice(fibonacci(), 8)))     # [0, 1, 1, 2, 3, 5, 8, 13]
print(list(itertools.pairwise([1, 2, 3])))        # [(1, 2), (2, 3)] — 3.10+

# itertools.batched() is 3.12+. In 3.11 write it yourself:
def batched(iterable, n: int) -> Iterator[tuple]:
    if n < 1:
        raise ValueError('n must be at least one')
    it = iter(iterable)
    while batch := tuple(itertools.islice(it, n)):
        yield batch

print(list(batched('ABCDEFG', 3)))   # [('A', 'B', 'C'), ('D', 'E', 'F'), ('G',)]
```

***

#### Dataclasses

```python
from dataclasses import KW_ONLY, dataclass, field

@dataclass(slots=True, kw_only=True)          # slots/kw_only: 3.10+
class User:
    id: int
    name: str
    tags: list[str] = field(default_factory=list)

@dataclass(frozen=True, slots=True)
class Coordinate:
    lat: float
    lon: float

@dataclass
class Event:
    name: str
    _: KW_ONLY                                 # fields after this are keyword-only
    priority: int = 0

u = User(id=1, name='a')
print(u, Coordinate(1.0, 2.0) == Coordinate(1.0, 2.0))
# User(id=1, name='a', tags=[]) True
print(Event('deploy', priority=2))             # Event(name='deploy', priority=2)
```

##### New in 3.11: Stricter Mutable-Default Check

Before 3.11, only `list`, `dict` and `set` defaults were rejected. Since 3.11, **any unhashable default** is rejected, which includes instances of ordinary (non-frozen) dataclasses and of subclasses of `list`/`dict`/`set`.

```python
from dataclasses import dataclass, field

@dataclass
class Settings:
    retries: int = 3

try:
    @dataclass
    class Service:
        settings: Settings = Settings()        # ❌ ValueError in 3.11 (silently shared in 3.10)
except ValueError as e:
    print(e)   # mutable default <class '__main__.Settings'> for field settings is not allowed: use default_factory

@dataclass
class ServiceFixed:
    settings: Settings = field(default_factory=Settings)   # ✅ one instance per object

@dataclass(frozen=True)
class Limits:
    max_rps: int = 100

@dataclass
class Service2:
    limits: Limits = Limits()                  # OK — frozen dataclasses are hashable
print(ServiceFixed(), Service2())
# ServiceFixed(settings=Settings(retries=3)) Service2(limits=Limits(max_rps=100))
```

The check uses hashability (`__hash__ is None`), not true mutability. A hashable but mutable object still slips through. Upgrading to 3.11 can therefore break import of modules that were relying on a shared default instance. That was always a latent bug.

##### New in 3.11: `weakref_slot=True`

```python
import weakref
from dataclasses import dataclass

@dataclass(slots=True, weakref_slot=True)      # slots=True alone makes instances non-weakref-able
class Node:
    value: int

n = Node(1)
r = weakref.ref(n)
print(r() is n)                                # True
```

***

#### Classes and OOP

##### Class Definition

```python
import math
from typing import Self

class Shape:
    count: int = 0                             # class attribute — shared

    def __init__(self, color: str = 'black') -> None:
        self.color = color
        Shape.count += 1

    def __repr__(self) -> str:
        return f'{type(self).__name__}(color={self.color!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, type(self)):
            return NotImplemented
        return self.__dict__ == other.__dict__
    # Defining __eq__ sets __hash__ = None — correct for a MUTABLE class.

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

print(Circle.unit(), round(Circle(2).area, 2))   # Circle(color='black') 12.57
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
    def close(self) -> None:
        pass

print(isinstance(Conn(), SupportsClose))   # True — structural check (method names only)
try:
    Repository()
except TypeError as e:
    print('abstract:', 'abstract' in str(e))   # abstract: True
```

**New in 3.11:** the `__init__` method of a `Protocol` subclass is preserved, where earlier it was silently replaced.

##### New in 3.11: `object.__getstate__()`

`object` now provides a default `__getstate__()`. Copying and pickling instances of subclasses of built-in types (`set`, `frozenset`, `bytearray`, `OrderedDict`, `deque`, `WeakSet`, `tzinfo`) now includes attributes stored in `__slots__`.

```python
class Plain:
    pass

class Slotted:
    __slots__ = ('x',)

p = Plain()
print(p.__getstate__())            # None — empty __dict__
p.a = 1
print(p.__getstate__())            # {'a': 1}
s = Slotted()
s.x = 1
print(s.__getstate__())            # (None, {'x': 1})
```

**Pitfall:** code that checks `hasattr(obj, '__getstate__')` to decide whether an object "customizes pickling" is now always true. Check `type(obj).__getstate__ is not object.__getstate__` instead.

##### Deprecated in 3.11: Chaining `classmethod` Descriptors

Wrapping other descriptors in `classmethod`, e.g. `@classmethod` over `@property`, was added in 3.9 and deprecated in 3.11. 3.11 emits no runtime warning for it, and **3.13 removes it**. Replace "class properties" with a metaclass property or a plain class attribute.

##### Enums

See **New in 3.11: `enum` Overhaul** above. Prefer `StrEnum` over `(str, Enum)`, and audit `str()`/f-strings of `IntEnum`/`IntFlag` members.

***

#### Exception Handling

```python
import json
import logging

logger = logging.getLogger(__name__)

class AppError(Exception):
    """Base for all application-specific errors."""

class ConfigError(AppError):
    def __init__(self, key: str, message: str = '') -> None:
        self.key = key
        super().__init__(f'Config error for {key!r}' + (f': {message}' if message else ''))

def read_config(text: str) -> dict[str, object]:
    try:
        return json.loads(text)
    except json.JSONDecodeError as e:
        raise ConfigError('file', 'invalid JSON') from e     # chain with `from`

try:
    read_config('{bad')
except ConfigError as e:
    print(e, '| cause:', type(e.__cause__).__name__)
    # Config error for 'file': invalid JSON | cause: JSONDecodeError
```

##### New in 3.11: `sys.exception()` and the Simplified Exception State

```python
import sys

try:
    raise KeyError('k')
except KeyError:
    print(repr(sys.exception()))   # KeyError('k') — equivalent to sys.exc_info()[1]
print(sys.exception())             # None outside a handler
```

- `sys.exc_info()` now derives type and traceback from the exception instance. Modifying `e.__traceback__` inside a handler is reflected in later `exc_info()` calls and in a bare `raise`.
- **Bare `raise` re-raises with the current `__traceback__`** of the active exception. Keep using bare `raise` (never `raise e`) to preserve the original traceback.

**Rules (unchanged):**

- Catch the narrowest exception type. Never use bare `except:`. Use `except Exception:` only at process or request boundaries, and always log or re-raise there.
- Never swallow `asyncio.CancelledError`, `KeyboardInterrupt` or `SystemExit`. These are `BaseException` subclasses and `except Exception:` correctly does not catch them.
- For concurrent or independent failures, use `ExceptionGroup`/`except*`. To add context without changing the exception type, use `add_note()`.

***

#### Context Managers

```python
import contextlib
import time
from collections.abc import Iterator

@contextlib.contextmanager
def timed(name: str) -> Iterator[None]:
    start = time.perf_counter()
    try:
        yield
    finally:
        print(f'{name} took {time.perf_counter() - start:.0f}s')

with timed('noop'):                 # noop took 0s
    pass

# Parenthesized multi-item with (officially 3.10+)
with (
    open(__file__, encoding='utf-8') as a,
    open(__file__, encoding='utf-8') as b,
):
    print(a.readline() == b.readline())   # True
```

##### New in 3.11: `contextlib.chdir`

```python
import contextlib
import os
import tempfile

before = os.getcwd()
with tempfile.TemporaryDirectory() as tmp:
    with contextlib.chdir(tmp):
        print(os.path.samefile(os.getcwd(), tmp))   # True
print(os.getcwd() == before)                       # True — restored on exit
```

`chdir()` changes the **process-wide** working directory, so it is not safe with threads or asyncio tasks running concurrently. The docs call it "non parallel-safe". In libraries and servers, pass absolute paths or `cwd=` to `subprocess` instead.

##### New in 3.11: `TypeError` (Not `AttributeError`) for Non-Context-Managers

```python
try:
    with object():                 # ❌ not a context manager
        pass
except TypeError as e:
    print(e)                       # 'object' object does not support the context manager protocol
```

3.10 raised `AttributeError: __enter__` here, and the same change applies to `async with`, `ExitStack.enter_context()` and `AsyncExitStack.enter_async_context()`. Tests asserting `AttributeError` must be updated.

***

#### File I/O and `pathlib`

Always pass `encoding='utf-8'` to text-mode `open()`, `Path.read_text()` and `Path.write_text()`.

```python
import tempfile
from pathlib import Path

with tempfile.TemporaryDirectory() as tmp:
    base = Path(tmp)
    report = base / 'reports' / 'output.csv'
    report.parent.mkdir(parents=True, exist_ok=True)
    report.write_text('a,b\n', encoding='utf-8')
    (base / 'sub').mkdir()

    print(report.name, report.stem, report.suffix)   # output.csv output .csv
    print(report.read_text(encoding='utf-8'), end='')   # a,b

    # New in 3.11: a pattern ending in a separator returns only directories
    print(sorted(p.name for p in base.glob('*/')))    # ['reports', 'sub']
    report.unlink(missing_ok=True)
```

**Removed in 3.11:** the `'U'` (universal newlines) mode flag. `open(path, 'rU')` now raises `ValueError: invalid mode: 'rU'`. Universal newlines have been the default for text mode since Python 3.0, so just drop the `U`.

```python
try:
    open(__file__, 'rU')
except ValueError as e:
    print(e)                        # invalid mode: 'rU'
```

`pathlib.Path.walk()` is **3.12+**. In 3.11 use `os.walk()`, or `Path.rglob('*')` with `is_dir()` filters.

##### New in 3.11: `hashlib.file_digest()`

```python
import hashlib
import io

with open(__file__, 'rb') as f:                # must be a binary file object
    digest = hashlib.file_digest(f, 'sha256').hexdigest()
print(len(digest))                             # 64

print(hashlib.file_digest(io.BytesIO(b'abc'), hashlib.sha256).hexdigest()[:16])   # ba7816bf8f01cfea

try:
    with open(__file__, encoding='utf-8') as f:    # ❌ text mode
        hashlib.file_digest(f, 'sha256')
except ValueError:
    print('text-mode files are rejected')          # text-mode files are rejected
```

`file_digest()` reads in chunks, avoiding the classic `hashlib.sha256(path.read_bytes())` that loads the whole file into memory. It may also bypass Python-level buffering for real files.

##### Temporary Files

```python
import tempfile

with tempfile.NamedTemporaryFile(mode='w', suffix='.csv', encoding='utf-8') as tmp:
    tmp.write('col1,col2\n')
    tmp.flush()

with tempfile.TemporaryDirectory() as tmpdir:
    pass

# NEVER use tempfile.mktemp() — TOCTOU race. Use mkstemp()/NamedTemporaryFile.
```

`SpooledTemporaryFile` now fully implements `io.BufferedIOBase`/`io.TextIOBase` (3.11), so it works with APIs that need a real file-like object (e.g. `zipfile`, compression modules). The `delete_on_close` parameter of `NamedTemporaryFile` is **3.12+**.

***

#### `async`/`await` and `asyncio`

The landmark 3.11 additions (`TaskGroup`, `timeout`) are covered above in **Structured Concurrency**. This section covers the rest.

##### Entry Points: `asyncio.run()` and New in 3.11 `asyncio.Runner`

```python
import asyncio

async def step(n: int) -> int:
    await asyncio.sleep(0)
    return n

print(asyncio.run(step(1)))          # 1 — the standard entry point

# Runner: several top-level calls on ONE loop and contextvars context
with asyncio.Runner() as runner:
    print(runner.run(step(2)), runner.run(step(3)))   # 2 3
```

Use `Runner` for CLIs or test harnesses that need several top-level awaits sharing loop-bound resources (connection pools). Don't call `asyncio.run()` repeatedly for that: each call creates and closes a new loop, which breaks loop-bound objects.

##### Core Patterns

```python
import asyncio

async def worker(name: str, queue: asyncio.Queue[int], out: list[str]) -> None:
    while True:
        item = await queue.get()
        try:
            out.append(f'{name}:{item}')
        finally:
            queue.task_done()

async def main() -> None:
    queue: asyncio.Queue[int] = asyncio.Queue(maxsize=100)   # bounded → backpressure
    out: list[str] = []
    workers = [asyncio.create_task(worker(f'w{i}', queue, out)) for i in range(3)]
    for i in range(6):
        await queue.put(i)
    await queue.join()
    for w in workers:
        w.cancel()
    await asyncio.gather(*workers, return_exceptions=True)   # reap cancelled workers
    print(len(out))                                           # 6

asyncio.run(main())
```

**Rules:**

- **Keep references to fire-and-forget tasks.** The event loop holds only weak references, so an unreferenced task can be garbage-collected mid-flight. Prefer `TaskGroup`. Otherwise store the tasks in a set and discard each one in a done-callback.
- **Always re-raise `CancelledError`** after cleanup. In 3.11 it matters even more, because `TaskGroup` and `timeout()` rely on cancellation (`Task.cancelling()`/`uncancel()` are new 3.11 internals).
- **Inside coroutines, call `asyncio.get_running_loop()`, not `get_event_loop()`.**
- **Run blocking calls with `await asyncio.to_thread(func, *args)`** (3.9+). Pass the callable, not the result of calling it.
- **New in 3.11:** `asyncio.Barrier`, `StreamWriter.start_tls()`, `loop.sock_sendto()`/`sock_recvfrom()`, and `create_connection(..., all_errors=True)`, which raises an `ExceptionGroup` of every failed address attempt.

##### Removed in 3.11: `@asyncio.coroutine`

```python
import asyncio

print(hasattr(asyncio, 'coroutine'))   # False — removed in 3.11 (deprecated since 3.8)

# ❌ legacy generator-based coroutine — AttributeError at import time on 3.11
# @asyncio.coroutine
# def old():
#     yield from asyncio.sleep(1)

async def new() -> None:               # ✅
    await asyncio.sleep(0)
```

Also removed: `asyncio.coroutines.CoroWrapper`, and the `reuse_address` parameter of `loop.create_datagram_endpoint()` (a security fix). `loop.set_default_executor()` now rejects anything but a `ThreadPoolExecutor`.

***

#### Concurrency

##### The GIL

CPython 3.11 has one GIL. Threads help with I/O-bound work and with C extensions that release the GIL. CPU-bound pure-Python code needs processes. 3.11's interpreter speedups don't change that. The experimental free-threaded build is 3.13+.

##### `concurrent.futures`

```python
from concurrent.futures import ThreadPoolExecutor, as_completed

def fetch(n: int) -> int:
    return n * n

with ThreadPoolExecutor(max_workers=4) as pool:
    futures = {pool.submit(fetch, n): n for n in range(5)}
    results = {futures[f]: f.result() for f in as_completed(futures)}
print(dict(sorted(results.items())))   # {0: 0, 1: 1, 2: 4, 3: 9, 4: 16}
```

- **New in 3.11:** `concurrent.futures.TimeoutError` is the builtin `TimeoutError`, so `except TimeoutError:` covers `future.result(timeout=...)`.
- `shutdown(cancel_futures=True)` (3.9+) drops pending work. Never wait with `as_completed()`/`wait()` on futures the shutdown may have cancelled: that can hang (bpo-43727).

##### `threading` and `time`

- **New in 3.11:** on Unix with glibc 2.30+, `threading.Lock.acquire(timeout=...)` uses the monotonic clock, so system clock changes no longer distort timeouts.
- **New in 3.11:** `time.sleep()` uses `clock_nanosleep()`/`nanosleep()` on Unix (1 ns resolution) and high-resolution waitable timers on Windows 8.1+. Very short sleeps are more precise but not free. Don't busy-loop on `time.sleep(0)`.
- Always protect shared mutable state with a `Lock`. The GIL makes individual bytecodes atomic, not compound operations like `counter += 1`.

```python
import threading

lock = threading.Lock()
counter = 0

def work() -> None:
    global counter
    for _ in range(10_000):
        with lock:
            counter += 1

threads = [threading.Thread(target=work) for _ in range(4)]
for t in threads:
    t.start()
for t in threads:
    t.join()
print(counter)                     # 40000
```

##### `multiprocessing`

```python
from multiprocessing import get_context

def square(x: int) -> int:
    return x * x

if __name__ == '__main__':          # required with the spawn/forkserver start methods
    with get_context('spawn').Pool(processes=2) as pool:
        print(pool.map(square, range(5)))   # [0, 1, 4, 9, 16]
```

On Linux the default start method in 3.11 is still `fork`. Forking a process that has started threads can deadlock. Pass an explicit context (`spawn` or `forkserver`) in code that also uses threads. 3.12 adds a `DeprecationWarning` for fork-with-threads, and 3.14 changes the Linux default to `forkserver`.

***

#### Logging

```python
import logging
import sys

def setup_logging(level: str = 'INFO') -> None:
    levels = logging.getLevelNamesMapping()     # New in 3.11: {'CRITICAL': 50, ...}
    if level.upper() not in levels:
        raise ValueError(f'unknown log level {level!r}; choose from {sorted(levels)}')
    logging.basicConfig(
        level=levels[level.upper()],
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        stream=sys.stdout,
        force=True,
    )

setup_logging('info')
logger = logging.getLogger(__name__)
logger.info('Loaded %d records', 3)       # %-style: formatting deferred until emitted
```

Use `getLevelNamesMapping()` instead of the undocumented `logging._nameToLevel`, or of `getLevelName()` (which maps both ways and returns `'Level X'` strings for unknown values).

**Exception groups in logs:** `logger.exception()` renders groups as an indented tree, including notes:

```python
import logging
import sys

logging.basicConfig(stream=sys.stdout, format='%(levelname)s %(message)s', force=True)
log = logging.getLogger('batch')

try:
    err = ValueError('bad row')
    err.add_note('row=3')
    raise ExceptionGroup('batch failed', [err])
except ExceptionGroup:
    log.exception('batch import failed')
# ERROR batch import failed
#   + Exception Group Traceback (most recent call last):
#   ...
#   | ExceptionGroup: batch failed (1 sub-exception)
#   +-+---------------- 1 ----------------
#     | ValueError: bad row
#     | row=3
```

Structured or JSON log formatters that serialize only `str(exc)` lose both the sub-exceptions and the notes. Use `traceback.format_exception(exc)` or walk `exc.exceptions`.

***

#### Security

##### New in 3.11: `-P` / `PYTHONSAFEPATH` — Don't Import From the Script Directory

By default, Python prepends the script's directory to `sys.path` (or the current directory, for `-c` and `-m`). A file named `json.py` or `logging.py` that an attacker can write into that directory then shadows the stdlib module. **New in 3.11:** `python -P` or `PYTHONSAFEPATH=1` disables the prepending.

```text
$ ls
json.py   main.py            # json.py is a malicious/accidental shadow
$ python3.11 main.py         # imports ./json.py
$ python3.11 -P main.py      # imports the stdlib json
$ PYTHONSAFEPATH=1 python3.11 main.py
```

```python
import sys

print(sys.flags.safe_path)   # True under -P, PYTHONSAFEPATH=1 or -I (isolated mode implies -P)
```

**Review guidance:** set `PYTHONSAFEPATH=1` in container images and systemd units, and add `-P` to the shebangs of admin scripts that run in user-writable directories. `-I` (isolated mode) also implies safe-path behavior. Don't use `-P` where the application *relies* on importing sibling modules of the entry script; install the package instead.

##### Security-Relevant Changes Across 3.11 Point Releases

Running "3.11" is not enough. These fixes exist only in later patch releases:

| Release | Fix |
|---|---|
| 3.11.0 | int/str conversion limit (CVE-2020-10735). `multiprocessing` on Linux stops using abstract-namespace sockets for forkserver (CVE-2022-42919). |
| 3.11.1 | `python -m http.server` no longer logs terminal control characters. IDNA codec decoding DoS fix. |
| 3.11.3 | Windows: `subprocess` shell search order made safer. |
| **3.11.4** | **`tarfile` extraction filters** (`filter='data'`, PEP 706). `urllib.parse.urlsplit()` strips leading C0 control and space characters (CVE-2023-24329). |
| 3.11.5 | `ssl.SSLSocket` TLS-handshake bypass fix (CVE-2023-40217). |
| 3.11.8 | `site` skips `.pth` files whose names start with a dot or are hidden. |
| 3.11.9 | Expat ≥ 2.6.0 reparse-deferral controls (CVE-2023-52425). |
| 3.11.10 | `email` refuses to serialize headers containing newlines (header injection). `email.utils.parseaddr()`/`getaddresses()` gain `strict=True` by default (CVE-2023-27043). `ipaddress` `is_private`/`is_global` fixed. Windows `os.mkdir(mode=0o700)` access control (CVE-2024-4030). |
| 3.11.13 | `tarfile` filter bypasses fixed (CVE-2024-12718, CVE-2025-4138, CVE-2025-4330, CVE-2025-4435). `os.path.realpath(strict=os.path.ALLOW_MISSING)` (CVE-2025-4517). |
| 3.11.14–3.11.17 | Many more `tarfile` filter bypasses closed; `zipfile`, `html.parser`, `http.client`, `http.cookies` and `email` DoS/injection fixes; bundled libexpat updates. 3.11.17 adds `DeprecationWarning`s when `ssl` hostname checking is enabled but no `server_hostname` is passed. |

`python.org` publishes no binary installers after 3.11.9. On Windows and macOS, installer-based deployments are stuck at 3.11.9 and **miss every later fix**. Use distro packages, containers or `uv`-managed builds that track the latest 3.11.x, or move to a newer minor version.

##### Archive Extraction — Always Pass a `filter` (3.11.4+)

```python
import io
import tarfile
import tempfile

buf = io.BytesIO()
with tarfile.open(fileobj=buf, mode='w') as tar:
    evil = tarfile.TarInfo('../escape.txt')
    evil.size = 4
    tar.addfile(evil, io.BytesIO(b'evil'))
buf.seek(0)

with tempfile.TemporaryDirectory() as dest, tarfile.open(fileobj=buf) as tar:
    try:
        tar.extractall(dest, filter='data')          # ✅ rejects path traversal, devices, setuid…
    except tarfile.FilterError as e:
        print(type(e).__name__)                      # OutsideDestinationError
```

Without `filter=`, 3.11 keeps the old, unsafe behavior and emits no warning. The `DeprecationWarning` for a missing filter starts in 3.12, and the default becomes `'data'` in 3.14. For untrusted archives on 3.11, **always pass `filter='data'`**. `shutil.unpack_archive(..., filter='data')` is also supported since 3.11.4. The filter API was also backported to security releases of older branches. Code that must run on unknown patch levels can feature-detect it with `hasattr(tarfile, 'data_filter')`, as PEP 706 recommends, and refuse to extract when it is missing.

##### Input Validation and Injection

```python
import sqlite3
import subprocess
import sys

conn = sqlite3.connect(':memory:')
conn.execute('CREATE TABLE users (id INTEGER, name TEXT)')
user_id = '1 OR 1=1'
rows = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,)).fetchall()   # ✅ parameterized
print(rows)                                    # []

# Subprocess: list form, no shell=True with user input
result = subprocess.run(
    [sys.executable, '-c', 'print("ok")'],
    capture_output=True, text=True, timeout=30, check=True,
)
print(result.stdout.strip())                   # ok
```

- **Annotate SQL and shell-command parameters as `LiteralString` (3.11)** so pyright rejects f-string-built queries. mypy does not enforce `LiteralString`. This complements parameterization; it doesn't replace it.
- **`eval()`/`exec()`/`pickle.load()` never touch untrusted input.**
- **Path traversal:** call `resolve()` first, then `is_relative_to(base)` (3.9+).
- **XML:** use `defusedxml` for untrusted documents. The stdlib parsers are hardened against some attacks, but not all of them.

##### Secrets and Cryptography

```python
import hmac
import secrets

token = secrets.token_urlsafe(32)              # session tokens, reset links
api_key = secrets.token_hex(32)
print(len(api_key))                            # 64
print(hmac.compare_digest(api_key, api_key))   # True — constant-time comparison

# ❌ random is a deterministic PRNG — never for security values
```

`hashlib` in 3.11 prefers OpenSSL's SHA-3/SHAKE implementations and the system `libb2` for BLAKE2. Results are identical; only performance and FIPS behavior change. In FIPS environments, pass `usedforsecurity=False` for non-security hashes such as cache keys: `hashlib.md5(data, usedforsecurity=False)` (3.9+).

***

#### JSON and Serialization

```python
import json
from datetime import UTC, datetime
from enum import StrEnum

class Status(StrEnum):
    OK = 'ok'

class AppEncoder(json.JSONEncoder):
    def default(self, obj: object) -> object:
        if isinstance(obj, datetime):
            return obj.isoformat()
        if isinstance(obj, set):
            return sorted(obj)
        return super().default(obj)

payload = {'ts': datetime(2024, 1, 1, tzinfo=UTC), 'status': Status.OK, 'tags': {'b', 'a'}}
print(json.dumps(payload, cls=AppEncoder))
# {"ts": "2024-01-01T00:00:00+00:00", "status": "ok", "tags": ["a", "b"]}
```

- **New in 3.11:** `json.loads()` integers obey the int/str digit limit. A 5000-digit JSON number raises `ValueError` (see Integers). Pass `parse_int=` explicitly if you must accept huge numbers from a trusted source.
- `json` never calls `format()` on enum members, so the 3.11 enum formatting change doesn't affect it. `json.dumps(Color.RED)` emits the value for both `StrEnum` and `(str, Enum)`.

##### Pickle

```python
import pickle

print(pickle.DEFAULT_PROTOCOL, pickle.HIGHEST_PROTOCOL)   # 4 5
data = pickle.dumps({'a': 1}, protocol=pickle.HIGHEST_PROTOCOL)
print(pickle.loads(data))                                # {'a': 1}
```

**Never unpickle untrusted data.** It executes arbitrary code. The default protocol is 4 in 3.11 and becomes 5 in 3.14. **New in 3.11:** the default `object.__getstate__()` changes pickles of slotted subclasses of built-ins (see Classes). Re-test custom `__reduce__`/`__getstate__` code after upgrading.

***

#### CSV

```python
import csv
import io

buf = io.StringIO(newline='')                  # files: open(path, 'w', encoding='utf-8', newline='')
writer = csv.DictWriter(buf, fieldnames=['name', 'age'])
writer.writeheader()
writer.writerow({'name': 'Alice', 'age': 30})
buf.seek(0)
print(list(csv.DictReader(buf)))               # [{'name': 'Alice', 'age': '30'}]

try:
    csv.writer(io.StringIO(), escapechar='')   # Changed in 3.11: empty escapechar rejected
except TypeError as e:
    print(e)                                   # "escapechar" must be a 1-character string
```

Always open CSV files with `newline=''` and an explicit `encoding`. Values read back are always `str`. To stop CSV formula injection when exporting user data for spreadsheets, prefix cells that start with `=`, `+`, `-` or `@`.

***

#### Regular Expressions

```python
import re

DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')
m = DATE_RE.fullmatch('2024-06-15')
print(m['year'] if m else None)                # 2024
```

##### New in 3.11: Atomic Groups and Possessive Quantifiers

`(?>...)` and the possessive quantifiers `*+`, `++`, `?+` and `{m,n}+` match without ever giving characters back. They are the standard tool against **catastrophic backtracking** (ReDoS).

```python
import re

print(re.fullmatch(r'a+a', 'aaa'))             # <re.Match object; span=(0, 3), match='aaa'> — backtracks
print(re.fullmatch(r'a++a', 'aaa'))            # None — possessive a++ keeps every 'a'
print(re.findall(r'(?>a+)b', 'aaab'))          # ['aaab']

# A quoted-string matcher that cannot backtrack exponentially on malformed input
QUOTED = re.compile(r'"(?>[^"\\]+|\\.)*+"')
print(QUOTED.fullmatch(r'"say \"hi\""') is not None)   # True
```

Possessive quantifiers change *what* matches, not just how fast. Use them only where giving characters back could never produce a valid match.

##### Changed in 3.11: Stricter Parsing

```python
import re
import warnings

try:
    re.compile(r'a(?i)b')                      # global inline flag not at the start
except re.error as e:
    print(e)                                   # global flags not at the start of the expression at position 1

print(re.compile(r'(?i)ab').match('AB') is not None)   # True — put global flags first
print(re.compile(r'a(?i:b)').match('aB') is not None)  # True — or use a scoped group (3.6+)

with warnings.catch_warnings(record=True) as rec:
    warnings.simplefilter('always')
    re.sub('(a)', r'\g<１>', 'a')              # non-ASCII digit in a group reference
print(rec[0].category.__name__)                # DeprecationWarning
```

- Global inline flags (`(?i)`, `(?x)` …) in the middle of a pattern raise `re.error` in 3.11. They were deprecated since 3.6.
- Numeric group references must be ASCII digits, and group names in `bytes` patterns must be ASCII identifiers. Violations emit `DeprecationWarning` and become errors in 3.12.
- `re.template()`/`re.TEMPLATE`/`re.T` are deprecated (removed in 3.13). The undocumented `sre_compile`, `sre_constants` and `sre_parse` modules are deprecated.
- **New in 3.11:** `re.NOFLAG` (value 0) is a clean default for flag parameters. `re.RegexFlag` is now an exported `IntFlag`.
- Some long-standing capture-group bugs were fixed. In rare patterns, captured groups differ from 3.10.
- The engine uses computed gotos and is up to 10% faster.

***

#### Memory Management

```python
import gc
import sys
import tracemalloc
import weakref

x = [1, 2, 3]
print(sys.getrefcount(x) >= 2)            # True (includes the temporary argument reference)

tracemalloc.start()
data = [bytes(1000) for _ in range(100)]
current, peak = tracemalloc.get_traced_memory()
print(current > 0, peak >= current)       # True True
tracemalloc.stop()

class Cache:
    def __init__(self) -> None:
        self._data: weakref.WeakValueDictionary[str, object] = weakref.WeakValueDictionary()

gc.collect()
```

**3.11 memory changes:**

- Instance `__dict__`s are created lazily and share keys more aggressively, so plain classes use less memory than before. `__slots__` still wins for millions of small objects.
- Dicts with all-`str` keys skip stored hashes (smaller).
- Code objects carry PEP 657 position tables (slightly larger). Strip them with `-X no_debug_ranges` if needed.
- Overall, What's New expects memory to be at most about 20% higher than 3.10 and often lower.

***

#### Performance

```python
import cProfile
import io
import pstats
import timeit

def work() -> int:
    return sum(i * i for i in range(10_000))

with cProfile.Profile() as pr:                 # context-manager form (3.8+)
    work()
stream = io.StringIO()
pstats.Stats(pr, stream=stream).sort_stats('cumulative').print_stats(5)
print('work' in stream.getvalue())             # True

print(timeit.timeit(work, number=10) > 0)      # True
```

**3.11-specific guidance:**

- **Measure on 3.11 before optimizing.** Many idioms that used to be "slow" (`try` blocks, method calls, global lookups, `%` formatting of literals) got much cheaper. Benchmarks taken on 3.10 or earlier are stale.
- **Optimizations outside the Faster CPython project:** `sum()` of small ints is about 30% faster. `//` by small ints is about 20% faster. `math.comb()`/`math.perm()` are about 10× faster for large arguments. `statistics.mean()`/`variance()`/`stdev()` now consume iterators in one pass. `unicodedata.normalize()` is constant-time for pure ASCII.
- **Profilers and tracers cost more relative to the faster baseline.** `sys.settrace`-based tools (coverage, some profilers) add a larger *relative* overhead on top of the faster 3.11 interpreter, so never benchmark under coverage. `sys.monitoring` (PEP 669), the low-overhead alternative, is 3.12+.
- **Prefer `set`/`frozenset` for membership tests and `''.join()` for building strings.** Complexity still beats interpreter speed.
- **Startup:** frozen core modules make 3.11 start 10–15% faster. For CLI tools, also check `python -X importtime` for heavy imports.

***

#### Testing

##### New in 3.11: `enterContext()` and Friends

```python
import os
import tempfile
import unittest
from contextlib import chdir

class TestWorkdir(unittest.TestCase):
    def setUp(self) -> None:
        # enters the CM now and registers its __exit__ with addCleanup()
        self.tmp = self.enterContext(tempfile.TemporaryDirectory())
        self.enterContext(chdir(self.tmp))

    def test_cwd(self) -> None:
        self.assertTrue(os.path.samefile(os.getcwd(), self.tmp))

suite = unittest.defaultTestLoader.loadTestsFromTestCase(TestWorkdir)
result = unittest.TextTestRunner(verbosity=0, stream=open(os.devnull, 'w')).run(suite)
print(result.wasSuccessful())       # True
```

New in 3.11: `TestCase.enterContext()`, `TestCase.enterClassContext()`, `IsolatedAsyncioTestCase.enterAsyncContext()` and `unittest.enterModuleContext()`. They replace the `cm.__enter__()` + `addCleanup(cm.__exit__, None, None, None)` boilerplate.

##### Testing Exception Groups

`assertRaises(ValueError)` doesn't match a group that contains a `ValueError`. Assert on the group, then inspect it:

```python
import unittest

def run_batch() -> None:
    raise ExceptionGroup('batch', [ValueError('row 1'), ValueError('row 7')])

class TestBatch(unittest.TestCase):
    def test_reports_all_rows(self) -> None:
        with self.assertRaises(ExceptionGroup) as cm:
            run_batch()
        match, rest = cm.exception.split(ValueError)
        self.assertIsNone(rest)
        self.assertEqual([str(e) for e in match.exceptions], ['row 1', 'row 7'])

suite = unittest.defaultTestLoader.loadTestsFromTestCase(TestBatch)
import io
print(unittest.TextTestRunner(stream=io.StringIO()).run(suite).wasSuccessful())   # True
```

With pytest: `with pytest.raises(ExceptionGroup) as excinfo:` then `assert excinfo.group_contains(ValueError)` (pytest ≥ 8.0), or `pytest.RaisesGroup(ValueError, ValueError)` (pytest ≥ 8.4).

##### Other 3.11 Testing Changes

- **Returning a non-`None` value from a test method is deprecated** and emits `DeprecationWarning`. It usually means a test that forgot to `assert`.
- **`unittest` discovery no longer supports namespace packages** as the start directory. Add `__init__.py` or point discovery at a regular package.
- `unittest.makeSuite()`, `findTestCases()` and `getTestCaseNames()` are deprecated (removed in 3.13). Use `TestLoader` methods.
- The deprecated assertion aliases (`assertEquals`, `assertRegexpMatches`, `failUnless`, …) still exist in 3.11 but are **removed in 3.12**. Fix them now.
- Run the suite with `-W error::DeprecationWarning` (or pytest's `filterwarnings = error`) to surface every 3.12/3.13 removal before you upgrade.

```python
import asyncio
import unittest
from unittest.mock import AsyncMock

class TestAsync(unittest.IsolatedAsyncioTestCase):
    async def test_fetch(self) -> None:
        fetch = AsyncMock(return_value={'id': 1})
        async with asyncio.timeout(1):            # 3.11 timeout inside async tests
            result = await fetch(1)
        fetch.assert_awaited_once_with(1)
        self.assertEqual(result['id'], 1)

import io
suite = unittest.defaultTestLoader.loadTestsFromTestCase(TestAsync)
print(unittest.TextTestRunner(stream=io.StringIO()).run(suite).wasSuccessful())   # True
```

***

#### Virtual Environments and Packaging

```bash
python3.11 -m venv .venv
source .venv/bin/activate
python -m pip install -r requirements.txt

# uv: fast resolver/installer that can also provision the latest 3.11.x
uv venv --python 3.11
uv pip install -r requirements.txt
```

##### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=69"]
build-backend = "setuptools.build_meta"

[project]
name = "myapp"
version = "1.0.0"
requires-python = ">=3.11"
dependencies = [
    "tzdata; sys_platform == 'win32'",
]

[tool.ruff]
target-version = "py311"
line-length = 88

[tool.ruff.lint]
select = ["E", "F", "I", "UP", "B", "S", "ASYNC"]

[tool.mypy]
python_version = "3.11"
strict = true

[tool.pytest.ini_options]
testpaths = ["tests"]
filterwarnings = ["error::DeprecationWarning"]
```

**Reading your own `pyproject.toml` at runtime** uses `tomllib`. Opening the file in binary mode is mandatory:

```python
import tomllib
from pathlib import Path

def load_project_version(pyproject: Path) -> str | None:
    if not pyproject.is_file():
        return None
    with pyproject.open('rb') as f:
        return tomllib.load(f).get('project', {}).get('version')

print(load_project_version(Path('nonexistent/pyproject.toml')))   # None
```

For the *installed* version of a distribution, use `importlib.metadata.version('myapp')`. Don't parse `pyproject.toml` from inside an installed package: it isn't shipped there.

**Packaging-related 3.11 facts:**

- **`distutils` still ships in 3.11 but is deprecated** (PEP 632) and is **removed in 3.12**. Replace `from distutils...` with `setuptools`, `packaging.version` or `sysconfig` now. The same goes for `imp` (use `importlib`), and `asynchat`/`asyncore`/`smtpd` (use `asyncio`, `aiosmtpd`); all of them are removed in 3.12.
- New `venv` sysconfig install scheme (`posix_venv`/`nt_venv`). Tools that create virtual environments should use the `venv` scheme.
- The deprecated `importlib.resources` functions (`read_text()`, `open_binary()`, `path()`, …) emit `DeprecationWarning` in 3.11. Use `importlib.resources.files(pkg).joinpath(name).read_text(encoding='utf-8')`, which works on 3.9+. (3.12 stopped emitting the warning and 3.13 formally un-deprecated the old functions with a new signature. `files()` is still the portable choice.)

***

#### Code Quality Tools

```bash
pip install ruff mypy

ruff check --target-version py311 src/ tests/     # lint (UP rules modernize to 3.11 idioms)
ruff format src/ tests/                            # Black-compatible formatter
mypy --python-version 3.11 --strict src/           # type checking
pyupgrade --py311-plus $(git ls-files '*.py')      # optional, overlaps ruff UP
```

**Target-version settings matter.** With `target-version = "py311"`, ruff and pyupgrade:

- rewrite `Optional[X]` → `X | None` and `List[X]` → `list[X]`;
- rewrite `asyncio.TimeoutError` → `TimeoutError` (UP041);
- flag `datetime.timezone.utc` → `datetime.UTC` (UP017);
- **don't** suggest PEP 695 generics or the `type` statement, which need `py312`.

Never set the target above the oldest interpreter you deploy on. With `py312`, the auto-fixes emit 3.12-only syntax that breaks 3.11 at import time.

mypy and pyright both support `Self`, `NotRequired`/`Required`, `TypeVarTuple` and `assert_never` when configured with `python_version = "3.11"`. Only pyright enforces `LiteralString`; mypy treats it as `str`.

***

#### Removed and Changed in 3.11 — Migration Notes

##### Removed in 3.11

| Removed | Replacement |
|---|---|
| `@asyncio.coroutine`, `asyncio.coroutines.CoroWrapper` | `async def` |
| `binhex` module; `binascii.a2b_hqx()`/`b2a_hqx()`/`rlecode_hqx()`/`rldecode_hqx()` | none (`binascii.crc_hqx()` remains) |
| `inspect.getargspec()`, `inspect.formatargspec()` | `inspect.signature()` |
| `inspect.Signature.from_builtin()`/`from_function()` | `Signature.from_callable()` |
| `gettext.lgettext()`, `ldgettext()`, `lngettext()`, `ldngettext()`, `bind_textdomain_codeset()`, `codeset=` parameters | `gettext()`/`dgettext()`/`ngettext()` |
| `'U'` mode in `open()`, `io.open()`, `codecs.open()`, `fileinput` | drop the flag (universal newlines are the default) |
| `random.sample()` on a `set` (now `TypeError`) | `random.sample(sorted(s), k)` |
| `random.shuffle(x, random)` second argument | `random.Random(seed).shuffle(x)` |
| `reuse_address` in `loop.create_datagram_endpoint()` | `reuse_port` where appropriate |
| `__getitem__` of `xml.dom.pulldom.DOMEventStream`, `wsgiref.util.FileWrapper`, `fileinput.FileInput` | iterate them |
| `pathlib.PurePath.__class_getitem__` (`PurePath[str]`) | plain `PurePath` |
| `distutils` `bdist_msi` command | wheels |
| `unittest` namespace-package discovery | regular packages |
| Bytes paths on `sys.path` | `str` paths |

```python
import random

try:
    random.sample({1, 2, 3}, 2)                # ❌ TypeError in 3.11
except TypeError as e:
    print(e)    # Population must be a sequence.  For dicts or sets, use sorted(d).
print(len(random.sample(sorted({1, 2, 3}), 2)))   # 2 ✅
```

##### Behavior Changes to Know

- **`enum`:** `str()` of `IntEnum`/`IntFlag` members returns the value, and `format()`/f-strings of `(str, Enum)`-style mixins return `Class.MEMBER`. See the enum section. This is the most common silent break.
- **`dataclasses`:** any unhashable default is rejected, not just `list`/`dict`/`set`.
- **`with`/`async with`** on a non-context-manager raises `TypeError` (was `AttributeError`).
- **`asyncio.TimeoutError`** and **`concurrent.futures.TimeoutError`** are now aliases of the builtin `TimeoutError`. Separate `except` clauses for them are redundant, and their order no longer matters.
- **`typing.get_type_hints()`** no longer wraps `None`-defaulted parameters in `Optional`.
- **`re`:** global inline flags must come first. Some capture-group results changed.
- **`math.pow(0.0, -inf)`/`math.pow(-0.0, -inf)`** return `inf` (used to raise `ValueError`).
- **`int()`/`str()`** of numbers with more than 4300 decimal digits raise `ValueError`.
- **`gzip.compress(data, mtime=0)`** may set a different OS byte in the header. Don't compare compressed bytes across versions.
- **`argparse`:** `const` now defaults to `None` for every action, so `action='store_const'`/`'append_const'` no longer require it. Calling `add_argument_group()` on an argument group is deprecated.
- **`calendar.LocaleTextCalendar`/`LocaleHTMLCalendar`** use `locale.getlocale()` instead of `getdefaultlocale()`.
- **`pdb`** reads `.pdbrc` as UTF-8.
- **`ast`:** node positions are validated by `compile()` (`ValueError` on invalid positions).
- **`sqlite3`:** fetching across a rollback no longer raises `InterfaceError`. `sqlite3.threadsafety` reflects the actual SQLite build. New `blobopen()`, `serialize()`/`deserialize()`, `create_window_function()`, `setlimit()`/`getlimit()`.

##### Deprecated in 3.11 — Removal Timeline (verified against 3.12/3.13 interpreters)

| Deprecated (in 3.11 or earlier) | Removed in |
|---|---|
| PEP 594 "dead batteries": `aifc`, `audioop`, `cgi`, `cgitb`, `chunk`, `crypt`, `imghdr`, `mailcap`, `msilib`, `nis`, `nntplib`, `ossaudiodev`, `pipes`, `sndhdr`, `spwd`, `sunau`, `telnetlib`, `uu`, `xdrlib` (each emits `DeprecationWarning` on import) | **3.13** |
| `asynchat`, `asyncore`, `smtpd`, `imp`, `distutils` (deprecated earlier; warnings updated) | **3.12** |
| `lib2to3` and the `2to3` tool | 3.13 |
| `configparser.SafeConfigParser`, `ParsingError.filename`, `RawConfigParser.readfp()` | 3.12 |
| `configparser.LegacyInterpolation` | 3.13 |
| `unittest` aliases `assertEquals`, `failUnless`, `assertRegexpMatches`, … | 3.12 |
| `unittest.makeSuite()`, `findTestCases()`, `getTestCaseNames()` | 3.13 |
| `typing.io`, `typing.re` namespaces | 3.13 |
| `TypedDict` keyword-argument syntax | 3.13 (`TypeError`) |
| `re.template()`, `re.TEMPLATE`/`re.T` | 3.13 |
| `locale.resetlocale()` | 3.13 |
| `pathlib.Path.link_to()` (deprecated in 3.10) | 3.12 |
| Chaining `classmethod` over other descriptors | 3.13 |
| `int()` delegating to `__trunc__()` | 3.14 |
| `locale.getdefaultlocale()` | planned for 3.15, then **un-deprecated** (gh-130796) — still prefer `getlocale()`/`getencoding()` |
| Legacy `importlib.resources` functions | warning dropped in 3.12, formally un-deprecated in 3.13 — `files()` remains preferred |
| `typing.Text` | no removal planned |

```python
import warnings

with warnings.catch_warnings(record=True) as rec:
    warnings.simplefilter('always')
    import telnetlib  # noqa: F401  — PEP 594 module
print(rec[0].category.__name__, '3.13' in str(rec[0].message))   # DeprecationWarning True
```

**New in 3.11:** `warnings.catch_warnings()` accepts `simplefilter()` arguments directly, e.g. `with warnings.catch_warnings(action='ignore'):`. Ignore warnings around one specific import, never globally.

***

#### Production Checklist

##### Python 3.11-Specific Items

- [ ] Running the **latest 3.11.x** (3.11.17 as of October 2026) from a source that tracks security releases. python.org binary installers stop at 3.11.9 and miss the tarfile/email/zipfile fixes.
- [ ] Upgrade path to 3.12+ planned: 3.11 security support ends October 2027.
- [ ] No 3.12+ features in the code base: no PEP 695 `def f[T]`/`class C[T]`/`type X =`, no PEP 701 f-strings (same quotes or backslashes inside `{}`), no `typing.override`/`TypeIs`/`ReadOnly`, no `itertools.batched`, no `Path.walk`.
- [ ] Linters and type checkers configured with `target-version = "py311"` / `python_version = "3.11"`. Never a higher target.
- [ ] `except*` (or explicit group handling) used around every `asyncio.TaskGroup` and every API that raises `ExceptionGroup`. No plain `except SomeError:` that can never match.
- [ ] No `contextlib.suppress(X)` expected to catch `X` inside an `ExceptionGroup` (that's 3.12+).
- [ ] Custom `ExceptionGroup` subclasses override `derive()`.
- [ ] `add_note()` + bare `raise` used to add context without changing exception types. Error reporters include `__notes__`.
- [ ] `asyncio.TaskGroup` preferred over bare `create_task()`/`gather()` for all-or-nothing concurrency. Semaphores bound fan-out.
- [ ] `asyncio.timeout()` preferred over `wait_for()`. `TimeoutError` caught **outside** the `async with`. `CancelledError` always re-raised.
- [ ] `except TimeoutError:` used instead of `asyncio.TimeoutError` (unless the code must still support 3.10).
- [ ] Every `class X(str, Enum)` migrated to `StrEnum`, or every f-string/`format()` of it audited.
- [ ] Every `str(IntEnum member)` / `str(IntFlag member)` audited (now returns the value).
- [ ] Dataclass defaults that are instances of non-frozen classes replaced with `default_factory`.
- [ ] `tomllib` used for TOML reading, with files opened in `'rb'`. `tomli` only as a `< 3.11` fallback.
- [ ] `datetime.now(UTC)` used; no `utcnow()`/`utcfromtimestamp()`.
- [ ] `typing.Self` replaces bound-`TypeVar` self-typing. `LiteralString` annotates SQL/shell sinks, with pyright in CI if it is meant as an injection guard (mypy doesn't enforce it).
- [ ] `NotRequired`/`Required` used instead of split `total=False` base classes. No keyword-argument `TypedDict(...)`.
- [ ] `PYTHONSAFEPATH=1` (or `-P`) set for services and admin scripts. The int/str digit limit left enabled for untrusted input.
- [ ] `tarfile.extractall(..., filter='data')` / `shutil.unpack_archive(..., filter='data')` for every untrusted archive.
- [ ] Regexes on untrusted input reviewed for catastrophic backtracking. Atomic groups or possessive quantifiers used where appropriate. No mid-pattern global flags.
- [ ] No `@asyncio.coroutine`, `binhex`, `inspect.getargspec()`, `'U'` open mode or `random.sample(set)` left over.
- [ ] Nothing imports `distutils`, `imp`, `asyncore`, `asynchat` or `smtpd` (removed in 3.12), or any PEP 594 module (removed in 3.13).
- [ ] Test suite runs clean with `-W error::DeprecationWarning`.
- [ ] Bytecode-level tools (coverage, profilers, debuggers, `bytecode`-based libraries) are 3.11-aware versions.

##### Code Correctness

- [ ] `bytes` and `str` never mixed without explicit encode/decode.
- [ ] `encoding='utf-8'` passed to every text-mode `open()`/`read_text()`/`write_text()`.
- [ ] `newline=''` used for CSV files.
- [ ] No mutable default arguments. Dataclass mutable fields use `field(default_factory=...)`.
- [ ] `__hash__` defined only on immutable classes, consistent with `__eq__`.
- [ ] `match` statements over enums end with `case _: assert_never(x)`.
- [ ] No `reveal_type()` left in committed code.

##### Safety and Security

- [ ] SQL parameterized. No `shell=True` with user input. No `eval`/`exec`/`pickle` on untrusted data.
- [ ] `secrets` used for tokens. `hmac.compare_digest()` used for comparisons.
- [ ] `defusedxml` used for untrusted XML.
- [ ] Paths `resolve()`d and checked with `is_relative_to()`.
- [ ] `str.format` never applied to user-controlled templates. `string.Template` used instead.

##### Performance

- [ ] Profiled on 3.11 before optimizing. Stale micro-optimizations (local aliasing, avoiding `try`) removed where they hurt clarity.
- [ ] No `sys._getframe()`/`inspect.stack()` in hot paths.
- [ ] Generators for large pipelines, `set`/`frozenset` for membership, `''.join()` for string building.
- [ ] Blocking calls in async code moved to `asyncio.to_thread()`.

##### Observability

- [ ] `logging.getLogger(__name__)` used in every module. `%`-style arguments, not f-strings.
- [ ] Log formatters/exporters render exception groups and notes (use `traceback.format_exception()`, not `str(exc)`).
- [ ] `logger.exception()` used inside handlers. Bare `raise` preserves tracebacks.
- [ ] Custom exceptions inherit from `Exception`, not `BaseException`.
