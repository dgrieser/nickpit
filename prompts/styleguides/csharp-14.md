### C# 14 on .NET 10 (LTS): The Complete Production Developer Guide

> **Version**: C# 14 ships with .NET 10, released **November 11, 2025** (SDK 10.0.100, Visual Studio 2026 18.0).[^1][^2] Latest servicing release as of October 2026: **.NET 10.0.13 (October 13, 2026)** with SDKs **10.0.402** and **10.0.113**; it fixes 44 CVEs, so anything older than 10.0.13 is a finding.[^3]
> **Support**: .NET 10 is a **Long Term Support (LTS)** release, in the *active* phase, supported until **November 14, 2028**.[^3][^4] .NET 8 (LTS) and .NET 9 (STS) both reach end of support on **November 10, 2026**.[^4] **.NET 11 / C# 15 is not released**: .NET 11 RC2 (11.0.0-rc.2, SDK 11.0.100-rc.2) shipped on October 13, 2026 with a go-live license; GA is planned for November 2026.[^5]
> **Scope**: The C# 14 language, the .NET 10 base class library, runtime, SDK, ASP.NET Core 10 and EF Core 10 as they affect production code and code review: idioms, pitfalls, security, performance, testing, analyzers, trimming and Native AOT.
> **Selection**: This guide applies when the project's language version is 14 — an explicit `<LangVersion>14</LangVersion>` (or `14.0`), or `<TargetFramework>net10.0</TargetFramework>` with no `LangVersion`, because C# 14 is the default for `net10.0`.[^6]
> **Note**: This document is self-contained. Sections marked **New in C# 14** or **New in .NET 10** cover version-specific behavior; everything else applies to all modern C#. Every behavioral claim and example was compiled and run on SDK 10.0.402 / runtime 10.0.13 (Linux x64); "new in" boundaries were checked by compiling with `LangVersion` 13 and/or targeting `net9.0` (runtime 9.0.21).

---

#### Table of Contents

