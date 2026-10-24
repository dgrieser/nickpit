### Python 3.10: The Complete Production Developer Guide

#### Overview

Python 3.10 was released on October 4, 2021. It reached **end-of-life on October 1, 2026**: per the release-schedule PEP 619, 3.10.22 (released 2026-10-01) is the final security release, the branch is frozen, and the bug tracker no longer accepts 3.10 issues. Bugfix releases with binary installers ended with 3.10.11 (April 2023); everything after that (3.10.12–3.10.22) was source-only security releases. As of October 2026 a 3.10 codebase receives **no further fixes of any kind** — not even for the tarfile, email, HTTP and XML vulnerabilities that were still being patched into 3.10.21/3.10.22. Treat an upgrade plan as a production requirement, and in the meantime make sure you run **3.10.22**, not an older patch release.

Python 3.10 is a landmark language release: **structural pattern matching** (`match`/`case`, PEPs 634–636), the **`X | Y` union operator** for types (PEP 604), **parenthesized context managers**, **`zip(strict=True)`** (PEP 618), `typing.ParamSpec`/`Concatenate` (PEP 612), `typing.TypeAlias` (PEP 613), `typing.TypeGuard` (PEP 647), `dataclass(slots=True, kw_only=True)` and `KW_ONLY`, the `aiter()`/`anext()` built-ins, opt-in `EncodingWarning` (PEP 597), precise line numbers for tracing (PEP 626), and dramatically better syntax/name/attribute error messages. It also requires OpenSSL 1.1.1+ (PEP 644), deprecates `distutils` (PEP 632), and **removes** the `collections` ABC aliases and the `loop=` parameter of the high-level asyncio API — the two removals that break most 3.9 code.

This guide targets Python 3.10 specifically. Features introduced in earlier versions — 3.6 f-strings, 3.7 dataclasses and guaranteed dict ordering, 3.8 walrus operator and positional-only parameters, 3.9 built-in generics (`list[int]`), dict `|`/`|=`, `str.removeprefix()`, `zoneinfo`, `graphlib`, `functools.cache`, `asyncio.to_thread()` — are fully available and used freely here. Every section that covers a 3.10-specific change is marked **New in 3.10**.

**Do not use or recommend anything newer than 3.10.** None of the following exist on a 3.10 interpreter (they raise `ImportError`/`AttributeError`/`SyntaxError`/`TypeError`):

- **3.11:** `ExceptionGroup`/`BaseExceptionGroup`, `except*`, `BaseException.add_note()`; `tomllib`; `asyncio.TaskGroup`, `asyncio.timeout()`/`timeout_at()`, `asyncio.Runner`, `asyncio.Barrier`; `typing.Self`, `LiteralString`, `Never`, `assert_never`, `assert_type`, `reveal_type`, `Required`/`NotRequired`, `TypeVarTuple`/`Unpack`, `dataclass_transform`, `get_overloads`; `enum.StrEnum`, `ReprEnum`, `enum.verify`; `datetime.UTC`; `hashlib.file_digest()`; `operator.call()`; `contextlib.chdir()`; `math.cbrt()`/`math.exp2()`; `logging.getLevelNamesMapping()`; `dataclass(weakref_slot=True)`; atomic groups `(?>...)` and possessive quantifiers (`*+`, `++`) in `re`; `asyncio.TimeoutError` being an alias of the built-in `TimeoutError`.
- **3.12:** the `type X = ...` statement and `class C[T]:` / `def f[T]()` generic syntax (PEP 695); `typing.override`; `itertools.batched()`; f-strings that reuse the outer quote or contain backslashes/comments inside `{}` (PEP 701); `pathlib.Path.walk()`.
- **3.13:** `typing.TypeIs`, `ReadOnly`, `warnings.deprecated`, type-parameter defaults (PEP 696), `copy.replace()`.
- **3.14:** deferred annotation evaluation by default (PEP 649/749) and `annotationlib`; `except A, B:` without parentheses (PEP 758); template strings (`t"..."`); `map(..., strict=True)`.
- **3.15:** `lazy import` (PEP 810); the `frozendict` and `sentinel` built-ins (PEP 814/661); `*`-unpacking in comprehensions (PEP 798); `math.integer`; UTF-8 mode on by default (PEP 686).

When a newer feature has a third-party backport (e.g. `typing_extensions.Self`, `tomli` for `tomllib`, `exceptiongroup`, `async-timeout`), recommending it is acceptable only together with adding that dependency explicitly.

Global truths about Python 3.10:

- **Everything is an object.** Integers, functions, classes, modules — all are objects.
- **Indentation is syntax.** Blocks are delimited by indentation, not braces.
- **Dynamic typing.** Variables carry no declared type; annotations are not enforced at runtime.
- **Reference counting + cyclic GC.** CPython frees most objects immediately when their reference count drops to zero; a cyclic collector handles reference cycles.
- **The GIL is always on.** There is no free-threaded build of 3.10; threads do not run Python bytecode in parallel.
- **Dicts are ordered by insertion** (language guarantee since 3.7).
- **`match` and `case` are *soft* keywords.** They are keywords only at the start of a `match` statement or `case` clause; `match = re.match(...)` and a variable named `case` remain valid.
- **Annotations are still evaluated eagerly** unless the module uses `from __future__ import annotations`. PEP 563 was *not* made the default in 3.10 (it was postponed, and eventually superseded by PEP 649 in 3.14).
- **The default text encoding is still locale-dependent.** `open()` without `encoding=` uses the locale encoding (often cp1252 on Windows). 3.10 only adds an *opt-in* warning (PEP 597); UTF-8 mode only becomes the default in Python 3.15.

***

#### PEP 8 Code Style

##### Naming Conventions

```python
# Variables and functions: snake_case
user_name = 'Alice'
def calculate_total(items):
    pass

# Constants: SCREAMING_SNAKE_CASE (module level)
MAX_CONNECTIONS = 100
DEFAULT_TIMEOUT = 30.0

# Classes: PascalCase
class UserAccount:
    pass

# Internal use: single underscore prefix
class User:
    def __init__(self):
        self._internal_state = {}

# Name mangling: double underscore prefix (not true privacy)
class Base:
    def __init__(self):
        self.__private = 'name-mangled'  # accessible as _Base__private

# Module-level "private": excluded from __all__
_module_cache: dict[str, object] = {}
```

In Python 3, all classes implicitly inherit from `object`. `class Foo:` and `class Foo(object):` are identical — never write the latter.

**Naming and pattern matching:** because a bare name in a `case` clause is a *capture* pattern (see the pattern-matching section), constants you intend to match against must be reachable through a dotted name (`HTTPStatus.NOT_FOUND`, `config.MAX`) or an `Enum`. Keep module constants in an `Enum` or namespace class when they will be matched.

##### Import Ordering

Three blocks separated by blank lines: standard library, third-party, local. Alphabetize within each group. `isort --profile=black` or Ruff's `I` rules automate this.

```python
# Standard library
import os
import sys
from collections.abc import Callable, Iterator, Mapping   # ABCs ONLY from collections.abc (aliases removed in 3.10)
from pathlib import Path
from typing import Any, ParamSpec, TypeVar                # no Optional/Union needed in 3.10

# Third-party
import requests

# Local application
from myapp.models import User
from myapp.utils import format_date

# Never wildcard imports — they defeat static analysis and __all__ hygiene
# Bad:  from module import *
# Good: from module import specific_item
```

##### Docstrings

Write Google-style (or NumPy-style, consistently) docstrings for every public module, class, and function. First line: a one-sentence imperative summary.

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

Note the quoted `'Cache | None'`: the *whole* annotation is a string, so the forward reference is fine. Writing `cache: 'Cache' | None` (only one operand quoted) raises `TypeError` at definition time — see the PEP 604 section.

***

#### New in 3.10: Structural Pattern Matching (PEP 634, 635, 636)

The `match` statement compares a *subject* value against a sequence of *patterns*, top to bottom, and runs the block of the first `case` that matches. Patterns can destructure sequences, mappings and objects, bind names, and be combined with `|` and guards. If no case matches, the statement does nothing — there is no implicit error and no fall-through.

```python
def http_error(status: int) -> str:
    match status:
        case 400:
            return 'Bad request'
        case 401 | 403 | 404:          # OR-pattern
            return 'Not allowed'
        case 418:
            return "I'm a teapot"
        case _:                        # wildcard: always matches, binds nothing
            return 'Something else'

print(http_error(404))   # Not allowed
print(http_error(500))   # Something else
```

`match` is not a C-style `switch`: there is no jump table, cases are tried strictly in order, and there is no fall-through or `break`. Its value is **destructuring by shape and type**, not replacing short `if`/`elif` chains on a single value.

##### Pattern Kinds — Complete Reference

| Pattern | Example | Matches when | Binds |
|---|---|---|---|
| Literal | `case 0`, `case 'GET'`, `case -1`, `case 1+2j`, `case None` | `subject == literal`; `None`/`True`/`False` compared with `is` | nothing |
| Capture | `case x` | always | `x = subject` |
| Wildcard | `case _` | always | nothing (`_` is never bound) |
| Value | `case Color.RED`, `case http.HTTPStatus.OK` | `subject == value` (dotted name required) | nothing |
| Group | `case (p)` | `p` matches | as `p` |
| Sequence | `case [a, b]`, `case (a, b, *rest)` | subject is a sequence (not `str`/`bytes`/`bytearray`) of matching length | sub-patterns; `*rest` becomes a `list` |
| Mapping | `case {'id': i, **rest}` | subject is a mapping containing **at least** those keys | sub-patterns; `**rest` becomes a `dict` |
| Class | `case Point(x=0, y=y)`, `case int(n)`, `case str()` | `isinstance(subject, Point)` and attributes match | sub-patterns |
| OR | `case 'a' \| 'b'` | any alternative matches (left to right) | all alternatives must bind the same names |
| AS | `case [x, y] as pair` | inner pattern matches | the inner bindings plus `pair = subject` |

A `case` may add a **guard**: `case Point(x, y) if x == y:`. The guard runs only after the pattern matched and its names are bound; if the guard is false, matching continues with the next case.

##### Destructuring Sequences and Mappings

```python
def handle(command: str) -> str:
    match command.split():
        case []:
            return 'empty'
        case ['quit' | 'exit']:
            return 'bye'
        case ['go', direction]:
            return f'going {direction}'
        case ['drop', *objects]:               # *objects is always a list
            return f'dropping {", ".join(objects)}'
        case ['say', first, *_]:              # *_ ignores the remainder without copying it into a name
            return f'says {first}...'
        case _:
            return f'unknown: {command!r}'

print(handle('go north'))          # going north
print(handle('drop sword shield')) # dropping sword, shield
print(handle('exit'))              # bye
```

```python
def route(event: dict) -> str:
    match event:
        case {'type': 'click', 'pos': (x, y)}:
            return f'click at {x},{y}'
        case {'type': 'key', 'key': str(key)} if len(key) == 1:
            return f'key {key!r}'
        case {'type': 'resize', **rest}:
            return f'resize with {sorted(rest)}'
        case {'type': str(kind)}:
            return f'unhandled {kind}'
        case _:
            return 'not an event'

print(route({'type': 'click', 'pos': (3, 4), 'button': 1}))  # click at 3,4 — extra keys ignored
print(route({'type': 'key', 'key': 'q'}))                    # key 'q'
print(route({'type': 'resize', 'w': 80, 'h': 24}))           # resize with ['h', 'w']
```

Rules worth knowing for review:

- **Mapping patterns ignore extra keys**; sequence patterns do **not** (`[a, b]` only matches length 2). Use `**rest` / `*rest` deliberately. `**_` is a `SyntaxError` (it would be redundant).
- **Mapping patterns use `get()`-style lookup and never call `__missing__`**: matching a `defaultdict` does not insert keys.
- **Only real mappings and sequences match.** A mapping pattern requires a `collections.abc.Mapping` (dict, `OrderedDict`, `MappingProxyType`…); a sequence pattern requires a `collections.abc.Sequence` (`list`, `tuple`, `range`, `deque`, `memoryview`, `array.array`…).
- **`str`, `bytes` and `bytearray` never match sequence patterns** — deliberately, to prevent iterating characters by accident. Match them with `case str()`.
- **Iterators and generators never match sequence patterns** — materialize first: `match list(gen):`.
- Sequence patterns written with `[...]` and `(...)` are identical; `[a, b]` matches tuples too.

##### Class Patterns and `__match_args__`

```python
from dataclasses import dataclass

@dataclass
class Point:
    x: float
    y: float

def where(p: object) -> str:
    match p:
        case Point(x=0, y=0):
            return 'origin'
        case Point(0, y):                   # positional: uses Point.__match_args__ == ('x', 'y')
            return f'on y-axis at {y}'
        case Point(x, 0):
            return f'on x-axis at {x}'
        case Point(x, y) if x == y:
            return f'on diagonal at {x}'
        case Point():
            return 'somewhere else'
        case _:
            return 'not a point'

print(Point.__match_args__)   # ('x', 'y') — generated by @dataclass (also by namedtuple)
print(where(Point(0, 5)))     # on y-axis at 5
print(where(Point(2, 2)))     # on diagonal at 2
```

- **Keyword sub-patterns** (`Point(x=0)`) look the attribute up with `getattr` — properties and `__getattr__` run, with any side effects they have.
- **Positional sub-patterns** require `__match_args__`, a **tuple** of attribute names (a list is rejected). `@dataclass` and `collections.namedtuple`/`typing.NamedTuple` generate it; plain classes do not:

```python
class Plain:
    def __init__(self, x, y):
        self.x, self.y = x, y

try:
    match Plain(1, 2):
        case Plain(1, 2):
            pass
except TypeError as e:
    print(e)   # Plain() accepts 0 positional sub-patterns (2 given)

class WithArgs:
    __match_args__ = ('x', 'y')        # opt in to positional matching
    def __init__(self, x, y):
        self.x, self.y = x, y

match WithArgs(1, 2):
    case WithArgs(1, y):
        print('y =', y)                # y = 2
```

