### Python 3.12: The Complete Production Developer Guide

#### Overview

Python 3.12 was released on October 2, 2023.[^1] It received regular bugfix releases until **3.12.10 (April 8, 2025)**, the last one with binary installers. Since then it is in the **security-fixes-only phase**: source-only releases come out as needed until **October 2028** (five years after 3.12.0), per the release-schedule PEP.[^2] As of October 2026 the latest release is **3.12.15** (October 1, 2026); 3.12.13 (March 2026) and 3.12.14 (August 2026) came before it.[^2] So python.org ships no new Windows/macOS installers for 3.12 any more. Production deployments get patched 3.12 builds from distributions, container images or build tools such as `uv`/`pyenv`. Three newer feature releases exist: 3.13, 3.14 and 3.15 (3.15.0 came out on October 9, 2026).

The landmark changes of 3.12:[^1]

- **PEP 695 type parameter syntax** (`def first[T](xs: list[T]) -> T`, `class Box[T]: ...`) and the **`type` statement** for lazily evaluated type aliases.
- **PEP 701 f-strings in the grammar**: you can reuse the same quote, use backslashes, write comments and nest f-strings without limit inside replacement fields.
- **PEP 698 `typing.override`** and **PEP 692 `**kwargs: Unpack[TypedDict]`** for type checkers.
- **PEP 709 comprehension inlining** (faster list/dict/set comprehensions, a few visible behavior changes).
- **PEP 669 `sys.monitoring`**, low-overhead profiling/debugging hooks, and **Linux `perf` profiler support**.
- **PEP 688 buffer protocol in Python** (`__buffer__`, `collections.abc.Buffer`).
- **PEP 684 per-interpreter GIL** and **PEP 683 immortal objects**. These are interpreter/C-API changes; Python code cannot yet spawn a subinterpreter with its own GIL.
- Better "Did you mean …" error messages, `itertools.batched()`, `Path.walk()`, subclassable `pathlib.Path`, `sqlite3` `autocommit`, `asyncio` eager tasks and large `asyncio` speed-ups.
- **Removals:** `distutils`, `imp`, `asynchat`, `asyncore`, `smtpd`, the old `unittest` method aliases (`assertEquals`, …), `configparser.SafeConfigParser`, `ssl.wrap_socket()`, and more. Also, `venv` no longer pre-installs `setuptools`.

This guide targets Python 3.12 specifically. Features from earlier versions remain fully available and are used freely here:

- **3.6–3.9:** f-strings, dataclasses, the walrus operator `:=`, positional-only parameters, built-in generics like `list[int]` (PEP 585), `dict` `|` merge, `str.removeprefix()`, `zoneinfo`.
- **3.10:** structural pattern matching (`match`/`case`), `X | Y` union syntax, parenthesized context managers, `zip(strict=True)`, `itertools.pairwise()`, `dataclass(slots=True, kw_only=True)`, `ParamSpec`/`Concatenate`/`TypeAlias`/`TypeGuard`.
- **3.11:** exception groups and `except*`, `add_note()`, `asyncio.TaskGroup` and `asyncio.timeout()`, `tomllib`, `typing.Self`, `LiteralString`, `Never`, `Required`/`NotRequired`, `TypeVarTuple`, `enum.StrEnum`, `datetime.UTC`, `hashlib.file_digest()`, `contextlib.chdir()`, and the faster CPython.

Every section that covers a 3.12-specific change is marked **New in 3.12**.

**Do NOT use or recommend anything newer than 3.12.** A 3.12 interpreter does not have any of the following:

- **3.13:** type-parameter defaults (`class Box[T = int]`, `TypeVar(default=...)`, PEP 696, a `SyntaxError` on 3.12), `typing.ReadOnly` (PEP 705), `typing.TypeIs` (PEP 742), `warnings.deprecated()` (PEP 702), `copy.replace()`, `queue.Queue.shutdown()`, the `strict=` parameter of `itertools.batched()`, `Path.from_uri()`/`PurePath.full_match()`, `os.process_cpu_count()`, `base64.z85encode()`, `PythonFinalizationError`, `typing.get_protocol_members()`/`NoDefault`, the free-threaded (no-GIL) build (PEP 703), the experimental JIT, the new colored REPL, and PEP 667 `locals()` semantics.
- **3.14:** template strings `t"..."` (PEP 750), deferred annotation evaluation and `annotationlib` (PEP 649/749), `concurrent.interpreters`/`InterpreterPoolExecutor` (PEP 734), `compression.zstd` (PEP 784), `except A, B:` without parentheses (PEP 758), `uuid.uuid7()`, `map(strict=True)`, and the `pathlib.Path.copy()`/`move()`/`info` API.
- **3.15:** `lazy import` (PEP 810), the `frozendict` and `sentinel` built-ins (PEP 814/661), unpacking in comprehensions (PEP 798), UTF-8 as the default encoding (PEP 686), the `profiling` package with the Tachyon sampling profiler (PEP 799), `TypedDict` `closed=`/`extra_items=` (PEP 728), `typing.TypeForm` (PEP 747) and `@typing.disjoint_base` (PEP 800).

When a project declares `requires-python = ">=3.12"`, none of these may appear in its code. When it still supports 3.11 (`>=3.11`), the 3.12-only syntax (`type` statements, `def f[T]`, f-strings that reuse quotes) is a `SyntaxError` on the older interpreter and must not be used either.

Global truths about Python 3.12:

- **Everything is an object.** Integers, functions, classes, modules, type aliases and type variables are all objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing, static checking.** Annotations are not enforced at runtime. Type checkers (mypy, pyright) enforce them. `override`, `Unpack[TypedDict]` and variance are **type-checker features**. They do nothing at runtime.
- **Reference counting + cyclic GC.** CPython manages memory with reference counts plus a cyclic garbage collector. Since 3.12, some objects (`None`, `True`, small ints, …) are *immortal*, and their reference count never changes (PEP 683).[^3]
- **One GIL per process for Python code.** The per-interpreter GIL (PEP 684) is reachable only from the C API in 3.12. Threads in pure Python still serialize on bytecode execution. There is no free-threaded build of 3.12.
- **Dicts are ordered by insertion.** The language spec guarantees this since Python 3.7.
- **Invalid escape sequences are now loud.** `"\d"` emits a `SyntaxWarning` at compile time in 3.12 (it was a silent `DeprecationWarning` before). Use raw strings for regexes and Windows paths.[^1]
- **Deprecations in 3.12 are removals soon.** Many 3.12 `DeprecationWarning`s turn into hard errors in 3.13 (the PEP 594 "dead batteries" `cgi`, `telnetlib`, `crypt`, …, plus `lib2to3`) or in 3.14 (`ast.Num` & co., `asyncio` child watchers, the default tar extraction filter changes to `'data'`). Run your test suite with `-W error::DeprecationWarning`.

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

type UserId = int                     # 3.12 `type` statement: alias names are PascalCase

# Type parameters: short PascalCase names (T, K, V, T_co is no longer needed —
# variance is inferred for PEP 695 type parameters)
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

# Module-level "private": excluded from __all__
_module_cache: dict[str, object] = {}
```

In Python 3, all classes implicitly inherit from `object`. `class Foo:` and `class Foo(object):` are identical. Never write the latter.

##### Import Ordering

Use three blocks separated by blank lines: standard library, third-party, local. Alphabetize within each group. `ruff check --select I` (or `isort --profile=black`) automates this.

```python
# Standard library
import os
import sys
from collections.abc import Callable, Iterator, Mapping   # ABCs come from collections.abc
from pathlib import Path
from typing import Self, override                         # override is new in 3.12

# Third-party
import requests

# Local application
from myapp.models import User
from myapp.utils import format_date

# Never wildcard imports — they defeat static analysis and __all__ hygiene
# Bad:  from module import *
# Good: from module import specific_item
```

In 3.12 code you no longer import `List`, `Dict`, `Optional`, `Union`, `TypeVar` or `Generic` from `typing` for ordinary annotations. Use `list[int]`, `int | None` and PEP 695 type parameters instead (see below).

##### Docstrings

Write Google-style docstrings for every public module, class, and function. The first line is a one-sentence imperative summary.

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

    def __init__(self, db: 'DatabaseConnection', cache: 'Cache | None' = None) -> None:
        self.db = db
        self.cache = cache
```

***

#### New in 3.12: Type Parameter Syntax (PEP 695)

This is the headline language change of 3.12. Generic functions, classes and type aliases get a compact, scoped syntax. You no longer declare module-level `TypeVar`s, and variance is inferred instead of declared.[^4][^5]

```python
from collections.abc import Callable, Hashable, Iterable, Sequence

# ❌ Pre-3.12 style — still valid in 3.12, but verbose and leaks T into module scope
from typing import Generic, TypeVar

T_old = TypeVar('T_old')

class OldBox(Generic[T_old]):
    def __init__(self, item: T_old) -> None:
        self.item = item

# ✅ 3.12 style — the type parameter list follows the name
def first[T](items: Sequence[T]) -> T:
    return items[0]

class Box[T]:
    def __init__(self, item: T) -> None:
        self.item = item

    def get(self) -> T:
        return self.item

# Bounds and constraints
def dedupe[H: Hashable](items: Iterable[H]) -> list[H]:   # bound: H must be Hashable
    return list(dict.fromkeys(items))

def concat[S: (str, bytes)](a: S, b: S) -> S:            # constraints: exactly str or bytes
    return a + b

# ParamSpec (**P) and TypeVarTuple (*Ts)
def logged[**P, R](func: Callable[P, R]) -> Callable[P, R]:
    def wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
        print(f'calling {func.__name__}')
        return func(*args, **kwargs)
    return wrapper

def prepend[*Ts](head: int, tail: tuple[*Ts]) -> tuple[int, *Ts]:
    return (head, *tail)

print(first([3, 1, 2]))          # 3
print(Box(42).get())             # 42
print(dedupe([1, 2, 1, 3]))      # [1, 2, 3]
print(concat('ab', 'cd'))        # abcd
print(prepend(1, ('a', 2.0)))    # (1, 'a', 2.0)
```

##### Mapping: Old → New

| Pre-3.12 (`typing` module) | 3.12 syntax |
|---|---|
| `T = TypeVar('T')` + `def f(x: T) -> T` | `def f[T](x: T) -> T` |
| `class C(Generic[T])` | `class C[T]` |
| `T = TypeVar('T', bound=Hashable)` | `[T: Hashable]` |
| `T = TypeVar('T', str, bytes)` | `[T: (str, bytes)]` |
| `P = ParamSpec('P')` | `[**P]` |
| `Ts = TypeVarTuple('Ts')` | `[*Ts]` |
| `T_co = TypeVar('T_co', covariant=True)` | `[T]` (variance inferred) |
| `Alias: TypeAlias = list[int]` | `type Alias = list[int]` |
| `Pair = tuple[T, T]` (generic alias via TypeVar) | `type Pair[T] = tuple[T, T]` |
| `class P(Protocol[T])` | `class P[T](Protocol)` |

##### The `type` Statement and Lazy Evaluation

`type X = ...` creates a `typing.TypeAliasType`. Its value is **evaluated lazily**, only when `__value__` is accessed. So aliases can refer to names defined later, including themselves (recursive types), without string quotes:[^4][^6]

```python
type JSON = dict[str, JSON] | list[JSON] | str | int | float | bool | None   # recursive, no quotes
type Pair[T] = tuple[T, T]                                                  # generic alias
type Handler = Callable[[Request], Response]                               # forward refs: fine

from collections.abc import Callable

class Request: ...
class Response: ...

print(type(JSON).__name__)        # TypeAliasType
print(Pair[int])                  # Pair[int]
print(Pair.__type_params__)       # (T,)
print(Handler.__value__)          # collections.abc.Callable[[__main__.Request], __main__.Response]

# A typo in an alias does NOT fail at definition time — only when evaluated
type Broken = list[Undefined]
try:
    Broken.__value__
except NameError as e:
    print(e)                      # name 'Undefined' is not defined
```

**Pitfall: a `TypeAliasType` is not the aliased class.** At runtime the alias is a separate object, not `list[int]` or `int`. Code that used old-style aliases as runtime values breaks when you mechanically convert it to `type`:

```python
type IntList = list[int]
type UserId = int

# ❌ All of these worked with `IntList = list[int]` / `UserId = int`, and fail with `type`:
for attempt in (
    lambda: isinstance([], UserId),        # TypeError: isinstance() arg 2 must be a type, ...
    lambda: UserId(5),                     # TypeError: 'typing.TypeAliasType' object is not callable
    lambda: type('Sub', (IntList,), {}),   # TypeError: metaclass conflict ... (cannot subclass)
):
    try:
        attempt()
    except TypeError as e:
        print('TypeError:', e)

# ✅ Use the underlying value explicitly when you need runtime behavior
print(isinstance([], IntList.__value__.__origin__))   # True
print(UserId.__value__(5))                             # 5
```

Rules for reviewers:

- Use `type X = ...` for aliases used **only in annotations**.
- Keep a plain assignment (`UserId = int`, or `typing.NewType` for a distinct nominal type) when the name is also **called, subclassed or used with `isinstance()`**.
- Runtime-introspecting libraries (validation, serialization, DI frameworks) must explicitly support `TypeAliasType`. Check the library version before converting aliases that they consume.
- `typing.get_type_hints()` does **not** unwrap a `TypeAliasType`. It returns the alias object itself, so introspection code must handle `__value__`.

##### Scoping: Annotation Scopes

Type parameters live in a new **annotation scope**. They are visible inside the function or class (including nested methods), but **not** in the enclosing module, and they are not class attributes:[^6]

```python
class Box[T]:
    def get(self) -> T: ...          # T visible in methods

print(Box.__type_params__)           # (T,)
try:
    Box.T                            # ❌ not an attribute
except AttributeError as e:
    print(e)                         # type object 'Box' has no attribute 'T'
try:
    T                                # ❌ not defined at module scope
except NameError as e:
    print(e)                         # name 'T' is not defined

# Annotation scopes inside a class body CAN see class-level names
# (normal nested functions cannot)
class Outer:
    Inner = int
    type Alias = Inner               # resolves Outer.Inner lazily

print(Outer.Alias.__value__)         # <class 'int'>
```

Bounds and constraints are lazy too. `class C[T: NotYetDefined]` compiles, and the `NameError` only appears when something reads `T.__bound__`.

##### Variance Inference

Type variables declared with the new syntax have `__infer_variance__ = True`. The type checker works out co/contra/invariance from how the parameter is used, so the `T_co`/`T_contra` naming conventions are obsolete in 3.12 code:[^4]

```python
class ReadOnlyBox[T]:          # only returned → inferred covariant by the type checker
    def __init__(self, item: T) -> None:
        self._item = item
    def get(self) -> T:
        return self._item

T = ReadOnlyBox.__type_params__[0]
print(T.__covariant__, T.__contravariant__, T.__infer_variance__)   # False False True

# The legacy TypeVar constructor also accepts infer_variance=True since 3.12
from typing import TypeVar
U = TypeVar('U', infer_variance=True)
print(U.__infer_variance__)    # True
```

The runtime flags `__covariant__`/`__contravariant__` stay `False`. Variance is computed only by the type checker, and the runtime never enforces it.

##### Pitfalls and Compatibility