1. [Overview and Global Truths](#overview-and-global-truths)
2. [What's New in C# 14](#whats-new-in-c-14)
3. [What's New in .NET 10 for Reviewers](#whats-new-in-net-10-for-reviewers)
4. [Do Not Use or Recommend (C# 15 / .NET 11)](#do-not-use-or-recommend-c-15--net-11)
5. [Deprecated, Obsoleted and Removed APIs](#deprecated-obsoleted-and-removed-apis)
6. [Version-Aware Review](#version-aware-review)
7. [Project Setup, Build and Analyzers](#project-setup-build-and-analyzers)
8. [Naming and Code Style](#naming-and-code-style)
9. [Nullable Reference Types](#nullable-reference-types)
10. [Types: Classes, Records, Structs, Primary Constructors](#types-classes-records-structs-primary-constructors)
11. [Pattern Matching](#pattern-matching)
12. [Collections, Collection Expressions, Span and Memory](#collections-collection-expressions-span-and-memory)
13. [LINQ](#linq)
14. [Exceptions and Error Handling](#exceptions-and-error-handling)
15. [Async/Await, Cancellation and Concurrency](#asyncawait-cancellation-and-concurrency)
16. [IDisposable and IAsyncDisposable](#idisposable-and-iasyncdisposable)
17. [Dependency Injection, Options, Configuration and Hosting](#dependency-injection-options-configuration-and-hosting)
18. [Logging and Observability](#logging-and-observability)
19. [HttpClient and IHttpClientFactory](#httpclient-and-ihttpclientfactory)
20. [System.Text.Json](#systemtextjson)
21. [ASP.NET Core 10 Minimal APIs](#aspnet-core-10-minimal-apis)
22. [EF Core 10](#ef-core-10)
23. [Security](#security)
24. [Performance](#performance)
25. [Testing](#testing)
26. [Trimming and Native AOT](#trimming-and-native-aot)
27. [Removed / Changed in .NET 10 — Migration Notes](#removed--changed-in-net-10--migration-notes)
28. [Production / Review Checklist](#production--review-checklist)

---

#### Overview and Global Truths

C# 14 adds **extension members** (extension properties, static extension members and extension operators declared in `extension` blocks), the **`field` keyword** for field-backed properties, **null-conditional assignment** (`a?.B = x`), **`nameof` on unbound generic types** (`nameof(List<>)`), **first-class `Span<T>`/`ReadOnlySpan<T>` conversions**, **parameter modifiers on implicitly typed lambda parameters**, **partial constructors and partial events**, **user-defined compound assignment and increment operators**, optional and named arguments in expression trees, and the `#:` directives of **file-based apps**.[^6][^7] Several of these change the meaning of existing code (`field` inside accessors, overload resolution with spans, `extension`/`partial`/`scoped` as contextual keywords); see [Breaking changes in C# 14](#breaking-changes-in-c-14-must-know-for-upgrades).

.NET 10 is the LTS successor of .NET 8. Its reviewer-relevant changes include: the runtime **no longer installs a default SIGTERM handler**, `BackgroundService.ExecuteAsync` **runs entirely on a background thread**, configuration binding **preserves `null`**, `System.Linq.AsyncEnumerable` is **in the box** (conflicts with `System.Linq.Async`), System.Text.Json gains `AllowDuplicateProperties`, `JsonSerializerOptions.Strict` and metadata-name conflict validation, LINQ gains `LeftJoin`/`RightJoin`/`Shuffle`/`Sequence`/`InfiniteSequence`, post-quantum cryptography (`MLKem`, `MLDsa`, `SlhDsa`, `CompositeMLDsa`), `Rfc2898DeriveBytes` constructors are obsolete, and ASP.NET Core 10 adds built-in minimal-API validation and returns **401/403 instead of login redirects** for API endpoints.[^8][^9][^10]

Everything from earlier versions is available and expected in modern code: records and `init` (C# 9); global usings, file-scoped namespaces, record structs (C# 10); raw string literals, list patterns, `required` members, generic math, UTF-8 literals `"..."u8` (C# 11); primary constructors for classes, collection expressions, `[InlineArray]`, alias any type, `ref readonly` parameters (C# 12); `params` collections, the `System.Threading.Lock` type, `\e`, partial properties/indexers, `ref struct` interfaces and `allows ref struct`, `[OverloadResolutionPriority]` (C# 13).[^11]

Global truths:

- **Managed, garbage-collected, type-safe.** Value types (`struct`, `record struct`, enums, primitives) are copied; reference types are shared. The GC is generational; `Span<T>` and `ref struct` keep stack-only data off the heap.
- **Nullable reference types are a compile-time contract only.** `string?` vs `string` produces warnings, never runtime checks. Public APIs still validate (`ArgumentNullException.ThrowIfNull`).
- **Async is pervasive.** I/O APIs are asynchronous, take a `CancellationToken`, and return `Task`/`ValueTask`. Blocking on them (`.Result`, `.Wait()`) is a bug in server code.
- **The SDK pins the language, not the runtime.** `LangVersion` defaults from the target framework (`net10.0` → 14, `net9.0` → 13). Using C# 14 with an older target framework is unsupported and can change overload resolution (see `Enumerable.Reverse` below).[^7]
- **Analyzers are part of the compiler pipeline.** .NET analyzers (CAxxxx), IDE rules (IDExxxx), platform obsoletions (SYSLIBxxxx) and package analyzers (EF1003, xUnit1xxx) are build diagnostics; `TreatWarningsAsErrors` turns them into gates.

---

#### What's New in C# 14

All examples in this section compile with `LangVersion` 14 and fail with `LangVersion` 13 with `CS9260: Feature '…' is not available in C# 13.0` (or a parse error for partial constructors).

##### 1. Extension members (`extension` blocks) [New in C# 14] ★★★

An `extension` block inside a non-generic static class declares members for a *receiver*. A block with a named receiver parameter declares instance-like members (methods and **properties**); a block with only a receiver type declares **static** members of the extended type, including **user-defined operators**.[^6][^12]

```csharp
var xs = new List<int> { 1, 2, 3 };
Console.WriteLine(xs.IsEmpty);                         // False  (extension property)
Console.WriteLine(new List<int>().IsEmpty);            // True
Console.WriteLine(IEnumerable<int>.Identity.Count());  // 0      (static extension property)
var both = xs.AsEnumerable() + new[] { 4 };            // extension operator
Console.WriteLine(string.Join(",", both));             // 1,2,3,4
Console.WriteLine(SequenceExtensions.get_IsEmpty(xs)); // False  (lowered accessor is callable)

static class SequenceExtensions
{
    extension<T>(IEnumerable<T> source)                // instance-like members
    {
        public bool IsEmpty => !source.Any();
        public int CountFast() => source.TryGetNonEnumeratedCount(out var n) ? n : source.Count();
    }

    extension<T>(IEnumerable<T>)                       // static members of IEnumerable<T>
    {
        public static IEnumerable<T> Identity => Enumerable.Empty<T>();
        public static IEnumerable<T> operator +(IEnumerable<T> left, IEnumerable<T> right) => left.Concat(right);
    }
}
```

Review notes:

- Extension members in blocks are source- and binary-compatible with classic `this`-parameter extension methods: a method in a block is emitted as a static method of the containing class and is still callable as `SequenceExtensions.CountFast(xs)`. Converting existing `this` methods to a block is not a breaking change for callers.
- Extension properties cannot have backing state (no instance fields exist on the receiver). A property that "caches" into a static `Dictionary` keyed by the receiver keeps every receiver alive (memory leak); use `ConditionalWeakTable<TKey,TValue>` if per-instance state is unavoidable.
- **Indexers in extension blocks are C# 15** (not available). **Extension constants/fields do not exist.**
- `extension` is now a contextual keyword: a type, alias or type parameter named `extension` no longer compiles (escape as `@extension`).[^7]
- Prefer a real instance member when you own the type. Use extension members for types you do not own, for interface "default" helpers, and for operators on generic interfaces (the .NET 10 `System.Numerics.Tensors` package uses extension operators this way).[^8]

##### 2. The `field` keyword [New in C# 14] ★★★

Inside a property accessor, `field` refers to a compiler-synthesized backing field, so validation or normalization no longer needs a hand-written field.[^6]

```csharp
var p = new Person();
p.Name = "  Ada ";
Console.WriteLine($"[{p.Name}]");          // [Ada]
try { p.Name = null!; }
catch (ArgumentNullException e) { Console.WriteLine(e.ParamName); }   // value
Console.WriteLine(p.Tags.Count);           // 0 — lazily created once

class Person
{
    public string Name
    {
        get;
        set => field = value?.Trim() ?? throw new ArgumentNullException(nameof(value));
    } = "";                                 // initializer assigns the backing field directly

    public List<string> Tags => field ??= [];   // lazy init (NOT thread-safe)
}
```

Pitfalls a reviewer must catch:

- **Silent meaning change.** In a type that already has a member named `field`, the identifier `field` *inside an accessor* now binds to the synthesized backing field. The compiler only warns (CS9258). Verified:

```csharp
class Legacy
{
    private int field = 1;
    public int P { get { return field; } }  // C# 13: returns 1. C# 14: warning CS9258, returns 0
    public int Q => this.field;             // explicit member access: returns 1 in both
}
```

  Upgrading a project to C# 14 must treat CS9258 as an error: rename the member or write `this.field`/`@field`. A local variable (or a lambda/local-function parameter) named `field` in an accessor is now an error: Roslyn 5.9 (SDK 10.0.402) reports it as **CS9273** (the breaking-changes document still says CS9272).[^7]
- The property initializer (`= ""`) assigns the backing field directly and bypasses the `set` logic, so the initializer value itself must already be valid. Object initializers, `with { Name = … }` and serializers go through the accessor; members not named in a `with` expression are copied field-by-field.
- `field ??= …` lazy initialization is not thread-safe; use `Lazy<T>` or `LazyInitializer.EnsureInitialized` for shared instances.
- `[field: JsonIgnore]`-style attribute targets apply to the synthesized field as before.

##### 3. Null-conditional assignment [New in C# 14] ★★

`?.` and `?[]` may appear on the left of `=` and compound assignments. The right-hand side is evaluated **only if** the receiver is non-null.[^6]

```csharp
Customer? c = null;
int calls = 0;
int Next() { calls++; return 42; }

c?.Total = Next();                 // c is null: Next() is NOT called
Console.WriteLine($"calls={calls}");                 // calls=0
c = new Customer();
c?.Total = Next();
c?.Total += 8;
Console.WriteLine($"calls={calls} total={c?.Total}"); // calls=1 total=50

int[]? arr = null;
arr?[0] = 5;                       // no-op, no exception

class Customer { public int Total; }
```

Review notes: `++`/`--` are not allowed (`c?.Total++` is an error). The right-hand side's side effects are skipped when the receiver is null — code that relies on them (logging, counters, `MoveNext()`) has changed behavior when rewritten from an `if` with an `else`. Silently ignoring a null receiver can also hide a bug; prefer an explicit null check (or `ArgumentNullException.ThrowIfNull`) when null is not a legal state.

##### 4. `nameof` with unbound generic types [New in C# 14] ★

```csharp
Console.WriteLine(nameof(List<>));          // List
Console.WriteLine(nameof(Dictionary<,>));   // Dictionary
Console.WriteLine(nameof(List<>.Count));    // Count
```

Before C# 14 a closed type (`nameof(List<int>)`) was required.[^6]

##### 5. First-class span conversions [New in C# 14] ★★★

The language now has implicit conversions `T[]` → `Span<T>`/`ReadOnlySpan<T>`, `Span<T>` → `ReadOnlySpan<T>` and `string` → `ReadOnlySpan<char>` that participate in **extension-method receiver lookup** and **generic type inference**.[^6][^13]

```csharp
int[] arr = [1, 2, 3];
Console.WriteLine(arr.StartsWith(1));   // binds MemoryExtensions.StartsWith<T>(ReadOnlySpan<T>, T); C# 13: CS1929
Console.WriteLine(Sum(arr));            // 3; C# 13: CS0411 (cannot infer T)

static int Sum<T>(ReadOnlySpan<T> xs) => xs.Length;
```

This is the C# 14 change most likely to **change behavior of existing code without any edit**:

- **Overload resolution can now pick a span overload.** With covariant arrays this throws at run time (verified: C# 13 prints `IEnumerable`, C# 14 throws `ArrayTypeMismatchException`):

```csharp
string[] s = ["a"];
object[] o = s;                         // array covariance
try { C.R(o); } catch (Exception e) { Console.WriteLine(e.GetType().Name); }  // ArrayTypeMismatchException

static class C
{
    public static void R<T>(IEnumerable<T> e) => Console.WriteLine("IEnumerable");
    public static void R<T>(Span<T> s) => Console.WriteLine("Span");   // now chosen in C# 14
}
```

  Library authors should offer `ReadOnlySpan<T>` (preferred by the new rules) rather than `Span<T>` overloads, or use `[OverloadResolutionPriority]`. Callers can force the old binding with `o.AsEnumerable()`.[^7]
- **Expression trees.** `array.Contains(x)` inside an `Expression<…>` now binds to `MemoryExtensions.Contains(ReadOnlySpan<T>, T)` instead of `Enumerable.Contains`. Verified: the tree contains an `op_Implicit` to `ReadOnlySpan<int>`; `Compile()` works, but `Compile(preferInterpretation: true)` throws `ArgumentException` (the interpreter cannot handle the `ReadOnlySpan<int>` generic argument), and LINQ providers that do not recognize `MemoryExtensions` fail to translate it. EF Core 10 (and patched EF Core 9.0.x) translate it; for other providers (OData, older Cosmos/Mongo providers, custom visitors) write `Enumerable.Contains(ids, x)` or `ids.AsEnumerable().Contains(x)`.

```csharp
using System.Linq.Expressions;

int[] ids = [1, 2];
Expression<Func<int, bool>> e = x => ids.Contains(x);
var call = (MethodCallExpression)e.Body;
Console.WriteLine(call.Method.DeclaringType + "." + call.Method.Name); // System.MemoryExtensions.Contains (C# 13: System.Linq.Enumerable.Contains)

Expression<Func<int, bool>> safe = x => Enumerable.Contains(ids, x);  // stays Enumerable.Contains
Console.WriteLine(safe.Compile(preferInterpretation: true)(2));        // True
```

- **`Enumerable.Reverse` on arrays** binds to the in-place, `void`-returning `MemoryExtensions.Reverse(Span<T>)` when C# 14 is used with a target framework **older than net10.0** (verified on net9.0: `var y = x.Reverse();` → CS0815). On `net10.0`, the new `Enumerable.Reverse<T>(this T[])` overload wins and behavior is unchanged.[^7]

##### 6. Modifiers on simple lambda parameters [New in C# 14] ★

`ref`, `out`, `in`, `ref readonly` and `scoped` can be used without spelling out parameter types; `params` still requires explicit types.[^6]

```csharp
TryParse<int> parse = (text, out result) => int.TryParse(text, out result);
Console.WriteLine(parse("12", out var v) + " " + v);   // True 12
Bump bump = (ref x) => x++;
int n = 1; bump(ref n); Console.WriteLine(n);           // 2

delegate bool TryParse<T>(string text, out T result);
delegate void Bump(ref int x);
```

`scoped` in a lambda parameter list is now always a modifier (`(scoped scoped s) => …` with a type named `scoped` breaks).[^7]

##### 7. Partial constructors and partial events [New in C# 14] ★

Instance constructors and events can be `partial` (one defining and one implementing declaration) — primarily for source generators. Only the implementing constructor may have a `this(...)`/`base(...)` initializer; the implementing event declaration must have `add`/`remove` accessors.[^6]

```csharp
var w = new Widget(5);
w.Changed += (_, _) => Console.WriteLine("changed");
w.Raise();                       // changed
Console.WriteLine(w.Size);       // 5

partial class Widget
{
    public partial Widget(int size);
    public partial event EventHandler? Changed;
}

partial class Widget
{
    private EventHandler? _changed;
    public int Size { get; }
    public partial Widget(int size) : this() { Size = size; }
    private Widget() { }
    public partial event EventHandler? Changed { add => _changed += value; remove => _changed -= value; }
    public void Raise() => _changed?.Invoke(this, EventArgs.Empty);
}
```

`partial` can no longer be used as a method return type without escaping (`@partial`).[^7] Partial *interface* properties and events became implicitly `public virtual` (previously they were not), which can change which implementation is called.[^7]

##### 8. User-defined compound assignment and increment operators [New in C# 14] ★★

A type can declare instance `operator +=`, `operator -=`, …, `operator ++` and `operator --` (returning `void`) that **mutate the receiver in place** instead of producing a new instance through `operator +`.[^14]

```csharp
var acc = new Accumulator();
var alias = acc;
acc += 5;
acc += 7;
Console.WriteLine($"{alias.Total} same={ReferenceEquals(alias, acc)}");  // 12 same=True

class Accumulator
{
    public int Total;
    public void operator +=(int x) => Total += x;   // in-place, no allocation
}
```

Review notes: for **reference types** an in-place `+=` is visible through every alias (`alias` above), unlike the classic `x = x + y` semantics where `x` is rebound to a new object. Do not add mutating compound operators to types that are used as values (immutable domain types, dictionary keys, types shared across threads). Keep `operator +` and `operator +=` semantically consistent.

##### 9. Optional and named arguments in expression trees [New in C# 14]

Expression trees may now contain calls that rely on optional parameters or use named arguments (in position order). Verified: C# 13 reports CS0854/CS0853; C# 14 compiles `() => M.F(1)` as `() => F(1, 10)`.[^15]

```csharp
using System.Linq.Expressions;

Expression<Func<string>> e = () => M.F(1);
Console.WriteLine(e);                       // () => F(1, 10)
Expression<Func<string>> e2 = () => M.F(a: 2, b: 3);
Console.WriteLine(e2.Compile()());          // 2+3

static class M { public static string F(int a, int b = 10) => $"{a}+{b}"; }
```

The default value is baked into the tree at compile time; changing the default in a library later does not affect already-compiled callers.

##### 10. File-based apps and `#:` directives [New in .NET 10 SDK]

`dotnet run app.cs` runs a single C# file without a project. `#:package Name@Version`, `#:sdk Microsoft.NET.Sdk.Web`, `#:property Name=Value` and `#:project ../Lib/Lib.csproj` configure the implicit project; a `#!/usr/bin/env dotnet` shebang makes the file executable. File-based apps **default to `PublishAot=true`** (and `PackAsTool=true`; verified with `dotnet project convert app.cs` on SDK 10.0.402); `dotnet publish app.cs` produces a native executable.[^16] Directives containing double quotes are rejected by the .NET 10 SDK (`error: Directives currently cannot contain double quotes (")`); quoted directive values arrive only with the .NET 11 SDK.[^9][^5] `dnx`/`dotnet tool exec` and file-based apps do not replace a project for production code.

Review notes: a `#:package` directive without a version, or with a floating version, is not reproducible; pin versions. These files are scripts, not libraries — production services still need a project, central package management and the analyzers configured below.

##### 11. Other compiler changes that surface in review

- **Redundant `or` pattern warning (CS9336).** `o is not null or 42` is parsed as `(not null) or 42`; C# 14 warns "The pattern is redundant". The author almost always meant `is not (null or 42)`. Verified: the two expressions return `True` and `False` for `o = 42`; C# 13 gives no warning.[^7]
- **Disposed iterators stop.** `MoveNext()` on a disposed compiler-generated enumerator now returns `false` instead of resuming user code.[^7] This and the partial interface member change (section 7) come from the newer compiler and apply even with `LangVersion` 13 (verified on SDK 10.0.402).
- **`[Obsolete]` on pattern-based `DisposeAsync`/`Dispose` is now reported in `foreach`/`await foreach`.**[^7]

##### Breaking changes in C# 14 (must know for upgrades)

| Change | Effect | Fix |
| --- | --- | --- |
| `field` in accessors binds to the backing field | Existing member named `field` silently stops being read (warning CS9258); local/parameter named `field` → error CS9273 | `this.field` / `@field`, or rename |
| First-class span conversions | Different overload chosen; `ArrayTypeMismatchException` with covariant arrays; ambiguity errors; expression trees contain `MemoryExtensions` calls | `AsEnumerable()`, `Enumerable.X(...)`, prefer `ReadOnlySpan<T>` overloads, `[OverloadResolutionPriority]` |
| `extension` is a contextual keyword | Types/aliases/type parameters named `extension` fail to compile | `@extension` |
| `partial` return type | `partial F()` no longer parses as a method returning type `partial` | `@partial` |
| `scoped` in lambdas | Always a modifier | `@scoped` for a type name |
| Partial interface properties/events | Implicitly `public virtual` | Add `private`/`sealed` to keep old behavior |
| Redundant `or` pattern | New warning CS9336 | Parenthesize: `is not (A or B)` |

Source: Roslyn "Compiler breaking changes since C# 13".[^7]

---

#### What's New in .NET 10 for Reviewers

##### Runtime and hosting behavior changes [New in .NET 10] ★★★

**No default SIGTERM handler.** On Unix the runtime no longer installs a SIGTERM handler, and on Windows no longer handles `CTRL_CLOSE_EVENT`/`CTRL_SHUTDOWN_EVENT`. A plain console app that relies on `AppDomain.ProcessExit` to flush or clean up on `docker stop`/Kubernetes termination now gets killed without running it.[^17] Verified with `kill -TERM`: on .NET 9 the program printed `ProcessExit raised`; on .NET 10 the process died (exit 143) without it. Apps built on the Generic Host (`Host.CreateApplicationBuilder`, ASP.NET Core) are not affected because the host's console lifetime registers its own handlers.

```csharp
using System.Runtime.InteropServices;

// Plain console/worker without the Generic Host: handle termination explicitly.
using var cts = new CancellationTokenSource();
using var sigterm = PosixSignalRegistration.Create(PosixSignal.SIGTERM, ctx =>
{
    ctx.Cancel = true;      // don't terminate immediately; let the loop drain
    cts.Cancel();
});

try
{
    await Task.Delay(Timeout.Infinite, cts.Token);
}
catch (OperationCanceledException)
{
    Console.WriteLine("draining and exiting");
}
```

**`BackgroundService.ExecuteAsync` runs entirely on a background thread.** Previously the synchronous part before the first `await` ran on the startup thread and blocked other hosted services and the server from starting. Verified with Microsoft.Extensions.Hosting 9.0 vs 10.0: on 9.0 the synchronous prefix ran on thread 1 and finished *before* `StartAsync` returned; on 10.0 it ran on a pool thread and `StartAsync` returned first.[^18] Code that *relied* on that ordering (e.g., "initialize the cache synchronously at the top of `ExecuteAsync` before the app accepts requests") is now racy — move such work into `StartAsync` (before `base.StartAsync`), an `IHostedLifecycleService`, or a startup task awaited before `app.Run()`.

**Configuration preserves `null`.** The JSON provider no longer turns `null` into `""`, the binder binds `null` (and empty arrays). Verified with `{ "Name": null, "Count": null, "Tags": [] }` bound to a class whose properties default to `"init"`, `7` and `null`: .NET 9 produced `Name=''`, `Count=7`, `Tags=null`; .NET 10 produces `Name=null`, `Count=null`, `Tags=[]`. A `null` bound to a non-nullable value type now yields `default(T)` instead of throwing.[^19] Options classes that assumed "property keeps its default unless configured" need `[Required]` validation or nullable types.

**`System.Linq.AsyncEnumerable` is in the box.** LINQ for `IAsyncEnumerable<T>` (`Where`, `Select`, `ToListAsync`, …) ships in .NET 10; a direct or transitive reference to the community `System.Linq.Async` (< 7.0) causes ambiguity errors. Remove it, upgrade to 7.x, or `<ExcludeAssets>compile</ExcludeAssets>` for transitive references. `SelectAwait` and friends become `Select` overloads that take `async` lambdas with a `CancellationToken`.[^20]

**Other runtime/library behavior changes** (all .NET 10):[^9]

- `Uri` length limits were removed (verified: a 70,015-character URI throws `UriFormatException` on .NET 9 and parses on .NET 10). Code that relied on the limit as an input-size guard must validate length itself.
- `GnuTarEntry`/`PaxTarEntry` no longer include atime/ctime by default.
- `DriveInfo.DriveFormat` returns real Linux filesystem type names.
- `MailAddress` rejects consecutive dots.
- Single-file apps no longer probe the executable's directory for native libraries; `DllImportSearchPath.AssemblyDirectory` searches only the assembly directory.
- Default `DistributedContextPropagator` is the W3C propagator; `ActivitySource.CreateActivity/StartActivity` sampling behavior changed.
- HTTP/3 is disabled by default when `PublishTrimmed` is set.
- OpenSSL 1.1.1 or later is required on Unix; the override variables are renamed to `DOTNET_OPENSSL_VERSION_OVERRIDE`/`DOTNET_ICU_VERSION_OVERRIDE`.
- `XmlSerializer` no longer ignores `[Obsolete]` properties.
- Default container images are based on Ubuntu (previously Debian).
- `BufferedStream.WriteByte` no longer flushes implicitly when the internal buffer is full; data reaches the underlying stream only on `Flush`/`FlushAsync` or dispose. Code that reads the underlying stream while still writing must flush explicitly.
- Shifts through generic math (`IShiftOperators<TSelf, TOther, TResult>`) now mask the shift amount consistently for all built-in integer types ("overshifting" a `byte` by 8 no longer differs by type).
- `[InlineArray]` structs with an explicit `StructLayout(Size = …)` throw `TypeLoadException` when instantiated.
- Crypto API changes since the .NET 10 previews: `MLDsa`/`SlhDsa` `*SecretKey*` members are renamed to `*PrivateKey*` (e.g. `ExportMLDsaPrivateKey`), `CompositeMLDsa` follows draft-08, `CoseSigner.Key`, `X509Certificate`/`PublicKey` key parameters can be `null`, `X500DistinguishedName` parsing is stricter, and OpenSSL-backed primitives are not supported on macOS.
- `FilePatternMatch.Stem` is non-nullable; `ProviderAliasAttribute` moved to `Microsoft.Extensions.Logging.Abstractions`.

##### New BCL APIs worth recommending [New in .NET 10]

Verified to exist on `net10.0` and to be absent on `net9.0`:[^8]

| Area | API |
| --- | --- |
| LINQ | `Enumerable.LeftJoin`, `Enumerable.RightJoin` (and `Queryable` equivalents, translated by EF Core 10), `Enumerable.Shuffle`, `Enumerable.Sequence`, `Enumerable.InfiniteSequence`, `System.Linq.AsyncEnumerable` |
| Strings/globalization | `CompareOptions.NumericOrdering` (`"v9" < "v10"`), span-based normalization (`IsNormalized`/`TryNormalize`/`GetNormalizedLength` on `ReadOnlySpan<char>`), `Convert.FromHexString(ReadOnlySpan<byte>)` and UTF-8 hex conversions, `ISOWeek` overloads for `DateOnly` |
| Collections | `OrderedDictionary<TKey,TValue>.TryAdd(key, value, out int index)` and `TryGetValue(key, out value, out int index)` |
| JSON | `JsonSerializerOptions.AllowDuplicateProperties`, `JsonSerializerOptions.Strict`, `JsonDocumentOptions.AllowDuplicateProperties`, `ReferenceHandler` in `[JsonSourceGenerationOptions]`, `PipeReader` overloads of `DeserializeAsync` |
| Crypto | `MLKem`, `MLDsa`, `SlhDsa`, `CompositeMLDsa` (`SlhDsa` and `CompositeMLDsa` are `[Experimental]` types, SYSLIB5006; `MLKem`/`MLDsa` are not, but some of their PKCS#8/SubjectPublicKeyInfo/PEM import/export members are), `X509Certificate2Collection.FindByThumbprint(HashAlgorithmName, string)`, `PemEncoding.FindUtf8`, `X509Certificate.ExportPkcs12(Pkcs12ExportPbeParameters, string)`, AES key wrap with padding (`Aes.EncryptKeyWrapPadded`/`DecryptKeyWrapPadded`) |
| I/O | Async ZIP APIs (`ZipFile.OpenReadAsync`, `ZipFile.ExtractToDirectoryAsync`, `ZipArchiveEntry.OpenAsync`, …), `WebSocketStream` |
| Diagnostics | `ActivitySourceOptions`, telemetry schema URLs on `ActivitySource`/`Meter` |
| Misc | `TimeSpan.FromMilliseconds(long)` single-argument overload (usable in expression trees), `ValidationContext(object, string, IServiceProvider?, IDictionary<object,object?>?)` (AOT-safe) |

Examples (verified output in comments):

```csharp
using System.Globalization;

var numeric = StringComparer.Create(CultureInfo.InvariantCulture, CompareOptions.NumericOrdering);
string[] versions = ["v10", "v9", "v100"];
Console.WriteLine(string.Join(",", versions.Order(numeric)));                // v9,v10,v100
Console.WriteLine(string.Join(",", versions.Order(StringComparer.Ordinal))); // v10,v100,v9
// NumericOrdering is rejected by IndexOf/StartsWith/EndsWith/IsPrefix/IsSuffix (ArgumentException).

var counts = new OrderedDictionary<string, int>();
foreach (var key in new[] { "b", "a", "b" })
    if (!counts.TryAdd(key, 1, out int index))
        counts.SetAt(index, counts.GetAt(index).Value + 1);   // single lookup per key
Console.WriteLine(string.Join(",", counts.Select(p => $"{p.Key}={p.Value}"))); // b=2,a=1

var left = new[] { (Id: 1, Name: "a"), (Id: 2, Name: "b") }
    .LeftJoin(new[] { (BlogId: 1, Title: "x") }, l => l.Id, r => r.BlogId,
              (l, r) => $"{l.Name}:{r.Title ?? "null"}");
Console.WriteLine(string.Join(",", left));                                    // a:x,b:null
Console.WriteLine(string.Join(",", Enumerable.Sequence(0, 10, 5)));           // 0,5,10
```

**Post-quantum crypto availability:** `MLKem.IsSupported`, `MLDsa.IsSupported` and `SlhDsa.IsSupported` require OpenSSL **3.5+** on Linux or Windows CNG with PQC support. Verified: on Ubuntu 24.04 with OpenSSL 3.0.13 all three return `false`. Code must check `IsSupported` and have a fallback; never assume PQC works because the API compiles.[^8]

##### JIT and GC improvements (no code change needed)

.NET 10 devirtualizes and inlines array interface methods (`foreach` over an array typed as `IEnumerable<T>`), stack-allocates small fixed-size arrays (of value and reference types) and non-escaping delegates, models struct fields in escape analysis, improves loop inversion and block layout, and adds a region-aware Arm64 write barrier (GC pause improvements of 8–20%+ in Microsoft's benchmarks). AVX10.2 intrinsics exist but are disabled by default.[^21] Do not "optimize" code by hand against these (e.g., rewriting all `IEnumerable<T>` parameters to arrays) without a benchmark.

##### SDK and tooling changes [New in .NET 10]

- `dotnet tool exec` / `dnx` run a tool package without installing it (downloads after a prompt) — treat it like `npx`: pin versions in CI (`dnx tool@1.2.3`).[^16]
- `dotnet new sln` creates **`.slnx`** files by default.[^9]
- **NuGet Audit includes transitive packages** (`dotnet restore` reports vulnerable transitive dependencies), and framework-provided package references are **pruned** (`RestoreEnablePackagePruning`); `PackageReference` without a version is an error (NU1015); NU1510 flags direct references that the framework already provides.[^9][^16]
- `dotnet test` can run on **Microsoft.Testing.Platform** natively (`"test": { "runner": "Microsoft.Testing.Platform" }` in `global.json`).[^16]
- Noun-first aliases: `dotnet package add|list|remove`, `dotnet reference add|list|remove`.
- Console apps can `dotnet publish /t:PublishContainer` without `EnableSdkContainerSupport`; `<ContainerImageFormat>` selects Docker or OCI.
- `--interactive` defaults to true in interactive terminals; CLI diagnostics go to stderr.
- `dnx` scripts bypass `global.json` SDK selection; `dotnet tool install --local` creates a tool manifest if none exists; NuGet audit sources reject plain HTTP; .NET tool packages can be RuntimeIdentifier-specific.[^9]
- `PackageReference` without a version fails restore with **NU1015**, and a direct reference to a package the framework already provides is pruned with **NU1510** (both verified on SDK 10.0.402).

##### ASP.NET Core 10 highlights [New in .NET 10]

- **Minimal API validation**: `builder.Services.AddValidation()` validates DataAnnotations on parameters and records (including nested types) and returns a 400 `application/problem+json` response; opt out per endpoint with `.DisableValidation()`. Without `AddValidation()` nothing is validated.[^10]
- **Cookie authentication returns 401/403 for API endpoints** (endpoints with `IApiEndpointMetadata`: `[ApiController]`, minimal APIs that read/write JSON or return `TypedResults`, SignalR) instead of redirecting to the login page. Verified: an unauthenticated request to a `TypedResults.Ok(...)` endpoint returns `401`, while an HTML endpoint still returns `302`.[^10][^22]
- **Server-Sent Events**: `TypedResults.ServerSentEvents(IAsyncEnumerable<T>, eventType)`.
- **OpenAPI 3.1 by default**, `Microsoft.OpenApi` 2.0 (`OpenApiAny` removed; use `JsonNode`), YAML endpoint (`MapOpenApi("/openapi/{documentName}.yaml")`).
- **`IExceptionHandler`**: when `TryHandleAsync` returns `true`, the exception handler middleware no longer emits diagnostics/logs for it (`ExceptionHandlerOptions.SuppressDiagnosticsCallback` restores them).[^22]
- Form binding: empty strings bind to `null` for nullable value types.
- JSON request bodies are read through `PipeReader`; custom `JsonConverter`s that ignore `Utf8JsonReader.HasValueSequence` can now fail.[^10]
- Passkeys (WebAuthn) in ASP.NET Core Identity; `*.localhost` binds to loopback.
- The validation APIs live in the `Microsoft.Extensions.Validation` package/namespace (usable outside HTTP); the resolver extensibility APIs are `[Experimental]`.
- JSON Patch has a System.Text.Json implementation (`Microsoft.AspNetCore.JsonPatch.SystemTextJson`) that is not a drop-in replacement for the Newtonsoft one (no `dynamic`/`ExpandoObject`); JSON Patch documents from clients must still be authorization-checked per path.[^10]
- A source generator emits `public partial class Program` for top-level-statement apps, so `WebApplicationFactory<Program>` works without the hand-written declaration (an analyzer flags the now-redundant one).
- The temporary escape hatch for converters broken by `PipeReader`-based parsing is the AppContext switch `Microsoft.AspNetCore.UseStreamBasedJsonParsing`; fix the converter instead.
- Deprecated: `IncludeOpenAPIAnalyzers` and the MVC API analyzers, the `Microsoft.Extensions.ApiDescription.Client` package.
- Obsoleted: `WebHostBuilder`/`IWebHost`/`WebHost`, `IActionContextAccessor`, `IPNetwork` and `ForwardedHeadersOptions.KnownNetworks` (use `KnownIPNetworks` with `System.Net.IPNetwork`), Razor runtime compilation, `WithOpenApi`.[^22]

##### EF Core 10 highlights [New in .NET 10]

- `LeftJoin`/`RightJoin` LINQ operators translate to `LEFT JOIN`/`RIGHT JOIN` (verified on SQLite).
- `ExecuteUpdateAsync` takes a regular `Action<UpdateSettersBuilder<T>>` lambda, so setters can be conditional with plain `if` statements (code that built expression trees by hand no longer compiles).[^23]
- Parameterized collections (`ids.Contains(x.Id)`) translate to **multiple scalar parameters** (`IN (@ids1, @ids2)`) instead of one JSON array parameter; tune with `UseParameterizedCollectionMode(ParameterTranslationMode.…)` or `EF.Parameter`/`EF.Constant`/`EF.MultipleParameters` per query.[^23]
- SQL parameter names are simplified (`@city` instead of `@__city_0`) — snapshot tests of SQL text break; a one-time plan-cache recompilation spike happens after deployment.[^23]
- SQL Server: the `json` data type is used for JSON columns with `UseAzureSql` or compatibility level ≥ 170 (an upgrade generates an `ALTER COLUMN` migration); complex types map to columns and JSON (`ComplexProperty(...).ToJson()`); named query filters; vector search.[^23][^24]
- Microsoft.Data.Sqlite 10: `GetDateTimeOffset` without offset assumes **UTC**; `GetDateTime` with an offset returns UTC; `DateTimeOffset` written to REAL columns is converted to UTC.[^23]
- `dotnet ef` requires `--framework` for multi-targeted projects.
- EF Core 10 injects an `Application Name` into SQL Server connection strings that lack one (can split connection pools or break login triggers keyed on it), and Microsoft.Data.SqlClient 6.1 pulls in `Microsoft.Identity.Client.Broker` (larger publish output).[^23]

---

#### Do Not Use or Recommend (C# 15 / .NET 11)

C# 15 and .NET 11 are **release candidates** (RC2, October 13, 2026); GA is expected in November 2026 under the annual release cadence (the RC2 announcement itself gives no date). A project on C# 14 / `net10.0` does not have any of the following, and the reviewer must never suggest them:[^5][^25][^26]

**C# 15 language features (not available in C# 14):**

- **Union types**: `public union Pet(Cat, Dog);` with exhaustive `switch` over case types (plus `System.Runtime.CompilerServices.UnionAttribute`/`IUnion`, .NET 11).
- **Closed hierarchies**: the `closed` modifier (`public closed record class GateState;`) and default-arm-free exhaustive switches over its derived types.
- **Collection expression arguments**: `List<string> names = [with(capacity: 8), .. values];`, `HashSet<string> set = [with(StringComparer.OrdinalIgnoreCase), ...];`.
- **Extension indexers**: `public int this[int i]` inside an `extension(...)` block.
- **Labeled `break`/`continue`**: `outer: for (...) { ... continue outer; }`.
- **Memory-safety evolution / pointer relaxations**: pointers, `&`, `fixed` and `sizeof` outside `unsafe`, `unsafe(expr)` expressions, the `safe` modifier (preview only even in .NET 11).
- Still in design and not part of C# 15: dictionary expressions, null-conditional `await`, extension constants, type-parameter inference from constraints.

**Verified finding about `LangVersion=preview`:** the Roslyn compiler in SDK **10.0.402** (5.9.0) already contains *preview* implementations of collection expression arguments, extension indexers and labeled `break`/`continue`. With `<LangVersion>preview</LangVersion>` on `net10.0` they compile and run; with `14` they fail with `CS8652: The feature '…' is currently in Preview and *unsupported*`. Unions and `closed` fail even with `preview` on `net10.0` because the runtime types (`IUnion`, `UnionAttribute`, `IsClosedTypeAttribute`) are missing. Treat `<LangVersion>preview</LangVersion>` (or `latest` combined with a newer SDK) in a production `net10.0` project as a finding: preview semantics can change before C# 15 GA and are unsupported on .NET 10.

**.NET 11 APIs (absent from the `net10.0` reference assemblies, verified):**

- `Enumerable.FullJoin`/`Queryable.FullJoin` (only `LeftJoin`/`RightJoin` exist in .NET 10), and the .NET 11 `LeftJoin`/`RightJoin` overloads **without** a result selector that return `(TOuter, TInner)` tuples — on .NET 10 a result selector is required.
- `JsonNamingPolicy.PascalCase`, `JsonSerializer.SerializeAsyncEnumerable`, `JsonUnionAttribute`, `JsonSerializerOptions.InferClosedTypePolymorphism`.
- `Process.Run`/`RunAsync`/`RunAndCaptureText`, `ProcessStartInfo.KillOnParentExit`, `SafeProcessHandle.Start`.
- `System.IO.ReadOnlyMemoryStream`, `WritableMemoryStream`, `StringStream`.
- `string.Contains(Rune)` and the other `Rune` overloads on `string`; `RegexOptions.AnyNewLine`.
- `System.Numerics.Decimal32`/`Decimal64`/`Decimal128`, `BitArray.PopCount()`, `Type.GetNullableUnderlyingType()`.
- `Dns.ResolveSrv`/`ResolveMx`/`ResolveTxt`/`ResolveCName`.
- `X25519DiffieHellman`, `CompositeMLKem` (experimental), the HPKE types (`Hpke`, `HpkeSender`, `HpkeRecipient`, `HpkeSuite`, …).
- `Microsoft.Extensions.*` 11.x features such as `ConfigurationIgnoreAttribute`, async DataAnnotations validation (`AsyncValidationAttribute`, `Validator.ValidateObjectAsync`), `IAsyncStartupValidator`.

Also do **not** tell a .NET 10 project that it can stay on .NET 8 or .NET 9 for long: both leave support on November 10, 2026.

---

#### Deprecated, Obsoleted and Removed APIs

New obsoletions in .NET 10 (custom diagnostic IDs; suppressing CS0618 does not suppress them):[^27]

| ID | API | Replacement |
| --- | --- | --- |
| SYSLIB0058 | `SslStream.KeyExchangeAlgorithm`, `KeyExchangeStrength`, `CipherAlgorithm`, `CipherStrength`, `HashAlgorithm`, `HashStrength` | `SslStream.NegotiatedCipherSuite` |
| SYSLIB0059 | `SystemEvents.EventsThreadShutdown` | `AppDomain.ProcessExit` |
| SYSLIB0060 | `Rfc2898DeriveBytes` **constructors** | `Rfc2898DeriveBytes.Pbkdf2(...)` (static, one-shot) |
| SYSLIB0061 | `Queryable.MaxBy/MinBy` overloads taking `IComparer<TSource>` | Overloads taking `IComparer<TKey>` |
| SYSLIB0062 | `XsltSettings.EnableScript` | No script support; do not enable XSLT scripts |

Still relevant from earlier releases (all apply to .NET 10):

- **`BinaryFormatter` is removed** (since .NET 9 its implementation always throws `PlatformNotSupportedException`); the same applies to `SoapFormatter`, `NetDataContractSerializer`, `LosFormatter`, `ObjectStateFormatter`. Replace with System.Text.Json, protobuf, or `System.Formats.Nrbf` for reading legacy payloads.[^28]
- `WebRequest`/`HttpWebRequest`/`WebClient` (SYSLIB0014) — use `HttpClient`.
- `Thread.Abort`, `Thread.Suspend`, CAS and `AppDomain` creation are unsupported.
- `SHA1Managed`/`*CryptoServiceProvider` constructors and `RNGCryptoServiceProvider` (SYSLIB0021/0023) — use `SHA256.HashData`, `RandomNumberGenerator`.
- ASP.NET Core 10: `WebHostBuilder`, `IWebHost`, `WebHost`, `IActionContextAccessor`, `IPNetwork`/`KnownNetworks`, Razor runtime compilation, `WithOpenApi()` are obsolete.[^22]

---

#### Version-Aware Review

- **Determine the pinned versions first**: `TargetFramework(s)`, `LangVersion`, `global.json` SDK (`rollForward`), `Directory.Build.props`, `Directory.Packages.props` (central package management), and the container base image tag. `net10.0` without `LangVersion` means C# 14.
- **Verify APIs against the pinned version before claiming something is missing or new.** Say "not available in .NET 10" only for APIs that are absent from the `net10.0` reference assemblies (the lists above were verified that way). Many APIs that look "new" exist since .NET 8/9: `Task.WhenEach`, `System.Threading.Lock`, `CountBy`, `AggregateBy`, `Index()`, `Guid.CreateVersion7()`, `Base64Url`, `SearchValues<string>`, `OrderedDictionary<TKey,TValue>`, `JsonSerializerOptions.Web`, `RespectNullableAnnotations`, `Convert.ToHexStringLower`, `CryptographicOperations.FixedTimeEquals`, `EqualityComparer<T>.Create`, `TimeProvider`, `FrozenDictionary`.
- **Multi-targeted libraries** (`net8.0;net10.0`) must only use .NET 10 APIs behind `#if NET10_0_OR_GREATER`. Each target framework gets its own default language version (`net8.0` → C# 12), so C# 14 syntax in shared code breaks the `net8.0` build unless `LangVersion` is raised explicitly — an unsupported combination that can also change overload resolution (see `Enumerable.Reverse`).
- **Package major versions follow the framework.** `Microsoft.Extensions.*`, `Microsoft.EntityFrameworkCore.*` and `Microsoft.AspNetCore.*` packages 10.x belong with `net10.0`; mixing 8.x and 10.x families causes downgrade warnings (NU1605) and runtime `MissingMethodException`s.
- **Patch level matters.** The October 2026 servicing release (10.0.13) fixes 44 CVEs, including remote code execution in ASP.NET Core and the SDK; 10.0.7 (April 21, 2026) was an out-of-band fix for **CVE-2026-40372**, an HMAC validation regression in `Microsoft.AspNetCore.DataProtection` 10.0.0–10.0.6 that let attackers forge protected payloads — rotate the key ring if an affected version was deployed.[^3][^29] Flag Dockerfiles or `global.json` that pin an older patch.

---

#### Project Setup, Build and Analyzers

##### Baseline project file

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <!-- LangVersion omitted: net10.0 implies C# 14. Never "preview" in production. -->
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <AnalysisLevel>latest-recommended</AnalysisLevel>
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
    <GenerateDocumentationFile>true</GenerateDocumentationFile>
    <Deterministic>true</Deterministic>
    <ContinuousIntegrationBuild Condition="'$(CI)' == 'true'">true</ContinuousIntegrationBuild>
  </PropertyGroup>
</Project>
```

Verified defaults on SDK 10.0.402 for `net10.0`: `LangVersion=14.0`, `AnalysisLevel=latest` (effective `10.0`), `EnableNETAnalyzers=true`, `TreatWarningsAsErrors=false`. For `net9.0` the same SDK gives `LangVersion=13.0`, effective analysis level `9.0`.

Review rules:

- `Nullable` must be `enable` for new code. `#nullable disable` regions and blanket `<NoWarn>CS86xx</NoWarn>` are findings unless justified.
- `TreatWarningsAsErrors` without `WarningsNotAsErrors` for obsoletion IDs means an SDK/package upgrade can break the build — acceptable, but CI must pin the SDK via `global.json`.
- `<AnalysisLevel>` values: `latest`, `preview`, a number (`10.0`), optionally suffixed with a mode: `-none`, `-default`, `-minimum`, `-recommended`, `-all` (e.g. `latest-recommended`). `AnalysisMode` alone sets the mode.[^30]
- `NoWarn` on security rules (`CA2100` SQL injection, `CA3075` insecure DTD, `CA5350`/`CA5351` weak crypto, `CA2300`-series deserialization) needs a written justification.
- Pin the SDK: `global.json` with `"rollForward": "latestPatch"` keeps builds reproducible within a feature band; `latestFeature` lets a machine silently use a newer band. Feature bands matter: SDK 10.0.1xx, 2xx, 3xx and 4xx ship different Roslyn versions (10.0.402 ships Roslyn 5.9.0), with new warnings and preview features.

```json
{
  "sdk": { "version": "10.0.402", "rollForward": "latestPatch" },
  "test": { "runner": "Microsoft.Testing.Platform" }
}
```

##### Central package management and supply chain

Use `Directory.Packages.props` with `<ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally>`; `PackageReference` items then carry no `Version`. In .NET 10, NuGet Audit scans **transitive** packages by default; keep `NuGetAuditMode=all` and do not suppress NU1901–NU1904 globally. Use `packages.lock.json` (`RestorePackagesWithLockFile`) plus `--locked-mode` in CI for reproducible restores, and `packageSourceMapping` in `nuget.config` to prevent dependency confusion.[^16]

##### .editorconfig essentials

```ini
root = true

[*.cs]
indent_style = space
indent_size = 4
csharp_style_namespace_declarations = file_scoped:warning
csharp_style_var_for_built_in_types = true:suggestion
dotnet_style_qualification_for_field = false:warning
dotnet_naming_rule.private_fields_underscore.symbols = private_fields
dotnet_naming_rule.private_fields_underscore.style = underscore_camel
dotnet_naming_rule.private_fields_underscore.severity = warning
dotnet_naming_symbols.private_fields.applicable_kinds = field
dotnet_naming_symbols.private_fields.applicable_accessibilities = private
dotnet_naming_style.underscore_camel.required_prefix = _
dotnet_naming_style.underscore_camel.capitalization = camel_case

# Correctness rules worth raising to errors
dotnet_diagnostic.CA2016.severity = warning   # forward CancellationToken
dotnet_diagnostic.CA1849.severity = warning   # call async methods in async methods
dotnet_diagnostic.CA2007.severity = none      # ConfigureAwait: enable only in libraries
dotnet_diagnostic.CA1848.severity = warning   # use LoggerMessage delegates
dotnet_diagnostic.CA2254.severity = warning   # logging template must be constant
dotnet_diagnostic.CS9258.severity = error     # C# 14 'field' keyword rebinding
```

`IDE*` style rules only fail the build with `EnforceCodeStyleInBuild=true`.

---

#### Naming and Code Style

These house rules apply to all C# code:

```csharp
// PascalCase for types, namespaces, methods, properties, events, constants
public interface IUserRepository { Task<User?> FindAsync(int id, CancellationToken ct = default); }
public sealed class UserService(IUserRepository repository)
{
    public const int MaxRetryCount = 3;            // constants: PascalCase, not SCREAMING_CASE
    private readonly IUserRepository _repository = repository;  // private fields: _camelCase
    private int _itemCount;

    public bool IsActive { get; private set; }     // booleans: Is/Has/Can prefix

    // Async methods end with Async and accept a CancellationToken last
    public async Task<User> GetUserAsync(int id, CancellationToken cancellationToken = default)
    {
        var user = await _repository.FindAsync(id, cancellationToken);
        return user ?? throw new KeyNotFoundException($"User {id} not found");
    }
}

public sealed record User(int Id, string Name);
```

- Interfaces start with `I`; type parameters start with `T` (`TKey`, `TResult`).
- One public type per file; file name matches the type name. File-scoped namespaces.
- Member order: constants, static fields, instance fields, constructors, properties, events, public methods, private methods.
- Seal classes that are not designed for inheritance (`sealed` also enables devirtualization).
- `var` when the type is obvious from the right-hand side; explicit types when it is not.
- No `this.` qualification except to disambiguate — and in C# 14 `this.field` is exactly such a case.
- Keep `async` all the way; do not suffix non-async methods with `Async`.
- Prefer expression-bodied members for one-liners; avoid nested ternaries.
- Solution layout: `src/` (Api, Core/Domain, Infrastructure), `tests/` (UnitTests, IntegrationTests), one `.slnx` at the root.

---

#### Nullable Reference Types

```csharp
#nullable enable
using System.Diagnostics.CodeAnalysis;

string? maybe = Environment.GetEnvironmentVariable("NOPE_NOT_SET");
// int len = maybe.Length;           // warning CS8602: dereference of a possibly null reference
int len = maybe?.Length ?? 0;
Console.WriteLine(len);              // 0

if (TryFind("a", out var value))
    Console.WriteLine(value.Length); // no warning thanks to [NotNullWhen(true)]

static bool TryFind(string key, [NotNullWhen(true)] out string? value)
{
    value = key == "a" ? "alpha" : null;
    return value is not null;
}
```

Rules:

- **`!` (null-forgiving) is a code smell.** Each use needs a reason; `= null!` on non-nullable properties is acceptable only for DI/serializer-populated members, and `required` is usually better.
- Annotate APIs with `[NotNullWhen]`, `[MaybeNullWhen]`, `[NotNull]`, `[MemberNotNull]`, `[NotNullIfNotNull]`, `[DoesNotReturn]` instead of suppressing warnings.
- Validate public inputs anyway: `ArgumentNullException.ThrowIfNull(arg)`, `ArgumentException.ThrowIfNullOrEmpty`/`ThrowIfNullOrWhiteSpace`, `ArgumentOutOfRangeException.ThrowIfNegative`/`ThrowIfGreaterThan`. Nullable annotations do not protect against callers with nullable disabled, reflection, or deserialization.
- **Deserializers ignore annotations by default.** System.Text.Json writes `null` into a non-nullable `string` property unless `RespectNullableAnnotations` (or `JsonSerializerOptions.Strict`) is enabled — see [System.Text.Json](#systemtextjson).
- **Configuration binding writes `null` in .NET 10** (see above). Options classes with non-nullable properties need validation.
- `default` of a struct containing reference-type fields has `null` in those fields even if they are declared non-nullable; arrays of non-nullable references (`new string[3]`) contain nulls.

```csharp
#nullable enable
var names = new string[2];                 // no warning, but both elements are null
Console.WriteLine(names[0] is null);       // True
var pair = default(Pair);
Console.WriteLine(pair.Name is null);      // True, despite 'string Name'

struct Pair { public string Name; }
```

---

#### Types: Classes, Records, Structs, Primary Constructors

##### Records

```csharp
var a = new Money(10m, "EUR");
var b = a with { Amount = 20m };          // non-destructive mutation
Console.WriteLine(a == new Money(10m, "EUR"));  // True: value equality
Console.WriteLine(b);                      // Money { Amount = 20, Currency = EUR }

var t1 = new Tagged("x", ["a"]);
var t2 = new Tagged("x", ["a"]);
Console.WriteLine(t1 == t2);               // False: arrays/lists compare by reference

public sealed record Money(decimal Amount, string Currency);
public sealed record Tagged(string Name, string[] Tags);
```

- Record equality is member-wise using `EqualityComparer<T>.Default`; collection members compare by **reference**. Records holding arrays/lists are rarely correct as dictionary keys or in `Distinct()`; use `ImmutableArray<T>` with a custom `Equals`, or compare explicitly.
- Positional records generate `init` properties; records are not deeply immutable if a property is a mutable collection.
- `record struct` is mutable by default (`set` accessors); write `readonly record struct` for value objects.
- `with` copies via the copy constructor: a record with a field that must be unique (an ID, a lock, a `CancellationTokenSource`) gets duplicated.
- Do not use records as EF Core entities that are tracked and mutated — value equality conflicts with identity tracking.

##### `required` and `init`

```csharp
var cfg = new SmtpSettings { Host = "smtp.example.test", Port = 587 };
Console.WriteLine(cfg);

public sealed record SmtpSettings
{
    public required string Host { get; init; }
    public int Port { get; init; } = 25;
}
```

`required` is enforced by the compiler for object creation (CS9035) and honored by System.Text.Json during deserialization (missing required property → `JsonException`). `[SetsRequiredMembers]` on a constructor disables the check — review such constructors carefully.

##### Primary constructors (C# 12) — pitfalls

Primary-constructor parameters of a class are **captured mutable state**, not readonly fields:

```csharp
var c = new Counter(5);
c.Increment();
Console.WriteLine(c.Value);   // 6: the parameter itself is mutated

public sealed class Counter(int start)
{
    public int Value => start;
    public void Increment() => start++;   // legal; there is no 'readonly' for captured parameters
}
```

- Capturing a parameter **and** assigning it to a field/property stores it twice (warning CS9124); the two copies can diverge.
- For DI services, prefer `private readonly IFoo _foo = foo;` when immutability matters, or use the parameter consistently and never assign to it.
- Primary-constructor parameters are not validated; add `ArgumentNullException.ThrowIfNull` via a field initializer when nulls are possible: `private readonly IFoo _foo = foo ?? throw new ArgumentNullException(nameof(foo));`.

##### Structs

- Make structs `readonly` when possible; mark methods `readonly` otherwise to avoid defensive copies on `in`/`readonly` fields.
- Keep structs small (≤ 16–24 bytes as a rule of thumb) — large structs are copied on every assignment and call.
- Never put a mutable struct in a `readonly` field or a `List<T>` and expect in-place mutation (`list[0].X = 1` does not compile; `GetAt(i).X++` on a copy is silently lost).
- Implement `IEquatable<T>` on structs used as dictionary keys; the default `ValueType.Equals` uses reflection for some layouts and boxes.

##### Classes and inheritance

- `sealed` by default; `virtual` only when designed for it.
- Do not call virtual members from constructors.
- Override `Equals` and `GetHashCode` together (`HashCode.Combine`), and `==`/`!=` with them.
- `ToString()` must not throw and must not leak secrets (records print all public properties, including `Password`!). Override `PrintMembers` or exclude sensitive members.

```csharp
var login = new Login("admin", "hunter2");
Console.WriteLine(login);   // Login { User = admin }

public sealed record Login(string User, string Password)
{
    private bool PrintMembers(System.Text.StringBuilder builder)
    {
        builder.Append("User = ").Append(User);
        return true;
    }
}
```

---

#### Pattern Matching

```csharp
Console.WriteLine(Discount(new Premium(1000m)));          // 200.0
Console.WriteLine(Shipping(new Order(150m, true)));       // Express
Console.WriteLine(Classify([1, 2, 3]));                   // starts with 1, ends with 3

static decimal Discount(Customer customer) => customer switch
{
    Premium p => p.Total * 0.2m,
    Regular { Years: > 5 } r => r.Total * 0.1m,
    Regular r => r.Total * 0.05m,
    _ => throw new ArgumentOutOfRangeException(nameof(customer))
};

static string Shipping(Order order) => order switch
{
    { Total: > 100, IsPriority: true } => "Express",
    { Total: > 100 } => "Standard",
    { IsPriority: true } => "Priority",
    _ => "Economy",
};

static string Classify(int[] xs) => xs switch
{
    [] => "empty",
    [var only] => $"one: {only}",
    [1, .., 3] => "starts with 1, ends with 3",
    [_, _, ..] => "two or more",
};

abstract record Customer(decimal Total);
sealed record Premium(decimal Total) : Customer(Total);
sealed record Regular(decimal Total, int Years) : Customer(Total);
record Order(decimal Total, bool IsPriority);
```

Review points:

- **Precedence of `not`/`and`/`or`**: `x is not null or ""` is `(not null) or ""` — always true for non-null `x`. C# 14 warns (CS9336) for the simple cases; parenthesize: `x is not (null or "")`.
- `switch` expressions without a discard arm throw `SwitchExpressionException` at run time for unmatched inputs (warning CS8509 at compile time — do not suppress it). In C# 14 class hierarchies are never considered exhaustive; closed hierarchies are C# 15.
- `is` patterns on `double` use `Equals` semantics: `double.NaN is double.NaN` is `true` while `double.NaN == double.NaN` is `false`.
- Prefer `is null`/`is not null` over `== null` (user-defined `==` operators can lie).

---

#### Collections, Collection Expressions, Span and Memory

##### Collection expressions (C# 12) and `params` collections (C# 13)

```csharp
using System.Collections.Frozen;
using System.Collections.Immutable;

int[] a = [1, 2, 3];
List<int> list = [.. a, 4];
ImmutableArray<int> imm = [1, 2];
ReadOnlySpan<int> span = [1, 2, 3];          // no heap allocation for constant data
IEnumerable<int> seq = [];                    // empty: no allocation (Array.Empty-like singleton)
Console.WriteLine($"{list.Count} {imm.Length} {span.Length} {Sum(1, 2, 3)}");  // 4 2 3 6

FrozenDictionary<string, int> codes = new Dictionary<string, int> { ["a"] = 1 }.ToFrozenDictionary();
Console.WriteLine(codes["a"]);               // 1

static int Sum(params ReadOnlySpan<int> xs) { var s = 0; foreach (var x in xs) s += x; return s; }
```

- **Collection expression arguments (`[with(capacity: n), ...]`) are C# 15 — not available.** Use `new List<T>(capacity)` followed by `AddRange`, or `new HashSet<string>(comparer) { ... }` when you need a capacity or comparer.
- Target type decides the concrete type: `IEnumerable<T> x = [1, 2]` creates a compiler-synthesized read-only type; `IList<T>`/`ICollection<T>` targets get a `List<T>`. Do not downcast them.
- `params ReadOnlySpan<T>` avoids the array allocation of `params T[]`; it is the preferred signature for new APIs (C# 13).
- Use `FrozenDictionary`/`FrozenSet` (`System.Collections.Frozen`) for read-mostly lookup tables built once at startup; creation is expensive, lookups are fast.
- `Dictionary<string, T>` from untrusted keys: specify `StringComparer.Ordinal` or `OrdinalIgnoreCase` explicitly; never a culture-sensitive comparer for identifiers.
- `ConcurrentDictionary.GetOrAdd(key, factory)` can invoke the factory more than once under contention; wrap expensive/side-effecting factories in `Lazy<T>`.
- Modifying a `List<T>`/`Dictionary<TKey,TValue>` while enumerating it throws `InvalidOperationException` (`Dictionary.Remove` during enumeration is the one allowed mutation since .NET Core 3.0).

##### Span<T> and Memory<T>

```csharp
using System.Buffers;
using System.Text;

ReadOnlySpan<char> line = "key=value;other=x";
int eq = line.IndexOf('=');
ReadOnlySpan<char> key = line[..eq];
Console.WriteLine(key.SequenceEqual("key"));              // True, no substring allocation

Span<byte> small = stackalloc byte[64];                   // small, bounded sizes only
int written = Encoding.UTF8.GetBytes("héllo", small);
Console.WriteLine(written);                               // 6

byte[] rented = ArrayPool<byte>.Shared.Rent(4096);
try
{
    // use rented.AsSpan(0, needed) — the array may be larger than requested
}
finally
{
    ArrayPool<byte>.Shared.Return(rented, clearArray: true);   // clear if it held secrets
}

var splitCount = 0;
foreach (Range r in "a,b,,c".AsSpan().Split(','))         // .NET 9+ allocation-free split
    splitCount++;
Console.WriteLine(splitCount);                            // 4
```

Rules:

- `Span<T>`/`ReadOnlySpan<T>` are `ref struct`s: they cannot be fields of classes, captured by lambdas, or used across `await` (C# 13 allows `ref struct` locals in async methods and iterators as long as they are not live across `await`/`yield`).
- Use `Memory<T>`/`ReadOnlyMemory<T>` for async APIs and storage.
- `stackalloc` only with a small, constant upper bound; `stackalloc byte[userControlledLength]` is a stack-overflow DoS. Pattern: `Span<byte> buf = len <= 256 ? stackalloc byte[256] : new byte[len];`.
- Never use a rented array after returning it; never return it twice; never keep a `Span` over a pooled array beyond the `Return`.
- `MemoryMarshal`/`Unsafe` APIs bypass safety; require justification and tests.
- C# 14 first-class spans mean array arguments may bind to span overloads (see above).

---

#### LINQ

```csharp
int calls = 0;
IEnumerable<int> Numbers() { for (int i = 0; i < 3; i++) { calls++; yield return i; } }

var q = Numbers().Where(n => n > 0);   // deferred: nothing executed yet
Console.WriteLine(calls);              // 0
_ = q.Count();
_ = q.Any();                           // enumerates again
Console.WriteLine(calls);              // 5 (3 + 2): multiple enumeration

var threshold = 0;
var lazy = new[] { 1, 2, 3 }.Where(x => x > threshold);
threshold = 2;                         // captured variable changes the query result
Console.WriteLine(string.Join(",", lazy));   // 3

var materialized = Numbers().Where(n => n > 0).ToList();   // enumerate once
Console.WriteLine(materialized.Count); // property, not Count()
```

Review rules:

- **Multiple enumeration** of an `IEnumerable<T>` (CA1851) re-runs the producer — a database query, an HTTP call, an iterator with side effects. Materialize once with `ToList()`/`ToArray()` when the sequence is used more than once.
- `Any()` over `Count() > 0` for lazy sequences; `Count`/`Length` properties for materialized collections. `TryGetNonEnumeratedCount` avoids enumeration when possible.
- `First`/`Single` when absence is a bug (fail fast); `FirstOrDefault`/`SingleOrDefault` only when absence is expected and handled — and remember they return `default` (0 for `int`) for value types. `Single` scans the whole sequence to verify uniqueness.
- `OrderBy(...).OrderBy(...)` discards the first ordering; use `ThenBy`.
- `Distinct`, `GroupBy`, `ToDictionary`, `ToHashSet` use the default equality comparer — pass `StringComparer.OrdinalIgnoreCase` explicitly where needed. `ToDictionary` throws on duplicate keys.
- `Select` with side effects, or `ToList().ForEach(...)` for side effects, hides intent — use `foreach`.
- LINQ in hot paths allocates enumerators and delegates; measure before rewriting, the .NET 10 JIT removes much of the overhead for arrays.
- New in .NET 10: `LeftJoin`, `RightJoin`, `Shuffle`, `Sequence`, `InfiniteSequence` and the in-box `System.Linq.AsyncEnumerable`. From .NET 9: `CountBy`, `AggregateBy`, `Index`. `FullJoin` is .NET 11 — not available.
- `Enumerable.Shuffle` is not a cryptographically secure shuffle — use `RandomNumberGenerator.Shuffle` (on a span) for security-sensitive cases (lotteries, card games, sampling for audits).

`IQueryable<T>` rules are in [EF Core 10](#ef-core-10): the same LINQ code means "run in the database" there, and calling a method the provider cannot translate throws at run time.

---

#### Exceptions and Error Handling

```csharp
try
{
    Parse("x");
}
catch (FormatException ex) when (ex.Data.Contains("input"))   // filter: stack not unwound before match
{
    Console.WriteLine($"bad input: {ex.Data["input"]}");
}

static int Parse(string s)
{
    try
    {
        return int.Parse(s);
    }
    catch (FormatException ex)
    {
        ex.Data["input"] = s;
        throw;                     // rethrow preserves the stack trace; 'throw ex;' resets it (CA2200)
    }
}
```

Rules:

- `throw;` not `throw ex;`. Wrap with context using `throw new DomainException("...", ex)` (inner exception preserved).
- Catch specific exceptions. `catch (Exception)` only at boundaries (request pipeline, background loop, `Main`) and always log it.
- **Never swallow `OperationCanceledException` as an error** and never log it at `Error` level when the cancellation was requested; check `when (!ct.IsCancellationRequested)` for real failures.
- Do not use exceptions for control flow in hot paths; provide `TryXxx` patterns (`int.TryParse`, `Dictionary.TryGetValue`).
- Throw helpers: `ArgumentNullException.ThrowIfNull`, `ArgumentException.ThrowIfNullOrEmpty`, `ArgumentOutOfRangeException.ThrowIfNegativeOrZero`, `ObjectDisposedException.ThrowIf(_disposed, this)`.
- `finally` and `using` run on exceptions; but code after `Environment.FailFast`, process kills and (since .NET 10, without a host) SIGTERM does not.
- Exception messages must not contain secrets, connection strings or full user input that will end up in logs.
- In ASP.NET Core, map exceptions to RFC 9457 problem details (`AddProblemDetails()`, `UseExceptionHandler()`, `IExceptionHandler`); never return `ex.ToString()` to clients.

---
#### Async/Await, Cancellation and Concurrency

##### Core rules

```csharp
using System.Diagnostics;

using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(5));
var sw = Stopwatch.StartNew();

// Start independent work, then await together
Task<int> a = LoadAsync(1, cts.Token);
Task<int> b = LoadAsync(2, cts.Token);
int[] results = await Task.WhenAll(a, b);
Console.WriteLine($"{results.Sum()} in ~{sw.ElapsedMilliseconds / 100 * 100} ms");   // 3 in ~200 ms (parallel, not 400)

// Process results as they complete (.NET 9+)
await foreach (Task<int> done in Task.WhenEach(LoadAsync(3, cts.Token), LoadAsync(4, cts.Token)))
    Console.WriteLine(await done);

static async Task<int> LoadAsync(int id, CancellationToken ct)
{
    await Task.Delay(200, ct);   // always pass the token down (CA2016)
    return id;
}
```

- **Never block on async code**: `.Result`, `.Wait()`, `.GetAwaiter().GetResult()` in request paths cause thread-pool starvation and, under a `SynchronizationContext` (WinForms/WPF/legacy ASP.NET), deadlocks. Async all the way up to `Main`/the endpoint.
- **No `async void`** except event handlers. An exception escaping an `async void` method is raised on the thread pool and **crashes the process**; it cannot be caught by the caller. Event handlers must wrap their body in `try/catch`.
- Do not "fire and forget" `Task`s (`_ = DoAsync();`) in services: exceptions go unobserved, the work outlives the request scope (disposed `DbContext`), and shutdown does not wait. Use a `Channel<T>` + `BackgroundService`, or a hosted queue.
- **`ConfigureAwait(false)`** in general-purpose libraries (code that may run under a UI or legacy `SynchronizationContext`). ASP.NET Core has no `SynchronizationContext`, so application code there does not need it. Do not mix: once a library uses it, use it on every `await`. `await using` and `await foreach` take `.ConfigureAwait(false)` too.
- Return `Task` directly (no `async`/`await`) only for trivial pass-through methods without `using`/`try`; eliding `await` inside a `using` disposes the resource before the task completes.
- `Task.WhenAll` throws only the **first** exception when awaited; inspect `task.Exception.InnerExceptions` (or each task) for the rest.
- `Task.Run` in ASP.NET Core request code only moves work to another pool thread — it does not add capacity. Use it for CPU-bound work in UI apps or to offload a truly blocking call.
- `ValueTask`/`ValueTask<T>`: await exactly once; never `await` twice, never call `.Result` before completion, never `WhenAll` them without `AsTask()`.
- `Thread.Sleep` in async code blocks a pool thread; use `await Task.Delay(…, ct)`.
- Use `TimeProvider` (`TimeProvider.System`, `FakeTimeProvider` in tests) instead of `DateTime.UtcNow`/`Task.Delay` when time must be testable.

##### `async void` crashes the process

```csharp
// BAD — compile-only illustration; this would terminate the process:
// button.Click += async (_, _) => await SaveAsync();     // exception -> unhandled -> crash
// GOOD:
EventHandler handler = async (_, _) =>
{
    try { await SaveAsync(); }
    catch (Exception ex) { Console.WriteLine($"save failed: {ex.Message}"); }
};
handler(null, EventArgs.Empty);
await Task.Delay(100);           // let the handler finish for this demo

static async Task SaveAsync() { await Task.Yield(); throw new IOException("disk full"); }
```

##### Cancellation

```csharp
using var cts = CancellationTokenSource.CreateLinkedTokenSource(CancellationToken.None);
cts.CancelAfter(TimeSpan.FromMilliseconds(100));
try
{
    await Task.Delay(Timeout.InfiniteTimeSpan, cts.Token);
}
catch (OperationCanceledException) when (cts.IsCancellationRequested)
{
    Console.WriteLine("cancelled as requested");   // expected path, not an error
}
```

- Every public async API takes `CancellationToken cancellationToken = default` as the **last** parameter and passes it to every async call (CA2016).
- In ASP.NET Core, bind `CancellationToken` in the handler (it is `HttpContext.RequestAborted`); in hosted services use the `stoppingToken`.
- Combine tokens with `CancellationTokenSource.CreateLinkedTokenSource(a, b)` and **dispose** the linked source (it registers callbacks on the parents; not disposing leaks them for long-lived parent tokens).
- `CancellationTokenSource` with `CancelAfter` must be disposed; `cts.Token` after dispose throws `ObjectDisposedException`.
- Check `ct.ThrowIfCancellationRequested()` in CPU loops.
- Throw `OperationCanceledException` (not `TaskCanceledException` specifically) and catch `OperationCanceledException` (it is the base type).

##### IAsyncEnumerable<T>

```csharp
using System.Runtime.CompilerServices;

using var cts = new CancellationTokenSource();
try
{
    await foreach (var n in ReadAsync().WithCancellation(cts.Token))
    {
        Console.WriteLine(n);             // 1, 2
        if (n == 2) cts.Cancel();         // the token flows into the iterator via [EnumeratorCancellation]
    }
}
catch (OperationCanceledException) when (cts.IsCancellationRequested)
{
    Console.WriteLine("stopped");
}

static async IAsyncEnumerable<int> ReadAsync([EnumeratorCancellation] CancellationToken ct = default)
{
    for (int i = 1; ; i++)
    {
        ct.ThrowIfCancellationRequested();
        await Task.Delay(10, ct);
        yield return i;
    }
}
```

Without `[EnumeratorCancellation]`, `WithCancellation(token)` cannot reach the iterator body (warning CS8425). After the cancel, the next `MoveNextAsync` throws `OperationCanceledException`, so the loop needs the `catch` shown when cancellation is an expected outcome. In .NET 10 LINQ over `IAsyncEnumerable<T>` is built in (`System.Linq.AsyncEnumerable`): `await source.Where(x => x > 1).ToListAsync(ct)`.

##### Synchronization primitives

```csharp
var counter = new SafeCounter();
await Parallel.ForEachAsync(Enumerable.Range(0, 1000), new ParallelOptions { MaxDegreeOfParallelism = 8 },
    (_, _) => { counter.Increment(); return ValueTask.CompletedTask; });
Console.WriteLine(counter.Value);    // 1000

var gate = new SemaphoreSlim(3);     // async-compatible throttle
await gate.WaitAsync();
try { /* at most 3 concurrent callers */ }
finally { gate.Release(); }

sealed class SafeCounter
{
    private readonly Lock _lock = new();   // System.Threading.Lock (.NET 9 / C# 13)
    private int _value;
    public void Increment() { lock (_lock) { _value++; } }
    public int Value { get { lock (_lock) { return _value; } } }
}
```

- `lock` on a `System.Threading.Lock` uses the new, faster lock; do not cast it to `object` (that falls back to `Monitor` and the compiler warns, CS9216).
- **`await` inside `lock` does not compile**; use `SemaphoreSlim.WaitAsync` for async critical sections, always released in `finally`.
- Never `lock (this)`, `lock (typeof(T))` or on strings.
- `Interlocked` for counters; `ConcurrentDictionary`, `Channel<T>` for producer/consumer; `ImmutableInterlocked` or copy-on-write for read-mostly state.
- Static mutable state (including `static Dictionary` caches) in web apps is shared across requests and threads.
- `Parallel.ForEach` with async lambdas is a bug (`async void` delegates); use `Parallel.ForEachAsync`.
- `SemaphoreSlim` around `Task.WhenAll(items.Select(...))` starts all tasks immediately (they wait on the semaphore) — fine for moderate counts; for very large inputs prefer `Parallel.ForEachAsync` or a `Channel`.

##### Channels for background work

```csharp
using System.Threading.Channels;

var channel = Channel.CreateBounded<int>(new BoundedChannelOptions(100) { FullMode = BoundedChannelFullMode.Wait });
var consumer = Task.Run(async () =>
{
    await foreach (var item in channel.Reader.ReadAllAsync())
        Console.WriteLine($"processed {item}");
});
for (int i = 0; i < 3; i++) await channel.Writer.WriteAsync(i);
channel.Writer.Complete();
await consumer;
```

Bounded channels apply back-pressure; unbounded channels from untrusted producers are a memory DoS.

---

#### IDisposable and IAsyncDisposable

```csharp
await using (var resource = new AsyncResource())
{
    await resource.WorkAsync();
}   // DisposeAsync awaited here

using var stream = new MemoryStream();   // using declaration: disposed at end of scope

sealed class AsyncResource : IAsyncDisposable
{
    private readonly MemoryStream _stream = new();
    private bool _disposed;

    public async Task WorkAsync()
    {
        ObjectDisposedException.ThrowIf(_disposed, this);
        await _stream.WriteAsync("x"u8.ToArray());
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed) return;
        _disposed = true;
        await _stream.DisposeAsync();
        Console.WriteLine("disposed");
    }
}
```

- Every `IDisposable` you create and own must be disposed (`using`), including `CancellationTokenSource`, `HttpResponseMessage`, `Stream`, `Timer`, `SemaphoreSlim` (when `AvailableWaitHandle` was used), `RSA`/`Aes`/`HMAC` instances, `JsonDocument`.
- Do **not** dispose what you do not own: DI-injected services (the container disposes them), `HttpClient` from `IHttpClientFactory` (harmless, but pointless), streams passed in by the caller (unless documented, e.g. `leaveOpen: false`).
- Use `await using` for `IAsyncDisposable` types (`DbContext`, `Utf8JsonWriter` over async streams, `ServiceProvider` with async-disposable services). Disposing a `ServiceProvider` synchronously that holds `IAsyncDisposable`-only services throws `InvalidOperationException`.
- The full `Dispose(bool)` pattern plus finalizer is only needed for types that directly own **unmanaged** handles — and those should use `SafeHandle` instead. A sealed class with managed resources needs just `Dispose()`.
- `JsonDocument` rents pooled memory: `using var doc = JsonDocument.Parse(json);` and do not let `JsonElement`s escape the `using` (clone with `element.Clone()` if needed).

---

#### Dependency Injection, Options, Configuration and Hosting

##### Lifetimes and captive dependencies

```csharp
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddScoped<IUnitOfWork, UnitOfWork>();
services.AddSingleton<ReportCache>();               // depends on a scoped service: captive dependency!

try
{
    using var root = services.BuildServiceProvider(new ServiceProviderOptions { ValidateScopes = true, ValidateOnBuild = true });
}
catch (AggregateException ex)    // ValidateOnBuild reports all invalid registrations at build time
{
    Console.WriteLine(ex.InnerException?.InnerException?.Message);
    // Cannot consume scoped service 'IUnitOfWork' from singleton 'ReportCache'.
}

interface IUnitOfWork;
sealed class UnitOfWork : IUnitOfWork;
sealed class ReportCache(IUnitOfWork uow);
```

- **Singleton → scoped** (e.g., a singleton holding a `DbContext`) is a captive dependency: the scoped instance lives forever and is shared across threads. ASP.NET Core enables `ValidateScopes`/`ValidateOnBuild` in Development only — run integration tests in Development or enable them explicitly.
- Resolve scoped services in singletons/background services through `IServiceScopeFactory.CreateAsyncScope()`.
- `services.BuildServiceProvider()` inside `ConfigureServices`/registration code creates a second container (duplicate singletons) — flagged by ASP0000.
- Transient `IDisposable`s resolved from the root provider are kept alive until the root is disposed (memory leak). Resolve them from a scope.
- Keyed services (.NET 8+): `AddKeyedSingleton<IStore, S3Store>("s3")` + `[FromKeyedServices("s3")]`. .NET 10 fixed `KeyedService.AnyKey` behavior in `GetKeyedService`/`GetKeyedServices`.[^9]
- Do not inject `IServiceProvider` into domain classes (service locator); inject what you need.

##### Options pattern

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Options;

// Worker SDK implicit usings (Microsoft.Extensions.Configuration/Hosting); ValidateDataAnnotations needs
// the Microsoft.Extensions.Options.DataAnnotations package (not part of Microsoft.Extensions.Hosting).
var builder = Host.CreateApplicationBuilder();
builder.Configuration.AddInMemoryCollection(new Dictionary<string, string?> { ["Email:Port"] = "70000" });
builder.Services.AddOptions<EmailOptions>()
    .Bind(builder.Configuration.GetSection(EmailOptions.Section))
    .ValidateDataAnnotations()
    .ValidateOnStart();                 // fail at startup, not on first use

using var host = builder.Build();
try
{
    await host.StartAsync();
}
catch (OptionsValidationException ex)
{
    Console.WriteLine(ex.Failures.Count());   // 2 (SmtpServer required, Port out of range)
}

public sealed class EmailOptions
{
    public const string Section = "Email";
    [Required] public string? SmtpServer { get; set; }
    [Range(1, 65535)] public int Port { get; set; } = 587;
}
```

- `IOptions<T>` is a singleton snapshot; `IOptionsSnapshot<T>` is scoped (re-read per request); `IOptionsMonitor<T>` supports change notifications in singletons.
- Without `ValidateOnStart`, invalid configuration surfaces on the first request that reads `.Value`.
- **.NET 10 binds explicit `null` values** (JSON provider no longer converts them to `""`), and `null` into non-nullable value types becomes `default`. Validate.
- Never put secrets in `appsettings.json` committed to source control; use environment variables, user secrets (development only), Key Vault/secret stores. Do not log bound options objects that contain secrets.
- Configuration source generator (`<EnableConfigurationBindingGenerator>true</EnableConfigurationBindingGenerator>`) makes binding trim/AOT-safe; verified on SDK 10.0.402 to be on by default when `PublishAot` or `PublishTrimmed` is set.

##### Hosting and background services

```csharp
var builder = Host.CreateApplicationBuilder();
builder.Services.AddHostedService<QueueWorker>();
builder.Services.Configure<HostOptions>(o =>
{
    o.ShutdownTimeout = TimeSpan.FromSeconds(20);
    o.BackgroundServiceExceptionBehavior = BackgroundServiceExceptionBehavior.StopHost;   // default since .NET 6
});
using var host = builder.Build();
await host.StartAsync();
await Task.Delay(150);
await host.StopAsync();

sealed class QueueWorker(ILogger<QueueWorker> logger) : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        // .NET 10: this whole method, including code before the first await, runs on a pool thread.
        using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(50));
        try
        {
            while (await timer.WaitForNextTickAsync(stoppingToken))
            {
                try
                {
                    Console.WriteLine("tick");
                }
                catch (Exception ex)                       // keep the loop alive for transient failures
                {
                    logger.LogError(ex, "Tick failed");
                }
            }
        }
        catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
        {
            // normal shutdown
        }
    }
}
```

- An unhandled exception in `ExecuteAsync` stops the host by default (`StopHost`) — intended; do not switch to `Ignore` to hide failures.
- Use `PeriodicTimer` instead of `System.Timers.Timer`/`Thread.Sleep` loops; it does not overlap ticks and honors cancellation.
- Kubernetes sends SIGTERM and waits `terminationGracePeriodSeconds` (30 s default); keep `HostOptions.ShutdownTimeout` below it.
- `IHostedLifecycleService` (`StartingAsync`, `StartedAsync`, `StoppingAsync`, `StoppedAsync`) for precise startup ordering.

---

#### Logging and Observability

##### Source-generated logging

```csharp
using Microsoft.Extensions.Logging;

using var factory = LoggerFactory.Create(b => b.AddSimpleConsole(o => o.SingleLine = true).SetMinimumLevel(LogLevel.Debug));
var logger = factory.CreateLogger("Orders");

Log.OrderCreated(logger, 42, "EUR");
logger.LogInformation("Order {OrderId} shipped", 42);      // OK: constant template, structured
// logger.LogInformation($"Order {id} shipped");           // BAD: CA2254 — interpolation defeats structured logging & allocates

static partial class Log
{
    [LoggerMessage(EventId = 1001, Level = LogLevel.Information, Message = "Order {OrderId} created in {Currency}")]
    public static partial void OrderCreated(ILogger logger, int orderId, string currency);
}
```

- Use `[LoggerMessage]` source generation for hot paths (CA1848): no boxing, no template parsing, level check before evaluation.
- Message templates are constant; use PascalCase placeholders; do not use string interpolation in log calls (CA2254).
- Pass the exception as the first argument (`logger.LogError(ex, "...")`), never `ex.Message` only.
- Never log secrets, tokens, passwords, full request bodies or PII. `[LogProperties]`/redaction (`Microsoft.Extensions.Compliance.Redaction`) for structured objects.
- Expensive arguments: guard with `if (logger.IsEnabled(LogLevel.Debug))` when not using the generator.
- .NET 10: the JSON console formatter no longer duplicates the message in the output.[^9]

##### Metrics and tracing

- Use `System.Diagnostics.Metrics.Meter` (via `IMeterFactory` from DI) and `ActivitySource`; export with OpenTelemetry. .NET 10 adds `ActivitySourceOptions` and telemetry schema URLs.[^8]
- Name meters/sources after the component (`"Contoso.Orders"`), use OpenTelemetry semantic conventions for tags, keep tag cardinality bounded (no user IDs or URLs with IDs as tag values).
- ASP.NET Core 10 emits new metrics for authentication/authorization, Identity and memory pools.[^10]

---

#### HttpClient and IHttpClientFactory

```csharp
using System.Net.Http.Json;
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddHttpClient<GitHubClient>(client =>
{
    client.BaseAddress = new Uri("https://api.github.com/");
    client.Timeout = TimeSpan.FromSeconds(10);
    client.DefaultRequestHeaders.UserAgent.ParseAdd("contoso-app/1.0");
});
using var sp = services.BuildServiceProvider();
var gh = sp.GetRequiredService<GitHubClient>();
Console.WriteLine(gh.BaseAddress);          // https://api.github.com/

sealed class GitHubClient(HttpClient http)
{
    public Uri? BaseAddress => http.BaseAddress;

    public async Task<Repo?> GetRepoAsync(string owner, string name, CancellationToken ct)
    {
        using var response = await http.GetAsync($"repos/{Uri.EscapeDataString(owner)}/{Uri.EscapeDataString(name)}", ct);
        if (response.StatusCode == System.Net.HttpStatusCode.NotFound) return null;
        response.EnsureSuccessStatusCode();
        return await response.Content.ReadFromJsonAsync<Repo>(ct);
    }
}

sealed record Repo(string Name, int StargazersCount);
```

- **Never `new HttpClient()` per request** (socket exhaustion, `TIME_WAIT`); never a single static `HttpClient` without `SocketsHttpHandler.PooledConnectionLifetime` (stale DNS). Use `IHttpClientFactory` (named/typed clients) or one long-lived `HttpClient` over a `SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(2) }`.
- **Typed clients must not be captured by singletons** — the factory rotates handlers; a typed client injected into a singleton keeps one handler forever. Inject `IHttpClientFactory` into singletons instead.
- Set `Timeout` (default 100 s) and pass a `CancellationToken`. `TaskCanceledException` from `HttpClient` means timeout *or* cancellation; check `ct.IsCancellationRequested`.
- Dispose `HttpResponseMessage` (or use `GetFromJsonAsync`); use `HttpCompletionOption.ResponseHeadersRead` for large downloads and stream the content.
- Escape path segments (`Uri.EscapeDataString`) and build query strings with `QueryHelpers`/`QueryString`, never by concatenating user input.
- Do not disable certificate validation (`ServerCertificateCustomValidationCallback = (_, _, _, _) => true` or `DangerousAcceptAnyServerCertificateValidator`) outside tests.
- Resilience: `Microsoft.Extensions.Http.Resilience` (`AddStandardResilienceHandler()`) — retries only for idempotent requests; do not stack it with hand-written Polly retries.
- Forwarding user-supplied URLs to `HttpClient` is SSRF: validate scheme/host against an allow-list and block private/link-local addresses after DNS resolution.

---

#### System.Text.Json

##### Defaults that surprise reviewers (verified on .NET 10)

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

string dup = """{ "Value": 1, "Value": -1 }""";
Console.WriteLine(JsonSerializer.Deserialize<Rec>(dup)!.Value);                         // -1: last duplicate wins
Console.WriteLine(JsonSerializer.Deserialize<Rec>("""{"value":3}""")!.Value);           // 0: case-sensitive by default
Console.WriteLine(JsonSerializer.Deserialize<Rec>("""{"value":3}""", JsonSerializerOptions.Web)!.Value); // 3
Console.WriteLine(JsonSerializer.Deserialize<Rec>("""{"Value":3,"Extra":2}""")!.Value); // 3: unknown members ignored
Console.WriteLine(JsonSerializer.Deserialize<Rec>("{}")!.Value);                        // 0: missing ctor parameter -> default
Console.WriteLine(JsonSerializer.Deserialize<Person>("""{"Name":null}""")!.Name is null); // True: null written into non-nullable string

// .NET 10: opt into strict parsing
try { JsonSerializer.Deserialize<Rec>(dup, new JsonSerializerOptions { AllowDuplicateProperties = false }); }
catch (JsonException) { Console.WriteLine("duplicate rejected"); }
try { JsonSerializer.Deserialize<Person>("""{"Name":null}""", JsonSerializerOptions.Strict); }
catch (JsonException) { Console.WriteLine("Strict: null for non-nullable rejected"); }

record Rec(int Value);
record Person(string Name);
```

- `JsonSerializerOptions.Strict` (new in .NET 10) = `UnmappedMemberHandling.Disallow` + `AllowDuplicateProperties = false` + case-sensitive + `RespectNullableAnnotations = true` + `RespectRequiredConstructorParameters = true`. Use it (or the individual options) for **security-relevant input**: duplicate keys are a classic parser-differential attack (one component validates the first `"role"`, another uses the last).[^8]
- `JsonSerializerOptions.Web` (and ASP.NET Core) use camelCase, case-insensitive matching and `JsonNumberHandling.AllowReadingFromString`.
- **Cache `JsonSerializerOptions` instances** (static readonly or DI). Creating new options per call rebuilds metadata caches — a severe performance bug (CA1869).
- **Metadata name conflicts throw in .NET 10**: a polymorphic type whose own property is named like the discriminator (`"Type"`, `$type`, `$id`, `$ref`) throws `InvalidOperationException` on first use; .NET 9 silently wrote invalid JSON with a duplicate key (verified: `{"Type":"Dog","Type":"dog"}` — discriminator first, then the property). Rename or `[JsonIgnore]` the property.[^31]
- Polymorphism needs `[JsonPolymorphic]`/`[JsonDerivedType]`; only listed derived types are (de)serialized — never enable type names from the payload.
- Use `JsonSerializer.DeserializeAsync`/`SerializeAsync` with streams; avoid `ReadAsStringAsync` + `Deserialize` for large bodies.
- Set `MaxDepth` (default 64) consciously; never raise it for untrusted input without reason.
- Newtonsoft.Json: `TypeNameHandling` other than `None` on untrusted input is remote code execution; flag it.

##### Source generation (required for trimming/AOT, faster startup)

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

var json = JsonSerializer.Serialize(new WeatherForecast(DateOnly.Parse("2026-10-22"), 18, null), AppJsonContext.Default.WeatherForecast);
Console.WriteLine(json);   // {"date":"2026-10-22","temperatureC":18}
var back = JsonSerializer.Deserialize(json, AppJsonContext.Default.WeatherForecast);
Console.WriteLine(back);

public sealed record WeatherForecast(DateOnly Date, int TemperatureC, string? Summary);

[JsonSourceGenerationOptions(JsonSerializerDefaults.Web,
    DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    RespectNullableAnnotations = true,
    AllowDuplicateProperties = false)]
[JsonSerializable(typeof(WeatherForecast))]
internal partial class AppJsonContext : JsonSerializerContext;
```

In ASP.NET Core register the context with `ConfigureHttpJsonOptions(o => o.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonContext.Default))`. .NET 10 allows `ReferenceHandler = JsonKnownReferenceHandler.Preserve` on `[JsonSourceGenerationOptions]`.[^8]

---
#### ASP.NET Core 10 Minimal APIs

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.AspNetCore.Http.HttpResults;

var builder = WebApplication.CreateSlimBuilder(args);
builder.Services.AddValidation();                 // .NET 10: DataAnnotations validation for minimal APIs
builder.Services.AddProblemDetails();
builder.Services.AddAuthentication().AddCookie();
builder.Services.AddAuthorization();
builder.Services.ConfigureHttpJsonOptions(o => o.SerializerOptions.TypeInfoResolverChain.Insert(0, ApiJson.Default));

var app = builder.Build();
app.UseExceptionHandler();
app.UseAuthentication();
app.UseAuthorization();

var products = app.MapGroup("/products").RequireAuthorization();

products.MapPost("/", async Task<Results<Created<Product>, Conflict>> (Product p, IProductStore store, CancellationToken ct) =>
    await store.TryAddAsync(p, ct) ? TypedResults.Created($"/products/{p.Sku}", p) : TypedResults.Conflict());

products.MapGet("/{sku}", async Task<Results<Ok<Product>, NotFound>> (string sku, IProductStore store, CancellationToken ct) =>
    await store.FindAsync(sku, ct) is { } p ? TypedResults.Ok(p) : TypedResults.NotFound());

app.Run();

public sealed record Product([Required, StringLength(32)] string Sku, [Range(1, 10_000)] int Quantity);

public interface IProductStore
{
    Task<bool> TryAddAsync(Product p, CancellationToken ct);
    Task<Product?> FindAsync(string sku, CancellationToken ct);
}

[System.Text.Json.Serialization.JsonSerializable(typeof(Product))]
internal partial class ApiJson : System.Text.Json.Serialization.JsonSerializerContext;
```

Verified behavior of this shape on ASP.NET Core 10.0.13: for an authenticated caller, an invalid body (`{"sku":"","quantity":0}`) returns `400` with `application/problem+json` listing both field errors; an unauthenticated request to a JSON/`TypedResults` endpoint returns `401` (no login redirect); an HTML endpoint still redirects (`302`).

Review points:

- **Validation is opt-in.** Without `AddValidation()` (and for endpoints with `.DisableValidation()`) DataAnnotations on parameters are ignored. The validation source generator only discovers types in the assembly that calls `AddValidation()`.[^10]
- Use `TypedResults` and `Results<T1, T2>` return types — they document responses in OpenAPI and mark endpoints as API endpoints for the cookie 401/403 behavior.
- Bind `CancellationToken` and pass it on. Do not inject scoped services into singletons used by endpoints.
- **Authorization**: `RequireAuthorization()` on groups; a missing `UseAuthorization()`/wrong middleware order is a security bug. Check object-level authorization (the user may access *this* resource), not just authentication.
- **Mass assignment**: never bind request bodies directly to EF entities; use request DTOs.
- **Antiforgery**: when antiforgery services are registered (`AddAntiforgery()`), minimal-API endpoints that bind forms (`[FromForm]`, `IFormFile`) validate antiforgery tokens automatically; `.DisableAntiforgery()` on such endpoints needs a justification. Cookie-authenticated form endpoints without antiforgery are CSRF-prone.
- **Forwarded headers**: `UseForwardedHeaders` with `KnownProxies`/`KnownIPNetworks` (`KnownNetworks` and `Microsoft.AspNetCore.HttpOverrides.IPNetwork` are obsolete in .NET 10).[^22]
- **CORS**: `AllowAnyOrigin()` with credentials is rejected by the framework; `SetIsOriginAllowed(_ => true)` + `AllowCredentials()` is the same hole by another name.
- **Rate limiting** (`AddRateLimiter`/`RequireRateLimiting`) on login, signup and expensive endpoints; request size limits (`MaxRequestBodySize`) for uploads.
- `IExceptionHandler` that returns `true` suppresses the middleware's diagnostics in .NET 10 — make sure the handler logs.[^22]
- Return RFC 9457 problem details; never return exception messages/stack traces outside Development (`app.Environment.IsDevelopment()` guard around `UseDeveloperExceptionPage`).
- `WebApplication.CreateSlimBuilder` omits some defaults (no HTTPS configuration, minimal logging providers) and is the template for Native AOT.
- `WebHostBuilder`/`IWebHost` are obsolete in .NET 10; use `WebApplication.CreateBuilder`.

---

#### EF Core 10

```csharp
using Microsoft.EntityFrameworkCore;

await using var db = new ShopContext();
await db.Database.OpenConnectionAsync();
await db.Database.EnsureCreatedAsync();
db.Blogs.Add(new Blog { Name = "a", Posts = [new() { Title = "p1" }, new() { Title = "p2" }] });
db.Blogs.Add(new Blog { Name = "b" });
await db.SaveChangesAsync();
db.ChangeTracker.Clear();

// GOOD: one query, read-only, projected
var summaries = await db.Blogs
    .AsNoTracking()
    .Select(b => new { b.Name, PostCount = b.Posts.Count })
    .ToListAsync();
Console.WriteLine(string.Join(",", summaries.Select(s => $"{s.Name}:{s.PostCount}")));   // a:2,b:0

// GOOD: parameterized raw SQL via interpolation (becomes @p0)
var name = "a' OR 1=1 --";
Console.WriteLine(await db.Blogs.FromSql($"SELECT * FROM Blogs WHERE Name = {name}").CountAsync());  // 0

// .NET 10: LeftJoin is translated to LEFT JOIN
var rows = await db.Blogs
    .LeftJoin(db.Posts, b => b.Id, p => p.BlogId, (b, p) => new { b.Name, Title = p == null ? null : p.Title })
    .ToListAsync();
Console.WriteLine(rows.Count);                                                           // 3

// .NET 10: ExecuteUpdateAsync takes a regular lambda; setters can be conditional
bool rename = true;
int affected = await db.Blogs.ExecuteUpdateAsync(s =>
{
    s.SetProperty(b => b.Views, b => b.Views + 1);
    if (rename) s.SetProperty(b => b.Name, "renamed");
});
Console.WriteLine(affected);                                                             // 2

sealed class ShopContext : DbContext
{
    public DbSet<Blog> Blogs => Set<Blog>();
    public DbSet<Post> Posts => Set<Post>();
    protected override void OnConfiguring(DbContextOptionsBuilder o) => o.UseSqlite("Data Source=:memory:");
}
sealed class Blog { public int Id { get; set; } public required string Name { get; set; } public int Views { get; set; } public List<Post> Posts { get; set; } = []; }
sealed class Post { public int Id { get; set; } public required string Title { get; set; } public int BlogId { get; set; } }
```

##### Query pitfalls

- **N+1 queries**: loading a list and then touching a navigation per item (lazy loading proxies, or `Entry(x).Collection(...).Load()` in a loop) issues one query per row — verified: two blogs → three queries; `Include` → one. Use `Include`/`ThenInclude`, projection (`Select`), or `AsSplitQuery()` for multiple collection includes (cartesian explosion).
- **Client evaluation**: a method the provider cannot translate in `Where`/`OrderBy` throws `InvalidOperationException` ("could not be translated"); calling `AsEnumerable()`/`ToList()` early to "fix" it pulls the whole table into memory.
- **Tracking**: queries track entities by default. Use `AsNoTracking()` for read-only queries (or `UseQueryTrackingBehavior(QueryTrackingBehavior.NoTracking)` on read-only contexts). Identity resolution means a second query returns the **same, possibly stale** instance: after `ExecuteUpdateAsync`, already-tracked entities are *not* refreshed (verified: a tracked entity kept `Views = 0` after a bulk update and a re-query returned the same stale instance).
- `ExecuteUpdate`/`ExecuteDelete` bypass the change tracker, interceptors that rely on `SaveChanges`, and concurrency tokens unless you filter on them.
- **Async all the way**: `ToListAsync`, `FirstOrDefaultAsync`, `SaveChangesAsync` with a `CancellationToken`. Do not mix sync and async calls on the same context.
- **`DbContext` is not thread-safe** and is scoped. Never share one across threads/parallel tasks (`Task.WhenAll` over queries on the same context throws "A second operation was started"). Use `IDbContextFactory<T>` for parallel or background work.
- Pagination: `Skip`/`Take` need a deterministic `OrderBy`; prefer keyset pagination for deep pages.
- `Count()` on an `IQueryable` runs `SELECT COUNT(*)`; `.ToList().Count` loads everything.
- Long-lived contexts accumulate tracked entities (memory, slow `DetectChanges`); keep units of work short.
- C# 14 span conversion: `ids.Contains(x.Id)` in queries binds to `MemoryExtensions.Contains`; EF Core 10 translates it, other LINQ providers may not (see C# 14 section).
- .NET 10 parameterized collections use one parameter per element (`IN (@ids1, @ids2, ...)`); very large lists can hit parameter limits (SQL Server 2,100) — use `EF.Parameter(ids)` (JSON parameter) or a temp table for big sets.[^23]

##### Raw SQL and injection

```csharp
using Microsoft.EntityFrameworkCore;

static Task<List<Blog>> Search(ShopContext db, string term) =>
    // BAD (EF1003 warning): concatenation into FromSqlRaw -> SQL injection
    // db.Blogs.FromSqlRaw("SELECT * FROM Blogs WHERE Name = '" + term + "'").ToListAsync();
    // GOOD: FromSql / FromSqlInterpolated turn holes into DbParameters
    db.Blogs.FromSql($"SELECT * FROM Blogs WHERE Name = {term}").ToListAsync();

sealed class ShopContext : DbContext { public DbSet<Blog> Blogs => Set<Blog>(); }
sealed class Blog { public int Id { get; set; } public required string Name { get; set; } }
```

- `FromSql($"...")`, `FromSqlInterpolated`, `SqlQuery<T>($"...")`, `ExecuteSql($"...")` parameterize interpolation holes. `FromSqlRaw`/`ExecuteSqlRaw`/`SqlQueryRaw` with concatenated or `string.Format`-ed input are SQL injection; EF Core's analyzer reports EF1003 for concatenation (verified).
- Building an interpolated string in a variable first (`var sql = $"...{x}"; FromSqlRaw(sql)`) loses parameterization — the `FormattableString` must be passed directly.
- Identifiers (table/column names, `ORDER BY` columns) cannot be parameters; map user input through an allow-list.
- ADO.NET/Dapper: always `command.Parameters.Add(...)`/anonymous parameter objects; CA2100 flags concatenated `CommandText`.

##### Migrations and model

- Review generated migrations: column renames that EF generates as drop+add lose data; `ALTER COLUMN` to the SQL Server `json` type is generated automatically when moving to `UseAzureSql`/compatibility level 170 in EF Core 10.[^23]
- Do not call `Database.Migrate()` from multiple app instances concurrently at startup; run migrations as a deployment step (bundles: `dotnet ef migrations bundle`).
- Configure `decimal` precision (`HasPrecision(18, 2)`), string lengths, and indexes for filtered/joined columns; concurrency tokens (`IsRowVersion()`) for optimistic concurrency.
- `EnableSensitiveDataLogging()` logs parameter values (PII/secrets) — Development only. EF Core 10 redacts inlined constants in logs unless it is enabled.[^23]
- Microsoft.Data.Sqlite 10 changed `DateTime`/`DateTimeOffset` handling to UTC; the `Microsoft.Data.Sqlite.Pre10TimeZoneHandling` AppContext switch is a temporary escape hatch.

---

#### Security

##### Deserialization

- `BinaryFormatter` and friends are gone (throw `PlatformNotSupportedException` since .NET 9); do not re-enable them through the unsupported compatibility package.[^28]
- Newtonsoft.Json `TypeNameHandling.All/Auto/Objects` and `JavaScriptSerializer` with type resolvers on untrusted data are RCE.
- `DataContractSerializer`/`XmlSerializer`: only with known types; never construct the serializer from a type name read from input.
- System.Text.Json polymorphism only via `[JsonDerivedType]` allow-lists. For security-critical payloads use `AllowDuplicateProperties = false` or `JsonSerializerOptions.Strict` (.NET 10).

##### Path traversal

```csharp
string root = Path.GetFullPath(Path.Combine(Path.GetTempPath(), "uploads"));
Directory.CreateDirectory(root);

Console.WriteLine(Path.Combine(root, "/etc/passwd"));         // /etc/passwd — an absolute segment discards root!
Console.WriteLine(TryResolve(root, "../../etc/passwd", out _)); // False
Console.WriteLine(TryResolve(root, "a/b.txt", out var ok));     // True
Console.WriteLine(ok.StartsWith(root, StringComparison.Ordinal));

static bool TryResolve(string root, string userPath, out string fullPath)
{
    // Ensure the root ends with a separator so "/uploads-evil" does not pass a "/uploads" prefix check
    string rootWithSep = Path.EndsInDirectorySeparator(root) ? root : root + Path.DirectorySeparatorChar;
    fullPath = Path.GetFullPath(Path.Combine(rootWithSep, userPath));
    return fullPath.StartsWith(rootWithSep, OperatingSystem.IsWindows() ? StringComparison.OrdinalIgnoreCase : StringComparison.Ordinal);
}
```

- `Path.Combine`/`Path.Join` do not sanitize: `Path.Combine(root, "/etc/passwd")` returns `/etc/passwd`, and `..` segments survive until `GetFullPath`. Always canonicalize and prefix-check against the root **with a trailing separator**.
- Use `Path.GetFileName(upload.FileName)` and preferably a server-generated file name for uploads; never trust `IFormFile.FileName` or `Content-Type`.
- Symlinks inside the root can still escape; for strict sandboxes check `FileSystemInfo.LinkTarget`/`ResolveLinkTarget` or avoid user-writable roots.
- ZIP extraction: `ZipFile.ExtractToDirectory` validates entry paths against the destination, but code that iterates entries and calls `Path.Combine(dest, entry.FullName)` itself is a "Zip Slip" vulnerability. Also cap total uncompressed size (zip bombs).

##### Cryptography

```csharp
using System.Security.Cryptography;
using System.Text;

// Password hashing: PBKDF2 one-shot (Rfc2898DeriveBytes constructors are obsolete in .NET 10, SYSLIB0060)
byte[] salt = RandomNumberGenerator.GetBytes(16);
byte[] hash = Rfc2898DeriveBytes.Pbkdf2("correct horse"u8, salt, 600_000, HashAlgorithmName.SHA256, 32);

// Verification: constant-time comparison
byte[] attempt = Rfc2898DeriveBytes.Pbkdf2("correct horse"u8, salt, 600_000, HashAlgorithmName.SHA256, 32);
Console.WriteLine(CryptographicOperations.FixedTimeEquals(hash, attempt));     // True

// Authenticated encryption
byte[] key = RandomNumberGenerator.GetBytes(32);
byte[] nonce = RandomNumberGenerator.GetBytes(AesGcm.NonceByteSizes.MaxSize);    // 12 bytes; never reuse with the same key
byte[] plaintext = Encoding.UTF8.GetBytes("secret");
byte[] ciphertext = new byte[plaintext.Length];
byte[] tag = new byte[AesGcm.TagByteSizes.MaxSize];
using (var aes = new AesGcm(key, tag.Length))
    aes.Encrypt(nonce, plaintext, ciphertext, tag);

byte[] decrypted = new byte[ciphertext.Length];
using (var aes = new AesGcm(key, tag.Length))
    aes.Decrypt(nonce, ciphertext, tag, decrypted);
Console.WriteLine(Encoding.UTF8.GetString(decrypted));                           // secret

// Tokens and IDs
string token = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
Console.WriteLine(token.Length);                                                 // 44
Console.WriteLine(RandomNumberGenerator.GetInt32(1, 7) is >= 1 and <= 6);        // True

// Post-quantum (.NET 10): always check availability
Console.WriteLine(MLKem.IsSupported ? "ML-KEM available" : "ML-KEM unavailable (needs OpenSSL 3.5+ / PQC-capable CNG)");
```

- **Never** `System.Random`/`Random.Shared` for secrets, tokens, salts, IDs that must be unguessable — use `RandomNumberGenerator`.
- Compare MACs, hashes and tokens with `CryptographicOperations.FixedTimeEquals`, never `SequenceEqual`/`==`.
- Passwords: ASP.NET Core Identity's `PasswordHasher<T>` or PBKDF2 with ≥ 600,000 iterations of SHA-256 (OWASP), Argon2id/bcrypt via vetted libraries. Never MD5/SHA-1/plain SHA-256 for passwords.
- Symmetric encryption: `AesGcm` or `ChaCha20Poly1305` (authenticated). AES-CBC without a MAC, ECB mode, and static IVs are findings. `AesGcm` constructor requires the tag size (the parameterless-tag constructor is obsolete, SYSLIB0053).
- Asymmetric: RSA ≥ 2048 bits with OAEP-SHA256 / PSS; ECDSA P-256+. Dispose key objects.
- `X509Certificate2` loading: use `X509CertificateLoader` (.NET 9+; the `X509Certificate2(byte[])`/`(string)` constructors are obsolete, SYSLIB0057). Find certificates by SHA-256 thumbprint with `FindByThumbprint(HashAlgorithmName.SHA256, …)` (.NET 10).
- Data Protection: persist keys to durable shared storage in multi-instance deployments and protect them at rest; patch to ≥ 10.0.7 (CVE-2026-40372) and rotate keys if 10.0.0–10.0.6 was deployed.[^29]
- PQC types (`MLKem`, `MLDsa`, `SlhDsa`, `CompositeMLDsa`) depend on OS crypto (OpenSSL 3.5+ / Windows CNG with PQC); `SlhDsa` and `CompositeMLDsa` are `[Experimental]` types (SYSLIB5006), and several `MLKem`/`MLDsa` key import/export members (PKCS#8, SPKI, PEM) are `[Experimental]` too (verified by reflection on 10.0.13) — their APIs may change.

##### Other injection and parsing risks

- **XML**: `XmlReader.Create` defaults to `DtdProcessing.Prohibit` — keep it; `XmlDocument` with `XmlResolver` set or `DtdProcessing.Parse` enables XXE. `XsltSettings.EnableScript` is obsolete (SYSLIB0062); never enable scripts or `document()` for untrusted XSLT.
- **Regex**: user-influenced patterns or inputs need a timeout (`new Regex(p, opts, TimeSpan.FromSeconds(1))`) or `RegexOptions.NonBacktracking`; prefer `[GeneratedRegex]` for static patterns.
- **Process start**: `ProcessStartInfo.ArgumentList` instead of a concatenated `Arguments` string; `UseShellExecute = false`; never pass user input to `cmd /c` or `bash -c`.
- **Open redirects**: validate `returnUrl` with `Url.IsLocalUrl` / `LocalRedirect`.
- **Logging/headers**: CRLF in user input written to headers is rejected by Kestrel, but log forging via newlines is not — use structured logging.
- **Secrets**: never in source, `appsettings*.json` in the repo, or logs; environment variables (`MyApp__Db__Password`) injected by the platform, or a secret store.
- **Timing/size limits**: `Uri` no longer enforces a ~65 KB limit in .NET 10 — validate input lengths explicitly.[^9]

---

#### Performance

- Measure first: BenchmarkDotNet for micro-benchmarks, `dotnet-counters`/`dotnet-trace`/`dotnet-gcdump` in production-like environments.
- Avoid allocations in hot paths: `Span<T>`, `stackalloc` (bounded), `ArrayPool<T>`, `string.Create`, `StringBuilder` for loops, `[GeneratedRegex]`, `SearchValues<T>` for repeated `IndexOfAny`, `CompositeFormat` for repeated formats, `FrozenDictionary` for static lookups.
- Strings: `StringComparison.Ordinal`/`OrdinalIgnoreCase` for identifiers (culture-aware comparison is slower and wrong for protocol data; CA1309/CA1310). `string.Equals(a, b, StringComparison.OrdinalIgnoreCase)` instead of `a.ToLower() == b.ToLower()`.
- Collections: presize (`new List<T>(n)`, `new Dictionary<K,V>(n)`), use `TryGetValue` instead of `ContainsKey` + indexer, `CollectionsMarshal.GetValueRefOrAddDefault` for update-in-place, `HashSet<T>` for membership.
- Async: avoid `async` state machines for synchronous fast paths (`ValueTask`), avoid `Task.Run` wrappers, avoid sync-over-async.
- Boxing: generic constraints instead of `object`/interfaces on structs; `Enum.HasFlag` is fine on .NET Core (JIT intrinsic).
- Server GC is default in ASP.NET Core; for containers with small memory limits consider `DOTNET_GCHeapHardLimit`/`GCHeapAffinitizeMask` or DATAS (dynamic adaptation, default for Server GC since .NET 9).
- Use `ReadyToRun`/Native AOT for startup-sensitive workloads; Tiered PGO is on by default.
- .NET 10 JIT improvements (array interface devirtualization, stack allocation of small arrays/delegates) reduce the cost of abstractions — benchmark before replacing LINQ/`IEnumerable<T>` with hand-written loops.[^21]
- `HttpClient`, `JsonSerializerOptions`, `Regex` (non-generated), `XmlSerializer(Type, ...)` overloads other than `(Type)` and `(Type, string)` must be cached — constructing them per call leaks or recompiles.

---

#### Testing

##### xUnit.net v3

```csharp
using Xunit;

public class PriceCalculatorTests
{
    [Fact]
    public void Discount_IsApplied() => Assert.Equal(90m, PriceCalculator.Apply(100m, 0.1m));

    [Theory]
    [InlineData(100, 0.0, 100)]
    [InlineData(100, 0.25, 75)]
    public void Apply_VariousRates(decimal price, decimal rate, decimal expected) =>
        Assert.Equal(expected, PriceCalculator.Apply(price, rate));

    [Fact]
    public async Task Cancellation_IsHonored()
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(TestContext.Current.CancellationToken);
        cts.Cancel();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => Task.Delay(1000, cts.Token));
    }

    [Fact]
    public void Apply_RejectsNegativeRate() =>
        Assert.Throws<ArgumentOutOfRangeException>(() => PriceCalculator.Apply(100m, -1m));
}

public static class PriceCalculator
{
    public static decimal Apply(decimal price, decimal rate)
    {
        ArgumentOutOfRangeException.ThrowIfNegative(rate);
        return price * (1 - rate);
    }
}
```

- Current versions (October 2026): **xunit.v3 4.0.2** (4.0.0 released August 2026: Native AOT support, full test parallelization, new test class/method orderers, Microsoft.Testing.Platform v2 by default, MTP v1 support removed), **MSTest 4.5.1**, **NUnit 5.0.0** (September 27, 2026), Microsoft.NET.Test.Sdk 18.11.0.[^32][^33] xUnit v3 test projects are executables (`<OutputType>Exe</OutputType>`) and run with `dotnet run`, `dotnet test` or the MTP runner.
- **Always `await` async assertions** (`await Assert.ThrowsAsync<T>(...)`). An un-awaited `Assert.ThrowsAsync` never fails the test. **NUnit 5 changed `Assert.ThrowsAsync`/`CatchAsync`/`DoesNotThrowAsync` to return a `Task`** that must be awaited; NUnit 4-style un-awaited calls now silently pass (verified: in NUnit 5.0.0 the three methods return `Task<T>`/`Task`, in 4.6.1 they returned the exception/`void`); NUnit.Analyzers 4.14+ helps convert existing code.[^33]
- Use `TestContext.Current.CancellationToken` (xUnit v3) so tests stop on timeout/cancellation; xUnit analyzers flag missing tokens (xUnit1051).
- Do not share mutable static state between tests — xUnit v3 4.0 can parallelize all tests, not just collections.
- Use `TimeProvider`/`FakeTimeProvider` (`Microsoft.Extensions.TimeProvider.Testing`) instead of `Thread.Sleep`/`DateTime.Now` in tests.
- Integration tests: `WebApplicationFactory<Program>` (`Microsoft.AspNetCore.Mvc.Testing` 10.x), Testcontainers for real databases. EF Core `InMemory` provider does not behave like a relational database (no transactions, no SQL translation errors) — prefer SQLite in-memory or a container.
- Mocks: mock interfaces you own; do not mock `HttpClient` directly — mock the `HttpMessageHandler` or use a fake server. Avoid mocking `DbContext`.
- Snapshot tests of EF Core SQL break on upgrade to EF Core 10 (simplified parameter names, multi-parameter `IN`).
- Coverage: `coverlet.collector` or `Microsoft.Testing.Extensions.CodeCoverage`; in .NET 10 `EnableDynamicNativeInstrumentation` defaults to `false`.[^9]

---

#### Trimming and Native AOT

```xml
<PropertyGroup>
  <PublishAot>true</PublishAot>            <!-- implies trimming; enables trim/AOT analyzers -->
  <IsAotCompatible>true</IsAotCompatible>  <!-- for libraries: run the analyzers without publishing -->
  <InvariantGlobalization>true</InvariantGlobalization>  <!-- only if the app does not need culture data -->
</PropertyGroup>
```

- Treat IL2xxx (trimming) and IL3xxx (AOT) warnings as errors; a suppressed `IL2026` usually means a runtime failure after publish.
- Reflection-based System.Text.Json, configuration binding, DI with open generics over reflection, `Activator.CreateInstance(Type)` on unknown types, `Assembly.Load`, `Reflection.Emit`, `dynamic` and LINQ `Expression.Compile()` (interpreted under AOT) are not AOT-friendly. Use source generators: JSON `JsonSerializerContext`, `[LoggerMessage]`, `[GeneratedRegex]`, configuration binding generator, minimal API request delegate generator (automatic with `PublishAot`), `[LibraryImport]` instead of `[DllImport]`.
- Annotate reflection with `[DynamicallyAccessedMembers]`, `[RequiresUnreferencedCode]`, `[RequiresDynamicCode]` rather than suppressing.
- ASP.NET Core Native AOT supports minimal APIs, gRPC and worker services; SignalR only partially; MVC controllers, Razor Pages and Blazor Server are not supported. EF Core Native AOT support (precompiled queries) is experimental.
- File-based apps (`dotnet run app.cs`) publish as Native AOT by default in .NET 10.[^16]
- `PublishTrimmed` disables HTTP/3 by default in .NET 10.[^9]

---

#### Removed / Changed in .NET 10 — Migration Notes

Upgrading from .NET 8/9 (C# 12/13) to .NET 10 (C# 14):

1. **Language**: compile, then fix CS9258 (`field`), CS9273 (variables named `field` in accessors), CS9336 (redundant `or` patterns), new ambiguity errors from span conversions, identifiers named `extension`/`partial`/`scoped`. Check expression trees that call `Contains`/`SequenceEqual`/`StartsWith` on arrays when using LINQ providers other than EF Core.
2. **Hosting**: plain console/worker apps without the Generic Host must register SIGTERM handling. `BackgroundService` code that relied on running synchronously before startup completes must move to `StartAsync`/`IHostedLifecycleService`.
3. **Configuration**: review options for `null` handling and empty arrays.
4. **Packages**: remove/upgrade `System.Linq.Async`; align all `Microsoft.Extensions.*`, `Microsoft.AspNetCore.*`, `Microsoft.EntityFrameworkCore.*` packages to 10.x; react to NU1510 (pruned references) and transitive NuGet Audit warnings.
5. **Obsoletions**: SYSLIB0058–SYSLIB0062; `Rfc2898DeriveBytes` constructors → `Pbkdf2`; ASP.NET Core `WebHostBuilder`, `IActionContextAccessor`, `KnownNetworks`, `WithOpenApi`, Razor runtime compilation.
6. **JSON**: polymorphic types with property names equal to the discriminator now throw; consider `AllowDuplicateProperties = false`/`Strict` for untrusted input.
7. **ASP.NET Core**: cookie auth now returns 401/403 for API endpoints (SPA clients that followed redirects need updating); `IExceptionHandler` diagnostics suppression; OpenAPI 3.1 (`OpenApiAny` → `JsonNode`); custom `JsonConverter`s must handle `HasValueSequence`.
8. **EF Core 10**: `ExecuteUpdate` signature, parameterized collection translation, simplified parameter names, complex-type column naming, SQL Server `json` type migrations, `dotnet ef --framework` for multi-targeting, Microsoft.Data.Sqlite UTC changes.
9. **Containers**: default images are Ubuntu-based; non-root `app` user (since .NET 8) and port 8080.
10. **Runtime**: `Uri` length limit removed; tar entries lack atime/ctime; OpenSSL ≥ 1.1.1 required; single-file native library probing changed.
11. **Tests**: xUnit v3 4.x / NUnit 5 / MSTest 4 upgrades have their own breaking changes (await `Assert.ThrowsAsync` in NUnit 5).
12. **Support**: .NET 8 and .NET 9 end support on November 10, 2026; .NET 10 is supported until November 14, 2028. .NET 11 (GA expected November 2026) is an STS release.

---

#### Production / Review Checklist

##### Language and types

- [ ] `net10.0` / C# 14 (no `LangVersion` preview), SDK pinned in `global.json`, runtime ≥ 10.0.13
- [ ] No C# 15 / .NET 11 features (unions, `closed`, `with(...)` in collection expressions, extension indexers, labeled `break`/`continue`, `FullJoin`, `Process.Run`, `JsonNamingPolicy.PascalCase`, …)
- [ ] CS9258 (`field` rebinding) treated as an error; no members named `field` read inside accessors
- [ ] Span-related overload changes reviewed (covariant arrays, expression trees with `Contains`)
- [ ] Nullable enabled, `!` justified, public APIs validated with throw helpers
- [ ] Records: no mutable collections in value-equality types; `PrintMembers` hides secrets; `readonly record struct` for value objects
- [ ] Primary-constructor parameters not double-captured (CS9124) or mutated unintentionally
- [ ] Pattern combinators parenthesized (`is not (A or B)`); no suppressed CS8509

##### Async and concurrency

- [ ] No `.Result`/`.Wait()`/`async void` (except handled event handlers); no fire-and-forget tasks
- [ ] `CancellationToken` accepted and forwarded everywhere (CA2016); linked CTS disposed
- [ ] `ConfigureAwait(false)` consistently in libraries; not needed in ASP.NET Core apps
- [ ] `SemaphoreSlim` released in `finally`; `Lock` type not converted to `object`
- [ ] `DbContext` never shared across concurrent operations
- [ ] Bounded channels / back-pressure for queues

##### Hosting, DI, configuration

- [ ] SIGTERM handled in non-host console apps (.NET 10); graceful shutdown within the orchestrator grace period
- [ ] `BackgroundService` startup-ordering assumptions revisited (.NET 10); loops catch and log per-iteration failures
- [ ] No captive dependencies; scopes created in singletons/background services
- [ ] Options validated with `ValidateOnStart`; `null` binding in .NET 10 considered
- [ ] Secrets only from environment/secret stores; never logged
- [ ] `System.Linq.Async` removed or ≥ 7.0

##### Data and I/O

- [ ] `IHttpClientFactory`/long-lived `SocketsHttpHandler`; timeouts; responses disposed; typed clients not captured by singletons
- [ ] Cached `JsonSerializerOptions`; source-generated contexts for AOT; `Strict`/`AllowDuplicateProperties=false` for untrusted input
- [ ] EF Core: `AsNoTracking` for reads, no N+1, projections, async with tokens, no client-side evaluation
- [ ] No `FromSqlRaw`/`ExecuteSqlRaw` with concatenation (EF1003); identifiers allow-listed
- [ ] Paths canonicalized and prefix-checked with a trailing separator; uploads renamed
- [ ] XML with `DtdProcessing.Prohibit`; regex timeouts or `NonBacktracking` for untrusted input

##### Security

- [ ] No `BinaryFormatter`/`TypeNameHandling`; polymorphic JSON via allow-lists
- [ ] `RandomNumberGenerator` for secrets; `FixedTimeEquals` for comparisons
- [ ] `Rfc2898DeriveBytes.Pbkdf2` (≥ 600,000 SHA-256 iterations) or Identity hasher; AES-GCM / ChaCha20-Poly1305
- [ ] `X509CertificateLoader`; no disabled certificate validation
- [ ] PQC usage guarded by `IsSupported`; experimental APIs acknowledged (SYSLIB5006)
- [ ] ASP.NET Core: authorization on every endpoint group, antiforgery for forms, rate limiting, problem details without stack traces, forwarded headers configured with `KnownIPNetworks`
- [ ] DataProtection ≥ 10.0.7 and key ring rotated if 10.0.0–10.0.6 was deployed

##### Tooling, tests, observability

- [ ] `TreatWarningsAsErrors`, `AnalysisLevel` set, `EnforceCodeStyleInBuild`, no blanket `NoWarn`
- [ ] Central package management, lock files, NuGet Audit (transitive) clean
- [ ] Trim/AOT warnings (IL2xxx/IL3xxx) fixed, not suppressed
- [ ] Tests await async assertions; use `TestContext.Current.CancellationToken`/`TimeProvider`; integration tests on a real provider
- [ ] `[LoggerMessage]` / constant templates; no secrets/PII in logs; `Meter`/`ActivitySource` with bounded tag cardinality

---

[^1]: Announcing .NET 10. .NET Blog, November 11, 2025. https://devblogs.microsoft.com/dotnet/announcing-dotnet-10/
[^2]: What's new in .NET 10 (overview). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/overview
[^3]: .NET 10.0 release metadata (releases.json: 10.0.0 on 2025-11-11 … 10.0.13 on 2026-10-13, SDKs, CVE lists, EOL date). https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json ; release notes: https://github.com/dotnet/core/blob/main/release-notes/10.0/10.0.13/10.0.13.md
[^4]: .NET and .NET Core Support Policy / release index (support phases and end-of-support dates for .NET 8, 9, 10). https://dotnet.microsoft.com/platform/support/policy/dotnet-core ; https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json
[^5]: .NET 11 Release Candidate 2 is now available. .NET Blog, October 13, 2026. https://devblogs.microsoft.com/dotnet/dotnet-11-rc-2/
[^6]: What's new in C# 14. Microsoft Learn. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-14
[^7]: C# compiler breaking changes since C# 13 (Roslyn "Compiler Breaking Changes - DotNet 10"). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%2010
[^8]: What's new in .NET libraries for .NET 10. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/libraries
[^9]: Breaking changes in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/10
[^10]: What's new in ASP.NET Core in .NET 10. https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-10.0
[^11]: The history of C# (C# 9–13 feature lists). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-version-history
[^12]: Extension members — feature specification and `extension` keyword reference. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/proposals/csharp-14.0/extensions ; https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/keywords/extension
[^13]: First-class Span types — feature specification. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/proposals/csharp-14.0/first-class-span-types
[^14]: User-defined compound assignment operators — feature specification. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/proposals/csharp-14.0/user-defined-compound-assignment
[^15]: Roslyn Language Feature Status (C# 14 and C# 15 feature tables). https://github.com/dotnet/roslyn/blob/main/docs/Language%20Feature%20Status.md
[^16]: What's new in the SDK and tooling for .NET 10. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/sdk
[^17]: Breaking change: .NET runtime no longer provides default termination signal handlers. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/sigterm-signal-handler
[^18]: Breaking change: BackgroundService runs all of ExecuteAsync as a Task. https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/10.0/backgroundservice-executeasync-task
[^19]: Breaking change: Null values preserved in configuration. https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/10.0/configuration-null-values-preserved
[^20]: Breaking change: System.Linq.AsyncEnumerable in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/asyncenumerable
[^21]: What's new in the .NET 10 runtime. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/runtime
[^22]: Breaking changes in ASP.NET Core 10. https://learn.microsoft.com/en-us/aspnet/core/breaking-changes/10/overview
[^23]: Breaking changes in EF Core 10. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-10.0/breaking-changes
[^24]: What's new in EF Core 10. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-10.0/whatsnew
[^25]: What's new in C# 15 (preview). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
[^26]: What's new in .NET libraries for .NET 11 (preview). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/libraries
[^27]: Breaking change: .NET 10 obsoletions with custom IDs (SYSLIB0058–SYSLIB0062). https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/obsolete-apis
[^28]: BinaryFormatter migration guide (removed in .NET 9). https://learn.microsoft.com/en-us/dotnet/standard/serialization/binaryformatter-migration-guide/
[^29]: .NET 10.0.7 release notes (CVE-2026-40372, Microsoft.AspNetCore.DataProtection). https://github.com/dotnet/core/blob/main/release-notes/10.0/10.0.7/10.0.7.md
[^30]: Code analysis in .NET — AnalysisLevel and AnalysisMode. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/overview
[^31]: Breaking change: System.Text.Json checks for property name conflicts. https://learn.microsoft.com/en-us/dotnet/core/compatibility/serialization/10/property-name-validation
[^32]: xUnit.net v3 4.0.0 release notes. https://xunit.net/releases/v3/4.0.0
[^33]: NUnit framework release notes (5.0.0, September 27, 2026). https://docs.nunit.org/articles/nunit/release-notes/framework.html
