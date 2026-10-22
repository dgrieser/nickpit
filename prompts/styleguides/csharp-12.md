### C# 12 on .NET 8 (LTS): The Complete Production Developer Guide

> **Version**: C# 12 ships with the .NET 8 SDK. .NET 8.0.0 was released on November 14, 2023 as a Long Term Support (LTS) release.[^1] The latest patch as of October 2026 is **.NET 8.0.32 (October 13, 2026)** — runtime 8.0.32, ASP.NET Core 8.0.32, SDKs **8.0.426** and 8.0.132; EF Core 8.0.32 shipped the same day.[^2]
> **Support status**: .NET 8 is in its **maintenance phase** and reaches **end of support on November 10, 2026** — less than three weeks after the date of this guide. After that date it receives no security fixes. The current LTS release is **.NET 10** (C# 14, released November 11, 2025, latest patch 10.0.13 on October 13, 2026, supported until November 14, 2028); .NET 9 (STS, C# 13) also ends support on November 10, 2026, and .NET 11 (C# 15) is at Release Candidate 2 (11.0.0-rc.2, October 13, 2026) — a preview with go-live support, not a stable release.[^3] **Recommend planning the move to .NET 10 LTS** for every .NET 8 service that is still maintained.
> **Scope**: The C# 12 language and the .NET 8 runtime, base class library (BCL), ASP.NET Core 8 and EF Core 8 as used in production services and libraries: language features, nullable reference types, async/cancellation, collections and LINQ, memory, DI/options/logging, HttpClient, System.Text.Json, ASP.NET Core, EF Core, security, performance, testing, analyzers, trimming and Native AOT.
> **Note**: This document is self-contained. Everything from C# 1–11 and .NET Core 1.0–7 that still applies is carried forward. Sections marked **New in C# 12** or **New in .NET 8** cover changes specific to this version. Every example compiles and runs on SDK 8.0.426 / runtime 8.0.32 with `<TargetFramework>net8.0</TargetFramework>`, `<LangVersion>12</LangVersion>`, `<Nullable>enable</Nullable>` and `<ImplicitUsings>enable</ImplicitUsings>`.

Always run the latest 8.0.x patch: 8.0.32 alone fixes 40 CVEs, several rated remote code execution, and 8.0.21 (October 14, 2025) fixed CVE-2025-55315, a critical HTTP request smuggling flaw in Kestrel.[^2] A `global.json` or Dockerfile pinned to an older 8.0 patch is a security finding.

Global truths about C# 12 / .NET 8:

- **Managed, garbage-collected, statically typed.** Value types (`struct`, `enum`, primitives) are copied; reference types (`class`, `record`, `interface`, `delegate`, arrays, `string`) are shared by reference.
- **Nullable reference types are a compile-time annotation**, not a runtime guarantee. `string?` and `string` are the same runtime type; the compiler only warns. Code that receives data from JSON, EF, reflection or `default` can still see `null` in a `string`.
- **`async`/`await` is the concurrency model for I/O.** There is no global lock; thread-safety is the author's responsibility.
- **The language version follows the target framework.** `net8.0` defaults to C# 12 even when built with a newer SDK; the project file can override it with `<LangVersion>` (see Version-aware review).
- **The BCL version follows the runtime.** An API introduced in .NET 9 or 10 does not exist on `net8.0`, whatever the `LangVersion` — but many `Microsoft.Extensions.*` and EF Core 9 NuGet packages still target `net8.0` and are usable.

***

#### What's New in C# 12 and .NET 8

##### Language (C# 12)

| Feature | Summary | Section |
| --- | --- | --- |
| Primary constructors for classes and structs | `class Service(ILogger<Service> logger) { … }` — parameters are in scope in the whole type body; captured parameters become hidden mutable fields | [Primary constructors](#new-in-c-12-primary-constructors) |
| Collection expressions | `int[] a = [1, 2, 3];`, `List<int> l = [.. a, 4];`, `[]` for empty | [Collection expressions](#new-in-c-12-collection-expressions) |
| Inline arrays | `[InlineArray(8)] struct Buffer8 { int _e0; }` — fixed-size buffers without `unsafe` | [Inline arrays](#new-in-c-12-inline-arrays) |
| Optional and `params` parameters in lambdas | `var f = (int x, int by = 1) => x + by;` | [Lambdas](#new-in-c-12-default-and-params-lambda-parameters) |
| `ref readonly` parameters | `void M(ref readonly Big value)` — like `in`, but callers must pass a variable | [ref readonly](#new-in-c-12-ref-readonly-parameters) |
| Alias any type | `using Point = (int X, int Y);`, `using unsafe Ptr = int*;` | [Aliases](#new-in-c-12-alias-any-type) |
| `[Experimental]` attribute | Using a type or member marked `[Experimental("ID")]` is a compile **error** until the ID is suppressed | [Experimental](#new-in-c-12-experimentalattribute) |
| Interceptors (*preview*) | Source generators substitute call sites; opt-in via `InterceptorsPreviewNamespaces` | [Interceptors](#new-in-c-12-interceptors-preview-only) |

Each of these features was verified on SDK 8.0.426: with `<LangVersion>11</LangVersion>` every language feature above fails with `error CS9058: Feature '…' is not available in C# 11.0`; the `[Experimental]` attribute is the exception (the compiler honors it at any language version, but the attribute type exists only in the .NET 8 BCL).[^4]

##### Runtime and libraries (.NET 8)

- **Time abstraction**: `TimeProvider` (`TimeProvider.System`, `GetUtcNow()`, `GetTimestamp()`, `GetElapsedTime()`, `CreateTimer()`), `ITimer`, and `Task.Delay`/`CancellationTokenSource` overloads that take a `TimeProvider`.[^5]
- **Performance types**: `System.Collections.Frozen` (`FrozenDictionary<TKey,TValue>`, `FrozenSet<T>`), `System.Buffers.SearchValues<T>`, `System.Text.CompositeFormat`, `IUtf8SpanFormattable`, `Utf8.TryWrite`, `Encoding.TryGetBytes`.[^5]
- **Randomness**: `Random.GetItems`, `Random.Shuffle`, `RandomNumberGenerator.GetItems/Shuffle/GetString/GetHexString`.[^5]
- **Throw helpers**: `ArgumentException.ThrowIfNullOrWhiteSpace`, `ArgumentOutOfRangeException.ThrowIfNegative/ThrowIfZero/ThrowIfNegativeOrZero/ThrowIfGreaterThan/ThrowIfLessThan/…` (analyzers CA1510–CA1513 suggest them).[^5]
- **Async**: `CancellationTokenSource.CancelAsync()`, `ConfigureAwaitOptions` (`SuppressThrowing`, `ForceYielding`, `ContinueOnCapturedContext`).[^6]
- **System.Text.Json**: snake_case/kebab-case naming policies, `JsonUnmappedMemberHandling.Disallow`, populate mode for read-only members (`JsonObjectCreationHandling.Populate`), non-public members with `[JsonInclude]`, `JsonSerializerOptions.MakeReadOnly()`, `TypeInfoResolverChain`, `JsonStringEnumConverter<TEnum>`, `IAsyncEnumerable` streaming helpers (`GetFromJsonAsAsyncEnumerable`), `Half`/`Int128`/`UInt128`/`Memory<T>` support, an option to disable reflection (`JsonSerializerIsReflectionEnabledByDefault`).[^5]
- **Extensions**: keyed DI services (`AddKeyedSingleton`, `[FromKeyedServices]`, `GetRequiredKeyedService`), `IHostedLifecycleService`, options validation source generator (`[OptionsValidator]`), configuration-binding source generator, `LoggerMessageAttribute` shorter constructors, `IMeterFactory` and `MetricCollector<T>`.[^5]
- **Data annotations**: `[Length]`, `[AllowedValues]`, `[DeniedValues]`, `[Base64String]`, `RangeAttribute.MinimumIsExclusive/MaximumIsExclusive`.[^5]
- **Other**: SHA-3 (`SHA3_256`, `SHA3_384`, `SHA3_512`, with `IsSupported`), `System.Net.IPNetwork` (new BCL type for CIDR ranges with `Parse`/`TryParse`/`Contains`), stream-based `ZipFile` overloads, `GC.RefreshMemoryLimit()`, dynamic PGO on by default, AVX-512/`Vector512<T>`.[^5]
- **ASP.NET Core 8**: Native AOT for minimal APIs and gRPC (`WebApplication.CreateSlimBuilder`, Request Delegate Generator), `IExceptionHandler`, `[FromForm]` binding with antiforgery middleware (`UseAntiforgery`), Identity API endpoints (`MapIdentityApi<TUser>()`), keyed services in endpoints, short-circuit routing (`.ShortCircuit()`, `MapShortCircuit`), `HTTP_PORTS`/`HTTPS_PORTS`, request timeouts middleware, new built-in metrics, Blazor Web App unified model.[^7]
- **EF Core 8**: complex types (`[ComplexType]`/`ComplexProperty`), primitive collections (`List<int>` columns stored as JSON, and `Contains` over parameter collections via `OPENJSON`/`json_each`), raw SQL for unmapped types (`Database.SqlQuery<T>`), lazy loading for no-tracking queries, `HierarchyId`, `DateOnly`/`TimeOnly` on SQL Server, sentinel values for defaults.[^8]
- **Containers and SDK**: ASP.NET Core images listen on **port 8080** by default, images include a non-root `app` user, Debian 12 base, `dotnet publish`/`dotnet pack` default to **Release**, `dotnet restore` reports known-vulnerable packages (NuGet audit), `BinaryFormatter` is disabled for most project types.[^9]

##### Do not use or recommend (newer than C# 12 / .NET 8)

The project's compiler language version is C# 12 and its BCL is .NET 8. Never suggest these — they either do not compile or do not exist at run time:

**C# 13 (.NET 9, November 2024)**:[^10]
- `params` collections: `params ReadOnlySpan<T>`, `params List<T>`, `params IEnumerable<T>` (C# 12 allows only `params T[]`).
- The `System.Threading.Lock` type and `lock (myLock)` on it (.NET 9 BCL). Use `private readonly object _gate = new();` with `lock`.
- The `\e` escape sequence (use `\u001b`).
- Implicit "from the end" index `[^1] = …` inside object initializers.
- `ref` locals and `unsafe` blocks inside iterators and `async` methods.
- `allows ref struct` anti-constraint, `ref struct` types implementing interfaces.
- `partial` properties and indexers.
- `[OverloadResolutionPriority]` (.NET 9 attribute).

**C# 14 (.NET 10, November 2025)**:[^11]
- Extension members / extension blocks: `extension(IEnumerable<T> source) { public bool IsEmpty => … }`, extension properties, static extension members, extension operators.
- The `field` keyword in property accessors (`set => field = value.Trim();`).
- Null-conditional assignment: `customer?.Order = …;`, `x?.Count += 1;`.
- `nameof(List<>)` with unbound generic types.
- First-class span conversions (implicit `T[]` → `ReadOnlySpan<T>` in extension-method receivers and generic inference).
- Modifiers on implicitly typed lambda parameters: `(text, out result) => …`.
- `partial` constructors and events; user-defined compound assignment operators (`operator +=`).

**C# 15 (.NET 11, preview — RC2 as of October 2026)**:[^12]
- Collection expression arguments `[with(capacity: n), ..items]`, union types (`union Pet(Cat, Dog)`), the `closed` class modifier, extension indexers, labeled `break`/`continue` (`continue outer;`), the `unsafe(expr)` expression and pointer relaxations.

**.NET 9 BCL / ASP.NET Core 9 / EF Core 9 APIs** (absent from `net8.0`, verified by compiling against it):[^13]
- `Task.WhenEach`, `Task.WhenAll(ReadOnlySpan<Task>)` and other `params ReadOnlySpan<T>` overloads.
- LINQ `CountBy`, `AggregateBy`, `Index`.
- `Guid.CreateVersion7()`, `System.Buffers.Text.Base64Url`, generic `OrderedDictionary<TKey,TValue>`, `Convert.ToHexStringLower`, `Dictionary<,>.GetAlternateLookup<TAlternate>()`, `SearchValues.Create(string[], StringComparison)` (`SearchValues<string>`), `X509CertificateLoader`, `TimeSpan.FromMilliseconds(long, long)`.
- `JsonSerializerOptions.Web`, `JsonSerializerOptions.RespectNullableAnnotations`, `RespectRequiredConstructorParameters`, `AllowOutOfOrderMetadataProperties`, `JsonSchemaExporter`.
- ASP.NET Core 9: built-in OpenAPI document generation (`builder.Services.AddOpenApi()` / `app.MapOpenApi()` from `Microsoft.AspNetCore.OpenApi` 9), `MapStaticAssets()`, `TypedResults.InternalServerError`, `HybridCache` as a framework feature (the `Microsoft.Extensions.Caching.Hybrid` package itself does support `net8.0`).
- EF Core 9 (`UseSeeding`/`UseAsyncSeeding`, `TranslateParameterizedCollectionsToConstants`) — EF Core 9 packages **do** target `net8.0`, so these are available only if the project references EF Core 9.x.[^14]

**.NET 10 BCL / ASP.NET Core 10**: `System.Linq.AsyncEnumerable` (LINQ over `IAsyncEnumerable<T>` in the BCL), LINQ `LeftJoin`/`RightJoin`/`Shuffle`, built-in minimal API validation (`builder.Services.AddValidation()`), file-based apps (`dotnet run app.cs`).[^15]

##### Deprecated or removed in .NET 8

- **`BinaryFormatter` is disabled** for ASP.NET Core, class library, console and most other project types: `Serialize`/`Deserialize` throw `NotSupportedException` unless `EnableUnsafeBinaryFormatterSerialization` is set. Treat any use as a security finding; it is removed entirely in .NET 9.[^9]
- New obsoletions with custom diagnostic IDs (SYSLIB0047–SYSLIB0051), for example `XmlSecureResolver` (SYSLIB0047), `RSA.EncryptValue`/`DecryptValue` (SYSLIB0048), `JsonSerializerOptions.AddContext<T>()` (SYSLIB0049), the formatter-based serialization infrastructure such as `FormatterConverter`/`IFormatter` (SYSLIB0050), and the legacy serialization hooks `Exception(SerializationInfo, StreamingContext)` / `Exception.GetObjectData` (SYSLIB0051) — all verified as warnings on SDK 8.0.426.[^9]
- `JsonSerializerOptions.AddContext<TContext>()` → use `TypeInfoResolver` or `TypeInfoResolverChain`.
- `dotnet publish` and `dotnet pack` default to the `Release` configuration in .NET 8 projects.[^9]

##### Version-aware review

- **Determine the language version before judging syntax.** The default is C# 12 for `net8.0` — also when the build uses the .NET 9 or 10 SDK (verified: SDK 10.0.402 + `net8.0` without `<LangVersion>` rejects `params ReadOnlySpan<int>` with `CS9202 … not available in C# 12.0`). An explicit `<LangVersion>13</LangVersion>`, `14`, `latest` or `preview` changes that: with SDK 10 and `LangVersion latest`, extension members, the `field` keyword and null-conditional assignment compile and run on `net8.0`. Microsoft does not support language versions newer than the target framework's default; flag such settings, but do not report the syntax as a compile error when the project explicitly opts in.
- **`<LangVersion>preview</LangVersion>` on the .NET 8 SDK is not C# 12.** SDK 8.0.4xx ships the Roslyn 4.11 compiler, and `preview` there enables early C# 13 features (`params` collections, `\e`, partial properties, `^` in object initializers — all verified to compile and run on `net8.0`). Preview features are unsupported in production; flag the setting.
- **Runtime-dependent features fail regardless of `LangVersion`**: `System.Threading.Lock` (CS0234), `allows ref struct` (`CS9240: Target runtime doesn't support by-ref-like generics`), and every .NET 9+ BCL API. Say "not available in .NET 8" only for these; say "not available in C# 12" only when the project uses the default language version.
- **Verify APIs against the pinned package versions.** `Microsoft.Extensions.*` 9.x/10.x, `Microsoft.Extensions.Http.Resilience`, `Microsoft.Extensions.TimeProvider.Testing` and EF Core 9 ship `net8.0` assets; an API from such a package can be valid in a `net8.0` project. Check the package version in the `.csproj` or `Directory.Packages.props` before claiming an API is missing.
- **Multi-targeted libraries** (`<TargetFrameworks>net8.0;netstandard2.0</TargetFrameworks>`) compile the same source for each target; .NET 8-only APIs need `#if NET8_0_OR_GREATER` guards, and C# 12 syntax that needs runtime types (`InlineArray`, `[Experimental]`, collection expressions targeting `ImmutableArray<T>`) needs polyfills on `netstandard2.0`.

***

#### Project Setup

##### Recommended project file

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <!-- LangVersion defaults to 12 for net8.0; do not set latest/preview -->
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <AnalysisLevel>8.0-recommended</AnalysisLevel>
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
    <InvariantGlobalization>true</InvariantGlobalization> <!-- only if the app needs no culture data -->
  </PropertyGroup>
</Project>
```

- `TreatWarningsAsErrors` makes nullable warnings (CS86xx) build-breaking. Reviewers should flag new `#pragma warning disable` and `!` (null-forgiving) without a justification comment.
- `AnalysisLevel` `8.0-recommended` (or `latest-recommended`) enables the recommended .NET 8 CA rule set; rules worth raising to warnings include CA1510–CA1513 (throw helpers), CA1860 (`Any()` vs `Length/Count`), CA1862 (`StringComparison` overloads), CA1863 (`CompositeFormat`), CA1864 (`TryAdd` instead of `ContainsKey`+`Add`), CA1865–CA1867 (`char` overloads), CA1869 (cache `JsonSerializerOptions`), CA1870 (`SearchValues`) and CA2021 (impossible `Cast<T>`/`OfType<T>`), all present in SDK 8.0.426.[^16]
- Pin the SDK with `global.json` to a **current** 8.0 band and let it roll forward to the latest patch:

```json
{
  "sdk": {
    "version": "8.0.426",
    "rollForward": "latestPatch"
  }
}
```

- Use `Directory.Build.props` for shared settings and Central Package Management (`Directory.Packages.props` with `<ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally>`) so every project uses one version of each package.
- Since SDK 8.0.100, `dotnet restore` runs NuGet audit and warns (NU1901–NU1904) about packages with known vulnerabilities. Do not suppress these warnings without an issue reference.[^9]

##### SDK-style behavior changes that surprise reviewers

- `dotnet publish` and `dotnet pack` build `Release` by default on .NET 8; CI scripts that pass `-c Debug` intentionally ship debug builds.
- Specifying a runtime identifier (`-r linux-x64`) no longer implies `--self-contained`; add `--self-contained` explicitly when it is required.[^9]
- `.user` files are imported in outer builds; `DebugSymbols=false` disables PDB generation.

***

#### Naming and Code Style

These house rules apply unchanged in C# 12.

```csharp
var order = new Order(new FakeRepository()) { Id = 7 };
Console.WriteLine(await order.CountAsync());   // 2

// PascalCase: types, namespaces, methods, properties, events, constants
public static class RetryPolicy
{
    public const int MaxRetryCount = 3;       // constants: PascalCase, not SCREAMING_CASE
}

public sealed class Order
{
    private readonly IOrderRepository _repository;   // private fields: _camelCase
    private int _itemCount;

    public Order(IOrderRepository repository) => _repository = repository;

    public int Id { get; init; }                     // properties: PascalCase
    public bool IsActive { get; set; }               // booleans: Is/Has/Can prefix
    public bool HasDiscount { get; set; }

    public async Task<int> CountAsync(CancellationToken ct = default)   // async methods: Async suffix
    {
        _itemCount = await _repository.CountAsync(Id, ct);
        return _itemCount;
    }
}

public interface IOrderRepository             // interfaces: I prefix
{
    Task<int> CountAsync(int orderId, CancellationToken ct);
}

public sealed class FakeRepository : IOrderRepository
{
    public Task<int> CountAsync(int orderId, CancellationToken ct) => Task.FromResult(2);
}
```

- One top-level type per file; file name matches the type name. Use file-scoped namespaces (`namespace MyApp.Orders;`).
- Member order: constants, static fields, instance fields, constructors, properties, public methods, private methods.
- Seal classes that are not designed for inheritance (`sealed` also enables devirtualization and analyzer CA1852).
- Prefer `var` when the type is obvious from the right-hand side; spell out the type when it is not.
- Primary constructor parameters are named like parameters (`camelCase`, no underscore) — they are parameters, not fields (see below).
- Typical solution layout: `src/MyApp.Api`, `src/MyApp.Core`, `src/MyApp.Infrastructure`, `tests/MyApp.UnitTests`, `tests/MyApp.IntegrationTests`.

***

#### New in C# 12: Primary Constructors

Any `class` or `struct` can declare a parameter list after its name. The parameters are in scope throughout the type body. Unlike `record` primary constructors, **no public properties are generated** for a plain class or struct: a parameter that is used inside a member is *captured* into a compiler-generated, unnamed, **mutable** field.[^4][^17]

```csharp
var greeter = new Greeter("Hello");
Console.WriteLine(greeter.Greet("Ada"));   // Hello, Ada

// Typical DI usage: parameters captured directly, no field boilerplate.
public sealed class Greeter(string greeting)
{
    public string Greet(string name) => $"{greeting}, {name}";
}
```

Rules that matter in review (all verified on SDK 8.0.426):

- Adding a primary constructor removes the implicit parameterless constructor of a class. Every other constructor must chain to it with `: this(…)` (`CS8862` otherwise).
- Parameters are not members: `this.greeting` does not compile (`CS1061`).
- Parameters cannot be marked `readonly` in C# 12. In a `readonly struct`, assigning a captured parameter is an error (`CS9114`); in a class it is allowed and silently mutates the hidden field.
- In a `struct`, `default(S)` and `new S()` skip the primary constructor; captured parameters are zero/`null`.
- A parameter that is never used produces `warning CS9113: Parameter '…' is unread`.

##### Pitfall: double storage (CS9124)

Using a parameter both to initialize a property/field **and** inside a member creates two copies of the state that drift apart:

```csharp
var c = new Counter(5);
c.Increment();
c.Increment();
Console.WriteLine($"{c.Value} {c.Current}");   // 5 7  — two different "counts"

// BAD: 'start' initializes Value AND is captured by Increment/Current.
public sealed class Counter(int start)
{
    public int Value { get; private set; } = start;   // warning CS9124
    public int Current => start;
    public void Increment() => start++;                // mutates the hidden field, not Value
}
```

The compiler reports `warning CS9124: Parameter 'int start' is captured into the state of the enclosing type and its value is also used to initialize a field, property, or event`. Treat CS9124 as a bug. Pick one storage: either use only the parameter, or only the property.

```csharp
var c = new Counter(5);
c.Increment();
Console.WriteLine(c.Value);   // 6

// GOOD: the parameter only initializes state; members use the property.
public sealed class Counter(int start)
{
    public int Value { get; private set; } = start;
    public void Increment() => Value++;
}
```

##### Pitfall: capturing and passing to a base class (CS9107)

```csharp
var d = new Derived("n");
Console.WriteLine(d.Describe());

public class Base(string name)
{
    public string Name => name;
}

// BAD: 'name' is passed to Base AND captured in Derived — two copies (warning CS9107).
public class Derived(string name) : Base(name)
{
    public string Describe() => $"derived {name}";
}
```

Fix: use the base member (`Name`) in the derived class.

##### Pitfall: mutable captured state and missing validation

Captured parameters are writable fields. For services whose dependencies must never change, or where arguments need validation, assign to explicit `readonly` fields or properties in initializers — initializers run in the primary constructor:

```csharp
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;

var svc = new OrderService(new InMemoryOrders(), NullLogger<OrderService>.Instance);
Console.WriteLine(await svc.CountAsync());

try { _ = new OrderService(null!, NullLogger<OrderService>.Instance); }
catch (ArgumentNullException e) { Console.WriteLine(e.ParamName); }   // orders

// GOOD: validated, readonly storage initialized from primary constructor parameters.
public sealed class OrderService(IOrders orders, ILogger<OrderService> logger)
{
    private readonly IOrders _orders = orders ?? throw new ArgumentNullException(nameof(orders));
    private readonly ILogger<OrderService> _logger = logger;

    public async Task<int> CountAsync(CancellationToken ct = default)
    {
        var count = await _orders.CountAsync(ct);
        _logger.LogDebug("Counted {Count} orders", count);
        return count;
    }
}

public interface IOrders { Task<int> CountAsync(CancellationToken ct); }
public sealed class InMemoryOrders : IOrders { public Task<int> CountAsync(CancellationToken ct) => Task.FromResult(3); }
```

When both styles appear in one type (`_orders` field *and* uses of `orders` in methods), that is the CS9124 double-storage bug. IDE0290 ("Use primary constructor") suggests converting constructors; accept the suggestion only for types whose constructor does nothing but assign fields.

Do not convert records to plain classes with primary constructors expecting properties: `class Point(int X, int Y)` has **no** `X`/`Y` properties, no value equality and no deconstruction. Use `record`/`record struct` for data.

***

#### New in C# 12: Collection Expressions

Collection expressions create arrays, spans, lists, sets, immutable collections and interface-typed collections with one syntax. `..e` spreads an enumerable.[^4][^18]

```csharp
using System.Collections.Immutable;

int[] numbers = [1, 2, 3];
List<int> list = [.. numbers, 4];
Span<int> span = [5, 6];
ReadOnlySpan<char> chars = ['x', 'y'];
HashSet<string> set = ["a", "b"];
ImmutableArray<int> immutable = [1, 2, 3];   // via [CollectionBuilder] on .NET 8
int[][] jagged = [[1, 2], [3]];
int[] empty = [];                            // reuses Array.Empty<int>()

Console.WriteLine($"{list.Count} {span.Length} {chars.Length} {set.Count} {immutable.Length} {jagged.Length} {empty.Length}");
Console.WriteLine(ReferenceEquals(empty, Array.Empty<int>()));   // True
```

Semantics a reviewer must know:

- **No natural type**: `var x = [1, 2];` is `error CS9176: There is no target type for the collection expression`. The target type decides what is built.
- **Interface targets get a compiler-chosen type.** `IEnumerable<T>`, `IReadOnlyCollection<T>` and `IReadOnlyList<T>` receive a read-only synthesized type (`<>z__ReadOnlyArray<T>` on SDK 8.0.426) whose `ICollection<T>.Add` throws `NotSupportedException`; `ICollection<T>`/`IList<T>` receive a `List<T>`. Never downcast the result.
- **Spans cannot escape.** `static Span<int> M() => [1, 2];` is `error CS9203` (the storage may be on the stack).
- **No dictionaries.** `Dictionary<K,V> d = [];` (empty) compiles, but elements do not (`CS9215`); dictionary expressions are not part of C# 12.
- **Overload ambiguity.** `M([1])` with overloads `M(List<int>)` and `M(HashSet<int>)` is `CS0121`. With `M(ReadOnlySpan<int>)` and `M(int[])`, the span overload wins.
- **`[]` is not `null`.** Replacing `null` returns with `[]` changes behavior for callers that test `is null`.

```csharp
IEnumerable<int> view = [1, 2];
try
{
    ((ICollection<int>)view).Add(3);
}
catch (NotSupportedException)
{
    Console.WriteLine("read-only");   // printed
}
```

Spread works with any enumerable and evaluates it once. It allocates a new collection each time; in hot paths, prefer passing the existing collection.

```csharp
List<string> defaults = ["a", "b"];
string[] extra = ["c"];
string[] all = [.. defaults, .. extra, "d"];
Console.WriteLine(string.Join(",", all));   // a,b,c,d
```

IDE0300–IDE0305 suggest collection expressions in place of `new[] { … }`, `new List<T> { … }`, `Array.Empty<T>()`, `.ToList()` / `.ToArray()` on literals. These are style suggestions, not bugs; the review concern is only the interface-target and `null` semantics above.

***

#### New in C# 12: Inline Arrays

`[InlineArray(N)]` on a struct with exactly one field makes a fixed-size buffer of N elements, stored inline, without `unsafe` code. It converts implicitly to `Span<T>`/`ReadOnlySpan<T>`, supports indexing and `foreach`.[^4]

```csharp
using System.Runtime.CompilerServices;

var buffer = new Buffer8();
for (int i = 0; i < 8; i++)
{
    buffer[i] = i * i;
}

Span<int> span = buffer;            // implicit conversion
Console.WriteLine(span[7]);         // 49

int index = 8;
try
{
    buffer[index] = 1;              // bounds-checked at run time
}
catch (IndexOutOfRangeException)
{
    Console.WriteLine("out of range");
}

[InlineArray(8)]
public struct Buffer8
{
    private int _element0;
}
```

A constant out-of-range index is a compile error (`CS9166`). Inline arrays are mostly a runtime/library tool; application code rarely needs them. Review copies carefully: an inline array is a value type, so passing it by value copies all N elements.

***

#### New in C# 12: Default and `params` Lambda Parameters

```csharp
var increment = (int x, int by = 1) => x + by;
var sum = (params int[] values) => values.Sum();
Console.WriteLine(increment(1));       // 2
Console.WriteLine(increment(1, 5));    // 6
Console.WriteLine(sum(1, 2, 3));       // 6

// The default value lives in the lambda's anonymous delegate type.
// Converting to Func<int,int> drops it: warning CS9099.
Func<int, int> f = (int x = 2) => x;
Console.WriteLine(f(10));              // 10 — f() would not compile
```

Only `params T[]` is allowed; `params ReadOnlySpan<T>` is C# 13.

***

#### New in C# 12: `ref readonly` Parameters

`ref readonly` passes a read-only reference like `in`, but tells callers that a *variable* is expected. Passing a variable without a modifier gives `warning CS9192` (pass with `ref` or `in`); passing an rvalue (a constant or temporary) gives `warning CS9193`.[^4]

```csharp
var big = new Matrix4(1, 2, 3, 4);
Print(in big);    // or: Print(ref big);

static void Print(ref readonly Matrix4 m) => Console.WriteLine(m.A + m.D);

public readonly record struct Matrix4(double A, double B, double C, double D);
```

Use `in` for "large read-only value, any expression is fine"; use `ref readonly` when the callee keeps or compares the reference (for example, `Unsafe.AsPointer`, `ReadOnlySpan<T>(ref readonly T)`). Changing a public `ref` parameter to `ref readonly` is source compatible for callers; changing it to `in` is not.

***

#### New in C# 12: Alias Any Type

`using` aliases may now name tuples, arrays, pointers and generic instantiations:

```csharp
using Point = (int X, int Y);
using Names = System.Collections.Generic.List<string?>;

Point p = (1, 2);
Names names = ["a", null];
Console.WriteLine(p.X + p.Y + names.Count);   // 5
```

- `using unsafe Ptr = int*;` requires `<AllowUnsafeBlocks>true</AllowUnsafeBlocks>`.
- An alias cannot be a nullable *reference* type (`using S = string?;` is `CS9132`); nullable value types (`int?`) are allowed.
- Prefer `global using` aliases in one file (`GlobalUsings.cs`) over repeating them. A tuple alias is still a tuple: it has structural equality and no validation. Use a `record struct` when the concept deserves invariants.

***

#### New in C# 12: `ExperimentalAttribute`

Marking an API with `[Experimental("DIAG001")]` (`System.Diagnostics.CodeAnalysis`, .NET 8 BCL) makes every use a **compile error** with that diagnostic ID until the caller suppresses it. All types in an assembly marked `[assembly: Experimental(...)]` are experimental.[^4]

```csharp
using System.Diagnostics.CodeAnalysis;

#pragma warning disable DIAG001 // opt-in: we accept that FastPath may change
Console.WriteLine(FastPath.Compute(2));
#pragma warning restore DIAG001

[Experimental("DIAG001")]
public static class FastPath
{
    public static int Compute(int x) => x * 2;
}
```

Review: a suppression of an experimental diagnostic is an explicit opt-in to an unstable API — require a comment that says why, and prefer `<NoWarn>` scoped to one project over a global suppression. On `netstandard2.0` or `net7.0` the attribute type does not exist (CS0234) unless polyfilled.

***

#### New in C# 12: Interceptors (preview only)

Interceptors let a source generator replace a specific call site with a call to another method, identified by `[InterceptsLocation]`. In .NET 8 they are an **experimental preview**: using one requires the project property `<InterceptorsPreviewNamespaces>$(InterceptorsPreviewNamespaces);My.Generated</InterceptorsPreviewNamespaces>`; without it the build fails with `CS9137`.[^4][^19] They became stable in later SDKs (.NET 9.0.2xx and newer, property `InterceptorsNamespaces`).

What reviewers will see in .NET 8 projects:

- ASP.NET Core's Request Delegate Generator (enabled by Native AOT publishing or `<EnableRequestDelegateGenerator>true</EnableRequestDelegateGenerator>`) and the configuration-binding generator (`<EnableConfigurationBindingGenerator>true</EnableConfigurationBindingGenerator>`) use interceptors; the SDK adds their namespaces automatically.[^7]
- Hand-written `[InterceptsLocation]` attributes in application code are a red flag: they hard-code file paths, line and column numbers and break on the next edit.

***

#### Nullable Reference Types

With `<Nullable>enable</Nullable>`, `string` means "never null" and `string?` means "may be null". The analysis is flow-based and compile-time only.

```csharp
var repo = new UserRepository();
Console.WriteLine(repo.Find(1)?.Name ?? "unknown");    // Ada
Console.WriteLine(repo.Find(2)?.Name ?? "unknown");    // unknown

if (repo.TryFind(1, out var user))
{
    Console.WriteLine(user.Name.Length);               // no warning: NotNullWhen(true)
}

public sealed record User(int Id, string Name);

public sealed class UserRepository
{
    private readonly Dictionary<int, User> _users = new() { [1] = new User(1, "Ada") };

    // Return T? when absence is a normal outcome.
    public User? Find(int id) => _users.GetValueOrDefault(id);

    public bool TryFind(int id, [System.Diagnostics.CodeAnalysis.NotNullWhen(true)] out User? user)
        => _users.TryGetValue(id, out user);
}
```

Review rules:

- **The null-forgiving operator `!` is an assertion, not a check.** `GetUser()!` silences the warning and produces a `NullReferenceException` later. Require a comment or a real check (`?? throw`, `ArgumentNullException.ThrowIfNull`).
- **`= null!;` on properties** is acceptable only for members that a framework sets after construction (EF navigation properties, options classes bound from configuration). For DTOs prefer `required` members.
- **Boundaries are not checked.** JSON deserialization, EF materialization, `Activator.CreateInstance`, reflection and arrays (`new string[3]` contains nulls) ignore annotations. Validate public API arguments with `ArgumentNullException.ThrowIfNull(arg)` even in nullable-enabled code — callers may have nullable disabled.
- Use the attributes in `System.Diagnostics.CodeAnalysis` (`[NotNullWhen]`, `[MaybeNullWhen]`, `[NotNull]`, `[MemberNotNull]`, `[DoesNotReturn]`) instead of `!` in helpers.
- `#nullable disable` in new files and `<Nullable>disable</Nullable>` in new projects are review findings.
- `required` (C# 11) forces object-initializer assignment at compile time; it does not prevent `null` being assigned to a `string?`, and System.Text.Json treats `required` properties as required in JSON (missing → `JsonException`).

```csharp
var ok = new CreateUser { Email = "a@example.com", DisplayName = "Ada" };
Console.WriteLine(ok.Email);

// var bad = new CreateUser { DisplayName = "x" };   // error CS9035: required member 'Email' must be set

public sealed class CreateUser
{
    public required string Email { get; init; }
    public required string DisplayName { get; init; }
    public string? Phone { get; init; }
}
```

***

#### Types: Classes, Records, Structs

##### Records

`record` (class) and `record struct` generate value equality, `ToString`, deconstruction and `with`. Positional records generate `init` properties (`record class`) or mutable properties (`record struct`; use `readonly record struct` for immutability).

```csharp
var a = new Money(10m, "EUR");
var b = a with { Amount = 20m };
Console.WriteLine(a == new Money(10m, "EUR"));   // True — value equality
Console.WriteLine(b);                            // Money { Amount = 20, Currency = EUR }

var o1 = new OrderLines([1, 2]);
var o2 = new OrderLines([1, 2]);
Console.WriteLine(o1 == o2);                     // False — List<int> compares by reference

var o3 = o1 with { };                            // shallow copy: shares the same list
o3.Lines.Add(3);
Console.WriteLine(o1.Lines.Count);               // 3 — mutation visible through o1

public readonly record struct Money(decimal Amount, string Currency);
public sealed record OrderLines(List<int> Lines);
```

Review:
- Records with collection properties do **not** have deep equality, and `with` copies references. Use immutable collections (`ImmutableArray<T>`) and implement equality explicitly if needed.
- Do not use records as EF Core entities that are tracked and mutated: value equality and `with` clones confuse identity resolution. Records are fine for DTOs, messages, value objects, query results.
- Mutable `record struct` copies on every assignment; prefer `readonly record struct`.

##### Classes and structs

- Make classes `sealed` by default; open them for inheritance only by design.
- Structs: small (≤ 16–24 bytes), immutable (`readonly struct`), no default-value surprises. Never expose mutable structs through properties (`list[i].X = 1` modifies a copy or fails to compile).
- Override `Equals` and `GetHashCode` together (or use records). `GetHashCode` must not depend on mutable state if instances are dictionary keys.
- `init` accessors allow object-initializer assignment only. `with` on records can still set them.

***

#### Pattern Matching

```csharp
Console.WriteLine(Discount(new Premium(1000m)));             // 200.0 (decimal keeps scale)
Console.WriteLine(Discount(new Regular(1000m, 6)));          // 100.0
Console.WriteLine(Shipping(new Order(150m, true)));          // Express
Console.WriteLine(Classify([1, 2, 3]));                      // starts with 1, ends with 3
Console.WriteLine(Classify([]));                             // empty

static decimal Discount(Customer customer) => customer switch
{
    Premium p => p.Total * 0.2m,
    Regular { Years: > 5 } r => r.Total * 0.1m,
    Regular r => r.Total * 0.05m,
    _ => throw new ArgumentOutOfRangeException(nameof(customer)),
};

static string Shipping(Order order) => order switch
{
    { Total: > 100m, IsPriority: true } => "Express",
    { Total: > 100m } => "Standard",
    { IsPriority: true } => "Priority",
    _ => "Economy",
};

static string Classify(int[] values) => values switch
{
    [] => "empty",
    [var only] => $"single {only}",
    [1, .., 3] => "starts with 1, ends with 3",
    [_, _, ..] => "two or more",
};

public abstract record Customer(decimal Total);
public sealed record Premium(decimal Total) : Customer(Total);
public sealed record Regular(decimal Total, int Years) : Customer(Total);
public sealed record Order(decimal Total, bool IsPriority);
```

Review:
- A `switch` expression without a discard arm over a non-exhaustive input warns `CS8509` and throws `SwitchExpressionException` at run time. Enums are never exhaustive (any integer can be cast to an enum): add `_ => throw new ArgumentOutOfRangeException(...)` or `UnreachableException` (.NET 7+).
- `is null` / `is not null` cannot be redirected by user-defined `==` operators; prefer them over `== null`.
- Order matters: a broader pattern before a narrower one makes the narrower arm unreachable (`CS8510`).
- A constant pattern needs a constant: `x is SomeStaticField` does not compile; use `x == field` or a relational pattern.

***

#### Strings, Formatting and Culture

```csharp
using System.Globalization;
using System.Text;

CultureInfo.CurrentCulture = new CultureInfo("tr-TR");
Console.WriteLine("title".ToUpper());                                              // TİTLE — culture-sensitive
Console.WriteLine("title".ToUpperInvariant());                                     // TITLE
Console.WriteLine("TITLE".Equals("title", StringComparison.CurrentCultureIgnoreCase)); // False in tr-TR
Console.WriteLine("TITLE".Equals("title", StringComparison.OrdinalIgnoreCase));    // True

CultureInfo.CurrentCulture = new CultureInfo("de-DE");
Console.WriteLine(double.Parse("1.5"));                                // 15 — '.' is a group separator in de-DE
Console.WriteLine(double.Parse("1.5", CultureInfo.InvariantCulture).ToString(CultureInfo.InvariantCulture)); // 1.5
Console.WriteLine($"{1.5}");                                           // 1,5
Console.WriteLine(string.Create(CultureInfo.InvariantCulture, $"{1.5}"));  // 1.5

// New in .NET 8: CompositeFormat for format strings loaded at run time.
CompositeFormat format = CompositeFormat.Parse("Range {0}..{1}");
Console.WriteLine(string.Format(CultureInfo.InvariantCulture, format, 1, 9));

// C# 11 raw string literals and UTF-8 literals remain available.
string json = """{"name": "Ada"}""";
ReadOnlySpan<byte> utf8 = "Ada"u8;
Console.WriteLine($"{json.Length} {utf8.Length}");
```

Review rules:
- Identifiers, protocol tokens, file paths, dictionary keys, headers: use `StringComparison.Ordinal`/`OrdinalIgnoreCase` and `StringComparer.Ordinal(IgnoreCase)`. Analyzer CA1862 flags `a.ToLower() == b.ToLower()`; CA1309/CA1310 flag culture-sensitive comparisons.
- Parsing and formatting of machine-readable data (JSON you build by hand, CSV, logs, SQL literals): pass `CultureInfo.InvariantCulture` (CA1304/CA1305).
- `InvariantGlobalization=true` makes all cultures behave as invariant — correct for many services, wrong for user-facing formatting.
- **New in .NET 8**: the default `TwoDigitYearMax` changed from 2029 to 2049, so parsing `"01/01/49"` with format `yy` yields 2049 on .NET 8 (1949 on .NET 7) — verified.[^9] Never parse two-digit years from external input.
- Build strings in loops with `StringBuilder` or `string.Join`/`string.Concat`, not `+=`.

***

#### Exceptions and Guard Clauses

```csharp
try
{
    Transfer(from: null!, to: "b", amount: 5m);
}
catch (ArgumentNullException ex)
{
    Console.WriteLine(ex.ParamName);           // from
}

try
{
    Transfer("a", "b", amount: -1m);
}
catch (ArgumentOutOfRangeException ex)
{
    Console.WriteLine(ex.ParamName);           // amount
}

static void Transfer(string from, string to, decimal amount)
{
    ArgumentNullException.ThrowIfNull(from);                 // .NET 6
    ArgumentException.ThrowIfNullOrWhiteSpace(to);           // New in .NET 8
    ArgumentOutOfRangeException.ThrowIfNegativeOrZero(amount); // New in .NET 8
    // CallerArgumentExpression fills in the parameter name automatically.
}
```

- The .NET 8 throw helpers `ArgumentOutOfRangeException.ThrowIfZero/ThrowIfNegative/ThrowIfNegativeOrZero/ThrowIfEqual/ThrowIfNotEqual/ThrowIfGreaterThan/ThrowIfGreaterThanOrEqual/ThrowIfLessThan/ThrowIfLessThanOrEqual` and `ArgumentException.ThrowIfNullOrWhiteSpace` replace hand-written `if (…) throw` (CA1510–CA1513). `ArgumentException.ThrowIfNullOrEmpty` and `ObjectDisposedException.ThrowIf` exist since .NET 7.
- Re-throw with `throw;` — `throw ex;` resets the stack trace (CA2200).
- Catch specific exceptions. `catch (Exception)` is acceptable at process/request boundaries that log and translate; elsewhere it hides bugs (CA1031).
- Use exception filters for conditional handling: `catch (HttpRequestException ex) when (ex.StatusCode == HttpStatusCode.NotFound)`.
- Do not use exceptions for expected control flow (parsing user input → `TryParse`).
- Wrap with context: `throw new OrderProcessingException($"Order {id} failed", ex);` — keep the inner exception.
- Custom exceptions: derive from `Exception`, provide the standard constructors, and do **not** add `[Serializable]` plus the `(SerializationInfo, StreamingContext)` constructor in new code — that constructor and `GetObjectData` are obsolete in .NET 8 (SYSLIB0051).[^9]

```csharp
try
{
    throw new OrderProcessingException("Order 42 failed", new TimeoutException("db"));
}
catch (OrderProcessingException ex) when (ex.InnerException is TimeoutException)
{
    Console.WriteLine($"{ex.Message} <- {ex.InnerException.Message}");
}

public sealed class OrderProcessingException : Exception
{
    public OrderProcessingException() { }
    public OrderProcessingException(string message) : base(message) { }
    public OrderProcessingException(string message, Exception inner) : base(message, inner) { }
}
```

***

#### Async/Await

##### Basic rules

```csharp
var service = new UserService(new FakeUsers());
var user = await service.GetUserAsync(1, CancellationToken.None);
Console.WriteLine(user.Name);

try
{
    await service.GetUserAsync(99, CancellationToken.None);
}
catch (KeyNotFoundException ex)
{
    Console.WriteLine(ex.Message);
}

public sealed record User(int Id, string Name);

public interface IUsers
{
    Task<User?> FindAsync(int id, CancellationToken ct);
}

public sealed class FakeUsers : IUsers
{
    public async Task<User?> FindAsync(int id, CancellationToken ct)
    {
        await Task.Delay(1, ct);
        return id == 1 ? new User(1, "Ada") : null;
    }
}

public sealed class UserService(IUsers users)
{
    // Async all the way; accept and forward a CancellationToken.
    public async Task<User> GetUserAsync(int id, CancellationToken ct)
    {
        var user = await users.FindAsync(id, ct);
        return user ?? throw new KeyNotFoundException($"User {id} not found");
    }
}
```

House rules (unchanged in .NET 8):

- **Never block on async code**: `.Result`, `.Wait()`, `.GetAwaiter().GetResult()` on an incomplete task. In UI apps and legacy ASP.NET this deadlocks; in ASP.NET Core it causes thread-pool starvation under load. Use `await` all the way up (CA1849 flags sync calls with async alternatives in async methods).
- **No `async void`** except event handlers. An exception escaping an `async void` method is raised on the synchronization context or thread pool and **crashes the process**. Event handlers must catch everything:

```csharp
var form = new FakeForm();
form.Clicked += form.OnClickAsync;
form.RaiseClick();
await Task.Delay(50);
Console.WriteLine(form.LastError);   // boom

public sealed class FakeForm
{
    public event EventHandler? Clicked;
    public string? LastError { get; private set; }

    public void RaiseClick() => Clicked?.Invoke(this, EventArgs.Empty);

    // async void is only acceptable for event handlers — and it must not let exceptions escape.
    public async void OnClickAsync(object? sender, EventArgs e)
    {
        try
        {
            await ProcessAsync();
        }
        catch (Exception ex)
        {
            LastError = ex.Message;
        }
    }

    private static async Task ProcessAsync()
    {
        await Task.Yield();
        throw new InvalidOperationException("boom");
    }
}
```

- Use `ConfigureAwait(false)` in **library** code that may run under a synchronization context (WPF, WinForms, MAUI, legacy ASP.NET). ASP.NET Core and console apps have no synchronization context, so it is unnecessary in application code there — but harmless (CA2007 enforces it when enabled for libraries).
- Async methods end in `Async` and return `Task`, `Task<T>`, `ValueTask`, `ValueTask<T>` or `IAsyncEnumerable<T>`.
- Elide `async`/`await` (return the task directly) only in trivial pass-through methods with no `using`, `try` or argument validation — otherwise exceptions surface at a different time and disposables are disposed too early.
- `ValueTask`/`ValueTask<T>`: await exactly once; never await twice, never call `.Result` before completion, never `Task.WhenAll` them without `.AsTask()`. Return `ValueTask` only from hot paths that usually complete synchronously.
- Fire-and-forget (`_ = DoAsync();`) loses exceptions. Use a hosted service, a queue (`Channel<T>`) or at least a continuation that logs failures.

##### Running independent operations concurrently

```csharp
var dashboard = await LoadDashboardAsync(CancellationToken.None);
Console.WriteLine($"{dashboard.Users} {dashboard.Orders}");

try
{
    await Task.WhenAll(FailAsync("a"), FailAsync("b"));
}
catch (Exception ex)
{
    // await rethrows only the FIRST exception; inspect the task for all of them.
    Console.WriteLine(ex.Message);   // a
}

var all = Task.WhenAll(FailAsync("a"), FailAsync("b"));
try { await all; } catch { }
Console.WriteLine(all.Exception!.InnerExceptions.Count);   // 2

static async Task<(int Users, int Orders)> LoadDashboardAsync(CancellationToken ct)
{
    Task<int> usersTask = CountAsync(10, ct);
    Task<int> ordersTask = CountAsync(20, ct);
    await Task.WhenAll(usersTask, ordersTask);
    return (await usersTask, await ordersTask);   // already completed: no extra wait
}

static async Task<int> CountAsync(int value, CancellationToken ct)
{
    await Task.Delay(10, ct);
    return value;
}

static async Task FailAsync(string message)
{
    await Task.Yield();
    throw new InvalidOperationException(message);
}
```

Throttle fan-out. `Parallel.ForEachAsync` (.NET 6+) is the simplest bounded loop; `SemaphoreSlim` works for ad-hoc fan-out:

```csharp
int[] items = Enumerable.Range(1, 20).ToArray();
int processed = 0;

await Parallel.ForEachAsync(
    items,
    new ParallelOptions { MaxDegreeOfParallelism = 4, CancellationToken = CancellationToken.None },
    async (item, ct) =>
    {
        await Task.Delay(5, ct);
        Interlocked.Increment(ref processed);
    });
Console.WriteLine(processed);   // 20

using var gate = new SemaphoreSlim(4);   // max 4 concurrent
var tasks = items.Select(async item =>
{
    await gate.WaitAsync();
    try
    {
        await Task.Delay(5);
        return item * 2;
    }
    finally
    {
        gate.Release();
    }
});
int[] doubled = await Task.WhenAll(tasks);
Console.WriteLine(doubled.Sum());   // 420
```

`Parallel.ForEach` (synchronous) with an `async` lambda is a bug: the lambda becomes `async void` and the loop returns before the work finishes.

##### New in .NET 8: `ConfigureAwaitOptions` and `CancelAsync`

```csharp
// SuppressThrowing: wait for completion without throwing (non-generic Task only).
Task failing = Task.FromException(new InvalidOperationException());
await failing.ConfigureAwait(ConfigureAwaitOptions.SuppressThrowing);
Console.WriteLine(failing.IsFaulted);   // True — observed without an exception

// ForceYielding: always continue asynchronously (like Task.Yield, but composable).
await Task.CompletedTask.ConfigureAwait(ConfigureAwaitOptions.ForceYielding);

// SuppressThrowing on Task<T> is rejected at run time (analyzer CA2261 catches it at build time).
try
{
    await Task.FromResult(1).ConfigureAwait(ConfigureAwaitOptions.SuppressThrowing);
}
catch (ArgumentOutOfRangeException)
{
    Console.WriteLine("not supported for Task<T>");
}

// CancelAsync runs registered callbacks without blocking the caller on them.
using var cts = new CancellationTokenSource();
cts.Token.Register(() => Console.WriteLine("callback"));
await cts.CancelAsync();
```

`ConfigureAwait(false)` is equivalent to `ConfigureAwait(ConfigureAwaitOptions.None)`; `ConfigureAwait(true)` to `ContinueOnCapturedContext`.[^6] Prefer `await cts.CancelAsync()` inside async code that cancels its own work; `Cancel()` runs all callbacks synchronously on the calling thread.

***

#### Cancellation and Timeouts

```csharp
using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(50));
try
{
    await SlowAsync(cts.Token);
}
catch (OperationCanceledException) when (cts.IsCancellationRequested)
{
    Console.WriteLine("timed out");
}

// Combine a caller token with a local timeout.
static async Task<string> FetchWithTimeoutAsync(Func<CancellationToken, Task<string>> fetch, CancellationToken ct)
{
    using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
    timeout.CancelAfter(TimeSpan.FromSeconds(5));
    return await fetch(timeout.Token);
}

Console.WriteLine(await FetchWithTimeoutAsync(async t => { await Task.Delay(1, t); return "ok"; }, CancellationToken.None));

// WaitAsync (.NET 6+): stop *waiting*; the underlying operation keeps running.
try
{
    await Task.Delay(Timeout.Infinite).WaitAsync(TimeSpan.FromMilliseconds(20));
}
catch (TimeoutException)
{
    Console.WriteLine("WaitAsync timeout");
}

static async Task SlowAsync(CancellationToken ct)
{
    for (int i = 0; i < 100; i++)
    {
        ct.ThrowIfCancellationRequested();
        await Task.Delay(10, ct);
    }
}
```

Review rules:
- Public async APIs accept a `CancellationToken` (last parameter, `= default` for public APIs) and **forward** it to every async call (CA2016). ASP.NET Core handlers receive `HttpContext.RequestAborted` by binding a `CancellationToken` parameter.
- Dispose `CancellationTokenSource` instances created with timers (`CancelAfter`, `new CancellationTokenSource(TimeSpan)`) and linked sources — they hold timer and registration resources.
- Catch `OperationCanceledException` (base of `TaskCanceledException`), not `TaskCanceledException`. Filter with `when (ct.IsCancellationRequested)` to distinguish "we cancelled" from "a dependency timed out" (`HttpClient` timeouts surface as `TaskCanceledException` with an inner `TimeoutException`).
- Do not swallow cancellation in loops: `catch (Exception)` must rethrow `OperationCanceledException` or come after a dedicated `catch (OperationCanceledException) { throw; }`.
- Never pass `CancellationToken.None` deep inside request handling when a real token is available.

***

#### `IAsyncEnumerable<T>`

```csharp
using System.Runtime.CompilerServices;

using var cts = new CancellationTokenSource();
try
{
    await foreach (var n in ReadNumbersAsync().WithCancellation(cts.Token))
    {
        Console.Write($"{n} ");
        if (n == 2) cts.Cancel();   // the next MoveNextAsync observes cancellation
    }
}
catch (OperationCanceledException)
{
    Console.WriteLine("cancelled");   // output: 0 1 2 cancelled
}

static async IAsyncEnumerable<int> ReadNumbersAsync(
    [EnumeratorCancellation] CancellationToken ct = default)   // without the attribute: warning CS8425
{
    for (int i = 0; i < 5; i++)
    {
        await Task.Delay(1, ct);   // throws OperationCanceledException once cancelled
        yield return i;
    }
}
```

Cancellation surfaces as `OperationCanceledException` out of the `await foreach` — handle it where the loop is started.

- `[EnumeratorCancellation]` is required for `WithCancellation(token)` to flow into the iterator; without it the token passed to `WithCancellation` is ignored (CS8425 warns).
- LINQ over `IAsyncEnumerable<T>` is **not** in the .NET 8 BCL; it comes from the `System.Linq.Async` package (the BCL `System.Linq.AsyncEnumerable` is .NET 10).
- ASP.NET Core 8 and `System.Text.Json` stream `IAsyncEnumerable<T>` results; `HttpClient.GetFromJsonAsAsyncEnumerable<T>` (**new in .NET 8**) consumes a JSON array as a stream.
- `await using` + `ConfigureAwait(false)` for async enumerators in libraries: `await foreach (var x in source.ConfigureAwait(false))`.

***

#### Concurrency and Thread Safety

```csharp
using System.Collections.Concurrent;
using System.Threading.Channels;

var counter = new SafeCounter();
Parallel.For(0, 1000, _ => counter.Increment());
Console.WriteLine(counter.Value);   // 1000

// GetOrAdd's factory can run more than once under contention; Lazy<T> makes the
// expensive creation run once per key.
var cache = new ConcurrentDictionary<string, Lazy<Expensive>>();
int created = 0;
Parallel.For(0, 100, _ =>
    cache.GetOrAdd("k", key => new Lazy<Expensive>(() =>
    {
        Interlocked.Increment(ref created);
        return new Expensive(key);
    })).Value.ToString());
Console.WriteLine(created);   // 1

// Bounded channel = producer/consumer queue with back-pressure.
var channel = Channel.CreateBounded<int>(new BoundedChannelOptions(10)
{
    FullMode = BoundedChannelFullMode.Wait,
});
var consumer = Task.Run(async () =>
{
    int sum = 0;
    await foreach (var item in channel.Reader.ReadAllAsync())
    {
        sum += item;
    }
    return sum;
});
for (int i = 1; i <= 100; i++)
{
    await channel.Writer.WriteAsync(i);
}
channel.Writer.Complete();
Console.WriteLine(await consumer);   // 5050

public sealed class SafeCounter
{
    private readonly object _gate = new();   // C# 12/.NET 8: lock on a private object, not System.Threading.Lock
    private int _value;

    public void Increment()
    {
        lock (_gate)
        {
            _value++;
        }
    }

    public int Value
    {
        get { lock (_gate) { return _value; } }
    }
}

public sealed record Expensive(string Key);
```

Review rules:
- Lock on a private `readonly object`; never on `this`, a `Type`, a string or a public object. `System.Threading.Lock` does not exist in .NET 8.
- `await` inside `lock` does not compile (`CS1996`). For async critical sections use `SemaphoreSlim(1, 1)` with `WaitAsync` and `try/finally Release()`.
- `Dictionary<,>`, `List<T>` and `HashSet<T>` are not thread-safe for concurrent writes (or writes concurrent with reads). Corruption can produce infinite loops in `Dictionary`. Use `ConcurrentDictionary`, immutable/frozen snapshots, or locking.
- `ConcurrentDictionary.GetOrAdd(key, factory)` may invoke the factory concurrently for the same key; `AddOrUpdate` may call the update delegate repeatedly. Factories must be side-effect free (or wrapped in `Lazy<T>`).
- `Interlocked` for counters; `volatile` is not a substitute for synchronization of compound operations.
- `static` mutable fields in services are shared across requests: flag them.
- `Task.Run` in ASP.NET Core request code just moves work to another pool thread; it does not add throughput. Use it to offload CPU-bound work from UI threads or to start background loops.

***

#### New in .NET 8: `TimeProvider`

`TimeProvider` abstracts the clock and timers so code is testable without `Thread.Sleep` or wall-clock waits.[^5]

```csharp
using Microsoft.Extensions.Time.Testing;   // package Microsoft.Extensions.TimeProvider.Testing

var fake = new FakeTimeProvider(new DateTimeOffset(2026, 1, 1, 0, 0, 0, TimeSpan.Zero));
var tokens = new TokenService(fake);
var token = tokens.Issue();
Console.WriteLine(tokens.IsValid(token));   // True

fake.Advance(TimeSpan.FromMinutes(16));
Console.WriteLine(tokens.IsValid(token));   // False

// Delays and timeouts can use the provider too.
var delay = Task.Delay(TimeSpan.FromSeconds(30), fake);
fake.Advance(TimeSpan.FromSeconds(30));
await delay;
Console.WriteLine("delay completed without waiting");

public sealed record AccessToken(string Value, DateTimeOffset ExpiresAt);

public sealed class TokenService(TimeProvider clock)
{
    private static readonly TimeSpan Lifetime = TimeSpan.FromMinutes(15);

    public AccessToken Issue() => new(Guid.NewGuid().ToString("N"), clock.GetUtcNow() + Lifetime);

    public bool IsValid(AccessToken token) => clock.GetUtcNow() < token.ExpiresAt;
}
```

- Inject `TimeProvider` (register `services.AddSingleton(TimeProvider.System)`) instead of reading `DateTime.UtcNow`/`DateTimeOffset.UtcNow` in business logic.
- Measure durations with `TimeProvider.GetTimestamp()`/`GetElapsedTime()` or `Stopwatch.GetTimestamp()`/`Stopwatch.GetElapsedTime()` (.NET 7), never by subtracting `DateTime.Now` values.
- `DateTime.Now` in server code is a smell: store and compare UTC (`DateTimeOffset.UtcNow`), convert to local time only for display.
- `new PeriodicTimer(period, timeProvider)`, `new CancellationTokenSource(delay, timeProvider)` and `Task.Delay(delay, timeProvider)`/`Task.WaitAsync(timeout, timeProvider)` accept a provider in .NET 8.
- For libraries targeting older frameworks, the `Microsoft.Bcl.TimeProvider` package provides the type.

***

#### Collections

##### Choosing a collection

| Need | Type |
| --- | --- |
| Ordered, growable | `List<T>` |
| Key lookup, mutable | `Dictionary<TKey,TValue>` (pass a comparer for string keys) |
| Key lookup, built once, read many times | **New in .NET 8**: `FrozenDictionary<TKey,TValue>` / `FrozenSet<T>` |
| Value semantics, thread-safe snapshots | `ImmutableArray<T>`, `ImmutableDictionary<TKey,TValue>` |
| Concurrent writers | `ConcurrentDictionary<TKey,TValue>`, `ConcurrentQueue<T>`, `Channel<T>` |
| Priority scheduling | `PriorityQueue<TElement,TPriority>` (.NET 6) |
| Search for any of a fixed set of chars/bytes | **New in .NET 8**: `SearchValues<T>` |
| Public API return/parameter | `IReadOnlyList<T>`, `IReadOnlyCollection<T>`, `IEnumerable<T>` |

```csharp
using System.Buffers;
using System.Collections.Frozen;

// Frozen collections: expensive to create, very fast to read. Build once (startup), reuse.
FrozenDictionary<string, int> limits = new Dictionary<string, int>
{
    ["free"] = 10,
    ["pro"] = 100,
}.ToFrozenDictionary(StringComparer.OrdinalIgnoreCase);
Console.WriteLine(limits["PRO"]);                         // 100

FrozenSet<string> reserved = new[] { "admin", "root" }.ToFrozenSet(StringComparer.OrdinalIgnoreCase);
Console.WriteLine(reserved.Contains("Admin"));            // True

// SearchValues: precomputed lookup for IndexOfAny/ContainsAny. Store in a static readonly field.
Console.WriteLine(Validator.HasForbiddenChars("file<name>"));   // True

// Mutating a collection while enumerating it throws.
var list = new List<int> { 1, 2, 3 };
try
{
    foreach (var x in list)
    {
        if (x == 2) list.Remove(x);
    }
}
catch (InvalidOperationException)
{
    Console.WriteLine("collection modified");
}
list.RemoveAll(x => x == 2);                              // GOOD

// TryGetValue/TryAdd instead of ContainsKey + indexer/Add (CA1854, CA1864).
var counts = new Dictionary<string, int>(StringComparer.Ordinal);
foreach (var word in new[] { "a", "b", "a" })
{
    counts[word] = counts.TryGetValue(word, out var c) ? c + 1 : 1;
}
Console.WriteLine(counts["a"]);                           // 2

public static class Validator
{
    private static readonly SearchValues<char> Forbidden = SearchValues.Create("<>:\"|?*");

    public static bool HasForbiddenChars(string name) => name.AsSpan().IndexOfAny(Forbidden) >= 0;
}
```

Review rules:
- `FrozenDictionary`/`FrozenSet` creation is slower than `Dictionary`; creating one per request is a performance bug. They are for static or rarely rebuilt lookup data.
- Dictionary enumeration order is unspecified; do not depend on it in output, hashing or tests.
- Pass `StringComparer.Ordinal`/`OrdinalIgnoreCase` explicitly for string keys that are identifiers; the default comparer is ordinal and case-sensitive.
- Return `IReadOnlyList<T>`/`IReadOnlyCollection<T>` from public APIs instead of `List<T>` (CA1002); do not return arrays that callers can mutate when they are cached internally.
- Return empty collections (`[]`, `Array.Empty<T>()`), never `null`, for "no items".
- `ImmutableArray<T>` is a struct: `default(ImmutableArray<T>)` is not empty — `.Length` throws `NullReferenceException`. Check `IsDefault` or initialize with `[]`.
- `SearchValues<string>` (multi-string search) is .NET 9; in .NET 8 `SearchValues.Create` accepts only `ReadOnlySpan<char>` or `ReadOnlySpan<byte>`.

***

#### LINQ

House rules (still correct): prefer method syntax for simple pipelines and query syntax for joins/groupings; project early; avoid multiple enumeration; `FirstOrDefault` when absence is expected and handled, `First`/`Single` when absence is a bug.

```csharp
var users = new List<User>
{
    new("ada", true, "Eng"),
    new("bob", false, "Ops"),
    new("cyd", true, "Eng"),
};

// Deferred execution: the query runs when enumerated, against the CURRENT state.
var active = users.Where(u => u.IsActive);
users.Add(new User("dee", true, "Ops"));
Console.WriteLine(active.Count());          // 3 — includes "dee"

// Materialize once when the result is used more than once (CA1851).
List<User> snapshot = users.Where(u => u.IsActive).ToList();
if (snapshot.Count > 0)                     // property on a materialized list (CA1829)
{
    Console.WriteLine(string.Join(",", snapshot.Select(u => u.Name)));
}

// Any() on lazy sequences; Count/Length properties on collections (CA1827, CA1860).
IEnumerable<int> lazy = Enumerable.Range(0, 1_000_000).Where(i => i % 2 == 0);
Console.WriteLine(lazy.Any());

// ThenBy, not a second OrderBy (which discards the first ordering).
var sorted = users.OrderBy(u => u.Department).ThenBy(u => u.Name).Select(u => u.Name);
Console.WriteLine(string.Join(",", sorted));   // ada,cyd,bob,dee

var byDepartment = users
    .GroupBy(u => u.Department)
    .Select(g => new { Department = g.Key, Count = g.Count() });
foreach (var g in byDepartment)
{
    Console.WriteLine($"{g.Department}: {g.Count}");
}

// .NET 6+ operators: DistinctBy, MaxBy, Chunk, TryGetNonEnumeratedCount.
Console.WriteLine(users.MaxBy(u => u.Name)!.Name);   // dee
foreach (var batch in users.Chunk(3))
{
    Console.WriteLine(batch.Length);                 // 3, then 1
}

// First/Single throw on empty (InvalidOperationException); Single also on duplicates.
try
{
    _ = users.Single(u => u.Department == "Eng");
}
catch (InvalidOperationException)
{
    Console.WriteLine("more than one");
}

public sealed record User(string Name, bool IsActive, string Department);
```

Pitfalls:
- **Side effects inside `Select`/`Where`** run once per enumeration (and not at all if never enumerated).
- **Closures over loop variables** are safe in `foreach` (each iteration has a new variable) but a captured variable that changes before enumeration changes the query.
- **New in .NET 8**: `Enumerable.Sum` over `int[]`/`List<int>` (span-able sources) is vectorized and adds elements in a different order. Inputs whose intermediate sums overflow can now throw `OverflowException` where .NET 7 returned a value — and vice versa (verified: 32 × 100,000,000 followed by 32 × −100,000,000 returns 0 on .NET 7 for an `IEnumerable` but throws on .NET 8; as an array it throws on .NET 7 and returns 0 on .NET 8). Use `long` accumulation for values near `int` limits.[^9]
- LINQ in hot paths allocates enumerators and delegates; loops over arrays/spans are cheaper. Measure before rewriting.
- `.ToList()` immediately followed by `foreach` is an unnecessary copy unless the source can change during iteration.
- `Cast<T>()`/`OfType<T>()` with a type that can never match is flagged by CA2021 (**new analyzer in .NET 8**).
- `IQueryable<T>` (EF Core) and `IEnumerable<T>` look the same; calling an `IEnumerable` method or `AsEnumerable()` early moves filtering to the client (see EF Core).
- `CountBy`, `AggregateBy`, `Index` (.NET 9) and `LeftJoin`/`RightJoin` (.NET 10) are not available.

***

#### `Span<T>`, `Memory<T>` and Pooling

```csharp
using System.Buffers;

// Parse without allocating substrings.
ReadOnlySpan<char> line = "id=42;name=ada";
int eq = line.IndexOf('=');
int semi = line.IndexOf(';');
int id = int.Parse(line[(eq + 1)..semi]);
Console.WriteLine(id);   // 42

// .NET 8: split a span into ranges without allocating strings.
ReadOnlySpan<char> csv = "a,b,c";
Span<Range> parts = stackalloc Range[4];
int count = csv.Split(parts, ',');
Console.WriteLine($"{count} {csv[parts[1]]}");   // 3 b

// stackalloc only for small, bounded sizes; fall back to the pool.
Console.WriteLine(Checksum(new byte[100]));
Console.WriteLine(Checksum(new byte[10_000]));

static int Checksum(ReadOnlySpan<byte> data)
{
    const int StackLimit = 256;
    byte[]? rented = null;
    Span<byte> buffer = data.Length <= StackLimit
        ? stackalloc byte[StackLimit]
        : (rented = ArrayPool<byte>.Shared.Rent(data.Length));
    try
    {
        data.CopyTo(buffer);
        int sum = 0;
        foreach (byte b in buffer[..data.Length]) sum += b;
        return sum;
    }
    finally
    {
        if (rented is not null)
        {
            ArrayPool<byte>.Shared.Return(rented, clearArray: true);   // clear if it held secrets
        }
    }
}
```

Rules:
- `Span<T>`/`ReadOnlySpan<T>` are `ref struct`s: they cannot be fields of classes, captured by lambdas, boxed, used across `await`, or used as locals in `async` methods or iterators in C# 12 (SDK 8.0.4xx reports this as `CS8652 … 'ref and unsafe in async and iterator methods' is currently in Preview` — that relaxation is C# 13). Use `Memory<T>`/`ReadOnlyMemory<T>` in async code.
- `stackalloc` with a size taken from input is a stack-overflow DoS. Bound it (≤ ~1 KB) and fall back to `ArrayPool<T>`.
- `ArrayPool<T>.Rent(n)` returns an array of **at least** `n` elements; always slice to the requested length. Return exactly once; never use the array after returning it.
- `MemoryExtensions.Split(ReadOnlySpan<char>, Span<Range>, char)` is .NET 8; the enumerator-returning `span.Split(',')` overloads are .NET 9.
- `CollectionsMarshal.AsSpan(list)` gives a span over a `List<T>`'s backing array; the span becomes stale if the list grows.

***

#### `IDisposable` and `IAsyncDisposable`

```csharp
await using (var writer = new AuditWriter(new MemoryStream()))
{
    await writer.WriteAsync("created");
}   // DisposeAsync flushes and closes

using var sha = System.Security.Cryptography.SHA256.Create();   // using declaration: disposed at end of scope
Console.WriteLine(Convert.ToHexString(sha.ComputeHash("x"u8.ToArray()))[..8]);

public sealed class AuditWriter(Stream stream) : IAsyncDisposable, IDisposable
{
    private readonly StreamWriter _writer = new(stream);
    private bool _disposed;

    public Task WriteAsync(string line)
    {
        ObjectDisposedException.ThrowIf(_disposed, this);   // .NET 7+
        return _writer.WriteLineAsync(line);
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed) return;
        _disposed = true;
        await _writer.DisposeAsync();   // flushes asynchronously
        Console.WriteLine("disposed async");
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        _writer.Dispose();
    }
}
```

The full dispose pattern (`Dispose(bool disposing)` + `GC.SuppressFinalize`) is needed only for unsealed classes or classes that own unmanaged handles directly; for sealed classes that only hold managed disposables, the simple form above is enough. Prefer `SafeHandle` over finalizers.

```csharp
using var manager = new ResourceManager(Path.GetTempFileName());
manager.DoWork();

// Unsealed base class: full pattern so derived classes can extend disposal.
public class ResourceManager : IDisposable
{
    private readonly FileStream _stream;
    private bool _disposed;

    public ResourceManager(string path) => _stream = File.OpenRead(path);

    public void DoWork()
    {
        ObjectDisposedException.ThrowIf(_disposed, this);
        Console.WriteLine(_stream.Length);
    }

    public void Dispose()
    {
        Dispose(disposing: true);
        GC.SuppressFinalize(this);   // CA1816
    }

    protected virtual void Dispose(bool disposing)
    {
        if (_disposed) return;
        if (disposing)
        {
            _stream.Dispose();
        }
        _disposed = true;
    }
}
```

Review rules:
- Every `IDisposable` created by the code must be disposed by the code (CA2000) — unless ownership is transferred (DI container, `HttpClientFactory`, returned to the caller). Do **not** dispose objects resolved from DI.
- `await using` for `IAsyncDisposable` (streams, `DbContext`, `ServiceProvider` with async-disposable services). Disposing an async-only disposable synchronously throws at host shutdown ("only implements IAsyncDisposable").
- `using var` disposes at the end of the enclosing scope — in a long method that can be much later than intended; use a block `using (…) { }` to release early.
- Dispose must be idempotent and must not throw.
- Returning a stream from inside `using` returns a disposed stream.

***

#### Dependency Injection

##### Lifetimes and registration

```csharp
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddTransient<IEmailSender, ConsoleEmailSender>();   // new instance per resolution
services.AddScoped<IUnitOfWork, UnitOfWork>();               // one per scope (per HTTP request)
services.AddSingleton(TimeProvider.System);                  // one for the app lifetime
services.AddSingleton<ReportCache>();                        // BAD: singleton depends on scoped

// ValidateScopes + ValidateOnBuild catch captive dependencies at startup.
// ASP.NET Core enables both automatically in the Development environment only.
try
{
    using var provider = services.BuildServiceProvider(new ServiceProviderOptions
    {
        ValidateScopes = true,
        ValidateOnBuild = true,
    });
}
catch (AggregateException ex)
{
    Console.WriteLine(ex.InnerExceptions[0].Message[..60]);
}

public interface IEmailSender { void Send(string to); }
public sealed class ConsoleEmailSender : IEmailSender { public void Send(string to) => Console.WriteLine(to); }
public interface IUnitOfWork { }
public sealed class UnitOfWork : IUnitOfWork { }
public sealed class ReportCache(IUnitOfWork uow) { public IUnitOfWork Uow { get; } = uow; }
```

Review rules:
- **Captive dependencies**: a singleton that takes a scoped service (`DbContext`, per-request state) keeps the first instance forever — shared across requests and threads. Inject `IServiceScopeFactory` and create a scope per unit of work instead.
- Turn on `ValidateOnBuild`/`ValidateScopes` in all environments for services (`builder.Host.UseDefaultServiceProvider(o => { o.ValidateScopes = true; o.ValidateOnBuild = true; })`), or at least in integration tests.
- Do not call `BuildServiceProvider()` inside `ConfigureServices`/`Program.cs` to resolve something early — it creates a second container with duplicate singletons (ASP0000).
- Service locator (`IServiceProvider.GetService` in business code) hides dependencies; prefer constructor injection.
- Transient `IDisposable` services resolved from the root provider are held until the provider is disposed — a memory leak when resolved repeatedly.
- `IAsyncDisposable`-only services must be disposed with `DisposeAsync`: disposing the container synchronously throws `InvalidOperationException` ("… only implements IAsyncDisposable. Use DisposeAsync to dispose the container").
- Constructor null checks (`?? throw new ArgumentNullException`) for DI-injected parameters are optional: the container never passes `null` for registered services. Keep them in public library types.

##### New in .NET 8: keyed services

```csharp
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddKeyedSingleton<ICache, BigCache>("big");
services.AddKeyedSingleton<ICache, SmallCache>("small");
services.AddSingleton<ReportService>();

await using var provider = services.BuildServiceProvider();
Console.WriteLine(provider.GetRequiredKeyedService<ICache>("small").Name);   // small
Console.WriteLine(provider.GetRequiredService<ReportService>().CacheName);   // big
Console.WriteLine(provider.GetService<ICache>() is null);                    // True — keyed only

public interface ICache { string Name { get; } }
public sealed class BigCache : ICache { public string Name => "big"; }
public sealed class SmallCache : ICache { public string Name => "small"; }

public sealed class ReportService([FromKeyedServices("big")] ICache cache)
{
    public string CacheName => cache.Name;
}
```

- A keyed registration is invisible to unkeyed resolution (`GetService<ICache>()` returns `null`).
- Keys are compared with `Equals`; a typo in a string key fails only at resolution time. Use constants or enums.
- ASP.NET Core 8 supports `[FromKeyedServices("key")]` on minimal API parameters, MVC action parameters and Blazor `[Inject(Key = "key")]`.[^7]

##### Options

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

var config = new ConfigurationBuilder()
    .AddInMemoryCollection(new Dictionary<string, string?>
    {
        ["Email:SmtpServer"] = "smtp.example.com",
        ["Email:Port"] = "70000",   // invalid
    })
    .Build();

var services = new ServiceCollection();
services.AddOptions<EmailSettings>()
    .Bind(config.GetSection(EmailSettings.Section))
    .ValidateDataAnnotations()
    .ValidateOnStart();                         // with a host: fail at startup, not on first use
services.AddSingleton<IValidateOptions<EmailSettings>, EmailSettingsValidator>();   // source-generated validator

await using var provider = services.BuildServiceProvider();
try
{
    _ = provider.GetRequiredService<IOptions<EmailSettings>>().Value;
}
catch (OptionsValidationException ex)
{
    Console.WriteLine(ex.Failures.First());
}

public sealed class EmailSettings
{
    public const string Section = "Email";

    [Required]
    public string SmtpServer { get; set; } = "";

    [Range(1, 65535)]
    public int Port { get; set; } = 587;

    [AllowedValues("ssl", "starttls", "none")]   // New in .NET 8
    public string Security { get; set; } = "starttls";
}

// New in .NET 8: compile-time generated validation (trimming/AOT friendly).
[OptionsValidator]
public sealed partial class EmailSettingsValidator : IValidateOptions<EmailSettings> { }
```

- `IOptions<T>`: singleton, read once. `IOptionsSnapshot<T>`: scoped, re-read per request (cannot be injected into singletons). `IOptionsMonitor<T>`: singleton with change notifications.
- `ValidateOnStart()` requires a host (`IHost.StartAsync` runs the validation); without it, invalid configuration surfaces on the first `.Value` access in production traffic.
- Options classes are mutable and bound by reflection unless `<EnableConfigurationBindingGenerator>true</EnableConfigurationBindingGenerator>` (on by default with Native AOT/trimming). **New in .NET 8**: the binder throws for values it cannot convert (`ConfigurationBinder` "throws for mismatched value") and silently skips invalid array elements.[^9]
- Never put secrets in `appsettings.json`; use environment variables, user secrets (development) or a vault provider.

***

#### Logging

```csharp
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

var services = new ServiceCollection();
services.AddLogging(b => b.AddSimpleConsole(o => o.SingleLine = true).SetMinimumLevel(LogLevel.Debug));
await using var provider = services.BuildServiceProvider();
var logger = provider.GetRequiredService<ILogger<Program>>();

int orderId = 42;
string customer = "ada";

// BAD: interpolation formats the string even when the level is disabled and
// destroys the structured properties (CA2254).
logger.LogInformation($"Order {orderId} placed by {customer}");

// GOOD: message template with named placeholders.
logger.LogInformation("Order {OrderId} placed by {Customer}", orderId, customer);

// BEST for hot paths: source-generated, allocation-free when disabled (CA1848).
Log.OrderPlaced(logger, orderId, customer);

try
{
    throw new InvalidOperationException("db down");
}
catch (Exception ex)
{
    logger.LogError(ex, "Failed to place order {OrderId}", orderId);   // exception first
}

static partial class Log
{
    // New in .NET 8: shorter LoggerMessage constructors (level, message).
    [LoggerMessage(LogLevel.Information, "Order {OrderId} placed by {Customer}")]
    public static partial void OrderPlaced(ILogger logger, int orderId, string customer);
}

public partial class Program { }
```

Review rules:
- No string interpolation or concatenation in log message templates; placeholders are PascalCase names, consistent across the codebase (CA2254). Argument count must match placeholders (CA2017).
- Pass the exception object as the first argument — `LogError(ex, …)` — not `ex.Message` in the template.
- Never log secrets, tokens, passwords, full connection strings, or personal data without redaction. Logging request/response bodies (`HttpLoggingFields.RequestBody`) is a data-protection finding.
- Use scopes (`logger.BeginScope`) or `Activity` tags for correlation IDs instead of repeating them in every message.
- `ILogger<T>` with the consuming type as `T`; do not inject `ILoggerFactory` just to create one logger.

***

#### HttpClient

```csharp
using System.Net.Http.Json;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Http.Resilience;   // package Microsoft.Extensions.Http.Resilience (8.x)

var services = new ServiceCollection();
services.AddHttpClient<GitHubClient>(client =>
    {
        client.BaseAddress = new Uri("https://api.github.com/");   // trailing slash matters
        client.DefaultRequestHeaders.UserAgent.ParseAdd("sample/1.0");
        client.Timeout = TimeSpan.FromSeconds(30);
    })
    .AddStandardResilienceHandler();   // retries, circuit breaker, timeouts (Polly v8)

await using var provider = services.BuildServiceProvider();
var gh = provider.GetRequiredService<GitHubClient>();
Console.WriteLine(gh.BaseAddress);

// BaseAddress + relative URI follow RFC 3986 resolution:
Console.WriteLine(new Uri(new Uri("https://host/api"), "users"));    // https://host/users
Console.WriteLine(new Uri(new Uri("https://host/api/"), "users"));   // https://host/api/users
Console.WriteLine(new Uri(new Uri("https://host/api/"), "/users"));  // https://host/users

public sealed class GitHubClient(HttpClient http)
{
    public Uri? BaseAddress => http.BaseAddress;

    public async Task<Repo?> GetRepoAsync(string owner, string name, CancellationToken ct)
    {
        using var response = await http.GetAsync(
            $"repos/{Uri.EscapeDataString(owner)}/{Uri.EscapeDataString(name)}",
            HttpCompletionOption.ResponseHeadersRead, ct);
        if (response.StatusCode == System.Net.HttpStatusCode.NotFound) return null;
        response.EnsureSuccessStatusCode();
        return await response.Content.ReadFromJsonAsync<Repo>(ct);
    }
}

public sealed record Repo(string Name, int StargazersCount);
```

Review rules:
- **Never `new HttpClient()` per request** (socket exhaustion) and never a single static `HttpClient` without `PooledConnectionLifetime` (stale DNS). Use `IHttpClientFactory` (named/typed clients) or a long-lived `HttpClient(new SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(2) })`.
- Typed clients are registered as **transient**. Injecting a typed client into a singleton captures one `HttpMessageHandler` forever and defeats handler rotation.
- Retries need idempotency. The standard resilience handler (defaults: 30 s total timeout, 3 exponential retries with jitter on 5xx/408/429/`HttpRequestException`, circuit breaker, 10 s per-attempt timeout) retries **all HTTP methods**, including `POST`. `options.Retry.DisableForUnsafeHttpMethods()` and `DisableFor(HttpMethod.Post, …)` exist only from package version 9.1.0 (verified: absent in 8.10.0 and 9.0.0; the 9.x packages support `net8.0`). On 8.x, set `options.Retry.ShouldHandle` to exclude unsafe methods, or use idempotency keys.[^20]
- Do not stack resilience handlers (`AddStandardResilienceHandler` plus a custom Polly handler on the same client); use one pipeline.
- Dispose `HttpResponseMessage` (`using var response`). Use `HttpCompletionOption.ResponseHeadersRead` for large/streamed bodies.
- `HttpClient.Timeout` defaults to 100 seconds; per-call timeouts go through a `CancellationToken`. Timeouts throw `TaskCanceledException` (inner `TimeoutException`).
- Escape path segments (`Uri.EscapeDataString`) — user input in a URL path is an SSRF/path-injection vector.
- `GetFromJsonAsync`/`ReadFromJsonAsync` use web defaults (camelCase, case-insensitive). **New in .NET 8**: `GetFromJsonAsAsyncEnumerable<T>` streams arrays.

***

#### System.Text.Json

##### Defaults that cause bugs

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

// Cache options: creating JsonSerializerOptions per call rebuilds metadata caches (CA1869).
JsonSerializerOptions strict = new()
{
    PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
    UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,   // New in .NET 8
};

// 1. Property names are case-sensitive by default (outside ASP.NET Core's web defaults).
var p1 = JsonSerializer.Deserialize<Person>("""{"name":"Ada","age":36}""");
Console.WriteLine(p1!.Name ?? "<null>");   // <null> — "name" did not match "Name"

// 2. Nullable annotations are NOT enforced in .NET 8: a non-nullable string can become null.
var p2 = JsonSerializer.Deserialize<Person>("""{"Name":null,"Age":1}""");
Console.WriteLine(p2!.Name is null);       // True

// 3. Unknown members are ignored unless Disallow is set.
try
{
    JsonSerializer.Deserialize<Person>("""{"name":"Ada","age":1,"isAdmin":true}""", strict);
}
catch (JsonException ex)
{
    Console.WriteLine(ex.Message[..40]);
}

// 4. "required" (C# 11) makes the JSON property mandatory.
try
{
    JsonSerializer.Deserialize<Signup>("""{"email":null}""", strict);
    JsonSerializer.Deserialize<Signup>("{}", strict);
}
catch (JsonException ex)
{
    Console.WriteLine(ex.Message[..60]);
}

// 5. The literal JSON "null" deserializes to null without an exception.
Console.WriteLine(JsonSerializer.Deserialize<Person>("null") is null);   // True

// New in .NET 8: snake_case / kebab-case naming policies.
var snake = new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower };
Console.WriteLine(JsonSerializer.Serialize(new Person("Ada", 36), snake));   // {"name":"Ada","age":36}

public sealed record Person(string Name, int Age);

public sealed class Signup
{
    public required string Email { get; init; }
}
```

The `required` example prints the first failure only: `{"email":null}` passes (the property is present — `null` is accepted), and `{}` throws. Validate values after deserialization; JSON shape checks are not business validation.

##### Source generation, enums, polymorphism

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

// Source-generated metadata: required for Native AOT and trimming, faster startup.
string json = JsonSerializer.Serialize(new Shipment(1, Status.Shipped), AppJson.Default.Shipment);
Console.WriteLine(json);                                         // {"id":1,"status":"Shipped"}
Shipment back = JsonSerializer.Deserialize(json, AppJson.Default.Shipment)!;
Console.WriteLine(back.Status);

// Polymorphism (.NET 7+): the discriminator must be the FIRST property in .NET 8.
Shape[] shapes = [new Circle(1), new Square(2)];
string polyJson = JsonSerializer.Serialize(shapes, AppJson.Default.ShapeArray);
Console.WriteLine(polyJson);    // [{"$type":"circle","radius":1},{"$type":"square","side":2}]
try
{
    JsonSerializer.Deserialize("""[{"radius":1,"$type":"circle"}]""", AppJson.Default.ShapeArray);
}
catch (NotSupportedException ex)
{
    // With $type not first, .NET 8 falls back to the abstract base type and fails.
    // (AllowOutOfOrderMetadataProperties is .NET 9.)
    Console.WriteLine(ex.GetType().Name);   // NotSupportedException
}

public enum Status { Pending, Shipped }

public sealed record Shipment(
    int Id,
    [property: JsonConverter(typeof(JsonStringEnumConverter<Status>))] Status Status);   // New in .NET 8: AOT-safe generic converter

[JsonPolymorphic]
[JsonDerivedType(typeof(Circle), "circle")]
[JsonDerivedType(typeof(Square), "square")]
public abstract record Shape;
public sealed record Circle(double Radius) : Shape;
public sealed record Square(double Side) : Shape;

[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
[JsonSerializable(typeof(Shipment))]
[JsonSerializable(typeof(Shape[]))]
internal sealed partial class AppJson : JsonSerializerContext { }
```

Review rules:
- **Security**: never deserialize to `object` or use custom type-name handling from untrusted input. `[JsonDerivedType]` is an allow-list — keep it closed. Newtonsoft.Json `TypeNameHandling` other than `None` on untrusted data is a remote-code-execution finding.
- `MaxDepth` defaults to 64; do not raise it for untrusted input. Limit request body sizes at the server.
- Enums serialize as numbers by default; changing to strings (or back) is a wire-format breaking change.
- `JsonSerializerOptions` become immutable after first use; mutating them afterwards throws `InvalidOperationException`. Configure once (ASP.NET Core: `builder.Services.ConfigureHttpJsonOptions(...)` for minimal APIs, `AddControllers().AddJsonOptions(...)` for MVC — they are separate).
- **New in .NET 8**: `JsonObjectCreationHandling.Populate` (deserialize into existing read-only collections), `[JsonInclude]` on non-public members, `JsonSerializerOptions.TypeInfoResolverChain`, `MakeReadOnly()`. `AddContext<T>()` is obsolete (SYSLIB0049).
- In Native AOT/trimmed apps, reflection-based serialization is disabled (`JsonSerializerIsReflectionEnabledByDefault=false`) — every serialized type must be in a `JsonSerializerContext`.[^5]
- `DateTime` round-trips as ISO 8601; `DateTime` with `Kind=Unspecified` loses offset information — use `DateTimeOffset` on the wire.
- `ReferenceHandler.IgnoreCycles` or DTOs instead of serializing EF entities with navigation cycles (`JsonException: A possible object cycle was detected`).

***

#### ASP.NET Core 8 Essentials

##### Minimal API service skeleton

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.AspNetCore.Diagnostics;
using Microsoft.AspNetCore.Http.HttpResults;
using Microsoft.AspNetCore.Mvc;

var builder = WebApplication.CreateBuilder(args);

builder.Services.AddProblemDetails();                              // RFC 7807 bodies for errors
builder.Services.AddExceptionHandler<GlobalExceptionHandler>();    // New in .NET 8: IExceptionHandler
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<ITodoStore, InMemoryTodoStore>();
builder.Services.AddRequestTimeouts(o =>
    o.DefaultPolicy = new() { Timeout = TimeSpan.FromSeconds(10) });   // New in .NET 8

var app = builder.Build();

app.UseExceptionHandler();      // uses the registered IExceptionHandler + ProblemDetails
app.UseStatusCodePages();
app.UseRequestTimeouts();

var todos = app.MapGroup("/todos");

// TypedResults + Results<...> make every response type explicit (and visible to OpenAPI).
todos.MapGet("/{id:int}", async Task<Results<Ok<Todo>, NotFound>> (int id, ITodoStore store, CancellationToken ct) =>
    await store.FindAsync(id, ct) is { } todo ? TypedResults.Ok(todo) : TypedResults.NotFound());

todos.MapPost("/", async Task<Results<Created<Todo>, ValidationProblem>> (CreateTodo request, ITodoStore store, CancellationToken ct) =>
{
    // ASP.NET Core 8 minimal APIs do not validate DataAnnotations automatically.
    var errors = Validate(request);
    if (errors.Count > 0)
    {
        return TypedResults.ValidationProblem(errors);
    }
    var todo = await store.AddAsync(request.Title, ct);
    return TypedResults.Created($"/todos/{todo.Id}", todo);
});

todos.MapGet("/boom", () => { throw new InvalidOperationException("secret detail"); });

app.MapGet("/healthz", () => TypedResults.Ok("ok")).ShortCircuit();   // New in .NET 8: skips later middleware

app.Run();

static Dictionary<string, string[]> Validate(object model)
{
    var results = new List<ValidationResult>();
    Validator.TryValidateObject(model, new ValidationContext(model), results, validateAllProperties: true);
    return results
        .SelectMany(r => r.MemberNames.Select(m => (Member: m, r.ErrorMessage)))
        .GroupBy(x => x.Member)
        .ToDictionary(g => g.Key, g => g.Select(x => x.ErrorMessage ?? "invalid").ToArray());
}

public sealed record Todo(int Id, string Title, DateTimeOffset CreatedAt);

public sealed record CreateTodo([property: Required, StringLength(100, MinimumLength = 1)] string Title);

public interface ITodoStore
{
    Task<Todo?> FindAsync(int id, CancellationToken ct);
    Task<Todo> AddAsync(string title, CancellationToken ct);
}

public sealed class InMemoryTodoStore(TimeProvider clock) : ITodoStore
{
    private readonly Dictionary<int, Todo> _items = new();
    private readonly object _gate = new();
    private int _nextId;

    public Task<Todo?> FindAsync(int id, CancellationToken ct)
    {
        lock (_gate) return Task.FromResult(_items.GetValueOrDefault(id));
    }

    public Task<Todo> AddAsync(string title, CancellationToken ct)
    {
        lock (_gate)
        {
            var todo = new Todo(++_nextId, title, clock.GetUtcNow());
            _items[todo.Id] = todo;
            return Task.FromResult(todo);
        }
    }
}

// New in .NET 8: central, DI-friendly exception handling.
public sealed class GlobalExceptionHandler(ILogger<GlobalExceptionHandler> logger, IProblemDetailsService problemDetails)
    : IExceptionHandler
{
    public async ValueTask<bool> TryHandleAsync(HttpContext context, Exception exception, CancellationToken ct)
    {
        logger.LogError(exception, "Unhandled exception for {Path}", context.Request.Path);
        context.Response.StatusCode = StatusCodes.Status500InternalServerError;
        return await problemDetails.TryWriteAsync(new ProblemDetailsContext
        {
            HttpContext = context,
            Exception = exception,
            ProblemDetails = new ProblemDetails { Title = "An unexpected error occurred." },  // no exception text
        });
    }
}
```

Verified behavior of this skeleton on ASP.NET Core 8.0.32: `GET /todos/1` → 404 with a ProblemDetails body (from `UseStatusCodePages` + `AddProblemDetails`); `POST /todos` with `{"title":""}` → 400 `application/problem+json` with `errors.Title`; after a valid POST, `GET /todos/1` → 200; `GET /todos/boom` → 500 ProblemDetails without the exception message; `/healthz` → 200.

Review rules for minimal APIs and controllers:
- **No automatic validation in .NET 8 minimal APIs.** DataAnnotations on parameters do nothing unless the code (or a filter/library) validates them; built-in minimal API validation (`AddValidation()`) is .NET 10. MVC controllers with `[ApiController]` do validate automatically and return 400.
- **Never leak exception details**: `UseDeveloperExceptionPage()` (or `ProblemDetails.Detail = exception.ToString()`) outside Development exposes stack traces and configuration. `WebApplication` enables the developer page automatically only in the Development environment.
- Prefer `TypedResults` over `Results` so the return types are checked at compile time; use `Results<T1, T2>` unions for multiple outcomes.
- Bind a `CancellationToken` parameter (it is `HttpContext.RequestAborted`) and pass it down.
- Route constraints (`{id:int}`) are for routing, not validation of business rules; never use them as security checks.
- Return DTOs, not EF entities (over-posting on input, data exposure and cycles on output).
- `IExceptionHandler.TryHandleAsync` returning `false` passes the exception to the next handler; multiple handlers run in registration order.[^7]
- **New in .NET 8**: request timeouts (`AddRequestTimeouts`/`UseRequestTimeouts`/`.WithRequestTimeout(...)`) cancel `HttpContext.RequestAborted` (the response is 504 by default if the handler does not produce one); they only work if the handler observes the token, nothing times out until a policy is configured, and the middleware does not trigger while a debugger is attached.[^21]

##### Middleware order

```csharp
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.AspNetCore.RateLimiting;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)   // a default scheme is required
    .AddJwtBearer(o =>                    // package Microsoft.AspNetCore.Authentication.JwtBearer 8.0.x
    {
        o.Authority = "https://login.example.com/";
        o.Audience = "api://my-api";      // validate audience; never disable issuer/lifetime validation
    });
builder.Services.AddAuthorizationBuilder()
    .SetFallbackPolicy(new Microsoft.AspNetCore.Authorization.AuthorizationPolicyBuilder()
        .RequireAuthenticatedUser()
        .Build());                        // endpoints require auth unless [AllowAnonymous]
builder.Services.AddCors(o => o.AddPolicy("spa", p => p
    .WithOrigins("https://app.example.com")
    .AllowCredentials()
    .AllowAnyHeader()
    .AllowAnyMethod()));
builder.Services.AddRateLimiter(o =>
{
    o.RejectionStatusCode = StatusCodes.Status429TooManyRequests;
    o.AddFixedWindowLimiter("api", l => { l.PermitLimit = 100; l.Window = TimeSpan.FromMinutes(1); });
});

var app = builder.Build();

app.UseExceptionHandler("/error");
if (!app.Environment.IsDevelopment())
{
    app.UseHsts();
}
app.UseHttpsRedirection();
app.UseCors("spa");               // before auth for preflight requests
app.UseAuthentication();
app.UseAuthorization();
app.UseRateLimiter();             // after routing info is available (WebApplication adds UseRouting implicitly)

app.MapGet("/error", () => Results.Problem()).AllowAnonymous();
app.MapGet("/public", () => "hello").AllowAnonymous();
app.MapGet("/api/me", (System.Security.Claims.ClaimsPrincipal user) => user.Identity?.Name)
    .RequireRateLimiting("api");

app.Run();
```

- `UseAuthentication` must come before `UseAuthorization`, and both before the endpoints they protect. `UseCors` goes before authentication/authorization.
- `AddAuthentication()` without a default scheme makes every challenge fail with `InvalidOperationException: No authenticationScheme was specified, and there was no DefaultChallengeScheme found` — a 500 instead of a 401 (verified).
- JWT bearer: keep `ValidateIssuer`, `ValidateAudience`, `ValidateLifetime` and `ValidateIssuerSigningKey` enabled (defaults when `Authority`/`Audience` are set); `RequireHttpsMetadata = false` belongs only in development.
- `AllowAnyOrigin()` combined with `AllowCredentials()` makes the app fail at startup once the policy is used (`InvalidOperationException: The CORS protocol does not allow specifying a wildcard (any) origin and credentials at the same time`, verified); `SetIsOriginAllowed(_ => true)` with credentials bypasses that check and is a security hole.
- A fallback policy that requires authenticated users makes "forgot `[Authorize]`" fail closed. Without it, endpoints are anonymous by default.
- Rate limiting partitions: a global limiter keyed on nothing throttles all users together; partition by user or client IP (after forwarded headers are processed).

##### Hosting, ports and proxies

- **New in .NET 8**: container images default to port **8080** (`ASPNETCORE_HTTP_PORTS=8080`) and run as the non-root `app` user; Kubernetes manifests and Dockerfiles that still expose port 80 break.[^9] `HTTP_PORTS`/`HTTPS_PORTS` configuration keys are new in .NET 8.[^7]
- Behind a reverse proxy, call `app.UseForwardedHeaders()` with `KnownProxies`/`KnownNetworks` configured (or `ASPNETCORE_FORWARDEDHEADERS_ENABLED=true` only when the proxy is trusted and the app is not directly reachable). Otherwise `RemoteIpAddress`, scheme and host are wrong — and trusting `X-Forwarded-For` from anyone lets clients spoof IPs for rate limiting and audit logs.
- `UseHttpsRedirection` + HSTS for internet-facing apps; when TLS terminates at a proxy, forwarded headers must run first or redirects loop.
- Kestrel request limits (`MaxRequestBodySize` defaults to 30,000,000 bytes, header limits) — raise per endpoint (`[RequestSizeLimit]`, `[DisableRequestSizeLimit]`) only where needed.

##### Forms and antiforgery (new in .NET 8 for minimal APIs)

Minimal API endpoints that bind `[FromForm]`, `IFormFile` or `IFormCollection` require antiforgery validation in .NET 8. Call `AddAntiforgery()` and `UseAntiforgery()`; without the middleware the request fails with `InvalidOperationException` ("…contains anti-forgery metadata, but a middleware was not found that supports anti-forgery"). For token-authenticated APIs (no cookies), opt out explicitly with `.DisableAntiforgery()`.[^7]

```csharp
var builder = WebApplication.CreateBuilder(args);
builder.Services.AddAntiforgery();
var app = builder.Build();
app.UseAntiforgery();

// Cookie-authenticated browser form: antiforgery token required.
app.MapPost("/profile", ([Microsoft.AspNetCore.Mvc.FromForm] string displayName) => Results.Ok(displayName));

// Bearer-token API receiving uploads: no ambient credentials, so antiforgery is not needed.
app.MapPost("/api/upload", async (IFormFile file, CancellationToken ct) =>
{
    // Never trust file.FileName for paths; validate size and content type.
    if (file.Length > 10 * 1024 * 1024) return Results.BadRequest("too large");
    var safeName = $"{Guid.NewGuid():N}{Path.GetExtension(file.FileName)}";
    await using var target = File.Create(Path.Combine(Path.GetTempPath(), safeName));
    await file.CopyToAsync(target, ct);
    return Results.Ok(safeName);
}).DisableAntiforgery();

app.Run();
```

##### Other ASP.NET Core 8 changes reviewers meet

- **Identity API endpoints**: `builder.Services.AddIdentityApiEndpoints<IdentityUser>()` + `app.MapIdentityApi<IdentityUser>()` add register/login/refresh endpoints with bearer tokens or cookies. They are opaque tokens issued by ASP.NET Core Identity — not JWTs, not OIDC; review that registration/confirmation flows and email sending are configured.[^7]
- **Keyed services** in endpoints: `([FromKeyedServices("big")] ICache cache) => …`.
- **`[AsParameters]`** (.NET 7) groups parameters into a type; `IFormFile` binding and complex form binding are new in .NET 8.
- **Metrics**: hosting, Kestrel and SignalR publish `System.Diagnostics.Metrics` instruments (`http.server.request.duration`, …) for OpenTelemetry.[^7]
- **Blazor**: the Blazor Web App template unifies Server and WebAssembly with per-component render modes (`@rendermode InteractiveServer`); code in an interactive WebAssembly component runs in the browser — never place secrets or authorization decisions there.
- `WebApplication.CreateSlimBuilder()` (Native AOT template) omits HTTPS/HTTP3 and some configuration sources unless added; `WebApplication.CreateEmptyBuilder()` adds nothing.

***

#### EF Core 8

EF Core 8 targets `net8.0` and is supported until November 10, 2026 — the same date as .NET 8. EF Core 9 also targets `net8.0` (same end date); EF Core 10 requires .NET 10.[^14] Keep the provider and EF packages on the same latest 8.0.x patch (8.0.32 as of October 2026).

##### Querying correctly

```csharp
using System.ComponentModel.DataAnnotations.Schema;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

await using var connection = new SqliteConnection("DataSource=:memory:");
await connection.OpenAsync();
var options = new DbContextOptionsBuilder<ShopContext>().UseSqlite(connection).Options;

await using (var setup = new ShopContext(options))
{
    await setup.Database.EnsureCreatedAsync();
    setup.Customers.Add(new Customer
    {
        Name = "Ada",
        Address = new Address("1 Main St", "Springfield"),
        Tags = ["vip", "beta"],
        Orders = { new Order { Total = 10m }, new Order { Total = 25m } },
    });
    await setup.SaveChangesAsync();
}

await using var db = new ShopContext(options);

// Read-only query: AsNoTracking, project to a DTO, filter in the database.
var summaries = await db.Customers
    .AsNoTracking()
    .Where(c => c.Orders.Any(o => o.Total > 20m))
    .Select(c => new { c.Name, OrderCount = c.Orders.Count, c.Address.City })
    .ToListAsync();
Console.WriteLine($"{summaries[0].Name} {summaries[0].OrderCount} {summaries[0].City}");

// New in EF Core 8: primitive collections. A parameter list is sent as ONE JSON parameter.
string[] wanted = ["vip", "gold"];
var query = db.Customers.Where(c => c.Tags.Any(t => wanted.Contains(t)));
Console.WriteLine(query.ToQueryString().Contains("json_each"));   // True
Console.WriteLine(await query.CountAsync());                      // 1

// FromSql with an interpolated string is parameterized — safe.
string name = "Ada'; DROP TABLE Customers; --";
var safe = db.Customers.FromSql($"SELECT * FROM Customers WHERE Name = {name}");
Console.WriteLine(safe.ToQueryString().Split('\n')[0]);           // .param set p0 'Ada''; DROP …'
Console.WriteLine(await safe.CountAsync());                       // 0

// New in EF Core 8: raw SQL for unmapped types.
int customerId = 1;
var orderIds = await db.Database
    .SqlQuery<int>($"SELECT Id AS Value FROM Orders WHERE CustomerId = {customerId}")
    .ToListAsync();
Console.WriteLine(orderIds.Count);                                // 2

// Set-based updates (EF Core 7+): no entities loaded, change tracker NOT updated.
int updated = await db.Orders
    .Where(o => o.CustomerId == customerId)
    .ExecuteUpdateAsync(s => s.SetProperty(o => o.Total, 0m));
Console.WriteLine(updated);                                       // 2

public sealed class ShopContext(DbContextOptions<ShopContext> options) : DbContext(options)
{
    public DbSet<Customer> Customers => Set<Customer>();
    public DbSet<Order> Orders => Set<Order>();
}

public sealed class Customer
{
    public int Id { get; set; }
    public required string Name { get; set; }
    public required Address Address { get; set; }      // complex type (EF Core 8)
    public List<string> Tags { get; set; } = [];       // primitive collection (EF Core 8), stored as JSON
    public List<Order> Orders { get; } = [];
}

[ComplexType]
public sealed record Address(string Street, string City);

public sealed class Order
{
    public int Id { get; set; }
    public decimal Total { get; set; }
    public int CustomerId { get; set; }
}
```

SQLite has no decimal type; EF Core's SQLite provider stores `decimal` as `TEXT`, so comparisons and arithmetic on such columns in raw SQL compare text. Do not use SQLite-backed tests to validate money logic that the production database (SQL Server, PostgreSQL) executes.

Review rules:
- **SQL injection**: `FromSql($"…{value}")`, `SqlQuery($"…")` and `ExecuteSql($"…")` turn interpolation holes into parameters. `FromSqlRaw("… '" + value + "'")`, `FromSqlRaw($"…{value}")` and `ExecuteSqlRaw` with concatenation are injection bugs. Identifiers (table/column names) cannot be parameters — validate them against an allow-list.
- **`DbContext` is not thread-safe.** Two concurrent queries on one context throw `InvalidOperationException: A second operation was started on this context instance before a previous operation completed`. `Task.WhenAll` over queries on the same context is a bug; use separate contexts (`IDbContextFactory<T>`) or sequential awaits.
- **Lifetime**: register with `AddDbContext` (scoped) or `AddDbContextPool`; never as a singleton, never captured in a singleton. Use `AddDbContextFactory` for Blazor Server and background services.
- **N+1 queries**: navigation access in a loop (with lazy loading) or a query per item. Use projection (`Select`) or `Include`; use `AsSplitQuery()` when several collection includes multiply rows (cartesian explosion).
- **Client evaluation**: `AsEnumerable()`/`ToList()` before `Where` downloads the whole table. EF Core throws for untranslatable expressions except in the final projection — do not "fix" that by inserting `AsEnumerable()` early.
- **Tracking**: `AsNoTracking()` for read-only queries; tracked entities in long-lived contexts grow memory without bound.
- **`ExecuteUpdate`/`ExecuteDelete`** bypass the change tracker, `SaveChanges` interceptors, and cascade logic configured only in EF; entities already loaded keep stale values.
- **Async all the way**: `ToListAsync`, `FirstOrDefaultAsync`, `SaveChangesAsync` with the request `CancellationToken`.
- **Pagination** needs a deterministic `OrderBy` before `Skip`/`Take`; prefer keyset pagination for deep pages.
- **Concurrency**: use a concurrency token (`[Timestamp]` row version on SQL Server, `IsConcurrencyToken()`) for data edited by several users, and handle `DbUpdateConcurrencyException`.
- **Migrations**: do not call `Database.Migrate()` at startup of horizontally scaled services (concurrent migrators); apply migrations in a deployment step or with SQL scripts/bundles. `EnsureCreated()` bypasses migrations — tests and prototypes only.
- **Connection resiliency** (`EnableRetryOnFailure`) retries whole operations; user-initiated transactions need an execution strategy (`db.Database.CreateExecutionStrategy().ExecuteAsync(...)`).

```csharp
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

await using var connection = new SqliteConnection("DataSource=:memory:");
await connection.OpenAsync();
var options = new DbContextOptionsBuilder<AppDb>().UseSqlite(connection).Options;
await using var db = new AppDb(options);
await db.Database.EnsureCreatedAsync();
for (int i = 0; i < 200; i++) db.Items.Add(new Item { Name = $"a{i}" });
await db.SaveChangesAsync();

// BAD: several concurrent operations on one DbContext (e.g. a parallel loop sharing it).
try
{
    await Task.WhenAll(Enumerable.Range(0, 8).Select(_ =>
        Task.Run(() => db.Items.Where(i => i.Name.StartsWith("a")).ToListAsync())));
}
catch (InvalidOperationException ex)
{
    Console.WriteLine(ex.Message[..60]);   // A second operation was started on this context instance…
}

public sealed class AppDb(DbContextOptions<AppDb> options) : DbContext(options)
{
    public DbSet<Item> Items => Set<Item>();
}

public sealed class Item
{
    public int Id { get; set; }
    public required string Name { get; set; }
}
```

##### EF Core 8 behavior changes to check when upgrading from EF Core 7[^22]

- **`Contains` over a parameter collection** (verified with `ToQueryString()` on the SQL Server provider 8.0.32) now sends the list as a JSON parameter: `OPENJSON` on SQL Server, `json_each` on SQLite. On SQL Server 2014 or databases with compatibility level < 130 (including Azure SQL databases migrated with an old level) the query **fails**; fix the compatibility level or configure `UseSqlServer(conn, o => o.UseCompatibilityLevel(120))`. The new translation also caused plan regressions for some queries — `EF.Constant(list).Contains(x)` restores the inlined-constants form per query (EF Core 8.0.2+).
- **Enums inside JSON columns** (owned types mapped with `ToJson()`) are stored as **integers** by default (EF Core 7 stored strings). Existing JSON data with string enums needs `HasConversion<string>()`.
- SQL Server `date`/`time` columns scaffold to `DateOnly`/`TimeOnly`; SQL Server string keys compare case-insensitively in the change tracker; discriminator columns get a max length (migration generated); with several `AddDbContext` calls for the same context, the **last** configuration wins.
- SQLite: `Math` methods now translate to SQL functions; a native SQLite build without math functions fails with "no such function".

***

#### Security

```csharp
using System.Security.Cryptography;
using System.Text;

// Passwords: a slow, salted KDF. Prefer ASP.NET Core Identity's PasswordHasher or a vetted
// library; if hand-rolled, use PBKDF2 with SHA-256+ and a high iteration count.
byte[] salt = RandomNumberGenerator.GetBytes(16);
byte[] hash = Rfc2898DeriveBytes.Pbkdf2("correct horse", salt, 600_000, HashAlgorithmName.SHA256, 32);

// Constant-time comparison for secrets, MACs and hashes.
byte[] candidate = Rfc2898DeriveBytes.Pbkdf2("correct horse", salt, 600_000, HashAlgorithmName.SHA256, 32);
Console.WriteLine(CryptographicOperations.FixedTimeEquals(hash, candidate));   // True

// Tokens and identifiers: cryptographic RNG, never System.Random (CA5394).
string apiKey = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
string code = RandomNumberGenerator.GetString("ABCDEFGHJKLMNPQRSTUVWXYZ23456789", 8);   // New in .NET 8
Console.WriteLine($"{apiKey.Length} {code.Length}");

// Authenticated encryption. New in .NET 8: the AesGcm constructor without a tag size is obsolete (SYSLIB0053).
byte[] key = RandomNumberGenerator.GetBytes(32);
byte[] nonce = RandomNumberGenerator.GetBytes(AesGcm.NonceByteSizes.MaxSize);   // never reuse a nonce with the same key
byte[] plaintext = Encoding.UTF8.GetBytes("secret");
byte[] ciphertext = new byte[plaintext.Length];
byte[] tag = new byte[16];
using (var aes = new AesGcm(key, tagSizeInBytes: 16))
{
    aes.Encrypt(nonce, plaintext, ciphertext, tag);
    var roundTrip = new byte[ciphertext.Length];
    aes.Decrypt(nonce, ciphertext, tag, roundTrip);
    Console.WriteLine(Encoding.UTF8.GetString(roundTrip));   // secret
}

// Path traversal: resolve and check containment.
Console.WriteLine(IsInside("/srv/files", "report.pdf"));          // True
Console.WriteLine(IsInside("/srv/files", "../../etc/passwd"));    // False
Console.WriteLine(IsInside("/srv/files", "/srv/files-evil/x"));   // False

static bool IsInside(string root, string userPath)
{
    string fullRoot = Path.GetFullPath(root).TrimEnd(Path.DirectorySeparatorChar) + Path.DirectorySeparatorChar;
    string full = Path.GetFullPath(Path.Combine(fullRoot, userPath));   // Combine drops fullRoot if userPath is rooted
    return full.StartsWith(fullRoot, StringComparison.Ordinal);
}
```

Review checklist for security findings:
- **Injection**: SQL built by concatenation or `FromSqlRaw`/`ExecuteSqlRaw` with interpolation; `Process.Start` with a shell (`/bin/sh -c`, `cmd /c`) or a single concatenated argument string — use `ProcessStartInfo.ArgumentList`; LDAP/XPath built from input; `Regex` patterns from input without `Regex.Escape`.
- **Deserialization**: `BinaryFormatter` (throws in .NET 8 unless re-enabled — re-enabling is a finding), `NetDataContractSerializer`, `SoapFormatter`, `LosFormatter`, Newtonsoft `TypeNameHandling` ≠ `None`, `JavaScriptSerializer` with a type resolver. System.Text.Json polymorphism only through an explicit `[JsonDerivedType]` allow-list.
- **Crypto**: MD5/SHA1 for security purposes (CA5350/CA5351), `Rfc2898DeriveBytes` constructors without an explicit hash and iteration count (SYSLIB0041, CA5379), ECB mode, static IVs/nonces, `new AesGcm(key)` (SYSLIB0053), custom crypto, hard-coded keys. SHA-3 (`SHA3_256` …, **new in .NET 8**) needs `SHA3_256.IsSupported` checks — it depends on OpenSSL 1.1.1+ or a recent Windows build.[^5]
- **Randomness**: `Random`/`Random.Shared` for tokens, passwords, IDs or nonces (CA5394). `Guid.NewGuid()` is random but not designed as a secret; use `RandomNumberGenerator` for secrets.
- **ReDoS**: regexes over untrusted input without a timeout. The default timeout is infinite (`Regex.InfiniteMatchTimeout`); `^(a+)+$` on 30 `a`s plus `!` runs for a very long time. Pass a timeout, set `REGEX_DEFAULT_MATCH_TIMEOUT` via `AppContext`, or use `RegexOptions.NonBacktracking` (.NET 7+). Prefer `[GeneratedRegex]` (.NET 7) for static patterns.
- **XML**: `XmlReaderSettings.DtdProcessing = Parse` with a resolver on untrusted XML (XXE). Defaults in .NET 8 prohibit DTDs and do not resolve external entities; `XmlSecureResolver` is obsolete (SYSLIB0047).
- **Path traversal**: `Path.Combine(root, userInput)` without the containment check above; `IFormFile.FileName` used as a path.
- **Secrets**: keys, connection strings or tokens in source, `appsettings.json`, logs, exception messages or ProblemDetails. ASP.NET Core Data Protection keys must be persisted and protected in multi-instance deployments (`PersistKeysTo…` + `ProtectKeysWith…`); the default container location is ephemeral.
- **Web**: missing `[Authorize]`/fallback policy, IDOR (resource IDs not checked against the current user), CORS `SetIsOriginAllowed(_ => true)` with credentials, cookie auth without antiforgery, open redirects (`Redirect(returnUrl)` without `Url.IsLocalUrl`/`LocalRedirect`), unbounded uploads, `UseForwardedHeaders` trusting everyone.
- **Patch level**: a runtime or ASP.NET Core older than 8.0.32 (or `mcr.microsoft.com/dotnet/aspnet:8.0.x` pinned to an older tag) is missing security fixes.[^2]

***

#### Performance

- **Measure first**: BenchmarkDotNet for micro-benchmarks, `dotnet-counters`, `dotnet-trace`, `dotnet-gcdump` in production. Do not accept "optimizations" without numbers.
- **.NET 8 defaults**: dynamic PGO is on by default; tiered compilation and ReadyToRun interact with it. Disabling tiered compilation or PGO in `runtimeconfig` usually makes steady-state code slower.[^5]
- **Allocation hot spots**: string concatenation in loops, LINQ in per-request hot paths, closures capturing locals in lambdas called per item, boxing value types through `object`/non-generic interfaces, `params object[]` calls, `ToList()`/`ToArray()` copies, `Enum.ToString()` in tight loops.
- **Strings**: `string.Create`, `StringBuilder` with capacity, span-based parsing (`int.Parse(ReadOnlySpan<char>)`), `CompositeFormat` for repeated runtime formats, UTF-8 literals (`"…"u8`) for protocol bytes, `SearchValues<char>` for repeated `IndexOfAny`.
- **Collections**: presize (`new List<T>(capacity)`, `new Dictionary<,>(capacity)`), `FrozenDictionary` for static lookups, `TryGetValue`/`TryAdd` instead of double lookups (CA1854/CA1864), `CollectionsMarshal.GetValueRefOrAddDefault` for counters in hot loops.
- **Async**: avoid `async` state machines in trivial pass-throughs, `ValueTask` for hot, mostly synchronous paths, no `Task.Run` wrappers around async I/O.
- **JSON**: cache `JsonSerializerOptions`; source-generated `JsonSerializerContext`; stream with `SerializeAsync`/`DeserializeAsync`/`DeserializeAsyncEnumerable` instead of buffering whole payloads into strings.
- **Regex**: `[GeneratedRegex]` for static patterns; cached `static readonly Regex` with `RegexOptions.Compiled` otherwise; never construct a `Regex` per call.
- **GC**: Server GC is the default for ASP.NET Core; in containers with small CPU/memory limits, measure `DOTNET_GCHeapHardLimit`/`GCHeapAffinitizeMask` or `DOTNET_GCgen0size` changes before shipping. `GC.RefreshMemoryLimit()` (**new in .NET 8**) applies a changed container memory limit at run time.[^5]
- **Database**: N+1 queries, missing indexes and over-fetching dominate service latency far more than micro-optimizations in C#.

```csharp
using System.Runtime.InteropServices;
using System.Text.RegularExpressions;

// GeneratedRegex: compiled at build time, no runtime Regex construction cost.
Console.WriteLine(Patterns.Sku().IsMatch("AB-1234"));   // True

// Hot-loop counter without double hashing.
var counts = new Dictionary<string, int>(StringComparer.Ordinal);
foreach (var word in "a b a c a".Split(' '))
{
    ref int slot = ref CollectionsMarshal.GetValueRefOrAddDefault(counts, word, out _);
    slot++;
}
Console.WriteLine(counts["a"]);                         // 3

static partial class Patterns
{
    [GeneratedRegex("^[A-Z]{2}-[0-9]{4}$", RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 100)]
    public static partial Regex Sku();
}
```

***

#### Testing

Tests use xUnit, NUnit or MSTest; the examples use xUnit. Name tests `Method_Scenario_Expected` (or the team's convention), keep Arrange/Act/Assert visible, prefer `[Theory]` for rule tables.

```csharp
using System.Diagnostics.Metrics;
using System.Net;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.Diagnostics.Metrics.Testing;   // package Microsoft.Extensions.Diagnostics.Testing
using Microsoft.Extensions.Time.Testing;                  // package Microsoft.Extensions.TimeProvider.Testing
using Xunit;

public class CalculatorTests
{
    [Theory]
    [InlineData(1, 1, 2)]
    [InlineData(0, 0, 0)]
    [InlineData(-1, 1, 0)]
    public void Add_VariousNumbers_ReturnsSum(int a, int b, int expected)
        => Assert.Equal(expected, Calculator.Add(a, b));
}

public class SubscriptionTests
{
    [Fact]
    public void IsActive_AfterExpiry_ReturnsFalse()
    {
        // New in .NET 8: deterministic time with TimeProvider + FakeTimeProvider.
        var clock = new FakeTimeProvider(new DateTimeOffset(2026, 1, 1, 0, 0, 0, TimeSpan.Zero));
        var subscription = new Subscription(clock, TimeSpan.FromDays(30));

        clock.Advance(TimeSpan.FromDays(31));

        Assert.False(subscription.IsActive);
    }

    [Fact]
    public void Renew_RecordsMetric()
    {
        // New in .NET 8: MetricCollector<T> captures measurements of an instrument.
        using var meter = new Meter("Billing");
        var renewals = meter.CreateCounter<int>("billing.renewals");
        using var collector = new MetricCollector<int>(renewals);

        renewals.Add(1, new KeyValuePair<string, object?>("plan", "pro"));

        var measurement = Assert.Single(collector.GetMeasurementSnapshot());
        Assert.Equal(1, measurement.Value);
        Assert.Equal("pro", measurement.Tags["plan"]);
    }
}

public class ApiTests
{
    [Fact]
    public async Task GetHealth_ReturnsOk()
    {
        // In-memory server (package Microsoft.AspNetCore.TestHost). For a real Program,
        // use WebApplicationFactory<Program> from Microsoft.AspNetCore.Mvc.Testing.
        var builder = WebApplication.CreateBuilder();
        builder.WebHost.UseTestServer();
        await using var app = builder.Build();
        app.MapGet("/healthz", () => "ok");
        await app.StartAsync();

        using var client = app.GetTestClient();
        var response = await client.GetAsync("/healthz");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("ok", await response.Content.ReadAsStringAsync());
    }
}

public static class Calculator
{
    public static int Add(int a, int b) => a + b;
}

public sealed class Subscription(TimeProvider clock, TimeSpan length)
{
    private readonly DateTimeOffset _expiresAt = clock.GetUtcNow() + length;
    public bool IsActive => clock.GetUtcNow() < _expiresAt;
}
```

Review rules:
- Tests must not depend on wall-clock time, `Thread.Sleep`/`Task.Delay` timing, test order, machine culture/time zone, or shared static state. Inject `TimeProvider`; set culture explicitly where formatting is tested.
- `async void` test methods are not awaited by xUnit v2 analyzers' expectations — use `async Task`.
- `WebApplicationFactory<Program>` needs `public partial class Program { }` (or `InternalsVisibleTo`) when `Program` is generated from top-level statements.
- Mocking `HttpClient`: fake the `HttpMessageHandler`, not `HttpClient`. Mocking `DbContext`/`DbSet`: prefer SQLite in-memory or Testcontainers with the production database engine — the EF in-memory provider does not enforce relational constraints or translate SQL.
- Mock libraries: Moq, NSubstitute, FakeItEasy are all fine; flag only over-mocking (tests that assert implementation details).
- Assert exceptions with `Assert.Throws`/`ThrowsAsync` (`await Assert.ThrowsAsync<T>(...)` — an un-awaited `ThrowsAsync` always passes).

***

#### Analyzers and Tooling

- Built-in .NET analyzers (CA rules) ship with SDK 8.0.426; enable them with `<AnalysisLevel>8.0-recommended</AnalysisLevel>` (or `latest-recommended`) and `<EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>` for IDE rules. Tune severities in `.editorconfig` (`dotnet_diagnostic.CA2007.severity = none` in application projects).
- Rules introduced with the .NET 8 SDK that are worth enforcing: CA1510–CA1513 (throw helpers), CA1856/CA1857 (`ConstantExpected`), CA1858 (`StartsWith` instead of `IndexOf == 0`), CA1859 (concrete types for performance), CA1860 (`Length/Count` instead of `Any()`), CA1861 (constant arrays as arguments), CA1862 (case-insensitive comparison overloads), CA1863 (`CompositeFormat`), CA1864 (`TryAdd`), CA1865–CA1867 (`char` overloads), CA1868 (unnecessary `Contains` before `Add`/`Remove` on sets), CA1869 (cache `JsonSerializerOptions`), CA1870 (`SearchValues`), CA2021 (impossible casts), CA2261 (`ConfigureAwaitOptions.SuppressThrowing` on `Task<T>`).[^16] Rules such as CA1514, CA1515, CA1871, CA1872, CA2022 and CA2262–CA2265 arrive with the .NET 9 SDK and are not present in SDK 8.0.426.
- IDE0290 (use primary constructor) and IDE0300–IDE0305 (use collection expressions) are present in SDK 8.0.426; they are style suggestions.
- Treat nullable warnings as errors (`<WarningsAsErrors>nullable</WarningsAsErrors>` at minimum).
- `dotnet format` enforces `.editorconfig` style in CI; `dotnet list package --vulnerable --include-transitive` and NuGet audit find vulnerable dependencies.
- Third-party analyzers commonly used and worth respecting: Roslynator, SonarAnalyzer.CSharp, Meziantou.Analyzer, xunit.analyzers, Microsoft.VisualStudio.Threading.Analyzers (async misuse).

***

#### Trimming and Native AOT

.NET 8 supports Native AOT for console apps, worker services, gRPC and ASP.NET Core **minimal APIs** (partially). In ASP.NET Core 8, MVC, Blazor Server, SignalR, Session, SPA middleware and authentication other than JWT bearer are **not** AOT-compatible.[^24] EF Core 8 does not support Native AOT (experimental support with precompiled queries starts in EF Core 9).[^23]

```xml
<PropertyGroup>
  <PublishAot>true</PublishAot>              <!-- implies trimming and AOT analyzers -->
  <InvariantGlobalization>true</InvariantGlobalization>
</PropertyGroup>
<!-- Libraries: <IsAotCompatible>true</IsAotCompatible> enables the trim/AOT analyzers without publishing AOT. -->
```

- With `PublishAot` (or `PublishTrimmed`) set, the build reports trim/AOT warnings even for `dotnet build`: reflection-based `JsonSerializer.Serialize<T>(value)` produces **IL2026** and **IL3050**, `Type.GetType(userString)` produces **IL2057** (verified). Treat these warnings as errors; each is a potential run-time failure after publishing.
- The setting also changes **JIT runs**: with `PublishAot=true`, `dotnet run` throws `InvalidOperationException: Reflection-based serialization has been disabled for this application` on the same call (verified), because the `JsonSerializerIsReflectionEnabledByDefault` feature switch is set to `false`. Use source-generated `JsonSerializerContext`s.
- AOT-incompatible patterns: `Assembly.LoadFrom`, `Activator.CreateInstance(Type.GetType(name))`, `MakeGenericType` over value types, `System.Reflection.Emit`, `dynamic`, runtime-compiled expression trees (they fall back to interpretation), libraries that scan assemblies (AutoMapper profiles, MediatR assembly scanning, reflection-based DI conventions).
- ASP.NET Core minimal APIs under AOT use the Request Delegate Generator; `ConfigureHttpJsonOptions(o => o.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonContext.Default))` registers source-generated metadata.[^7]

***

#### Observability

```csharp
using System.Diagnostics;
using System.Diagnostics.Metrics;
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddMetrics();                          // registers IMeterFactory (New in .NET 8)
services.AddSingleton<CheckoutMetrics>();
await using var provider = services.BuildServiceProvider();

var metrics = provider.GetRequiredService<CheckoutMetrics>();
using (var activity = CheckoutMetrics.Source.StartActivity("checkout"))   // null when nobody listens
{
    activity?.SetTag("order.id", 42);
    metrics.OrderPlaced("pro");
}
Console.WriteLine("recorded");

public sealed class CheckoutMetrics
{
    public static readonly ActivitySource Source = new("Shop.Checkout");
    private readonly Counter<long> _orders;

    public CheckoutMetrics(IMeterFactory meterFactory)
    {
        // Meters from IMeterFactory are scoped to the DI container (testable, no static state).
        var meter = meterFactory.Create("Shop.Checkout");
        _orders = meter.CreateCounter<long>("shop.orders.placed", unit: "{order}");
    }

    public void OrderPlaced(string plan) => _orders.Add(1, new KeyValuePair<string, object?>("plan", plan));
}
```

- Use `System.Diagnostics.Metrics` (`Meter`, `Counter<T>`, `Histogram<T>`, `UpDownCounter<T>`, observable gauges) and `ActivitySource`/`Activity` for tracing; export with OpenTelemetry (`OpenTelemetry.Extensions.Hosting`, `AddAspNetCoreInstrumentation`, `AddHttpClientInstrumentation`, OTLP exporter).
- `StartActivity` returns `null` when no listener is registered — always use `?.`.
- Metric tags with unbounded cardinality (user IDs, URLs with IDs, raw paths) explode time-series storage; use route templates and bounded values.
- Health checks: `builder.Services.AddHealthChecks().AddCheck<DbHealthCheck>("db")` + `app.MapHealthChecks("/healthz")`. Separate liveness (process alive) from readiness (dependencies reachable); do not put slow dependency checks on liveness probes.
- Structured logs (JSON console formatter or OpenTelemetry logs) with trace/span IDs correlate logs and traces automatically when `Activity.Current` is set.

***

#### Containers and Deployment

- **New in .NET 8**: official images listen on port 8080 (`ASPNETCORE_HTTP_PORTS=8080`) and include a non-root `app` user (`USER app` or `USER $APP_UID`); Debian 12 is the default base; chiseled Ubuntu images (`8.0-jammy-chiseled`) have no shell or package manager.[^9]
- `dotnet publish` can build container images without a Dockerfile (`dotnet publish -t:PublishContainer`, .NET 8 SDK); since SDK 8 the default tag is `latest` — pin explicit tags in CI.[^9]
- Do not run as root; do not bake secrets into images; set `DOTNET_gcServer`/memory limits consciously; configure `HEALTHCHECK`/Kubernetes probes against health endpoints.
- Graceful shutdown: hosted services must honor the `CancellationToken` in `StopAsync`/`ExecuteAsync`; `HostOptions.ShutdownTimeout` defaults to 30 seconds in .NET 8.
- `BackgroundService.ExecuteAsync` that throws stops the host by default (`BackgroundServiceExceptionBehavior.StopHost`, .NET 6+) — catch and log per iteration in long-running loops.

```csharp
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

var builder = Host.CreateApplicationBuilder(args);
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddHostedService<QueueWorker>();
using var host = builder.Build();
await host.StartAsync();
await Task.Delay(120);
await host.StopAsync();

public sealed class QueueWorker(ILogger<QueueWorker> logger, TimeProvider clock) : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(50), clock);   // New in .NET 8: TimeProvider overload
        try
        {
            while (await timer.WaitForNextTickAsync(stoppingToken))
            {
                try
                {
                    await ProcessBatchAsync(stoppingToken);
                }
                catch (Exception ex) when (ex is not OperationCanceledException)
                {
                    logger.LogError(ex, "Batch failed; continuing");   // one bad batch must not stop the host
                }
            }
        }
        catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
        {
            // normal shutdown
        }
        logger.LogInformation("Worker stopped");
    }

    private Task ProcessBatchAsync(CancellationToken ct)
    {
        logger.LogInformation("Processing batch");
        return Task.CompletedTask;
    }
}
```

***

#### Removed / Changed in .NET 8 — Migration Notes

##### From .NET 7 / C# 11 to .NET 8 / C# 12

| Area | Change | Action |
| --- | --- | --- |
| Serialization | `BinaryFormatter.Serialize/Deserialize` throw `NotSupportedException` in most project types | Replace with System.Text.Json, protobuf, MessagePack; do not set `EnableUnsafeBinaryFormatterSerialization` |
| Serialization | Formatter-based serialization APIs obsolete (SYSLIB0050/0051) | Drop `[Serializable]` + `(SerializationInfo, StreamingContext)` constructors from new exception types |
| Crypto | `new AesGcm(key)` obsolete (SYSLIB0053); `RSA.EncryptValue/DecryptValue` obsolete (SYSLIB0048) | Pass `tagSizeInBytes`; use `Encrypt/Decrypt` with padding |
| LINQ | Vectorized `Enumerable.Sum` changes overflow behavior for some inputs | Accumulate in `long` near limits |
| Globalization | `TwoDigitYearMax` default 2029 → 2049 | Never parse two-digit years from external data |
| Configuration | Binder throws for unconvertible values; skips invalid array elements | Fix configuration or validate options at startup |
| Containers | Default port 80 → 8080; non-root `app` user; Debian 12 | Update `EXPOSE`, probes, Kubernetes `containerPort`, file permissions |
| SDK | `dotnet publish`/`pack` default to Release; RID no longer implies self-contained; NuGet audit warnings on restore | Update CI scripts; fix vulnerable packages |
| ASP.NET Core | Minimal API form binding requires antiforgery (`UseAntiforgery`) | Add middleware or `.DisableAntiforgery()` for token-authenticated APIs |
| EF Core 8 | `Contains` → `OPENJSON` (SQL Server compat level ≥ 130), JSON enums as ints, last `AddDbContext` wins | See EF Core section |
| Language | C# 12 is the default for `net8.0` | Primary constructors / collection expressions are safe to adopt; review CS9124/CS9107 warnings |

##### Planning the move to .NET 10 LTS

.NET 8 ends support on November 10, 2026. .NET 10 (LTS, supported until November 14, 2028) is the target; skipping .NET 9 (STS, also ending November 10, 2026) is normal.[^3] When reviewing code that will move soon:

- Code written for C# 12 compiles on C# 14 with few changes. Watch for the C# 14 `field` keyword (a member named `field` used inside property accessors changes meaning — rename it or use `@field`), the C# 13 `params` collection overload preferences, and first-class span conversions that change which extension overload is chosen.[^11]
- .NET 9 removed `BinaryFormatter` entirely, and .NET 9/10 add analyzers (CA2022, CA2262–CA2265, …) that may flag existing code.
- Third-party packages pinned to 8.x `Microsoft.Extensions.*`/EF Core 8 must move to 10.x together.
- Until the migration ships, keep .NET 8 on the newest patch every month (Patch Tuesday) and track the runtime version in container images.

***

#### Production / Review Checklist

##### Version and project

- [ ] `TargetFramework` is `net8.0` and the runtime/ASP.NET Core image is **8.0.32** or newer; a plan exists to move to .NET 10 LTS before/at the November 10, 2026 end of support
- [ ] No `LangVersion` `preview`/`latest`/13+ on `net8.0`; no C# 13–15 syntax, no .NET 9/10 APIs (`Lock`, `params ReadOnlySpan`, `field`, extension blocks, `Task.WhenEach`, `CountBy`, `Guid.CreateVersion7`, `JsonSerializerOptions.Web`, `AddOpenApi`, `MapStaticAssets`)
- [ ] `Nullable` enabled; nullable warnings are errors; `!` and `#pragma warning disable` justified
- [ ] `AnalysisLevel` set; CA/IDE rules enforced in build; NuGet audit warnings addressed
- [ ] `global.json` pins a current 8.0 SDK band with `rollForward: latestPatch`

##### C# 12 features

- [ ] No CS9124 (double storage) or CS9107 (captured and passed to base) warnings with primary constructors
- [ ] Primary-constructor parameters are not mutated accidentally; validated where needed via initializers
- [ ] Plain classes with primary constructors are not mistaken for records (no properties, no value equality)
- [ ] Collection expressions targeting interfaces are not downcast or mutated; `[]` vs `null` semantics preserved
- [ ] No hand-written interceptors; `[Experimental]` suppressions justified

##### Correctness

- [ ] Ordinal comparisons for identifiers; invariant culture for machine-readable formatting/parsing
- [ ] `switch` expressions exhaustive (discard arm for enums); no `SwitchExpressionException` paths
- [ ] Records with collections not used where deep equality is assumed
- [ ] No mutation of collections during enumeration; no multiple enumeration of expensive sequences
- [ ] UTC (`DateTimeOffset.UtcNow` / `TimeProvider`) for stored and compared times

##### Async and concurrency

- [ ] No `.Result`/`.Wait()`/`GetAwaiter().GetResult()` on incomplete tasks; no `async void` outside event handlers
- [ ] `CancellationToken` accepted and forwarded; `OperationCanceledException` not swallowed
- [ ] `ConfigureAwaitOptions.SuppressThrowing` only on non-generic `Task`
- [ ] No `await` in `lock`; `SemaphoreSlim` released in `finally`; no shared mutable static state
- [ ] `DbContext` never used concurrently; `ConcurrentDictionary` factories side-effect free
- [ ] Background services catch per-iteration failures and honor the stopping token

##### Dependencies, configuration, HTTP

- [ ] No captive dependencies (scoped in singleton); `ValidateScopes`/`ValidateOnBuild` on in tests
- [ ] Keyed services resolved with the right key constants
- [ ] Options validated with `ValidateDataAnnotations()`/`[OptionsValidator]` + `ValidateOnStart()`
- [ ] `IHttpClientFactory` or a long-lived `HttpClient` with `PooledConnectionLifetime`; responses disposed; retries only for idempotent requests
- [ ] `JsonSerializerOptions` cached; `UnmappedMemberHandling`/`required`/explicit validation for untrusted JSON; no reflection JSON in AOT/trimmed apps

##### ASP.NET Core

- [ ] `UseExceptionHandler` + `IExceptionHandler`/ProblemDetails; no exception details in production responses
- [ ] Minimal API inputs validated explicitly (no built-in validation in .NET 8)
- [ ] Authentication has a default scheme; fallback authorization policy or `[Authorize]` everywhere; correct middleware order
- [ ] CORS lists origins explicitly; antiforgery for cookie-authenticated forms
- [ ] Forwarded headers restricted to known proxies; container port 8080 and non-root user reflected in deployment manifests
- [ ] Request timeouts/rate limits configured where needed and handlers observe `RequestAborted`

##### Data access

- [ ] No `FromSqlRaw`/`ExecuteSqlRaw` with concatenated or interpolated input
- [ ] `AsNoTracking` + projections for reads; no N+1; `AsSplitQuery` for multiple collection includes
- [ ] SQL Server compatibility level ≥ 130 (or `UseCompatibilityLevel(120)`) for EF Core 8 `Contains`
- [ ] Migrations applied by a deployment step, not by every instance at startup
- [ ] Concurrency tokens on contended entities; transactions wrapped in execution strategies when retries are enabled

##### Security

- [ ] No `BinaryFormatter`/`TypeNameHandling`; polymorphic JSON via `[JsonDerivedType]` allow-lists
- [ ] `RandomNumberGenerator` for secrets; PBKDF2/Identity hasher for passwords; `FixedTimeEquals` for comparisons
- [ ] Regex on untrusted input has a timeout or `NonBacktracking`
- [ ] Paths from users are contained; uploads size-limited and renamed
- [ ] No secrets in source, config files, logs or ProblemDetails; Data Protection keys persisted and protected

##### Performance and observability

- [ ] Hot paths measured; no per-request `FrozenDictionary`/`Regex`/`JsonSerializerOptions` construction
- [ ] `stackalloc` bounded; pooled arrays returned once and sliced
- [ ] Message-template logging (no interpolation); source-generated `LoggerMessage` in hot paths; no PII in logs
- [ ] Metrics via `IMeterFactory`; bounded tag cardinality; `ActivitySource` usage null-safe
- [ ] Health checks split into liveness and readiness

***

[^1]: Announcing .NET 8. Microsoft .NET Blog, November 14, 2023. https://devblogs.microsoft.com/dotnet/announcing-dotnet-8/
[^2]: .NET 8.0.32 release notes (October 13, 2026) and the .NET 8 release metadata (latest release, SDKs, CVE lists incl. CVE-2025-55315 in 8.0.21). https://github.com/dotnet/core/blob/main/release-notes/8.0/8.0.32/8.0.32.md , https://builds.dotnet.microsoft.com/dotnet/release-metadata/8.0/releases.json
[^3]: .NET and .NET Core support policy; .NET releases index (support phases and end-of-support dates for .NET 8, 9, 10, 11). https://dotnet.microsoft.com/en-us/platform/support/policy/dotnet-core , https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json
[^4]: What's new in C# 12. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-12
[^5]: What's new in the .NET 8 runtime and libraries. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-8/runtime
[^6]: ConfigureAwaitOptions enum; CancellationTokenSource.CancelAsync. https://learn.microsoft.com/en-us/dotnet/api/system.threading.tasks.configureawaitoptions , https://learn.microsoft.com/en-us/dotnet/api/system.threading.cancellationtokensource.cancelasync
[^7]: What's new in ASP.NET Core in .NET 8. https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-8.0
[^8]: What's new in EF Core 8. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-8.0/whatsnew
[^9]: Breaking changes in .NET 8 (containers, core libraries, cryptography, extensions, globalization, SDK, serialization). https://learn.microsoft.com/en-us/dotnet/core/compatibility/8.0
[^10]: What's new in C# 13. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-13
[^11]: What's new in C# 14. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-14
[^12]: What's new in C# 15 (preview). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
[^13]: What's new in .NET 9 libraries. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-9/libraries
[^14]: EF Core releases and planning (target frameworks and support dates for EF Core 8, 9, 10). https://learn.microsoft.com/en-us/ef/core/what-is-new/
[^15]: What's new in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/overview
[^16]: Code quality rules (CA) reference. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/quality-rules/
[^17]: Tutorial: Explore primary constructors. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/tutorials/primary-constructors
[^18]: Collection expressions — C# language reference. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/operators/collection-expressions
[^19]: Interceptors feature specification (Roslyn). https://github.com/dotnet/roslyn/blob/main/docs/features/interceptors.md
[^20]: Build resilient HTTP apps (Microsoft.Extensions.Http.Resilience defaults, DisableFor/DisableForUnsafeHttpMethods). https://learn.microsoft.com/en-us/dotnet/core/resilience/http-resilience
[^21]: Request timeouts middleware in ASP.NET Core. https://learn.microsoft.com/en-us/aspnet/core/performance/timeouts?view=aspnetcore-8.0
[^22]: Breaking changes in EF Core 8. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-8.0/breaking-changes
[^23]: What's new in EF Core 9 (experimental Native AOT and precompiled queries). https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-9.0/whatsnew
[^24]: ASP.NET Core support for Native AOT (.NET 8 compatibility table). https://learn.microsoft.com/en-us/aspnet/core/fundamentals/native-aot?view=aspnetcore-8.0
