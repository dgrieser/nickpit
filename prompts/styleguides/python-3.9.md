# Python 3.9: The Complete Production Developer Guide

## Overview

Python 3.9 was released on October 5, 2020 — the first release on the annual release cadence (PEP 602) — and reached end-of-life on October 31, 2025 with the final release 3.9.25; plan upgrades accordingly.[^1] It introduced a focused set of high-impact language changes: dict merge/update operators, built-in generic type hints, `str.removeprefix`/`removesuffix`, the `zoneinfo` standard library module for IANA time zones, `graphlib` for dependency resolution, relaxed decorator grammar, `typing.Annotated`, and a new PEG-based parser that sets the stage for future language features. On top of that, Python 3.9 finalizes the removal of a large set of long-deprecated Python-2-era APIs (see the migration notes near the end).[^1]

This guide targets Python 3.9 specifically. Features introduced in earlier versions (3.6 f-strings, 3.7 dataclasses, 3.8 walrus operator, 3.8 positional-only parameters) are documented here for completeness — they remain fully available in 3.9. Every section that covers a 3.9-specific change is marked **New in 3.9**. The guide does not cover features from 3.10 or later: no `match`/`case` pattern matching, no `X | Y` union syntax, no officially supported parenthesized multi-item `with`, no `zip(strict=True)`, no `dataclass(slots=True)`/`KW_ONLY`, no `typing.ParamSpec`/`typing.TypeAlias` (all 3.10), no `typing.Self` or `ExceptionGroup` (3.11).

Global truths about Python 3.9:

- **Everything is an object.** Integers, functions, classes, modules — all are objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing.** Variables carry no declared type; the type lives in the value.
- **Reference counting + cyclic GC.** CPython manages memory with reference counts plus a cyclic garbage collector.
- **`str` is Unicode everywhere.** All string literals are Unicode text; `bytes` is the separate binary type.
- **Dicts are ordered by insertion.** Guaranteed by the language spec since Python 3.7.[^2]
- **`collections.Mapping` et al. are on their way out.** Importing ABCs directly from `collections` (rather than `collections.abc`) emits `DeprecationWarning` in 3.9 and stops working entirely in 3.10. Fix now or your 3.9 codebase will not run on 3.10.[^3]

***

## PEP 8 Code Style

### Naming Conventions

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
_module_cache: dict = {}
```

In Python 3, all classes implicitly inherit from `object`. `class Foo:` and `class Foo(object):` are identical. Never write the latter.[^4]

### Import Ordering

Three blocks separated by blank lines: standard library, third-party, local. Alphabetize within each group. `isort --profile=black` automates this.

```python
# Standard library
import os
import sys
from pathlib import Path
from typing import Optional, Union  # still needed in 3.9 for Union/Optional

# Third-party
import requests

# Local application
from myapp.models import User
from myapp.utils import format_date

# Never wildcard imports — they defeat static analysis and __all__ hygiene
# Bad:  from module import *
# Good: from module import specific_item
```

### Docstrings

Write Google-style docstrings for every public module, class, and function. First line: a one-sentence imperative summary.

```python
from typing import Optional

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

    def __init__(self, db: 'DatabaseConnection', cache: Optional['Cache'] = None) -> None:
        self.db = db
        self.cache = cache
```

***

## New in 3.9: Built-in Generic Type Hints (PEP 585)

This is one of the most practically important changes in Python 3.9. You can now use built-in collection types **directly as generics in type annotations**, without importing capitalized equivalents from `typing`.[^5]

```python
# Python 3.8 and earlier — required typing imports
from typing import Dict, FrozenSet, List, Optional, Set, Tuple, Type

def process(items: List[str]) -> Dict[str, int]:
    ...

# Python 3.9+ — use built-in types directly
def process(items: list[str]) -> dict[str, int]:
    ...

def scores(data: list[tuple[str, int]]) -> dict[str, list[int]]:
    ...

def find(name: str) -> Optional[int]:   # Optional still from typing — see below
    ...
```

### Full Mapping: Old → New in 3.9

| `typing` form (3.8 and earlier) | Built-in form (3.9+) |
|---|---|
| `typing.List[X]` | `list[X]` |
| `typing.Dict[K, V]` | `dict[K, V]` |
| `typing.Set[X]` | `set[X]` |
| `typing.FrozenSet[X]` | `frozenset[X]` |
| `typing.Tuple[X, Y]` | `tuple[X, Y]` |
| `typing.Type[X]` | `type[X]` |
| `typing.Deque[X]` | `collections.deque[X]` |
| `typing.DefaultDict[K, V]` | `collections.defaultdict[K, V]` |
| `typing.OrderedDict[K, V]` | `collections.OrderedDict[K, V]` |
| `typing.Counter[X]` | `collections.Counter[X]` |
| `typing.ChainMap[K, V]` | `collections.ChainMap[K, V]` |
| `typing.Pattern[str]` | `re.Pattern[str]` |
| `typing.Match[str]` | `re.Match[str]` |
| `typing.Awaitable[X]` | `collections.abc.Awaitable[X]` |
| `typing.Iterable[X]` | `collections.abc.Iterable[X]` |
| `typing.Iterator[X]` | `collections.abc.Iterator[X]` |
| `typing.Generator[Y, S, R]` | `collections.abc.Generator[Y, S, R]` |
| `typing.Callable[[A], R]` | `collections.abc.Callable[[A], R]` |
| `typing.Sequence[X]` | `collections.abc.Sequence[X]` |
| `typing.Mapping[K, V]` | `collections.abc.Mapping[K, V]` |
| `typing.MutableMapping[K, V]` | `collections.abc.MutableMapping[K, V]` |

### What Still Requires `typing` in 3.9

```python
from typing import (
    Annotated,      # new in 3.9 — but lives in typing
    Any,
    ClassVar,
    Final,
    Generic,
    Literal,
    NamedTuple,
    NewType,
    NoReturn,
    Optional,       # Optional[X] is Union[X, None] — still from typing
    Protocol,
    TypedDict,
    TypeVar,
    Union,          # Union[X, Y] — still from typing (| syntax is 3.10+)
    cast,
    final,
    get_type_hints,
    overload,
    runtime_checkable,
)
```

**Note on `Optional` and `Union`:** The `X | Y` union syntax (`int | None` instead of `Optional[int]`) is **Python 3.10+** only. In 3.9, continue using `Optional[X]` and `Union[X, Y]`.

**Pitfall: subscripted built-ins are not valid at runtime for `isinstance()`:**

```python
# VALID — type annotation
def f(x: list[int]) -> None:
    pass

# VALID at runtime — generic alias object (types.GenericAlias)
alias = list[int]
print(alias.__origin__)  # <class 'list'>
print(alias.__args__)    # (<class 'int'>,)

# INVALID — cannot use in isinstance() at runtime
isinstance([1, 2], list[int])  # TypeError: isinstance() argument 2 cannot be a parameterized generic
isinstance([1, 2], list)       # CORRECT — use the bare type
```

**Pitfall: the old `typing.List` etc. still work in 3.9** but are **deprecated as of 3.9**. Per PEP 585 the deprecation deliberately emits **no runtime `DeprecationWarning`** — only type checkers and linters flag it. Migrate now anyway; removal is planned for after 3.9's end-of-life.

**Point-release note:** `collections.abc.Callable[[int, str], str]` only flattens its `__args__` the way `typing.Callable` does since **3.9.2** — 3.9.0/3.9.1 behave differently for code that introspects `__args__`.

***

## New in 3.9: Dict Merge and Update Operators (PEP 584)

Python 3.9 adds `|` (merge, returns new dict) and `|=` (update in-place) to `dict`.[^6]

```python
defaults = {'host': 'localhost', 'port': 5432, 'timeout': 30}
overrides = {'port': 9432, 'debug': True}

# | — creates a new dict, does NOT modify either operand
# right-hand side wins on key collision
merged = defaults | overrides
# {'host': 'localhost', 'port': 9432, 'timeout': 30, 'debug': True}

print(defaults)   # unchanged: {'host': 'localhost', 'port': 5432, 'timeout': 30}
print(overrides)  # unchanged: {'port': 9432, 'debug': True}

# |= — updates left operand in-place
config = dict(defaults)
config |= overrides
# config is now {'host': 'localhost', 'port': 9432, 'timeout': 30, 'debug': True}

# Chaining
result = base | layer1 | layer2 | layer3   # rightmost wins

# Common pattern: config with per-call overrides
def connect(host: str, port: int = 5432, **kwargs) -> 'Connection':
    base_opts = {'connect_timeout': 5, 'application_name': 'myapp'}
    opts = base_opts | kwargs
    return make_connection(host, port, **opts)
```

### `|` vs `{**a, **b}` — Differences

```python
# | requires both operands to be dicts (or dict subclasses)
# {**a, **b} accepts arbitrary mappings
# They produce identical results for plain dicts

# Stdlib dict subclasses override the operators to preserve their type:
from collections import OrderedDict, defaultdict
od = OrderedDict(a=1, b=2)
print(type(od | {'c': 3}))   # OrderedDict

dd = defaultdict(list, {'a': [1]})
merged = dd | {'b': [2]}     # defaultdict — keeps the LEFT operand's default_factory
print(type(merged), merged.default_factory)   # <class 'collections.defaultdict'> <class 'list'>

# But a plain dict subclass WITHOUT its own __or__ returns a plain dict:
class MyDict(dict): ...
print(type(MyDict(a=1) | {'b': 2}))   # dict — NOT MyDict

# {**od, 'c': 3} always produces a plain dict
print(type({**od, 'c': 3}))  # dict

# |= accepts any mapping or iterable of key-value pairs (like dict.update)
d = {'a': 1}
d |= [('b', 2), ('c', 3)]   # valid — list of tuples
d |= (('d', 4),)             # valid — tuple of tuples
```

**Pitfall: `|=` accepts iterables of pairs, but `|` (non-augmented) only accepts dicts on both sides:**

```python
d = {'a': 1}
d |= [('b', 2)]     # VALID — |= accepts iterables
e = d | [('c', 3)]  # TypeError — | only accepts dicts
```

**Pitfall: `collections.Counter` has its own `|` — multiset maximum, NOT a merge.** `Counter.__or__` predates PEP 584 and computes the element-wise **max of counts**:

```python
from collections import Counter
Counter(a=3, b=1) | Counter(a=1, b=5)   # Counter({'b': 5, 'a': 3}) — max, not last-wins
```

Never use `|` on `Counter` objects expecting PEP 584 merge semantics — use `{**c1, **c2}` or `dict(c1) | dict(c2)` if you really want last-wins.

***

## New in 3.9: `str.removeprefix()` and `str.removesuffix()` (PEP 616)

Two long-requested string methods that do exactly one thing, clearly.[^7]

```python
s = 'Hello, World!'