```python
from typing import Generic, TypeVar

U = TypeVar('U')

# ❌ Mixing old and new styles: a PEP 695 class already inherits Generic implicitly
try:
    class Bad[T](Generic[U]):       # TypeError: Cannot inherit from Generic[...] multiple times.
        pass
except TypeError as e:
    print(e)

# ✅ PEP 695 classes are ordinary Generic subclasses at runtime
class Stack[T]:
    def __init__(self) -> None:
        self._items: list[T] = []
    def push(self, item: T) -> None:
        self._items.append(item)
    def pop(self) -> T:
        return self._items.pop()

print(Stack.__mro__)               # (<class '__main__.Stack'>, <class 'typing.Generic'>, <class 'object'>)
print(Stack[int])                  # __main__.Stack[int]
```

- **Do not mix a module-level `TypeVar` into a PEP 695 signature.** `def f[T](x: T, y: U)` with a legacy `U` is rejected by type checkers. Pick one style per definition.
- **Type-parameter defaults (`[T = int]`) are 3.13 (PEP 696).** On 3.12 `def f[T = int](x: T)` is a `SyntaxError` ("expected '('"). Never suggest them for 3.12 code.
- **3.11 compatibility:** any `type` statement or `[T]` list makes the whole module a `SyntaxError` on 3.11. If `requires-python` still includes 3.11, keep `TypeVar`/`Generic`/`TypeAlias`.
- **`type` is a soft keyword.** `type = 'admin'` and `type(x)` still work, so existing code is not broken.
- **Tooling:** mypy enables PEP 695 syntax by default since **mypy 1.12** (it was experimental in 1.11).[^7] Ruff's `UP040` (aliases), `UP046` (generic classes) and `UP047` (generic functions) rewrite old-style code to PEP 695 when `target-version = "py312"`.[^8]
- **`typing.TypeAlias` is deprecated in documentation** in favor of the `type` statement (no runtime warning). Keep it only for code that must run on 3.11.[^9]

***

#### New in 3.12: `@typing.override` (PEP 698)

`@override` tells the type checker that a method is meant to override a base-class method. If the base method is renamed or removed, the checker reports an error instead of silently leaving a dead "override".[^10]

```python
from typing import override

class Base:
    def get_color(self) -> str:
        return 'blue'

class GoodChild(Base):
    @override                   # ✅ checker verifies Base.get_color exists
    def get_color(self) -> str:
        return 'yellow'

class BadChild(Base):
    @override                   # ❌ type checker error: does not override anything (typo)
    def get_colour(self) -> str:
        return 'red'

# Runtime effect: none, except a best-effort marker attribute
print(GoodChild.get_color.__override__)   # True
print(BadChild().get_colour())            # red — no runtime error
```

Reviewer guidance:

- In 3.12 code, put `@override` on every method that overrides a base class method in a hierarchy you own. Pyright's `reportImplicitOverride` and mypy's `--enable-error-code explicit-override` can make it mandatory.
- Decorator order (PEP 698) only matters for the runtime marker. Type checkers accept either order. Put `@override` **outside** (above) wrapper decorators (anything that returns a new function, e.g. a `functools.wraps`-based decorator or `@functools.lru_cache`), so that `__override__` lands on the outermost wrapper. Put it **inside** (directly above the `def`, below) descriptor decorators such as `@property`, `@classmethod` and `@staticmethod`:[^10]

```python
import functools
from collections.abc import Callable
from typing import override

def traced[**P, R](fn: Callable[P, R]) -> Callable[P, R]:      # a typical wrapper decorator
    @functools.wraps(fn)
    def wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
        return fn(*args, **kwargs)
    return wrapper

class Base:
    def total(self) -> int: return 0
    @property
    def name(self) -> str: return 'base'

class Child(Base):
    @override                      # outermost: marks the wrapper function
    @traced
    def total(self) -> int: return 42

    @property
    @override                      # innermost: marks the function before property wraps it
    def name(self) -> str: return 'child'

print(Child.total.__override__, Child.name.fget.__override__)   # True True
```

- `typing_extensions.override` exists for code that must also run on 3.11 or older.

***

#### New in 3.12: Precise `**kwargs` Typing with `Unpack[TypedDict]` (PEP 692)

Before 3.12, `**kwargs: int` meant that *every* keyword argument is an `int`. PEP 692 lets a `TypedDict` describe each keyword separately. `typing.Unpack` itself has existed since 3.11 (for `TypeVarTuple`). 3.12 standardizes its use on `**kwargs`.[^11]

```python
from typing import NotRequired, TypedDict, Unpack

class ConnectOptions(TypedDict):
    timeout: float
    retries: NotRequired[int]        # may be omitted by callers
    ssl: NotRequired[bool]

def connect(host: str, **kwargs: Unpack[ConnectOptions]) -> str:
    timeout = kwargs['timeout']
    retries = kwargs.get('retries', 3)
    return f'{host} timeout={timeout} retries={retries}'

print(connect('db', timeout=5.0))                 # ✅ db timeout=5.0 retries=3
print(connect('db', timeout=5.0, retries=1))      # ✅
# connect('db', retries=1)          # ❌ type checker: missing required 'timeout'
# connect('db', timeout='5')        # ❌ type checker: str is not float
# connect('db', timeout=1, colour=1)# ❌ type checker: unexpected keyword 'colour'
```

Rules:

- This is purely static. At runtime `kwargs` is a plain `dict` and no validation happens.
- A keyword name used by both a named parameter and the `TypedDict` is a type error (`def f(timeout, **kw: Unpack[Opts])` when `Opts` also has `timeout`).
- Use it for wrapper functions that forward options (`def get(url, **kw: Unpack[RequestOpts])`) instead of `**kwargs: Any`.

***

#### New in 3.12: f-strings in the Grammar (PEP 701)

f-strings are now parsed by the PEG parser like any other expression. The old restrictions are gone:[^12]

```python
songs = ['Take me back to Eden', 'Alkaline', 'Ascensionism']

# ✅ Reuse the same quote type inside the replacement field
print(f"Playlist: {", ".join(songs)}")

# ✅ Backslashes and \N{...} escapes inside expressions
print(f"{'\n'.join(songs)}")
print(f"{'\N{BLACK HEART SUIT}'.join(songs)}")

# ✅ Multi-line expressions with comments
total = f"Total: {sum([
    10,   # base
    5,    # tax
])}"
print(total)                                   # Total: 15

# ✅ Arbitrary nesting
print(f"{f"{f"{1 + 1}"}"}")                    # 2

# Nested format specs and the = debug specifier still work
width, value = 10, 3.14159
print(f"{value:>{width}.2f}|")                 # '      3.14|'
print(f"{value=:.3f}")                         # value=3.142
```

Reviewer guidance:

- **Readability still rules.** Reusing the same quotes is legal but harder to scan in editors and diff tools that are not 3.12-aware. Prefer alternating quotes when that is just as easy. Flag deeply nested f-strings.
- **Do not use these forms if the project still supports 3.11.** `f"{", ".join(x)}"` and backslashes in replacement fields are a `SyntaxError` on 3.11 and earlier, and that fails the **whole module** at import time.
- **Tooling impact:** `tokenize` now emits `FSTRING_START`/`FSTRING_MIDDLE`/`FSTRING_END` tokens instead of a single `STRING` token for f-strings. Custom tokenizer-based tools (linters, code generators) must be updated.[^1]
- Error messages inside f-strings now point at the exact location in the line.
- A `:` inside a replacement field still starts the format spec, and `!` still starts a conversion. Wrap lambdas and walrus expressions in parentheses: `f"{(lambda x: x * 2)(3)}"`, `f"{(n := 10)}"`.

***

#### New in 3.12: Comprehension Inlining (PEP 709)

List, dict and set comprehensions are now **inlined** into the enclosing function. They no longer create and call a hidden function object, which makes them up to ~2x faster. Generator expressions are **not** inlined.[^13]

Iteration variables stay isolated, as before:

```python
x = 'outer'
squares = [x * x for x in range(4)]
print(x)          # 'outer' — comprehension variable does not leak
print(squares)    # [0, 1, 4, 9]
```

Visible behavior changes that a reviewer may meet:

```python
def f():
    outer = 1
    # locals() inside a comprehension now sees the enclosing function's locals
    return [sorted(locals()) for _ in range(1)][0]

print(f())        # ['_', 'outer'] on 3.12 — on 3.11 it was ['.0', '_']

# Tracebacks no longer show a separate <listcomp> frame
def boom():
    return [1 / 0 for _ in range(1)]

import traceback
try:
    boom()
except ZeroDivisionError:
    names = [fr.name for fr in traceback.extract_tb(__import__('sys').exc_info()[2])]
    print(names)  # ['<module>', 'boom'] — 3.11 also listed '<listcomp>'
```

- Code that iterates **directly** over `locals()` inside a comprehension can raise `RuntimeError: dictionary changed size during iteration` under tracing (coverage, debuggers). Snapshot it first: `keys = list(locals()); [k for k in keys]`.
- Profilers and tracers no longer report comprehensions as separate function calls, so profiles look different after upgrading.
- `symtable` no longer creates child tables for comprehensions.
- **Class bodies are unchanged:** a comprehension in a class body still cannot see class-level names (`class K: y = 5; v = [y for _ in range(2)]` raises `NameError`).

***

#### New in 3.12: Improved Error Messages

The interpreter's suggestions keep improving. Recognize them in logs and bug reports:[^1]

```python
# NameError for an un-imported stdlib module:
#   NameError: name 'sys' is not defined. Did you forget to import 'sys'?

# NameError inside a method for an instance attribute:
#   NameError: name 'blech' is not defined. Did you mean: 'self.blech'?

# Swapped import syntax:
#   import a.y.z from b.y.z
#   SyntaxError: Did you mean to use 'from ... import ...' instead?

# ImportError suggestions for misspelled names (new in 3.12), shown in the traceback:
#   ImportError: cannot import name 'chainmap' from 'collections' (...). Did you mean: 'ChainMap'?
try:
    from collections import chainmap
except ImportError as e:
    print(e)   # cannot import name 'chainmap' from 'collections' (/.../collections/__init__.py)
               # — no suggestion in str(e)
```

All of these "Did you …" hints are added only when the traceback is **printed** (by the default exception hook or `traceback.format_exception()`). `str(exc)` holds just the base message, so tests and log parsers must not assert on the suggestion text.

***

#### New in 3.12: The Buffer Protocol from Python (PEP 688)

Pure-Python classes can now expose the buffer protocol by implementing `__buffer__(self, flags)` (and optionally `__release_buffer__(self, view)`). `collections.abc.Buffer` is the standard annotation for "anything `memoryview()` accepts".[^14]

```python
import inspect
from collections.abc import Buffer

class Pixels:
    def __init__(self, n: int) -> None:
        self._data = bytearray(n)

    def __buffer__(self, flags: int) -> memoryview:
        return memoryview(self._data)

    def __release_buffer__(self, view: memoryview) -> None:
        pass   # called when the consumer releases the view

def checksum(data: Buffer) -> int:          # ✅ accepts bytes, bytearray, memoryview, array, Pixels…
    with memoryview(data) as mv:
        return sum(mv.cast('B'))

p = Pixels(3)
with memoryview(p) as mv:
    mv[0] = 255

print(checksum(b'abc'), checksum(bytearray(b'abc')), checksum(p))   # 294 294 255
print(isinstance(p, Buffer), isinstance('text', Buffer))            # True False
print(repr(inspect.BufferFlags.WRITABLE))                           # <BufferFlags.WRITABLE: 1>
```

- **Replace `ByteString` with `Buffer`.** `collections.abc.ByteString` and `typing.ByteString` are deprecated. In 3.12, `isinstance()`/subclassing with them emits a `DeprecationWarning`. In annotations use `Buffer` (or `bytes | bytearray | memoryview`).[^1]
- Always release views (`with memoryview(x) as mv:` or `mv.release()`). A live export blocks resizing of the underlying `bytearray` (`BufferError`).

***

#### New in 3.12: `sys.monitoring` — Low-Impact Monitoring (PEP 669)

`sys.monitoring` is a new event API for debuggers, profilers and coverage tools. Unlike `sys.settrace()`/`sys.setprofile()`, a tool subscribes only to the events it needs, and a callback can return `sys.monitoring.DISABLE` to switch an event off for that code location. That gives near-zero overhead once the tool has seen a location.[^15]

```python
import sys

mon = sys.monitoring
TOOL = mon.PROFILER_ID                     # predefined IDs: DEBUGGER_ID=0, COVERAGE_ID=1, PROFILER_ID=2, OPTIMIZER_ID=5

calls: dict[str, int] = {}

def on_py_start(code, instruction_offset):
    calls[code.co_name] = calls.get(code.co_name, 0) + 1

def work(n: int) -> int:
    return sum(range(n))

mon.use_tool_id(TOOL, 'call-counter')      # raises ValueError if the ID is already taken
try:
    mon.register_callback(TOOL, mon.events.PY_START, on_py_start)
    mon.set_events(TOOL, mon.events.PY_START)
    for _ in range(3):
        work(10)
finally:
    mon.set_events(TOOL, mon.events.NO_EVENTS)
    mon.register_callback(TOOL, mon.events.PY_START, None)
    mon.free_tool_id(TOOL)

print(calls['work'])                       # 3
```

Guidance:

- Application code rarely needs `sys.monitoring` directly. Use it in in-house profilers, tracers and coverage tools. Always free the tool ID in `finally`.
- Coverage measurement through `sys.monitoring` is much cheaper than through `sys.settrace()`. coverage.py (7.4+) can use it on 3.12 (`COVERAGE_CORE=sysmon`), but check which coverage features are supported in that mode before you switch CI to it.
- `sys.settrace()`-based debuggers keep working. 3.12 also adds `threading.settrace_all_threads()` and `threading.setprofile_all_threads()` to install a trace/profile function in all running threads.[^1]

***

#### New in 3.12: Interpreter Internals That Affect Production

##### Per-Interpreter GIL (PEP 684) — C API Only

Subinterpreters can now be created with their own GIL, but **only through the C API** (`Py_NewInterpreterFromConfig()` with `PyInterpreterConfig_OWN_GIL`). 3.12 has no public Python API for this: `_xxsubinterpreters` is private and unsupported, and the `concurrent.interpreters` module is 3.14. Extension modules must opt in via multi-phase init to be loaded in such interpreters.[^1][^16]

**Reviewer rule:** in 3.12 application code, CPU parallelism still means `multiprocessing`/`ProcessPoolExecutor` (or native code that releases the GIL). Reject imports of `_xxsubinterpreters`/`_interpreters` in production code.

##### Immortal Objects (PEP 683)

`None`, `True`, `False`, small integers, and many interned strings and static types are **immortal**: their reference count is fixed and never changes, which avoids cache-line contention.[^3]

```python
import sys
print(sys.getrefcount(None))     # 4294967295 on 64-bit 3.12 (a fixed sentinel value)
print(sys.getrefcount(1))        # 4294967295 — small ints are immortal too
x = object()
print(sys.getrefcount(x))        # 2 — normal objects are unaffected
```

Tests and leak detectors that assert exact `sys.getrefcount()` values for singletons or small ints break on 3.12. Measure refcounts only on your own objects. The sentinel value itself is an implementation detail (3.14 reports a different number), so never hard-code it.

##### Linux `perf` Profiler Support

CPython 3.12 can emit trampolines so that the Linux `perf` profiler shows **Python function names** in native stack traces.[^17]

```bash
# Enable for a whole run
python -X perf my_service.py                # or: PYTHONPERFSUPPORT=1 python my_service.py
perf record -F 9999 -g -o perf.data python -X perf my_service.py
perf report -g -i perf.data
```

