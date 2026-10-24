### Python 3.15: The Complete Production Developer Guide

#### Overview

Python 3.15 was released on October 9, 2026[^3] — one week later than planned, because a surprise third release candidate (3.15.0rc3, October 2, 2026) was cut to fix last-minute lazy-import release blockers.[^5][^2] The first maintenance release, **3.15.1**, followed on October 21, 2026 as an *expedited security release*: it bundles pip 26.2.2 (with certifi 2026.10.19, removing the distrusted emSign CA) and libexpat 2.9.0, plus about 330 bug fixes — several of them for lazy imports.[^4][^23] As of October 2026, 3.15 is the **latest stable release** and is in full bugfix support: bugfix releases roughly every two months until about October 2028 (3.15.12, planned for 2028-10-03, is the last regular bugfix release with binary installers), then source-only security releases until **October 2031**.[^2][^6] Python 3.16 is only a pre-release (final planned for October 2027).[^29]

The headline changes are **explicit lazy imports** (the `lazy` soft keyword, PEP 810), two new builtins — **`frozendict`** (PEP 814) and **`sentinel`** (PEP 661) — **UTF-8 as the default text encoding everywhere** (PEP 686), **`*`/`**` unpacking inside comprehensions** (PEP 798), the new **`math.integer`** module (PEP 791), a dedicated **`profiling`** package with the **Tachyon** sampling profiler (PEP 799), typed extra items for `TypedDict` (PEP 728), `TypeForm` (PEP 747), a substantially upgraded (still experimental, still off by default) JIT, frame pointers enabled by default (PEP 831), and a Stable ABI for free-threaded builds (`abi3t`, PEP 803). It also completes a large batch of removals scheduled years ago (see the migration notes near the end).[^1]

This guide targets Python 3.15 specifically. Features introduced in earlier versions are fully available and are used freely here: 3.10 `match`/`case` and `X | Y` unions; 3.11 `ExceptionGroup`/`except*`, `asyncio.TaskGroup`, `tomllib`, `typing.Self`; 3.12 type-parameter syntax (`def f[T]`, `type Alias = ...`, PEP 695), the formalized f-string grammar (PEP 701), `@typing.override`, `itertools.batched`; 3.13 `copy.replace`, `@warnings.deprecated`, `TypeIs`/`ReadOnly`/TypeVar defaults; 3.14 deferred evaluation of annotations (PEP 649/749, `annotationlib`), template strings (`t"..."`, PEP 750), `except A, B:` without parentheses (PEP 758), `compression.zstd`, `concurrent.interpreters`, `uuid.uuid7()`, `Path.copy()`/`Path.move()`. Every section that covers a 3.15-specific change is marked **New in 3.15**.

**Do not recommend anything newer than 3.15.** 3.15 is the newest release, so the boundary is: nothing from 3.16 (alpha/pre-release, PEP 826) or from `docs.python.org/3.16`/`dev` docs, and nothing that was *proposed* for 3.15 but did not ship in that form. Commonly repeated but wrong claims about 3.15:

- There is **no `none` lazy-imports mode**. The PEP 810 text still describes three global modes (`normal`/`all`/`none`), but 3.15 ships only `normal` (default) and `all`. `-X lazy_imports=none` is a fatal startup error and `sys.set_lazy_imports("none")` raises `ValueError`.
- The integer-math module is **`math.integer`**, not `imath`/`intmath` (naming-poll candidates; `intmath` is only a third-party backport package on PyPI).[^13]
- The sentinel factory is the **builtin `sentinel(...)`**, not `typing.Sentinel` / `enum.Sentinel` (`typing_extensions.Sentinel` is a separate backport).
- `frozendict` is **not** a `dict` subclass — `isinstance(fd, dict)` is `False`.
- `locale.getdefaultlocale()` was **not removed** in 3.15 — it was *un-deprecated*.[^1]
- The free-threaded build is still **optional** (PEP 779 "phase II": supported, not default), and the JIT is still **experimental and disabled by default**.
- The incremental garbage collector from 3.14.0 is **gone**: it was reverted in 3.14.5, and 3.15 uses the generational GC from 3.13.[^1]

APIs scheduled for removal in **3.16** still exist in 3.15 but emit `DeprecationWarning` — flag any new use of them (see "Pending Removal in 3.16 and 3.17").

Global truths about Python 3.15:

