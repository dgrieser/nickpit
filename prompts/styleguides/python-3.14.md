### Python 3.14: The Complete Production Developer Guide

#### Overview

Python 3.14 was released on October 7, 2025.[^1] It is in the **bugfix ("maintenance") phase** as of October 2026: bugfix releases ship roughly every two months for about 24 months — the final regular bugfix release with binary installers is planned for around October 2027 (PEP 745 lists it as 3.14.14 on October 5, 2027; 3.14.9 shipped ahead of that schedule, so later numbers may shift) — followed by source-only security fixes until **October 2030**.[^2] The latest patch release is **3.14.9 (October 21, 2026)**; Python 3.15.0 shipped on October 9, 2026 (3.15.1 followed on October 21, 2026), so 3.14 is now the *previous* stable release, not the newest.[^3] Always run the latest 3.14.x patch: several point releases contain security fixes and one (3.14.5) changed the garbage collector back (see the point-release notes throughout this guide).

Python 3.14 is a large release. Its headline changes are **deferred evaluation of annotations** (PEP 649/749) with the new `annotationlib` module, **template string literals** (t-strings, PEP 750), **multiple interpreters in the standard library** (PEP 734: `concurrent.interpreters` and `InterpreterPoolExecutor`), **officially supported free-threaded builds** (PEP 779), **Zstandard compression** in the new `compression` package (PEP 784), a **safe external debugger interface** (PEP 768: `sys.remote_exec()`, `python -m pdb -p PID`), `except A, B:` **without parentheses** (PEP 758), a **`SyntaxWarning` for `return`/`break`/`continue` leaving a `finally` block** (PEP 765), asyncio task introspection (`python -m asyncio ps/pstree`), and several *behavior changes that break old code*: `forkserver` replaces `fork` as the default multiprocessing start method on Linux, `tarfile` extraction defaults to the `'data'` filter, `asyncio.get_event_loop()` no longer creates a loop, `int()` no longer falls back to `__trunc__()`, `NotImplemented` in boolean context raises `TypeError`, and a batch of long-deprecated APIs is gone (`ast.Num`, asyncio child watchers, `pkgutil.find_loader`, `urllib.request.URLopener`, ...).[^1]

This guide targets Python 3.14 specifically. Everything from earlier versions is available and should be used where it fits: structural pattern matching, `X | Y` unions, parenthesized context managers, `zip(strict=True)` and `dataclass(slots=True, kw_only=True)` (3.10); `ExceptionGroup`/`except*`, `asyncio.TaskGroup`, `asyncio.timeout()`, `tomllib`, `typing.Self` and `BaseException.add_note()` (3.11); PEP 695 type-parameter syntax and the `type` statement, PEP 701 f-strings, `@typing.override`, `itertools.batched()` and `Path.walk()` (3.12); `copy.replace()`, `@warnings.deprecated`, `typing.TypeIs`/`ReadOnly`, type-parameter defaults (PEP 696), `queue.Queue.shutdown()` and the new REPL (3.13).[^4] Every section that covers a 3.14-specific change is marked **New in 3.14**. **Do not use or recommend anything from Python 3.15 or later** — the project's interpreter does not have it: no explicit lazy imports (`lazy import x`, `__lazy_modules__`, PEP 810), no `frozendict` built-in (PEP 814), no `sentinel` built-in (PEP 661), no unpacking in comprehensions (`[*xs for xs in rows]`, PEP 798), no `TypedDict` `closed=`/`extra_items=` (PEP 728), no `typing.TypeForm` (PEP 747) or `@typing.disjoint_base` (PEP 800), no `profiling` package / Tachyon sampling profiler (PEP 799), no `math.integer`, `math.isnormal()`/`fmax()`/`signbit()`, `re.prefixmatch()`, `asyncio.TaskGroup.cancel()`, `Counter.__xor__`, `threading.serialize_iterator`, `json.load(array_hook=)` or `bytearray.take_bytes()` (all 3.15) — and do not assume UTF-8 is the default text encoding: UTF-8 mode becomes the default only in 3.15 (PEP 686).[^5]

Global truths about Python 3.14:

- **Everything is an object.** Integers, functions, classes, modules, annotations' `__annotate__` functions and t-string `Template`s — all are objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing, static checking.** Values carry types; annotations are *not* enforced at runtime. Type checkers (mypy, pyright) do the static work.
- **Annotations are lazy.** In 3.14, function, class and module annotations are compiled into an `__annotate__` function and evaluated only when someone asks for them (PEP 649/749). Forward references no longer need quotes.[^6]
- **Reference counting + cyclic GC.** CPython frees most objects immediately when their refcount drops to zero; a cyclic collector handles cycles. (3.14.0–3.14.4 shipped an *incremental* collector; 3.14.5 reverted to the 3.13 generational collector.)[^1]
- **The GIL is still the default.** The standard `python3.14` build has a GIL. A separate, officially supported free-threaded build (`python3.14t`) runs without it.[^7]
- **`str` is Unicode; `bytes` is binary.** They never mix implicitly. The default text encoding is still locale-dependent in 3.14 — always pass `encoding=`.
- **Dicts are ordered by insertion.** Guaranteed by the language since 3.7.

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

type UserId = int                 # PEP 695 type alias statement (3.12+)

# Type parameters: short PascalCase names in the new bracket syntax (3.12+)
def first[T](items: list[T]) -> T:
    return items[0]

# Internal use: single underscore prefix
class User:
    def __init__(self) -> None:
        self._internal_state: dict[str, object] = {}

# Name mangling: double underscore prefix (not true privacy)
class Base:
    def __init__(self) -> None:
        self.__private = 'name-mangled'   # stored as _Base__private
```

All classes implicitly inherit from `object`; never write `class Foo(object):`.

##### Import Ordering

Three blocks separated by blank lines: standard library, third-party, local. Alphabetize within each group; `ruff check --select I` (or isort) automates this.

```python
# Standard library
import os
import sys
from collections.abc import Callable, Iterator   # ABCs from collections.abc, not typing
from pathlib import Path

# Third-party
# import httpx

# Local application
# from myapp.models import User

# Never wildcard imports — they defeat static analysis and __all__ hygiene
```

**Do not** write `lazy import x` or rely on `__lazy_modules__`: explicit lazy imports are Python 3.15 (PEP 810). In 3.14, `lazy import json` is a `SyntaxError`, and a `__lazy_modules__` list is silently ignored (imports stay eager).[^5] Keep the 3.14 idioms for deferring expensive imports: import inside the function that needs it, or use `importlib.util.LazyLoader`.

##### Docstrings

Write Google-style (or NumPy-style, consistently) docstrings for every public module, class, and function. First line: a one-sentence imperative summary. With deferred annotations there is no reason to repeat types in docstrings — the signature carries them.

```python
def calculate_discount(price: float, discount_percent: float, min_price: float = 0.0) -> float:
    """Calculate the discounted price.

    Args:
        price: Original price of the item.
        discount_percent: Discount percentage (0-100).
        min_price: Minimum price floor.

    Returns:
        The discounted price, not less than min_price.

    Raises:
        ValueError: If discount_percent is not between 0 and 100.
    """
    if not 0 <= discount_percent <= 100:
        raise ValueError('Discount must be between 0 and 100')
    return max(price * (1 - discount_percent / 100), min_price)


class UserService:
    """Service for managing user operations.

    Attributes:
        db: Database connection.
        cache: Optional cache for user lookups.
    """

    def __init__(self, db: DatabaseConnection, cache: Cache | None = None) -> None:
        # No quotes needed around DatabaseConnection/Cache in 3.14:
        # the annotations are not evaluated when the def runs.
        self.db = db
        self.cache = cache


class DatabaseConnection: ...
class Cache: ...
```

`dataclasses.field(doc=...)` is **new in 3.14** — use it to document dataclass fields so `help()` and `pydoc` show them (see Dataclasses).[^8]

***

#### New in 3.14: Deferred Evaluation of Annotations (PEP 649 / PEP 749)

This is the most far-reaching change in 3.14. Annotations on functions, classes and modules are **no longer evaluated when the `def`/`class` statement runs**. The compiler stores them in a generated `__annotate__` function, and they are evaluated the first time something reads `__annotations__` (or calls `annotationlib.get_annotations()`, `typing.get_type_hints()`, `inspect.signature()`, ...).[^6][^9]

```python
# Python 3.13: NameError at the def line — Node is not defined yet.
# Python 3.14: fine — the annotation is evaluated lazily, on first access.
class Node:
    def __init__(self, value: int, next: Node | None = None) -> None:
        self.value = value
        self.next = next

    def append(self, value: int) -> Node:     # ✅ no quotes, no __future__ import
        self.next = Node(value)
        return self.next

print(Node.append.__annotations__)   # {'value': <class 'int'>, 'return': <class '__main__.Node'>}
```

Consequences a reviewer must know:

- **Forward references need no quotes** and no `from __future__ import annotations`. New 3.14-only code should write `def f() -> Node:` rather than `-> 'Node'`.
- **Annotations with side effects no longer run at definition time.** Code that relied on an annotation expression executing during import (registration hacks, `print`, function calls) now runs it lazily — or never.
- **Errors move.** A typo in an annotation (`def f(x: Strr)`) is no longer an import-time `NameError`; it surfaces when the annotation is first read — possibly inside a framework at request time. Run a type checker in CI.
- **Annotations on local variables inside a function body are still never evaluated** (unchanged).

##### The Three Formats (`annotationlib.Format`)

`annotationlib` (new module) reads annotations in one of three formats:[^9]

```python
from annotationlib import Format, get_annotations

def handler(request: Request, retries: int = 3) -> Response: ...

# VALUE (default) — evaluate to real objects; raises NameError while names are missing
try:
    get_annotations(handler, format=Format.VALUE)
except NameError as exc:
    print(exc)                                   # name 'Request' is not defined

# FORWARDREF — real values where possible, ForwardRef proxies for undefined names
print(get_annotations(handler, format=Format.FORWARDREF)['retries'])   # <class 'int'>
print(get_annotations(handler, format=Format.FORWARDREF)['request'])
# ForwardRef('Request', owner=<function handler at 0x...>)

# STRING — the source text, no evaluation of names
print(get_annotations(handler, format=Format.STRING))
# {'request': 'Request', 'retries': 'int', 'return': 'Response'}

class Request: ...
class Response: ...
print(get_annotations(handler)['request'])   # <class '__main__.Request'> — VALUE works now
```

`Format` is an `IntEnum`: `VALUE = 1`, `VALUE_WITH_FAKE_GLOBALS = 2` (internal — never pass it yourself), `FORWARDREF = 3`, `STRING = 4`.[^9]

**Rule: library and framework code that introspects annotations should call `annotationlib.get_annotations(obj, format=Format.FORWARDREF)`** (or `typing.get_type_hints(obj, format=...)`) instead of reading `obj.__annotations__` directly. That is what `dataclasses` itself does in 3.14.[^1]

##### `typing.get_type_hints()`, `inspect.signature()` and `TYPE_CHECKING` Imports

Names imported only under `if TYPE_CHECKING:` never exist at runtime. With deferred annotations the *definition* now succeeds (in 3.13 it was an immediate `NameError` unless the annotation was quoted or the module used `from __future__ import annotations`), but anything that evaluates the annotation in `VALUE` format still fails — including `inspect.signature()`:

```python
import inspect
import typing
from typing import TYPE_CHECKING

from annotationlib import Format

if TYPE_CHECKING:
    from decimal import Decimal

def price(amount: Decimal) -> str:      # ✅ defines fine in 3.14 (NameError in 3.13)
    return str(amount)

# ❌ These evaluate annotations in VALUE format → NameError at runtime
for call in (lambda: typing.get_type_hints(price), lambda: inspect.signature(price)):
    try:
        call()
    except NameError as exc:
        print('NameError:', exc)         # NameError: name 'Decimal' is not defined

# ✅ Pass a non-VALUE format when the names may be missing
print(typing.get_type_hints(price, format=Format.FORWARDREF)['return'])   # <class 'str'>
print(inspect.signature(price, annotation_format=Format.STRING))          # (amount: 'Decimal') -> 'str'
```

**Pitfall for runtime-typed frameworks** (FastAPI, pydantic, typer, cattrs, dependency injectors): they evaluate annotations. Types they need at runtime must be importable at runtime — never hide them behind `TYPE_CHECKING`, quoted or not. `typing.get_type_hints()` gained the `format=` parameter in 3.14, and calling it on an *instance* (rather than a class/function/module) now raises `TypeError`.[^10]

##### `from __future__ import annotations` in 3.14

The PEP 563 future import still works exactly as before in 3.14: annotations become strings and `__annotations__` returns them as strings. It does **not** emit a warning in 3.14. It is, however, officially on its way out: it will be deprecated (compiling it will emit `DeprecationWarning`) no sooner than the first release after Python 3.13 reaches end-of-life in 2029, and removed at least two releases after that.[^1][^11]

```python
from __future__ import annotations   # only needed if the code must also run on <= 3.13

def f(x: Undefined) -> int: ...

print(f.__annotations__)   # {'x': 'Undefined', 'return': 'int'} — plain strings, as before
```

Reviewer guidance:

- **Code that supports only 3.14+**: remove the future import and the quotes around forward references. Deferred evaluation gives you real objects on access, which runtime tools handle far better than strings.
- **Code that must also run on 3.13 and older**: keep using the future import or quotes there; it behaves identically on 3.14.
- **Do not mix**: a module with the future import produces *strings* even in `Format.VALUE`; tools reading those must resolve them with `get_annotations(obj, eval_str=True)` or `typing.get_type_hints()`.

##### Reading Annotations Correctly (and What Broke)

```python
import annotationlib

class Config:
    host: str
    port: int = 8080

# ✅ Correct ways to read class annotations
print(annotationlib.get_annotations(Config))   # {'host': <class 'str'>, 'port': <class 'int'>}
print(Config.__annotations__)                 # {'host': <class 'str'>, 'port': <class 'int'>} — cached

# ❌ Reading annotations through an instance no longer works (was accidental)
try:
    Config().__annotations__
except AttributeError as exc:
    print(exc)          # 'Config' object has no attribute '__annotations__'

# ❌ The class namespace no longer contains '__annotations__' during class creation
class Meta(type):
    def __new__(mcls, name, bases, ns):
        print('__annotations__' in ns)                                       # False
        print(annotationlib.get_annotate_from_class_namespace(ns) is not None)  # True
        return super().__new__(mcls, name, bases, ns)

class Model(metaclass=Meta):
    field: int
```

Metaclasses, class decorators and `__init_subclass__` hooks that read `ns['__annotations__']` or `cls.__dict__['__annotations__']` get a `KeyError` in 3.14 — or, with `.get('__annotations__', {})`, silently see no annotations at all (unless the module uses the future import). Use `annotationlib.get_annotate_from_class_namespace(ns)` + `annotationlib.call_annotate_function(annotate, Format.FORWARDREF)` during class creation, and `annotationlib.get_annotations(cls)` afterwards.[^9]

**Pitfall: `typing.ForwardRef` is now `annotationlib.ForwardRef`** and no longer caches its evaluated value; private methods such as `ForwardRef._evaluate` are undocumented and changed. Use `ForwardRef.evaluate()` or `typing.evaluate_forward_ref()` (new in 3.14), which also resolves forward references nested inside the hint.[^10]

**Security:** evaluating annotations executes arbitrary code — even `Format.STRING` can execute code for crafted annotations. Never feed untrusted strings into `ForwardRef`, `__annotations__` or `eval_str=True`.[^9]

##### Dataclasses with Forward References

```python
import inspect
from dataclasses import dataclass

