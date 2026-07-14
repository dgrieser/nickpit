### Python 3.6: The Complete Production Developer Guide

#### Overview

Python 3.6 was released on December 23, 2016.[^1] It is a dynamically typed, interpreted, general-purpose language with a clean, whitespace-significant syntax. Python 3.6 delivered fourteen PEPs — one of the most significant feature releases since the Python 2→3 transition — and is a fully production-ready, long-supported release that underpins enormous amounts of real-world infrastructure.

This guide is the reference for projects on any Python 3.x release up to and including 3.6. Features that first appeared in 3.6 (f-strings, variable annotations, `secrets`, async generators, ...) are marked as such — skip them when targeting an older 3.x, and do not use anything newer than 3.6 (no `dataclasses` stdlib module, no `asyncio.run()`, no ordered-dict language guarantee).

A few global truths about Python 3.6 before diving in:

- **Everything is an object.** Integers, functions, classes, modules — all are objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing.** Variables carry no declared type; the type lives in the value.
- **Reference counting + cyclic GC.** CPython manages memory with reference counts plus a cyclic garbage collector.
- **`str` is Unicode everywhere.** All string literals are Unicode text; `bytes` is the separate binary type.

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

**Note:** In Python 3, all classes implicitly inherit from `object`. You do not need to write `class Foo(object):`; `class Foo:` is equivalent and preferred.[^2]

##### Indentation and Line Length

```python
# 4 spaces per indentation level — never tabs, never mixed
def function():
    if condition:
        do_something()

# PEP 8 recommends 79 characters; Black (the dominant formatter for Py3)
# uses 88 characters. Pick one and enforce it project-wide.
result = some_function(
    argument_one,
    argument_two,
    argument_three,
)

# Implicit line continuation inside brackets — preferred over backslashes
users = [
    'alice',
    'bob',
    'charlie',
]
```

##### Import Ordering

Group imports in three blocks separated by blank lines: standard library, third-party, local application — alphabetized within each group. `isort` automates this.

```python
# Standard library
import os
import sys
from pathlib import Path

# Third-party
import requests

# Local application
from myapp.models import User
from myapp.utils import format_date

# Avoid wildcard imports
# Bad:  from module import *
# Good: from module import specific_item
```

***

#### Primitive Types and Numerics

##### Integers

Python 3.6 has a single integer type: `int`. It is **arbitrary precision** — there is no separate `long`.[^3] Arithmetic never overflows; Python silently allocates more memory.

```python
import sys
print(sys.maxsize)        # 9223372036854775807 on 64-bit (not the limit — just the C ssize_t max)
x = 10 ** 100             # perfectly fine — a googol
print(type(x))            # <class 'int'>
```

**Numeric literals with underscores (PEP 515 — new in 3.6):**[^1]

```python
one_million   = 1_000_000
hex_mask      = 0xFF_FF_FF_FF
binary_val    = 0b_1001_0111_1010
pi_approx     = 3.141_592_653_589_793
sci_notation  = 1.5e1_0  # 15000000000.0
```

Underscores are purely cosmetic — the interpreter strips them. Single underscores between digits and after any base specifier are allowed; leading, trailing, or consecutive underscores are syntax errors.[^1]

**Bitwise operations:**

```python
a = 0b1010   # binary literal
b = 0xFF     # hex literal
c = 0o17     # octal literal

print(a & b)   # AND
print(a | b)   # OR
print(a ^ b)   # XOR
print(~a)      # NOT (bitwise complement)
print(a << 2)  # left shift
print(a >> 1)  # right shift
```

##### Floats

Floats are C `double` (64-bit IEEE 754).[^3]

```python
x = 3.14
y = 1.5e-10
z = float('inf')     # positive infinity
n = float('nan')     # Not a Number
print(x.is_integer())          # False
print((2.0).is_integer())      # True
print(x.as_integer_ratio())    # (7070651414971679, 2251799813685248)
```

**Pitfall: Float comparison.** Never use `==` to compare floats. Use `abs(a - b) < epsilon` or `math.isclose()`:

```python
import math

# BAD
assert 0.1 + 0.2 == 0.3  # fails

# GOOD — tolerance-based
EPSILON = 1e-9
assert abs((0.1 + 0.2) - 0.3) < EPSILON

# BETTER — math.isclose (added in Python 3.5)
assert math.isclose(0.1 + 0.2, 0.3)
# rel_tol: relative tolerance (default 1e-9); abs_tol: absolute tolerance (default 0)
assert math.isclose(a, b, rel_tol=1e-9, abs_tol=1e-12)
```

For exact decimal arithmetic, use the `decimal` module:

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

**Falsy values** in Python 3.6:[^3]
- `None`
- `False`
- Zero of any numeric type: `0`, `0.0`, `0j`
- Empty sequences: `''`, `b''`, `()`, `[]`
- Empty mapping: `{}`
- Empty set: `set()`
- Instances of classes whose `__bool__` returns `False` or `__len__` returns `0`

Everything else is truthy.