# removeprefix — removes the prefix if present, otherwise returns unchanged string
print(s.removeprefix('Hello, '))   # 'World!'
print(s.removeprefix('Goodbye'))   # 'Hello, World!' — unchanged, no error

# removesuffix — removes the suffix if present, otherwise returns unchanged string
print(s.removesuffix('!'))         # 'Hello, World'
print(s.removesuffix('?'))         # 'Hello, World!' — unchanged

# Works on bytes and bytearray too
b = b'  binary data  '
print(b.removeprefix(b'  '))   # b'binary data  '
print(b.removesuffix(b'  '))   # b'  binary data'
```

### Critical Difference from `str.strip()`

`strip()` removes **all characters in a set, from either end, repeatedly**. `removeprefix`/`removesuffix` remove **exactly one occurrence of the specified substring**, if it is present at the start/end.

```python
s = '---hello---'

# strip removes ALL leading/trailing '-' characters
print(s.strip('-'))          # 'hello'

# removeprefix removes ONLY the exact prefix once
print(s.removeprefix('---')) # 'hello---'  — only the prefix removed
print(s.removeprefix('-'))   # '--hello---' — removes only one '-'

# Real-world pitfall: stripping a URL scheme
url = 'https://example.com'
bad  = url.lstrip('https://')   # 'example.com' — WRONG: strips any of h,t,p,s,:,/
good = url.removeprefix('https://')  # 'example.com' — CORRECT and unambiguous

# Another pitfall: lstrip('<xml>') on an XML tag
tag = '<xml>content</xml>'
tag.lstrip('<xml>')         # 'content</xml>' — strips ALL of <, x, m, l, > from the left!
tag.removeprefix('<xml>')   # 'content</xml>' — removes exactly '<xml>', once, by design
```

**Rule:** Whenever you intend to remove a specific prefix or suffix string (not a set of characters), always use `removeprefix`/`removesuffix`. Never use `lstrip`/`rstrip` for that purpose.

### Practical Patterns

```python
# File extension stripping (though Path.stem is cleaner)
filename = 'report.csv'
name = filename.removesuffix('.csv')   # 'report'

# Protocol stripping
def normalize_url(url: str) -> str:
    return url.removeprefix('https://').removeprefix('http://')

# Namespace removal
def strip_ns(tag: str, ns: str) -> str:
    return tag.removeprefix(f'{{{ns}}}')

# Django/Flask route normalization
path = '/api/v1/users/'
path = path.removeprefix('/api').removesuffix('/')
```

***

## Primitive Types and Numerics

### Integers

Python 3.9 has a single integer type: `int`. It is **arbitrary precision** — there is no separate `long`. Arithmetic never overflows; Python allocates more memory silently.[^4]

```python
import sys
print(sys.maxsize)     # 9223372036854775807 on 64-bit (C ssize_t max, NOT the int limit)
x = 10 ** 1000         # perfectly fine — arbitrary precision
print(type(x))         # <class 'int'>

one_million = 1_000_000   # underscores in numeric literals since 3.6
hex_mask    = 0xFF_FF_FF_FF
binary_val  = 0b_1001_0111_1010
```

```python
import math
# math.gcd now accepts any number of arguments (new in 3.9) — was exactly two
print(math.gcd(12, 18, 24))    # 6
print(math.gcd())              # 0 — zero arguments allowed

# math.lcm — new in 3.9, also variadic
print(math.lcm(4, 6))          # 12
print(math.lcm(4, 6, 10))      # 60
print(math.lcm(0, 5))          # 0 — zero if any argument is 0; lcm() with no args is 1

# math.isqrt — exact integer square root (from 3.8, still recommended)
# int(math.sqrt(n)) breaks for large ints due to float rounding:
n = 4503599761588224**2 - 1
print(math.isqrt(n))       # 4503599761588223 — correct
print(int(math.sqrt(n)))   # 4503599761588224 — WRONG, float precision error

# pow() modular inverse (from 3.8)
pow(38, -1, 137)   # 119
```

### Floats

Floats are C `double` (64-bit IEEE 754). New `math` additions in 3.9:[^4]

```python
import math

# math.nextafter — next representable float towards y
print(math.nextafter(0.0, 1.0))    # 5e-324 (smallest positive float)
print(math.nextafter(1.0, 0.0))    # 0.9999999999999999

# math.ulp — unit in the last place (precision at a given value)
print(math.ulp(1.0))    # 2.220446049250313e-16
print(math.ulp(0.0))    # 5e-324

# Float comparison — always use math.isclose
import math
assert math.isclose(0.1 + 0.2, 0.3)
assert math.isclose(a, b, rel_tol=1e-9, abs_tol=1e-12)
```

**`math.nextafter` and `math.ulp` are new in 3.9.** They are indispensable for numerical algorithms that need to step through IEEE 754 floats, test for maximum precision, or implement robust interval arithmetic.

### Booleans

`bool` is a subtype of `int`. `True == 1` and `False == 0`.[^4]

```python
print(isinstance(True, int))  # True
print(True + True)            # 2
print(True * 5)               # 5
```

**Falsy values:** `None`, `False`, zero of any numeric type (`0`, `0.0`, `0j`), empty sequences (`''`, `b''`, `()`, `[]`), empty mapping (`{}`), empty set (`set()`), and instances whose `__bool__` returns `False` or `__len__` returns `0`.

***

## Strings

### The Two Types

- **`str`**: A sequence of Unicode code points. All undecorated string literals are `str`.
- **`bytes`**: A sequence of raw 8-bit values. Declared with `b''` prefix. **Never mixed implicitly with `str`.**

```python
text = 'hello'    # str — Unicode
data = b'hello'   # bytes — binary
# text + data     # TypeError in Python 3 — always
```

**The Unicode sandwich rule:**
1. Decode `bytes` → `str` at input boundaries.
2. Work entirely in `str` internally.
3. Encode `str` → `bytes` at output boundaries.

Always pass `encoding='utf-8'` to `open()` — the default is the platform locale, which is `cp1252` on many Windows systems.

### String Methods (Complete Reference)

```python
s = 'Hello, World!'

# Case
s.upper()           # 'HELLO, WORLD!'
s.lower()           # 'hello, world!'
s.casefold()        # aggressive lowercase — use for case-insensitive comparisons
s.capitalize()      # 'Hello, world!'
s.title()           # 'Hello, World!'
s.swapcase()        # 'hELLO, wORLD!'

# Whitespace and stripping
'  hi  '.strip()         # 'hi'
'  hi  '.lstrip()        # 'hi  '
'  hi  '.rstrip()        # '  hi'
'xxhello'.lstrip('x')    # 'hello' — strips ALL 'x' chars from left
'hellox'.rstrip('x')     # 'hello'

# New in 3.9 — unambiguous prefix/suffix removal
'https://x.com'.removeprefix('https://')  # 'x.com'
'file.tar.gz'.removesuffix('.gz')         # 'file.tar'

# Search
s.find('World')        # 7  — returns -1 if not found
s.index('World')       # 7  — raises ValueError if not found
s.rfind('l')           # 10 — rightmost
s.count('l')           # 3
s.startswith('Hello')  # True
s.endswith('!')        # True
s.startswith(('Hello', 'Hi'))  # accepts tuple of prefixes

# Split and join
s.split(', ')                      # ['Hello', 'World!']
s.split()                          # splits on any whitespace, strips empties
s.rsplit(', ', maxsplit=1)         # ['Hello', 'World!']
s.splitlines()                     # ['Hello, World!']
', '.join(['a', 'b', 'c'])         # 'a, b, c'
s.partition(', ')                  # ('Hello', ', ', 'World!')
s.rpartition(', ')                 # ('Hello', ', ', 'World!')

# Replace
s.replace('World', 'Python')       # 'Hello, Python!'
s.replace('l', 'L', 2)            # 'HeLLo, World!' — max 2 replacements

# Justify and fill
'hi'.center(10, '-')   # '----hi----'
'hi'.ljust(10, '.')    # 'hi........'
'hi'.rjust(10, '.')    # '........hi'
'42'.zfill(6)          # '000042'

# Test
'abc'.isalpha()    # True
'123'.isdigit()    # True — only decimal digits
'123'.isnumeric()  # True — also ² ³ etc.
'abc123'.isalnum() # True
'  '.isspace()     # True
'HELLO'.isupper()  # True
'hello'.islower()  # True
'Hello World'.istitle()  # True

# Encode/decode
'café'.encode('utf-8')           # b'caf\xc3\xa9'
b'caf\xc3\xa9'.decode('utf-8')  # 'café'
```

### f-strings and String Formatting

```python
name, score = 'Alice', 98.765

# f-strings (since 3.6)
print(f'Hello, {name}!')
print(f'{score:.2f}')                   # '98.76'
print(f'{1_000_000:_}')                 # '1_000_000'
print(f'{"left":<10}|')                 # 'left      |'
print(f'{255:#x}')                      # '0xff'

# f-string = specifier (3.8+) — debugging only, never in production logs
x = 42
print(f'{x=}')               # x=42
print(f'{x * 2 = }')         # x * 2 = 84

# str.format()
'{} and {}'.format('a', 'b')
'{name} is {age}'.format(name='Alice', age=30)
'{0:.2f}'.format(3.14159)

# % operator — legacy, still fully functional
'%s is %d' % ('Alice', 30)
'%.2f' % 3.14159

# Pitfall in 3.9: backslashes cannot appear inside f-string expressions, and
# the outer quote type cannot be reused inside {} (both fixed in 3.12, NOT in 3.9)
d = {'key': 'value'}
# f'{d['key']}'         # SyntaxError — same quote type inside and out
# f'{"\n".join(x)}'     # SyntaxError — backslash in expression
f"Value: {d['key']}"    # CORRECT — switch the outer quote type
key = 'key'
f'Value: {d[key]}'      # also CORRECT — hoist into a variable
nl = '\n'
f'lines: {nl.join(items)}'   # workaround for backslashes
```

***

## Variable Annotations and Type Hints

### PEP 585 in Practice (3.9+)

```python
from __future__ import annotations  # optional in 3.9; postpones evaluation

