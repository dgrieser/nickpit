### Citations: Python 2.7: The Complete Production Developer Guide

Sources for `python-2.7.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: [What’s New in Python 2.7](https://docs.python.org/3/whatsnew/2.7.html) - Author, A.M. Kuchling (amk at amk.ca),. This article explains the new features in Python 2.7. Python...
    - Overview — paragraph 1: "…of the Python 2 line, released on July 3, 2010."

[^2]: [Python 2.7. Unicode Errors Simply Explained - GitHub Gist](https://gist.github.com/gornostal/1f123aaf838506038710) - Python 2.7. Unicode Errors Simply Explained. GitHub Gist: instantly share code, notes, and snippets.
    - File Encoding and the `__future__` Module › Source File Encoding — paragraph 1: "…source file without a declared encoding declaration produces a `SyntaxError`."
    - File Encoding and the `__future__` Module › Source File Encoding — paragraph 2: "…literals, docstrings with non-ASCII characters, or comments in non-ASCII scripts."
    - Strings: `str` vs `unicode` › The Two String Types — bullet 1: "Does NOT inherently carry encoding information."
    - Strings: `str` vs `unicode` › The Two String Types — bullet 2: "Declared with `u''` prefix or created by `unicode()` or `.decode()`."
    - Strings: `str` vs `unicode` › The Two String Types — paragraph 1: "**The Unicode sandwich rule for production code:**"
    - Strings: `str` vs `unicode` › Encoding and Decoding — paragraph 1: "…bytes but raises `UnicodeDecodeError` the moment any byte exceeds 127."

[^3]: [Integer division in Python 2 and Python 3](https://stackoverflow.com/questions/21316968/integer-division-in-python-2-and-python-3/21317109) - How can I divide two numbers in Python 2.7 and get the result with decimals? I don't get it why ther...
    - File Encoding and the `__future__` Module › The `__future__` Module — paragraph 1: "…before any other code (except the docstring and encoding declaration)."
    - File Encoding and the `__future__` Module › The `__future__` Module — paragraph 2: "…(integer division truncation, print statement syntax, silent ASCII encoding errors)."
    - File Encoding and the `__future__` Module › The `__future__` Module — paragraph 3: "…`from __future__ import X` syntax triggers the compiler special-case behavior."
    - Division — paragraph 2: "…division (like Python 3), and `//` always perform floor division."

