### Python 2.7: The Complete Production Developer Guide

#### Overview

Python 2.7 is the final release of the Python 2 line, released on July 3, 2010.[^1] It is a dynamically typed, interpreted, general-purpose language with a clean, whitespace-significant syntax. This guide covers every language feature, data type, pattern, and production consideration you need to write rock-solid, efficient Python 2.7 code.

A few global truths about Python 2.7 before diving in:

- **Everything is an object.** Integers, functions, classes, modules — all are objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing.** Variables carry no declared type; the type lives in the value.
- **Reference counting + cyclic GC.** CPython manages memory with reference counts plus a cyclic garbage collector.

***

#### File Encoding and the `__future__` Module

##### Source File Encoding

By default, Python 2.7 assumes ASCII source encoding. Any non-ASCII byte in a source file without a declared encoding declaration produces a `SyntaxError`.[^2] Always declare the encoding in the first or second line:

```python
# -*- coding: utf-8 -*-
```

This is mandatory whenever your source contains Unicode literals, docstrings with non-ASCII characters, or comments in non-ASCII scripts.[^2]

##### The `__future__` Module

Python 2.7 provides a mechanism to opt into Python 3 behaviors at the module level. These imports must appear before any other code (except the docstring and encoding declaration).[^3]

```python
from __future__ import print_function    # print() becomes a function
from __future__ import division          # / always does true division
from __future__ import unicode_literals  # string literals are unicode by default
from __future__ import absolute_import   # disables implicit relative imports
```

These are not optional suggestions for production code — they are essential guards against the most common Python 2.7 pitfalls (integer division truncation, print statement syntax, silent ASCII encoding errors).[^3][^4]

**Pitfall:** `import __future__` (without `from`) does NOT activate any feature; only `from __future__ import X` syntax triggers the compiler special-case behavior.[^3]

**Note on the examples in this guide:** code samples deliberately use the interpreter's *default* semantics — `print` statements, truncating `/` division — so each snippet runs as-is in a bare 2.7 session and matches what legacy code looks like. New production modules should start with the `__future__` imports above and use the `print()` function form; converting a multi-argument `print a, b` requires the import, since without it `print(a, b)` prints a tuple (see The `print` Statement).

#### PEP 8 Code Style

##### Naming Conventions

```python
# Variables and functions: snake_case
user_name = 'John'
def calculate_total(items):
    pass

# Constants: SCREAMING_SNAKE_CASE (module level)
MAX_CONNECTIONS = 100
DEFAULT_TIMEOUT = 30

# Classes: PascalCase — and always new-style (inherit from object)
class UserAccount(object):
    pass

# Internal use: single underscore prefix
class User(object):
    def __init__(self):
        self._internal_state = {}

# Name mangling: double underscore prefix avoids subclass attribute
# collisions — not true privacy (still reachable as _Base__private)
class Base(object):
    def __init__(self):
        self.__private = 'name-mangled'

# Module-level "private": single underscore, and excluded from __all__
_module_cache = {}
```

##### Indentation and Line Length

```python
# 4 spaces per indentation level — never tabs, never mixed
# (running with python -tt makes mixed indentation an error)
def function():
    if condition:
        do_something()

# Line length: 79 characters (PEP 8). Black and Ruff never supported
# Python 2 source, so there is no 88-character formatter convention here.
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

Group imports in three blocks separated by blank lines: standard library,
third-party, local application — alphabetized within each block (`isort`
4.3.x automates this on Python 2; see Code Quality Tools).

```python
# Standard library
import os
import sys

# Third-party
import requests
import six

# Local application
from myapp.models import User
from myapp.utils import format_date

# Avoid wildcard imports — they defeat static analysis and __all__ hygiene
# Bad:  from module import *
# Good: from module import specific_item
```

***

#### Primitive Types and Numerics

##### Integers: `int` and `long`

Python 2.7 has two integer types.[^5]

- **`int`**: A C `long`, platform-dependent (at least 32 bits). `sys.maxint` is the maximum value.
- **`long`**: Arbitrary precision integers. Python automatically promotes `int` to `long` on overflow.

```python
import sys
print sys.maxint          # 9223372036854775807 on 64-bit
x = sys.maxint + 1        # automatically becomes long
print type(x)             # <type 'long'>
x_explicit = 123456789L   # L suffix forces long literal (uppercase L preferred)
```

**Pitfall:** Integer literal with lowercase `l` (`1l`) looks like `11`. Always use uppercase `L`.[^5]

**Bitwise operations** work on both `int` and `long`:[^5]

```python
a = 0b1010   # binary literal
b = 0xFF     # hex literal
c = 0o17     # octal literal
print a & b  # AND
print a | b  # OR
print a ^ b  # XOR
print ~a     # NOT (bitwise complement)
print a << 2 # left shift
print a >> 1 # right shift
```

**Negative shift counts raise `ValueError`.** Left shift by `n` is equivalent to multiplication by `2**n`; the result promotes to `long` if it overflows `int`.[^5]

##### Floats

Floats are C `double` (64-bit IEEE 754).[^5]

```python
x = 3.14
y = 1.5e-10
z = float('inf')    # positive infinity
n = float('nan')    # Not a Number (added in 2.6)
print x.is_integer()         # False
print (2.0).is_integer()     # True
print x.as_integer_ratio()   # (7070651414971679, 2251799813685248)
```

**Pitfall: Float comparison.** Never use `==` to compare floats. Use `abs(a - b) < epsilon`:

```python
# BAD — may fail due to IEEE 754 representation
assert 0.1 + 0.2 == 0.3

# GOOD
EPSILON = 1e-9
assert abs((0.1 + 0.2) - 0.3) < EPSILON
```

For exact decimal arithmetic, use the `decimal` module:

```python
from decimal import Decimal, getcontext
getcontext().prec = 28
result = Decimal('0.1') + Decimal('0.2')
print result  # 0.3
```

##### Booleans

`bool` is a subtype of `int`. `True == 1` and `False == 0`.[^5]

```python
print isinstance(True, int)  # True
print True + True             # 2
print True * 5                # 5
```

**Falsy values** in Python 2.7:[^5]
- `None`
- `False`
- Zero of any numeric type: `0`, `0L`, `0.0`, `0j`
- Empty sequences: `''`, `u''`, `()`, `[]`
- Empty mapping: `{}`
- Instances of classes whose `__nonzero__` (Python 2's spelling of `__bool__`) or `__len__` returns zero/False

Everything else is truthy.

##### Complex Numbers

```python
z = 3 + 4j
print z.real   # 3.0
print z.imag   # 4.0
print abs(z)   # 5.0 — magnitude
print z.conjugate()  # (3-4j)
```

***

#### Strings: `str` vs `unicode`

This is the single most important area to understand in Python 2.7 for production code.

##### The Two String Types

- **`str`**: A sequence of raw bytes (8-bit). The "native" string type. Does NOT inherently carry encoding information.[^2][^6]
- **`unicode`**: A sequence of Unicode code points. Declared with `u''` prefix or created by `unicode()` or `.decode()`.[^2]

```python
byte_str = 'hello'           # str — bytes
uni_str  = u'hello'          # unicode — code points
uni_str2 = u'\u03b1\u03b2'   # greek alpha-beta
```

**The Unicode sandwich rule for production code:**[^2]
1. Decode all bytes to `unicode` on input boundaries (file reads, network, stdin, databases).
2. Work with `unicode` internally throughout your application.
3. Encode `unicode` back to `str`/bytes on output boundaries (file writes, network, stdout).

```python
def force_unicode(text, encoding='utf-8'):
    """Decode bytes to unicode; pass unicode through unchanged."""
    if isinstance(text, unicode):
        return text
    return text.decode(encoding)

def force_bytes(text, encoding='utf-8'):
    """Encode unicode to bytes; pass bytes through unchanged."""
    if isinstance(text, str):
        return text
    return text.encode(encoding)
```

##### Encoding and Decoding

```python
# DECODE: bytes -> unicode
byte_str = '\xc3\xa9'        # UTF-8 for 'é'
uni = byte_str.decode('utf-8')
print uni                     # u'\xe9'

# ENCODE: unicode -> bytes
uni = u'caf\xe9'
encoded = uni.encode('utf-8')
print encoded                 # 'caf\xc3\xa9'
```

**Pitfall: implicit ASCII coercion.** When you mix `str` and `unicode`, Python 2.7 implicitly tries to decode the `str` using ASCII. This works fine for pure-ASCII bytes but raises `UnicodeDecodeError` the moment any byte exceeds 127.[^2][^7]

```python
# DANGEROUS — works only if 'name' is pure ASCII
greeting = u'Hello, ' + name  # 'name' is str; implicit decode

# SAFE
greeting = u'Hello, ' + force_unicode(name)
```

**Pitfall: default encoding is ASCII.** Python 2.7's `sys.getdefaultencoding()` is `'ascii'`. Any implicit encode/decode uses ASCII unless you change this — which you should NOT do via `sys.setdefaultencoding()` as that causes subtle bugs across the whole runtime.[^8]

##### String Methods

All of the following work on both `str` and `unicode`:[^5]

```python
s = 'Hello, World!'
print s.upper()              # 'HELLO, WORLD!'
print s.lower()              # 'hello, world!'
print s.strip()              # strips whitespace
print s.lstrip('H')          # 'ello, World!'
print s.rstrip('!')          # 'Hello, World'
print s.replace('World', 'Python')  # 'Hello, Python!'
print s.startswith('Hello')  # True
print s.endswith('!')        # True
print s.split(', ')          # ['Hello', 'World!']
print ', '.join(['a','b','c'])  # 'a, b, c'
print s.find('World')        # 7
print s.index('World')       # 7  (raises ValueError if not found)
print s.count('l')           # 3
print s.center(20, '-')      # '---Hello, World!----'
print s.zfill(20)            # '0000000Hello, World!'
print 'abc'.partition('b')   # ('a', 'b', 'c')
print 'abc'.splitlines()     # ['abc']
```

**`find` vs `index`:** Use `in` operator to check membership; use `find` when you need the position; use `index` when absence is a programming error that should raise.[^5]

##### String Formatting

Python 2.7 supports two formatting styles:

**Old-style `%` formatting (printf-style):**

```python
name = 'Alice'
age  = 30
# Positional
print 'Name: %s, Age: %d' % (name, age)
# Named (dict-style)
print '%(name)s is %(age)d years old' % {'name': name, 'age': age}
# Width and precision
print '%10.4f' % 3.14159  # '    3.1416'
print '%-10s|' % 'left'   # 'left      |'
print '%05d' % 42         # '00042'
```

**New-style `.format()` (preferred in 2.7):**

```python
# Positional
print '{0} is {1} years old'.format(name, age)
# Named
print '{name} is {age} years old'.format(name=name, age=age)
# Formatting spec
print '{0:10.4f}'.format(3.14159)
print '{0!r}'.format('hello')   # repr
print '{0!s}'.format(42)        # str
# Fill and align
print '{0:>10}'.format('right')  # '     right'
print '{0:<10}'.format('left')   # 'left      '
print '{0:^10}'.format('ctr')    # '   ctr    '
```

**Recommendation:** Prefer `.format()` in new code for clarity and power, and use `%` only when working with logging (see Logging section below).

##### String Concatenation Performance

Building strings by concatenation in a loop is O(n²) because each `+` creates a new string object.[^5]

```python
# BAD — quadratic performance
result = ''
for item in large_list:
    result += str(item)