# In 3.9: use built-in generics directly in annotations
def process(
    items: list[str],
    mapping: dict[str, int],
    matrix: list[list[float]],
    pairs: list[tuple[str, int]],
) -> dict[str, list[int]]:
    ...

# Class annotations
class Pipeline:
    stages: list[str]
    config: dict[str, object]
    handlers: dict[str, collections.abc.Callable[..., None]]

    def __init__(self) -> None:
        self.stages = []
        self.config = {}
        self.handlers = {}
```

**`from __future__ import annotations` in Python 3.9:**  This PEP 563 import postpones annotation evaluation, turning all annotations into strings. It is useful when:
- You have forward references that would otherwise fail at module load time.
- You want `X | Y` union syntax (runtime-valid only in 3.10+) to at least *parse* in 3.9.

**Caveat on `X | Y` with the future import:** the annotation is stored as a string, so anything that **evaluates** annotations at runtime — `typing.get_type_hints()`, dataclass introspection, pydantic, FastAPI — still raises `TypeError` on 3.9. The safe rule for 3.9 code: **keep writing `Optional[X]` / `Union[X, Y]`**; reserve the future import for forward references.

### New in 3.9: `typing.Annotated` (PEP 593)

`Annotated` attaches arbitrary metadata to a type annotation. The first argument is the actual type; subsequent arguments are metadata.[^8]

```python
from typing import Annotated, get_type_hints

# Annotated[T, metadata...]
# The type checker sees only T; the metadata is accessible at runtime

# Define reusable annotated types for validation, serialization, etc.
NonEmptyStr = Annotated[str, 'non-empty']
PositiveInt = Annotated[int, 'gt:0']
Port        = Annotated[int, 'range:1-65535']

class User:
    name: Annotated[str, 'max_length:100']
    age:  Annotated[int, 'ge:0', 'le:150']
    role: Annotated[str, 'choices:admin,user,guest']
```

**Accessing metadata at runtime:**

```python
from typing import Annotated, get_type_hints, get_args, get_origin

def validate_from_annotations(cls: type, data: dict) -> dict:
    hints = get_type_hints(cls, include_extras=True)
    errors = {}
    for field, hint in hints.items():
        if get_origin(hint) is Annotated:
            base_type, *metadata = get_args(hint)
            value = data.get(field)
            if not isinstance(value, base_type):
                errors[field] = f'Expected {base_type.__name__}'
                continue
            for m in metadata:
                if isinstance(m, str) and m == 'non-empty':
                    if not value:
                        errors[field] = 'Must not be empty'
    return errors

# Use include_extras=True to preserve Annotated metadata in get_type_hints
hints = get_type_hints(User, include_extras=True)
# Without include_extras=True, Annotated is stripped — only base type returned
```

**Practical use case — building a lightweight validator:**

```python
from dataclasses import dataclass
from typing import Annotated, Optional, get_type_hints, get_origin, get_args

class MinLen:
    def __init__(self, n: int): self.n = n
    def validate(self, v: str) -> Optional[str]:
        return f'Minimum length is {self.n}' if len(v) < self.n else None

class MaxVal:
    def __init__(self, n: int): self.n = n
    def validate(self, v: int) -> Optional[str]:
        return f'Must be <= {self.n}' if v > self.n else None

@dataclass
class Config:
    host: Annotated[str, MinLen(1)]
    port: Annotated[int, MaxVal(65535)]

def validate(obj: object) -> list[str]:
    errors: list[str] = []
    hints = get_type_hints(type(obj), include_extras=True)
    for name, hint in hints.items():
        if get_origin(hint) is Annotated:
            _, *validators = get_args(hint)
            for v in validators:
                if err := v.validate(getattr(obj, name)):
                    errors.append(f'{name}: {err}')
    return errors
```

### `TypedDict`, `Literal`, `Final`, `Protocol` (from 3.8)

All four remain fully available in 3.9. See the Python 3.8 guide for their complete documentation. In 3.9, `TypedDict` and `Protocol` fields can use built-in generics rather than `typing` versions:

```python
from typing import TypedDict, Protocol

class User(TypedDict):
    name: str
    tags: list[str]       # list[str] in 3.9 — no typing.List needed
    config: dict[str, int]  # dict[str, int] in 3.9

class Drawable(Protocol):
    def draw(self, canvas: list[tuple[int, int]]) -> None:  # built-in generics
        ...
```

**`Literal` point-release fix (3.9.1):** `Literal` de-duplicates parameters, equality ignores parameter order, and comparisons respect types (`Literal[0] != Literal[False]`) only **since 3.9.1** — 3.9.0 has the old, buggy behavior. One more reason to run the latest 3.9.x patch release.

***

## New in 3.9: `zoneinfo` — IANA Time Zone Support (PEP 615)

Python 3.9 adds the `zoneinfo` module, which provides a concrete, DST-aware time zone implementation backed by the IANA time zone database. This replaces the long-standing dependence on the third-party `pytz` library.[^9]

```python
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError, available_timezones
from datetime import datetime, timedelta, timezone

# Create aware datetime in a named IANA zone
dt_ny    = datetime(2024, 6, 15, 12, 0, tzinfo=ZoneInfo('America/New_York'))
dt_tokyo = datetime(2024, 6, 15, 12, 0, tzinfo=ZoneInfo('Asia/Tokyo'))
dt_utc   = datetime(2024, 6, 15, 12, 0, tzinfo=timezone.utc)

# Convert between zones
dt_berlin = dt_ny.astimezone(ZoneInfo('Europe/Berlin'))
print(dt_berlin)

# Get current local time in a specific zone
now_la = datetime.now(ZoneInfo('America/Los_Angeles'))
print(f'LA: {now_la:%Y-%m-%d %H:%M %Z %z}')

# Enumerate all known zone names
zones = available_timezones()  # set of IANA zone keys (~450-600, data-dependent)
print('America/New_York' in zones)   # True
# NOTE: recomputed on every call by scanning the tz data — cache it, don't call in a loop
```

### Key `zoneinfo` Rules

**Rule 1: `ZoneInfo` is a regular `tzinfo` — no pytz idioms.** Unlike `pytz`, attaching the zone directly (constructor `tzinfo=` or `.replace(tzinfo=...)`) is **correct** with `zoneinfo`; DST gaps/overlaps are handled via the `fold` attribute, not a `localize()` call. The robust production pattern is still: store and compute in UTC, convert to a zone only for display:

```python
tz = ZoneInfo('America/New_York')

# Localize a naive wall-clock time — both are correct with zoneinfo
dt = datetime(2024, 3, 10, 2, 30, tzinfo=tz)
dt = datetime(2024, 3, 10, 2, 30).replace(tzinfo=tz)   # equivalent
# (2:30 falls inside the spring-forward gap; zoneinfo maps it per fold=0/1,
#  it never raises — validate wall times yourself if gaps matter)

# The safe default: work in UTC, convert at display time only
utc_dt = datetime(2024, 3, 10, 7, 30, tzinfo=timezone.utc)
local  = utc_dt.astimezone(tz)
```

**Rule 2: Handle `ZoneInfoNotFoundError` (a `KeyError` subclass) gracefully:**

```python
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

def get_tz(name: str) -> ZoneInfo:
    try:
        return ZoneInfo(name)
    except ZoneInfoNotFoundError as e:
        raise ValueError(
            f'Unknown timezone {name!r}. '
            f'Install tzdata: pip install tzdata'
        ) from e
```

**Rule 3: Install `tzdata` as an explicit dependency for portability:**

```python
# requirements.txt
tzdata>=2024.1    # required on Windows and minimal Docker images
                  # On Linux/macOS, zoneinfo falls back to system /usr/share/zoneinfo
```

`zoneinfo` looks for data in the following order:
1. System timezone database (`/usr/share/zoneinfo` on Linux/macOS).
2. The `tzdata` PyPI package if installed.
If neither is present, `ZoneInfoNotFoundError` is raised. On Windows the system database is always absent — `tzdata` must be an explicit dependency there (declare it as `tzdata; sys_platform == "win32"`, or unconditionally). Some minimal container images also lack it.[^10]

**Instances are cached:** `ZoneInfo('UTC') is ZoneInfo('UTC')` is `True` — the constructor caches by key (this also makes aware-datetime comparisons behave sanely). Consequence: updated tz data on disk is not picked up for already-cached keys without `ZoneInfo.clear_cache()`; use `ZoneInfo.no_cache(key)` only in tests.

**Rule 4: DST-ambiguous times — use `fold` parameter:**

```python
tz = ZoneInfo('America/New_York')

# 2024-11-03 01:30: ambiguous — clocks fall back at 2 AM
# fold=0 (default): earlier offset (Daylight Time, UTC-4)
# fold=1: later offset (Standard Time, UTC-5)
ambiguous_dt = datetime(2024, 11, 3, 1, 30, tzinfo=tz, fold=0)
standard_dt  = datetime(2024, 11, 3, 1, 30, tzinfo=tz, fold=1)

print(ambiguous_dt.utcoffset())  # -1 day, 20:00:00  (UTC-4)
print(standard_dt.utcoffset())   # -1 day, 19:00:00  (UTC-5)

# Best practice: store all timestamps in UTC and convert to local for display only
def now_utc() -> datetime:
    return datetime.now(timezone.utc)

def display(dt_utc: datetime, zone: str) -> str:
    return dt_utc.astimezone(ZoneInfo(zone)).isoformat()
```

### `zoneinfo` vs `pytz` — Key Differences

| | `pytz` | `zoneinfo` (3.9+) |
|---|---|---|
| Localize naive datetime | Must use `.localize()` | Pass `tzinfo=` in constructor or use `.astimezone()` |
| DST-gap handling | Raises `NonExistentTimeError` or `AmbiguousTimeError` | Uses `fold` attribute |
| Caching | Manual | Automatic (module-level cache) |
| Runtime dependency | `pip install pytz` | Stdlib (+ optional `tzdata` for Windows) |
| IANA data source | Bundled in package | System + `tzdata` fallback |

**Pitfall: `pytz` `.localize()` pattern does not apply to `zoneinfo`.**

```python
import pytz