@dataclass
class Tree:
    left: Tree | None = None          # ✅ self-reference, no quotes
    right: Leaf | None = None         # ✅ Leaf is defined later — still fine to define

print(Tree())                         # Tree(left=None, right=None)

try:
    inspect.signature(Tree)           # ❌ evaluates annotations before Leaf exists
except NameError as exc:
    print(exc)                        # name 'Leaf' is not defined

class Leaf: ...

print(inspect.signature(Tree))
# (left: __main__.Tree | None = None, right: __main__.Leaf | None = None) -> None
```

**Point-release note:** 3.14.1 introduced two annotation-related regressions in `dataclasses` (with `slots=True` + `init=False`, and with dynamically removed fields); they were fixed in the expedited **3.14.2** release three days later. Do not pin 3.14.1.[^12]

##### Typing Annotations: What Changed at Runtime

```python
import typing
from typing import Union

# typing.Union and types.UnionType are now the same type
print(Union[int, str])                      # int | str   (was typing.Union[int, str])
print(Union[int, str] == (int | str))       # True
print(Union[int, str] is Union[int, str])   # False — no longer cached; compare with ==
print(isinstance(int | str, Union))         # True — new in 3.14 (was TypeError)
print(typing.get_origin(int | None))        # <class 'typing.Union'>
print(typing.get_args(int | None))          # (<class 'int'>, <class 'NoneType'>)
```

Code that compared unions with `is`, string-matched `repr()` output (`'typing.Union['`), or checked for the private `typing._UnionGenericAlias` must be updated; the private class remains only as a compatibility shim until 3.17. Use `typing.get_origin()`/`get_args()`.[^1]

***

#### New in 3.14: Template String Literals — t-strings (PEP 750)

A t-string uses f-string syntax with a `t` prefix, but evaluates to a `string.templatelib.Template` object instead of a `str`. The template keeps the static text and the interpolated values **separate**, so the code that consumes it can escape, parameterize or validate each value before combining them.[^13][^14]

```python
from string.templatelib import Interpolation, Template

user = 'Alice'
amount = 12.5
template = t'Hello {user}, you owe {amount:.2f}'

print(type(template))           # <class 'string.templatelib.Template'>
print(template.strings)         # ('Hello ', ', you owe ', '')
print(template.values)          # ('Alice', 12.5)
for part in template:           # iteration yields str and Interpolation, skipping empty strings
    print(repr(part))
# 'Hello '
# Interpolation('Alice', 'user', None, '')
# ', you owe '
# Interpolation(12.5, 'amount', None, '.2f')
```

An `Interpolation` has four attributes: `value` (the evaluated object), `expression` (the source text, e.g. `'amount'`), `conversion` (`'r'`, `'s'`, `'a'` or `None`) and `format_spec` (e.g. `'.2f'`). Unlike f-strings, **the conversion and format spec are *not* applied automatically** — the consuming function decides; `string.templatelib.convert(value, conversion)` reproduces f-string conversion semantics.[^14]

##### Writing a Template Processor

```python
import html
from string.templatelib import Interpolation, Template, convert

def render_html(template: Template) -> str:
    """Render a t-string, HTML-escaping every interpolated value."""
    parts: list[str] = []
    for item in template:
        match item:
            case str() as text:
                parts.append(text)                       # static text is trusted
            case Interpolation(value, _, conversion, format_spec):
                value = convert(value, conversion)
                parts.append(html.escape(format(value, format_spec)))
    return ''.join(parts)

comment = '<script>alert("x")</script>'
print(render_html(t'<p>{comment}</p>'))
# <p>&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;</p>
print(render_html(t'<td>{3.14159:.2f}</td>'))   # <td>3.14</td>
```

The same shape turns a t-string into a parameterized SQL query — the values never touch the SQL text:

```python
import sqlite3
from string.templatelib import Interpolation, Template

def to_sql(template: Template) -> tuple[str, tuple[object, ...]]:
    sql: list[str] = []
    params: list[object] = []
    for item in template:
        if isinstance(item, Interpolation):
            sql.append('?')
            params.append(item.value)
        else:
            sql.append(item)
    return ''.join(sql), tuple(params)

conn = sqlite3.connect(':memory:')
conn.execute('CREATE TABLE users (id INTEGER, name TEXT)')
conn.execute('INSERT INTO users VALUES (1, ?)', ("Robert'); DROP TABLE users;--",))

name = "Robert'); DROP TABLE users;--"
query, params = to_sql(t'SELECT id FROM users WHERE name = {name}')
print(query)                                  # SELECT id FROM users WHERE name = ?
print(conn.execute(query, params).fetchall()) # [(1,)]
conn.close()
```

**Rule:** a t-string is only as safe as the function that consumes it. Accept `Template` (not `str`) in APIs that must never receive pre-formatted text, and type-annotate them so a type checker rejects an f-string passed by mistake.

##### t-string Pitfalls

```python
from string.templatelib import Template

name = 'World'
tpl = t'Hello {name}'

# ❌ A Template is not a str — there is no rendering __str__
print(isinstance(tpl, str))      # False
print(str(tpl))                  # Template(strings=('Hello ', ''), interpolations=(Interpolation('World', 'name', None, ''),))

# ❌ Concatenating a Template with a str is a TypeError (ambiguous: static or dynamic?)
try:
    tpl + '!'
except TypeError as exc:
    print(exc)       # can only concatenate string.templatelib.Template (not "str") to string.templatelib.Template

# ✅ Wrap the str explicitly: Template('!') is static text
print((tpl + Template('!')).strings)   # ('Hello ', '!')

# ✅ Templates concatenate with templates, including implicit literal concatenation
print((t'a' t'{name}').strings)        # ('a', '')

# ❌ Equality is identity — two identical t-strings are not ==
print(t'x' == t'x')                    # False
```

- **Mixing literal kinds is a `SyntaxError`:** `t'a' 'b'` and `t'a' f'b'` do not compile ("cannot mix t-string literals with string or bytes literals").
- **Interpolations are evaluated eagerly**, exactly like f-strings. A t-string is not a lazy logging message: `logger.debug(t'{expensive()}')` still calls `expensive()`. Keep `%`-style logging arguments.
- **`{x=}` works**: `t'{x=}'` stores the debug text `'x='` in `strings` and sets the interpolation's `conversion` to `'r'` — the processor still decides how to render it.
- `string.templatelib.Template` (t-strings) is unrelated to the old `string.Template` (`$name` substitution). Do not confuse them in reviews.
- No standard-library API in 3.14 accepts `Template` objects; they are a building block for libraries (HTML, SQL, shell, logging) and your own code. Code that must run on 3.13 or older cannot contain t-string literals at all (they are a `SyntaxError` at compile time).

***

#### New in 3.14: `except A, B:` Without Parentheses (PEP 758)

`except` and `except*` may now list several exception types without parentheses — **only when there is no `as` clause**.[^15]

```python
def connect(host: str) -> None:
    raise ConnectionRefusedError(host)

try:
    connect('db.internal')
except TimeoutError, ConnectionRefusedError:        # ✅ 3.14 syntax
    print('network unavailable')

try:
    connect('db.internal')
except (TimeoutError, ConnectionRefusedError) as exc:   # ✅ `as` still requires parentheses
    print(type(exc).__name__)                            # ConnectionRefusedError
```

```python
# ❌ SyntaxError in 3.14: "multiple exception types must be parenthesized when using 'as'"
src = 'try:\n    pass\nexcept TimeoutError, ConnectionRefusedError as exc:\n    pass\n'
try:
    compile(src, '<review>', 'exec')
except SyntaxError as exc:
    print(exc.msg)
```

**Reviewer warning — the Python 2 trap:** in Python 2, `except ValueError, e:` meant "catch `ValueError` and bind it to `e`". In 3.14 the same line means "catch `ValueError` **or** the exception class named `e`" — when an exception actually reaches the handler, it raises `TypeError` ("catching classes that do not inherit from BaseException is not allowed") if `e` is bound to a non-exception, or `NameError` if `e` is unbound — and nothing at all happens while no exception occurs, so the bug survives every happy-path test. Treat any `except X, name:` where `name` looks like a variable as a bug. Prefer the parenthesized form in libraries that must still compile on 3.13 and older (unparenthesized is a `SyntaxError` there); formatters and linters configured for an older target flag it.

***

#### New in 3.14: `return`/`break`/`continue` Leaving a `finally` Block (PEP 765)

The compiler now emits a **`SyntaxWarning`** when `return`, `break` or `continue` would transfer control out of a `finally` block. Such statements silently swallow any in-flight exception — including `KeyboardInterrupt` and `SystemExit`.[^16]

```python
import warnings

def swallow() -> str:
    try:
        raise RuntimeError('lost')
    finally:
        return 'ok'          # the RuntimeError disappears without a trace

src = "def f():\n    try:\n        1 / 0\n    finally:\n        return 1\n"
with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter('always')
    compile(src, '<review>', 'exec')
print(caught[0].category.__name__, caught[0].message)   # SyntaxWarning 'return' in a 'finally' block
```

```python
# ✅ Return after the try statement; let exceptions propagate
def safe() -> str:
    try:
        result = 'ok'
    finally:
        cleanup_done = True      # cleanup only — no control flow out of finally
    return result

# ✅ Nested functions and loops *inside* finally are fine (no warning)
def ok() -> None:
    try:
        pass
    finally:
        for i in range(3):
            if i == 1:
                break            # leaves the inner loop, not the finally block

print(safe())   # ok
```

Notes:

- The warning is emitted at **compile time**, so it appears when the module is first compiled (CI, `python -m compileall`, first import without a cached `.pyc`), not on every run. Under `-W error` it becomes a `SyntaxError` and the module fails to import.
- A runtime `warnings.filterwarnings(...)` only affects code compiled *after* the filter is installed. To silence it for a dependency, use `-W ignore::SyntaxWarning` / `PYTHONWARNINGS=ignore::SyntaxWarning` — but fix your own code instead.[^1]
- PEP 765 leaves open whether this ever becomes a `SyntaxError` in CPython. The fix is always the same: move the `return`/`break`/`continue` out of `finally`.

***

#### Primitive Types and Numerics

##### Integers

`int` is arbitrary precision; arithmetic never overflows. The 3.14 changes that matter in reviews are `int()` no longer falling back to `__trunc__()` and `NotImplemented` being rejected in boolean context:

```python
import sys

x = 10 ** 1000               # fine — arbitrary precision
one_million = 1_000_000      # underscores in numeric literals

# Converting very large ints to/from decimal strings is capped (CVE-2020-10735 fix, since 3.11)
print(sys.get_int_max_str_digits())     # 4300 by default
try:
    int('9' * 5000)
except ValueError as exc:
    print(exc)   # Exceeds the limit (4300 digits) for integer string conversion: value has 5000 digits; ...

# New in 3.14: int() no longer falls back to __trunc__()
class Legacy:
    def __trunc__(self) -> int:
        return 3

try:
    int(Legacy())
except TypeError as exc:
    print(exc)   # int() argument must be a string, a bytes-like object or a real number, not 'Legacy'

class Modern:
    def __index__(self) -> int:    # ✅ implement __int__ or __index__ instead
        return 3

print(int(Modern()))               # 3
```

`__trunc__` delegation was deprecated in 3.11 and removed in 3.14 (gh-119743): classes that want `int(obj)` must implement `__int__()` or `__index__()`.[^1]

**New in 3.14: `NotImplemented` in a boolean context raises `TypeError`** (it emitted `DeprecationWarning` since 3.9). This catches a classic bug in hand-written comparison methods:

```python
class Money:
    def __init__(self, cents: int) -> None:
        self.cents = cents

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Money):
            return NotImplemented
        return self.cents == other.cents

    def __ne__(self, other: object) -> bool:
        # ❌ `not NotImplemented` → TypeError in 3.14
        return not self.__eq__(other)

try:
    Money(1) != 5
except TypeError as exc:
    print(exc)    # NotImplemented should not be used in a boolean context

# ✅ Don't define __ne__ at all — Python derives it from __eq__ correctly
```

Three-argument `pow()` now also tries `__rpow__()` (gh-130104).[^1]

##### Floats

```python
import math

# New in 3.14: float.from_number() / complex.from_number() — numbers only, no str parsing
print(float.from_number(3))          # 3.0
try:
    float.from_number('3')           # float('3') would parse; from_number refuses
except TypeError as exc:
    print(exc)                       # must be real number, not str

# New in 3.14: thousands separators in the *fractional* part of float formatting
print(f'{1234567.891011:_._f}')      # 1_234_567.891_011
print(format(1234.5678, ',.4_f'))    # 1,234.567_8

# Always compare floats with a tolerance
print(math.isclose(0.1 + 0.2, 0.3))  # True
print(0.1 + 0.2 == 0.3)              # False
print(math.fma(2.0, 3.0, 4.0))       # 10.0 — fused multiply-add, since 3.13
```

**New deprecation in 3.14:** passing a complex number as the `real` or `imag` argument of `complex()` (`complex(1j, 2)`) emits `DeprecationWarning`; pass a single complex positional argument instead.[^1]

##### Booleans

`bool` is a subclass of `int`. `~True` is `-2` and has emitted `DeprecationWarning` since 3.12 (removal pending in 3.16) — use `not x`.[^1]

```python
print(isinstance(True, int))   # True
print(True + True)             # 2
print(not True)                # False — never ~True
```

**Falsy values:** `None`, `False`, numeric zero (`0`, `0.0`, `0j`), empty sequences/collections (`''`, `b''`, `()`, `[]`, `{}`, `set()`, `range(0)`), and objects whose `__bool__` returns `False` or `__len__` returns `0`. Testing the truth value of an `xml.etree.ElementTree.Element` is deprecated (it will always be `True` in a future release) — use `len(elem)` or `elem is not None`.

***

#### Strings

##### The Two Types

- **`str`**: Unicode code points. **`bytes`**: raw octets. Never mixed implicitly.
- Decode at input boundaries, work in `str`, encode at output boundaries.
- **Always pass `encoding='utf-8'` to `open()`** and friends. In 3.14 the default is still the locale encoding (`locale.getencoding()`); UTF-8 mode is opt-in (`-X utf8`, `PYTHONUTF8=1`) and only becomes the default in Python 3.15. Run tests with `-X warn_default_encoding` to get an `EncodingWarning` for every `open()` that omits `encoding=`.[^5]

```python
text = 'café'
data = text.encode('utf-8')          # b'caf\xc3\xa9'
print(data.decode('utf-8'))          # café
print(len(text), len(data))          # 4 5