- **Built-in "self-matching" types** — `bool`, `bytearray`, `bytes`, `dict`, `float`, `frozenset`, `int`, `list`, `set`, `str`, `tuple` — accept one positional sub-pattern that matches the *whole subject*: `case int(n)` means "is an int, bind it to `n`", `case str() as s` is equivalent to `case str(s)`.
- The name before `(...)` must evaluate to a class; `case len():` raises `TypeError: called match pattern must be a type` at match time.

##### Pitfalls — the Bugs Reviewers Must Catch

**Pitfall 1: a bare name is a capture, not a comparison.** This is the single most common pattern-matching bug:

```python
NOT_FOUND = 404

def describe(status: int) -> str:
    match status:
        case 200:
            return 'ok'
        case NOT_FOUND:            # ❌ captures ANY value into a new local named NOT_FOUND
            return 'not found'
    return 'other'

print(describe(500))   # 'not found' — wrong!
```

The compiler only rejects the mistake when it is *not* the last case (`SyntaxError: name capture 'NOT_FOUND' makes remaining patterns unreachable`). As the last case it compiles silently, and at module or class level it even **rebinds the global constant**:

```python
NOT_FOUND = 404
match 500:
    case 200:
        pass
    case NOT_FOUND:     # ❌ module-level capture rebinds the global
        pass
print(NOT_FOUND)        # 500
```

```python
from http import HTTPStatus

def describe(status: int) -> str:
    match status:
        case HTTPStatus.OK:            # ✅ dotted name → value pattern (compared with ==)
            return 'ok'
        case HTTPStatus.NOT_FOUND:     # ✅ IntEnum member == 404
            return 'not found'
        case code if code == LIMIT:    # ✅ guard for non-dotted constants
            return 'limit'
        case _:
            return 'other'

LIMIT = 429
print(describe(404), describe(429), describe(500))   # not found limit other
```

**Pitfall 2: `case int | str:` is not a type test.** `int` and `str` are bare names → capture patterns → `SyntaxError` (or a silent capture if it is a single name). Type tests need parentheses:

```python
def kind(value: object) -> str:
    match value:
        case bool():                   # ✅ before int() — bool is a subclass of int
            return 'bool'
        case int() | float():          # ✅ class patterns
            return 'number'
        case str() | bytes():
            return 'text'
        case _:
            return 'other'

print(kind(True), kind(3), kind(2.5), kind('x'))   # bool number number text
```

**Pitfall 3: literal patterns use `==` (except `None`/`True`/`False`).** `case 1:` matches `True` and `1.0`; `case True:` does *not* match `1` (identity). Order `bool` cases first and use `case bool()` / `case True` when the distinction matters:

```python
match True:
    case 1:
        print('True matched case 1')   # printed — True == 1
match 1.0:
    case 1:
        print('1.0 matched case 1')    # printed — 1.0 == 1
match 1:
    case True:
        print('never printed')         # True is compared with `is`
    case _:
        print('1 is not True')
```

**Pitfall 4: `case float('nan')` never matches.** It is a class pattern (float self-match against the *string* `'nan'`), and NaN is unequal to itself anyway. Use a guard:

```python
import math

def classify(x: float) -> str:
    match x:
        case float() if math.isnan(x):   # ✅
            return 'nan'
        case 0.0:
            return 'zero'
        case _:
            return 'number'

print(classify(float('nan')), classify(0.0), classify(-0.0))   # nan zero zero
```

**Pitfall 5: no exhaustiveness at runtime.** A `match` without a matching case silently does nothing. When every value must be handled, end with an explicit `case _: raise`. (`typing.assert_never` is **3.11+**; on 3.10 raise directly, or use `typing_extensions.assert_never` if that dependency is present.)

```python
from enum import Enum

class Shape(Enum):
    CIRCLE = 'circle'
    SQUARE = 'square'

def sides(shape: Shape) -> int:
    match shape:
        case Shape.CIRCLE:
            return 0
        case Shape.SQUARE:
            return 4
        case _:
            raise ValueError(f'unhandled shape: {shape!r}')   # ✅ fail loudly
```

**Pitfall 6: names bound by a failed case may stay bound.** A guard runs *after* binding, so a case whose guard fails leaves its captures assigned; do not read capture names after the `match` unless the case that binds them is the one that ran:

```python
y = 'unset'
match (1, 2):
    case (x, y) if x > 5:      # pattern matched, guard failed
        pass
    case _:
        pass
print(y)    # 2 — rebound by the failed case
```

**Pitfall 7: OR-alternatives must bind the same names, and names may not repeat.** `case [a] | (a, b):` → `SyntaxError: alternative patterns bind different names`; `case [a, a]:` → `SyntaxError: multiple assignments to name 'a' in pattern` (it is *not* an equality test — use a guard). An irrefutable case (`case _:` or `case name:`) anywhere but last is a `SyntaxError`.

**Pitfall 8: patterns are not expressions.** No function calls, arithmetic, f-strings or subscripts: `case 1+2:` and `case f'x':` are `SyntaxError`; `case -1`, `case 1+2j` and implicitly concatenated strings (`case 'a' 'b'`) are allowed. Mapping keys must be literals or dotted values (`case {k: 1}` is a `SyntaxError`).

##### When to Use (and Not Use) `match`

```python
# ✅ Good fit: heterogeneous, nested data — AST nodes, JSON messages, command tuples
def evaluate(node):
    match node:
        case ('num', value):
            return value
        case ('add', left, right):
            return evaluate(left) + evaluate(right)
        case ('mul', left, right):
            return evaluate(left) * evaluate(right)
        case ('neg', operand):
            return -evaluate(operand)
        case _:
            raise ValueError(f'bad node: {node!r}')

print(evaluate(('add', ('num', 2), ('mul', ('num', 3), ('num', 4)))))   # 14

# ❌ Poor fit: a lookup table dressed up as match — use a dict
def color_code_bad(name: str) -> str:
    match name:
        case 'red':
            return '#f00'
        case 'green':
            return '#0f0'
        case _:
            return '#000'

COLOR_CODES = {'red': '#f00', 'green': '#0f0'}

def color_code(name: str) -> str:   # ✅ O(1), data instead of code
    return COLOR_CODES.get(name, '#000')
```

Review guidance:

- `match` is **linear**: each case is tried in order; there is no hashing or jump table. For hot paths dispatching on one value among many constants, a `dict` is faster.
- Do not use `match` as an `isinstance` chain over *your own* class hierarchy when a method (polymorphism) or `functools.singledispatch` expresses it better.
- Pattern matching on untrusted JSON is fine and idiomatic — but remember mapping patterns ignore unknown keys, so validate rather than assume a schema.
- Libraries that must still support 3.9 cannot use `match` at all (it is a `SyntaxError` there) — check `requires-python` before suggesting it.

***

#### New in 3.10: Union Types as `X | Y` (PEP 604)

`X | Y` now builds a union type at runtime. It replaces `typing.Union[X, Y]` and `typing.Optional[X]` in annotations, and it is accepted by `isinstance()` and `issubclass()` (since 3.10 `isinstance()` also accepts `typing.Union[int, str]`; on 3.9 that raised `TypeError`).

```python
# Python 3.9
from typing import Optional, Union
def parse_old(raw: Union[str, bytes], default: Optional[int] = None) -> Union[int, None]: ...

# ✅ Python 3.10
def parse(raw: str | bytes, default: int | None = None) -> int | None:
    ...

print(isinstance(3, int | str))          # True
print(issubclass(bool, int | str))       # True
print(int | None)                        # int | None
print(type(int | None))                  # <class 'types.UnionType'>
```

`int | None == typing.Optional[int]` and `int | str == typing.Union[int, str]` are both `True`, so mixing styles is harmless for equality — but **not** for introspection:

```python
import types
import typing

print(typing.get_origin(int | str))             # <class 'types.UnionType'>
print(typing.get_origin(typing.Union[int, str]))  # typing.Union

# ✅ Introspection code (validators, serializers, DI containers) must accept both on 3.10:
def is_union(tp: object) -> bool:
    return typing.get_origin(tp) in (typing.Union, types.UnionType)

print(is_union(int | None), is_union(typing.Optional[int]))   # True True
```

Mixing the operator with `typing` objects produces a `typing.Union`, not a `types.UnionType`: `int | typing.Optional[str]` is `typing.Union[int, str, NoneType]` and `typing.List[int] | None` is `typing.Optional[typing.List[int]]`. Code that checks `isinstance(tp, types.UnionType)` misses these — use `get_origin` as above. (In 3.14 the two union types were unified; on 3.10 they are distinct.)

##### PEP 604 Pitfalls

**Pitfall: a string operand is a `TypeError` at definition time.** `|` is evaluated eagerly like any expression:

```python
class Node:
    # ❌ def link(self, other: 'Node' | None) -> None: ...
    #    TypeError: unsupported operand type(s) for |: 'str' and 'NoneType'

    # ✅ quote the whole annotation
    def link(self, other: 'Node | None') -> None:
        self.next = other
```

Alternatively use `from __future__ import annotations` (every annotation becomes a string and nothing is evaluated at definition time). Prefer that over sprinkling quotes in modules with many forward references.

**Pitfall: parameterized generics are not allowed in `isinstance()`**, inside a union or not:

```python
value = [1, 2]
# ❌ isinstance(value, list[int] | None)
#    TypeError: isinstance() argument 2 cannot contain a parameterized generic
print(isinstance(value, list | None))   # ✅ True — check the bare type, validate items separately
```

**Pitfall: runtime-typed libraries.** Pydantic, FastAPI, attrs converters, `typing.get_type_hints()` and `dataclasses` *evaluate* annotations. On 3.10 `int | None` works for all of them; the problem is **code that must still run on 3.9**, where the same annotation raises `TypeError` the moment the module is imported (or when the library evaluates the stringified annotation). Check the project's `requires-python` before converting `Optional[X]` to `X | None`.

**Equivalent spellings (all valid in 3.10):**

| Old | 3.10 |
|---|---|
| `Optional[int]` | `int \| None` |
| `Union[int, str]` | `int \| str` |
| `Union[int, str, None]` | `int \| str \| None` |
| `Optional[Union[int, str]]` | `int \| str \| None` |
| `isinstance(x, (int, str))` | `isinstance(x, int \| str)` (the tuple form is still fine) |
| `Optional['Node']` | `'Node \| None'` or `Node \| None` with the `__future__` import |

`None | int` also works (`NoneType` supports the reflected operator), and `T | None` works for a `TypeVar`.

***

#### New in 3.10: Parenthesized Context Managers

A `with` statement may now wrap several context managers in parentheses, one per line, with a trailing comma — no backslashes, no `ExitStack` needed for a fixed set of managers.

```python
import tempfile
from pathlib import Path

tmp = Path(tempfile.mkdtemp())
(tmp / 'input.txt').write_text('a\nb\n', encoding='utf-8')

# ✅ Officially supported from 3.10
with (
    open(tmp / 'input.txt', encoding='utf-8') as fin,
    open(tmp / 'output.txt', 'w', encoding='utf-8') as fout,
):
    for line in fin:
        fout.write(line.upper())

print(repr((tmp / 'output.txt').read_text(encoding='utf-8')))   # 'A\nB\n'
```

- Managers are still entered left to right and exited in reverse order; if entering the second one fails, the first is exited properly.
- For a *variable* number of resources, `contextlib.ExitStack` remains the right tool.
- The parentheses are grammar, not a tuple: `with (lock_a, lock_b):` enters *both* locks.
- CPython 3.9's PEG parser happened to accept this syntax, but it only became official in 3.10. Code that must run on 3.9 (or be parsed by older tools) should keep the backslash form.

***

#### New in 3.10: Better Error Messages

3.10 rewrote many parser and runtime messages. They do not change behavior, but they change *text* — tests that assert on exact `SyntaxError`/`NameError` messages may need updating.

```python
cases = [
    "expected = {9: 1, 18: 2,\nother = foo()\n",
    "if x = 3:\n    pass\n",
    "def f():\n    if x:\n    y = 2\n",
    "try:\n    x = 2\ny = 3\n",
    "items = {'a': 1 'b': 2}\n",
    "f(a, b for b in c)\n",
    "s = 'abc\n",
]
for src in cases:
    try:
        compile(src, 'example.py', 'exec')
    except SyntaxError as e:
        print(f'{type(e).__name__}: {e.msg} (line {e.lineno})')

# SyntaxError: '{' was never closed (line 1)
# SyntaxError: invalid syntax. Maybe you meant '==' or ':=' instead of '='? (line 1)
# IndentationError: expected an indented block after 'if' statement on line 2 (line 3)
# SyntaxError: expected 'except' or 'finally' block (line 3)
# SyntaxError: invalid syntax. Perhaps you forgot a comma? (line 1)
# SyntaxError: Generator expression must be parenthesized (line 1)
# SyntaxError: unterminated string literal (detected at line 1) (line 1)
```

`SyntaxError` now also has `end_lineno` and `end_offset`, and the interpreter underlines the whole offending range.

**"Did you mean" suggestions** for `NameError` and `AttributeError` (e.g. `NameError: name 'schwarschild' is not defined. Did you mean: 'schwarzschild'?`) are added **only by the interpreter's default exception display** (`sys.excepthook`). They are **not** part of `str(exc)`, and in 3.10 (and 3.11) **not** produced by the `traceback` module — so `logging.exception()`, Sentry-style reporters, IPython and custom excepthooks do not show them. (The `traceback` module gained them in 3.12.)

```python
import collections
import traceback

try:
    collections.namedtoplo
except AttributeError as e:
    print(str(e))                                   # module 'collections' has no attribute 'namedtoplo'
    print(traceback.format_exc().splitlines()[-1])  # same — no suggestion in 3.10
    print(e.name, e.obj is collections)             # namedtoplo True — new attributes in 3.10
```

`AttributeError` gained `.name` and `.obj`, and `NameError` gained `.name` (both populated by the interpreter), which makes structured error reporting possible without parsing messages.

***

#### New in 3.10: `zip(strict=True)` (PEP 618)

`zip()` silently truncates to the shortest input — a classic source of lost data. `strict=True` raises `ValueError` when the iterables have different lengths.