# GOOD — linear, uses join
result = ''.join(str(item) for item in large_list)
```

For small counts (< ~5 concatenations), `+` is fine. The threshold where `join` becomes measurably faster is around 6+ concatenations.[^5]

##### Raw Strings and Escape Sequences

```python
path     = r'C:\Users\alice\new_folder'  # r prefix: backslashes are literal
pattern  = r'\d+\.\d+'                  # raw string for regex
newline  = '\n'                          # escape still works without r
tab      = '\t'
null     = '\x00'
unicode_ = u'\u03b1'                     # unicode escape
```

##### Multiline Strings

```python
sql = """
    SELECT *
    FROM users
    WHERE active = 1
"""

docstring_example = (
    "This is a very long string that "
    "spans multiple lines in source "
    "but is one string at runtime."
)
```

Adjacent string literals (separated only by whitespace) are automatically concatenated at compile time — no `+` needed, no runtime cost.[^5]

***

#### Lists

Lists are mutable, ordered, heterogeneous sequences of objects.[^5]

```python
lst = [1, 'two', 3.0, [4, 5]]

# Indexing
print lst[0]     # 1
print lst[-1]    # [4, 5]

# Slicing
print lst[1:3]   # ['two', 3.0]
print lst[::-1]  # reversed
print lst[::2]   # every other element

# Mutation
lst.append(6)
lst.extend([7, 8])
lst.insert(0, 0)
lst.remove('two')    # removes first occurrence; raises ValueError if absent
popped = lst.pop()   # removes and returns last; pop(i) for arbitrary index
del lst[0]           # removes element at index 0 (del lst on the bare name
                     # would unbind the variable itself, not empty the list)

# Search
print 3.0 in lst          # True
print lst.index(3.0)      # position; raises ValueError if absent
print lst.count(6)        # count occurrences

# Sorting
lst.sort()                    # in-place, stable sort
lst.sort(key=lambda x: -x)   # custom key
sorted_copy = sorted(lst)     # returns new list, original unchanged
lst.reverse()                 # in-place

# Other
print len(lst)
lst2 = lst[:]                 # shallow copy
lst3 = list(lst)              # also shallow copy
```

**Pitfall: List multiplication shares references:**[^5]

```python
# WRONG — all three sublists are the SAME object
matrix = [[0] * 3] * 3
matrix[0][0] = 99
print matrix  # [[99, 0, 0], [99, 0, 0], [99, 0, 0]]

# CORRECT — each sublist is a distinct object
matrix = [[0] * 3 for _ in xrange(3)]
matrix[0][0] = 99
print matrix  # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

**Pitfall: Mutating a list while iterating it** leads to skipped elements:

```python
# WRONG
for item in lst:
    if condition(item):
        lst.remove(item)  # modifies lst during iteration

# CORRECT — iterate a copy
for item in lst[:]:
    if condition(item):
        lst.remove(item)

# ALSO CORRECT — list comprehension filter
lst = [item for item in lst if not condition(item)]
```

##### List Comprehensions

List comprehensions are the idiomatic Python way to build or transform lists.[^9]

```python
squares = [x**2 for x in xrange(10)]
evens   = [x for x in xrange(20) if x % 2 == 0]
flat    = [item for sublist in nested for item in sublist]
matrix  = [[i * j for j in xrange(1, 4)] for i in xrange(1, 4)]
```

**Pitfall:** In Python 2.7, the loop variable of a list comprehension leaks into the enclosing scope:

```python
x = 10
result = [x for x in xrange(5)]
print x   # 4 — x was overwritten! (Python 3 fixes this)

# Workaround: use generator expression or distinct variable name
result = [i for i in xrange(5)]
print x   # 10 — x untouched
```

***

#### Tuples

Tuples are immutable, ordered sequences. Immutability makes them hashable (if all elements are hashable), lighter than lists, and suitable as dictionary keys.[^5]

```python
t = (1, 2, 3)
single = (42,)   # trailing comma required for single-element tuple
empty  = ()

# Unpacking
a, b, c = t
a, b = b, a      # swap without temp variable (pythonic idiom)

# Iteration, slicing — same as lists
# Mutation — not supported; tuple is immutable

# Named tuples
from collections import namedtuple
Point = namedtuple('Point', ['x', 'y'])
p = Point(3, 4)
print p.x, p.y        # 3 4
print p[0], p[1]      # 3 4 — also supports index access
print p._asdict()     # OrderedDict([('x', 3), ('y', 4)])
```

**When to use tuple vs list:** Use tuples for heterogeneous collections of fixed structure (a record, a coordinate). Use lists for homogeneous collections of variable length.[^5]

***

#### Dictionaries

Dictionaries are mutable, unordered (insertion order is NOT preserved in Python 2.7 — that guarantee came in Python 3.7) mappings from hashable keys to arbitrary values.[^5]

```python
d = {'name': 'Alice', 'age': 30}

# Access
print d['name']               # 'Alice'; raises KeyError if absent
print d.get('age')            # 30; returns None if absent
print d.get('height', 0)      # 0 — default value
print d.setdefault('role', 'user')  # sets and returns default if absent

# Mutation
d['email'] = 'alice@example.com'
del d['age']
d.update({'city': 'Berlin', 'country': 'DE'})

# Iteration — IMPORTANT: know your method names
for key in d:                 # iterates keys (equivalent to d.iterkeys())
    pass

# Python 2.7-specific: prefer iter* methods for memory efficiency
for k, v in d.iteritems():    # iterator over (key, value) pairs — O(1) memory
    print k, v

for k in d.iterkeys():        # iterator over keys
    pass

for v in d.itervalues():      # iterator over values
    pass

# d.items() / d.keys() / d.values() return full LISTS — avoid in hot loops on large dicts
items_list = d.items()        # creates a copy of all pairs

# Membership test: always check keys
print 'name' in d             # True — O(1) average
print 'name' in d.keys()      # SLOWER — creates a list first, then O(n) scan
```

**Performance note:** `d.iteritems()`, `d.iterkeys()`, `d.itervalues()` return lazy iterators and do not copy the data. Prefer them in loops. `d.items()`, `d.keys()`, `d.values()` construct full lists — avoid in hot loops over large dicts or when you only need to iterate once.[^10]

**Pitfall:** Modifying a dictionary's keys while iterating it raises `RuntimeError`. Iterate a copy or collect modifications, then apply:

```python
# WRONG
for k in d:
    if should_delete(k):
        del d[k]

# CORRECT
keys_to_delete = [k for k in d if should_delete(k)]
for k in keys_to_delete:
    del d[k]
```

##### Dict Comprehensions (Python 2.7+)

```python
squares = {x: x**2 for x in xrange(10)}
inverted = {v: k for k, v in d.iteritems()}
filtered = {k: v for k, v in d.iteritems() if v is not None}
```

##### `collections.OrderedDict`

When insertion order matters (e.g., serialization, protocol headers):

```python
from collections import OrderedDict

od = OrderedDict()
od['first'] = 1
od['second'] = 2
od['third'] = 3
print list(od.keys())  # ['first', 'second', 'third'] — order preserved

# OrderedDict supports move_to_end via popitem (LIFO) or reinsertion
```

##### `collections.defaultdict`

```python
from collections import defaultdict

word_count = defaultdict(int)
for word in text.split():
    word_count[word] += 1   # no KeyError on first access

graph = defaultdict(list)
graph['A'].append('B')      # no need to check if 'A' exists

nested = defaultdict(lambda: defaultdict(int))
nested['row']['col'] += 1
```

##### `collections.Counter`

```python
from collections import Counter

c = Counter('abracadabra')
print c.most_common(3)   # [('a', 5), ('b', 2), ('r', 2)]
print c['a']             # 5
c.update('hello')
```

***

#### Sets

Sets are mutable, unordered collections of unique, hashable elements.[^5]

```python
s = {1, 2, 3}
s2 = set([1, 2, 2, 3])   # duplicate removed
empty_set = set()          # NOT {} — that's an empty dict!

s.add(4)
s.discard(10)   # no error if absent
s.remove(4)     # raises KeyError if absent
popped = s.pop()  # removes and returns an arbitrary element

# Set operations
a = {1, 2, 3, 4}
b = {3, 4, 5, 6}
print a | b     # union: {1, 2, 3, 4, 5, 6}
print a & b     # intersection: {3, 4}
print a - b     # difference: {1, 2}
print a ^ b     # symmetric difference: {1, 2, 5, 6}
print a <= b    # subset
print a >= b    # superset

# frozenset — immutable, hashable (can be dict key or element of another set)
fs = frozenset([1, 2, 3])
```

##### Set Comprehensions (Python 2.7+)

```python
unique_squares = {x**2 for x in range(-5, 6)}  # duplicates collapsed
```

***

#### `range` vs `xrange`

This is a critical Python 2.7 performance decision.[^11]

- **`range(n)`**: Returns a **list** of `n` integers. Allocates all values in memory at once. O(n) memory.
- **`xrange(n)`**: Returns a lazy **iterator** that generates values on demand. O(1) memory.

```python
# xrange — preferred for iteration, especially large ranges
for i in xrange(1000000):
    process(i)  # only one integer in memory at a time

# range — needed when you require a list (slicing, len, multiple passes, etc.)
indices = range(10)  # store as list for random access
```

**Rule of thumb:** In Python 2.7, use `xrange` in `for` loops unless you specifically need a list.[^12][^13]

```python
# Memory comparison (approximate)
import sys
print sys.getsizeof(range(1000))    # ~8072 bytes — full list
print sys.getsizeof(xrange(1000))   # 40 bytes — single object
```

**Pitfall:** `xrange` does not support slicing. If you need `range(start, stop)[::step]` as a list, use `range`.

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
```

**Ternary expression (Python 2.5+):**

```python
label = 'pass' if score >= 60 else 'fail'
```

##### `for` Loops

```python
for i, item in enumerate(my_list):
    print i, item

for key, val in d.iteritems():
    process(key, val)

for a, b in zip(list1, list2):   # stops at shortest
    print a, b

import itertools
for a, b in itertools.izip_longest(list1, list2, fillvalue=None):
    print a, b
```

**`for / else`**: The `else` block runs only if the loop was NOT terminated by `break`:

```python
for item in collection:
    if matches(item):
        result = item
        break
else:
    result = default_value   # only runs if no break occurred
```

##### `while` Loops

```python
while not done:
    process()

# Enumerate manually
i = 0
while i < len(lst):
    lst[i] = transform(lst[i])
    i += 1
```

##### `break`, `continue`, `pass`

```python
for x in data:
    if x is None:
        continue    # skip to next iteration
    if x < 0:
        break       # exit loop entirely
    process(x)

class EmptyClass(object):
    pass            # syntactic placeholder
```

***

#### Functions

##### Defining Functions

```python
def greet(name, greeting='Hello', punctuation='!'):
    """Return a greeting string.

    Args:
        name: The person to greet.
        greeting: The greeting word (default 'Hello').
        punctuation: Trailing punctuation (default '!').

    Returns:
        A formatted greeting string.
    """
    return '{0}, {1}{2}'.format(greeting, name, punctuation)