# New in 3.14: bytes.fromhex()/bytearray.fromhex() also accept ASCII bytes
print(bytes.fromhex(b'de ad be ef')) # b'\xde\xad\xbe\xef'
```

##### String Methods Worth Knowing

```python
s = '  Hello, World!  '
print(s.strip())                               # Hello, World!
print('report.csv'.removesuffix('.csv'))       # report  (since 3.9)
print('https://x.io'.removeprefix('https://')) # x.io
print('a,b,,c'.split(','))                     # ['a', 'b', '', 'c']
print('Straße'.casefold() == 'STRASSE'.casefold())   # True — use casefold for caseless compare
print('Hello'.partition('l'))                  # ('He', 'l', 'lo')
print('42'.zfill(5))                           # 00042
print(' '.join(['a', 'b']))                    # a b
```

**Pitfall:** `lstrip`/`rstrip`/`strip` take a *set of characters*, not a prefix: `'https://x.io'.lstrip('https://')` returns `'x.io'` only by luck — use `removeprefix`.

##### f-strings (PEP 701, since 3.12)

Since 3.12, f-string expressions can reuse the outer quote type, contain backslashes and comments, and nest arbitrarily — workarounds written for 3.11 and older are no longer needed in 3.14 code:

```python
users = {'name': 'Alice'}
items = ['a', 'b']
print(f'User: {users['name']}')           # User: Alice — same quotes inside (3.12+)
joined = f'{'\n'.join(items)}'           # backslash in expression (3.12+)
print(repr(joined))                       # 'a\nb'
value = 42
print(f'{value=}')                        # value=42 — debugging only
print(f'{value:>8_}|')                    #       42|
```

Use **t-strings** (above) instead of f-strings whenever the result is fed to an interpreter (SQL, HTML, shell, templates): the consumer can then escape each value.

***

#### Type Hints in 3.14

##### The Modern Syntax (all available in 3.14)

```python
from collections.abc import Callable, Iterable, Iterator, Mapping
from typing import Literal, NamedTuple, Protocol, Self, TypedDict, override

# Unions with | (3.10+) — never Optional[X]/Union[X, Y] in new 3.14 code
def find(name: str) -> int | None: ...

# PEP 695 generics (3.12+) — no TypeVar boilerplate
def first[T](items: Iterable[T]) -> T:
    return next(iter(items))

class Stack[T]:
    def __init__(self) -> None:
        self._items: list[T] = []
    def push(self, item: T) -> Self:            # Self (3.11+)
        self._items.append(item)
        return self

# Type parameter defaults (PEP 696, 3.13+)
class Box[T = int]:
    def __init__(self, value: T) -> None:
        self.value = value

# type statement (3.12+) — lazily evaluated alias
type Json = dict[str, Json] | list[Json] | str | int | float | bool | None
type Handler[E] = Callable[[E], None]

class Base:
    def run(self) -> None: ...

class Child(Base):
    @override                                   # 3.12+: type checker verifies Base.run exists
    def run(self) -> None: ...

class Point(NamedTuple):
    x: float
    y: float

class Movie(TypedDict):
    title: str
    year: int

class SupportsClose(Protocol):
    def close(self) -> None: ...

Mode = Literal['r', 'w']
print(first([3, 1, 2]), Stack[int]().push(1)._items, Box(5).value)   # 3 [1] 5
print(Json.__value__)   # dict[str, Json] | list[Json] | str | int | float | bool | None
```

##### 3.13 Typing Additions You Can Use

```python
from typing import ReadOnly, TypedDict, TypeIs
from warnings import deprecated

def is_str_list(val: list[object]) -> TypeIs[list[str]]:   # narrows in both branches
    return all(isinstance(x, str) for x in val)

class Settings(TypedDict):
    name: ReadOnly[str]          # type checkers reject writes to this key

@deprecated('Use fetch_v2() instead')
def fetch() -> None: ...

import warnings
with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter('always')
    fetch()
print(caught[0].category.__name__, caught[0].message)   # DeprecationWarning Use fetch_v2() instead
```

##### New in 3.14: Typing Details

- `typing.get_type_hints(obj, format=...)` and `typing.evaluate_forward_ref()` (see the annotations section).[^10]
- `typing.Union` is `types.UnionType`; `Union[...]` reprs as `int | str` and is no longer cached.[^1]
- Type aliases support star unpacking: `type Row = tuple[bool, *Pair]` where `Pair` is itself a `type` alias (3.13 raises `TypeError` when the value is evaluated).[^10]
- `memoryview` is generic (`memoryview[int]`); `io.Reader`/`io.Writer` are new minimal protocols — prefer them to `typing.IO`/`TextIO`/`BinaryIO` for parameters.[^1]
- Using `super()` (or `__class__`) inside methods of a `typing.NamedTuple` subclass now raises `TypeError` at class creation (it never worked — 3.13 raised `RuntimeError`).[^10]
- **Not in 3.14 (3.15+):** `TypedDict(closed=True)`/`extra_items=`, `TypeForm`, `@disjoint_base`.

```python
import io

def copy_stream(src: io.Reader[bytes], dst: io.Writer[bytes]) -> int:   # new in 3.14
    data = src.read()
    return dst.write(data)

print(copy_stream(io.BytesIO(b'abc'), io.BytesIO()))   # 3
```

**Do not write `Optional[X]` / `Union[X, Y]` / `List[int]` / `typing.Callable` in 3.14 code.** `typing.List` etc. are deprecated aliases; `collections.abc.Callable`, `X | None` and `list[int]` are correct. `typing.ByteString` and `collections.abc.ByteString` are scheduled for removal in 3.17 — use `collections.abc.Buffer` or `bytes | bytearray | memoryview`.[^1]

***

#### Lists, Tuples and Iteration

```python
nums = [3, 1, 4, 1, 5, 9]
print(sorted(nums, reverse=True))          # [9, 5, 4, 3, 1, 1]
print(nums[::-1][:2])                      # [9, 5]
first, *rest = nums
print(first, rest)                         # 3 [1, 4, 1, 5, 9]

# New in 3.14: map(strict=True), like zip(strict=True) — catch silently truncated data
names = ['a', 'b', 'c']
scores = [1, 2]
print(list(map(lambda n, s: f'{n}={s}', names, scores)))   # ['a=1', 'b=2'] — 'c' silently lost
try:
    list(map(lambda n, s: f'{n}={s}', names, scores, strict=True))
except ValueError as exc:
    print(exc)                             # map() argument 2 is shorter than argument 1

import itertools
print(list(itertools.batched(range(7), 3)))               # [(0, 1, 2), (3, 4, 5), (6,)]
print(list(itertools.pairwise([1, 2, 3])))                # [(1, 2), (2, 3)]
try:
    list(itertools.batched(range(7), 3, strict=True))     # strict= since 3.13
except ValueError as exc:
    print(exc)                                            # batched(): incomplete batch
```

**Pitfall: list multiplication shares references** — `[[0] * 3] * 3` creates three references to one inner list; use `[[0] * 3 for _ in range(3)]`.

**Removed in 3.14:** `itertools` iterators no longer support `copy.copy()`, `copy.deepcopy()` or pickling (deprecated since 3.12). Code that pickled `itertools.count()`/`chain()` objects (e.g., checkpointing generators) now gets `TypeError`.[^1]

```python
import itertools, pickle
try:
    pickle.dumps(itertools.count(5))
except TypeError as exc:
    print(exc)    # cannot pickle 'itertools.count' object
```

##### New in 3.14: Max-Heaps in `heapq`

`heapq` gained public max-heap functions — no more negating keys or using the private `_heapify_max`:[^17]

```python
import heapq

tasks = [3, 1, 4, 1, 5]
heapq.heapify_max(tasks)
print(tasks[0])                        # 5 — largest at index 0
heapq.heappush_max(tasks, 10)
print(heapq.heappop_max(tasks))        # 10
print(heapq.heappushpop_max(tasks, 2)) # 5
print(heapq.heapreplace_max(tasks, 0)) # 4
```

Never mix min-heap and max-heap functions on the same list.

***

#### Dictionaries and `collections`

```python
from collections import ChainMap, Counter, OrderedDict, defaultdict, deque

defaults = {'host': 'localhost', 'port': 5432}
overrides = {'port': 6543}
print(defaults | overrides)            # {'host': 'localhost', 'port': 6543} — right wins

counts = Counter('abracadabra')
print(counts.most_common(2))           # [('a', 5), ('b', 2)]

graph: defaultdict[str, list[str]] = defaultdict(list)
graph['a'].append('b')

q: deque[int] = deque(maxlen=3)
q.extend([1, 2, 3, 4])
print(q)                               # deque([2, 3, 4], maxlen=3)

layered = ChainMap({'color': 'red'}, {'color': 'blue', 'user': 'guest'})
print(layered['color'], layered['user'])   # red guest
```

**Pitfall: never mutate a dict while iterating it** (`RuntimeError: dictionary changed size during iteration`). Rebuild with a comprehension.

**Point-release note:** since **3.14.3**, `defaultdict` stores the `default_factory` result with `__setitem__()` semantics that cannot overwrite a value another thread set while the factory ran (gh-142495) — relevant on the free-threaded build.[^12]

`Counter.__xor__` (symmetric difference) is **3.15** — do not use `c1 ^ c2` in 3.14 code. The `frozendict` built-in is also 3.15; in 3.14 use `types.MappingProxyType` for a read-only view.

```python
from types import MappingProxyType

config = MappingProxyType({'debug': False})
try:
    config['debug'] = True
except TypeError as exc:
    print(exc)    # 'mappingproxy' object does not support item assignment
```

##### Sets

```python
a, b = {1, 2, 3, 4}, {3, 4, 5}
print(a | b, a & b, a - b, a ^ b)     # {1, 2, 3, 4, 5} {3, 4} {1, 2} {1, 2, 5}
empty = set()                         # NOT {} — that is a dict
```

**New in 3.14:** adding an unhashable object to a set or using one as a dict key produces a clearer error: `TypeError: cannot use 'list' as a dict key (unhashable type: 'list')`. Do not match on the old message text in tests.[^1]

***

#### Control Flow

```python
# match/case (3.10+) — structural pattern matching
def describe(event: dict[str, object]) -> str:
    match event:
        case {'type': 'click', 'x': int(x), 'y': int(y)}:
            return f'click at {x},{y}'
        case {'type': 'key', 'key': str(key)} if len(key) == 1:
            return f'key {key!r}'
        case {'type': str(other)}:
            return f'unhandled {other}'
        case _:
            return 'invalid'

print(describe({'type': 'click', 'x': 1, 'y': 2}))   # click at 1,2
print(describe({'type': 'key', 'key': 'q'}))          # key 'q'

# for/else — else runs only when the loop did not break
for n in [2, 4, 6]:
    if n % 2:
        break
else:
    print('all even')                                  # all even

# Walrus (3.8+) in a read loop
import io
stream = io.BytesIO(b'x' * 100_000)
total = 0
while chunk := stream.read(65_536):
    total += len(chunk)
print(total)                                           # 100000
```

**Pitfall: `case NAME:` is a capture pattern, not a comparison.** `case RED:` binds any value to a new variable `RED`; use dotted names (`case Color.RED:`) or literals for value patterns.

**New in 3.14 error messages** help newcomers but must not be matched in tests: misspelled keywords (`whille True:` → "Did you mean 'while'?"), `elif` after `else`, statements after `else` in a conditional expression, incompatible string prefixes (`ub''`), `async with` on a synchronous context manager and vice versa.[^1]

***

#### Functions

```python
def greet(name: str, /, greeting: str = 'Hello', *, punctuation: str = '!') -> str:
    """name is positional-only, punctuation keyword-only."""
    return f'{greeting}, {name}{punctuation}'

print(greet('Alice'))                       # Hello, Alice!
print(greet('Alice', 'Hi', punctuation='.'))  # Hi, Alice.

# Mutable default arguments are shared between calls — use None
def append_to(item: int, target: list[int] | None = None) -> list[int]:
    target = [] if target is None else target
    target.append(item)
    return target

print(append_to(1), append_to(2))           # [1] [2]

# Late binding in closures
funcs = [lambda: i for i in range(3)]
print([f() for f in funcs])                 # [2, 2, 2]
funcs = [lambda i=i: i for i in range(3)]
print([f() for f in funcs])                 # [0, 1, 2]
```

##### `functools` — New in 3.14: `Placeholder`, and `partial` Is Now a Method Descriptor

```python
import functools
from functools import Placeholder, partial

# Placeholder reserves a positional slot (new in 3.14)
replace_a_with_b = partial(str.replace, Placeholder, 'a', 'b')
print(replace_a_with_b('banana'))           # bbnbnb

# reduce(initial=...) may now be passed by keyword (new in 3.14)
import operator
print(functools.reduce(operator.add, [1, 2, 3], initial=10))   # 16

# ❌ Behavior change: a partial stored as a class attribute now binds `self`
def add(a: int, b: int) -> tuple[int, int]:
    return (a, b)

class Calc:
    add_one = partial(add, 1)                  # 3.13: Calc().add_one(2) -> (1, 2)

print(Calc().add_one())                        # (1, <__main__.Calc object at 0x...>) in 3.14

class FixedCalc:
    add_one = staticmethod(partial(add, 1))    # ✅ preserve the old behavior explicitly

print(FixedCalc().add_one(2))                  # (1, 2)
```

`functools.partial` became a method descriptor in 3.14 (3.13 emitted a `FutureWarning`). Any class body that stores a `partial` as an attribute and calls it through an instance changes meaning — wrap it in `staticmethod()`.[^1][^18]

```python
import functools

@functools.cache                      # unbounded — only for bounded argument spaces
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)

@functools.lru_cache(maxsize=1024)    # bounded — default choice for unbounded inputs
def lookup(key: str) -> str:
    return key.upper()

class Dataset:
    def __init__(self, data: list[float]) -> None:
        self.data = data

    @functools.cached_property         # computed once per instance; needs a __dict__
    def mean(self) -> float:
        return sum(self.data) / len(self.data)

@functools.singledispatch
def to_text(arg: object) -> str:
    raise TypeError(f'unsupported: {type(arg).__name__}')

@to_text.register
def _(arg: int) -> str:               # dispatch from the annotation
    return f'int {arg}'

print(fib(30), lookup('a'), Dataset([1.0, 3.0]).mean, to_text(5))   # 832040 A 2.0 int 5
```

**Note:** `singledispatch`'s `register()` reads the annotation of the registered function when the decorator runs. Deferred evaluation does not help here: an annotation naming a class that is not defined yet raises `TypeError: Invalid annotation for 'x'. ForwardRef('Later', ...) is an unresolved forward reference.` Define the class first or pass it explicitly: `@to_text.register(Later)`.

##### Generators

```python
from collections.abc import Generator, Iterator

def fibonacci() -> Iterator[int]:
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

def accumulate() -> Generator[int, int, str]:
    total = 0
    while (value := (yield total)) is not None:
        total += value
    return 'done'

import itertools
print(list(itertools.islice(fibonacci(), 8)))   # [0, 1, 1, 2, 3, 5, 8, 13]
gen = accumulate()
next(gen)
print(gen.send(5), gen.send(10))                # 5 15
```

The three-argument forms `gen.throw(type, value, tb)` / `agen.athrow(type, value, tb)` are deprecated — pass an exception instance: `gen.throw(ValueError('x'))`.[^1]

***

#### Dataclasses

```python
from dataclasses import KW_ONLY, dataclass, field
import copy

@dataclass(slots=True)
class User:
    id: int
    name: str
    email: str = field(doc='Primary e-mail address; stored lower-case.')   # doc= is new in 3.14
    tags: list[str] = field(default_factory=list)
    _: KW_ONLY
    active: bool = True

    def __post_init__(self) -> None:
        self.email = self.email.lower()

@dataclass(frozen=True, slots=True)
class Coordinate:
    lat: float
    lon: float

u = User(1, 'Ann', 'ANN@EXAMPLE.COM', active=False)
print(u)                       # User(id=1, name='Ann', email='ann@example.com', tags=[], active=False)
print(User.__dataclass_fields__['email'].doc)   # Primary e-mail address; stored lower-case.
moved = copy.replace(Coordinate(1.0, 2.0), lon=3.0)   # copy.replace() is 3.13+
print(moved)                   # Coordinate(lat=1.0, lon=3.0)
```

New in 3.14 for dataclasses: `field(doc=...)` (shown by `help()`/`pydoc`), `make_dataclass(..., decorator=...)` to plug in a custom decorator, and annotation handling built on `annotationlib` — fields may use forward references without quotes.[^8]

**Critical pitfalls (unchanged):**

```python
from dataclasses import dataclass, field

