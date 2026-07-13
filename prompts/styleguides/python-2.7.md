### Python 2.7 Style Guide

Python conventions following PEP 8, adapted to what is available on Python 2.7
(EOL since 2020-01-01 — for maintenance of legacy code only). Modern-Python
constructs that do not exist here: f-strings, inline type annotations,
`dataclasses`, `pathlib`, `venv`, `async`/`await`, keyword-only arguments,
`raise ... from`. This guide lists the 2.7-correct replacement for each.

#### PEP 8 Fundamentals

##### Naming Conventions

```python
# Variables and functions: snake_case
user_name = "John"
def calculate_total(items):
    pass

# Constants: SCREAMING_SNAKE_CASE
MAX_CONNECTIONS = 100
DEFAULT_TIMEOUT = 30

# Classes: PascalCase. ALWAYS inherit from object (new-style classes) --
# Python 2 still has old-style classes and they break super(), descriptors,
# __slots__, and isinstance checks in subtle ways.
class UserAccount(object):
    pass

# Private: single underscore prefix
class User(object):
    def __init__(self):
        self._internal_state = {}

# Name mangling: double underscore prefix
# Avoids subclass attribute collisions -- not true privacy
# (still reachable as _Base__private)
class Base(object):
    def __init__(self):
        self.__private = "name-mangled"

# Module-level "private": single underscore
_module_cache = {}
```

##### Indentation and Line Length

```python
# 4 spaces per indentation level -- never tabs, never mixed
# (python -tt turns mixed indentation into an error)
def function():
    if condition:
        do_something()

# Line length: 79 characters (PEP 8). Black and Ruff do not support
# Python 2 source, so there is no 88-character formatter convention here.
# Break long lines appropriately
result = some_function(
    argument_one,
    argument_two,
    argument_three,
)

# Implicit line continuation in brackets
users = [
    "alice",
    "bob",
    "charlie",
]
```

##### Module Preamble and Future Imports

```python
# -*- coding: utf-8 -*-
# Encoding declaration (PEP 263) is required whenever the file contains
# non-ASCII characters; the default source encoding in Python 2 is ASCII.

# Start every module with the __future__ imports to get Python 3 semantics.
# They must be the first statement after the docstring.
from __future__ import absolute_import, division, print_function, unicode_literals

# absolute_import:  "import foo" is absolute, use "from . import foo" for
#                   relative imports (implicit relative imports are a bug farm)
# division:         / is true division, // is floor division
# print_function:   print("x") instead of the print statement
# unicode_literals: "abc" is unicode by default, like Python 3
```

##### Imports

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

# Avoid wildcard imports
# Bad: from module import *
# Good: from module import specific_item

# Guard Python-3-only stdlib names behind backports
try:
    from unittest import mock  # only on Python 3
except ImportError:
    import mock  # pip install mock
```

#### Strings and Unicode

The `str`/`unicode` split is the single largest source of Python 2 bugs.
Decode bytes at the boundary, work in unicode internally, encode on output.

```python
# -*- coding: utf-8 -*-
from __future__ import unicode_literals
import io

# With unicode_literals, plain literals are unicode; mark bytes explicitly
name = "Jürgen"          # unicode
payload = b"\x89PNG"     # bytes (str in Python 2)

# Decode at input boundaries, encode at output boundaries
with io.open("names.txt", "r", encoding="utf-8") as handle:  # io.open, not open
    lines = handle.read().splitlines()  # unicode lines

with io.open("out.txt", "w", encoding="utf-8") as handle:
    handle.write("Grüße\n")

# Never mix bytes and unicode -- implicit ASCII coercion raises
# UnicodeDecodeError at runtime on the first non-ASCII byte
# Bad:  b"prefix-" + name
# Good: b"prefix-" + name.encode("utf-8")

# Type checks that accept both string types
import six
isinstance(value, six.string_types)   # str or unicode
isinstance(value, six.text_type)      # unicode only
isinstance(value, six.binary_type)    # bytes only