```

##### `*args` and `**kwargs`

```python
def variadic(*args, **kwargs):
    """Accept any positional and keyword arguments."""
    for i, arg in enumerate(args):
        print 'arg[{0}] = {1!r}'.format(i, arg)
    for key, val in sorted(kwargs.iteritems()):
        print '{0} = {1!r}'.format(key, val)

variadic(1, 2, three=3)

# Unpacking
def add(a, b, c):
    return a + b + c

nums = [1, 2, 3]
print add(*nums)            # unpacks list as positional args

opts = {'a': 1, 'b': 2, 'c': 3}
print add(**opts)           # unpacks dict as keyword args
```

##### First-Class Functions and Closures

Functions are objects. They can be stored, passed, and returned.

```python
def make_multiplier(n):
    """Return a closure that multiplies by n."""
    def multiplier(x):
        return x * n       # 'n' is captured from enclosing scope
    return multiplier

double = make_multiplier(2)
triple = make_multiplier(3)
print double(5)   # 10
print triple(5)   # 15
```

**Pitfall: Late binding in closures.** Closures capture the variable, not its value at the time of creation:

```python
# WRONG — all functions see the same 'i', which ends at 4
funcs = [lambda: i for i in xrange(5)]
print [f() for f in funcs]  # [4, 4, 4, 4, 4]

# CORRECT — use default argument to capture the value
funcs = [lambda i=i: i for i in xrange(5)]
print [f() for f in funcs]  # [0, 1, 2, 3, 4]
```

##### Lambda

`lambda` creates anonymous single-expression functions. Use them sparingly — named functions are almost always clearer:

```python
# Acceptable for sort key
names.sort(key=lambda s: s.lower())

# Acceptable for simple callbacks
button.on_click(lambda: handle_click())

# Avoid for anything complex — use a named function instead
```

##### `map`, `filter`, `reduce`

These are functional programming primitives, but list comprehensions and generator expressions are usually more readable.

```python
# map
squares = map(lambda x: x**2, xrange(10))  # returns a list in Python 2

# filter
evens = filter(lambda x: x % 2 == 0, xrange(10))  # returns a list

# reduce
from functools import reduce  # not needed in Py2 (it's a builtin), but good habit
total = reduce(lambda a, b: a + b, xrange(1, 6), 0)  # 15

# Prefer comprehensions and generator expressions:
squares = [x**2 for x in xrange(10)]
evens   = [x for x in xrange(10) if x % 2 == 0]
```

In Python 2.7, `map`, `filter`, and `zip` return lists. For memory-efficient lazy variants use `itertools.imap`, `itertools.ifilter`, and `itertools.izip`.[^14]

***

#### Type Comments (PEP 484)

Python 2.7 has no annotation syntax, but PEP 484 defines an equivalent
comment-based form that mypy, PyCharm, and other tools understand. Install
the backport with `pip install typing` (pip on 2.7 resolves a compatible
release automatically).

##### Basic Type Comments

```python
from typing import Dict, List, Optional, Text

# Variables
name = 'John'        # type: Text
age = 30             # type: int
scores = [90, 85]    # type: List[int]

# Functions: the comment goes on the line after the header, before the
# docstring. Use Text for unicode-or-str parameters; checkers treat int
# and long as equivalent on Python 2.
def greet(name):
    # type: (Text) -> Text
    return 'Hello, {0}!'.format(name)

def find_user(user_id):
    # type: (int) -> Optional[User]
    """Returns User or None if not found."""
    pass

def process_items(items):
    # type: (List[Text]) -> Dict[Text, int]
    """Returns count of each item."""
    pass

# Long signatures: one comment per argument, then the return type
def send_email(address,        # type: Text
               subject,        # type: Text
               attachments,    # type: List[bytes]
               ):
    # type: (...) -> bool
    pass
```

##### Advanced Type Comments

```python
from typing import Any, Callable, Generic, Iterable, Optional, TypeVar

# TypeVar for generics
T = TypeVar('T')

def first(items):
    # type: (List[T]) -> Optional[T]
    return items[0] if items else None

# Callable values
def apply_all(handlers, event):
    # type: (Iterable[Callable[[Any], None]], Any) -> None
    for handler in handlers:
        handler(event)

# Generic classes
class Repository(Generic[T]):
    def get(self, key):
        # type: (int) -> Optional[T]
        pass

# Not available for Python 2.7: Literal, Final, Self, ParamSpec, Protocol,
# and the builtin-generic (list[int]) and X | None union syntax. Spell
# unions as Union[...] / Optional[...]. TypedDict-style shapes are
# available from mypy_extensions if needed.
```

***

#### Docstrings

With no inline annotations, docstrings carry the types too — the
`Args`/`Returns` sections are the contract readers see first. Use Google
style consistently.

##### Function Docstrings

```python
def calculate_discount(price, discount_percent, min_price=0.0):
    # type: (float, float, float) -> float
    """Calculate the discounted price.

    Args:
        price (float): Original price of the item.
        discount_percent (float): Discount percentage (0-100).
        min_price (float): Minimum price floor. Defaults to 0.0.

    Returns:
        float: The discounted price, not less than min_price.

    Raises:
        ValueError: If discount_percent is not between 0 and 100.

    Example:
        >>> calculate_discount(100.0, 20.0)
        80.0
    """
    if not 0 <= discount_percent <= 100:
        raise ValueError('Discount must be between 0 and 100')

    discounted = price * (1 - discount_percent / 100.0)
    return max(discounted, min_price)
```

##### Class Docstrings

```python
class UserService(object):
    """Service for managing user operations.

    This service handles user CRUD operations and authentication.
    It requires a database connection and optional cache.

    Attributes:
        db (DatabaseConnection): Database connection instance.
        cache (Cache): Optional cache for user lookups, or None.

    Example:
        >>> service = UserService(db_connection)
        >>> user = service.get_user(123)
    """

    def __init__(self, db, cache=None):
        # type: (DatabaseConnection, Optional[Cache]) -> None
        """Initialize the UserService.

        Args:
            db (DatabaseConnection): Active database connection.
            cache (Cache): Optional cache instance for performance.
        """
        self.db = db
        self.cache = cache
```

***

#### Generators and Iterators

##### The Iterator Protocol

An iterator is any object with a `__iter__()` method (returning itself) and a `next()` method (returning the next value or raising `StopIteration`).[^5]

```python
class CountDown(object):
    def __init__(self, n):
        self.n = n
    def __iter__(self):
        return self
    def next(self):
        if self.n <= 0:
            raise StopIteration
        val = self.n
        self.n -= 1
        return val

for x in CountDown(3):
    print x   # 3, 2, 1
```

##### Generator Functions

A function containing `yield` is a generator function. Calling it returns a generator object (a lazy iterator). State is preserved between calls.

```python
def fibonacci():
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

gen = fibonacci()
print [next(gen) for _ in xrange(10)]  # [0, 1, 1, 2, 3, 5, 8, 13, 21, 34]
```

```python
def read_lines_lazily(filename):
    """Read a file line by line, yielding each line lazily."""
    import io
    with io.open(filename, 'r', encoding='utf-8') as f:
        for line in f:
            yield line.rstrip('\n')
```

##### Generator Expressions

Generator expressions are comprehension-like syntax producing a generator (not a list). They are memory-efficient for large or infinite sequences.[^9][^15]

```python
# List comprehension — builds entire list in memory
total = sum([x**2 for x in xrange(1000000)])

# Generator expression — computes on demand, O(1) memory
total = sum(x**2 for x in xrange(1000000))

# The outer parentheses can be omitted when the genexp is the sole argument:
total = sum(x**2 for x in xrange(1000000))   # same as above
```

##### `send()` and Two-Way Generators

Generators can receive values via `.send()`, enabling coroutine-like patterns:

```python
def accumulator():
    total = 0
    while True:
        value = yield total
        if value is None:
            break
        total += value

acc = accumulator()
next(acc)       # advance to first yield
acc.send(10)    # total = 10
acc.send(20)    # total = 30
print acc.send(5)  # 35
```

***

#### `itertools` — Efficient Iteration

`itertools` provides memory-efficient building blocks for iterators:[^16]

```python
import itertools

# Infinite iterators
itertools.count(10)           # 10, 11, 12, ...
itertools.cycle('ABC')        # A, B, C, A, B, C, ...
itertools.repeat(42, 3)       # 42, 42, 42

# Finite iterators
list(itertools.chain([1,2], [3,4], [5]))  # [1, 2, 3, 4, 5]
list(itertools.islice(itertools.count(), 5))  # [0, 1, 2, 3, 4]
list(itertools.compress('ABCDEF', [1,0,1,0,1,0]))  # ['A', 'C', 'E']
list(itertools.dropwhile(lambda x: x < 5, [1,4,6,4,1]))  # [6, 4, 1]
list(itertools.takewhile(lambda x: x < 5, [1,4,6,4,1]))  # [1, 4]
list(itertools.ifilterfalse(lambda x: x%2, range(10)))   # [0, 2, 4, 6, 8]

# Combinatorics
list(itertools.product('AB', repeat=2))     # [('A','A'),('A','B'),('B','A'),('B','B')]
list(itertools.permutations('ABC', 2))      # 6 items
list(itertools.combinations('ABC', 2))      # [('A','B'),('A','C'),('B','C')]

# Grouping (input must be sorted on the key)
data = sorted([('A',1), ('B',2), ('A',3)], key=lambda x: x[0])
for key, group in itertools.groupby(data, key=lambda x: x[0]):
    print key, list(group)

# Lazy zip
for a, b in itertools.izip([1,2,3], [4,5,6]):  # no list copy; use in hot loops
    print a + b
```

***

#### Classes and Object-Oriented Programming

##### New-Style vs Old-Style Classes

**Always use new-style classes** in Python 2.7: inherit explicitly from `object`.[^17][^18]

```python
# OLD-STYLE — DO NOT USE in Python 2.7
class OldFoo:
    pass

# NEW-STYLE — always do this in Python 2.7
class NewFoo(object):
    pass
```

Old-style classes have a completely different and broken MRO (method resolution order), do not support `super()`, `property`, `classmethod`, `staticmethod`, descriptors, or `__slots__` correctly.[^17][^19]

##### Class Definition

```python
class Animal(object):
    """Base class for all animals."""

    # Class attribute — shared by all instances
    kingdom = 'Animalia'

    def __init__(self, name, species):
        # Instance attributes — unique per instance
        self.name = name
        self.species = species
        self._energy = 100    # convention: single _ means "internal use"
        self.__secret = 'x'   # double __ triggers name mangling -> _Animal__secret

    def eat(self, food):
        """Consume food and gain energy."""
        self._energy += 10
        return '{0} eats {1}'.format(self.name, food)

    def __repr__(self):
        return 'Animal(name={0!r}, species={1!r})'.format(self.name, self.species)

    def __str__(self):
        return '{0} ({1})'.format(self.name, self.species)

    def __eq__(self, other):
        if not isinstance(other, Animal):
            return NotImplemented
        return self.name == other.name and self.species == other.species

    def __hash__(self):
        # When defining __eq__, also define __hash__ or set it to None
        return hash((self.name, self.species))