try:
    @dataclass
    class Bad:
        items: list[int] = []            # ❌ mutable default
except ValueError as exc:
    print(exc)    # mutable default <class 'list'> for field items is not allowed: use default_factory

try:
    @dataclass
    class BadOrder:
        x: int = 0
        y: int                           # ❌ non-default after default
except TypeError as exc:
    print(exc)    # non-default argument 'y' follows default argument 'x'

@dataclass
class Good:
    items: list[int] = field(default_factory=list)
```

**`slots=True` + `functools.cached_property` do not mix** (no instance `__dict__`); choose one.

***

#### Classes and OOP

```python
import math
from typing import Self, override

class Shape:
    def __init__(self, color: str = 'black') -> None:
        self.color = color

    def __repr__(self) -> str:
        return f'{type(self).__name__}(color={self.color!r})'

    def area(self) -> float:
        raise NotImplementedError

    def with_color(self, color: str) -> Self:   # returns the subclass type
        clone = copy_of(self)
        clone.color = color
        return clone

class Circle(Shape):
    def __init__(self, radius: float, **kwargs: str) -> None:
        super().__init__(**kwargs)
        self.radius = radius

    @override
    def area(self) -> float:
        return math.pi * self.radius ** 2

    @classmethod
    def unit(cls) -> Self:
        return cls(1.0)

def copy_of[T](obj: T) -> T:
    import copy
    return copy.copy(obj)

c = Circle.unit().with_color('red')
print(c, round(c.area(), 4))    # Circle(color='red') 3.1416
```

**`__eq__` and `__hash__`:** defining `__eq__` sets `__hash__ = None`. Keep mutable classes unhashable; for value objects use `@dataclass(frozen=True)`, which generates a consistent pair.

##### Abstract Base Classes and Protocols

```python
from abc import ABC, abstractmethod
from typing import Protocol, runtime_checkable

class Repository(ABC):
    @abstractmethod
    def get(self, id: int) -> dict[str, object] | None: ...

    def exists(self, id: int) -> bool:
        return self.get(id) is not None

try:
    Repository()
except TypeError as exc:
    print(exc)   # Can't instantiate abstract class Repository without an implementation for abstract method 'get'

@runtime_checkable
class Closeable(Protocol):
    def close(self) -> None: ...

import io
print(isinstance(io.StringIO(), Closeable))   # True — checks method presence only, not signatures
```

##### Enums

```python
from enum import Enum, Flag, IntEnum, StrEnum, auto

class Color(Enum):
    RED = auto()
    GREEN = auto()

class Status(StrEnum):              # 3.11+: members are str
    ACTIVE = auto()                 # value 'active'

class HttpStatus(IntEnum):
    OK = 200

class Perm(Flag):
    READ = auto()
    WRITE = auto()

print(Status.ACTIVE == 'active', f'{Status.ACTIVE}')   # True active
print(Perm.READ in (Perm.READ | Perm.WRITE))            # True
print(Color['RED'], Color(1))                           # Color.RED Color.RED
```

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

def read_config(path: str) -> dict[str, object]:
    try:
        with open(path, encoding='utf-8') as f:
            return json.load(f)
    except FileNotFoundError:
        raise
    except json.JSONDecodeError as exc:
        raise ConfigError('file', f'invalid JSON in {path!r}') from exc

try:
    read_config('/nonexistent/config.json')
except FileNotFoundError as exc:
    print(type(exc).__name__)          # FileNotFoundError

# Exception notes (3.11+) — attach context without wrapping
try:
    try:
        int('N/A')
    except ValueError as exc:
        exc.add_note('while parsing field "port"')
        raise
except ValueError as exc:
    print(exc.__notes__)               # ['while parsing field "port"']
```

##### Exception Groups and `except*` (3.11+)

```python
import asyncio

async def fail(kind: type[Exception]) -> None:
    raise kind('boom')

async def main() -> None:
    try:
        async with asyncio.TaskGroup() as tg:
            tg.create_task(fail(ValueError))
            tg.create_task(fail(TypeError))
    except* ValueError as group:
        print('value errors:', len(group.exceptions))    # value errors: 1
    except* TypeError, KeyError:                          # PEP 758 form, new in 3.14
        print('type or key errors')                       # type or key errors

asyncio.run(main())
```

**Rules:**

- Catch the narrowest exception type; never bare `except:` (it also catches `KeyboardInterrupt`/`SystemExit`).
- Re-raise with bare `raise` to preserve the traceback; chain with `raise New(...) from exc`.
- Custom exceptions inherit from `Exception`, never `BaseException`.
- **New in 3.14:** `json.dumps()` and `pickle.dumps()` attach `__notes__` that pinpoint where in a nested structure serialization failed, and `pickle` raises `PicklingError` (with the original exception chained) for failures that previously raised `AttributeError`, `ValueError`, etc. — update `except` clauses around `pickle.dumps()` accordingly.[^1]

```python
import json, pickle

try:
    json.dumps({'a': [1, {2}]})
except TypeError as exc:
    print(exc.__notes__)    # ['when serializing list item 1', "when serializing dict item 'a'"]

def make_local():
    return lambda: None

try:
    pickle.dumps(make_local())
except pickle.PicklingError as exc:          # 3.13 raised AttributeError here
    print(type(exc).__name__)                # PicklingError
```

***

#### Context Managers

```python
import contextlib
import contextvars
import os
import tempfile

# Parenthesized multi-item with (official since 3.10)
with (
    tempfile.TemporaryDirectory() as tmp,
    open(os.path.join(tmp, 'out.txt'), 'w', encoding='utf-8') as out,
):
    out.write('data\n')

# contextlib.chdir (3.11+) — not thread-safe: changes the process-wide cwd
with tempfile.TemporaryDirectory() as tmp, contextlib.chdir(tmp):
    print(os.getcwd() == os.path.realpath(tmp))     # True

# New in 3.14: contextvars.Token is a context manager — resets the var on exit
request_id: contextvars.ContextVar[str] = contextvars.ContextVar('request_id', default='-')
with request_id.set('req-42'):
    print(request_id.get())        # req-42
print(request_id.get())            # -

@contextlib.contextmanager
def timed(name: str):
    import time
    start = time.perf_counter()
    try:
        yield
    finally:
        print(f'{name} took {time.perf_counter() - start:.0f}s')

with timed('noop'):
    pass                           # noop took 0s

with contextlib.suppress(FileNotFoundError):
    os.remove('/tmp/definitely-not-here.txt')
```

***

#### File I/O and `pathlib`

Always pass `encoding='utf-8'` for text files (the default is still the locale encoding in 3.14).

```python
import tempfile
from pathlib import Path

base = Path(tempfile.mkdtemp())
p = base / 'reports' / 'output.csv'
p.parent.mkdir(parents=True, exist_ok=True)
p.write_text('a,b\n', encoding='utf-8')

print(p.name, p.stem, p.suffix)              # output.csv output .csv
print(p.read_text(encoding='utf-8'), end='') # a,b
print(p.with_stem('summary').name)           # summary.csv
print(p.is_relative_to(base))                # True
print(next(base.walk())[1])                  # ['reports'] — Path.walk() is 3.12+
p.unlink(missing_ok=True)
```

##### New in 3.14: `Path.copy()`, `copy_into()`, `move()`, `move_into()`

`pathlib` can finally copy and move files **and directory trees** without `shutil`:[^19]

```python
import tempfile
from pathlib import Path

base = Path(tempfile.mkdtemp())
src = base / 'src'
(src / 'sub').mkdir(parents=True)
(src / 'a.txt').write_text('A', encoding='utf-8')
(src / 'sub' / 'b.txt').write_text('B', encoding='utf-8')

dst = src.copy(base / 'backup')               # recursive for directories; returns the new Path
print(sorted(str(x.relative_to(dst)) for x in dst.rglob('*')))   # ['a.txt', 'sub', 'sub/b.txt']

(base / 'inbox').mkdir()
copied = (src / 'a.txt').copy_into(base / 'inbox')   # -> base/inbox/a.txt
moved = copied.move(base / 'archived.txt')           # rename, or copy+delete across filesystems
print(moved.name, copied.exists())                   # archived.txt False

try:
    src.copy(base / 'backup')                 # ❌ copying a tree onto an existing directory
except FileExistsError as exc:
    print(type(exc).__name__)                 # FileExistsError
```

Details a reviewer should check: `copy()` follows symlinks by default (`follow_symlinks=True`) and does **not** preserve permissions/timestamps unless `preserve_metadata=True` (on Windows, file metadata is always preserved); copying a *file* onto an existing file overwrites it; copying a *directory* onto an existing path raises `FileExistsError`. There is no `dirs_exist_ok`/`ignore=` equivalent — keep `shutil.copytree()` when you need those.

##### New in 3.14: `Path.info` — Cached File-Type Queries

```python
import tempfile
from pathlib import Path

base = Path(tempfile.mkdtemp())
(base / 'f.txt').write_text('x', encoding='utf-8')
(base / 'd').mkdir()

# Paths yielded by iterdir() carry file-type info from the directory scan (no extra stat())
for entry in sorted(base.iterdir()):
    print(entry.name, entry.info.is_dir(), entry.info.is_file())
# d True False
# f.txt False True

info = (base / 'f.txt').info
print(info.exists())          # True
(base / 'f.txt').unlink()
print(info.exists())          # True — ❌ cached! the file is gone
print((base / 'f.txt').exists())   # False — ✅ Path.exists() queries the filesystem
```

`Path.info` (a `pathlib.types.PathInfo`) caches results; never use it to re-check state that may have changed.[^19]

##### Behavior Changes in `pathlib` (3.14)

- **`Path.exists()`, `is_file()`, `is_dir()`, `is_symlink()` etc. no longer raise `OSError`** (e.g. `PermissionError` for an unreadable parent directory) — they return `False`, consistent with `os.path.exists()`. Code that relied on the exception to detect permission problems must call `path.stat()` instead.[^19]
- `PurePath.with_suffix('.')` is now valid (a single dot is a valid suffix) instead of raising `ValueError`.
- **Removed:** extra keyword arguments to `Path(...)` (silently ignored before; `TypeError` now) and extra positional arguments to `relative_to()`/`is_relative_to()` (they used to be joined onto the first).[^1]
- **Deprecated (removal 3.19):** calling `as_uri()` on a `PurePath` — use `Path.as_uri()`. `Path.from_uri()` exists since 3.13.
- `Path.read_bytes()` now opens files unbuffered — 9–17% faster for whole-file reads.[^1]

```python
from pathlib import Path, PurePath

try:
    PurePath('/a/b').relative_to('/', 'a')      # ❌ removed in 3.14
except TypeError as exc:
    print(exc)    # PurePath.relative_to() takes 2 positional arguments but 3 were given

print(PurePath('/a/b').relative_to('/a'))       # b
print(PurePath('a.txt').with_suffix('.'))       # a.  (ValueError in 3.13)
```

##### Path Traversal Checks

```python
from pathlib import Path

def safe_read(base_dir: str, user_filename: str) -> bytes:
    base = Path(base_dir).resolve()
    target = (base / user_filename).resolve()       # resolve() defeats '..' and symlinks
    if not target.is_relative_to(base):             # lexical check — only valid after resolve()
        raise PermissionError(f'{user_filename!r} escapes {base}')
    return target.read_bytes()

try:
    safe_read('/srv/data', '../../etc/passwd')
except PermissionError as exc:
    print('blocked')                                # blocked
```

**Added in 3.14.0 with the CVE-2025-4517 fixes (also backported to 3.13.4+):** `os.path.realpath(path, strict=os.path.ALLOW_MISSING)` resolves symlinks while tolerating a missing final path, but re-raises other errors — useful for validating a path you are about to create.[^1]

```python
import os
print(os.path.realpath('/nonexistent/x', strict=os.path.ALLOW_MISSING))   # /nonexistent/x
```

##### Temporary Files and the Environment

```python
import os
import tempfile

with tempfile.NamedTemporaryFile('w', suffix='.csv', encoding='utf-8', delete_on_close=False) as tmp:
    tmp.write('col1,col2\n')
    tmp.close()                     # 3.12+: close but keep the file until the with block ends
    print(os.path.exists(tmp.name)) # True

with tempfile.TemporaryDirectory() as tmpdir:
    print(os.path.isdir(tmpdir))    # True
# NEVER tempfile.mktemp() — race-prone (TOCTOU)

# New in 3.14: os.reload_environ() — re-read the process environment into os.environ
os.putenv('DEMO_FLAG', '1')          # putenv() does NOT update os.environ
print('DEMO_FLAG' in os.environ)     # False
os.reload_environ()
print(os.environ['DEMO_FLAG'])       # 1
```

`os.reload_environ()` is not thread-safe with concurrent `os.environ` access; call it at startup or in single-threaded sections only.[^1] `os.popen()` and the `os.spawn*()` functions are **soft-deprecated** in 3.14 — use `subprocess` in new code.[^1]

**Point-release security fix (3.14.9):** `tempfile.TemporaryDirectory` cleanup could follow a symlink swapped into the tree and delete files outside the temporary directory when working around permission errors; 3.14.9 removes files relative to open directory descriptors (gh-157579).[^12]

***

#### New in 3.14: The `compression` Package and Zstandard (PEP 784)

3.14 adds a `compression` namespace: `compression.zstd` is new, and `compression.gzip`, `compression.bz2`, `compression.lzma`, `compression.zlib` re-export the existing modules. The `compression.*` names are the preferred spelling for 3.14+ code; the old top-level modules are **not** deprecated, and any deprecation or removal will come no sooner than five years after PEP 784's acceptance (April 2025).[^20]

```python
import gzip
import compression.gzip

print(compression.gzip.compress is gzip.compress)   # True — same module, new import path
```

##### `compression.zstd`

```python
from compression import zstd

data = b'hello world ' * 1000
packed = zstd.compress(data, level=10)          # levels: negative (fastest) .. 22 (smallest)
print(len(data), len(packed))                   # 12000 30
print(zstd.decompress(packed) == data)          # True
print(zstd.COMPRESSION_LEVEL_DEFAULT)           # 3

try:
    zstd.decompress(b'not zstd data')
except zstd.ZstdError as exc:                   # module-specific exception
    print(type(exc).__name__)                   # ZstdError
```

Streaming and file APIs mirror `lzma`/`bz2`:

```python
import tempfile
from pathlib import Path
from compression import zstd

path = Path(tempfile.mkdtemp()) / 'log.zst'
with zstd.open(path, 'wt', encoding='utf-8') as f:     # text mode needs an explicit encoding
    f.write('line 1\nline 2\n')
with zstd.open(path, 'rt', encoding='utf-8') as f:
    print(f.read().splitlines())                       # ['line 1', 'line 2']

# Bounded incremental decompression — the defense against decompression bombs
decomp = zstd.ZstdDecompressor()
chunk = decomp.decompress(zstd.compress(b'x' * 10_000_000), max_length=1_000_000)
print(len(chunk), decomp.needs_input)                  # 1000000 False
```

**Rule for untrusted input:** never call the one-shot `zstd.decompress()` (or `gzip.decompress()`, `zlib.decompress()`) on attacker-controlled data — the output size is unbounded. Use a decompressor object with `max_length` (or read a `ZstdFile` in fixed-size chunks) and enforce a total-size limit.