# No f-strings in Python 2 -- use str.format() (preferred) or % formatting
greeting = "Hello, {}!".format(name)
point = "({x}, {y})".format(x=1, y=2)
logger.info("fetched %s in %.2fs", url, elapsed)  # lazy %-style for logging
```

#### Type Comments

Python 2.7 has no annotation syntax. PEP 484 defines an equivalent
comment-based form that mypy and IDEs understand. Install the backport:
`pip install typing` (pip on 2.7 resolves a compatible release).

##### Basic Type Comments

```python
from typing import Dict, List, Optional, Text

# Variables
name = "John"        # type: Text
age = 30             # type: int
scores = [90, 85]    # type: List[int]

# Functions: the comment goes on the line after the header, before the
# docstring. Use Text for unicode-or-str parameters; checkers treat int
# and long as equivalent on Python 2.
def greet(name):
    # type: (Text) -> Text
    return "Hello, {}!".format(name)

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
from typing import Any, AnyStr, Callable, Generic, Iterable, Optional, TypeVar

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

#### Docstrings

Because there are no inline annotations, docstrings carry the types too —
state them in the `Args`/`Returns` sections even when a type comment exists.

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
        raise ValueError("Discount must be between 0 and 100")

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

#### Virtual Environments

##### Setup Commands

There is no `python -m venv` on Python 2 — use `virtualenv`. Modern
`virtualenv` releases dropped the ability to create 2.7 environments in
20.22.0, and pip 20.3.4 is the last pip release that runs on Python 2.7.

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

# Install dependencies
pip install -r requirements.txt

# Freeze dependencies
pip freeze > requirements.txt
```

##### Dependency Pins

Package ecosystems have moved on; unpinned installs pull Python-3-only
releases and fail at install or import time. Pin the ceilings explicitly.

```text
# requirements.txt -- last releases that support Python 2.7
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
│   ├── __init__.py      # Required -- no implicit namespace packages in 2.7
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

#### Testing

##### pytest Basics

Use the pytest 4.6 series (`pytest==4.6.11`) — it is the last line that
runs on Python 2.7, and it already supports plain-assert tests, fixtures,
and parametrization as shown below.

```python
import pytest
from myapp.calculator import add, divide

def test_add_positive_numbers():
    assert add(2, 3) == 5

def test_add_negative_numbers():
    assert add(-1, -1) == -2

def test_divide_by_zero_raises():
    with pytest.raises(ZeroDivisionError):
        divide(10, 0)

# Parametrized tests
@pytest.mark.parametrize("a,b,expected", [
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
    database = Database(":memory:")
    database.create_tables()
    yield database
    database.close()

@pytest.fixture
def sample_user(db):
    """Create a sample user in the database."""
    user = User(name="Test User", email="test@example.com")
    db.save(user)
    return user

def test_user_creation(db, sample_user):
    found = db.find_user(sample_user.id)
    assert found.name == "Test User"
```

##### Mocking

`unittest.mock` does not exist on Python 2 — install the rolling backport
(`pip install mock==3.0.5`, the last release supporting 2.7) and import
from `mock`. The API is identical.

```python
from mock import Mock, patch, MagicMock  # NOT unittest.mock
import pytest

def test_api_client_with_mock():
    # Create mock
    mock_response = Mock()
    mock_response.json.return_value = {"id": 1, "name": "Test"}
    mock_response.status_code = 200

    with patch('requests.get', return_value=mock_response) as mock_get:
        result = fetch_user(1)

        mock_get.assert_called_once_with('/users/1')
        assert result['name'] == "Test"

@patch('myapp.service.external_api')
def test_with_patch_decorator(mock_api):
    mock_api.get_data.return_value = {"status": "ok"}
    result = process_data()
    assert result["status"] == "ok"
```

#### Error Handling

##### Exception Patterns

```python
# Define custom exceptions -- inherit from Exception, never from
# BaseException, and never use old-style string exceptions
class AppError(Exception):
    """Base exception for application errors."""
    pass

class ValidationError(AppError):
    """Raised when validation fails."""
    def __init__(self, field, message):
        # type: (str, str) -> None
        self.field = field
        self.message = message
        super(ValidationError, self).__init__(
            "{}: {}".format(field, message))

class NotFoundError(AppError):
    """Raised when a resource is not found."""
    def __init__(self, resource, identifier):
        self.resource = resource
        self.identifier = identifier
        super(NotFoundError, self).__init__(
            "{} '{}' not found".format(resource, identifier))
```