```

**Pitfall: Mutable class attributes.** If a class attribute is a mutable object (list, dict), all instances share the same object. Mutations affect all instances:

```python
class Bad(object):
    items = []   # shared mutable class attribute

class Good(object):
    def __init__(self):
        self.items = []  # instance attribute — each instance gets its own list
```

##### Inheritance and `super()`

```python
class Dog(Animal):
    def __init__(self, name, breed):
        # CORRECT — always use super() with new-style classes
        super(Dog, self).__init__(name, 'Canis lupus familiaris')
        self.breed = breed

    def speak(self):
        return '{0} says: Woof!'.format(self.name)

    def __repr__(self):
        return 'Dog(name={0!r}, breed={1!r})'.format(self.name, self.breed)
```

**MRO — Method Resolution Order.** For new-style classes Python uses C3 linearization (also called MRO). Use `ClassName.__mro__` or `ClassName.mro()` to inspect it:

```python
class A(object): pass
class B(A): pass
class C(A): pass
class D(B, C): pass
print D.__mro__  # [D, B, C, A, object]
```

##### Properties

`property` descriptors provide controlled attribute access without breaking the uniform access principle:

```python
class Circle(object):
    def __init__(self, radius):
        self._radius = radius

    @property
    def radius(self):
        """The circle's radius (must be positive)."""
        return self._radius

    @radius.setter
    def radius(self, value):
        if value < 0:
            raise ValueError('Radius must be non-negative, got {0}'.format(value))
        self._radius = value

    @radius.deleter
    def radius(self):
        del self._radius

    @property
    def area(self):
        import math
        return math.pi * self._radius ** 2

c = Circle(5)
print c.radius  # 5
c.radius = 10   # calls setter
print c.area    # 314.159...
```

##### `classmethod` and `staticmethod`

```python
class Config(object):
    _instance = None

    def __init__(self, data):
        self.data = data

    @classmethod
    def from_file(cls, path):
        """Alternative constructor — receives the class, not an instance."""
        with open(path) as f:
            import json
            return cls(json.load(f))

    @staticmethod
    def validate_key(key):
        """Utility function scoped to the class; receives neither cls nor self."""
        # basestring matches both str and unicode — isinstance(key, str)
        # would reject unicode keys (all literals under unicode_literals)
        return isinstance(key, basestring) and len(key) > 0
```

##### `__slots__`

`__slots__` restricts attribute creation to a fixed set, eliminating the per-instance `__dict__`. This reduces memory usage significantly for large numbers of small objects:[^20]

```python
class Point(object):
    __slots__ = ('x', 'y')

    def __init__(self, x, y):
        self.x = x
        self.y = y

# Attempting to add undeclared attributes raises AttributeError:
p = Point(1, 2)
p.z = 3   # AttributeError: 'Point' object has no attribute 'z'
```

**Caveats:** Classes with `__slots__` cannot be weakly referenced unless `'__weakref__'` is explicitly listed in `__slots__`. Subclasses must also declare `__slots__` (without re-declaring parent slots) to gain the memory benefit.[^20][^21]

##### Important Dunder (Magic) Methods

| Method | Purpose |
|---|---|
| `__init__(self, ...)` | Instance initializer |
| `__repr__(self)` | Unambiguous string representation (for debugging) |
| `__str__(self)` | Readable string representation (for display) |
| `__len__(self)` | Length; makes `len(obj)` work |
| `__getitem__(self, key)` | Index/key access: `obj[key]` |
| `__setitem__(self, key, val)` | Index assignment: `obj[key] = val` |
| `__delitem__(self, key)` | Index deletion: `del obj[key]` |
| `__contains__(self, item)` | Membership: `item in obj` |
| `__iter__(self)` | Return iterator |
| `__next__` / `next` | Next value from iterator (Python 2 uses `next`) |
| `__eq__(self, other)` | Equality: `obj == other` |
| `__ne__(self, other)` | Inequality: `obj != other` |
| `__lt__`, `__le__`, `__gt__`, `__ge__` | Rich comparisons |
| `__hash__(self)` | Hash for use in sets/dict keys |
| `__call__(self, ...)` | Make instance callable |
| `__enter__`, `__exit__` | Context manager protocol |
| `__del__(self)` | Finalizer (called by GC; do not rely on timing) |
| `__copy__`, `__deepcopy__` | Custom copy behavior |
| `__nonzero__` | Python 2 bool cast (Python 3: `__bool__`) |

**Pitfall:** In Python 2.7, the boolean truth value method is `__nonzero__`, not `__bool__`. Define `__nonzero__` on your objects.

**Pitfall:** If you define `__eq__`, Python sets `__hash__ = None` implicitly for new-style classes that don't also define `__hash__`. This makes the class unhashable. Always define both, or explicitly set `__hash__ = None` if unhashability is intentional.

##### Abstract Base Classes

```python
from abc import ABCMeta, abstractmethod

class Shape(object):
    __metaclass__ = ABCMeta

    @abstractmethod
    def area(self):
        """Return the area of the shape."""

    @abstractmethod
    def perimeter(self):
        """Return the perimeter of the shape."""

class Rectangle(Shape):
    def __init__(self, w, h):
        self.w, self.h = w, h
    def area(self):
        return self.w * self.h
    def perimeter(self):
        return 2 * (self.w + self.h)

# Shape() raises TypeError — cannot instantiate abstract class
# Rectangle() is fine because it implements all abstract methods
```

***

#### Declarative Classes: attrs

`dataclasses` requires Python 3.7. On 2.7 the same declarative style comes
from `attrs` (pin `attrs<=21.4.0` — the last line that runs on Python 2);
for simple immutable records, stdlib `collections.namedtuple` (see Tuples)
is enough.

```python
import datetime
import attr

@attr.s
class User(object):
    id = attr.ib()                            # type: int
    name = attr.ib()                          # type: str
    email = attr.ib(converter=lambda v: v.lower())
    active = attr.ib(default=True)            # type: bool
    created_at = attr.ib(factory=datetime.datetime.now)
    tags = attr.ib(factory=list)              # type: list

@attr.s(frozen=True)
class Point(object):
    """Immutable point — hashable, usable as dict key."""
    x = attr.ib()  # type: float
    y = attr.ib()  # type: float

    def distance_to(self, other):
        # type: (Point) -> float
        return ((self.x - other.x) ** 2 + (self.y - other.y) ** 2) ** 0.5
```

`attr.s` generates `__init__`, `__repr__`, `__eq__`, and (with
`frozen=True`) `__hash__` — the same boilerplate this guide otherwise
writes by hand, without the shared-mutable-default pitfalls.

***

#### Decorators

Decorators wrap a function or class, replacing it with the wrapper. The `@syntax` is syntactic sugar for `func = decorator(func)`.

```python
import functools
import time

def timer(func):
    """Decorator: measure and print execution time."""
    @functools.wraps(func)   # preserves __name__, __doc__, etc.
    def wrapper(*args, **kwargs):
        start = time.time()
        result = func(*args, **kwargs)
        elapsed = time.time() - start
        print '{0} took {1:.4f}s'.format(func.__name__, elapsed)
        return result
    return wrapper

@timer
def slow_function(n):
    """Compute the sum of 0..n."""
    return sum(xrange(n))
```

**Decorator factories (decorators with arguments):**

```python
import logging
import sys

def retry(max_attempts=3, exceptions=(Exception,)):
    """Retry a function up to max_attempts times on specified exceptions."""
    def decorator(func):
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            exc_info = None
            for attempt in xrange(max_attempts):
                try:
                    return func(*args, **kwargs)
                except exceptions as e:
                    # keep type/value/traceback — a later "raise e" would
                    # point the traceback here instead of at the failure
                    exc_info = sys.exc_info()
                    logging.warning('Attempt %d/%d for %s failed: %s',
                                    attempt + 1, max_attempts, func.__name__, e)
            # three-argument raise re-raises with the ORIGINAL traceback
            # (Python-2-only syntax; six.reraise(*exc_info) works on 2 and 3)
            raise exc_info[0], exc_info[1], exc_info[2]
        return wrapper
    return decorator

@retry(max_attempts=5, exceptions=(IOError, OSError))
def read_remote_file(url):
    pass
```

**`functools.wraps` is mandatory.** Without it, the wrapped function loses its `__name__`, `__doc__`, and other metadata, breaking introspection and logging.

***

#### Exception Handling

##### Syntax

Python 2.7 supports both the old comma syntax and the new `as` keyword. Always use `as`:[^22]

```python
# DEPRECATED — old syntax; works in 2.7 but not in Python 3
try:
    risky()
except ValueError, e:   # DO NOT USE
    handle(e)

# CORRECT — works in both 2.7 and Python 3
try:
    risky()
except ValueError as e:
    handle(e)
```

##### Exception Hierarchy

```python
BaseException
 ├── SystemExit
 ├── KeyboardInterrupt
 ├── GeneratorExit
 └── Exception
      ├── ArithmeticError
      │    ├── ZeroDivisionError
      │    ├── OverflowError
      │    └── FloatingPointError
      ├── LookupError
      │    ├── IndexError
      │    └── KeyError
      ├── ValueError
      ├── TypeError
      ├── IOError / OSError (largely synonymous in 2.7)
      ├── AttributeError
      ├── ImportError
      ├── RuntimeError
      ├── StopIteration
      ├── UnicodeError
      │    ├── UnicodeDecodeError
      │    └── UnicodeEncodeError
      └── ...
```

**Pitfall: Never catch `BaseException` or bare `except:` in production** unless you are at the topmost level of a main loop that must guarantee cleanup. Catching `BaseException` also catches `KeyboardInterrupt` and `SystemExit`, preventing clean shutdown.[^23]

```python
# BAD — swallows everything including Ctrl-C and sys.exit()
try:
    do_work()
except:
    pass

# ALSO BAD — too broad
try:
    do_work()
except Exception:
    pass  # ignores the error

# GOOD — catch only what you can handle
try:
    result = int(user_input)
except ValueError as e:
    logger.warning('Invalid input %r: %s', user_input, e)
    result = default_value
```

##### Full `try/except/else/finally`

```python
def read_config(path):
    """Read and parse a JSON config file."""
    import io
    import json
    try:
        with io.open(path, 'r', encoding='utf-8') as f:
            config = json.load(f)   # IOError on read, ValueError on bad JSON
    except IOError as e:
        logger.error('Cannot open config %r: %s', path, e)
        raise
    except ValueError as e:
        logger.error('Malformed config %r: %s', path, e)
        raise
    else:
        # only executes if no exception was raised. Careful: an exception
        # raised HERE is NOT caught by the except clauses above — keep
        # anything that can fail inside the try block.
        return config
    finally:
        # always executes, even when an exception propagates — for actions
        # needed on success AND failure (the file itself is already closed
        # by the with statement)
        logger.debug('read_config finished for %r', path)
```

The `else` clause of `try` is underused but important: it separates "the protected code succeeded" logic from "clean up regardless" logic in `finally`.

##### Custom Exceptions

```python
class AppError(Exception):
    """Base class for application-specific exceptions."""

class ConfigError(AppError):
    """Raised when configuration is invalid or missing."""
    def __init__(self, key, message=''):
        self.key = key
        msg = 'Config error for key {0!r}'.format(key)
        if message:
            msg += ': ' + message
        super(ConfigError, self).__init__(msg)