Dictionaries help with many small, similar payloads (JSON messages, log lines):

```python
from compression import zstd

samples = [f'{{"user": {i}, "status": "active"}}'.encode() for i in range(2000)]
zdict = zstd.train_dict(samples, 4096)
message = samples[5]
print(len(zstd.compress(message)), len(zstd.compress(message, zstd_dict=zdict)))   # 40 22
print(zstd.decompress(zstd.compress(message, zstd_dict=zdict), zstd_dict=zdict) == message)   # True
```

The same dictionary must be used for compression and decompression — version and ship it with the data.

##### Zstandard in `tarfile`, `zipfile` and `shutil`

```python
import io
import tarfile
import zipfile
import shutil

buf = io.BytesIO()
with tarfile.open(fileobj=buf, mode='w:zst') as tf:     # also 'r:zst', 'x:zst', 'r|zst', 'w|zst'
    info = tarfile.TarInfo('a.txt')
    info.size = 3
    tf.addfile(info, io.BytesIO(b'abc'))
buf.seek(0)
with tarfile.open(fileobj=buf, mode='r:*') as tf:       # transparent detection
    print(tf.getnames())                                # ['a.txt']

zbuf = io.BytesIO()
with zipfile.ZipFile(zbuf, 'w', compression=zipfile.ZIP_ZSTANDARD) as zf:
    zf.writestr('a.txt', b'x' * 1000)
print('zstdtar' in [name for name, _ in shutil.get_archive_formats()])   # True
```

**Caveats:**

- `compression.zstd` is an **optional module**: CPython builds without libzstd lack it (`ImportError`). Guard optional use with `try: from compression import zstd` / `except ImportError` in libraries.[^20]
- **`ZIP_ZSTANDARD` archives are not readable by older Python versions** or many unzip tools; use deflate for archives meant for wide distribution.
- **Point-release security fix (3.14.8):** `zipfile` now bounds how much data it decompresses per read for bzip2, LZMA and Zstandard members, matching deflate — on 3.14.0–3.14.7 a tiny member could force an unbounded allocation (gh-156002).[^12]

***

#### `async`/`await` and `asyncio`

##### New in 3.14: `asyncio.get_event_loop()` No Longer Creates a Loop

`asyncio.get_event_loop()` now raises `RuntimeError` when there is no current event loop, instead of implicitly creating one (3.12–3.13 emitted a `DeprecationWarning` and created it).[^21]

```python
import asyncio

async def main() -> str:
    return 'done'

# ❌ Pre-3.10 idiom — RuntimeError in 3.14 when no loop has been set
try:
    loop = asyncio.get_event_loop()
    loop.run_until_complete(main())
except RuntimeError as exc:
    print(exc)                       # There is no current event loop in thread 'MainThread'.

# ✅ Entry point
print(asyncio.run(main()))           # done

# ✅ Several top-level calls sharing one loop
with asyncio.Runner() as runner:
    print(runner.run(main()))        # done
    print(runner.run(main()))        # done

# ✅ Inside a coroutine, get the running loop
async def needs_loop() -> bool:
    return asyncio.get_running_loop() is asyncio.get_event_loop()   # fine while a loop runs

print(asyncio.run(needs_loop()))     # True
```

For a server that should run forever: `async def main(): await start_server(); await asyncio.Event().wait()` and `asyncio.run(main())`.[^1]

##### New in 3.14: Policy System Deprecated (Removal in 3.16)

`asyncio.get_event_loop_policy()`, `set_event_loop_policy()`, `AbstractEventLoopPolicy`, `DefaultEventLoopPolicy` and the Windows policy classes emit `DeprecationWarning` and are scheduled for removal in **3.16**. Choose the loop implementation with `loop_factory`:[^1]

```python
import asyncio

async def main() -> str:
    return 'ok'

# ❌ asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())   # deprecated
# ✅ asyncio.run(loop_factory=...) is 3.12+; asyncio.Runner(loop_factory=...) is 3.11+
print(asyncio.run(main(), loop_factory=asyncio.SelectorEventLoop))   # ok

# uvloop and similar: asyncio.run(main(), loop_factory=uvloop.new_event_loop)
```

Also deprecated in 3.14 (removal 3.16): `asyncio.iscoroutinefunction()` — use `inspect.iscoroutinefunction()`.[^1]

##### Structured Concurrency (3.11+)

```python
import asyncio

async def fetch(i: int) -> int:
    await asyncio.sleep(0.01)
    return i * 2

async def main() -> list[int]:
    async with asyncio.TaskGroup() as tg:                       # cancels siblings on failure
        tasks = [tg.create_task(fetch(i), name=f'fetch-{i}') for i in range(5)]
    async with asyncio.timeout(1.0):                            # 3.11+, replaces wait_for()
        await asyncio.sleep(0)
    return [t.result() for t in tasks]

print(asyncio.run(main()))    # [0, 2, 4, 6, 8]
```

```python
import asyncio

sem = asyncio.Semaphore(10)

async def bounded(i: int) -> int:
    async with sem:                     # cap concurrency
        await asyncio.sleep(0)
        return i

async def worker() -> None:
    try:
        await asyncio.sleep(10)
    except asyncio.CancelledError:
        # cleanup...
        raise                           # ✅ always re-raise CancelledError

async def main() -> None:
    print(sum(await asyncio.gather(*(bounded(i) for i in range(100)))))   # 4950
    task = asyncio.create_task(worker())
    await asyncio.sleep(0)
    task.cancel()
    try:
        await task
    except asyncio.CancelledError:
        print('cancelled')                                                # cancelled

asyncio.run(main())
```

**New in 3.14:** `create_task()` (module function, `loop.create_task()`, `TaskGroup.create_task()`) forwards **arbitrary keyword arguments** to the `Task` constructor or custom task factory — e.g. `eager_start=True`. Custom task factories must accept `**kwargs`.[^1]

**Blocking calls:** never call blocking I/O or CPU-heavy code in a coroutine; use `await asyncio.to_thread(func, *args)`. Keep strong references to fire-and-forget tasks (the loop holds only weak references) — or better, use a `TaskGroup`.

##### New in 3.14: Asyncio Introspection

Inspect a **running process** without modifying it:[^1][^22]

```bash
python -m asyncio ps 12345      # flat table: task ids, names, coroutine stacks, awaiters
python -m asyncio pstree 12345  # tree of tasks and what each one awaits
```

These use the PEP 768 remote-debugging machinery (see Debugging below) and need the same OS permissions as attaching a debugger. Name your tasks (`name=`) — the output is only as readable as the names. In-process, `asyncio.print_call_graph()` and `asyncio.capture_call_graph()` show who awaits the current task:

```python
import asyncio
import io

async def leaf() -> str:
    buf = io.StringIO()
    asyncio.print_call_graph(file=buf)     # new in 3.14
    return buf.getvalue()

async def main() -> None:
    async with asyncio.TaskGroup() as tg:
        task = tg.create_task(leaf(), name='leaf-task')
    print(task.result().splitlines()[0])   # * Task(name='leaf-task', id=0x...)

asyncio.run(main())
```

For interactive debugging of coroutines, **`pdb.set_trace_async()`** (new in 3.14) supports `await` at the prompt, and `$_asynctask` refers to the current task.[^1]

**Point-release security fix (3.14.8):** `loop.create_connection()` / `loop.start_tls()` now raise `ValueError` when given an `SSLContext` with `check_hostname=True` but no `server_hostname`, instead of completing a handshake that skipped hostname verification (gh-156793); `ssl.SSLContext.wrap_bio()` got the same validation.[^21][^12]

**Performance:** asyncio is 10–20% faster on standard benchmarks in 3.14 and has first-class support for the free-threaded build (several event loops in different threads scale linearly).[^1]

***

#### Concurrency

##### The GIL, and the Officially Supported Free-Threaded Build (PEP 779)

The default `python3.14` build still has a Global Interpreter Lock: threads help I/O-bound work, not CPU-bound pure-Python code. **New in 3.14:** the free-threaded build (`python3.14t`, PEP 703) is **officially supported** (PEP 779, "phase II") — no longer experimental, but still optional and not the default. Its single-threaded overhead dropped to roughly 5–10% because the specializing adaptive interpreter is now enabled in it.[^7][^1]

```python
import sys
import sysconfig

# Is this a free-threaded build? (recommended check for build-configuration decisions)
print(sysconfig.get_config_var('Py_GIL_DISABLED'))   # 1 on python3.14t, 0 on the default build
# Is the GIL actually disabled right now? (it can be re-enabled at runtime)
print(sys._is_gil_enabled())                         # False on 3.14t, True on the default build
```

Rules for free-threaded deployments:[^23]

- **Extensions decide.** Importing a C extension that is not marked free-threading-safe **re-enables the GIL** for the whole process (with a `RuntimeWarning`). Check `sys._is_gil_enabled()` at startup and in health checks; `PYTHON_GIL=0` / `-X gil=0` forces it off (at your own risk). Wheels for the free-threaded build carry the `cp314t` ABI tag — they are separate builds.
- **Built-in containers are internally locked, your invariants are not.** `list.append` won't corrupt memory, but check-then-act sequences (`if key not in d: d[key] = ...`) race exactly as before. Use `threading.Lock`; don't rely on the GIL for atomicity anywhere.
- **Iterators are not thread-safe**: sharing one iterator/generator between threads can yield duplicates or skip items.
- **Memory**: higher usage (larger object headers for non-GC objects, all interned strings immortal, mimalloc, deferred freeing).
- **Defaults differ**: `sys.flags.context_aware_warnings` and `sys.flags.thread_inherit_context` are **on** by default in 3.14t and **off** in the default build (see below).
- The experimental JIT is not available in free-threaded builds.[^1]

```python
import threading

class Counter:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self.value = 0

    def increment(self) -> None:
        with self._lock:            # ✅ required for correctness on any build
            self.value += 1

c = Counter()
threads = [threading.Thread(target=lambda: [c.increment() for _ in range(10_000)]) for _ in range(4)]
for t in threads:
    t.start()
for t in threads:
    t.join()
print(c.value)                      # 40000
```

##### New in 3.14: Multiple Interpreters — `concurrent.interpreters` (PEP 734)

Subinterpreters are isolated Python runtimes inside one process, each with its own GIL (since 3.12): "the isolation of processes with the efficiency of threads".[^24][^25]

```python
from concurrent import interpreters

def square_sum(n: int) -> int:
    return sum(i * i for i in range(n))

def boom() -> None:
    raise ValueError('bad input')

interp = interpreters.create()
print(interp.call(square_sum, 1000))          # 332833500 — runs in the current thread

queue = interpreters.create_queue()           # cross-interpreter queue
interp.prepare_main(queue=queue)              # bind objects in the interpreter's __main__
interp.exec('queue.put(40 + 2)')
print(queue.get(timeout=5))                   # 42

try:
    interp.call(boom)
except interpreters.ExecutionFailed as exc:   # uncaught exception in the other interpreter
    print(exc.excinfo.type.__name__, exc.excinfo.msg)   # ValueError bad input

try:
    import threading
    interp.prepare_main(lock=threading.Lock())
except interpreters.NotShareableError as exc: # subclass of TypeError
    print(type(exc).__name__)                 # NotShareableError
interp.close()
```

Most objects are **copied via pickle** when crossing interpreters; `None`, `bool`, `bytes`, `str`, `int`, `float` and tuples of those are shared or copied efficiently; only `memoryview` and interpreter `Queue`s actually share mutable data. `call_in_thread()` runs a function in the interpreter on a new thread.[^25]

`concurrent.futures.InterpreterPoolExecutor` (new in 3.14) wraps this in the familiar executor API — a `ThreadPoolExecutor` whose workers each own an interpreter, so CPU-bound work runs in parallel on the default (GIL) build:[^26]

```python
from concurrent.futures import InterpreterPoolExecutor

def cpu_bound(n: int) -> int:
    return sum(i * i for i in range(n))

def fails() -> None:
    raise ValueError('bad')

if __name__ == '__main__':
    with InterpreterPoolExecutor(max_workers=4) as pool:
        print(list(pool.map(cpu_bound, [10, 100, 1000])))   # [285, 328350, 332833500]
        try:
            pool.submit(fails).result()
        except ValueError as exc:                           # original exception preserved
            print(exc, type(exc.__cause__).__name__)        # bad ExecutionFailed
```

**Limitations to flag in review:** many third-party C extensions do not support subinterpreters yet (all stdlib extension modules do); start-up and per-interpreter memory are not optimized; arguments/results are pickled; module-level state (caches, connections) is per interpreter. Do not use subinterpreters as a security boundary — they share one address space.[^25]

##### `concurrent.futures` and `multiprocessing` — New Default Start Method

**Behavior change: on Linux and other non-macOS POSIX platforms, the default start method for `multiprocessing` and `ProcessPoolExecutor` is now `'forkserver'` instead of `'fork'`.** Windows and macOS already used `'spawn'`.[^27][^1] Workers no longer inherit a copy of the parent's current memory; they are forked from a clean server process that preloads your main module.

```python
import multiprocessing as mp
from concurrent.futures import ProcessPoolExecutor

CONFIG = {'mode': 'default'}

def read_mode(_: int) -> str:
    return CONFIG['mode']

def main() -> None:
    CONFIG['mode'] = 'configured-at-runtime'      # mutated in the parent only
    with ProcessPoolExecutor(max_workers=1) as pool:
        print(mp.get_start_method(), list(pool.map(read_mode, [0])))
        # forkserver ['default']   ← with 'fork' (3.13 on Linux) this was ['configured-at-runtime']
    fork = mp.get_context('fork')                 # ✅ opt back in explicitly if you really must
    with ProcessPoolExecutor(max_workers=1, mp_context=fork) as pool:
        print(list(pool.map(read_mode, [0])))     # ['configured-at-runtime']

if __name__ == '__main__':                        # REQUIRED with forkserver/spawn
    main()
```

What breaks after upgrading on Linux:

- **Runtime-mutated globals** (config loaded in `main()`, registries filled after import) are not visible in workers. Pass state explicitly as arguments or via the pool `initializer=`/`initargs=`.
- **Unpicklable callables/arguments** (lambdas, nested functions, open connections, locks) now fail with `PicklingError`.
- **Missing `if __name__ == '__main__':` guard** — the main module is imported by the forkserver.
- Prefer a context object (`mp.get_context('fork')`) over the global `mp.set_start_method()`; `fork` in a multi-threaded process is unsafe (3.12+ warns).

**Point-release notes:** 3.14.9 falls back to `'spawn'` when there is no writable temporary directory (read-only file systems), where `forkserver` previously failed with `OSError` (gh-155717); 3.14.3 fixed `sys.argv` being empty while the forkserver preloads `__main__` (regression in 3.14.1); 3.14.4 fixed a `BrokenPipeError` with a very large `sys.argv` (regression in 3.14.3) and propagates all `-X` options to child processes.[^28][^12]

Other 3.14 executor additions:[^26]

```python
from concurrent.futures import ProcessPoolExecutor, ThreadPoolExecutor
import itertools

def square(x: int) -> int:
    return x * x

if __name__ == '__main__':
    # Executor.map(buffersize=...) bounds in-flight work — safe with huge/infinite inputs
    with ThreadPoolExecutor(max_workers=4) as pool:
        results = pool.map(square, itertools.count(), buffersize=8)
        print(list(itertools.islice(results, 5)))      # [0, 1, 4, 9, 16]

    pool = ProcessPoolExecutor(max_workers=2)
    print(pool.submit(square, 7).result())             # 49
    pool.terminate_workers()                           # new: SIGTERM all workers, then shutdown
```