- **Everything is an object.** Integers, functions, classes, modules — all are objects.
- **Indentation is syntax.** Python uses indentation levels to delimit blocks, not braces.
- **Dynamic typing; annotations are lazy.** Since 3.14, annotations are evaluated on demand (PEP 649), so forward references no longer need quotes or `from __future__ import annotations`.
- **Reference counting + cyclic GC.** The default build uses reference counts plus the *generational* cyclic collector (3.14.0–3.14.4's incremental collector was reverted).
- **UTF-8 is the default encoding.** `open()` without `encoding=` now uses UTF-8 on every platform (PEP 686) — but still pass `encoding=` explicitly in code that must also run on ≤3.14.
- **Dicts are ordered by insertion** (language guarantee since 3.7), and there is now an immutable, hashable `frozendict` to go with them.
- **Imports can be lazy.** A `lazy import` binds a proxy; the module loads on first use. Import-time side effects and import errors move to that first use.

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

# Sentinels (new builtin in 3.15): the variable name matches the sentinel name
MISSING = sentinel('MISSING')

# Classes: PascalCase
class UserAccount:
    pass

# Type parameters (3.12+ syntax): short PascalCase names
def first[T](items: list[T]) -> T:
    return items[0]

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

In Python 3, all classes implicitly inherit from `object`. `class Foo:` and `class Foo(object):` are identical. Never write the latter.

`lazy` is a **soft keyword**: it only has meaning directly before `import`/`from`. `lazy = 3` is still a legal variable name, but don't use it — it confuses readers of 3.15 code.

##### Import Ordering

Three blocks separated by blank lines: standard library, third-party, local. Alphabetize within each group. Keep **all imports at the top of the module** — in 3.15 that is now compatible with fast startup, because expensive imports can be marked `lazy` instead of being hidden inside functions.

```python
# Standard library — cheap, always-needed modules stay eager
import logging
import os
import sys
from pathlib import Path

# Standard library / third-party — heavy and only needed on some code paths: lazy (3.15+)
lazy import json
lazy from decimal import Decimal

# Local application
from myapp.models import User      # eager: needed on every run
lazy import myapp.reports          # lazy: only the 'report' subcommand uses it

# Never wildcard imports — and `lazy from x import *` is a SyntaxError anyway
```

Rules for lazy imports in import blocks (details in the PEP 810 section below):

- Mark an import `lazy` only if the module has **no import-time side effects you rely on** (registering plugins, codecs, signal handlers, monkey-patching, configuring logging).
- `lazy` is a SyntaxError inside functions, class bodies and `try`/`except`/`finally` — **optional-dependency imports (`try: import x except ImportError:`) must stay eager**.
- Group lazy imports visibly (e.g. after the eager ones within each block) so reviewers can see what is deferred. Ruff/isort ordering support for `lazy` depends on the tool version — check before enforcing.

##### Docstrings

Write Google-style docstrings for every public module, class, and function. First line: a one-sentence imperative summary.

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

    # No quotes needed for forward references: annotations are lazy since 3.14
    def __init__(self, db: DatabaseConnection, cache: Cache | None = None) -> None:
        self.db = db
        self.cache = cache


class DatabaseConnection: ...
class Cache: ...
```

***

#### New in 3.15: Explicit Lazy Imports (PEP 810)

The single most consequential change in 3.15 for application code. Prefixing an import with the soft keyword `lazy` binds a lightweight proxy instead of loading the module; the module is loaded ("reified") the first time the name is used.[^7][^8]

```python
import sys

lazy import json
lazy from collections import OrderedDict, Counter

print('json' in sys.modules)               # False — not loaded yet
print(type(globals()['json']).__name__)    # lazy_import — a proxy

data = json.dumps({'key': 'value'})        # first use: json is imported HERE
print('json' in sys.modules)               # True
print(type(globals()['json']).__name__)    # module — the binding was replaced

c = Counter('aab')                         # reifies only 'Counter'
print(type(globals()['Counter']).__name__)      # type
print(type(globals()['OrderedDict']).__name__)  # lazy_import — still a proxy
```

The proxy type is exposed as `types.LazyImportType` (its `__name__` is `lazy_import`) for code that must detect it.

##### Where `lazy` Is Allowed

`lazy` is only permitted at **module scope**. Each of the following is a `SyntaxError` with a specific message:

```python
# Each snippet is compiled separately to show the exact error
bad = {
    'def f():\n    lazy import json':                  'inside functions',
    'class C:\n    lazy import json':                  'inside classes',
    'try:\n    lazy import json\nexcept ImportError:\n    pass': 'inside try/except blocks',
    'lazy from os import *':                           'star import',
    'lazy from __future__ import annotations':         '__future__ import',
}
for src, why in bad.items():
    try:
        compile(src, '<demo>', 'exec')
    except SyntaxError as e:
        print(f'{why}: {e.msg}')
# inside functions: lazy import not allowed inside functions
# inside classes: lazy import not allowed inside classes
# inside try/except blocks: lazy import not allowed inside try/except blocks
# star import: lazy from ... import * is not allowed
# __future__ import: lazy from __future__ import is not allowed

# Allowed at module level, including inside `if` and `with` blocks:
compile('if True:\n    lazy import json', '<demo>', 'exec')
compile('with open("/dev/null") as f:\n    lazy import json', '<demo>', 'exec')
```

On 3.14 and earlier, `lazy import x` is a plain `SyntaxError: invalid syntax` — the whole module fails to compile. Code that must also run on older versions uses `__lazy_modules__` instead (below).

##### Errors Move to First Use

If the module cannot be loaded, nothing happens at the `lazy import` line; the exception is raised where the name is first used, chained to an `ImportError` that points back at the import statement. A failed reification is **retried on every later use**:

```python
lazy import does_not_exist_xyz       # no error here

print('module body continues')       # module body continues

def use():
    return does_not_exist_xyz.thing

for attempt in range(2):
    try:
        use()
    except ImportError as e:         # ModuleNotFoundError is an ImportError subclass
        print(type(e).__name__, e)
# ModuleNotFoundError No module named 'does_not_exist_xyz'   (both attempts)
```

**Pitfall: you cannot guard a lazy import with `try`/`except ImportError`.** The `try` form is a SyntaxError, and wrapping only the *use* means a missing dependency surfaces deep inside unrelated code at runtime. Keep optional dependencies eager:

```python
# ✅ Optional dependency: must stay an eager import
try:
    import yaml_does_not_exist as yaml
except ImportError:
    yaml = None

# ❌ Not possible in 3.15:
# try:
#     lazy import yaml          # SyntaxError: lazy import not allowed inside try/except blocks
# except ImportError: ...
print(yaml)   # None
```

##### Reification Semantics Reviewers Must Know

- **Accessing the name reifies it** — any load of the global (or `module.attr` from another module) imports the module and replaces the binding.
- **`globals()`, `vars(module)` and `module.__dict__` do NOT reify.** Code that iterates a module namespace (plugin discovery, registries built from `vars()`, serializers, debuggers) sees `lazy_import` proxies. Use `getattr(module, name)` — which reifies — or call `proxy.resolve()` explicitly.
- `proxy.resolve()` returns the real object but does **not** rebind the global; the next normal access does that.
- A lazy `from m import a, b` loads the whole module on first use of *either* name, but only rebinds the name that was used.
- Reification goes through the normal import machinery **at the time of first use** — `sys.path`, `sys.meta_path` and import hooks in effect *then* apply, not those at the import statement.
- `sys.lazy_modules` (a `set`) lists lazy imports that have not been reified yet — module names, plus `module.attr` entries for lazy `from` imports; an entry disappears when the module is loaded. The 3.15 standard library itself uses lazy imports internally, so this set is non-empty even in a plain script.

```python
import types

lazy import json

print(isinstance(globals()['json'], types.LazyImportType))  # True — dict access: no reification
module = globals()['json'].resolve()            # explicit reification
print(type(module).__name__)                    # module
print(type(globals()['json']).__name__)         # lazy_import — resolve() doesn't rebind

# Careful: binding the proxy to another GLOBAL name and reading that name reifies it
alias = globals()['json']
print(type(alias).__name__)                     # module — loading a global reifies

def inspect_proxy() -> str:
    proxy = globals()['json']                   # a LOCAL holds the proxy as-is
    return type(proxy).__name__

print(inspect_proxy())                          # lazy_import

# Namespace scanning must not assume real objects:
real = {name: value for name, value in globals().items()
        if not isinstance(value, types.LazyImportType)}
print('json' in real)                           # False
```

**Pitfall (fixed in 3.15.1): module-level `__getattr__` shadowed lazy imports.** On 3.15.0, if a module defines `__getattr__` and also has `lazy import json`, then `othermodule.json` from *outside* calls `__getattr__('json')` instead of reifying the lazy import (gh-158824).[^23] Other 3.15.1 lazy-import fixes: `unittest.mock.patch(..., autospec=True)` on lazily imported objects (gh-153888), `@dataclass` with a *string* annotation naming an unresolvable lazy import (gh-158936), and a crash (now `RecursionError`) on long chains like `lazy import a.b.c as d` (gh-159288). **Require ≥ 3.15.1 for code that uses lazy imports.**

##### Lazy Imports and Annotations

Because annotations are evaluated lazily (3.14+), a `lazy` import used only in annotations is never loaded unless something introspects the annotations. This is the 3.15 replacement for many `if TYPE_CHECKING:` blocks — but unlike `TYPE_CHECKING`, the name *does* resolve at runtime when needed (dataclasses, pydantic, `typing.get_type_hints()`):

```python
import sys
lazy from decimal import Decimal

def price(amount: Decimal) -> Decimal:
    return amount

print('decimal' in sys.modules)   # False — defining the function did not load decimal
price.__annotations__             # evaluating annotations reifies Decimal
print('decimal' in sys.modules)   # True
```

##### Global Mode and Filters

```python
import sys

print(sys.get_lazy_imports())         # normal — only `lazy`-marked imports are lazy
print(sys.get_lazy_imports_filter())  # None

try:
    sys.set_lazy_imports('none')      # the PEP text's third mode did not ship
except ValueError as e:
    print(e)                          # mode must be 'normal' or 'all'

def only_my_app(importing: str | None, imported: str, fromlist) -> bool:
    """Return True to allow the import to be lazy."""
    return imported.startswith('myapp.')

sys.set_lazy_imports_filter(only_my_app)
sys.set_lazy_imports_filter(None)     # remove the filter again
```

- `-X lazy_imports=all` / `PYTHON_LAZY_IMPORTS=all` / `sys.set_lazy_imports('all')` make every eligible module-level import lazy; `normal` is the default. Any other value is rejected (`-X lazy_imports=none` aborts at startup with *"invalid value; expected 'all' or 'normal'"*).
- The filter receives `(importing_module_name_or_None, imported_module_name, fromlist_or_None)`.
- **Never turn on `all` mode in production without testing**: every side-effect import (`import myapp.signals`, `import myapp.plugins.csv  # registers itself`) silently stops running until — or unless — the name is used. A plugin registry that relies on import side effects comes up **empty** under `-X lazy_imports=all`.

##### Version-Compatible Opt-In: `__lazy_modules__`

A module-level `__lazy_modules__` container makes *plain* `import` statements for the listed modules lazy on 3.15+, while older interpreters simply ignore the variable:

```python
__lazy_modules__ = ['json', 'decimal']   # fully qualified module names

import sys
import json                 # lazy on 3.15+, eager on 3.14 and earlier
import decimal as dec       # lazy on 3.15+
from json import dumps      # lazy: the `from` module ('json') is what is matched

print('json' in sys.modules, 'decimal' in sys.modules)   # False False on 3.15
```

Relative imports are resolved to their absolute name before matching (`from .sub.utils import f` in package `pkg` matches `'pkg.sub.utils'`). Imports inside functions, class bodies and `try` blocks stay eager regardless.[^8]

##### Review Rules for Lazy Imports

- ✅ `lazy` for heavy, side-effect-free modules used on a subset of code paths (CLI subcommands, rarely used exporters, big scientific stacks in a web worker).
- ✅ `lazy` for imports used only in annotations.
- ❌ `lazy` on modules imported *for* their side effects (registration, `codecs`/`mimetypes` setup, `gevent.monkey`, ORM model registration, `logging.config`).
- ❌ Function-local imports added "for startup time" in 3.15-only code — use `lazy` at the top instead.
- ❌ Code that iterates `globals()`/`vars(mod)` and assumes every value is a real object.
- ❌ Libraries supporting <3.15 using the `lazy` keyword — that is a SyntaxError on old interpreters; use `__lazy_modules__`.
- ⚠️ Lazy imports move `ImportError` and import-time exceptions to first use — make sure the first use is somewhere errors are handled and logged, and keep a startup smoke test that touches every lazily imported name.

***

#### New in 3.15: `frozendict` Built-in Type (PEP 814)

`frozendict` is an immutable, insertion-ordered mapping in `builtins`. It inherits directly from `object` (it is **not** a `dict` subclass), is hashable when all keys and values are hashable, and compares equal regardless of order.[^9]

```python
fd = frozendict(x=1, y=2)
print(fd)                                   # frozendict({'x': 1, 'y': 2})
print(fd == frozendict(y=2, x=1))           # True — order-insensitive equality
print(hash(fd) == hash(frozendict(y=2, x=1)))   # True
print(fd == {'x': 1, 'y': 2})               # True — compares equal to an equal dict
print(list(frozendict({'b': 1, 'a': 2})))   # ['b', 'a'] — iteration keeps insertion order

try:
    fd['z'] = 3
except TypeError as e:
    print(e)                                # 'frozendict' object does not support item assignment

print(hasattr(fd, 'update'), hasattr(fd, 'pop'))   # False False — no mutators at all
print(sorted(n for n in dir(frozendict) if not n.startswith('_')))
# ['copy', 'fromkeys', 'get', 'items', 'keys', 'values']
```

##### `isinstance` Checks Must Be Updated

```python
from collections.abc import Hashable, Mapping, MutableMapping

fd = frozendict(a=1)
print(isinstance(fd, dict))             # False — NOT a dict subclass
print(isinstance(fd, Mapping))          # True
print(isinstance(fd, MutableMapping))   # False
print(isinstance(fd, Hashable))         # True

# ❌ rejects frozendict (and MappingProxyType):
def bad_accepts(arg: object) -> bool:
    return isinstance(arg, dict)

# ✅ either list both concrete types, or accept any Mapping
def accepts(arg: object) -> bool:
    return isinstance(arg, (dict, frozendict))

def accepts_any_mapping(arg: object) -> bool:
    return isinstance(arg, Mapping)

print(bad_accepts(fd), accepts(fd), accepts_any_mapping(fd))   # False True True
```

Stdlib support: `copy`, `decimal`, `json`, `marshal`, `pickle`, `pprint`, `plistlib` (serialization only) and `xml.etree.ElementTree` accept `frozendict`; `eval()`/`exec()` accept it as globals (it must already contain `__builtins__` — a frozendict cannot have it inserted, so `eval(src, frozendict(a=1))` raises `TypeError`); `type()` and `str.maketrans()` accept it where they took a dict.[^1]

##### Shallow Immutability and Hashing

```python
fd = frozendict(name='cfg', tags=['a', 'b'])
fd['tags'].append('c')          # the frozendict is immutable, its values are not
print(fd['tags'])               # ['a', 'b', 'c']

try:
    hash(fd)                    # construction succeeded, hashing fails
except TypeError as e:
    print(e)                    # unhashable type: 'list'

# Snapshot semantics: building from a dict copies the items
src = {'a': 1}
snap = frozendict(src)
src['a'] = 2
print(snap)                     # frozendict({'a': 1})
```

##### Operators, Copies and Subclasses

```python
import copy

fd = frozendict(x=1, y=2)
print(fd | {'z': 3})                 # frozendict({'x': 1, 'y': 2, 'z': 3})
print(type({'z': 3} | fd).__name__)  # dict — the LEFT operand's type wins

alias = fd
alias |= {'w': 4}                    # rebinds `alias` to a NEW frozendict
print(alias is fd, fd)               # False frozendict({'x': 1, 'y': 2})

print(fd.copy() is fd, copy.copy(fd) is fd)   # True True — no copy needed

class Settings(frozendict):
    pass
s = Settings(a=1)
print(s, type(s | {'b': 2}).__name__)  # Settings({'a': 1}) frozendict — | drops the subclass
```

##### Practical Uses

```python
import json
from dataclasses import dataclass
from functools import cache

# 1. Immutable defaults: frozendict is hashable, so dataclasses accept it as a plain default
@dataclass(frozen=True)
class RetryPolicy:
    backoff: frozendict = frozendict(base=0.5, factor=2.0, max=30.0)

print(RetryPolicy())   # RetryPolicy(backoff=frozendict({'base': 0.5, 'factor': 2.0, 'max': 30.0}))

# 2. Cache keys: dict arguments break functools.cache, frozendict works
@cache
def render(template: str, context: frozendict) -> str:
    return template.format_map(context)

print(render('{user} logged in', frozendict(user='alice')))   # alice logged in

# 3. Deeply immutable JSON (3.15: json.loads gained array_hook)
cfg = json.loads('{"hosts": ["a", "b"], "db": {"port": 5432}}',
                 object_pairs_hook=frozendict, array_hook=tuple)
print(cfg)   # frozendict({'hosts': ('a', 'b'), 'db': frozendict({'port': 5432})})
print(hash(cfg) == hash(cfg))   # True — fully hashable
```

**Review rules:** prefer `frozendict` over `types.MappingProxyType` for constants that must be hashable or picklable (a mapping proxy is a *view* of a still-mutable dict); never call mutating methods on mapping parameters typed `Mapping`; replace `isinstance(x, dict)` gates in public APIs with `Mapping` or `(dict, frozendict)`; for code that must run on ≤3.14, `frozendict` is a `NameError` — guard or vendor a fallback.

***

#### New in 3.15: `sentinel` Built-in Type (PEP 661)

The builtin `sentinel(name, /, *, repr=None)` creates unique marker objects with a readable repr — the standard replacement for `_MISSING = object()`.[^10]

```python
import copy
import pickle

MISSING = sentinel('MISSING')
print(MISSING)                                  # MISSING
print(repr(sentinel('NOTSET', repr='<NOTSET>')))   # <NOTSET>
print(bool(MISSING))                            # True — sentinels are truthy
print(MISSING == sentinel('MISSING'))           # False — each call makes a new object
print(copy.deepcopy(MISSING) is MISSING)        # True — identity survives copies
print(pickle.loads(pickle.dumps(MISSING)) is MISSING)   # True — module-level name matches

def get_setting(key: str, default: str | MISSING = MISSING) -> str:
    """Sentinels can be used directly in type expressions with `|`."""
    if default is MISSING:                      # always compare with `is`
        raise KeyError(key)
    return default

print(get_setting('color', 'blue'))             # blue
print(MISSING.__name__, MISSING.__module__)     # MISSING __main__

try:
    class MySentinel(sentinel):
        pass
except TypeError as e:
    print(e)                                    # type 'sentinel' is not an acceptable base type
```

Rules:

- **Assign at module level (or class level) under the same name** — `MISSING = sentinel('MISSING')`; class attributes use the qualified name: `class Cls: NOTSET = sentinel('Cls.NOTSET')`. Otherwise pickling fails (`PicklingError: Can't pickle LOCAL: it's not found as __main__.LOCAL`) and the repr is misleading.
- Always test with `is`, never `==` or truthiness — **sentinels are truthy**, so `if not value:` does not detect them.
- Use a sentinel (not `None`) when `None` is a legitimate value of the parameter.
- `object()` sentinels remain fine for code that must support ≤3.14; `sentinel` is a `NameError` there.

***

#### New in 3.15: Unpacking in Comprehensions (PEP 798)

List, set and dict comprehensions and generator expressions accept `*` and `**` unpacking in the element position — a flat, readable alternative to nested comprehensions and `itertools.chain.from_iterable()`.[^11]

```python
import itertools

lists = [[1, 2], [3, 4], [5]]
print([*L for L in lists])                     # [1, 2, 3, 4, 5]
print([x for L in lists for x in L])           # same thing, the pre-3.15 spelling
print(list(itertools.chain.from_iterable(lists)))   # same again

sets = [{1, 2}, {2, 3}, {3, 4}]
print({*s for s in sets})                      # {1, 2, 3, 4}

dicts = [{'a': 1}, {'b': 2}, {'a': 3}]
print({**d for d in dicts})                    # {'a': 3, 'b': 2} — later keys win

print(list(*L for L in lists))                 # [1, 2, 3, 4, 5] — generator expression
print([*L for L in lists if len(L) > 1])       # [1, 2, 3, 4] — filters still apply per item
```

It also works in async comprehensions: `[*a async for a in agen()]`.

**Pitfalls:**

```python
lists = [[1], [2, 3]]

# A generator expression as the sole call argument is still a generator,
# so these are aggregations over the flattened stream:
print(sum(*L for L in lists))     # 6
print(max(*L for L in lists))     # 3

# Not allowed:
for src in ['[**d for d in []]',          # dict unpacking in a list comprehension
            '[*x, 1 for x in [[1]]]',      # extra elements need parentheses/brackets
            '{*k: v for k in []}']:        # starred dict key
    try:
        compile(src, '<demo>', 'eval')
    except SyntaxError as e:
        print(e.msg)
# cannot use dict unpacking in list comprehension
# did you forget parentheses around the comprehension target?
# cannot use a starred expression in a dictionary key
```

On 3.14, `[*L for L in lists]` is `SyntaxError: iterable unpacking cannot be used in comprehension` — don't use it in code that supports older versions. Readability rule: use the unpacking form for *one* level of flattening; for deeper structures a named generator function is clearer.

***

#### New in 3.15: UTF-8 Is the Default Encoding (PEP 686)

Python now runs in UTF-8 mode by default on every platform. `open()`, `Path.read_text()`, `subprocess` text mode, `sys.stdin`/`stdout` and the filesystem encoding all use UTF-8 when no encoding is given, regardless of the locale (Windows code pages like cp1252 no longer leak in).[^12][^1]

```python
import locale
import sys

print(sys.flags.utf8_mode)                    # 1 — on by default in 3.15
print(locale.getpreferredencoding(False))     # utf-8 (follows UTF-8 mode)
print(locale.getencoding())                   # the real locale encoding (e.g. UTF-8 or cp1252)

with open(__file__ if '__file__' in globals() else '/dev/null') as f:
    print(f.encoding)                         # utf-8 — no encoding= needed in 3.15-only code

# encoding='locale' (3.10+) explicitly requests the locale encoding
with open('/dev/null', encoding='locale') as f:
    print(f.encoding == locale.getencoding())   # True
```

What it means for code review:

- **Code that only runs on 3.15+** no longer has a Windows-vs-Linux encoding bug when `encoding=` is omitted. Still prefer explicit `encoding='utf-8'` — it documents intent and keeps the code correct on ≤3.14 (where the default is the locale encoding, e.g. cp1252 on many Windows machines).
- **Code that deliberately read locale-encoded files** (legacy Windows text exports, files written by other local programs) now silently decodes them as UTF-8 and may raise `UnicodeDecodeError` or produce mojibake. Use `encoding='locale'` for those.
- Opt-out for a whole process: `PYTHONUTF8=0` or `-X utf8=0`. Note that `-I`/`-E` ignore `PYTHON*` environment variables, so use `-X utf8=0` there.
- Find call sites that rely on the default: run tests with `-X warn_default_encoding` (still supported) — each omitted `encoding=` emits an `EncodingWarning`:

```bash
python3.15 -X warn_default_encoding -W error::EncodingWarning -m pytest
```

- `.pth` files: under PEP 829 they will be required to be `utf-8-sig` (locale decoding goes away in 3.20).

***

#### Primitive Types and Numerics

##### Integers

Python 3.15 has a single arbitrary-precision `int`. Arithmetic never overflows.

```python
import sys

print(sys.maxsize)       # 9223372036854775807 on 64-bit (C ssize_t max, NOT the int limit)
x = 10 ** 1000           # fine — arbitrary precision
print(type(x))           # <class 'int'>

one_million = 1_000_000
hex_mask = 0xFF_FF_FF_FF

# int <-> str conversion of huge numbers is limited (since 3.11, CVE-2020-10735)
print(sys.get_int_max_str_digits())   # 4300
try:
    int('9' * 5000)
except ValueError as e:
    print('limit hit:', str(e)[:40])  # limit hit: Exceeds the limit (4300 digits) for inte
```

##### New in 3.15: `math.integer` (PEP 791)

Integer-only functions now live in the `math.integer` submodule. The six functions are `comb`, `factorial`, `gcd`, `isqrt`, `lcm` and `perm`; the old `math.*` names are kept as **soft-deprecated aliases** (no warning, no removal planned — but new code should use `math.integer`).[^13]

```python
import math
from math import integer

print(integer.comb(5, 2), integer.perm(4), integer.factorial(5))   # 10 24 120
print(integer.gcd(12, 18, 24), integer.lcm(4, 6, 10), integer.lcm())   # 6 60 1
print(integer.isqrt(17))           # 4 — floor of the exact square root
print(integer.gcd is math.gcd)     # True — the math.* names are aliases

# isqrt is exact; int(math.sqrt(n)) is not for large n
n = 4503599761588224**2 - 1
print(integer.isqrt(n))            # 4503599761588223 — correct
print(int(math.sqrt(n)))           # 4503599761588224 — WRONG, float rounding

# ceiling square root for positive n
print(1 + integer.isqrt(17 - 1))   # 5
```

`import math` alone also makes `math.integer` available as an attribute. The proposed names `imath`/`intmath` do not exist (`import imath` → `ModuleNotFoundError`). On 3.14, `import math.integer` fails with *"'math' is not a package"* — use `math.gcd` etc. in code that supports older versions.

##### Floats

Floats are C `double` (IEEE 754 binary64). 3.15 adds `math.isnormal()`, `math.issubnormal()`, `math.fmax()`, `math.fmin()` and `math.signbit()`:[^1]

```python
import math

print(math.isclose(0.1 + 0.2, 0.3))            # True — never compare floats with ==
print(math.isnormal(1.0), math.issubnormal(5e-324))   # True True  (new in 3.15)
print(math.signbit(-0.0), -0.0 < 0.0)          # True False — detects negative zero
print(math.fmax(float('nan'), 1.0))            # 1.0 — fmax/fmin ignore a NaN operand (new in 3.15)
print(max(float('nan'), 1.0))                  # nan — builtin max() depends on argument order
print(math.nextafter(1.0, 2.0) - 1.0 == math.ulp(1.0))   # True
```

`time.sleep()` and other functions taking timestamps/timeouts now accept any real number — `Decimal`, `Fraction` — not just `int`/`float` (no precision gain).

##### Booleans

`bool` is a subtype of `int`. **`~True` / `~False` is deprecated (since 3.12) and the deprecation is scheduled to take effect in 3.16** — it returns `-2`/`-1`, which is almost never what was meant:

```python
import warnings

print(True + True)          # 2
with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter('always')
    value = ~True
print(value, w[0].category.__name__)   # -2 DeprecationWarning
print(not True)             # False — what was meant
```

**Falsy values:** `None`, `False`, numeric zeros, empty sequences/collections (including an empty `frozendict()`), and objects whose `__bool__` returns `False` or `__len__` returns 0. **`NotImplemented` in a boolean context raises `TypeError`** (since 3.14). Sentinels created with `sentinel()` are truthy.

##### `bytes` and `bytearray` — New in 3.15: `bytearray.take_bytes()`

`bytearray.take_bytes(n=None, /)` removes and returns the first `n` bytes (or all) **without copying** — the idiom for buffers in protocol parsers and stream decoders:[^1]

```python
buffer = bytearray(b'abc\ndef')
n = buffer.find(b'\n')
line = buffer.take_bytes(n + 1)        # replaces bytes(buffer[:n+1]); del buffer[:n+1]
print(line, buffer)                    # b'abc\n' bytearray(b'def')

rest = buffer.take_bytes()             # replaces bytes(buffer); buffer.clear()
print(rest, buffer, type(rest).__name__)   # b'def' bytearray(b'') bytes

print(b'aaa'.replace(b'a', b'b', count=2))   # b'bba' — count is now a keyword (3.15)
```

***

#### Strings

##### The Two Types

- **`str`**: a sequence of Unicode code points (Unicode 17.0 database in 3.15).
- **`bytes`**: a sequence of raw 8-bit values. **Never mixed implicitly with `str`.**

The Unicode sandwich still applies: decode at input boundaries, work in `str`, encode at output boundaries. In 3.15 the *default* for every boundary is UTF-8 — but explicit `encoding='utf-8'` remains the reviewable, version-portable choice.

```python
text = 'café'
data = text.encode('utf-8')
print(data, data.decode('utf-8'))      # b'caf\xc3\xa9' café

# New in 3.15: grapheme clusters (what users perceive as "characters")
import unicodedata
family = '👨‍👩‍👧'                        # 5 code points joined by ZWJ
print(len(family))                      # 5
print(len(list(unicodedata.iter_graphemes(family))))   # 1
```

Use `unicodedata.iter_graphemes()` (new in 3.15) when truncating or counting user-visible text — slicing a `str` can cut emoji sequences and combining marks in half.

##### String Methods (Quick Reference)

```python
s = 'Hello, World!'

s.casefold()                        # aggressive lowercase — use for case-insensitive comparison
'https://x.com'.removeprefix('https://')   # 'x.com' — never lstrip('https://')
'file.tar.gz'.removesuffix('.gz')          # 'file.tar'
s.find('World')                     # 7  (-1 if absent)
s.index('World')                    # 7  (ValueError if absent)
s.split(', ')                       # ['Hello', 'World!']
s.partition(', ')                   # ('Hello', ', ', 'World!')
', '.join(['a', 'b'])               # 'a, b'
s.startswith(('Hello', 'Hi'))       # True — tuple of prefixes
'42'.zfill(6)                       # '000042'
'²'.isdigit(), '²'.isdecimal()      # (True, False) — use isdecimal() before int()
```

##### f-strings (PEP 701, 3.12+)

```python
name, score = 'Alice', 98.765
items = {'key': 'value'}
lines = ['a', 'b']

print(f'{score:.2f}')                  # 98.77
print(f'{items['key']}')               # value — same quotes inside are fine since 3.12
print(f'{'\n'.join(lines)!r}')         # 'a\nb' — backslashes allowed since 3.12
x = 42
print(f'{x = }')                       # x = 42 — debugging only, never in logs
```

##### Template Strings (PEP 750, 3.14+)

A `t'...'` literal produces a `string.templatelib.Template` instead of a `str`, so the receiving function can escape interpolations. Use them for SQL, HTML, shell commands — anywhere f-string interpolation would be an injection risk. 3.15 adds t-string support to `pprint`.[^26]

```python
from string.templatelib import Interpolation, Template
import html

def safe_html(template: Template) -> str:
    out = []
    for part in template:
        if isinstance(part, Interpolation):
            out.append(html.escape(str(part.value)))
        else:
            out.append(part)
    return ''.join(out)

user = '<script>alert(1)</script>'
print(safe_html(t'<p>Hello {user}</p>'))
# <p>Hello &lt;script&gt;alert(1)&lt;/script&gt;</p>

tpl = t'Hi {user}!'
print(type(tpl).__name__, tpl.strings)   # Template ('Hi ', '!')
```

**Pitfall:** a `Template` is not a `str`. Passing one to an API expecting `str` (`logging`, `print`, `str.format`) does not render it — the API must explicitly process templates.

***

#### Type Hints

##### Annotations Are Lazy (PEP 649/749, 3.14+)

Annotations are evaluated only when requested, so forward references need no quotes and `from __future__ import annotations` is unnecessary in 3.14+ code. Use `annotationlib` to read annotations safely:[^25]

```python
import annotationlib

def handler(request: Request) -> Response:      # Request/Response not defined yet: fine
    ...

try:
    handler.__annotations__                      # evaluates -> NameError
except NameError as e:
    print('NameError:', e)                       # NameError: name 'Request' is not defined

print(annotationlib.get_annotations(handler, format=annotationlib.Format.STRING))
# {'request': 'Request', 'return': 'Response'}
fwd = annotationlib.get_annotations(handler, format=annotationlib.Format.FORWARDREF)
print(type(fwd['request']).__name__)             # ForwardRef

class Request: ...
class Response: ...
print(annotationlib.get_annotations(handler))   # VALUE format now succeeds
```

`from __future__ import annotations` still works in 3.15 (no warning) and turns annotations into strings; mixing it into new 3.14+ code just loses the benefits of PEP 649. Leave it only where code must still support ≤3.13.

##### Modern Generic Syntax (PEP 695, 3.12+)

```python
from collections.abc import Callable, Iterable

type JSON = dict[str, JSON] | list[JSON] | str | int | float | bool | None

def first[T](items: Iterable[T]) -> T:
    for item in items:
        return item
    raise ValueError('empty')

class Registry[K, V]:
    def __init__(self) -> None:
        self._items: dict[K, V] = {}

    def register(self, key: K) -> Callable[[V], V]:
        def deco(value: V) -> V:
            self._items[key] = value
            return value
        return deco

print(first([3, 4]), JSON.__name__)   # 3 JSON
```

Write `X | None`, never `Optional[X]`, and import ABCs (`Callable`, `Iterable`, `Mapping`) from `collections.abc`, not `typing`.

##### New in 3.15: `TypedDict` with `closed` and `extra_items` (PEP 728)

A **closed** `TypedDict` declares that no keys beyond the listed ones may appear; `extra_items=T` allows arbitrary extra keys whose values must be `T`. The checks are for **static type checkers** — at runtime a `TypedDict` is still a plain `dict` and nothing is validated.[^14]

```python
from typing import NotRequired, ReadOnly, TypedDict

class Movie(TypedDict, closed=True):
    name: str
    year: int

class Labels(TypedDict, extra_items=str):       # any extra key, value must be str
    app: str

class Meta(TypedDict, extra_items=ReadOnly[int]):
    version: int

print(Movie.__closed__, Movie.__extra_items__)  # True NoExtraItems
print(Labels.__closed__, Labels.__extra_items__)   # None <class 'str'>

m: Movie = {'name': 'Blade Runner', 'year': 1982, 'rating': 8}   # type checker: error
print(m['rating'])                              # 8 — NOT enforced at runtime

try:
    class Both(TypedDict, closed=True, extra_items=int):
        pass
except TypeError as e:
    print(e)                                    # Cannot combine closed=True and extra_items

Functional = TypedDict('Functional', {'a': int}, closed=True)
print(Functional.__closed__)                    # True
```

Use `closed=True` for API payloads where an unexpected key is a bug (so `**kwargs: Unpack[Movie]` and assignments to `Mapping[str, int | str]` type-check precisely); use `extra_items=` for label/annotation maps with a fixed value type. Validate untrusted JSON at runtime with a real validator — `TypedDict` is not one.

Removed in 3.15 (see migration notes): `TypedDict('TD')` and `TypedDict('TD', None)` — write `TypedDict('TD', {})` or a class with `pass`.

##### New in 3.15: `typing.TypeForm` (PEP 747)

`TypeForm[T]` annotates a value that is itself a type expression (`int`, `str | None`, `list[int]`, a `TypedDict` class). At runtime `TypeForm(x)` just returns `x`.[^15]

```python
from typing import Any, TypeForm

def cast_to[T](typ: TypeForm[T], value: Any) -> T:
    return value      # a real implementation would validate `value` against `typ`

print(TypeForm(int | None))          # int | None — identity at runtime
print(cast_to(int, 3))               # 3
print(cast_to(list[int], [1, 2]))    # [1, 2]
```

Prefer `TypeForm[T]` over `type[T]` in validators/deserializers that accept unions, generics or `Literal[...]` — `type[T]` only admits classes.

##### New in 3.15: `@typing.disjoint_base` (PEP 800) and `TypeVarTuple` Options

`@disjoint_base` tells type checkers that a class cannot be combined with other disjoint bases in multiple inheritance (mirroring builtins/C extensions with incompatible layouts). It's for stub/library authors; application code rarely needs it. `TypeVarTuple` now accepts `bound=`, `covariant=`, `contravariant=` and `infer_variance=` like `TypeVar` (bound semantics are not yet specified for type checkers).[^16]

```python
from typing import TypeVarTuple, disjoint_base

@disjoint_base
class Buffer:
    __slots__ = ('data',)

print(Buffer.__disjoint_base__)              # True
Ts = TypeVarTuple('Ts', bound=int)
print(Ts.__bound__)                          # <class 'int'>
```

##### Runtime Typing Changes in 3.15

```python
import warnings
from typing import Protocol, runtime_checkable

@runtime_checkable
class Closeable(Protocol):
    def close(self) -> None: ...

class NamedCloseable(Closeable, Protocol):   # inherits, but is NOT itself decorated
    name: str

with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter('always')
    isinstance(open('/dev/null'), NamedCloseable)
print(w[0].category.__name__)   # DeprecationWarning — becomes TypeError in 3.20

# ✅ decorate every protocol you use with isinstance()/issubclass()
@runtime_checkable
class NamedCloseable2(Closeable, Protocol):
    name: str
```

- `typing.ByteString` / `collections.abc.ByteString`: **merely importing or accessing them now emits `DeprecationWarning`**; removal is in **3.17**. Use `collections.abc.Buffer` for `isinstance` checks and `bytes | bytearray | memoryview` in annotations.
- Protocol type-parameter order is now inferred correctly (`class B(A[T2], Protocol[T1, T2])` has parameters `(T1, T2)`), and a `Protocol[...]` that omits a type variable used by another base raises `TypeError`.
- `typing.no_type_check_decorator` was **removed**; keyword-argument `NamedTuple('P', x=int)` is **removed** (`TypeError`).
- `slice` is now generic: `slice[int]` is valid in annotations.

##### `Self`, `override`, `TypeIs`, `ReadOnly`, `deprecated` (3.11–3.13)

```python
from typing import ReadOnly, Self, TypedDict, TypeIs, override
from warnings import deprecated

class Builder:
    def __init__(self) -> None:
        self.parts: list[str] = []

    def add(self, part: str) -> Self:
        self.parts.append(part)
        return self

class LoudBuilder(Builder):
    @override                      # type checker errors if Builder.add disappears
    def add(self, part: str) -> Self:
        return super().add(part.upper())

def is_str_list(val: list[object]) -> TypeIs[list[str]]:
    return all(isinstance(x, str) for x in val)

class Config(TypedDict):
    name: ReadOnly[str]

@deprecated('Use new_api() instead')
def old_api() -> None: ...

print(LoudBuilder().add('a').add('b').parts)   # ['A', 'B']
```

***

#### Lists

```python
lst: list[int] = [1, 2, 3, 4, 5]

print(lst[-1], lst[1:3], lst[::-1])   # 5 [2, 3] [5, 4, 3, 2, 1]

lst.append(6)
lst.extend([7, 8])
lst.remove(3)                  # first occurrence; ValueError if absent
popped = lst.pop()             # last element

nums = [3, 1, 4, 1, 5, 9]
nums.sort(key=lambda x: -x)    # in-place, stable
sorted_copy = sorted(nums)     # new list

nested = [[1, 2], [3, 4]]
print([*row for row in nested])        # [1, 2, 3, 4] — 3.15 comprehension unpacking
```

**Pitfall: list multiplication shares references:**

```python
matrix = [[0] * 3] * 3            # ❌ three references to ONE row
matrix[0][0] = 99
print(matrix)                     # [[99, 0, 0], [99, 0, 0], [99, 0, 0]]

matrix = [[0] * 3 for _ in range(3)]   # ✅
matrix[0][0] = 99
print(matrix)                     # [[99, 0, 0], [0, 0, 0], [0, 0, 0]]
```

***

#### Tuples

```python
from typing import NamedTuple

t = (1, 2, 3)
single = (42,)
first, *rest = [1, 2, 3, 4]
print(first, rest)                 # 1 [2, 3, 4]

class Point(NamedTuple):
    x: float
    y: float
    label: str = ''

p = Point(1.0, 2.0, 'origin')
print(p._replace(x=5.0))           # Point(x=5.0, y=2.0, label='origin')

# New in 3.15: subclasses of tuple (including namedtuples) may define any __slots__
class Tagged(Point):
    __slots__ = ('tag',)
tp = Tagged(1.0, 2.0)
tp.tag = 'a'
print(tp.tag)                      # a
```

Removed in 3.15: `NamedTuple('Point', x=int, y=int)` keyword syntax — use the class form or `NamedTuple('Point', [('x', int), ('y', int)])`.

***

#### Dictionaries

```python
d: dict[str, int] = {'a': 1, 'b': 2}
print(d.get('c', 0))                    # 0
d |= {'c': 3}                           # in-place merge (3.9+)
merged = d | {'a': 100}                 # new dict, right side wins
print(merged)                           # {'a': 100, 'b': 2, 'c': 3}

configs = [{'host': 'a'}, {'port': 1}, {'host': 'b'}]
print({**c for c in configs})           # {'host': 'b', 'port': 1} — 3.15 unpacking

frozen = frozendict(d)                  # 3.15: immutable, hashable snapshot
print(frozen)                           # frozendict({'a': 1, 'b': 2, 'c': 3})
```

**Pitfall: never modify a dict while iterating it** (`RuntimeError: dictionary changed size during iteration`) — rebuild with a comprehension.

##### `collections`

```python
from collections import ChainMap, Counter, defaultdict, deque

word_count: defaultdict[str, int] = defaultdict(int)
for word in 'hello world hello'.split():
    word_count[word] += 1

c = Counter('abracadabra')
print(c.most_common(2))                           # [('a', 5), ('b', 2)]

# New in 3.15: symmetric difference of multisets
print(Counter(a=3, b=1) ^ Counter(a=1, c=2))      # Counter({'a': 2, 'c': 2, 'b': 1})

env = ChainMap({'color': 'red'}, {'color': 'blue', 'user': 'guest'})
print(env['color'], env['user'])                  # red guest

q: deque[str] = deque(maxlen=1000)                # O(1) appends/pops at both ends
q.append('item')
```

`Counter.__xor__` keeps, for each element, the absolute difference of the counts (`|3-1| = 2` for `a`).

***

#### Sets

```python
a, b = {1, 2, 3, 4}, {3, 4, 5, 6}
print(a | b, a & b, a - b, a ^ b)   # {1, 2, 3, 4, 5, 6} {3, 4} {1, 2} {1, 2, 5, 6}
empty = set()                       # NOT {} — that's an empty dict
fs = frozenset([1, 2, 3])           # immutable, hashable
print({*s for s in [{1}, {2, 3}]})  # {1, 2, 3} — 3.15 unpacking
```

***

#### Control Flow

```python
# Structural pattern matching (3.10+)
def handle(event: dict) -> str:
    match event:
        case {'type': 'click', 'pos': (x, y)}:
            return f'click at {x},{y}'
        case {'type': 'key', 'key': str(k)} if k.isalpha():
            return f'key {k}'
        case {'type': 'scroll', 'delta': +1}:      # unary plus in literal patterns: new in 3.15
            return 'scroll down'
        case _:
            return 'unknown'

print(handle({'type': 'click', 'pos': (1, 2)}))   # click at 1,2
print(handle({'type': 'scroll', 'delta': 1}))     # scroll down

# for / else — else runs only when the loop did not break
for n in [2, 4, 6]:
    if n % 2:
        break
else:
    print('all even')                             # all even

# Walrus in while
import io
stream = io.BytesIO(b'x' * 100_000)
while chunk := stream.read(65_536):
    pass
```

`case +1:` was a `SyntaxError` before 3.15 — avoid it in code that supports older versions.

##### Exceptions Without Parentheses (PEP 758, 3.14+) and `finally` Control Flow (PEP 765, 3.14+)

```python
try:
    int('x')
except ValueError, TypeError:          # 3.14+: same as `except (ValueError, TypeError):`
    print('bad number')                # bad number

# `as` still requires parentheses: `except (ValueError, TypeError) as e:`

import warnings
with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter('always')
    compile('def f():\n    try:\n        return 1\n    finally:\n        return 2\n', 'm', 'exec')
print(w[0].category.__name__)          # SyntaxWarning — return/break/continue in finally
```

`return`, `break` or `continue` inside `finally` silently swallows the in-flight exception; it is a `SyntaxWarning` since 3.14. Treat it as an error in review.

***

#### Functions

```python
def greet(name: str, greeting: str = 'Hello', /, *, punctuation: str = '!') -> str:
    """Positional-only before `/`, keyword-only after `*`."""
    return f'{greeting}, {name}{punctuation}'

print(greet('Alice', punctuation='.'))   # Hello, Alice.

# Mutable default trap — still the #1 function bug
def append_bad(item, bucket=[]):        # ❌ shared across calls
    bucket.append(item)
    return bucket

print(append_bad(1), append_bad(2))     # [1, 2] [1, 2]

def append_good(item, bucket: list | None = None):   # ✅
    bucket = [] if bucket is None else bucket
    bucket.append(item)
    return bucket

# When None is a valid argument, use a 3.15 sentinel instead
UNSET = sentinel('UNSET')

def set_timeout(value: float | None | UNSET = UNSET) -> str:
    if value is UNSET:
        return 'keep current timeout'
    return 'disable timeout' if value is None else f'timeout={value}'

print(set_timeout(), '|', set_timeout(None))   # keep current timeout | disable timeout
```

##### Closures

```python
funcs = [lambda: i for i in range(3)]
print([f() for f in funcs])        # [2, 2, 2] — late binding ❌

funcs = [lambda i=i: i for i in range(3)]
print([f() for f in funcs])        # [0, 1, 2] ✅
```

##### `functools`

```python
import functools

@functools.cache                        # unbounded — only for bounded argument spaces
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)

@functools.lru_cache(maxsize=1024)      # bounded — for open-ended inputs
def lookup(key: str) -> str:
    return key.upper()

# Placeholder (3.14+) reserves positional slots in partial()
square = functools.partial(pow, functools.Placeholder, 2)
print(square(7))                        # 49

class Formatter:
    @functools.singledispatchmethod
    def fmt(self, arg: object) -> str:
        return str(arg)

    @fmt.register
    def _(self, arg: int) -> str:
        return f'int:{arg}'

print(Formatter().fmt(3), Formatter().fmt('x'))   # int:3 x
```

3.15: `singledispatchmethod` supports non-descriptor callables and, when it wraps a regular method accessed through the class, dispatches on the second argument.[^1] `functools.cache` arguments must be hashable — pass `frozendict`/`tuple` instead of `dict`/`list`.

##### Generators and `itertools`

```python
import itertools
from collections.abc import Iterator

def fibonacci() -> Iterator[int]:
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

print(list(itertools.islice(fibonacci(), 8)))          # [0, 1, 1, 2, 3, 5, 8, 13]
print(list(itertools.batched(range(7), 3)))             # [(0, 1, 2), (3, 4, 5), (6,)]
print(list(itertools.pairwise('abc')))                  # [('a', 'b'), ('b', 'c')]
try:
    list(itertools.batched(range(7), 3, strict=True))   # strict= since 3.13
except ValueError as e:
    print(e)                                            # batched(): incomplete batch
print(list(map(lambda a, b: a + b, [1, 2], [3, 4], strict=True)))   # [4, 6] — 3.14+
```

**Generators are not thread-safe** — concurrent `next()` calls from several threads raise `ValueError: generator already executing` (and on the free-threaded build iterators in general may duplicate/skip items). Use the new `threading.serialize_iterator` (see Concurrency).

***

#### Dataclasses

```python
import copy
from dataclasses import KW_ONLY, dataclass, field

@dataclass(slots=True, kw_only=True)
class User:
    id: int
    name: str
    email: str
    tags: list[str] = field(default_factory=list)
    settings: frozendict = frozendict(theme='dark')   # 3.15: hashable immutable default is allowed

    def __post_init__(self) -> None:
        self.email = self.email.lower()

@dataclass(frozen=True, order=True)
class Version:
    major: int
    minor: int
    patch: int = 0

u = User(id=1, name='A', email='A@X.COM')
print(u.email, u.settings['theme'])       # a@x.com dark
v = Version(3, 15)
print(copy.replace(v, patch=1))           # Version(major=3, minor=15, patch=1) — 3.13+
print(sorted([Version(3, 15), Version(3, 9)])[0])   # Version(major=3, minor=9, patch=0)

try:
    @dataclass
    class Bad:
        items: list[int] = []
except ValueError as e:
    print(e)   # mutable default <class 'list'> for field items is not allowed: use default_factory
```

`slots=True` (3.10+) saves memory and catches attribute typos; `kw_only=True` avoids positional-order bugs when fields are added. 3.15 cleaned up the annotations of generated `__init__` methods (no internal type names leak into `__init__.__annotations__`).

***

#### Classes and OOP

##### Class Definition

```python
import math
from typing import Self

class Shape:
    count: int = 0          # class attribute — shared

    def __init__(self, color: str = 'black') -> None:
        self.color = color
        Shape.count += 1

    def __repr__(self) -> str:
        return f'{type(self).__name__}(color={self.color!r})'

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, type(self)):
            return NotImplemented
        return self.__dict__ == other.__dict__
    # Defining __eq__ sets __hash__ = None: correct for a MUTABLE class.

class Circle(Shape):
    def __init__(self, radius: float, **kwargs) -> None:
        super().__init__(**kwargs)
        self.radius = radius

    @property
    def area(self) -> float:
        return math.pi * self.radius ** 2

    @classmethod
    def unit(cls) -> Self:
        return cls(1.0)

print(Circle.unit(), round(Circle(2).area, 2))   # Circle(color='black') 12.57
print(Shape.__hash__)                             # None
```

**3.15 slot changes:** any class may now list `__dict__` and `__weakref__` in `__slots__`, and subclasses of `tuple` (including `namedtuple` classes) may define arbitrary `__slots__`.[^1]

##### Improved `AttributeError` Hints (New in 3.15)

```python
import traceback
from dataclasses import dataclass

@dataclass
class Circle:
    radius: float
    @property
    def area(self) -> float:
        return 3.14159 * self.radius ** 2

class Container:
    def __init__(self, inner: Circle) -> None:
        self.inner = inner

for expr in ['Container(Circle(1.0)).area', '[1, 2].push(3)', '{}.put("a", 1)', '(1, 2).append(3)']:
    try:
        eval(expr)
    except AttributeError as e:
        # The hint is added when the traceback is FORMATTED — str(e) does not contain it
        print(traceback.format_exception_only(e)[-1].strip())
# AttributeError: 'Container' object has no attribute 'area'. Did you mean '.inner.area' instead of '.area'?
# AttributeError: 'list' object has no attribute 'push'. Did you mean '.append'?
# AttributeError: 'dict' object has no attribute 'put'. Use d[k] = v.
# AttributeError: 'tuple' object has no attribute 'append'. Did you mean to use a 'list' object?
```

The hints appear in tracebacks (and `traceback.format_exception*()`), not in `str(exc)` — so logging `str(e)` loses them; use `logger.exception()`. Don't write tests that assert exact `AttributeError` message text — these hints change between versions.

##### Abstract Base Classes and Enums

```python
from abc import ABC, abstractmethod
from enum import Enum, Flag, IntEnum, StrEnum, auto

class Repository(ABC):
    @abstractmethod
    def get(self, id: int) -> dict[str, object] | None: ...

class Color(StrEnum):           # 3.11+: members are str
    RED = auto()
    GREEN = auto()

class Permission(Flag):
    READ = auto()
    WRITE = auto()

print(Color.RED, Color.RED == 'red')                # red True
print(Permission.READ in (Permission.READ | Permission.WRITE))   # True
try:
    Repository()
except TypeError as e:
    print(e)   # Can't instantiate abstract class Repository without an implementation for abstract method 'get'
```

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
        err.add_note(f'raw value was {raw!r}')     # 3.11+
        raise err from e                            # keep the cause

try:
    parse_port('N/A')
except ConfigError as e:
    print(e, e.__notes__, type(e.__cause__).__name__)
# Config error for 'port': must be an integer ["raw value was 'N/A'"] ValueError

# Exception groups (3.11+) — e.g. from asyncio.TaskGroup
try:
    raise ExceptionGroup('batch', [ValueError('a'), KeyError('b')])
except* ValueError as eg:
    print('values:', [str(x) for x in eg.exceptions])   # values: ['a']
except* KeyError as eg:
    print('keys:', len(eg.exceptions))                  # keys: 1
```

- Always re-raise with bare `raise` (not `raise e`) after logging.
- **New in 3.15:** `ImportError`/`ModuleNotFoundError` repr shows `name=`/`path=` when passed as keywords: `repr(ImportError('x', name='mod'))` → `ImportError('x', name='mod')`.
- Lazy imports change *where* `ImportError` surfaces (first use, chained to an `ImportError` "lazy import of 'x' raised an exception during resolution"). Broad `except Exception:` blocks around business logic can now accidentally swallow a missing-dependency error.

***

#### Context Managers

```python
from contextlib import ExitStack, contextmanager, suppress
import os
import time

with (
    open(os.devnull, encoding='utf-8') as fin,     # parenthesized multi-with (3.10+)
    open(os.devnull, 'w', encoding='utf-8') as fout,
):
    fout.write(fin.read())

@contextmanager
def timed(name: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        print(f'{name}: {time.perf_counter() - start:.0f}s')

with timed('noop'):
    pass                                # noop: 0s

with suppress(FileNotFoundError):
    os.remove('/tmp/definitely-not-here-3.15')

with ExitStack() as stack:
    files = [stack.enter_context(open(os.devnull, encoding='utf-8')) for _ in range(3)]
```

##### New in 3.15: Context-Manager Decorators Respect Generators and Coroutines

`ContextDecorator`/`AsyncContextDecorator` — and therefore `@contextmanager`/`@asynccontextmanager` objects used **as decorators** — now detect generator, coroutine and async-generator functions and keep the context open across iteration/await. Previously the context exited as soon as the generator object was created, so the body ran *outside* it.[^30]

```python
from contextlib import contextmanager

events: list[str] = []

@contextmanager
def tracked():
    events.append('enter')
    try:
        yield
    finally:
        events.append('exit')

@tracked()
def produce():
    events.append('body-start')
    yield 1
    events.append('body-end')

gen = produce()
events.append('created')
list(gen)
print(events)
# 3.15: ['created', 'enter', 'body-start', 'body-end', 'exit']
# 3.14: ['enter', 'exit', 'created', 'body-start', 'body-end']  — the old, broken behavior
```

Review impact: decorating a generator/`async def` with a transaction, lock or timer context manager is now correct on 3.15 — but **silently wrong on ≤3.14**. In multi-version code, put the `with` inside the function body instead. 3.15 also makes `ExitStack`/`AsyncExitStack` accept arbitrary descriptors for `__enter__`/`__exit__`, matching the `with` statement.

***

#### File I/O and `pathlib`

```python
import os
import tempfile
from pathlib import Path

base = Path(tempfile.mkdtemp())
report = base / 'reports' / 'output.csv'

print(report.name, report.stem, report.suffix)   # output.csv output .csv

report.parent.mkdir(parents=True, exist_ok=True)
report.write_text('a,b\n', encoding='utf-8')     # explicit encoding: portable to ≤3.14
print(report.read_text())                        # a,b  (UTF-8 by default in 3.15)

# 3.14+: copy/move without shutil
backup = report.copy(base / 'backup.csv')
moved = backup.move(base / 'archive.csv')
print(moved.exists(), backup.exists())           # True False

# New in 3.15: parent_mode for intermediate directories
deep = base / 'a' / 'b' / 'c'
deep.mkdir(mode=0o700, parents=True, parent_mode=0o750)
print(oct(deep.stat().st_mode & 0o777), oct(deep.parent.stat().st_mode & 0o777))   # 0o700 0o750
os.makedirs(base / 'x' / 'y', mode=0o700, parent_mode=0o700)   # same for os.makedirs

report.unlink(missing_ok=True)
```

Without `parent_mode`, intermediate directories are created with the default mode (subject to umask), not `mode` — a common source of overly permissive directory trees. Removed in 3.15: `PurePath.is_reserved()` (use `os.path.isreserved()`, Windows), `glob.glob0()`/`glob.glob1()` (use `glob.glob(..., root_dir=...)`).

##### Path Traversal Checks

```python
from pathlib import Path
import os
import tempfile

def safe_resolve(base_dir: str, user_path: str) -> Path:
    base = Path(base_dir).resolve()
    target = (base / user_path).resolve()
    if not target.is_relative_to(base):          # lexical check AFTER resolve()
        raise PermissionError(f'Access denied: {user_path!r}')
    return target

root = tempfile.mkdtemp()
print(safe_resolve(root, 'a/b.txt').name)        # b.txt
try:
    safe_resolve(root, '../../etc/passwd')
except PermissionError as e:
    print(e)                                     # Access denied: '../../etc/passwd'

# ❌ os.path.commonprefix is character-based — and deprecated (warns) in 3.15
print(os.path.commonpath(['/srv/app', '/srv/apple']))   # /srv  ✅ component-based
```

`os.path.commonprefix(['/srv/app', '/srv/apple'])` returns `'/srv/app'` — a classic traversal bug; 3.15 emits `DeprecationWarning` for it. `os.path.realpath(path, strict=os.path.ALLOW_MISSING)` resolves symlinks while tolerating a missing final path (CVE-2025-4517 fix; also backported to security releases of older branches).

##### Archives: `tarfile` Hardening

Since 3.14, `extract()`/`extractall()` default to the `'data'` filter. `data_filter()` also normalizes symlink targets (CVE-2025-4138), the filter is re-applied when substituting links (`LinkFallbackError`, CVE-2025-4330), and rejected members are no longer extracted when `errorlevel=0` (CVE-2025-4435). These fixes are listed in the 3.15 What's New but are **not 3.15-only**: they already shipped in 3.14.0 (see the 3.14 What's New) and were backported to security releases of older branches.[^1][^24]

```python
import io
import tarfile
import tempfile

buf = io.BytesIO()
with tarfile.open(fileobj=buf, mode='w') as tf:
    info = tarfile.TarInfo('../evil.txt')
    info.size = 1
    tf.addfile(info, io.BytesIO(b'x'))
buf.seek(0)

with tempfile.TemporaryDirectory() as dest, tarfile.open(fileobj=buf) as tf:
    try:
        tf.extractall(dest)                       # 'data' filter applied by default
    except tarfile.OutsideDestinationError as e:
        print(type(e).__name__)                   # OutsideDestinationError
```

Never pass `filter='fully_trusted'` for archives from users or the network.

##### Temporary Files

```python
import tempfile

with tempfile.NamedTemporaryFile('w', suffix='.csv', encoding='utf-8', delete=True) as tmp:
    tmp.write('col1,col2\n')
    tmp.flush()

with tempfile.TemporaryDirectory() as tmpdir:
    pass
# NEVER tempfile.mktemp() — TOCTOU race
```

***

#### `async`/`await` and `asyncio`

##### Structured Concurrency — New in 3.15: `TaskGroup.cancel()`

`TaskGroup.cancel()` ends a task group early (e.g. once one task found the answer) without the old "raise a private exception and suppress it" boilerplate:[^1]

```python
import asyncio

async def probe(name: str, delay: float) -> str:
    await asyncio.sleep(delay)
    return name

async def first_responder() -> str:
    result: str | None = None
    async with asyncio.TaskGroup() as tg:
        async def run(name: str, delay: float) -> None:
            nonlocal result
            value = await probe(name, delay)
            if result is None:
                result = value
                tg.cancel()                 # cancels the remaining tasks, exits cleanly
        for name, delay in [('slow', 0.5), ('fast', 0.01), ('slower', 1.0)]:
            tg.create_task(run(name, delay))
    return result

print(asyncio.run(first_responder()))       # fast
```

##### Core Patterns

```python
import asyncio

async def fetch(i: int) -> int:
    await asyncio.sleep(0.01)
    return i * 2

async def main() -> None:
    sem = asyncio.Semaphore(10)                       # bounded concurrency

    async def bounded(i: int) -> int:
        async with sem:
            return await fetch(i)

    async with asyncio.timeout(5):                    # 3.11+, preferred over wait_for
        async with asyncio.TaskGroup() as tg:         # 3.11+, preferred over gather
            tasks = [tg.create_task(bounded(i)) for i in range(5)]
    print([t.result() for t in tasks])                # [0, 2, 4, 6, 8]

    data = await asyncio.to_thread(sum, range(1000))  # blocking work off the loop
    print(data)                                       # 499500

asyncio.run(main())

async def worker() -> None:
    try:
        await asyncio.sleep(1)
    except asyncio.CancelledError:
        # clean up, then ALWAYS re-raise
        raise
```

##### asyncio APIs That Disappear in 3.16

These still work in 3.15 but emit `DeprecationWarning` and are **removed in 3.16** — flag every use:[^1]

| Deprecated (removed in 3.16) | Replacement |
|---|---|
| `asyncio.iscoroutinefunction()` | `inspect.iscoroutinefunction()` |
| `asyncio.get_event_loop_policy()` / `set_event_loop_policy()` | `asyncio.run(..., loop_factory=...)` / `asyncio.Runner(loop_factory=...)` |
| `asyncio.AbstractEventLoopPolicy`, `DefaultEventLoopPolicy`, `WindowsSelectorEventLoopPolicy`, `WindowsProactorEventLoopPolicy` | `loop_factory=` (e.g. `asyncio.SelectorEventLoop`) |

```python
import asyncio
import inspect

async def main() -> str:
    return 'ok'

print(inspect.iscoroutinefunction(main))                          # True ✅
print(asyncio.run(main(), loop_factory=asyncio.SelectorEventLoop))   # ok ✅ (instead of a policy)
```

`asyncio.get_event_loop()` raises `RuntimeError` when no loop is running (since 3.14) — use `asyncio.get_running_loop()` inside coroutines and `asyncio.run()` at the top. Sampling-profile async code with `python -m profiling.sampling run --async-aware` (see Profiling).

***

#### Concurrency

##### Free-Threading Status in 3.15

The free-threaded ("no-GIL") build is **officially supported but optional** — PEP 779 "phase II", reached in 3.14; 3.15 does **not** make it the default. It is a separate interpreter (`python3.15t`, ABI tag `cp315t`); the default `python3.15` still has a GIL.[^19][^20]

What changed for free-threading in 3.15:

- **Stable ABI for free-threaded builds (`abi3t`, PEP 803).** Extensions can ship one wheel for all free-threaded 3.15+ interpreters (after porting to PEP 697/793/820 APIs). Build tools with `abi3t` support include meson-python, scikit-build-core and Maturin; otherwise build `cp315t` wheels separately.[^21]
- The official **macOS installers now install free-threading support by default**.[^1]
- `mimalloc` is the default raw allocator (better free-threaded performance).
- Many free-threading crash fixes continued into 3.15.1 (e.g. concurrent `bytes.join()` mutation, split-dict iteration) — run the latest patch release.[^23]

```python
import sys
import sysconfig

print(sysconfig.get_config_var('Py_GIL_DISABLED'))   # 1 on a free-threaded build, 0 otherwise
print(sys.abi_info.free_threaded)                     # new in 3.15: build-level ABI info
print(sys._is_gil_enabled())                          # runtime: GIL may be re-enabled
```

On a free-threaded build: the GIL is **re-enabled automatically** (with a warning) when an extension that isn't marked free-threading-safe is imported, and can be forced with `PYTHON_GIL=1`/`-X gil=1`; threads inherit the caller's `contextvars` context (`sys.flags.thread_inherit_context == 1`) and `warnings.catch_warnings` is context-local (`sys.flags.context_aware_warnings == 1`) — both are `0` on the default build. Single-threaded overhead is roughly 1–8% depending on platform, and memory use is higher.[^19]

Review rules for code that claims free-threading support:

- Built-in containers are internally locked, but **compound operations are not atomic** (`if k not in d: d[k] = v`, `x += 1` on shared state). Use `threading.Lock`.
- **Sharing one iterator/generator between threads is unsafe** (duplicated or missing items; generators raise `ValueError: generator already executing`). Wrap it with the new 3.15 helpers:[^27]

```python
import threading

@threading.synchronized_iterator           # new in 3.15
def squares(n: int):
    for x in range(n):
        yield x * x

results: list[int] = []
lock = threading.Lock()
source = squares(100)

def consume() -> None:
    for item in source:                    # each item delivered to exactly one thread
        with lock:
            results.append(item)

threads = [threading.Thread(target=consume) for _ in range(4)]
for t in threads: t.start()
for t in threads: t.join()
print(len(results), sum(results) == sum(x * x for x in range(100)))   # 100 True

shared = threading.serialize_iterator(iter(range(5)))   # wrap an existing iterator
left, right = threading.concurrent_tee(range(3))        # every consumer gets every item
print(list(left), list(right))                          # [0, 1, 2] [0, 1, 2]
```

- Accessing `frame.f_locals` of a frame running in another thread may crash on the free-threaded build.
- CPU-bound threading only scales on the `t` build; on the default build use processes or subinterpreters.

##### `threading`

```python
import threading

lock = threading.Lock()
rlock = threading.RLock()            # 3.15: RLock() takes NO arguments (TypeError otherwise)
event = threading.Event()

def worker(n: int) -> None:
    with lock:
        pass

threads = [threading.Thread(target=worker, args=(i,), name=f'worker-{i}') for i in range(4)]
for t in threads: t.start()
for t in threads: t.join()
```

Deprecated aliases (`Thread.setDaemon/isDaemon/getName/setName`, `Event.isSet`, `Condition.notifyAll`, `currentThread`, `activeCount`) are pending removal — use the attribute/snake_case forms.

##### Processes, Interpreters and Executors

```python
import os
from concurrent.futures import InterpreterPoolExecutor, ProcessPoolExecutor
from concurrent.futures.process import BrokenProcessPool

def square(x: int) -> int:
    return x * x

def crash() -> None:
    os._exit(3)

if __name__ == '__main__':   # REQUIRED: forkserver (Linux, 3.14+) and spawn re-import __main__
    with ProcessPoolExecutor(max_workers=2) as pool:
        print(list(pool.map(square, range(5))))          # [0, 1, 4, 9, 16]

    with ProcessPoolExecutor(max_workers=1) as pool:
        try:
            pool.submit(crash).result()
        except BrokenProcessPool as e:
            print(e.__cause__)   # 3.15: names the PID and exit code of the dead worker

    with InterpreterPoolExecutor(max_workers=2) as pool:   # 3.14+: subinterpreters
        print(list(pool.map(square, range(5))))          # [0, 1, 4, 9, 16]
```

Since 3.14 the default `multiprocessing` start method on Linux/BSD is **`forkserver`**, not `fork` (macOS and Windows use `spawn`): worker functions and arguments must be picklable and module-level, and the `__main__` guard is mandatory. 3.15's `BrokenProcessPool` cause reads like `Process 5213 terminated abruptly with exit code 3`.

***

#### New in 3.15: The `profiling` Package and Tachyon (PEP 799)

The new `profiling` package groups the built-in profilers:[^17][^18]

- `profiling.tracing` — the deterministic profiler (the `cProfile` implementation; `cProfile` stays as an alias, `profiling.tracing.Profile is cProfile.Profile`).
- `profiling.sampling` — **Tachyon**, a statistical sampling profiler that can attach to a **running process by PID** with negligible overhead, at up to 1,000,000 Hz.
- The pure-Python **`profile` module is deprecated** (warns on import) and will be **removed in 3.17**.

```python
import io
import pstats
import profiling.tracing

with profiling.tracing.Profile() as pr:
    sum(i * i for i in range(10_000))

out = io.StringIO()
pstats.Stats(pr, stream=out).sort_stats('cumulative').print_stats(5)
print('function calls' in out.getvalue())   # True
```

```bash
# Profile a script or module from the start
python3.15 -m profiling.sampling run --pstats app.py
python3.15 -m profiling.sampling run --flamegraph -o profile.html -m mypkg.cli   # profiler options BEFORE the target; anything after it goes to the program

# Attach to a live production process for 30 s (needs ptrace rights: root,
# CAP_SYS_PTRACE, or kernel.yama.ptrace_scope=0 on Linux)
python3.15 -m profiling.sampling attach --flamegraph -d 30 -o profile.html 12345

# Hung process? One-shot stack dump of every thread / every asyncio task
python3.15 -m profiling.sampling dump -a 12345
python3.15 -m profiling.sampling dump --async-aware 12345

# Modes: --mode wall (default) | cpu | gil | exception ; -a samples all threads
# Outputs: --pstats --collapsed --flamegraph --gecko --heatmap ; --live is a top-like TUI
```

Review guidance: prefer Tachyon for production investigations (attach, no restart, no code change); keep `profiling.tracing`/`cProfile` for exact call counts in tests/benchmarks; replace `import profile` with `profiling.tracing` (same API); container images need `CAP_SYS_PTRACE` for `attach`/`dump`.

***

#### Performance

##### Interpreter and JIT

- **JIT (experimental, off by default).** 3.15's JIT has a new tracing frontend, basic register allocation and better code generation: about 7–8% geometric-mean speedup on x86-64 Linux over the standard interpreter and 11–12% on AArch64 macOS over the tail-calling interpreter, with individual benchmarks ranging from ~15% slower to 2× faster. Enable it per process with `PYTHON_JIT=1` on JIT-capable builds; measure before adopting.[^1]
- **Tail-calling interpreter on Windows.** Official 64-bit Windows binaries are built with MSVC 18's tail-calling interpreter (15–20% pyperformance speedup reported).[^1]
- **Frame pointers by default (PEP 831).** Native profilers (`perf`, eBPF tools) can unwind through Python. Extensions built by tools that consume `sysconfig` inherit `-fno-omit-frame-pointer`; custom native builds should add it too.[^28]
- **GC:** the generational collector (the 3.14 incremental GC was reverted in 3.14.5); `gc.get_threshold()` is `(2000, 10, 10)` again.[^24]

```python
import sys

if hasattr(sys, '_jit'):                     # private, informational only
    print(sys._jit.is_available(), sys._jit.is_enabled())   # e.g. True False (enable: PYTHON_JIT=1)

import gc
print(gc.get_threshold())                    # (2000, 10, 10)
```

##### Startup: Lazy Imports

The biggest *application-level* performance lever in 3.15 is startup time. Measure, then mark heavy imports `lazy`:

```bash
python3.15 -X importtime -c "import myapp.cli" 2> imports.log   # find expensive imports
python3.15 -X lazy_imports=all -m myapp.cli --help               # experiment (watch for side effects!)
```

##### Idioms That Still Matter

```python
import math
from collections import defaultdict

VALID_ROLES = frozenset({'admin', 'editor', 'viewer'})   # O(1) membership
print('admin' in VALID_ROLES)                            # True

parts = [str(i) for i in range(5)]
print(''.join(parts))                                    # 01234 — never += in a loop

total = sum(x * x for x in range(1_000))                 # generator: O(1) memory
print(total)                                             # 332833500

groups: defaultdict[str, list[int]] = defaultdict(list)
for k, v in [('a', 1), ('b', 2), ('a', 3)]:
    groups[k].append(v)
print(dict(groups))                                      # {'a': [1, 3], 'b': [2]}
```

3.15 also speeds up `base64` (2× encode/3× decode), rewrites Base32/Base85/Ascii85 in C (two orders of magnitude faster), and makes `subprocess.Popen.wait(timeout=...)` event-driven (pidfd on Linux ≥ 5.3, kqueue on macOS/BSD) instead of a sleep loop.[^1]

***

#### Logging

```python
import logging

def setup_logging(level: str = 'INFO') -> None:
    logging.basicConfig(
        level=getattr(logging, level.upper()),
        format='%(asctime)s %(name)s %(levelname)s %(message)s',
        force=True,
    )

setup_logging()
logger = logging.getLogger(__name__)

item, count = 'x', 3
logger.debug('Processing %r', item)          # %-style: formatted only if emitted
logger.info('Loaded %d records', count)
try:
    1 / 0
except ZeroDivisionError:
    logger.exception('Unexpected error')     # includes traceback
```

- `Logger.warn()` is deprecated — use `warning()`.
- Custom handlers that take a `strm` argument are deprecated and **break in 3.16** — use `stream`.
- `logging.__version__`/`__date__` are deprecated (removal 3.20).
- Never log a `Template` (t-string) directly — `logging` does not render templates.

***

#### Security

##### Security Content of 3.15 Point Releases

- **3.15.1 (October 21, 2026, expedited):** bundled pip 26.2.2 with certifi 2026.10.19 (removes the distrusted eMudhra/emSign root CA); libexpat updated to 2.9.0; on Windows `mimetypes` no longer reads `mimetypes.knownfiles` by default (registry only). The release notes ask users to **upgrade pip/uv/certifi in every existing virtual environment** too — upgrading the interpreter does not touch venvs.[^4][^23]
- **3.15.0** includes the tarfile hardening listed under File I/O (CVE-2025-4138, CVE-2025-4330, CVE-2025-4435, CVE-2024-12718) and `os.path.realpath(strict=ALLOW_MISSING)` (CVE-2025-4517) — both already present since 3.14.0 — plus Expat billion-laughs/allocation-amplification tuning methods on `xmlparser` objects (documented as new in 3.15, but also backported to 3.14.x/3.13.x patch releases).[^1][^24]

Pin deployments to the latest 3.15.x; treat 3.15.0 as superseded.

##### New in 3.15: Auditable Startup Code (PEP 829)

`.pth` files in site-packages can execute arbitrary code on **every** interpreter start via `import` lines — a known supply-chain persistence vector. 3.15 adds `<name>.start` files containing entry points (`pkg.mod:callable`); when a matching `<name>.start` exists, `import` lines in `<name>.pth` are ignored. `import` lines in `.pth` files are now *silently* deprecated, will be ignored in 3.18 and warn from 3.20.[^22]

```python
import pathlib
import site
import sys
import tempfile

sitedir = pathlib.Path(tempfile.mkdtemp())
(sitedir / 'mypkg_hook.py').write_text('def hook():\n    print("start hook ran")\n', encoding='utf-8')
(sitedir / 'mypkg.start').write_text('mypkg_hook:hook\n', encoding='utf-8')
(sitedir / 'mypkg.pth').write_text('import sys; print("pth import line ran")\n', encoding='utf-8')

site.addsitedir(str(sitedir))
# start hook ran          <- the .pth import line was skipped because mypkg.start exists
print(str(sitedir) in sys.path)   # True — path lines in .pth files keep working
```

Review rule: packages you publish should move `.pth` import hooks to `.start` entry points; audit site-packages for unexpected `.pth`/`.start` files in incident response.

##### Input Validation and Encoding Strictness

```python
import base64
import binascii
import xml

# New in 3.15: canonical=True rejects non-canonical encodings (malleability)
print(base64.b64decode(b'YQ==', canonical=True))      # b'a'
try:
    base64.b64decode(b'YR==', canonical=True)         # same bytes, non-zero padding bits
except binascii.Error as e:
    print(e)                                          # Non-zero padding bits

# 3.15: urlsafe_b64decode no longer requires padding; require it explicitly if needed
print(base64.urlsafe_b64decode('YWI'))                # b'ab'
try:
    base64.urlsafe_b64decode('YWI', padded=True)
except binascii.Error as e:
    print(e)                                          # Incorrect padding

# New in 3.15: XML name/text validation before building documents from user data
print(xml.is_valid_name('user-name'), xml.is_valid_name('1abc'))   # True False
print(xml.is_valid_text('ok'), xml.is_valid_text('bad\x00'))       # True False
```

`urlsafe_b64decode()` accepting `+` and `/` (standard alphabet) is deprecated — it now emits a `FutureWarning`, and in future versions those characters will be errors in strict mode and discarded otherwise.

##### Secrets, SQL, Subprocess

```python
import secrets
import sqlite3
import subprocess
import sys

token = secrets.token_urlsafe(32)               # never random.* for security values
print(secrets.compare_digest(token, token))     # True — constant-time comparison

# 3.15: every sqlite3.connect() parameter except `database` is keyword-only
conn = sqlite3.connect(':memory:', timeout=5.0)
try:
    sqlite3.connect(':memory:', 5.0)
except TypeError as e:
    print(e)                                    # connect() takes at most 1 positional arguments (2 given)

conn.execute('CREATE TABLE users (id INTEGER, name TEXT)')
user_id = '1 OR 1=1'
rows = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,)).fetchall()   # ✅ parameterized
print(rows)                                     # []
conn.close()

