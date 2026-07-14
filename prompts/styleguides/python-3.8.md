### Python 3.8: The Complete Production Developer Guide

#### Overview

Python 3.8 was released on October 14, 2019.[^1] It is a dynamically typed, interpreted, general-purpose language with a clean, whitespace-significant syntax. Python 3.8 delivered several landmark PEPs — the walrus operator, positional-only parameters, a now-stable `asyncio.run()`, four major typing additions, and a long list of stdlib enhancements — making it one of the most impactful releases in the 3.x line.

This guide is the reference for projects targeting Python 3.8. Features introduced in 3.8 are marked as such. Where behavior differs from 3.6 or 3.7, the difference is called out explicitly. Do not use anything newer than 3.8: no `str.removeprefix()`/`removesuffix()` or dict `|` merge operator (3.9), no `match` statement or parenthesized context managers (3.10), no `dataclasses.KW_ONLY` (3.10).

A few global truths about Python 3.8 before diving in:

- **Everything is an object.** Integers, functions, classes, modules — all are objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing.** Variables carry no declared type; the type lives in the value.
- **Reference counting + cyclic GC.** CPython manages memory with reference counts plus a cyclic garbage collector.
- **`str` is Unicode everywhere.** All string literals are Unicode text; `bytes` is the separate binary type.
- **Dicts are ordered.** Insertion order is guaranteed by the language spec as of Python 3.7.[^2]

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
DEFAULT_TIMEOUT = 30

# Classes: PascalCase
class UserAccount:
    pass

# Internal use: single underscore prefix
class User:
    def __init__(self):
        self._internal_state = {}

# Name mangling: double underscore prefix
# (still reachable as _Base__private — not true privacy)
class Base:
    def __init__(self):
        self.__private = 'name-mangled'

# Module-level "private": excluded from __all__
_module_cache = {}
```

In Python 3, all classes implicitly inherit from `object`. `class Foo:` and `class Foo(object):` are identical.[^3] Never write the latter in Python 3 code.

##### Indentation and Line Length

```python
# 4 spaces per indentation level — never tabs, never mixed
def function():
    if condition:
        do_something()

# PEP 8 recommends 79 chars; Black (de-facto formatter) uses 88.
# Pick one and enforce it project-wide via CI.
result = some_function(
    argument_one,
    argument_two,
    argument_three,
)

# Implicit line continuation inside brackets — always preferred over backslashes
users = [
    'alice',
    'bob',
    'charlie',
]
```

##### Import Ordering

Three blocks separated by blank lines: standard library, third-party, local. Alphabetize within each group. `isort --profile=black` automates this.

```python
# Standard library
import os
import sys
from pathlib import Path
from typing import Dict, List, Optional

# Third-party
import requests

# Local application
from myapp.models import User
from myapp.utils import format_date

# Never wildcard imports — they defeat static analysis and __all__ hygiene
# Bad:  from module import *
# Good: from module import specific_item
```

***

#### Primitive Types and Numerics

##### Integers

Python 3.8 has a single integer type: `int`. It is **arbitrary precision** — there is no separate `long`.[^3] Arithmetic never overflows; Python allocates more memory silently.

```python
import sys
print(sys.maxsize)     # 9223372036854775807 on 64-bit (C ssize_t max, NOT the limit)
x = 10 ** 1000         # perfectly fine — arbitrary precision
print(type(x))         # <class 'int'>
```

**Numeric literals with underscores (PEP 515, since 3.6):**

```python
one_million  = 1_000_000
hex_mask     = 0xFF_FF_FF_FF
binary_val   = 0b_1001_0111_1010
pi_approx    = 3.141_592_653_589_793
```

**Bitwise operations:**

```python
a = 0b1010
b = 0xFF
c = 0o17
print(a & b)   # AND
print(a | b)   # OR
print(a ^ b)   # XOR
print(~a)      # NOT (bitwise complement)
print(a << 2)  # left shift
print(a >> 1)  # right shift
```

**`pow()` now supports negative exponents (new in 3.8):** For integers, `pow(base, exp, mod)` accepts a negative exponent when `base` and `mod` are coprime, computing the modular inverse:[^1]

```python
pow(38, -1, 137)   # 119 — modular inverse of 38 mod 137
119 * 38 % 137     # 1  — verify
```

##### Floats

Floats are C `double` (64-bit IEEE 754).[^3]

```python
x = 3.14
y = 1.5e-10
z = float('inf')
n = float('nan')
print(x.is_integer())          # False
print((2.0).is_integer())      # True
print(x.as_integer_ratio())    # (7070651414971679, 2251799813685248)
```

**Pitfall: Float comparison.** Never use `==` to compare floats. Use `math.isclose()`:

```python
import math

# BAD
assert 0.1 + 0.2 == 0.3  # fails

# GOOD
assert math.isclose(0.1 + 0.2, 0.3)
assert math.isclose(a, b, rel_tol=1e-9, abs_tol=1e-12)
```

For exact decimal arithmetic, use `decimal`:

```python
from decimal import Decimal, getcontext
getcontext().prec = 28
result = Decimal('0.1') + Decimal('0.2')
print(result)  # 0.3
```

##### Booleans

`bool` is a subtype of `int`. `True == 1` and `False == 0`.[^3]

```python
print(isinstance(True, int))  # True
print(True + True)            # 2
print(True * 5)               # 5
```

**Falsy values** in Python 3.8:[^3]

- `None`
- `False`
- Zero of any numeric type: `0`, `0.0`, `0j`
- Empty sequences: `''`, `b''`, `()`, `[]`
- Empty mapping: `{}`
- Empty set: `set()`
- Instances whose `__bool__` returns `False` or `__len__` returns `0`

**`SyntaxWarning` on identity checks with literals (new in 3.8):** Python 3.8 emits a `SyntaxWarning` when you use `is` or `is not` with string or integer literals:[^1]

```python
# 3.8 emits: SyntaxWarning: "is" with a literal. Did you mean "=="?
if x is 'hello':   # WRONG — use ==
    pass
if x == 'hello':   # CORRECT
    pass