Without `buffersize`, `Executor.map()` submits **every** item up front — with `itertools.count()` it never returns. `ProcessPoolExecutor.kill_workers()` sends `SIGKILL`; `multiprocessing.Process.interrupt()` (new) sends `SIGINT` so the child's `finally` blocks run. The `forkserver` control socket is now authenticated (security hardening).[^1]

##### `threading`

```python
import threading
from concurrent.futures import ThreadPoolExecutor

results: list[int] = []
lock = threading.Lock()

def work(n: int) -> None:
    with lock:
        results.append(n * n)

with ThreadPoolExecutor(max_workers=4, thread_name_prefix='worker') as pool:
    list(pool.map(work, range(10)))
print(sorted(results)[:3])        # [0, 1, 4]
```

**New in 3.14:** `Thread.start()` sets the **operating-system thread name** from `Thread.name` (visible in `top -H`, `py-spy`, debuggers) — give threads meaningful names. Passing arguments to `threading.RLock()` is deprecated (they were ignored; removed in 3.15).[^1]

##### New in 3.14: Concurrency-Safe Warnings Control

`warnings.catch_warnings()` historically mutated the **process-global** filter list — not thread-safe and not task-safe. In 3.14 it can use a context variable instead when `sys.flags.context_aware_warnings` is true (`-X context_aware_warnings` / `PYTHON_CONTEXT_AWARE_WARNINGS=1`); this is the default only on the free-threaded build. The companion flag `thread_inherit_context` makes new threads start with a copy of the caller's `contextvars` context.[^29]

```python
import sys
print(sys.flags.context_aware_warnings, sys.flags.thread_inherit_context)   # 0 0 on the default build
```

On the default build, do not use `catch_warnings()` concurrently from several threads or tasks (e.g. in parallel tests) unless the flag is enabled.

***

#### New in 3.14: Debugging and Observability

##### Safe External Debugger Interface (PEP 768)

A process can now execute a Python file inside another running Python process at the next safe point, with no overhead when unused:[^30][^31]

```bash
python -m pdb -p 12345           # attach pdb to a running process (new in 3.14)
python -m asyncio pstree 12345   # inspect its asyncio tasks
```

```python
import sys

# Run /tmp/probe.py inside process 12345 (requires the same rights as a debugger: ptrace on Linux)
# sys.remote_exec(12345, '/tmp/probe.py')
print(callable(sys.remote_exec))    # True
```

Operational facts for production:

- **Attach is cooperative:** the code runs when the target next executes bytecode or handles a signal. A process blocked in a long system call (`select`, `epoll`, a C library call) only runs it once that call returns.
- **Permissions:** the caller needs the same OS rights as a debugger — on Linux, ptrace access to the target (typically root or `CAP_SYS_PTRACE`; some distributions allow the same uid, depending on the ptrace scope setting); comparable privileges on macOS and Windows.[^30]
- **Disable it where untrusted local users exist:** `PYTHON_DISABLE_REMOTE_DEBUG=1`, `-X disable-remote-debug`, or build with `--without-remote-debug`. Like every `PYTHON*` variable, the environment variable is ignored under `-E`/`-I` — use the `-X` option there. With it disabled, `sys.remote_exec()` against that process raises `RuntimeError: Remote debugging is not enabled in the remote process`.[^1]
- `sys.remote_exec()` returns immediately with no completion signal; the caller must keep the script file in place until it has run. Caller and target must run the same `major.minor` CPython version.
- Auditing: the caller raises the audit event `sys.remote_exec`; the **target** raises `cpython.remote_debugger_script` before running the script — an audit hook in the target can log or block injected code.[^31]
- **Point-release hardening (3.14.5):** the remote-debugging reader validates offset tables read from the target before using them (gh-148178).[^12]

##### Other New Debugging Aids

- `faulthandler.dump_c_stack()` and `faulthandler.enable(c_stack=True)` print the **C** stack on crashes (where supported).[^1]
- `python -X importtime=2` also reports modules that were already loaded ("cached") — useful for start-up profiling.
- `python -c` now **dedents** its argument automatically, so indented multi-line snippets in shell scripts work.
- `breakpoint()`/`pdb.set_trace()` reuse the most recent `Pdb` instance (display/commands persist); the `pdb` CLI and `breakpoint()` use the low-overhead `sys.monitoring` backend.
- The REPL has **syntax highlighting** and import auto-completion; `unittest`, `argparse` help, `json` CLI and `calendar` output are colored. Disable color in CI logs with `NO_COLOR=1` or `PYTHON_COLORS=0`; disable the new REPL with `PYTHON_BASIC_REPL=1`.[^1]

```python
import faulthandler
print(hasattr(faulthandler, 'dump_c_stack'))   # True
```

***

#### Logging

```python
import logging
import logging.handlers
import queue

logger = logging.getLogger(__name__)

def setup_logging(level: str = 'INFO') -> None:
    logging.basicConfig(
        level=level,
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        force=True,
    )

setup_logging()

# %-style lazy formatting — never f-strings or t-strings in log calls
items = 3
logger.info('Loaded %d records', items)
try:
    1 / 0
except ZeroDivisionError:
    logger.exception('Computation failed')     # includes the traceback

# New in 3.14: QueueListener is a context manager (start on enter, stop on exit)
log_queue: queue.Queue[logging.LogRecord] = queue.Queue()
console = logging.StreamHandler()
with logging.handlers.QueueListener(log_queue, console):
    logging.getLogger('worker').addHandler(logging.handlers.QueueHandler(log_queue))
    logging.getLogger('worker').warning('queued message')
```

3.14 changes: `QueueListener.start()` raises `RuntimeError` if the listener is already started; custom handlers that take a `strm` argument are deprecated (use `stream`, removal 3.16); `Logger.warn()` remains deprecated — use `warning()`.[^1]

**Security:** logs may contain request data, prompts and tokens. Never log secrets; log identifiers instead.

***

#### Security

##### New in 3.14: `tarfile` Extracts with the `'data'` Filter by Default (PEP 706)

`TarFile.extract()`/`extractall()` without `filter=` now use the **`'data'` filter**: leading `/` is stripped from member names; members that would land outside the destination (`..` escapes), links that are absolute or point outside the destination, and device files/pipes are refused; setuid/setgid/sticky and group/other-write bits are cleared, and ownership is not restored. In 3.12–3.13 the default was `fully_trusted` (with a `DeprecationWarning` announcing this change).[^32][^33]

```python
import io
import tarfile
import tempfile

buf = io.BytesIO()
with tarfile.open(fileobj=buf, mode='w') as tf:
    evil = tarfile.TarInfo('../escape.txt')
    evil.size = 2
    tf.addfile(evil, io.BytesIO(b'hi'))
    link = tarfile.TarInfo('passwd-link')
    link.type = tarfile.SYMTYPE
    link.linkname = '/etc/passwd'
    tf.addfile(link)

for name in ('../escape.txt', 'passwd-link'):
    buf.seek(0)
    with tarfile.open(fileobj=buf) as tf, tempfile.TemporaryDirectory() as dest:
        try:
            tf.extract(name, dest)                    # 3.13: silently extracted outside dest!
        except tarfile.FilterError as exc:
            print(type(exc).__name__)
# OutsideDestinationError
# AbsoluteLinkError
```

Reviewer rules:

- **Always pass `filter='data'` explicitly** in code that may also run on older Pythons (the default only changed in 3.14), and for untrusted archives.
- `filter='tar'` keeps Unix-specific features (e.g. permissions) but still blocks the dangerous ones; `filter='fully_trusted'` must be justified in review.
- Catch `tarfile.FilterError` (base of `OutsideDestinationError`, `AbsoluteLinkError`, `AbsolutePathError`, `SpecialFileError`, ...). `tarfile.LinkFallbackError` (added with the 3.14.0 CVE fixes, also present in patched older branches) is raised when a link that would be replaced by a copy of another member is rejected.
- The `'data'` filter is a security boundary that has needed repeated fixes. **Run the latest 3.14.x:** 3.14.0 shipped the CVE-2025-4138/4330/4435 and CVE-2024-12718 fixes; later releases closed further bypasses — 3.14.6 (link targets validated with the normalized on-disk value, gh-149486), 3.14.7 (symlinks pointing outside the destination, a bypass of CVE-2025-4330, gh-151558), 3.14.8 (directories created outside the destination for names such as `../evil/../dest/sub/file`, gh-155999; `..` components normalized), 3.14.9 (filter result honored when a link falls back to extracting a member, gh-157265).[^12]

##### Security Fixes in 3.14 Point Releases

Selected from the official changelog — the reason "latest patch release" is a hard requirement:[^12]

| Release | Fix |
|---|---|
| 3.14.1 | Memory-exhaustion DoS fixes in `http.client` and `plistlib`; linear-time `email` parameter parsing; quadratic `os.path.expandvars()` fixed; zip64 end-of-central-directory consistency check in `zipfile` |
| 3.14.2 | Quadratic node-ID cache clearing in `xml.dom.minidom`; memory DoS in the `http.server` CGI handler on Windows (3.14.2 was an expedited release that also fixed 3.14.1 regressions) |
| 3.14.3 | `email` header injection via folded comments; control characters rejected in `http.cookies.Morsel`, `wsgiref.headers` and `data:` URL media types |
| 3.14.4 | Unbounded C recursion in `xml.parsers.expat` (CVE-2026-4224); control characters rejected in `Morsel.update()`/`js_output()` (CVE-2026-3644); `.pyc` loading via `io.open_code()` (CVE-2026-2297); HTTP header injection via `wsgiref` status; leading dashes rejected in `webbrowser.open()` URLs |
| 3.14.5 | `shutil.unpack_archive()` ZIP drive-prefix path traversal on Windows; quadratic backtracking in `configparser`; CR/LF rejected in `HTTPConnection.set_tunnel()` headers; dangling pointer in decompressors on `MemoryError`; libexpat 2.8.0 |
| 3.14.6 | `shutil.move()` symlink bypass of the "destination inside source" guard; `bz2` decompressor reuse after an error; quadratic `unicodedata.normalize()`; `tarfile` data-filter bypasses; libexpat 2.8.1 (CVE-2026-45186) |
| 3.14.7 | Quadratic `html.parser` parsing of unterminated constructs; quadratic `ElementTree` XPath index predicates; `http.client` limits on chunked trailers and interim 1xx responses; `tarfile` symlink-escape bypass |
| 3.14.8 | `asyncio`/`ssl` hostname verification silently skipped when `server_hostname` was missing; `HTTPPasswordMgr` credentials leaking from HTTPS to HTTP URLs (CVE-2026-15806); `ssl` SNI-callback crash (CVE-2026-19445); bounded `zipfile` decompression for bzip2/LZMA/Zstandard members; `tarfile` directory-escape fix |
| 3.14.9 | `tempfile.TemporaryDirectory` cleanup symlink race; `tarfile` link-fallback filter fix; `mimetypes` no longer reads `mimetypes.knownfiles` by default on Windows (registry only); libexpat 2.9.0; bundled pip 26.2.2 |

##### Other Security-Relevant Changes in 3.14

- **PGP signatures are discontinued** for 3.14+ release artifacts (PEP 761). Verify downloads with **Sigstore** (`sigstore verify identity ...`), which CPython has used since 3.11.[^34]
- `configparser` refuses to **write** keys it could not read back (keys containing delimiters, or starting like a section header) — `configparser.InvalidWriteError`.
- The `multiprocessing` **forkserver control socket is authenticated** instead of relying only on filesystem permissions.
- `hmac` has a built-in, formally verified HACL* implementation used when OpenSSL lacks HMAC.[^1]
- `http.server` can serve **HTTPS** (`python -m http.server --tls-cert cert.pem --tls-key key.pem`) — still a development server, never production.
- `urllib.request` supports SHA-256 HTTP digest authentication (RFC 7616).
- Remote debugging (above) is a new local attack surface: disable it with `PYTHON_DISABLE_REMOTE_DEBUG=1` on multi-tenant hosts.

```python
import configparser
import io

cfg = configparser.ConfigParser()
cfg['section'] = {'user=admin': 'x'}          # a key containing the delimiter
try:
    cfg.write(io.StringIO())
except configparser.InvalidWriteError as exc:
    print(exc)        # Cannot write key user=admin; contains delimiter =
```

##### Secrets, Hashing and Tokens

```python
import hashlib
import hmac
import secrets

token = secrets.token_urlsafe(32)                 # session tokens, password-reset links
api_key = secrets.token_hex(32)
print(len(api_key))                               # 64

stored, provided = token, token
print(secrets.compare_digest(stored, provided))   # True — constant-time

# Password hashing: use a slow KDF (hashlib.scrypt / pbkdf2_hmac, or argon2 via a library)
salt = secrets.token_bytes(16)
digest = hashlib.scrypt(b'correct horse', salt=salt, n=2**14, r=8, p=1)
print(len(digest))                                # 64

signature = hmac.new(b'server-key', b'payload', 'sha256').hexdigest()
print(hmac.compare_digest(signature, hmac.new(b'server-key', b'payload', 'sha256').hexdigest()))  # True
```

Never use `random` (Mersenne Twister, predictable) or `uuid.uuid1()/uuid6()/uuid8()` (time/MAC or non-cryptographic randomness) for secrets; `uuid.uuid4()` and `uuid.uuid7()` draw from `os.urandom()`, but a UUIDv7 also **exposes its creation time** (see UUIDs below).[^35]

##### SQL, Subprocess and XML

```python
import sqlite3
import subprocess
import sys

conn = sqlite3.connect(':memory:')
conn.execute('CREATE TABLE users (id INTEGER, name TEXT)')
user_id = '1 OR 1=1'
# ❌ f'SELECT * FROM users WHERE id = {user_id}'  — injection
print(conn.execute('SELECT * FROM users WHERE id = ?', (user_id,)).fetchall())   # [] ✅

# New in 3.14: a *sequence* with *named* placeholders is a ProgrammingError
try:
    conn.execute('SELECT :name', ('x',))
except sqlite3.ProgrammingError as exc:
    print(type(exc).__name__)               # ProgrammingError
print(conn.execute('SELECT :name', {'name': 'x'}).fetchall())   # [('x',)]
conn.close()

# subprocess: list form, no shell, timeout, check
result = subprocess.run([sys.executable, '-c', 'print("hi")'],
                        capture_output=True, encoding='utf-8', timeout=30, check=True)
print(result.stdout.strip())                # hi
```

Use `defusedxml` for untrusted XML (entity-expansion and external-entity attacks); the stdlib parsers depend on the bundled libexpat version, which 3.14 point releases keep updating.

***

#### JSON and Serialization

```python
import json
from datetime import UTC, datetime

data = {'name': 'Alice', 'tags': ['admin', 'user']}
text = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False)
print(json.loads(text) == data)                     # True

class AppEncoder(json.JSONEncoder):
    def default(self, obj: object) -> object:
        if isinstance(obj, datetime):
            return obj.isoformat()
        if isinstance(obj, set):
            return sorted(obj)
        return super().default(obj)

print(json.dumps({'ts': datetime(2025, 10, 7, tzinfo=UTC)}, cls=AppEncoder))
# {"ts": "2025-10-07T00:00:00+00:00"}
```

**New in 3.14:** `python -m json` is the preferred CLI (`python -m json.tool` still works but is soft-deprecated), with colored output; serialization errors carry `__notes__` locating the failing element (see Exception Handling).[^1]