[^4]: [Should I use the print statement or function in Python 2.7?](https://stackoverflow.com/questions/34191828/should-i-use-the-print-statement-or-function-in-python-2-7/34191915) - To make my simple scripts in Python 2.7.10, should I use the print function instead of the statement...
    - File Encoding and the `__future__` Module › The `__future__` Module — paragraph 2: "…(integer division truncation, print statement syntax, silent ASCII encoding errors)."
    - The `print` Statement — paragraph 1: "…always use the print function via `from __future__ import print_function`."
    - The `print` Statement — paragraph 3: "…NOT the same as calling a function with two arguments."

[^5]: [5. Built-in Types¶](https://docs.python.org/2.7/library/stdtypes.html)
    - Primitive Types and Numerics › Integers: `int` and `long` — paragraph 1: "Python 2.7 has two integer types."
    - Primitive Types and Numerics › Integers: `int` and `long` — paragraph 2: "Always use uppercase `L`."
    - Primitive Types and Numerics › Integers: `int` and `long` — paragraph 3: "**Bitwise operations** work on both `int` and `long`:"
    - Primitive Types and Numerics › Integers: `int` and `long` — paragraph 4: "…`2**n`; the result promotes to `long` if it overflows `int`."
    - Primitive Types and Numerics › Floats — paragraph 1: "Floats are C `double` (64-bit IEEE 754)."
    - Primitive Types and Numerics › Booleans — paragraph 1: "`True == 1` and `False == 0`."
    - Primitive Types and Numerics › Booleans — paragraph 2: "**Falsy values** in Python 2.7:"
    - Strings: `str` vs `unicode` › String Methods — paragraph 1: "All of the following work on both `str` and `unicode`:"
    - Strings: `str` vs `unicode` › String Methods — paragraph 2: "…`index` when absence is a programming error that should raise."
    - Strings: `str` vs `unicode` › String Concatenation Performance — paragraph 1: "…is O(n²) because each `+` creates a new string object."
    - Strings: `str` vs `unicode` › String Concatenation Performance — paragraph 2: "…threshold where `join` becomes measurably faster is around 6+ concatenations."
    - Strings: `str` vs `unicode` › Multiline Strings — paragraph 1: "…at compile time — no `+` needed, no runtime cost."
    - Lists — paragraph 1: "Lists are mutable, ordered, heterogeneous sequences of objects."
    - Lists — paragraph 2: "List multiplication shares references:**"
    - Tuples — paragraph 1: "…are hashable), lighter than lists, and suitable as dictionary keys."
    - Tuples — paragraph 2: "Use lists for homogeneous collections of variable length."
    - Dictionaries — paragraph 1: "…in Python 3.7) mappings from hashable keys to arbitrary values."
    - Sets — paragraph 1: "Sets are mutable, unordered collections of unique, hashable elements."
    - Generators and Iterators › The Iterator Protocol — paragraph 1: "…a `next()` method (returning the next value or raising `StopIteration`)."

[^6]: [python-notes/unicode-vs-str-vs-bytes.md at master · peter-can-write/python-notes](https://github.com/peter-can-write/python-notes/blob/master/unicode-vs-str-vs-bytes.md) - Notes on Python. Contribute to peter-can-write/python-notes development by creating an account on Gi...
    - Strings: `str` vs `unicode` › The Two String Types — bullet 1: "Does NOT inherently carry encoding information."
    - File I/O › Unicode-Safe File I/O — paragraph 2: "…2.7 and Python 3, and it prevents accidental binary/text mixing."

[^7]: [error encoding string as unicode in python 2.7?](https://stackoverflow.com/questions/24742886/error-encoding-string-as-unicode-in-python-2-7) - I want to print the unicode version of a string in Python 2.7. It works fine in Python 3. But with p...
    - Strings: `str` vs `unicode` › Encoding and Decoding — paragraph 1: "…bytes but raises `UnicodeDecodeError` the moment any byte exceeds 127."

[^8]: [Solving Unicode Problems in Python 2.7 - Element 84](https://element84.com/software-engineering/solving-unicode-problems-in-python-2-7/) - We discuss three steps you can take to begin thinking about strings and unicode the right way.
    - Strings: `str` vs `unicode` › Encoding and Decoding — paragraph 2: "…`sys.setdefaultencoding()` as that causes subtle bugs across the whole runtime."

[^9]: [Idiomatic Python: comprehensions - Microsoft Developer Blogs](https://devblogs.microsoft.com/python/idiomatic-python-comprehensions/) - We’re lucky to have a few people on our team who have been programming in Python for quite a while (...
    - Lists › List Comprehensions — paragraph 1: "…are the idiomatic Python way to build or transform lists."
    - Generators and Iterators › Generator Expressions — paragraph 1: "They are memory-efficient for large or infinite sequences."

[^10]: [问dict.items()和dict.iteritems()在Python2中有什么区别？ - 腾讯云](https://cloud.tencent.com/developer/ask/sof/109423346) - 和之间有什么适用的区别吗？来自dict.items()：返回字典(键、值)对列表的副本。dict.iteritems()：在字典的(键，值)对上返回一个迭代器。如果我运行下面的代码，每个代码似乎都会返...
    - Dictionaries — paragraph 2: "…large dicts or when you only need to iterate once."

[^11]: [range vs. xrange in Python](https://python.code-maven.com/range-vs-xrange-in-python)
    - `range` vs `xrange` — paragraph 1: "This is a critical Python 2.7 performance decision."

[^12]: [Python: range() vs. xrange()](http://justindailey.blogspot.com/2011/09/python-range-vs-xrange.html) - I recently discovered a really cool Python module called timeit . It let's you efficiently time bloc...
    - `range` vs `xrange` — paragraph 2: "…`xrange` in `for` loops unless you specifically need a list."

[^13]: [Difference Between range and xrange in Python | xrange vs range](https://www.csestack.org/difference-range-xrange-python/) - What is the main difference between range and xrange? How has this changed over time?
    - `range` vs `xrange` — paragraph 2: "…`xrange` in `for` loops unless you specifically need a list."

[^14]: [Python: Joining multiple generators/iterators - Mark Needham](https://www.markhneedham.com/blog/2015/05/24/python-joining-multiple-generatorsiterators/) - `itertools.chain()` returns an iterator, not a generator. Generator is a subclass of Iterator. Not e...
    - Functions › `map`, `filter`, `reduce` — paragraph 2: "For memory-efficient lazy variants use `itertools.imap`, `itertools.ifilter`, and `itertools.izip`."

[^15]: [List Comprehensions | Set Comprehension | Dict Comprehension | Python Tutorials Ep. 32](https://www.youtube.com/watch?v=3SR110cPcug) - In this Python tutorial, dive deep into the power of list comprehensions, set comprehensions, and di...
    - Generators and Iterators › Generator Expressions — paragraph 1: "They are memory-efficient for large or infinite sequences."

[^16]: [9.7. — Functions creating iterators for efficient looping¶](https://docs.python.org/2.7/library/itertools.html)
    - `itertools` — Efficient Iteration — paragraph 1: "`itertools` provides memory-efficient building blocks for iterators:"
    - The `collections` Module — paragraph 2: "Use `deque` whenever you need FIFO or double-ended access."

[^17]: [2.7. New and Old Style Class - Python - from None to AI](https://www.python3.edu.pl/oop/python/new-style-classes.html) - New and Old Style Class . Important. Old Style classes - Not existing in Python 3. In Python 3 class...
    - Classes and Object-Oriented Programming › New-Style vs Old-Style Classes — paragraph 1: "…use new-style classes** in Python 2.7: inherit explicitly from `object`."
    - Classes and Object-Oriented Programming › New-Style vs Old-Style Classes — paragraph 2: "…not support `super()`, `property`, `classmethod`, `staticmethod`, descriptors, or `__slots__` correctly."

[^18]: [Python Language Tutorial => New-style vs. old-style classes](https://riptutorial.com/python/example/1402/new-style-vs--old-style-classes) - Learn Python Language - New-style vs. old-style classes
    - Classes and Object-Oriented Programming › New-Style vs Old-Style Classes — paragraph 1: "…use new-style classes** in Python 2.7: inherit explicitly from `object`."

[^19]: [Python2.7で class C: と class C(object): の違いに大ハマりした話 - Qiita](https://qiita.com/alt-core/items/d20e9a1864ff3a35946e) - 旧スタイルのクラス定義というものを知っていますか Python2 系では、class C: と class C(object): が異なる挙動をするということを知らずに、大ハマりしました。 参考: h...
    - Classes and Object-Oriented Programming › New-Style vs Old-Style Classes — paragraph 2: "…not support `super()`, `property`, `classmethod`, `staticmethod`, descriptors, or `__slots__` correctly."

[^20]: [Usage of __slots__?](https://stackoverflow.com/questions/472000/usage-of-slots) - What is the purpose of __slots__ in Python — especially with respect to when I would want to use it,...
    - Classes and Object-Oriented Programming › `__slots__` — paragraph 1: "…reduces memory usage significantly for large numbers of small objects:"
    - Classes and Object-Oriented Programming › `__slots__` — paragraph 2: "…`__slots__` (without re-declaring parent slots) to gain the memory benefit."

[^21]: [Inherit class with weakref in slots](https://stackoverflow.com/questions/24407874/inherit-class-with-weakref-in-slots) - I tried to use weak references on my classes, where I use slots to save some memory, but I wasn't ab...
    - Classes and Object-Oriented Programming › `__slots__` — paragraph 2: "…`__slots__` (without re-declaring parent slots) to gain the memory benefit."

[^22]: [Python 2.7 exception handling syntax - Stack Overflow](https://stackoverflow.com/questions/32613375/python-2-7-exception-handling-syntax) - Catch exceptions of the type Exception (or any subclass) and store them in the variable e … don't ca...
    - Exception Handling › Syntax — paragraph 1: "Always use `as`:"

[^23]: [exception handling | Python Best Practices](https://realpython.com/ref/best-practices/exception-handling/) - Guidelines and best practices for handling exceptions and errors in your Python code.
    - Exception Handling › Exception Hierarchy — paragraph 1: "Catching `BaseException` also catches `KeyboardInterrupt` and `SystemExit`, preventing clean shutdown."

[^24]: [Python Exception Handling: Patterns and Best Practices - Jerry Ng](https://jerrynsh.com/python-exception-handling-patterns-and-best-practices/) - In terms of best practices – it is generally recommended to use the from e syntax when raising a new...
    - Exception Handling › Re-raising Exceptions — paragraph 1: "…e` — that resets the traceback to the current line."

[^25]: [PEP 343 – The “with” Statement - Python Enhancement Proposals](https://peps.python.org/pep-0343/) - This PEP adds a new statement “with” to the Python language to make it possible to factor out standa...
    - Context Managers and the `with` Statement — paragraph 1: "…`with` statement ensures cleanup code runs even when exceptions occur."

[^26]: [3.4.9 With Statement Context Managers](https://python.developpez.com/cours/PythonDocs/ref/context-managers.php) - Documentation officielle de Python
    - Context Managers and the `with` Statement › Writing a Context Manager Class — paragraph 1: "Suppressing exceptions should be rare and deliberate."

[^27]: [Python's with Statement and Context Managers](https://www.blog.pythonlibrary.org/2021/04/07/pythons-with-statement-and-context-managers/) - Learn about Python's context managers, the contextlib module and Python's with statement in this art...
    - Context Managers and the `with` Statement › `contextlib.contextmanager` — paragraph 1: "…to write context managers as generators, which is often cleaner:"

[^28]: [Python Security: 6 Common Risks & What You Can Do About Them](https://www.aquasec.com/cloud-native-academy/application-security/python-security/) - 6 Common Python Security Vulnerabilities · 1. Injections and Arbitrary Command Execution · 2. Overly...
    - File I/O › Temporary Files — paragraph 1: "…to create a file with the same name in between)."
    - JSON, CSV, and Data Serialization › Pickle — Use with Extreme Caution — paragraph 1: "…file can execute shell commands, read files, or install backdoors."
    - Secure Coding Patterns › SQL Injection Prevention — paragraph 1: "Never format SQL strings with user input:"

[^29]: [Why does integer division yield a float instead of another integer?](https://stackoverflow.com/questions/1282945/why-does-integer-division-yield-a-float-instead-of-another-integer) - In Python 2.7: By default, division operator will return integer output. To get the result in double...
    - Division — paragraph 1: "…**integer division** when both operands are integers, discarding the remainder."

[^30]: [Why does Python 2 use '/' only as integer division?](https://stackoverflow.com/questions/42380531/why-does-python-2-use-only-as-integer-division) - I have noticed that Python 2.7 uses '/' as integer division. >>> print 1/2 0 Since Python is a dynam...
    - Division — paragraph 1: "…**integer division** when both operands are integers, discarding the remainder."

[^31]: [PEP 238 – Changing the Division Operator | peps.python.org](https://peps.python.org/pep-0238/) - The future division statement, spelled from __future__ import division , will change the / operator ...
    - Division — paragraph 2: "…division (like Python 3), and `//` always perform floor division."

[^32]: [logging | Python Best Practices](https://realpython.com/ref/best-practices/logging/) - Guidelines and best practices for logging in Python.
    - Logging — paragraph 1: "…the `logging` module — never `print` — for production observability."
    - Logging — paragraph 2: "**Critical rules:**"

[^33]: [Python best practice in terms of logging](https://stackoverflow.com/questions/22807972/python-best-practice-in-terms-of-logging) - When using the logging module from python for logging purposes. Is it best-practice to define a logg...
    - Logging — paragraph 2: "**Critical rules:**"

[^34]: [Dangerous os.system() or os.popen() Call - Python SAST Security Rule | Code Pathfinder](https://codepathfinder.dev/registry/python/lang/PYTHON-LANG-SEC-010) - os.system() and os.popen() execute shell commands via /bin/sh, enabling command injection when argum...
    - Subprocess and Shell Execution — paragraph 1: "…the command to the system shell, enabling OS command injection."

[^35]: [Preventing my script from os command injection python](https://stackoverflow.com/questions/44040687/preventing-my-script-from-os-command-injection-python) - i am using python 2.7.x I automating my stuffs and in there i need run to another python program fro...
    - Subprocess and Shell Execution — paragraph 1: "…the command to the system shell, enabling OS command injection."

[^36]: [Living with the GIL: Strategies for Concurrent Python - DEV Community](https://dev.to/aaron_rose_0787cc8b4775a0/living-with-the-gil-strategies-for-concurrent-python-2d5e) - Timothy and Margaret walked through the library's quiet reading room toward the small coffee shop in...
    - Concurrency › `threading` — paragraph 2: "Even `+=` on an integer is not atomic in Python."

[^37]: [Understanding Python's Garbage Collection and Memory Optimization](https://dev.to/pragativerma18/understanding-pythons-garbage-collection-and-memory-optimization-4mi2) - While Python's garbage collection system handles memory cleanup automatically, unoptimized memory us...
    - Memory Management — paragraph 1: "…reference count drops to zero, its memory is reclaimed immediately."

[^38]: [20% Faster Python with a Single GC Tweak - Michael Kennedy](https://mkennedy.codes/posts/python-gc-settings-change-this-and-make-your-app-go-20pc-faster/) - 20% Faster Python with a Single GC Tweak ... TL;DR: Increase Python's GC allocation threshold from 7...
    - Memory Management › Cyclic Garbage Collector — paragraph 2: "…(e.g., to 50,000 from 700) can significantly reduce GC overhead."