```python
import sys
if sys.platform == 'linux':
    sys.activate_stack_trampoline('perf')   # enable at runtime (Linux only)
    print(sys.is_stack_trampoline_active()) # True
    sys.deactivate_stack_trampoline()
```

Use it for whole-process CPU profiling in production-like environments. For the best results the docs recommend building CPython with frame pointers (`CFLAGS="-fno-omit-frame-pointer -mno-omit-leaf-frame-pointer"`), so `perf` can unwind without DWARF information. Measure the overhead in your workload before you leave the trampoline enabled permanently.

##### Recursion and the Garbage Collector

- `sys.setrecursionlimit()` now limits **Python** frames only. C-level recursion has separate stack-overflow protection, so deep recursion in pure Python still raises `RecursionError` (default limit 1000).[^1]
- The cyclic GC now runs only at the "eval breaker" checkpoints of the bytecode loop, not during arbitrary object allocations. Long-running C extensions get a chance to run GC via `PyErr_CheckSignals()`.[^1]

***

#### Primitive Types and Numerics

##### Integers

`int` is arbitrary precision: arithmetic never overflows. **Since 3.11 (and backported to 3.10.7, 3.9.14 and other security releases), converting between `int` and decimal `str` is limited to 4300 digits by default**, a guard against quadratic-time denial of service (CVE-2020-10735):[^18]

```python
import sys

one_million = 1_000_000          # underscores in numeric literals
print(sys.get_int_max_str_digits())     # 4300

try:
    int('1' * 5000)              # parsing untrusted huge numbers
except ValueError as e:
    print(e)                     # Exceeds the limit (4300 digits) for integer string conversion: value has 5000 digits; ...

big = 10 ** 5000                 # arithmetic is fine…
print(big.bit_length())          # 16610
print(hex(big)[:10])             # …and power-of-two bases (hex/bin/oct) are not limited
# str(big) raises ValueError — use sys.set_int_max_str_digits() only where you trust the input

print((5).is_integer())          # True — int.is_integer() is new in 3.12 (duck-typing with float)
print((255).bit_count())         # 8 (3.10+)
```

##### Floats — New in 3.12: More Accurate `sum()`

`sum()` of floats now uses **Neumaier compensated summation**. Results are more accurate and closer to commutative, but they can **differ in the last bits from 3.11**. Golden-file tests comparing exact float output may need updating.[^1]

```python
import math

print(sum([0.1] * 10))                  # 1.0 on 3.12 (0.9999999999999999 on 3.11)
print(sum([1e100, 1.0, -1e100]))        # 1.0 on 3.12 (0.0 on 3.11)
print(math.fsum([0.1] * 10))            # 1.0 — fsum remains the exactly-rounded option

# New in 3.12: math.sumprod — sum of products, computed with extended precision
print(math.sumprod([1, 2, 3], [4, 5, 6]))        # 32
prices, qty = [19.99, 5.25], [3, 4]
print(round(math.sumprod(prices, qty), 2))       # 80.97

# New in 3.12: math.nextafter(x, y, steps=n)
print(math.nextafter(1.0, 2.0, steps=2))         # 1.0000000000000004

# 3.11 additions still worth knowing
print(math.cbrt(27.0), math.exp2(10))            # 3.0000000000000004 1024.0

# Float comparison — always math.isclose
assert math.isclose(0.1 + 0.2, 0.3)
```

##### Booleans — `~bool` Deprecated

`bool` is a subclass of `int`. **New in 3.12:** bitwise inversion `~` on a `bool` emits `DeprecationWarning`. `~True` is `-2`, which is almost never what the author meant, and it becomes an error in 3.16.[^1]

```python
flag = True
# ❌ ~flag   → -2 (DeprecationWarning in 3.12)
print(not flag)            # ✅ False — logical negation
print(~int(flag))          # ✅ -2 — explicit, if you really want the int inversion
# numpy/pandas boolean arrays are unaffected: `~mask` on arrays is fine
```

**Falsy values:** `None`, `False`, zero of any numeric type (`0`, `0.0`, `0j`, `Decimal(0)`, `Fraction(0)`), empty sequences and collections (`''`, `b''`, `()`, `[]`, `{}`, `set()`, `range(0)`), and objects whose `__bool__` returns `False` or `__len__` returns `0`.

##### Other 3.12 Numeric and Built-in Changes

```python
from fractions import Fraction

# New in 3.12: Fraction supports float-style format specs
print(f'{Fraction(1, 3):.3f}')          # 0.333
print(f'{Fraction(22, 7):.2%}')         # 314.29%

# New in 3.12: slice objects are hashable (usable as dict keys / in sets)
cache = {slice(0, 10): 'first page'}
print(cache[slice(0, 10)])              # first page

# New in 3.12: builtins/extension functions accept any object for boolean parameters
print(sorted([3, 1, 2], reverse='yes'))   # [3, 2, 1] — 3.11 raised TypeError for a str here
print('a\nb'.splitlines(keepends=[0]))    # ['a\n', 'b'] — any truthy object counts
```

***

#### Strings

##### The Two Types

- **`str`**: a sequence of Unicode code points. All undecorated string literals are `str`.
- **`bytes`**: a sequence of raw 8-bit values, declared with the `b''` prefix. **Never mixed implicitly with `str`.**

**The Unicode sandwich rule:** decode `bytes` → `str` at input boundaries, work in `str` internally, and encode `str` → `bytes` at output boundaries.

Always pass `encoding='utf-8'` to `open()`, `Path.read_text()`, `subprocess.run(text=True, ...)` and similar. In 3.12 the default is still the **locale encoding** (for example `cp1252` on many Windows systems). UTF-8 becomes the default only in 3.15 (PEP 686). Run tests with `-X warn_default_encoding` (3.10+) to find the calls that omit it.

##### New in 3.12: Invalid Escape Sequences Are a `SyntaxWarning`

A backslash followed by a character that is not a valid escape (`\d`, `\s`, `\.`, `\p`, …) now emits a **`SyntaxWarning`** at compile time. It used to be a `DeprecationWarning` that was hidden by default. Users see these warnings (for example when a module is first compiled), and a future version will make them a `SyntaxError`. Octal escapes above `\377` get the same treatment.[^1]

```python
import re
import warnings

with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter('always')
    compile('pattern = "\\d+\\.\\d+"', '<demo>', 'exec')   # source text: "\d+\.\d+"
print(caught[0].category.__name__, caught[0].message)
# SyntaxWarning invalid escape sequence '\d'

# ❌ re.compile("\d+\.\d+")        — SyntaxWarning in 3.12
# ✅ raw strings for regexes and Windows paths
NUMBER = re.compile(r'\d+\.\d+')
WIN_PATH = r'C:\new\table'          # without r: \n and \t are real escapes — silent bug!
print(NUMBER.findall('pi=3.14, e=2.72'))   # ['3.14', '2.72']
```

The warning is raised **when the code is compiled**, so cached `.pyc` files hide it. Run `python -W error::SyntaxWarning -m compileall -f -q src/` in CI to catch every occurrence. Numeric literals directly followed by a keyword (`0x1for`, `1if x else 2`) also produce a `SyntaxWarning`.

##### String Methods (Quick Reference)

```python
s = 'Hello, World!'

s.casefold()                         # aggressive lowercase — case-insensitive comparisons
'  hi  '.strip()                     # 'hi'
'xxhello'.lstrip('x')                # 'hello' — strips a CHARACTER SET, not a prefix
'https://x.com'.removeprefix('https://')   # 'x.com' — exact prefix (3.9+)
'file.tar.gz'.removesuffix('.gz')          # 'file.tar'

s.find('World')                      # 7  (-1 if absent)
s.index('World')                     # 7  (ValueError if absent)
s.startswith(('Hello', 'Hi'))        # True — tuple of prefixes
s.split(', ')                        # ['Hello', 'World!']
s.partition(', ')                    # ('Hello', ', ', 'World!')
', '.join(['a', 'b', 'c'])           # 'a, b, c'
'42'.zfill(6)                        # '000042'
'123'.isdigit()                      # True
'café'.encode('utf-8')               # b'caf\xc3\xa9'
```

**Pitfall:** `url.lstrip('https://')` strips any of the characters `h t p s : /`. Use `removeprefix()`.

##### String Formatting in 3.12

```python
name, score = 'Alice', 98.765
print(f'Hello, {name}!')
print(f'{score:.2f}')                   # '98.77'
print(f'{1_000_000:_}')                 # '1_000_000'
print(f'{255:#x}')                      # '0xff'
x = 42
print(f'{x=}')                          # x=42  (debugging only)

# New in 3.12 (PEP 701) — see the dedicated section
items = ['a', 'b']
print(f"{"\n".join(items)}")           # backslashes and same-quote reuse inside {} are legal

# Logging: never f-strings — use lazy %-style (see Logging)
```

`str.format()` and `%`-formatting remain fully functional. **Never** build SQL, shell commands or HTML with any of them (see Security).

***

#### Type Hints in 3.12

##### The Modern Baseline (3.10–3.12)

```python
from collections.abc import Callable, Iterator, Mapping, Sequence
from typing import Any, Literal, Never, Self, assert_never

# Unions with | (3.10+), no Optional/Union imports
def find(users: Mapping[int, str], uid: int) -> str | None:
    return users.get(uid)

# Self for fluent/alternative-constructor methods (3.11+)
class Query:
    def __init__(self) -> None:
        self.filters: list[str] = []

    def where(self, clause: str) -> Self:
        self.filters.append(clause)
        return self

    @classmethod
    def all(cls) -> Self:
        return cls()

# Exhaustiveness checking with Never/assert_never (3.11+)
type Mode = Literal['r', 'w']

def open_mode(mode: Mode) -> int:
    match mode:
        case 'r':
            return 0
        case 'w':
            return 1
        case _:
            assert_never(mode)   # type checker errors if a Literal case is unhandled

print(find({1: 'a'}, 2), Query.all().where('x > 1').filters, open_mode('w'))
```

##### What Each `typing` Name Is For in 3.12

| Need | Use in 3.12 | Avoid / legacy |
|---|---|---|
| Optional value | `int \| None` | `Optional[int]` |
| Union | `int \| str` | `Union[int, str]` |
| Generic function/class | `def f[T]`, `class C[T]` | `TypeVar` + `Generic[T]` |
| Type alias | `type A = ...` | `A: TypeAlias = ...` |
| Collections | `list[int]`, `collections.abc.Sequence[int]` | `typing.List`, `typing.Sequence` |
| Byte-like input | `collections.abc.Buffer` | `typing.ByteString` (warns) |
| Method override | `@typing.override` | comments |
| `**kwargs` schema | `**kw: Unpack[TD]` | `**kw: Any` |
| Return own class | `Self` | `TypeVar('T', bound='C')` |
| Read-only TypedDict key | — (3.13 `ReadOnly`, not available) | — |
| Narrowing helper | `TypeGuard[...]` (3.10) | `TypeIs` (3.13, not available) |
| Mark deprecated API | docstring + `warnings.warn(..., DeprecationWarning, stacklevel=2)` | `warnings.deprecated` (3.13, not available) |

`typing.Hashable` and `typing.Sized` are deprecated aliases (use `collections.abc`). `typing.Text` is deprecated (use `str`).[^1][^9]

##### `TypedDict` in 3.12

```python
from typing import NotRequired, Required, TypedDict

class Movie(TypedDict):
    title: str
    year: NotRequired[int]                 # 3.11+

class Partial(TypedDict, total=False):
    id: Required[int]                      # required even though total=False
    note: str

class Page[T](TypedDict):                  # generic TypedDict — PEP 695 syntax works (3.12)
    items: list[T]
    total: int

m: Movie = {'title': 'Alien'}
p: Page[int] = {'items': [1, 2], 'total': 2}
print(m, p, Page.__orig_bases__)           # since 3.12 also set on call-based TypedDict('X', {...}) (gh-103699)
```

##### Protocols — 3.12 Runtime-Check Changes

`isinstance()` against a `@runtime_checkable` protocol now uses `inspect.getattr_static()`. Properties and `__getattr__` are **no longer executed** during the check, and the check is 2–20x faster for small protocols. Some objects may change classification:[^1]

```python
from typing import Protocol, runtime_checkable

@runtime_checkable
class HasName(Protocol):
    name: str

class LazyRecord:
    @property
    def name(self) -> str:
        raise RuntimeError('expensive lookup!')

class Dynamic:
    def __getattr__(self, attr: str) -> str:   # dynamic attributes
        return 'x'

print(isinstance(LazyRecord(), HasName))   # True — on 3.11 this RAISED RuntimeError
print(isinstance(Dynamic(), HasName))      # False — on 3.11 this was True (hasattr called __getattr__)
```

Protocol members are also **frozen at class creation**. Monkey-patching new attributes onto a protocol no longer changes `isinstance()` results. Runtime protocol checks only verify attribute *presence*, never signatures or types.

##### `dataclass_transform` and Generic Introspection

```python
import types
from typing import dataclass_transform

@dataclass_transform(frozen_default=True)     # frozen_default is new in 3.12
def model[T](cls: type[T]) -> type[T]:
    return cls

class Base[T]: ...
class Child(Base[int]): ...
print(types.get_original_bases(Child))        # (__main__.Base[int],) — new in 3.12
```

***

#### Lists, Tuples and Sequences

```python
lst: list[int] = [1, 2, 3, 4, 5]

print(lst[-1], lst[1:3], lst[::-1])   # 5 [2, 3] [5, 4, 3, 2, 1]
lst.append(6)
lst.extend([7, 8])
lst.remove(3)                         # first occurrence; ValueError if absent
popped = lst.pop()                    # last element

nums = [3, 1, 4, 1, 5, 9]
nums.sort()                           # in-place, stable
desc = sorted(nums, reverse=True)     # new list

squares = [x**2 for x in range(10)]   # inlined in 3.12 (PEP 709) — faster
flat = [item for sub in [[1, 2], [3, 4]] for item in sub]   # [1, 2, 3, 4]

# Extended unpacking
head, *middle, tail = [1, 2, 3, 4, 5]
print(head, middle, tail)             # 1 [2, 3, 4] 5
```

**Pitfall: list multiplication shares references:**

```python
matrix = [[0] * 3] * 3            # ❌ three references to ONE row
matrix[0][0] = 99
print(matrix)                     # [[99, 0, 0], [99, 0, 0], [99, 0, 0]]

matrix = [[0] * 3 for _ in range(3)]   # ✅ independent rows
matrix[0][0] = 99
print(matrix)                     # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

##### New in 3.12: `itertools.batched()`

`batched(iterable, n)` yields **tuples** of length `n`. The last tuple may be shorter. It replaces the hand-rolled "chunks" helper found in nearly every codebase:[^19]

```python
from itertools import batched

print(list(batched('ABCDEFG', 3)))     # [('A', 'B', 'C'), ('D', 'E', 'F'), ('G',)]

def insert_rows(rows: list[dict[str, object]]) -> int:
    sent = 0
    for chunk in batched(rows, 500):   # ✅ bulk-insert in batches of 500
        sent += len(chunk)              # db.executemany(SQL, chunk)
    return sent

print(insert_rows([{'id': i} for i in range(1234)]))   # 1234

try:
    batched([1, 2], 0)
except ValueError as e:
    print(e)                            # n must be at least one
```

- `batched(..., strict=True)` (raise if the last batch is short) is **3.13**. In 3.12, check `len(chunk) == n` yourself if you need it.
- Batches are tuples, not lists. Call `list(chunk)` only if you need to mutate.

##### Named Tuples and `array`

```python
from array import array
from typing import NamedTuple