**Note:** In Python 3, the boolean method is `__bool__`, not `__nonzero__` (Python 2's name).

##### Complex Numbers

```python
z = 3 + 4j
print(z.real)       # 3.0
print(z.imag)       # 4.0
print(abs(z))       # 5.0 — magnitude
print(z.conjugate())  # (3-4j)
```

***

#### Division

In Python 3, `/` **always performs true (float) division** — this is one of the most important changes from Python 2.[^3]

```python
print(7 / 2)    # 3.5  — true division (no truncation!)
print(7 // 2)   # 3    — floor division (explicit)
print(7 % 2)    # 1    — modulo
print(7 / 2.0)  # 3.5
```

`from __future__ import division` is NOT needed in Python 3 and should not be used. If you see it in a file, remove it — it's a no-op holdover from Python 2.

**Pitfall: floor division with negative numbers** uses true floor (rounds toward negative infinity), not truncation:

```python
print(-7 // 2)   # -4, not -3
print(-7 % 2)    # 1   (satisfies: (a // b) * b + a % b == a)
```

***

#### Strings: `str` and `bytes`

This is the most fundamental difference from Python 2. Python 3.6 has two distinct string-like types:[^4]

- **`str`**: A sequence of Unicode code points. The "native" string type. **All string literals are `str`** — there is no `u''` / `unicode` split. (`u'...'` prefix is allowed again as of 3.3 for source compatibility but is redundant.)
- **`bytes`**: A sequence of raw 8-bit values. Declared with `b''` prefix. Not text — never implicitly mixed with `str`.

```python
text = 'hello'         # str — Unicode text
data = b'hello'        # bytes — raw binary
# text + data          # TypeError — no implicit mixing!
```

**The Unicode sandwich rule for Python 3:**

1. Decode `bytes` to `str` at input boundaries (file reads in binary mode, network sockets, database BLOBs).
2. Work with `str` internally throughout your application.
3. Encode `str` to `bytes` at output boundaries (writing binary files, network sockets, HTTP responses).

```python
def ensure_str(data: object, encoding: str = 'utf-8') -> str:
    """Decode bytes to str; pass str through unchanged."""
    if isinstance(data, bytes):
        return data.decode(encoding)
    return str(data)

def ensure_bytes(data: object, encoding: str = 'utf-8') -> bytes:
    """Encode str to bytes; pass bytes through unchanged."""
    if isinstance(data, str):
        return data.encode(encoding)
    return bytes(data)
```

##### Encoding and Decoding

```python
# DECODE: bytes -> str
raw = b'\xc3\xa9'        # UTF-8 for 'é'
text = raw.decode('utf-8')
print(text)              # 'é'

# ENCODE: str -> bytes
text = 'café'
encoded = text.encode('utf-8')
print(encoded)           # b'caf\xc3\xa9'

# Default encoding for encode()/decode() is utf-8
text.encode()            # equivalent to text.encode('utf-8')
```

**Pitfall: `str` and `bytes` are never equal**, even empty:[^4]

```python
print('' == b'')   # False — always False in Python 3
print(b'' == b'')  # True
```

**Pitfall: `open()` defaults to the platform locale encoding.** On Windows this is often `cp1252`, not UTF-8. Always pass `encoding='utf-8'` explicitly for text files:

```python
# UNSAFE on Windows
with open('data.txt', 'r') as f:  # uses locale encoding
    text = f.read()

# SAFE everywhere
with open('data.txt', 'r', encoding='utf-8') as f:
    text = f.read()
```

##### String Methods

All of the following work on `str`:[^3]

```python
s = 'Hello, World!'

print(s.upper())                  # 'HELLO, WORLD!'
print(s.lower())                  # 'hello, world!'
print(s.casefold())               # 'hello, world!' — aggressive lowercase for comparisons
print(s.strip())                  # strips whitespace
print(s.lstrip('H'))              # 'ello, World!'
print(s.rstrip('!'))              # 'Hello, World'
print(s.replace('World', 'Python'))  # 'Hello, Python!'
print(s.startswith('Hello'))      # True
print(s.endswith('!'))            # True
print(s.split(', '))              # ['Hello', 'World!']
print(', '.join(['a', 'b', 'c'])) # 'a, b, c'
print(s.find('World'))            # 7
print(s.index('World'))           # 7 (raises ValueError if absent)
print(s.count('l'))               # 3
print(s.center(20, '-'))          # '---Hello, World!----'
print(s.zfill(20))                # '0000000Hello, World!'
print('abc'.partition('b'))       # ('a', 'b', 'c')
print(s.splitlines())             # ['Hello, World!']
print('hello'.capitalize())       # 'Hello'
print('hello world'.title())      # 'Hello World'
print('abc'.isalpha())            # True
print('123'.isdigit())            # True
print('abc123'.isalnum())         # True
print('  '.isspace())             # True
```

**`casefold()` vs `lower()`:** `casefold()` is more aggressive — it handles German ß → ss and other Unicode folding rules. Prefer `casefold()` for case-insensitive comparisons of user-supplied text.[^3]

##### f-strings — Formatted String Literals (PEP 498, new in 3.6)

f-strings are one of the biggest Python 3.6 additions. They are prefixed with `f` or `F` and allow embedding expressions directly inside string literals.[^1]

```python
name = 'Alice'
age = 30

# Basic interpolation
greeting = f'Hello, {name}!'         # 'Hello, Alice!'

# Expressions are evaluated at runtime
result = f'2 + 2 = {2 + 2}'          # '2 + 2 = 4'

# Format spec (same spec language as str.format())
pi = 3.141592653589793
print(f'{pi:.4f}')         # '3.1416'
print(f'{pi:10.4f}')       # '    3.1416'
print(f'{1000000:_}')      # '1_000_000'  (underscore separator, Python 3.6+)
print(f'{"left":<10}|')    # 'left      |'
print(f'{"right":>10}|')   # '     right|'
print(f'{"ctr":^10}|')     # '   ctr    |'
print(f'{42:05d}')         # '00042'
print(f'{255:#x}')         # '0xff'

# Nested braces for computed format spec
width, precision = 10, 4
value = 12.34567
print(f'{value:{width}.{precision}f}')  # '   12.3457'

# repr() with !r, str() with !s, ascii() with !a
print(f'Debug: {name!r}')    # Debug: 'Alice'
print(f'ASCII: {"café"!a}')  # ASCII: 'caf\xe9'
```

**Pitfall: f-strings cannot contain backslashes inside `{}`:**

```python
# SyntaxError in Python 3.6
# f'Value: {d["key"]}'  — nested same quotes conflict
# f'Path: {path\n}'     — backslash in expression

# Workarounds:
key = 'name'
print(f'Value: {d[key]}')          # use a variable
print(f'Newline: {chr(10)}')       # chr() for special chars
```

**Pitfall: f-strings are not templates.** They are evaluated immediately at the line they appear. They cannot be stored as a string and evaluated later (unlike `str.format_map()`).

**Performance:** f-strings are faster than `%` formatting and `str.format()` because they are compiled directly to bytecode string-building instructions.[^5]

```python
# Slowest — two lookups, call overhead
result = 'Hello, %s!' % name

# Slower — method call + format parsing
result = 'Hello, {}!'.format(name)

# Fastest — compiled at parse time
result = f'Hello, {name}!'
```

##### `bytes` Methods

The `bytes` type has a subset of the methods available on `str`:

```python
b = b'hello world'
print(b.upper())                # b'HELLO WORLD'
print(b.split(b' '))            # [b'hello', b'world']
print(b.startswith(b'hello'))   # True
print(b.replace(b'o', b'0'))    # b'hell0 w0rld'
print(b.hex())                  # '68656c6c6f20776f726c64'
print(b[0])                  # 104 (integer, not b'h')

# bytes.fromhex() — inverse of hex()
print(bytes.fromhex('68656c6c6f'))  # b'hello'
```

**Pitfall: Indexing `bytes` returns an integer, not a single-byte `bytes` object:**

```python
b = b'hello'
print(b[0])      # 104 — integer
print(b[0:1])    # b'h' — bytes slice
```

***

#### Variable Annotations (PEP 526, new in 3.6)

Python 3.6 adds syntax for annotating the types of variables — including module-level variables, class variables, and instance variables.[^1]

```python
from typing import Dict, List, Optional

# Module-level annotation with value
count: int = 0
name: str = 'Alice'

# Annotation without assignment (reserves the annotation but does not define the name)
captain: str  # name is NOT defined; accessing it raises NameError

# Class variables and instance variable annotations
class Starship:
    stats: Dict[str, int] = {}   # class variable

    def __init__(self, name: str, crew: int) -> None:
        self.name = name           # type inferred from __init__ signature
        self.crew: int = crew      # explicit instance variable annotation

# Annotations are stored in __annotations__
print(Starship.__annotations__)
# {'stats': typing.Dict[str, int]}

# The interpreter does NOT enforce annotations at runtime —
# they are pure metadata for type checkers and IDEs.
x: int = 'not an int'  # No error at runtime!
```

**Best practice:** Use annotations throughout all new production code. Run `mypy` or `pyright` in CI to enforce them statically.

***

#### Type Hints (PEP 484 + PEP 526)

##### Basic Type Hints

```python
from typing import Any, Dict, FrozenSet, List, Optional, Set, Tuple, Union

# Function annotations
def greet(name: str) -> str:
    return f'Hello, {name}!'

def find_user(user_id: int) -> Optional['User']:
    """Returns User or None if not found."""
    pass

def process_items(items: List[str]) -> Dict[str, int]:
    return {item: len(item) for item in items}

# Multiple return values (use Tuple)
def divmod_custom(a: int, b: int) -> Tuple[int, int]:
    return a // b, a % b

# Union — accepts multiple types
def parse_id(raw: Union[str, int]) -> int:
    return int(raw)
```

##### TypeVar for Generics

```python
from typing import TypeVar, List, Optional

T = TypeVar('T')

def first(items: List[T]) -> Optional[T]:
    return items if items else None
```

##### `typing.NamedTuple` — Class-Based Syntax (Python 3.6+)

Python 3.6 introduces a class-based syntax for `NamedTuple` with full type annotation support:[^6]

```python
from typing import NamedTuple

class Point(NamedTuple):
    x: float
    y: float
    z: float = 0.0   # default values supported in 3.6.1+

    def distance_to(self, other: 'Point') -> float:
        return ((self.x - other.x)**2 +
                (self.y - other.y)**2 +
                (self.z - other.z)**2) ** 0.5

p1 = Point(1.0, 2.0)
p2 = Point(4.0, 6.0)
print(p1.distance_to(p2))  # 5.0
print(p1._asdict())         # OrderedDict([('x', 1.0), ('y', 2.0), ('z', 0.0)])
print(p1._replace(x=5.0))  # Point(x=5.0, y=2.0, z=0.0)
```

##### Dataclasses (backport for 3.6)

`dataclasses` is a Python 3.7 stdlib module. On 3.6, install the backport (`pip install dataclasses`):

```python
from dataclasses import dataclass, field
from typing import List

@dataclass
class User:
    id: int
    name: str
    email: str
    active: bool = True
    tags: List[str] = field(default_factory=list)  # mutable default — NEVER use = []

    def display_name(self) -> str:
        return f'{self.name} <{self.email}>'

@dataclass(frozen=True)  # immutable, hashable
class Coordinate:
    lat: float
    lon: float

u = User(id=1, name='Alice', email='alice@example.com')
print(u)  # User(id=1, name='Alice', email='alice@example.com', active=True, tags=[])
```

**Pitfall: Never use a mutable default value directly.** Using `tags: List[str] = []` raises `ValueError`. Always use `field(default_factory=list)`.

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
lst.remove('two')       # removes first occurrence; raises ValueError if absent
popped = lst.pop()      # removes and returns last; pop(i) for arbitrary index
del lst[0]              # delete by index

# Search
print(3.0 in lst)       # True
print(lst.index(3.0))   # position; raises ValueError if absent
print(lst.count(6))     # count occurrences

# Sorting
nums = [3, 1, 4, 1, 5, 9]
nums.sort()                      # in-place, stable
nums.sort(key=lambda x: -x)     # descending
sorted_copy = sorted(nums)       # returns new list
nums.reverse()                   # in-place

# Copying
import copy
lst2 = lst[:]               # shallow copy
lst3 = list(lst)            # also shallow copy
lst4 = copy.deepcopy(lst)   # deep copy
```

**Pitfall: List multiplication shares references:**

```python
# WRONG — all three sublists are the SAME object
matrix = [[0] * 3] * 3
matrix[0][0] = 99
print(matrix)  # [[99, 0, 0], [99, 0, 0], [99, 0, 0]]

# CORRECT — each sublist is distinct
matrix = [[0] * 3 for _ in range(3)]
matrix[0][0] = 99
print(matrix)  # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

**Pitfall: Mutating a list while iterating it** skips elements:

```python
# WRONG
for item in lst:
    if should_remove(item):
        lst.remove(item)

# CORRECT — iterate a copy
for item in lst[:]:
    if should_remove(item):
        lst.remove(item)

# MOST IDIOMATIC — comprehension filter
lst = [item for item in lst if not should_remove(item)]
```

##### List Comprehensions

```python
squares = [x**2 for x in range(10)]
evens   = [x for x in range(20) if x % 2 == 0]
flat    = [item for sublist in nested for item in sublist]
matrix  = [[i * j for j in range(1, 4)] for i in range(1, 4)]
```

**Python 3 fix:** Unlike Python 2, list comprehension variables do NOT leak into the enclosing scope.[^7]

```python
x = 10
result = [x for x in range(5)]
print(x)  # 10 — x is unchanged (Python 3 correctly scopes comprehension variables)
```

***

#### Tuples

Tuples are immutable, ordered sequences.[^3]

```python
t = (1, 2, 3)
single = (42,)   # trailing comma required for single-element tuples
empty  = ()

# Unpacking
a, b, c = t
a, b = b, a      # swap without temp variable

# Extended unpacking (Python 3 only)
first, *rest = [1, 2, 3, 4, 5]
print(first)  # 1
print(rest)   # [2, 3, 4, 5]

head, *middle, tail = [1, 2, 3, 4, 5]
print(head, middle, tail)  # 1 [2, 3, 4] 5
```

**Extended unpacking (`*`) is Python 3 only.** It provides idiomatic head/tail splitting without `lst[0]` / `lst[1:]`.

***

#### Dictionaries

Dictionaries in Python 3.6 are ordered by insertion order as a CPython implementation detail (the language guarantee came with 3.7).[^8] In practice, CPython 3.6 dicts are ordered; use `collections.OrderedDict` if cross-implementation portability is needed.

```python
d = {'name': 'Alice', 'age': 30}

# Access
print(d['name'])                   # 'Alice'; raises KeyError if absent
print(d.get('age'))                # 30; returns None if absent
print(d.get('height', 0))         # 0 — default value
print(d.setdefault('role', 'user'))  # sets and returns default if absent

# Mutation
d['email'] = 'alice@example.com'
del d['age']
d.update({'city': 'Berlin', 'country': 'DE'})

# Iteration — Python 3: all dict methods return VIEWS, not lists
for key in d:                      # iterates keys
    pass
for k, v in d.items():             # view of (key, value) pairs
    print(k, v)
for k in d.keys():                 # view of keys
    pass
for v in d.values():               # view of values
    pass
```

**Python 3 dict views vs Python 2 iter methods:** In Python 3, `d.items()`, `d.keys()`, and `d.values()` return lightweight **view objects** — they don't copy the data. There is no equivalent of Python 2's `d.iteritems()` because the default methods ARE already lazy views.[^3]

**Pitfall:** Adding or removing keys while iterating a dictionary raises `RuntimeError` (the check is size-based, so the docs only promise it *may* be detected — never rely on it):

```python
# WRONG
for k in d:
    if should_delete(k):
        del d[k]   # RuntimeError: dictionary changed size during iteration

# CORRECT
keys_to_delete = [k for k in d if should_delete(k)]
for k in keys_to_delete:
    del d[k]

# ALSO CORRECT — build new dict
d = {k: v for k, v in d.items() if not should_delete(k)}
```

##### Dict Comprehensions and Merging

```python
squares  = {x: x**2 for x in range(10)}
inverted = {v: k for k, v in d.items()}
filtered = {k: v for k, v in d.items() if v is not None}

# Merge into a new dict (Python 3.5+ **-unpacking)
merged = {**dict1, **dict2}  # dict2 values win on key collision
config = {**defaults, **overrides}
```

##### `collections.defaultdict` and `Counter`

```python
from collections import defaultdict, Counter

word_count = defaultdict(int)
for word in text.split():
    word_count[word] += 1

graph = defaultdict(list)
graph['A'].append('B')

c = Counter('abracadabra')
print(c.most_common(3))   # [('a', 5), ('b', 2), ('r', 2)]
c.update('hello')
```

***

#### Sets

Sets are mutable, unordered collections of unique, hashable elements.[^3]

```python
s = {1, 2, 3}
s2 = set([1, 2, 2, 3])    # duplicate removed
empty_set = set()          # NOT {} — that's an empty dict!

s.add(4)
s.discard(10)    # no error if absent
s.remove(4)      # raises KeyError if absent

a = {1, 2, 3, 4}
b = {3, 4, 5, 6}
print(a | b)     # union: {1, 2, 3, 4, 5, 6}
print(a & b)     # intersection: {3, 4}
print(a - b)     # difference: {1, 2}
print(a ^ b)     # symmetric difference: {1, 2, 5, 6}
print(a.isdisjoint(b))   # False (they share 3, 4)

fs = frozenset([1, 2, 3])   # immutable and hashable

# Set comprehensions
unique_squares = {x**2 for x in range(-5, 6)}
```

***

#### `range` in Python 3

In Python 3, `range()` is always a lazy object — there is no `xrange()` at all.[^3]

```python
for i in range(1_000_000):
    process(i)

# range objects support len(), indexing, and slicing
r = range(10)
print(len(r))       # 10
print(r[3])         # 3
print(4 in r)       # True — O(1) membership test (not linear scan!)
print(r[::-1])      # range(9, -1, -1)

# Use list(range(10)) when you need an actual list
indices = list(range(10))
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

label = 'pass' if score >= 60 else 'fail'  # ternary expression
```

##### `for` Loops

```python
for i, item in enumerate(my_list):
    print(i, item)

for key, val in d.items():
    process(key, val)

for a, b in zip(list1, list2):   # stops at shortest
    print(a, b)

from itertools import zip_longest
for a, b in zip_longest(list1, list2, fillvalue=None):
    print(a, b)

# for / else — else runs only if loop was NOT terminated by break
for item in collection:
    if matches(item):
        result = item
        break
else:
    result = default_value
```

***

#### Functions

##### Defining Functions

```python
def greet(name: str, greeting: str = 'Hello', punctuation: str = '!') -> str:
    """Return a greeting string.

    Args:
        name: The person to greet.
        greeting: The greeting word (default 'Hello').
        punctuation: Trailing punctuation (default '!').

    Returns:
        A formatted greeting string.
    """
    return f'{greeting}, {name}{punctuation}'
```

##### `*args`, `**kwargs`, and Keyword-Only Arguments

```python
def variadic(*args: object, **kwargs: object) -> None:
    for i, arg in enumerate(args):
        print(f'arg[{i}] = {arg!r}')
    for key, val in sorted(kwargs.items()):
        print(f'{key} = {val!r}')

# Keyword-only arguments (after *)
def create_user(
    name: str,
    email: str,
    *,                # everything after this is keyword-only
    active: bool = True,
    role: str = 'user',
) -> dict:
    return {'name': name, 'email': email, 'active': active, 'role': role}

create_user('Alice', 'alice@example.com', active=True)   # OK
# create_user('Alice', 'alice@example.com', True)         # TypeError!
```

**Recommendation:** Use keyword-only arguments for any optional parameter with a non-obvious meaning, especially boolean flags. This prevents accidental positional misuse.

##### `nonlocal` Keyword

`nonlocal` allows a nested function to rebind a variable in the enclosing scope (Python 3 only):[^3]

```python
from typing import Callable

def make_counter(start: int = 0) -> Callable[[], int]:
    count = start

    def counter() -> int:
        nonlocal count   # declare intent to rebind the outer 'count'
        count += 1
        return count

    return counter

c = make_counter()
print(c())  # 1
print(c())  # 2
```

##### Closures and Late Binding

**Pitfall: Late binding in closures** — closures capture the variable, not its value:

```python
# WRONG — all functions see the same 'i', which ends at 4
funcs = [lambda: i for i in range(5)]
print([f() for f in funcs])  # [4, 4, 4, 4, 4]

# CORRECT — use default argument to capture the current value
funcs = [lambda i=i: i for i in range(5)]
print([f() for f in funcs])  # [0, 1, 2, 3, 4]
```

##### `map`, `filter`, `zip`, `reduce`

In Python 3, `map()`, `filter()`, and `zip()` all return lazy iterators — no list copy.[^3] `reduce` moved to `functools`:

```python
from functools import reduce

# All lazy in Python 3
squared = list(map(lambda x: x**2, range(10)))
evens   = list(filter(lambda x: x % 2 == 0, range(10)))

total = reduce(lambda a, b: a + b, range(1, 6), 0)  # 15

# Prefer comprehensions for readability:
squared = [x**2 for x in range(10)]
evens   = [x for x in range(10) if x % 2 == 0]
```

***

#### Generators and Iterators

##### The Iterator Protocol

An iterator in Python 3 uses `__next__()` (NOT `next()` as in Python 2):[^3]

```python
class CountDown:
    def __init__(self, n: int) -> None:
        self.n = n

    def __iter__(self) -> 'CountDown':
        return self

    def __next__(self) -> int:   # Python 3 spelling
        if self.n <= 0:
            raise StopIteration
        val = self.n
        self.n -= 1
        return val

for x in CountDown(3):
    print(x)  # 3, 2, 1
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
total = sum(x**2 for x in range(1_000_000))   # never builds list
```

##### Asynchronous Generators (PEP 525, new in 3.6)

Python 3.6 lifts the restriction that prevented using `yield` inside `async def` functions:[^1]

```python
import asyncio
from typing import AsyncGenerator   # typing.AsyncGenerator requires 3.6.1+

async def ticker(delay: float, count: int) -> AsyncGenerator[int, None]:
    """Yield numbers 0..count-1, each delayed by 'delay' seconds."""
    for i in range(count):
        yield i
        await asyncio.sleep(delay)

async def main() -> None:
    async for value in ticker(0.1, 5):
        print(value)

loop = asyncio.get_event_loop()
loop.run_until_complete(main())
loop.close()
```

##### `send()` and Two-Way Generators

```python
def accumulator() -> Generator[int, int, None]:
    total = 0
    while True:
        value = yield total
        if value is None:
            break
        total += value

acc = accumulator()
next(acc)           # advance to first yield
acc.send(10)        # total = 10
acc.send(20)        # total = 30
print(acc.send(5))  # 35
```

***

#### `itertools` — Efficient Iteration

```python
import itertools

# Infinite iterators
itertools.count(10)           # 10, 11, 12, ...
itertools.cycle('ABC')        # A, B, C, A, B, C, ...
itertools.repeat(42, 3)       # 42, 42, 42

# Finite iterators
list(itertools.chain([1, 2], [3, 4], [5]))   # [1, 2, 3, 4, 5]
list(itertools.islice(itertools.count(), 5))  # [0, 1, 2, 3, 4]
list(itertools.compress('ABCDEF', [1, 0, 1, 0, 1, 0]))  # ['A', 'C', 'E']
list(itertools.dropwhile(lambda x: x < 5, [1, 4, 6, 4, 1]))  # [6, 4, 1]
list(itertools.takewhile(lambda x: x < 5, [1, 4, 6, 4, 1]))  # [1, 4]
list(itertools.filterfalse(lambda x: x % 2, range(10)))  # [0, 2, 4, 6, 8]

# Accumulate (running total / prefix fold)
import operator
list(itertools.accumulate([1, 2, 3, 4], operator.mul))  # [1, 2, 6, 24]

# Combinatorics
list(itertools.product('AB', repeat=2))    # AA, AB, BA, BB
list(itertools.permutations('ABC', 2))     # 6 items
list(itertools.combinations('ABC', 2))    # AB, AC, BC

# Grouping (input must be sorted on the key!)
data = sorted([('A', 1), ('B', 2), ('A', 3)], key=lambda x: x[0])
for key, group in itertools.groupby(data, key=lambda x: x[0]):
    print(key, list(group))
```

**Python 3 note:** `itertools.izip`, `itertools.imap`, `itertools.ifilter` do NOT exist in Python 3. The built-in `zip`, `map`, `filter` are already lazy.

***

#### Classes and Object-Oriented Programming

##### Class Definition

In Python 3, all classes implicitly inherit from `object`. `class Foo:` and `class Foo(object):` are identical.[^2]

```python
class Animal:
    kingdom = 'Animalia'   # class attribute

    def __init__(self, name: str, species: str) -> None:
        self.name = name
        self.species = species
        self._energy = 100
        self.__secret = 'x'   # name-mangled to _Animal__secret

    def eat(self, food: str) -> str:
        self._energy += 10
        return f'{self.name} eats {food}'

    def __repr__(self) -> str:
        return f'Animal(name={self.name!r}, species={self.species!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Animal):
            return NotImplemented
        return self.name == other.name and self.species == other.species

    def __hash__(self) -> int:
        # Python 3: defining __eq__ sets __hash__ = None unless you define it too
        return hash((self.name, self.species))
```

##### Inheritance and `super()`

Python 3 supports zero-argument `super()` — no longer need to pass `ClassName, self`:[^3]

```python
class Dog(Animal):
    def __init__(self, name: str, breed: str) -> None:
        super().__init__(name, 'Canis lupus familiaris')  # zero-arg super()
        self.breed = breed

    def speak(self) -> str:
        return f'{self.name} says: Woof!'
```

##### `__init_subclass__` (PEP 487, new in 3.6)

A cleaner alternative to metaclasses for customizing subclass creation:[^1]

```python
class PluginBase:
    subclasses: list = []

    def __init_subclass__(cls, required_field: str = '', **kwargs):
        super().__init_subclass__(**kwargs)
        cls.subclasses.append(cls)

class Plugin1(PluginBase):
    pass

class Plugin2(PluginBase):
    pass

print(PluginBase.subclasses)  # [<class 'Plugin1'>, <class 'Plugin2'>]
```

##### `__set_name__` Descriptor Enhancement (PEP 487, new in 3.6)

Descriptors now receive their attribute name at class creation time:[^1]

```python
class Validated:
    def __set_name__(self, owner, name):
        self.name = name
        self.private_name = f'_{name}'

    def __get__(self, obj, objtype=None):
        if obj is None:
            return self
        return getattr(obj, self.private_name, None)

    def __set__(self, obj, value):
        if not isinstance(value, int):
            raise TypeError(f'{self.name!r} must be an int')
        if value < 0:
            raise ValueError(f'{self.name!r} must be non-negative')
        setattr(obj, self.private_name, value)

class Product:
    price = Validated()     # self.name = 'price' set automatically
    quantity = Validated()  # self.name = 'quantity' set automatically

    def __init__(self, price: int, quantity: int) -> None:
        self.price = price
        self.quantity = quantity
```

##### Properties, `classmethod`, `staticmethod`

```python
import math

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

import json

class Config:
    def __init__(self, data: dict) -> None:
        self.data = data

    @classmethod
    def from_file(cls, path: str) -> 'Config':
        with open(path, encoding='utf-8') as f:
            return cls(json.load(f))

    @staticmethod
    def validate_key(key: str) -> bool:
        return isinstance(key, str) and len(key) > 0
```

##### `__slots__`

`__slots__` eliminates the per-instance `__dict__`, reducing memory significantly for large numbers of small objects:[^9]

```python
class Point:
    __slots__ = ('x', 'y')

    def __init__(self, x: float, y: float) -> None:
        self.x = x
        self.y = y

# Attempting undeclared attributes raises AttributeError
# p.z = 3.0  # AttributeError
```

**Caveats:** `__slots__` classes cannot be weakly referenced unless `'__weakref__'` is listed. Subclasses must declare `__slots__ = ()` to preserve the benefit.

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
| `__next__(self)` | Next value (**Python 3 spelling — NOT `next`**) |
| `__eq__(self, other)` | Equality `==` |
| `__lt__`, `__le__`, `__gt__`, `__ge__` | Rich comparisons |
| `__hash__(self)` | Hash for sets/dict keys |
| `__bool__(self)` | Bool cast (**Python 3 — NOT `__nonzero__`**) |
| `__call__(self, ...)` | Make instance callable |
| `__enter__`, `__exit__` | Context manager protocol |
| `__init_subclass__` | Subclass creation hook (3.6+) |
| `__set_name__` | Descriptor attribute name (3.6+) |

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

**Python 3 ABC style:** Inherit from `ABC` directly (cleaner than `metaclass=ABCMeta`).

***

#### Enum (Python 3.4+)

Enums provide named constants with type safety. `enum.auto()` was enhanced in Python 3.6:[^10]

```python
from enum import Enum, IntEnum, Flag, auto

class Color(Enum):
    RED = 1
    GREEN = 2
    BLUE = 3

print(Color.RED)           # Color.RED
print(Color.RED.name)      # 'RED'
print(Color.RED.value)     # 1

# auto() — automatic value assignment (3.6)
class Direction(Enum):
    NORTH = auto()   # 1
    SOUTH = auto()   # 2
    EAST  = auto()   # 3
    WEST  = auto()   # 4

# IntEnum — also an int, comparable with ints
class HttpStatus(IntEnum):
    OK          = 200
    NOT_FOUND   = 404
    SERVER_ERROR = 500

print(HttpStatus.OK == 200)   # True

# Flag — bitwise composition (3.6)
class Permission(Flag):
    READ    = auto()    # 1
    WRITE   = auto()    # 2
    EXECUTE = auto()    # 4
    ALL     = READ | WRITE | EXECUTE

user_perms = Permission.READ | Permission.WRITE
print(Permission.READ in user_perms)    # True
print(Permission.EXECUTE in user_perms) # False
```

**Pitfall: Use `is` for enum identity comparison, not `==`** (though `==` works for `Enum`, `is` is more explicit). Never compare `Enum` members to raw ints unless using `IntEnum` intentionally.

***

#### Decorators

```python
import functools
import time
from typing import Callable, Tuple, Type

def timer(func):
    @functools.wraps(func)   # preserves __name__, __doc__, etc. — MANDATORY
    def wrapper(*args, **kwargs):
        start = time.perf_counter()
        result = func(*args, **kwargs)
        elapsed = time.perf_counter() - start
        print(f'{func.__name__} took {elapsed:.4f}s')
        return result
    return wrapper

@timer
def slow_function(n: int) -> int:
    return sum(range(n))

# Decorator factory (with arguments)
def retry(
    max_attempts: int = 3,
    exceptions: Tuple[Type[Exception], ...] = (Exception,),
) -> Callable:
    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            last_exc: Exception = RuntimeError('No attempts made')
            for attempt in range(1, max_attempts + 1):
                try:
                    return func(*args, **kwargs)
                except exceptions as e:
                    last_exc = e
                    import logging
                    logging.warning(
                        'Attempt %d/%d for %s failed: %s',
                        attempt, max_attempts, func.__name__, e,
                    )
            raise last_exc
        return wrapper
    return decorator

@retry(max_attempts=5, exceptions=(IOError, OSError))
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
      │         ├── UnicodeEncodeError
      │         └── UnicodeTranslateError
      ├── TypeError
      ├── OSError  (also IOError, EnvironmentError — all unified under OSError in Py3)
      │    ├── FileNotFoundError
      │    ├── PermissionError
      │    └── TimeoutError
      ├── AttributeError
      ├── ImportError
      │    └── ModuleNotFoundError  (new in Python 3.6)
      ├── RuntimeError
      │    └── RecursionError  (new in Python 3.5)
      ├── StopIteration
      └── ...
```

**Python 3 improvements:**
- `IOError` and `OSError` are now the same class (`OSError`). `IOError` is a backwards-compatibility alias.[^3]
- Specific `OSError` subclasses (`FileNotFoundError`, `PermissionError`, `TimeoutError`) allow fine-grained catching without checking `errno`.[^3]
- `ModuleNotFoundError` (subclass of `ImportError`) is raised when a module is not found.[^1]

```python
# Python 2 pattern — do not use in Python 3
try:
    open('/nonexistent')
except IOError as e:
    if e.errno == 2:   # errno.ENOENT
        handle_not_found()

# Python 3 pattern — cleaner
try:
    open('/nonexistent')
except FileNotFoundError:
    handle_not_found()
except PermissionError:
    handle_permission_denied()
```

**Pitfall: Never catch `BaseException` or bare `except:` in production:**[^11]

```python
# BAD — swallows Ctrl-C and sys.exit()
try:
    do_work()
except:
    pass

# GOOD
try:
    result = int(user_input)
except ValueError as e:
    logger.warning('Invalid input %r: %s', user_input, e)
    result = default_value
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
        logger.error('Malformed config %r: %s', path, e)
        raise ValueError(f'Config file {path!r} contains invalid JSON') from e
    except OSError as e:
        logger.error('Cannot read config %r: %s', path, e)
        raise
    else:
        # Only runs if no exception occurred in try block
        return config
    finally:
        # Always runs — even when exceptions propagate
        logger.debug('read_config finished for %r', path)
```

##### Exception Chaining (Python 3)

Python 3 supports explicit exception chaining via `raise ... from`:[^12]

```python
# Explicit chaining — sets __cause__
try:
    parse_config(text)
except ValueError as e:
    raise ConfigError('Config is malformed') from e
# Traceback: "The above exception was the direct cause of..."

# Suppress chaining — raise from None
try:
    int('N/A')
except ValueError:
    raise RuntimeError('Failed to parse value') from None
# Only RuntimeError shown, original ValueError hidden

# Inspect chain
try:
    raise_something()
except RuntimeError as e:
    print(e.__cause__)    # explicitly chained
    print(e.__context__)  # implicitly chained
```

**Rule:** Use `raise New(...) from original` when raising a descriptive exception in response to a lower-level one. Use `from None` when the original is a private implementation detail.

##### Custom Exceptions

```python
class AppError(Exception):
    """Base class for application-specific exceptions."""

class ConfigError(AppError):
    def __init__(self, key: str, message: str = '') -> None:
        self.key = key
        msg = f'Config error for key {key!r}'
        if message:
            msg += f': {message}'
        super().__init__(msg)

class NetworkError(AppError):
    def __init__(self, url: str, status_code: int = None) -> None:
        self.url = url
        self.status_code = status_code
        msg = f'Network error for {url!r}'
        if status_code:
            msg += f' (HTTP {status_code})'
        super().__init__(msg)
```

##### Re-raising Exceptions

```python
try:
    do_work()
except SomeError as e:
    logger.error('Error: %s', e, exc_info=True)
    raise   # bare raise — re-raises with the original traceback
    # Avoid 'raise e': the original traceback is kept (it travels on
    # e.__traceback__) but this line is appended as a misleading extra frame
```

***

#### Context Managers and the `with` Statement

```python
with open('data.txt', 'r', encoding='utf-8') as f:
    content = f.read()

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
        return False   # False: do not suppress the exception

from contextlib import contextmanager
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

# contextlib.suppress — idiomatic no-op for expected exceptions
from contextlib import suppress
import os

with suppress(FileNotFoundError):
    os.remove('/tmp/optional_cache_file')
```

***

#### File I/O

##### Text Files

In Python 3, `open()` in text mode returns a `TextIOWrapper` that decodes bytes to `str` automatically. Always pass `encoding` explicitly:[^4]

```python
# Reading text
with open('file.txt', 'r', encoding='utf-8') as f:
    content = f.read()

with open('large_file.txt', 'r', encoding='utf-8') as f:
    for line in f:                # lazy iteration — O(1) memory
        process(line.rstrip('\n'))

# Writing text
with open('out.txt', 'w', encoding='utf-8') as f:
    f.write('line 1\n')
    f.writelines(['line 2\n', 'line 3\n'])
```

**`errors` parameter:**
- `'strict'` (default): raises `UnicodeDecodeError`
- `'replace'`: replaces bad bytes with `?`
- `'ignore'`: silently drops undecodable bytes

##### Binary Files

```python
with open('image.png', 'rb') as f:
    data = f.read()

with open('copy.png', 'wb') as f:
    f.write(data)

CHUNK_SIZE = 65536  # 64 KiB
with open('large.bin', 'rb') as f:
    while True:
        chunk = f.read(CHUNK_SIZE)
        if not chunk:
            break
        process(chunk)
```

##### `pathlib` — Object-Oriented Paths (Python 3.4+, `os.PathLike` support new in 3.6)

`pathlib.Path` is the preferred path API in Python 3. In Python 3.6, `Path` objects implement `os.PathLike` (PEP 519) and work directly with `open()` and all standard library functions:[^13]

```python
from pathlib import Path

p = Path('/home/alice/data.csv')
p = Path.home() / 'data' / 'output.csv'   # / operator for joining

print(p.name)       # 'output.csv'
print(p.stem)       # 'output'
print(p.suffix)     # '.csv'
print(p.parent)     # PosixPath('/home/alice/data')
print(p.parts)      # ('/', 'home', 'alice', 'data', 'output.csv')

print(p.exists())
print(p.is_file())
print(p.is_dir())

# Read and write directly (Python 3 pathlib convenience methods)
text = p.read_text(encoding='utf-8')
p.write_text('hello\n', encoding='utf-8')
data = p.read_bytes()
p.write_bytes(b'\x00\x01')

# Directory operations
p.parent.mkdir(parents=True, exist_ok=True)
p.unlink()
p.rename(p.with_suffix('.txt'))

# Globbing
for csv_file in Path('.').glob('**/*.csv'):   # recursive
    process(csv_file)

# Use Path directly with open() in Python 3.6+
with open(Path('data.txt'), 'r', encoding='utf-8') as f:
    text = f.read()
```

##### Temporary Files

```python
import tempfile

# Secure temporary file
with tempfile.NamedTemporaryFile(
    suffix='.csv', mode='w', encoding='utf-8', delete=True,
) as tmp:
    tmp.write('col1,col2\n')
    tmp.flush()
    process(tmp.name)

# Temporary directory — context manager in Python 3
with tempfile.TemporaryDirectory() as tmpdir:
    work_in(tmpdir)
# directory deleted here automatically
```

**Do NOT use `tempfile.mktemp()`** — deprecated and vulnerable to TOCTOU race conditions.[^11] In Python 3, `TemporaryDirectory()` is a proper context manager, unlike Python 2's `mkdtemp()`.

***

#### `async`/`await` and `asyncio`

`asyncio` became stable (no longer provisional) in Python 3.6.[^1]

##### Mental Model

- `asyncio` runs on a **single thread** with cooperative scheduling.[^14]
- Tasks yield control only when they explicitly `await`.
- Ideal for **I/O-bound** work (HTTP, databases, file I/O).
- A blocking call inside `async def` **freezes the entire event loop** for all tasks.

##### Basic Patterns

```python
import asyncio
from typing import List

async def fetch_data(url: str) -> str:
    await asyncio.sleep(0.1)   # yield to event loop
    return f'data from {url}'

async def main() -> None:
    result = await fetch_data('https://api.example.com/users')
    print(result)

# Python 3.6: asyncio.run() does not exist — use the event loop directly
loop = asyncio.get_event_loop()
loop.run_until_complete(main())
loop.close()
```

##### Concurrent Tasks with `gather`

```python
async def fetch_all(urls: List[str]) -> List[str]:
    tasks = [fetch_data(url) for url in urls]
    results = await asyncio.gather(*tasks)
    return list(results)

# return_exceptions=True — collect results AND exceptions instead of raising
results = await asyncio.gather(*tasks, return_exceptions=True)
for r in results:
    if isinstance(r, Exception):
        handle_error(r)
```

##### Bounded Concurrency with Semaphore

```python
async def limited_fetch(semaphore: asyncio.Semaphore, url: str) -> str:
    async with semaphore:
        return await do_fetch(url)

async def fetch_all_bounded(urls: List[str], limit: int = 50) -> List[str]:
    semaphore = asyncio.Semaphore(limit)
    tasks = [limited_fetch(semaphore, url) for url in urls]
    return await asyncio.gather(*tasks)
```

##### `run_in_executor` — Blocking Calls in Async Code

```python
async def read_file_async(path: str) -> str:
    """Read a file without blocking the event loop."""
    loop = asyncio.get_event_loop()
    # None selects the loop's default ThreadPoolExecutor, created once and
    # reused. Do NOT build a ThreadPoolExecutor per call: thread startup is
    # wasted work, and leaving its `with` block calls shutdown() synchronously
    # on the event loop thread.
    # run_in_executor forwards positional args only ('utf-8' is read_text's
    # encoding parameter); use functools.partial when you need keyword args.
    content = await loop.run_in_executor(None, Path(path).read_text, 'utf-8')
    return content
```

##### Asynchronous Comprehensions (PEP 530, new in 3.6)

```python
async def get_values() -> list:
    return [i async for i in async_generator()]

async def get_filtered() -> list:
    return [await fetch(url) for url in urls if await is_valid(url)]
```

##### Common asyncio Pitfalls

```python
# PITFALL 1: Forgetting to await — coroutine runs never
async def broken():
    result = fetch_data('url')  # Missing await! Coroutine object, never runs.
    result = await fetch_data('url')  # Correct

# PITFALL 2: Blocking calls freeze the event loop
async def broken2():
    import requests
    r = requests.get('https://example.com')  # blocks entire event loop!
    # Fix: use aiohttp, or loop.run_in_executor()

# PITFALL 3: asyncio.run() doesn't exist in Python 3.6
# asyncio.run(main())  # AttributeError in 3.6!
loop = asyncio.get_event_loop()          # correct in 3.6
loop.run_until_complete(main())
loop.close()

# PITFALL 4: Timeout missing — one slow call can hang indefinitely
result = await asyncio.wait_for(do_fetch(url), timeout=5.0)  # always add timeout
```

***

#### Modules and Packages

```python
import os
import os.path
from os import path, getcwd
import numpy as np
from mypackage import mymodule
```

**Absolute imports are the default in Python 3.** Never need `from __future__ import absolute_import`.

Explicit relative imports:

```python
from . import sibling_module       # same package
from .. import parent_module       # parent package
from .utils import helper_function
```

##### `__all__`

```python
__all__ = ['PublicClass', 'public_function', 'CONSTANT']

CONSTANT: int = 42

def public_function() -> None:
    pass

def _private_helper() -> None:  # excluded from star imports
    pass
```

***

#### The `secrets` Module (new in Python 3.6)

Python 3.6 introduces `secrets` for cryptographically strong random values suitable for security-sensitive purposes:[^1]

```python
import secrets
import string

# Secure random bytes
token_bytes = secrets.token_bytes(32)         # 32 random bytes

# Secure hex token (64 hex chars = 32 bytes of entropy)
token_hex = secrets.token_hex(32)             # 'a3f4b2...'

# URL-safe token (base64 encoded)
token_url = secrets.token_urlsafe(32)         # 'A3f4B2...'

# Secure random integer in range [0, n)
roll = secrets.randbelow(100)

# Secure choice from a sequence
alphabet = string.ascii_letters + string.digits
password = ''.join(secrets.choice(alphabet) for _ in range(16))

# Constant-time comparison — prevents timing attacks on token checks
if secrets.compare_digest(provided_token, stored_token):
    grant_access()
```

**Rule:** Use `secrets` for every value that must be unpredictable: session tokens, CSRF tokens, API keys, password reset links.[^1] **Never use `random` module for security-sensitive values** — it is a deterministic PRNG seeded from the clock and is predictable.

---

#### Secure Coding Patterns

##### Input Validation

```python
def validate_age(value: object) -> int:
    try:
        age = int(str(value))
    except (ValueError, TypeError):
        raise ValueError(f'Age must be an integer, got {value!r}')
    if not (0 <= age <= 150):
        raise ValueError(f'Age must be between 0 and 150, got {age}')
    return age
```

##### Path Traversal Prevention

```python
from pathlib import Path

def safe_read_file(base_dir: str, user_filename: str) -> str:
    base = Path(base_dir).resolve()
    requested = (base / user_filename).resolve()
    try:
        requested.relative_to(base)
    except ValueError:
        raise PermissionError(f'Access denied: {user_filename!r} escapes {base_dir!r}')
    return requested.read_text(encoding='utf-8')
```

##### SQL Injection Prevention

```python
import sqlite3

# DANGEROUS — never do this
def bad_query(conn, user_id):
    conn.execute(f'SELECT * FROM users WHERE id = {user_id}')  # injection!

# SAFE — parameterized queries
def safe_query(conn: sqlite3.Connection, user_id: int) -> list:
    cursor = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,))
    return cursor.fetchall()
```

##### Password Hashing

```python
# pip install bcrypt  (bcrypt <= 4.0.x is the last line supporting Python 3.6)
import bcrypt

def hash_password(password: str) -> bytes:
    salt = bcrypt.gensalt(rounds=12)
    return bcrypt.hashpw(password.encode('utf-8'), salt)

def verify_password(stored_hash: bytes, provided: str) -> bool:
    return bcrypt.checkpw(provided.encode('utf-8'), stored_hash)

# Built-in fallback — hashlib.scrypt (requires OpenSSL 1.1+)
import hashlib, os, hmac

def hash_password_scrypt(password: str) -> str:
    salt = os.urandom(16)
    dk = hashlib.scrypt(
        password.encode('utf-8'), salt=salt,
        n=2**14, r=8, p=1, dklen=32,
    )
    return salt.hex() + ':' + dk.hex()
```

---

#### Logging

```python
import logging

logger = logging.getLogger(__name__)

def setup_logging(level: str = 'INFO') -> None:
    logging.basicConfig(
        level=getattr(logging, level.upper()),
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        datefmt='%Y-%m-%dT%H:%M:%S',
    )

# Use % style in logging — NOT f-strings (deferred evaluation)
logger.debug('Processing item: %r', item)
logger.info('Loaded %d records', count)
logger.warning('Config key %r missing, using default', key)
logger.error('Failed to connect: %s', exc)
logger.critical('Unrecoverable state, shutting down')

try:
    risky()
except Exception:
    logger.exception('Unexpected error')  # includes traceback automatically
```

**Critical rules:**[^14]
- Use `logging.getLogger(__name__)` in each module.
- **Use `%` style in logging, NOT f-strings.** `logger.info(f'Value: {x}')` evaluates the f-string even when INFO is suppressed. `logger.info('Value: %r', x)` defers formatting to only when needed.
- Configure handlers only in the application entry point.
- Never call `logging.basicConfig()` in library modules.

---

#### Regular Expressions

```python
import re

EMAIL_RE = re.compile(r'^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$')

m = EMAIL_RE.match(email)
matches = re.findall(r'\d+', text)
cleaned = re.sub(r'<[^>]+>', '', html)

# Named groups — dict-style access is new in Python 3.6
DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')
m = DATE_RE.match('2016-12-23')
if m:
    print(m['year'], m['month'], m['day'])   # dict-style: new in 3.6
```

**Pitfall: Greedy vs non-greedy:**

```python
html = '<b>bold</b> and <i>italic</i>'
re.findall(r'<.+>', html)    # ['<b>bold</b> and <i>italic</i>']  — greedy
re.findall(r'<.+?>', html)   # ['<b>', '</b>', '<i>', '</i>']     — non-greedy
```

---

#### Subprocess and Shell Execution

```python
import subprocess

# SAFE — list form, no shell
result = subprocess.check_output(
    ['ls', '-la', '/tmp'],
    stderr=subprocess.STDOUT,
    encoding='utf-8',   # Python 3.6+: encoding parameter decodes to str
)

# subprocess.run() — preferred high-level API
result = subprocess.run(
    ['git', 'status'],
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    encoding='utf-8',  # text mode for Python 3.6
    check=True,         # raises CalledProcessError on non-zero exit
)
print(result.stdout)

# DANGEROUS — never interpolate user input into shell=True
user_dir = get_user_input()
# subprocess.call(f'ls {user_dir}', shell=True)  # shell injection!
subprocess.call(['ls', user_dir])   # SAFE — no shell
```

---

#### Concurrency

##### The GIL

CPython's Global Interpreter Lock (GIL) means only one thread runs Python bytecode at a time:[^15]
- `threading` is effective for **I/O-bound** work (threads release the GIL while waiting).
- `threading` does NOT give CPU parallelism for pure Python.
- `multiprocessing` bypasses the GIL using separate processes.
- `asyncio` provides I/O concurrency on a single thread.

##### `threading`

```python
import threading
from queue import Queue

lock = threading.Lock()
results: list = []

def worker(task_id: int) -> None:
    result = do_work(task_id)
    with lock:
        results.append(result)

threads = []
for i in range(10):
    t = threading.Thread(target=worker, args=(i,))
    t.daemon = True
    t.start()
    threads.append(t)

for t in threads:
    t.join()

# Thread-safe queue
q: Queue = Queue(maxsize=100)
```

##### `concurrent.futures` — High-Level Concurrency (Recommended)

```python
from concurrent.futures import ThreadPoolExecutor, ProcessPoolExecutor
import concurrent.futures

# I/O-bound: ThreadPoolExecutor
with ThreadPoolExecutor(max_workers=10) as executor:
    futures = {executor.submit(fetch_url, url): url for url in urls}
    for future in concurrent.futures.as_completed(futures):
        url = futures[future]
        try:
            data = future.result()
            process(data)
        except Exception as e:
            logger.error('Failed %s: %s', url, e)

# CPU-bound: ProcessPoolExecutor
with ProcessPoolExecutor(max_workers=4) as executor:
    results = list(executor.map(cpu_task, range(10)))
```

##### `multiprocessing`

```python
from multiprocessing import Pool, cpu_count

def process_chunk(chunk: list) -> list:
    return [expensive_transform(item) for item in chunk]

if __name__ == '__main__':   # REQUIRED — prevents recursive spawning on Windows
    data = load_large_dataset()
    n = cpu_count()
    chunks = [data[i::n] for i in range(n)]

    with Pool(processes=n) as pool:   # Pool is a context manager in Python 3
        results = pool.map(process_chunk, chunks)

    flat = [item for sublist in results for item in sublist]
```

---

#### Memory Management

```python
import sys
import gc

x = [1, 2, 3]
print(sys.getrefcount(x))  # 2

gc.collect()
gc.set_threshold(700, 10, 10)

# tracemalloc — memory profiling (enhanced in 3.6)
import tracemalloc
tracemalloc.start(10)   # keep 10-frame tracebacks
snapshot = tracemalloc.take_snapshot()
for stat in snapshot.statistics('lineno')[:10]:
    print(stat)

# Optimization patterns
total = sum(x**2 for x in range(1_000_000))   # generator — O(1) memory

import array
int_array = array.array('i', [1, 2, 3])       # less memory than list

import weakref
cache: weakref.WeakValueDictionary = weakref.WeakValueDictionary()
```

---

#### JSON, CSV, and Data Serialization

##### JSON

```python
import json

data = {'name': 'Alice', 'age': 30, 'scores': [95, 87, 92]}

json_str = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False)
restored = json.loads(json_str)
json.loads(b'{"key": "value"}')   # bytes accepted in Python 3.6+

with open('data.json', 'w', encoding='utf-8') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)

# Custom encoder for non-JSON types
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

# Python 3: text mode + newline='' (prevents double newlines on Windows)
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

**Python 3 CSV:** Unlike Python 2, use text mode (not `'rb'`), `newline=''`, and Unicode is handled natively — no wrapper needed.

---

#### Performance Patterns and Optimization

##### Profiling First

```python
import cProfile, pstats, io

pr = cProfile.Profile()
pr.enable()
main()
pr.disable()

stream = io.StringIO()
pstats.Stats(pr, stream=stream).sort_stats('cumulative').print_stats(20)
print(stream.getvalue())

import timeit
elapsed = timeit.timeit('sum(range(1000))', number=10_000)
```

##### Common Patterns

```python
# Local variable alias (faster attribute lookup)
sqrt = math.sqrt
for i in range(100_000):
    result = sqrt(i)

# O(n) string building
result = ''.join(str(item) for item in data)

# O(1) membership testing
allowed = frozenset({'admin', 'editor', 'viewer'})
if role in allowed:
    grant_access()

# defaultdict
from collections import defaultdict
d: defaultdict = defaultdict(list)
d[key].append(val)

# enumerate — avoid manual index tracking
for i, item in enumerate(lst, start=1):
    print(f'{i}: {item}')

# Lazy pipelines
total = sum(process(x) for x in huge_data if predicate(x))
```

---

#### Virtual Environments and Packaging

##### `venv` — Built-in (Python 3.3+)

```bash
# Create virtual environment
python3.6 -m venv .venv

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
requires = ["setuptools>=40.8", "wheel"]
build-backend = "setuptools.build_meta"

[tool.black]
line-length = 88
target-version = ["py36"]

[tool.mypy]
python_version = "3.6"
strict = true

[tool.pytest.ini_options]
testpaths = ["tests"]
```

**Tool-version caveats for a 3.6 interpreter:** `[tool.pytest.ini_options]` needs pytest >= 6.0 (pytest 7.0.x is the last series that runs on 3.6); `[tool.mypy]` needs mypy >= 0.900 (mypy 0.971 is the last that runs on 3.6; mypy 1.5.x the last that can still *target* `python_version = "3.6"` from a newer interpreter); Black 22.8.0 is the last release that runs on 3.6 (newer Black still formats `target-version = ["py36"]` code from a newer interpreter). On a pure 3.6 toolchain, `setup.cfg` remains the more common home for this configuration.

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

---

#### Testing

```python
import pytest
from unittest.mock import Mock, patch, MagicMock
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

def test_with_mock() -> None:
    mock_response = Mock()
    mock_response.json.return_value = {'id': 1, 'name': 'Test'}
    mock_response.status_code = 200

    with patch('requests.get', return_value=mock_response) as mock_get:
        result = fetch_user(1)

    mock_get.assert_called_once_with('/users/1')
    assert result['name'] == 'Test'
```

**Python 3 advantage:** `unittest.mock` is in the standard library. No separate `mock` package needed (unlike Python 2).

---

#### Code Quality Tools

```bash
pip install mypy flake8 black isort bandit

# Type checking
mypy --strict myapp/

# Linting (compatible with Black's style; W503 is off by default — never enable it)
flake8 myapp/ --max-line-length=88 --extend-ignore=E203

# Auto-formatting (de-facto Python 3 standard)
black myapp/ tests/

# Import sorting (Black-compatible profile)
isort --profile=black myapp/ tests/

# Security audit
bandit -r myapp/

# CI check pipeline
black --check myapp/ && isort --check-only myapp/ && flake8 myapp/ && mypy myapp/
```

---

#### Production Checklist

**Code correctness:**
- [ ] All f-strings use valid expressions (no backslashes inside `{}`)
- [ ] `bytes` and `str` never mixed without explicit encode/decode
- [ ] `encoding='utf-8'` passed to all `open()` calls for text files
- [ ] `newline=''` passed to `open()` for CSV files
- [ ] No mutable default arguments in function signatures (`def f(x=[])` is WRONG)
- [ ] Mutable `dataclass` fields use `field(default_factory=...)`, not `= []`
- [ ] `__hash__` defined whenever `__eq__` is defined
- [ ] All abstract methods implemented in concrete subclasses

**Python 3.6-specific:**
- [ ] f-strings used for all interpolation in application code
- [ ] Variable annotations present on module-level and instance variables
- [ ] `typing.NamedTuple` class syntax used (not `collections.namedtuple`)
- [ ] `enum.auto()` used for auto-value enums; `Flag` for bitfield enums
- [ ] `secrets` module used for all security tokens (not `random`)
- [ ] `pathlib.Path` used for file system operations
- [ ] `asyncio.run()` NOT used (it's 3.7+); use `loop.run_until_complete()` in 3.6

**Safety and security:**
- [ ] All SQL queries use parameterized form — no f-string SQL interpolation
- [ ] `pickle.load()` never called on untrusted data
- [ ] `shell=True` never used with any user-controlled input
- [ ] `eval()` and `exec()` never called on untrusted input
- [ ] `secrets.compare_digest()` used for all token comparisons
- [ ] Path traversal check (`Path.relative_to()`) in all file-accepting endpoints
- [ ] `tempfile.NamedTemporaryFile` / `TemporaryDirectory` used (not `mktemp`)

**Performance:**
- [ ] Profiled before optimizing — no premature optimization
- [ ] Generator expressions used in pipelines (not list comprehensions)
- [ ] Membership tests on frequently searched collections use `set`/`frozenset`
- [ ] `str.join()` used instead of loop `+=` for string building
- [ ] `logging` uses `%`-style deferred formatting (not f-strings)
- [ ] Long-running asyncio code does not contain blocking calls

**Concurrency:**
- [ ] `if __name__ == '__main__':` guard present in multiprocessing scripts
- [ ] Shared mutable state protected with appropriate locks
- [ ] Async code uses `run_in_executor` for any blocking I/O
- [ ] `asyncio.gather(..., return_exceptions=True)` used when partial failure is acceptable

**Observability:**
- [ ] `logging.getLogger(__name__)` used in every module
- [ ] No `print()` in production code paths
- [ ] All custom exceptions inherit from `Exception` (not `BaseException`)
- [ ] `logger.exception()` used inside `except` blocks to capture tracebacks
- [ ] Bare `raise` (not `raise e`) to preserve original tracebacks

**Packaging:**
- [ ] `venv` environment present and activated for development
- [ ] `requirements.txt` pinned with exact versions (`pip freeze`)
- [ ] `.venv/` in `.gitignore`
- [ ] `mypy --strict` passes cleanly
- [ ] All tests pass under `pytest`

---

[^1]: Python 3.6 What's New. Python Software Foundation. https://docs.python.org/3/whatsnew/3.6.html
[^2]: Python 3 Data Model — Classes. https://docs.python.org/3.6/reference/datamodel.html
[^3]: Python 3 Built-in Types. https://docs.python.org/3.6/library/stdtypes.html
[^4]: Python 3 io module. https://docs.python.org/3.6/library/io.html
[^5]: PEP 498 – Literal String Interpolation. https://peps.python.org/pep-0498/
[^6]: typing.NamedTuple. https://docs.python.org/3.6/library/typing.html#typing.NamedTuple
[^7]: Python 3 Expressions — Comprehensions. https://docs.python.org/3/reference/expressions.html
[^8]: CPython dict implementation — compact representation. Python 3.6 What's New.
[^9]: Python Data Model — `__slots__`. https://docs.python.org/3/reference/datamodel.html
[^10]: Python enum module. https://docs.python.org/3.6/library/enum.html
[^11]: Python Security Best Practices. OWASP Python Security Project.
[^12]: PEP 3134 – Exception Chaining and Embedded Tracebacks. https://peps.python.org/pep-3134/
[^13]: pathlib module. https://docs.python.org/3.6/library/pathlib.html
[^14]: Python logging HOWTO. https://docs.python.org/3.6/howto/logging.html
[^15]: Python GIL. https://docs.python.org/3/glossary.html#term-global-interpreter-lock