result = subprocess.run([sys.executable, '-c', 'print("hi")'],   # ✅ list form, no shell
                        capture_output=True, text=True, timeout=30, check=True)
print(result.stdout.strip())                    # hi
```

3.15 also changed `sqlite3.Connection.create_function()`/`create_aggregate()` (first three parameters positional-only) and `set_authorizer()`/`set_progress_handler()`/`set_trace_callback()` (first parameter positional-only).

##### XML and HTTP

- Untrusted XML: use `defusedxml`, or at minimum the Expat amplification limits (`SetBillionLaughsAttackProtectionMaximumAmplification()`, `SetAllocTrackerMaximumAmplification()` etc.; documented as new in 3.15, also present in recent 3.14.x/3.13.x patch releases).
- `http.server` is not a production server. Its `CGIHTTPRequestHandler` and `--cgi` flag were **removed** in 3.15.
- `http.client.HTTPConnection(..., max_response_headers=N)` (new in 3.15) bounds header floods from untrusted servers.
- `ssl.SSLContext.set_groups()`/`set_ciphersuites()` (3.15) configure TLS 1.3 key-exchange groups (including post-quantum groups with suitable OpenSSL) and cipher suites explicitly.

***

#### JSON and Serialization

```python
import json
from datetime import datetime, timezone