# pytz — must use .localize(), NOT .replace(tzinfo=...)
eastern = pytz.timezone('America/New_York')
dt = eastern.localize(datetime(2024, 6, 15, 12, 0))  # correct for pytz

# zoneinfo — pass tzinfo= directly or use .astimezone()
from zoneinfo import ZoneInfo
dt = datetime(2024, 6, 15, 12, 0, tzinfo=ZoneInfo('America/New_York'))  # correct
```

***

## New in 3.9: `graphlib` — Topological Sorting

The `graphlib` module provides `TopologicalSorter` for ordering nodes in a dependency graph. It is particularly powerful because it supports **parallel execution** via an incremental API.[^11]

```python
from graphlib import TopologicalSorter, CycleError

# Simple use — static_order()
dependencies: dict[str, set[str]] = {
    'pandas': {'numpy', 'python-dateutil'},
    'scipy':  {'numpy'},
    'seaborn': {'pandas', 'scipy', 'matplotlib'},
    'matplotlib': {'numpy', 'python-dateutil'},
}

ts = TopologicalSorter(dependencies)
install_order = list(ts.static_order())
print(install_order)
# ['python-dateutil', 'numpy', 'pandas', 'scipy', 'matplotlib', 'seaborn']
# (exact order within a tier may vary — only dependency order is guaranteed)

# Cycle detection — CycleError is a ValueError subclass; the cycle is in args[1]
bad_graph = {'A': {'B'}, 'B': {'C'}, 'C': {'A'}}  # cycle!
try:
    list(TopologicalSorter(bad_graph).static_order())
except CycleError as e:
    print(f'Cycle detected: {e.args[1]}')   # e.g. ['A', 'C', 'B', 'A']
```

### Incremental API for Parallel Execution

The full incremental API (`prepare()`, `is_active()`, `get_ready()`, `done()`) is designed for processing ready nodes concurrently:

```python
from concurrent.futures import ThreadPoolExecutor
from graphlib import TopologicalSorter

def execute_parallel(graph: dict[str, set[str]], execute_fn) -> None:
    ts = TopologicalSorter(graph)
    ts.prepare()
    with ThreadPoolExecutor(max_workers=4) as pool:
        while ts.is_active():
            ready = ts.get_ready()          # all nodes whose predecessors are done
            list(pool.map(execute_fn, ready))   # run the tier concurrently
            ts.done(*ready)                 # unlock the next tier

# Build graph incrementally instead of all at once
ts = TopologicalSorter()
ts.add('install_app', 'install_deps', 'create_db')   # add(node, *predecessors)
ts.add('install_deps', 'download_packages')
ts.add('create_db', 'check_config')
order = list(ts.static_order())   # static_order() calls prepare() itself
```

(`TopologicalSorter` documents no thread-safety guarantees — keep `get_ready()`/`done()` calls in one coordinating thread, as above, and parallelize only the node execution.)

**Pitfall: a sorter is single-use.** After `prepare()`/`static_order()` the graph is frozen — build a new `TopologicalSorter` for each run.

**Pitfall: `add()` cannot be called after `prepare()`:**

```python
ts = TopologicalSorter({'A': {'B'}})
ts.prepare()
ts.add('C', 'A')   # ValueError: Nodes cannot be added after a call to prepare()
```

**Pitfall: graph dict maps each node to its *predecessors* (dependencies), not its successors:**

```python
# The dict maps: node -> {things that must happen BEFORE node}
graph = {
    'test': {'build'},    # test depends on build
    'deploy': {'test'},   # deploy depends on test
    'build': set(),       # build has no dependencies
}
# Correct order: build -> test -> deploy
```

***

## New in 3.9: Relaxed Decorator Grammar (PEP 614)

Decorators can now be **any valid expression**, not just a dotted name optionally followed by a call.[^1]

```python
# Python 3.8 and earlier — decorator must be a (dotted) name, optionally called once:
# @buttons.clicked.connect was fine, but @buttons[0].clicked.connect was a SyntaxError

# Python 3.9+ — any expression is valid as a decorator
buttons = [Button(), Button()]

@buttons[0].clicked.connect   # subscript — SyntaxError before 3.9
def on_click():
    pass

# Dictionary dispatch of decorators
decorators = {'cached': functools.lru_cache, 'logged': log_calls}

@decorators['cached']   # dict lookup — SyntaxError before 3.9
def expensive():
    pass

# Conditional decorator
import os

@(functools.lru_cache if os.environ.get('CACHE') else lambda f: f)
def maybe_cached():
    pass

# Lambda decorator (unusual but valid in 3.9)
@(lambda f: f)   # identity decorator
def plain():
    pass
```

**Recommendation:** The relaxed grammar exists mainly for GUI frameworks (Qt signal/slot, Tkinter bindings) and dispatch tables. In general application code, readable named decorator definitions are strongly preferred over complex decorator expressions.

***

## Lists

```python
lst: list[int] = [1, 2, 3, 4, 5]

# Indexing and slicing
print(lst[0])        # 1
print(lst[-1])       # 5
print(lst[1:3])      # [2, 3]
print(lst[::-1])     # [5, 4, 3, 2, 1] — reversed copy
print(lst[::2])      # [1, 3, 5]

# Mutation
lst.append(6)
lst.extend([7, 8])
lst.insert(0, 0)
lst.remove(3)          # removes first occurrence; raises ValueError if absent
popped = lst.pop()     # removes and returns last element
popped_idx = lst.pop(0)
del lst[0]

# Sorting
nums = [3, 1, 4, 1, 5, 9]
nums.sort()                     # in-place, stable
nums.sort(key=lambda x: -x)     # descending
sorted_copy = sorted(nums)      # returns new list — does not modify nums
nums.reverse()                  # in-place

# Copying
import copy
shallow = lst[:]
shallow2 = list(lst)
deep = copy.deepcopy(lst)

# List comprehensions
squares = [x**2 for x in range(10)]
evens   = [x for x in range(20) if x % 2 == 0]
flat    = [item for sublist in nested for item in sublist]
```

**Pitfall: list multiplication shares references:**

```python
# WRONG — all rows are the SAME list object
matrix = [[0] * 3] * 3
matrix[0][0] = 99
print(matrix)  # [[99, 0, 0], [99, 0, 0], [99, 0, 0]] — unexpected!

# CORRECT
matrix = [[0] * 3 for _ in range(3)]
matrix[0][0] = 99
print(matrix)  # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

**Pitfall: comprehension variables do NOT leak in Python 3:**

```python
x = 10
result = [x for x in range(5)]
print(x)  # 10 — x in enclosing scope is unchanged
```

***

## Tuples

```python
t = (1, 2, 3)
single = (42,)   # trailing comma required for single-element tuple
empty  = ()

# Unpacking
a, b, c = t
a, b = b, a        # swap without temp variable

# Extended unpacking
first, *rest = [1, 2, 3, 4, 5]
head, *middle, tail = [1, 2, 3, 4, 5]
print(head, middle, tail)   # 1 [2, 3, 4] 5

# Named tuples
from typing import NamedTuple

class Point(NamedTuple):
    x: float
    y: float
    label: str = ''

p = Point(1.0, 2.0, 'origin')
print(p.x, p.y, p.label)
print(p._asdict())    # {'x': 1.0, 'y': 2.0, 'label': 'origin'}
```

***

## Dictionaries

Dicts in Python 3.7+ are ordered by insertion order. Python 3.9 adds `|` and `|=` (covered above).

```python
d: dict[str, int] = {'a': 1, 'b': 2}

print(d['a'])
print(d.get('c', 0))            # 0
print(d.setdefault('d', 99))    # 99 — inserts and returns

d['e'] = 5
del d['a']
d.update({'f': 6, 'g': 7})

for k, v in d.items():
    print(k, v)

# Reversed iteration (since 3.8)
for k in reversed(d):
    print(k)

# Dict comprehension
squares = {x: x**2 for x in range(10)}
inverted = {v: k for k, v in d.items()}
filtered = {k: v for k, v in d.items() if v is not None}

# Merge (3.9+)
config = defaults | overrides
```

**Pitfall: never modify a dict while iterating its keys:**

```python
# WRONG — RuntimeError: dictionary changed size during iteration
for k in d:
    if should_delete(k):
        del d[k]

# CORRECT
for k in list(d.keys()):
    if should_delete(k):
        del d[k]

# MOST IDIOMATIC
d = {k: v for k, v in d.items() if not should_delete(k)}
```

### `collections.defaultdict`, `Counter`, `OrderedDict`, `deque`

```python
from collections import defaultdict, Counter, OrderedDict, deque, ChainMap

# defaultdict
word_count: defaultdict[str, int] = defaultdict(int)
for word in text.split():
    word_count[word] += 1

graph: defaultdict[str, list[str]] = defaultdict(list)

# Counter
c = Counter('abracadabra')
print(c.most_common(3))   # [('a', 5), ('b', 2), ('r', 2)]
c.update('hello')

# ChainMap — layered lookup
from collections import ChainMap
defaults_map = {'color': 'blue', 'user': 'guest'}
user_prefs   = {'color': 'red'}
env          = ChainMap(user_prefs, defaults_map)
print(env['color'])   # 'red' — user_prefs wins
print(env['user'])    # 'guest' — falls through to defaults

# deque — O(1) append and popleft (unlike list)
q: deque[str] = deque(maxlen=1000)
q.append('item')
item = q.popleft()
q.appendleft('front')
q.rotate(1)
```

***

## Sets

```python
s: set[int] = {1, 2, 3}
s2 = set([1, 2, 2, 3])
empty_set = set()          # NOT {} — that's an empty dict

s.add(4)
s.discard(10)    # no error if absent
s.remove(4)      # raises KeyError if absent

a = {1, 2, 3, 4}
b = {3, 4, 5, 6}
print(a | b)     # union: {1, 2, 3, 4, 5, 6}
print(a & b)     # intersection: {3, 4}
print(a - b)     # difference: {1, 2}
print(a ^ b)     # symmetric difference: {1, 2, 5, 6}
print(a <= b)    # subset check
print(a.isdisjoint({7, 8}))  # True

fs = frozenset([1, 2, 3])   # immutable and hashable — valid dict key / set member
unique = {x**2 for x in range(-5, 6)}  # set comprehension
```

***

## Control Flow