```python
names = ['alice', 'bob', 'carol']
scores = [90, 85]

print(list(zip(names, scores)))     # [('alice', 90), ('bob', 85)] — carol silently dropped

try:
    list(zip(names, scores, strict=True))
except ValueError as e:
    print(e)                        # zip() argument 2 is shorter than argument 1

try:
    list(zip(scores, names, strict=True))
except ValueError as e:
    print(e)                        # zip() argument 2 is longer than argument 1
```

**Pitfall: the check happens at exhaustion, not up front.** Pairs before the mismatch have already been yielded (and possibly written to a database) when the error is raised:

```python
rows = []
try:
    for pair in zip([1, 2, 3], 'ab', strict=True):
        rows.append(pair)            # side effects already happened
except ValueError:
    print('partial:', rows)          # partial: [(1, 'a'), (2, 'b')]
```

When the inputs are sized collections and side effects matter, check `len()` first. Use `strict=True` whenever the iterables are *supposed* to be the same length (parallel arrays, headers and values, keys and results); use `itertools.zip_longest()` when they legitimately differ. `map()` has no `strict` parameter until 3.14.

***

#### Primitive Types and Numerics

##### Integers

`int` is arbitrary precision; there is no overflow and no separate `long`.

```python
import sys
print(sys.maxsize)       # 9223372036854775807 on 64-bit (C ssize_t max, NOT the int limit)
x = 10 ** 100
print(type(x))           # <class 'int'>

one_million = 1_000_000
mask = 0xFF_FF_FF_FF

# New in 3.10: int.bit_count() — population count
print((255).bit_count())       # 8
print((-7).bit_count())        # 3 — counts bits of the absolute value
print(bin(0b1011).count('1'))  # 3 — the slower pre-3.10 idiom
```

##### Security Change in 3.10.7: Integer/String Conversion Limit

Since **3.10.7**, converting between `int` and `str` in a non-power-of-two base (i.e. decimal) raises `ValueError` once the number has more than **4300 digits** — a mitigation for CVE-2020-10735 (quadratic-time conversion as a denial-of-service vector). `hex()`, `oct()`, `bin()` and `int(s, 16)` are not limited.

```python
import sys

try:
    int('1' * 5000)
except ValueError as e:
    print(e)   # Exceeds the limit (4300) for integer string conversion: value has 5000 digits; use sys.set_int_max_str_digits() to increase the limit

print(sys.get_int_max_str_digits())   # 4300
print(len(hex(10 ** 5000)))           # 4155 — hex is unaffected

# Raise the limit only where you genuinely need huge decimal ints, and keep untrusted input bounded:
sys.set_int_max_str_digits(10_000)    # process-wide; 0 disables the limit (never do that for untrusted input)
print(len(str(10 ** 5000)))           # 5001
sys.set_int_max_str_digits(4300)
```

The limit can also be set with `-X int_max_str_digits=N` or `PYTHONINTMAXSTRDIGITS`. Code that worked on 3.10.0–3.10.6 with very large decimal numbers (bignum math, cryptography test vectors, `json.loads` of huge numbers) breaks on later 3.10 patches — this is a point-release behavior change.

##### Floats

```python
import math

print(math.isclose(0.1 + 0.2, 0.3))   # True — never compare floats with ==

# 3.10: NaN hashes are identity-based. Distinct NaN objects no longer all hash to 0,
# which used to make dicts/sets with many NaN keys quadratic.
a, b = float('nan'), float('nan')
print(hash(a) == hash(b))   # False
print(hash(a) == hash(a))   # True
print(a in {a})             # True — identity check short-circuits
print(b in {a})             # False — NaN != NaN, and the hashes now differ
```

##### Booleans

`bool` is a subclass of `int` (`True == 1`, `isinstance(True, int)`). Remember this in pattern matching (`case 1:` matches `True`) and in `case int():` (matches `True` — put `case bool():` first).

**Falsy values:** `None`, `False`, numeric zero (`0`, `0.0`, `0j`, `Decimal(0)`), empty sequences and collections, and objects whose `__bool__` returns `False` or `__len__` returns `0`.

##### Deprecated in 3.10: Numeric Literals Glued to Keywords

`0in x`, `1if c else 2`, `[0x1for x in y]` used to be accepted silently. 3.10 emits a `DeprecationWarning` when compiling them (a `SyntaxWarning` from 3.11, planned to become an error); under `-W error` the warning turns into a `SyntaxError`, so such a module fails to import in a strict test run:

```python
print(eval('[0x1for x in (1, 2)]'))   # [31] — parsed as [0x1f or x in (1, 2)]!
```

Always put a space between a number and a following keyword.

***

#### Strings

##### `str` vs `bytes`

- **`str`** is Unicode text; **`bytes`** is binary. They never mix implicitly (`'a' + b'a'` is a `TypeError`).
- Decode at input boundaries, work in `str`, encode at output boundaries.
- Always pass `encoding=` to `open()`; see the EncodingWarning section below for how 3.10 helps you find omissions.

##### String Methods (Reference)

```python
s = 'Hello, World!'

s.upper(); s.lower(); s.casefold()          # casefold() for case-insensitive comparison
s.strip(); s.lstrip('H'); s.rstrip('!')     # strip CHARACTER SETS, not substrings
s.removeprefix('Hello, ')                   # 'World!' — exact prefix, once (3.9+)
s.removesuffix('!')                         # 'Hello, World'
s.find('World'); s.index('World')           # 7, 7 — index() raises ValueError if absent
s.startswith(('Hello', 'Hi'))               # tuple of prefixes
s.split(', '); s.rsplit(', ', maxsplit=1); s.partition(', ')
', '.join(['a', 'b', 'c'])
s.replace('World', 'Python')
'42'.zfill(6); 'hi'.center(10, '-')
'abc'.isalpha(); '123'.isdigit(); '½'.isnumeric()
'café'.encode('utf-8'); b'caf\xc3\xa9'.decode('utf-8')

url = 'https://example.com'
print(url.lstrip('https://'))        # ❌ 'example.com' only by luck — strips any of h,t,p,s,:,/
print(url.removeprefix('https://'))  # ✅ 'example.com'
```

##### f-strings and Formatting

```python
name, score = 'Alice', 98.765
print(f'{name!r} scored {score:.2f}')   # 'Alice' scored 98.77
print(f'{1_000_000:_}')                  # 1_000_000
print(f'{score = :.1f}')                 # score = 98.8 — "=" specifier (3.8+), debugging only

d = {'key': 'value'}
print(f"Value: {d['key']}")              # ✅ switch the outer quote type
nl = '\n'
print(f'{nl.join(["a", "b"])}')          # ✅ hoist backslashes into a variable
# ❌ f'{d['key']}'          — SyntaxError in 3.10 (reusing the outer quote is allowed only from 3.12)
# ❌ f'{"\n".join(items)}'  — SyntaxError in 3.10 (backslash inside {} allowed only from 3.12)
```

3.10's parser also reports `f"{*items}"` as `SyntaxError: f-string: cannot use starred expression here`.

***

#### Variable Annotations and Type Hints

##### 3.10 Typing at a Glance

```python
from collections.abc import Callable, Iterable, Mapping   # ABCs and their generics
from typing import Any, Concatenate, ParamSpec, TypeAlias, TypeGuard, TypeVar

T = TypeVar('T')
P = ParamSpec('P')

JsonScalar: TypeAlias = str | int | float | bool | None
Json: TypeAlias = 'JsonScalar | list[Json] | dict[str, Json]'   # recursive alias must be a string

def first(items: Iterable[T], default: T | None = None) -> T | None:
    for item in items:
        return item
    return default

def index(rows: Iterable[Mapping[str, Any]], key: str) -> dict[Any, Mapping[str, Any]]:
    return {row[key]: row for row in rows}

print(first([3, 4]), first([], default=0))   # 3 0
```

- Use **built-in generics** (`list[int]`, `dict[str, int]`, 3.9+) and **`collections.abc`** for abstract types (`Iterable`, `Mapping`, `Callable`). `typing.List`, `typing.Dict`, `typing.Callable` etc. are deprecated aliases (no runtime warning).
- Use **`X | None`**, not `Optional[X]`, in code that only targets 3.10+.
- Still from `typing`: `Any`, `TypeVar`, `Generic`, `Protocol`, `TypedDict`, `NamedTuple`, `Literal`, `Final`, `ClassVar`, `Annotated`, `NewType`, `NoReturn`, `overload`, `final`, `cast`, `get_type_hints`, and the new `ParamSpec`, `Concatenate`, `TypeAlias`, `TypeGuard`, `is_typeddict`.

##### New in 3.10: `ParamSpec` and `Concatenate` (PEP 612)

Before 3.10, a decorator that preserved its wrapped function's signature could only be typed as `Callable[..., R]`, erasing all parameter information. `ParamSpec` captures the full parameter list.

```python
import functools
import logging
from collections.abc import Callable
from typing import ParamSpec, TypeVar

P = ParamSpec('P')
R = TypeVar('R')
logger = logging.getLogger(__name__)

def logged(func: Callable[P, R]) -> Callable[P, R]:
    @functools.wraps(func)
    def wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
        logger.debug('calling %s', func.__qualname__)
        return func(*args, **kwargs)
    return wrapper

@logged
def add(a: int, b: int = 0) -> int:
    return a + b

print(add(1, b=2))   # 3 — and a type checker still knows add(a: int, b: int = 0) -> int
```

`Concatenate` expresses decorators that **add or remove** leading parameters — e.g. injecting a connection:

```python
import sqlite3
from collections.abc import Callable
from typing import Concatenate, ParamSpec, TypeVar

P = ParamSpec('P')
R = TypeVar('R')

def with_connection(
    func: Callable[Concatenate[sqlite3.Connection, P], R],
) -> Callable[P, R]:
    def wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
        conn = sqlite3.connect(':memory:')
        try:
            return func(conn, *args, **kwargs)
        finally:
            conn.close()
    return wrapper

@with_connection
def count_rows(conn: sqlite3.Connection, table: str) -> int:
    conn.execute(f'CREATE TABLE {table} (id INTEGER)')   # demo only — never format SQL from input
    return conn.execute(f'SELECT COUNT(*) FROM {table}').fetchone()[0]

print(count_rows('users'))   # 0 — callers no longer pass conn
```

Rules a type checker enforces (and reviewers should too):

- `*args: P.args` and `**kwargs: P.kwargs` must appear **together**, and only in a function whose signature is tied to `P` (e.g. a wrapper inside a function that received `Callable[P, R]`).
- `Concatenate[...]` must end with the `ParamSpec` and is only valid as the first argument of `Callable[...]`.
- Generic classes can be parameterized by a `ParamSpec`: `class Task(Generic[P, R])`.
- `ParamSpec` exists only for type checkers; at runtime it does no validation. Several runtime substitution bugs were fixed in **3.10.3** (bpo-44791, bpo-46581) — another reason to run the latest patch.
- `TypeVarTuple`/`Unpack` (variadic generics) are **3.11**, and the `def f[**P]` syntax is **3.12** — not available here.

##### New in 3.10: Explicit Type Aliases — `TypeAlias` (PEP 613)

```python
from typing import TypeAlias

UserId: TypeAlias = int                         # explicit alias, not a variable
Headers: TypeAlias = dict[str, list[str]]
Callback: TypeAlias = 'Callable[[Headers], None]'   # forward-referenced alias, as a string

DEFAULT_HEADERS = {'accept': ['*/*']}          # a module constant — NOT an alias
```

`TypeAlias` makes intent explicit and lets type checkers accept string forward references in aliases. It is only an annotation; at runtime `UserId` is simply `int`. The `type UserId = int` statement (PEP 695) is **3.12+** — never suggest it for 3.10 code. To create a *distinct* type, use `typing.NewType`, not an alias.

##### New in 3.10: User-Defined Type Guards — `TypeGuard` (PEP 647)

```python
from typing import TypeGuard

def is_str_list(values: list[object]) -> TypeGuard[list[str]]:
    return all(isinstance(v, str) for v in values)

def join_all(values: list[object]) -> str:
    if is_str_list(values):
        return ', '.join(values)   # type checker: values is list[str] here
    return ''

print(join_all(['a', 'b']), repr(join_all([1, 'b'])))   # a, b ''
```

Semantics to keep in mind:

- A `TypeGuard` function narrows **only in the positive branch**. In the `else` branch the type is *not* narrowed (that two-way behavior is `typing.TypeIs`, **3.13+**).
- The narrowed type need not be a subtype of the input (`list[object]` → `list[str]` is allowed), so a guard can lie — the checker trusts it. Make guards check everything they claim (here: *every* element).
- At runtime it is just a function returning `bool`.

##### Postponed Evaluation (`from __future__ import annotations`) in 3.10

PEP 563 was scheduled to become the default in 3.10, but the Steering Council postponed it (and later superseded it by PEP 649/749, which landed in 3.14). In 3.10 the future import still changes behavior **per module**:

- Without it, annotations are evaluated at definition time (so forward references need quotes).
- With it, every annotation is stored as a string; `typing.get_type_hints()` or `inspect.get_annotations(obj, eval_str=True)` evaluates them on demand.
- With it, annotations containing `yield`, `await` or `:=` are now a `SyntaxError`, and annotations on complex targets (`self.x: int = 0`) have no runtime effect.

```python
from __future__ import annotations   # must be the first statement in the module

import dataclasses
import inspect
import typing

@dataclasses.dataclass
class Config:
    host: str
    port: int | None = None

print(Config.__annotations__)                 # {'host': 'str', 'port': 'int | None'} — strings
print(typing.get_type_hints(Config))          # {'host': <class 'str'>, 'port': int | None}
print(inspect.get_annotations(Config, eval_str=True))
```

Rule of thumb for 3.10: the future import is a reasonable choice for modules with many forward references or heavy annotation cost, but libraries that resolve annotations at runtime (pydantic, FastAPI, typer, dependency injectors) need every name used in an annotation to be importable at module level — names imported under `if TYPE_CHECKING:` will fail when they evaluate the strings.

##### New in 3.10: `inspect.get_annotations()` — Reading Annotations Safely