data = {'name': 'Alice', 'tags': ['admin']}
text = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False)
print(json.loads(text) == data)                  # True

# New in 3.15: array_hook — customize decoded JSON arrays
frozen = json.loads('{"a": [1, [2]]}', object_pairs_hook=frozendict, array_hook=tuple)
print(frozen)                                    # frozendict({'a': (1, (2,))})
print(json.dumps(frozen))                        # {"a": [1, [2]]} — frozendict serializes

class AppEncoder(json.JSONEncoder):
    def default(self, obj: object) -> object:
        if isinstance(obj, datetime):
            return obj.isoformat()
        if isinstance(obj, (set, frozenset)):
            return sorted(obj)
        return super().default(obj)

print(json.dumps({'ts': datetime(2026, 10, 9, tzinfo=timezone.utc)}, cls=AppEncoder))
# {"ts": "2026-10-09T00:00:00+00:00"}
```

##### Pickle

```python
import pickle

print(pickle.DEFAULT_PROTOCOL)              # 5 (since 3.14)

class Outer:
    class __Hidden:                          # private (name-mangled) nested class
        pass

    def make(self) -> object:
        return self.__Hidden()

    def __private(self) -> str:
        return 'secret'

# New in 3.15: PRIVATE nested classes and private methods pickle by reference
# (3.14: PicklingError / AttributeError). Plain nested classes always worked.
print(type(pickle.loads(pickle.dumps(Outer().make()))).__name__)   # __Hidden
method = pickle.loads(pickle.dumps(Outer()._Outer__private))
print(method())                              # secret
```

**SECURITY WARNING:** never unpickle untrusted data — it executes arbitrary code. The same applies to `shelve` (3.15 adds custom `serializer=`/`deserializer=` hooks to `shelve`, which lets you store JSON instead of pickles).

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
print(list(csv.DictReader(buf)))     # [{'name': 'Alice', 'age': '30'}]
```