##### Exception Handling

```python
import six

def get_user(user_id):
    # type: (int) -> User
    try:
        user = db.find_user(user_id)
        if user is None:
            raise NotFoundError("User", user_id)
        return user
    except DatabaseError as e:  # "except X as e" -- "except X, e" is obsolete
        logger.error("Database error: %s", e)
        # "raise ... from e" is Python-3-only; six.raise_from keeps the cause
        six.raise_from(AppError("Unable to fetch user"), e)

# A bare raise re-raises the active exception with its original traceback
def middleware(request):
    try:
        return handle(request)
    except Exception:
        rollback()
        raise

# Context managers for cleanup
from contextlib import contextmanager

@contextmanager
def database_transaction(db):
    try:
        yield db
        db.commit()
    except Exception:
        db.rollback()
        raise
```

#### Common Patterns

##### Value Objects: attrs and namedtuple

`dataclasses` requires Python 3.7. On 2.7 use `attrs` (`attrs<=21.4.0`,
the last releases that run on Python 2) for the same declarative style, or
`collections.namedtuple` for simple immutable records.

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
    """Immutable point."""
    x = attr.ib()  # type: float
    y = attr.ib()  # type: float

    def distance_to(self, other):
        # type: (Point) -> float
        return ((self.x - other.x) ** 2 + (self.y - other.y) ** 2) ** 0.5

# Stdlib-only alternative for simple records
from collections import namedtuple
Coordinate = namedtuple("Coordinate", ["latitude", "longitude"])
```

##### Context Managers

```python
from contextlib import contextmanager
import time

@contextmanager
def timer(name):
    # type: (str) -> Iterator[None]
    """Time a block of code."""
    start = time.time()  # time.perf_counter() is Python-3-only
    try:
        yield
    finally:
        elapsed = time.time() - start
        print("{}: {:.3f}s".format(name, elapsed))

# Usage
with timer("data processing"):
    process_large_dataset()

# Class-based context manager
class DatabaseConnection(object):
    def __init__(self, connection_string):
        self.connection_string = connection_string
        self.connection = None

    def __enter__(self):
        self.connection = connect(self.connection_string)
        return self.connection

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.connection:
            self.connection.close()
        return False  # Don't suppress exceptions

# contextlib.ExitStack is Python-3-only; use contextlib2 when a dynamic
# number of context managers must be entered together
from contextlib2 import ExitStack
```

##### Iteration and Comprehensions

```python
# Dict and set comprehensions ARE available in 2.7
counts = {word: len(word) for word in words}
unique = {normalize(tag) for tag in tags}

# Prefer the lazy iterators when looping over large dicts
for key, value in mapping.iteritems():   # .items() copies into a list in 2.7
    process(key, value)

# xrange is the lazy range; range() builds a full list
for index in xrange(1000000):
    step(index)

# When 2/3 compatibility matters, route through six instead
import six
for key, value in six.iteritems(mapping):
    process(key, value)

# zip/map/filter return lists in 2.7 -- use itertools for lazy variants
from itertools import izip, imap, ifilter
```

##### Decorators

```python
from functools import wraps
import time

def retry(max_attempts=3, delay=1.0):
    # type: (int, float) -> Callable
    """Retry decorator with exponential backoff."""
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            last_exception = None
            for attempt in xrange(max_attempts):
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    last_exception = e
                    if attempt < max_attempts - 1:
                        time.sleep(delay * (2 ** attempt))
            raise last_exception
        return wrapper
    return decorator

@retry(max_attempts=3, delay=0.5)
def fetch_data(url):
    # type: (str) -> dict
    response = requests.get(url)
    response.raise_for_status()
    return response.json()
```

#### Code Quality Tools

Ruff and Black are Python-3-only. The 2.7 toolchain is flake8 + isort +
pylint, configured through `setup.cfg`/`tox.ini` (not `pyproject.toml`),
with pinned versions: `flake8<3.9` (linter must run on the 2.7
interpreter to parse 2.7 syntax), `isort<5` (4.3.21 is the last 2.7
release), `pylint<2` (1.9.5), `coverage<6`.

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