##### Pickle — New in 3.14: Default Protocol 5

```python
import pickle

print(pickle.DEFAULT_PROTOCOL)        # 5 (was 4 since 3.8)
blob = pickle.dumps({'a': 1})
print(pickle.loads(blob))             # {'a': 1}
```

The default protocol is now **5** (PEP 574, out-of-band buffers, available since 3.8). Pickles written by 3.14 with the default protocol **cannot be read by Python 3.7 or older**; pass `protocol=4` if a legacy consumer reads them.[^36] Failures now raise `PicklingError` with the original exception chained, and errors include notes locating the unpicklable item.[^1]

**Point-release note (3.14.7):** the C unpickler now enforces frame boundaries like the pure-Python implementation, raising `UnpicklingError` for malformed frame layouts it used to accept (gh-154848).[^12]

**SECURITY:** never unpickle untrusted data — `pickle.loads()` executes arbitrary code. Use JSON, MessagePack or protobuf across trust boundaries.

***

#### CSV

```python
import csv
import io

buf = io.StringIO(newline='')
writer = csv.DictWriter(buf, fieldnames=['name', 'note'])
writer.writeheader()
writer.writerow({'name': 'a,b', 'note': 'say "hi"'})
print(repr(buf.getvalue()))     # 'name,note\r\n"a,b","say ""hi"""\r\n'

buf.seek(0)
print(list(csv.DictReader(buf)))   # [{'name': 'a,b', 'note': 'say "hi"'}]
```

Open CSV files with `open(path, newline='', encoding='utf-8')`. **CSV injection:** when exporting user data for spreadsheets, neutralize cells beginning with `=`, `+`, `-`, `@`.

***

#### Regular Expressions

```python
import re

DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')
m = DATE_RE.fullmatch('2025-10-07')
print(m['year'] if m else None)                      # 2025

# New in 3.14: \z — synonym of Python's \Z (absolute end of string)
print(bool(re.search(r'\d+\z', '123')))              # True
print(bool(re.search(r'\d+\z', '123\n')))            # False — no trailing newline allowed
print(bool(re.search(r'\d+$', '123\n')))             # True  — $ also matches before a final \n

# New in 3.14: \B matches the empty string (always the opposite of \b)
print(re.findall(r'\B', ''))                         # ['']  (3.13: [])
```

- In Python, `\Z` has always meant "very end of string"; `\z` was added because in most other regex engines `\Z` allows a trailing newline. Prefer `\z` in new patterns for unambiguous, portable intent. **Use `\z` or `fullmatch()` instead of `$` when validating input** — `$` accepts a trailing newline (`'admin\n'` passes `^admin$`).
- `re.prefixmatch()` is **3.15** — keep using `re.match()`/`fullmatch()`/`search()` in 3.14 code.
- The undocumented `sre_compile`/`sre_parse`/`sre_constants` modules are deprecated and **removed in 3.15** — import only `re`.[^1][^5]
- `re.Scanner` (undocumented): a 3.14.1 change was reverted in 3.14.2; capturing groups inside scanner patterns still work but can give wrong results and will be forbidden in a future version (gh-140797).[^12]

***

#### New in 3.14: UUID Versions 6, 7 and 8 (RFC 9562)

```python
import uuid

u7 = uuid.uuid7()                       # 48-bit Unix ms timestamp + counter + random
print(u7.version, len(str(u7)))         # 7 36
print(all(uuid.uuid7() < uuid.uuid7() for _ in range(1000)))   # True — monotonic in-process
print(uuid.NIL, uuid.MAX)
# 00000000-0000-0000-0000-000000000000 ffffffff-ffff-ffff-ffff-ffffffffffff
print(isinstance(u7.time, int))         # True — for v7, milliseconds since the Unix epoch
```

- **Use `uuid7()` for database primary keys**: it is time-ordered, so B-tree inserts stay localized (unlike random `uuid4()`), and its random parts come from `os.urandom()`.[^35]
- **A UUIDv7 reveals when it was created** (`u.time`). Do not use it where creation time must stay secret, and never as an unguessable token — use `secrets.token_urlsafe()`.
- `uuid6()` is a field-reordered `uuid1()` (time + node; leaks the MAC address unless you pass `node=`). `uuid8()` is a custom-layout format; its default randomness is **not** cryptographically secure.[^35]
- `python -m uuid -u uuid7 -C 5` generates several at once (`--count` is new).

***

#### Memory Management and Garbage Collection

##### The 3.14 GC Story: Incremental, Then Reverted (3.14.5)

- **3.14.0–3.14.4** shipped a new **incremental** cyclic garbage collector (two generations; `gc.collect(1)` performed one increment; `gc.get_threshold()` returned `(2000, 10, 0)` and the third threshold was ignored).
- Because of **significant memory-pressure reports in production**, **3.14.5 reverted** the default build to the **3.13 generational collector** (`gc.get_threshold()` is `(2000, 10, 10)` again, three generations, `gc.collect(1)` collects generation 1). The free-threaded build's collector was not affected.[^1][^12]

```python
import gc
print(gc.get_threshold())     # (2000, 10, 10) on 3.14.5+ ((2000, 10, 0) on 3.14.0–3.14.4)
print(len(gc.get_count()))    # 3
```

Review consequences: memory/latency measurements taken on 3.14.0–3.14.4 do not carry over to 3.14.5+; code tuning `gc.set_threshold()` behaves as on 3.13 again. **Upgrade services still on 3.14.0–3.14.4.**

##### Practical Memory Tools

```python
import sys
import tracemalloc
import weakref

tracemalloc.start()
data = [bytes(1000) for _ in range(1000)]
current, peak = tracemalloc.get_traced_memory()
print(current > 1_000_000)          # True
tracemalloc.stop()

class Node:
    __slots__ = ('value', '__weakref__')   # __slots__ saves memory per instance
    def __init__(self, value: int) -> None:
        self.value = value

cache: weakref.WeakValueDictionary[int, Node] = weakref.WeakValueDictionary()
n = Node(1)
cache[1] = n
del n
print(1 in cache)                   # False — no strong reference kept it alive
```

- **New in 3.14:** the interpreter borrows references internally where safe, so `sys.getrefcount()` can return **smaller values than in 3.13**. Never assert on exact reference counts in tests; C extensions must not use `Py_REFCNT(op) == 1` to detect temporaries (use `PyUnstable_Object_IsUniqueReferencedTemporary()`).[^1]
- `sys._is_immortal()` (new, CPython detail) reports immortal objects; on the free-threaded build, all interned strings are immortal.
- On the free-threaded build, memory is freed later (QSBR, biased refcounting, mimalloc). `gc.collect()` forces QSBR-deferred frees; RSS may still not shrink immediately.[^23]

***

#### Performance

```python
import cProfile
import io
import pstats
import timeit

def work() -> int:
    return sum(i * i for i in range(10_000))

with cProfile.Profile() as profiler:
    work()
stream = io.StringIO()
pstats.Stats(profiler, stream=stream).sort_stats('cumulative').print_stats(3)
print('function calls' in stream.getvalue())                 # True

print(timeit.timeit('sum(range(100))', number=1000) > 0)     # True
```

What 3.14 changes for performance (measure before relying on any of it):[^1]

- **Tail-calling interpreter** (opt-in build, Clang 19+ on x86-64/AArch64): ~3–5% faster on pyperformance; invisible to Python code.
- **Experimental JIT** is included in the official Windows and macOS binaries, **off by default** (`PYTHON_JIT=1`); effect ranges from 10% slower to 20% faster, and native profilers cannot unwind JIT frames — **not recommended for production**. `sys._jit.is_available()` / `is_enabled()` report status.
- **Free-threaded build**: ~5–10% single-thread penalty, real multi-core scaling for threaded CPU-bound code (if all extensions support it).
- **asyncio** 10–20% faster; `uuid4()` ~30% faster; `Path.read_bytes()` faster; `textwrap.dedent()` ~2.4x faster; faster import of many stdlib modules.
- `zlib` on Windows uses **zlib-ng**: `Z_BEST_SPEED` (level 1) may compress noticeably worse than before.
- The `profiling` package and the Tachyon sampling profiler are **3.15** — in 3.14 use `cProfile`, `tracemalloc`, `python -X importtime`, or external samplers such as py-spy.

Classic rules still dominate: O(1) membership with `set`/`frozenset`, `str.join()` instead of `+=` in loops, generators for streaming, `functools.cache` for pure functions with bounded inputs, and avoiding work in hot loops.

***

#### Testing

```python
import unittest

class TestNew314Asserts(unittest.TestCase):
    def test_strings(self) -> None:
        self.assertStartsWith('python3.14', 'python')       # new in 3.14
        self.assertEndsWith('report.zst', '.zst')           # new in 3.14
        self.assertNotStartsWith('main', 'test_')           # new in 3.14

    def test_attrs_and_types(self) -> None:
        self.assertHasAttr(unittest, 'TestCase')            # new in 3.14
        self.assertIsSubclass(bool, int)                    # new in 3.14

    def test_annotations_lazy(self) -> None:
        def f(x: NotYetDefined) -> None: ...                # defining is fine in 3.14
        with self.assertRaises(NameError):
            f.__annotations__

if __name__ == '__main__':
    unittest.main(argv=['x'], exit=False, verbosity=0)
```

Test-suite guidance for 3.14:

- Run CI with **`-W error`** (or pytest `filterwarnings = ["error"]`) to surface the 3.14 deprecations (asyncio policies, `codecs.open()`, `asyncio.iscoroutinefunction()`, ...) and PEP 765 `SyntaxWarning`s *before* they become removals. Add `-X warn_default_encoding` to catch `open()` without `encoding=`.
- `unittest` output is colored by default; set `NO_COLOR=1` or `PYTHON_COLORS=0` if CI logs show escape codes.
- `unittest` discovery accepts a namespace package as start directory again.
- Don't assert on exact error-message texts that 3.14 improved (unhashable types, keyword typos) or on `sys.getrefcount()` values.
- For multiprocessing tests on Linux, remember the `forkserver` default: test helpers must be importable, top-level functions.
- `unittest.IsolatedAsyncioTestCase` and pytest-asyncio work as before; never create loops via `asyncio.get_event_loop()` in fixtures — it raises `RuntimeError` in 3.14.

```python
import asyncio
import unittest
from unittest.mock import AsyncMock

class TestService(unittest.IsolatedAsyncioTestCase):
    async def test_fetch(self) -> None:
        fetch = AsyncMock(return_value={'id': 1})
        result = await fetch(1)
        fetch.assert_awaited_once_with(1)
        self.assertEqual(result['id'], 1)

if __name__ == '__main__':
    unittest.main(argv=['x'], exit=False, verbosity=0)
```

***

#### Virtual Environments and Packaging

```bash
python3.14 -m venv .venv
source .venv/bin/activate

# uv: fast installer that can also provision interpreters, incl. free-threaded ones
uv venv --python 3.14
uv python install 3.14t        # free-threaded build (python3.14t)
uv pip install -r requirements.txt
```

##### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=77"]
build-backend = "setuptools.build_meta"

[project]
name = "myapp"
version = "1.0.0"
requires-python = ">=3.14"
dependencies = []

[tool.ruff]
target-version = "py314"

[tool.mypy]
python_version = "3.14"
strict = true

[tool.pytest.ini_options]
testpaths = ["tests"]
filterwarnings = ["error"]
```

Packaging notes for 3.14:

- **`requires-python = ">=3.14"` is mandatory** if the code uses t-strings, `except A, B:`, unquoted forward references evaluated at runtime, `annotationlib`, `compression.zstd`, `concurrent.interpreters` or any other 3.14 API — older interpreters fail at import or compile time.
- Free-threaded wheels use the **`cp314t`** ABI tag and are separate from `cp314` wheels; the Limited API/stable ABI (`abi3`) is **not** available for free-threaded builds in 3.14 (a free-threaded stable ABI arrives in 3.15). Extension authors must build `cp314t` wheels explicitly (opting out of `py_limited_api` for that build) and declare `Py_mod_gil` support.[^5][^37]
- Installations now include **`build-details.json`** (PEP 739) in the stdlib directory — a static description of the build for launchers and cross-compilation tools.[^1]
- Verify CPython downloads with **Sigstore**, not PGP (PEP 761).[^34]
- When running under a virtual environment, `sys.prefix`/`sys.exec_prefix` are now set during path initialization rather than by `site` — tools that compute paths before `site` runs see the venv prefix.[^31]

##### Project Layout

```
project/
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
ruff check .            # lint (target-version = "py314")
ruff format .           # format
mypy --strict src/      # type check (python_version = "3.14")
python -W error -X dev -m pytest
python -m compileall -q src   # surfaces PEP 765 SyntaxWarnings at compile time
```

Tool support (verified with ruff 0.17.1 and mypy 2.4.0, October 2026): both parse t-strings and unparenthesized `except` clauses, and report them as errors when the configured target is older than 3.14 ("Cannot use t-strings on Python 3.13"). Ruff's `B012` flags `return`/`break`/`continue` in `finally` independently of the interpreter warning. Set the target version to **3.14** in every tool — a lower target hides 3.14-only syntax only until the code is run on an older interpreter, and a higher target (3.15) lets 3.15-only syntax slip through.

***

#### Removed and Changed in 3.14 — Migration Notes

##### Removed in 3.14

Code using any of these fails on 3.14 with `AttributeError`, `ImportError` or `TypeError`:[^1]

| Removed in 3.14 | Replacement |
|---|---|
| `ast.Num`, `ast.Str`, `ast.Bytes`, `ast.NameConstant`, `ast.Ellipsis`; `Constant.n` / `Constant.s` | `ast.Constant` and `.value`; define `visit_Constant` (old `visit_Num`/`visit_Str` methods are silently **never called**) |
| asyncio child watchers (`AbstractChildWatcher`, `ThreadedChildWatcher`, ..., `get_child_watcher()`, `set_child_watcher()`) | nothing needed — the event loop manages child processes |
| `asyncio.get_event_loop()` creating a loop when none is set | `asyncio.run()`, `asyncio.Runner`, `asyncio.get_running_loop()` |
| `pkgutil.find_loader()`, `pkgutil.get_loader()` | `importlib.util.find_spec()` |
| `urllib.request.URLopener`, `FancyURLopener` | `urllib.request.urlopen()`, `build_opener()` |
| `urllib.parse.Quoter` | `urllib.parse.quote()` |
| `sqlite3.version`, `sqlite3.version_info` | `sqlite3.sqlite_version`, `sqlite_version_info` |
| sqlite3: sequence of params with *named* placeholders | pass a `dict` (now `ProgrammingError`) |
| `pty.master_open()`, `pty.slave_open()` | `pty.openpty()` |
| `importlib.abc.ResourceReader`, `Traversable`, `TraversableResources` | `importlib.resources.abc` equivalents |
| `email.utils.localtime(isdst=...)` | drop the argument |
| copy/deepcopy/pickle of `itertools` iterators | materialize the data, or re-create the iterator |
| `argparse.BooleanOptionalAction(type=, choices=, metavar=)`; nested argument groups | drop the parameters; flatten groups (`ValueError` now) |
| extra `Path(..., **kwargs)`; extra positional args to `relative_to()`/`is_relative_to()` | remove them |
| `int(x)` via `__trunc__` | implement `__int__` or `__index__` |
| `NotImplemented` in boolean context | `TypeError` now — return/compare it explicitly |

```python
import ast