class Point(NamedTuple):
    x: float
    y: float
    label: str = ''

p = Point(1.0, 2.0, 'origin')
print(p._asdict())                  # {'x': 1.0, 'y': 2.0, 'label': 'origin'}

samples: array[float] = array('d', [1.0, 2.5])   # array is subscriptable in annotations (new in 3.12)
print(array[int])                                 # array.array[int]
```

***

#### Dictionaries and Sets

```python
d: dict[str, int] = {'a': 1, 'b': 2}
print(d.get('c', 0))                    # 0
print(d.setdefault('d', 99))            # 99

defaults = {'host': 'localhost', 'port': 5432}
overrides = {'port': 9432}
config = defaults | overrides           # merge (3.9+), right side wins
config |= {'debug': True}               # in-place update

inverted = {v: k for k, v in d.items()}
for k in reversed(d):                   # reverse insertion order (3.8+)
    pass

# Never mutate a dict while iterating it
d = {k: v for k, v in d.items() if v != 99}   # ✅ rebuild

s = {1, 2, 3}
print(s | {4}, s & {2, 3}, s - {1}, s ^ {3, 4})
empty = set()                           # NOT {} — that's an empty dict
```

```python
from collections import ChainMap, Counter, defaultdict, deque

word_count: defaultdict[str, int] = defaultdict(int)
for w in 'a b a'.split():
    word_count[w] += 1

c = Counter('abracadabra')
print(c.most_common(2))                 # [('a', 5), ('b', 2)]
print(Counter(a=3) | Counter(a=1))      # Counter({'a': 3}) — multiset MAX, not a merge

env = ChainMap({'color': 'red'}, {'color': 'blue', 'user': 'guest'})
print(env['color'], env['user'])        # red guest

q: deque[str] = deque(maxlen=1000)      # O(1) appendleft/popleft
q.append('item')
```

***

#### Control Flow

```python
# for/else — else runs only when the loop did not break
def find_admin(users: list[dict[str, str]]) -> str:
    for user in users:
        if user.get('role') == 'admin':
            result = user['name']
            break
    else:
        result = 'nobody'
    return result

# Walrus (3.8+)
import io
stream = io.BytesIO(b'x' * 100_000)
while chunk := stream.read(65_536):
    pass
```

##### Structural Pattern Matching (3.10+)

`match` is fully available and idiomatic in 3.12 for destructuring structured data:

```python
from dataclasses import dataclass

@dataclass
class Point:
    x: int
    y: int

def describe(event: object) -> str:
    match event:
        case {'type': 'click', 'pos': (x, y)}:        # mapping + sequence pattern
            return f'click at {x},{y}'
        case Point(x=0, y=0):                         # class pattern with keywords
            return 'origin'
        case Point(x=x, y=y) if x == y:               # guard
            return f'diagonal {x}'
        case [first, *rest]:                          # sequence pattern (not str!)
            return f'list starting {first!r} (+{len(rest)})'
        case str() | bytes():
            return 'text'
        case _:
            return 'unknown'

print(describe({'type': 'click', 'pos': (1, 2)}))   # click at 1,2
print(describe(Point(0, 0)), describe(Point(3, 3)), describe([1, 2, 3]), describe('abc'))
```

Pitfalls:

- `case NAME:` with a bare name is a **capture pattern** that matches anything. To compare against a constant, use a dotted name (`case Color.RED:`, `case http.HTTPStatus.OK:`) or a literal.
- `str`/`bytes` never match sequence patterns. That is deliberate.
- Mapping patterns match if the listed keys are present; extra keys are ignored.

***

#### Functions

```python
def greet(name: str, greeting: str = 'Hello', /, *, punctuation: str = '!') -> str:
    """Return a greeting string (name/greeting positional-only, punctuation keyword-only)."""
    return f'{greeting}, {name}{punctuation}'

print(greet('Alice'), greet('Bob', 'Hi', punctuation='.'))

# ❌ Mutable default argument — shared across calls
def append_bad(item: int, bucket: list[int] = []) -> list[int]:
    bucket.append(item)
    return bucket

print(append_bad(1), append_bad(2))   # [1, 2] [1, 2]  — same list!

# ✅ None sentinel
def append_good(item: int, bucket: list[int] | None = None) -> list[int]:
    bucket = [] if bucket is None else bucket
    bucket.append(item)
    return bucket

# Closures: late binding
funcs = [lambda: i for i in range(3)]
print([f() for f in funcs])           # [2, 2, 2]
funcs = [lambda i=i: i for i in range(3)]
print([f() for f in funcs])           # [0, 1, 2]
```

##### Generic Functions with PEP 695

```python
from collections.abc import Callable, Iterable

def group_by[K, V](items: Iterable[V], key: Callable[[V], K]) -> dict[K, list[V]]:
    out: dict[K, list[V]] = {}
    for item in items:
        out.setdefault(key(item), []).append(item)
    return out

print(group_by(['apple', 'avocado', 'banana'], key=lambda s: s[0]))
# {'a': ['apple', 'avocado'], 'b': ['banana']}
```

##### `functools` — `cached_property` Lost Its Lock in 3.12

```python
import functools

@functools.cache                       # unbounded memoization (3.9+)
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)

@functools.lru_cache(maxsize=1024)     # bounded — use for unbounded argument spaces
def lookup(key: str) -> str:
    return key.upper()

class Dataset:
    def __init__(self, data: list[float]) -> None:
        self.data = data

    @functools.cached_property
    def mean(self) -> float:
        return sum(self.data) / len(self.data)

print(fib(50), lookup('x'), Dataset([1.0, 2.0, 3.0]).mean)   # 12586269025 X 2.0
```

**Changed in 3.12:** `functools.cached_property` no longer holds a lock. Before 3.12 it held a lock that was shared by **all instances** of a class, which serialized unrelated objects. Now, when two threads race on the same instance, **the getter can run more than once**.[^1] Idempotent getters are fine. If the getter has side effects (opens connections, increments counters, charges an API), add your own per-instance lock:

```python
import functools
import threading

class Client:
    def __init__(self) -> None:
        self._lock = threading.Lock()

    @functools.cached_property
    def session(self) -> dict[str, str]:
        with self._lock:                               # ✅ explicit synchronization
            if 'session' in self.__dict__:             # another thread already computed it
                return self.__dict__['session']
            return {'token': 'expensive-handshake'}

print(Client().session)
```

`cached_property` still requires an instance `__dict__`. It fails with `TypeError` on classes with `__slots__` and no `__dict__`, which includes `@dataclass(slots=True)`.

##### Generators and `itertools`

```python
from collections.abc import Iterator
from itertools import accumulate, batched, chain, islice, pairwise

def fibonacci() -> Iterator[int]:
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

print(list(islice(fibonacci(), 8)))               # [0, 1, 1, 2, 3, 5, 8, 13]
print(list(pairwise([1, 2, 4, 7])))                # [(1, 2), (2, 4), (4, 7)] — 3.10+
print(list(accumulate([1, 2, 3], initial=0)))      # [0, 1, 3, 6]
print(list(chain.from_iterable(batched(range(5), 2))))   # [0, 1, 2, 3, 4]
total = sum(x * x for x in range(1_000))           # generator expression: O(1) memory
```

**Deprecated in 3.12:** the three-argument form `gen.throw(type, value, traceback)` (and the same for coroutines and async generators). Use `gen.throw(exc_instance)`. Also deprecated (removed in 3.14): `copy`/`deepcopy`/`pickle` support for `itertools` objects.[^1]

***

#### Dataclasses

```python
from dataclasses import KW_ONLY, dataclass, field
from typing import ClassVar

@dataclass(slots=True, kw_only=True)        # slots/kw_only: 3.10+
class User:
    id: int
    name: str
    email: str
    active: bool = True
    tags: list[str] = field(default_factory=list)
    registry: ClassVar[dict[int, 'User']] = {}

    def __post_init__(self) -> None:
        self.email = self.email.lower()

@dataclass(frozen=True, slots=True)
class Coordinate:
    lat: float
    lon: float

@dataclass
class Request:
    url: str
    _: KW_ONLY                              # everything after is keyword-only
    timeout: float = 10.0

@dataclass
class Box[T]:                               # PEP 695 generic dataclass
    item: T

u = User(id=1, name='Ann', email='ANN@EXAMPLE.COM')
print(u.email, Coordinate(1.0, 2.0), Request('http://x', timeout=3), Box(5))
```

**Critical pitfalls:**

```python
from dataclasses import dataclass, field

# ❌ Mutable default — ValueError at class creation (3.11+ rejects ANY unhashable default)
try:
    @dataclass
    class Bad:
        items: list[int] = []
except ValueError as e:
    print(e)        # mutable default <class 'list'> for field items is not allowed: use default_factory

# ❌ Zero-argument super() inside a slots=True dataclass fails on 3.12
@dataclass(slots=True)
class Base:
    x: int
    def describe(self) -> str:
        return super().__repr__()    # TypeError at call time on 3.12

try:
    Base(1).describe()
except TypeError as e:
    print(e)        # super(type, obj): obj must be an instance or subtype of type

# ✅ Two-argument super() works (the class name resolves to the final slotted class)
@dataclass(slots=True)
class Fixed:
    x: int
    def describe(self) -> str:
        return super(Fixed, self).__repr__()

print(Fixed(1).describe().startswith('<__main__.Fixed object'))   # True

@dataclass
class Good:
    items: list[int] = field(default_factory=list)   # ✅ mutable default via factory
```

`@dataclass(slots=True)` builds a **new class**. The `__class__` cell used by zero-argument `super()` still points to the original class, so the call fails. The 3.12 `dataclasses` documentation lists this limitation and the two-argument workaround (gh-90562).[^31] The 3.13.16 and 3.14 interpreters no longer fail, but every 3.12.x does. Reviewers should flag zero-argument `super()` in methods of `slots=True` dataclasses on 3.12.

***

#### Classes and OOP

##### Class Definition

```python
import math
from typing import Self, override

class Shape:
    count: int = 0                     # class attribute — shared

    def __init__(self, color: str = 'black') -> None:
        self.color = color
        Shape.count += 1

    def area(self) -> float:
        raise NotImplementedError

    def __repr__(self) -> str:
        return f'{type(self).__name__}(color={self.color!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, type(self)):
            return NotImplemented
        return vars(self) == vars(other)
    # Defining __eq__ sets __hash__ = None — correct for a mutable class

class Circle(Shape):
    def __init__(self, radius: float, **kwargs: str) -> None:
        super().__init__(**kwargs)
        self.radius = radius

    @override                          # ✅ 3.12: declare intent to override
    def area(self) -> float:
        return math.pi * self.radius ** 2

    @classmethod
    def unit(cls) -> Self:             # Self (3.11+) instead of a bound TypeVar
        return cls(1.0)

print(Circle.unit().area(), Circle(2.0) == Circle(2.0))
```

##### Abstract Base Classes and Protocols

```python
from abc import ABC, abstractmethod
from typing import Protocol, override

class Repository[T](ABC):
    @abstractmethod
    def get(self, id: int) -> T | None: ...

    @abstractmethod
    def save(self, entity: T) -> int: ...

    def exists(self, id: int) -> bool:
        return self.get(id) is not None

class MemoryRepo(Repository[dict[str, object]]):
    def __init__(self) -> None:
        self._rows: dict[int, dict[str, object]] = {}

    @override
    def get(self, id: int) -> dict[str, object] | None:
        return self._rows.get(id)

    @override
    def save(self, entity: dict[str, object]) -> int:
        self._rows[len(self._rows)] = entity
        return len(self._rows) - 1

class SupportsClose(Protocol):          # structural typing — no inheritance needed
    def close(self) -> None: ...

repo = MemoryRepo()
print(repo.save({'a': 1}), repo.exists(0))   # 0 True
```

##### `__set_name__` Errors Are No Longer Wrapped (3.12)

Exceptions raised in a descriptor's `__set_name__` now propagate **unchanged**, with a PEP 678 note attached. Before 3.12 they were wrapped in `RuntimeError`. `except RuntimeError` around class creation no longer catches them.[^1]

```python
class Field:
    def __set_name__(self, owner: type, name: str) -> None:
        if name.startswith('_'):
            raise ValueError(f'private field {name!r} not allowed')

try:
    class Model:
        _secret = Field()
except ValueError as e:          # 3.11 raised RuntimeError here
    print(e, e.__notes__)
    # private field '_secret' not allowed ["Error calling __set_name__ on 'Field' instance '_secret' in 'Model'"]
```

##### Enums

```python
from enum import Enum, Flag, IntEnum, StrEnum, auto, verify, UNIQUE

class Color(Enum):
    RED = 1
    GREEN = 2

class HttpStatus(IntEnum):
    OK = 200
    NOT_FOUND = 404

@verify(UNIQUE)                 # 3.11+: reject aliases at class creation
class Env(StrEnum):             # 3.11+: members ARE str
    PROD = auto()               # 'prod'
    DEV = auto()

class Perm(Flag):
    READ = auto()
    WRITE = auto()

print(Env.PROD == 'prod', f'{Env.DEV}')        # True dev
print(Perm.READ in (Perm.READ | Perm.WRITE))   # True

# Changed in 3.12: `in` accepts non-member values instead of raising TypeError
print(1 in Color, 3 in Color, 'x' in Color)    # True False False  (3.11: TypeError for non-members)
```

- **3.12 behavior change:** `value in SomeEnum` returns `True` when `value` is a member **or the value of a member**. On 3.11 non-member operands raised `TypeError` (with a `DeprecationWarning` announcing this change). Code that relied on the `TypeError` must be updated.[^20]
- `str()`/`format()` of `IntEnum`/`IntFlag` members return the plain value since 3.11 (`str(HttpStatus.OK) == '200'`). Use `.name` for the name.
- `EnumMeta.__getattr__` was removed in 3.12. Subclassing `EnumType` and relying on it is broken.

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
        err.add_note(f'raw value was {raw!r}')     # 3.11+: attach context
        raise err from e                             # explicit chaining

try:
    parse_port('N/A')
except ConfigError as e:
    print(e, e.__notes__, type(e.__cause__).__name__)
    # Config error for 'port': must be an integer ["raw value was 'N/A'"] ValueError
```

##### Exception Groups and `except*` (3.11+)

```python
def run_all() -> None:
    errors: list[Exception] = []
    for job in ('a', 'b', 'c'):
        try:
            if job != 'b':
                raise ValueError(f'job {job} failed')
        except ValueError as e:
            errors.append(e)
    if errors:
        raise ExceptionGroup('some jobs failed', errors)

try:
    run_all()
except* ValueError as eg:
    print([str(e) for e in eg.exceptions])     # ['job a failed', 'job c failed']
```

**Changed in 3.12 (also in 3.11.4):** when an `except*` clause handles the whole group and raises **one** new exception, that exception is no longer wrapped in an `ExceptionGroup`:[^1]

```python
try:
    try:
        raise ExceptionGroup('g', [ValueError(1)])
    except* ValueError:
        raise KeyError('translated')
except KeyError as e:                 # ✅ caught directly in 3.12
    print('KeyError', e)
```

Also new in 3.12: `contextlib.suppress()` suppresses matching exceptions **inside** an exception group and re-raises the rest:

```python
from contextlib import suppress

with suppress(ValueError):
    raise ExceptionGroup('g', [ValueError(1)])          # fully suppressed
try:
    with suppress(ValueError):
        raise ExceptionGroup('g', [ValueError(1), KeyError(2)])
except ExceptionGroup as eg:
    print(repr(eg))                                     # ExceptionGroup('g', [KeyError(2)])
```

##### Other 3.12 Exception-Related Changes

- `sys.last_exc` holds the last unhandled exception (for post-mortem tools). `sys.last_type`, `sys.last_value` and `sys.last_traceback` are deprecated.[^1]
- `sys._current_exceptions()` maps thread IDs to exception **instances** instead of `(type, value, tb)` tuples.
- Re-raise with a bare `raise` (preserves the traceback). Use `logger.exception()` inside `except` to log with the traceback.
- Use `sys.exception()` (3.11+) instead of `sys.exc_info()[1]`.

***

#### Context Managers

```python
import logging
import os
import time
from contextlib import ExitStack, chdir, contextmanager, suppress
from pathlib import Path
import tempfile

logger = logging.getLogger(__name__)

work = Path(tempfile.mkdtemp())
(work / 'input.txt').write_text('a\nb\n', encoding='utf-8')

# Parenthesized multi-item with (3.10+ — official syntax)
with (
    open(work / 'input.txt', encoding='utf-8') as fin,
    open(work / 'output.txt', 'w', encoding='utf-8') as fout,
):
    for line in fin:
        fout.write(line.upper())

@contextmanager
def timed(name: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        logger.info('%s took %.3fs', name, time.perf_counter() - start)

with timed('job'), suppress(FileNotFoundError):
    os.remove(work / 'missing.tmp')

with chdir(work):                      # 3.11+: temporarily change cwd (not thread-safe!)
    print(sorted(os.listdir('.')))     # ['input.txt', 'output.txt']

with ExitStack() as stack:             # dynamic number of resources
    files = [stack.enter_context(open(work / n, encoding='utf-8')) for n in ('input.txt', 'output.txt')]
    print(len(files))                  # 2
```

`contextlib.chdir()` changes process-global state. Never use it in threaded or async servers.

***

#### Dates and Times

##### New in 3.12: `datetime.utcnow()` and `utcfromtimestamp()` Are Deprecated

Both return **naive** datetimes that silently represent UTC, a long-standing source of bugs (naive values get interpreted as local time by `.timestamp()` and `.astimezone()`). In 3.12 they emit `DeprecationWarning`. Removal is planned for a future version without a fixed date.[^21]

```python
from datetime import UTC, datetime, timezone

# ❌ datetime.utcnow()                  → naive, DeprecationWarning in 3.12
# ❌ datetime.utcfromtimestamp(ts)      → naive, DeprecationWarning in 3.12
now = datetime.now(UTC)                 # ✅ aware; datetime.UTC is an alias of timezone.utc (3.11+)
epoch = datetime.fromtimestamp(0, tz=UTC)
print(now.tzinfo, epoch)                # UTC 1970-01-01 00:00:00+00:00

# The classic bug utcnow() causes:
naive_utc = datetime(2024, 6, 15, 12, 0)           # what utcnow() returns: no tzinfo
aware_utc = naive_utc.replace(tzinfo=timezone.utc)
print(aware_utc.isoformat())                        # 2024-06-15T12:00:00+00:00
# naive_utc.timestamp() would treat 12:00 as LOCAL time — wrong unless the host runs in UTC

# fromisoformat accepts most ISO 8601 forms since 3.11, including a trailing Z
print(datetime.fromisoformat('2024-06-15T12:00:00Z'))   # 2024-06-15 12:00:00+00:00
```

Migration: `datetime.utcnow()` → `datetime.now(UTC)`. Then check every consumer: ordering comparisons (`<`, `>`, sorting) and subtraction between naive and aware datetimes raise `TypeError`, `==` silently returns `False`, and database drivers may treat aware values differently. If a legacy interface demands naive UTC, write `datetime.now(UTC).replace(tzinfo=None)` explicitly.

##### `zoneinfo`

```python
from datetime import UTC, datetime
from zoneinfo import ZoneInfo

utc = datetime(2024, 3, 10, 7, 30, tzinfo=UTC)
print(utc.astimezone(ZoneInfo('America/New_York')))    # 2024-03-10 03:30:00-04:00
```

Store and compute in UTC, convert for display. Declare `tzdata` as a dependency for Windows and minimal containers.

##### New in 3.12: `calendar.Month` and `calendar.Day` Enums

```python
import calendar

print(calendar.Month.FEBRUARY.value, calendar.Month(2).name)   # 2 FEBRUARY
print(calendar.Day.MONDAY == calendar.MONDAY == 0)              # True
print(repr(calendar.JANUARY), calendar.JANUARY == 1)            # calendar.JANUARY True (an IntEnum member)
# ❌ calendar.January / calendar.February — DeprecationWarning in 3.12; use JANUARY/FEBRUARY
```

***

#### File I/O and `pathlib`

Always pass `encoding='utf-8'` explicitly for text I/O.

```python
import tempfile
from pathlib import Path

base = Path(tempfile.mkdtemp())
p = base / 'reports' / 'output.csv'

print(p.name, p.stem, p.suffix)          # output.csv output .csv
p.parent.mkdir(parents=True, exist_ok=True)
p.write_text('a,b\n1,2\n', encoding='utf-8')
print(p.read_text(encoding='utf-8').splitlines())   # ['a,b', '1,2']
print(p.with_stem('summary').name)       # summary.csv (3.9+)
p.unlink(missing_ok=True)                # 3.8+
```

##### New in 3.12: `Path.walk()`

`Path.walk()` is the `pathlib` counterpart of `os.walk()`. It yields `(dirpath: Path, dirnames: list[str], filenames: list[str])`:[^22]

```python
import tempfile
from pathlib import Path

root = Path(tempfile.mkdtemp())
(root / 'src' / 'pkg').mkdir(parents=True)
(root / 'src' / 'pkg' / 'mod.py').write_text('x = 1\n', encoding='utf-8')
(root / '.git').mkdir()
(root / 'link').symlink_to(root / 'src')          # symlink to a directory

for dirpath, dirnames, filenames in root.walk():  # top_down=True by default
    dirnames[:] = [d for d in dirnames if not d.startswith('.')]   # ✅ prune in place
    for name in filenames:
        print((dirpath / name).relative_to(root))
# link            ← the symlink to a directory is reported as a FILE (see below)
# src/pkg/mod.py

errors: list[OSError] = []
list(Path('/definitely/missing').walk(on_error=errors.append))
print(type(errors[0]).__name__)                   # FileNotFoundError
```

Differences from `os.walk()` that matter in review:

- `dirpath` is a `Path`, and the error callback parameter is `on_error` (not `onerror`).
- `follow_symlinks=False` is the default. **Unlike `os.walk()`, symlinks to directories are listed in `filenames`, not `dirnames`**, when symlinks are not followed. Code ported from `os.walk()` that filters `dirnames` for symlinks needs adjusting.
- Errors are **ignored silently** unless you pass `on_error`. Production code should always pass one.
- With `top_down=False`, modifying `dirnames` has no effect.

##### New in 3.12: Subclassing `pathlib.Path`

`Path`, `PurePath` and their Posix/Windows variants can now be subclassed directly (in 3.11 `class MyPath(Path)` failed with `AttributeError: ... '_flavour'`). To carry extra state through derived paths (`/`, `.parent`, `.with_name()`, `iterdir()`, …), override `with_segments()`:[^22]

```python
from pathlib import Path

class TenantPath(Path):
    def __init__(self, *segments: str | Path, tenant: str) -> None:
        super().__init__(*segments)
        self.tenant = tenant

    def with_segments(self, *segments: str | Path) -> 'TenantPath':
        return type(self)(*segments, tenant=self.tenant)   # ✅ propagate state

p = TenantPath('/srv/data', tenant='acme')
child = p / 'reports' / 'q1.csv'
print(type(child).__name__, child.tenant, child.parent.tenant)   # TenantPath acme acme

class Plain(Path):          # simple subclasses need nothing extra
    pass
print(type(Plain('/tmp') / 'x').__name__)                         # Plain
```

Do not override private attributes (`_raw_paths`, `_str`, …). Their internals changed again in 3.13/3.14. `with_segments()` is the supported hook.

##### Other `pathlib` / `os.path` Additions in 3.12

```python
import os.path
import ntpath
from pathlib import Path, PurePath, PureWindowsPath

# relative_to(walk_up=True) — may insert '..' like os.path.relpath
print(PurePath('/srv/app/static').relative_to('/srv/app/templates', walk_up=True))   # ../static
try:
    PurePath('/srv/app/static').relative_to('/srv/app/templates')
except ValueError as e:
    print(e)   # '/srv/app/static' is not in the subpath of '/srv/app/templates'

# case_sensitive= for glob(), rglob() and match()
print(PurePath('REPORT.TXT').match('*.txt', case_sensitive=False))   # True

# os.path.splitroot — (drive, root, tail)
print(os.path.splitroot('/home/user/file'))             # ('', '/', 'home/user/file')
print(ntpath.splitroot(r'C:\Users\x'))                   # ('C:', '\\', 'Users\\x')

# Junctions (Windows); always False on POSIX
print(os.path.isjunction('/tmp'), Path('/tmp').is_junction())   # False False
```

- **Deprecated in 3.12:** passing additional positional arguments to `PurePath.relative_to()` and `is_relative_to()` (`p.relative_to('a', 'b')`). Join them first.
- `os` functions **no longer accept bytes-like paths** such as `bytearray`/`memoryview`. Only `str`, `bytes` or `os.PathLike` work (`TypeError` in 3.12, a `DeprecationWarning` before).[^1]
- On Windows, `os.stat()` fills `st_birthtime` with the creation time, and `st_ctime` is deprecated there (it will become the metadata-change time).

##### Temporary Files and `shutil`

```python
import os
import shutil
import tempfile

# New in 3.12: delete_on_close=False — close the file (e.g. so another process
# or Windows can open it) but still delete it when the context manager exits
with tempfile.NamedTemporaryFile('w', suffix='.csv', delete_on_close=False, encoding='utf-8') as tmp:
    tmp.write('col1,col2\n')
    tmp.close()
    print(os.path.exists(tmp.name))     # True — still there after close()
print(os.path.exists(tmp.name))         # False — removed at context exit

# Changed in 3.12: mkdtemp() always returns an absolute path
print(os.path.isabs(tempfile.mkdtemp(dir='.')))   # True

# New in 3.12: shutil.rmtree(onexc=...) receives the exception instance
def on_rmtree_error(func, path, exc: BaseException) -> None:
    print(f'cannot remove {path}: {type(exc).__name__}')

shutil.rmtree('/nonexistent/dir', onexc=on_rmtree_error)   # cannot remove /nonexistent/dir: FileNotFoundError
# ❌ onerror=(func, path, exc_info) is deprecated in 3.12 (documentation only — no runtime warning)

# NEVER use tempfile.mktemp() — TOCTOU race
```

**Security note (3.12.4+, Windows):** `os.mkdir(path, mode=0o700)` now applies an access-control list that restricts the directory to the current user. This makes `tempfile.mkdtemp()` private on Windows (fix for CVE-2024-4030). Older 3.12.x releases ignored `mode` on Windows.[^1]

***

#### `async`/`await` and `asyncio`

##### Structured Concurrency Baseline (3.11+)

```python
import asyncio

async def fetch(i: int) -> int:
    await asyncio.sleep(0.01)
    return i * 2

async def main() -> list[int]:
    async with asyncio.timeout(5):                   # ✅ 3.11+: scoped timeout
        async with asyncio.TaskGroup() as tg:        # ✅ 3.11+: cancels siblings on failure
            tasks = [tg.create_task(fetch(i), name=f'fetch-{i}') for i in range(3)]
    return [t.result() for t in tasks]

print(asyncio.run(main()))                           # [0, 2, 4]
```

- Prefer `TaskGroup` to bare `create_task()` + `gather()`. Errors propagate as an `ExceptionGroup` (handle with `except*`), and no task is left orphaned.
- **Keep a reference to every task you create outside a `TaskGroup`.** The event loop only holds weak references, so an unreferenced task can be garbage-collected mid-flight.
- Always re-raise `asyncio.CancelledError` after cleanup.
- **Changed in 3.12:** `asyncio.wait_for()` is now implemented on top of `asyncio.timeout()`. Both raise `TimeoutError` (`asyncio.TimeoutError` has been an alias of the built-in since 3.11).[^1]

##### New in 3.12: Eager Task Execution

With the eager task factory, a new task starts executing its coroutine **synchronously inside `create_task()`**, up to its first suspension point. Coroutines that finish without ever awaiting (cache hits, memoized results) never get scheduled on the loop at all, which is 2–5x faster for some workloads.[^23]

```python
import asyncio

log: list[str] = []

async def cached_lookup(key: str) -> str:
    log.append(f'run {key}')
    return key.upper()            # completes without suspending

async def main(eager: bool) -> list[str]:
    if eager:
        asyncio.get_running_loop().set_task_factory(asyncio.eager_task_factory)
    task = asyncio.create_task(cached_lookup('a'))
    log.append(f'created done={task.done()}')
    await task
    return log.copy()

print(asyncio.run(main(eager=False)))   # ['created done=False', 'run a']
log.clear()
print(asyncio.run(main(eager=True)))    # ['run a', 'created done=True']  — ORDER CHANGED
```

Reviewer rules for eager tasks:

- **Execution order changes.** Code that relies on `create_task()` *not* running anything until the next `await` breaks. For example, code that sets up state after creating the task, or assumes a lock acquired after `create_task()` is taken first.
- A task may already be `done()` when `create_task()` returns. Callbacks added later still fire, but code must not assume `pending` status.
- `asyncio.create_eager_task_factory(custom_task_constructor)` wraps custom task classes. The `asyncio.Task(..., eager_start=True)` constructor parameter exists in 3.12, but the `eager_start=` parameter of **`create_task()` is 3.14**. Do not use it on 3.12.
- **Run 3.12.9 or later if you use eager tasks with `TaskGroup`.** Earlier 3.12.x releases had an optimization that could miss cancellations of eager tasks in a `TaskGroup` (gh-128550), and created reference cycles for eager tasks (gh-128552).[^24]

##### New in 3.12: `asyncio.run(loop_factory=...)` and Other Changes

```python
import asyncio

async def main() -> str:
    return type(asyncio.get_running_loop()).__name__

# Pass a custom loop factory instead of setting a global policy
print(asyncio.run(main(), loop_factory=asyncio.new_event_loop))   # _UnixSelectorEventLoop (on Linux)
# e.g. asyncio.run(main(), loop_factory=uvloop.new_event_loop)
```

- **`asyncio.get_event_loop()` deprecation (3.12):** called with no running loop **and no loop ever set** in that thread, it now emits `DeprecationWarning` and still creates a loop. If a loop was set and later cleared (for example after `asyncio.run()` finished), it raises `RuntimeError`. In 3.14 it always raises `RuntimeError` when there is no current loop. Use `asyncio.run()` at entry points and `asyncio.get_running_loop()` inside coroutines.[^1]
- **Child watchers are deprecated** (`asyncio.set_child_watcher()`, `get_child_watcher()`, `SafeChildWatcher`, `FastChildWatcher`, `MultiLoopChildWatcher`, `AbstractChildWatcher`; all removed in 3.14). The loop now picks `PidfdChildWatcher` on Linux automatically. Delete manual watcher setup.[^1]
- `asyncio.iscoroutine()` now returns `False` for **generators**. Legacy generator-based coroutines are fully gone.
- `asyncio.wait()` and `asyncio.as_completed()` accept generators that yield tasks. (Passing bare **coroutines** to `wait()` has been a `TypeError` since 3.11.)
- Performance: socket writes avoid copies and use `sendmsg()`, `current_task()` is implemented in C (4–6x faster), and task creation defers expensive string formatting. Some benchmarks run up to 75% faster.[^1]