class NetworkError(AppError):
    """Raised on network-related failures."""
    def __init__(self, url, status_code=None):
        self.url = url
        self.status_code = status_code
        msg = 'Network error for {0!r}'.format(url)
        if status_code:
            msg += ' (HTTP {0})'.format(status_code)
        super(NetworkError, self).__init__(msg)

# Usage
try:
    connect(url)
except NetworkError as e:
    logger.error('Failed to connect: %s', e)
    if e.status_code == 404:
        raise ConfigError('endpoint_url', 'endpoint not found')
    raise
```

**Always inherit custom exceptions from `Exception` (or a subclass), never from `BaseException` directly.**

##### Re-raising Exceptions

```python
try:
    do_work()
except SomeError as e:
    logger.error('Error: %s', e, exc_info=True)  # log with traceback
    raise   # re-raise the original exception with original traceback
```

Using bare `raise` preserves the original traceback. Do NOT do `raise e` — that resets the traceback to the current line.[^24]

##### Exception Chaining

`raise NewError(...) from original` is Python-3-only syntax — Python 2.7
has no exception chaining. To keep the root cause diagnosable, log the
original exception with `exc_info=True` before raising the replacement:

```python
try:
    parse(config_text)
except ValueError:
    logger.error('Failed to parse config', exc_info=True)
    raise ConfigError('config', 'unparseable')
```

In code that must also run on Python 3, `six.raise_from(new, original)`
gives proper chaining there — but on a Python 2 interpreter it is
equivalent to a plain `raise new` and preserves nothing by itself, so
keep the `exc_info=True` logging either way.

***

#### Context Managers and the `with` Statement

The `with` statement ensures cleanup code runs even when exceptions occur.[^25] It was added in Python 2.5.

```python
# File handling — the canonical use case
with open('data.txt', 'r') as f:
    content = f.read()
# f is automatically closed here, even if an exception was raised

# Multiple context managers on one line (Python 2.7+)
with open('input.txt') as fin, open('output.txt', 'w') as fout:
    for line in fin:
        fout.write(line.upper())
```

##### Writing a Context Manager Class

```python
class DatabaseTransaction(object):
    def __init__(self, connection):
        self.conn = connection

    def __enter__(self):
        self.conn.begin()
        return self.conn   # value bound to 'as' target

    def __exit__(self, exc_type, exc_val, exc_tb):
        if exc_type is None:
            self.conn.commit()
        else:
            self.conn.rollback()
        return False  # False: do not suppress the exception

with DatabaseTransaction(db_conn) as conn:
    conn.execute('UPDATE ...')
```

**Return value of `__exit__`:** Return `True` to suppress the exception (pretend it never happened). Return `False` (or `None`) to let the exception propagate. Suppressing exceptions should be rare and deliberate.[^26]

##### `contextlib.contextmanager`

Use this decorator to write context managers as generators, which is often cleaner:[^27]

```python
from contextlib import contextmanager
import logging

@contextmanager
def logged_operation(name):
    logging.info('Starting: %s', name)
    try:
        yield
    except Exception as e:
        logging.error('Failed: %s — %s', name, e)
        raise
    else:
        logging.info('Completed: %s', name)

with logged_operation('data import'):
    import_data()
```

***

#### File I/O

##### Basic File Operations

```python
# Reading text
with open('file.txt', 'r') as f:
    content = f.read()         # entire file as str
    lines = content.splitlines()

# Iterate lines without loading the entire file
with open('large_file.txt', 'r') as f:
    for line in f:             # lazy iteration — O(1) memory
        process(line.rstrip('\n'))

# Writing text
with open('out.txt', 'w') as f:    # 'w' truncates; 'a' appends
    f.write('line 1\n')
    f.writelines(['line 2\n', 'line 3\n'])

# Binary I/O — always use 'b' mode for binary data
with open('image.png', 'rb') as f:
    data = f.read()

with open('copy.png', 'wb') as f:
    f.write(data)
```

##### Unicode-Safe File I/O

Python 2.7's built-in `open()` reads and writes `str` (bytes). For Unicode text files, use `io.open()` which handles encoding automatically:

```python
import io

# Read UTF-8 text file as unicode
with io.open('utf8_file.txt', 'r', encoding='utf-8') as f:
    text = f.read()   # text is unicode

# Write unicode to UTF-8 file
with io.open('output.txt', 'w', encoding='utf-8') as f:
    f.write(u'Caf\xe9\n')
```

**Recommendation:** Always use `io.open` for text files in production. It works identically in Python 2.7 and Python 3, and it prevents accidental binary/text mixing.[^6]

##### File Positions and Random Access

```python
with open('data.bin', 'rb') as f:
    f.seek(100)             # move to byte 100
    chunk = f.read(50)      # read 50 bytes
    pos = f.tell()          # current position
    f.seek(0, 2)            # seek to end (whence=2)
    size = f.tell()         # file size
    f.seek(0)               # rewind to beginning
```

##### `os` and `os.path` for File System Operations

```python
import os
import os.path

# Path manipulation
path = os.path.join('/home', 'alice', 'data.csv')
print os.path.dirname(path)    # '/home/alice'
print os.path.basename(path)   # 'data.csv'
name, ext = os.path.splitext('data.csv')   # ('data', '.csv')
print os.path.abspath('relative/path')
print os.path.expanduser('~/config')

# File tests
print os.path.exists(path)
print os.path.isfile(path)
print os.path.isdir('/home/alice')

# Directory operations
os.makedirs('/new/dir/structure')  # creates all intermediate dirs
os.listdir('.')                    # returns list of filenames
for dirpath, dirnames, filenames in os.walk('/some/dir'):
    for fname in filenames:
        full = os.path.join(dirpath, fname)

# File operations
os.rename('old.txt', 'new.txt')
os.remove('to_delete.txt')
os.rmdir('empty_dir')
import shutil
shutil.rmtree('dir_with_contents')  # recursive delete
shutil.copy2('src', 'dst')          # copy with metadata
```

##### Temporary Files

```python
import tempfile

# Secure temporary file (automatically deleted on close)
with tempfile.NamedTemporaryFile(suffix='.csv', delete=True) as tmp:
    tmp.write(b'col1,col2\n')
    tmp.flush()
    process(tmp.name)
# file is deleted here

# Temporary directory — mkdtemp() returns a plain path string, NOT a
# context manager; clean it up yourself
import shutil
tmpdir = tempfile.mkdtemp()
try:
    work_in(tmpdir)
finally:
    shutil.rmtree(tmpdir)
```

**Do NOT use `tempfile.mktemp()`** — it is deprecated and vulnerable to race conditions (TOCTOU: the file name is returned before the file is created, allowing another process to create a file with the same name in between).[^28]

***

#### Modules and Packages

##### Import System

```python
import os                      # import module
import os.path                 # import submodule
from os import path, getcwd    # import specific names
from os.path import join, exists  # import specific names from submodule
import numpy as np             # alias (common convention)
from mypackage import mymodule
```

**Absolute vs relative imports.** In Python 2.7 without `from __future__ import absolute_import`, `import foo` can resolve to a sibling module rather than an installed package. This causes subtle shadowing bugs. Always use:

```python
from __future__ import absolute_import

# Now relative imports require explicit syntax:
from . import sibling_module        # same package
from .. import parent_module        # parent package
from .utils import helper_function
```

##### Module Search Path

Python resolves imports by searching `sys.path` in order:
1. The directory containing the script (or `''` for interactive)
2. Directories listed in `PYTHONPATH` environment variable
3. Installation-dependent default paths

Manipulating `sys.path` at runtime is an antipattern for production code. Use proper packaging instead.

##### `__all__`

Define `__all__` in every module to control what `from module import *` exposes, and to clearly document public API:

```python
__all__ = ['PublicClass', 'public_function', 'CONSTANT']

CONSTANT = 42

def public_function():
    pass

def _private_helper():   # not exported
    pass

class PublicClass(object):
    pass
```

##### `if __name__ == '__main__':`

```python
def main():
    """Entry point for the script."""
    import argparse
    parser = argparse.ArgumentParser(description='My tool')
    parser.add_argument('input', help='Input file')
    args = parser.parse_args()
    process(args.input)

if __name__ == '__main__':
    main()
```

This guard ensures that `main()` is only called when the file is run directly, not when it is imported as a module.

***

#### The `collections` Module

Python 2.7's `collections` module provides specialized container types:

```python
from collections import (
    OrderedDict,   # dict that remembers insertion order
    defaultdict,   # dict with a default factory
    Counter,       # multiset / frequency counter
    deque,         # double-ended queue; O(1) append/pop at both ends
    namedtuple,    # tuple with named fields
)

# deque — prefer over list for queues
from collections import deque
q = deque(maxlen=100)  # fixed-size circular buffer
q.appendleft(item)
q.append(item)
q.popleft()
q.pop()
```

`deque.appendleft` and `deque.popleft` are O(1). `list.insert(0, item)` is O(n). Use `deque` whenever you need FIFO or double-ended access.[^16]

***

#### Division

The `/` operator in Python 2.7 performs **integer division** when both operands are integers, discarding the remainder.[^29][^30]

```python
print 7 / 2      # 3  — integer division (UNEXPECTED if you wanted 3.5)
print 7 / 2.0    # 3.5 — float division
print 7.0 / 2    # 3.5
print 7 // 2     # 3  — explicit floor division (works in Python 2 and 3)
print 7 % 2      # 1  — modulo
```

**Production recommendation:** Always add `from __future__ import division` at the top of every module. This makes `/` always perform true division (like Python 3), and `//` always perform floor division.[^3][^31]

```python
from __future__ import division
print 7 / 2    # 3.5
print 7 // 2   # 3
```

***

#### The `print` Statement

In Python 2.7, `print` is a statement with special syntax. For production code, always use the print function via `from __future__ import print_function`.[^4]

```python
from __future__ import print_function

print('Hello, World!')
print('a', 'b', 'c', sep=', ')       # 'a, b, c'
print('loading...', end='')           # no trailing newline
print('error!', file=sys.stderr)      # write to stderr
```

Without the import, `print` is a statement:

```python
# Statement syntax (do NOT use in new production code)
print 'Hello'
print 'a', 'b'     # prints 'a b'
print >> sys.stderr, 'error'
```

**Pitfall:** `print(a, b)` in Python 2 without the `__future__` import prints the tuple `(a, b)` if `a` and `b` are separate arguments — it is NOT the same as calling a function with two arguments.[^4]

***

#### Input and Output

```python
# Input from stdin
name = raw_input('Enter name: ')   # returns str (bytes)
# input() in Python 2 evaluates the input as Python code — DANGEROUS
# Never use input() for user data in Python 2.7

# Safer input with type conversion
try:
    age = int(raw_input('Enter age: '))
except ValueError:
    print 'Invalid age'
```

***

#### Logging

Use the `logging` module — never `print` — for production observability.[^32]