class Visitor(ast.NodeVisitor):
    def visit_Num(self, node):                 # ❌ never called in 3.14
        print('Num', node.n)
    def visit_Constant(self, node: ast.Constant) -> None:   # ✅
        print('Constant', node.value)

Visitor().visit(ast.parse('x = 42'))           # Constant 42
print(hasattr(ast, 'Num'))                     # False
```

##### Behavior Changes That Break Code Silently

| Change | What to look for in review |
|---|---|
| Deferred annotations | metaclasses/decorators reading `ns['__annotations__']`; `obj.__annotations__` read via instances; import-time side effects in annotations; `TYPE_CHECKING`-only names in runtime-evaluated hints |
| `forkserver` default (Linux) | globals mutated after import and read in workers; lambdas/closures sent to pools; missing `__main__` guard |
| `tarfile` `'data'` filter default | archives that legitimately contain absolute or escaping symlinks or device files now raise `FilterError`; setuid bits and ownership are silently dropped |
| `functools.partial` is a method descriptor | `partial` objects stored as class attributes now receive `self` |
| `Path.exists()`/`is_*()` no longer raise | permission problems become `False` silently |
| `typing.Union` is `types.UnionType` | `is` comparisons of unions, `repr()` string matching, `typing._UnionGenericAlias` checks |
| `pickle` default protocol 5; more `PicklingError`s | consumers on Python ≤ 3.7; `except AttributeError` around `pickle.dumps()` |
| `re` `\B` matches the empty string | patterns that are only `\B` |
| `gc` incremental (3.14.0–3.14.4) → generational (3.14.5+) | benchmarks or `gc.set_threshold()` tuning done on early 3.14.x |
| `locale.nl_langinfo()` temporarily sets `LC_CTYPE` | other threads observing locale changes |
| io: non-blocking text reads | `read()` on a non-blocking text stream may raise `BlockingIOError` |
| `mimetypes` CLI exit codes; `sys.platform` on FreeBSD is always `'freebsd'` | scripts parsing those |

##### Deprecated in 3.14 — Fix Before 3.15/3.16

**Removed in 3.15** (deprecated in 3.14 or earlier, most with a runtime `DeprecationWarning` — the project's next upgrade will break):[^5]

- `typing.NamedTuple('P', x=int)` keyword syntax; `TypedDict('TD')` / `TypedDict('TD', None)` with no fields; `@typing.no_type_check_decorator`
- arguments to `threading.RLock()`
- `codeobject.co_lnotab` (use `co_lines()`)
- `pathlib.PurePath.is_reserved()` (use `os.path.isreserved()`), `platform.java_ver()`, `http.server.CGIHTTPRequestHandler` and `python -m http.server --cgi`
- `importlib` `load_module()` (use `exec_module()`), `ctypes.SetPointerType()`, `wave` `getmark()`/`setmark()`/`getmarkers()`, `sre_compile`/`sre_parse`/`sre_constants`
- `datetime.strptime()` with a day of month but **no year** in the format (`'%m-%d'`) — `DeprecationWarning` since 3.13, `ValueError` in 3.15; add a year (or parse with a fixed leap year yourself)
- `module.__cached__` is no longer set or consulted by the import system — read `__spec__.cached` (`__package__` and `__loader__` are deprecated too; use `__spec__.parent`/`__spec__.loader`)

**Scheduled for 3.16:** the asyncio policy system and `asyncio.iscoroutinefunction()`; `~bool`; `logging` handlers' `strm` argument; undotted `mimetypes.add_type()` extensions; Python implementation of `functools.reduce()` with keyword `function`/`sequence`; `shutil.ExecError`; `symtable.Class.get_methods()`; `sysconfig.expand_makefile_vars()`; `array` `'u'` format code (use `'w'`).[^1]

**Scheduled for 3.17:** `collections.abc.ByteString` and `typing.ByteString`; `typing._UnionGenericAlias` shim. **3.19:** `PurePath.as_uri()`; implicit MSVC layout via `ctypes` `_pack_`.

**Deprecated in 3.14 without a removal date:** `codecs.open()` (`DeprecationWarning`; use `open()`), `argparse.FileType` (`PendingDeprecationWarning`), `complex()` with a complex `real`/`imag`, `os.popen()`/`os.spawn*()` (soft — documentation only, no warning). Older still-pending ones: three-argument `throw()`/`athrow()`, numeric literals immediately followed by keywords (`0in x`, `SyntaxWarning`), `datetime.utcnow()`/`utcfromtimestamp()`.[^1]

**Point-release reversal:** `locale.getdefaultlocale()` was deprecated (slated for 3.15 removal) and emitted `DeprecationWarning` on 3.14.0–3.14.6; **3.14.7 un-deprecated it** (gh-130796) and it remains in 3.15. Don't "fix" working calls just to silence a warning seen on an early 3.14.x.[^12]

```python
import warnings, datetime
with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter('always')
    datetime.datetime.strptime('03-15', '%m-%d')        # ❌ ambiguous: no year
print(caught[0].category.__name__)                      # DeprecationWarning
print(datetime.datetime.strptime('2024-03-15', '%Y-%m-%d').date())   # 2024-03-15 ✅
```

***

#### Production Checklist

##### Python 3.14-Specific Items

- [ ] Running the latest 3.14.x (3.14.9 as of October 21, 2026) — not 3.14.0–3.14.4 (incremental GC memory pressure, tarfile filter bypasses) and never 3.14.1 (dataclass regressions)
- [ ] `requires-python = ">=3.14"` declared if any 3.14 syntax/API is used; tools target `py314`
- [ ] No 3.15+ features: no `lazy import`, `frozendict`, `sentinel`, comprehension unpacking, `TypedDict(closed=...)`, `TypeForm`, `re.prefixmatch()`, `TaskGroup.cancel()`, `profiling`, `math.integer`
- [ ] Forward references written without quotes in 3.14-only code; `from __future__ import annotations` kept only where older Pythons must be supported
- [ ] Annotation readers use `annotationlib.get_annotations(obj, format=Format.FORWARDREF)` / `get_type_hints(format=...)`, not `obj.__annotations__` or class-namespace lookups
- [ ] No runtime-evaluated annotations (FastAPI, pydantic, `inspect.signature()`) depend on `TYPE_CHECKING`-only imports
- [ ] t-strings consumed by escaping/parameterizing processors; APIs that must not receive pre-formatted text accept `Template`, not `str`
- [ ] No `except X, name:` that was meant as Python 2 `except X as name`; `as` forms parenthesized
- [ ] No `return`/`break`/`continue` inside `finally` (PEP 765 `SyntaxWarning`); `python -W error -m compileall` clean
- [ ] `ProcessPoolExecutor`/`multiprocessing` code works under `forkserver`: top-level picklable callables, no runtime-mutated globals, `__main__` guard; `fork` requested explicitly only with justification
- [ ] `Executor.map()` over large/infinite iterables uses `buffersize=`
- [ ] `asyncio.get_event_loop()` outside a running loop replaced with `asyncio.run()`/`Runner`; no asyncio policy APIs; `inspect.iscoroutinefunction()` used
- [ ] `tarfile` extraction passes `filter='data'` explicitly; `FilterError` handled
- [ ] Untrusted compressed input decompressed incrementally with `max_length` limits (zstd, gzip, zlib, bz2, lzma)
- [ ] `ZIP_ZSTANDARD` used only for consumers on Python 3.14+ / zstd-aware tools; `compression.zstd` import guarded where it may be missing
- [ ] `functools.partial` class attributes wrapped in `staticmethod()` where the old behavior was intended
- [ ] `int()` conversions don't rely on `__trunc__`; no `NotImplemented` in boolean context
- [ ] Permission checks use `Path.stat()` (not `exists()` expecting `PermissionError`)
- [ ] Free-threaded deployments verify `sys._is_gil_enabled()` at startup, ship `cp314t` wheels for all extensions, and protect shared state with locks
- [ ] Subinterpreter/`InterpreterPoolExecutor` use limited to subinterpreter-compatible extensions; not used as a security boundary
- [ ] Remote debugging disabled (`PYTHON_DISABLE_REMOTE_DEBUG=1`) on hosts with untrusted local users; ptrace scope reviewed
- [ ] `uuid7()` for time-ordered IDs where creation time may be public; `secrets` for tokens
- [ ] Pickles for consumers on Python ≤ 3.7 written with `protocol=4`
- [ ] CI runs `-W error` (3.14 deprecations scheduled for 3.15/3.16 removal) and `-X warn_default_encoding`
- [ ] CPython artifacts verified with Sigstore (no PGP signatures for 3.14)

##### Code Correctness

- [ ] `bytes` and `str` never mixed without explicit encode/decode; `encoding=` passed to every `open()`
- [ ] No mutable default arguments; dataclass collections use `field(default_factory=...)`
- [ ] `__hash__` only on immutable classes; value objects are `@dataclass(frozen=True)`
- [ ] No dict/list mutation during iteration
- [ ] `match` value patterns use dotted names or literals, not bare capture names
- [ ] Exact refcounts, error-message texts and union `repr()`s not asserted in tests

##### Safety and Security

- [ ] SQL parameterized (or built from t-strings by a parameterizing processor) — never f-strings
- [ ] `pickle.load()` never on untrusted data; `eval`/`exec` never on untrusted input — including annotation strings and `ForwardRef`s
- [ ] `subprocess` with argument lists, no `shell=True` on user input, with timeouts
- [ ] `secrets.compare_digest()` for token comparison; slow KDF for passwords
- [ ] `defusedxml` for untrusted XML; latest 3.14.x for libexpat and parser DoS fixes
- [ ] Path traversal checked with `resolve()` + `is_relative_to()`

##### Performance

- [ ] Profiled before optimizing (`cProfile`, `tracemalloc`, `-X importtime=2`)
- [ ] JIT (`PYTHON_JIT=1`) not enabled in production
- [ ] Free-threaded build adopted only after measuring single-thread overhead and memory
- [ ] Generators for streaming; sets for membership; `str.join()` for building strings
- [ ] Blocking calls in async code moved to `asyncio.to_thread()` or executors

##### Observability

- [ ] `logging.getLogger(__name__)` per module; `%`-style arguments, no f-/t-strings in log calls
- [ ] Tasks and threads named (`name=`) — visible in `python -m asyncio ps/pstree` and OS thread names
- [ ] `logger.exception()` inside `except` blocks; bare `raise` to re-raise
- [ ] `faulthandler` enabled in services (`-X faulthandler`; C stacks with `c_stack=True`)
- [ ] `NO_COLOR=1` / `PYTHON_COLORS=0` in CI if colored output breaks log parsing

***

[^1]: What's New in Python 3.14. Python Software Foundation. https://docs.python.org/3.14/whatsnew/3.14.html
[^2]: PEP 745 – Python 3.14 Release Schedule. https://peps.python.org/pep-0745/
[^3]: Python downloads — active releases and release dates (3.14.9 and 3.15.0/3.15.1). https://www.python.org/downloads/
[^4]: What's New in Python 3.13 (and the linked 3.10–3.12 What's New documents). https://docs.python.org/3.14/whatsnew/3.13.html
[^5]: What's New in Python 3.15 (lazy imports PEP 810, frozendict PEP 814, sentinel PEP 661, PEP 798, PEP 686 UTF-8 mode, removals). https://docs.python.org/3.15/whatsnew/3.15.html
[^6]: PEP 649 – Deferred Evaluation Of Annotations Using Descriptors. https://peps.python.org/pep-0649/
[^7]: PEP 779 – Criteria for supported status for free-threaded Python. https://peps.python.org/pep-0779/
[^8]: dataclasses — Data Classes (field doc=, make_dataclass decorator=). https://docs.python.org/3.14/library/dataclasses.html
[^9]: annotationlib — Functionality for introspecting annotations. https://docs.python.org/3.14/library/annotationlib.html
[^10]: typing — Support for type hints (get_type_hints format=, evaluate_forward_ref, Union changes, NamedTuple super()). https://docs.python.org/3.14/library/typing.html
[^11]: PEP 749 – Implementing PEP 649 (future-import deprecation plan). https://peps.python.org/pep-0749/
[^12]: Python 3.14 changelog (3.14.1–3.14.9 entries, including Security sections). https://docs.python.org/3.14/whatsnew/changelog.html
[^13]: PEP 750 – Template Strings. https://peps.python.org/pep-0750/
[^14]: string.templatelib — Support for template string literals. https://docs.python.org/3.14/library/string.templatelib.html
[^15]: PEP 758 – Allow except and except* expressions without brackets. https://peps.python.org/pep-0758/
[^16]: PEP 765 – Disallow return/break/continue that exit a finally block. https://peps.python.org/pep-0765/
[^17]: heapq — Heap queue algorithm (max-heap functions). https://docs.python.org/3.14/library/heapq.html
[^18]: functools — Placeholder, partial. https://docs.python.org/3.14/library/functools.html
[^19]: pathlib — Object-oriented filesystem paths (copy/move, info, query methods). https://docs.python.org/3.14/library/pathlib.html
[^20]: compression.zstd — Compression compatible with the Zstandard format; PEP 784. https://docs.python.org/3.14/library/compression.zstd.html , https://peps.python.org/pep-0784/
[^21]: asyncio Event Loop (get_event_loop RuntimeError; 3.14.8 server_hostname check). https://docs.python.org/3.14/library/asyncio-eventloop.html
[^22]: asyncio call graph introspection. https://docs.python.org/3.14/library/asyncio-graph.html
[^23]: Python support for free threading (HOWTO). https://docs.python.org/3.14/howto/free-threading-python.html
[^24]: PEP 734 – Multiple Interpreters in the Stdlib. https://peps.python.org/pep-0734/
[^25]: concurrent.interpreters — Multiple interpreters in the same process. https://docs.python.org/3.14/library/concurrent.interpreters.html
[^26]: concurrent.futures — InterpreterPoolExecutor, Executor.map buffersize, terminate_workers/kill_workers. https://docs.python.org/3.14/library/concurrent.futures.html
[^27]: multiprocessing — Contexts and start methods (forkserver default; 3.14.9 spawn fallback). https://docs.python.org/3.14/library/multiprocessing.html#contexts-and-start-methods
[^28]: gh-155717 / gh-143706 / gh-146004 — multiprocessing start-method fixes in 3.14.x (see changelog). https://docs.python.org/3.14/whatsnew/changelog.html
[^29]: warnings — Concurrent safety of context managers (context_aware_warnings). https://docs.python.org/3.14/library/warnings.html
[^30]: PEP 768 – Safe external debugger interface for CPython. https://peps.python.org/pep-0768/
[^31]: sys — sys.remote_exec(), sys.flags, sys.prefix under venvs. https://docs.python.org/3.14/library/sys.html
[^32]: tarfile — Extraction filters (default 'data' in 3.14). https://docs.python.org/3.14/library/tarfile.html#tarfile-extraction-filter
[^33]: PEP 706 – Filter for tarfile.extractall. https://peps.python.org/pep-0706/
[^34]: PEP 761 – Deprecating PGP signatures for CPython artifacts. https://peps.python.org/pep-0761/
[^35]: uuid — UUID objects according to RFC 9562 (uuid6/7/8, NIL, MAX). https://docs.python.org/3.14/library/uuid.html
[^36]: pickle — Data stream format (protocol 5 default in 3.14). https://docs.python.org/3.14/library/pickle.html#data-stream-format
[^37]: C API Extension Support for Free Threading (limited API not supported in free-threaded 3.14). https://docs.python.org/3.14/howto/free-threading-extensions.html
