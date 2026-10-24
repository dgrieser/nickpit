### Citations: Python 3.6: The Complete Production Developer Guide

Sources for `python-3.6.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Python 3.6 What's New. Python Software Foundation. https://docs.python.org/3/whatsnew/3.6.html
    - Overview — paragraph 1: "Python 3.6 was released on December 23, 2016."
    - Primitive Types and Numerics › Integers — paragraph 2: "**Numeric literals with underscores (PEP 515 — new in 3.6):**"
    - Primitive Types and Numerics › Integers — paragraph 3: "…are allowed; leading, trailing, or consecutive underscores are syntax errors."
    - Strings: `str` and `bytes` › f-strings — Formatted String Literals (PEP 498, new in 3.6) — paragraph 1: "…or `F` and allow embedding expressions directly inside string literals."
    - Variable Annotations (PEP 526, new in 3.6) — paragraph 1: "…variables — including module-level variables, class variables, and instance variables."
    - Generators and Iterators › Asynchronous Generators (PEP 525, new in 3.6) — paragraph 1: "…the restriction that prevented using `yield` inside `async def` functions:"
    - Classes and Object-Oriented Programming › `__init_subclass__` (PEP 487, new in 3.6) — paragraph 1: "A cleaner alternative to metaclasses for customizing subclass creation:"
    - Classes and Object-Oriented Programming › `__set_name__` Descriptor Enhancement (PEP 487, new in 3.6) — paragraph 1: "Descriptors now receive their attribute name at class creation time:"
    - Exception Handling › Exception Hierarchy — bullet 3: "…of `ImportError`) is raised when a module is not found."
    - `async`/`await` and `asyncio` — paragraph 1: "`asyncio` became stable (no longer provisional) in Python 3.6."
    - The `secrets` Module (new in Python 3.6) — paragraph 1: "…`secrets` for cryptographically strong random values suitable for security-sensitive purposes:"
    - The `secrets` Module (new in Python 3.6) — paragraph 2: "…unpredictable: session tokens, CSRF tokens, API keys, password reset links."
[^2]: Python 3 Data Model — Classes. https://docs.python.org/3.6/reference/datamodel.html
    - PEP 8 Code Style › Naming Conventions — paragraph 1: "`class Foo:` is equivalent and preferred."
    - Classes and Object-Oriented Programming › Class Definition — paragraph 1: "`class Foo:` and `class Foo(object):` are identical."
[^3]: Python 3 Built-in Types. https://docs.python.org/3.6/library/stdtypes.html
    - Primitive Types and Numerics › Integers — paragraph 1: "It is **arbitrary precision** — there is no separate `long`."
    - Primitive Types and Numerics › Floats — paragraph 1: "Floats are C `double` (64-bit IEEE 754)."
    - Primitive Types and Numerics › Booleans — paragraph 1: "`True == 1` and `False == 0`."
    - Primitive Types and Numerics › Booleans — paragraph 2: "**Falsy values** in Python 3.6:"
    - Division — paragraph 1: "…is one of the most important changes from Python 2."
    - Strings: `str` and `bytes` › String Methods — paragraph 1: "All of the following work on `str`:"
    - Strings: `str` and `bytes` › String Methods — paragraph 2: "Prefer `casefold()` for case-insensitive comparisons of user-supplied text."
    - Lists — paragraph 1: "Lists are mutable, ordered, heterogeneous sequences."
    - Tuples — paragraph 1: "Tuples are immutable, ordered sequences."
    - Dictionaries — paragraph 2: "…2's `d.iteritems()` because the default methods ARE already lazy views."
    - Sets — paragraph 1: "Sets are mutable, unordered collections of unique, hashable elements."
    - `range` in Python 3 — paragraph 1: "…a lazy object — there is no `xrange()` at all."
    - Functions › `nonlocal` Keyword — paragraph 1: "…rebind a variable in the enclosing scope (Python 3 only):"
    - Functions › `map`, `filter`, `zip`, `reduce` — paragraph 1: "…and `zip()` all return lazy iterators — no list copy."
    - Generators and Iterators › The Iterator Protocol — paragraph 1: "…Python 3 uses `__next__()` (NOT `next()` as in Python 2):"
    - Classes and Object-Oriented Programming › Inheritance and `super()` — paragraph 1: "…zero-argument `super()` — no longer need to pass `ClassName, self`:"
    - Exception Handling › Exception Hierarchy — bullet 1: "`IOError` is a backwards-compatibility alias."
    - Exception Handling › Exception Hierarchy — bullet 2: "…subclasses (`FileNotFoundError`, `PermissionError`, `TimeoutError`) allow fine-grained catching without checking `errno`."
[^4]: Python 3 io module. https://docs.python.org/3.6/library/io.html
    - Strings: `str` and `bytes` — paragraph 1: "Python 3.6 has two distinct string-like types:"
    - Strings: `str` and `bytes` › Encoding and Decoding — paragraph 1: "`str` and `bytes` are never equal**, even empty:"
    - File I/O › Text Files — paragraph 1: "Always pass `encoding` explicitly:"
[^5]: PEP 498 – Literal String Interpolation. https://peps.python.org/pep-0498/
    - Strings: `str` and `bytes` › f-strings — Formatted String Literals (PEP 498, new in 3.6) — paragraph 4: "…`str.format()` because they are compiled directly to bytecode string-building instructions."
[^6]: typing.NamedTuple. https://docs.python.org/3.6/library/typing.html#typing.NamedTuple
    - Type Hints (PEP 484 + PEP 526) › `typing.NamedTuple` — Class-Based Syntax (Python 3.6+) — paragraph 1: "…a class-based syntax for `NamedTuple` with full type annotation support:"
[^7]: Python 3 Expressions — Comprehensions. https://docs.python.org/3/reference/expressions.html
    - Lists › List Comprehensions — paragraph 1: "…list comprehension variables do NOT leak into the enclosing scope."
[^8]: CPython dict implementation — compact representation. Python 3.6 What's New.
    - Dictionaries — paragraph 1: "…a CPython implementation detail (the language guarantee came with 3.7)."
[^9]: Python Data Model — `__slots__`. https://docs.python.org/3/reference/datamodel.html
    - Classes and Object-Oriented Programming › `__slots__` — paragraph 1: "…`__dict__`, reducing memory significantly for large numbers of small objects:"
[^10]: Python enum module. https://docs.python.org/3.6/library/enum.html
    - Enum (Python 3.4+) — paragraph 1: "`enum.auto()` was enhanced in Python 3.6:"
[^11]: Python Security Best Practices. OWASP Python Security Project.
    - Exception Handling › Exception Hierarchy — paragraph 2: "Never catch `BaseException` or bare `except:` in production:**"
    - File I/O › Temporary Files — paragraph 1: "…use `tempfile.mktemp()`** — deprecated and vulnerable to TOCTOU race conditions."
[^12]: PEP 3134 – Exception Chaining and Embedded Tracebacks. https://peps.python.org/pep-3134/
    - Exception Handling › Exception Chaining (Python 3) — paragraph 1: "Python 3 supports explicit exception chaining via `raise ... from`:"
[^13]: pathlib module. https://docs.python.org/3.6/library/pathlib.html
    - File I/O › `pathlib` — Object-Oriented Paths (Python 3.4+, `os.PathLike` support new in 3.6) — paragraph 1: "…and work directly with `open()` and all standard library functions:"
[^14]: Python logging HOWTO. https://docs.python.org/3.6/howto/logging.html
    - `async`/`await` and `asyncio` › Mental Model — bullet 1: "`asyncio` runs on a **single thread** with cooperative scheduling."
    - Logging — paragraph 1: "**Critical rules:**"
[^15]: Python GIL. https://docs.python.org/3/glossary.html#term-global-interpreter-lock
    - Concurrency › The GIL — paragraph 1: "…means only one thread runs Python bytecode at a time:"