```python
# if / elif / else
score = 85
label = 'pass' if score >= 60 else 'fail'

# for / else (else runs only when no break occurred)
for item in collection:
    if matches(item):
        result = item
        break
else:
    result = default_value

# Walrus operator in while (3.8+)
with open('data.bin', 'rb') as f:
    while chunk := f.read(65536):
        process(chunk)
```

***

## Functions

```python
from typing import Optional

def greet(
    name: str,
    greeting: str = 'Hello',
    /,               # positional-only up to here (3.8+)
    *,               # keyword-only from here
    punctuation: str = '!',
) -> str:
    """Return a greeting string."""
    return f'{greeting}, {name}{punctuation}'

greet('Alice')                    # 'Hello, Alice!'
greet('Alice', 'Hi')              # 'Hi, Alice!'
greet('Alice', punctuation='.')   # 'Hello, Alice.'
```

### Closures and `nonlocal`

```python
from collections.abc import Callable   # typing.Callable is deprecated in 3.9

def make_counter(start: int = 0) -> Callable[[], int]:
    count = start
    def counter() -> int:
        nonlocal count
        count += 1
        return count
    return counter

c = make_counter()
print(c(), c(), c())  # 1 2 3

# Pitfall: late binding in closures
# WRONG — all lambdas capture the same i
funcs = [lambda: i for i in range(5)]
print([f() for f in funcs])  # [4, 4, 4, 4, 4]

# CORRECT — default argument captures current value
funcs = [lambda i=i: i for i in range(5)]
print([f() for f in funcs])  # [0, 1, 2, 3, 4]
```

### `functools`

```python
import functools

# lru_cache — bare decorator syntax since 3.8 (default maxsize=128)
@functools.lru_cache
def fib(n: int) -> int:
    if n < 2: return n
    return fib(n-1) + fib(n-2)

# functools.cache — NEW in 3.9: unbounded cache, identical to
# lru_cache(maxsize=None) but smaller and faster (no eviction bookkeeping)
@functools.cache
def factorial(n: int) -> int:
    return n * factorial(n - 1) if n else 1
# Use cache when the argument space is bounded (enum-like inputs, config
# lookups); use lru_cache with a limit when unbounded growth would leak memory.

# cached_property — compute once, cache in instance __dict__
# Pitfalls: (a) fails with TypeError on classes defining __slots__ (no __dict__);
# (b) in 3.8/3.9 it holds a lock that is shared across ALL instances of the
# class while computing — slow getters hit concurrently on many instances
# serialize on that one lock (locking removed in 3.12)
class Dataset:
    def __init__(self, data: list[float]) -> None:
        self.data = data

    @functools.cached_property
    def mean(self) -> float:
        import statistics
        return statistics.mean(self.data)

# singledispatch
@functools.singledispatch
def process(arg: object) -> str:
    raise TypeError(f'Unsupported type: {type(arg).__name__}')

@process.register(str)
def _(arg: str) -> str:
    return arg.upper()

@process.register(int)
@process.register(float)
def _(arg) -> str:
    return str(arg * 2)

# partial
from functools import partial
print_error = partial(print, file=__import__('sys').stderr, flush=True)
```

### Generators and `itertools`

```python
from collections.abc import Generator   # typing.Generator is deprecated in 3.9
import itertools

def fibonacci() -> Generator[int, None, None]:
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

# itertools
list(itertools.chain([1, 2], [3, 4]))               # [1, 2, 3, 4]
list(itertools.islice(itertools.count(), 5))         # [0, 1, 2, 3, 4]
list(itertools.accumulate([1,2,3,4], initial=0))     # [0, 1, 3, 6, 10]
list(itertools.pairwise([1,2,3,4]))                  # [(1,2),(2,3),(3,4)] — NOTE: 3.10+ only!
list(itertools.combinations('ABC', 2))               # AB, AC, BC
list(itertools.product([0,1], repeat=3))             # 8 binary triples

# Generator expressions — O(1) memory
total = sum(x**2 for x in range(1_000_000))
```

**Note: `itertools.pairwise()` is Python 3.10+.** In 3.9, use:

```python
def pairwise(iterable):
    it = iter(iterable)
    a = next(it, None)
    for b in it:
        yield a, b
        a = b
```

***

## Dataclasses

`dataclasses` is fully available. In 3.9, you can use built-in generics in field annotations without `typing` imports.

```python
from dataclasses import dataclass, field
from typing import ClassVar

@dataclass
class User:
    id: int
    name: str
    email: str
    active: bool = True
    tags: list[str] = field(default_factory=list)   # list[str] — no typing.List
    config: dict[str, object] = field(default_factory=dict)

    def __post_init__(self) -> None:
        self.email = self.email.lower()

@dataclass(frozen=True)
class Coordinate:
    lat: float
    lon: float

@dataclass(order=True)
class Version:
    major: int
    minor: int
    patch: int = 0
```

**Critical pitfalls:**

```python
# WRONG — mutable default raises ValueError at class definition time
@dataclass
class Bad:
    items: list[int] = []   # ValueError

# CORRECT
@dataclass
class Good:
    items: list[int] = field(default_factory=list)

# WRONG — non-default field after default field
@dataclass
class BadOrder:
    x: int = 0
    y: int      # TypeError

# CORRECT
@dataclass
class GoodOrder:
    y: int
    x: int = 0
```

***

## Classes and OOP

### Class Definition

```python
import math

class Shape:
    count: int = 0   # class attribute — shared across all instances

    def __init__(self, color: str = 'black') -> None:
        self.color = color
        Shape.count += 1

    def __repr__(self) -> str:
        return f'{type(self).__name__}(color={self.color!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, type(self)):
            return NotImplemented
        return self.__dict__ == other.__dict__

    def __hash__(self) -> int:
        # Always define __hash__ when defining __eq__
        return hash(tuple(sorted(self.__dict__.items())))

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
```

### Abstract Base Classes

```python
from abc import ABC, abstractmethod
from typing import Optional

class Repository(ABC):
    @abstractmethod
    def get(self, id: int) -> Optional[dict[str, object]]:
        ...

    @abstractmethod
    def save(self, entity: dict[str, object]) -> int:
        ...

    def exists(self, id: int) -> bool:
        return self.get(id) is not None

class PostgresRepository(Repository):
    def get(self, id: int) -> Optional[dict[str, object]]:
        ...

    def save(self, entity: dict[str, object]) -> int:
        ...
```

**Note on `X | Y` union syntax in annotations:** `int | None` is a runtime expression evaluated when the `def`/`class` statement executes — in 3.9 it raises `TypeError` immediately, without any framework involved. With `from __future__ import annotations` it parses (annotations become lazy strings), but anything that *evaluates* annotations — `typing.get_type_hints()`, dataclass/pydantic/FastAPI introspection — still fails with the same `TypeError` on 3.9. **Recommendation for 3.9: write `Optional[X]`/`Union[X, Y]`.** `X | Y` is only fully usable from 3.10.

### Enums

```python
from enum import Enum, IntEnum, Flag, auto
from typing import final

class Color(Enum):
    RED   = auto()
    GREEN = auto()
    BLUE  = auto()

class HttpStatus(IntEnum):
    OK           = 200
    NOT_FOUND    = 404
    SERVER_ERROR = 500

class Permission(Flag):
    READ    = auto()
    WRITE   = auto()
    EXECUTE = auto()
    ALL     = READ | WRITE | EXECUTE

perms = Permission.READ | Permission.WRITE
print(Permission.EXECUTE in perms)  # False
```

***

## Exception Handling

```python
import logging

logger = logging.getLogger(__name__)

class AppError(Exception):
    """Base for all application-specific errors."""

class ConfigError(AppError):
    def __init__(self, key: str, message: str = '') -> None:
        self.key = key
        super().__init__(f'Config error for {key!r}' + (f': {message}' if message else ''))

def read_config(path: str) -> dict[str, object]:
    try:
        import json
        with open(path, 'r', encoding='utf-8') as f:
            return json.load(f)
    except FileNotFoundError:
        raise
    except json.JSONDecodeError as e:
        raise ConfigError('file', f'Invalid JSON in {path!r}') from e
    except OSError as e:
        logger.error('Cannot read %r: %s', path, e)
        raise
    finally:
        logger.debug('read_config finished for %r', path)  # always runs

# Exception chaining
try:
    int('N/A')
except ValueError as e:
    raise ConfigError('port', 'must be an integer') from e

# Re-raise preserving traceback — always bare raise
try:
    risky()
except Exception:
    logger.exception('Unexpected error')
    raise   # CORRECT — preserves original traceback (not raise e)
```

***

## Context Managers

```python
# Multiple with resources in 3.9 — backslash continuation or contextlib.ExitStack.
# (The parenthesized multi-with form happens to compile on CPython 3.9's PEG
# parser, but it is NOT official syntax until 3.10 — 3.9's -X oldparser mode
# and many tools reject it. Don't rely on it in 3.9 code.)
with open('input.txt', encoding='utf-8') as fin, \
     open('output.txt', 'w', encoding='utf-8') as fout:
    for line in fin:
        fout.write(line.upper())

from contextlib import contextmanager, suppress

@contextmanager
def timed_operation(name: str):
    import time
    start = time.perf_counter()
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        logger.info('%s completed in %.3fs', name, elapsed)

with suppress(FileNotFoundError):
    import os
    os.remove('/tmp/optional_cache')
```

***

## File I/O and `pathlib`

Always pass `encoding='utf-8'` explicitly to `open()`.

```python
from pathlib import Path

p = Path('/home/alice/data.csv')
p = Path.home() / 'reports' / 'output.csv'

# Path properties
print(p.name)    # 'output.csv'
print(p.stem)    # 'output'
print(p.suffix)  # '.csv'
print(p.parent)  # PosixPath('.../reports')

# Read / write
text = p.read_text(encoding='utf-8')
p.write_text('hello\n', encoding='utf-8')
data = p.read_bytes()

# Directory operations
p.parent.mkdir(parents=True, exist_ok=True)
p.unlink(missing_ok=True)    # missing_ok=True from 3.8

# New in 3.9: Path.is_relative_to()
print(Path('/etc/passwd').is_relative_to('/etc'))     # True
print(Path('/usr/bin/python').is_relative_to('/etc')) # False

# New in 3.9: PurePath.with_stem() and Path.readlink()
p = Path('/srv/app/data/report.txt')
print(p.with_stem('summary'))          # /srv/app/data/summary.txt
target = Path('current').readlink()    # symlink target as a Path, not a str
```