Always open CSV files with `newline=''` and an explicit `encoding=`. `csv.Sniffer.sniff()` is up to 1.6× faster in 3.15; `csv.__version__` is deprecated.

***

#### Regular Expressions

##### New in 3.15: `re.prefixmatch()` (and soft-deprecated `re.match()`)

`re.prefixmatch()` / `Pattern.prefixmatch()` are new, explicit names for what `match()` always did: match **at the start** of the string. `re.match()` is now **soft deprecated** — no warning, no removal; new 3.15-only code should prefer `prefixmatch()`, code supporting older versions keeps `match()`.[^31]

```python
import re

print(re.prefixmatch(r'\d+', '123abc'))   # <re.Match object; span=(0, 3), match='123'>
print(re.fullmatch(r'\d+', '123abc'))     # None — the WHOLE string must match
print(re.search(r'abc', '123abc'))        # <re.Match object; span=(3, 6), match='abc'>

DATE_RE = re.compile(r'(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})')
if m := DATE_RE.prefixmatch('2026-10-09 release'):
    print(m['year'], m['month'])          # 2026 10

# Validation must anchor the end: prefixmatch/match alone accept trailing garbage
print(bool(re.prefixmatch(r'[a-z]+', 'abc; DROP TABLE')))   # True  ❌ as a validator
print(bool(re.fullmatch(r'[a-z]+', 'abc; DROP TABLE')))     # False ✅
```