```python
import inspect

class Base:
    a: int

class Derived(Base):
    pass

# 3.10: annotation dicts on classes/modules are created lazily per class.
print(Derived.__annotations__)                 # {} — in 3.9 this returned Base's dict by inheritance!
print(inspect.get_annotations(Derived))        # {}
print(inspect.get_annotations(Base))           # {'a': <class 'int'>}

def f(x: 'int') -> 'str': ...
print(inspect.get_annotations(f))                  # {'x': 'int', 'return': 'str'}
print(inspect.get_annotations(f, eval_str=True))   # {'x': <class 'int'>, 'return': <class 'str'>}
```

`inspect.get_annotations()` is the 3.10 best practice for reading an object's own annotations; use `typing.get_type_hints()` when you need the merged class hierarchy (MRO) with forward references resolved. Never read `cls.__dict__['__annotations__']` or `cls.__annotations__` directly in framework code.

##### Other 3.10 Typing Changes

- **`typing.Literal`** de-duplicates parameters, compares order-independently and type-strictly: `Literal[1, 1, 2] == Literal[2, 1]` is `True`, `Literal[0] == Literal[False]` is `False` (listed in What's New 3.10, but already backported to 3.9.1).
- **`typing.Protocol`**: `isinstance()` against a protocol that declares only data members and is not `@runtime_checkable` now raises `TypeError` (it previously passed silently).
- **`typing.is_typeddict(tp)`** — new runtime check for `TypedDict` classes.
- **`collections.abc.Callable[[int, str], str].__args__`** is flattened to `(int, str, str)`, matching `typing.Callable` (listed in What's New 3.10, but already backported to 3.9.2, so only code coming from 3.9.0/3.9.1 sees a change); code reading `__args__` directly must handle it.
- **`types.NoneType`, `types.EllipsisType`, `types.NotImplementedType`** are back as public names (`types.NoneType` is handy for `isinstance` and `singledispatch` registration).

```python
from typing import Literal, Protocol, runtime_checkable

class HasName(Protocol):
    name: str

try:
    isinstance(object(), HasName)
except TypeError as e:
    print(e)   # Instance and class checks can only be used with @runtime_checkable protocols

@runtime_checkable
class Named(Protocol):
    name: str

class User:
    name = 'ann'

print(isinstance(User(), Named))                                # True — checks attribute presence only
print(Literal[1, 1, 2] == Literal[2, 1], Literal[0] == Literal[False])   # True False
```

##### Typing Features NOT in 3.10

| Feature | Available from | 3.10 alternative |
|---|---|---|
| `typing.Self` | 3.11 | a bound `TypeVar` (`TSelf = TypeVar('TSelf', bound='Base')`) or `typing_extensions.Self` |
| `typing.Never`, `assert_never` | 3.11 | `NoReturn`; `raise` in the fall-through branch |
| `Required[...]`/`NotRequired[...]` in `TypedDict` | 3.11 | split into a `total=True` base and a `total=False` subclass |
| `LiteralString`, `TypeVarTuple`, `Unpack`, `dataclass_transform`, `reveal_type`, `assert_type` | 3.11 | `typing_extensions` (checker-only) |
| Generic `TypedDict`/`NamedTuple` | 3.11 | non-generic definitions |
| `type X = ...`, `class C[T]:` | 3.12 | `X: TypeAlias = ...`, `TypeVar` + `Generic[T]` |
| `@typing.override` | 3.12 | `typing_extensions.override` |
| `TypeIs`, `ReadOnly`, TypeVar defaults | 3.13 | `TypeGuard`; `typing_extensions` |

```python
from typing import TypeVar

TShape = TypeVar('TShape', bound='Shape')

class Shape:
    def __init__(self) -> None:
        self.scale = 1.0

    def scaled(self: TShape, factor: float) -> TShape:   # ✅ 3.10 spelling of "-> Self"
        self.scale *= factor
        return self

class Circle(Shape):
    pass

c: Circle = Circle().scaled(2.0)   # checker infers Circle, not Shape
print(type(c).__name__, c.scale)   # Circle 2.0
```

***

#### Lists

```python
lst: list[int] = [1, 2, 3, 4, 5]

print(lst[0], lst[-1], lst[1:3], lst[::-1])   # 1 5 [2, 3] [5, 4, 3, 2, 1]

lst.append(6)
lst.extend([7, 8])
lst.insert(0, 0)
lst.remove(3)            # first occurrence; ValueError if absent
last = lst.pop()         # O(1)
first = lst.pop(0)       # O(n) — use collections.deque for queues

nums = [3, 1, 4, 1, 5, 9]
nums.sort()                          # in place, stable
nums.sort(key=lambda n: -n)          # descending
ordered = sorted(nums)               # new list

squares = [n * n for n in range(10)]
evens = [n for n in range(20) if n % 2 == 0]
flat = [x for row in [[1, 2], [3, 4]] for x in row]   # [1, 2, 3, 4]
```

**Pitfall: list multiplication shares references.**

```python
matrix = [[0] * 3] * 3          # ❌ three references to ONE row
matrix[0][0] = 99
print(matrix)                   # [[99, 0, 0], [99, 0, 0], [99, 0, 0]]

matrix = [[0] * 3 for _ in range(3)]   # ✅
matrix[0][0] = 99
print(matrix)                   # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

##### New in 3.10: `bisect` with `key=`

`bisect.bisect_left/bisect_right/insort_left/insort_right` accept a `key` function — no more parallel key lists.

```python
import bisect

events = [(1, 'boot'), (5, 'login'), (9, 'logout')]   # sorted by timestamp

# key= is applied to the LIST ELEMENTS; the search value is already a key
idx = bisect.bisect_left(events, 5, key=lambda e: e[0])
print(idx)                                            # 1

# insort with key=: the inserted ITEM is passed through key
bisect.insort(events, (7, 'idle'), key=lambda e: e[0])
print(events)   # [(1, 'boot'), (5, 'login'), (7, 'idle'), (9, 'logout')]
```

**Pitfall:** `bisect_*` with `key=` expects the search value to be a *key* (`5`), while `insort_*` expects a full *item* (`(7, 'idle')`). Passing a full tuple to `bisect_left(..., key=...)` compares a tuple to ints and raises `TypeError`.

***

#### Tuples

```python
point = (1, 2)
single = (42,)                  # the comma makes the tuple
a, b = 1, 2
a, b = b, a                     # swap without a temporary
head, *middle, tail = [1, 2, 3, 4, 5]
print(head, middle, tail)       # 1 [2, 3, 4] 5

from typing import NamedTuple

class Coord(NamedTuple):
    lat: float
    lon: float
    label: str = ''

c = Coord(52.5, 13.4, 'Berlin')
print(c.lat, c._asdict())        # 52.5 {'lat': 52.5, 'lon': 13.4, 'label': 'Berlin'}
print(Coord.__match_args__)      # ('lat', 'lon', 'label') — usable in class patterns
```

***

#### Dictionaries

```python
d: dict[str, int] = {'a': 1, 'b': 2}

print(d.get('c', 0))            # 0
print(d.setdefault('d', 99))    # 99 — inserts and returns
merged = d | {'e': 5}           # 3.9+: new dict, right side wins
d |= {'f': 6}                   # 3.9+: in-place update

for key, value in d.items():
    pass
for key in reversed(d):         # 3.8+
    pass

inverted = {v: k for k, v in d.items()}

# New in 3.10: dict views expose the underlying dict read-only via .mapping
keys = d.keys()
print(repr(keys.mapping))           # mappingproxy({'a': 1, 'b': 2, 'd': 99, 'f': 6})
print(type(keys.mapping).__name__)  # mappingproxy
```

**Pitfall: never mutate a dict while iterating it** (`RuntimeError: dictionary changed size during iteration`). Rebuild instead: `d = {k: v for k, v in d.items() if keep(k)}` or iterate over `list(d)`.

##### `collections`

```python
from collections import ChainMap, Counter, OrderedDict, defaultdict, deque

word_count: defaultdict[str, int] = defaultdict(int)
for word in 'to be or not to be'.split():
    word_count[word] += 1

c = Counter('abracadabra')
print(c.most_common(2))          # [('a', 5), ('b', 2)]
print(c.total())                 # 11 — Counter.total() is new in 3.10

settings = ChainMap({'color': 'red'}, {'color': 'blue', 'user': 'guest'})
print(settings['color'], settings['user'])   # red guest

q: deque[str] = deque(maxlen=1000)
q.append('job')
q.appendleft('urgent')
print(q.popleft())               # urgent
```

`Counter.total()` replaces `sum(c.values())` (new in 3.10). The ABCs (`Mapping`, `Sequence`, `Callable`…) are **no longer importable from `collections`** — see the migration section.

***

#### Sets

```python
s: set[int] = {1, 2, 3}
empty = set()                  # {} is an empty dict
s.add(4)
s.discard(10)                  # no error if absent
a, b = {1, 2, 3, 4}, {3, 4, 5}
print(a | b, a & b, a - b, a ^ b)   # {1, 2, 3, 4, 5} {3, 4} {1, 2} {1, 2, 5}
frozen = frozenset(a)          # hashable — usable as dict key

# 3.10 officially allows an unparenthesized walrus inside set literals/comprehensions
# (3.9's PEG parser already accepted it); in sequence indexes it is new in 3.10
data = [1, 2, 3]
print(sorted({total := sum(data), total * 2}))   # [6, 12]
```

***

#### Control Flow

```python
score = 85
label = 'pass' if score >= 60 else 'fail'

# for/else: else runs only when the loop did not break
for n in range(2, 10):
    if 10 % n == 0:
        print('first divisor', n)   # first divisor 2
        break
else:
    print('prime')

# Walrus (3.8+); 3.10 also allows it unparenthesized in sequence indexes (SyntaxError on 3.9)
values = [10, 20, 30]
print(values[i := 1], i)          # 20 1
```

**`match` vs `if`/`elif`:** use `match` when the *structure* (shape, type, nesting) of the subject drives the branch; use `if`/`elif` for boolean conditions and ranges (`if 0 <= x < 10`), which pattern matching can only express through guards.

```python
import json

def handle_message(raw: str) -> str:
    match json.loads(raw):
        case {'op': 'ping'}:
            return 'pong'
        case {'op': 'sum', 'args': [*nums]} if all(isinstance(n, (int, float)) for n in nums):
            return str(sum(nums))
        case {'op': str(op)}:
            return f'unknown op {op!r}'
        case _:
            return 'malformed'

print(handle_message('{"op": "sum", "args": [1, 2, 3]}'))   # 6
print(handle_message('{"op": "sum", "args": "123"}'))       # unknown op 'sum' — str is not a sequence pattern
print(handle_message('[1, 2]'))                             # malformed
```

***

#### Functions

```python
def greet(
    name: str,
    greeting: str = 'Hello',
    /,                    # positional-only before this (3.8+)
    *,                    # keyword-only after this
    punctuation: str = '!',
) -> str:
    """Return a greeting string."""
    return f'{greeting}, {name}{punctuation}'

print(greet('Alice'))                    # Hello, Alice!
print(greet('Alice', 'Hi'))              # Hi, Alice!
print(greet('Alice', punctuation='.'))   # Hello, Alice.
```

**Pitfall: mutable default arguments are evaluated once.**

```python
def append_bad(item: int, bucket: list[int] = []) -> list[int]:   # ❌ shared across calls
    bucket.append(item)
    return bucket

print(append_bad(1), append_bad(2))   # [1, 2] [1, 2]

def append_good(item: int, bucket: list[int] | None = None) -> list[int]:   # ✅
    if bucket is None:
        bucket = []
    bucket.append(item)
    return bucket

print(append_good(1), append_good(2))   # [1] [2]
```

##### Closures and Late Binding

```python
from collections.abc import Callable

def make_counter(start: int = 0) -> Callable[[], int]:
    count = start
    def counter() -> int:
        nonlocal count
        count += 1
        return count
    return counter

c = make_counter()
print(c(), c(), c())   # 1 2 3

funcs = [lambda: i for i in range(3)]        # ❌ late binding
print([f() for f in funcs])                  # [2, 2, 2]
funcs = [lambda i=i: i for i in range(3)]    # ✅ bind at definition time
print([f() for f in funcs])                  # [0, 1, 2]
```

##### `functools`

```python
import functools
import statistics

@functools.cache                 # 3.9+: unbounded — only for bounded argument spaces
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)

@functools.lru_cache(maxsize=1024)   # bounded — for open-ended inputs
def normalize(s: str) -> str:
    return ' '.join(s.split()).casefold()

class Dataset:
    def __init__(self, data: list[float]) -> None:
        self.data = data

    @functools.cached_property    # computed once per instance, stored in __dict__
    def mean(self) -> float:
        return statistics.mean(self.data)

print(fib(30), normalize('  Hello   World '), Dataset([1.0, 2.0, 3.0]).mean)   # 832040 hello world 2.0

@functools.singledispatch
def render(value: object) -> str:
    raise TypeError(f'unsupported: {type(value).__name__}')

@render.register
def _(value: int) -> str:
    return f'int:{value}'

@render.register
def _(value: list) -> str:
    return '[' + ', '.join(render(v) for v in value) + ']'

print(render([1, 2]))   # [int:1, int:2]
```

`cached_property` pitfalls on 3.10: it fails with `TypeError` on classes with `__slots__` (including `dataclass(slots=True)`), and it still holds a **class-wide lock** while computing (removed in 3.12), so concurrent first accesses on *different* instances serialize. Note also that `singledispatch` registration via annotation does **not** accept `int | str` unions in 3.10 (union support arrived in 3.11) — register each type separately.

```python
import functools

@functools.singledispatch
def size(x: object) -> int:
    return 0

try:
    @size.register
    def _(x: int | str) -> int:      # ❌ 3.10: TypeError — unions accepted from 3.11
        return 1
except TypeError as e:
    print('TypeError:', e)   # TypeError: Invalid annotation for 'x'. int | str is not a class.

@size.register(int)                  # ✅ register types one by one
@size.register(str)
def _(x) -> int:
    return 1

print(size(3), size('a'), size(2.5))   # 1 1 0
```

##### Generators and `itertools` — New in 3.10: `pairwise()`

```python
import itertools
from collections.abc import Iterator

def fibonacci() -> Iterator[int]:
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

print(list(itertools.islice(fibonacci(), 8)))        # [0, 1, 1, 2, 3, 5, 8, 13]

# New in 3.10: itertools.pairwise — overlapping pairs
print(list(itertools.pairwise([1, 4, 9, 16])))        # [(1, 4), (4, 9), (9, 16)]
deltas = [b - a for a, b in itertools.pairwise([1, 4, 9, 16])]
print(deltas)                                          # [3, 5, 7]

print(list(itertools.accumulate([1, 2, 3], initial=0)))   # [0, 1, 3, 6]
print(list(itertools.chain.from_iterable([[1], [2, 3]]))) # [1, 2, 3]

# itertools.batched() is 3.12+ — in 3.10, chunk with islice:
def batched(iterable, n: int):
    it = iter(iterable)
    while chunk := tuple(itertools.islice(it, n)):
        yield chunk

print(list(batched('abcdefg', 3)))   # [('a', 'b', 'c'), ('d', 'e', 'f'), ('g',)]
```

Remove hand-written `pairwise` helpers (and `more_itertools.pairwise` imports) in 3.10-only code; do **not** use `itertools.batched` — it does not exist before 3.12.

***

#### Dataclasses

##### New in 3.10: `slots=True`, `kw_only=True`, `KW_ONLY`, `match_args`

```python
from dataclasses import KW_ONLY, dataclass, field

@dataclass(slots=True)                 # generates __slots__ — less memory, faster attribute access
class Point:
    x: float
    y: float = 0.0

p = Point(1.0)
print(Point.__slots__)                 # ('x', 'y')
try:
    p.z = 3.0                          # typo-safe: no __dict__
except AttributeError as e:
    print(e)                           # 'Point' object has no attribute 'z'

@dataclass(kw_only=True)               # every field keyword-only
class Request:
    method: str = 'GET'
    url: str                           # allowed: no "non-default after default" error for kw-only fields

print(Request(url='/health'))          # Request(method='GET', url='/health')

@dataclass
class Query:
    table: str                         # positional
    _: KW_ONLY                         # everything after the sentinel is keyword-only
    limit: int = 100
    order_by: str | None = None

print(Query('users', limit=10))        # Query(table='users', limit=10, order_by=None)
print(Query.__match_args__)            # ('table',) — keyword-only fields are excluded

@dataclass
class Mixed:
    debug: bool = field(default=False, kw_only=True)   # per-field
    name: str = 'svc'
```

Why it matters for review:

- `kw_only` removes the "non-default argument follows default argument" constraint and prevents positional-argument mixups in wide records (`Request('GET', '/x')` cannot silently swap arguments when every field is keyword-only).
- Keyword-only fields are **moved to the end** of `__init__`, after all positional fields, regardless of declaration order — keep this in mind with inheritance.
- `slots=True` returns a **new class** (the decorator must recreate the class to add `__slots__`). Consequences on 3.10:

```python
import functools
import weakref
from dataclasses import dataclass

@dataclass(slots=True)
class Base:
    a: int
    def describe(self) -> str:
        return 'base'

@dataclass(slots=True)
class Child(Base):
    b: int = 0
    def describe(self) -> str:
        return super().describe() + '+child'          # ❌ zero-arg super() breaks with slots=True on 3.10

try:
    Child(1).describe()
except TypeError as e:
    print(e)    # super(type, obj): obj must be an instance or subtype of type

@dataclass(slots=True)
class Child2(Base):
    b: int = 0
    def describe(self) -> str:
        return super(Child2, self).describe() + '+child'   # ✅ two-argument super() works

print(Child2(1).describe())   # base+child

try:
    weakref.ref(Child2(1))    # ❌ no __weakref__ slot (weakref_slot=True is 3.11+)
except TypeError as e:
    print(e)    # cannot create weak reference to 'Child2' object
```

| `slots=True` limitation on 3.10 | Workaround |
|---|---|
| Zero-argument `super()` in methods raises `TypeError` (gh-90562, fixed in later versions) | `super(Cls, self)` |
| No weak references (`weakref_slot` is 3.11+) | declare `__weakref__` in a manual `__slots__`, or don't use `slots=True` |
| `functools.cached_property` fails (`No '__dict__' attribute`) | compute in `__post_init__` or use a plain `@property` |
| Base-class `__init_subclass__` with required keyword arguments raises `TypeError` (gh-91126) | give those parameters defaults |
| Default values are fine (handled by the decorator) | — |

##### Dataclass Fundamentals and Pitfalls

```python
from dataclasses import asdict, dataclass, field, replace
from typing import ClassVar

@dataclass
class User:
    id: int
    name: str
    email: str
    active: bool = True
    tags: list[str] = field(default_factory=list)   # ✅ never `= []`
    MAX_TAGS: ClassVar[int] = 10                     # class constant, not a field

    def __post_init__(self) -> None:
        self.email = self.email.lower()
        if len(self.tags) > self.MAX_TAGS:
            raise ValueError('too many tags')

u = User(1, 'Ann', 'ANN@EXAMPLE.COM')
print(u.email, asdict(u)['tags'])            # ann@example.com []
print(replace(u, active=False).active)       # False — copy with changes (copy.replace is 3.13+)

@dataclass(frozen=True, slots=True)
class Money:
    amount: int
    currency: str

print(hash(Money(1, 'EUR')) == hash(Money(1, 'EUR')))   # True — frozen → hashable

try:
    @dataclass
    class Bad:
        items: list[int] = []          # ❌
except ValueError as e:
    print(e)    # mutable default <class 'list'> for field items is not allowed: use default_factory
```

`frozen=True` raises `dataclasses.FrozenInstanceError` (a subclass of `AttributeError`) on assignment. `eq=True` without `frozen=True` sets `__hash__` to `None` — mutable dataclasses are deliberately unhashable.

***

#### Classes and OOP

##### Class Definition

```python
import math

class Shape:
    count: int = 0                     # class attribute — shared

    def __init__(self, color: str = 'black') -> None:
        self.color = color
        Shape.count += 1

    def __repr__(self) -> str:
        return f'{type(self).__name__}(color={self.color!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, type(self)):
            return NotImplemented
        return self.__dict__ == other.__dict__
    # defining __eq__ sets __hash__ = None: correct for a MUTABLE class

class Circle(Shape):
    def __init__(self, radius: float, **kwargs) -> None:
        super().__init__(**kwargs)
        self.radius = radius

    @property
    def area(self) -> float:
        return math.pi * self.radius ** 2

    @classmethod
    def unit(cls) -> 'Circle':
        return cls(1.0)

    @staticmethod
    def is_valid(r: float) -> bool:
        return r > 0

print(Circle.unit().area, Circle(2.0, color='red'))   # 3.141592653589793 Circle(color='red')
```

**3.10 change to `staticmethod`/`classmethod`:** they now copy `__name__`, `__qualname__`, `__doc__`, `__module__`, `__annotations__` from the wrapped function and expose `__wrapped__`; a `staticmethod` object is now directly callable (e.g. from a class-body dict of handlers).

```python
def _double(x: int) -> int:
    return x * 2

class Ops:
    double = staticmethod(_double)
    HANDLERS = {'double': double}          # the staticmethod object itself

print(Ops.HANDLERS['double'](4))           # 8 — TypeError before 3.10
print(Ops.__dict__['double'].__wrapped__ is _double)   # True
```

##### Abstract Base Classes and Protocols

```python
from abc import ABC, abstractmethod
from typing import Protocol

class Repository(ABC):
    @abstractmethod
    def get(self, id: int) -> dict[str, object] | None: ...

    @abstractmethod
    def save(self, entity: dict[str, object]) -> int: ...

    def exists(self, id: int) -> bool:
        return self.get(id) is not None

class InMemoryRepository(Repository):
    def __init__(self) -> None:
        self._rows: dict[int, dict[str, object]] = {}

    def get(self, id: int) -> dict[str, object] | None:
        return self._rows.get(id)

    def save(self, entity: dict[str, object]) -> int:
        new_id = len(self._rows) + 1
        self._rows[new_id] = entity
        return new_id

class SupportsClose(Protocol):       # structural typing — no inheritance needed
    def close(self) -> None: ...

repo = InMemoryRepository()
print(repo.exists(repo.save({'name': 'x'})))   # True
```

##### Enums — Behavior That Changes in 3.11

```python
from enum import Enum, Flag, IntEnum, auto

class Color(Enum):
    RED = auto()
    GREEN = auto()

class HttpStatus(IntEnum):
    OK = 200
    NOT_FOUND = 404

class Perm(Flag):
    READ = auto()
    WRITE = auto()
    EXEC = auto()

print(Perm.READ | Perm.WRITE)          # Perm.WRITE|READ
print(Perm.EXEC in (Perm.READ | Perm.WRITE))   # False

# 3.10 has no StrEnum (3.11). The str mix-in is the 3.10 idiom:
class Env(str, Enum):
    PROD = 'prod'
    DEV = 'dev'

print(Env.PROD == 'prod')          # True
print(f'{Env.PROD}')               # 'prod' on 3.10 — but 'Env.PROD' on 3.11+!
print(str(Env.PROD))               # 'Env.PROD' — str() and format() DISAGREE on 3.10
print(f'{HttpStatus.OK}', str(HttpStatus.OK))   # '200' 'HttpStatus.OK' on 3.10
```

**Pitfall (upgrade hazard):** for mixed-in enums, 3.10's `format()`/f-strings use the *value* while `str()` uses `Class.MEMBER`. 3.11 changed `format()` to match `str()` for `(str, Enum)` mixins (and changed `IntEnum.__str__` to the value). Code that builds URLs, SQL, file names or log fields with `f'{member}'` silently changes output on upgrade. Always write `member.value` explicitly.

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
        super().__init__(f'config error for {key!r}' + (f': {message}' if message else ''))

def read_config(text: str) -> dict[str, object]:
    try:
        return json.loads(text)
    except json.JSONDecodeError as e:
        raise ConfigError('file', 'invalid JSON') from e    # ✅ explicit chaining

try:
    read_config('{bad')
except ConfigError as e:
    print(e, '| caused by', type(e.__cause__).__name__)   # config error for 'file': invalid JSON | caused by JSONDecodeError
```

**3.10 notes:**

- **No `ExceptionGroup`, `except*` or `add_note()`** — those are 3.11. For multiple concurrent failures use `asyncio.gather(..., return_exceptions=True)` and inspect the list.
- **`except A, B:` is a `SyntaxError`** (3.10 reports `multiple exception types must be parenthesized`); write `except (A, B):`. The unparenthesized form only becomes valid in 3.14.
- **`traceback.format_exception(exc)`** and `traceback.print_exception(exc)` accept the exception object alone (new in 3.10) — no more `(type(e), e, e.__traceback__)` triples. The old `etype` parameter was renamed `exc`.
- **Timeout exceptions are not unified yet:** `socket.timeout` *is* `TimeoutError` (new in 3.10), but `asyncio.TimeoutError` and `concurrent.futures.TimeoutError` are **distinct classes** until 3.11. `except TimeoutError:` does **not** catch `asyncio.wait_for()` timeouts on 3.10.

```python
import asyncio
import concurrent.futures
import socket
import traceback

print(socket.timeout is TimeoutError)                       # True  (3.10)
print(asyncio.TimeoutError is TimeoutError)                 # False (True from 3.11)
print(concurrent.futures.TimeoutError is TimeoutError)      # False (True from 3.11)

try:
    1 / 0
except ZeroDivisionError as e:
    print(traceback.format_exception(e)[-1].strip())        # ZeroDivisionError: division by zero
```

```python
import asyncio

async def slow() -> None:
    await asyncio.sleep(10)

async def main() -> str:
    try:
        await asyncio.wait_for(slow(), timeout=0.01)
    except TimeoutError:                     # ❌ never reached on 3.10
        return 'builtin TimeoutError'
    except asyncio.TimeoutError:             # ✅ required on 3.10
        return 'asyncio.TimeoutError'
    return 'no timeout'

print(asyncio.run(main()))   # asyncio.TimeoutError
```

Re-raise with a bare `raise` (preserves the traceback), log with `logger.exception()` inside `except`, and never catch `BaseException` unless you re-raise.

***

#### Context Managers

```python
import contextlib
import logging
import os
import time

logger = logging.getLogger(__name__)

@contextlib.contextmanager
def timed(name: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        logger.info('%s took %.3fs', name, time.perf_counter() - start)

with timed('work'):
    sum(range(1000))

with contextlib.suppress(FileNotFoundError):
    os.remove('/nonexistent/cache')

# Variable number of resources → ExitStack
paths: list[str] = []
with contextlib.ExitStack() as stack:
    files = [stack.enter_context(open(p, encoding='utf-8')) for p in paths]
```

**New in 3.10 (`contextlib`):**

- **`contextlib.aclosing(agen)`** — the async counterpart of `closing()`; guarantees an async generator's `aclose()` runs even if the consumer breaks out early.
- **`contextlib.AsyncContextDecorator`** — lets an async context manager also be used as a decorator.
- **`contextlib.nullcontext()`** now also works with `async with`.
- `contextlib.chdir()` is **3.11** — on 3.10, write a small `@contextmanager` around `os.chdir` (and remember it is process-global, not thread-safe).

```python
import asyncio
import contextlib

async def ticker(n: int):
    try:
        for i in range(n):
            yield i
    finally:
        print('ticker closed')

async def main() -> None:
    async with contextlib.aclosing(ticker(10)) as agen:   # ✅ deterministic cleanup
        async for value in agen:
            if value == 2:
                break
    async with contextlib.nullcontext('resource') as r:   # 3.10: async support
        print(r)

asyncio.run(main())
# ticker closed
# resource
```

***

#### File I/O and `pathlib`

##### New in 3.10: `EncodingWarning` and `encoding="locale"` (PEP 597)

`open()`, `Path.read_text()`, `Path.write_text()`, `io.TextIOWrapper` and friends default to the **locale** encoding when `encoding=` is omitted. On Linux/macOS that is usually UTF-8; on Windows it is usually a legacy code page such as cp1252 — so the same code corrupts data or raises `UnicodeDecodeError` depending on where it runs. 3.10 adds an **opt-in** `EncodingWarning` to find every such call.

```bash
# Enable in CI / test runs:
python -X warn_default_encoding -W error::EncodingWarning -m pytest
# or
PYTHONWARNDEFAULTENCODING=1 python -m pytest -W error::EncodingWarning
```

```python
import tempfile
import warnings
from pathlib import Path

path = Path(tempfile.mkdtemp()) / 'data.json'
path.write_text('{"a": 1}', encoding='utf-8')          # ✅ explicit

with open(path, encoding='utf-8') as f:                # ✅ explicit UTF-8
    print(f.read())

with open(path, encoding='locale') as f:               # ✅ new in 3.10: "I really want the locale encoding"
    f.read()

with open(path, 'rb') as f:                            # ✅ binary mode never needs an encoding
    f.read()
```

- The warning is emitted only when `sys.flags.warn_default_encoding` is set (`-X warn_default_encoding` or `PYTHONWARNDEFAULTENCODING`); it is **off by default**, so add it to CI explicitly.
- `encoding='locale'` documents an intentional locale-dependent read (e.g. a console tool reading user-edited files). It is new in 3.10 — passing it on 3.9 raises `LookupError`.
- Library functions that take an `encoding=None` parameter and forward it to `open()` should call `encoding = io.text_encoding(encoding)` so that the warning points at *their caller*, not at the library.
- Python 3.15 makes UTF-8 mode the default (PEP 686); explicit `encoding='utf-8'` is the only spelling that behaves identically on 3.10 and later versions on all platforms.

```python
import io

def read_config(path: str, encoding: str | None = None) -> str:
    encoding = io.text_encoding(encoding)   # None -> 'locale' on 3.10, and the EncodingWarning is attributed to our caller
    with open(path, encoding=encoding) as f:
        return f.read()
```

##### `pathlib` Essentials and 3.10 Additions

```python
import tempfile
from pathlib import Path, PurePosixPath

base = Path(tempfile.mkdtemp())
report = base / 'reports' / 'q3.csv'
report.parent.mkdir(parents=True, exist_ok=True)
report.write_text('a,b\n1,2\n', encoding='utf-8')

print(report.name, report.stem, report.suffix)        # q3.csv q3 .csv
print(report.with_stem('q4').name)                    # q4.csv (3.9+)
report.unlink(missing_ok=True)                        # 3.8+

p = PurePosixPath('/srv/app/data/file.txt')
# New in 3.10: parents supports slices and negative indexes
print(p.parents[-1])          # /
print(p.parents[0:2])         # (PurePosixPath('/srv/app/data'), PurePosixPath('/srv/app'))

# New in 3.10: Path.hardlink_to() replaces the deprecated link_to() (removed in 3.12)
src = base / 'src.txt'
src.write_text('x', encoding='utf-8')
(base / 'hard.txt').hardlink_to(src)                  # same argument order as symlink_to()

# New in 3.10: Path.stat()/chmod() accept follow_symlinks=
print((base / 'hard.txt').stat(follow_symlinks=False).st_nlink)   # 2
```

`Path.walk()` (3.12), `Path.is_junction()` (3.12) and `Path.from_uri()` (3.13) do not exist in 3.10 — use `os.walk()`.

##### Path Traversal Safety

```python
import os
import tempfile
from pathlib import Path

def safe_resolve(base_dir: str, user_path: str) -> Path:
    base = Path(base_dir).resolve()
    target = (base / user_path).resolve()
    if not target.is_relative_to(base):            # 3.9+, purely lexical — resolve() first!
        raise PermissionError(f'{user_path!r} escapes {base}')
    return target

root = tempfile.mkdtemp()
print(safe_resolve(root, 'a/b.txt').name)          # b.txt
try:
    safe_resolve(root, '../../etc/passwd')
except PermissionError as e:
    print('blocked')

# New in 3.10: os.path.realpath(strict=True) raises instead of returning a non-existent path
try:
    os.path.realpath('/definitely/missing', strict=True)
except FileNotFoundError:
    print('missing')
```

Since **3.10.18**, `os.path.realpath()` also accepts `strict=os.path.ALLOW_MISSING` (resolve symlinks, tolerate missing path components, but re-raise errors other than `FileNotFoundError`) — added as part of the tarfile CVE-2025-4517 fixes.

##### Other 3.10 I/O Additions

- `glob.glob(pattern, root_dir=..., dir_fd=...)` — search relative to a directory without `os.chdir()`.
- `fileinput.input(..., encoding=..., errors=...)`.
- `os.eventfd()` and `os.splice()` (Linux); `os.mkdir(mode=0o700)` is honored on Windows since **3.10.15** (CVE-2024-4030), which also makes `tempfile.mkdtemp()` private on Windows.

##### Binary and Temporary Files

```python
import tempfile

CHUNK = 64 * 1024
with tempfile.TemporaryFile() as f:
    f.write(b'x' * 200_000)
    f.seek(0)
    total = 0
    while chunk := f.read(CHUNK):
        total += len(chunk)
print(total)   # 200000

with tempfile.NamedTemporaryFile('w', suffix='.csv', encoding='utf-8', delete=True) as tmp:
    tmp.write('col1,col2\n')
    tmp.flush()

with tempfile.TemporaryDirectory() as tmpdir:
    pass
# NEVER tempfile.mktemp() — race-prone (TOCTOU)
```

***

#### `async`/`await` and `asyncio`

##### Removed in 3.10: the `loop=` Parameter

The `loop` argument of the high-level API — `asyncio.sleep()`, `gather()`, `wait_for()`, `wait()`, `shield()`, `as_completed()`, `open_connection()`, `start_server()`, `Queue()`, `Lock()`, `Event()`, `Condition()`, `Semaphore()`, `BoundedSemaphore()` — is **removed**; passing it raises `TypeError`.

```python
import asyncio

async def main() -> None:
    try:
        await asyncio.sleep(0, loop=asyncio.get_running_loop())   # ❌ removed in 3.10
    except TypeError as e:
        print(e)          # sleep() got an unexpected keyword argument 'loop'
    try:
        asyncio.Lock(loop=asyncio.get_running_loop())            # ❌
    except TypeError as e:
        print(e)          # As of 3.10, the *loop* parameter was removed from Lock() since it is no longer necessary

    lock = asyncio.Lock()                                       # ✅ binds to the running loop on first use
    async with lock:
        await asyncio.sleep(0)

asyncio.run(main())
```

To run a coroutine on a loop in *another thread*, use `asyncio.run_coroutine_threadsafe(coro, loop)`. Low-level loop APIs (`loop.create_connection()` etc.) are unchanged.

##### `get_event_loop()` — the Patch-Level Story

`asyncio.get_event_loop()` without a running loop was slated for deprecation in 3.10, and its behavior **changed across 3.10 patch releases**:

| 3.10 patch | `get_event_loop()` with no running loop |
|---|---|
| 3.10.0 – 3.10.8 | `DeprecationWarning` (even if a loop had been set with `set_event_loop()`) |
| 3.10.9 | `DeprecationWarning` only if no current loop is set and one would be created implicitly |
| 3.10.10 and later | **no warning at all** — the deprecation was deferred to 3.12 (gh-100160) |

It still silently *creates* a new loop in the main thread. Do not write new code against it:

```python
import asyncio

async def worker() -> int:
    loop = asyncio.get_running_loop()      # ✅ inside coroutines/callbacks; RuntimeError if no loop runs
    return await loop.run_in_executor(None, sum, [1, 2, 3])

print(asyncio.run(worker()))               # ✅ entry point — creates, runs and closes the loop: 6
```

Likewise, `asyncio.ensure_future()`/`gather()` called outside a running loop on 3.10 implicitly create a loop. Create tasks only inside coroutines.

##### Structured Concurrency Without `TaskGroup` (3.11+)

`asyncio.TaskGroup` and `asyncio.timeout()` do not exist on 3.10. The 3.10 equivalents:

```python
import asyncio

async def fetch(i: int) -> int:
    await asyncio.sleep(0.01 * i)
    if i == 3:
        raise ValueError(f'job {i} failed')
    return i * 10

async def run_all() -> list[int]:
    tasks = [asyncio.create_task(fetch(i), name=f'fetch-{i}') for i in range(5)]
    try:
        return await asyncio.gather(*tasks)       # first exception propagates...
    except Exception:
        for t in tasks:                           # ...but siblings keep running: cancel them yourself
            t.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)   # wait for cancellation to finish
        raise

async def run_all_collect() -> list[int | BaseException]:
    return await asyncio.gather(*(fetch(i) for i in range(5)), return_exceptions=True)

async def main() -> None:
    try:
        await run_all()
    except ValueError as e:
        print('failed:', e)                       # failed: job 3 failed
    results = await run_all_collect()
    print([r if isinstance(r, int) else type(r).__name__ for r in results])   # [0, 10, 20, 'ValueError', 40]

    try:
        await asyncio.wait_for(asyncio.sleep(1), timeout=0.05)   # 3.10 timeout API
    except asyncio.TimeoutError:                                  # NOT the builtin TimeoutError on 3.10
        print('timed out')

asyncio.run(main())
```

**Pitfall:** `gather()` without `return_exceptions=True` propagates the first exception but does **not** cancel the other awaitables — they keep running in the background (and their exceptions may later be logged as "never retrieved"). That is exactly what `TaskGroup` fixed in 3.11; on 3.10 cancel explicitly as above.

##### New in 3.10: `aiter()` and `anext()`

```python
import asyncio

async def numbers():
    yield 1
    yield 2

async def main() -> None:
    it = aiter(numbers())                  # async counterpart of iter()
    print(await anext(it))                 # 1
    print(await anext(it))                 # 2
    print(await anext(it, 'done'))         # 'done' — default instead of StopAsyncIteration
    try:
        await anext(it)
    except StopAsyncIteration:
        print('exhausted')

asyncio.run(main())
```

Use `anext(agen, default)` to read the first item of an async stream; `aiter()`/`anext()` only accept *async* iterables/iterators (`TypeError` otherwise).

##### Core `asyncio` Patterns

```python
import asyncio

sem = asyncio.Semaphore(10)

async def fetch(url: str) -> str:
    await asyncio.sleep(0)
    return url.upper()

async def bounded(url: str) -> str:
    async with sem:                         # bounded concurrency
        return await fetch(url)

async def worker(queue: asyncio.Queue[str]) -> None:
    while True:
        item = await queue.get()
        try:
            await fetch(item)
        finally:
            queue.task_done()

async def main() -> None:
    print(await asyncio.gather(*(bounded(u) for u in ['a', 'b'])))   # ['A', 'B']
    queue: asyncio.Queue[str] = asyncio.Queue()
    workers = [asyncio.create_task(worker(queue)) for _ in range(3)]
    for u in 'xyz':
        queue.put_nowait(u)
    await queue.join()
    for w in workers:
        w.cancel()
    await asyncio.gather(*workers, return_exceptions=True)
    data = await asyncio.to_thread(sum, range(100))   # blocking call off the loop (3.9+)
    print(data)                                       # 4950

asyncio.run(main())
```

- **Always re-raise `CancelledError`** after cleanup; swallowing it breaks cancellation and shutdown.
- **Keep a reference to fire-and-forget tasks** (`background.add(task); task.add_done_callback(background.discard)`) — the loop holds only weak references, so an unreferenced task can be garbage-collected mid-flight.
- `asyncio.Queue[str]` as an annotation works at runtime in 3.10 (`asyncio.Queue` supports `__class_getitem__` since 3.9).

***

#### Concurrency

##### The GIL

CPython 3.10 always has a Global Interpreter Lock: threads help I/O-bound work; CPU-bound pure-Python work needs processes (`ProcessPoolExecutor`, `multiprocessing`) or native extensions that release the GIL.

##### `threading` — Deprecated camelCase Aliases (3.10)

```python
import threading

lock = threading.Lock()
results: list[int] = []

def work(n: int) -> None:
    with lock:
        results.append(n * n)

threads = [threading.Thread(target=work, args=(i,), daemon=True) for i in range(4)]
for t in threads:
    t.start()
for t in threads:
    t.join()
print(sorted(results))                       # [0, 1, 4, 9]

print(threading.current_thread().name)       # ✅ not currentThread()
print(threading.active_count() >= 1)         # ✅ not activeCount()
event = threading.Event()
print(event.is_set())                        # ✅ not isSet()
t = threading.Thread(target=lambda: None)
t.daemon = True                              # ✅ not setDaemon(True)
t.name = 'bg'                                # ✅ not setName()
```

`currentThread()`, `activeCount()`, `Condition.notifyAll()`, `Event.isSet()`, `Thread.setName()/getName()`, `Thread.isDaemon()/setDaemon()` emit `DeprecationWarning` in 3.10. New: `threading.gettrace()`, `threading.getprofile()`, `threading.__excepthook__`.

##### `concurrent.futures`

```python
from concurrent.futures import ThreadPoolExecutor, as_completed

def fetch(n: int) -> int:
    return n * 2

with ThreadPoolExecutor(max_workers=4) as pool:
    futures = {pool.submit(fetch, n): n for n in range(5)}
    results = {futures[f]: f.result() for f in as_completed(futures)}
print(dict(sorted(results.items())))   # {0: 0, 1: 2, 2: 4, 3: 6, 4: 8}
```

`concurrent.futures.TimeoutError` is **not** the builtin `TimeoutError` on 3.10 (aliased in 3.11). `shutdown(cancel_futures=True)` (3.9+) still has the unfixed `as_completed()`/`wait()` hang on drained futures (bpo-43727) — filter `f.cancelled()` first.

##### `multiprocessing`

```python
from multiprocessing import Pool

def square(n: int) -> int:
    return n * n

if __name__ == '__main__':          # required with the spawn start method (Windows, macOS default)
    with Pool(processes=2) as pool:
        print(pool.map(square, range(5)))   # [0, 1, 4, 9, 16]
```

***

#### `collections.abc` — Aliases Removed in 3.10

The ABC aliases in `collections` (`collections.Mapping`, `collections.Sequence`, `collections.Callable`, `collections.Iterable`, …), deprecated since 3.3, are **removed** in 3.10. This is the most common import-time failure when upgrading from 3.9:

```python
import collections

try:
    collections.Mapping                     # ❌ AttributeError in 3.10
except AttributeError as e:
    print(e)    # module 'collections' has no attribute 'Mapping'

try:
    from collections import Mapping         # ❌ ImportError in 3.10
except ImportError as e:
    print(type(e).__name__)                 # ImportError

from collections.abc import Callable, Iterable, Mapping, MutableMapping, Sequence   # ✅
```

Old third-party packages pinned to ancient versions (e.g. old `requests`/`urllib3`, `jinja2<2.11`, `pyyaml<5.1`-era code) hit this on import — upgrade them. Find first-party violations with:

```bash
grep -rnE "collections\.(Mapping|MutableMapping|Sequence|MutableSequence|Set|MutableSet|Callable|Iterable|Iterator|Generator|Hashable|Sized|Container|Awaitable|Coroutine|AsyncIterable|AsyncIterator)\b|from collections import[^#]*\b(Mapping|MutableMapping|Sequence|Callable|Iterable|Iterator|Set|MutableSet|Hashable|Sized|Container|Awaitable|Coroutine)\b" --include='*.py' .
```

***

#### New in 3.10: Precise Line Numbers (PEP 626)

Tracing (`sys.settrace`), profilers, coverage tools and debuggers now get a line event for every executed line and only for executed lines; `frame.f_lineno` is always correct. Practical consequences:

- **Coverage numbers can shift** when moving from 3.9 to 3.10 (e.g. `pass`, `else`, decorators and multi-line expressions are reported differently). Re-baseline coverage thresholds instead of chasing phantom misses.
- `code.co_lnotab` is deprecated (silently in 3.10; it emits `DeprecationWarning` from 3.12); use `code.co_lines()`, which yields `(start, end, line)` ranges.

```python
def f(x):
    y = x + 1
    return y

for start, end, line in f.__code__.co_lines():
    pass
print(sorted({line for _, _, line in f.__code__.co_lines() if line is not None}))   # [2, 3] on 3.10
```

***

#### Logging

```python
import logging

def setup_logging(level: str = 'INFO') -> None:
    logging.basicConfig(
        level=level.upper(),                 # basicConfig accepts level names
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        force=True,                          # 3.8+: replace existing handlers
    )

logger = logging.getLogger(__name__)
setup_logging()

item, count = {'id': 1}, 3
logger.debug('Processing %r', item)          # ✅ lazy %-formatting
logger.info('Loaded %d records', count)
try:
    1 / 0
except ZeroDivisionError:
    logger.exception('Unexpected error')     # includes traceback (no "Did you mean" hints in 3.10)
```

`logging.getLevelNamesMapping()` is 3.11 — on 3.10 use `logging.getLevelName(name)` (returns the int for a known name) or a `dict` of your own.

***

#### Security

##### New in 3.10: OpenSSL 1.1.1+ Required and Stricter `ssl` Defaults (PEP 644)

3.10 requires **OpenSSL 1.1.1 or newer**; building against 1.0.2 or LibreSSL is unsupported. Old enterprise Linux images that only ship OpenSSL 1.0.2 produce a 3.10 build without a working `ssl` module (pip then reports that the ssl module in Python is not available).

The `ssl` module also got secure-by-default settings:

```python
import ssl

ctx = ssl.create_default_context()           # ✅ always start here for clients
print(ctx.minimum_version)                   # TLSVersion.TLSv1_2 — TLS 1.0/1.1 off by default
print(ctx.verify_mode, ctx.check_hostname)   # VerifyMode.CERT_REQUIRED True

ctx.minimum_version = ssl.TLSVersion.TLSv1_3  # ✅ raise the floor with minimum_version, not OP_NO_* flags
```

- `SSLContext` defaults to **TLS 1.2 minimum**, security level 2, and no ciphers without forward secrecy or with SHA-1 MACs. Legacy servers that only speak TLS 1.0/1.1 fail to handshake after upgrading — fix the server, or (consciously, documented) lower `minimum_version` for that one context.
- **Deprecated (DeprecationWarning in 3.10):** `ssl.SSLContext()` without a protocol argument, `ssl.PROTOCOL_TLS`/`PROTOCOL_SSLv23`/`PROTOCOL_TLSv1*`, the `OP_NO_SSLv2…OP_NO_TLSv1_3` options, `ssl.wrap_socket()`, `ssl.match_hostname()`, `RAND_pseudo_bytes()`, and NPN. Use `PROTOCOL_TLS_CLIENT`/`PROTOCOL_TLS_SERVER`, `minimum_version`/`maximum_version`, and `SSLContext.wrap_socket()`. (What's New 3.10 said they would be removed in 3.11; in fact `wrap_socket()` and `match_hostname()` were removed in **3.12**.)
- `hashlib` and `hmac` use OpenSSL implementations; the pure-Python `pbkdf2_hmac` fallback is deprecated.

```python
import ssl
import warnings

with warnings.catch_warnings():
    warnings.simplefilter('error')
    try:
        ssl.SSLContext(ssl.PROTOCOL_TLS)      # ❌ deprecated
    except DeprecationWarning as e:
        print(e)                              # ssl.PROTOCOL_TLS is deprecated
    server_ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)   # ✅
```

##### Security Fixes in 3.10 and Its Point Releases

| Version | Change |
|---|---|
| 3.10.0 | `urllib.parse.parse_qs()/parse_qsl()` accept only one separator (`&` by default, not `;`) — web-cache-poisoning fix; `urllib.parse` strips `\t`, `\r`, `\n` from URLs (bpo-43882) |
| 3.10.7 | 4300-digit limit on `int`↔`str` conversion (CVE-2020-10735) |
| 3.10.8 | deprecated `mailcap` refuses to inject unsafe text into shell commands |
| 3.10.12 | `tarfile` extraction filters (PEP 706): `extractall(path, filter='data')` |
| 3.10.15 | `email` refuses to serialize headers with injected newlines; `email.utils.getaddresses()/parseaddr()` stricter (`strict=` parameter); `ipaddress` `is_private`/`is_global` corrected; Windows `os.mkdir(mode=0o700)` |
| 3.10.18 | `tarfile` filter bypasses fixed (CVE-2025-4517, -4138, -4330, CVE-2024-12718); `os.path.ALLOW_MISSING` |
| 3.10.19–3.10.22 | `html.parser` HTML5-conformant parsing and quadratic-time fixes, zip64 consistency checks, `http.client` memory/trailer-flood DoS fixes, `http.cookies`/`wsgiref` control-character rejection, further `tarfile` filter bypasses (incl. CVE-2026-82049 in 3.10.22), bundled libexpat updates |

Source: What's New 3.10 and the 3.10 changelog. Because 3.10 is now EOL, vulnerabilities found after October 2026 will **not** be fixed in 3.10.

##### `tarfile` Extraction — Always Pass `filter=` (3.10.12+)

```python
import io
import tarfile
import tempfile

buf = io.BytesIO()
with tarfile.open(fileobj=buf, mode='w') as tar:
    data = b'hello'
    info = tarfile.TarInfo('../escape.txt')     # malicious path
    info.size = len(data)
    tar.addfile(info, io.BytesIO(data))
buf.seek(0)

dest = tempfile.mkdtemp()
with tarfile.open(fileobj=buf) as tar:
    try:
        tar.extractall(dest, filter='data')     # ✅ rejects absolute paths, '..', device files, unsafe links
    except tarfile.FilterError as e:
        print(type(e).__name__)                 # OutsideDestinationError
```

On 3.10 the default is still the unsafe legacy behavior (with no warning); 3.12 warns and 3.14 defaults to `'data'`. Never call `extractall()`/`extract()` on untrusted archives without `filter='data'`. Feature-detect with `hasattr(tarfile, 'data_filter')` if the code might run on a patch older than 3.10.12.

##### Input Validation, Secrets, SQL, Subprocess, XML

```python
import secrets
import sqlite3
import subprocess

def validate_port(value: object) -> int:
    match value:
        case bool():
            raise ValueError('port must not be a bool')
        case int(port) if 1 <= port <= 65535:
            return port
        case str() if value.isdigit():
            return validate_port(int(value))
        case _:
            raise ValueError(f'invalid port: {value!r}')

print(validate_port('8080'))                 # 8080

token = secrets.token_urlsafe(32)            # never random.* for secrets
print(secrets.compare_digest(token, token))  # constant-time comparison

conn = sqlite3.connect(':memory:')
conn.execute('CREATE TABLE users (id INTEGER, name TEXT)')
user_id = '1 OR 1=1'
rows = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,)).fetchall()   # ✅ parameterized
print(rows)                                   # []

result = subprocess.run(['echo', 'safe; rm -rf /'], capture_output=True, text=True,
                        encoding='utf-8', check=True, timeout=10)                # ✅ list form, no shell
print(result.stdout.strip())                  # safe; rm -rf /
```

For untrusted XML use `defusedxml`; never `eval()`/`exec()`/`pickle.load()` untrusted input; never `yaml.load()` without `SafeLoader`.

***

#### JSON and Serialization

```python
import json
from dataclasses import asdict, dataclass
from datetime import datetime, timezone

@dataclass
class Event:
    name: str
    at: datetime

class AppEncoder(json.JSONEncoder):
    def default(self, obj: object) -> object:
        match obj:
            case datetime():
                return obj.isoformat()
            case set() | frozenset():
                return sorted(obj)
            case _:
                return super().default(obj)

e = Event('deploy', datetime(2026, 1, 1, tzinfo=timezone.utc))
print(json.dumps(asdict(e), cls=AppEncoder))   # {"name": "deploy", "at": "2026-01-01T00:00:00+00:00"}
```

Huge integers in JSON are subject to the 3.10.7+ 4300-digit conversion limit (`json.loads` raises `ValueError`).

##### Pickle

```python
import pickle

print(pickle.DEFAULT_PROTOCOL, pickle.HIGHEST_PROTOCOL)   # 4 5
blob = pickle.dumps({'a': 1}, protocol=pickle.HIGHEST_PROTOCOL)
print(pickle.loads(blob))                                  # {'a': 1}
```

The default protocol is 4 (protocol 5 becomes the default only in 3.14). `shelve` now uses `pickle.DEFAULT_PROTOCOL` instead of protocol 3 (new in 3.10). **Never unpickle untrusted data** — it executes arbitrary code.

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
    print(row)          # {'name': 'Alice', 'age': '30'} — values are always str
```

For files: `open(path, 'r', encoding='utf-8', newline='')`. Values from `DictReader` are strings — convert explicitly (pattern matching with `case {'age': str(age)} if age.isdigit():` works well for row validation).

***

#### Regular Expressions

```python
import re

DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')

if m := DATE_RE.fullmatch('2026-10-21'):
    print(m['year'], m.group('month'), m.groupdict()['day'])   # 2026 10 21

print(re.findall(r'<.+?>', '<b>x</b>'))   # ['<b>', '</b>'] — non-greedy

# ❌ 3.11+ only: atomic groups and possessive quantifiers
for pattern in [r'(?>a+)b', r'a++b']:
    try:
        re.compile(pattern)
    except re.error as e:
        print('re.error:', pattern)
```

Pre-compile hot regexes at module level, prefer `fullmatch()` for validation (`match()` only anchors at the start), and avoid nested quantifiers on untrusted input (catastrophic backtracking — 3.10 has no atomic groups to defuse them; bound input length instead).

***

#### Memory Management

```python
import gc
import sys
import tracemalloc
import weakref

data = [1, 2, 3]
print(sys.getrefcount(data) >= 2)        # True — includes the temporary argument reference

tracemalloc.start()
blob = [bytes(1000) for _ in range(1000)]
current, peak = tracemalloc.get_traced_memory()
print(current > 1_000_000)               # True
tracemalloc.reset_peak()                 # 3.9+
tracemalloc.stop()

class Node:
    __slots__ = ('value', '__weakref__')  # slots save memory; add __weakref__ if weakrefs are needed
    def __init__(self, value: int) -> None:
        self.value = value

cache: weakref.WeakValueDictionary[int, Node] = weakref.WeakValueDictionary()
n = Node(1)
cache[1] = n
del n
gc.collect()
print(len(cache))                         # 0
```

`dataclass(slots=True)` (3.10) gives the memory benefit of `__slots__` without hand-writing them — but without `__weakref__` (see the dataclasses section).

***

#### Performance

3.10 is **not** the "Faster CPython" release (that is 3.11, with the specializing adaptive interpreter). 3.10's gains are targeted:

- `LOAD_ATTR` per-opcode cache: attribute reads ~36% faster, `__slots__` attributes ~44% faster.
- `str()`, `bytes()`, `bytearray()` constructors ~30–40% faster for small objects; `map`, `filter`, `reversed`, `bool`, `float` use vectorcall.
- Substring search (`in`, `find`) uses the Two-Way algorithm for long needles, avoiding quadratic worst cases.
- `python -m module` starts faster (fewer imports in `runpy`); stringized annotations make annotated function definitions about twice as fast to create.
- bz2/lzma/zlib decompression speedups; `BZ2File` dropped its internal lock and is **no longer thread-safe** for concurrent readers/writers (like `GzipFile`/`LZMAFile`).

```python
import cProfile
import io
import pstats

def work() -> int:
    return sum(i * i for i in range(10_000))

with cProfile.Profile() as pr:          # 3.8+
    work()

out = io.StringIO()
pstats.Stats(pr, stream=out).sort_stats('cumulative').print_stats(3)

VALID_ROLES = frozenset({'admin', 'editor', 'viewer'})   # O(1) membership
print('admin' in VALID_ROLES)                            # True
print(''.join(str(i) for i in range(5)))                 # 01234 — join, not += in a loop
```

Pattern matching is not a performance feature: a long `match` over constants is linear; a dict lookup is O(1).

***

#### Testing

```python
import asyncio
import logging
import unittest
from unittest.mock import AsyncMock, patch

logger = logging.getLogger('app')

def quiet() -> int:
    return 1

class TestThings(unittest.TestCase):
    def test_no_logs(self) -> None:
        with self.assertNoLogs('app', level='WARNING'):   # new in 3.10
            quiet()

    def test_logs(self) -> None:
        with self.assertLogs('app', level='INFO') as cm:
            logger.info('hello')
        self.assertEqual(cm.output, ['INFO:app:hello'])

class TestAsync(unittest.IsolatedAsyncioTestCase):        # 3.8+
    async def test_mock(self) -> None:
        fetch = AsyncMock(return_value={'id': 1})
        self.assertEqual(await fetch(1), {'id': 1})
        fetch.assert_awaited_once_with(1)

if __name__ == '__main__':
    unittest.main(argv=['x'], exit=False, verbosity=0)
```

Testing guidance specific to 3.10:

- Run the suite with `-X warn_default_encoding -W error::EncodingWarning` and `-W error::DeprecationWarning` to surface implicit encodings and 3.12 removals (distutils, `imp`, `asynchat`/`asyncore`/`smtpd`, `unittest` aliases like `assertEquals`) as well as still-pending deprecations such as the threading camelCase aliases.
- Tests asserting exact `SyntaxError`/`IndentationError` messages, `argparse` help text ("optional arguments:" became "options:"), or traceback text need updates.
- Use `pytest.mark.parametrize` with built-in generics and `X | Y` annotations freely.

```python
import pytest

@pytest.mark.parametrize(('raw', 'expected'), [('8080', 8080), (443, 443)])
def test_port(raw: str | int, expected: int) -> None:
    assert int(raw) == expected
```

***

#### Virtual Environments and Packaging

```bash
python3.10 -m venv .venv            # 3.10 venvs still get pip AND setuptools (3.12+ only pip)
source .venv/bin/activate
python -m pip install -r requirements.txt

uv venv --python 3.10               # uv can provision the (EOL) 3.10 interpreter itself
```

##### Deprecated in 3.10: `distutils` (PEP 632)

The whole `distutils` package emits `DeprecationWarning` on import in 3.10 and is **removed in 3.12**.

| `distutils` API | Replacement |
|---|---|
| `distutils.core.setup`, `Extension` | `setuptools.setup`, `setuptools.Extension` (better: declarative `pyproject.toml`) |
| `distutils.version.LooseVersion`/`StrictVersion` | `packaging.version.Version` |
| `distutils.util.strtobool` | a small local helper |
| `distutils.spawn.find_executable` | `shutil.which` |
| `distutils.dir_util.copy_tree` | `shutil.copytree(..., dirs_exist_ok=True)` |
| `distutils.sysconfig` | `sysconfig` |

**Pitfall:** when setuptools is installed, its `distutils-precedence.pth` hook makes `import distutils` return **setuptools' vendored copy**, which does not warn — so the deprecation is invisible in most virtualenvs, and the code then breaks on 3.12 environments without setuptools. Grep for `distutils` rather than relying on warnings.

```python
from packaging.version import Version   # third-party "packaging" — the standard replacement

print(Version('3.10.22') > Version('3.10.9'))   # True — LooseVersion-style string compares get this wrong
print('3.10.22' > '3.10.9')                     # False — never compare version strings lexicographically
```

##### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=64", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "myapp"
version = "1.0.0"
requires-python = ">=3.10"
dependencies = [
    "tomli>=2.0; python_version < '3.11'",   # tomllib is 3.11+
]

[tool.ruff]
target-version = "py310"

[tool.mypy]
python_version = "3.10"
strict = true

[tool.pytest.ini_options]
addopts = "-W error::DeprecationWarning"
```

Reading TOML on 3.10 requires the `tomli` backport (the stdlib `tomllib` is 3.11+):

```python
import sys

if sys.version_info >= (3, 11):
    import tomllib
else:
    import tomli as tomllib   # pip install tomli

config = tomllib.loads('name = "app"\nport = 8080\n')
print(config)   # {'name': 'app', 'port': 8080}
```

##### Other Packaging-Relevant 3.10 Changes

- `importlib.metadata.entry_points()` gained a selection API: `entry_points(group='console_scripts')` returns an `EntryPoints` object; the old dict-style `entry_points()['group']` access is deprecated. `importlib.metadata.packages_distributions()` is new.
- `sys.stdlib_module_names` (frozenset of stdlib module names) and `sys.orig_argv` are new.
- Old `load_module()`/`find_module()` import hooks emit `ImportWarning`/`DeprecationWarning` (removed in 3.12) — port custom importers to `find_spec()`/`exec_module()`.

```python
import importlib.metadata
import sys

print('json' in sys.stdlib_module_names)                 # True
eps = importlib.metadata.entry_points(group='console_scripts')
print(type(eps).__name__)                                 # EntryPoints
```

***

#### Code Quality Tools

```bash
pip install ruff mypy

ruff check --target-version py310 src/        # lint (pyupgrade "UP" rules: Optional[X] -> X | None, etc.)
ruff format src/                               # formatter (Black-compatible)
mypy --python-version 3.10 --strict src/
```

- Set the **target version to 3.10** everywhere (`ruff`, `black --target-version py310`, `mypy --python-version 3.10`, `pyupgrade --py310-plus`). A higher target makes auto-fixers rewrite code into 3.11+/3.12+ syntax that crashes on the 3.10 interpreter; a lower one keeps `Optional[...]`/`Union[...]` noise.
- Ruff's `UP007` (`Union[X, Y]` → `X | Y`) and `UP045` (`Optional[X]` → `X | None`; a separate rule in current Ruff, added in 0.12.0), like `pyupgrade --py310-plus`, rewrite annotations to PEP 604 syntax — enable them only once the project has dropped 3.9.
- Type checkers flag misuse of `ParamSpec`, `TypeGuard` and non-exhaustive `match` on enums/unions (with `assert_never` from `typing_extensions`); Python itself does not.
- `bandit -r src/` for common security smells; `pip-audit` for known-vulnerable dependencies.

***

#### Removed and Changed in 3.10 — Migration Notes

**Removed in 3.10 (code that worked on 3.9 breaks):**

| Removed | Replacement |
|---|---|
| ABC aliases in `collections` (`collections.Mapping`, …) | `collections.abc` |
| `loop=` parameter of high-level `asyncio` APIs | omit it; `asyncio.run_coroutine_threadsafe()` for cross-thread |
| `parser` module | `ast` |
| `symbol` module | `token`/`ast` |
| `formatter` module | copy the code you need |
| `-X oldparser` / `PYTHONOLDPARSER` (LL(1) parser) | — (PEG parser only) |
| `distutils` `bdist_wininst` command | `bdist_wheel` |
| `complex.__int__`, `__float__`, `__floordiv__`, `__mod__`, `__divmod__` (+ reflected) | they always raised `TypeError` — use `.real`/`abs()` |
| Py_UNICODE encoder C APIs (PEP 624) | PEP 393 APIs (C extensions only) |

**Behavior changes that can bite:**

- Builtins and C functions taking integers **no longer accept `Decimal`, `Fraction` or other objects with only `__int__`** (they require `__index__`): `range(Decimal(3))` or `'x' * Fraction(2)` raise `TypeError`.
- `traceback.format_exception()`/`print_exception()` parameter `etype` renamed to `exc` (keyword callers break).
- Invalid `collections.abc.Callable` parameterizations (`Callable[int, str]`) now raise `TypeError` (3.9 accepted them); the flattened `__args__` already arrived in 3.9.2.
- `socket.htons()`/`ntohs()` raise `OverflowError` for values outside 16 bits (was a `DeprecationWarning`).
- `argparse` help says "options:" instead of "optional arguments:" (snapshot tests).
- `atexit` callbacks that fail are now always logged.
- Class/module `__annotations__` are created lazily per object: `Sub.__annotations__` no longer returns the base class's dict.
- `typing.Protocol` with only data members raises `TypeError` in `isinstance()` unless `@runtime_checkable`.
- `types.FunctionType(code, globals)` inherits the current builtins when `globals` lacks `__builtins__`.
- NaN hashes depend on object identity.
- Point releases: int/str digit limit (3.10.7); `get_event_loop()` warnings appear (3.10.0–3.10.9) and disappear (3.10.10+).

```python
from decimal import Decimal

try:
    range(Decimal(3))           # ❌ 3.10: TypeError ('decimal.Decimal' object cannot be interpreted as an integer)
except TypeError as e:
    print(type(e).__name__)
print(list(range(int(Decimal(3)))))   # ✅ explicit conversion: [0, 1, 2]
```

**Deprecated in 3.10 — removed in 3.12 (fix now, before the next upgrade):** `distutils`; `imp`; `asynchat`, `asyncore`, `smtpd` (import-time `DeprecationWarning`); `pathlib.Path.link_to()`; `importlib` `find_module()`/`find_loader()`/`load_module()`/`module_repr()`; `sqlite3.OptimizedUnicode` and `enable_shared_cache`; `configparser.SafeConfigParser`; `unittest` aliases (`assertEquals`, …); `ssl.wrap_socket()`/`match_hostname()`. Also deprecated in 3.10: `cgi.log()` (still present in 3.12; the whole `cgi` module is removed in 3.13), threading camelCase aliases (still present, still warning, through 3.15), non-integer arguments to `random.randrange()`, numeric literals immediately followed by keywords, the pure-Python `hashlib.pbkdf2_hmac` fallback.

**Note:** What's New 3.10 states that importing from `typing.io`/`typing.re` emits a `DeprecationWarning`; the 3.10 interpreter actually does not (the warning appears in 3.11; the submodules are gone in 3.13). Import `IO`, `Pattern`, `Match` from `typing` (or `re`) directly anyway.

***

#### Production Checklist

##### Python 3.10-Specific Items

- [ ] Running **3.10.22** (final release) — and an upgrade to a supported version is scheduled: 3.10 is EOL since 2026-10-01 and gets no more security fixes
- [ ] No 3.11+ APIs or syntax: no `ExceptionGroup`/`except*`/`add_note`, `tomllib`, `TaskGroup`, `asyncio.timeout`, `typing.Self`/`Never`/`assert_never`/`Required`/`override`, `StrEnum`, `datetime.UTC`, `itertools.batched`, `type X = ...`, PEP 695 generics, PEP 701 f-strings, `except A, B:`
- [ ] All ABC imports come from `collections.abc` (aliases removed in 3.10)
- [ ] No `loop=` arguments to high-level asyncio APIs; no `get_event_loop()` in new code — `asyncio.run()` + `get_running_loop()`
- [ ] Timeouts from `asyncio.wait_for()` caught as `asyncio.TimeoutError` (not the builtin `TimeoutError` on 3.10); futures timeouts as `concurrent.futures.TimeoutError`
- [ ] `match` statements: constants matched via dotted names/enums, never bare names; type tests written `int()`, not `int`; `bool()` before `int()`; explicit `case _: raise` where every value must be handled
- [ ] Sequence patterns never expected to match `str`/`bytes` or iterators; mapping patterns' "extra keys ignored" semantics intended
- [ ] Classes used in positional class patterns define `__match_args__` (a tuple) or are dataclasses/namedtuples
- [ ] `X | Y` annotations never have a bare string operand (`'Node' | None` → `'Node | None'`); union introspection checks both `typing.Union` and `types.UnionType`
- [ ] `isinstance()` never given parameterized generics (`list[int]`), with or without `|`
- [ ] `zip(..., strict=True)` used where iterables must have equal length — remembering it raises only at exhaustion
- [ ] `dataclass(slots=True)` classes don't use zero-arg `super()`, `cached_property` or weak references; `kw_only`/`KW_ONLY` used for wide records
- [ ] Decorators typed with `ParamSpec`/`Concatenate` instead of `Callable[..., Any]`; `TypeGuard` functions check everything they claim
- [ ] f-strings with `(str, Enum)`/`IntEnum` members use `.value` explicitly (format output changes in 3.11)
- [ ] CI runs with `-X warn_default_encoding -W error::EncodingWarning`; every text `open()` has `encoding=`
- [ ] `distutils` usages removed (grep — setuptools' shim hides the warning); `LooseVersion` → `packaging.version.Version`
- [ ] Other 3.12 removals cleaned up: `imp`, `asynchat`/`asyncore`/`smtpd`, `Path.link_to`, `SafeConfigParser`, `assertEquals`-style aliases, `ssl.wrap_socket`; deprecated threading camelCase aliases replaced too
- [ ] `ssl` contexts built with `create_default_context()` or `PROTOCOL_TLS_CLIENT/SERVER`; TLS floor raised via `minimum_version`, not `OP_NO_*`; servers verified to support TLS 1.2+
- [ ] Interpreter linked against OpenSSL ≥ 1.1.1 (`ssl.OPENSSL_VERSION`)
- [ ] `tarfile.extractall()`/`shutil.unpack_archive()` on untrusted archives use `filter='data'`
- [ ] Very large decimal integer parsing accounts for the 4300-digit limit (3.10.7+)
- [ ] Coverage thresholds re-baselined after the PEP 626 line-number changes

##### Code Correctness

- [ ] `bytes` and `str` never mixed without explicit encode/decode
- [ ] `newline=''` passed to `open()` for CSV files
- [ ] No mutable default arguments; dataclass mutable fields use `field(default_factory=...)`
- [ ] `__hash__` defined alongside `__eq__` only on immutable classes
- [ ] No mutation of a dict/list while iterating it
- [ ] `asyncio.gather()` failures cancel sibling tasks explicitly (no `TaskGroup` on 3.10)

##### Safety and Security

- [ ] SQL queries parameterized — never f-string interpolation
- [ ] `pickle.load()` never called on untrusted data
- [ ] `shell=True` never used with user-controlled input
- [ ] `eval()`/`exec()` never called on untrusted input
- [ ] `secrets` (not `random`) for tokens; `secrets.compare_digest()` for comparisons
- [ ] `defusedxml` for untrusted XML
- [ ] Paths from users resolved and checked with `is_relative_to()`

##### Performance

- [ ] Profiled before optimizing
- [ ] Long `match` statements over constants replaced by dict dispatch on hot paths
- [ ] Generator expressions for single-pass pipelines; `set`/`frozenset` for membership
- [ ] `str.join()` for string building; `%`-style lazy logging arguments
- [ ] Blocking calls wrapped in `asyncio.to_thread()` inside async code

##### Observability

- [ ] `logging.getLogger(__name__)` in every module; no `print()` in production paths
- [ ] `logger.exception()` inside `except` blocks; bare `raise` to re-raise
- [ ] Custom exceptions inherit from `Exception`, with explicit `raise ... from ...` chaining
- [ ] Error reports do not rely on "Did you mean" hints (only shown by the default excepthook in 3.10)