```python
import logging

# Module-level logger (canonical pattern)
logger = logging.getLogger(__name__)

# Application entry point — configure once
def setup_logging(level=logging.INFO):
    logging.basicConfig(
        level=level,
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        datefmt='%Y-%m-%dT%H:%M:%S',
    )

# Usage in modules
logger.debug('Processing item: %r', item)    # use % formatting, not .format()
logger.info('Loaded %d records', count)
logger.warning('Config key %r missing, using default', key)
logger.error('Failed to connect: %s', exc)
logger.critical('Unrecoverable state, shutting down')

# Log exceptions with traceback
try:
    risky()
except Exception as e:
    logger.exception('Unexpected error in risky()')  # includes traceback automatically
    # or
    logger.error('Error: %s', e, exc_info=True)
```

**Critical rules:**[^32][^33]
- Use `logging.getLogger(__name__)` in each module — never pass loggers as function arguments.
- Use `%` style format strings in logging calls, not `str.format()` or f-strings. The logging system defers formatting until the message actually needs to be emitted, avoiding the cost of string formatting for suppressed log levels.
- Configure handlers and levels only in the application entry point, never in library modules.
- Never call `logging.basicConfig()` in a module that will be imported.

***

#### Regular Expressions

```python
import re

# Compile for reuse — avoids recompiling on every call
EMAIL_RE = re.compile(
    r'^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$'
)

# Match functions
m = EMAIL_RE.match(email)       # anchored at start
m = re.search(r'\d+', text)     # anywhere in string
matches = re.findall(r'\d+', text)  # all non-overlapping matches
parts = re.split(r'\s+', text)  # split on whitespace
cleaned = re.sub(r'<[^>]+>', '', html)  # replace HTML tags

if m:
    print m.group(0)   # full match
    print m.group(1)   # first capture group
    print m.start(), m.end()
```

**Performance:** Always compile regex patterns used in loops or called frequently. Compiled pattern objects cache the compiled bytecode.

**Pitfall: Greedy vs non-greedy matching.**

```python
html = '<b>bold</b> and <i>italic</i>'
# Greedy (default) — matches as much as possible
re.findall(r'<.+>', html)    # ['<b>bold</b> and <i>italic</i>']
# Non-greedy — matches as little as possible
re.findall(r'<.+?>', html)   # ['<b>', '</b>', '<i>', '</i>']
```

***

#### Subprocess and Shell Execution

**Never use `os.system()` with user-provided input.** It passes the command to the system shell, enabling OS command injection.[^34][^35]

```python
import subprocess
import shlex

# SAFE — list form bypasses the shell entirely
result = subprocess.check_output(['ls', '-la', '/tmp'], stderr=subprocess.STDOUT)

# SAFE — when you must use shell features, but NEVER interpolate user input
result = subprocess.check_output('echo $HOME', shell=True)

# DANGEROUS — user input in shell string
# subprocess.call('ls ' + user_dir, shell=True)  # DO NOT DO THIS

# Full pattern with return code checking
proc = subprocess.Popen(
    ['git', 'log', '--oneline', '-10'],
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
)
stdout, stderr = proc.communicate()
if proc.returncode != 0:
    raise RuntimeError('git failed: {0}'.format(stderr))
```

**`check_call` and `check_output`** raise `CalledProcessError` on non-zero exit. Use them instead of manually checking `returncode`:

```python
try:
    output = subprocess.check_output(['grep', pattern, filename])
except subprocess.CalledProcessError as e:
    if e.returncode == 1:
        output = b''  # grep returns 1 when no match found
    else:
        raise
```

***

#### Concurrency

##### The GIL

CPython has a Global Interpreter Lock (GIL) — only one thread executes Python bytecode at a time. This means:
- `threading` does NOT give you true CPU parallelism for CPU-bound Python code.
- `threading` IS effective for I/O-bound work (network, file I/O), because threads can release the GIL while waiting.
- For CPU-bound parallelism, use `multiprocessing`.

##### `threading`

```python
import threading

def worker(task_id, results, lock):
    result = do_work(task_id)
    with lock:
        results.append(result)

lock = threading.Lock()
results = []
threads = []

for i in xrange(10):
    t = threading.Thread(target=worker, args=(i, results, lock))
    t.daemon = True  # thread dies when main thread exits
    t.start()
    threads.append(t)

for t in threads:
    t.join()
```

**Synchronization primitives:**

```python
lock = threading.Lock()          # mutual exclusion
rlock = threading.RLock()        # reentrant lock (same thread can acquire multiple times)
event = threading.Event()        # one-shot signal
sem = threading.Semaphore(5)     # limit concurrent access
cond = threading.Condition(lock) # wait/notify pattern

import Queue
q = Queue.Queue(maxsize=100)     # thread-safe queue
q.put(item)
item = q.get()
q.task_done()
q.join()   # block until all items are processed
```

**Pitfall:** Do not share mutable state across threads without proper synchronization. Even `+=` on an integer is not atomic in Python.[^36]

##### `multiprocessing`

For CPU-bound work, bypass the GIL with separate processes:

```python
from multiprocessing import Pool, cpu_count

def process_chunk(chunk):
    return [expensive_transform(item) for item in chunk]

if __name__ == '__main__':   # REQUIRED on Windows — prevents recursive spawning
    data = load_large_dataset()
    chunks = [data[i::cpu_count()] for i in xrange(cpu_count())]

    # Pool is NOT a context manager in Python 2.7 (that arrived in 3.3)
    pool = Pool(processes=cpu_count())
    try:
        results = pool.map(process_chunk, chunks)
    finally:
        pool.close()
        pool.join()

    flat = [item for sublist in results for item in sublist]
```

***

#### Memory Management

CPython uses reference counting as its primary memory management mechanism. When an object's reference count drops to zero, its memory is reclaimed immediately.[^37]

##### Reference Counting

```python
import sys
x = [1, 2, 3]
print sys.getrefcount(x)  # 2 — one for 'x', one for the getrefcount argument

y = x            # refcount += 1
del y            # refcount -= 1
x = None         # refcount -= 1 -> 0 -> freed
```

##### Cyclic Garbage Collector

Reference counting cannot free objects involved in reference cycles (A refers to B, B refers to A). CPython has a supplemental cyclic garbage collector for this. Use the `gc` module to control it:

```python
import gc

gc.collect()                          # manually trigger a full collection
gc.set_threshold(700, 10, 10)         # adjust generation thresholds
gc.disable()                          # disable automatic GC (use with care)
print gc.get_count()                  # (gen0, gen1, gen2) object counts
```

**Performance tip:** For applications that allocate many short-lived container objects, increasing the generation 0 threshold (e.g., to 50,000 from 700) can significantly reduce GC overhead.[^38]

##### Memory Optimization Patterns

```python
# Use __slots__ for classes with many instances (see OOP section)

# Use generators instead of lists for pipelines
total = sum(x**2 for x in xrange(1000000))  # never builds the list

# Use array module for large arrays of homogeneous primitives
import array
int_array = array.array('i', [1, 2, 3, 4, 5])  # much less memory than list

# Use weakref for caches that should not prevent GC
import weakref
cache = weakref.WeakValueDictionary()
```

***

#### JSON, CSV, and Data Serialization

##### JSON

```python
import json

# Serialize to string
data = {'name': u'Alice', 'age': 30, 'scores': [95, 87, 92]}
json_str = json.dumps(data, indent=2, sort_keys=True)

# Deserialize from string
restored = json.loads(json_str)

# File I/O
with open('data.json', 'w') as f:
    json.dump(data, f, indent=2)

with open('data.json', 'r') as f:
    restored = json.load(f)
```

**Pitfall:** JSON keys are always Unicode strings. After `json.loads()`, all string keys and values are `unicode` objects, not `str`. This matters when interfacing with APIs that expect `str` keys.

**Security:** Never use `json.loads()` on untrusted input that could contain deeply nested structures — deeply nested JSON can cause a Python stack overflow via recursion.

##### Pickle — Use with Extreme Caution

```python
import pickle

# Serialize
data = {'key': [1, 2, 3]}
with open('data.pkl', 'wb') as f:
    pickle.dump(data, f, protocol=2)  # protocol=2 is Python 2 compatible

# Deserialize
with open('data.pkl', 'rb') as f:
    restored = pickle.load(f)
```

**SECURITY WARNING:** **Never unpickle data from untrusted sources.** Pickle can execute arbitrary Python code on deserialization. A maliciously crafted pickle file can execute shell commands, read files, or install backdoors.[^28] For inter-system communication, use JSON, protobuf, or msgpack instead.

##### CSV

```python
import csv

# Reading
with open('data.csv', 'rb') as f:  # 'rb' required on Python 2 Windows
    reader = csv.DictReader(f)
    for row in reader:
        process(row)

# Writing
fieldnames = ['name', 'age', 'city']
with open('output.csv', 'wb') as f:
    writer = csv.DictWriter(f, fieldnames=fieldnames)
    writer.writeheader()
    writer.writerow({'name': 'Alice', 'age': 30, 'city': 'Berlin'})
```

**Python 2.7 CSV caveat:** The `csv` module does not support Unicode directly. You must encode Unicode to UTF-8 bytes before writing and decode on read. Use a wrapper:

```python
import csv
import io

def unicode_csv_reader(unicode_csv_data, **kwargs):
    encoded = (row.encode('utf-8') for row in unicode_csv_data)
    for row in csv.reader(encoded, **kwargs):
        yield [cell.decode('utf-8') for cell in row]
```

***

#### Secure Coding Patterns

##### Input Validation

```python
def validate_age(value):
    """Validate and coerce age from user input."""
    try:
        age = int(value)
    except (ValueError, TypeError):
        raise ValueError('Age must be an integer, got {0!r}'.format(value))
    if not (0 <= age <= 150):
        raise ValueError('Age must be between 0 and 150, got {0}'.format(age))
    return age

def validate_filename(name):
    """Validate that a filename is safe (no path traversal)."""
    import os.path
    # Prevent path traversal: ../../etc/passwd
    if os.path.sep in name or (os.path.altsep and os.path.altsep in name):
        raise ValueError('Filename must not contain path separators')
    if name.startswith('.'):
        raise ValueError('Filename must not start with a dot')
    return name
```

##### Path Traversal Prevention

```python
import io
import os.path

def safe_read_file(base_dir, user_filename):
    """Read a file within base_dir; reject path traversal attempts."""
    # Normalize and resolve, then compare via relpath. A raw
    # startswith(base + os.sep) check falsely rejects everything when
    # base is a root directory ('/' + os.sep == '//'), and a raw
    # relative.startswith('..') would falsely reject a file literally
    # named '..foo' — compare against os.pardir exactly.
    requested = os.path.realpath(os.path.join(base_dir, user_filename))
    base = os.path.realpath(base_dir)
    relative = os.path.relpath(requested, base)
    if relative == os.pardir or relative.startswith(os.pardir + os.sep):
        raise ValueError('Access denied: path traversal detected')
    with io.open(requested, 'r', encoding='utf-8') as f:
        return f.read()
```

##### SQL Injection Prevention

Always use parameterized queries. Never format SQL strings with user input:[^28]

```python
import sqlite3

# DANGEROUS
def bad_query(user_id):
    conn.execute('SELECT * FROM users WHERE id = ' + user_id)

# SAFE — DB-API 2.0 parameterization
def safe_query(conn, user_id):
    cursor = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,))
    return cursor.fetchall()

# For multiple parameters
def update_user(conn, name, email, user_id):
    conn.execute(
        'UPDATE users SET name=?, email=? WHERE id=?',
        (name, email, user_id)
    )
    conn.commit()
```