##### Running Blocking Code

```python
import asyncio
import hashlib
from pathlib import Path
import tempfile

async def hash_file(path: Path) -> str:
    def compute() -> str:                          # ALL blocking work inside the thread
        with path.open('rb') as fh:
            return hashlib.file_digest(fh, 'sha256').hexdigest()   # file_digest: 3.11+
    return await asyncio.to_thread(compute)

f = Path(tempfile.mkdtemp()) / 'blob.bin'
f.write_bytes(b'abc')
print(asyncio.run(hash_file(f))[:16])              # ba7816bf8f01cfea
```

Pass the **callable** to `to_thread()`, never its result. `to_thread(path.read_bytes())` does the read on the event loop.

***

#### Concurrency

##### GIL Reality in 3.12

CPython 3.12 still has one GIL per interpreter, and normal Python code runs in a single interpreter. Threads help I/O-bound work. For CPU-bound pure-Python work use processes (or native extensions that release the GIL). Free-threaded CPython (PEP 703) starts with 3.13 and is **not** an option for 3.12.

##### `os.fork()` in Multithreaded Processes Now Warns

**New in 3.12 (POSIX):** `os.fork()` (and anything that uses it, such as the `'fork'` start method of `multiprocessing`) emits a **`DeprecationWarning`** when it detects that the process has more than one thread. Forking a threaded process can deadlock the child on locks held by other threads (logging handlers, allocator locks, …).[^1]

```python
import multiprocessing as mp

def square(x: int) -> int:
    return x * x

if __name__ == '__main__':
    ctx = mp.get_context('spawn')         # ✅ explicit start method — or 'forkserver'
    with ctx.Pool(processes=2) as pool:
        print(pool.map(square, range(5)))  # [0, 1, 4, 9, 16]
```

- **Linux default start method is still `'fork'` in 3.12. It changes to `'forkserver'` in 3.14.** If code depends on `fork` semantics (inheriting module globals or open objects), request it explicitly with `get_context('fork')` now. Better: make the code work with `spawn`/`forkserver`, which require picklable arguments and `if __name__ == '__main__':` guards.[^1]
- The same warning appears when `concurrent.futures.ProcessPoolExecutor` forks from a threaded process. Pass `mp_context=mp.get_context('spawn')`.

##### `concurrent.futures` and `threading`

```python
import logging
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed

logger = logging.getLogger(__name__)

def fetch(url: str) -> int:
    return len(url)

urls = ['https://a.example', 'https://bb.example']
with ThreadPoolExecutor(max_workers=8) as executor:
    futures = {executor.submit(fetch, u): u for u in urls}
    for future in as_completed(futures):
        try:
            print(futures[future], future.result())
        except Exception:
            logger.exception('failed %s', futures[future])

lock = threading.Lock()
results: list[int] = []
def worker(n: int) -> None:
    with lock:
        results.append(n)

threads = [threading.Thread(target=worker, args=(i,)) for i in range(4)]
for t in threads: t.start()
for t in threads: t.join()
print(sorted(results))     # [0, 1, 2, 3]
```

- The legacy camelCase aliases (`Thread.setDaemon()`, `isDaemon()`, `getName()`, `setName()`, `Event.isSet()`, `Condition.notifyAll()`, `threading.currentThread()`, `activeCount()`) have emitted `DeprecationWarning` since 3.10 and still do in 3.12. Use `daemon`, `name`, `is_set()`, `notify_all()`, `current_thread()` and `active_count()`.[^1]
- Never pass arguments to `threading.RLock()`. The C implementation silently ignores them on 3.12. The `DeprecationWarning` only arrives in 3.13 (removal in 3.15), even though the 3.12 "What's New" lists this deprecation.
- `queue.Queue.shutdown()` is **3.13**. In 3.12, signal consumers with a sentinel object.

***

#### Logging

```python
import logging
import logging.config

LOGGING = {
    'version': 1,
    'disable_existing_loggers': False,
    'formatters': {'std': {'format': '%(asctime)s %(name)s %(levelname)s %(message)s'}},
    'handlers': {
        'console': {'class': 'logging.StreamHandler', 'formatter': 'std'},
    },
    'root': {'level': 'INFO', 'handlers': ['console']},
}
logging.config.dictConfig(LOGGING)

logger = logging.getLogger(__name__)
logger.info('Loaded %d records', 42)          # lazy %-formatting, not f-strings

# New in 3.12: look up configured handlers by name
print(logging.getHandlerByName('console'))    # <StreamHandler <stderr> (NOTSET)>
print(sorted(logging.getHandlerNames()))      # ['console'] (frozenset of names)

# 3.11+: name → level mapping
print(logging.getLevelNamesMapping()['WARNING'])   # 30
```

**New in 3.12: `QueueHandler` configuration in `dictConfig`.** You can now configure a `QueueHandler`/`QueueListener` pair declaratively. The listener is created and attached as `handler.listener`, but `dictConfig()` does **not start it for you** (not on 3.12, and not on later versions either): you must call `start()`. Until you do, records pile up in the queue and nothing is emitted. This setup keeps slow handlers (network, files) off request threads:[^25]

```python
import atexit
import logging
import logging.config

logging.config.dictConfig({
    'version': 1,
    'handlers': {
        'file': {'class': 'logging.StreamHandler'},          # the slow sink in real life
        'queue': {
            'class': 'logging.handlers.QueueHandler',
            'handlers': ['file'],                            # QueueListener targets (new in 3.12)
            'respect_handler_level': True,
        },
    },
    'root': {'level': 'INFO', 'handlers': ['queue']},
})
queue_handler = logging.getHandlerByName('queue')
queue_handler.listener.start()                               # ❗ required on 3.12 — not auto-started
atexit.register(queue_handler.listener.stop)                 # flush queued records on shutdown
logging.getLogger('app').info('non-blocking log record')
```

Calling `stop()` on a listener that was never started raises `AttributeError` on 3.12. Stop the listener on shutdown, or queued records can be lost.

***

#### Security

##### New in 3.12: Tar Extraction Filters (PEP 706) — Always Pass `filter=`

Tar archives can contain absolute paths, `../` components, links that point outside the destination, device files and setuid bits. `TarFile.extract()`/`extractall()` and `shutil.unpack_archive()` take a `filter=` argument that limits what extraction may do:[^26][^27]

| Filter | Use for | Behavior |
|---|---|---|
| `'data'` | **Untrusted archives** (uploads, downloads, CI artifacts) | Everything `'tar'` does, plus: refuses links to absolute paths or outside the destination (`AbsoluteLinkError`, `LinkOutsideDestinationError`), refuses device files and pipes (`SpecialFileError`), makes regular files owner-readable/writable, drops group/other exec bits when the owner has none, and ignores stored owner/group (uid/gid) |
| `'tar'` | Trusted archives that need UNIX tar semantics | Strips leading `/`, refuses still-absolute paths (`AbsolutePathError`) and paths that end up outside the destination after following symlinks (`OutsideDestinationError`), clears setuid/setgid/sticky and group/other write bits |
| `'fully_trusted'` | Archives you created yourself | Old behavior: no restrictions |

```python
import io
import tarfile
import tempfile

def make_tar(name: str) -> io.BytesIO:
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode='w') as tf:
        info = tarfile.TarInfo(name)
        info.size = 1
        tf.addfile(info, io.BytesIO(b'x'))
    buf.seek(0)
    return buf

dest = tempfile.mkdtemp()

# ✅ Untrusted input: filter='data'
try:
    with tarfile.open(fileobj=make_tar('../evil.txt')) as tf:
        tf.extractall(dest, filter='data')
except tarfile.FilterError as e:             # base class of all filter refusals (a TarError)
    print(type(e).__name__, e)
# OutsideDestinationError '../evil.txt' would be extracted to '/tmp/evil.txt', which is outside the destination

# ❌ No filter: 3.12 keeps the unsafe 'fully_trusted' behavior and emits DeprecationWarning
# with tarfile.open(...) as tf: tf.extractall(dest)
#   DeprecationWarning: Python 3.14 will, by default, filter extracted tar archives ...

# Process-wide opt-in for code you cannot change (e.g. third-party callers):
tarfile.TarFile.extraction_filter = staticmethod(tarfile.data_filter)
```

Reviewer rules:

- **Every `extractall()`/`extract()`/`shutil.unpack_archive()` on a tar must pass `filter=`.** Use `'data'` unless there is a documented reason not to. **The default changes to `'data'` in 3.14**, so code that relies on extracting device files, absolute symlinks or setuid bits will break there anyway. The `filter=` argument was also backported to security releases of 3.8–3.11. Shared code that must run on older patch levels should feature-detect with `hasattr(tarfile, 'data_filter')`.
- Run the **latest 3.12.x**. The filters had several bypasses that were fixed in point releases:[^24]
  - **3.12.11** (June 2025): symlink-target normalization, `os.path.realpath(strict=os.path.ALLOW_MISSING)`, re-applying the filter on link fallbacks (new `tarfile.LinkFallbackError`), no extraction of rejected members when `errorlevel=0`. These fix CVE-2024-12718, CVE-2025-4138, CVE-2025-4330, CVE-2025-4435 and CVE-2025-4517. The 3.12 "What's New" files these under "Notable changes in 3.12.10", but the changelog shows they shipped in **3.12.11** (gh-135034). 3.12.10 is still vulnerable.
  - **3.12.14**: a symlink-outside-destination bypass of CVE-2025-4330, link-target validation, and the filter applied on link-target fallback.
  - **3.12.15**: a hard-link-to-symlink bypass (CVE-2026-82049), and directories created outside the destination via `../x/../dest` names.
- `zipfile.extractall()` sanitizes `..` and absolute names itself, but **3.12.14** fixed a `shutil.unpack_archive()` ZIP path traversal on Windows via drive prefixes. Validate member names yourself for anything security-critical.

##### Security Fixes in 3.12 Point Releases

3.12 is in security-only maintenance. Pin to the newest patch release and rebuild images when a new one ships. Notable fixes and the version that has them (from the official changelog and "What's New"):[^24][^1]

| Release | Fix |
|---|---|
| 3.12.2 | `.pth` files whose names start with a dot (hidden files) are skipped |
| 3.12.3 | Bundled libexpat 2.6.0. New `XMLParser.flush()`/`SetReparseDeferralEnabled()` to control Expat reparse deferral (CVE-2023-52425) |
| 3.12.4 | `ipaddress` `is_global`/`is_private` corrected. Windows `os.mkdir(mode=0o700)` restricts access, so `tempfile.mkdtemp()` is private (CVE-2024-4030) |
| 3.12.5 | `email` refuses to serialize headers with embedded newlines (header injection; `policy.verify_generated_headers`). The `socket.socketpair()` fallback (Windows) authenticates the connection |
| 3.12.6 | `email.utils.getaddresses()`/`parseaddr()` reject malformed addresses (`strict=True` default; CVE-2023-27043). Tar header parsing no longer backtracks |
| 3.12.7 | IPv4-mapped `IPv6Address` properties use the mapped IPv4 value |
| 3.12.9 | `urllib.parse.urlsplit()` rejects `[`/`]` in non-IPv6 hosts. `imaplib` memory DoS. Email quoted-string folding spoofing |
| 3.12.10 | `SpooledTemporaryFile.writelines()` unbounded buffering. Email encoded-word folding spoofing. libexpat 2.7.1 |
| 3.12.11 | Tar filter bypasses (CVE-2024-12718, CVE-2025-4138, CVE-2025-4330, CVE-2025-4435, CVE-2025-4517). Use-after-free in the `unicode-escape` decoder. Long IPv6 address DoS in `ipaddress` |
| 3.12.12 | `html.parser.HTMLParser` follows HTML5 for comments, CDATA, script/style end tags, and has quadratic-time fixes. zip64 end-of-central-directory consistency check |
| 3.12.13 | Memory DoS fixes in `http.client`, `http.server` (CGI on Windows) and `plistlib`. Quadratic `os.path.expandvars()`. Control characters rejected in `http.cookies.Morsel`, `wsgiref.headers` and `data:` URLs. Email comment-folding header injection |
| 3.12.14 | `http.client` trailer/1xx-response flood limits. `webbrowser` rejects dash-prefixed URLs. `wsgiref` status-line injection. `configparser` ReDoS. `unicodedata.normalize()` quadratic DoS. `SourcelessFileLoader` uses `io.open_code()` (CVE-2026-2297). Several tar filter bypasses. libexpat 2.8.x |
| 3.12.15 | Tar hard-link-to-symlink bypass (CVE-2026-82049). `zipfile` bounded decompression for bzip2/LZMA. `urllib.request` `HTTPPasswordMgr` scopes credentials to the URL scheme (CVE-2026-15806). `ssl` SNI-callback crash (CVE-2026-19445). `asyncio`/`ssl` now warn (`DeprecationWarning`) when a hostname-checking context gets no `server_hostname` (3.13+ raises `ValueError`) |

Point-release API additions that you can only rely on when you require that patch level: `email.utils.getaddresses(strict=...)`/`supports_strict_parsing` (3.12.6), `email.policy.verify_generated_headers` (3.12.5), `os.path.ALLOW_MISSING` and `tarfile.LinkFallbackError` (3.12.11), and `XMLParser.flush()` (3.12.3). Feature-detect with `hasattr()` if you support older 3.12.x.

##### Removed Insecure APIs (3.12)

- `ssl.wrap_socket()` is removed. It sent no SNI and did not verify hostnames (CWE-295). Use `ssl.create_default_context().wrap_socket(sock, server_hostname=host)`.
- `ssl.match_hostname()` is removed (OpenSSL does hostname matching), and so is `ssl.RAND_pseudo_bytes()` (use `os.urandom()` or `secrets`).
- The `keyfile`/`certfile` parameters of `ftplib`, `imaplib`, `poplib` and `smtplib`, and `key_file`/`cert_file`/`check_hostname` of `http.client`, are removed. Pass an `ssl.SSLContext` via `context=` (`ssl_context=` in `imaplib`).[^1]

```python
import socket
import ssl

ctx = ssl.create_default_context()          # verifies certs + hostname, TLS ≥ 1.2 by default
print(ctx.check_hostname, ctx.verify_mode == ssl.CERT_REQUIRED, repr(ctx.minimum_version))
# True True <TLSVersion.TLSv1_2: 771>

def open_tls(host: str, port: int = 443) -> ssl.SSLSocket:
    raw = socket.create_connection((host, port), timeout=10)
    return ctx.wrap_socket(raw, server_hostname=host)   # ✅ SNI + hostname verification
```