### New in 3.9: `Path.is_relative_to()`

`Path.is_relative_to()` tests whether a path is relative to another path — the recommended way to do path-traversal checks in 3.9+:[^1]

```python
from pathlib import Path

def safe_read(base_dir: str, user_filename: str) -> str:
    base = Path(base_dir).resolve()
    requested = (base / user_filename).resolve()

    # Python 3.8 approach (still works in 3.9):
    try:
        requested.relative_to(base)
    except ValueError:
        raise PermissionError(f'{user_filename!r} escapes base directory')

    # Python 3.9 approach — cleaner:
    if not requested.is_relative_to(base):
        raise PermissionError(f'{user_filename!r} escapes base directory')

    return requested.read_text(encoding='utf-8')
```

### Binary Files with Walrus

```python
CHUNK_SIZE = 65_536   # 64 KiB

with open('large.bin', 'rb') as f:
    while chunk := f.read(CHUNK_SIZE):
        process(chunk)
```

### Temporary Files

```python
import tempfile

with tempfile.NamedTemporaryFile(
    suffix='.csv', mode='w', encoding='utf-8', delete=True,
) as tmp:
    tmp.write('col1,col2\n')
    tmp.flush()
    process(tmp.name)

with tempfile.TemporaryDirectory() as tmpdir:
    work_in(tmpdir)

# NEVER use tempfile.mktemp() — TOCTOU-vulnerable
```

***

## `async`/`await` and `asyncio`

### New in 3.9: `asyncio.to_thread()`

`asyncio.to_thread()` is a high-level convenience wrapper around `loop.run_in_executor()` for running blocking code in a thread:[^1]

```python
import asyncio

# OLD (3.8) — verbose
async def read_file_old(path: str) -> str:
    loop = asyncio.get_running_loop()
    return await loop.run_in_executor(None, Path(path).read_text)

# NEW (3.9) — clean
async def read_file(path: str) -> str:
    return await asyncio.to_thread(Path(path).read_text, encoding='utf-8')

# With keyword arguments — to_thread passes them through
async def hash_file(path: str) -> str:
    import hashlib

    def compute() -> str:   # ALL blocking work inside the thread
        return hashlib.sha256(Path(path).read_bytes()).hexdigest()

    return await asyncio.to_thread(compute)
```

`asyncio.to_thread()` accepts the callable and positional/keyword arguments separately, runs it in the event loop's default `ThreadPoolExecutor`, propagates the current `contextvars` context, and returns a coroutine. It is essentially a clean alternative to `loop.run_in_executor(None, partial(func, *args, **kwargs))`. Two caveats:

- **Evaluate nothing blocking at the call site.** `asyncio.to_thread(Path(p).read_bytes())` reads the file in the event loop *before* the thread starts — pass the callable, not the result.
- **The GIL still applies** — `to_thread()` helps blocking I/O, not CPU-bound pure-Python code. Use a `ProcessPoolExecutor` with `run_in_executor()` for CPU work.

`loop.run_in_executor()` remains the tool when you need a custom executor (process pool, bounded pool).

### Core `asyncio` Patterns (unchanged from 3.8)

```python
import asyncio

# Entry point — always asyncio.run()
asyncio.run(main())

# Named tasks
task = asyncio.create_task(
    coroutine(),
    name='descriptive-name',
)

# Concurrent execution
results = await asyncio.gather(task1, task2, task3)
results = await asyncio.gather(*tasks, return_exceptions=True)

# Timeout
try:
    result = await asyncio.wait_for(coroutine(), timeout=10.0)
except asyncio.TimeoutError:
    handle_timeout()

# Bounded concurrency
sem = asyncio.Semaphore(50)
async def bounded(url: str) -> str:
    async with sem:
        return await fetch(url)

# ALWAYS re-raise CancelledError
async def worker():
    try:
        await task()
    except asyncio.CancelledError:
        await cleanup()
        raise  # mandatory
```

### Removed/Deprecated asyncio APIs in 3.9

- `asyncio.Task.current_task()` and `asyncio.Task.all_tasks()` class methods are **removed in 3.9** — use the module-level `asyncio.current_task()` and `asyncio.all_tasks()` (both 3.7+).[^1]
- Acquiring locks via `with await lock:` / `with (yield from lock):` no longer works in 3.9 — use `async with lock:`.[^1]
- The `loop` parameter of high-level APIs (`asyncio.gather()`, `asyncio.sleep()`, `asyncio.wait_for()`, `asyncio.Lock()`, ...) is deprecated since 3.8 and **removed in 3.10** — it emits `DeprecationWarning` in 3.9. Never pass `loop=`; rely on the running loop.
- Inside async code, use `asyncio.get_running_loop()` (3.7+), not `get_event_loop()` — it raises `RuntimeError` when no loop is running instead of silently creating one.

***

## Concurrency

### GIL

CPython's Global Interpreter Lock means only one thread executes Python bytecode at a time. `threading` works for I/O-bound work; `multiprocessing` for CPU-bound work.

### `concurrent.futures` — New in 3.9: `cancel_futures`

`Executor.shutdown()` gains a `cancel_futures=True` parameter that discards all pending (not-yet-started) futures immediately instead of waiting for them:[^12]

```python
from concurrent.futures import ThreadPoolExecutor, ProcessPoolExecutor, as_completed
import signal

executor = ThreadPoolExecutor(max_workers=10)

def graceful_shutdown():
    # 3.8 and earlier — had to drain queue manually
    # 3.9+ — cancel_futures=True handles it
    executor.shutdown(wait=True, cancel_futures=True)

signal.signal(signal.SIGTERM, lambda *_: graceful_shutdown())

with ThreadPoolExecutor(max_workers=10) as executor:
    futures = {executor.submit(fetch_url, url): url for url in urls}
    for future in as_completed(futures):
        url = futures[future]
        try:
            data = future.result()
        except Exception as e:
            logger.error('Failed %s: %s', url, e)
```

**Pitfall: `shutdown(cancel_futures=True)` + `as_completed()`/`wait()` on the same futures can hang — on every Python version.** Futures cancelled by the shutdown drain stay in `CANCELLED` (never `CANCELLED_AND_NOTIFIED`), so `as_completed()`/`wait()` waiting on them never wakes up (bpo-43727 — still unfixed as of 2026).[^13] Work around it: never wait on futures that the shutdown may have drained, or filter them first:

```python
executor.shutdown(wait=False, cancel_futures=True)
remaining = [f for f in futures if not f.cancelled()]   # drop drained futures
for future in as_completed(remaining):
    ...
```

**Other 3.9 executor changes:** `ProcessPoolExecutor` spawns worker processes **on demand** (only when no idle worker is available) instead of eagerly starting `max_workers` processes, and both executors no longer use daemon threads — fixing hangs/races at interpreter shutdown.[^1]

### `threading`

```python
import threading

lock = threading.Lock()
results: list = []

def worker(task_id: int) -> None:
    result = do_work(task_id)
    with lock:
        results.append(result)

threads = [threading.Thread(target=worker, args=(i,), daemon=True) for i in range(10)]
for t in threads: t.start()
for t in threads: t.join()
```

### `multiprocessing`

```python
from multiprocessing import Pool, cpu_count

def process_chunk(chunk: list) -> list:
    return [expensive_transform(item) for item in chunk]

if __name__ == '__main__':  # REQUIRED on Windows and macOS (spawn default since 3.8)
    data = load_dataset()
    n = cpu_count()
    chunks = [data[i::n] for i in range(n)]

    with Pool(processes=n) as pool:
        results = pool.map(process_chunk, chunks)

    flat = [item for sub in results for item in sub]
```

***

## `collections.abc` Migration (Critical for 3.9 Compatibility)

Python 3.9 is the last version where importing ABCs from `collections` directly (e.g., `from collections import Mapping`) works at all — it emits a `DeprecationWarning`. **In Python 3.10 the aliases are removed: `from collections import Mapping` raises `ImportError`, and `collections.Mapping` attribute access raises `AttributeError`.** Fix all occurrences now.[^3]

```python
# WRONG — DeprecationWarning in 3.9, ImportError in 3.10+
from collections import Callable, Iterable, Mapping, MutableMapping, Sequence

# CORRECT — always import ABCs from collections.abc
from collections.abc import Callable, Iterable, Mapping, MutableMapping, Sequence
```

**Finding violations in your codebase:**

```bash
# grep for the broken import pattern
grep -rn "from collections import" . | grep -v "collections.abc" | grep -v "deque\|Counter\|OrderedDict\|defaultdict\|ChainMap\|namedtuple\|UserDict\|UserList\|UserString"

# or run with warnings-as-errors to catch all occurrences
python -W error::DeprecationWarning -m pytest tests/
```

**If a third-party dependency triggers the warning:**

```python
import warnings

# Suppress only the specific warning from a specific package — never globally
with warnings.catch_warnings():
    warnings.filterwarnings('ignore', category=DeprecationWarning, module='thirdparty')
    import thirdparty
```

***

## The New PEG Parser (CPython Internals)

Python 3.9 replaces the LL(1) recursive-descent parser with a PEG (Parsing Expression Grammar) parser. From an application developer's perspective:[^1]

- **No behavior change** — the AST produced is identical.
- **Performance** is roughly comparable.
- The old parser can be re-enabled in 3.9 only with `-X oldparser` or `PYTHONOLDPARSER=1` — this option is removed in 3.10.
- The PEG parser **enables future language features** (structured pattern matching in 3.10, etc.) that were not expressible in LL(1).

The practical implication: if you have code that directly interacts with the `parser` module (deprecated in 3.9, removed in 3.10), migrate to `ast` now:

```python
# WRONG — parser module deprecated in 3.9, removed in 3.10
import parser
st = parser.suite(source)

# CORRECT
import ast
tree = ast.parse(source)
ast.dump(tree, indent=2)

# ast.unparse — new in 3.9
code = ast.unparse(ast.parse('x = 1 + 2'))   # 'x = 1 + 2'
```

`ast.unparse()` is new in 3.9 — it converts an AST back to source code. Useful for code generation and AST transformations.

***

## Logging