```

This catches a common category of subtle identity-vs-equality bugs.

##### Complex Numbers

```python
z = 3 + 4j
print(z.real)         # 3.0
print(z.imag)         # 4.0
print(abs(z))         # 5.0 — magnitude
print(z.conjugate())  # (3-4j)
```

***

#### Division

In Python 3, `/` **always performs true (float) division**.[^3]

```python
print(7 / 2)    # 3.5
print(7 // 2)   # 3   — floor division
print(7 % 2)    # 1   — modulo
```

**Pitfall: floor division with negative numbers** uses true floor (rounds toward negative infinity), not truncation:

```python
print(-7 // 2)   # -4, not -3
print(-7 % 2)    # 1   (satisfies: (a // b) * b + a % b == a)
```

***

#### New in 3.8: The Walrus Operator `:=` (PEP 572)

Assignment expressions — the `:=` "walrus operator" — are the headline feature of Python 3.8.[^4] They assign a value to a variable **as part of an expression**, avoiding redundant re-evaluation.

```python
# Pattern 1: test-and-use in if/while
import re

# OLD — computes the match twice or uses an extra variable
match = re.search(r'(\d+)% discount', advertisement)
if match:
    discount = float(match.group(1)) / 100.0

# NEW — single expression
if (match := re.search(r'(\d+)% discount', advertisement)):
    discount = float(match.group(1)) / 100.0

# Pattern 2: while loop with terminal condition
# OLD
chunk = f.read(8192)
while chunk:
    process(chunk)
    chunk = f.read(8192)

# NEW — eliminates duplicate read call
while chunk := f.read(8192):
    process(chunk)

# Pattern 3: filter that needs the transformed value
raw = ['  Alice', '  ', 'Bob  ', '', '  Charlie  ']
clean = [stripped for name in raw if (stripped := name.strip())]
# ['Alice', 'Bob', 'Charlie']

# Pattern 4: expensive computation reused in comprehension filter and body
results = [y for x in data if (y := expensive_transform(x)) is not None]
```

##### Walrus Operator: Scoping and Pitfalls

**Scope:** An assignment expression in a comprehension binds its target in the **enclosing** function scope, not inside the comprehension:[^4]

```python
total = 0
partial_sums = [total := total + v for v in [1, 2, 3, 4]]
print(total)         # 10 — total was updated in enclosing scope
print(partial_sums)  # [1, 3, 6, 10]
```

**Pitfall: Cannot use the walrus target as the loop variable of the same comprehension:**

```python
# SyntaxError — 'x' is both the for-target and the walrus target
# [y for x in data if (x := f(x)) is not None]

# CORRECT — use a different name
[y for item in data if (y := f(item)) is not None]
```

**Pitfall: Top-level walrus without parentheses is a `SyntaxError`:**

```python
# SyntaxError — walrus cannot appear unparenthesized as a statement
y := f(x)

# CORRECT — parenthesize when used as an expression statement (rare)
(y := f(x))
```

**Recommendation:** Limit the walrus operator to cases where it genuinely reduces duplication. When in doubt, use a separate assignment statement — it is clearer in most cases.

***

#### New in 3.8: Positional-Only Parameters (PEP 570)

The `/` marker in a function signature declares all parameters **to its left** as positional-only — they cannot be passed as keyword arguments.[^5]

```python
# a, b are positional-only; c, d are normal; e, f are keyword-only
def f(a, b, /, c, d, *, e, f):
    print(a, b, c, d, e, f)

f(10, 20, 30, d=40, e=50, f=60)    # valid
# f(10, b=20, ...)                  # TypeError: b is positional-only
# f(10, 20, 30, 40, 50, f=60)       # TypeError: e must be keyword

# Pure positional-only function
def divmod_impl(a, b, /):
    return a // b, a % b

divmod_impl(10, 3)      # (3, 1)
# divmod_impl(a=10, b=3)  # TypeError
```

**Why it matters for API design:**

1. **Freedom to rename parameters** without breaking callers — the name is not part of the API contract.
2. **Prevents accidental keyword use** of parameters whose names are meaningless (`len(obj=x)` is ugly; `len(x)` is correct).
3. **Eliminates name collisions** with `**kwargs`: positional-only parameter names remain available as keys in `**kwargs`.

```python
class Counter(dict):
    def __init__(self, iterable=None, /, **kwds):
        # 'iterable' can coexist as a kwarg key because it's positional-only
        super().__init__(**kwds)
        if iterable is not None:
            self.update(iterable)
```

**Parameter kind reference:**

| Syntax | Kind |
|---|---|
| `def f(a, b, /):` | `a`, `b` positional-only |
| `def f(a, b):` | `a`, `b` positional-or-keyword (default) |
| `def f(*, a, b):` | `a`, `b` keyword-only |
| `def f(a, /, b, *, c):` | `a` pos-only; `b` pos-or-kw; `c` kw-only |

***

#### Strings: `str` and `bytes`

##### The Two Types

- **`str`**: A sequence of Unicode code points. All undecorated string literals are `str`.[^3]
- **`bytes`**: A sequence of raw 8-bit values, declared with `b''` prefix. **Never implicitly mixed with `str`.**

```python
text = 'hello'         # str — Unicode
data = b'hello'        # bytes — binary
# text + data          # TypeError in Python 3 — always
```

**The Unicode sandwich rule:**

1. Decode `bytes` → `str` at input boundaries (network sockets, binary file reads, OS env vars on some platforms).
2. Work entirely in `str` internally.
3. Encode `str` → `bytes` at output boundaries.

```python
def ensure_str(data: object, encoding: str = 'utf-8') -> str:
    if isinstance(data, bytes):
        return data.decode(encoding)
    return str(data)

def ensure_bytes(data: object, encoding: str = 'utf-8') -> bytes:
    if isinstance(data, str):
        return data.encode(encoding)
    return bytes(data)
```

**Pitfall: `open()` defaults to the platform locale encoding.** On Windows this is often `cp1252`, not UTF-8. Always pass `encoding='utf-8'` explicitly:

```python
# UNSAFE on Windows
with open('data.txt', 'r') as f:
    text = f.read()

# SAFE on all platforms
with open('data.txt', 'r', encoding='utf-8') as f:
    text = f.read()
```

##### String Methods

```python
s = 'Hello, World!'
print(s.upper())                   # 'HELLO, WORLD!'
print(s.lower())                   # 'hello, world!'
print(s.casefold())                # aggressive lowercase — use for comparisons
print(s.strip())                   # strip leading/trailing whitespace
print(s.lstrip('H'))               # 'ello, World!'
print(s.rstrip('!'))               # 'Hello, World'
print(s.replace('World', 'Python'))
print(s.startswith('Hello'))       # True
print(s.endswith('!'))             # True
print(s.split(', '))               # ['Hello', 'World!']
print(', '.join(['a', 'b', 'c']))  # 'a, b, c'
print(s.find('World'))             # 7
print(s.index('World'))            # 7 — raises ValueError if absent
print(s.count('l'))                # 3
print(s.center(20, '-'))           # '---Hello, World!----'
print(s.zfill(20))                 # '0000000Hello, World!'
print('abc'.partition('b'))        # ('a', 'b', 'c')
print(s.splitlines())              # ['Hello, World!']
print('hello'.capitalize())        # 'Hello'
print('hello world'.title())       # 'Hello World'
print('abc'.isalpha())             # True
print('123'.isdigit())             # True
print('abc123'.isalnum())          # True
print('  '.isspace())              # True
```

**Note:** `str.removeprefix()` and `str.removesuffix()` are **3.9+ and not available in 3.8**. On 3.8, use `s[len(prefix):]` after an `if s.startswith(prefix):` check.

##### f-strings in Python 3.8: The `=` Specifier (new in 3.8)

Python 3.8 adds `{expr=}` to f-strings — a debugging specifier that prints **the expression text, an `=`, and the repr of the value**:[^1]

```python
x = 42
y = [1, 2, 3]
print(f'{x=}')         # x=42
print(f'{y=}')         # y=[1, 2, 3]
print(f'{x * 2=}')     # x * 2=84  — works with any expression

# Spaces around = are preserved in the output (useful for readability)
print(f'{x = }')       # x = 42

# Can combine with format specs — spec goes AFTER the =
from math import pi
print(f'{pi=:.4f}')    # pi=3.1416

# Can combine with conversion flags
user = 'Alice'
print(f'{user=!r}')    # user='Alice'
```

**Critical rule:** The `f'{expr=}'` form is strictly for debugging and development. **Never leave it in production log calls.** Use `logger.debug('x = %r', x)` in production code.

All other f-string features from 3.6 remain fully available:

```python
name, score = 'Alice', 98.765
print(f'Hello, {name}!')
print(f'{score:.2f}')           # '98.77'
print(f'{1_000_000:_}')         # '1_000_000'
print(f'{"left":<10}|')         # 'left      |'
print(f'{255:#x}')              # '0xff'
width, prec = 10, 4
print(f'{score:{width}.{prec}f}')  # '   98.7650'
```

**Pitfall: f-strings cannot contain backslashes inside `{}`** (fixed in Python 3.12, but NOT in 3.8):

```python
# SyntaxError in 3.8
# f'Value: {d["key"]}'  — same quote type inside and out
# f'Newline: {"\n"}'    — backslash in expression

# Workarounds:
key = 'name'
print(f'Value: {d[key]}')           # use variable
print(f"Value: {d['key']}")         # switch outer quotes
print(f'Newline: {chr(10)}')        # chr() for special chars
```

***

#### Variable Annotations (PEP 526)

```python
from typing import ClassVar, Dict, List, Optional

count: int = 0
name: str = 'Alice'

class Starship:
    stats: ClassVar[Dict[str, int]] = {}

    def __init__(self, name: str, crew: int) -> None:
        self.name = name
        self.crew: int = crew

# Annotations stored in __annotations__ — NOT enforced at runtime
x: int = 'not an int'  # no error at runtime — use mypy/pyright
```

***

#### Type Hints (PEP 484 + Python 3.8 additions)

##### Python 3.8 Typing Additions: `TypedDict`, `Literal`, `Final`, `Protocol`

These four major constructs entered the standard library `typing` module in Python 3.8.[^6]

###### `TypedDict` (PEP 589)

`TypedDict` creates a dict type with a fixed, known schema — each key maps to a declared type.

```python
from typing import TypedDict

class Movie(TypedDict):
    name: str
    year: int
    rating: float

# total=False — all keys are optional
class MoviePartial(TypedDict, total=False):
    name: str
    year: int

# Mixed required/optional via inheritance
class MovieRequired(TypedDict):
    name: str

class MovieFull(MovieRequired, total=False):
    year: int
    rating: float

# Functional syntax (for keys that are Python keywords or invalid identifiers)
Config = TypedDict('Config', {'from': str, 'to': str, 'retry-count': int})

movie: Movie = {'name': 'Blade Runner', 'year': 1982, 'rating': 8.1}
print(movie['name'])   # 'Blade Runner'
```

**Key rules:**

- `TypedDict` is **a type hint only** — the runtime object is a plain `dict`. No field validation occurs at runtime.
- `isinstance(movie, Movie)` raises `TypeError`. Never use `isinstance()` with `TypedDict`.[^6]
- Keys must be string literals — `TypedDict` does not support integer or other-type keys.
- Type checkers enforce the schema; your code does not.

###### `Literal` (PEP 586)

`Literal` restricts a parameter or return value to specific literal values:

```python
from typing import Literal

def set_log_level(level: Literal['debug', 'info', 'warning', 'error']) -> None:
    ...

def open_mode(mode: Literal['r', 'w', 'rb', 'wb']) -> None:
    ...

HttpMethod = Literal['GET', 'POST', 'PUT', 'DELETE', 'PATCH']

def request(method: HttpMethod, url: str) -> dict:
    ...

# Literal can hold integers, booleans, None, bytes, and enums too
def exit_code(code: Literal[0, 1]) -> None:
    ...
```

**Pitfall:** `Literal` is static metadata for type checkers only. At runtime, `Literal['a', 'b']` does NOT enforce that only `'a'` or `'b'` are passed. For runtime validation, use an explicit check or an `Enum`.

###### `Final` (PEP 591)

`Final` declares a name as non-reassignable and a class/method as non-overridable:

```python
from typing import Final

# Module-level constant
MAX_RETRIES: Final = 5
API_BASE_URL: Final[str] = 'https://api.example.com/v1'

# In a class
class Config:
    DEBUG: Final = False

    def __init__(self) -> None:
        self.id: Final = generate_id()  # instance-level Final

# Subclassing: @final prevents subclassing (Python 3.8+)
from typing import final

class Base:
    @final
    def critical_method(self) -> None:
        """Type checkers will flag overrides of this method."""
        ...

@final
class Singleton:
    """Type checkers will flag subclasses of this class."""
    ...
```

**Note:** `Final` and `@final` are enforced by type checkers, not the runtime. Reassigning a `Final` variable or subclassing a `@final` class raises no runtime error — it is a static-analysis concern.

###### `Protocol` (PEP 544) — Structural Subtyping

`Protocol` formalizes duck typing: a class satisfies a `Protocol` if it has the required methods/attributes, **regardless of inheritance**.[^6]

```python
from typing import Protocol, runtime_checkable

class Drawable(Protocol):
    def draw(self) -> None:
        ...

class Resizable(Protocol):
    def resize(self, factor: float) -> None:
        ...

class Widget(Drawable, Resizable, Protocol):
    """Combined protocol."""
    ...

# Any class with .draw() satisfies Drawable — no explicit inheritance needed
class Circle:
    def draw(self) -> None:
        print('Drawing circle')

def render(obj: Drawable) -> None:
    obj.draw()

render(Circle())   # type-safe — Circle structurally satisfies Drawable

# @runtime_checkable enables isinstance() checks at runtime
@runtime_checkable
class Closeable(Protocol):
    def close(self) -> None:
        ...

class Connection:
    def close(self) -> None:
        ...

print(isinstance(Connection(), Closeable))  # True
```

**Pitfall:** `isinstance()` with a `@runtime_checkable` Protocol only checks for **method existence**, not signatures. It does not verify argument types or return types. Never use it as a security or validation gate.

##### Other Typing Constructs

```python
from typing import (
    Any, Callable, Dict, FrozenSet, Generator, Generic,
    Iterable, Iterator, List, Optional, Set, Tuple,
    Type, TypeVar, Union,
)

T = TypeVar('T')
T_contra = TypeVar('T_contra', contravariant=True)
T_co = TypeVar('T_co', covariant=True)

def first(items: List[T]) -> Optional[T]:
    return items[0] if items else None

# Callable type — args types, return type
Handler = Callable[[str, int], bool]

# Optional[X] is shorthand for Union[X, None]
def find(key: str) -> Optional[str]:
    return cache.get(key)
```

***

#### Dataclasses (Python 3.7+ — stdlib in 3.8)

`dataclasses` is a full stdlib module in Python 3.7+. No backport needed in 3.8.[^7] Note: `dataclasses.KW_ONLY` and the `slots=True` decorator argument are **3.10+** — not available in 3.8.

```python
from dataclasses import dataclass, field
from typing import ClassVar, List, Optional

@dataclass
class User:
    id: int
    name: str
    email: str
    active: bool = True
    tags: List[str] = field(default_factory=list)

    def display_name(self) -> str:
        return f'{self.name} <{self.email}>'

    def __post_init__(self) -> None:
        # Called after __init__ — for validation and derived fields
        self.email = self.email.lower()
        if not self.email.endswith('.com'):
            raise ValueError(f'Email must end with .com: {self.email}')

@dataclass(frozen=True)   # immutable and hashable
class Coordinate:
    lat: float
    lon: float

@dataclass(order=True)    # enables __lt__, __le__, __gt__, __ge__
class Version:
    major: int
    minor: int
    patch: int = 0

    def __str__(self) -> str:
        return f'{self.major}.{self.minor}.{self.patch}'

# repr=False suppresses __repr__ generation (define your own)
@dataclass(repr=False)
class Node:
    value: int
    children: List['Node'] = field(default_factory=list)

    def __repr__(self) -> str:
        return f'Node({self.value!r}, children={len(self.children)})'
```

**Critical pitfalls:**

```python
# WRONG — mutable default raises ValueError
@dataclass
class Bad:
    items: List[int] = []   # ValueError: mutable default

# CORRECT — always use field(default_factory=...)
@dataclass
class Good:
    items: List[int] = field(default_factory=list)

# WRONG — non-default cannot follow default
@dataclass
class BadOrder:
    x: int = 0
    y: int    # TypeError: non-default argument 'y' follows default argument

# CORRECT — defaults must come after non-defaults
@dataclass
class GoodOrder:
    y: int
    x: int = 0
```

**`field()` parameters:**

```python
from dataclasses import field

@dataclass
class Config:
    host: str
    port: int = field(default=8080)
    _connection: object = field(default=None, init=False, repr=False)
    metadata: dict = field(default_factory=dict, compare=False, hash=False)
```

| Parameter | Meaning |
|---|---|
| `default` | Default value (immutable only) |
| `default_factory` | Callable producing the default (for mutable) |
| `init` | Include in `__init__` (default `True`) |
| `repr` | Include in `__repr__` (default `True`) |
| `compare` | Include in `__eq__` and `__lt__` etc. (default `True`) |
| `hash` | Include in `__hash__` (default: same as `compare`) |

***

#### Lists

Lists are mutable, ordered, heterogeneous sequences.[^3]

```python
lst = [1, 'two', 3.0, [4, 5]]

# Indexing and slicing
print(lst[0])    # 1
print(lst[-1])   # [4, 5]
print(lst[1:3])  # ['two', 3.0]
print(lst[::-1]) # reversed copy
print(lst[::2])  # every other element

# Mutation
lst.append(6)
lst.extend([7, 8])
lst.insert(0, 0)
lst.remove('two')
popped = lst.pop()
del lst[0]

# Sorting
nums = [3, 1, 4, 1, 5, 9]
nums.sort()                      # in-place, stable
nums.sort(key=lambda x: -x)      # descending
sorted_copy = sorted(nums)        # returns new list
nums.reverse()                    # in-place

import copy
lst2 = lst[:]               # shallow copy
lst3 = list(lst)            # also shallow copy
lst4 = copy.deepcopy(lst)   # deep copy
```

**Pitfall: List multiplication shares references:**

```python
# WRONG — all rows are the SAME object
matrix = [[0] * 3] * 3
matrix[0][0] = 99
print(matrix)  # [[99, 0, 0], [99, 0, 0], [99, 0, 0]]

# CORRECT — each row is a new object
matrix = [[0] * 3 for _ in range(3)]
matrix[0][0] = 99
print(matrix)  # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

**Pitfall: Never mutate a list while iterating it.** Iterate a copy or rebuild:

```python
# WRONG — skips elements
for item in lst:
    if should_remove(item):
        lst.remove(item)

# CORRECT
lst = [item for item in lst if not should_remove(item)]
```

##### List Comprehensions

```python
squares  = [x**2 for x in range(10)]
evens    = [x for x in range(20) if x % 2 == 0]
flat     = [item for sublist in nested for item in sublist]
matrix   = [[i * j for j in range(1, 4)] for i in range(1, 4)]

# Comprehension variables do NOT leak in Python 3 (unlike Python 2)
x = 10
result = [x for x in range(5)]
print(x)  # 10 — x is unchanged
```

***

#### Tuples

Tuples are immutable, ordered sequences.[^3]

```python
t = (1, 2, 3)
single = (42,)   # trailing comma required for single-element tuple
empty  = ()

# Unpacking
a, b, c = t
a, b = b, a      # swap without temp

# Extended unpacking (Python 3 only)
first, *rest = [1, 2, 3, 4, 5]
head, *middle, tail = [1, 2, 3, 4, 5]
print(head, middle, tail)  # 1 [2, 3, 4] 5
```

**Iterable unpacking in `return` and `yield` (new in 3.8):**[^1] Parentheses are no longer required around starred expressions in `return` and `yield`:

```python
def parse(family: str):
    lastname, *members = family.split()
    return lastname.upper(), *members   # valid in 3.8 — was SyntaxError before

parse('simpsons homer marge bart')   # ('SIMPSONS', 'homer', 'marge', 'bart')
```

***

#### Dictionaries

Dicts in Python 3.7+ are ordered by insertion order — this is a **language guarantee** (not just a CPython implementation detail as it was in 3.6).[^2]

```python
d = {'name': 'Alice', 'age': 30}

print(d['name'])
print(d.get('height', 0))
print(d.setdefault('role', 'user'))

d['email'] = 'alice@example.com'
del d['age']
d.update({'city': 'Berlin', 'country': 'DE'})

for k, v in d.items():
    print(k, v)
```

**`reversed()` now works on dicts (new in 3.8):**[^1]

```python
d = {'a': 1, 'b': 2, 'c': 3}
print(list(reversed(d)))         # ['c', 'b', 'a']
print(list(reversed(d.items()))) # [('c', 3), ('b', 2), ('a', 1)]
```

**Pitfall: Never modify a dict while iterating its keys:**

```python
# WRONG — RuntimeError: dictionary changed size during iteration
for k in d:
    if should_delete(k):
        del d[k]

# CORRECT — iterate a snapshot of keys
for k in list(d.keys()):
    if should_delete(k):
        del d[k]

# MOST IDIOMATIC — rebuild the dict
d = {k: v for k, v in d.items() if not should_delete(k)}
```

##### Dict Merging

```python
# Merge with ** (since 3.5) — last dict wins on collision
merged = {**defaults, **overrides}
config = {**base_config, 'debug': True, 'port': 9000}

# Dict comprehension
squares  = {x: x**2 for x in range(10)}
inverted = {v: k for k, v in d.items()}
filtered = {k: v for k, v in d.items() if v is not None}
```

**Note:** Python 3.9 adds `|` and `|=` dict merge operators. These are **not available in 3.8** — continue using `{**a, **b}`.

##### `collections.defaultdict`, `Counter`, `OrderedDict`

```python
from collections import defaultdict, Counter, OrderedDict

# defaultdict
word_count: defaultdict = defaultdict(int)
for word in text.split():
    word_count[word] += 1

graph: defaultdict = defaultdict(list)
graph['A'].append('B')

# Counter
c = Counter('abracadabra')
print(c.most_common(3))   # [('a', 5), ('b', 2), ('r', 2)]
c.update('hello')

# OrderedDict — still useful for move_to_end() and LRU patterns
od = OrderedDict()
od['first'] = 1
od.move_to_end('first')          # move to last
od.move_to_end('first', last=False)  # move to front
```

***

#### Sets

Sets are mutable, unordered collections of unique, hashable elements.[^3]

```python
s = {1, 2, 3}
s2 = set([1, 2, 2, 3])
empty_set = set()          # NOT {} — that's an empty dict!

s.add(4)
s.discard(10)   # no error if absent
s.remove(4)     # raises KeyError if absent

a = {1, 2, 3, 4}
b = {3, 4, 5, 6}
print(a | b)     # union: {1, 2, 3, 4, 5, 6}
print(a & b)     # intersection: {3, 4}
print(a - b)     # difference: {1, 2}
print(a ^ b)     # symmetric difference: {1, 2, 5, 6}
print(a.isdisjoint(b))   # False

fs = frozenset([1, 2, 3])  # immutable and hashable — valid dict key

unique_squares = {x**2 for x in range(-5, 6)}  # set comprehension
```

***

#### `range` in Python 3

`range()` is always a lazy object — no `xrange()` exists.[^3]

```python
for i in range(1_000_000):
    process(i)

r = range(10)
print(len(r))       # 10
print(r[3])         # 3
print(4 in r)       # True — O(1) membership test for ints
print(r[::-1])      # range(9, -1, -1)
indices = list(range(10))   # explicit list when needed
```

***

#### Control Flow

##### `if / elif / else`

```python
score = 85
if score >= 90:
    grade = 'A'
elif score >= 80:
    grade = 'B'
elif score >= 70:
    grade = 'C'
else:
    grade = 'F'

label = 'pass' if score >= 60 else 'fail'
```

##### `for` Loops

```python
for i, item in enumerate(my_list):
    print(i, item)

for k, v in d.items():
    process(k, v)

for a, b in zip(list1, list2):    # stops at shortest
    print(a, b)

from itertools import zip_longest
for a, b in zip_longest(list1, list2, fillvalue=None):
    print(a, b)

# for/else — else runs only if no break occurred
for item in collection:
    if matches(item):
        result = item
        break
else:
    result = default_value
```

##### `continue` now legal in `finally` (new in 3.8)

Prior to 3.8, `continue` inside a `finally` block was a `SyntaxError`. It is now allowed:[^1]

```python
for i in range(10):
    try:
        process(i)
    except ProcessingError:
        log_error(i)
    finally:
        cleanup(i)
        continue   # valid in 3.8 — was SyntaxError in 3.7 and earlier
```

***

#### Functions

##### Defining Functions

```python
from typing import Optional

def greet(
    name: str,
    greeting: str = 'Hello',
    *,
    punctuation: str = '!',
) -> str:
    """Return a greeting string.

    Args:
        name: The person to greet.
        greeting: The greeting word.
        punctuation: Trailing punctuation (keyword-only).

    Returns:
        A formatted greeting string.
    """
    return f'{greeting}, {name}{punctuation}'
```

##### Full Parameter Syntax (3.8 — all four zones)

```python
def full_example(
    pos_only_a: int,
    pos_only_b: str,
    /,
    normal_c: float,
    normal_d: bool = True,
    *args: object,
    kw_only_e: str,
    kw_only_f: int = 0,
    **kwargs: object,
) -> None:
    pass

full_example(1, 'x', 3.0, True, 'extra', kw_only_e='value')
```

##### `nonlocal` and Closures

```python
from typing import Callable

def make_counter(start: int = 0) -> Callable[[], int]:
    count = start

    def counter() -> int:
        nonlocal count
        count += 1
        return count

    return counter

c = make_counter()
print(c())  # 1
print(c())  # 2
```

**Pitfall: Late binding in closures:**

```python
# WRONG — all functions see the final value of i (4)
funcs = [lambda: i for i in range(5)]
print([f() for f in funcs])  # [4, 4, 4, 4, 4]

# CORRECT — capture current value with default argument
funcs = [lambda i=i: i for i in range(5)]
print([f() for f in funcs])  # [0, 1, 2, 3, 4]
```

##### `functools.lru_cache` — New Simplified Syntax (new in 3.8)

`@lru_cache` can now be used as a bare decorator without parentheses:[^1]

```python
import functools

# Old syntax (still valid)
@functools.lru_cache(maxsize=128)
def fibonacci(n: int) -> int:
    if n < 2:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

# New 3.8 syntax — bare decorator, uses default maxsize=128
@functools.lru_cache
def fibonacci_v2(n: int) -> int:
    if n < 2:
        return n
    return fibonacci_v2(n - 1) + fibonacci_v2(n - 2)

# Cache management
fibonacci.cache_info()    # CacheInfo(hits=..., misses=..., maxsize=128, currsize=...)
fibonacci.cache_clear()
```

##### `functools.cached_property` (new in 3.8)

`cached_property` computes a property once and caches it on the instance for the object's lifetime:[^13]

```python
import functools
import statistics

class Dataset:
    def __init__(self, data):
        self.data = data

    @functools.cached_property
    def mean(self) -> float:
        return statistics.mean(self.data)

    @functools.cached_property
    def variance(self) -> float:
        return statistics.variance(self.data)

ds = Dataset([1, 2, 3, 4, 5])
print(ds.mean)       # computed once
print(ds.mean)       # returned from cache — no recomputation
```

**Pitfall:** `cached_property` stores the value in `instance.__dict__`. It therefore does NOT work with classes that define `__slots__` (no `__dict__`) — accessing the property raises `TypeError`. Use a manual `_cache` attribute pattern for those cases.

**Threading note:** In Python 3.8, `cached_property` **is thread-safe**: it holds an internal lock while computing, so the getter runs at most once per instance even under concurrent access. The catch is that the lock is per-property, shared across **all instances** of the class, so concurrent first-time access on many instances serializes on one lock and can contend badly.[^13] (Python 3.12 later removed this locking entirely — but that does not affect 3.8.) If the cached computation is slow and instances are hit concurrently, compute the value eagerly or manage your own per-instance lock inside the getter.

##### `functools.singledispatchmethod` (new in 3.8)

`singledispatchmethod` extends `singledispatch` to class methods:[^1]

```python
from functools import singledispatchmethod

class Processor:
    @singledispatchmethod
    def process(self, arg: object) -> str:
        raise NotImplementedError(f'Cannot process {type(arg).__name__}')

    @process.register(str)
    def _(self, arg: str) -> str:
        return arg.upper()

    @process.register(int)
    @process.register(float)
    def _(self, arg) -> str:
        return str(arg * 2)

    @process.register(list)
    def _(self, arg: list) -> str:
        return ', '.join(str(x) for x in arg)

p = Processor()
print(p.process('hello'))    # HELLO
print(p.process(42))         # 84
print(p.process([1, 2, 3]))  # 1, 2, 3
```

***

#### Generators and Iterators

##### The Iterator Protocol

An iterator in Python 3 uses `__next__()`:[^3]

```python
class CountDown:
    def __init__(self, n: int) -> None:
        self.n = n

    def __iter__(self) -> 'CountDown':
        return self

    def __next__(self) -> int:
        if self.n <= 0:
            raise StopIteration
        val = self.n
        self.n -= 1
        return val

for x in CountDown(3):
    print(x)   # 3, 2, 1
```

##### Generator Functions and Expressions

```python
from typing import Generator

def fibonacci() -> Generator[int, None, None]:
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

gen = fibonacci()
print([next(gen) for _ in range(10)])  # [0, 1, 1, 2, 3, 5, 8, 13, 21, 34]

# Generator expressions — O(1) memory
total = sum(x**2 for x in range(1_000_000))
```

##### `itertools` — Efficient Iteration

```python
import itertools

# Infinite iterators
itertools.count(10)          # 10, 11, 12, ...
itertools.cycle('ABC')       # A, B, C, A, B, C, ...
itertools.repeat(42, 3)      # 42, 42, 42

# Finite
list(itertools.chain([1, 2], [3, 4]))                  # [1, 2, 3, 4]
list(itertools.islice(itertools.count(), 5))            # [0, 1, 2, 3, 4]
list(itertools.dropwhile(lambda x: x < 5, [1, 4, 6]))  # [6]
list(itertools.takewhile(lambda x: x < 5, [1, 4, 6]))  # [1, 4]

# accumulate: now supports initial= keyword (new in 3.8)
list(itertools.accumulate([10, 5, 30, 15], initial=1000))
# [1000, 1010, 1015, 1045, 1060]

# Combinatorics
list(itertools.product('AB', repeat=2))   # AA, AB, BA, BB
list(itertools.permutations('ABC', 2))    # 6 items
list(itertools.combinations('ABC', 2))   # AB, AC, BC

# Grouping — input MUST be sorted on the key
data = sorted([('A', 1), ('B', 2), ('A', 3)], key=lambda x: x[0])
for key, group in itertools.groupby(data, key=lambda x: x[0]):
    print(key, list(group))
```

***

#### Classes and Object-Oriented Programming

##### Class Definition

```python
import math

class Animal:
    kingdom = 'Animalia'  # class attribute

    def __init__(self, name: str, species: str) -> None:
        self.name = name
        self.species = species
        self._energy = 100
        self.__secret = 'x'   # name-mangled to _Animal__secret

    def __repr__(self) -> str:
        return f'Animal(name={self.name!r}, species={self.species!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Animal):
            return NotImplemented
        return self.name == other.name and self.species == other.species

    def __hash__(self) -> int:
        # Python 3: defining __eq__ silently sets __hash__ = None unless
        # you define __hash__ too. Always pair them.
        return hash((self.name, self.species))
```

##### Inheritance and `super()`

```python
class Dog(Animal):
    def __init__(self, name: str, breed: str) -> None:
        super().__init__(name, 'Canis lupus familiaris')  # zero-arg super
        self.breed = breed

    def speak(self) -> str:
        return f'{self.name} says: Woof!'
```

##### Properties, `classmethod`, `staticmethod`

```python
class Circle:
    def __init__(self, radius: float) -> None:
        self._radius = radius

    @property
    def radius(self) -> float:
        return self._radius

    @radius.setter
    def radius(self, value: float) -> None:
        if value < 0:
            raise ValueError(f'Radius must be non-negative, got {value}')
        self._radius = value

    @property
    def area(self) -> float:
        return math.pi * self._radius ** 2

    @classmethod
    def unit(cls) -> 'Circle':
        return cls(1.0)

    @staticmethod
    def is_valid_radius(r: float) -> bool:
        return r >= 0
```

##### `__slots__`

`__slots__` eliminates the per-instance `__dict__`, reducing memory significantly for classes with many instances:[^3]

```python
class Point:
    __slots__ = ('x', 'y')

    def __init__(self, x: float, y: float) -> None:
        self.x = x
        self.y = y
```

**Caveats:**

- Undeclared attributes raise `AttributeError`.
- No weak references unless `'__weakref__'` is listed.
- Subclasses must also declare `__slots__ = ()` to preserve memory savings.
- Incompatible with `functools.cached_property`.

##### Important Dunder Methods

| Method | Purpose |
|---|---|
| `__init__(self, ...)` | Instance initializer |
| `__repr__(self)` | Unambiguous debug representation |
| `__str__(self)` | Readable display representation |
| `__len__(self)` | `len(obj)` |
| `__getitem__(self, key)` | `obj[key]` |
| `__setitem__(self, key, val)` | `obj[key] = val` |
| `__delitem__(self, key)` | `del obj[key]` |
| `__contains__(self, item)` | `item in obj` |
| `__iter__(self)` | Return iterator |
| `__next__(self)` | Next value (NOT `next` — Python 3 spelling) |
| `__eq__(self, other)` | Equality `==` |
| `__lt__`, `__le__`, `__gt__`, `__ge__` | Rich comparisons |
| `__hash__(self)` | Hash value — must pair with `__eq__` |
| `__bool__(self)` | Bool cast (NOT `__nonzero__`) |
| `__call__(self, ...)` | Make instance callable |
| `__enter__`, `__exit__` | Context manager protocol |
| `__init_subclass__` | Subclass creation hook |
| `__set_name__` | Descriptor attribute name hook |
| `__class_getitem__` | Allows `Class[T]` subscript syntax |

##### Abstract Base Classes

```python
from abc import ABC, abstractmethod

class Shape(ABC):
    @abstractmethod
    def area(self) -> float:
        """Return the area of the shape."""

    @abstractmethod
    def perimeter(self) -> float:
        """Return the perimeter of the shape."""

class Rectangle(Shape):
    def __init__(self, width: float, height: float) -> None:
        self.width = width
        self.height = height

    def area(self) -> float:
        return self.width * self.height

    def perimeter(self) -> float:
        return 2 * (self.width + self.height)
```

##### Enum

```python
from enum import Enum, IntEnum, Flag, auto

class Color(Enum):
    RED   = 1
    GREEN = 2
    BLUE  = 3

class Direction(Enum):
    NORTH = auto()
    SOUTH = auto()
    EAST  = auto()
    WEST  = auto()

class HttpStatus(IntEnum):
    OK           = 200
    NOT_FOUND    = 404
    SERVER_ERROR = 500

class Permission(Flag):
    READ    = auto()
    WRITE   = auto()
    EXECUTE = auto()
    ALL     = READ | WRITE | EXECUTE

user_perms = Permission.READ | Permission.WRITE
print(Permission.READ in user_perms)     # True
print(Permission.EXECUTE in user_perms)  # False
```

***

#### Decorators

```python
import functools
import logging
import time
from typing import Callable, Tuple, Type, TypeVar

F = TypeVar('F', bound=Callable)

def timer(func: F) -> F:
    @functools.wraps(func)   # MANDATORY — preserves __name__, __doc__, __annotations__
    def wrapper(*args, **kwargs):
        start = time.perf_counter()
        result = func(*args, **kwargs)
        elapsed = time.perf_counter() - start
        print(f'{func.__name__} took {elapsed:.4f}s')
        return result
    return wrapper  # type: ignore[return-value]

@timer
def slow_function(n: int) -> int:
    return sum(range(n))

def retry(
    max_attempts: int = 3,
    exceptions: Tuple[Type[Exception], ...] = (Exception,),
    backoff: float = 0.0,
) -> Callable[[F], F]:
    def decorator(func: F) -> F:
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            last_exc: Exception = RuntimeError('No attempts made')
            for attempt in range(1, max_attempts + 1):
                try:
                    return func(*args, **kwargs)
                except exceptions as e:
                    last_exc = e
                    logging.warning(
                        'Attempt %d/%d for %s failed: %s',
                        attempt, max_attempts, func.__name__, e,
                    )
                    if backoff and attempt < max_attempts:
                        time.sleep(backoff * attempt)
            raise last_exc
        return wrapper  # type: ignore[return-value]
    return decorator

@retry(max_attempts=5, exceptions=(IOError, OSError), backoff=0.5)
def read_remote_file(url: str) -> bytes:
    pass
```

***

#### Exception Handling

##### Exception Hierarchy

```
BaseException
 ├── SystemExit
 ├── KeyboardInterrupt
 ├── GeneratorExit
 └── Exception
      ├── ArithmeticError
      │    ├── ZeroDivisionError
      │    └── OverflowError
      ├── LookupError
      │    ├── IndexError
      │    └── KeyError
      ├── ValueError
      │    └── UnicodeError
      │         ├── UnicodeDecodeError
      │         └── UnicodeEncodeError
      ├── TypeError
      ├── OSError  (IOError, EnvironmentError are aliases)
      │    ├── FileNotFoundError
      │    ├── PermissionError
      │    ├── IsADirectoryError
      │    └── TimeoutError
      ├── AttributeError
      ├── ImportError
      │    └── ModuleNotFoundError
      ├── RuntimeError
      │    └── RecursionError
      ├── StopIteration
      └── ...
```

**Python 3.8-specific:** `asyncio.CancelledError` now inherits from `BaseException`, not `Exception`.[^1] This means bare `except Exception:` no longer accidentally swallows task cancellations.

```python
# Pre-3.8: CancelledError was an Exception, could be caught by accident
# 3.8+: CancelledError is a BaseException — never caught by except Exception
try:
    await some_task()
except asyncio.CancelledError:
    cleanup()
    raise   # ALWAYS re-raise CancelledError — do not suppress it
```

##### Full `try/except/else/finally`

```python
import json
import logging

logger = logging.getLogger(__name__)

def read_config(path: str) -> dict:
    try:
        with open(path, 'r', encoding='utf-8') as f:
            config = json.load(f)
    except FileNotFoundError:
        logger.error('Config file not found: %r', path)
        raise
    except json.JSONDecodeError as e:
        raise ValueError(f'Config file {path!r} contains invalid JSON') from e
    except PermissionError as e:
        raise RuntimeError(f'Cannot read {path!r}: permission denied') from e
    except OSError as e:
        logger.error('Cannot read config %r: %s', path, e)
        raise
    else:
        return config   # only runs when no exception occurred
    finally:
        logger.debug('read_config finished for %r', path)  # always runs
```

##### Exception Chaining

```python
# Explicit chaining: sets __cause__ — "The above exception was the direct cause of..."
try:
    int('N/A')
except ValueError as e:
    raise ConfigError('Invalid integer value') from e

# Suppress context: raise from None
try:
    int('N/A')
except ValueError:
    raise ConfigError('Bad value') from None   # hides original ValueError

# Always use bare raise to re-raise — never 'raise e' (resets traceback to current line)
try:
    risky()
except Exception:
    logger.exception('Unexpected error')
    raise   # CORRECT — preserves original traceback
```

##### Custom Exceptions

```python
from typing import Optional

class AppError(Exception):
    """Base for all application-specific exceptions."""

class ConfigError(AppError):
    def __init__(self, key: str, message: str = '') -> None:
        self.key = key
        msg = f'Config error for key {key!r}'
        if message:
            msg += f': {message}'
        super().__init__(msg)

class NetworkError(AppError):
    def __init__(self, url: str, status_code: Optional[int] = None) -> None:
        self.url = url
        self.status_code = status_code
        parts = [f'Network error for {url!r}']
        if status_code:
            parts.append(f'HTTP {status_code}')
        super().__init__(': '.join(parts))
```

***

#### Context Managers and the `with` Statement

```python
# Single resource
with open('data.txt', 'r', encoding='utf-8') as f:
    content = f.read()

# Multiple resources — the parenthesized form is 3.10+; use backslash
# continuation (or contextlib.ExitStack) in 3.8
with open('input.txt', encoding='utf-8') as fin, \
     open('output.txt', 'w', encoding='utf-8') as fout:
    for line in fin:
        fout.write(line.upper())
```

##### Writing Context Managers

```python
class DatabaseTransaction:
    def __init__(self, connection) -> None:
        self.conn = connection

    def __enter__(self):
        self.conn.begin()
        return self.conn

    def __exit__(self, exc_type, exc_val, exc_tb):
        if exc_type is None:
            self.conn.commit()
        else:
            self.conn.rollback()
        return False   # never suppress exceptions

from contextlib import contextmanager, suppress
import logging

@contextmanager
def logged_operation(name: str):
    logging.info('Starting: %s', name)
    try:
        yield
    except Exception as e:
        logging.error('Failed: %s — %s', name, e)
        raise
    else:
        logging.info('Completed: %s', name)

# contextlib.suppress — idiomatic for expected no-op exceptions
import os
with suppress(FileNotFoundError):
    os.remove('/tmp/optional_cache_file')
```

***

#### File I/O

##### Text Files

Always pass `encoding` explicitly:[^9]

```python
with open('file.txt', 'r', encoding='utf-8') as f:
    content = f.read()

with open('large.txt', 'r', encoding='utf-8') as f:
    for line in f:                # lazy iteration — O(1) memory
        process(line.rstrip('\n'))

with open('out.txt', 'w', encoding='utf-8') as f:
    f.write('line 1\n')
    f.writelines(['line 2\n', 'line 3\n'])
```

**`errors` parameter values:**

- `'strict'` (default): raises `UnicodeDecodeError`
- `'replace'`: substitutes undecodable bytes with U+FFFD `�` when decoding (and `?` when encoding)
- `'ignore'`: silently drops undecodable bytes
- `'surrogateescape'`: preserves byte values as surrogate codepoints (useful for round-tripping arbitrary bytes)

##### Binary Files

```python
with open('image.png', 'rb') as f:
    data = f.read()

CHUNK_SIZE = 65536  # 64 KiB
with open('large.bin', 'rb') as f:
    while chunk := f.read(CHUNK_SIZE):   # walrus operator!
        process(chunk)
```

**Note:** The walrus operator in `while chunk := f.read(CHUNK_SIZE):` is a natural fit for binary I/O in 3.8 — it eliminates the repetitive `chunk = ...; while chunk: ...; chunk = ...` pattern.

##### `pathlib` — Preferred Path API

`pathlib.Path` objects implement `os.PathLike` and work with all standard library functions:[^8]

```python
from pathlib import Path

p = Path('/home/alice/data.csv')
p = Path.home() / 'data' / 'output.csv'

print(p.name)        # 'output.csv'
print(p.stem)        # 'output'
print(p.suffix)      # '.csv'
print(p.parent)      # PosixPath('/home/alice/data')

# Read/write directly (no open() call needed for small files)
text = p.read_text(encoding='utf-8')
p.write_text('hello\n', encoding='utf-8')
data = p.read_bytes()
p.write_bytes(b'\x00\x01')

# Directory operations
p.parent.mkdir(parents=True, exist_ok=True)
p.unlink(missing_ok=True)    # missing_ok=True added in Python 3.8!
p.rename(p.with_suffix('.txt'))

# Globbing
for csv_file in Path('.').glob('**/*.csv'):
    process(csv_file)

# Use Path directly with open()
with open(Path('data.txt'), 'r', encoding='utf-8') as f:
    text = f.read()
```

**`Path.unlink(missing_ok=True)` is new in 3.8** — previously you had to wrap it in `suppress(FileNotFoundError)`.

##### Temporary Files

```python
import tempfile

with tempfile.NamedTemporaryFile(
    suffix='.csv', mode='w', encoding='utf-8', delete=True,
) as tmp:
    tmp.write('col1,col2\n')
    tmp.flush()
    process(tmp.name)

with tempfile.TemporaryDirectory() as tmpdir:
    work_in(tmpdir)   # directory deleted on exit
```

**Never use `tempfile.mktemp()`** — deprecated, TOCTOU-vulnerable.

***

#### `async`/`await` and `asyncio` (Python 3.8)

##### `asyncio.run()` — Now Stable (new in 3.8)

`asyncio.run()` was added in 3.7 as a provisional API; in Python 3.8 it graduates to stable.[^1] It is the **canonical** entry point for running async programs:

```python
import asyncio

async def main() -> None:
    result = await fetch_data('https://api.example.com/users')
    print(result)

# CORRECT for Python 3.8+ — use asyncio.run()
asyncio.run(main())

# OLD pattern (3.6-era) — no longer needed
# loop = asyncio.get_event_loop()
# loop.run_until_complete(main())
# loop.close()
```

`asyncio.run()` creates a new event loop, runs the coroutine to completion, and properly closes the loop, including running shutdown callbacks. It should be called **once** at the top level of your program — **never inside an async function** (that raises `RuntimeError: asyncio.run() cannot be called from a running event loop`).

##### Task Naming (new in 3.8)

Tasks can now be named for observability:[^1]

```python
async def main() -> None:
    task = asyncio.create_task(
        fetch_user(user_id),
        name=f'fetch_user_{user_id}',   # new in 3.8
    )
    print(task.get_name())   # 'fetch_user_42'
    result = await task
```

##### Concurrent Tasks with `gather`

```python
from typing import List

async def fetch_all(urls: List[str]) -> List[str]:
    tasks = [fetch_data(url) for url in urls]
    results = await asyncio.gather(*tasks)
    return list(results)

# return_exceptions=True — collect exceptions alongside results
results = await asyncio.gather(*tasks, return_exceptions=True)
for r in results:
    if isinstance(r, Exception):
        handle_error(r)
    else:
        process(r)
```

##### Bounded Concurrency with Semaphore

```python
async def limited_fetch(
    semaphore: asyncio.Semaphore,
    session,
    url: str,
) -> str:
    async with semaphore:
        async with session.get(url) as response:
            return await response.text()

async def fetch_bounded(urls: List[str], limit: int = 50) -> List[str]:
    sem = asyncio.Semaphore(limit)
    async with aiohttp.ClientSession() as session:
        tasks = [limited_fetch(sem, session, url) for url in urls]
        return await asyncio.gather(*tasks)
```

##### Timeouts

```python
try:
    result = await asyncio.wait_for(
        slow_operation(),
        timeout=5.0,   # seconds
    )
except asyncio.TimeoutError:
    handle_timeout()
```

##### `run_in_executor` — Blocking Calls

```python
from pathlib import Path

async def read_file_async(path: str) -> str:
    loop = asyncio.get_running_loop()   # use get_running_loop() in 3.8, not get_event_loop()
    # run_in_executor passes positional args only — use a lambda (or
    # functools.partial) to forward keyword arguments like encoding
    content = await loop.run_in_executor(
        None, lambda: Path(path).read_text(encoding='utf-8'))
    return content
```

**`asyncio.get_running_loop()` vs `asyncio.get_event_loop()`:** In Python 3.8, use `get_running_loop()` (added in 3.7) inside async code — it raises `RuntimeError` if no loop is running, which catches bugs. `get_event_loop()` silently creates a new loop in some contexts, masking issues.

##### Async Generators and Comprehensions

```python
from typing import AsyncGenerator

async def ticker(delay: float, count: int) -> AsyncGenerator[int, None]:
    for i in range(count):
        yield i
        await asyncio.sleep(delay)

async def collect() -> list:
    return [i async for i in ticker(0.01, 10)]

async def filtered() -> list:
    return [await fetch(url) for url in urls if await is_valid(url)]
```

##### `asyncio.CancelledError` is now a `BaseException` (breaking change in 3.8)

```python
# CRITICAL: Always re-raise CancelledError
async def worker():
    try:
        await long_running_task()
    except asyncio.CancelledError:
        await cleanup()
        raise   # MANDATORY — never suppress CancelledError
    except Exception as e:
        logger.error('Worker error: %s', e)
        raise
```

##### asyncio Pitfalls

```python
# PITFALL 1: Forgetting await — coroutine silently does nothing
async def broken():
    result = fetch_data('url')   # Missing await — creates coroutine object, never runs
    # Fix:
    result = await fetch_data('url')

# PITFALL 2: Blocking call freezes the event loop
async def broken2():
    import requests
    r = requests.get('https://example.com')  # BLOCKS entire event loop!
    # Fix: use aiohttp, httpx, or loop.run_in_executor()

# PITFALL 3: Calling asyncio.run() inside async code
async def broken3():
    asyncio.run(other_coroutine())   # RuntimeError: cannot be called from a running event loop
    # Fix:
    await other_coroutine()

# PITFALL 4: No timeout — one slow I/O can hang indefinitely
async def broken4():
    result = await slow_api_call()  # hangs forever if server doesn't respond
    # Fix:
    result = await asyncio.wait_for(slow_api_call(), timeout=10.0)
```

##### Windows asyncio: `ProactorEventLoop` default (new in 3.8)

On Windows, Python 3.8 switches the default event loop to `ProactorEventLoop`, which supports subprocess and IOCP natively.[^1] If your code relied on `SelectorEventLoop` on Windows, add:

```python
import asyncio
import sys

if sys.platform == 'win32':
    asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())
```

***

#### New in 3.8: `importlib.metadata`

Python 3.8 introduces `importlib.metadata` (as a provisional API) for reading installed package metadata without importing the package:[^14]

```python
from importlib.metadata import PackageNotFoundError, requires, version

# Get installed version of a package
print(version('requests'))   # e.g. '2.28.2'

# Get declared dependencies (list of requirement strings, or None)
print(requires('requests'))

# Check if a package is installed
def is_installed(package: str) -> bool:
    try:
        version(package)
        return True
    except PackageNotFoundError:
        return False
```

**Note:** `importlib.metadata.packages_distributions()` is **3.10+** — not available in 3.8.

***

#### Modules and Packages

```python
import os
from pathlib import Path
from typing import Optional

# Absolute imports are the default in Python 3 — no __future__ needed

# Explicit relative imports
from . import sibling_module
from .. import parent_module
from .utils import helper_function

__all__ = ['PublicClass', 'public_function', 'CONSTANT']
CONSTANT: int = 42

def _private_helper() -> None:
    pass
```

***

#### The `secrets` Module

`secrets` provides cryptographically strong random values. Always use it for security-sensitive values:[^10]

```python
import secrets

token_bytes = secrets.token_bytes(32)     # 32 random bytes
token_hex   = secrets.token_hex(32)       # 64 hex chars — 32 bytes entropy
token_url   = secrets.token_urlsafe(32)   # URL-safe base64

n = secrets.randbelow(1_000_000)          # secure random int in [0, 1_000_000)
```

**Rule:** Use `secrets` for all tokens, CSRF values, password reset links, API keys, and session IDs. **Never use the `random` module for security values** — it is a deterministic PRNG and is predictable.

***

#### Secure Coding Patterns

##### Input Validation

```python
def validate_age(value: object) -> int:
    try:
        age = int(str(value))
    except (ValueError, TypeError):
        raise ValueError(f'Age must be an integer, got {value!r}')
    if not (0 <= age <= 150):
        raise ValueError(f'Age out of range: {age}')
    return age
```

##### Path Traversal Prevention

```python
from pathlib import Path

def safe_read(base_dir: str, user_filename: str) -> str:
    base = Path(base_dir).resolve()
    requested = (base / user_filename).resolve()
    try:
        requested.relative_to(base)
    except ValueError:
        raise PermissionError(
            f'Access denied: {user_filename!r} escapes base directory'
        )
    return requested.read_text(encoding='utf-8')
```

##### SQL Injection Prevention

```python
import sqlite3

# DANGEROUS — never interpolate user input into SQL
def bad_query(conn, user_id):
    conn.execute(f'SELECT * FROM users WHERE id = {user_id}')  # injection!

# SAFE — parameterized queries only
def safe_query(conn: sqlite3.Connection, user_id: int) -> list:
    cursor = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,))
    return cursor.fetchall()
```

##### Password Hashing

```python
import bcrypt  # pip install bcrypt

def hash_password(password: str) -> bytes:
    salt = bcrypt.gensalt(rounds=12)
    return bcrypt.hashpw(password.encode('utf-8'), salt)

def verify_password(stored_hash: bytes, provided: str) -> bool:
    return bcrypt.checkpw(provided.encode('utf-8'), stored_hash)

# Stdlib fallback: hashlib.scrypt (requires OpenSSL)
import hashlib, os

def hash_password_scrypt(password: str) -> str:
    salt = os.urandom(16)
    dk = hashlib.scrypt(
        password.encode('utf-8'), salt=salt,
        n=2**14, r=8, p=1, dklen=32,
    )
    return salt.hex() + ':' + dk.hex()
```

##### XML External Entity (XXE) Handling

`xml.etree.ElementTree` has never resolved external entities (it raises a `ParseError` on undefined entities), and since Python 3.7.1 (and 3.6.8) the `xml.sax` and `xml.dom` parsers no longer process external entities by default either (bpo-17239).[^15] Denial-of-service vectors such as entity-expansion bombs ("billion laughs") remain, so for untrusted XML still use `defusedxml`:

```python
# Reasonable for trusted XML
import xml.etree.ElementTree as ET
tree = ET.parse('document.xml')

# For untrusted XML, use defusedxml:
# pip install defusedxml
import defusedxml.ElementTree as ET
tree = ET.parse('untrusted.xml')
```

***

#### Logging

```python
import logging

logger = logging.getLogger(__name__)

def setup_logging(level: str = 'INFO') -> None:
    logging.basicConfig(
        level=getattr(logging, level.upper()),
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        datefmt='%Y-%m-%dT%H:%M:%S',
        force=True,   # force=True is NEW in 3.8 — reconfigures existing handlers
    )
```

**`force=True` in `basicConfig` is new in 3.8.** Previously, once `basicConfig()` had been called, subsequent calls were silently ignored. `force=True` removes existing handlers and reconfigures the root logger — useful in interactive environments and Jupyter notebooks.[^1]

```python
# Use % style in logging calls — NOT f-strings
# f-strings are evaluated even when the log level is disabled
logger.debug('Processing item: %r', item)
logger.info('Loaded %d records', count)
logger.warning('Config key %r missing, using default', key)
logger.error('Failed to connect: %s', exc)

try:
    risky()
except Exception:
    logger.exception('Unexpected error')  # automatically includes traceback

# Structured logging for JSON log pipelines
import json

class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        return json.dumps({
            'time': self.formatTime(record),
            'level': record.levelname,
            'name': record.name,
            'msg': record.getMessage(),
        }, ensure_ascii=False)
```

***

#### Regular Expressions

```python
import re

EMAIL_RE = re.compile(r'^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$')

m = EMAIL_RE.match(email)
matches = re.findall(r'\d+', text)
cleaned = re.sub(r'<[^>]+>', '', html)

# Named groups — dict-style access
DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')
m = DATE_RE.match('2019-10-14')
if m:
    print(m['year'], m['month'], m['day'])   # dict-style subscript (3.6+)

# Unicode name escape support in re (new in 3.8)
copyright_re = re.compile(r'\N{copyright sign}\s*(\d{4})')
print(copyright_re.search('Copyright © 2019').group(1))  # '2019'
```

**Pitfall: Greedy vs non-greedy:**

```python
html = '<b>bold</b> and <i>italic</i>'
re.findall(r'<.+>', html)    # ['<b>bold</b> and <i>italic</i>']  — greedy
re.findall(r'<.+?>', html)   # ['<b>', '</b>', '<i>', '</i>']     — non-greedy
```

***

#### Subprocess and Shell Execution

```python
import subprocess

# SAFE — list form, no shell
result = subprocess.run(
    ['git', 'status'],
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    encoding='utf-8',
    check=True,           # raises CalledProcessError on non-zero exit
)
print(result.stdout)

# DANGEROUS — never interpolate user input into shell=True
user_dir = get_user_input()
# subprocess.call(f'ls {user_dir}', shell=True)  # command injection!
subprocess.call(['ls', user_dir])    # SAFE

# Timeout — always add to prevent hangs
try:
    result = subprocess.run(
        ['slow_command'],
        timeout=30,       # seconds
        capture_output=True,   # capture_output=True shorthand since 3.7
        encoding='utf-8',
    )
except subprocess.TimeoutExpired:
    handle_timeout()
```

***

#### Concurrency

##### The GIL

CPython's Global Interpreter Lock means only one thread executes Python bytecode at a time:[^11]

- `threading` is effective for **I/O-bound** work.
- `threading` does NOT provide CPU parallelism for pure Python.
- `multiprocessing` bypasses the GIL with separate processes.
- `asyncio` provides single-thread I/O concurrency.

##### `concurrent.futures` — High-Level Concurrency (Recommended)

```python
from concurrent.futures import ThreadPoolExecutor, ProcessPoolExecutor, as_completed

# I/O-bound: ThreadPoolExecutor
with ThreadPoolExecutor(max_workers=10) as executor:
    future_to_url = {executor.submit(fetch_url, url): url for url in urls}
    for future in as_completed(future_to_url):
        url = future_to_url[future]
        try:
            data = future.result()
            process(data)
        except Exception as e:
            logger.error('Failed %s: %s', url, e)

# CPU-bound: ProcessPoolExecutor
with ProcessPoolExecutor(max_workers=4) as executor:
    results = list(executor.map(cpu_task, range(10)))
```

##### `threading`

```python
import threading

lock = threading.Lock()
results: list = []

def worker(task_id: int) -> None:
    result = do_work(task_id)
    with lock:
        results.append(result)

threads = [threading.Thread(target=worker, args=(i,)) for i in range(10)]
for t in threads:
    t.start()
for t in threads:
    t.join()

# Thread-safe queue
from queue import Queue
q: Queue = Queue(maxsize=100)
```

##### `multiprocessing` with Shared Memory (new in 3.8)

Python 3.8 introduces `multiprocessing.shared_memory` — zero-copy shared memory between processes:[^12]

```python
from multiprocessing import Process
from multiprocessing.shared_memory import SharedMemory
from multiprocessing.managers import SharedMemoryManager
import numpy as np

# Using SharedMemoryManager for automatic lifecycle management
def process_chunk(shm_name: str, shape: tuple, dtype, result_queue) -> None:
    shm = SharedMemory(name=shm_name)
    arr = np.ndarray(shape, dtype=dtype, buffer=shm.buf)
    result = np.sum(arr)
    result_queue.put(result)
    shm.close()

if __name__ == '__main__':
    import multiprocessing
    result_queue = multiprocessing.Queue()
    data = np.arange(100, dtype=np.float64)

    with SharedMemoryManager() as smm:
        shm = smm.SharedMemory(size=data.nbytes)
        shared_arr = np.ndarray(data.shape, dtype=data.dtype, buffer=shm.buf)
        shared_arr[:] = data[:]

        p = Process(target=process_chunk,
                    args=(shm.name, data.shape, data.dtype, result_queue))
        p.start()
        p.join()
        print(result_queue.get())  # 4950.0
    # SharedMemory automatically cleaned up by SharedMemoryManager
```

**Critical rules for `SharedMemory`:**

- Always call `shm.close()` in every process that opens the memory.
- Call `shm.unlink()` **once** in the creating process to release the OS resource.
- Use `SharedMemoryManager` as a context manager to automate this lifecycle.
- Only raw bytes are shared (numeric arrays, byte buffers) — never Python objects (no pickling happens).

##### Standard `multiprocessing`

```python
from multiprocessing import Pool, cpu_count

def process_chunk(chunk: list) -> list:
    return [expensive_transform(item) for item in chunk]

if __name__ == '__main__':  # REQUIRED — prevents recursive spawning on Windows and macOS
    data = load_large_dataset()
    n = cpu_count()
    chunks = [data[i::n] for i in range(n)]

    with Pool(processes=n) as pool:   # Pool is a context manager
        results = pool.map(process_chunk, chunks)

    flat = [item for sublist in results for item in sublist]
```

**`multiprocessing` macOS behavior change (3.8):** macOS now uses `spawn` instead of `fork` as the default start method.[^1] Code that worked on Linux with `fork` may fail on macOS in 3.8 if it uses resources that cannot be pickled. Always use the `if __name__ == '__main__':` guard and ensure all arguments to worker functions are picklable.

***

#### New in 3.8: `math` Additions

```python
import math

# math.prod — product of an iterable (like sum() but for multiplication)
print(math.prod([1, 2, 3, 4, 5]))          # 120
print(math.prod([0.5, 0.5, 0.5], start=8)) # 1.0

# math.isqrt — exact integer square root (no float imprecision)
print(math.isqrt(16))    # 4
print(math.isqrt(17))    # 4 (floor of √17)

r = 650320427
s = r ** 2
print(math.isqrt(s - 1))       # 650320426 — correct
print(int((s - 1) ** 0.5))     # 650320427 — WRONG, float precision error

# math.dist — Euclidean distance between two points
print(math.dist((0, 0), (3, 4)))         # 5.0
print(math.dist((1, 2, 3), (4, 6, 3)))  # 5.0

# math.hypot — now supports more than 2 dimensions
print(math.hypot(3, 4))         # 5.0
print(math.hypot(1, 2, 2))      # 3.0 (√(1+4+4))

# math.perm and math.comb
print(math.perm(10, 3))  # 720  — permutations P(10,3)
print(math.comb(10, 3))  # 120  — combinations C(10,3)
```

***

#### Memory Management

```python
import sys
import gc
import tracemalloc
import weakref

x = [1, 2, 3]
print(sys.getrefcount(x))   # typically 2

gc.collect()                          # force collection of cyclic garbage
gc.set_threshold(700, 10, 10)

# tracemalloc — memory profiling
tracemalloc.start(10)
do_work()
snapshot = tracemalloc.take_snapshot()
for stat in snapshot.statistics('lineno')[:10]:
    print(stat)

# Optimization patterns
total = sum(x**2 for x in range(1_000_000))   # generator — O(1) memory

import array
int_array = array.array('i', [1, 2, 3])       # typed C array — far less memory than list

cache: weakref.WeakValueDictionary = weakref.WeakValueDictionary()
```

***

#### JSON, CSV, and Data Serialization

##### JSON

```python
import json

data = {'name': 'Alice', 'age': 30, 'scores': [95, 87, 92]}

json_str = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False)
restored = json.loads(json_str)
json.loads(b'{"key": "value"}')  # bytes accepted in 3.6+

with open('data.json', 'w', encoding='utf-8') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)

from datetime import datetime

class AppEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, datetime):
            return obj.isoformat()
        if isinstance(obj, set):
            return sorted(obj)
        return super().default(obj)

json.dumps({'ts': datetime.now()}, cls=AppEncoder)
```

##### CSV

```python
import csv

# text mode + newline='' prevents double newlines on Windows
with open('data.csv', 'r', encoding='utf-8', newline='') as f:
    reader = csv.DictReader(f)
    for row in reader:
        process(row)

fieldnames = ['name', 'age', 'city']
with open('output.csv', 'w', encoding='utf-8', newline='') as f:
    writer = csv.DictWriter(f, fieldnames=fieldnames)
    writer.writeheader()
    writer.writerow({'name': 'Alice', 'age': 30, 'city': 'Berlin'})
```

##### Pickle

```python
import pickle

with open('data.pkl', 'wb') as f:
    pickle.dump(data, f, protocol=pickle.HIGHEST_PROTOCOL)

with open('data.pkl', 'rb') as f:
    restored = pickle.load(f)
```

**`pickle` now uses Protocol 4 by default in 3.8** (up from Protocol 3).[^1] Protocol 5 (out-of-band buffers, PEP 574) was also added in 3.8 for efficient large-data transfer.

**SECURITY WARNING:** Never unpickle data from untrusted sources. Pickle can execute arbitrary code during deserialization. For inter-system communication, use JSON, protobuf, or msgpack.

***

#### Performance Patterns and Optimization

##### Profiling First

```python
import cProfile, pstats, io

# cProfile as a context manager (new in 3.8)
with cProfile.Profile() as pr:
    main()

stream = io.StringIO()
pstats.Stats(pr, stream=stream).sort_stats('cumulative').print_stats(20)
print(stream.getvalue())

import timeit
elapsed = timeit.timeit('sum(range(1000))', number=10_000)
```

**`cProfile.Profile` is now a context manager (new in 3.8).** No more manual `enable()`/`disable()` calls.[^1]

##### Common Performance Patterns

```python
import math

# Local variable alias (faster attribute lookup inside hot loops)
sqrt = math.sqrt
for i in range(100_000):
    result = sqrt(i)

# String building — O(n) with join, O(n²) with +=
result = ''.join(str(item) for item in data)

# O(1) membership testing — use frozenset for lookup sets
ALLOWED_ROLES = frozenset({'admin', 'editor', 'viewer'})
if role in ALLOWED_ROLES:   # O(1) instead of O(n) list scan
    pass

# defaultdict — avoid setdefault in hot paths
from collections import defaultdict
d: defaultdict = defaultdict(list)
d[key].append(val)

# enumerate — never track i manually
for i, item in enumerate(lst, start=1):
    print(f'{i}: {item}')

# Lazy pipelines — sum over generator avoids building intermediate list
total = sum(process(x) for x in huge_data if predicate(x))

# LOAD_GLOBAL is ~40% faster in 3.8 (per-opcode cache) — frequently accessed
# globals benefit from local aliasing less than in earlier versions, but the
# alias is still valid for inner loops with tens of millions of iterations
```

***

#### Virtual Environments and Packaging

```bash
# Create with built-in venv
python3.8 -m venv .venv

# Activate (Unix/macOS)
source .venv/bin/activate

# Activate (Windows)
.venv\Scripts\activate

# Install and freeze
pip install -r requirements.txt
pip freeze > requirements.txt
```

##### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=42", "wheel"]
build-backend = "setuptools.build_meta"

[tool.black]
line-length = 88
target-version = ["py38"]

[tool.mypy]
python_version = "3.8"
strict = true

[tool.pytest.ini_options]
testpaths = ["tests"]
```

##### Project Structure

```
project/
├── .venv/
├── src/
│   └── myapp/
│       ├── __init__.py
│       ├── main.py
│       └── utils.py
├── tests/
│   ├── __init__.py
│   ├── conftest.py
│   └── test_main.py
├── pyproject.toml
├── requirements.txt
└── README.md
```

***

#### Testing

```python
import asyncio
import unittest
from unittest.mock import Mock, patch, AsyncMock   # AsyncMock added in Python 3.8!

import pytest

from myapp.calculator import add, divide

def test_add() -> None:
    assert add(2, 3) == 5

def test_divide_by_zero_raises() -> None:
    with pytest.raises(ZeroDivisionError):
        divide(10, 0)

@pytest.mark.parametrize('a,b,expected', [
    (1, 1, 2),
    (0, 0, 0),
    (-1, 1, 0),
])
def test_add_parametrized(a: int, b: int, expected: int) -> None:
    assert add(a, b) == expected

@pytest.fixture
def db():
    database = Database(':memory:')
    database.create_tables()
    yield database
    database.close()

# AsyncMock — new in Python 3.8! No more manual async mock setup
def test_async_with_mock() -> None:
    async_mock = AsyncMock(return_value={'id': 1, 'name': 'Test'})

    async def run():
        with patch('myapp.api.fetch_user', async_mock):
            from myapp import api
            return await api.fetch_user(1)

    result = asyncio.run(run())
    async_mock.assert_awaited_once_with(1)
    assert result['name'] == 'Test'

# IsolatedAsyncioTestCase — new in 3.8 stdlib
class TestAsync(unittest.IsolatedAsyncioTestCase):

    async def asyncSetUp(self) -> None:
        self.conn = await create_connection()

    async def test_fetch(self) -> None:
        result = await self.conn.fetch('SELECT 1')
        self.assertEqual(result, [(1,)])

    async def asyncTearDown(self) -> None:
        await self.conn.close()
```

**`AsyncMock` and `IsolatedAsyncioTestCase` are both new in Python 3.8.**[^1] Prior to 3.8, testing async code required third-party packages like `pytest-asyncio` combined with manually crafted mock coroutines. In 3.8, this is covered by the stdlib.

***

#### Code Quality Tools

```bash
pip install mypy flake8 black isort bandit

# Type checking (strict mode recommended)
mypy --strict myapp/

# Formatting (Black — de-facto Python 3 standard)
black myapp/ tests/

# Import sorting (Black-compatible)
isort --profile=black myapp/ tests/

# Linting (E203 and W503 suppress Black-incompatible Flake8 rules)
flake8 myapp/ --max-line-length=88 --extend-ignore=E203,W503

# Security audit
bandit -r myapp/

# CI check pipeline (run all checks before merging)
black --check myapp/ && isort --check-only myapp/ && flake8 myapp/ && mypy myapp/
```

***

#### Production Checklist

**Python 3.8-specific items:**

- [ ] `asyncio.run()` used as the single entry point — no `loop.run_until_complete()` in new code
- [ ] `asyncio.CancelledError` always re-raised — never suppressed
- [ ] `asyncio.get_running_loop()` used inside async functions (not `get_event_loop()`)
- [ ] Named tasks created with `create_task(..., name=...)` for observability
- [ ] `@functools.lru_cache` used without parentheses only when default `maxsize=128` is intended
- [ ] `functools.cached_property` not used on classes with `__slots__`
- [ ] `functools.cached_property` avoided for slow computations hit concurrently across many instances (class-wide lock contention in 3.8)
- [ ] `TypedDict` never passed to `isinstance()` at runtime
- [ ] `Literal` types have runtime validation alongside them (enum or explicit check)
- [ ] `Final` and `@final` paired with `mypy --strict` in CI
- [ ] `Protocol` with `@runtime_checkable` understood to check method names only
- [ ] Walrus operator (`:=`) targets are not the same name as the `for` variable in comprehensions
- [ ] `Path.unlink(missing_ok=True)` used instead of `suppress(FileNotFoundError)` pattern
- [ ] `logging.basicConfig(force=True)` used when reconfiguring an already-configured logger
- [ ] `cProfile` used as context manager (not manual `enable`/`disable`)
- [ ] `math.isqrt()` used for exact integer square roots (not `int(math.sqrt(n))`)
- [ ] `multiprocessing` code tested on both Linux and macOS (macOS uses `spawn` default in 3.8)
- [ ] `SharedMemory` always cleaned up: `close()` in all processes, `unlink()` once in creator
- [ ] No 3.9+ features: no `str.removeprefix()`/`removesuffix()`, no dict `|` merge, no `match`, no parenthesized multi-`with`

**Code correctness:**

- [ ] `bytes` and `str` never mixed without explicit encode/decode
- [ ] `encoding='utf-8'` passed to all `open()` calls for text files
- [ ] `newline=''` passed to `open()` for CSV files
- [ ] No mutable default arguments in function signatures
- [ ] Mutable `dataclass` fields use `field(default_factory=...)`, not `= []`
- [ ] `__hash__` defined whenever `__eq__` is defined
- [ ] f-string `=` specifier (`{x=}`) absent from all production log/print calls
- [ ] `is` never used to compare non-None literals (heeds 3.8's `SyntaxWarning`)

**Safety and security:**

- [ ] All SQL queries use parameterized form — never f-string SQL
- [ ] `pickle.load()` never called on untrusted data
- [ ] `shell=True` never used with user-controlled input
- [ ] `eval()` and `exec()` never called on untrusted input
- [ ] `secrets.compare_digest()` used for all token comparisons
- [ ] Path traversal check (`Path.relative_to()`) in all file-accepting endpoints
- [ ] `tempfile.NamedTemporaryFile` / `TemporaryDirectory` used (not `mktemp`)
- [ ] XML parsing uses `defusedxml` for untrusted input (stdlib defaults still allow entity-expansion DoS)

**Performance:**

- [ ] Profiled before optimizing — no premature optimization
- [ ] Generator expressions in pipelines (not list comprehensions)
- [ ] Membership tests on frequently-searched collections use `set`/`frozenset`
- [ ] `str.join()` used for multi-item string building (not loop `+=`)
- [ ] Logging uses `%`-style deferred formatting (not f-strings)
- [ ] Long-running asyncio code uses `run_in_executor` for any blocking calls

**Concurrency:**

- [ ] `if __name__ == '__main__':` guard present in all multiprocessing scripts
- [ ] Shared mutable state protected with appropriate `threading.Lock`
- [ ] All `asyncio.gather()` calls have `return_exceptions=True` where partial failure is acceptable
- [ ] Every external I/O call wrapped in `asyncio.wait_for(..., timeout=...)`
- [ ] `SharedMemory` used via `SharedMemoryManager` context manager

**Observability:**

- [ ] `logging.getLogger(__name__)` used in every module
- [ ] No `print()` in production code paths
- [ ] All custom exceptions inherit from `Exception` (not `BaseException`)
- [ ] `logger.exception()` used inside `except` blocks to capture tracebacks
- [ ] Bare `raise` (not `raise e`) to preserve original tracebacks

***

[^1]: Python 3.8 What's New. Python Software Foundation. https://docs.python.org/3/whatsnew/3.8.html
[^2]: Python 3.7 language guarantee: dict insertion order. https://docs.python.org/3.7/whatsnew/3.7.html
[^3]: Python 3 Built-in Types. https://docs.python.org/3.8/library/stdtypes.html
[^4]: PEP 572 – Assignment Expressions. https://peps.python.org/pep-0572/
[^5]: PEP 570 – Python Positional-Only Parameters. https://peps.python.org/pep-0570/
[^6]: PEP 544 (Protocol), PEP 586 (Literal), PEP 589 (TypedDict), PEP 591 (Final). https://peps.python.org/
[^7]: PEP 557 – Data Classes. https://peps.python.org/pep-0557/
[^8]: Python pathlib module. https://docs.python.org/3.8/library/pathlib.html
[^9]: Python io module. https://docs.python.org/3.8/library/io.html
[^10]: Python secrets module. https://docs.python.org/3.8/library/secrets.html
[^11]: Python GIL. https://docs.python.org/3/glossary.html#term-global-interpreter-lock
[^12]: multiprocessing.shared_memory. https://docs.python.org/3.8/library/multiprocessing.shared_memory.html
[^13]: functools.cached_property. https://docs.python.org/3.8/library/functools.html#functools.cached_property (locking behavior: bpo-43468; removed in 3.12)
[^14]: importlib.metadata (provisional in 3.8). https://docs.python.org/3.8/library/importlib.metadata.html
[^15]: XML vulnerabilities and bpo-17239 (external entities disabled by default in xml.sax/xml.dom since 3.7.1/3.6.8). https://docs.python.org/3/library/xml.html#xml-vulnerabilities