`hashlib` in 3.12 uses formally verified HACL\* implementations of MD5, SHA-1, SHA-2 and SHA-3 as fallbacks when OpenSSL does not provide them. The pure-Python `pbkdf2_hmac` fallback was removed (OpenSSL's is always used).[^1]

##### Secrets, SQL, Subprocess, XML

```python
import secrets
import sqlite3
import subprocess
import sys

token = secrets.token_urlsafe(32)                 # session tokens, reset links
print(secrets.compare_digest(token, token))        # constant-time comparison → True
# NEVER random.random()/randbytes() for security values

conn = sqlite3.connect(':memory:')
conn.execute('CREATE TABLE users (id INTEGER, name TEXT)')
user_id = '1 OR 1=1'
# ❌ conn.execute(f'SELECT * FROM users WHERE id = {user_id}')
rows = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,)).fetchall()   # ✅
print(rows)                                         # []
conn.close()

# ✅ argument list, no shell, timeout, check
result = subprocess.run([sys.executable, '-c', 'print("ok")'],
                        capture_output=True, text=True, encoding='utf-8', timeout=30, check=True)
print(result.stdout.strip())                        # ok
# ❌ subprocess.run(f'ls {user_input}', shell=True)  — command injection
```

- Parse untrusted XML with `defusedxml`. The stdlib parsers depend on the bundled/system libexpat, and 3.12.x keeps updating it for hash-flooding and recursion fixes.
- **`int()`/`str()` digit limit:** parsing untrusted numeric strings is bounded at 4300 digits by default. Do not raise the limit globally with `sys.set_int_max_str_digits(0)` in services that parse user input.
- `pickle.load()`/`marshal`/`shelve` on untrusted data allow arbitrary code execution. `eval()`/`exec()` on untrusted input is forbidden.

***

#### `sqlite3`

##### New in 3.12: `autocommit` — PEP 249-Compliant Transactions

`sqlite3.connect(..., autocommit=...)` and `Connection.autocommit` give explicit, standards-compliant transaction control. The default is `sqlite3.LEGACY_TRANSACTION_CONTROL`, which keeps the old `isolation_level`-based behavior:[^28]

| `autocommit=` | Behavior |
|---|---|
| `LEGACY_TRANSACTION_CONTROL` (default, `-1`) | Pre-3.12 behavior: implicit `BEGIN` before DML, governed by `isolation_level`. DDL handling is surprising |
| `False` (**recommended** for new code) | PEP 249: a transaction is **always open**. `commit()`/`rollback()` start a new one. **`close()` rolls back uncommitted changes** |
| `True` | SQLite autocommit mode: every statement commits immediately. `commit()`/`rollback()` do nothing. Use explicit `BEGIN`/`COMMIT` |

```python
import sqlite3
import tempfile
from pathlib import Path

db = Path(tempfile.mkdtemp()) / 'app.db'

with sqlite3.connect(db, autocommit=False) as conn:   # `with` commits on success, rolls back on error
    conn.execute('CREATE TABLE t (x INTEGER)')
    conn.execute('INSERT INTO t VALUES (1)')
conn.close()                                           # `with` does NOT close the connection!

conn = sqlite3.connect(db, autocommit=False)
conn.execute('INSERT INTO t VALUES (2)')
conn.close()                                           # ❗ uncommitted INSERT is rolled back

conn = sqlite3.connect(db, autocommit=False)
print(conn.execute('SELECT x FROM t').fetchall())      # [(1,)]
print(conn.in_transaction)                             # True — always in a transaction
conn.close()
```

- `with conn:` manages a **transaction**, not the connection's lifetime. Close explicitly or use `contextlib.closing(conn)`.
- Under `autocommit=False`, long-lived connections hold a read transaction open. Commit (or roll back) promptly so other writers and WAL checkpoints are not blocked.
- New in 3.12: `Connection.getconfig()`/`setconfig()`, the `entrypoint=` parameter of `load_extension()`, and a command-line shell: `python -m sqlite3 app.db`.

##### Deprecated in 3.12 (warns now)

```python
import sqlite3
import warnings
from datetime import datetime

conn = sqlite3.connect(':memory:')
conn.execute('CREATE TABLE t (a, b)')

with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter('always')
    # ❌ named placeholders with a SEQUENCE — DeprecationWarning; ProgrammingError in 3.14
    conn.execute('INSERT INTO t VALUES (:a, :b)', (1, 2))
    # ❌ default datetime adapter — DeprecationWarning
    conn.execute('INSERT INTO t VALUES (?, ?)', (datetime(2024, 1, 1), 1))
print(len(w), w[0].category.__name__)            # 3 DeprecationWarning (one per named binding + adapter)

# ✅ named placeholders take a dict; store datetimes explicitly as ISO strings
conn.execute('INSERT INTO t VALUES (:a, :b)', {'a': 1, 'b': 2})
conn.execute('INSERT INTO t VALUES (?, ?)', (datetime(2024, 1, 1).isoformat(), 1))
conn.close()
```

The default adapters (`date`, `datetime`) and converters (`"date"`, `"timestamp"` with `detect_types`) are deprecated. Register explicit adapters/converters (`sqlite3.register_adapter(datetime, datetime.isoformat)`) or store ISO-8601 strings / epoch integers. `sqlite3.version` and `sqlite3.version_info` (the module version, not SQLite's) are deprecated and removed in 3.14. Use `sqlite3.sqlite_version`.

***

#### JSON, TOML and Serialization

```python
import json
import tomllib
from datetime import UTC, datetime

data = {'name': 'Alice', 'tags': ['admin', 'user']}
text = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False)
assert json.loads(text) == data

class AppEncoder(json.JSONEncoder):
    def default(self, o: object) -> object:
        if isinstance(o, datetime):
            return o.isoformat()
        if isinstance(o, set):
            return sorted(o)
        return super().default(o)

print(json.dumps({'ts': datetime(2024, 1, 1, tzinfo=UTC)}, cls=AppEncoder))
# {"ts": "2024-01-01T00:00:00+00:00"}

# tomllib (3.11+): read-only TOML parser — open in BINARY mode
config = tomllib.loads('[server]\nport = 8080\n')
print(config['server']['port'])          # 8080
# with open('pyproject.toml', 'rb') as f: tomllib.load(f)   — text mode raises TypeError
```

`pickle.DEFAULT_PROTOCOL` is **4** in 3.12 (it becomes 5 in 3.14). Pass `protocol=pickle.HIGHEST_PROTOCOL` (5) explicitly for large buffers. Never unpickle untrusted data.

***

#### CSV

```python
import csv
import io

buf = io.StringIO(newline='')
row = ['text', 1, None, '', 2.5]

csv.writer(buf, quoting=csv.QUOTE_STRINGS).writerow(row)   # New in 3.12: quote only str
csv.writer(buf, quoting=csv.QUOTE_NOTNULL).writerow(row)   # New in 3.12: quote everything except None
print(buf.getvalue())
# "text",1,,"",2.5
# "text","1",,"","2.5"
```

The new constants make `None` distinguishable from `''` in the output. With `QUOTE_STRINGS`, readers can treat unquoted empty fields as `None`. Always open CSV files with `newline=''` and an explicit `encoding`.

***

#### Regular Expressions

```python
import re

DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')   # raw strings!
m = DATE_RE.match('2024-06-15')
if m:
    print(m['year'], m.group('month'))          # 2024 06

# Atomic groups and possessive quantifiers (3.11+) — prevent catastrophic backtracking
print(re.fullmatch(r'(?>a+)b', 'aaab') is not None)   # True
print(re.fullmatch(r'a++b', 'aaab') is not None)      # True

# Changed in 3.12: group references in replacements must be plain ASCII digits/names
print(re.sub('(a)', r'\g<1>', 'a'))                   # a
try:
    re.sub('(a)', r'\g<+1>', 'a')                     # ❌ accepted (with a warning) before 3.12
except re.error as e:
    print(e)                                          # bad character in group name '+1' at position 3
```

`re.sub()`/`subn()` with group references in the replacement string is 2–3x faster in 3.12. Since 3.12, group names in **bytes** patterns and replacements may only contain ASCII letters, digits and underscores.[^1]

***

#### `random` and `statistics`

```python
import random
import statistics

rng = random.Random(42)                         # reproducible simulations/tests
print(rng.binomialvariate(n=10, p=0.5))         # New in 3.12 — an int in 0..10
print(rng.expovariate() > 0)                    # New in 3.12: lambd defaults to 1.0 → True

# Changed in 3.12: non-integer arguments to randrange() raise TypeError
try:
    random.randrange(10.0)                      # ❌ worked (with a warning) before 3.12
except TypeError as e:
    print(e)                                    # 'float' object cannot be interpreted as an integer

# New in 3.12: Spearman rank correlation
print(statistics.correlation([1, 2, 3, 4], [1, 4, 9, 16], method='ranked'))   # 1.0
```

`random` is never suitable for secrets. Use `secrets`.

***

#### Memory Management

```python
import gc
import sys
import tracemalloc

print(sys.getsizeof('a'))        # 42 on 64-bit 3.12 — 8 bytes smaller than 3.11 (PEP 623 removed wstr)

tracemalloc.start(10)
data = [str(i) for i in range(10_000)]
snapshot = tracemalloc.take_snapshot()
top = snapshot.statistics('lineno')[:3]
print(len(top) > 0)              # True
tracemalloc.stop()

gc.collect()                     # explicit collection of reference cycles
print(gc.isenabled())            # True
```

- **Immortal objects** (PEP 683) have a fixed refcount. Do not write tests that assert `sys.getrefcount()` of `None`, `True`, small ints or interned strings.
- Weak references (`weakref.WeakValueDictionary`, `weakref.finalize`) are still the tool for caches that must not keep objects alive.
- `__slots__` (or `@dataclass(slots=True)`) saves memory for classes with many instances.

***

#### Performance

What 3.12 gives you for free:[^1]

- **Comprehensions** (list/dict/set) are inlined: up to 2x faster (PEP 709).
- **`super()` method calls and attribute loads** are faster (new `LOAD_SUPER_ATTR` opcode).
- **`isinstance()` against small runtime-checkable protocols** is 2–20x faster. Protocols with many members may be *slower* than in 3.11.
- **`asyncio`**: faster socket writes, a C `current_task()` and eager tasks.
- **`re.sub()` with group references** is 2–3x faster, and **`tokenize`** is up to 64% faster.
- **`str` objects** are 8–16 bytes smaller.

```python
import cProfile
import io
import pstats
import timeit

def work() -> int:
    return sum(i * i for i in range(10_000))

with cProfile.Profile() as pr:          # context-manager form (3.8+)
    work()
stream = io.StringIO()
pstats.Stats(pr, stream=stream).sort_stats('cumulative').print_stats(5)
print('work' in stream.getvalue())      # True

print(timeit.timeit('[x * 2 for x in range(100)]', number=1000) > 0)   # True
```

- Use **`python -X perf`** with Linux `perf` for production-like profiling (see the internals section), and `python -X importtime` to find slow imports.
- `frozenset` membership tests, `str.join()` instead of `+=` in loops, and generator expressions for one-pass aggregation remain the basic wins.

***

#### Testing

##### Removed in 3.12: Deprecated `unittest` Aliases

These `TestCase` aliases, deprecated since 3.1–3.5, are **gone**. Old test suites fail with `AttributeError` on 3.12:[^1]

| Removed alias | Use instead |
|---|---|
| `failUnless`, `assert_` | `assertTrue()` |
| `failIf` | `assertFalse()` |
| `failUnlessEqual`, `assertEquals` | `assertEqual()` |
| `failIfEqual`, `assertNotEquals` | `assertNotEqual()` |
| `failUnlessAlmostEqual`, `assertAlmostEquals` | `assertAlmostEqual()` |
| `failIfAlmostEqual`, `assertNotAlmostEquals` | `assertNotAlmostEqual()` |
| `failUnlessRaises` | `assertRaises()` |
| `assertRegexpMatches` | `assertRegex()` |
| `assertRaisesRegexp` | `assertRaisesRegex()` |
| `assertNotRegexpMatches` | `assertNotRegex()` |

`assertDictContainsSubset()`, `TestLoader.loadTestsFromModule(use_load_tests=...)` and `unittest._TextTestResult` are removed too. **`unittest.findTestCases()`, `makeSuite()` and `getTestCaseNames()` still exist but warn, and are removed in 3.13.** Use `TestLoader` methods.

```python
import io
import unittest

class TestMath(unittest.TestCase):
    def test_add(self) -> None:
        self.assertEqual(1 + 1, 2)          # ✅ not assertEquals

    def test_raises(self) -> None:
        with self.assertRaisesRegex(ZeroDivisionError, 'division'):
            1 / 0

suite = unittest.defaultTestLoader.loadTestsFromTestCase(TestMath)
result = unittest.TextTestRunner(stream=io.StringIO()).run(suite)
print(result.wasSuccessful())              # True
print(hasattr(unittest.TestCase, 'assertEquals'))   # False on 3.12
```

- **New in 3.12:** `python -m unittest --durations=5` reports the slowest tests.
- Returning a non-`None` value from a test method is deprecated (`DeprecationWarning` since 3.11). It usually means a forgotten `assert` or an un-awaited coroutine.
- Run CI with `-W error::DeprecationWarning` (or pytest's `filterwarnings = ["error"]`) so 3.13/3.14 removals surface now.

```python
import asyncio
import unittest
from unittest.mock import AsyncMock

class TestAsync(unittest.IsolatedAsyncioTestCase):
    async def test_fetch(self) -> None:
        fetch = AsyncMock(return_value={'id': 1})
        self.assertEqual(await fetch(1), {'id': 1})
        fetch.assert_awaited_once_with(1)

    async def test_timeout(self) -> None:
        with self.assertRaises(TimeoutError):
            async with asyncio.timeout(0.01):
                await asyncio.sleep(1)
```

***

#### Virtual Environments and Packaging

##### Removed in 3.12: `distutils`; `venv` No Longer Installs `setuptools`

- **`distutils` is gone** (PEP 632). `import distutils` raises `ModuleNotFoundError`, **unless `setuptools` is installed**: setuptools injects its own vendored copy via a `.pth` hook, so the import may "work" in one environment and fail in a clean one. Never rely on that. Migrate `distutils.version.LooseVersion` → `packaging.version.Version`, `distutils.util.strtobool` → your own helper, `distutils.spawn.find_executable` → `shutil.which`, `distutils.dir_util.copy_tree` → `shutil.copytree(..., dirs_exist_ok=True)`, and `distutils.sysconfig` → `sysconfig`.[^29]
- **`venv` and `ensurepip` no longer pre-install `setuptools`** (gh-95299). In a fresh 3.12 venv, `import setuptools`, `import pkg_resources` and `easy_install` fail. Packages that use `pkg_resources` at runtime must declare `setuptools` as a dependency, and should migrate to `importlib.metadata`/`importlib.resources`. `pip` still provides setuptools in its isolated build environments, so `pip install` of setuptools-based sdists keeps working.[^1]
- **`imp` is gone.** Use `importlib` (`imp.reload` → `importlib.reload`, `imp.load_source` → `importlib.util.spec_from_file_location` + `module_from_spec` + `loader.exec_module`, `imp.new_module` → `types.ModuleType`, `imp.find_module` → `importlib.util.find_spec`).
- `pkgutil.find_loader()`/`get_loader()` warn (removed in 3.14). `importlib.abc.ResourceReader`, `Traversable` and `TraversableResources` are deprecated in favor of `importlib.resources.abc`.

```python
import importlib.metadata
import importlib.util
import sys
import tempfile
import types
from pathlib import Path

# Replacement for imp.load_source()
def load_source(modname: str, filename: str) -> types.ModuleType:
    spec = importlib.util.spec_from_file_location(modname, filename)
    if spec is None or spec.loader is None:
        raise ImportError(f'cannot load {filename}')
    module = importlib.util.module_from_spec(spec)
    sys.modules[modname] = module          # optional: make it importable by name
    spec.loader.exec_module(module)
    return module

plugin = Path(tempfile.mkdtemp()) / 'plugin.py'
plugin.write_text('VALUE = 42\n', encoding='utf-8')
print(load_source('plugin', str(plugin)).VALUE)        # 42

# Replacement for pkg_resources.get_distribution(...).version
print(importlib.metadata.version('pip') is not None)   # True when pip is installed
```

```bash
python3.12 -m venv .venv
source .venv/bin/activate
python -m pip install -r requirements.txt

# uv can provision a patched 3.12 interpreter itself
uv venv --python 3.12
uv pip install -r requirements.txt
```

##### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=69", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "myapp"
version = "1.0.0"
requires-python = ">=3.12"      # unlocks type statements, PEP 695 generics, PEP 701 f-strings
dependencies = ["tzdata; sys_platform == 'win32'"]

[tool.ruff]
target-version = "py312"
line-length = 88

[tool.ruff.lint]
select = ["E", "F", "W", "I", "UP", "B", "S", "DTZ"]   # DTZ flags naive datetimes (utcnow, …)

[tool.mypy]
python_version = "3.12"
strict = true
enable_error_code = ["explicit-override"]

[tool.pytest.ini_options]
testpaths = ["tests"]
filterwarnings = ["error"]      # surface 3.12 deprecations before 3.13/3.14 remove the APIs
```

##### Project Layout

```
project/
├── .venv/
├── src/
│   └── myapp/
│       ├── __init__.py
│       ├── py.typed
│       └── main.py
├── tests/
│   ├── conftest.py
│   └── test_main.py
├── pyproject.toml
└── uv.lock / requirements.txt
```

***

#### Code Quality Tools

```bash
pip install ruff mypy bandit

ruff check --target-version py312 src/ tests/     # lint (incl. pyupgrade "UP" rules)
ruff format src/ tests/                            # format (understands PEP 695/701 syntax)
mypy --strict src/                                 # PEP 695 syntax on by default since mypy 1.12
bandit -r src/                                     # security lint
python -W error::SyntaxWarning -m compileall -f -q src/   # catch invalid escape sequences
```

- Ruff with `target-version = "py312"` offers the `UP040`/`UP046`/`UP047` rewrites to `type` aliases and PEP 695 generics, plus `UP` rules that drop `typing.List`/`Optional`/`Union`. Review these rewrites. A `type` alias is **not** a drop-in replacement where the alias is used at runtime (see the PEP 695 section).[^8]
- Pyright fully supports PEP 695, 692, 698 and 701. Turn on `reportImplicitOverride` to require `@override`.
- Use formatter and linter releases that understand PEP 701. Old tokenizer-based tools mis-parse f-strings that reuse quotes.

***

#### Removed and Changed in 3.12 — Migration Notes

##### Removed in 3.12

| Removed | Replacement |
|---|---|
| `distutils` package | `setuptools`, `packaging`, `shutil`, `sysconfig` |
| `imp` module | `importlib` (see the Packaging section) |
| `asynchat`, `asyncore` | `asyncio` |
| `smtpd` | `aiosmtpd` (PyPI) |
| `unittest` aliases (`assertEquals`, `failUnless`, `assertRegexpMatches`, …) | `assertEqual`, `assertTrue`, `assertRegex`, … |
| `configparser.SafeConfigParser`, `ConfigParser.readfp()`, `ParsingError.filename` | `ConfigParser`, `read_file()`, `.source` |
| `ssl.wrap_socket()`, `ssl.match_hostname()`, `ssl.RAND_pseudo_bytes()` | `SSLContext.wrap_socket(server_hostname=...)`, built-in hostname checks, `os.urandom()` |
| `keyfile`/`certfile` in `ftplib`/`imaplib`/`poplib`/`smtplib`; `key_file`/`cert_file`/`check_hostname` in `http.client` | `context=` with an `ssl.SSLContext` |
| `locale.format()` | `locale.format_string()` |
| `gzip.GzipFile.filename` | `.name` |
| `io.OpenWrapper` | `open()` |
| `sqlite3.enable_shared_cache()`, `sqlite3.OptimizedUnicode` | URI `?cache=shared`, `str` |
| `importlib.util.set_package`/`set_loader`/`module_for_loader`, `find_loader()`/`find_module()` APIs, `importlib.abc.Finder`, `pkgutil.ImpImporter`/`ImpLoader` | `find_spec()`/`exec_module()`-based APIs |
| `zipimport.zipimporter.find_loader()`/`find_module()` | `find_spec()` |
| `xml.etree.ElementTree.Element.copy()` (pure-Python impl.) | `copy.copy()` |
| `EnumMeta.__getattr__` | normal attribute access |
| `ftplib.FTP_TLS.ssl_version` | `context=` |
| Old `webbrowser` browsers (Netscape, Mosaic, Galeon, …) | — |

##### Behavior Changes That Break Old Code

- **Invalid escape sequences** → `SyntaxWarning` (visible) instead of a silent `DeprecationWarning`.
- **`random.randrange()`** with non-integers (`10.0`, `'10'`, `10.5`) → `TypeError`.
- **`shlex.split(None)`** raises instead of reading `sys.stdin`.
- **`os` functions reject `bytearray`/`memoryview` paths.**
- **`re`**: numeric group references must be ASCII digits. Group names in bytes patterns must be ASCII identifiers.
- **`functools.cached_property`** no longer locks, so the getter may run concurrently on the same instance.
- **`sum()` of floats** is more accurate, so the last-bit results differ from 3.11.
- **Comprehensions** have no separate frame, and `locals()` inside a comprehension includes outer variables.
- **`enum`**: `x in MyEnum` returns a bool for non-members instead of raising `TypeError`.
- **`__set_name__`** exceptions are not wrapped in `RuntimeError`.
- **`asyncio.iscoroutine()`** returns `False` for generators. `get_event_loop()` warns when it would create a loop.
- **`tempfile.mkdtemp()`** always returns an absolute path.
- **`sys._current_exceptions()`** values are exception instances.
- **`argparse`** reads `fromfile_prefix_chars` argument files with the filesystem encoding (UTF-8 on Windows) instead of the locale encoding.
- **`tokenize`** emits `FSTRING_*` tokens, raises `TokenError` for incomplete single-line strings, and raises `TabError` for mixed tabs/spaces.
- **Null bytes in source** → `SyntaxError` (also from `ast.parse()`, which used to raise `ValueError`).
- **`typing`**: runtime-checkable protocol `isinstance()` uses `getattr_static()` (properties and `__getattr__` are not called).
- **`syslog.openlog()`/`closelog()`** fail in subinterpreters.

##### Deprecated in 3.12 — Removed in 3.13

Importing these modules emits `DeprecationWarning` in 3.12. They are **gone** in 3.13 (PEP 594 and others), so migrate now:[^1][^30]

| Module / API | Replacement |
|---|---|
| `cgi`, `cgitb` | `email.message`/`multipart` (PyPI), a web framework, `logging` + `traceback` |
| `crypt`, `spwd` | `hashlib.scrypt()`, `bcrypt`/`argon2-cffi` (PyPI); PAM libraries |
| `telnetlib`, `nntplib` | `telnetlib3`/`asyncssh` (PyPI); a third-party NNTP client |
| `pipes` | `subprocess` + `shlex.quote()` |
| `imghdr`, `sndhdr` | `filetype`/`puremagic` (PyPI) or the file's magic bytes |
| `aifc`, `audioop`, `chunk`, `sunau`, `ossaudiodev`, `msilib`, `nis`, `mailcap`, `uu`, `xdrlib` | Third-party packages or format-specific code |
| `lib2to3` and the `2to3` tool | `ast`, LibCST, parso |
| `unittest.makeSuite()`, `findTestCases()`, `getTestCaseNames()` | `unittest.TestLoader` methods |
| `locale.resetlocale()` | `locale.setlocale(locale.LC_ALL, '')` |
| `configparser.LegacyInterpolation` | `BasicInterpolation`/`ExtendedInterpolation` |

##### Deprecated in 3.12 — Removed in 3.14

| API (warns in 3.12) | Replacement |
|---|---|
| `ast.Num`, `ast.Str`, `ast.Bytes`, `ast.NameConstant`, `ast.Ellipsis` | `ast.Constant` |
| `asyncio` child watchers and `set_child_watcher()`/`get_child_watcher()` | nothing needed (automatic) |
| `pkgutil.find_loader()`, `pkgutil.get_loader()` | `importlib.util.find_spec()` |
| `sqlite3.version`, `sqlite3.version_info` | `sqlite3.sqlite_version` |
| `sqlite3` named placeholders with a sequence of parameters | pass a `dict` (`ProgrammingError` in 3.14) |
| More than one argument to `PurePath.relative_to()`/`is_relative_to()` | join the parts first |
| `copy`/`deepcopy`/`pickle` of `itertools` objects | materialize (`list(...)`) first |
| `importlib.abc.ResourceReader`/`Traversable`/`TraversableResources` | `importlib.resources.abc` |
| Tar extraction without `filter=` (default becomes `'data'`) | pass `filter='data'` |
| Linux default `multiprocessing` start method `'fork'` (becomes `'forkserver'`) | choose a start method explicitly |
| `pty.master_open()`, `pty.slave_open()` | `pty.openpty()` |
| `isdst` parameter of `email.utils.localtime()` | omit it |
| `argparse.BooleanOptionalAction` `type`/`choices`/`metavar` | drop them |

Other 3.12 deprecations without a removal date (or later): `datetime.utcnow()`/`utcfromtimestamp()`; `calendar.January`/`February`; `~bool` (error in 3.16); `collections.abc.ByteString`/`typing.ByteString` (warn on `isinstance()`/subclassing); `typing.Hashable`/`Sized`; `sys.last_type`/`last_value`/`last_traceback`; `shutil.rmtree(onerror=)` (documentation only); 3-argument `throw()`; truth-testing an `xml.etree.ElementTree.Element` (`DeprecationWarning`; use `len(elem)` or `elem is not None`); `os.fork()` in multithreaded processes; setting `__package__`/`__cached__` on modules; `co_lnotab` (removed in 3.15; use `co_lines()`).

***

#### Production Checklist

##### Python 3.12-Specific Items

- [ ] Running the **latest 3.12.x** (3.12.15 as of October 2026). 3.12 is security-only until October 2028. Tar-filter fixes need ≥ 3.12.11 (better 3.12.15), eager tasks with `TaskGroup` need ≥ 3.12.9, and Windows `mkdtemp()` privacy needs ≥ 3.12.4
- [ ] No 3.13+ features in code: no `[T = default]` type-parameter defaults, `ReadOnly`, `TypeIs`, `warnings.deprecated`, `copy.replace`, `Queue.shutdown`, `batched(strict=)`, t-strings, `annotationlib`, `except A, B:` without parentheses, `create_task(eager_start=)`, `lazy import`, `frozendict`/`sentinel`
- [ ] If `requires-python` includes 3.11: no `type` statements, no `def f[T]`/`class C[T]`, and no f-strings with reused quotes, backslashes or comments inside `{}`
- [ ] New generic code uses PEP 695 syntax. `T_co`/`T_contra` naming is gone. Old- and new-style type parameters are not mixed in one definition
- [ ] `type X = ...` used only for annotation-only aliases. Aliases that are called, subclassed or used in `isinstance()` stay plain assignments or `NewType`
- [ ] `@override` on overriding methods. `**kwargs` typed with `Unpack[TypedDict]` where the keys are known
- [ ] No invalid escape sequences: all regexes and Windows paths use raw strings (`compileall` with `-W error::SyntaxWarning` in CI)
- [ ] Every tar extraction passes `filter='data'` (or a justified `'tar'`/`'fully_trusted'`)
- [ ] `datetime.utcnow()`/`utcfromtimestamp()` replaced by `datetime.now(UTC)`/`fromtimestamp(ts, tz=UTC)`. No naive/aware mixing
- [ ] No `distutils`, `imp`, `asyncore`, `asynchat`, `smtpd`. `setuptools`/`pkg_resources` declared explicitly if imported at runtime
- [ ] No imports of PEP 594 modules (`cgi`, `crypt`, `telnetlib`, `pipes`, …) or `lib2to3`. They fail on 3.13
- [ ] `unittest` uses `assertEqual` & co. (aliases removed). No `makeSuite()`/`findTestCases()`
- [ ] `sqlite3`: named placeholders get dicts. No reliance on the default datetime adapters/converters. New code chooses `autocommit=` explicitly and closes connections (`with conn:` is a transaction, not a close)
- [ ] `multiprocessing`/`ProcessPoolExecutor` choose a start method explicitly. No `fork` from multithreaded processes
- [ ] `functools.cached_property` getters with side effects are locked explicitly (no implicit lock since 3.12)
- [ ] No zero-argument `super()` in methods of `@dataclass(slots=True)` classes
- [ ] Eager task factory only where ordering changes were reviewed. Tasks created outside a `TaskGroup` are referenced
- [ ] No `asyncio.get_event_loop()` outside a running loop. No child-watcher configuration
- [ ] No `~` on booleans. No `ByteString` (use `collections.abc.Buffer`)
- [ ] Tests do not assert refcounts of immortal objects or exact float sums that changed with Neumaier summation
- [ ] CI runs with `-W error::DeprecationWarning` (pytest `filterwarnings = ["error"]`)

##### Code Correctness

- [ ] `bytes` and `str` never mixed without explicit encode/decode
- [ ] `encoding='utf-8'` passed to all text `open()`/`read_text()`/`write_text()` calls
- [ ] `newline=''` passed to `open()` for CSV files
- [ ] No mutable default arguments. Dataclass mutable fields use `field(default_factory=...)`
- [ ] `__hash__` defined alongside `__eq__` only on immutable classes
- [ ] No mutation of a dict/list while iterating it
- [ ] `match` statements use dotted names or literals for constants (bare names capture)
- [ ] Exhaustive `match`/`if` chains end in `assert_never()` where the input is a closed `Literal`/enum

##### Safety and Security

- [ ] SQL queries parameterized. No f-string interpolation
- [ ] `subprocess` with argument lists, no `shell=True` on user input, with `timeout=`
- [ ] `pickle`, `marshal`, `shelve`, `eval`, `exec` never applied to untrusted data
- [ ] `secrets` (not `random`) for tokens. `secrets.compare_digest()` for comparisons
- [ ] `defusedxml` for untrusted XML
- [ ] TLS via `ssl.create_default_context()` and `server_hostname=`. No removed `ssl.wrap_socket()`
- [ ] Untrusted archive extraction constrained (`filter='data'` for tar, member-name validation for zip)
- [ ] `sys.set_int_max_str_digits()` not raised globally in services that parse user input

##### Performance

- [ ] Profiled before optimizing (`cProfile`, `python -X perf` + Linux `perf`, `-X importtime`)
- [ ] `itertools.batched()` for chunking. Generator expressions for one-pass pipelines
- [ ] `frozenset`/`set` for membership tests. `str.join()` for building strings
- [ ] Logging uses lazy `%`-formatting. Slow handlers sit behind `QueueHandler`
- [ ] Blocking calls in async code wrapped in `asyncio.to_thread()`

##### Observability

- [ ] `logging.getLogger(__name__)` in every module. No `print()` in production paths
- [ ] `logger.exception()` inside `except` blocks. Bare `raise` to re-raise
- [ ] Custom exceptions inherit from `Exception`. Context added with `add_note()`
- [ ] `QueueListener` from `dictConfig()` explicitly started (`dictConfig()` never starts it) and stopped at shutdown so queued records are flushed
