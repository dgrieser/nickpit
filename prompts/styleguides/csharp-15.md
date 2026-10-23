### C# 15 on .NET 11: The Complete Production Developer Guide

> **Version**: C# 15 ships with the .NET 11 SDK (Roslyn 5.12). **.NET 11 is not generally available yet**: the latest build is **.NET 11.0.0 RC 2** (SDK `11.0.100-rc.2.26504.105`, runtime `11.0.0-rc.2.26504.105`, released **2026-10-13**). RC 1 (`11.0.100-rc.1.26425.128`) shipped on 2026-09-08. Both release candidates carry a **go-live license** (Microsoft supports them in production; RC 2 is supported until 2026-11-10). General availability is expected in **November 2026** (the launch is planned for .NET Conf 2026, November 10–12).[^1][^2][^3]
> **Support**: .NET 11 is a **Standard Term Support (STS)** release. Since .NET 9, STS releases get **24 months** of support (previously 18), so .NET 11 is expected to be supported until about November 2028; the exact end date is published at GA.[^4][^5]
> **Newest stable release as of 2026-10-22**: .NET 10 (LTS, C# 14), latest patch **10.0.13** (2026-10-13), supported until 2028-11-14. .NET 9 (STS) and .NET 8 (LTS) both reach end of support on **2026-11-10**.[^4]
> **Scope**: the C# 15 language (unions, closed hierarchies, collection expression arguments, extension indexers, labeled `break`/`continue`), the .NET 11 runtime/BCL/ASP.NET Core/EF Core changes a reviewer must know, and the idioms and pitfalls of modern production C# (nullable reference types, records, pattern matching, async, cancellation, spans, collections, LINQ, exceptions, disposal, DI, options, logging, `HttpClient`, `System.Text.Json`, minimal APIs, EF Core, security, performance, testing, analyzers, trimming/Native AOT).
> **Note**: this document is self-contained. Everything from C# 8–14 and .NET 5–10 that is still current is carried forward. Sections marked **New in C# 15** or **New in .NET 11** cover version-specific behaviour. All examples were compiled and run with the .NET 11 RC 2 SDK; behaviour boundaries were checked against the .NET 10.0.402 SDK (runtime 10.0.13). Because .NET 11 is a release candidate, details can still change before GA — verify against the GA release notes when the project moves to the final SDK.

---

#### Table of Contents

1. [What's New in C# 15 and .NET 11](#whats-new-in-c-15-and-net-11)
2. [Version-Aware Review](#version-aware-review)
3. [Do Not Use or Recommend](#do-not-use-or-recommend)
4. [Project Setup](#project-setup)
5. [Naming and Code Style](#naming-and-code-style)
6. [Union Types (New in C# 15)](#union-types-new-in-c-15)
7. [Closed Hierarchies (New in C# 15)](#closed-hierarchies-new-in-c-15)
8. [Choosing Between Unions, Closed Hierarchies and Enums](#choosing-between-unions-closed-hierarchies-and-enums)
9. [Collection Expression Arguments (New in C# 15)](#collection-expression-arguments-new-in-c-15)
10. [Extension Members and Extension Indexers](#extension-members-and-extension-indexers)
11. [Labeled break and continue (New in C# 15)](#labeled-break-and-continue-new-in-c-15)
12. [Memory Safety Preview (Not for Production)](#memory-safety-preview-not-for-production)
13. [C# 14 Features Available in C# 15](#c-14-features-available-in-c-15)
14. [Nullable Reference Types](#nullable-reference-types)
15. [Records and Immutability](#records-and-immutability)
16. [Pattern Matching and Exhaustiveness](#pattern-matching-and-exhaustiveness)
17. [Exceptions](#exceptions)
18. [Async/Await and Runtime Async](#asyncawait-and-runtime-async)
19. [Cancellation, Timeouts and TimeProvider](#cancellation-timeouts-and-timeprovider)
20. [IAsyncEnumerable and Async LINQ](#iasyncenumerable-and-async-linq)
21. [Concurrency and Synchronization](#concurrency-and-synchronization)
22. [Span, Memory and Allocation-Free Code](#span-memory-and-allocation-free-code)
23. [Collections and Collection Expressions](#collections-and-collection-expressions)
24. [LINQ](#linq)
25. [Strings, Culture and Comparison](#strings-culture-and-comparison)
26. [Numerics, Dates and Times](#numerics-dates-and-times)
27. [IDisposable and IAsyncDisposable](#idisposable-and-iasyncdisposable)
28. [Dependency Injection and Options](#dependency-injection-and-options)
29. [Logging and Observability](#logging-and-observability)
30. [HttpClient](#httpclient)
31. [System.Text.Json](#systemtextjson)
32. [ASP.NET Core Minimal APIs](#aspnet-core-minimal-apis)
33. [Hosting and Background Services](#hosting-and-background-services)
34. [EF Core 11](#ef-core-11)
35. [Security](#security)
36. [Performance](#performance)
37. [Testing](#testing)
38. [Analyzers and Build Hygiene](#analyzers-and-build-hygiene)
39. [Trimming and Native AOT](#trimming-and-native-aot)
40. [Removed / Changed in .NET 11 — Migration Notes](#removed--changed-in-net-11--migration-notes)
41. [Production / Review Checklist](#production--review-checklist)

---

#### What's New in C# 15 and .NET 11

C# 15 is the default language version for projects that target `net11.0` (`csc -langversion:?` in the RC 2 SDK prints `15.0 (default)`). A `net11.0` project built with the RC 2 SDK compiles `union` and `closed` declarations without any `<LangVersion>` element.[^2]

##### Language features (C# 15)

| Feature | Status in the RC 2 SDK | Under `<LangVersion>14</LangVersion>` |
|---|---|---|
| **Union types** — `union Pet(Cat, Dog, Bird);` | Stable in C# 15 | Parse errors (`CS1001`, `CS0106`); no "feature not available" message |
| **Closed hierarchies** — `closed` class modifier | Stable in C# 15 | `CS9327: Feature 'closed classes' is not available in C# 14.0` |
| **Collection expression arguments** — `[with(capacity: n), ..xs]` | Stable in C# 15 | `CS9327: Feature 'collection expression arguments'...` |
| **Extension indexers** — `this[...]` inside `extension(...)` blocks | Stable in C# 15 | `CS9327: Feature 'extension indexers'...` |
| **Labeled `break` / `continue`** — `break outer;` | Stable in C# 15 | `CS9327: Feature 'labeled break and continue'...` |
| **Memory safety** (pointer relaxations, `unsafe(expr)`, `safe` modifier, requires-unsafe members) | **Preview only** — requires `<LangVersion>preview</LangVersion>`; with `15` the compiler reports `CS8652 ... 'updated memory safety rules' is currently in Preview and *unsupported*` | — |

All six are listed on the official "What's new in C# 15" page; the Roslyn feature-status page lists the first five under **C# 15.0** and "Unsafe evolution" as "Merged as preview feature".[^6][^7] The Roslyn compiler breaking-changes document describes the memory-safety changes as **C# 16** behaviour, which confirms they are not part of C# 15.[^8]

The Roslyn feature-status page lists two smaller C# 15 items that the "What's new" page omits:[^7]

- **Non-virtual static interface members without runtime DIM support**: static (non-virtual) methods and constants in interfaces now compile for targets whose runtime lacks default-interface-member support, such as `netstandard2.0`. Verified: `interface I { static int F() => 1; const int C = 2; }` in a `netstandard2.0` library reports `CS9327: Feature 'static members in interfaces' is not available in C# 14.0` under `LangVersion` 14 and compiles under 15. Static *abstract*/*virtual* members still need runtime support.
- **`ExtendedLayoutAttribute`** (`System.Runtime.InteropServices`, interop layouts; the memory-safety preview's `safe` modifier refers to "explicit or extended layout types"). Interop-only; rarely seen in application code.

The .NET 11 docs still call C# 15 "the latest C# preview release" until .NET 11 reaches GA.[^6]

##### Runtime and libraries (.NET 11)

- **Runtime Async** (runtime-native async methods): the BCL itself is compiled with it; **your code opts in** with `<Features>runtime-async=on</Features>`, and the runtime docs still call it **"a preview feature"** for application code. Verified on RC 2: without the flag a `net11.0` app still compiles async methods to state machines. See [Async/Await and Runtime Async](#asyncawait-and-runtime-async).[^9]
- **Hardware baseline raised**: x86/x64 now requires **x86-64-v2** (SSE4.2, POPCNT, CX16); Windows Arm64 requires **LSE**. Older CPUs fail at startup with "The current CPU is missing one or more of the baseline instruction sets."[^9][^10]
- **Behavioural changes that silently change results** (all verified on 10.0.13 vs 11 RC 2): unchecked float→`byte`/`sbyte`/`short`/`ushort`/`char` casts **saturate**; `Math.Round(double, digits)` is **correctly rounded** (`Math.Round(2.675, 2)` is `2.68` on .NET 10 and `2.67` on .NET 11); `DateOnly`/`TimeOnly.TryParse*` **throw** `ArgumentException` for invalid `DateTimeStyles`/format specifiers; `IHost.RunAsync`/`StopAsync` **throw** when a `BackgroundService` fails.[^11]
- **New APIs**: `Process.RunAndCaptureTextAsync` and friends, `ZstandardStream`, `X25519DiffieHellman`, `Enumerable.FullJoin` and tuple-returning `Join`/`GroupJoin`, `EqualityComparer<T>.Create(keySelector)`, `Random.NextInteger<T>`/`NextBinaryFloat<T>`, `StringStream`/`ReadOnlyMemoryStream`/`WritableMemoryStream`/`ReadOnlySequenceStream`, `Decimal32`/`Decimal64`/`Decimal128`, `JsonNamingPolicy.PascalCase`, `System.Text.Json` union support and JSON Lines, async validation (`AsyncValidationAttribute`, `IAsyncValidatableObject`, `IAsyncValidateOptions<T>`), request-body compression content types.[^12]
- **Shared framework grows**: nine `Microsoft.Extensions.*` abstraction libraries (`Options`, `Logging.Abstractions`, `DependencyInjection.Abstractions`, `Configuration.Abstractions`, `Hosting.Abstractions`, `Primitives`, …) are part of `Microsoft.NETCore.App`; explicit `PackageReference`s produce **NU1510**.[^13]
- **ASP.NET Core 11**: `[ShortCircuit]` attribute, union types in JSON bodies, async validation for minimal APIs, endpoint filters documented to run on binding failures, OpenAPI 3.2 default, **`ConcurrencyLimiter` middleware removed**, stricter Kestrel protocol compliance.[^14][^15]
- **EF Core 11**: requires .NET 11; `FullJoin` → `FULL JOIN`, `MaxByAsync`/`MinByAsync`, SQL Server compatibility level default 160, `Migrate()` throws when no migrations exist, split queries throw `DbQueryConcurrencyException` on concurrent modification, `Microsoft.Data.SqlClient` 7.0 (Entra ID auth moved to a separate package).[^16][^17]
- **SDK**: `dotnet run -e KEY=VALUE`, `#:include` for file-based apps, `dotnet test` improvements, solution filter editing, IDE0410 (suggest labeled jumps), less noisy CA1873; the `xunit` template still defaults to xUnit v2 in RC 2 (`--xunit-version v3` opts into v3 with Microsoft.Testing.Platform).[^18]

##### What C# 14 / .NET 10 already gave you (still current)

The `field` keyword, extension members (`extension(T receiver) { ... }` blocks with properties and static members), null-conditional assignment (`a?.B = c`), first-class `Span<T>` conversions, partial constructors and events, user-defined compound assignment operators (`operator +=`), `nameof(List<>)`, modifiers on simple lambda parameters (`(s, out v) => ...`); in the BCL `System.Linq.AsyncEnumerable`, `LeftJoin`/`RightJoin`, `Shuffle`, `JsonSerializerOptions.Strict`, `AllowDuplicateProperties`, and minimal API validation via `AddValidation()`.[^19]

---

#### Version-Aware Review

- **Determine the language version before judging syntax.** C# 15 is implied by `<TargetFramework>net11.0</TargetFramework>` with no `<LangVersion>`, or by an explicit `<LangVersion>15</LangVersion>`/`15.0`. `latest` and `latestmajor` also resolve to 15 with this SDK. `preview` enables the memory-safety preview and is not appropriate for production. Multi-targeted projects (`<TargetFrameworks>net10.0;net11.0</TargetFrameworks>`) compile the `net10.0` leg with C# 14 — C# 15 syntax breaks that leg unless `<LangVersion>` is raised explicitly, and even then the runtime-backed features fail there: against `net10.0` references a `union` reports `CS0518`/`CS0656` (`IUnion`/`UnionAttribute` missing) and a `closed` class reports `CS0656` (`IsClosedTypeAttribute` missing), while collection expression arguments, extension indexers and labeled jumps compile (verified).
- **Check `global.json`.** A pinned SDK (`"version": "11.0.100-rc.2.26504.105"`) with `"rollForward": "latestFeature"` or similar decides which compiler builds the code. Once GA ships, projects should move off the RC SDK; RC 2's go-live support ends 2026-11-10.[^4]
- **Verify APIs against .NET 11 before claiming they are missing — and against .NET 10 before claiming they exist.** Everything in the "New APIs" list above fails to compile against the `net10.0` reference assemblies (checked); everything from .NET 9/10 (`CountBy`, `Index`, `OrderedDictionary<TKey,TValue>`, `System.Threading.Lock`, `Task.WhenEach`, `LeftJoin`, `Shuffle`, `ToAsyncEnumerable`, `JsonSerializerOptions.Strict`) is available.
- Say "not available in C# 15 / .NET 11" only for items in the next section or when the pinned toolchain proves it. Preview features are *available* only with `<LangVersion>preview</LangVersion>`; flag their use in production code rather than calling them nonexistent.
- **Release-candidate caveat.** Union types in particular were still gaining spec features during the previews ("Some features from the proposal specification aren't yet implemented"); behaviour described here is that of RC 2.[^6]

---

#### Do Not Use or Recommend

##### Features that are not in C# 15 / .NET 11

These are on the Roslyn "working set" (in progress, targeting a later C#) or preview-only. Do not suggest them for C# 15 code:[^7]

- **Dictionary expressions** — `Dictionary<string, int> d = ["a": 1];` is a syntax error in C# 15 and in `preview` with the RC 2 compiler. Use `new() { ["a"] = 1 }` or a collection initializer.
- **Null-conditional `await`** — `await? task` is a syntax error. Use `if (task is not null) await task;`.
- **Chained relational comparisons** (`0 < x < 10`), **target-typed static member access**, **relaxed modifier ordering**, **compound assignment in object initializers**, **extension constants**, **extension members on typeless receivers**, **type-parameter inference from constraints** — none exist in C# 15.
- **Memory-safety rules** (pointer types without `unsafe`, `fixed`/`&`/`sizeof` outside `unsafe`, `unsafe(expression)`, the `safe` modifier, the `updated-memory-safety-rules` feature, `MemorySafetyRulesAttribute`) — **preview only** (C# 16 track). With `LangVersion` 15, `int* p = &x;` outside an `unsafe` context is still `CS0214`, and `sizeof(Guid)` is still `CS0233`.
- **Runtime Async as a default** — it is opt-in for application code in .NET 11 (`<Features>runtime-async=on</Features>`); do not assume app methods are runtime-async. Do not recommend the removed `DOTNET_RuntimeAsync` environment variable.[^9]
- **`System.Text.Json` contracts for unions without a classifier** — a union whose cases serialize to the same JSON shape (two record cases with object shape) cannot be deserialized (see [System.Text.Json](#systemtextjson)).
- Any **.NET 12 / C# 16** API or syntax: the next versions are in development and nothing from them is part of this toolchain.

##### Deprecated or removed in .NET 11 (flag in review)

- `RSACryptoServiceProvider.Encrypt(byte[], bool)` / `Decrypt(byte[], bool)` — obsolete **SYSLIB0064**; use the overloads that take `RSAEncryptionPadding` (prefer `RSA.Create()` with `RSAEncryptionPadding.OaepSHA256`).[^20]
- `IStartupValidator` — obsolete **SYSLIB0066**; implement `IAsyncStartupValidator`.[^20]
- The `set` accessor of `AsnEncodedData.RawData` — obsolete **SYSLIB0065** (cryptography obsoletion, .NET 11 Preview 6); construct the appropriate type to decode, or use `AsnEncodedData.CopyFrom(AsnEncodedData)`. Verified: all three IDs (SYSLIB0064/0065/0066) warn on RC 2.[^33]
- ASP.NET Core **`ConcurrencyLimiter` middleware removed** (`UseConcurrencyLimiter`, `Microsoft.AspNetCore.ConcurrencyLimiter`) — use the rate limiting middleware (`AddRateLimiter` with a concurrency limiter).[^15]
- MVC compatibility options, obsolete Blazor APIs, obsolete `ImageTagHelper` members and an obsolete `ModelMetadataIdentity.ForProperty` overload are removed.[^15]
- EF Core: the `EFOptimizeContext` MSBuild property is removed (use `EFScaffoldModelStage`/`EFPrecompileQueriesStage`); owned JSON collections without an explicit key are obsolete (warning `OwnedEntityMappedToJsonCollectionWarning`); synchronous I/O with the Cosmos provider can no longer be re-enabled; `SqlAuthenticationMethod.ActiveDirectoryPassword` is obsolete.[^17]
- Explicit `PackageReference`s to the nine `Microsoft.Extensions.*` libraries now in the shared framework (NU1510) when targeting only `net11.0`.[^13]
- Contextual-keyword fallout: a type named `closed` is rejected (`CS9380`); members typed with a type named `closed` or `union` are parsed as modifiers/declarations (`closed field;` → `CS1519`, `union Field;` → `CS9370`). Escape as `@closed` / `@union` (verified; a type *declared* as `class union` still compiles).[^8]
- Long-standing obsolete APIs remain errors or warnings and must not be reintroduced: `BinaryFormatter` (removed from the runtime since .NET 9 — it throws `PlatformNotSupportedException`), `WebRequest`/`HttpWebRequest`/`WebClient` (SYSLIB0014), all `Rfc2898DeriveBytes` constructors (SYSLIB0060 since .NET 10 — use the static `Rfc2898DeriveBytes.Pbkdf2`), `Thread.Abort` (SYSLIB0006, throws `PlatformNotSupportedException`).

---

#### Project Setup

##### Recommended project file

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net11.0</TargetFramework>
    <!-- C# 15 is the default for net11.0; pin it explicitly only if you need to. Never ship with "preview". -->
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <AnalysisLevel>latest-recommended</AnalysisLevel>
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
    <!-- For libraries and AOT-published apps: turns on trim/AOT/single-file analyzers. -->
    <!-- <IsAotCompatible>true</IsAotCompatible> -->
  </PropertyGroup>
</Project>
```

- `Nullable` must be `enable` in new code; `TreatWarningsAsErrors` makes nullable, exhaustiveness (`CS8509`, `CS8655`) and obsoletion warnings blocking.
- Use **Central Package Management** (`Directory.Packages.props` with `<ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally>`) and lock files (`<RestorePackagesWithLockFile>true</RestorePackagesWithLockFile>`, `dotnet restore --locked-mode` in CI) for reproducible builds.
- Remove `PackageReference`s to `Microsoft.Extensions.Options`, `Microsoft.Extensions.Logging.Abstractions`, `Microsoft.Extensions.DependencyInjection.Abstractions`, `Microsoft.Extensions.Configuration.Abstractions`, `Microsoft.Extensions.Hosting.Abstractions`, `Microsoft.Extensions.Primitives`, `Microsoft.Extensions.Caching.Abstractions`, `Microsoft.Extensions.Diagnostics.Abstractions` and `Microsoft.Extensions.FileProviders.Abstractions` in `net11.0`-only projects: they are in the shared framework (**New in .NET 11**, warning NU1510). Keep them, conditioned on the TFM, in multi-targeted projects.[^13]
- **New in .NET 11 SDK**: `dotnet nuget add source --min-publish-age-hours` lets you refuse packages published less than N hours ago (a supply-chain guard), and `dotnet run -e NAME=value` passes environment variables.[^18]

##### global.json

```json
{
  "sdk": {
    "version": "11.0.100-rc.2.26504.105",
    "rollForward": "latestFeature"
  },
  "test": {
    "runner": "Microsoft.Testing.Platform"
  }
}
```

Pinning the exact RC version works as shown (verified with `dotnet --version`); replace it with the GA version once released. The `"test"` section selects Microsoft.Testing.Platform for `dotnet test` (the .NET 11 `xunit --xunit-version v3` template generates it).

---

#### Naming and Code Style

These house rules apply to all C# code. Format with `dotnet format`; do not hand-align.

```csharp
namespace Contoso.Orders;

// PascalCase for types, methods, properties, events, constants and namespaces
public interface IOrderRepository                      // interfaces start with I
{
    Task<Order?> FindAsync(int id, CancellationToken cancellationToken = default); // async methods end in Async
}

public sealed class OrderService(IOrderRepository repository, ILogger<OrderService> logger)
{
    public const int MaxRetryCount = 3;                // constants: PascalCase, not SCREAMING_CASE
    private static readonly TimeSpan s_defaultTimeout = TimeSpan.FromSeconds(30); // static field: s_ prefix (runtime convention)
    private int _processedCount;                       // instance field: _camelCase

    public bool IsBusy { get; private set; }           // booleans read as predicates: Is/Has/Can

    public async Task<Order> GetAsync(int orderId, CancellationToken cancellationToken)  // parameters/locals: camelCase
    {
        var order = await repository.FindAsync(orderId, cancellationToken)
            ?? throw new KeyNotFoundException($"Order {orderId} not found");
        _processedCount++;
        logger.LogDebug("Loaded order {OrderId} (timeout {Timeout})", orderId, s_defaultTimeout);
        return order;
    }
}

public sealed record Order(int Id, decimal Total);
```

- **Seal classes by default** (`sealed`) unless designed for inheritance; CA1852 flags internal types that can be sealed.
- **File-scoped namespaces**, one top-level type per file (file name = type name), `using` directives outside the namespace, `var` when the type is obvious from the right-hand side.
- **Primary constructors** (C# 12) are fine for DI. Their parameters are *captured mutable state*, not `readonly` fields: assigning to them later is legal and is a review smell. If immutability matters, assign to a `private readonly` field.
- **Member order**: constants, static fields, instance fields, constructors, properties, public methods, private methods.
- Prefer `is null` / `is not null` over `== null` (operators can be overloaded), `string.IsNullOrEmpty`, `ArgumentNullException.ThrowIfNull`, `ArgumentException.ThrowIfNullOrWhiteSpace`, `ArgumentOutOfRangeException.ThrowIfNegative`/`ThrowIfGreaterThan`.
- Solution layout: `src/` (one project per deployable or bounded context), `tests/` (unit and integration test projects), `Directory.Build.props` for shared settings.

---

#### Union Types (New in C# 15)

##### Declaration and generated shape

A union declares a value that is exactly one of a fixed list of **case types**. The case types are existing types; the union does not define new data.[^21]

```csharp
Pet pet = new Dog("Rex");                 // implicit union conversion from each case type

string name = pet switch                  // exhaustive: no default arm, no warning
{
    Dog d => d.Name,
    Cat c => c.Name,
    Bird b => b.Name,
};
Console.WriteLine(name);                  // Rex
Console.WriteLine(pet.Value);             // Dog { Name = Rex }

public sealed record Cat(string Name);
public sealed record Dog(string Name);
public sealed record Bird(string Name);

public union Pet(Cat, Dog, Bird);
```

Verified facts about the compiler-generated type (RC 2):

- `union Pet(...)` becomes a **`struct`** (`typeof(Pet).IsValueType` is `true`) marked `[System.Runtime.CompilerServices.Union]` and implementing `System.Runtime.CompilerServices.IUnion`, with one public constructor per case type and a `public object? Value { get; }`. These types exist only in .NET 11, so unions cannot be used from a `net10.0` target.[^21]
- The contents are stored as **`object?`**: value-type cases (`int`, `DateOnly`, structs) are **boxed** on every construction.
- A union declaration may contain methods and computed properties but **no instance fields, auto-properties or field-like events** (`CS9373`), and no public single-parameter constructors of its own.
- Patterns apply to **`Value`**, not to the struct: `pet is Dog d` tests the contents. Exceptions are `_`, `var` and `not`, which apply to the union value itself. Testing the union against its own type (`pet is Pet`) is rejected with `CS8121` by the RC 2 compiler (the reference documentation still describes a CS0183 warning); to ask "does it hold anything?" write `pet.Value is not null` or `pet is not null`.[^21]
- Generic unions work: `public union Result<T>(T, Error);`.
- Overlapping cases are allowed: in `union U(object, string)`, `U u = "x";` picks the `string` constructor (better conversion) and `U u = 5;` boxes into the `object` case.

##### The `default` trap — always handle `null`

`default(Pet)` (an unassigned field, an array element, `default`, a failed `TryGetValue` out parameter) has `Value == null`. A switch that only lists the case types then throws `SwitchExpressionException` at run time. When no case type is nullable, the compiler treats `Value` as "not null" and **never** warns about the missing `null` arm — not even for an explicit `Pet pet = default;` local:

```csharp
// BAD: compiles without any warning, throws at run time
var shelter = new Shelter();
try
{
    Console.WriteLine(Describe(shelter.Resident));
}
catch (System.Runtime.CompilerServices.SwitchExpressionException ex)
{
    Console.WriteLine($"Unmatched: {ex.UnmatchedValue}");   // Unmatched: Pet
}

// GOOD: an explicit null arm documents and handles the empty union
Console.WriteLine(DescribeSafe(shelter.Resident));          // (no pet)

static string Describe(Pet pet) => pet switch
{
    Dog d => d.Name,
    Cat c => c.Name,
    Bird b => b.Name,
};

static string DescribeSafe(Pet pet) => pet switch
{
    Dog d => d.Name,
    Cat c => c.Name,
    Bird b => b.Name,
    null => "(no pet)",
};

public sealed class Shelter
{
    public Pet Resident;   // default(Pet): Value is null
}

public sealed record Cat(string Name);
public sealed record Dog(string Name);
public sealed record Bird(string Name);
public union Pet(Cat, Dog, Bird);
```

Verified on RC 2: `Pet pet = default; _ = pet switch { Cat c => ..., Dog d => ... };` compiles without any warning and throws `SwitchExpressionException` at run time. The language reference states the rule: "When no case type is nullable, the compiler treats `Value` as 'not null' and does not warn if a `null` arm is omitted. However, handling `null` is still needed for correctness whenever a union can have its default value." The review must therefore check every union that can be `default` (fields, arrays, `default`, `out` parameters, deserialized DTO members). If any case type is nullable (`union MaybeName(string?, int)`), `Value` is maybe-null and `CS8655` fires for every switch without a `null` arm (verified).[^21]

##### Exhaustiveness is the point — keep `CS8509` an error

Adding a case to a union makes every non-exhaustive `switch` *expression* report `CS8509`. That is the feature: treat `CS8509` and `CS8655` as errors (`TreatWarningsAsErrors` or `<WarningsAsErrors>CS8509;CS8655</WarningsAsErrors>`), and **do not add a discard arm** (`_ => throw ...`) to a union switch — it silences exactly the warning that would catch the new case. `switch` *statements* are not checked for exhaustiveness; prefer switch expressions for union dispatch.

Adding a case type to a **public** union shipped in a library is a breaking change for consumers: their compiled switches do not know the new case and throw `SwitchExpressionException` when they meet it.

##### Equality

The generated struct has no custom equality: `==` is not defined (`CS0019`), and `Equals` is the `ValueType` default, which compares `Value` with `Equals` (`new Pet(new Dog("Rex")).Equals(new Pet(new Dog("Rex")))` is `true` because records have value equality). Do not use unions as dictionary keys unless all case types have well-defined equality and hashing.

##### Result types

Unions are a natural fit for "value or error" returns that callers must handle:

```csharp
var outcome = Parse("42");
Console.WriteLine(outcome switch
{
    int value => $"ok {value}",
    ParseError error => $"error at {error.Position}: {error.Message}",
    null => "not evaluated",
});

static ParseResult Parse(string text) =>
    int.TryParse(text, out var value)
        ? value                                    // boxed into the union
        : new ParseError(0, $"'{text}' is not a number");

public sealed record ParseError(int Position, string Message);
public union ParseResult(int, ParseError);
```

Review notes:

- Do not use unions to replace exceptions for **exceptional** conditions (I/O failure, bugs); use them for expected, domain-level alternatives.
- For hot paths with value-type cases, the boxing in the generated union costs an allocation per value. A hand-written union (below) with the non-boxing `HasValue`/`TryGetValue` pattern avoids it.

##### Custom unions and `IUnion`

Any class or struct marked `[Union]` with one public single-parameter constructor per case and a public `object? Value` getter is a union type to the compiler. The compiler *assumes* (does not verify) that `Value` only ever returns `null` or an instance of a case type, that `default` yields `null`, and that optional `HasValue`/`TryGetValue` members agree with `Value`. A custom union that violates these rules produces silently wrong pattern-matching results — review custom unions like unsafe code.[^21]

```csharp
using System.Runtime.CompilerServices;

IntOrBool flag = new IntOrBool((bool?)true);
Console.WriteLine(flag switch { int i => $"int {i}", bool b => $"bool {b}", null => "none" }); // bool True
Console.WriteLine(new IntOrBool((int?)null).Value is null);                                     // True

[Union]
public readonly struct IntOrBool : IUnion
{
    private readonly int _int;
    private readonly bool _bool;
    private readonly byte _tag;   // 0 = none, 1 = int, 2 = bool

    public IntOrBool(int? value)
    {
        if (value is { } v) { _int = v; _tag = 1; }
    }

    public IntOrBool(bool? value)
    {
        if (value is { } v) { _bool = v; _tag = 2; }
    }

    public object? Value => _tag switch { 1 => _int, 2 => _bool, _ => null };

    // Non-boxing access pattern: the compiler calls these for type and null patterns.
    public bool HasValue => _tag != 0;
    public bool TryGetValue(out int value) { value = _int; return _tag == 1; }
    public bool TryGetValue(out bool value) { value = _bool; return _tag == 2; }
}
```

Custom unions may also be **classes** (for class-based unions the `null` pattern matches both a null reference and a null `Value`) or delegate their union members to a nested `IUnionMembers` interface with static `Create` factory methods (useful when constructors must stay private). Review these with the same soundness rules.[^21]

At run time, `value is IUnion { Value: var inner }` inspects a boxed union generically. Compiler-generated unions always implement `IUnion`; custom unions do so only if they declare it (the compiler requires just `[Union]`, the constructors and `Value`).

##### Unions over the wire

`System.Text.Json` in .NET 11 serializes a union as its case value (**New in .NET 11**): `new Reading(5)` serializes to `5`, `new Reading("hello")` to `"hello"`. **The case type is not written**, so deserialization only works when the JSON token kind identifies the case. For `union Pet(Cat, Dog)` with two object-shaped cases, `JsonSerializer.Deserialize<Pet>` throws `JsonException: JSON value type 'Object' is ambiguous for union type 'Pet' because multiple case types can use this value type. Specify a custom type classifier...` (verified). Use `[JsonUnion(TypeClassifier = typeof(...))]` with a classifier, or model wire contracts with a closed hierarchy and `[JsonPolymorphic]`/`[JsonDerivedType]` discriminators. ASP.NET Core 11 accepts unions wherever it uses `System.Text.Json` (minimal API/MVC bodies, SignalR JSON protocol) but not for route values, query strings, headers or form fields; OpenAPI describes them as `anyOf`.[^12][^14]

---

#### Closed Hierarchies (New in C# 15)

`closed` on a class fixes the set of **direct** subclasses to those declared in the same assembly, so a `switch` that handles each direct subclass is exhaustive without a discard arm.[^22]

```csharp
Console.WriteLine(Describe(new Open(50)));       // 50% open
Console.WriteLine(Describe(new Shut()));         // closed

static string Describe(GateState state) => state switch
{
    Shut => "closed",
    Open(var percent) => $"{percent}% open",
    // no discard: every direct subclass of GateState is handled (adding one produces CS8509)
};

public closed record GateState;
public sealed record Shut : GateState;
public sealed record Open(float Percent) : GateState;
```

Verified rules (RC 2):

- `closed` needs `System.Runtime.CompilerServices.IsClosedTypeAttribute`, which only .NET 11 provides: against `net10.0` references the declaration fails with `CS0656`.
- A `closed` class is **implicitly abstract**: `new GateState()` is `CS0144`. `closed abstract` is `CS9384`, `closed sealed`/`closed static` is `CS9381`. `closed` is not valid on interfaces or structs (`CS0106`).
- Deriving from a closed class in **another assembly** is `CS9382: cannot use a closed type ... from another assembly as a base type` — also when the consuming project uses `LangVersion` 14 or the .NET 10.0.402 SDK compiler. The compiler makes the (implicit) constructor **protected**, marks it `[CompilerFeatureRequired("ClosedClasses")]`, and puts `[IsClosedType(DerivedTypes = ...)]` on the class; compilers that do not understand the feature refuse the constructor, but reflection emit and other languages that ignore `CompilerFeatureRequired` are not stopped. Treat "closed" as a compile-time contract, not a security boundary.
- **Not transitive**: a non-`closed` subclass (`class Poly : Shape`) can be derived from in other assemblies. Exhaustiveness still holds at the closed level (a `Poly` arm matches all its descendants), but if you switch on the subclasses of `Poly`, mark `Poly` `closed` too.
- Adding a subclass to a public closed hierarchy is a breaking change for consumers' switches, exactly like adding a union case.

```csharp
// BAD: the discard defeats exhaustiveness — a new subclass silently hits the exception at run time
static decimal FeeBad(PaymentMethod method) => method switch
{
    Card c => c.Amount * 0.029m,
    BankTransfer => 0.25m,
    _ => throw new ArgumentOutOfRangeException(nameof(method)),
};

// GOOD: no discard; adding "Wallet" turns every such switch into CS8509 at compile time
static decimal Fee(PaymentMethod method) => method switch
{
    Card c => c.Amount * 0.029m,
    BankTransfer => 0.25m,
};

Console.WriteLine(Fee(new Card(100m)) + FeeBad(new BankTransfer(5m)));   // 3.150

public closed record PaymentMethod(decimal Amount);
public sealed record Card(decimal Amount) : PaymentMethod(Amount);
public sealed record BankTransfer(decimal Amount) : PaymentMethod(Amount);
```

A closed hierarchy can be `null` at the reference level: the parameter `PaymentMethod method` is non-nullable, so the switch is exhaustive; if the parameter were `PaymentMethod?`, the compiler would require a `null` arm (`CS8655`).

---

#### Choosing Between Unions, Closed Hierarchies and Enums

| Need | Use |
|---|---|
| Alternatives that are **existing, unrelated types** (`int` or `string`, `Order` or `Error`), no shared behaviour | `union` |
| Alternatives that **share data or behaviour**, need reference identity or polymorphic JSON with a discriminator | `closed` record/class hierarchy |
| A fixed set of **named constants** without data | `enum` — but remember enums are not exhaustive: any `int` can be cast to an enum, so switches over enums still need a default arm (`CS8524`) |
| Callers in other assemblies must be able to add cases | ordinary abstract class or interface (open hierarchy) |

Do not wrap a closed hierarchy in a union of its subclasses — switch on the base type directly. Do not replace an interface with a union when callers rely on polymorphic dispatch (`shape.Area()`); unions only give you the "which case is it?" question.

---

#### Collection Expression Arguments (New in C# 15)

A `with(...)` element, which must come **first**, passes arguments to the target collection's constructor or builder method:[^23]

```csharp
string[] values = ["one", "two", "three"];

List<string> names = [with(capacity: values.Length * 2), .. values];
Console.WriteLine($"{names.Count}/{names.Capacity}");               // 3/6

HashSet<string> set = [with(StringComparer.OrdinalIgnoreCase), "Hello", "HELLO", "hello"];
Console.WriteLine(set.Count);                                      // 1

Dictionary<string, int> counts = [with(StringComparer.OrdinalIgnoreCase)];
counts["Apple"] = 1;
Console.WriteLine(counts["APPLE"]);                                // 1

IList<int> buffer = [with(capacity: 64), 1, 2];                    // mutable interface targets allow arguments
Console.WriteLine(buffer.Count);                                   // 2
```

Rules verified on RC 2:

- `with(...)` not in first position: `CS9354`.
- Arrays, spans and other targets without constructor arguments: `CS9355: 'with(...)' elements are not supported for type 'object[]'` / `'ReadOnlySpan<int>'`.
- Read-only interface targets (`IEnumerable<T>`, `IReadOnlyList<T>`) accept only an empty `with()`: `CS9357`.
- A `Dictionary<,>` collection expression can take constructor arguments but **cannot contain elements** in C# 15 (`CS9215`: `Dictionary` has no single-argument `Add`); dictionary element syntax (`["k": v]`) is not part of C# 15.
- **Breaking change**: inside a collection expression, `with(...)` now always means construction arguments. Code that called a method named `with` (`[with(x, y), z]`) must write `@with(x, y)`. The breaking-changes document says this applies "when the LangVersion is set to 15 or greater", but the compilers are stricter: verified, the .NET 10.0.402 compiler (Roslyn 5.9) already binds it as construction arguments under C# 14 (`CS8652` preview + `CS9355`), and the RC 2 compiler does the same under `LangVersion` 14 (`CS9327` + `CS9355`). The break therefore appears when upgrading to a 10.0.4xx or 11 SDK, regardless of `LangVersion`.[^8]

Review uses: passing a comparer (`StringComparer.Ordinal`/`OrdinalIgnoreCase`) to sets and dictionaries is a correctness issue, not style — a `HashSet<string>` built with the default comparer is case-sensitive ordinal, which may not be what the domain needs. Capacity hints are a performance nicety; only flag their absence on hot paths with known large sizes.

---

#### Extension Members and Extension Indexers

C# 14 introduced `extension` blocks (extension properties, static extension members, and operators); **C# 15 adds extension indexers**.[^6][^19]

```csharp
IEnumerable<int> numbers = Enumerable.Range(1, 10);
Console.WriteLine(numbers[2]);                 // 3 — extension indexer (C# 15)
Console.WriteLine(numbers.IsEmpty);            // False — extension property (C# 14)
Console.WriteLine(numbers.Describe());        // 1,2,3

var grid = new Grid(3, 3);
grid[(1, 1)] = 5;                              // extension indexer with a setter
Console.WriteLine(grid[(1, 1)]);               // 5

public static class SequenceExtensions
{
    extension(IEnumerable<int> sequence)
    {
        public int this[int index] => sequence.ElementAt(index);   // O(n) for non-lists: document it
        public bool IsEmpty => !sequence.Any();
    }

    extension(Grid grid)
    {
        public int this[(int Row, int Column) cell]
        {
            get => grid.Get(cell.Row, cell.Column);
            set => grid.Set(cell.Row, cell.Column, value);
        }
    }

    // Classic extension methods remain valid and interoperate with extension blocks.
    public static string Describe(this IEnumerable<int> sequence) => string.Join(",", sequence.Take(3));
}

public sealed class Grid(int rows, int columns)
{
    private readonly int[] _cells = new int[rows * columns];
    public int Get(int row, int column) => _cells[row * columns + column];
    public void Set(int row, int column, int value) => _cells[row * columns + column] = value;
}
```

Review notes:

- An extension indexer that calls `ElementAt` on an `IEnumerable<T>` hides an O(n) (or re-enumerating) operation behind `[]` syntax. Flag it on lazy sequences and database queries — indexers look O(1).
- Indexers are instance members, so the `extension(...)` block must name its receiver parameter.
- Members declared on the type win over extensions, and for indexers this is stricter than for methods: if the receiver type has **any** indexer, extension indexers are not considered at all. A `(int, int)` extension indexer on `int[,]` does not compile — `grid[(1, 1)]` reports `CS0022: Wrong number of indices` because only the array's own indexer is looked at (verified).

---

#### Labeled break and continue (New in C# 15)

`break label;` exits the enclosing loop or `switch` statement that carries the label; `continue label;` starts the next iteration of the enclosing **loop** that carries the label (a `continue` aimed at a labeled `switch` is `CS9394`, verified). This replaces flag variables and `goto` for nested loops.[^24]

```csharp
int[,] grid = { { 0, 0, 1 }, { 0, 2, 0 }, { 3, 0, 0 } };

// BAD (pre-C# 15 style): a flag checked at each level — IDE0410 suggests a labeled jump
(int Row, int Col)? FindFlag(int target)
{
    (int, int)? result = null;
    bool found = false;
    for (int r = 0; r < grid.GetLength(0); r++)
    {
        for (int c = 0; c < grid.GetLength(1); c++)
        {
            if (grid[r, c] == target) { result = (r, c); found = true; break; }
        }
        if (found) break;
    }
    return result;
}

// GOOD: name the loop you mean
(int Row, int Col)? Find(int target)
{
    (int, int)? result = null;
    rows: for (int r = 0; r < grid.GetLength(0); r++)
    {
        for (int c = 0; c < grid.GetLength(1); c++)
        {
            if (grid[r, c] == 1) continue rows;          // skip the rest of this row
            if (grid[r, c] == target) { result = (r, c); break rows; }
        }
    }
    return result;
}

Console.WriteLine(Find(2));        // (1, 1)
Console.WriteLine(Find(1));        // nothing found: row 0 is skipped at the 1
Console.WriteLine(FindFlag(3));    // (2, 0)
```

- The label must sit **directly on a loop or `switch` statement**; `break lbl;` targeting a labeled block is `CS9393`.
- A labeled `break` inside a `switch` section can leave an outer loop, which removes the classic "`break` only leaves the `switch`" bug.
- Unlabeled `break`/`continue` keep their old meaning. IDE0410 (new analyzer) flags flag-variable and `goto` patterns that a labeled jump can replace (verified with `EnforceCodeStyleInBuild` and severity `warning`).[^25]

---

#### Memory Safety Preview (Not for Production)

C# 15's documentation describes the start of a multi-release redesign of `unsafe`: pointer *types*, `&`, `fixed`, `stackalloc`-to-pointer and `sizeof` would no longer need an `unsafe` context; only dereferencing would. A member marked `unsafe` would become "requires-unsafe" for its callers; `unsafe(expression)` and a `safe` modifier are added.[^6][^26]

In the RC 2 SDK all of this is **preview**:

- `<LangVersion>15</LangVersion>`: `int* p = &x;` outside `unsafe` → `CS0214`; `unsafe(*p)` → `CS8652 ... 'updated memory safety rules' is currently in Preview and *unsupported*`.
- `<LangVersion>preview</LangVersion>` plus `<AllowUnsafeBlocks>true</AllowUnsafeBlocks>`: the relaxed rules compile and run.

Review rule: production code targets `15`; any `preview` language version, `updated-memory-safety-rules` feature flag or `unsafe(...)` expression is a blocker. Today's rules still apply: keep `unsafe` blocks minimal, prefer `Span<T>`, `MemoryMarshal` and `Unsafe` APIs only with bounds reasoning written down, and require `AllowUnsafeBlocks` to be justified. When the new rules arrive (C# 16), overload resolution can change (lambdas become convertible to delegates with pointer parameters), so expect new ambiguities (`CS0121`).[^8]

---

#### C# 14 Features Available in C# 15

```csharp
var person = new Person { Name = "  Ada " };
Console.WriteLine($"[{person.Name}]");                     // [Ada]

Customer? customer = Lookup("missing");
customer?.LastOrder = "A-1";                               // null-conditional assignment: no-op when null
customer = new Customer();
customer?.LastOrder = "A-2";
Console.WriteLine(customer?.LastOrder);                    // A-2

int[] numbers = [3, 1, 2];
Console.WriteLine(Sum(numbers));                           // 6: implicit array -> ReadOnlySpan<int>
Console.WriteLine(nameof(List<>));                         // List: unbound generic in nameof

TryParser parse = (text, out value) => int.TryParse(text, out value);  // modifiers without types
Console.WriteLine(parse("5", out var five) ? five : -1);   // 5

var total = new Money(500);
total += new Money(250);                                   // user-defined compound assignment, in place
Console.WriteLine(total.Cents);                            // 750

static Customer? Lookup(string id) => null;
static int Sum(ReadOnlySpan<int> values) { var s = 0; foreach (var v in values) s += v; return s; }

delegate bool TryParser(string text, out int value);

sealed class Person
{
    // 'field' refers to the compiler-generated backing field
    public string Name { get; set => field = value.Trim(); } = "";
}

sealed class Customer { public string? LastOrder { get; set; } }

sealed class Money(long cents)
{
    public long Cents { get; private set; } = cents;
    public void operator +=(Money other) => Cents += other.Cents;   // instance compound operator (C# 14)
}

partial class Widget
{
    public partial Widget(int size);                                   // partial constructor (C# 14)
}

partial class Widget
{
    public partial Widget(int size) { Size = size; }
    public int Size { get; }
}
```

Review notes for C# 14 features:

- `field` is a contextual keyword inside property accessors: an existing *field named `field`* now needs `this.field` or `@field` inside accessors.
- A user-defined `operator +=` mutates the left operand in place; on a shared reference this is visible to every holder. Prefer it only for types meant to be mutable accumulators.
- First-class spans change overload resolution: span extension methods now apply to arrays. Verified: in `Expression<Func<int[], bool>> e = a => a.Contains(1);` the call binds to `System.Linq.Enumerable.Contains` under C# 13 but to `System.MemoryExtensions.Contains` under C# 14 and 15. Expression-tree consumers that do not understand span methods fail on such trees: `e.Compile(preferInterpretation: true)` throws `ArgumentException` (a `ReadOnlySpan<int>` violates the interpreter's generic constraint) under C# 14/15 while it returns `True` under C# 13; `e.Compile()` still works (verified). Use current LINQ-provider versions, or write `Enumerable.Contains(a, 1)` explicitly in expressions handed to providers or interpreters.[^19]
- `customer?.LastOrder = value` evaluates the right-hand side only when `customer` is non-null — side effects in the right-hand side are skipped.

---

#### Nullable Reference Types

Nullable annotations are compile-time only: a `string` parameter can still receive `null` from callers compiled without nullable analysis, reflection, deserialization or `default!`. Validate at trust boundaries; rely on the analysis inside.

```csharp
using System.Diagnostics.CodeAnalysis;

var users = new Dictionary<int, User> { [1] = new("Ada", null) };

// BAD: the null-forgiving operator hides a real null
// string email = users[1].Email!.ToLowerInvariant();   // NullReferenceException at run time

// GOOD: handle the null explicitly
string email = users[1].Email?.ToLowerInvariant() ?? "(none)";
Console.WriteLine(email);

if (TryFind(users, 2, out var user))
{
    Console.WriteLine(user.Name);           // no warning: [NotNullWhen(true)] informs the analysis
}

Console.WriteLine(Normalize(null) ?? "null in, null out");

static bool TryFind(Dictionary<int, User> source, int id, [NotNullWhen(true)] out User? user) =>
    source.TryGetValue(id, out user);

[return: NotNullIfNotNull(nameof(input))]
static string? Normalize(string? input) => input?.Trim().ToUpperInvariant();

public sealed record User(string Name, string? Email);
```

Review rules:

- Every `!` (null-forgiving) needs a justification; `default!` / `null!` initializers on non-nullable properties are a smell — use `required` members or constructor parameters.
- Public APIs validate arguments (`ArgumentNullException.ThrowIfNull(arg)`) even with annotations, because callers may have nullable analysis disabled.
- `#nullable disable` in new files, or `<Nullable>disable</Nullable>`, is a regression.
- Use the flow attributes (`[NotNullWhen]`, `[MaybeNullWhen]`, `[NotNullIfNotNull]`, `[MemberNotNull]`, `[DoesNotReturn]`) instead of `!` at call sites.
- `required` (C# 11) forces object-initializer assignment, but **not** non-null values from deserializers that bypass it; `System.Text.Json` honours `required` (missing property → `JsonException`) and, with `RespectNullableAnnotations = true` (.NET 9+), rejects `null` for non-nullable reference properties.
- **New in C# 15**: a union's `Value` defaults to "maybe null" only if some case type is nullable; otherwise its default null state is "not null" — even for `default(Pet)`, whose `Value` is actually `null` at run time. Creating a union from a case value gives `Value` that value's null state, and a `true` result from `HasValue`/`TryGetValue` makes it "not null". Missing `null` arms on `default`-able unions are therefore invisible to the compiler — see the union section.[^21]

---

#### Records and Immutability

```csharp
var original = new Invoice("INV-1", [new Line("Pen", 2)]);
var copy = original with { Number = "INV-2" };

Console.WriteLine(original == copy with { Number = "INV-1" });   // True: same list instance is shared!
copy.Lines.Add(new Line("Ink", 1));                               // mutates original.Lines too
Console.WriteLine(original.Lines.Count);                          // 2

var a = new Snapshot("A", [1, 2]);
var b = new Snapshot("A", [1, 2]);
Console.WriteLine(a == b);                                        // False: arrays compare by reference

// 'with' is a shallow copy; collections inside records are shared and compare by reference.
public sealed record Invoice(string Number, List<Line> Lines);
public sealed record Line(string Item, int Quantity);
public sealed record Snapshot(string Name, int[] Values);
```

- Records give value equality over their **fields**; collection-typed members compare by reference, so `Equals`/`GetHashCode` on records holding `List<T>`/arrays is almost never what callers expect. Use `ImmutableArray<T>`/`IReadOnlyList<T>` and override `Equals(T? other)` + `GetHashCode()` if structural equality is required.
- `with` performs a **shallow** copy. Mutable members are shared between the original and the copy.
- `record struct` is mutable unless declared `readonly record struct`. Positional `record struct` properties are `get; set;`.
- Do not put records with mutable members into `HashSet`/`Dictionary` keys: mutation changes the hash code.
- `init` accessors and `required` members give immutable construction without positional parameters.
- Records work well as EF Core *value objects* (complex types) and DTOs. Do not use record classes as EF entities that rely on reference identity in the change tracker: record equality conflicts with entity identity.

---

#### Pattern Matching and Exhaustiveness

```csharp
Console.WriteLine(Shipping(new Order(150m, Priority: true, Country: "DE")));   // Express
Console.WriteLine(Classify([1, 2, 3]));                                         // starts with 1, ends with 3
Console.WriteLine(Classify([]));                                                // empty
Console.WriteLine(Grade(85));                                                   // B

static string Shipping(Order order) => order switch
{
    { Total: > 100m, Priority: true } => "Express",
    { Total: > 100m } => "Standard",
    { Country: "DE" or "AT" or "CH" } => "DACH",
    _ => "Economy",
};

static string Classify(int[] values) => values switch
{
    [] => "empty",
    [var only] => $"single {only}",
    [1, .., var last] => $"starts with 1, ends with {last}",
    [_, _, ..] => "two or more",
};

static char Grade(int score) => score switch
{
    >= 90 => 'A',
    >= 80 and < 90 => 'B',
    >= 70 => 'C',
    _ => 'F',
};

public sealed record Order(decimal Total, bool Priority, string Country);
```

Review rules:

- Arm order matters: the first matching arm wins, and the compiler reports unreachable arms (`CS8510`) only for fully subsumed patterns. A broad arm (`{ Total: > 100m }`) placed before a narrower one silently swallows it when the compiler cannot prove subsumption (for example with `when` clauses).
- **Exhaustiveness** (`CS8509`) is reliable for unions, closed hierarchies, `bool`, and relational patterns over numbers; it is not reliable for enums (any integer is a valid enum value, `CS8524`) or open hierarchies — those need a discard arm that throws.
- `is not null` and `is { }` both test for non-null; `is var x` always matches (including `null`).
- Property patterns on nullable members: `{ Address.City: "Berlin" }` (extended property pattern, C# 10) is null-safe — it does not throw if `Address` is `null`.
- `switch` statements are not checked for exhaustiveness: for unions and closed hierarchies prefer switch expressions.
- Pattern matching on a union unwraps `Value` (see above); `when` clauses run after the pattern matched and should be side-effect free.

---

#### Exceptions

```csharp
try
{
    await ProcessAsync("orders.csv");
}
catch (FileNotFoundException ex)                   // most specific first
{
    Console.WriteLine($"missing: {ex.FileName}");
}
catch (IOException ex) when (ex.HResult == unchecked((int)0x80070020))   // Windows sharing violation; filter: no unwinding when false
{
    Console.WriteLine("file locked");
}

static async Task ProcessAsync(string path)
{
    try
    {
        await using var stream = File.OpenRead(path);
    }
    catch (UnauthorizedAccessException ex)
    {
        // GOOD: wrap with context and keep the inner exception
        throw new InvalidOperationException($"Cannot read '{path}'", ex);
    }
}
```

```csharp
// BAD patterns — each one is a review finding
static void Bad(Action work)
{
    try { work(); }
    catch (Exception ex)
    {
        Console.WriteLine(ex.Message);
        throw ex;            // resets the stack trace (CA2200): use 'throw;'
    }
}

static void Swallow(Action work)
{
    try { work(); }
    catch { }                // swallows everything, including bugs (CA1031)
}

static void Good(Action work)
{
    try { work(); }
    catch (Exception ex) when (ex is not OperationCanceledException)
    {
        Console.Error.WriteLine(ex);
        throw;               // preserves the stack trace
    }
}

Good(() => Console.WriteLine("ok"));
```

- Throw the most specific built-in exception (`ArgumentException`, `ArgumentOutOfRangeException`, `InvalidOperationException`, `KeyNotFoundException`, `NotSupportedException`) or a domain exception deriving from `Exception`. Never throw `Exception`, `SystemException`, `NullReferenceException` or `IndexOutOfRangeException` yourself.
- Use guard helpers: `ArgumentNullException.ThrowIfNull`, `ArgumentException.ThrowIfNullOrEmpty`/`ThrowIfNullOrWhiteSpace`, `ArgumentOutOfRangeException.ThrowIfNegative`/`ThrowIfZero`/`ThrowIfGreaterThan`, `ObjectDisposedException.ThrowIf`.
- `catch (Exception)` is acceptable only at process/request boundaries (middleware, top of a background loop) and must log and either rethrow or translate. Always let `OperationCanceledException` from the caller's token propagate.
- `ExceptionDispatchInfo.Capture(ex).Throw()` rethrows a captured exception on another thread with its original stack.
- Exceptions are for exceptional paths: do not use them for control flow on hot paths (use `TryParse`/`TryGetValue` patterns or result unions).
- **New in .NET 11**: `DateOnly.TryParse`/`TryParseExact` and `TimeOnly.TryParse`/`TryParseExact` **throw** `ArgumentException` for invalid `DateTimeStyles` or format specifiers instead of returning `false` (verified: `.NET 10` returns `False`, `.NET 11` throws). A `Try*` method throwing is now expected for *programming* errors.[^27]

---

#### Async/Await and Runtime Async

##### Rules that have not changed

```csharp
using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(5));
var service = new DashboardService();
var data = await service.LoadAsync(cts.Token);
Console.WriteLine($"{data.Users} users, {data.Orders} orders");

public sealed class DashboardService
{
    // Independent operations run concurrently; await each task after WhenAll for its result.
    public async Task<Dashboard> LoadAsync(CancellationToken ct)
    {
        Task<int> users = CountUsersAsync(ct);
        Task<int> orders = CountOrdersAsync(ct);
        await Task.WhenAll(users, orders);
        return new Dashboard(await users, await orders);
    }

    private static async Task<int> CountUsersAsync(CancellationToken ct) { await Task.Delay(10, ct); return 42; }
    private static async Task<int> CountOrdersAsync(CancellationToken ct) { await Task.Delay(10, ct); return 7; }
}

public sealed record Dashboard(int Users, int Orders);
```

Review checklist for async code:

- **Never block on async code**: `.Result`, `.Wait()`, `.GetAwaiter().GetResult()` on an incomplete task risks deadlocks (UI/legacy sync contexts) and thread-pool starvation (ASP.NET Core). `Task.Run(...).Result` inside request handling is the classic starvation bug.
- **No `async void`** except event handlers; exceptions from `async void` crash the process. Lambdas passed to `Action` parameters (`list.ForEach(async x => ...)`, `Parallel.ForEach(..., async x => ...)`) are `async void` in disguise.
- **Fire-and-forget** (`_ = DoAsync();`) loses exceptions and outlives the request scope (disposed `DbContext`, `HttpContext`). Use a `BackgroundService` with a `Channel<T>` queue instead.
- `ConfigureAwait(false)` in **library** code that may be called from a context with a `SynchronizationContext`; application code in ASP.NET Core has no context and does not need it. `ConfigureAwait(ConfigureAwaitOptions.SuppressThrowing)` (.NET 8) awaits without throwing — only for `Task`, not `Task<T>`.
- `Task.WhenAll` rethrows only the **first** exception when awaited; inspect `task.Exception.InnerExceptions` (or each task) for all of them. `Task.WhenEach` (.NET 9) processes tasks as they complete.
- **`ValueTask`**: await it exactly once; never await twice, never call `.Result` before completion, never store it — call `.AsTask()` if you need any of that.
- Return the task directly (`Task<int> F() => G();`) only when there is no `using`/`try` around it; otherwise the resource is disposed before the inner task finishes. Prefer `async`/`await` by default.
- Pass `CancellationToken` through every async call chain; accept it as the last parameter named `cancellationToken` (CA2016 flags tokens not forwarded).
- `Task.Run` in ASP.NET Core request code only moves work to another thread-pool thread — it adds overhead without freeing resources. Use it for CPU-bound work in UI apps or to offload genuinely blocking APIs.
- `lock` cannot contain `await` (`CS1996`); use `SemaphoreSlim.WaitAsync`.
- `IAsyncDisposable` resources need `await using`.

##### Runtime Async (New in .NET 11)

.NET 11 can implement `async` methods in the runtime instead of compiler-generated state machines. The BCL itself is compiled this way; application and library code **opt in**:[^9]

```xml
<PropertyGroup>
  <TargetFramework>net11.0</TargetFramework>
  <Features>runtime-async=on</Features>
</PropertyGroup>
```

Verified on RC 2 with three nested `async Task` methods that complete synchronously:

| Build | Nested compiler-generated types | `new StackTrace().FrameCount` in the innermost method |
|---|---|---|
| default `net11.0` (no flag) | 4 state machines | 13 frames (state-machine `MoveNext`/`Start` frames visible) |
| `-features:runtime-async=on` | 0 | 5 frames (just the real call chain) |

- `EnablePreviewFeatures` is no longer required for `net11.0`, but the runtime docs still describe Runtime Async as "a preview feature", and it is **not** on by default for your code. Against `net10.0` references the same flag fails with `SYSLIB5007` (`AsyncHelpers` is "for evaluation purposes only") plus `CA2252` (preview features) (verified).
- The docs name `<UseRuntimeAsync>false</UseRuntimeAsync>` as the per-project opt-out; the RC 2 SDK's MSBuild targets do not reference that property, and `<UseRuntimeAsync>true</UseRuntimeAsync>` does **not** opt in (verified: state machines are still emitted). Only the `Features` flag above changes code generation.
- Observable semantics are unchanged in the cases checked: `AsyncLocal<T>` values set inside an awaited method do not flow back to the caller, exceptions are stored in the returned task (not thrown at the call site), and `await` rethrows them.
- Benefits: smaller code, fewer allocations, cleaner *live* stack traces for profilers and debuggers. Exception stack traces (`ex.StackTrace`) already looked clean before.
- The removed `DOTNET_RuntimeAsync`/`UNSUPPORTED_RuntimeAsync` environment variables no longer do anything.
- Review stance: opting in is reasonable for applications after load testing; libraries should wait for GA guidance. Code that inspects stack frames or async state-machine types via reflection (logging helpers, some APM agents, `AsyncStateMachineAttribute` checks) changes behaviour — verified: under runtime async the method has no `AsyncStateMachineAttribute` and no nested state-machine type; instead `MethodInfo.MethodImplementationFlags` reports `Async`.

Also **new in .NET 11** for all async code: continuations skip `ExecutionContext` capture/restore when no `AsyncLocal<T>` state is in use, and the JIT recognizes `Task.FromResult`, `Task.CompletedTask` and `ValueTask.FromResult`. No code change is needed.[^9]

---

#### Cancellation, Timeouts and TimeProvider

```csharp
using var shutdown = new CancellationTokenSource();
var poller = new Poller(TimeProvider.System);
var result = await poller.FetchWithTimeoutAsync(TimeSpan.FromMilliseconds(200), shutdown.Token);
Console.WriteLine(result);   // timed out

public sealed class Poller(TimeProvider time)
{
    public async Task<string> FetchWithTimeoutAsync(TimeSpan timeout, CancellationToken cancellationToken)
    {
        // Link the caller's token with a timeout; dispose the linked source.
        using var timeoutCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeoutCts.CancelAfter(timeout);
        try
        {
            await Task.Delay(TimeSpan.FromSeconds(10), time, timeoutCts.Token);
            return "done";
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            return "timed out";   // our timeout fired, not the caller's cancellation
        }
    }
}
```

- Distinguish *caller cancellation* (let `OperationCanceledException` propagate) from *your own timeout* (translate to `TimeoutException` or a result). The `when (!cancellationToken.IsCancellationRequested)` filter is the standard idiom.
- `CancellationTokenSource` with `CancelAfter` or a timer must be disposed. `CreateLinkedTokenSource` registrations leak if not disposed.
- `CancellationToken.None` in production code that has a token available is a bug (CA2016).
- `token.ThrowIfCancellationRequested()` inside CPU-bound loops; `token.Register(...)` callbacks must not throw and must be disposed.
- Inject **`TimeProvider`** (.NET 8) instead of calling `DateTime.UtcNow`, `Task.Delay(TimeSpan)` or `new Timer` directly: `time.GetUtcNow()`, `Task.Delay(delay, time, ct)`, `time.CreateTimer(...)`, `new CancellationTokenSource(delay, time)`, `Task.WaitAsync(timeout, time, ct)`. Tests use `FakeTimeProvider` (`Microsoft.Extensions.TimeProvider.Testing`).
- `Task.WaitAsync(TimeSpan)` gives up *waiting*; it does not cancel the underlying operation — pass a token to the operation itself.

---

#### IAsyncEnumerable and Async LINQ

```csharp
using System.Runtime.CompilerServices;

using var cts = new CancellationTokenSource();
await foreach (var page in ReadPagesAsync(3).WithCancellation(cts.Token))
{
    Console.WriteLine(page);
}

// System.Linq.AsyncEnumerable is part of the BCL since .NET 10: no System.Linq.Async package
var evens = await ReadPagesAsync(6).Where(p => p % 2 == 0).Select(p => p * 10).ToListAsync();
Console.WriteLine(string.Join(",", evens));   // 20,40,60

static async IAsyncEnumerable<int> ReadPagesAsync(
    int count,
    [EnumeratorCancellation] CancellationToken cancellationToken = default)
{
    for (var i = 1; i <= count; i++)
    {
        await Task.Delay(1, cancellationToken);
        yield return i;
    }
}
```

- Without `[EnumeratorCancellation]`, a token passed via `.WithCancellation(token)` never reaches the iterator (CS8425 warns).
- `System.Linq.AsyncEnumerable` ships in the .NET 10+ BCL. The community `System.Linq.Async` 6.x package defines the same extension methods and causes ambiguous-call errors on `net10.0`+; remove it, or move to `System.Linq.Async` 7.x, whose `net10.0` asset no longer duplicates the BCL operators.[^28]
- **New in .NET 11**: `AsyncEnumerable` gains `FullJoin` and tuple-returning `Join`/`GroupJoin` like `Enumerable`.[^12]
- EF Core: use `AsAsyncEnumerable()` on queries, not `ToAsyncEnumerable()` — EF Core 11 ships an analyzer and code fix for this.[^16]
- **New in .NET 11**: `JsonSerializer.SerializeAsyncEnumerable(stream, source, topLevelValues: true)` writes JSON Lines (one value per line).[^12]

---

#### Concurrency and Synchronization

```csharp
using System.Threading.Channels;

var counter = new Counter();
await Parallel.ForEachAsync(Enumerable.Range(0, 1000), new ParallelOptions { MaxDegreeOfParallelism = 8 },
    async (i, ct) => { counter.Increment(); await Task.Yield(); });
Console.WriteLine(counter.Value);          // 1000

// Bounded channel: producer/consumer with back-pressure
var channel = Channel.CreateBounded<int>(new BoundedChannelOptions(100) { FullMode = BoundedChannelFullMode.Wait });
var consumer = Task.Run(async () =>
{
    var sum = 0;
    await foreach (var item in channel.Reader.ReadAllAsync()) sum += item;
    return sum;
});
for (var i = 1; i <= 10; i++) await channel.Writer.WriteAsync(i);
channel.Writer.Complete();
Console.WriteLine(await consumer);         // 55

public sealed class Counter
{
    private readonly Lock _gate = new();   // System.Threading.Lock (.NET 9 / C# 13)
    private int _value;

    public void Increment()
    {
        lock (_gate) { _value++; }         // compiles to Lock.EnterScope(), not Monitor
    }

    public int Value { get { lock (_gate) { return _value; } } }
}
```

- Prefer `System.Threading.Lock` for new private lock objects (.NET 9+). Never `lock (this)`, `lock (typeof(T))` or `lock` on strings. Casting a `Lock` to `object` and locking on it falls back to `Monitor` (warning `CS9216`).
- Simple counters: `Interlocked.Increment(ref _count)`.
- `ConcurrentDictionary.GetOrAdd(key, factory)` may run the factory more than once under contention; wrap the value in `Lazy<T>` when the factory is expensive or has side effects.
- `Parallel.ForEach` with an async lambda is `async void`: use `Parallel.ForEachAsync`.
- Throttle concurrent I/O with `SemaphoreSlim` (release in `finally`) or `Parallel.ForEachAsync`'s `MaxDegreeOfParallelism`.
- `static` mutable fields in services are shared across requests; ASP.NET Core singletons must be thread-safe.
- `Channel.CreateUnbounded` lets a fast producer exhaust memory; use bounded channels in services.
- **New in .NET 11**: named mutexes (`new Mutex(false, "name")`) on Unix default to **user scope**, so processes running as different users no longer share them by default.[^11]

---

#### Span, Memory and Allocation-Free Code

```csharp
using System.Buffers;
using System.Text;

Console.WriteLine(SumCsv("10,20,30"));                    // 60
Console.WriteLine(Checksum("hello"u8));                   // 532

var vowels = SearchValues.Create("aeiouAEIOU");
Console.WriteLine("rhythm and blues".AsSpan().IndexOfAny(vowels));   // 7

// Rent large temporary buffers instead of allocating; always return them.
byte[] rented = ArrayPool<byte>.Shared.Rent(4096);
try
{
    int written = Encoding.UTF8.GetBytes("payload", rented);
    Console.WriteLine(written);                           // 7
}
finally
{
    ArrayPool<byte>.Shared.Return(rented, clearArray: true);   // clear if it held secrets
}

static int SumCsv(ReadOnlySpan<char> text)
{
    var total = 0;
    foreach (Range range in text.Split(','))              // MemoryExtensions.Split (.NET 9): no string allocations
    {
        total += int.Parse(text[range]);
    }
    return total;
}

static int Checksum(ReadOnlySpan<byte> data)
{
    // Small, bounded stackalloc only; never size it from untrusted input.
    Span<byte> scratch = stackalloc byte[16];
    var sum = 0;
    foreach (var b in data) sum += b;
    scratch[0] = (byte)sum;
    return sum;
}
```

Rules for review:

- `Span<T>`/`ReadOnlySpan<T>` are `ref struct`s: no fields in classes, no boxing, no capture in lambdas; since C# 13 they can be locals in `async` methods and iterators as long as they do not live across `await`/`yield`. Use `Memory<T>` when a buffer must survive an `await`.
- `stackalloc` sizes must be small constants or validated upper bounds (`len <= 256 ? stackalloc byte[len] : new byte[len]`); an unbounded `stackalloc` from input is a stack-overflow DoS.
- `ArrayPool<T>.Rent` may return a **larger** array — slice to the requested length; never use the array after `Return`; return it exactly once.
- `MemoryMarshal`/`Unsafe`/`CollectionsMarshal.AsSpan(list)` bypass safety: a span over a `List<T>`'s buffer becomes stale if the list grows.
- **Breaking change in the .NET 11 compiler**: a collection expression targeting `Span<T>`/`ReadOnlySpan<T>` now has *declaration-block* safe-context. Assigning `Span<int> items = [x];` declared inside a loop body to a `scoped` variable declared outside the loop is now an error; use an array or move the declaration.[^8]
- **New in .NET 11**: `StringStream` and `ReadOnlyMemoryStream` expose strings and memory as `Stream` without copying; `BitArray` gains span constructors.[^12]

---

#### Collections and Collection Expressions

```csharp
using System.Collections.Frozen;
using System.Collections.Immutable;

int[] primes = [2, 3, 5, 7];
List<int> more = [.. primes, 11, 13];                     // spread
ImmutableArray<string> tags = ["a", "b"];                  // works for immutable and builder-based types
ReadOnlySpan<int> stackOnly = [1, 2, 3];                   // may be stack/inline-array backed
IReadOnlyList<int> readOnly = [1, 2, 3];                   // compiler picks an implementation

// Frozen collections: build once at startup, read many times
FrozenDictionary<string, int> statusCodes = new Dictionary<string, int>(StringComparer.OrdinalIgnoreCase)
{
    ["ok"] = 200,
    ["not-found"] = 404,
}.ToFrozenDictionary(StringComparer.OrdinalIgnoreCase);

Console.WriteLine(statusCodes["NOT-FOUND"]);               // 404
Console.WriteLine($"{more.Count} {tags.Length} {stackOnly.Length} {readOnly.Count}");

// OrderedDictionary<TKey,TValue> (.NET 9): insertion order + index access
var ordered = new OrderedDictionary<string, int> { ["z"] = 1, ["a"] = 2 };
Console.WriteLine(string.Join(",", ordered.Keys));         // z,a
Console.WriteLine(ordered.GetAt(1));                       // [a, 2]
```

- Do not mutate a `List<T>`/`Dictionary<,>` while enumerating it (`InvalidOperationException`); collect changes first. `Dictionary<,>.Remove` during `foreach` is allowed since .NET Core 3.0, but other mutations are not.
- Returning `IReadOnlyList<T>` over a `List<T>` is not immutability: callers can cast back. Return `ImmutableArray<T>`/`FrozenSet<T>` or a copy for real immutability.
- `Dictionary<string, T>`/`HashSet<string>` keys from users need an explicit comparer: `StringComparer.Ordinal` or `OrdinalIgnoreCase`, never culture-sensitive comparers for identifiers. In C# 15, pass it with `[with(StringComparer.OrdinalIgnoreCase)]` or the constructor.
- `ToFrozenDictionary`/`ToFrozenSet` are slower to build and faster to read: use them for static lookup tables, not for per-request data.
- `ImmutableList<T>` is a tree (O(log n) indexing); prefer `ImmutableArray<T>` for small, read-mostly data.
- Collection expressions on `IEnumerable<T>`/`IReadOnlyList<T>` targets produce compiler-chosen types; do not cast the result back to `List<T>` or arrays.
- **New in .NET 11**: `EqualityComparer<T>.Create(keySelector)` builds an equality comparer from a key (`EqualityComparer<(string Name, int Age)>.Create(p => p.Name)`); .NET 8 already had `EqualityComparer<T>.Create(equals, getHashCode)`.[^12]

---

#### LINQ

```csharp
var products = new[] { (Id: 1, Name: "Laptop", Category: (string?)"Electronics"), (Id: 2, Name: "Orphan", Category: (string?)null) };
var categories = new[] { (Name: "Electronics", Description: "Devices"), (Name: "Furniture", Description: "Office") };

// New in .NET 11: FullJoin returns (outer, inner) tuples; missing sides are default
foreach (var (product, category) in products.FullJoin(categories, p => p.Category, c => c.Name))
{
    Console.WriteLine($"{product.Name ?? "(none)"} -> {category.Description ?? "(none)"}");
}
// Laptop -> Devices
// Orphan -> (none)
// (none) -> Office

// .NET 10: LeftJoin / RightJoin with a result selector
var left = products.LeftJoin(categories, p => p.Category, c => c.Name, (p, c) => $"{p.Name}:{c.Description ?? "-"}");
Console.WriteLine(string.Join(" ", left));                 // Laptop:Devices Orphan:-

// .NET 9: CountBy, AggregateBy, Index
var words = new[] { "apple", "avocado", "banana" };
foreach (var (letter, count) in words.CountBy(w => w[0])) Console.Write($"{letter}={count} ");   // a=2 b=1
Console.WriteLine();
foreach (var (index, word) in words.Index()) Console.Write($"{index}:{word} ");
Console.WriteLine();
```

Pitfalls a reviewer must catch:

```csharp
// 1. Multiple enumeration of a lazy/expensive sequence
var evaluations = 0;
var query = Enumerable.Range(1, 3).Where(x => { evaluations++; return x > 1; });
if (query.Any()) Console.WriteLine(query.Count());         // 2
Console.WriteLine(evaluations);                            // 5: Any() ran the predicate twice, Count() three times
var list = query.ToList();                                 // GOOD: materialize once, then use the list
if (list.Count > 0) Console.WriteLine(list.Count);

// 2. Deferred execution captures variables, not values
var threshold = 1;
var filtered = new[] { 1, 2, 3 }.Where(x => x > threshold);
threshold = 2;
Console.WriteLine(filtered.Count());                       // 1, not 2

// 3. First() vs FirstOrDefault(): pick by intent
int[] empty = [];
Console.WriteLine(empty.FirstOrDefault());                 // 0 — indistinguishable from a real 0
Console.WriteLine(empty.FirstOrDefault(-1));               // -1 (default-value overload, .NET 6)

// 4. Count() on a materialized collection: use the property
List<int> items = [1, 2, 3];
Console.WriteLine(items.Count);                            // property, not Count()
```

- `Any()` instead of `Count() > 0` for lazy sequences; the `Count`/`Length` property for materialized collections (CA1827/CA1829/CA1860).
- `Single` when exactly one is a business rule; `First`/`FirstOrDefault` otherwise. Never `FirstOrDefault()` followed by `!`.
- `OrderBy(x => x)` → `Order()`; `Where(...).First()` → `First(...)`; `Select(...).Where(...)` order affects how much work runs — filter first.
- `GroupBy` materializes groups; on large data prefer `CountBy`/`AggregateBy` (.NET 9).
- `array.Reverse()` and first-class spans: with C# 14+, span extension methods apply to arrays, so on an older BCL `array.Reverse();` binds to `MemoryExtensions.Reverse(Span<T>)` and reverses **in place** (verified with `LangVersion` 14 against `net9.0` references: the array prints `3,2,1`, and `var r = array.Reverse();` is `CS0815`). .NET 10+ adds an `Enumerable.Reverse<T>(this T[])` overload, so on `net10.0`/`net11.0` it returns a lazy reversed sequence and leaves the array unchanged (verified). Watch for this in multi-targeted code and in projects that raise `LangVersion` on older TFMs.
- LINQ on `IQueryable` (EF Core) is translated: client-side methods in predicates throw or evaluate in memory. See [EF Core 11](#ef-core-11).
- **New in .NET 11**: `FullJoin`, tuple-returning `Join`/`GroupJoin` overloads (with optional comparer) on `Enumerable`, `Queryable` and `AsyncEnumerable`; EF Core 11 translates `FullJoin` to `FULL JOIN`.[^12][^16]

---

#### Strings, Culture and Comparison

```csharp
using System.Globalization;
using System.Text;

// Ordinal comparison for identifiers, keys, protocols, file paths
Console.WriteLine(string.Equals("FILE.txt", "file.TXT", StringComparison.OrdinalIgnoreCase));   // True
Console.WriteLine("Straße".StartsWith("STRASSE", StringComparison.OrdinalIgnoreCase));          // False (ordinal)

// Culture-sensitive formatting and parsing only for user-facing text
var german = CultureInfo.GetCultureInfo("de-DE");
Console.WriteLine(1234.5m.ToString("N2", german));                       // 1.234,50
Console.WriteLine(decimal.Parse("1234.5", CultureInfo.InvariantCulture)); // 1234.5

// Build strings in loops with StringBuilder or string.Join / string.Create
var sb = new StringBuilder();
for (var i = 0; i < 3; i++) sb.Append(i).Append(';');
Console.WriteLine(sb.ToString());                                         // 0;1;2;

// Raw string literals for JSON/SQL/regex templates (C# 11)
var json = """{"name": "Ada", "tags": ["x"]}""";
Console.WriteLine(json.Length);
```

- Every `string.Compare`, `StartsWith`, `EndsWith`, `IndexOf(string)` and `ToUpper`/`ToLower` call should state its intent: `StringComparison.Ordinal`/`OrdinalIgnoreCase` for machine strings, `ToUpperInvariant()`/`ToLowerInvariant()` for normalization (CA1307/CA1309/CA1310/CA1304/CA1311). Default culture-sensitive comparisons give different results under `tr-TR` and in ICU vs NLS.
- `int.Parse`/`decimal.Parse`/`DateTime.Parse` without an `IFormatProvider` use the current culture: always pass `CultureInfo.InvariantCulture` for machine data (CA1305).
- Globalization-invariant mode (`<InvariantGlobalization>true</InvariantGlobalization>`, set by the `webapiaot` and `console --aot` templates — verified on the RC 2 SDK) makes all cultures behave like invariant — culture-specific formatting silently disappears.
- `Regex`: use the `[GeneratedRegex]` source generator for fixed patterns, always set a timeout (`matchTimeout`) or `RegexOptions.NonBacktracking` for patterns applied to untrusted input (ReDoS).
- `string.Format`/interpolation with user data in log templates or SQL is an injection risk — see Logging and EF Core.

---

#### Numerics, Dates and Times

##### Behaviour changes in .NET 11 (verified 10.0.13 vs 11 RC 2)

```csharp
static short ToShort(double value) => unchecked((short)value);
static byte ToByte(double value) => unchecked((byte)value);

Console.WriteLine(ToShort(65578.0));    // .NET 10: 42      .NET 11: 32767 (saturates)
Console.WriteLine(ToByte(298));         // .NET 10: 42      .NET 11: 255
Console.WriteLine(ToByte(-42));         // .NET 10: 214     .NET 11: 0
Console.WriteLine(ToByte(double.NaN));  // 0 on both

Console.WriteLine(Math.Round(2.675, 2));                                  // .NET 10: 2.68   .NET 11: 2.67
Console.WriteLine(Math.Round(655.925, 2, MidpointRounding.AwayFromZero)); // .NET 11: 655.92 (correct for the stored binary value)
Console.WriteLine(Math.Round(1.5, 16));                                   // .NET 11: 1.5 (digits > 15 no longer throws)

Console.WriteLine(checked((int)2.9));   // 2: in-range conversions truncate toward zero on all versions
```

- Unchecked float→small-integer conversions now **saturate** (sbyte/byte/short/ushort/char); `NaN` → 0. Code relying on wrap-around (hashing, pixel math) changes results. Use `checked` to detect overflow, or `double.ConvertToIntegerNative<T>` if platform-specific behaviour is intended. Mono is not included in this change.[^29]
- `Math.Round(double, digits[, mode])` and `MathF.Round` now round the **exact** binary value: `2.675` is stored as `2.67499999…`, so the correct result is `2.67`. Money must use `decimal` (`Math.Round(2.675m, 2)` is `2.68` on every version).[^30]
- `decimal`/`BigInteger` ↔ floating-point conversions are now correctly rounded; `Complex` special values follow C23 Annex G.[^11]
- **New in .NET 11**: `System.Numerics.Decimal32`/`Decimal64`/`Decimal128` (IEEE 754 decimal floating point, with `System.Text.Json` converters), generic `Complex<T>`, `INumberBase<T>.TryParsePartial`, `Random.NextInteger<T>`/`NextBinaryFloat<T>`. These are new types; do not replace `decimal` for money without a reason.[^12]

##### Money and floating point

```csharp
decimal price = 19.99m;
decimal total = price * 3;                                  // 59.97 exactly
Console.WriteLine(total);

double d = 0.1 + 0.2;
Console.WriteLine(d == 0.3);                                // False
Console.WriteLine(Math.Abs(d - 0.3) < 1e-9);                // compare with a tolerance

Console.WriteLine(int.MaxValue + Random.Shared.Next(1, 2)); // wraps silently: -2147483648
Console.WriteLine(checked(int.MaxValue - 1 + 1));           // 2147483647
```

- `decimal` for money; never `double`/`float`. Integer overflow wraps silently unless the code is `checked` (or `<CheckForOverflowUnderflow>true</CheckForOverflowUnderflow>`); use `checked` for counters/sizes derived from input.
- `Random.Shared` is thread-safe; individual `Random` instances are not. `new Random()` is randomly seeded on .NET (Core), so the .NET Framework same-seed problem is gone, but no `Random` is suitable for security. Use `RandomNumberGenerator` for tokens, keys and anything security-relevant.

##### Dates and times

```csharp
using System.Globalization;

DateTimeOffset now = TimeProvider.System.GetUtcNow();         // inject TimeProvider in real code
DateOnly due = DateOnly.FromDateTime(now.UtcDateTime).AddDays(30);
Console.WriteLine(due > DateOnly.FromDateTime(now.UtcDateTime));   // True

var berlin = TimeZoneInfo.FindSystemTimeZoneById("Europe/Berlin");   // IANA IDs work on all platforms (.NET 6+)
Console.WriteLine(TimeZoneInfo.ConvertTime(new DateTimeOffset(2026, 10, 25, 0, 30, 0, TimeSpan.Zero), berlin).Offset); // 02:00:00

bool ok = DateOnly.TryParseExact("2026-10-22", "yyyy-MM-dd", CultureInfo.InvariantCulture, DateTimeStyles.None, out var parsed);
Console.WriteLine($"{ok} {parsed:O}");                         // True 2026-10-22

try
{
    DateOnly.TryParseExact("2026-10-22", "yyyy-MM-dd", CultureInfo.InvariantCulture, (DateTimeStyles)999, out _);
}
catch (ArgumentException)
{
    Console.WriteLine("New in .NET 11: invalid styles throw instead of returning false");
}
```

- Store and transmit instants as `DateTimeOffset` (or UTC `DateTime` with `Kind == Utc`); `DateTime.Now` in server code is a bug magnet (DST, container time zones). `DateOnly`/`TimeOnly` for calendar dates and wall-clock times.
- `DateTime` comparisons ignore `Kind`: comparing a local and a UTC `DateTime` gives wrong answers silently.
- `TimeZoneInfo.FindSystemTimeZoneById` accepts IANA and Windows IDs (with ICU); conversions across DST use the rules of the target zone.

---

#### IDisposable and IAsyncDisposable

```csharp
await using (var resource = new AuditLog("audit.log"))
{
    await resource.WriteAsync("started");
}
Console.WriteLine(File.ReadAllText("audit.log").Trim());   // started
File.Delete("audit.log");

public sealed class AuditLog : IAsyncDisposable, IDisposable
{
    private readonly StreamWriter _writer;
    private bool _disposed;

    public AuditLog(string path) => _writer = new StreamWriter(path, append: false);

    public async Task WriteAsync(string line)
    {
        ObjectDisposedException.ThrowIf(_disposed, this);
        await _writer.WriteLineAsync(line);
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed) return;
        _disposed = true;
        await _writer.DisposeAsync();     // flushes asynchronously
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        _writer.Dispose();
    }
}
```

- `sealed` classes that only own managed resources need just `Dispose()` (no finalizer, no `Dispose(bool)`); the full `Dispose(bool)` pattern is for unsealed base classes and types that own unmanaged handles (prefer `SafeHandle` over finalizers).
- Every `IDisposable` created in a method is disposed with `using`/`await using` (CA2000), unless ownership is transferred and documented.
- Types that implement `IAsyncDisposable` must be disposed with `await using`; a plain `using` on an async-only disposable is a compile error, but on a type with both, `using` calls the synchronous `Dispose`, which may block on I/O (e.g., `DbContext`, `Utf8JsonWriter` over a network stream).
- Do not dispose objects you do not own: DI-resolved services are disposed by the container; disposing an injected `HttpClient` from `IHttpClientFactory` is harmless, but disposing a shared singleton is a bug.
- `using var` disposes at the end of the enclosing scope — in a long method, that can be much later than intended.

---

#### Dependency Injection and Options

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Options;

var builder = Host.CreateApplicationBuilder(args);
builder.Configuration["Smtp:Host"] = "smtp.example.com";
builder.Configuration["Smtp:Port"] = "587";

builder.Services.AddOptions<SmtpOptions>()
    .BindConfiguration(SmtpOptions.Section)
    .ValidateDataAnnotations()
    .Validate(o => o.Port is 25 or 465 or 587, "Unsupported SMTP port")
    .ValidateOnStart();                                    // fail at startup, not on first use

builder.Services.AddSingleton<IClock, SystemClock>();
builder.Services.AddScoped<IMailer, SmtpMailer>();
builder.Services.AddKeyedSingleton<INotifier, SmsNotifier>("sms");          // keyed services (.NET 8)

using var host = builder.Build();
await host.StartAsync();

using (var scope = host.Services.CreateScope())
{
    var mailer = scope.ServiceProvider.GetRequiredService<IMailer>();
    Console.WriteLine(mailer.Describe());                                    // smtp.example.com:587
}
Console.WriteLine(host.Services.GetRequiredKeyedService<INotifier>("sms").GetType().Name);   // SmsNotifier
await host.StopAsync();

public sealed class SmtpOptions
{
    public const string Section = "Smtp";
    [Required] public string Host { get; set; } = "";
    [Range(1, 65535)] public int Port { get; set; }
}

public interface IClock { DateTimeOffset UtcNow { get; } }
public sealed class SystemClock : IClock { public DateTimeOffset UtcNow => DateTimeOffset.UtcNow; }
public interface IMailer { string Describe(); }
public sealed class SmtpMailer(IOptions<SmtpOptions> options) : IMailer
{
    private readonly SmtpOptions _options = options.Value;
    public string Describe() => $"{_options.Host}:{_options.Port}";
}
public interface INotifier;
public sealed class SmsNotifier : INotifier;
```

Lifetime rules a reviewer must enforce:

- **Captive dependencies**: a singleton must not depend on a scoped service (e.g., `DbContext`). The default host validates scopes only in `Development` (`ValidateScopes`/`ValidateOnBuild`); enable them in tests.
- Never resolve scoped services from the root provider; create a scope (`IServiceScopeFactory.CreateScope()`/`CreateAsyncScope()`) in singletons and background services.
- `IOptions<T>` is a singleton snapshot; `IOptionsSnapshot<T>` is per scope (re-reads per request); `IOptionsMonitor<T>` is for singletons that need reload notifications.
- Service locator (`IServiceProvider` injected into business classes) hides dependencies; inject what you need.
- Transient `IDisposable` services resolved from the root provider are kept alive until the provider is disposed — a memory leak.
- Options classes bound from configuration: use settable properties with defaults plus validation attributes. The **configuration-binding source generator** (on automatically with `PublishAot`, or via `<EnableConfigurationBindingGenerator>true</EnableConfigurationBindingGenerator>`) does not support `required` members: `section.Get<T>()` fails to compile (`CS9035` in the generated code), and in our test `BindConfiguration` left a `required init` property unset so `ValidateOnStart` failed — while the reflection binder handled the same type. Do not mix `required` and configuration binding in code that may be AOT-published.
- `ActivatorUtilities.CreateInstance` and keyed services (`[FromKeyedServices("sms")]`) are fine; avoid registering the same implementation as separate singletons for multiple interfaces (two instances) — register once and forward (`AddSingleton<IFoo>(sp => sp.GetRequiredService<Impl>())`).

##### Async options validation (New in .NET 11)

```csharp
using Microsoft.Extensions.Options;

var builder = Host.CreateApplicationBuilder(args);
builder.Logging.ClearProviders();
builder.Services.AddOptions<BackendOptions>()
    .Configure(o => o.HostName = "evil.example")
    .ValidateOnStart();
builder.Services.AddSingleton<IValidateOptions<BackendOptions>>(new AsyncValidateOptions<BackendOptions>(
    Options.DefaultName,
    async (o, ct) => { await Task.Delay(1, ct); return o.HostName.EndsWith(".internal", StringComparison.Ordinal); },
    "Backend host must be internal"));

using var host = builder.Build();

try
{
    _ = host.Services.GetRequiredService<IOptions<BackendOptions>>().Value;   // synchronous access before startup
}
catch (OptionsValidationException)
{
    Console.WriteLine("sync access before start throws");
}

try
{
    await host.StartAsync();                                                    // runs the async validator
}
catch (OptionsValidationException ex)
{
    Console.WriteLine($"startup failed: {ex.Message}");                         // Backend host must be internal
}

public sealed class BackendOptions { public string HostName { get; set; } = ""; }
```

- `IAsyncValidateOptions<T>` derives from `IValidateOptions<T>` (since RC 1), so async validators are registered in the normal `IValidateOptions<T>` collection; `AsyncValidateOptions<T>` wraps a `Func<T, CancellationToken, Task<bool>>`. Options that have **only** async validators throw `OptionsValidationException` when read synchronously (`IOptions<T>.Value`, `IOptionsMonitor<T>.CurrentValue`, `IOptionsSnapshot<T>`) before host startup has validated them — verified, even when the values are valid. Keep a synchronous validator as well if the options are re-created synchronously (`IOptionsMonitor` reload, `IOptionsSnapshot` in later scopes).[^31]
- The `OptionsBuilder<T>.ValidateAsync(...)` helper shown in the breaking-change article does not exist in the RC 2 reference assemblies (checked by reflection over `Microsoft.Extensions.Options` and all `Microsoft.Extensions.*` assemblies); register `AsyncValidateOptions<T>` or your own `IAsyncValidateOptions<T>` implementation as above.
- `IStartupValidator` is obsolete (SYSLIB0066); implement `IAsyncStartupValidator`.[^20]
- `System.ComponentModel.DataAnnotations` gains `AsyncValidationAttribute`, `IAsyncValidatableObject` and `Validator.ValidateObjectAsync`/`TryValidateObjectAsync`.[^12]

---

#### Logging and Observability

```csharp
var builder = Host.CreateApplicationBuilder(args);
builder.Logging.ClearProviders();
builder.Logging.AddSimpleConsole(o => o.SingleLine = true);
builder.Services.AddSingleton<PaymentProcessor>();
using var host = builder.Build();

host.Services.GetRequiredService<PaymentProcessor>().Charge("order-42", 19.99m);

public sealed partial class PaymentProcessor(ILogger<PaymentProcessor> logger)
{
    public void Charge(string orderId, decimal amount)
    {
        // BAD: interpolation allocates even when the level is disabled and destroys structure
        // logger.LogInformation($"Charging {orderId} {amount}");

        // GOOD: compile-time generated, structured, no boxing when disabled
        LogCharging(orderId, amount);

        using (logger.BeginScope(new Dictionary<string, object> { ["OrderId"] = orderId }))
        {
            LogCharged(amount);
        }
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "Charging order {OrderId} amount {Amount}")]
    private partial void LogCharging(string orderId, decimal amount);

    [LoggerMessage(Level = LogLevel.Debug, Message = "Charged {Amount}")]
    private partial void LogCharged(decimal amount);
}
```

- Message templates must be **constant** (CA2254); placeholders are PascalCase names, not positions. String interpolation in log calls is a finding.
- `[LoggerMessage]` source-generated methods (CA1848) on hot paths; CA1873 (improved in the .NET 11 SDK) flags arguments that are expensive to evaluate when logging is disabled.[^18]
- Never log secrets, tokens, passwords, full connection strings or personal data; use redaction (`Microsoft.Extensions.Compliance.Redaction`) for classified data.
- Log exceptions as the **first argument** (`logger.LogError(ex, "Failed to {Action}", action)`), not `ex.Message` in the template.
- Use `System.Diagnostics.Activity`/`ActivitySource` for tracing and `System.Diagnostics.Metrics.Meter` for metrics, exported through OpenTelemetry. **New in .NET 11**: `MemoryCache` emits built-in metrics; ASP.NET Core hosting emits OpenTelemetry HTTP semantic-convention tags by default; `HttpClient` high-cardinality metrics use observable counters; `Microsoft.Extensions.Diagnostics` can configure `Activity` tracing declaratively with rules.[^12][^15][^11]

---

#### HttpClient

```csharp
using System.IO.Compression;
using System.Net.Http.Json;

var builder = Host.CreateApplicationBuilder(args);
builder.Logging.ClearProviders();
builder.Services.AddHttpClient<GitHubClient>(client =>
{
    client.BaseAddress = new Uri("https://api.github.com/");   // trailing slash matters for relative URIs
    client.Timeout = TimeSpan.FromSeconds(10);
    client.DefaultRequestHeaders.UserAgent.ParseAdd("contoso-app/1.0");
});
using var host = builder.Build();
var github = host.Services.GetRequiredService<GitHubClient>();
Console.WriteLine(github.BaseAddress);                          // https://api.github.com/

// New in .NET 11: compress a request body (sets Content-Encoding: gzip)
using var body = new GZipCompressedContent(JsonContent.Create(new { name = "x" }), CompressionLevel.Fastest);
Console.WriteLine(string.Join(",", body.Headers.ContentEncoding));   // gzip

public sealed class GitHubClient(HttpClient http)
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

public sealed record Repo(string Name, int StargazersCount);
```

- **Never `new HttpClient()` per request** (socket exhaustion) and never a single static `HttpClient` without `SocketsHttpHandler.PooledConnectionLifetime` (stale DNS). Use `IHttpClientFactory` (typed or named clients) or one long-lived client with `new SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(2) }`.
- Typed clients are **transient**: do not capture them in singletons (the handler rotation is lost); inject `IHttpClientFactory` into singletons instead.
- Dispose `HttpResponseMessage` (`using var response`), especially with `HttpCompletionOption.ResponseHeadersRead`.
- Escape user input in URLs (`Uri.EscapeDataString`); a `BaseAddress` without a trailing slash drops its last path segment when combined with a relative URI.
- `HttpClient.Timeout` covers the whole request; per-call deadlines use a `CancellationToken`. A timeout surfaces as `TaskCanceledException` with an inner `TimeoutException`.
- Resilience: `Microsoft.Extensions.Http.Resilience` (`AddStandardResilienceHandler()`) gives retries with jitter, circuit breaker and timeouts. Retries must be limited to idempotent requests.
- `EnsureSuccessStatusCode()` throws `HttpRequestException` with `StatusCode` set; check status codes explicitly where 404/409 are expected outcomes.
- **New in .NET 11**: `GZipCompressedContent`, `BrotliCompressedContent`, `ZstandardCompressedContent` (request-body compression), `SocketsHttpHandler.InitialHttp2MaxConcurrentStreams`, typed DNS queries via `Dns.ResolveSrv/Mx/Txt/...` and `DnsResolver`. `SocketsHttpHandler.ShouldEvictConnection` is **experimental** (verified: using it is error `SYSLIB5008` "for evaluation purposes only") — flag it in production code like any other `SYSLIB5xxx` API. Only compress request bodies for servers known to accept `Content-Encoding` on requests.[^12]
- **New in .NET 11**: `SslStream` no longer downloads intermediate certificates (AIA) on the **server** side by default; Linux client-side AIA fetching is limited to two fetches per chain build.[^11]

---

#### System.Text.Json

##### Source generation and safe defaults

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

var order = new OrderDto(42, "Ada", [new LineDto("Pen", 2)]);
string json = JsonSerializer.Serialize(order, AppJson.Default.OrderDto);
Console.WriteLine(json);   // {"id":42,"customer":"Ada","lines":[{"item":"Pen","quantity":2}]}

var back = JsonSerializer.Deserialize(json, AppJson.Default.OrderDto);
Console.WriteLine(back?.Lines.Count);                                    // 1

// .NET 10: Strict preset — disallows unmapped members and duplicate properties, respects nullability and required
try
{
    JsonSerializer.Deserialize<LineDto>("""{"Item":"Pen","Quantity":1,"Extra":true}""", JsonSerializerOptions.Strict);
}
catch (JsonException ex)
{
    Console.WriteLine(ex.Message.Split('.')[0]);   // The JSON property 'Extra' could not be mapped to any ...
}

// New in .NET 11: PascalCase naming policy
Console.WriteLine(JsonSerializer.Serialize(new { orderId = 1 },
    new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.PascalCase }));   // {"OrderId":1}

public sealed record OrderDto(int Id, string Customer, List<LineDto> Lines);
public sealed record LineDto(string Item, int Quantity);

[JsonSourceGenerationOptions(JsonSerializerDefaults.Web)]
[JsonSerializable(typeof(OrderDto))]
internal sealed partial class AppJson : JsonSerializerContext;
```

Review rules:

- Reuse `JsonSerializerOptions` instances (cache them in a `static readonly` field); creating options per call discards the metadata cache (CA1869).
- Use **source generation** (`JsonSerializerContext`) for AOT/trimmed apps and hot paths; reflection-based `JsonSerializer.Serialize<T>(value)` raises IL2026/IL3050 under `IsAotCompatible`.
- `JsonSerializerDefaults.Web` = camelCase + case-insensitive reads + numbers from strings. ASP.NET Core uses it by default.
- Untrusted input: keep `MaxDepth` (default 64), consider `JsonSerializerOptions.Strict` or `UnmappedMemberHandling.Disallow`, `AllowDuplicateProperties = false` (.NET 10) and `RespectNullableAnnotations = true` (.NET 9). Never deserialize to `object`/`dynamic` with type-name handling (that is the Newtonsoft `TypeNameHandling.All` RCE class of bug; `System.Text.Json` has no equivalent, keep it that way).
- Polymorphism needs `[JsonPolymorphic]` + `[JsonDerivedType(typeof(Sub), "discriminator")]`; without them, a derived instance serialized through a base-typed property loses its derived members and deserialization produces the base type — silently.
- `JsonDocument` is `IDisposable` (pooled buffers): `using var doc = JsonDocument.Parse(...)`. `JsonNode` is not pooled.

##### Unions, closed hierarchies and JSON (New in .NET 11)

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

Reading r1 = 5;
Reading r2 = "offline";
Console.WriteLine(JsonSerializer.Serialize(r1) + " " + JsonSerializer.Serialize(r2));   // 5 "offline"
Console.WriteLine(JsonSerializer.Deserialize<Reading>("\"x\"").Value);                // x — token kind picks the case

// A closed hierarchy with discriminators round-trips object-shaped cases safely
Shape shape = new Circle(2);
string json = JsonSerializer.Serialize(shape);
Console.WriteLine(json);                                                              // {"kind":"circle","Radius":2}
Console.WriteLine(JsonSerializer.Deserialize<Shape>(json));                           // Circle { Radius = 2 }

public union Reading(int, string);

[JsonPolymorphic(TypeDiscriminatorPropertyName = "kind")]
[JsonDerivedType(typeof(Circle), "circle")]
[JsonDerivedType(typeof(Square), "square")]
public closed record Shape;
public sealed record Circle(double Radius) : Shape;
public sealed record Square(double Side) : Shape;
```

- A union is serialized as its current case value; nothing records which case it was. Deserialization works only when the JSON token kind (number vs string vs object vs array) selects exactly one case; otherwise it throws `JsonException` ("ambiguous for union type ... Specify a custom type classifier"). For object-shaped alternatives use a closed hierarchy with `[JsonPolymorphic]`, or `[JsonUnion(TypeClassifier = typeof(JsonUnionTypeStructuralClassifier))]` (built in; picks the case whose properties match — verified with `union Pet(Cat, Dog)` where `Cat(Name, Lives)` and `Dog(Name, Age)` round-trip) or a custom `JsonTypeClassifierFactory`. Structural classification breaks silently when two cases have the same property names; prefer an explicit discriminator for public contracts.
- **New in .NET 11**: `[JsonPolymorphic(InferClosedTypePolymorphism = true)]` on a `closed` base derives the `[JsonDerivedType]` list from the closed hierarchy (verified: `new Circle(2)` serializes as `{"$type":"Circle","Radius":2}` and round-trips). The discriminator defaults to the type name, so renaming a subclass changes the wire format — use explicit `[JsonDerivedType(typeof(Circle), "circle")]` for stable public contracts.[^12]
- A `default` union serializes as `null` (verified).
- **New in .NET 11**: `JsonSerializer.SerializeAsyncEnumerable(..., topLevelValues: true)` writes JSON Lines; `[JsonNamingPolicy]` attribute on types/members; type-level `[JsonIgnore(Condition = ...)]` defaults; built-in converters for `Decimal32`/`Decimal64`/`Decimal128`.[^12]

---

#### ASP.NET Core Minimal APIs

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.AspNetCore.Http.HttpResults;

var builder = WebApplication.CreateSlimBuilder(args);
builder.WebHost.UseUrls("http://127.0.0.1:0");
builder.Logging.ClearProviders();
builder.Services.AddValidation();                 // DataAnnotations validation for minimal APIs (.NET 10)
builder.Services.AddProblemDetails();             // RFC 9457 problem responses
builder.Services.AddSingleton<TodoStore>();

var app = builder.Build();
app.UseExceptionHandler();                        // no stack traces to clients
app.UseStatusCodePages();

var todos = app.MapGroup("/todos");

todos.MapGet("/{id:int}", Results<Ok<Todo>, NotFound> (int id, TodoStore store) =>
    store.Find(id) is { } todo ? TypedResults.Ok(todo) : TypedResults.NotFound());

todos.MapPost("/", Created<Todo> (CreateTodo request, TodoStore store) =>
{
    var todo = store.Add(request.Title);
    return TypedResults.Created($"/todos/{todo.Id}", todo);
});

app.MapGet("/health", [ShortCircuit] () => "Healthy");   // New in .NET 11: skips the rest of the middleware pipeline

await app.StartAsync();
using var http = new HttpClient { BaseAddress = new Uri(app.Urls.First()) };
Console.WriteLine(await http.GetStringAsync("/health"));                                     // Healthy
Console.WriteLine((int)(await http.PostAsJsonAsync("/todos/", new CreateTodo("Write guide"))).StatusCode);  // 201
Console.WriteLine((int)(await http.PostAsJsonAsync("/todos/", new CreateTodo(""))).StatusCode);             // 400
Console.WriteLine(await http.GetStringAsync("/todos/1"));                                     // {"id":1,"title":"Write guide"}
Console.WriteLine((int)(await http.GetAsync("/todos/99")).StatusCode);                        // 404
await app.StopAsync();

public sealed record CreateTodo([property: Required, StringLength(200, MinimumLength = 1)] string Title);
public sealed record Todo(int Id, string Title);

public sealed class TodoStore
{
    private readonly Lock _gate = new();
    private readonly Dictionary<int, Todo> _items = [];
    private int _nextId;

    public Todo Add(string title)
    {
        lock (_gate)
        {
            var todo = new Todo(++_nextId, title);
            _items[todo.Id] = todo;
            return todo;
        }
    }

    public Todo? Find(int id)
    {
        lock (_gate) { return _items.GetValueOrDefault(id); }
    }
}
```

Review rules:

- Return `TypedResults` with `Results<T1, T2>` union return types so OpenAPI metadata is accurate and tests can assert result types.
- Validate input: `AddValidation()` (.NET 10) validates DataAnnotations on minimal API parameters and returns a 400 problem response; without it, attributes on parameters are ignored. **New in .NET 11**: async validation attributes run here too, and `ValidatableTypeAttribute`/`SkipValidationAttribute` are no longer experimental.[^14]
- **New in .NET 11** (documented): endpoint filters run **even when parameter binding fails**, with `HttpContext.Response.StatusCode` already 400, so filters can replace the response; in `Development`, set `RouteHandlerOptions.ThrowOnBadRequest = false` to observe this. Our RC 2 tests could not isolate the boundary: for a malformed query-string or missing header value the filter ran with status 400 on both 10.0.13 and 11 RC 2 (Production environment), and for a malformed JSON body it ran on neither. Review rule: filters must not assume their arguments were bound — check `context.HttpContext.Response.StatusCode` before reading `context.Arguments`.[^14]
- Bind `CancellationToken` parameters (request-aborted token) and pass them down.
- Never return EF entities directly (over-posting and lazy-loading surprises): map to DTOs.
- Authorization: `RequireAuthorization()` on groups, fallback policy (`AddAuthorizationBuilder().SetFallbackPolicy(...)`) for secure-by-default; `[AllowAnonymous]` must be deliberate. Anti-forgery for form posts (`app.UseAntiforgery()`; form-bound minimal endpoints validate tokens automatically).
- Rate limiting: `AddRateLimiter` + `UseRateLimiter` + `RequireRateLimiting(...)`. **Removed in .NET 11**: the `ConcurrencyLimiter` middleware — use a concurrency limiter policy in the rate limiting middleware.[^15]
- `UseExceptionHandler` + `AddProblemDetails` in production; `app.UseDeveloperExceptionPage()` only in Development.
- Kestrel in .NET 11 enforces HTTP protocol compliance more strictly (malformed requests that used to be tolerated are rejected).[^15]
- **New in .NET 11**: OpenAPI documents default to **OpenAPI 3.2** (Microsoft.OpenApi 3.x) — downstream tooling that only understands 3.0/3.1 needs `options.OpenApiVersion = OpenApiSpecVersion.OpenApi3_1` or similar.[^15]
- Union types are accepted and returned as JSON bodies (not from route/query/header/form) — see System.Text.Json for the ambiguity rule.

---

#### Hosting and Background Services

```csharp
var builder = Host.CreateApplicationBuilder(args);
builder.Logging.ClearProviders();
builder.Services.AddHostedService<QueueWorker>();
using var host = builder.Build();

try
{
    await host.RunAsync();
    Console.WriteLine("completed");                                       // .NET 10 printed this
}
catch (InvalidOperationException ex)
{
    Console.WriteLine($"host failed: {ex.Message}");                      // .NET 11: host failed: queue broken
}

public sealed class QueueWorker(ILogger<QueueWorker> logger, IServiceScopeFactory scopes) : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        await Task.Delay(20, stoppingToken);
        await using var scope = scopes.CreateAsyncScope();                // scoped services (DbContext) per unit of work
        logger.LogInformation("processing batch");
        throw new InvalidOperationException("queue broken");
    }
}
```

- **New in .NET 11**: when a `BackgroundService` throws and `HostOptions.BackgroundServiceExceptionBehavior` is `StopHost` (the default), `RunAsync`, `StopAsync`, `WaitForShutdownAsync` and their sync versions **throw** the service's exception (an `AggregateException` if several failed) instead of completing — the process now exits non-zero (verified: .NET 10 completed normally, .NET 11 threw). Top-level `try`/`catch` around `RunAsync` that logged "graceful shutdown" now sees the failure.[^32]
- `ExecuteAsync` must observe `stoppingToken`; a loop without cancellation blocks shutdown until `HostOptions.ShutdownTimeout` (30 s default).
- Since .NET 10, all of `ExecuteAsync` runs as a background task (code before the first `await` no longer blocks startup).
- Catch and log per-item failures inside the loop if one bad message must not stop the host; let fatal errors escape.
- Resolve scoped services through `IServiceScopeFactory`, one scope per unit of work.

---

#### EF Core 11

EF Core 11 runs only on .NET 11 (not on .NET 10 or .NET Framework).[^16]

```csharp
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

await using var connection = new SqliteConnection("Data Source=:memory:");
await connection.OpenAsync();
var options = new DbContextOptionsBuilder<ShopDb>().UseSqlite(connection).Options;

await using (var db = new ShopDb(options))
{
    await db.Database.EnsureCreatedAsync();
    db.Customers.Add(new Customer { Name = "Ada", Orders = [new Order { Total = 10m }, new Order { Total = 25m }] });
    db.Customers.Add(new Customer { Name = "Bob", Orders = [new Order { Total = 5m }] });
    await db.SaveChangesAsync();
}

await using (var db = new ShopDb(options))
{
    // GOOD: projection + AsNoTracking for read-only queries; one SQL statement, no N+1
    var summaries = await db.Customers.AsNoTracking()
        .OrderBy(c => c.Name)
        .Select(c => new { c.Name, Count = c.Orders.Count, Sum = c.Orders.Sum(o => o.Total) })
        .ToListAsync();
    foreach (var s in summaries) Console.WriteLine($"{s.Name}: {s.Count} orders, {s.Sum}");

    // GOOD: FromSql with an interpolated string is parameterized
    var name = "Ada'; DROP TABLE Customers; --";
    Console.WriteLine(await db.Customers.FromSql($"SELECT * FROM Customers WHERE Name = {name}").CountAsync());   // 0

    // Set-based update without loading entities (EF Core 7+)
    var updated = await db.Orders.Where(o => o.Total < 10m).ExecuteUpdateAsync(s => s.SetProperty(o => o.Total, 10m));
    Console.WriteLine($"updated {updated}");

    // MaxByAsync / MinByAsync are translated in EF Core 11
    var biggest = await db.Orders.MaxByAsync(o => o.Total);
    Console.WriteLine(biggest?.Total);                                            // 25.0
}

public sealed class ShopDb(DbContextOptions<ShopDb> options) : DbContext(options)
{
    public DbSet<Customer> Customers => Set<Customer>();
    public DbSet<Order> Orders => Set<Order>();
}

public sealed class Customer
{
    public int Id { get; set; }
    public required string Name { get; set; }
    public List<Order> Orders { get; set; } = [];
}

public sealed class Order
{
    public int Id { get; set; }
    public decimal Total { get; set; }
    public int CustomerId { get; set; }
}
```

Pitfalls a reviewer must catch:

- **SQL injection**: `FromSqlRaw`/`ExecuteSqlRaw` with string concatenation or `$"..."` passed as a *string* is injectable. `FromSql($"...")`/`ExecuteSql($"...")` (taking `FormattableString`) parameterize. Never build SQL from `string.Format`.
- **N+1 queries**: iterating entities and touching navigation properties (lazy loading or per-item queries). Use projections, `Include`, or split queries.
- **Client evaluation**: methods EF cannot translate in `Where`/`OrderBy` throw `InvalidOperationException` at run time; review custom methods inside query expressions.
- **Tracking**: read-only queries should use `AsNoTracking()`/`AsNoTrackingWithIdentityResolution()`; long-lived contexts accumulate tracked entities (memory, slow `SaveChanges`).
- **`DbContext` is not thread-safe**: no parallel queries on one context (`Task.WhenAll` over the same context throws); it is scoped per request. Use `IDbContextFactory<T>` in singletons/background services.
- **Sync over async**: `ToList()`/`SaveChanges()` in async code paths block threads. (The Cosmos provider now always throws on sync I/O.)
- Unbounded queries: always paginate (`Skip`/`Take` with a stable `OrderBy`), prefer keyset pagination for large tables.
- `ExecuteUpdate`/`ExecuteDelete` bypass the change tracker, interceptors that hook `SaveChanges`, and concurrency tokens unless included in the predicate.
- Migrations: do not call `Database.Migrate()` from multiple app instances concurrently at startup; prefer migration bundles or scripts in deployment. **New in EF Core 11**: `Migrate()`/`MigrateAsync()` **throw** when the assembly contains no migrations (suppress with `ConfigureWarnings(w => w.Ignore(RelationalEventId.MigrationsNotFound))` if intentional).[^17]
- **New in EF Core 11**: `UseSqlServer` defaults to compatibility level **160** (SQL Server 2022: `LEAST`/`GREATEST` in generated SQL) — configure `UseCompatibilityLevel(150)` for SQL Server 2019 or Azure SQL databases at an older level; split queries throw `DbQueryConcurrencyException` when concurrent modifications make results uncorrelatable (they used to silently return empty collections) — retry or use a serializable/snapshot transaction; `SqlVector<T>` properties are no longer loaded by default; `Microsoft.Data.SqlClient` 7.0 needs `Microsoft.Data.SqlClient.Extensions.Azure` for Entra ID authentication; owned JSON collections without keys are obsolete — migrate to complex types (`ComplexCollection(...).ToJson()`); `Property(...)` no longer configures primitive collections (use `PrimitiveCollection`).[^17]
- **New in EF Core 11**: `FullJoin` translates to `FULL JOIN`; complex types and JSON columns work with TPT/TPC; keys and indexes on complex-type properties; `EF.Functions.JsonPathExists`/`JsonContains` (SQL Server); `dotnet ef database update --add` creates and applies a migration in one step (review: generated migrations still need code review).[^16]
- `Microsoft.Data.Sqlite` 11 targets `net10.0` only (no .NET Framework).[^17]

---

#### Security

```csharp
using System.Security.Cryptography;
using System.Text;

// Random tokens: CSPRNG only
string token = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
Console.WriteLine(token.Length);                                              // 44

// Password hashing: PBKDF2 with a per-user salt and high iteration count (or ASP.NET Core Identity's PasswordHasher)
byte[] salt = RandomNumberGenerator.GetBytes(16);
byte[] hash = Rfc2898DeriveBytes.Pbkdf2("correct horse"u8, salt, 600_000, HashAlgorithmName.SHA256, 32);
byte[] attempt = Rfc2898DeriveBytes.Pbkdf2("correct horse"u8, salt, 600_000, HashAlgorithmName.SHA256, 32);
Console.WriteLine(CryptographicOperations.FixedTimeEquals(hash, attempt));    // True — constant-time compare

// Authenticated encryption
byte[] key = RandomNumberGenerator.GetBytes(32);
using var aes = new AesGcm(key, tagSizeInBytes: 16);
byte[] nonce = RandomNumberGenerator.GetBytes(AesGcm.NonceByteSizes.MaxSize);  // never reuse a nonce with the same key
byte[] plaintext = Encoding.UTF8.GetBytes("secret");
byte[] ciphertext = new byte[plaintext.Length];
byte[] tag = new byte[16];
aes.Encrypt(nonce, plaintext, ciphertext, tag);
byte[] decrypted = new byte[ciphertext.Length];
aes.Decrypt(nonce, ciphertext, tag, decrypted);
Console.WriteLine(Encoding.UTF8.GetString(decrypted));                        // secret

// New in .NET 11: X25519 key agreement
using X25519DiffieHellman alice = X25519DiffieHellman.GenerateKey();
using X25519DiffieHellman bob = X25519DiffieHellman.GenerateKey();
Console.WriteLine(alice.DeriveRawSecretAgreement(bob).AsSpan().SequenceEqual(bob.DeriveRawSecretAgreement(alice)));   // True

// Path traversal: resolve and check containment
string root = Path.GetFullPath("uploads") + Path.DirectorySeparatorChar;
string requested = Path.GetFullPath(Path.Combine(root, "../etc/passwd"));
Console.WriteLine(requested.StartsWith(root, StringComparison.Ordinal));      // False — reject
```

Review checklist:

- **Randomness**: `Random`/`Random.Shared`/`Guid.NewGuid()` are not for secrets; use `RandomNumberGenerator`.
- **Comparisons of secrets** (HMACs, tokens): `CryptographicOperations.FixedTimeEquals`, never `SequenceEqual`/`==`.
- **Algorithms**: no MD5/SHA-1 for security, no DES/3DES/RC2, no ECB, no `RSAEncryptionPadding.Pkcs1` for new designs; AES-GCM or ChaCha20-Poly1305 for encryption; SHA-256+ for hashing; `HMACSHA256.HashData` for MACs. **New in .NET 11**: `RSACryptoServiceProvider.Encrypt/Decrypt(byte[], bool)` are obsolete (SYSLIB0064); experimental composite ML-KEM and HPKE APIs (SYSLIB5006/SYSLIB5009) are not for production yet.[^20][^12]
- **Passwords**: PBKDF2 via `Rfc2898DeriveBytes.Pbkdf2` with ≥ 600,000 SHA-256 iterations (OWASP guidance), or ASP.NET Core Identity's `PasswordHasher<T>`; all `Rfc2898DeriveBytes` constructors are obsolete (SYSLIB0060).
- **Deserialization**: `BinaryFormatter` is gone (throws `PlatformNotSupportedException` on .NET 9+); do not reintroduce it through compatibility packages. No `TypeNameHandling` other than `None` in Newtonsoft.Json; no `XmlSerializer` on types chosen by input.
- **XML**: `XmlReader` with `DtdProcessing.Prohibit` (default) — never `DtdProcessing.Parse` on untrusted XML (XXE).
- **Injection**: parameterized SQL only; `Process.Start` with `ArgumentList` (not a concatenated `Arguments` string) and `UseShellExecute = false`. **New in .NET 11**: `Process.RunAndCaptureTextAsync(fileName, arguments)` takes the arguments as a collection — keep it that way.
- **Paths**: `Path.Combine` with an absolute second argument discards the first; resolve with `Path.GetFullPath` and check containment with an ordinal `StartsWith` on a root that ends with a separator.
- **Secrets**: no secrets in `appsettings.json` or source; use user-secrets in development and a vault/environment in production. Connection strings in logs are a leak.
- **ASP.NET Core**: HTTPS redirection + HSTS, `UseForwardedHeaders` only with `KnownProxies`/`KnownNetworks` configured, CORS with explicit origins (never `AllowAnyOrigin` with credentials), cookie auth with `SameSite`/`Secure`/`HttpOnly`, antiforgery for forms, output encoding (Razor encodes by default; `Html.Raw`/`MarkupString` on user data is XSS).
- **TLS**: never set `ServerCertificateCustomValidationCallback` to accept all certificates outside tests.
- **Regex/JSON/XML DoS**: timeouts and limits on untrusted input (`Regex` with `matchTimeout`, JSON `MaxDepth`, request body size limits).
- **New in .NET 11**: `NamedPipeServerStream` with `PipeOptions.CurrentUserOnly` tightens Unix socket permissions; named mutexes are user-scoped on Unix; TAR readers verify header checksums; ZIP reading validates CRC32.[^11]

---

#### Performance

- **Measure first** with BenchmarkDotNet (microbenchmarks), `dotnet-counters`, `dotnet-trace` and production metrics. Do not accept "optimizations" without numbers.
- Allocation hot spots: LINQ in tight loops, closures capturing variables, boxing (value types in `object` APIs, **value-type cases in generated unions**), `params object[]` logging, string concatenation in loops, `ToList()` on already materialized data.
- Prefer `Span<T>`, `stackalloc` (bounded), `ArrayPool<T>`, `string.Create`, `SearchValues<T>` (.NET 8), `CompositeFormat` (.NET 8) for repeated formats, `FrozenDictionary` for static lookups.
- Async: `ValueTask` only when the method usually completes synchronously and is hot; otherwise `Task`. **New in .NET 11**: runtime async (opt-in), no `ExecutionContext` capture when unused, JIT intrinsics for `Task.FromResult`/`ValueTask.FromResult`.[^9]
- **New in .NET 11 JIT**: more bounds-check elimination (`i + const < len`, `span[^1]`, first element after `!span.IsEmpty`), constant-folded `SequenceEqual`/`string.Equals` on constants, devirtualization of generic virtual methods, branchless folding of `x is 0 or 1 or 2`. Do not hand-optimize code into unreadable forms for things the JIT now does.[^9]
- **New in .NET 11 hardware baseline**: x86-64-v2 minimum; ReadyToRun images target x86-64-v3 on Windows/Linux (code may be re-jitted on older v2 CPUs, costing startup).[^10]
- `ReadyToRun` now specializes `EqualityComparer<T>.Default`/`Comparer<T>.Default`.[^9]
- Server GC is the default for ASP.NET Core; containers respect cgroup limits. `DOTNET_GCHeapHardLimit` and `GCHeapAffinitizeMask` only with measurements.
- EF Core: projections, `AsNoTracking`, compiled queries for hot paths, batching, avoid N+1.
- `HttpClient` connection reuse; `System.Text.Json` source generation; response compression only for compressible payloads.

---

#### Testing

```csharp
using Microsoft.Extensions.Time.Testing;

public class TokenCacheTests
{
    [Fact]
    public void Entry_expires_after_ttl()
    {
        var time = new FakeTimeProvider(new DateTimeOffset(2026, 10, 22, 12, 0, 0, TimeSpan.Zero));
        var cache = new TokenCache(time, TimeSpan.FromMinutes(5));
        cache.Set("abc");

        time.Advance(TimeSpan.FromMinutes(4));
        Assert.True(cache.TryGet(out _));

        time.Advance(TimeSpan.FromMinutes(2));
        Assert.False(cache.TryGet(out _));
    }

    [Theory]
    [InlineData(-1, "negative")]
    [InlineData(0, "zero")]
    [InlineData(7, "positive")]
    public void Classify(int value, string expected) =>
        Assert.Equal(expected, value switch { < 0 => "negative", 0 => "zero", _ => "positive" });

    [Fact]
    public async Task Delay_uses_injected_time_provider()
    {
        var time = new FakeTimeProvider();
        Task delay = Task.Delay(TimeSpan.FromSeconds(30), time, TestContext.Current.CancellationToken);
        Assert.False(delay.IsCompleted);
        time.Advance(TimeSpan.FromSeconds(30));
        await delay;
    }

    [Fact]
    public void Default_union_is_empty()
    {
        Shape shape = default;
        Assert.Null(shape.Value);
        Assert.Throws<System.Runtime.CompilerServices.SwitchExpressionException>(() => Area(shape));
    }

    private static double Area(Shape s) => s switch { Circle c => Math.PI * c.R * c.R, Square q => q.Side * q.Side };
}

public sealed class TokenCache(TimeProvider time, TimeSpan ttl)
{
    private (string Token, DateTimeOffset Expires)? _entry;
    public void Set(string token) => _entry = (token, time.GetUtcNow() + ttl);
    public bool TryGet(out string? token)
    {
        if (_entry is { } e && time.GetUtcNow() < e.Expires) { token = e.Token; return true; }
        token = null;
        return false;
    }
}

public sealed record Circle(double R);
public sealed record Square(double Side);
public union Shape(Circle, Square);
```

(Verified with `dotnet new xunit --xunit-version v3` on the RC 2 SDK — package `xunit.v3.mtp-v2` 3.2.2 — plus `Microsoft.Extensions.TimeProvider.Testing` 10.11.0: 6 tests passed under Microsoft.Testing.Platform.)

- Name tests `Method_Scenario_Expectation` or as sentences; one behaviour per test; Arrange/Act/Assert.
- **Time**: inject `TimeProvider` and use `FakeTimeProvider`; tests that `Thread.Sleep`/`Task.Delay` real time are slow and flaky.
- **xUnit v3**: `TestContext.Current.CancellationToken` for cancellation-aware tests (analyzer xUnit1051 asks for it); tests are an executable (`OutputType` `Exe`). The RC 2 `dotnet new xunit` template still defaults to xUnit **v2** (described as "deprecated, kept for backwards compatibility"); pass `--xunit-version v3` for new projects. With Microsoft.Testing.Platform, `global.json` needs `"test": { "runner": "Microsoft.Testing.Platform" }` for `dotnet test`.[^18]
- Integration tests: `WebApplicationFactory<Program>` (`Microsoft.AspNetCore.Mvc.Testing`) with real middleware; replace external dependencies in `ConfigureTestServices`. Use real databases (Testcontainers) rather than EF Core InMemory for query behaviour — the InMemory provider does not translate SQL and hides translation errors.
- Mocks: mock interfaces you own; do not mock `DbContext`, `HttpClient` (use a fake `HttpMessageHandler`) or `ILogger` extension methods (they are static; verify via a test logger such as `FakeLogger` from `Microsoft.Extensions.Diagnostics.Testing`).
- Assert exceptions with `Assert.Throws`/`ThrowsAsync` — never `try { ...; Assert.Fail(); } catch {}`.
- Union and closed-hierarchy code: add a test that constructs every case (and `default`) — a new case without a test is how `SwitchExpressionException` reaches production through a library boundary.
- **New in .NET 11 SDK**: `dotnet test` gains `--timeout`, `--maximum-failed-tests`, `--no-dependencies`; MSTest/NUnit templates support `--test-runner`.[^18]

---

#### Analyzers and Build Hygiene

- `<AnalysisLevel>latest-recommended</AnalysisLevel>` (or `latest-all` with selective suppressions), `<TreatWarningsAsErrors>true</TreatWarningsAsErrors>`, `<EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>` with an `.editorconfig`.
- Do not suppress with `#pragma warning disable` without a comment; never suppress `CS8509`/`CS8655` (exhaustiveness), `CA2000` (disposal), `CA2100`/`CA3001` (SQL injection), `CA5350`/`CA5351` (weak crypto), `CA2016` (forward `CancellationToken`), `CA2254` (constant log templates), `CS8600`–`CS8625` (nullability) wholesale.
- New/changed in the .NET 11 SDK: **IDE0410** (use labeled jump statements), reduced-noise **CA1873** (expensive logging arguments; now only Information level and below by default, with the reason in the message). `AnalysisLevel=latest` previously applied the .NET 9 rule set by mistake; the .NET 11 SDK applies the .NET 11 rules, so upgrading can surface new analyzer warnings. Verified: with `EnforceCodeStyleInBuild` and `dotnet_diagnostic.IDE0410.severity = warning`, the flag-variable nested-loop pattern reports IDE0410.[^25]
- Obsoletion warnings with custom IDs (`SYSLIBxxxx`) cannot be suppressed with `CS0618`; suppress the specific ID only with a tracked migration plan.
- Trimming/AOT analyzers: `IsAotCompatible`/`IsTrimmable` for libraries; IL2026/IL2057/IL3050 are real runtime failures in published AOT apps (see next section).
- Package hygiene: `dotnet list package --vulnerable --include-transitive` (NuGet audit warnings NU1901–NU1904 are on by default), lock files, `--min-publish-age-hours` source policy (.NET 11 SDK).

---

#### Trimming and Native AOT

```csharp
using System.Diagnostics.CodeAnalysis;
using System.Text.Json;
using System.Text.Json.Serialization;

// BAD under <IsAotCompatible>true</IsAotCompatible>: IL2026 + IL3050 (reflection-based serializer)
// var json = JsonSerializer.Serialize(new Point(1, 2));

// GOOD: source-generated metadata
var json = JsonSerializer.Serialize(new Point(1, 2), PointJson.Default.Point);
Console.WriteLine(json);                                   // {"X":1,"Y":2}

Console.WriteLine(Create<Widget>());                       // Widget
Console.WriteLine(CreateByName(typeof(Widget)));           // Widget

// Annotate reflection so the trimmer keeps what you need
static T Create<[DynamicallyAccessedMembers(DynamicallyAccessedMemberTypes.PublicParameterlessConstructor)] T>()
    where T : new() => new();

static object? CreateByName(
    [DynamicallyAccessedMembers(DynamicallyAccessedMemberTypes.PublicParameterlessConstructor)] Type type) =>
    Activator.CreateInstance(type);

public sealed record Point(int X, int Y);
public sealed class Widget { public override string ToString() => "Widget"; }

[JsonSerializable(typeof(Point))]
internal sealed partial class PointJson : JsonSerializerContext;
```

Verified analyzer output for a `net11.0` project with `<IsAotCompatible>true</IsAotCompatible>`: `JsonSerializer.Serialize<T>(value)` → **IL2026** and **IL3050**; `Type.GetType(userInput)` → **IL2057**.

- Native AOT: no runtime code generation (`Reflection.Emit`, `Expression.Compile` falls back to interpretation), no dynamic assembly loading, limited reflection. Use source generators: System.Text.Json, `[LoggerMessage]`, `[GeneratedRegex]`, `[LibraryImport]`, configuration binding (`EnableConfigurationBindingGenerator`, see the `required` limitation in DI and Options), the minimal API Request Delegate Generator (enabled by `PublishAot`; pair it with `WebApplication.CreateSlimBuilder`).
- EF Core with AOT needs precompiled queries/compiled models: in EF Core 11 these are produced automatically on publish when `PublishAot` is `true`; the `EFOptimizeContext` property was removed.[^17]
- **New in .NET 11**: NativeAOT on Unix names native library outputs with a `lib` prefix; interface dispatch uses a shared helper (smaller, faster); `Assembly.GetCallingAssembly()` works when stack-trace data is available, and behaves differently when `StackTraceSupport` is disabled.[^11][^9]
- Treat every IL2xxx/IL3xxx warning as a bug in AOT-published apps; `[UnconditionalSuppressMessage]` requires a justification that proves safety.

---

#### Removed / Changed in .NET 11 — Migration Notes

When a project moves from `net10.0`/C# 14 to `net11.0`/C# 15, check these first. Items marked (verified) were reproduced on 10.0.13 vs 11 RC 2.

##### Language / compiler

| Change | Symptom | Fix |
|---|---|---|
| `union`, `closed` become contextual keywords | `union Foo;` → `CS9370`; `closed field;` → `CS1519`; types named `closed` → `CS9380` (verified) | Escape: `@union`, `@closed` |
| `with(...)` in a collection expression means constructor arguments | `[with(x, y), z]` no longer calls a method named `with` (`CS9355` for arrays) (verified, already with the 10.0.4xx compiler and also under `LangVersion` 14) | `@with(x, y)` |
| Collection expressions of span type have *declaration-block* safe-context | New ref-safety errors when a `Span<T> s = [..]` declared in a loop is assigned to an outer `scoped` span | Use an array or hoist the declaration |
| `ref readonly`-returning synthesized delegates/local functions need `InAttribute` | `CS0518` on unusual target frameworks | Reference an assembly that defines `System.Runtime.InteropServices.InAttribute` |
| `&&`/`||` with an interface-typed left operand and a `dynamic` right operand | `CS7083` at compile time (was a run-time `RuntimeBinderException`) | Cast the left operand to a class or `dynamic` |
| `nameof(this.P)` in attributes | Error (was accidentally allowed since C# 12) | `nameof(P)` |
| `(X.Y) when` in a switch arm | Now a constant pattern plus `when` clause (was a cast of `when`) | Review switch arms with parenthesized constants |

Sources: Roslyn "breaking changes since C# 14".[^8]

##### Runtime and libraries

| Change | Effect | Action |
|---|---|---|
| Hardware baseline x86-64-v2 / Arm64 LSE on Windows | Process fails to start on old CPUs | Check fleet hardware and CI runners |
| Unchecked float → `sbyte`/`byte`/`short`/`ushort`/`char` saturate (verified) | `(byte)298.0` was 42, now 255 | Use `checked`, or `ConvertToIntegerNative` to opt into platform behaviour |
| `Math.Round(double, digits)` correctly rounded; no `digits` upper limit (verified) | `Math.Round(2.675, 2)` was 2.68, now 2.67 | Use `decimal` for decimal quantities |
| `DateOnly`/`TimeOnly.TryParse*` throw on invalid styles/formats (verified) | `ArgumentException` from a `Try` method | Validate the style/format arguments |
| `IHost.RunAsync`/`StopAsync` throw on `BackgroundService` failure (verified) | Non-zero exit code; `catch` blocks around `RunAsync` now run | Intended; log and exit |
| Nine `Microsoft.Extensions.*` libraries in the shared framework (verified: in the ref pack, and restore reports NU1510) | NU1510 for explicit references; assemblies not copied to output | Remove the references for `net11.0` |
| `IAsyncValidateOptions<T>` derives from `IValidateOptions<T>` | Synchronous access to async-only-validated options throws (verified) | `ValidateOnStart()` and start the host first; implement `Validate` |
| `IStartupValidator` obsolete (SYSLIB0066), `RSACryptoServiceProvider.Encrypt/Decrypt(byte[], bool)` obsolete (SYSLIB0064), `AsnEncodedData.RawData` setter obsolete (SYSLIB0065) (verified) | Warnings with custom IDs | Migrate APIs |
| `ChangeToken.OnChange` gains `Func<Task>`/`Func<TState, Task>` overloads (verified present only on `net11.0`) | Existing `async` lambdas silently rebind from `Action` (`async void`) to `Func<Task>`: re-registration now waits for the task, and changes during the callback are coalesced | Usually an improvement; cast to `Action` to keep fire-and-forget |
| `Environment.TickCount`/`TickCount64` on Windows exclude sleep/hibernation time | Timeouts measured with tick counts no longer "expire" across a suspend | Use `DateTime.UtcNow`/`TimeProvider` if wall-clock time is meant |
| `CborReader`/`CborWriter` enforce a default maximum nesting depth | Deeply nested CBOR that used to be accepted now fails | Keep a depth limit for untrusted input; see the breaking-change article before raising it |
| `[OptionsValidator]` source generator reports error SYSLIB1205 when the class also implements `IValidateOptions<T>.Validate` explicitly (RC 1) | Build error; previously two divergent `Validate` paths existed | Keep either the generator or the handwritten `Validate`, not both |
| VSTest / `Microsoft.NET.Test.Sdk` no longer bring in `Newtonsoft.Json` | Test projects that used `JsonConvert`/`JObject` transitively fail to compile, or fail at run time with `FileNotFoundException` | Add an explicit `Newtonsoft.Json` `PackageReference` |
| NuGet restore no longer considers the global packages folder when the requested version does not exist (RC 1) | Restore may pick a different (higher) version from the sources, or fail with NU1102 where a cached version used to satisfy it | Reference only versions that exist on your package sources |
| Named mutexes default to user scope on Unix | Cross-user coordination stops working | Review cross-user named-mutex use against the breaking-change article |
| ZIP entries validate CRC32; TAR headers validate checksums; `TarWriter` writes hard-link entries | Corrupt or tampered archives now fail while reading | Handle read failures on untrusted archives |
| `DeflateStream`/`GZipStream` write headers/footers for empty payloads | Byte-exact output comparisons change | Update golden files |
| `Environment.GetCommandLineArgs()[0]` preserves the apphost invocation name | Code that parses argv[0] changes | Use `Environment.ProcessPath` |
| `configProperties` in `.runtimeconfig.dev.json` override `.runtimeconfig.json` | Dev-only settings win locally | Review dev runtimeconfig |
| `Nullable.GetUnderlyingType` throws for custom `Type` subclasses; `Decimal`/`BigInteger` float conversions correctly rounded; `Complex` special values per C23 Annex G; Japanese calendar minimum date corrected | Edge-case behaviour | Re-run numeric/golden tests |
| `SslStream` server-side AIA downloads disabled; Linux AIA fetching capped at two per chain | Incomplete client chains may fail validation on servers | Send full chains |
| DSA removed on macOS; HKDF on Windows uses CNG | Platform-specific crypto behaviour | Move off DSA |
| NativeAOT native library outputs on Unix get a `lib` prefix | Build scripts that copy `foo.so` break | Update scripts |

Sources: .NET 11 breaking changes index and linked articles.[^11]

##### ASP.NET Core 11 and EF Core 11

- `ConcurrencyLimiter` middleware removed; MVC compatibility options removed; Microsoft.OpenApi 3.x with OpenAPI 3.2 as the default document version (verified: `AddOpenApi()` emits `"openapi": "3.2.0"` with `Microsoft.AspNetCore.OpenApi` 11.0.0-rc.2); OpenAPI server URL no longer has a trailing slash when `PathBase` is empty; Blazor static SSR defers antiforgery validation to the middleware; `WebAssemblyHostBuilder` loads environment variables into `IConfiguration`; OpenAPI lists all `ProducesResponseType` entries per status code; response compression always emits `Vary: Accept-Encoding`; hosting emits OpenTelemetry HTTP semantic-convention tags; Kestrel stricter protocol compliance; HttpSys exposes TLS channel binding tokens by default; passkey sign-in enforces confirmation and lockout; SqlClient Entra ID authentication moved to a separate package.[^15]
- EF Core 11: requires .NET 11; SQL Server compatibility level 160 default; `Migrate()` throws without migrations; split-query concurrency exception; `SqlVector<T>` not loaded by default; `EFOptimizeContext` removed; tools packages no longer reference `Microsoft.EntityFrameworkCore.Design` (add it explicitly); Cosmos provider: unmapped JSON properties are no longer preserved (data loss on save!), `__jObject` removed, sync I/O impossible, `id` characters no longer escaped.[^17]

---

#### Production / Review Checklist

##### Version and build

- [ ] `TargetFramework` is `net11.0` (or the project deliberately multi-targets and keeps `union`/`closed` and other C# 15 syntax out of the older legs)
- [ ] No `<LangVersion>preview</LangVersion>`, no `runtime-async` or memory-safety feature flags without an explicit decision
- [ ] No suppressed experimental diagnostics (`SYSLIB5006` composite ML-KEM, `SYSLIB5008` `ShouldEvictConnection`, `SYSLIB5009` HPKE, `SYSLIB5007`) in production code without an explicit decision
- [ ] `global.json` moves off the RC SDK once .NET 11 GA ships; RC 2 go-live support ends 2026-11-10
- [ ] `Nullable` enabled, `TreatWarningsAsErrors` on, `AnalysisLevel` set, `EnforceCodeStyleInBuild` on
- [ ] No `PackageReference` to the nine `Microsoft.Extensions.*` libraries now in the shared framework (`net11.0`-only projects)
- [ ] Package versions central and locked; `dotnet list package --vulnerable --include-transitive` clean

##### C# 15 features

- [ ] Every switch over a union or closed hierarchy is a switch **expression** without a discard arm; `CS8509`/`CS8655` are errors
- [ ] Union switches handle `null` wherever the union can be `default` (fields, arrays, `default`, out parameters)
- [ ] Union emptiness tested via `.Value is null` / `is null`, not by inspecting the struct
- [ ] Public unions/closed hierarchies treated as versioned contracts (adding a case is breaking)
- [ ] Unions with value-type cases not used on allocation-sensitive hot paths (boxing)
- [ ] Unions crossing JSON boundaries have token-distinguishable cases or a classifier; object-shaped alternatives use `[JsonPolymorphic]` closed hierarchies
- [ ] Custom `[Union]` types obey the soundness rules (`Value` only returns case values or `null`; `TryGetValue`/`HasValue` consistent)
- [ ] `closed` not relied on as a security boundary
- [ ] `with(...)` collection arguments used for comparers where key semantics matter; no method named `with` called inside collection expressions without `@`
- [ ] Extension indexers do not hide O(n) work or re-enumeration
- [ ] Labeled `break`/`continue` replace flag variables/`goto`; labels are on loops/switches only

##### Correctness

- [ ] No `!`/`default!` without justification; public APIs validate arguments
- [ ] No `async void` (except event handlers), no `.Result`/`.Wait()`, no fire-and-forget in request code
- [ ] `CancellationToken` accepted and forwarded everywhere; timeouts distinguished from caller cancellation
- [ ] `ValueTask` awaited once; `IAsyncDisposable` with `await using`
- [ ] LINQ sequences not enumerated multiple times; deferred execution captures reviewed
- [ ] Ordinal string comparisons for identifiers; `CultureInfo.InvariantCulture` for machine parsing/formatting
- [ ] `decimal` for money; `checked` arithmetic where overflow matters; no reliance on wrap-around float→small-int conversions (.NET 11 saturates)
- [ ] `Math.Round(double, digits)` results re-validated after upgrade; `DateOnly/TimeOnly.TryParse*` callers pass valid styles
- [ ] Records with collection members do not rely on value equality; `with` shallow copies understood
- [ ] Exceptions: specific types, `throw;` to rethrow, no swallowing, `OperationCanceledException` propagates

##### Services and data

- [ ] DI lifetimes correct (no captive scoped dependencies); scopes created in singletons/background services
- [ ] Options validated with `ValidateOnStart`; async-only validators never read before startup; no `required` members in AOT-bound options
- [ ] `IHttpClientFactory` or a pooled long-lived `HttpClient`; responses disposed; URLs escaped
- [ ] `JsonSerializerOptions` cached; source generation for AOT/hot paths; strict settings for untrusted input
- [ ] Minimal APIs: `TypedResults`, `AddValidation()`, DTOs (not entities), auth by default, rate limiting (not the removed `ConcurrencyLimiter`), ProblemDetails in production
- [ ] Endpoint filters tolerate failed binding (they can run with status 400 and unbound arguments)
- [ ] `BackgroundService` loops observe `stoppingToken`; host failure exit codes expected (.NET 11)
- [ ] EF Core: parameterized SQL (`FromSql($"...")`), `AsNoTracking` for reads, no N+1, pagination, no shared `DbContext` across threads, compatibility level matches the SQL Server version, `DbQueryConcurrencyException` handled for split queries

##### Security

- [ ] `RandomNumberGenerator` for secrets; `FixedTimeEquals` for secret comparison
- [ ] Modern algorithms only (AES-GCM, SHA-256+, PBKDF2 ≥ 600k iterations via `Rfc2898DeriveBytes.Pbkdf2` or Identity's hasher); no obsolete crypto APIs
- [ ] No `BinaryFormatter`, no unsafe Newtonsoft `TypeNameHandling`, XML DTD processing prohibited
- [ ] Process arguments passed as lists; file paths resolved and containment-checked
- [ ] Secrets not in source/config/logs; TLS validation never disabled
- [ ] Regex timeouts / `NonBacktracking` on untrusted input; request and JSON size/depth limits

##### Performance and operations

- [ ] Claims backed by benchmarks or profiles; hot paths free of LINQ/closure/boxing churn
- [ ] Runtime async opt-in load-tested before enabling; diagnostics that inspect async state machines reviewed
- [ ] Structured logging with constant templates / `[LoggerMessage]`; no secrets or PII in logs
- [ ] Metrics and traces via `Meter`/`ActivitySource` and OpenTelemetry
- [ ] Hardware baseline (x86-64-v2) satisfied by production and CI machines
- [ ] AOT/trim warnings (IL2xxx/IL3xxx) fixed, not suppressed, in AOT-published apps

##### Testing

- [ ] Tests for every union case and closed-hierarchy subclass, including `default`
- [ ] Time-dependent code tested with `FakeTimeProvider`
- [ ] Integration tests use `WebApplicationFactory` and a real database engine (not EF InMemory) for query behaviour
- [ ] New test projects on xUnit v3 (`--xunit-version v3`) with Microsoft.Testing.Platform configured in `global.json`

---

[^1]: .NET 11.0 downloads — release list with dates, SDK/runtime versions and go-live status (RC 2 2026-10-13, RC 1 2026-09-08). https://dotnet.microsoft.com/en-us/download/dotnet/11.0
[^2]: What's new in .NET 11 (updated for RC 2; GA expected November 2026; C# 15 is the default for net11.0). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/overview
[^3]: .NET Conf 2026 announcement (November 10–12, 2026, .NET 11 launch). https://devblogs.microsoft.com/dotnet/dotnet-conf-2026/
[^4]: .NET and .NET Core official support policy (versions, latest patches 10.0.13/9.0.21/8.0.32 on 2026-10-13, end-of-support dates, go-live RC support windows). https://dotnet.microsoft.com/en-us/platform/support/policy/dotnet-core
[^5]: .NET STS releases extended to 24 months starting with .NET 9 (heise report of the September 2025 announcement); .NET 8 and .NET 9 end of support on November 10, 2026. https://heise.de/-10661376 , https://devblogs.microsoft.com/dotnet/dotnet-8-9-end-of-support/
[^6]: What's new in C# 15. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
[^7]: Roslyn — Language Feature Status (C# 15.0 and working set). https://github.com/dotnet/roslyn/blob/main/docs/Language%20Feature%20Status.md
[^8]: C# compiler breaking changes since C# 14 (.NET 11). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%2011
[^9]: What's new in the .NET 11 runtime (Runtime Async, JIT, NativeAOT, ReadyToRun). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/runtime
[^10]: Breaking change: Minimum hardware requirements updated (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/jit/11/minimum-hardware-requirements
[^11]: Breaking changes in .NET 11 (index of core library, crypto, extensions, deployment, interop, JIT, networking and SDK changes). https://learn.microsoft.com/en-us/dotnet/core/compatibility/11
[^12]: What's new in the .NET 11 libraries. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/libraries
[^13]: Breaking change: Some Microsoft.Extensions packages included in shared framework (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/11/extensions-in-shared-framework
[^14]: What's new in ASP.NET Core in .NET 11. https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-11
[^15]: Breaking changes in ASP.NET Core 11. https://learn.microsoft.com/en-us/aspnet/core/breaking-changes/11/overview
[^16]: What's new in EF Core 11. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-11.0/whatsnew
[^17]: Breaking changes in EF Core 11. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-11.0/breaking-changes
[^18]: What's new in the SDK and tooling for .NET 11. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/sdk
[^19]: What's new in C# 14. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-14
[^20]: Breaking change: .NET 11 obsoletions with custom IDs (SYSLIB0064, SYSLIB0066). https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/11/obsolete-apis
[^21]: Union types — C# reference. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/builtin-types/union
[^22]: What's new in C# 15 — Closed hierarchies; closed modifier reference. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15#closed-hierarchies
[^23]: Collection expressions — collection expression arguments. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/operators/collection-expressions#collection-expression-arguments
[^24]: Jump statements — labeled break and continue. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/statements/jump-statements
[^25]: IDE0410 — Use labeled jump statement. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/style-rules/ide0410
[^26]: Unsafe code, pointer types, and function pointers — the updated memory safety model (preview). https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/unsafe-code
[^27]: Breaking change: DateOnly and TimeOnly TryParse methods throw for invalid input. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/11/dateonly-timeonly-tryparse-argumentexception
[^28]: Breaking change: System.Linq.AsyncEnumerable in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/asyncenumerable
[^29]: Breaking change: Floating-point conversions to small integral types are saturating (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/jit/11/fp-to-small-integer
[^30]: Breaking change: Math.Round and MathF.Round return correctly rounded results (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/11/math-round-digits
[^31]: Breaking change: Synchronous access to async-validated options throws (.NET 11 RC 1). https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/11/async-options-validation-sync-access
[^32]: Breaking change: IHost.RunAsync and IHost.StopAsync throw when a BackgroundService fails (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/11/ihost-runasync-stopasync-throw-backgroundservice-failure
[^33]: Breaking change: .NET 11 obsoletions in cryptography (SYSLIB0065). https://learn.microsoft.com/en-us/dotnet/core/compatibility/cryptography/11/obsolete-apis