##### Secrets and Randomness

```python
import os
import hashlib
import hmac

# Cryptographically secure random bytes
token = os.urandom(32)
token_hex = token.encode('hex')   # Python 2 hex encoding

# Constant-time comparison (prevents timing attacks)
def safe_compare(a, b):
    return hmac.compare_digest(a, b)

# Hashing passwords
import hashlib
import os

def hash_password(password):
    salt = os.urandom(16)
    # encode only unicode — .encode('utf-8') on a non-ASCII str would
    # implicitly ascii-DECODE it first and raise UnicodeDecodeError
    # (see Strings: force_bytes)
    pw_bytes = password.encode('utf-8') if isinstance(password, unicode) else password
    dk = hashlib.pbkdf2_hmac('sha256', pw_bytes, salt, 100000)
    return salt.encode('hex') + ':' + dk.encode('hex')

def verify_password(stored, provided):
    salt_hex, dk_hex = stored.split(':')
    salt = salt_hex.decode('hex')
    dk = dk_hex.decode('hex')
    pw_bytes = provided.encode('utf-8') if isinstance(provided, unicode) else provided
    check = hashlib.pbkdf2_hmac('sha256', pw_bytes, salt, 100000)
    return hmac.compare_digest(dk, check)
```

**Version note:** `hmac.compare_digest` requires Python 2.7.7+ and `hashlib.pbkdf2_hmac` requires 2.7.8+ — run the final 2.7.18 point release.

**Never use `random` for security-sensitive values** — it is a pseudo-random number generator seeded from the clock and is predictable. Use `os.urandom()` or `SystemRandom`:

```python
import random
secure_random = random.SystemRandom()   # uses os.urandom internally
token = secure_random.randint(100000, 999999)
```

***

#### Performance Patterns and Optimization

##### Profiling

Always profile before optimizing:

```python
import cProfile
import pstats

# Profile to file
cProfile.run('main()', 'profile_output')
stats = pstats.Stats('profile_output')
stats.sort_stats('cumulative')
stats.print_stats(20)   # top 20 functions

# Profile a specific section
import timeit
elapsed = timeit.timeit('sum(xrange(1000))', number=10000)
```

##### Common Performance Patterns

**Local variable lookups are faster than global/attribute lookups:**

```python
# BAD — repeated global lookup
for i in xrange(100000):
    result = math.sqrt(i)

# GOOD — local alias
sqrt = math.sqrt
for i in xrange(100000):
    result = sqrt(i)
```

**String building:**

```python
# O(n²) — BAD for large n
result = ''
for item in data:
    result += transform(item)

# O(n) — GOOD
result = ''.join(transform(item) for item in data)
```

**Membership testing:**

```python
# O(n) per lookup
allowed = ['admin', 'editor', 'viewer']
if role in allowed:   # linear scan

# O(1) per lookup
allowed = frozenset(['admin', 'editor', 'viewer'])
if role in allowed:   # hash lookup
```

**Dict `get` vs `setdefault` vs `defaultdict`:**

```python
# Two lookups
if key not in d:
    d[key] = []
d[key].append(val)

# One lookup — setdefault
d.setdefault(key, []).append(val)

# Clearest — defaultdict
from collections import defaultdict
d = defaultdict(list)
d[key].append(val)
```

**`enumerate` over manual index tracking:**

```python
# Fragile
i = 0
for item in lst:
    process(i, item)
    i += 1

# Pythonic
for i, item in enumerate(lst):
    process(i, item)
```

**`zip` / `izip` for parallel iteration:**

```python
import itertools

# zip creates a full list (Python 2)
for a, b in zip(list1, list2):
    pass

# izip is lazy — O(1) memory; prefer in loops
for a, b in itertools.izip(list1, list2):
    pass
```

***

#### `argparse` for Command-Line Tools

```python
import argparse
import sys

def build_parser():
    parser = argparse.ArgumentParser(
        description='Process data files',
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument('input', help='Input file path')
    parser.add_argument('-o', '--output', default='output.csv',
                        help='Output file path')
    parser.add_argument('-n', '--count', type=int, default=100,
                        help='Number of records to process')
    parser.add_argument('-v', '--verbose', action='store_true',
                        help='Enable verbose output')
    parser.add_argument('--log-level', choices=['DEBUG','INFO','WARNING','ERROR'],
                        default='INFO', help='Log level')
    return parser

def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    setup_logging(args.log_level)
    run(args.input, args.output, args.count)

if __name__ == '__main__':
    main()
```

***

#### Virtual Environments and Packaging

There is no `python -m venv` on Python 2 — use `virtualenv`. Modern
`virtualenv` releases dropped the ability to create 2.7 environments in
20.22.0, and pip 20.3.4 is the last pip release that runs on Python 2.7.

##### Setup Commands

```bash
# Install a virtualenv that can still create 2.7 environments
pip install 'virtualenv<20.22'

# Create virtual environment for the 2.7 interpreter
virtualenv -p python2.7 .venv

# Activate (Unix/macOS)
source .venv/bin/activate

# Activate (Windows)
.venv\Scripts\activate

# Pin the toolchain ceiling inside the environment
pip install 'pip==20.3.4' 'setuptools<45'

# Install and freeze dependencies
pip install -r requirements.txt
pip freeze > requirements.txt
```

##### Dependency Pins

Package ecosystems have moved on; unpinned installs pull Python-3-only
releases and fail at install or import time. Pin the ceilings explicitly.

```text
# requirements.txt — last releases that support Python 2.7
six                      # 2/3 compatibility helpers
typing; python_version < "3.5"
mock==3.0.5              # unittest.mock backport
pytest==4.6.11           # pytest 4.6.x is the final 2.7 series
pytest-cov<2.13
coverage<6               # coverage 6.0 dropped Python 2
attrs<=21.4.0            # 22.1.0 dropped Python 2
enum34; python_version < "3.4"
pathlib2; python_version < "3.6"
```

##### Project Structure

`pyproject.toml`-driven builds do not apply to 2.7 — packaging is
`setup.py` + `setup.cfg`, and tool configuration lives in `setup.cfg`
or `tox.ini`.

```
project/
├── .venv/               # Virtual environment (gitignored)
├── myapp/
│   ├── __init__.py      # Required — no implicit namespace packages in 2.7
│   ├── main.py
│   └── utils.py
├── tests/
│   ├── __init__.py
│   └── test_main.py
├── setup.py             # Packaging metadata
├── setup.cfg            # flake8 / isort / coverage configuration
├── requirements.txt     # Pinned dependencies
└── README.md
```

***

#### Testing

```python
import unittest

class TestStringUtils(unittest.TestCase):

    def setUp(self):
        """Called before every test method."""
        self.sample = u'Hello, World!'

    def tearDown(self):
        """Called after every test method."""
        pass

    def test_uppercase(self):
        self.assertEqual(self.sample.upper(), u'HELLO, WORLD!')

    def test_startswith(self):
        self.assertTrue(self.sample.startswith(u'Hello'))

    def test_invalid_type_raises(self):
        with self.assertRaises(AttributeError):
            None.upper()

    def test_split_returns_list(self):
        parts = self.sample.split(', ')
        self.assertIsInstance(parts, list)
        self.assertEqual(len(parts), 2)

if __name__ == '__main__':
    unittest.main()
```

Run tests with:

```bash
python -m unittest discover -s tests -p 'test_*.py'
```

##### pytest (Recommended Runner)

The pytest 4.6 series (`pytest==4.6.11`) is the last line that runs on
Python 2.7 — plain-assert tests, fixtures, and parametrization all work.
Prefer it over raw `unittest` for new tests; both coexist fine.

```python
import pytest
from myapp.calculator import add, divide

def test_add_positive_numbers():
    assert add(2, 3) == 5

def test_divide_by_zero_raises():
    with pytest.raises(ZeroDivisionError):
        divide(10, 0)

@pytest.mark.parametrize('a,b,expected', [
    (1, 1, 2),
    (0, 0, 0),
    (-1, 1, 0),
])
def test_add_parametrized(a, b, expected):
    assert add(a, b) == expected
```

##### Fixtures

```python
import pytest
from myapp.database import Database
from myapp.models import User

@pytest.fixture
def db():
    """Provide a clean database for each test."""
    database = Database(':memory:')
    database.create_tables()
    yield database
    database.close()

def test_user_creation(db):
    user = User(name='Test User', email='test@example.com')
    db.save(user)
    assert db.find_user(user.id).name == 'Test User'
```

##### Mocking

`unittest.mock` does not exist on Python 2 — install the rolling backport
(`pip install mock==3.0.5`, the last release supporting 2.7) and import
from `mock`. The API matches `unittest.mock`.

```python
from mock import Mock, patch  # NOT unittest.mock

def test_api_client_with_mock():
    mock_response = Mock()
    mock_response.json.return_value = {'id': 1, 'name': 'Test'}
    mock_response.status_code = 200

    with patch('requests.get', return_value=mock_response) as mock_get:
        result = fetch_user(1)

    mock_get.assert_called_once_with('/users/1')
    assert result['name'] == 'Test'

@patch('myapp.service.external_api')
def test_with_patch_decorator(mock_api):
    mock_api.get_data.return_value = {'status': 'ok'}
    result = process_data()
    assert result['status'] == 'ok'
```


***

#### Code Quality Tools

Ruff and Black are Python-3-only. The 2.7 toolchain is flake8 + isort +
pylint, configured through `setup.cfg`/`tox.ini`, with pinned versions:
`flake8<3.9` (the linter must run on the 2.7 interpreter to parse 2.7
syntax), `isort<5` (4.3.21 is the last 2.7 release), `pylint<2` (1.9.5),
`coverage<6`.

##### flake8 and isort Configuration

```ini
# setup.cfg
[flake8]
max-line-length = 79
extend-ignore = E203
exclude = .venv,build,dist

[isort]
line_length = 79
known_first_party = myapp
default_section = THIRDPARTY
```

##### Type Checking with mypy

mypy itself runs under Python 3 but can check Python 2 code via type
comments. Python 2 checking was removed in mypy 0.980, so pin an older
release (`pip install 'mypy<0.980'`, e.g. 0.971) and enable `--py2`.

```ini
# mypy.ini
[mypy]
python_version = 2.7
warn_return_any = True
warn_unused_configs = True

# Silence missing stubs per dependency, not globally
[mypy-untyped_package.*]
ignore_missing_imports = True
```

```bash
# Run from a Python 3 environment against the 2.7 codebase
mypy --py2 myapp/
```

***

#### Key Python 2.7 Idioms Summary

| Pattern | BAD (avoid) | GOOD (pythonic) |
|---|---|---|
| Iteration | `for i in range(len(lst)):` | `for i, v in enumerate(lst):` |
| Dict iteration | `for k in d.keys():` | `for k in d:` or `for k in d.iterkeys():` |
| String build | `s = '' ; s += x` in loop | `''.join(parts)` |
| Integer ranges | `for i in range(n):` | `for i in xrange(n):` |
| Type test | `type(x) == int` | `isinstance(x, int)` |
| None check | `if x == None:` | `if x is None:` |
| Bool check | `if bool(lst) == True:` | `if lst:` |
| Context mgmt | `f = open(...) ; f.close()` | `with open(...) as f:` |
| Exception var | `except ValueError, e:` | `except ValueError as e:` |
| Swap vars | `t=a; a=b; b=t` | `a, b = b, a` |
| Unicode str | `'Hello'` | `u'Hello'` or use `unicode_literals` |
| Division | `7 / 2` (unpredictable) | `from __future__ import division` |
| Print | `print x` | `from __future__ import print_function` |