The undocumented `sre_compile`, `sre_parse` and `sre_constants` modules were **removed** in 3.15 (`ModuleNotFoundError`).

***

#### Dates and Times

```python
from datetime import UTC, datetime, timedelta
from zoneinfo import ZoneInfo
import warnings

now = datetime.now(UTC)                        # ✅ aware UTC (datetime.UTC since 3.11)
local = now.astimezone(ZoneInfo('Europe/Berlin'))
print(local.tzinfo)                            # Europe/Berlin

print(datetime.strptime('2024-02-29', '%Y-%m-%d').day)   # 29

# 3.15: a day-of-month directive without a year is an ERROR (leap-day ambiguity)
try:
    datetime.strptime('02 29', '%m %d')
except ValueError as e:
    print(str(e)[:60])     # Day of month directive '%d' may not be used without a year d
with warnings.catch_warnings(record=True) as w:
    warnings.simplefilter('always')
    datetime.strptime('3 2', '%m %e')          # %e without a year: deprecated, error in 3.17
print(w[0].category.__name__)                  # DeprecationWarning
```

`datetime.utcnow()`/`utcfromtimestamp()` remain deprecated — use `datetime.now(UTC)` / `datetime.fromtimestamp(ts, tz=UTC)`. To parse month/day strings, append a leap year explicitly: `datetime.strptime(f'2000 {s}', '%Y %m %d')`.