```python
import logging

def setup_logging(level: str = 'INFO') -> None:
    logging.basicConfig(
        level=getattr(logging, level.upper()),
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        datefmt='%Y-%m-%dT%H:%M:%S',
        force=True,   # reconfigures existing handlers (from 3.8)
    )

logger = logging.getLogger(__name__)

# % style in logging calls — NOT f-strings
# f-strings are evaluated even when the log level would suppress the message
logger.debug('Processing %r', item)
logger.info('Loaded %d records', count)
logger.error('Failed to connect: %s', exc)

try:
    risky()
except Exception:
    logger.exception('Unexpected error')  # includes traceback automatically
```

### New in 3.9: `logging.getLogger('root')` change

In Python 3.9, `logging.getLogger('root')` now returns the **root logger**, whereas in earlier versions it returned a non-root logger named `'root'`. This is a subtle breaking change for any code that explicitly names a logger `'root'`.[^1]

```python
# If you have any module named 'root.py' or logger named 'root':
# Python 3.8: logging.getLogger('root') -> non-root logger named 'root'
# Python 3.9: logging.getLogger('root') -> the actual root logger

# Fix: use a different logger name, or use logging.root directly
root_logger = logging.root    # unambiguous reference to the root logger
```

***

## Security

### Input Validation

```python
def validate_port(value: object) -> int:
    try:
        port = int(str(value))
    except (ValueError, TypeError):
        raise ValueError(f'Port must be an integer, got {value!r}')
    if not (1 <= port <= 65535):
        raise ValueError(f'Port out of range: {port}')
    return port
```

### Path Traversal — `is_relative_to()` (new in 3.9)

```python
from pathlib import Path

def safe_serve(base_dir: str, user_path: str) -> bytes:
    base = Path(base_dir).resolve()
    target = (base / user_path).resolve()
    if not target.is_relative_to(base):   # new in 3.9
        raise PermissionError(f'Access denied: {user_path!r}')
    return target.read_bytes()
```

**Note:** `is_relative_to()` is a purely lexical check — the `resolve()` calls before it are what defeat `..` segments and symlinks. Never call `is_relative_to()` on unresolved paths.

### Security Fixes in 3.9 Point Releases

- Since **3.9.2**, `urllib.parse.parse_qs()`/`parse_qsl()` no longer treat `;` as a query separator (web-cache-poisoning fix; a `separator` parameter was added).
- Since **3.9.5**, `ipaddress` rejects leading zeros in IPv4 address strings (octal-ambiguity fix).

Always run the latest 3.9.x patch release.

### Secrets and Cryptography

```python
import secrets
import string

token    = secrets.token_urlsafe(32)    # 32 bytes of entropy, URL-safe base64
hex_tok  = secrets.token_hex(32)        # 64-char hex string
raw      = secrets.token_bytes(32)

alphabet = string.ascii_letters + string.digits
password = ''.join(secrets.choice(alphabet) for _ in range(20))

# Constant-time comparison — prevents timing attacks on tokens
if secrets.compare_digest(stored_token, provided_token):
    grant_access()

# NEVER use the random module for security values — deterministic PRNG, predictable
import random
random.randbytes(16).hex()   # WRONG for tokens — use secrets.token_hex(16)
```

### SQL Injection Prevention

```python
import sqlite3

# WRONG — string interpolation
conn.execute(f'SELECT * FROM users WHERE id = {user_id}')

# CORRECT — parameterized query
cursor = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,))
results = cursor.fetchall()
```

### Subprocess Safety

```python
import subprocess

# SAFE — list form, no shell interpretation
result = subprocess.run(
    ['git', 'log', '--oneline', '-n', '10'],
    capture_output=True,
    encoding='utf-8',
    timeout=30,
    check=True,
)

# DANGEROUS — never interpolate user input with shell=True
user_input = get_user_input()
# subprocess.call(f'ls {user_input}', shell=True)  # command injection!
subprocess.call(['ls', user_input])    # SAFE
```

### XML Security

Use `defusedxml` for untrusted XML input:

```python
# pip install defusedxml
import defusedxml.ElementTree as ET
tree = ET.parse('untrusted.xml')   # safe against XXE, billion-laughs attacks
```

***

## JSON and Serialization

```python
import json

data = {'name': 'Alice', 'tags': ['admin', 'user']}
json_str = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False)
restored = json.loads(json_str)

from datetime import datetime

class AppEncoder(json.JSONEncoder):
    def default(self, obj: object) -> object:
        if isinstance(obj, datetime):
            return obj.isoformat()
        if isinstance(obj, set):
            return sorted(obj)
        return super().default(obj)

json.dumps({'ts': datetime.now()}, cls=AppEncoder)
```

### Pickle

```python
import pickle

with open('data.pkl', 'wb') as f:
    pickle.dump(data, f, protocol=pickle.HIGHEST_PROTOCOL)

with open('data.pkl', 'rb') as f:
    restored = pickle.load(f)
```

`pickle` defaults to **Protocol 4** in 3.9 (the default since 3.8; it only becomes 5 in Python 3.14). Protocol 5 (out-of-band buffers, PEP 574, since 3.8) is available as `pickle.HIGHEST_PROTOCOL` — pass it explicitly for efficient large-data transfer.

**SECURITY WARNING:** Never unpickle data from untrusted sources — pickle can execute arbitrary code during deserialization. For inter-system communication, use JSON, protobuf, or msgpack.

***

## CSV

```python
import csv

# text mode + newline='' prevents double newlines on Windows
with open('data.csv', 'r', encoding='utf-8', newline='') as f:
    reader = csv.DictReader(f)
    for row in reader:
        process(row)

with open('output.csv', 'w', encoding='utf-8', newline='') as f:
    writer = csv.DictWriter(f, fieldnames=['name', 'age'])
    writer.writeheader()
    writer.writerow({'name': 'Alice', 'age': 30})
```

***

## Regular Expressions

```python
import re

EMAIL_RE = re.compile(r'^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$')
DATE_RE  = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')

m = DATE_RE.match('2024-06-15')
if m:
    print(m['year'], m['month'], m['day'])   # dict-style subscript

cleaned = re.sub(r'<[^>]+>', '', html)    # strip HTML tags
tokens  = re.findall(r'\w+', text)

# Greedy vs non-greedy
re.findall(r'<.+>', '<b>bold</b><i>italic</i>')    # ['<b>bold</b><i>italic</i>']
re.findall(r'<.+?>', '<b>bold</b><i>italic</i>')   # ['<b>', '</b>', '<i>', '</i>']
```

***

## `random` Module (New in 3.9: `randbytes`)

```python
import random

# random.randbytes — new in 3.9
b = random.randbytes(16)   # 16 random bytes (NOT cryptographically secure!)
print(type(b))             # <class 'bytes'>

# For cryptographic use, ALWAYS use secrets module:
import secrets
b = secrets.token_bytes(16)   # cryptographically secure

# Reproducible randomness (testing, simulation)
rng = random.Random(42)
rng.shuffle(data)
value = rng.uniform(0, 1)
```

`random.randbytes()` is a convenience addition in 3.9 — it is **not** a secure RNG. It uses the deterministic Mersenne Twister: its output is fully predictable once the internal state is known or the seed is set. The docs say it explicitly: do not use it for security tokens — use `secrets.token_bytes()`.[^1]

***

## Memory Management

```python
import gc
import sys
import tracemalloc
import weakref
from typing import Optional

# Check reference count (includes the local ref in getrefcount itself)
x = [1, 2, 3]
print(sys.getrefcount(x))   # typically 2

# Force collection of cycles
gc.collect()

# Memory profiling
tracemalloc.start(10)
do_work()
snapshot = tracemalloc.take_snapshot()
for stat in snapshot.statistics('lineno')[:10]:
    print(stat)
tracemalloc.reset_peak()   # new in 3.9 — reset peak to current

# Weak references — avoid preventing garbage collection
class Cache:
    def __init__(self):
        self._data: weakref.WeakValueDictionary = weakref.WeakValueDictionary()

    def store(self, key: str, obj: object) -> None:
        self._data[key] = obj

    def retrieve(self, key: str) -> Optional[object]:
        return self._data.get(key)
```

**`tracemalloc.reset_peak()` is new in 3.9.** It allows you to reset the peak memory measurement to the current usage, making it possible to measure peak memory for specific code sections rather than the entire program run.[^1]

***

## Performance

```python
import cProfile, pstats, io, timeit

# cProfile as context manager (from 3.8)
with cProfile.Profile() as pr:
    main()

stream = io.StringIO()
pstats.Stats(pr, stream=stream).sort_stats('cumulative').print_stats(20)
print(stream.getvalue())

# Common patterns
# --- O(1) membership testing
VALID_ROLES = frozenset({'admin', 'editor', 'viewer'})
if role in VALID_ROLES:   # O(1) hash lookup vs O(n) list scan
    pass

# --- String building: O(n) join vs O(n²) += in loop
result = ''.join(str(x) for x in items)   # correct
# result = ''; for x in items: result += str(x)  # quadratic — WRONG

# --- Local variable aliasing in hot loops
sqrt = math.sqrt
for i in range(1_000_000):
    result = sqrt(i)   # avoids LOAD_GLOBAL on every iteration

# --- defaultdict avoids repeated setdefault in hot paths
from collections import defaultdict
d: defaultdict[str, list] = defaultdict(list)
for k, v in pairs:
    d[k].append(v)   # no key-existence check

# --- Lazy iteration — generator expression vs list
# Avoid building a full list in memory when you only need to sum/iterate once
total = sum(x**2 for x in range(1_000_000))   # O(1) memory

# --- Vectorcall speedup (3.9) — builtins range, tuple, set, frozenset,
# list, dict now use the vectorcall protocol internally — no change needed
# in user code; these are faster in 3.9 for free
```

***

## Testing

```python
import asyncio
import pytest
from unittest.mock import AsyncMock, Mock, patch

# AsyncMock from 3.8
async def test_fetch():
    mock = AsyncMock(return_value={'id': 1})
    with patch('myapp.api.fetch_user', mock):
        result = await fetch_user(1)
    mock.assert_awaited_once_with(1)
    assert result['id'] == 1

# IsolatedAsyncioTestCase from 3.8
import unittest

class TestAsync(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.conn = await create_connection()

    async def test_fetch(self) -> None:
        result = await self.conn.fetch('SELECT 1')
        self.assertEqual(result, [(1,)])

    async def asyncTearDown(self) -> None:
        await self.conn.close()

# pytest parametrize with type-annotated built-in generics
@pytest.mark.parametrize('items,expected', [
    ([1, 2, 3], 6),
    ([], 0),
    ([10], 10),
])
def test_sum(items: list[int], expected: int) -> None:
    assert sum(items) == expected

# Testing zoneinfo-aware code
from datetime import datetime, timezone
from zoneinfo import ZoneInfo

def test_timezone_conversion():
    utc = datetime(2024, 6, 15, 12, 0, tzinfo=timezone.utc)
    ny  = utc.astimezone(ZoneInfo('America/New_York'))
    assert ny.hour == 8   # UTC-4 during EDT
    assert ny.tzname() == 'EDT'
```