***

#### Production Checklist

Before deploying Python 2.7 code to production, verify:

- [ ] `# -*- coding: utf-8 -*-` in every source file with non-ASCII content
- [ ] `from __future__ import print_function, division, unicode_literals, absolute_import` at the top of every module
- [ ] All classes inherit from `object` (new-style classes)
- [ ] `super(ClassName, self).__init__(...)` used in all `__init__` overrides
- [ ] Unicode sandwich: decode on input, `unicode` internally, encode on output
- [ ] `io.open()` used for text file I/O with explicit encoding
- [ ] `xrange` used instead of `range` in all loops
- [ ] `d.iteritems()` / `d.iterkeys()` / `d.itervalues()` used in hot loops
- [ ] `with` statement used for all file, lock, and resource management
- [ ] No bare `except:` or `except Exception: pass` in production code
- [ ] `logging` module used (not `print`) for all observability
- [ ] Logger instantiated as `logging.getLogger(__name__)` per module
- [ ] Logging calls use `%` style (`logger.info('x=%s', x)` not `'x={}'.format(x)`)
- [ ] `subprocess` with list arguments used instead of `os.system()`
- [ ] All SQL queries use parameterized queries — no string formatting
- [ ] No `pickle.load` on untrusted data
- [ ] No `tempfile.mktemp()` — use `NamedTemporaryFile` or `mkdtemp()`
- [ ] `os.urandom()` or `SystemRandom()` for all security-sensitive random values
- [ ] `hmac.compare_digest()` for constant-time secret comparison
- [ ] `__all__` defined in every public-facing module
- [ ] `functools.wraps` applied in every decorator
- [ ] `if __name__ == '__main__':` guards on all runnable scripts
- [ ] Toolchain pinned to the last 2.7-compatible releases (pip==20.3.4, virtualenv<20.22, pytest==4.6.11, mock==3.0.5, flake8<3.9, pylint<2, isort<5, coverage<6, attrs<=21.4.0)
- [ ] PEP 484 type comments on public APIs, checked with `mypy<0.980 --py2`
- [ ] Imports grouped stdlib / third-party / local; no wildcard imports

---

#### References

[^1]: [What’s New in Python 2.7](https://docs.python.org/3/whatsnew/2.7.html) - Author, A.M. Kuchling (amk at amk.ca),. This article explains the new features in Python 2.7. Python...

[^2]: [Python 2.7. Unicode Errors Simply Explained - GitHub Gist](https://gist.github.com/gornostal/1f123aaf838506038710) - Python 2.7. Unicode Errors Simply Explained. GitHub Gist: instantly share code, notes, and snippets.

[^3]: [Integer division in Python 2 and Python 3](https://stackoverflow.com/questions/21316968/integer-division-in-python-2-and-python-3/21317109) - How can I divide two numbers in Python 2.7 and get the result with decimals? I don't get it why ther...

[^4]: [Should I use the print statement or function in Python 2.7?](https://stackoverflow.com/questions/34191828/should-i-use-the-print-statement-or-function-in-python-2-7/34191915) - To make my simple scripts in Python 2.7.10, should I use the print function instead of the statement...

[^5]: [5. Built-in Types¶](https://docs.python.org/2.7/library/stdtypes.html)

[^6]: [python-notes/unicode-vs-str-vs-bytes.md at master · peter-can-write/python-notes](https://github.com/peter-can-write/python-notes/blob/master/unicode-vs-str-vs-bytes.md) - Notes on Python. Contribute to peter-can-write/python-notes development by creating an account on Gi...

[^7]: [error encoding string as unicode in python 2.7?](https://stackoverflow.com/questions/24742886/error-encoding-string-as-unicode-in-python-2-7) - I want to print the unicode version of a string in Python 2.7. It works fine in Python 3. But with p...

[^8]: [Solving Unicode Problems in Python 2.7 - Element 84](https://element84.com/software-engineering/solving-unicode-problems-in-python-2-7/) - We discuss three steps you can take to begin thinking about strings and unicode the right way.

[^9]: [Idiomatic Python: comprehensions - Microsoft Developer Blogs](https://devblogs.microsoft.com/python/idiomatic-python-comprehensions/) - We’re lucky to have a few people on our team who have been programming in Python for quite a while (...

[^10]: [问dict.items()和dict.iteritems()在Python2中有什么区别？ - 腾讯云](https://cloud.tencent.com/developer/ask/sof/109423346) - 和之间有什么适用的区别吗？来自dict.items()：返回字典(键、值)对列表的副本。dict.iteritems()：在字典的(键，值)对上返回一个迭代器。如果我运行下面的代码，每个代码似乎都会返...

[^11]: [range vs. xrange in Python](https://python.code-maven.com/range-vs-xrange-in-python)

[^12]: [Python: range() vs. xrange()](http://justindailey.blogspot.com/2011/09/python-range-vs-xrange.html) - I recently discovered a really cool Python module called timeit . It let's you efficiently time bloc...

[^13]: [Difference Between range and xrange in Python | xrange vs range](https://www.csestack.org/difference-range-xrange-python/) - What is the main difference between range and xrange? How has this changed over time?

[^14]: [Python: Joining multiple generators/iterators - Mark Needham](https://www.markhneedham.com/blog/2015/05/24/python-joining-multiple-generatorsiterators/) - `itertools.chain()` returns an iterator, not a generator. Generator is a subclass of Iterator. Not e...

[^15]: [List Comprehensions | Set Comprehension | Dict Comprehension | Python Tutorials Ep. 32](https://www.youtube.com/watch?v=3SR110cPcug) - In this Python tutorial, dive deep into the power of list comprehensions, set comprehensions, and di...

[^16]: [9.7. — Functions creating iterators for efficient looping¶](https://docs.python.org/2.7/library/itertools.html)

[^17]: [2.7. New and Old Style Class - Python - from None to AI](https://www.python3.edu.pl/oop/python/new-style-classes.html) - New and Old Style Class . Important. Old Style classes - Not existing in Python 3. In Python 3 class...

[^18]: [Python Language Tutorial => New-style vs. old-style classes](https://riptutorial.com/python/example/1402/new-style-vs--old-style-classes) - Learn Python Language - New-style vs. old-style classes

[^19]: [Python2.7で class C: と class C(object): の違いに大ハマりした話 - Qiita](https://qiita.com/alt-core/items/d20e9a1864ff3a35946e) - 旧スタイルのクラス定義というものを知っていますか Python2 系では、class C: と class C(object): が異なる挙動をするということを知らずに、大ハマりしました。 参考: h...

[^20]: [Usage of __slots__?](https://stackoverflow.com/questions/472000/usage-of-slots) - What is the purpose of __slots__ in Python — especially with respect to when I would want to use it,...

[^21]: [Inherit class with weakref in slots](https://stackoverflow.com/questions/24407874/inherit-class-with-weakref-in-slots) - I tried to use weak references on my classes, where I use slots to save some memory, but I wasn't ab...

[^22]: [Python 2.7 exception handling syntax - Stack Overflow](https://stackoverflow.com/questions/32613375/python-2-7-exception-handling-syntax) - Catch exceptions of the type Exception (or any subclass) and store them in the variable e … don't ca...

[^23]: [exception handling | Python Best Practices](https://realpython.com/ref/best-practices/exception-handling/) - Guidelines and best practices for handling exceptions and errors in your Python code.

[^24]: [Python Exception Handling: Patterns and Best Practices - Jerry Ng](https://jerrynsh.com/python-exception-handling-patterns-and-best-practices/) - In terms of best practices – it is generally recommended to use the from e syntax when raising a new...

[^25]: [PEP 343 – The “with” Statement - Python Enhancement Proposals](https://peps.python.org/pep-0343/) - This PEP adds a new statement “with” to the Python language to make it possible to factor out standa...

[^26]: [3.4.9 With Statement Context Managers](https://python.developpez.com/cours/PythonDocs/ref/context-managers.php) - Documentation officielle de Python

[^27]: [Python's with Statement and Context Managers](https://www.blog.pythonlibrary.org/2021/04/07/pythons-with-statement-and-context-managers/) - Learn about Python's context managers, the contextlib module and Python's with statement in this art...

[^28]: [Python Security: 6 Common Risks & What You Can Do About Them](https://www.aquasec.com/cloud-native-academy/application-security/python-security/) - 6 Common Python Security Vulnerabilities · 1. Injections and Arbitrary Command Execution · 2. Overly...

[^29]: [Why does integer division yield a float instead of another integer?](https://stackoverflow.com/questions/1282945/why-does-integer-division-yield-a-float-instead-of-another-integer) - In Python 2.7: By default, division operator will return integer output. To get the result in double...

[^30]: [Why does Python 2 use '/' only as integer division?](https://stackoverflow.com/questions/42380531/why-does-python-2-use-only-as-integer-division) - I have noticed that Python 2.7 uses '/' as integer division. >>> print 1/2 0 Since Python is a dynam...

[^31]: [PEP 238 – Changing the Division Operator | peps.python.org](https://peps.python.org/pep-0238/) - The future division statement, spelled from __future__ import division , will change the / operator ...

[^32]: [logging | Python Best Practices](https://realpython.com/ref/best-practices/logging/) - Guidelines and best practices for logging in Python.

[^33]: [Python best practice in terms of logging](https://stackoverflow.com/questions/22807972/python-best-practice-in-terms-of-logging) - When using the logging module from python for logging purposes. Is it best-practice to define a logg...

[^34]: [Dangerous os.system() or os.popen() Call - Python SAST Security Rule | Code Pathfinder](https://codepathfinder.dev/registry/python/lang/PYTHON-LANG-SEC-010) - os.system() and os.popen() execute shell commands via /bin/sh, enabling command injection when argum...

[^35]: [Preventing my script from os command injection python](https://stackoverflow.com/questions/44040687/preventing-my-script-from-os-command-injection-python) - i am using python 2.7.x I automating my stuffs and in there i need run to another python program fro...

[^36]: [Living with the GIL: Strategies for Concurrent Python - DEV Community](https://dev.to/aaron_rose_0787cc8b4775a0/living-with-the-gil-strategies-for-concurrent-python-2d5e) - Timothy and Margaret walked through the library's quiet reading room toward the small coffee shop in...

[^37]: [Understanding Python's Garbage Collection and Memory Optimization](https://dev.to/pragativerma18/understanding-pythons-garbage-collection-and-memory-optimization-4mi2) - While Python's garbage collection system handles memory cleanup automatically, unoptimized memory us...

[^38]: [20% Faster Python with a Single GC Tweak - Michael Kennedy](https://mkennedy.codes/posts/python-gc-settings-change-this-and-make-your-app-go-20pc-faster/) - 20% Faster Python with a Single GC Tweak ... TL;DR: Increase Python's GC allocation threshold from 7...