***

#### Memory Management

```python
import gc
import sys
import tracemalloc
import weakref

x = [1, 2, 3]
print(sys.getrefcount(x) >= 2)          # True — exact counts are an implementation detail

tracemalloc.start()
blob = [bytes(1000) for _ in range(100)]
current, peak = tracemalloc.get_traced_memory()
print(peak >= 100_000)                  # True
tracemalloc.stop()

class Node:
    pass

cache: weakref.WeakValueDictionary[str, Node] = weakref.WeakValueDictionary()
n = Node()
cache['a'] = n
del n
gc.collect()
print('a' in cache)                     # False — weak refs don't keep objects alive
```

- 3.15's default build uses the **generational GC** (3.14.0–3.14.4's incremental GC was reverted for memory-pressure regressions) — memory tuning advice for 3.13 applies again.
- Free-threaded builds use more memory: immortal interned strings, larger non-GC object headers, delayed freeing (QSBR, biased/deferred reference counting). `gc.collect()` releases memory held for delayed freeing.[^19]
- `mmap.mmap.set_name()` (3.15, Linux ≥ 5.17) labels anonymous mappings in `/proc/<pid>/maps`.
- `bytearray.take_bytes()` avoids one full copy when turning a buffer into `bytes`.

***

#### Testing

```python
import logging
import unittest
import warnings
from unittest.mock import AsyncMock, patch

class TestExamples(unittest.TestCase):
    def test_assert_warns_no_longer_swallows(self) -> None:
        # 3.15: warnings that DON'T match the expected category are re-emitted, not swallowed
        with warnings.catch_warnings(record=True) as outer:
            warnings.simplefilter('always')
            with self.assertWarns(DeprecationWarning):
                warnings.warn('other', UserWarning)
                warnings.warn('dep', DeprecationWarning)
        messages = [str(w.message) for w in outer]
        self.assertIn('other', messages)      # re-emitted to the outer handler (3.14: swallowed)
        self.assertNotIn('dep', messages)     # the matching warning is still captured

    def test_assert_logs_formatter(self) -> None:
        # 3.15: assertLogs(formatter=...)
        fmt = logging.Formatter('%(levelname)s|%(message)s')
        with self.assertLogs('app', level='INFO', formatter=fmt) as cm:
            logging.getLogger('app').info('hello')
        self.assertEqual(cm.output, ['INFO|hello'])

class TestAsync(unittest.IsolatedAsyncioTestCase):
    async def test_fetch(self) -> None:
        fetch = AsyncMock(return_value={'id': 1})
        self.assertEqual((await fetch(1))['id'], 1)
        fetch.assert_awaited_once_with(1)

if __name__ == '__main__':
    unittest.main(argv=['tests'], exit=False, verbosity=0)
```

3.15-specific testing notes:

- **`assertWarns()`/`assertWarnsRegex()` no longer swallow non-matching warnings** and support nesting — suites may suddenly show (or, under `-W error`, fail on) warnings that were previously hidden. Silence with filters or add explicit `assertWarns`.
- **Run tests with `-W error::DeprecationWarning`** to catch the 3.16/3.17 removals early (asyncio policies, `asyncio.iscoroutinefunction`, `~bool`, `ByteString`, `profile`).
- **Lazy imports:** add a smoke test that imports every module *and touches every lazily imported name* (or run the suite once with `-X lazy_imports=normal` and once with `all`). `mock.patch(..., autospec=True)` on lazily imported targets needs 3.15.1.
- `IsolatedAsyncioTestCase` test methods must return `None` (returning a value is deprecated).
- `python -m unittest`, `doctest`, `timeit`, `pdb` and other CLIs are colorized; set `NO_COLOR=1` (or `PYTHON_COLORS=0`) when tests parse CLI output.

***

#### Virtual Environments and Packaging

```bash
python3.15 -m venv .venv
source .venv/bin/activate
python -m pip install --upgrade pip      # 3.15.1 bundles pip 26.2.2 — upgrade old venvs!

# uv — fast installer/resolver that can also provision interpreters
uv venv --python 3.15
uv pip install -r requirements.txt
uv python install 3.15t                  # free-threaded build, side by side
```

3.15 venv change: on POSIX platforms whose `sys.platlibdir` is not `lib` (e.g. `lib64`), `venv` now creates a real platlib directory instead of a `lib64 -> lib` symlink, so purelib and platlib no longer share a directory.[^1]

##### `pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=77"]
build-backend = "setuptools.build_meta"

[project]
name = "myapp"
version = "1.0.0"
requires-python = ">=3.15"   # lazy imports, frozendict, sentinel are SyntaxError/NameError on <=3.14
dependencies = []

[tool.mypy]
python_version = "3.15"

[tool.ruff]
target-version = "py315"     # only if your Ruff version knows py315 — check `ruff --help`

[tool.pytest.ini_options]
testpaths = ["tests"]
filterwarnings = ["error::DeprecationWarning"]
```

If the project must support older interpreters, set `requires-python` accordingly and do **not** use 3.15-only syntax (`lazy`, `[*x for x in y]`, `case +1:`) or builtins (`frozendict`, `sentinel`) without guards; `__lazy_modules__` is the version-portable way to get lazy imports.

##### C Extensions on 3.15

- Free-threaded wheels: target `abi3t` (one wheel for free-threaded 3.15+) when the extension can use PEP 697/793/820 APIs; otherwise build `cp315t`.[^21]
- Frame pointers: keep `-fno-omit-frame-pointer -mno-omit-leaf-frame-pointer` in custom native builds (Rust/C++ included), or native stack unwinding breaks for the whole process.[^28]
- `PyGILState_Ensure()/Release()` are soft deprecated in favour of the PEP 788 thread-state/interpreter-guard APIs; several long-deprecated C APIs (`PyWeakref_GetObject`, `Py_GetPath`, `Py_GetPrefix`, `PyImport_ImportModuleNoBlock`, ...) were removed — `pythoncapi-compat` provides replacements.[^1]

##### Project Layout

```
project/
├── src/
│   └── myapp/
│       ├── __init__.py
│       └── cli.py
├── tests/
│   └── test_cli.py
└── pyproject.toml
```

***

#### Writing Code That Also Runs on 3.14 and Earlier

Libraries and services that support several Python versions must not use 3.15-only **syntax** at all (the module fails to compile on old interpreters), and must guard 3.15-only **names**:

```python
import sys

# Lazy imports without the `lazy` keyword: ignored on <=3.14, honored on 3.15+
__lazy_modules__ = ['json']
import json

# frozendict / sentinel fallbacks
try:
    FrozenDict = frozendict                     # 3.15+
except NameError:                               # <=3.14
    from types import MappingProxyType as FrozenDict   # read-only view, NOT hashable

try:
    MISSING = sentinel('MISSING')               # 3.15+
except NameError:
    MISSING = object()                          # identity-only fallback

# Feature gates: compare version tuples, never strings
if sys.version_info >= (3, 15):
    from math.integer import isqrt
else:
    from math import isqrt

print(isqrt(17), FrozenDict({'a': 1})['a'], MISSING is MISSING)   # 4 1 True
```

| 3.15-only construct | On ≤3.14 | Portable alternative |
|---|---|---|
| `lazy import x` | `SyntaxError` (whole module) | `__lazy_modules__ = ['x']` |
| `[*xs for xs in seqs]`, `{**d for d in ds}` | `SyntaxError` | nested comprehension / `itertools.chain.from_iterable` |
| `case +1:` | `SyntaxError` | `case 1:` |
| `frozendict`, `sentinel` | `NameError` | guarded fallback (above) |
| `math.integer` | `ModuleNotFoundError` | `math.gcd`, `math.isqrt`, ... |
| `re.prefixmatch()` | `AttributeError` | `re.match()` |
| `TypedDict(closed=True)`, `TypeForm`, `disjoint_base` | `TypeError` / `ImportError` | `typing_extensions` |
| `@contextmanager` decorating a generator/coroutine | runs the body *outside* the context | `with` inside the function body |
| default `open()` encoding | locale encoding (cp1252 on many Windows hosts) | explicit `encoding='utf-8'` |

`sys.version_info >= (3, 15)` comparisons are also understood by type checkers, which then skip the branch that doesn't apply to the configured target version.

***

#### Code Quality Tools

```bash
pip install mypy ruff bandit pytest