***

## Virtual Environments and Packaging

```bash
python3.9 -m venv .venv
source .venv/bin/activate   # Unix/macOS
.venv\Scripts\activate      # Windows

pip install -r requirements.txt
pip freeze > requirements.txt

# Modern alternative: uv — fast installer that can also provision the
# (EOL) 3.9 interpreter itself
uv venv --python 3.9
uv pip install -r requirements.txt
```

### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=42", "wheel"]
build-backend = "setuptools.build_meta"

[tool.black]
line-length = 88
target-version = ["py39"]

[tool.mypy]
python_version = "3.9"
strict = true

[tool.isort]
profile = "black"

[tool.pytest.ini_options]
testpaths = ["tests"]
asyncio_mode = "auto"   # requires the pytest-asyncio plugin
```

### Project Layout

```
project/
├── .venv/
├── src/
│   └── myapp/
│       ├── __init__.py
│       ├── main.py
│       └── utils.py
├── tests/
│   ├── conftest.py
│   └── test_main.py
├── pyproject.toml
└── requirements.txt
```

***

## Code Quality Tools

```bash
pip install mypy flake8 black isort bandit

# Type checking
mypy --strict src/

# Format
black src/ tests/
isort --profile=black src/ tests/

# Lint (suppress Black-incompatible rules)
flake8 src/ --max-line-length=88 --extend-ignore=E203,W503

# Security audit
bandit -r src/

# Full CI pipeline
black --check src/ && isort --check-only src/ && flake8 src/ && mypy src/ && bandit -r src/ -q
```

**Modern alternative:** Ruff replaces flake8 + isort (and can replace Black via `ruff format`) with a single fast tool and fully supports 3.9 targets — set `target-version = "py39"` under `[tool.ruff]` in `pyproject.toml`. Its `UP` (pyupgrade) rules auto-migrate `typing.List` → `list` for you.

***

## Removed and Changed in 3.9 — Migration Notes

Long-deprecated Python-2-era aliases were **removed** in 3.9 — old code breaks here, not at 3.10:[^1]

| Removed in 3.9 | Replacement |
|---|---|
| `array.array.tostring()` / `.fromstring()` | `.tobytes()` / `.frombytes()` |
| `threading.Thread.isAlive()` | `.is_alive()` |
| `ElementTree.getchildren()` / `.getiterator()` | `list(elem)` / `elem.iter()` |
| `base64.encodestring()` / `.decodestring()` | `.encodebytes()` / `.decodebytes()` |
| `fractions.gcd()` | `math.gcd()` |
| `json.loads(..., encoding=...)` | drop the parameter |
| `typing.NamedTuple._field_types` | `__annotations__` |
| `asyncio.Task.current_task()` / `.all_tasks()` | `asyncio.current_task()` / `asyncio.all_tasks()` |

**Behavior changes to know:**

- `"".replace("", s, n)` returns `s` for non-zero `n` (used to return `''`).
- `__file__` of the `__main__` module is now an absolute path and stays usable after `os.chdir()`.
- `__import__()` raises `ImportError` instead of `ValueError` for relative imports past the top-level package.
- `date.isocalendar()` / `datetime.isocalendar()` return a named tuple instead of a plain tuple.
- `sys.stderr` is line-buffered even when redirected (was block-buffered).
- `ftplib.FTP` default encoding changed from Latin-1 to UTF-8.
- asyncio's `loop.create_datagram_endpoint()` no longer accepts `reuse_address` (security fix).

**Newly deprecated in 3.9** (clean up proactively): `NotImplemented` in boolean context, `math.factorial()` with floats, `random.sample()` on a `set`, and the `parser`/`symbol` modules (removed in 3.10).

***

## Production Checklist

### Python 3.9-Specific Items

- [ ] All `from collections import Mapping/Sequence/Callable/...` migrated to `from collections.abc import ...` — will fail on 3.10+
- [ ] `typing.List`, `typing.Dict`, `typing.Set`, etc. replaced with built-in `list[...]`, `dict[...]`, `set[...]` in all new code
- [ ] `str.removeprefix()`/`str.removesuffix()` used instead of `lstrip()`/`rstrip()` for prefix/suffix removal
- [ ] `zoneinfo` used for all timezone-aware datetimes — `pytz` audited/replaced
- [ ] `tzdata` listed as an explicit dependency for cross-platform tz support (Windows, minimal Docker)
- [ ] `ZoneInfo` objects created with correct IANA names (`'America/New_York'`, not `'EST'`)
- [ ] DST-ambiguous times handled with `fold` parameter or by working in UTC
- [ ] Dict merging uses `|`/`|=` where applicable — and never `|` on `Counter` expecting last-wins merge
- [ ] `resolve()` + `Path.is_relative_to()` used in path-traversal security checks
- [ ] `asyncio.to_thread()` used for blocking I/O calls in async code (replaces verbose `run_in_executor`)
- [ ] `ast.unparse()` used in code-generation tools (replaces third-party alternatives)
- [ ] `math.lcm()`, `math.gcd(n...)`, `math.nextafter()`, `math.ulp()` used where relevant
- [ ] `concurrent.futures` `shutdown(cancel_futures=True)` used for clean executor shutdown
- [ ] `as_completed()`/`wait()` never called on futures that `shutdown(cancel_futures=True)` may have drained (unfixed hang, bpo-43727) — filter `f.cancelled()` first
- [ ] `parser` module usage removed — replaced with `ast` module
- [ ] `logging.getLogger('root')` calls audited — now returns the actual root logger in 3.9
- [ ] `random.randbytes()` not used for security-sensitive values — use `secrets.token_bytes()`
- [ ] Unions written as `Optional[X]` / `Union[X, Y]` — `X | Y` is 3.10+ (even with `from __future__ import annotations`, runtime introspection like `get_type_hints()` fails on 3.9)
- [ ] No `loop=` parameter passed to asyncio APIs (deprecated since 3.8, removed in 3.10)
- [ ] `functools.cache` only used where the argument space is bounded
- [ ] `tracemalloc.reset_peak()` used in memory profiling for granular section-level peaks
- [ ] `graphlib.TopologicalSorter.add()` never called after `prepare()` (raises `ValueError`)
- [ ] Running the latest 3.9.x patch (`Literal` fixes in 3.9.1, `urllib` `;`-separator fix in 3.9.2, `ipaddress` leading-zeros fix in 3.9.5) — and remember 3.9 is EOL since October 2025

### Code Correctness

- [ ] `bytes` and `str` never mixed without explicit encode/decode
- [ ] `encoding='utf-8'` passed to all `open()` calls for text files
- [ ] `newline=''` passed to `open()` for CSV files
- [ ] No mutable default arguments in function signatures
- [ ] `dataclass` mutable fields use `field(default_factory=...)`, not `= []`
- [ ] `__hash__` defined whenever `__eq__` is defined
- [ ] No list mutation during iteration — use comprehension to rebuild

### Safety and Security

- [ ] SQL queries parameterized — never f-string interpolation
- [ ] `pickle.load()` never called on untrusted data
- [ ] `shell=True` never used with user-controlled input in `subprocess`
- [ ] `eval()` and `exec()` never called on untrusted input
- [ ] `secrets.compare_digest()` used for all token comparisons
- [ ] `defusedxml` used for parsing untrusted XML
- [ ] `secrets` module (not `random`) used for all security-sensitive values

### Performance

- [ ] Profiled before optimizing — no premature optimization
- [ ] Generator expressions in pipelines (not list comprehensions)
- [ ] Membership tests on frequently-searched collections use `set`/`frozenset`
- [ ] `str.join()` for multi-item string building (not loop `+=`)
- [ ] Logging uses `%`-style deferred formatting (not f-strings)
- [ ] Blocking calls wrapped in `asyncio.to_thread()` in async code

### Observability

- [ ] `logging.getLogger(__name__)` used in every module
- [ ] No `print()` in production code paths
- [ ] All custom exceptions inherit from `Exception` (not `BaseException`)
- [ ] `logger.exception()` used inside `except` blocks (includes traceback)
- [ ] Bare `raise` used (not `raise e`) to preserve original tracebacks

***

[^1]: Python 3.9 What's New. Python Software Foundation. https://docs.python.org/3/whatsnew/3.9.html (release/EOL dates: PEP 596, https://peps.python.org/pep-0596/; annual cadence: PEP 602, https://peps.python.org/pep-0602/)
[^2]: Python 3.7 language guarantee: dict insertion order. https://docs.python.org/3.7/whatsnew/3.7.html
[^3]: collections ABC aliases: DeprecationWarning in 3.9, removed in 3.10 (bpo-37324). https://docs.python.org/3/whatsnew/3.10.html#removed
[^4]: Python 3.9 Built-in Types. https://docs.python.org/3.9/library/stdtypes.html
[^5]: PEP 585 – Type Hinting Generics In Standard Collections. https://peps.python.org/pep-0585/
[^6]: PEP 584 – Add Union Operators To dict. https://peps.python.org/pep-0584/
[^7]: PEP 616 – String methods to remove prefixes and suffixes. https://peps.python.org/pep-0616/
[^8]: PEP 593 – Flexible function and variable annotations (typing.Annotated). https://peps.python.org/pep-0593/
[^9]: PEP 615 – Support for the IANA Time Zone Database in the Standard Library. https://peps.python.org/pep-0615/
[^10]: zoneinfo — IANA time zone support. https://docs.python.org/3.9/library/zoneinfo.html
[^11]: graphlib — Functionality for topological sorting. https://docs.python.org/3.9/library/graphlib.html
[^12]: concurrent.futures — Executor.shutdown(cancel_futures=...). https://docs.python.org/3.9/library/concurrent.futures.html
[^13]: bpo-43727 — futures cancelled by shutdown(cancel_futures=True) are not yielded by as_completed()/wait(); unresolved. https://bugs.python.org/issue43727