ruff check src/ tests/        # lint (replaces flake8 + isort + pyupgrade)
ruff format src/ tests/       # format
mypy --strict src/            # type check
bandit -r src/ -q             # security lint
python3.15 -W error::DeprecationWarning -X warn_default_encoding -m pytest
```

**Tooling caveat for 3.15 syntax:** linters, formatters and type checkers must understand `lazy import`, comprehension unpacking and PEP 728/747 typing before you adopt them — older tool versions report syntax errors or wrong diagnostics. Verify the tool version supports Python 3.15 before enabling those features; until then, `__lazy_modules__` keeps the source parseable by older tools.

***

#### Removed and Changed in 3.15 — Migration Notes

APIs **removed** in 3.15 — code using them fails on 3.15:[^1]

| Removed in 3.15 | Replacement |
|---|---|
| `http.server.CGIHTTPRequestHandler`, `python -m http.server --cgi` | a real WSGI/ASGI server or framework |
| `typing.no_type_check_decorator` | none (decorate with `@typing.no_type_check` directly) |
| `NamedTuple('P', x=int)` keyword syntax | class syntax or `NamedTuple('P', [('x', int)])` |
| `TypedDict('TD')` / `TypedDict('TD', None)` | `TypedDict('TD', {})` or `class TD(TypedDict): pass` |
| `ctypes.SetPointerType()` | — (undocumented; deprecated since 3.13) |
| `glob.glob0()` / `glob.glob1()` | `glob.glob(pattern, root_dir=...)` |
| `importlib.abc.Loader.load_module()` (and the import system calling it) | `exec_module()` / `create_module()` |
| `zipimport.zipimporter.load_module()` | `exec_module()` |
| `importlib.resources.files(package=...)` keyword | `files('pkg')` positional anchor |
| `pathlib.PurePath.is_reserved()` | `os.path.isreserved()` |
| `platform.java_ver()` | — |
| `sre_compile`, `sre_constants`, `sre_parse` modules | `re` |
| `sysconfig.is_python_build(check_home=...)` | drop the argument |
| `threading.RLock(*args, **kwargs)` arguments | `threading.RLock()` |
| `codeobject.co_lnotab` | `codeobject.co_lines()` |
| `wave` `getmark()`/`setmark()`/`getmarkers()` | — |
| AST node constructors with missing required args / unknown keywords | pass all required fields (now `TypeError`, was `DeprecationWarning`) |
| `datetime.strptime()` with `%d` and no year | include a year directive (now `ValueError`) |
| `ByteString` in `typing.__all__` / `collections.abc.__all__` | `collections.abc.Buffer` (the names themselves go in 3.17) |
| `module.__cached__` (no longer set or used) | `module.__spec__.cached` |

**Not removed (despite older deprecation notices):** `locale.getdefaultlocale()` was scheduled for removal in 3.15 but was **un-deprecated** instead — it no longer warns. (`locale.getlocale()`/`getencoding()` are still the better tools.)

**Behavior changes to know:**

- **UTF-8 is the default encoding** for files, pipes and the filesystem (PEP 686); `PYTHONUTF8=0`/`-X utf8=0` restores locale encodings.
- **`argparse`:** `suggest_on_error` now defaults to `True`; with `add_argument('-f', '-foo')` the `dest` is now `'foo'` (was `'f'`) — pass `dest=` explicitly to keep old names.
- **`base64.urlsafe_b64decode()`** no longer requires padding.
- **`sqlite3.connect()`**: all parameters except `database` are keyword-only.
- **`contextlib`** decorators keep the context open across generator iteration/`await`.
- **`unittest.assertWarns*`** no longer swallow non-matching warnings.
- **`importlib.metadata`** raises `MetadataNotFound` for a metadata directory without a metadata file (it used to return an empty object).
- **`array.typecodes`** is now a `tuple` (two-letter codes `'Zf'`, `'Zd'`); `ctypes` complex types' `_type_` changed to `'Zf'`/`'Zd'`/`'Zg'`.
- **`resource.RLIM_INFINITY`** is always positive; passing the old negative value is deprecated.
- **`xml.etree.ElementTree.iterparse()`** emits `ResourceWarning` if left unclosed after opening a file — close it or wrap in `contextlib.closing()`.
- **`email`** generators raise instead of producing a corrupt message when an address header contains a non-ASCII mailbox (see `EmailPolicy.utf8`).
- **Import locks** are taken parent-before-child, fixing a long-standing deadlock between threads importing `pkg.sub` and `pkg.sub.mod`.
- **`mmap.mmap.resize()`** no longer exists on platforms that cannot support it (was `SystemError`).

```python
import argparse
import warnings

parser = argparse.ArgumentParser()
action = parser.add_argument('-f', '-foo')
print(action.dest)                   # foo (3.14: f)
print(parser.suggest_on_error)       # True

import array
print(type(array.typecodes).__name__)   # tuple

import locale
with warnings.catch_warnings():
    warnings.simplefilter('error')
    locale.getdefaultlocale()        # no DeprecationWarning in 3.15 (3.14.0–3.14.6 warned; the un-deprecation was backported in 3.14.7)
```

**Newly deprecated in 3.15** (clean up proactively): the `profile` module (removal 3.17); `%e` without a year in `strptime` (3.17); `-b`/`-bb` (become no-ops in 3.17); `hashlib` `string=` keyword (3.19); `http.cookies` `js_output()` (3.19); altering `imaplib.IMAP4.file` (3.19); `__version__`/`version`/`VERSION` attributes in `argparse`, `csv`, `ctypes`, `ctypes.macholib`, `decimal` (use `decimal.SPEC_VERSION`), `http.server`, `imaplib`, `ipaddress`, `json`, `logging` (also `__date__`), `optparse`, `pickle`, `platform`, `re`, `socketserver`, `tabnanny`, `tarfile`, `tkinter.font`, `tkinter.ttk`, `wsgiref.simple_server`, `xml.etree.ElementTree`, `xml.sax.expatreader`, `xml.sax.handler`, `zlib` (3.20 — use `sys.version_info`); instantiating abstract AST nodes (3.20); `isinstance()` against non-decorated subclasses of runtime-checkable protocols (3.20); `struct.Struct` re-initialization (3.20); `os.path.commonprefix()` (now warns); `webbrowser.MacOSXOSAScript` (3.17); soft deprecations: `re.match()`, `math.gcd`/`comb`/... aliases, `struct` `'F'`/`'D'` codes.

##### Pending Removal in 3.16 and 3.17

| Removed in 3.16 | Use instead |
|---|---|
| asyncio policy system (`get/set_event_loop_policy`, `*EventLoopPolicy` classes) | `asyncio.run(loop_factory=...)`, `asyncio.Runner` |
| `asyncio.iscoroutinefunction()` | `inspect.iscoroutinefunction()` |
| `~True` / `~False` (bitwise inversion on `bool`) | `not x` (or `~int(x)`) |
| `array.array('u', ...)` | `'w'` type code |
| logging handlers taking `strm=` | `stream=` |
| undotted extensions in `mimetypes.MimeTypes.add_type()` | `'.ext'` (will raise `ValueError`) |
| `shutil.ExecError` | `RuntimeError` |
| `symtable.Class.get_methods()` | — |
| `sys._enablelegacywindowsfsencoding()` | `PYTHONLEGACYWINDOWSFSENCODING` |
| `sysconfig.expand_makefile_vars()` | `sysconfig.get_paths(vars=...)` |
| `tarfile.TarFile.tarfile` attribute | — |
| setting `module.__loader__` without `__spec__.loader` (import system stops using `__loader__`) | set `__spec__.loader` |
| Python `functools.reduce()` with `function`/`sequence` as keywords | positional arguments |

| Removed in 3.17 | Use instead |
|---|---|
| `typing.ByteString`, `collections.abc.ByteString` | `collections.abc.Buffer` / `bytes \| bytearray \| memoryview` |
| `profile` module | `profiling.tracing` (or `cProfile`) |
| `strptime()` with `%e` and no year | include a year |
| `encodings.normalize_encoding()` with non-ASCII names | ASCII names |
| `tkinter.Variable.trace_variable()/trace()/trace_vdelete()/trace_vinfo()` | `trace_add()/trace_remove()/trace_info()` |
| `webbrowser.MacOSXOSAScript` | `webbrowser.MacOS` |
| private `typing._UnionGenericAlias` | `typing.get_origin()` / `get_args()` |
| `-b`/`-bb` `BytesWarning` (become no-ops) | type checker |

```python
import warnings

with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter('always')
    import asyncio
    asyncio.iscoroutinefunction(lambda: None)
    from collections.abc import ByteString
for w in caught:
    print(w.category.__name__, w.message)
# DeprecationWarning 'asyncio.iscoroutinefunction' is deprecated and slated for removal in Python 3.16; use inspect.iscoroutinefunction() instead
# DeprecationWarning 'collections.abc.ByteString' is deprecated and slated for removal in Python 3.17
```

***

#### Production Checklist

##### Python 3.15-Specific Items

- [ ] Running **3.15.1 or later** (expedited security release: pip/certifi emSign fix, libexpat 2.9.0; several lazy-import fixes) — and pip/uv/certifi upgraded inside every existing venv
- [ ] `lazy import` used only for side-effect-free modules; side-effect imports (plugin registration, monkey-patching, logging config) stay eager
- [ ] No `lazy` inside functions/classes/`try` blocks (SyntaxError); optional dependencies keep the eager `try: import x / except ImportError` pattern
- [ ] Code that scans `globals()`/`vars(module)` handles `types.LazyImportType` proxies (or uses `getattr`)
- [ ] First use of every lazily imported name is covered by a startup/smoke test (import errors now surface at first use)
- [ ] `-X lazy_imports=all` / `PYTHON_LAZY_IMPORTS=all` not enabled in production without a full test pass; no references to a nonexistent `none` mode
- [ ] Modules defining `__getattr__` alongside lazy imports run on ≥3.15.1 (gh-158824)
- [ ] Libraries supporting ≤3.14 use `__lazy_modules__`, not the `lazy` keyword
- [ ] `isinstance(x, dict)` gates in public APIs reviewed — accept `Mapping` or `(dict, frozendict)`
- [ ] `frozendict` used for hashable constants/cache keys/immutable defaults; values inside are immutable when hashing is required
- [ ] `object()` sentinels in 3.15-only code replaced with `sentinel('NAME')` assigned to a matching module/class-level name; compared with `is` (sentinels are truthy)
- [ ] Files that are genuinely locale-encoded are opened with `encoding='locale'` (UTF-8 is the default now); explicit `encoding='utf-8'` kept in multi-version code
- [ ] `math.integer.*` used for integer functions in new code (`math.gcd` & co. are soft-deprecated aliases)
- [ ] `TypedDict` payload types use `closed=True`/`extra_items=` where appropriate — with runtime validation still done separately
- [ ] Every protocol used in `isinstance()`/`issubclass()` is itself decorated `@runtime_checkable` (non-decorated subclasses warn; `TypeError` in 3.20)
- [ ] `@contextmanager` objects used as decorators on generators/coroutines are only relied on for 3.15+ (broken on ≤3.14)
- [ ] `argparse` options with a short flag plus single-dash long flag have explicit `dest=` where the old name mattered
- [ ] `sqlite3.connect()` called with keyword arguments only (beyond `database`)
- [ ] `datetime.strptime()` formats with `%d`/`%e` include a year
- [ ] Removed APIs gone: `CGIHTTPRequestHandler`, `typing.no_type_check_decorator`, `sre_*`, `PurePath.is_reserved`, `glob.glob0/1`, `Loader.load_module`, `RLock(args)`, `co_lnotab`, keyword `NamedTuple`, zero-field `TypedDict('TD')`
- [ ] 3.16 removals cleared: asyncio policies, `asyncio.iscoroutinefunction`, `~bool`, `array('u')`, logging `strm=`; tests run with `-W error::DeprecationWarning`
- [ ] `import profile` replaced by `profiling.tracing`; Tachyon (`python -m profiling.sampling`) available to ops with the needed ptrace capability
- [ ] Shared iterators/generators across threads wrapped with `threading.serialize_iterator` / `synchronized_iterator`
- [ ] Free-threaded deployment (if any) uses `python3.15t`, checks `sys._is_gil_enabled()` after imports, and all C extensions ship `cp315t`/`abi3t` wheels
- [ ] Custom native builds keep frame-pointer flags (PEP 831)
- [ ] `.pth` startup `import` hooks in packages you publish migrated to `.start` files (PEP 829)
- [ ] No reliance on the 3.14.0–3.14.4 incremental GC behavior or JIT being on by default

##### Code Correctness

- [ ] `bytes` and `str` never mixed without explicit encode/decode
- [ ] No mutable default arguments; dataclass mutable fields use `default_factory`
- [ ] `__hash__` only on immutable classes (or `@dataclass(frozen=True)`)
- [ ] No `return`/`break`/`continue` in `finally` (SyntaxWarning since 3.14)
- [ ] No dict/list mutation during iteration
- [ ] `X | None` instead of `Optional[X]`; ABCs from `collections.abc`

##### Safety and Security

- [ ] SQL parameterized; `subprocess` with argument lists, never `shell=True` with user input
- [ ] `pickle`/`shelve` never loaded from untrusted sources
- [ ] Tar archives extracted with the default `'data'` filter (never `'fully_trusted'` for untrusted input)
- [ ] Path checks use `resolve()` + `is_relative_to()` or `os.path.commonpath()` — never `commonprefix()`
- [ ] Validation regexes use `re.fullmatch()`, not `match()`/`prefixmatch()`
- [ ] `secrets` for tokens, `secrets.compare_digest()` for comparisons
- [ ] Untrusted XML parsed with `defusedxml` or Expat amplification limits

##### Performance

- [ ] Profiled before optimizing (Tachyon for production, `profiling.tracing` for exact counts)
- [ ] Startup measured with `-X importtime`; heavy imports made `lazy`
- [ ] JIT (`PYTHON_JIT=1`) benchmarked on the real workload before enabling
- [ ] Sets/frozensets for membership; `str.join()` for string building; `%`-style logging

##### Observability

- [ ] `logging.getLogger(__name__)` per module; no `print()` in production paths
- [ ] `logger.exception()` inside `except`; bare `raise` to re-raise
- [ ] Lazy-import resolution failures (chained `ImportError` "lazy import of ... raised an exception during resolution") are logged where first use happens
- [ ] Frame pointers kept in native builds so `perf`/eBPF profiles show full stacks
